package moq

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Runner interface {
	Run(context.Context) error
}

type ForwardRunner struct {
	cfg Config
}

type PublishRunner struct {
	cfg Config
}

func NewForwardRunner(cfg Config) (*ForwardRunner, error) {
	normalized := NormalizeConfig(cfg)
	if err := ValidateForwardConfig(normalized); err != nil {
		return nil, err
	}
	return &ForwardRunner{cfg: normalized}, nil
}

func NewPublishRunner(cfg Config) (*PublishRunner, error) {
	normalized := NormalizeConfig(cfg)
	if err := ValidatePublishConfig(normalized); err != nil {
		return nil, err
	}
	return &PublishRunner{cfg: normalized}, nil
}

func (runner *ForwardRunner) Run(ctx context.Context) error {
	return runPipe(
		ctx,
		ffmpegForwardArgs(runner.cfg.RTSPSourceURL),
		"moq-cli",
		moqPublishArgs(runner.cfg, "ts"),
		runner.cfg.CommandLogWriter,
		fmt.Sprintf("forwarding RTSP to MoQ relay=%s broadcast=%s\n", runner.cfg.RelayURL, runner.cfg.Broadcast),
	)
}

func (runner *PublishRunner) Run(ctx context.Context) error {
	if runner.cfg.CatalogControlDynamic != "" {
		return runner.runWithDynamicCatalogControl(ctx)
	}

	return runCommand(
		ctx,
		"cstream-moq-publisher",
		cstreamMoQPublisherArgs(runner.cfg),
		runner.cfg.CommandLogWriter,
		publishStartMessage(runner.cfg),
	)
}

func (runner *PublishRunner) runWithDynamicCatalogControl(ctx context.Context) error {
	tempDir, err := os.MkdirTemp("", "cstream-moq-catalog-*")
	if err != nil {
		return fmt.Errorf("create catalog control temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	controlPath := filepath.Join(tempDir, "catalog-control.json")
	if err := seedCatalogControlFile(controlPath, runner.cfg.CatalogControl); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dynamicErr := make(chan error, 1)
	go func() {
		dynamicErr <- runDynamicCatalogControl(
			ctx,
			runner.cfg.CatalogControlDynamic,
			controlPath,
			runner.cfg.CommandLogWriter,
		)
	}()

	commandErr := make(chan error, 1)
	go func() {
		cfg := runner.cfg
		cfg.CatalogControl = controlPath
		cfg.CatalogControlDynamic = ""
		commandErr <- runCommand(
			ctx,
			"cstream-moq-publisher",
			cstreamMoQPublisherArgs(cfg),
			cfg.CommandLogWriter,
			publishStartMessage(cfg),
		)
	}()

	select {
	case err := <-dynamicErr:
		cancel()
		<-commandErr
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	case err := <-commandErr:
		cancel()
		<-dynamicErr
		return err
	case <-ctx.Done():
		cancel()
		<-dynamicErr
		<-commandErr
		return ctx.Err()
	}
}

func runCommand(ctx context.Context, command string, args []string, logWriter io.Writer, startMessage string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter

	if startMessage != "" {
		_, _ = io.WriteString(logWriter, startMessage)
	}

	if err := cmd.Run(); err != nil && !errors.Is(ctx.Err(), context.Canceled) {
		return fmt.Errorf("%s failed: %w", command, err)
	}

	return ctx.Err()
}

func runPipe(ctx context.Context, ffmpegArgs []string, publisherCommand string, publisherArgs []string, logWriter io.Writer, startMessage string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()

	ffmpeg := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	publisher := exec.CommandContext(ctx, publisherCommand, publisherArgs...)

	ffmpeg.Stdout = pipeWriter
	ffmpeg.Stderr = logWriter
	publisher.Stdin = pipeReader
	publisher.Stdout = logWriter
	publisher.Stderr = logWriter

	if startMessage != "" {
		_, _ = io.WriteString(logWriter, startMessage)
	}

	if err := publisher.Start(); err != nil {
		pipeWriter.Close()
		return fmt.Errorf("start %s: %w", publisherCommand, err)
	}

	if err := ffmpeg.Start(); err != nil {
		pipeWriter.Close()
		cancel()
		_ = publisher.Wait()
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	ffmpegDone := make(chan error, 1)
	go func() {
		err := ffmpeg.Wait()
		_ = pipeWriter.Close()
		ffmpegDone <- err
	}()

	publisherDone := make(chan error, 1)
	go func() {
		publisherDone <- publisher.Wait()
	}()

	select {
	case err := <-ffmpegDone:
		if err != nil && !errors.Is(ctx.Err(), context.Canceled) {
			cancel()
			_ = pipeReader.Close()
			<-publisherDone
			return fmt.Errorf("ffmpeg failed: %w", err)
		}
		if err := <-publisherDone; err != nil && !errors.Is(ctx.Err(), context.Canceled) {
			return fmt.Errorf("%s failed: %w", publisherCommand, err)
		}
		return ctx.Err()
	case err := <-publisherDone:
		cancel()
		_ = pipeReader.Close()
		if err != nil && !errors.Is(ctx.Err(), context.Canceled) {
			<-ffmpegDone
			return fmt.Errorf("%s failed: %w", publisherCommand, err)
		}
		<-ffmpegDone
		return ctx.Err()
	case <-ctx.Done():
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		<-ffmpegDone
		<-publisherDone
		return ctx.Err()
	}
}

func ffmpegForwardArgs(rtspSourceURL string) []string {
	return []string{
		"-hide_banner",
		"-nostdin",
		"-fflags",
		"nobuffer",
		"-rtsp_transport",
		"tcp",
		"-i",
		rtspSourceURL,
		"-c",
		"copy",
		"-f",
		"mpegts",
		"-flush_packets",
		"1",
		"-muxdelay",
		"0",
		"-muxpreload",
		"0",
		"-",
	}
}

func moqPublishArgs(cfg Config, format string) []string {
	args := []string{
		"publish",
		"--client-bind",
		cfg.ClientBind,
	}

	if cfg.TLSDisableVerify {
		args = append(args, "--tls-disable-verify")
	}

	args = append(args,
		"--url",
		cfg.RelayURL,
		"--broadcast",
		cfg.Broadcast,
		format,
	)

	return args
}

func cstreamMoQPublisherArgs(cfg Config) []string {
	args := []string{
		"--client-bind",
		cfg.ClientBind,
	}

	if cfg.TLSDisableVerify {
		args = append(args, "--tls-disable-verify")
	}

	if len(cfg.RenditionSources) == 0 {
		args = append(args,
			"--source-rtsp",
			cfg.RTSPSourceURL,
			"--video-codec",
			cfg.VideoCodec,
		)
	}

	args = append(args,
		"--url",
		cfg.RelayURL,
		"--broadcast",
		cfg.Broadcast,
	)

	if len(cfg.RenditionSources) > 0 {
		for _, source := range cfg.RenditionSources {
			args = append(args, "--rendition-source", formatRenditionSource(source))
			args = append(args, "--advertise-rendition", source.Name)
		}
	} else {
		for _, rendition := range cfg.Renditions {
			args = append(args, "--rendition", formatRendition(rendition))
			args = append(args, "--advertise-rendition", rendition.Name)
		}
	}

	if cfg.CatalogControl != "" {
		args = append(args, "--catalog-control", cfg.CatalogControl)
	}

	return args
}

func publishStartMessage(cfg Config) string {
	if len(cfg.RenditionSources) > 0 {
		return fmt.Sprintf("publishing RTSP rendition sources to MoQ relay=%s broadcast=%s renditions=%d\n", cfg.RelayURL, cfg.Broadcast, len(cfg.RenditionSources))
	}
	return fmt.Sprintf("publishing RTSP renditions to MoQ relay=%s broadcast=%s codec=%s renditions=%d\n", cfg.RelayURL, cfg.Broadcast, cfg.VideoCodec, len(cfg.Renditions))
}

func formatRendition(rendition Rendition) string {
	if rendition.Passthrough {
		if rendition.Name == "passthrough" {
			return "passthrough"
		}
		return fmt.Sprintf("%s:passthrough", rendition.Name)
	}
	return fmt.Sprintf("%s:%dx%d:%s", rendition.Name, rendition.Width, rendition.Height, rendition.Bitrate)
}

func formatRenditionSource(source RenditionSource) string {
	return fmt.Sprintf("%s=%s", source.Name, source.RTSPURL)
}

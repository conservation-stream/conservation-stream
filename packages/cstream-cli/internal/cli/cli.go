package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"cstream-cli/internal/forward"
	"cstream-cli/internal/moq"
	"cstream-cli/internal/pipeline"

	ucli "github.com/urfave/cli/v2"
)

const version = "0.1.0"

type publishPipeline interface {
	Run(ctx context.Context) error
}

type forwardRunner interface {
	Run(ctx context.Context) error
}

type moqRunner interface {
	Run(ctx context.Context) error
}

var newPublishPipeline = func(cfg pipeline.Config) (publishPipeline, error) {
	return pipeline.NewPipeline(cfg)
}

var newForwardRunner = func(cfg forward.Config) (forwardRunner, error) {
	return forward.NewRunner(cfg)
}

var newMoQForwardRunner = func(cfg moq.Config) (moqRunner, error) {
	return moq.NewForwardRunner(cfg)
}

var newMoQPublishRunner = func(cfg moq.Config) (moqRunner, error) {
	return moq.NewPublishRunner(cfg)
}

func Run(args []string) error {
	argv := append([]string{"cstream"}, args...)
	return run(argv, os.Stdout, os.Stderr)
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	app := newApp(stdout, stderr)
	return app.Run(args)
}

func newApp(stdout io.Writer, stderr io.Writer) *ucli.App {
	return &ucli.App{
		Name:      "cstream",
		Usage:     "starter Go CLI",
		Version:   version,
		Writer:    stdout,
		ErrWriter: stderr,
		Action: func(ctx *ucli.Context) error {
			if ctx.Args().Len() > 0 {
				return fmt.Errorf("unknown command: %q", ctx.Args().First())
			}

			return ucli.ShowAppHelp(ctx)
		},
		Commands: []*ucli.Command{
			rtspCommand(),
			webrtcCommand(),
			moqCommand(),
			{
				Name:  "version",
				Usage: "Print CLI version",
				Action: func(ctx *ucli.Context) error {
					_, err := fmt.Fprintln(ctx.App.Writer, ctx.App.Version)
					return err
				},
			},
		},
	}
}

func rtspCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "rtsp",
		Usage: "RTSP/RTMP stream publishing",
		Subcommands: []*ucli.Command{
			{
				Name:  "publish",
				Usage: "Publish with encoding to RTSP/RTMP output",
				Flags: publishFlags(),
				Action: func(ctx *ucli.Context) error {
					in := ctx.String("in")
					out := ctx.String("out")
					dynamic := ctx.String("dynamic")
					preset := ctx.String("preset")
					baseBitrate := ctx.Uint("base-bitrate")
					height := ctx.String("height")
					width := ctx.String("width")
					rate := ctx.String("rate")
					debug := ctx.Bool("debug")

					if err := validatePublish(in, out, dynamic, preset, baseBitrate); err != nil {
						return err
					}

					cfg, err := newPublishPipelineConfig(in, out, dynamic, baseBitrate, height, width, rate, ctx.App.Writer, debug)
					if err != nil {
						return err
					}

					publishPipeline, err := newPublishPipeline(cfg)
					if err != nil {
						return err
					}

					return publishPipeline.Run(ctx.Context)
				},
			},
		},
	}
}

func webrtcCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "webrtc",
		Usage: "WebRTC forwarding",
		Subcommands: []*ucli.Command{
			{
				Name:  "forward",
				Usage: "Forward RTSP input to WHIP/WebRTC output without encoding",
				Flags: []ucli.Flag{
					&ucli.StringFlag{
						Name:     "in",
						Usage:    "input RTSP URL",
						Required: true,
					},
					&ucli.StringFlag{
						Name:     "out",
						Usage:    "output WHIP HTTPS URL",
						Required: true,
					},
					&ucli.BoolFlag{
						Name:  "debug",
						Usage: "enable verbose WebRTC forward debug logs",
					},
				},
				Action: func(ctx *ucli.Context) error {
					cfg, err := newWebRTCForwardConfig(ctx.String("in"), ctx.String("out"), ctx.App.Writer, ctx.Bool("debug"))
					if err != nil {
						return err
					}

					runner, err := newForwardRunner(cfg)
					if err != nil {
						return err
					}

					return runner.Run(ctx.Context)
				},
			},
		},
	}
}

func moqCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "moq",
		Usage: "Media over QUIC publishing and forwarding",
		Subcommands: []*ucli.Command{
			{
				Name:  "forward",
				Usage: "Forward RTSP input to MoQ without encoding",
				Flags: moqBaseFlags(false),
				Action: func(ctx *ucli.Context) error {
					cfg, err := newMoQForwardConfig(ctx)
					if err != nil {
						return err
					}

					runner, err := newMoQForwardRunner(cfg)
					if err != nil {
						return err
					}

					return runner.Run(ctx.Context)
				},
			},
			{
				Name:  "publish",
				Usage: "Publish encoded RTSP renditions to MoQ",
				Flags: moqBaseFlags(true),
				Action: func(ctx *ucli.Context) error {
					cfg, err := newMoQPublishConfig(ctx)
					if err != nil {
						return err
					}

					runner, err := newMoQPublishRunner(cfg)
					if err != nil {
						return err
					}

					return runner.Run(ctx.Context)
				},
			},
		},
	}
}

func publishFlags() []ucli.Flag {
	return []ucli.Flag{
		&ucli.StringFlag{
			Name:     "in",
			Usage:    "input RTSP/RTMP URL",
			Required: true,
		},
		&ucli.StringFlag{
			Name:     "out",
			Usage:    "output RTSP/RTMP URL",
			Required: true,
		},
		&ucli.StringFlag{
			Name:  "dynamic",
			Usage: "dynamic config WebSocket URL (ws:// or wss://)",
		},
		&ucli.UintFlag{
			Name:  "base-bitrate",
			Usage: "initial encoder bitrate in kbps (required with --dynamic)",
		},
		&ucli.StringFlag{
			Name:  "preset",
			Usage: "publish preset: twitch|youtube",
		},
		&ucli.StringFlag{
			Name:     "height",
			Usage:    "origin frame height",
			Required: true,
		},
		&ucli.StringFlag{
			Name:     "width",
			Usage:    "origin frame width",
			Required: true,
		},
		&ucli.StringFlag{
			Name:     "rate",
			Usage:    "origin frame rate",
			Required: true,
		},
		&ucli.BoolFlag{
			Name:  "debug",
			Usage: "enable verbose publish debug logs",
		},
	}
}

func moqBaseFlags(includeRenditions bool) []ucli.Flag {
	flags := []ucli.Flag{
		&ucli.StringFlag{
			Name:     "in",
			Usage:    "input RTSP URL",
			Required: true,
		},
		&ucli.StringFlag{
			Name:     "out",
			Usage:    "MoQ relay URL",
			Required: true,
		},
		&ucli.StringFlag{
			Name:     "broadcast",
			Usage:    "MoQ broadcast name",
			Required: true,
		},
		&ucli.StringFlag{
			Name:  "moq-client-bind",
			Usage: "MoQ client UDP bind address",
			Value: moq.DefaultClientBind,
		},
		&ucli.BoolFlag{
			Name:  "moq-tls-disable-verify",
			Usage: "disable MoQ relay TLS certificate verification",
		},
		&ucli.BoolFlag{
			Name:  "debug",
			Usage: "enable verbose MoQ logs",
		},
	}

	if includeRenditions {
		flags = append(flags, &ucli.StringSliceFlag{
			Name:  "rendition",
			Usage: "rendition as name:WIDTHxHEIGHT:BITRATE or passthrough; repeat for ABR ladders",
		}, &ucli.StringFlag{
			Name:  "video-codec",
			Usage: "MoQ video codec: h264|h265",
			Value: moq.DefaultVideoCodec,
		}, &ucli.StringFlag{
			Name:  "catalog-control",
			Usage: "JSON file watched for live MoQ catalog advertisement changes",
		}, &ucli.StringFlag{
			Name:  "catalog-control-dynamic",
			Usage: "dynamic MoQ catalog control WebSocket URL (ws:// or wss://)",
		})
	}

	return flags
}

func validatePublish(in string, out string, dynamic string, preset string, baseBitrate uint) error {
	if _, err := publishConnectionFromURL("--in", in); err != nil {
		return err
	}

	if _, err := publishConnectionFromURL("--out", out); err != nil {
		return err
	}

	hasDynamic := strings.TrimSpace(dynamic) != ""
	hasPreset := strings.TrimSpace(preset) != ""

	if hasDynamic == hasPreset {
		return errors.New("exactly one of --dynamic or --preset is required")
	}

	if hasDynamic {
		if err := validateURLScheme("--dynamic", dynamic, "ws", "wss"); err != nil {
			return err
		}
		if baseBitrate == 0 {
			return errors.New("--base-bitrate is required when --dynamic is set")
		}
	}

	if !hasDynamic && baseBitrate > 0 {
		return errors.New("--base-bitrate can only be used with --dynamic")
	}

	if hasPreset {
		switch preset {
		case "twitch", "youtube":
		default:
			return errors.New("--preset must be one of: twitch, youtube")
		}
	}

	return nil
}

func newPublishPipelineConfig(in string, out string, dynamic string, baseBitrate uint, height string, width string, rate string, stdout io.Writer, debug bool) (pipeline.Config, error) {
	inConnection, err := publishConnectionFromURL("--in", in)
	if err != nil {
		return pipeline.Config{}, err
	}

	outConnection, err := publishConnectionFromURL("--out", out)
	if err != nil {
		return pipeline.Config{}, err
	}

	cfg := pipeline.Config{
		In:  inConnection,
		Out: outConnection,
		OriginFrameInfo: pipeline.OriginFrameInfo{
			Height: height,
			Width:  width,
			Rate:   rate,
		},
		Debug:     debug,
		LogWriter: stdout,
		RuntimeFixes: pipeline.RuntimeFixes{
			EnforceMonotonicH264PTS:     outConnection.Type == pipeline.ConnectionTypeRTSP,
			DropRestartEventsAfterFirst: outConnection.Type == pipeline.ConnectionTypeRTSP,
		},
	}

	if strings.TrimSpace(dynamic) != "" {
		cfg.OnRunning = watchDynamicBitrate(stdout, dynamic, baseBitrate)
	}

	return cfg, nil
}

func publishConnectionFromURL(flagName string, raw string) (pipeline.Connection, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return pipeline.Connection{}, fmt.Errorf("%s must be a valid URL: %w", flagName, err)
	}

	if u.Scheme == "" || u.Host == "" {
		return pipeline.Connection{}, fmt.Errorf("%s must include scheme and host", flagName)
	}

	switch u.Scheme {
	case "rtsp", "rtsps":
		return pipeline.Connection{Type: pipeline.ConnectionTypeRTSP, URL: raw}, nil
	case "rtmp", "rtmps":
		return pipeline.Connection{Type: pipeline.ConnectionTypeRTMP, URL: raw}, nil
	default:
		return pipeline.Connection{}, errors.New(flagName + " must use one of: rtsp, rtsps, rtmp, rtmps")
	}
}

func newWebRTCForwardConfig(in string, out string, stdout io.Writer, debug bool) (forward.Config, error) {
	if err := validateWebRTCForward(in, out); err != nil {
		return forward.Config{}, err
	}

	logger := forward.NewNoopLogger()
	if debug {
		logger = forward.NewLogger(stdout)
	}

	return forward.Config{
		RTSPSourceURL:  in,
		WHIPPublishURL: out,
		Logger:         logger,
	}, nil
}

func validateWebRTCForward(in string, out string) error {
	if err := validateURLScheme("--in", in, "rtsp"); err != nil {
		return err
	}

	if err := validateURLScheme("--out", out, "https"); err != nil {
		return err
	}

	return nil
}

func newMoQForwardConfig(ctx *ucli.Context) (moq.Config, error) {
	cfg, err := newMoQBaseConfig(ctx)
	if err != nil {
		return moq.Config{}, err
	}
	return cfg, moq.ValidateForwardConfig(moq.NormalizeConfig(cfg))
}

func newMoQPublishConfig(ctx *ucli.Context) (moq.Config, error) {
	cfg, err := newMoQBaseConfig(ctx)
	if err != nil {
		return moq.Config{}, err
	}

	for _, raw := range ctx.StringSlice("rendition") {
		rendition, err := moq.ParseRendition(raw)
		if err != nil {
			return moq.Config{}, fmt.Errorf("--rendition %q: %w", raw, err)
		}
		cfg.Renditions = append(cfg.Renditions, rendition)
	}
	cfg.CatalogControl = ctx.String("catalog-control")
	cfg.CatalogControlDynamic = ctx.String("catalog-control-dynamic")
	cfg.VideoCodec = ctx.String("video-codec")

	if strings.TrimSpace(cfg.CatalogControlDynamic) != "" {
		if err := validateURLScheme("--catalog-control-dynamic", cfg.CatalogControlDynamic, "ws", "wss"); err != nil {
			return moq.Config{}, err
		}
	}

	return cfg, moq.ValidatePublishConfig(moq.NormalizeConfig(cfg))
}

func newMoQBaseConfig(ctx *ucli.Context) (moq.Config, error) {
	in := ctx.String("in")
	out := ctx.String("out")
	broadcast := ctx.String("broadcast")

	if err := validateMoQBase(in, out, broadcast); err != nil {
		return moq.Config{}, err
	}

	relayURL, err := moqRelayURL(out)
	if err != nil {
		return moq.Config{}, err
	}

	commandLogWriter := io.Discard
	if ctx.Bool("debug") {
		commandLogWriter = ctx.App.Writer
	}

	return moq.Config{
		RTSPSourceURL:    in,
		RelayURL:         relayURL,
		Broadcast:        broadcast,
		ClientBind:       ctx.String("moq-client-bind"),
		TLSDisableVerify: ctx.Bool("moq-tls-disable-verify"),
		CommandLogWriter: commandLogWriter,
	}, nil
}

func validateMoQBase(in string, out string, broadcast string) error {
	if err := validateURLScheme("--in", in, "rtsp"); err != nil {
		return err
	}

	if err := validateURLScheme("--out", out, "https"); err != nil {
		return err
	}

	if strings.TrimSpace(broadcast) == "" {
		return errors.New("--broadcast is required")
	}

	if strings.ContainsAny(broadcast, " \t\r\n") {
		return errors.New("--broadcast must not contain whitespace")
	}

	return nil
}

func moqRelayURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("--out must be a valid URL: %w", err)
	}
	u.Fragment = ""
	return u.String(), nil
}

func validateURLScheme(flagName string, raw string, allowedSchemes ...string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", flagName, err)
	}

	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must include scheme and host", flagName)
	}

	for _, scheme := range allowedSchemes {
		if u.Scheme == scheme {
			return nil
		}
	}

	return fmt.Errorf("%s must use one of: %s", flagName, strings.Join(allowedSchemes, ", "))
}

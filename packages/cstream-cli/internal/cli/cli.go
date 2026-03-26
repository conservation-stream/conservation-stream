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

var newPublishPipeline = func(cfg pipeline.Config) (publishPipeline, error) {
	return pipeline.NewPipeline(cfg)
}

var newForwardRunner = func(cfg forward.Config) (forwardRunner, error) {
	return forward.NewRunner(cfg)
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
			{
				Name:  "publish",
				Usage: "Publish a stream to RTSP/RTMP output",
				Flags: []ucli.Flag{
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
				},
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
			{
				Name:  "forward",
				Usage: "Forward RTSP input to WHIP output",
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
						Usage: "enable verbose forward debug logs",
					},
				},
				Action: func(ctx *ucli.Context) error {
					in := ctx.String("in")
					out := ctx.String("out")
					debug := ctx.Bool("debug")
					if err := validateForward(in, out); err != nil {
						return err
					}

					cfg, err := newForwardConfig(in, out, ctx.App.Writer, debug)
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

func newForwardConfig(in string, out string, stdout io.Writer, debug bool) (forward.Config, error) {
	if err := validateForward(in, out); err != nil {
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

func validateForward(in string, out string) error {
	if err := validateURLScheme("--in", in, "rtsp"); err != nil {
		return err
	}

	if err := validateURLScheme("--out", out, "https"); err != nil {
		return err
	}

	return nil
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

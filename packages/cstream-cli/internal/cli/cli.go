package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"cstream-cli/internal/forward"
	"cstream-cli/internal/pipeline"

	"github.com/go-gst/go-gst/gst"

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
						Usage: "dynamic config URL",
					},
					&ucli.StringFlag{
						Name:  "preset",
						Usage: "publish preset: twitch|youtube",
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
					debug := ctx.Bool("debug")

					if err := validatePublish(in, out, dynamic, preset); err != nil {
						return err
					}

					cfg, err := newPublishPipelineConfig(in, out, ctx.App.Writer, debug)
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

func validatePublish(in string, out string, dynamic string, preset string) error {
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
		if err := validateAnyURL("--dynamic", dynamic); err != nil {
			return err
		}
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

func newPublishPipelineConfig(in string, out string, stdout io.Writer, debug bool) (pipeline.Config, error) {
	inConnection, err := publishConnectionFromURL("--in", in)
	if err != nil {
		return pipeline.Config{}, err
	}

	outConnection, err := publishConnectionFromURL("--out", out)
	if err != nil {
		return pipeline.Config{}, err
	}

	return pipeline.Config{
		In:  inConnection,
		Out: outConnection,
		OriginFrameInfo: pipeline.OriginFrameInfo{
			Height: "720",
			Width:  "1280",
			Rate:   "30/1",
		},
		Debug:     debug,
		LogWriter: stdout,
		OnRunning: cyclePublishBitrate(stdout, []uint{100, 1000, pipeline.DefaultEncoderBitrateKbps}),
		RuntimeFixes: pipeline.RuntimeFixes{
			EnforceMonotonicH264PTS:     outConnection.Type == pipeline.ConnectionTypeRTSP,
			DropRestartEventsAfterFirst: outConnection.Type == pipeline.ConnectionTypeRTSP,
		},
	}, nil
}

func cyclePublishBitrate(stdout io.Writer, bitrates []uint) func(context.Context, *gst.Pipeline) error {
	return func(ctx context.Context, gstPipeline *gst.Pipeline) error {
		if len(bitrates) == 0 {
			<-ctx.Done()
			return ctx.Err()
		}

		encoder, err := gstPipeline.GetElementByName("encoder")
		if err != nil {
			return fmt.Errorf("get encoder element: %w", err)
		}

		encoder.SetProperty("bitrate", bitrates[0])
		_, _ = fmt.Fprintf(stdout, "publish bitrate=%d kbps\n", bitrates[0])

		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		index := 0
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				index = (index + 1) % len(bitrates)
				encoder.SetProperty("bitrate", bitrates[index])
				_, _ = fmt.Fprintf(stdout, "publish bitrate=%d kbps\n", bitrates[index])
			}
		}
	}
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

func validateAnyURL(flagName string, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", flagName, err)
	}

	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must include scheme and host", flagName)
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

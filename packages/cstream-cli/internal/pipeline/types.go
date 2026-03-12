package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/go-gst/go-gst/gst"
)

const (
	ConnectionTypeRTSP        = "rtsp"
	ConnectionTypeRTMP        = "rtmp"
	DefaultEncoderBitrateKbps = 4500
	DefaultEncoderSpeedPreset = "veryfast"
)

var (
	ErrUnsupportedInputType      = errors.New("unsupported input connection type")
	ErrUnsupportedOutputType     = errors.New("unsupported output connection type")
	ErrUnsupportedScheme         = errors.New("unsupported URL scheme for connection type")
	ErrUnsupportedConnectionPair = errors.New("unsupported connection pair")
)

type Connection struct {
	Type string
	URL  string
}

type OriginFrameInfo struct {
	Height string
	Width  string
	Rate   string
}

type RuntimeFixes struct {
	EnforceMonotonicH264PTS     bool
	DropRestartEventsAfterFirst bool
}

type Config struct {
	In                 Connection
	Out                Connection
	OriginFrameInfo    OriginFrameInfo
	EncoderSpeedPreset string
	RuntimeFixes       RuntimeFixes
	Debug              bool
	LogWriter          io.Writer

	OnReady   func(*gst.Pipeline) error
	OnRunning func(ctx context.Context, pipeline *gst.Pipeline) error
}

type Pipeline struct {
	cfg         Config
	gstPipeline *gst.Pipeline
	launch      string
}

func normalizeConfig(cfg Config) (Config, error) {
	in, err := normalizeConnection("in", cfg.In, ErrUnsupportedInputType)
	if err != nil {
		return Config{}, err
	}

	out, err := normalizeConnection("out", cfg.Out, ErrUnsupportedOutputType)
	if err != nil {
		return Config{}, err
	}

	normalized := cfg
	normalized.In = in
	normalized.Out = out
	normalized.OriginFrameInfo = OriginFrameInfo{
		Height: strings.TrimSpace(cfg.OriginFrameInfo.Height),
		Width:  strings.TrimSpace(cfg.OriginFrameInfo.Width),
		Rate:   strings.TrimSpace(cfg.OriginFrameInfo.Rate),
	}
	normalized.EncoderSpeedPreset = strings.TrimSpace(cfg.EncoderSpeedPreset)
	if normalized.EncoderSpeedPreset == "" {
		normalized.EncoderSpeedPreset = DefaultEncoderSpeedPreset
	}

	if !isSupportedPair(in.Type, out.Type) {
		return Config{}, fmt.Errorf("%w: in=%s out=%s", ErrUnsupportedConnectionPair, in.Type, out.Type)
	}

	if err := validateOriginFrameInfo(normalized.OriginFrameInfo); err != nil {
		return Config{}, err
	}

	return normalized, nil
}

func validateOriginFrameInfo(info OriginFrameInfo) error {
	values := []string{info.Height, info.Width, info.Rate}
	setCount := 0
	for _, value := range values {
		if value != "" {
			setCount++
		}
	}

	if setCount == 0 {
		return nil
	}

	if setCount != len(values) {
		return errors.New("origin frame info requires height, width, and rate")
	}

	return nil
}

func normalizeConnection(role string, connection Connection, typeErr error) (Connection, error) {
	connectionType := strings.TrimSpace(strings.ToLower(connection.Type))
	rawURL := strings.TrimSpace(connection.URL)

	if connectionType == "" {
		return Connection{}, fmt.Errorf("%s type is required", role)
	}

	if rawURL == "" {
		return Connection{}, fmt.Errorf("%s URL is required", role)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return Connection{}, fmt.Errorf("parse %s URL: %w", role, err)
	}

	if parsedURL.Host == "" {
		return Connection{}, fmt.Errorf("%s URL must include a host", role)
	}

	switch connectionType {
	case ConnectionTypeRTSP:
		if parsedURL.Scheme != "rtsp" && parsedURL.Scheme != "rtsps" {
			return Connection{}, fmt.Errorf("%w: %s URL requires rtsp or rtsps scheme", ErrUnsupportedScheme, role)
		}
	case ConnectionTypeRTMP:
		if parsedURL.Scheme != "rtmp" && parsedURL.Scheme != "rtmps" {
			return Connection{}, fmt.Errorf("%w: %s URL requires rtmp or rtmps scheme", ErrUnsupportedScheme, role)
		}
	default:
		return Connection{}, fmt.Errorf("%w: %s=%s", typeErr, role, connectionType)
	}

	return Connection{
		Type: connectionType,
		URL:  rawURL,
	}, nil
}

func isSupportedPair(in string, out string) bool {
	switch in {
	case ConnectionTypeRTSP, ConnectionTypeRTMP:
	default:
		return false
	}

	switch out {
	case ConnectionTypeRTSP, ConnectionTypeRTMP:
		return true
	default:
		return false
	}
}

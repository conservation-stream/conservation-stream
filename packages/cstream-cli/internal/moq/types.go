package moq

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

const DefaultClientBind = "0.0.0.0:0"
const DefaultVideoCodec = "h264"

type Config struct {
	RTSPSourceURL         string
	RelayURL              string
	Broadcast             string
	ClientBind            string
	TLSDisableVerify      bool
	CatalogControl        string
	CatalogControlDynamic string
	VideoCodec            string
	Renditions            []Rendition
	CommandLogWriter      io.Writer
}

type Rendition struct {
	Name        string
	Width       int
	Height      int
	Bitrate     string
	Passthrough bool
}

func NormalizeConfig(cfg Config) Config {
	normalized := cfg
	normalized.RTSPSourceURL = strings.TrimSpace(cfg.RTSPSourceURL)
	normalized.RelayURL = strings.TrimSpace(cfg.RelayURL)
	normalized.Broadcast = strings.Trim(strings.TrimSpace(cfg.Broadcast), "/")
	normalized.ClientBind = strings.TrimSpace(cfg.ClientBind)
	normalized.CatalogControl = strings.TrimSpace(cfg.CatalogControl)
	normalized.CatalogControlDynamic = strings.TrimSpace(cfg.CatalogControlDynamic)
	normalized.VideoCodec = strings.ToLower(strings.TrimSpace(cfg.VideoCodec))
	if normalized.ClientBind == "" {
		normalized.ClientBind = DefaultClientBind
	}
	if normalized.VideoCodec == "" {
		normalized.VideoCodec = DefaultVideoCodec
	}
	if normalized.CommandLogWriter == nil {
		normalized.CommandLogWriter = io.Discard
	}
	return normalized
}

func ValidateForwardConfig(cfg Config) error {
	if cfg.RTSPSourceURL == "" {
		return fmt.Errorf("rtsp source URL is required")
	}
	if cfg.RelayURL == "" {
		return fmt.Errorf("MoQ relay URL is required")
	}
	if cfg.Broadcast == "" {
		return fmt.Errorf("MoQ broadcast is required")
	}
	return nil
}

func ValidatePublishConfig(cfg Config) error {
	if err := ValidateForwardConfig(cfg); err != nil {
		return err
	}
	if len(cfg.Renditions) == 0 {
		return fmt.Errorf("at least one --rendition is required")
	}
	if !isSupportedVideoCodec(cfg.VideoCodec) {
		return fmt.Errorf("video codec must be h264 or h265")
	}
	for index, rendition := range cfg.Renditions {
		if rendition.Name == "" {
			return fmt.Errorf("rendition %d name is required", index+1)
		}
		if rendition.Passthrough {
			continue
		}
		if rendition.Width <= 0 || rendition.Height <= 0 {
			return fmt.Errorf("rendition %q dimensions must be positive", rendition.Name)
		}
		if strings.TrimSpace(rendition.Bitrate) == "" {
			return fmt.Errorf("rendition %q bitrate is required", rendition.Name)
		}
	}
	return nil
}

func isSupportedVideoCodec(codec string) bool {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "h264", "h265":
		return true
	default:
		return false
	}
}

func ParseRendition(raw string) (Rendition, error) {
	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, "passthrough") {
		return Rendition{Name: "passthrough", Passthrough: true}, nil
	}

	parts := strings.Split(raw, ":")
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "passthrough") {
		name, err := parseRenditionName(parts[0])
		if err != nil {
			return Rendition{}, err
		}
		return Rendition{Name: name, Passthrough: true}, nil
	}

	if len(parts) != 3 {
		return Rendition{}, fmt.Errorf("rendition must use name:WIDTHxHEIGHT:BITRATE, passthrough, or name:passthrough")
	}

	name, err := parseRenditionName(parts[0])
	if err != nil {
		return Rendition{}, err
	}

	dimensions := strings.Split(strings.ToLower(strings.TrimSpace(parts[1])), "x")
	if len(dimensions) != 2 {
		return Rendition{}, fmt.Errorf("rendition dimensions must use WIDTHxHEIGHT")
	}

	width, err := strconv.Atoi(dimensions[0])
	if err != nil || width <= 0 {
		return Rendition{}, fmt.Errorf("rendition width must be a positive integer")
	}

	height, err := strconv.Atoi(dimensions[1])
	if err != nil || height <= 0 {
		return Rendition{}, fmt.Errorf("rendition height must be a positive integer")
	}

	bitrate := strings.TrimSpace(parts[2])
	if bitrate == "" || strings.ContainsAny(bitrate, " \t\r\n") {
		return Rendition{}, fmt.Errorf("rendition bitrate must be non-empty and must not contain whitespace")
	}

	return Rendition{
		Name:    name,
		Width:   width,
		Height:  height,
		Bitrate: bitrate,
	}, nil
}

func parseRenditionName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || strings.ContainsAny(name, " \t\r\n:/") {
		return "", fmt.Errorf("rendition name must be non-empty and must not contain whitespace, ':' or '/'")
	}
	return name, nil
}

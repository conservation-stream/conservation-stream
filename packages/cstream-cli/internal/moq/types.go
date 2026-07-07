package moq

import (
	"fmt"
	"io"
	"net/url"
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
	RenditionSources      []RenditionSource
	CommandLogWriter      io.Writer
}

type Rendition struct {
	Name        string
	Width       int
	Height      int
	Bitrate     string
	Passthrough bool
}

type RenditionSource struct {
	Name    string
	RTSPURL string
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
	for index := range normalized.RenditionSources {
		normalized.RenditionSources[index].Name = strings.TrimSpace(normalized.RenditionSources[index].Name)
		normalized.RenditionSources[index].RTSPURL = strings.TrimSpace(normalized.RenditionSources[index].RTSPURL)
	}
	if normalized.ClientBind == "" {
		normalized.ClientBind = DefaultClientBind
	}
	if normalized.VideoCodec == "" {
		normalized.VideoCodec = DefaultVideoCodec
	}
	if normalized.RTSPSourceURL != "" && len(normalized.Renditions) == 0 && len(normalized.RenditionSources) == 0 {
		normalized.RenditionSources = DefaultAxisRenditionSources(normalized.RTSPSourceURL, normalized.VideoCodec)
		normalized.RTSPSourceURL = ""
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
	if cfg.RelayURL == "" {
		return fmt.Errorf("MoQ relay URL is required")
	}
	if cfg.Broadcast == "" {
		return fmt.Errorf("MoQ broadcast is required")
	}
	if !isSupportedVideoCodec(cfg.VideoCodec) {
		return fmt.Errorf("video codec must be h264 or h265")
	}
	if len(cfg.RenditionSources) > 0 {
		if cfg.RTSPSourceURL != "" {
			return fmt.Errorf("--in cannot be combined with --rendition-source")
		}
		if len(cfg.Renditions) > 0 {
			return fmt.Errorf("--rendition cannot be combined with --rendition-source")
		}
		for index, source := range cfg.RenditionSources {
			if source.Name == "" {
				return fmt.Errorf("rendition source %d name is required", index+1)
			}
			if source.RTSPURL == "" {
				return fmt.Errorf("rendition source %q RTSP URL is required", source.Name)
			}
		}
		return nil
	}
	if err := ValidateForwardConfig(cfg); err != nil {
		return err
	}
	if len(cfg.Renditions) == 0 {
		return fmt.Errorf("at least one --rendition or --rendition-source is required")
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

func ParseRenditionSource(raw string) (RenditionSource, error) {
	name, sourceURL, ok := strings.Cut(strings.TrimSpace(raw), "=")
	if !ok {
		return RenditionSource{}, fmt.Errorf("rendition source must use name=rtsp://...")
	}
	parsedName, err := parseRenditionName(name)
	if err != nil {
		return RenditionSource{}, err
	}
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return RenditionSource{}, fmt.Errorf("rendition source URL is required")
	}
	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		return RenditionSource{}, fmt.Errorf("rendition source URL is invalid: %w", err)
	}
	if parsedURL.Scheme != "rtsp" && parsedURL.Scheme != "rtsps" {
		return RenditionSource{}, fmt.Errorf("rendition source URL must use rtsp or rtsps")
	}
	return RenditionSource{Name: parsedName, RTSPURL: sourceURL}, nil
}

func DefaultAxisRenditionSources(rtspSourceURL string, videoCodec string) []RenditionSource {
	return []RenditionSource{
		{
			Name:    "low",
			RTSPURL: axisRenditionURL(rtspSourceURL, videoCodec, "640x360", "6", "800", "fullframerate"),
		},
		{
			Name:    "high",
			RTSPURL: axisRenditionURL(rtspSourceURL, videoCodec, "1280x720", "60", "6000", "quality"),
		},
	}
}

func axisRenditionURL(raw string, videoCodec string, resolution string, keyframeInterval string, maxBitrate string, bitratePriority string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}

	query := parsed.Query()
	query.Set("videoframeskipmode", "empty")
	query.Set("videozprofile", "classic")
	query.Set("videozgopmode", "fixed")
	query.Set("resolution", resolution)
	query.Set("fps", "30")
	query.Set("audio", "0")
	query.Set("timestamp", "0")
	query.Set("videocodec", strings.ToLower(strings.TrimSpace(videoCodec)))
	query.Set("videokeyframeinterval", keyframeInterval)
	query.Set("videobitratemode", "mbr")
	query.Set("videomaxbitrate", maxBitrate)
	query.Set("videobitratepriority", bitratePriority)

	if strings.EqualFold(videoCodec, "h264") {
		if resolution == "640x360" {
			query.Set("h264profile", "baseline")
		} else {
			query.Set("h264profile", "high")
		}
	} else {
		query.Del("h264profile")
	}

	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func parseRenditionName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || strings.ContainsAny(name, " \t\r\n:/") {
		return "", fmt.Errorf("rendition name must be non-empty and must not contain whitespace, ':' or '/'")
	}
	return name, nil
}

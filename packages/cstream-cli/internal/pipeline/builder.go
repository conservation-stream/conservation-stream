package pipeline

import (
	"fmt"
	"strings"
)

func buildLaunch(cfg Config) (string, error) {
	in, err := buildInputChain(cfg.In)
	if err != nil {
		return "", err
	}

	out, err := buildOutputChain(cfg.Out)
	if err != nil {
		return "", err
	}

	videoCore := make([]string, 0, 6)
	videoCore = append(videoCore,
		"videoconvert",
		"videorate",
		"queue max-size-time=250000000 max-size-buffers=0 max-size-bytes=0",
	)

	if originCaps := buildOriginFrameCaps(cfg.OriginFrameInfo); originCaps != "" {
		videoCore = append(videoCore, originCaps)
	}

	videoCore = append(videoCore,
		fmt.Sprintf(`x264enc name=encoder bitrate=%d cabac=0 bframes=0 ref=1 key-int-max=30 threads=0 sliced-threads=true speed-preset=%s option-string="vbv-maxrate=%d:vbv-bufsize=%d"`, DefaultEncoderBitrateKbps, cfg.EncoderSpeedPreset, DefaultEncoderBitrateKbps, DefaultEncoderBitrateKbps),
		"video/x-h264,profile=baseline",
		"h264parse name=parser",
	)

	parts := make([]string, 0, len(in)+len(videoCore)+len(out))
	parts = append(parts, in...)
	parts = append(parts, videoCore...)
	parts = append(parts, out...)

	return strings.Join(parts, " ! "), nil
}

func buildOriginFrameCaps(info OriginFrameInfo) string {
	if info.Width == "" || info.Height == "" || info.Rate == "" {
		return ""
	}

	return fmt.Sprintf("video/x-raw,format=I420,width=%s,height=%s,framerate=%s", info.Width, info.Height, info.Rate)
}

func buildInputChain(in Connection) ([]string, error) {
	switch in.Type {
	case ConnectionTypeRTSP:
		return []string{
			fmt.Sprintf(`rtspsrc location="%s" latency=0 drop-on-latency=true`, in.URL),
			"decodebin",
		}, nil
	case ConnectionTypeRTMP:
		return []string{
			fmt.Sprintf(`rtmpsrc location="%s"`, in.URL),
			"flvdemux name=demux0",
			"demux0.video ! queue",
			"h264parse",
			"avdec_h264",
		}, nil
	default:
		return nil, fmt.Errorf("%w: in=%s", ErrUnsupportedInputType, in.Type)
	}
}

func buildOutputChain(out Connection) ([]string, error) {
	switch out.Type {
	case ConnectionTypeRTMP:
		return []string{
			"flvmux name=flvmux0 streamable=true",
			fmt.Sprintf(`rtmpsink location="%s"`, out.URL),
		}, nil
	case ConnectionTypeRTSP:
		return []string{
			"identity name=h264tsfix single-segment=true",
			fmt.Sprintf(`rtspclientsink name=outsink location="%s" protocols=tcp`, out.URL),
		}, nil
	default:
		return nil, fmt.Errorf("%w: out=%s", ErrUnsupportedOutputType, out.Type)
	}
}

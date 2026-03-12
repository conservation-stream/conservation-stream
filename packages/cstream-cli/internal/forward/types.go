package forward

import (
	"fmt"
	"strings"
	"time"

	"github.com/pion/rtp"
)

const (
	DefaultPayloadMaxSize    = 1188
	DefaultChannelBufferSize = 512
	DefaultWHIPTimeout       = 10 * time.Second
)

var DefaultSTUNServers = []string{"stun:stun.l.google.com:19302"}

type Config struct {
	RTSPSourceURL     string
	WHIPPublishURL    string
	PayloadMaxSize    int
	ChannelBufferSize int
	WHIPTimeout       time.Duration
	STUNServers       []string
	Logger            Logger
}

type MediaUnit struct {
	IsVideo    bool
	Timestamp  uint32
	NTP        time.Time
	RTPPackets []*rtp.Packet
	H264NALUs  [][]byte
	OpusFrames [][]byte
}

type CodecProcessor interface {
	ProcessPacket(pkt *rtp.Packet, pts int64) (*MediaUnit, error)
}

func normalizeConfig(cfg Config) Config {
	normalized := cfg
	normalized.RTSPSourceURL = strings.TrimSpace(cfg.RTSPSourceURL)
	normalized.WHIPPublishURL = strings.TrimSpace(cfg.WHIPPublishURL)
	if normalized.PayloadMaxSize == 0 {
		normalized.PayloadMaxSize = DefaultPayloadMaxSize
	}
	if normalized.ChannelBufferSize == 0 {
		normalized.ChannelBufferSize = DefaultChannelBufferSize
	}
	if normalized.WHIPTimeout == 0 {
		normalized.WHIPTimeout = DefaultWHIPTimeout
	}
	if len(normalized.STUNServers) == 0 {
		normalized.STUNServers = append([]string(nil), DefaultSTUNServers...)
	}
	if normalized.Logger == nil {
		normalized.Logger = NewNoopLogger()
	}
	return normalized
}

func validateConfig(cfg Config) error {
	if cfg.RTSPSourceURL == "" {
		return fmt.Errorf("rtsp source URL is required")
	}
	if cfg.WHIPPublishURL == "" {
		return fmt.Errorf("WHIP publish URL is required")
	}
	return nil
}

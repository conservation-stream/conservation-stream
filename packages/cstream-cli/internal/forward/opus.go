package forward

import (
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

type OpusProcessor struct {
	format           *format.Opus
	currentTimestamp uint32
}

func NewOpusProcessor(streamFormat *format.Opus) (*OpusProcessor, error) {
	return &OpusProcessor{format: streamFormat}, nil
}

func (processor *OpusProcessor) ProcessPacket(pkt *rtp.Packet, pts int64) (*MediaUnit, error) {
	newPacket := &rtp.Packet{
		Header:  pkt.Header,
		Payload: pkt.Payload,
	}

	newPacket.Timestamp = processor.currentTimestamp
	processor.currentTimestamp += uint32(opusPacketDuration(pkt.Payload))

	return &MediaUnit{
		IsVideo:    false,
		Timestamp:  newPacket.Timestamp,
		NTP:        time.Now(),
		RTPPackets: []*rtp.Packet{newPacket},
		OpusFrames: [][]byte{pkt.Payload},
	}, nil
}

func opusPacketDuration(payload []byte) int {
	if len(payload) < 1 {
		return 960
	}

	toc := payload[0]
	frameSize := []int{480, 960, 1920, 2880}[(toc>>3)&0x3]

	switch toc & 0x3 {
	case 0:
		return frameSize
	case 3:
		if len(payload) < 2 {
			return frameSize
		}
		return frameSize * int(payload[1]&0x3F)
	default:
		return frameSize * 2
	}
}

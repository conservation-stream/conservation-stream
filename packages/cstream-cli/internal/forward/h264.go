package forward

import (
	"bytes"
	"fmt"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/format/rtph264"
	"github.com/pion/rtp"
)

type H264Processor struct {
	decoder      *rtph264.Decoder
	encoder      *rtph264.Encoder
	lastSPS      []byte
	lastPPS      []byte
	sentKeyframe bool
}

func NewH264Processor(streamFormat *format.H264, payloadMaxSize int) (*H264Processor, error) {
	decoder, err := streamFormat.CreateDecoder()
	if err != nil {
		return nil, fmt.Errorf("create H264 decoder: %w", err)
	}

	encoder := &rtph264.Encoder{
		PayloadType:    96,
		PayloadMaxSize: payloadMaxSize,
	}
	encoder.Init()

	return &H264Processor{
		decoder: decoder,
		encoder: encoder,
		lastSPS: streamFormat.SPS,
		lastPPS: streamFormat.PPS,
	}, nil
}

func (processor *H264Processor) ProcessPacket(pkt *rtp.Packet, pts int64) (*MediaUnit, error) {
	nalus, err := processor.decoder.Decode(pkt)
	if err != nil {
		return nil, err
	}

	nalus, isKeyframe := processor.remuxNALUs(nalus)
	if len(nalus) == 0 {
		return nil, fmt.Errorf("no H264 NALUs ready")
	}

	if !processor.sentKeyframe && !isKeyframe {
		return nil, fmt.Errorf("waiting for H264 keyframe")
	}

	rtpPackets, err := processor.encoder.Encode(nalus)
	if err != nil {
		return nil, fmt.Errorf("encode H264 RTP: %w", err)
	}

	for _, newPacket := range rtpPackets {
		newPacket.Timestamp = pkt.Timestamp
	}

	if isKeyframe {
		processor.sentKeyframe = true
	}

	return &MediaUnit{
		IsVideo:    true,
		Timestamp:  pkt.Timestamp,
		NTP:        time.Now(),
		RTPPackets: rtpPackets,
		H264NALUs:  nalus,
	}, nil
}

func (processor *H264Processor) remuxNALUs(nalus [][]byte) ([][]byte, bool) {
	isKeyframe := false
	filtered := make([][]byte, 0, len(nalus))

	for _, nalu := range nalus {
		if len(nalu) == 0 {
			continue
		}

		switch nalu[0] & 0x1F {
		case 7:
			if !bytes.Equal(processor.lastSPS, nalu) {
				processor.lastSPS = nalu
			}
		case 8:
			if !bytes.Equal(processor.lastPPS, nalu) {
				processor.lastPPS = nalu
			}
		case 9:
		case 5:
			isKeyframe = true
			filtered = append(filtered, nalu)
		default:
			filtered = append(filtered, nalu)
		}
	}

	if len(filtered) == 0 {
		return nil, false
	}

	if isKeyframe {
		if len(processor.lastSPS) == 0 || len(processor.lastPPS) == 0 {
			return nil, false
		}
		filtered = append([][]byte{processor.lastSPS, processor.lastPPS}, filtered...)
	}

	return filtered, isKeyframe
}

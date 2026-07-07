package forward

import (
	"context"
	"fmt"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

type Source interface {
	Open(context.Context) error
	HasVideo() bool
	HasAudio() bool
	Receive(context.Context, chan<- *MediaUnit) error
	Close() error
}

type RTSPSource struct {
	cfg           Config
	client        *gortsplib.Client
	session       *description.Session
	h264Processor *H264Processor
	opusProcessor *OpusProcessor
}

func NewRTSPSource(cfg Config) *RTSPSource {
	return &RTSPSource{cfg: cfg}
}

func (source *RTSPSource) Open(ctx context.Context) error {
	parsedURL, err := base.ParseURL(source.cfg.RTSPSourceURL)
	if err != nil {
		return fmt.Errorf("parse RTSP URL: %w", err)
	}

	source.client = &gortsplib.Client{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
	}

	if err := source.client.Start(); err != nil {
		return fmt.Errorf("start RTSP client: %w", err)
	}

	session, _, err := source.client.Describe(parsedURL)
	if err != nil {
		return fmt.Errorf("describe RTSP stream: %w", err)
	}
	source.session = session

	if err := source.client.SetupAll(session.BaseURL, session.Medias); err != nil {
		return fmt.Errorf("setup RTSP media: %w", err)
	}

	for _, media := range session.Medias {
		for _, streamFormat := range media.Formats {
			switch formatValue := streamFormat.(type) {
			case *format.H264:
				processor, err := NewH264Processor(formatValue, source.cfg.PayloadMaxSize)
				if err != nil {
					return err
				}
				source.h264Processor = processor
			case *format.Opus:
				processor, err := NewOpusProcessor(formatValue)
				if err != nil {
					return err
				}
				source.opusProcessor = processor
			}
		}
	}

	return nil
}

func (source *RTSPSource) HasVideo() bool {
	return source.h264Processor != nil
}

func (source *RTSPSource) HasAudio() bool {
	return source.opusProcessor != nil
}

func (source *RTSPSource) Receive(ctx context.Context, output chan<- *MediaUnit) error {
	for _, media := range source.session.Medias {
		for _, streamFormat := range media.Formats {
			source.registerPacketHandler(media, streamFormat, output)
		}
	}

	if _, err := source.client.Play(nil); err != nil {
		return fmt.Errorf("play RTSP stream: %w", err)
	}

	waitErrChannel := make(chan error, 1)
	go func() {
		waitErrChannel <- source.client.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-waitErrChannel:
		return err
	}
}

func (source *RTSPSource) registerPacketHandler(media *description.Media, streamFormat format.Format, output chan<- *MediaUnit) {
	switch streamFormat.(type) {
	case *format.H264:
		source.client.OnPacketRTP(media, streamFormat, func(pkt *rtp.Packet) {
			if source.h264Processor == nil {
				return
			}
			pts, _ := source.client.PacketPTS(media, pkt)
			unit, err := source.h264Processor.ProcessPacket(pkt, pts)
			if err == nil && unit != nil {
				sendUnit(output, unit)
			}
		})
	case *format.Opus:
		source.client.OnPacketRTP(media, streamFormat, func(pkt *rtp.Packet) {
			if source.opusProcessor == nil {
				return
			}
			pts, _ := source.client.PacketPTS(media, pkt)
			unit, err := source.opusProcessor.ProcessPacket(pkt, pts)
			if err == nil && unit != nil {
				sendUnit(output, unit)
			}
		})
	}
}

// sendUnit drops the unit when the channel is full instead of blocking. A
// blocking send here would deadlock shutdown: once the publish loop exits,
// nothing drains the channel, the RTP callback would block forever, and
// closing the RTSP client (and the whole process) would hang.
func sendUnit(output chan<- *MediaUnit, unit *MediaUnit) {
	select {
	case output <- unit:
	default:
	}
}

func (source *RTSPSource) Close() error {
	if source.client != nil {
		source.client.Close()
	}
	return nil
}

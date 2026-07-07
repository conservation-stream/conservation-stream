package forward

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	neturl "net/url"

	"github.com/pion/webrtc/v4"
)

type Publisher interface {
	Open(context.Context, bool, bool) error
	Publish(*MediaUnit) error
	// Failed reports asynchronous fatal publisher failures (e.g. the WebRTC
	// peer connection entering a failed state) that Publish cannot surface.
	Failed() <-chan error
	Close() error
}

type WHIPPublisher struct {
	cfg         Config
	peerConn    *webrtc.PeerConnection
	videoTrack  *webrtc.TrackLocalStaticRTP
	audioTrack  *webrtc.TrackLocalStaticRTP
	resourceURL string
	failed      chan error
}

func NewWHIPPublisher(cfg Config) *WHIPPublisher {
	return &WHIPPublisher{cfg: cfg, failed: make(chan error, 1)}
}

func (publisher *WHIPPublisher) Failed() <-chan error {
	return publisher.failed
}

func (publisher *WHIPPublisher) reportFailure(err error) {
	select {
	case publisher.failed <- err:
	default:
	}
}

func (publisher *WHIPPublisher) Open(ctx context.Context, hasVideo bool, hasAudio bool) error {
	mediaEngine := &webrtc.MediaEngine{}

	if hasVideo {
		if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:    webrtc.MimeTypeH264,
				ClockRate:   90000,
				SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			},
			PayloadType: 96,
		}, webrtc.RTPCodecTypeVideo); err != nil {
			return fmt.Errorf("register H264 codec: %w", err)
		}
	}

	if hasAudio {
		if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:    webrtc.MimeTypeOpus,
				ClockRate:   48000,
				Channels:    2,
				SDPFmtpLine: "minptime=10;useinbandfec=1;stereo=1;sprop-stereo=1",
			},
			PayloadType: 111,
		}, webrtc.RTPCodecTypeAudio); err != nil {
			return fmt.Errorf("register Opus codec: %w", err)
		}
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	peerConn, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: publisher.iceServers(),
	})
	if err != nil {
		return fmt.Errorf("create peer connection: %w", err)
	}
	publisher.peerConn = peerConn

	// pion's TrackLocalStaticRTP.WriteRTP keeps succeeding after the peer
	// connection dies, so a failed Cloudflare session would otherwise be
	// forwarded into silently forever.
	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			publisher.reportFailure(fmt.Errorf("WebRTC peer connection entered state %s", state))
		}
	})

	if hasVideo {
		videoTrack, err := webrtc.NewTrackLocalStaticRTP(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000},
			"video",
			"stream",
		)
		if err != nil {
			return fmt.Errorf("create video track: %w", err)
		}

		sender, err := peerConn.AddTrack(videoTrack)
		if err != nil {
			return fmt.Errorf("add video track: %w", err)
		}
		publisher.videoTrack = videoTrack
		go drainRTCP(sender)
	}

	if hasAudio {
		audioTrack, err := webrtc.NewTrackLocalStaticRTP(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2},
			"audio",
			"stream",
		)
		if err != nil {
			return fmt.Errorf("create audio track: %w", err)
		}

		sender, err := peerConn.AddTrack(audioTrack)
		if err != nil {
			return fmt.Errorf("add audio track: %w", err)
		}
		publisher.audioTrack = audioTrack
		go drainRTCP(sender)
	}

	offer, err := peerConn.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("create WHIP offer: %w", err)
	}

	if err := peerConn.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("set local description: %w", err)
	}

	<-webrtc.GatheringCompletePromise(peerConn)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, publisher.cfg.WHIPPublishURL, bytes.NewBufferString(peerConn.LocalDescription().SDP))
	if err != nil {
		return fmt.Errorf("create WHIP request: %w", err)
	}
	request.Header.Set("Content-Type", "application/sdp")

	httpClient := &http.Client{Timeout: publisher.cfg.WHIPTimeout}
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("publish WHIP offer: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(response.Body)
		return fmt.Errorf("WHIP server returned status %d: %s", response.StatusCode, body.String())
	}

	answerBody := new(bytes.Buffer)
	if _, err := answerBody.ReadFrom(response.Body); err != nil {
		return fmt.Errorf("read WHIP answer: %w", err)
	}

	if locationHeader := response.Header.Get("Location"); locationHeader != "" {
		publisher.resourceURL = resolveResourceURL(request.URL, locationHeader)
	}

	if err := peerConn.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerBody.String(),
	}); err != nil {
		return fmt.Errorf("set remote description: %w", err)
	}

	return nil
}

func (publisher *WHIPPublisher) Publish(unit *MediaUnit) error {
	var track *webrtc.TrackLocalStaticRTP
	if unit.IsVideo {
		track = publisher.videoTrack
	} else {
		track = publisher.audioTrack
	}
	if track == nil {
		return nil
	}

	for _, packet := range unit.RTPPackets {
		if err := track.WriteRTP(packet); err != nil {
			return fmt.Errorf("write RTP packet: %w", err)
		}
	}

	return nil
}

func (publisher *WHIPPublisher) Close() error {
	if publisher.resourceURL != "" {
		request, err := http.NewRequest(http.MethodDelete, publisher.resourceURL, nil)
		if err == nil {
			httpClient := &http.Client{Timeout: publisher.cfg.WHIPTimeout}
			response, doErr := httpClient.Do(request)
			if doErr == nil && response != nil {
				response.Body.Close()
			}
		}
	}

	if publisher.peerConn != nil {
		return publisher.peerConn.Close()
	}

	return nil
}

func (publisher *WHIPPublisher) iceServers() []webrtc.ICEServer {
	servers := make([]webrtc.ICEServer, 0, len(publisher.cfg.STUNServers))
	for _, stunServer := range publisher.cfg.STUNServers {
		servers = append(servers, webrtc.ICEServer{URLs: []string{stunServer}})
	}
	return servers
}

func resolveResourceURL(baseURL *neturl.URL, locationHeader string) string {
	locationURL, err := neturl.Parse(locationHeader)
	if err != nil {
		return locationHeader
	}
	return baseURL.ResolveReference(locationURL).String()
}

func drainRTCP(sender *webrtc.RTPSender) {
	buffer := make([]byte, 1500)
	for {
		if _, _, err := sender.Read(buffer); err != nil {
			return
		}
	}
}

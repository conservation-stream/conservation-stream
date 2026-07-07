package forward

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSource struct {
	receive func(ctx context.Context, output chan<- *MediaUnit) error
}

func (source *fakeSource) Open(ctx context.Context) error { return nil }
func (source *fakeSource) HasVideo() bool                 { return true }
func (source *fakeSource) HasAudio() bool                 { return false }
func (source *fakeSource) Close() error                   { return nil }

func (source *fakeSource) Receive(ctx context.Context, output chan<- *MediaUnit) error {
	if source.receive != nil {
		return source.receive(ctx, output)
	}
	<-ctx.Done()
	return ctx.Err()
}

type fakePublisher struct {
	publish func(*MediaUnit) error
	failed  chan error
	closed  chan struct{}
}

func newFakePublisher() *fakePublisher {
	return &fakePublisher{
		failed: make(chan error, 1),
		closed: make(chan struct{}),
	}
}

func (publisher *fakePublisher) Open(ctx context.Context, hasVideo bool, hasAudio bool) error {
	return nil
}

func (publisher *fakePublisher) Publish(unit *MediaUnit) error {
	if publisher.publish != nil {
		return publisher.publish(unit)
	}
	return nil
}

func (publisher *fakePublisher) Failed() <-chan error { return publisher.failed }

func (publisher *fakePublisher) Close() error {
	close(publisher.closed)
	return nil
}

func newTestRunner(source Source, publisher Publisher, stallTimeout time.Duration) *WHIPRunner {
	cfg := normalizeConfig(Config{
		RTSPSourceURL:      "rtsp://in.local/live",
		WHIPPublishURL:     "https://whip.example.com/endpoint",
		SourceStallTimeout: stallTimeout,
	})
	return &WHIPRunner{
		cfg:       cfg,
		source:    source,
		publisher: publisher,
		units:     make(chan *MediaUnit, cfg.ChannelBufferSize),
		logger:    cfg.Logger,
	}
}

func runWithTimeout(t *testing.T, runner Runner) error {
	t.Helper()

	errChannel := make(chan error, 1)
	go func() {
		errChannel <- runner.Run(context.Background())
	}()

	select {
	case err := <-errChannel:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not exit within timeout")
		return nil
	}
}

func TestRunnerExitsOnReceiveError(t *testing.T) {
	receiveErr := errors.New("rtsp connection reset")
	source := &fakeSource{
		receive: func(ctx context.Context, output chan<- *MediaUnit) error {
			return receiveErr
		},
	}
	publisher := newFakePublisher()

	err := runWithTimeout(t, newTestRunner(source, publisher, time.Minute))
	if !errors.Is(err, receiveErr) {
		t.Fatalf("expected receive error, got: %v", err)
	}

	select {
	case <-publisher.closed:
	default:
		t.Fatal("expected publisher to be closed so the WHIP session is deleted")
	}
}

func TestRunnerExitsWhenSourceStalls(t *testing.T) {
	source := &fakeSource{}
	publisher := newFakePublisher()

	err := runWithTimeout(t, newTestRunner(source, publisher, 50*time.Millisecond))
	if err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("expected stall error, got: %v", err)
	}

	select {
	case <-publisher.closed:
	default:
		t.Fatal("expected publisher to be closed so the WHIP session is deleted")
	}
}

func TestRunnerExitsOnPublishError(t *testing.T) {
	publishErr := errors.New("write RTP packet: broken pipe")
	source := &fakeSource{}
	publisher := newFakePublisher()
	publisher.publish = func(unit *MediaUnit) error { return publishErr }

	runner := newTestRunner(source, publisher, time.Minute)
	runner.units <- &MediaUnit{IsVideo: true}

	err := runWithTimeout(t, runner)
	if !errors.Is(err, publishErr) {
		t.Fatalf("expected publish error, got: %v", err)
	}
}

func TestRunnerExitsOnPublisherFailure(t *testing.T) {
	source := &fakeSource{}
	publisher := newFakePublisher()
	connErr := errors.New("WebRTC peer connection entered state failed")
	publisher.failed <- connErr

	err := runWithTimeout(t, newTestRunner(source, publisher, time.Minute))
	if !errors.Is(err, connErr) {
		t.Fatalf("expected peer connection failure error, got: %v", err)
	}
}

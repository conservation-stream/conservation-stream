package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
)

func TestNormalizeConfigNormalizesConnectionTypes(t *testing.T) {
	cfg := Config{
		In:  Connection{Type: "RTSP", URL: "rtsp://in.local/live"},
		Out: Connection{Type: "RTMP", URL: "rtmp://out.local/live"},
	}

	got, err := normalizeConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if got.In.Type != ConnectionTypeRTSP {
		t.Fatalf("expected in type %q, got %q", ConnectionTypeRTSP, got.In.Type)
	}
	if got.Out.Type != ConnectionTypeRTMP {
		t.Fatalf("expected out type %q, got %q", ConnectionTypeRTMP, got.Out.Type)
	}
}

func TestNormalizeConfigNormalizesOriginFrameInfo(t *testing.T) {
	cfg := Config{
		In:                 Connection{Type: "RTSP", URL: "rtsp://in.local/live"},
		Out:                Connection{Type: "RTMP", URL: "rtmp://out.local/live"},
		EncoderSpeedPreset: " fast ",
		OriginFrameInfo: OriginFrameInfo{
			Height: " 720 ",
			Width:  " 1280 ",
			Rate:   " 30/1 ",
		},
	}

	got, err := normalizeConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if got.OriginFrameInfo.Height != "720" || got.OriginFrameInfo.Width != "1280" || got.OriginFrameInfo.Rate != "30/1" {
		t.Fatalf("expected trimmed origin frame info, got %+v", got.OriginFrameInfo)
	}
	if got.EncoderSpeedPreset != "fast" {
		t.Fatalf("expected trimmed encoder speed preset, got %q", got.EncoderSpeedPreset)
	}
}

func TestNormalizeConfigDefaultsEncoderSpeedPreset(t *testing.T) {
	cfg := Config{
		In:  Connection{Type: "RTSP", URL: "rtsp://in.local/live"},
		Out: Connection{Type: "RTMP", URL: "rtmp://out.local/live"},
	}

	got, err := normalizeConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if got.EncoderSpeedPreset != "veryfast" {
		t.Fatalf("expected default encoder speed preset %q, got %q", "veryfast", got.EncoderSpeedPreset)
	}
}

func TestNormalizeConfigRejectsPartialOriginFrameInfo(t *testing.T) {
	cfg := Config{
		In:  Connection{Type: "RTSP", URL: "rtsp://in.local/live"},
		Out: Connection{Type: "RTMP", URL: "rtmp://out.local/live"},
		OriginFrameInfo: OriginFrameInfo{
			Height: "720",
			Width:  "1280",
		},
	}

	_, err := normalizeConfig(cfg)
	if err == nil {
		t.Fatal("expected error for partial origin frame info")
	}
}

func TestNormalizeConfigRejectsInvalidSchemeForType(t *testing.T) {
	cfg := Config{
		In:  Connection{Type: "rtsp", URL: "http://in.local/live"},
		Out: Connection{Type: "rtmp", URL: "rtmp://out.local/live"},
	}

	_, err := normalizeConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid input scheme")
	}
	if !errors.Is(err, ErrUnsupportedScheme) {
		t.Fatalf("expected ErrUnsupportedScheme, got: %v", err)
	}
}

func TestParseFrameRateSupportsFractionalRates(t *testing.T) {
	got, err := parseFrameRate("30000/1001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if got < 29.96 || got > 29.98 {
		t.Fatalf("expected NTSC-like frame rate, got %f", got)
	}
}

func TestParseFrameRateRejectsZeroDenominator(t *testing.T) {
	_, err := parseFrameRate("30/0")
	if err == nil {
		t.Fatal("expected error for zero denominator")
	}
}

func TestBuildLaunchIncludesNamedEncoderAndParser(t *testing.T) {
	cfg := Config{
		In:                 Connection{Type: ConnectionTypeRTSP, URL: "rtsp://in.local/live"},
		Out:                Connection{Type: ConnectionTypeRTMP, URL: "rtmp://out.local/live"},
		EncoderSpeedPreset: "fast",
	}

	launch, err := buildLaunch(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(launch, "x264enc name=encoder") {
		t.Fatalf("expected launch to include encoder element, got: %s", launch)
	}
	if !strings.Contains(launch, "h264parse name=parser") {
		t.Fatalf("expected launch to include parser element, got: %s", launch)
	}
	if !strings.Contains(launch, `x264enc name=encoder bitrate=4500`) {
		t.Fatalf("expected launch to include tuned encoder bitrate, got: %s", launch)
	}
	if !strings.Contains(launch, "speed-preset=fast") {
		t.Fatalf("expected launch to include fast encoder preset, got: %s", launch)
	}
	if !strings.Contains(launch, "key-int-max=30") {
		t.Fatalf("expected launch to include shorter GOP, got: %s", launch)
	}
	if !strings.Contains(launch, "threads=0 sliced-threads=true") {
		t.Fatalf("expected launch to include encoder threading hints, got: %s", launch)
	}
}

func TestBuildLaunchForRTSPIncludesLowLatencyOutputSettings(t *testing.T) {
	cfg := Config{
		In:                 Connection{Type: ConnectionTypeRTSP, URL: "rtsp://in.local/live"},
		Out:                Connection{Type: ConnectionTypeRTSP, URL: "rtsp://out.local/live"},
		EncoderSpeedPreset: "fast",
		OriginFrameInfo: OriginFrameInfo{
			Height: "720",
			Width:  "1280",
			Rate:   "30/1",
		},
	}

	launch, err := buildLaunch(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(launch, "latency=0") {
		t.Fatalf("expected RTSP input launch to set latency=0, got: %s", launch)
	}
	if !strings.Contains(launch, "video/x-h264,profile=baseline") {
		t.Fatalf("expected launch to constrain H264 profile, got: %s", launch)
	}
	if !strings.Contains(launch, "videorate") {
		t.Fatalf("expected launch to include videorate before raw caps, got: %s", launch)
	}
	if !strings.Contains(launch, "video/x-raw,format=I420,width=1280,height=720,framerate=30/1") {
		t.Fatalf("expected launch to include origin frame caps, got: %s", launch)
	}
	if !strings.Contains(launch, "rtspclientsink name=outsink") {
		t.Fatalf("expected launch to name the RTSP sink, got: %s", launch)
	}
	if !strings.Contains(launch, "rtspclientsink name=outsink location=\"rtsp://out.local/live\" protocols=tcp") {
		t.Fatalf("expected RTSP output launch to force tcp transport, got: %s", launch)
	}
}

func TestRunCallsOnReadyBeforeOnRunning(t *testing.T) {
	initGStreamer()

	gstPipeline, err := gst.NewPipeline("hook-order")
	if err != nil {
		t.Fatalf("create gst pipeline: %v", err)
	}

	var mutex sync.Mutex
	order := make([]string, 0, 2)
	runningStarted := make(chan struct{})
	runResult := make(chan error, 1)

	pipeline := &Pipeline{
		cfg: Config{
			OnReady: func(_ *gst.Pipeline) error {
				mutex.Lock()
				order = append(order, "ready")
				mutex.Unlock()
				return nil
			},
			OnRunning: func(ctx context.Context, _ *gst.Pipeline) error {
				mutex.Lock()
				order = append(order, "running")
				mutex.Unlock()
				close(runningStarted)
				<-ctx.Done()
				return ctx.Err()
			},
		},
		gstPipeline: gstPipeline,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		runResult <- pipeline.Run(ctx)
	}()

	select {
	case <-runningStarted:
		cancel()
	case <-time.After(2 * time.Second):
		t.Fatal("on running hook did not start")
	}

	select {
	case err := <-runResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return")
	}

	mutex.Lock()
	defer mutex.Unlock()
	if len(order) != 2 {
		t.Fatalf("expected 2 hook entries, got %d", len(order))
	}
	if order[0] != "ready" || order[1] != "running" {
		t.Fatalf("expected hook order [ready running], got %v", order)
	}
}

func TestRunReturnsOnRunningError(t *testing.T) {
	initGStreamer()

	gstPipeline, err := gst.NewPipeline("runtime-error")
	if err != nil {
		t.Fatalf("create gst pipeline: %v", err)
	}

	wantErr := errors.New("runtime hook failed")
	pipeline := &Pipeline{
		cfg: Config{
			OnRunning: func(_ context.Context, _ *gst.Pipeline) error {
				return wantErr
			},
		},
		gstPipeline: gstPipeline,
	}

	err = pipeline.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected runtime error %v, got: %v", wantErr, err)
	}
}

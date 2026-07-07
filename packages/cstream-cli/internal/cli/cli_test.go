package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"cstream-cli/internal/forward"
	"cstream-cli/internal/moq"
	"cstream-cli/internal/pipeline"
)

var publishFrameFlags = []string{"--height", "720", "--width", "1280", "--rate", "30/1"}
var publishDynamicFlags = append([]string{"--dynamic", "wss://api.example.com/config", "--base-bitrate", "100"}, publishFrameFlags...)

type stubPublishPipeline struct {
	run func(ctx context.Context) error
}

type stubForwardRunner struct {
	run func(ctx context.Context) error
}

type stubMoQRunner struct {
	run func(ctx context.Context) error
}

func (pipeline stubPublishPipeline) Run(ctx context.Context) error {
	if pipeline.run == nil {
		return nil
	}

	return pipeline.run(ctx)
}

func (runner stubForwardRunner) Run(ctx context.Context) error {
	if runner.run == nil {
		return nil
	}

	return runner.run(ctx)
}

func (runner stubMoQRunner) Run(ctx context.Context) error {
	if runner.run == nil {
		return nil
	}

	return runner.run(ctx)
}

func withPublishPipelineStub(t *testing.T, stub func(cfg pipeline.Config) (publishPipeline, error)) {
	t.Helper()

	original := newPublishPipeline
	newPublishPipeline = stub
	t.Cleanup(func() {
		newPublishPipeline = original
	})
}

func withForwardRunnerStub(t *testing.T, stub func(cfg forward.Config) (forwardRunner, error)) {
	t.Helper()

	original := newForwardRunner
	newForwardRunner = stub
	t.Cleanup(func() {
		newForwardRunner = original
	})
}

func withMoQForwardRunnerStub(t *testing.T, stub func(cfg moq.Config) (moqRunner, error)) {
	t.Helper()

	original := newMoQForwardRunner
	newMoQForwardRunner = stub
	t.Cleanup(func() {
		newMoQForwardRunner = original
	})
}

func withMoQPublishRunnerStub(t *testing.T, stub func(cfg moq.Config) (moqRunner, error)) {
	t.Helper()

	original := newMoQPublishRunner
	newMoQPublishRunner = stub
	t.Cleanup(func() {
		newMoQPublishRunner = original
	})
}

func TestPublishWithDynamicPasses(t *testing.T) {
	withPublishPipelineStub(t, func(cfg pipeline.Config) (publishPipeline, error) {
		return stubPublishPipeline{}, nil
	})

	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live"}, publishDynamicFlags...)
	err := run(args, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPublishWithPresetPasses(t *testing.T) {
	withPublishPipelineStub(t, func(cfg pipeline.Config) (publishPipeline, error) {
		return stubPublishPipeline{}, nil
	})

	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtmp://in.local/live", "--out", "rtsp://out.local/live", "--preset", "twitch"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPublishBuildsAndRunsPipeline(t *testing.T) {
	var gotConfig pipeline.Config
	var runCalled bool

	withPublishPipelineStub(t, func(cfg pipeline.Config) (publishPipeline, error) {
		gotConfig = cfg
		return stubPublishPipeline{
			run: func(ctx context.Context) error {
				runCalled = true
				return nil
			},
		}, nil
	})

	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsps://in.local/live", "--out", "rtmps://out.local/live", "--preset", "youtube"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !runCalled {
		t.Fatal("expected publish pipeline to run")
	}

	if gotConfig.In.Type != pipeline.ConnectionTypeRTSP {
		t.Fatalf("expected input type %q, got %q", pipeline.ConnectionTypeRTSP, gotConfig.In.Type)
	}
	if gotConfig.In.URL != "rtsps://in.local/live" {
		t.Fatalf("expected input URL to be preserved, got %q", gotConfig.In.URL)
	}
	if gotConfig.Out.Type != pipeline.ConnectionTypeRTMP {
		t.Fatalf("expected output type %q, got %q", pipeline.ConnectionTypeRTMP, gotConfig.Out.Type)
	}
	if gotConfig.Out.URL != "rtmps://out.local/live" {
		t.Fatalf("expected output URL to be preserved, got %q", gotConfig.Out.URL)
	}
	if gotConfig.OriginFrameInfo.Height != "720" || gotConfig.OriginFrameInfo.Width != "1280" || gotConfig.OriginFrameInfo.Rate != "30/1" {
		t.Fatalf("expected origin frame info to be preserved, got %+v", gotConfig.OriginFrameInfo)
	}
	if gotConfig.RuntimeFixes.EnforceMonotonicH264PTS {
		t.Fatal("expected RTMP output to leave runtime fixes disabled")
	}
	if gotConfig.RuntimeFixes.DropRestartEventsAfterFirst {
		t.Fatal("expected RTMP output to leave runtime fixes disabled")
	}
	if gotConfig.OnRunning != nil {
		t.Fatal("expected preset mode to leave OnRunning unset")
	}
	if gotConfig.Debug {
		t.Fatal("expected publish debug to be disabled by default")
	}
}

func TestPublishEnablesRuntimeFixesForRTSPOutput(t *testing.T) {
	var gotConfig pipeline.Config

	withPublishPipelineStub(t, func(cfg pipeline.Config) (publishPipeline, error) {
		gotConfig = cfg
		return stubPublishPipeline{}, nil
	})

	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtsp://out.local/live", "--preset", "youtube"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !gotConfig.RuntimeFixes.EnforceMonotonicH264PTS {
		t.Fatal("expected RTSP output to enable monotonic PTS fix")
	}
	if !gotConfig.RuntimeFixes.DropRestartEventsAfterFirst {
		t.Fatal("expected RTSP output to enable restart-event drop fix")
	}
}

func TestPublishEnablesDebugMode(t *testing.T) {
	var gotConfig pipeline.Config

	withPublishPipelineStub(t, func(cfg pipeline.Config) (publishPipeline, error) {
		gotConfig = cfg
		return stubPublishPipeline{}, nil
	})

	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtsp://out.local/live", "--preset", "youtube", "--debug"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !gotConfig.Debug {
		t.Fatal("expected publish debug to be enabled")
	}
	if gotConfig.LogWriter == nil {
		t.Fatal("expected publish debug to carry a log writer")
	}
}

func TestPublishRequiresDynamicOrPreset(t *testing.T) {
	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when dynamic/preset missing")
	}
}

func TestPublishRejectsBothDynamicAndPreset(t *testing.T) {
	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--dynamic", "wss://api.example.com/config", "--preset", "youtube", "--base-bitrate", "100"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when both dynamic and preset set")
	}
}

func TestPublishRejectsInvalidPreset(t *testing.T) {
	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--preset", "vimeo"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for invalid preset")
	}
}

func TestPublishRequiresOriginFrameFlags(t *testing.T) {
	err := run([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--preset", "youtube"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when origin frame flags are missing")
	}
}

func TestPublishDynamicRequiresBaseBitrate(t *testing.T) {
	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--dynamic", "wss://api.example.com/config"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when --base-bitrate is missing with --dynamic")
	}
}

func TestPublishRejectsBaseBitrateWithoutDynamic(t *testing.T) {
	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--preset", "youtube", "--base-bitrate", "100"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when --base-bitrate is used without --dynamic")
	}
}

func TestPublishDynamicRejectsNonWebsocketURL(t *testing.T) {
	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--dynamic", "https://api.example.com/config", "--base-bitrate", "100"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when --dynamic uses non-websocket URL")
	}
}

func TestPublishDynamicSetsOnRunning(t *testing.T) {
	var gotConfig pipeline.Config

	withPublishPipelineStub(t, func(cfg pipeline.Config) (publishPipeline, error) {
		gotConfig = cfg
		return stubPublishPipeline{}, nil
	})

	args := append([]string{"cstream", "rtsp", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live"}, publishDynamicFlags...)
	err := run(args, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if gotConfig.OnRunning == nil {
		t.Fatal("expected dynamic mode to set OnRunning hook")
	}
}

func TestWebRTCForwardPasses(t *testing.T) {
	withForwardRunnerStub(t, func(cfg forward.Config) (forwardRunner, error) {
		return stubForwardRunner{}, nil
	})

	err := run([]string{"cstream", "webrtc", "forward", "--in", "rtsp://in.local/live", "--out", "https://whip.example.com/endpoint"}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestWebRTCForwardBuildsAndRunsRunner(t *testing.T) {
	var gotConfig forward.Config
	var runCalled bool

	withForwardRunnerStub(t, func(cfg forward.Config) (forwardRunner, error) {
		gotConfig = cfg
		return stubForwardRunner{
			run: func(ctx context.Context) error {
				runCalled = true
				return nil
			},
		}, nil
	})

	err := run([]string{"cstream", "webrtc", "forward", "--in", "rtsp://in.local/live", "--out", "https://whip.example.com/endpoint"}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !runCalled {
		t.Fatal("expected forward runner to run")
	}

	if gotConfig.RTSPSourceURL != "rtsp://in.local/live" {
		t.Fatalf("expected RTSP source URL to be preserved, got %q", gotConfig.RTSPSourceURL)
	}
	if gotConfig.WHIPPublishURL != "https://whip.example.com/endpoint" {
		t.Fatalf("expected WHIP publish URL to be preserved, got %q", gotConfig.WHIPPublishURL)
	}
	if gotConfig.Logger == nil {
		t.Fatal("expected forward config to include a logger")
	}
	if got := fmt.Sprintf("%T", gotConfig.Logger); got != "forward.noopLogger" {
		t.Fatalf("expected default forward logger to be noop, got %s", got)
	}
}

func TestMoQForwardBuildsAndRunsRunner(t *testing.T) {
	var gotConfig moq.Config
	var runCalled bool

	withMoQForwardRunnerStub(t, func(cfg moq.Config) (moqRunner, error) {
		gotConfig = cfg
		return stubMoQRunner{
			run: func(ctx context.Context) error {
				runCalled = true
				return nil
			},
		}, nil
	})

	err := run([]string{
		"cstream",
		"moq",
		"forward",
		"--in",
		"rtsp://in.local/live",
		"--out",
		"https://cdn.moq.dev/anon?jwt=abc#fragment",
		"--broadcast",
		"my-stream.hang",
		"--moq-client-bind",
		"0.0.0.0:0",
		"--moq-tls-disable-verify",
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !runCalled {
		t.Fatal("expected MoQ forward runner to run")
	}

	if gotConfig.RTSPSourceURL != "rtsp://in.local/live" {
		t.Fatalf("expected RTSP source URL to be preserved, got %q", gotConfig.RTSPSourceURL)
	}
	if gotConfig.RelayURL != "https://cdn.moq.dev/anon?jwt=abc" {
		t.Fatalf("expected MoQ relay URL to preserve path and query, got %q", gotConfig.RelayURL)
	}
	if gotConfig.Broadcast != "my-stream.hang" {
		t.Fatalf("expected MoQ broadcast to be preserved, got %q", gotConfig.Broadcast)
	}
	if gotConfig.ClientBind != "0.0.0.0:0" {
		t.Fatalf("expected MoQ client bind to be preserved, got %q", gotConfig.ClientBind)
	}
	if !gotConfig.TLSDisableVerify {
		t.Fatal("expected MoQ TLS verification to be disabled")
	}
}

func TestMoQPublishBuildsRenditionConfig(t *testing.T) {
	var gotConfig moq.Config
	var runCalled bool

	withMoQPublishRunnerStub(t, func(cfg moq.Config) (moqRunner, error) {
		gotConfig = cfg
		return stubMoQRunner{
			run: func(ctx context.Context) error {
				runCalled = true
				return nil
			},
		}, nil
	})

	err := run([]string{
		"cstream",
		"moq",
		"publish",
		"--in",
		"rtsp://in.local/live",
		"--out",
		"https://cdn.moq.dev/anon",
		"--broadcast",
		"my-stream.hang",
		"--rendition",
		"720p:1280x720:2500k",
		"--rendition",
		"passthrough",
		"--rendition",
		"360p:640x360:800k",
		"--video-codec",
		"h265",
		"--catalog-control",
		"/tmp/catalog.json",
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !runCalled {
		t.Fatal("expected MoQ publish runner to run")
	}
	if len(gotConfig.Renditions) != 3 {
		t.Fatalf("expected three renditions, got %+v", gotConfig.Renditions)
	}
	if gotConfig.Renditions[0] != (moq.Rendition{Name: "720p", Width: 1280, Height: 720, Bitrate: "2500k"}) {
		t.Fatalf("unexpected first rendition: %+v", gotConfig.Renditions[0])
	}
	if gotConfig.Renditions[1] != (moq.Rendition{Name: "passthrough", Passthrough: true}) {
		t.Fatalf("unexpected second rendition: %+v", gotConfig.Renditions[1])
	}
	if gotConfig.Renditions[2] != (moq.Rendition{Name: "360p", Width: 640, Height: 360, Bitrate: "800k"}) {
		t.Fatalf("unexpected third rendition: %+v", gotConfig.Renditions[2])
	}
	if gotConfig.CatalogControl != "/tmp/catalog.json" {
		t.Fatalf("unexpected catalog control path: %q", gotConfig.CatalogControl)
	}
	if gotConfig.VideoCodec != "h265" {
		t.Fatalf("unexpected video codec: %q", gotConfig.VideoCodec)
	}
}

func TestMoQPublishBuildsRenditionSourceConfig(t *testing.T) {
	var gotConfig moq.Config
	var runCalled bool

	withMoQPublishRunnerStub(t, func(cfg moq.Config) (moqRunner, error) {
		gotConfig = cfg
		return stubMoQRunner{
			run: func(ctx context.Context) error {
				runCalled = true
				return nil
			},
		}, nil
	})

	err := run([]string{
		"cstream",
		"moq",
		"publish",
		"--out",
		"https://cdn.moq.dev/anon",
		"--broadcast",
		"my-stream.hang",
		"--rendition-source",
		"720p=rtsp://camera.local/high",
		"--rendition-source",
		"360p=rtsp://camera.local/low?token=a=b",
		"--catalog-control",
		"/tmp/catalog.json",
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !runCalled {
		t.Fatal("expected MoQ publish runner to run")
	}
	if gotConfig.RTSPSourceURL != "" {
		t.Fatalf("expected no shared RTSP source, got %q", gotConfig.RTSPSourceURL)
	}
	if len(gotConfig.RenditionSources) != 2 {
		t.Fatalf("expected two rendition sources, got %+v", gotConfig.RenditionSources)
	}
	if gotConfig.RenditionSources[0] != (moq.RenditionSource{Name: "720p", RTSPURL: "rtsp://camera.local/high"}) {
		t.Fatalf("unexpected first rendition source: %+v", gotConfig.RenditionSources[0])
	}
	if gotConfig.RenditionSources[1] != (moq.RenditionSource{Name: "360p", RTSPURL: "rtsp://camera.local/low?token=a=b"}) {
		t.Fatalf("unexpected second rendition source: %+v", gotConfig.RenditionSources[1])
	}
	if gotConfig.CatalogControl != "/tmp/catalog.json" {
		t.Fatalf("unexpected catalog control path: %q", gotConfig.CatalogControl)
	}
}

func TestMoQPublishBuildsDefaultAxisRenditionSources(t *testing.T) {
	var gotConfig moq.Config

	withMoQPublishRunnerStub(t, func(cfg moq.Config) (moqRunner, error) {
		gotConfig = moq.NormalizeConfig(cfg)
		return stubMoQRunner{}, nil
	})

	err := run([]string{
		"cstream",
		"moq",
		"publish",
		"--in",
		"rtsp://camera.local/axis-media/media.amp?camera=1",
		"--out",
		"https://cdn.moq.dev/anon",
		"--broadcast",
		"my-stream.hang",
		"--video-codec",
		"h265",
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if gotConfig.RTSPSourceURL != "" {
		t.Fatalf("expected generated sources to consume shared input, got %q", gotConfig.RTSPSourceURL)
	}
	if len(gotConfig.RenditionSources) != 2 {
		t.Fatalf("expected two generated rendition sources, got %+v", gotConfig.RenditionSources)
	}
	if gotConfig.RenditionSources[0].Name != "low" || !strings.Contains(gotConfig.RenditionSources[0].RTSPURL, "videocodec=h265") {
		t.Fatalf("unexpected low source: %+v", gotConfig.RenditionSources[0])
	}
	if gotConfig.RenditionSources[1].Name != "high" || !strings.Contains(gotConfig.RenditionSources[1].RTSPURL, "videokeyframeinterval=60") {
		t.Fatalf("unexpected high source: %+v", gotConfig.RenditionSources[1])
	}
}

func TestMoQPublishBuildsDynamicCatalogControlConfig(t *testing.T) {
	var gotConfig moq.Config

	withMoQPublishRunnerStub(t, func(cfg moq.Config) (moqRunner, error) {
		gotConfig = cfg
		return stubMoQRunner{}, nil
	})

	err := run([]string{
		"cstream",
		"moq",
		"publish",
		"--in",
		"rtsp://in.local/live",
		"--out",
		"https://cdn.moq.dev/anon",
		"--broadcast",
		"my-stream.hang",
		"--rendition",
		"360p:640x360:800k",
		"--catalog-control",
		"/tmp/catalog.json",
		"--catalog-control-dynamic",
		"wss://api.example.com/catalog",
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if gotConfig.CatalogControl != "/tmp/catalog.json" {
		t.Fatalf("unexpected catalog control path: %q", gotConfig.CatalogControl)
	}
	if gotConfig.CatalogControlDynamic != "wss://api.example.com/catalog" {
		t.Fatalf("unexpected dynamic catalog control URL: %q", gotConfig.CatalogControlDynamic)
	}
}

func TestMoQPublishRejectsInvalidDynamicCatalogControlURL(t *testing.T) {
	err := run([]string{
		"cstream",
		"moq",
		"publish",
		"--in",
		"rtsp://in.local/live",
		"--out",
		"https://cdn.moq.dev/anon",
		"--broadcast",
		"my-stream.hang",
		"--rendition",
		"360p:640x360:800k",
		"--catalog-control-dynamic",
		"https://api.example.com/catalog",
	}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected invalid dynamic catalog control URL to fail")
	}
}

func TestWebRTCForwardEnablesDebugLogger(t *testing.T) {
	var gotConfig forward.Config

	withForwardRunnerStub(t, func(cfg forward.Config) (forwardRunner, error) {
		gotConfig = cfg
		return stubForwardRunner{}, nil
	})

	err := run([]string{"cstream", "webrtc", "forward", "--in", "rtsp://in.local/live", "--out", "https://whip.example.com/endpoint", "--debug"}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if gotConfig.Logger == nil {
		t.Fatal("expected forward config to include a logger")
	}
	if got := fmt.Sprintf("%T", gotConfig.Logger); got != "*forward.stdLogger" {
		t.Fatalf("expected debug forward logger to be std logger, got %s", got)
	}
}

func TestMoQForwardRejectsInvalidBroadcast(t *testing.T) {
	err := run([]string{"cstream", "moq", "forward", "--in", "rtsp://in.local/live", "--out", "https://cdn.moq.dev/anon", "--broadcast", "two words"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for invalid MoQ broadcast")
	}
}

func TestMoQPublishRequiresInputOrRenditions(t *testing.T) {
	err := run([]string{"cstream", "moq", "publish", "--out", "https://cdn.moq.dev/anon", "--broadcast", "stream.hang"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when MoQ publish has no input or renditions")
	}
}

func TestWebRTCForwardRejectsNonRTSPInput(t *testing.T) {
	err := run([]string{"cstream", "webrtc", "forward", "--in", "rtmp://in.local/live", "--out", "https://whip.example.com/endpoint"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for non-rtsp input")
	}
}

func TestWebRTCForwardRejectsNonHTTPSOutput(t *testing.T) {
	err := run([]string{"cstream", "webrtc", "forward", "--in", "rtsp://in.local/live", "--out", "http://whip.example.com/endpoint"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for non-https output")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := run([]string{"cstream", "wat"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

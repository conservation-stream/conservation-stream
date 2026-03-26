package cli

import (
	"context"
	"fmt"
	"io"
	"testing"

	"cstream-cli/internal/forward"
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

func TestPublishWithDynamicPasses(t *testing.T) {
	withPublishPipelineStub(t, func(cfg pipeline.Config) (publishPipeline, error) {
		return stubPublishPipeline{}, nil
	})

	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live"}, publishDynamicFlags...)
	err := run(args, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPublishWithPresetPasses(t *testing.T) {
	withPublishPipelineStub(t, func(cfg pipeline.Config) (publishPipeline, error) {
		return stubPublishPipeline{}, nil
	})

	args := append([]string{"cstream", "publish", "--in", "rtmp://in.local/live", "--out", "rtsp://out.local/live", "--preset", "twitch"}, publishFrameFlags...)
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

	args := append([]string{"cstream", "publish", "--in", "rtsps://in.local/live", "--out", "rtmps://out.local/live", "--preset", "youtube"}, publishFrameFlags...)
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

	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtsp://out.local/live", "--preset", "youtube"}, publishFrameFlags...)
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

	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtsp://out.local/live", "--preset", "youtube", "--debug"}, publishFrameFlags...)
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
	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when dynamic/preset missing")
	}
}

func TestPublishRejectsBothDynamicAndPreset(t *testing.T) {
	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--dynamic", "wss://api.example.com/config", "--preset", "youtube", "--base-bitrate", "100"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when both dynamic and preset set")
	}
}

func TestPublishRejectsInvalidPreset(t *testing.T) {
	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--preset", "vimeo"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for invalid preset")
	}
}

func TestPublishRequiresOriginFrameFlags(t *testing.T) {
	err := run([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--preset", "youtube"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when origin frame flags are missing")
	}
}

func TestPublishDynamicRequiresBaseBitrate(t *testing.T) {
	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--dynamic", "wss://api.example.com/config"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when --base-bitrate is missing with --dynamic")
	}
}

func TestPublishRejectsBaseBitrateWithoutDynamic(t *testing.T) {
	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--preset", "youtube", "--base-bitrate", "100"}, publishFrameFlags...)
	err := run(args, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when --base-bitrate is used without --dynamic")
	}
}

func TestPublishDynamicRejectsNonWebsocketURL(t *testing.T) {
	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live", "--dynamic", "https://api.example.com/config", "--base-bitrate", "100"}, publishFrameFlags...)
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

	args := append([]string{"cstream", "publish", "--in", "rtsp://in.local/live", "--out", "rtmp://out.local/live"}, publishDynamicFlags...)
	err := run(args, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if gotConfig.OnRunning == nil {
		t.Fatal("expected dynamic mode to set OnRunning hook")
	}
}

func TestForwardPasses(t *testing.T) {
	withForwardRunnerStub(t, func(cfg forward.Config) (forwardRunner, error) {
		return stubForwardRunner{}, nil
	})

	err := run([]string{"cstream", "forward", "--in", "rtsp://in.local/live", "--out", "https://whip.example.com/endpoint"}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestForwardBuildsAndRunsRunner(t *testing.T) {
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

	err := run([]string{"cstream", "forward", "--in", "rtsp://in.local/live", "--out", "https://whip.example.com/endpoint"}, io.Discard, io.Discard)
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

func TestForwardEnablesDebugLogger(t *testing.T) {
	var gotConfig forward.Config

	withForwardRunnerStub(t, func(cfg forward.Config) (forwardRunner, error) {
		gotConfig = cfg
		return stubForwardRunner{}, nil
	})

	err := run([]string{"cstream", "forward", "--in", "rtsp://in.local/live", "--out", "https://whip.example.com/endpoint", "--debug"}, io.Discard, io.Discard)
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

func TestForwardRejectsNonRTSPInput(t *testing.T) {
	err := run([]string{"cstream", "forward", "--in", "rtmp://in.local/live", "--out", "https://whip.example.com/endpoint"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error for non-rtsp input")
	}
}

func TestForwardRejectsNonHTTPSOutput(t *testing.T) {
	err := run([]string{"cstream", "forward", "--in", "rtsp://in.local/live", "--out", "http://whip.example.com/endpoint"}, io.Discard, io.Discard)
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

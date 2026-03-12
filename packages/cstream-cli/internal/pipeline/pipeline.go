package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-gst/go-gst/gst"
)

var initOnce sync.Once

func initGStreamer() {
	initOnce.Do(func() {
		gst.Init(nil)
	})
}

func NewPipeline(cfg Config) (*Pipeline, error) {
	initGStreamer()

	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	launch, err := buildLaunch(normalized)
	if err != nil {
		return nil, err
	}

	logPipelineDebug(normalized, "publish debug: launch string:\n%s\n", launch)

	gstPipeline, err := gst.NewPipelineFromString(launch)
	if err != nil {
		logPipelineDebug(normalized, "publish debug: pipeline construction failed: %v\n", err)
		return nil, fmt.Errorf("build gstreamer pipeline: %w", err)
	}

	if err := applyRuntimeFixes(gstPipeline, normalized); err != nil {
		logPipelineDebug(normalized, "publish debug: runtime fixes failed: %v\n", err)
		return nil, fmt.Errorf("apply runtime fixes: %w", err)
	}

	logPipelineDebug(normalized, "publish debug: runtime fixes enabled: monotonic_pts=%t drop_restart_events=%t\n",
		normalized.RuntimeFixes.EnforceMonotonicH264PTS,
		normalized.RuntimeFixes.DropRestartEventsAfterFirst,
	)

	return &Pipeline{
		cfg:         normalized,
		gstPipeline: gstPipeline,
		launch:      launch,
	}, nil
}

func (pipeline *Pipeline) Run(ctx context.Context) error {
	if pipeline == nil {
		return errors.New("pipeline is nil")
	}

	pipeline.debugf("publish debug: starting pipeline run\n")

	if pipeline.cfg.OnReady != nil {
		if err := pipeline.cfg.OnReady(pipeline.gstPipeline); err != nil {
			return fmt.Errorf("on ready hook: %w", err)
		}
	}

	pipeline.debugf("publish debug: setting pipeline state to PLAYING\n")
	if err := pipeline.gstPipeline.SetState(gst.StatePlaying); err != nil {
		pipeline.debugf("publish debug: failed to set PLAYING: %v\n", err)
		return fmt.Errorf("set pipeline playing: %w", err)
	}
	pipeline.debugf("publish debug: pipeline state is PLAYING\n")

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	busCtx, busCancel := context.WithCancel(context.Background())
	defer busCancel()

	runtimeErrChannel := make(chan error, 1)
	if pipeline.cfg.OnRunning != nil {
		go func() {
			runtimeErrChannel <- pipeline.cfg.OnRunning(runCtx, pipeline.gstPipeline)
		}()
	}

	busErrChannel := make(chan error, 1)
	go pipeline.watchBus(busCtx, busErrChannel)

	var runErr error
	select {
	case <-ctx.Done():
		pipeline.debugf("publish debug: run canceled by context\n")
		runErr = ctx.Err()
	case runErr = <-runtimeErrChannel:
		pipeline.debugf("publish debug: runtime hook returned: %v\n", runErr)
	case runErr = <-busErrChannel:
		pipeline.debugf("publish debug: bus watcher returned: %v\n", runErr)
	}

	cancel()
	busCancel()
	stopErr := pipeline.gstPipeline.BlockSetState(gst.StateNull)
	if stopErr != nil {
		pipeline.debugf("publish debug: failed to set NULL: %v\n", stopErr)
		if runErr != nil {
			return fmt.Errorf("%w; set pipeline null: %v", runErr, stopErr)
		}
		return fmt.Errorf("set pipeline null: %w", stopErr)
	}

	pipeline.debugf("publish debug: pipeline stopped cleanly\n")

	return runErr
}

func (pipeline *Pipeline) watchBus(ctx context.Context, errChannel chan<- error) {
	bus := pipeline.gstPipeline.GetPipelineBus()
	filter := gst.MessageError | gst.MessageEOS
	if pipeline.cfg.Debug {
		filter |= gst.MessageWarning | gst.MessageStateChanged
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg := bus.PopFiltered(filter)
		if msg == nil {
			continue
		}

		switch msg.Type() {
		case gst.MessageError:
			gErr := msg.ParseError()
			if gErr == nil {
				pipeline.debugf("publish debug: gst error without details\n")
				errChannel <- errors.New("gstreamer message error without details")
				return
			}
			pipeline.debugf("publish debug: gst error: %v\n", gErr)
			if debugInfo := gErr.DebugString(); debugInfo != "" {
				pipeline.debugf("publish debug: gst error details: %s\n", debugInfo)
			}
			errChannel <- fmt.Errorf("pipeline error: %w", gErr)
			return
		case gst.MessageWarning:
			gWarn := msg.ParseWarning()
			if gWarn != nil {
				pipeline.debugf("publish debug: gst warning: %v\n", gWarn)
				if debugInfo := gWarn.DebugString(); debugInfo != "" {
					pipeline.debugf("publish debug: gst warning details: %s\n", debugInfo)
				}
			} else {
				pipeline.debugf("publish debug: gst warning: %s\n", msg.String())
			}
		case gst.MessageStateChanged:
			oldState, newState := msg.ParseStateChanged()
			pipeline.debugf("publish debug: gst state changed: %s -> %s\n", oldState.String(), newState.String())
		case gst.MessageEOS:
			pipeline.debugf("publish debug: gst eos received\n")
		}
	}
}

func (pipeline *Pipeline) debugf(format string, args ...any) {
	logPipelineDebug(pipeline.cfg, format, args...)
}

func logPipelineDebug(cfg Config, format string, args ...any) {
	if !cfg.Debug || cfg.LogWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(cfg.LogWriter, format, args...)
}

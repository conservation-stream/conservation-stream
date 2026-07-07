package forward

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Runner interface {
	Run(context.Context) error
}

type WHIPRunner struct {
	cfg       Config
	source    Source
	publisher Publisher
	units     chan *MediaUnit
	logger    Logger
}

func NewRunner(cfg Config) (Runner, error) {
	normalized := normalizeConfig(cfg)
	if err := validateConfig(normalized); err != nil {
		return nil, err
	}

	return &WHIPRunner{
		cfg:       normalized,
		source:    NewRTSPSource(normalized),
		publisher: NewWHIPPublisher(normalized),
		units:     make(chan *MediaUnit, normalized.ChannelBufferSize),
		logger:    normalized.Logger,
	}, nil
}

func (runner *WHIPRunner) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := runner.source.Open(ctx); err != nil {
		return fmt.Errorf("open RTSP source: %w", err)
	}
	defer runner.source.Close()

	if err := runner.publisher.Open(ctx, runner.source.HasVideo(), runner.source.HasAudio()); err != nil {
		return fmt.Errorf("open WHIP publisher: %w", err)
	}
	defer runner.publisher.Close()

	var workers sync.WaitGroup
	publishErrChannel := make(chan error, 1)
	receiveErrChannel := make(chan error, 1)

	workers.Add(1)
	go func() {
		defer workers.Done()
		failPublish := func(err error) {
			select {
			case publishErrChannel <- err:
			default:
			}
			cancel()
		}

		// Watchdog: an RTSP source can stop delivering media without the
		// underlying connection erroring; treat prolonged silence as fatal.
		stallTimer := time.NewTimer(runner.cfg.SourceStallTimeout)
		defer stallTimer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stallTimer.C:
				failPublish(fmt.Errorf("RTSP source stalled: no media received for %s", runner.cfg.SourceStallTimeout))
				return
			case unit := <-runner.units:
				if unit == nil {
					continue
				}
				if !stallTimer.Stop() {
					<-stallTimer.C
				}
				stallTimer.Reset(runner.cfg.SourceStallTimeout)
				if err := runner.publisher.Publish(unit); err != nil {
					failPublish(err)
					return
				}
			}
		}
	}()

	workers.Add(1)
	go func() {
		defer workers.Done()
		err := runner.source.Receive(ctx, runner.units)
		select {
		case receiveErrChannel <- err:
		default:
		}
		cancel()
	}()

	var runErr error
	select {
	case <-ctx.Done():
		runErr = ctx.Err()
	case err := <-publishErrChannel:
		runErr = err
	case err := <-receiveErrChannel:
		runErr = err
	case err := <-runner.publisher.Failed():
		runErr = err
	}

	cancel()
	workers.Wait()

	if runErr != nil {
		runner.logger.Printf("webrtc forward terminating: %v", runErr)
	}

	if runErr == context.Canceled && ctx.Err() == context.Canceled {
		return ctx.Err()
	}
	return runErr
}

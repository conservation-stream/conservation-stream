package forward

import (
	"context"
	"fmt"
	"sync"
)

type Runner struct {
	cfg       Config
	source    Source
	publisher Publisher
	units     chan *MediaUnit
	logger    Logger
}

func NewRunner(cfg Config) (*Runner, error) {
	normalized := normalizeConfig(cfg)
	if err := validateConfig(normalized); err != nil {
		return nil, err
	}

	return &Runner{
		cfg:       normalized,
		source:    NewRTSPSource(normalized),
		publisher: NewWHIPPublisher(normalized),
		units:     make(chan *MediaUnit, normalized.ChannelBufferSize),
		logger:    normalized.Logger,
	}, nil
}

func (runner *Runner) Run(ctx context.Context) error {
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
		for {
			select {
			case <-ctx.Done():
				return
			case unit := <-runner.units:
				if unit == nil {
					continue
				}
				if err := runner.publisher.Publish(unit); err != nil {
					select {
					case publishErrChannel <- err:
					default:
					}
					cancel()
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
	}

	cancel()
	workers.Wait()

	if runErr == context.Canceled && ctx.Err() == context.Canceled {
		return ctx.Err()
	}
	return runErr
}

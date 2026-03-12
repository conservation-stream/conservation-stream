package logforwarder

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"time"
)

func NewSyslogForwarder(upstreamPublisherUrl string, cfg Config) (*SyslogForwarder, error) {
	if upstreamPublisherUrl == "" {
		return nil, fmt.Errorf("upstream URL is required")
	}

	u, err := url.Parse(upstreamPublisherUrl)
	if err != nil {
		return nil, fmt.Errorf("parse upstream URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("upstream URL must use http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("upstream URL must include a host")
	}

	ctx, cancel := context.WithCancel(context.Background())

	forwarder := &SyslogForwarder{
		Url:                     upstreamPublisherUrl,
		Config:                  cfg,
		parsedLogRecordsChannel: make(chan LogRecord, cfg.QueueSize),
		shutdownDoneChannel:     make(chan struct{}),
		context:                 ctx,
		cancel:                  cancel,
	}
	forwarder.batcherWaitGroup.Add(1)
	go forwarder.run()

	return forwarder, nil
}

func (forwarder *SyslogForwarder) Handle(conn net.Conn) {
	// Each connection handler is a producer into f.parsedLogRecordsChannel.
	// Close() waits for all producers to exit before closing the channel.
	forwarder.producers.Add(1)
	defer forwarder.producers.Done()
	defer conn.Close()

	// Note:
	// This assumes newline-delimited syslog frames.
	// It does not yet support RFC6587 octet-counted framing.
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		// Copy scanner bytes because scanner.Bytes() is reused on next Scan().
		line := append([]byte(nil), scanner.Bytes()...)

		// Fast exit if shutdown has begun.
		select {
		case <-forwarder.shutdownDoneChannel:
			return
		default:
		}

		// Parse RFC5424 into a structured message and convert to LogRecord.
		rec, err := ParseSyslogLine(forwarder.Config.RFC5424Parser, line, time.Now().UTC())
		if err != nil {
			fmt.Println("parse syslog message:", err)
			continue
		}

		// Backpressure policy: block until queued or shutdown starts.
		// This preserves logs at the cost of slowing readers when the uploader falls behind.
		if !forwarder.enqueue(rec) {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("read syslog stream: %w", err)
	}
}

func (forwarder *SyslogForwarder) Close() error {
	forwarder.mutex.Lock()
	if forwarder.isClosed {
		forwarder.mutex.Unlock()
		return nil
	}
	forwarder.isClosed = true

	// Signal producers to stop accepting/enqueueing and cancel in-flight posts/retries.
	close(forwarder.shutdownDoneChannel)
	forwarder.cancel()
	forwarder.mutex.Unlock()

	// Wait until all connection handlers have exited.
	// Only then is it safe to close the shared producer channel.
	forwarder.producers.Wait()
	close(forwarder.parsedLogRecordsChannel)

	// Wait until the batching/uploader goroutine drains any queued records and exits.
	forwarder.batcherWaitGroup.Wait()
	return nil
}

func (forwarder *SyslogForwarder) enqueue(rec LogRecord) bool {
	select {
	case forwarder.parsedLogRecordsChannel <- rec:
		return true
	case <-forwarder.shutdownDoneChannel:
		return false
	}
}

func (forwarder *SyslogForwarder) run() {
	defer forwarder.batcherWaitGroup.Done()

	ticker := time.NewTicker(forwarder.Config.FlushEvery)
	defer ticker.Stop()

	batch := make([]LogRecord, 0, forwarder.Config.BatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		// Copy out so the current batch slice can be reused immediately.
		toSend := append([]LogRecord(nil), batch...)
		batch = batch[:0]

		// Data flow: buffered records -> BatchSender (e.g. NDJSON POST with retries)
		if err := forwarder.Config.BatchSender.SendBatch(forwarder.context, toSend); err != nil {
			fmt.Println("flush batch:", err)
		}
	}

	for {
		select {
		case rec, ok := <-forwarder.parsedLogRecordsChannel:
			if !ok {
				// All producers are done and channel is closed.
				// Drain complete: flush final partial batch, then exit.
				flush()
				return
			}

			// Accumulate records into the current batch.
			batch = append(batch, rec)
			if len(batch) >= forwarder.Config.BatchSize {
				flush()
			}

		case <-ticker.C:
			// Time-based flush for low-volume periods.
			flush()
		}
	}
}

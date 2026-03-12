package logforwarder

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	syslog "github.com/observiq/go-syslog/v3"
)

type LogRecord struct {
	TS         string            `json:"ts"`
	Hostname   string            `json:"hostname,omitempty"`
	Source     string            `json:"source,omitempty"`
	Message    string            `json:"message,omitempty"`
	Severity   *uint8            `json:"severity,omitempty"`
	Facility   *uint8            `json:"facility,omitempty"`
	ProcID     string            `json:"proc_id,omitempty"`
	MsgID      string            `json:"msg_id,omitempty"`
	Structured map[string]string `json:"structured,omitempty"`
	Raw        string            `json:"raw,omitempty"`
	ReceivedAt string            `json:"received_at"`
}

type Config struct {
	BatchSize     int
	FlushEvery    time.Duration
	Retries       uint
	HTTPClient    *http.Client
	RFC5424Parser syslog.Machine
	QueueSize     int
	// BatchSender sends batches upstream. If nil, an HTTP sender is created from the forwarder URL and HTTPClient/Retries.
	BatchSender BatchSender
}

// BatchSender sends a batch of log records upstream (e.g. NDJSON over HTTP).
type BatchSender interface {
	SendBatch(ctx context.Context, records []LogRecord) error
}

type LogForwarder interface {
	Handle(net.Conn)
	Close() error
}

type SyslogForwarder struct {
	Url    string
	Config Config

	// Data path:
	// Handle() parses TCP syslog lines into LogRecord values and pushes them to ch.
	// run() is the single consumer that batches from ch and posts NDJSON upstream.
	parsedLogRecordsChannel chan LogRecord

	// done is closed once shutdown starts. Producers stop enqueueing after this.
	shutdownDoneChannel chan struct{}

	// ctx/cancel control in-flight and retrying HTTP posts so Close() can stop them.
	context context.Context
	cancel  context.CancelFunc

	// wg waits for the batcher/uploader goroutine.
	batcherWaitGroup sync.WaitGroup

	// producers waits for all Handle() calls to exit before ch is closed.
	producers sync.WaitGroup

	mutex    sync.Mutex
	isClosed bool
}

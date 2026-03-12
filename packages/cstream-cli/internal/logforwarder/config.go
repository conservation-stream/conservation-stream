package logforwarder

import (
	"net/http"
	"time"

	"github.com/observiq/go-syslog/v3/rfc5424"
)

func NewDefaultSyslogConfig() Config {
	return Config{
		BatchSize:     100,
		FlushEvery:    time.Second,
		Retries:       5,
		HTTPClient:    &http.Client{Timeout: 15 * time.Second},
		RFC5424Parser: rfc5424.NewParser(rfc5424.WithBestEffort()),
	}
}

package logforwarder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cenkalti/backoff/v5"
)

// httpBatchSender sends batches as NDJSON POST requests with exponential backoff.
type httpBatchSender struct {
	url     string
	client  *http.Client
	retries uint
}

// NewHTTPBatchSender returns a BatchSender that POSTs NDJSON to url with the given client and retry count.
func NewHTTPBatchSender(url string, client *http.Client, retries uint) BatchSender {
	return &httpBatchSender{url: url, client: client, retries: retries}
}

func (sender *httpBatchSender) SendBatch(ctx context.Context, records []LogRecord) error {
	if len(records) == 0 {
		return nil
	}
	bo := backoff.NewExponentialBackOff()
	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		err := sender.post(ctx, records)
		return struct{}{}, err
	},
		backoff.WithBackOff(bo),
		backoff.WithMaxTries(sender.retries),
	)
	return err
}

func (sender *httpBatchSender) post(ctx context.Context, records []LogRecord) error {
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("encode ndjson: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sender.url, &body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := sender.client.Do(req)
	if err != nil {
		return fmt.Errorf("post upstream: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream status: %s", resp.Status)
	}
	return nil
}

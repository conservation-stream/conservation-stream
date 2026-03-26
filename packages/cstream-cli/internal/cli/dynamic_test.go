package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestRunDynamicBitrateAppliesUpdates(t *testing.T) {
	var applied []uint
	apply := func(bitrate uint) error {
		applied = append(applied, bitrate)
		return nil
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}

		var init map[string]string
		if err := json.Unmarshal(data, &init); err != nil || init["type"] != "init" {
			t.Errorf("expected init message, got %s", data)
			return
		}

		payload, _ := json.Marshal(dynamicMessage{Bitrate: 500})
		if err := conn.Write(r.Context(), websocket.MessageText, payload); err != nil {
			return
		}

		conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = runDynamicBitrate(ctx, wsURL, 100, apply)

	if len(applied) != 2 {
		t.Fatalf("expected 2 bitrate applications (base + update), got %d: %v", len(applied), applied)
	}
	if applied[0] != 100 {
		t.Fatalf("expected base bitrate 100, got %d", applied[0])
	}
	if applied[1] != 500 {
		t.Fatalf("expected updated bitrate 500, got %d", applied[1])
	}
}

func TestRunDynamicBitrateSendsInit(t *testing.T) {
	initReceived := make(chan map[string]string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}

		var msg map[string]string
		json.Unmarshal(data, &msg)
		initReceived <- msg

		conn.Close(websocket.StatusNormalClosure, "done")
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = runDynamicBitrate(ctx, wsURL, 100, func(uint) error { return nil })

	select {
	case msg := <-initReceived:
		if msg["type"] != "init" {
			t.Fatalf("expected init type, got %q", msg["type"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for init message")
	}
}

func TestRunDynamicBitrateReturnsApplyError(t *testing.T) {
	applyErr := errors.New("encoder failure")
	apply := func(uint) error {
		return applyErr
	}

	ctx := context.Background()
	err := runDynamicBitrate(ctx, "ws://unused:1234", 100, apply)
	if !errors.Is(err, applyErr) {
		t.Fatalf("expected wrapped apply error, got: %v", err)
	}
}

func TestRunDynamicBitrateReturnsConnectError(t *testing.T) {
	apply := func(uint) error { return nil }

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := runDynamicBitrate(ctx, "ws://127.0.0.1:1/nonexistent", 100, apply)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

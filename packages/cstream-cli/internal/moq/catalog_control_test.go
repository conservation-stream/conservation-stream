package moq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestRunDynamicCatalogControlWritesUpdatesAndSendsInit(t *testing.T) {
	controlPath := t.TempDir() + "/catalog-control.json"
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

		var init map[string]string
		if err := json.Unmarshal(data, &init); err != nil {
			t.Errorf("decode init message: %v", err)
			return
		}
		initReceived <- init

		if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"advertise":["360p"]}`)); err != nil {
			return
		}

		<-r.Context().Done()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errs := make(chan error, 1)
	var logs bytes.Buffer
	go func() {
		errs <- runDynamicCatalogControl(ctx, wsURL, controlPath, &logs)
	}()

	select {
	case msg := <-initReceived:
		if msg["type"] != "init" {
			t.Fatalf("expected init type, got %q", msg["type"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for init message")
	}

	waitForFileContent(t, controlPath, "{\"advertise\":[\"360p\"]}\n")
	if !strings.Contains(logs.String(), "moq catalog control={\"advertise\":[\"360p\"]}") {
		t.Fatalf("expected compact catalog control log, got %q", logs.String())
	}

	cancel()
	select {
	case err := <-errs:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dynamic catalog control to stop")
	}
}

func TestSeedCatalogControlFileDefaultsToEmptyAdvertise(t *testing.T) {
	controlPath := t.TempDir() + "/catalog-control.json"

	if err := seedCatalogControlFile(controlPath, ""); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatalf("read control file: %v", err)
	}
	if string(data) != emptyCatalogControl {
		t.Fatalf("unexpected seed content: %q", data)
	}
}

func TestSeedCatalogControlFileUsesValidatedSeedFile(t *testing.T) {
	dir := t.TempDir()
	seedPath := dir + "/seed.json"
	controlPath := dir + "/catalog-control.json"
	if err := os.WriteFile(seedPath, []byte(`{"advertise":["720p"]}`), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	if err := seedCatalogControlFile(controlPath, seedPath); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatalf("read control file: %v", err)
	}
	if string(data) != "{\"advertise\":[\"720p\"]}\n" {
		t.Fatalf("unexpected seed content: %q", data)
	}
}

func TestWriteCatalogControlUpdateIgnoresTypeOnlyMessage(t *testing.T) {
	controlPath := t.TempDir() + "/catalog-control.json"
	if err := os.WriteFile(controlPath, []byte(emptyCatalogControl), 0o644); err != nil {
		t.Fatalf("write control file: %v", err)
	}

	applied, err := writeCatalogControlUpdate(controlPath, []byte(`{"type":"init"}`))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if applied {
		t.Fatal("expected type-only message to be ignored")
	}

	data, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatalf("read control file: %v", err)
	}
	if string(data) != emptyCatalogControl {
		t.Fatalf("expected control file to remain unchanged, got %q", data)
	}
}

func TestWriteCatalogControlUpdateRejectsUnknownObject(t *testing.T) {
	controlPath := t.TempDir() + "/catalog-control.json"

	applied, err := writeCatalogControlUpdate(controlPath, []byte(`{"foo":true}`))
	if err == nil {
		t.Fatal("expected error for unknown control message")
	}
	if applied {
		t.Fatal("expected invalid message not to be applied")
	}
}

func TestWriteCatalogControlUpdateAllowsEmptyObjectAsAdvertiseAll(t *testing.T) {
	controlPath := t.TempDir() + "/catalog-control.json"

	applied, err := writeCatalogControlUpdate(controlPath, []byte(`{}`))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !applied {
		t.Fatal("expected empty object to be applied")
	}

	data, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatalf("read control file: %v", err)
	}
	if string(data) != "{}\n" {
		t.Fatalf("unexpected control file content: %q", data)
	}
}

func waitForFileContent(t *testing.T, path string, want string) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-deadline:
			data, _ := os.ReadFile(path)
			t.Fatalf("timed out waiting for %s, last content %q", want, data)
		case <-tick.C:
			data, err := os.ReadFile(path)
			if err == nil && string(data) == want {
				return
			}
		}
	}
}

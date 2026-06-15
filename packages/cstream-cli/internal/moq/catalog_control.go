package moq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/coder/websocket"
)

const emptyCatalogControl = "{\"advertise\":[]}\n"

func runDynamicCatalogControl(ctx context.Context, endpoint string, controlPath string, logWriter io.Writer) error {
	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("connect to catalog control endpoint: %w", err)
	}
	defer conn.CloseNow()

	initPayload, _ := json.Marshal(map[string]string{"type": "init"})
	if err := conn.Write(ctx, websocket.MessageText, initPayload); err != nil {
		return fmt.Errorf("send catalog control init message: %w", err)
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read catalog control message: %w", err)
		}

		applied, err := writeCatalogControlUpdate(controlPath, data)
		if err != nil {
			return err
		}
		if applied {
			_, _ = fmt.Fprintf(logWriter, "moq catalog control=%s\n", compactJSON(data))
		}
	}
}

func seedCatalogControlFile(path string, seedPath string) error {
	seed := []byte(emptyCatalogControl)
	if seedPath != "" {
		raw, err := os.ReadFile(seedPath)
		if err != nil {
			return fmt.Errorf("read catalog control seed file: %w", err)
		}
		if _, err := normalizeCatalogControl(raw); err != nil {
			return fmt.Errorf("validate catalog control seed file: %w", err)
		}
		seed = raw
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create catalog control directory: %w", err)
	}
	if err := os.WriteFile(path, ensureTrailingNewline(seed), 0o644); err != nil {
		return fmt.Errorf("write catalog control seed file: %w", err)
	}
	return nil
}

func writeCatalogControlUpdate(path string, raw []byte) (bool, error) {
	normalized, err := normalizeCatalogControl(raw)
	if err != nil {
		return false, err
	}
	if normalized == nil {
		return false, nil
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, ensureTrailingNewline(normalized), 0o644); err != nil {
		return false, fmt.Errorf("write catalog control update: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return false, fmt.Errorf("replace catalog control file: %w", err)
	}
	return true, nil
}

func normalizeCatalogControl(raw []byte) ([]byte, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("catalog control message is empty")
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("decode catalog control message: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("catalog control message must be a JSON object")
	}

	hasControlField := false
	for _, field := range []string{"advertise", "renditions", "video"} {
		if _, ok := object[field]; ok {
			hasControlField = true
			break
		}
	}

	if !hasControlField {
		if _, hasType := object["type"]; hasType && len(object) == 1 {
			return nil, nil
		}
		if len(object) != 0 {
			return nil, fmt.Errorf("catalog control message must contain advertise, renditions, or video")
		}
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return nil, fmt.Errorf("compact catalog control message: %w", err)
	}
	return compact.Bytes(), nil
}

func compactJSON(raw []byte) string {
	var compact bytes.Buffer
	if err := json.Compact(&compact, bytes.TrimSpace(raw)); err != nil {
		return string(bytes.TrimSpace(raw))
	}
	return compact.String()
}

func ensureTrailingNewline(raw []byte) []byte {
	raw = bytes.TrimSpace(raw)
	out := make([]byte, 0, len(raw)+1)
	out = append(out, raw...)
	out = append(out, '\n')
	return out
}

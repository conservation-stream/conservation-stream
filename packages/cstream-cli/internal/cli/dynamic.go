package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/coder/websocket"
	"github.com/go-gst/go-gst/gst"
)

type bitrateUpdater func(bitrate uint) error

type dynamicMessage struct {
	Bitrate uint `json:"bitrate"`
}

func watchDynamicBitrate(stdout io.Writer, endpoint string, baseBitrate uint) func(context.Context, *gst.Pipeline) error {
	return func(ctx context.Context, gstPipeline *gst.Pipeline) error {
		encoder, err := gstPipeline.GetElementByName("encoder")
		if err != nil {
			return fmt.Errorf("get encoder element: %w", err)
		}

		apply := func(bitrate uint) error {
			encoder.SetProperty("bitrate", bitrate)
			_, _ = fmt.Fprintf(stdout, "publish bitrate=%d kbps\n", bitrate)
			return nil
		}

		return runDynamicBitrate(ctx, endpoint, baseBitrate, apply)
	}
}

func runDynamicBitrate(ctx context.Context, endpoint string, baseBitrate uint, apply bitrateUpdater) error {
	if err := apply(baseBitrate); err != nil {
		return fmt.Errorf("apply base bitrate: %w", err)
	}

	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("connect to dynamic endpoint: %w", err)
	}
	defer conn.CloseNow()

	initPayload, _ := json.Marshal(map[string]string{"type": "init"})
	if err := conn.Write(ctx, websocket.MessageText, initPayload); err != nil {
		return fmt.Errorf("send init message: %w", err)
	}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read dynamic message: %w", err)
		}

		var msg dynamicMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("decode dynamic message: %w", err)
		}

		if err := apply(msg.Bitrate); err != nil {
			return err
		}
	}
}

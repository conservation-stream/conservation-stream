package pipeline

import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"

	"github.com/go-gst/go-gst/gst"
)

func applyRuntimeFixes(gstPipeline *gst.Pipeline, cfg Config) error {
	if gstPipeline == nil {
		return fmt.Errorf("pipeline is nil")
	}

	if cfg.RuntimeFixes.EnforceMonotonicH264PTS {
		fps, err := parseFrameRate(cfg.OriginFrameInfo.Rate)
		if err != nil {
			return fmt.Errorf("parse origin frame rate: %w", err)
		}

		identity, err := gstPipeline.GetElementByName("h264tsfix")
		if err != nil {
			return fmt.Errorf("get h264tsfix element: %w", err)
		}

		if err := enforceMonotonicH264PTS(identity, fps); err != nil {
			return err
		}
	}

	if cfg.RuntimeFixes.DropRestartEventsAfterFirst {
		outsink, err := gstPipeline.GetElementByName("outsink")
		if err != nil {
			return fmt.Errorf("get outsink element: %w", err)
		}

		if err := dropRestartEventsAfterFirst(outsink); err != nil {
			return err
		}
	}

	return nil
}

func parseFrameRate(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("frame rate is required")
	}

	if strings.Contains(raw, "/") {
		numeratorRaw, denominatorRaw, ok := strings.Cut(raw, "/")
		if !ok {
			return 0, fmt.Errorf("invalid frame rate %q", raw)
		}

		numerator, err := strconv.ParseFloat(strings.TrimSpace(numeratorRaw), 64)
		if err != nil {
			return 0, fmt.Errorf("parse frame rate numerator: %w", err)
		}

		denominator, err := strconv.ParseFloat(strings.TrimSpace(denominatorRaw), 64)
		if err != nil {
			return 0, fmt.Errorf("parse frame rate denominator: %w", err)
		}
		if denominator == 0 {
			return 0, fmt.Errorf("frame rate denominator cannot be zero")
		}

		return numerator / denominator, nil
	}

	fps, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("parse frame rate: %w", err)
	}

	return fps, nil
}

func enforceMonotonicH264PTS(element *gst.Element, fps float64) error {
	pad := element.GetStaticPad("src")
	if pad == nil {
		return fmt.Errorf("%s has no src pad", element.GetName())
	}

	frameDuration := clockTimeFromUint64(uint64(1e9 / fps))

	var (
		haveLast        bool
		lastAdjustedPTS gst.ClockTime
		offset          gst.ClockTime
	)

	_ = pad.AddProbe(gst.PadProbeTypeBuffer, func(_ *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		buffer := info.GetBuffer()
		if buffer == nil {
			return gst.PadProbeOK
		}

		pts := buffer.PresentationTimestamp()
		if pts == gst.ClockTimeNone {
			return gst.PadProbeOK
		}

		adjustedPTS := pts.Add(offset)
		if haveLast && adjustedPTS <= lastAdjustedPTS {
			requiredOffset := lastAdjustedPTS.Add(frameDuration).Sub(adjustedPTS)
			offset += requiredOffset
			adjustedPTS = pts.Add(offset)
			buffer.SetPresentationTimestamp(adjustedPTS)
		}

		lastAdjustedPTS = adjustedPTS
		haveLast = true
		return gst.PadProbeOK
	})

	return nil
}

func dropRestartEventsAfterFirst(outsink *gst.Element) error {
	pad := outsink.GetStaticPad("sink_0")
	if pad == nil {
		return fmt.Errorf("%s has no sink_0 pad", outsink.GetName())
	}

	seen := map[string]bool{}

	_ = pad.AddProbe(gst.PadProbeTypeEventDownstream, func(_ *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		event := info.GetEvent()
		if event == nil {
			return gst.PadProbeOK
		}

		eventType := event.Type().String()
		switch eventType {
		case "stream-start", "segment", "caps":
			if seen[eventType] {
				return gst.PadProbeDrop
			}
			seen[eventType] = true
		}

		return gst.PadProbeOK
	})

	return nil
}

func clockTimeFromUint64(value uint64) gst.ClockTime {
	return *(*gst.ClockTime)(unsafe.Pointer(&value))
}

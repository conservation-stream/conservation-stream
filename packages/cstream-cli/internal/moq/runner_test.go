package moq

import (
	"reflect"
	"testing"
)

func TestFFmpegForwardArgsRemuxRTSPToMPEGTS(t *testing.T) {
	got := ffmpegForwardArgs("rtsp://source.local/live")
	want := []string{
		"-hide_banner",
		"-nostdin",
		"-fflags",
		"nobuffer",
		"-rtsp_transport",
		"tcp",
		"-i",
		"rtsp://source.local/live",
		"-c",
		"copy",
		"-f",
		"mpegts",
		"-flush_packets",
		"1",
		"-muxdelay",
		"0",
		"-muxpreload",
		"0",
		"-",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ffmpeg args:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestMoQPublishArgs(t *testing.T) {
	got := moqPublishArgs(Config{
		RelayURL:         "https://relay.example.com/anon",
		Broadcast:        "stream.hang",
		ClientBind:       "0.0.0.0:0",
		TLSDisableVerify: true,
	}, "fmp4")
	want := []string{
		"publish",
		"--client-bind",
		"0.0.0.0:0",
		"--tls-disable-verify",
		"--url",
		"https://relay.example.com/anon",
		"--broadcast",
		"stream.hang",
		"fmp4",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected moq-cli args:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestCstreamMoQPublisherArgs(t *testing.T) {
	got := cstreamMoQPublisherArgs(Config{
		RTSPSourceURL:    "rtsp://source.local/live",
		RelayURL:         "https://relay.example.com/anon",
		Broadcast:        "stream.hang",
		ClientBind:       "0.0.0.0:0",
		TLSDisableVerify: true,
		CatalogControl:   "/tmp/catalog.json",
		VideoCodec:       "h265",
		Renditions: []Rendition{
			{Name: "720p", Width: 1280, Height: 720, Bitrate: "2500k"},
			{Name: "360p", Width: 640, Height: 360, Bitrate: "800k"},
			{Name: "passthrough", Passthrough: true},
		},
	})
	want := []string{
		"--client-bind",
		"0.0.0.0:0",
		"--tls-disable-verify",
		"--source-rtsp",
		"rtsp://source.local/live",
		"--video-codec",
		"h265",
		"--url",
		"https://relay.example.com/anon",
		"--broadcast",
		"stream.hang",
		"--rendition",
		"720p:1280x720:2500k",
		"--advertise-rendition",
		"720p",
		"--rendition",
		"360p:640x360:800k",
		"--advertise-rendition",
		"360p",
		"--rendition",
		"passthrough",
		"--advertise-rendition",
		"passthrough",
		"--catalog-control",
		"/tmp/catalog.json",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected cstream-moq-publisher args:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestParseRendition(t *testing.T) {
	got, err := ParseRendition("720p:1280x720:2500k")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := Rendition{Name: "720p", Width: 1280, Height: 720, Bitrate: "2500k"}
	if got != want {
		t.Fatalf("unexpected rendition: got %+v want %+v", got, want)
	}
}

func TestParsePassthroughRendition(t *testing.T) {
	got, err := ParseRendition("passthrough")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := Rendition{Name: "passthrough", Passthrough: true}
	if got != want {
		t.Fatalf("unexpected rendition: got %+v want %+v", got, want)
	}
}

func TestParseNamedPassthroughRendition(t *testing.T) {
	got, err := ParseRendition("original:passthrough")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := Rendition{Name: "original", Passthrough: true}
	if got != want {
		t.Fatalf("unexpected rendition: got %+v want %+v", got, want)
	}
}

func TestParseRenditionRejectsInvalidShape(t *testing.T) {
	if _, err := ParseRendition("720p:1280x720"); err == nil {
		t.Fatal("expected invalid rendition shape to fail")
	}
}

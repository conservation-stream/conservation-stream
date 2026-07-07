# cstream-cli

## Run locally

```bash
go run ./cmd/cstream --help
go run ./cmd/cstream rtsp publish --in rtsp://source/live --out rtmp://target/live --preset twitch --height 720 --width 1280 --rate 30/1
go run ./cmd/cstream rtsp publish --in rtsp://source/live --out rtsp://target/live --dynamic wss://api.example.com/config --base-bitrate 100 --height 720 --width 1280 --rate 30/1
go run ./cmd/cstream webrtc forward --in rtsp://source/live --out https://whip.example.com/endpoint
go run ./cmd/cstream moq forward --in rtsp://source/live --out https://cdn.moq.dev/anon --broadcast unique-name.hang
go run ./cmd/cstream moq publish --in rtsp://camera/axis-media/media.amp?camera=1 --out https://cdn.moq.dev/anon --broadcast unique-name.hang --video-codec h265 --catalog-control ./catalog-control.json
go run ./cmd/cstream moq publish --in rtsp://source/live --out https://cdn.moq.dev/anon --broadcast unique-name.hang --video-codec h265 --rendition passthrough --rendition 720p:1280x720:2500k --rendition 360p:640x360:800k --catalog-control ./catalog-control.json --catalog-control-dynamic ws://localhost:8080/catalog
go run ./cmd/cstream moq publish --out https://cdn.moq.dev/anon --broadcast unique-name.hang --rendition-source 720p=rtsp://camera/high --rendition-source 360p=rtsp://camera/low --catalog-control ./catalog-control.json
go run ./cmd/cstream version
```

## Build

```bash
go build -o bin/cstream ./cmd/cstream
./bin/cstream rtsp publish --in rtsp://source/live --out rtmp://target/live --preset youtube --height 720 --width 1280 --rate 30/1
```

## MoQ forward

`moq forward` remuxes an RTSP input to MPEG-TS and publishes it to a MoQ relay without re-encoding:

```bash
cstream moq forward \
  --in rtsp://127.0.0.1:8554/live \
  --out https://cdn.moq.dev/anon \
  --broadcast unique-name.hang
```

MoQ forward mode shells out to `ffmpeg` and `moq-cli`: FFmpeg reads the RTSP input, remuxes H.264/AAC-style streams to MPEG-TS, and pipes them into `moq-cli publish`. Use a unique broadcast name; the `.hang` suffix makes the catalog format explicit for moq.dev tooling.

## MoQ publish renditions

`moq publish` publishes one or more video renditions through `cstream-moq-publisher`, a small Hang/MoQ publisher that owns the broadcast catalog.

For Axis cameras, the simplest form uses `--in` as a camera shortcut. If no explicit `--rendition` or `--rendition-source` is provided, cstream builds two H.265/H.264 RTSP rendition URLs from the camera URL and publishes both without transcoding:

```bash
cstream moq publish \
  --in rtsp://camera.local/axis-media/media.amp?camera=1 \
  --out https://cdn.moq.dev/anon \
  --broadcast unique-name.hang \
  --video-codec h265 \
  --catalog-control ./catalog-control.json
```

The generated sources are `low` (`640x360`, short keyframe interval, lower bitrate) and `high` (`1280x720`, longer keyframe interval, higher bitrate). This is the preferred path when the camera can produce the variants itself.

To publish separate pre-encoded camera or encoder outputs without transcoding, pass one RTSP source per rendition:

```bash
cstream moq publish \
  --out https://cdn.moq.dev/anon \
  --broadcast unique-name.hang \
  --rendition-source 720p=rtsp://camera.local/high \
  --rendition-source 360p=rtsp://camera.local/low \
  --catalog-control ./catalog-control.json
```

Each `--rendition-source` uses `name=rtsp://...`; `--in` is optional and should be omitted in this mode. cstream opens each source as RTSP/RTP directly, requests depacketized Annex B frames, detects H.264 vs H.265 from RTSP metadata, and feeds those frames into `moq-mux` without FFmpeg or re-encoding. For Axis-style URLs, `videobitrate`, `videomaxbitrate`, or `bitrate` query values are used as the catalog bitrate when present.

The older single-input encode mode remains available:

```bash
cstream moq publish \
  --in rtsp://127.0.0.1:8554/live \
  --out https://cdn.moq.dev/anon \
  --broadcast unique-name.hang \
  --video-codec h265 \
  --rendition passthrough \
  --rendition 720p:1280x720:2500k \
  --rendition 360p:640x360:800k \
  --catalog-control ./catalog-control.json \
  --catalog-control-dynamic ws://localhost:8080/catalog
```

Each encoded `--rendition` uses `name:WIDTHxHEIGHT:BITRATE`. Use `--rendition passthrough` to publish the original video without re-encoding, or `--rendition original:passthrough` if you want a custom catalog label. `--video-codec` selects the published codec (`h264` by default, or `h265` for HEVC/Hang `hev1` tracks). Advertised renditions are kept warm and emitted as Hang legacy frame tracks, matching the smoother per-frame cadence used by `moq forward` while still avoiding an HLS playlist/segment loop.

The optional `--catalog-control` file is watched while publishing. It controls which configured video renditions are advertised in `catalog.json` without changing the encoder ladder:

```json
{
  "advertise": ["360p"]
}
```

Omit `advertise`, or include all rendition names, to advertise the full ladder again. Unadvertised tracks stay out of the live catalog and their encoders remain stopped.

`--catalog-control-dynamic` connects to a WebSocket control endpoint and uses `--catalog-control` as the initial seed file. On connect, cstream sends:

```json
{"type":"init"}
```

Control messages use the same file shape, for example:

```json
{"advertise":["360p"]}
```

Messages containing only `type` are ignored, `{"advertise":[]}` hides all renditions, and `{}` advertises the full configured ladder again.

Watch the broadcast with the moq.dev playground:

```text
https://moq.dev/watch/?name=unique-name.hang
```

For Cloudflare interop testing, the current interop relay negotiates draft-14 with `moq-cli`, but its certificate may require:

```bash
cstream moq forward \
  --in rtsp://127.0.0.1:8554/live \
  --out https://interop-relay.cloudflare.mediaoverquic.com \
  --broadcast unique-name.hang \
  --moq-tls-disable-verify
```

## Test

```bash
go test ./...
```

## Release

Releases are handled in the repo's shared GitHub Actions CI pattern by `build.action.ts` and `deploy.action.ts`.

The build jobs publish per-architecture container images, then `deploy.action.ts` creates the GitHub Release and the final multi-arch tags from the package `Dockerfile`.

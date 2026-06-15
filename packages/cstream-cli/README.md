# cstream-cli

## Run locally

```bash
go run ./cmd/cstream --help
go run ./cmd/cstream rtsp publish --in rtsp://source/live --out rtmp://target/live --preset twitch --height 720 --width 1280 --rate 30/1
go run ./cmd/cstream rtsp publish --in rtsp://source/live --out rtsp://target/live --dynamic wss://api.example.com/config --base-bitrate 100 --height 720 --width 1280 --rate 30/1
go run ./cmd/cstream webrtc forward --in rtsp://source/live --out https://whip.example.com/endpoint
go run ./cmd/cstream moq forward --in rtsp://source/live --out https://cdn.moq.dev/anon --broadcast unique-name.hang
go run ./cmd/cstream moq publish --in rtsp://source/live --out https://cdn.moq.dev/anon --broadcast unique-name.hang --video-codec h265 --rendition passthrough --rendition 720p:1280x720:2500k --rendition 360p:640x360:800k --catalog-control ./catalog-control.json --catalog-control-dynamic ws://localhost:8080/catalog
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

`moq publish` encodes one or more video renditions through `cstream-moq-publisher`, a small Hang/MoQ publisher that owns the broadcast catalog:

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

# cstream-cli

## Run locally

```bash
go run ./cmd/cstream --help
go run ./cmd/cstream publish --in rtsp://source/live --out rtmp://target/live --preset twitch --height 720 --width 1280 --rate 30/1
go run ./cmd/cstream publish --in rtsp://source/live --out rtsp://target/live --dynamic wss://api.example.com/config --base-bitrate 100 --height 720 --width 1280 --rate 30/1
go run ./cmd/cstream forward --in rtsp://source/live --out https://whip.example.com/endpoint
go run ./cmd/cstream version
```

## Build

```bash
go build -o bin/cstream ./cmd/cstream
./bin/cstream publish --in rtsp://source/live --out rtmp://target/live --preset youtube --height 720 --width 1280 --rate 30/1
```

## Test

```bash
go test ./...
```

## Release

Releases are handled in the repo's shared GitHub Actions CI pattern by `build.action.ts` and `deploy.action.ts`.

The build jobs publish per-architecture container images, then `deploy.action.ts` creates the GitHub Release and the final multi-arch tags from the package `Dockerfile`.

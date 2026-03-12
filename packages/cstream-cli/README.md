# cstream-cli

Starter Go CLI using `urfave/cli/v2` with `publish` and `forward` commands.

## Run locally

```bash
go run ./cmd/cstream --help
go run ./cmd/cstream publish --in rtsp://source/live --out rtmp://target/live --preset twitch
go run ./cmd/cstream publish --in rtsp://source/live --out rtsp://target/live --dynamic https://api.example.com/config
go run ./cmd/cstream forward --in rtsp://source/live --out https://whip.example.com/endpoint
go run ./cmd/cstream version
```

## Build

```bash
go build -o bin/cstream ./cmd/cstream
./bin/cstream publish --in rtsp://source/live --out rtmp://target/live --preset youtube
```

## Test

```bash
go test ./...
```

## Release

Releases are handled in the repo's shared GitHub Actions CI pattern by `build.action.ts` and `deploy.action.ts`.

The build jobs publish per-architecture container images, then `deploy.action.ts` creates the GitHub Release and the final multi-arch tags from the package `Dockerfile`.

The image is published to `ghcr.io/<owner>/mediamtx-cstream` with:

- the current release ref name, for example `v0.1.0`
- `latest`

FROM golang:1.24-bookworm AS builder

WORKDIR /src/packages/cstream-cli

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		build-essential \
		libglib2.0-dev \
		libgstreamer-plugins-base1.0-dev \
		libgstreamer1.0-dev \
		pkg-config \
	&& rm -rf /var/lib/apt/lists/*

COPY packages/cstream-cli/go.mod packages/cstream-cli/go.sum ./
RUN go mod download

COPY packages/cstream-cli ./

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/cstream ./cmd/cstream

FROM ubuntu:24.04

ARG TARGETARCH
ARG MEDIAMTX_VERSION=1.17.0

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		bash \
		ca-certificates \
		curl \
		ffmpeg \
		gstreamer1.0-libav \
		gstreamer1.0-plugins-bad \
		gstreamer1.0-plugins-base \
		gstreamer1.0-plugins-good \
		gstreamer1.0-plugins-ugly \
		gstreamer1.0-rtsp \
		gstreamer1.0-tools \
		libglib2.0-0 \
		libgstreamer-plugins-base1.0-0 \
		libgstreamer1.0-0 \
	&& rm -rf /var/lib/apt/lists/*

RUN set -eux; \
	case "${TARGETARCH}" in \
		amd64) mediamtx_arch="amd64" ;; \
		arm64) mediamtx_arch="arm64" ;; \
		*) echo "unsupported TARGETARCH: ${TARGETARCH}"; exit 1 ;; \
	esac; \
	curl -fsSL "https://github.com/bluenviron/mediamtx/releases/download/v${MEDIAMTX_VERSION}/mediamtx_v${MEDIAMTX_VERSION}_linux_${mediamtx_arch}.tar.gz" -o /tmp/mediamtx.tar.gz; \
	tar -xzf /tmp/mediamtx.tar.gz -C /usr/local/bin mediamtx; \
	chmod +x /usr/local/bin/mediamtx; \
	rm -f /tmp/mediamtx.tar.gz

COPY --from=builder /out/cstream /usr/local/bin/cstream

RUN chmod +x /usr/local/bin/cstream

EXPOSE 8554 1935 8889 8189/udp

ENTRYPOINT ["/usr/local/bin/mediamtx"]

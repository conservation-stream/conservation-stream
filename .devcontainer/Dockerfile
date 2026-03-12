FROM mcr.microsoft.com/devcontainers/base:ubuntu-24.04

RUN apt-get update \
	&& DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
		build-essential \
		gir1.2-gstreamer-1.0 \
		gstreamer1.0-libav \
		gstreamer1.0-plugins-bad \
		gstreamer1.0-plugins-base \
		gstreamer1.0-plugins-good \
		gstreamer1.0-plugins-ugly \
		gstreamer1.0-rtsp \
		gstreamer1.0-tools \
		libglib2.0-dev \
		libgstreamer-plugins-base1.0-dev \
		libgstreamer1.0-dev \
		pkg-config \
	&& rm -rf /var/lib/apt/lists/*

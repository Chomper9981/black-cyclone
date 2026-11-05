# Build environment for linux/package-deb.sh (see linux/docker-deb.ps1).
# Ubuntu 22.04: the oldest glibc / WebKitGTK 4.1 we target, so the .deb also
# installs on Ubuntu 24.04+ and Debian 12+.
FROM ubuntu:22.04
ARG GO_VERSION=1.27.0
ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential pkg-config ca-certificates curl dpkg-dev \
        libgtk-3-dev libwebkit2gtk-4.1-dev \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" | tar -xz -C /usr/local
ENV PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=local

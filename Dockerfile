# Where the binaries come from: "builder" compiles them here, "prebuilt"
# takes go_media_downloader-linux-<arch> / init_imdb-linux-<arch> from the
# build context (CI passes the binaries its build job already made).
ARG BINARIES=builder

# Build stage: runs on the build machine and cross-compiles for the target.
FROM --platform=$BUILDPLATFORM golang:trixie AS builder

LABEL org.opencontainers.image.authors="Kellerman81 <Kellerman81@gmail.com>"

WORKDIR /build

# Module files first for better caching
COPY ./pkg/main/go.mod ./pkg/main/go.sum ./pkg/main/
COPY ./pkg/imdb/go.mod ./pkg/imdb/go.sum ./pkg/imdb/
RUN cd pkg/main && go mod download && cd ../imdb && go mod download

COPY ./pkg ./pkg

ARG VERSION=dev
ARG GITHASH=unknown
ARG BUILD_DATE=unknown
ARG TARGETOS=linux
ARG TARGETARCH

# Both modules are pure Go (modernc.org/sqlite): no C toolchain needed, and
# the binaries are static.
ENV CGO_ENABLED=0

RUN set -e; \
    if [ "$TARGETARCH" = "arm" ]; then export GOARM=7; fi; \
    export GOOS=$TARGETOS GOARCH=$TARGETARCH; \
    LDF="-s -w -X 'main.version=${VERSION}' -X 'main.githash=${GITHASH}' -X 'main.buildstamp=${BUILD_DATE}'"; \
    cd /build/pkg/main && go build -trimpath -ldflags="$LDF" -o /build/go_media_downloader main.go; \
    cd /build/pkg/imdb && go build -trimpath -ldflags="$LDF" -o /build/init_imdb imdb.go

FROM scratch AS prebuilt
ARG TARGETARCH
COPY go_media_downloader-linux-${TARGETARCH} /build/go_media_downloader
COPY init_imdb-linux-${TARGETARCH} /build/init_imdb

FROM ${BINARIES} AS binaries

# Runtime stage
FROM debian:trixie-slim

LABEL org.opencontainers.image.authors="Kellerman81 <Kellerman81@gmail.com>"

# unrar is only in Debian's non-free component, which trixie-slim doesn't
# enable by default. p7zip-full is a transitional package for 7zip in trixie
# but still provides /usr/bin/7za, which existing configs point at.
RUN sed -i 's/^Components: main$/Components: main non-free/' /etc/apt/sources.list.d/debian.sources && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ffmpeg \
        mediainfo \
        flac \
        p7zip-full \
        tzdata \
        libchromaprint-tools \
        unzip unrar \
        ca-certificates && \
    # GPU drivers for the Library player's player_hwaccel: VA-API for AMD
    # (all platforms), Intel VA-API and Quick Sync on amd64 only.
    apt-get install -y --no-install-recommends mesa-va-drivers && \
    if [ "$(dpkg --print-architecture)" = "amd64" ]; then \
        apt-get install -y --no-install-recommends intel-media-va-driver-non-free libmfx-gen1.2 libvpl2; \
    fi && \
    rm -rf /var/lib/apt/lists/*

RUN ln -fs /usr/share/zoneinfo/Europe/Berlin /etc/localtime && \
    dpkg-reconfigure --frontend noninteractive tzdata

# UID/GID 999, same as the bookworm image, so existing volumes stay writable.
RUN groupadd -r appuser && useradd -r -g appuser appuser

RUN mkdir -p /app/config /app/databases /app/logs /app/backup /app/temp /app/docs /app/schema /app/static && \
    chown -R appuser:appuser /app

WORKDIR /app

COPY --from=binaries --chmod=755 /build/go_media_downloader /build/init_imdb /app/

COPY --chown=appuser:appuser ./config/ /app/config/
COPY --chown=appuser:appuser ./databases/ /app/databases/
COPY --chown=appuser:appuser ./docs/ /app/docs/
COPY --chown=appuser:appuser ./schema/ /app/schema/
COPY --chown=appuser:appuser ./static/ /app/static/
COPY --chown=appuser:appuser ./LICENSE ./README.md /app/

USER appuser

VOLUME ["/app/databases", "/app/config", "/app/logs"]
EXPOSE 9090
CMD ["/app/go_media_downloader"]

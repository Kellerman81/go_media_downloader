# Where the binaries come from: "builder" compiles them here, "prebuilt"
# takes go_media_downloader-linux-<arch> / init_imdb-linux-<arch> from the
# build context (CI passes the binaries its build job already made).
ARG BINARIES=builder

# Build stage
FROM golang:1.26.10-bookworm AS builder

LABEL org.opencontainers.image.authors="Kellerman81 <Kellerman81@gmail.com>"

# Install build dependencies
RUN apt-get update && \
    apt-get -y install clang cmake gcc-i686-linux-gnu gcc-aarch64-linux-gnu gcc-arm-linux-gnueabihf gcc-mingw-w64
RUN dpkg-reconfigure --frontend noninteractive tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY ./pkg/main/go.mod ./pkg/main/go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG GITHASH=unknown
ARG BUILD_DATE=unknown
ARG TARGETOS
ARG TARGETARCH
ARG SETGCC=gcc
ARG SETGOARM

ENV CGO_ENABLED=1

RUN if [ "$TARGETARCH" = "arm64" ]; then \
        SETGCC=aarch64-linux-gnu-gcc; \
    elif [ "$TARGETARCH" = "armhf" ]; then \
        SETGCC=arm-linux-gnueabihf-gcc; \
        SETGOARM=7; \
    elif [ "$TARGETOS" = "windows" ]; then \
        SETGCC=x86_64-w64-mingw32-gcc; \
    fi;
    
# Build the applications
RUN cd ./pkg/main/ && \
    CC=${SETGCC} GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${SETGOARM} go build \
    -ldflags="-s -w -X 'main.version=${VERSION}' -X 'main.githash=${GITHASH}' -X 'main.buildstamp=${BUILD_DATE}'" \
    -o ../../go_media_downloader main.go

RUN cd ./pkg/imdb/ && \
    CC=${SETGCC} GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${SETGOARM} go build \
    -ldflags="-s -w -X 'main.version=${VERSION}' -X 'main.githash=${GITHASH}' -X 'main.buildstamp=${BUILD_DATE}'" \
    -o ../../init_imdb imdb.go

FROM scratch AS prebuilt
ARG TARGETARCH
COPY go_media_downloader-linux-${TARGETARCH} /build/go_media_downloader
COPY init_imdb-linux-${TARGETARCH} /build/init_imdb

FROM ${BINARIES} AS binaries

# Runtime stage
FROM debian:bookworm-slim

LABEL org.opencontainers.image.authors="Kellerman81 <Kellerman81@gmail.com>"

# Install runtime dependencies. unrar is only in Debian's non-free component,
# which bookworm-slim doesn't enable by default.
RUN sed -i 's/^Components: main$/Components: main non-free/' /etc/apt/sources.list.d/debian.sources && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ffmpeg \
        mediainfo \
        flac \
        p7zip \
        p7zip-full \
        graphviz \
        gv \
        tzdata \
        libchromaprint-tools \
        unzip unrar \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Set timezone
RUN ln -fs /usr/share/zoneinfo/Europe/Berlin /etc/localtime && \
    dpkg-reconfigure --frontend noninteractive tzdata

# Create app user for security
RUN groupadd -r appuser && useradd -r -g appuser appuser

# Create application directories
RUN mkdir -p /app/{config,databases,logs,backup,temp,docs,schema,static} && \
    chown -R appuser:appuser /app

WORKDIR /app

# Copy built binaries from builder stage
COPY --from=binaries /build/go_media_downloader /build/init_imdb /app/

# Copy application files
COPY --chown=appuser:appuser ./config/ /app/config/
COPY --chown=appuser:appuser ./databases/ /app/databases/
COPY --chown=appuser:appuser ./docs/ /app/docs/
COPY --chown=appuser:appuser ./schema/ /app/schema/
COPY --chown=appuser:appuser ./static/ /app/static/
COPY --chown=appuser:appuser ./LICENSE /app/LICENSE
COPY --chown=appuser:appuser ./README.md /app/README.md

# Make binaries executable
RUN chmod +x /app/go_media_downloader /app/init_imdb

# Switch to non-root user
USER appuser

VOLUME ["/app/databases", "/app/config", "/app/logs"]
# Expose port 9090 to the outside world
EXPOSE 9090
CMD ["/app/go_media_downloader"]
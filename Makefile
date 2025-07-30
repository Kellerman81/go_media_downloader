# Binary names
BINARY_NAME = go_media_downloader
IMDB_BINARY_NAME = init_imdb

# Build metadata
BUILD_DATE ?= $(shell date +'%Y-%m-%d')
GITHASH ?= $(shell git rev-parse --short HEAD)
VERSION ?= $(shell git describe --exclude latest_develop)

# Build flags
LDFLAGS := -s -w
LDFLAGS += -X 'main.version=${VERSION}'
LDFLAGS += -X 'main.githash=${GITHASH}'
LDFLAGS += -X 'main.buildstamp=${BUILD_DATE}'

# Go build settings
export CGO_ENABLED := 1

# Cross-compilation settings
LINUX_AMD64_CC = gcc
LINUX_ARM64_CC = aarch64-linux-gnu-gcc
LINUX_ARM_CC = arm-linux-gnueabihf-gcc
WINDOWS_AMD64_CC = x86_64-w64-mingw32-gcc

GOARM = "7"

# Targets
.PHONY: help clean buildmain buildimdb build-all

help:
	@echo "Available targets:"
	@echo "  buildmain    - Build main application for all platforms"
	@echo "  buildimdb    - Build IMDB utility for all platforms"
	@echo "  build-all    - Build both applications for all platforms"
	@echo "  clean        - Clean build artifacts"

clean:
	@echo "Cleaning build artifacts..."
	rm -f ${BINARY_NAME}-*
	rm -f ${IMDB_BINARY_NAME}-*
	rm -f *.zip

buildmain:
	@echo "Building main application (${VERSION}) - ${BUILD_DATE}"
	cd pkg/main && go mod tidy
	
	# Linux AMD64
	cd pkg/main && \
		CC=${LINUX_AMD64_CC} GOARCH=amd64 GOOS=linux \
		go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-linux-amd64 main.go
	
	# Linux ARM64
	cd pkg/main && \
		CC=${LINUX_ARM64_CC} GOARCH=arm64 GOOS=linux \
		go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-linux-arm64 main.go
		
	# Linux ARM7
	cd pkg/main && \
		CC=${LINUX_ARM_CC} GOARCH=arm GOOS=linux GOARM=${GOARM} \
		go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-linux-arm7 main.go
	
	# Windows AMD64
	cd pkg/main && \
		CC=${WINDOWS_AMD64_CC} GOARCH=amd64 GOOS=windows \
		go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-windows-amd64.exe main.go

buildimdb:
	@echo "Building IMDB utility (${VERSION}) - ${BUILD_DATE}"
	cd pkg/imdb && go mod tidy
	
	# Linux AMD64
	cd pkg/imdb && \
		CC=${LINUX_AMD64_CC} GOARCH=amd64 GOOS=linux \
		go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-linux-amd64 imdb.go
	
	# Linux ARM64
	cd pkg/imdb && \
		CC=${LINUX_ARM64_CC} GOARCH=arm64 GOOS=linux \
		go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-linux-arm64 imdb.go

	# Linux ARM7
	cd pkg/imdb && \
		CC=${LINUX_ARM_CC} GOARCH=arm GOOS=linux GOARM=${GOARM} \
		go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-linux-arm7 imdb.go
	
	# Windows AMD64
	cd pkg/imdb && \
		CC=${WINDOWS_AMD64_CC} GOARCH=amd64 GOOS=windows \
		go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-windows-amd64.exe imdb.go

build-all: buildmain buildimdb
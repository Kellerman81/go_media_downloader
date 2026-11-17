# Binary names
BINARY_NAME = go_media_downloader
IMDB_BINARY_NAME = init_imdb
PLAYER_BINARY_NAME = go_media_player

# Build metadata
BUILD_DATE ?= $(shell date +'%Y-%m-%d')
GITHASH ?= $(shell git rev-parse --short HEAD)
VERSION ?= $(shell git describe --exclude latest_develop)

# Build flags
LDFLAGS := -s -w
LDFLAGS += -X 'main.version=${VERSION}'
LDFLAGS += -X 'main.githash=${GITHASH}'
LDFLAGS += -X 'main.buildstamp=${BUILD_DATE}'

# swaggo/swag (go install github.com/swaggo/swag/cmd/swag@v1.16.4). Taken from
# GOPATH/bin so a different "swag" earlier in PATH can't shadow it.
SWAG ?= $(shell go env GOPATH)/bin/swag

# Go build settings. Both modules are pure Go (modernc.org/sqlite), so every
# target cross-compiles without a C toolchain and the Linux binaries are static.
export CGO_ENABLED := 0

GOARM = "7"

# Targets
.PHONY: help clean buildmain buildimdb buildplayer build-all swagger

help:
	@echo "Available targets:"
	@echo "  buildmain    - Build main application for all platforms"
	@echo "  buildimdb    - Build IMDB utility for all platforms"
	@echo "  buildplayer  - Build the desktop player (Linux, Windows)"
	@echo "  build-all    - Build all applications for all platforms"
	@echo "  swagger      - Regenerate docs/swagger.json and swagger.yaml from the code annotations"
	@echo "  clean        - Clean build artifacts"

clean:
	@echo "Cleaning build artifacts..."
	rm -f ${BINARY_NAME}-*
	rm -f ${IMDB_BINARY_NAME}-*
	rm -f ${PLAYER_BINARY_NAME}-*
	rm -f *.zip

buildmain:
	@echo "Building main application (${VERSION}) - ${BUILD_DATE}"
	cd pkg/main && go mod tidy
	
	# Linux AMD64
	cd pkg/main && \
		GOARCH=amd64 GOOS=linux \
		go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-linux-amd64 main.go
	
	# Linux ARM64
	cd pkg/main && \
		GOARCH=arm64 GOOS=linux \
		go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-linux-arm64 main.go
		
	# Linux ARM7
	cd pkg/main && \
		GOARCH=arm GOOS=linux GOARM=${GOARM} \
		go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-linux-arm7 main.go
	
	# Windows AMD64
	cd pkg/main && \
		GOARCH=amd64 GOOS=windows \
		go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-windows-amd64.exe main.go

buildimdb:
	@echo "Building IMDB utility (${VERSION}) - ${BUILD_DATE}"
	cd pkg/imdb && go mod tidy
	
	# Linux AMD64
	cd pkg/imdb && \
		GOARCH=amd64 GOOS=linux \
		go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-linux-amd64 imdb.go
	
	# Linux ARM64
	cd pkg/imdb && \
		GOARCH=arm64 GOOS=linux \
		go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-linux-arm64 imdb.go

	# Linux ARM7
	cd pkg/imdb && \
		GOARCH=arm GOOS=linux GOARM=${GOARM} \
		go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-linux-arm7 imdb.go
	
	# Windows AMD64
	cd pkg/imdb && \
		GOARCH=amd64 GOOS=windows \
		go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-windows-amd64.exe imdb.go

# The desktop player: opens the portal's end-user interface in an app
# window. Standard library only (no go mod tidy needed); the Windows build
# is a GUI program without a console window.
buildplayer:
	@echo "Building desktop player (${VERSION}) - ${BUILD_DATE}"

	cd pkg/player && GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -o ../../${PLAYER_BINARY_NAME}-linux-amd64 .
	cd pkg/player && GOARCH=arm64 GOOS=linux go build -ldflags="-s -w" -o ../../${PLAYER_BINARY_NAME}-linux-arm64 .
	cd pkg/player && GOARCH=amd64 GOOS=windows go build -ldflags="-s -w -H=windowsgui" -o ../../${PLAYER_BINARY_NAME}-windows-amd64.exe .

build-all: buildmain buildimdb buildplayer

swagger:
	cd pkg/main && $(SWAG) init -g main.go -o ../../docs --outputTypes json,yaml --parseDependency --propertyStrategy pascalcase
	# swagger.js lets docs/index.html work when opened as a file (browsers block fetch on file://)
	{ printf 'window.SWAGGER_SPEC = '; cat docs/swagger.json; printf ';\n'; } > docs/swagger.js

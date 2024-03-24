BINARY_NAME = go_media_downloader
IMDB_BINARY_NAME = init_imdb

BUILD_DATE ?= $(shell date +'%Y-%m-%d')

ifndef GITHASH
	GITHASH = $(shell git rev-parse --short HEAD)
endif
ifndef VERSION
	VERSION = $(shell git describe --tags --exclude latest_develop)
endif

LDFLAGS := $(LDFLAGS)
$(eval LDFLAGS += -s -w)
$(eval LDFLAGS += -X 'main.version=${VERSION}')
$(eval LDFLAGS += -X 'main.githash=${GITHASH}')
$(eval LDFLAGS += -X 'main.buildstamp=${BUILD_DATE}')

export CGO_ENABLED := 1

help:
	@echo "Please use \`make <target>' where <target> is one of the following: buildmain, buildimdb"

buildmain:
	cd pkg/main && echo "building main ${BUILD_DATE}  ${LDFLAGS}" && \
		go get && \
		GOARCH=amd64 GOOS=linux go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-linux-amd64 main.go && \
		CC=x86_64-w64-mingw32-gcc GOARCH=amd64 GOOS=windows go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-windows-amd64.exe main.go
#CC=i686-linux-gnu-gcc GOARCH=386 GOOS=linux go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-linux-386 main.go
#CC=i686-w64-mingw32-gcc GOARCH=386 GOOS=windows go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-windows-386.exe main.go
#GOARCH=amd64 GOOS=darwin go build -ldflags="${LDFLAGS}" -o ../../${BINARY_NAME}-darwin-amd64 main.go && \
		
buildimdb:
	cd pkg/imdb && echo "building imdb ${BUILD_DATE}" && \
		go get && \
		GOARCH=amd64 GOOS=linux go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-linux-amd64 imdb.go && \
		CC=x86_64-w64-mingw32-gcc GOARCH=amd64 GOOS=windows go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-windows-amd64.exe imdb.go
#CC=i686-linux-gnu-gcc GOARCH=386 GOOS=linux go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-linux-386 imdb.go
#CC=i686-w64-mingw32-gcc GOARCH=386 GOOS=windows go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-windows-386.exe imdb.go
#GOARCH=amd64 GOOS=darwin go build -ldflags="${LDFLAGS}" -o ../../${IMDB_BINARY_NAME}-darwin-amd64 imdb.go && \
		
GIT_VERSION=$(shell git describe --dirty || echo dev)

GIT_VERSION=$(shell git describe --dirty)
CURRENT_TIME=$(shell date +%Y%m%d%H%M%S)

LD_VERSION_FLAGS=-X main.BuildVersion=$(GIT_VERSION) -X main.BuildTime=$(CURRENT_TIME)
LDFLAGS=-ldflags "$(LD_VERSION_FLAGS)"

BINARIES=itzo-launcher itzo-launcher-arm64

TOP_DIR=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))
CMD_SRC=$(shell find $(TOP_DIR)cmd -type f -name '*.go')
PKG_SRC=$(shell find $(TOP_DIR)pkg -type f -name '*.go')

all: $(BINARIES)

itzo-launcher: $(CMD_SRC) $(PKG_SRC) go.sum
	go build $(LDFLAGS) -o itzo-launcher cmd/itzo-launcher/itzo-launcher.go

itzo-launcher-arm64: $(CMD_SRC) $(PKG_SRC) go.sum
	GOARCH=arm64 GOOS=linux go build $(LDFLAGS) -o itzo-launcher-arm64 cmd/itzo-launcher/itzo-launcher.go

clean:
	rm -f $(BINARIES)

.PHONY: all clean install

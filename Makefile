GIT_VERSION=$(shell git describe --dirty || echo dev)

LDFLAGS=-ldflags "-X main.VERSION=$(GIT_VERSION)"

BINARIES=itzo-launcher

TOP_DIR=$(dir $(realpath $(firstword $(MAKEFILE_LIST))))
CMD_SRC=$(shell find $(TOP_DIR)cmd -type f -name '*.go')
#PKG_SRC=$(shell find $(TOP_DIR)pkg -type f -name '*.go')

all: $(BINARIES)

itzo-launcher: $(CMD_SRC) go.sum
	go build $(LDFLAGS) -o itzo-launcher cmd/itzo-launcher/itzo-launcher.go

clean:
	rm -f $(BINARIES)

.PHONY: all clean install

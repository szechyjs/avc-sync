BINARY_NAME     := avc-sync
VERSION         ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: all build test clean

all: build

build:
	CGO_ENABLED=1 go build -ldflags="-X main.version=$(VERSION)" -o $(BINARY_NAME) .

test:
	CGO_ENABLED=1 go test ./...

clean:
	rm -rf $(BINARY_NAME)

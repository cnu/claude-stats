BINARY := claude-stats
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/cnu/claude-stats/internal/cli.Version=$(VERSION) -X github.com/cnu/claude-stats/internal/cli.BuildDate=$(BUILD_DATE)"

.PHONY: build test lint install clean hooks

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/claude-stats

test:
	go test ./... -v

lint:
	golangci-lint run ./...

install:
	go install $(LDFLAGS) ./cmd/claude-stats

clean:
	rm -f $(BINARY)

hooks:
	git config core.hooksPath .githooks

ROOT_DIR := $(abspath ..)
PACKAGE_VERSION ?= $(shell tr -d '[:space:]' < "$(ROOT_DIR)/VERSION")
GIT_SHA ?= $(shell git -C "$(ROOT_DIR)" rev-parse HEAD 2>/dev/null || echo unknown)
GIT_DIRTY ?= $(shell git -C "$(ROOT_DIR)" diff --quiet --ignore-submodules HEAD -- 2>/dev/null && echo clean || echo dirty)
GO_DAEMON_LDFLAGS := -X main.daemonVersion=$(PACKAGE_VERSION) -X main.buildGitSHA=$(GIT_SHA) -X main.buildDirty=$(GIT_DIRTY)
GO_CLI_LDFLAGS := -X main.version=$(PACKAGE_VERSION)
GO_HELPER_LDFLAGS := -X main.version=$(PACKAGE_VERSION)

.PHONY: build build-helper verify test test-helper helper-check lint clean release release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

DAEMON_BIN := bin/striatumd
CLI_BIN := bin/striatum
HELPER_BIN := bin/striatum-supervisor-helper

build:
	mkdir -p bin
	go build -ldflags "$(GO_CLI_LDFLAGS)" -o $(CLI_BIN) ./cmd/striatum
	go build -ldflags "$(GO_DAEMON_LDFLAGS)" -o $(DAEMON_BIN) ./cmd/striatumd
	go build -ldflags "$(GO_HELPER_LDFLAGS)" -o $(HELPER_BIN) ./cmd/striatum-supervisor-helper

build-helper:
	mkdir -p bin
	go build -ldflags "$(GO_HELPER_LDFLAGS)" -o $(HELPER_BIN) ./cmd/striatum-supervisor-helper

verify:
	go mod verify
	go mod tidy -diff

test:
	go test ./...

test-helper:
	go test ./cmd/striatum-supervisor-helper ./pkg/supervisor

helper-check: verify
	$(MAKE) build-helper
	$(MAKE) test-helper

lint:
	go vet ./...

clean:
	rm -rf bin

# Cross-compile release targets for Go-only binary archives.
release-linux-amd64:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(GO_CLI_LDFLAGS)" -o bin/striatum-linux-amd64 ./cmd/striatum
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(GO_DAEMON_LDFLAGS)" -o bin/striatumd-linux-amd64 ./cmd/striatumd
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(GO_HELPER_LDFLAGS)" -o bin/striatum-supervisor-helper-linux-amd64 ./cmd/striatum-supervisor-helper

release-linux-arm64:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(GO_CLI_LDFLAGS)" -o bin/striatum-linux-arm64 ./cmd/striatum
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(GO_DAEMON_LDFLAGS)" -o bin/striatumd-linux-arm64 ./cmd/striatumd
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(GO_HELPER_LDFLAGS)" -o bin/striatum-supervisor-helper-linux-arm64 ./cmd/striatum-supervisor-helper

release-darwin-amd64:
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(GO_CLI_LDFLAGS)" -o bin/striatum-darwin-amd64 ./cmd/striatum
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(GO_DAEMON_LDFLAGS)" -o bin/striatumd-darwin-amd64 ./cmd/striatumd
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(GO_HELPER_LDFLAGS)" -o bin/striatum-supervisor-helper-darwin-amd64 ./cmd/striatum-supervisor-helper

release-darwin-arm64:
	mkdir -p bin
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(GO_CLI_LDFLAGS)" -o bin/striatum-darwin-arm64 ./cmd/striatum
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(GO_DAEMON_LDFLAGS)" -o bin/striatumd-darwin-arm64 ./cmd/striatumd
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(GO_HELPER_LDFLAGS)" -o bin/striatum-supervisor-helper-darwin-arm64 ./cmd/striatum-supervisor-helper

release: release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

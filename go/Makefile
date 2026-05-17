.PHONY: build build-helper verify test test-helper helper-check lint clean release release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

DAEMON_BIN := bin/striatumd
HELPER_BIN := bin/striatum-supervisor-helper

build:
	mkdir -p bin
	go build -o $(DAEMON_BIN) ./cmd/striatumd
	go build -o $(HELPER_BIN) ./cmd/striatum-supervisor-helper

build-helper:
	mkdir -p bin
	go build -o $(HELPER_BIN) ./cmd/striatum-supervisor-helper

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

# RFC 0039 Phase 2 Step 6: cross-compile targets. Each produces
# bin/striatumd-<os>-<arch> for the wheel package-data path.
release-linux-amd64:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/striatumd-linux-amd64 ./cmd/striatumd
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/striatum-supervisor-helper-linux-amd64 ./cmd/striatum-supervisor-helper

release-linux-arm64:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/striatumd-linux-arm64 ./cmd/striatumd
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/striatum-supervisor-helper-linux-arm64 ./cmd/striatum-supervisor-helper

release-darwin-amd64:
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o bin/striatumd-darwin-amd64 ./cmd/striatumd
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o bin/striatum-supervisor-helper-darwin-amd64 ./cmd/striatum-supervisor-helper

release-darwin-arm64:
	mkdir -p bin
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bin/striatumd-darwin-arm64 ./cmd/striatumd
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bin/striatum-supervisor-helper-darwin-arm64 ./cmd/striatum-supervisor-helper

release: release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

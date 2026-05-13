.PHONY: build test lint clean release release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

BIN := bin/striatumd

build:
	mkdir -p $(dir $(BIN))
	go build -o $(BIN) ./cmd/striatumd

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin

# RFC 0039 Phase 2 Step 6: cross-compile targets. Each produces
# bin/striatumd-<os>-<arch> for the wheel package-data path.
release-linux-amd64:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/striatumd-linux-amd64 ./cmd/striatumd

release-linux-arm64:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bin/striatumd-linux-arm64 ./cmd/striatumd

release-darwin-amd64:
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o bin/striatumd-darwin-amd64 ./cmd/striatumd

release-darwin-arm64:
	mkdir -p bin
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o bin/striatumd-darwin-arm64 ./cmd/striatumd

release: release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

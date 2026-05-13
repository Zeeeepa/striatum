.PHONY: build test lint clean

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

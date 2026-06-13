ROOT_DIR := $(abspath ..)
PACKAGE_VERSION ?= $(shell tr -d '[:space:]' < "$(ROOT_DIR)/VERSION")
GIT_SHA ?= $(shell git -C "$(ROOT_DIR)" rev-parse HEAD 2>/dev/null || echo unknown)
GIT_DIRTY ?= $(shell git -C "$(ROOT_DIR)" diff --quiet --ignore-submodules HEAD -- 2>/dev/null && echo clean || echo dirty)
GO_DAEMON_LDFLAGS := -X main.daemonVersion=$(PACKAGE_VERSION) -X main.buildGitSHA=$(GIT_SHA) -X main.buildDirty=$(GIT_DIRTY)
GO_CLI_LDFLAGS := -X main.version=$(PACKAGE_VERSION)
GO_HELPER_LDFLAGS := -X main.version=$(PACKAGE_VERSION)

.PHONY: build build-helper verify test race vet test-helper helper-check lint lint-tools coverage cover-floor check check-tests installed-cli-check clean release release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

DAEMON_BIN := bin/striatumd
CLI_BIN := bin/striatum
HELPER_BIN := bin/striatum-supervisor-helper
CORE_COVERAGE_FLOOR ?= 20.0
# Core packages whose statement coverage is gated by CORE_COVERAGE_FLOOR. The
# CSV form feeds `go test -coverpkg` so a single instrumented run measures their
# coverage from the whole suite.
COVERAGE_PKGS := ./pkg/admin ./pkg/db ./pkg/mutations ./pkg/rpc ./pkg/reads ./pkg/recovery ./pkg/supervisor ./pkg/workflowauthoring ./pkg/workflowgenerate
empty :=
space := $(empty) $(empty)
comma := ,
COVERAGE_PKGS_CSV := $(subst $(space),$(comma),$(strip $(COVERAGE_PKGS)))
# golangci-lint is pinned so the local dev-loop lint matches CI exactly. A
# missing binary previously let a red CI lint go invisible locally (main sat
# red over two errcheck findings); `make lint-tools` installs the pin and
# `lint` refuses with that hint when the binary is absent.
GOLANGCI_LINT_VERSION ?= v2.12.2
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo "$(shell go env GOPATH)/bin/golangci-lint")

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
	go test -timeout 30m ./...

race:
	go test -race -timeout 30m ./...

vet:
	go vet ./...

test-helper:
	go test ./cmd/striatum-supervisor-helper ./pkg/supervisor

helper-check: verify
	$(MAKE) build-helper
	$(MAKE) test-helper

lint-tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint:
	@test -x "$(GOLANGCI_LINT)" || { echo "golangci-lint not found at '$(GOLANGCI_LINT)'; run 'make -C go lint-tools' to install the pinned $(GOLANGCI_LINT_VERSION)"; exit 1; }
	"$(GOLANGCI_LINT)" run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./...

coverage:
	go test -timeout 30m -coverprofile=coverage.out $(COVERAGE_PKGS)
	@$(MAKE) cover-floor

# check-tests is the single test pass the CI gate runs: race detection AND core
# coverage in one instrumented run over the whole suite, replacing the separate
# `test`, `race`, and `coverage` passes that each re-ran the PostgreSQL-heavy
# suite (three runs of the slowest packages). -count=1 disables Go's test-result
# cache so a warm build cache never serves a stale pass for a PG-backed test.
check-tests:
	go test -race -count=1 -timeout 30m -coverprofile=coverage.out -coverpkg=$(COVERAGE_PKGS_CSV) ./...
	@$(MAKE) cover-floor

# cover-floor gates an existing coverage.out against CORE_COVERAGE_FLOOR. Shared
# by `coverage` and `check-tests` so the floor logic lives in one place.
cover-floor:
	@total="$$(go tool cover -func=coverage.out | awk '/^total:/ {print substr($$3, 1, length($$3)-1)}')"; \
	awk -v got="$$total" -v want="$(CORE_COVERAGE_FLOOR)" 'BEGIN { if (got + 0 < want + 0) { printf "core coverage %.1f%% below floor %.1f%%\n", got, want; exit 1 } printf "core coverage %.1f%% >= %.1f%%\n", got, want }'

# check is the full local gate. CI runs the same work split across two parallel
# jobs (vet+lint, and verify+check-tests) for wall-clock — see .github/workflows/ci.yml.
check: verify vet lint check-tests

installed-cli-check:
	@test -n "$$STRIATUM_PG_TEST_URL" || { echo "STRIATUM_PG_TEST_URL is required for installed-cli-check" >&2; exit 1; }
	@command -v agy >/dev/null || { echo "agy is required on PATH for installed-cli-check" >&2; exit 1; }
	STRIATUM_P3_INSTALLED_CLI=1 go test ./pkg/adapterconformance -run '^TestInstalledCLISeatAgyTwoTurn$$' -count=1 -v

clean:
	rm -rf bin coverage.out

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

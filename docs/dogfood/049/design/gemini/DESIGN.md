author: designer-unknown-model-001

# DESIGN: RFC 0039 Phase 2 — Go Daemon Core Steps 3-6

This design details the transition of the Striatum daemon from Python to Go for the mutation surface and process supervision, completing Phase 2 of RFC 0039.

## Track A — CLI Integration + Mutating Verbs

### 1. CLI Entry Integration
The Python CLI will be updated to support selecting the Go daemon core.

*   **`src/striatum/cli/parser.py`**: Update `build_parser()` to add `--core {python,go}` to the `daemon start` subparser.
*   **`src/striatum/cli/dispatch.py`**: Update `_dispatch_daemon()` to handle `args.core == "go"`.
*   **Binary Resolution**: `src/striatum/cli/daemon_go_launcher.py` (new) will resolve the `striatumd` binary by checking:
    1.  `STRIATUMD_GO_BIN` env var.
    2.  `src/striatum/_daemongo/bin/striatumd-<platform>` (bundled).
    3.  `go/bin/striatumd` (dev fallback).

### 2. Go Mutation Registry
The Go daemon must implement the full mutation surface defined in RFC 0043 §5.

*   **`go/pkg/rpc/registry.go`**: Update `methodEntries` to include dotted canonical names (e.g., `work.ack`, `review.verdict`) and their required capabilities (e.g., `CapabilityClaim`, `CapabilityReview`).
*   **`go/pkg/rpc/server.go`**: Register handlers for these methods in `NewServer()`.

### 3. Apply Service
*   **`go/pkg/apply/service.go`**: New service mirroring `src/striatum/daemon_apply/service.py`. Implements `AuthorizeApply(session_id, verdict_id)` which validates the audit chain and Postgres state before allowing a repo-write operation.
*   **`go/pkg/apply/receipt.go`**: Mirror `src/striatum/daemon_apply/receipt.py` schemas.

### 4. MCP Capabilities
*   **`go/pkg/mcp/tools.go`**: Implement `tools/list` and `tools/call`.
*   **`go/pkg/mcp/capabilities.go`**: Gate tool access based on the `CapabilityToken` in the RPC envelope, mirroring `src/striatum/daemon_rpc/mcp.py`. Every mutation must record a row in the audit log via `AuditRecorder.RecordRPC`.

### 5. Cross-Repo Lifecycle
*   **`go/pkg/crossrepo/lifecycle.go`**: Implement cross-repo run state machine transitions, mirroring `src/striatum/daemon_rpc/multi_repo.py`.
*   **`go/pkg/crossrepo/prepare.go`**: Implement `run.prepare` for cross-repo workflows.

## Track B — Supervisor + Distribution + CI

### 1. Go Supervisor Implementation
*   **`go/pkg/supervisor/pty.go`**: Uses `github.com/creack/pty` to launch supervised lanes. Mirror `supervise_start` logic from `src/striatum/supervisor.py`.
*   **`go/pkg/supervisor/fifo.go`**: Implements the byte-compatible NDJSON packet delivery to the lane's stdin FIFO, mirroring `supervise_send`.
*   **`go/pkg/supervisor/liveness.go`**: Track pid liveness and heartbeat via supervised progress signals.
*   **Signal Handling**: `go/cmd/striatumd/main.go` will catch `SIGTERM` and use a `sync.WaitGroup` to wait for all supervised workers in `go/pkg/supervisor/manager.go` to cleanly terminate and record `supervisor.stopped` events.

### 2. Distribution & Wheel Packaging
*   **`go/Makefile`**: Add cross-compilation targets:
    ```makefile
    platforms := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64
    release: $(platforms)
    linux-amd64:
    	GOOS=linux GOARCH=amd64 go build -o bin/striatumd-linux-amd64 ./cmd/striatumd
    ```
*   **`src/striatum/_daemongo/`**: New package acting as the binary carrier.
*   **`pyproject.toml`**: Include `src/striatum/_daemongo/bin/*` in `package-data`.

### 3. CI Integration
*   **.github/workflows/ci.yml**: Update the test matrix:
    ```yaml
    strategy:
      matrix:
        core: [python, go]
    env:
      STRIATUM_DAEMON_CORE: ${{ matrix.core }}
    ```
*   Run `make test-multi-repo` for both cores.

## Verification Plan
1.  **Go Unit Tests**: `go test ./go/pkg/...`.
2.  **Audit Chain Audit**: `tests/test_daemon_go_audit.py` (existing) expanded to verify every new mutation generates a validly hashed audit row.
3.  **Supervisor Smoke**: `tests/test_daemon_go_supervisor.py` (new) verifying start/stop/send/signal-cleanup via the Go core.
4.  **Cross-Repo E2E**: `tests/test_cross_repo_lifecycle_e2e.py` with `CORE=go`.

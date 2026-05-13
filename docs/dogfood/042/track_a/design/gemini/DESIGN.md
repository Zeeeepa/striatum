# Design: Go Daemon Core (Phase 1, Steps 1+2)

author: designer-gemini-pro-002

This document specifies the implementation design for Phase 1 (Steps 1 and 2) of the Go-language daemon core rewrite, as proposed in RFC 0039.

## 1. Project Skeleton & Module

### 1.1 Go Module
- **Path**: `go/`
- **Module Name**: `github.com/halbritt/striatum/go`
- **Go Version**: 1.23
- **Dependencies**:
    - `github.com/jackc/pgx/v5`: PostgreSQL driver and connection pooling.
    - `github.com/google/uuid`: RFC 0030 `request_id` (UUIDv7) and internal identifiers.
    - Standard Library: `net`, `os`, `encoding/json`, `crypto/sha256`, `embed`.

### 1.2 Layout
```
go/
├── cmd/
│   └── striatumd/
│       └── main.go       # Entry point: flag parsing, signal handling, boot
├── pkg/
│   ├── rpc/
│   │   ├── envelope.go   # RFC 0030 JSON-RPC v1 framing
│   │   ├── registry.go   # Method registration and dispatch
│   │   ├── capability.go # Capability-bound authorization logic
│   │   └── server.go     # Unix socket listener and session loop
│   └── db/
│       ├── connection.go # pgxpool management and connection lifecycle
│       ├── migrations.go # Embedded SQL migration runner
│       └── audit.go      # Audit-chain hashing and append logic
├── go.mod
├── go.sum
└── Makefile              # Go-specific build and test targets
```

## 2. Step 1: RPC Skeleton & envelope-v1

### 2.1 Server Lifecycle
`striatumd` (Go) will implement a Unix domain socket listener at the path specified by the operator (defaulting to `${XDG_RUNTIME_DIR}/striatum/striatumd.sock`).
- **Permissions**: Enforce `0600` on the socket file.
- **Signals**: Handle `SIGTERM` and `SIGINT` for graceful shutdown, closing the listener and draining the `pgxpool`.
- **Concurrency**: Each connection is handled in a separate goroutine.

### 2.2 Wire Protocol (envelope-v1)
`go/pkg/rpc/envelope.go` defines the JSON structures for RFC 0030:
- `Request`: `schema_version`, `request_id`, `method`, `params`, `capability_token`, `deadline_ms`.
- `Response`: `schema_version`, `request_id`, `ok`, `data`, `error`, `audit_id`.

**Deterministic JSON**: To ensure audit-chain integrity, the daemon must serialize the `material` dictionary (see §3.3) into a canonical JSON format matching the Python implementation:
- `sort_keys=True`
- `separators=(",", ":")` (no whitespace).

### 2.3 Method Registry & Handshaking
- **Handshake**: Implement `daemon.hello` and `daemon.welcome`.
    - `daemon.welcome` returns the substrate type (`postgres`), current schema version, and `methods_etag`.
- **Exposition**: Implement `daemon.describe`.
    - Returns the list of registered methods, their required capabilities, and parameter schemas.
- **Capabilities**: Support the full V2 vocabulary: `read`, `write`, `review`, `claim`, `apply`, `admin`, `recovery`, `surgical_recovery`.
- **Registry**: A central map of `method -> Handler` where a `Handler` encapsulates:
    - Required capability.
    - Repository scope requirements.
    - Execution logic.

Step 1 implements **read-only verbs** only:
- `daemon.status`
- `daemon.health`
- `daemon.describe`
- `daemon.audit`
- `repo.list`

## 3. Step 2: Postgres Substrate

### 3.1 Connection Management
`go/pkg/db/connection.go` manages a `*pgxpool.Pool`.
- Connection string read from `STRIATUM_DAEMON_DB_URL`.
- Pool configuration tuned for local-first concurrency (max connections, idle timeouts).

### 3.2 Migration Runner
`go/pkg/db/migrations.go` will embed the SQL files from `src/striatum/daemon_pg/sql/` using `//go:embed`.
- At startup, the daemon checks the `striatumd.schema_meta` and `striatumd.schema_migrations` tables.
- If migrations are missing, it applies them sequentially.
- If the on-disk schema is *newer* than the binary, the daemon refuses to start (Exit 9).

### 3.3 Audit-Chain Integrity
`go/pkg/db/audit.go` replicates the `v2_row_hash` logic from `src/striatum/daemon_pg/audit.py`.
- **Integrity**: The hash is computed from the same fields: `ts`, `schema_version`, `hash_format_version`, `daemon_version`, `client_id`, `repository_id`, `method`, `decision`, `denial_reason`, `transport`, `request_id`, `exit_code`, `params_sha256`, `previous_hash`, `segment_id`.
- **Deterministic Encoding**: Go's `encoding/json` does not sort keys by default for maps. The `audit` package will use a dedicated `AuditMaterial` struct with `json` tags or a custom marshaler to guarantee key order and whitespace-free separators, ensuring bit-for-bit compatibility with Python-generated hashes.
- **SQL Layer**: All inserts to `striatumd.audit_log` are performed via the Go daemon using a transaction that updates `striatumd.audit_chain_head` to ensure strict serializability of the chain.

## 4. Coexistence & Test Parity

### 4.1 Coexistence (RFC 0039 §9 Phase 1)
- The Go binary is named `striatumd-go` to avoid collision with the Python `striatumd` console script.
- The Python CLI (`striatum daemon start`) gains a `--core {python,go}` flag.
- When `--core go` is used, the Python CLI looks for `striatumd-go` in the PATH or via `STRIATUM_DAEMON_GO_PATH`.

### 4.2 Test Harness Integration
`tests/_harness/daemon.py` is extended to support `daemon_core="go"`:
- The `MultiRepoHarness` constructor accepts the parameter.
- The `boot()` method chooses between `sys.executable -m striatum.daemon` and the `striatumd-go` binary.
- Phase 1 Step 3 (Read-only daemon CLI integration) will enable the first e2e tests against the Go core.

## 5. Build & Platform

### 5.1 Makefile
- `build`: `go build -o ../bin/striatumd-go ./cmd/striatumd`
- `test`: `go test ./pkg/...` (covers RPC framing, capability matching, and DB interaction).
- `dist`: Cross-compilation targets for `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`.

### 5.2 Supply-Chain Integrity
- Minimal dependency footprint (standard library + `pgx` + `uuid`).
- No CGO requirements (statically linked by default where possible).
- `go.sum` committed to repository.

## 6. Constraints & Reality
- **Platform**: Linux + macOS only. Windows-specific code (e.g., named pipes) is explicitly out of scope for this design.
- **Persistence**: No SQLite support for the daemon registry. System PostgreSQL is a hard requirement for the Go core.

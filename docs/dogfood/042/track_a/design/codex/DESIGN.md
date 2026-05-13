# Track A Design: Go Daemon Phase 1 Steps 1+2

date: 2026-05-13
status: draft
author: designer-codex-gpt-5.5-002

## Scope

This design covers only RFC 0039 Phase 1 Step 1, "Skeleton + envelope-v1", and Step 2, "Postgres substrate". The implementation should land a buildable Go daemon skeleton, the RFC 0030 envelope-v1 RPC foundation, the capability-bound method registry, and the RFC 0033 PostgreSQL connection, migration, and audit helpers.

The design deliberately excludes RFC 0039 Steps 3-6: Python CLI `--core go` integration, mutating workflow verbs, apply, MCP mutation tools, cross-repo lifecycle implementations, supervised processes, PTY handling, release binaries, and product documentation updates. Those pieces should be designed in Phase 2 or later workflows after the Go skeleton and database substrate are proven.

The product boundary stays unchanged. Repo-local `.striatum/state.sqlite3` remains authoritative workflow state for target repositories. The daemon DB is PostgreSQL-owned daemon state only. The Go daemon must not introduce hosted service semantics, telemetry, transcript capture, remote persistence, or Engram dependency.

## Phase 1 Architecture

The Go daemon is a second implementation of the existing daemon domain, not a new product surface. Python and Go remain separated by contracts:

- The shared wire contract is RFC 0030 envelope-v1 JSON over an owner-local transport.
- The shared storage contract is RFC 0033 PostgreSQL migrations under `src/striatum/daemon_pg/sql/*.sql`.
- The shared vocabulary is the existing daemon method registry, capability set, audit shape, and request log shape.

For Steps 1+2, the binary may start as a local developer daemon invoked directly from `go/cmd/striatumd`. Coexistence with the Python daemon is a runtime rule: only one daemon core may own the socket and PostgreSQL daemon DB at a time. Python remains the default daemon core until RFC 0039 Step 3 introduces CLI selection.

## Go Layout

Create a top-level `go/` tree:

```text
go/
  go.mod
  go.sum
  Makefile
  cmd/
    striatumd/
      main.go
  pkg/
    rpc/
      envelope.go
      registry.go
      capability.go
      server.go
    db/
      connection.go
      migrations.go
      audit.go
```

Use module path:

```text
github.com/halbritt/striatum/go
```

Use Go 1.23 in `go.mod`. Keep third-party dependencies minimal. Step 1 can use only the standard library. Step 2 should use `github.com/jackc/pgx/v5/pgxpool` for PostgreSQL pooling because it is the standard Go driver family for production Postgres use and avoids wrapping a `database/sql` abstraction around Postgres-specific behavior.

Future RFC 0039 packages such as `pkg/apply`, `pkg/mcp`, `pkg/crossrepo`, and `pkg/supervisor` should not be created in Steps 1+2 unless empty package placeholders are needed for acceptance tracking. If placeholders are created, they must not contain behavioral design beyond a package comment marking them deferred.

## Step 1: RPC Skeleton

`go/cmd/striatumd/main.go` is intentionally thin. It should parse only the developer flags needed for Steps 1+2:

- `--socket <path>` for the Unix socket path.
- `--postgres-url <url>` as an explicit override for Step 2.
- `--migrations-dir <path>` defaulting to `src/striatum/daemon_pg/sql` relative to the repository root or current working directory in developer mode.
- `--version` to print the daemon version and exit.

The command should initialize the Postgres pool and migrations once Step 2 lands, construct `rpc.Server`, listen on the Unix socket, set owner-only permissions, accept newline-delimited JSON requests, and shut down cleanly on SIGINT/SIGTERM. It should not spawn agents, call Python CLI verbs, supervise processes, or implement daemon-core selection.

### `go/pkg/rpc/envelope.go`

Responsibilities:

- Define `SupportedEnvelopeVersion = 1` and `DefaultFraming = "json"`.
- Define `Envelope` with `schema_version`, `request_id`, `method`, `params`, optional `capability_token`, and `deadline_ms`.
- Define `Response` with `schema_version`, `request_id`, `ok`, `data` on success, `error` on failure, and optional `audit_id`.
- Validate the same invariants as `src/striatum/daemon_rpc/envelope.py`: non-empty request id, dotted method name, object params, string token when present, and non-negative deadline.
- Return stable RPC errors with code, message, details, and exit-code-equivalent metadata for `version_incompatible`, `schema_invalid`, `method_unknown`, and authorization failures.
- Encode and decode canonical JSON. For hashing, use a deterministic encoder path that sorts map keys before hashing params or responses.

`params` should be represented as `map[string]any` at the boundary. Method-specific structs can come later when Step 3+ routes are implemented.

### `go/pkg/rpc/registry.go`

Responsibilities:

- Define the closed capability vocabulary used by the current code: `read`, `write`, `review`, `claim`, `apply`, `admin`, `recovery`, and `surgical_recovery`.
- Define `MethodEntry` with method, required capability, repository scope mode, params schema version, audit class, minimum envelope, and deprecated flag.
- Mirror the current Python registry entries from `src/striatum/daemon_rpc/registry.py`.
- Publish `Describe()` returning `methods_etag` plus sorted public method entries.
- Compute `methods_etag` as `sha256:` over the canonical JSON representation of sorted public entries.

Step 1 may only route `daemon.hello`, `daemon.describe`, and read-only methods whose implementation is backed by the daemon DB. All other methods can be advertised if they are in the registry, but the server must refuse unimplemented methods with `method_unknown` or `method_unimplemented` consistently. To avoid false capability promises, the safer Step 1 default is to advertise only implemented routes plus the full capability vocabulary. The first parity test should assert this deliberate subset.

### `go/pkg/rpc/capability.go`

Responsibilities:

- Validate that required capabilities are in the closed vocabulary.
- Authorize a request by hashing and looking up `capability_token` in `striatumd.client_capabilities`.
- Enforce expiration, revocation, repository scope, and required capability match.
- Never log or persist the raw token.
- Return denial reasons matching the existing vocabulary: `capability_missing`, `capability_expired`, `capability_scope_mismatch`, `token_revoked`, and `repo_not_registered` where applicable.

`daemon.hello` requires no capability. `daemon.describe` requires `read`, matching the current Python registry. Step 1 should keep authorization explicit even for read-only methods so the Go implementation exercises the same security path early.

### `go/pkg/rpc/server.go`

Responsibilities:

- Enforce mandatory handshake per connection: ordinary routes are refused until `daemon.hello` succeeds.
- Implement `daemon.hello` by returning `daemon_version`, `envelope: 1`, `framing: "json"`, `substrate: "postgres"`, `substrate_schema`, `methods_etag`, and sealed-apply status as unsupported/key-not-loaded for Steps 1+2.
- Implement `daemon.describe` by returning the method registry view.
- Apply deadline handling through `context.WithTimeout` when `deadline_ms > 0`.
- Detect duplicate `request_id` through the request log once Step 2 exists.
- Record request log and audit rows for allowed and denied calls once Step 2 exists.
- Refuse unknown or out-of-scope methods without falling back to direct repo-local reads.

The server should be transport-light. A small Unix socket accept loop can live in this file or a private helper, but the route handler should accept decoded `Envelope` values and return `Response` values so tests can exercise it without sockets.

## Step 2: PostgreSQL Substrate

The Go DB package owns daemon PostgreSQL concerns only. It must not open repo-local SQLite databases. That prevents Step 2 from accidentally becoming cross-repo lifecycle work.

### `go/pkg/db/connection.go`

Responsibilities:

- Resolve the database URL from explicit flag, `STRIATUM_DAEMON_DB_URL`, then daemon config. Config parsing should initially support the existing simple daemon config path and can stay narrow if the current Python config shape is narrow.
- Open a `pgxpool.Pool` with caller-provided context.
- Check `SHOW server_version_num` and refuse versions below the existing Python floor, currently `140000`.
- Provide a doctor-style summary helper for tests: configured source, redacted URL, server version, schema version, and audit privilege safety.
- Check that the daemon role cannot update or delete `striatumd.audit_log` after migrations are applied.

Connection functions should return typed errors that the RPC layer can map to stable daemon errors without leaking credentials.

### `go/pkg/db/migrations.go`

Responsibilities:

- Load forward-only SQL migrations from `src/striatum/daemon_pg/sql/*.sql`.
- Preserve the Python migration metadata: versions 1-4, labels, SHA-256 hashes, and `striatumd.schema_meta` key `substrate_version`.
- Use PostgreSQL advisory lock key `332933`, matching the Python migration runner.
- Refuse startup when the recorded schema version is newer than the Go binary supports.
- Verify recorded migration hashes before skipping already-applied migrations.
- Apply pending migrations in order inside a transaction, recording `schema_migrations` rows and updating `schema_meta`.

The loader should prefer reading the repository SQL files directly during Phase 1 so Python and Go cannot drift silently. A later distribution step may embed those same files with `go:embed`, but that belongs to RFC 0039 Step 6. If an embed path is added early for tests, it must be generated from the same SQL files and verified by SHA.

### `go/pkg/db/audit.go`

Responsibilities:

- Append metadata-only audit rows using the existing `striatumd.audit_log`, `striatumd.audit_chain_head`, and `striatumd.audit_segments` tables.
- Compute row hashes byte-for-byte compatibly with `src/striatum/daemon_pg/audit.py` for hash format version 2.
- Hash canonicalized params and responses; never store request bodies, response bodies, artifact contents, transcripts, token secrets, or model output.
- Update the audit chain head and open segment in a short transaction.
- Verify audit rows for tests by walking `previous_hash` and recomputing row hashes.

RFC 0039 says the audit chain is a DB/schema property. The current Python code computes `v2_row_hash` in application code before inserting, so the Go implementation should match that helper unless the SQL migration later adds a database-side hash function. Do not invent a new hash material order.

## Read-Only Method Boundary

Step 1 says the daemon handles read-only RPC verbs first. In this design, Steps 1+2 should fully implement only:

- `daemon.hello`
- `daemon.describe`
- `daemon.token` authorization checks needed to protect `daemon.describe`
- request/audit logging for these calls after Step 2 lands

Other read routes such as `status`, `why`, `doctor`, `dashboard`, `dashboard.all`, `evidence.export`, `supervise.status`, `supervise.list`, `supervise.reattach_status`, `apply.receipt.show`, `apply.receipt.verify`, `cross_repo.list`, `cross_repo.describe`, and `cross_repo.why` should be present in the registry only when the implementation has a real handler. Because many of these require repo-local SQLite reads or cross-repo lifecycle state, they should be deferred unless a handler can operate solely from the daemon DB without changing Phase 1 scope.

This keeps the Step 1 skeleton honest: envelope, handshake, registry, capability, request log, and audit are production-shaped without pretending the Go daemon already replaces the Python router.

## `go/Makefile`

Add a focused contributor Makefile:

```make
.PHONY: build test fmt vet clean

build:
	go build ./...

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./pkg

vet:
	go vet ./...

clean:
	rm -f striatumd-go

striatumd-go:
	go build -o striatumd-go ./cmd/striatumd
```

The top-level Makefile can gain wrapper targets later, but Step 1+2 do not require changing Python packaging or release behavior.

## Coexistence With Python

During RFC 0039 Phase 1, Python remains the default daemon. The Go daemon can be run by developers directly for unit and harness experiments, but `striatum daemon start --core go` is Step 3 and should not be implemented here.

Coexistence rules for Steps 1+2:

- Use the same PostgreSQL schema and migration SQL as Python.
- Refuse to start if schema is newer than the Go binary.
- Use the same method registry semantics and capability vocabulary.
- Use the same owner-local Unix socket permissions.
- Prevent concurrent daemon ownership through socket creation and later pidfile/lock integration. If the socket already exists and responds to `daemon.hello`, the Go daemon should refuse startup.
- Do not import Python packages from Go or shell out to Python for request handling.

The Go daemon should identify its core in internal diagnostics and handshake extension data only if doing so does not alter RFC 0030 envelope semantics. A `daemon_core: "go"` field may be included in `daemon.welcome` as additive data if current Python clients ignore unknown fields; otherwise leave it to Step 3's describe surface.

## RFC 0035 Harness Extension Shape

The implementation design should be testable by extending the existing multi-repo harness with a `daemon_core` parameter, but Steps 1+2 should only add the minimum hooks needed to run Go package tests and an optional daemon smoke test.

Target shape for the later harness extension:

```python
harness = MultiRepoHarness(
    daemon_pg_url=postgres_url,
    repo_count=2,
    scratch_dir=tmp_path,
    daemon_core="go",
)
```

For `daemon_core="go"`, the harness eventually starts the Go binary with the same ephemeral PostgreSQL URL and Unix socket path used for the Python daemon. It should assert the `daemon.hello` handshake, `daemon.describe` registry, migration version, capability refusal path, and audit chain continuity before any Step 3+ end-to-end lifecycle tests are attempted.

Do not wire the full RFC 0035 matrix in Steps 1+2. Full prepare, lifecycle, crash-recovery, MCP capability-scope, and per-repo write-scope parity depends on Step 3+ route implementations.

## Testing Plan

Step 1 tests:

- `go test ./pkg/rpc/...` for envelope decode/encode, schema refusals, response encoding, registry sorting, methods etag stability, capability vocabulary validation, handshake success, handshake version refusal, pre-handshake route refusal, and unknown method refusal.
- Socket-level smoke test using a temporary Unix socket and newline-delimited JSON request.

Step 2 tests:

- `go test ./pkg/db/...` against ephemeral PostgreSQL.
- Migration test applies all SQL files, records schema version 4, verifies migration hashes, and refuses a simulated newer schema.
- Connection doctor test checks supported Postgres version and redacted URL behavior.
- Audit append test inserts allowed and denied rows, verifies `previous_hash` continuity, and confirms no raw capability token appears in audit or request log tables.

Integration smoke:

- Start `go/cmd/striatumd` against ephemeral PostgreSQL.
- Send `daemon.hello`.
- Create or seed a read capability token through direct test fixture SQL.
- Send `daemon.describe`.
- Verify an audit row is appended for the allowed call and for a denied call with a bad token.

Run Python tests only if touched indirectly. Since Steps 1+2 should not modify Python behavior, the expected verification is Go tests plus a focused Python smoke if any shared SQL migration fixture changes.

## Out Of Scope For This Artifact

- Python CLI `striatum daemon start --core go`.
- Route parity for `status`, `why`, `dashboard`, `repo.add`, workflow lifecycle, MCP, apply, or dogfood tools.
- Supervised process ownership, PTY behavior, FIFO packet delivery, or supervised-progress heartbeat.
- Cross-repo run lifecycle implementation.
- Release binaries, wheel packaging, install docs, or default-core changes.
- Engram memory integration.
- Any change to RFC 0030 envelope shape or RFC 0033 schema semantics.

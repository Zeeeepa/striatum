---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/042/track_a/design/codex/DESIGN.md", "docs/dogfood/042/track_a/design/claude_code/DESIGN.md", "docs/dogfood/042/track_a/design/gemini/DESIGN.md"]
---

author: synthesizer-codex-1

# Track A Synthesis: Go Daemon Phase 1 (RFC 0039 Steps 1+2)

This synthesis reconciles the three Track A designs (codex, claude_code,
gemini) into one concrete plan for RFC 0039 Phase 1 Steps 1+2 only:
envelope-v1 RPC skeleton and PostgreSQL substrate. It excludes RFC 0039
Steps 3-6 (CLI `--core go` integration, mutating verbs + apply,
supervised processes, distribution, product-doc updates); those are
deferred to a Phase 2 dogfood and noted in the RFC 0039 status block
when this work lands.

The product boundary is unchanged. Repo-local `.striatum/state.sqlite3`
remains the authoritative workflow state for target repositories. The
Go daemon owns daemon-side PostgreSQL state only and must not open any
repo-local SQLite, introduce hosted-service semantics, telemetry,
transcript capture, or Engram dependency.

## 1. Layout

A single top-level `go/` tree:

```text
go/
  go.mod
  go.sum
  Makefile
  cmd/
    striatumd/
      main.go              # entrypoint: flags, signals, boot
  pkg/
    rpc/
      envelope.go          # envelope-v1 codec + RpcError vocabulary
      registry.go          # MethodEntry + Describe() + methods_etag
      capability.go        # closed capability set + token check
      server.go            # Unix-socket accept loop + dispatch
    db/
      connection.go        # pgxpool open + doctor summary
      migrations.go        # forward-only loader from Python SQL source
      audit.go             # v2 row-hash helper + AppendAuditRow
```

No placeholder packages (`pkg/apply`, `pkg/supervisor`, `pkg/mcp`,
`pkg/crossrepo`) land in Phase 1. Their absence is intentional.

### 1.1 `go.mod`

```text
module github.com/halbritt/striatum/go

go 1.23

require (
    github.com/jackc/pgx/v5 v5.7.x
    github.com/google/uuid v1.6.x
)
```

Toolchain directive `toolchain go1.23.X` is set so contributors on
newer Go transparently fetch the pinned compiler. No gRPC, no protobuf,
no third-party RPC framework — envelope-v1 is implemented on
`net.Listener` + `bufio.Scanner`. No CGO. Argon2id token verification
uses `golang.org/x/crypto/argon2` when capability storage requires it.

The module path is namespaced under `/go` so the Python wheel and Go
module share one repository without colliding. `go install
github.com/halbritt/striatum/go/cmd/striatumd@<tag>` yields a
`striatumd` binary.

### 1.2 `go/Makefile`

Contributor-only; CI matrix wiring lands with Step 6.

```make
.PHONY: build test test-race fmt vet lint clean

GO ?= go
BIN := bin/striatumd
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo "0.0.0-dev")

build:
	$(GO) build -o $(BIN) -ldflags "-X main.Version=$(VERSION)" ./cmd/striatumd

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint:
	$(GO) run honnef.co/go/tools/cmd/staticcheck@latest ./...

clean:
	rm -rf bin
```

No `embed-sql` target and no `//go:embed` of SQL in Phase 1. The Go
daemon loads migrations directly from
`src/striatum/daemon_pg/sql/*.sql` resolved via `--migrations-dir`
(default: upward search for `go.mod` then
`../src/striatum/daemon_pg/sql`). Embedding is a Step 6 distribution
concern and is deferred. The top-level `Makefile` is not modified in
Phase 1.

## 2. Step 1: RPC envelope-v1

`go/cmd/striatumd/main.go` is intentionally thin. Phase 1 flags:

- `--socket <path>` — Unix socket, default
  `${XDG_RUNTIME_DIR}/striatum/daemon.sock` (the same path the Python
  daemon binds, by design — see §4 coexistence).
- `--db-url <url>` — PostgreSQL URL override; otherwise
  `STRIATUM_DAEMON_DB_URL`, then `~/.config/striatum/daemon.conf`.
- `--migrations-dir <path>` — defaults to repo-resolved
  `src/striatum/daemon_pg/sql`.
- `--version` — print `striatumd-go/<semver>` and exit.

`main.go` opens the pool, applies pending migrations, builds the
registry with explicit registration (no `init()`-time side effects),
binds the socket at `0600`, and shuts down on SIGINT/SIGTERM.

### 2.1 `pkg/rpc/envelope.go`

Mirrors `src/striatum/daemon_rpc/envelope.py`:

- Constants `SupportedEnvelopeVersion = 1`, `DefaultFraming = "json"`.
- `Envelope{SchemaVersion int, RequestID string, Method string,
  Params json.RawMessage, CapabilityToken string, DeadlineMS int64}`.
- `Response{SchemaVersion int, RequestID string, OK bool,
  Data json.RawMessage, Error *RpcError, AuditID string}` with success
  and refusal branches structurally separate so the server cannot emit
  both.
- `DecodeRequest(line []byte) (Envelope, error)` and
  `EncodeResponse(Response) ([]byte, error)` exchange
  newline-delimited JSON per RFC 0030 §1.
- Refusal codes produced by the envelope alone:
  `version_incompatible`, `framing_unsupported`, `schema_invalid`,
  `request_too_large`, `duplicate_request_id`. Codes match the Python
  daemon's `RpcError` set byte-for-byte; a parity test asserts
  equality.
- Bounded scanner:
  `scanner.Buffer(make([]byte,0,64*1024), 1024*1024)` hard-caps frames
  at 1 MiB.

Canonical JSON for hashing uses `sort_keys=True` and
`separators=(",", ":")` — the exact shape produced by
`src/striatum/db.py::json_dumps`. This is **not** Go's default
`encoding/json` map ordering; the audit and `methods_etag` paths use a
dedicated canonical encoder (sorted keys, no whitespace, ASCII-safe
escapes) over explicit struct fields rather than `map[string]any`.

### 2.2 `pkg/rpc/registry.go`

Mirrors `src/striatum/daemon_rpc/registry.py`:

- `MethodEntry{Name string, RequiredCapability Capability,
  RepositoryScope ScopeMode, ParamsSchemaVersion int,
  AuditClass string, MinEnvelope int, Deprecated bool,
  Handler func(ctx, env, conn) (json.RawMessage, *RpcError)}`.
- `Registry` is a plain `map[string]MethodEntry` populated by an
  explicit `buildRegistry(pool)` call from `main.go`. Init-time
  registration is rejected: a misplaced import must not silently
  expose a security-sensitive verb.
- `Describe()` returns the public method view plus
  `methods_etag = "sha256:" + hex(sha256(canonical-json(sorted entries)))`,
  byte-stable across runs.

Phase 1 registers exactly the verbs whose handler is fully
implementable from the daemon DB:

```text
daemon.hello       (no capability; pre-handshake)
daemon.welcome     (server-emitted; not callable)
daemon.describe    read
daemon.status      read
daemon.version     read
audit.show         read
repo.list          read
```

Other read-only verbs (`why`, `doctor`, `dashboard`, `dashboard.all`,
`evidence.export`, `supervise.status`, `supervise.list`,
`apply.receipt.show`, `apply.receipt.verify`, `cross_repo.*`) are
**not registered** in Phase 1 because their Python implementations
read repo-local SQLite or cross-repo lifecycle state that has not been
modeled in the Go daemon. Registering an unimplemented verb would be a
false capability promise; the registry refuses unknown methods with
`method_not_found`.

All mutating verbs (`session.register`, `claim_next`, `ack`,
`publish_artifact`, `verdict`, `complete`, `repo.add`, `repo.remove`,
MCP, `recovery.*`, `surgical_recovery.*`) are deferred to Step 4.

### 2.3 `pkg/rpc/capability.go`

The capability vocabulary is a **closed set** inherited from RFC 0030
plus the RFC 0032 / surgical-recovery extensions:

```go
type Capability string

const (
    CapRead             Capability = "read"
    CapWrite            Capability = "write"
    CapReview           Capability = "review"
    CapClaim            Capability = "claim"
    CapApply            Capability = "apply"
    CapAdmin            Capability = "admin"
    CapRecovery         Capability = "recovery"
    CapSurgicalRecovery Capability = "surgical_recovery"
)
```

`ParseCapability(s string) (Capability, error)` refuses any string
outside this list with stable code `capability_unknown`. The full
vocabulary is parseable in Phase 1 even though only `read` gates any
registered verb; this exercises the security surface early and lets
the registry survive Step 4 without restructuring.

Token check:
`CheckCapability(ctx, conn, token, required, repositoryID) (ClientID,
AuditFields, error)` is the single authorization gate. It resolves
`striatumd.clients` ⨝ `striatumd.client_capabilities` in one
parameterized `pgx.QueryRow` so revocation between issuance and check
is honored. Token hashes are argon2id with the same parameters as the
Python daemon, so a token issued by either core validates in both.

Refusal codes (audited before the refusal is returned, so brute-force
attempts cannot bypass the chain): `capability_token_missing`,
`capability_token_invalid`, `capability_token_expired`,
`capability_token_revoked`, `capability_missing`,
`capability_repository_mismatch`, `repo_not_registered`.

`daemon.hello` requires no capability. `daemon.describe` requires
`read`, matching the Python registry.

### 2.4 `pkg/rpc/server.go`

Responsibilities:

- **Listen**: bind `${XDG_RUNTIME_DIR}/striatum/daemon.sock` at
  `0600`. Refuse to bind if the parent directory is world-writable or
  the filesystem doesn't support Unix-domain sockets. Stale sockets
  are detected by probing the path with `unix.Connect`; a responding
  peer means another daemon owns the database and the Go daemon exits
  with code 14 `daemon_already_running`. Only unresponsive stale
  paths are unlinked.
- **Handshake**: the first envelope on every connection must be
  `daemon.hello`; pre-handshake calls are refused with
  `handshake_required`. `daemon.welcome` returns
  `{daemon_version, daemon_core: "go", envelope: 1, framing: "json",
  substrate: "postgres", substrate_schema, methods_etag,
  sealed_apply: {supported: false}}`.
- **Dispatch**: lookup → `CheckCapability` → handler bound to a
  `context.WithTimeout(deadline_ms)`. Panics are recovered, audited
  as `internal_error`, and returned as an envelope error.
- **Audit**: every dispatch (allow and deny) appends one audit row
  via the helper in §3.3.
- **Shutdown**: SIGTERM cancels the root context, closes the
  listener, drains the waitgroup with a 5 s deadline, then closes the
  pool.

The accept loop is transport-thin; the route handler accepts decoded
`Envelope` values and returns `Response` so unit tests exercise
dispatch without sockets.

## 3. Step 2: PostgreSQL substrate

The DB package owns daemon PostgreSQL concerns only. It does not open
repo-local SQLite. That boundary is what keeps Step 2 from
accidentally absorbing cross-repo lifecycle work.

### 3.1 `pkg/db/connection.go`

- Resolve URL precedence: explicit `--db-url` flag →
  `STRIATUM_DAEMON_DB_URL` → `~/.config/striatum/daemon.conf`.
- Open `*pgxpool.Pool` with `min=1`, `max=8`,
  `max_conn_lifetime=1h`.
- After-connect hook sets `application_name=striatumd-go/<semver>`,
  `statement_timeout=30s`,
  `idle_in_transaction_session_timeout=60s`.
- Refuse PostgreSQL `server_version_num` below the existing Python
  floor (`140000`).
- Refuse hosts other than `localhost`, `127.0.0.1`, `::1` unless
  `--allow-remote-pg` is passed. Striatum is single-machine per D083;
  a remote URL is almost certainly an operator mistake.
- Wrap pool errors with a redactor that strips `password=…` fragments
  before any error string surfaces to the CLI or audit row.
- Expose `Doctor(ctx) (Summary, error)` returning configured source,
  redacted URL, server version, schema version, and audit-role
  restrictions for tests and `daemon.status`.

### 3.2 `pkg/db/migrations.go`

Phase 1 does **not** change the schema. The migrations
`0001_baseline.sql` … `0004_dogfood_surgical_recovery.sql` already
shipped under `src/striatum/daemon_pg/sql/` are the contract. The Go
daemon reads those files directly:

- Source: filesystem only. Default path resolves via
  `--migrations-dir` or upward search for `go.mod` then
  `../src/striatum/daemon_pg/sql/`. No `//go:embed` in Phase 1 —
  keeping Python as the single source of truth removes the drift
  class entirely. Embedding lands with Step 6 release packaging, with
  build-time SHA verification against the Python source.
- Lock: PostgreSQL advisory lock key `332933`, matching the Python
  migration runner.
- Apply pending migrations in version order inside a single
  transaction. Record each in `striatumd.schema_migrations` with
  `(version, label, sha256, applied_at, daemon_version)`; set
  `daemon_version='striatumd-go/<semver>'` so operator tooling can
  see which core last touched the schema.
- Verify recorded SHA-256 against the file before skipping an
  already-applied migration. Mismatch → exit with stable code
  `schema_drift_detected` (someone hand-edited a frozen migration;
  this would silently fork the schema between cores).
- Refuse startup if the recorded `striatumd.schema_meta` substrate
  version is **newer** than the Go binary supports.
- Phase 1 still runs migrations on startup even though the daemon is
  read-only at the method layer: capability denials write to the
  audit chain, so the daemon needs the audit table at the current
  shape.

### 3.3 `pkg/db/audit.go`

The audit chain is the v2 row hash defined in
`src/striatum/daemon_pg/audit.py::v2_row_hash`. The Go helper must be
byte-for-byte compatible. Cross-language parity is a release blocker.

Material fields, in struct order:

```text
ts, schema_version, hash_format_version, daemon_version, client_id,
repository_id, method, decision, denial_reason, transport,
request_id, exit_code, params_sha256, previous_hash, segment_id
```

Canonical JSON: `sort_keys=True`, `separators=(",", ":")` — exactly
the shape produced by `src/striatum/db.py::json_dumps`. (Note:
Python's `json_dumps` sorts keys; the Go encoder must do the same
regardless of the struct-field order above. This is the contested
point §6.) The Go implementation uses a small canonical-JSON helper
(sorted keys, whitespace-free separators, ASCII escape rules) over an
explicit struct, not a `map[string]any`, so field discipline is
enforced at compile time.

`V2RowHash(row AuditRow) string` returns the hex SHA-256. Phase 1
only appends v2 rows; v1 verification is delegated to the Python
verifier for historical segments and the Go daemon refuses to append
v1-shaped rows.

`AppendAuditRow(ctx, tx pgx.Tx, row AuditRow) (auditID string, err error)`
performs `SELECT previous_hash FROM striatumd.audit_chain_head ...
FOR UPDATE` + `INSERT INTO striatumd.audit_log ...` + chain-head
update in a single short transaction so the chain cannot fork under
concurrent writers. The helper takes a `pgx.Tx` so capability denials
and handler success paths can both compose the audit append into
their own transactional scope.

Phase 1 callers: every capability denial path in `capability.go`,
plus each registered verb's handler. `audit.show` does not audit its
own listing call (consistent with the Python daemon).

### 3.4 Read-path scope (Phase 1)

Phase 1 handlers touch only:

- `striatumd.repositories` (`repo.list`).
- `striatumd.audit_log` and `striatumd.audit_chain_head`
  (`audit.show`, `AppendAuditRow`).
- `striatumd.daemon_meta`, `striatumd.schema_meta`,
  `striatumd.schema_migrations` (`daemon.status`, `daemon.version`,
  `daemon.welcome`).
- `striatumd.clients` and `striatumd.client_capabilities` (capability
  check).

No handler reaches `striatumd.cross_repo_runs`,
`striatumd.supervisor_*`, or any write surface beyond `audit_log` /
`audit_chain_head`. Defense in depth: the daemon connects as a role
that has `INSERT` on `striatumd.audit_log` and `UPDATE` on
`striatumd.audit_chain_head` only; everywhere else it is
`SELECT`-only. The role escalation lands with Step 4.

## 4. Coexistence with the Python daemon

Phase 1 keeps the Python daemon as the default core. The Go daemon
is opt-in for developers running it directly:

- Both daemons bind the same default socket path
  (`${XDG_RUNTIME_DIR}/striatum/daemon.sock`). Envelope-v1 is the
  contract; clients don't care which core answers.
- Mutually-exclusive ownership is enforced at the Postgres layer.
  Startup queries `pg_stat_activity` for any connection whose
  `application_name LIKE 'striatumd-%'`; if one exists, the Go
  daemon refuses with exit code 14 `daemon_already_running`. The
  Python daemon's symmetric check lands as a small follow-up before
  Step 4; until then, operators stop the Python daemon before
  starting the Go one.
- The schema-drift refusal (§3.2) prevents out-of-band migration
  application from silently forking the substrate.
- Envelope-v1 version negotiation refuses cross-version mismatch
  with `version_incompatible`.

A `striatumd.daemon_meta.boot_lock` row keyed by `pid` + `hostname` +
`started_at` is written on successful boot for operator visibility,
but the authoritative liveness signal is the `pg_stat_activity`
check above. No pidfile is written — the pidfile pattern does not
cross the Python↔Go boundary cleanly.

CLI selection (`striatum daemon start --core go`), launcher
resolution, and `STRIATUM_DAEMON_GO_PATH` discovery are **Step 3**
and out of scope.

## 5. Test strategy

### 5.1 Go unit tests (`go test ./...`)

`pkg/rpc/...`:

- Envelope decode/encode parity: refuse `version != 1`, refuse
  non-dotted method, refuse > 1 MiB lines, refuse non-object params.
- `RpcError` code set byte-equality with a Python-generated fixture.
- Registry `Describe()` shape and `methods_etag` stability across
  runs and process restarts.
- Capability vocabulary parse + reject; `ParseCapability` round-trip.
- Handshake: `daemon.hello` success, `daemon.hello` version refusal,
  pre-handshake route refusal, unknown method refusal.
- Socket-level smoke: temporary Unix socket, newline-delimited JSON
  exchange, owner-only permission assertion.

`pkg/db/...` (against an ephemeral PostgreSQL, fixture-managed):

- `Open` connects, sets `application_name`, applies session
  timeouts, refuses below floor version.
- Migrations apply 0001-0004 to an empty DB, record version 4 in
  `striatumd.schema_meta`, are idempotent on a second call, and
  refuse on a simulated newer schema.
- Migration SHA mismatch refusal: hand-edit a temp copy and assert
  `schema_drift_detected`.
- `V2RowHash` matches a Python-generated fixture for a
  representative audit row (the cross-language hash parity test;
  release-blocking).
- `AppendAuditRow` produces a valid chain for two sequential rows
  and refuses to insert a v1-shaped row.
- Doctor summary returns redacted URL (no `password=` substring) and
  audit-role privilege report.

### 5.2 RFC 0035 harness `daemon_core` parameter

The harness gains one parameter and one e2e smoke test in Phase 1:

```python
class MultiRepoHarness:
    def __init__(self, *, daemon_core: Literal["python", "go"] = "python", ...):
        self._daemon_core = daemon_core
```

`tests/_harness/daemon.py::start_daemon()` reads the parameter:

- `python` (default): existing `striatum daemon start` subprocess —
  unchanged.
- `go`: spawns `./go/bin/striatumd` (override via `STRIATUMD_GO_BIN`)
  with `--socket=<path>` and `--db-url=<ephemeral-pg-url>`. The
  harness fixture invokes `make -C go build` if the binary is
  missing. No cross-compile, no release-binary download.

Phase 1 ships exactly one new e2e test:

```text
tests/test_daemon_go_smoke.py
```

Steps:

1. Boot `MultiRepoHarness(daemon_core="go", repos=1)`.
2. Drive `daemon.hello` + `daemon.welcome` from the Python CLI
   client.
3. Call `daemon.describe`; assert the registry is exactly the
   Phase 1 read-only set.
4. Call `daemon.status`, `audit.show`, `repo.list`; assert each
   returns the same data shape the Python daemon would for the same
   DB state.
5. Insert a deliberately-invalid capability token; assert the call
   is refused with `capability_token_invalid` **and** an audit row
   is written.
6. Verify every audit row written by the Go daemon during the test
   via the **Python** verifier
   (`src/striatum/daemon_pg/audit.py::verify_rows`) — this is the
   cross-language hash parity gate.

Marker: `@pytest.mark.requires_go_daemon`, skipped on runners
without Go. CI matrix wiring lands with Step 6.

The `daemon_core` parameter is intentionally plumbed end-to-end so
that Step 4 can flip the existing e2e suite to
`@pytest.mark.parametrize("daemon_core", ["python", "go"])` per test
without restructuring fixtures.

## 6. Contested decisions (chosen, not enumerated)

- **Canonical JSON for audit hashing**: `sort_keys=True`,
  `separators=(",", ":")`. Gemini's reading of `db.py::json_dumps`
  is correct; the claude_code design's claim that Python uses
  `sort_keys=False` is wrong (verified against `src/striatum/db.py`).
  The Go side uses an explicit struct + a sorted-keys canonical
  encoder; no `map[string]any` on the hash path.
- **SQL migration source**: filesystem-only in Phase 1, no
  `//go:embed`. Embedding belongs to Step 6 distribution; keeping a
  single source of truth removes the drift class.
- **Method registry surface**: advertise only implemented routes
  (codex's posture), not the full Python catalogue. False capability
  promises are worse than a smaller surface.
- **Daemon liveness signal**: `pg_stat_activity` `application_name`
  query (claude_code), not a pidfile. The lock is in the place both
  cores already share — the DB.
- **Capability vocabulary**: all eight names parseable from day one,
  even though only `read` gates any registered verb. The cost is
  zero and Step 4 inherits a working surface.
- **Binary name**: `striatumd` (not `striatumd-go`). The launcher in
  Step 3 picks the binary; the binary's own name doesn't have to
  encode its core.

## 7. Implementer split

Two work packets land this synthesis.

### 7.1 `implement_go_systems_codex` — codex

Owns everything inside `go/` plus daemon-side test files:

- `go/go.mod`, `go/go.sum`, `go/Makefile`.
- `go/cmd/striatumd/main.go`.
- `go/pkg/rpc/{envelope,registry,capability,server}.go` and their
  unit tests.
- `go/pkg/db/{connection,migrations,audit}.go` and their unit tests
  (the ephemeral-Postgres fixture in this codepath is Go-side; the
  Python harness fixture in §5.2 is glue).
- The canonical-JSON helper used by `audit.go` and registry etag
  computation.
- A Python-generated audit-row fixture committed at
  `go/pkg/db/testdata/v2_row_hash_fixture.json` (produced once by
  the Python verifier, regenerated by a script the glue packet
  owns).
- No changes to Python source, Python tests, or product docs.

### 7.2 `implement_go_glue_claude` — claude_code

Owns the Python-side glue and documentation:

- `tests/_harness/multi_repo.py`: add `daemon_core` parameter,
  default `python`.
- `tests/_harness/daemon.py`: dispatch `daemon_core="go"` to the
  Go-binary launcher; auto-build via `make -C go build` when
  missing; honor `STRIATUMD_GO_BIN`.
- `tests/test_daemon_go_smoke.py`: the new e2e smoke test from §5.2
  with `@pytest.mark.requires_go_daemon`.
- A small script (e.g. `tests/_harness/audit_fixture.py`) that emits
  the canonical Python `v2_row_hash` fixture consumed by the Go unit
  tests. The Go packet writes the fixture into its testdata; the
  glue packet owns the generator.
- Doc updates:
  - `docs/HOW_TO_HUMAN.md`: add a "Running the Go daemon (developer
    preview)" section covering `make -C go build`, the
    `STRIATUMD_GO_BIN` override, and the "stop the Python daemon
    first" coexistence rule.
  - `docs/SPEC.md`: extend the daemon section to note Go is a
    second implementation behind the same envelope-v1 + Postgres
    contracts; no product-surface change.
  - `docs/UBIQUITOUS_LANGUAGE.md`: add **daemon core** as the term
    for "Python vs Go implementation of the daemon process".
  - `docs/rfcs/0039-go-daemon-core.md`: status block updated with
    `Phase 1 Steps 1+2: landed in dogfood-042`, plus a sentence
    deferring Steps 3-6 to a Phase 2 dogfood.
- No changes to anything inside `go/`.

The split is strict. Each packet's `write_scope.allowed_paths`
excludes the other packet's territory so they can land in either
order without merge conflicts. Both packets share read access to
this synthesis and to RFCs 0030, 0033, 0039.

## 8. Out of scope (deferred to Phase 2 / future dogfoods)

Explicitly **not** in Phase 1 Steps 1+2:

- **Step 3** — `striatum daemon start --core go` flag in the Python
  CLI, launcher binary discovery, fallback behavior when the binary
  is missing.
- **Step 4** — mutating verbs (`session.register`, `claim_next`,
  `ack`, `publish_artifact`, `verdict`, `complete`, `repo.add`,
  `repo.remove`, `recovery.*`, `surgical_recovery.*`), MCP tool
  dispatch, apply-receipt signing, capability-token issuance, full
  e2e suite parametrization across `daemon_core`.
- **Step 5** — supervised processes: `os/exec` + `creack/pty`, FIFO
  packet delivery, supervised-progress lease heartbeat,
  deterministic SIGTERM drain, transcript discipline.
- **Step 6** — distribution: cross-compile matrix, release
  binaries, PyPI wheel-with-binary packaging, `//go:embed` of SQL
  migrations against build-time SHA verification, top-level
  `Makefile` `daemon-go-*` targets, CI matrix changes, default-core
  flip.
- Engram memory integration, hosted services, telemetry, transcript
  capture, remote persistence.
- Any change to RFC 0030 envelope shape or RFC 0033 schema
  semantics.

Anyone landing this work and looking for those topics: they are
deliberately absent, not forgotten.

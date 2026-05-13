# Track A Design: Go Daemon (RFC 0039 Phase 1 Steps 1+2)

author: designer-claude-opus-002

## Scope

This design covers **RFC 0039 §Implementation Plan Steps 1 and 2 only**:

- **Step 1**: Go skeleton at `go/cmd/striatumd/main.go`; envelope-v1 framing;
  `daemon.hello` / `daemon.welcome` handshake; `daemon.describe` method
  registry exposition; capability-bound method registry; read-only RPC verbs
  served from Postgres.
- **Step 2**: Postgres substrate — `go/pkg/db/connection.go`,
  `go/pkg/db/migrations.go`, `go/pkg/db/audit.go`.

Steps 3 (CLI integration), 4 (mutating verbs + apply), 5 (supervised
processes), and 6 (distribution + docs) are explicitly **out of scope** for
this design and are deferred to Phase 2 of dogfood-042 or follow-up
dogfoods.

Trust boundary lens: the Go daemon is a process that the operator's Python
CLI starts; it speaks RFC 0030 envelope-v1 over a Unix-domain socket; it
reads and writes the same Postgres database the Python daemon writes today.
Three trust boundaries cross this design:

1. **Python CLI ↔ Go daemon** over a Unix socket (envelope-v1 JSON).
2. **Go daemon ↔ Postgres** over a libpq connection string.
3. **Go daemon ↔ filesystem** for migration SQL discovery and the socket
   itself.

Footgun discipline (called out at each boundary): the Go daemon and the
Python daemon are mutually exclusive owners of the same database. Operator
mistakes that violate that invariant (running both at once, pointing the
binaries at different DBs, applying migrations out-of-band) must be
detected and refused, not silently tolerated.

## 1. Repository layout (Steps 1+2)

Only the directories needed for Steps 1+2 land in this phase:

```text
striatum/
  go/
    cmd/
      striatumd/
        main.go              # process entrypoint; no business logic
    pkg/
      rpc/
        envelope.go          # envelope-v1 codec
        registry.go          # method registry + describe
        capability.go        # capability vocabulary + token check
        server.go            # accept loop + dispatch
      db/
        connection.go        # pgx pool + STRIATUM_DAEMON_DB_URL parsing
        migrations.go        # migration loader (filesystem + embed fallback)
        audit.go             # audit-chain hash helper (v2 only in Phase 1)
    go.mod
    go.sum
    Makefile                 # build, test, fmt, vet, lint
    embed_sql.go             # //go:embed of frozen SQL migrations
```

Out-of-scope subpackages (`go/pkg/apply/`, `go/pkg/supervisor/`,
`go/pkg/mcp/`, `go/pkg/crossrepo/`) are not created in this phase. Their
absence is intentional and recorded in `go.mod` package documentation:
nothing imports a stub.

The top-level `Makefile` does not add `daemon-go-*` targets in this phase;
the Go-side `go/Makefile` is sufficient for contributor build/test. CI
matrix changes that wire Go into the release pipeline land with Step 6.

### 1.1 `go.mod` shape

```text
module github.com/halbritt/striatum/go

go 1.23

require (
    github.com/jackc/pgx/v5 v5.7.x
    github.com/google/uuid v1.6.x      // UUIDv7 generation for request_id
)
```

Toolchain pin: `go 1.23`. The `toolchain go1.23.X` directive is set so
contributors who run a newer Go automatically download the pinned
toolchain. No third-party RPC framework; the daemon implements envelope-v1
on raw `net.Listener` + `bufio.Scanner` to keep the wire format
language-agnostic and to avoid pulling in gRPC/protobuf (rejected per
RFC 0039 §Open Questions).

The module path is `github.com/halbritt/striatum/go` rather than
`github.com/halbritt/striatum` so the Python wheel and the Go module live
under one repo without colliding on path semantics. Anyone running
`go install github.com/halbritt/striatum/go/cmd/striatumd@<tag>` gets a
`striatumd` binary.

### 1.2 Why this layout and not flatter

A flatter `go/main.go` would couple envelope, capability, registry, and
storage in one package and force re-organization at Step 4. The four-file
split inside `pkg/rpc/` matches the Python daemon's existing split
(`envelope.py`, `registry.py`, `capability.py`, `server.py`), which
shortens review for reviewers who know one side well and makes parity
testing a one-to-one mapping.

## 2. `go/pkg/rpc/` responsibilities

### 2.1 `envelope.go`

Mirrors `src/striatum/daemon_rpc/envelope.py`:

- Constant `SupportedEnvelopeVersion int = 1`, `DefaultFraming = "json"`.
- Type `Envelope` with fields `SchemaVersion`, `RequestID`, `Method`,
  `Params json.RawMessage`, `CapabilityToken string`, `DeadlineMS int64`.
- `func DecodeRequest(line []byte) (Envelope, error)` — refuses any
  schema_version != 1 with stable code `version_incompatible`; refuses
  non-dotted method names with `schema_invalid`; refuses oversized lines
  (> 1 MiB default) with `request_too_large`.
- `func EncodeResponse(resp Response) ([]byte, error)` — emits the
  newline-delimited JSON shape `{schema_version, request_id, ok,
  data|error, audit_id}` per RFC 0030 §1.
- `Response` separates the success branch (`data json.RawMessage`) from
  the refuse branch (`error RpcError{Code, Message, Details}`), so the
  server cannot accidentally emit both.

Refusal codes the envelope can produce on its own (without dispatching to
a method): `version_incompatible`, `framing_unsupported`, `schema_invalid`,
`request_too_large`, `duplicate_request_id`. These match the Python
daemon's `RpcError` codes byte-for-byte; the parity test for Phase 1
serializes the same input through both daemons and compares response
bodies.

**Trust-boundary note**: the envelope decoder is the only code that sees
unauthenticated bytes from the socket. It does no DB work, no logging of
the capability token, and no allocation beyond bounded scratch buffers.
Any malformed input results in a deterministic refusal frame and immediate
connection close. The Go scanner's default 64 KiB token size is too small
for some `daemon.describe` responses; we set
`scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)` to hard-cap at 1 MiB
per line.

### 2.2 `registry.go`

Mirrors `src/striatum/daemon_rpc/registry.py`:

- Type `Method` with fields `Name string`, `RequiredCapabilities []Capability`,
  `Handler func(ctx context.Context, env Envelope, conn DBConn) (json.RawMessage, error)`,
  `Mutating bool`, `Description string`.
- Type `Registry` is a `map[string]Method` populated at daemon startup.
  Methods register themselves via package-level `init()` is **rejected**;
  registration is explicit in `main.go` so the visible call site is the
  source of truth. Hidden-init registration would let a misplaced import
  silently expose a new method — a real footgun on a security-sensitive
  surface.
- `func (r *Registry) Describe() DescribeResponse` produces the
  `daemon.describe` payload: every method's name, required capabilities,
  mutating flag, params schema reference, and a stable
  `methods_etag = sha256(canonical-json(methods))` that clients cache.

Phase 1 method set (read-only verbs from Postgres only):

```text
daemon.hello       (no capability; transport-level)
daemon.welcome     (server-emitted; not callable)
daemon.describe    read
daemon.status      read
daemon.version     read
audit.show         read
repo.list          read
```

Mutating verbs (`session.register`, `claim_next`, `ack`, `publish_artifact`,
`verdict`, `complete`, `repo.add`, `repo.remove`, MCP, recovery,
surgical_recovery) are **not registered** in the Phase 1 daemon. The
registry refuses unknown methods with `method_not_found`. This is the
"Go daemon handles read-only RPC verbs first" gate from RFC 0039 §10
Step 1.

### 2.3 `capability.go`

Capability vocabulary is a closed set, matching RFC 0030 §4 + the RFC 0032
extensions + the surgical-recovery extension:

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

Closed set: any input string outside this list is rejected by the
constructor `ParseCapability(s string) (Capability, error)` with stable
code `capability_unknown`. The token-issuance code (Python today, Go
later) never writes a string the daemon will refuse to parse.

Token check:

- The Postgres substrate stores `token_hash` and `token_salt` per client
  (see `striatumd.clients`). Hash format is the same `argon2id` parameters
  the Python daemon uses; the Go implementation calls
  `golang.org/x/crypto/argon2`. The parity test asserts that a token
  issued by the Python daemon validates inside the Go daemon and
  vice-versa.
- `func CheckCapability(ctx, conn, token, required []Capability,
  repositoryID string) (ClientID, AuditFields, error)` is the single
  authorization gate. It runs in a single `SELECT ... FROM
  striatumd.clients JOIN striatumd.client_capabilities` query so that
  capability revocation between issuance and check is honored. The query
  uses `pgx.QueryRow` with parameterized bindings; **no string
  concatenation, ever**.
- Refusal codes: `capability_token_missing`, `capability_token_invalid`,
  `capability_token_expired`, `capability_token_revoked`,
  `capability_missing`, `capability_repository_mismatch`. Each is
  audited (see §3.4) with the exact denial reason.

**Trust-boundary note**: the capability check is the gate between
unauthenticated socket input and any DB read beyond the audit-append
itself. We deliberately audit the capability denial **before** returning
the refusal, so an attacker who finds a denial path that doesn't audit
cannot use the daemon to brute-force tokens silently.

### 2.4 `server.go`

The accept loop and dispatch:

```go
func Run(ctx context.Context, cfg Config) error {
    lis, err := listenUnix(cfg.SocketPath)   // 0600, owner-only, cleanup atexit
    if err != nil { return err }
    defer lis.Close()
    defer os.Remove(cfg.SocketPath)

    pool, err := db.Open(ctx, cfg.DBURL)
    if err != nil { return err }
    defer pool.Close()

    registry := buildRegistry(pool)

    var wg sync.WaitGroup
    for {
        conn, err := lis.Accept()
        if errors.Is(err, net.ErrClosed) { break }
        if err != nil { logAccept(err); continue }

        wg.Add(1)
        go func(c net.Conn) {
            defer wg.Done()
            defer c.Close()
            handleConnection(ctx, c, registry, pool)
        }(conn)
    }
    wg.Wait()
    return nil
}
```

Responsibilities:

- **Listen**: socket at `${XDG_RUNTIME_DIR}/striatum/daemon.sock` with
  `0600` permissions. Refuse to bind if the parent dir is world-writable
  or if the path is on a filesystem that doesn't support Unix-domain
  sockets (e.g., some NFS variants). Stale socket files left by a crashed
  predecessor are detected by attempting `unix.Connect` first; a
  responding peer means another daemon is running — exit code 14
  (`daemon_already_running`). A non-responding stale socket is removed
  before bind. **Operator-mistake footgun**: never `unlink` a socket
  without first probing it, or two daemons can both think they own the
  database.
- **Accept**: one connection per CLI client. Each connection lives for
  the duration of the CLI command (the Python client speaks `daemon.hello`
  first, then one method call, then closes). The daemon does not pool or
  reuse connections across clients; each accept gets a fresh
  authenticated session state.
- **Handshake**: the first envelope on each connection MUST be
  `daemon.hello`. If it isn't, refuse with `handshake_required`. After
  `daemon.welcome`, the daemon enforces that the negotiated envelope
  version matches every subsequent envelope on that connection.
- **Dispatch**: for each envelope, the server (a) looks up the method,
  (b) calls `CheckCapability` if the method requires any, (c) calls the
  handler with a deadline-bounded `context.Context`. Handler errors are
  wrapped into the envelope `error` branch; panics in handlers are
  recovered, logged, and returned as `internal_error` with audit row
  appended.
- **Shutdown**: a SIGTERM cancels `ctx`, which closes the listener; the
  waitgroup drains in-flight requests up to a 5-second deadline.
  SIGKILL is the operator's choice and leaves the socket file behind
  for the next start to recover from. We do not write a `pidfile`; the
  Postgres `striatumd.daemon_meta` table holds an exclusive `boot_lock`
  row (see §3.5) instead, which crosses the language boundary cleanly.

**Trust-boundary note**: the server never trusts the file system to tell
it whether a daemon is running. The authoritative liveness check is the
`boot_lock` row in Postgres. The pidfile pattern is rejected here because
it doesn't survive Python↔Go cutover (the Python daemon's pidfile shape
isn't readable by Go without copying parsing code, and vice-versa).

## 3. `go/pkg/db/` responsibilities (Step 2)

### 3.1 `connection.go`

Single concern: produce a `*pgxpool.Pool` bound to
`STRIATUM_DAEMON_DB_URL` (or `~/.config/striatum/daemon.conf` if the env
is unset, same precedence as the Python daemon).

```go
type Config struct {
    URL           string
    MinConns      int32         // default 1
    MaxConns      int32         // default 8
    MaxConnLife   time.Duration // default 1h
    StatementTO   time.Duration // default 30s — applied via pgx after-connect hook
    ApplicationName string      // "striatumd-go/<ver>"
}

func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error)
```

Behaviors:

- Refuse to start if the URL points to a hostname that isn't `localhost`,
  `127.0.0.1`, or `::1`, **unless** an explicit `--allow-remote-pg`
  flag is set. The local-first ethos and D083's single-machine assumption
  mean a remote Postgres is almost certainly an operator mistake (e.g.,
  pasting a staging URL into a dev shell). The flag exists so the (rare)
  legitimate remote case fails loudly but is recoverable.
- The pool sets `application_name=striatumd-go/<semver>` on every
  connection. The Python daemon currently sets `striatumd-python/<semver>`.
  This is the simplest way for an operator running `pg_stat_activity` to
  see which core is connected. **Operator-mistake footgun**: if both
  rows show up at the same time, the operator instantly sees the
  invariant violation (only one daemon should be connected).
- After connect, the daemon runs `SET statement_timeout = <cfg>` and
  `SET idle_in_transaction_session_timeout = '60s'`. Long-running
  read queries are bounded; abandoned transactions can't pin the bloat
  budget.
- TLS: pgx default. We do not override `sslmode`. Operators who run
  Postgres on localhost without TLS rely on Unix sockets or loopback;
  remote configurations honor the URL's `sslmode` parameter.

**Trust-boundary note**: connection-string credentials are loaded from
the environment or a 0600 config file and never logged. The pool's
internal error log is wrapped to redact `password=` fragments from any
error string that might surface in a refusal message to the CLI.

### 3.2 `migrations.go`

Phase 1 deliberately **does not change the schema**. The migrations
already shipped under `src/striatum/daemon_pg/sql/0001_baseline.sql` …
`0004_dogfood_surgical_recovery.sql` are the contract; the Go daemon
loads and applies the same files.

Two source modes, in this precedence:

1. **Filesystem** (development default): if the directory
   `src/striatum/daemon_pg/sql/` is present relative to the binary's
   working tree (resolved via `STRIATUM_REPO_ROOT` or upward search for
   `go.mod`), read SQL files from there. This is the path the contributor
   workflow uses: edit a Python-side migration, run the Go daemon, see
   the same schema.
2. **Embedded** (release default): when `STRIATUM_REPO_ROOT` is unset
   and no filesystem source is found, fall back to a
   `//go:embed sql/*.sql` filesystem baked into the binary at build time.
   `go/embed_sql.go` runs `go generate` (or a `Makefile` step) to copy
   the Python-side SQL files into `go/pkg/db/sql/` before each build, so
   the embedded copy is byte-identical with the Python source. The
   `migrations.go` constructor refuses to start if both sources exist
   but disagree on file hashes — that means the build forgot to refresh
   the embedded copy, and applying it silently would be a footgun.

The migrations table is the existing `striatumd.schema_migrations`. Each
applied migration writes `(version, label, sha256, applied_at,
daemon_version)`. The Go daemon sets `daemon_version` to
`striatumd-go/<semver>`; the Python daemon currently sets a Python
identifier. **Operator-mistake guardrail**: the daemon refuses to start
if it finds a migration row whose `sha256` differs from the SQL it would
apply for that version — that indicates someone hand-edited a frozen
migration, which would silently fork the schema between cores.

Phase 1 migration semantics:

- Read-only daemon mode means we still apply pending migrations on
  startup (to keep the substrate at the current schema version) but
  every method handler is a `SELECT`-only path. There is no read-only
  "skip migrations" mode in Phase 1 because the audit-append on
  capability denial is a write; the daemon needs the audit table to
  exist with the current shape.

### 3.3 `audit.go`

The audit-chain hash helper. RFC 0033 + RFC 0030 specify the v2 row hash;
the Python implementation lives at
`src/striatum/daemon_pg/audit.py::v2_row_hash`. The Go implementation
mirrors it byte-for-byte:

```go
func V2RowHash(row AuditRow) string {
    material := struct{
        TS               string `json:"ts"`
        SchemaVersion    int    `json:"schema_version"`
        HashFormatVersion int   `json:"hash_format_version"`
        DaemonVersion    string `json:"daemon_version"`
        ClientID         string `json:"client_id"`
        RepositoryID     string `json:"repository_id"`
        Method           string `json:"method"`
        Decision         string `json:"decision"`
        DenialReason     string `json:"denial_reason"`
        Transport        string `json:"transport"`
        RequestID        string `json:"request_id"`
        ExitCode         int    `json:"exit_code"`
        ParamsSHA256     string `json:"params_sha256"`
        PreviousHash     string `json:"previous_hash"`
        SegmentID        string `json:"segment_id"`
    }{ /* ... */ }
    body, _ := canonicaljson.Marshal(material)
    sum := sha256.Sum256(body)
    return hex.EncodeToString(sum[:])
}
```

Critical: the JSON encoder must match Python's `json.dumps(..., sort_keys=False,
separators=(", ", ": "))` byte-for-byte for the hash to match. The Python
side uses a deliberate non-default `json_dumps` (see
`src/striatum/db.py::json_dumps`); the Go implementation uses a vendored
canonical encoder that emits the same key order and separators. A parity
test computes both hashes from a fixture row and asserts equality; it is
a release blocker.

Phase 1 only implements **v2**. The Python `v1_row_hash` shape exists for
pre-RFC-0030 anchor compatibility and is not needed for new rows the Go
daemon writes. The Go verifier reads `hash_format_version` from the row
and delegates v1 verification to the Python daemon for historical
segments; the Go daemon refuses to **append** v1-shaped rows.

`audit.go` also exposes `AppendAuditRow(ctx, tx, row)` that takes a
`pgx.Tx`, performs the `SELECT previous_hash` + `INSERT` in the same
transaction (so the chain doesn't fork under concurrent writers), and
returns the new `audit_id`. Phase 1 callers are: capability denial paths
in `capability.go` and successful read calls in handlers that opt into
audit (`audit.show` itself doesn't audit its own listing; everything else
does).

### 3.4 Read-path scope (Phase 1)

The handlers registered in §2.2 access only these tables:

- `striatumd.repositories` (for `repo.list`)
- `striatumd.audit_chain` (for `audit.show`)
- `striatumd.daemon_meta` (for `daemon.status`, `daemon.version`)
- `striatumd.clients` and `striatumd.client_capabilities` (for
  capability checks)

No handler touches `striatumd.cross_repo_runs`, `striatumd.supervisor_*`,
`striatumd.apply_receipts`, or any write surface. Read-only enforcement
is structural (the handlers don't reference write tables) and
defense-in-depth: the daemon's pgx pool connects as a database role that
has `INSERT` on `striatumd.audit_chain` only and `SELECT` elsewhere in
Phase 1. The role escalation to full write access lands with Step 4.

### 3.5 `daemon_meta.boot_lock` invariant

A new (Phase 1 introduces this **only if it doesn't already exist** — the
RFC 0033 schema reserves the row key but didn't operationalize it)
`striatumd.daemon_meta` row with key `boot_lock` holds JSON like:

```json
{
  "daemon_core": "go",
  "daemon_version": "1.30.0-go.1",
  "pid": 12345,
  "hostname": "...",
  "started_at": "2026-05-13T...Z"
}
```

The daemon takes the row with `INSERT ... ON CONFLICT (key) DO UPDATE SET
value = $new WHERE value->>'pid'::int NOT IN (SELECT pid FROM
pg_stat_activity WHERE application_name LIKE 'striatumd-%')`. The
SELECT-from-`pg_stat_activity` clause means the daemon can take the lock
only if the previous owner's connection has actually dropped. **Operator-
mistake footgun guard**: a half-crashed daemon (process gone but TCP
connection lingering in `CLOSE_WAIT`) is detected and the new daemon
refuses to start, exit code 14 `daemon_already_running`. The operator
fixes the connection by killing the orphan or waiting for the TCP
timeout.

If Phase 1 lands the row but the Python daemon doesn't yet check it, the
asymmetry is acceptable for one release cycle: the Go daemon respects
the Python daemon's running state via the `application_name LIKE
'striatumd-%'` lookup, and the Python daemon is rarely co-resident with
the Go daemon in dev (operators run one or the other). The Python
daemon's `boot_lock` participation lands as a small follow-up before
Step 4.

## 4. `go/Makefile` build targets

The Makefile is contributor-side; CI orchestration lives in the top-level
Makefile and is not modified in Phase 1.

```make
.PHONY: build test vet fmt lint embed-sql clean

GO ?= go
BIN := bin/striatumd
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo "0.0.0-dev")

build: embed-sql
	$(GO) build -o $(BIN) \
	    -ldflags "-X main.Version=$(VERSION)" \
	    ./cmd/striatumd

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint:
	$(GO) run honnef.co/go/tools/cmd/staticcheck@latest ./...

embed-sql:
	@mkdir -p pkg/db/sql
	@cp ../src/striatum/daemon_pg/sql/*.sql pkg/db/sql/
	@echo "Embedded $$(ls pkg/db/sql/*.sql | wc -l) SQL files."

clean:
	rm -rf bin pkg/db/sql/*.sql
```

Notes:

- `embed-sql` is the seam between the Python source of truth and the Go
  binary. Running `make build` always refreshes the embedded copies; the
  drift-check from `migrations.go` ensures a stale embed cannot ship.
- `staticcheck` is the only linter; `golangci-lint` is rejected here
  because it pulls in 30+ analyzers that produce noise on a small
  codebase. We can add it later if the scope grows.
- No `make install` target in Phase 1; we don't promote the binary
  globally until Step 6.

## 5. Coexistence with the Python daemon (RFC 0039 §9 Phase 1)

In dogfood-042 Phase 1, both daemons exist; neither is removed; the
operator picks via:

- **Default**: the existing `striatum daemon start` invokes the Python
  daemon. No change to operator behavior unless explicitly opted in.
- **Opt-in**: `striatum daemon start --core go` invokes the Go binary.
  The Python CLI's launcher (which lands in Step 3, not Phase 1) is
  responsible for resolving the binary; Phase 1 only requires the Go
  binary to be runnable standalone via `./go/bin/striatumd
  --socket=/path --db-url=postgres://...`.

Within Phase 1's read-only scope, **coexistence is mutually-exclusive
runtime ownership of the Postgres database**:

1. **Boot-time check**: the Go daemon's startup query against
   `pg_stat_activity` (§3.5) refuses to start if any connection with
   `application_name LIKE 'striatumd-%'` is active. The Python daemon
   gets the same protection in a follow-up; until then the operator is
   expected to stop the Python daemon before starting the Go one.
2. **Socket-path separation**: the Go daemon binds to
   `${XDG_RUNTIME_DIR}/striatum/daemon.sock` by default — the same path
   as the Python daemon. The CLI doesn't have to know which core is on
   the other end; envelope-v1 is the contract. If both daemons try to
   bind, the second loses on the socket and exits cleanly; the boot_lock
   row remains the authoritative serialization point because the socket
   alone doesn't survive crashes cleanly.
3. **Schema version pinning**: both daemons read `striatumd.daemon_meta`
   for the current schema version on startup. Phase 1 introduces no
   schema changes, so version drift between cores is impossible by
   construction. The schema-drift refusal in `migrations.go` (§3.2) is
   the long-term guard for Steps 4+.

Operator-mistake footguns this design closes:

- Running both daemons against the same DB → detected at boot (§3.5).
- Pointing one daemon at the dev DB and one at the prod DB → both will
  start successfully because they have different DB URLs; this is not a
  footgun in V1 because Striatum is single-machine and there's typically
  only one DB. A documentation note in `docs/HOW_TO_HUMAN.md` (lands in
  Step 6) covers the multi-DB dev case.
- Out-of-band migration application (running `psql` manually) →
  `migrations.go` checks file sha256 vs `striatumd.schema_migrations` and
  refuses to start on mismatch.
- Old CLI talking to new daemon or vice-versa → envelope-v1 handshake
  (§3 of RFC 0030) refuses the connection with `version_incompatible`
  and exit code 10. Already enforced by the protocol; the Go daemon
  inherits it.

Footguns this design **does not** close (and explicitly defers):

- Operator running the Go binary directly via `./striatumd` and
  forgetting to set `STRIATUM_DAEMON_DB_URL`. The daemon refuses to
  start with a clear "no DB configured" error, but a tired operator
  could miss it. The CLI launcher (Step 3) wraps this in a friendlier
  flow.
- Operator pointing the Go daemon at a fresh Postgres that has never
  seen migrations. Phase 1 applies migrations 0001-0004 fresh; this is
  intended. A future RFC may add a `--require-existing-schema` flag
  for the "I expect this DB to already be set up" case.

## 6. RFC 0035 multi-repo harness extension shape

RFC 0039 §10 Step 1 names test coverage as `go test go/pkg/rpc/...`
(envelope round-trip + capability matching). Step 2 names `go test
go/pkg/db/...` against ephemeral Postgres. Phase 1's contribution to the
RFC 0035 harness is **minimal and forward-compatible**:

### 6.1 Harness parameter

`tests/_harness/multi_repo.py::MultiRepoHarness.__init__` gains:

```python
def __init__(self, *, daemon_core: Literal["python", "go"] = "python", ...):
    self._daemon_core = daemon_core
```

`tests/_harness/daemon.py::start_daemon` reads the parameter and
dispatches:

- `python` (default): existing `striatum daemon start` subprocess shape;
  no change.
- `go`: invoke the binary at `$STRIATUMD_GO_BIN` (default
  `./go/bin/striatumd`), passing `--socket=<path>` and
  `--db-url=<harness-test-pg-url>`. The harness builds the binary on
  fixture setup if missing (calls `make -C go build`).

The harness does **not** auto-cross-compile or fetch release binaries;
the Go binary is a contributor-built artifact in Phase 1.

### 6.2 Test selection for `daemon_core="go"` in Phase 1

The Go daemon in Phase 1 supports only read-only verbs. The e2e tests
that exercise mutating paths (`test_cross_repo_lifecycle_e2e.py`,
`test_mcp_capability_scope_e2e.py`, etc.) cannot run against
`daemon_core="go"` until Step 4 lands those handlers.

Phase 1 introduces **one new e2e test**:

```text
tests/test_daemon_go_smoke.py
```

That test:

1. Boots `MultiRepoHarness(daemon_core="go", repos=1)`.
2. Performs `daemon.hello` + `daemon.welcome` handshake from the Python
   CLI client.
3. Calls `daemon.describe`; asserts the method registry is exactly the
   Phase 1 read-only set.
4. Calls `daemon.status`, `audit.show`, `repo.list`; asserts each returns
   the same data shape as the Python daemon would for the same DB state.
5. Asserts the audit chain rows written by the Go daemon during the test
   validate via the **Python** verifier (`src/striatum/daemon_pg/audit.py`).
   This is the cross-language hash parity assertion that gates Phase 1
   acceptance.

Marker: `@pytest.mark.requires_go_daemon` so CI can skip the test on
runners without Go. The CI matrix change (running Go-daemon tests on
every PR) is Step 6 scope, not Phase 1.

### 6.3 Parameter shape for future steps

The `daemon_core="go"` parameter is intentionally a constructor argument
so Step 4 can run the existing e2e suite parametrically:

```python
@pytest.mark.parametrize("daemon_core", ["python", "go"])
def test_cross_repo_prepare_e2e(daemon_core, multi_repo_harness):
    ...
```

Phase 1 doesn't enable this parametrization (the tests would fail on
`go` until mutating verbs land). The parameter exists, plumbed end-to-
end, so Step 4 only needs to flip the `parametrize` decorator on per-
test once the handlers exist.

## 7. Phase 1 acceptance criteria (Steps 1+2 only)

A reviewer can verify Phase 1 by:

1. `cd go && make build` produces `bin/striatumd` with version embedded.
2. `cd go && make test` runs the Go unit tests:
   - envelope round-trip (encode/decode/refuse paths)
   - capability vocabulary parse + reject
   - method registry describe shape + etag stability
   - `db.Open` connects to the harness Postgres
   - `db.Migrations.Apply` is idempotent
   - `db.V2RowHash` matches a fixture from the Python implementation
3. `./go/bin/striatumd --db-url=$HARNESS_PG --socket=/tmp/striatumd-go.sock`
   starts, binds, and serves the Phase 1 read-only verbs.
4. From a Python CLI built against the harness:
   `striatum --socket=/tmp/striatumd-go.sock daemon describe` lists
   exactly the Phase 1 methods.
5. `pytest tests/test_daemon_go_smoke.py` passes green.
6. With the Python daemon already running, starting the Go daemon against
   the same DB exits with code 14 `daemon_already_running`.
7. Modifying a frozen SQL migration file (e.g., touching
   `0001_baseline.sql`) and rebuilding the Go daemon results in startup
   refusal with `schema_drift_detected`.

What is **not** acceptance for Phase 1 (deferred):

- Mutating verb coverage (Step 4).
- Supervised lane PTY/signal handling (Step 5).
- CLI auto-launcher and `--core go` plumbing (Step 3).
- Cross-compiled release binaries (Step 6).
- CI matrix changes (Step 6).
- Python daemon retirement (separate future RFC, §9 Phase 3).

## 8. Trust boundary summary

| Boundary | Defense |
|---|---|
| Python CLI ↔ Go daemon (Unix socket) | Owner-only socket perms (0600); envelope-v1 version handshake refuses mismatched clients; capability tokens hashed with argon2id; envelope decoder bounded at 1 MiB/line; method registry is explicit, not init-based. |
| Go daemon ↔ Postgres | Localhost-only by default (override with explicit flag); parameterized queries everywhere; `statement_timeout` + `idle_in_transaction_session_timeout` set per session; Phase 1 role has INSERT only on `audit_chain`, SELECT elsewhere; connection-string credentials never logged. |
| Go daemon ↔ filesystem | SQL migration files validated by SHA-256 against `schema_migrations` rows; embedded SQL is checked against filesystem when both present; socket parent dir refuses world-writable mounts; binary refuses to start without explicit DB URL. |
| Cross-core coexistence | `striatumd.daemon_meta.boot_lock` row + `pg_stat_activity` liveness check is the single source of truth for "is a daemon running"; `application_name` makes it visible in operator tooling; SQL migration sha256 drift detection prevents silent schema fork; envelope-v1 is the contract on both sides. |

## 9. Out-of-scope reminders

Per the work packet and RFC 0039 §10, this design intentionally does
**not** cover:

- **Step 3** (CLI integration): the Python CLI's `--core go` flag wiring,
  binary discovery via `STRIATUMD_GO_PATH`, fallback behavior when the
  binary is missing, and CLI-side launching of the Go subprocess. These
  belong in the Phase 2 dogfood.
- **Step 4** (mutating verbs + apply): cross-repo prepare/start/cancel,
  `tools/call`, capability token issuance, apply-receipt signing. The
  registry shape in §2.2 can absorb these without restructuring; the
  handlers and the write-role escalation in §3.4 are Step 4 work.
- **Step 5** (supervised processes): Go's `os/exec` + `creack/pty`,
  packet delivery via FIFO, supervised-progress lease heartbeat,
  deterministic SIGTERM drain. The signal-handling skeleton in §2.4 is
  the minimal foundation; the supervised-lane lifecycle is Phase 2+.
- **Step 6** (distribution): cross-compilation matrix, release pipeline,
  PyPI wheel-with-binary packaging, `docs/SPEC.md`/`docs/HOW_TO_HUMAN.md`
  updates, top-level `Makefile` `daemon-go-*` targets.

Anyone reading this design and looking for those topics: they are
deliberately absent, not forgotten.

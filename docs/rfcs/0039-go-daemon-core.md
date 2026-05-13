# RFC 0039: Go Daemon Core

Status: proposed (Phase 1 Steps 1+2 landed in dogfood-042; Steps 3-6 deferred to a Phase 2 dogfood)
Date: 2026-05-13
Context:
[`RFC 0028`](0028-long-running-daemon-and-multi-repository-control-plane.md),
[`RFC 0030`](0030-daemon-rpc-server-and-version-skew-protocol.md),
[`RFC 0031`](0031-daemon-owned-supervision-and-sealed-apply-boundary.md),
[`RFC 0032`](0032-cross-repo-workflows-and-mcp-mutation-capabilities.md),
[`RFC 0033`](0033-storage-substrate-rewrite-for-daemon-v2.md),
[`RFC 0035`](0035-multi-repo-test-harness-for-cross-repo-workflows.md),
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D082, D084, D086, D087, D088),
`src/striatum/daemon.py`,
`src/striatum/daemon_rpc/`,
`src/striatum/daemon_apply/`,
`src/striatum/daemon_supervisor/`,
`src/striatum/daemon_pg/`

## Problem

D084 planned a Go-language core for the daemon. The reasoning at the
time of D084 was: "Daemon-first product positioning makes long-running
process supervision, signal handling, packaging, and single-binary
distribution first-class concerns. Go is a better fit for that
operational surface than Python, but a rewrite has cost; designing the
protocol to be language-agnostic now avoids relitigating the wire
format later."

The conditions D084 listed as worth revisiting the rewrite cost have
arrived:

1. **RFC 0030 has shipped** (dogfood-034, v1.23.0). The daemon RPC
   envelope-v1, version-skew handshake, capability-bound method
   registry, and audit chain helpers are all language-agnostic from
   day one — exactly as D084 required.
2. **RFC 0033 has shipped** (dogfood-033, v1.22.0). The PostgreSQL
   substrate is the daemon's authoritative store; the Go daemon does
   not need to reimplement SQLite handling. The daemon DB schema is
   defined by SQL migrations under `src/striatum/daemon_pg/sql/` which
   can run unchanged against the same PostgreSQL instance.
3. **RFC 0031 has shipped** (dogfood-034 + dogfood-035, v1.23.0 +
   v1.24.0). Daemon-owned supervisor metadata, apply receipt schema,
   and sealed-apply authority semantics are nailed down.
4. **RFC 0032 has shipped** (dogfood-035, v1.24.0). MCP capability-
   gated `tools/call` + `tools/list` filtering + audit row append are
   complete.
5. **RFC 0035 has shipped** (dogfood-037, v1.27.0). The multi-repo
   test harness exercises every RFC 0032 V2 threat surface end-to-end.
   The Go daemon can be validated against the same harness.
6. **Daemon V2 surface area has stabilized.** The Python daemon
   currently spans `src/striatum/daemon.py` (foreground supervision,
   process boot, signal handling), `src/striatum/daemon_rpc/` (RPC
   server + method registry + capability + envelope), `src/striatum/
   daemon_apply/` (apply service), `src/striatum/daemon_supervisor/`
   (supervisor pointers), and `src/striatum/daemon_pg/` (Postgres
   substrate). Python is currently fighting itself on three of these
   surfaces:
   - **Long-running process supervision.** Python's signal handling
     interacts poorly with threaded subprocess management. Several
     dogfoods (036, 037, 038) hit minor friction patterns around
     supervisor cleanup, stale leases under active load, and
     PID-recycling correctness checks. Go's goroutine + signal-channel
     model is the well-trodden path for this shape.
   - **Single-binary distribution.** The daemon currently ships as
     a `striatumd` console script that depends on the full
     `striatum-orchestrator` wheel + `psycopg[binary]` + the Python
     runtime version compatibility matrix. A Go daemon can ship as
     a single statically-linked binary per platform, addressable via
     `go install` or a downloadable artifact.
   - **PTY handling for supervised lanes.** Python's `pty` module and
     `subprocess.Popen` interaction is platform-specific and has
     surprised us in cross-platform tests. Go's `os/exec` + `pty`
     packages are well-tested across Linux/macOS.

The CLI client (the `striatum` command) stays in Python. It is the
operator's surface and benefits from Python's REPL-debuggability,
docstring tooling, and pip distribution. CLI ↔ daemon communication
goes over the RFC 0030 envelope-v1 protocol (Unix socket, JSON), which
is already language-agnostic by D084 design.

RFC 0039 scopes the actual Go rewrite: layout, build, distribution,
test parity, migration path, retirement of the Python daemon.

## Goals

- Rewrite the daemon core in Go: process supervision, RPC server,
  apply service, supervisor metadata management, Postgres database
  access layer.
- Preserve the RFC 0030 envelope-v1 wire protocol unchanged. CLI
  clients (the Python `striatum` command and any future Go test
  client) talk to the daemon via the same Unix socket + JSON
  framing.
- Preserve the RFC 0033 Postgres substrate unchanged. Same schema,
  same migrations, same audit chain helper semantics. The Go daemon
  reads/writes the same DB.
- Ship the Go daemon as a single statically-linked binary per
  platform (Linux x86_64, Linux arm64, macOS x86_64, macOS arm64).
- Maintain test parity: the RFC 0035 multi-repo test harness must
  cover the Go daemon end-to-end before the Python daemon is
  retired. The harness already boots a daemon subprocess; the
  binary just needs to be the Go one.
- Provide a migration path: Python daemon and Go daemon coexist
  during transition; one or the other is selected via
  configuration; eventual retirement of the Python daemon happens
  in a separate RFC.

## Non-Goals

- Rewriting the CLI in Go. The operator-facing CLI stays Python.
- Rewriting the agent SDK (`striatum.skills`, `striatum.plugin`,
  `striatum.workflow_generator`, `striatum.web`). These are
  CLI/web-side and stay Python.
- Multi-machine / hosted-mode daemon (D083 out of scope).
- Windows daemon support. Per RFC 0030/0031 V2 scope, Windows
  daemon is deferred. Linux + macOS only for the Go rewrite too.
- Changing the wire protocol, schema, or audit-chain semantics.
- Changing the operator-facing semantics of `striatum daemon
  start`, `repo add/list/remove`, `recovery *`, dashboard, MCP, etc.
- Performance optimization beyond what's required for correctness.
  The Go rewrite's motivation is operational surface (signals,
  packaging, PTY) not raw throughput.
- Sealing the apply path behind cryptographic non-repudiation. RFC
  0031's threat model (AI guardrail, not malicious-local-root) is
  preserved.

## External Prior Art

Daemon-and-CLI-in-different-languages is a common pattern:

- **Docker** — daemon in Go (`dockerd`), CLI in Go (also). The
  Go-daemon-Go-CLI pattern is the most-trodden, but Striatum's
  Python CLI is intentionally kept because the operator surface
  benefits from Python tooling.
- **Kubernetes** — control plane in Go (`kube-apiserver`,
  `kube-controller-manager`), various CLIs in Go but also Python
  (`kubectl-helm-python`, etc.). The pattern of "Go control plane
  + per-language CLIs over a stable wire protocol" matches
  Striatum's intent.
- **containerd** — Go daemon, gRPC API, multiple language clients.
  Wire protocol stability is the key constraint; Striatum's RFC
  0030 envelope-v1 fills the same role.
- **Buildkit** — Go daemon, gRPC API. Similar shape.
- **PostgreSQL** — server in C, clients in everything. The
  precedent of "server in operationally-strong language, clients
  everywhere" is decades old.

## Proposal

### 1. Repository layout

A new top-level directory `go/` houses the Go daemon. The existing
Python package tree under `src/striatum/` stays.

```
striatum/
  src/striatum/           # Python CLI + agent SDK + web UI (unchanged)
  go/
    cmd/
      striatumd/          # daemon main package
        main.go
    pkg/
      rpc/                # RFC 0030 envelope-v1 implementation
        envelope.go
        registry.go
        capability.go
        server.go
      apply/              # RFC 0031 apply service
        receipt.go
        service.go
      supervisor/         # supervised process owner
        pointer.go
        liveness.go
        pty.go
      db/                 # daemon Postgres substrate
        connection.go
        migrations.go
        audit.go
      mcp/                # RFC 0032 MCP tools/call + tools/list
        capabilities.go
        tools.go
      crossrepo/          # RFC 0032 cross-repo run lifecycle
        prepare.go
        lifecycle.go
    go.mod
    go.sum
    Makefile              # contributor-side build for the go daemon
  Makefile                # top-level; gains daemon-go-* targets
  docs/
  tests/                  # Python tests stay; new Go tests under go/
```

The Go daemon's source is **separate** from the Python source. The
RFC 0030 wire protocol is the shared contract; neither side imports
the other.

### 2. Wire protocol contract

RFC 0030 envelope-v1 is the unchanged contract. The Go daemon implements
the same:

- Unix socket transport with owner-only permissions.
- Newline-delimited JSON envelope per request.
- `daemon.hello` / `daemon.welcome` version handshake.
- `daemon.describe` method registry exposition.
- Capability-bound method registry (seven capabilities: `read`,
  `write`, `review`, `claim`, `apply`, `admin`, `recovery`).
- Audit row append for every mutating method call.

The Python CLI client code under `src/striatum/daemon_rpc/client.py`
(or equivalent) talks to either daemon implementation interchangeably;
no client-side change.

### 3. Database contract

RFC 0033 Postgres substrate is the unchanged contract. The Go daemon:

- Reads connection details from the same `STRIATUM_DAEMON_DB_URL` env
  var or `~/.config/striatum/daemon.conf`.
- Runs migrations from `src/striatum/daemon_pg/sql/*.sql` (or a Go
  embedding of those same SQL files). Migrations are Postgres SQL;
  they don't care which language applies them.
- Uses the same schema (cross_repo_runs, audit_chain, capability
  tokens, supervisor pointers, etc.).
- Uses the same audit-chain hash helper (defined as SQL/Postgres
  function or replicated in Go).

The Python daemon and Go daemon are **mutually exclusive** in a given
run: only one daemon owns the Postgres database at a time. The pidfile
+ socket-path lock prevents concurrent daemons.

### 4. Selection mechanism

A new operator-side config flag chooses which daemon implementation
runs:

- `striatum daemon start` (Python CLI) defaults to the Python daemon
  for backwards compat during transition.
- `striatum daemon start --core go` boots the Go daemon binary instead.
- `STRIATUM_DAEMON_CORE=go` env var sets the default.
- A future RFC retires the Python daemon and flips the default to Go.

The Python CLI launches the Go daemon as a subprocess via the
installed binary path (looked up via `which striatumd-go` or a
configured `STRIATUM_DAEMONGO_PATH`). The Python CLI client speaks
envelope-v1 over the Unix socket regardless of daemon language.

### 5. Distribution

**Per-platform binaries:**

- `go install` from the source tree produces `striatumd-go` on the
  contributor's machine.
- CI cross-compiles for linux-amd64, linux-arm64, darwin-amd64,
  darwin-arm64 and uploads the binaries as release artifacts.
- Future RFC may explore a `pip install striatum-daemon-go` PyPI
  package that ships the per-platform binary as a Python wheel
  (similar to `psycopg[binary]`).

**Operator install during transition:**

```bash
pip install striatum-orchestrator    # CLI + Python daemon + web UI (unchanged)
# Optional: install Go daemon binary
curl -L https://github.com/halbritt/striatum/releases/download/v<ver>/striatumd-go-linux-amd64 -o ~/.local/bin/striatumd-go
chmod +x ~/.local/bin/striatumd-go
striatum daemon start --core go
```

After the Go daemon is the default, the Python daemon becomes optional
and a future RFC may retire it entirely.

### 6. Process supervision

The Go daemon owns the supervised-lane subprocesses (agent CLIs in
`bash -lc '... | exec <wrapper>.sh'` shape). Per RFC 0031, supervisor
metadata lives in the daemon DB. The Go implementation:

- Spawns supervised processes with `os/exec` + `creack/pty` (the
  well-trodden Go PTY library).
- Writes packet JSON to the supervised wrapper's stdin via the FIFO
  pipe (same shape as the Python daemon).
- Heartbeats the lease while the supervised process is making forward
  progress (per the friction note from dogfood-038 OPERATOR_REPORT
  intervention #5: supervised-progress lease heartbeat).
- Cleans up subprocesses on SIGTERM via deterministic signal channel
  + waitgroup drain (Go's well-trodden pattern).

### 7. Test parity

The RFC 0035 multi-repo test harness boots a daemon subprocess via
`tests/_harness/daemon.py`. Extending the harness:

- New `daemon_core` parameter (`"python"` or `"go"`) on the
  `MultiRepoHarness` constructor.
- When `daemon_core="go"`, the harness invokes `striatumd-go` instead
  of the Python `striatum daemon start`.
- All five e2e test files (prepare, lifecycle, crash-recovery,
  MCP capability scope, per-repo write-scope) run against both daemon
  cores in CI.
- The acceptance bar for shipping the Go daemon is parity: every test
  passing for Python must pass for Go.

Per-language unit tests:

- Go tests under `go/pkg/*/test_*.go` exercise the Go daemon's
  internal behaviors (envelope parsing, capability matching,
  PTY interaction, signal handling).
- Python tests under `tests/*` stay unchanged.

### 8. Audit-chain semantics

The audit-chain hash is a SQL function in `src/striatum/daemon_pg/sql/`.
The Go daemon calls the same function. The chain is verified
end-to-end by the same harness assertions (RFC 0035). No language-
specific audit-chain code; the chain is a property of the daemon DB
schema.

### 9. Migration path

Three phases:

**Phase 1 — coexistence (RFC 0039 V1 scope):**
- Both daemons exist; operators choose via `--core` flag.
- Default stays Python.
- Multi-repo test harness covers both.
- CI runs both daemon test matrices.
- Documentation labels each daemon's tradeoffs.

**Phase 2 — Go default (separate future RFC):**
- After production validation, flip the `striatum daemon start`
  default to Go.
- Python daemon stays as a fallback for one release cycle.

**Phase 3 — Python retirement (separate future RFC):**
- Python daemon code removed from `src/striatum/`.
- The Python `daemon_rpc/`, `daemon_apply/`, `daemon_supervisor/`,
  and `daemon_pg/` packages become library-only (used by CLI client
  for envelope parsing); the daemon-server code is removed.
- Single-binary `striatumd-go` is the only daemon.

RFC 0039 covers Phase 1 only.

### 10. CI matrix

Existing CI matrix gains:

- Go 1.23 (or current Go LTS) toolchain setup.
- `go build ./...` step for Go daemon binaries.
- `go test ./...` for Go-side unit tests.
- `make test-multi-repo CORE=go` runs the harness against the Go
  daemon.
- Cross-compilation for linux-arm64, darwin-amd64, darwin-arm64
  artifacts (release-time only; not every PR).
- Bundle hash check for the Python wheel does NOT include the Go
  binary (different artifact, different release pipeline).

## Acceptance Criteria

- `go/` directory layout exists with `cmd/striatumd/`, `pkg/{rpc,
  apply,supervisor,db,mcp,crossrepo}/`, `go.mod`, and
  `go/Makefile`.
- Go daemon implements the full RFC 0030 envelope-v1 + version
  handshake + method registry + capability gating.
- Go daemon reads/writes the RFC 0033 Postgres substrate using the
  same schema + migrations as the Python daemon.
- Go daemon owns supervised processes per RFC 0031, including PTY
  + signal handling + supervised-progress lease heartbeat.
- Go daemon implements the RFC 0032 cross-repo run lifecycle +
  MCP `tools/call` + `tools/list` + audit append.
- `striatum daemon start --core go` launches the Go daemon binary
  via the Python CLI.
- `MultiRepoHarness(daemon_core="go")` boots the Go daemon and runs
  all five e2e test files green.
- CI runs the Python and Go daemon test matrices on every PR.
- Cross-compile produces linux-amd64 + linux-arm64 + darwin-amd64
  + darwin-arm64 binaries on release.
- Distribution: a release ships the wheel (Python) + four Go binaries
  per platform.
- Documentation: `docs/SPEC.md` daemon section names both daemon
  cores; `docs/HOW_TO_HUMAN.md` documents the `--core go` flag and
  installation; `CHANGELOG.md` records the addition.
- No regression in any existing Python test.

## Implementation Plan

This is a large rewrite. Six phases land independently with green
test parity at each step.

> Status (dogfood-042): Steps 1+2 are landed per the
> [Track A synthesis](../dogfood/042/track_a/DESIGN_SYNTHESIS.md). The
> Go daemon now exposes the read-only RPC envelope-v1 method registry
> (`daemon.hello`, `daemon.welcome`, `daemon.describe`, `daemon.status`,
> `daemon.version`, `audit.show`, `repo.list`) on top of the RFC 0033
> PostgreSQL substrate, with a cross-language v2 audit-row hash that
> the Python verifier accepts. The RFC 0035 multi-repo test harness
> gained a `daemon_core` parameter (`"python"` default; `"go"` opts in)
> so e2e fixtures can target either core. Steps 3-6 — the Python CLI
> `striatum daemon start --core go` flag, mutating verbs / apply,
> supervised processes, and distribution / CI matrix — are deferred to
> a Phase 2 dogfood.

### Step 1. Skeleton + envelope-v1

Land `go/cmd/striatumd/`, `go/pkg/rpc/{envelope,registry,capability,server}.go`,
`go.mod`. Implement: socket listen, accept, envelope parse/serialize,
hello/welcome handshake, describe, capability table, ack/refuse paths.
Tests: `go test go/pkg/rpc/...` covers envelope round-trip and
capability matching. The daemon at this stage handles read-only RPC
verbs only.

### Step 2. Postgres substrate

Land `go/pkg/db/{connection,migrations,audit}.go`. Implement
connection pool, migration loader (reading from
`src/striatum/daemon_pg/sql/*.sql`), and the audit-chain wrapper.
Tests: `go test go/pkg/db/...` against ephemeral Postgres (matching
the RFC 0035 harness path).

### Step 3. Read-only daemon (CLI integration)

Wire the Python CLI's `striatum daemon start --core go` to launch
the Go binary. The Go daemon handles read-only verbs (status,
dashboard, audit show) from the Python CLI. First end-to-end test:
`MultiRepoHarness(daemon_core="go")` smoke + `test_cross_repo_prepare_e2e`
read-only assertions.

### Step 4. Mutating verbs + apply

Land `go/pkg/apply/`, `go/pkg/mcp/`, `go/pkg/crossrepo/`. Implement
the full mutation verb table: cross-repo prepare/start/cancel,
`tools/call`, capability token issuance/revocation/expiry. Run the
full RFC 0035 harness against `daemon_core="go"`; iterate until
green.

### Step 5. Supervised processes

Land `go/pkg/supervisor/{pointer,liveness,pty}.go`. Implement
supervised-lane spawning with PTY, packet delivery via FIFO,
heartbeat from supervised-progress signal, deterministic cleanup
on SIGTERM. Smoke test with a real codex/claude/gemini supervised
lane.

### Step 6. Distribution + docs

Cross-compile the four platform binaries in CI. Land
`docs/SPEC.md` daemon section update, `docs/HOW_TO_HUMAN.md` flag
documentation, `CHANGELOG.md` entry. Tag a release with both wheel +
Go binaries.

## Open Questions

- Should the Go daemon's source live in this repo or a separate
  `striatum-daemon-go` repo? Recommendation: same repo for V1 to
  keep the wire protocol contract changes co-located; consider
  splitting only if the Python and Go release cadences diverge.
- Should the Go daemon use a Go gRPC + protobuf stack instead of the
  RFC 0030 envelope-v1 JSON-over-Unix-socket protocol? Recommendation:
  no — RFC 0030 is the contract; switching to protobuf would force
  the Python CLI client to also switch and break compatibility. JSON
  envelope is intentionally simple.
- Should the Go daemon ship as a Python wheel (`pip install
  striatum-daemon-go`) using a `cibuildwheel`-style binary-payload
  approach? Recommendation: V1 ships as separate downloadable
  binaries; PyPI wheel-with-binary follow-up RFC if operators
  prefer pip-only install.
- Should the Go daemon take over the apply-receipt signing path
  from RFC 0031? Recommendation: yes — apply-receipts are daemon-
  owned per D088; the Go daemon implements the same fail-closed
  authority semantics.
- Should the Go daemon expose Prometheus metrics? Recommendation:
  no — local-first ethos says no telemetry surface. Operators
  who want metrics can scrape the audit chain.
- Should the existing Python daemon `striatum daemon start` keep
  working forever, or be removed in a future RFC? Recommendation:
  remove in a future RFC (Phase 3 per §9) after Go has been the
  default for one release cycle.

## Domain Modeling

This RFC adds an alternative implementation of the existing daemon
domain, not new aggregates. The daemon's domain (RPC method registry,
capability tokens, audit chain, cross-repo runs, supervised processes,
apply receipts) is preserved verbatim across languages. The wire
protocol (RFC 0030 envelope-v1) and the storage substrate (RFC 0033
Postgres) are the language-independent contracts.

The single relevant new concept is **daemon core**: a value object on
the operator-facing configuration enumerating which language
implementation is running. V1 closed set: `{python, go}`. V1 default:
`python`. The operator sees `striatum daemon describe` reflect the
current core; the CLI client behavior is identical against either.

## V1.5 Deltas (correctness slice)

V1 shipped a Go daemon that bound the envelope-v1 socket and applied
migrations but had five correctness gaps that blocked promotion of the
Go core to operator workloads. V1.5 closes those gaps and is the merge
slice before mutating routes land in Step 4.

The findings are pinned to dogfood-047 designs and the implementation
order is locked by `docs/dogfood/047/DESIGN_SYNTHESIS.md`. F5 lands
before F4 and F1 because those two correctness fixes need the
parameter-binding and transaction support of the new driver; F2 and F3
land after the daemon can authorize and audit requests correctly.

### F5 — Pure-Go PostgreSQL driver

`go/pkg/db/connection.go` no longer shells out to `psql`. The connection
pool is `pgx/v5` (the first third-party Go runtime dependency for this
repository — `go/go.mod` now requires `github.com/jackc/pgx/v5 v5.7.2`,
with five indirect modules). The pool is configured with
`application_name = "striatumd-go/<daemon_version>"`, a default
`statement_timeout`, and the PostgreSQL simple protocol so the embedded
multi-statement migration files keep working unchanged while parameters
are still bound through the driver. `db.Runner` is the
parameter-aware database surface used by the rest of the daemon, and
`db.TxRunner` is its transactional sibling.

### F4 — Transactional audit append

`go/pkg/db/audit.go` no longer races. `AuditRecorder.RecordRPC` opens
one `READ COMMITTED` transaction, locks the singleton
`striatumd.audit_chain_head` row with `FOR UPDATE`, computes the row
hash from the locked `previous_hash`, inserts the new audit row with
`RETURNING audit_id`, updates the chain head, and commits. The returned
audit id flows back into the RFC 0030 response so the chain remains
linear under concurrent RPC traffic. The opt-in Go race test in
`go/pkg/db/audit_race_test.go` exercises this against an ephemeral
Postgres URL (`STRIATUM_PG_TEST_URL`); the Python cross-core regression
lives in `tests/test_daemon_go_audit.py` and runs under
`make test-multi-repo CORE=go`.

### F1 — PostgreSQL-backed RPC authorization

`go/pkg/rpc/auth_pg.go` introduces `PostgresAuthorizer`, which validates
Python-issued capability tokens against the same `striatumd.clients` /
`striatumd.client_capabilities` rows the Python authorizer uses. Token
secrets are HMAC-SHA256 compared with `subtle.ConstantTimeCompare`
against the stored salt+hash, and the capability lookup mirrors the
Python query (including the `repository_id IS NULL OR repository_id =
$3` wildcard rule and the scope-mismatch fallback). Denial reasons line
up one-for-one with `src/striatum/daemon_rpc/capability.py` so clients
cannot tell the two cores apart from the refusal envelope. The serving
daemon in `go/cmd/striatumd/main.go` wires this authorizer when a
PostgreSQL URL is configured; `AllowAllAuthorizer` is now test-only.

### F2 — Go harness launch contract

The Go binary is the canonical CLI: it accepts `--socket`,
`--postgres-url`, `--migrate`, `--describe`, and the new optional
`--migrations-sha-source`. The SHA-source flag compares the embedded
migration file hashes against the SQL files on disk before serving and
exits non-zero on drift — that replaces V1's `--migrations-dir`
re-loader without giving up the drift signal. `go/Makefile` writes
`go/bin/striatumd` so `tests/_harness/daemon.py` can locate the binary
without an environment override; `STRIATUMD_GO_BIN` remains a trusted
developer-environment override.

### F3 — `make test-multi-repo CORE=go`

`Makefile` accepts `CORE ?= python` and forwards it through
`STRIATUM_MULTI_REPO_DAEMON_CORE`. The class-scoped `daemon_core`
fixture in `tests/conftest.py` reads that variable and passes it to
`MultiRepoHarness`; the test list now includes
`tests/test_daemon_go_smoke.py` and `tests/test_daemon_go_audit.py` so
the Go-core matrix exercises a real boot, a read-only RPC, and the F4
audit chain. The CI shape is intentionally two explicit jobs
(`CORE=python`, `CORE=go`) rather than in-process parametrization, so
the Go-core evidence is intentional rather than implied.


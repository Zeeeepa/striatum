# RFC 0039 V1.5 Go Daemon Correctness Deltas

author: designer-unknown-model-001

Status: handoff
Date: 2026-05-13

## Scope

This handoff designs the RFC 0039 V1.5 repair slice for dogfood-042
Track A build review findings F1-F5. It deliberately stays below RFC
0039 V2 scope: no new daemon capabilities, no Python-daemon retirement,
no hosted mode, and no harness rewrite beyond Go-core launch and
parameter selection.

Primary review source is the codex build review at
`docs/dogfood/042/track_a/review/build/codex/REVIEW.md`, which returned
`needs_revision` for fail-open authorization, broken Go harness launch,
missing Go parity wiring, non-transactional audit append, and `psql`
shell-out database access. Claude corroborated the launch and binary
path breakage in its TB-Glue-1/TB-Glue-2 findings, and Gemini
corroborated the DB/audit attack surface by calling out shell-out SQL
construction, credential leakage through process arguments, and audit
chain race risk.

## Current Implementation Facts

`go/cmd/striatumd/main.go:24-28` accepts `--socket`,
`--postgres-url`, `--migrate`, and `--describe`. The same file wires the
server at `main.go:50-56`, setting `server.Authorizer =
rpc.AllowAllAuthorizer{}` even when a PostgreSQL connection and audit
recorder exist.

`go/pkg/rpc/capability.go:21-23` defines the production-facing
`Authorizer` interface. `AllowAllAuthorizer` returns `Decision:
"allowed"` at `capability.go:25-33`; `MemoryAuthorizer` at
`capability.go:50-130` already models the denial vocabulary but is
in-memory and hashes secrets as `sha256(secret)`, which does not match
the Python daemon token hash.

The daemon token schema already exists in both SQL trees. The current Go
copy declares `striatumd.clients` with `token_id`, `token_hash`,
`token_salt`, `expires_at`, and `revoked_at` at
`go/pkg/db/sql/0001_baseline.sql:44-55`, and declares
`striatumd.client_capabilities` with `repository_id`, `capability`,
`expires_at`, and `revoked_at` at
`go/pkg/db/sql/0001_baseline.sql:57-66`. The Python source of truth has
the same shape at `src/striatum/daemon_pg/sql/0001_baseline.sql:44-66`.
The Python harness creates compatible rows in
`tests/_harness/tokens.py:13-48`, using `striatum.daemon._hash_token`;
that hash is HMAC-SHA256 keyed by salt at
`src/striatum/daemon.py:420-421`. The Python RPC authorizer implements
the parity behavior at `src/striatum/daemon_rpc/capability.py:25-98`,
including `token_missing`, `token_malformed`, `token_invalid`,
`token_revoked`, `token_expired`, `capability_scope_mismatch`,
`capability_missing`, and `capability_expired`.

The harness already has a `daemon_core` constructor parameter in
`tests/_harness/multi_repo.py:26-44` and passes it into
`DaemonProcess` at `multi_repo.py:63-67`, but the pytest fixture in
`tests/conftest.py:15-21` does not select or parametrize that core. The
Go start path expects the binary at `tests/_harness/daemon.py:22-24`,
auto-builds with `make -C go build` at `daemon.py:49-53`, then invokes
`--db-url` and `--migrations-dir` at `daemon.py:114-125`. Those flags
do not exist in `main.go:24-28`, and `go/Makefile:3-4` builds without
`-o`, producing `go/striatumd` rather than `go/bin/striatumd`.

The top-level Makefile has `daemon-go-build`, `daemon-go-test`, and
`daemon-go-lint` at `Makefile:69-76`; `test-multi-repo` at
`Makefile:81-88` always runs the same pytest command with no `CORE`
plumbing. The marked multi-repo tests include
`tests/test_multi_repo_harness.py`, `tests/test_cross_repo_prepare_e2e.py`,
`tests/test_cross_repo_lifecycle_e2e.py`,
`tests/test_cross_repo_crash_recovery_e2e.py`,
`tests/test_mcp_capability_scope_e2e.py`, and
`tests/test_per_repo_write_scope_e2e.py`.

The Go audit recorder reads chain head and segment id before constructing
the row at `go/pkg/db/audit.go:49-82`, computes the row hash at
`audit.go:83-86`, then inserts and updates chain head in one SQL string
at `audit.go:87-120`. There is no transaction, isolation level, or
`FOR UPDATE` lock across read-head plus insert/update. The schema stores
`audit_log.previous_hash`, unique `row_hash`, and the singleton
`audit_chain_head` at `go/pkg/db/sql/0001_baseline.sql:83-108`.

Database access currently shells out through `PsqlRunner`. `Connect`
returns `PsqlRunner{URL: postgresURL}` at `go/pkg/db/connection.go:78-87`;
`Exec` and `QueryScalar` call `exec.CommandContext(ctx, "psql", r.URL,
...)` at `connection.go:105-125`. That keeps SQL construction stringly
typed and places DB credentials in process argv.

## F1: Postgres-Backed RPC Authorization

Replace production `AllowAllAuthorizer` wiring with a new
`db.Authorizer` or `rpc.PostgresAuthorizer` backed by the same
PostgreSQL pool used for migrations and audit. `AllowAllAuthorizer`
may remain only for unit tests that instantiate `rpc.NewServer()`
without a database; `go/cmd/striatumd/main.go:50-56` must fail closed
when serving ordinary routes with `--postgres-url` configured and no
Postgres-backed authorizer available.

The new authorizer should implement the existing interface:

```go
type Authorizer interface {
    Authorize(required *Capability, repositoryID string, token string) AuthContext
}
```

Lookup behavior must match `src/striatum/daemon_rpc/capability.py:25-98`.
If `required == nil`, return allowed without a token for handshake-only
routes. Otherwise, validate that the requested capability is in
`rpc.Capabilities`; reject missing, malformed, unknown, revoked, and
expired tokens with the same denial reason strings as Python. Split
tokens as `<token_id>.<secret>`, fetch `striatumd.clients` by
`token_id`, compute HMAC-SHA256 with the row's `token_salt` and the
supplied secret, compare with `token_hash` using constant-time compare,
then check `revoked_at` and `expires_at`.

Capability lookup should use the Python ordering semantics:

```sql
SELECT *
FROM striatumd.client_capabilities
WHERE client_id = $1
  AND capability = $2
  AND (repository_id IS NULL OR repository_id = $3)
  AND revoked_at IS NULL
ORDER BY repository_id IS NULL
LIMIT 1
```

If no matching row exists but another non-revoked row for the same
client/capability is scoped to a different repository, deny with
`capability_scope_mismatch`; otherwise deny with `capability_missing`.
If the selected grant is expired, deny with `capability_expired`.
Allowed authorization updates `striatumd.clients.last_used_at = now()`.

Do not add a positive authorization cache in V1.5. Token and grant
revocation must take effect on the next request, and this daemon is not
yet bottlenecked on authorization reads. A later cache may be added only
with an explicit maximum TTL and revocation invalidation story. A small
negative parse cache is unnecessary because malformed tokens are cheap.

Denial response shape stays the existing RFC 0030 error path:
`rpc.RequireAllowed` returns `rpc.NewError(reason, "daemon RPC
authorization failed", nil)` at `go/pkg/rpc/capability.go:132-140`,
and `Server.Handle` turns that into an error response at
`go/pkg/rpc/server.go:77-97`. Audit must still run for denied requests:
`Server.Handle` already calls `AuditRecorder.RecordRPC` after response
construction at `server.go:98-103`, so the authorizer only needs to
return `AuthContext` with `ClientID`, `TokenID`, `RepositoryID`,
`Capability` when allowed, `Decision`, and `DenialReason`. This closes
the codex F1 fail-open finding and preserves Gemini's requirement that
denials remain auditable.

Acceptance tests:

- Go unit test for token hashing parity using a row issued with the
  same salt/secret shape as `tests/_harness/tokens.py:20-35`.
- Go unit test covering missing, malformed, invalid, revoked, expired,
  capability missing, scope mismatch, capability expired, and allowed.
- Python e2e test using `MultiRepoHarness(daemon_core="go")` against
  `tests/test_mcp_capability_scope_e2e.py` equivalents for
  `token_revoked`, `token_expired`, `capability_missing`, and
  `capability_scope_mismatch`, then asserting the corresponding
  `audit_log.denial_reason`.

## F2: Harness Launch Fix For `daemon_core="go"`

Use the Go binary's current flag vocabulary rather than adding duplicate
aliases. The harness command at `tests/_harness/daemon.py:117-125`
should become:

```python
cmd = [
    str(binary),
    "--socket",
    str(self.socket_path),
    "--postgres-url",
    self.postgres_url,
]
```

`--migrate` defaults true in `go/cmd/striatumd/main.go:26`, so the
harness does not need to pass it. `--migrations-dir` remains out of
scope for F2 because the Go daemon currently embeds SQL; if a later
design reverts to loading Python SQL files directly, that should be a
separate migration-drift fix with SHA verification. For V1.5, the
launch contract is: `STRIATUMD_GO_BIN` may point at a trusted local test
binary, otherwise `make -C go build` produces the default binary at the
path the harness expects.

Change `go/Makefile` to build to `bin/striatumd`:

```make
BIN ?= bin/striatumd

build:
	mkdir -p $(dir $(BIN))
	go build -o $(BIN) ./cmd/striatumd
```

That aligns `_DEFAULT_GO_BIN = ROOT / "go" / "bin" / "striatumd"` at
`tests/_harness/daemon.py:22-24` with the build artifact. The top-level
`daemon-go-build` target at `Makefile:69-70` should keep delegating to
`make -C go build`; no new top-level binary path is needed.

Add a small smoke test in `tests/test_multi_repo_harness.py` or a new
`tests/test_go_daemon_harness.py` that constructs
`MultiRepoHarness(daemon_core="go", ...)`, starts it, asserts the socket
exists, calls `daemon.describe` through the RPC client or performs a
read-only MCP/status call with a read token, and stops cleanly. Mark it
`multi_repo` and skip with a clear reason if `go` or `make` is
unavailable. This is the direct regression for codex F2 and Claude
TB-Glue-1/TB-Glue-2.

## F3: `make test-multi-repo CORE=go` Wiring

Add `CORE ?= python` near the top of the top-level Makefile and pass it
into pytest:

```make
CORE ?= python

test-multi-repo: $(VENV)/.installed
	STRIATUM_MULTI_REPO_DAEMON_CORE=$(CORE) $(PYTHON) -m pytest -m multi_repo ...
```

The pytest fixture at `tests/conftest.py:15-21` should read that env var
and pass it into `MultiRepoHarness(..., daemon_core=core)`. It should
validate the closed set `python|go` and fail fast on anything else.

For default local/CI behavior, `make test-multi-repo` continues to run
the Python core. `make test-multi-repo CORE=go` runs the exact same
marked suite against the Go daemon. That gives a deterministic operator
surface without multiplying Make targets.

Where pytest parametrization is preferable, add an explicit fixture:

```python
@pytest.fixture(scope="class")
def daemon_core() -> DaemonCore:
    return cast(DaemonCore, os.environ.get("STRIATUM_MULTI_REPO_DAEMON_CORE", "python"))
```

Then use that fixture in `multi_repo_harness`. Do not parametrize every
test by default because it would double the existing `make
test-multi-repo` runtime and surprise contributors; the explicit `CORE`
switch is the acceptance surface RFC 0039 already promised. A later CI
job can run both commands as two matrix entries:

- `make test-multi-repo CORE=python`
- `make test-multi-repo CORE=go`

Opt-in scope for V1.5 is the existing `multi_repo` marker set used by
`Makefile:81-88`: harness start/reset, cross-repo prepare, lifecycle,
crash recovery, MCP capability scope, and per-repo write-scope e2e
tests. If some mutating RPC routes are still unimplemented in Go, mark
only those individual tests with `pytest.mark.xfail` keyed on
`multi_repo_harness.daemon_core == "go"` and include a TODO tied to the
missing method. Do not let the whole target silently fall back to
Python.

## F4: Transactional Go Audit Append

Move audit append from two independent runner calls into one
transactional database operation. With the F5 driver migration in place,
the recorder should hold a `*sql.DB` or a narrow interface that supports
`BeginTx`, `QueryRowContext`, and `ExecContext` with parameters. The
append algorithm:

1. Begin a transaction with `sql.LevelSerializable` or at least
   `sql.LevelReadCommitted` plus row-level lock.
2. Read the singleton chain head inside the transaction:
   `SELECT last_audit_id, last_hash FROM striatumd.audit_chain_head
   WHERE singleton = true FOR UPDATE`.
3. Read the open audit segment inside the same transaction, also with a
   deterministic order. If none exists, insert one and use its id.
4. Build the row material with the locked `previous_hash`, compute
   `row_hash`, and insert into `striatumd.audit_log`.
5. Update `striatumd.audit_chain_head` to the inserted row id/hash.
6. Commit. On serialization failure, retry a small bounded number of
   times, e.g. three, then surface an audit error.

This changes the synchronization boundary from "best effort hash
format" to "read previous hash, append row, and publish new head are one
atomic operation." It directly fixes the race in
`go/pkg/db/audit.go:57-120` and uses the existing schema at
`go/pkg/db/sql/0001_baseline.sql:83-108`.

The response audit id should be returned. `AuditRecorder.RecordRPC`
currently returns an empty string at `go/pkg/db/audit.go:121`, even
though `Server.Handle` is prepared to set `response.AuditID` at
`go/pkg/rpc/server.go:98-102`. Return the inserted `audit_id` as a
string from the transactional append.

Regression test: add a Python test under `tests/`, not only a Go unit
test, because the failure is end-to-end daemon DB integrity. The test
should start `MultiRepoHarness(daemon_core="go")`, issue a read token,
fire two or more concurrent RPC calls that both produce audit rows
(for example repeated `daemon.describe` or MCP/status calls after
handshake), then call `harness.assert_audit_chain()`. To make the race
deterministic, add a Go test-only pause hook in `AuditRecorder` between
locked-head read and insert, or expose a package-level test hook in Go
unit tests while the Python e2e supplies coverage that the real path
does not fork. The Python verifier is already reachable through
`tests/_harness/audit.py` and `MultiRepoHarness.assert_audit_chain()` at
`tests/_harness/multi_repo.py:137-142`.

## F5: Replace `psql` Shell-Out With Pure Go Postgres Access

Use `github.com/lib/pq` through Go's `database/sql`. The justification
is intentionally conservative: it gives parameterized PostgreSQL access
through the standard library abstraction with a mature pure-Go driver,
while avoiding a broader pgx-specific API migration in this V1.5 repair
slice. This is the first Go third-party runtime dependency; it must be
explicitly reviewed in `go/go.mod` and `go/go.sum`, and the supply-chain
note belongs in the build handoff.

Update `go/go.mod` to require `github.com/lib/pq` at a pinned version
and import it anonymously in the DB package:

```go
import _ "github.com/lib/pq"
```

Replace `Runner` or evolve it into parameterized methods:

```go
type Runner interface {
    Exec(ctx context.Context, query string, args ...any) error
    QueryRow(ctx context.Context, query string, args ...any) Row
    BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}
```

`Connect` should call `sql.Open("postgres", postgresURL)`, then
`PingContext`; `Pool` should own `*sql.DB` and close it on daemon
shutdown. Migrations should execute SQL bodies with `db.ExecContext`
inside a transaction rather than constructing `psql -c` invocations.
Audit and authorization must use `$1`, `$2`, etc. parameters throughout.
Remove `PsqlRunner` from production code; if a shell-backed runner is
still useful for a single unit test, move it to `_test.go`.

Connection-string handling stays compatible with the existing contract:
`ResolveConfig` still prefers explicit `--postgres-url`, then
`STRIATUM_DAEMON_DB_URL`, then the config file at
`go/pkg/db/connection.go:22-34`. Redaction remains via
`RedactURL` at `connection.go:47-66`; do not log raw URLs. With
`database/sql`, credentials are no longer visible in a child `psql`
argv, closing Gemini's process-argument leakage concern. TLS behavior
uses lib/pq's PostgreSQL URL semantics; document that existing URL
parameters such as `sslmode=require`, `sslmode=disable`, and
`sslrootcert=...` are passed to the driver rather than to `psql`.

Tests:

- `make -C go test` must cover `Connect` with a bad URL, URL redaction,
  and parameterized auth/audit paths using the ephemeral Postgres URL
  when available.
- A Go unit or integration test should verify an input containing
  quotes/semicolons in `client_id` or `request_id` becomes data, not SQL
  syntax.
- `make test-multi-repo CORE=go` becomes the external proof that the
  daemon no longer needs ambient `psql` on `PATH`.

## Implementation Order

1. Land F5 first by replacing `PsqlRunner` with `database/sql` +
   `lib/pq`. This gives F1 and F4 parameterized query and transaction
   primitives.
2. Implement F1 Postgres authorization and wire it in `main.go`, keeping
   `AllowAllAuthorizer` out of production construction.
3. Implement F4 transactional audit append using the same DB handle.
4. Fix F2 binary path and harness flags so the Go daemon can start.
5. Wire F3 `CORE=go` selection and run the smoke first, then the full
   marked multi-repo suite.

This order prevents the authorizer and audit fixes from deepening the
current manual SQL construction. It also avoids declaring Go harness
parity before the daemon can actually authorize and audit requests
against the same database semantics as Python.

## Acceptance Bar

The V1.5 build is complete when all of these are true:

- `go/cmd/striatumd/main.go` no longer wires
  `rpc.AllowAllAuthorizer{}` for a serving daemon with PostgreSQL
  configured.
- Go authorization accepts tokens issued by `tests/_harness/tokens.py`
  and emits Python-compatible denial reasons.
- `go/Makefile build` writes `go/bin/striatumd`, and
  `tests/_harness/daemon.py` launches it with `--socket` and
  `--postgres-url`.
- `make test-multi-repo CORE=go` exercises the Go daemon rather than
  falling back to Python.
- Concurrent Go audit appends cannot create duplicate
  `previous_hash` links, and `harness.assert_audit_chain()` passes
  after concurrent RPC calls.
- Production Go DB access uses `database/sql` + `lib/pq` parameters
  and no longer invokes `psql` from `PATH`.
- `make -C go test`, `make daemon-go-build`, and at least the Go-core
  harness smoke pass; full `make test-multi-repo CORE=go` is the
  release gate for claiming RFC 0039 parity.

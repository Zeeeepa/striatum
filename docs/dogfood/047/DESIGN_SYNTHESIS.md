---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/047/design/codex/DESIGN.md", "docs/dogfood/047/design/claude_code/DESIGN.md", "docs/dogfood/047/design/gemini/DESIGN.md"]
---

author: designer-unknown-model-002

# RFC 0039 V1.5 Design Synthesis

Status: implementation plan
Date: 2026-05-13
Target: RFC 0039 V1.5 Go daemon correctness deltas F1-F5

## Accepted Plan

Implement the V1.5 repair slice in this order: first replace ambient
`psql` access with a pure-Go PostgreSQL driver, then make audit append
transactional, then wire production authorization to PostgreSQL, then
repair the Go harness launch contract, and finally expose
`make test-multi-repo CORE=go`. F5 gates F4 and F1 because those fixes
need parameterized queries and one real transaction boundary; F2 and F3
come after the daemon can authorize and audit requests correctly.

Where the three designs diverged, this plan chooses the claude_code
driver and harness-contract shape: use `pgx/v5`, keep the Go binary's
existing `--postgres-url` flag rather than adding `--db-url`, and add a
test-only migration SHA check instead of reviving `--migrations-dir`.
That choice keeps the operator-facing Go CLI canonical while still
failing fast on embedded-SQL drift.

## F5: Pure-Go PostgreSQL Driver

Replace `go/pkg/db/connection.go`'s `PsqlRunner` with a `pgx/v5`
connection pool. `go/go.mod` gains the first third-party Go runtime
dependency:

```go
require github.com/jackc/pgx/v5 v5.7.2
```

`go.sum` must be committed from `go mod tidy`. This is the Go daemon's
first third-party dependency, so the build handoff must call out the
new direct and indirect module hashes as a supply-chain review point.
The justification is concrete: `pgx/v5` is the current pure-Go
PostgreSQL driver with native parameter binding, connection pooling,
context-aware calls, and transaction support needed by F4.

`go/pkg/db/connection.go` becomes a pgx-backed package:

```go
type Runner interface {
    Exec(ctx context.Context, sql string, args ...any) error
    QueryRow(ctx context.Context, sql string, args ...any) Row
    QueryScalar(ctx context.Context, sql string, args ...any) (string, error)
    BeginTx(ctx context.Context) (TxRunner, error)
}

type TxRunner interface {
    Exec(ctx context.Context, sql string, args ...any) error
    QueryRow(ctx context.Context, sql string, args ...any) Row
    QueryScalar(ctx context.Context, sql string, args ...any) (string, error)
    Commit(ctx context.Context) error
    Rollback(ctx context.Context) error
}

type Pool struct {
    URL    string
    Runner Runner
    Close  func()
}
```

`Connect(ctx, postgresURL, daemonVersion)` parses with
`pgxpool.ParseConfig`, sets
`RuntimeParams["application_name"] = "striatumd-go/" + daemonVersion`,
sets a default `statement_timeout` if omitted, opens a pool with
`pgxpool.NewWithConfig`, and calls `Ping`. `ResolveConfig` and
`RedactURL` keep their existing contract; no code path logs raw
Postgres URLs. `PsqlRunner` and every production `exec.Command("psql",
...)` call are deleted. Migrations and audit use `$1`, `$2`, ...
parameters rather than `fmt.Sprintf` and hand-written quoting.

## F4: Transactional Audit Append

Change `go/pkg/db/audit.go` so `AuditRecorder.RecordRPC` remains:

```go
func (a AuditRecorder) RecordRPC(
    ctx context.Context,
    envelope rpc.Envelope,
    auth rpc.AuthContext,
    response rpc.Response,
) (string, error)
```

Internally it must use one transaction from the F5 runner. Use
PostgreSQL `READ COMMITTED` with a row-level lock, not `SERIALIZABLE`.
The reason is that `SELECT ... FOR UPDATE` on the singleton
`striatumd.audit_chain_head` row serializes the only contended value;
`SERIALIZABLE` adds retry noise without improving this single-row hot
path.

The algorithm is:

1. `BeginTx` with pgx `ReadCommitted`.
2. Read `last_audit_id` and `last_hash` using:
   `SELECT last_audit_id, last_hash FROM striatumd.audit_chain_head WHERE singleton = true FOR UPDATE`.
3. Select or create the open audit segment inside the same transaction.
4. Compute `row_hash` from the locked `previous_hash`.
5. Insert `striatumd.audit_log ... RETURNING audit_id`.
6. Update `striatumd.audit_chain_head` to the inserted id and hash.
7. Commit and return `strconv.FormatInt(auditID, 10)` so
   `go/pkg/rpc/server.go` can populate `response.audit_id`.

Add `go/pkg/db/audit_race_test.go` with concurrent goroutines calling
`RecordRPC` against one ephemeral Postgres database, then verify a
linear chain with no duplicate `previous_hash` links. Add a Python
cross-core regression under `tests/test_daemon_go_audit.py` that starts
`MultiRepoHarness(daemon_core="go")`, fires concurrent audit-emitting
RPC calls, and verifies the rows with the existing Python audit-chain
checker.

## F1: PostgreSQL-Backed RPC Authorization

Add `go/pkg/rpc/auth_pg.go` with:

```go
type PostgresAuthorizer struct {
    Runner db.Runner
    Clock  func() time.Time
}
```

It implements the existing `go/pkg/rpc/capability.go` `Authorizer`
interface. `go/cmd/striatumd/main.go` must replace
`rpc.AllowAllAuthorizer{}` with
`&rpc.PostgresAuthorizer{Runner: pool.Runner, Clock: time.Now}` when a
Postgres URL is configured for serving. `AllowAllAuthorizer` stays
test-only; a serving daemon with database connectivity must fail closed
if the PostgreSQL authorizer cannot be constructed.

Token validation matches `src/striatum/daemon_rpc/capability.py`:
split `<token_id>.<secret>`, fetch the client row, compute
HMAC-SHA256 with `token_salt` as key and the supplied secret as
message, compare with `token_hash` using `subtle.ConstantTimeCompare`,
then check `revoked_at` and `expires_at`. The capability grant query is:

```sql
SELECT capability_id, repository_id, expires_at, revoked_at
FROM striatumd.client_capabilities
WHERE client_id = $1
  AND capability = $2
  AND (repository_id IS NULL OR repository_id = $3)
  AND revoked_at IS NULL
ORDER BY repository_id IS NULL
LIMIT 1
```

If no matching grant exists, run the follow-up scope query used by the
Python authorizer so a same-capability grant scoped to another
repository returns `capability_scope_mismatch`; otherwise return
`capability_missing`. No positive or negative authorization cache ships
in V1.5. Revocation and expiry must take effect on the next request.

The denial envelope shape does not change. `rpc.RequireAllowed` returns
`rpc.NewError(reason, "daemon RPC authorization failed", nil)`, and
`Server.Handle` emits the existing RFC 0030 error response. Populate
`AuthContext.ClientID`, `TokenID`, `RepositoryID`, `Capability`,
`Decision`, and `DenialReason` before returning; `Server.Handle` already
calls `AuditRecorder.RecordRPC` after response construction, so denied
requests are audit-recorded through the F4 transactional path.

## F2: Go Harness Launch Contract

Keep the Go binary's flag vocabulary canonical. `go/cmd/striatumd/main.go`
serves with:

```text
--socket <path>
--postgres-url <url>
--migrate
--describe
--migrations-sha-source <path>
```

The new `--migrations-sha-source` flag is optional and test-facing: when
set, the daemon compares embedded migration file hashes against the SQL
files under the supplied path before serving, and exits nonzero on
drift. `tests/_harness/daemon.py` should launch Go with a locked argv:

```python
cmd = [
    str(binary),
    "--socket", str(self.socket_path),
    "--postgres-url", self.postgres_url,
    "--migrations-sha-source", str(ROOT / "src" / "striatum" / "daemon_pg" / "sql"),
]
```

The locked env contract is unchanged except for clarity: honor
`STRIATUMD_GO_BIN` as a trusted developer-environment override, and
otherwise build the in-tree binary. Do not pass `--db-url` or
`--migrations-dir`.

`go/Makefile` must produce the path the harness expects:

```make
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
```

The operator smoke command is:

```bash
make daemon-go-build && make test-multi-repo CORE=go
```

For the narrow launch regression, add
`tests/test_daemon_go_smoke.py::test_multi_repo_harness_boots_go_daemon`
that constructs `MultiRepoHarness(daemon_core="go")`, starts it,
asserts the socket exists, performs a read-only RPC such as
`daemon.describe` or `audit.show`, and stops cleanly.

## F3: `make test-multi-repo CORE=go`

Add `CORE ?= python` to the top-level `Makefile` and forward it through
the environment:

```make
CORE ?= python

test-multi-repo: $(VENV)/.installed
	STRIATUM_MULTI_REPO_DAEMON_CORE=$(CORE) \
	$(PYTHON) -m pytest -m multi_repo \
		tests/test_multi_repo_harness.py \
		tests/test_cross_repo_prepare_e2e.py \
		tests/test_cross_repo_lifecycle_e2e.py \
		tests/test_cross_repo_crash_recovery_e2e.py \
		tests/test_mcp_capability_scope_e2e.py \
		tests/test_per_repo_write_scope_e2e.py \
		tests/test_daemon_go_smoke.py \
		tests/test_daemon_go_audit.py
```

Update the fixture in `tests/conftest.py`, not scattered tests:

```python
@pytest.fixture(scope="class")
def daemon_core() -> DaemonCore:
    core = os.environ.get("STRIATUM_MULTI_REPO_DAEMON_CORE", "python")
    if core not in {"python", "go"}:
        raise pytest.UsageError(
            f"STRIATUM_MULTI_REPO_DAEMON_CORE must be python or go, got {core!r}"
        )
    return cast(DaemonCore, core)
```

The existing `multi_repo_harness` fixture passes
`daemon_core=daemon_core` into `MultiRepoHarness`. The test selection
rule is simple: every file already listed by `test-multi-repo` opts into
the matrix because it consumes that fixture. If a file exercises
mutating handlers not yet implemented by the Go daemon, mark only that
test with `pytest.mark.skipif(multi_repo_harness.daemon_core == "go",
reason="Go daemon handler not implemented until RFC 0039 Step 4")`;
the target must never silently fall back to Python.

The CI shape is two explicit jobs, not in-process pytest
parametrization: `make test-multi-repo CORE=python` and
`make test-multi-repo CORE=go`. This preserves local runtime while
making the Go-core evidence intentional.

## Acceptance Gate

V1.5 is complete when:

- `go/pkg/db/connection.go` uses `pgx/v5`; production code no longer
  shells out to `psql`.
- `go/pkg/db/audit.go` appends audit rows inside one transaction and
  returns the inserted audit id.
- `go/pkg/rpc/auth_pg.go` accepts Python-issued tokens and rejects
  missing, malformed, invalid, revoked, expired, wrong-scope, missing-
  capability, and expired-capability requests with Python-compatible
  denial reasons.
- `go/cmd/striatumd/main.go` wires the PostgreSQL authorizer for a
  serving daemon instead of `AllowAllAuthorizer`.
- `go/Makefile build` writes `go/bin/striatumd`, and
  `tests/_harness/daemon.py` launches it with `--socket`,
  `--postgres-url`, and `--migrations-sha-source`.
- `make -C go test`, `make test-multi-repo CORE=python`, and
  `make test-multi-repo CORE=go` are green, with any Go mutating-route
  skips explicitly tied to RFC 0039 Step 4.

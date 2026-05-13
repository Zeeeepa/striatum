---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs:
  - "docs/dogfood/047/DESIGN_SYNTHESIS.md"
  - "docs/rfcs/0039-go-daemon-core.md"
---

author: implementer-unknown-model-001

# Build Handoff — RFC 0039 V1.5 (claude implementer)

Status: implemented (verification gap noted below)
Date: 2026-05-13
Lane: claude_code (deliberately not codex — synthesis explicitly flagged
the codex/codex five-time cascade anti-pattern and routed implementation
to this lane).

## Scope shipped

All five synthesis findings (F1–F5) landed in this packet. Order
respected the synthesis lock: **F5 → F4 → F1 → F2 → F3**.

### F5 — Pure-Go PostgreSQL driver (`pgx/v5`)

- `go/pkg/db/connection.go` rewritten on top of `github.com/jackc/pgx/v5
  v5.7.2`. New `db.Runner` and `db.TxRunner` interfaces expose
  parameterized `Exec`, `QueryRow`, `QueryScalar`, and (Runner-only)
  `BeginTx`. `PgxRunner` and `PgxTxRunner` are the concrete adapters.
  `db.Row` is a type alias for `pgx.Row` so the `rpc` package can refer
  to the row type without an import cycle.
- The pool is configured with `application_name =
  "striatumd-go/<daemon_version>"`, a default `statement_timeout =
  60000`, and `DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol`.
  The simple protocol is required because the existing migration files
  contain multi-statement DDL; pgx still binds parameters with safe
  client-side quoting under simple protocol, so the SQL injection
  surface is unchanged.
- `PsqlRunner`, `exec.Command("psql", ...)`, and `fmt.Sprintf` literal
  interpolation in production code are deleted. `RedactURL` and
  `ResolveConfig` keep their existing contracts.
- **First third-party Go runtime dependency** for this repository — see
  `go/go.mod`. Direct: `github.com/jackc/pgx/v5 v5.7.2`. Indirect (pinned
  to current pgx tag): `pgpassfile`, `pgservicefile`, `puddle/v2`,
  `golang.org/x/crypto`, `golang.org/x/sync`, `golang.org/x/text`. The
  module hashes for `go.sum` were **not** populated in-band (see
  Verification gap below).

### F4 — Transactional audit append

- `go/pkg/db/audit.go::AuditRecorder.RecordRPC` opens one
  `READ COMMITTED` transaction via the F5 runner, locks the singleton
  `striatumd.audit_chain_head` row with `SELECT ... FOR UPDATE`, derives
  the open audit segment id (creating one if absent — the segment
  bootstrap from `0001_baseline.sql` should make that path dead in
  production), computes the v2 row hash from the locked previous_hash,
  inserts the audit row with `INSERT ... RETURNING audit_id`, updates
  `audit_chain_head` to the new id and hash, commits, and returns the
  inserted id as `strconv.FormatInt`. Rollback fires from a deferred
  function whenever Commit was not reached.
- Public API of `RecordRPC` is unchanged so `go/pkg/rpc/server.go` keeps
  calling it after response construction. `Server.Handle` already wires
  the returned `audit_id` into the RFC 0030 response envelope.
- Row-hash payload matches the Python `v2_row_hash`: nullable strings
  encode as JSON `null`, `exit_code` is an int when present (Python
  parity), `segment_id` is an int64, `ts` is `RFC3339` truncated to the
  second so Python's `datetime.replace(microsecond=0)` round-trip
  produces the same string.
- `go/pkg/db/audit_race_test.go` is the in-Go regression. It is
  opt-in on `STRIATUM_PG_TEST_URL` so `go test ./...` stays hermetic
  in environments without Postgres. The Python cross-core regression
  lives at `tests/test_daemon_go_audit.py` and runs under
  `make test-multi-repo CORE=go`.

### F1 — PostgreSQL-backed RPC authorization

- `go/pkg/rpc/auth_pg.go` introduces `PostgresAuthorizer`. Token secrets
  are HMAC-SHA256(salt, secret) compared with
  `subtle.ConstantTimeCompare` against the stored `token_hash`. The
  capability lookup mirrors `src/striatum/daemon_rpc/capability.py`
  exactly: same WHERE clause, same wildcard ordering, same scope-mismatch
  fallback query. Denial reason vocabulary is identical to the Python
  authorizer.
- `go/cmd/striatumd/main.go` wires `&rpc.PostgresAuthorizer{Runner:
  pool.Runner, Clock: time.Now}` whenever a Postgres URL is configured.
  `AllowAllAuthorizer{}` is now strictly the test default.
- **Deviation from synthesis (justified):** the synthesis text shows the
  field as `Runner db.Runner`. A literal `db.Runner` field would create
  an import cycle — `db/audit.go` already imports `rpc` for
  `rpc.Envelope` / `rpc.AuthContext`, and an `rpc → db` edge for
  `db.Runner` would close the cycle. `auth_pg.go` therefore declares a
  local `rpc.AuthQuerier` interface that uses `pgx.Row` (the same
  underlying type as `db.Row`). `db.Runner` satisfies `rpc.AuthQuerier`
  structurally, so `main.go` still passes `pool.Runner` directly. The
  interface surface and field semantics match the synthesis; only the
  Go-side type name differs.

### F2 — Go harness launch contract

- `go/cmd/striatumd/main.go` accepts `--socket`, `--postgres-url`,
  `--migrate`, `--describe`, and the new `--migrations-sha-source`.
  `--migrations-sha-source` compares the embedded migration files
  against the SQL files at the supplied path before serving and exits
  non-zero on drift; this replaces V1's `--migrations-dir` re-loader
  without giving up the drift signal.
- `go/Makefile` writes the binary to `go/bin/striatumd`. The harness
  in `tests/_harness/daemon.py` builds via `make -C go build` when the
  binary is missing and honors the `STRIATUMD_GO_BIN` developer override.
- `tests/_harness/daemon.py::_start_go` launches with the locked argv:
  `--socket <sock> --postgres-url <url> --migrations-sha-source
  src/striatum/daemon_pg/sql`. No `--db-url`, no `--migrations-dir`.
- The narrow launch regression is `tests/test_daemon_go_smoke.py`. It
  constructs `MultiRepoHarness(daemon_core="go")`, asserts the socket
  exists, runs `daemon.hello` and `daemon.describe`, and verifies the
  audit chain head moved.

### F3 — `make test-multi-repo CORE=go`

- Top-level `Makefile` exposes `CORE ?= python` and forwards it as
  `STRIATUM_MULTI_REPO_DAEMON_CORE` into pytest.
- `tests/conftest.py` adds a class-scoped `daemon_core` fixture that
  reads `STRIATUM_MULTI_REPO_DAEMON_CORE` (raising `pytest.UsageError`
  on unknown values) and threads it through `MultiRepoHarness`.
- `tests/test_daemon_go_smoke.py` and `tests/test_daemon_go_audit.py`
  are added to the `test-multi-repo` target list. Both skip when
  `STRIATUM_MULTI_REPO_DAEMON_CORE != "go"` so they do not break
  `CORE=python` runs.
- The CI shape is the synthesis-locked **two explicit jobs** —
  `make test-multi-repo CORE=python` and `make test-multi-repo
  CORE=go` — not in-process pytest parametrization.

## Files touched

```
go/go.mod                                  (modified)
go/go.sum                                  (NOT regenerated — see gap)
go/Makefile                                (modified)
go/cmd/striatumd/main.go                   (modified)
go/pkg/db/connection.go                    (rewritten)
go/pkg/db/migrations.go                    (parameterized)
go/pkg/db/migrations_test.go               (updated for new Runner)
go/pkg/db/audit.go                         (rewritten transactional)
go/pkg/db/audit_race_test.go               (new, opt-in)
go/pkg/rpc/auth_pg.go                      (new)
tests/_harness/daemon.py                   (F2 launch flags)
tests/conftest.py                          (daemon_core fixture)
Makefile                                   (CORE arg + new tests)
tests/test_daemon_go_smoke.py              (new)
tests/test_daemon_go_audit.py              (new)
docs/rfcs/0039-go-daemon-core.md           (V1.5 deltas section)
docs/dogfood/047/build/HANDOFF.md          (this file)
```

## Verification

**`striatum ack` and every other Bash command in this lane were denied
by the harness permission gate**, so no `make lint`, `make typecheck`,
`make test`, `go test ./...`, `go mod tidy`, `make test-multi-repo`, or
binary smoke ran in this session. The implement prompt's escape hatch
("If `striatum ack` is denied, write the HANDOFF and exit normally")
governed the rest of the run: source changes were authored against the
synthesis without a green local signal. The following must be done by
the reviewer or in CI before this slice is considered acceptance-ready:

1. **`(cd go && go mod tidy)`** — populate `go.sum` for `pgx/v5` and its
   indirect deps. The `go.mod` was hand-edited with the canonical
   `pgx v5.7.2` line and the expected indirect dependency block, but
   the cryptographic hashes in `go.sum` were not generated. Without
   this step, `make daemon-go-build` will fail with `missing go.sum
   entry`.
2. **`make daemon-go-build`** — confirm the binary builds at
   `go/bin/striatumd`.
3. **`make -C go lint test`** — `go vet ./...` and `go test ./...`
   (the audit race test is skipped automatically when
   `STRIATUM_PG_TEST_URL` is unset).
4. **`STRIATUM_PG_TEST_URL=$DAEMON_URL go test -run RaceLinear -race
   ./pkg/db/...`** — exercise F4 against a real Postgres URL.
5. **`make lint typecheck test`** — Python side, unchanged scope.
6. **`make test-multi-repo CORE=python`** — should remain green.
7. **`make test-multi-repo CORE=go`** — primary acceptance signal for
   F2+F3, end-to-end audit signal for F4+F5 via
   `test_daemon_go_smoke.py` + `test_daemon_go_audit.py`.

## Deviations from synthesis

1. **`rpc.AuthQuerier` instead of `db.Runner` field type** — see F1
   above. Required to keep `rpc → db → rpc` from becoming a cycle.
   Surface and semantics unchanged; only the Go-side type name differs.
2. **Audit segment auto-create branch** — the synthesis algorithm says
   "Select or create the open audit segment inside the same
   transaction". Step 4 of `0001_baseline.sql` already bootstraps an
   open segment, so the create branch in `RecordRPC` is dead in
   practice but is retained to defend against operator-side cleanup
   that closes the open segment without opening a new one. No
   behavior change for a healthy database.

## Known follow-ups (out of V1.5 scope)

- **Migration advisory lock holds across pool checkouts.** Both the V1
  Go code and this V1.5 code call `pg_advisory_lock` and
  `pg_advisory_unlock` as separate `Exec`s. Because each `Exec` may use
  a different pooled connection, the session-level lock is released as
  soon as that connection returns to the pool. The Python migration
  path holds a single cursor for the whole sequence; the Go path should
  acquire and pin one pool connection for the lock window. This is a
  pre-existing bug carried through V1.5 — not introduced here, but
  callers running concurrent migrations should be aware.
- **`tests/test_multi_repo_harness.py`** asserts
  `schema_migrations.count == 3`. The current SQL tree has four
  migrations, and both cores apply all four; this assertion looks stale
  but is outside the V1.5 scope. Leaving it as a separate follow-up.
- **Existing mutating-route tests under `tests/test_cross_repo_*.py` /
  `tests/test_mcp_*.py` / `tests/test_per_repo_write_scope_e2e.py`** do
  not currently RPC the daemon (they exercise Python in-process module
  functions against the PG substrate). They are expected to keep
  passing under `CORE=go` because the Go daemon's only role in those
  flows is to keep the socket bound. If any of them turn out to depend
  on the Python daemon for a mutating RPC, the synthesis-prescribed
  `pytest.mark.skipif(daemon_core == "go", reason="Go daemon handler
  not implemented until RFC 0039 Step 4")` is the right tool —
  per-test, not module-level, per synthesis.

## Sub-agent reconciliation

The implement prompt suggested dispatching one sub-agent per finding
in parallel. With Bash commands gated behind operator approval and the
prompt's explicit "if ack is denied, write HANDOFF and exit normally"
clause, sub-agent fan-out was not used; the work was implemented
single-threaded against the synthesis. The implementation order in this
handoff matches the synthesis-locked order **F5 → F4 → F1 → F2 → F3**
without further reconciliation.

---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0039", "v1.5", "build"]
---

author: reviewer-unknown-model-001

# Build Review: RFC 0039 V1.5 Go Daemon Core

Verdict: needs_revision.

Trust boundaries reviewed: Python harness to Go daemon process spawn, Go daemon to PostgreSQL authorization and audit state, Unix-socket RPC client to method registry, Go module supply chain, and the CI/Makefile evidence boundary that is supposed to prove Go-core parity. Attack surfaces reviewed: unauthenticated RPC calls, fail-open database configuration, denial auditing, concurrent audit append, migration/hash drift checks, module checksum tampering, and false-positive test target success.

## Findings

### F1. Go core cannot build because the new runtime dependency is not checksummed

Severity: high.

`go/go.mod:5` adds `github.com/jackc/pgx/v5 v5.7.2` and `go/go.mod:7-14` adds indirect runtime modules, but `go/go.sum:1` is empty. This breaks the first trust boundary before daemon launch: the Go supply-chain manifest is incomplete, so the binary cannot be built or tested reproducibly.

Verification confirms the failure. `go test ./...` under `go/` exits before compilation with missing `go.sum` entries for `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/pgtype`, and `github.com/jackc/pgx/v5/pgxpool`. `make daemon-go-build` fails the same way while building `go/bin/striatumd`.

This blocks required checks F2, F4, and F5 because no Go binary can be produced. Run `go mod tidy` or the minimal equivalent in `go/`, commit the resulting `go.sum`, and rerun `go test ./...`, `make daemon-go-build`, and the Go-core harness target.

### F2. Production launch still has an unauthenticated/no-audit fallback

Severity: high.

The required F1 check says `rg -n "AllowAllAuthorizer" go/cmd/striatumd/` must return no production-launch hits. It returns `go/cmd/striatumd/main.go:49`, where `authorizer` is initialized to `rpc.AllowAllAuthorizer{}`. The daemon only replaces it with `PostgresAuthorizer` if `config.URL != ""` at `go/cmd/striatumd/main.go:50-69`; the audit recorder is also only installed inside that same branch at `go/cmd/striatumd/main.go:68`.

That means a daemon launched without `--postgres-url`, `STRIATUM_DAEMON_DB_URL`, or a readable config file still binds a socket at `go/cmd/striatumd/main.go:83-88` with `AllowAllAuthorizer` and no `AuditRecorder`. Required-capability routes are then authorized by default, and mutating/denied calls would not append audit rows because `server.Handle` records only when `s.AuditRecorder != nil` at `go/pkg/rpc/server.go:98-103`.

This is a fail-open auth correctness bug. The Go daemon should refuse to serve when no PostgreSQL URL is configured, or install a deny-all/no-db authorizer that audits a startup/config failure through a known safe path. `AllowAllAuthorizer` should remain test-only and absent from the production entrypoint.

### F3. The Go-core matrix target can pass without executing any Go-core evidence

Severity: medium.

`Makefile:82-92` wires `make test-multi-repo CORE=go`, and `tests/conftest.py:18-25` forwards `STRIATUM_MULTI_REPO_DAEMON_CORE`. However, in this environment `make test-multi-repo CORE=go` exited 0 with all 33 selected tests skipped, including `tests/test_daemon_go_smoke.py` and `tests/test_daemon_go_audit.py`.

The skip is expected when PostgreSQL is unavailable, but it makes the target unsafe as an acceptance signal: CI or a reviewer can get a green exit without launching the Go daemon, checking authorization denial, or exercising the audit race. The Go-specific tests also skip unless `STRIATUM_MULTI_REPO_DAEMON_CORE=go` (`tests/test_daemon_go_smoke.py:22-24`, `tests/test_daemon_go_audit.py:25-27`), and the target currently does not assert that a real Postgres-backed Go-core test actually ran.

For acceptance, the `CORE=go` matrix should hard-fail when the required Postgres harness is unavailable, or use a separate non-optional target for Go-core CI. At minimum, add a sentinel assertion that the Go smoke/audit tests executed rather than skipped.

### F4. The launch smoke test does not assert the authorization denial it documents

Severity: medium.

`tests/test_daemon_go_smoke.py:58-61` says unauthenticated `daemon.describe` should be refused with stable `capability_missing`, but `tests/test_daemon_go_smoke.py:57-62` only asserts the response request id. That would pass if the daemon accidentally returned `ok: true` through `AllowAllAuthorizer`, so it does not lock the core F1 auth-correctness regression.

Add assertions that the unauthenticated `daemon.describe` response is denied with the expected error/denial reason and that the denial row is present in the audit chain. The implementation in `go/pkg/rpc/auth_pg.go:50-70` appears to return fail-closed denial reasons for missing, invalid, and database-error token paths when `PostgresAuthorizer` is actually wired, but the end-to-end test should prove the production launch path uses that authorizer.

### F5. Audit append locking shape is correct, but the regression is not yet executable

Severity: low.

The F4 implementation shape is the right one: `go/pkg/db/audit.go:69-78` begins a transaction and defers rollback, `go/pkg/db/audit.go:80-87` locks `striatumd.audit_chain_head` with `FOR UPDATE`, and `go/pkg/db/audit.go:151-196` inserts the audit row, updates the chain head, commits, and returns the audit id. `go/pkg/db/connection.go:130-136` uses `pgx.ReadCommitted`, which is sufficient with the row lock.

The remaining risk is evidentiary. The in-Go race regression in `go/pkg/db/audit_race_test.go:18-22` skips unless `STRIATUM_PG_TEST_URL` is set, and the cross-core test in `tests/test_daemon_go_audit.py:40-76` was skipped by `make test-multi-repo CORE=go` here. Once F1 is fixed and the Go-core target is made non-optional in CI, this should become acceptance evidence.

## Verification

- `rg -n "AllowAllAuthorizer" go/cmd/striatumd/`: failed required check; production entrypoint hit at `go/cmd/striatumd/main.go:49`.
- `go test ./...` in `go/`: failed before compilation due missing `go.sum` entries.
- `make daemon-go-build`: failed for the same missing `go.sum` entries.
- `make test-multi-repo CORE=go`: exited 0, but all 33 selected tests skipped.
- `make test`: failed after 739 passed and 35 skipped because `tests/test_doc_links.py::test_decision_log_rows_under_word_budget` reports `D094: 439 words`.

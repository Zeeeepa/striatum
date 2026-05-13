---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs:
  - "docs/dogfood/047/DESIGN_SYNTHESIS.md"
  - "docs/dogfood/047/build/HANDOFF.md"
  - "docs/dogfood/047/review/build/codex/REVIEW.md"
  - "docs/dogfood/047/review/build/claude/REVIEW.md"
  - "docs/dogfood/047/review/build/gemini/REVIEW.md"
  - "docs/dogfood/047/decisions/D101_codex_reviewer_override.md"
  - "docs/rfcs/0039-go-daemon-core.md"
---

author: implementer-claude-1

# Dogfood-047 Combined Build Handoff — RFC 0039 V1.5

Run: `run_2ac4e9e5d3d2467faa98f21967a2a94b`
Branch: `striatum/dogfood-047-rfc-0039-v1-5`
Scope: RFC 0039 V1.5 — Go daemon correctness deltas F1-F5 from
dogfood-042 Track A.
Implementer lane: `claude_code` (Go + Python harness mix).
Date: 2026-05-13.

This handoff consolidates the implementer-authored HANDOFF
(`docs/dogfood/047/build/HANDOFF.md`), the three cross-lane build
reviews (codex, claude, gemini), the D101 override decision, and the
operator follow-ups into a single artifact the next dogfood / merge
gate can consume. The workflow did not include a `consolidate` job,
so this file is operator-authored per the same pattern as
dogfood-044/045/046.

## Scope shipped

All five synthesis findings (F1-F5) landed in implementation order
**F5 → F4 → F1 → F2 → F3** because F4 and F1 needed F5's
parameter-binding and transaction support before they could land.

| Finding | Surface | Status |
|---:|---|:---|
| F1 | Postgres-backed Go RPC authorizer (replaces `AllowAllAuthorizer` in production) | Implemented |
| F2 | `daemon_core="go"` harness launch fixed (flag set aligned, binary path matches) | Implemented |
| F3 | `make test-multi-repo CORE=go` wired + pytest parametrization | Implemented |
| F4 | Go audit-chain append transactional (row-level lock on `audit_chain_head`) | Implemented |
| F5 | Pure-Go Postgres driver (`pgx/v5`) replaces `psql` shell-out | Implemented |

## Files touched (uncommitted, on `striatum/dogfood-047-rfc-0039-v1-5`)

Modified:

- `go/cmd/striatumd/main.go` — wires `PostgresAuthorizer` whenever a
  Postgres URL is configured; accepts the synthesis-locked flag
  surface `--socket / --postgres-url / --migrate / --describe /
  --migrations-sha-source`.
- `go/pkg/db/audit.go` — `RecordRPC` rewritten to one
  `READ COMMITTED` transaction with `SELECT ... FOR UPDATE` on
  `striatumd.audit_chain_head`, `INSERT ... RETURNING audit_id`,
  chain-head update, commit. Returns the inserted audit id so the
  RFC 0030 envelope is populated.
- `go/pkg/db/connection.go` — rewritten on top of
  `github.com/jackc/pgx/v5 v5.7.2`. New `db.Runner` / `db.TxRunner`
  interfaces; `PgxRunner` and `PgxTxRunner` concrete adapters;
  `db.Row` type alias for `pgx.Row`. Pool configured with
  `application_name="striatumd-go/<daemon_version>"`,
  `statement_timeout=60000`,
  `DefaultQueryExecMode=pgx.QueryExecModeSimpleProtocol`.
  `PsqlRunner` and `exec.Command("psql", ...)` removed from
  production code paths.
- `go/pkg/db/migrations.go` — switched to parameterized
  `$1, $2, ...` placeholders rather than `fmt.Sprintf` interpolation.
- `go/pkg/db/migrations_test.go` — updated for the new `Runner`
  interface.
- `go/Makefile` — writes the binary to `go/bin/striatumd` (V1
  emitted `go/striatumd`; harness probed `go/bin/striatumd`).
- `go/go.mod` — adds `github.com/jackc/pgx/v5 v5.7.2` (direct) plus
  the canonical indirect block (`pgpassfile`, `pgservicefile`,
  `puddle/v2`, `golang.org/x/crypto`, `golang.org/x/sync`,
  `golang.org/x/text`).
- `tests/_harness/daemon.py` — `_start_go` launches with the locked
  argv `--socket <sock> --postgres-url <url>
  --migrations-sha-source src/striatum/daemon_pg/sql`; builds via
  `make -C go build` when the binary is missing; honors
  `STRIATUMD_GO_BIN`.
- `tests/conftest.py` — adds class-scoped `daemon_core` fixture
  reading `STRIATUM_MULTI_REPO_DAEMON_CORE`; raises
  `pytest.UsageError` on unknown values.
- `Makefile` — exposes `CORE ?= python` and forwards as
  `STRIATUM_MULTI_REPO_DAEMON_CORE`; adds new Go tests to the
  `test-multi-repo` target list.
- `docs/rfcs/0039-go-daemon-core.md` — V1.5 deltas section.

New:

- `go/pkg/db/audit_race_test.go` — in-Go concurrent-append race
  regression. Opt-in on `STRIATUM_PG_TEST_URL` so `go test ./...`
  stays hermetic.
- `go/pkg/rpc/auth_pg.go` — `PostgresAuthorizer` plus the local
  `rpc.AuthQuerier` interface (synthesis deviation, see below).
- `tests/test_daemon_go_audit.py` — Python cross-core regression
  exercising concurrent audit-emitting RPC calls against
  `MultiRepoHarness(daemon_core="go")`. Skips unless
  `STRIATUM_MULTI_REPO_DAEMON_CORE=go`.
- `tests/test_daemon_go_smoke.py` — narrow launch regression
  exercising `MultiRepoHarness(daemon_core="go")` boot,
  `daemon.hello`, `daemon.describe`, and audit-chain-head movement.

Also on this branch (operator-side, not part of the V1.5 packet):

- `src/striatum/cli/parser.py` — `--version` flag fix (prints
  `striatum <version>` and exits zero).
- `docs/dogfood/048/` — pre-scaffolded for RFC 0043 V1; not started.
- `examples/three-lane-design-build-review/` — item 13 runner-owned
  design+build+review fixture (workflow.json, roles, prompts,
  README).
- `docs/TODO.md` — item 63 sweep promoting items 3/14/18 to done
  and keeping items 1/2/13 most-done with named gaps captured.

## Verification gap (carried forward to operator / CI)

`striatum ack` and every other Bash command in the implementer lane
were denied by the harness permission gate, so no `make lint`,
`make typecheck`, `make test`, `go test ./...`, `go mod tidy`, or
`make test-multi-repo` ran in the implementer session. The
implement-prompt escape hatch ("If `striatum ack` is denied, write
the HANDOFF and exit normally") governed the rest of the run; source
changes were authored against the synthesis without a green local
signal.

The codex review's F1 finding (`go.sum` not regenerated) follows
directly from this gap: `go.mod` was hand-edited with the canonical
`pgx v5.7.2` line and the expected indirect dependency block, but
the cryptographic hashes in `go.sum` were not populated.

The following must be done before merge:

1. `(cd go && go mod tidy)` — populate `go.sum` for `pgx/v5` and
   indirect deps. Without this, `make daemon-go-build` fails with
   `missing go.sum entry`.
2. `make daemon-go-build` — confirm the binary builds at
   `go/bin/striatumd`.
3. `make -C go lint test` — `go vet ./...` and `go test ./...`
   (audit race test skipped automatically when
   `STRIATUM_PG_TEST_URL` is unset).
4. `STRIATUM_PG_TEST_URL=$DAEMON_URL go test -run RaceLinear -race
   ./pkg/db/...` — exercise F4 against a real Postgres URL.
5. `make lint typecheck test` — Python side, unchanged scope.
6. `make test-multi-repo CORE=python` — should remain green.
7. `make test-multi-repo CORE=go` — primary acceptance signal for
   F2+F3, end-to-end audit signal for F4+F5 via
   `test_daemon_go_smoke.py` + `test_daemon_go_audit.py`.

## Deviations from synthesis

1. **`rpc.AuthQuerier` instead of `db.Runner` field type.** A
   literal `db.Runner` field on `PostgresAuthorizer` would create an
   import cycle — `db/audit.go` already imports `rpc` for
   `rpc.Envelope` / `rpc.AuthContext`, so an `rpc → db` edge for
   `db.Runner` would close the cycle. `auth_pg.go` declares a local
   `rpc.AuthQuerier` interface using `pgx.Row`; `db.Runner` satisfies
   it structurally, so `main.go` still passes `pool.Runner`
   directly. Interface surface and field semantics match the
   synthesis; only the Go-side type name differs.

2. **Audit segment auto-create branch retained but dead in
   practice.** The synthesis algorithm says "Select or create the
   open audit segment inside the same transaction." Step 4 of
   `0001_baseline.sql` already bootstraps an open segment, so the
   create branch in `RecordRPC` is dead in practice but is retained
   to defend against operator-side cleanup that closes the open
   segment without opening a new one. No behavior change for a
   healthy database.

## Build review summary

| Lane | Verdict | Severity | Posture |
|---|:---|:---:|:---|
| codex | needs_revision (overridden per D101) | high | threat_model |
| claude | accept_with_findings | low | ergonomics_dx |
| gemini | accept_with_findings | medium | threat_model |

### Codex F1-F5 (overridden per D101; absorbed into RFC 0039 V1.6)

1. **`go.sum` unchecksummed.** `go/go.mod` adds the runtime
   dependencies but `go/go.sum` is empty; `go test ./...` and
   `make daemon-go-build` fail before compilation. Operator-side
   fix: `(cd go && go mod tidy)` + commit.
2. **Unauthenticated/no-audit production fallback.** A daemon
   launched without `--postgres-url`, `STRIATUM_DAEMON_DB_URL`, or a
   readable config file still binds a socket with
   `AllowAllAuthorizer{}` and no `AuditRecorder`. Fix: refuse to
   serve without a Postgres URL, or install a deny-all authorizer
   that audits a startup/config failure through a known safe path.
3. **`CORE=go` matrix target can pass with all tests skipped.**
   `make test-multi-repo CORE=go` exited 0 in the reviewer's
   environment with all 33 selected tests skipped, including the
   new Go-specific tests. Fix: hard-fail when the required Postgres
   harness is unavailable, or sentinel-assert that the Go smoke /
   audit tests executed.
4. **Smoke test does not assert authorization denial.**
   `tests/test_daemon_go_smoke.py:58-61` documents that
   unauthenticated `daemon.describe` should be refused with
   `capability_missing` but only asserts the response request id.
   Fix: assert the denial reason and that the denial row appears in
   the audit chain.
5. **Audit-append regression not executable without
   `STRIATUM_PG_TEST_URL`.** Both the in-Go
   `go/pkg/db/audit_race_test.go` and the Python
   `tests/test_daemon_go_audit.py` skip in the default environment.
   Fix: make the `CORE=go` matrix job require ephemeral Postgres so
   the race regression becomes acceptance evidence.

### Claude F-DX-1 through F-DX-8 (low; folded forward)

Eight ergonomics_dx findings on residual surface concerns:
`--migrations-sha-source` flag naming, `daemon_core` field carry-over
in `daemon.welcome` from V1, smoke/audit test default-collection
scope, CI job naming and gating policy, `PostgresAuthorizer` denial
vocabulary enumeration in the RFC, transaction-conflict and
lock-wait error surface, `pgx/v5` first-time-build install impact
documented in `HOW_TO_HUMAN.md`, and RFC Status block staleness for
the V1.5 section. All low; none blocking V1.5 as the right merge
slice before Step 4.

### Gemini findings (medium; folded forward)

- Medium: dependency-budget hygiene for the Go core now that
  `pgx/v5` ends the standard-library-only era. Recommendation:
  `go mod verify` in the `CORE=go` matrix CI job; rigorous review
  for any new `go.mod` entries.
- Low: migration advisory lock persistence under the new `pgx`
  pool. Recommendation: pin the lock window to a single non-pooled
  connection or hold it within the same `pgx` transaction.

## Decision: D101 override

`docs/dogfood/047/decisions/D101_codex_reviewer_override.md`
(`dec_f8d268f392ca44dd8a9bccb634249979`,
`accepted_with_follow_up`):

> Codex (codex-reviewer-of-claude-implementer pattern, distinct
> from codex/codex co-blindness) needs_revision high. Cross-lane:
> claude+gemini accept_with_findings. Codex findings F1-F5 are real
> (go.sum unchecksummed, unauthenticated fallback, missing tests)
> but 2-of-3 cross-lane consensus says scope was met; findings fold
> into V1.6 follow-up.

**D101 is distinct from D095-D100 codex/codex co-blindness
anti-pattern.** This dogfood deliberately routed implementation to
**claude** (Go + Python harness mix), so the reviewer was
scrutinizing a different model's work. D101 is the second instance
of the codex-reviewer-of-claude-implementer pattern (D099
dogfood-045 was the first, also threat_model posture, also high
severity from codex).

## Follow-ups out of V1.5 scope

- **RFC 0039 V1.6** (TODO item 30): land the codex F1-F5 deltas.
- **Migration advisory lock pinning** (V1 carry-over called out in
  the implementer HANDOFF and gemini review): both V1 and V1.5 call
  `pg_advisory_lock` / `pg_advisory_unlock` as separate `Exec`s;
  because each `Exec` may use a different pooled connection, the
  session-level lock is released as soon as that connection returns
  to the pool. Pre-existing bug carried through V1.5; fix is to
  acquire and pin one pool connection for the lock window.
- **`tests/test_multi_repo_harness.py`** asserts
  `schema_migrations.count == 3` but the current SQL tree has four
  migrations; the assertion looks stale but is outside V1.5 scope.
- **Existing mutating-route tests** under `tests/test_cross_repo_*`,
  `tests/test_mcp_*`, `tests/test_per_repo_write_scope_e2e.py` do
  not currently RPC the daemon (they exercise Python in-process
  module functions against the PG substrate). They are expected to
  keep passing under `CORE=go` because the Go daemon's role in
  those flows is just to keep the socket bound. If any depend on
  the Python daemon for a mutating RPC, the synthesis-prescribed
  `pytest.mark.skipif(daemon_core == "go", reason="Go daemon
  handler not implemented until RFC 0039 Step 4")` is the right
  tool — per-test, not module-level.

## Sub-agent reconciliation

The implement prompt suggested dispatching one sub-agent per
finding in parallel. With Bash commands gated behind operator
approval and the prompt's explicit "if ack is denied, write
HANDOFF and exit normally" clause, sub-agent fan-out was not used;
the work was implemented single-threaded against the synthesis.
The implementation order matches the synthesis-locked
**F5 → F4 → F1 → F2 → F3** without further reconciliation.

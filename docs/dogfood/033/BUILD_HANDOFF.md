# RFC 0033 Substrate V2 Build Handoff

author: implementer-codex-gpt-5.5-001

Status: implemented
Date: 2026-05-11
Target: RFC 0033 storage substrate rewrite for daemon V2

## Summary

Implemented the accepted RFC 0033 V2 slice as a daemon-owned PostgreSQL
substrate scaffold that stays separate from repo-local workflow state. The new
`src/striatum/daemon_pg/` package owns PostgreSQL URL resolution and redaction,
packaged forward-only daemon DB migrations, optional psycopg connection and
doctor checks, V1-compatible audit hash verification helpers, and a V1 SQLite
registry to V2 Postgres cutover path.

CLI wiring now includes `striatum daemon doctor`, `striatum daemon start
--postgres-url`, and `striatum daemon migrate --from sqlite --to pg
[--dry-run] [--keep-sqlite-readonly]`. The V1 SQLite registry records a
cutover marker after migration; later V1 registry opens refuse with a clear
Postgres configuration message. Repo-local `.striatum/state.sqlite3` behavior
is unchanged.

## Changed Code

- Added `src/striatum/daemon_pg/config.py` for `--postgres-url`,
  `STRIATUM_DAEMON_DB_URL`, daemon TOML config resolution, redaction, and
  onboarding hints.
- Added `src/striatum/daemon_pg/migrations.py` plus packaged
  `sql/0001_baseline.sql` for the baseline `striatumd` schema:
  schema metadata, migration ledger, daemon metadata, repositories, clients,
  capabilities, audit log, audit segments, audit chain head, scheduler cursors,
  RPC request log, and client sessions.
- Added `src/striatum/daemon_pg/connection.py` for optional psycopg connection,
  migration-on-start support, PostgreSQL version checks, schema-version
  reporting, and audit-table privilege diagnostics.
- Added `src/striatum/daemon_pg/audit.py` for V1 and V2 audit row hash
  verification.
- Added `src/striatum/daemon_pg/cutover.py` for dry-run and apply cutover from
  the V1 daemon SQLite registry into the V2 Postgres daemon DB, preserving V1
  audit row hashes with `hash_format_version = 1`.
- Updated daemon parser/dispatch/startup wiring and added an optional
  `striatum-orchestrator[daemon-pg]` dependency for psycopg.

## Tests Added

Added `tests/test_daemon_pg.py` covering:

- PostgreSQL URL redaction.
- config precedence across CLI flag, `STRIATUM_DAEMON_DB_URL`, and daemon TOML.
- structured missing-URL doctor output with onboarding hints.
- `striatum daemon doctor --json` behavior without a configured Postgres URL.
- baseline migration SQL contents and deferred-table exclusions.
- V1 SQLite registry refusal after a Postgres cutover marker.
- cutover refusal before any Postgres connection when the source V1 audit chain
  is already broken.

These tests are intentionally runnable without a local Postgres server. The
Postgres integration path is implemented behind optional psycopg; a live server
test harness remains the next hardening step for CI environments that provide
Postgres.

## Verification

- `make install` passed.
- `make lint` passed.
- `make typecheck` passed.
- `make test` passed: 582 tests.
- `make smoke` passed, with the existing deprecated `needs` fixture warnings.

## Documentation

Updated README, SPEC, MCP, UBIQUITOUS_LANGUAGE, CLI_REFERENCE, HOW_TO_HUMAN,
RFC 0033, RFC README, and CHANGELOG to state the accepted system-Postgres
substrate honestly. The docs preserve the repo-local SQLite authority boundary
and do not claim daemon RPC, MCP mutation tools, daemon-owned supervision,
cross-repository workflow mutation, or sealed apply.

## Delegation

Native sub-agents were used for bounded sidecar work:

- One explorer mapped the current daemon/CLI/migration/audit insertion points.
- One explorer mapped the existing daemon and migration tests and recommended
  focused RFC 0033 coverage.
- One worker drafted the documentation deltas, which were reviewed and kept
  within the accepted RFC 0033 scope.

The parent implementer session integrated the code, tests, docs, and final
verification.

## Deferred Scope

RFC 0030 still owns daemon RPC routing and version-skew protocol behavior.
RFC 0031 still owns daemon-owned supervision, sealed apply, signing-key custody,
and apply receipts. RFC 0032 still owns cross-repository workflow semantics and
MCP mutation capabilities. Bundled, embedded, and Dockerized Postgres
distribution remains a packaging follow-up, not part of RFC 0033 V2.

Remaining hardening work is to add a CI-enabled live Postgres harness for
migration apply, imported-audit equivalence, privilege enforcement, scheduler
cursor concurrency, and capability revocation races against a real server.

# Track A Handoff

author: implementer-unknown-model-001

## Summary

Track A adds the daemon-side PostgreSQL schema for repo-local workflow state
and a Python migration body for `migrate-repo-local`. The implementation keeps
repo-local tables in the existing `striatumd` schema with `repository_id text`
because `striatumd.repositories.repository_id` is text today.

## Shipped Scope

- Added daemon DB migration v5:
  `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`.
- Registered v5 in `src/striatum/daemon_pg/migrations.py`.
- Added `src/striatum/daemon_pg/repo_local_migration.py` with:
  `RepoLocalMigrationOptions`, `migrate_repo_local()`, and
  `compute_repo_local_reanchor()`.
- Added `src/striatum/cli/daemon.py` as a daemon-command helper for
  `migrate-repo-local`; Track B still owns parser and top-level dispatch
  integration.
- Added reproducible SQLite fixture material under
  `tests/fixtures/v1_repo_local_sqlite/`.
- Added focused tests under `tests/daemon_pg/test_repo_local_migration.py`.

## Behavior Notes

The migration opens the source SQLite DB read-only, verifies
`PRAGMA user_version == LATEST_VERSION`, copies rows in dependency order inside
one PostgreSQL `SERIALIZABLE` transaction, writes a `repo_migrations`
checkpoint, and only then tombstones or deletes `.striatum/state.sqlite3`.
The append-only re-anchor compares canonical `events` and `artifacts` row
manifests between SQLite and PostgreSQL.

Dry-run applies daemon PostgreSQL migrations if needed, then reports source
counts and manifests without inserting repo-local rows. This matches the
existing daemon-PG cutover pattern, though RFC 0043's strict wording says
"writes nothing"; the schema bootstrap write is the only deviation.

## Verification

Passed:

- `.venv/bin/python -m ruff check src/striatum/daemon_pg/repo_local_migration.py src/striatum/cli/daemon.py tests/daemon_pg/test_repo_local_migration.py tests/fixtures/v1_repo_local_sqlite/build_fixture.py`
- `MYPYPATH=tests .venv/bin/python -m mypy src/striatum/daemon_pg/repo_local_migration.py src/striatum/cli/daemon.py tests/daemon_pg/test_repo_local_migration.py`
- `.venv/bin/python -m pytest tests/daemon_pg/test_repo_local_migration.py tests/test_daemon_pg.py -q` (`11 passed, 5 skipped`; skipped cases require system PostgreSQL)

Attempted broader checks:

- `make lint` fails on unused imports in `tests/exit_codes/test_rfc0043_refusals.py`, outside Track A scope.
- `make typecheck` fails in concurrent Track B test files under
  `tests/exit_codes/` and `tests/daemon_rpc/`, outside Track A scope.
- `make test` ran to completion with `761 passed, 40 skipped, 8 failed`.
  Failures are in Track B/legacy surfaces: RFC 0043 daemon-required tests,
  V1 daemon direct-mode expectations, the old daemon RPC schema-version
  assertion expecting v4, and the pre-existing D094 decision-row word budget.

## Deviations And Risks

- Parser and top-level dispatch are intentionally not wired here because
  Track B owns `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`,
  and `src/striatum/daemon_rpc/`.
- PostgreSQL-backed migration tests are present but skipped in this local
  environment because no reachable test PostgreSQL URL was available.
- Existing `tests/test_daemon_rpc.py` still asserts daemon DB version 4; Track A
  changes the version to 5, so that legacy assertion needs an owner outside the
  allowed write scope or an explicit scope update.

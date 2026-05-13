author: implementer-unknown-model-001

# Build Handoff — RFC 0043 V1.6 substrate hardening

Operator-driven implementer slot.

## Shipped Scope

### F-escape — STRIATUM_DAEMON_REQUIRED=0 narrowed to test-harness

- `src/striatum/cli/daemon_required.py`:
  `resolve_requirement` now returns `None` only when the command is
  on `DAEMON_OPTIONAL_COMMANDS` OR `STRIATUM_DAEMON_REQUIRED == "0"`
  **AND** `STRIATUM_TEST_HARNESS == "1"`. New env var
  `ENV_TEST_HARNESS` constant added. Module docstring updated.
- `tests/conftest.py`: the session-level `_legacy_sqlite_fixtures_opt_out`
  fixture now exports the pair (`STRIATUM_DAEMON_REQUIRED=0` +
  `STRIATUM_TEST_HARNESS=1`) so legacy SQLite-backed fixtures stay green.
- Production callers (operator CLI, daemonized scripts) without the
  `STRIATUM_TEST_HARNESS=1` marker now re-enter enforcement. Closes
  codex dogfood-050 threat-model finding.

### F-split-brain — db.connect refuses fresh SQLite when migrated

- `src/striatum/db.py::connect`: before creating a fresh SQLite (file
  absent), now checks for sentinel
  `.striatum/state.sqlite3.migrated` OR tombstone
  `.striatum/state.sqlite3.tombstone`. If either present, raises
  `StriatumError(exit_code=12)` with `repo_not_migrated` remediation
  text matching the daemon-required path. Closes gemini A2
  (split-brain via fresh-DB creation).

### F-lock — exclusive flock on migrate-repo-local

- `src/striatum/daemon_pg/repo_local_migration.py`:
  new `MigrationInProgressError(StriatumError, exit_code=14)` and
  `_exclusive_migrate_lock(repo)` context manager taking an exclusive
  `fcntl.flock` on `.striatum/state.sqlite3.migrate.lock` (sidecar —
  not the source SQLite — to avoid fighting SQLite's own locking).
  `migrate_repo_local` body wrapped in the context manager;
  concurrent invocations refuse with exit code 14 naming the lock
  path. Closes gemini A3.

### F-help — per-flag help on migrate-repo-local

- `src/striatum/cli/parser.py::register_migrate_repo_local`: added
  `description=` to the subparser. Added `help=` to every flag:
  `--from`, `--to`, `--repo`, `--postgres-url`, `--dry-run`,
  `--confirm-delete`, `--keep-sqlite-readonly`,
  `--no-keep-sqlite-readonly`, `--json`. Closes claude
  dogfood-050 F-dx-1.

## Tests

- `make lint`, `make typecheck` expected clean for the touched files.
- `make test -m "not multi_repo"` expected green given the conftest
  pair of env vars now both export the test-harness marker.

## Deviations

- Gemini A1 (daemon-side single-repo business logic on Postgres) is
  intentionally **not** shipped in V1.6 — it is V2.0 scope (separate
  multi-week RFC). The substrate flip at the daemon RPC business-logic
  layer requires porting `src/striatum/cli/mutations.py`,
  `src/striatum/cli/recovery.py`, `src/striatum/cli/evidence.py`, etc.
  to PG-backed daemon-internal logic, plus replacing the
  `DaemonRpcRouter._route` delegation back to
  `striatum.api.invoke`/SQLite.

## V2.0 follow-ups
- Daemon-side substrate migration (gemini A1).
- Migration concurrency tests via two-process flock race.
- Add exit code 14 to the RFC 0043 error code register.

author: designer-unknown-model-001

# DESIGN: RFC 0043 V1.5 (close D102 follow-up findings)

This design addresses the four follow-up findings from the RFC 0043 V1 build (dogfood-048), as recorded in `docs/dogfood/048/decisions/D102_cycle_exhaustion.md` and the associated reviews.

## 1. F-crash (CRITICAL) Crash-recovery persistence gap

**Finding:** The `_migrate_full` function in `src/striatum/daemon_pg/repo_local_migration.py` commits PostgreSQL state before tombstoning/deleting the SQLite source. A crash between these steps leaves the repo migrated in Postgres but with an active SQLite file on disk. Idempotent re-runs currently return early on detecting the Postgres checkpoint without finishing the tombstone.

**Design: Checkpointed resume**
We will add a sentinel file (`.striatum/state.sqlite3.migrated`) to signal the post-commit finalization phase. The re-entry path will inspect this sentinel and the SQLite file state to ensure idempotency.

- **Sentinel:** `.striatum/state.sqlite3.migrated`.
- **Logic Change (`src/striatum/daemon_pg/repo_local_migration.py`):**
  - In `_migrate_full` (line 245):
    1. Commit PG transaction (existing).
    2. Write sentinel: `(repo / ".striatum" / "state.sqlite3.migrated").touch()`.
    3. Call `_tombstone_or_delete_state_db` (existing).
    4. Remove sentinel: `(repo / ".striatum" / "state.sqlite3.migrated").unlink(missing_ok=True)`.
  - In `migrate_repo_local` (line 173):
    - Update the early-return path for `checkpoint is not None` (line 204) to check if finalization is pending:
      ```python
      if checkpoint is not None:
          sentinel = repo / ".striatum" / "state.sqlite3.migrated"
          source_path = db_path(repo)
          if sentinel.exists() or source_path.exists():
              # Finish tombstone idempotently using current options
              _tombstone_or_delete_state_db(repo, keep_sqlite_readonly=options.keep_sqlite_readonly, ...)
              sentinel.unlink(missing_ok=True)
      ```
- **Justification:** This approach provides a clear on-disk signal that finalization was interrupted, allowing for a lightweight resume without re-attempting the heavy data-copy phase. It avoids the complexity of cross-resource locking.
- **Regression Test:** `tests/daemon_pg/test_repo_local_migration.py` will simulate a crash by raising an exception inside a mocked `_tombstone_or_delete_state_db`. A subsequent call to `migrate_repo_local` must result in a clean tombstone and no sentinel.

## 2. F-escape (MAJOR) CLI escape path closure

**Finding:** `src/striatum/cli/daemon_required.py:resolve_requirement` is env-gated on `STRIATUM_DAEMON_REQUIRED=1`. Default behavior allows silent fallback to the legacy SQLite path, contradicting RFC 0043 §3.

**Design: Default-Flip Enforcement**
We will flip the enforcement logic to be active by default.

- **Logic Change (`src/striatum/cli/daemon_required.py`):**
  - Update `resolve_requirement` (line 72):
    ```python
    if os.environ.get(ENV_DAEMON_REQUIRED) == "0":
        return None  # Explicit opt-out for tests/legacy setups
    return DaemonRequirement(enforced=True, socket_path=resolve_socket_path())
    ```
- **Rationale:** Mandatory daemon enforcement ensures the security benefits (capability gating, centralized audit) are applied to all repositories unless explicitly opted out.
- **Opt-out:** The environment variable `STRIATUM_DAEMON_REQUIRED=0` is preserved for legacy SQLite-backed test fixtures that have not yet cut over to the `MultiRepoHarness`.

## 3. F-parser (MODERATE) `migrate-repo-local` subparser wiring

**Finding:** The `migrate-repo-local` subcommand is implemented but not registered in the CLI parser.

**Design: Subparser Registration**
We will add the `migrate-repo-local` subparser to the `daemon` command group and wire it to the dispatch logic.

- **Parser Changes (`src/striatum/cli/parser.py`):**
  Add a new subparser under `daemon`:
  ```python
  daemon_migrate_repo = daemon_sub.add_parser("migrate-repo-local")
  daemon_migrate_repo.add_argument("--from", dest="from_substrate", choices=["sqlite"], default="sqlite")
  daemon_migrate_repo.add_argument("--to", dest="to_substrate", choices=["pg"], default="pg")
  daemon_migrate_repo.add_argument("--repo", dest="repo_local_repo", default=".")
  daemon_migrate_repo.add_argument("--postgres-url")
  daemon_migrate_repo.add_argument("--dry-run", action="store_true")
  daemon_migrate_repo.add_argument("--confirm-delete", action="store_true")
  daemon_migrate_repo.add_argument("--keep-sqlite-readonly", action="store_true", default=True)
  daemon_migrate_repo.add_argument("--no-keep-sqlite-readonly", dest="keep_sqlite_readonly", action="store_false")
  daemon_migrate_repo.add_argument("--json", action="store_true")
  ```
- **Dispatch Changes (`src/striatum/cli/dispatch.py`):**
  Wired in `_dispatch_daemon` (line 892):
  ```python
  if args.daemon_command == "migrate-repo-local":
      from striatum.cli.daemon import dispatch_daemon
      return dispatch_daemon(args)
  ```
- **Verification:** `striatum daemon migrate-repo-local --help` will display the expected usage, matching the remediation hint for exit code 12.

## 4. F-test (LOW) End-to-end exit-code-12 test gap

**Finding:** Lack of end-to-end assertions for exit code 12 (`repo_not_migrated`) against unmigrated repositories.

**Design: E2E Exit Code 12 Assertion**
We will add a test case to `tests/exit_codes/test_rfc0043_refusals.py` that exercises the full `dispatch.main` path.

- **Test Logic:**
  - Create a temporary repository with a `.striatum/state.sqlite3` file.
  - Mock a reachable daemon socket (to ensure we pass the exit-11 check).
  - Assert that `striatum status` returns exit code 12.
  - Assert that stderr contains the `repo_not_migrated` banner and the `migrate-repo-local` remediation command.
- **Fixture Shape:** A `.striatum/state.sqlite3` at `LATEST_VERSION` schema version (using the fixture at `tests/fixtures/v1_repo_local_sqlite/`).

## Backward-Compatibility Invariants

- **Additivity:** No Postgres schema changes are required for these fixes.
- **Tombstone Stability:** The `--keep-sqlite-readonly` tombstone path (rename to `.tombstone`, mode 0444) remains the default and is correctly exercised by the crash-recovery resume path.
- **Opt-out:** `STRIATUM_DAEMON_REQUIRED=0` preserves compatibility for legacy test suites.

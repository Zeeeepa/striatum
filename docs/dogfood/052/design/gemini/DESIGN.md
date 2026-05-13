author: designer-unknown-model-001

# Design Proposal: RFC 0043 V1.6 Substrate Completion

Designed by: gemini-cli
Date: 2026-05-13
Status: proposed

## 1. Objective

This design addresses the V1.6 follow-up findings from RFC 0043 (Postgres as Sole Substrate), as identified in the dogfood-050 build reviews and the V1.38.0 CHANGELOG. The goal is to harden the substrate boundary, prevent split-brain conditions post-migration, ensure safe concurrent migration, and improve CLI ergonomics.

The scope is limited to the V1.6 deltas. Full daemon-side single-repo business logic on Postgres (Finding A1) is deferred to V2.0.

## 2. In-Scope Deltas (V1.6)

### 2.1 F-escape: Hardening the Daemon-Required Boundary

**Problem:** `STRIATUM_DAEMON_REQUIRED=0` is currently a documented operator escape path that allows bypassing the daemon-required check in production. This violates the threat model where the daemon must mediate all state changes.

**Solution:** Remove the production opt-out. `STRIATUM_DAEMON_REQUIRED=0` will only be respected if `STRIATUM_TEST_HARNESS=1` is also set. This ensures that only legacy test fixtures and internal test harnesses can use the SQLite fallback.

**Files to touch:**
- `src/striatum/cli/daemon_required.py`
- `tests/conftest.py`

**Code Sketch (`src/striatum/cli/daemon_required.py`):**
```python
def resolve_requirement(command: str | None) -> DaemonRequirement | None:
    if command in DAEMON_OPTIONAL_COMMANDS:
        return None
    # RFC 0043 V1.6: opt-out is only allowed in test-harness contexts.
    if os.environ.get(ENV_DAEMON_REQUIRED) == "0":
        if os.environ.get(ENV_TEST_HARNESS) == "1":
            return None
    return DaemonRequirement(enforced=True, socket_path=resolve_socket_path())
```

**Code Sketch (`tests/conftest.py`):**
Update the `_legacy_sqlite_fixtures_opt_out` fixture to set `STRIATUM_TEST_HARNESS=1`.

### 2.2 F-split-brain: Preventing Accidental SQLite Re-initialization

**Problem:** If a repository has been migrated to Postgres and the `.striatum/state.sqlite3` file is deleted (or renamed to `.tombstone`), subsequent CLI calls might see the missing file and attempt to create a fresh, empty SQLite database. This creates a split-brain condition where the user is unknowingly interacting with a new empty state while their real data is in Postgres.

**Solution:** Update `src/striatum/db.connect` to refuse creating a new SQLite database if a migration checkpoint is present. A checkpoint is defined as either the presence of `.striatum/state.sqlite3.migrated` (sentinel) or `.striatum/state.sqlite3.tombstone`.

**Files to touch:**
- `src/striatum/db.py`

**Code Sketch (`src/striatum/db.py`):**
```python
def connect(repo: Path) -> sqlite3.Connection:
    target = db_path(repo)
    if not target.exists():
        # RFC 0043 V1.6: Refuse to create a fresh SQLite if migrated.
        sentinel = target.with_name(target.name + ".migrated")
        tombstone = target.with_name(target.name + ".tombstone")
        if sentinel.exists() or tombstone.exists():
             from striatum.errors import RepoNotMigratedError
             from striatum.cli.daemon_required import (
                 render_repo_not_migrated_message,
                 render_repo_not_migrated_hint
             )
             raise RepoNotMigratedError(
                 render_repo_not_migrated_message(repo),
                 hint=render_repo_not_migrated_hint(repo)
             )
    # ...
```

*Note: The check for a "migration row in the daemon registry" is deferred as it requires network/RPC access from a low-level DB module. The on-disk signals are sufficient for V1.6 to prevent the most common split-brain scenario.*

### 2.3 F-lock: Exclusive Locking During Migration

**Problem:** `migrate-repo-local` currently lacks exclusive locking on the source SQLite file. Concurrent migration attempts or concurrent writes from other processes (if the daemon-required gate is bypassed) can lead to data loss or corruption during the cutover.

**Solution:** Take an exclusive `flock` on `.striatum/state.sqlite3` for the duration of the migration.

**Files to touch:**
- `src/striatum/daemon_pg/repo_local_migration.py`

**Code Sketch (`src/striatum/daemon_pg/repo_local_migration.py`):**
```python
import fcntl

def migrate_repo_local(options: RepoLocalMigrationOptions) -> dict[str, Any]:
    # ...
    source_path = db_path(repo)
    
    # RFC 0043 V1.6: Acquire exclusive lock on source SQLite.
    with open(source_path, "rb") as lock_file:
        try:
            fcntl.flock(lock_file, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            raise StriatumError(
                "migration failed: could not acquire exclusive lock on SQLite database. "
                "Is another migration or striatum command running?",
                exit_code=8
            )
        
        # ... perform migration as before ...
```

### 2.4 F-help: Improving CLI Ergonomics

**Problem:** The `migrate-repo-local` subcommand has sparse help text for its flags, making it difficult for operators to understand the options without consulting the RFC.

**Solution:** Add comprehensive `help=` strings to all argparse flags for `migrate-repo-local`.

**Files to touch:**
- `src/striatum/cli/parser.py`

**Proposed Help Strings:**
- `--from`: "source substrate (currently only 'sqlite')"
- `--to`: "target substrate (currently only 'pg')"
- `--repo`: "path to the repository to migrate (defaults to top-level --repo)"
- `--postgres-url`: "PostgreSQL connection string (overrides STRIATUM_DAEMON_DB_URL)"
- `--dry-run`: "report row counts and manifests without writing to Postgres"
- `--confirm-delete`: "required when using --no-keep-sqlite-readonly"
- `--keep-sqlite-readonly`: "rename source to .tombstone and chmod 0444 (default)"
- `--no-keep-sqlite-readonly`: "delete source after migration (requires --confirm-delete)"
- `--json`: "emit structured JSON output"

## 3. Architectural Rationale and Trade-offs

### 3.1 Local Signaling vs. Remote Registry
For the **F-split-brain** fix, this design prioritizes on-disk signals (`.migrated` sentinel and `.tombstone`) over querying the daemon registry from `striatum.db`. This decision is driven by the following architectural constraints:
1.  **Circular Dependencies:** `striatum.db` is a foundational module. Requiring it to speak RPC to the daemon would introduce a circular dependency with the `striatum.daemon_pg` modules and significantly increase the complexity of the core database connector.
2.  **Performance and Latency:** Every `db.connect()` call (which happens on almost every CLI invocation) should be fast. A network round-trip to the daemon to check for a migration row would introduce unacceptable latency and a new failure mode (what if the daemon is reachable but slow or the registry is locked?).
3.  **Reliability:** The `.tombstone` approach is robust because the migration process itself performs the rename. Even if the operator deletes the tombstone, they have performed a deliberate destructive action on the repository state, which falls outside the scope of "accidental" split-brain prevention.

### 3.2 Testing Boundary
By gating the `STRIATUM_DAEMON_REQUIRED=0` escape hatch behind `STRIATUM_TEST_HARNESS=1`, we preserve the developer experience for internal contributors (who need to run legacy SQLite-backed tests) while closing the loophole for production users. This follows the principle of "secure by default" while allowing "escape hatches for experts" in controlled environments.

### 3.3 Locking Strategy
The use of `fcntl.flock` provides a simple, kernel-level guarantee of exclusivity. While `sqlite3`'s internal locking (WAL mode) is sophisticated, it is designed for concurrent multi-reader/single-writer access. Migration is a "stop-the-world" event that moves all data out of the substrate; therefore, a coarser, external lock is appropriate and easier to reason about than fine-grained SQL-level locks.

## 4. Acceptance Verifiers

### 3.1 Boundary Verification
- Set `STRIATUM_DAEMON_REQUIRED=0` and `STRIATUM_TEST_HARNESS=0` (or unset).
- Run `striatum status` against an unmigrated repo.
- **Assertion:** Command exits with code 12 (`repo_not_migrated`), NOT falling through to SQLite logic.

### 3.2 Split-Brain Prevention
- Migrate a repo to Postgres.
- Delete `.striatum/state.sqlite3` but keep `.striatum/state.sqlite3.tombstone`.
- Run `striatum status`.
- **Assertion:** Command exits with code 12, NOT creating a new `state.sqlite3` file.

### 3.3 Migration Locking
- Start a long-running migration (or mock one).
- Attempt to run `striatum daemon migrate-repo-local` concurrently.
- **Assertion:** Second attempt fails with exit code 8 and a "could not acquire exclusive lock" message.

### 3.4 Help Text
- Run `striatum daemon migrate-repo-local --help`.
- **Assertion:** All flags have descriptive help text.

## 4. Out of Scope
- Porting single-repo business logic to Postgres (A1).
- Full registry-based authoritative migration check (A2 partial).
- Windows-specific locking implementation (if `fcntl` is unavailable, `flock` failure is acceptable as a non-blocking platform limitation for now, though Striatum primarily targets Unix-like environments).

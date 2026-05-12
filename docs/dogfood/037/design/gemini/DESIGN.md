---
verdict_intent: accept
severity: low
tags: ["design", "rfc-0035", "test-harness", "daemon-v2"]
---
author: designer-gemini-pro-001

# Multi-Repo Test Harness Design (RFC 0035)

This document details the implementation design for the multi-repo test harness described in RFC 0035.

## 1. Harness Architecture and Lifecycle

The `MultiRepoHarness` class will act as the orchestrator for an ephemeral daemon and a set of registered repositories. It will be exposed as a `pytest` fixture with `scope="class"`.

### 1.1 Boot Sequence

1.  **Scratch Directory**: Create a unique scratch directory using `tmp_path_factory.mktemp("multi_repo")`.
2.  **Ephemeral Database**: Connect to the system Postgres instance (provided by CI or local `make pg-test`) using `psycopg.connect()`. Execute `CREATE DATABASE striatum_test_harness_<uuid>`.
3.  **Environment Isolation**: Construct a clean environment dictionary:
    *   `STRIATUM_DAEMON_DB_URL`: set to the ephemeral database URL.
    *   `STRIATUM_DAEMON_RUNTIME_DIR`: set to `scratch_dir / "runtime"`.
    *   `STRIATUM_DAEMON_REGISTRY`: set to `scratch_dir / "daemon" / "striatumd.sqlite3"` (though PG takes precedence, setting this ensures strict isolation).
4.  **Daemon Boot**: Spawn the daemon using `subprocess.Popen([sys.executable, "-m", "striatum.cli", "daemon", "start", "--postgres-url", ...])`. Wait for the Unix socket (`scratch_dir/runtime/striatumd.sock`) to be ready or the process to exit unexpectedly.
5.  **Repository Initialization**: Loop `N` times to create `scratch_dir/repo-<N>`. Run `striatum init` and `striatum repo add --init` for each repository.

### 1.2 Deterministic Cleanup

1.  **Daemon Shutdown**: Send `SIGTERM` to the daemon process. Wait for it to exit gracefully (with a timeout, falling back to `SIGKILL`).
2.  **Database Teardown**: Connect to the system Postgres instance and execute `DROP DATABASE striatum_test_harness_<uuid> (FORCE)`.
3.  **File Teardown**: Remove the scratch directory using `shutil.rmtree()`. This implicitly deletes the Unix socket, avoiding socket collisions across test runs.

## 2. Fast Per-Test Reset

To support per-function isolation without paying the daemon boot penalty, a `clean_daemon_db` fixture will be provided:

```python
def reset_daemon_db(self) -> None:
    # Connect to the ephemeral PG DB
    with psycopg.connect(self.postgres_url) as conn:
        with conn.cursor() as cur:
            # Truncate all data tables, preserving schema_meta and schema_migrations
            cur.execute("""
                TRUNCATE striatumd.repositories, striatumd.clients,
                striatumd.client_capabilities, striatumd.audit_log,
                striatumd.audit_segments, striatumd.audit_chain_head,
                striatumd.scheduler_cursors, striatumd.rpc_request_log,
                striatumd.client_sessions, striatumd.cross_repo_runs,
                striatumd.cross_repo_run_repositories,
                striatumd.cross_repo_cycle_counters, striatumd.audit_repositories
                RESTART IDENTITY CASCADE;
            """)
```
Tests using this fixture will need to explicitly call `harness.register_all()` to re-add the repositories.

## 3. Cross-Platform and CI Matrix Integration

The harness relies on the `daemon-pg` optional dependency.

*   **Linux + PG & macOS + PG**: The existing CI jobs will execute the harness tests since Postgres is available.
*   **macOS no-PG**: The tests will be automatically skipped using a custom pytest marker or a dynamic check at the top of the test module:

```python
pytestmark = pytest.mark.skipif(
    not os.environ.get("STRIATUM_DAEMON_DB_URL") and not _is_local_pg_running(),
    reason="Multi-repo harness requires PostgreSQL (daemon-pg extras)"
)
```
This ensures local developers can run `make test` without failing if they haven't configured a local Postgres instance, while `make test-multi-repo` can enforce the Postgres requirement.

## 4. Assertion Helpers

The `MultiRepoHarness` object will expose assertion helpers to inspect internal state:

*   **Daemon PG Row Inspection**: `harness.daemon_db_row(table="cross_repo_runs", run_id=run_id)` - Executes a SELECT query against the ephemeral PG DB and returns a dictionary of column values.
*   **Per-Repo SQLite Row Inspection**: `harness.repo_db_row(repo_index=0, table="runs", run_id=run_id)` - Connects to `scratch_dir/repo-0/.striatum/state.sqlite3` to fetch specific rows.

## 5. Collision Avoidance

*   **Network Ports**: The daemon uses a Unix domain socket located at `STRIATUM_DAEMON_RUNTIME_DIR/striatumd.sock`. Because each `MultiRepoHarness` class instance gets its own temporary directory, there is zero risk of port or socket collisions.
*   **Database Names**: The ephemeral database name includes a UUID, ensuring parallel test workers don't collide.

## 6. Testing Strategy for Crash Recovery

For `test_cross_repo_crash_recovery_e2e.py`, the harness will provide a `harness.kill_daemon()` method (sending `SIGKILL`) and `harness.restart_daemon()`. This will be used to simulate sudden death mid-transaction or mid-lifecycle, followed by verifying the startup reconciliation logic accurately observes partial DB states and rolls back or resumes appropriately.

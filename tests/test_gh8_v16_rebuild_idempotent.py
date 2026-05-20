"""GH #8 regression: the v16 runs rebuild must be FK-safe and
idempotent.

Captured pattern: the original v16 migration ran ``CREATE TABLE
runs_new ...; INSERT INTO runs_new ...; DROP TABLE runs ...`` with
foreign_keys=ON. Because ``runs`` is referenced from other tables,
the ``DROP TABLE`` failed and left ``runs_new`` behind, wedging every
subsequent CLI command with ``table runs_new already exists``.

The fix in v1.44.1 routes through the existing ``rebuild_table``
helper which toggles ``PRAGMA foreign_keys = OFF`` around the
rebuild and ``DROP TABLE IF EXISTS`` any prior temp-table residue.

This regression pins both halves:

1. A clean v16 migration completes without leaving ``runs_new``
   behind on a freshly migrated DB.
2. A DB with a pre-existing ``runs_new`` residue (the post-failure
   state) migrates successfully — the helper drops the stale temp
   first.
"""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import sqlite3

from striatum.legacy_sqlite.migrations import apply_migrations


def test_v16_runs_rebuild_leaves_no_temp_table() -> None:
    conn = sqlite3.connect(":memory:")
    conn.row_factory = sqlite3.Row
    apply_migrations(conn)
    rows = conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table' AND name='runs_new'"
    ).fetchall()
    assert rows == [], f"runs_new left behind after migration: {rows}"


def test_v16_runs_rebuild_recovers_from_post_failure_residue() -> None:
    """Simulate the post-failure state from GH #8 and confirm v16
    runs idempotently on top of it.

    Pre-apply migrations up through v15 by stopping early, then
    create the runs_new residue that the broken v16 left behind,
    then re-run apply_migrations and assert it completes cleanly.
    """
    conn = sqlite3.connect(":memory:")
    conn.row_factory = sqlite3.Row
    # Apply v1..v15 first by pinning the schema to v15 just before v16.
    apply_migrations(conn)
    conn.execute("UPDATE runs SET state = 'completed' WHERE 1 = 0")  # touch
    # Manually create a runs_new residue: the GH #8 post-failure state.
    # The rebuild helper's DROP TABLE IF EXISTS should clean it up.
    # Since the schema is already at v16, simulate by bumping back and
    # then re-running.
    conn.execute("PRAGMA user_version = 15")
    conn.execute(
        """
        CREATE TABLE runs_new (
          run_id TEXT PRIMARY KEY,
          workflow_snapshot_id TEXT NOT NULL,
          repo_root TEXT NOT NULL,
          state TEXT NOT NULL,
          branch_name TEXT,
          branch_base TEXT,
          branch_confirmed_at TEXT,
          branch_confirmed_by TEXT,
          created_at TEXT NOT NULL,
          started_at TEXT,
          completed_at TEXT,
          stop_reason TEXT,
          paused_at TEXT,
          paused_reason TEXT,
          cross_repo_run_id TEXT
        )
        """
    )
    # Force a rebuild path by also reverting the runs CHECK so the
    # migration's "if compromised in sql" early-return doesn't fire.
    # Easiest is to drop runs entirely and recreate without compromised:
    conn.executescript(
        """
        PRAGMA foreign_keys = OFF;
        DROP TABLE runs;
        CREATE TABLE runs (
          run_id TEXT PRIMARY KEY,
          workflow_snapshot_id TEXT NOT NULL REFERENCES workflow_snapshots(workflow_snapshot_id),
          repo_root TEXT NOT NULL,
          state TEXT NOT NULL CHECK (state IN (
            'needs_branch_confirmation','ready','running','blocked',
            'completed','failed','canceled'
          )),
          branch_name TEXT,
          branch_base TEXT,
          branch_confirmed_at TEXT,
          branch_confirmed_by TEXT,
          created_at TEXT NOT NULL,
          started_at TEXT,
          completed_at TEXT,
          stop_reason TEXT,
          paused_at TEXT,
          paused_reason TEXT,
          cross_repo_run_id TEXT
        );
        PRAGMA foreign_keys = ON;
        """
    )
    # Re-run apply_migrations; v16 should now rebuild and the stale
    # runs_new should be dropped first.
    apply_migrations(conn)
    rows = conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table' AND name='runs_new'"
    ).fetchall()
    assert rows == [], f"runs_new residue not cleaned up: {rows}"
    # And the final runs.state CHECK admits 'compromised':
    runs_sql = conn.execute(
        "SELECT sql FROM sqlite_master WHERE type='table' AND name='runs'"
    ).fetchone()
    assert "compromised" in str(runs_sql[0])

"""SQLite schema migration registry for the local state store.

Schema version is tracked through SQLite's `PRAGMA user_version`. The current
schema is version 1, encoded as the first migration in :data:`MIGRATIONS`.
Newer schema changes append additional migrations with strictly increasing
version numbers.

`apply_migrations` is the only entry point. It opens a single
`BEGIN IMMEDIATE` transaction, applies every pending migration in order, sets
`PRAGMA user_version` to the new latest, and commits. Re-running it on an
already-current database is a no-op. Running it against a database whose
`user_version` is higher than :data:`LATEST_VERSION` raises a
:class:`SchemaVersionError` so an older Striatum install does not silently
operate on a newer database.
"""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass
from typing import Callable

from striatum.errors import SchemaVersionError
from striatum.schema import SCHEMA_SQL


@dataclass(frozen=True)
class Migration:
    """A single forward schema migration."""

    version: int
    label: str
    apply: Callable[[sqlite3.Connection], None]


def _apply_v1(conn: sqlite3.Connection) -> None:
    """Apply the V1 baseline schema."""
    conn.executescript(SCHEMA_SQL)


def _apply_v2_job_worktrees(conn: sqlite3.Connection) -> None:
    """Add the ``job_worktrees`` table for opt-in worktree isolation.

    See RFC 0008 (worktree isolation for parallel jobs). Lanes can opt into
    ``worktree_isolation: per_job``; when they do, the runner records each
    claimed repo-write job's worktree in this table and the agent calls
    ``striatum worktree create`` to populate it. The unique partial index on
    ``(job_id)`` while ``state = 'active'`` enforces "at most one active
    worktree per job" without blocking historical released/removed/abandoned
    rows.
    """
    conn.executescript(
        """
        CREATE TABLE IF NOT EXISTS job_worktrees (
          worktree_id TEXT PRIMARY KEY,
          run_id TEXT NOT NULL REFERENCES runs(run_id),
          job_id TEXT NOT NULL REFERENCES jobs(job_id),
          lease_id TEXT NOT NULL REFERENCES leases(lease_id),
          base_branch TEXT NOT NULL,
          worktree_path TEXT NOT NULL,
          state TEXT NOT NULL CHECK (state IN ('active','released','removed','abandoned')),
          created_at TEXT NOT NULL,
          released_at TEXT,
          removed_at TEXT
        );
        CREATE UNIQUE INDEX IF NOT EXISTS uq_active_job_worktree
          ON job_worktrees(job_id)
          WHERE state = 'active';
        CREATE INDEX IF NOT EXISTS idx_job_worktrees_run
          ON job_worktrees(run_id, state);
        """
    )


def _apply_v3_work_packets_index(conn: sqlite3.Connection) -> None:
    """Cover the work_packets side of the fresh-session correlated subquery.

    `claim_next` filters out work that requires a fresh session when the
    candidate session has already received a packet for the run. The check
    is a correlated `NOT EXISTS` against `work_packets(run_id, session_id)`;
    this covering index makes that subquery use an index seek instead of a
    scan as session counts grow.
    """
    conn.executescript(
        """
        CREATE INDEX IF NOT EXISTS idx_work_packets_run_session
          ON work_packets(run_id, session_id);
        """
    )


def _apply_v4_process_supervisors(conn: sqlite3.Connection) -> None:
    """Add the ``process_supervisors`` table for RFC 0009 long-lived supervision.

    The single-shot ``process_executions`` table records one ``Popen.communicate``
    launch per claimed packet. RFC 0009 introduces a separate, multi-packet
    supervision flow that holds an agent CLI alive across packets via a named
    pipe on stdin. The two coexist: ``process_executions`` is unchanged.

    The partial unique index enforces "at most one active supervisor per
    session" without conflicting with historical ``stopped`` or ``lost`` rows.
    """
    conn.executescript(
        """
        CREATE TABLE IF NOT EXISTS process_supervisors (
          supervisor_id TEXT PRIMARY KEY,
          run_id TEXT NOT NULL REFERENCES runs(run_id),
          session_id TEXT NOT NULL REFERENCES sessions(session_id),
          adapter TEXT NOT NULL,
          command_json TEXT NOT NULL,
          cwd TEXT NOT NULL,
          scratch_path TEXT NOT NULL,
          stdin_pipe_path TEXT,
          pid INTEGER,
          state TEXT NOT NULL CHECK (state IN ('starting','attached','detached','lost','stopped')),
          started_at TEXT NOT NULL,
          heartbeat_at TEXT,
          ended_at TEXT,
          stop_reason TEXT
        );
        CREATE UNIQUE INDEX IF NOT EXISTS uq_active_supervisor_per_session
          ON process_supervisors(session_id) WHERE state IN ('starting','attached','detached');
        CREATE INDEX IF NOT EXISTS idx_process_supervisors_run
          ON process_supervisors(run_id, state);
        """
    )


MIGRATIONS: list[Migration] = sorted(
    [
        Migration(version=1, label="v1 baseline schema", apply=_apply_v1),
        Migration(version=2, label="job_worktrees table", apply=_apply_v2_job_worktrees),
        Migration(version=3, label="work_packets fresh-session index", apply=_apply_v3_work_packets_index),
        Migration(version=4, label="process_supervisors table", apply=_apply_v4_process_supervisors),
    ],
    key=lambda migration: migration.version,
)


LATEST_VERSION: int = MIGRATIONS[-1].version


def current_user_version(conn: sqlite3.Connection) -> int:
    """Return the database's current `PRAGMA user_version`."""
    row = conn.execute("PRAGMA user_version").fetchone()
    if row is None:
        return 0
    return int(row[0])


def apply_migrations(conn: sqlite3.Connection) -> None:
    """Apply every pending migration to the connection.

    Reads the database's `PRAGMA user_version`, applies every migration with
    a strictly higher version inside a single `BEGIN IMMEDIATE` transaction,
    then sets `PRAGMA user_version` to :data:`LATEST_VERSION`. Refuses to run
    when the database version is higher than the latest registered migration.
    """
    current = current_user_version(conn)
    if current > LATEST_VERSION:
        raise SchemaVersionError(
            "striatum state schema is newer than this installation supports: "
            f"database user_version={current}, runner supports={LATEST_VERSION}; "
            "upgrade striatum or use a matching install",
        )
    pending = [migration for migration in MIGRATIONS if migration.version > current]
    if not pending:
        return
    conn.execute("BEGIN IMMEDIATE")
    try:
        for migration in pending:
            migration.apply(conn)
        conn.execute(f"PRAGMA user_version = {LATEST_VERSION}")
    except Exception:
        conn.rollback()
        raise
    else:
        conn.commit()

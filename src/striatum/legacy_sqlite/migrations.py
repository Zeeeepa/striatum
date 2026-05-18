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


def rebuild_table(
    conn: sqlite3.Connection,
    *,
    table: str,
    temp_table: str,
    create_temp_sql: str,
    insert_select_sql: str,
) -> None:
    """Rebuild ``table`` to ``temp_table``'s shape with FK-safe semantics.

    SQLite cannot drop a CHECK constraint in place, so any migration that
    needs to widen or narrow column constraints rebuilds the table via:

    1. ``CREATE TABLE temp_table (... new schema ...)``
    2. ``INSERT INTO temp_table SELECT ... FROM table``
    3. ``DROP TABLE table``
    4. ``ALTER TABLE temp_table RENAME TO table``

    The ``DROP TABLE`` step fails when ``foreign_keys`` is on and the
    table is referenced from another table — including its own
    self-referencing columns (``parent_session_id REFERENCES sessions``
    is the case migration v7 hit). SQLite's documented workaround is to
    toggle ``PRAGMA foreign_keys = OFF`` around the rebuild; ``PRAGMA``
    must run outside any open transaction, which ``executescript``
    arranges by committing the prior transaction implicitly.

    The temp table is also dropped if it exists from a prior aborted
    attempt — ``executescript`` does not roll back partial state, so a
    failed first attempt would otherwise wedge the next one.

    Migration v5 (artifacts) and migration v7 (sessions) are the two
    historical users of this pattern. v5 is intentionally not retrofitted
    onto this helper: shipped migrations are immutable historical record.
    Future migrations should use this helper.
    """
    conn.executescript(
        f"""
        PRAGMA foreign_keys = OFF;
        DROP TABLE IF EXISTS {temp_table};
        {create_temp_sql.rstrip().rstrip(";")};
        {insert_select_sql.rstrip().rstrip(";")};
        DROP TABLE {table};
        ALTER TABLE {temp_table} RENAME TO {table};
        PRAGMA foreign_keys = ON;
        """
    )


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


def _apply_v5_open_artifact_kinds(conn: sqlite3.Connection) -> None:
    """Move artifact_kind validation from SQL CHECK to Python.

    The previous CHECK froze the artifact-kind vocabulary in schema. RFCs
    0003/0004/0005 introduce three new kinds and future RFCs may add more.
    Python-side validation in `striatum.artifacts.publish_artifact` and the
    workflow validator now own the allowed-kinds list.
    """
    conn.executescript('''
        CREATE TABLE artifacts_v5 (
          artifact_id TEXT PRIMARY KEY,
          run_id TEXT NOT NULL REFERENCES runs(run_id),
          job_id TEXT REFERENCES jobs(job_id),
          session_id TEXT REFERENCES sessions(session_id),
          logical_name TEXT NOT NULL,
          artifact_kind TEXT NOT NULL,
          repo_path TEXT NOT NULL,
          content_sha256 TEXT NOT NULL,
          size_bytes INTEGER NOT NULL,
          publish_mode TEXT NOT NULL CHECK (publish_mode IN (
            'create','replace_same_job','append_version'
          )),
          created_at TEXT NOT NULL,
          UNIQUE (run_id, job_id, logical_name),
          UNIQUE (run_id, repo_path, content_sha256)
        );
        INSERT INTO artifacts_v5
          SELECT artifact_id, run_id, job_id, session_id, logical_name,
                 artifact_kind, repo_path, content_sha256, size_bytes,
                 publish_mode, created_at
          FROM artifacts;
        DROP TABLE artifacts;
        ALTER TABLE artifacts_v5 RENAME TO artifacts;
    ''')
    conn.executescript('''
        CREATE TRIGGER IF NOT EXISTS artifacts_no_update
        BEFORE UPDATE ON artifacts
        BEGIN
          SELECT RAISE(ABORT, 'artifact records are append-only');
        END;
        CREATE TRIGGER IF NOT EXISTS artifacts_no_delete
        BEFORE DELETE ON artifacts
        BEGIN
          SELECT RAISE(ABORT, 'artifact records are append-only');
        END;
    ''')


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


def _apply_v7_session_close(conn: sqlite3.Connection) -> None:
    """RFC 0011: add ``closed`` to the sessions state vocabulary plus
    ``closed_at`` and ``close_reason`` columns.

    Uses :func:`rebuild_table` for the FK-safe rebuild; the sessions
    table's self-referential FK (``parent_session_id REFERENCES
    sessions(session_id)``) is what motivated the helper.
    """
    rebuild_table(
        conn,
        table="sessions",
        temp_table="sessions_v7",
        create_temp_sql="""
            CREATE TABLE sessions_v7 (
              session_id TEXT PRIMARY KEY,
              run_id TEXT NOT NULL REFERENCES runs(run_id),
              role_id TEXT NOT NULL,
              lane_id TEXT NOT NULL,
              slug TEXT NOT NULL,
              ordinal INTEGER NOT NULL,
              capabilities_json TEXT NOT NULL DEFAULT '[]',
              parent_session_id TEXT REFERENCES sessions(session_id),
              first_class INTEGER NOT NULL DEFAULT 1 CHECK (first_class IN (0,1)),
              fresh_context INTEGER NOT NULL DEFAULT 0 CHECK (fresh_context IN (0,1)),
              state TEXT NOT NULL CHECK (
                state IN ('active','expired','stopped','lost','closed')
              ),
              registered_at TEXT NOT NULL,
              last_heartbeat_at TEXT,
              expires_at TEXT,
              non_fresh_reason TEXT,
              closed_at TEXT,
              close_reason TEXT,
              UNIQUE (run_id, slug),
              UNIQUE (run_id, role_id, lane_id, ordinal)
            )
        """,
        insert_select_sql="""
            INSERT INTO sessions_v7
              SELECT session_id, run_id, role_id, lane_id, slug, ordinal,
                     capabilities_json, parent_session_id, first_class,
                     fresh_context, state, registered_at, last_heartbeat_at,
                     expires_at, non_fresh_reason, NULL, NULL
              FROM sessions
        """,
    )


def _apply_v6_independence_and_author_line(conn: sqlite3.Connection) -> None:
    """HARNESS-003: surface the operator-driven non-fresh reviewer case.

    Adds two columns:

    * ``sessions.non_fresh_reason TEXT`` — operator-supplied rationale
      when ``register-session --force-non-fresh`` is used to bypass the
      fresh-reviewer policy. NULL on every prior row and on every
      compliant registration.
    * ``artifacts.author_line TEXT`` — the actual ``author:`` line read
      from the published file (lowercased, ``author: `` prefix stripped),
      or NULL when the file omitted the line. Lets snapshot renderers
      distinguish "the workflow declared a byline" from "the artifact
      file actually carried that byline" (HARNESS-003 byline integrity).
    """
    conn.executescript(
        """
        ALTER TABLE sessions ADD COLUMN non_fresh_reason TEXT;
        ALTER TABLE artifacts ADD COLUMN author_line TEXT;
        """
    )


def _apply_v8_process_state_enum(conn: sqlite3.Connection) -> None:
    """RFC 0014 V1: extend ``process_executions.state`` with ``timed_out``
    and ``lost`` for the post-exit completion-guarantee path.

    Uses :func:`rebuild_table` for the CHECK rebuild. The
    ``process_executions`` table is referenced from ``process_supervisors``
    (well, indirectly via ``session_id``) but has no self-referential FK,
    so the rebuild is straightforward. Existing rows preserve their state.
    """
    rebuild_table(
        conn,
        table="process_executions",
        temp_table="process_executions_v8",
        create_temp_sql="""
            CREATE TABLE process_executions_v8 (
              process_id TEXT PRIMARY KEY,
              run_id TEXT NOT NULL REFERENCES runs(run_id),
              job_id TEXT NOT NULL REFERENCES jobs(job_id),
              session_id TEXT NOT NULL REFERENCES sessions(session_id),
              lease_id TEXT NOT NULL REFERENCES leases(lease_id),
              packet_id TEXT NOT NULL REFERENCES work_packets(packet_id),
              adapter TEXT NOT NULL,
              command_json TEXT NOT NULL,
              cwd TEXT NOT NULL,
              scratch_path TEXT NOT NULL,
              stdin_mode TEXT NOT NULL CHECK (stdin_mode IN ('packet','none')),
              stdio_mode TEXT NOT NULL CHECK (stdio_mode IN ('suppressed','inherit')),
              pid INTEGER,
              state TEXT NOT NULL CHECK (state IN (
                'starting','running','exited','failed','timed_out','lost'
              )),
              exit_code INTEGER,
              started_at TEXT NOT NULL,
              ended_at TEXT
            )
        """,
        insert_select_sql="""
            INSERT INTO process_executions_v8
              SELECT process_id, run_id, job_id, session_id, lease_id,
                     packet_id, adapter, command_json, cwd, scratch_path,
                     stdin_mode, stdio_mode, pid, state, exit_code,
                     started_at, ended_at
              FROM process_executions
        """,
    )
    conn.executescript(
        """
        CREATE INDEX IF NOT EXISTS idx_process_executions_run_job
          ON process_executions(run_id, job_id, started_at);
        """
    )


def _apply_v10_verdicts_posture(conn: sqlite3.Connection) -> None:
    """RFC 0018 step 3 (V1.5): add ``posture`` column to ``verdicts`` and
    a covering index for per-posture queries.

    Forward-only ``ALTER TABLE``; existing rows backfill to ``'neutral'``
    via the ``DEFAULT`` clause. Idempotent against a fresh DB whose
    ``schema.py`` already created the column (the V1.5 baseline schema is
    updated alongside the migration so freshly-initialized DBs install
    the column directly, and a re-run of ``apply_migrations`` would
    otherwise duplicate it).
    """
    cols = [row[1] for row in conn.execute("PRAGMA table_info(verdicts)").fetchall()]
    if "posture" not in cols:
        conn.executescript(
            "ALTER TABLE verdicts ADD COLUMN posture TEXT NOT NULL DEFAULT 'neutral';"
        )
    conn.executescript(
        "CREATE INDEX IF NOT EXISTS idx_verdicts_posture ON verdicts(posture);"
    )


def _apply_v9_blockers_payload_json(conn: sqlite3.Connection) -> None:
    """RFC 0014 V1: add ``payload_json`` to the ``blockers`` table so the
    diagnostic envelope from process-adapter completion validation is
    recorded structurally.

    Forward-only ``ALTER TABLE``; existing rows default to ``'{}'``.
    Idempotent against a fresh DB whose ``schema.py`` already created
    the column (the V1 baseline schema is updated alongside the
    migration so freshly-initialized DBs install the column directly,
    and a re-run of ``apply_migrations`` would otherwise duplicate it).
    """
    cols = [row[1] for row in conn.execute("PRAGMA table_info(blockers)").fetchall()]
    if "payload_json" in cols:
        return
    conn.executescript(
        """
        ALTER TABLE blockers ADD COLUMN payload_json TEXT NOT NULL DEFAULT '{}';
        """
    )


def _apply_v11_runs_paused_columns(conn: sqlite3.Connection) -> None:
    """RFC 0024 V4: add ``paused_at`` and ``paused_reason`` columns to
    ``runs`` so the visual builder's Pause/Resume buttons can suspend
    claim-next without changing the run state machine.

    Forward-only ``ALTER TABLE``; existing rows default to ``NULL``.
    Idempotent against a fresh DB whose ``schema.py`` already created
    the columns (the V1 baseline schema is updated alongside the
    migration so freshly-initialized DBs install the columns directly).
    """
    cols = [row[1] for row in conn.execute("PRAGMA table_info(runs)").fetchall()]
    if "paused_at" not in cols:
        conn.execute("ALTER TABLE runs ADD COLUMN paused_at TEXT")
    if "paused_reason" not in cols:
        conn.execute("ALTER TABLE runs ADD COLUMN paused_reason TEXT")


def _apply_v12_lane_attestation_columns(conn: sqlite3.Connection) -> None:
    """RFC 0026: operator labels and PID identity for lane attestation."""
    session_cols = [row[1] for row in conn.execute("PRAGMA table_info(sessions)").fetchall()]
    if "operator_label" not in session_cols:
        conn.execute("ALTER TABLE sessions ADD COLUMN operator_label TEXT")
    supervisor_cols = [
        row[1] for row in conn.execute("PRAGMA table_info(process_supervisors)").fetchall()
    ]
    if "pid_start_time" not in supervisor_cols:
        conn.execute("ALTER TABLE process_supervisors ADD COLUMN pid_start_time TEXT")


def _apply_v13_daemon_supervisor_pointers(conn: sqlite3.Connection) -> None:
    """RFC 0031: repo-local pointers for daemon-owned supervisors.

    The daemon owns the OS child in V2, but repo-local workflow state remains
    authoritative. This table records the relationship between a daemon-owned
    supervisor row and the compatible repo-local supervisor identity used by
    packet delivery, lane attestation, evidence, and doctor checks.
    """
    conn.executescript(
        """
        CREATE TABLE IF NOT EXISTS process_supervisor_pointers (
          supervisor_id TEXT PRIMARY KEY,
          daemon_supervisor_id TEXT NOT NULL,
          run_id TEXT NOT NULL REFERENCES runs(run_id),
          session_id TEXT NOT NULL REFERENCES sessions(session_id),
          pid INTEGER,
          pid_start_time TEXT,
          state TEXT NOT NULL CHECK (state IN ('starting','attached','detached','lost','stopped')),
          updated_at TEXT NOT NULL,
          metadata_json TEXT NOT NULL
        );
        CREATE UNIQUE INDEX IF NOT EXISTS uq_active_daemon_supervisor_pointer_per_session
          ON process_supervisor_pointers(session_id)
          WHERE state IN ('starting','attached','detached');
        CREATE INDEX IF NOT EXISTS idx_process_supervisor_pointers_run
          ON process_supervisor_pointers(run_id, state);
        """
    )


def _apply_v14_cross_repo_run_pointer(conn: sqlite3.Connection) -> None:
    """RFC 0032: repo-local back-reference to daemon-owned cross-repo runs."""
    run_cols = [row[1] for row in conn.execute("PRAGMA table_info(runs)").fetchall()]
    if "cross_repo_run_id" not in run_cols:
        conn.execute("ALTER TABLE runs ADD COLUMN cross_repo_run_id TEXT")
    conn.execute(
        """
        CREATE INDEX IF NOT EXISTS idx_runs_cross_repo_run_id
          ON runs(cross_repo_run_id)
          WHERE cross_repo_run_id IS NOT NULL
        """
    )


def _apply_v15_attestation_override_rationale(conn: sqlite3.Connection) -> None:
    """RFC 0046 V1 F-schema: capture operator override rationale when
    publish-artifact runs with --allow-no-process-execution.

    Empty-string vs NULL contract (per design review F-dx-1): NULL means
    no override applied; non-empty string means the operator passed
    --allow-no-process-execution + a non-empty --override-rationale.
    Empty string is intentionally not allowed by the CLI surface
    (publish-artifact refuses with exit code 2).
    """
    art_cols = [row[1] for row in conn.execute("PRAGMA table_info(artifacts)").fetchall()]
    if "attestation_override_rationale" not in art_cols:
        conn.execute(
            "ALTER TABLE artifacts ADD COLUMN attestation_override_rationale TEXT"
        )


def _apply_v16_decision_propagation(conn: sqlite3.Connection) -> None:
    """RFC 0047 V1: decision-record propagation.

    Adds the ``compromised`` value to the ``runs.state`` CHECK
    constraint and the ``superseded_by_decision_id`` /
    ``superseded_at`` columns to ``verdicts``. SQLite cannot alter a
    CHECK constraint in place; the runs table is rebuilt with the new
    constraint and existing rows are copied across.
    """
    verdict_cols = [
        row[1] for row in conn.execute("PRAGMA table_info(verdicts)").fetchall()
    ]
    if "superseded_by_decision_id" not in verdict_cols:
        conn.execute(
            "ALTER TABLE verdicts ADD COLUMN superseded_by_decision_id TEXT"
        )
    if "superseded_at" not in verdict_cols:
        conn.execute("ALTER TABLE verdicts ADD COLUMN superseded_at TEXT")

    # Rebuild runs with the extended state CHECK that admits 'compromised'.
    runs_sql = conn.execute(
        "SELECT sql FROM sqlite_master WHERE type='table' AND name='runs'"
    ).fetchone()
    if runs_sql is None or "compromised" in str(runs_sql[0]):
        return
    rebuild_table(
        conn,
        table="runs",
        temp_table="runs_new",
        create_temp_sql="""
        CREATE TABLE runs_new (
          run_id TEXT PRIMARY KEY,
          workflow_snapshot_id TEXT NOT NULL REFERENCES workflow_snapshots(workflow_snapshot_id),
          repo_root TEXT NOT NULL,
          state TEXT NOT NULL CHECK (state IN (
            'needs_branch_confirmation','ready','running','blocked',
            'completed','failed','canceled','compromised'
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
        )
        """,
        insert_select_sql="INSERT INTO runs_new SELECT * FROM runs",
    )
    conn.executescript(
        """
        CREATE INDEX IF NOT EXISTS idx_runs_cross_repo_run_id
          ON runs(cross_repo_run_id)
          WHERE cross_repo_run_id IS NOT NULL;
        """
    )


MIGRATIONS: list[Migration] = sorted(
    [
        Migration(version=1, label="v1 baseline schema", apply=_apply_v1),
        Migration(version=2, label="job_worktrees table", apply=_apply_v2_job_worktrees),
        Migration(version=3, label="work_packets fresh-session index", apply=_apply_v3_work_packets_index),
        Migration(version=4, label="process_supervisors table", apply=_apply_v4_process_supervisors),
        Migration(version=5, label="open artifact_kind to python validation", apply=_apply_v5_open_artifact_kinds),
        Migration(
            version=6,
            label="non_fresh_reason and author_line columns",
            apply=_apply_v6_independence_and_author_line,
        ),
        Migration(
            version=7,
            label="sessions closed state and close columns",
            apply=_apply_v7_session_close,
        ),
        Migration(
            version=8,
            label="process_executions state enum (timed_out, lost)",
            apply=_apply_v8_process_state_enum,
        ),
        Migration(
            version=9,
            label="blockers payload_json column",
            apply=_apply_v9_blockers_payload_json,
        ),
        Migration(
            version=10,
            label="verdicts.posture column + index",
            apply=_apply_v10_verdicts_posture,
        ),
        Migration(
            version=11,
            label="runs.paused_at + runs.paused_reason columns",
            apply=_apply_v11_runs_paused_columns,
        ),
        Migration(
            version=12,
            label="lane attestation operator_label + pid_start_time columns",
            apply=_apply_v12_lane_attestation_columns,
        ),
        Migration(
            version=13,
            label="daemon supervisor pointers",
            apply=_apply_v13_daemon_supervisor_pointers,
        ),
        Migration(
            version=14,
            label="cross-repo run pointer",
            apply=_apply_v14_cross_repo_run_pointer,
        ),
        Migration(
            version=15,
            label="attestation override rationale (RFC 0046 V1)",
            apply=_apply_v15_attestation_override_rationale,
        ),
        Migration(
            version=16,
            label="decision propagation + compromised run state (RFC 0047 V1)",
            apply=_apply_v16_decision_propagation,
        ),
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

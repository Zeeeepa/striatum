# ruff: noqa
"""GH #27 / RFC 0072 follow-up — migration 0010 column-aware artifacts trigger.

Migration 0010 (``0010_artifact_blob_update_trigger.sql``) replaces the
generic ``artifacts_no_update`` trigger with a column-aware variant
that allows UPDATE statements touching only ``blob_key``,
``blob_sha256``, and ``blob_content_type`` while still refusing every
other artifact-column update with the existing ``P0001`` exception.
It also grants column-level ``UPDATE`` on those three columns to
``striatumd_rw`` so the runtime role can drive
``artifact.backfill_blob`` without a table-wide UPDATE privilege.

These tests pin the trigger and grant behavior in place against an
ephemeral PostgreSQL database created by the multi-repo harness.
"""

from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION, read_schema_version


pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def _seed_artifact(conn: Any, tmp_path: Path) -> None:
    now = "2026-05-19T00:00:00Z"
    repo_root = tmp_path / "repo"
    repo_root.mkdir(exist_ok=True)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path,
              display_name, registered_at, last_schema_version, state
            ) VALUES ('repo_1', 'repo_1', %s, %s, 'repo_1', %s, 10, 'active')
            """,
            (str(repo_root), str(repo_root / ".striatum" / "state.sqlite3"), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots(
              repository_id, workflow_snapshot_id, workflow_id, content_sha256,
              workflow_json, loaded_at
            ) VALUES ('repo_1', 'snap_1', 'wf', 'sha', '{}'::jsonb, %s)
            """,
            (now,),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs(
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              created_at
            ) VALUES ('repo_1', 'run_1', 'snap_1', %s, 'running', %s)
            """,
            (str(repo_root), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions(
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            ) VALUES ('repo_1', 'sess_1', 'run_1', 'implementer', 'claude_code',
                      'implementer-claude_code-1', 1,
                      '["read","write"]'::jsonb, 'active', %s, %s)
            """,
            (now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs(
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at
            ) VALUES ('repo_1', 'job_1', 'run_1', 'fix', 'Fix', 'build',
                      'implementer',
                      '{"lane_id":"claude_code"}'::jsonb, '{}'::jsonb,
                      'blocked',
                      '{"mode":"repo_write","allowed_paths":["src/"],"forbidden_paths":[]}'::jsonb,
                      '[]'::jsonb, 'fix-1', %s)
            """,
            (now,),
        )
        cur.execute(
            """
            INSERT INTO striatumd.artifacts(
              repository_id, artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
              created_at
            ) VALUES ('repo_1', 'art_1', 'run_1', 'job_1', 'sess_1',
                      'finding', 'finding',
                      'docs/findings/x.md',
                      'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
                      42, 'create', %s)
            """,
            (now,),
        )
    conn.commit()


def test_migration_0010_lands_and_bumps_latest_version(pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        assert read_schema_version(conn) == LATEST_DAEMON_DB_VERSION
        assert LATEST_DAEMON_DB_VERSION >= 10
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT label
                FROM striatumd.schema_migrations
                WHERE version = 10
                """
            )
            row = cur.fetchone()
        assert row is not None
        assert "GH #27" in str(row[0])
    finally:
        conn.close()


def test_artifacts_trigger_uses_column_aware_function(pg_url: str) -> None:
    """The `artifacts_no_update` trigger must call the new column-aware
    function, not the generic refuse_repo_append_only_change shared with
    events."""
    conn = connect(pg_url)
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT action_statement
                FROM information_schema.triggers
                WHERE event_object_schema = 'striatumd'
                  AND event_object_table = 'artifacts'
                  AND trigger_name = 'artifacts_no_update'
                """
            )
            row = cur.fetchone()
        assert row is not None
        statement = str(row[0])
        assert "allow_artifact_blob_reference_update" in statement
        assert "refuse_repo_append_only_change" not in statement
    finally:
        conn.close()


def test_blob_only_update_succeeds(pg_url: str, tmp_path: Path) -> None:
    conn = connect(pg_url)
    try:
        _seed_artifact(conn, tmp_path)
        digest = "b" * 64
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.artifacts
                   SET blob_key = 'runs/run_1/jobs/job_1/artifacts/finding',
                       blob_sha256 = %s,
                       blob_content_type = 'text/markdown'
                 WHERE repository_id = 'repo_1' AND artifact_id = 'art_1'
                """,
                (digest,),
            )
        conn.commit()
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT blob_key, blob_sha256, blob_content_type
                FROM striatumd.artifacts
                WHERE repository_id = 'repo_1' AND artifact_id = 'art_1'
                """
            )
            row = cur.fetchone()
        assert row is not None
        assert row[0] == "runs/run_1/jobs/job_1/artifacts/finding"
        assert row[1] == digest
        assert row[2] == "text/markdown"
    finally:
        conn.close()


def test_blob_columns_can_round_trip_to_null(pg_url: str, tmp_path: Path) -> None:
    """value-to-NULL transitions on blob columns are also allowed."""
    conn = connect(pg_url)
    try:
        _seed_artifact(conn, tmp_path)
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.artifacts
                   SET blob_key = 'k', blob_sha256 = %s, blob_content_type = 'text/markdown'
                 WHERE repository_id = 'repo_1' AND artifact_id = 'art_1'
                """,
                ("c" * 64,),
            )
            cur.execute(
                """
                UPDATE striatumd.artifacts
                   SET blob_key = NULL, blob_sha256 = NULL, blob_content_type = NULL
                 WHERE repository_id = 'repo_1' AND artifact_id = 'art_1'
                """
            )
        conn.commit()
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT blob_key, blob_sha256, blob_content_type
                FROM striatumd.artifacts
                WHERE repository_id = 'repo_1' AND artifact_id = 'art_1'
                """
            )
            row = cur.fetchone()
        assert row is not None
        assert row == (None, None, None)
    finally:
        conn.close()


@pytest.mark.parametrize(
    "column,value",
    [
        ("content_sha256", "d" * 64),
        ("repo_path", "docs/findings/y.md"),
        ("artifact_kind", "synthesis"),
        ("logical_name", "renamed"),
        ("size_bytes", 99),
    ],
)
def test_non_blob_update_still_refused_with_p0001(
    pg_url: str, tmp_path: Path, column: str, value: object
) -> None:
    conn = connect(pg_url)
    try:
        _seed_artifact(conn, tmp_path)
        with conn.cursor() as cur:
            with pytest.raises(Exception) as exc_info:
                cur.execute(
                    f"""
                    UPDATE striatumd.artifacts
                       SET {column} = %s
                     WHERE repository_id = 'repo_1' AND artifact_id = 'art_1'
                    """,
                    (value,),
                )
        sqlstate = getattr(exc_info.value, "sqlstate", None) or getattr(
            getattr(exc_info.value, "diag", None), "sqlstate", None
        )
        assert sqlstate == "P0001", (
            f"expected raise_exception (P0001) when updating {column!r}, got {sqlstate!r}"
        )
        conn.rollback()
    finally:
        conn.close()


def test_artifact_delete_still_refused(pg_url: str, tmp_path: Path) -> None:
    """The migration must not weaken the delete trigger — `artifacts_no_delete`
    still calls the generic refuse function."""
    conn = connect(pg_url)
    try:
        _seed_artifact(conn, tmp_path)
        with conn.cursor() as cur:
            with pytest.raises(Exception) as exc_info:
                cur.execute(
                    "DELETE FROM striatumd.artifacts WHERE artifact_id = 'art_1'"
                )
        sqlstate = getattr(exc_info.value, "sqlstate", None) or getattr(
            getattr(exc_info.value, "diag", None), "sqlstate", None
        )
        assert sqlstate == "P0001"
        conn.rollback()
    finally:
        conn.close()


def test_events_update_still_refused(pg_url: str, tmp_path: Path) -> None:
    """The migration must leave the events update trigger alone."""
    conn = connect(pg_url)
    try:
        _seed_artifact(conn, tmp_path)
        with conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.events(
                  repository_id, run_id, event_type, payload_json, created_at,
                  row_hash
                ) VALUES ('repo_1', 'run_1', 'test', '{}'::jsonb, now(),
                          'hash_1')
                """
            )
        conn.commit()
        with conn.cursor() as cur:
            with pytest.raises(Exception) as exc_info:
                cur.execute(
                    "UPDATE striatumd.events SET payload_json = '{}'::jsonb"
                )
        sqlstate = getattr(exc_info.value, "sqlstate", None) or getattr(
            getattr(exc_info.value, "diag", None), "sqlstate", None
        )
        assert sqlstate == "P0001"
        conn.rollback()
    finally:
        conn.close()


def _provision_striatumd_rw(conn: Any) -> bool:
    """Create the role then re-apply the schema-level GRANT/REVOKE blocks
    from migrations 0005 and 0010. Re-applying 0010's DO-block exercises
    the migration's column-level grant code path against a now-existing
    role (mirroring the operator's create-role-then-doctor sequence in
    docs/POSTGRES_TRANSITION.md)."""
    from striatum.daemon_pg.migrations import MIGRATIONS

    migration_0010_sql = next(m for m in MIGRATIONS if m.version == 10).sql
    with conn.cursor() as cur:
        try:
            cur.execute(
                "DO $$ BEGIN "
                "  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN "
                "    CREATE ROLE striatumd_rw WITH LOGIN PASSWORD 'rwpass'; "
                "  END IF; "
                "END $$;"
            )
            cur.execute("SELECT current_database()")
            row = cur.fetchone()
            assert row is not None
            database = str(row[0])
            cur.execute(
                f'GRANT CONNECT ON DATABASE "{database}" TO striatumd_rw'
            )
            cur.execute("GRANT USAGE ON SCHEMA striatumd TO striatumd_rw")
            cur.execute(
                "GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES "
                "IN SCHEMA striatumd TO striatumd_rw"
            )
            cur.execute(
                "REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw"
            )
            cur.execute(migration_0010_sql)
        except Exception:
            return False
    conn.commit()
    return True


def test_striatumd_rw_has_column_level_update_on_blob_columns_only(
    pg_url: str,
) -> None:
    """information_schema.column_privileges must list UPDATE for the three
    blob columns and only those columns; information_schema.table_privileges
    must still not list table-wide UPDATE."""
    conn = connect(pg_url)
    try:
        if not _provision_striatumd_rw(conn):
            pytest.skip("test runner lacks CREATEROLE on the ephemeral database")
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT privilege_type
                FROM information_schema.table_privileges
                WHERE grantee = 'striatumd_rw'
                  AND table_schema = 'striatumd'
                  AND table_name = 'artifacts'
                """
            )
            table_priv = {str(row[0]) for row in cur.fetchall()}
            cur.execute(
                """
                SELECT column_name
                FROM information_schema.column_privileges
                WHERE grantee = 'striatumd_rw'
                  AND table_schema = 'striatumd'
                  AND table_name = 'artifacts'
                  AND privilege_type = 'UPDATE'
                ORDER BY column_name
                """
            )
            update_columns = {str(row[0]) for row in cur.fetchall()}
    finally:
        conn.close()

    assert "UPDATE" not in table_priv, (
        "striatumd_rw must NOT have table-wide UPDATE on artifacts"
    )
    assert update_columns == {"blob_key", "blob_sha256", "blob_content_type"}, (
        f"unexpected column-level UPDATE grant set: {update_columns}"
    )

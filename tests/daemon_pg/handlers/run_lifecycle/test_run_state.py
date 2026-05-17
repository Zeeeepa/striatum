from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_pg.handlers.run_lifecycle.run_state import (
    handle_run_cancel,
    handle_run_pause,
    handle_run_resume,
    handle_run_retry_job,
    handle_run_start,
)
from striatum.daemon_rpc.capability import RpcAuthContext

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_run_start_enqueues_roots_and_preserves_repo_scope(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        _seed_run_with_jobs(conn, tmp_path, repository_id="repo_a", run_state="ready")
        _seed_run_with_jobs(conn, tmp_path, repository_id="repo_b", run_state="ready")
        conn.commit()

        result = handle_run_start(_ctx(conn, tmp_path, "repo_a"), {"run_id": "run_1"})

        assert result == {"run_id": "run_1", "state": "running"}
        assert _one(
            conn,
            "SELECT state, started_at FROM striatumd.runs WHERE repository_id = %s AND run_id = 'run_1'",
            ("repo_a",),
        )["state"] == "running"
        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.queue_messages "
            "WHERE repository_id = %s AND job_id = 'job_root' AND state = 'pending'",
            ("repo_a",),
        ) == {"count": 1}
        assert _one(
            conn,
            "SELECT state FROM striatumd.jobs WHERE repository_id = %s AND job_id = 'job_child'",
            ("repo_a",),
        ) == {"state": "blocked"}
        assert _one(
            conn,
            "SELECT state FROM striatumd.runs WHERE repository_id = %s AND run_id = 'run_1'",
            ("repo_b",),
        ) == {"state": "ready"}
        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.queue_messages WHERE repository_id = %s",
            ("repo_b",),
        ) == {"count": 0}
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "queue.message_enqueued",
            "run.started",
        ]
    finally:
        conn.close()


def test_pause_and_resume_are_idempotent(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        _seed_run_with_jobs(conn, tmp_path, repository_id="repo_a", run_state="running")
        conn.commit()

        paused = handle_run_pause(
            _ctx(conn, tmp_path, "repo_a"),
            {"run_id": "run_1", "reason": "maintenance"},
        )
        paused_again = handle_run_pause(_ctx(conn, tmp_path, "repo_a"), {"run_id": "run_1"})
        resumed = handle_run_resume(_ctx(conn, tmp_path, "repo_a"), {"run_id": "run_1"})
        resumed_again = handle_run_resume(_ctx(conn, tmp_path, "repo_a"), {"run_id": "run_1"})

        assert paused["status"] == "paused"
        assert paused_again["status"] == "already_paused"
        assert resumed == {"run_id": "run_1", "state": "running", "paused_at": None, "status": "resumed"}
        assert resumed_again == {
            "run_id": "run_1",
            "state": "running",
            "paused_at": None,
            "status": "not_paused",
        }
        assert _one(
            conn,
            "SELECT paused_at, paused_reason FROM striatumd.runs WHERE repository_id = %s AND run_id = 'run_1'",
            ("repo_a",),
        ) == {"paused_at": None, "paused_reason": None}
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "run.paused",
            "run.resumed",
        ]
    finally:
        conn.close()


def test_run_cancel_releases_leases_cancels_jobs_and_closes_sessions(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_run_with_jobs(conn, tmp_path, repository_id="repo_a", run_state="running")
        _seed_session_and_lease(conn, repository_id="repo_a")
        conn.commit()

        result = handle_run_cancel(
            _ctx(conn, tmp_path, "repo_a"),
            {"run_id": "run_1", "reason": "operator request"},
        )

        assert result == {"run_id": "run_1", "state": "canceled", "status": "canceled"}
        assert _one(
            conn,
            "SELECT state, stop_reason FROM striatumd.runs WHERE repository_id = %s AND run_id = 'run_1'",
            ("repo_a",),
        ) == {"state": "canceled", "stop_reason": "operator request"}
        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.jobs "
            "WHERE repository_id = %s AND state = 'canceled'",
            ("repo_a",),
        ) == {"count": 2}
        assert _one(
            conn,
            "SELECT state, release_reason FROM striatumd.leases WHERE repository_id = %s AND lease_id = 'lease_1'",
            ("repo_a",),
        ) == {"state": "released", "release_reason": "run_canceled"}
        assert _one(
            conn,
            "SELECT state, close_reason FROM striatumd.sessions WHERE repository_id = %s AND session_id = 'sess_1'",
            ("repo_a",),
        ) == {"state": "closed", "close_reason": "run_canceled"}
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "run.canceled",
            "session.closed",
        ]
    finally:
        conn.close()


def test_retry_job_revives_terminal_run_and_reenqueues_work(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        _seed_run_with_jobs(conn, tmp_path, repository_id="repo_a", run_state="failed")
        _mark_job_failed_with_blocker(conn, repository_id="repo_a")
        conn.commit()

        result = handle_run_retry_job(
            _ctx(conn, tmp_path, "repo_a"),
            {"run_id": "run_1", "job_id": "job_root"},
        )

        assert result == {
            "run_id": "run_1",
            "job_id": "job_root",
            "previous_state": "failed",
            "new_state": "queued",
            "run_revived": True,
        }
        assert _one(
            conn,
            "SELECT state, completed_at, stop_reason FROM striatumd.runs "
            "WHERE repository_id = %s AND run_id = 'run_1'",
            ("repo_a",),
        ) == {"state": "running", "completed_at": None, "stop_reason": None}
        assert _one(
            conn,
            "SELECT state, attempt, current_lease_id FROM striatumd.jobs "
            "WHERE repository_id = %s AND job_id = 'job_root'",
            ("repo_a",),
        ) == {"state": "queued", "attempt": 2, "current_lease_id": None}
        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.queue_messages "
            "WHERE repository_id = %s AND job_id = 'job_root' AND state = 'pending'",
            ("repo_a",),
        ) == {"count": 1}
        assert _one(
            conn,
            "SELECT state FROM striatumd.blockers WHERE repository_id = %s AND blocker_id = 'blk_1'",
            ("repo_a",),
        ) == {"state": "canceled"}
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "queue.message_enqueued",
            "run.revived",
            "job.retried",
        ]
    finally:
        conn.close()


def test_run_state_handlers_are_registered() -> None:
    assert resolve_pg_handler("run.start") is handle_run_start
    assert resolve_pg_handler("run.pause") is handle_run_pause
    assert resolve_pg_handler("run.resume") is handle_run_resume
    assert resolve_pg_handler("run.cancel") is handle_run_cancel
    assert resolve_pg_handler("run.retry_job") is handle_run_retry_job


def _ctx(conn: Any, tmp_path: Path, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=tmp_path / repository_id,
        auth=RpcAuthContext("client", "token", repository_id, "admin", "allowed"),
    )


def _seed_run_with_jobs(conn: Any, tmp_path: Path, *, repository_id: str, run_state: str) -> None:
    now = "2026-05-14T00:00:00Z"
    repo_root = tmp_path / repository_id
    repo_root.mkdir()
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"author": {}, "reviewer": {}},
        "lanes": {"local": {}},
        "jobs": [{"id": "root", "type": "draft"}, {"id": "child", "type": "review"}],
    }
    completed_at = now if run_state in {"completed", "failed", "canceled"} else None
    stop_reason = "job_failed" if run_state == "failed" else None
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories (
              repository_id, repo_identity, repo_root, state_db_path, display_name,
              registered_at, last_schema_version, state
            )
            VALUES (%s, %s, %s, %s, %s, %s, 5, 'active')
            """,
            (repository_id, repository_id, str(repo_root), str(repo_root / ".striatum"), repository_id, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots (
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, 'snap_1', 'wf', '1', 'sha', %s, %s)
            """,
            (repository_id, Jsonb(workflow), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs (
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              created_at, started_at, completed_at, stop_reason
            )
            VALUES (%s, 'run_1', 'snap_1', %s, %s, %s, %s, %s, %s)
            """,
            (
                repository_id,
                str(repo_root),
                run_state,
                now,
                now if run_state in {"running", "failed", "canceled"} else None,
                completed_at,
                stop_reason,
            ),
        )
        for job_id, workflow_job_id, role_id in (
            ("job_root", "root", "author"),
            ("job_child", "child", "reviewer"),
        ):
            cur.execute(
                """
                INSERT INTO striatumd.jobs (
                  repository_id, job_id, run_id, workflow_job_id, title, job_type,
                  role_id, lane_selector_json, capability_requirements_json, state,
                  write_scope_json, expected_artifacts_json, idempotency_key, created_at
                )
                VALUES (%s, %s, 'run_1', %s, %s, 'draft', %s,
                        %s, %s, 'blocked', %s, %s, %s, %s)
                """,
                (
                    repository_id,
                    job_id,
                    workflow_job_id,
                    workflow_job_id.title(),
                    role_id,
                    Jsonb({"lane_id": "local"}),
                    Jsonb({}),
                    Jsonb({"repo_write": False}),
                    Jsonb([]),
                    f"{workflow_job_id}:1",
                    now,
                ),
            )
        cur.execute(
            """
            INSERT INTO striatumd.job_dependencies (
              repository_id, job_id, depends_on_job_id, gate_json
            )
            VALUES (%s, 'job_child', 'job_root', %s)
            """,
            (repository_id, Jsonb({})),
        )


def _seed_session_and_lease(conn: Any, *, repository_id: str) -> None:
    now = "2026-05-14T00:00:00Z"
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.sessions (
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, 'sess_1', 'run_1', 'author', 'local', 'author-local-1',
                    1, %s, 'active', %s, %s)
            """,
            (repository_id, Jsonb(["admin"]), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases (
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at
            )
            VALUES (%s, 'lease_1', 'run_1', 'job', 'job_root', 'sess_1',
                    'active', %s, '2099-01-01T00:00:00Z')
            """,
            (repository_id, now),
        )
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'running', started_at = %s, current_lease_id = 'lease_1'
            WHERE repository_id = %s AND job_id = 'job_root'
            """,
            (now, repository_id),
        )


def _mark_job_failed_with_blocker(conn: Any, *, repository_id: str) -> None:
    now = "2026-05-14T00:00:00Z"
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'failed', completed_at = %s
            WHERE repository_id = %s AND job_id = 'job_root'
            """,
            (now, repository_id),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages (
              repository_id, message_id, run_id, job_id, kind, state, target_role_id,
              payload_json, created_at, updated_at
            )
            VALUES (%s, 'msg_old', 'run_1', 'job_root', 'work', 'pending', 'author',
                    %s, %s, %s)
            """,
            (repository_id, Jsonb({}), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.blockers (
              repository_id, blocker_id, run_id, job_id, severity, blocker_kind,
              description, state, created_at
            )
            VALUES (%s, 'blk_1', 'run_1', 'job_root', 'blocked', 'test',
                    'blocked', 'open', %s)
            """,
            (repository_id, now),
        )


def _events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            "SELECT * FROM striatumd.events WHERE repository_id = %s ORDER BY event_id",
            (repository_id,),
        )
        return [dict(row) for row in cur.fetchall()]


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)

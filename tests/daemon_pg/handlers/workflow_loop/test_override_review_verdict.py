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
from striatum.daemon_pg.handlers.workflow_loop.override_review_verdict import handle
from striatum.daemon_rpc.capability import RpcAuthContext


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_override_appends_accepting_verdict_and_resolves_waiting_human_review(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_waiting_review(conn, tmp_path, repository_id="repo_a")
        _seed_waiting_review(conn, tmp_path, repository_id="repo_b")
        conn.commit()

        result = handle(
            _ctx(conn, tmp_path, repository_id="repo_a"),
            {
                "session_id": "sess_override",
                "job_id": "job_review",
                "verdict": "accept_with_findings",
                "rationale": "operator accepts with known findings",
            },
        )

        assert result["status"] == "overridden"
        assert result["previous_verdict"] == "needs_revision"
        assert result["resolved_blockers"] == 1
        assert _one(
            conn,
            "SELECT state FROM striatumd.jobs WHERE repository_id = %s AND job_id = %s",
            ("repo_a", "job_review"),
        ) == {"state": "completed"}
        assert _one(
            conn,
            "SELECT state FROM striatumd.blockers WHERE repository_id = %s AND blocker_id = %s",
            ("repo_a", "blk_1"),
        ) == {"state": "resolved"}
        verdicts = _all(
            conn,
            "SELECT verdict, session_id FROM striatumd.verdicts "
            "WHERE repository_id = %s AND job_id = %s ORDER BY created_at, verdict_id",
            ("repo_a", "job_review"),
        )
        assert verdicts == [
            {"verdict": "needs_revision", "session_id": "sess_1"},
            {"verdict": "accept_with_findings", "session_id": "sess_override"},
        ]
        assert _one(
            conn,
            "SELECT state FROM striatumd.jobs WHERE repository_id = %s AND job_id = %s",
            ("repo_b", "job_review"),
        ) == {"state": "waiting_human"}
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "verdict.overridden",
            "run.completed",
            "session.closed",
            "session.closed",
        ]
    finally:
        conn.close()


def _ctx(conn: Any, tmp_path: Path, *, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=tmp_path / repository_id,
        auth=RpcAuthContext("client", "token", repository_id, "admin", "allowed"),
    )


def _seed_waiting_review(conn: Any, tmp_path: Path, *, repository_id: str) -> None:
    now = "2026-05-14T00:00:00Z"
    repo_root = tmp_path / repository_id
    repo_root.mkdir()
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"reviewer": {}},
        "lanes": {"local": {}},
        "jobs": [{"id": "review", "type": "review"}],
        "cycles": [],
    }
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
              created_at, started_at
            )
            VALUES (%s, 'run_1', 'snap_1', %s, 'running', %s, %s)
            """,
            (repository_id, str(repo_root), now, now),
        )
        for session_id, slug in (
            ("sess_1", "reviewer-local-1"),
            ("sess_override", "reviewer-local-2"),
        ):
            ordinal = 1 if session_id == "sess_1" else 2
            cur.execute(
                """
                INSERT INTO striatumd.sessions (
                  repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
                  capabilities_json, state, registered_at, last_heartbeat_at
                )
                VALUES (%s, %s, 'run_1', 'reviewer', 'local', %s, %s,
                        %s, 'active', %s, %s)
                """,
                (repository_id, session_id, slug, ordinal, Jsonb(["review"]), now, now),
            )
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at, current_message_id
            )
            VALUES (%s, 'job_review', 'run_1', 'review', 'Review', 'review',
                    'reviewer', %s, %s, 'waiting_human', %s, %s, 'review-1',
                    %s, 'msg_1')
            """,
            (
                repository_id,
                Jsonb({"lane_id": "local"}),
                Jsonb({}),
                Jsonb({"repo_write": False}),
                Jsonb([]),
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages (
              repository_id, message_id, run_id, job_id, kind, state,
              target_role_id, payload_json, created_at, updated_at
            )
            VALUES (%s, 'msg_1', 'run_1', 'job_review', 'work', 'blocked',
                    'reviewer', %s, %s, %s)
            """,
            (repository_id, Jsonb({}), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.verdicts (
              repository_id, verdict_id, run_id, job_id, session_id, verdict,
              rationale, created_at, posture
            )
            VALUES (%s, 'verdict_1', 'run_1', 'job_review', 'sess_1',
                    'needs_revision', 'needs changes', %s, 'neutral')
            """,
            (repository_id, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.blockers (
              repository_id, blocker_id, run_id, job_id, session_id, severity,
              blocker_kind, description, state, created_at
            )
            VALUES (%s, 'blk_1', 'run_1', 'job_review', 'sess_1',
                    'human_checkpoint', 'revision_routing',
                    'needs_revision verdict has no matching workflow cycle',
                    'open', %s)
            """,
            (repository_id, now),
        )


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)


def _all(conn: Any, sql: str, args: tuple[object, ...]) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        return [dict(row) for row in cur.fetchall()]


def _events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            "SELECT * FROM striatumd.events WHERE repository_id = %s ORDER BY event_id",
            (repository_id,),
        )
        return [dict(row) for row in cur.fetchall()]

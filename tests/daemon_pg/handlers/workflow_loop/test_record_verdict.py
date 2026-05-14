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
from striatum.daemon_pg.handlers.workflow_loop.record_verdict import handle
from striatum.daemon_rpc.capability import RpcAuthContext


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_record_accept_completes_review_enqueues_downstream_and_scopes_repository(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_running_review(conn, tmp_path, repository_id="repo_a", with_downstream=True)
        _seed_running_review(conn, tmp_path, repository_id="repo_b", with_downstream=False)
        conn.commit()

        result = handle(
            _ctx(conn, tmp_path, repository_id="repo_a"),
            {
                "session_id": "sess_1",
                "job_id": "job_review",
                "lease_id": "lease_1",
                "verdict": "accept_with_findings",
                "rationale": "looks good with nits",
            },
        )

        assert result["status"] == "completed"
        assert result["verdict"] == "accept_with_findings"
        assert _one(
            conn,
            "SELECT state, current_lease_id FROM striatumd.jobs "
            "WHERE repository_id = %s AND job_id = %s",
            ("repo_a", "job_review"),
        ) == {"state": "completed", "current_lease_id": None}
        assert _one(
            conn,
            "SELECT state, release_reason FROM striatumd.leases "
            "WHERE repository_id = %s AND lease_id = %s",
            ("repo_a", "lease_1"),
        ) == {"state": "released", "release_reason": "verdict"}
        assert _one(
            conn,
            "SELECT state FROM striatumd.jobs WHERE repository_id = %s AND job_id = %s",
            ("repo_a", "job_downstream"),
        ) == {"state": "queued"}
        assert _one(
            conn,
            "SELECT state FROM striatumd.jobs WHERE repository_id = %s AND job_id = %s",
            ("repo_b", "job_review"),
        ) == {"state": "running"}
        assert _one(
            conn,
            "SELECT verdict, rationale, posture FROM striatumd.verdicts "
            "WHERE repository_id = %s AND job_id = %s",
            ("repo_a", "job_review"),
        ) == {
            "verdict": "accept_with_findings",
            "rationale": "looks good with nits",
            "posture": "security",
        }
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "verdict.recorded",
            "job.completed",
            "queue.message_enqueued",
        ]
    finally:
        conn.close()


def _ctx(conn: Any, tmp_path: Path, *, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=tmp_path / repository_id,
        auth=RpcAuthContext("client", "token", repository_id, "review", "allowed"),
    )


def _seed_running_review(
    conn: Any, tmp_path: Path, *, repository_id: str, with_downstream: bool
) -> None:
    now = "2026-05-14T00:00:00Z"
    repo_root = tmp_path / repository_id
    repo_root.mkdir()
    workflow_jobs: list[dict[str, object]] = [
        {"id": "review", "type": "review", "review_posture": "security"}
    ]
    if with_downstream:
        workflow_jobs.append({"id": "build", "type": "build"})
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"reviewer": {}, "author": {}},
        "lanes": {"local": {}},
        "jobs": workflow_jobs,
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
        cur.execute(
            """
            INSERT INTO striatumd.sessions (
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, 'sess_1', 'run_1', 'reviewer', 'local',
                    'reviewer-local-1', 1, %s, 'active', %s, %s)
            """,
            (repository_id, Jsonb(["review"]), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at, started_at, current_message_id, current_lease_id
            )
            VALUES (%s, 'job_review', 'run_1', 'review', 'Review', 'review',
                    'reviewer', %s, %s, 'running', %s, %s, 'review-1',
                    %s, %s, 'msg_1', 'lease_1')
            """,
            (
                repository_id,
                Jsonb({"lane_id": "local"}),
                Jsonb({}),
                Jsonb({"repo_write": False}),
                Jsonb([]),
                now,
                now,
            ),
        )
        if with_downstream:
            cur.execute(
                """
                INSERT INTO striatumd.jobs (
                  repository_id, job_id, run_id, workflow_job_id, title, job_type,
                  role_id, lane_selector_json, capability_requirements_json,
                  state, write_scope_json, expected_artifacts_json,
                  idempotency_key, created_at
                )
                VALUES (%s, 'job_downstream', 'run_1', 'build', 'Build',
                        'build', 'author', %s, %s, 'blocked', %s, %s,
                        'build-1', %s)
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
                INSERT INTO striatumd.job_dependencies (
                  repository_id, job_id, depends_on_job_id, gate_json
                )
                VALUES (%s, 'job_downstream', 'job_review', %s)
                """,
                (repository_id, Jsonb({"requires_verdict": ["accept_with_findings"]})),
            )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages (
              repository_id, message_id, run_id, job_id, kind, state,
              target_role_id, payload_json, created_at, updated_at, claimed_at,
              acked_at, current_lease_id
            )
            VALUES (%s, 'msg_1', 'run_1', 'job_review', 'work', 'acked',
                    'reviewer', %s, %s, %s, %s, %s, 'lease_1')
            """,
            (repository_id, Jsonb({}), now, now, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases (
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at
            )
            VALUES (%s, 'lease_1', 'run_1', 'job', 'job_review', 'sess_1',
                    'active', %s, '2099-01-01T00:00:00Z')
            """,
            (repository_id, now),
        )


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)


def _events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            "SELECT * FROM striatumd.events WHERE repository_id = %s ORDER BY event_id",
            (repository_id,),
        )
        return [dict(row) for row in cur.fetchall()]

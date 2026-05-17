from __future__ import annotations

from collections.abc import Iterator, Mapping
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_pg.handlers.run_lifecycle.checkpoint_resolve import handle
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.errors import InvalidTransitionError

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_checkpoint_continue_restores_existing_message_and_links_decision(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_checkpoint_run(
            conn,
            tmp_path,
            repository_id="repo_a",
            message_id="msg_1",
            with_downstream=True,
            with_decision=True,
        )
        conn.commit()

        result = handle(
            _ctx(conn, tmp_path, "repo_a"),
            {
                "blocker_id": "blk_1",
                "action": "continue",
                "decision_id": "decision-001",
            },
        )

        assert result == {
            "status": "resolved",
            "blocker_id": "blk_1",
            "job_id": "job_1",
            "action": "continue",
            "decision_id": "decision-001",
            "decision_artifact_id": "artifact_decision",
            "run_state": "running",
            "downstream_jobs": [
                {"job_id": "job_2", "workflow_job_id": "review", "state": "blocked"}
            ],
            "next_actions": ["claim_available_work", "monitor_run_progress"],
        }
        assert _one(
            conn,
            """
            SELECT state, current_message_id, current_lease_id
            FROM striatumd.jobs
            WHERE repository_id = %s AND job_id = 'job_1'
            """,
            ("repo_a",),
        ) == {"state": "queued", "current_message_id": "msg_1", "current_lease_id": None}
        assert _one(
            conn,
            """
            SELECT state, current_lease_id
            FROM striatumd.queue_messages
            WHERE repository_id = %s AND message_id = 'msg_1'
            """,
            ("repo_a",),
        ) == {"state": "pending", "current_lease_id": None}
        assert _one(
            conn,
            "SELECT state FROM striatumd.blockers WHERE repository_id = %s AND blocker_id = 'blk_1'",
            ("repo_a",),
        ) == {"state": "resolved"}
        events = _events(conn, repository_id="repo_a")
        assert [row["event_type"] for row in events] == ["checkpoint.resolved"]
        assert events[0]["artifact_id"] == "artifact_decision"
        assert events[0]["payload_json"] == {
            "blocker_id": "blk_1",
            "action": "continue",
            "decision_id": "decision-001",
            "decision_artifact_id": "artifact_decision",
        }
    finally:
        conn.close()


def test_checkpoint_continue_without_current_message_enqueues_work(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_checkpoint_run(conn, tmp_path, repository_id="repo_a", message_id=None)
        conn.commit()

        result = handle(_ctx(conn, tmp_path, "repo_a"), {"blocker_id": "blk_1", "action": "continue"})

        assert result["status"] == "resolved"
        assert result["action"] == "continue"
        job = _one(
            conn,
            """
            SELECT state, current_message_id
            FROM striatumd.jobs
            WHERE repository_id = %s AND job_id = 'job_1'
            """,
            ("repo_a",),
        )
        assert job["state"] == "queued"
        assert isinstance(job["current_message_id"], str)
        assert job["current_message_id"].startswith("msg_")
        assert _one(
            conn,
            """
            SELECT state
            FROM striatumd.queue_messages
            WHERE repository_id = %s AND message_id = %s
            """,
            ("repo_a", job["current_message_id"]),
        ) == {"state": "pending"}
        assert [row["event_type"] for row in _events(conn, repository_id="repo_a")] == [
            "queue.message_enqueued",
            "checkpoint.resolved",
        ]
    finally:
        conn.close()


def test_checkpoint_cancel_marks_job_canceled_and_completes_run(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_checkpoint_run(
            conn,
            tmp_path,
            repository_id="repo_a",
            message_id="msg_1",
            with_completed_upstream=True,
        )
        conn.commit()

        result = handle(_ctx(conn, tmp_path, "repo_a"), {"blocker_id": "blk_1", "action": "cancel"})

        assert result == {
            "status": "resolved",
            "blocker_id": "blk_1",
            "job_id": "job_1",
            "action": "cancel",
            "decision_id": None,
            "decision_artifact_id": None,
            "run_state": "completed",
            "downstream_jobs": [],
            "next_actions": ["inspect_run_state", "export_run_evidence"],
        }
        assert _one(
            conn,
            """
            SELECT state, current_message_id, current_lease_id
            FROM striatumd.jobs
            WHERE repository_id = %s AND job_id = 'job_1'
            """,
            ("repo_a",),
        ) == {"state": "canceled", "current_message_id": None, "current_lease_id": None}
        assert _one(
            conn,
            "SELECT state FROM striatumd.queue_messages WHERE repository_id = %s AND message_id = 'msg_1'",
            ("repo_a",),
        ) == {"state": "canceled"}
        assert _one(
            conn,
            "SELECT state, stop_reason FROM striatumd.runs WHERE repository_id = %s AND run_id = 'run_1'",
            ("repo_a",),
        ) == {"state": "completed", "stop_reason": "all_jobs_terminal"}
        assert [row["event_type"] for row in _events(conn, repository_id="repo_a")] == [
            "run.completed",
            "checkpoint.canceled",
        ]
    finally:
        conn.close()


def test_checkpoint_resolve_rejects_unknown_action(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        with pytest.raises(InvalidTransitionError, match="unknown checkpoint resolve action"):
            handle(
                _ctx(conn, tmp_path, "repo_a"),
                {"blocker_id": "blk_1", "action": "defer"},
            )
    finally:
        conn.close()


@pytest.mark.parametrize(
    ("blocker_state", "severity", "message"),
    [
        ("resolved", "human_checkpoint", "blocker is not open"),
        ("open", "blocked", "checkpoint resolve only applies to human_checkpoint blockers"),
    ],
)
def test_checkpoint_resolve_requires_open_human_checkpoint_blocker(
    tmp_path: Path,
    pg_url: str,
    blocker_state: str,
    severity: str,
    message: str,
) -> None:
    conn = connect(pg_url)
    try:
        _seed_checkpoint_run(
            conn,
            tmp_path,
            repository_id="repo_a",
            message_id="msg_1",
            blocker_state=blocker_state,
            severity=severity,
        )
        conn.commit()

        with pytest.raises(InvalidTransitionError, match=message):
            handle(
                _ctx(conn, tmp_path, "repo_a"),
                {"blocker_id": "blk_1", "action": "continue"},
            )
    finally:
        conn.close()


def test_checkpoint_resolve_handler_registered() -> None:
    assert resolve_pg_handler("checkpoint.resolve") is handle


def _ctx(conn: Any, tmp_path: Path, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=tmp_path / repository_id,
        auth=RpcAuthContext("client", "token", repository_id, "admin", "allowed"),
    )


def _seed_checkpoint_run(
    conn: Any,
    tmp_path: Path,
    *,
    repository_id: str,
    message_id: str | None,
    blocker_state: str = "open",
    severity: str = "human_checkpoint",
    with_completed_upstream: bool = False,
    with_downstream: bool = False,
    with_decision: bool = False,
) -> None:
    now = "2026-05-14T00:00:00Z"
    repo_root = tmp_path / repository_id
    repo_root.mkdir()
    workflow_jobs = [{"id": "checkpoint", "type": "human_checkpoint"}]
    if with_completed_upstream:
        workflow_jobs.insert(0, {"id": "draft", "type": "draft"})
    if with_downstream:
        workflow_jobs.append({"id": "review", "type": "review"})
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"author": {}, "reviewer": {}, "human_checkpoint": {}},
        "lanes": {"local": {}},
        "jobs": workflow_jobs,
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
        if with_completed_upstream:
            cur.execute(
                """
                INSERT INTO striatumd.jobs (
                  repository_id, job_id, run_id, workflow_job_id, title, job_type,
                  role_id, lane_selector_json, capability_requirements_json, state,
                  write_scope_json, expected_artifacts_json, idempotency_key,
                  created_at, completed_at
                )
                VALUES (%s, 'job_done', 'run_1', 'draft', 'Draft', 'draft', 'author',
                        %s, %s, 'completed', %s, %s, 'draft-1', %s, %s)
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
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at, current_message_id
            )
            VALUES (%s, 'job_1', 'run_1', 'checkpoint', 'Checkpoint',
                    'human_checkpoint', 'human_checkpoint', %s, %s,
                    'waiting_human', %s, %s, 'checkpoint-1', %s, %s)
            """,
            (
                repository_id,
                Jsonb({"lane_id": "local"}),
                Jsonb({}),
                Jsonb({"repo_write": False}),
                Jsonb([]),
                now,
                message_id,
            ),
        )
        if message_id is not None:
            cur.execute(
                """
                INSERT INTO striatumd.queue_messages (
                  repository_id, message_id, run_id, job_id, kind, state,
                  target_role_id, payload_json, created_at, updated_at
                )
                VALUES (%s, %s, 'run_1', 'job_1', 'work', 'blocked',
                        'human_checkpoint', %s, %s, %s)
                """,
                (repository_id, message_id, Jsonb({}), now, now),
            )
        if with_downstream:
            cur.execute(
                """
                INSERT INTO striatumd.jobs (
                  repository_id, job_id, run_id, workflow_job_id, title, job_type,
                  role_id, lane_selector_json, capability_requirements_json, state,
                  write_scope_json, expected_artifacts_json, idempotency_key,
                  created_at
                )
                VALUES (%s, 'job_2', 'run_1', 'review', 'Review', 'review',
                        'reviewer', %s, %s, 'blocked', %s, %s, 'review-1', %s)
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
                VALUES (%s, 'job_2', 'job_1', %s)
                """,
                (repository_id, Jsonb({})),
            )
        cur.execute(
            """
            INSERT INTO striatumd.blockers (
              repository_id, blocker_id, run_id, job_id, severity, blocker_kind,
              description, state, created_at, resolved_at
            )
            VALUES (%s, 'blk_1', 'run_1', 'job_1', %s, 'revision_routing',
                    'needs operator decision', %s, %s, %s)
            """,
            (
                repository_id,
                severity,
                blocker_state,
                now,
                now if blocker_state != "open" else None,
            ),
        )
        if with_decision:
            cur.execute(
                """
                INSERT INTO striatumd.artifacts (
                  repository_id, artifact_id, run_id, logical_name, artifact_kind,
                  repo_path, content_sha256, size_bytes, publish_mode, created_at
                )
                VALUES (%s, 'artifact_decision', 'run_1', 'decision-001',
                        'decision', 'docs/decision.md', 'sha', 12, 'create', %s)
                """,
                (repository_id, now),
            )


def _events(conn: Any, *, repository_id: str) -> list[Mapping[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            SELECT event_type, job_id, artifact_id, payload_json
            FROM striatumd.events
            WHERE repository_id = %s
            ORDER BY event_id
            """,
            (repository_id,),
        )
        return list(cur.fetchall())


def _one(conn: Any, sql: str, params: tuple[Any, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, params)
        row = cur.fetchone()
    assert row is not None
    return dict(row)

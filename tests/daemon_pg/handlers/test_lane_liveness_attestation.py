# ruff: noqa
from __future__ import annotations

import os
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext, session_lane_attestation
from striatum.daemon_pg.handlers.workflow_loop.record_verdict import handle as record_verdict
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.errors import InvalidTransitionError
from striatum.identity import process_start_time


pytestmark = pytest.mark.multi_repo

LANE_COMMAND = ["agent", "--run"]


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_pg_lane_attestation_requires_live_pid_identity_and_command(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_review_run(conn, tmp_path, repository_id="repo_a")
        pid = os.getpid()
        pid_start = process_start_time(pid)
        if pid_start is None:
            pytest.skip("process start-time token unavailable on this platform")
        _insert_attached_supervisor(
            conn,
            repository_id="repo_a",
            pid=pid,
            pid_start_time=pid_start,
            command=LANE_COMMAND,
        )
        conn.commit()

        result = session_lane_attestation(_ctx(conn, tmp_path, "repo_a"), session_id="sess_1")

        assert result == {
            "attested": True,
            "state": "attested",
            "supervisor_id": "sup_1",
            "pid": pid,
            "reason": None,
        }
    finally:
        conn.close()


@pytest.mark.parametrize(
    ("pid", "pid_start_time", "command", "reason"),
    [
        (0, "stale-token", LANE_COMMAND, "pid_gone"),
        (os.getpid(), "stale-token", LANE_COMMAND, "pid_identity_mismatch"),
        (os.getpid(), None, LANE_COMMAND, "pid_identity_unavailable"),
        (os.getpid(), "current", ["agent", "--other"], "lane_command_mismatch"),
    ],
)
def test_pg_lane_attestation_reports_unattested_reasons(
    tmp_path: Path,
    pg_url: str,
    pid: int,
    pid_start_time: str | None,
    command: list[str],
    reason: str,
) -> None:
    conn = connect(pg_url)
    try:
        _seed_review_run(conn, tmp_path, repository_id="repo_a")
        if pid_start_time == "current":
            current = process_start_time(pid)
            if current is None:
                pytest.skip("process start-time token unavailable on this platform")
            pid_start_time = current
        _insert_attached_supervisor(
            conn,
            repository_id="repo_a",
            pid=pid,
            pid_start_time=pid_start_time,
            command=command,
        )
        conn.commit()

        result = session_lane_attestation(_ctx(conn, tmp_path, "repo_a"), session_id="sess_1")

        assert result["attested"] is False
        assert result["state"] == "unattested"
        assert result["supervisor_id"] == "sup_1"
        assert result["reason"] == reason
    finally:
        conn.close()


def test_require_attested_lane_verdict_rejects_stale_attached_supervisor(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_review_run(conn, tmp_path, repository_id="repo_a", require_attested_lane=True)
        _insert_attached_supervisor(
            conn,
            repository_id="repo_a",
            pid=0,
            pid_start_time="stale-token",
            command=LANE_COMMAND,
        )
        conn.commit()

        with pytest.raises(InvalidTransitionError, match="pid_gone"):
            record_verdict(
                _ctx(conn, tmp_path, "repo_a"),
                {
                    "session_id": "sess_1",
                    "job_id": "job_review",
                    "lease_id": "lease_1",
                    "verdict": "accept",
                    "rationale": "looks good",
                },
            )

        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.verdicts WHERE repository_id = %s",
            ("repo_a",),
        ) == {"count": 0}
        assert _one(
            conn,
            "SELECT state FROM striatumd.jobs WHERE repository_id = %s AND job_id = %s",
            ("repo_a", "job_review"),
        ) == {"state": "running"}
    finally:
        conn.close()


def _ctx(conn: Any, tmp_path: Path, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=tmp_path / repository_id,
        auth=RpcAuthContext("client", "token", repository_id, "review", "allowed"),
    )


def _seed_review_run(
    conn: Any,
    tmp_path: Path,
    *,
    repository_id: str,
    require_attested_lane: bool = False,
) -> None:
    now = "2026-05-14T00:00:00Z"
    repo_root = tmp_path / repository_id
    repo_root.mkdir()
    review_job: dict[str, Any] = {
        "id": "review",
        "type": "review",
        "review_posture": "security",
    }
    if require_attested_lane:
        review_job["require_attested_lane"] = True
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"reviewer": {}},
        "lanes": {"local": {"display_model": "codex-gpt-5", "command": LANE_COMMAND}},
        "jobs": [review_job],
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


def _insert_attached_supervisor(
    conn: Any,
    *,
    repository_id: str,
    pid: int,
    pid_start_time: str | None,
    command: list[str],
) -> None:
    now = "2026-05-14T00:00:00Z"
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.process_supervisors (
              repository_id, supervisor_id, run_id, session_id, adapter,
              command_json, cwd, scratch_path, pid, pid_start_time, state,
              started_at, heartbeat_at
            )
            VALUES (%s, 'sup_1', 'run_1', 'sess_1', 'process', %s, '/tmp',
                    '/tmp/scratch', %s, %s, 'attached', %s, %s)
            """,
            (repository_id, Jsonb(command), pid, pid_start_time, now, now),
        )


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)

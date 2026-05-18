from __future__ import annotations

import inspect
import os
import time
from collections.abc import Iterator, Mapping, Sequence
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.reads import _read_model
from striatum.daemon_pg.handlers.recovery_evidence.auto_finalize import (
    dry_run_projection,
    handle,
)
from striatum.daemon_pg.handlers.recovery_evidence.evidence_export import (
    _evidence_artifact_summaries,
)
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.errors import InvalidTransitionError
from striatum.identity import process_start_time
from striatum.primitives import json_dumps, sha256_bytes


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_auto_finalize_review_artifact_records_verdict_and_completes_run(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001")
        _seed_review_job(conn, repo_root, repository_id="repo_a")
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "run_id": "run_1",
                "dry_run": False,
                "mtime_grace_seconds": 0,
            },
        )

        assert result["finalized_count"] == 1
        artifact = _one(
            conn,
            """
            SELECT artifact_id, job_id, session_id, logical_name, artifact_kind,
                   repo_path, author_line
            FROM striatumd.artifacts
            WHERE repository_id = %s AND job_id = %s
            """,
            ("repo_a", "job_1"),
        )
        assert artifact == {
            "artifact_id": result["finalized"][0]["artifacts"][0]["artifact_id"],
            "job_id": "job_1",
            "session_id": "sess_1",
            "logical_name": "review",
            "artifact_kind": "finding",
            "repo_path": "docs/review.md",
            "author_line": "author: reviewer-codex-gpt-5-001",
        }
        assert _one(
            conn,
            "SELECT verdict, findings_artifact_id FROM striatumd.verdicts "
            "WHERE repository_id = %s AND job_id = %s",
            ("repo_a", "job_1"),
        ) == {
            "verdict": "accept_with_findings",
            "findings_artifact_id": artifact["artifact_id"],
        }
        assert _states(conn, repository_id="repo_a") == {
            "job": "completed",
            "job_lease": None,
            "message": "completed",
            "message_lease": None,
            "lease": "released",
            "lease_reason": "verdict",
            "run": "completed",
            "run_stop_reason": "all_jobs_terminal",
            "session": "closed",
        }
        event_types = [row["event_type"] for row in _events(conn, "repo_a")]
        assert "artifact.auto_finalized" in event_types
        assert "job.auto_finalized" in event_types
        assert "artifact.published" in event_types
        assert "verdict.recorded" in event_types
        job_event = next(row for row in _events(conn, "repo_a") if row["event_type"] == "job.auto_finalized")
        assert job_event["payload_json"]["lane_finalization"] == "auto_from_artifact"
        evidence_artifacts = _evidence_artifact_summaries(
            _ctx(conn, repo_root, repository_id="repo_a"),
            run_id="run_1",
            workflow={},
        )
        assert evidence_artifacts[0]["publish_origin"] == "auto_from_artifact"
    finally:
        conn.close()


def test_auto_finalize_dry_run_reports_unstable_file_without_mutation(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001", stable=False)
        _seed_review_job(conn, repo_root, repository_id="repo_a")
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "run_id": "run_1",
                "dry_run": True,
                "mtime_grace_seconds": 3600,
            },
        )

        assert result["eligible_count"] == 0
        assert result["skipped_count"] == 1
        assert result["skipped"][0]["artifacts"][0]["reason"] == (
            "expected artifact file mtime is inside the grace period"
        )
        assert _count(conn, "striatumd.artifacts", repository_id="repo_a") == 0
        assert _count(conn, "striatumd.verdicts", repository_id="repo_a") == 0
        assert _states(conn, repository_id="repo_a")["job"] == "running"
    finally:
        conn.close()


def test_auto_finalize_dry_run_projection_reports_status_eligibility(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001")
        _seed_review_job(conn, repo_root, repository_id="repo_a")
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        ctx = _ctx(conn, repo_root, repository_id="repo_a")
        projection = dry_run_projection(ctx, run_id="run_1", mtime_grace_seconds=0)
        status_payload = _read_model.status_payload(ctx, run_id="run_1")
        dashboard_payload = _read_model.dashboard_payload(ctx, run_id="run_1")

        assert projection["eligible_count"] == 1
        assert projection["finalized_count"] == 0
        assert projection["policy"]["live_allowed"] is True
        assert status_payload["auto_finalize_dry_run"]["eligible_count"] == 1
        assert "recovery_auto_finalize" in status_payload["next_actions"]
        assert dashboard_payload["status"]["auto_finalize_dry_run"]["eligible_count"] == 1
        assert _count(conn, "striatumd.artifacts", repository_id="repo_a") == 0
        assert _states(conn, repository_id="repo_a")["job"] == "running"
    finally:
        conn.close()


def test_status_auto_finalize_projection_reports_refusal_without_live_opt_in(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001", stable=False)
        _seed_review_job(conn, repo_root, repository_id="repo_a", auto_finalize_enabled=False)
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        ctx = _ctx(conn, repo_root, repository_id="repo_a")
        payload = _read_model.status_payload(ctx, run_id="run_1")
        projection = payload["auto_finalize_dry_run"]

        assert projection["policy"]["live_allowed"] is False
        assert projection["eligible_count"] == 0
        assert projection["skipped_count"] == 1
        assert projection["skipped"][0]["reason"] == "expected_artifact validation refused"
        assert projection["skipped"][0]["artifacts"][0]["reason"] == (
            "expected artifact file mtime is inside the grace period"
        )
        assert "recovery_auto_finalize" not in payload["next_actions"]
        assert _count(conn, "striatumd.artifacts", repository_id="repo_a") == 0
        assert _states(conn, repository_id="repo_a")["job"] == "running"
    finally:
        conn.close()


def test_auto_finalize_requires_all_required_artifacts_before_publishing(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001")
        expected = [
            {
                "logical_name": "review",
                "kind": "finding",
                "path": "docs/review.md",
                "required": True,
            },
            {
                "logical_name": "support",
                "kind": "support_ledger",
                "path": "docs/support.md",
                "required": True,
            },
        ]
        _seed_review_job(conn, repo_root, repository_id="repo_a", expected_artifacts=expected)
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "run_id": "run_1",
                "dry_run": False,
                "mtime_grace_seconds": 0,
            },
        )

        assert result["finalized_count"] == 0
        assert result["skipped_count"] == 1
        assert result["skipped"][0]["artifacts"][0]["reason"] == (
            "expected artifact file does not exist"
        )
        assert _count(conn, "striatumd.artifacts", repository_id="repo_a") == 0
        assert _count(conn, "striatumd.verdicts", repository_id="repo_a") == 0
        assert _states(conn, repository_id="repo_a")["job"] == "running"
    finally:
        conn.close()


def test_auto_finalize_refuses_live_without_policy_opt_in(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001")
        _seed_review_job(conn, repo_root, repository_id="repo_a", auto_finalize_enabled=False)
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        with pytest.raises(InvalidTransitionError, match="workflow recovery.auto_finalize.enabled"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "run_id": "run_1",
                    "dry_run": False,
                    "mtime_grace_seconds": 0,
                },
            )

        assert _count(conn, "striatumd.artifacts", repository_id="repo_a") == 0
    finally:
        conn.close()


def test_auto_finalize_handler_registered() -> None:
    assert resolve_pg_handler("recovery.auto_finalize") is handle
    assert inspect.signature(handle).parameters.keys() == {"ctx", "params"}


def _ctx(conn: Any, repo_root: Path, *, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "recovery", "allowed"),
    )


def _write_finding(
    repo_root: Path,
    *,
    byline: str,
    stable: bool = True,
    path_text: str = "docs/review.md",
    verdict_intent: str = "accept_with_findings",
) -> None:
    path = repo_root / path_text
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        "---\n"
        'schema_version: "striatum.finding.v1"\n'
        'artifact_kind: "finding"\n'
        f'verdict_intent: "{verdict_intent}"\n'
        'severity: "low"\n'
        "---\n\n"
        "# Review\n"
        f"{byline}\n\n"
        "Looks acceptable with follow-up notes.\n",
        encoding="utf-8",
    )
    if stable:
        old = time.time() - 120
        os.utime(path, (old, old))


def _seed_review_job(
    conn: Any,
    repo_root: Path,
    *,
    repository_id: str,
    expected_artifacts: Sequence[Mapping[str, Any]] | None = None,
    auto_finalize_enabled: bool = True,
) -> None:
    now = "2026-05-14T00:00:00Z"
    repo_root.mkdir(parents=True, exist_ok=True)
    expected = expected_artifacts or [
        {
            "logical_name": "review",
            "kind": "finding",
            "path": "docs/review.md",
            "required": True,
        }
    ]
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"reviewer": {}},
        "lanes": {"local": {"display_model": "codex-gpt-5", "command": ["agent"]}},
        "jobs": [
            {
                "id": "review",
                "type": "review",
                "require_attested_lane": True,
            }
        ],
        "cycles": [],
        "recovery": {"auto_finalize": {"enabled": auto_finalize_enabled}},
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
            (
                repository_id,
                repository_id,
                str(repo_root),
                str(repo_root / ".striatum"),
                repository_id,
                now,
            ),
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
            (repository_id, Jsonb(["write"]), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at, started_at, current_message_id, current_lease_id
            )
            VALUES (%s, 'job_1', 'run_1', 'review', 'Review', 'review',
                    'reviewer', %s, %s, 'running', %s, %s, 'review-1',
                    %s, %s, 'msg_1', 'lease_1')
            """,
            (
                repository_id,
                Jsonb({"lane_id": "local"}),
                Jsonb({}),
                Jsonb({"repo_write": True, "allowed_paths": ["docs/"], "forbidden_paths": []}),
                Jsonb(expected),
                now,
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages (
              repository_id, message_id, run_id, job_id, kind, state,
              target_role_id, target_lane_id, payload_json, created_at,
              updated_at, claimed_at, acked_at, current_lease_id
            )
            VALUES (%s, 'msg_1', 'run_1', 'job_1', 'work', 'acked',
                    'reviewer', 'local', %s, %s, %s, %s, %s, 'lease_1')
            """,
            (repository_id, Jsonb({}), now, now, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases (
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at,
              last_heartbeat_at
            )
            VALUES (%s, 'lease_1', 'run_1', 'job', 'job_1', 'sess_1',
                    'active', %s, '2099-01-01T00:00:00Z', %s)
            """,
            (repository_id, now, now),
        )


def _insert_review_job_instance(
    conn: Any,
    *,
    repository_id: str,
    workflow_job_id: str,
    job_id: str,
    session_id: str,
    session_ordinal: int,
    lease_id: str,
    message_id: str,
    artifact_path: str,
) -> None:
    now = "2026-05-14T00:00:00Z"
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.sessions (
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, %s, 'run_1', 'reviewer', 'local',
                    %s, %s, %s, 'active', %s, %s)
            """,
            (
                repository_id,
                session_id,
                f"reviewer-local-{session_ordinal}",
                session_ordinal,
                Jsonb(["write"]),
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
              created_at, started_at, current_message_id, current_lease_id
            )
            VALUES (%s, %s, 'run_1', %s, 'Review', 'review',
                    'reviewer', %s, %s, 'running', %s, %s, %s,
                    %s, %s, %s, %s)
            """,
            (
                repository_id,
                job_id,
                workflow_job_id,
                Jsonb({"lane_id": "local"}),
                Jsonb({}),
                Jsonb({"repo_write": True, "allowed_paths": ["docs/"], "forbidden_paths": []}),
                Jsonb([
                    {
                        "logical_name": "review",
                        "kind": "finding",
                        "path": artifact_path,
                        "required": True,
                    }
                ]),
                f"{workflow_job_id}-1",
                now,
                now,
                message_id,
                lease_id,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages (
              repository_id, message_id, run_id, job_id, kind, state,
              target_role_id, target_lane_id, payload_json, created_at,
              updated_at, claimed_at, acked_at, current_lease_id
            )
            VALUES (%s, %s, 'run_1', %s, 'work', 'acked',
                    'reviewer', 'local', %s, %s, %s, %s, %s, %s)
            """,
            (repository_id, message_id, job_id, Jsonb({}), now, now, now, now, lease_id),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases (
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at,
              last_heartbeat_at
            )
            VALUES (%s, %s, 'run_1', 'job', %s, %s,
                    'active', %s, '2099-01-01T00:00:00Z', %s)
            """,
            (repository_id, lease_id, job_id, session_id, now, now),
        )
    _insert_attached_supervisor(
        conn,
        repository_id=repository_id,
        supervisor_id=f"sup_{session_ordinal}",
        session_id=session_id,
    )
    _insert_work_packet_and_clean_process(
        conn,
        repository_id=repository_id,
        job_id=job_id,
        session_id=session_id,
        lease_id=lease_id,
        message_id=message_id,
        packet_id=f"wp_{session_ordinal}",
        process_id=f"proc_{session_ordinal}",
    )


def _insert_attached_supervisor(
    conn: Any,
    *,
    repository_id: str,
    supervisor_id: str = "sup_1",
    session_id: str = "sess_1",
) -> None:
    pid = os.getpid()
    pid_start = process_start_time(pid)
    assert pid_start is not None
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.process_supervisors (
              repository_id, supervisor_id, run_id, session_id, adapter,
              command_json, cwd, scratch_path, pid, pid_start_time, state,
              started_at, heartbeat_at
            )
            VALUES (%s, %s, 'run_1', %s, 'test', %s, '/tmp',
                    '/tmp/scratch', %s, %s, 'attached', '2026-05-14T00:00:00Z',
                    '2026-05-14T00:00:00Z')
            """,
            (repository_id, supervisor_id, session_id, Jsonb(["agent"]), pid, pid_start),
        )


def _insert_work_packet_and_clean_process(
    conn: Any,
    *,
    repository_id: str,
    job_id: str = "job_1",
    session_id: str = "sess_1",
    lease_id: str = "lease_1",
    message_id: str = "msg_1",
    packet_id: str = "wp_1",
    process_id: str = "proc_1",
) -> None:
    packet = {"packet_id": packet_id}
    packet_json = json_dumps(packet)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.work_packets (
              repository_id, packet_id, run_id, job_id, message_id, lease_id,
              session_id, packet_json, packet_sha256, created_at
            )
            VALUES (%s, %s, 'run_1', %s, %s, %s,
                    %s, %s, %s, '2026-05-14T00:00:00Z')
            """,
            (
                repository_id,
                packet_id,
                job_id,
                message_id,
                lease_id,
                session_id,
                Jsonb(packet),
                sha256_bytes(packet_json.encode("utf-8")),
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.process_executions (
              repository_id, process_id, run_id, job_id, session_id, lease_id,
              packet_id, adapter, command_json, cwd, scratch_path, stdin_mode,
              stdio_mode, pid, state, exit_code, started_at, ended_at
            )
            VALUES (%s, %s, 'run_1', %s, %s, %s,
                    %s, 'test', %s, '/tmp', '/tmp/scratch', 'packet',
                    'suppressed', 1234, 'exited', 0, '2026-05-14T00:00:00Z',
                    '2026-05-14T00:01:00Z')
            """,
            (repository_id, process_id, job_id, session_id, lease_id, packet_id, Jsonb(["agent"])),
        )


def _states(conn: Any, *, repository_id: str) -> dict[str, Any]:
    job = _one(
        conn,
        "SELECT state, current_lease_id FROM striatumd.jobs "
        "WHERE repository_id = %s AND job_id = 'job_1'",
        (repository_id,),
    )
    message = _one(
        conn,
        "SELECT state, current_lease_id FROM striatumd.queue_messages "
        "WHERE repository_id = %s AND message_id = 'msg_1'",
        (repository_id,),
    )
    lease = _one(
        conn,
        "SELECT state, release_reason FROM striatumd.leases "
        "WHERE repository_id = %s AND lease_id = 'lease_1'",
        (repository_id,),
    )
    run = _one(
        conn,
        "SELECT state, stop_reason FROM striatumd.runs "
        "WHERE repository_id = %s AND run_id = 'run_1'",
        (repository_id,),
    )
    session = _one(
        conn,
        "SELECT state FROM striatumd.sessions "
        "WHERE repository_id = %s AND session_id = 'sess_1'",
        (repository_id,),
    )
    return {
        "job": job["state"],
        "job_lease": job["current_lease_id"],
        "message": message["state"],
        "message_lease": message["current_lease_id"],
        "lease": lease["state"],
        "lease_reason": lease["release_reason"],
        "run": run["state"],
        "run_stop_reason": run["stop_reason"],
        "session": session["state"],
    }


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


def _count(conn: Any, table: str, *, repository_id: str) -> int:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            f"SELECT COUNT(*) AS count FROM {table} WHERE repository_id = %s",
            (repository_id,),
        )
        row = cur.fetchone()
    assert row is not None
    return int(row["count"])

from __future__ import annotations

import importlib
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import pytest
from psycopg.types.json import Jsonb

from striatum.daemon_rpc.envelope import RpcError

from test_list_read_handlers import insert_fixture, insert_repo, pg_conn as pg_conn, repo_context

pytestmark = pytest.mark.multi_repo


def test_job_detail_projects_expected_artifacts_process_and_verdict(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo)
    _seed_job_detail(pg_conn, repository_id="repo_a", repo_root=repo)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.job_detail")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo),
        {"run_id": "run_1", "job_id": "review"},
    )

    assert result["run"]["run_id"] == "run_1"
    assert result["job"]["job_id"] == "job_review"
    assert result["job"]["lane_id"] == "codex"
    assert result["job"]["lane_attestation_chip"]["attested"] is False
    assert result["latest_verdict"]["verdict_id"] == "verdict_1"
    assert result["latest_verdict"]["provenance"] == "operator-override"
    assert result["latest_verdict"]["override_rationale"] == "operator accepted"

    expected = result["expected_artifact_rows"][0]
    assert expected["path"] == "docs/review.md"
    assert expected["status"] == "missing_required"
    assert "striatum publish-artifact" in expected["publish_recipe"]
    assert "--session-id sess_1" in expected["publish_recipe"]
    assert result["artifacts"] == []

    process = result["process_evidence"][0]
    assert process["process_id"] == "proc_job_detail"
    assert process["command"] == ["false"]
    assert process["diagnostics"][0]["process_id"] == "proc_job_detail"


def test_job_detail_scopes_repository(pg_conn: Any, tmp_path: Path) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    _seed_job_detail(pg_conn, repository_id="repo_b", repo_root=repo_b)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.job_detail")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"run_id": "run_1", "job_id": "review"},
    )

    assert result["process_evidence"] == []
    assert result["expected_artifact_rows"] == []


def test_job_detail_rejects_unknown_run_or_job(pg_conn: Any, tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.job_detail")
    ctx = repo_context(pg_conn, repository_id="repo_a", repo_root=repo)

    with pytest.raises(RpcError, match="not found"):
        module.handle(ctx, {"run_id": "run_missing", "job_id": "review"})
    with pytest.raises(RpcError, match="not found"):
        module.handle(ctx, {"run_id": "run_1", "job_id": "missing"})
    with pytest.raises(RpcError, match="job_id must be a non-empty string"):
        module.handle(ctx, {"run_id": "run_1"})


def _seed_job_detail(conn: Any, *, repository_id: str, repo_root: Path) -> None:
    now = datetime.now(UTC)
    lease_expires = now + timedelta(hours=1)
    packet = {
        "session_id": "sess_1",
        "lease_id": "lease_job_detail",
        "message_id": "msg_job_detail",
        "expected_artifacts": [
            {
                "logical_name": "review",
                "kind": "finding",
                "path": "docs/review.md",
                "required": True,
                "author_line": "author: reviewer-codex-001",
            }
        ],
    }
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET expected_artifacts_json = %s, current_lease_id = 'lease_job_detail'
            WHERE repository_id = %s AND job_id = 'job_review'
            """,
            (
                Jsonb(
                    [
                        {
                            "logical_name": "review",
                            "kind": "finding",
                            "path": "docs/review.md",
                            "required": True,
                        }
                    ]
                ),
                repository_id,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases(
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at
            )
            VALUES (
              %s, 'lease_job_detail', 'run_1', 'job', 'job_review',
              'sess_1', 'active', %s, %s
            )
            """,
            (repository_id, now, lease_expires),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages(
              repository_id, message_id, run_id, job_id, kind, state,
              target_session_id, target_role_id, target_lane_id,
              payload_json, created_at, updated_at
            )
            VALUES (
              %s, 'msg_job_detail', 'run_1', 'job_review', 'work', 'claimed',
              'sess_1', 'reviewer', 'codex', '{}'::jsonb, %s, %s
            )
            """,
            (repository_id, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.work_packets(
              repository_id, packet_id, run_id, job_id, message_id, lease_id,
              session_id, packet_json, packet_sha256, created_at
            )
            VALUES (
              %s, 'packet_job_detail', 'run_1', 'job_review', 'msg_job_detail',
              'lease_job_detail', 'sess_1', %s, 'sha256:packet', %s
            )
            """,
            (repository_id, Jsonb(packet), now),
        )
        cur.execute(
            """
            UPDATE striatumd.verdicts
            SET rationale = 'operator accepted'
            WHERE repository_id = %s AND verdict_id = 'verdict_1'
            """,
            (repository_id,),
        )
        cur.execute(
            """
            INSERT INTO striatumd.events(
              repository_id, event_id, run_id, event_type, actor_session_id,
              payload_json, created_at
            )
            VALUES (%s, 101, 'run_1', 'verdict.overridden', 'sess_1', %s, %s)
            """,
            (repository_id, Jsonb({"verdict_id": "verdict_1"}), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.process_executions(
              repository_id, process_id, run_id, job_id, session_id, lease_id,
              packet_id, adapter, command_json, cwd, scratch_path, stdin_mode,
              stdio_mode, state, exit_code, started_at, ended_at
            )
            VALUES (
              %s, 'proc_job_detail', 'run_1', 'job_review', 'sess_1',
              'lease_job_detail', 'packet_job_detail', 'subprocess',
              %s, %s, '.striatum/scratch/proc_job_detail', 'packet',
              'suppressed', 'failed', 1, %s, %s
            )
            """,
            (repository_id, Jsonb(["false"]), str(repo_root), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.blockers(
              repository_id, blocker_id, run_id, job_id, session_id, severity,
              blocker_kind, description, state, created_at, payload_json
            )
            VALUES (
              %s, 'block_job_detail', 'run_1', 'job_review', 'sess_1',
              'blocked', 'process_outputs_missing', 'missing output',
              'open', %s, %s
            )
            """,
            (
                repository_id,
                now,
                Jsonb({"process_id": "proc_job_detail", "detail": "missing"}),
            ),
        )
    conn.commit()

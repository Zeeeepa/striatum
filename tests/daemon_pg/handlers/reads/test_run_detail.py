from __future__ import annotations

import importlib
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import pytest
from psycopg.types.json import Jsonb

from striatum.daemon_rpc.envelope import RpcError

from test_list_read_handlers import insert_fixture, insert_repo, pg_conn as pg_conn, repo_context

pytestmark = pytest.mark.multi_repo


def test_run_detail_projects_page_dto_and_scopes_repository(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    _seed_run_detail_extras(pg_conn, repository_id="repo_a")
    _seed_run_detail_extras(pg_conn, repository_id="repo_b", blocker_description="wrong repo")
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_detail")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"run_id": "run_1"},
    )

    assert result["run"]["run_id"] == "run_1"
    assert result["workflow"]["workflow_id"] == "workflow-a"
    assert result["suggested_branch_name"] == "feature/detail"
    assert result["allow_dirty"] is True
    assert result["verdicts_by_posture"] == {"security": 1}
    assert result["sessions"][0]["session_id"] == "sess_1"
    assert result["recovery_panel"]["blockers"][0]["description"] == "missing output"
    assert result["recovery_panel"]["blockers"][0]["recipes"] == [
        "striatum recovery process-reconcile --run-id run_1"
    ]
    assert "wrong repo" not in str(result["recovery_panel"])

    review_job = next(job for job in result["jobs"] if job["workflow_job_id"] == "review")
    assert review_job["lane_id"] == "codex"
    assert review_job["latest_verdict"]["verdict_id"] == "verdict_1"
    assert review_job["latest_verdict"]["provenance"] == "operator-override"
    assert review_job["latest_verdict"]["override_rationale"] == "accepted risk"
    assert review_job["lane_attestation_chip"]["attested"] is False


def test_run_detail_rejects_unknown_run(pg_conn: Any, tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_detail")

    with pytest.raises(RpcError, match="run not found"):
        module.handle(
            repo_context(pg_conn, repository_id="repo_a", repo_root=repo),
            {"run_id": "run_missing"},
        )


def test_run_detail_rejects_missing_run_id(pg_conn: Any, tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_detail")

    with pytest.raises(RpcError, match="run_id must be a non-empty string"):
        module.handle(repo_context(pg_conn, repository_id="repo_a", repo_root=repo), {})


def _seed_run_detail_extras(
    conn: Any,
    *,
    repository_id: str,
    blocker_description: str = "missing output",
) -> None:
    now = datetime.now(UTC)
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "workflow-a",
        "workflow_version": "1",
        "name": "Workflow A",
        "branch": {
            "mode": "auto",
            "suggested_name": "feature/detail",
            "allow_dirty": True,
        },
        "coordinator": {"role_id": "author", "lane_id": "codex"},
        "lanes": {"codex": {"adapter": "manual"}},
        "roles": {"author": {}, "reviewer": {}},
        "context_docs": [],
        "parallelism": {"mode": "serial", "max_active_jobs": 1},
        "jobs": [],
        "edges": [],
        "cycles": [],
    }
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.workflow_snapshots
            SET workflow_json = %s
            WHERE repository_id = %s AND workflow_snapshot_id = 'snap_1'
            """,
            (Jsonb(workflow), repository_id),
        )
        cur.execute(
            """
            UPDATE striatumd.verdicts
            SET posture = 'security', rationale = 'accepted risk'
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
            VALUES (%s, 100, 'run_1', 'verdict.overridden', 'sess_1', %s, %s)
            """,
            (repository_id, Jsonb({"verdict_id": "verdict_1"}), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.blockers(
              repository_id, blocker_id, run_id, job_id, session_id, severity,
              blocker_kind, description, state, created_at, payload_json
            )
            VALUES (
              %s, 'block_detail', 'run_1', 'job_review', 'sess_1', 'blocked',
              'process_outputs_missing', %s, 'open', %s, '{}'::jsonb
            )
            """,
            (repository_id, blocker_description, now),
        )
    conn.commit()

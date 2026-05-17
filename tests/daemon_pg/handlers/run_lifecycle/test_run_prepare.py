from __future__ import annotations

import json
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_pg.handlers.run_lifecycle.run_prepare import handle_run_prepare
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import WorkflowError

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_run_prepare_creates_snapshot_run_jobs_dependency_and_auto_branch(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        repo_root.mkdir()
        _seed_repo(conn, repo_root, repository_id="repo_a")
        workflow_path = repo_root / "workflow.json"
        _write_workflow(workflow_path, branch_mode="auto", branch_name="feature/phase-1")
        conn.commit()

        result = handle_run_prepare(_ctx(conn, repo_root, "repo_a"), {"workflow": "workflow.json"})

        run_id = str(result["run_id"])
        assert run_id.startswith("run_")
        assert result == {
            "run_id": run_id,
            "state": "ready",
            "branch_mode": "auto",
            "suggested_branch_name": "feature/phase-1",
            "branch": {
                "name": "feature/phase-1",
                "mode": "auto",
                "confirmed": True,
            },
        }
        run = _one(
            conn,
            """
            SELECT state, branch_name, branch_confirmed_by, repo_root
            FROM striatumd.runs
            WHERE repository_id = %s AND run_id = %s
            """,
            ("repo_a", run_id),
        )
        assert run == {
            "state": "ready",
            "branch_name": "feature/phase-1",
            "branch_confirmed_by": "daemon",
            "repo_root": str(repo_root),
        }
        snapshot = _one(
            conn,
            """
            SELECT workflow_id, workflow_version, source_path, workflow_json
            FROM striatumd.workflow_snapshots
            WHERE repository_id = %s
            """,
            ("repo_a",),
        )
        assert snapshot["workflow_id"] == "wf_prepare"
        assert snapshot["workflow_version"] == "1"
        assert snapshot["source_path"] == "workflow.json"
        assert snapshot["workflow_json"]["workflow_id"] == "wf_prepare"

        jobs = _all(
            conn,
            """
            SELECT workflow_job_id, title, job_type, role_id, state, lane_selector_json,
                   fresh_session_required, expected_artifacts_json
            FROM striatumd.jobs
            WHERE repository_id = %s AND run_id = %s
            ORDER BY workflow_job_id
            """,
            ("repo_a", run_id),
        )
        assert jobs == [
            {
                "workflow_job_id": "draft",
                "title": "Draft",
                "job_type": "draft",
                "role_id": "author",
                "state": "blocked",
                "lane_selector_json": {"lane_id": "local"},
                "fresh_session_required": False,
                "expected_artifacts_json": [
                    {
                        "logical_name": "draft",
                        "kind": "handoff",
                        "path": "artifacts/draft.md",
                        "required": True,
                    }
                ],
            },
            {
                "workflow_job_id": "review",
                "title": "Review",
                "job_type": "review",
                "role_id": "reviewer",
                "state": "blocked",
                "lane_selector_json": {"lane_id": "local"},
                "fresh_session_required": True,
                "expected_artifacts_json": [
                    {
                        "logical_name": "review",
                        "kind": "finding",
                        "path": "artifacts/review.md",
                        "required": True,
                    }
                ],
            },
        ]
        dependency = _one(
            conn,
            """
            SELECT downstream.workflow_job_id AS job, upstream.workflow_job_id AS depends_on,
                   dep.gate_json
            FROM striatumd.job_dependencies dep
            JOIN striatumd.jobs downstream
              ON downstream.repository_id = dep.repository_id
             AND downstream.job_id = dep.job_id
            JOIN striatumd.jobs upstream
              ON upstream.repository_id = dep.repository_id
             AND upstream.job_id = dep.depends_on_job_id
            WHERE dep.repository_id = %s
            """,
            ("repo_a",),
        )
        assert dependency == {
            "job": "review",
            "depends_on": "draft",
            "gate_json": {"on": "completed", "from": "draft", "to": "review"},
        }
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "run.prepared",
            "run.branch_confirmed",
        ]
    finally:
        conn.close()


def test_run_prepare_confirm_mode_waits_for_branch_confirmation(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        repo_root.mkdir()
        _seed_repo(conn, repo_root, repository_id="repo_a")
        _write_workflow(
            repo_root / "flows" / "workflow.json",
            branch_mode="confirm",
            branch_name="feature/manual",
        )
        conn.commit()

        result = handle_run_prepare(
            _ctx(conn, repo_root, "repo_a"),
            {"workflow": "flows/workflow.json"},
        )

        run_id = str(result["run_id"])
        assert result["state"] == "needs_branch_confirmation"
        assert result["branch"] == {
            "name": "feature/manual",
            "mode": "confirm",
            "confirmed": False,
        }
        assert _one(
            conn,
            """
            SELECT state, branch_name, branch_confirmed_at, branch_confirmed_by
            FROM striatumd.runs
            WHERE repository_id = %s AND run_id = %s
            """,
            ("repo_a", run_id),
        ) == {
            "state": "needs_branch_confirmation",
            "branch_name": "feature/manual",
            "branch_confirmed_at": None,
            "branch_confirmed_by": None,
        }
        assert [row["event_type"] for row in _events(conn, "repo_a")] == ["run.prepared"]
    finally:
        conn.close()


def test_run_prepare_reports_file_and_path_errors(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        repo_root.mkdir()
        _seed_repo(conn, repo_root, repository_id="repo_a")
        outside = tmp_path / "outside.json"
        _write_workflow(outside, branch_mode="auto", branch_name="feature/outside")
        conn.commit()

        with pytest.raises(RpcError, match="run.prepare requires workflow"):
            handle_run_prepare(_ctx(conn, repo_root, "repo_a"), {})
        with pytest.raises(WorkflowError, match="read workflow"):
            handle_run_prepare(_ctx(conn, repo_root, "repo_a"), {"workflow": "missing.json"})
        with pytest.raises(WorkflowError, match="must stay inside"):
            handle_run_prepare(_ctx(conn, repo_root, "repo_a"), {"workflow": "../outside.json"})
    finally:
        conn.close()


def test_run_prepare_handler_registered() -> None:
    assert resolve_pg_handler("run.prepare") is handle_run_prepare


def _ctx(conn: Any, repo_root: Path, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "admin", "allowed"),
    )


def _seed_repo(conn: Any, repo_root: Path, *, repository_id: str) -> None:
    now = "2026-05-14T00:00:00Z"
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


def _write_workflow(path: Path, *, branch_mode: str, branch_name: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(
            {
                "schema_version": "striatum.workflow.v1",
                "workflow_id": "wf_prepare",
                "workflow_version": "1",
                "name": "Prepare Test",
                "branch": {"mode": branch_mode, "suggested_name": branch_name},
                "coordinator": {"role_id": "author", "lane_id": "local"},
                "lanes": {"local": {"adapter": "process", "command": ["true"]}},
                "roles": {"author": {}, "reviewer": {}},
                "context_docs": [],
                "parallelism": {"max_active_jobs": 1},
                "jobs": [
                    {
                        "id": "draft",
                        "type": "draft",
                        "title": "Draft",
                        "role_id": "author",
                        "lane_id": "local",
                        "objective": "Draft the artifact.",
                        "write_scope": {
                            "mode": "repo_write",
                            "repo_write": True,
                            "allowed_paths": ["artifacts/"],
                            "forbidden_paths": [".striatum/"],
                        },
                        "expected_artifacts": [
                            {
                                "logical_name": "draft",
                                "kind": "handoff",
                                "path": "artifacts/draft.md",
                                "required": True,
                            }
                        ],
                    },
                    {
                        "id": "review",
                        "type": "review",
                        "title": "Review",
                        "role_id": "reviewer",
                        "lane_id": "local",
                        "reviewer_context_policy": "fresh",
                        "objective": "Review the draft.",
                        "write_scope": {
                            "mode": "review_only_artifact",
                            "repo_write": False,
                            "allowed_paths": ["artifacts/"],
                            "forbidden_paths": [".striatum/"],
                        },
                        "expected_artifacts": [
                            {
                                "logical_name": "review",
                                "kind": "finding",
                                "path": "artifacts/review.md",
                                "required": True,
                            }
                        ],
                    },
                ],
                "edges": [{"from": "draft", "to": "review", "on": "completed"}],
                "cycles": [],
            }
        ),
        encoding="utf-8",
    )


def _events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    return _all(
        conn,
        "SELECT * FROM striatumd.events WHERE repository_id = %s ORDER BY event_id",
        (repository_id,),
    )


def _all(conn: Any, sql: str, args: tuple[object, ...]) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        return [dict(row) for row in cur.fetchall()]


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)

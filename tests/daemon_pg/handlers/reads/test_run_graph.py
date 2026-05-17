from __future__ import annotations

import importlib
import inspect
from collections.abc import Iterator
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import pytest
from psycopg.types.json import Jsonb

from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_pg.migrations import apply_migrations
from striatum.daemon_rpc.capability import RpcAuthContext

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_conn(postgres_url: str) -> Iterator[Any]:
    from _harness.pg import create_ephemeral_database, drop_ephemeral_database

    ephemeral = create_ephemeral_database(postgres_url)
    conn = connect(ephemeral.database_url)
    try:
        apply_migrations(conn)
        yield conn
    finally:
        conn.close()
        drop_ephemeral_database(ephemeral)


def _ctx(conn: Any, *, repository_id: str, repo_root: Path) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "read", "allowed"),
    )


def _insert_repo(conn: Any, repo_root: Path, repository_id: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path, display_name,
              registered_at, last_seen_at, last_schema_version, state, settings_json
            )
            VALUES (%s, %s, %s, %s, %s, now(), now(), 5, 'active', '{}'::jsonb)
            """,
            (
                repository_id,
                f"identity:{repository_id}",
                str(repo_root.resolve()),
                str((repo_root / ".striatum" / "state.sqlite3").resolve()),
                repository_id,
            ),
        )
    conn.commit()


def _workflow() -> dict[str, Any]:
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "workflow-graph",
        "workflow_version": "1",
        "name": "Workflow Graph",
        "branch": {"mode": "confirm"},
        "coordinator": {"role_id": "author", "lane_id": "codex"},
        "lanes": {"codex": {"adapter": "manual"}},
        "roles": {"author": {}, "reviewer": {}},
        "context_docs": [],
        "parallelism": {"mode": "serial", "max_active_jobs": 1},
        "jobs": [
            {
                "id": "draft",
                "type": "draft",
                "role_id": "author",
                "lane_id": "codex",
                "write_scope": {"mode": "repo_write", "allowed_paths": ["docs/"], "forbidden_paths": []},
                "expected_artifacts": [],
            },
            {
                "id": "review",
                "type": "review",
                "role_id": "reviewer",
                "lane_id": "codex",
                "write_scope": {"mode": "review_only", "allowed_paths": [], "forbidden_paths": []},
                "expected_artifacts": [],
            },
        ],
        "edges": [{"from": "draft", "to": "review", "on": "completed"}],
        "cycles": [],
    }


def _insert_graph_fixture(conn: Any, *, repository_id: str, repo_root: Path) -> None:
    now = datetime.now(UTC)
    workflow = _workflow()
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots(
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              source_path, content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, 'snap_graph', 'workflow-graph', '1',
                    'workflow.json', 'sha256:wf', %s, %s)
            """,
            (repository_id, Jsonb(workflow), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs(
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              branch_name, branch_base, branch_confirmed_at, branch_confirmed_by,
              created_at, started_at
            )
            VALUES (%s, 'run_graph', 'snap_graph', %s, 'running',
                    'main', 'main', %s, 'operator', %s, %s)
            """,
            (repository_id, str(repo_root.resolve()), now, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions(
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, 'sess_reviewer', 'run_graph', 'reviewer', 'codex',
                    'reviewer-codex-1', 1, '["read"]'::jsonb, 'active', %s, %s)
            """,
            (repository_id, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs(
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, state, attempt, max_attempts,
              idempotency_key, created_at
            )
            VALUES
              (%s, 'job_draft_1', 'run_graph', 'draft', 'Draft', 'draft',
               'author', '{"lane_id":"codex"}'::jsonb, 'failed', 1, 2,
               'draft:1', %s),
              (%s, 'job_draft_2', 'run_graph', 'draft', 'Draft', 'draft',
               'author', '{"lane_id":"codex"}'::jsonb, 'completed', 2, 2,
               'draft:2', %s),
              (%s, 'job_review', 'run_graph', 'review', 'Review', 'review',
               'reviewer', '{"lane_id":"codex"}'::jsonb, 'completed', 1, 1,
               'review:1', %s)
            """,
            (repository_id, now, repository_id, now, repository_id, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.job_dependencies(
              repository_id, job_id, depends_on_job_id, gate_json
            )
            VALUES (%s, 'job_review', 'job_draft_2',
                    '{"on":"completed","from":"draft","to":"review"}'::jsonb)
            """,
            (repository_id,),
        )
        cur.execute(
            """
            INSERT INTO striatumd.verdicts(
              repository_id, verdict_id, run_id, job_id, session_id, verdict,
              rationale, created_at, posture
            )
            VALUES (%s, 'verdict_accept', 'run_graph', 'job_review',
                    'sess_reviewer', 'accept', 'looks good', %s, 'neutral')
            """,
            (repository_id, now),
        )
    conn.commit()


def test_run_graph_registers_with_locked_signature() -> None:
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_graph")
    handler = resolve_pg_handler("run.graph")

    assert handler is not None
    assert handler is module.handle
    assert list(inspect.signature(handler).parameters) == ["ctx", "params"]
    assert getattr(handler, "read_only", False) is True


def test_run_graph_json_uses_pg_jobs_and_dependencies(pg_conn: Any, tmp_path: Path) -> None:
    repo_root = tmp_path / "repo"
    _insert_repo(pg_conn, repo_root, "repo_graph")
    _insert_graph_fixture(pg_conn, repository_id="repo_graph", repo_root=repo_root)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_graph")
    handle = module.handle

    result = handle(_ctx(pg_conn, repository_id="repo_graph", repo_root=repo_root), {"run_id": "run_graph", "format": "json"})

    assert result["run_id"] == "run_graph"
    assert result["workflow_id"] == "workflow-graph"
    nodes = {node["job_id"]: node for node in result["graph"]["nodes"]}
    assert nodes["draft"]["current_state"] == "completed"
    assert nodes["draft"]["attempt"] == 2
    assert nodes["draft"]["state_class"] == "state-completed"
    assert nodes["review"]["latest_verdict"]["verdict"] == "accept"
    assert result["graph"]["edges"] == [
        {"from": "draft", "to": "review", "gate": {"on": "completed"}}
    ]


def test_run_graph_mermaid_returns_stateful_source(pg_conn: Any, tmp_path: Path) -> None:
    repo_root = tmp_path / "repo"
    _insert_repo(pg_conn, repo_root, "repo_graph")
    _insert_graph_fixture(pg_conn, repository_id="repo_graph", repo_root=repo_root)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_graph")
    handle = module.handle

    result = handle(
        _ctx(pg_conn, repository_id="repo_graph", repo_root=repo_root),
        {"run_id": "run_graph", "format": "mermaid"},
    )

    assert result["format"] == "mermaid"
    assert "flowchart TD" in result["source"]
    assert "draft" in result["source"]
    assert "-->|completed|" in result["source"]
    assert "classDef state-completed" in result["source"]
    assert "class n0 state-completed" in result["source"]

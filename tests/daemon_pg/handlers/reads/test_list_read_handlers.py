from __future__ import annotations

import importlib
import inspect
from collections.abc import Iterator
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, cast

import pytest
from psycopg.types.json import Jsonb

from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.migrations import apply_migrations
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import PgHandler
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.envelope import RpcError

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


def repo_context(conn: Any, *, repository_id: str, repo_root: Path) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "read", "allowed"),
    )


def insert_repo(conn: Any, repo_root: Path, repository_id: str) -> None:
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


def insert_fixture(conn: Any, *, repository_id: str, repo_root: Path) -> None:
    now = datetime.now(UTC)
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "workflow-a",
        "workflow_version": "1",
        "name": "Workflow A",
        "branch": {"mode": "manual"},
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
            INSERT INTO striatumd.workflow_snapshots(
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              source_path, content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, 'snap_1', 'workflow-a', '1', 'workflow.json', 'sha256:wf', %s, %s)
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
            VALUES (%s, 'run_1', 'snap_1', %s, 'running',
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
            VALUES (%s, 'sess_1', 'run_1', 'author', 'codex', 'author-codex-1',
                    1, '["read"]'::jsonb, 'active', %s, %s)
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
              (%s, 'job_draft', 'run_1', 'draft', 'Draft', 'draft',
               'author', '{"lane_id":"codex"}'::jsonb, 'completed', 1, 1,
               'draft:1', %s),
              (%s, 'job_review', 'run_1', 'review', 'Review', 'review',
               'reviewer', '{"lane_id":"codex"}'::jsonb, 'completed', 1, 1,
               'review:1', %s)
            """,
            (repository_id, now, repository_id, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.artifacts(
              repository_id, artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
              created_at, author_line
            )
            VALUES (%s, 'art_1', 'run_1', 'job_draft', 'sess_1', 'draft',
                    'handoff', 'docs/draft.md', 'sha256:artifact', 12,
                    'create', %s, 'author: author-codex-001')
            """,
            (repository_id, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.verdicts(
              repository_id, verdict_id, run_id, job_id, session_id, verdict,
              created_at
            )
            VALUES (%s, 'verdict_1', 'run_1', 'job_review', 'sess_1', 'accept', %s)
            """,
            (repository_id, now),
        )
    conn.commit()


@pytest.mark.parametrize(
    ("module_name", "method"),
    [
        ("list_runs", "list.runs"),
        ("list_sessions", "list.sessions"),
        ("list_jobs", "list.jobs"),
        ("list_artifacts", "list.artifacts"),
        ("list_workflows", "list.workflows"),
    ],
)
def test_read_handlers_register_with_locked_signature(module_name: str, method: str) -> None:
    module = importlib.import_module(f"striatum.daemon_pg.handlers.reads.{module_name}")
    handle = cast(PgHandler, module.handle)

    assert resolve_pg_handler(method) is handle
    assert list(inspect.signature(handle).parameters) == ["ctx", "params"]
    source = inspect.getsource(module)
    assert f'@register_pg_handler("{method}", read_only=True)' in source


def test_list_runs_filters_and_scopes_repository(pg_conn: Any, tmp_path: Path) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.list_runs")

    result = module.handle(repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a), {"state": "running"})

    assert result["count"] == 1
    assert result["items"][0]["run_id"] == "run_1"
    assert result["items"][0]["workflow_id"] == "workflow-a"
    with pytest.raises(RpcError, match="unknown run state"):
        module.handle(repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a), {"state": "weird"})


def test_list_sessions_filters_and_scopes_repository(pg_conn: Any, tmp_path: Path) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.list_sessions")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"run_id": "run_1", "role": "author", "lane": "codex"},
    )

    assert result["count"] == 1
    row = result["items"][0]
    assert row["session_id"] == "sess_1"
    assert row["capabilities"] == ["read"]
    assert row["lane_attestation"] == "unattested"
    with pytest.raises(RpcError, match="unknown session state"):
        module.handle(
            repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
            {"run_id": "run_1", "state": "weird"},
        )


def test_list_jobs_filters_verdict_and_scopes_repository(pg_conn: Any, tmp_path: Path) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.list_jobs")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"run_id": "run_1", "workflow_job_id": "review"},
    )

    assert result["count"] == 1
    assert result["items"][0]["job_id"] == "job_review"
    assert result["items"][0]["lane_id"] == "codex"
    assert result["items"][0]["verdict"] == "accept"
    with pytest.raises(RpcError, match="unknown job state"):
        module.handle(
            repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
            {"run_id": "run_1", "state": "weird"},
        )


def test_list_artifacts_filters_and_scopes_repository(pg_conn: Any, tmp_path: Path) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.list_artifacts")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"run_id": "run_1", "kind": "handoff"},
    )

    assert result["count"] == 1
    assert result["items"][0]["artifact_id"] == "art_1"
    assert result["items"][0]["author"]["author_line"] == "author: author-codex-001"
    with pytest.raises(RpcError, match="unknown artifact kind"):
        module.handle(
            repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
            {"run_id": "run_1", "kind": "transcript"},
        )


def test_list_workflows_scopes_repository(pg_conn: Any, tmp_path: Path) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.list_workflows")

    result = module.handle(repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a), {})

    assert result["count"] == 1
    assert result["items"][0]["workflow_snapshot_id"] == "snap_1"
    assert result["items"][0]["workflow_id"] == "workflow-a"

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
                str((repo_root / ".striatum" / "retired-local-state").resolve()),
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
        "lanes": {"codex": {"adapter": "manual", "display_model": "Codex"}},
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
        ("artifact_show", "artifact.show"),
        ("job_detail", "job.detail"),
        ("list_runs", "list.runs"),
        ("list_sessions", "list.sessions"),
        ("list_jobs", "list.jobs"),
        ("list_artifacts", "list.artifacts"),
        ("list_workflows", "list.workflows"),
        ("run_detail", "run.detail"),
        ("run_events", "run.events"),
        ("run_posture_verdicts", "run.posture_verdicts"),
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
    assert result["items"][0]["workflow_name"] == "Workflow A"
    assert result["items"][0]["workflow_version"] == "1"
    assert result["items"][0]["workflow_snapshot_id"] == "snap_1"
    assert result["items"][0]["source_path"] == "workflow.json"
    assert result["items"][0]["workflow_identity"] == {
        "workflow_id": "workflow-a",
        "workflow_version": "1",
        "workflow_snapshot_id": "snap_1",
    }
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
    author = result["items"][0]["author"]
    assert author["line"] == "author: author-codex-001"
    assert author["actual_author_line"] == "author: author-codex-001"
    assert author["author_line"] == "author: author-codex-001"
    assert author["display_model"] == "Codex"
    with pytest.raises(RpcError, match="unknown artifact kind"):
        module.handle(
            repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
            {"run_id": "run_1", "kind": "transcript"},
        )


def test_artifact_show_returns_metadata_and_scopes_repository(pg_conn: Any, tmp_path: Path) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.artifact_show")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"artifact_id": "art_1"},
    )

    artifact = result["artifact"]
    assert artifact["artifact_id"] == "art_1"
    assert artifact["run_id"] == "run_1"
    assert artifact["artifact_kind"] == "handoff"
    assert artifact["repo_path"] == "docs/draft.md"
    assert artifact["content_sha256"] == "sha256:artifact"
    assert artifact["size_bytes"] == 12
    assert artifact["author_line"] == "author: author-codex-001"

    with pytest.raises(RpcError, match="artifact not found"):
        module.handle(
            repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
            {"artifact_id": "art_missing"},
        )
    with pytest.raises(RpcError, match="artifact_id must be a non-empty string"):
        module.handle(repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a), {})


def test_artifact_show_web_context_scopes_run_and_projects_provenance(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    now = datetime.now(UTC)
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET expected_artifacts_json = %s
            WHERE repository_id = 'repo_a' AND job_id = 'job_draft'
            """,
            (Jsonb([{"path": "docs/draft.md", "kind": "handoff", "logical_name": "draft"}]),),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages(
              repository_id, message_id, run_id, job_id, kind, state,
              priority, target_session_id, target_role_id, target_lane_id,
              payload_json, claim_count, max_claims, created_at, updated_at
            )
            VALUES (
              'repo_a', 'msg_artifact_show', 'run_1', 'job_draft', 'work',
              'claimed', 0, 'sess_1', 'author', 'codex', '{}'::jsonb,
              1, 1, %s, %s
            )
            """,
            (now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases(
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at,
              last_heartbeat_at
            )
            VALUES (
              'repo_a', 'lease_artifact_show', 'run_1', 'job', 'job_draft',
              'sess_1', 'active', %s, %s, %s
            )
            """,
            (now, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.work_packets(
              repository_id, packet_id, run_id, job_id, message_id, lease_id,
              session_id, packet_json, packet_sha256, created_at
            )
            VALUES (
              'repo_a', 'packet_artifact_show', 'run_1', 'job_draft',
              'msg_artifact_show', 'lease_artifact_show', 'sess_1',
              %s, 'sha256:packet', %s
            )
            """,
            (
                Jsonb(
                    {
                        "expected_artifacts": [
                            {
                                "path": "docs/draft.md",
                                "author_line": "author: author-codex-001",
                            }
                        ]
                    }
                ),
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.events(
              repository_id, event_id, run_id, event_type, actor_session_id,
              job_id, artifact_id, payload_json, created_at
            )
            VALUES (
              'repo_a', 101, 'run_1',
              'provenance.publish_without_process_execution', 'sess_1',
              'job_draft', 'art_1', %s, %s
            ),
            (
              'repo_b', 102, 'run_1',
              'provenance.publish_without_process_execution', 'sess_1',
              'job_draft', 'art_1', %s, %s
            )
            """,
            (
                Jsonb({"artifact_id": "art_1", "note": "scoped"}),
                now,
                Jsonb({"artifact_id": "art_1", "note": "wrong-repo"}),
                now,
            ),
        )
    pg_conn.commit()
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.artifact_show")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"artifact_id": "art_1", "run_id": "run_1", "include_web_context": True},
    )

    assert result["run"] == {"run_id": "run_1", "state": "running", "branch_name": "main"}
    assert result["artifact"]["artifact_id"] == "art_1"
    assert result["expected_author_line"] == "author: author-codex-001"
    assert [event["payload"]["note"] for event in result["provenance_trail"]] == ["scoped"]
    with pytest.raises(RpcError, match="artifact not found in run"):
        module.handle(
            repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
            {"artifact_id": "art_1", "run_id": "run_missing", "include_web_context": True},
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

from __future__ import annotations

import importlib.util
import sys
import types
from collections.abc import Iterator
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import pytest
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from _harness.tokens import issue_token
from striatum.daemon_pg.connection import connect
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.envelope import RpcEnvelope
from striatum.daemon_rpc.server import DaemonRpcRouter

pytestmark = pytest.mark.multi_repo

ROOT = Path(__file__).resolve().parents[4]
HANDLERS_DIR = ROOT / "src" / "striatum" / "daemon_pg" / "handlers"


@pytest.fixture
def pg_conn(postgres_url: str) -> Iterator[Any]:
    ephemeral = create_ephemeral_database(postgres_url)
    conn = connect(ephemeral.database_url)
    try:
        yield conn
    finally:
        conn.close()
        drop_ephemeral_database(ephemeral)


def load_pg_handler(monkeypatch: pytest.MonkeyPatch, module: str) -> tuple[Any, Any]:
    """Load one handler file while Track A's package __init__ is incomplete."""
    package = types.ModuleType("striatum.daemon_pg.handlers")
    package.__path__ = [str(HANDLERS_DIR)]
    monkeypatch.setitem(sys.modules, "striatum.daemon_pg.handlers", package)
    context = _load_module(
        monkeypatch,
        "striatum.daemon_pg.handlers.context",
        HANDLERS_DIR / "context.py",
    )
    _load_module(
        monkeypatch,
        "striatum.daemon_pg.handlers.registry",
        HANDLERS_DIR / "registry.py",
    )
    handler = _load_module(
        monkeypatch,
        f"striatum.daemon_pg.handlers.workflow_loop.{module}",
        HANDLERS_DIR / "workflow_loop" / f"{module}.py",
    )
    return handler, context


def _load_module(monkeypatch: pytest.MonkeyPatch, name: str, path: Path) -> Any:
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load module spec for {path}")
    mod = importlib.util.module_from_spec(spec)
    monkeypatch.setitem(sys.modules, name, mod)
    spec.loader.exec_module(mod)
    return mod


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


def insert_run(conn: Any, *, repository_id: str, repo_root: Path, run_id: str = "run_1") -> str:
    now = datetime.now(UTC)
    snapshot_id = "snap_1"
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots(
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              source_path, content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, %s, 'test-workflow', '1', 'workflow.json', 'sha256:test', %s, %s)
            """,
            (repository_id, snapshot_id, Jsonb(workflow_json()), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs(
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              branch_name, branch_base, branch_confirmed_at, branch_confirmed_by,
              created_at, started_at
            )
            VALUES (%s, %s, %s, %s, 'running', 'main', 'main', %s, 'operator', %s, %s)
            """,
            (repository_id, run_id, snapshot_id, str(repo_root.resolve()), now, now, now),
        )
    conn.commit()
    return run_id


def workflow_json() -> dict[str, Any]:
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "test-workflow",
        "workflow_version": "1",
        "name": "Test workflow",
        "branch": {"mode": "manual"},
        "coordinator": {"role_id": "author", "lane_id": "codex"},
        "lanes": {"codex": {"adapter": "manual"}},
        "roles": {"author": {"summary": "Author"}, "reviewer": {"summary": "Reviewer"}},
        "context_docs": [],
        "parallelism": {"mode": "serial", "max_active_jobs": 1},
        "jobs": [{"id": "draft", "type": "draft", "role": "author", "lane_id": "codex"}],
        "edges": [],
        "cycles": [],
    }


def repo_context(context_mod: Any, conn: Any, *, repository_id: str, repo_root: Path) -> Any:
    return context_mod.RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "claim", "allowed"),
    )


def fetch_events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT *
            FROM striatumd.events
            WHERE repository_id = %s
            ORDER BY event_id
            """,
            (repository_id,),
        )
        columns = [desc[0] for desc in cur.description]
        return [dict(zip(columns, row, strict=True)) for row in cur.fetchall()]


def rpc(
    conn: Any,
    *,
    repo_root: Path,
    method: str,
    params: dict[str, Any],
    token: str,
    request_id: str,
) -> Any:
    router = DaemonRpcRouter(pg_conn=conn, repo_root=repo_root)
    return router.handle(
        RpcEnvelope(
            schema_version=1,
            request_id=request_id,
            method=method,
            params=params,
            capability_token=token,
        ),
        require_handshake=False,
    )


def test_register_session_scopes_rows_and_appends_hashable_event(
    pg_conn: Any,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    handler, context_mod = load_pg_handler(monkeypatch, "register_session")
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_run(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_run(pg_conn, repository_id="repo_b", repo_root=repo_b)

    result = handler.handle(
        repo_context(context_mod, pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"run_id": "run_1", "role": "author", "lane": "codex", "capabilities": ["claim"]},
    )

    assert result["slug"] == "author-codex-1"
    with pg_conn.cursor() as cur:
        cur.execute(
            "SELECT COUNT(*) FROM striatumd.sessions WHERE repository_id = %s",
            ("repo_a",),
        )
        assert cur.fetchone()[0] == 1
        cur.execute(
            "SELECT COUNT(*) FROM striatumd.sessions WHERE repository_id = %s",
            ("repo_b",),
        )
        assert cur.fetchone()[0] == 0
    events = fetch_events(pg_conn, "repo_a")
    assert [row["event_type"] for row in events] == ["session.registered"]
    assert context_mod.canonical_event_hash(events[0])


def test_register_session_requires_claim_capability_before_route(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_run(pg_conn, repository_id="repo_a", repo_root=repo)
    token = issue_token(pg_conn, capabilities=["read"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="session.register",
        params={"repository_id": "repo_a", "run_id": "run_1", "role": "author", "lane": "codex"},
        token=token,
        request_id="register-denied",
    )

    assert response.ok is False
    assert response.data["code"] == "capability_missing"
    with pg_conn.cursor() as cur:
        cur.execute("SELECT COUNT(*) FROM striatumd.sessions")
        assert cur.fetchone()[0] == 0


def insert_claimable_work(
    conn: Any,
    *,
    repository_id: str,
    repo_root: Path,
    run_id: str = "run_1",
    session_id: str = "sess_author",
    job_id: str = "job_draft",
    message_id: str = "msg_draft",
) -> None:
    insert_run(conn, repository_id=repository_id, repo_root=repo_root, run_id=run_id)
    now = datetime.now(UTC)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.sessions(
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, fresh_context, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, %s, %s, 'author', 'codex', 'author-codex-1', 1,
                    '["claim"]'::jsonb, false, 'active', %s, %s)
            """,
            (repository_id, session_id, run_id, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs(
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              attempt, max_attempts, fresh_session_required, write_scope_json,
              expected_artifacts_json, idempotency_key, created_at, ready_at
            )
            VALUES (%s, %s, %s, 'draft', 'Draft', 'draft', 'author',
                    '{"lane_id":"codex"}'::jsonb, '{"objective":"Draft"}'::jsonb,
                    'queued', 1, 1, false,
                    '{"mode":"repo_write","repo_write":true,"allowed_paths":["docs"],"forbidden_paths":[]}'::jsonb,
                    '[]'::jsonb, 'draft:1', %s, %s)
            """,
            (repository_id, job_id, run_id, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages(
              repository_id, message_id, run_id, job_id, kind, state, priority,
              target_role_id, target_lane_id, payload_json, claim_count,
              max_claims, created_at, updated_at
            )
            VALUES (%s, %s, %s, %s, 'work', 'pending', 0, 'author', 'codex',
                    '{}'::jsonb, 0, 1, %s, %s)
            """,
            (repository_id, message_id, run_id, job_id, now, now),
        )
    conn.commit()


def mark_work_claimed(
    conn: Any,
    *,
    repository_id: str,
    lease_id: str = "lease_draft",
    run_id: str = "run_1",
    session_id: str = "sess_author",
    job_id: str = "job_draft",
    message_id: str = "msg_draft",
) -> None:
    now = datetime.now(UTC)
    expires_at = now + timedelta(minutes=15)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.leases(
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
            )
            VALUES (%s, %s, %s, 'job', %s, %s, 'active', %s, %s, %s)
            """,
            (repository_id, lease_id, run_id, job_id, session_id, now, expires_at, now),
        )
        cur.execute(
            """
            UPDATE striatumd.queue_messages
            SET state = 'claimed', claimed_at = %s, current_lease_id = %s, claim_count = 1
            WHERE repository_id = %s AND message_id = %s
            """,
            (now, lease_id, repository_id, message_id),
        )
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET state = 'claimed', current_lease_id = %s
            WHERE repository_id = %s AND job_id = %s
            """,
            (lease_id, repository_id, job_id),
        )
    conn.commit()

from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_pg.handlers.run_lifecycle.branch_confirm import handle
from striatum.daemon_rpc.capability import RpcAuthContext

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_branch_confirm_records_ready_state_and_event(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        repo_root.mkdir()
        _seed_run(conn, repo_root, repository_id="repo_a", run_state="needs_branch_confirmation")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, "repo_a"),
            {"run_id": "run_1", "branch": "feature/phase-1"},
        )

        assert result == {
            "run_id": "run_1",
            "state": "ready",
            "branch": "feature/phase-1",
            "requested_branch": "feature/phase-1",
            "current_git_branch": None,
            "records_only": True,
            "warning": None,
            "created": False,
            "mode": "records_only",
        }
        assert _one(
            conn,
            """
            SELECT state, branch_name, branch_confirmed_by
            FROM striatumd.runs
            WHERE repository_id = %s AND run_id = 'run_1'
            """,
            ("repo_a",),
        ) == {"state": "ready", "branch_name": "feature/phase-1", "branch_confirmed_by": "human"}
        events = _events(conn, "repo_a")
        assert [row["event_type"] for row in events] == ["run.branch_confirmed"]
        assert events[0]["payload_json"] == {
            "branch": "feature/phase-1",
            "mode": "records_only",
            "created": False,
        }
    finally:
        conn.close()


def test_branch_confirm_handler_registered() -> None:
    assert resolve_pg_handler("branch.confirm") is handle


def _ctx(conn: Any, repo_root: Path, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "admin", "allowed"),
    )


def _seed_run(conn: Any, repo_root: Path, *, repository_id: str, run_state: str) -> None:
    now = "2026-05-14T00:00:00Z"
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"author": {}},
        "lanes": {"local": {}},
        "jobs": [],
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
              repository_id, run_id, workflow_snapshot_id, repo_root, state, created_at
            )
            VALUES (%s, 'run_1', 'snap_1', %s, %s, %s)
            """,
            (repository_id, str(repo_root), run_state, now),
        )


def _events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            "SELECT * FROM striatumd.events WHERE repository_id = %s ORDER BY event_id",
            (repository_id,),
        )
        return [dict(row) for row in cur.fetchall()]


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)

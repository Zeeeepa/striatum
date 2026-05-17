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


def test_run_events_lists_after_cursor_and_scopes_repository(pg_conn: Any, tmp_path: Path) -> None:
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
            INSERT INTO striatumd.events(
              repository_id, event_id, run_id, event_type, actor_session_id,
              job_id, payload_json, created_at
            )
            VALUES
              ('repo_a', 10, 'run_1', 'demo.before', 'sess_1', 'job_draft', %s, %s),
              ('repo_a', 11, 'run_1', 'demo.after', 'sess_1', 'job_draft', %s, %s),
              ('repo_b', 12, 'run_1', 'demo.wrong_repo', 'sess_1', 'job_draft', %s, %s)
            """,
            (
                Jsonb({"i": 0}),
                now,
                Jsonb({"i": 1}),
                now,
                Jsonb({"i": 2}),
                now,
            ),
        )
    pg_conn.commit()
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_events")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"run_id": "run_1", "since_event_id": 10, "limit": 100},
    )

    assert result["run"] == {"run_id": "run_1", "state": "running"}
    assert result["count"] == 1
    assert result["terminal"] is False
    assert result["events"] == [
        {
            "event_id": 11,
            "run_id": "run_1",
            "type": "demo.after",
            "actor_session_id": "sess_1",
            "job_id": "job_draft",
            "payload": {"i": 1},
            "created_at": result["events"][0]["created_at"],
        }
    ]


def test_run_events_reports_terminal_run_and_rejects_bad_params(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo)
    with pg_conn.cursor() as cur:
        cur.execute(
            "UPDATE striatumd.runs SET state = 'completed' WHERE repository_id = 'repo_a' AND run_id = 'run_1'"
        )
    pg_conn.commit()
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_events")
    ctx = repo_context(pg_conn, repository_id="repo_a", repo_root=repo)

    result = module.handle(ctx, {"run_id": "run_1"})

    assert result["events"] == []
    assert result["terminal"] is True
    with pytest.raises(RpcError, match="run not found"):
        module.handle(ctx, {"run_id": "missing"})
    with pytest.raises(RpcError, match="since_event_id must be"):
        module.handle(ctx, {"run_id": "run_1", "since_event_id": -1})

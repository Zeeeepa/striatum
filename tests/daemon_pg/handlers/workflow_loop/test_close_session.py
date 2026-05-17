from __future__ import annotations

import importlib.util
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import pytest

from _harness.tokens import issue_token

_HELPERS_PATH = Path(__file__).with_name("test_register_session.py")
_HELPERS_SPEC = importlib.util.spec_from_file_location("_workflow_loop_test_helpers", _HELPERS_PATH)
if _HELPERS_SPEC is None or _HELPERS_SPEC.loader is None:
    raise RuntimeError(f"could not load helper module from {_HELPERS_PATH}")
_helpers = importlib.util.module_from_spec(_HELPERS_SPEC)
_HELPERS_SPEC.loader.exec_module(_helpers)

fetch_events = _helpers.fetch_events
insert_repo = _helpers.insert_repo
insert_run = _helpers.insert_run
load_pg_handler = _helpers.load_pg_handler
pg_conn = _helpers.pg_conn
repo_context = _helpers.repo_context
rpc = _helpers.rpc

pytestmark = pytest.mark.multi_repo


def test_close_session_closes_active_session_and_preserves_repo_scope(
    pg_conn: Any,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    handler, context_mod = load_pg_handler(monkeypatch, "close_session")
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_run(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_run(pg_conn, repository_id="repo_b", repo_root=repo_b)
    _insert_session(pg_conn, repository_id="repo_a")
    _insert_session(pg_conn, repository_id="repo_b")

    result = handler.handle(
        repo_context(context_mod, pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"session_id": "sess_author", "reason": "terminal_run_cleanup"},
    )

    assert result["state"] == "closed"
    assert result["close_reason"] == "terminal_run_cleanup"
    with pg_conn.cursor() as cur:
        cur.execute(
            "SELECT state, close_reason FROM striatumd.sessions WHERE repository_id = %s AND session_id = %s",
            ("repo_a", "sess_author"),
        )
        assert cur.fetchone() == ("closed", "terminal_run_cleanup")
        cur.execute(
            "SELECT state, close_reason FROM striatumd.sessions WHERE repository_id = %s AND session_id = %s",
            ("repo_b", "sess_author"),
        )
        assert cur.fetchone() == ("active", None)

    events = fetch_events(pg_conn, "repo_a")
    assert [row["event_type"] for row in events] == ["session.closed"]
    assert events[0]["payload_json"]["source"] == "explicit"


def test_close_session_refuses_active_lease(
    pg_conn: Any,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    handler, context_mod = load_pg_handler(monkeypatch, "close_session")
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_run(pg_conn, repository_id="repo_a", repo_root=repo)
    _insert_session(pg_conn, repository_id="repo_a")
    _insert_active_lease(pg_conn, repository_id="repo_a")

    with pytest.raises(Exception, match=r"active lease"):
        handler.handle(
            repo_context(context_mod, pg_conn, repository_id="repo_a", repo_root=repo),
            {"session_id": "sess_author", "reason": "done"},
        )


def test_close_session_requires_claim_capability_before_route(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_run(pg_conn, repository_id="repo_a", repo_root=repo)
    _insert_session(pg_conn, repository_id="repo_a")
    token = issue_token(pg_conn, capabilities=["read"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="session.close",
        params={"repository_id": "repo_a", "session_id": "sess_author", "reason": "done"},
        token=token,
        request_id="close-denied",
    )

    assert response.ok is False
    assert response.data["code"] == "capability_missing"
    with pg_conn.cursor() as cur:
        cur.execute("SELECT state FROM striatumd.sessions WHERE repository_id = 'repo_a'")
        assert cur.fetchone()[0] == "active"


def test_close_session_routes_through_native_pg_handler(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_run(pg_conn, repository_id="repo_a", repo_root=repo)
    _insert_session(pg_conn, repository_id="repo_a")
    token = issue_token(pg_conn, capabilities=["claim"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="session.close",
        params={"repository_id": "repo_a", "session_id": "sess_author", "reason": "done"},
        token=token,
        request_id="close-native",
    )

    assert response.ok is True
    assert response.data["state"] == "closed"


def _insert_session(conn: Any, *, repository_id: str) -> None:
    now = datetime.now(UTC)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.sessions(
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, fresh_context, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, 'sess_author', 'run_1', 'author', 'codex', 'author-codex-1', 1,
                    '["claim"]'::jsonb, false, 'active', %s, %s)
            """,
            (repository_id, now, now),
        )
    conn.commit()


def _insert_active_lease(conn: Any, *, repository_id: str) -> None:
    now = datetime.now(UTC)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.leases(
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
            )
            VALUES (%s, 'lease_1', 'run_1', 'job', 'job_1',
                    'sess_author', 'active', %s, %s, %s)
            """,
            (repository_id, now, now + timedelta(minutes=30), now),
        )
    conn.commit()

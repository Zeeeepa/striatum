from __future__ import annotations

import importlib.util
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
insert_claimable_work = _helpers.insert_claimable_work
insert_repo = _helpers.insert_repo
load_pg_handler = _helpers.load_pg_handler
pg_conn = _helpers.pg_conn
repo_context = _helpers.repo_context
rpc = _helpers.rpc

pytestmark = pytest.mark.multi_repo


def test_send_message_inserts_completed_agent_message_and_preserves_repo_scope(
    pg_conn: Any,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    handler, context_mod = load_pg_handler(monkeypatch, "send_message")
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_claimable_work(pg_conn, repository_id="repo_b", repo_root=repo_b)

    result = handler.handle(
        repo_context(context_mod, pg_conn, repository_id="repo_a", repo_root=repo_a),
        {
            "session_id": "sess_author",
            "kind": "note",
            "body_json": '{"summary":"working"}',
        },
    )

    assert result["message_id"].startswith("msg_")
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            SELECT kind, state, payload_json
            FROM striatumd.queue_messages
            WHERE repository_id = %s AND message_id = %s
            """,
            ("repo_a", result["message_id"]),
        )
        assert cur.fetchone() == ("agent_message", "completed", {"kind": "note", "body": {"summary": "working"}})
        cur.execute(
            """
            SELECT COUNT(*)
            FROM striatumd.queue_messages
            WHERE repository_id = %s AND kind = 'agent_message'
            """,
            ("repo_b",),
        )
        assert cur.fetchone()[0] == 0

    events = fetch_events(pg_conn, "repo_a")
    assert [row["event_type"] for row in events] == ["message.sent"]
    assert events[0]["message_id"] == result["message_id"]
    assert events[0]["payload_json"] == {"kind": "note"}


def test_send_message_rejects_non_object_body(
    pg_conn: Any,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    handler, context_mod = load_pg_handler(monkeypatch, "send_message")
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo)

    with pytest.raises(Exception, match="message body must be a JSON object"):
        handler.handle(
            repo_context(context_mod, pg_conn, repository_id="repo_a", repo_root=repo),
            {"session_id": "sess_author", "kind": "note", "body_json": "[]"},
        )


def test_send_message_requires_write_capability_before_route(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo)
    token = issue_token(pg_conn, capabilities=["claim"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="work.send_message",
        params={
            "repository_id": "repo_a",
            "session_id": "sess_author",
            "kind": "note",
            "body_json": '{"summary":"working"}',
        },
        token=token,
        request_id="send-denied",
    )

    assert response.ok is False
    assert response.data["code"] == "capability_missing"


def test_send_message_routes_through_native_pg_handler(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo)
    token = issue_token(pg_conn, capabilities=["write"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="work.send_message",
        params={
            "repository_id": "repo_a",
            "session_id": "sess_author",
            "kind": "note",
            "body_json": '{"summary":"working"}',
        },
        token=token,
        request_id="send-native",
    )

    assert response.ok is True
    assert response.data["message_id"].startswith("msg_")

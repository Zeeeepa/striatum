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
mark_work_claimed = _helpers.mark_work_claimed
pg_conn = _helpers.pg_conn
repo_context = _helpers.repo_context
rpc = _helpers.rpc

pytestmark = pytest.mark.multi_repo


def test_ack_work_scopes_state_and_extends_event_chain(
    pg_conn: Any,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    handler, context_mod = load_pg_handler(monkeypatch, "ack_work")
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    for repo_id, repo_root in (("repo_a", repo_a), ("repo_b", repo_b)):
        insert_claimable_work(pg_conn, repository_id=repo_id, repo_root=repo_root)
        mark_work_claimed(pg_conn, repository_id=repo_id)

    ctx = repo_context(context_mod, pg_conn, repository_id="repo_a", repo_root=repo_a)
    ctx.append_event(run_id="run_1", event_type="test.seed")
    result = handler.handle(
        ctx,
        {
            "session_id": "sess_author",
            "message_id": "msg_draft",
            "lease_id": "lease_draft",
        },
    )

    assert result == {"status": "acked", "job_id": "job_draft"}
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            SELECT m.state, j.state
            FROM striatumd.queue_messages m
            JOIN striatumd.jobs j
              ON j.repository_id = m.repository_id AND j.job_id = m.job_id
            WHERE m.repository_id = %s AND m.message_id = 'msg_draft'
            """,
            ("repo_a",),
        )
        assert cur.fetchone() == ("acked", "running")
        cur.execute(
            """
            SELECT m.state, j.state
            FROM striatumd.queue_messages m
            JOIN striatumd.jobs j
              ON j.repository_id = m.repository_id AND j.job_id = m.job_id
            WHERE m.repository_id = %s AND m.message_id = 'msg_draft'
            """,
            ("repo_b",),
        )
        assert cur.fetchone() == ("claimed", "claimed")

    events = fetch_events(pg_conn, "repo_a")
    assert [row["event_type"] for row in events] == ["test.seed", "queue.acked"]
    first_hash = context_mod.canonical_event_hash(events[0])
    chained_hash = context_mod.canonical_event_hash(events[1], previous_hash=first_hash)
    assert chained_hash != context_mod.canonical_event_hash(events[1])


def test_ack_work_requires_claim_capability_before_route(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo)
    mark_work_claimed(pg_conn, repository_id="repo_a")
    token = issue_token(pg_conn, capabilities=["read"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="work.ack",
        params={
            "repository_id": "repo_a",
            "session_id": "sess_author",
            "message_id": "msg_draft",
            "lease_id": "lease_draft",
        },
        token=token,
        request_id="ack-denied",
    )

    assert response.ok is False
    assert response.data["code"] == "capability_missing"
    with pg_conn.cursor() as cur:
        cur.execute(
            "SELECT state FROM striatumd.queue_messages WHERE repository_id = 'repo_a' AND message_id = 'msg_draft'"
        )
        assert cur.fetchone()[0] == "claimed"

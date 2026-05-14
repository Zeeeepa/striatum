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
pg_conn = _helpers.pg_conn
rpc = _helpers.rpc

pytestmark = pytest.mark.multi_repo


def test_claim_next_public_route_claims_pg_work_packet(
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
        method="work.claim_next",
        params={"repository_id": "repo_a", "session_id": "sess_author"},
        token=token,
        request_id="claim-next-ok",
    )

    assert response.ok is True
    assert response.data["status"] == "claimed"
    packet = response.data["packet"]
    assert packet["session"]["session_id"] == "sess_author"
    assert packet["job"]["job_id"] == "job_draft"
    assert packet["lease"]["message_id"] == "msg_draft"
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            SELECT m.state, j.state, l.state
            FROM striatumd.queue_messages m
            JOIN striatumd.jobs j
              ON j.repository_id = m.repository_id AND j.job_id = m.job_id
            JOIN striatumd.leases l
              ON l.repository_id = m.repository_id AND l.lease_id = m.current_lease_id
            WHERE m.repository_id = 'repo_a' AND m.message_id = 'msg_draft'
            """
        )
        assert cur.fetchone() == ("claimed", "claimed", "active")
    events = fetch_events(pg_conn, "repo_a")
    assert [row["event_type"] for row in events] == ["queue.claimed"]


def test_claim_next_requires_claim_capability_before_route(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo)
    token = issue_token(pg_conn, capabilities=["read"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="work.claim_next",
        params={"repository_id": "repo_a", "session_id": "sess_author"},
        token=token,
        request_id="claim-next-denied",
    )

    assert response.ok is False
    assert response.data["code"] == "capability_missing"
    with pg_conn.cursor() as cur:
        cur.execute(
            "SELECT state FROM striatumd.queue_messages WHERE repository_id = 'repo_a' AND message_id = 'msg_draft'"
        )
        assert cur.fetchone()[0] == "pending"

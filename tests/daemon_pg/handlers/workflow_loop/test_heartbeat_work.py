from __future__ import annotations

import importlib.util
from datetime import UTC, datetime
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


def test_heartbeat_extends_lease_and_preserves_repo_scope(
    pg_conn: Any,
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    handler, context_mod = load_pg_handler(monkeypatch, "heartbeat_work")
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    for repo_id, repo_root in (("repo_a", repo_a), ("repo_b", repo_b)):
        insert_claimable_work(pg_conn, repository_id=repo_id, repo_root=repo_root)
        mark_work_claimed(pg_conn, repository_id=repo_id)

    result = handler.handle(
        repo_context(context_mod, pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"session_id": "sess_author", "lease_id": "lease_draft", "extend_seconds": 2400},
    )

    assert result["status"] == "heartbeat"
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            SELECT s.last_heartbeat_at, l.last_heartbeat_at, l.expires_at
            FROM striatumd.sessions s
            JOIN striatumd.leases l
              ON l.repository_id = s.repository_id AND l.owner_session_id = s.session_id
            WHERE s.repository_id = %s AND s.session_id = 'sess_author'
            """,
            ("repo_a",),
        )
        repo_a_row = cur.fetchone()
        assert repo_a_row[0] == repo_a_row[1]
        assert _iso_utc(repo_a_row[2]) == result["expires_at"]
        cur.execute(
            """
            SELECT s.last_heartbeat_at, l.last_heartbeat_at
            FROM striatumd.sessions s
            JOIN striatumd.leases l
              ON l.repository_id = s.repository_id AND l.owner_session_id = s.session_id
            WHERE s.repository_id = %s AND s.session_id = 'sess_author'
            """,
            ("repo_b",),
        )
        assert cur.fetchone() != repo_a_row[:2]

    events = fetch_events(pg_conn, "repo_a")
    assert [row["event_type"] for row in events] == ["lease.heartbeat"]
    assert events[0]["payload_json"] == {"expires_at": result["expires_at"]}


def test_heartbeat_requires_claim_capability_before_route(
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
        method="work.heartbeat",
        params={
            "repository_id": "repo_a",
            "session_id": "sess_author",
            "lease_id": "lease_draft",
            "extend_seconds": 60,
        },
        token=token,
        request_id="heartbeat-denied",
    )

    assert response.ok is False
    assert response.data["code"] == "capability_missing"


def test_heartbeat_routes_through_native_pg_handler(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo)
    mark_work_claimed(pg_conn, repository_id="repo_a")
    token = issue_token(pg_conn, capabilities=["claim"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="work.heartbeat",
        params={
            "repository_id": "repo_a",
            "session_id": "sess_author",
            "lease_id": "lease_draft",
            "extend_seconds": 60,
        },
        token=token,
        request_id="heartbeat-native",
    )

    assert response.ok is True
    assert response.data["status"] == "heartbeat"


def _iso_utc(value: Any) -> str:
    if isinstance(value, datetime):
        return value.astimezone(UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    return str(value)

from __future__ import annotations

import importlib.util
import os
import threading
from pathlib import Path
from typing import Any

import pytest
from psycopg.types.json import Jsonb

from _harness.tokens import issue_token
from striatum.identity import process_start_time

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


def test_claim_next_auto_delivery_honors_one_shot_eof_metadata(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    pipe_path = tmp_path / "stdin.pipe"
    os.mkfifo(pipe_path, 0o600)
    received: list[bytes] = []

    def read_packet() -> None:
        with pipe_path.open("rb", buffering=0) as fifo:
            received.append(fifo.read())

    reader = threading.Thread(target=read_packet, daemon=True)
    reader.start()

    insert_repo(pg_conn, repo, "repo_a")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo)
    _insert_attached_supervisor(
        pg_conn,
        repository_id="repo_a",
        repo_root=repo,
        pipe_path=pipe_path,
        stdin_delivery="one_shot_eof",
    )
    token = issue_token(pg_conn, capabilities=["claim"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="work.claim_next",
        params={"repository_id": "repo_a", "session_id": "sess_author"},
        token=token,
        request_id="claim-next-one-shot-delivery",
    )

    reader.join(timeout=5)
    assert response.ok is True
    delivery = response.data["supervisor_delivery"]
    assert delivery["stdin_delivery"] == "one_shot_eof"
    assert delivery["stdin_closed_after_write"] is True
    assert not pipe_path.exists()
    assert received
    assert b'"packet_id":"wp_' in received[0]
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            SELECT metadata_json
            FROM striatumd.process_supervisor_pointers
            WHERE repository_id = 'repo_a' AND supervisor_id = 'sup_one_shot'
            """
        )
        metadata = cur.fetchone()[0]
    assert metadata["stdin_delivery"] == "one_shot_eof"
    assert metadata["stdin_delivery_consumed"] is True
    events = fetch_events(pg_conn, "repo_a")
    delivered = next(row for row in events if row["event_type"] == "supervisor.packet_delivered")
    assert delivered["payload_json"]["stdin_delivery"] == "one_shot_eof"
    assert delivered["payload_json"]["stdin_closed_after_write"] is True


def test_claim_next_auto_delivery_marks_stale_pid_identity_lost(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    repo = tmp_path / "repo"
    pipe_path = tmp_path / "stdin.pipe"
    os.mkfifo(pipe_path, 0o600)
    insert_repo(pg_conn, repo, "repo_a")
    insert_claimable_work(pg_conn, repository_id="repo_a", repo_root=repo)
    _insert_attached_supervisor(
        pg_conn,
        repository_id="repo_a",
        repo_root=repo,
        pipe_path=pipe_path,
        stdin_delivery="persistent_fifo",
        pid_start_time="stale-start-token",
    )
    token = issue_token(pg_conn, capabilities=["claim"], repo_id="repo_a")

    response = rpc(
        pg_conn,
        repo_root=repo,
        method="work.claim_next",
        params={"repository_id": "repo_a", "session_id": "sess_author"},
        token=token,
        request_id="claim-next-stale-pid-delivery",
    )

    assert response.ok is True
    delivery = response.data["supervisor_delivery"]
    assert delivery == {
        "supervisor_id": "sup_one_shot",
        "error": "supervisor_requires_reconciliation",
        "reason": "pid_identity_mismatch",
    }
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            SELECT ps.state, p.state AS pointer_state, ds.state AS daemon_state
            FROM striatumd.process_supervisors ps
            JOIN striatumd.process_supervisor_pointers p
              ON p.repository_id = ps.repository_id
             AND p.supervisor_id = ps.supervisor_id
            JOIN striatumd.daemon_supervisors ds
              ON ds.repository_id = p.repository_id
             AND ds.daemon_supervisor_id = p.daemon_supervisor_id
            WHERE ps.repository_id = 'repo_a' AND ps.supervisor_id = 'sup_one_shot'
            """
        )
        assert cur.fetchone() == ("lost", "lost", "lost")
    events = fetch_events(pg_conn, "repo_a")
    assert [row["event_type"] for row in events] == [
        "queue.claimed",
        "supervisor.lost",
    ]
    lost = events[1]["payload_json"]
    assert lost["phase"] == "claim_next_auto_delivery"
    assert lost["reattach_reason"] == "pid_identity_mismatch"


def _insert_attached_supervisor(
    conn: Any,
    *,
    repository_id: str,
    repo_root: Path,
    pipe_path: Path,
    stdin_delivery: str,
    pid_start_time: str | None = None,
) -> None:
    pid = os.getpid()
    pid_start_time = pid_start_time or process_start_time(pid)
    assert pid_start_time is not None
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.process_supervisors(
              repository_id, supervisor_id, run_id, session_id, adapter,
              command_json, cwd, scratch_path, stdin_pipe_path, pid,
              pid_start_time, state, started_at, heartbeat_at
            )
            VALUES (
              %s, 'sup_one_shot', 'run_1', 'sess_author', 'process',
              '["python"]'::jsonb, %s, %s, %s, %s, %s,
              'attached', now(), now()
            )
            """,
            (repository_id, str(repo_root), str(pipe_path.parent), str(pipe_path), pid, pid_start_time),
        )
        cur.execute(
            """
            INSERT INTO striatumd.process_supervisor_pointers(
              repository_id, supervisor_id, daemon_supervisor_id, run_id,
              session_id, pid, pid_start_time, state, updated_at, metadata_json
            )
            VALUES (
              %s, 'sup_one_shot', 'dsup_one_shot', 'run_1', 'sess_author',
              %s, %s, 'attached', now(), %s
            )
            """,
            (
                repository_id,
                pid,
                pid_start_time,
                Jsonb({"source": "test", "transport": "pipe", "stdin_delivery": stdin_delivery}),
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.daemon_supervisors(
              daemon_supervisor_id, repository_id, run_id, session_id,
              repo_supervisor_id, daemon_instance_id, adapter, command_json,
              command_sha256, cwd, stdin_pipe_path, pid, pid_start_time,
              state, started_at, heartbeat_at
            )
            VALUES (
              'dsup_one_shot', %s, 'run_1', 'sess_author',
              'sup_one_shot', 'daemon-test', 'process', '["python"]'::jsonb,
              'sha256:test', %s, %s, %s, %s, 'attached', now(), now()
            )
            """,
            (repository_id, str(repo_root), str(pipe_path), pid, pid_start_time),
        )
    conn.commit()

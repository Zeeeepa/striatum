from __future__ import annotations

import os
import signal
import sys
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
from striatum.daemon_pg.handlers.supervision import (
    handle_list,
    handle_report,
    handle_send,
    handle_start,
    handle_status,
    handle_stop,
)
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.errors import InvalidTransitionError

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_supervision_methods_register() -> None:
    for method in (
        "supervise.start",
        "supervise.send",
        "supervise.report",
        "supervise.stop",
        "supervise.status",
        "supervise.list",
    ):
        assert resolve_pg_handler(method) is not None


def test_start_status_list_stop_preserve_repo_scope(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    started: dict[str, Any] | None = None
    try:
        repo_a = tmp_path / "repo_a"
        repo_b = tmp_path / "repo_b"
        _seed_supervisable_session(conn, repo_a, repository_id="repo_a")
        _seed_supervisable_session(conn, repo_b, repository_id="repo_b")
        conn.commit()

        started = handle_start(_ctx(conn, repo_a, "repo_a"), {"session_id": "sess_1"})

        assert started["state"] == "attached"
        assert started["pid"] > 0
        assert Path(str(started["stdin_pipe_path"])).exists()
        assert _one(
            conn,
            "SELECT state, session_id, run_id FROM striatumd.process_supervisors "
            "WHERE repository_id = %s",
            ("repo_a",),
        ) == {"state": "attached", "session_id": "sess_1", "run_id": "run_1"}
        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.process_supervisors "
            "WHERE repository_id = %s",
            ("repo_b",),
        ) == {"count": 0}
        assert _one(
            conn,
            "SELECT state, daemon_supervisor_id FROM striatumd.process_supervisor_pointers "
            "WHERE repository_id = %s AND supervisor_id = %s",
            ("repo_a", started["supervisor_id"]),
        ) == {
            "state": "attached",
            "daemon_supervisor_id": started["daemon_supervisor_id"],
        }

        status = handle_status(_ctx(conn, repo_a, "repo_a"), {"session_id": "sess_1"})
        assert status["state"] == "attached"
        assert status["liveness"] == "alive"

        listed = handle_list(_ctx(conn, repo_a, "repo_a"), {"run_id": "run_1"})
        assert [row["supervisor_id"] for row in listed["supervisors"]] == [
            started["supervisor_id"]
        ]

        stopped = handle_stop(
            _ctx(conn, repo_a, "repo_a"),
            {"session_id": "sess_1", "reason": "test complete"},
        )
        started = None
        assert stopped["state"] == "stopped"
        assert stopped["stop_reason"] == "test complete"
        assert not Path(str(status["stdin_pipe_path"])).exists()
        assert _one(
            conn,
            "SELECT state, stop_reason FROM striatumd.process_supervisors "
            "WHERE repository_id = %s AND supervisor_id = %s",
            ("repo_a", stopped["supervisor_id"]),
        ) == {"state": "stopped", "stop_reason": "test complete"}
        assert _one(
            conn,
            "SELECT state FROM striatumd.process_supervisor_pointers "
            "WHERE repository_id = %s AND supervisor_id = %s",
            ("repo_a", stopped["supervisor_id"]),
        ) == {"state": "stopped"}
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "supervisor.starting",
            "supervisor.started",
            "supervisor.stopped",
        ]
    finally:
        if started is not None:
            _kill_pid(started.get("pid"))
        conn.close()


def test_stop_is_idempotent_for_terminal_supervisor(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    started: dict[str, Any] | None = None
    try:
        repo = tmp_path / "repo"
        _seed_supervisable_session(conn, repo, repository_id="repo_a")
        conn.commit()
        started = handle_start(_ctx(conn, repo, "repo_a"), {"session_id": "sess_1"})
        first = handle_stop(
            _ctx(conn, repo, "repo_a"),
            {"session_id": "sess_1", "reason": "first stop"},
        )
        started = None

        second = handle_stop(
            _ctx(conn, repo, "repo_a"),
            {"session_id": "sess_1", "reason": "second stop"},
        )

        assert first["state"] == "stopped"
        assert second["state"] == "stopped"
        assert second["note"] == "supervisor was already stopped"
        assert second["stop_reason"] == "first stop"
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "supervisor.starting",
            "supervisor.started",
            "supervisor.stopped",
        ]
    finally:
        if started is not None:
            _kill_pid(started.get("pid"))
        conn.close()


def test_report_records_wrapper_control_events_without_reading_output(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    started: dict[str, Any] | None = None
    try:
        repo = tmp_path / "repo"
        _seed_supervisable_session(conn, repo, repository_id="repo_a")
        _seed_work_packet(
            conn,
            repository_id="repo_a",
            session_id="sess_1",
            packet_id="wp_1",
        )
        conn.commit()
        started = handle_start(_ctx(conn, repo, "repo_a"), {"session_id": "sess_1"})

        accepted = handle_report(
            _ctx(conn, repo, "repo_a"),
            {
                "session_id": "sess_1",
                "event_type": "packet_accepted",
                "packet_id": "wp_1",
                "payload": {"wrapper_pid": 1234},
            },
        )
        assert accepted["state"] == "attached"
        assert accepted["event_type"] == "packet_accepted"

        exited = handle_report(
            _ctx(conn, repo, "repo_a"),
            {
                "supervisor_id": started["supervisor_id"],
                "event_type": "agent_exited",
                "payload": {"exit_code": 0},
            },
        )

        assert exited["state"] == "stopped"
        assert _one(
            conn,
            "SELECT state, stop_reason FROM striatumd.process_supervisors "
            "WHERE repository_id = %s AND supervisor_id = %s",
            ("repo_a", started["supervisor_id"]),
        ) == {"state": "stopped", "stop_reason": "agent exited with code 0"}
        events = _events(conn, "repo_a")
        assert [row["event_type"] for row in events] == [
            "supervisor.starting",
            "supervisor.started",
            "supervisor.packet_accepted",
            "supervisor.agent_exited",
        ]
        assert events[2]["payload_json"]["payload"] == {"wrapper_pid": 1234}
        assert events[3]["payload_json"]["payload"] == {"exit_code": 0}
    finally:
        if started is not None:
            _kill_pid(started.get("pid"))
        conn.close()


def test_send_fails_closed_for_foreign_packet(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    started: dict[str, Any] | None = None
    try:
        repo = tmp_path / "repo"
        _seed_supervisable_session(conn, repo, repository_id="repo_a")
        _seed_session(conn, repository_id="repo_a", session_id="sess_other")
        _seed_work_packet(
            conn,
            repository_id="repo_a",
            session_id="sess_other",
            packet_id="wp_other",
        )
        conn.commit()
        started = handle_start(_ctx(conn, repo, "repo_a"), {"session_id": "sess_1"})

        with pytest.raises(InvalidTransitionError, match="does not belong"):
            handle_send(
                _ctx(conn, repo, "repo_a"),
                {"session_id": "sess_1", "packet_id": "wp_other"},
            )

        assert _one(
            conn,
            "SELECT state FROM striatumd.process_supervisors "
            "WHERE repository_id = %s AND supervisor_id = %s",
            ("repo_a", started["supervisor_id"]),
        ) == {"state": "attached"}
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "supervisor.starting",
            "supervisor.started",
        ]
    finally:
        if started is not None:
            try:
                handle_stop(
                    _ctx(conn, repo, "repo_a"),
                    {"session_id": "sess_1", "reason": "cleanup"},
                )
            except Exception:
                _kill_pid(started.get("pid"))
        conn.close()


def test_start_rejects_non_process_lane(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        repo = tmp_path / "repo"
        _seed_supervisable_session(conn, repo, repository_id="repo_a", adapter="manual")
        conn.commit()

        with pytest.raises(InvalidTransitionError, match="process-adapter lane"):
            handle_start(_ctx(conn, repo, "repo_a"), {"session_id": "sess_1"})

        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.process_supervisors "
            "WHERE repository_id = %s",
            ("repo_a",),
        ) == {"count": 0}
    finally:
        conn.close()


def _seed_supervisable_session(
    conn: Any,
    repo_root: Path,
    *,
    repository_id: str,
    adapter: str = "process",
) -> None:
    repo_root.mkdir(parents=True, exist_ok=True)
    now = "2026-05-16T00:00:00Z"
    command = _supervised_command() if adapter == "process" else []
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "test-workflow",
        "workflow_version": "1",
        "name": "Test workflow",
        "branch": {"mode": "manual"},
        "coordinator": {"role_id": "author", "lane_id": "codex"},
        "lanes": {
            "codex": {
                "adapter": adapter,
                "command": command,
                "display_model": "codex-gpt-5",
            }
        },
        "roles": {"author": {"summary": "Author"}},
        "context_docs": [],
        "parallelism": {"mode": "serial", "max_active_jobs": 1},
        "jobs": [{"id": "draft", "type": "draft", "role": "author", "lane_id": "codex"}],
        "edges": [],
        "cycles": [],
    }
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path, display_name,
              registered_at, last_seen_at, last_schema_version, state, settings_json
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, 5, 'active', '{}'::jsonb)
            """,
            (
                repository_id,
                f"identity:{repository_id}",
                str(repo_root.resolve()),
                str((repo_root / ".striatum" / "state.sqlite3").resolve()),
                repository_id,
                now,
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots(
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              source_path, content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, 'snap_1', 'test-workflow', '1', 'workflow.json',
                    'sha256:test', %s, %s)
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
            VALUES (%s, 'run_1', 'snap_1', %s, 'running', 'main', 'main',
                    %s, 'operator', %s, %s)
            """,
            (repository_id, str(repo_root.resolve()), now, now, now),
        )
    _seed_session(conn, repository_id=repository_id, session_id="sess_1")


def _seed_session(conn: Any, *, repository_id: str, session_id: str) -> None:
    now = "2026-05-16T00:00:00Z"
    ordinal = 1 if session_id == "sess_1" else 2
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.sessions(
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, fresh_context, state, registered_at,
              last_heartbeat_at
            )
            VALUES (%s, %s, 'run_1', 'author', 'codex', %s, %s,
                    '["claim"]'::jsonb, false, 'active', %s, %s)
            """,
            (repository_id, session_id, f"author-codex-{session_id}", ordinal, now, now),
        )


def _seed_work_packet(
    conn: Any,
    *,
    repository_id: str,
    session_id: str,
    packet_id: str,
) -> None:
    now = "2026-05-16T00:00:00Z"
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.jobs(
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              attempt, max_attempts, fresh_session_required, write_scope_json,
              expected_artifacts_json, idempotency_key, created_at, ready_at
            )
            VALUES (%s, 'job_1', 'run_1', 'draft', 'Draft', 'draft',
                    'author', '{"lane_id":"codex"}'::jsonb, '[]'::jsonb,
                    'claimed', 1, 1, false, '{}'::jsonb, '[]'::jsonb,
                    'draft:1', %s, %s)
            """,
            (repository_id, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages(
              repository_id, message_id, run_id, job_id, kind, state,
              priority, target_session_id, target_role_id, target_lane_id,
              payload_json, claim_count, max_claims, created_at, updated_at
            )
            VALUES (%s, 'msg_1', 'run_1', 'job_1', 'work', 'claimed',
                    0, %s, 'author', 'codex', '{}'::jsonb, 1, 1, %s, %s)
            """,
            (repository_id, session_id, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases(
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at,
              last_heartbeat_at
            )
            VALUES (%s, 'lease_1', 'run_1', 'job', 'job_1', %s, 'active',
                    %s, '2026-05-16T01:00:00Z', %s)
            """,
            (repository_id, session_id, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.work_packets(
              repository_id, packet_id, run_id, job_id, message_id, lease_id,
              session_id, packet_json, packet_sha256, created_at
            )
            VALUES (%s, %s, 'run_1', 'job_1', 'msg_1', 'lease_1',
                    %s, %s, 'sha256:packet', %s)
            """,
            (
                repository_id,
                packet_id,
                session_id,
                Jsonb({"packet_id": packet_id, "session_id": session_id}),
                now,
            ),
        )


def _supervised_command() -> list[str]:
    script = (
        "import signal, time\n"
        "signal.signal(signal.SIGTERM, lambda *_: raise_system_exit())\n"
        "def raise_system_exit():\n"
        "    raise SystemExit(0)\n"
        "time.sleep(60)\n"
    )
    return [sys.executable, "-c", script]


def _ctx(conn: Any, repo_root: Path, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "claim", "allowed"),
    )


def _one(conn: Any, sql: str, params: tuple[Any, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, params)
        row = cur.fetchone()
    assert row is not None
    return dict(row)


def _events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            SELECT event_type, payload_json
            FROM striatumd.events
            WHERE repository_id = %s
            ORDER BY event_id
            """,
            (repository_id,),
        )
        return [dict(row) for row in cur.fetchall()]


def _kill_pid(value: object) -> None:
    if value is None:
        return
    if not isinstance(value, int):
        return
    try:
        os.kill(value, signal.SIGKILL)
    except OSError:
        return

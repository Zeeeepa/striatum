from __future__ import annotations

from collections.abc import Iterator, Mapping
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext, canonical_event_hash
from striatum.daemon_pg.handlers.workflow_loop.block_job import handle
from striatum.daemon_rpc.capability import RpcAuthContext


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_block_job_moves_running_work_to_blocked_and_appends_event(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_claimed_work(conn, tmp_path, repository_id="repo_a")
        _seed_claimed_work(conn, tmp_path, repository_id="repo_b")
        seed_event_id = _append_seed_event(conn, repository_id="repo_a")
        conn.commit()
        ctx = _ctx(conn, tmp_path, repository_id="repo_a")

        result = handle(
            ctx,
            {
                "session_id": "sess_1",
                "job_id": "job_1",
                "lease_id": "lease_1",
                "kind": "needs_operator",
                "severity": "blocked",
                "description": "blocked for test",
            },
        )

        assert result["status"] == "blocked"
        assert isinstance(result["blocker_id"], str)
        assert _states(conn, repository_id="repo_a") == {
            "job": "blocked",
            "job_lease": None,
            "message": "blocked",
            "message_lease": None,
            "lease": "released",
            "lease_reason": "blocked",
        }
        assert _states(conn, repository_id="repo_b") == {
            "job": "running",
            "job_lease": "lease_1",
            "message": "acked",
            "message_lease": "lease_1",
            "lease": "active",
            "lease_reason": None,
        }
        blocker = _one(
            conn,
            "SELECT blocker_kind, description, severity, state FROM striatumd.blockers "
            "WHERE repository_id = %s",
            ("repo_a",),
        )
        assert blocker == {
            "blocker_kind": "needs_operator",
            "description": "blocked for test",
            "severity": "blocked",
            "state": "open",
        }
        events = _events(conn, repository_id="repo_a")
        assert [row["event_type"] for row in events] == ["seed.event", "job.blocked"]
        assert events[-1]["event_id"] > seed_event_id
        assert events[-1]["payload_json"] == {
            "blocker_id": result["blocker_id"],
            "severity": "blocked",
        }
        seed_hash = canonical_event_hash(events[0])
        blocked_hash = canonical_event_hash(events[-1], previous_hash=seed_hash)
        assert blocked_hash != seed_hash
    finally:
        conn.close()


def test_block_job_human_checkpoint_moves_job_to_waiting_human(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        _seed_claimed_work(conn, tmp_path, repository_id="repo_a")
        conn.commit()

        handle(
            _ctx(conn, tmp_path, repository_id="repo_a"),
            {
                "session_id": "sess_1",
                "job_id": "job_1",
                "lease_id": "lease_1",
                "kind": "needs_human",
                "severity": "human_checkpoint",
                "description": "needs principal decision",
            },
        )

        assert _one(
            conn,
            "SELECT state FROM striatumd.jobs WHERE repository_id = %s AND job_id = %s",
            ("repo_a", "job_1"),
        ) == {"state": "waiting_human"}
    finally:
        conn.close()


def _ctx(conn: Any, tmp_path: Path, *, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=tmp_path / repository_id,
        auth=RpcAuthContext("client", "token", repository_id, "write", "allowed"),
    )


def _seed_claimed_work(conn: Any, tmp_path: Path, *, repository_id: str) -> None:
    now = "2026-05-14T00:00:00Z"
    repo_root = tmp_path / repository_id
    repo_root.mkdir()
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"author": {}},
        "lanes": {"local": {}},
        "jobs": [{"id": "draft", "type": "draft"}],
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
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              created_at, started_at
            )
            VALUES (%s, 'run_1', 'snap_1', %s, 'running', %s, %s)
            """,
            (repository_id, str(repo_root), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions (
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, 'sess_1', 'run_1', 'author', 'local', 'author-local-1', 1,
                    %s, 'active', %s, %s)
            """,
            (repository_id, Jsonb(["write"]), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at, started_at, current_message_id, current_lease_id
            )
            VALUES (%s, 'job_1', 'run_1', 'draft', 'Draft', 'draft', 'author',
                    %s, %s, 'running', %s, %s, 'draft-1', %s, %s,
                    'msg_1', 'lease_1')
            """,
            (
                repository_id,
                Jsonb({"lane_id": "local"}),
                Jsonb({}),
                Jsonb({"repo_write": False}),
                Jsonb([]),
                now,
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages (
              repository_id, message_id, run_id, job_id, kind, state, target_role_id,
              payload_json, created_at, updated_at, claimed_at, acked_at,
              current_lease_id
            )
            VALUES (%s, 'msg_1', 'run_1', 'job_1', 'work', 'acked', 'author',
                    %s, %s, %s, %s, %s, 'lease_1')
            """,
            (repository_id, Jsonb({}), now, now, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases (
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at
            )
            VALUES (%s, 'lease_1', 'run_1', 'job', 'job_1', 'sess_1',
                    'active', %s, '2099-01-01T00:00:00Z')
            """,
            (repository_id, now),
        )


def _append_seed_event(conn: Any, *, repository_id: str) -> int:
    payload = {"seed": True}
    with conn.cursor() as cur:
        cur.execute(
            "SELECT nextval(pg_get_serial_sequence('striatumd.events', 'event_id'))"
        )
        event_id = int(cur.fetchone()[0])
        row_hash = canonical_event_hash(
            {
                "repository_id": repository_id,
                "event_id": event_id,
                "run_id": "run_1",
                "event_type": "seed.event",
                "payload_json": payload,
                "created_at": "2026-05-14T00:00:00Z",
            },
            previous_hash=None,
        )
        cur.execute(
            """
            INSERT INTO striatumd.events (
              repository_id, event_id, run_id, event_type, payload_json,
              created_at, previous_hash, row_hash
            )
            VALUES (%s, %s, 'run_1', 'seed.event', %s,
                    '2026-05-14T00:00:00Z', NULL, %s)
            """,
            (repository_id, event_id, Jsonb(payload), row_hash),
        )
        cur.execute(
            """
            INSERT INTO striatumd.repo_event_chain_heads(
              repository_id, last_event_id, last_hash, updated_at
            ) VALUES (%s, %s, %s, now())
            ON CONFLICT (repository_id) DO UPDATE SET
              last_event_id = EXCLUDED.last_event_id,
              last_hash = EXCLUDED.last_hash,
              updated_at = now()
            """,
            (repository_id, event_id, row_hash),
        )
        return event_id


def _states(conn: Any, *, repository_id: str) -> dict[str, Any]:
    job = _one(
        conn,
        "SELECT state, current_lease_id FROM striatumd.jobs "
        "WHERE repository_id = %s AND job_id = 'job_1'",
        (repository_id,),
    )
    message = _one(
        conn,
        "SELECT state, current_lease_id FROM striatumd.queue_messages "
        "WHERE repository_id = %s AND message_id = 'msg_1'",
        (repository_id,),
    )
    lease = _one(
        conn,
        "SELECT state, release_reason FROM striatumd.leases "
        "WHERE repository_id = %s AND lease_id = 'lease_1'",
        (repository_id,),
    )
    return {
        "job": job["state"],
        "job_lease": job["current_lease_id"],
        "message": message["state"],
        "message_lease": message["current_lease_id"],
        "lease": lease["state"],
        "lease_reason": lease["release_reason"],
    }


def _events(conn: Any, *, repository_id: str) -> list[Mapping[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            SELECT repository_id, event_id, run_id, event_type, actor_session_id,
                   job_id, message_id, artifact_id, lease_id, payload_json,
                   created_at
            FROM striatumd.events
            WHERE repository_id = %s
            ORDER BY event_id
            """,
            (repository_id,),
        )
        return list(cur.fetchall())


def _one(conn: Any, sql: str, params: tuple[Any, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, params)
        row = cur.fetchone()
    assert row is not None
    return dict(row)

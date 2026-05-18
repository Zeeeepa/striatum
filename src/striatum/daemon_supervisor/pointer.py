"""Repo-local pointer helpers for daemon-owned supervisors."""

from __future__ import annotations

import sqlite3
from typing import Any

from striatum.primitives import JsonObject, json_dumps, utc_now


def record_pointer(
    conn: sqlite3.Connection,
    *,
    supervisor_id: str,
    daemon_supervisor_id: str,
    run_id: str,
    session_id: str,
    pid: int,
    pid_start_time: str | None,
    state: str = "starting",
) -> JsonObject:
    if state == "attached" and pid_start_time is None:
        raise ValueError("daemon supervisor pointers cannot attach without pid_start_time verification")
    if state not in {"starting", "detached", "lost", "stopped"}:
        raise ValueError("daemon supervisor pointer state is not enabled before reattach verification")
    conn.execute(
        """
        INSERT INTO process_supervisor_pointers (
          supervisor_id, daemon_supervisor_id, run_id, session_id, pid,
          pid_start_time, state, updated_at, metadata_json
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(supervisor_id) DO UPDATE SET
          daemon_supervisor_id = excluded.daemon_supervisor_id,
          pid = excluded.pid,
          pid_start_time = excluded.pid_start_time,
          state = excluded.state,
          updated_at = excluded.updated_at,
          metadata_json = excluded.metadata_json
        """,
        (
            supervisor_id,
            daemon_supervisor_id,
            run_id,
            session_id,
            pid,
            pid_start_time,
            state,
            utc_now(),
            json_dumps({}),
        ),
    )
    return {
        "supervisor_id": supervisor_id,
        "daemon_supervisor_id": daemon_supervisor_id,
        "state": state,
    }


def pointer_for_session(conn: sqlite3.Connection, *, session_id: str) -> dict[str, Any] | None:
    row = conn.execute(
        """
        SELECT * FROM process_supervisor_pointers
        WHERE session_id = ? AND state IN ('starting','attached','detached')
        ORDER BY updated_at DESC
        LIMIT 1
        """,
        (session_id,),
    ).fetchone()
    return dict(row) if row is not None else None

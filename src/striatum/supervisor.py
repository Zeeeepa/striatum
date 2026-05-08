"""Long-lived process supervision (RFC 0009).

The supervisor module holds an agent CLI process alive across multiple work
packets so that a session that received packet N retains the same OS process
when it later receives packet N+1. This is distinct from the single-shot
``process_adapter`` flow: that flow runs one ``Popen.communicate(...)`` and
exits with the child. The supervisor instead detaches the child via
``start_new_session=True`` and writes work packets to a per-supervisor named
pipe (FIFO) on each ``supervise send``.

Per RFC 0009 and decisions D028/D037, the supervisor never reads agent stdout
to derive workflow state and never captures transcripts: stdout/stderr are
sent to ``DEVNULL`` by default. State transitions remain CLI-driven and
recorded in SQLite via ``process_supervisors`` rows and ``supervisor.*``
events.
"""

from __future__ import annotations

import fcntl
import os
import signal
import sqlite3
import subprocess
import time
from pathlib import Path
from typing import cast

from striatum.db import (
    JsonObject,
    insert_event,
    json_dumps,
    new_id,
    row_by_id,
    state_dir,
    transaction,
    utc_now,
    workflow_for_run,
)
from striatum.errors import InvalidTransitionError, NotFoundError


SUPERVISOR_ACTIVE_STATES = ("starting", "attached", "detached")


def supervise_start(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
) -> JsonObject:
    """Start a long-lived supervised process for ``session_id``.

    Validates that the session is active and that its lane uses the
    ``process`` adapter. Refuses if the session already has a supervisor in
    one of :data:`SUPERVISOR_ACTIVE_STATES`. Forks the lane command via
    ``subprocess.Popen`` with ``start_new_session=True`` so the child is
    detached, redirects stdout/stderr to ``DEVNULL`` (no transcripts, per
    D028), and reads stdin from a freshly created FIFO at
    ``.striatum/scratch/<supervisor_id>/stdin.pipe``.

    Returns a JSON-friendly mapping with ``supervisor_id``, ``pid``,
    ``stdin_pipe_path``, and ``state``.
    """
    session = row_by_id(conn, "sessions", "session_id", session_id)
    if session["state"] != "active":
        raise InvalidTransitionError("supervise start requires an active session")
    run_id = str(session["run_id"])
    lane_id = str(session["lane_id"])
    workflow = workflow_for_run(conn, run_id=run_id)
    lane = _lane_config(workflow, lane_id=lane_id)
    if lane.get("adapter") != "process":
        raise InvalidTransitionError(
            "supervise start requires a process-adapter lane"
        )
    command = _command_array(lane)

    existing = conn.execute(
        f"""
        SELECT supervisor_id, state FROM process_supervisors
        WHERE session_id = ? AND state IN ({_placeholders(SUPERVISOR_ACTIVE_STATES)})
        """,
        (session_id, *SUPERVISOR_ACTIVE_STATES),
    ).fetchone()
    if existing is not None:
        raise InvalidTransitionError(
            "session already has an active supervisor: "
            f"{existing['supervisor_id']} (state={existing['state']})"
        )

    supervisor_id = new_id("sup")
    scratch = state_dir(repo) / "scratch" / supervisor_id
    scratch.mkdir(parents=True, exist_ok=True)
    pipe_path = scratch / "stdin.pipe"
    if pipe_path.exists():
        pipe_path.unlink()
    os.mkfifo(pipe_path, 0o600)

    started_at = utc_now()
    with transaction(conn):
        conn.execute(
            """
            INSERT INTO process_supervisors (
              supervisor_id, run_id, session_id, adapter, command_json,
              cwd, scratch_path, stdin_pipe_path, state, started_at
            )
            VALUES (?, ?, ?, 'process', ?, ?, ?, ?, 'starting', ?)
            """,
            (
                supervisor_id,
                run_id,
                session_id,
                json_dumps(command),
                str(repo),
                str(scratch),
                str(pipe_path),
                started_at,
            ),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="supervisor.starting",
            actor_session_id=session_id,
            payload={
                "supervisor_id": supervisor_id,
                "adapter": "process",
                "stdin_pipe_path": str(pipe_path),
            },
        )

    # Open the FIFO O_RDWR in the parent before forking the child. This is
    # the standard POSIX trick for survivable FIFO stdin: passing only a
    # read-only fd would let the child see EOF as soon as the parent closes
    # its descriptor (no writers, no readers waiting). Opening O_RDWR keeps
    # the kernel-level "has writer" flag set for the duration of the child's
    # life, so subsequent supervise-send processes can attach as writers
    # without races. The child only ever performs read syscalls on its fd 0;
    # the dup'd open mode is benign. The flag is also non-blocking on open
    # so the parent does not stall on an absent writer; we clear that before
    # handing the fd to the child so child reads behave normally.
    pipe_fd = os.open(str(pipe_path), os.O_RDWR | os.O_NONBLOCK)
    flags = fcntl.fcntl(pipe_fd, fcntl.F_GETFL)
    fcntl.fcntl(pipe_fd, fcntl.F_SETFL, flags & ~os.O_NONBLOCK)
    env = _supervised_env(repo=repo, run_id=run_id, session_id=session_id, supervisor_id=supervisor_id)
    try:
        child = subprocess.Popen(
            command,
            cwd=str(repo),
            env=env,
            stdin=pipe_fd,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
            close_fds=True,
        )
    except OSError as exc:
        os.close(pipe_fd)
        _mark_failed_to_start(conn, supervisor_id=supervisor_id, run_id=run_id, session_id=session_id, error=str(exc))
        if pipe_path.exists():
            pipe_path.unlink()
        raise InvalidTransitionError(
            f"supervisor could not launch lane command: {exc}"
        ) from exc
    finally:
        os.close(pipe_fd)

    pid = child.pid
    if not _pid_alive(pid):
        # Child exited before we could attach — record as lost rather than
        # leaving the row stuck in starting.
        with transaction(conn):
            now = utc_now()
            conn.execute(
                """
                UPDATE process_supervisors
                SET state = 'lost', pid = ?, ended_at = ?, stop_reason = ?
                WHERE supervisor_id = ?
                """,
                (pid, now, "child exited before attach", supervisor_id),
            )
            insert_event(
                conn,
                run_id=run_id,
                event_type="supervisor.lost",
                actor_session_id=session_id,
                payload={"supervisor_id": supervisor_id, "pid": pid, "phase": "start"},
            )
        if pipe_path.exists():
            pipe_path.unlink()
        raise InvalidTransitionError(
            "supervisor child exited before it could be attached"
        )

    with transaction(conn):
        now = utc_now()
        conn.execute(
            """
            UPDATE process_supervisors
            SET state = 'attached', pid = ?, heartbeat_at = ?
            WHERE supervisor_id = ?
            """,
            (pid, now, supervisor_id),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="supervisor.started",
            actor_session_id=session_id,
            payload={
                "supervisor_id": supervisor_id,
                "pid": pid,
                "stdin_pipe_path": str(pipe_path),
            },
        )

    return {
        "supervisor_id": supervisor_id,
        "pid": pid,
        "stdin_pipe_path": str(pipe_path),
        "state": "attached",
    }


def supervise_send(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    packet_id: str,
) -> JsonObject:
    """Deliver a stored work packet to the session's attached supervisor.

    The packet's ``packet_json`` is written to the supervisor's stdin pipe
    with a trailing newline (NDJSON-style framing). ``heartbeat_at`` is
    refreshed and a ``supervisor.packet_delivered`` event is recorded with
    the byte count actually written.
    """
    supervisor = _require_active_supervisor(conn, session_id=session_id)
    if supervisor["state"] != "attached":
        raise InvalidTransitionError(
            "supervise send requires an attached supervisor "
            f"(supervisor_id={supervisor['supervisor_id']}, state={supervisor['state']})"
        )

    packet = conn.execute(
        "SELECT packet_id, run_id, session_id, lease_id, packet_json FROM work_packets WHERE packet_id = ?",
        (packet_id,),
    ).fetchone()
    if packet is None:
        raise NotFoundError(f"could not find work packet for packet_id={packet_id!r}")
    if str(packet["session_id"]) != session_id:
        raise InvalidTransitionError(
            "work packet does not belong to this session: "
            f"packet_session={packet['session_id']!r}, requested_session={session_id!r}"
        )

    pipe_path = Path(str(supervisor["stdin_pipe_path"]))
    if not pipe_path.exists():
        raise InvalidTransitionError(
            f"supervisor stdin pipe is missing: {pipe_path}"
        )

    payload = str(packet["packet_json"])
    if not payload.endswith("\n"):
        payload = payload + "\n"
    payload_bytes = payload.encode("utf-8")

    pid_value = supervisor["pid"]
    if pid_value is None or not _pid_alive(int(pid_value)):
        _mark_lost(conn, supervisor=supervisor, reason="pid is gone before send")
        raise InvalidTransitionError(
            f"supervisor pid is gone: {supervisor['supervisor_id']}"
        )

    bytes_written = _write_to_pipe(pipe_path, payload_bytes)

    delivered_at = utc_now()
    run_id = str(supervisor["run_id"])
    with transaction(conn):
        conn.execute(
            "UPDATE process_supervisors SET heartbeat_at = ? WHERE supervisor_id = ?",
            (delivered_at, supervisor["supervisor_id"]),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="supervisor.packet_delivered",
            actor_session_id=session_id,
            payload={
                "supervisor_id": str(supervisor["supervisor_id"]),
                "packet_id": packet_id,
                "bytes_written": bytes_written,
            },
        )
    return {
        "supervisor_id": str(supervisor["supervisor_id"]),
        "packet_id": packet_id,
        "delivered_at": delivered_at,
        "bytes": bytes_written,
    }


def supervise_stop(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    reason: str,
) -> JsonObject:
    """Stop the active supervisor for ``session_id``.

    Sends ``SIGTERM`` to the recorded pid and waits up to five seconds; if
    the process is still present, falls back to ``SIGKILL``. Marks the row
    ``stopped``, records ``ended_at`` and ``stop_reason``, removes the named
    pipe, and emits ``supervisor.stopped``.

    Idempotent against a supervisor whose row is already ``lost`` or
    ``stopped`` (HARNESS-001): the latest supervisor row is returned with a
    ``note`` describing the prior terminal state instead of raising
    ``InvalidTransitionError``. This avoids forcing operators to remember
    whether a supervisor died on its own before they invoke ``stop``.
    """
    terminal = _latest_terminal_supervisor(conn, session_id=session_id)
    if terminal is not None:
        prior_state = str(terminal["state"])
        return {
            "supervisor_id": str(terminal["supervisor_id"]),
            "session_id": session_id,
            "pid": terminal["pid"],
            "state": "stopped",
            "ended_at": terminal["ended_at"],
            "stop_reason": terminal["stop_reason"],
            "signal": None,
            "note": f"supervisor was already {prior_state}",
        }
    supervisor = _require_active_supervisor(conn, session_id=session_id)
    pid_value = supervisor["pid"]
    pid: int | None = int(pid_value) if pid_value is not None else None
    signaled = None
    if pid is not None and _pid_alive(pid):
        try:
            os.kill(pid, signal.SIGTERM)
            signaled = "SIGTERM"
        except ProcessLookupError:
            signaled = None
        else:
            deadline = time.time() + 5.0
            while time.time() < deadline:
                if not _pid_alive(pid):
                    break
                time.sleep(0.05)
            if _pid_alive(pid):
                try:
                    os.kill(pid, signal.SIGKILL)
                    signaled = "SIGKILL"
                except ProcessLookupError:
                    pass
                # Give the kernel a brief moment to reap.
                deadline = time.time() + 1.0
                while time.time() < deadline and _pid_alive(pid):
                    time.sleep(0.02)

    pipe_path_text = supervisor["stdin_pipe_path"]
    if pipe_path_text:
        pipe_path = Path(str(pipe_path_text))
        if pipe_path.exists():
            try:
                pipe_path.unlink()
            except OSError:
                # Leave the path on disk; doctor will surface a missing/extra
                # pipe rather than failing stop. Not an error condition.
                pass

    ended_at = utc_now()
    run_id = str(supervisor["run_id"])
    with transaction(conn):
        conn.execute(
            """
            UPDATE process_supervisors
            SET state = 'stopped', ended_at = ?, stop_reason = ?
            WHERE supervisor_id = ?
            """,
            (ended_at, reason, supervisor["supervisor_id"]),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="supervisor.stopped",
            actor_session_id=session_id,
            payload={
                "supervisor_id": str(supervisor["supervisor_id"]),
                "pid": pid,
                "reason": reason,
                "signal": signaled,
            },
        )
    return {
        "supervisor_id": str(supervisor["supervisor_id"]),
        "session_id": session_id,
        "pid": pid,
        "state": "stopped",
        "ended_at": ended_at,
        "stop_reason": reason,
        "signal": signaled,
    }


def supervise_status(
    conn: sqlite3.Connection,
    *,
    session_id: str,
) -> JsonObject:
    """Return the latest supervisor row for ``session_id`` plus liveness.

    Probes liveness via ``os.kill(pid, 0)``. If the pid is gone for an
    otherwise active row, the row is transitioned to ``lost`` and a
    ``supervisor.lost`` event is recorded before returning. Already-terminal
    states (``stopped``, ``lost``) are returned verbatim with
    ``liveness=gone``.
    """
    row = conn.execute(
        """
        SELECT * FROM process_supervisors
        WHERE session_id = ?
        ORDER BY started_at DESC
        LIMIT 1
        """,
        (session_id,),
    ).fetchone()
    if row is None:
        raise NotFoundError(
            f"no supervisor recorded for session_id={session_id!r}"
        )
    supervisor = dict(row)
    pid_value = supervisor.get("pid")
    state = str(supervisor.get("state"))
    liveness = "gone"
    if pid_value is not None and state in SUPERVISOR_ACTIVE_STATES:
        if _pid_alive(int(pid_value)):
            liveness = "alive"
        else:
            _mark_lost(
                conn,
                supervisor=row,
                reason="pid is gone observed by status",
            )
            supervisor["state"] = "lost"
            supervisor["ended_at"] = utc_now()
            supervisor["stop_reason"] = "pid is gone observed by status"
            liveness = "gone"
    elif pid_value is not None and state == "stopped":
        liveness = "alive" if _pid_alive(int(pid_value)) else "gone"
    supervisor["liveness"] = liveness
    return supervisor


def supervise_list(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    state: str | None = None,
) -> JsonObject:
    """List supervisors for ``run_id``, optionally filtered by state."""
    if state is None:
        rows = conn.execute(
            """
            SELECT * FROM process_supervisors
            WHERE run_id = ?
            ORDER BY started_at DESC
            """,
            (run_id,),
        ).fetchall()
    else:
        rows = conn.execute(
            """
            SELECT * FROM process_supervisors
            WHERE run_id = ? AND state = ?
            ORDER BY started_at DESC
            """,
            (run_id, state),
        ).fetchall()
    return {
        "run_id": run_id,
        "state": state,
        "supervisors": [dict(row) for row in rows],
    }


def deliver_packet_to_attached_supervisor(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    session_id: str,
    packet_id: str,
    packet_json: str,
) -> JsonObject | None:
    """Auto-route a freshly claimed packet to this session's supervisor.

    Called from :func:`striatum.db.claim_next` while still inside the claim
    transaction so packet creation and supervisor bookkeeping land atomically.

    The helper looks for an ``attached`` supervisor row for ``session_id``. If
    none exists, it returns ``None`` (the caller leaves the response shape
    unchanged for backwards compatibility). Otherwise it attempts to write the
    packet payload (plus a trailing newline) to the supervisor's stdin pipe
    and:

    - On success, refreshes ``heartbeat_at``, records a
      ``supervisor.packet_delivered`` event, and returns
      ``{"supervisor_id": ..., "bytes_written": int}``.
    - If the pipe is missing on disk or the write fails with
      ``BrokenPipeError``/``OSError``, transitions the supervisor to ``lost``,
      emits ``supervisor.lost``, and returns
      ``{"supervisor_id": ..., "error": "stdin_pipe_missing"}``. The packet
      itself remains valid; the caller can retry once the supervisor is
      restored.

    All SQL runs on the caller-supplied connection without opening a nested
    transaction. The caller must already be inside ``transaction(conn)``.
    """
    row = conn.execute(
        """
        SELECT * FROM process_supervisors
        WHERE session_id = ? AND state = 'attached'
        ORDER BY started_at DESC
        LIMIT 1
        """,
        (session_id,),
    ).fetchone()
    if row is None:
        return None

    supervisor_id = str(row["supervisor_id"])
    pipe_path_text = row["stdin_pipe_path"]
    pipe_path = Path(str(pipe_path_text)) if pipe_path_text else None

    payload = packet_json if packet_json.endswith("\n") else packet_json + "\n"
    payload_bytes = payload.encode("utf-8")

    error: str | None = None
    if pipe_path is None or not pipe_path.exists():
        error = "stdin_pipe_missing"
    else:
        try:
            bytes_written = _write_to_pipe(pipe_path, payload_bytes)
        except (InvalidTransitionError, BrokenPipeError, OSError):
            error = "stdin_pipe_missing"

    if error is not None:
        now = utc_now()
        conn.execute(
            """
            UPDATE process_supervisors
            SET state = 'lost', ended_at = ?, stop_reason = ?
            WHERE supervisor_id = ?
            """,
            (now, "stdin pipe missing during auto-delivery", supervisor_id),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="supervisor.lost",
            actor_session_id=session_id,
            payload={
                "supervisor_id": supervisor_id,
                "pid": row["pid"],
                "reason": "stdin pipe missing during auto-delivery",
                "phase": "claim_next_auto_delivery",
            },
        )
        return {"supervisor_id": supervisor_id, "error": error}

    delivered_at = utc_now()
    conn.execute(
        "UPDATE process_supervisors SET heartbeat_at = ? WHERE supervisor_id = ?",
        (delivered_at, supervisor_id),
    )
    insert_event(
        conn,
        run_id=run_id,
        event_type="supervisor.packet_delivered",
        actor_session_id=session_id,
        payload={
            "supervisor_id": supervisor_id,
            "packet_id": packet_id,
            "bytes_written": bytes_written,
            "via": "claim_next_auto_delivery",
        },
    )
    return {"supervisor_id": supervisor_id, "bytes_written": bytes_written}


def mark_supervisor_lost_for_lease(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    session_id: str,
    lease_id: str,
) -> None:
    """Mark a supervisor ``lost`` when its session's lease just expired.

    Called from :func:`striatum.db.expire_leases`. RFC 0009 explicitly does
    not auto-kill supervised processes on lease expiry: operator inspection
    is required for repo-write work, mirroring the existing stale-lease
    policy. A ``supervisor.lease_expired_with_supervisor`` event is recorded
    so ``why`` and ``doctor`` surfaces can explain the transition.
    """
    row = conn.execute(
        """
        SELECT supervisor_id, state, pid FROM process_supervisors
        WHERE session_id = ? AND state = 'attached'
        ORDER BY started_at DESC
        LIMIT 1
        """,
        (session_id,),
    ).fetchone()
    if row is None:
        return
    now = utc_now()
    conn.execute(
        """
        UPDATE process_supervisors
        SET state = 'lost', ended_at = ?, stop_reason = ?
        WHERE supervisor_id = ?
        """,
        (now, "lease expired while attached", row["supervisor_id"]),
    )
    insert_event(
        conn,
        run_id=run_id,
        event_type="supervisor.lease_expired_with_supervisor",
        actor_session_id=session_id,
        lease_id=lease_id,
        payload={
            "supervisor_id": str(row["supervisor_id"]),
            "pid": row["pid"],
        },
    )


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------


def _placeholders(values: tuple[str, ...]) -> str:
    return ",".join(["?"] * len(values))


def _lane_config(workflow: JsonObject, *, lane_id: str) -> JsonObject:
    lanes = workflow.get("lanes", {})
    if not isinstance(lanes, dict):
        raise InvalidTransitionError("workflow lanes must be an object")
    lane = lanes.get(lane_id)
    if not isinstance(lane, dict):
        raise InvalidTransitionError(f"workflow lane {lane_id!r} is not configured")
    return cast(JsonObject, lane)


def _command_array(lane: JsonObject) -> list[str]:
    command = lane.get("command")
    if not isinstance(command, list) or not command:
        raise InvalidTransitionError("process lane command must be a non-empty array")
    if not all(isinstance(part, str) and part != "" for part in command):
        raise InvalidTransitionError("process lane command entries must be non-empty strings")
    return cast(list[str], command)


def _supervised_env(
    *,
    repo: Path,
    run_id: str,
    session_id: str,
    supervisor_id: str,
) -> dict[str, str]:
    base_env = dict(os.environ)
    base_env.update(
        {
            "STRIATUM_RUN_ID": run_id,
            "STRIATUM_SESSION_ID": session_id,
            "STRIATUM_SUPERVISOR_ID": supervisor_id,
            "STRIATUM_REPO": str(repo),
        }
    )
    return base_env


def _pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        # The pid exists but is owned by another user; from the supervisor's
        # perspective the original child is gone.
        return False
    return True


def _require_active_supervisor(
    conn: sqlite3.Connection, *, session_id: str
) -> sqlite3.Row:
    row = conn.execute(
        f"""
        SELECT * FROM process_supervisors
        WHERE session_id = ?
          AND state IN ({_placeholders(SUPERVISOR_ACTIVE_STATES)})
        ORDER BY started_at DESC
        LIMIT 1
        """,
        (session_id, *SUPERVISOR_ACTIVE_STATES),
    ).fetchone()
    if row is None:
        raise InvalidTransitionError(
            f"no active supervisor for session_id={session_id!r}"
        )
    return cast(sqlite3.Row, row)


_TERMINAL_SUPERVISOR_STATES = ("lost", "stopped")


def _latest_terminal_supervisor(
    conn: sqlite3.Connection, *, session_id: str
) -> sqlite3.Row | None:
    """Return the most recent terminal supervisor row for ``session_id``.

    Used by :func:`supervise_stop` to make stop idempotent against a
    supervisor that has already exited (state ``lost``) or already been
    stopped. Returns ``None`` when the session has an active supervisor or
    no supervisor at all — in those cases the caller falls through to the
    normal ``_require_active_supervisor`` path.
    """
    active = conn.execute(
        f"""
        SELECT 1 FROM process_supervisors
        WHERE session_id = ?
          AND state IN ({_placeholders(SUPERVISOR_ACTIVE_STATES)})
        LIMIT 1
        """,
        (session_id, *SUPERVISOR_ACTIVE_STATES),
    ).fetchone()
    if active is not None:
        return None
    row = conn.execute(
        f"""
        SELECT * FROM process_supervisors
        WHERE session_id = ?
          AND state IN ({_placeholders(_TERMINAL_SUPERVISOR_STATES)})
        ORDER BY started_at DESC
        LIMIT 1
        """,
        (session_id, *_TERMINAL_SUPERVISOR_STATES),
    ).fetchone()
    return cast(sqlite3.Row, row) if row is not None else None


def _mark_failed_to_start(
    conn: sqlite3.Connection,
    *,
    supervisor_id: str,
    run_id: str,
    session_id: str,
    error: str,
) -> None:
    with transaction(conn):
        now = utc_now()
        conn.execute(
            """
            UPDATE process_supervisors
            SET state = 'lost', ended_at = ?, stop_reason = ?
            WHERE supervisor_id = ?
            """,
            (now, f"start failed: {error}", supervisor_id),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="supervisor.lost",
            actor_session_id=session_id,
            payload={
                "supervisor_id": supervisor_id,
                "phase": "start",
                "error": error,
            },
        )


def _mark_lost(
    conn: sqlite3.Connection,
    *,
    supervisor: sqlite3.Row,
    reason: str,
) -> None:
    """Transition ``supervisor`` to ``lost`` and emit ``supervisor.lost``."""
    with transaction(conn):
        now = utc_now()
        conn.execute(
            """
            UPDATE process_supervisors
            SET state = 'lost', ended_at = ?, stop_reason = ?
            WHERE supervisor_id = ?
            """,
            (now, reason, supervisor["supervisor_id"]),
        )
        insert_event(
            conn,
            run_id=str(supervisor["run_id"]),
            event_type="supervisor.lost",
            actor_session_id=str(supervisor["session_id"]),
            payload={
                "supervisor_id": str(supervisor["supervisor_id"]),
                "pid": supervisor["pid"],
                "reason": reason,
            },
        )


def _write_to_pipe(pipe_path: Path, payload: bytes) -> int:
    """Write ``payload`` to ``pipe_path``, blocking until the reader drains it.

    The FIFO is opened blocking; if the reader is alive but slow, the write
    will queue until pipe capacity is hit and then block. If the reader has
    closed its end, the OS raises ``BrokenPipeError`` on write and the
    caller surfaces that as a ``InvalidTransitionError``.
    """
    fd = os.open(str(pipe_path), os.O_WRONLY)
    try:
        view = memoryview(payload)
        total = 0
        while total < len(view):
            try:
                written = os.write(fd, view[total:])
            except BrokenPipeError as exc:
                raise InvalidTransitionError(
                    "supervisor pipe is broken; child has closed stdin"
                ) from exc
            if written <= 0:
                raise InvalidTransitionError(
                    "supervisor pipe write returned zero bytes"
                )
            total += written
        return total
    finally:
        os.close(fd)


# Re-exported for downstream import discovery (e.g., doctor checks).
__all__ = [
    "SUPERVISOR_ACTIVE_STATES",
    "supervise_start",
    "supervise_send",
    "supervise_stop",
    "supervise_status",
    "supervise_list",
    "mark_supervisor_lost_for_lease",
    "deliver_packet_to_attached_supervisor",
]

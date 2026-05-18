"""Generic local process adapter for configured workflow lanes."""

from __future__ import annotations

import os
import sqlite3
import string
import subprocess
from collections.abc import Mapping
from pathlib import Path
from typing import Any, cast

from striatum.legacy_sqlite.db import (
    active_lease_for,
    insert_event,
    row_by_id,
    transaction,
)
from striatum.errors import InvalidTransitionError
from striatum.primitives import JsonObject, json_dumps, json_loads, new_id, utc_now
from striatum.repo_policy import state_dir


def _expand_lane_env_values(
    raw_env: Mapping[str, str], expansion: Mapping[str, str]
) -> dict[str, str]:
    """Expand ``${VAR}`` / ``$VAR`` references in lane env values.

    Subprocess does not perform shell substitution on env values, so a lane
    config like ``CODEX_HOME=${STRIATUM_SCRATCH_DIR}/codex-home`` would otherwise
    reach the child literally. Unknown variables are left untouched, matching
    ``os.path.expandvars`` semantics.
    """
    return {
        key: string.Template(value).safe_substitute(expansion)
        for key, value in raw_env.items()
    }


PROCESS_SCHEMA_SQL = """
CREATE TABLE IF NOT EXISTS process_executions (
  process_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT NOT NULL REFERENCES jobs(job_id),
  session_id TEXT NOT NULL REFERENCES sessions(session_id),
  lease_id TEXT NOT NULL REFERENCES leases(lease_id),
  packet_id TEXT NOT NULL REFERENCES work_packets(packet_id),
  adapter TEXT NOT NULL,
  command_json TEXT NOT NULL,
  cwd TEXT NOT NULL,
  scratch_path TEXT NOT NULL,
  stdin_mode TEXT NOT NULL CHECK (stdin_mode IN ('packet','none')),
  stdio_mode TEXT NOT NULL CHECK (stdio_mode IN ('suppressed','inherit')),
  pid INTEGER,
  state TEXT NOT NULL CHECK (state IN (
    'starting','running','exited','failed','timed_out','lost'
  )),
  exit_code INTEGER,
  started_at TEXT NOT NULL,
  ended_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_process_executions_run_job
  ON process_executions(run_id, job_id, started_at);
"""


def run_process_adapter(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    lease_id: str,
    stdin_mode: str,
    inherit_stdio: bool,
    timeout_seconds: int | None = None,
) -> JsonObject:
    """Run the process command configured for a claimed session's lane.

    RFC 0014 V1: when ``timeout_seconds`` is set, ``communicate`` is wrapped
    in a try/except for ``subprocess.TimeoutExpired``; on timeout the child
    is SIGTERM'd, then SIGKILL'd after a 5-second wait, and the job is
    transitioned to ``blocked`` with the ``process_timeout_exceeded`` reason.

    After every normal exit, ``evaluate_and_block_inline`` runs against the
    job and inserts a blocker row when required artifacts or review-job
    verdicts are missing.
    """
    if stdin_mode not in {"packet", "none"}:
        raise InvalidTransitionError("stdin mode must be packet or none")
    if timeout_seconds is not None and timeout_seconds <= 0:
        raise InvalidTransitionError("timeout_seconds must be a positive integer")
    ensure_process_schema(conn)
    launch = prepare_process_launch(
        conn,
        repo=repo,
        session_id=session_id,
        lease_id=lease_id,
        stdin_mode=stdin_mode,
        stdio_mode="inherit" if inherit_stdio else "suppressed",
    )
    process_id = str(launch["process_id"])
    command = cast(list[str], launch["command"])
    scratch_path = Path(str(launch["scratch_path"]))
    scratch_path.mkdir(parents=True, exist_ok=True)
    constraints = cast(dict[str, str], launch.get("lane_constraints") or {})
    if timeout_seconds is None:
        lane_timeout = launch.get("lane_timeout_seconds")
        if isinstance(lane_timeout, int) and lane_timeout > 0:
            timeout_seconds = int(lane_timeout)
    base_env = dict(os.environ)
    if constraints.get("network") == "forbidden":
        for key in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
                    "http_proxy", "https_proxy", "all_proxy", "no_proxy"):
            base_env.pop(key, None)
    striatum_env: dict[str, str] = {
        "STRIATUM_RUN_ID": str(launch["run_id"]),
        "STRIATUM_JOB_ID": str(launch["job_id"]),
        "STRIATUM_WORKFLOW_JOB_ID": str(launch["workflow_job_id"]),
        "STRIATUM_SESSION_ID": session_id,
        "STRIATUM_LEASE_ID": lease_id,
        "STRIATUM_PACKET_ID": str(launch["packet_id"]),
        "STRIATUM_PROCESS_ID": process_id,
        "STRIATUM_REPO": str(repo),
        "STRIATUM_SCRATCH_DIR": str(scratch_path),
    }
    if constraints.get("network") == "forbidden":
        striatum_env["STRIATUM_NETWORK_POLICY"] = "forbidden"
    if constraints.get("repo_scope") == "local_only":
        striatum_env["STRIATUM_REPO_SCOPE"] = "local_only"
    lane_env_raw = cast(dict[str, str], launch["env"]) or {}
    lane_env = _expand_lane_env_values(lane_env_raw, {**base_env, **striatum_env})
    env = {**base_env, **lane_env, **striatum_env}
    payload = str(launch["packet_json"]) if stdin_mode == "packet" else None
    stdio = None if inherit_stdio else subprocess.DEVNULL
    try:
        process = subprocess.Popen(
            command,
            cwd=repo,
            env=env,
            stdin=subprocess.PIPE if payload is not None else subprocess.DEVNULL,
            stdout=stdio,
            stderr=stdio,
            text=True,
        )
    except OSError as exc:
        mark_process_failed(conn, process_id=process_id, error=str(exc))
        raise InvalidTransitionError(f"process adapter could not launch command: {exc}") from exc
    mark_process_running(conn, process_id=process_id, pid=process.pid)
    timed_out = False
    try:
        stdout_data, stderr_data = process.communicate(payload, timeout=timeout_seconds)
        del stdout_data, stderr_data
    except subprocess.TimeoutExpired:
        timed_out = True
        process.terminate()
        try:
            stdout_data, stderr_data = process.communicate(timeout=5)
            del stdout_data, stderr_data
        except subprocess.TimeoutExpired:
            process.kill()
            stdout_data, stderr_data = process.communicate()
            del stdout_data, stderr_data
    if timed_out:
        result = mark_process_timed_out(
            conn,
            process_id=process_id,
            timeout_seconds=int(timeout_seconds) if timeout_seconds else 0,
        )
        exit_code: int | None = None
    else:
        result = mark_process_exited(conn, process_id=process_id, exit_code=process.returncode)
        exit_code = int(process.returncode)
    _evaluate_and_block_after_run(
        conn,
        process_id=process_id,
        session_id=session_id,
        lease_id=lease_id,
        command=command,
        exit_code=exit_code,
        timed_out=timed_out,
        timeout_seconds=int(timeout_seconds) if timeout_seconds else None,
    )
    return result


def _evaluate_and_block_after_run(
    conn: sqlite3.Connection,
    *,
    process_id: str,
    session_id: str,
    lease_id: str,
    command: list[str],
    exit_code: int | None,
    timed_out: bool,
    timeout_seconds: int | None,
) -> None:
    """Run RFC 0014 V1 post-exit validation and block the job if needed."""
    from striatum.process_completion import evaluate_and_block_inline

    with transaction(conn):
        process = row_by_id(conn, "process_executions", "process_id", process_id)
        job = row_by_id(conn, "jobs", "job_id", str(process["job_id"]))
        started = process["started_at"]
        ended = process["ended_at"] or utc_now()
        duration = _duration_seconds(str(started), str(ended))
        evaluate_and_block_inline(
            conn,
            job=job,
            session_id=session_id,
            process_id=process_id,
            command=command,
            exit_code=exit_code,
            duration_seconds=duration,
            timed_out=timed_out,
            timeout_seconds=timeout_seconds,
        )


def _duration_seconds(started_at: str, ended_at: str) -> float:
    """Return the seconds between two UTC ISO timestamps."""
    from datetime import datetime

    def parse(value: str) -> datetime:
        # Accept both '2026-05-08T12:34:56.789Z' and '...+00:00' shapes.
        cleaned = value.replace("Z", "+00:00")
        return datetime.fromisoformat(cleaned)

    return max(0.0, (parse(ended_at) - parse(started_at)).total_seconds())


def ensure_process_schema(conn: sqlite3.Connection) -> None:
    """Create process metadata storage for state databases initialized before this table existed."""
    conn.executescript(PROCESS_SCHEMA_SQL)


def prepare_process_launch(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    lease_id: str,
    stdin_mode: str,
    stdio_mode: str,
) -> JsonObject:
    """Validate the active lease and insert starting process metadata."""
    with transaction(conn):
        session = row_by_id(conn, "sessions", "session_id", session_id)
        lease = active_lease_for(conn, lease_id=lease_id, session_id=session_id)
        job = row_by_id(conn, "jobs", "job_id", str(lease["resource_id"]))
        if job["state"] not in {"claimed", "running"}:
            raise InvalidTransitionError("process adapter requires claimed or running work")
        packet = conn.execute(
            """
            SELECT * FROM work_packets
            WHERE lease_id = ? AND session_id = ?
            ORDER BY created_at DESC LIMIT 1
            """,
            (lease_id, session_id),
        ).fetchone()
        if packet is None:
            raise InvalidTransitionError("process adapter requires a claimed work packet")
        run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
        snapshot = row_by_id(
            conn,
            "workflow_snapshots",
            "workflow_snapshot_id",
            str(run["workflow_snapshot_id"]),
        )
        workflow = json_loads(str(snapshot["workflow_json"]))
        lane_id = str(session["lane_id"])
        lane = lane_config(workflow, lane_id=lane_id)
        if lane.get("adapter") != "process":
            raise InvalidTransitionError("session lane is not configured for the process adapter")
        command = command_array(lane)
        env_overrides = lane.get("env", {})
        if env_overrides is not None and not string_mapping(env_overrides):
            raise InvalidTransitionError("process lane env must be an object of strings")
        process_id = new_id("proc")
        scratch = state_dir(repo) / "scratch" / process_id
        now = utc_now()
        conn.execute(
            """
            INSERT INTO process_executions (
              process_id, run_id, job_id, session_id, lease_id, packet_id,
              adapter, command_json, cwd, scratch_path, stdin_mode, stdio_mode,
              state, started_at
            )
            VALUES (?, ?, ?, ?, ?, ?, 'process', ?, ?, ?, ?, ?, 'starting', ?)
            """,
            (
                process_id,
                run["run_id"],
                job["job_id"],
                session_id,
                lease_id,
                packet["packet_id"],
                json_dumps(command),
                str(repo),
                str(scratch),
                stdin_mode,
                stdio_mode,
                now,
            ),
        )
        insert_event(
            conn,
            run_id=str(run["run_id"]),
            event_type="process.starting",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            lease_id=lease_id,
            payload={"process_id": process_id, "adapter": "process", "stdio": stdio_mode},
        )
        constraints_value = lane.get("constraints")
        lane_constraints = constraints_value if isinstance(constraints_value, dict) else {}
        lane_timeout = lane.get("adapter_timeout_seconds")
        if not isinstance(lane_timeout, int) or isinstance(lane_timeout, bool) or lane_timeout <= 0:
            lane_timeout = None
        return {
            "process_id": process_id,
            "run_id": run["run_id"],
            "job_id": job["job_id"],
            "workflow_job_id": job["workflow_job_id"],
            "session_id": session_id,
            "lease_id": lease_id,
            "packet_id": packet["packet_id"],
            "packet_json": packet["packet_json"],
            "command": command,
            "env": env_overrides if isinstance(env_overrides, dict) else {},
            "scratch_path": str(scratch),
            "lane_constraints": lane_constraints,
            "lane_timeout_seconds": lane_timeout,
        }


def mark_process_running(conn: sqlite3.Connection, *, process_id: str, pid: int) -> None:
    """Record the operating-system pid after launch."""
    with transaction(conn):
        process = row_by_id(conn, "process_executions", "process_id", process_id)
        conn.execute(
            "UPDATE process_executions SET pid = ?, state = 'running' WHERE process_id = ?",
            (pid, process_id),
        )
        insert_event(
            conn,
            run_id=str(process["run_id"]),
            event_type="process.started",
            actor_session_id=str(process["session_id"]),
            job_id=str(process["job_id"]),
            lease_id=str(process["lease_id"]),
            payload={"process_id": process_id, "pid": pid},
        )


def mark_process_failed(conn: sqlite3.Connection, *, process_id: str, error: str) -> None:
    """Record a launch failure without storing process output."""
    with transaction(conn):
        process = row_by_id(conn, "process_executions", "process_id", process_id)
        ended_at = utc_now()
        conn.execute(
            "UPDATE process_executions SET state = 'failed', ended_at = ? WHERE process_id = ?",
            (ended_at, process_id),
        )
        insert_event(
            conn,
            run_id=str(process["run_id"]),
            event_type="process.failed",
            actor_session_id=str(process["session_id"]),
            job_id=str(process["job_id"]),
            lease_id=str(process["lease_id"]),
            payload={"process_id": process_id, "error": error},
        )


def mark_process_exited(conn: sqlite3.Connection, *, process_id: str, exit_code: int) -> JsonObject:
    """Record process exit metadata."""
    with transaction(conn):
        process = row_by_id(conn, "process_executions", "process_id", process_id)
        ended_at = utc_now()
        conn.execute(
            """
            UPDATE process_executions
            SET state = 'exited', exit_code = ?, ended_at = ?
            WHERE process_id = ?
            """,
            (exit_code, ended_at, process_id),
        )
        insert_event(
            conn,
            run_id=str(process["run_id"]),
            event_type="process.exited",
            actor_session_id=str(process["session_id"]),
            job_id=str(process["job_id"]),
            lease_id=str(process["lease_id"]),
            payload={"process_id": process_id, "exit_code": exit_code},
        )
        return {
            "process_id": process_id,
            "run_id": process["run_id"],
            "job_id": process["job_id"],
            "session_id": process["session_id"],
            "lease_id": process["lease_id"],
            "state": "exited",
            "exit_code": exit_code,
            "scratch_path": process["scratch_path"],
        }


def mark_process_timed_out(
    conn: sqlite3.Connection,
    *,
    process_id: str,
    timeout_seconds: int,
) -> JsonObject:
    """Record that a process was SIGTERM'd after exceeding its timeout.

    RFC 0014 V1 step 2. The blocker row + envelope are written by
    :func:`process_completion.evaluate_and_block_inline`; this helper
    only updates the ``process_executions`` row and records the
    ``process.timed_out`` event.
    """
    with transaction(conn):
        process = row_by_id(conn, "process_executions", "process_id", process_id)
        ended_at = utc_now()
        conn.execute(
            """
            UPDATE process_executions
            SET state = 'timed_out', exit_code = NULL, ended_at = ?
            WHERE process_id = ?
            """,
            (ended_at, process_id),
        )
        insert_event(
            conn,
            run_id=str(process["run_id"]),
            event_type="process.timed_out",
            actor_session_id=str(process["session_id"]),
            job_id=str(process["job_id"]),
            lease_id=str(process["lease_id"]),
            payload={"process_id": process_id, "timeout_seconds": timeout_seconds},
        )
        return {
            "process_id": process_id,
            "run_id": process["run_id"],
            "job_id": process["job_id"],
            "session_id": process["session_id"],
            "lease_id": process["lease_id"],
            "state": "timed_out",
            "exit_code": None,
            "scratch_path": process["scratch_path"],
            "timeout_seconds": timeout_seconds,
        }


def mark_process_lost(
    conn: sqlite3.Connection,
    *,
    process_id: str,
) -> JsonObject:
    """Record that a process was reconciled to ``lost`` (external kill / runner exit).

    RFC 0014 V1 step 3. Used by ``recovery process-reconcile``.
    """
    with transaction(conn):
        process = row_by_id(conn, "process_executions", "process_id", process_id)
        ended_at = utc_now()
        conn.execute(
            """
            UPDATE process_executions
            SET state = 'lost', ended_at = ?
            WHERE process_id = ?
            """,
            (ended_at, process_id),
        )
        insert_event(
            conn,
            run_id=str(process["run_id"]),
            event_type="process.lost",
            actor_session_id=str(process["session_id"]),
            job_id=str(process["job_id"]),
            lease_id=str(process["lease_id"]),
            payload={"process_id": process_id, "pid": process["pid"]},
        )
        return {
            "process_id": process_id,
            "run_id": process["run_id"],
            "job_id": process["job_id"],
            "session_id": process["session_id"],
            "lease_id": process["lease_id"],
            "state": "lost",
            "pid": process["pid"],
            "scratch_path": process["scratch_path"],
        }


def lane_config(workflow: JsonObject, *, lane_id: str) -> JsonObject:
    """Return a lane config object from a workflow snapshot."""
    lanes = workflow.get("lanes", {})
    if not isinstance(lanes, dict):
        raise InvalidTransitionError("workflow lanes must be an object")
    lane = lanes.get(lane_id)
    if not isinstance(lane, dict):
        raise InvalidTransitionError(f"workflow lane {lane_id!r} is not configured")
    return cast(JsonObject, lane)


def command_array(lane: JsonObject) -> list[str]:
    """Return a validated command array."""
    command = lane.get("command")
    if not isinstance(command, list) or not command:
        raise InvalidTransitionError("process lane command must be a non-empty array")
    if not all(isinstance(part, str) and part != "" for part in command):
        raise InvalidTransitionError("process lane command entries must be non-empty strings")
    return cast(list[str], command)


def string_mapping(value: Any) -> bool:
    """Return whether value is an object with string keys and values."""
    return isinstance(value, dict) and all(
        isinstance(key, str) and isinstance(item, str) for key, item in value.items()
    )

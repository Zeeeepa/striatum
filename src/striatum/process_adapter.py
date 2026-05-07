"""Generic local process adapter for configured workflow lanes."""

from __future__ import annotations

import os
import sqlite3
import subprocess
from pathlib import Path
from typing import Any, cast

from striatum.db import (
    JsonObject,
    active_lease_for,
    insert_event,
    json_dumps,
    json_loads,
    new_id,
    row_by_id,
    state_dir,
    transaction,
    utc_now,
)
from striatum.errors import InvalidTransitionError


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
  state TEXT NOT NULL CHECK (state IN ('starting','running','exited','failed')),
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
) -> JsonObject:
    """Run the process command configured for a claimed session's lane."""
    if stdin_mode not in {"packet", "none"}:
        raise InvalidTransitionError("stdin mode must be packet or none")
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
    base_env = dict(os.environ)
    if constraints.get("network") == "forbidden":
        for key in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
                    "http_proxy", "https_proxy", "all_proxy", "no_proxy"):
            base_env.pop(key, None)
    env = {
        **base_env,
        **cast(dict[str, str], launch["env"]),
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
        env["STRIATUM_NETWORK_POLICY"] = "forbidden"
    if constraints.get("repo_scope") == "local_only":
        env["STRIATUM_REPO_SCOPE"] = "local_only"
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
    stdout_data, stderr_data = process.communicate(payload)
    del stdout_data, stderr_data
    return mark_process_exited(conn, process_id=process_id, exit_code=process.returncode)


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

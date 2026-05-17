"""PG-backed long-lived supervision handlers."""

from __future__ import annotations

import fcntl
import json
import os
import signal
import shutil
import subprocess
import time
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any, cast

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _jsonb,
    _ts,
    transaction,
    workflow_for_run,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import InvalidTransitionError, NotFoundError
from striatum.identity import process_start_time
from striatum.primitives import JsonObject, json_dumps, sha256_bytes

SUPERVISOR_ACTIVE_STATES = ("starting", "attached", "detached")
_TERMINAL_SUPERVISOR_STATES = ("lost", "stopped")
CONTROL_EVENT_TYPES = frozenset(
    {
        "packet_accepted",
        "agent_started",
        "artifact_observed",
        "progress",
        "agent_exited",
        "helper_error",
    }
)
HELPER_EVENT_SCHEMA_VERSION = "striatum.supervisor_helper.event.v1"
HELPER_LAUNCH_SCHEMA_VERSION = "striatum.supervisor_helper.launch.v1"
PTY_HELPER_TRANSPORT = "pty_helper"


@dataclass(frozen=True)
class _ProcessLaunch:
    pid: int
    pid_start_time: str | None
    helper_pid: int | None = None
    helper_pid_start_time: str | None = None
    metadata: dict[str, Any] | None = None


@register_pg_handler("supervise.start")
def handle_start(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = _required_text(params, "session_id")
    session, run_id, command, transport = _validate_start(ctx, session_id=session_id)

    supervisor_id = ctx.new_id("sup")
    daemon_supervisor_id = ctx.new_id("dsup")
    scratch = ctx.repo_root / ".striatum" / "scratch" / supervisor_id
    pipe_path = scratch / "stdin.pipe"
    event_path = scratch / "helper-events.jsonl"
    scratch.mkdir(parents=True, exist_ok=True)
    if pipe_path.exists():
        pipe_path.unlink()
    os.mkfifo(pipe_path, 0o600)

    started_at = ctx.now()
    try:
        with transaction(ctx):
            _insert_starting_rows(
                ctx,
                supervisor_id=supervisor_id,
                daemon_supervisor_id=daemon_supervisor_id,
                run_id=run_id,
                session_id=session_id,
                command=command,
                scratch=scratch,
                pipe_path=pipe_path,
                transport=transport,
                event_path=event_path,
                started_at=started_at,
            )
            ctx.append_event(
                run_id=run_id,
                event_type="supervisor.starting",
                actor_session_id=session_id,
                payload={
                    "supervisor_id": supervisor_id,
                    "daemon_supervisor_id": daemon_supervisor_id,
                    "adapter": "process",
                    "transport": transport,
                    "stdin_pipe_path": str(pipe_path),
                    **({"helper_events_path": str(event_path)} if transport == PTY_HELPER_TRANSPORT else {}),
                },
            )
    except Exception:
        _unlink_quietly(pipe_path)
        raise

    try:
        launch = _launch_supervised_process(
            ctx,
            command=command,
            run_id=run_id,
            session_id=session_id,
            supervisor_id=supervisor_id,
            scratch=scratch,
            pipe_path=pipe_path,
            event_path=event_path,
            transport=transport,
        )
    except OSError as exc:
        _mark_lost(
            ctx,
            supervisor_id=supervisor_id,
            run_id=run_id,
            session_id=session_id,
            reason=f"start failed: {exc}",
            payload={"phase": "start", "error": str(exc)},
        )
        _unlink_quietly(pipe_path)
        raise InvalidTransitionError(f"supervisor could not launch lane command: {exc}") from exc

    if launch.metadata:
        with transaction(ctx):
            _merge_pointer_metadata(ctx, supervisor_id=supervisor_id, metadata=launch.metadata)

    pid = launch.pid
    pid_start_time = launch.pid_start_time
    if not _pid_alive(pid):
        _mark_lost(
            ctx,
            supervisor_id=supervisor_id,
            run_id=run_id,
            session_id=session_id,
            reason="child exited before attach",
            pid=pid,
            payload={"phase": "start"},
        )
        _unlink_quietly(pipe_path)
        raise InvalidTransitionError("supervisor child exited before it could be attached")

    attached_at = ctx.now()
    with transaction(ctx):
        _update_supervisor_state(
            ctx,
            supervisor_id=supervisor_id,
            daemon_supervisor_id=daemon_supervisor_id,
            state="attached",
            updated_at=attached_at,
            pid=pid,
            pid_start_time=pid_start_time,
            heartbeat_at=attached_at,
        )
        ctx.append_event(
            run_id=run_id,
            event_type="supervisor.started",
            actor_session_id=session_id,
            payload={
                "supervisor_id": supervisor_id,
                "daemon_supervisor_id": daemon_supervisor_id,
                "pid": pid,
                "transport": transport,
                "stdin_pipe_path": str(pipe_path),
                **(
                    {
                        "helper_pid": launch.helper_pid,
                        "helper_events_path": str(event_path),
                    }
                    if transport == PTY_HELPER_TRANSPORT
                    else {}
                ),
            },
        )

    return {
        "supervisor_id": supervisor_id,
        "daemon_supervisor_id": daemon_supervisor_id,
        "session_id": session_id,
        "run_id": run_id,
        "pid": pid,
        "pid_start_time": pid_start_time,
        "stdin_pipe_path": str(pipe_path),
        "state": "attached",
        "transport": transport,
        "helper_process": (
            {
                "pid": launch.helper_pid,
                "pid_start_time": launch.helper_pid_start_time,
                "events_path": str(event_path),
            }
            if transport == PTY_HELPER_TRANSPORT
            else None
        ),
        "lane_attestation": "attested" if pid_start_time is not None else "unattested",
        "lane_id": str(session["lane_id"]),
    }


@register_pg_handler("supervise.send")
def handle_send(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = _required_text(params, "session_id")
    packet_id = _required_text(params, "packet_id")
    with transaction(ctx):
        supervisor = _require_active_supervisor(ctx, session_id=session_id, for_update=True)
        if supervisor["state"] != "attached":
            raise InvalidTransitionError(
                "supervise send requires an attached supervisor "
                f"(supervisor_id={supervisor['supervisor_id']}, state={supervisor['state']})"
            )
        packet = _work_packet(ctx, packet_id=packet_id)
        if str(packet["session_id"]) != session_id:
            raise InvalidTransitionError(
                "work packet does not belong to this session: "
                f"packet_session={packet['session_id']!r}, requested_session={session_id!r}"
            )
        if str(packet["run_id"]) != str(supervisor["run_id"]):
            raise InvalidTransitionError("work packet run does not match supervisor run")

        pipe_text = supervisor.get("stdin_pipe_path")
        pipe_path = Path(str(pipe_text)) if pipe_text else None
        if pipe_path is None or not pipe_path.exists():
            raise InvalidTransitionError(
                f"supervisor stdin pipe is missing: {pipe_path or '<unset>'}"
            )

        pid_value = supervisor.get("pid")
        pid = int(pid_value) if pid_value is not None else None
        if pid is None or not _pid_alive(pid):
            _mark_lost(
                ctx,
                supervisor_id=str(supervisor["supervisor_id"]),
                run_id=str(supervisor["run_id"]),
                session_id=session_id,
                reason="pid is gone before send",
                pid=pid,
            )
            raise InvalidTransitionError(f"supervisor pid is gone: {supervisor['supervisor_id']}")

        payload = json_dumps(packet["packet_json"])
        if not payload.endswith("\n"):
            payload += "\n"
        bytes_written = _write_to_pipe(pipe_path, payload.encode("utf-8"))
        delivered_at = ctx.now()
        _refresh_heartbeat(
            ctx,
            supervisor_id=str(supervisor["supervisor_id"]),
            daemon_supervisor_id=_daemon_supervisor_id(
                ctx, supervisor_id=str(supervisor["supervisor_id"])
            ),
            updated_at=delivered_at,
        )
        ctx.append_event(
            run_id=str(supervisor["run_id"]),
            event_type="supervisor.packet_delivered",
            actor_session_id=session_id,
            payload={
                "supervisor_id": str(supervisor["supervisor_id"]),
                "packet_id": packet_id,
                "bytes_written": bytes_written,
            },
        )
        _drain_helper_events(ctx, supervisor_id=str(supervisor["supervisor_id"]), wait_seconds=0.25)
        return {
            "supervisor_id": str(supervisor["supervisor_id"]),
            "packet_id": packet_id,
            "delivered_at": delivered_at,
            "bytes": bytes_written,
            "delivery_state": "delivered_unacknowledged",
            "control_ack_expected": True,
        }


@register_pg_handler("supervise.report")
def handle_report(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    """Record a wrapper control event on a daemon-owned supervisor.

    This is the first control-channel slice: wrappers can acknowledge packet
    receipt and report lifecycle/progress without Striatum parsing model output.
    """

    helper_source = _helper_event_source(params)
    if helper_source is not None:
        return _handle_helper_event_batch(ctx, params=params, source=helper_source)

    event_type = _required_text(params, "event_type")
    if event_type not in CONTROL_EVENT_TYPES:
        allowed = ", ".join(sorted(CONTROL_EVENT_TYPES))
        raise RpcError("schema_invalid", f"event_type must be one of: {allowed}")
    session_id = _optional_text(params.get("session_id"))
    supervisor_id = _optional_text(params.get("supervisor_id"))
    if session_id is None and supervisor_id is None:
        raise RpcError("schema_invalid", "session_id or supervisor_id is required")
    packet_id = _optional_text(params.get("packet_id"))
    payload = params.get("payload")
    if payload is None:
        payload_map: dict[str, Any] = {}
    elif isinstance(payload, Mapping):
        payload_map = dict(payload)
    else:
        raise RpcError("schema_invalid", "payload must be an object when provided")
    reported_at = _optional_text(params.get("reported_at"))

    with transaction(ctx):
        if supervisor_id is not None:
            supervisor = _supervisor_by_id(ctx, supervisor_id=supervisor_id, for_update=True)
            if session_id is not None and str(supervisor["session_id"]) != session_id:
                raise InvalidTransitionError("session_id does not match supervisor_id")
        else:
            assert session_id is not None
            supervisor = _require_active_supervisor(ctx, session_id=session_id, for_update=True)
            supervisor_id = str(supervisor["supervisor_id"])
        session_id = str(supervisor["session_id"])
        state = str(supervisor["state"])
        if state not in SUPERVISOR_ACTIVE_STATES:
            raise InvalidTransitionError(
                f"supervise report requires an active supervisor (state={state})"
            )

        now = ctx.now()
        daemon_supervisor_id = _daemon_supervisor_id(ctx, supervisor_id=supervisor_id)
        if event_type == "agent_exited":
            exit_code = payload_map.get("exit_code")
            stop_reason = (
                f"agent exited with code {exit_code}" if exit_code is not None else "agent exited"
            )
            _update_supervisor_state(
                ctx,
                supervisor_id=supervisor_id,
                daemon_supervisor_id=daemon_supervisor_id,
                state="stopped",
                updated_at=now,
                ended_at=now,
                stop_reason=stop_reason,
            )
        else:
            _refresh_heartbeat(
                ctx,
                supervisor_id=supervisor_id,
                daemon_supervisor_id=daemon_supervisor_id,
                updated_at=now,
            )

        event_payload: dict[str, Any] = {
            "supervisor_id": supervisor_id,
            "daemon_supervisor_id": daemon_supervisor_id,
            "session_id": session_id,
            "control_event_type": event_type,
        }
        if packet_id is not None:
            event_payload["packet_id"] = packet_id
        if reported_at is not None:
            event_payload["reported_at"] = reported_at
        if payload_map:
            event_payload["payload"] = payload_map
        ctx.append_event(
            run_id=str(supervisor["run_id"]),
            event_type=f"supervisor.{event_type}",
            actor_session_id=session_id,
            payload=event_payload,
        )

    return {
        "supervisor_id": supervisor_id,
        "session_id": session_id,
        "event_type": event_type,
        "recorded_at": now,
        "state": "stopped" if event_type == "agent_exited" else state,
    }


@register_pg_handler("supervise.stop")
def handle_stop(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = _required_text(params, "session_id")
    reason = _required_text(params, "reason")
    ctx.row_by_id("sessions", "session_id", session_id)
    terminal = _latest_terminal_supervisor(ctx, session_id=session_id)
    if terminal is not None:
        return {
            "supervisor_id": str(terminal["supervisor_id"]),
            "session_id": session_id,
            "pid": terminal.get("pid"),
            "state": "stopped",
            "ended_at": _maybe_ts(terminal.get("ended_at")),
            "stop_reason": terminal.get("stop_reason"),
            "signal": None,
            "note": f"supervisor was already {terminal['state']}",
        }

    with transaction(ctx):
        supervisor = _require_active_supervisor(ctx, session_id=session_id, for_update=True)
        _drain_helper_events(ctx, supervisor_id=str(supervisor["supervisor_id"]), wait_seconds=0.0)
        pid_value = supervisor.get("pid")
        pid = int(pid_value) if pid_value is not None else None
        signaled = _terminate_process(pid)
        helper_pid = _helper_pid_from_metadata(ctx, supervisor_id=str(supervisor["supervisor_id"]))
        if helper_pid is not None and helper_pid != pid:
            _terminate_process(helper_pid)
        pipe_text = supervisor.get("stdin_pipe_path")
        if pipe_text:
            _unlink_quietly(Path(str(pipe_text)))

        ended_at = ctx.now()
        daemon_supervisor_id = _daemon_supervisor_id(
            ctx, supervisor_id=str(supervisor["supervisor_id"])
        )
        _update_supervisor_state(
            ctx,
            supervisor_id=str(supervisor["supervisor_id"]),
            daemon_supervisor_id=daemon_supervisor_id,
            state="stopped",
            updated_at=ended_at,
            ended_at=ended_at,
            stop_reason=reason,
        )
        ctx.append_event(
            run_id=str(supervisor["run_id"]),
            event_type="supervisor.stopped",
            actor_session_id=session_id,
            payload={
                "supervisor_id": str(supervisor["supervisor_id"]),
                "daemon_supervisor_id": daemon_supervisor_id,
                "pid": pid,
                "reason": reason,
                "signal": signaled,
            },
        )
        return {
            "supervisor_id": str(supervisor["supervisor_id"]),
            "daemon_supervisor_id": daemon_supervisor_id,
            "session_id": session_id,
            "pid": pid,
            "state": "stopped",
            "ended_at": ended_at,
            "stop_reason": reason,
            "signal": signaled,
        }


@register_pg_handler("supervise.status")
def handle_status(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = _required_text(params, "session_id")
    ctx.row_by_id("sessions", "session_id", session_id)
    with transaction(ctx):
        supervisor = _latest_supervisor(ctx, session_id=session_id, for_update=True)
        if supervisor is None:
            raise NotFoundError(f"no supervisor recorded for session_id={session_id!r}")
        _drain_helper_events(ctx, supervisor_id=str(supervisor["supervisor_id"]), wait_seconds=0.0)
        supervisor = _latest_supervisor(ctx, session_id=session_id, for_update=True)
        if supervisor is None:
            raise NotFoundError(f"no supervisor recorded for session_id={session_id!r}")
        state = str(supervisor["state"])
        pid_value = supervisor.get("pid")
        pid = int(pid_value) if pid_value is not None else None
        liveness = "gone"
        if pid is not None and state in SUPERVISOR_ACTIVE_STATES:
            if _pid_alive(pid):
                liveness = "alive"
            else:
                reason = "pid is gone observed by status"
                _mark_lost(
                    ctx,
                    supervisor_id=str(supervisor["supervisor_id"]),
                    run_id=str(supervisor["run_id"]),
                    session_id=session_id,
                    reason=reason,
                    pid=pid,
                )
                supervisor = {
                    **supervisor,
                    "state": "lost",
                    "ended_at": ctx.now(),
                    "stop_reason": reason,
                }
        elif pid is not None and state == "stopped":
            liveness = "alive" if _pid_alive(pid) else "gone"

        view = _supervisor_view(supervisor)
        view["liveness"] = liveness
        view["lane_attestation"] = (
            "attested" if view.get("state") == "attached" and liveness == "alive" else "unattested"
        )
        view["lane_attestation_reason"] = (
            None if view["lane_attestation"] == "attested" else "no_live_attached_supervisor"
        )
        return view


@register_pg_handler("supervise.list")
def handle_list(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = _required_text(params, "run_id")
    state = _optional_text(params.get("state"))
    ctx.row_by_id("runs", "run_id", run_id)
    with ctx.cursor() as cur:
        if state is None:
            cur.execute(
                """
                SELECT *
                FROM striatumd.process_supervisors
                WHERE repository_id = %s AND run_id = %s
                ORDER BY started_at DESC, supervisor_id DESC
                """,
                (ctx.repository_id, run_id),
            )
        else:
            cur.execute(
                """
                SELECT *
                FROM striatumd.process_supervisors
                WHERE repository_id = %s AND run_id = %s AND state = %s
                ORDER BY started_at DESC, supervisor_id DESC
                """,
                (ctx.repository_id, run_id, state),
            )
        rows = [dict(row) for row in cur.fetchall()]
    return {
        "run_id": run_id,
        "state": state,
        "supervisors": [_supervisor_view(row) for row in rows],
    }


@register_pg_handler("supervise.reattach_status")
def handle_reattach_status(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    return reattach_status_payload(
        ctx,
        run_id=_optional_text(params.get("run_id")),
        session_id=_optional_text(params.get("session_id")),
        supervisor_id=_optional_text(params.get("supervisor_id")),
    )


def reattach_status_payload(
    ctx: RepoHandlerContext,
    *,
    run_id: str | None = None,
    session_id: str | None = None,
    supervisor_id: str | None = None,
) -> dict[str, Any]:
    if run_id is not None:
        ctx.row_by_id("runs", "run_id", run_id)
    if session_id is not None:
        ctx.row_by_id("sessions", "session_id", session_id)
    rows = _reattach_status_rows(
        ctx,
        run_id=run_id,
        session_id=session_id,
        supervisor_id=supervisor_id,
    )
    supervisors = [_reattach_status_view(row) for row in rows]
    summary: dict[str, int] = {
        "total": len(supervisors),
        "reattachable": 0,
        "lost_candidate": 0,
        "needs_repair": 0,
        "needs_verification": 0,
        "terminal": 0,
    }
    for supervisor in supervisors:
        state = str(supervisor["reattach_state"])
        summary[state] = summary.get(state, 0) + 1
    return {
        "run_id": run_id,
        "session_id": session_id,
        "supervisor_id": supervisor_id,
        "summary": summary,
        "supervisors": supervisors,
    }


def _reattach_status_rows(
    ctx: RepoHandlerContext,
    *,
    run_id: str | None,
    session_id: str | None,
    supervisor_id: str | None,
) -> list[dict[str, Any]]:
    clauses = ["ps.repository_id = %s"]
    values: list[Any] = [ctx.repository_id]
    if run_id is not None:
        clauses.append("ps.run_id = %s")
        values.append(run_id)
    if session_id is not None:
        clauses.append("ps.session_id = %s")
        values.append(session_id)
    if supervisor_id is not None:
        clauses.append("ps.supervisor_id = %s")
        values.append(supervisor_id)
    where = " AND ".join(clauses)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT
              ps.supervisor_id,
              ps.run_id,
              ps.session_id,
              ps.adapter,
              ps.cwd,
              ps.scratch_path,
              ps.stdin_pipe_path,
              ps.pid,
              ps.pid_start_time,
              ps.state,
              ps.started_at,
              ps.heartbeat_at,
              ps.ended_at,
              ps.stop_reason,
              p.daemon_supervisor_id AS pointer_daemon_supervisor_id,
              p.pid AS pointer_pid,
              p.pid_start_time AS pointer_pid_start_time,
              p.state AS pointer_state,
              p.updated_at AS pointer_updated_at,
              p.metadata_json AS pointer_metadata_json,
              ds.daemon_supervisor_id AS daemon_supervisor_id,
              ds.daemon_instance_id,
              ds.pid AS daemon_pid,
              ds.pid_start_time AS daemon_pid_start_time,
              ds.state AS daemon_state,
              ds.heartbeat_at AS daemon_heartbeat_at,
              ds.ended_at AS daemon_ended_at,
              ds.stop_reason AS daemon_stop_reason
            FROM striatumd.process_supervisors ps
            LEFT JOIN striatumd.process_supervisor_pointers p
              ON p.repository_id = ps.repository_id
             AND p.supervisor_id = ps.supervisor_id
            LEFT JOIN striatumd.daemon_supervisors ds
              ON ds.repository_id = ps.repository_id
             AND ds.daemon_supervisor_id = p.daemon_supervisor_id
            WHERE {where}
            ORDER BY ps.started_at DESC, ps.supervisor_id DESC
            """,
            values,
        )
        rows = [dict(row) for row in cur.fetchall()]
    if supervisor_id is not None and not rows:
        raise NotFoundError(f"no supervisor recorded for supervisor_id={supervisor_id!r}")
    return rows


def _reattach_status_view(row: Mapping[str, Any]) -> dict[str, Any]:
    pid = _optional_int(row.get("pid"))
    pid_alive = pid is not None and _pid_alive(pid)
    current_start = process_start_time(pid) if pid is not None and pid_alive else None
    state, reason, action = _reattach_state(row, pid_alive=pid_alive, current_start=current_start)
    return {
        "supervisor_id": row["supervisor_id"],
        "run_id": row["run_id"],
        "session_id": row["session_id"],
        "state": row["state"],
        "pid": pid,
        "pid_liveness": "alive" if pid_alive else "gone",
        "pid_start_time": row.get("pid_start_time"),
        "current_pid_start_time": current_start,
        "pid_identity": _pid_identity(row, pid_alive=pid_alive, current_start=current_start),
        "reattach_state": state,
        "reattach_reason": reason,
        "recommended_action": action,
        "started_at": _maybe_ts(row.get("started_at")),
        "heartbeat_at": _maybe_ts(row.get("heartbeat_at")),
        "ended_at": _maybe_ts(row.get("ended_at")),
        "stop_reason": row.get("stop_reason"),
        "stdin_pipe_path": row.get("stdin_pipe_path"),
        "pointer": {
            "daemon_supervisor_id": row.get("pointer_daemon_supervisor_id"),
            "state": row.get("pointer_state"),
            "pid": _optional_int(row.get("pointer_pid")),
            "pid_start_time": row.get("pointer_pid_start_time"),
            "updated_at": _maybe_ts(row.get("pointer_updated_at")),
            "metadata": row.get("pointer_metadata_json") or {},
        },
        "daemon_supervisor": {
            "daemon_supervisor_id": row.get("daemon_supervisor_id"),
            "daemon_instance_id": row.get("daemon_instance_id"),
            "state": row.get("daemon_state"),
            "pid": _optional_int(row.get("daemon_pid")),
            "pid_start_time": row.get("daemon_pid_start_time"),
            "heartbeat_at": _maybe_ts(row.get("daemon_heartbeat_at")),
            "ended_at": _maybe_ts(row.get("daemon_ended_at")),
            "stop_reason": row.get("daemon_stop_reason"),
        },
    }


def _reattach_state(
    row: Mapping[str, Any],
    *,
    pid_alive: bool,
    current_start: str | None,
) -> tuple[str, str | None, str]:
    state = str(row["state"])
    if state in _TERMINAL_SUPERVISOR_STATES:
        return ("terminal", state, "no_action")
    pid = _optional_int(row.get("pid"))
    if pid is None:
        return ("lost_candidate", "pid_missing", "mark_lost_or_reconcile")
    if not pid_alive:
        return ("lost_candidate", "pid_gone", "mark_lost_or_reconcile")
    expected_start = _optional_text(row.get("pid_start_time"))
    if expected_start is None or current_start is None:
        return ("needs_verification", "pid_identity_unavailable", "verify_before_reattach")
    if current_start != expected_start:
        return ("lost_candidate", "pid_identity_mismatch", "mark_lost_or_reconcile")
    pointer_state = _optional_text(row.get("pointer_state"))
    if row.get("pointer_daemon_supervisor_id") is None:
        return ("needs_repair", "pointer_missing", "repair_supervisor_pointer")
    if pointer_state != state:
        return ("needs_repair", "pointer_state_mismatch", "repair_supervisor_pointer")
    if row.get("daemon_supervisor_id") is None:
        return ("needs_repair", "daemon_supervisor_missing", "repair_daemon_supervisor")
    daemon_state = _optional_text(row.get("daemon_state"))
    if daemon_state != state:
        return ("needs_repair", "daemon_state_mismatch", "repair_daemon_supervisor")
    return ("reattachable", None, "reattach")


def _pid_identity(
    row: Mapping[str, Any],
    *,
    pid_alive: bool,
    current_start: str | None,
) -> str:
    expected_start = _optional_text(row.get("pid_start_time"))
    if not pid_alive:
        return "pid_gone"
    if expected_start is None or current_start is None:
        return "unverified"
    if current_start != expected_start:
        return "mismatch"
    return "matched"


def _helper_event_source(params: Mapping[str, Any]) -> object | None:
    if params.get("event_type") is not None:
        return None
    if "events_path" in params:
        path = params["events_path"]
        if not isinstance(path, (str, Path)):
            raise RpcError("schema_invalid", "events_path must be a string path")
        return Path(path)
    for key in ("events", "events_jsonl"):
        if key in params:
            value: object = params[key]
            return value
    return None


def _handle_helper_event_batch(
    ctx: RepoHandlerContext,
    *,
    params: Mapping[str, Any],
    source: object,
) -> dict[str, Any]:
    session_id = _optional_text(params.get("session_id"))
    supervisor_id = _optional_text(params.get("supervisor_id"))
    if session_id is None and supervisor_id is None:
        raise RpcError("schema_invalid", "session_id or supervisor_id is required")

    events = _parse_helper_events(source)
    results: list[dict[str, Any]] = []
    for index, event in enumerate(events, start=1):
        report_params = _normalize_helper_event(
            event,
            default_session_id=session_id,
            default_supervisor_id=supervisor_id,
            index=index,
        )
        results.append(handle_report(ctx, report_params))
    return {
        "events_recorded": len(results),
        "results": results,
        "state": results[-1]["state"] if results else None,
    }


def _parse_helper_events(source: object) -> list[Mapping[str, Any]]:
    if isinstance(source, list):
        return [
            _require_helper_event_object(value, index=index)
            for index, value in enumerate(source, start=1)
        ]
    if isinstance(source, tuple):
        return [
            _require_helper_event_object(value, index=index)
            for index, value in enumerate(source, start=1)
        ]
    if isinstance(source, Path):
        return _parse_helper_jsonl(source.read_text(encoding="utf-8"))
    if isinstance(source, str):
        return _parse_helper_jsonl(source)
    raise RpcError(
        "schema_invalid",
        "helper events must be provided as JSONL text, a path, or a list of objects",
    )


def _parse_helper_jsonl(text: str) -> list[Mapping[str, Any]]:
    events: list[Mapping[str, Any]] = []
    for line_number, line in enumerate(text.splitlines(), start=1):
        stripped = line.strip()
        if not stripped:
            continue
        try:
            decoded = json.loads(stripped)
        except json.JSONDecodeError as exc:
            raise RpcError(
                "schema_invalid",
                f"invalid helper event JSON on line {line_number}: {exc.msg}",
            ) from exc
        events.append(_require_helper_event_object(decoded, index=line_number))
    return events


def _require_helper_event_object(value: object, *, index: int) -> Mapping[str, Any]:
    if not isinstance(value, Mapping):
        raise RpcError("schema_invalid", f"helper event {index} must be an object")
    return value


def _normalize_helper_event(
    event: Mapping[str, Any],
    *,
    default_session_id: str | None,
    default_supervisor_id: str | None,
    index: int,
) -> dict[str, Any]:
    schema_version = event.get("schema_version")
    if schema_version not in (None, "", HELPER_EVENT_SCHEMA_VERSION):
        raise RpcError(
            "schema_invalid",
            f"helper event {index} has unsupported schema_version {schema_version!r}",
        )
    event_type = event.get("event_type")
    if not isinstance(event_type, str) or not event_type:
        raise RpcError("schema_invalid", f"helper event {index} requires event_type")
    if event_type not in CONTROL_EVENT_TYPES:
        allowed = ", ".join(sorted(CONTROL_EVENT_TYPES))
        raise RpcError(
            "schema_invalid",
            f"helper event {index} event_type must be one of: {allowed}",
        )

    supervisor_id = _optional_text(event.get("supervisor_id")) or default_supervisor_id
    session_id = _optional_text(event.get("session_id")) or default_session_id
    if supervisor_id is None and session_id is None:
        raise RpcError(
            "schema_invalid",
            f"helper event {index} requires supervisor_id or session_id",
        )
    packet_id = _optional_text(event.get("packet_id"))
    timestamp = _optional_text(event.get("timestamp"))
    payload = event.get("payload")
    if payload is None:
        payload_map: dict[str, Any] = {}
    elif isinstance(payload, Mapping):
        payload_map = dict(payload)
    else:
        raise RpcError("schema_invalid", f"helper event {index} payload must be an object")

    normalized: dict[str, Any] = {
        "event_type": event_type,
        "payload": payload_map,
    }
    if supervisor_id is not None:
        normalized["supervisor_id"] = supervisor_id
    if session_id is not None:
        normalized["session_id"] = session_id
    if packet_id is not None:
        normalized["packet_id"] = packet_id
    if timestamp is not None:
        normalized["reported_at"] = timestamp
    return normalized


def _validate_start(
    ctx: RepoHandlerContext, *, session_id: str
) -> tuple[dict[str, Any], str, list[str], str]:
    with transaction(ctx):
        session = ctx.row_by_id("sessions", "session_id", session_id, for_update=True)
        if session["state"] != "active":
            raise InvalidTransitionError("supervise start requires an active session")
        run_id = str(session["run_id"])
        ctx.row_by_id("runs", "run_id", run_id, for_update=True)
        workflow = workflow_for_run(ctx, run_id=run_id)
        lane = _lane_config(workflow, lane_id=str(session["lane_id"]))
        if lane.get("adapter") != "process":
            raise InvalidTransitionError("supervise start requires a process-adapter lane")
        command = _command_array(lane)
        transport = _supervision_transport(lane)
        existing = _active_supervisor(ctx, session_id=session_id, for_update=True)
        if existing is not None:
            raise InvalidTransitionError(
                "session already has an active supervisor: "
                f"{existing['supervisor_id']} (state={existing['state']})"
            )
        return session, run_id, command, transport


def _insert_starting_rows(
    ctx: RepoHandlerContext,
    *,
    supervisor_id: str,
    daemon_supervisor_id: str,
    run_id: str,
    session_id: str,
    command: list[str],
    scratch: Path,
    pipe_path: Path,
    transport: str,
    event_path: Path,
    started_at: str,
) -> None:
    command_json = json_dumps(command)
    daemon_instance_id = os.environ.get("STRIATUM_DAEMON_INSTANCE_ID", "python-pg-handler")
    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.process_supervisors (
              repository_id, supervisor_id, run_id, session_id, adapter,
              command_json, cwd, scratch_path, stdin_pipe_path, state, started_at
            )
            VALUES (%s, %s, %s, %s, 'process', %s, %s, %s, %s, 'starting', %s)
            """,
            (
                ctx.repository_id,
                supervisor_id,
                run_id,
                session_id,
                _jsonb(command),
                str(ctx.repo_root),
                str(scratch),
                str(pipe_path),
                started_at,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.process_supervisor_pointers (
              repository_id, supervisor_id, daemon_supervisor_id, run_id,
              session_id, state, updated_at, metadata_json
            )
            VALUES (%s, %s, %s, %s, %s, 'starting', %s, %s)
            """,
            (
                ctx.repository_id,
                supervisor_id,
                daemon_supervisor_id,
                run_id,
                session_id,
                started_at,
                _jsonb(
                    {
                        "source": "pg_supervision_handler",
                        "daemon_instance_id": daemon_instance_id,
                        "transport": transport,
                        **(
                            {
                                "helper_events_path": str(event_path),
                                "helper_events_offset": 0,
                            }
                            if transport == PTY_HELPER_TRANSPORT
                            else {}
                        ),
                    }
                ),
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.daemon_supervisors (
              daemon_supervisor_id, repository_id, run_id, session_id,
              repo_supervisor_id, daemon_instance_id, adapter, command_json,
              command_sha256, cwd, stdin_pipe_path, state, started_at
            )
            VALUES (%s, %s, %s, %s, %s, %s, 'process', %s, %s, %s, %s, 'starting', %s)
            """,
            (
                daemon_supervisor_id,
                ctx.repository_id,
                run_id,
                session_id,
                supervisor_id,
                daemon_instance_id,
                _jsonb(command),
                sha256_bytes(command_json.encode("utf-8")),
                str(ctx.repo_root),
                str(pipe_path),
                started_at,
            ),
        )


def _launch_supervised_process(
    ctx: RepoHandlerContext,
    *,
    command: list[str],
    run_id: str,
    session_id: str,
    supervisor_id: str,
    scratch: Path,
    pipe_path: Path,
    event_path: Path,
    transport: str,
) -> _ProcessLaunch:
    if transport == PTY_HELPER_TRANSPORT:
        return _launch_pty_helper(
            ctx,
            command=command,
            run_id=run_id,
            session_id=session_id,
            supervisor_id=supervisor_id,
            scratch=scratch,
            pipe_path=pipe_path,
            event_path=event_path,
        )
    child = _launch_process(
        ctx,
        command=command,
        run_id=run_id,
        session_id=session_id,
        supervisor_id=supervisor_id,
        pipe_path=pipe_path,
    )
    return _ProcessLaunch(pid=child.pid, pid_start_time=process_start_time(child.pid))


def _launch_process(
    ctx: RepoHandlerContext,
    *,
    command: list[str],
    run_id: str,
    session_id: str,
    supervisor_id: str,
    pipe_path: Path,
) -> subprocess.Popen[bytes]:
    pipe_fd = os.open(str(pipe_path), os.O_RDWR | os.O_NONBLOCK)
    try:
        flags = fcntl.fcntl(pipe_fd, fcntl.F_GETFL)
        fcntl.fcntl(pipe_fd, fcntl.F_SETFL, flags & ~os.O_NONBLOCK)
        return subprocess.Popen(
            command,
            cwd=str(ctx.repo_root),
            env=_supervised_env(
                repo=ctx.repo_root,
                run_id=run_id,
                session_id=session_id,
                supervisor_id=supervisor_id,
            ),
            stdin=pipe_fd,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
            close_fds=True,
        )
    finally:
        os.close(pipe_fd)


def _launch_pty_helper(
    ctx: RepoHandlerContext,
    *,
    command: list[str],
    run_id: str,
    session_id: str,
    supervisor_id: str,
    scratch: Path,
    pipe_path: Path,
    event_path: Path,
) -> _ProcessLaunch:
    helper = _resolve_supervisor_helper()
    launch_spec = {
        "schema_version": HELPER_LAUNCH_SCHEMA_VERSION,
        "supervisor_id": supervisor_id,
        "scratch_dir": str(scratch.parent),
        "command": command,
        "env": _supervised_env_entries(
            repo=ctx.repo_root,
            run_id=run_id,
            session_id=session_id,
            supervisor_id=supervisor_id,
        ),
        "working_dir": str(ctx.repo_root),
        "packet_input_path": str(pipe_path),
    }
    launch_spec_path = scratch / "helper-launch.json"
    launch_spec_path.write_text(json_dumps(launch_spec) + "\n", encoding="utf-8")
    event_path.write_text("", encoding="utf-8")
    with event_path.open("ab", buffering=0) as event_file:
        helper_proc = subprocess.Popen(
            [helper],
            cwd=str(ctx.repo_root),
            stdin=subprocess.PIPE,
            stdout=event_file,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
            close_fds=True,
        )
        assert helper_proc.stdin is not None
        try:
            helper_proc.stdin.write((json_dumps(launch_spec) + "\n").encode("utf-8"))
            helper_proc.stdin.close()
        except OSError:
            _terminate_process(helper_proc.pid)
            raise

        events, offset = _wait_for_helper_agent_start(
            helper_proc=helper_proc,
            event_path=event_path,
            timeout_seconds=_helper_start_timeout_seconds(),
        )

    agent_started = next(
        (event for event in events if event.get("event_type") == "agent_started"),
        None,
    )
    if agent_started is None:
        _terminate_process(helper_proc.pid)
        raise OSError("PTY helper did not report agent_started")
    payload = agent_started.get("payload")
    if not isinstance(payload, Mapping):
        _terminate_process(helper_proc.pid)
        raise OSError("PTY helper agent_started payload is invalid")
    agent_pid = _optional_int(payload.get("pid"))
    if agent_pid is None:
        _terminate_process(helper_proc.pid)
        raise OSError("PTY helper did not report agent pid")
    report_result = handle_report(ctx, {"supervisor_id": supervisor_id, "events": events})
    metadata = {
        "transport": PTY_HELPER_TRANSPORT,
        "helper_binary": helper,
        "helper_pid": helper_proc.pid,
        "helper_pid_start_time": process_start_time(helper_proc.pid),
        "helper_launch_spec_path": str(launch_spec_path),
        "helper_events_path": str(event_path),
        "helper_events_offset": offset,
        "helper_initial_events_recorded": report_result["events_recorded"],
    }
    return _ProcessLaunch(
        pid=agent_pid,
        pid_start_time=process_start_time(agent_pid),
        helper_pid=helper_proc.pid,
        helper_pid_start_time=process_start_time(helper_proc.pid),
        metadata=metadata,
    )


def _wait_for_helper_agent_start(
    *,
    helper_proc: subprocess.Popen[bytes],
    event_path: Path,
    timeout_seconds: float,
) -> tuple[list[Mapping[str, Any]], int]:
    deadline = time.time() + timeout_seconds
    last_events: list[Mapping[str, Any]] = []
    last_offset = 0
    while time.time() < deadline:
        events, offset = _read_helper_events_from_file(event_path, offset=0)
        if events:
            last_events = events
            last_offset = offset
            if any(event.get("event_type") == "agent_started" for event in events):
                return events, offset
            error_event = next(
                (event for event in events if event.get("event_type") == "helper_error"),
                None,
            )
            if error_event is not None:
                raise OSError(f"PTY helper failed before attach: {error_event.get('payload')}")
            if any(event.get("event_type") == "agent_exited" for event in events):
                raise OSError("PTY helper agent exited before attach")
        if helper_proc.poll() is not None:
            events, offset = _read_helper_events_from_file(event_path, offset=0)
            if events:
                last_events = events
                last_offset = offset
            break
        time.sleep(0.05)
    raise OSError(
        "PTY helper did not report agent_started before timeout"
        f" (events={len(last_events)}, offset={last_offset})"
    )


def _drain_helper_events(
    ctx: RepoHandlerContext,
    *,
    supervisor_id: str,
    wait_seconds: float,
) -> None:
    metadata = _pointer_metadata(ctx, supervisor_id=supervisor_id)
    if metadata.get("transport") != PTY_HELPER_TRANSPORT:
        return
    events_path = metadata.get("helper_events_path")
    if not isinstance(events_path, str) or not events_path:
        return
    path = Path(events_path)
    offset = _optional_int(metadata.get("helper_events_offset")) or 0
    deadline = time.time() + max(wait_seconds, 0.0)
    events: list[Mapping[str, Any]] = []
    new_offset = offset
    while True:
        events, new_offset = _read_helper_events_from_file(path, offset=offset)
        if events or time.time() >= deadline:
            break
        time.sleep(0.05)
    if not events and new_offset == offset:
        return
    if events:
        handle_report(ctx, {"supervisor_id": supervisor_id, "events": events})
    _merge_pointer_metadata(
        ctx,
        supervisor_id=supervisor_id,
        metadata={"helper_events_offset": new_offset},
    )


def _read_helper_events_from_file(path: Path, *, offset: int) -> tuple[list[Mapping[str, Any]], int]:
    try:
        data = path.read_bytes()
    except FileNotFoundError:
        return [], offset
    if offset >= len(data):
        return [], len(data)
    chunk = data[offset:]
    if not chunk:
        return [], offset
    complete = chunk
    new_offset = len(data)
    if not chunk.endswith(b"\n"):
        last_newline = chunk.rfind(b"\n")
        if last_newline < 0:
            return [], offset
        complete = chunk[: last_newline + 1]
        new_offset = offset + last_newline + 1
    if not complete.strip():
        return [], new_offset
    return _parse_helper_jsonl(complete.decode("utf-8")), new_offset


def _pointer_metadata(ctx: RepoHandlerContext, *, supervisor_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT metadata_json
            FROM striatumd.process_supervisor_pointers
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (ctx.repository_id, supervisor_id),
        )
        row = cur.fetchone()
    if row is None:
        return {}
    metadata = row.get("metadata_json") or {}
    return dict(metadata) if isinstance(metadata, Mapping) else {}


def _merge_pointer_metadata(
    ctx: RepoHandlerContext,
    *,
    supervisor_id: str,
    metadata: Mapping[str, Any],
) -> None:
    current = _pointer_metadata(ctx, supervisor_id=supervisor_id)
    current.update(dict(metadata))
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.process_supervisor_pointers
            SET metadata_json = %s
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (_jsonb(current), ctx.repository_id, supervisor_id),
        )


def _helper_pid_from_metadata(ctx: RepoHandlerContext, *, supervisor_id: str) -> int | None:
    metadata = _pointer_metadata(ctx, supervisor_id=supervisor_id)
    if metadata.get("transport") != PTY_HELPER_TRANSPORT:
        return None
    return _optional_int(metadata.get("helper_pid"))


def _resolve_supervisor_helper() -> str:
    override = os.environ.get("STRIATUM_SUPERVISOR_HELPER")
    if override:
        return override
    found = shutil.which("striatum-supervisor-helper")
    if found:
        return found
    repo_helper = Path(__file__).resolve().parents[4] / "go" / "bin" / "striatum-supervisor-helper"
    if repo_helper.exists():
        return str(repo_helper)
    raise OSError(
        "striatum-supervisor-helper not found; set STRIATUM_SUPERVISOR_HELPER "
        "or run `make daemon-go-helper-check` to build go/bin/striatum-supervisor-helper"
    )


def _helper_start_timeout_seconds() -> float:
    raw = os.environ.get("STRIATUM_SUPERVISOR_HELPER_START_TIMEOUT", "5")
    try:
        return max(float(raw), 0.1)
    except ValueError:
        return 5.0


def _update_supervisor_state(
    ctx: RepoHandlerContext,
    *,
    supervisor_id: str,
    daemon_supervisor_id: str | None,
    state: str,
    updated_at: str,
    pid: int | None = None,
    pid_start_time: str | None = None,
    heartbeat_at: str | None = None,
    ended_at: str | None = None,
    stop_reason: str | None = None,
) -> None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.process_supervisors
            SET state = %s,
                pid = COALESCE(%s, pid),
                pid_start_time = COALESCE(%s, pid_start_time),
                heartbeat_at = COALESCE(%s, heartbeat_at),
                ended_at = %s,
                stop_reason = %s
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (
                state,
                pid,
                pid_start_time,
                heartbeat_at,
                ended_at,
                stop_reason,
                ctx.repository_id,
                supervisor_id,
            ),
        )
        cur.execute(
            """
            UPDATE striatumd.process_supervisor_pointers
            SET state = %s,
                pid = COALESCE(%s, pid),
                pid_start_time = COALESCE(%s, pid_start_time),
                updated_at = %s
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (
                state,
                pid,
                pid_start_time,
                updated_at,
                ctx.repository_id,
                supervisor_id,
            ),
        )
        if daemon_supervisor_id is not None:
            cur.execute(
                """
                UPDATE striatumd.daemon_supervisors
                SET state = %s,
                    pid = COALESCE(%s, pid),
                    pid_start_time = COALESCE(%s, pid_start_time),
                    heartbeat_at = COALESCE(%s, heartbeat_at),
                    ended_at = %s,
                    stop_reason = %s
                WHERE daemon_supervisor_id = %s AND repository_id = %s
                """,
                (
                    state,
                    pid,
                    pid_start_time,
                    heartbeat_at,
                    ended_at,
                    stop_reason,
                    daemon_supervisor_id,
                    ctx.repository_id,
                ),
            )


def _refresh_heartbeat(
    ctx: RepoHandlerContext,
    *,
    supervisor_id: str,
    daemon_supervisor_id: str | None,
    updated_at: str,
) -> None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.process_supervisors
            SET heartbeat_at = %s
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (updated_at, ctx.repository_id, supervisor_id),
        )
        cur.execute(
            """
            UPDATE striatumd.process_supervisor_pointers
            SET updated_at = %s
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (updated_at, ctx.repository_id, supervisor_id),
        )
        if daemon_supervisor_id is not None:
            cur.execute(
                """
                UPDATE striatumd.daemon_supervisors
                SET heartbeat_at = %s
                WHERE repository_id = %s AND daemon_supervisor_id = %s
                """,
                (updated_at, ctx.repository_id, daemon_supervisor_id),
            )


def _mark_lost(
    ctx: RepoHandlerContext,
    *,
    supervisor_id: str,
    run_id: str,
    session_id: str,
    reason: str,
    pid: int | None = None,
    payload: Mapping[str, Any] | None = None,
) -> None:
    now = ctx.now()
    with transaction(ctx):
        daemon_supervisor_id = _daemon_supervisor_id(ctx, supervisor_id=supervisor_id)
        _update_supervisor_state(
            ctx,
            supervisor_id=supervisor_id,
            daemon_supervisor_id=daemon_supervisor_id,
            state="lost",
            updated_at=now,
            pid=pid,
            ended_at=now,
            stop_reason=reason,
        )
        event_payload = {
            "supervisor_id": supervisor_id,
            "daemon_supervisor_id": daemon_supervisor_id,
            "pid": pid,
            "reason": reason,
        }
        if payload is not None:
            event_payload.update(payload)
        ctx.append_event(
            run_id=run_id,
            event_type="supervisor.lost",
            actor_session_id=session_id,
            payload=event_payload,
        )


def _active_supervisor(
    ctx: RepoHandlerContext, *, session_id: str, for_update: bool = False
) -> dict[str, Any] | None:
    suffix = " FOR UPDATE" if for_update else ""
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT *
            FROM striatumd.process_supervisors
            WHERE repository_id = %s
              AND session_id = %s
              AND state = ANY(%s)
            ORDER BY started_at DESC, supervisor_id DESC
            LIMIT 1{suffix}
            """,
            (ctx.repository_id, session_id, list(SUPERVISOR_ACTIVE_STATES)),
        )
        row = cur.fetchone()
    return dict(row) if row is not None else None


def _require_active_supervisor(
    ctx: RepoHandlerContext, *, session_id: str, for_update: bool = False
) -> dict[str, Any]:
    supervisor = _active_supervisor(ctx, session_id=session_id, for_update=for_update)
    if supervisor is None:
        raise InvalidTransitionError(f"no active supervisor for session_id={session_id!r}")
    return supervisor


def _supervisor_by_id(
    ctx: RepoHandlerContext, *, supervisor_id: str, for_update: bool = False
) -> dict[str, Any]:
    suffix = " FOR UPDATE" if for_update else ""
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT *
            FROM striatumd.process_supervisors
            WHERE repository_id = %s AND supervisor_id = %s
            LIMIT 1{suffix}
            """,
            (ctx.repository_id, supervisor_id),
        )
        row = cur.fetchone()
    if row is None:
        raise NotFoundError(f"no supervisor recorded for supervisor_id={supervisor_id!r}")
    return dict(row)


def _latest_supervisor(
    ctx: RepoHandlerContext, *, session_id: str, for_update: bool = False
) -> dict[str, Any] | None:
    suffix = " FOR UPDATE" if for_update else ""
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT *
            FROM striatumd.process_supervisors
            WHERE repository_id = %s AND session_id = %s
            ORDER BY started_at DESC, supervisor_id DESC
            LIMIT 1{suffix}
            """,
            (ctx.repository_id, session_id),
        )
        row = cur.fetchone()
    return dict(row) if row is not None else None


def _latest_terminal_supervisor(
    ctx: RepoHandlerContext, *, session_id: str
) -> dict[str, Any] | None:
    active = _active_supervisor(ctx, session_id=session_id)
    if active is not None:
        return None
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT *
            FROM striatumd.process_supervisors
            WHERE repository_id = %s
              AND session_id = %s
              AND state = ANY(%s)
            ORDER BY started_at DESC, supervisor_id DESC
            LIMIT 1
            """,
            (ctx.repository_id, session_id, list(_TERMINAL_SUPERVISOR_STATES)),
        )
        row = cur.fetchone()
    return dict(row) if row is not None else None


def _work_packet(ctx: RepoHandlerContext, *, packet_id: str) -> dict[str, Any]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT packet_id, run_id, session_id, lease_id, packet_json
            FROM striatumd.work_packets
            WHERE repository_id = %s AND packet_id = %s
            """,
            (ctx.repository_id, packet_id),
        )
        row = cur.fetchone()
    if row is None:
        raise NotFoundError(f"could not find work packet for packet_id={packet_id!r}")
    return dict(row)


def _daemon_supervisor_id(ctx: RepoHandlerContext, *, supervisor_id: str) -> str | None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT daemon_supervisor_id
            FROM striatumd.process_supervisor_pointers
            WHERE repository_id = %s AND supervisor_id = %s
            """,
            (ctx.repository_id, supervisor_id),
        )
        row = cur.fetchone()
    if row is None:
        return None
    return str(row["daemon_supervisor_id"])


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


def _supervision_transport(lane: JsonObject) -> str:
    supervision = lane.get("supervision")
    if supervision is None:
        return "pipe"
    if not isinstance(supervision, Mapping):
        raise InvalidTransitionError("lane supervision must be an object when provided")
    transport = supervision.get("transport", "pipe")
    if transport == "pipe":
        return "pipe"
    if transport == PTY_HELPER_TRANSPORT:
        return PTY_HELPER_TRANSPORT
    raise InvalidTransitionError("lane supervision.transport must be 'pipe' or 'pty_helper'")


def _supervisor_view(row: Mapping[str, Any]) -> dict[str, Any]:
    view = dict(row)
    for key in ("started_at", "heartbeat_at", "ended_at"):
        view[key] = _maybe_ts(view.get(key))
    return view


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


def _supervised_env_entries(
    *,
    repo: Path,
    run_id: str,
    session_id: str,
    supervisor_id: str,
) -> list[str]:
    return [
        f"STRIATUM_RUN_ID={run_id}",
        f"STRIATUM_SESSION_ID={session_id}",
        f"STRIATUM_SUPERVISOR_ID={supervisor_id}",
        f"STRIATUM_REPO={repo}",
    ]


def _terminate_process(pid: int | None) -> str | None:
    if pid is None or not _pid_alive(pid):
        return None
    signaled = None
    try:
        os.kill(pid, signal.SIGTERM)
        signaled = "SIGTERM"
    except (ProcessLookupError, PermissionError):
        return None
    deadline = time.time() + 5.0
    while time.time() < deadline:
        _reap_if_child(pid)
        if not _pid_alive(pid):
            return signaled
        time.sleep(0.05)
    try:
        os.kill(pid, signal.SIGKILL)
        signaled = "SIGKILL"
    except (ProcessLookupError, PermissionError):
        return signaled
    deadline = time.time() + 1.0
    while time.time() < deadline and _pid_alive(pid):
        _reap_if_child(pid)
        time.sleep(0.02)
    return signaled


def _pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return False
    return True


def _reap_if_child(pid: int) -> None:
    try:
        os.waitpid(pid, os.WNOHANG)
    except ChildProcessError:
        return
    except OSError:
        return


def _write_to_pipe(pipe_path: Path, payload: bytes) -> int:
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
                raise InvalidTransitionError("supervisor pipe write returned zero bytes")
            total += written
        return total
    finally:
        os.close(fd)


def _unlink_quietly(path: Path) -> None:
    try:
        path.unlink()
    except FileNotFoundError:
        pass
    except OSError:
        pass


def _required_text(params: Mapping[str, Any], key: str) -> str:
    value = params.get(key)
    if not isinstance(value, str) or not value:
        raise InvalidTransitionError(f"{key} is required")
    return value


def _optional_text(value: object) -> str | None:
    if value is None:
        return None
    text = str(value)
    return text if text else None


def _optional_int(value: object) -> int | None:
    if value is None:
        return None
    if isinstance(value, int):
        return value
    return int(str(value))


def _maybe_ts(value: object) -> str | None:
    return None if value is None else _ts(value)


__all__ = [
    "CONTROL_EVENT_TYPES",
    "SUPERVISOR_ACTIVE_STATES",
    "handle_list",
    "handle_report",
    "handle_send",
    "handle_start",
    "handle_status",
    "handle_stop",
    "handle_reattach_status",
    "reattach_status_payload",
]

"""Supervisor heartbeat-stall projections and recovery actions."""

from __future__ import annotations

import json
from collections.abc import Mapping
from datetime import UTC, datetime
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, _ts

from ._sql import execute

DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS = 300
MAX_PRE_EXPIRY_LEASE_SECONDS = 24 * 60 * 60
STALL_BLOCKER_KIND = "heartbeat_stall_lease_expired"
STALL_EVENT_TYPE = "supervisor.heartbeat_stall"
STALL_RELEASE_REASON = "heartbeat_stall"


def supervisor_progress_for_session(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    stall_after_seconds: int = DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS,
) -> dict[str, Any] | None:
    rows = supervisor_progress_rows(
        ctx,
        session_id=session_id,
        stall_after_seconds=stall_after_seconds,
    )
    return rows[0] if rows else None


def supervisor_progress_rows(
    ctx: RepoHandlerContext,
    *,
    run_id: str | None = None,
    session_id: str | None = None,
    supervisor_id: str | None = None,
    stall_after_seconds: int = DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS,
) -> list[dict[str, Any]]:
    clauses = [
        "ps.repository_id = %(repository_id)s",
        "ps.state = 'attached'",
        "l.state = 'active'",
        "l.resource_type = 'job'",
        "j.state IN ('claimed', 'running')",
    ]
    params: dict[str, Any] = {}
    if run_id is not None:
        clauses.append("ps.run_id = %(run_id)s")
        params["run_id"] = run_id
    if session_id is not None:
        clauses.append("ps.session_id = %(session_id)s")
        params["session_id"] = session_id
    if supervisor_id is not None:
        clauses.append("ps.supervisor_id = %(supervisor_id)s")
        params["supervisor_id"] = supervisor_id
    where = " AND ".join(clauses)
    rows = _fetch_all(
        ctx,
        sql=(
            "SELECT ps.supervisor_id, ps.run_id, ps.session_id, ps.pid, "
            "       ps.state AS supervisor_state, "
            "       COALESCE(ptr.last_heartbeat_at, ps.heartbeat_at) AS supervisor_heartbeat_at, "
            "       s.last_heartbeat_at AS session_last_heartbeat_at, "
            "       l.lease_id, l.resource_id AS job_id, l.acquired_at, "
            "       l.expires_at, l.last_heartbeat_at AS lease_last_heartbeat_at, "
            "       j.workflow_job_id, j.state AS job_state, "
            "       j.current_message_id AS message_id, "
            "       qm.state AS message_state "
            "FROM striatumd.process_supervisors ps "
            "JOIN striatumd.sessions s "
            "  ON s.repository_id = ps.repository_id "
            " AND s.session_id = ps.session_id "
            "JOIN striatumd.leases l "
            "  ON l.repository_id = ps.repository_id "
            " AND l.run_id = ps.run_id "
            " AND l.owner_session_id = ps.session_id "
            "JOIN striatumd.jobs j "
            "  ON j.repository_id = l.repository_id "
            " AND j.job_id = l.resource_id "
            " AND j.current_lease_id = l.lease_id "
            "LEFT JOIN striatumd.queue_messages qm "
            "  ON qm.repository_id = j.repository_id "
            " AND qm.message_id = j.current_message_id "
            "LEFT JOIN striatumd.process_supervisor_pointers ptr "
            "  ON ptr.repository_id = ps.repository_id "
            " AND ptr.supervisor_id = ps.supervisor_id "
            f"WHERE {where} "
            "ORDER BY ps.started_at DESC, ps.supervisor_id DESC"
        ),
        params=params,
    )
    now = _now_dt(ctx)
    enriched: list[dict[str, Any]] = []
    for row in rows:
        item = _enrich_progress_row(
            row,
            now=now,
            stall_after_seconds=stall_after_seconds,
        )
        enriched.append(item)
    return enriched


def stalled_supervisors(
    ctx: RepoHandlerContext,
    *,
    run_id: str | None = None,
    stall_after_seconds: int = DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS,
) -> list[dict[str, Any]]:
    return [
        row
        for row in supervisor_progress_rows(
            ctx,
            run_id=run_id,
            stall_after_seconds=stall_after_seconds,
        )
        if row["stalled"]
    ]


def record_stall_warnings(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    dry_run: bool,
    stall_after_seconds: int = DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS,
) -> list[dict[str, Any]]:
    actions: list[dict[str, Any]] = []
    for row in stalled_supervisors(
        ctx,
        run_id=run_id,
        stall_after_seconds=stall_after_seconds,
    ):
        if row["lease_expired"]:
            continue
        if _stall_event_exists(ctx, row=row, phase="pre_expiry"):
            continue
        action = _stall_action(row, kind="supervisor_heartbeat_stall", dry_run=dry_run)
        actions.append(action)
        if dry_run:
            continue
        ctx.append_event(
            run_id=run_id,
            event_type=STALL_EVENT_TYPE,
            actor_session_id=str(row["session_id"]),
            job_id=str(row["job_id"]),
            message_id=_optional_text(row.get("message_id")),
            lease_id=str(row["lease_id"]),
            payload={
                "phase": "pre_expiry",
                "supervisor_id": str(row["supervisor_id"]),
                "last_progress_at": row["last_progress_at"],
                "last_progress_age_seconds": row["last_progress_age_seconds"],
                "stall_after_seconds": row["stall_after_seconds"],
                "lease_expires_at": row["lease_expires_at"],
            },
        )
    return actions


def block_expired_stalls(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    dry_run: bool,
    stall_after_seconds: int = DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS,
) -> list[dict[str, Any]]:
    actions: list[dict[str, Any]] = []
    for row in stalled_supervisors(
        ctx,
        run_id=run_id,
        stall_after_seconds=stall_after_seconds,
    ):
        if not row["lease_expired"]:
            continue
        existing = _existing_open_stall_blocker(ctx, job_id=str(row["job_id"]))
        if existing is not None:
            continue
        if dry_run:
            actions.append(_stall_action(row, kind="supervisor_stall_block_eligible", dry_run=True))
            continue
        blocker_id = _open_stall_blocker(ctx, row=row)
        _expire_and_block_job(ctx, row=row, blocker_id=blocker_id)
        _mark_supervisor_lost(ctx, row=row)
        action = _stall_action(row, kind="supervisor_stall_blocked", dry_run=False)
        action["blocker_id"] = blocker_id
        actions.append(action)
    return actions


def _open_stall_blocker(ctx: RepoHandlerContext, *, row: Mapping[str, Any]) -> str:
    blocker_id = ctx.new_id("blk")
    now = ctx.now()
    description = (
        "attached supervisor stopped reporting progress and the active lease expired "
        f"for workflow job {row['workflow_job_id']}"
    )
    payload = {
        "schema_version": "striatum.supervisor_stall.v1",
        "supervisor_id": str(row["supervisor_id"]),
        "lease_id": str(row["lease_id"]),
        "message_id": _optional_text(row.get("message_id")),
        "last_progress_at": row["last_progress_at"],
        "last_progress_age_seconds": row["last_progress_age_seconds"],
        "stall_after_seconds": row["stall_after_seconds"],
        "lease_expires_at": row["lease_expires_at"],
        "release_reason": STALL_RELEASE_REASON,
    }
    execute(
        ctx,
        sql=(
            "INSERT INTO striatumd.blockers "
            "(repository_id, blocker_id, run_id, job_id, session_id, severity, "
            " blocker_kind, description, state, created_at, payload_json) "
            "VALUES (%(repository_id)s, %(blocker_id)s, %(run_id)s, %(job_id)s, "
            "        %(session_id)s, 'blocked', %(blocker_kind)s, %(description)s, "
            "        'open', %(now)s, %(payload_json)s::jsonb)"
        ),
        params={
            "blocker_id": blocker_id,
            "run_id": row["run_id"],
            "job_id": row["job_id"],
            "session_id": row["session_id"],
            "blocker_kind": STALL_BLOCKER_KIND,
            "description": description,
            "now": now,
            "payload_json": json.dumps(payload),
        },
    )
    return blocker_id


def _expire_and_block_job(
    ctx: RepoHandlerContext,
    *,
    row: Mapping[str, Any],
    blocker_id: str,
) -> None:
    now = ctx.now()
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.leases "
            "SET state = 'expired', released_at = %(now)s, "
            "    release_reason = %(release_reason)s "
            "WHERE repository_id = %(repository_id)s "
            "  AND lease_id = %(lease_id)s "
            "  AND state = 'active'"
        ),
        params={
            "now": now,
            "release_reason": STALL_RELEASE_REASON,
            "lease_id": row["lease_id"],
        },
    )
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.jobs "
            "SET state = 'blocked', current_lease_id = NULL "
            "WHERE repository_id = %(repository_id)s "
            "  AND job_id = %(job_id)s"
        ),
        params={"job_id": row["job_id"]},
    )
    message_id = _optional_text(row.get("message_id"))
    if message_id is not None:
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.queue_messages "
                "SET state = 'blocked', current_lease_id = NULL, updated_at = %(now)s "
                "WHERE repository_id = %(repository_id)s "
                "  AND message_id = %(message_id)s"
            ),
            params={"now": now, "message_id": message_id},
        )
    ctx.append_event(
        run_id=str(row["run_id"]),
        event_type=STALL_EVENT_TYPE,
        actor_session_id=str(row["session_id"]),
        job_id=str(row["job_id"]),
        message_id=message_id,
        lease_id=str(row["lease_id"]),
        payload={
            "phase": "lease_expired",
            "supervisor_id": str(row["supervisor_id"]),
            "blocker_id": blocker_id,
            "last_progress_at": row["last_progress_at"],
            "last_progress_age_seconds": row["last_progress_age_seconds"],
            "stall_after_seconds": row["stall_after_seconds"],
            "lease_expires_at": row["lease_expires_at"],
        },
    )
    ctx.append_event(
        run_id=str(row["run_id"]),
        event_type="lease.expired",
        actor_session_id=str(row["session_id"]),
        job_id=str(row["job_id"]),
        message_id=message_id,
        lease_id=str(row["lease_id"]),
        payload={
            "job_state": "blocked",
            "message_state": "blocked",
            "release_reason": STALL_RELEASE_REASON,
            "blocker_id": blocker_id,
            "blocker_kind": STALL_BLOCKER_KIND,
        },
    )
    ctx.append_event(
        run_id=str(row["run_id"]),
        event_type="job.blocked",
        actor_session_id=str(row["session_id"]),
        job_id=str(row["job_id"]),
        message_id=message_id,
        lease_id=str(row["lease_id"]),
        payload={
            "blocker_id": blocker_id,
            "blocker_kind": STALL_BLOCKER_KIND,
            "severity": "blocked",
        },
    )


def _mark_supervisor_lost(ctx: RepoHandlerContext, *, row: Mapping[str, Any]) -> None:
    now = ctx.now()
    reason = "lease expired after supervisor heartbeat stall"
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.process_supervisors "
            "SET state = 'lost', ended_at = %(now)s, stop_reason = %(reason)s "
            "WHERE repository_id = %(repository_id)s "
            "  AND supervisor_id = %(supervisor_id)s "
            "  AND state = 'attached'"
        ),
        params={"now": now, "reason": reason, "supervisor_id": row["supervisor_id"]},
    )
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.process_supervisor_pointers "
            "SET state = 'lost', updated_at = %(now)s "
            "WHERE repository_id = %(repository_id)s "
            "  AND supervisor_id = %(supervisor_id)s"
        ),
        params={"now": now, "supervisor_id": row["supervisor_id"]},
    )
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.daemon_supervisors "
            "SET state = 'lost', ended_at = %(now)s, stop_reason = %(reason)s "
            "WHERE repository_id = %(repository_id)s "
            "  AND repo_supervisor_id = %(supervisor_id)s "
            "  AND state IN ('starting', 'attached', 'detached')"
        ),
        params={"now": now, "reason": reason, "supervisor_id": row["supervisor_id"]},
    )
    ctx.append_event(
        run_id=str(row["run_id"]),
        event_type="supervisor.lease_expired_with_supervisor",
        actor_session_id=str(row["session_id"]),
        job_id=str(row["job_id"]),
        message_id=_optional_text(row.get("message_id")),
        lease_id=str(row["lease_id"]),
        payload={
            "supervisor_id": str(row["supervisor_id"]),
            "pid": row.get("pid"),
            "reason": reason,
            "release_reason": STALL_RELEASE_REASON,
        },
    )


def _existing_open_stall_blocker(ctx: RepoHandlerContext, *, job_id: str) -> str | None:
    rows = _fetch_all(
        ctx,
        sql=(
            "SELECT blocker_id "
            "FROM striatumd.blockers "
            "WHERE repository_id = %(repository_id)s "
            "  AND job_id = %(job_id)s "
            "  AND blocker_kind = %(blocker_kind)s "
            "  AND state = 'open' "
            "ORDER BY created_at "
            "LIMIT 1"
        ),
        params={"job_id": job_id, "blocker_kind": STALL_BLOCKER_KIND},
    )
    return str(rows[0]["blocker_id"]) if rows else None


def _stall_event_exists(
    ctx: RepoHandlerContext,
    *,
    row: Mapping[str, Any],
    phase: str,
) -> bool:
    rows = _fetch_all(
        ctx,
        sql=(
            "SELECT 1 "
            "FROM striatumd.events "
            "WHERE repository_id = %(repository_id)s "
            "  AND event_type = %(event_type)s "
            "  AND lease_id = %(lease_id)s "
            "  AND payload_json->>'supervisor_id' = %(supervisor_id)s "
            "  AND payload_json->>'phase' = %(phase)s "
            "LIMIT 1"
        ),
        params={
            "event_type": STALL_EVENT_TYPE,
            "lease_id": row["lease_id"],
            "supervisor_id": row["supervisor_id"],
            "phase": phase,
        },
    )
    return bool(rows)


def _enrich_progress_row(
    row: Mapping[str, Any],
    *,
    now: datetime,
    stall_after_seconds: int,
) -> dict[str, Any]:
    candidates = [
        row.get("lease_last_heartbeat_at"),
        row.get("session_last_heartbeat_at"),
        row.get("supervisor_heartbeat_at"),
        row.get("acquired_at"),
    ]
    parsed_candidates = [value for value in (_parse_ts(item) for item in candidates) if value]
    last_progress = max(parsed_candidates) if parsed_candidates else None
    age_seconds = (
        int(max(0.0, (now - last_progress).total_seconds()))
        if last_progress is not None
        else None
    )
    expires_at = _parse_ts(row.get("expires_at"))
    lease_expired = bool(expires_at is not None and expires_at < now)
    acquired_at = _parse_ts(row.get("acquired_at"))
    lease_seconds = (
        int(max(0.0, (expires_at - acquired_at).total_seconds()))
        if expires_at is not None and acquired_at is not None
        else None
    )
    pre_expiry_stall_tracked = (
        lease_seconds is None or lease_seconds <= MAX_PRE_EXPIRY_LEASE_SECONDS
    )
    stalled = lease_expired or (
        pre_expiry_stall_tracked
        and age_seconds is not None
        and age_seconds >= stall_after_seconds
    )
    item = dict(row)
    item.update(
        {
            "last_progress_at": _format_ts(last_progress),
            "last_progress_age_seconds": age_seconds,
            "stall_after_seconds": stall_after_seconds,
            "lease_duration_seconds": lease_seconds,
            "lease_expired": lease_expired,
            "stalled": stalled,
            "lease_expires_at": _format_ts(expires_at),
            "lease_last_heartbeat_at": _format_ts(row.get("lease_last_heartbeat_at")),
            "session_last_heartbeat_at": _format_ts(row.get("session_last_heartbeat_at")),
            "supervisor_heartbeat_at": _format_ts(row.get("supervisor_heartbeat_at")),
        }
    )
    return item


def _fetch_all(
    ctx: RepoHandlerContext,
    *,
    sql: str,
    params: Mapping[str, Any],
) -> list[dict[str, Any]]:
    scoped = {"repository_id": ctx.repository_id, **params}
    with ctx.cursor() as cur:
        cur.execute(sql, scoped)
        return [dict(row) for row in cur.fetchall()]


def _stall_action(row: Mapping[str, Any], *, kind: str, dry_run: bool) -> dict[str, Any]:
    return {
        "kind": kind,
        "dry_run": dry_run,
        "supervisor_id": row["supervisor_id"],
        "session_id": row["session_id"],
        "job_id": row["job_id"],
        "workflow_job_id": row["workflow_job_id"],
        "lease_id": row["lease_id"],
        "message_id": row.get("message_id"),
        "last_progress_at": row["last_progress_at"],
        "last_progress_age_seconds": row["last_progress_age_seconds"],
        "stall_after_seconds": row["stall_after_seconds"],
        "lease_expires_at": row["lease_expires_at"],
        "lease_expired": row["lease_expired"],
    }


def _now_dt(ctx: RepoHandlerContext) -> datetime:
    return _parse_ts(ctx.now()) or datetime.now(UTC).replace(microsecond=0)


def _parse_ts(value: object) -> datetime | None:
    if value is None:
        return None
    if isinstance(value, datetime):
        dt = value if value.tzinfo is not None else value.replace(tzinfo=UTC)
        return dt.astimezone(UTC).replace(microsecond=0)
    text = str(value)
    if not text:
        return None
    try:
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=UTC)
    return parsed.astimezone(UTC).replace(microsecond=0)


def _format_ts(value: object) -> str | None:
    parsed = _parse_ts(value)
    return _ts(parsed) if parsed is not None else None


def _optional_text(value: object) -> str | None:
    return str(value) if value is not None else None


__all__ = [
    "DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS",
    "STALL_BLOCKER_KIND",
    "STALL_EVENT_TYPE",
    "STALL_RELEASE_REASON",
    "block_expired_stalls",
    "record_stall_warnings",
    "stalled_supervisors",
    "supervisor_progress_for_session",
    "supervisor_progress_rows",
]

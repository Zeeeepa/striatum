"""PG-backed ``work.block`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _jsonb,
    active_lease_for,
    transaction,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_rpc.envelope import RpcError
from striatum.escalations import (
    blocker_payload,
    is_escalation_blocker,
    normalize_blocker_request,
)


@register_pg_handler("work.block", "block")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    try:
        request = normalize_blocker_request(params)
    except ValueError as exc:
        raise RpcError("schema_invalid", str(exc)) from exc
    session_id = request["session_id"]
    job_id = request["job_id"]
    lease_id = request["lease_id"]
    kind = request["kind"]
    severity = request["severity"]
    description = request["description"]
    is_escalation = is_escalation_blocker(severity=severity, kind=kind)
    payload = blocker_payload(
        severity=severity,
        kind=kind,
        description=description,
    )
    with transaction(ctx):
        job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
        active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=job_id)
        now = ctx.now()
        blocker_id = ctx.new_id("blk")
        state = "waiting_human" if severity == "human_checkpoint" else "blocked"
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.blockers (
                  repository_id, blocker_id, run_id, job_id, session_id, severity,
                  blocker_kind, description, state, created_at, payload_json
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 'open', %s, %s)
                """,
                (
                    ctx.repository_id,
                    blocker_id,
                    job["run_id"],
                    job_id,
                    session_id,
                    severity,
                    kind,
                    description,
                    now,
                    _jsonb(payload),
                ),
            )
            if is_escalation:
                cur.execute(
                    """
                    INSERT INTO striatumd.escalation_inbox (
                      repository_id, escalation_id, run_id, job_id, session_id,
                      blocker_id, blocker_kind, severity, state, created_at, payload_json
                    )
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 'pending', %s, %s)
                    """,
                    (
                        ctx.repository_id,
                        blocker_id,
                        job["run_id"],
                        job_id,
                        session_id,
                        blocker_id,
                        kind,
                        severity,
                        now,
                        _jsonb(payload),
                    ),
                )
            cur.execute(
                "UPDATE striatumd.jobs SET state = %s, current_lease_id = NULL WHERE repository_id = %s AND job_id = %s",
                (state, ctx.repository_id, job_id),
            )
            cur.execute(
                """
                UPDATE striatumd.leases
                SET state = 'released', released_at = %s, release_reason = 'blocked'
                WHERE repository_id = %s AND lease_id = %s
                """,
                (now, ctx.repository_id, lease_id),
            )
            if job["current_message_id"] is not None:
                cur.execute(
                    """
                    UPDATE striatumd.queue_messages
                    SET state = 'blocked', current_lease_id = NULL
                    WHERE repository_id = %s AND message_id = %s
                    """,
                    (ctx.repository_id, job["current_message_id"]),
                )
        ctx.append_event(
            run_id=str(job["run_id"]),
            event_type="job.blocked",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={
                "blocker_id": blocker_id,
                "severity": severity,
                "blocker_kind": kind,
                "is_escalation": is_escalation,
                "payload_schema_version": payload["schema_version"],
            },
        )
        return {"status": "blocked", "blocker_id": blocker_id}

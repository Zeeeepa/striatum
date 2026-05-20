"""PG-backed ``work.block`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, active_lease_for, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler


@register_pg_handler("work.block", "block")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    job_id = str(params["job_id"])
    lease_id = str(params["lease_id"])
    kind = str(params["kind"])
    severity = str(params["severity"])
    description = str(params["description"])
    from striatum.escalations import ESCALATION_BLOCKER_KINDS
    is_escalation = (severity == "human_checkpoint" or kind in ESCALATION_BLOCKER_KINDS)
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
                  blocker_kind, description, state, created_at
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 'open', %s)
                """,
                (ctx.repository_id, blocker_id, job["run_id"], job_id, session_id, severity, kind, description, now),
            )
            if is_escalation:
                cur.execute(
                    """
                    INSERT INTO striatumd.escalation_inbox (
                      repository_id, escalation_id, run_id, job_id, session_id,
                      blocker_id, blocker_kind, severity, state, created_at
                    )
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 'pending', %s)
                    """,
                    (ctx.repository_id, blocker_id, job["run_id"], job_id, session_id, blocker_id, kind, severity, now),
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
            payload={"blocker_id": blocker_id, "severity": severity},
        )
        return {"status": "blocked", "blocker_id": blocker_id}

"""PG-backed ``work.release`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, active_lease_for, is_repo_write, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler


@register_pg_handler("work.release", "release")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    message_id = str(params["message_id"])
    lease_id = str(params["lease_id"])
    reason = str(params.get("reason") or "released")
    requeue = bool(params.get("requeue", False))
    with transaction(ctx):
        message = ctx.row_by_id("queue_messages", "message_id", message_id, for_update=True)
        job = ctx.row_by_id("jobs", "job_id", str(message["job_id"]), for_update=True)
        active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
        now = ctx.now()
        if requeue and not is_repo_write(job):
            job_state = "queued"
            msg_state = "pending"
        else:
            job_state = "blocked"
            msg_state = "blocked"
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.leases
                SET state = 'released', released_at = %s, release_reason = %s
                WHERE repository_id = %s AND lease_id = %s
                """,
                (now, reason, ctx.repository_id, lease_id),
            )
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = %s, current_lease_id = NULL
                WHERE repository_id = %s AND job_id = %s
                """,
                (job_state, ctx.repository_id, job["job_id"]),
            )
            cur.execute(
                """
                UPDATE striatumd.queue_messages
                SET state = %s, current_lease_id = NULL, updated_at = %s
                WHERE repository_id = %s AND message_id = %s
                """,
                (msg_state, now, ctx.repository_id, message_id),
            )
        ctx.append_event(
            run_id=str(job["run_id"]),
            event_type="lease.released",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=lease_id,
            payload={"reason": reason, "job_state": job_state},
        )
        return {"status": "released", "job_state": job_state}

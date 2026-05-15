"""PG-backed ``work.ack`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, active_lease_for, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import InvalidTransitionError


@register_pg_handler("work.ack", "ack")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    message_id = str(params["message_id"])
    lease_id = str(params["lease_id"])
    return ack_inline(
        ctx,
        session_id=session_id,
        message_id=message_id,
        lease_id=lease_id,
    )


def ack_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    message_id: str,
    lease_id: str,
) -> dict[str, Any]:
    """Run the work.ack substrate transition.

    RFC 0048 V1.5 #2: ``recovery.auto`` live-mode auto-publish calls
    this inside its own transaction so publish + ack + complete are
    serialized as one block. psycopg's nested ``transaction()`` is a
    savepoint, so re-entry from another transaction is safe.
    """
    with transaction(ctx):
        message = ctx.row_by_id("queue_messages", "message_id", message_id, for_update=True)
        job = ctx.row_by_id("jobs", "job_id", str(message["job_id"]), for_update=True)
        active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
        if message["state"] == "acked":
            return {"status": "acked", "job_id": job["job_id"]}
        if message["state"] != "claimed" or job["state"] != "claimed":
            raise InvalidTransitionError("work must be claimed before ack")
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.queue_messages
                SET state = 'acked', acked_at = %s, updated_at = %s
                WHERE repository_id = %s AND message_id = %s
                """,
                (now, now, ctx.repository_id, message_id),
            )
            cur.execute(
                """
                UPDATE striatumd.jobs
                SET state = 'running', started_at = %s
                WHERE repository_id = %s AND job_id = %s
                """,
                (now, ctx.repository_id, job["job_id"]),
            )
        ctx.append_event(
            run_id=str(job["run_id"]),
            event_type="queue.acked",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=lease_id,
        )
        return {"status": "acked", "job_id": job["job_id"]}

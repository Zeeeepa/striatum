"""PG-backed ``work.heartbeat`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, active_lease_for, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler


@register_pg_handler("work.heartbeat", "heartbeat")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    lease_id = str(params["lease_id"])
    extend_seconds = int(params.get("extend_seconds") or 1800)

    with transaction(ctx):
        lease = active_lease_for(ctx, lease_id=lease_id, session_id=session_id)
        now = ctx.now()
        expires_at = ctx.expires_after(extend_seconds)
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.sessions
                SET last_heartbeat_at = %s
                WHERE repository_id = %s AND session_id = %s
                """,
                (now, ctx.repository_id, session_id),
            )
            cur.execute(
                """
                UPDATE striatumd.leases
                SET last_heartbeat_at = %s, expires_at = %s
                WHERE repository_id = %s AND lease_id = %s
                """,
                (now, expires_at, ctx.repository_id, lease_id),
            )
        ctx.append_event(
            run_id=str(lease["run_id"]),
            event_type="lease.heartbeat",
            actor_session_id=session_id,
            job_id=str(lease["resource_id"]),
            lease_id=lease_id,
            payload={"expires_at": expires_at},
        )
        return {"status": "heartbeat", "expires_at": expires_at}

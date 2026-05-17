"""PG-backed ``session.close`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, _ts, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import InvalidTransitionError, LeaseError

_SESSION_TERMINAL_STATES = {"expired", "stopped", "lost", "closed"}


@register_pg_handler("session.close")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    session_id = str(params["session_id"])
    reason = str(params.get("reason") or "").strip()
    if reason == "":
        raise InvalidTransitionError("session close reason must not be empty")

    with transaction(ctx):
        session = ctx.row_by_id("sessions", "session_id", session_id, for_update=True)
        if session["state"] in _SESSION_TERMINAL_STATES:
            return {
                "session_id": session_id,
                "run_id": str(session["run_id"]),
                "role_id": str(session["role_id"]),
                "lane_id": str(session["lane_id"]),
                "state": str(session["state"]),
                "closed_at": _ts(session["closed_at"]) if session.get("closed_at") is not None else None,
                "close_reason": session["close_reason"],
                "note": f"session was already {session['state']}",
            }

        with ctx.cursor() as cur:
            cur.execute(
                """
                SELECT lease_id
                FROM striatumd.leases
                WHERE repository_id = %s AND owner_session_id = %s AND state = 'active'
                LIMIT 1
                """,
                (ctx.repository_id, session_id),
            )
            active_lease = cur.fetchone()
        if active_lease is not None:
            raise LeaseError(
                f"session has an active lease ({active_lease['lease_id']}); "
                "release the lease (striatum release) before closing the session"
            )

        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.sessions
                SET state = 'closed', closed_at = %s, close_reason = %s
                WHERE repository_id = %s AND session_id = %s
                """,
                (now, reason, ctx.repository_id, session_id),
            )
        ctx.append_event(
            run_id=str(session["run_id"]),
            event_type="session.closed",
            actor_session_id=session_id,
            payload={
                "session_id": session_id,
                "role_id": session["role_id"],
                "lane_id": session["lane_id"],
                "reason": reason,
                "source": "explicit",
            },
        )
        return {
            "session_id": session_id,
            "run_id": str(session["run_id"]),
            "role_id": str(session["role_id"]),
            "lane_id": str(session["lane_id"]),
            "state": "closed",
            "closed_at": now,
            "close_reason": reason,
        }

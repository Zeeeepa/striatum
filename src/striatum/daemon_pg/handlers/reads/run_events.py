"""PG handler for ``run.events``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._registry import register_pg_handler
from ._sql import positive_limit, require_text, row_to_json


@register_pg_handler("run.events", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    since_event_id = _since_event_id(params)
    limit = min(positive_limit(params, default=100), 500)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.run_id, r.state
            FROM striatumd.runs r
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        run_row = cur.fetchone()
        if run_row is None:
            raise RpcError("not_found", f"run not found: {run_id}")
        cur.execute(
            """
            SELECT e.event_id, e.run_id, e.event_type, e.actor_session_id,
                   e.job_id, e.payload_json, e.created_at
            FROM striatumd.events e
            WHERE e.repository_id = %s
              AND e.run_id = %s
              AND e.event_id > %s
            ORDER BY e.event_id
            LIMIT %s
            """,
            (ctx.repository_id, run_id, since_event_id, limit),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    events = [_shape_event(row) for row in rows]
    run = row_to_json(run_row)
    return {
        "run": run,
        "events": events,
        "count": len(events),
        "since_event_id": since_event_id,
        "terminal": run.get("state") in {"completed", "failed", "canceled"},
    }


def _since_event_id(params: Mapping[str, Any]) -> int:
    raw = params.get("since_event_id", 0)
    if not isinstance(raw, int) or raw < 0:
        raise RpcError("schema_invalid", "since_event_id must be a non-negative integer")
    return raw


def _shape_event(row: dict[str, Any]) -> dict[str, Any]:
    return {
        "event_id": int(row["event_id"]),
        "run_id": row.get("run_id"),
        "type": row.get("event_type"),
        "actor_session_id": row.get("actor_session_id"),
        "job_id": row.get("job_id"),
        "payload": row.get("payload_json") if isinstance(row.get("payload_json"), dict) else {},
        "created_at": row.get("created_at"),
    }

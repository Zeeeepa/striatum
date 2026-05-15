"""PG handler for ``list.sessions``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext

from ._registry import register_pg_handler
from ._sql import SESSION_STATES, optional_text, parse_json_list, require_run, require_text, row_to_json, validate_choice


@register_pg_handler("list.sessions", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    state = optional_text(params, "state")
    role = optional_text(params, "role")
    lane = optional_text(params, "lane")
    validate_choice(state, allowed=SESSION_STATES, label="session state")
    require_run(ctx, run_id)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT s.session_id, s.role_id, s.lane_id, s.slug, s.ordinal, s.state,
                   s.capabilities_json, s.registered_at, s.last_heartbeat_at,
                   s.operator_label, ps.supervisor_id
            FROM striatumd.sessions s
            LEFT JOIN striatumd.process_supervisors ps
              ON ps.repository_id = s.repository_id
             AND ps.session_id = s.session_id
             AND ps.state = 'attached'
            WHERE s.repository_id = %s AND s.run_id = %s
              AND (%s IS NULL OR s.state = %s)
              AND (%s IS NULL OR s.role_id = %s)
              AND (%s IS NULL OR s.lane_id = %s)
            ORDER BY s.registered_at, s.session_id
            """,
            (ctx.repository_id, run_id, state, state, role, role, lane, lane),
        )
        items = []
        for row in cur.fetchall():
            item = row_to_json(row)
            item["capabilities"] = parse_json_list(item.pop("capabilities_json", []))
            item["lane_attestation"] = "attested" if item.get("supervisor_id") else "unattested"
            item["lane_attestation_reason"] = None if item.get("supervisor_id") else "no_attached_supervisor"
            items.append(item)
    return {"items": items, "count": len(items)}

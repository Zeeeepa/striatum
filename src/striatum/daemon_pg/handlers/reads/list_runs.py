"""PG handler for ``list.runs``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext

from ._registry import register_pg_handler
from ._sql import RUN_STATES, optional_text, positive_limit, row_to_json, validate_choice


@register_pg_handler("list.runs", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    state = optional_text(params, "state")
    validate_choice(state, allowed=RUN_STATES, label="run state")
    limit = positive_limit(params)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.run_id, r.state, r.branch_name, r.created_at, r.started_at,
                   r.completed_at, w.workflow_id, w.workflow_version,
                   w.workflow_snapshot_id
            FROM striatumd.runs r
            LEFT JOIN striatumd.workflow_snapshots w
              ON w.repository_id = r.repository_id
             AND w.workflow_snapshot_id = r.workflow_snapshot_id
            WHERE r.repository_id = %s
              AND (%s::text IS NULL OR r.state = %s)
            ORDER BY r.created_at, r.run_id
            LIMIT %s
            """,
            (ctx.repository_id, state, state, limit),
        )
        items = []
        for row in cur.fetchall():
            item = row_to_json(row)
            item["workflow_identity"] = {
                "workflow_id": item.get("workflow_id"),
                "workflow_version": item.get("workflow_version"),
                "workflow_snapshot_id": item.get("workflow_snapshot_id"),
            }
            items.append(item)
    return {"items": items, "count": len(items)}

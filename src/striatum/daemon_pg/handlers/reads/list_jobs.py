"""PG handler for ``list.jobs``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext

from ._registry import register_pg_handler
from ._sql import JOB_STATES, optional_text, parse_json_object, require_run, require_text, row_to_json, validate_choice


@register_pg_handler("list.jobs", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    state = optional_text(params, "state")
    workflow_job_id = optional_text(params, "workflow_job_id")
    validate_choice(state, allowed=JOB_STATES, label="job state")
    require_run(ctx, run_id)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.job_id, j.workflow_job_id, j.state, j.role_id,
                   j.lane_selector_json, j.attempt, j.max_attempts, j.job_type,
                   j.created_at, j.started_at, j.completed_at,
                   (
                     SELECT v.verdict
                     FROM striatumd.verdicts v
                     WHERE v.repository_id = j.repository_id AND v.job_id = j.job_id
                     ORDER BY v.created_at DESC, v.verdict_id DESC
                     LIMIT 1
                   ) AS verdict
            FROM striatumd.jobs j
            WHERE j.repository_id = %s AND j.run_id = %s
              AND (%s::text IS NULL OR j.state = %s)
              AND (%s::text IS NULL OR j.workflow_job_id = %s)
            ORDER BY j.created_at, j.workflow_job_id, j.attempt
            """,
            (ctx.repository_id, run_id, state, state, workflow_job_id, workflow_job_id),
        )
        items = []
        for row in cur.fetchall():
            item = row_to_json(row)
            item["lane_id"] = parse_json_object(item.pop("lane_selector_json", {})).get("lane_id")
            if item.get("job_type") != "review":
                item["verdict"] = None
            items.append(item)
    return {"items": items, "count": len(items)}

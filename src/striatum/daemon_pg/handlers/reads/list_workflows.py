"""PG handler for ``list.workflows``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext

from ._registry import register_pg_handler
from ._sql import positive_limit, row_to_json


@register_pg_handler("list.workflows", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    limit = positive_limit(params)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT w.workflow_snapshot_id, w.workflow_id, w.workflow_version,
                   w.source_path, w.content_sha256, w.loaded_at
            FROM striatumd.workflow_snapshots w
            WHERE w.repository_id = %s
            ORDER BY w.loaded_at, w.workflow_snapshot_id
            LIMIT %s
            """,
            (ctx.repository_id, limit),
        )
        items = [row_to_json(row) for row in cur.fetchall()]
    return {"items": items, "count": len(items)}

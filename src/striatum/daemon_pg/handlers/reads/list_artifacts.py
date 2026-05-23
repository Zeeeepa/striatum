"""PG handler for ``list.artifacts``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext

from ._read_model import evidence_artifact_summaries, workflow_for_run
from ._registry import register_pg_handler
from ._sql import ARTIFACT_KINDS, optional_text, require_run, require_text, validate_choice


@register_pg_handler("list.artifacts", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    kind = optional_text(params, "kind")
    validate_choice(kind, allowed=ARTIFACT_KINDS, label="artifact kind")
    require_run(ctx, run_id)
    items = evidence_artifact_summaries(
        ctx,
        run_id=run_id,
        workflow=workflow_for_run(ctx, run_id),
    )
    if kind is not None:
        items = [item for item in items if item.get("artifact_kind") == kind]
    return {"items": items, "count": len(items)}

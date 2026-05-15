"""PG handler for ``dashboard``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext

from ._read_model import dashboard_payload
from ._registry import register_pg_handler
from ._sql import require_text


@register_pg_handler("dashboard", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    return dashboard_payload(ctx, run_id=run_id)

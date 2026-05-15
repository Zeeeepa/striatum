"""PG handler for ``status``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.envelope import RpcError

from ._read_model import status_payload
from ._registry import register_pg_handler
from ._sql import optional_text, run_exists


@register_pg_handler("status", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = optional_text(params, "run_id")
    if run_id is not None and not run_exists(ctx, run_id):
        raise RpcError("not_found", f"run not found: {run_id}")
    return status_payload(ctx, run_id=run_id)

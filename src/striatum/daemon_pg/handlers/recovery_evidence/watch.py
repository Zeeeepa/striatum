"""Fail-closed PG handler for ``recovery.watch``."""

from __future__ import annotations

from typing import Any, Mapping

from striatum.daemon_rpc.envelope import RpcError

from ._shim import RepoHandlerContext, register_pg_handler


@register_pg_handler("recovery.watch")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    _ = (ctx, params)
    raise RpcError(
        "not_implemented",
        "recovery.watch is not supported inside daemon RPC; use the daemon recovery sweep or run recovery.auto explicitly",
    )


__all__ = ["handle"]

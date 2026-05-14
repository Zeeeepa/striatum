"""Native PostgreSQL handler registry for daemon RPC methods."""

from __future__ import annotations

from collections.abc import Callable, Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext

PgHandler = Callable[[RepoHandlerContext, Mapping[str, Any]], dict[str, Any]]

_PG_HANDLERS: dict[str, PgHandler] = {}


def register_pg_handler(*methods: str) -> Callable[[PgHandler], PgHandler]:
    """Register one handler for one or more daemon RPC method names."""

    def decorate(handler: PgHandler) -> PgHandler:
        for method in methods:
            _PG_HANDLERS[method] = handler
        return handler

    return decorate


def resolve_pg_handler(method: str) -> PgHandler | None:
    """Return the native PG handler for *method*, when one is ported."""

    return _PG_HANDLERS.get(method)

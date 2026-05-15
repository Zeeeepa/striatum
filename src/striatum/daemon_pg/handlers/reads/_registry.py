"""Read-handler registration compatibility helpers."""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

from striatum.daemon_pg.handlers.registry import register_pg_handler as _register_pg_handler


def register_pg_handler(*methods: str, read_only: bool = True) -> Callable[[Any], Any]:
    """Register a read handler while preserving the Phase-C decorator shape.

    The shared registry currently lives outside this packet's write scope and
    does not yet persist handler metadata. The read package still uses the
    locked ``read_only=True`` call sites and delegates registration to the
    existing registry.
    """

    base_decorator = _register_pg_handler(*methods)

    def decorate(handler: Any) -> Any:
        setattr(handler, "read_only", bool(read_only))
        return base_decorator(handler)

    return decorate

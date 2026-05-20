"""Supervisor constants and legacy compatibility wrappers."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

SUPERVISOR_ACTIVE_STATES = ("starting", "attached", "detached")

__all__ = [
    "SUPERVISOR_ACTIVE_STATES",
]

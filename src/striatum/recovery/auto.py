"""Compatibility wrappers for the legacy repo-local SQLite recovery sweep."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

__all__ = ["run_auto_sweep", "_seconds_between"]


def run_auto_sweep(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_recovery_auto().run_auto_sweep(*args, **kwargs))


def _seconds_between(start: str, end: str) -> float:
    return cast(float, _legacy_recovery_auto()._seconds_between(start, end))


def _legacy_recovery_auto() -> Any:
    return import_module("striatum.legacy_sqlite.recovery_auto")

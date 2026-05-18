"""Legacy run-summary CLI compatibility wrappers."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

__all__ = ["run_summary_export", "run_summary_snapshot"]


def run_summary_export(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(
        JsonObject,
        _legacy_run_summary().run_summary_export(*args, **kwargs),
    )


def run_summary_snapshot(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(
        JsonObject,
        _legacy_run_summary().run_summary_snapshot(*args, **kwargs),
    )


def _legacy_run_summary() -> Any:
    return import_module("striatum.legacy_sqlite.cli_run_summary")

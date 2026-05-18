"""Legacy worktree CLI compatibility wrappers."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

__all__ = ["worktree_create", "worktree_list", "worktree_release"]


def worktree_create(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_worktree().worktree_create(*args, **kwargs))


def worktree_release(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_worktree().worktree_release(*args, **kwargs))


def worktree_list(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_worktree().worktree_list(*args, **kwargs))


def _legacy_worktree() -> Any:
    return import_module("striatum.legacy_sqlite.cli_worktree")

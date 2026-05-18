"""Legacy list CLI compatibility wrappers."""

from __future__ import annotations

from importlib import import_module
from typing import Any, cast

from striatum.primitives import JsonObject

__all__ = [
    "ARTIFACT_KINDS",
    "JOB_STATES",
    "RUN_STATES",
    "SESSION_STATES",
    "list_artifacts",
    "list_jobs",
    "list_runs",
    "list_sessions",
    "list_workflows",
]


RUN_STATES: tuple[str, ...] = (
    "needs_branch_confirmation",
    "ready",
    "running",
    "blocked",
    "completed",
    "failed",
    "canceled",
    "compromised",
)

SESSION_STATES: tuple[str, ...] = ("active", "expired", "stopped", "lost", "closed")

JOB_STATES: tuple[str, ...] = (
    "blocked",
    "queued",
    "claimed",
    "running",
    "stale_lease",
    "waiting_human",
    "completed",
    "failed",
    "canceled",
    "skipped",
)

ARTIFACT_KINDS: tuple[str, ...] = (
    "prompt",
    "finding",
    "findings_ledger",
    "synthesis",
    "marker",
    "handoff",
    "decision",
    "patch_summary",
    "test_report",
    "other",
)


def list_runs(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_list_commands().list_runs(*args, **kwargs))


def list_sessions(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_list_commands().list_sessions(*args, **kwargs))


def list_jobs(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_list_commands().list_jobs(*args, **kwargs))


def list_artifacts(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_list_commands().list_artifacts(*args, **kwargs))


def list_workflows(*args: Any, **kwargs: Any) -> JsonObject:
    return cast(JsonObject, _legacy_list_commands().list_workflows(*args, **kwargs))


def _legacy_list_commands() -> Any:
    return import_module("striatum.legacy_sqlite.cli_list_commands")

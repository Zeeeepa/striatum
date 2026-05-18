"""Legacy introspection CLI compatibility facade."""

from __future__ import annotations

from importlib import import_module
from typing import Any

PHASE_ACTIVE_STATES = frozenset(
    {
        "queued",
        "claimed",
        "running",
        "waiting_human",
        "blocked",
    }
)

_LEGACY_EXPORTS = (
    "blocked_downstream_jobs",
    "blocker_summaries",
    "blockers_for_job",
    "claimable_jobs_by_role_lane",
    "dependency_context",
    "doctor",
    "downstream_jobs",
    "events_for",
    "events_for_process",
    "human_checkpoint_context",
    "jobs_for_run",
    "jobs_for_session",
    "latest_non_accepting_verdicts",
    "latest_verdict_row",
    "phase_progress_for_run",
    "recent_events_for_run",
    "run_graph",
    "status",
    "table_exists",
    "verdicts_for_artifact",
    "why",
)

__all__ = ["PHASE_ACTIVE_STATES", *_LEGACY_EXPORTS]


def __getattr__(name: str) -> Any:
    if name in _LEGACY_EXPORTS:
        return getattr(_legacy_introspect(), name)
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")


def _legacy_introspect() -> Any:
    return import_module("striatum.legacy_sqlite.cli_introspect")

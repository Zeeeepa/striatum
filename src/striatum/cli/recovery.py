"""Legacy recovery CLI compatibility facade."""

from __future__ import annotations

from importlib import import_module
from typing import Any

CANCELABLE_JOB_STATES = frozenset(
    {"blocked", "queued", "claimed", "running", "stale_lease", "waiting_human"}
)

PROCESS_ADAPTER_BLOCKER_KINDS = frozenset(
    {
        "process_outputs_missing",
        "process_review_verdict_missing",
        "process_exit_nonzero",
        "process_timeout_exceeded",
        "process_lost_with_outputs_missing",
    }
)

PROCESS_EXIT_BLOCKER_KINDS = frozenset(
    {"process_exit_nonzero", "process_timeout_exceeded"}
)

_LEGACY_EXPORTS = (
    "auto_publish_stale_artifacts",
    "cancel_job",
    "process_reconcile",
    "requeue_stale",
    "resume_blocker",
    "stale_leases",
)

__all__ = [
    "CANCELABLE_JOB_STATES",
    "PROCESS_ADAPTER_BLOCKER_KINDS",
    "PROCESS_EXIT_BLOCKER_KINDS",
    *_LEGACY_EXPORTS,
]


def __getattr__(name: str) -> Any:
    if name in _LEGACY_EXPORTS:
        return getattr(_legacy_recovery(), name)
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")


def _legacy_recovery() -> Any:
    return import_module("striatum.legacy_sqlite.cli_recovery")

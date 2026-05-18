"""Legacy mutation CLI compatibility facade."""

from __future__ import annotations

from importlib import import_module
from typing import Any

VERDICT_JOB_TYPES = frozenset({"review", "phase_synthesis"})

_LEGACY_EXPORTS = (
    "_propagate_decision_outcome",
    "ack_work",
    "block_work",
    "branch_confirm",
    "checkpoint_resolve",
    "close_session",
    "decision_record",
    "git_create_or_checkout_branch",
    "heartbeat",
    "prevalidate_submit_review",
    "register_session",
    "release_work",
    "render_decision_markdown",
    "run_start",
    "send_message",
    "submit_review",
    "verdict_work",
)

__all__ = [
    "VERDICT_JOB_TYPES",
    *[name for name in _LEGACY_EXPORTS if not name.startswith("_")],
]


def __getattr__(name: str) -> Any:
    if name in _LEGACY_EXPORTS:
        return getattr(_legacy_mutations(), name)
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")


def _legacy_mutations() -> Any:
    return import_module("striatum.legacy_sqlite.cli_mutations")

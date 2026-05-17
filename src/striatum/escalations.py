"""Shared escalation vocabulary."""

from __future__ import annotations

ESCALATION_BLOCKER_KINDS: frozenset[str] = frozenset(
    {
        "ambiguous_goal",
        "missing_authority",
        "contradicting_decisions",
        "no_available_reviewer_lane",
        "committee_stalemate",
        "override_required",
        "ai_self_declared",
    }
)


__all__ = ["ESCALATION_BLOCKER_KINDS"]

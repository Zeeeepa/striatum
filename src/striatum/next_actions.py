"""Substrate-neutral next-action projection helpers."""

from __future__ import annotations

from striatum.primitives import JsonObject


def next_actions(
    *,
    open_blockers: list[JsonObject],
    human_checkpoints: list[JsonObject],
    non_accepting_verdicts: list[JsonObject],
    claimable_jobs: list[JsonObject],
    has_orphan_supervisor: bool = False,
    has_stale_leases: bool = False,
) -> list[str]:
    """Return deterministic coordinator next-action names."""
    actions: list[str] = []
    if claimable_jobs:
        actions.append("claim_available_work")
        actions.append("inspect_packet_with_inbox")
    if has_orphan_supervisor:
        actions.append("recover_orphan_supervisor")
    if has_stale_leases:
        actions.append("recovery_auto_publish")
    if open_blockers:
        actions.extend(["inspect_blocker", "export_run_evidence"])
    if human_checkpoints:
        actions.append("resolve_human_checkpoint")
        actions.append("derive_expected_byline")
    if non_accepting_verdicts:
        actions.append("revise_workflow_cycle")
        actions.append("derive_expected_byline")
    return list(dict.fromkeys(actions))

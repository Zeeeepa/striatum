"""Pin the UI_REWORK.md §9.9 / §9.10 parity contract.

OQ-4 in ``docs/design/UI_REWORK.md`` makes dashboard ↔ web parity
acceptance conditional on ``cli/introspect.py::next_actions`` emitting
the V1.41 burn-down verbs:

- ``inspect_packet_with_inbox`` — surfaced whenever a packet is
  claimable; signals the operator should run ``striatum inbox``.
- ``derive_expected_byline`` — surfaced alongside any verdict-
  override or checkpoint-resolution scenario; signals the operator
  should run ``striatum byline``.
- ``recovery_auto_publish`` — surfaced when ``has_stale_leases=True``.

The dashboard and web both consume this list verbatim. This test
keeps the contract honest so a future refactor of the action emitter
cannot silently drop a name the UI depends on.
"""

from __future__ import annotations

from striatum.cli.introspect import next_actions


def test_inbox_action_surfaces_on_claimable_packets() -> None:
    actions = next_actions(
        open_blockers=[],
        human_checkpoints=[],
        non_accepting_verdicts=[],
        claimable_jobs=[{"workflow_job_ids": ["j"], "lane_id": "codex", "role_id": "implementer", "count": 1}],
    )
    assert "claim_available_work" in actions
    assert "inspect_packet_with_inbox" in actions
    # Ordering: claim_available_work first, then inbox.
    assert actions.index("claim_available_work") < actions.index("inspect_packet_with_inbox")


def test_byline_action_surfaces_on_human_checkpoint() -> None:
    actions = next_actions(
        open_blockers=[],
        human_checkpoints=[{"blocker_id": "blk_t"}],
        non_accepting_verdicts=[],
        claimable_jobs=[],
    )
    assert "resolve_human_checkpoint" in actions
    assert "derive_expected_byline" in actions


def test_byline_action_surfaces_on_non_accepting_verdict() -> None:
    actions = next_actions(
        open_blockers=[],
        human_checkpoints=[],
        non_accepting_verdicts=[{"verdict_id": "v_t"}],
        claimable_jobs=[],
    )
    assert "revise_workflow_cycle" in actions
    assert "derive_expected_byline" in actions


def test_recovery_auto_publish_surfaces_when_stale_lease_flag_set() -> None:
    actions = next_actions(
        open_blockers=[],
        human_checkpoints=[],
        non_accepting_verdicts=[],
        claimable_jobs=[],
        has_stale_leases=True,
    )
    assert "recovery_auto_publish" in actions


def test_recovery_auto_publish_absent_when_no_stale_lease() -> None:
    actions = next_actions(
        open_blockers=[],
        human_checkpoints=[],
        non_accepting_verdicts=[],
        claimable_jobs=[],
        has_stale_leases=False,
    )
    assert "recovery_auto_publish" not in actions


def test_burndown_verbs_deduplicate_in_compound_state() -> None:
    """A compound situation (claimable + checkpoint + non-accept + stale)
    emits each new verb exactly once, in the order they would surface."""
    actions = next_actions(
        open_blockers=[{"blocker_id": "blk_t"}],
        human_checkpoints=[{"blocker_id": "blk_t"}],
        non_accepting_verdicts=[{"verdict_id": "v_t"}],
        claimable_jobs=[{"workflow_job_ids": ["j"], "lane_id": "codex", "role_id": "implementer", "count": 1}],
        has_orphan_supervisor=False,
        has_stale_leases=True,
    )
    assert actions.count("inspect_packet_with_inbox") == 1
    assert actions.count("derive_expected_byline") == 1
    assert actions.count("recovery_auto_publish") == 1
    # All three burn-down verbs land in the same list.
    for name in ("inspect_packet_with_inbox", "derive_expected_byline", "recovery_auto_publish"):
        assert name in actions

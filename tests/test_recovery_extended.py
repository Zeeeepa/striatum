"""End-to-end tests for explicit checkpoint and recovery cancel commands.

These cover the operator-facing resolution flows added on top of the existing
``block --severity human_checkpoint`` and ``recovery requeue-stale`` machinery:

- ``striatum checkpoint resolve --action {continue,cancel}`` resolves an open
  human-checkpoint blocker, transitions the affected job, and optionally
  records a link to a run-level decision artifact.
- ``striatum recovery cancel-job`` cancels a non-terminal job, refuses by
  default when blocked-only-through-this dependents exist, and cascades when
  ``--cascade`` is set.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any, cast

from striatum.legacy_sqlite.db import connect
from test_cli_mvp import (
    HUMAN_CHECKPOINT_WORKFLOW,
    WORKFLOW,
    claim,
    complete_claimed_job,
    data,
    init_repo,
    packet_ids,
    register,
    run_cli,
    verdict_claimed_review,
)

JsonDict = dict[str, Any]


def _drive_to_human_checkpoint(tmp_path: Path) -> tuple[str, str, str, str, str]:
    """Run the human-checkpoint fixture up to an open ``human_checkpoint`` blocker.

    Returns ``(run_id, decider_session_id, decide_job_id, decide_lease_id,
    blocker_id)``. The decide job is parked in ``waiting_human`` and the
    blocker is recorded with severity ``human_checkpoint``.
    """
    init_repo(tmp_path)
    prepared = data(
        run_cli(tmp_path, "run", "prepare", "--workflow", str(HUMAN_CHECKPOINT_WORKFLOW))
    )
    run_id = str(prepared["run_id"])
    run_cli(
        tmp_path,
        "branch",
        "confirm",
        "--run-id",
        run_id,
        "--branch",
        "wf/checkpoint-resolve-test",
    )
    run_cli(tmp_path, "run", "start", "--run-id", run_id)

    author = register(tmp_path, run_id, "author", "local")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="analysis",
        kind="handoff",
        path="docs/checkpoints/human-checkpoint-flow/ANALYSIS.md",
    )

    reviewer = register(tmp_path, run_id, "reviewer", "local")
    verdict_claimed_review(
        tmp_path,
        reviewer,
        claim(tmp_path, reviewer),
        verdict="accept",
        path="docs/checkpoints/human-checkpoint-flow/review/REVIEW.md",
    )

    decider = register(tmp_path, run_id, "human_checkpoint", "local")
    decide_packet = claim(tmp_path, decider)
    decide_job_id, decide_message_id, decide_lease_id = packet_ids(decide_packet)
    run_cli(
        tmp_path,
        "ack",
        "--session-id",
        decider,
        "--message-id",
        decide_message_id,
        "--lease-id",
        decide_lease_id,
    )
    blocked = data(
        run_cli(
            tmp_path,
            "block",
            "--session-id",
            decider,
            "--job-id",
            decide_job_id,
            "--lease-id",
            decide_lease_id,
            "--kind",
            "owner_decision",
            "--severity",
            "human_checkpoint",
            "--description",
            "operator must accept or reject the analysis",
        )
    )
    return run_id, decider, decide_job_id, decide_lease_id, str(blocked["blocker_id"])


def _job_state(repo: Path, job_id: str) -> str:
    conn = connect(repo)
    try:
        row = conn.execute(
            "SELECT state FROM jobs WHERE job_id = ?", (job_id,)
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    return str(row[0])


def _blocker_state(repo: Path, blocker_id: str) -> str:
    conn = connect(repo)
    try:
        row = conn.execute(
            "SELECT state FROM blockers WHERE blocker_id = ?", (blocker_id,)
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    return str(row[0])


def _events_for_job(repo: Path, job_id: str) -> list[tuple[str, str]]:
    conn = connect(repo)
    try:
        rows = conn.execute(
            "SELECT event_type, payload_json FROM events WHERE job_id = ? ORDER BY event_id",
            (job_id,),
        ).fetchall()
    finally:
        conn.close()
    return [(str(row[0]), str(row[1] or "{}")) for row in rows]


def test_checkpoint_resolve_continue_unblocks_job(tmp_path: Path) -> None:
    """``--action continue`` closes the blocker and re-queues the affected job."""
    (
        run_id,
        decider,
        decide_job_id,
        decide_lease_id,
        blocker_id,
    ) = _drive_to_human_checkpoint(tmp_path)
    assert _job_state(tmp_path, decide_job_id) == "waiting_human"
    assert _blocker_state(tmp_path, blocker_id) == "open"

    resolved = data(
        run_cli(
            tmp_path,
            "checkpoint",
            "resolve",
            "--blocker-id",
            blocker_id,
            "--action",
            "continue",
        )
    )
    assert resolved["status"] == "resolved"
    assert resolved["action"] == "continue"
    assert resolved["blocker_id"] == blocker_id
    assert resolved["job_id"] == decide_job_id
    assert _blocker_state(tmp_path, blocker_id) == "resolved"
    # The job should now be reachable via claim-next again. The original
    # message was retained: the job is queued, the message is pending.
    assert _job_state(tmp_path, decide_job_id) == "queued"

    # status no longer surfaces the open human checkpoint and instead surfaces
    # claim_available_work, since the decide job is reachable again.
    status_payload = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status_payload["human_checkpoints"] == []
    assert any(
        group["role_id"] == "human_checkpoint"
        for group in status_payload["claimable_jobs"]
    )

    # claim-next on the same session is fine: the same decider can re-claim
    # the work since the queue message is back to pending. We claim with a
    # fresh session here to mirror operator-driven recovery.
    second_decider = register(tmp_path, run_id, "human_checkpoint", "local")
    reclaimed = claim(tmp_path, second_decider)
    assert reclaimed["job"]["job_id"] == decide_job_id

    # The continue path emitted ``checkpoint.resolved``.
    events = _events_for_job(tmp_path, decide_job_id)
    assert any(name == "checkpoint.resolved" for name, _ in events)


def test_checkpoint_resolve_cancel_marks_job_canceled(tmp_path: Path) -> None:
    """``--action cancel`` closes the blocker and cancels the affected job.

    Downstream blocked jobs are left untouched in the human-checkpoint fixture
    because the decide job has no downstream dependents — but we explicitly
    assert the cancel path does not reactivate or auto-progress anything.
    """
    (
        run_id,
        _decider,
        decide_job_id,
        _decide_lease_id,
        blocker_id,
    ) = _drive_to_human_checkpoint(tmp_path)

    canceled = data(
        run_cli(
            tmp_path,
            "checkpoint",
            "resolve",
            "--blocker-id",
            blocker_id,
            "--action",
            "cancel",
        )
    )
    assert canceled["status"] == "resolved"
    assert canceled["action"] == "cancel"
    assert canceled["job_id"] == decide_job_id
    assert _blocker_state(tmp_path, blocker_id) == "resolved"
    assert _job_state(tmp_path, decide_job_id) == "canceled"

    events = _events_for_job(tmp_path, decide_job_id)
    assert any(name == "checkpoint.canceled" for name, _ in events)

    # The decide job has no downstream dependents in this fixture, so the run
    # should finalize once the cancel commits — every job is now in a terminal
    # state.
    status_payload = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status_payload["runs"][0]["state"] == "completed"


def test_checkpoint_resolve_links_decision_artifact(tmp_path: Path) -> None:
    """``--decision-id`` records a link to a run-level decision artifact."""
    (
        run_id,
        _decider,
        decide_job_id,
        _decide_lease_id,
        blocker_id,
    ) = _drive_to_human_checkpoint(tmp_path)

    recorded = data(
        run_cli(
            tmp_path,
            "decision",
            "record",
            "--run-id",
            run_id,
            "--path",
            "docs/checkpoints/human-checkpoint-flow/decisions/DECISION.md",
            "--decision-id",
            "human-checkpoint-flow-resolve-001",
            "--outcome",
            "accepted",
            "--title",
            "Operator accepts and continues",
            "--rationale",
            "Operator reviewed both upstream artifacts.",
        )
    )
    assert recorded["status"] == "recorded"

    resolved = data(
        run_cli(
            tmp_path,
            "checkpoint",
            "resolve",
            "--blocker-id",
            blocker_id,
            "--action",
            "continue",
            "--decision-id",
            "human-checkpoint-flow-resolve-001",
        )
    )
    assert resolved["decision_id"] == "human-checkpoint-flow-resolve-001"
    assert resolved["decision_artifact_id"] == recorded["artifact_id"]

    # The checkpoint.resolved event payload links back to the decision.
    events = _events_for_job(tmp_path, decide_job_id)
    resolved_events = [(name, json.loads(payload)) for name, payload in events]
    matches = [payload for name, payload in resolved_events if name == "checkpoint.resolved"]
    assert matches, "expected a checkpoint.resolved event"
    payload = matches[-1]
    assert payload.get("decision_id") == "human-checkpoint-flow-resolve-001"
    assert payload.get("decision_artifact_id") == recorded["artifact_id"]


def _job_id_for(repo: Path, run_id: str, workflow_job_id: str) -> str:
    conn = connect(repo)
    try:
        row = conn.execute(
            """
            SELECT job_id FROM jobs
            WHERE run_id = ? AND workflow_job_id = ?
            ORDER BY attempt DESC
            LIMIT 1
            """,
            (run_id, workflow_job_id),
        ).fetchone()
    finally:
        conn.close()
    assert row is not None, f"workflow_job_id={workflow_job_id} not found"
    return str(row[0])


def _started_rfc_ledger_run(repo: Path) -> str:
    init_repo(repo)
    prepared = data(run_cli(repo, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = str(prepared["run_id"])
    run_cli(
        repo,
        "branch",
        "confirm",
        "--run-id",
        run_id,
        "--branch",
        "wf/cancel-job-test",
    )
    run_cli(repo, "run", "start", "--run-id", run_id)
    return run_id


def test_recovery_cancel_job_blocks_when_dependents_exist_without_cascade(
    tmp_path: Path,
) -> None:
    """Cancelling ``draft`` without ``--cascade`` is refused (exit code 4)."""
    run_id = _started_rfc_ledger_run(tmp_path)
    draft_id = _job_id_for(tmp_path, run_id, "draft")

    rejected = run_cli(
        tmp_path,
        "recovery",
        "cancel-job",
        "--run-id",
        run_id,
        "--job-id",
        draft_id,
        "--reason",
        "operator chose to abandon draft",
        check=False,
    )
    assert rejected["returncode"] == 4
    error = cast(JsonDict, rejected["error"])
    message = str(error["message"])
    assert "blocked dependents" in message
    assert "review_codex" in message
    assert "review_gemini" in message

    # State unchanged: draft still queued, dependents still blocked.
    assert _job_state(tmp_path, draft_id) == "queued"
    assert _job_state(tmp_path, _job_id_for(tmp_path, run_id, "review_codex")) == "blocked"


def test_recovery_cancel_job_with_cascade_cancels_dependents(tmp_path: Path) -> None:
    """``--cascade`` cancels every dependent whose only path was through this job."""
    run_id = _started_rfc_ledger_run(tmp_path)
    draft_id = _job_id_for(tmp_path, run_id, "draft")

    canceled = data(
        run_cli(
            tmp_path,
            "recovery",
            "cancel-job",
            "--run-id",
            run_id,
            "--job-id",
            draft_id,
            "--reason",
            "operator chose to abandon draft",
            "--cascade",
        )
    )
    assert canceled["status"] == "canceled"
    assert canceled["job_id"] == draft_id
    assert canceled["cascade"] is True

    cascaded = {entry["workflow_job_id"] for entry in canceled["downstream_canceled"]}
    # Every downstream job's only path runs through ``draft`` (transitively
    # via the two reviews and the ledger), so cascade should sweep all five.
    assert cascaded == {
        "review_codex",
        "review_gemini",
        "findings_ledger",
        "synthesis",
        "final_review",
    }

    # Every job is now canceled and the run is canceled too (D055
    # follow-up: a run with no completed jobs transitions to
    # `canceled`, not `completed`).
    for workflow_job_id in (
        "draft",
        "review_codex",
        "review_gemini",
        "findings_ledger",
        "synthesis",
        "final_review",
    ):
        assert (
            _job_state(tmp_path, _job_id_for(tmp_path, run_id, workflow_job_id))
            == "canceled"
        )
    status_payload = data(run_cli(tmp_path, "status", "--run-id", run_id))
    assert status_payload["runs"][0]["state"] == "canceled"


def test_recovery_cancel_job_refuses_terminal_states(tmp_path: Path) -> None:
    """A completed job cannot be canceled (exit code 4)."""
    run_id = _started_rfc_ledger_run(tmp_path)
    # Drive the draft job to completed.
    author = register(tmp_path, run_id, "author", "codex")
    complete_claimed_job(
        tmp_path,
        author,
        claim(tmp_path, author),
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    draft_id = _job_id_for(tmp_path, run_id, "draft")
    assert _job_state(tmp_path, draft_id) == "completed"

    rejected = run_cli(
        tmp_path,
        "recovery",
        "cancel-job",
        "--run-id",
        run_id,
        "--job-id",
        draft_id,
        "--reason",
        "should not work on a completed job",
        check=False,
    )
    assert rejected["returncode"] == 4
    error = cast(JsonDict, rejected["error"])
    assert "terminal" in str(error["message"])
    # State unchanged: still completed.
    assert _job_state(tmp_path, draft_id) == "completed"

"""RFC 0020 V1: autonomous stalled-run recovery sweep.

A single sweep is composable with cron / systemd timer / ad-hoc
operator runs. The sweep is a pure orchestrator over existing
primitives — it does not introduce a new aggregate or mutate
schema. Sweep order is fixed:

1. ``expire_leases`` (always; same as today's lazy-on-CLI policy).
2. ``process_reconcile`` (when policy permits).
3. Autonomous review-only requeue (when policy permits and the
   stale row is `safe_to_reclaim_when_pending`).
4. Checkpoint-timeout escalation (always considered; the hook
   only fires when the policy declares one).
5. Eligible-blocker doctor flag (always reported; the autonomous
   loop never mutates `severity = 'blocked'` jobs).
"""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path
from typing import Any, Callable, Mapping

from striatum.db import (
    JsonObject,
    expire_leases,
    json_loads,
    row_by_id,
    transaction,
    utc_now,
)
from striatum.recovery.hooks import run_escalation_hook
from striatum.recovery.policy import resolve_policy


def run_auto_sweep(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    repo: Path,
    policy: dict[str, Any] | None = None,
    dry_run: bool = False,
    hook_runner: Callable[..., dict[str, Any]] | None = None,
    now: Callable[[], str] = utc_now,
    recovery_author: str | None = None,
) -> JsonObject:
    """Perform a single autonomous-recovery sweep against ``run_id``.

    ``policy`` is the resolved policy returned by
    :func:`striatum.recovery.resolve_policy`. When ``None``, the
    workflow snapshot's ``recovery_policy`` block is loaded and
    merged with defaults.
    """
    run = row_by_id(conn, "runs", "run_id", run_id)

    if policy is None:
        policy = _policy_from_workflow(conn, run)

    actions: list[dict[str, Any]] = []
    escalations: list[dict[str, Any]] = []
    still_stuck: list[dict[str, Any]] = []

    # Step 1: lazy lease expiry (always, same as today).
    if not dry_run:
        with transaction(conn):
            expire_leases(conn, run_id=run_id)

    # Step 2: process reconciliation (when policy permits or CLI
    # override sets it).
    if policy.get("autonomous_process_reconcile"):
        from striatum.cli.recovery import process_reconcile

        if dry_run:
            # Read-only inspection of executions.running rows.
            stale_processes = conn.execute(
                "SELECT process_id FROM process_executions "
                "WHERE run_id = ? AND state = 'running'",
                (run_id,),
            ).fetchall()
            for row in stale_processes:
                actions.append({
                    "kind": "process_reconcile_eligible",
                    "process_id": row["process_id"],
                    "dry_run": True,
                })
        else:
            result = process_reconcile(conn, run_id=run_id)
            for entry in result.get("reconciled", []):
                actions.append({
                    "kind": "process_reconciled",
                    "process_id": entry.get("process_id"),
                    "transition": entry.get("transition"),
                })

    # Step 3: autonomous review-only requeue.
    from striatum.cli.recovery import requeue_stale, stale_leases

    inventory = stale_leases(conn, run_id=run_id)
    requeued = 0
    cap = int(policy.get("max_requeues_per_sweep", 8))
    for entry in inventory.get("stale_leases", []):
        if requeued >= cap:
            still_stuck.append({
                "reason": "max_requeues_per_sweep_reached",
                "job_id": entry.get("job_id"),
            })
            continue
        if entry.get("repo_write"):
            still_stuck.append({
                "reason": "repo_write_requires_operator_inspection",
                "job_id": entry.get("job_id"),
                "workflow_job_id": entry.get("workflow_job_id"),
            })
            continue
        if not policy.get("autonomous_review_requeue"):
            still_stuck.append({
                "reason": "autonomous_review_requeue_disabled",
                "job_id": entry.get("job_id"),
                "workflow_job_id": entry.get("workflow_job_id"),
            })
            continue
        if dry_run:
            actions.append({
                "kind": "review_requeue_eligible",
                "job_id": entry.get("job_id"),
                "workflow_job_id": entry.get("workflow_job_id"),
                "dry_run": True,
            })
            requeued += 1
            continue
        result = requeue_stale(
            conn,
            run_id=run_id,
            job_id=str(entry.get("job_id")),
            recovery_author=recovery_author,
        )
        actions.append({
            "kind": "review_requeued",
            "job_id": entry.get("job_id"),
            "workflow_job_id": entry.get("workflow_job_id"),
            "result": result,
        })
        requeued += 1

    # Step 4: human_checkpoint timeout escalation.
    timeout = int(policy.get("checkpoint_timeout_seconds", 14400))
    open_checkpoints = conn.execute(
        """
        SELECT b.*
        FROM blockers b
        WHERE b.run_id = ?
          AND b.state = 'open'
          AND b.severity = 'human_checkpoint'
        """,
        (run_id,),
    ).fetchall()
    swept_at = now()
    for row in open_checkpoints:
        opened_at = row["created_at"]
        age = _seconds_between(opened_at, swept_at)
        if age < timeout:
            continue
        hook = policy.get("escalation_hook")
        envelope = {
            "kind": "checkpoint_timeout",
            "run_id": run_id,
            "blocker_id": row["blocker_id"],
            "job_id": row["job_id"],
            "severity": row["severity"],
            "opened_at": opened_at,
            "age_seconds": int(age),
            "hook": None,
        }
        if hook is None:
            envelope["hook"] = {"kind": "no_hook_configured"}
            escalations.append(envelope)
            continue
        if dry_run:
            envelope["hook"] = {"kind": hook.get("kind"), "dry_run": True}
            escalations.append(envelope)
            continue
        envelope["hook"] = _run_hook(
            hook=hook,
            repo=repo,
            envelope=envelope,
            hook_runner=hook_runner,
        )
        escalations.append(envelope)

    # Step 5: eligible-blocker doctor flagging.
    eligible_after = int(policy.get("eligible_after_seconds", 600))
    open_blockers = conn.execute(
        """
        SELECT b.*, j.state AS job_state, j.current_lease_id
        FROM blockers b
        JOIN jobs j ON j.job_id = b.job_id
        WHERE b.run_id = ?
          AND b.state = 'open'
          AND b.severity = 'blocked'
        """,
        (run_id,),
    ).fetchall()
    # GH #14: process-adapter blockers attached to terminal jobs are
    # benign trailing signals (post-completion nonzero exit, etc.).
    # When the operator opted into autonomous process reconciliation,
    # dismiss them in the same sweep so they stop showing up as
    # `blocker_recovery_eligible` indefinitely. Outside of that opt-in,
    # surface them via `still_stuck` exactly as before.
    autonomous_process = bool(policy.get("autonomous_process_reconcile"))
    from striatum.cli.recovery import (
        PROCESS_ADAPTER_BLOCKER_KINDS,
        resume_blocker,
    )

    terminal_states = {"completed", "failed", "canceled", "skipped"}
    for row in open_blockers:
        opened_at = row["created_at"]
        age = _seconds_between(opened_at, swept_at)
        if age < eligible_after:
            continue
        blocker_kind = str(row["blocker_kind"])
        job_state = str(row["job_state"])
        is_terminal_process_adapter = (
            blocker_kind in PROCESS_ADAPTER_BLOCKER_KINDS
            and job_state in terminal_states
        )
        if is_terminal_process_adapter and autonomous_process:
            if dry_run:
                actions.append({
                    "kind": "terminal_blocker_dismiss_eligible",
                    "blocker_id": row["blocker_id"],
                    "job_id": row["job_id"],
                    "blocker_kind": blocker_kind,
                    "job_state": job_state,
                    "age_seconds": int(age),
                    "dry_run": True,
                })
                continue
            try:
                dismissal = resume_blocker(
                    conn,
                    blocker_id=str(row["blocker_id"]),
                    complete=False,
                    session_id=None,
                    summary=None,
                    force=True,
                    extend_seconds=600,
                )
            except Exception as exc:  # noqa: BLE001
                still_stuck.append({
                    "reason": "terminal_blocker_dismiss_failed",
                    "blocker_id": row["blocker_id"],
                    "job_id": row["job_id"],
                    "age_seconds": int(age),
                    "error": str(exc),
                })
                continue
            actions.append({
                "kind": "terminal_blocker_dismissed",
                "blocker_id": row["blocker_id"],
                "job_id": row["job_id"],
                "blocker_kind": blocker_kind,
                "job_state": job_state,
                "age_seconds": int(age),
                "result": dismissal,
            })
            continue
        still_stuck.append({
            "reason": "blocker_recovery_eligible",
            "blocker_id": row["blocker_id"],
            "job_id": row["job_id"],
            "age_seconds": int(age),
        })

    return {
        "run_id": run_id,
        "swept_at": swept_at,
        "policy_source": policy.get("policy_source", "default"),
        "dry_run": bool(dry_run),
        "actions": actions,
        "escalations": escalations,
        "still_stuck": still_stuck,
    }


def _policy_from_workflow(
    conn: sqlite3.Connection, run: sqlite3.Row
) -> dict[str, Any]:
    """Read the run's workflow snapshot and resolve the policy."""
    snapshot_id = str(run["workflow_snapshot_id"])
    row = conn.execute(
        "SELECT workflow_json FROM workflow_snapshots "
        "WHERE workflow_snapshot_id = ?",
        (snapshot_id,),
    ).fetchone()
    workflow_payload: Mapping[str, Any] | None = None
    if row is not None:
        try:
            wf = json_loads(str(row["workflow_json"]))
        except Exception:  # noqa: BLE001 - resilience: bad snapshot → defaults
            wf = {}
        if isinstance(wf, Mapping):
            workflow_payload = wf.get("recovery_policy")
    return resolve_policy(workflow_payload=workflow_payload)


def _run_hook(
    *,
    hook: Mapping[str, Any],
    repo: Path,
    envelope: dict[str, Any],
    hook_runner: Callable[..., dict[str, Any]] | None,
) -> dict[str, Any]:
    """Dispatch one hook invocation; never raises."""
    if hook_runner is not None:
        return hook_runner(hook=hook, repo=repo, envelope=envelope)
    return run_escalation_hook(hook=hook, repo=repo, envelope=envelope)


def _seconds_between(start: str, end: str) -> float:
    """Return ``(end - start)`` in seconds for two RFC3339 strings."""
    from datetime import datetime, timezone

    def parse(s: str) -> datetime:
        if s.endswith("Z"):
            s = s[:-1] + "+00:00"
        try:
            return datetime.fromisoformat(s)
        except ValueError:
            return datetime.now(tz=timezone.utc)

    delta = parse(end) - parse(start)
    return max(0.0, delta.total_seconds())


# Re-export for `striatum.cli.dispatch` import without a circular path.
__all__ = ["run_auto_sweep", "_seconds_between"]


# Minor helper to keep mypy quiet about unused imports in slim builds.
_ = json

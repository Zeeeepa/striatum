"""PG handler for ``recovery.sweep``.

``recovery.sweep`` is the daemon RPC name for the CLI's
``striatum recovery auto`` one-shot autonomous recovery pass. It keeps the
legacy recovery-auto envelope shape while running the Phase 8
auto-finalize check before lazy lease expiry, so valid active work can
complete before the lease is marked stale.
"""

from __future__ import annotations

from collections.abc import Mapping
from datetime import UTC, datetime
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, workflow_for_run
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import NotFoundError
from striatum.recovery.hooks import run_escalation_hook
from striatum.recovery.policy import resolve_policy

from . import (
    auto_finalize,
    process_reconcile,
    requeue_stale,
    resume_blocker,
    stale_leases,
    supervisor_stalls,
)
from ._sql import fetch_all, is_repo_write_scope


@register_pg_handler("recovery.sweep")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = str(params.get("run_id") or "")
    if not run_id:
        raise NotFoundError("recovery.sweep requires run_id")

    run = ctx.row_by_id("runs", "run_id", run_id)
    workflow = workflow_for_run(ctx, run_id=run_id)
    policy = _resolve_policy(workflow, params)
    dry_run = bool(params.get("dry_run", False))
    swept_at = ctx.now()

    actions: list[dict[str, Any]] = []
    escalations: list[dict[str, Any]] = []
    still_stuck: list[dict[str, Any]] = []

    # Phase 8 checkpoint: only run auto-finalize from the sweep when the
    # workflow opted in. Do not honor any force flag here.
    if _auto_finalize_enabled(workflow) and str(run["state"]) == "running":
        _append_auto_finalize_action(
            ctx,
            params=params,
            run_id=run_id,
            dry_run=dry_run,
            actions=actions,
            still_stuck=still_stuck,
        )
        run = ctx.row_by_id("runs", "run_id", run_id)
        if str(run["state"]) != "running":
            return _envelope(
                run_id=run_id,
                run_state=str(run["state"]),
                swept_at=swept_at,
                policy=policy,
                dry_run=dry_run,
                actions=actions,
                escalations=escalations,
                still_stuck=still_stuck,
            )

    stall_actions = _append_supervisor_stall_actions(
        ctx,
        params=params,
        run_id=run_id,
        dry_run=dry_run,
        actions=actions,
    )

    inventory = (
        _stale_leases_preview(ctx, run_id=run_id)
        if dry_run
        else stale_leases.handle(ctx, {"run_id": run_id})
    )

    if policy.get("autonomous_process_reconcile"):
        _append_process_reconcile_actions(
            ctx,
            run_id=run_id,
            dry_run=dry_run,
            actions=actions,
        )

    _append_requeue_actions(
        ctx,
        run_id=run_id,
        dry_run=dry_run,
        policy=policy,
        inventory=inventory,
        skip_lease_ids=stall_actions,
        actions=actions,
        still_stuck=still_stuck,
    )
    escalations.extend(
        _checkpoint_escalations(
            ctx,
            run_id=run_id,
            policy=policy,
            dry_run=dry_run,
            swept_at=swept_at,
        )
    )
    _append_eligible_blockers(
        ctx,
        run_id=run_id,
        policy=policy,
        dry_run=dry_run,
        swept_at=swept_at,
        actions=actions,
        still_stuck=still_stuck,
    )
    run = ctx.row_by_id("runs", "run_id", run_id)

    return _envelope(
        run_id=run_id,
        run_state=str(run["state"]),
        swept_at=swept_at,
        policy=policy,
        dry_run=dry_run,
        actions=actions,
        escalations=escalations,
        still_stuck=still_stuck,
    )


def _resolve_policy(
    workflow: Mapping[str, Any],
    params: Mapping[str, Any],
) -> dict[str, Any]:
    return resolve_policy(
        workflow_payload=workflow.get("recovery_policy"),
        cli_overrides={
            "autonomous_review_requeue": params.get("autonomous_review_requeue"),
            "autonomous_process_reconcile": params.get("autonomous_process_reconcile"),
            "max_requeues_per_sweep": params.get("max_requeues_per_sweep"),
            "checkpoint_timeout_seconds": params.get("checkpoint_timeout_seconds"),
            "eligible_after_seconds": params.get("eligible_after_seconds"),
        },
    )


def _auto_finalize_enabled(workflow: Mapping[str, Any]) -> bool:
    recovery = workflow.get("recovery")
    auto_finalize_policy: object = None
    if isinstance(recovery, dict):
        auto_finalize_policy = recovery.get("auto_finalize")
    if auto_finalize_policy is None:
        auto_finalize_policy = workflow.get("auto_finalize")
    if isinstance(auto_finalize_policy, dict):
        return auto_finalize_policy.get("enabled") is True
    return auto_finalize_policy is True


def _append_auto_finalize_action(
    ctx: RepoHandlerContext,
    *,
    params: Mapping[str, Any],
    run_id: str,
    dry_run: bool,
    actions: list[dict[str, Any]],
    still_stuck: list[dict[str, Any]],
) -> None:
    auto_params: dict[str, Any] = {"run_id": run_id, "dry_run": dry_run, "force": False}
    for key in ("job_id", "mtime_grace_seconds", "allow_no_process_execution"):
        if params.get(key) is not None:
            auto_params[key] = params[key]
    try:
        result = auto_finalize.handle(ctx, auto_params)
    except Exception as exc:  # noqa: BLE001 - keep the sweep envelope useful.
        still_stuck.append({"reason": "auto_finalize_failed", "error": str(exc)})
        return
    if dry_run and int(result.get("eligible_count", 0)) > 0:
        actions.append({"kind": "auto_finalize_eligible", "dry_run": True, "result": result})
    elif not dry_run and int(result.get("finalized_count", 0)) > 0:
        actions.append({"kind": "auto_finalized", "result": result})


def _append_process_reconcile_actions(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    dry_run: bool,
    actions: list[dict[str, Any]],
) -> None:
    if dry_run:
        rows = fetch_all(
            ctx,
            sql=(
                "SELECT process_id FROM striatumd.process_executions "
                "WHERE repository_id = %(repository_id)s "
                "  AND run_id = %(run_id)s "
                "  AND state = 'running' "
                "ORDER BY started_at"
            ),
            params={"run_id": run_id},
        )
        for row in rows:
            actions.append({
                "kind": "process_reconcile_eligible",
                "process_id": row["process_id"],
                "dry_run": True,
            })
        return

    result = process_reconcile.handle(ctx, {"run_id": run_id})
    for entry in result.get("transitioned_to_lost", []):
        if not isinstance(entry, dict):
            continue
        actions.append({
            "kind": "process_reconciled",
            "process_id": entry.get("process_id"),
            "job_id": entry.get("job_id"),
            "transition": "lost",
            "blocker_kind": entry.get("blocker_kind"),
        })


def _append_supervisor_stall_actions(
    ctx: RepoHandlerContext,
    *,
    params: Mapping[str, Any],
    run_id: str,
    dry_run: bool,
    actions: list[dict[str, Any]],
) -> set[str]:
    stall_after_seconds = int(
        params.get("supervisor_stall_after_seconds")
        or supervisor_stalls.DEFAULT_SUPERVISOR_STALL_AFTER_SECONDS
    )
    generated = [
        *supervisor_stalls.record_stall_warnings(
            ctx,
            run_id=run_id,
            dry_run=dry_run,
            stall_after_seconds=stall_after_seconds,
        ),
        *supervisor_stalls.block_expired_stalls(
            ctx,
            run_id=run_id,
            dry_run=dry_run,
            stall_after_seconds=stall_after_seconds,
        ),
    ]
    actions.extend(generated)
    return {
        str(action["lease_id"])
        for action in generated
        if action.get("kind") in {"supervisor_stall_block_eligible", "supervisor_stall_blocked"}
        and action.get("lease_id") is not None
    }


def _append_requeue_actions(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    dry_run: bool,
    policy: Mapping[str, Any],
    inventory: Mapping[str, Any],
    skip_lease_ids: set[str],
    actions: list[dict[str, Any]],
    still_stuck: list[dict[str, Any]],
) -> None:
    requeued = 0
    cap = int(policy.get("max_requeues_per_sweep", 8))
    for entry in inventory.get("stale_leases", []):
        if not isinstance(entry, dict):
            continue
        lease_id = str(entry.get("lease_id")) if entry.get("lease_id") is not None else None
        if lease_id in skip_lease_ids or entry.get("release_reason") == supervisor_stalls.STALL_RELEASE_REASON:
            continue
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
        result = requeue_stale.handle(
            ctx,
            {
                "run_id": run_id,
                "job_id": str(entry.get("job_id") or ""),
            },
        )
        actions.append({
            "kind": "review_requeued",
            "job_id": entry.get("job_id"),
            "workflow_job_id": entry.get("workflow_job_id"),
            "result": result,
        })
        requeued += 1


def _checkpoint_escalations(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    policy: Mapping[str, Any],
    dry_run: bool,
    swept_at: str,
) -> list[dict[str, Any]]:
    timeout = int(policy.get("checkpoint_timeout_seconds", 14400))
    rows = fetch_all(
        ctx,
        sql=(
            "SELECT blocker_id, job_id, severity, created_at "
            "FROM striatumd.blockers "
            "WHERE repository_id = %(repository_id)s "
            "  AND run_id = %(run_id)s "
            "  AND state = 'open' "
            "  AND severity = 'human_checkpoint' "
            "ORDER BY created_at"
        ),
        params={"run_id": run_id},
    )
    escalations: list[dict[str, Any]] = []
    for row in rows:
        age = _seconds_between(str(row["created_at"]), swept_at)
        if age < timeout:
            continue
        hook = policy.get("escalation_hook")
        envelope: dict[str, Any] = {
            "kind": "checkpoint_timeout",
            "run_id": run_id,
            "blocker_id": row["blocker_id"],
            "job_id": row["job_id"],
            "severity": row["severity"],
            "opened_at": str(row["created_at"]),
            "age_seconds": int(age),
            "hook": None,
        }
        if hook is None:
            envelope["hook"] = {"kind": "no_hook_configured"}
        elif dry_run and isinstance(hook, dict):
            envelope["hook"] = {"kind": hook.get("kind"), "dry_run": True}
        elif isinstance(hook, dict):
            envelope["hook"] = run_escalation_hook(
                hook=hook,
                repo=ctx.repo_root,
                envelope=envelope,
            )
        escalations.append(envelope)
    return escalations


def _append_eligible_blockers(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    policy: Mapping[str, Any],
    dry_run: bool,
    swept_at: str,
    actions: list[dict[str, Any]],
    still_stuck: list[dict[str, Any]],
) -> None:
    eligible_after = int(policy.get("eligible_after_seconds", 600))
    rows = fetch_all(
        ctx,
        sql=(
            "SELECT b.blocker_id, b.blocker_kind, b.job_id, b.created_at, "
            "       j.state AS job_state "
            "FROM striatumd.blockers b "
            "JOIN striatumd.jobs j "
            "  ON j.repository_id = b.repository_id "
            " AND j.job_id = b.job_id "
            "WHERE b.repository_id = %(repository_id)s "
            "  AND b.run_id = %(run_id)s "
            "  AND b.state = 'open' "
            "  AND b.severity = 'blocked' "
            "ORDER BY b.created_at"
        ),
        params={"run_id": run_id},
    )
    autonomous_process = bool(policy.get("autonomous_process_reconcile"))
    for row in rows:
        age = _seconds_between(str(row["created_at"]), swept_at)
        if age < eligible_after:
            continue
        blocker_kind = str(row["blocker_kind"])
        job_state = str(row["job_state"])
        terminal_process = (
            blocker_kind in resume_blocker.PROCESS_ADAPTER_BLOCKER_KINDS
            and job_state in resume_blocker.TERMINAL_JOB_STATES
        )
        if terminal_process and autonomous_process:
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
                dismissal = resume_blocker.handle(
                    ctx,
                    {
                        "blocker_id": str(row["blocker_id"]),
                        "force": True,
                        "extend_seconds": 600,
                    },
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


def _stale_leases_preview(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    rows = fetch_all(
        ctx,
        sql=(
            "SELECT j.job_id, j.workflow_job_id, j.state AS job_state, "
            "       j.write_scope_json, "
            "       l.lease_id, l.owner_session_id, l.acquired_at, "
            "       l.expires_at, l.released_at, l.release_reason, "
            "       l.state AS lease_state, "
            "       qm.message_id, qm.state AS message_state "
            "FROM striatumd.jobs j "
            "LEFT JOIN striatumd.leases l "
            "  ON l.repository_id = j.repository_id "
            " AND (l.lease_id = j.current_lease_id "
            "      OR (l.resource_id = j.job_id AND l.state = 'expired')) "
            "LEFT JOIN striatumd.queue_messages qm "
            "  ON qm.repository_id = j.repository_id "
            " AND qm.message_id = j.current_message_id "
            "WHERE j.repository_id = %(repository_id)s "
            "  AND j.run_id = %(run_id)s "
            "  AND (j.state = 'stale_lease' "
            "       OR l.state = 'expired' "
            "       OR (j.state IN ('claimed', 'running') "
            "           AND l.state = 'active' "
            "           AND l.expires_at < %(now)s)) "
            "ORDER BY j.workflow_job_id, l.expires_at"
        ),
        params={"run_id": run_id, "now": ctx.now()},
    )
    entries: list[dict[str, Any]] = []
    seen: set[tuple[str, str | None]] = set()
    for row in rows:
        key = (
            str(row["job_id"]),
            str(row["lease_id"]) if row.get("lease_id") is not None else None,
        )
        if key in seen:
            continue
        seen.add(key)
        repo_write = is_repo_write_scope(row.get("write_scope_json"))
        entries.append({
            "job_id": row["job_id"],
            "workflow_job_id": row["workflow_job_id"],
            "job_state": row["job_state"],
            "lease_id": row.get("lease_id"),
            "owner_session_id": row.get("owner_session_id"),
            "expires_at": row.get("expires_at"),
            "released_at": row.get("released_at"),
            "release_reason": row.get("release_reason"),
            "message_id": row.get("message_id"),
            "message_state": row.get("message_state"),
            "repo_write": repo_write,
            "recovery_policy": (
                "manual_inspection_required"
                if repo_write
                else "safe_to_reclaim_when_pending"
            ),
            "next_actions": (
                ["inspect_worktree_and_artifacts", "decide_requeue_or_cancel"]
                if repo_write
                else ["register_or_select_session", "claim_available_work"]
            ),
        })
    return {
        "run_id": run_id,
        "stale_count": len(entries),
        "stale_leases": entries,
        "next_actions": (
            ["inspect_worktree_and_artifacts", "decide_requeue_or_cancel"]
            if entries
            else []
        ),
    }


def _seconds_between(start: str, end: str) -> float:
    def parse(value: str) -> datetime:
        cleaned = value.replace("Z", "+00:00")
        try:
            parsed = datetime.fromisoformat(cleaned)
        except ValueError:
            return datetime.now(tz=UTC)
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=UTC)
        return parsed

    return max(0.0, (parse(end) - parse(start)).total_seconds())


def _envelope(
    *,
    run_id: str,
    run_state: str,
    swept_at: str,
    policy: Mapping[str, Any],
    dry_run: bool,
    actions: list[dict[str, Any]],
    escalations: list[dict[str, Any]],
    still_stuck: list[dict[str, Any]],
) -> dict[str, Any]:
    return {
        "run_id": run_id,
        "run_state": run_state,
        "swept_at": swept_at,
        "policy_source": policy.get("policy_source", "default"),
        "dry_run": bool(dry_run),
        "actions": actions,
        "escalations": escalations,
        "still_stuck": still_stuck,
    }


__all__ = ["handle"]

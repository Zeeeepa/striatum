"""PG handler for ``recovery.resume``.

Port of :func:`striatum.cli.recovery.resume_blocker` (lines 373-593).
Synthesis: docs/dogfood/057/DESIGN_SYNTHESIS.md L161-169.

Inline-completion note (synthesis L167): when ``complete=True``, this handler
calls Track A's PG ``complete_job`` helper in the same transaction after
resolver state is committed in memory — *not* by re-entering RPC. Track A
must expose ``daemon_pg.handlers.workflow_loop.complete_job._complete_inline``
(or equivalent name) for that call. Until Track A lands its plumbing, the
inline-completion branch raises ``InvalidTransitionError`` so callers know
to wait for the dependency to be wired.
"""

from __future__ import annotations

from typing import Any, Mapping

from striatum.errors import InvalidTransitionError, NotFoundError

from ._shim import RepoHandlerContext, register_pg_handler
from ._sql import (
    execute,
    parse_json,
    row_by_id,
    utc_in_seconds,
    utc_now,
)
from .process_reconcile import _validate_outputs

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

TERMINAL_JOB_STATES = frozenset({"completed", "failed", "canceled", "skipped"})


@register_pg_handler("recovery.resume")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    """Resolve a process-adapter blocker after operator remediation.

    Plain resume returns the job to ``running`` using the existing claimed
    lease. With ``complete=True``, also completes the job after revalidation.
    """
    blocker_id = str(params.get("blocker_id") or "")
    if not blocker_id:
        raise NotFoundError("recovery.resume requires blocker_id")
    complete = bool(params.get("complete") or False)
    session_id_param = params.get("session_id")
    session_id = str(session_id_param) if session_id_param else None
    summary = params.get("summary")
    summary = str(summary) if summary else None
    force = bool(params.get("force") or False)
    extend_seconds_raw = params.get("extend_seconds")
    extend_seconds_param = 900 if extend_seconds_raw is None else extend_seconds_raw
    try:
        extend_seconds = int(extend_seconds_param)
    except (TypeError, ValueError) as exc:
        raise InvalidTransitionError(
            "--extend-seconds must be a positive integer"
        ) from exc
    if extend_seconds <= 0:
        raise InvalidTransitionError("--extend-seconds must be positive")
    if complete and not session_id:
        raise InvalidTransitionError("--complete requires --session-id")

    blocker = row_by_id(ctx, table="blockers", id_column="blocker_id", value=blocker_id)
    if not blocker:
        raise NotFoundError(f"blocker {blocker_id!r} not found")
    blocker_kind = str(blocker["blocker_kind"])
    if blocker_kind not in PROCESS_ADAPTER_BLOCKER_KINDS:
        raise InvalidTransitionError(
            "recovery resume supports only process-adapter blockers"
        )
    if blocker.get("job_id") is None:
        raise InvalidTransitionError("process-adapter blocker is not job-bound")
    job_id = str(blocker["job_id"])
    job = row_by_id(ctx, table="jobs", id_column="job_id", value=job_id)
    if not job:
        raise NotFoundError(f"job {job_id!r} not found")
    run_id = str(job["run_id"])
    if str(blocker["run_id"]) != run_id:
        raise InvalidTransitionError("blocker does not belong to the job run")

    if blocker["state"] != "open":
        return {
            "status": "already_resolved",
            "run_id": run_id,
            "job_id": job_id,
            "workflow_job_id": job["workflow_job_id"],
            "blocker_id": blocker_id,
            "blocker_kind": blocker_kind,
            "completed_inline": False,
            "next_actions": ["inspect_job_state"],
        }

    if str(job["state"]) in TERMINAL_JOB_STATES and force:
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.blockers "
                "SET state = 'resolved', resolved_at = %(now)s "
                "WHERE repository_id = %(repository_id)s "
                "  AND blocker_id = %(blocker_id)s"
            ),
            params={"now": utc_now(), "blocker_id": blocker_id},
        )
        ctx.append_event(
            run_id=run_id,
            event_type="recovery.blocker_dismissed_terminal",
            actor_session_id=session_id,
            job_id=job_id,
            payload={
                "blocker_id": blocker_id,
                "blocker_kind": blocker_kind,
                "job_state": str(job["state"]),
                "reason": (
                    "process-adapter blocker dismissed against terminal "
                    "job (GH #7 legacy state)"
                ),
            },
        )
        return {
            "status": "resolved_terminal_no_op",
            "run_id": run_id,
            "job_id": job_id,
            "workflow_job_id": job["workflow_job_id"],
            "blocker_id": blocker_id,
            "blocker_kind": blocker_kind,
            "completed_inline": False,
            "job_state": str(job["state"]),
            "next_actions": ["inspect_job_state"],
        }

    if job["state"] != "blocked":
        raise InvalidTransitionError(
            f"job must be blocked before recovery resume "
            f"(state={job['state']!r}); pass --force for GH #7 legacy "
            f"process-adapter blockers on terminal jobs"
        )
    if blocker_kind in PROCESS_EXIT_BLOCKER_KINDS and not force:
        raise InvalidTransitionError(
            f"{blocker_kind} requires --force after operator inspection"
        )

    missing_paths, review_verdict_missing = _validate_outputs(ctx, job=job)
    if missing_paths:
        raise InvalidTransitionError(
            "process-adapter blocker still has missing required artifacts: "
            + ", ".join(missing_paths)
        )
    if complete and review_verdict_missing:
        raise InvalidTransitionError(
            "process-adapter blocker cannot complete while review verdict is missing"
        )
    if review_verdict_missing and str(job["job_type"]) != "review":
        raise InvalidTransitionError(
            "process-adapter blocker still has a missing review verdict"
        )

    lease_id = job.get("current_lease_id")
    if lease_id is None:
        raise InvalidTransitionError(
            "process-adapter blocker job has no current lease to resume"
        )
    lease = row_by_id(ctx, table="leases", id_column="lease_id", value=str(lease_id))
    if not lease or lease["state"] != "active":
        raise InvalidTransitionError(
            f"process-adapter blocker lease is not active "
            f"(state={lease.get('state') if lease else None!r})"
        )
    lease_owner = str(lease["owner_session_id"])
    if session_id is not None and session_id != lease_owner:
        raise InvalidTransitionError("session does not own the process-adapter lease")

    now = utc_now()
    expires_at = utc_in_seconds(extend_seconds)
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.leases "
            "SET last_heartbeat_at = %(now)s, expires_at = %(expires_at)s "
            "WHERE repository_id = %(repository_id)s "
            "  AND lease_id = %(lease_id)s"
        ),
        params={"now": now, "expires_at": expires_at, "lease_id": str(lease_id)},
    )
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.blockers "
            "SET state = 'resolved', resolved_at = %(now)s "
            "WHERE repository_id = %(repository_id)s "
            "  AND blocker_id = %(blocker_id)s"
        ),
        params={"now": now, "blocker_id": blocker_id},
    )
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.jobs SET state = 'running' "
            "WHERE repository_id = %(repository_id)s "
            "  AND job_id = %(job_id)s"
        ),
        params={"job_id": job_id},
    )

    actor_session_id = (
        session_id or str(blocker.get("session_id") or lease_owner)
    )
    blocker_payload = parse_json(blocker.get("payload_json"), default={})
    event_payload: dict[str, Any] = {
        "blocker_id": blocker_id,
        "blocker_kind": blocker_kind,
        "verb": "recovery resume",
        "force": force,
        "completed_inline": complete,
        "missing_artifact_paths": missing_paths,
        "review_verdict_missing": review_verdict_missing,
        "lease_extended_until": expires_at,
    }
    if isinstance(blocker_payload, dict) and blocker_payload:
        event_payload["original_envelope"] = blocker_payload
    ctx.append_event(
        run_id=run_id,
        event_type="recovery.process_blocker_resolved",
        actor_session_id=actor_session_id,
        job_id=job_id,
        lease_id=str(lease_id),
        payload=event_payload,
    )

    result: dict[str, Any] = {
        "status": "resumed",
        "run_id": run_id,
        "job_id": job_id,
        "workflow_job_id": job["workflow_job_id"],
        "blocker_id": blocker_id,
        "blocker_kind": blocker_kind,
        "lease_id": str(lease_id),
        "lease_extended_until": expires_at,
        "force": force,
        "completed_inline": False,
        "review_verdict_missing": review_verdict_missing,
        "next_actions": (
            ["record_review_verdict"]
            if review_verdict_missing
            else ["complete_job", "monitor_run_progress"]
        ),
    }
    if not complete:
        return result

    completion = _complete_inline(
        ctx,
        session_id=str(session_id),
        job_id=job_id,
        lease_id=str(lease_id),
        summary=summary,
    )
    result["status"] = "resumed_completed"
    result["completed_inline"] = True
    result["completion"] = completion
    result["next_actions"] = ["monitor_run_progress", "export_run_evidence"]
    return result


def _complete_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    summary: str | None,
) -> dict[str, Any]:
    """Call Track A's PG inline-completion helper in the same transaction.

    Synthesis L167 requires that ``complete=True`` finishes the job in the
    same serializable transaction as resolver state — *not* by re-entering
    the RPC. Track A exposes the helper under
    ``daemon_pg.handlers.workflow_loop.complete_job`` (synthesis L59-67).
    Until that lands we raise so callers see the dependency clearly.
    """
    try:
        from striatum.daemon_pg.handlers.workflow_loop.complete_job import (  # type: ignore[import-not-found]
            complete_inline,
        )
    except ImportError as exc:
        raise InvalidTransitionError(
            "inline completion requires Track A's PG complete_job helper; "
            "rerun without --complete and call work.complete explicitly"
        ) from exc
    return complete_inline(
        ctx,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        summary=summary,
    )


__all__ = ["handle"]

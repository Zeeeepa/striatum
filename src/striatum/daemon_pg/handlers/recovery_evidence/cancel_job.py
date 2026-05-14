"""PG handler for ``recovery.cancel_job``.

Port of :func:`striatum.cli.recovery.cancel_job` (lines 283-370) plus the
helpers ``_cancel_single_job`` (219-280) and
``_dependents_blocked_only_through`` (182-216).
Synthesis: docs/dogfood/057/DESIGN_SYNTHESIS.md L141-149.
"""

from __future__ import annotations

from typing import Any, Mapping

from striatum.errors import InvalidTransitionError, NotFoundError

from ._shim import RepoHandlerContext, register_pg_handler
from ._sql import (
    execute,
    expire_leases,
    fetch_all,
    lock_run,
    maybe_complete_run,
    row_by_id,
    utc_now,
)

CANCELABLE_JOB_STATES = frozenset(
    {"blocked", "queued", "claimed", "running", "stale_lease", "waiting_human"}
)


def _dependents_blocked_only_through(
    ctx: RepoHandlerContext, *, job_id: str
) -> list[dict[str, Any]]:
    """Return blocked jobs whose only unsatisfied upstream is ``job_id``.

    A job qualifies when it is in state ``blocked`` and depends on ``job_id``
    and every other upstream dependency it has is either ``completed`` or
    ``canceled`` (i.e. cancelling ``job_id`` would orphan it). Jobs with
    another non-terminal upstream are ignored — they still have an alternate
    path.
    """
    candidates = fetch_all(
        ctx,
        sql=(
            "SELECT j.* FROM striatumd.job_dependencies dep "
            "JOIN striatumd.jobs j ON j.repository_id = dep.repository_id "
            "   AND j.job_id = dep.job_id "
            "WHERE dep.repository_id = %(repository_id)s "
            "  AND dep.depends_on_job_id = %(job_id)s "
            "  AND j.state = 'blocked' "
            "ORDER BY j.workflow_job_id"
        ),
        params={"job_id": job_id},
    )
    qualifying: list[dict[str, Any]] = []
    for candidate in candidates:
        other_deps = fetch_all(
            ctx,
            sql=(
                "SELECT up.state FROM striatumd.job_dependencies dep "
                "JOIN striatumd.jobs up ON up.repository_id = dep.repository_id "
                "   AND up.job_id = dep.depends_on_job_id "
                "WHERE dep.repository_id = %(repository_id)s "
                "  AND dep.job_id = %(candidate_id)s "
                "  AND dep.depends_on_job_id != %(job_id)s"
            ),
            params={"candidate_id": candidate["job_id"], "job_id": job_id},
        )
        only_through = all(
            row["state"] in {"completed", "canceled"} for row in other_deps
        )
        if only_through:
            qualifying.append(candidate)
    return qualifying


def _cancel_single_job(
    ctx: RepoHandlerContext,
    *,
    job_row: dict[str, Any],
    reason: str,
    now: str,
) -> dict[str, Any]:
    """Cancel one job: release leases, clear messages, mark canceled, emit event."""
    job_id = str(job_row["job_id"])
    run_id = str(job_row["run_id"])
    lease_id = job_row.get("current_lease_id")
    message_id = job_row.get("current_message_id")
    if lease_id is not None:
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.leases "
                "SET state = 'released', released_at = %(now)s, "
                "    release_reason = 'canceled' "
                "WHERE repository_id = %(repository_id)s "
                "  AND lease_id = %(lease_id)s "
                "  AND state = 'active'"
            ),
            params={"now": now, "lease_id": lease_id},
        )
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.leases "
            "SET release_reason = COALESCE(release_reason, 'canceled') "
            "WHERE repository_id = %(repository_id)s "
            "  AND resource_id = %(job_id)s "
            "  AND state = 'expired'"
        ),
        params={"job_id": job_id},
    )
    if message_id is not None:
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.queue_messages "
                "SET state = 'canceled', current_lease_id = NULL, "
                "    updated_at = %(now)s "
                "WHERE repository_id = %(repository_id)s "
                "  AND message_id = %(message_id)s"
            ),
            params={"now": now, "message_id": message_id},
        )
    execute(
        ctx,
        sql=(
            "UPDATE striatumd.jobs "
            "SET state = 'canceled', current_lease_id = NULL, "
            "    current_message_id = NULL, completed_at = %(now)s "
            "WHERE repository_id = %(repository_id)s "
            "  AND job_id = %(job_id)s"
        ),
        params={"now": now, "job_id": job_id},
    )
    ctx.append_event(
        run_id=run_id,
        event_type="job.canceled",
        job_id=job_id,
        message_id=str(message_id) if message_id is not None else None,
        lease_id=str(lease_id) if lease_id is not None else None,
        payload={"reason": reason, "workflow_job_id": job_row.get("workflow_job_id")},
    )
    return {
        "job_id": job_id,
        "workflow_job_id": job_row.get("workflow_job_id"),
        "previous_state": job_row.get("state"),
    }


@register_pg_handler("recovery.cancel_job")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    """Cancel a non-terminal job and (optionally) its orphaned blocked dependents.

    Refuses to cancel terminal-state jobs. When the job has blocked
    dependents whose only upstream path was through this job, the call
    refuses unless ``cascade`` is set; with ``cascade=True``, all such
    dependents are canceled in the same transaction (recursively).
    """
    run_id = str(params.get("run_id") or "")
    job_id = str(params.get("job_id") or "")
    reason = str(params.get("reason") or "").strip()
    cascade = bool(params.get("cascade") or False)
    if not run_id or not job_id:
        raise NotFoundError("recovery.cancel_job requires run_id and job_id")
    if not reason:
        raise InvalidTransitionError("cancel reason must not be empty")
    run = row_by_id(ctx, table="runs", id_column="run_id", value=run_id)
    if not run:
        raise NotFoundError(f"run {run_id!r} not found")
    job_row = row_by_id(ctx, table="jobs", id_column="job_id", value=job_id)
    if not job_row:
        raise NotFoundError(f"job {job_id!r} not found")
    if str(job_row["run_id"]) != run_id:
        raise InvalidTransitionError("job does not belong to the requested run")
    if job_row["state"] not in CANCELABLE_JOB_STATES:
        raise InvalidTransitionError(
            f"job state {job_row['state']!r} is terminal and cannot be canceled"
        )

    lock_run(ctx, run_id=run_id)
    expire_leases(ctx, run_id=run_id)
    job_row = row_by_id(ctx, table="jobs", id_column="job_id", value=job_id)
    if job_row["state"] not in CANCELABLE_JOB_STATES:
        raise InvalidTransitionError(
            f"job state {job_row['state']!r} is terminal and cannot be canceled"
        )

    dependents = _dependents_blocked_only_through(ctx, job_id=job_id)
    if dependents and not cascade:
        raise InvalidTransitionError(
            "job has blocked dependents whose only path is through this job; "
            "rerun with --cascade or cancel them explicitly: "
            + ", ".join(str(row["workflow_job_id"]) for row in dependents)
        )

    now = utc_now()
    canceled_summary = _cancel_single_job(
        ctx, job_row=job_row, reason=reason, now=now
    )
    downstream_canceled: list[dict[str, Any]] = []
    if cascade:
        queue: list[dict[str, Any]] = list(dependents)
        visited: set[str] = {job_id}
        while queue:
            next_queue: list[dict[str, Any]] = []
            for dep_row in queue:
                dep_id = str(dep_row["job_id"])
                if dep_id in visited:
                    continue
                visited.add(dep_id)
                fresh = row_by_id(ctx, table="jobs", id_column="job_id", value=dep_id)
                if not fresh or fresh["state"] not in CANCELABLE_JOB_STATES:
                    continue
                summary = _cancel_single_job(
                    ctx, job_row=fresh, reason=f"cascade:{reason}", now=now
                )
                downstream_canceled.append(summary)
                next_queue.extend(
                    _dependents_blocked_only_through(ctx, job_id=dep_id)
                )
            queue = next_queue

    run_after = maybe_complete_run(ctx, run_id=run_id)
    return {
        "status": "canceled",
        "run_id": run_id,
        "job_id": job_id,
        "workflow_job_id": canceled_summary["workflow_job_id"],
        "previous_state": canceled_summary["previous_state"],
        "reason": reason,
        "cascade": cascade,
        "downstream_canceled": downstream_canceled,
        "run_state": run_after.get("state"),
        "next_actions": ["inspect_run_state", "export_run_evidence"],
    }


__all__ = ["handle"]

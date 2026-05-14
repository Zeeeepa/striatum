"""PG handler for ``recovery.requeue_stale``.

Port of :func:`striatum.cli.recovery.requeue_stale` (lines 85-160).
Synthesis: docs/dogfood/057/DESIGN_SYNTHESIS.md L131-139.
"""

from __future__ import annotations

from typing import Any, Mapping

from striatum.errors import InvalidTransitionError, NotFoundError

from ._shim import RepoHandlerContext, register_pg_handler
from ._sql import (
    execute,
    expire_leases,
    fetch_all,
    is_repo_write_scope,
    row_by_id,
    utc_now,
)


@register_pg_handler("recovery.requeue_stale")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    """Requeue a stale non-repo-write job after lazy lease expiry.

    Repo-write jobs are refused — they require operator inspection because
    a partially written file tree can corrupt the next session's work
    (parity with ``striatum.cli.recovery.requeue_stale``).
    """
    run_id = str(params.get("run_id") or "")
    job_id = str(params.get("job_id") or "")
    if not run_id or not job_id:
        raise NotFoundError("recovery.requeue_stale requires run_id and job_id")
    recovery_author = params.get("recovery_author")
    recovery_author = str(recovery_author) if recovery_author else None

    run = row_by_id(ctx, table="runs", id_column="run_id", value=run_id)
    if not run:
        raise NotFoundError(f"run {run_id!r} not found")

    expire_leases(ctx, run_id=run_id)

    rows = fetch_all(
        ctx,
        sql=(
            "SELECT j.*, l.lease_id, l.owner_session_id, l.expires_at, "
            "       qm.message_id, qm.state AS message_state "
            "FROM striatumd.jobs j "
            "JOIN striatumd.leases l ON l.repository_id = j.repository_id "
            "   AND l.resource_id = j.job_id AND l.state = 'expired' "
            "LEFT JOIN striatumd.queue_messages qm "
            "   ON qm.repository_id = j.repository_id "
            "  AND qm.message_id = j.current_message_id "
            "WHERE j.repository_id = %(repository_id)s "
            "  AND j.run_id = %(run_id)s "
            "  AND j.job_id = %(job_id)s "
            "  AND j.state IN ('queued', 'blocked', 'stale_lease') "
            "ORDER BY l.expires_at DESC "
            "LIMIT 1 "
            "FOR UPDATE"
        ),
        params={"run_id": run_id, "job_id": job_id},
    )
    if not rows:
        raise InvalidTransitionError("job has no stale expired lease to requeue")
    row = rows[0]
    if is_repo_write_scope(row.get("write_scope_json")):
        raise InvalidTransitionError("repo-write stale jobs require manual inspection")

    now = utc_now()
    already_reclaimable = (
        row["state"] == "queued" and row.get("message_state") == "pending"
    )
    message_id = row.get("message_id")
    if message_id is None:
        message_id = ctx.new_id("msg")
        execute(
            ctx,
            sql=(
                "INSERT INTO striatumd.queue_messages "
                "(repository_id, message_id, run_id, job_id, kind, state, "
                " priority, created_at, updated_at) "
                "VALUES (%(repository_id)s, %(message_id)s, %(run_id)s, "
                "        %(job_id)s, 'work', 'pending', 0, %(now)s, %(now)s)"
            ),
            params={
                "message_id": message_id,
                "run_id": run_id,
                "job_id": job_id,
                "now": now,
            },
        )
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.jobs "
                "SET state = 'queued', current_lease_id = NULL, "
                "    current_message_id = %(message_id)s "
                "WHERE repository_id = %(repository_id)s "
                "  AND job_id = %(job_id)s"
            ),
            params={"message_id": message_id, "job_id": job_id},
        )
    else:
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.jobs "
                "SET state = 'queued', current_lease_id = NULL "
                "WHERE repository_id = %(repository_id)s "
                "  AND job_id = %(job_id)s"
            ),
            params={"job_id": job_id},
        )
        execute(
            ctx,
            sql=(
                "UPDATE striatumd.queue_messages "
                "SET state = 'pending', current_lease_id = NULL, "
                "    updated_at = %(now)s "
                "WHERE repository_id = %(repository_id)s "
                "  AND message_id = %(message_id)s"
            ),
            params={"now": now, "message_id": message_id},
        )

    payload: dict[str, Any] = {
        "already_reclaimable": already_reclaimable,
        "repo_write": False,
    }
    if recovery_author is not None:
        payload["author"] = recovery_author
    ctx.append_event(
        run_id=run_id,
        event_type="recovery.stale_requeued",
        job_id=job_id,
        message_id=str(message_id),
        lease_id=str(row["lease_id"]),
        payload=payload,
    )
    return {
        "status": "already_reclaimable" if already_reclaimable else "requeued",
        "run_id": run_id,
        "job_id": job_id,
        "workflow_job_id": row["workflow_job_id"],
        "lease_id": row["lease_id"],
        "message_id": message_id,
        "repo_write": False,
        "next_actions": ["register_or_select_session", "claim_available_work"],
    }


__all__ = ["handle"]

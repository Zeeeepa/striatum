"""PG handler for ``recovery.stale_leases``.

Port of :func:`striatum.cli.recovery.stale_leases` (lines 25-82).
Synthesis: docs/dogfood/057/DESIGN_SYNTHESIS.md L121-129.
"""

from __future__ import annotations

from typing import Any, Mapping

from striatum.errors import NotFoundError

from ._shim import RepoHandlerContext, register_pg_handler
from ._sql import expire_leases, fetch_all, is_repo_write_scope, row_by_id


@register_pg_handler("recovery.stale_leases")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    """List stale-lease jobs for ``run_id`` after running lazy expiry.

    The handler opens one serializable transaction at the router boundary
    (synthesis L22). Lazy expiry runs first and emits ``lease.expired`` /
    ``worktree.abandoned`` events for any newly expired rows; the listing
    that follows emits no event.
    """
    run_id = str(params.get("run_id") or "")
    if not run_id:
        raise NotFoundError("recovery.stale_leases requires run_id")
    run = row_by_id(ctx, table="runs", id_column="run_id", value=run_id)
    if not run:
        raise NotFoundError(f"run {run_id!r} not found")

    expire_leases(ctx, run_id=run_id)

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
            "LEFT JOIN striatumd.leases l ON l.repository_id = j.repository_id "
            "   AND (l.lease_id = j.current_lease_id "
            "        OR (l.resource_id = j.job_id AND l.state = 'expired')) "
            "LEFT JOIN striatumd.queue_messages qm "
            "   ON qm.repository_id = j.repository_id "
            "  AND qm.message_id = j.current_message_id "
            "WHERE j.repository_id = %(repository_id)s "
            "  AND j.run_id = %(run_id)s "
            "  AND (j.state = 'stale_lease' OR l.state = 'expired') "
            "ORDER BY j.workflow_job_id, l.expires_at"
        ),
        params={"run_id": run_id},
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
        entries.append(
            {
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
            }
        )

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


__all__ = ["handle"]

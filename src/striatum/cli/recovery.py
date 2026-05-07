"""Stale-lease recovery handlers for the Striatum CLI."""

from __future__ import annotations

import sqlite3

from striatum.db import (
    JsonObject,
    expire_leases,
    insert_event,
    row_by_id,
    transaction,
    utc_now,
)
from striatum.errors import InvalidTransitionError


def stale_leases(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Inspect stale lease recovery state for a run."""
    row_by_id(conn, "runs", "run_id", run_id)
    with transaction(conn):
        expire_leases(conn, run_id=run_id)
    from striatum.db import is_repo_write

    stale_jobs = conn.execute(
        """
        SELECT j.*, l.lease_id, l.owner_session_id, l.acquired_at, l.expires_at,
               l.released_at, l.release_reason, qm.message_id, qm.state AS message_state
        FROM jobs j
        LEFT JOIN leases l ON l.lease_id = j.current_lease_id
           OR (l.resource_id = j.job_id AND l.state = 'expired')
        LEFT JOIN queue_messages qm ON qm.message_id = j.current_message_id
        WHERE j.run_id = ? AND (j.state = 'stale_lease' OR l.state = 'expired')
        ORDER BY j.workflow_job_id, l.expires_at
        """,
        (run_id,),
    ).fetchall()
    entries: list[JsonObject] = []
    seen: set[tuple[str, str | None]] = set()
    for row in stale_jobs:
        key = (str(row["job_id"]), str(row["lease_id"]) if row["lease_id"] is not None else None)
        if key in seen:
            continue
        seen.add(key)
        repo_write = is_repo_write(row)
        entries.append(
            {
                "job_id": row["job_id"],
                "workflow_job_id": row["workflow_job_id"],
                "job_state": row["state"],
                "lease_id": row["lease_id"],
                "owner_session_id": row["owner_session_id"],
                "expires_at": row["expires_at"],
                "released_at": row["released_at"],
                "release_reason": row["release_reason"],
                "message_id": row["message_id"],
                "message_state": row["message_state"],
                "repo_write": repo_write,
                "recovery_policy": "manual_inspection_required" if repo_write else "safe_to_reclaim_when_pending",
                "next_actions": [
                    "inspect_worktree_and_artifacts",
                    "decide_requeue_or_cancel",
                ]
                if repo_write
                else ["register_or_select_session", "claim_available_work"],
            }
        )
    return {
        "run_id": run_id,
        "stale_count": len(entries),
        "stale_leases": entries,
        "next_actions": ["inspect_worktree_and_artifacts", "decide_requeue_or_cancel"]
        if entries
        else [],
    }


def requeue_stale(conn: sqlite3.Connection, *, run_id: str, job_id: str) -> JsonObject:
    """Requeue stale review-only work after lazy lease expiry."""
    row_by_id(conn, "runs", "run_id", run_id)
    from striatum.db import enqueue_job, is_repo_write

    with transaction(conn):
        expire_leases(conn, run_id=run_id)
        row = conn.execute(
            """
            SELECT j.*, l.lease_id, l.owner_session_id, l.expires_at,
                   qm.message_id, qm.state AS message_state
            FROM jobs j
            JOIN leases l ON l.resource_id = j.job_id AND l.state = 'expired'
            LEFT JOIN queue_messages qm ON qm.message_id = j.current_message_id
            WHERE j.run_id = ? AND j.job_id = ?
              AND j.state IN ('queued', 'blocked', 'stale_lease')
            ORDER BY l.expires_at DESC
            LIMIT 1
            """,
            (run_id, job_id),
        ).fetchone()
        if row is None:
            raise InvalidTransitionError("job has no stale expired lease to requeue")
        if is_repo_write(row):
            raise InvalidTransitionError("repo-write stale jobs require manual inspection")

        now = utc_now()
        already_reclaimable = row["state"] == "queued" and row["message_state"] == "pending"
        message_id = row["message_id"]
        if message_id is None:
            message_id = enqueue_job(conn, job_id=job_id)
        else:
            conn.execute(
                """
                UPDATE jobs
                SET state = 'queued', current_lease_id = NULL
                WHERE job_id = ?
                """,
                (job_id,),
            )
            conn.execute(
                """
                UPDATE queue_messages
                SET state = 'pending', current_lease_id = NULL, updated_at = ?
                WHERE message_id = ?
                """,
                (now, message_id),
            )
        insert_event(
            conn,
            run_id=run_id,
            event_type="recovery.stale_requeued",
            job_id=job_id,
            message_id=str(message_id),
            lease_id=str(row["lease_id"]),
            payload={"already_reclaimable": already_reclaimable, "repo_write": False},
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

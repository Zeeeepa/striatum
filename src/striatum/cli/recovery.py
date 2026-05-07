"""Stale-lease recovery handlers for the Striatum CLI."""

from __future__ import annotations

import sqlite3

from striatum.db import (
    JsonObject,
    expire_leases,
    insert_event,
    maybe_complete_run,
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


CANCELABLE_JOB_STATES = frozenset(
    {"blocked", "queued", "claimed", "running", "stale_lease", "waiting_human"}
)


def _dependents_blocked_only_through(
    conn: sqlite3.Connection, *, job_id: str
) -> list[sqlite3.Row]:
    """Return blocked jobs whose only unsatisfied upstream is ``job_id``.

    A job qualifies when it is in state ``blocked`` and depends on ``job_id``
    and every other upstream dependency it has is either already completed or
    canceled (i.e. cancelling ``job_id`` would orphan it). Jobs with another
    non-terminal upstream are ignored — those still have an alternate path.
    """
    candidates = conn.execute(
        """
        SELECT j.* FROM job_dependencies dep
        JOIN jobs j ON j.job_id = dep.job_id
        WHERE dep.depends_on_job_id = ? AND j.state = 'blocked'
        ORDER BY j.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    qualifying: list[sqlite3.Row] = []
    for candidate in candidates:
        other_deps = conn.execute(
            """
            SELECT up.state FROM job_dependencies dep
            JOIN jobs up ON up.job_id = dep.depends_on_job_id
            WHERE dep.job_id = ? AND dep.depends_on_job_id != ?
            """,
            (candidate["job_id"], job_id),
        ).fetchall()
        only_through = all(
            row["state"] in {"completed", "canceled"} for row in other_deps
        )
        if only_through:
            qualifying.append(candidate)
    return qualifying


def _cancel_single_job(
    conn: sqlite3.Connection,
    *,
    job_row: sqlite3.Row,
    reason: str,
    now: str,
) -> JsonObject:
    """Cancel one job: release leases, clear messages, mark canceled, emit event."""
    job_id = str(job_row["job_id"])
    run_id = str(job_row["run_id"])
    lease_id = job_row["current_lease_id"]
    message_id = job_row["current_message_id"]
    if lease_id is not None:
        conn.execute(
            """
            UPDATE leases
            SET state = 'released', released_at = ?, release_reason = 'canceled'
            WHERE lease_id = ? AND state = 'active'
            """,
            (now, lease_id),
        )
    # Mark any expired leases for this job released too, so doctor stays clean.
    conn.execute(
        """
        UPDATE leases
        SET release_reason = COALESCE(release_reason, 'canceled')
        WHERE resource_id = ? AND state IN ('expired')
        """,
        (job_id,),
    )
    if message_id is not None:
        conn.execute(
            """
            UPDATE queue_messages
            SET state = 'canceled', current_lease_id = NULL, updated_at = ?
            WHERE message_id = ?
            """,
            (now, message_id),
        )
    conn.execute(
        """
        UPDATE jobs
        SET state = 'canceled', current_lease_id = NULL,
            current_message_id = NULL, completed_at = ?
        WHERE job_id = ?
        """,
        (now, job_id),
    )
    insert_event(
        conn,
        run_id=run_id,
        event_type="job.canceled",
        job_id=job_id,
        message_id=str(message_id) if message_id is not None else None,
        lease_id=str(lease_id) if lease_id is not None else None,
        payload={"reason": reason, "workflow_job_id": job_row["workflow_job_id"]},
    )
    return {
        "job_id": job_id,
        "workflow_job_id": job_row["workflow_job_id"],
        "previous_state": job_row["state"],
    }


def cancel_job(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    job_id: str,
    reason: str,
    cascade: bool,
) -> JsonObject:
    """Cancel a non-terminal job and (optionally) its orphaned blocked dependents.

    Refuses to cancel terminal-state jobs (``completed``, ``failed``,
    ``canceled``, ``skipped``). When the job has blocked dependents whose only
    upstream path was through this job, the call refuses with exit code 4
    unless ``cascade`` is set; with ``cascade=True``, all such dependents are
    canceled in the same transaction (recursively).
    """
    if reason.strip() == "":
        raise InvalidTransitionError("cancel reason must not be empty")
    row_by_id(conn, "runs", "run_id", run_id)
    job_row = row_by_id(conn, "jobs", "job_id", job_id)
    if str(job_row["run_id"]) != run_id:
        raise InvalidTransitionError("job does not belong to the requested run")
    if job_row["state"] not in CANCELABLE_JOB_STATES:
        raise InvalidTransitionError(
            f"job state {job_row['state']!r} is terminal and cannot be canceled"
        )

    with transaction(conn):
        expire_leases(conn, run_id=run_id)
        # Re-read after expire_leases since job state may have shifted.
        job_row = row_by_id(conn, "jobs", "job_id", job_id)
        if job_row["state"] not in CANCELABLE_JOB_STATES:
            raise InvalidTransitionError(
                f"job state {job_row['state']!r} is terminal and cannot be canceled"
            )

        dependents = _dependents_blocked_only_through(conn, job_id=job_id)
        if dependents and not cascade:
            raise InvalidTransitionError(
                "job has blocked dependents whose only path is through this job; "
                "rerun with --cascade or cancel them explicitly: "
                + ", ".join(str(row["workflow_job_id"]) for row in dependents)
            )

        now = utc_now()
        canceled_summary = _cancel_single_job(
            conn, job_row=job_row, reason=reason, now=now
        )
        downstream_canceled: list[JsonObject] = []
        if cascade:
            # Iteratively cancel dependents: cancelling one may orphan
            # transitive descendants whose only remaining upstream is the just
            # canceled job. Continue until the dependents set is empty.
            queue: list[sqlite3.Row] = list(dependents)
            visited: set[str] = {job_id}
            while queue:
                next_queue: list[sqlite3.Row] = []
                for dep_row in queue:
                    dep_id = str(dep_row["job_id"])
                    if dep_id in visited:
                        continue
                    visited.add(dep_id)
                    fresh = row_by_id(conn, "jobs", "job_id", dep_id)
                    if fresh["state"] not in CANCELABLE_JOB_STATES:
                        continue
                    summary = _cancel_single_job(
                        conn, job_row=fresh, reason=f"cascade:{reason}", now=now
                    )
                    downstream_canceled.append(summary)
                    next_queue.extend(
                        _dependents_blocked_only_through(conn, job_id=dep_id)
                    )
                queue = next_queue

        maybe_complete_run(conn, run_id=run_id)
        run_row = row_by_id(conn, "runs", "run_id", run_id)
        return {
            "status": "canceled",
            "run_id": run_id,
            "job_id": job_id,
            "workflow_job_id": canceled_summary["workflow_job_id"],
            "previous_state": canceled_summary["previous_state"],
            "reason": reason,
            "cascade": cascade,
            "downstream_canceled": downstream_canceled,
            "run_state": run_row["state"],
            "next_actions": ["inspect_run_state", "export_run_evidence"],
        }

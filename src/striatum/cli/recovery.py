"""Stale-lease recovery handlers for the Striatum CLI."""

from __future__ import annotations

import json
import os
import sqlite3
from datetime import UTC, datetime, timedelta
from pathlib import Path

from striatum.db import (
    JsonObject,
    complete_job,
    expire_leases,
    insert_event,
    json_loads,
    maybe_complete_run,
    row_by_id,
    transaction,
    utc_now,
)
from striatum.errors import InvalidTransitionError
from striatum.process_completion import validate_outputs


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


def requeue_stale(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    job_id: str,
    recovery_author: str | None = None,
) -> JsonObject:
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
        payload: JsonObject = {"already_reclaimable": already_reclaimable, "repo_write": False}
        if recovery_author is not None:
            payload["author"] = recovery_author
        insert_event(
            conn,
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


CANCELABLE_JOB_STATES = frozenset(
    {"blocked", "queued", "claimed", "running", "stale_lease", "waiting_human"}
)

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


def resume_blocker(
    conn: sqlite3.Connection,
    *,
    blocker_id: str,
    complete: bool,
    session_id: str | None,
    summary: str | None,
    force: bool,
    extend_seconds: int,
) -> JsonObject:
    """Resolve a process-adapter blocker after operator remediation.

    Plain ``resume`` returns the job to ``running`` using the existing
    claimed lease. For review blockers, that is the missing bridge that lets
    the operator record the normal ``accept_with_findings`` verdict. With
    ``--complete``, the function also completes the job after revalidation.
    """
    if complete and not session_id:
        raise InvalidTransitionError("--complete requires --session-id")
    if extend_seconds <= 0:
        raise InvalidTransitionError("--extend-seconds must be positive")

    complete_args: JsonObject | None = None
    with transaction(conn):
        blocker = row_by_id(conn, "blockers", "blocker_id", blocker_id)
        blocker_kind = str(blocker["blocker_kind"])
        if blocker_kind not in PROCESS_ADAPTER_BLOCKER_KINDS:
            raise InvalidTransitionError(
                "recovery resume supports only process-adapter blockers"
            )
        job_id = str(blocker["job_id"]) if blocker["job_id"] is not None else None
        if job_id is None:
            raise InvalidTransitionError("process-adapter blocker is not job-bound")
        job = row_by_id(conn, "jobs", "job_id", job_id)
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
        # GH #7: when the job has already reached a terminal state and a
        # process-adapter blocker is still open (legacy state from
        # before the v1.41 post-completion guard), close the blocker as
        # a no-op without changing the job's terminal state. The
        # blocker is a stale trailing signal — the workflow completed
        # successfully.
        terminal_states = {"completed", "failed", "canceled", "skipped"}
        if str(job["state"]) in terminal_states and force:
            conn.execute(
                "UPDATE blockers SET state = 'resolved', resolved_at = ? "
                "WHERE blocker_id = ?",
                (utc_now(), blocker_id),
            )
            insert_event(
                conn,
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

        missing_paths, review_verdict_missing = validate_outputs(conn, job=job)
        if missing_paths:
            raise InvalidTransitionError(
                "process-adapter blocker still has missing required artifacts: "
                + ", ".join(missing_paths)
            )
        if complete and review_verdict_missing:
            raise InvalidTransitionError(
                "process-adapter blocker cannot complete while review verdict is missing"
            )
        if (
            review_verdict_missing
            and str(job["job_type"]) != "review"
        ):
            raise InvalidTransitionError(
                "process-adapter blocker still has a missing review verdict"
            )

        lease_id = job["current_lease_id"]
        if lease_id is None:
            raise InvalidTransitionError(
                "process-adapter blocker job has no current lease to resume"
            )
        lease = row_by_id(conn, "leases", "lease_id", str(lease_id))
        if lease["state"] != "active":
            raise InvalidTransitionError(
                f"process-adapter blocker lease is not active (state={lease['state']!r})"
            )
        lease_owner = str(lease["owner_session_id"])
        if session_id is not None and session_id != lease_owner:
            raise InvalidTransitionError("session does not own the process-adapter lease")

        now = utc_now()
        expires_at = (
            datetime.now(UTC) + timedelta(seconds=extend_seconds)
        ).replace(microsecond=0).isoformat().replace("+00:00", "Z")
        conn.execute(
            """
            UPDATE leases
            SET last_heartbeat_at = ?, expires_at = ?
            WHERE lease_id = ?
            """,
            (now, expires_at, str(lease_id)),
        )
        conn.execute(
            "UPDATE blockers SET state = 'resolved', resolved_at = ? WHERE blocker_id = ?",
            (now, blocker_id),
        )
        conn.execute(
            "UPDATE jobs SET state = 'running' WHERE job_id = ?",
            (job_id,),
        )

        actor_session_id = session_id or str(blocker["session_id"] or lease_owner)
        payload = _blocker_payload(blocker)
        event_payload: JsonObject = {
            "blocker_id": blocker_id,
            "blocker_kind": blocker_kind,
            "verb": "recovery resume",
            "force": force,
            "completed_inline": complete,
            "missing_artifact_paths": missing_paths,
            "review_verdict_missing": review_verdict_missing,
            "lease_extended_until": expires_at,
        }
        if payload:
            event_payload["original_envelope"] = payload
        insert_event(
            conn,
            run_id=run_id,
            event_type="recovery.process_blocker_resolved",
            actor_session_id=actor_session_id,
            job_id=job_id,
            lease_id=str(lease_id),
            payload=event_payload,
        )

        result: JsonObject = {
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
        if complete:
            complete_args = {
                "session_id": str(session_id),
                "job_id": job_id,
                "lease_id": str(lease_id),
                "summary": summary,
                "resume_result": result,
            }
        else:
            return result

    assert complete_args is not None
    completion = complete_job(
        conn,
        session_id=str(complete_args["session_id"]),
        job_id=str(complete_args["job_id"]),
        lease_id=str(complete_args["lease_id"]),
        summary=summary,
    )
    resume_result = complete_args["resume_result"]
    assert isinstance(resume_result, dict)
    resume_result["status"] = "resumed_completed"
    resume_result["completed_inline"] = True
    resume_result["completion"] = completion
    resume_result["next_actions"] = ["monitor_run_progress", "export_run_evidence"]
    return resume_result


def _blocker_payload(blocker: sqlite3.Row) -> JsonObject:
    raw = blocker["payload_json"]
    if raw is None:
        return {}
    try:
        loaded = json.loads(str(raw))
    except json.JSONDecodeError:
        return {}
    return loaded if isinstance(loaded, dict) else {}


def process_reconcile(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """RFC 0014 V1: walk ``process_executions.state = 'running'`` rows and
    transition externally-killed processes to ``'lost'``.

    Mirrors the lazy-on-CLI shape of :func:`requeue_stale` (D036). For
    each row whose pid is gone (``os.kill(pid, 0)`` raises
    ``ProcessLookupError``), transitions the row to ``'lost'`` and runs
    the same output validation step the inline path uses, blocking the
    job with ``process_lost_with_outputs_missing`` when required outputs
    are absent.

    Returns a JSON envelope with per-row detail and ``next_actions``.
    """
    row_by_id(conn, "runs", "run_id", run_id)
    from striatum.process_adapter import mark_process_lost
    from striatum.process_completion import evaluate_and_block_after_reconcile

    still_running: list[JsonObject] = []
    transitioned_to_lost: list[JsonObject] = []
    rows = conn.execute(
        """
        SELECT pe.*
        FROM process_executions pe
        WHERE pe.run_id = ? AND pe.state = 'running'
        ORDER BY pe.started_at
        """,
        (run_id,),
    ).fetchall()
    for row in rows:
        pid = row["pid"]
        if pid is None:
            # state='running' without pid is degenerate; treat as lost.
            alive = False
        else:
            alive = _pid_alive(int(pid))
        if alive:
            still_running.append(
                {
                    "process_id": str(row["process_id"]),
                    "job_id": str(row["job_id"]),
                    "pid": int(pid) if pid is not None else None,
                    "started_at": str(row["started_at"]),
                }
            )
            continue
        mark_process_lost(conn, process_id=str(row["process_id"]))
        with transaction(conn):
            process = row_by_id(conn, "process_executions", "process_id", str(row["process_id"]))
            job = row_by_id(conn, "jobs", "job_id", str(process["job_id"]))
            try:
                command = json.loads(str(process["command_json"]))
            except json.JSONDecodeError:
                command = []
            if not isinstance(command, list):
                command = []
            started = str(process["started_at"])
            ended = str(process["ended_at"] or utc_now())
            duration = _duration_seconds(started, ended)
            blocker_kind, _envelope = evaluate_and_block_after_reconcile(
                conn,
                job=job,
                session_id=str(process["session_id"]),
                process_id=str(process["process_id"]),
                command=command,
                duration_seconds=duration,
            )
        transitioned_to_lost.append(
            {
                "process_id": str(row["process_id"]),
                "job_id": str(row["job_id"]),
                "pid": int(pid) if pid is not None else None,
                "blocker_kind": blocker_kind,
            }
        )
    next_actions: list[str] = []
    if transitioned_to_lost:
        next_actions.append("inspect_blockers")
        next_actions.append("decide_recovery_path")
    return {
        "run_id": run_id,
        "still_running": still_running,
        "transitioned_to_lost": transitioned_to_lost,
        "next_actions": next_actions,
    }


def _pid_alive(pid: int) -> bool:
    """Return True when ``pid`` is reachable from this process.

    ``os.kill(pid, 0)`` raises:
      - ProcessLookupError (ESRCH) → process is gone.
      - PermissionError (EPERM)    → process exists but not ours.
      - returns None on success    → process is alive and reachable.

    The reconciler treats EPERM as "still running" (we can't act on it
    anyway) so an operator-killed-then-replaced PID does not get
    spuriously reconciled.
    """
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    return True


def _duration_seconds(started_at: str, ended_at: str) -> float:
    from datetime import datetime

    def parse(value: str) -> datetime:
        cleaned = value.replace("Z", "+00:00")
        return datetime.fromisoformat(cleaned)

    return max(0.0, (parse(ended_at) - parse(started_at)).total_seconds())


def _declared_get(declared: object, key: str, default: object = None) -> object:
    """V1.41 helper: tolerant get from json_loads'd expected_artifacts."""
    if isinstance(declared, dict):
        return declared.get(key, default)
    return default


def auto_publish_stale_artifacts(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    run_id: str,
    dry_run: bool = False,
) -> JsonObject:
    """V1.41 (harness friction burn-down): drive down the recurring
    claude-no-explicit-publish anti-pattern (6+ instances).

    Walk stale leases in *run_id*. For each, check whether the session
    wrote the work-packet's `expected_artifacts[].path` file conformantly
    to disk. If so — and only if the on-disk file's byline matches the
    expected_author_line exactly AND the path matches the declared
    expected_artifacts path exactly — auto-publish the artifact and
    complete the job on the dead session's behalf.

    Two-condition gate prevents misfiring: if either the byline or the
    path doesn't match, fall through to the existing stale-lease blocker
    so the operator still sees the gap.
    """
    from striatum.artifacts import (
        expected_author_line,
        markdown_title_block_author_lines,
        publish_artifact,
        _canonical_byline_form,
    )

    row_by_id(conn, "runs", "run_id", run_id)
    with transaction(conn):
        expire_leases(conn, run_id=run_id)

    # Find jobs with expired leases that still hold expected_artifacts.
    candidates = conn.execute(
        """
        SELECT j.*, l.lease_id, l.owner_session_id, l.expires_at,
               qm.message_id, qm.state AS message_state
          FROM jobs j
          LEFT JOIN leases l ON l.lease_id = j.current_lease_id
             OR (l.resource_id = j.job_id AND l.state = 'expired')
          LEFT JOIN queue_messages qm ON qm.message_id = j.current_message_id
         WHERE j.run_id = ?
           AND j.state IN ('claimed', 'running', 'stale_lease')
           AND l.state = 'expired'
         ORDER BY j.workflow_job_id
        """,
        (run_id,),
    ).fetchall()

    published: list[JsonObject] = []
    skipped: list[JsonObject] = []
    seen: set[tuple[str, str]] = set()
    for row in candidates:
        key = (str(row["job_id"]), str(row["lease_id"]) if row["lease_id"] is not None else "")
        if key in seen:
            continue
        seen.add(key)
        job_id = str(row["job_id"])
        workflow_job_id = str(row["workflow_job_id"])
        session_id = str(row["owner_session_id"]) if row["owner_session_id"] is not None else None
        lease_id = str(row["lease_id"]) if row["lease_id"] is not None else None
        message_id = str(row["message_id"]) if row["message_id"] is not None else None
        expected_artifacts_raw = json_loads(str(row["expected_artifacts_json"] or "[]"))
        expected_artifacts: list[object] = (
            list(expected_artifacts_raw)
            if isinstance(expected_artifacts_raw, list)
            else []
        )
        if not expected_artifacts:
            skipped.append({
                "workflow_job_id": workflow_job_id,
                "reason": "no expected_artifacts declared",
            })
            continue
        if session_id is None or lease_id is None or message_id is None:
            skipped.append({
                "workflow_job_id": workflow_job_id,
                "reason": "no recoverable session/lease/message triple",
            })
            continue
        # Resolve the expected_author_line for this (job, session).
        try:
            expected_byline = expected_author_line(conn, job=row, session_id=session_id)
        except Exception as exc:  # noqa: BLE001
            skipped.append({
                "workflow_job_id": workflow_job_id,
                "reason": f"could not derive expected byline: {exc}",
            })
            continue
        publishable: list[dict[str, object]] = []
        for declared_obj in expected_artifacts:
            if not isinstance(declared_obj, dict):
                continue
            declared = declared_obj
            if not _declared_get(declared, "required", True):
                continue
            path_text = _declared_get(declared, "path")
            if not isinstance(path_text, str) or not path_text:
                continue
            disk_path = repo / path_text
            if not disk_path.is_file():
                continue
            try:
                payload = disk_path.read_bytes().decode("utf-8")
            except (OSError, UnicodeDecodeError):
                continue
            byline_lines = markdown_title_block_author_lines(payload)
            canonical_match = False
            for line in byline_lines:
                if _canonical_byline_form(line) == expected_byline:
                    canonical_match = True
                    break
            if not canonical_match:
                continue
            publishable.append(declared)
        if not publishable:
            skipped.append({
                "workflow_job_id": workflow_job_id,
                "reason": "no required expected_artifact found on disk with matching byline",
            })
            continue
        if dry_run:
            published.append({
                "workflow_job_id": workflow_job_id,
                "session_id": session_id,
                "would_publish": [
                    {
                        "path": _declared_get(d, "path"),
                        "kind": _declared_get(d, "kind"),
                        "logical_name": _declared_get(d, "logical_name"),
                    }
                    for d in publishable
                ],
            })
            continue
        # Auto-publish path: ack the existing message + re-activate the
        # expired lease for the duration of this transaction. The
        # publish_artifact + complete_job flow then writes through normally.
        from striatum.cli.mutations import ack_work
        try:
            ack_work(
                conn,
                session_id=session_id,
                message_id=message_id,
                lease_id=lease_id,
            )
        except Exception:  # noqa: BLE001
            # Ack may legitimately fail if the message has already been
            # acked once but the implementer never completed. Continue;
            # publish_artifact will succeed against the active lease row.
            pass
        artifact_results: list[object] = []
        for declared in publishable:
            try:
                result = publish_artifact(
                    conn,
                    repo=repo,
                    session_id=session_id,
                    job_id=job_id,
                    lease_id=lease_id,
                    kind=str(_declared_get(declared, "kind")),
                    logical_name=str(_declared_get(declared, "logical_name")),
                    path_text=str(_declared_get(declared, "path")),
                )
                artifact_results.append(result)
            except Exception as exc:  # noqa: BLE001
                artifact_results.append({"error": str(exc), "path": _declared_get(declared, "path")})
        try:
            complete_result = complete_job(
                conn,
                session_id=session_id,
                job_id=job_id,
                lease_id=lease_id,
                summary="auto-published on stale lease (V1.41 harness friction burn-down)",
            )
        except Exception as exc:  # noqa: BLE001
            complete_result = {"error": str(exc)}
        with transaction(conn):
            insert_event(
                conn,
                run_id=run_id,
                event_type="recovery.auto_published",
                actor_session_id=session_id,
                job_id=job_id,
                lease_id=lease_id,
                payload={
                    "workflow_job_id": workflow_job_id,
                    "artifacts": [a.get("path") if isinstance(a, dict) else None for a in publishable],
                    "byline": expected_byline,
                },
            )
        published.append({
            "workflow_job_id": workflow_job_id,
            "session_id": session_id,
            "artifacts": artifact_results,
            "complete": complete_result,
        })
    # Try to advance run state if all jobs are terminal now.
    with transaction(conn):
        maybe_complete_run(conn, run_id=run_id)
    return {
        "run_id": run_id,
        "dry_run": bool(dry_run),
        "published_count": len(published),
        "published": published,
        "skipped_count": len(skipped),
        "skipped": skipped,
    }

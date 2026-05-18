"""Per-job worktree handlers for the Striatum CLI."""

from __future__ import annotations

import sqlite3
import subprocess
from pathlib import Path

from striatum.legacy_sqlite.db import (
    active_lease_for,
    active_worktree_for_job,
    insert_event,
    is_repo_write,
    job_lane_id,
    row_by_id,
    transaction,
    workflow_for_run,
)
from striatum.errors import InvalidTransitionError, NotFoundError
from striatum.primitives import JsonObject, new_id, utc_now
from striatum.repo_policy import STATE_DIR, WORKTREES_SUBDIR, lane_worktree_isolation


def worktree_create(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    job_id: str,
    lease_id: str,
) -> JsonObject:
    """Create a per-job git worktree for a claimed repo-write job.

    The lane must declare ``worktree_isolation: per_job``, the job must be
    repo-write, and there must be no other active worktree for the job. The
    worktree is created at ``.striatum/worktrees/<worktree_id>`` based on the
    run's confirmed branch. The directory itself is owned by git; the row is
    the source of truth for state.
    """
    job = row_by_id(conn, "jobs", "job_id", job_id)
    active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
    if not is_repo_write(job):
        raise InvalidTransitionError("worktree create requires a repo-write job")
    if active_worktree_for_job(conn, job_id=job_id) is not None:
        raise InvalidTransitionError("job already has an active worktree")

    workflow = workflow_for_run(conn, run_id=str(job["run_id"]))
    lane_id = job_lane_id(job)
    isolation = lane_worktree_isolation(workflow, lane_id)
    if isolation != "per_job":
        raise InvalidTransitionError(
            "lane is not configured for worktree_isolation: per_job"
        )

    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    base_branch = run["branch_name"]
    if base_branch is None or str(base_branch) == "":
        raise InvalidTransitionError("run has no confirmed branch for worktree base")
    if run["branch_confirmed_at"] is None:
        raise InvalidTransitionError("run branch must be confirmed before worktree create")

    worktree_id = new_id("wt")
    relative = f"{STATE_DIR}/{WORKTREES_SUBDIR}/{worktree_id}"
    target = repo / STATE_DIR / WORKTREES_SUBDIR / worktree_id
    target.parent.mkdir(parents=True, exist_ok=True)

    # Use --detach so the worktree starts at the base branch's tip without
    # conflicting with the main worktree's checkout of the same branch.
    # Operators recover work from the worktree directly; V1 does not commit.
    result = subprocess.run(
        ["git", "worktree", "add", "--detach", str(target), str(base_branch)],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        stderr = (result.stderr or result.stdout or "").strip()
        if len(stderr) > 200:
            stderr = stderr[:200] + "..."
        raise InvalidTransitionError(
            f"git worktree add failed: {stderr}" if stderr
            else "git worktree add failed"
        )

    with transaction(conn):
        now = utc_now()
        conn.execute(
            """
            INSERT INTO job_worktrees (
              worktree_id, run_id, job_id, lease_id, base_branch,
              worktree_path, state, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, 'active', ?)
            """,
            (
                worktree_id,
                str(job["run_id"]),
                job_id,
                lease_id,
                str(base_branch),
                relative,
                now,
            ),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="worktree.created",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={
                "worktree_id": worktree_id,
                "worktree_path": relative,
                "base_branch": str(base_branch),
            },
        )
    return {
        "worktree_id": worktree_id,
        "worktree_path": relative,
        "base_branch": str(base_branch),
    }


def worktree_release(
    conn: sqlite3.Connection, *, repo: Path, worktree_id: str
) -> JsonObject:
    """Remove a per-job git worktree directory and mark the row removed.

    Idempotent: releasing a worktree that is already in a terminal state
    (``released``, ``removed``, ``abandoned``) returns success without
    rerunning ``git worktree remove``.
    """
    row = conn.execute(
        "SELECT * FROM job_worktrees WHERE worktree_id = ?",
        (worktree_id,),
    ).fetchone()
    if row is None:
        raise NotFoundError(f"could not find job_worktrees row for {worktree_id!r}")
    if row["state"] != "active":
        return {
            "status": "already_released",
            "worktree_id": worktree_id,
            "state": str(row["state"]),
        }
    target = repo / str(row["worktree_path"])
    result = subprocess.run(
        ["git", "worktree", "remove", "--force", str(target)],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0 and target.exists():
        stderr = (result.stderr or result.stdout or "").strip()
        if len(stderr) > 200:
            stderr = stderr[:200] + "..."
        raise InvalidTransitionError(
            f"git worktree remove failed: {stderr}" if stderr
            else "git worktree remove failed"
        )

    with transaction(conn):
        now = utc_now()
        conn.execute(
            """
            UPDATE job_worktrees
            SET state = 'removed', released_at = ?, removed_at = ?
            WHERE worktree_id = ?
            """,
            (now, now, worktree_id),
        )
        insert_event(
            conn,
            run_id=str(row["run_id"]),
            event_type="worktree.released",
            job_id=str(row["job_id"]),
            lease_id=str(row["lease_id"]),
            payload={
                "worktree_id": worktree_id,
                "worktree_path": str(row["worktree_path"]),
            },
        )
    return {
        "status": "released",
        "worktree_id": worktree_id,
        "state": "removed",
    }


def worktree_list(conn: sqlite3.Connection, *, run_id: str | None) -> JsonObject:
    """Return per-job worktree rows with their workflow_job_id."""
    rows = conn.execute(
        """
        SELECT w.*, j.workflow_job_id
        FROM job_worktrees w
        JOIN jobs j ON j.job_id = w.job_id
        WHERE (? IS NULL OR w.run_id = ?)
        ORDER BY w.created_at, w.worktree_id
        """,
        (run_id, run_id),
    ).fetchall()
    return {"worktrees": [dict(row) for row in rows]}

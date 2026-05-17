"""PG-backed per-job worktree handlers."""

from __future__ import annotations

import subprocess
from collections.abc import Mapping
from pathlib import Path
from typing import Any

from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _json_loads,
    _ts,
    active_lease_for,
    is_repo_write,
    transaction,
    workflow_for_run,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import InvalidTransitionError, LeaseError, NotFoundError
from striatum.primitives import JsonObject
from striatum.repo_policy import STATE_DIR, WORKTREES_SUBDIR, lane_worktree_isolation


@register_pg_handler("worktree.create")
def handle_create(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    return worktree_create(
        ctx,
        session_id=_required_text(params, "session_id"),
        job_id=_required_text(params, "job_id"),
        lease_id=_required_text(params, "lease_id"),
    )


@register_pg_handler("worktree.release")
def handle_release(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    return worktree_release(ctx, worktree_id=_required_text(params, "worktree_id"))


@register_pg_handler("worktree.list")
def handle_list(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    return worktree_list(ctx, run_id=_optional_text(params, "run_id"))


setattr(handle_list, "read_only", True)


def worktree_create(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
) -> JsonObject:
    """Create a git worktree for an active repo-write job lease."""

    with transaction(ctx):
        _, base_branch = _validated_create_inputs(
            ctx,
            session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            for_update=True,
        )
    worktree_id = ctx.new_id("wt")
    relative = f"{STATE_DIR}/{WORKTREES_SUBDIR}/{worktree_id}"
    target = _worktree_target(ctx, relative)
    target.parent.mkdir(parents=True, exist_ok=True)

    result = subprocess.run(
        ["git", "worktree", "add", "--detach", str(target), base_branch],
        cwd=ctx.repo_root,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise InvalidTransitionError(_git_error_message("git worktree add failed", result))

    with transaction(ctx):
        job, base_branch = _validated_create_inputs(
            ctx,
            session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            for_update=True,
        )
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.job_worktrees (
                  repository_id, worktree_id, run_id, job_id, lease_id,
                  base_branch, worktree_path, state, created_at
                )
                VALUES (%s, %s, %s, %s, %s, %s, %s, 'active', %s)
                """,
                (
                    ctx.repository_id,
                    worktree_id,
                    job["run_id"],
                    job_id,
                    lease_id,
                    base_branch,
                    relative,
                    now,
                ),
            )
        ctx.append_event(
            run_id=str(job["run_id"]),
            event_type="worktree.created",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={
                "worktree_id": worktree_id,
                "worktree_path": relative,
                "base_branch": base_branch,
            },
        )
    return {
        "worktree_id": worktree_id,
        "worktree_path": relative,
        "base_branch": base_branch,
    }


def worktree_release(ctx: RepoHandlerContext, *, worktree_id: str) -> JsonObject:
    """Remove an active git worktree and mark its row removed.

    Releasing a row that is already terminal remains idempotent.
    """

    with transaction(ctx):
        row = _worktree_row(ctx, worktree_id)
    if row["state"] != "active":
        return {
            "status": "already_released",
            "worktree_id": worktree_id,
            "state": str(row["state"]),
        }

    target = _worktree_target(ctx, str(row["worktree_path"]))
    result = subprocess.run(
        ["git", "worktree", "remove", "--force", str(target)],
        cwd=ctx.repo_root,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0 and target.exists():
        raise InvalidTransitionError(_git_error_message("git worktree remove failed", result))

    with transaction(ctx):
        row = _worktree_row(ctx, worktree_id, for_update=True)
        if row["state"] != "active":
            return {
                "status": "already_released",
                "worktree_id": worktree_id,
                "state": str(row["state"]),
            }
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.job_worktrees
                SET state = 'removed', released_at = %s, removed_at = %s
                WHERE repository_id = %s AND worktree_id = %s
                """,
                (now, now, ctx.repository_id, worktree_id),
            )
        ctx.append_event(
            run_id=str(row["run_id"]),
            event_type="worktree.released",
            job_id=str(row["job_id"]),
            lease_id=str(row["lease_id"]),
            payload={
                "worktree_id": worktree_id,
                "worktree_path": str(row["worktree_path"]),
            },
        )
    return {"status": "released", "worktree_id": worktree_id, "state": "removed"}


def worktree_list(ctx: RepoHandlerContext, *, run_id: str | None) -> JsonObject:
    """Return repo-scoped per-job worktree rows."""

    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT w.worktree_id, w.run_id, w.job_id, w.lease_id,
                   w.base_branch, w.worktree_path, w.state, w.created_at,
                   w.released_at, w.removed_at, j.workflow_job_id
            FROM striatumd.job_worktrees w
            JOIN striatumd.jobs j
              ON j.repository_id = w.repository_id AND j.job_id = w.job_id
            WHERE w.repository_id = %s
              AND (%s::text IS NULL OR w.run_id = %s)
            ORDER BY w.created_at, w.worktree_id
            """,
            (ctx.repository_id, run_id, run_id),
        )
        rows = [dict(row) for row in cur.fetchall()]
    return {"worktrees": [_row_to_json(row) for row in rows]}


def _validated_create_inputs(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    for_update: bool,
) -> tuple[Mapping[str, Any], str]:
    job = ctx.row_by_id("jobs", "job_id", job_id, for_update=for_update)
    lease = active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=job_id)
    session = ctx.row_by_id("sessions", "session_id", session_id, for_update=for_update)
    if session["state"] != "active":
        raise LeaseError("lease owner session is not active")
    if lease["resource_type"] != "job":
        raise LeaseError("lease does not belong to a job")
    if str(lease["run_id"]) != str(job["run_id"]):
        raise LeaseError("lease does not belong to the job run")
    if not is_repo_write(job):
        raise InvalidTransitionError("worktree create requires a repo-write job")
    if _active_worktree_for_job(ctx, job_id=job_id, for_update=for_update) is not None:
        raise InvalidTransitionError("job already has an active worktree")

    workflow = workflow_for_run(ctx, run_id=str(job["run_id"]))
    lane_id = _job_lane_id(job)
    isolation = lane_worktree_isolation(workflow, lane_id)
    if isolation != "per_job":
        raise InvalidTransitionError(
            "lane is not configured for worktree_isolation: per_job"
        )

    run = ctx.row_by_id("runs", "run_id", str(job["run_id"]), for_update=for_update)
    base_branch = str(run["branch_name"] or "")
    if base_branch == "":
        raise InvalidTransitionError("run has no confirmed branch for worktree base")
    if run["branch_confirmed_at"] is None:
        raise InvalidTransitionError("run branch must be confirmed before worktree create")
    return job, base_branch


def _worktree_row(
    ctx: RepoHandlerContext, worktree_id: str, *, for_update: bool = False
) -> dict[str, Any]:
    suffix = " FOR UPDATE" if for_update else ""
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT *
            FROM striatumd.job_worktrees
            WHERE repository_id = %s AND worktree_id = %s
            {suffix}
            """,
            (ctx.repository_id, worktree_id),
        )
        row = cur.fetchone()
    if row is None:
        raise NotFoundError(f"could not find job_worktrees row for {worktree_id!r}")
    return dict(row)


def _active_worktree_for_job(
    ctx: RepoHandlerContext, *, job_id: str, for_update: bool
) -> dict[str, Any] | None:
    suffix = " FOR UPDATE" if for_update else ""
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT *
            FROM striatumd.job_worktrees
            WHERE repository_id = %s AND job_id = %s AND state = 'active'
            LIMIT 1
            {suffix}
            """,
            (ctx.repository_id, job_id),
        )
        row = cur.fetchone()
    return dict(row) if row is not None else None


def _job_lane_id(job: Mapping[str, Any]) -> str | None:
    selector = _json_loads(job["lane_selector_json"])
    lane = selector.get("lane_id")
    return lane if isinstance(lane, str) else None


def _worktree_target(ctx: RepoHandlerContext, path_text: str) -> Path:
    raw = Path(path_text)
    target = raw.resolve() if raw.is_absolute() else (ctx.repo_root / raw).resolve()
    repo_root = ctx.repo_root.resolve()
    try:
        target.relative_to(repo_root)
    except ValueError as exc:
        raise InvalidTransitionError("worktree path must stay inside the repository") from exc
    worktrees_root = (repo_root / STATE_DIR / WORKTREES_SUBDIR).resolve()
    try:
        target.relative_to(worktrees_root)
    except ValueError as exc:
        raise InvalidTransitionError(
            f"worktree path must stay under {STATE_DIR}/{WORKTREES_SUBDIR}"
        ) from exc
    return target


def _git_error_message(prefix: str, result: subprocess.CompletedProcess[str]) -> str:
    stderr = (result.stderr or result.stdout or "").strip()
    if len(stderr) > 200:
        stderr = stderr[:200] + "..."
    return f"{prefix}: {stderr}" if stderr else prefix


def _required_text(params: Mapping[str, Any], key: str) -> str:
    value = params.get(key)
    if not isinstance(value, str) or not value:
        raise RpcError("schema_invalid", f"{key} must be a non-empty string")
    return value


def _optional_text(params: Mapping[str, Any], key: str) -> str | None:
    value = params.get(key)
    if value is None:
        return None
    if not isinstance(value, str) or not value:
        raise RpcError("schema_invalid", f"{key} must be a non-empty string when present")
    return value


def _row_to_json(row: Mapping[str, Any]) -> JsonObject:
    return {str(key): _json_value(value) for key, value in row.items()}


def _json_value(value: Any) -> Any:
    if isinstance(value, Path):
        return str(value)
    if hasattr(value, "isoformat"):
        return _ts(value)
    if isinstance(value, (dict, list, str, int, float, bool)) or value is None:
        return value
    return str(value)

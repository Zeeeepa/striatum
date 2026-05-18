"""PG-backed branch confirmation handler."""

from __future__ import annotations

import subprocess
from collections.abc import Mapping
from pathlib import Path
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import InvalidTransitionError, WorkflowError
from striatum.git_helpers import current_git_branch


@register_pg_handler("branch.confirm")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = _required_text(params, "run_id", method="branch.confirm")
    branch = _required_text(params, "branch", method="branch.confirm")
    create = bool(params.get("create", False))
    use_current = bool(params.get("use_current", False))
    strict = bool(params.get("strict", False))
    if create and use_current:
        raise WorkflowError("--create and --use-current are mutually exclusive")
    if strict and (create or use_current):
        raise WorkflowError("--strict is incompatible with --create and --use-current")

    with transaction(ctx):
        run = ctx.row_by_id("runs", "run_id", run_id, for_update=True)
        state = str(run["state"])
        if state not in {"needs_branch_confirmation", "ready"}:
            raise InvalidTransitionError("run is not waiting for branch confirmation")

        repo_root = _repo_root_for(ctx, run)
        requested_branch = branch
        target_branch = branch
        created = False
        mode = "records_only"

        if use_current:
            mode = "use_current"
            current = current_git_branch(repo_root)
            if current is None:
                raise WorkflowError(
                    "--use-current requires a detectable current git branch in the target repo"
                )
            if branch != current:
                raise WorkflowError(
                    f"--use-current was given but --branch={branch!r} does not match "
                    f"current git branch {current!r}"
                )
            target_branch = current
        elif create:
            mode = "create"
            target_branch, created = git_create_or_checkout_branch(repo_root, branch)
        elif strict:
            mode = "strict"
            current = current_git_branch(repo_root)
            if current != branch:
                raise WorkflowError(
                    f"--strict requires current git branch to match --branch={branch!r}; "
                    f"current branch is {current!r}"
                )

        current_branch = current_git_branch(repo_root)
        now = ctx.now()
        with ctx.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.runs
                SET branch_name = %s,
                    branch_confirmed_at = %s,
                    branch_confirmed_by = 'human',
                    state = 'ready'
                WHERE repository_id = %s AND run_id = %s
                """,
                (target_branch, now, ctx.repository_id, run_id),
            )
        ctx.append_event(
            run_id=run_id,
            event_type="run.branch_confirmed",
            payload={"branch": target_branch, "mode": mode, "created": created},
        )
        warning = None
        if current_branch is not None and current_branch != target_branch:
            warning = "current git branch differs from recorded branch confirmation"
        return {
            "run_id": run_id,
            "state": "ready",
            "branch": target_branch,
            "requested_branch": requested_branch,
            "current_git_branch": current_branch,
            "records_only": True,
            "warning": warning,
            "created": created,
            "mode": mode,
        }


def git_create_or_checkout_branch(repo: Path, branch: str) -> tuple[str, bool]:
    create_result = subprocess.run(
        ["git", "checkout", "-b", branch],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if create_result.returncode == 0:
        return branch, True
    checkout_result = subprocess.run(
        ["git", "checkout", branch],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if checkout_result.returncode == 0:
        return branch, False
    stderr = (checkout_result.stderr or create_result.stderr or "").strip()
    if len(stderr) > 200:
        stderr = stderr[:200] + "..."
    raise WorkflowError(
        f"git checkout failed for branch {branch!r}: {stderr}"
        if stderr
        else f"git checkout failed for branch {branch!r}"
    )


def _repo_root_for(ctx: RepoHandlerContext, run: Mapping[str, Any]) -> Path:
    raw = run.get("repo_root")
    if raw is None:
        return ctx.repo_root
    return Path(str(raw)).expanduser().resolve()


def _required_text(params: Mapping[str, Any], key: str, *, method: str) -> str:
    value = str(params.get(key) or "")
    if value == "":
        required = "run_id and branch" if key in {"run_id", "branch"} else key
        raise RpcError("schema_invalid", f"{method} requires {required}")
    return value

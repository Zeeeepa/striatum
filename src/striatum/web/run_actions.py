"""Run and job mutation route helpers for the local web UI."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from striatum.git_utils import short_git_status as _short_git_status

JsonObject = dict[str, Any]

JsonSender = Callable[[int, JsonObject], None]
JsonBodyReader = Callable[[int], dict[str, Any] | None]


@dataclass(frozen=True)
class RunActionContext:
    repo: Path
    allow_mutations: bool
    headers: Any
    rfile: Any
    send_json: JsonSender
    read_json_body_strict: JsonBodyReader


def handle_workflow_run_now(ctx: RunActionContext, rel_path: str) -> None:
    """Handle POST /workflows/run/<path>."""

    from striatum.errors import InvalidTransitionError, WorkflowError
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "run requires --allow-mutations"))
        return
    target = _workflow_run_target(ctx, rel_path)
    if target is None:
        return
    repo_root = ctx.repo.resolve()
    rel_parts = target.relative_to(repo_root).parts
    try:
        workflow_arg = "/".join(rel_parts)
        prepared = call_repo_method(ctx.repo, "run.prepare", {"workflow": workflow_arg})
        run_id = str(prepared["run_id"])
        requires_confirm = (
            prepared.get("state") == "needs_branch_confirmation"
            and prepared.get("branch_mode") != "auto"
        )
        if prepared.get("branch_mode") == "auto":
            suggested = prepared.get("suggested_branch_name")
            if isinstance(suggested, str) and suggested:
                call_repo_method(
                    ctx.repo,
                    "branch.confirm",
                    {"run_id": run_id, "branch": suggested, "create": True},
                )
        if requires_confirm:
            ctx.send_json(
                200,
                {
                    "ok": True,
                    "data": {
                        "run_id": run_id,
                        "status": "needs_branch_confirmation",
                        "suggested_branch_name": prepared.get("suggested_branch_name"),
                    },
                },
            )
            return
        call_repo_method(ctx.repo, "run.start", {"run_id": run_id})
    except ServiceDaemonRpcError as exc:
        if exc.code in {"workflow_error", "schema_invalid"}:
            _send_workflow_error(
                ctx,
                exc.message,
                field_path=exc.details.get("field_path") if exc.details else None,
            )
            return
        ctx.send_json(exc.status, _daemon_error(exc))
        return
    except WorkflowError as exc:
        _send_workflow_error(ctx, str(exc), field_path=getattr(exc, "field_path", None))
        return
    except InvalidTransitionError as exc:
        ctx.send_json(
            409,
            {"ok": False, "error": {"code": 409, "message": str(exc), "kind": "invalid_transition"}},
        )
        return
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
        return
    ctx.send_json(200, {"ok": True, "data": {"run_id": run_id, "status": "running"}})


def handle_run_branch_confirm(ctx: RunActionContext, run_id: str) -> None:
    """Confirm a run branch and start the run."""

    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "branch confirm requires --allow-mutations"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    branch_name = body.get("branch")
    create = bool(body.get("create", True))
    use_current = bool(body.get("use_current", False))
    if not isinstance(branch_name, str) or not branch_name:
        ctx.send_json(400, _error(400, "branch is required"))
        return
    try:
        confirmed = call_repo_method(
            ctx.repo,
            "branch.confirm",
            {
                "run_id": run_id,
                "branch": branch_name,
                "create": create,
                "use_current": use_current,
            },
        )
        call_repo_method(ctx.repo, "run.start", {"run_id": run_id})
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _daemon_error(exc))
        return
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
        return
    ctx.send_json(
        200,
        {
            "ok": True,
            "data": {
                "run_id": run_id,
                "state": "running",
                "branch": confirmed.get("branch"),
            },
        },
    )


def handle_run_cancel(ctx: RunActionContext, run_id: str) -> None:
    """Cancel a run from the web UI."""

    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "cancel run requires --allow-mutations"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    reason = body.get("reason") if isinstance(body.get("reason"), str) else None
    _call_with_daemon(
        ctx,
        method="run.cancel",
        params={"run_id": run_id, "reason": reason},
    )


def handle_run_pause(ctx: RunActionContext, run_id: str) -> None:
    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "pause requires --allow-mutations"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    reason = body.get("reason") if isinstance(body.get("reason"), str) else None
    _call_with_daemon(
        ctx,
        method="run.pause",
        params={"run_id": run_id, "reason": reason},
    )


def handle_run_resume(ctx: RunActionContext, run_id: str) -> None:
    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "resume requires --allow-mutations"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    _call_with_daemon(
        ctx,
        method="run.resume",
        params={"run_id": run_id},
    )


def handle_job_action(ctx: RunActionContext, path: str) -> None:
    """Handle /run/<id>/job/<job_id>/(cancel|retry)."""

    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "job action requires --allow-mutations"))
        return
    parts = path.strip("/").split("/")
    if len(parts) != 5 or parts[0] != "run" or parts[2] != "job":
        ctx.send_json(400, _error(400, "invalid job-action path"))
        return
    run_id, job_id, action = parts[1], parts[3], parts[4]
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    reason = "operator_canceled_via_web"
    cascade = True
    try:
        if action == "cancel":
            raw_reason = body.get("reason")
            reason = raw_reason if isinstance(raw_reason, str) and raw_reason else reason
            cascade = bool(body.get("cascade", True))
            result = call_repo_method(
                ctx.repo,
                "recovery.cancel_job",
                {"run_id": run_id, "job_id": job_id, "reason": reason, "cascade": cascade},
            )
        elif action == "retry":
            result = call_repo_method(
                ctx.repo,
                "run.retry_job",
                {"run_id": run_id, "job_id": job_id},
            )
        else:
            ctx.send_json(400, _error(400, f"unknown job action: {action}"))
            return
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _daemon_error(exc))
        return
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
        return
    ctx.send_json(200, {"ok": True, "data": result})


def _workflow_run_target(ctx: RunActionContext, rel_path: str) -> Path | None:
    if not rel_path:
        ctx.send_json(404, _error(404, "missing path"))
        return None
    if rel_path.startswith("/") or "\x00" in rel_path or ".." in Path(rel_path).parts:
        ctx.send_json(400, _error(400, "invalid path"))
        return None
    ctype = ctx.headers.get("Content-Type", "")
    if "application/json" not in ctype:
        ctx.send_json(415, _error(415, "Content-Type must be application/json"))
        return None
    try:
        length = int(ctx.headers.get("Content-Length") or "0")
    except ValueError:
        length = 0
    if length > 1024 * 1024:
        ctx.send_json(413, _error(413, "body too large (1 MB cap)"))
        return None
    try:
        _ = ctx.rfile.read(length).decode("utf-8", errors="replace") if length > 0 else ""
    except OSError as exc:
        ctx.send_json(400, _error(400, str(exc)))
        return None
    repo_root = ctx.repo.resolve()
    target = (ctx.repo / rel_path).resolve()
    try:
        target.relative_to(repo_root)
    except ValueError:
        ctx.send_json(400, _error(400, "path escapes repo"))
        return None
    rel_parts = target.relative_to(repo_root).parts
    if rel_parts and rel_parts[0] in (".git", ".striatum"):
        ctx.send_json(404, _error(404, "hidden path"))
        return None
    if not target.is_file():
        ctx.send_json(404, _error(404, "workflow.json not found"))
        return None
    return target


def _call_with_daemon(
    ctx: RunActionContext,
    *,
    method: str,
    params: JsonObject,
) -> None:
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        result = call_repo_method(ctx.repo, method, params)
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _daemon_error(exc))
        return
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, f"{type(exc).__name__}: {exc}"))
        return
    ctx.send_json(200, {"ok": True, "data": result})


def _send_workflow_error(
    ctx: RunActionContext,
    message: str,
    *,
    field_path: Any = None,
) -> None:
    if "git checkout failed" in message:
        ctx.send_json(
            409,
            {
                "ok": False,
                "error": {
                    "code": 409,
                    "message": message,
                    "kind": "dirty_tree",
                    "git_status": _short_git_status(ctx.repo),
                },
            },
        )
        return
    errors = []
    if isinstance(field_path, str) and field_path:
        errors.append({"field_path": field_path, "message": message})
    ctx.send_json(422, {"ok": False, "error": {"code": 422, "message": message, "errors": errors}})


def _daemon_error(exc: Any) -> JsonObject:
    error: JsonObject = {"code": exc.status, "message": exc.message}
    if exc.kind is not None:
        error["kind"] = exc.kind
    return {"ok": False, "error": error}


def _error(code: int, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


__all__ = [
    "RunActionContext",
    "handle_job_action",
    "handle_run_branch_confirm",
    "handle_run_cancel",
    "handle_run_pause",
    "handle_run_resume",
    "handle_workflow_run_now",
]

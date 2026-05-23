"""Daemon-routed operator action helpers for RFC 0050/RFC 0075 cutover."""

from __future__ import annotations

from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]
HtmlSender = Callable[[int, str], None]
JsonBodyReader = Callable[[int], dict[str, Any] | None]
TemplateEnvFactory = Callable[[], Any]

_SAFE_ID_CHARS = frozenset(
    "0123456789"
    "abcdefghijklmnopqrstuvwxyz"
    "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    "_-"
)


@dataclass(frozen=True)
class OperatorActionContext:
    repo: Path
    allow_mutations: bool
    send_json: JsonSender
    send_html: HtmlSender
    read_json_body_strict: JsonBodyReader
    jinja_env: TemplateEnvFactory


def render_cross_repo_page(ctx: OperatorActionContext) -> None:
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        payload = call_repo_method(ctx.repo, "cross_repo.list", {})
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _daemon_error(exc))
        return
    rows = payload.get("cross_repo_runs", payload.get("items", payload.get("runs")))
    runs = [dict(row) for row in rows if isinstance(row, Mapping)] if isinstance(rows, list) else []
    html = ctx.jinja_env().get_template("cross_repo_list.html").render(runs=runs)
    ctx.send_html(200, html)


def handle_recovery_action(ctx: OperatorActionContext, run_id: str, action: str) -> None:
    if not _mutation_allowed(ctx, "recovery action requires --allow-mutations"):
        return
    if not _is_safe_id(run_id):
        ctx.send_json(400, _error(400, "invalid run id"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    if action == "stale-leases":
        _call_with_daemon(ctx, "recovery.stale_leases", {"run_id": run_id})
        return
    if action == "process-reconcile":
        _call_with_daemon(ctx, "recovery.process_reconcile", {"run_id": run_id})
        return
    if action == "sweep":
        _call_with_daemon(
            ctx,
            "recovery.sweep",
            {
                "run_id": run_id,
                "dry_run": bool(body.get("dry_run", False)),
            },
        )
        return
    if action == "auto-finalize":
        params: JsonObject = {
            "run_id": run_id,
            "dry_run": bool(body.get("dry_run", True)),
            "force": bool(body.get("force", False)),
            "reset_circuit_breaker": bool(body.get("reset_circuit_breaker", False)),
            "allow_no_process_execution": bool(body.get("allow_no_process_execution", False)),
        }
        if body.get("mtime_grace_seconds") is not None:
            try:
                params["mtime_grace_seconds"] = int(body["mtime_grace_seconds"])
            except (TypeError, ValueError):
                ctx.send_json(400, _error(400, "mtime_grace_seconds must be an integer"))
                return
        _call_with_daemon(ctx, "recovery.auto_finalize", params)
        return
    if action == "resume":
        blocker_id = _required_body_text(body, "blocker_id", ctx)
        if blocker_id is None:
            return
        params = {
            "blocker_id": blocker_id,
            "force": bool(body.get("force", False)),
            "complete": bool(body.get("complete", False)),
        }
        for key in ("session_id", "summary"):
            value = _optional_body_text(body, key, ctx)
            if value is None and key in body:
                return
            if value:
                params[key] = value
        if body.get("extend_seconds") is not None:
            try:
                params["extend_seconds"] = int(body["extend_seconds"])
            except (TypeError, ValueError):
                ctx.send_json(400, _error(400, "extend_seconds must be an integer"))
                return
        _call_with_daemon(ctx, "recovery.resume", params)
        return
    ctx.send_json(404, _error(404, "unknown recovery action"))


def handle_worktree_action(
    ctx: OperatorActionContext,
    run_id: str,
    job_id: str,
    action: str,
) -> None:
    del run_id
    if not _mutation_allowed(ctx, "worktree action requires --allow-mutations"):
        return
    if not _is_safe_id(job_id):
        ctx.send_json(400, _error(400, "invalid job id"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    if action == "create":
        session_id = _required_body_text(body, "session_id", ctx)
        lease_id = _required_body_text(body, "lease_id", ctx)
        if session_id is None or lease_id is None:
            return
        _call_with_daemon(
            ctx,
            "worktree.create",
            {"session_id": session_id, "job_id": job_id, "lease_id": lease_id},
        )
        return
    if action == "release":
        worktree_id = _required_body_text(body, "worktree_id", ctx)
        if worktree_id is None:
            return
        _call_with_daemon(ctx, "worktree.release", {"worktree_id": worktree_id})
        return
    ctx.send_json(404, _error(404, "unknown worktree action"))


def handle_supervise_action(
    ctx: OperatorActionContext,
    session_id: str,
    action: str,
) -> None:
    if not _mutation_allowed(ctx, "supervise action requires --allow-mutations"):
        return
    if not _is_safe_id(session_id):
        ctx.send_json(400, _error(400, "invalid session id"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    if action == "start":
        _call_with_daemon(ctx, "supervise.start", {"session_id": session_id})
        return
    if action == "send":
        packet_id = _required_body_text(body, "packet_id", ctx)
        if packet_id is None:
            return
        _call_with_daemon(ctx, "supervise.send", {"session_id": session_id, "packet_id": packet_id})
        return
    if action == "stop":
        reason = _optional_body_text(body, "reason", ctx)
        if reason is None and "reason" in body:
            return
        _call_with_daemon(
            ctx,
            "supervise.stop",
            {"session_id": session_id, "reason": reason or "operator_stop_via_web"},
        )
        return
    ctx.send_json(404, _error(404, "unknown supervise action"))


def handle_cross_repo_cancel(ctx: OperatorActionContext, cross_repo_run_id: str) -> None:
    if not _mutation_allowed(ctx, "cross-repo cancel requires --allow-mutations"):
        return
    if not _is_safe_id(cross_repo_run_id):
        ctx.send_json(400, _error(400, "invalid cross-repo run id"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    reason = _optional_body_text(body, "reason", ctx)
    if reason is None and "reason" in body:
        return
    _call_with_daemon(
        ctx,
        "cross_repo.cancel",
        {"cross_repo_run_id": cross_repo_run_id, "reason": reason or "operator canceled via web"},
    )


def handle_artifact_commit_apply(
    ctx: OperatorActionContext,
    run_id: str,
    artifact_id: str,
) -> None:
    del run_id
    if not _mutation_allowed(ctx, "git commit apply requires --allow-mutations"):
        return
    if not _is_safe_id(artifact_id):
        ctx.send_json(400, _error(400, "invalid artifact id"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    confirm_request_id = _required_body_text(body, "confirm_request_id", ctx)
    if confirm_request_id is None:
        return

    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        payload = call_repo_method(ctx.repo, "artifact.show", {"artifact_id": artifact_id})
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _daemon_error(exc))
        return
    artifact = payload.get("artifact")
    if not isinstance(artifact, Mapping):
        ctx.send_json(500, _error(500, "daemon artifact DTO missing artifact"))
        return
    if artifact.get("artifact_kind") != "commit_request":
        ctx.send_json(400, _error(400, "artifact is not a commit_request"))
        return
    repo_path = artifact.get("repo_path")
    if not isinstance(repo_path, str) or not repo_path:
        ctx.send_json(500, _error(500, "commit_request artifact missing repo_path"))
        return
    _call_with_daemon(
        ctx,
        "git.commit_apply",
        {
            "schema_version": 1,
            "commit_request_path": repo_path,
            "confirm": True,
            "confirm_request_id": confirm_request_id,
        },
    )


def _mutation_allowed(ctx: OperatorActionContext, message: str) -> bool:
    if ctx.allow_mutations:
        return True
    ctx.send_json(405, _error(405, message))
    return False


def _call_with_daemon(ctx: OperatorActionContext, method: str, params: JsonObject) -> None:
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


def _required_body_text(
    body: Mapping[str, Any],
    key: str,
    ctx: OperatorActionContext,
) -> str | None:
    if key in body and not isinstance(body.get(key), str):
        ctx.send_json(400, _error(400, f"{key} must be a string"))
        return None
    value = _optional_body_text(body, key, ctx)
    if value is None:
        ctx.send_json(400, _error(400, f"{key} is required"))
    return value


def _optional_body_text(
    body: Mapping[str, Any],
    key: str,
    ctx: OperatorActionContext,
) -> str | None:
    value = body.get(key)
    if value is None:
        return None
    if not isinstance(value, str):
        ctx.send_json(400, _error(400, f"{key} must be a string"))
        return None
    stripped = value.strip()
    return stripped or None


def _is_safe_id(value: str) -> bool:
    return bool(value) and all(char in _SAFE_ID_CHARS for char in value)


def _daemon_error(exc: Any) -> JsonObject:
    error: JsonObject = {"code": exc.status, "message": exc.message}
    if exc.kind is not None:
        error["kind"] = exc.kind
    return {"ok": False, "error": error}


def _error(code: int, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


__all__ = [
    "OperatorActionContext",
    "handle_artifact_commit_apply",
    "handle_cross_repo_cancel",
    "handle_recovery_action",
    "handle_supervise_action",
    "handle_worktree_action",
    "render_cross_repo_page",
]

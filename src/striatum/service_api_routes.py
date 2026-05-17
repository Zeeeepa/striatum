"""JSON and SSE route helpers for the local service handler."""

from __future__ import annotations

from collections.abc import Callable, Mapping
from dataclasses import dataclass
from typing import Any

from striatum.service_sse import (
    encode_sse_event,
    sse_since as parse_sse_since,
    stream_daemon_events,
)
from striatum.web.workflows import list_repo_tree

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]
InvokeFunc = Callable[[list[str]], JsonObject]
VersionFunc = Callable[[], str]
ServiceModeFunc = Callable[[Any], str]
LegacyFallbackChecker = Callable[[str], bool]
LegacyStreamEvents = Callable[..., Any]


@dataclass(frozen=True)
class ServiceApiRouteContext:
    state: Any
    server: Any
    headers: Any
    writer: Any
    send_json: JsonSender
    invoke_func: InvokeFunc
    striatum_version: VersionFunc
    service_mode: ServiceModeFunc
    legacy_web_read_fallback_enabled: LegacyFallbackChecker
    legacy_stream_events_body: LegacyStreamEvents
    poll_interval_seconds: float


def handle_health(ctx: ServiceApiRouteContext) -> None:
    ctx.send_json(
        200,
        {
            "ok": True,
            "data": {
                "started_at": ctx.state.started_at,
                "version": ctx.striatum_version(),
                "mode": ctx.service_mode(ctx.server),
                "allow_mutations": bool(ctx.state.allow_mutations),
            },
        },
    )


def handle_invoke(ctx: ServiceApiRouteContext, argv: list[str]) -> None:
    result = ctx.invoke_func(argv)
    status = 200 if result.get("ok") else 500
    ctx.send_json(status, result)


def handle_doctor(
    ctx: ServiceApiRouteContext,
    query: dict[str, list[str]],
) -> None:
    run_id = query.get("run_id", [None])[0]
    params: dict[str, Any] = {"verbose": True}
    argv = ["doctor", "--verbose"]
    if run_id:
        params["run_id"] = run_id
        argv.extend(["--run-id", run_id])
    handle_daemon_read(ctx, "doctor", params, legacy_argv=argv)


def handle_repo_tree(
    ctx: ServiceApiRouteContext,
    query: dict[str, list[str]],
) -> None:
    rel_path = query.get("path", [""])[0]
    tree = list_repo_tree(ctx.state.repo, rel_path)
    if tree is None:
        ctx.send_json(404, _error(404, "directory not found"))
        return
    ctx.send_json(200, {"ok": True, "data": tree})


def handle_run_subpath(
    ctx: ServiceApiRouteContext,
    suffix: str,
    query: dict[str, list[str]],
) -> None:
    parts = suffix.split("/")
    run_id = parts[0]
    if not run_id:
        ctx.send_json(400, _error(400, "missing run_id"))
        return
    if len(parts) == 1:
        handle_daemon_read(
            ctx,
            "status",
            {"run_id": run_id},
            legacy_argv=["status", "--run-id", run_id],
        )
        return
    sub = parts[1]
    if sub == "why":
        target = query.get("id", [None])[0]
        if not target:
            ctx.send_json(400, _error(400, "missing ?id=<entity>"))
            return
        handle_daemon_read(ctx, "why", {"target_id": target}, legacy_argv=["why", target])
        return
    if sub == "dashboard":
        handle_daemon_read(
            ctx,
            "dashboard",
            {"run_id": run_id},
            legacy_argv=["dashboard", "--run-id", run_id, "--once"],
        )
        return
    if sub == "events":
        since = sse_since(ctx, query)
        stream_events(ctx, run_id, since=since)
        return
    if sub == "artifacts":
        handle_daemon_read(
            ctx,
            "list.artifacts",
            {"run_id": run_id},
            legacy_argv=["list", "artifacts", "--run-id", run_id],
        )
        return
    ctx.send_json(404, _error(404, "not found"))


def handle_daemon_read(
    ctx: ServiceApiRouteContext,
    method: str,
    params: Mapping[str, Any],
    *,
    legacy_argv: list[str] | None = None,
) -> None:
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        payload = call_repo_method(ctx.state.repo, method, dict(params))
    except ServiceDaemonRpcError as exc:
        if legacy_argv is not None and ctx.legacy_web_read_fallback_enabled(exc.code):
            handle_invoke(ctx, legacy_argv)
            return
        ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
        return
    ctx.send_json(200, {"ok": True, "data": payload})


def sse_since(
    ctx: ServiceApiRouteContext,
    query: dict[str, list[str]],
) -> int:
    return parse_sse_since(dict(ctx.headers.items()), query)


def stream_events(
    ctx: ServiceApiRouteContext,
    run_id: str,
    *,
    since: int,
) -> None:
    if not ctx.state.acquire_sse_slot(run_id):
        ctx.send_json(
            429,
            _error(429, f"too many concurrent SSE streams for run {run_id}"),
        )
        return
    try:
        if ctx.legacy_web_read_fallback_enabled("daemon_unreachable"):
            ctx.legacy_stream_events_body(
                ctx.writer,
                repo=ctx.state.repo,
                run_id=run_id,
                since=since,
                poll_interval_seconds=ctx.poll_interval_seconds,
            )
            return
        stream_events_daemon_body(ctx, run_id, since=since)
    finally:
        ctx.state.release_sse_slot(run_id)


def stream_events_daemon_body(
    ctx: ServiceApiRouteContext,
    run_id: str,
    *,
    since: int,
) -> None:
    from striatum import service_daemon

    stream_daemon_events(
        ctx.writer,
        repo=ctx.state.repo,
        run_id=run_id,
        since=since,
        shutting_down=lambda: ctx.state.shutting_down,
        send_json=ctx.send_json,
        poll_interval_seconds=ctx.poll_interval_seconds,
        call_method=service_daemon.call_repo_method,
    )


def write_sse_event(
    ctx: ServiceApiRouteContext,
    event: str,
    event_id: int,
    payload: JsonObject,
) -> None:
    ctx.writer.wfile.write(encode_sse_event(event, int(event_id), payload))
    ctx.writer.wfile.flush()


def _error(code: int, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


def _rpc_error(code: str, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


__all__ = [
    "ServiceApiRouteContext",
    "handle_daemon_read",
    "handle_doctor",
    "handle_health",
    "handle_invoke",
    "handle_repo_tree",
    "handle_run_subpath",
    "sse_since",
    "stream_events",
    "stream_events_daemon_body",
    "write_sse_event",
]

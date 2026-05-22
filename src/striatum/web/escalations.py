"""Escalation inbox route helpers for the local web UI."""

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
class EscalationRouteContext:
    repo: Path
    allow_mutations: bool
    send_json: JsonSender
    send_html: HtmlSender
    read_json_body_strict: JsonBodyReader
    jinja_env: TemplateEnvFactory


def render_escalation_list_page(
    ctx: EscalationRouteContext,
    query: Mapping[str, list[str]],
) -> None:
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    run_id = _optional_query_value(query, "run_id")
    if run_id is not None and not _is_safe_id(run_id):
        ctx.send_json(400, _error(400, "invalid run id"))
        return
    params: JsonObject = {}
    if run_id is not None:
        params["run_id"] = run_id
    try:
        payload = call_repo_method(ctx.repo, "escalation.list", params)
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
        return
    rows = payload.get("escalations")
    escalations = (
        [dict(row) for row in rows if isinstance(row, Mapping)]
        if isinstance(rows, list)
        else []
    )
    html = ctx.jinja_env().get_template("escalation_list.html").render(
        escalations=escalations,
        count=payload.get("count", len(escalations)),
        run_id=run_id or "",
        state=payload.get("state") or "open",
    )
    ctx.send_html(200, html)


def render_escalation_detail_page(
    ctx: EscalationRouteContext,
    escalation_id: str,
) -> None:
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    if not _is_safe_id(escalation_id):
        ctx.send_json(400, _error(400, "invalid escalation id"))
        return
    try:
        payload = call_repo_method(
            ctx.repo,
            "escalation.show",
            {"escalation_id": escalation_id},
        )
    except ServiceDaemonRpcError as exc:
        ctx.send_json(exc.status, _rpc_error(exc.code, exc.message))
        return
    escalation_raw = payload.get("escalation")
    if not isinstance(escalation_raw, Mapping):
        ctx.send_json(500, _error(500, "daemon escalation detail DTO missing escalation"))
        return
    escalation = dict(escalation_raw)
    html = ctx.jinja_env().get_template("escalation_detail.html").render(
        escalation=escalation,
        can_resolve=(
            escalation.get("state") == "open"
            and escalation.get("source") != "human_checkpoint"
        ),
    )
    ctx.send_html(200, html)


def handle_escalation_resolve(
    ctx: EscalationRouteContext,
    escalation_id: str,
) -> None:
    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "escalation resolve requires --allow-mutations"))
        return
    if not _is_safe_id(escalation_id):
        ctx.send_json(400, _error(400, "invalid escalation id"))
        return
    body = ctx.read_json_body_strict(64 * 1024)
    if body is None:
        return
    params: JsonObject = {"escalation_id": escalation_id}
    decision_id, ok = _optional_body_text(body, "decision_id", ctx)
    if not ok:
        return
    resolution_note, ok = _optional_body_text(body, "resolution_note", ctx)
    if not ok:
        return
    if decision_id is not None:
        params["decision_id"] = decision_id
    if resolution_note is not None:
        params["resolution_note"] = resolution_note
    _call_with_daemon(ctx, method="escalation.resolve", params=params)


def _call_with_daemon(
    ctx: EscalationRouteContext,
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


def _optional_query_value(query: Mapping[str, list[str]], key: str) -> str | None:
    values = query.get(key)
    if not values:
        return None
    value = values[0]
    return value if value else None


def _optional_body_text(
    body: Mapping[str, Any],
    key: str,
    ctx: EscalationRouteContext,
) -> tuple[str | None, bool]:
    value = body.get(key)
    if value is None:
        return None, True
    if not isinstance(value, str):
        ctx.send_json(400, _error(400, f"{key} must be a string"))
        return None, False
    stripped = value.strip()
    return (stripped if stripped else None), True


def _is_safe_id(value: str) -> bool:
    return bool(value) and all(char in _SAFE_ID_CHARS for char in value)


def _daemon_error(exc: Any) -> JsonObject:
    error: JsonObject = {"code": exc.status, "message": exc.message}
    if exc.kind is not None:
        error["kind"] = exc.kind
    return {"ok": False, "error": error}


def _error(code: int, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


def _rpc_error(code: str, message: str) -> JsonObject:
    return {"ok": False, "error": {"code": code, "message": message}}


__all__ = [
    "EscalationRouteContext",
    "handle_escalation_resolve",
    "render_escalation_detail_page",
    "render_escalation_list_page",
]

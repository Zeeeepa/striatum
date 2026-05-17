"""HTTP route helpers for the local web chat surface."""

from __future__ import annotations

import json
import time
import uuid
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from striatum.web import chat_session

JsonObject = dict[str, Any]
ChatConfigLoader = Callable[[], Any | None]
FormReader = Callable[[int], dict[str, list[str]] | None]
JsonSender = Callable[[int, JsonObject], None]
HtmlSender = Callable[[int, str], None]
ResponseStarter = Callable[[int], None]
HeaderSender = Callable[[str, str], None]
HeaderFinisher = Callable[[], None]
TemplateEnvFactory = Callable[[], Any]
BriefingBuilder = Callable[[Path, bool], str]
HtmlEscaper = Callable[[str], str]
TokenFactory = Callable[[], str]


@dataclass(frozen=True)
class ChatRouteContext:
    repo: Path
    allow_mutations: bool
    chat_config: ChatConfigLoader
    read_form_body: FormReader
    send_json: JsonSender
    send_html: HtmlSender
    send_response: ResponseStarter
    send_header: HeaderSender
    end_headers: HeaderFinisher
    wfile: Any
    jinja_env: TemplateEnvFactory
    build_chat_briefing: BriefingBuilder
    html_escape: HtmlEscaper
    token_factory: TokenFactory
    poll_interval_seconds: float


def is_safe_id(value: str) -> bool:
    """Return true for ASCII-ish path component ids used by chat routes."""

    if not value:
        return False
    return all(ch.isalnum() or ch in "-_" for ch in value)


def render_chat_index_page(ctx: ChatRouteContext) -> None:
    config = ctx.chat_config()
    sessions = chat_session.list_chat_sessions(ctx.repo) if config else []
    try:
        html = ctx.jinja_env().get_template("chat_index.html").render(
            chat_configured=config is not None,
            sessions=sessions,
            model=getattr(config, "model", None),
            flavor=getattr(config, "flavor", None),
            mutations_allowed=ctx.allow_mutations,
            env_help="",
        )
        ctx.send_html(200, html)
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


def render_chat_subpath(ctx: ChatRouteContext, subpath: str) -> None:
    if not subpath:
        ctx.send_json(404, _error(404, "missing chat session"))
        return
    parts = subpath.strip("/").split("/")
    session_id = parts[0]
    if not is_safe_id(session_id):
        ctx.send_json(400, _error(400, "invalid chat session id"))
        return
    if len(parts) == 1:
        render_chat_session_page(ctx, session_id)
        return
    if len(parts) == 2 and parts[1] == "events":
        stream_chat_events(ctx, session_id)
        return
    ctx.send_json(404, _error(404, "not found"))


def render_chat_session_page(ctx: ChatRouteContext, session_id: str) -> None:
    from striatum.web.markdown import render as md_render

    config = ctx.chat_config()
    path = chat_session.chat_session_path(ctx.repo, session_id)
    if path is None or not path.is_file():
        ctx.send_json(404, _error(404, "chat session not found"))
        return
    messages = chat_session.read_display_messages(
        path,
        markdown_render=md_render,
        html_escape=ctx.html_escape,
    )
    try:
        html = ctx.jinja_env().get_template("chat.html").render(
            session_id=session_id,
            messages=messages,
            model=getattr(config, "model", None),
            flavor=getattr(config, "flavor", None),
        )
        ctx.send_html(200, html)
    except Exception as exc:  # noqa: BLE001
        ctx.send_json(500, _error(500, str(exc)))


def handle_chat_new(ctx: ChatRouteContext) -> None:
    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "chat new requires --allow-mutations"))
        return
    config = ctx.chat_config()
    if config is None:
        ctx.send_json(
            412,
            _error(
                412,
                "chat is not configured; set STRIATUM_CHAT_API_BASE_URL / "
                "API_KEY / MODEL / API_FLAVOR",
            ),
        )
        return

    # Read but discard the form body if present.
    ctx.read_form_body(4096)
    session_id = uuid.uuid4().hex[:8]
    target = chat_session.chat_session_path(ctx.repo, session_id)
    if target is None:
        ctx.send_json(500, _error(500, "could not allocate chat session"))
        return
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text("", encoding="utf-8")

    # Briefing failure must not block chat creation.
    try:
        briefing = ctx.build_chat_briefing(ctx.repo, ctx.allow_mutations)
        chat_session.append_jsonl(
            target,
            {
                "role": "system",
                "content": briefing,
                "created_at": chat_session.utc_now_iso(),
            },
        )
    except Exception:  # noqa: BLE001
        pass

    _redirect(ctx, f"/chat/{session_id}")


def handle_chat_send(ctx: ChatRouteContext, session_id: str) -> None:
    if not is_safe_id(session_id):
        ctx.send_json(400, _error(400, "invalid chat session id"))
        return
    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "chat send requires --allow-mutations"))
        return
    config = ctx.chat_config()
    if config is None:
        ctx.send_json(412, _error(412, "chat is not configured"))
        return
    path = chat_session.chat_session_path(ctx.repo, session_id)
    if path is None or not path.is_file():
        ctx.send_json(404, _error(404, "chat session not found"))
        return
    form = ctx.read_form_body(64 * 1024)
    if form is None:
        return
    message = (form.get("message") or [""])[0]
    if not isinstance(message, str) or not message.strip():
        ctx.send_json(400, _error(400, "message is required"))
        return

    chat_session.append_jsonl(
        path,
        {"role": "user", "content": message, "created_at": chat_session.utc_now_iso()},
    )

    from striatum.web.chat_provider import ChatProviderError, stream_chat_response
    from striatum.web.chat_tools import execute_tool, tool_schemas, wrap_tool_result

    tools = tool_schemas(allow_mutations=ctx.allow_mutations, flavor=config.flavor)
    max_iterations = 10
    try:
        for _iteration in range(max_iterations):
            history = chat_session.read_chat_history(path, flavor=config.flavor)
            system_text, conversational = chat_session.split_system(history)
            tool_calls: list[dict[str, Any]] = []
            assistant_text = ""
            for event in stream_chat_response(
                config,
                conversational,
                tools=tools,
                system=system_text,
            ):
                etype = event.get("type")
                if etype == "text":
                    chunk = str(event.get("text") or "")
                    if chunk:
                        assistant_text += chunk
                elif etype == "tool_call":
                    tool_calls.append(event)
                elif etype == "stop":
                    break

            if assistant_text:
                chat_session.append_jsonl(
                    path,
                    {
                        "role": "assistant",
                        "content": assistant_text,
                        "streaming": False,
                        "created_at": chat_session.utc_now_iso(),
                    },
                )
            if not tool_calls:
                break
            for call in tool_calls:
                _execute_chat_tool_call(
                    ctx,
                    path=path,
                    session_id=session_id,
                    call=call,
                    execute_tool=execute_tool,
                    wrap_tool_result=wrap_tool_result,
                )
        else:
            chat_session.append_jsonl(
                path,
                {
                    "role": "system",
                    "content": (
                        f"[loop cap] tool-call loop hit {max_iterations} "
                        "iterations; halted."
                    ),
                    "created_at": chat_session.utc_now_iso(),
                },
            )
    except ChatProviderError as exc:
        chat_session.append_jsonl(
            path,
            {
                "role": "system",
                "content": f"[chat error] {exc}",
                "created_at": chat_session.utc_now_iso(),
            },
        )
        ctx.send_json(502, _error(502, str(exc)))
        return
    except Exception as exc:  # noqa: BLE001
        chat_session.append_jsonl(
            path,
            {
                "role": "system",
                "content": f"[unexpected error] {type(exc).__name__}: {exc}",
                "created_at": chat_session.utc_now_iso(),
            },
        )
        ctx.send_json(500, _error(500, "chat send failed"))
        return

    ctx.send_response(204)
    ctx.send_header("Content-Length", "0")
    ctx.end_headers()


def handle_chat_confirm_tool(ctx: ChatRouteContext, session_id: str, tool_id: str) -> None:
    if not is_safe_id(session_id) or not is_safe_id(tool_id):
        ctx.send_json(400, _error(400, "invalid chat confirmation id"))
        return
    if not ctx.allow_mutations:
        ctx.send_json(
            405,
            _error(405, "mutations_disabled", kind="mutations_disabled"),
        )
        return
    path = chat_session.chat_session_path(ctx.repo, session_id)
    if path is None or not path.is_file():
        ctx.send_json(404, _error(404, "chat session not found"))
        return
    form = ctx.read_form_body(4096)
    if form is None:
        return
    token = (form.get("token") or [""])[0]
    action = (form.get("action") or ["confirm"])[0]
    pending = chat_session.find_pending_tool_confirmation(path=path, tool_id=tool_id)
    if pending is None:
        ctx.send_json(404, _error(404, "confirmation not found"))
        return
    if str(pending.get("confirmation_token") or "") != token:
        ctx.send_json(403, _error(403, "confirmation token mismatch"))
        return
    if action == "cancel":
        _record_canceled_confirmation(path, pending)
        ctx.send_json(
            200,
            _error(
                "mutation_canceled",
                "operator canceled workflow write",
                ok=False,
            ),
        )
        return

    raw_tool_args = pending.get("tool_input")
    tool_args: dict[str, Any] = dict(raw_tool_args) if isinstance(raw_tool_args, dict) else {}
    from striatum.web.chat_tools import execute_tool, wrap_tool_result

    raw_result = execute_tool(
        "generate_workflow_write",
        tool_args,
        repo=ctx.repo,
        allow_mutations=True,
        operator_confirmed=True,
    )
    chat_session.append_jsonl(
        path,
        {
            **pending,
            "state": "used",
            "used_at": chat_session.utc_now_iso(),
            "confirmation_token": "<used>",
        },
    )
    chat_session.append_jsonl(
        path,
        {
            "role": "tool_result",
            "tool_use_id": tool_id,
            "tool_name": "generate_workflow_write",
            "result": wrap_tool_result("generate_workflow_write", tool_args, raw_result),
            "created_at": chat_session.utc_now_iso(),
        },
    )
    if raw_result.startswith("[error]"):
        ctx.send_json(400, _error(400, raw_result))
        return
    ctx.send_json(200, json.loads(raw_result))


def handle_chat_stop(ctx: ChatRouteContext, session_id: str) -> None:
    if not is_safe_id(session_id):
        ctx.send_json(400, _error(400, "invalid chat session id"))
        return
    if not ctx.allow_mutations:
        ctx.send_json(405, _error(405, "chat stop requires --allow-mutations"))
        return
    _redirect(ctx, "/chat")


def stream_chat_events(ctx: ChatRouteContext, session_id: str) -> None:
    if not is_safe_id(session_id):
        ctx.send_json(400, _error(400, "invalid chat session id"))
        return
    path = chat_session.chat_session_path(ctx.repo, session_id)
    if path is None:
        ctx.send_json(404, _error(404, "chat session not found"))
        return

    ctx.send_response(200)
    ctx.send_header("Content-Type", "text/event-stream")
    ctx.send_header("Cache-Control", "no-cache")
    ctx.send_header("Connection", "keep-alive")
    ctx.send_header(
        "Content-Security-Policy",
        "default-src 'self'; script-src 'self'; style-src 'self'; "
        "img-src 'self' data:; connect-src 'self'",
    )
    ctx.end_headers()
    last_offset = 0
    deadline = time.monotonic() + 600.0
    try:
        while time.monotonic() < deadline:
            if path.is_file():
                size = path.stat().st_size
                if size > last_offset:
                    with path.open("rb") as fh:
                        fh.seek(last_offset)
                        chunk = fh.read()
                    last_offset = size
                    for raw_line in chunk.decode("utf-8", errors="replace").splitlines():
                        if not raw_line.strip():
                            continue
                        ctx.wfile.write(b"data: ")
                        ctx.wfile.write(raw_line.encode("utf-8"))
                        ctx.wfile.write(b"\n\n")
                        ctx.wfile.flush()
            time.sleep(ctx.poll_interval_seconds)
    except (BrokenPipeError, ConnectionResetError):
        return


def _execute_chat_tool_call(
    ctx: ChatRouteContext,
    *,
    path: Path,
    session_id: str,
    call: Mapping[str, Any],
    execute_tool: Callable[..., str],
    wrap_tool_result: Callable[[str, dict[str, Any], str], str],
) -> None:
    tool_id = str(call.get("id") or "")
    tool_name = str(call.get("name") or "")
    tool_args_raw = call.get("args") or {}
    tool_args: dict[str, Any] = (
        dict(tool_args_raw) if isinstance(tool_args_raw, Mapping) else {}
    )
    chat_session.append_jsonl(
        path,
        {
            "role": "tool_use",
            "tool_use_id": tool_id,
            "tool_name": tool_name,
            "tool_input": tool_args,
            "created_at": chat_session.utc_now_iso(),
        },
    )
    if tool_name == "generate_workflow_write":
        raw_result = chat_session.queue_workflow_write_confirmation(
            path=path,
            session_id=session_id,
            tool_id=tool_id,
            tool_args=tool_args,
            allow_mutations=ctx.allow_mutations,
            token_factory=ctx.token_factory,
        )
    else:
        raw_result = execute_tool(
            tool_name,
            tool_args,
            repo=ctx.repo,
            allow_mutations=ctx.allow_mutations,
        )
    wrapped = wrap_tool_result(tool_name, tool_args, raw_result)
    chat_session.append_jsonl(
        path,
        {
            "role": "tool_result",
            "tool_use_id": tool_id,
            "tool_name": tool_name,
            "result": wrapped,
            "created_at": chat_session.utc_now_iso(),
        },
    )


def _record_canceled_confirmation(path: Path, pending: Mapping[str, Any]) -> None:
    chat_session.append_jsonl(
        path,
        {
            "role": "system",
            "content": "[mutation_canceled] operator canceled generate_workflow_write",
            "created_at": chat_session.utc_now_iso(),
        },
    )
    chat_session.append_jsonl(
        path,
        {
            **pending,
            "state": "used",
            "used_at": chat_session.utc_now_iso(),
            "confirmation_token": "<used>",
        },
    )


def _redirect(ctx: ChatRouteContext, location: str) -> None:
    ctx.send_response(303)
    ctx.send_header("Location", location)
    ctx.send_header("Content-Length", "0")
    ctx.end_headers()


def _error(
    code: object,
    message: str,
    *,
    ok: bool = False,
    kind: str | None = None,
) -> JsonObject:
    error: JsonObject = {"code": code, "message": message}
    if kind is not None:
        error["kind"] = kind
    return {"ok": ok, "error": error}


__all__ = [
    "ChatRouteContext",
    "handle_chat_confirm_tool",
    "handle_chat_new",
    "handle_chat_send",
    "handle_chat_stop",
    "is_safe_id",
    "render_chat_index_page",
    "render_chat_session_page",
    "render_chat_subpath",
    "stream_chat_events",
]

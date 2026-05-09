"""RFC 0023 V1: outbound chat-completion clients.

Provider-neutral: striatum streams HTTPS requests to an
operator-configured chat endpoint. Two flavors:

- ``anthropic_messages`` — Anthropic Messages API
  (``POST /v1/messages``, ``x-api-key`` header).
- ``openai_chat`` — OpenAI Chat Completions API
  (``POST /v1/chat/completions``, ``Authorization: Bearer`` header).

Both flavors stream responses and yield ``(text_chunk, is_final)``
tuples. The route handler in ``service.py`` collects the chunks into
the chat session's transcript JSONL and pushes them to the SSE stream.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from http.client import HTTPConnection, HTTPSConnection
from typing import Any, Iterator, Mapping
from urllib.parse import urlsplit

__all__ = [
    "ChatProviderConfig",
    "ChatProviderError",
    "stream_chat_response",
    "validate_base_url",
]


ALLOWED_FLAVORS = ("anthropic_messages", "openai_chat")


class ChatProviderError(RuntimeError):
    """Raised when a chat provider request fails or returns malformed
    output. The message is operator-facing; never includes the API key
    even when the upstream error did."""


@dataclass(frozen=True)
class ChatProviderConfig:
    base_url: str
    api_key: str
    model: str
    flavor: str

    @classmethod
    def from_env(cls, env: Mapping[str, str]) -> "ChatProviderConfig | None":
        base_url = env.get("STRIATUM_CHAT_API_BASE_URL")
        api_key = env.get("STRIATUM_CHAT_API_KEY")
        model = env.get("STRIATUM_CHAT_MODEL")
        flavor = env.get("STRIATUM_CHAT_API_FLAVOR")
        if not (base_url and api_key and model and flavor):
            return None
        if flavor not in ALLOWED_FLAVORS:
            raise ChatProviderError(
                f"unknown STRIATUM_CHAT_API_FLAVOR {flavor!r}; "
                f"expected one of: {', '.join(ALLOWED_FLAVORS)}"
            )
        validate_base_url(base_url)
        return cls(
            base_url=base_url.rstrip("/"),
            api_key=api_key,
            model=model,
            flavor=flavor,
        )


def validate_base_url(base_url: str) -> None:
    """RFC 0023 V1 design-review F1 (security): refuse non-HTTPS unless
    the endpoint is loopback (e.g., a local Ollama instance).

    Raises :class:`ChatProviderError` for invalid schemes or hosts.
    """
    parts = urlsplit(base_url)
    if parts.scheme not in ("http", "https"):
        raise ChatProviderError(
            f"STRIATUM_CHAT_API_BASE_URL must be http or https, got {parts.scheme!r}"
        )
    if parts.scheme == "http":
        host = parts.hostname or ""
        if not (
            host == "localhost"
            or host == "127.0.0.1"
            or host == "::1"
            or host.startswith("127.")
        ):
            raise ChatProviderError(
                f"STRIATUM_CHAT_API_BASE_URL with scheme http is allowed only for "
                f"loopback hosts (localhost / 127.*); got host {host!r}. "
                f"Use https for remote endpoints."
            )


def stream_chat_response(
    config: ChatProviderConfig,
    messages: list[dict[str, Any]],
    *,
    tools: list[dict[str, Any]] | None = None,
    system: str | None = None,
    max_tokens: int = 4096,
    timeout_seconds: float = 60.0,
) -> Iterator[dict[str, Any]]:
    """Stream the assistant response for the given conversation.

    Yields events of the form:

    - ``{"type": "text", "text": "..."}`` — a text chunk to append.
    - ``{"type": "tool_call", "id": "...", "name": "...", "args": {...}}`` —
      the model requested a tool. Emitted once per tool call after the
      input JSON is fully accumulated.
    - ``{"type": "stop", "reason": "end_turn"|"tool_use"|...}`` — final
      event. ``tool_use`` means the caller should execute the requested
      tools and re-enter with the tool results appended to ``messages``.

    ``tools`` is the flavor-specific tool spec (Anthropic shape OR
    OpenAI shape). The caller picks one matching ``config.flavor``.
    ``system`` is an optional system prompt; for OpenAI it becomes a
    system-role message at the top, for Anthropic it goes in the
    top-level ``system`` field.
    """
    if config.flavor == "anthropic_messages":
        yield from _stream_anthropic(
            config, messages, max_tokens, timeout_seconds,
            tools=tools, system=system,
        )
    elif config.flavor == "openai_chat":
        yield from _stream_openai(
            config, messages, max_tokens, timeout_seconds,
            tools=tools, system=system,
        )
    else:  # defensive; from_env validates this
        raise ChatProviderError(f"unsupported flavor {config.flavor!r}")


def _open_connection(
    base_url: str, *, timeout_seconds: float
) -> tuple[HTTPConnection | HTTPSConnection, str]:
    parts = urlsplit(base_url)
    host = parts.hostname or "127.0.0.1"
    port = parts.port
    if parts.scheme == "https":
        conn: HTTPConnection | HTTPSConnection = HTTPSConnection(
            host, port=port, timeout=timeout_seconds,
        )
    else:
        conn = HTTPConnection(host, port=port, timeout=timeout_seconds)
    path_prefix = parts.path.rstrip("/")
    return conn, path_prefix


def _stream_anthropic(
    config: ChatProviderConfig,
    messages: list[dict[str, Any]],
    max_tokens: int,
    timeout_seconds: float,
    *,
    tools: list[dict[str, Any]] | None = None,
    system: str | None = None,
) -> Iterator[dict[str, Any]]:
    conn, path_prefix = _open_connection(config.base_url, timeout_seconds=timeout_seconds)
    body_dict: dict[str, Any] = {
        "model": config.model,
        "messages": [m for m in messages if m.get("role") in ("user", "assistant")],
        "stream": True,
        "max_tokens": max_tokens,
    }
    if system:
        body_dict["system"] = system
    if tools:
        body_dict["tools"] = tools
    body = json.dumps(body_dict).encode("utf-8")
    headers = {
        "x-api-key": config.api_key,
        "anthropic-version": "2023-06-01",
        "content-type": "application/json",
        "accept": "text/event-stream",
    }
    # Per-tool-use input accumulators, keyed by content-block index.
    tool_block: dict[int, dict[str, Any]] = {}
    stop_reason = "end_turn"
    try:
        conn.request("POST", path_prefix + "/v1/messages", body=body, headers=headers)
        response = conn.getresponse()
        if response.status >= 400:
            error_body = response.read().decode("utf-8", errors="replace")[:500]
            raise ChatProviderError(
                f"provider returned {response.status} {response.reason}: {error_body}"
            )
        for event in _parse_sse_events(response):
            event_type = event.get("event") or ""
            data = event.get("data") or ""
            if not data:
                continue
            try:
                payload = json.loads(data)
            except json.JSONDecodeError:
                continue
            if event_type == "content_block_start":
                block = payload.get("content_block", {})
                index = int(payload.get("index", 0))
                if block.get("type") == "tool_use":
                    tool_block[index] = {
                        "id": str(block.get("id") or ""),
                        "name": str(block.get("name") or ""),
                        "input_json": "",
                    }
            elif event_type == "content_block_delta":
                index = int(payload.get("index", 0))
                delta = payload.get("delta", {})
                if delta.get("type") == "text_delta":
                    text = str(delta.get("text", ""))
                    if text:
                        yield {"type": "text", "text": text}
                elif delta.get("type") == "input_json_delta" and index in tool_block:
                    tool_block[index]["input_json"] += str(delta.get("partial_json", ""))
            elif event_type == "content_block_stop":
                index = int(payload.get("index", 0))
                if index in tool_block:
                    block = tool_block.pop(index)
                    try:
                        args = json.loads(block["input_json"]) if block["input_json"] else {}
                    except json.JSONDecodeError:
                        args = {}
                    yield {
                        "type": "tool_call",
                        "id": block["id"],
                        "name": block["name"],
                        "args": args,
                    }
            elif event_type == "message_delta":
                delta = payload.get("delta", {})
                if "stop_reason" in delta and delta["stop_reason"]:
                    stop_reason = str(delta["stop_reason"])
            elif event_type == "message_stop":
                yield {"type": "stop", "reason": stop_reason}
                return
        yield {"type": "stop", "reason": stop_reason}
    finally:
        conn.close()


def _stream_openai(
    config: ChatProviderConfig,
    messages: list[dict[str, Any]],
    max_tokens: int,
    timeout_seconds: float,
    *,
    tools: list[dict[str, Any]] | None = None,
    system: str | None = None,
) -> Iterator[dict[str, Any]]:
    conn, path_prefix = _open_connection(config.base_url, timeout_seconds=timeout_seconds)
    out_messages: list[dict[str, Any]] = []
    if system:
        out_messages.append({"role": "system", "content": system})
    out_messages.extend(messages)
    body_dict: dict[str, Any] = {
        "model": config.model,
        "messages": out_messages,
        "stream": True,
        "max_tokens": max_tokens,
    }
    if tools:
        body_dict["tools"] = tools
    body = json.dumps(body_dict).encode("utf-8")
    headers = {
        "authorization": f"Bearer {config.api_key}",
        "content-type": "application/json",
        "accept": "text/event-stream",
    }
    # Per-tool-call accumulators, keyed by tool_calls index.
    tool_calls: dict[int, dict[str, Any]] = {}
    stop_reason = "stop"
    try:
        conn.request(
            "POST", path_prefix + "/v1/chat/completions",
            body=body, headers=headers,
        )
        response = conn.getresponse()
        if response.status >= 400:
            error_body = response.read().decode("utf-8", errors="replace")[:500]
            raise ChatProviderError(
                f"provider returned {response.status} {response.reason}: {error_body}"
            )
        for event in _parse_sse_events(response):
            data = event.get("data") or ""
            if not data:
                continue
            if data.strip() == "[DONE]":
                break
            try:
                payload = json.loads(data)
            except json.JSONDecodeError:
                continue
            choices = payload.get("choices") or []
            if not choices:
                continue
            choice = choices[0]
            delta = choice.get("delta") or {}
            text = str(delta.get("content") or "")
            if text:
                yield {"type": "text", "text": text}
            for tc_delta in delta.get("tool_calls") or []:
                index = int(tc_delta.get("index", 0))
                slot = tool_calls.setdefault(
                    index,
                    {"id": "", "name": "", "args_json": ""},
                )
                if tc_delta.get("id"):
                    slot["id"] = str(tc_delta["id"])
                fn = tc_delta.get("function") or {}
                if fn.get("name"):
                    slot["name"] = str(fn["name"])
                if "arguments" in fn:
                    slot["args_json"] += str(fn.get("arguments") or "")
            finish = choice.get("finish_reason")
            if finish:
                stop_reason = "tool_use" if finish == "tool_calls" else str(finish)
        # Emit accumulated tool calls now that the stream is done.
        for index in sorted(tool_calls.keys()):
            slot = tool_calls[index]
            try:
                args = json.loads(slot["args_json"]) if slot["args_json"] else {}
            except json.JSONDecodeError:
                args = {}
            yield {
                "type": "tool_call",
                "id": slot["id"] or f"call_{index}",
                "name": slot["name"],
                "args": args,
            }
        yield {"type": "stop", "reason": stop_reason}
    finally:
        conn.close()


def _parse_sse_events(response: Any) -> Iterator[dict[str, str]]:
    """Parse a streaming SSE response body into ``{event, data}`` events.

    Minimal SSE parser: line-buffered; ``event:`` and ``data:`` only;
    blank line terminates an event. ``data:`` lines accumulate into a
    single ``\\n``-joined string per event.
    """
    current: dict[str, list[str] | str] = {}
    while True:
        raw = response.readline()
        if not raw:
            return
        if isinstance(raw, bytes):
            line = raw.decode("utf-8", errors="replace")
        else:
            line = raw
        line = line.rstrip("\r\n")
        if line == "":
            if current:
                event = {
                    "event": str(current.get("event") or ""),
                    "data": "\n".join(current.get("data", [])) if isinstance(current.get("data"), list) else str(current.get("data", "")),
                }
                if event.get("event") or event.get("data"):
                    yield event
                current = {}
            continue
        if line.startswith(":"):
            # comment / heartbeat
            continue
        if ":" in line:
            field, _, value = line.partition(":")
            value = value.lstrip(" ")
            if field == "data":
                bucket = current.setdefault("data", [])
                if isinstance(bucket, list):
                    bucket.append(value)
            elif field == "event":
                current["event"] = value

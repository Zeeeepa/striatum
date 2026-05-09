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
    messages: list[dict[str, str]],
    *,
    max_tokens: int = 4096,
    timeout_seconds: float = 60.0,
) -> Iterator[tuple[str, bool]]:
    """Stream the assistant response for the given conversation.

    Yields ``(text_chunk, is_final)`` tuples. ``is_final`` is True on
    the last chunk so the caller can finalize the transcript without
    consuming the iterator a second time.
    """
    if config.flavor == "anthropic_messages":
        yield from _stream_anthropic(config, messages, max_tokens, timeout_seconds)
    elif config.flavor == "openai_chat":
        yield from _stream_openai(config, messages, max_tokens, timeout_seconds)
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
    messages: list[dict[str, str]],
    max_tokens: int,
    timeout_seconds: float,
) -> Iterator[tuple[str, bool]]:
    conn, path_prefix = _open_connection(config.base_url, timeout_seconds=timeout_seconds)
    body = json.dumps(
        {
            "model": config.model,
            "messages": [m for m in messages if m.get("role") in ("user", "assistant")],
            "stream": True,
            "max_tokens": max_tokens,
        }
    ).encode("utf-8")
    headers = {
        "x-api-key": config.api_key,
        "anthropic-version": "2023-06-01",
        "content-type": "application/json",
        "accept": "text/event-stream",
    }
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
            if event_type == "content_block_delta":
                delta = payload.get("delta", {})
                if delta.get("type") == "text_delta":
                    text = str(delta.get("text", ""))
                    if text:
                        yield text, False
            elif event_type == "message_stop":
                yield "", True
                return
        # End of stream without explicit message_stop
        yield "", True
    finally:
        conn.close()


def _stream_openai(
    config: ChatProviderConfig,
    messages: list[dict[str, str]],
    max_tokens: int,
    timeout_seconds: float,
) -> Iterator[tuple[str, bool]]:
    conn, path_prefix = _open_connection(config.base_url, timeout_seconds=timeout_seconds)
    body = json.dumps(
        {
            "model": config.model,
            "messages": messages,
            "stream": True,
            "max_tokens": max_tokens,
        }
    ).encode("utf-8")
    headers = {
        "authorization": f"Bearer {config.api_key}",
        "content-type": "application/json",
        "accept": "text/event-stream",
    }
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
                yield "", True
                return
            try:
                payload = json.loads(data)
            except json.JSONDecodeError:
                continue
            choices = payload.get("choices") or []
            if not choices:
                continue
            delta = choices[0].get("delta") or {}
            text = str(delta.get("content") or "")
            if text:
                yield text, False
        yield "", True
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

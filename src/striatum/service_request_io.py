"""Request body and response helpers for the local service handler."""

from __future__ import annotations

import json
from typing import Any, Callable, Protocol
from urllib.parse import parse_qs

from striatum.service_http import is_json_content_type
from striatum.web.chat_session import parse_simple_multipart

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]

HTML_CONTENT_SECURITY_POLICY = (
    "default-src 'self'; script-src 'self'; style-src 'self'; "
    "img-src 'self' data:; connect-src 'self'"
)


class ResponseWriter(Protocol):
    wfile: Any

    def send_response(self, status: int) -> None: ...

    def send_header(self, name: str, value: str) -> None: ...

    def end_headers(self) -> None: ...


def read_json_body_strict(
    headers: Any,
    rfile: Any,
    send_json: JsonSender,
    *,
    max_bytes: int,
) -> dict[str, Any] | None:
    """Validate JSON Content-Type, cap body size, and parse an object body."""
    if not is_json_content_type(str(headers.get("Content-Type", "") or "")):
        send_json(
            415,
            {"ok": False, "error": {"code": 415, "message": "Content-Type must be application/json"}},
        )
        return None
    try:
        length = int(headers.get("Content-Length") or "0")
    except ValueError:
        length = 0
    if length > max_bytes:
        send_json(413, {"ok": False, "error": {"code": 413, "message": "body too large"}})
        return None
    try:
        raw = rfile.read(length).decode("utf-8", errors="replace") if length > 0 else "{}"
    except OSError as exc:
        send_json(400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
        return None
    try:
        body = json.loads(raw or "{}")
    except json.JSONDecodeError as exc:
        send_json(400, {"ok": False, "error": {"code": 400, "message": f"invalid JSON: {exc}"}})
        return None
    if not isinstance(body, dict):
        send_json(400, {"ok": False, "error": {"code": 400, "message": "body must be a JSON object"}})
        return None
    return body


def read_json_body(headers: Any, rfile: Any, send_json: JsonSender) -> JsonObject | None:
    """Parse the strict JSON object body used by API-style POST handlers."""
    if not is_json_content_type(str(headers.get("Content-Type", "") or "")):
        send_json(
            415,
            {"ok": False, "error": {"code": 415, "message": "Content-Type must be application/json"}},
        )
        return None
    length_header = headers.get("Content-Length")
    if not length_header:
        send_json(400, {"ok": False, "error": {"code": 400, "message": "missing Content-Length"}})
        return None
    try:
        length = int(length_header)
    except ValueError:
        send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid Content-Length"}})
        return None
    raw = rfile.read(length).decode("utf-8")
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError:
        send_json(400, {"ok": False, "error": {"code": 400, "message": "request body must be JSON"}})
        return None
    if not isinstance(parsed, dict):
        send_json(
            400,
            {"ok": False, "error": {"code": 400, "message": "request body must be a JSON object"}},
        )
        return None
    return parsed


def read_form_body(
    headers: Any,
    rfile: Any,
    send_json: JsonSender,
    *,
    max_bytes: int,
) -> dict[str, list[str]] | None:
    try:
        length = int(headers.get("Content-Length") or "0")
    except ValueError:
        length = 0
    if length > max_bytes:
        send_json(413, {"ok": False, "error": {"code": 413, "message": "form body too large"}})
        return None
    if length <= 0:
        return {}
    try:
        raw = rfile.read(length).decode("utf-8", errors="replace")
    except OSError as exc:
        send_json(400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
        return None
    ctype = str(headers.get("Content-Type", "") or "")
    if "multipart/form-data" in ctype:
        return parse_simple_multipart(raw, ctype)
    return parse_qs(raw, keep_blank_values=True)


def send_html_response(handler: ResponseWriter, status: int, body: str) -> None:
    data = body.encode("utf-8")
    handler.send_response(status)
    handler.send_header("Content-Type", "text/html; charset=utf-8")
    handler.send_header("Content-Length", str(len(data)))
    handler.send_header("Content-Security-Policy", HTML_CONTENT_SECURITY_POLICY)
    handler.send_header("Connection", "close")
    handler.end_headers()
    try:
        handler.wfile.write(data)
    except BrokenPipeError:
        return


def send_json_response(
    handler: ResponseWriter,
    status: int,
    payload: JsonObject,
) -> None:
    try:
        body = (json.dumps(payload) + "\n").encode("utf-8")
    except (TypeError, ValueError):
        body = b'{"ok":false,"error":{"code":500,"message":"json encoding failed"}}\n'
    handler.send_response(status)
    handler.send_header("Content-Type", "application/json")
    handler.send_header("Content-Length", str(len(body)))
    handler.send_header("Connection", "close")
    handler.end_headers()
    try:
        handler.wfile.write(body)
    except BrokenPipeError:
        return


__all__ = [
    "HTML_CONTENT_SECURITY_POLICY",
    "read_form_body",
    "read_json_body",
    "read_json_body_strict",
    "send_html_response",
    "send_json_response",
]

from __future__ import annotations

import json
from email.message import Message
from io import BytesIO
from typing import Any

from striatum.service_request_io import (
    HTML_CONTENT_SECURITY_POLICY,
    read_form_body,
    read_json_body,
    read_json_body_strict,
    send_html_response,
    send_json_response,
)


def _headers(**items: str) -> Message:
    headers = Message()
    for key, value in items.items():
        headers[key.replace("_", "-")] = value
    return headers


class _JsonCapture:
    def __init__(self) -> None:
        self.sent: list[tuple[int, dict[str, Any]]] = []

    def send_json(self, status: int, body: dict[str, Any]) -> None:
        self.sent.append((status, body))


def test_read_json_body_requires_exact_json_content_type_and_length() -> None:
    capture = _JsonCapture()

    assert read_json_body(_headers(Content_Type="text/plain"), BytesIO(b"{}"), capture.send_json) is None
    assert capture.sent == [
        (
            415,
            {"ok": False, "error": {"code": 415, "message": "Content-Type must be application/json"}},
        )
    ]

    capture = _JsonCapture()
    assert read_json_body(_headers(Content_Type="application/json"), BytesIO(b"{}"), capture.send_json) is None
    assert capture.sent == [
        (400, {"ok": False, "error": {"code": 400, "message": "missing Content-Length"}})
    ]


def test_read_json_body_parses_only_json_objects() -> None:
    raw = json.dumps({"argv": ["status"]}).encode("utf-8")
    parsed = read_json_body(
        _headers(Content_Type="application/json", Content_Length=str(len(raw))),
        BytesIO(raw),
        _JsonCapture().send_json,
    )

    assert parsed == {"argv": ["status"]}

    capture = _JsonCapture()
    raw_list = b"[]"
    assert read_json_body(
        _headers(Content_Type="application/json", Content_Length=str(len(raw_list))),
        BytesIO(raw_list),
        capture.send_json,
    ) is None
    assert capture.sent == [
        (
            400,
            {
                "ok": False,
                "error": {"code": 400, "message": "request body must be a JSON object"},
            },
        )
    ]


def test_read_json_body_strict_defaults_empty_body_to_object_and_caps_size() -> None:
    capture = _JsonCapture()
    parsed = read_json_body_strict(
        _headers(Content_Type="application/json"),
        BytesIO(b""),
        capture.send_json,
        max_bytes=64,
    )

    assert parsed == {}
    assert capture.sent == []

    capture = _JsonCapture()
    assert read_json_body_strict(
        _headers(Content_Type="application/json", Content_Length="65"),
        BytesIO(b"{}"),
        capture.send_json,
        max_bytes=64,
    ) is None
    assert capture.sent == [
        (413, {"ok": False, "error": {"code": 413, "message": "body too large"}})
    ]


def test_read_form_body_parses_urlencoded_and_caps_size() -> None:
    raw = b"message=hello+there&empty="
    capture = _JsonCapture()

    parsed = read_form_body(
        _headers(
            Content_Type="application/x-www-form-urlencoded",
            Content_Length=str(len(raw)),
        ),
        BytesIO(raw),
        capture.send_json,
        max_bytes=64,
    )

    assert parsed == {"message": ["hello there"], "empty": [""]}
    assert capture.sent == []

    assert read_form_body(
        _headers(Content_Length="65"),
        BytesIO(b""),
        capture.send_json,
        max_bytes=64,
    ) is None
    assert capture.sent == [
        (413, {"ok": False, "error": {"code": 413, "message": "form body too large"}})
    ]


class _ResponseCapture:
    def __init__(self) -> None:
        self.status: int | None = None
        self.headers: dict[str, str] = {}
        self.ended = False
        self.wfile = BytesIO()

    def send_response(self, status: int) -> None:
        self.status = status

    def send_header(self, name: str, value: str) -> None:
        self.headers[name] = value

    def end_headers(self) -> None:
        self.ended = True


def test_send_json_response_falls_back_when_payload_cannot_encode() -> None:
    response = _ResponseCapture()

    send_json_response(response, 200, {"bad": object()})

    assert response.status == 200
    assert response.headers["Content-Type"] == "application/json"
    assert response.ended is True
    assert json.loads(response.wfile.getvalue().decode("utf-8")) == {
        "ok": False,
        "error": {"code": 500, "message": "json encoding failed"},
    }


def test_send_html_response_sets_csp_and_closes_connection() -> None:
    response = _ResponseCapture()

    send_html_response(response, 200, "<main>ok</main>")

    assert response.status == 200
    assert response.headers["Content-Type"] == "text/html; charset=utf-8"
    assert response.headers["Content-Security-Policy"] == HTML_CONTENT_SECURITY_POLICY
    assert response.headers["Connection"] == "close"
    assert response.wfile.getvalue() == b"<main>ok</main>"

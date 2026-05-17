from __future__ import annotations

from io import BytesIO
import json
from pathlib import Path
from typing import Any, Mapping

from striatum.service_daemon import ServiceDaemonRpcError
from striatum.service_sse import encode_sse_event, sse_since, stream_daemon_events


class _SSEWriter:
    def __init__(self) -> None:
        self.wfile = BytesIO()
        self.responses: list[int] = []
        self.headers: dict[str, str] = {}

    def send_response(self, status: int) -> None:
        self.responses.append(status)

    def send_header(self, name: str, value: str) -> None:
        self.headers[name] = value

    def end_headers(self) -> None:
        return None


def test_sse_since_prefers_last_event_id_header() -> None:
    assert sse_since({"Last-Event-ID": "12"}, {"since": ["5"]}) == 12
    assert sse_since({"Last-Event-ID": "-3"}, {"since": ["5"]}) == 0


def test_sse_since_falls_back_to_query() -> None:
    assert sse_since({}, {"since": ["7"]}) == 7
    assert sse_since({"Last-Event-ID": "not-int"}, {"since": ["7"]}) == 7
    assert sse_since({}, {"since": ["not-int"]}) == 0
    assert sse_since({}, {}) == 0


def test_encode_sse_event_frames_json_payload() -> None:
    raw = encode_sse_event("striatum.event", 42, {"run_id": "run_1", "ok": True})
    text = raw.decode("utf-8")

    assert text.startswith("event: striatum.event\nid: 42\ndata: ")
    assert text.endswith("\n\n")
    payload_text = text.split("data: ", 1)[1].strip()
    assert json.loads(payload_text) == {"run_id": "run_1", "ok": True}


def test_stream_daemon_events_emits_rows_and_terminal_event(tmp_path: Path) -> None:
    writer = _SSEWriter()
    calls: list[tuple[Path, str, dict[str, Any]]] = []
    batches: list[dict[str, Any]] = [
        {
            "run": {"run_id": "run_1", "state": "running"},
            "events": [{"event_id": 8, "run_id": "run_1", "event_type": "job.completed"}],
        },
        {"run": {"run_id": "run_1", "state": "completed"}, "events": []},
    ]

    def call_method(repo: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return batches.pop(0)

    sent_json: list[tuple[int, dict[str, Any]]] = []

    stream_daemon_events(
        writer,
        repo=tmp_path,
        run_id="run_1",
        since=7,
        shutting_down=lambda: False,
        send_json=lambda status, body: sent_json.append((status, body)),
        poll_interval_seconds=0,
        call_method=call_method,
    )

    assert calls == [
        (tmp_path, "run.events", {"run_id": "run_1", "since_event_id": 7, "limit": 100}),
        (tmp_path, "run.events", {"run_id": "run_1", "since_event_id": 8, "limit": 100}),
    ]
    assert sent_json == []
    assert writer.responses == [200]
    assert writer.headers["Content-Type"] == "text/event-stream"
    body = writer.wfile.getvalue().decode("utf-8")
    assert "event: striatum.event" in body
    assert '"event_type": "job.completed"' in body
    assert "event: striatum.run_terminal" in body


def test_stream_daemon_events_maps_initial_rpc_error(tmp_path: Path) -> None:
    writer = _SSEWriter()
    sent_json: list[tuple[int, dict[str, Any]]] = []

    def call_method(_repo: Path, _method: str, _params: Mapping[str, Any]) -> dict[str, Any]:
        raise ServiceDaemonRpcError(503, "daemon_unreachable", "daemon down")

    stream_daemon_events(
        writer,
        repo=tmp_path,
        run_id="run_1",
        since=0,
        shutting_down=lambda: False,
        send_json=lambda status, body: sent_json.append((status, body)),
        poll_interval_seconds=0,
        call_method=call_method,
    )

    assert writer.responses == []
    assert writer.wfile.getvalue() == b""
    assert sent_json == [
        (
            503,
            {"ok": False, "error": {"code": "daemon_unreachable", "message": "daemon down"}},
        )
    ]

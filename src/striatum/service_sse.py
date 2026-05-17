"""Small SSE helpers for the local service."""

from __future__ import annotations

import json
import time
from pathlib import Path
from typing import Any, Callable, Mapping, Protocol

from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

JsonObject = dict[str, Any]
JsonSender = Callable[[int, JsonObject], None]
RepoMethodCaller = Callable[[Path, str, Mapping[str, Any]], dict[str, Any]]


class SSEWriter(Protocol):
    wfile: Any

    def send_response(self, status: int) -> None: ...

    def send_header(self, name: str, value: str) -> None: ...

    def end_headers(self) -> None: ...


__all__ = ["encode_sse_event", "sse_since", "stream_daemon_events"]


def sse_since(headers: Mapping[str, str], query: Mapping[str, list[str]]) -> int:
    """Return the SSE replay offset, preferring Last-Event-ID."""
    header = headers.get("Last-Event-ID")
    if header:
        try:
            return max(0, int(header))
        except ValueError:
            pass
    raw = query.get("since", [None])[0]
    if raw:
        try:
            return max(0, int(raw))
        except ValueError:
            return 0
    return 0


def encode_sse_event(event: str, event_id: int, payload: JsonObject) -> bytes:
    body = (
        f"event: {event}\n"
        f"id: {event_id}\n"
        f"data: {json.dumps(payload)}\n\n"
    )
    return body.encode("utf-8")


def stream_daemon_events(
    writer: SSEWriter,
    *,
    repo: Path,
    run_id: str,
    since: int,
    shutting_down: Callable[[], bool],
    send_json: JsonSender,
    poll_interval_seconds: float,
    call_method: RepoMethodCaller = call_repo_method,
) -> None:
    """Stream daemon ``run.events`` rows as browser SSE frames."""
    last_id = since
    try:
        payload = call_method(
            repo,
            "run.events",
            {"run_id": run_id, "since_event_id": last_id, "limit": 100},
        )
    except ServiceDaemonRpcError as exc:
        send_json(
            exc.status,
            {"ok": False, "error": {"code": exc.code, "message": exc.message}},
        )
        return
    try:
        writer.send_response(200)
        writer.send_header("Content-Type", "text/event-stream")
        writer.send_header("Cache-Control", "no-cache")
        writer.send_header("Connection", "close")
        writer.send_header("X-Accel-Buffering", "no")
        writer.end_headers()
        while not shutting_down():
            rows = payload.get("events")
            events = rows if isinstance(rows, list) else []
            for row in events:
                if not isinstance(row, Mapping):
                    continue
                event_id = int(row.get("event_id") or last_id)
                _write_sse_event(writer, "striatum.event", event_id, dict(row))
                last_id = event_id
            run_raw = payload.get("run")
            run: Mapping[str, Any] = run_raw if isinstance(run_raw, Mapping) else {}
            run_state = str(run.get("state") or "")
            if run_state in {"completed", "failed", "canceled"} and not events:
                _write_sse_event(
                    writer,
                    "striatum.run_terminal",
                    last_id,
                    {"run_id": run_id, "state": run_state},
                )
                break
            time.sleep(poll_interval_seconds)
            try:
                payload = call_method(
                    repo,
                    "run.events",
                    {"run_id": run_id, "since_event_id": last_id, "limit": 100},
                )
            except ServiceDaemonRpcError as exc:
                _write_sse_event(
                    writer,
                    "striatum.error",
                    last_id,
                    {"run_id": run_id, "code": exc.code, "message": exc.message},
                )
                break
        if shutting_down():
            _write_sse_event(
                writer,
                "striatum.shutdown",
                last_id,
                {"run_id": run_id, "reason": "service_shutting_down"},
            )
    except (BrokenPipeError, ConnectionResetError):
        return


def _write_sse_event(
    writer: SSEWriter,
    event: str,
    event_id: int,
    payload: JsonObject,
) -> None:
    writer.wfile.write(encode_sse_event(event, event_id, payload))
    writer.wfile.flush()

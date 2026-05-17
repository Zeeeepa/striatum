"""Small SSE helpers for the local service."""

from __future__ import annotations

import json
from typing import Any, Mapping

JsonObject = dict[str, Any]

__all__ = ["encode_sse_event", "sse_since"]


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

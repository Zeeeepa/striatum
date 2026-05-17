from __future__ import annotations

import json

from striatum.service_sse import encode_sse_event, sse_since


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

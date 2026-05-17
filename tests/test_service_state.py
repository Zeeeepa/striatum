from __future__ import annotations

from pathlib import Path

from striatum.service_state import SSE_MAX_CONCURRENT_PER_RUN, ServiceState, utcnow_iso


def test_utcnow_iso_uses_utc_millisecond_shape() -> None:
    value = utcnow_iso()

    assert value.endswith("Z")
    assert "." in value
    assert len(value.rsplit(".", 1)[1]) == 4


def test_service_state_tracks_shutdown(tmp_path: Path) -> None:
    state = ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )

    assert state.shutting_down is False

    state.signal_shutdown()

    assert state.shutting_down is True


def test_service_state_caps_sse_slots_per_run(tmp_path: Path) -> None:
    state = ServiceState(
        repo=tmp_path,
        allow_mutations=False,
        token=None,
        web_enabled=True,
    )

    for _ in range(SSE_MAX_CONCURRENT_PER_RUN):
        assert state.acquire_sse_slot("run-1") is True

    assert state.acquire_sse_slot("run-1") is False

    state.release_sse_slot("run-1")

    assert state.acquire_sse_slot("run-1") is True

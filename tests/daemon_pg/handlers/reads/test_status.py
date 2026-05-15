from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import pytest

from striatum.daemon_pg.handlers.reads import status
from striatum.daemon_pg.handlers.reads import _read_model


pytest_plugins = ("read_handler_fixtures",)

def test_status_empty_repo_shape(empty_ctx: RepoHandlerContext) -> None:
    payload = status.handle(empty_ctx, {})
    assert set(payload) >= {
        "runs",
        "provenance_mode",
        "sessions",
        "jobs",
        "open_blockers",
        "human_checkpoints",
        "latest_non_accepting_review_verdicts",
        "verdicts_by_posture",
        "claimable_jobs",
        "blocked_downstream_jobs",
        "process_health",
        "next_actions",
    }
    assert payload["runs"] == []


def test_status_next_actions_use_legacy_operator_names(
    empty_ctx: RepoHandlerContext, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(_read_model, "claimable_jobs", lambda *_args, **_kwargs: [{"job_id": "job_1"}])
    monkeypatch.setattr(_read_model, "latest_non_accepting_verdicts", lambda *_args, **_kwargs: [])
    monkeypatch.setattr(_read_model, "process_health", lambda *_args, **_kwargs: {"next_actions": []})
    monkeypatch.setattr(_read_model, "has_supervisor_lost_with_held_lease", lambda *_args, **_kwargs: False)
    monkeypatch.setattr(_read_model, "has_stale_leases_with_on_disk_artifacts", lambda *_args, **_kwargs: False)

    payload = status.handle(empty_ctx, {})

    assert "claim_available_work" in payload["next_actions"]
    assert "inspect_packet_with_inbox" in payload["next_actions"]
    assert "claim_next" not in payload["next_actions"]

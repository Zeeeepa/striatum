from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import inspect

import pytest

from striatum.daemon_pg.handlers.reads import run_summary
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_run_summary_requires_path(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        run_summary.handle(empty_ctx, {"run_id": "run_1"})
    assert exc.value.code == "schema_invalid"


def test_run_summary_rejects_path_escape(empty_ctx: RepoHandlerContext, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(run_summary, "_run_row", lambda *_args, **_kwargs: {"run_id": "run_1", "state": "running"})
    monkeypatch.setattr(
        run_summary,
        "run_summary_payload",
        lambda *_args, **_kwargs: {
            "status": {"jobs": {}, "next_actions": []},
            "doctor": {"ok": True},
            "artifacts": [],
            "sessions": [],
            "verdicts_by_workflow_job": [],
            "blockers": [],
            "branch_context": {},
            "timing": {},
        },
    )

    with pytest.raises(RpcError) as exc:
        run_summary.handle(empty_ctx, {"run_id": "run_1", "path": "../RUN_SUMMARY.md"})
    assert exc.value.code == "path_outside_scope"


def test_run_summary_queries_are_repository_scoped() -> None:
    source = inspect.getsource(run_summary)
    assert source.count("repository_id = %s") >= 2
    assert "j.repository_id = v.repository_id" in source


def test_run_summary_payload_uses_pg_doctor_payload(empty_ctx: RepoHandlerContext, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(
        run_summary,
        "_run_row",
        lambda *_args, **_kwargs: {
            "run_id": "run_1",
            "state": "running",
            "branch_name": "main",
            "created_at": None,
            "started_at": None,
            "completed_at": None,
        },
    )
    monkeypatch.setattr(run_summary, "workflow_for_run", lambda *_args, **_kwargs: {})
    monkeypatch.setattr(run_summary, "status_payload", lambda *_args, **_kwargs: {"jobs": {}, "next_actions": []})
    monkeypatch.setattr(run_summary, "doctor_payload", lambda *_args, **_kwargs: {"ok": False, "problems": ["sentinel"]})
    monkeypatch.setattr(run_summary, "evidence_artifact_summaries", lambda *_args, **_kwargs: [])
    monkeypatch.setattr(run_summary, "evidence_session_summaries", lambda *_args, **_kwargs: [])
    monkeypatch.setattr(run_summary, "_verdict_rows", lambda *_args, **_kwargs: [])
    monkeypatch.setattr(run_summary, "blocker_rows", lambda *_args, **_kwargs: [])
    monkeypatch.setattr(run_summary, "current_git_branch", lambda *_args, **_kwargs: "main")

    payload = run_summary.run_summary_payload(empty_ctx, run_id="run_1")

    assert payload["doctor"] == {"ok": False, "problems": ["sentinel"]}

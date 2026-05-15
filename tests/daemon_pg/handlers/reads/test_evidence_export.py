from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import inspect
from pathlib import Path

import pytest

from striatum.daemon_pg.handlers.reads import evidence_export
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_evidence_export_requires_path(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        evidence_export.handle(empty_ctx, {"run_id": "run_1"})
    assert exc.value.code == "schema_invalid"


def test_evidence_export_rejects_path_escape(empty_ctx: RepoHandlerContext, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setattr(
        evidence_export,
        "_run_row",
        lambda *_args, **_kwargs: {"run_id": "run_1", "branch_name": "main", "state": "running"},
    )
    monkeypatch.setattr(evidence_export, "status_payload", lambda *_args, **_kwargs: {"jobs": {}, "next_actions": []})
    monkeypatch.setattr(
        evidence_export,
        "evidence_snapshot",
        lambda *_args, **_kwargs: {
            "schema_version": "striatum.evidence.v1",
            "exported_at": "2026-05-15T00:00:00Z",
        },
    )

    with pytest.raises(RpcError) as exc:
        evidence_export.handle(empty_ctx, {"run_id": "run_1", "path": "../EVIDENCE.md"})
    assert exc.value.code == "path_outside_scope"


def test_evidence_export_uses_pg_doctor_payload(
    empty_ctx: RepoHandlerContext, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setattr(
        evidence_export,
        "_run_row",
        lambda *_args, **_kwargs: {"run_id": "run_1", "branch_name": "main", "state": "running"},
    )
    monkeypatch.setattr(evidence_export, "status_payload", lambda *_args, **_kwargs: {"jobs": {}, "next_actions": []})
    monkeypatch.setattr(evidence_export, "doctor_payload", lambda *_args, **_kwargs: {"ok": False, "problems": ["sentinel"]})
    monkeypatch.setattr(
        evidence_export,
        "evidence_snapshot",
        lambda *_args, **_kwargs: {
            "schema_version": "striatum.evidence.v1",
            "exported_at": "2026-05-15T00:00:00Z",
        },
    )

    result = evidence_export.handle(empty_ctx, {"run_id": "run_1", "path": "EVIDENCE.md"})

    assert result["status"] == "exported"
    assert "sentinel" in (tmp_path / "EVIDENCE.md").read_text(encoding="utf-8")


def test_evidence_export_is_read_only_no_workflow_event_append() -> None:
    source = inspect.getsource(evidence_export)
    assert "append_event" not in source
    assert "INSERT INTO striatumd.events" not in source
    assert "evidence.exported" not in source

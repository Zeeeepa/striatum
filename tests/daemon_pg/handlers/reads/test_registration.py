from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from pathlib import Path

import pytest

import striatum.daemon_pg.handlers  # noqa: F401 - import side effects register handlers
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

METHODS = {
    "status": "status.py",
    "dashboard": "dashboard.py",
    "list.runs": "list_runs.py",
    "list.sessions": "list_sessions.py",
    "list.jobs": "list_jobs.py",
    "list.artifacts": "list_artifacts.py",
    "list.workflows": "list_workflows.py",
    "run.summary": "run_summary.py",
    "why": "why.py",
    "doctor": "doctor.py",
    "evidence.export": "evidence_export.py",
    "corpus.export": "corpus_export.py",
}


@pytest.mark.parametrize("method", sorted(METHODS))
def test_read_method_registered(method: str) -> None:
    assert resolve_pg_handler(method) is not None
    assert getattr(resolve_pg_handler(method), "read_only", False) is True


@pytest.mark.parametrize("method,filename", sorted(METHODS.items()))
def test_handlers_use_locked_read_only_decorator(method: str, filename: str) -> None:
    source = (Path("src/striatum/daemon_pg/handlers/reads") / filename).read_text()
    assert f'@register_pg_handler("{method}", read_only=True)' in source


@pytest.mark.parametrize("filename", sorted(METHODS.values()))
def test_handlers_keep_grep_visible_repository_scope(filename: str) -> None:
    source = (Path("src/striatum/daemon_pg/handlers/reads") / filename).read_text()
    if filename in {"dashboard.py", "status.py", "list_artifacts.py"}:
        source += Path("src/striatum/daemon_pg/handlers/reads/_read_model.py").read_text()
    assert "repository_id = %s" in source or ".repository_id = %s" in source


def test_bad_params_raise_rpc_error(empty_ctx: RepoHandlerContext) -> None:
    handler = resolve_pg_handler("why")
    assert handler is not None
    with pytest.raises(RpcError) as exc:
        handler(empty_ctx, {})
    assert exc.value.code == "schema_invalid"

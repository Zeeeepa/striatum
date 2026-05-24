from __future__ import annotations

import inspect

import pytest

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.reads import corpus_export
from striatum.daemon_pg.handlers.reads._sql import safe_output_path
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_corpus_export_requires_out(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        corpus_export.handle(empty_ctx, {"since": "HEAD"})
    assert exc.value.code == "schema_invalid"


def test_corpus_export_rejects_output_escape(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        safe_output_path(empty_ctx, "../corpus")
    assert exc.value.code == "path_outside_scope"


def test_corpus_export_queries_are_repository_scoped_and_pg_backed() -> None:
    source = inspect.getsource(corpus_export)

    assert source.count("repository_id = %s") >= 2
    assert "events_for" in source
    assert '"substrate": "postgresql"' in source
    assert "<daemon-postgres>" in source

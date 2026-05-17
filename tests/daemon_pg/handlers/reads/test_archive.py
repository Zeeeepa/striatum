from __future__ import annotations

import inspect

import pytest

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.reads import archive
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)


def test_archive_create_requires_out(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        archive.handle(empty_ctx, {"run_id": "run_1"})
    assert exc.value.code == "schema_invalid"


def test_archive_create_rejects_output_escape(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        archive.handle(empty_ctx, {"run_id": "run_1", "out": "../archive"})
    assert exc.value.code == "path_outside_scope"


def test_archive_handler_queries_are_repository_scoped_and_pg_backed() -> None:
    source = inspect.getsource(archive)
    assert source.count("repository_id = %s") >= 4
    assert '("events", "events"' in source
    assert '("artifacts", "artifacts"' in source
    assert "sqlite3" not in source
    assert "connect(" not in source

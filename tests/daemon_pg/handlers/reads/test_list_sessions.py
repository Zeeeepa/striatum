from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import pytest

from striatum.daemon_pg.handlers.reads import list_sessions
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_list_sessions_requires_run_id(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        list_sessions.handle(empty_ctx, {})
    assert exc.value.code == "schema_invalid"

from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import pytest

from striatum.daemon_pg.handlers.reads import why
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_why_requires_target_id(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        why.handle(empty_ctx, {})
    assert exc.value.code == "schema_invalid"

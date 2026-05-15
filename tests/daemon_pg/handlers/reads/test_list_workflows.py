from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import pytest

from striatum.daemon_pg.handlers.reads import list_workflows
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_list_workflows_empty_envelope(empty_ctx: RepoHandlerContext) -> None:
    assert list_workflows.handle(empty_ctx, {"limit": 10}) == {"items": [], "count": 0}


def test_list_workflows_rejects_bad_limit(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        list_workflows.handle(empty_ctx, {"limit": 0})
    assert exc.value.code == "schema_invalid"

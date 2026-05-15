from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import pytest

from striatum.daemon_pg.handlers.reads import list_runs
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_list_runs_empty_envelope(empty_ctx: RepoHandlerContext) -> None:
    assert list_runs.handle(empty_ctx, {"limit": 10}) == {"items": [], "count": 0}


def test_list_runs_rejects_bad_state(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        list_runs.handle(empty_ctx, {"state": "bogus"})
    assert exc.value.code == "schema_invalid"

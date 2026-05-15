from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import pytest

from striatum.daemon_pg.handlers.reads import list_artifacts
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_list_artifacts_rejects_bad_kind(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        list_artifacts.handle(empty_ctx, {"run_id": "run_1", "kind": "bogus"})
    assert exc.value.code == "schema_invalid"

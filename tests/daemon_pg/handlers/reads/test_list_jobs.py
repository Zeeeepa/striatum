from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import pytest

from striatum.daemon_pg.handlers.reads import list_jobs
from striatum.daemon_rpc.envelope import RpcError


pytest_plugins = ("read_handler_fixtures",)

def test_list_jobs_rejects_bad_state(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        list_jobs.handle(empty_ctx, {"run_id": "run_1", "state": "bogus"})
    assert exc.value.code == "schema_invalid"

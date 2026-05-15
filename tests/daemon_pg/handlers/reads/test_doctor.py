from __future__ import annotations

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.reads import doctor


pytest_plugins = ("read_handler_fixtures",)

def test_doctor_empty_repo_shape(empty_ctx: RepoHandlerContext) -> None:
    assert doctor.handle(empty_ctx, {"verbose": True}) == {
        "ok": True,
        "schema_version": 5,
        "problems": [],
        "problem_records": [],
    }

from __future__ import annotations

from pathlib import Path

import pytest

from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.capability import RpcAuthContext


class EmptyCursor:
    def __enter__(self) -> "EmptyCursor":
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def execute(self, *_args: object) -> None:
        return None

    def fetchone(self) -> None:
        return None

    def fetchall(self) -> list[dict[str, object]]:
        return []


class EmptyConnection:
    def cursor(self, *args: object, **kwargs: object) -> EmptyCursor:
        return EmptyCursor()


@pytest.fixture
def empty_ctx(tmp_path: Path) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=EmptyConnection(),
        repository_id="repo_a",
        repo_root=tmp_path,
        auth=RpcAuthContext("client", "token", "repo_a", "read", "allowed"),
    )

from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest

from _harness import pg
from _harness.tokens import issue_token
from striatum.daemon_pg import client_admin as daemon
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.mcp_resources import dashboard_all_resource_pg
from striatum.daemon_rpc.envelope import RpcEnvelope
from striatum.daemon_rpc.server import DaemonRpcRouter
from striatum.errors import SchemaVersionError, StriatumError


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = pg.create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        pg.drop_ephemeral_database(ephemeral)


@pytest.fixture
def pg_conn(pg_url: str) -> Iterator[Any]:
    conn = connect(pg_url)
    conn.autocommit = True
    try:
        yield conn
    finally:
        conn.close()


def test_repo_add_registers_pg_without_creating_sqlite(
    tmp_path: Path,
    pg_url: str,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    monkeypatch.setenv("STRIATUM_DAEMON_DB_URL", pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")

    added = daemon.repo_add(repo, init=True)

    assert str(added["repository_id"]).startswith("repo_")
    assert added["state"] == "active"
    assert added["state_db_path"] == str((repo / ".striatum").resolve())
    assert "bootstrap_admin" in added
    assert (repo / ".striatum" / "scratch").is_dir()
    assert not (repo / ".striatum" / "state.sqlite3").exists()
    assert ".striatum/" in (repo / ".gitignore").read_text(encoding="utf-8").splitlines()

    listed = daemon.repo_list()
    assert [row["repository_id"] for row in listed["repositories"]] == [added["repository_id"]]

    duplicate = daemon.repo_add(repo / ".")
    assert duplicate["already_registered"] is True
    assert duplicate["repository_id"] == added["repository_id"]


def test_repo_add_refuses_existing_sqlite_source_without_opening_it(
    tmp_path: Path,
    pg_url: str,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = tmp_path / "repo"
    state_dir = repo / ".striatum"
    state_dir.mkdir(parents=True)
    (state_dir / "state.sqlite3").write_bytes(b"not a sqlite database")
    monkeypatch.setenv("STRIATUM_DAEMON_DB_URL", pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")

    with pytest.raises(SchemaVersionError, match="SQLite import windows are closed"):
        daemon.repo_add(repo)


def test_repo_remove_marks_pg_row_and_revokes_repo_scoped_capabilities(
    tmp_path: Path,
    pg_url: str,
    pg_conn: Any,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    monkeypatch.setenv("STRIATUM_DAEMON_DB_URL", pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))

    added = daemon.repo_add(repo, init=True)
    repository_id = str(added["repository_id"])
    issue_token(pg_conn, capabilities=["read"], repo_id=repository_id)

    removed = daemon.repo_remove(repository_id)

    assert removed == {
        "repository_id": repository_id,
        "state": "removed",
        "revoked_capabilities": 1,
    }
    with pg_conn.cursor() as cur:
        cur.execute(
            "SELECT state FROM striatumd.repositories WHERE repository_id = %s",
            (repository_id,),
        )
        assert cur.fetchone()[0] == "removed"
        cur.execute(
            """
            SELECT revoked_reason
            FROM striatumd.client_capabilities
            WHERE repository_id = %s
            """,
            (repository_id,),
        )
        assert cur.fetchone()[0] == "repo_removed"

    readded = daemon.repo_add(repo)
    assert readded["repository_id"] != repository_id


def test_repo_remove_revoked_repo_scoped_token_cannot_read_dashboard_all(
    tmp_path: Path,
    pg_url: str,
    pg_conn: Any,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    monkeypatch.setenv("STRIATUM_DAEMON_DB_URL", pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))

    added = daemon.repo_add(repo, init=True)
    repository_id = str(added["repository_id"])
    scoped_read_token = issue_token(pg_conn, capabilities=["read"], repo_id=repository_id)

    removed = daemon.repo_remove(repository_id)

    assert removed["revoked_capabilities"] == 1
    with pytest.raises(PermissionError, match="capability"):
        dashboard_all_resource_pg(pg_conn, token=scoped_read_token)


def test_repo_add_refuses_symlink_paths_and_traversal(
    tmp_path: Path,
    pg_url: str,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    real_parent = tmp_path / "real"
    repo = real_parent / "repo"
    repo.mkdir(parents=True)
    linked_repo = tmp_path / "repo-link"
    linked_repo.symlink_to(repo, target_is_directory=True)
    linked_parent = tmp_path / "linked-parent"
    linked_parent.symlink_to(real_parent, target_is_directory=True)
    monkeypatch.setenv("STRIATUM_DAEMON_DB_URL", pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))

    with pytest.raises(StriatumError, match="symlink paths"):
        daemon.repo_add(linked_repo, init=True)
    with pytest.raises(StriatumError, match="symlink paths"):
        daemon.repo_add(linked_parent / "repo", init=True)
    with pytest.raises(StriatumError, match="path traversal"):
        daemon.repo_add(tmp_path / "real" / ".." / "real" / "repo", init=True)


def test_repo_rpc_routes_are_pg_native_and_authorized(
    tmp_path: Path,
    pg_conn: Any,
) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    admin_token = issue_token(pg_conn, capabilities=["admin", "read"], repo_id=None)
    router = DaemonRpcRouter(pg_conn=pg_conn, repo_root=tmp_path)

    add_response = router.handle(
        RpcEnvelope(
            schema_version=1,
            request_id="repo-add",
            method="repo.add",
            params={"path": str(repo), "init": True},
            capability_token=admin_token,
        ),
        require_handshake=False,
    )

    assert add_response.ok is True
    repository_id = str(add_response.data["repository_id"])
    assert repository_id.startswith("repo_")
    assert not (repo / ".striatum" / "state.sqlite3").exists()
    scoped_read_token = issue_token(pg_conn, capabilities=["read"], repo_id=repository_id)

    list_response = router.handle(
        RpcEnvelope(
            schema_version=1,
            request_id="repo-list",
            method="repo.list",
            params={},
            capability_token=admin_token,
        ),
        require_handshake=False,
    )
    assert list_response.ok is True
    assert [row["repository_id"] for row in list_response.data["repositories"]] == [repository_id]

    scoped_denied = router.handle(
        RpcEnvelope(
            schema_version=1,
            request_id="repo-list-scoped-denied",
            method="repo.list",
            params={},
            capability_token=scoped_read_token,
        ),
        require_handshake=False,
    )
    assert scoped_denied.ok is False
    assert scoped_denied.data["code"] == "capability_missing"

    remove_response = router.handle(
        RpcEnvelope(
            schema_version=1,
            request_id="repo-remove",
            method="repo.remove",
            params={"id": repository_id},
            capability_token=admin_token,
        ),
        require_handshake=False,
    )
    assert remove_response.ok is True
    assert remove_response.data["state"] == "removed"

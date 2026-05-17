from __future__ import annotations

from pathlib import Path

import pytest

from _harness.multi_repo import MultiRepoHarness

pytestmark = pytest.mark.multi_repo


def test_start_registers_two_repos_with_distinct_sqlite_state(multi_repo_harness: MultiRepoHarness) -> None:
    harness = multi_repo_harness

    assert len(harness.repos) == 2
    assert harness.repos[0].repository_id != harness.repos[1].repository_id
    assert harness.socket_path.exists()
    assert all((repo.path / ".striatum" / "state.sqlite3").exists() for repo in harness.repos)


def test_reset_daemon_db_preserves_schema_metadata_and_clears_data(
    multi_repo_harness: MultiRepoHarness,
) -> None:
    harness = multi_repo_harness

    harness.reset_daemon_db()

    assert harness.daemon_db_query("SELECT value FROM striatumd.schema_meta WHERE key = 'substrate_version'")
    # Schema_migrations grows as we land migrations. Pin against the live
    # constant rather than a literal so the assertion survives each new
    # migration (0001-0006 currently land via apply_migrations).
    from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION
    assert harness.daemon_db_query("SELECT COUNT(*) AS count FROM striatumd.schema_migrations")[0]["count"] == LATEST_DAEMON_DB_VERSION
    assert harness.daemon_db_query("SELECT COUNT(*) AS count FROM striatumd.repositories")[0]["count"] == 0
    assert harness.daemon_db_query("SELECT COUNT(*) AS count FROM striatumd.audit_segments")[0]["count"] == 1
    assert harness.daemon_db_query("SELECT last_hash FROM striatumd.audit_chain_head")[0]["last_hash"] is None


def test_register_all_after_reset_restores_repository_rows(multi_repo_harness: MultiRepoHarness) -> None:
    harness = multi_repo_harness
    harness.reset_daemon_db()

    repo_ids = harness.register_all()

    assert len(repo_ids) == 2
    rows = harness.daemon_db_query("SELECT repository_id FROM striatumd.repositories")
    assert {row["repository_id"] for row in rows} == set(repo_ids)


def test_stop_drops_ephemeral_database_removes_scratch_and_socket(
    tmp_path: Path,
    postgres_url: str,
) -> None:
    scratch = tmp_path / "scratch"
    harness = MultiRepoHarness(daemon_pg_url=postgres_url, scratch_dir=scratch)
    harness.start()
    database_url = harness.daemon_pg_url
    socket = harness.socket_path

    harness.stop()

    assert not scratch.exists()
    assert not socket.exists()
    with pytest.raises(Exception):
        harness.__class__(daemon_pg_url=database_url, scratch_dir=tmp_path / "unused").pg_conn()


def test_start_stop_start_has_no_socket_or_database_collision(tmp_path: Path, postgres_url: str) -> None:
    first = MultiRepoHarness(daemon_pg_url=postgres_url, scratch_dir=tmp_path / "first")
    first.start()
    first.stop()

    second = MultiRepoHarness(daemon_pg_url=postgres_url, scratch_dir=tmp_path / "second")
    second.start()
    try:
        assert second.socket_path.exists()
        assert len(second.repos) == 2
    finally:
        second.stop()


def test_reset_reseeds_empty_audit_chain(multi_repo_harness: MultiRepoHarness) -> None:
    harness = multi_repo_harness
    harness.reset_daemon_db()
    harness.register_all()
    token = harness.issue_token(["read"], repo_id=str(harness.repos[0].repository_id))

    client = harness.mcp_client(token)
    client.call_tool("status", repository_id=str(harness.repos[0].repository_id))
    assert harness.audit_rows()

    harness.reset_daemon_db()

    assert harness.audit_rows() == []
    assert harness.daemon_db_query("SELECT last_hash FROM striatumd.audit_chain_head")[0]["last_hash"] is None

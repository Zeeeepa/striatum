from __future__ import annotations

import argparse
import shutil
import sqlite3
from pathlib import Path
from typing import Iterator

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.cli.daemon import dispatch_daemon
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION, MIGRATIONS
from striatum.daemon_pg.repo_local_migration import (
    RepoLocalMigrationOptions,
    compute_repo_local_reanchor,
    migrate_repo_local,
)
from striatum.errors import SchemaVersionError, StriatumError
from striatum.migrations import LATEST_VERSION


ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "tests" / "fixtures" / "v1_repo_local_sqlite" / "state.sqlite3"


def _repo_with_fixture(tmp_path: Path) -> Path:
    repo = tmp_path / "repo"
    state_dir = repo / ".striatum"
    state_dir.mkdir(parents=True)
    shutil.copy2(FIXTURE, state_dir / "state.sqlite3")
    return repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_repo_local_migration_registered_as_daemon_pg_v5() -> None:
    # Migration 0006 lands the events chain anchors on top of 0005; the
    # repo-local workflow state migration that this test pins remains 0005
    # by version+resource. LATEST_DAEMON_DB_VERSION is now 6 (the chain
    # anchor migration); this test asserts that 0005 stays the locked
    # repo-local schema, not that LATEST is 5.
    assert LATEST_DAEMON_DB_VERSION >= 5
    sql_0005 = next(m for m in MIGRATIONS if m.version == 5).sql
    for table in (
        "workflow_snapshots",
        "runs",
        "sessions",
        "jobs",
        "job_dependencies",
        "queue_messages",
        "leases",
        "work_packets",
        "artifacts",
        "verdicts",
        "blockers",
        "command_requests",
        "process_executions",
        "events",
        "job_worktrees",
        "process_supervisors",
        "process_supervisor_pointers",
        "repo_migrations",
    ):
        assert f"striatumd.{table}" in sql_0005
    assert "repository_id text NOT NULL" in sql_0005
    assert "refuse_repo_append_only_change" in sql_0005


@pytest.mark.multi_repo
def test_dry_run_reports_counts_and_does_not_write(tmp_path: Path, pg_url: str) -> None:
    repo = _repo_with_fixture(tmp_path)

    result = migrate_repo_local(
        RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url, dry_run=True)
    )

    assert result["dry_run"] is True
    assert result["source_user_version"] == LATEST_VERSION
    assert result["source_counts"]["events"] == 1
    assert result["source_counts"]["artifacts"] == 1
    assert (repo / ".striatum" / "state.sqlite3").exists()
    conn = connect(pg_url)
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT COUNT(*) FROM striatumd.workflow_snapshots")
            assert cur.fetchone()[0] == 0
            cur.execute("SELECT COUNT(*) FROM striatumd.repo_migrations")
            assert cur.fetchone()[0] == 0
    finally:
        conn.close()


@pytest.mark.multi_repo
def test_full_run_copies_rows_checkpoints_and_tombstones(tmp_path: Path, pg_url: str) -> None:
    repo = _repo_with_fixture(tmp_path)

    result = migrate_repo_local(RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url))

    repository_id = str(result["repository_id"])
    assert result["already_migrated"] is False
    assert result["imported_counts"]["events"] == 1
    tombstone = repo / ".striatum" / "state.sqlite3.tombstone"
    assert tombstone.exists()
    assert not (repo / ".striatum" / "state.sqlite3").exists()
    assert tombstone.stat().st_mode & 0o777 == 0o444

    conn = connect(pg_url)
    sqlite_conn = sqlite3.connect(f"file:{tombstone}?mode=ro", uri=True)
    sqlite_conn.row_factory = sqlite3.Row
    try:
        reanchor = compute_repo_local_reanchor(sqlite_conn, conn, repository_id)
        assert reanchor["source_events_sha256"] == reanchor["destination_events_sha256"]
        assert reanchor["source_artifacts_sha256"] == reanchor["destination_artifacts_sha256"]
        with conn.cursor() as cur:
            cur.execute(
                "SELECT COUNT(*) FROM striatumd.repo_migrations WHERE repository_id = %s",
                (repository_id,),
            )
            assert cur.fetchone()[0] == 1
            cur.execute(
                "SELECT COUNT(*) FROM striatumd.events WHERE repository_id = %s",
                (repository_id,),
            )
            assert cur.fetchone()[0] == 1
    finally:
        sqlite_conn.close()
        conn.close()

    rerun = migrate_repo_local(RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url))
    assert rerun["already_migrated"] is True
    assert rerun["repository_id"] == repository_id


def test_delete_path_requires_confirm_delete(tmp_path: Path) -> None:
    repo = _repo_with_fixture(tmp_path)

    with pytest.raises(StriatumError, match="--confirm-delete"):
        migrate_repo_local(
            RepoLocalMigrationOptions(
                repo=repo,
                postgres_url="postgresql://unused/db",
                keep_sqlite_readonly=False,
            )
        )


@pytest.mark.multi_repo
def test_confirm_delete_removes_sqlite_after_commit(tmp_path: Path, pg_url: str) -> None:
    repo = _repo_with_fixture(tmp_path)

    result = migrate_repo_local(
        RepoLocalMigrationOptions(
            repo=repo,
            postgres_url=pg_url,
            keep_sqlite_readonly=False,
            confirm_delete=True,
        )
    )

    assert result["sqlite_finalization"]["action"] == "delete"
    assert not (repo / ".striatum" / "state.sqlite3").exists()
    assert not (repo / ".striatum" / "state.sqlite3.tombstone").exists()


@pytest.mark.multi_repo
def test_old_sqlite_user_version_is_refused(tmp_path: Path, pg_url: str) -> None:
    repo = _repo_with_fixture(tmp_path)
    conn = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    conn.execute(f"PRAGMA user_version = {LATEST_VERSION - 1}")
    conn.close()

    with pytest.raises(SchemaVersionError):
        migrate_repo_local(
            RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url, dry_run=True)
        )


@pytest.mark.multi_repo
def test_cli_daemon_helper_dispatches_migrate_repo_local(tmp_path: Path, pg_url: str) -> None:
    repo = _repo_with_fixture(tmp_path)
    args = argparse.Namespace(
        daemon_command="migrate-repo-local",
        from_substrate="sqlite",
        to_substrate="pg",
        repo_local_repo=str(repo),
        postgres_url=pg_url,
        dry_run=True,
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )

    result = dispatch_daemon(args)

    assert result["mode"] == "repo_local_migration"
    assert result["dry_run"] is True

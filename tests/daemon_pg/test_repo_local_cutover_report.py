"""Repository-specific cutover verification report coverage."""

from __future__ import annotations

import json
import shutil
from pathlib import Path
from typing import Iterator

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg import repo_cutover_report as report_mod
from striatum.daemon_pg import repo_local_migration as migration_mod
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.repo_cutover_report import (
    RepoCutoverReportOptions,
    verify_repo_cutover,
)
from striatum.daemon_pg.repo_local_migration import (
    RepoLocalMigrationOptions,
    migrate_repo_local,
)
from striatum.daemon_rpc.capability import RpcAuthContext


pytestmark = [
    pytest.mark.multi_repo,
    pytest.mark.usefixtures("legacy_sqlite_import_enabled"),
]

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


def _fail_sqlite_connect(*_args: object, **_kwargs: object) -> None:
    raise AssertionError("verify-cutover must not open SQLite as a database")


def test_verify_cutover_reports_tombstoned_migration_without_opening_sqlite(
    tmp_path: Path,
    pg_url: str,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = _repo_with_fixture(tmp_path)
    migrate_repo_local(RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url))
    sqlite_module = getattr(migration_mod, "sqlite3")
    monkeypatch.setattr(sqlite_module, "connect", _fail_sqlite_connect)
    assert not hasattr(report_mod, "sqlite3")

    report = verify_repo_cutover(RepoCutoverReportOptions(repo=repo, postgres_url=pg_url))

    assert report["schema_version"] == "striatum.repo_cutover_report.v1"
    assert report["ok"] is True
    assert report["repository"]["registered"] is True
    assert report["checkpoint"]["present"] is True
    assert report["destination_counts"]["not_below_checkpoint"] is True
    assert report["sqlite_finalization"]["status"] == "tombstoned"
    assert report["sqlite_finalization"]["source_state_db_absent"] is True
    assert report["sqlite_finalization"]["sentinel_absent"] is True
    assert report["sqlite_finalization"]["tombstone"]["mode"] == "0444"
    assert report["sqlite_finalization"]["tombstone_sha256_matches_checkpoint"] is True
    assert report["event_chain"]["status"] == "legacy_unanchored"
    assert report["event_chain"]["legacy_unanchored_event_count"] == 1
    assert any(
        note["scope"] == "migration_source_import"
        for note in report["sqlite_exceptions"]
    )


def test_verify_cutover_diagnoses_incomplete_finalization_without_opening_sqlite(
    tmp_path: Path,
    pg_url: str,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = _repo_with_fixture(tmp_path)

    def crashing_tombstone(repo_arg: Path, **_kwargs: object) -> dict[str, object]:
        assert repo_arg == repo.resolve()
        raise RuntimeError("simulated crash before finalization")

    monkeypatch.setattr(
        migration_mod,
        "_tombstone_or_delete_state_db",
        crashing_tombstone,
    )
    with pytest.raises(RuntimeError):
        migrate_repo_local(RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url))
    sqlite_module = getattr(migration_mod, "sqlite3")
    monkeypatch.setattr(sqlite_module, "connect", _fail_sqlite_connect)
    assert not hasattr(report_mod, "sqlite3")

    report = verify_repo_cutover(RepoCutoverReportOptions(repo=repo, postgres_url=pg_url))

    assert report["ok"] is False
    finalization = report["sqlite_finalization"]
    assert finalization["status"] == "incomplete_finalization_after_checkpoint"
    assert finalization["source_state_db_absent"] is False
    assert finalization["sentinel_absent"] is False
    assert finalization["source_sha256_matches_checkpoint"] is True
    assert "state.sqlite3 is still present" in finalization["diagnosis"][0]


def test_verify_cutover_flags_destination_counts_below_checkpoint(
    tmp_path: Path,
    pg_url: str,
) -> None:
    repo = _repo_with_fixture(tmp_path)
    result = migrate_repo_local(RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url))
    conn = connect(pg_url)
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.repo_migrations
                SET row_counts = row_counts || %s::jsonb
                WHERE repository_id = %s
                """,
                (json.dumps({"events": 2}), result["repository_id"]),
            )
        conn.commit()
    finally:
        conn.close()

    report = verify_repo_cutover(RepoCutoverReportOptions(repo=repo, postgres_url=pg_url))

    assert report["ok"] is False
    assert report["destination_counts"]["not_below_checkpoint"] is False
    assert report["destination_counts"]["violations"] == [
        {"table": "events", "checkpoint_count": 2, "actual_count": 1}
    ]


def test_verify_cutover_reports_mixed_event_chain_after_post_cutover_event(
    tmp_path: Path,
    pg_url: str,
) -> None:
    repo = _repo_with_fixture(tmp_path)
    result = migrate_repo_local(RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url))
    repository_id = str(result["repository_id"])
    conn = connect(pg_url)
    try:
        ctx = RepoHandlerContext(
            pg_conn=conn,
            repository_id=repository_id,
            repo_root=repo,
            auth=RpcAuthContext(None, None, repository_id, None, "allowed"),
        )
        event_id = ctx.append_event(
            run_id=None,
            event_type="test.post_cutover",
            payload={"phase": "post_cutover"},
        )
        conn.commit()
    finally:
        conn.close()

    report = verify_repo_cutover(RepoCutoverReportOptions(repo=repo, postgres_url=pg_url))

    chain = report["event_chain"]
    assert chain["ok"] is True
    assert chain["status"] == "mixed_legacy_and_anchored"
    assert chain["anchored_event_count"] == 1
    assert chain["legacy_unanchored_event_count"] == 1
    assert chain["head"]["last_event_id"] == event_id
    assert chain["latest_anchored_event"]["event_id"] == event_id

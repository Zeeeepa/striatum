"""RFC 0043 V1.5 F-crash regression coverage.

Exercises the sentinel-based crash-resume path in
``src/striatum/daemon_pg/repo_local_migration.py`` — the migration must
finish the SQLite tombstone idempotently when the post-commit
finalization is interrupted.
"""

from __future__ import annotations

import shutil
from pathlib import Path
from typing import Iterator

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg import repo_local_migration as migration_mod
from striatum.daemon_pg.repo_local_migration import (
    RepoLocalMigrationOptions,
    _read_sentinel,
    _resume_sqlite_finalization_after_checkpoint,
    _write_sentinel_atomically,
    migrate_repo_local,
)
from striatum.errors import StriatumError


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


# --- pure-Python coverage of the resume helper -----------------------


def test_sentinel_roundtrip_writes_atomically_and_reads_back(tmp_path: Path) -> None:
    sentinel = tmp_path / ".striatum" / "state.sqlite3.migrated"
    _write_sentinel_atomically(
        sentinel,
        repository_id="repo_unit_test",
        source_state_db_sha256="abc",
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )
    payload = _read_sentinel(sentinel)
    assert payload is not None
    assert payload["repository_id"] == "repo_unit_test"
    assert payload["source_state_db_sha256"] == "abc"
    assert payload["keep_sqlite_readonly"] is True
    assert payload["confirm_delete"] is False
    # The atomic write must not leave the temp file behind.
    assert not (sentinel.parent / (sentinel.name + ".tmp")).exists()


def test_resume_finalizes_tombstone_when_source_matches(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    state_dir = repo / ".striatum"
    state_dir.mkdir(parents=True)
    source = state_dir / "state.sqlite3"
    source.write_bytes(b"sqlite-bytes-v1")
    expected_sha = migration_mod._file_sha256(source)
    sentinel = state_dir / "state.sqlite3.migrated"
    _write_sentinel_atomically(
        sentinel,
        repository_id="repo_resume_demo",
        source_state_db_sha256=expected_sha,
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )

    result = _resume_sqlite_finalization_after_checkpoint(
        repo,
        source_path=source,
        sentinel_path=sentinel,
        checkpoint={"source_state_db_sha256": expected_sha},
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )

    assert result is not None
    assert result["action"] == "tombstone"
    assert result["resumed_from_checkpoint"] is True
    tombstone = state_dir / "state.sqlite3.tombstone"
    assert tombstone.exists()
    assert tombstone.stat().st_mode & 0o777 == 0o444
    assert not source.exists()
    assert not sentinel.exists()


def test_resume_refuses_on_sha_mismatch_without_deleting(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    state_dir = repo / ".striatum"
    state_dir.mkdir(parents=True)
    source = state_dir / "state.sqlite3"
    source.write_bytes(b"corrupted-bytes")
    sentinel = state_dir / "state.sqlite3.migrated"
    _write_sentinel_atomically(
        sentinel,
        repository_id="repo_resume_demo",
        source_state_db_sha256="expected-sha-from-checkpoint",
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )

    with pytest.raises(StriatumError) as exc:
        _resume_sqlite_finalization_after_checkpoint(
            repo,
            source_path=source,
            sentinel_path=sentinel,
            checkpoint={"source_state_db_sha256": "expected-sha-from-checkpoint"},
            keep_sqlite_readonly=True,
            confirm_delete=False,
        )

    assert exc.value.exit_code == 8
    assert "changed since the Postgres checkpoint" in str(exc.value)
    # Non-destructive: both the source and the sentinel are intact.
    assert source.exists()
    assert sentinel.exists()
    assert not (state_dir / "state.sqlite3.tombstone").exists()


def test_resume_clears_orphan_sentinel(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    state_dir = repo / ".striatum"
    state_dir.mkdir(parents=True)
    sentinel = state_dir / "state.sqlite3.migrated"
    _write_sentinel_atomically(
        sentinel,
        repository_id="repo_resume_orphan",
        source_state_db_sha256="ignored",
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )
    source = state_dir / "state.sqlite3"
    assert not source.exists()

    result = _resume_sqlite_finalization_after_checkpoint(
        repo,
        source_path=source,
        sentinel_path=sentinel,
        checkpoint={"source_state_db_sha256": "ignored"},
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )

    assert result is not None
    assert result["action"] == "cleared_orphan_sentinel"
    assert not sentinel.exists()


def test_resume_returns_none_when_already_finalized(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    state_dir = repo / ".striatum"
    state_dir.mkdir(parents=True)
    sentinel = state_dir / "state.sqlite3.migrated"
    source = state_dir / "state.sqlite3"

    result = _resume_sqlite_finalization_after_checkpoint(
        repo,
        source_path=source,
        sentinel_path=sentinel,
        checkpoint={"source_state_db_sha256": "anything"},
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )

    assert result is None


# --- end-to-end crash-resume against a real Postgres ------------------


@pytest.mark.multi_repo
def test_rerun_after_crashed_tombstone_finishes_finalization(
    tmp_path: Path,
    pg_url: str,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """F-crash regression: the un-fixed code leaves a stranded SQLite
    source after a post-commit crash. The V1.5 sentinel-based resume
    completes the tombstone idempotently on the next run.
    """
    repo = _repo_with_fixture(tmp_path)
    source = repo / ".striatum" / "state.sqlite3"
    sentinel = repo / ".striatum" / "state.sqlite3.migrated"

    class CrashError(RuntimeError):
        pass

    original_tombstone = migration_mod._tombstone_or_delete_state_db
    crash_calls: list[Path] = []

    def crashing_tombstone(repo_arg: Path, **kwargs: object) -> dict[str, object]:
        crash_calls.append(repo_arg)
        raise CrashError("simulated post-commit crash mid-finalization")

    monkeypatch.setattr(
        migration_mod, "_tombstone_or_delete_state_db", crashing_tombstone
    )

    with pytest.raises(CrashError):
        migrate_repo_local(
            RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url)
        )

    assert crash_calls, "the simulated crash should have been triggered"
    # After the crash the source is still on disk and the sentinel remains.
    assert source.exists()
    assert sentinel.exists()
    sentinel_payload = _read_sentinel(sentinel)
    assert sentinel_payload is not None
    assert sentinel_payload["keep_sqlite_readonly"] is True

    # Restore the real helper before the resume attempt.
    monkeypatch.setattr(
        migration_mod, "_tombstone_or_delete_state_db", original_tombstone
    )

    rerun = migrate_repo_local(
        RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url)
    )

    assert rerun["already_migrated"] is True
    finalization = rerun["sqlite_finalization"]
    assert finalization["action"] == "tombstone"
    assert finalization["resumed_from_checkpoint"] is True
    tombstone = repo / ".striatum" / "state.sqlite3.tombstone"
    assert tombstone.exists()
    assert tombstone.stat().st_mode & 0o777 == 0o444
    assert not source.exists()
    assert not sentinel.exists()


@pytest.mark.multi_repo
def test_rerun_with_corrupted_source_refuses_with_exit_code_8(
    tmp_path: Path,
    pg_url: str,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Corrupting the source between the Postgres commit and the resume
    must yield a non-destructive exit-code-8 refusal rather than
    silently tombstoning the wrong bytes.
    """
    repo = _repo_with_fixture(tmp_path)
    source = repo / ".striatum" / "state.sqlite3"
    sentinel = repo / ".striatum" / "state.sqlite3.migrated"

    def crashing_tombstone(repo_arg: Path, **kwargs: object) -> dict[str, object]:
        raise RuntimeError("simulated crash before tombstone")

    monkeypatch.setattr(
        migration_mod, "_tombstone_or_delete_state_db", crashing_tombstone
    )
    with pytest.raises(RuntimeError):
        migrate_repo_local(
            RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url)
        )

    # Drop the monkeypatch so the rerun can call into the real helper.
    monkeypatch.undo()
    # Tamper with the source after the checkpoint commit.
    source.write_bytes(b"corrupted-after-checkpoint")

    with pytest.raises(StriatumError) as exc:
        migrate_repo_local(
            RepoLocalMigrationOptions(repo=repo, postgres_url=pg_url)
        )

    assert exc.value.exit_code == 8
    assert "changed since the Postgres checkpoint" in str(exc.value)
    # Non-destructive: source + sentinel are still on disk, no tombstone.
    assert source.exists()
    assert sentinel.exists()
    assert not (repo / ".striatum" / "state.sqlite3.tombstone").exists()

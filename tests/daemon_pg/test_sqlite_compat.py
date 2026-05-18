from __future__ import annotations

from pathlib import Path

from striatum.daemon_pg.sqlite_compat import repo_identity


def test_repo_identity_matches_legacy_shape(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    state_dir = repo / ".striatum"
    state_dir.mkdir(parents=True)
    (state_dir / "state.sqlite3").write_bytes(b"sqlite")

    repo_stat = repo.stat()
    state_stat = (state_dir / "state.sqlite3").stat()

    assert repo_identity(repo) == (
        f"inode:{repo_stat.st_dev}:{repo_stat.st_ino}:"
        f"state:{state_stat.st_dev}:{state_stat.st_ino}"
    )

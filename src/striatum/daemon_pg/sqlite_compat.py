"""SQLite-era repository identity helper retained for migration fixtures."""

from __future__ import annotations

from pathlib import Path

from striatum.repo_policy import db_path


def repo_identity(repo: Path) -> str:
    """Return the pre-D094 daemon registry identity for a repo-local SQLite DB."""
    repo_stat = repo.stat()
    state_stat = db_path(repo).stat()
    return (
        f"inode:{repo_stat.st_dev}:{repo_stat.st_ino}:"
        f"state:{state_stat.st_dev}:{state_stat.st_ino}"
    )

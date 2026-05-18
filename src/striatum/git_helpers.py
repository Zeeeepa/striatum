"""Small substrate-neutral Git helpers."""

from __future__ import annotations

import subprocess
from pathlib import Path


def current_git_branch(repo: Path) -> str | None:
    """Return the current Git branch when detectable."""
    result = subprocess.run(
        ["git", "branch", "--show-current"],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    branch = result.stdout.strip()
    if result.returncode != 0 or branch == "":
        return None
    return branch

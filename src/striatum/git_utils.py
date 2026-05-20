"""Git utility helpers for Striatum."""

from __future__ import annotations

import subprocess
from pathlib import Path


def short_git_status(repo: Path) -> str:
    """Return a compact multi-line summary of git status (up to 80 lines)."""
    try:
        proc = subprocess.run(
            ["git", "status", "--short"],
            cwd=repo,
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    lines = (proc.stdout or "").splitlines()
    if len(lines) > 80:
        lines = lines[:80] + [f"... ({len(lines) - 80} more lines)"]
    return "\n".join(lines)

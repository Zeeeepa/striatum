"""Day-zero filesystem bootstrap helpers.

These helpers create only operational scratch next to a target repository.
Authoritative workflow state lives in the daemon-owned PostgreSQL instance.
"""

from __future__ import annotations

import os
from importlib import resources
from pathlib import Path
from typing import Callable

from striatum.errors import EXIT_DAEMON_CAPABILITY, StriatumError


class OperationalScratchError(StriatumError):
    """Raised when the target repo's operational scratch is unsafe."""

    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_DAEMON_CAPABILITY)


def init_operational_scratch(
    repo: Path,
    *,
    error_factory: Callable[[str], Exception] = OperationalScratchError,
) -> Path:
    """Create `.striatum/` operational scratch without creating state storage."""

    state_dir = repo / ".striatum"
    if _has_symlink_component(state_dir) or state_dir.is_symlink():
        raise error_factory("repo scratch directory symlink is not allowed")
    state_dir.mkdir(parents=True, exist_ok=True)
    scratch = state_dir / "scratch"
    scratch.mkdir(parents=True, exist_ok=True)
    for path in (state_dir, scratch):
        try:
            os.chmod(path, 0o700)
        except PermissionError:
            pass
    _ensure_gitignore_entry(repo)
    _copy_reference_wrappers(state_dir)
    return state_dir


def _copy_reference_wrappers(state_dir: Path) -> None:
    bin_dir = state_dir / "bin"
    bin_dir.mkdir(parents=True, exist_ok=True)
    try:
        os.chmod(bin_dir, 0o700)
    except PermissionError:
        pass

    try:
        files = resources.files("striatum.scaffold.templates.bin")
        for file in files.iterdir():
            if file.is_file() and file.name.endswith(".sh"):
                dest = bin_dir / file.name
                dest.write_bytes(file.read_bytes())
                try:
                    os.chmod(dest, 0o755)
                except PermissionError:
                    pass
    except Exception:
        pass


def require_operational_scratch(
    repo: Path,
    *,
    error_factory: Callable[[str], Exception] = OperationalScratchError,
) -> Path:
    """Return the existing operational scratch directory or raise."""

    state_dir = repo / ".striatum"
    if _has_symlink_component(state_dir) or state_dir.is_symlink():
        raise error_factory("repo scratch directory symlink is not allowed")
    if not state_dir.is_dir():
        raise error_factory(
            "repo scratch is not initialized; rerun repo add with --init or run striatum init first"
        )
    return state_dir


def _ensure_gitignore_entry(repo: Path) -> None:
    ignore_path = repo / ".gitignore"
    existing = ignore_path.read_text(encoding="utf-8") if ignore_path.exists() else ""
    if ".striatum/" not in existing.splitlines():
        prefix = "" if existing == "" or existing.endswith("\n") else "\n"
        ignore_path.write_text(f"{existing}{prefix}.striatum/\n", encoding="utf-8")


def _has_symlink_component(path: Path) -> bool:
    current = Path(path.anchor) if path.is_absolute() else Path()
    for part in path.parts:
        if part == path.anchor:
            continue
        current = current / part
        try:
            if current.is_symlink():
                return True
        except OSError:
            return True
    return False

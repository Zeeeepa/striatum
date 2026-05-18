"""Daemon runtime paths and token-file helpers.

This module is intentionally independent of the legacy Python daemon so
Python CLI clients and the Go daemon launcher can share runtime discovery
without importing SQLite-backed daemon registry code.
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

from striatum.errors import EXIT_DAEMON_AUTH, StriatumError

ENV_RUNTIME = "STRIATUM_DAEMON_RUNTIME_DIR"


class DaemonRuntimeTokenError(StriatumError):
    def __init__(self, message: str) -> None:
        super().__init__(message, exit_code=EXIT_DAEMON_AUTH)


def runtime_dir() -> Path:
    override = os.environ.get(ENV_RUNTIME)
    if override:
        return Path(override).expanduser().resolve()
    if sys.platform == "darwin":
        return Path.home() / "Library" / "Caches" / "striatum" / "runtime"
    base = os.environ.get("XDG_RUNTIME_DIR")
    if base:
        return Path(base) / "striatum"
    return Path.home() / ".cache" / "striatum" / "runtime"


def token_file() -> Path:
    return runtime_dir() / "client-token"


def socket_path() -> Path:
    return runtime_dir() / "striatumd.sock"


def pid_path() -> Path:
    return runtime_dir() / "striatumd.pid"


def ensure_private_dir(path: Path) -> None:
    path.mkdir(parents=True, exist_ok=True)
    try:
        os.chmod(path, 0o700)
    except PermissionError:
        pass


def read_runtime_token() -> str | None:
    path = token_file()
    if not path.exists():
        return None
    mode = path.stat().st_mode & 0o777
    if mode & 0o077:
        raise DaemonRuntimeTokenError("daemon token fallback file is not owner-only")
    return path.read_text(encoding="utf-8").strip() or None


def write_runtime_token(token: str) -> None:
    path = token_file()
    ensure_private_dir(path.parent)
    path.write_text(token + "\n", encoding="utf-8")
    try:
        os.chmod(path, 0o600)
    except PermissionError:
        pass

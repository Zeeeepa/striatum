"""Legacy SQLite daemon-registry wrappers for paired test fixtures only."""

from __future__ import annotations

from pathlib import Path
from typing import Any


def repo_add(
    path: Path,
    *,
    display_name: str | None = None,
    no_migrate: bool = False,
    init: bool = False,
) -> dict[str, Any]:
    from striatum import daemon

    return daemon.repo_add(path, display_name=display_name, no_migrate=no_migrate, init=init)


def repo_list() -> dict[str, Any]:
    from striatum import daemon

    return daemon.repo_list()


def repo_remove(identifier: str) -> dict[str, Any]:
    from striatum import daemon

    return daemon.repo_remove(identifier)


def read_status(repo: Path, *, run_id: str | None) -> dict[str, Any]:
    from striatum import daemon

    return daemon.read_status(repo, run_id=run_id)


def read_why(repo: Path, *, target_id: str) -> dict[str, Any]:
    from striatum import daemon

    return daemon.read_why(repo, target_id=target_id)


def read_doctor(
    repo: Path | None,
    *,
    run_id: str | None,
    verbose: bool,
    postgres_url: str | None = None,
) -> dict[str, Any]:
    from striatum import daemon

    _ = postgres_url
    return daemon.read_doctor(repo, run_id=run_id, verbose=verbose)


def dashboard_all() -> dict[str, Any]:
    from striatum import daemon

    return daemon.dashboard_all()


def daemon_status() -> dict[str, Any]:
    from striatum import daemon

    return daemon.daemon_status()


def daemon_stop() -> dict[str, Any]:
    from striatum import daemon

    return daemon.daemon_stop()


def health() -> dict[str, Any]:
    from striatum import daemon

    return daemon.health()


def daemon_audit(*, limit: int = 100) -> dict[str, Any]:
    from striatum import daemon

    return daemon.daemon_audit(limit=limit)


def daemon_sweep_once(*, require_client_auth: bool = False) -> dict[str, Any]:
    from striatum import daemon

    return daemon.daemon_sweep_once(require_client_auth=require_client_auth)

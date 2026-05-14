"""Daemon command helpers that are independent of top-level dispatch wiring."""

from __future__ import annotations

import argparse
import os
import shutil
from pathlib import Path
from typing import Any

from striatum import daemon as daemon_mod
from striatum.daemon_pg.config import resolve_config
from striatum.daemon_pg.repo_local_migration import (
    RepoLocalMigrationOptions,
    migrate_repo_local,
)
from striatum.errors import StriatumError

ENV_DAEMON_CORE = "STRIATUM_DAEMON_CORE"
ENV_GO_BIN = "STRIATUMD_GO_BIN"
VALID_DAEMON_CORES = frozenset({"python", "go"})


def dispatch_daemon(args: argparse.Namespace) -> Any:
    """Dispatch daemon subcommands owned by the daemon CLI slice."""
    if getattr(args, "daemon_command", None) == "migrate-repo-local":
        if args.from_substrate != "sqlite" or args.to_substrate != "pg":
            raise StriatumError("migrate-repo-local supports only --from sqlite --to pg", exit_code=2)
        config = resolve_config(postgres_url=getattr(args, "postgres_url", None))
        if config.url is None:
            raise StriatumError("daemon PostgreSQL URL is not configured", exit_code=13)
        repo_arg = getattr(args, "repo_local_repo", None) or getattr(args, "repo", None)
        if not repo_arg:
            raise StriatumError("migrate-repo-local requires --repo", exit_code=2)
        return migrate_repo_local(
            RepoLocalMigrationOptions(
                repo=Path(repo_arg),
                postgres_url=config.url,
                dry_run=bool(getattr(args, "dry_run", False)),
                keep_sqlite_readonly=bool(getattr(args, "keep_sqlite_readonly", True)),
                confirm_delete=bool(getattr(args, "confirm_delete", False)),
            )
        )
    raise StriatumError("unknown daemon command", exit_code=2)


def resolve_daemon_core(cli_value: str | None) -> str:
    """Resolve the daemon core while preserving the Phase 2 python default."""
    value = cli_value or os.environ.get(ENV_DAEMON_CORE) or "python"
    if value not in VALID_DAEMON_CORES:
        raise StriatumError(
            f"unknown daemon core {value!r}; expected python or go",
            exit_code=2,
        )
    return value


def run_python_daemon_foreground(args: argparse.Namespace) -> Any:
    return daemon_mod.run_daemon_foreground(
        sweep_interval_seconds=float(args.sweep_interval_seconds),
        max_sweeps=args.max_sweeps,
        postgres_url=getattr(args, "postgres_url", None),
    )


def run_go_daemon_foreground(*, postgres_url: str | None = None) -> Any:
    binary = resolve_go_binary()
    command = [str(binary)]
    socket_path = daemon_mod.socket_path()
    command.extend(["--socket", str(socket_path)])
    if postgres_url:
        command.extend(["--postgres-url", postgres_url])
    os.execv(str(binary), command)


def launch_daemon_start(args: argparse.Namespace) -> Any:
    core = resolve_daemon_core(getattr(args, "core", None))
    if core == "python":
        return run_python_daemon_foreground(args)
    return run_go_daemon_foreground(postgres_url=getattr(args, "postgres_url", None))


def resolve_go_binary() -> Path:
    packaged = _resolve_packaged_go_binary()
    if packaged is not None:
        return packaged
    override = os.environ.get(ENV_GO_BIN)
    if override:
        binary = Path(override).expanduser().resolve()
        if not binary.exists():
            raise StriatumError(f"{ENV_GO_BIN}={override} does not exist", exit_code=2)
        return binary
    repo_binary = Path(__file__).resolve().parents[3] / "go" / "bin" / "striatumd"
    if repo_binary.exists():
        return repo_binary
    path_binary = shutil.which("striatumd-go") or shutil.which("striatumd")
    if path_binary:
        return Path(path_binary).resolve()
    raise StriatumError(
        "Go daemon binary not found; set STRIATUMD_GO_BIN or build go/bin/striatumd",
        exit_code=2,
    )


def _resolve_packaged_go_binary() -> Path | None:
    try:
        from striatum import _daemongo
    except Exception:
        return None
    for name in ("resolve_binary", "binary_path", "path"):
        resolver = getattr(_daemongo, name, None)
        if resolver is None:
            continue
        try:
            value = resolver() if callable(resolver) else resolver
        except Exception:
            continue
        if value:
            path = Path(value).expanduser().resolve()
            if path.exists():
                return path
    return None

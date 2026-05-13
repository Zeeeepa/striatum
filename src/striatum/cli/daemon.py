"""Daemon command helpers that are independent of top-level dispatch wiring."""

from __future__ import annotations

import argparse
from pathlib import Path
from typing import Any

from striatum.daemon_pg.config import resolve_config
from striatum.daemon_pg.repo_local_migration import (
    RepoLocalMigrationOptions,
    migrate_repo_local,
)
from striatum.errors import StriatumError


def dispatch_daemon(args: argparse.Namespace) -> Any:
    """Dispatch daemon subcommands owned by the daemon CLI slice."""
    if getattr(args, "daemon_command", None) == "migrate-repo-local":
        if args.from_substrate != "sqlite" or args.to_substrate != "pg":
            raise StriatumError("migrate-repo-local supports only --from sqlite --to pg", exit_code=2)
        config = resolve_config(postgres_url=getattr(args, "postgres_url", None))
        if config.url is None:
            raise StriatumError("daemon PostgreSQL URL is not configured", exit_code=13)
        return migrate_repo_local(
            RepoLocalMigrationOptions(
                repo=Path(args.repo_local_repo),
                postgres_url=config.url,
                dry_run=bool(getattr(args, "dry_run", False)),
                keep_sqlite_readonly=bool(getattr(args, "keep_sqlite_readonly", True)),
                confirm_delete=bool(getattr(args, "confirm_delete", False)),
            )
        )
    raise StriatumError("unknown daemon command", exit_code=2)

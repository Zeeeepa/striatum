"""Daemon command helpers that are independent of top-level dispatch wiring."""

from __future__ import annotations

import argparse
import os
import subprocess
import shutil
from pathlib import Path
from typing import Any

from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION
from striatum.daemon_rpc.registry import METHODS_ETAG
from striatum.daemon_runtime import socket_path
from striatum.daemon_pg.config import resolve_config
from striatum.daemon_pg.repo_local_migration import (
    RepoLocalMigrationOptions,
    migrate_repo_local,
    verify_repo_cutover,
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
        options = RepoLocalMigrationOptions(
            repo=Path(repo_arg),
            postgres_url=config.url,
            dry_run=bool(getattr(args, "dry_run", False)),
            keep_sqlite_readonly=bool(getattr(args, "keep_sqlite_readonly", True)),
            confirm_delete=bool(getattr(args, "confirm_delete", False)),
        )
        if bool(getattr(args, "verify_cutover", False)):
            return verify_repo_cutover(options)
        return migrate_repo_local(options)
    raise StriatumError("unknown daemon command", exit_code=2)


def resolve_daemon_core(cli_value: str | None) -> str:
    """Resolve the daemon core; Go is the production default after RFC 0068 parity."""
    value = cli_value or os.environ.get(ENV_DAEMON_CORE) or "go"
    if value not in VALID_DAEMON_CORES:
        raise StriatumError(
            f"unknown daemon core {value!r}; expected python or go",
            exit_code=2,
        )
    return value


def run_python_daemon_foreground(args: argparse.Namespace) -> Any:
    from striatum import daemon as daemon_mod

    return daemon_mod.run_daemon_foreground(
        sweep_interval_seconds=float(args.sweep_interval_seconds),
        max_sweeps=args.max_sweeps,
        postgres_url=getattr(args, "postgres_url", None),
    )


def run_go_daemon_foreground(
    *,
    postgres_url: str | None = None,
    sweep_interval_seconds: float = 60.0,
    max_sweeps: int | None = None,
) -> Any:
    binary = resolve_go_binary()
    command = [str(binary)]
    command.extend(["--socket", str(socket_path())])
    if postgres_url:
        command.extend(["--postgres-url", postgres_url])
    command.extend(["--sweep-interval-seconds", str(float(sweep_interval_seconds))])
    if max_sweeps is not None:
        command.extend(["--max-sweeps", str(max_sweeps)])
    command.extend(["--migrations-sha-source", str(resolve_migrations_sha_source())])
    os.execv(str(binary), command)


def launch_daemon_start(args: argparse.Namespace) -> Any:
    core = resolve_daemon_core(getattr(args, "core", None))
    if core == "python":
        return run_python_daemon_foreground(args)
    return run_go_daemon_foreground(
        postgres_url=getattr(args, "postgres_url", None),
        sweep_interval_seconds=float(args.sweep_interval_seconds),
        max_sweeps=args.max_sweeps,
    )


def resolve_go_binary() -> Path:
    packaged = _resolve_packaged_go_binary()
    if packaged is not None:
        return _verify_go_binary_contract(packaged)
    override = os.environ.get(ENV_GO_BIN)
    if override:
        binary = Path(override).expanduser().resolve()
        if not binary.exists():
            raise StriatumError(f"{ENV_GO_BIN}={override} does not exist", exit_code=2)
        return _verify_go_binary_contract(binary)
    repo_binary = Path(__file__).resolve().parents[3] / "go" / "bin" / "striatumd"
    if repo_binary.exists():
        return _verify_go_binary_contract(repo_binary)
    path_binary = shutil.which("striatumd-go") or shutil.which("striatumd")
    if path_binary:
        return _verify_go_binary_contract(Path(path_binary).resolve())
    raise StriatumError(
        "Go daemon binary not found; set STRIATUMD_GO_BIN or build go/bin/striatumd",
        exit_code=2,
    )


def resolve_migrations_sha_source() -> Path:
    return Path(__file__).resolve().parents[1] / "daemon_pg" / "sql"


def _resolve_packaged_go_binary() -> Path | None:
    try:
        from striatum import _daemongo
    except Exception:
        return None
    for name in ("find_binary", "resolve_binary", "binary_path", "path"):
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


def _verify_go_binary_contract(binary: Path) -> Path:
    """Reject stale Go daemon binaries before they can bind a socket."""

    try:
        result = subprocess.run(
            [str(binary), "--describe"],
            check=False,
            capture_output=True,
            text=True,
            timeout=5,
        )
    except (OSError, subprocess.SubprocessError) as exc:
        raise StriatumError(
            f"Go daemon binary {binary} cannot self-describe; rebuild go/bin/striatumd: {exc}",
            exit_code=2,
        ) from exc
    if result.returncode != 0:
        detail = (result.stderr or result.stdout).strip()
        raise StriatumError(
            f"Go daemon binary {binary} failed --describe; rebuild go/bin/striatumd"
            + (f": {detail}" if detail else ""),
            exit_code=2,
        )
    fields = _parse_go_describe(result.stdout)
    expected_schema = str(LATEST_DAEMON_DB_VERSION)
    if fields.get("supported_schema") != expected_schema:
        raise StriatumError(
            f"Go daemon binary {binary} supports schema {fields.get('supported_schema') or 'unknown'}; "
            f"expected {expected_schema}. Rebuild go/bin/striatumd.",
            exit_code=2,
        )
    if fields.get("migration_count") != expected_schema:
        raise StriatumError(
            f"Go daemon binary {binary} embeds {fields.get('migration_count') or 'unknown'} migrations; "
            f"expected {expected_schema}. Rebuild go/bin/striatumd.",
            exit_code=2,
        )
    if fields.get("methods_etag") != METHODS_ETAG:
        raise StriatumError(
            f"Go daemon binary {binary} has method contract {fields.get('methods_etag') or 'unknown'}; "
            f"expected {METHODS_ETAG}. Regenerate and rebuild the Go daemon.",
            exit_code=2,
        )
    return binary


def _parse_go_describe(output: str) -> dict[str, str]:
    fields: dict[str, str] = {}
    for part in output.split():
        if "=" not in part:
            continue
        key, value = part.split("=", 1)
        fields[key] = value
    return fields

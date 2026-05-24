"""Daemon command helpers that are independent of top-level dispatch wiring."""

from __future__ import annotations

import argparse
import os
import subprocess
import shutil
from pathlib import Path
from typing import Any

from striatum import __version__ as STRIATUM_VERSION
from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION
from striatum.daemon_rpc.registry import METHODS_ETAG
from striatum.daemon_runtime import socket_path
from striatum.errors import StriatumError

ENV_GO_BIN = "STRIATUMD_GO_BIN"
RETIRED_SQLITE_IMPORT_MESSAGE = (
    "Retired import windows are closed; Striatum no longer opens legacy "
    "daemon registries or repo-local .striatum/state.sqlite3 files "
    "for migration. Archive or remove the retired local-state files and register "
    "the repository with `striatum adopt` or `striatum repo add --init`. "
    "Use an older Striatum release in an isolated environment only for "
    "historical one-time export."
)


def dispatch_daemon(args: argparse.Namespace) -> Any:
    """Dispatch daemon subcommands owned by the daemon CLI slice."""
    if getattr(args, "daemon_command", None) in {"migrate", "migrate-repo-local"}:
        raise StriatumError(RETIRED_SQLITE_IMPORT_MESSAGE, exit_code=12)
    raise StriatumError("unknown daemon command", exit_code=2)


def run_go_daemon_foreground(
    *,
    postgres_url: str | None = None,
    mcp_http_addr: str | None = None,
    sweep_interval_seconds: float = 60.0,
    max_sweeps: int | None = None,
) -> Any:
    binary = resolve_go_binary()
    command = [str(binary)]
    command.extend(["--socket", str(socket_path())])
    if postgres_url:
        command.extend(["--postgres-url", postgres_url])
    if mcp_http_addr:
        command.extend(["--mcp-http-addr", mcp_http_addr])
    command.extend(["--sweep-interval-seconds", str(float(sweep_interval_seconds))])
    if max_sweeps is not None:
        command.extend(["--max-sweeps", str(max_sweeps)])
    command.extend(["--migrations-sha-source", str(resolve_migrations_sha_source())])
    os.execv(str(binary), command)


def launch_daemon_start(args: argparse.Namespace) -> Any:
    return run_go_daemon_foreground(
        postgres_url=getattr(args, "postgres_url", None),
        mcp_http_addr=getattr(args, "mcp_http_addr", None),
        sweep_interval_seconds=float(args.sweep_interval_seconds),
        max_sweeps=args.max_sweeps,
    )


def resolve_go_binary() -> Path:
    override = os.environ.get(ENV_GO_BIN)
    if override:
        binary = Path(override).expanduser().resolve()
        if not binary.exists():
            raise StriatumError(f"{ENV_GO_BIN}={override} does not exist", exit_code=2)
        return _verify_go_binary_contract(binary)
    packaged = _resolve_packaged_go_binary()
    if packaged is not None:
        return _verify_go_binary_contract(packaged)
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
    if fields.get("daemon_version") != STRIATUM_VERSION:
        raise StriatumError(
            f"Go daemon binary {binary} reports version {fields.get('daemon_version') or 'unknown'}; "
            f"expected {STRIATUM_VERSION}. Rebuild go/bin/striatumd.",
            exit_code=2,
        )
    if fields.get("git_sha") in {None, "", "unknown"}:
        raise StriatumError(
            f"Go daemon binary {binary} does not report a git SHA; rebuild go/bin/striatumd.",
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

"""Command read/mutation policy for the local service invoke gate."""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

SERVICE_CLI_LOCAL_READ_SUBCOMMANDS: dict[str, frozenset[str]] = {
    "workflow": frozenset({"validate", "lint", "plan", "graph", "templates"}),
}


@dataclass(frozen=True)
class DaemonCommandRoute:
    method: str
    params: dict[str, object]


def is_read_command(argv: list[str]) -> bool:
    """Return True when ``argv`` resolves to a known read-only command.

    Daemon-routed commands are classified by the daemon method contract's
    required capability. CLI-local file-authoring reads keep a small explicit
    list because they intentionally are not daemon RPC methods.
    """
    if not argv:
        return False
    method = _daemon_method_for_argv(argv)
    if method is not None:
        from striatum.daemon_rpc.registry import METHOD_REGISTRY

        entry = METHOD_REGISTRY.get(method)
        return entry is not None and entry.required_capability == "read"
    top = argv[0]
    if top in SERVICE_CLI_LOCAL_READ_SUBCOMMANDS and len(argv) >= 2:
        return argv[1] in SERVICE_CLI_LOCAL_READ_SUBCOMMANDS[top]
    return False


def daemon_route_for_argv(argv: list[str], repo: Path) -> DaemonCommandRoute | None:
    """Return the daemon RPC route for production service invoke argv.

    The test-harness direct path is deliberately preserved for legacy
    SQLite-backed service tests. Production service invocations route mapped
    daemon methods through RPC instead of re-entering local CLI dispatch through
    ``striatum.api.invoke``.
    """

    if (
        os.environ.get("STRIATUM_TEST_HARNESS") == "1"
        and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"
    ):
        return None
    route = _daemon_route_for_argv(argv, repo)
    if route is None:
        return None
    return route


def daemon_mutation_route_for_argv(argv: list[str], repo: Path) -> DaemonCommandRoute | None:
    """Return the daemon RPC route for production mutation-shaped invoke argv."""

    route = daemon_route_for_argv(argv, repo)
    if route is None:
        return None
    from striatum.daemon_rpc.registry import METHOD_REGISTRY

    entry = METHOD_REGISTRY.get(route.method)
    if entry is None or entry.required_capability == "read":
        return None
    return route


def _daemon_method_for_argv(argv: list[str]) -> str | None:
    route = _daemon_route_for_argv(argv, Path("."))
    if route is None:
        return None
    return route.method


def _daemon_route_for_argv(argv: list[str], repo: Path) -> DaemonCommandRoute | None:
    import contextlib
    import io

    from striatum.cli import build_parser
    from striatum.cli.daemon_rpc_route import _LOOKUP, _subcommand

    parser = build_parser()
    try:
        with contextlib.redirect_stderr(io.StringIO()):
            args = parser.parse_args(argv)
    except SystemExit:
        return None
    translator = _LOOKUP.get((args.command, _subcommand(args)))
    if translator is None:
        translator = _LOOKUP.get((args.command, None))
    if translator is None:
        return None
    try:
        method, params = translator(args, repo)
    except Exception:  # noqa: BLE001 - malformed args are not read commands.
        return None
    from striatum.daemon_rpc.registry import METHOD_REGISTRY

    if method in METHOD_REGISTRY:
        return DaemonCommandRoute(method=method, params=dict(params))
    return None

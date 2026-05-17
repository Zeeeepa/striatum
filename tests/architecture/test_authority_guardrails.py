from __future__ import annotations

import importlib
from pathlib import Path
from typing import Any

import pytest

from striatum.cli.dispatch import dispatch
from striatum.cli.parser import build_parser
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError
from striatum.daemon_rpc.registry import METHOD_REGISTRY
from striatum.daemon_rpc.server import CLI_ROUTES, LOCAL_FILE_AUTHORING_METHODS, DaemonRpcRouter
from striatum.errors import DaemonUnreachableError, StriatumError


DIRECT_DAEMON_METHODS: frozenset[str] = frozenset(
    {
        "daemon.hello",
        "daemon.describe",
        "dashboard.all",
        "cross_repo.list",
        "cross_repo.describe",
        "cross_repo.why",
        "cross_repo.cancel",
        "apply.reviewed_patch",
        "apply.receipt.show",
        "apply.receipt.verify",
    }
)

BOOTSTRAP_OR_MIGRATION_METHODS: frozenset[str] = frozenset(
    {
        "repo.add",
        "repo.remove",
        "repo.list",
        "repo.resolve",
        "repo.init",
        "daemon.token.create",
        "daemon.token.revoke",
        "daemon.token.rotate",
        "daemon.key.rotate",
        "daemon.shutdown",
        "daemon.migrate",
        "daemon.migrate_repo_local",
    }
)

NOT_IMPLEMENTED_METHODS: frozenset[str] = frozenset(
    {
        "workflow.generate.preview",
    }
)

LOCAL_FILE_AUTHORING_METHODS_EXPECTED: frozenset[str] = frozenset(
    {
        "workflow.validate",
        "workflow.plan",
        "workflow.graph",
        "workflow.templates.list",
        "workflow.templates.show",
        "workflow.init",
        "workflow.generate",
        "workflow.upgrade",
    }
)

DOGFOOD_SQLITE_METHODS: frozenset[str] = frozenset(
    {
        "dogfood.publish_on_behalf",
        "dogfood.surgical_recovery",
    }
)


def _pg_handlers() -> set[str]:
    import striatum.daemon_pg.handlers  # noqa: F401 - registers decorators.
    from striatum.daemon_pg.handlers.registry import resolve_pg_handler

    return {
        method
        for method in METHOD_REGISTRY
        if resolve_pg_handler(method) is not None
    }


def test_registry_methods_have_explicit_authority_path() -> None:
    pg_handlers = _pg_handlers()
    unclassified: list[str] = []
    for method, entry in sorted(METHOD_REGISTRY.items()):
        if entry.deprecated:
            continue
        if method in pg_handlers:
            continue
        if method in DIRECT_DAEMON_METHODS:
            continue
        if method in BOOTSTRAP_OR_MIGRATION_METHODS:
            continue
        if method in NOT_IMPLEMENTED_METHODS:
            continue
        if method in LOCAL_FILE_AUTHORING_METHODS:
            continue
        if method in DOGFOOD_SQLITE_METHODS:
            continue
        unclassified.append(method)

    assert not unclassified, (
        "daemon RPC methods lack an explicit authority classification: "
        + ", ".join(unclassified)
    )


def test_daemon_cli_routes_are_empty_after_phase_1_fallback_removal() -> None:
    import striatum.daemon_rpc.server as server_mod

    assert CLI_ROUTES == {}
    assert not hasattr(server_mod, "invoke")


def test_pg_backed_methods_do_not_keep_daemon_cli_fallback_routes() -> None:
    overlap = sorted(_pg_handlers() & set(CLI_ROUTES))
    assert not overlap, (
        "PG-backed methods must not keep daemon CLI_ROUTES fallbacks: "
        + ", ".join(overlap)
    )


def test_local_file_authoring_methods_do_not_keep_daemon_cli_fallback_routes() -> None:
    assert LOCAL_FILE_AUTHORING_METHODS == LOCAL_FILE_AUTHORING_METHODS_EXPECTED
    overlap = sorted(LOCAL_FILE_AUTHORING_METHODS & set(CLI_ROUTES))
    assert not overlap, (
        "CLI-local workflow authoring methods must not keep daemon "
        "CLI_ROUTES fallbacks: " + ", ".join(overlap)
    )


def test_local_file_authoring_daemon_methods_fail_closed_without_cli_invoke(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    import striatum.daemon_pg.handlers.registry as registry_mod

    monkeypatch.setattr(registry_mod, "resolve_pg_handler", lambda _method: None)

    router = DaemonRpcRouter(pg_conn=object(), repo_root=tmp_path)
    auth = RpcAuthContext("client", "token", "repo_a", "read", "allowed")

    for method in sorted(LOCAL_FILE_AUTHORING_METHODS):
        with pytest.raises(RpcError, match="workflow authoring is CLI-local"):
            router._route(  # noqa: SLF001 - dispatch-order guardrail.
                RpcEnvelope(
                    schema_version=1,
                    request_id=f"cli-local-{method}",
                    method=method,
                    params={"repository_id": "repo_a"},
                ),
                repo_root=tmp_path,
                auth=auth,
            )


def test_pg_backed_router_dispatch_does_not_call_cli_invoke(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    import striatum.daemon_pg.handlers  # noqa: F401 - registers decorators.
    import striatum.daemon_pg.handlers.registry as registry_mod

    router = DaemonRpcRouter(pg_conn=object(), repo_root=tmp_path)
    auth = RpcAuthContext("client", "token", "repo_a", "read", "allowed")

    for method in sorted(_pg_handlers()):
        seen: list[str] = []

        def sentinel_handler(_ctx: Any, _params: Any, *, expected: str = method) -> dict[str, Any]:
            seen.append(expected)
            return {"method": expected}

        monkeypatch.setattr(
            registry_mod,
            "resolve_pg_handler",
            lambda requested, expected=method: sentinel_handler if requested == expected else None,
        )
        result = router._route(  # noqa: SLF001 - dispatch-order guardrail.
            RpcEnvelope(
                schema_version=1,
                request_id=f"dispatch-{method}",
                method=method,
                params={"repository_id": "repo_a"},
            ),
            repo_root=tmp_path,
            auth=auth,
        )
        assert result == {"method": method}
        assert seen == [method]


@pytest.mark.parametrize(
    "argv",
    [
        ["status"],
        ["run", "start", "--run-id", "run_1"],
        ["session", "close", "--session-id", "sess_1", "--reason", "done"],
        ["claim-next", "--session-id", "sess_1"],
        ["send", "--session-id", "sess_1", "--kind", "note", "--body-json", "{}"],
        [
            "publish-artifact",
            "--session-id",
            "sess_1",
            "--job-id",
            "job_1",
            "--lease-id",
            "lease_1",
            "--path",
            "docs/out.md",
        ],
        ["recovery", "auto-publish", "--run-id", "run_1", "--dry-run"],
    ],
)
def test_production_daemon_required_commands_refuse_before_sqlite_connect(
    argv: list[str],
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "1")
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_SOCKET", str(tmp_path / "missing.sock"))

    parser = build_parser()
    args = parser.parse_args(["--repo", str(tmp_path), *argv])

    with pytest.raises(DaemonUnreachableError):
        dispatch(args)


def test_recovery_watch_scheduler_refuses_before_sqlite_connect(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "1")
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_SOCKET", str(tmp_path / "missing.sock"))

    parser = build_parser()
    args = parser.parse_args(
        [
            "--repo",
            str(tmp_path),
            "recovery",
            "watch",
            "--run-id",
            "run_1",
            "--max-sweeps",
            "1",
            "--json",
        ]
    )

    with pytest.raises(StriatumError, match="daemon_unreachable"):
        dispatch(args)


def test_daemon_routed_command_fails_closed_when_route_layer_crashes(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    import striatum.cli.daemon_rpc_route as route_mod

    dispatch_mod = importlib.import_module("striatum.cli.dispatch")

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.setenv("STRIATUM_SQLITE_CONNECT_TRIPWIRE", "1")
    monkeypatch.setattr(dispatch_mod, "enforce_daemon_required", lambda *_args, **_kwargs: None)

    def crash_route(_args: Any, _repo: Path) -> tuple[bool, object]:
        raise RuntimeError("route translation exploded")

    monkeypatch.setattr(route_mod, "try_route", crash_route)

    parser = build_parser()
    args = parser.parse_args(["--repo", str(tmp_path), "status"])

    with pytest.raises(StriatumError, match="daemon_route_failed"):
        dispatch_mod.dispatch(args)

from __future__ import annotations

import importlib
import json
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

REPO_ROOT = Path(__file__).resolve().parents[2]
AUTHORITY_MATRIX_PATH = REPO_ROOT / "docs" / "architecture" / "COMMAND_AUTHORITY_MATRIX.md"
DAEMON_METHOD_CONTRACT_PATH = REPO_ROOT / "contracts" / "daemon_methods.json"


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

GO_DAEMON_RETIREMENT_BLOCKER_METHODS: frozenset[str] = frozenset(
    {
        "apply.reviewed_patch",
        "daemon.migrate_repo_local",
        "dogfood.publish_on_behalf",
        "dogfood.surgical_recovery",
    }
)


def _authority_matrix_section(start: str, end: str | None = None) -> str:
    text = AUTHORITY_MATRIX_PATH.read_text(encoding="utf-8")
    section = text.split(start, 1)[1]
    if end is not None:
        section = section.split(end, 1)[0]
    return section


def _authority_matrix_rows(section: str) -> dict[str, list[str]]:
    rows: dict[str, list[str]] = {}
    for line in section.splitlines():
        if not line.startswith("| `"):
            continue
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if not cells or not (cells[0].startswith("`") and cells[0].endswith("`")):
            continue
        rows[cells[0].strip("`")] = cells
    return rows


def _registered_matrix_rows() -> dict[str, list[str]]:
    return _authority_matrix_rows(
        _authority_matrix_section("## Registered Daemon Methods", "## Deprecated Alias Methods")
    )


def _deprecated_alias_matrix_rows() -> dict[str, list[str]]:
    return _authority_matrix_rows(
        _authority_matrix_section("## Deprecated Alias Methods", "## CLI-Only Or Out-Of-Band Commands")
    )


def _contract_cli_routes_by_method() -> dict[str, list[str]]:
    contract = json.loads(DAEMON_METHOD_CONTRACT_PATH.read_text(encoding="utf-8"))
    raw_routes = contract.get("cli_routes")
    assert isinstance(raw_routes, list)
    routes_by_method: dict[str, list[str]] = {}
    for index, raw_route in enumerate(raw_routes):
        assert isinstance(raw_route, dict), f"cli_routes[{index}] must be an object"
        command = raw_route.get("command")
        subcommand = raw_route.get("subcommand")
        method = raw_route.get("method")
        assert isinstance(command, str), f"cli_routes[{index}].command must be a string"
        assert subcommand is None or isinstance(subcommand, str), (
            f"cli_routes[{index}].subcommand must be a string or null"
        )
        assert isinstance(method, str), f"cli_routes[{index}].method must be a string"
        label = command if subcommand is None else f"{command} {subcommand}"
        routes_by_method.setdefault(method, []).append(label)
    return {method: sorted(labels) for method, labels in routes_by_method.items()}


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


def test_authority_matrix_covers_active_registry_methods() -> None:
    rows = _registered_matrix_rows()
    active_methods = {
        method
        for method, entry in METHOD_REGISTRY.items()
        if not entry.deprecated
    }
    assert not sorted(active_methods - set(rows)), (
        "active daemon RPC methods missing from COMMAND_AUTHORITY_MATRIX.md: "
        + ", ".join(sorted(active_methods - set(rows)))
    )
    unknown_rows = sorted(method for method in rows if method not in METHOD_REGISTRY)
    assert not unknown_rows, (
        "COMMAND_AUTHORITY_MATRIX.md registered rows are not in METHOD_REGISTRY: "
        + ", ".join(unknown_rows)
    )


def test_authority_matrix_capabilities_and_scopes_match_registry() -> None:
    mismatches: list[str] = []
    for method, row in sorted(_registered_matrix_rows().items()):
        entry = METHOD_REGISTRY[method]
        expected_capability = entry.required_capability or "none"
        expected_scope = entry.effective_repository_scope_mode
        actual_capability = row[2]
        actual_scope = row[3]
        if actual_capability != expected_capability:
            mismatches.append(
                f"{method} capability: matrix={actual_capability!r}, registry={expected_capability!r}"
            )
        if actual_scope != expected_scope:
            mismatches.append(
                f"{method} scope: matrix={actual_scope!r}, registry={expected_scope!r}"
            )

    assert not mismatches, (
        "COMMAND_AUTHORITY_MATRIX.md capability/scope drift: " + "; ".join(mismatches)
    )


def test_authority_matrix_cli_command_cells_include_contract_route_labels() -> None:
    rows = _registered_matrix_rows()
    missing: list[str] = []
    for method, labels in sorted(_contract_cli_routes_by_method().items()):
        row = rows.get(method)
        if row is None:
            missing.append(f"{method} is routed by cli_routes but absent from registered matrix")
            continue
        cli_cell = row[1]
        missing.extend(
            f"{method} CLI command cell missing route label {label!r}"
            for label in labels
            if label not in cli_cell
        )

    assert not missing, (
        "COMMAND_AUTHORITY_MATRIX.md CLI command route-label drift: "
        + "; ".join(missing)
    )


def test_authority_matrix_cli_fallback_cells_match_runtime_cli_routes() -> None:
    mismatches: list[str] = []
    for method, row in sorted(_registered_matrix_rows().items()):
        expected = "yes" if method in CLI_ROUTES else "no"
        actual = row[6]
        if actual != expected:
            mismatches.append(
                f"{method} CLI fallback: matrix={actual!r}, runtime={expected!r}"
            )
    for method, row in sorted(_deprecated_alias_matrix_rows().items()):
        expected = "yes" if method in CLI_ROUTES else "no"
        actual = row[4]
        if actual != expected:
            mismatches.append(
                f"{method} deprecated CLI fallback: matrix={actual!r}, runtime={expected!r}"
            )

    assert not mismatches, (
        "COMMAND_AUTHORITY_MATRIX.md CLI fallback drift from runtime CLI_ROUTES: "
        + "; ".join(mismatches)
    )


def test_authority_matrix_deprecated_alias_placement_matches_registry() -> None:
    deprecated_registry = {
        method for method, entry in METHOD_REGISTRY.items() if entry.deprecated
    }
    deprecated_alias_rows = set(_deprecated_alias_matrix_rows())
    # `recovery.auto` is the one deprecated method still documented in the
    # registered-method table because it names the canonical CLI replacement
    # and currently shares the recovery.auto-publish handler.
    expected_alias_rows = deprecated_registry - {"recovery.auto"}

    assert deprecated_alias_rows == expected_alias_rows
    registered_rows = _registered_matrix_rows()
    assert "recovery.auto" in registered_rows
    assert all(
        method not in registered_rows for method in expected_alias_rows
    ), "legacy aliases belong in the deprecated-alias table, not registered methods"


def test_authority_matrix_has_no_go_placeholders() -> None:
    placeholders = [
        method
        for method, row in sorted(_registered_matrix_rows().items())
        if row[5] == "placeholder"
    ]
    assert not placeholders, (
        "COMMAND_AUTHORITY_MATRIX.md still marks Go authority as placeholder: "
        + ", ".join(placeholders)
    )


def test_authority_matrix_go_fail_closed_rows_are_retirement_blockers() -> None:
    fail_closed = {
        method
        for method, row in sorted(_registered_matrix_rows().items())
        if row[5] == "fail_closed"
    }

    assert fail_closed == GO_DAEMON_RETIREMENT_BLOCKER_METHODS


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

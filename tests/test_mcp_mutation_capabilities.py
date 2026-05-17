from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.registry import CAPABILITIES, METHOD_REGISTRY, mcp_tool_descriptor
from striatum.daemon_rpc.server import LOCAL_FILE_AUTHORING_METHODS
from striatum.mcp import DaemonRpcServer


def _method_contract() -> dict[str, dict[str, Any]]:
    contract_path = Path(__file__).resolve().parents[1] / "contracts" / "daemon_methods.json"
    if contract_path.exists():
        raw = json.loads(contract_path.read_text())
        raw_methods: object
        if isinstance(raw, dict):
            raw_methods = raw.get("methods", raw.get("entries", raw))
        else:
            raw_methods = raw
        if isinstance(raw_methods, dict):
            return {
                str(method): dict(meta) if isinstance(meta, dict) else {}
                for method, meta in raw_methods.items()
            }
        if isinstance(raw_methods, list):
            contract: dict[str, dict[str, Any]] = {}
            for item in raw_methods:
                if not isinstance(item, dict):
                    continue
                method = item.get("method", item.get("name"))
                if isinstance(method, str):
                    contract[method] = dict(item)
            return contract
    return {
        method: {
            "required_capability": entry.required_capability,
            "deprecated": entry.deprecated,
            "local_file_authoring": method in LOCAL_FILE_AUTHORING_METHODS,
        }
        for method, entry in METHOD_REGISTRY.items()
    }


def _contract_bool(meta: dict[str, Any], *names: str) -> bool:
    return any(bool(meta.get(name)) for name in names)


def _expected_daemon_mcp_tools(allowed_capabilities: set[str]) -> set[str]:
    expected: set[str] = set()
    for method, meta in _method_contract().items():
        required = meta.get("required_capability")
        if required is None:
            continue
        if method.startswith("daemon."):
            continue
        if method in LOCAL_FILE_AUTHORING_METHODS or _contract_bool(
            meta,
            "local_file_authoring",
            "cli_local",
            "cli_local_only",
        ):
            continue
        if _contract_bool(meta, "deprecated"):
            continue
        if required in allowed_capabilities:
            expected.add(method)
    return expected


def test_daemon_mcp_tools_list_filters_by_capability(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        decision = "allowed" if required == "read" else "denied"
        return RpcAuthContext("client", "token", repository_id, required, decision, None if decision == "allowed" else "capability_missing")

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)

    tools = DaemonRpcServer(pg_conn=object()).daemon_tool_specs({"token": "dtok.secret", "repository_id": "repo_a"})

    names = {tool["name"] for tool in tools}
    assert "status" in names
    assert "workflow.validate" not in names
    assert "workflow.generate" not in names
    assert "workflow.upgrade" not in names
    assert "publish_artifact" not in names
    assert "apply.reviewed_patch" not in names
    assert "dogfood.surgical_recovery" not in names


def test_daemon_mcp_tools_match_registered_non_deprecated_authorized_methods(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    allowed_capabilities = set(CAPABILITIES)

    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        decision = "allowed" if required in allowed_capabilities else "denied"
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            required if decision == "allowed" else None,
            decision,
            None if decision == "allowed" else "capability_missing",
        )

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)

    tools = DaemonRpcServer(pg_conn=object()).daemon_tool_specs({"token": "dtok.secret", "repository_id": "repo_a"})

    names = {tool["name"] for tool in tools}
    assert names == _expected_daemon_mcp_tools(allowed_capabilities)
    tools_by_name = {str(tool["name"]): tool for tool in tools}
    for name in names:
        assert tools_by_name[str(name)] == mcp_tool_descriptor(METHOD_REGISTRY[str(name)])
    assert not (names & LOCAL_FILE_AUTHORING_METHODS)
    assert not {
        method
        for method in names
        if _contract_bool(_method_contract().get(str(method), {}), "deprecated")
    }


def test_daemon_mcp_tools_list_exposes_surgical_recovery_only_for_matching_capability(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        decision = "allowed" if required == "surgical_recovery" else "denied"
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            required if decision == "allowed" else None,
            decision,
            None if decision == "allowed" else "capability_missing",
        )

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)

    tools = DaemonRpcServer(pg_conn=object()).daemon_tool_specs({"token": "dtok.secret", "repository_id": "repo_a"})

    names = {tool["name"] for tool in tools}
    assert "dogfood.surgical_recovery" in names
    assert "dogfood.publish_on_behalf" not in names


def test_daemon_mcp_tools_call_reauthorizes_and_audits_denial(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    audit_calls: list[dict[str, Any]] = []
    request_logs: list[dict[str, Any]] = []

    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        return RpcAuthContext("client", "token", repository_id, None, "denied", "capability_missing")

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)
    # daemon_pg/mcp_dispatch.py rebinds `authorize` as a local name at
    # import time; under certain pytest collection orders it has already
    # imported the original symbol and the source-module monkeypatch
    # above doesn't propagate. Patch the bound reference too — defensive
    # against test-ordering flakes.
    monkeypatch.setattr("striatum.daemon_pg.mcp_dispatch.authorize", fake_authorize)
    def fake_append_audit_row(*args: Any, **kwargs: Any) -> int:
        audit_calls.append(kwargs)
        return 7

    def fake_append_request_log(*args: Any, **kwargs: Any) -> None:
        request_logs.append(kwargs)

    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_audit_row", fake_append_audit_row)
    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_request_log", fake_append_request_log)

    result = DaemonRpcServer(pg_conn=object()).call_daemon_tool(
        {
            "name": "publish_artifact",
            "token": "dtok.secret",
            "request_id": "req-1",
            "arguments": {"repository_id": "repo_a"},
        }
    )

    assert result["isError"] is True
    structured = result["structuredContent"]
    assert isinstance(structured, dict)
    assert structured["error"] == "capability_missing"
    assert audit_calls[0]["transport"] == "mcp"
    assert audit_calls[0]["auth"].decision == "denied"
    assert request_logs[0]["decision"] == "denied"


def test_daemon_mcp_unknown_tool_is_default_denied_and_audited(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    audit_calls: list[dict[str, Any]] = []
    def fake_append_unknown_audit_row(*args: Any, **kwargs: Any) -> int:
        audit_calls.append(kwargs)
        return 9

    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_audit_row", fake_append_unknown_audit_row)
    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_request_log", lambda *args, **kwargs: None)

    result = DaemonRpcServer(pg_conn=object()).call_daemon_tool(
        {"name": "not.a.method", "request_id": "req-2", "arguments": {}}
    )

    assert result["isError"] is True
    structured = result["structuredContent"]
    assert isinstance(structured, dict)
    assert structured["error"] == "method_unknown"
    assert audit_calls[0]["auth"].denial_reason == "method_unknown"

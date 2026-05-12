from __future__ import annotations

from typing import Any

from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.mcp import DaemonRpcServer


def test_daemon_mcp_tools_list_filters_by_capability(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        decision = "allowed" if required == "read" else "denied"
        return RpcAuthContext("client", "token", repository_id, required, decision, None if decision == "allowed" else "capability_missing")

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)

    tools = DaemonRpcServer(pg_conn=object()).daemon_tool_specs({"token": "dtok.secret", "repository_id": "repo_a"})

    names = {tool["name"] for tool in tools}
    assert "status" in names
    assert "publish_artifact" not in names
    assert "apply.reviewed_patch" not in names


def test_daemon_mcp_tools_call_reauthorizes_and_audits_denial(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    audit_calls: list[dict[str, Any]] = []
    request_logs: list[dict[str, Any]] = []

    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        return RpcAuthContext("client", "token", repository_id, None, "denied", "capability_missing")

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)
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

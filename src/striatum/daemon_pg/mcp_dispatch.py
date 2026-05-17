"""MCP-to-daemon-RPC dispatch bridge."""

from __future__ import annotations

from pathlib import Path
from typing import Any, Mapping

from striatum.daemon_rpc.capability import RpcAuthContext, authorize
from striatum.daemon_rpc.envelope import SUPPORTED_ENVELOPE_VERSION, RpcEnvelope
from striatum.daemon_rpc.registry import METHOD_REGISTRY
from striatum.daemon_rpc import request_log
from striatum.daemon_rpc.server import DaemonRpcRouter
from striatum.primitives import JsonObject


def dispatch_mcp_tool_call(
    *,
    pg_conn: Any,
    router: DaemonRpcRouter | None = None,
    name: str,
    arguments: JsonObject,
    token: str | None,
    request_id: str,
    repo_root: Path | None = None,
    substrate_schema: int = 1,
) -> JsonObject:
    """Dispatch one MCP ``tools/call`` through the daemon RPC method router."""
    entry = METHOD_REGISTRY.get(name)
    if entry is None:
        repository_id = _repository_id(arguments)
        auth = RpcAuthContext(None, None, repository_id, None, "denied", "method_unknown")
        audit_id = request_log.append_audit_row(
            pg_conn,
            auth=auth,
            method=name,
            transport="mcp",
            request_id=request_id,
            params=arguments,
            exit_code=10,
        )
        tool_response = _tool_result(name, ok=False, audit_id=audit_id, error="method_unknown")
        request_log.append_request_log(
            pg_conn,
            request_id=request_id,
            method=name,
            params=arguments,
            auth=auth,
            decision="denied",
            response=tool_response,
            audit_id=audit_id,
        )
        return tool_response

    repository_id = (
        _repository_id(arguments)
        if entry.effective_repository_scope_mode == "single_repo"
        else None
    )
    auth = authorize(
        pg_conn,
        required=entry.required_capability,
        repository_id=repository_id,
        token=token,
    )
    if auth.decision != "allowed":
        audit_id = request_log.append_audit_row(
            pg_conn,
            auth=auth,
            method=name,
            transport="mcp",
            request_id=request_id,
            params=arguments,
            exit_code=10,
        )
        tool_response = _tool_result(
            name,
            ok=False,
            audit_id=audit_id,
            error=auth.denial_reason or "capability_missing",
        )
        request_log.append_request_log(
            pg_conn,
            request_id=request_id,
            method=name,
            params=arguments,
            auth=auth,
            decision="denied",
            response=tool_response,
            audit_id=audit_id,
        )
        return tool_response

    effective_router = router or DaemonRpcRouter(
        pg_conn=pg_conn,
        repo_root=repo_root,
        substrate_schema=substrate_schema,
    )
    envelope = RpcEnvelope(
        schema_version=SUPPORTED_ENVELOPE_VERSION,
        request_id=request_id,
        method=name,
        params=dict(arguments),
        capability_token=token,
    )
    response = effective_router.handle(
        envelope,
        connection_id="mcp",
        transport="mcp",
        require_handshake=False,
    )
    audit_id = _audit_int(response.audit_id)
    if response.ok:
        return _tool_result(name, ok=True, audit_id=audit_id, data=response.data)
    error = str(response.data.get("code") or "command_failed")
    message = response.data.get("message")
    return _tool_result(
        name,
        ok=False,
        audit_id=audit_id,
        error=error,
        error_message=str(message) if message is not None else None,
        data=response.data,
    )


def _repository_id(arguments: Mapping[str, Any]) -> str | None:
    value = arguments.get("repository_id")
    return str(value) if value is not None else None


def _audit_int(audit_id: str | None) -> int | None:
    if audit_id is None:
        return None
    if audit_id.startswith("aud_"):
        audit_id = audit_id[4:]
    try:
        return int(audit_id)
    except ValueError:
        return None


def _tool_result(
    name: str,
    *,
    ok: bool,
    audit_id: int | None,
    error: str | None = None,
    error_message: str | None = None,
    data: Mapping[str, Any] | None = None,
) -> JsonObject:
    structured: JsonObject = {
        "ok": ok,
        "method": name,
        "audit_id": audit_id,
    }
    if data is not None:
        structured["data"] = dict(data)
    if error is not None:
        structured["error"] = error
    if error_message is not None:
        structured["error_message"] = error_message
    return {
        "content": [{"type": "text", "text": name}],
        "structuredContent": structured,
        "isError": not ok,
    }


__all__ = ["dispatch_mcp_tool_call"]

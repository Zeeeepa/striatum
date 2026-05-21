"""MCP-shaped client helpers for daemon mutation tests."""

from __future__ import annotations

import json
import uuid
from pathlib import Path
from typing import Any
from urllib.request import Request, urlopen

from striatum.daemon_pg.mcp_dispatch import dispatch_mcp_tool_call
from striatum.daemon_pg.mcp_resources import daemon_mcp_read_resource_pg, daemon_mcp_resources_pg
from striatum.daemon_rpc import capability
from striatum.daemon_rpc.registry import METHOD_REGISTRY, mcp_tool_descriptor
from striatum.daemon_rpc.server import PRODUCTION_MCP_HIDDEN_METHODS
from striatum.primitives import json_dumps


def daemon_tool_specs(pg_conn: Any, params: dict[str, object]) -> list[dict[str, object]]:
    token_value = params.get("token")
    if token_value is not None and not isinstance(token_value, str):
        raise ValueError("token must be a string")
    repository_id = _optional_string(params, "repository_id")
    tools: list[dict[str, object]] = []
    for entry in sorted(METHOD_REGISTRY.values(), key=lambda item: item.method):
        if entry.required_capability is None or entry.method.startswith("daemon."):
            continue
        if entry.deprecated or entry.method in PRODUCTION_MCP_HIDDEN_METHODS:
            continue
        auth = capability.authorize(
            pg_conn,
            required=entry.required_capability,
            repository_id=repository_id
            if entry.effective_repository_scope_mode == "single_repo"
            else None,
            token=token_value,
        )
        if auth.decision == "allowed":
            tools.append(dict(mcp_tool_descriptor(entry)))
    return tools


class McpClient:
    def __init__(
        self,
        *,
        pg_conn: Any,
        token: str,
        repo_root: Path | None = None,
        endpoint: str | None = None,
    ) -> None:
        self._pg_conn = pg_conn
        self._token = token
        self._repo_root = repo_root
        self._endpoint = endpoint

    def list_tools(
        self, *, repository_id: str | None = None, **extra: object
    ) -> list[dict[str, object]]:
        if self._endpoint:
            params: dict[str, object] = dict(extra)
            if repository_id is not None:
                params["repository_id"] = repository_id
            result = self._jsonrpc("tools/list", params)
            tools = result.get("tools")
            if not isinstance(tools, list):
                raise AssertionError(f"tools/list did not return tools: {result!r}")
            return [dict(item) for item in tools if isinstance(item, dict)]
        params: dict[str, object] = {"token": self._token, **extra}
        if repository_id is not None:
            params["repository_id"] = repository_id
        return daemon_tool_specs(self._pg_conn, params)

    def list_resources(self, **extra: object) -> list[dict[str, object]]:
        resources = daemon_mcp_resources_pg(
            self._pg_conn,
            token=self._token,
            request_id=f"req_{uuid.uuid4().hex}",
        )
        return [dict(item) for item in resources]

    def read_resource(self, uri: str, **extra: object) -> dict[str, Any]:
        result = daemon_mcp_read_resource_pg(
            self._pg_conn,
            uri,
            token=self._token,
            request_id=f"req_{uuid.uuid4().hex}",
        )
        return dict(json.loads(json_dumps(result)))

    def call_tool(
        self,
        name: str,
        *,
        repository_id: str | None = None,
        arguments: dict[str, object] | None = None,
    ) -> dict[str, Any]:
        args = dict(arguments or {})
        if repository_id is not None:
            args["repository_id"] = repository_id
        if self._endpoint:
            params: dict[str, object] = {
                "name": name,
                "arguments": args,
                "request_id": f"req_{uuid.uuid4().hex}",
            }
            if repository_id is not None:
                params["repository_id"] = repository_id
            return dict(self._jsonrpc("tools/call", params))
        return dict(
            dispatch_mcp_tool_call(
                pg_conn=self._pg_conn,
                name=name,
                token=self._token,
                request_id=f"req_{uuid.uuid4().hex}",
                arguments=args,
                repo_root=self._repo_root,
            )
        )

    def _jsonrpc(self, method: str, params: dict[str, object]) -> dict[str, Any]:
        assert self._endpoint is not None
        request_id = f"req_{uuid.uuid4().hex}"
        payload = {
            "jsonrpc": "2.0",
            "id": request_id,
            "method": method,
            "params": params,
        }
        body = json_dumps(payload).encode("utf-8")
        request = Request(
            self._endpoint,
            data=body,
            headers={
                "Accept": "application/json",
                "Authorization": f"Bearer {self._token}",
                "Content-Type": "application/json",
            },
            method="POST",
        )
        with urlopen(request, timeout=10) as response:
            raw = response.read().decode("utf-8")
        decoded = json.loads(raw)
        if not isinstance(decoded, dict):
            raise AssertionError(f"{method} JSON-RPC response is not an object: {raw}")
        if decoded.get("id") != request_id:
            raise AssertionError(f"{method} JSON-RPC response id mismatch: {decoded!r}")
        error = decoded.get("error")
        if isinstance(error, dict):
            raise AssertionError(f"{method} JSON-RPC error: {error!r}")
        result = decoded.get("result")
        if not isinstance(result, dict):
            raise AssertionError(f"{method} JSON-RPC result is not an object: {decoded!r}")
        return result


def _optional_string(values: dict[str, object], key: str) -> str | None:
    value = values.get(key)
    if value is None:
        return None
    if not isinstance(value, str | int | float | bool):
        raise ValueError(f"argument {key!r} must be a scalar value")
    return str(value)

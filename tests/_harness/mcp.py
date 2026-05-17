"""MCP client wrapper for daemon mutation tests."""

from __future__ import annotations

import json
import uuid
from pathlib import Path
from typing import Any

from striatum.mcp import DaemonRpcServer


class McpClient:
    def __init__(self, *, pg_conn: Any, token: str, repo_root: Path | None = None) -> None:
        self._server = DaemonRpcServer(pg_conn=pg_conn, repo_root=repo_root)
        self._token = token

    def list_tools(self, *, repository_id: str | None = None, **extra: object) -> list[dict[str, object]]:
        params: dict[str, object] = {"token": self._token, **extra}
        if repository_id is not None:
            params["repository_id"] = repository_id
        result = self._server.daemon_tool_specs(params)
        return [dict(item) for item in result]

    def list_resources(self, **extra: object) -> list[dict[str, object]]:
        response = self._server.handle_request(
            {
                "jsonrpc": "2.0",
                "id": f"req_{uuid.uuid4().hex}",
                "method": "resources/list",
                "params": {
                    "token": self._token,
                    "request_id": f"req_{uuid.uuid4().hex}",
                    **extra,
                },
            }
        )
        if response is None or "error" in response:
            raise AssertionError(f"resources/list failed: {response}")
        result = response["result"]
        return [dict(item) for item in result["resources"]]

    def read_resource(self, uri: str, **extra: object) -> dict[str, Any]:
        response = self._server.handle_request(
            {
                "jsonrpc": "2.0",
                "id": f"req_{uuid.uuid4().hex}",
                "method": "resources/read",
                "params": {
                    "uri": uri,
                    "token": self._token,
                    "request_id": f"req_{uuid.uuid4().hex}",
                    **extra,
                },
            }
        )
        if response is None or "error" in response:
            raise AssertionError(f"resources/read failed: {response}")
        contents = response["result"]["contents"]
        return dict(json.loads(str(contents[0]["text"])))

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
        return dict(
            self._server.call_daemon_tool(
                {
                    "name": name,
                    "token": self._token,
                    "request_id": f"req_{uuid.uuid4().hex}",
                    "arguments": args,
                }
            )
        )

"""Blob diagnostics bridge for ``striatum daemon doctor``.

The blob probe lives in the Go daemon because it owns the configured S3
client. This module keeps the Python doctor path as a consumer of that block
instead of re-reading daemon process environment or duplicating probe logic.
"""

from __future__ import annotations

import socket
from typing import Any, Mapping

from striatum.cli.daemon_required import resolve_socket_path
from striatum.cli.daemon_rpc_route import _call_with_handshake, _rpc_data_or_raise
from striatum.daemon_rpc.envelope import RpcError


def fetch_blob_doctor_block(repository_id: str = "") -> dict[str, Any]:
    params: dict[str, Any] = {}
    if repository_id:
        params["repository_id"] = repository_id
    try:
        response = _call_with_handshake(resolve_socket_path(), "doctor.blob_block", params)
        data = _rpc_data_or_raise(response)
    except RpcError as exc:
        if exc.code == "method_unknown":
            return {
                "configured": None,
                "errors": ["daemon binary predates RFC 0073; rebuild and restart daemon"],
            }
        return {"configured": False, "errors": [f"daemon RPC failed: {exc.code}: {exc}"]}
    except (OSError, socket.timeout) as exc:
        return {"configured": False, "errors": [f"daemon socket unreachable: {exc}"]}
    if isinstance(data, Mapping):
        return dict(data)
    return {
        "configured": None,
        "errors": ["daemon returned invalid blob doctor block"],
    }

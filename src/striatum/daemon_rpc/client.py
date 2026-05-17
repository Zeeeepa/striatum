"""Small daemon RPC client helpers used by CLI routing tests and future clients."""

from __future__ import annotations

import socket
from pathlib import Path
from typing import Any

from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError
from striatum.primitives import json_loads


def call_unix(path: Path, envelope: RpcEnvelope, *, timeout_seconds: float = 30.0) -> dict[str, Any]:
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
        sock.settimeout(timeout_seconds)
        sock.connect(str(path))
        sock.sendall(envelope.encode() + b"\n")
        body = _recv_line(sock)
    payload = json_loads(body.decode("utf-8"))
    if not isinstance(payload, dict):
        raise RpcError("schema_invalid", "daemon RPC response must be a JSON object")
    return payload


def hello_envelope(*, request_id: str, client_name: str, client_version: str) -> RpcEnvelope:
    return RpcEnvelope(
        schema_version=1,
        request_id=request_id,
        method="daemon.hello",
        params={
            "client": {
                "name": client_name,
                "version": client_version,
                "supported_envelope": [1],
                "supported_framings": ["json"],
            }
        },
    )


def _recv_line(sock: socket.socket) -> bytes:
    chunks: list[bytes] = []
    while True:
        chunk = sock.recv(1)
        if not chunk:
            break
        if chunk == b"\n":
            break
        chunks.append(chunk)
    return b"".join(chunks)

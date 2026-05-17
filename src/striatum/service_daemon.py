"""Daemon RPC helpers for local service mutation handlers."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping

from striatum.cli.daemon_required import daemon_socket_is_reachable, resolve_socket_path
from striatum.cli.daemon_rpc_route import _call_with_handshake, _lookup_repository_id
from striatum.daemon_rpc.envelope import RpcError
from striatum.daemon_rpc.registry import METHOD_REGISTRY


@dataclass(frozen=True)
class ServiceDaemonRpcError(Exception):
    """HTTP-shaped daemon RPC failure for service handlers."""

    status: int
    code: str
    message: str
    kind: str | None = None
    details: dict[str, Any] | None = None

    def __str__(self) -> str:
        return self.message


def call_repo_method(repo: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
    """Call a repository-scoped daemon RPC method and return its data object."""

    if method not in METHOD_REGISTRY:
        raise ServiceDaemonRpcError(500, "method_unknown", f"method has no registry entry: {method}")
    sock_path = resolve_socket_path()
    if not daemon_socket_is_reachable(sock_path):
        raise ServiceDaemonRpcError(
            503,
            "daemon_unreachable",
            f"daemon_unreachable: {method} requires the Striatum daemon",
        )
    repository_id = params.get("repository_id") or _lookup_repository_id(repo)
    if repository_id is None:
        raise ServiceDaemonRpcError(
            503,
            "repo_not_registered",
            f"repo_not_registered: {method} requires a daemon-registered repository",
        )
    rpc_params = dict(params)
    rpc_params.setdefault("repository_id", repository_id)
    try:
        response = _call_with_handshake(sock_path, method, rpc_params)
    except OSError as exc:
        raise ServiceDaemonRpcError(
            503,
            "daemon_unreachable",
            f"daemon_unreachable: {exc}",
        ) from exc
    except RpcError as exc:
        status, kind = _http_error_shape(exc.code)
        raise ServiceDaemonRpcError(status, exc.code, str(exc), kind, dict(exc.details)) from exc
    if not bool(response.get("ok")):
        raw_error = response.get("data")
        err: Mapping[str, Any] = raw_error if isinstance(raw_error, dict) else {}
        code = str(err.get("code") or "rpc_error")
        message = str(err.get("message") or "daemon RPC call failed")
        raw_details = err.get("details")
        details = dict(raw_details) if isinstance(raw_details, Mapping) else {}
        status, kind = _http_error_shape(code)
        raise ServiceDaemonRpcError(status, code, message, kind, details)
    data = response.get("data")
    if isinstance(data, dict):
        return data
    return {"value": data}


def _http_error_shape(code: str) -> tuple[int, str | None]:
    if code == "not_found":
        return 404, None
    if code == "invalid_transition":
        return 409, "invalid_transition"
    if code == "branch_confirmation_required":
        return 409, "branch_confirmation"
    if code in {"schema_invalid", "workflow_error"}:
        return 400, None
    if code in {"token_missing", "capability_missing"}:
        return 403, None
    if code == "version_incompatible":
        return 503, None
    return 500, None

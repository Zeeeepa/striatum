"""Sealed-apply RPC refusal skeleton.

The accepted dogfood-034 slice establishes the daemon-owned route and denial
vocabulary. The actual patch mutation is intentionally fail-closed until the
daemon starts with a signing key and apply token.
"""

from __future__ import annotations

from typing import Any, Mapping

from striatum.daemon_rpc.envelope import RpcError


def handle_apply_rpc(method: str, params: Mapping[str, Any]) -> dict[str, Any]:
    del params
    if method in {"apply.receipt.show", "apply.receipt.verify"}:
        raise RpcError("receipt_missing", "apply receipt was not found", exit_code=4)
    raise RpcError("method_unknown", f"unknown apply RPC method: {method}")

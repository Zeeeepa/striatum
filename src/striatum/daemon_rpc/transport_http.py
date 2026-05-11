"""Loopback-only HTTP transport guard for daemon RPC."""

from __future__ import annotations

import ipaddress

from striatum.daemon_rpc.envelope import RpcError


def validate_loopback_bind(host: str) -> None:
    try:
        ip = ipaddress.ip_address(host)
    except ValueError as exc:
        raise RpcError("schema_invalid", "daemon RPC HTTP host must be an IP address") from exc
    if not ip.is_loopback:
        raise RpcError("non_loopback_refused", "daemon RPC HTTP transport only permits loopback bind addresses")

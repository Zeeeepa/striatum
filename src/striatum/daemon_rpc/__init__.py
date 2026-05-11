"""Daemon V2 RPC primitives.

The package is intentionally transport-light: envelope validation,
handshake/version rules, method metadata, capability checks, request logging,
and small transports live here, while workflow mutations still route through
the existing CLI/API command service.
"""

from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError, RpcResponse
from striatum.daemon_rpc.registry import METHODS_ETAG, METHOD_REGISTRY

__all__ = [
    "METHODS_ETAG",
    "METHOD_REGISTRY",
    "RpcEnvelope",
    "RpcError",
    "RpcResponse",
]

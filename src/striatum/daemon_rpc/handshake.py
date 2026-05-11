"""Daemon RPC version handshake."""

from __future__ import annotations

from typing import Any, Mapping

from striatum import __version__
from striatum.daemon_rpc.envelope import DEFAULT_FRAMING, SUPPORTED_ENVELOPE_VERSION, RpcError


def build_welcome(
    params: Mapping[str, Any],
    *,
    methods_etag: str,
    substrate_schema: int,
    sealed_apply_supported: bool,
    key_loaded: bool = False,
    public_key: str | None = None,
) -> dict[str, Any]:
    client = params.get("client")
    if not isinstance(client, dict):
        raise RpcError("schema_invalid", "daemon.hello requires a client object")
    envelopes = client.get("supported_envelope")
    framings = client.get("supported_framings")
    if not isinstance(envelopes, list) or SUPPORTED_ENVELOPE_VERSION not in envelopes:
        raise RpcError(
            "version_incompatible",
            "client and daemon have no shared envelope version",
            details={"daemon_supported_envelope": [SUPPORTED_ENVELOPE_VERSION]},
        )
    if not isinstance(framings, list) or DEFAULT_FRAMING not in framings:
        raise RpcError(
            "version_incompatible",
            "client and daemon have no shared framing",
            details={"daemon_supported_framings": [DEFAULT_FRAMING]},
        )
    return {
        "daemon_version": __version__,
        "envelope": SUPPORTED_ENVELOPE_VERSION,
        "framing": DEFAULT_FRAMING,
        "substrate": "postgres",
        "substrate_schema": substrate_schema,
        "methods_etag": methods_etag,
        "sealed_apply": {
            "supported": sealed_apply_supported,
            "key_loaded": key_loaded,
            "public_key": public_key,
        },
    }

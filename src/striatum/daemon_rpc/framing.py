"""NDJSON framing helpers for daemon RPC transports."""

from __future__ import annotations

from typing import BinaryIO, Iterator

from striatum.daemon_rpc.envelope import RpcEnvelope, RpcResponse


def read_envelopes(stream: BinaryIO) -> Iterator[RpcEnvelope]:
    for line in stream:
        if not line.strip():
            continue
        yield RpcEnvelope.decode(line)


def write_response(stream: BinaryIO, response: RpcResponse) -> None:
    stream.write(response.encode() + b"\n")
    stream.flush()

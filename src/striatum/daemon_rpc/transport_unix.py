"""Unix-domain socket transport validation for daemon RPC."""

from __future__ import annotations

import os
import socket
from pathlib import Path

from striatum.daemon_rpc.envelope import RpcError


def bind_unix_socket(path: Path) -> socket.socket:
    path.parent.mkdir(parents=True, exist_ok=True)
    mode = path.parent.stat().st_mode & 0o777
    if mode & 0o077:
        raise RpcError("permission_denied", "daemon RPC runtime directory must be owner-only")
    if path.exists():
        path.unlink()
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    try:
        sock.bind(str(path))
        os.chmod(path, 0o600)
        sock.listen()
    except Exception:
        sock.close()
        raise
    return sock

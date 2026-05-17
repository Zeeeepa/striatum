"""Runtime helpers for the local service process."""

from __future__ import annotations

import os
import socket
import threading
import time
from pathlib import Path
from typing import Any

from striatum.service_http import LOOPBACK_HOSTS

__all__ = [
    "ServiceAlreadyRunningError",
    "ServiceConfigError",
    "check_single_instance",
    "ensure_loopback",
    "service_mode",
    "striatum_version",
    "wait_for_service_shutdown",
]


class ServiceConfigError(Exception):
    """Raised at startup for service configuration errors."""


class ServiceAlreadyRunningError(Exception):
    """Raised when a PID file points at a live process."""


def striatum_version() -> str:
    try:
        from importlib.metadata import version as _meta_version

        return _meta_version("striatum")
    except Exception:  # noqa: BLE001
        return "unknown"


def service_mode(server: Any) -> str:
    sock = getattr(server, "socket", None)
    if sock is None:
        return "unknown"
    if sock.family == socket.AF_UNIX:
        return "unix"
    return "tcp"


def ensure_loopback(host: str) -> None:
    if host in LOOPBACK_HOSTS:
        return
    raise ServiceConfigError(
        f"refusing to bind to non-loopback host {host!r}; allowed: "
        f"{sorted(LOOPBACK_HOSTS)}"
    )


def check_single_instance(pid_path: Path) -> None:
    if not pid_path.exists():
        return
    try:
        text = pid_path.read_text(encoding="utf-8").strip()
        existing_pid = int(text)
    except (OSError, ValueError):
        return
    try:
        os.kill(existing_pid, 0)
    except ProcessLookupError:
        return
    except PermissionError:
        # Process exists, owned by another uid. Treat as alive.
        pass
    raise ServiceAlreadyRunningError(
        f"service already running (pid {existing_pid}); pid file {pid_path}"
    )


def wait_for_service_shutdown(
    shutdown_event: threading.Event,
    *,
    idle_timeout_seconds: int | None,
) -> None:
    deadline = (
        None
        if idle_timeout_seconds is None
        else time.monotonic() + float(idle_timeout_seconds)
    )
    while not shutdown_event.is_set():
        if deadline is None:
            shutdown_event.wait(timeout=0.2)
            continue
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            return
        shutdown_event.wait(timeout=min(0.2, remaining))

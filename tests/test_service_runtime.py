from __future__ import annotations

import os
import socket
import threading
from pathlib import Path
from typing import Any

import pytest

from striatum.service_runtime import (
    ServiceAlreadyRunningError,
    ServiceConfigError,
    check_single_instance,
    ensure_loopback,
    service_mode,
    wait_for_service_shutdown,
)


class _FakeSocket:
    def __init__(self, family: socket.AddressFamily) -> None:
        self.family = family


class _FakeServer:
    def __init__(self, family: socket.AddressFamily | None) -> None:
        self.socket: Any = None if family is None else _FakeSocket(family)


def test_service_mode_detects_tcp_unix_and_unknown() -> None:
    assert service_mode(_FakeServer(None)) == "unknown"
    assert service_mode(_FakeServer(socket.AF_INET)) == "tcp"
    assert service_mode(_FakeServer(socket.AF_UNIX)) == "unix"


def test_ensure_loopback_rejects_non_loopback() -> None:
    ensure_loopback("127.0.0.1")
    ensure_loopback("localhost")
    ensure_loopback("::1")

    with pytest.raises(ServiceConfigError):
        ensure_loopback("0.0.0.0")


def test_check_single_instance_ignores_missing_and_invalid_pid(tmp_path: Path) -> None:
    pid_path = tmp_path / "service.pid"

    check_single_instance(pid_path)

    pid_path.write_text("not-a-pid", encoding="utf-8")

    check_single_instance(pid_path)


def test_check_single_instance_refuses_live_pid(tmp_path: Path) -> None:
    pid_path = tmp_path / "service.pid"
    pid_path.write_text(str(os.getpid()), encoding="utf-8")

    with pytest.raises(ServiceAlreadyRunningError):
        check_single_instance(pid_path)


def test_wait_for_service_shutdown_returns_when_event_set() -> None:
    event = threading.Event()
    event.set()

    wait_for_service_shutdown(event, idle_timeout_seconds=None)


def test_wait_for_service_shutdown_returns_on_idle_timeout() -> None:
    wait_for_service_shutdown(threading.Event(), idle_timeout_seconds=0)

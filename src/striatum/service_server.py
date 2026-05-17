"""TCP/Unix server lifecycle helpers for the local service."""

from __future__ import annotations

import os
import signal
import socket
import socketserver
import threading
from collections.abc import Callable
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any

from striatum.service_http import allowed_origins_for_bind
from striatum.service_runtime import (
    ServiceConfigError,
    check_single_instance,
    ensure_loopback,
    wait_for_service_shutdown,
)
from striatum.service_state import ServiceState

JsonObject = dict[str, Any]
HandlerFactory = Callable[[ServiceState], type[BaseHTTPRequestHandler]]
StartupVerifier = Callable[[Path], None]


class ThreadedTCPServer(socketserver.ThreadingMixIn, HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


class ThreadedUnixServer(socketserver.ThreadingMixIn, HTTPServer):
    daemon_threads = True
    address_family = socket.AF_UNIX

    def server_bind(self) -> None:  # noqa: D401 - HTTPServer API
        socketserver.UnixStreamServer.server_bind(self)


def run_tcp(
    *,
    repo: Path,
    host: str,
    port: int,
    token: str | None,
    allow_mutations: bool,
    idle_timeout_seconds: int | None,
    web_enabled: bool,
    make_handler: HandlerFactory,
    verify_startup: StartupVerifier,
    shutdown_drain_seconds: float,
) -> JsonObject:
    ensure_loopback(host)
    pid_path = repo / ".striatum" / "service.pid"
    check_single_instance(pid_path)
    verify_startup(repo)
    state = ServiceState(
        repo=repo,
        allow_mutations=allow_mutations,
        token=token,
        web_enabled=web_enabled,
    )
    handler = make_handler(state)
    server = ThreadedTCPServer((host, port), handler)
    bound_address = server.server_address
    bound_host = bound_address[0] if isinstance(bound_address, tuple) else host
    bound_port = bound_address[1] if isinstance(bound_address, tuple) else port
    state.origin_check_enabled = True
    state.allowed_origins = allowed_origins_for_bind(str(bound_host), int(bound_port))
    return serve_forever(
        server=server,
        state=state,
        pid_path=pid_path,
        unix_path=None,
        bind_summary={"mode": "tcp", "host": str(bound_host), "port": int(bound_port)},
        idle_timeout_seconds=idle_timeout_seconds,
        web_enabled=web_enabled,
        shutdown_drain_seconds=shutdown_drain_seconds,
    )


def run_unix(
    *,
    repo: Path,
    unix_path: str,
    token: str | None,
    allow_mutations: bool,
    idle_timeout_seconds: int | None,
    web_enabled: bool,
    make_handler: HandlerFactory,
    verify_startup: StartupVerifier,
    shutdown_drain_seconds: float,
) -> JsonObject:
    verify_startup(repo)
    socket_path = Path(unix_path)
    if socket_path.exists():
        try:
            socket_path.unlink()
        except OSError as exc:
            raise ServiceConfigError(
                f"cannot remove stale Unix socket {socket_path}: {exc}"
            ) from exc
    pid_path = socket_path.with_suffix(socket_path.suffix + ".pid")
    check_single_instance(pid_path)
    state = ServiceState(
        repo=repo,
        allow_mutations=allow_mutations,
        token=token,
        web_enabled=web_enabled,
    )
    handler = make_handler(state)
    # ThreadedUnixServer overrides address_family to AF_UNIX and binds
    # by string path; mypy's HTTPServer signature doesn't model that
    # variant, hence the cast to Any to suppress the spurious tuple
    # expectation.
    server: Any = ThreadedUnixServer(str(socket_path), handler)  # type: ignore[arg-type]
    os.chmod(socket_path, 0o600)
    return serve_forever(
        server=server,
        state=state,
        pid_path=pid_path,
        unix_path=str(socket_path),
        bind_summary={"mode": "unix", "path": str(socket_path)},
        idle_timeout_seconds=idle_timeout_seconds,
        web_enabled=web_enabled,
        shutdown_drain_seconds=shutdown_drain_seconds,
    )


def serve_forever(
    *,
    server: HTTPServer,
    state: ServiceState,
    pid_path: Path,
    unix_path: str | None,
    bind_summary: JsonObject,
    idle_timeout_seconds: int | None,
    web_enabled: bool,
    shutdown_drain_seconds: float,
) -> JsonObject:
    pid_path.parent.mkdir(parents=True, exist_ok=True)
    pid_path.write_text(str(os.getpid()), encoding="utf-8")

    shutdown_event = threading.Event()

    def _on_signal(signum: int, frame: Any) -> None:  # noqa: ARG001
        # Run from the main thread when CPython delivers the signal. We
        # avoid calling ``server.shutdown()`` here because it would
        # block waiting for ``serve_forever`` to acknowledge; the main
        # thread waits on the event below and shuts down synchronously.
        state.signal_shutdown()
        shutdown_event.set()

    signal.signal(signal.SIGTERM, _on_signal)
    signal.signal(signal.SIGINT, _on_signal)

    server_thread = threading.Thread(target=server.serve_forever, daemon=True)
    server_thread.start()
    startup = {
        **bind_summary,
        "allow_mutations": state.allow_mutations,
        "token": state.token is not None,
        "started_at": state.started_at,
        "pid": os.getpid(),
        "web_enabled": web_enabled,
    }
    try:
        wait_for_service_shutdown(
            shutdown_event,
            idle_timeout_seconds=idle_timeout_seconds,
        )
        server.shutdown()
        server_thread.join(timeout=shutdown_drain_seconds)
    finally:
        try:
            server.server_close()
        except OSError:
            pass
        if unix_path is not None:
            try:
                Path(unix_path).unlink()
            except OSError:
                pass
        try:
            pid_path.unlink()
        except OSError:
            pass
    return {"started": True, **startup}

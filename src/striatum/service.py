"""RFC 0012 V1: local HTTP + Unix-socket service.

Operationalizes D006's promise of an "optional Unix-socket / local HTTP
API later for Slack, TUI, and web adapters." Every endpoint that mutates
state delegates to ``striatum.api.invoke``; the events table is read
directly only for the SSE stream. Localhost-only by default; non-loopback
hosts are refused at startup. Mutations are gated behind
``--allow-mutations``.
"""

from __future__ import annotations

import hmac
import json
import os
import signal
import socket
import socketserver
import sqlite3
import threading
import time
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any
from urllib.parse import parse_qs, urlsplit

from striatum.api import invoke
from striatum.db import db_path

JsonObject = dict[str, Any]

LOOPBACK_HOSTS = frozenset({"127.0.0.1", "localhost", "::1"})

SSE_POLL_INTERVAL_SECONDS = 0.25
SSE_MAX_CONCURRENT_PER_RUN = 32
SHUTDOWN_DRAIN_SECONDS = 5.0

# Top-level CLI verbs whose all subcommands are reads. Subcommand-aware
# whitelists for the four mixed parents follow.
SERVICE_READ_TOP_COMMANDS = frozenset({
    "status",
    "why",
    "doctor",
    "list",
    "evidence",
    "dashboard",
})

SERVICE_READ_SUBCOMMANDS: dict[str, frozenset[str]] = {
    "workflow": frozenset({"validate", "plan", "graph"}),
    "supervise": frozenset({"status", "list"}),
    "worktree": frozenset({"list"}),
    "run": frozenset({"summary", "graph"}),
    "recovery": frozenset({"stale-leases"}),
}


def is_read_command(argv: list[str]) -> bool:
    """Return True when ``argv`` resolves to a known read-only command.

    The whitelist approach is conservative: any command not explicitly
    listed is treated as a mutation. Future mutating verbs default to
    blocked when ``--allow-mutations`` is off.
    """
    if not argv:
        return False
    top = argv[0]
    if top in SERVICE_READ_TOP_COMMANDS:
        return True
    if top in SERVICE_READ_SUBCOMMANDS and len(argv) >= 2:
        return argv[1] in SERVICE_READ_SUBCOMMANDS[top]
    return False


def tokens_match(provided: str, expected: str) -> bool:
    """Constant-time token comparison that masks length differences.

    ``hmac.compare_digest`` short-circuits on length mismatch, leaking
    the expected length through wall-clock time. Padding both sides to
    a fixed minimum and explicitly comparing lengths after the
    constant-time digest avoids the leak (design-review F1).
    """
    p = provided.encode("utf-8")
    e = expected.encode("utf-8")
    target = max(len(p), len(e), 64)
    p_padded = p.ljust(target, b"\x00")
    e_padded = e.ljust(target, b"\x00")
    return hmac.compare_digest(p_padded, e_padded) and len(p) == len(e)


def utcnow_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


class ServiceState:
    """Shared service state.

    Owns the repo path, mutation flag, optional token, the started_at
    timestamp, and the per-run SSE counter (for the cap of 32 concurrent
    streams per run).
    """

    def __init__(
        self,
        *,
        repo: Path,
        allow_mutations: bool,
        token: str | None,
        web_enabled: bool,
    ) -> None:
        self.repo = repo
        self.allow_mutations = allow_mutations
        self.token = token
        self.web_enabled = web_enabled
        self.started_at = utcnow_iso()
        self._sse_counts: dict[str, int] = {}
        self._sse_lock = threading.Lock()
        self._shutdown = threading.Event()

    def acquire_sse_slot(self, run_id: str) -> bool:
        with self._sse_lock:
            current = self._sse_counts.get(run_id, 0)
            if current >= SSE_MAX_CONCURRENT_PER_RUN:
                return False
            self._sse_counts[run_id] = current + 1
        return True

    def release_sse_slot(self, run_id: str) -> None:
        with self._sse_lock:
            self._sse_counts[run_id] = max(0, self._sse_counts.get(run_id, 1) - 1)

    @property
    def shutting_down(self) -> bool:
        return self._shutdown.is_set()

    def signal_shutdown(self) -> None:
        self._shutdown.set()


class StriatumServiceHandler(BaseHTTPRequestHandler):
    """HTTP request handler for the local service.

    Endpoints route through ``striatum.api.invoke`` for everything except
    SSE event streaming, which reads the events table directly via a
    dedicated read connection.
    """

    server_version = "Striatum-Service/1"
    state: ServiceState  # set on the server instance

    # Suppress BaseHTTPRequestHandler's stderr access log (D028: no
    # request bodies / response payloads logged to disk).
    def log_message(self, format: str, *args: Any) -> None:  # noqa: A002
        return

    # --- routing --------------------------------------------------------

    def do_GET(self) -> None:  # noqa: N802 (BaseHTTPRequestHandler API)
        try:
            self._dispatch_get()
        except BrokenPipeError:
            return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def do_POST(self) -> None:  # noqa: N802
        try:
            self._dispatch_post()
        except BrokenPipeError:
            return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _dispatch_get(self) -> None:
        if not self._authenticate():
            return
        parsed = urlsplit(self.path)
        path = parsed.path
        query = parse_qs(parsed.query)
        if path == "/v1/health":
            self._handle_health()
            return
        if path == "/v1/runs":
            self._handle_invoke(["status"])
            return
        if path == "/v1/doctor":
            self._handle_doctor(query)
            return
        if path.startswith("/v1/runs/"):
            self._handle_run_subpath(path[len("/v1/runs/"):], query)
            return
        if path == "/" or path == "":
            if self.state.web_enabled:
                self._send_json(404, {"ok": False, "error": {"code": 404, "message": "web UI assets not bundled in V1; pass --web with RFC 0013 build to enable"}})
            else:
                self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})
            return
        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})

    def _dispatch_post(self) -> None:
        if not self._authenticate():
            return
        parsed = urlsplit(self.path)
        if parsed.path != "/v1/invoke":
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})
            return
        body = self._read_json_body()
        if body is None:
            return
        argv = body.get("argv")
        if not isinstance(argv, list) or not all(isinstance(part, str) for part in argv):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "argv must be a list of strings"}})
            return
        if not self.state.allow_mutations and not is_read_command(argv):
            verb = " ".join(argv[:2]) if argv else ""
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": f"command requires --allow-mutations: {verb}"}})
            return
        result = invoke(argv, repo=self.state.repo)
        status = 200 if result.get("ok") else 500
        if not result.get("ok"):
            err = result.get("error") or {}
            code = err.get("code")
            if isinstance(code, int):
                if code in (400, 401, 403, 404, 405, 409):
                    status = code
                elif code in (3, 4, 5, 6, 7, 8):
                    status = 400
        self._send_json(status, result)

    # --- endpoint helpers ----------------------------------------------

    def _handle_health(self) -> None:
        self._send_json(
            200,
            {
                "ok": True,
                "data": {
                    "started_at": self.state.started_at,
                    "version": _striatum_version(),
                    "mode": _service_mode(self.server),
                },
            },
        )

    def _handle_invoke(self, argv: list[str]) -> None:
        result = invoke(argv, repo=self.state.repo)
        status = 200 if result.get("ok") else 500
        self._send_json(status, result)

    def _handle_doctor(self, query: dict[str, list[str]]) -> None:
        argv = ["doctor", "--verbose"]
        run_id = query.get("run_id", [None])[0]
        if run_id:
            argv.extend(["--run-id", run_id])
        self._handle_invoke(argv)

    def _handle_run_subpath(self, suffix: str, query: dict[str, list[str]]) -> None:
        parts = suffix.split("/")
        run_id = parts[0]
        if not run_id:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "missing run_id"}})
            return
        if len(parts) == 1:
            self._handle_invoke(["status", "--run-id", run_id])
            return
        sub = parts[1]
        if sub == "why":
            target = query.get("id", [None])[0]
            if not target:
                self._send_json(400, {"ok": False, "error": {"code": 400, "message": "missing ?id=<entity>"}})
                return
            self._handle_invoke(["why", target])
            return
        if sub == "dashboard":
            self._handle_invoke(["dashboard", "--run-id", run_id, "--once"])
            return
        if sub == "events":
            since = self._sse_since(query)
            self._stream_events(run_id, since=since)
            return
        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})

    # --- SSE -----------------------------------------------------------

    def _sse_since(self, query: dict[str, list[str]]) -> int:
        # ``Last-Event-ID`` header takes precedence per the synthesis.
        header = self.headers.get("Last-Event-ID")
        if header:
            try:
                return max(0, int(header))
            except ValueError:
                pass
        raw = query.get("since", [None])[0]
        if raw:
            try:
                return max(0, int(raw))
            except ValueError:
                return 0
        return 0

    def _stream_events(self, run_id: str, *, since: int) -> None:
        if not self.state.acquire_sse_slot(run_id):
            self._send_json(429, {"ok": False, "error": {"code": 429, "message": f"too many concurrent SSE streams for run {run_id}"}})
            return
        conn: sqlite3.Connection | None = None
        try:
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.send_header("Connection", "close")
            self.send_header("X-Accel-Buffering", "no")
            self.end_headers()
            conn = sqlite3.connect(str(db_path(self.state.repo)))
            conn.row_factory = sqlite3.Row
            last_id = since
            terminal_states = {"completed", "failed", "canceled"}
            while not self.state.shutting_down:
                rows = conn.execute(
                    "SELECT * FROM events WHERE run_id = ? AND event_id > ? ORDER BY event_id LIMIT 100",
                    (run_id, last_id),
                ).fetchall()
                for row in rows:
                    payload = {
                        "event_id": int(row["event_id"]),
                        "run_id": row["run_id"],
                        "type": row["event_type"],
                        "actor_session_id": row["actor_session_id"],
                        "job_id": row["job_id"],
                        "payload": json.loads(row["payload_json"] or "{}"),
                        "created_at": row["created_at"],
                    }
                    self._write_sse_event("striatum.event", row["event_id"], payload)
                    last_id = int(row["event_id"])
                run_state = conn.execute(
                    "SELECT state FROM runs WHERE run_id = ?",
                    (run_id,),
                ).fetchone()
                if run_state is not None and run_state["state"] in terminal_states and not rows:
                    self._write_sse_event(
                        "striatum.run_terminal",
                        last_id,
                        {"run_id": run_id, "state": run_state["state"]},
                    )
                    break
                time.sleep(SSE_POLL_INTERVAL_SECONDS)
            if self.state.shutting_down:
                self._write_sse_event(
                    "striatum.shutdown",
                    last_id,
                    {"run_id": run_id, "reason": "service_shutting_down"},
                )
        except (BrokenPipeError, ConnectionResetError):
            return
        finally:
            if conn is not None:
                conn.close()
            self.state.release_sse_slot(run_id)

    def _write_sse_event(self, event: str, event_id: int, payload: JsonObject) -> None:
        body = (
            f"event: {event}\n"
            f"id: {event_id}\n"
            f"data: {json.dumps(payload)}\n\n"
        )
        self.wfile.write(body.encode("utf-8"))
        self.wfile.flush()

    # --- request helpers ----------------------------------------------

    def _authenticate(self) -> bool:
        if self.state.token is None:
            return True
        header = self.headers.get("Authorization", "")
        prefix = "Bearer "
        if not header.startswith(prefix):
            self._send_json(401, {"ok": False, "error": {"code": 401, "message": "missing or invalid Authorization header"}})
            return False
        provided = header[len(prefix):]
        if not tokens_match(provided, self.state.token):
            self._send_json(401, {"ok": False, "error": {"code": 401, "message": "invalid token"}})
            return False
        return True

    def _read_json_body(self) -> JsonObject | None:
        length_header = self.headers.get("Content-Length")
        if not length_header:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "missing Content-Length"}})
            return None
        try:
            length = int(length_header)
        except ValueError:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid Content-Length"}})
            return None
        raw = self.rfile.read(length).decode("utf-8")
        try:
            parsed = json.loads(raw)
        except json.JSONDecodeError:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "request body must be JSON"}})
            return None
        if not isinstance(parsed, dict):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "request body must be a JSON object"}})
            return None
        return parsed

    def _send_json(self, status: int, payload: JsonObject) -> None:
        try:
            body = (json.dumps(payload) + "\n").encode("utf-8")
        except (TypeError, ValueError):
            body = b'{"ok":false,"error":{"code":500,"message":"json encoding failed"}}\n'
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Connection", "close")
        self.end_headers()
        try:
            self.wfile.write(body)
        except BrokenPipeError:
            return


def _striatum_version() -> str:
    try:
        from importlib.metadata import version as _meta_version

        return _meta_version("striatum")
    except Exception:  # noqa: BLE001
        return "unknown"


def _service_mode(server: Any) -> str:
    sock = getattr(server, "socket", None)
    if sock is None:
        return "unknown"
    if sock.family == socket.AF_UNIX:
        return "unix"
    return "tcp"


# --- server classes ----------------------------------------------------


class _ThreadedTCPServer(socketserver.ThreadingMixIn, HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


class _ThreadedUnixServer(socketserver.ThreadingMixIn, HTTPServer):
    daemon_threads = True
    address_family = socket.AF_UNIX

    def server_bind(self) -> None:  # noqa: D401 - HTTPServer API
        socketserver.UnixStreamServer.server_bind(self)


# --- public API --------------------------------------------------------


def run_service(
    *,
    repo: Path,
    host: str | None,
    port: int | None,
    unix_path: str | None,
    token: str | None,
    allow_mutations: bool,
    idle_timeout_seconds: int | None,
    web_enabled: bool,
) -> JsonObject:
    """Boot the local service. Returns the startup envelope.

    The function blocks until SIGTERM / SIGINT (or idle timeout). Returns
    a ``data`` envelope describing the bound address, mode, and PID.
    """
    if unix_path is not None:
        return _run_unix(
            repo=repo,
            unix_path=unix_path,
            token=token,
            allow_mutations=allow_mutations,
            idle_timeout_seconds=idle_timeout_seconds,
            web_enabled=web_enabled,
        )
    return _run_tcp(
        repo=repo,
        host=host or "127.0.0.1",
        port=port if port is not None else 0,
        token=token,
        allow_mutations=allow_mutations,
        idle_timeout_seconds=idle_timeout_seconds,
        web_enabled=web_enabled,
    )


def _ensure_loopback(host: str) -> None:
    if host in LOOPBACK_HOSTS:
        return
    raise ServiceConfigError(
        f"refusing to bind to non-loopback host {host!r}; allowed: "
        f"{sorted(LOOPBACK_HOSTS)}"
    )


def _run_tcp(
    *,
    repo: Path,
    host: str,
    port: int,
    token: str | None,
    allow_mutations: bool,
    idle_timeout_seconds: int | None,
    web_enabled: bool,
) -> JsonObject:
    _ensure_loopback(host)
    pid_path = repo / ".striatum" / "service.pid"
    _check_single_instance(pid_path)
    state = ServiceState(
        repo=repo,
        allow_mutations=allow_mutations,
        token=token,
        web_enabled=web_enabled,
    )
    handler = _make_handler(state)
    server = _ThreadedTCPServer((host, port), handler)
    bound_address = server.server_address
    bound_host = bound_address[0] if isinstance(bound_address, tuple) else host
    bound_port = bound_address[1] if isinstance(bound_address, tuple) else port
    return _serve_forever(
        server=server,
        state=state,
        pid_path=pid_path,
        unix_path=None,
        bind_summary={"mode": "tcp", "host": str(bound_host), "port": int(bound_port)},
        idle_timeout_seconds=idle_timeout_seconds,
        web_enabled=web_enabled,
    )


def _run_unix(
    *,
    repo: Path,
    unix_path: str,
    token: str | None,
    allow_mutations: bool,
    idle_timeout_seconds: int | None,
    web_enabled: bool,
) -> JsonObject:
    socket_path = Path(unix_path)
    if socket_path.exists():
        try:
            socket_path.unlink()
        except OSError as exc:
            raise ServiceConfigError(
                f"cannot remove stale Unix socket {socket_path}: {exc}"
            ) from exc
    pid_path = socket_path.with_suffix(socket_path.suffix + ".pid")
    _check_single_instance(pid_path)
    state = ServiceState(
        repo=repo,
        allow_mutations=allow_mutations,
        token=None,
        web_enabled=web_enabled,
    )
    handler = _make_handler(state)
    # _ThreadedUnixServer overrides address_family to AF_UNIX and binds
    # by string path; mypy's HTTPServer signature doesn't model that
    # variant, hence the cast to Any to suppress the spurious tuple
    # expectation.
    server: Any = _ThreadedUnixServer(str(socket_path), handler)  # type: ignore[arg-type]
    os.chmod(socket_path, 0o600)
    return _serve_forever(
        server=server,
        state=state,
        pid_path=pid_path,
        unix_path=str(socket_path),
        bind_summary={"mode": "unix", "path": str(socket_path)},
        idle_timeout_seconds=idle_timeout_seconds,
        web_enabled=web_enabled,
    )


def _make_handler(state: ServiceState) -> type[StriatumServiceHandler]:
    class _Bound(StriatumServiceHandler):
        pass

    _Bound.state = state
    return _Bound


def _check_single_instance(pid_path: Path) -> None:
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


def _serve_forever(
    *,
    server: HTTPServer,
    state: ServiceState,
    pid_path: Path,
    unix_path: str | None,
    bind_summary: JsonObject,
    idle_timeout_seconds: int | None,
    web_enabled: bool,
) -> JsonObject:
    pid_path.parent.mkdir(parents=True, exist_ok=True)
    pid_path.write_text(str(os.getpid()), encoding="utf-8")

    shutdown_event = threading.Event()

    def _on_signal(signum: int, frame: Any) -> None:  # noqa: ARG001
        # Run from the main thread when CPython delivers the signal. We
        # avoid calling ``server.shutdown()`` here because it would
        # block waiting for ``serve_forever`` to acknowledge — instead
        # the main thread waits on the event below and calls shutdown
        # synchronously after the event fires.
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
    if web_enabled:
        startup["web_warning"] = (
            "--web flag accepted but the web UI is not yet bundled "
            "(RFC 0013 not implemented); / will return 404"
        )
    try:
        if idle_timeout_seconds is None:
            shutdown_event.wait()
        else:
            shutdown_event.wait(timeout=idle_timeout_seconds)
        # Either signal-driven shutdown or idle timeout. Shut the server
        # synchronously now; serve_forever's poll loop will pick up the
        # request within its poll interval (default 0.5s).
        server.shutdown()
        server_thread.join(timeout=SHUTDOWN_DRAIN_SECONDS)
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


# --- exception types ---------------------------------------------------


class ServiceConfigError(Exception):
    """Raised at startup for refusing to bind a non-loopback host or
    similar config errors. Mapped to exit 8 by the CLI dispatcher."""


class ServiceAlreadyRunningError(Exception):
    """Raised when a PID file points at a live process. Mapped to
    exit 7 by the CLI dispatcher."""

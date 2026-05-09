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
import uuid
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


def _is_safe_id(value: str) -> bool:
    """RFC 0023 V1: chat session ids and similar paths must be ASCII
    alphanumeric / underscore / hyphen only."""
    if not value:
        return False
    return all(ch.isalnum() or ch in "-_" for ch in value)


def _utc_now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _format_ts(epoch_seconds: float) -> str:
    return datetime.fromtimestamp(epoch_seconds, tz=timezone.utc).strftime("%Y-%m-%d %H:%M:%S")


def _escape_html(s: str) -> str:
    return (
        s.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace('"', "&quot;")
        .replace("'", "&#x27;")
    )


def _append_jsonl(path: Path, entry: dict[str, Any]) -> None:
    line = json.dumps(entry, ensure_ascii=False) + "\n"
    with path.open("a", encoding="utf-8") as fh:
        fh.write(line)


def _read_chat_history(path: Path) -> list[dict[str, str]]:
    """Read the transcript JSONL and project to a chat-completion
    messages list (only role/content; coalesce streaming chunks)."""
    if not path.is_file():
        return []
    coalesced: list[dict[str, str]] = []
    pending_assistant: list[str] = []
    for raw in path.read_text(encoding="utf-8").splitlines():
        if not raw.strip():
            continue
        try:
            entry = json.loads(raw)
        except json.JSONDecodeError:
            continue
        role = str(entry.get("role") or "")
        content = str(entry.get("content") or "")
        if not role or not content:
            continue
        if role == "assistant":
            if entry.get("streaming") is True:
                pending_assistant.append(content)
                continue
            if pending_assistant:
                # The non-streaming entry is the canonical full message.
                pending_assistant = []
            coalesced.append({"role": "assistant", "content": content})
            continue
        # Flush any pending streaming chunks before switching role.
        if pending_assistant:
            coalesced.append({"role": "assistant", "content": "".join(pending_assistant)})
            pending_assistant = []
        if role in ("user", "system"):
            coalesced.append({"role": role, "content": content})
    if pending_assistant:
        coalesced.append({"role": "assistant", "content": "".join(pending_assistant)})
    return coalesced


def _parse_simple_multipart(raw: str, ctype: str) -> dict[str, list[str]]:
    """Very small multipart/form-data parser sufficient for the one-
    field chat-input form. Only handles UTF-8 text fields."""
    boundary_marker = "boundary="
    if boundary_marker not in ctype:
        return {}
    boundary = "--" + ctype.split(boundary_marker, 1)[1].split(";", 1)[0].strip()
    out: dict[str, list[str]] = {}
    parts = raw.split(boundary)
    for part in parts:
        part = part.strip("\r\n -")
        if not part or part == "--":
            continue
        if "\r\n\r\n" not in part:
            continue
        head, body = part.split("\r\n\r\n", 1)
        headers = head.splitlines()
        name: str | None = None
        for hdr in headers:
            if hdr.lower().startswith("content-disposition:"):
                for kv in hdr.split(";"):
                    kv = kv.strip()
                    if kv.startswith("name="):
                        name = kv.split("=", 1)[1].strip().strip('"')
        if not name:
            continue
        out.setdefault(name, []).append(body.rstrip("\r\n"))
    return out


def _jinja_env() -> Any:
    """Return a cached Jinja2 environment that loads templates from the
    ``striatum.web.templates`` package.

    RFC 0022 V1: server-rendered multi-page UI uses Jinja2; the
    environment is constructed lazily and cached on the function via
    ``functools.lru_cache``.
    """
    return _jinja_env_factory()


def _jinja_env_factory() -> Any:
    from functools import lru_cache

    @lru_cache(maxsize=1)
    def _build() -> Any:
        from jinja2 import Environment, PackageLoader, select_autoescape
        return Environment(
            loader=PackageLoader("striatum.web", "templates"),
            autoescape=select_autoescape(["html"]),
            keep_trailing_newline=False,
        )

    return _build()

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
        if path.startswith("/v1/artifacts/") and path.endswith("/raw"):
            artifact_id = path[len("/v1/artifacts/"):-len("/raw")]
            self._handle_artifact_raw(artifact_id)
            return
        if self.state.web_enabled and (path == "/" or path == ""):
            self._render_run_list_page()
            return
        if self.state.web_enabled and path == "/doctor":
            self._render_doctor_page()
            return
        if self.state.web_enabled and path == "/chat":
            self._render_chat_index_page()
            return
        if self.state.web_enabled and path.startswith("/chat/"):
            self._render_chat_subpath(path[len("/chat/"):])
            return
        if self.state.web_enabled and path.startswith("/view/"):
            self._render_view_path(path[len("/view/"):])
            return
        if self.state.web_enabled and path.startswith("/run/"):
            self._render_run_subpath(path[len("/run/"):])
            return
        if self.state.web_enabled and path.startswith("/static/"):
            relative = path[len("/static/"):]
            self._serve_static_asset(relative)
            return
        if path == "/" or path == "":
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found; pass --web to enable the local UI (RFC 0013 V1)"}})
            return
        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})

    def _dispatch_post(self) -> None:
        if not self._authenticate():
            return
        parsed = urlsplit(self.path)
        if self.state.web_enabled and parsed.path == "/chat/new":
            self._handle_chat_new()
            return
        if self.state.web_enabled and parsed.path.startswith("/chat/") and parsed.path.endswith("/send"):
            session_id = parsed.path[len("/chat/"):-len("/send")]
            self._handle_chat_send(session_id)
            return
        if self.state.web_enabled and parsed.path.startswith("/chat/") and parsed.path.endswith("/stop"):
            session_id = parsed.path[len("/chat/"):-len("/stop")]
            self._handle_chat_stop(session_id)
            return
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
                    # RFC 0013 step 7: SPA reads this to decide whether
                    # to render mutation buttons. The runner-side gate
                    # in _dispatch_post is still authoritative; this
                    # field is the SPA's hint, not a security boundary.
                    "allow_mutations": bool(self.state.allow_mutations),
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
        if sub == "artifacts":
            # RFC 0013 step 7 follow-up: run-level artifact rollup so the
            # SPA can show every published artifact for a run without
            # navigating per-job. Wraps the existing read-only `list
            # artifacts` verb.
            self._handle_invoke(["list", "artifacts", "--run-id", run_id])
            return
        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})

    def _handle_artifact_raw(self, artifact_id: str) -> None:
        """RFC 0013 V1: serve the raw bytes of an artifact for the web UI viewer.

        Looks up the artifact row, opens the file at ``repo_path``, and streams
        the bytes back. Read-only; no mutation gate. Returns 404 if the row or
        the file is missing.
        """
        if not artifact_id:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "missing artifact id"}})
            return
        conn: sqlite3.Connection | None = None
        try:
            conn = sqlite3.connect(str(db_path(self.state.repo)))
            conn.row_factory = sqlite3.Row
            row = conn.execute(
                "SELECT artifact_kind, repo_path FROM artifacts WHERE artifact_id = ?",
                (artifact_id,),
            ).fetchone()
        except sqlite3.Error as exc:
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})
            return
        finally:
            if conn is not None:
                conn.close()
        if row is None:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "artifact not found"}})
            return
        repo_path = self.state.repo / str(row["repo_path"])
        if not repo_path.is_file():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "artifact file missing on disk"}})
            return
        try:
            data = repo_path.read_bytes()
        except OSError as exc:
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})
            return
        # Choose a safe content-type. The web UI handles rendering; the
        # service just streams bytes.
        suffix = repo_path.suffix.lower()
        content_type = {
            ".md": "text/markdown; charset=utf-8",
            ".markdown": "text/markdown; charset=utf-8",
            ".json": "application/json",
            ".txt": "text/plain; charset=utf-8",
        }.get(suffix, "application/octet-stream")
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Content-Security-Policy", "default-src 'none'")
        self.send_header("Connection", "close")
        self.end_headers()
        try:
            self.wfile.write(data)
        except BrokenPipeError:
            return

    # --- RFC 0022 V1 page rendering -----------------------------------

    def _render_run_list_page(self) -> None:
        """Server-side render the run-list page."""
        try:
            with sqlite3.connect(str(db_path(self.state.repo))) as conn:
                conn.row_factory = sqlite3.Row
                rows = conn.execute(
                    "SELECT run_id, state, branch_name, created_at "
                    "FROM runs ORDER BY created_at DESC"
                ).fetchall()
                runs = [dict(row) for row in rows]
            html = _jinja_env().get_template("run_list.html").render(runs=runs)
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_run_subpath(self, subpath: str) -> None:
        """Dispatch /run/<run_id> + /run/<run_id>/job/<id> + /run/<run_id>/artifact/<id>."""
        if not subpath:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "missing run id"}})
            return
        parts = subpath.strip("/").split("/")
        run_id = parts[0]
        if any(c in run_id for c in ("..", "/", "\x00")):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid run id"}})
            return
        if len(parts) == 1:
            self._render_run_detail_page(run_id)
            return
        if len(parts) == 3 and parts[1] == "job":
            self._render_job_detail_page(run_id, parts[2])
            return
        if len(parts) == 3 and parts[1] == "artifact":
            self._render_artifact_view_page(run_id, parts[2])
            return
        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})

    def _render_run_detail_page(self, run_id: str) -> None:
        from striatum.cli.introspect import status as status_command
        from striatum.web.graph_svg import (
            compute_node_states_from_jobs,
            render_run_graph,
        )
        try:
            with sqlite3.connect(str(db_path(self.state.repo))) as conn:
                conn.row_factory = sqlite3.Row
                run_row = conn.execute(
                    "SELECT * FROM runs WHERE run_id = ?", (run_id,)
                ).fetchone()
                if run_row is None:
                    self._send_json(404, {"ok": False, "error": {"code": 404, "message": "run not found"}})
                    return
                run = dict(run_row)
                job_rows = conn.execute(
                    "SELECT * FROM jobs WHERE run_id = ? ORDER BY workflow_job_id",
                    (run_id,),
                ).fetchall()
                jobs = [dict(row) for row in job_rows]
                snapshot_row = conn.execute(
                    "SELECT workflow_json FROM workflow_snapshots WHERE workflow_snapshot_id = ?",
                    (str(run["workflow_snapshot_id"]),),
                ).fetchone()
                workflow = (
                    json.loads(str(snapshot_row["workflow_json"]))
                    if snapshot_row is not None else {}
                )
                status_payload = status_command(conn, run_id=run_id)
            node_states = compute_node_states_from_jobs(jobs)
            graph_svg = render_run_graph(workflow, node_states, run_id=run_id)
            html = _jinja_env().get_template("run_detail.html").render(
                run=run,
                jobs=jobs,
                graph_svg=graph_svg,
                next_actions=status_payload.get("next_actions") or [],
                verdicts_by_posture=status_payload.get("verdicts_by_posture") or {},
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_job_detail_page(self, run_id: str, job_id: str) -> None:
        from striatum.cli.introspect import latest_verdict_row
        try:
            with sqlite3.connect(str(db_path(self.state.repo))) as conn:
                conn.row_factory = sqlite3.Row
                run_row = conn.execute(
                    "SELECT * FROM runs WHERE run_id = ?", (run_id,)
                ).fetchone()
                job_row = conn.execute(
                    "SELECT * FROM jobs WHERE job_id = ? AND run_id = ?",
                    (job_id, run_id),
                ).fetchone()
                if run_row is None or job_row is None:
                    self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})
                    return
                run = dict(run_row)
                job = dict(job_row)
                artifact_rows = conn.execute(
                    "SELECT * FROM artifacts WHERE job_id = ? ORDER BY created_at",
                    (job_id,),
                ).fetchall()
                artifacts = [dict(row) for row in artifact_rows]
                latest_verdict = latest_verdict_row(conn, job_id=job_id)
            html = _jinja_env().get_template("job_detail.html").render(
                run=run, job=job, artifacts=artifacts, latest_verdict=latest_verdict,
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_artifact_view_page(self, run_id: str, artifact_id: str) -> None:
        try:
            with sqlite3.connect(str(db_path(self.state.repo))) as conn:
                conn.row_factory = sqlite3.Row
                run_row = conn.execute(
                    "SELECT * FROM runs WHERE run_id = ?", (run_id,)
                ).fetchone()
                artifact_row = conn.execute(
                    "SELECT * FROM artifacts WHERE artifact_id = ? AND run_id = ?",
                    (artifact_id, run_id),
                ).fetchone()
                if run_row is None or artifact_row is None:
                    self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})
                    return
                run = dict(run_row)
                artifact = dict(artifact_row)
            # RFC 0023 V1: inline-render Markdown artifacts.
            rendered_md: str | None = None
            body_text: str | None = None
            try:
                repo_path = artifact.get("repo_path") or ""
                if isinstance(repo_path, str) and repo_path.endswith(".md"):
                    full = (self.state.repo / repo_path).resolve()
                    repo_root = self.state.repo.resolve()
                    full.relative_to(repo_root)  # raises if escapes
                    if full.is_file():
                        from striatum.web.markdown import render as md_render
                        body = full.read_text(encoding="utf-8", errors="replace")
                        rendered_md = md_render(body)
            except (ValueError, OSError):
                rendered_md = None
            html = _jinja_env().get_template("artifact_view.html").render(
                run=run, artifact=artifact,
                rendered_md=rendered_md, body_text=body_text,
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_doctor_page(self) -> None:
        from striatum.cli.introspect import doctor as doctor_command
        try:
            with sqlite3.connect(str(db_path(self.state.repo))) as conn:
                conn.row_factory = sqlite3.Row
                doctor_payload = doctor_command(conn, repo=self.state.repo, run_id=None)
            html = _jinja_env().get_template("doctor.html").render(doctor=doctor_payload)
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    # --- RFC 0023 V1 chat + view ---------------------------------------

    def _chat_config(self) -> Any | None:
        from striatum.web.chat_provider import ChatProviderConfig, ChatProviderError
        try:
            return ChatProviderConfig.from_env(os.environ)
        except ChatProviderError:
            return None

    def _chat_scratch_root(self) -> Path:
        return self.state.repo / ".striatum" / "scratch"

    def _chat_session_path(self, session_id: str) -> Path | None:
        if not session_id or "/" in session_id or ".." in session_id:
            return None
        return self._chat_scratch_root() / f"chat-{session_id}" / "transcript.jsonl"

    def _list_chat_sessions(self) -> list[dict[str, Any]]:
        root = self._chat_scratch_root()
        if not root.exists():
            return []
        sessions = []
        for entry in sorted(root.iterdir()):
            if not entry.is_dir() or not entry.name.startswith("chat-"):
                continue
            transcript = entry / "transcript.jsonl"
            if not transcript.is_file():
                continue
            session_id = entry.name[len("chat-"):]
            try:
                lines = transcript.read_text(encoding="utf-8").splitlines()
                count = sum(1 for line in lines if line.strip())
                started_at = entry.stat().st_mtime
            except OSError:
                continue
            sessions.append(
                {"id": session_id, "message_count": count, "started_at": _format_ts(started_at)}
            )
        return sessions

    def _render_chat_index_page(self) -> None:
        config = self._chat_config()
        sessions = self._list_chat_sessions() if config else []
        try:
            html = _jinja_env().get_template("chat_index.html").render(
                chat_configured=config is not None,
                sessions=sessions,
                model=getattr(config, "model", None),
                flavor=getattr(config, "flavor", None),
                mutations_allowed=self.state.allow_mutations,
                env_help="",
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_chat_subpath(self, subpath: str) -> None:
        if not subpath:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "missing chat session"}})
            return
        parts = subpath.strip("/").split("/")
        session_id = parts[0]
        if not _is_safe_id(session_id):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid chat session id"}})
            return
        if len(parts) == 1:
            self._render_chat_session_page(session_id)
            return
        if len(parts) == 2 and parts[1] == "events":
            self._stream_chat_events(session_id)
            return
        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})

    def _render_chat_session_page(self, session_id: str) -> None:
        from striatum.web.markdown import render as md_render
        config = self._chat_config()
        path = self._chat_session_path(session_id)
        if path is None or not path.is_file():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "chat session not found"}})
            return
        messages: list[dict[str, Any]] = []
        try:
            for raw in path.read_text(encoding="utf-8").splitlines()[-200:]:
                if not raw.strip():
                    continue
                try:
                    entry = json.loads(raw)
                except json.JSONDecodeError:
                    continue
                role = str(entry.get("role") or "")
                content = str(entry.get("content") or "")
                rendered = md_render(content) if role in ("assistant", "system") else (
                    "<p>" + _escape_html(content).replace("\n", "<br>") + "</p>"
                )
                messages.append(
                    {
                        "role": role,
                        "content": content,
                        "rendered": rendered,
                        "created_at": entry.get("created_at") or "",
                    }
                )
        except OSError:
            pass
        try:
            html = _jinja_env().get_template("chat.html").render(
                session_id=session_id,
                messages=messages,
                model=getattr(config, "model", None),
                flavor=getattr(config, "flavor", None),
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _handle_chat_new(self) -> None:
        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "chat new requires --allow-mutations"}})
            return
        config = self._chat_config()
        if config is None:
            self._send_json(412, {"ok": False, "error": {"code": 412, "message": "chat is not configured; set STRIATUM_CHAT_API_BASE_URL / API_KEY / MODEL / API_FLAVOR"}})
            return
        # Read but discard the form body if present.
        self._read_form_body(max_bytes=4096)
        session_id = uuid.uuid4().hex[:8]
        target = self._chat_session_path(session_id)
        if target is None:
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": "could not allocate chat session"}})
            return
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text("", encoding="utf-8")
        self.send_response(303)
        self.send_header("Location", f"/chat/{session_id}")
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _handle_chat_send(self, session_id: str) -> None:
        if not _is_safe_id(session_id):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid chat session id"}})
            return
        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "chat send requires --allow-mutations"}})
            return
        config = self._chat_config()
        if config is None:
            self._send_json(412, {"ok": False, "error": {"code": 412, "message": "chat is not configured"}})
            return
        path = self._chat_session_path(session_id)
        if path is None or not path.is_file():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "chat session not found"}})
            return
        form = self._read_form_body(max_bytes=64 * 1024)
        if form is None:
            return
        message = (form.get("message") or [""])[0]
        if not isinstance(message, str) or not message.strip():
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "message is required"}})
            return
        # Append the user turn.
        _append_jsonl(path, {"role": "user", "content": message, "created_at": _utc_now_iso()})
        # Build the conversation history and stream the response.
        history = _read_chat_history(path)
        from striatum.web.chat_provider import stream_chat_response, ChatProviderError
        try:
            collected: list[str] = []
            for chunk, is_final in stream_chat_response(config, history):
                if chunk:
                    collected.append(chunk)
                    _append_jsonl(
                        path,
                        {"role": "assistant", "content": chunk, "streaming": True,
                         "created_at": _utc_now_iso()},
                    )
                if is_final:
                    break
            full = "".join(collected)
            _append_jsonl(
                path,
                {"role": "assistant", "content": full, "streaming": False,
                 "created_at": _utc_now_iso()},
            )
        except ChatProviderError as exc:
            _append_jsonl(
                path,
                {"role": "system", "content": f"[chat error] {exc}",
                 "created_at": _utc_now_iso()},
            )
            self._send_json(502, {"ok": False, "error": {"code": 502, "message": str(exc)}})
            return
        except Exception as exc:  # noqa: BLE001
            _append_jsonl(
                path,
                {"role": "system", "content": f"[unexpected error] {type(exc).__name__}",
                 "created_at": _utc_now_iso()},
            )
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": "chat send failed"}})
            return
        self.send_response(204)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _handle_chat_stop(self, session_id: str) -> None:
        if not _is_safe_id(session_id):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid chat session id"}})
            return
        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "chat stop requires --allow-mutations"}})
            return
        # V1: stopping is a no-op since each request is independent.
        # The transcript JSONL persists; cleanup is via `striatum chat purge`
        # (V1.5).
        self.send_response(303)
        self.send_header("Location", "/chat")
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _stream_chat_events(self, session_id: str) -> None:
        if not _is_safe_id(session_id):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid chat session id"}})
            return
        path = self._chat_session_path(session_id)
        if path is None:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "chat session not found"}})
            return
        # SSE stream that tails the JSONL file. Send the tail of the
        # current state, then poll for new lines.
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.send_header(
            "Content-Security-Policy",
            "default-src 'self'; script-src 'self'; style-src 'self'; "
            "img-src 'self' data:; connect-src 'self'",
        )
        self.end_headers()
        last_offset = 0
        deadline = time.monotonic() + 600.0  # 10 min cap
        try:
            while time.monotonic() < deadline:
                if path.is_file():
                    size = path.stat().st_size
                    if size > last_offset:
                        with path.open("rb") as fh:
                            fh.seek(last_offset)
                            chunk = fh.read()
                        last_offset = size
                        for raw_line in chunk.decode("utf-8", errors="replace").splitlines():
                            if not raw_line.strip():
                                continue
                            self.wfile.write(b"data: ")
                            self.wfile.write(raw_line.encode("utf-8"))
                            self.wfile.write(b"\n\n")
                            self.wfile.flush()
                time.sleep(SSE_POLL_INTERVAL_SECONDS)
        except (BrokenPipeError, ConnectionResetError):
            return

    def _render_view_path(self, subpath: str) -> None:
        from striatum.web.markdown import render as md_render
        if not subpath:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "missing path"}})
            return
        rel = subpath
        if rel.startswith("/") or ".." in Path(rel).parts or "\x00" in rel:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid path"}})
            return
        target = (self.state.repo / rel).resolve()
        repo_root = self.state.repo.resolve()
        try:
            target.relative_to(repo_root)
        except ValueError:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "path escapes repo root"}})
            return
        # Hide `.git/` and `.striatum/` by default.
        rel_parts = target.relative_to(repo_root).parts
        if rel_parts and rel_parts[0] in (".git", ".striatum"):
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "hidden path"}})
            return
        if not target.exists():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "path not found"}})
            return
        if not target.is_file():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "directory listing not in V1; view a file directly"}})
            return
        try:
            raw = target.read_bytes()
        except OSError as exc:
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})
            return
        rel_path = str(target.relative_to(repo_root))
        ext = target.suffix.lower()
        binary_exts = {
            ".png", ".jpg", ".jpeg", ".gif", ".pdf", ".zip", ".tar",
            ".gz", ".ico", ".woff", ".woff2", ".ttf", ".eot",
            ".mp3", ".mp4", ".mov", ".bin", ".so",
        }
        is_binary = (ext in binary_exts) or (b"\x00" in raw[:1024])
        ctx: dict[str, Any] = {
            "rel_path": rel_path,
            "size_bytes": len(raw),
            "mode": None,
            "rendered_html": None,
            "text_body": None,
            "lang": ext.lstrip(".") or "text",
            "message": None,
        }
        if is_binary:
            ctx["message"] = f"Binary file ({len(raw)} bytes); use the raw API to download."
        elif ext == ".md":
            ctx["rendered_html"] = md_render(raw.decode("utf-8", errors="replace"))
        else:
            ctx["text_body"] = raw.decode("utf-8", errors="replace")
        try:
            html = _jinja_env().get_template("view_file.html").render(**ctx)
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _read_form_body(self, *, max_bytes: int) -> dict[str, list[str]] | None:
        from urllib.parse import parse_qs as _parse_qs
        try:
            length = int(self.headers.get("Content-Length") or "0")
        except ValueError:
            length = 0
        if length > max_bytes:
            self._send_json(413, {"ok": False, "error": {"code": 413, "message": "form body too large"}})
            return None
        if length <= 0:
            return {}
        try:
            raw = self.rfile.read(length).decode("utf-8", errors="replace")
        except OSError as exc:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
            return None
        ctype = self.headers.get("Content-Type", "")
        if "multipart/form-data" in ctype:
            # Minimal multipart parser for our one-field form.
            return _parse_simple_multipart(raw, ctype)
        return _parse_qs(raw, keep_blank_values=True)

    def _send_html(self, status: int, body: str) -> None:
        data = body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.send_header(
            "Content-Security-Policy",
            "default-src 'self'; script-src 'self'; style-src 'self'; "
            "img-src 'self' data:; connect-src 'self'",
        )
        self.send_header("Connection", "close")
        self.end_headers()
        try:
            self.wfile.write(data)
        except BrokenPipeError:
            return

    def _serve_static_asset(self, relative: str) -> None:
        """RFC 0013 V1: serve a bundled SPA asset from striatum.web.static."""
        if not relative or ".." in relative or relative.startswith("/"):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid asset path"}})
            return
        try:
            from importlib.resources import files

            asset = files("striatum.web.static").joinpath(relative)
            if not asset.is_file():
                self._send_json(404, {"ok": False, "error": {"code": 404, "message": "asset not found"}})
                return
            data = asset.read_bytes()
        except (FileNotFoundError, ModuleNotFoundError, OSError):
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "asset not found"}})
            return
        suffix = relative.rsplit(".", 1)[-1].lower()
        content_type = {
            "html": "text/html; charset=utf-8",
            "css": "text/css; charset=utf-8",
            "js": "application/javascript; charset=utf-8",
            "json": "application/json",
            "svg": "image/svg+xml",
            "png": "image/png",
        }.get(suffix, "application/octet-stream")
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.send_header(
            "Content-Security-Policy",
            "default-src 'self'; script-src 'self'; style-src 'self'; "
            "img-src 'self' data:; connect-src 'self'",
        )
        self.send_header("Connection", "close")
        self.end_headers()
        try:
            self.wfile.write(data)
        except BrokenPipeError:
            return

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
    # RFC 0022 V1 + RFC 0023 V1: web UI is bundled and active when
    # --web is passed; no warning needed. Earlier versions emitted
    # one here; the warning is dropped now that the SSR pages ship.
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

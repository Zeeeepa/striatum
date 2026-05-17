"""RFC 0012 V1: local HTTP + Unix-socket service.

Operationalizes D006's promise of an "optional Unix-socket / local HTTP
API later for Slack, TUI, and web adapters." Production state-changing
endpoints and migrated read pages delegate to daemon RPC; legacy SQLite reads
remain transition debt for pages that have not yet moved to daemon DTOs.
Localhost-only by default; non-loopback hosts are refused at startup.
Mutations are gated behind ``--allow-mutations``.
"""

from __future__ import annotations

import json
import os
import secrets
import signal
import socket
import socketserver
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any, Mapping
from urllib.parse import parse_qs, urlsplit

from striatum.api import invoke
from striatum.service_command_policy import is_read_command as is_read_command
from striatum.service_http import (
    allowed_origins_for_bind as allowed_origins_for_bind,
    is_json_content_type as is_json_content_type,
    make_web_context_token as make_web_context_token,
    tokens_match as tokens_match,  # noqa: F401 - compatibility re-export
)
from striatum.service_request_security import (
    SecurityDecision,
    authenticate_request as _authenticate_request,
    has_valid_bearer as _has_valid_bearer,
    requires_same_origin as _requires_same_origin,
    verify_override_verdict_context as _verify_override_verdict_context,
    verify_same_origin_mutation as _verify_same_origin_mutation,
)
from striatum import service_request_io as _request_io
from striatum.service_sse import (
    encode_sse_event as _encode_sse_event,
    sse_since as _sse_since,
)
from striatum.service_runtime import (
    ServiceAlreadyRunningError as ServiceAlreadyRunningError,
    ServiceConfigError as ServiceConfigError,
    check_single_instance as _check_single_instance,
    ensure_loopback as _ensure_loopback,
    service_mode as _service_mode,
    striatum_version as _striatum_version,
    wait_for_service_shutdown as _wait_for_service_shutdown,
)
from striatum.service_state import (
    SSE_MAX_CONCURRENT_PER_RUN as SSE_MAX_CONCURRENT_PER_RUN,
    ServiceState as ServiceState,
    utcnow_iso as utcnow_iso,
)
from striatum.web.doctor import (
    DoctorPageError as _DoctorPageError,
    doctor_page_response as _doctor_page_response,
)
from striatum.web import chat_session as _chat_session
from striatum.web.chat_session import (
    append_jsonl as _append_jsonl,
    chat_session_path as _chat_session_path,
    find_pending_tool_confirmation as _find_pending_tool_confirmation,
    list_chat_sessions as _list_chat_sessions,
    queue_workflow_write_confirmation as _queue_workflow_write_confirmation,
    read_chat_history as _read_chat_history,
    read_display_messages as _read_display_messages,
    safe_git as _safe_git,
    split_system as _split_system,
    utc_now_iso as _utc_now_iso,
)
from striatum.web import template_env as _template_env
from striatum.web.static_assets import (
    StaticAssetError as StaticAssetError,
    load_static_asset as _load_static_asset,
)
from striatum.web.artifacts import (
    artifact_content_type as _artifact_content_type,
    inline_artifact_body as _inline_artifact_body,
    resolve_artifact_file as _resolve_artifact_file,
)
from striatum.web.run_list import run_list_view_item as _run_list_view_item
from striatum.web.workflow_generation import (
    generator_error_response as _generator_error_response,
    workflow_generate_response as _workflow_generate_response,
    workflow_template_show_response as _workflow_template_show_response,
    workflow_templates_response as _workflow_templates_response,
)

JsonObject = dict[str, Any]
_project_history_anthropic = _chat_session.project_history_anthropic
_project_history_openai = _chat_session.project_history_openai


def _legacy_service() -> Any:
    from striatum.legacy_sqlite import service as legacy_service

    return legacy_service


class _LazyLegacyCallable:
    def __init__(self, name: str) -> None:
        self._name = name

    def __call__(self, *args: Any, **kwargs: Any) -> Any:
        return getattr(_legacy_service(), self._name)(*args, **kwargs)


_byline_line = _LazyLegacyCallable("_byline_line")
_shape_artifact_rows = _LazyLegacyCallable("legacy_shape_artifact_rows")
_shape_verdict_rows = _LazyLegacyCallable("_shape_verdict_rows")
_legacy_artifact_metadata = _LazyLegacyCallable("legacy_artifact_metadata")
_legacy_artifact_raw_fallback_enabled = _LazyLegacyCallable(
    "legacy_artifact_raw_fallback_enabled"
)
_legacy_artifact_view_payload = _LazyLegacyCallable("legacy_artifact_view_payload")
_legacy_doctor_page_payload = _LazyLegacyCallable("legacy_doctor_page_payload")
_legacy_fixture_fallback_enabled = _LazyLegacyCallable("legacy_fixture_fallback_enabled")
_legacy_job_cancel = _LazyLegacyCallable("legacy_job_cancel")
_legacy_job_detail_payload = _LazyLegacyCallable("legacy_job_detail_payload")
_legacy_job_retry = _LazyLegacyCallable("legacy_job_retry")
_legacy_run_cancel = _LazyLegacyCallable("legacy_run_cancel")
_legacy_run_detail_payload = _LazyLegacyCallable("legacy_run_detail_payload")
_legacy_run_list_items_for_test_harness = _LazyLegacyCallable(
    "legacy_run_list_items_for_test_harness"
)
_legacy_run_pause = _LazyLegacyCallable("legacy_run_pause")
_legacy_run_posture_verdicts_payload = _LazyLegacyCallable(
    "legacy_run_posture_verdicts_payload"
)
_legacy_run_resume = _LazyLegacyCallable("legacy_run_resume")
_legacy_shape_artifact_rows = _shape_artifact_rows
_legacy_stream_events_body = _LazyLegacyCallable("legacy_stream_events_body")
_legacy_verify_state_health = _LazyLegacyCallable("legacy_verify_state_health")
_legacy_view_file_run_breadcrumb = _LazyLegacyCallable("legacy_view_file_run_breadcrumb")
_legacy_web_read_fallback_enabled = _LazyLegacyCallable("legacy_web_read_fallback_enabled")
_legacy_workflow_run_now = _LazyLegacyCallable("legacy_workflow_run_now")
_send_legacy_fixture_error = _LazyLegacyCallable("send_legacy_fixture_error")
_send_legacy_run_now_error = _LazyLegacyCallable("send_legacy_run_now_error")
_short_git_status = _LazyLegacyCallable("short_git_status")

SSE_POLL_INTERVAL_SECONDS = 0.25


def _handler_send_json(handler: BaseHTTPRequestHandler, status: int, body: Mapping[str, Any]) -> None:
    send_json = getattr(handler, "_send_json")
    send_json(status, dict(body))


SHUTDOWN_DRAIN_SECONDS = 5.0


def _is_safe_id(value: str) -> bool:
    """RFC 0023 V1: chat session ids and similar paths must be ASCII
    alphanumeric / underscore / hyphen only."""
    if not value:
        return False
    return all(ch.isalnum() or ch in "-_" for ch in value)


def _escape_html(s: str) -> str:
    return _template_env.escape_html(s)


def _build_chat_briefing(repo: Path, *, allow_mutations: bool = False) -> str:
    return _chat_session.build_chat_briefing(
        repo,
        allow_mutations=allow_mutations,
        safe_git_func=_safe_git,
    )


def _jinja_env() -> Any:
    return _template_env.jinja_env()


def _jinja_env_factory() -> Any:
    return _template_env.jinja_env_factory()


class StriatumServiceHandler(BaseHTTPRequestHandler):
    """HTTP request handler for the local service.

    The production web/API read paths call daemon RPC directly where a
    daemon DTO exists. Legacy CLI invoke fallbacks are retained only for the
    subprocess test harness while repo-local SQLite compatibility fixtures
    still exist.
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
            self._handle_daemon_read("status", {}, legacy_argv=["status"])
            return
        if path == "/v1/doctor":
            self._handle_doctor(query)
            return
        if path == "/v1/repo/tree":
            self._handle_repo_tree(query)
            return
        if path.startswith("/v1/runs/"):
            self._handle_run_subpath(path[len("/v1/runs/"):], query)
            return
        if path.startswith("/v1/artifacts/") and path.endswith("/raw"):
            artifact_id = path[len("/v1/artifacts/"):-len("/raw")]
            self._handle_artifact_raw(artifact_id)
            return
        if path == "/workflow-templates":
            kind = query.get("kind", [None])[0]
            self._handle_workflow_templates(kind)
            return
        if path.startswith("/workflow-templates/"):
            self._handle_workflow_template_show(path[len("/workflow-templates/"):])
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
        if self.state.web_enabled and path == "/view":
            self._render_view_path("")
            return
        if self.state.web_enabled and path.startswith("/view/"):
            self._render_view_path(path[len("/view/"):])
            return
        if self.state.web_enabled and path == "/workflows":
            self._render_workflows_index_page()
            return
        if self.state.web_enabled and path == "/workflows/new":
            self._render_workflows_new_page()
            return
        if self.state.web_enabled and path.startswith("/workflows/edit/"):
            self._render_workflow_edit_page(path[len("/workflows/edit/"):])
            return
        if self.state.web_enabled and path.startswith("/workflows/"):
            self._render_workflow_detail_page(path[len("/workflows/"):])
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
        if self._requires_same_origin(parsed.path) and not self._verify_same_origin_mutation():
            return
        if self.state.web_enabled and parsed.path == "/chat/new":
            self._handle_chat_new()
            return
        if self.state.web_enabled and parsed.path.startswith("/workflows/edit/"):
            self._handle_workflow_edit_save(parsed.path[len("/workflows/edit/"):])
            return
        if self.state.web_enabled and parsed.path.startswith("/workflows/run/"):
            self._handle_workflow_run_now(parsed.path[len("/workflows/run/"):])
            return
        if parsed.path == "/workflows/generate/preview":
            self._handle_workflow_generate(preview=True)
            return
        if parsed.path == "/workflows/generate":
            self._handle_workflow_generate(preview=False)
            return
        if self.state.web_enabled and parsed.path.startswith("/run/") and parsed.path.endswith("/branch-confirm"):
            run_id = parsed.path[len("/run/"):-len("/branch-confirm")]
            self._handle_run_branch_confirm(run_id)
            return
        if self.state.web_enabled and parsed.path.startswith("/run/") and parsed.path.endswith("/cancel") and "/job/" not in parsed.path:
            run_id = parsed.path[len("/run/"):-len("/cancel")]
            self._handle_run_cancel(run_id)
            return
        if self.state.web_enabled and parsed.path.startswith("/run/") and parsed.path.endswith("/pause"):
            run_id = parsed.path[len("/run/"):-len("/pause")]
            self._handle_run_pause(run_id)
            return
        if self.state.web_enabled and parsed.path.startswith("/run/") and parsed.path.endswith("/resume"):
            run_id = parsed.path[len("/run/"):-len("/resume")]
            self._handle_run_resume(run_id)
            return
        if self.state.web_enabled and "/job/" in parsed.path and (parsed.path.endswith("/cancel") or parsed.path.endswith("/retry")):
            self._handle_job_action(parsed.path)
            return
        if self.state.web_enabled and parsed.path.startswith("/chat/") and parsed.path.endswith("/send"):
            session_id = parsed.path[len("/chat/"):-len("/send")]
            self._handle_chat_send(session_id)
            return
        if self.state.web_enabled and parsed.path.startswith("/chat/") and "/confirm-tool/" in parsed.path:
            rest = parsed.path[len("/chat/"):]
            session_id, _, tool_id = rest.partition("/confirm-tool/")
            self._handle_chat_confirm_tool(session_id, tool_id)
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
        # GH #10: when web UI is enabled, override-verdict POSTs must
        # carry a server-issued context token bound to the rendered
        # job page. This defeats DOM-tampering attacks (where another
        # script flips data-job-id between page render and submit).
        if self.state.web_enabled and argv and argv[0] == "override-verdict":
            if not self._verify_override_verdict_context(argv, body):
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

    def _handle_workflow_templates(self, kind: str | None) -> None:
        response = _workflow_templates_response(kind)
        self._send_json(response.status, response.payload)

    def _handle_workflow_template_show(self, raw_template_id: str) -> None:
        response = _workflow_template_show_response(raw_template_id)
        self._send_json(response.status, response.payload)

    def _handle_workflow_generate(self, *, preview: bool) -> None:
        body = self._read_json_body()
        if body is None:
            return
        response = _workflow_generate_response(
            repo=self.state.repo,
            body=body,
            preview=preview,
            allow_mutations=self.state.allow_mutations,
        )
        self._send_json(response.status, response.payload)

    def _send_generator_error(self, exc: Exception, *, status: int = 400) -> None:
        response = _generator_error_response(exc, status=status)
        self._send_json(response.status, response.payload)

    def _handle_doctor(self, query: dict[str, list[str]]) -> None:
        run_id = query.get("run_id", [None])[0]
        params: dict[str, Any] = {"verbose": True}
        argv = ["doctor", "--verbose"]
        if run_id:
            params["run_id"] = run_id
            argv.extend(["--run-id", run_id])
        self._handle_daemon_read("doctor", params, legacy_argv=argv)

    def _handle_repo_tree(self, query: dict[str, list[str]]) -> None:
        from striatum.web.workflows import list_repo_tree

        rel_path = query.get("path", [""])[0]
        tree = list_repo_tree(self.state.repo, rel_path)
        if tree is None:
            self._send_json(
                404,
                {"ok": False, "error": {"code": 404, "message": "directory not found"}},
            )
            return
        self._send_json(200, {"ok": True, "data": tree})

    def _handle_run_subpath(self, suffix: str, query: dict[str, list[str]]) -> None:
        parts = suffix.split("/")
        run_id = parts[0]
        if not run_id:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "missing run_id"}})
            return
        if len(parts) == 1:
            self._handle_daemon_read(
                "status",
                {"run_id": run_id},
                legacy_argv=["status", "--run-id", run_id],
            )
            return
        sub = parts[1]
        if sub == "why":
            target = query.get("id", [None])[0]
            if not target:
                self._send_json(400, {"ok": False, "error": {"code": 400, "message": "missing ?id=<entity>"}})
                return
            self._handle_daemon_read(
                "why",
                {"target_id": target},
                legacy_argv=["why", target],
            )
            return
        if sub == "dashboard":
            self._handle_daemon_read(
                "dashboard",
                {"run_id": run_id},
                legacy_argv=["dashboard", "--run-id", run_id, "--once"],
            )
            return
        if sub == "events":
            since = self._sse_since(query)
            self._stream_events(run_id, since=since)
            return
        if sub == "artifacts":
            self._handle_daemon_read(
                "list.artifacts",
                {"run_id": run_id},
                legacy_argv=["list", "artifacts", "--run-id", run_id],
            )
            return
        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})

    def _handle_daemon_read(
        self,
        method: str,
        params: Mapping[str, Any],
        *,
        legacy_argv: list[str] | None = None,
    ) -> None:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        try:
            payload = call_repo_method(self.state.repo, method, dict(params))
        except ServiceDaemonRpcError as exc:
            if legacy_argv is not None and _legacy_web_read_fallback_enabled(exc.code):
                self._handle_invoke(legacy_argv)
                return
            self._send_json(
                exc.status,
                {"ok": False, "error": {"code": exc.code, "message": exc.message}},
            )
            return
        self._send_json(200, {"ok": True, "data": payload})

    def _handle_artifact_raw(self, artifact_id: str) -> None:
        """RFC 0013 V1: serve the raw bytes of an artifact for the web UI viewer.

        Looks up artifact metadata through the daemon, opens the file at
        ``repo_path``, and streams the bytes back. Read-only; no mutation gate.
        Returns 404 if the row or the file is missing.
        """
        if not artifact_id:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "missing artifact id"}})
            return
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        try:
            payload = call_repo_method(self.state.repo, "artifact.show", {"artifact_id": artifact_id})
            artifact_raw = payload.get("artifact")
            if not isinstance(artifact_raw, Mapping):
                self._send_json(
                    500,
                    {"ok": False, "error": {"code": 500, "message": "daemon artifact DTO missing artifact"}},
                )
                return
            artifact = artifact_raw
        except ServiceDaemonRpcError as exc:
            if _legacy_artifact_raw_fallback_enabled(exc.code):
                try:
                    legacy = _legacy_artifact_metadata(self.state.repo, artifact_id=artifact_id)
                except Exception as legacy_exc:  # noqa: BLE001 - legacy fixture path.
                    self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(legacy_exc)}})
                    return
                if legacy is None:
                    self._send_json(404, {"ok": False, "error": {"code": 404, "message": "artifact not found"}})
                    return
                artifact = legacy
            else:
                self._send_json(
                    exc.status,
                    {"ok": False, "error": {"code": exc.code, "message": exc.message}},
                )
                return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": f"{type(exc).__name__}: {exc}"}})
            return

        try:
            repo_path = _resolve_artifact_file(self.state.repo, artifact.get("repo_path"))
        except ValueError:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "artifact file missing on disk"}})
            return
        if not repo_path.is_file():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "artifact file missing on disk"}})
            return
        try:
            data = repo_path.read_bytes()
        except OSError as exc:
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})
            return
        self.send_response(200)
        self.send_header("Content-Type", _artifact_content_type(repo_path))
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
            from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

            try:
                payload = call_repo_method(self.state.repo, "list.runs", {"limit": 500})
                items = payload.get("items")
                raw_runs = items if isinstance(items, list) else []
                runs = [
                    _run_list_view_item(self.state, item)
                    for item in raw_runs
                    if isinstance(item, Mapping)
                ]
                runs.sort(key=lambda item: str(item.get("created_at") or ""), reverse=True)
            except ServiceDaemonRpcError as exc:
                if (
                    os.environ.get("STRIATUM_TEST_HARNESS") == "1"
                    and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"
                ):
                    runs = _legacy_run_list_items_for_test_harness(
                        self.state.repo,
                        shape_run=lambda item: _run_list_view_item(self.state, item),
                    )
                else:
                    self._send_json(
                        exc.status,
                        {
                            "ok": False,
                            "error": {"code": exc.code, "message": exc.message},
                        },
                    )
                    return
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
        if len(parts) == 3 and parts[1] == "posture":
            self._render_run_posture_verdicts_page(run_id, parts[2])
            return
        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})

    def _render_run_posture_verdicts_page(self, run_id: str, posture: str) -> None:
        """RFC 0024 V4.1: drill-down for `verdicts_by_posture` chips.

        Lists every verdict for `(run_id, posture)` with links to the
        review job and (when present) the finding artifact.
        """
        if any(c in posture for c in ("/", "\x00", "..")) or not posture:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid posture"}})
            return
        try:
            from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

            try:
                payload = call_repo_method(
                    self.state.repo,
                    "run.posture_verdicts",
                    {"run_id": run_id, "posture": posture},
                )
            except ServiceDaemonRpcError as exc:
                if _legacy_web_read_fallback_enabled(exc.code):
                    try:
                        payload = _legacy_run_posture_verdicts_payload(
                            self.state.repo,
                            run_id=run_id,
                            posture=posture,
                        )
                    except KeyError:
                        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "run not found"}})
                        return
                else:
                    self._send_json(
                        exc.status,
                        {"ok": False, "error": {"code": exc.code, "message": exc.message}},
                    )
                    return
            run = payload.get("run")
            verdicts = payload.get("verdicts")
            if not isinstance(run, Mapping) or not isinstance(verdicts, list):
                self._send_json(
                    500,
                    {
                        "ok": False,
                        "error": {"code": 500, "message": "daemon posture verdict DTO missing fields"},
                    },
                )
                return
            html = _jinja_env().get_template("run_posture_verdicts.html").render(
                run=dict(run),
                posture=str(payload.get("posture") or posture),
                verdicts=[
                    dict(verdict)
                    for verdict in verdicts
                    if isinstance(verdict, Mapping)
                ],
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_run_detail_page(self, run_id: str) -> None:
        from striatum.web.graph_svg import (
            compute_node_states_from_jobs,
            render_run_graph,
        )
        try:
            from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

            try:
                payload = call_repo_method(self.state.repo, "run.detail", {"run_id": run_id})
            except ServiceDaemonRpcError as exc:
                if _legacy_web_read_fallback_enabled(exc.code):
                    try:
                        payload = _legacy_run_detail_payload(self.state.repo, run_id=run_id)
                    except KeyError:
                        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "run not found"}})
                        return
                else:
                    self._send_json(
                        exc.status,
                        {"ok": False, "error": {"code": exc.code, "message": exc.message}},
                    )
                    return
            run_raw = payload.get("run")
            workflow_raw = payload.get("workflow")
            jobs_raw = payload.get("jobs")
            if not isinstance(run_raw, Mapping) or not isinstance(workflow_raw, Mapping) or not isinstance(jobs_raw, list):
                self._send_json(
                    500,
                    {"ok": False, "error": {"code": 500, "message": "daemon run detail DTO missing fields"}},
                )
                return
            run = dict(run_raw)
            workflow = dict(workflow_raw)
            jobs = [
                dict(job)
                for job in jobs_raw
                if isinstance(job, Mapping)
            ]
            node_states = compute_node_states_from_jobs(jobs)
            graph_svg = render_run_graph(
                workflow,
                node_states,
                run_id=run_id,
                jobs=jobs,
            )
            html = _jinja_env().get_template("run_detail.html").render(
                run=run,
                jobs=jobs,
                graph_svg=graph_svg,
                next_actions=payload.get("next_actions") or [],
                recovery_panel=payload.get("recovery_panel") or {},
                verdicts_by_posture=payload.get("verdicts_by_posture") or {},
                sessions=payload.get("sessions") or [],
                phase_progress=payload.get("phase_progress") or [],
                current_phase_id=payload.get("current_phase_id"),
                suggested_branch_name=payload.get("suggested_branch_name") or "",
                allow_dirty=bool(payload.get("allow_dirty")),
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_job_detail_page(self, run_id: str, job_id: str) -> None:
        try:
            from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

            try:
                payload = call_repo_method(
                    self.state.repo,
                    "job.detail",
                    {"run_id": run_id, "job_id": job_id},
                )
            except ServiceDaemonRpcError as exc:
                if _legacy_web_read_fallback_enabled(exc.code):
                    try:
                        payload = _legacy_job_detail_payload(
                            self.state.repo,
                            run_id=run_id,
                            job_ref=job_id,
                        )
                    except KeyError:
                        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})
                        return
                else:
                    self._send_json(
                        exc.status,
                        {"ok": False, "error": {"code": exc.code, "message": exc.message}},
                    )
                    return
            run_raw = payload.get("run")
            job_raw = payload.get("job")
            if not isinstance(run_raw, Mapping) or not isinstance(job_raw, Mapping):
                self._send_json(
                    500,
                    {"ok": False, "error": {"code": 500, "message": "daemon job detail DTO missing fields"}},
                )
                return
            run = dict(run_raw)
            job = dict(job_raw)
            artifacts = [
                dict(artifact)
                for artifact in payload.get("artifacts") or []
                if isinstance(artifact, Mapping)
            ]
            verdicts = [
                dict(verdict)
                for verdict in payload.get("verdicts") or []
                if isinstance(verdict, Mapping)
            ]
            latest_raw = payload.get("latest_verdict")
            latest_verdict = dict(latest_raw) if isinstance(latest_raw, Mapping) else None
            expected_artifact_rows = [
                dict(row)
                for row in payload.get("expected_artifact_rows") or []
                if isinstance(row, Mapping)
            ]
            process_evidence = [
                dict(row)
                for row in payload.get("process_evidence") or []
                if isinstance(row, Mapping)
            ]
            # GH #10: mint a context token binding the rendered page to
            # the override-verdict action's (run_id, job_id, session_id).
            override_session_id = ""
            if latest_verdict is not None:
                override_session_id = str(latest_verdict.get("session_id", "")) or ""
            override_context_token = ""
            if override_session_id:
                override_context_token = make_web_context_token(
                    self.state.web_context_secret,
                    run_id=str(run["run_id"]),
                    job_id=str(job["job_id"]),
                    session_id=override_session_id,
                )
            html = _jinja_env().get_template("job_detail.html").render(
                run=run,
                job=job,
                artifacts=artifacts,
                latest_verdict=latest_verdict,
                verdicts=verdicts,
                expected_artifact_rows=expected_artifact_rows,
                process_evidence=process_evidence,
                override_context_token=override_context_token,
                override_session_id=override_session_id,
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_artifact_view_page(self, run_id: str, artifact_id: str) -> None:
        try:
            from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

            try:
                payload = call_repo_method(
                    self.state.repo,
                    "artifact.show",
                    {
                        "artifact_id": artifact_id,
                        "run_id": run_id,
                        "include_web_context": True,
                    },
                )
            except ServiceDaemonRpcError as exc:
                if _legacy_web_read_fallback_enabled(exc.code):
                    try:
                        payload = _legacy_artifact_view_payload(
                            self.state.repo,
                            run_id=run_id,
                            artifact_id=artifact_id,
                        )
                    except KeyError:
                        self._send_json(404, {"ok": False, "error": {"code": 404, "message": "not found"}})
                        return
                else:
                    self._send_json(
                        exc.status,
                        {"ok": False, "error": {"code": exc.code, "message": exc.message}},
                    )
                    return
            run_raw = payload.get("run")
            artifact_raw = payload.get("artifact")
            if not isinstance(run_raw, Mapping) or not isinstance(artifact_raw, Mapping):
                self._send_json(
                    500,
                    {
                        "ok": False,
                        "error": {"code": 500, "message": "daemon artifact DTO missing fields"},
                    },
                )
                return
            run = dict(run_raw)
            artifact = dict(artifact_raw)
            if "lane_attestation_chip" not in artifact:
                expected_rows = [
                    {
                        "path": artifact.get("repo_path"),
                        "expected_author_line": payload.get("expected_author_line"),
                    }
                ]
                artifact = _legacy_shape_artifact_rows(
                    None,
                    artifacts=[artifact],
                    expected_rows=expected_rows,
                )[0]
            if "provenance_trail" not in artifact:
                trail = payload.get("provenance_trail")
                artifact["provenance_trail"] = trail if isinstance(trail, list) else []
            body = _inline_artifact_body(self.state.repo, artifact)
            html = _jinja_env().get_template("artifact_view.html").render(
                run=run, artifact=artifact,
                rendered_md=body.rendered_md, body_text=body.body_text,
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_workflows_index_page(self) -> None:
        from striatum.web.workflows import discover
        try:
            workflows = discover(self.state.repo)
            # Drop the parsed `data` dict per entry to keep the index
            # response small (the detail page reloads from disk).
            slim = [
                {k: v for k, v in entry.items() if k != "data"}
                for entry in workflows
            ]
            html = _jinja_env().get_template("workflows_index.html").render(
                workflows=slim,
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_workflows_new_page(self) -> None:
        try:
            html = _jinja_env().get_template("workflow_new.html").render(
                allow_mutations=self.state.allow_mutations,
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_workflow_detail_page(self, rel_path: str) -> None:
        from striatum.web.graph_svg import render_run_graph
        from striatum.web.workflows import load_workflow_at
        if not rel_path:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "missing path"}})
            return
        if rel_path.startswith("/") or "\x00" in rel_path or ".." in Path(rel_path).parts:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid path"}})
            return
        entry = load_workflow_at(self.state.repo, rel_path)
        if entry is None:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "workflow not found"}})
            return
        graph_svg: str | None = None
        data = entry.get("data")
        if isinstance(data, dict) and entry.get("status") == "valid":
            try:
                graph_svg = render_run_graph(data, node_states={}, run_id=None)
            except Exception:  # noqa: BLE001
                graph_svg = None
        try:
            html = _jinja_env().get_template("workflow_detail.html").render(
                workflow=entry,
                graph_svg=graph_svg,
            )
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _render_workflow_edit_page(self, rel_path: str) -> None:
        """RFC 0024 V1.5: render the visual builder for a workflow path.

        Existing files load their parsed JSON (even if invalid — the
        editor opens so the user can fix). Non-existent paths render an
        empty scaffold derived from the path stem.
        """
        from striatum.web.workflows import WorkflowFileError, workflow_edit_payload

        try:
            payload = workflow_edit_payload(self.state.repo, rel_path)
            html = _jinja_env().get_template("workflow_edit.html").render(
                rel_path=payload["rel_path"],
                is_new=payload["is_new"],
                workflow_json=json.dumps(payload["workflow_data"]),
                workflow_sha256=payload["workflow_sha256"],
            )
            self._send_html(200, html)
        except WorkflowFileError as exc:
            self._send_json(
                exc.status_code,
                {"ok": False, "error": {"code": exc.status_code, "message": exc.message}},
            )
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _handle_workflow_edit_save(self, rel_path: str) -> None:
        """RFC 0024 V1.5: POST endpoint for the visual builder.

        Mutation-gated. Validates the body via ``validate_workflow``;
        on failure returns 422 with the error message; on success
        atomically writes ``<path>.tmp`` then renames into place and
        returns 200.
        """
        from striatum.web.workflows import WorkflowFileError, save_workflow_file

        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "workflow edit requires --allow-mutations"}})
            return
        if not rel_path:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "missing path"}})
            return
        if rel_path.startswith("/") or "\x00" in rel_path or ".." in Path(rel_path).parts:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid path"}})
            return
        # F1 (design review): refuse non-JSON content-types and cap body size.
        ctype = self.headers.get("Content-Type", "")
        if "application/json" not in ctype:
            self._send_json(415, {"ok": False, "error": {"code": 415, "message": "Content-Type must be application/json"}})
            return
        try:
            length = int(self.headers.get("Content-Length") or "0")
        except ValueError:
            length = 0
        if length > 1024 * 1024:
            self._send_json(413, {"ok": False, "error": {"code": 413, "message": "body too large (1 MB cap)"}})
            return
        try:
            raw = self.rfile.read(length).decode("utf-8", errors="replace") if length > 0 else ""
        except OSError as exc:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
            return
        try:
            data = json.loads(raw)
        except json.JSONDecodeError as exc:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": f"invalid JSON: {exc}"}})
            return
        if not isinstance(data, dict):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "body must be a JSON object"}})
            return
        try:
            result = save_workflow_file(
                self.state.repo,
                rel_path,
                data,
                if_match=self.headers.get("If-Match", ""),
            )
        except WorkflowFileError as exc:
            error: JsonObject = {"code": exc.status_code, "message": exc.message}
            if exc.errors is not None:
                error["errors"] = exc.errors
            if exc.current_sha256 is not None:
                error["current_sha256"] = exc.current_sha256
            self._send_json(exc.status_code, {"ok": False, "error": error})
            return
        self._send_json(200, {"ok": True, "data": result})

    def _handle_workflow_run_now(self, rel_path: str) -> None:
        """RFC 0024 V2: POST /workflows/run/<path> — lift workflow.json into a fresh run.

        Mutation-gated. Validates path safety + content-type + body cap.
        Calls daemon ``run.prepare`` (which validates the workflow);
        auto-branch confirms when ``branch.mode == auto``; calls daemon
        ``run.start``.
        Returns ``{run_id}`` on 200; ``409`` on dirty-tree branch refusal;
        ``422`` on validation failure.
        """
        from striatum.errors import InvalidTransitionError, WorkflowError
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "run requires --allow-mutations"}})
            return
        if not rel_path:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "missing path"}})
            return
        if rel_path.startswith("/") or "\x00" in rel_path or ".." in Path(rel_path).parts:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid path"}})
            return
        ctype = self.headers.get("Content-Type", "")
        if "application/json" not in ctype:
            self._send_json(415, {"ok": False, "error": {"code": 415, "message": "Content-Type must be application/json"}})
            return
        try:
            length = int(self.headers.get("Content-Length") or "0")
        except ValueError:
            length = 0
        if length > 1024 * 1024:
            self._send_json(413, {"ok": False, "error": {"code": 413, "message": "body too large (1 MB cap)"}})
            return
        # Body is reserved for V3 overrides; ignored in V2.
        try:
            _ = self.rfile.read(length).decode("utf-8", errors="replace") if length > 0 else ""
        except OSError as exc:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
            return
        repo_root = self.state.repo.resolve()
        target = (self.state.repo / rel_path).resolve()
        try:
            target.relative_to(repo_root)
        except ValueError:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "path escapes repo"}})
            return
        rel_parts = target.relative_to(repo_root).parts
        if rel_parts and rel_parts[0] in (".git", ".striatum"):
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "hidden path"}})
            return
        if not target.is_file():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "workflow.json not found"}})
            return
        try:
            workflow_arg = "/".join(rel_parts)
            prepared = call_repo_method(self.state.repo, "run.prepare", {"workflow": workflow_arg})
            run_id = str(prepared["run_id"])
            requires_confirm = (
                prepared.get("state") == "needs_branch_confirmation"
                and prepared.get("branch_mode") != "auto"
            )
            if prepared.get("branch_mode") == "auto":
                suggested = prepared.get("suggested_branch_name")
                if isinstance(suggested, str) and suggested:
                    call_repo_method(
                        self.state.repo,
                        "branch.confirm",
                        {"run_id": run_id, "branch": suggested, "create": True},
                    )
            if requires_confirm:
                self._send_json(200, {"ok": True, "data": {"run_id": run_id, "status": "needs_branch_confirmation", "suggested_branch_name": prepared.get("suggested_branch_name")}})
                return
            call_repo_method(self.state.repo, "run.start", {"run_id": run_id})
        except ServiceDaemonRpcError as exc:
            if _legacy_web_read_fallback_enabled(exc.code):
                try:
                    result = _legacy_workflow_run_now(self.state.repo, workflow_path=target)
                except Exception as legacy_exc:  # noqa: BLE001 - mapped below.
                    if _send_legacy_run_now_error(self, self.state.repo, legacy_exc):
                        return
                    raise
                self._send_json(200, {"ok": True, "data": result})
                return
            if exc.code in {"workflow_error", "schema_invalid"}:
                msg = exc.message
                if "git checkout failed" in msg:
                    self._send_json(409, {"ok": False, "error": {"code": 409, "message": msg, "kind": "dirty_tree", "git_status": _short_git_status(self.state.repo)}})
                    return
                errors = []
                details = exc.details or {}
                field_path = details.get("field_path")
                if isinstance(field_path, str) and field_path:
                    errors.append({"field_path": field_path, "message": msg})
                self._send_json(422, {"ok": False, "error": {"code": 422, "message": msg, "errors": errors}})
                return
            error: dict[str, Any] = {"code": exc.status, "message": exc.message}
            if exc.kind is not None:
                error["kind"] = exc.kind
            self._send_json(exc.status, {"ok": False, "error": error})
            return
        except WorkflowError as exc:
            # RFC 0024 V3 (closes V2 design-review F3): dirty-tree
            # checkout failures bubble up here as WorkflowError. Detect
            # them and re-emit as a structured 409 with `git status`
            # so the operator sees what's blocking without dropping
            # to a terminal.
            msg = str(exc)
            if "git checkout failed" in msg:
                self._send_json(409, {"ok": False, "error": {"code": 409, "message": msg, "kind": "dirty_tree", "git_status": _short_git_status(self.state.repo)}})
                return
            errors = []
            field_path = getattr(exc, "field_path", None)
            if isinstance(field_path, str) and field_path:
                errors.append({"field_path": field_path, "message": str(exc)})
            self._send_json(422, {"ok": False, "error": {"code": 422, "message": str(exc), "errors": errors}})
            return
        except InvalidTransitionError as exc:
            self._send_json(409, {"ok": False, "error": {"code": 409, "message": str(exc), "kind": "invalid_transition"}})
            return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": f"{type(exc).__name__}: {exc}"}})
            return
        self._send_json(200, {"ok": True, "data": {"run_id": run_id, "status": "running"}})

    def _handle_run_branch_confirm(self, run_id: str) -> None:
        """RFC 0024 V2.1: confirm a run's branch from the web UI.

        Body: ``{"branch": "<name>", "create": true|false, "use_current": true|false}``.
        Mutation-gated. After successful confirm, also drives ``run.start``
        so the run leaves ``needs_branch_confirmation`` immediately.
        """
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "branch confirm requires --allow-mutations"}})
            return
        ctype = self.headers.get("Content-Type", "")
        if "application/json" not in ctype:
            self._send_json(415, {"ok": False, "error": {"code": 415, "message": "Content-Type must be application/json"}})
            return
        try:
            length = int(self.headers.get("Content-Length") or "0")
        except ValueError:
            length = 0
        if length > 64 * 1024:
            self._send_json(413, {"ok": False, "error": {"code": 413, "message": "body too large"}})
            return
        try:
            raw = self.rfile.read(length).decode("utf-8", errors="replace") if length > 0 else "{}"
        except OSError as exc:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
            return
        try:
            body = json.loads(raw or "{}")
        except json.JSONDecodeError as exc:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": f"invalid JSON: {exc}"}})
            return
        if not isinstance(body, dict):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "body must be a JSON object"}})
            return
        branch_name = body.get("branch")
        create = bool(body.get("create", True))
        use_current = bool(body.get("use_current", False))
        if not isinstance(branch_name, str) or not branch_name:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "branch is required"}})
            return
        try:
            confirmed = call_repo_method(
                self.state.repo,
                "branch.confirm",
                {
                    "run_id": run_id,
                    "branch": branch_name,
                    "create": create,
                    "use_current": use_current,
                },
            )
            call_repo_method(self.state.repo, "run.start", {"run_id": run_id})
        except ServiceDaemonRpcError as exc:
            error: dict[str, Any] = {"code": exc.status, "message": exc.message}
            if exc.kind is not None:
                error["kind"] = exc.kind
            self._send_json(exc.status, {"ok": False, "error": error})
            return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": f"{type(exc).__name__}: {exc}"}})
            return
        self._send_json(200, {"ok": True, "data": {"run_id": run_id, "state": "running", "branch": confirmed.get("branch")}})

    def _handle_run_cancel(self, run_id: str) -> None:
        """RFC 0024 V3: cancel a run from the web UI.

        Mutation-gated. Idempotent: re-cancelling an already-canceled
        run returns 200 with the same payload.
        """
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "cancel run requires --allow-mutations"}})
            return
        ctype = self.headers.get("Content-Type", "")
        if "application/json" not in ctype:
            self._send_json(415, {"ok": False, "error": {"code": 415, "message": "Content-Type must be application/json"}})
            return
        try:
            length = int(self.headers.get("Content-Length") or "0")
        except ValueError:
            length = 0
        if length > 64 * 1024:
            self._send_json(413, {"ok": False, "error": {"code": 413, "message": "body too large"}})
            return
        try:
            raw = self.rfile.read(length).decode("utf-8", errors="replace") if length > 0 else "{}"
        except OSError as exc:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": str(exc)}})
            return
        try:
            body = json.loads(raw or "{}")
        except json.JSONDecodeError as exc:
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": f"invalid JSON: {exc}"}})
            return
        if not isinstance(body, dict):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "body must be a JSON object"}})
            return
        reason = body.get("reason") if isinstance(body.get("reason"), str) else None
        try:
            result = call_repo_method(self.state.repo, "run.cancel", {"run_id": run_id, "reason": reason})
        except ServiceDaemonRpcError as exc:
            if _legacy_fixture_fallback_enabled(exc.code):
                try:
                    result = _legacy_run_cancel(self.state.repo, run_id=run_id, reason=reason)
                except Exception as legacy_exc:  # noqa: BLE001 - mapped below.
                    if _send_legacy_fixture_error(self, legacy_exc):
                        return
                    raise
                self._send_json(200, {"ok": True, "data": result})
                return
            error: dict[str, Any] = {"code": exc.status, "message": exc.message}
            if exc.kind is not None:
                error["kind"] = exc.kind
            self._send_json(exc.status, {"ok": False, "error": error})
            return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": f"{type(exc).__name__}: {exc}"}})
            return
        self._send_json(200, {"ok": True, "data": result})

    def _handle_run_pause(self, run_id: str) -> None:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "pause requires --allow-mutations"}})
            return
        body = self._read_json_body_strict(64 * 1024)
        if body is None:
            return
        reason = body.get("reason") if isinstance(body.get("reason"), str) else None
        try:
            result = call_repo_method(self.state.repo, "run.pause", {"run_id": run_id, "reason": reason})
        except ServiceDaemonRpcError as exc:
            if _legacy_fixture_fallback_enabled(exc.code):
                try:
                    result = _legacy_run_pause(self.state.repo, run_id=run_id, reason=reason)
                except Exception as legacy_exc:  # noqa: BLE001 - mapped below.
                    if _send_legacy_fixture_error(self, legacy_exc):
                        return
                    raise
                self._send_json(200, {"ok": True, "data": result})
                return
            error: dict[str, Any] = {"code": exc.status, "message": exc.message}
            if exc.kind is not None:
                error["kind"] = exc.kind
            self._send_json(exc.status, {"ok": False, "error": error})
            return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": f"{type(exc).__name__}: {exc}"}})
            return
        self._send_json(200, {"ok": True, "data": result})

    def _handle_run_resume(self, run_id: str) -> None:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "resume requires --allow-mutations"}})
            return
        body = self._read_json_body_strict(64 * 1024)
        if body is None:
            return
        try:
            result = call_repo_method(self.state.repo, "run.resume", {"run_id": run_id})
        except ServiceDaemonRpcError as exc:
            if _legacy_fixture_fallback_enabled(exc.code):
                try:
                    result = _legacy_run_resume(self.state.repo, run_id=run_id)
                except Exception as legacy_exc:  # noqa: BLE001 - mapped below.
                    if _send_legacy_fixture_error(self, legacy_exc):
                        return
                    raise
                self._send_json(200, {"ok": True, "data": result})
                return
            error: dict[str, Any] = {"code": exc.status, "message": exc.message}
            if exc.kind is not None:
                error["kind"] = exc.kind
            self._send_json(exc.status, {"ok": False, "error": error})
            return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": f"{type(exc).__name__}: {exc}"}})
            return
        self._send_json(200, {"ok": True, "data": result})

    def _handle_job_action(self, path: str) -> None:
        """RFC 0024 V4: per-job cancel + retry. Path: /run/<id>/job/<jid>/(cancel|retry)."""
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "job action requires --allow-mutations"}})
            return
        # Parse /run/<rid>/job/<jid>/<action>.
        parts = path.strip("/").split("/")
        if len(parts) != 5 or parts[0] != "run" or parts[2] != "job":
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid job-action path"}})
            return
        run_id, job_id, action = parts[1], parts[3], parts[4]
        body = self._read_json_body_strict(64 * 1024)
        if body is None:
            return
        try:
            if action == "cancel":
                raw_reason = body.get("reason")
                reason: str = raw_reason if isinstance(raw_reason, str) and raw_reason else "operator_canceled_via_web"
                cascade = bool(body.get("cascade", True))
                result = call_repo_method(
                    self.state.repo,
                    "recovery.cancel_job",
                    {"run_id": run_id, "job_id": job_id, "reason": reason, "cascade": cascade},
                )
            elif action == "retry":
                result = call_repo_method(
                    self.state.repo,
                    "run.retry_job",
                    {"run_id": run_id, "job_id": job_id},
                )
            else:
                self._send_json(400, {"ok": False, "error": {"code": 400, "message": f"unknown job action: {action}"}})
                return
        except ServiceDaemonRpcError as exc:
            if _legacy_fixture_fallback_enabled(exc.code):
                try:
                    if action == "cancel":
                        result = _legacy_job_cancel(
                            self.state.repo,
                            run_id=run_id,
                            job_id=job_id,
                            reason=reason,
                            cascade=cascade,
                        )
                    elif action == "retry":
                        result = _legacy_job_retry(
                            self.state.repo,
                            run_id=run_id,
                            job_id=job_id,
                        )
                    else:
                        raise
                except Exception as legacy_exc:  # noqa: BLE001 - mapped below.
                    if _send_legacy_fixture_error(self, legacy_exc):
                        return
                    raise
                self._send_json(200, {"ok": True, "data": result})
                return
            error: dict[str, Any] = {"code": exc.status, "message": exc.message}
            if exc.kind is not None:
                error["kind"] = exc.kind
            self._send_json(exc.status, {"ok": False, "error": error})
            return
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": f"{type(exc).__name__}: {exc}"}})
            return
        self._send_json(200, {"ok": True, "data": result})

    def _read_json_body_strict(self, max_bytes: int) -> "dict[str, Any] | None":
        return _request_io.read_json_body_strict(
            self.headers,
            self.rfile,
            self._send_json,
            max_bytes=max_bytes,
        )

    def _render_doctor_page(self) -> None:
        try:
            response = _doctor_page_response(
                self.state.repo,
                legacy_fallback_enabled=_legacy_web_read_fallback_enabled,
                legacy_payload=_legacy_doctor_page_payload,
            )
            html = _jinja_env().get_template("doctor.html").render(
                doctor=response.doctor,
                problem_groups=response.problem_groups,
            )
            self._send_html(200, html)
        except _DoctorPageError as exc:
            self._send_json(
                exc.status,
                {"ok": False, "error": {"code": exc.code, "message": exc.message}},
            )
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    # --- RFC 0023 V1 chat + view ---------------------------------------

    def _chat_config(self) -> Any | None:
        from striatum.web.chat_provider import ChatProviderConfig, ChatProviderError
        try:
            return ChatProviderConfig.from_env(os.environ)
        except ChatProviderError:
            return None

    def _render_chat_index_page(self) -> None:
        config = self._chat_config()
        sessions = _list_chat_sessions(self.state.repo) if config else []
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
        path = _chat_session_path(self.state.repo, session_id)
        if path is None or not path.is_file():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "chat session not found"}})
            return
        messages = _read_display_messages(
            path,
            markdown_render=md_render,
            html_escape=_escape_html,
        )
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
        target = _chat_session_path(self.state.repo, session_id)
        if target is None:
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": "could not allocate chat session"}})
            return
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text("", encoding="utf-8")
        # RFC 0023 V1.5: seed the transcript with the system briefing
        # so the model has bearings on its first turn.
        try:
            briefing = _build_chat_briefing(self.state.repo, allow_mutations=self.state.allow_mutations)
            _append_jsonl(
                target,
                {"role": "system", "content": briefing,
                 "created_at": _utc_now_iso()},
            )
        except Exception:  # noqa: BLE001
            pass  # Briefing failure must not block chat creation.
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
        path = _chat_session_path(self.state.repo, session_id)
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
        # RFC 0023 V1.5: tool-call loop. Up to MAX_TOOL_ITERATIONS
        # rounds of (request → assistant text + tool calls → execute
        # tools → re-request with results). Loop terminates when the
        # model returns a stop without tool calls.
        from striatum.web.chat_provider import stream_chat_response, ChatProviderError
        from striatum.web.chat_tools import (
            execute_tool, tool_schemas, wrap_tool_result,
        )
        tools = tool_schemas(allow_mutations=self.state.allow_mutations, flavor=config.flavor)
        max_iterations = 10
        try:
            for iteration in range(max_iterations):
                history = _read_chat_history(path, flavor=config.flavor)
                # Pull the briefing system message out (Anthropic API
                # uses a top-level `system` field; OpenAI accepts it
                # as a system-role message).
                system_text, conversational = _split_system(history)
                tool_calls: list[dict[str, Any]] = []
                assistant_text = ""
                for event in stream_chat_response(
                    config, conversational,
                    tools=tools, system=system_text,
                ):
                    etype = event.get("type")
                    if etype == "text":
                        chunk = str(event.get("text") or "")
                        if chunk:
                            assistant_text += chunk
                    elif etype == "tool_call":
                        tool_calls.append(event)
                    elif etype == "stop":
                        break
                # Persist assistant text (if any) before tool execution.
                if assistant_text:
                    _append_jsonl(
                        path,
                        {"role": "assistant", "content": assistant_text,
                         "streaming": False, "created_at": _utc_now_iso()},
                    )
                if not tool_calls:
                    break
                # Persist + execute each tool call.
                for call in tool_calls:
                    tool_id = str(call.get("id") or "")
                    tool_name = str(call.get("name") or "")
                    tool_args = call.get("args") or {}
                    if not isinstance(tool_args, dict):
                        tool_args = {}
                    _append_jsonl(
                        path,
                        {"role": "tool_use", "tool_use_id": tool_id,
                         "tool_name": tool_name, "tool_input": tool_args,
                         "created_at": _utc_now_iso()},
                    )
                    if tool_name == "generate_workflow_write":
                        raw_result = _queue_workflow_write_confirmation(
                            path=path,
                            session_id=session_id,
                            tool_id=tool_id,
                            tool_args=tool_args,
                            allow_mutations=self.state.allow_mutations,
                            token_factory=lambda: secrets.token_urlsafe(24),
                        )
                        wrapped = wrap_tool_result(tool_name, tool_args, raw_result)
                        _append_jsonl(
                            path,
                            {"role": "tool_result", "tool_use_id": tool_id,
                             "tool_name": tool_name, "result": wrapped,
                             "created_at": _utc_now_iso()},
                        )
                        continue
                    raw_result = execute_tool(
                        tool_name,
                        tool_args,
                        repo=self.state.repo,
                        allow_mutations=self.state.allow_mutations,
                    )
                    wrapped = wrap_tool_result(tool_name, tool_args, raw_result)
                    _append_jsonl(
                        path,
                        {"role": "tool_result", "tool_use_id": tool_id,
                         "tool_name": tool_name, "result": wrapped,
                         "created_at": _utc_now_iso()},
                    )
            else:
                # Loop hit cap.
                _append_jsonl(
                    path,
                    {"role": "system", "content":
                        f"[loop cap] tool-call loop hit {max_iterations} iterations; halted.",
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
                {"role": "system", "content": f"[unexpected error] {type(exc).__name__}: {exc}",
                 "created_at": _utc_now_iso()},
            )
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": "chat send failed"}})
            return
        self.send_response(204)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _handle_chat_confirm_tool(self, session_id: str, tool_id: str) -> None:
        if not _is_safe_id(session_id) or not _is_safe_id(tool_id):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": "invalid chat confirmation id"}})
            return
        if not self.state.allow_mutations:
            self._send_json(405, {"ok": False, "error": {"code": 405, "message": "mutations_disabled", "kind": "mutations_disabled"}})
            return
        path = _chat_session_path(self.state.repo, session_id)
        if path is None or not path.is_file():
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "chat session not found"}})
            return
        form = self._read_form_body(max_bytes=4096)
        if form is None:
            return
        token = (form.get("token") or [""])[0]
        action = (form.get("action") or ["confirm"])[0]
        pending = _find_pending_tool_confirmation(path=path, tool_id=tool_id)
        if pending is None:
            self._send_json(404, {"ok": False, "error": {"code": 404, "message": "confirmation not found"}})
            return
        if str(pending.get("confirmation_token") or "") != token:
            self._send_json(403, {"ok": False, "error": {"code": 403, "message": "confirmation token mismatch"}})
            return
        if action == "cancel":
            _append_jsonl(
                path,
                {"role": "system", "content": "[mutation_canceled] operator canceled generate_workflow_write",
                 "created_at": _utc_now_iso()},
            )
            _append_jsonl(path, {**pending, "state": "used", "used_at": _utc_now_iso(), "confirmation_token": "<used>"})
            self._send_json(200, {"ok": False, "error": {"code": "mutation_canceled", "message": "operator canceled workflow write"}})
            return
        raw_tool_args = pending.get("tool_input")
        tool_args: dict[str, Any] = dict(raw_tool_args) if isinstance(raw_tool_args, dict) else {}
        from striatum.web.chat_tools import execute_tool, wrap_tool_result

        raw_result = execute_tool(
            "generate_workflow_write",
            tool_args,
            repo=self.state.repo,
            allow_mutations=True,
            operator_confirmed=True,
        )
        _append_jsonl(path, {**pending, "state": "used", "used_at": _utc_now_iso(), "confirmation_token": "<used>"})
        _append_jsonl(
            path,
            {"role": "tool_result", "tool_use_id": tool_id,
             "tool_name": "generate_workflow_write",
             "result": wrap_tool_result("generate_workflow_write", tool_args, raw_result),
             "created_at": _utc_now_iso()},
        )
        if raw_result.startswith("[error]"):
            self._send_json(400, {"ok": False, "error": {"code": 400, "message": raw_result}})
            return
        self._send_json(200, json.loads(raw_result))

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
        path = _chat_session_path(self.state.repo, session_id)
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
        from striatum.web.view_file import ViewFileError, view_file_payload

        if not subpath:
            try:
                html = _jinja_env().get_template("view_tree.html").render(root_path="")
                self._send_html(200, html)
            except Exception as exc:  # noqa: BLE001
                self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})
            return
        try:
            ctx = view_file_payload(self.state.repo, subpath)
        except ViewFileError as exc:
            self._send_json(
                exc.status_code,
                {"ok": False, "error": {"code": exc.status_code, "message": exc.message}},
            )
            return
        ctx["run_breadcrumb"] = _legacy_view_file_run_breadcrumb(
            self.state.repo,
            rel_path=str(ctx["rel_path"]),
        )
        try:
            html = _jinja_env().get_template("view_file.html").render(**ctx)
            self._send_html(200, html)
        except Exception as exc:  # noqa: BLE001
            self._send_json(500, {"ok": False, "error": {"code": 500, "message": str(exc)}})

    def _read_form_body(self, *, max_bytes: int) -> dict[str, list[str]] | None:
        return _request_io.read_form_body(
            self.headers,
            self.rfile,
            self._send_json,
            max_bytes=max_bytes,
        )

    def _send_html(self, status: int, body: str) -> None:
        _request_io.send_html_response(self, status, body)

    def _serve_static_asset(self, relative: str) -> None:
        """RFC 0013 V1: serve a bundled SPA asset from striatum.web.static."""
        try:
            asset = _load_static_asset(relative)
        except StaticAssetError as exc:
            self._send_json(
                exc.status_code,
                {"ok": False, "error": {"code": exc.status_code, "message": exc.message}},
            )
            return
        self.send_response(200)
        self.send_header("Content-Type", asset.content_type)
        self.send_header("Content-Length", str(len(asset.data)))
        self.send_header(
            "Content-Security-Policy",
            "default-src 'self'; script-src 'self'; style-src 'self'; "
            "img-src 'self' data:; connect-src 'self'",
        )
        self.send_header("Connection", "close")
        self.end_headers()
        try:
            self.wfile.write(asset.data)
        except BrokenPipeError:
            return

    # --- SSE -----------------------------------------------------------

    def _sse_since(self, query: dict[str, list[str]]) -> int:
        return _sse_since(dict(self.headers.items()), query)

    def _stream_events(self, run_id: str, *, since: int) -> None:
        if not self.state.acquire_sse_slot(run_id):
            self._send_json(429, {"ok": False, "error": {"code": 429, "message": f"too many concurrent SSE streams for run {run_id}"}})
            return
        try:
            if _legacy_web_read_fallback_enabled("daemon_unreachable"):
                _legacy_stream_events_body(
                    self,
                    repo=self.state.repo,
                    run_id=run_id,
                    since=since,
                    poll_interval_seconds=SSE_POLL_INTERVAL_SECONDS,
                )
                return
            self._stream_events_daemon_body(run_id, since=since)
        finally:
            self.state.release_sse_slot(run_id)

    def _stream_events_daemon_body(self, run_id: str, *, since: int) -> None:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        last_id = since
        try:
            payload = call_repo_method(
                self.state.repo,
                "run.events",
                {"run_id": run_id, "since_event_id": last_id, "limit": 100},
            )
        except ServiceDaemonRpcError as exc:
            self._send_json(
                exc.status,
                {"ok": False, "error": {"code": exc.code, "message": exc.message}},
            )
            return
        try:
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.send_header("Connection", "close")
            self.send_header("X-Accel-Buffering", "no")
            self.end_headers()
            while not self.state.shutting_down:
                rows = payload.get("events")
                events = rows if isinstance(rows, list) else []
                for row in events:
                    if not isinstance(row, Mapping):
                        continue
                    event_id = int(row.get("event_id") or last_id)
                    self._write_sse_event("striatum.event", event_id, dict(row))
                    last_id = event_id
                run_raw = payload.get("run")
                run: Mapping[str, Any] = run_raw if isinstance(run_raw, Mapping) else {}
                run_state = str(run.get("state") or "")
                if run_state in {"completed", "failed", "canceled"} and not events:
                    self._write_sse_event(
                        "striatum.run_terminal",
                        last_id,
                        {"run_id": run_id, "state": run_state},
                    )
                    break
                time.sleep(SSE_POLL_INTERVAL_SECONDS)
                try:
                    payload = call_repo_method(
                        self.state.repo,
                        "run.events",
                        {"run_id": run_id, "since_event_id": last_id, "limit": 100},
                    )
                except ServiceDaemonRpcError as exc:
                    self._write_sse_event(
                        "striatum.error",
                        last_id,
                        {"run_id": run_id, "code": exc.code, "message": exc.message},
                    )
                    break
            if self.state.shutting_down:
                self._write_sse_event(
                    "striatum.shutdown",
                    last_id,
                    {"run_id": run_id, "reason": "service_shutting_down"},
                )
        except (BrokenPipeError, ConnectionResetError):
            return

    def _write_sse_event(self, event: str, event_id: int, payload: JsonObject) -> None:
        self.wfile.write(_encode_sse_event(event, event_id, payload))
        self.wfile.flush()

    # --- request helpers ----------------------------------------------

    def _authenticate(self) -> bool:
        decision = _authenticate_request(
            dict(self.headers.items()),
            token=self.state.token,
        )
        if decision.ok:
            return True
        self._send_security_decision(decision)
        return False

    def _has_valid_bearer(self) -> bool:
        """True when the request carries an Authorization: Bearer header
        matching the configured token. Used to grant authenticated
        non-browser API clients an exception to same-origin enforcement.
        """
        return _has_valid_bearer(dict(self.headers.items()), token=self.state.token)

    def _requires_same_origin(self, path: str) -> bool:
        return _requires_same_origin(
            path,
            origin_check_enabled=self.state.origin_check_enabled,
            web_enabled=self.state.web_enabled,
        )

    def _verify_same_origin_mutation(self) -> bool:
        """GH #9: reject cross-origin browser POSTs to the web UI's
        mutation surface. Returns True if the request may proceed,
        False if a 403 was already sent.

        Policy:
        - Authenticated Bearer-token clients are exempt — these are
          non-browser API clients that cannot be impersonated via CSRF.
        - The request Host must be one of the origins derived from the
          actual bound loopback listener; matching Host to Origin is not
          sufficient because of DNS rebinding.
        - ``Origin`` must match that allowlist. If Origin is absent,
          same-origin ``Referer`` is accepted instead.
        - Missing, ``null``, malformed, or cross-origin evidence fails
          closed with 403.
        """
        decision = _verify_same_origin_mutation(
            dict(self.headers.items()),
            token=self.state.token,
            allowed_origins=self.state.allowed_origins,
        )
        if decision.ok:
            return True
        self._send_security_decision(decision)
        return False

    def _verify_override_verdict_context(
        self,
        argv: list[str],
        body: JsonObject,
    ) -> bool:
        """GH #10: validate the ``web_context`` envelope on
        override-verdict POSTs. Returns True if the request may proceed,
        False if a 403 was already sent.

        The token is bound to the (run_id, job_id, session_id) tuple
        that the server rendered onto the page. argv ``--job-id`` and
        ``--session-id`` must match the context exactly; the context
        token must verify against the process secret. This prevents
        DOM-tampering attacks even when the browser is same-origin and
        the CSRF defenses pass.
        """
        decision = _verify_override_verdict_context(
            argv,
            body,
            web_context_secret=self.state.web_context_secret,
        )
        if decision.ok:
            return True
        self._send_security_decision(decision)
        return False

    def _send_security_decision(self, decision: SecurityDecision) -> None:
        error = decision.error or {"code": decision.status, "message": "request refused"}
        self._send_json(decision.status, {"ok": False, "error": error})

    def _read_json_body(self) -> JsonObject | None:
        return _request_io.read_json_body(self.headers, self.rfile, self._send_json)

    def _send_json(self, status: int, payload: JsonObject) -> None:
        _request_io.send_json_response(self, status, payload)


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


def _verify_service_startup(repo: Path) -> None:
    if _legacy_web_read_fallback_enabled("daemon_unreachable"):
        _legacy_verify_state_health(repo, error_type=ServiceConfigError)
        return
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    try:
        call_repo_method(repo, "doctor", {"verbose": False})
    except ServiceDaemonRpcError as exc:
        raise ServiceConfigError(
            f"refusing to start serve: daemon doctor failed with {exc.code}: {exc.message}"
        ) from exc


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
    _verify_service_startup(repo)
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
    state.origin_check_enabled = True
    state.allowed_origins = allowed_origins_for_bind(str(bound_host), int(bound_port))
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
    _verify_service_startup(repo)
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
        _wait_for_service_shutdown(
            shutdown_event,
            idle_timeout_seconds=idle_timeout_seconds,
        )
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

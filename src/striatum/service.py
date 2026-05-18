"""RFC 0012 V1: local HTTP + Unix-socket service.

Operationalizes D006's promise of an "optional Unix-socket / local HTTP
API later for Slack, TUI, and web adapters." Production state-changing
endpoints and migrated read pages delegate to daemon RPC; repo-local SQLite
compatibility is isolated under ``striatum.legacy_sqlite`` for explicit
subprocess test-harness paths.
Localhost-only by default; non-loopback hosts are refused at startup.
Mutations are gated behind ``--allow-mutations``.
"""

from __future__ import annotations

import os
import secrets
import time  # noqa: F401 - compatibility monkeypatch seam for legacy SSE tests.
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any, Mapping

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
from striatum import service_api_routes as _service_api_routes
from striatum import service_request_io as _request_io
from striatum import service_routes as _service_routes
from striatum import service_server as _service_server
from striatum.service_api_routes import (
    ServiceApiRouteContext as _ServiceApiRouteContext,
)
from striatum.service_runtime import (
    ServiceAlreadyRunningError as ServiceAlreadyRunningError,
    ServiceConfigError as ServiceConfigError,
    service_mode as _service_mode,
    striatum_version as _striatum_version,
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
from striatum.web import chat_routes as _chat_routes
from striatum.web.chat_routes import (
    ChatRouteContext as _ChatRouteContext,
)
from striatum.web import template_env as _template_env
from striatum.web.static_assets import (
    StaticAssetError as StaticAssetError,
    load_static_asset as _load_static_asset,
)
from striatum.web.artifacts import (
    ArtifactRawContext as _ArtifactRawContext,
    byline_line as _byline_line_web,
    serve_artifact_raw as _serve_artifact_raw,
    shape_artifact_rows as _shape_artifact_rows_web,
)
from striatum.web import run_actions as _run_actions
from striatum.web.run_actions import RunActionContext as _RunActionContext
from striatum.web import run_pages as _run_pages
from striatum.web.run_pages import RunPageContext as _RunPageContext
from striatum.web import workflows as _workflows
from striatum.web.workflows import WorkflowRouteContext as _WorkflowRouteContext
from striatum.web import view_file as _view_file
from striatum.web.view_file import ViewRouteContext as _ViewRouteContext
from striatum.web.workflow_generation import (
    generator_error_response as _generator_error_response,
    workflow_generate_response as _workflow_generate_response,
    workflow_template_show_response as _workflow_template_show_response,
    workflow_templates_response as _workflow_templates_response,
)

JsonObject = dict[str, Any]
_project_history_anthropic = _chat_session.project_history_anthropic
_project_history_openai = _chat_session.project_history_openai
_safe_git = _chat_session.safe_git


def invoke(argv: list[str], *, repo: Path) -> JsonObject:
    """Compatibility wrapper for tests that monkeypatch ``service.invoke``."""

    from striatum import api as _api

    return _api.invoke(argv, repo=repo)


def _legacy_service() -> Any:
    from striatum.legacy_sqlite import service as legacy_service

    return legacy_service


class _LazyLegacyCallable:
    def __init__(self, name: str) -> None:
        self._name = name

    def __call__(self, *args: Any, **kwargs: Any) -> Any:
        return getattr(_legacy_service(), self._name)(*args, **kwargs)


_shape_verdict_rows = _LazyLegacyCallable("_shape_verdict_rows")
_legacy_artifact_metadata = _LazyLegacyCallable("legacy_artifact_metadata")
_legacy_artifact_raw_fallback_enabled = _LazyLegacyCallable(
    "legacy_artifact_raw_fallback_enabled"
)
_legacy_artifact_view_payload = _LazyLegacyCallable("legacy_artifact_view_payload")
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
_legacy_stream_events_body = _LazyLegacyCallable("legacy_stream_events_body")
_legacy_verify_state_health = _LazyLegacyCallable("legacy_verify_state_health")
_legacy_web_read_fallback_enabled = _LazyLegacyCallable("legacy_web_read_fallback_enabled")
_legacy_workflow_run_now = _LazyLegacyCallable("legacy_workflow_run_now")
_send_legacy_fixture_error = _LazyLegacyCallable("send_legacy_fixture_error")
_send_legacy_run_now_error = _LazyLegacyCallable("send_legacy_run_now_error")
_short_git_status = _LazyLegacyCallable("short_git_status")

SSE_POLL_INTERVAL_SECONDS = 0.25


def _shape_artifact_rows(
    _conn: object = None,
    *,
    artifacts: list[dict[str, Any]],
    expected_rows: list[Mapping[str, Any]],
) -> list[dict[str, Any]]:
    del _conn
    return _shape_artifact_rows_web(artifacts=artifacts, expected_rows=expected_rows)


def _byline_line(
    author_line: Any,
    *,
    expected_author_line: Any = None,
    attested: bool | None = None,
    operator_label: Any = None,
) -> dict[str, Any]:
    return _byline_line_web(
        author_line,
        expected_author_line=expected_author_line,
        attested=attested,
        operator_label=operator_label,
    )


def _handler_send_json(handler: BaseHTTPRequestHandler, status: int, body: Mapping[str, Any]) -> None:
    send_json = getattr(handler, "_send_json")
    send_json(status, dict(body))


SHUTDOWN_DRAIN_SECONDS = 5.0
_ThreadedTCPServer = _service_server.ThreadedTCPServer
_ThreadedUnixServer = _service_server.ThreadedUnixServer


def _is_safe_id(value: str) -> bool:
    """RFC 0023 V1: chat session ids and similar paths must be ASCII
    alphanumeric / underscore / hyphen only."""
    return _chat_routes.is_safe_id(value)


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
        _service_routes.dispatch_get(self)

    def _dispatch_post(self) -> None:
        _service_routes.dispatch_post(self)

    # --- endpoint helpers ----------------------------------------------

    def _api_route_context(self) -> _ServiceApiRouteContext:
        return _ServiceApiRouteContext(
            state=self.state,
            server=getattr(self, "server", None),
            headers=getattr(self, "headers", {}),
            writer=self,
            send_json=self._send_json,
            invoke_func=lambda argv: invoke(argv, repo=self.state.repo),
            striatum_version=_striatum_version,
            service_mode=_service_mode,
            legacy_web_read_fallback_enabled=_legacy_web_read_fallback_enabled,
            legacy_stream_events_body=_legacy_stream_events_body,
            poll_interval_seconds=SSE_POLL_INTERVAL_SECONDS,
        )

    def _handle_health(self) -> None:
        _service_api_routes.handle_health(self._api_route_context())

    def _handle_invoke(self, argv: list[str]) -> None:
        _service_api_routes.handle_invoke(self._api_route_context(), argv)

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
        _service_api_routes.handle_doctor(self._api_route_context(), query)

    def _handle_repo_tree(self, query: dict[str, list[str]]) -> None:
        _service_api_routes.handle_repo_tree(self._api_route_context(), query)

    def _handle_run_subpath(self, suffix: str, query: dict[str, list[str]]) -> None:
        _service_api_routes.handle_run_subpath(
            self._api_route_context(),
            suffix,
            query,
        )

    def _handle_daemon_read(
        self,
        method: str,
        params: Mapping[str, Any],
        *,
        legacy_argv: list[str] | None = None,
    ) -> None:
        _service_api_routes.handle_daemon_read(
            self._api_route_context(),
            method,
            params,
            legacy_argv=legacy_argv,
        )

    def _handle_artifact_raw(self, artifact_id: str) -> None:
        """RFC 0013 V1: serve the raw bytes of an artifact for the web UI viewer.

        Looks up artifact metadata through the daemon, opens the file at
        ``repo_path``, and streams the bytes back. Read-only; no mutation gate.
        Returns 404 if the row or the file is missing.
        """
        _serve_artifact_raw(
            _ArtifactRawContext(
                repo=self.state.repo,
                send_json=self._send_json,
                send_response=self.send_response,
                send_header=self.send_header,
                end_headers=self.end_headers,
                write_body=self.wfile.write,
                legacy_artifact_raw_fallback_enabled=_legacy_artifact_raw_fallback_enabled,
                legacy_artifact_metadata=_legacy_artifact_metadata,
            ),
            artifact_id,
        )

    # --- RFC 0022 V1 page rendering -----------------------------------

    def _run_page_context(self) -> _RunPageContext:
        return _RunPageContext(
            state=self.state,
            send_json=self._send_json,
            send_html=self._send_html,
            jinja_env=_jinja_env,
            legacy_web_read_fallback_enabled=_legacy_web_read_fallback_enabled,
            legacy_run_list_items_for_test_harness=_legacy_run_list_items_for_test_harness,
            legacy_run_posture_verdicts_payload=_legacy_run_posture_verdicts_payload,
            legacy_run_detail_payload=_legacy_run_detail_payload,
            legacy_job_detail_payload=_legacy_job_detail_payload,
            legacy_artifact_view_payload=_legacy_artifact_view_payload,
        )

    def _render_run_list_page(self) -> None:
        _run_pages.render_run_list_page(self._run_page_context())

    def _render_run_subpath(self, subpath: str) -> None:
        _run_pages.render_run_subpath(self._run_page_context(), subpath)

    def _render_run_posture_verdicts_page(self, run_id: str, posture: str) -> None:
        _run_pages.render_run_posture_verdicts_page(
            self._run_page_context(),
            run_id,
            posture,
        )

    def _render_run_detail_page(self, run_id: str) -> None:
        _run_pages.render_run_detail_page(self._run_page_context(), run_id)

    def _render_job_detail_page(self, run_id: str, job_id: str) -> None:
        _run_pages.render_job_detail_page(self._run_page_context(), run_id, job_id)

    def _render_artifact_view_page(self, run_id: str, artifact_id: str) -> None:
        _run_pages.render_artifact_view_page(
            self._run_page_context(),
            run_id,
            artifact_id,
        )

    def _render_workflows_index_page(self) -> None:
        _workflows.render_workflows_index_page(self._workflow_route_context())

    def _render_workflow_detail_page(self, rel_path: str) -> None:
        _workflows.render_workflow_detail_page(
            self._workflow_route_context(),
            rel_path,
        )

    def _workflow_route_context(self) -> _WorkflowRouteContext:
        return _WorkflowRouteContext(
            repo=self.state.repo,
            allow_mutations=self.state.allow_mutations,
            headers=self.headers,
            rfile=self.rfile,
            send_json=self._send_json,
            send_html=self._send_html,
            jinja_env=_jinja_env,
        )

    def _run_action_context(self) -> _RunActionContext:
        return _RunActionContext(
            repo=self.state.repo,
            allow_mutations=self.state.allow_mutations,
            headers=self.headers,
            rfile=self.rfile,
            send_json=self._send_json,
            read_json_body_strict=self._read_json_body_strict,
            legacy_error_handler=self,
            legacy_web_read_fallback_enabled=_legacy_web_read_fallback_enabled,
            legacy_fixture_fallback_enabled=_legacy_fixture_fallback_enabled,
            legacy_workflow_run_now=_legacy_workflow_run_now,
            legacy_run_cancel=_legacy_run_cancel,
            legacy_run_pause=_legacy_run_pause,
            legacy_run_resume=_legacy_run_resume,
            legacy_job_cancel=_legacy_job_cancel,
            legacy_job_retry=_legacy_job_retry,
            send_legacy_run_now_error=_send_legacy_run_now_error,
            send_legacy_fixture_error=_send_legacy_fixture_error,
            short_git_status=_short_git_status,
        )

    def _handle_workflow_run_now(self, rel_path: str) -> None:
        _run_actions.handle_workflow_run_now(self._run_action_context(), rel_path)

    def _handle_run_branch_confirm(self, run_id: str) -> None:
        _run_actions.handle_run_branch_confirm(self._run_action_context(), run_id)

    def _handle_run_cancel(self, run_id: str) -> None:
        _run_actions.handle_run_cancel(self._run_action_context(), run_id)

    def _handle_run_pause(self, run_id: str) -> None:
        _run_actions.handle_run_pause(self._run_action_context(), run_id)

    def _handle_run_resume(self, run_id: str) -> None:
        _run_actions.handle_run_resume(self._run_action_context(), run_id)

    def _handle_job_action(self, path: str) -> None:
        _run_actions.handle_job_action(self._run_action_context(), path)

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

    def _chat_route_context(self) -> _ChatRouteContext:
        return _ChatRouteContext(
            repo=self.state.repo,
            allow_mutations=self.state.allow_mutations,
            chat_config=self._chat_config,
            read_form_body=lambda max_bytes: self._read_form_body(
                max_bytes=max_bytes
            ),
            send_json=self._send_json,
            send_html=self._send_html,
            send_response=self.send_response,
            send_header=self.send_header,
            end_headers=self.end_headers,
            wfile=self.wfile,
            jinja_env=_jinja_env,
            build_chat_briefing=lambda repo, allow_mutations: _build_chat_briefing(
                repo,
                allow_mutations=allow_mutations,
            ),
            html_escape=_escape_html,
            token_factory=lambda: secrets.token_urlsafe(24),
            poll_interval_seconds=SSE_POLL_INTERVAL_SECONDS,
        )

    def _render_chat_index_page(self) -> None:
        _chat_routes.render_chat_index_page(self._chat_route_context())

    def _render_chat_subpath(self, subpath: str) -> None:
        _chat_routes.render_chat_subpath(self._chat_route_context(), subpath)

    def _render_chat_session_page(self, session_id: str) -> None:
        _chat_routes.render_chat_session_page(self._chat_route_context(), session_id)

    def _handle_chat_new(self) -> None:
        _chat_routes.handle_chat_new(self._chat_route_context())

    def _handle_chat_send(self, session_id: str) -> None:
        _chat_routes.handle_chat_send(self._chat_route_context(), session_id)

    def _handle_chat_confirm_tool(self, session_id: str, tool_id: str) -> None:
        _chat_routes.handle_chat_confirm_tool(
            self._chat_route_context(),
            session_id,
            tool_id,
        )

    def _handle_chat_stop(self, session_id: str) -> None:
        _chat_routes.handle_chat_stop(self._chat_route_context(), session_id)

    def _stream_chat_events(self, session_id: str) -> None:
        _chat_routes.stream_chat_events(self._chat_route_context(), session_id)

    def _view_route_context(self) -> _ViewRouteContext:
        return _ViewRouteContext(
            repo=self.state.repo,
            send_json=self._send_json,
            send_html=self._send_html,
            jinja_env=_jinja_env,
        )

    def _render_view_path(self, subpath: str) -> None:
        _view_file.render_view_path(self._view_route_context(), subpath)

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
        return _service_api_routes.sse_since(self._api_route_context(), query)

    def _stream_events(self, run_id: str, *, since: int) -> None:
        _service_api_routes.stream_events(
            self._api_route_context(),
            run_id,
            since=since,
        )

    def _stream_events_daemon_body(self, run_id: str, *, since: int) -> None:
        _service_api_routes.stream_events_daemon_body(
            self._api_route_context(),
            run_id,
            since=since,
        )

    def _write_sse_event(
        self,
        event: str,
        event_id: int,
        payload: JsonObject,
    ) -> None:
        _service_api_routes.write_sse_event(
            self._api_route_context(),
            event,
            event_id,
            payload,
        )

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
    return _service_server.run_tcp(
        repo=repo,
        host=host,
        port=port,
        token=token,
        allow_mutations=allow_mutations,
        idle_timeout_seconds=idle_timeout_seconds,
        web_enabled=web_enabled,
        make_handler=_make_handler,
        verify_startup=_verify_service_startup,
        shutdown_drain_seconds=SHUTDOWN_DRAIN_SECONDS,
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
    return _service_server.run_unix(
        repo=repo,
        unix_path=unix_path,
        token=None,
        allow_mutations=allow_mutations,
        idle_timeout_seconds=idle_timeout_seconds,
        web_enabled=web_enabled,
        make_handler=_make_handler,
        verify_startup=_verify_service_startup,
        shutdown_drain_seconds=SHUTDOWN_DRAIN_SECONDS,
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
    return _service_server.serve_forever(
        server=server,
        state=state,
        pid_path=pid_path,
        unix_path=unix_path,
        bind_summary=bind_summary,
        idle_timeout_seconds=idle_timeout_seconds,
        web_enabled=web_enabled,
        shutdown_drain_seconds=SHUTDOWN_DRAIN_SECONDS,
    )

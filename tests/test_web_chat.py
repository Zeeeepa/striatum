# ruff: noqa
"""RFC 0023 V1: web chat surface tests.

Spawn `striatum serve --web --allow-mutations` (with chat env vars
set against a fake provider) and exercise the chat lifecycle.
"""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite / web_ui test helpers are retired", allow_module_level=True)

import json
import os
import socket as _socket
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import cast

import pytest

from test_web_ui import (
    _git_init_repo,
    _http_get_raw,
    _stop_service,
    _striatum_init,
)


ROOT = Path(__file__).resolve().parents[1]


# --- Fake provider ---------------------------------------------------


class FakeOpenAIChatHandler(BaseHTTPRequestHandler):
    """Streams a fixed two-chunk OpenAI Chat-shaped response."""

    received_body: dict[str, object] | None = None
    received_headers: dict[str, object] | None = None

    def log_message(self, format: str, *args: object) -> None:  # noqa: A002
        pass

    def do_POST(self) -> None:  # noqa: N802
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)
        FakeOpenAIChatHandler.received_body = json.loads(raw)
        FakeOpenAIChatHandler.received_headers = dict(self.headers.items())
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        chunks = [
            'data: {"choices":[{"delta":{"content":"Hel"}}]}\n\n',
            'data: {"choices":[{"delta":{"content":"lo"}}]}\n\n',
            "data: [DONE]\n\n",
        ]
        for chunk in chunks:
            self.wfile.write(chunk.encode("utf-8"))
            self.wfile.flush()


class FakeOpenAIWorkflowWriteHandler(BaseHTTPRequestHandler):
    """Streams one generate_workflow_write tool call."""

    received_body: dict[str, object] | None = None

    def log_message(self, format: str, *args: object) -> None:  # noqa: A002
        pass

    def do_POST(self) -> None:  # noqa: N802
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)
        FakeOpenAIWorkflowWriteHandler.received_body = json.loads(raw)
        spec = {
            "schema_version": "striatum.workflow_generator.v1",
            "shape": "minimal",
            "lane_set": "local",
            "workflow_id": "chat-demo",
            "name": "Chat Demo",
            "workflow_version": "2026-05-12",
            "branch": {"mode": "confirm", "suggested_name": "striatum/chat-demo", "allow_dirty": False},
            "scaffold_root": "workflows/chat-demo",
            "artifact_root": "striatum/chat-demo",
            "lanes": {},
            "options": {},
        }
        args = json.dumps({"spec": spec, "confirm_write": True}, separators=(",", ":"))
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        payload = {
            "choices": [
                {
                    "delta": {
                        "tool_calls": [
                            {
                                "index": 0,
                                "id": "call_write",
                                "type": "function",
                                "function": {
                                    "name": "generate_workflow_write",
                                    "arguments": args,
                                },
                            }
                        ]
                    }
                }
            ]
        }
        self.wfile.write(("data: " + json.dumps(payload) + "\n\n").encode("utf-8"))
        self.wfile.write('data: {"choices":[{"finish_reason":"tool_calls","delta":{}}]}\n\n'.encode("utf-8"))
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()


class FakeAnthropicHandler(BaseHTTPRequestHandler):
    received_body: dict[str, object] | None = None
    received_headers: dict[str, object] | None = None

    def log_message(self, format: str, *args: object) -> None:  # noqa: A002
        pass

    def do_POST(self) -> None:  # noqa: N802
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)
        FakeAnthropicHandler.received_body = json.loads(raw)
        FakeAnthropicHandler.received_headers = dict(self.headers.items())
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        events = [
            'event: content_block_delta\ndata: {"delta":{"type":"text_delta","text":"Hi"}}\n\n',
            'event: content_block_delta\ndata: {"delta":{"type":"text_delta","text":" there"}}\n\n',
            "event: message_stop\ndata: {}\n\n",
        ]
        for event in events:
            self.wfile.write(event.encode("utf-8"))
            self.wfile.flush()


def _start_fake_server(handler_cls: type[BaseHTTPRequestHandler]) -> tuple[HTTPServer, int]:
    server = HTTPServer(("127.0.0.1", 0), handler_cls)
    port = server.server_address[1]
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, port


def _spawn_service_with_chat(
    repo: Path, *, base_url: str, flavor: str, model: str = "test-model",
) -> tuple[subprocess.Popen[bytes], int]:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    env["STRIATUM_CHAT_API_BASE_URL"] = base_url
    env["STRIATUM_CHAT_API_KEY"] = "fake-key"
    env["STRIATUM_CHAT_MODEL"] = model
    env["STRIATUM_CHAT_API_FLAVOR"] = flavor
    env["STRIATUM_LEGACY_SERVICE_FIXTURE"] = "1"
    with _socket.socket(_socket.AF_INET, _socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        port = cast(int, sock.getsockname()[1])
    proc = subprocess.Popen(
        [
            sys.executable, "-m", "striatum.cli",
            "--repo", str(repo), "serve",
            "--host", "127.0.0.1", "--port", str(port),
            "--web", "--allow-mutations",
            "--idle-timeout-seconds", "30",
        ],
        cwd=repo, env=env,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    deadline = time.time() + 10.0
    while time.time() < deadline:
        try:
            with _socket.create_connection(("127.0.0.1", port), timeout=0.5):
                return proc, port
        except (ConnectionRefusedError, _socket.timeout, OSError):
            time.sleep(0.1)
        if proc.poll() is not None:
            stdout, stderr = proc.communicate(timeout=2)
            raise AssertionError(
                f"service exited: rc={proc.returncode} stderr={stderr!r}"
            )
    raise AssertionError("service did not respond on port within 10s")


class _NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    def http_error_303(self, req, fp, code, msg, headers):  # type: ignore[no-untyped-def]
        # Surface as HTTPError so we can read Location.
        raise urllib.error.HTTPError(req.full_url, code, msg, headers, fp)


def _http_post_form(port: int, path: str, fields: dict[str, str]) -> tuple[int, dict[str, str], bytes]:
    body = "&".join(
        f"{urllib.parse.quote(k)}={urllib.parse.quote(v)}"
        for k, v in fields.items()
    ).encode("utf-8")
    req = urllib.request.Request(
        f"http://127.0.0.1:{port}{path}",
        data=body,
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "Origin": f"http://127.0.0.1:{port}",
        },
    )
    opener = urllib.request.build_opener(_NoRedirectHandler())
    try:
        with opener.open(req, timeout=10) as resp:
            return resp.status, dict(resp.headers.items()), resp.read()
    except urllib.error.HTTPError as exc:
        return exc.code, dict(exc.headers.items()) if exc.headers else {}, exc.read()


# --- Tests -----------------------------------------------------------


def test_chat_disabled_without_env_vars(tmp_path: Path) -> None:
    """`/chat` index renders the empty-state when no chat env vars."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    env["STRIATUM_LEGACY_SERVICE_FIXTURE"] = "1"
    # Strip any existing chat env vars so we test the empty-state.
    for key in (
        "STRIATUM_CHAT_API_BASE_URL", "STRIATUM_CHAT_API_KEY",
        "STRIATUM_CHAT_MODEL", "STRIATUM_CHAT_API_FLAVOR",
    ):
        env.pop(key, None)
    with _socket.socket(_socket.AF_INET, _socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        port = cast(int, sock.getsockname()[1])
    proc = subprocess.Popen(
        [
            sys.executable, "-m", "striatum.cli",
            "--repo", str(tmp_path), "serve",
            "--host", "127.0.0.1", "--port", str(port),
            "--web", "--idle-timeout-seconds", "30",
        ],
        cwd=tmp_path, env=env,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    deadline = time.time() + 10.0
    while time.time() < deadline:
        try:
            with _socket.create_connection(("127.0.0.1", port), timeout=0.5):
                break
        except (ConnectionRefusedError, OSError):
            time.sleep(0.1)
    try:
        status, headers, body = _http_get_raw(port, "/chat")
        assert status == 200
        assert b"Chat is not configured" in body
        assert b"STRIATUM_CHAT_API_BASE_URL" in body
    finally:
        _stop_service(proc)


def test_post_chat_new_creates_session(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    server, prov_port = _start_fake_server(FakeOpenAIChatHandler)
    try:
        proc, port = _spawn_service_with_chat(
            tmp_path,
            base_url=f"http://127.0.0.1:{prov_port}",
            flavor="openai_chat",
        )
        try:
            status, headers, body = _http_post_form(port, "/chat/new", {})
            assert status == 303
            location = headers.get("Location", "")
            assert location.startswith("/chat/")
            session_id = location[len("/chat/"):]
            scratch_path = tmp_path / ".striatum" / "scratch" / f"chat-{session_id}" / "transcript.jsonl"
            assert scratch_path.exists()
        finally:
            _stop_service(proc)
    finally:
        server.shutdown()


def test_chat_send_openai_flavor_request_shape(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    FakeOpenAIChatHandler.received_body = None
    server, prov_port = _start_fake_server(FakeOpenAIChatHandler)
    try:
        proc, port = _spawn_service_with_chat(
            tmp_path,
            base_url=f"http://127.0.0.1:{prov_port}",
            flavor="openai_chat",
        )
        try:
            # Create session.
            status, headers, _ = _http_post_form(port, "/chat/new", {})
            session_id = headers["Location"][len("/chat/"):]
            # Send a message.
            status, _, _ = _http_post_form(
                port, f"/chat/{session_id}/send", {"message": "hello world"},
            )
            assert status == 204
            assert FakeOpenAIChatHandler.received_body is not None
            body = FakeOpenAIChatHandler.received_body
            assert body["model"] == "test-model"
            assert body["stream"] is True
            assert any(
                m.get("role") == "user" and m.get("content") == "hello world"
                for m in body["messages"]
            )
            transcript = (
                tmp_path / ".striatum" / "scratch" / f"chat-{session_id}"
                / "transcript.jsonl"
            ).read_text(encoding="utf-8")
            assert "hello world" in transcript
            assert "Hel" in transcript or "lo" in transcript
        finally:
            _stop_service(proc)
    finally:
        server.shutdown()


def test_chat_workflow_write_requires_operator_confirmation(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    FakeOpenAIWorkflowWriteHandler.received_body = None
    server, prov_port = _start_fake_server(FakeOpenAIWorkflowWriteHandler)
    try:
        proc, port = _spawn_service_with_chat(
            tmp_path,
            base_url=f"http://127.0.0.1:{prov_port}",
            flavor="openai_chat",
        )
        try:
            status, headers, _ = _http_post_form(port, "/chat/new", {})
            session_id = headers["Location"][len("/chat/"):]
            status, _, _ = _http_post_form(
                port, f"/chat/{session_id}/send", {"message": "write a workflow"},
            )
            assert status == 204
            workflow_path = tmp_path / "workflows" / "chat-demo" / "workflow.json"
            assert not workflow_path.exists()
            transcript = (
                tmp_path / ".striatum" / "scratch" / f"chat-{session_id}" / "transcript.jsonl"
            )
            pending = None
            for raw in transcript.read_text(encoding="utf-8").splitlines():
                entry = json.loads(raw)
                if entry.get("role") == "tool_confirmation":
                    pending = entry
            assert pending is not None
            status, _, body = _http_post_form(
                port,
                f"/chat/{session_id}/confirm-tool/{pending['tool_use_id']}",
                {"token": str(pending["confirmation_token"]), "action": "confirm"},
            )
            assert status == 200, body
            assert workflow_path.exists()
        finally:
            _stop_service(proc)
    finally:
        server.shutdown()


def test_chat_send_anthropic_flavor_request_shape(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    FakeAnthropicHandler.received_body = None
    FakeAnthropicHandler.received_headers = None
    server, prov_port = _start_fake_server(FakeAnthropicHandler)
    try:
        proc, port = _spawn_service_with_chat(
            tmp_path,
            base_url=f"http://127.0.0.1:{prov_port}",
            flavor="anthropic_messages",
        )
        try:
            status, headers, _ = _http_post_form(port, "/chat/new", {})
            session_id = headers["Location"][len("/chat/"):]
            status, _, _ = _http_post_form(
                port, f"/chat/{session_id}/send", {"message": "hi"},
            )
            assert status == 204
            assert FakeAnthropicHandler.received_body is not None
            body = FakeAnthropicHandler.received_body
            hdrs = FakeAnthropicHandler.received_headers or {}
            assert "x-api-key" in {k.lower() for k in hdrs}
            # V1.5 projects user turns to Anthropic content-block shape:
            #   {"role": "user", "content": [{"type": "text", "text": "hi"}]}
            user_msgs = [m for m in body["messages"] if m.get("role") == "user"]
            assert user_msgs
            content = user_msgs[0]["content"]
            if isinstance(content, str):
                assert content == "hi"
            else:
                texts = [b.get("text") for b in content if b.get("type") == "text"]
                assert "hi" in texts
        finally:
            _stop_service(proc)
    finally:
        server.shutdown()


def test_chat_unconfigured_post_new_returns_412(tmp_path: Path) -> None:
    """POST /chat/new without env vars returns 412 (precondition failed)."""
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    env["STRIATUM_LEGACY_SERVICE_FIXTURE"] = "1"
    for key in (
        "STRIATUM_CHAT_API_BASE_URL", "STRIATUM_CHAT_API_KEY",
        "STRIATUM_CHAT_MODEL", "STRIATUM_CHAT_API_FLAVOR",
    ):
        env.pop(key, None)
    with _socket.socket(_socket.AF_INET, _socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        port = cast(int, sock.getsockname()[1])
    proc = subprocess.Popen(
        [
            sys.executable, "-m", "striatum.cli",
            "--repo", str(tmp_path), "serve",
            "--host", "127.0.0.1", "--port", str(port),
            "--web", "--allow-mutations", "--idle-timeout-seconds", "30",
        ],
        cwd=tmp_path, env=env,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
    )
    deadline = time.time() + 10.0
    while time.time() < deadline:
        try:
            with _socket.create_connection(("127.0.0.1", port), timeout=0.5):
                break
        except (ConnectionRefusedError, OSError):
            time.sleep(0.1)
    try:
        status, _, _ = _http_post_form(port, "/chat/new", {})
        assert status == 412
    finally:
        _stop_service(proc)


def test_chat_csp_unchanged(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    server, prov_port = _start_fake_server(FakeOpenAIChatHandler)
    try:
        proc, port = _spawn_service_with_chat(
            tmp_path,
            base_url=f"http://127.0.0.1:{prov_port}",
            flavor="openai_chat",
        )
        try:
            _, headers, _ = _http_get_raw(port, "/chat")
            csp = headers.get("Content-Security-Policy", "")
            assert "default-src 'self'" in csp
            assert "unsafe-inline" not in csp
            assert "unsafe-eval" not in csp
        finally:
            _stop_service(proc)
    finally:
        server.shutdown()


def test_chat_provider_url_scheme_validation_rejects_remote_http(tmp_path: Path) -> None:
    """RFC 0023 V1 design-review F1 (security): non-loopback http:// is refused."""
    from striatum.web.chat_provider import ChatProviderError, validate_base_url
    with pytest.raises(ChatProviderError, match="loopback"):
        validate_base_url("http://example.com/v1")
    # https remote OK.
    validate_base_url("https://api.anthropic.com")
    # http localhost OK.
    validate_base_url("http://localhost:11434/v1")
    validate_base_url("http://127.0.0.1:8080")


def test_chat_send_exposes_dogfood_lifecycle_tools_when_mutations_enabled(
    tmp_path: Path,
) -> None:
    """RFC 0040 V1: when serve runs with --allow-mutations, the request body
    forwarded to the LLM lists the new dogfood-lifecycle tools alongside the
    existing closed-set. The capability-token-gated daemon path is daemon-
    side; this checks the local chat surface wiring.
    """
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    FakeOpenAIChatHandler.received_body = None
    server, prov_port = _start_fake_server(FakeOpenAIChatHandler)
    try:
        proc, port = _spawn_service_with_chat(
            tmp_path,
            base_url=f"http://127.0.0.1:{prov_port}",
            flavor="openai_chat",
        )
        try:
            status, headers, _ = _http_post_form(port, "/chat/new", {})
            session_id = headers["Location"][len("/chat/"):]
            _http_post_form(
                port, f"/chat/{session_id}/send", {"message": "hello"},
            )
            assert FakeOpenAIChatHandler.received_body is not None
            tools = FakeOpenAIChatHandler.received_body.get("tools") or []
            names = {t.get("function", {}).get("name") for t in tools}
            # RFC 0040 V1 dogfood-lifecycle tools must be exposed.
            assert {
                "ack",
                "publish_artifact",
                "verdict",
                "complete",
                "run_prepare",
                "register_session",
                "supervise_start",
                "supervise_stop",
                "claim_next",
                "run_start",
                "run_summary",
                "evidence_export",
            }.issubset(names)
        finally:
            _stop_service(proc)
    finally:
        server.shutdown()


def test_chat_provider_unknown_flavor_refused(tmp_path: Path) -> None:
    from striatum.web.chat_provider import ChatProviderConfig, ChatProviderError
    env = {
        "STRIATUM_CHAT_API_BASE_URL": "https://api.anthropic.com",
        "STRIATUM_CHAT_API_KEY": "k",
        "STRIATUM_CHAT_MODEL": "m",
        "STRIATUM_CHAT_API_FLAVOR": "telepathy",
    }
    with pytest.raises(ChatProviderError, match="unknown STRIATUM_CHAT_API_FLAVOR"):
        ChatProviderConfig.from_env(env)

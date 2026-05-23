from __future__ import annotations

import json
import importlib
from email.message import Message
from io import BytesIO
from pathlib import Path
from typing import Any

import pytest

import striatum.service as service
import striatum.service_daemon as service_daemon
from striatum.web.template_env import jinja_env


class _FakeTemplate:
    def __init__(self, captured: dict[str, Any]) -> None:
        self._captured = captured

    def render(self, **kwargs: Any) -> str:
        self._captured.update(kwargs)
        return "ok"


class _FakeEnvironment:
    def __init__(self, expected_template: str, captured: dict[str, Any]) -> None:
        self._expected_template = expected_template
        self._captured = captured

    def get_template(self, name: str) -> _FakeTemplate:
        assert name == self._expected_template
        return _FakeTemplate(self._captured)


def _headers_for_json(raw: bytes) -> Message:
    headers = Message()
    headers["Content-Type"] = "application/json"
    headers["Content-Length"] = str(len(raw))
    return headers


def _handler(
    tmp_path: Path,
    *,
    payload: dict[str, Any] | None = None,
    allow_mutations: bool = False,
) -> tuple[service.StriatumServiceHandler, dict[str, Any]]:
    raw = json.dumps(payload or {}).encode("utf-8")
    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=allow_mutations,
        token=None,
        web_enabled=True,
    )
    handler.headers = _headers_for_json(raw)
    handler.rfile = BytesIO(raw)
    sent: dict[str, Any] = {}
    setattr(handler, "_send_json", lambda status, body: sent.update({"status": status, "body": body}))
    setattr(handler, "_send_html", lambda status, body: sent.update({"status": status, "body": body}))
    return handler, sent


def _sqlite_module() -> Any:
    return importlib.import_module("sqlite3")


def test_escalation_list_reads_daemon_dto_without_sqlite(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def sqlite_tripwire(*_args: Any, **_kwargs: Any) -> None:
        raise AssertionError("escalation list page opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "count": 1,
            "state": "open",
            "escalations": [
                {
                    "escalation_id": "blk_1",
                    "run_id": "run_1",
                    "source": "blocker",
                    "class": "missing_authority",
                    "state": "open",
                    "created_at": "2026-05-22T00:00:00Z",
                }
            ],
        }

    handler, sent = _handler(tmp_path)
    monkeypatch.setattr(service, "_jinja_env", lambda: _FakeEnvironment("escalation_list.html", captured))
    monkeypatch.setattr(_sqlite_module(), "connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_escalation_list_page({})

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [(tmp_path, "escalation.list", {})]
    assert captured["run_id"] == ""
    assert captured["escalations"][0]["escalation_id"] == "blk_1"


def test_escalation_list_passes_run_id_filter(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {"count": 0, "state": "open", "escalations": []}

    handler, sent = _handler(tmp_path)
    monkeypatch.setattr(service, "_jinja_env", lambda: _FakeEnvironment("escalation_list.html", captured))
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_escalation_list_page({"run_id": ["run_1"]})

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [(tmp_path, "escalation.list", {"run_id": "run_1"})]
    assert captured["run_id"] == "run_1"


def test_escalation_detail_reads_daemon_dto_without_sqlite(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def sqlite_tripwire(*_args: Any, **_kwargs: Any) -> None:
        raise AssertionError("escalation detail page opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "escalation": {
                "escalation_id": "blk_1",
                "run_id": "run_1",
                "source": "blocker",
                "class": "missing_authority",
                "state": "open",
                "description": "needs approval",
            }
        }

    handler, sent = _handler(tmp_path)
    monkeypatch.setattr(service, "_jinja_env", lambda: _FakeEnvironment("escalation_detail.html", captured))
    monkeypatch.setattr(_sqlite_module(), "connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_escalation_detail_page("blk_1")

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [(tmp_path, "escalation.show", {"escalation_id": "blk_1"})]
    assert captured["escalation"]["description"] == "needs approval"
    assert captured["can_resolve"] is True


def test_escalation_detail_preserves_daemon_not_found_error(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        raise service_daemon.ServiceDaemonRpcError(
            404,
            "not_found",
            "escalation not found: blk_missing",
        )

    handler, sent = _handler(tmp_path)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_escalation_detail_page("blk_missing")

    assert sent == {
        "status": 404,
        "body": {
            "ok": False,
            "error": {
                "code": "not_found",
                "message": "escalation not found: blk_missing",
            },
        },
    }
    assert calls == [
        (tmp_path, "escalation.show", {"escalation_id": "blk_missing"})
    ]


@pytest.mark.parametrize("bad_id", ["", "..", "../blk", "blk/1", "blk%2F1", "blk\x001"])
def test_escalation_detail_rejects_malformed_ids(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    bad_id: str,
) -> None:
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {}

    handler, sent = _handler(tmp_path)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._render_escalation_detail_page(bad_id)

    assert sent["status"] == 400
    assert calls == []


def test_escalation_resolve_posts_daemon_rpc_without_sqlite(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def sqlite_tripwire(*_args: Any, **_kwargs: Any) -> None:
        raise AssertionError("escalation resolve handler opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {"status": "resolved", "escalation": {"escalation_id": "blk_1"}}

    handler, sent = _handler(
        tmp_path,
        payload={"decision_id": "dec_1", "resolution_note": "principal approved"},
        allow_mutations=True,
    )
    monkeypatch.setattr(_sqlite_module(), "connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._handle_escalation_resolve("blk_1")

    assert sent["status"] == 200
    assert sent["body"]["data"]["status"] == "resolved"
    assert calls == [
        (
            tmp_path,
            "escalation.resolve",
            {
                "escalation_id": "blk_1",
                "decision_id": "dec_1",
                "resolution_note": "principal approved",
            },
        )
    ]


def test_escalation_resolve_without_mutation_gate_makes_no_daemon_call(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {}

    handler, sent = _handler(tmp_path, payload={"resolution_note": "done"})
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._handle_escalation_resolve("blk_1")

    assert sent["status"] == 405
    assert calls == []


@pytest.mark.parametrize("bad_id", ["", "..", "../blk", "blk/1", "blk%2F1", "blk\x001"])
def test_escalation_resolve_rejects_malformed_ids_before_daemon_call(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    bad_id: str,
) -> None:
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {}

    handler, sent = _handler(tmp_path, payload={}, allow_mutations=True)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._handle_escalation_resolve(bad_id)

    assert sent["status"] == 400
    assert calls == []


def test_escalation_resolve_preserves_daemon_error_status_and_kind(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def fake_call_repo_method(
        _repo: Path,
        _method: str,
        _params: dict[str, Any],
    ) -> dict[str, Any]:
        raise service_daemon.ServiceDaemonRpcError(
            409,
            "invalid_transition",
            "escalation is not open",
            kind="invalid_transition",
        )

    handler, sent = _handler(tmp_path, payload={"resolution_note": "done"}, allow_mutations=True)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._handle_escalation_resolve("blk_1")

    assert sent == {
        "status": 409,
        "body": {
            "ok": False,
            "error": {
                "code": 409,
                "message": "escalation is not open",
                "kind": "invalid_transition",
            },
        },
    }


@pytest.mark.parametrize(
    ("headers", "message"),
    [
        (
            {"Host": "127.0.0.1:1234", "Origin": "http://evil.example"},
            "cross-origin request refused",
        ),
        ({"Host": "127.0.0.1:1234"}, "Origin or Referer required"),
        (
            {"Host": "127.0.0.1:1234", "Origin": "http://127.0.0.1:bad"},
            "cross-origin request refused",
        ),
    ],
)
def test_escalation_resolve_dispatch_uses_same_origin_gate_before_daemon_call(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    headers: dict[str, str],
    message: str,
) -> None:
    calls: list[tuple[Path, str, dict[str, Any]]] = []
    raw = b'{"resolution_note":"done"}'
    request_headers = _headers_for_json(raw)
    for key, value in headers.items():
        request_headers[key] = value

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {}

    handler, sent = _handler(tmp_path, allow_mutations=True)
    handler.path = "/escalations/blk_1/resolve"
    handler.headers = request_headers
    handler.rfile = BytesIO(raw)
    handler.state.origin_check_enabled = True
    handler.state.allowed_origins = {("http", "127.0.0.1", 1234)}
    handler._authenticate = lambda: True  # type: ignore[method-assign]
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    handler._dispatch_post()

    assert sent["status"] == 403
    assert sent["body"]["error"]["message"] == message
    assert calls == []


def test_escalation_dispatch_get_routes_list_and_detail(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured: dict[str, Any] = {}
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        if method == "escalation.list":
            return {"count": 0, "state": "open", "escalations": []}
        return {
            "escalation": {
                "escalation_id": "blk_1",
                "run_id": "run_1",
                "source": "blocker",
                "class": "missing_authority",
                "state": "open",
                "description": "needs approval",
            }
        }

    handler, sent = _handler(tmp_path)
    handler._authenticate = lambda: True  # type: ignore[method-assign]
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    monkeypatch.setattr(service, "_jinja_env", lambda: _FakeEnvironment("escalation_list.html", captured))
    handler.path = "/escalations?run_id=run_1"
    handler._dispatch_get()
    assert sent == {"status": 200, "body": "ok"}

    sent.clear()
    captured.clear()
    monkeypatch.setattr(service, "_jinja_env", lambda: _FakeEnvironment("escalation_detail.html", captured))
    handler.path = "/escalations/blk_1"
    handler._dispatch_get()

    assert sent == {"status": 200, "body": "ok"}
    assert calls == [
        (tmp_path, "escalation.list", {"run_id": "run_1"}),
        (tmp_path, "escalation.show", {"escalation_id": "blk_1"}),
    ]


def test_escalation_detail_template_hides_resolve_for_human_checkpoint() -> None:
    html = jinja_env().get_template("escalation_detail.html").render(
        escalation={
            "escalation_id": "blk_human",
            "run_id": "run_1",
            "job_id": "job_1",
            "workflow_job_id": "review",
            "session_id": None,
            "source": "human_checkpoint",
            "class": "revision_routing",
            "state": "open",
            "severity": "human_checkpoint",
            "description": "needs principal choice",
            "payload": {},
        },
        can_resolve=False,
    )

    assert "data-escalation-resolve-form" not in html
    assert "Checkpoint resolution required." in html

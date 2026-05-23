from __future__ import annotations

from pathlib import Path
from typing import Any

import pytest

from striatum.service_daemon import ServiceDaemonRpcError
from striatum.web import doctor as web_doctor


def test_doctor_page_response_shapes_problem_groups_and_recipes(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "ok": False,
            "problem_records": [
                {
                    "check": "human_checkpoint_open",
                    "severity": "info",
                    "blocker_id": "blk_1",
                },
                {"severity": "warning"},
            ],
        }

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)

    response = web_doctor.doctor_page_response(
        tmp_path,
    )

    assert calls == [(tmp_path, "doctor", {"verbose": True})]
    assert response.doctor["problem_records"][0]["recipes"] == [
        "striatum checkpoint resolve --blocker-id blk_1 --action continue",
        "striatum checkpoint resolve --blocker-id blk_1 --action cancel",
    ]
    assert response.problem_groups["human_checkpoint_open"] == [
        response.doctor["problem_records"][0]
    ]
    assert response.problem_groups["unknown"] == [response.doctor["problem_records"][1]]


def test_doctor_page_response_raises_http_shaped_error_when_fallback_refused(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        raise ServiceDaemonRpcError(503, "repo_not_registered", "register repo first")

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)

    with pytest.raises(web_doctor.DoctorPageError) as excinfo:
        web_doctor.doctor_page_response(
            tmp_path,
        )

    assert excinfo.value.status == 503
    assert excinfo.value.code == "repo_not_registered"
    assert excinfo.value.message == "register repo first"


def test_render_doctor_page_sends_html(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        return {"problem_records": [{"check": "human_checkpoint_open", "blocker_id": "blk_1"}]}

    class FakeTemplate:
        def render(self, **kwargs: Any) -> str:
            assert kwargs["problem_groups"]["human_checkpoint_open"][0]["recipes"] == [
                "striatum checkpoint resolve --blocker-id blk_1 --action continue",
                "striatum checkpoint resolve --blocker-id blk_1 --action cancel",
            ]
            return "doctor html"

    class FakeEnvironment:
        def get_template(self, name: str) -> FakeTemplate:
            assert name == "doctor.html"
            return FakeTemplate()

    sent: dict[str, Any] = {}
    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)

    web_doctor.render_doctor_page(
        web_doctor.DoctorRouteContext(
            repo=tmp_path,
            send_json=lambda status, body: sent.update({"json": (status, body)}),
            send_html=lambda status, body: sent.update({"html": (status, body)}),
            jinja_env=lambda: FakeEnvironment(),
        )
    )

    assert sent == {"html": (200, "doctor html")}


def test_render_doctor_page_sends_daemon_error(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        raise ServiceDaemonRpcError(503, "daemon_unreachable", "daemon is required")

    sent: dict[str, Any] = {}
    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)

    web_doctor.render_doctor_page(
        web_doctor.DoctorRouteContext(
            repo=tmp_path,
            send_json=lambda status, body: sent.update({"json": (status, body)}),
            send_html=lambda status, body: sent.update({"html": (status, body)}),
            jinja_env=lambda: object(),
        )
    )

    assert sent == {
        "json": (
            503,
            {
                "ok": False,
                "error": {"code": "daemon_unreachable", "message": "daemon is required"},
            },
        )
    }

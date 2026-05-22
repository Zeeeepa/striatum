from __future__ import annotations

from pathlib import Path
from typing import Any

import pytest

from striatum.service_daemon import ServiceDaemonRpcError
from striatum.web.workflow_generation import (
    workflow_generate_response,
    workflow_template_show_response,
    workflow_templates_response,
)


def _spec(shape: str = "review") -> dict[str, Any]:
    return {
        "schema_version": "striatum.workflow_generator.v1",
        "shape": shape,
        "lane_set": "local",
        "workflow_id": "demo",
        "name": "Demo",
        "workflow_version": "2026-05-12",
        "branch": {
            "mode": "confirm",
            "suggested_name": "striatum/demo",
            "allow_dirty": False,
        },
        "scaffold_root": "workflows/demo",
        "artifact_root": "striatum/demo",
        "lanes": {},
        "options": {},
    }


def test_workflow_templates_response_lists_shape_templates() -> None:
    response = workflow_templates_response("shape")

    assert response.status == 200
    assert any(
        item["template_id"] == "review"
        for item in response.payload["data"]["templates"]
    )


def test_workflow_templates_response_lists_role_packs() -> None:
    response = workflow_templates_response("role_pack")

    assert response.status == 200
    assert any(
        item["template_id"] == "implementation_panel_roles"
        for item in response.payload["data"]["templates"]
    )


def test_workflow_template_show_response_decodes_template_id() -> None:
    response = workflow_template_show_response("review")

    assert response.status == 200
    assert response.payload["data"]["template_id"] == "review"


def test_workflow_generate_preview_does_not_fallback_with_legacy_service_fixture(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    monkeypatch.setenv("STRIATUM_LEGACY_SERVICE_FIXTURE", "1")

    def fake_call_repo_method(_repo: Path, _method: str, _params: dict[str, Any]) -> dict[str, Any]:
        raise ServiceDaemonRpcError(
            503,
            "daemon_unreachable",
            "daemon_unreachable: workflow.generate.preview requires the Striatum daemon",
        )

    def local_generator_tripwire(_spec: object) -> None:
        raise AssertionError("legacy service fixture used local preview fallback")

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)
    monkeypatch.setattr("striatum.web.workflow_generation.generate_workflow", local_generator_tripwire)

    response = workflow_generate_response(
        repo=tmp_path,
        body={"spec": _spec()},
        preview=True,
        allow_mutations=False,
    )

    assert response.status == 503
    assert response.payload["ok"] is False
    assert response.payload["error"]["code"] == "daemon_unreachable"
    assert not (tmp_path / "workflows" / "demo").exists()


def test_workflow_generate_preview_routes_through_daemon_rpc(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[tuple[Path, str, dict[str, Any]]] = []
    rpc_only_spec = {"not": "a-valid-generator-spec"}

    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {
            "workflow": {"workflow_id": "rpc-demo"},
            "files": [],
            "metadata": {},
            "warnings": [],
            "validation": {},
            "lint": {"valid": True},
            "planned_writes": [],
        }

    def local_generator_tripwire(_spec: object) -> None:
        raise AssertionError("preview bypassed daemon RPC")

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)
    monkeypatch.setattr("striatum.web.workflow_generation.generate_workflow", local_generator_tripwire)

    response = workflow_generate_response(
        repo=tmp_path,
        body={"spec": rpc_only_spec},
        preview=True,
        allow_mutations=False,
    )

    assert response.status == 200
    assert response.payload["data"]["workflow"]["workflow_id"] == "rpc-demo"
    assert calls == [(tmp_path, "workflow.generate.preview", {"spec": rpc_only_spec})]
    assert not (tmp_path / "workflows" / "demo").exists()


def test_workflow_generate_preview_does_not_fallback_in_production(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)

    def fake_call_repo_method(_repo: Path, _method: str, _params: dict[str, Any]) -> dict[str, Any]:
        raise ServiceDaemonRpcError(
            503,
            "daemon_unreachable",
            "daemon_unreachable: workflow.generate.preview requires the Striatum daemon",
            details={"hint": "start the daemon"},
        )

    def local_generator_tripwire(_spec: object) -> None:
        raise AssertionError("production preview used local fallback")

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)
    monkeypatch.setattr("striatum.web.workflow_generation.generate_workflow", local_generator_tripwire)

    response = workflow_generate_response(
        repo=tmp_path,
        body={"spec": _spec()},
        preview=True,
        allow_mutations=False,
    )

    assert response.status == 503
    assert response.payload["ok"] is False
    assert response.payload["error"]["code"] == "daemon_unreachable"
    assert response.payload["error"]["hint"] == "start the daemon"
    assert not (tmp_path / "workflows" / "demo").exists()


def test_workflow_generate_preview_does_not_fallback_with_broad_harness_only(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    monkeypatch.delenv("STRIATUM_LEGACY_SERVICE_FIXTURE", raising=False)

    def fake_call_repo_method(_repo: Path, _method: str, _params: dict[str, Any]) -> dict[str, Any]:
        raise ServiceDaemonRpcError(
            503,
            "daemon_unreachable",
            "daemon_unreachable: workflow.generate.preview requires the Striatum daemon",
        )

    def local_generator_tripwire(_spec: object) -> None:
        raise AssertionError("broad test harness used local fallback")

    monkeypatch.setattr("striatum.service_daemon.call_repo_method", fake_call_repo_method)
    monkeypatch.setattr("striatum.web.workflow_generation.generate_workflow", local_generator_tripwire)

    response = workflow_generate_response(
        repo=tmp_path,
        body={"spec": _spec()},
        preview=True,
        allow_mutations=False,
    )

    assert response.status == 503
    assert response.payload["ok"] is False
    assert response.payload["error"]["code"] == "daemon_unreachable"
    assert not (tmp_path / "workflows" / "demo").exists()


def test_workflow_generate_write_requires_mutation_gate(tmp_path: Path) -> None:
    response = workflow_generate_response(
        repo=tmp_path,
        body={"spec": _spec(), "confirm_write": True},
        preview=False,
        allow_mutations=False,
    )

    assert response.status == 405
    assert response.payload["error"]["field_path"] == "server.allow_mutations"


def test_workflow_generate_write_requires_confirmation(tmp_path: Path) -> None:
    response = workflow_generate_response(
        repo=tmp_path,
        body={"spec": _spec("minimal")},
        preview=False,
        allow_mutations=True,
    )

    assert response.status == 400
    assert response.payload["error"]["field_path"] == "confirm_write"


def test_workflow_generate_write_creates_files(tmp_path: Path) -> None:
    response = workflow_generate_response(
        repo=tmp_path,
        body={"spec": _spec("minimal"), "confirm_write": True},
        preview=False,
        allow_mutations=True,
    )

    assert response.status == 200
    assert response.payload["data"]["status"] == "created"
    assert (tmp_path / "workflows" / "demo" / "workflow.json").exists()

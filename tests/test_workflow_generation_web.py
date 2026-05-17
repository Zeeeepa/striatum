from __future__ import annotations

from pathlib import Path
from typing import Any

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


def test_workflow_template_show_response_decodes_template_id() -> None:
    response = workflow_template_show_response("review")

    assert response.status == 200
    assert response.payload["data"]["template_id"] == "review"


def test_workflow_generate_preview_writes_nothing(tmp_path: Path) -> None:
    response = workflow_generate_response(
        repo=tmp_path,
        body={"spec": _spec()},
        preview=True,
        allow_mutations=False,
    )

    assert response.status == 200
    assert response.payload["data"]["workflow"]["workflow_id"] == "demo"
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

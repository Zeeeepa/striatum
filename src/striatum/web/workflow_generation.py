"""Workflow generation response helpers for the local service."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping, cast
from urllib.parse import unquote

from striatum.workflow_generator import (
    GeneratorError,
    WorkflowGenerationSpec,
    generate_workflow,
)
from striatum.workflow_generator.catalog import get_template, list_templates
from striatum.workflow_generator.write import write_generated_workflow

JsonObject = dict[str, Any]

__all__ = [
    "WorkflowGenerationResponse",
    "generator_error_response",
    "workflow_generate_response",
    "workflow_template_show_response",
    "workflow_templates_response",
]


@dataclass(frozen=True)
class WorkflowGenerationResponse:
    status: int
    payload: JsonObject


def generator_error_response(exc: Exception, *, status: int = 400) -> WorkflowGenerationResponse:
    error: JsonObject = {"code": getattr(exc, "exit_code", 8), "message": str(exc)}
    field_path = getattr(exc, "field_path", None)
    if isinstance(field_path, str):
        error["field_path"] = field_path
    hint = getattr(exc, "hint", None)
    if isinstance(hint, str):
        error["hint"] = hint
    ref = getattr(exc, "ref", None)
    if isinstance(ref, str):
        error["ref"] = ref
    return WorkflowGenerationResponse(status, {"ok": False, "error": error})


def workflow_templates_response(kind: str | None) -> WorkflowGenerationResponse:
    try:
        data = {"templates": list_templates(kind=kind)}
    except GeneratorError as exc:
        return generator_error_response(exc)
    return WorkflowGenerationResponse(200, {"ok": True, "data": data})


def workflow_template_show_response(raw_template_id: str) -> WorkflowGenerationResponse:
    try:
        data = get_template(unquote(raw_template_id))
    except GeneratorError as exc:
        return generator_error_response(exc, status=404)
    return WorkflowGenerationResponse(200, {"ok": True, "data": data})


def workflow_generate_response(
    *,
    repo: Path,
    body: Mapping[str, Any],
    preview: bool,
    allow_mutations: bool,
) -> WorkflowGenerationResponse:
    spec_body = body.get("spec")
    if not isinstance(spec_body, dict):
        return WorkflowGenerationResponse(
            400,
            {
                "ok": False,
                "error": {
                    "code": 8,
                    "message": "missing spec object",
                    "field_path": "spec",
                },
            },
        )
    if not preview:
        if not allow_mutations:
            return WorkflowGenerationResponse(
                405,
                {
                    "ok": False,
                    "error": {
                        "code": 405,
                        "message": "workflow generation requires --allow-mutations",
                        "field_path": "server.allow_mutations",
                    },
                },
            )
        if body.get("confirm_write") is not True:
            return WorkflowGenerationResponse(
                400,
                {
                    "ok": False,
                    "error": {
                        "code": 8,
                        "message": "confirm_write must be true",
                        "field_path": "confirm_write",
                    },
                },
            )
    try:
        spec = WorkflowGenerationSpec.from_json(cast(JsonObject, spec_body))
        generated = generate_workflow(spec)
        if preview:
            return WorkflowGenerationResponse(200, {"ok": True, "data": generated.to_json()})
        return WorkflowGenerationResponse(
            200,
            {"ok": True, "data": write_generated_workflow(generated, repo=repo)},
        )
    except GeneratorError as exc:
        return generator_error_response(exc)

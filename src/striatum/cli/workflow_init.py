"""Workflow scaffolding helpers (``striatum workflow init``)."""

from __future__ import annotations

from pathlib import Path

from striatum.errors import WorkflowError
from striatum.primitives import JsonObject
from striatum.workflow_generator import generate_workflow
from striatum.workflow_generator.core import default_spec
from striatum.workflow_generator.write import write_generated_workflow


def workflow_init(target: Path, *, style: str) -> JsonObject:
    """Write a starter workflow tree at ``target`` for the requested style."""
    if style not in {"minimal", "review", "code-change"}:
        raise WorkflowError(f"unknown workflow style: {style!r}")
    if target.exists():
        raise WorkflowError(
            f"workflow init refuses to overwrite existing path: {target}"
        )
    safe_slug = target.name if target.name else "starter-workflow"
    shape = "code_change" if style == "code-change" else style
    spec = default_spec(
        scaffold_root=safe_slug,
        artifact_root=f"docs/workflows/{safe_slug}",
        shape=shape,
        lane_set="local",
        workflow_id=f"{safe_slug}-starter",
        name=f"{safe_slug} starter ({style})",
        branch_suggestion=f"striatum/{safe_slug}",
    )
    generated = generate_workflow(spec)
    write_generated_workflow(generated, repo=target.parent)
    return {
        "status": "created",
        "path": str(target),
        "workflow_path": str(target / "workflow.json"),
        "style": style,
        "files": _legacy_file_list(style),
    }


def _legacy_file_list(style: str) -> list[str]:
    if style == "minimal":
        return ["roles/author.md", "prompts/draft.md", "workflow.json"]
    return [
        "roles/author.md",
        "roles/reviewer.md",
        "prompts/draft.md",
        "prompts/review.md",
        "prompts/apply.md",
        "workflow.json",
    ]

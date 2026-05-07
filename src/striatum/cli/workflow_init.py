"""Workflow scaffolding helpers (``striatum workflow init``)."""

from __future__ import annotations

import json
from pathlib import Path

from striatum.db import JsonObject, utc_now
from striatum.errors import WorkflowError
from striatum.workflow import load_workflow


def workflow_init(target: Path, *, style: str) -> JsonObject:
    """Write a starter workflow tree at ``target`` for the requested style.

    The generated tree always validates with ``striatum workflow validate``
    and uses repo-relative paths inside ``expected_artifacts``. Refuses to
    overwrite an existing path so the caller never accidentally clobbers an
    in-progress workflow.
    """
    if style not in {"minimal", "review", "code-change"}:
        raise WorkflowError(f"unknown workflow style: {style!r}")
    if target.exists():
        raise WorkflowError(
            f"workflow init refuses to overwrite existing path: {target}"
        )
    workflow = _starter_workflow(target.name, style=style)
    roles_dir = target / "roles"
    prompts_dir = target / "prompts"
    target.mkdir(parents=True)
    roles_dir.mkdir()
    prompts_dir.mkdir()
    role_stubs: dict[str, str] = {
        "author": (
            "# Author Role\n\n"
            "You are the author for this workflow. Produce the expected handoff "
            "artifact at the path declared in the workflow. Stay inside the "
            "declared write scope.\n"
        ),
        "reviewer": (
            "# Reviewer Role\n\n"
            "You are the reviewer for this workflow. Read the upstream draft "
            "and write a single review-only finding artifact at the declared "
            "path; do not modify other files.\n"
        ),
    }
    prompt_stubs: dict[str, str] = {
        "draft": (
            "Draft the initial artifact described by the workflow. Replace this "
            "stub with the concrete authoring instructions for your team.\n"
        ),
        "review": (
            "Review the upstream draft and record a finding with one of the "
            "supported verdicts. Replace this stub with reviewer guidance.\n"
        ),
        "apply": (
            "Apply the accepted review by producing the final synthesis "
            "artifact. Replace this stub with concrete apply instructions.\n"
        ),
    }
    written: list[str] = []
    if style == "minimal":
        (roles_dir / "author.md").write_text(role_stubs["author"], encoding="utf-8")
        (prompts_dir / "draft.md").write_text(prompt_stubs["draft"], encoding="utf-8")
        written.extend(["roles/author.md", "prompts/draft.md"])
    else:
        (roles_dir / "author.md").write_text(role_stubs["author"], encoding="utf-8")
        (roles_dir / "reviewer.md").write_text(role_stubs["reviewer"], encoding="utf-8")
        (prompts_dir / "draft.md").write_text(prompt_stubs["draft"], encoding="utf-8")
        (prompts_dir / "review.md").write_text(prompt_stubs["review"], encoding="utf-8")
        (prompts_dir / "apply.md").write_text(prompt_stubs["apply"], encoding="utf-8")
        written.extend(
            [
                "roles/author.md",
                "roles/reviewer.md",
                "prompts/draft.md",
                "prompts/review.md",
                "prompts/apply.md",
            ]
        )
    workflow_path = target / "workflow.json"
    workflow_path.write_text(
        json.dumps(workflow, indent=2, sort_keys=False) + "\n", encoding="utf-8"
    )
    written.append("workflow.json")
    # Validate the freshly written tree so a misconfigured generator is caught
    # at init time rather than the next time someone tries to prepare a run.
    load_workflow(workflow_path)
    return {
        "status": "created",
        "path": str(target),
        "workflow_path": str(workflow_path),
        "style": style,
        "files": written,
    }


def _starter_workflow(slug: str, *, style: str) -> JsonObject:
    """Return a starter workflow JSON object matching the requested style."""
    safe_slug = slug if slug != "" else "starter-workflow"
    workflow_id = f"{safe_slug}-starter"
    coordinator_role = "author"
    lane_id = "local"
    lanes: JsonObject = {
        lane_id: {
            "adapter": "process",
            "display_model": "Local Fixture",
            "command": ["sh", "-c", "cat >/dev/null"],
            "capabilities": ["write", "review"],
        }
    }
    base_dir = f"docs/workflows/{safe_slug}"
    jobs: list[JsonObject]
    edges: list[JsonObject]
    cycles: list[JsonObject]
    roles: JsonObject
    if style == "minimal":
        roles = {"author": {"definition_path": "roles/author.md"}}
        jobs = [
            {
                "id": "draft",
                "type": "draft",
                "title": "Draft starter artifact",
                "role_id": "author",
                "lane_id": lane_id,
                "objective": "Produce the starter artifact for this workflow.",
                "task_prompt": {"path": "prompts/draft.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": [f"{base_dir}/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "draft",
                        "kind": "handoff",
                        "path": f"{base_dir}/DRAFT.md",
                        "required": True,
                    }
                ],
            }
        ]
        edges = []
        cycles = []
    else:
        roles = {
            "author": {"definition_path": "roles/author.md"},
            "reviewer": {"definition_path": "roles/reviewer.md"},
        }
        jobs = [
            {
                "id": "draft",
                "type": "draft",
                "title": "Draft starter artifact",
                "role_id": "author",
                "lane_id": lane_id,
                "objective": "Produce the starter artifact for this workflow.",
                "task_prompt": {"path": "prompts/draft.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": [f"{base_dir}/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "draft",
                        "kind": "handoff",
                        "path": f"{base_dir}/DRAFT.md",
                        "required": True,
                    }
                ],
            },
            {
                "id": "review",
                "type": "review",
                "title": "Review the draft",
                "role_id": "reviewer",
                "lane_id": lane_id,
                "fresh_session_required": True,
                "objective": "Review the draft and record a finding.",
                "task_prompt": {"path": "prompts/review.md"},
                "write_scope": {
                    "mode": "review_only_artifact",
                    "repo_write": False,
                    "allowed_paths": [f"{base_dir}/review/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "review",
                        "kind": "finding",
                        "path": f"{base_dir}/review/REVIEW.md",
                        "required": True,
                    }
                ],
            },
            {
                "id": "apply",
                "type": "synthesis",
                "title": "Apply the accepted review",
                "role_id": "author",
                "lane_id": lane_id,
                "objective": "Apply the accepted review findings.",
                "task_prompt": {"path": "prompts/apply.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": [f"{base_dir}/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "summary",
                        "kind": "synthesis",
                        "path": f"{base_dir}/SUMMARY.md",
                        "required": True,
                    }
                ],
            },
        ]
        edges = [
            {"from": "draft", "to": "review", "on": "completed"},
            {"from": "review", "to": "apply", "on": "completed"},
        ]
        if style == "code-change":
            cycles = [
                {
                    "from": "review",
                    "to": "draft",
                    "on_verdict": "needs_revision",
                    "max_iterations": 1,
                }
            ]
        else:
            cycles = []
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": workflow_id,
        "workflow_version": utc_now().split("T", 1)[0],
        "name": f"{safe_slug} starter ({style})",
        "branch": {
            "mode": "confirm",
            "suggested_name": f"striatum/{safe_slug}",
            "allow_dirty": False,
        },
        "coordinator": {
            "role_id": coordinator_role,
            "lane_id": lane_id,
        },
        "lanes": lanes,
        "roles": roles,
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": jobs,
        "edges": edges,
        "cycles": cycles,
    }

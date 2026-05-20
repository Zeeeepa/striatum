"""Tests for RFC 0002 (Reviewer Independence Policy) workflow fields."""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
from pathlib import Path
from typing import cast

import pytest

from striatum.errors import WorkflowError
from striatum.legacy_sqlite.db import connect
from striatum.workflow import validate_workflow

from test_cli_mvp import (
    DOCS_REVIEW_WORKFLOW,
    JsonDict,
    WORKFLOW,
    claim,
    complete_claimed_job,
    data,
    init_repo,
    prepare_started_run,
    register,
    run_cli,
    temporary_workflow,
)


def _base_review_workflow() -> JsonDict:
    """Return a minimal valid workflow with a single fresh review job."""
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf-reviewer-policy",
        "workflow_version": "1",
        "name": "Reviewer policy",
        "branch": {"mode": "confirm", "suggested_name": "wf/reviewer", "allow_dirty": False},
        "coordinator": {"role_id": "reviewer", "lane_id": "lane_a"},
        "lanes": {
            "lane_a": {
                "adapter": "process",
                "display_model": "X",
                "command": ["echo"],
                "capabilities": ["review"],
            },
        },
        "roles": {"reviewer": {"definition_path": "roles/reviewer.md"}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 1,
            "require_disjoint_write_scopes": True,
        },
        "jobs": [
            {
                "id": "review_only",
                "type": "review",
                "title": "Review",
                "role_id": "reviewer",
                "lane_id": "lane_a",
                "fresh_session_required": True,
                "objective": "Review",
                "task_prompt": {"path": "prompts/review.md"},
                "write_scope": {
                    "mode": "review_only_artifact",
                    "repo_write": False,
                    "allowed_paths": ["docs/reviews/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "review",
                        "kind": "finding",
                        "path": "docs/reviews/REVIEW.md",
                        "required": True,
                    }
                ],
            },
        ],
        "edges": [],
        "cycles": [],
    }


def test_workflow_validates_reviewer_access_scope_values() -> None:
    """All three documented values pass; an unknown value is rejected."""
    for value in ("document_only", "artifact_augmented", "repo_level"):
        workflow = _base_review_workflow()
        workflow["jobs"][0]["reviewer_access_scope"] = value
        validate_workflow(workflow)

    workflow = _base_review_workflow()
    workflow["jobs"][0]["reviewer_access_scope"] = "filesystem"
    with pytest.raises(
        WorkflowError,
        match=r"review job 'review_only' has unknown reviewer_access_scope 'filesystem'; "
        r"allowed: document_only\|artifact_augmented\|repo_level",
    ):
        validate_workflow(workflow)


def test_workflow_validates_reviewer_context_policy_values() -> None:
    """Both documented values pass; an unknown value is rejected."""
    for value in ("fresh", "cross_round"):
        workflow = _base_review_workflow()
        workflow["jobs"][0]["reviewer_context_policy"] = value
        validate_workflow(workflow)

    workflow = _base_review_workflow()
    workflow["jobs"][0]["reviewer_context_policy"] = "stale"
    with pytest.raises(
        WorkflowError,
        match=r"review job 'review_only' has unknown reviewer_context_policy 'stale'; "
        r"allowed: fresh\|cross_round",
    ):
        validate_workflow(workflow)


def test_workflow_rejects_reviewer_policy_on_non_review_job() -> None:
    """Reviewer-policy fields cannot be declared on a non-review job, even with valid values."""
    workflow = _base_review_workflow()
    workflow["roles"]["author"] = {"definition_path": "roles/author.md"}
    workflow["jobs"] = [
        {
            "id": "draft",
            "type": "draft",
            "title": "Draft",
            "role_id": "author",
            "lane_id": "lane_a",
            "objective": "Draft",
            "task_prompt": {"path": "prompts/draft.md"},
            "reviewer_access_scope": "document_only",
            "reviewer_context_policy": "fresh",
            "write_scope": {
                "mode": "repo_write",
                "repo_write": True,
                "allowed_paths": ["docs/output/"],
                "forbidden_paths": [".striatum/"],
            },
            "expected_artifacts": [
                {
                    "logical_name": "draft",
                    "kind": "handoff",
                    "path": "docs/output/DRAFT.md",
                    "required": True,
                }
            ],
        },
    ]
    with pytest.raises(
        WorkflowError,
        match=r"non-review job 'draft' cannot declare reviewer_access_scope/reviewer_context_policy",
    ):
        validate_workflow(workflow)


def test_workflow_fresh_context_implies_fresh_session_required(tmp_path: Path) -> None:
    """A review job with reviewer_context_policy=fresh and no fresh_session_required
    should be stored with fresh_session_required=1.
    """
    workflow = _base_review_workflow()
    review_job = workflow["jobs"][0]
    # Strip the explicit fresh_session_required so the implication is exercised.
    review_job.pop("fresh_session_required", None)
    review_job["reviewer_context_policy"] = "fresh"
    review_job["reviewer_access_scope"] = "document_only"

    workflow_path = temporary_workflow(tmp_path, workflow)
    init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])

    with connect(tmp_path) as conn:
        row = conn.execute(
            "SELECT fresh_session_required FROM jobs "
            "WHERE run_id = ? AND workflow_job_id = ?",
            (run_id, "review_only"),
        ).fetchone()
    assert row is not None
    assert row[0] == 1


def test_workflow_rejects_fresh_policy_with_explicit_non_fresh_session() -> None:
    """fresh_session_required=False alongside reviewer_context_policy=fresh is rejected."""
    workflow = _base_review_workflow()
    review_job = workflow["jobs"][0]
    review_job["fresh_session_required"] = False
    review_job["reviewer_context_policy"] = "fresh"
    with pytest.raises(
        WorkflowError,
        match=r"review job 'review_only' declares reviewer_context_policy=fresh but "
        r"fresh_session_required=false",
    ):
        validate_workflow(workflow)


def _patched_docs_review_workflow(
    tmp_path: Path,
    *,
    access_scope: str,
    context_policy: str,
) -> Path:
    """Copy the docs-review-flow fixture into tmp_path, with reviewer policy fields set on review_docs."""
    workflow = cast(JsonDict, json.loads(DOCS_REVIEW_WORKFLOW.read_text(encoding="utf-8")))
    for job in workflow["jobs"]:
        if job.get("id") == "review_docs":
            job["reviewer_access_scope"] = access_scope
            job["reviewer_context_policy"] = context_policy
            break
    return temporary_workflow(tmp_path, workflow)


def test_work_packet_exposes_review_policy_when_declared(tmp_path: Path) -> None:
    """Driving a review claim under a workflow that declares both reviewer policy
    fields surfaces an explicit review_policy block on the work packet.
    """
    workflow_path = _patched_docs_review_workflow(
        tmp_path,
        access_scope="document_only",
        context_policy="fresh",
    )
    run_id = prepare_started_run(tmp_path, workflow_path)
    author = register(tmp_path, run_id, "author", "local")
    draft_packet = claim(tmp_path, author)
    complete_claimed_job(
        tmp_path,
        author,
        draft_packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/docs-review-flow/DOCS_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "local")
    review_packet = claim(tmp_path, reviewer)

    assert "review_policy" in review_packet
    review_policy = review_packet["review_policy"]
    assert isinstance(review_policy, dict)
    assert review_policy["access_scope"] == "document_only"
    assert review_policy["context_policy"] == "fresh"
    instruction = review_policy["instruction"]
    assert isinstance(instruction, str)
    assert (
        "Read only the target documents listed in inputs. Do not consult other "
        "artifacts, ledgers, reports, or repository contents beyond inputs."
    ) in instruction
    assert (
        "This is a fresh-context review. Do not rely on prior thread state from "
        "earlier rounds."
    ) in instruction


def test_work_packet_omits_review_policy_when_not_declared(tmp_path: Path) -> None:
    """Workflows that do not declare reviewer-policy fields still produce
    work packets without a review_policy block (regression guard for
    existing fixtures).
    """
    run_id = prepare_started_run(tmp_path, WORKFLOW)
    author = register(tmp_path, run_id, "author", "codex")
    draft_packet = claim(tmp_path, author)
    complete_claimed_job(
        tmp_path,
        author,
        draft_packet,
        logical_name="draft",
        kind="handoff",
        path="docs/reviews/rfc-ledger/RFC_LEDGER_DRAFT.md",
    )
    reviewer = register(tmp_path, run_id, "reviewer", "codex")
    review_packet = claim(tmp_path, reviewer)
    assert "review_policy" not in review_packet

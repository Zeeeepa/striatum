"""Shared identity helpers for jobs and artifacts."""

from __future__ import annotations

from typing import Mapping, TypedDict


class ArtifactAuthorIdentity(TypedDict):
    """Stable author metadata suitable for evidence and artifact headers."""

    role_id: str
    lane_id: str | None
    display_model: str | None
    workflow_job_id: str
    line: str


def artifact_author_identity(
    workflow: Mapping[str, object],
    *,
    role_id: str,
    lane_id: str | None,
    workflow_job_id: str,
) -> ArtifactAuthorIdentity:
    """Return stable author identity without using free-text job titles."""
    display_model = lane_display_model(workflow, lane_id)
    lane_label = lane_id or "unassigned_lane"
    model_label = display_model or "unknown_model"
    line = f"Author: {role_id} / {lane_label} / {model_label} / {workflow_job_id}"
    return {
        "role_id": role_id,
        "lane_id": lane_id,
        "display_model": display_model,
        "workflow_job_id": workflow_job_id,
        "line": line,
    }


def lane_display_model(workflow: Mapping[str, object], lane_id: str | None) -> str | None:
    """Return the workflow display model for a lane when declared."""
    if lane_id is None:
        return None
    lanes = workflow.get("lanes")
    if not isinstance(lanes, dict):
        return None
    lane_config = lanes.get(lane_id)
    if not isinstance(lane_config, dict):
        return None
    display_model = lane_config.get("display_model")
    return display_model if isinstance(display_model, str) else None

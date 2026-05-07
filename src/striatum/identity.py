"""Shared identity helpers for jobs and artifacts."""

from __future__ import annotations

import re
from typing import Mapping, TypedDict


class ArtifactAuthorIdentity(TypedDict):
    """Author metadata for evidence export plus optional artifact byline."""

    role_id: str
    lane_id: str | None
    display_model: str | None
    workflow_job_id: str
    ordinal: int | None
    line: str | None


def artifact_author_identity(
    workflow: Mapping[str, object],
    *,
    role_id: str,
    lane_id: str | None,
    workflow_job_id: str,
    ordinal: int | None = None,
) -> ArtifactAuthorIdentity:
    """Return stable export identity and a non-leaky artifact byline."""
    display_model = lane_display_model(workflow, lane_id)
    line = (
        f"author: {author_part(role_id)}-"
        f"{author_part(display_model or 'unknown-model')}-"
        f"{ordinal:03d}"
        if ordinal is not None
        else None
    )
    return {
        "role_id": role_id,
        "lane_id": lane_id,
        "display_model": display_model,
        "workflow_job_id": workflow_job_id,
        "ordinal": ordinal,
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


def author_part(value: str) -> str:
    """Normalize a role or model label for a low-leak artifact byline."""
    normalized = re.sub(r"[^a-z0-9.]+", "-", value.lower()).strip("-")
    return normalized or "unknown"

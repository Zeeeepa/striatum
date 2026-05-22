"""Stable cause classes for auto-finalize skip/refusal reporting."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

RUN_NOT_RUNNING = "run_not_running"
NO_REQUIRED_EXPECTED_ARTIFACTS = "no_required_expected_artifacts"
NO_ACTIVE_MESSAGE = "no_active_message"
EXPECTED_BYLINE_UNRESOLVABLE = "expected_byline_unresolvable"

ARTIFACT_PATH_MISSING = "artifact_path_missing"
ARTIFACT_KIND_INVALID = "artifact_kind_invalid"
ARTIFACT_KIND_TRANSCRIPT = "artifact_kind_transcript"
ARTIFACT_LOGICAL_NAME_MISSING = "artifact_logical_name_missing"
ARTIFACT_PATH_OUTSIDE_WRITE_SCOPE = "artifact_path_outside_write_scope"
ARTIFACT_PAYLOAD_PATH_INVALID = "artifact_payload_path_invalid"
ARTIFACT_FILE_MISSING = "artifact_file_missing"
ARTIFACT_MTIME_INSIDE_GRACE = "artifact_mtime_inside_grace"
ARTIFACT_READ_FAILED = "artifact_read_failed"
ARTIFACT_FRONT_MATTER_INVALID = "artifact_front_matter_invalid"
ARTIFACT_BYLINE_MISSING = "artifact_byline_missing"
ARTIFACT_BYLINE_MISMATCH = "artifact_byline_mismatch"
LANE_EVIDENCE_MISSING = "lane_evidence_missing"
ARTIFACT_CONFLICT_EXISTING_CONTENT = "artifact_conflict_existing_content"

FINALIZE_PUBLISH_FAILED = "finalize_publish_failed"
FINALIZE_COMPLETE_FAILED = "finalize_complete_failed"
FINALIZE_VERDICT_FAILED = "finalize_verdict_failed"
FINALIZE_UNEXPECTED_ERROR = "finalize_unexpected_error"
CIRCUIT_BREAKER_OPEN = "circuit_breaker_open"
PROJECTION_EVALUATION_FAILED = "projection_evaluation_failed"

CAUSES: frozenset[str] = frozenset(
    {
        RUN_NOT_RUNNING,
        NO_REQUIRED_EXPECTED_ARTIFACTS,
        NO_ACTIVE_MESSAGE,
        EXPECTED_BYLINE_UNRESOLVABLE,
        ARTIFACT_PATH_MISSING,
        ARTIFACT_KIND_INVALID,
        ARTIFACT_KIND_TRANSCRIPT,
        ARTIFACT_LOGICAL_NAME_MISSING,
        ARTIFACT_PATH_OUTSIDE_WRITE_SCOPE,
        ARTIFACT_PAYLOAD_PATH_INVALID,
        ARTIFACT_FILE_MISSING,
        ARTIFACT_MTIME_INSIDE_GRACE,
        ARTIFACT_READ_FAILED,
        ARTIFACT_FRONT_MATTER_INVALID,
        ARTIFACT_BYLINE_MISSING,
        ARTIFACT_BYLINE_MISMATCH,
        LANE_EVIDENCE_MISSING,
        ARTIFACT_CONFLICT_EXISTING_CONTENT,
        FINALIZE_PUBLISH_FAILED,
        FINALIZE_COMPLETE_FAILED,
        FINALIZE_VERDICT_FAILED,
        FINALIZE_UNEXPECTED_ERROR,
        CIRCUIT_BREAKER_OPEN,
        PROJECTION_EVALUATION_FAILED,
    }
)


def cause_fields(cause: str, detail: object | None = None) -> dict[str, Any]:
    if cause not in CAUSES:
        raise ValueError(f"unknown auto-finalize cause: {cause}")
    fields: dict[str, Any] = {"cause": cause}
    if detail is not None:
        text = str(detail).strip()
        if text:
            fields["cause_detail"] = text[:240]
    return fields


def artifact_validation_cause(reason: str) -> str:
    text = reason.lower()
    if "author line is required" in text:
        return ARTIFACT_BYLINE_MISSING
    if "author line must match" in text:
        return ARTIFACT_BYLINE_MISMATCH
    return ARTIFACT_FRONT_MATTER_INVALID


def first_refusal_cause(refusals: list[Mapping[str, Any]]) -> str:
    for refusal in refusals:
        cause = refusal.get("cause")
        if isinstance(cause, str) and cause in CAUSES:
            return cause
    return ARTIFACT_FRONT_MATTER_INVALID


def finalize_failure_cause(exc: BaseException) -> str:
    text = str(exc).lower()
    if "publish" in text or "artifact" in text:
        return FINALIZE_PUBLISH_FAILED
    if "verdict" in text or "review" in text:
        return FINALIZE_VERDICT_FAILED
    if "complete" in text or "job" in text:
        return FINALIZE_COMPLETE_FAILED
    return FINALIZE_UNEXPECTED_ERROR

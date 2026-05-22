from __future__ import annotations

import pytest

from striatum.recovery import auto_finalize_causes as causes


def test_auto_finalize_cause_set_is_closed() -> None:
    assert causes.CAUSES == {
        causes.RUN_NOT_RUNNING,
        causes.NO_REQUIRED_EXPECTED_ARTIFACTS,
        causes.NO_ACTIVE_MESSAGE,
        causes.EXPECTED_BYLINE_UNRESOLVABLE,
        causes.ARTIFACT_PATH_MISSING,
        causes.ARTIFACT_KIND_INVALID,
        causes.ARTIFACT_KIND_TRANSCRIPT,
        causes.ARTIFACT_LOGICAL_NAME_MISSING,
        causes.ARTIFACT_PATH_OUTSIDE_WRITE_SCOPE,
        causes.ARTIFACT_PAYLOAD_PATH_INVALID,
        causes.ARTIFACT_FILE_MISSING,
        causes.ARTIFACT_MTIME_INSIDE_GRACE,
        causes.ARTIFACT_READ_FAILED,
        causes.ARTIFACT_FRONT_MATTER_INVALID,
        causes.ARTIFACT_BYLINE_MISSING,
        causes.ARTIFACT_BYLINE_MISMATCH,
        causes.LANE_EVIDENCE_MISSING,
        causes.ARTIFACT_CONFLICT_EXISTING_CONTENT,
        causes.FINALIZE_PUBLISH_FAILED,
        causes.FINALIZE_COMPLETE_FAILED,
        causes.FINALIZE_VERDICT_FAILED,
        causes.FINALIZE_UNEXPECTED_ERROR,
        causes.PROJECTION_EVALUATION_FAILED,
    }


def test_auto_finalize_cause_helper_rejects_unknown_cause() -> None:
    with pytest.raises(ValueError, match="unknown auto-finalize cause"):
        causes.cause_fields("not_a_cause")


def test_artifact_validation_cause_classifies_byline_failures() -> None:
    assert (
        causes.artifact_validation_cause("markdown artifact author line is required for auto-finalize")
        == causes.ARTIFACT_BYLINE_MISSING
    )
    assert (
        causes.artifact_validation_cause(
            "markdown artifact author line must match expected work packet author line"
        )
        == causes.ARTIFACT_BYLINE_MISMATCH
    )
    assert (
        causes.artifact_validation_cause("finding artifact front matter missing required field")
        == causes.ARTIFACT_FRONT_MATTER_INVALID
    )


def test_finalize_failure_cause_classifies_common_failure_sites() -> None:
    assert causes.finalize_failure_cause(RuntimeError("artifact publish failed")) == causes.FINALIZE_PUBLISH_FAILED
    assert causes.finalize_failure_cause(RuntimeError("record verdict failed")) == causes.FINALIZE_VERDICT_FAILED
    assert causes.finalize_failure_cause(RuntimeError("complete job failed")) == causes.FINALIZE_COMPLETE_FAILED
    assert causes.finalize_failure_cause(RuntimeError("unexpected")) == causes.FINALIZE_UNEXPECTED_ERROR

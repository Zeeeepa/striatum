from __future__ import annotations

import pytest

from striatum.cli.evidence import EVIDENCE_FREE_TEXT_PLACEHOLDER
from striatum.corpus.redaction import (
    redact_commit_message,
    redact_event_payload,
    redact_run_summary_payload,
    validate_source_path,
)
from striatum.errors import StriatumError


@pytest.mark.parametrize(
    "path",
    [
        ".env",
        ".env.local",
        "keys/private.pem",
        ".striatum/state.sqlite3",
        "transcripts/session.md",
        "raw_model_output/out.log",
        "docs/transcript.txt",
    ],
)
def test_source_path_denylist(path: str) -> None:
    with pytest.raises(StriatumError):
        validate_source_path(path)


def test_curated_markdown_path_passes_validation() -> None:
    validate_source_path("docs/rfcs/0044-engram-phase-1-implementation-spec.md")


def test_audit_payload_keeps_metadata_only() -> None:
    assert redact_event_payload(
        {
            "event_id": "evt_1",
            "event_type": "job.completed",
            "description": "secret",
            "payload_json": {"body": "secret"},
            "row_hash": "hash",
        }
    ) == {"event_id": "evt_1", "event_type": "job.completed", "row_hash": "hash"}


def test_commit_message_strips_coauthor_email_and_token_lines() -> None:
    token = "a" * 64
    redacted = redact_commit_message(
        f"Subject\n\nCo-Authored-By: Person <p@example.com>\n{token}\nnormal line"
    )
    assert "example.com" not in redacted
    assert token not in redacted
    assert "normal line" in redacted


def test_run_summary_redaction_preserves_renderer_shape_and_redacts_unknowns() -> None:
    payload = {
        "status": {"runs": [{"run_id": "run_1", "workflow_id": "wf", "state": "running", "secret": "x"}]},
        "doctor": {"ok": True, "problems": [], "secret": "x"},
        "blockers": [{"blocker_id": "blk", "description": "secret", "state": "open"}],
        "branch_context": {},
        "timing": {},
        "artifacts": [],
        "sessions": [],
        "verdicts": [],
        "verdicts_by_workflow_job": [],
        "new_future_field": "secret",
    }
    redacted = redact_run_summary_payload(payload)

    assert redacted["doctor"]["ok"] is True
    assert "description" not in redacted["blockers"][0]
    assert redacted["new_future_field"] == EVIDENCE_FREE_TEXT_PLACEHOLDER

from __future__ import annotations

import pytest

from striatum.service_http import verify_web_context_token
from striatum.web.job_detail import JobDetailPayloadError, job_detail_template_context


def _payload() -> dict[str, object]:
    return {
        "run": {"run_id": "run_1", "state": "running"},
        "job": {"job_id": "job_1", "workflow_job_id": "review", "state": "completed"},
        "artifacts": [{"artifact_id": "art_1"}, "noise"],
        "latest_verdict": {"verdict_id": "verdict_1", "session_id": "sess_1"},
        "verdicts": [{"verdict_id": "verdict_1"}],
        "expected_artifact_rows": [{"path": "docs/review.md"}],
        "process_evidence": [{"process_id": "proc_1"}],
    }


def test_job_detail_template_context_shapes_daemon_payload() -> None:
    context = job_detail_template_context(_payload(), web_context_secret=b"secret")

    assert context["run"] == {"run_id": "run_1", "state": "running"}
    assert context["job"] == {
        "job_id": "job_1",
        "workflow_job_id": "review",
        "state": "completed",
    }
    assert context["artifacts"] == [{"artifact_id": "art_1"}]
    assert context["latest_verdict"] == {"verdict_id": "verdict_1", "session_id": "sess_1"}
    assert context["verdicts"] == [{"verdict_id": "verdict_1"}]
    assert context["expected_artifact_rows"] == [{"path": "docs/review.md"}]
    assert context["process_evidence"] == [{"process_id": "proc_1"}]


def test_job_detail_template_context_mints_override_context_token() -> None:
    context = job_detail_template_context(_payload(), web_context_secret=b"secret")

    assert context["override_session_id"] == "sess_1"
    assert verify_web_context_token(
        b"secret",
        token=str(context["override_context_token"]),
        run_id="run_1",
        job_id="job_1",
        session_id="sess_1",
    )


def test_job_detail_template_context_omits_token_without_latest_verdict_session() -> None:
    payload = _payload()
    payload["latest_verdict"] = {"verdict_id": "verdict_1"}

    context = job_detail_template_context(payload, web_context_secret=b"secret")

    assert context["override_session_id"] == ""
    assert context["override_context_token"] == ""


def test_job_detail_template_context_rejects_missing_run_or_job() -> None:
    with pytest.raises(JobDetailPayloadError, match="daemon job detail DTO missing fields"):
        job_detail_template_context({"run": {"run_id": "run_1"}}, web_context_secret=b"secret")

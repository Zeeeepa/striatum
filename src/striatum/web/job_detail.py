"""Template-context shaping for the job detail page."""

from __future__ import annotations

from typing import Any, Mapping

from striatum.service_http import make_web_context_token


class JobDetailPayloadError(ValueError):
    """Raised when the daemon job-detail DTO is missing required fields."""


def job_detail_template_context(
    payload: Mapping[str, Any],
    *,
    web_context_secret: bytes,
) -> dict[str, Any]:
    """Return the Jinja context for ``job_detail.html`` from a daemon DTO."""

    run_raw = payload.get("run")
    job_raw = payload.get("job")
    if not isinstance(run_raw, Mapping) or not isinstance(job_raw, Mapping):
        raise JobDetailPayloadError("daemon job detail DTO missing fields")

    run = dict(run_raw)
    job = dict(job_raw)
    latest_raw = payload.get("latest_verdict")
    latest_verdict = dict(latest_raw) if isinstance(latest_raw, Mapping) else None

    override_session_id = ""
    if latest_verdict is not None:
        override_session_id = str(latest_verdict.get("session_id", "")) or ""

    override_context_token = ""
    if override_session_id:
        override_context_token = make_web_context_token(
            web_context_secret,
            run_id=str(run["run_id"]),
            job_id=str(job["job_id"]),
            session_id=override_session_id,
        )

    return {
        "run": run,
        "job": job,
        "artifacts": _mapping_list(payload, "artifacts"),
        "latest_verdict": latest_verdict,
        "verdicts": _mapping_list(payload, "verdicts"),
        "expected_artifact_rows": _mapping_list(payload, "expected_artifact_rows"),
        "process_evidence": _mapping_list(payload, "process_evidence"),
        "override_context_token": override_context_token,
        "override_session_id": override_session_id,
    }


def _mapping_list(payload: Mapping[str, Any], key: str) -> list[dict[Any, Any]]:
    return [
        dict(row)
        for row in payload.get(key) or []
        if isinstance(row, Mapping)
    ]


__all__ = [
    "JobDetailPayloadError",
    "job_detail_template_context",
]

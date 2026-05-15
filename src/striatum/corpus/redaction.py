"""Fail-closed source and field redaction for corpus exports."""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

from copy import deepcopy

from striatum.cli.evidence import EVIDENCE_FREE_TEXT_PLACEHOLDER, redact_evidence_payload
from striatum.errors import StriatumError

TOKEN_RE = re.compile(r"\b(?:[A-Fa-f0-9]{40,}|[A-Za-z0-9+/]{48,}={0,2})\b")

DENIED_NAMES = {
    ".env",
}

DENIED_SUFFIXES = (".pem", ".key", ".p12", ".sqlite3", ".sqlite", ".db", ".swp", ".swo")
DENIED_PARTS = {
    ".striatum",
    "transcripts",
    "terminal_output",
    "raw_model_output",
}

SAFE_EVENT_FIELDS = {
    "event_id",
    "event_type",
    "run_id",
    "job_id",
    "workflow_job_id",
    "session_id",
    "artifact_id",
    "verdict_id",
    "blocker_kind",
    "state",
    "created_at",
    "request_hash",
    "response_hash",
    "row_hash",
    "previous_hash",
    "repository_id",
}


def validate_source_path(path: str) -> None:
    """Refuse source paths that are not allowed in the corpus."""
    p = Path(path)
    parts = set(p.parts)
    name = p.name
    lower = path.lower()
    if name in DENIED_NAMES or name.startswith(".env."):
        raise StriatumError(f"corpus source path is denied: {path}", exit_code=8)
    if lower.endswith(DENIED_SUFFIXES):
        raise StriatumError(f"corpus source path is denied: {path}", exit_code=8)
    if parts & DENIED_PARTS:
        raise StriatumError(f"corpus source path is denied: {path}", exit_code=8)
    if name.startswith("transcript") and lower.endswith((".txt", ".md", ".log")):
        raise StriatumError(f"corpus source path is denied: {path}", exit_code=8)


def redact_run_summary_payload(payload: dict[str, Any]) -> dict[str, Any]:
    """Redact the nested run-summary snapshot while preserving renderer shape."""
    redacted = deepcopy(payload)
    status = payload.get("status")
    if isinstance(status, dict):
        redacted["status"] = redact_evidence_payload(status)
    doctor = payload.get("doctor")
    if isinstance(doctor, dict):
        redacted["doctor"] = redact_evidence_payload(doctor)
    blockers = payload.get("blockers")
    if isinstance(blockers, list):
        clean_blockers: list[dict[str, Any]] = []
        for blocker in blockers:
            if not isinstance(blocker, dict):
                continue
            clean_blockers.append(
                {
                    "blocker_id": blocker.get("blocker_id"),
                    "job_id": blocker.get("job_id"),
                    "severity": blocker.get("severity"),
                    "blocker_kind": blocker.get("blocker_kind"),
                    "state": blocker.get("state"),
                }
            )
        redacted["blockers"] = clean_blockers
    verdicts = payload.get("verdicts")
    if isinstance(verdicts, list):
        redacted["verdicts"] = [_safe_verdict(entry) for entry in verdicts if isinstance(entry, dict)]
    artifacts = payload.get("artifacts")
    if isinstance(artifacts, list):
        redacted["artifacts"] = [_safe_artifact(entry) for entry in artifacts if isinstance(entry, dict)]
    sessions = payload.get("sessions")
    if isinstance(sessions, list):
        redacted["sessions"] = [_safe_session(entry) for entry in sessions if isinstance(entry, dict)]
    # The renderer only needs grouped verdicts, artifact/session summaries,
    # branch context, timing, status, doctor, and blockers. If a future helper
    # adds prose-heavy siblings, keep the shape but replace with a placeholder.
    for key in list(redacted.keys()):
        if key not in {
            "status",
            "doctor",
            "artifacts",
            "sessions",
            "verdicts",
            "verdicts_by_workflow_job",
            "blockers",
            "branch_context",
            "timing",
        }:
            redacted[key] = EVIDENCE_FREE_TEXT_PLACEHOLDER
    return redacted


def _safe_verdict(entry: dict[str, Any]) -> dict[str, Any]:
    return {
        "verdict_id": entry.get("verdict_id"),
        "job_id": entry.get("job_id"),
        "workflow_job_id": entry.get("workflow_job_id"),
        "verdict": entry.get("verdict"),
        "findings_artifact_id": entry.get("findings_artifact_id"),
        "created_at": entry.get("created_at"),
        "posture": entry.get("posture"),
    }


def _safe_artifact(entry: dict[str, Any]) -> dict[str, Any]:
    return {
        "artifact_id": entry.get("artifact_id"),
        "job_id": entry.get("job_id"),
        "logical_name": entry.get("logical_name"),
        "artifact_kind": entry.get("artifact_kind"),
        "repo_path": entry.get("repo_path"),
        "content_sha256": entry.get("content_sha256"),
        "size_bytes": entry.get("size_bytes"),
        "created_at": entry.get("created_at"),
    }


def _safe_session(entry: dict[str, Any]) -> dict[str, Any]:
    # Run summaries render close/non-fresh reasons verbatim. Those are useful
    # in evidence exports, but corpus exports must not carry operator prose.
    result = {
        "session_id": entry.get("session_id"),
        "role_id": entry.get("role_id"),
        "lane_id": entry.get("lane_id"),
        "slug": entry.get("slug"),
        "ordinal": entry.get("ordinal"),
        "state": entry.get("state"),
        "registered_at": entry.get("registered_at"),
        "closed_at": entry.get("closed_at"),
    }
    for key in ("close_reason", "non_fresh_reason"):
        value = entry.get(key)
        result[key] = EVIDENCE_FREE_TEXT_PLACEHOLDER if value not in (None, "") else value
    return result


def redact_event_payload(payload: dict[str, Any]) -> dict[str, Any]:
    """Keep only structural metadata from event/audit-like rows."""
    return {key: value for key, value in payload.items() if key in SAFE_EVENT_FIELDS}


def redact_commit_message(text: str) -> str:
    """Strip email co-author lines and standalone long token-like strings."""
    lines: list[str] = []
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.lower().startswith("co-authored-by:") and "<" in stripped and ">" in stripped:
            continue
        if TOKEN_RE.fullmatch(stripped):
            continue
        lines.append(TOKEN_RE.sub("<redacted-token>", line))
    return "\n".join(lines).strip()

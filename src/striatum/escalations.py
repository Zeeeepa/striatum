"""Shared escalation vocabulary."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

ESCALATION_BLOCKER_KINDS: frozenset[str] = frozenset(
    {
        "ambiguous_goal",
        "missing_authority",
        "contradicting_decisions",
        "no_available_reviewer_lane",
        "committee_stalemate",
        "override_required",
        "ai_self_declared",
    }
)

BLOCKER_PAYLOAD_SCHEMA_VERSION = "striatum.blocker_payload.v1"
BLOCKER_SEVERITIES: frozenset[str] = frozenset({"blocked", "human_checkpoint"})
MAX_BLOCKER_KIND_LENGTH = 64
MAX_BLOCKER_DESCRIPTION_LENGTH = 8000


def normalize_blocker_request(params: Mapping[str, Any]) -> dict[str, str]:
    """Return validated ``work.block`` request fields."""

    severity = _valid_severity(_required_str(params, "severity"))
    kind = _valid_blocker_kind(_required_str(params, "kind"))
    return {
        "session_id": _required_str(params, "session_id"),
        "job_id": _required_str(params, "job_id"),
        "lease_id": _required_str(params, "lease_id"),
        "kind": kind,
        "severity": severity,
        "description": _valid_description(_required_str(params, "description")),
    }


def is_escalation_blocker(*, severity: str, kind: str) -> bool:
    return severity == "human_checkpoint" or kind in ESCALATION_BLOCKER_KINDS


def blocker_payload(*, severity: str, kind: str, description: str) -> dict[str, object]:
    is_escalation = is_escalation_blocker(severity=severity, kind=kind)
    trigger: str | None = None
    if severity == "human_checkpoint":
        trigger = "human_checkpoint"
    elif kind in ESCALATION_BLOCKER_KINDS:
        trigger = "escalation_blocker_kind"
    return {
        "schema_version": BLOCKER_PAYLOAD_SCHEMA_VERSION,
        "source": "work.block",
        "severity": severity,
        "blocker_kind": kind,
        "description_format": "plain_text",
        "description_length": len(description),
        "is_escalation": is_escalation,
        "escalation_trigger": trigger,
    }


def _required_str(params: Mapping[str, Any], key: str) -> str:
    value = params.get(key)
    if not isinstance(value, str):
        raise ValueError(f"work.block requires string field {key}")
    cleaned = value.strip()
    if not cleaned:
        raise ValueError(f"work.block field {key} must be non-empty")
    return cleaned


def _valid_severity(value: str) -> str:
    if value not in BLOCKER_SEVERITIES:
        raise ValueError("work.block severity must be one of blocked, human_checkpoint")
    return value


def _valid_blocker_kind(value: str) -> str:
    if len(value) > MAX_BLOCKER_KIND_LENGTH:
        raise ValueError("work.block kind must be at most 64 characters")
    for char in value:
        if not (char.isascii() and (char.islower() or char.isdigit() or char in "._-")):
            raise ValueError("work.block kind must match ^[a-z0-9._-]{1,64}$")
    return value


def _valid_description(value: str) -> str:
    if len(value) > MAX_BLOCKER_DESCRIPTION_LENGTH:
        raise ValueError("work.block description must be at most 8000 characters")
    return value

__all__ = [
    "BLOCKER_PAYLOAD_SCHEMA_VERSION",
    "ESCALATION_BLOCKER_KINDS",
    "blocker_payload",
    "is_escalation_blocker",
    "normalize_blocker_request",
]

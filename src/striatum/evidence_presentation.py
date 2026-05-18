"""Substrate-neutral evidence redaction and Markdown rendering."""

from __future__ import annotations

import json

from striatum.primitives import JsonObject, json_dumps


EVIDENCE_FREE_TEXT_PLACEHOLDER = "<redacted-free-text>"

# Evidence redaction is a typed-field registry, not a blocklist. The export
# payload schema is fixed and known: every field that should appear verbatim
# in the committed Markdown export must be classified as "safe" below. Any
# field not listed in the registry is redacted by default. This default-deny
# rule is the contract: when someone adds a new field to evidence_snapshot(),
# status(), or doctor(), it is replaced with the placeholder until they
# explicitly extend the registry. See docs/SPEC.md (Artifacts/Evidence).
#
# Policy values:
#   "safe"     - emit value verbatim (ids, enums, counts, hashes, timestamps,
#                role/lane/state names, structured author identity).
#   "redacted" - replace value with EVIDENCE_FREE_TEXT_PLACEHOLDER.
#   "dropped"  - omit the field entirely from the redacted output.
#
# Special keys inside a section policy:
#   "_each"    - apply this policy to every dict element of a list.
#   "_items"   - apply this policy to every primitive element of a list
#                (use "safe" for lists of ids, enum names, or counts).
EVIDENCE_POLICY: JsonObject = {
    "runs": {
        "_each": {
            "run_id": "safe",
            "state": "safe",
            "branch_name": "safe",
        },
    },
    "jobs": {
        "_dict": "safe",
    },
    "open_blockers": {
        "_each": {
            "blocker_id": "safe",
            "run_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "severity": "safe",
            "blocker_kind": "safe",
            "description": "redacted",
            "state": "safe",
            "workflow_job_id": "safe",
            "job_state": "safe",
        },
    },
    "human_checkpoints": {
        "_each": {
            "blocker_id": "safe",
            "run_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "severity": "safe",
            "blocker_kind": "safe",
            "description": "redacted",
            "state": "safe",
            "workflow_job_id": "safe",
            "job_state": "safe",
        },
    },
    "latest_non_accepting_review_verdicts": {
        "_each": {
            "verdict_id": "safe",
            "run_id": "safe",
            "job_id": "safe",
            "workflow_job_id": "safe",
            "job_state": "safe",
            "session_id": "safe",
            "verdict": "safe",
            "findings_artifact_id": "safe",
            "rationale": "redacted",
        },
    },
    "claimable_jobs": {
        "_each": {
            "role_id": "safe",
            "lane_id": "safe",
            "count": "safe",
            "workflow_job_ids": {"_items": "safe"},
        },
    },
    "blocked_downstream_jobs": {
        "_each": {
            "job_id": "safe",
            "workflow_job_id": "safe",
            "state": "safe",
            "role_id": "safe",
            "lane": "safe",
            "blocked_by": {
                "_each": {
                    "depends_on_job_id": "safe",
                    "workflow_job_id": "safe",
                    "state": "safe",
                    "required_verdicts": {"_items": "safe"},
                    "latest_verdict": "safe",
                },
            },
        },
    },
    "next_actions": {"_items": "safe"},
    "ok": "safe",
    "schema_version": "safe",
    "problems": {"_items": "safe"},
    "exported_at": "safe",
    "workflow": {
        "workflow_id": "safe",
        "workflow_version": "safe",
    },
    "run": {
        "run_id": "safe",
        "branch_name": "safe",
        "state": "safe",
    },
    "snapshot_jobs": {
        "_each": {
            "job_id": "safe",
            "workflow_job_id": "safe",
            "job_type": "safe",
            "role_id": "safe",
            "lane": "safe",
            "display_model": "safe",
            "author": {
                "role_id": "safe",
                "lane_id": "safe",
                "display_model": "safe",
                "workflow_job_id": "safe",
                "ordinal": "safe",
                "line": "safe",
            },
            "state": "safe",
            "attempt": "safe",
            "max_attempts": "safe",
            "fresh_session_required": "safe",
            "title": "redacted",
            "dependencies": {
                "_each": {
                    "depends_on_job_id": "safe",
                    "workflow_job_id": "safe",
                    "state": "safe",
                    "required_verdicts": {"_items": "safe"},
                    "latest_verdict": "safe",
                },
            },
        },
    },
    "artifacts": {
        "_each": {
            "artifact_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "logical_name": "safe",
            "artifact_kind": "safe",
            "repo_path": "safe",
            "content_sha256": "safe",
            "author": {
                "role_id": "safe",
                "lane_id": "safe",
                "display_model": "safe",
                "workflow_job_id": "safe",
                "ordinal": "safe",
                "line": "safe",
                "actual_author_line": "safe",
            },
        },
    },
    "sessions": {
        "_each": {
            "session_id": "safe",
            "role_id": "safe",
            "lane_id": "safe",
            "slug": "safe",
            "ordinal": "safe",
            "state": "safe",
            "registered_at": "safe",
            "closed_at": "safe",
            "close_reason": "safe",
            "non_fresh_reason": "safe",
        },
    },
    "verdicts": {
        "_each": {
            "verdict_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "verdict": "safe",
            "findings_artifact_id": "safe",
            "rationale": "redacted",
            "posture": "safe",
        },
    },
    "blockers": {
        "_each": {
            "blocker_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "severity": "safe",
            "blocker_kind": "safe",
            "description": "redacted",
            "state": "safe",
        },
    },
}


_EVIDENCE_DROP = object()


def redact_evidence_payload(payload: JsonObject) -> JsonObject:
    """Return a redacted copy of an evidence payload."""
    redacted: JsonObject = {}
    for key, value in payload.items():
        policy = _evidence_policy_for_top_level(str(key), value)
        result = _apply_evidence_policy(value, policy)
        if result is _EVIDENCE_DROP:
            continue
        redacted[str(key)] = result
    return redacted


def _evidence_policy_for_top_level(key: str, value: object) -> object:
    if key == "jobs":
        if isinstance(value, list):
            return EVIDENCE_POLICY["snapshot_jobs"]
        return EVIDENCE_POLICY["jobs"]
    return EVIDENCE_POLICY.get(key, "redacted")


def _apply_evidence_policy(value: object, policy: object) -> object:
    if policy == "safe":
        if isinstance(value, dict | list):
            return EVIDENCE_FREE_TEXT_PLACEHOLDER
        return value
    if policy == "redacted":
        if value is None:
            return None
        return EVIDENCE_FREE_TEXT_PLACEHOLDER
    if policy == "dropped":
        return _EVIDENCE_DROP
    if isinstance(policy, dict):
        if isinstance(value, list):
            if "_each" in policy:
                element_policy = policy["_each"]
                return [
                    _apply_evidence_policy(item, element_policy)
                    for item in value
                    if _apply_evidence_policy(item, element_policy) is not _EVIDENCE_DROP
                ]
            if "_items" in policy:
                item_policy = policy["_items"]
                return [
                    _apply_evidence_policy(item, item_policy)
                    for item in value
                    if _apply_evidence_policy(item, item_policy) is not _EVIDENCE_DROP
                ]
            return EVIDENCE_FREE_TEXT_PLACEHOLDER
        if isinstance(value, dict):
            if "_dict" in policy:
                value_policy = policy["_dict"]
                redacted_dict: JsonObject = {}
                for child_key, child_value in value.items():
                    result = _apply_evidence_policy(child_value, value_policy)
                    if result is _EVIDENCE_DROP:
                        continue
                    redacted_dict[str(child_key)] = result
                return redacted_dict
            redacted_dict = {}
            for child_key, child_value in value.items():
                child_policy = policy.get(str(child_key), "redacted")
                result = _apply_evidence_policy(child_value, child_policy)
                if result is _EVIDENCE_DROP:
                    continue
                redacted_dict[str(child_key)] = result
            return redacted_dict
        if value is None:
            return None
        return EVIDENCE_FREE_TEXT_PLACEHOLDER
    if value is None:
        return None
    return EVIDENCE_FREE_TEXT_PLACEHOLDER


def render_evidence_markdown(
    *,
    run: JsonObject,
    status_payload: JsonObject,
    doctor_payload: JsonObject,
    snapshot: JsonObject,
) -> str:
    """Render a redacted evidence snapshot as Markdown."""
    return "\n".join(
        [
            "# Striatum Evidence Export",
            "",
            f"Run ID: `{run['run_id']}`",
            f"Branch: `{run['branch_name']}`",
            f"Run state: `{run['state']}`",
            f"Exported at: `{snapshot['exported_at']}`",
            "",
            "Daemon-owned PostgreSQL is authoritative; `.striatum/` scratch is not part of this export.",
            "",
            "## Status Output",
            "",
            "```json",
            json_dumps(status_payload),
            "```",
            "",
            "## Doctor Output",
            "",
            "```json",
            json_dumps(doctor_payload),
            "```",
            "",
            "## Snapshot",
            "",
            "```json",
            json.dumps(snapshot, indent=2, sort_keys=True),
            "```",
            "",
        ]
    )

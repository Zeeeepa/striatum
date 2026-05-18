"""Evidence redaction policy and export helpers for the Striatum CLI."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from striatum.db import (
    insert_event,
    latest_verdict,
    row_by_id,
)
from striatum.identity import artifact_author_identity, session_lane_attestation
from striatum.primitives import JsonObject, json_dumps, json_loads, sha256_bytes, utc_now
from striatum.repo_policy import repo_relative_path

from striatum.cli.introspect import (
    blocked_downstream_jobs,
    jobs_for_run,
)


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
    # --- status() output -------------------------------------------------
    "runs": {
        "_each": {
            "run_id": "safe",
            "state": "safe",
            "branch_name": "safe",
        },
    },
    "jobs": {
        # status returns a dict[str, int] of state -> count; every key is a
        # job-state enum name and every value is a count, so the whole
        # mapping is safe. Any unexpected nested structure is redacted.
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
    # --- doctor() output -------------------------------------------------
    "ok": "safe",
    "schema_version": "safe",
    "problems": {"_items": "safe"},
    # --- evidence_snapshot() output --------------------------------------
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
    # snapshot.jobs is a list of job summary dicts (key reused from status
    # but reached via a different path; the walker disambiguates by context).
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
            # Workflow job titles are project-specific prose; per docs/SPEC.md
            # they are omitted by default. evidence_job_summaries() does not
            # include "title" today, but if a future change adds it the
            # default-deny rule keeps it out of the export.
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
    # RFC 0011: per-session terminal disposition. The role/lane/state/
    # timestamp fields are project-neutral identifiers; the explicit
    # close_reason and non_fresh_reason strings are operator-supplied
    # rationales that should be retained verbatim so evidence captures
    # the documented breach (HARNESS-003) and close source (RFC 0011).
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

# The "jobs" key appears at the top level in two payload shapes:
#   - status(): dict[state -> count]
#   - evidence_snapshot(): list of job summary dicts
# _evidence_policy_for_top_level() dispatches by value type to pick between
# EVIDENCE_POLICY["jobs"] and EVIDENCE_POLICY["snapshot_jobs"].


_EVIDENCE_DROP = object()


def redact_evidence_payload(payload: JsonObject) -> JsonObject:
    """Return a redacted copy of an evidence payload (status, doctor, or snapshot).

    The redaction is policy-driven: each top-level field is matched against
    EVIDENCE_POLICY and walked recursively. Fields not listed in the policy
    are replaced with EVIDENCE_FREE_TEXT_PLACEHOLDER (default-deny). This
    prevents future schema additions from silently leaking agent or user
    prose into the committed Markdown export.
    """
    redacted: JsonObject = {}
    for key, value in payload.items():
        policy = _evidence_policy_for_top_level(str(key), value)
        result = _apply_evidence_policy(value, policy)
        if result is _EVIDENCE_DROP:
            continue
        redacted[str(key)] = result
    return redacted


def _evidence_policy_for_top_level(key: str, value: object) -> object:
    """Pick the policy entry for a top-level payload key.

    Disambiguates the "jobs" key, which is a state-count dict in status()
    output but a list of job summary dicts in snapshot output. Other keys
    look up by name; missing keys fall through to default-deny redaction.
    """
    if key == "jobs":
        if isinstance(value, list):
            return EVIDENCE_POLICY["snapshot_jobs"]
        return EVIDENCE_POLICY["jobs"]
    return EVIDENCE_POLICY.get(key, "redacted")


def _apply_evidence_policy(value: object, policy: object) -> object:
    """Recursively apply a policy node to a value.

    Policy is one of:
      - "safe": value passes through verbatim.
      - "redacted": non-None values become the placeholder.
      - "dropped": signals omission (caller must check for _EVIDENCE_DROP).
      - dict with field-name keys: applies to dict values; "_each" applies
        to each dict element of a list; "_items" applies to each primitive
        element of a list; "_dict" applies to all values of a dict.
    """
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
            # List with no list-shape policy: redact by default.
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
        # Primitive value with a structured policy: nothing to walk; default
        # redact unless explicitly safe.
        if value is None:
            return None
        return EVIDENCE_FREE_TEXT_PLACEHOLDER
    # Unknown policy shape: redact for safety.
    if value is None:
        return None
    return EVIDENCE_FREE_TEXT_PLACEHOLDER


def evidence_export(conn: sqlite3.Connection, *, repo: Path, run_id: str, path_text: str) -> JsonObject:
    """Write a redacted Markdown snapshot of runner state."""
    # Look up status/doctor/evidence_snapshot via the package so test
    # monkeypatches against ``striatum.cli`` continue to work (see
    # tests/test_cli_mvp.py::test_evidence_redaction_drops_unknown_fields_by_default).
    from striatum import cli as _cli

    run = row_by_id(conn, "runs", "run_id", run_id)
    target = repo_relative_path(repo, path_text)
    target.parent.mkdir(parents=True, exist_ok=True)
    status_payload = redact_evidence_payload(_cli.status(conn, run_id=run_id))
    doctor_payload = redact_evidence_payload(_cli.doctor(conn, repo=repo, run_id=run_id))
    snapshot = redact_evidence_payload(_cli.evidence_snapshot(conn, run_id=run_id))
    body = render_evidence_markdown(
        run=dict(run),
        status_payload=status_payload,
        doctor_payload=doctor_payload,
        snapshot=snapshot,
    )
    target.write_text(body, encoding="utf-8")
    digest = sha256_bytes(body.encode("utf-8"))
    insert_event(
        conn,
        run_id=run_id,
        event_type="evidence.exported",
        payload={"path": path_text, "sha256": digest},
    )
    return {"status": "exported", "run_id": run_id, "path": path_text, "sha256": digest}


def evidence_snapshot(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Return redacted run state for evidence export."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    snapshot = row_by_id(conn, "workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"]))
    workflow = json_loads(str(snapshot["workflow_json"]))
    jobs = evidence_job_summaries(conn, run_id=run_id, workflow=workflow)
    artifacts = evidence_artifact_summaries(conn, run_id=run_id, workflow=workflow)
    sessions = evidence_session_summaries(conn, run_id=run_id)
    verdicts = conn.execute(
        """
        SELECT verdict_id, job_id, session_id, verdict, findings_artifact_id, rationale, posture
        FROM verdicts WHERE run_id = ? ORDER BY created_at
        """,
        (run_id,),
    ).fetchall()
    blockers = conn.execute(
        """
        SELECT blocker_id, job_id, session_id, severity, blocker_kind, description, state
        FROM blockers WHERE run_id = ? ORDER BY created_at
        """,
        (run_id,),
    ).fetchall()
    return {
        "schema_version": "striatum.evidence.v1",
        "exported_at": utc_now(),
        "workflow": {
            "workflow_id": snapshot["workflow_id"],
            "workflow_version": snapshot["workflow_version"],
        },
        "run": {
            "run_id": run["run_id"],
            "branch_name": run["branch_name"],
            "state": run["state"],
        },
        "jobs": jobs,
        "artifacts": artifacts,
        "sessions": sessions,
        "verdicts": [dict(row) for row in verdicts],
        "blockers": [dict(row) for row in blockers],
        "blocked_downstream_jobs": blocked_downstream_jobs(conn, run_id=run_id),
    }


def evidence_session_summaries(
    conn: sqlite3.Connection, *, run_id: str
) -> list[JsonObject]:
    """Return per-session terminal disposition for evidence export (RFC 0011).

    The new ``closed_at`` and ``close_reason`` columns are surfaced
    alongside ``state`` so snapshot consumers can audit how each
    session ended (auto-close on run-terminal vs explicit operator
    close vs lease-driven expire). ``non_fresh_reason`` is also
    included since HARNESS-003 stores the reviewer-independence
    breach reason there.
    """
    rows = conn.execute(
        """
        SELECT session_id, role_id, lane_id, slug, ordinal, state,
               registered_at, closed_at, close_reason, non_fresh_reason,
               operator_label
        FROM sessions
        WHERE run_id = ?
        ORDER BY registered_at, session_id
        """,
        (run_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        item = dict(row)
        attestation = session_lane_attestation(conn, session_id=str(row["session_id"]))
        item["lane_attestation"] = attestation.state
        item["lane_attestation_reason"] = attestation.reason
        item["supervisor_id"] = attestation.supervisor_id
        result.append(item)
    return result


def evidence_job_summaries(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    workflow: JsonObject,
) -> list[JsonObject]:
    """Return redacted job summaries for evidence export."""
    summaries: list[JsonObject] = []
    for job in jobs_for_run(conn, run_id=run_id):
        lane = json_loads(str(job["lane_selector_json"])).get("lane_id")
        lane_id = lane if isinstance(lane, str) else None
        author = artifact_author_identity(
            workflow,
            role_id=str(job["role_id"]),
            lane_id=lane_id,
            workflow_job_id=str(job["workflow_job_id"]),
        )
        summaries.append(
            {
                "job_id": job["job_id"],
                "workflow_job_id": job["workflow_job_id"],
                "job_type": job["job_type"],
                "role_id": job["role_id"],
                "lane": lane_id,
                "display_model": author["display_model"],
                "author": author,
                "state": job["state"],
                "attempt": job["attempt"],
                "max_attempts": job["max_attempts"],
                "fresh_session_required": bool(job["fresh_session_required"]),
                "dependencies": dependency_summary(conn, job_id=str(job["job_id"])),
            }
        )
    return summaries


def evidence_artifact_summaries(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    workflow: JsonObject,
) -> list[JsonObject]:
    """Return artifact summaries with stable author identity."""
    rows = conn.execute(
        """
        SELECT a.artifact_id, a.job_id, a.session_id, a.logical_name,
               a.artifact_kind, a.repo_path, a.content_sha256, a.author_line,
               j.workflow_job_id, j.role_id, j.lane_selector_json,
               s.role_id AS session_role_id, s.lane_id AS session_lane_id,
               s.ordinal AS session_ordinal, s.operator_label AS session_operator_label
        FROM artifacts a
        LEFT JOIN jobs j ON j.job_id = a.job_id
        LEFT JOIN sessions s ON s.session_id = a.session_id
        WHERE a.run_id = ?
        ORDER BY a.repo_path
        """,
        (run_id,),
    ).fetchall()
    artifacts: list[JsonObject] = []
    for row in rows:
        lane_id: str | None = None
        if row["lane_selector_json"] is not None:
            lane = json_loads(str(row["lane_selector_json"])).get("lane_id")
            lane_id = lane if isinstance(lane, str) else None
        artifact: JsonObject = {
            "artifact_id": row["artifact_id"],
            "job_id": row["job_id"],
            "session_id": row["session_id"],
            "logical_name": row["logical_name"],
            "artifact_kind": row["artifact_kind"],
            "repo_path": row["repo_path"],
            "content_sha256": row["content_sha256"],
        }
        if row["workflow_job_id"] is not None and row["role_id"] is not None:
            author_role = row["session_role_id"] or row["role_id"]
            author_lane = row["session_lane_id"] or lane_id
            author_ordinal = int(row["session_ordinal"]) if row["session_ordinal"] is not None else None
            author = artifact_author_identity(
                workflow,
                role_id=str(author_role),
                lane_id=str(author_lane) if author_lane is not None else None,
                workflow_job_id=str(row["workflow_job_id"]),
                ordinal=author_ordinal,
                attested=session_lane_attestation(
                    conn, session_id=str(row["session_id"])
                ).attested if row["session_id"] is not None else False,
                operator_label=row["session_operator_label"],
            )
            # HARNESS-003 byline integrity: prefer the file's actual
            # author line. ``None`` means the artifact file omitted the
            # line entirely; render it as "missing" so snapshot readers
            # can distinguish that from a present-but-different byline.
            actual = row["author_line"]
            if actual is None or actual == "":
                author["line"] = None
                author["actual_author_line"] = None
            else:
                author["line"] = str(actual)
                author["actual_author_line"] = str(actual)
            artifact["author"] = author
        artifacts.append(artifact)
    return artifacts


def dependency_summary(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return all upstream dependency states for export."""
    rows = conn.execute(
        """
        SELECT dep.depends_on_job_id, dep.gate_json, up.workflow_job_id, up.state
        FROM job_dependencies dep
        JOIN jobs up ON up.job_id = dep.depends_on_job_id
        WHERE dep.job_id = ?
        ORDER BY up.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        gate = json_loads(str(row["gate_json"]))
        result.append(
            {
                "depends_on_job_id": row["depends_on_job_id"],
                "workflow_job_id": row["workflow_job_id"],
                "state": row["state"],
                "required_verdicts": gate.get("requires_verdict"),
                "latest_verdict": latest_verdict(conn, job_id=str(row["depends_on_job_id"])),
            }
        )
    return result


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
            "Live SQLite state remains ignored under `.striatum/` and is not part of this export.",
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

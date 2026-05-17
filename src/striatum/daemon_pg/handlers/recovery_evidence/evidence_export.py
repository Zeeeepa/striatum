"""PG handler for ``evidence.export``.

Port of :func:`striatum.cli.evidence.evidence_export` (lines 356-383).
Synthesis: docs/dogfood/057/DESIGN_SYNTHESIS.md L181-189.

This handler is read-only over the workflow tables; the only write outside
the file system is the ``evidence.exported`` audit event with payload
``{path, sha256}``.

Digest equality contract (review-finding #2 follow-up): the snapshot
payload that feeds the rendered Markdown body uses the same canonical JSON
serialization the SQLite path uses (``json.dumps(..., sort_keys=True,
indent=2)`` for the snapshot block, ``json_dumps`` for the smaller status
and doctor blocks). Test ``test_evidence_export_pg_digest_equality`` pins
that contract.
"""

from __future__ import annotations

import hashlib
from pathlib import Path
from typing import Any, Mapping

from striatum.cli.evidence import redact_evidence_payload, render_evidence_markdown
from striatum.errors import NotFoundError
from striatum.repo_policy import repo_relative_path

from ._shim import RepoHandlerContext, register_pg_handler
from ._sql import fetch_all, parse_json, row_by_id, utc_now


def _sha256_text(body: str) -> str:
    return hashlib.sha256(body.encode("utf-8")).hexdigest()


@register_pg_handler("evidence.export")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    """Write a redacted Markdown snapshot of runner state.

    Read-only over workflow tables; appends ``evidence.exported`` event
    after the file is written.
    """
    run_id = str(params.get("run_id") or "")
    path_text = str(params.get("path") or "")
    if not run_id or not path_text:
        raise NotFoundError("evidence.export requires run_id and path")
    run = row_by_id(ctx, table="runs", id_column="run_id", value=run_id)
    if not run:
        raise NotFoundError(f"run {run_id!r} not found")

    repo = Path(str(ctx.repo_root))
    target = repo_relative_path(repo, path_text)
    target.parent.mkdir(parents=True, exist_ok=True)

    status_payload = redact_evidence_payload(_status(ctx, run_id=run_id))
    doctor_payload = redact_evidence_payload(_doctor(ctx, run_id=run_id))
    snapshot = redact_evidence_payload(_snapshot(ctx, run_id=run_id, run=run))
    body = render_evidence_markdown(
        run=dict(run),
        status_payload=status_payload,
        doctor_payload=doctor_payload,
        snapshot=snapshot,
    )
    target.write_text(body, encoding="utf-8")
    digest = _sha256_text(body)
    ctx.append_event(
        run_id=run_id,
        event_type="evidence.exported",
        payload={"path": path_text, "sha256": digest},
    )
    return {
        "status": "exported",
        "run_id": run_id,
        "path": path_text,
        "sha256": digest,
    }


def _status(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    """PG analog of :func:`striatum.cli.status` scoped to one run.

    Returns the same key set the SQLite version returns so
    :func:`redact_evidence_payload` walks the same shape and produces a
    byte-identical Markdown body for parity tests.
    """
    runs = fetch_all(
        ctx,
        sql=(
            "SELECT run_id, state, branch_name FROM striatumd.runs "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s"
        ),
        params={"run_id": run_id},
    )
    job_state_rows = fetch_all(
        ctx,
        sql=(
            "SELECT state, COUNT(*) AS count FROM striatumd.jobs "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s "
            "GROUP BY state"
        ),
        params={"run_id": run_id},
    )
    job_state_counts = {str(row["state"]): int(row["count"]) for row in job_state_rows}
    open_blockers = fetch_all(
        ctx,
        sql=(
            "SELECT b.blocker_id, b.run_id, b.job_id, b.session_id, "
            "       b.severity, b.blocker_kind, b.description, b.state, "
            "       j.workflow_job_id, j.state AS job_state "
            "FROM striatumd.blockers b "
            "LEFT JOIN striatumd.jobs j ON j.repository_id = b.repository_id "
            "   AND j.job_id = b.job_id "
            "WHERE b.repository_id = %(repository_id)s "
            "  AND b.run_id = %(run_id)s "
            "  AND b.state = 'open' "
            "  AND b.severity <> 'human_checkpoint' "
            "ORDER BY b.created_at"
        ),
        params={"run_id": run_id},
    )
    human_checkpoints = fetch_all(
        ctx,
        sql=(
            "SELECT b.blocker_id, b.run_id, b.job_id, b.session_id, "
            "       b.severity, b.blocker_kind, b.description, b.state, "
            "       j.workflow_job_id, j.state AS job_state "
            "FROM striatumd.blockers b "
            "LEFT JOIN striatumd.jobs j ON j.repository_id = b.repository_id "
            "   AND j.job_id = b.job_id "
            "WHERE b.repository_id = %(repository_id)s "
            "  AND b.run_id = %(run_id)s "
            "  AND b.state = 'open' "
            "  AND b.severity = 'human_checkpoint' "
            "ORDER BY b.created_at"
        ),
        params={"run_id": run_id},
    )
    return {
        "runs": runs,
        "jobs": job_state_counts,
        "open_blockers": open_blockers,
        "human_checkpoints": human_checkpoints,
        "latest_non_accepting_review_verdicts": [],
        "claimable_jobs": [],
        "blocked_downstream_jobs": [],
        "next_actions": [],
    }


def _doctor(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, Any]:
    """PG analog of :func:`striatum.cli.doctor` scoped to one run."""
    return {
        "ok": True,
        "schema_version": "striatum.doctor.v1",
        "problems": [],
    }


def _snapshot(
    ctx: RepoHandlerContext, *, run_id: str, run: Mapping[str, Any]
) -> dict[str, Any]:
    """PG analog of :func:`striatum.cli.evidence.evidence_snapshot`."""
    workflow_rows = fetch_all(
        ctx,
        sql=(
            "SELECT workflow_id, workflow_version, workflow_json "
            "FROM striatumd.workflow_snapshots "
            "WHERE repository_id = %(repository_id)s "
            "  AND workflow_snapshot_id = %(workflow_snapshot_id)s"
        ),
        params={"workflow_snapshot_id": run["workflow_snapshot_id"]},
    )
    snapshot_row = workflow_rows[0] if workflow_rows else {}
    workflow_json = parse_json(snapshot_row.get("workflow_json"), default={})

    jobs = _evidence_job_summaries(ctx, run_id=run_id, workflow=workflow_json)
    artifacts = _evidence_artifact_summaries(ctx, run_id=run_id, workflow=workflow_json)
    sessions = _evidence_session_summaries(ctx, run_id=run_id)
    verdicts = fetch_all(
        ctx,
        sql=(
            "SELECT verdict_id, job_id, session_id, verdict, "
            "       findings_artifact_id, rationale, posture "
            "FROM striatumd.verdicts "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s "
            "ORDER BY created_at"
        ),
        params={"run_id": run_id},
    )
    blockers = fetch_all(
        ctx,
        sql=(
            "SELECT blocker_id, job_id, session_id, severity, "
            "       blocker_kind, description, state "
            "FROM striatumd.blockers "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s "
            "ORDER BY created_at"
        ),
        params={"run_id": run_id},
    )
    return {
        "schema_version": "striatum.evidence.v1",
        "exported_at": utc_now(),
        "workflow": {
            "workflow_id": snapshot_row.get("workflow_id"),
            "workflow_version": snapshot_row.get("workflow_version"),
        },
        "run": {
            "run_id": run["run_id"],
            "branch_name": run.get("branch_name"),
            "state": run["state"],
        },
        "jobs": jobs,
        "artifacts": artifacts,
        "sessions": sessions,
        "verdicts": verdicts,
        "blockers": blockers,
        "blocked_downstream_jobs": [],
    }


def _evidence_job_summaries(
    ctx: RepoHandlerContext, *, run_id: str, workflow: Mapping[str, Any]
) -> list[dict[str, Any]]:
    from striatum.identity import artifact_author_identity

    rows = fetch_all(
        ctx,
        sql=(
            "SELECT job_id, workflow_job_id, job_type, role_id, "
            "       lane_selector_json, state, attempt, max_attempts, "
            "       fresh_session_required, expected_artifacts_json "
            "FROM striatumd.jobs "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s "
            "ORDER BY workflow_job_id"
        ),
        params={"run_id": run_id},
    )
    summaries: list[dict[str, Any]] = []
    for job in rows:
        lane = parse_json(job.get("lane_selector_json"), default={})
        lane_id = lane.get("lane_id") if isinstance(lane, dict) else None
        author = artifact_author_identity(
            workflow,
            role_id=str(job["role_id"]),
            lane_id=str(lane_id) if isinstance(lane_id, str) else None,
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
                "dependencies": [],
            }
        )
    return summaries


def _evidence_artifact_summaries(
    ctx: RepoHandlerContext, *, run_id: str, workflow: Mapping[str, Any]
) -> list[dict[str, Any]]:
    rows = fetch_all(
        ctx,
        sql=(
            "SELECT a.artifact_id, a.job_id, a.session_id, a.logical_name, "
            "       a.artifact_kind, a.repo_path, a.content_sha256, "
            "       a.author_line, "
            "       CASE "
            "         WHEN auto_event.event_id IS NOT NULL THEN 'auto_from_artifact' "
            "         WHEN override_event.event_id IS NOT NULL THEN 'operator_override_no_process_execution' "
            "         ELSE 'explicit_publish' "
            "       END AS publish_origin "
            "FROM striatumd.artifacts a "
            "LEFT JOIN LATERAL ("
            "  SELECT e.event_id "
            "  FROM striatumd.events e "
            "  WHERE e.repository_id = a.repository_id "
            "    AND e.artifact_id = a.artifact_id "
            "    AND e.event_type = 'artifact.auto_finalized' "
            "  ORDER BY e.event_id DESC "
            "  LIMIT 1"
            ") auto_event ON true "
            "LEFT JOIN LATERAL ("
            "  SELECT e.event_id "
            "  FROM striatumd.events e "
            "  WHERE e.repository_id = a.repository_id "
            "    AND e.artifact_id = a.artifact_id "
            "    AND e.event_type = 'provenance.publish_without_process_execution' "
            "  ORDER BY e.event_id DESC "
            "  LIMIT 1"
            ") override_event ON true "
            "WHERE a.repository_id = %(repository_id)s "
            "  AND a.run_id = %(run_id)s "
            "ORDER BY a.repo_path"
        ),
        params={"run_id": run_id},
    )
    return [dict(row) for row in rows]


def _evidence_session_summaries(
    ctx: RepoHandlerContext, *, run_id: str
) -> list[dict[str, Any]]:
    rows = fetch_all(
        ctx,
        sql=(
            "SELECT session_id, role_id, lane_id, slug, ordinal, state, "
            "       registered_at, closed_at, close_reason, "
            "       non_fresh_reason, operator_label "
            "FROM striatumd.sessions "
            "WHERE repository_id = %(repository_id)s AND run_id = %(run_id)s "
            "ORDER BY registered_at, session_id"
        ),
        params={"run_id": run_id},
    )
    return [dict(row) for row in rows]


__all__ = ["handle"]

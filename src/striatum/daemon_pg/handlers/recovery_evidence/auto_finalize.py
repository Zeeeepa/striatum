"""PG handler for ``recovery.auto_finalize``.

Auto-finalize is the RFC 0051 safety net for sessions that wrote the
declared expected artifact but failed to publish and complete the packet.
It uses durable repository files plus daemon-owned lease/session state; it
does not inspect terminal output or provider transcripts.
"""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, cast

from striatum.artifact_contracts import (
    ALLOWED_ARTIFACT_KINDS,
    FRONT_MATTER_SCHEMAS,
    _canonical_byline_form,
    _front_matter_block,
    _parse_front_matter,
    markdown_title_block_author_lines,
    validate_artifact_front_matter,
)
from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _json_list,
    _json_loads,
    expected_author_line_pg,
    transaction,
    validate_optional_markdown_author_line_pg,
    workflow_for_run,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_pg.handlers.workflow_loop.ack_work import ack_inline
from striatum.daemon_pg.handlers.workflow_loop.artifact_publish import (
    _artifact_payload_path,
    publish_artifact_inline,
)
from striatum.daemon_pg.handlers.workflow_loop.complete_job import complete_inline
from striatum.daemon_pg.handlers.workflow_loop.record_verdict import record_verdict_inline
from striatum.errors import ArtifactError, InvalidTransitionError, NotFoundError
from striatum.primitives import sha256_bytes
from striatum.recovery import auto_finalize_causes as causes
from striatum.repo_policy import path_allowed

DEFAULT_MTIME_GRACE_SECONDS = 30
AUTO_FINALIZE_SUMMARY = "auto-finalized from stable expected artifact files"
NO_PROCESS_RATIONALE = (
    "auto-finalized from stable expected artifact front matter; no terminal "
    "output or transcript was used as evidence"
)


@register_pg_handler("recovery.auto_finalize")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = str(params.get("run_id") or "")
    if not run_id:
        raise NotFoundError("recovery.auto_finalize requires run_id")
    run = ctx.row_by_id("runs", "run_id", run_id)
    if run["state"] != "running":
        return _empty_result(
            run_id=run_id,
            dry_run=_dry_run(params),
            reason=f"run is not running: {run['state']}",
            cause=causes.RUN_NOT_RUNNING,
            cause_detail=run["state"],
            policy=_policy_result(ctx, run_id=run_id, params=params),
        )

    dry_run = _dry_run(params)
    policy = _policy_result(ctx, run_id=run_id, params=params)
    if not dry_run and not policy["live_allowed"]:
        raise InvalidTransitionError(
            "recovery.auto_finalize live mode requires workflow recovery.auto_finalize.enabled=true "
            "or --force"
        )

    mtime_grace_seconds = _mtime_grace_seconds(params)
    allow_no_process_execution = bool(params.get("allow_no_process_execution", False))
    candidates = _candidate_rows(
        ctx,
        run_id=run_id,
        job_id=(
            str(params["job_id"])
            if params.get("job_id") is not None
            else None
        ),
    )
    eligible: list[dict[str, Any]] = []
    finalized: list[dict[str, Any]] = []
    skipped: list[dict[str, Any]] = []

    for row in candidates:
        evaluation = _evaluate_candidate(
            ctx,
            row=row,
            mtime_grace_seconds=mtime_grace_seconds,
            allow_no_process_execution=allow_no_process_execution,
        )
        if not evaluation["eligible"]:
            skipped.append(evaluation["skip"])
            continue
        if dry_run:
            eligible.append(evaluation["candidate"])
            continue
        try:
            finalized.append(
                _finalize_candidate(
                    ctx,
                    candidate=evaluation["candidate"],
                    allow_no_process_execution=allow_no_process_execution,
                )
            )
        except Exception as exc:  # noqa: BLE001 - continue sweep, report refusal.
            skipped.append({
                "workflow_job_id": row.get("workflow_job_id"),
                "job_id": row.get("job_id"),
                "reason": f"live auto-finalize failed: {exc}",
                **causes.cause_fields(
                    causes.finalize_failure_cause(exc),
                    type(exc).__name__,
                ),
            })

    return {
        "run_id": run_id,
        "dry_run": dry_run,
        "policy": policy,
        "lane_finalization_summary": _lane_finalization_summary(
            auto_from_artifact=len(eligible) + len(finalized)
        ),
        "eligible_count": len(eligible),
        "eligible": eligible,
        "finalized_count": len(finalized),
        "finalized": finalized,
        "skipped_count": len(skipped),
        "skipped": skipped,
    }


def dry_run_projection(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    mtime_grace_seconds: int = DEFAULT_MTIME_GRACE_SECONDS,
) -> dict[str, Any]:
    """Return the read-only auto-finalize projection used by status surfaces."""
    try:
        run = ctx.row_by_id("runs", "run_id", run_id)
    except NotFoundError:
        raise
    params: dict[str, Any] = {
        "run_id": run_id,
        "dry_run": True,
        "mtime_grace_seconds": mtime_grace_seconds,
    }
    policy = _policy_result(ctx, run_id=run_id, params=params)
    if run["state"] != "running":
        return _empty_result(
            run_id=run_id,
            dry_run=True,
            reason=f"run is not running: {run['state']}",
            cause=causes.RUN_NOT_RUNNING,
            cause_detail=run["state"],
            policy=policy,
        )

    eligible: list[dict[str, Any]] = []
    skipped: list[dict[str, Any]] = []
    for row in _candidate_rows(ctx, run_id=run_id, job_id=None):
        try:
            evaluation = _evaluate_candidate(
                ctx,
                row=row,
                mtime_grace_seconds=mtime_grace_seconds,
                allow_no_process_execution=False,
            )
        except Exception as exc:  # noqa: BLE001 - projection should not break status.
            skipped.append(
                {
                    "workflow_job_id": row.get("workflow_job_id"),
                    "job_id": row.get("job_id"),
                    "reason": f"auto-finalize dry-run projection failed: {exc}",
                    **causes.cause_fields(
                        causes.PROJECTION_EVALUATION_FAILED,
                        type(exc).__name__,
                    ),
                }
            )
            continue
        if evaluation["eligible"]:
            eligible.append(evaluation["candidate"])
        else:
            skipped.append(evaluation["skip"])

    return {
        "run_id": run_id,
        "dry_run": True,
        "policy": policy,
        "lane_finalization_summary": _lane_finalization_summary(
            auto_from_artifact=len(eligible)
        ),
        "eligible_count": len(eligible),
        "eligible": eligible,
        "finalized_count": 0,
        "finalized": [],
        "skipped_count": len(skipped),
        "skipped": skipped,
    }


def _dry_run(params: Mapping[str, Any]) -> bool:
    value = params.get("dry_run")
    if value is None:
        return True
    return bool(value)


def _mtime_grace_seconds(params: Mapping[str, Any]) -> int:
    raw = params.get("mtime_grace_seconds", DEFAULT_MTIME_GRACE_SECONDS)
    value = int(raw)
    if value < 0:
        raise InvalidTransitionError("mtime_grace_seconds must be non-negative")
    return value


def _policy_result(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    params: Mapping[str, Any],
) -> dict[str, Any]:
    workflow = workflow_for_run(ctx, run_id=run_id)
    recovery = workflow.get("recovery")
    auto_finalize: object = None
    if isinstance(recovery, dict):
        auto_finalize = recovery.get("auto_finalize")
    if auto_finalize is None:
        auto_finalize = workflow.get("auto_finalize")
    workflow_enabled = False
    if isinstance(auto_finalize, dict):
        workflow_enabled = auto_finalize.get("enabled") is True
    elif isinstance(auto_finalize, bool):
        workflow_enabled = auto_finalize
    force = bool(params.get("force", False))
    return {
        "workflow_enabled": workflow_enabled,
        "force": force,
        "live_allowed": workflow_enabled or force,
        "mtime_grace_seconds": _mtime_grace_seconds(params),
        "allow_no_process_execution": bool(params.get("allow_no_process_execution", False)),
    }


def _empty_result(
    *,
    run_id: str,
    dry_run: bool,
    reason: str,
    cause: str,
    cause_detail: object | None = None,
    policy: Mapping[str, Any],
) -> dict[str, Any]:
    return {
        "run_id": run_id,
        "dry_run": dry_run,
        "policy": dict(policy),
        "lane_finalization_summary": _lane_finalization_summary(),
        "eligible_count": 0,
        "eligible": [],
        "finalized_count": 0,
        "finalized": [],
        "skipped_count": 1,
        "skipped": [{"reason": reason, **causes.cause_fields(cause, cause_detail)}],
    }


def _candidate_rows(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    job_id: str | None,
) -> list[dict[str, Any]]:
    now = ctx.now()
    job_filter = "AND j.job_id = %s" if job_id is not None else ""
    params: list[Any] = [ctx.repository_id, run_id, now]
    if job_id is not None:
        params.append(job_id)
    with ctx.cursor() as cur:
        cur.execute(
            f"""
            SELECT j.*, l.lease_id, l.owner_session_id, l.expires_at,
                   l.state AS lease_state, s.state AS session_state,
                   qm.message_id, qm.state AS message_state
            FROM striatumd.jobs j
            JOIN striatumd.leases l
              ON l.repository_id = j.repository_id
             AND l.lease_id = j.current_lease_id
            JOIN striatumd.sessions s
              ON s.repository_id = j.repository_id
             AND s.session_id = l.owner_session_id
            LEFT JOIN striatumd.queue_messages qm
              ON qm.repository_id = j.repository_id
             AND qm.message_id = j.current_message_id
            WHERE j.repository_id = %s
              AND j.run_id = %s
              AND j.state IN ('claimed', 'running')
              AND l.state = 'active'
              AND l.expires_at >= %s
              AND s.state = 'active'
              AND (qm.message_id IS NULL OR qm.state IN ('claimed', 'acked'))
              {job_filter}
            ORDER BY j.workflow_job_id
            """,
            tuple(params),
        )
        return [dict(row) for row in cur.fetchall()]


def _evaluate_candidate(
    ctx: RepoHandlerContext,
    *,
    row: Mapping[str, Any],
    mtime_grace_seconds: int,
    allow_no_process_execution: bool,
) -> dict[str, Any]:
    job_id = str(row["job_id"])
    session_id = str(row["owner_session_id"])
    lease_id = str(row["lease_id"])
    message_id = (
        str(row["message_id"]) if row.get("message_id") is not None else None
    )
    workflow_job_id = str(row["workflow_job_id"])
    expected_artifacts = [
        item
        for item in _json_list(row["expected_artifacts_json"])
        if isinstance(item, dict) and item.get("required", True) is not False
    ]
    base: dict[str, Any] = {
        "workflow_job_id": workflow_job_id,
        "job_id": job_id,
        "session_id": session_id,
        "lease_id": lease_id,
        "message_id": message_id,
    }
    if not expected_artifacts:
        return _not_eligible(
            base,
            "no required expected_artifacts declared",
            causes.NO_REQUIRED_EXPECTED_ARTIFACTS,
        )
    if message_id is None:
        return _not_eligible(base, "no active queue message is attached", causes.NO_ACTIVE_MESSAGE)
    try:
        expected_byline = expected_author_line_pg(ctx, job=row, session_id=session_id)
    except ArtifactError as exc:
        return _not_eligible(
            base,
            f"could not derive expected byline: {exc}",
            causes.EXPECTED_BYLINE_UNRESOLVABLE,
            type(exc).__name__,
        )

    artifacts: list[dict[str, Any]] = []
    refusals: list[dict[str, Any]] = []
    for declared in expected_artifacts:
        result = _evaluate_artifact(
            ctx,
            job=row,
            job_id=job_id,
            session_id=session_id,
            expected_byline=expected_byline,
            declared=cast(Mapping[str, Any], declared),
            mtime_grace_seconds=mtime_grace_seconds,
            allow_no_process_execution=allow_no_process_execution,
        )
        if result.get("ok") is True:
            artifacts.append(cast(dict[str, Any], result["artifact"]))
        else:
            refusals.append(cast(dict[str, Any], result["refusal"]))
    if refusals:
        skip = dict(base)
        skip["reason"] = "expected_artifact validation refused"
        skip.update(causes.cause_fields(causes.first_refusal_cause(refusals)))
        skip["artifacts"] = refusals
        return {"eligible": False, "skip": skip}

    candidate = dict(base)
    candidate.update({
        "lane_finalization": "auto_from_artifact",
        "message_state": row.get("message_state"),
        "job_state": row.get("state"),
        "expected_byline": expected_byline,
        "artifacts": artifacts,
    })
    return {"eligible": True, "candidate": candidate}


def _evaluate_artifact(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    job_id: str,
    session_id: str,
    expected_byline: str,
    declared: Mapping[str, Any],
    mtime_grace_seconds: int,
    allow_no_process_execution: bool,
) -> dict[str, Any]:
    path_text = declared.get("path")
    kind = declared.get("kind")
    logical_name = declared.get("logical_name")
    if not isinstance(path_text, str) or not path_text:
        return _artifact_refusal(declared, "expected_artifact path is missing", causes.ARTIFACT_PATH_MISSING)
    if not isinstance(kind, str) or kind not in ALLOWED_ARTIFACT_KINDS:
        return _artifact_refusal(declared, "expected_artifact kind is invalid", causes.ARTIFACT_KIND_INVALID)
    if kind == "transcript":
        return _artifact_refusal(
            declared,
            "transcript artifacts are not auto-finalized",
            causes.ARTIFACT_KIND_TRANSCRIPT,
        )
    if not isinstance(logical_name, str) or not logical_name:
        return _artifact_refusal(
            declared,
            "expected_artifact logical_name is missing",
            causes.ARTIFACT_LOGICAL_NAME_MISSING,
        )
    write_scope = _json_loads(job["write_scope_json"])
    if not path_allowed(ctx.repo_root, path_text, write_scope):
        return _artifact_refusal(
            declared,
            "artifact path is outside the job write scope",
            causes.ARTIFACT_PATH_OUTSIDE_WRITE_SCOPE,
        )
    try:
        path = _artifact_payload_path(ctx, job_id=job_id, path_text=path_text)
    except ArtifactError as exc:
        return _artifact_refusal(declared, str(exc), causes.ARTIFACT_PAYLOAD_PATH_INVALID, type(exc).__name__)
    if not path.is_file():
        return _artifact_refusal(declared, "expected artifact file does not exist", causes.ARTIFACT_FILE_MISSING)
    stable = _mtime_stable(path, mtime_grace_seconds=mtime_grace_seconds)
    if not stable["stable"]:
        return _artifact_refusal(
            declared,
            "expected artifact file mtime is inside the grace period",
            causes.ARTIFACT_MTIME_INSIDE_GRACE,
            extra={"age_seconds": stable["age_seconds"]},
        )
    try:
        payload = path.read_bytes()
    except OSError as exc:
        return _artifact_refusal(
            declared,
            f"could not read expected artifact: {exc}",
            causes.ARTIFACT_READ_FAILED,
            type(exc).__name__,
        )
    try:
        validate_artifact_front_matter(kind=kind, path=path, payload=payload)
        front_matter = _required_front_matter(kind=kind, path=path, payload=payload)
        validate_optional_markdown_author_line_pg(
            ctx, job=job, session_id=session_id, path=path, payload=payload
        )
        _validate_required_byline(
            payload=payload,
            expected_byline=expected_byline,
        )
    except ArtifactError as exc:
        return _artifact_refusal(declared, str(exc), causes.artifact_validation_cause(str(exc)))
    lane_reason = _lane_evidence_refusal(
        ctx,
        session_id=session_id,
        expected_byline=expected_byline,
        allow_no_process_execution=allow_no_process_execution,
    )
    if lane_reason is not None:
        return _artifact_refusal(declared, lane_reason, causes.LANE_EVIDENCE_MISSING)
    existing = _existing_artifact_status(
        ctx,
        run_id=str(job["run_id"]),
        job_id=job_id,
        logical_name=logical_name,
        path_text=path_text,
        payload=payload,
    )
    if existing.get("status") == "conflict":
        return _artifact_refusal(
            declared,
            str(existing["reason"]),
            causes.ARTIFACT_CONFLICT_EXISTING_CONTENT,
        )
    return {
        "ok": True,
        "artifact": {
            "path": path_text,
            "disk_path": str(path),
            "kind": kind,
            "logical_name": logical_name,
            "sha256": sha256_bytes(payload),
            "size_bytes": len(payload),
            "mtime_age_seconds": stable["age_seconds"],
            "publish_status": existing["status"],
            "artifact_id": existing.get("artifact_id"),
            "front_matter": front_matter,
        },
    }


def _not_eligible(
    base: Mapping[str, Any],
    reason: str,
    cause: str,
    cause_detail: object | None = None,
) -> dict[str, Any]:
    skip = dict(base)
    skip["reason"] = reason
    skip.update(causes.cause_fields(cause, cause_detail))
    return {"eligible": False, "skip": skip}


def _artifact_refusal(
    declared: Mapping[str, Any],
    reason: str,
    cause: str,
    cause_detail: object | None = None,
    *,
    extra: Mapping[str, Any] | None = None,
) -> dict[str, Any]:
    refusal = {
        "path": declared.get("path"),
        "kind": declared.get("kind"),
        "logical_name": declared.get("logical_name"),
        "reason": reason,
        **causes.cause_fields(cause, cause_detail),
    }
    if extra:
        refusal.update(extra)
    return {"ok": False, "refusal": refusal}


def _mtime_stable(path: Path, *, mtime_grace_seconds: int) -> dict[str, Any]:
    mtime = path.stat().st_mtime
    age_seconds = max(0.0, datetime.now(UTC).timestamp() - mtime)
    return {
        "stable": age_seconds >= mtime_grace_seconds,
        "age_seconds": round(age_seconds, 3),
    }


def _validate_required_byline(*, payload: bytes, expected_byline: str) -> None:
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError("markdown artifact must be UTF-8") from exc
    author_lines = markdown_title_block_author_lines(text)
    if not author_lines:
        raise ArtifactError("markdown artifact author line is required for auto-finalize")
    if not any(_canonical_byline_form(line) == expected_byline for line in author_lines):
        raise ArtifactError("markdown artifact author line must match expected work packet author line")


def _required_front_matter(
    *,
    kind: str,
    path: Path,
    payload: bytes,
) -> dict[str, Any] | None:
    if kind not in FRONT_MATTER_SCHEMAS:
        return None
    if path.suffix.lower() not in {".md", ".markdown"}:
        raise ArtifactError(f"{kind} artifact must be Markdown to auto-finalize front matter")
    try:
        text = payload.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ArtifactError(f"{kind} artifact must be UTF-8 to validate front matter") from exc
    block = _front_matter_block(text)
    if block is None:
        raise ArtifactError(f"{kind} artifact front matter is required for auto-finalize")
    parsed = _parse_front_matter(block, kind=kind)
    if kind == "finding" and not isinstance(parsed.get("verdict_intent"), str):
        raise ArtifactError("finding artifact front matter missing required field 'verdict_intent'")
    return parsed


def _lane_evidence_refusal(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    expected_byline: str,
    allow_no_process_execution: bool,
) -> str | None:
    if expected_byline.startswith("author: operator") or allow_no_process_execution:
        return None
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT 1
            FROM striatumd.process_executions
            WHERE repository_id = %s
              AND session_id = %s
              AND state = 'exited'
              AND exit_code = 0
            LIMIT 1
            """,
            (ctx.repository_id, session_id),
        )
        found = cur.fetchone()
    if found is None:
        return "lane_evidence_missing: no clean process_executions row for the session"
    return None


def _existing_artifact_status(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    job_id: str,
    logical_name: str,
    path_text: str,
    payload: bytes,
) -> dict[str, Any]:
    digest = sha256_bytes(payload)
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT artifact_id, repo_path, content_sha256
            FROM striatumd.artifacts
            WHERE repository_id = %s AND run_id = %s AND job_id = %s
              AND logical_name = %s
            """,
            (ctx.repository_id, run_id, job_id, logical_name),
        )
        row = cur.fetchone()
    if row is None:
        return {"status": "would_publish"}
    if row["repo_path"] == path_text and row["content_sha256"] == digest:
        return {"status": "already_published", "artifact_id": row["artifact_id"]}
    return {"status": "conflict", "reason": "artifact logical name already exists with different content"}


def _finalize_candidate(
    ctx: RepoHandlerContext,
    *,
    candidate: Mapping[str, Any],
    allow_no_process_execution: bool,
) -> dict[str, Any]:
    session_id = str(candidate["session_id"])
    job_id = str(candidate["job_id"])
    lease_id = str(candidate["lease_id"])
    message_id = str(candidate["message_id"])
    artifacts = cast(list[Mapping[str, Any]], candidate["artifacts"])
    with transaction(ctx):
        if candidate.get("message_state") == "claimed":
            ack_inline(
                ctx,
                session_id=session_id,
                message_id=message_id,
                lease_id=lease_id,
            )
        published: list[dict[str, Any]] = []
        for artifact in artifacts:
            result = publish_artifact_inline(
                ctx,
                session_id=session_id,
                job_id=job_id,
                lease_id=lease_id,
                kind=str(artifact["kind"]),
                logical_name=str(artifact["logical_name"]),
                path_text=str(artifact["path"]),
                allow_no_process_execution=allow_no_process_execution,
                override_rationale=(
                    NO_PROCESS_RATIONALE if allow_no_process_execution else None
                ),
            )
            artifact_id = str(result["artifact_id"])
            ctx.append_event(
                run_id=_run_id_for_job(ctx, job_id=job_id),
                event_type="artifact.auto_finalized",
                actor_session_id=session_id,
                job_id=job_id,
                artifact_id=artifact_id,
                lease_id=lease_id,
                payload={
                    "path": artifact["path"],
                    "kind": artifact["kind"],
                    "logical_name": artifact["logical_name"],
                    "sha256": result.get("sha256") or artifact.get("sha256"),
                    "validation": {
                        "byline": candidate.get("expected_byline"),
                        "mtime_age_seconds": artifact.get("mtime_age_seconds"),
                        "source": "expected_artifact_file",
                    },
                    "rationale": AUTO_FINALIZE_SUMMARY,
                },
            )
            published.append({
                "path": artifact["path"],
                "kind": artifact["kind"],
                "logical_name": artifact["logical_name"],
                "artifact_id": artifact_id,
                "status": result["status"],
            })
        run_id = _run_id_for_job(ctx, job_id=job_id)
        job = ctx.row_by_id("jobs", "job_id", job_id)
        ctx.append_event(
            run_id=run_id,
            event_type="job.auto_finalized",
            actor_session_id=session_id,
            job_id=job_id,
            message_id=message_id,
            lease_id=lease_id,
            payload={
                "artifact_count": len(published),
                "rationale": AUTO_FINALIZE_SUMMARY,
                "lane_finalization": "auto_from_artifact",
                "validation": {
                    "byline": candidate.get("expected_byline"),
                    "source": "expected_artifact_file",
                },
            },
        )
        if job["job_type"] in {"review", "phase_synthesis"}:
            verdict_artifact = _verdict_artifact(artifacts)
            verdict = _verdict_from_artifact(verdict_artifact)
            complete_result = record_verdict_inline(
                ctx,
                session_id=session_id,
                job_id=job_id,
                lease_id=lease_id,
                verdict=verdict,
                findings_artifact_id=str(
                    _published_artifact_id(
                        published,
                        logical_name=str(verdict_artifact["logical_name"]),
                    )
                ),
                rationale=AUTO_FINALIZE_SUMMARY,
            )
        else:
            complete_result = complete_inline(
                ctx,
                session_id=session_id,
                job_id=job_id,
                lease_id=lease_id,
                summary=AUTO_FINALIZE_SUMMARY,
            )
        return {
            "workflow_job_id": candidate["workflow_job_id"],
            "job_id": job_id,
            "session_id": session_id,
            "lane_finalization": "auto_from_artifact",
            "summary": AUTO_FINALIZE_SUMMARY,
            "artifacts": published,
            "complete": complete_result,
        }


def _lane_finalization_summary(
    *,
    auto_from_artifact: int = 0,
    manual_publish: int = 0,
    pending: int = 0,
) -> dict[str, int]:
    return {
        "auto_from_artifact": auto_from_artifact,
        "manual_publish": manual_publish,
        "pending": pending,
    }


def _run_id_for_job(ctx: RepoHandlerContext, *, job_id: str) -> str:
    job = ctx.row_by_id("jobs", "job_id", job_id)
    return str(job["run_id"])


def _verdict_artifact(artifacts: list[Mapping[str, Any]]) -> Mapping[str, Any]:
    for artifact in artifacts:
        if artifact.get("kind") == "finding":
            front_matter = artifact.get("front_matter")
            if isinstance(front_matter, dict) and isinstance(front_matter.get("verdict_intent"), str):
                return artifact
    raise InvalidTransitionError(
        "review auto-finalize requires a finding expected_artifact with verdict_intent front matter"
    )


def _verdict_from_artifact(artifact: Mapping[str, Any]) -> str:
    front_matter = artifact.get("front_matter")
    if not isinstance(front_matter, dict):
        raise InvalidTransitionError("review auto-finalize could not read finding front matter")
    verdict = front_matter.get("verdict_intent")
    if not isinstance(verdict, str):
        raise InvalidTransitionError("review auto-finalize requires verdict_intent")
    return verdict


def _published_artifact_id(
    published: Sequence[Mapping[str, Any]],
    *,
    logical_name: str,
) -> str:
    for artifact in published:
        if artifact.get("logical_name") == logical_name:
            return str(artifact["artifact_id"])
    raise InvalidTransitionError("review auto-finalize could not resolve findings artifact id")


__all__ = ["dry_run_projection", "handle"]

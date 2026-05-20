"""PG-backed ``artifact.publish`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from pathlib import Path, PurePosixPath
from typing import Any, cast

from striatum.artifact_contracts import (
    ALLOWED_ARTIFACT_KINDS,
    _first_author_line,
    ensure_required_front_matter,
    parse_artifact_front_matter,
    validate_artifact_front_matter,
)
from striatum.daemon_pg.handlers.context import (
    RepoHandlerContext,
    _jsonb,
    _json_loads,
    active_lease_for,
    expected_author_line_pg,
    session_lane_attestation,
    transaction,
    validate_optional_markdown_author_line_pg,
    workflow_for_run,
)
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.errors import ArtifactError
from striatum.escalations import ESCALATION_BLOCKER_KINDS
from striatum.primitives import sha256_bytes
from striatum.repo_policy import path_allowed, repo_relative_path


@register_pg_handler("artifact.publish", "publish_artifact")
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    with transaction(ctx):
        return publish_artifact_inline(
            ctx,
            session_id=str(params["session_id"]),
            job_id=str(params["job_id"]),
            lease_id=str(params["lease_id"]),
            kind=str(params["kind"]),
            logical_name=str(params["logical_name"]),
            path_text=str(params["path"] if "path" in params else params["path_text"]),
            allow_no_process_execution=bool(params.get("allow_no_process_execution", False)),
            override_rationale=(
                str(params["override_rationale"])
                if params.get("override_rationale") is not None
                else None
            ),
        )


def publish_artifact_inline(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    kind: str,
    logical_name: str,
    path_text: str,
    allow_no_process_execution: bool = False,
    override_rationale: str | None = None,
) -> dict[str, object]:
    job = ctx.row_by_id("jobs", "job_id", job_id, for_update=True)
    active_lease_for(ctx, lease_id=lease_id, session_id=session_id, job_id=job_id)
    if kind == "transcript":
        raise ArtifactError("transcript artifacts are not allowed by default")
    if kind not in ALLOWED_ARTIFACT_KINDS:
        allowed = ", ".join(sorted(ALLOWED_ARTIFACT_KINDS))
        raise ArtifactError(
            f"artifact kind {kind!r} is not in the allowed kinds list: {allowed}"
        )
    _enforce_required_attestation_for_artifact(ctx, job=job, session_id=session_id)
    write_scope = _json_loads(job["write_scope_json"])
    if not path_allowed(ctx.repo_root, path_text, write_scope):
        raise ArtifactError("artifact path is outside the job write scope")
    repo_relative_path(ctx.repo_root, path_text)
    path = _artifact_payload_path(ctx, job_id=job_id, path_text=path_text)
    if not path.exists() or not path.is_file():
        raise ArtifactError("artifact file does not exist")
    payload = ensure_required_front_matter(kind=kind, path=path, payload=path.read_bytes())
    validate_optional_markdown_author_line_pg(
        ctx, job=job, session_id=session_id, path=path, payload=payload
    )
    validate_artifact_front_matter(kind=kind, path=path, payload=payload)
    byline = _expected_byline_or_none(ctx, job=job, session_id=session_id)
    stored_rationale = _validate_lane_evidence(
        ctx,
        session_id=session_id,
        path_text=path_text,
        byline=byline,
        allow_no_process_execution=allow_no_process_execution,
        override_rationale=override_rationale,
    )
    digest = sha256_bytes(payload)
    now = ctx.now()
    existing = _existing_artifact(
        ctx, run_id=str(job["run_id"]), job_id=job_id, logical_name=logical_name
    )
    if existing is not None:
        if existing["content_sha256"] == digest and existing["repo_path"] == path_text:
            if kind == "escalation":
                front_matter = parse_artifact_front_matter(
                    kind=kind, path=path, payload=payload
                )
                if front_matter is None:
                    raise ArtifactError(
                        "escalation artifact front matter is required to link an escalation blocker"
                    )
                _link_escalation_artifact(
                    ctx,
                    front_matter=front_matter,
                    artifact_id=str(existing["artifact_id"]),
                    run_id=str(job["run_id"]),
                    job_id=job_id,
                    session_id=session_id,
                    repo_path=path_text,
                    content_sha256=digest,
                    linked_at=str(now),
                )
            return {"status": "already_published", "artifact_id": existing["artifact_id"]}
        raise ArtifactError("artifact logical name already exists with different content")

    artifact_id = ctx.new_id("art")
    actual_author_line = _first_author_line(payload)
    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.artifacts (
              repository_id, artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
              created_at, author_line, attestation_override_rationale
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, 'create', %s, %s, %s)
            """,
            (
                ctx.repository_id,
                artifact_id,
                job["run_id"],
                job_id,
                session_id,
                logical_name,
                kind,
                path_text,
                digest,
                len(payload),
                now,
                actual_author_line,
                stored_rationale,
            ),
        )
    if kind == "escalation":
        front_matter = parse_artifact_front_matter(kind=kind, path=path, payload=payload)
        if front_matter is None:
            raise ArtifactError(
                "escalation artifact front matter is required to link an escalation blocker"
            )
        _link_escalation_artifact(
            ctx,
            front_matter=front_matter,
            artifact_id=artifact_id,
            run_id=str(job["run_id"]),
            job_id=job_id,
            session_id=session_id,
            repo_path=path_text,
            content_sha256=digest,
            linked_at=str(now),
        )
    ctx.append_event(
        run_id=str(job["run_id"]),
        event_type="artifact.published",
        actor_session_id=session_id,
        job_id=job_id,
        artifact_id=artifact_id,
        lease_id=lease_id,
        payload={"logical_name": logical_name, "path": path_text, "sha256": digest},
    )
    if stored_rationale is not None:
        ctx.append_event(
            run_id=str(job["run_id"]),
            event_type="provenance.publish_without_process_execution",
            actor_session_id=session_id,
            job_id=job_id,
            artifact_id=artifact_id,
            lease_id=lease_id,
            payload={"byline": byline, "path": path_text, "rationale": stored_rationale},
        )
    return {"status": "published", "artifact_id": artifact_id, "sha256": digest}


def _artifact_payload_path(
    ctx: RepoHandlerContext,
    *,
    job_id: str,
    path_text: str,
) -> Path:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT worktree_path
            FROM striatumd.job_worktrees
            WHERE repository_id = %s AND job_id = %s AND state = 'active'
            LIMIT 1
            """,
            (ctx.repository_id, job_id),
        )
        worktree = cur.fetchone()
    if worktree is None:
        return repo_relative_path(ctx.repo_root, path_text)
    root = Path(str(worktree["worktree_path"])).resolve()
    path = (root / path_text).resolve()
    try:
        path.relative_to(root)
    except ValueError as exc:
        raise ArtifactError("artifact path must stay inside the active worktree") from exc
    return path


def _existing_artifact(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    job_id: str,
    logical_name: str,
) -> Mapping[str, Any] | None:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT * FROM striatumd.artifacts
            WHERE repository_id = %s AND run_id = %s AND job_id = %s AND logical_name = %s
            """,
            (ctx.repository_id, run_id, job_id, logical_name),
        )
        return cast(Mapping[str, Any] | None, cur.fetchone())


def _link_escalation_artifact(
    ctx: RepoHandlerContext,
    *,
    front_matter: Mapping[str, object],
    artifact_id: str,
    run_id: str,
    job_id: str,
    session_id: str,
    repo_path: str,
    content_sha256: str,
    linked_at: str,
) -> None:
    escalation_id = front_matter.get("escalation_id")
    if not isinstance(escalation_id, str) or not escalation_id.strip():
        raise ArtifactError("escalation artifact front matter missing escalation_id")
    if front_matter.get("run_id") != run_id:
        raise ArtifactError("escalation artifact run_id must match publish context")
    front_job_id = front_matter.get("job_id")
    if front_job_id is not None and front_job_id != job_id:
        raise ArtifactError("escalation artifact job_id must match publish context")
    front_session_id = front_matter.get("session_id")
    if front_session_id is not None and front_session_id != session_id:
        raise ArtifactError("escalation artifact session_id must match publish context")

    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT blocker_id, run_id, severity, blocker_kind, payload_json
              FROM striatumd.blockers
             WHERE repository_id = %s
               AND blocker_id = %s
             FOR UPDATE
            """,
            (ctx.repository_id, escalation_id),
        )
        blocker = cur.fetchone()
    if blocker is None:
        raise ArtifactError(
            "escalation artifact escalation_id does not match an existing blocker"
        )
    if str(blocker["run_id"]) != run_id:
        raise ArtifactError("escalation artifact blocker must belong to the same run")
    if not _is_escalation_class_blocker(blocker):
        raise ArtifactError("escalation artifact blocker is not escalation-class")

    metadata = {
        "artifact_id": artifact_id,
        "repo_path": repo_path,
        "content_sha256": content_sha256,
        "linked_at": linked_at,
        "link_source": "artifact.publish",
    }
    already_linked = _compatible_existing_escalation_link(
        blocker.get("payload_json"), metadata
    )
    if already_linked:
        return
    with ctx.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.blockers
               SET payload_json = jsonb_set(
                       COALESCE(payload_json, '{}'::jsonb),
                       '{escalation_artifact}',
                       %s,
                       true
                   )
             WHERE repository_id = %s
               AND blocker_id = %s
            """,
            (_jsonb(metadata), ctx.repository_id, escalation_id),
        )
        cur.execute(
            """
            UPDATE striatumd.escalation_inbox
               SET payload_json = jsonb_set(
                       COALESCE(payload_json, '{}'::jsonb),
                       '{escalation_artifact}',
                       %s,
                       true
                   )
             WHERE repository_id = %s
               AND escalation_id = %s
            """,
            (_jsonb(metadata), ctx.repository_id, escalation_id),
        )


def _is_escalation_class_blocker(blocker: Mapping[str, Any]) -> bool:
    return (
        blocker.get("severity") == "human_checkpoint"
        or str(blocker.get("blocker_kind")) in ESCALATION_BLOCKER_KINDS
    )


def _compatible_existing_escalation_link(
    payload_json: object,
    metadata: Mapping[str, str],
) -> bool:
    if not isinstance(payload_json, Mapping):
        return False
    existing = payload_json.get("escalation_artifact")
    if not isinstance(existing, Mapping):
        return False
    for key in ("artifact_id", "repo_path", "content_sha256", "link_source"):
        value = existing.get(key)
        if isinstance(value, str) and value == metadata[key]:
            continue
        raise ArtifactError("escalation blocker is already linked to a different artifact")
    linked_at = existing.get("linked_at")
    if not isinstance(linked_at, str) or not linked_at:
        return False
    return True


def _expected_byline_or_none(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
) -> str | None:
    try:
        return expected_author_line_pg(ctx, job=job, session_id=session_id)
    except ArtifactError:
        return None


def _validate_lane_evidence(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    path_text: str,
    byline: str | None,
    allow_no_process_execution: bool,
    override_rationale: str | None,
) -> str | None:
    if byline is None or _is_operator_byline(byline):
        return None
    if _lane_evidence_present(ctx, session_id=session_id, path_text=path_text):
        return None
    if not allow_no_process_execution:
        raise ArtifactError(
            "lane_evidence_missing: artifact path "
            f"{path_text!r} is not present in any process_executions row "
            f"for session {session_id!r}; pass --allow-no-process-execution "
            '--override-rationale "<text>" to record an operator override.'
        )
    if not (override_rationale and override_rationale.strip()):
        raise ArtifactError(
            "publish-artifact --allow-no-process-execution requires a "
            "non-empty --override-rationale"
        )
    return override_rationale.strip()


def _lane_evidence_present(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    path_text: str,
) -> bool:
    observed = _supervisor_artifact_observed(ctx, session_id=session_id, path_text=path_text)
    if observed is not None:
        return observed
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
        return cur.fetchone() is not None


def _supervisor_artifact_observed(
    ctx: RepoHandlerContext,
    *,
    session_id: str,
    path_text: str,
) -> bool | None:
    target_path = _normalize_evidence_path(path_text)
    if target_path is None:
        return False
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT payload_json
            FROM striatumd.events
            WHERE repository_id = %s
              AND actor_session_id = %s
              AND event_type = 'supervisor.artifact_observed'
            ORDER BY event_id
            """,
            (ctx.repository_id, session_id),
        )
        rows = cur.fetchall()
    saw_observation = False
    for row in rows:
        for observed_path in _artifact_observed_paths(row["payload_json"]):
            saw_observation = True
            if _normalize_evidence_path(observed_path) == target_path:
                return True
    return False if saw_observation else None


def _artifact_observed_paths(payload_json: Any) -> list[str]:
    if not isinstance(payload_json, Mapping):
        return []
    candidates: list[str] = []
    payloads: list[Mapping[str, Any]] = [payload_json]
    inner = payload_json.get("payload")
    if isinstance(inner, Mapping):
        payloads.append(inner)
    for payload in payloads:
        for key in ("path", "repo_path", "artifact_path"):
            value = payload.get(key)
            if isinstance(value, str):
                candidates.append(value)
        for key in (
            "paths",
            "repo_paths",
            "artifact_paths",
            "observed_output_paths",
            "observed_output_paths_json",
        ):
            value = payload.get(key)
            if isinstance(value, list):
                candidates.extend(item for item in value if isinstance(item, str))
    return candidates


def _normalize_evidence_path(value: object) -> str | None:
    if not isinstance(value, str):
        return None
    text = value.strip().replace("\\", "/")
    if not text:
        return None
    pure = PurePosixPath(text)
    if pure.is_absolute():
        return None
    parts: list[str] = []
    for part in pure.parts:
        if part in ("", "."):
            continue
        if part == "..":
            return None
        parts.append(part)
    return "/".join(parts) if parts else None


def _is_operator_byline(byline: str) -> bool:
    return byline.startswith("author: operator")


def _enforce_required_attestation_for_artifact(
    ctx: RepoHandlerContext,
    *,
    job: Mapping[str, Any],
    session_id: str,
) -> None:
    workflow = workflow_for_run(ctx, run_id=str(job["run_id"]))
    job_def = _workflow_job_def(workflow, workflow_job_id=str(job["workflow_job_id"]))
    if not isinstance(job_def, dict) or job_def.get("require_attested_lane") is not True:
        return
    attestation = session_lane_attestation(ctx, session_id=session_id)
    if attestation["attested"]:
        return
    reason = f" ({attestation['reason']})" if attestation.get("reason") else ""
    raise ArtifactError(
        "job requires an attached lane supervisor before publishing artifacts"
        f"{reason}; recovery: striatum supervise start --session-id {session_id}"
    )


def _workflow_job_def(
    workflow: Mapping[str, Any],
    *,
    workflow_job_id: str,
) -> dict[str, Any] | None:
    jobs = workflow.get("jobs")
    if not isinstance(jobs, list):
        return None
    for item in jobs:
        if isinstance(item, dict) and item.get("id") == workflow_job_id:
            return item
    return None


__all__ = [
    "handle",
    "publish_artifact_inline",
]

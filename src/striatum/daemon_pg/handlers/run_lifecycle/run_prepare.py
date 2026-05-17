"""PG-backed ``run.prepare`` handler."""

from __future__ import annotations

from collections.abc import Mapping
from pathlib import Path
from typing import Any, cast

from psycopg.types.json import Jsonb

from striatum.daemon_pg.handlers.context import RepoHandlerContext, transaction
from striatum.daemon_pg.handlers.registry import register_pg_handler
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import WorkflowError
from striatum.primitives import JsonObject, json_dumps, sha256_bytes
from striatum.workflow import (
    VERDICT_JOB_TYPES,
    _effective_fresh_session_required,
    edge_dependency_pairs,
    load_workflow,
    workflow_job_map,
)

_PG_JOB_TYPES = frozenset(
    {
        "draft",
        "review",
        "ledger",
        "synthesis",
        "build",
        "test",
        "human_checkpoint",
        "generic",
    }
)


@register_pg_handler("run.prepare")
def handle_run_prepare(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    workflow_arg = _required_text(params, "workflow", method="run.prepare")
    repo_root = _repo_root_for(ctx)
    workflow_path, source_path = _resolve_workflow_path(repo_root, workflow_arg)
    workflow = _load_workflow(workflow_path)

    with transaction(ctx):
        return _create_run(
            ctx,
            repo_root=repo_root,
            workflow=workflow,
            source_path=source_path,
        )


def _create_run(
    ctx: RepoHandlerContext,
    *,
    repo_root: Path,
    workflow: JsonObject,
    source_path: str,
) -> dict[str, Any]:
    now = ctx.now()
    raw_json = json_dumps(workflow)
    workflow_snapshot_id = ctx.new_id("wfs")
    run_id = ctx.new_id("run")
    workflow_id = str(workflow["workflow_id"])
    branch = _branch_section(workflow)
    branch_mode = str(branch.get("mode", "auto"))
    suggested_branch = _optional_text(branch.get("suggested_name"))
    state = "needs_branch_confirmation"
    branch_confirmed_at: str | None = None
    branch_confirmed_by: str | None = None
    if branch_mode == "auto" and suggested_branch:
        state = "ready"
        branch_confirmed_at = now
        branch_confirmed_by = "daemon"

    with ctx.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots (
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              source_path, content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
            """,
            (
                ctx.repository_id,
                workflow_snapshot_id,
                workflow_id,
                _optional_text(workflow.get("workflow_version")),
                source_path,
                sha256_bytes(raw_json.encode("utf-8")),
                Jsonb(workflow),
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs (
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              branch_name, branch_base, branch_confirmed_at, branch_confirmed_by,
              created_at
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
            """,
            (
                ctx.repository_id,
                run_id,
                workflow_snapshot_id,
                str(repo_root),
                state,
                suggested_branch,
                None,
                branch_confirmed_at,
                branch_confirmed_by,
                now,
            ),
        )

    workflow_jobs = workflow_job_map(workflow)
    job_ids: dict[str, str] = {}
    jobs_value = workflow.get("jobs")
    if not isinstance(jobs_value, list):
        raise WorkflowError("workflow field 'jobs' must be a list")
    for job_value in jobs_value:
        if not isinstance(job_value, dict):
            raise WorkflowError("each job must be an object")
        job = cast(dict[str, Any], job_value)
        workflow_job_id = _job_text(job, "id")
        job_id = f"job_{run_id}_{workflow_job_id}"
        job_ids[workflow_job_id] = job_id
        lane_id = _optional_text(job.get("lane_id"))
        stored_job_type = _stored_job_type(job, workflow_job_id)
        capability_requirements = {
            "objective": job.get("objective"),
            "task_prompt": job.get("task_prompt", {}),
            "inputs": job.get("inputs", []),
        }
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.jobs (
                  repository_id, job_id, run_id, workflow_job_id, title, job_type,
                  role_id, lane_selector_json, capability_requirements_json, state,
                  max_attempts, fresh_session_required, write_scope_json,
                  expected_artifacts_json, idempotency_key, created_at
                )
                VALUES (
                  %s, %s, %s, %s, %s, %s, %s, %s, %s, 'blocked',
                  %s, %s, %s, %s, %s, %s
                )
                """,
                (
                    ctx.repository_id,
                    job_id,
                    run_id,
                    workflow_job_id,
                    str(job.get("title") or workflow_job_id),
                    stored_job_type,
                    _job_text(job, "role_id"),
                    Jsonb({"lane_id": lane_id} if lane_id is not None else {}),
                    Jsonb(capability_requirements),
                    _max_attempts(job, workflow_job_id),
                    _effective_fresh_session_required(job),
                    Jsonb(job.get("write_scope", {})),
                    Jsonb(job.get("expected_artifacts", [])),
                    f"{run_id}:{workflow_job_id}:1",
                    now,
                ),
            )

    for upstream_id, downstream_id, gate in edge_dependency_pairs(workflow):
        from_job_id = job_ids.get(upstream_id)
        to_job_id = job_ids.get(downstream_id)
        if from_job_id is None or to_job_id is None:
            raise WorkflowError("workflow edge references an unknown job")
        gate_json = dict(gate)
        upstream_job = workflow_jobs[upstream_id]
        if isinstance(upstream_job, dict) and upstream_job.get("type") in VERDICT_JOB_TYPES:
            gate_json["requires_verdict"] = ["accept", "accept_with_findings"]
        with ctx.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.job_dependencies (
                  repository_id, job_id, depends_on_job_id, gate_json
                )
                VALUES (%s, %s, %s, %s)
                ON CONFLICT (repository_id, job_id, depends_on_job_id) DO NOTHING
                """,
                (ctx.repository_id, to_job_id, from_job_id, Jsonb(gate_json)),
            )

    ctx.append_event(
        run_id=run_id,
        event_type="run.prepared",
        payload={
            "workflow_id": workflow_id,
            "workflow_snapshot_id": workflow_snapshot_id,
            "source_path": source_path,
        },
    )
    if state == "ready":
        ctx.append_event(
            run_id=run_id,
            event_type="run.branch_confirmed",
            payload={"branch": suggested_branch, "mode": "auto", "created": False},
        )

    return {
        "run_id": run_id,
        "state": state,
        "branch_mode": branch_mode,
        "suggested_branch_name": suggested_branch,
        "branch": {
            "name": suggested_branch,
            "mode": branch_mode,
            "confirmed": state == "ready",
        },
    }


def _repo_root_for(ctx: RepoHandlerContext) -> Path:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT repo_root
            FROM striatumd.repositories
            WHERE repository_id = %s AND state = 'active'
            """,
            (ctx.repository_id,),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("repo_not_registered", "daemon RPC repository is not registered")
    return Path(str(row["repo_root"])).expanduser().resolve()


def _resolve_workflow_path(repo_root: Path, raw_path: str) -> tuple[Path, str]:
    candidate = Path(raw_path).expanduser()
    resolved = candidate.resolve() if candidate.is_absolute() else (repo_root / candidate).resolve()
    try:
        source_path = resolved.relative_to(repo_root).as_posix()
    except ValueError as exc:
        raise WorkflowError("workflow path must stay inside the repository") from exc
    return resolved, source_path


def _load_workflow(path: Path) -> JsonObject:
    try:
        return load_workflow(path)
    except OSError as exc:
        raise WorkflowError(f"read workflow: {exc}") from exc


def _branch_section(workflow: JsonObject) -> dict[str, Any]:
    raw = workflow.get("branch")
    if not isinstance(raw, dict):
        raise WorkflowError("workflow branch must be an object")
    return cast(dict[str, Any], raw)


def _stored_job_type(job: Mapping[str, Any], workflow_job_id: str) -> str:
    raw = job.get("type", "generic")
    job_type = "generic" if raw is None else str(raw)
    if job_type == "phase_synthesis":
        return "review"
    if job_type not in _PG_JOB_TYPES:
        raise WorkflowError(f"job {workflow_job_id!r} declares unsupported type {job_type!r}")
    return job_type


def _max_attempts(job: Mapping[str, Any], workflow_job_id: str) -> int:
    raw = job.get("max_attempts", 1)
    if isinstance(raw, bool) or not isinstance(raw, int) or raw < 1:
        raise WorkflowError(f"job {workflow_job_id!r} max_attempts must be an integer >= 1")
    return raw


def _job_text(job: Mapping[str, Any], key: str) -> str:
    value = job.get(key)
    if not isinstance(value, str) or value == "":
        raise WorkflowError(f"workflow job field {key!r} must be a non-empty string")
    return value


def _optional_text(value: object) -> str | None:
    if value is None:
        return None
    text = str(value)
    return text if text else None


def _required_text(params: Mapping[str, Any], key: str, *, method: str) -> str:
    value = str(params.get(key) or "")
    if value == "":
        raise RpcError("schema_invalid", f"{method} requires {key}")
    return value

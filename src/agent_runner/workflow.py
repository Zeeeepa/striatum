"""Workflow JSON validation and run loading."""

from __future__ import annotations

import json
import sqlite3
from pathlib import Path
from typing import Any, cast

from agent_runner.db import JsonObject, insert_event, json_dumps, new_id, sha256_bytes, utc_now
from agent_runner.errors import WorkflowError

# JSON workflow files are user-authored and need dynamic validation.
JsonValue = dict[str, Any]

REQUIRED_TOP_LEVEL = {
    "schema_version",
    "workflow_id",
    "workflow_version",
    "name",
    "branch",
    "coordinator",
    "lanes",
    "roles",
    "context_docs",
    "parallelism",
    "jobs",
    "edges",
    "cycles",
}


def load_workflow(path: Path) -> JsonObject:
    """Load and validate a workflow JSON file."""
    if path.suffix.lower() in {".yaml", ".yml"}:
        raise WorkflowError("workflow config must be JSON, not YAML")
    raw = path.read_text(encoding="utf-8")
    if raw.lstrip()[:1] != "{":
        raise WorkflowError("workflow config must be a JSON object")
    try:
        loaded = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise WorkflowError(f"workflow JSON is invalid: {exc.msg}") from exc
    if not isinstance(loaded, dict):
        raise WorkflowError("workflow config must be a JSON object")
    workflow = cast(JsonObject, loaded)
    validate_workflow(workflow)
    return workflow


def validate_workflow(workflow: JsonObject) -> None:
    """Validate the V1 workflow shape."""
    missing = sorted(REQUIRED_TOP_LEVEL.difference(workflow))
    if missing:
        raise WorkflowError(f"workflow is missing required fields: {', '.join(missing)}")
    if workflow.get("schema_version") != "agent-runner.workflow.v1":
        raise WorkflowError("workflow schema_version must be agent-runner.workflow.v1")
    lanes = _object(workflow, "lanes")
    roles = _object(workflow, "roles")
    jobs = _list(workflow, "jobs")
    job_map: dict[str, JsonValue] = {}
    for job_value in jobs:
        if not isinstance(job_value, dict):
            raise WorkflowError("each job must be an object")
        job = cast(JsonValue, job_value)
        job_id = _string(job, "id")
        if job_id in job_map:
            raise WorkflowError(f"duplicate job id {job_id!r}")
        job_map[job_id] = job
        role_id = _string(job, "role_id")
        if role_id not in roles:
            raise WorkflowError(f"job {job_id!r} references unknown role {role_id!r}")
        lane_id = job.get("lane_id")
        if lane_id is not None and lane_id not in lanes:
            raise WorkflowError(f"job {job_id!r} references unknown lane {lane_id!r}")
        for dep in job.get("needs", []):
            if not isinstance(dep, str):
                raise WorkflowError(f"job {job_id!r} has non-string dependency")
        for artifact in job.get("expected_artifacts", []):
            if not isinstance(artifact, dict):
                raise WorkflowError(f"job {job_id!r} expected artifact must be an object")
            path = artifact.get("path")
            if not isinstance(path, str) or path.startswith("/") or ".." in Path(path).parts:
                raise WorkflowError(f"job {job_id!r} has invalid artifact path")
    edge_dependency_pairs(workflow)
    validate_needs_match_edges(workflow)
    for cycle_value in _list(workflow, "cycles"):
        if not isinstance(cycle_value, dict):
            raise WorkflowError("each cycle must be an object")
        cycle = cast(JsonValue, cycle_value)
        from_id = _string(cycle, "from")
        to_id = _string(cycle, "to")
        if from_id not in job_map or to_id not in job_map:
            raise WorkflowError("workflow cycle references an unknown job")
        if _string(cycle, "on_verdict") != "needs_revision":
            raise WorkflowError("workflow cycles must use on_verdict needs_revision")
        max_iterations = cycle.get("max_iterations")
        if not isinstance(max_iterations, int) or max_iterations < 1:
            raise WorkflowError("workflow cycles must declare max_iterations >= 1")
    _validate_parallelism(jobs)


def create_run(conn: sqlite3.Connection, *, repo: Path, workflow_path: Path) -> JsonObject:
    """Snapshot workflow JSON and create a prepared run."""
    workflow = load_workflow(workflow_path)
    now = utc_now()
    raw_json = json_dumps(workflow)
    workflow_snapshot_id = new_id("wfs")
    run_id = new_id("run")
    conn.execute(
        """
        INSERT INTO workflow_snapshots (
          workflow_snapshot_id, workflow_id, workflow_version, source_path,
          content_sha256, workflow_json, loaded_at
        )
        VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
        (
            workflow_snapshot_id,
            workflow["workflow_id"],
            workflow.get("workflow_version"),
            str(workflow_path),
            sha256_bytes(raw_json.encode("utf-8")),
            raw_json,
            now,
        ),
    )
    conn.execute(
        """
        INSERT INTO runs (
          run_id, workflow_snapshot_id, repo_root, state, branch_name,
          branch_base, created_at
        )
        VALUES (?, ?, ?, 'needs_branch_confirmation', ?, ?, ?)
        """,
        (
            run_id,
            workflow_snapshot_id,
            str(repo),
            _object(workflow, "branch").get("suggested_name"),
            None,
            now,
        ),
    )
    workflow_jobs = workflow_job_map(workflow)
    job_map: dict[str, str] = {}
    for job_value in _list(workflow, "jobs"):
        job = cast(JsonValue, job_value)
        workflow_job_id = _string(job, "id")
        job_id = f"job_{run_id}_{workflow_job_id}"
        job_map[workflow_job_id] = job_id
        lane_id = job.get("lane_id")
        conn.execute(
            """
            INSERT INTO jobs (
              job_id, run_id, workflow_job_id, title, job_type, role_id,
              lane_selector_json, capability_requirements_json, state, max_attempts,
              fresh_session_required, write_scope_json, expected_artifacts_json,
              idempotency_key, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'blocked', ?, ?, ?, ?, ?, ?)
            """,
            (
                job_id,
                run_id,
                workflow_job_id,
                job.get("title", workflow_job_id),
                job.get("type", "generic"),
                job["role_id"],
                json_dumps({"lane_id": lane_id} if lane_id is not None else {}),
                json_dumps(
                    {
                        "objective": job.get("objective"),
                        "task_prompt": job.get("task_prompt", {}),
                        "inputs": job.get("inputs", []),
                    }
                ),
                int(job.get("max_attempts", 1)),
                1 if job.get("fresh_session_required") is True else 0,
                json_dumps(job.get("write_scope", {})),
                json_dumps(job.get("expected_artifacts", [])),
                f"{run_id}:{workflow_job_id}:1",
                now,
            ),
        )
    for upstream_id, downstream_id, gate in edge_dependency_pairs(workflow):
        upstream_job = workflow_jobs[upstream_id]
        gate_json = dict(gate)
        if upstream_job.get("type") == "review":
            gate_json["requires_verdict"] = ["accept", "accept_with_findings"]
        conn.execute(
            """
            INSERT OR IGNORE INTO job_dependencies(job_id, depends_on_job_id, gate_json)
            VALUES (?, ?, ?)
            """,
            (job_map[downstream_id], job_map[upstream_id], json_dumps(gate_json)),
        )
    insert_event(
        conn,
        run_id=run_id,
        event_type="run.created",
        payload={"workflow_id": workflow["workflow_id"], "workflow_snapshot_id": workflow_snapshot_id},
    )
    return {"run_id": run_id, "state": "needs_branch_confirmation"}


def workflow_job_map(workflow: JsonObject) -> dict[str, JsonValue]:
    """Return jobs keyed by workflow job id."""
    result: dict[str, JsonValue] = {}
    for job_value in _list(workflow, "jobs"):
        if not isinstance(job_value, dict):
            raise WorkflowError("each job must be an object")
        job = cast(JsonValue, job_value)
        result[_string(job, "id")] = job
    return result


def edge_dependency_pairs(workflow: JsonObject) -> list[tuple[str, str, JsonObject]]:
    """Return normalized dependency pairs from top-level edges."""
    jobs = workflow_job_map(workflow)
    pairs: list[tuple[str, str, JsonObject]] = []
    for edge_value in _list(workflow, "edges"):
        if not isinstance(edge_value, dict):
            raise WorkflowError("each edge must be an object")
        edge = cast(JsonValue, edge_value)
        from_id = _string(edge, "from")
        to_id = _string(edge, "to")
        if from_id not in jobs or to_id not in jobs:
            raise WorkflowError("workflow edge references an unknown job")
        if edge.get("on") != "completed":
            raise WorkflowError("workflow edges must use on completed")
        pairs.append((from_id, to_id, {"on": "completed", "from": from_id, "to": to_id}))
    return pairs


def validate_needs_match_edges(workflow: JsonObject) -> None:
    """Reject workflows where legacy needs diverge from authoritative edges."""
    edge_needs: dict[str, set[str]] = {}
    for from_id, to_id, _gate in edge_dependency_pairs(workflow):
        edge_needs.setdefault(to_id, set()).add(from_id)
    for job_id, job in workflow_job_map(workflow).items():
        needs = job.get("needs")
        if needs is None:
            continue
        if not isinstance(needs, list):
            raise WorkflowError(f"job {job_id!r} needs must be a list")
        declared = set()
        for dep in needs:
            if not isinstance(dep, str):
                raise WorkflowError(f"job {job_id!r} has non-string dependency")
            declared.add(dep)
        if declared != edge_needs.get(job_id, set()):
            raise WorkflowError(f"job {job_id!r} needs disagree with workflow edges")


def _validate_parallelism(jobs: list[object]) -> None:
    groups: dict[str, list[JsonValue]] = {}
    for job_value in jobs:
        if not isinstance(job_value, dict):
            continue
        job = cast(JsonValue, job_value)
        group = job.get("parallel_group")
        if isinstance(group, str):
            groups.setdefault(group, []).append(job)
    for group, members in groups.items():
        artifact_paths: set[str] = set()
        write_paths: set[str] = set()
        for job in members:
            for artifact in job.get("expected_artifacts", []):
                if not isinstance(artifact, dict):
                    continue
                path = artifact.get("path")
                if not isinstance(path, str):
                    continue
                if path in artifact_paths:
                    raise WorkflowError(f"parallel group {group!r} reuses artifact path {path!r}")
                artifact_paths.add(path)
            scope = job.get("write_scope", {})
            if not isinstance(scope, dict) or scope.get("repo_write") is not True:
                continue
            for allowed in scope.get("allowed_paths", []):
                if not isinstance(allowed, str):
                    continue
                if allowed in write_paths:
                    raise WorkflowError(f"parallel group {group!r} has overlapping write scope")
                write_paths.add(allowed)


def _object(value: JsonObject, key: str) -> JsonObject:
    item = value.get(key)
    if not isinstance(item, dict):
        raise WorkflowError(f"workflow field {key!r} must be an object")
    return cast(JsonObject, item)


def _list(value: JsonObject, key: str) -> list[object]:
    item = value.get(key)
    if not isinstance(item, list):
        raise WorkflowError(f"workflow field {key!r} must be a list")
    return item


def _string(value: JsonValue, key: str) -> str:
    item = value.get(key)
    if not isinstance(item, str) or item == "":
        raise WorkflowError(f"workflow field {key!r} must be a non-empty string")
    return item

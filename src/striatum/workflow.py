"""Workflow JSON validation and run loading."""

from __future__ import annotations

import json
import sqlite3
import sys
from pathlib import Path, PurePosixPath
from typing import Any, cast

from striatum.db import (
    ADAPTER_ENFORCEMENT_LEVELS,
    JsonObject,
    adapter_constraint_enforcement,
    adapter_enforcement_satisfies,
    insert_event,
    json_dumps,
    new_id,
    sha256_bytes,
    utc_now,
)
from striatum.errors import WorkflowError

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

CONSTRAINT_VALUES = {
    "network": {"allowed", "forbidden", "advisory_forbidden"},
    "transcripts": {"off", "redacted", "allowed"},
    "repo_scope": {"local_only", "unrestricted"},
}

WORKTREE_ISOLATION_VALUES = {"off", "per_job"}


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


def plan_workflow(workflow: JsonObject) -> JsonObject:
    """Return a dry-run execution plan for an already validated workflow."""
    validate_workflow(workflow)
    jobs = workflow_job_map(workflow)
    edges = edge_dependency_pairs(workflow)
    downstream: dict[str, list[str]] = {job_id: [] for job_id in jobs}
    indegree: dict[str, int] = {job_id: 0 for job_id in jobs}
    for from_id, to_id, _gate in edges:
        downstream[from_id].append(to_id)
        indegree[to_id] += 1

    ready = sorted(job_id for job_id, count in indegree.items() if count == 0)
    claim_order: list[JsonObject] = []
    visited: set[str] = set()
    step = 1
    while ready:
        wave = ready
        ready = []
        claim_order.append(
            {
                "step": step,
                "claimable": [_planned_job(jobs[job_id]) for job_id in wave],
            }
        )
        step += 1
        for job_id in wave:
            visited.add(job_id)
            for downstream_id in sorted(downstream[job_id]):
                indegree[downstream_id] -= 1
                if indegree[downstream_id] == 0:
                    ready.append(downstream_id)
        ready.sort()

    if len(visited) != len(jobs):
        remaining = sorted(set(jobs).difference(visited))
        raise WorkflowError(f"workflow edges contain a dependency cycle involving: {', '.join(remaining)}")

    return {
        "workflow_id": workflow["workflow_id"],
        "workflow_version": workflow.get("workflow_version"),
        "valid": True,
        "summary": {
            "jobs": len(jobs),
            "edges": len(edges),
            "cycles": len(_list(workflow, "cycles")),
            "claim_steps": len(claim_order),
        },
        "claim_order": claim_order,
        "review_gates": _planned_review_gates(workflow, jobs=jobs, edges=edges),
        "cycles": _planned_cycles(workflow),
        "graph": workflow_graph_data(workflow)["graph"],
    }


def workflow_graph_data(workflow: JsonObject) -> JsonObject:
    """Return workflow graph data for authoring and visualization."""
    validate_workflow(workflow)
    jobs = workflow_job_map(workflow)
    edges = edge_dependency_pairs(workflow)
    return {
        "workflow_id": workflow["workflow_id"],
        "workflow_version": workflow.get("workflow_version"),
        "graph": {
            "nodes": [_planned_job(job) for job in jobs.values()],
            "edges": [_planned_edge(jobs, from_id=from_id, to_id=to_id) for from_id, to_id, _gate in edges],
            "cycles": _planned_cycles(workflow),
        },
    }


def workflow_graph_mermaid(
    workflow: JsonObject,
    *,
    node_states: dict[str, str] | None = None,
) -> str:
    """Return a Mermaid flowchart for a workflow graph.

    When ``node_states`` is supplied, append Mermaid ``classDef`` lines and
    per-node ``class`` assignments so a renderer can highlight current job
    states. Keys are workflow job ids; values are state strings (e.g. the
    ``jobs.state`` column or the ``pending`` sentinel for jobs that have no
    row yet).
    """
    graph_data = workflow_graph_data(workflow)
    graph = cast(JsonObject, graph_data["graph"])
    nodes = cast(list[JsonObject], graph["nodes"])
    edges = cast(list[JsonObject], graph["edges"])
    cycles = cast(list[JsonObject], graph["cycles"])

    node_names = {str(node["job_id"]): f"n{index}" for index, node in enumerate(nodes)}
    parallel_groups: dict[str, list[JsonObject]] = {}
    ungrouped: list[JsonObject] = []
    for node in nodes:
        group = node.get("parallel_group")
        if isinstance(group, str) and group != "":
            parallel_groups.setdefault(group, []).append(node)
        else:
            ungrouped.append(node)

    lines = ["flowchart TD"]
    for node in ungrouped:
        lines.append(_mermaid_node_line(node, node_names=node_names, indent="  "))
    for group_index, group_id in enumerate(sorted(parallel_groups)):
        lines.append(f'  subgraph pg{group_index}["parallel: {_mermaid_label(group_id)}"]')
        for node in sorted(parallel_groups[group_id], key=lambda item: str(item["job_id"])):
            lines.append(_mermaid_node_line(node, node_names=node_names, indent="    "))
        lines.append("  end")
    for edge in edges:
        from_id = str(edge["from"])
        to_id = str(edge["to"])
        gate = edge.get("gate")
        label = "completed"
        if isinstance(gate, dict) and "requires_verdict" in gate:
            label = "accepted review"
        lines.append(f"  {node_names[from_id]} -->|{label}| {node_names[to_id]}")
    for cycle in cycles:
        from_id = str(cycle["from"])
        to_id = str(cycle["to"])
        max_iterations = cycle.get("max_iterations")
        label = f"needs_revision max {max_iterations}"
        lines.append(f"  {node_names[from_id]} -.->|{label}| {node_names[to_id]}")
    if node_states is not None:
        for class_name, fill in MERMAID_STATE_FILLS.items():
            lines.append(f"  classDef {class_name} fill:{fill}")
        for node in nodes:
            job_id = str(node["job_id"])
            state = node_states.get(job_id, "pending")
            class_name = mermaid_state_class(state)
            lines.append(f"  class {node_names[job_id]} {class_name}")
    return "\n".join(lines) + "\n"


# Mermaid class palette for stateful graph highlighting. Keep deterministic
# (insertion order) so generated Mermaid output is stable across runs.
MERMAID_STATE_FILLS: dict[str, str] = {
    "state-completed": "#c8e6c9",
    "state-running": "#bbdefb",
    "state-claimed": "#bbdefb",
    "state-acked": "#bbdefb",
    "state-blocked": "#fff59d",
    "state-stale_lease": "#fff59d",
    "state-waiting_human": "#fff59d",
    "state-failed": "#ffcdd2",
    "state-canceled": "#ffcdd2",
    "state-queued": "#e0e0e0",
    "state-pending": "#f5f5f5",
}


def mermaid_state_class(state: str) -> str:
    """Map a job state (or ``pending`` sentinel) to a Mermaid class name."""
    if state == "skipped":
        # Skipped jobs are terminal but not a success; group with canceled.
        return "state-canceled"
    candidate = f"state-{state}"
    if candidate in MERMAID_STATE_FILLS:
        return candidate
    return "state-pending"


def validate_workflow(workflow: JsonObject) -> None:
    """Validate the V1 workflow shape."""
    missing = sorted(REQUIRED_TOP_LEVEL.difference(workflow))
    if missing:
        raise WorkflowError(f"workflow is missing required fields: {', '.join(missing)}")
    if workflow.get("schema_version") != "striatum.workflow.v1":
        raise WorkflowError("workflow schema_version must be striatum.workflow.v1")
    lanes = _object(workflow, "lanes")
    _validate_lane_constraints(lanes)
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
        _validate_write_scope_paths(job_id, job)
        for artifact in job.get("expected_artifacts", []):
            if not isinstance(artifact, dict):
                raise WorkflowError(f"job {job_id!r} expected artifact must be an object")
            path = artifact.get("path")
            if not isinstance(path, str) or path.startswith("/") or ".." in Path(path).parts:
                raise WorkflowError(f"job {job_id!r} has invalid artifact path")
            _validate_artifact_in_write_scope(job_id, job, path)
    _validate_artifact_path_uniqueness(jobs)
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
    _validate_cycle_targets_feed_sources(workflow, job_map=job_map)
    _validate_parallelism(jobs)
    _validate_revision_policy(workflow, jobs=jobs)


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
    deprecated_jobs: list[str] = []
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
        deprecated_jobs.append(job_id)
    if deprecated_jobs:
        names = ", ".join(sorted(deprecated_jobs))
        print(
            "warning: workflow uses deprecated 'needs' field on jobs: "
            f"{names}. 'edges' is authoritative; remove 'needs' to silence this warning.",
            file=sys.stderr,
        )


def _planned_job(job: JsonValue) -> JsonObject:
    lane_id = job.get("lane_id")
    parallel_group = job.get("parallel_group")
    artifacts = []
    for artifact in job.get("expected_artifacts", []):
        if isinstance(artifact, dict):
            artifacts.append(
                {
                    "logical_name": artifact.get("logical_name"),
                    "kind": artifact.get("kind"),
                    "path": artifact.get("path"),
                    "required": artifact.get("required") is True,
                }
            )
    return {
        "job_id": _string(job, "id"),
        "type": job.get("type", "generic"),
        "role_id": _string(job, "role_id"),
        "lane_id": lane_id if isinstance(lane_id, str) else None,
        "parallel_group": parallel_group if isinstance(parallel_group, str) else None,
        "fresh_session_required": job.get("fresh_session_required") is True,
        "write_scope_mode": _write_scope_mode(job),
        "expected_artifacts": artifacts,
    }


def _planned_edge(jobs: dict[str, JsonValue], *, from_id: str, to_id: str) -> JsonObject:
    gate: JsonObject = {"on": "completed"}
    if jobs[from_id].get("type") == "review":
        gate["requires_verdict"] = ["accept", "accept_with_findings"]
    return {"from": from_id, "to": to_id, "gate": gate}


def _planned_review_gates(
    workflow: JsonObject,
    *,
    jobs: dict[str, JsonValue],
    edges: list[tuple[str, str, JsonObject]],
) -> list[JsonObject]:
    cycle_by_review = {
        str(cycle["from"]): cycle
        for cycle in _planned_cycles(workflow)
        if isinstance(cycle.get("from"), str)
    }
    downstream_by_review: dict[str, list[str]] = {}
    for from_id, to_id, _gate in edges:
        if jobs[from_id].get("type") == "review":
            downstream_by_review.setdefault(from_id, []).append(to_id)

    policy = workflow.get("review_revision_policy")
    root_policy = policy.get("root_review_needs_revision") if isinstance(policy, dict) else None
    gates: list[JsonObject] = []
    for job_id, job in jobs.items():
        if job.get("type") != "review":
            continue
        revision_cycle = cycle_by_review.get(job_id)
        if revision_cycle is not None:
            needs_revision: JsonObject = {
                "action": "cycle",
                "to": revision_cycle["to"],
                "max_iterations": revision_cycle["max_iterations"],
            }
        elif root_policy == "human_checkpoint":
            needs_revision = {"action": "human_checkpoint"}
        else:
            needs_revision = {"action": "no_declared_route"}
        gates.append(
            {
                "review_job_id": job_id,
                "downstream_jobs": sorted(downstream_by_review.get(job_id, [])),
                "accepting_verdicts": ["accept", "accept_with_findings"],
                "needs_revision": needs_revision,
                "reject": {"action": "fail_review"},
            }
        )
    return sorted(gates, key=lambda gate: str(gate["review_job_id"]))


def _planned_cycles(workflow: JsonObject) -> list[JsonObject]:
    cycles: list[JsonObject] = []
    for cycle_value in _list(workflow, "cycles"):
        cycle = cast(JsonValue, cycle_value)
        cycles.append(
            {
                "from": _string(cycle, "from"),
                "to": _string(cycle, "to"),
                "on_verdict": _string(cycle, "on_verdict"),
                "max_iterations": cycle["max_iterations"],
            }
        )
    return cycles


def _write_scope_mode(job: JsonValue) -> str | None:
    write_scope = job.get("write_scope")
    if not isinstance(write_scope, dict):
        return None
    mode = write_scope.get("mode")
    return mode if isinstance(mode, str) else None


def _mermaid_node_line(node: JsonObject, *, node_names: dict[str, str], indent: str) -> str:
    job_id = str(node["job_id"])
    type_text = str(node.get("type", "generic"))
    role_id = str(node.get("role_id", ""))
    lane_id = node.get("lane_id")
    lane_text = f"/{lane_id}" if isinstance(lane_id, str) and lane_id != "" else ""
    label = _mermaid_label(f"{job_id}<br/>{type_text} {role_id}{lane_text}")
    return f'{indent}{node_names[job_id]}["{label}"]'


def _mermaid_label(value: str) -> str:
    return value.replace("\\", "\\\\").replace('"', '\\"')


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
        repo_write_modes: set[bool] = set()
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
            if not isinstance(scope, dict):
                continue
            repo_write_modes.add(scope.get("repo_write") is True)
            if scope.get("repo_write") is not True:
                continue
            for allowed in scope.get("allowed_paths", []):
                if not isinstance(allowed, str):
                    continue
                if allowed in write_paths:
                    raise WorkflowError(f"parallel group {group!r} has overlapping write scope")
                write_paths.add(allowed)
        if len(repo_write_modes) > 1:
            raise WorkflowError(
                f"parallel group {group!r} mixes repo_write and review-only jobs; "
                "split them into separate groups"
            )


def _validate_lane_constraints(lanes: JsonObject) -> None:
    """Validate optional lane adapter constraints."""
    for lane_id, lane_value in lanes.items():
        if not isinstance(lane_value, dict):
            raise WorkflowError(f"lane {lane_id!r} must be an object")
        if lane_value.get("adapter") == "process":
            command = lane_value.get("command")
            if not isinstance(command, list) or not command:
                raise WorkflowError(f"process lane {lane_id!r} command must be a non-empty array")
            if not all(isinstance(part, str) and part != "" for part in command):
                raise WorkflowError(f"process lane {lane_id!r} command entries must be non-empty strings")
            env = lane_value.get("env")
            if env is not None and (
                not isinstance(env, dict)
                or not all(isinstance(key, str) and isinstance(value, str) for key, value in env.items())
            ):
                raise WorkflowError(f"process lane {lane_id!r} env must be an object of strings")
        worktree_value = lane_value.get("worktree_isolation")
        if worktree_value is not None:
            if not isinstance(worktree_value, str) or worktree_value not in WORKTREE_ISOLATION_VALUES:
                raise WorkflowError(
                    f"lane {lane_id!r} worktree_isolation must be one of "
                    f"{sorted(WORKTREE_ISOLATION_VALUES)!r}"
                )
        constraints_value = lane_value.get("constraints")
        if constraints_value is None:
            constraints: dict[str, object] = {}
        elif not isinstance(constraints_value, dict):
            raise WorkflowError(f"lane {lane_id!r} constraints must be an object")
        else:
            constraints = constraints_value
        for key, value in constraints.items():
            if key not in CONSTRAINT_VALUES:
                raise WorkflowError(f"lane {lane_id!r} has unknown constraint {key!r}")
            if value not in CONSTRAINT_VALUES[key]:
                raise WorkflowError(f"lane {lane_id!r} has invalid {key!r} constraint value")
        required = lane_value.get("required_enforcement")
        if required is None:
            continue
        if not isinstance(required, dict):
            raise WorkflowError(f"lane {lane_id!r} required_enforcement must be an object")
        for key, value in required.items():
            if key not in constraints:
                raise WorkflowError(
                    f"lane {lane_id!r} requires enforcement for undeclared constraint {key!r}"
                )
            if value not in ADAPTER_ENFORCEMENT_LEVELS:
                raise WorkflowError(f"lane {lane_id!r} has invalid required enforcement level")
            constraint_value = constraints[key]
            if not isinstance(constraint_value, str) or not isinstance(value, str):
                raise WorkflowError(f"lane {lane_id!r} required_enforcement entries must be strings")
            actual = adapter_constraint_enforcement(
                lane_value.get("adapter"),
                constraint=str(key),
                requested=constraint_value,
            )
            if not adapter_enforcement_satisfies(actual=actual, required=value):
                raise WorkflowError(
                    f"lane {lane_id!r} requires {value!r} enforcement for {key!r}, "
                    f"but adapter provides {actual!r}"
                )


def _validate_revision_policy(workflow: JsonObject, *, jobs: list[object]) -> None:
    """Validate optional explicit review revision policy."""
    policy = workflow.get("review_revision_policy")
    if policy is None:
        return
    if not isinstance(policy, dict):
        raise WorkflowError("review_revision_policy must be an object")
    root_policy = policy.get("root_review_needs_revision")
    if root_policy not in {"human_checkpoint", "declared_cycle"}:
        raise WorkflowError("review_revision_policy.root_review_needs_revision is invalid")
    description = policy.get("description")
    if description is not None and not isinstance(description, str):
        raise WorkflowError("review_revision_policy.description must be a string")
    if root_policy != "declared_cycle":
        return
    cycle_sources = {
        cycle.get("from")
        for cycle in workflow.get("cycles", [])
        if isinstance(cycle, dict) and cycle.get("on_verdict") == "needs_revision"
    }
    missing = sorted(_root_review_job_ids(workflow, jobs=jobs).difference(cycle_sources))
    if missing:
        raise WorkflowError(
            "declared_cycle review_revision_policy requires needs_revision cycles "
            f"for root review jobs: {', '.join(missing)}"
        )


def _root_review_job_ids(workflow: JsonObject, *, jobs: list[object]) -> set[str]:
    """Return review job ids that have no upstream workflow dependency."""
    dependency_targets = {
        edge.get("to")
        for edge in workflow.get("edges", [])
        if isinstance(edge, dict) and isinstance(edge.get("to"), str)
    }
    root_review_ids: set[str] = set()
    for job_value in jobs:
        if not isinstance(job_value, dict):
            continue
        job = cast(JsonValue, job_value)
        job_id = job.get("id")
        if job.get("type") == "review" and isinstance(job_id, str) and job_id not in dependency_targets:
            root_review_ids.add(job_id)
    return root_review_ids


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


def _validate_artifact_path_uniqueness(jobs: list[object]) -> None:
    """Reject workflows where two jobs declare the same expected artifact path."""
    seen: dict[str, str] = {}
    for job_value in jobs:
        if not isinstance(job_value, dict):
            continue
        job = cast(JsonValue, job_value)
        job_id = job.get("id")
        if not isinstance(job_id, str):
            continue
        for artifact in job.get("expected_artifacts", []):
            if not isinstance(artifact, dict):
                continue
            path = artifact.get("path")
            if not isinstance(path, str):
                continue
            normalized = _normalize_path_string(path)
            if normalized in seen and seen[normalized] != job_id:
                raise WorkflowError(
                    f"jobs {seen[normalized]!r} and {job_id!r} both declare expected "
                    f"artifact path {path!r}"
                )
            seen.setdefault(normalized, job_id)


def _validate_write_scope_paths(job_id: str, job: JsonValue) -> None:
    """Reject write_scopes where allowed_paths overlap forbidden_paths."""
    scope = job.get("write_scope")
    if not isinstance(scope, dict):
        return
    allowed_paths = scope.get("allowed_paths", [])
    forbidden_paths = scope.get("forbidden_paths", [])
    if not isinstance(allowed_paths, list) or not isinstance(forbidden_paths, list):
        return
    for allowed in allowed_paths:
        if not isinstance(allowed, str):
            continue
        for forbidden in forbidden_paths:
            if not isinstance(forbidden, str):
                continue
            if _path_within(allowed, forbidden):
                raise WorkflowError(
                    f"job {job_id!r} write_scope allowed_path {allowed!r} is inside "
                    f"forbidden_path {forbidden!r}"
                )


def _validate_artifact_in_write_scope(job_id: str, job: JsonValue, artifact_path: str) -> None:
    """Reject expected artifact paths that are not inside the job's write_scope."""
    scope = job.get("write_scope")
    if not isinstance(scope, dict):
        return
    allowed_paths = scope.get("allowed_paths", [])
    forbidden_paths = scope.get("forbidden_paths", [])
    if not isinstance(allowed_paths, list) or not isinstance(forbidden_paths, list):
        return
    if not allowed_paths:
        return
    for forbidden in forbidden_paths:
        if not isinstance(forbidden, str):
            continue
        if _path_within(artifact_path, forbidden):
            raise WorkflowError(
                f"job {job_id!r} expected artifact {artifact_path!r} is inside "
                f"forbidden_path {forbidden!r}"
            )
    inside_any = False
    for allowed in allowed_paths:
        if not isinstance(allowed, str):
            continue
        if _path_within(artifact_path, allowed):
            inside_any = True
            break
    if not inside_any:
        raise WorkflowError(
            f"job {job_id!r} expected artifact {artifact_path!r} is not inside "
            f"any allowed_path"
        )


def _validate_cycle_targets_feed_sources(workflow: JsonObject, *, job_map: dict[str, JsonValue]) -> None:
    """Reject cycles whose target does not feed back into the cycle source via edges."""
    edges: list[tuple[str, str]] = []
    for edge in workflow.get("edges", []):
        if not isinstance(edge, dict):
            continue
        from_id = edge.get("from")
        to_id = edge.get("to")
        if isinstance(from_id, str) and isinstance(to_id, str):
            edges.append((from_id, to_id))
    for cycle_value in workflow.get("cycles", []):
        if not isinstance(cycle_value, dict):
            continue
        from_id = cycle_value.get("from")
        to_id = cycle_value.get("to")
        if not isinstance(from_id, str) or not isinstance(to_id, str):
            continue
        if from_id not in job_map or to_id not in job_map:
            continue
        if not _has_path(edges, source=to_id, target=from_id):
            raise WorkflowError(
                f"workflow cycle from {from_id!r} to {to_id!r} is unsound: "
                f"{to_id!r} does not feed back into {from_id!r} through workflow edges"
            )


def _has_path(edges: list[tuple[str, str]], *, source: str, target: str) -> bool:
    """Return True if the edge graph has a directed path from source to target."""
    if source == target:
        return True
    adjacency: dict[str, list[str]] = {}
    for from_id, to_id in edges:
        adjacency.setdefault(from_id, []).append(to_id)
    visited: set[str] = set()
    stack: list[str] = [source]
    while stack:
        current = stack.pop()
        if current == target:
            return True
        if current in visited:
            continue
        visited.add(current)
        stack.extend(adjacency.get(current, []))
    return False


def _normalize_path_string(path_text: str) -> str:
    """Normalize a repo-relative path string for stable comparison."""
    pure = PurePosixPath(path_text)
    parts: list[str] = []
    for part in pure.parts:
        if part in ("", "."):
            continue
        parts.append(part)
    if not parts:
        return ""
    normalized = "/".join(parts)
    return normalized.rstrip("/")


def _path_within(child: str, parent: str) -> bool:
    """Return True if child path is equal to or inside parent path (string-only)."""
    child_norm = _normalize_path_string(child)
    parent_norm = _normalize_path_string(parent)
    if parent_norm == "":
        return True
    if child_norm == parent_norm:
        return True
    return child_norm.startswith(parent_norm + "/")

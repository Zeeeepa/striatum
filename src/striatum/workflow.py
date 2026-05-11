"""Workflow JSON validation and run loading."""

from __future__ import annotations

import json
import re
import sqlite3
import sys
from pathlib import Path, PurePosixPath
from typing import Any, cast

from striatum.artifacts import ALLOWED_ARTIFACT_KINDS
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
PROVENANCE_MODES = frozenset({"advisory", "attested_bylines", "sealed_patch"})

# Workflow branch.mode values:
# - "auto": run prepare creates the suggested branch and transitions the run
#           directly to state="ready" with no separate branch-confirm step.
#           Requires `suggested_name` to be set.
# - "confirm": run prepare returns state="needs_branch_confirmation"; the
#              operator runs `striatum branch confirm` (with --create / etc.)
#              to advance.
# Default when `branch.mode` is omitted: "auto".
BRANCH_MODE_VALUES = ("auto", "confirm")

REVIEWER_ACCESS_SCOPE_VALUES = ("document_only", "artifact_augmented", "repo_level")
REVIEWER_CONTEXT_POLICY_VALUES = ("fresh", "cross_round")

# RFC 0018 V1: closed set of first-class review postures. Workflows may also
# declare a ``custom:<name>`` posture for off-list adversarial flavors; the
# runner records the literal string and does not normalise.
ALLOWED_POSTURES = frozenset({
    "neutral",
    "devils_advocate",
    "security",
    "threat_model",
    "latency_performance",
    "ergonomics_dx",
    "accessibility",
    "compliance_license",
    "supply_chain",
})

# RFC 0018 V1: deterministic instruction sentence appended to a review job's
# packet ``review_policy.instruction`` for non-neutral first-class postures.
# Custom postures get no auto-appended sentence; the workflow author owns the
# prompt body for off-list flavors.
POSTURE_INSTRUCTIONS: dict[str, str] = {
    "neutral": "",
    "devils_advocate": (
        " This is a devil's-advocate review. Argue against the artifact's "
        "claims; verdict acceptance means the claims survived your strongest "
        "counterarguments."
    ),
    "security": (
        " This is a security-focused review. Read the artifact looking for "
        "security weaknesses; verdict acceptance means you actively looked "
        "and found nothing actionable."
    ),
    "threat_model": (
        " This is a threat-modeling review. Enumerate the trust boundaries "
        "and attack surfaces the artifact introduces; verdict acceptance "
        "means each is acknowledged or mitigated."
    ),
    "latency_performance": (
        " This is a latency / performance review. Evaluate the artifact's "
        "runtime and resource cost; verdict acceptance means no "
        "acceptance-blocking regression was found."
    ),
    "ergonomics_dx": (
        " This is a developer-ergonomics review. Evaluate the artifact's "
        "surface from a first-time-user perspective; verdict acceptance "
        "means the affordances are discoverable and consistent."
    ),
    "accessibility": (
        " This is an accessibility review. Evaluate the artifact against "
        "accessibility expectations; verdict acceptance means the "
        "affordances meet the declared accessibility bar."
    ),
    "compliance_license": (
        " This is a compliance / license review. Evaluate the artifact for "
        "license, attribution, or compliance issues; verdict acceptance "
        "means none are unresolved."
    ),
    "supply_chain": (
        " This is a supply-chain review. Evaluate the artifact's external "
        "dependencies and their provenance; verdict acceptance means each "
        "is justified and pinned."
    ),
}

# RFC 0010 V1: closed set of recognised tool families. Profiles that declare
# any other family are rejected at validation time.
HARNESS_PROFILE_TOOL_FAMILIES = frozenset({
    "generic",
    "codex",
    "claude_code",
    "gemini_cli",
})

# RFC 0010 V1: required-when-declared and known optional fields in a profile
# body. Unknown sibling fields produce a lint warning rather than an error so
# the schema can grow without breaking workflows.
_HARNESS_PROFILE_REQUIRED_FIELDS: frozenset[str] = frozenset({
    "tool_family",
    "strategy_version",
})
_HARNESS_PROFILE_KNOWN_FIELDS: frozenset[str] = frozenset({
    "tool_family",
    "strategy_version",
    "native_delegation",
    "feature_flags",
    "accountability",
    "supervision",
    "workspace_isolation",
    "agent_loop_budget",
    "approval_mode",
    "output_format",
    "memory_files",
    "mcp_servers",
    "turn_caps",
    "prompt_envelope_path",
    "fallback_profile_id",
})


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


def plan_workflow(
    workflow: JsonObject, *, repo_root: Path | None = None
) -> JsonObject:
    """Return a dry-run execution plan for an already validated workflow."""
    warnings: list[str] = []
    validate_workflow(workflow, warnings=warnings, repo_root=repo_root)
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

    plan: JsonObject = {
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
    if warnings:
        plan["warnings"] = warnings
    return plan


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


def workflow_graph_dot(workflow: JsonObject) -> str:
    """Return a Graphviz DOT digraph for a workflow graph.

    Mirrors :func:`workflow_graph_mermaid`'s shape: same nodes, same
    dependency edges, same parallel groups (rendered as ``cluster_*``
    subgraphs), and the same bounded ``needs_revision`` cycle edges
    (rendered as dashed arrows). Output is deterministic — node names
    follow ``n0``, ``n1``, ... in insertion order, and parallel groups
    are emitted in sorted order.
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

    lines = [
        "digraph striatum_workflow {",
        "  rankdir=TB;",
        '  node [shape=box, fontname="Helvetica"];',
    ]
    for node in ungrouped:
        lines.append(_dot_node_line(node, node_names=node_names, indent="  "))
    for group_index, group_id in enumerate(sorted(parallel_groups)):
        cluster_id = _dot_cluster_id(group_id, group_index)
        lines.append(f"  subgraph {cluster_id} {{")
        lines.append(f'    label="parallel: {_dot_label(group_id)}";')
        for node in sorted(parallel_groups[group_id], key=lambda item: str(item["job_id"])):
            lines.append(_dot_node_line(node, node_names=node_names, indent="    "))
        lines.append("  }")
    for edge in edges:
        from_id = str(edge["from"])
        to_id = str(edge["to"])
        gate = edge.get("gate")
        label = "completed"
        if isinstance(gate, dict) and "requires_verdict" in gate:
            label = "accepted review"
        lines.append(
            f'  {node_names[from_id]} -> {node_names[to_id]} [label="{_dot_label(label)}"];'
        )
    for cycle in cycles:
        from_id = str(cycle["from"])
        to_id = str(cycle["to"])
        max_iterations = cycle.get("max_iterations")
        label = f"needs_revision max {max_iterations}"
        lines.append(
            f'  {node_names[from_id]} -> {node_names[to_id]} '
            f'[style=dashed, label="{_dot_label(label)}"];'
        )
    lines.append("}")
    return "\n".join(lines) + "\n"


_DOT_ID_SANITIZER = re.compile(r"[^A-Za-z0-9_]+")


def _dot_cluster_id(group_id: str, index: int) -> str:
    sanitized = _DOT_ID_SANITIZER.sub("_", group_id).strip("_")
    if sanitized == "":
        sanitized = f"pg{index}"
    return f"cluster_{sanitized}"


def _dot_node_line(node: JsonObject, *, node_names: dict[str, str], indent: str) -> str:
    job_id = str(node["job_id"])
    type_text = str(node.get("type", "generic"))
    role_id = str(node.get("role_id", ""))
    lane_id = node.get("lane_id")
    lane_text = f"/{lane_id}" if isinstance(lane_id, str) and lane_id != "" else ""
    line1 = _dot_label(job_id)
    line2 = _dot_label(f"{type_text} {role_id}{lane_text}")
    label = f"{line1}\\n{line2}"
    return f'{indent}{node_names[job_id]} [label="{label}"];'


def _dot_label(value: str) -> str:
    return value.replace("\\", "\\\\").replace('"', '\\"')


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


def compute_node_states(
    conn: sqlite3.Connection, *, run_id: str
) -> dict[str, str]:
    """Return ``{workflow_job_id: current_state}`` for the highest attempt.

    RFC 0016 V1: extracted from ``introspect.run_graph`` so the dashboard
    and the existing graph CLI share a single source of truth for "current
    state after a requeue." Workflow jobs that have not been materialised
    yet are absent from the result; callers fall back to ``"pending"``.
    """
    rows = conn.execute(
        """
        SELECT workflow_job_id, state, attempt
        FROM jobs
        WHERE run_id = ?
        ORDER BY workflow_job_id, attempt DESC
        """,
        (run_id,),
    ).fetchall()
    seen: set[str] = set()
    result: dict[str, str] = {}
    for row in rows:
        wf_id = str(row["workflow_job_id"])
        if wf_id in seen:
            continue
        seen.add(wf_id)
        result[wf_id] = str(row["state"])
    return result


def validate_workflow(
    workflow: JsonObject,
    *,
    warnings: list[str] | None = None,
    repo_root: Path | None = None,
) -> None:
    """Validate the V1 workflow shape.

    When ``warnings`` is provided, advisory lint warnings are appended:

    - RFC 0010: unknown sibling fields on harness profile bodies.
    - RFC 0010 V1.5 / HARNESS-001: repo-relative process-lane command
      paths that do not exist under ``repo_root`` (only fired when
      ``repo_root`` is also provided).

    Hard schema violations always raise ``WorkflowError`` regardless.
    """
    missing = sorted(REQUIRED_TOP_LEVEL.difference(workflow))
    if missing:
        raise WorkflowError(f"workflow is missing required fields: {', '.join(missing)}")
    if workflow.get("schema_version") != "striatum.workflow.v1":
        raise WorkflowError(
            "workflow schema_version must be striatum.workflow.v1",
            field_path="schema_version",
        )
    _validate_provenance_mode(workflow)
    _validate_branch_section(workflow)
    # RFC 0020 V1: optional `recovery_policy` block. Validated here
    # so a workflow that declares an invalid hook is rejected at
    # `workflow validate` time, not at sweep time.
    from striatum.recovery.policy import validate_recovery_policy

    if "recovery_policy" in workflow:
        validate_recovery_policy(
            workflow.get("recovery_policy"),
            workflow_id=str(workflow.get("workflow_id") or ""),
        )
    lanes = _object(workflow, "lanes")
    profile_ids = _validate_harness_profiles(workflow, warnings=warnings)
    _validate_lane_constraints(
        lanes,
        harness_profile_ids=profile_ids,
        repo_root=repo_root,
        warnings=warnings,
    )
    roles = _object(workflow, "roles")
    jobs = _list(workflow, "jobs")
    job_map: dict[str, JsonValue] = {}
    for job_index, job_value in enumerate(jobs):
        if not isinstance(job_value, dict):
            raise WorkflowError(
                "each job must be an object", field_path=f"jobs[{job_index}]"
            )
        job = cast(JsonValue, job_value)
        job_id = _string(job, "id")
        if job_id in job_map:
            raise WorkflowError(
                f"duplicate job id {job_id!r}",
                field_path=f"jobs[{job_index}].id",
            )
        job_map[job_id] = job
        role_id = _string(job, "role_id")
        if role_id not in roles:
            raise WorkflowError(
                f"job {job_id!r} references unknown role {role_id!r}",
                field_path=f"jobs[{job_index}].role_id",
            )
        lane_id = job.get("lane_id")
        if lane_id is not None and lane_id not in lanes:
            raise WorkflowError(
                f"job {job_id!r} references unknown lane {lane_id!r}",
                field_path=f"jobs[{job_index}].lane_id",
            )
        for dep in job.get("needs", []):
            if not isinstance(dep, str):
                raise WorkflowError(f"job {job_id!r} has non-string dependency")
        _validate_write_scope_paths(job_id, job)
        for artifact_index, artifact in enumerate(job.get("expected_artifacts", [])):
            if not isinstance(artifact, dict):
                raise WorkflowError(f"job {job_id!r} expected artifact must be an object")
            path = artifact.get("path")
            if not isinstance(path, str) or path.startswith("/") or ".." in Path(path).parts:
                raise WorkflowError(
                    f"job {job_id!r} has invalid artifact path",
                    field_path=f"jobs[{job_index}].expected_artifacts[{artifact_index}].path",
                )
            kind = artifact.get("kind")
            if isinstance(kind, str) and kind not in ALLOWED_ARTIFACT_KINDS:
                raise WorkflowError(
                    f"job {job_id} declares unknown artifact kind {kind}"
                )
            _validate_artifact_in_write_scope(job_id, job, path)
        _validate_reviewer_policy(job_id, job)
        _validate_review_posture(job_id, job)
        _validate_required_review_postures(job_id, job)
        _validate_require_attested_lane(job_id, job)
    _validate_artifact_path_uniqueness(jobs)
    _validate_required_postures_reachable(workflow, job_map=job_map)
    edge_dependency_pairs(workflow)
    validate_needs_match_edges(workflow)
    for cycle_index, cycle_value in enumerate(_list(workflow, "cycles")):
        if not isinstance(cycle_value, dict):
            raise WorkflowError("each cycle must be an object")
        cycle = cast(JsonValue, cycle_value)
        from_id = _string(cycle, "from")
        to_id = _string(cycle, "to")
        if from_id not in job_map or to_id not in job_map:
            raise WorkflowError(
                "workflow cycle references an unknown job",
                field_path=f"cycles[{cycle_index}].from",
            )
        if _string(cycle, "on_verdict") != "needs_revision":
            raise WorkflowError("workflow cycles must use on_verdict needs_revision")
        max_iterations = cycle.get("max_iterations")
        if not isinstance(max_iterations, int) or max_iterations < 1:
            raise WorkflowError(
                "workflow cycles must declare max_iterations >= 1",
                field_path=f"cycles[{cycle_index}].max_iterations",
            )
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
                1 if _effective_fresh_session_required(job) else 0,
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
    branch_section = _object(workflow, "branch")
    return {
        "run_id": run_id,
        "state": "needs_branch_confirmation",
        "branch_mode": branch_section.get("mode", "auto"),
        "suggested_branch_name": branch_section.get("suggested_name"),
    }


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


def _validate_branch_section(workflow: JsonObject) -> None:
    """Validate the workflow ``branch`` block.

    ``branch.mode`` defaults to ``"auto"`` when omitted. Auto mode requires
    ``suggested_name`` so ``run prepare`` can create the branch atomically.
    """
    raw = workflow.get("branch")
    if raw is None:
        # `branch` is in REQUIRED_TOP_LEVEL; this branch is unreachable, but
        # keep the guard for callers that might construct workflows in-memory.
        return
    if not isinstance(raw, dict):
        raise WorkflowError("workflow branch must be an object")
    mode = raw.get("mode", "auto")
    if mode not in BRANCH_MODE_VALUES:
        raise WorkflowError(
            f"workflow branch.mode must be one of {list(BRANCH_MODE_VALUES)!r}; "
            f"got {mode!r}"
        )
    suggested = raw.get("suggested_name")
    if mode == "auto" and (not isinstance(suggested, str) or not suggested):
        raise WorkflowError(
            "workflow branch.mode 'auto' requires a non-empty suggested_name"
        )
    if suggested is not None and (
        not isinstance(suggested, str) or not suggested
    ):
        raise WorkflowError(
            "workflow branch.suggested_name must be a non-empty string when set"
        )


def _validate_provenance_mode(workflow: JsonObject) -> None:
    mode = workflow.get("provenance_mode", "advisory")
    if not isinstance(mode, str) or mode not in PROVENANCE_MODES:
        raise WorkflowError(
            f"workflow provenance_mode must be one of {sorted(PROVENANCE_MODES)!r}; got {mode!r}",
            field_path="provenance_mode",
        )
    if mode != "sealed_patch":
        return
    protected = workflow.get("protected_paths", [])
    operator_writable = workflow.get("operator_writable_paths", [])
    if not isinstance(protected, list) or not all(isinstance(item, str) for item in protected):
        raise WorkflowError(
            "sealed_patch workflows must declare protected_paths as a list of repo-relative strings",
            field_path="protected_paths",
        )
    if not isinstance(operator_writable, list) or not all(
        isinstance(item, str) for item in operator_writable
    ):
        raise WorkflowError(
            "sealed_patch workflows must declare operator_writable_paths as a list of repo-relative strings",
            field_path="operator_writable_paths",
        )
    _validate_path_policy("protected_paths", protected)
    _validate_path_policy("operator_writable_paths", operator_writable)
    for left in protected:
        left_path = PurePosixPath(left)
        for right in operator_writable:
            right_path = PurePosixPath(right)
            if _path_prefix(left_path, right_path) or _path_prefix(right_path, left_path):
                raise WorkflowError(
                    "sealed_patch protected_paths and operator_writable_paths must not overlap",
                    field_path="protected_paths",
                )


def _validate_path_policy(field_name: str, paths: list[str]) -> None:
    for index, value in enumerate(paths):
        path = PurePosixPath(value)
        if value == "" or value.startswith("/") or ".." in path.parts:
            raise WorkflowError(
                f"{field_name} entries must be repo-relative without '..'",
                field_path=f"{field_name}[{index}]",
            )
        if path.parts and path.parts[0] == ".striatum":
            raise WorkflowError(
                f"{field_name} must not protect .striatum/ as source",
                field_path=f"{field_name}[{index}]",
            )


def _path_prefix(left: PurePosixPath, right: PurePosixPath) -> bool:
    left_parts = left.parts
    right_parts = right.parts
    return len(left_parts) <= len(right_parts) and right_parts[: len(left_parts)] == left_parts


def _validate_harness_profiles(
    workflow: JsonObject,
    *,
    warnings: list[str] | None = None,
) -> frozenset[str]:
    """Validate the optional RFC 0010 ``harness_profiles`` map.

    Returns the set of declared profile ids (empty if the workflow does not
    declare any). Hard schema violations raise ``WorkflowError``. Unknown
    sibling fields on a profile body produce an advisory warning when
    ``warnings`` is provided; otherwise they are silently accepted (V1
    lint-warning posture per RFC 0010).
    """
    raw = workflow.get("harness_profiles")
    if raw is None:
        return frozenset()
    if not isinstance(raw, dict):
        raise WorkflowError("harness_profiles must be an object")
    profile_ids: list[str] = []
    declared_fallbacks: list[tuple[str, str]] = []
    for profile_id, body in raw.items():
        if not isinstance(profile_id, str) or not profile_id:
            raise WorkflowError("harness_profiles keys must be non-empty strings")
        if not isinstance(body, dict):
            raise WorkflowError(
                f"harness profile {profile_id!r} body must be an object"
            )
        missing = sorted(_HARNESS_PROFILE_REQUIRED_FIELDS.difference(body))
        if missing:
            raise WorkflowError(
                f"harness profile {profile_id!r} is missing required fields: "
                f"{', '.join(missing)}"
            )
        tool_family = body.get("tool_family")
        if tool_family not in HARNESS_PROFILE_TOOL_FAMILIES:
            raise WorkflowError(
                f"harness profile {profile_id!r} has unknown tool_family "
                f"{tool_family!r}; expected one of "
                f"{sorted(HARNESS_PROFILE_TOOL_FAMILIES)!r}"
            )
        strategy_version = body.get("strategy_version")
        if not isinstance(strategy_version, str) or not strategy_version:
            raise WorkflowError(
                f"harness profile {profile_id!r} strategy_version must be a "
                f"non-empty string"
            )
        accountability = body.get("accountability")
        if accountability is not None:
            if not isinstance(accountability, dict):
                raise WorkflowError(
                    f"harness profile {profile_id!r} accountability must be "
                    f"an object"
                )
            native = accountability.get("native_subagents")
            if native is not None and native != "internal_to_parent_session":
                raise WorkflowError(
                    f"harness profile {profile_id!r} "
                    f"accountability.native_subagents must be "
                    f"'internal_to_parent_session' in V1; got {native!r}"
                )
            first_class = accountability.get("first_class_registration")
            if first_class is not None and first_class != "not_supported":
                raise WorkflowError(
                    f"harness profile {profile_id!r} "
                    f"accountability.first_class_registration must be "
                    f"'not_supported' in V1; got {first_class!r}"
                )
        envelope = body.get("prompt_envelope_path")
        if envelope is not None:
            if not isinstance(envelope, str) or not envelope:
                raise WorkflowError(
                    f"harness profile {profile_id!r} prompt_envelope_path "
                    f"must be a non-empty string"
                )
            if envelope.startswith("/") or ".." in PurePosixPath(envelope).parts:
                raise WorkflowError(
                    f"harness profile {profile_id!r} prompt_envelope_path "
                    f"must be repo-relative without '..' segments"
                )
        fallback = body.get("fallback_profile_id")
        if fallback is not None:
            if not isinstance(fallback, str) or not fallback:
                raise WorkflowError(
                    f"harness profile {profile_id!r} fallback_profile_id must "
                    f"be a non-empty string"
                )
            declared_fallbacks.append((profile_id, fallback))
        if warnings is not None:
            unknown_fields = sorted(set(body).difference(_HARNESS_PROFILE_KNOWN_FIELDS))
            for field in unknown_fields:
                warnings.append(
                    f"harness profile {profile_id!r} has unknown field "
                    f"{field!r}; accepted as lint warning in V1"
                )
        profile_ids.append(profile_id)
    declared_set = frozenset(profile_ids)
    for profile_id, fallback in declared_fallbacks:
        if fallback not in declared_set:
            raise WorkflowError(
                f"harness profile {profile_id!r} fallback_profile_id "
                f"{fallback!r} is not a declared profile"
            )
    return declared_set


ADAPTER_TIMEOUT_SECONDS_MAX = 86400


def _validate_lane_constraints(
    lanes: JsonObject,
    *,
    harness_profile_ids: frozenset[str] | None = None,
    repo_root: Path | None = None,
    warnings: list[str] | None = None,
) -> None:
    """Validate optional lane adapter constraints."""
    declared_profiles = harness_profile_ids or frozenset()
    for lane_id, lane_value in lanes.items():
        if not isinstance(lane_value, dict):
            raise WorkflowError(f"lane {lane_id!r} must be an object")
        timeout = lane_value.get("adapter_timeout_seconds")
        if timeout is not None:
            if not isinstance(timeout, int) or isinstance(timeout, bool) or timeout <= 0:
                raise WorkflowError(
                    f"lane {lane_id!r} adapter_timeout_seconds must be a "
                    f"positive integer"
                )
            if timeout > ADAPTER_TIMEOUT_SECONDS_MAX:
                raise WorkflowError(
                    f"lane {lane_id!r} adapter_timeout_seconds {timeout} "
                    f"exceeds the V1 cap of {ADAPTER_TIMEOUT_SECONDS_MAX} "
                    f"(24 hours); override at CLI invocation time with "
                    f"--timeout-seconds for legitimate >24h needs"
                )
        profile_ref = lane_value.get("harness_profile_id")
        if profile_ref is not None:
            if not isinstance(profile_ref, str) or not profile_ref:
                raise WorkflowError(
                    f"lane {lane_id!r} harness_profile_id must be a non-empty string"
                )
            if profile_ref not in declared_profiles:
                raise WorkflowError(
                    f"lane {lane_id!r} references undeclared harness profile "
                    f"{profile_ref!r}"
                )
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
            if (
                warnings is not None
                and repo_root is not None
                and isinstance(command, list)
                and command
                and isinstance(command[0], str)
                and _looks_like_repo_relative_path(command[0])
                and not (repo_root / command[0]).is_file()
            ):
                warnings.append(
                    f"lane {lane_id!r} command path {command[0]!r} does not exist "
                    f"under {repo_root!s}; supervised use will fail at exec time. "
                    f"(RFC 0010 V1.5 lint; HARNESS-001 follow-up)"
                )
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


def _effective_fresh_session_required(job: JsonValue) -> bool:
    """Return whether a job effectively requires a fresh session.

    RFC 0002: ``reviewer_context_policy: "fresh"`` implies
    ``fresh_session_required: true``. When ``fresh_session_required`` is
    unset, a fresh-context review is silently treated as fresh-session
    required. Explicit conflicts are rejected during validation.
    """
    if job.get("fresh_session_required") is True:
        return True
    if (
        job.get("type") == "review"
        and job.get("reviewer_context_policy") == "fresh"
        and "fresh_session_required" not in job
    ):
        return True
    return False


def _validate_reviewer_policy(job_id: str, job: JsonValue) -> None:
    """Validate optional RFC 0002 reviewer-policy fields on a job.

    Non-review jobs cannot declare these fields. Review jobs accept
    ``reviewer_access_scope`` and ``reviewer_context_policy`` from a closed
    set of values. ``reviewer_context_policy: "fresh"`` implies
    ``fresh_session_required: true``; explicitly setting
    ``fresh_session_required: false`` alongside it is rejected.
    """
    has_access = "reviewer_access_scope" in job
    has_context = "reviewer_context_policy" in job
    if not has_access and not has_context:
        return
    if job.get("type") != "review":
        raise WorkflowError(
            f"non-review job {job_id!r} cannot declare reviewer_access_scope/reviewer_context_policy"
        )
    if has_access:
        access = job.get("reviewer_access_scope")
        if not isinstance(access, str) or access not in REVIEWER_ACCESS_SCOPE_VALUES:
            raise WorkflowError(
                f"review job {job_id!r} has unknown reviewer_access_scope {access!r}; "
                "allowed: document_only|artifact_augmented|repo_level"
            )
    if has_context:
        context = job.get("reviewer_context_policy")
        if not isinstance(context, str) or context not in REVIEWER_CONTEXT_POLICY_VALUES:
            raise WorkflowError(
                f"review job {job_id!r} has unknown reviewer_context_policy {context!r}; "
                "allowed: fresh|cross_round"
            )
        if context == "fresh" and job.get("fresh_session_required") is False:
            raise WorkflowError(
                f"review job {job_id!r} declares reviewer_context_policy=fresh but "
                "fresh_session_required=false"
            )


def _validate_review_posture(job_id: str, job: JsonValue) -> None:
    """Validate optional RFC 0018 ``review_posture`` on a review job.

    Non-review jobs cannot declare ``review_posture``. Review jobs accept
    the closed :data:`ALLOWED_POSTURES` set or a ``custom:<non-empty>``
    grammar. Empty strings, bare ``"custom:"``, and whitespace-only custom
    names are rejected.
    """
    if "review_posture" not in job:
        return
    if job.get("type") != "review":
        raise WorkflowError(
            f"non-review job {job_id!r} cannot declare review_posture"
        )
    posture = job.get("review_posture")
    if not isinstance(posture, str) or posture == "":
        raise WorkflowError(
            f"review job {job_id!r} review_posture must be a non-empty string"
        )
    if posture in ALLOWED_POSTURES:
        return
    if not posture.startswith("custom:"):
        raise WorkflowError(
            f"review job {job_id!r} has unknown review_posture {posture!r}; "
            f"allowed: {sorted(ALLOWED_POSTURES)} or custom:<name>"
        )
    custom_name = posture[len("custom:"):]
    if not custom_name.strip():
        raise WorkflowError(
            f"review job {job_id!r} review_posture {posture!r} has empty custom name"
        )


def _validate_required_review_postures(job_id: str, job: JsonValue) -> None:
    """Validate optional RFC 0018 ``required_review_postures`` on a build job.

    Non-build jobs cannot declare the field. Build jobs declare it as a
    non-empty list of strings, each either in :data:`ALLOWED_POSTURES` or
    a ``custom:<non-empty>`` value.
    """
    if "required_review_postures" not in job:
        return
    if job.get("type") != "build":
        raise WorkflowError(
            f"non-build job {job_id!r} cannot declare required_review_postures"
        )
    postures = job.get("required_review_postures")
    if not isinstance(postures, list) or not postures:
        raise WorkflowError(
            f"build job {job_id!r} required_review_postures must be a non-empty list"
        )
    for entry in postures:
        if not isinstance(entry, str) or entry == "":
            raise WorkflowError(
                f"build job {job_id!r} required_review_postures entries must be non-empty strings"
            )
        if entry in ALLOWED_POSTURES:
            continue
        if not entry.startswith("custom:") or not entry[len("custom:"):].strip():
            raise WorkflowError(
                f"build job {job_id!r} required_review_postures contains invalid entry {entry!r}; "
                f"allowed: {sorted(ALLOWED_POSTURES)} or custom:<name>"
            )


def _validate_require_attested_lane(job_id: str, job: JsonValue) -> None:
    if "require_attested_lane" not in job:
        return
    value = job.get("require_attested_lane")
    if not isinstance(value, bool):
        raise WorkflowError(
            f"job {job_id!r} require_attested_lane must be a boolean"
        )
    if value is True and job.get("type") != "review":
        raise WorkflowError(
            f"job {job_id!r} require_attested_lane is supported only on review jobs in V1"
        )


def _validate_required_postures_reachable(
    workflow: JsonObject, *, job_map: dict[str, JsonValue]
) -> None:
    """RFC 0018 V1 § Step 2: each build's required postures must be reachable.

    For every build job ``B`` with ``required_review_postures``, each entry
    must be the ``review_posture`` of at least one review job ``R`` such
    that there is a directed edge path from ``B`` to ``R`` *or* from ``R``
    to ``B``. This catches workflows whose declared review jobs cannot
    collectively satisfy a build's posture coverage at workflow-validation
    time (`run prepare` also revalidates), well before any session claims
    work.

    See ``docs/dogfood/016/decisions/V1_ACCEPTANCE.md`` for the lifecycle
    rationale (the runtime build-completion gate as originally written
    in RFC 0018 deadlocks because a build's ``complete`` mutation
    precedes its downstream review's verdict).
    """
    edges_value = workflow.get("edges", [])
    if not isinstance(edges_value, list):
        return
    forward: dict[str, set[str]] = {}
    reverse: dict[str, set[str]] = {}
    for edge in edges_value:
        if not isinstance(edge, dict):
            continue
        src = edge.get("from")
        dst = edge.get("to")
        if not isinstance(src, str) or not isinstance(dst, str):
            continue
        forward.setdefault(src, set()).add(dst)
        reverse.setdefault(dst, set()).add(src)

    def _reachable(start: str, adjacency: dict[str, set[str]]) -> set[str]:
        seen: set[str] = set()
        stack = [start]
        while stack:
            node = stack.pop()
            for neighbor in adjacency.get(node, ()):
                if neighbor in seen:
                    continue
                seen.add(neighbor)
                stack.append(neighbor)
        return seen

    for build_id, build in job_map.items():
        if not isinstance(build, dict):
            continue
        if build.get("type") != "build":
            continue
        required_value = build.get("required_review_postures")
        if not isinstance(required_value, list) or not required_value:
            continue
        reachable_jobs = _reachable(build_id, forward) | _reachable(build_id, reverse)
        available_postures: set[str] = set()
        for candidate_id in reachable_jobs:
            candidate = job_map.get(candidate_id)
            if not isinstance(candidate, dict):
                continue
            if candidate.get("type") != "review":
                continue
            posture = candidate.get("review_posture")
            if isinstance(posture, str):
                available_postures.add(posture)
            else:
                # An unposted review job covers the implicit "neutral" posture.
                available_postures.add("neutral")
        for required in required_value:
            if not isinstance(required, str):
                continue
            if required not in available_postures:
                raise WorkflowError(
                    f"build job {build_id!r} requires review posture {required!r} "
                    f"but no reachable review job declares it; available postures "
                    f"across reachable reviews: {sorted(available_postures) or ['<none>']}"
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


def _looks_like_repo_relative_path(value: str) -> bool:
    """Heuristic: does this command[0] look like a repo-relative file path?

    Returns True for ``./bin/x``, ``bin/x``, ``.striatum/bin/wrapper.sh``,
    and similar shapes. Returns False for absolute paths (``/usr/bin/x``)
    and for bare binary names that resolve via ``$PATH`` (``codex``,
    ``claude``, ``gemini``).
    """
    if value.startswith("/"):
        return False
    if value.startswith("./") or value.startswith("../"):
        return True
    return "/" in value


def _path_within(child: str, parent: str) -> bool:
    """Return True if child path is equal to or inside parent path (string-only)."""
    child_norm = _normalize_path_string(child)
    parent_norm = _normalize_path_string(parent)
    if parent_norm == "":
        return True
    if child_norm == parent_norm:
        return True
    return child_norm.startswith(parent_norm + "/")

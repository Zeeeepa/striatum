"""Workflow JSON validation and run loading."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path, PurePosixPath
from typing import Any, cast

from striatum.artifact_contracts import ALLOWED_ARTIFACT_KINDS
from striatum.errors import WorkflowError
from striatum.primitives import JsonObject
from striatum.repo_policy import (
    ADAPTER_ENFORCEMENT_LEVELS,
    adapter_constraint_enforcement,
    adapter_enforcement_satisfies,
)

# JSON workflow files are user-authored and need dynamic validation.
JsonValue = dict[str, Any]
PhaseIndex = dict[str, Any]

WORKFLOW_SCHEMA_VERSION_V1 = "striatum.workflow.v1"
WORKFLOW_SCHEMA_VERSION_V1_1 = "striatum.workflow.v1.1"
ACCEPTED_WORKFLOW_SCHEMA_VERSIONS = frozenset({
    WORKFLOW_SCHEMA_VERSION_V1,
    WORKFLOW_SCHEMA_VERSION_V1_1,
})
VERDICT_JOB_TYPES = frozenset({"review", "phase_synthesis"})

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
SUPERVISION_TRANSPORT_VALUES = {"pipe", "pty_helper"}
SUPERVISION_STDIN_DELIVERY_VALUES = {"persistent_fifo", "one_shot_eof"}
PROVENANCE_MODES = frozenset({"advisory", "attested_bylines", "sealed_patch"})
SEALED_PATCH_PROVIDERS = frozenset({"daemon", "refuse"})
APPLY_GATE_JOB_TYPES = frozenset({"build", "handoff"})

# Workflow branch.mode values:
# - "auto": run prepare creates the suggested branch and transitions the run
#           directly to state="ready" with no separate branch-confirm step.
#           Requires `suggested_name` to be set.
# - "confirm": run prepare returns state="needs_branch_confirmation"; the
#              operator runs `striatum branch confirm` (with --create / etc.)
#              to advance.
# Default when `branch.mode` is omitted: "auto".
BRANCH_MODE_VALUES = ("auto", "confirm")

REVIEWER_ACCESS_SCOPE_VALUES = (
    "document_only",
    "artifact_augmented",
    "repo_level",
    "cross_repo_artifact_augmented",
)
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
        "license, attribution, telemetry, hosted-service, data-handling, "
        "regulatory, or external-persistence issues; this scopes findings, "
        "not evidence. Read the handoff, changed files, tests, and command "
        "outputs needed to verify the declared acceptance criteria."
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
    "agy",
    "antigravity",
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


def lint_workflow(workflow: JsonObject, *, repo_root: Path | None = None) -> JsonObject:
    """Return advisory workflow risk findings separate from validation.

    Structural validation remains authoritative for accept/reject. This lint
    surface records dogfood-derived risk patterns that may be acceptable with
    an explicit operator rationale.
    """

    warnings: list[str] = []
    try:
        validate_workflow(workflow, warnings=warnings, repo_root=repo_root)
    except WorkflowError as exc:
        error: JsonObject = {"message": str(exc)}
        field_path = getattr(exc, "field_path", None)
        if isinstance(field_path, str) and field_path:
            error["field_path"] = field_path
        return {
            "workflow_id": str(workflow.get("workflow_id") or ""),
            "valid": False,
            "errors": [error],
            "warnings": [],
            "warning_count": 0,
            "coverage": _invalid_lint_coverage(),
        }

    findings: list[JsonObject] = [
        {
            "rule": "validation_warning",
            "severity": "warning",
            "message": warning,
        }
        for warning in warnings
    ]
    job_map = workflow_job_map(workflow)
    _lint_same_model_review_pairs(workflow, job_map=job_map, findings=findings)
    _lint_review_freshness(job_map=job_map, findings=findings)
    _lint_write_scope_risk(workflow, job_map=job_map, findings=findings)
    _lint_missing_escalation_path(workflow, job_map=job_map, findings=findings)
    coverage = _lint_coverage(workflow, job_map=job_map, findings=findings)

    return {
        "workflow_id": str(workflow.get("workflow_id") or ""),
        "valid": True,
        "errors": [],
        "warnings": findings,
        "warning_count": len(findings),
        "coverage": coverage,
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
    schema_version = workflow.get("schema_version")
    if schema_version not in ACCEPTED_WORKFLOW_SCHEMA_VERSIONS:
        raise WorkflowError(
            "workflow schema_version must be one of: "
            f"{', '.join(sorted(ACCEPTED_WORKFLOW_SCHEMA_VERSIONS))}",
            field_path="schema_version",
        )
    cross_repo = _validate_repositories_block(workflow)
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
        _validate_job_repository(job_index, job_id, job, cross_repo=cross_repo)
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
        _validate_reviewer_policy(job_id, job, cross_repo=cross_repo is not None)
        _validate_review_posture(job_id, job)
        _validate_required_review_postures(job_id, job)
        _validate_require_attested_lane(job_id, job)
        _validate_apply_gate(job_index, job_id, job)
    _validate_artifact_path_uniqueness(jobs)
    _validate_required_postures_reachable(workflow, job_map=job_map)
    phase_index = _validate_phases(workflow, job_map=job_map)
    explicit_edges = edge_dependency_pairs(workflow, include_phase_materialized=False)
    _validate_phase_edges(explicit_edges, job_map=job_map, phase_index=phase_index)
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
        if cross_repo is not None and _jobs_cross_repositories(job_map, from_id, to_id):
            if cycle.get("cross_repo_cycle") is not True:
                raise WorkflowError(
                    "cross-repo cycles must declare cross_repo_cycle=true",
                    field_path=f"cycles[{cycle_index}].cross_repo_cycle",
                )
        _validate_phase_cycle(
            cycle_index,
            from_id,
            to_id,
            phase_index=phase_index,
        )
    _validate_cycle_targets_feed_sources(workflow, job_map=job_map)
    _validate_parallelism(jobs)
    _validate_parallelism_config(workflow, cross_repo=cross_repo)
    _validate_revision_policy(workflow, jobs=jobs)
    _warn_same_lane_review_implement_cycles(workflow, job_map=job_map, warnings=warnings)




def workflow_job_map(workflow: JsonObject) -> dict[str, JsonValue]:
    """Return jobs keyed by workflow job id."""
    result: dict[str, JsonValue] = {}
    for job_value in _list(workflow, "jobs"):
        if not isinstance(job_value, dict):
            raise WorkflowError("each job must be an object")
        job = cast(JsonValue, job_value)
        result[_string(job, "id")] = job
    return result


def workflow_phase_index(workflow: JsonObject) -> PhaseIndex:
    """Return validated phase metadata for a workflow.

    Workflows without declared phases return ``{"declared": False, ...}``
    with empty maps. This helper is intentionally JSON-safe so CLI/status
    code can reuse it directly.
    """
    return _validate_phases(workflow, job_map=workflow_job_map(workflow))


def _empty_phase_index() -> PhaseIndex:
    return {
        "declared": False,
        "phase_order": [],
        "phase_by_id": {},
        "phase_position": {},
        "job_phase": {},
        "synthesis_by_phase": {},
    }


def _validate_phases(
    workflow: JsonObject,
    *,
    job_map: dict[str, JsonValue],
) -> PhaseIndex:
    schema_version = workflow.get("schema_version")
    phases_value = workflow.get("phases")
    jobs = list(job_map.values())

    if schema_version == WORKFLOW_SCHEMA_VERSION_V1:
        if phases_value is not None:
            raise WorkflowError(
                "striatum.workflow.v1 workflows must not declare phases",
                field_path="phases",
            )
        for job_index, job in enumerate(jobs):
            job_id = str(job.get("id") or f"jobs[{job_index}]")
            if "phase" in job:
                raise WorkflowError(
                    f"striatum.workflow.v1 job {job_id!r} must not declare phase",
                    field_path=f"jobs[{job_index}].phase",
                )
            if "phase_id" in job:
                raise WorkflowError(
                    f"striatum.workflow.v1 job {job_id!r} must not declare phase_id",
                    field_path=f"jobs[{job_index}].phase_id",
                )
            if job.get("type") == "phase_synthesis":
                raise WorkflowError(
                    f"striatum.workflow.v1 job {job_id!r} must not use type phase_synthesis",
                    field_path=f"jobs[{job_index}].type",
                )
        return _empty_phase_index()

    if phases_value is None or phases_value == []:
        for job_index, job in enumerate(jobs):
            job_id = str(job.get("id") or f"jobs[{job_index}]")
            if "phase" in job:
                raise WorkflowError(
                    f"job {job_id!r} may declare phase only when workflow phases are declared",
                    field_path=f"jobs[{job_index}].phase",
                )
            if "phase_id" in job:
                raise WorkflowError(
                    f"job {job_id!r} may declare phase_id only when workflow phases are declared",
                    field_path=f"jobs[{job_index}].phase_id",
                )
            if job.get("type") == "phase_synthesis":
                raise WorkflowError(
                    f"job {job_id!r} may use type phase_synthesis only when workflow phases are declared",
                    field_path=f"jobs[{job_index}].type",
                )
        return _empty_phase_index()

    if not isinstance(phases_value, list):
        raise WorkflowError("workflow field 'phases' must be a list", field_path="phases")

    phase_order: list[str] = []
    phase_by_id: dict[str, JsonObject] = {}
    phase_index_by_id: dict[str, int] = {}
    for phase_index, phase_value in enumerate(phases_value):
        if not isinstance(phase_value, dict):
            raise WorkflowError(
                "each phase must be an object",
                field_path=f"phases[{phase_index}]",
            )
        phase = cast(JsonObject, phase_value)
        phase_id = phase.get("id")
        if not isinstance(phase_id, str) or phase_id == "":
            raise WorkflowError(
                "phase id must be a non-empty string",
                field_path=f"phases[{phase_index}].id",
            )
        if phase_id in phase_by_id:
            raise WorkflowError(
                f"duplicate phase id {phase_id!r}",
                field_path=f"phases[{phase_index}].id",
            )
        name = phase.get("name")
        if not isinstance(name, str) or name == "":
            raise WorkflowError(
                f"phase {phase_id!r} name must be a non-empty string",
                field_path=f"phases[{phase_index}].name",
            )
        for optional_key in ("color", "description"):
            optional_value = phase.get(optional_key)
            if optional_value is not None and not isinstance(optional_value, str):
                raise WorkflowError(
                    f"phase {phase_id!r} {optional_key} must be a string when set",
                    field_path=f"phases[{phase_index}].{optional_key}",
                )
        phase_order.append(phase_id)
        phase_by_id[phase_id] = phase
        phase_index_by_id[phase_id] = phase_index

    phase_position = {phase_id: index for index, phase_id in enumerate(phase_order)}
    job_phase: dict[str, str] = {}
    synthesis_by_phase: dict[str, str] = {}
    job_count_by_phase: dict[str, int] = {phase_id: 0 for phase_id in phase_order}
    review_only_fields = {
        "reviewer_access_scope",
        "reviewer_context_policy",
        "review_posture",
        "required_review_postures",
    }

    for job_index, job in enumerate(jobs):
        job_id = _string(job, "id")
        phase_id = _job_phase_id(job, job_index=job_index, job_id=job_id)
        if not isinstance(phase_id, str) or phase_id == "":
            raise WorkflowError(
                f"job {job_id!r} must declare phase when workflow phases are declared",
                field_path=f"jobs[{job_index}].phase",
            )
        if phase_id not in phase_by_id:
            raise WorkflowError(
                f"job {job_id!r} references unknown phase {phase_id!r}",
                field_path=f"jobs[{job_index}].phase",
            )
        job_phase[job_id] = phase_id
        job_count_by_phase[phase_id] += 1
        if job.get("type") != "phase_synthesis":
            continue
        declared_review_fields = sorted(review_only_fields.intersection(job))
        if declared_review_fields:
            raise WorkflowError(
                f"phase_synthesis job {job_id!r} cannot declare review-only fields: "
                f"{', '.join(declared_review_fields)}",
                field_path=f"jobs[{job_index}].type",
            )
        existing = synthesis_by_phase.get(phase_id)
        if existing is not None:
            raise WorkflowError(
                f"phase {phase_id!r} has multiple phase_synthesis jobs: {existing}, {job_id}",
                field_path=f"jobs[{job_index}].type",
            )
        synthesis_by_phase[phase_id] = job_id

    for phase_id in phase_order:
        phase = phase_by_id[phase_id]
        phase_index = phase_index_by_id[phase_id]
        declared_synthesis_id = phase.get("synthesis_job_id")
        if not isinstance(declared_synthesis_id, str) or declared_synthesis_id == "":
            raise WorkflowError(
                f"phase {phase_id!r} must declare synthesis_job_id",
                field_path=f"phases[{phase_index}].synthesis_job_id",
            )
        if declared_synthesis_id not in job_map:
            raise WorkflowError(
                f"phase {phase_id!r} synthesis_job_id {declared_synthesis_id!r} references unknown job",
                field_path=f"phases[{phase_index}].synthesis_job_id",
            )
        if (
            job_map[declared_synthesis_id].get("type") != "phase_synthesis"
            or job_phase.get(declared_synthesis_id) != phase_id
        ):
            raise WorkflowError(
                f"phase {phase_id!r} synthesis_job_id must reference a phase_synthesis "
                "job in the same phase",
                field_path=f"phases[{phase_index}].synthesis_job_id",
            )
        synthesis_id = synthesis_by_phase.get(phase_id)
        if synthesis_id is None:
            raise WorkflowError(f"phase {phase_id!r} must declare exactly one phase_synthesis job")
        if declared_synthesis_id != synthesis_id:
            raise WorkflowError(
                f"phase {phase_id!r} synthesis_job_id {declared_synthesis_id!r} "
                f"does not match phase_synthesis job {synthesis_id!r}",
                field_path=f"phases[{phase_index}].synthesis_job_id",
            )
        if job_count_by_phase[phase_id] < 2:
            raise WorkflowError(
                f"phase {phase_id!r} phase_synthesis job must have at least one peer job"
            )

    return {
        "declared": True,
        "phase_order": phase_order,
        "phase_by_id": phase_by_id,
        "phase_position": phase_position,
        "job_phase": job_phase,
        "synthesis_by_phase": synthesis_by_phase,
    }


def _job_phase_id(job: JsonValue, *, job_index: int, job_id: str) -> Any:
    phase = job.get("phase")
    phase_id = job.get("phase_id")
    if phase is not None and phase_id is not None and phase != phase_id:
        raise WorkflowError(
            f"job {job_id!r} declares conflicting phase and phase_id values",
            field_path=f"jobs[{job_index}].phase",
        )
    return phase if phase is not None else phase_id


def _validate_phase_edges(
    edges: list[tuple[str, str, JsonObject]],
    *,
    job_map: dict[str, JsonValue],
    phase_index: PhaseIndex,
) -> None:
    if not phase_index["declared"]:
        return
    job_phase = cast(dict[str, str], phase_index["job_phase"])
    phase_position = cast(dict[str, int], phase_index["phase_position"])
    synthesis_by_phase = cast(dict[str, str], phase_index["synthesis_by_phase"])
    for from_id, to_id, _gate in edges:
        from_phase = job_phase[from_id]
        to_phase = job_phase[to_id]
        if from_phase == to_phase:
            continue
        from_position = phase_position[from_phase]
        to_position = phase_position[to_phase]
        if to_position < from_position:
            raise WorkflowError(
                f"workflow edge {from_id!r} -> {to_id!r} points from later phase "
                f"{from_phase!r} to earlier phase {to_phase!r}"
            )
        if to_position != from_position + 1:
            raise WorkflowError(
                f"workflow edge {from_id!r} -> {to_id!r} skips phases; "
                "cross-phase edges may target only the immediate next phase"
            )
        if synthesis_by_phase[from_phase] != from_id:
            raise WorkflowError(
                f"workflow edge {from_id!r} -> {to_id!r} crosses phases without "
                f"using source phase {from_phase!r} synthesis job"
            )
        if job_map[to_id].get("type") == "phase_synthesis":
            raise WorkflowError(
                f"workflow edge {from_id!r} -> {to_id!r} cannot target a later "
                "phase_synthesis job"
            )


def _validate_phase_cycle(
    cycle_index: int,
    from_id: str,
    to_id: str,
    *,
    phase_index: PhaseIndex,
) -> None:
    if not phase_index["declared"]:
        return
    job_phase = cast(dict[str, str], phase_index["job_phase"])
    from_phase = job_phase[from_id]
    to_phase = job_phase[to_id]
    if from_phase == to_phase:
        return
    raise WorkflowError(
        f"workflow cycle {from_id!r} -> {to_id!r} crosses phases; "
        "revision cycles must stay within a single phase",
        field_path=f"cycles[{cycle_index}].to",
    )


def edge_dependency_pairs(
    workflow: JsonObject,
    *,
    include_phase_materialized: bool = True,
) -> list[tuple[str, str, JsonObject]]:
    """Return normalized dependency pairs from top-level edges."""
    jobs = workflow_job_map(workflow)
    pairs: list[tuple[str, str, JsonObject]] = []
    seen: set[tuple[str, str]] = set()
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
        edge_key = (from_id, to_id)
        if edge_key in seen:
            continue
        seen.add(edge_key)
        pairs.append((from_id, to_id, {"on": "completed", "from": from_id, "to": to_id}))
    if include_phase_materialized:
        phase_index = workflow_phase_index(workflow)
        if phase_index["declared"]:
            synthesis_by_phase = cast(dict[str, str], phase_index["synthesis_by_phase"])
            job_phase = cast(dict[str, str], phase_index["job_phase"])
            for job_id in jobs:
                phase_id = job_phase[job_id]
                synthesis_id = synthesis_by_phase[phase_id]
                if job_id == synthesis_id:
                    continue
                edge_key = (job_id, synthesis_id)
                if edge_key in seen:
                    continue
                seen.add(edge_key)
                pairs.append(
                    (
                        job_id,
                        synthesis_id,
                        {"on": "completed", "from": job_id, "to": synthesis_id},
                    )
                )
    return pairs


def validate_needs_match_edges(workflow: JsonObject) -> None:
    """Reject workflows where legacy needs diverge from authoritative edges."""
    edge_needs: dict[str, set[str]] = {}
    for from_id, to_id, _gate in edge_dependency_pairs(
        workflow,
        include_phase_materialized=False,
    ):
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
    if jobs[from_id].get("type") in VERDICT_JOB_TYPES:
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
        if jobs[from_id].get("type") in VERDICT_JOB_TYPES:
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
        artifact_paths: set[tuple[str | None, str]] = set()
        write_paths: set[tuple[str | None, str]] = set()
        repo_write_modes: set[bool] = set()
        for job in members:
            for artifact in job.get("expected_artifacts", []):
                if not isinstance(artifact, dict):
                    continue
                path = artifact.get("path")
                if not isinstance(path, str):
                    continue
                key = (_job_repository_alias(job), _normalize_path_string(path))
                if key in artifact_paths:
                    raise WorkflowError(f"parallel group {group!r} reuses artifact path {path!r}")
                artifact_paths.add(key)
            scope = job.get("write_scope", {})
            if not isinstance(scope, dict):
                continue
            repo_write_modes.add(scope.get("repo_write") is True)
            if scope.get("repo_write") is not True:
                continue
            for allowed in scope.get("allowed_paths", []):
                if not isinstance(allowed, str):
                    continue
                key = (_job_repository_alias(job), _normalize_path_string(allowed))
                if key in write_paths:
                    raise WorkflowError(f"parallel group {group!r} has overlapping write scope")
                write_paths.add(key)
        if len(repo_write_modes) > 1:
            raise WorkflowError(
                f"parallel group {group!r} mixes repo_write and review-only jobs; "
                "split them into separate groups"
            )


def _validate_parallelism_config(
    workflow: JsonObject, *, cross_repo: dict[str, str] | None
) -> None:
    parallelism = workflow.get("parallelism")
    if not isinstance(parallelism, dict):
        return
    per_repo = parallelism.get("per_repo_max_active_jobs")
    if per_repo is None:
        return
    if cross_repo is None:
        raise WorkflowError(
            "parallelism.per_repo_max_active_jobs is valid only for cross-repo workflows",
            field_path="parallelism.per_repo_max_active_jobs",
        )
    if not isinstance(per_repo, dict):
        raise WorkflowError(
            "parallelism.per_repo_max_active_jobs must be an object",
            field_path="parallelism.per_repo_max_active_jobs",
        )
    for alias, value in per_repo.items():
        if alias not in cross_repo:
            raise WorkflowError(
                f"parallelism.per_repo_max_active_jobs references unknown repository alias {alias!r}",
                field_path="parallelism.per_repo_max_active_jobs",
            )
        if not isinstance(value, int) or isinstance(value, bool) or value < 1:
            raise WorkflowError(
                "parallelism.per_repo_max_active_jobs values must be positive integers",
                field_path=f"parallelism.per_repo_max_active_jobs.{alias}",
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
    require_daemon = workflow.get("require_daemon")
    if require_daemon is not None and not isinstance(require_daemon, bool):
        raise WorkflowError(
            "workflow require_daemon must be a boolean when set",
            field_path="require_daemon",
        )
    provider = workflow.get("sealed_patch_provider")
    if provider is not None:
        if not isinstance(provider, str) or provider not in SEALED_PATCH_PROVIDERS:
            raise WorkflowError(
                f"workflow sealed_patch_provider must be one of {sorted(SEALED_PATCH_PROVIDERS)!r}; got {provider!r}",
                field_path="sealed_patch_provider",
            )
        if workflow.get("provenance_mode", "advisory") != "sealed_patch":
            raise WorkflowError(
                "workflow sealed_patch_provider is valid only when provenance_mode is sealed_patch",
                field_path="sealed_patch_provider",
            )
    mode = workflow.get("provenance_mode", "advisory")
    if not isinstance(mode, str) or mode not in PROVENANCE_MODES:
        raise WorkflowError(
            f"workflow provenance_mode must be one of {sorted(PROVENANCE_MODES)!r}; got {mode!r}",
            field_path="provenance_mode",
        )
    if mode != "sealed_patch":
        return
    if require_daemon is False:
        raise WorkflowError(
            "sealed_patch workflows require require_daemon to be true or omitted",
            field_path="require_daemon",
        )
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


def _validate_repositories_block(workflow: JsonObject) -> dict[str, str] | None:
    """Validate RFC 0032's opt-in cross-repo workflow block.

    The validator stays shape-only. Daemon-backed ``run prepare`` owns the
    live check that each ``repo_id`` is registered and active.
    """
    raw = workflow.get("repositories")
    primary = workflow.get("primary_repository")
    if raw is None:
        if primary is not None:
            raise WorkflowError(
                "workflow primary_repository is valid only with repositories",
                field_path="primary_repository",
            )
        for job_index, job_value in enumerate(workflow.get("jobs", [])):
            if isinstance(job_value, dict) and "repository" in job_value:
                raise WorkflowError(
                    "single-repo workflows must not declare job repository",
                    field_path=f"jobs[{job_index}].repository",
                )
        return None
    if not isinstance(raw, dict):
        raise WorkflowError("workflow repositories must be an object", field_path="repositories")
    if len(raw) < 2:
        raise WorkflowError(
            "cross-repo workflows must declare at least two repositories",
            field_path="repositories",
        )
    aliases: dict[str, str] = {}
    repo_ids: dict[str, str] = {}
    for alias, body in raw.items():
        if not isinstance(alias, str) or alias == "":
            raise WorkflowError("repository aliases must be non-empty strings", field_path="repositories")
        if not isinstance(body, dict):
            raise WorkflowError(
                f"repository alias {alias!r} body must be an object",
                field_path=f"repositories.{alias}",
            )
        repo_id = body.get("repo_id")
        if not isinstance(repo_id, str) or repo_id == "":
            raise WorkflowError(
                f"repository alias {alias!r} must declare non-empty repo_id",
                field_path=f"repositories.{alias}.repo_id",
            )
        if repo_id in repo_ids:
            raise WorkflowError(
                f"repositories {repo_ids[repo_id]!r} and {alias!r} share repo_id {repo_id!r}",
                field_path=f"repositories.{alias}.repo_id",
            )
        aliases[alias] = repo_id
        repo_ids[repo_id] = alias
    if not isinstance(primary, str) or primary == "":
        raise WorkflowError(
            "cross-repo workflows must declare primary_repository",
            field_path="primary_repository",
        )
    if primary not in aliases:
        raise WorkflowError(
            f"primary_repository {primary!r} is not a declared repository alias",
            field_path="primary_repository",
        )
    if workflow.get("require_daemon") is False:
        raise WorkflowError(
            "cross-repo workflows require daemon mode; require_daemon cannot be false",
            field_path="require_daemon",
        )
    return aliases


def _validate_job_repository(
    job_index: int,
    job_id: str,
    job: JsonValue,
    *,
    cross_repo: dict[str, str] | None,
) -> None:
    value = job.get("repository")
    if cross_repo is None:
        if value is not None:
            raise WorkflowError(
                "single-repo workflows must not declare job repository",
                field_path=f"jobs[{job_index}].repository",
            )
        return
    if not isinstance(value, str) or value == "":
        raise WorkflowError(
            f"cross-repo job {job_id!r} must declare repository",
            field_path=f"jobs[{job_index}].repository",
        )
    if value not in cross_repo:
        raise WorkflowError(
            f"job {job_id!r} references unknown repository alias {value!r}",
            field_path=f"jobs[{job_index}].repository",
        )


def _validate_apply_gate(job_index: int, job_id: str, job: JsonValue) -> None:
    value = job.get("apply_gate")
    if value is None:
        return
    if not isinstance(value, bool):
        raise WorkflowError(
            "job apply_gate must be a boolean when set",
            field_path=f"jobs[{job_index}].apply_gate",
        )
    if value is False:
        return
    job_type = job.get("type")
    if job_type not in APPLY_GATE_JOB_TYPES:
        raise WorkflowError(
            f"job {job_id!r} may set apply_gate only on {sorted(APPLY_GATE_JOB_TYPES)!r} jobs",
            field_path=f"jobs[{job_index}].apply_gate",
        )
    artifacts = job.get("expected_artifacts", [])
    if not isinstance(artifacts, list) or not any(_artifact_is_patch_summary(item) for item in artifacts):
        raise WorkflowError(
            f"job {job_id!r} with apply_gate must declare a patch-summary expected artifact",
            field_path=f"jobs[{job_index}].apply_gate",
        )


def _artifact_is_patch_summary(value: Any) -> bool:
    if not isinstance(value, dict):
        return False
    logical_name = str(value.get("logical_name") or "")
    kind = str(value.get("kind") or "")
    path = str(value.get("path") or "")
    return (
        kind == "patch_summary"
        or logical_name in {"patch", "patch_summary"}
        or "patch" in PurePosixPath(path).name.lower()
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
        model = lane_value.get("model")
        if model is not None:
            if not isinstance(model, str) or not model:
                raise WorkflowError(
                    f"lane {lane_id!r} model must be a non-empty string"
                )
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
            supervision = lane_value.get("supervision")
            if supervision is not None:
                if not isinstance(supervision, dict):
                    raise WorkflowError(
                        f"process lane {lane_id!r} supervision must be an object"
                    )
                transport = supervision.get("transport", "pipe")
                if transport not in SUPERVISION_TRANSPORT_VALUES:
                    raise WorkflowError(
                        f"process lane {lane_id!r} supervision.transport must be one of "
                        f"{sorted(SUPERVISION_TRANSPORT_VALUES)!r}"
                    )
                stdin_delivery = supervision.get("stdin_delivery", "persistent_fifo")
                if stdin_delivery not in SUPERVISION_STDIN_DELIVERY_VALUES:
                    raise WorkflowError(
                        f"process lane {lane_id!r} supervision.stdin_delivery must be one of "
                        f"{sorted(SUPERVISION_STDIN_DELIVERY_VALUES)!r}"
                    )
                if stdin_delivery == "one_shot_eof" and transport != "pipe":
                    raise WorkflowError(
                        f"process lane {lane_id!r} supervision.stdin_delivery "
                        "'one_shot_eof' requires supervision.transport 'pipe'"
                    )
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


def _validate_reviewer_policy(job_id: str, job: JsonValue, *, cross_repo: bool) -> None:
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
                "allowed: document_only|artifact_augmented|repo_level|cross_repo_artifact_augmented"
            )
        if access == "cross_repo_artifact_augmented" and not cross_repo:
            raise WorkflowError(
                f"review job {job_id!r} may use reviewer_access_scope "
                "cross_repo_artifact_augmented only in cross-repo workflows"
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
    seen: dict[tuple[str | None, str], str] = {}
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
            normalized = (_job_repository_alias(job), _normalize_path_string(path))
            if normalized in seen and seen[normalized] != job_id:
                raise WorkflowError(
                    f"jobs {seen[normalized]!r} and {job_id!r} both declare expected "
                    f"artifact path {path!r}"
                )
            seen.setdefault(normalized, job_id)


def _job_repository_alias(job: JsonValue) -> str | None:
    value = job.get("repository")
    return value if isinstance(value, str) and value != "" else None


def _jobs_cross_repositories(
    job_map: dict[str, JsonValue], left_id: str, right_id: str
) -> bool:
    left = _job_repository_alias(job_map[left_id])
    right = _job_repository_alias(job_map[right_id])
    return left is not None and right is not None and left != right


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
    edges = [
        (from_id, to_id)
        for from_id, to_id, _gate in edge_dependency_pairs(workflow)
    ]
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


def _warn_same_lane_review_implement_cycles(
    workflow: JsonObject,
    *,
    job_map: dict[str, JsonValue],
    warnings: list[str] | None,
) -> None:
    """Warn when a needs_revision cycle pairs a reviewer and an implementer on
    the same lane.

    Dogfood-042 D095/D096 documented the codex/codex anti-pattern: when the
    cycle's `from` (review job) and `to` (implementer job) share a lane, the
    iteration loops back into the same lane's voice and tends not to converge
    on the same findings the cross-lane reviewers consider acceptable. This
    is a soft warning, not a hard refusal — operators may explicitly opt in
    by setting `cycle.allow_same_lane: true`.
    """
    if warnings is None:
        return
    for cycle_value in workflow.get("cycles", []):
        if not isinstance(cycle_value, dict):
            continue
        if cycle_value.get("allow_same_lane") is True:
            continue
        from_id = cycle_value.get("from")
        to_id = cycle_value.get("to")
        if not isinstance(from_id, str) or not isinstance(to_id, str):
            continue
        from_job = job_map.get(from_id)
        to_job = job_map.get(to_id)
        if not isinstance(from_job, dict) or not isinstance(to_job, dict):
            continue
        from_lane = from_job.get("lane_id")
        to_lane = to_job.get("lane_id")
        if not isinstance(from_lane, str) or not isinstance(to_lane, str):
            continue
        if from_lane == to_lane:
            warnings.append(
                f"cycle from {from_id!r} to {to_id!r} pairs reviewer and "
                f"implementer on the same lane ({from_lane!r}); per "
                f"dogfood-042 D095/D096 this often fails to converge. "
                f"Set cycle.allow_same_lane=true to suppress."
            )


def _lint_same_model_review_pairs(
    workflow: JsonObject,
    *,
    job_map: dict[str, JsonValue],
    findings: list[JsonObject],
) -> None:
    lanes = workflow.get("lanes")
    if not isinstance(lanes, dict):
        return
    emitted: set[tuple[str, str, str]] = set()

    def family_for_job(job: JsonValue) -> str | None:
        lane_id = job.get("lane_id")
        if not isinstance(lane_id, str):
            return None
        lane = lanes.get(lane_id)
        if not isinstance(lane, dict):
            return None
        display_model = lane.get("display_model")
        source = display_model if isinstance(display_model, str) and display_model else lane_id
        return _model_family(source)

    for from_id, to_id, _gate in edge_dependency_pairs(workflow):
        upstream = job_map[from_id]
        review_job = job_map[to_id]
        if review_job.get("type") != "review" or upstream.get("type") in VERDICT_JOB_TYPES:
            continue
        upstream_family = family_for_job(upstream)
        review_family = family_for_job(review_job)
        if upstream_family is None or review_family is None or upstream_family != review_family:
            continue
        key = ("edge", from_id, to_id)
        if key in emitted:
            continue
        emitted.add(key)
        findings.append(
            {
                "rule": "same_model_review_pair",
                "severity": "warning",
                "message": (
                    f"review job {to_id!r} and upstream job {from_id!r} use "
                    f"the same model family {review_family!r}; use an independent "
                    "review lane or record an explicit override rationale"
                ),
                "job_id": to_id,
                "related_job_id": from_id,
                "model_family": review_family,
            }
        )

    for cycle in workflow.get("cycles", []):
        if not isinstance(cycle, dict):
            continue
        if cycle.get("allow_same_model") is True:
            continue
        cycle_from = cycle.get("from")
        cycle_to = cycle.get("to")
        if not isinstance(cycle_from, str) or not isinstance(cycle_to, str):
            continue
        cycle_review = job_map.get(cycle_from)
        implementer = job_map.get(cycle_to)
        if not isinstance(cycle_review, dict) or not isinstance(implementer, dict):
            continue
        if cycle_review.get("type") not in VERDICT_JOB_TYPES:
            continue
        review_family = family_for_job(cycle_review)
        implementer_family = family_for_job(implementer)
        if review_family is None or implementer_family is None or review_family != implementer_family:
            continue
        key = ("cycle", cycle_from, cycle_to)
        if key in emitted:
            continue
        emitted.add(key)
        findings.append(
            {
                "rule": "same_model_revision_cycle",
                "severity": "warning",
                "message": (
                    f"revision cycle {cycle_from!r} -> {cycle_to!r} returns review "
                    f"to the same model family {review_family!r}; set "
                    "cycle.allow_same_model=true only with an override rationale"
                ),
                "job_id": cycle_from,
                "related_job_id": cycle_to,
                "model_family": review_family,
            }
        )


def _lint_review_freshness(
    *, job_map: dict[str, JsonValue], findings: list[JsonObject]
) -> None:
    for job_id, job in job_map.items():
        if job.get("type") != "review":
            continue
        if _effective_fresh_session_required(job) or job.get("reviewer_context_policy") == "fresh":
            continue
        findings.append(
            {
                "rule": "review_without_fresh_context",
                "severity": "warning",
                "message": (
                    f"review job {job_id!r} does not require a fresh session; "
                    "fresh review context reduces reviewer contamination"
                ),
                "job_id": job_id,
            }
        )


def _lint_write_scope_risk(
    workflow: JsonObject,
    *,
    job_map: dict[str, JsonValue],
    findings: list[JsonObject],
) -> None:
    lanes = workflow.get("lanes")
    lane_map = lanes if isinstance(lanes, dict) else {}
    parallelism = workflow.get("parallelism")
    workflow_can_overlap = False
    if isinstance(parallelism, dict):
        max_active = parallelism.get("max_active_jobs")
        workflow_can_overlap = isinstance(max_active, int) and max_active > 1
    for job_id, job in job_map.items():
        scope = job.get("write_scope")
        if not isinstance(scope, dict):
            continue
        repo_write = scope.get("repo_write") is True or scope.get("mode") == "repo_write"
        if not repo_write:
            continue
        allowed = scope.get("allowed_paths")
        broad = not isinstance(allowed, list) or not allowed or any(
            item in {"", ".", "./", "/"} for item in allowed if isinstance(item, str)
        )
        if broad:
            findings.append(
                {
                    "rule": "broad_write_scope",
                    "severity": "warning",
                    "message": (
                        f"repo-write job {job_id!r} has broad or empty allowed_paths; "
                        "narrow write scope before running untrusted changes"
                    ),
                    "job_id": job_id,
                }
            )
        lane_id = job.get("lane_id")
        lane = lane_map.get(lane_id) if isinstance(lane_id, str) else None
        if workflow_can_overlap and (
            not isinstance(lane, dict) or lane.get("worktree_isolation") != "per_job"
        ):
            findings.append(
                {
                    "rule": "repo_write_without_worktree_isolation",
                    "severity": "warning",
                    "message": (
                        f"repo-write job {job_id!r} is not on a per-job worktree lane; "
                        "parallel or revision work can collide in the main worktree"
                    ),
                    "job_id": job_id,
                }
            )


def _lint_missing_escalation_path(
    workflow: JsonObject,
    *,
    job_map: dict[str, JsonValue],
    findings: list[JsonObject],
) -> None:
    has_review = any(job.get("type") in VERDICT_JOB_TYPES for job in job_map.values())
    if not has_review:
        return
    policy = workflow.get("review_revision_policy")
    root_policy = policy.get("root_review_needs_revision") if isinstance(policy, dict) else None
    has_revision_cycle = any(
        isinstance(cycle, dict) and cycle.get("on_verdict") == "needs_revision"
        for cycle in workflow.get("cycles", [])
    )
    if has_revision_cycle or root_policy == "human_checkpoint":
        return
    findings.append(
        {
            "rule": "missing_review_escalation_path",
            "severity": "warning",
            "message": (
                "workflow has review jobs but no needs_revision cycle or "
                "review_revision_policy.root_review_needs_revision=human_checkpoint"
            ),
        }
    )


def _invalid_lint_coverage() -> JsonObject:
    checks = [
        _lint_coverage_check(
            "reviewer_independence",
            passed=False,
            reason="workflow is invalid; reviewer independence was not evaluated",
        ),
        _lint_coverage_check(
            "fresh_context",
            passed=False,
            reason="workflow is invalid; review context freshness was not evaluated",
        ),
        _lint_coverage_check(
            "write_isolation",
            passed=False,
            reason="workflow is invalid; write isolation was not evaluated",
        ),
        _lint_coverage_check(
            "revision_or_escalation_path",
            passed=False,
            reason="workflow is invalid; revision and escalation paths were not evaluated",
        ),
        _lint_coverage_check(
            "posture_diversity",
            passed=False,
            reason="workflow is invalid; review posture diversity was not evaluated",
        ),
    ]
    return {
        "score": 0,
        "max_score": len(checks),
        "level": "weak",
        "checks": checks,
    }


def _lint_coverage(
    workflow: JsonObject,
    *,
    job_map: dict[str, JsonValue],
    findings: list[JsonObject],
) -> JsonObject:
    rules = {str(finding.get("rule")) for finding in findings}
    review_jobs = [
        job
        for job in job_map.values()
        if job.get("type") in VERDICT_JOB_TYPES
    ]
    reviewer_independent = not (
        {"same_model_review_pair", "same_model_revision_cycle"} & rules
    )
    fresh_context = "review_without_fresh_context" not in rules
    write_isolated = not (
        {"broad_write_scope", "repo_write_without_worktree_isolation"}
        & rules
    )
    has_revision_or_escalation = "missing_review_escalation_path" not in rules
    checks = [
        _lint_coverage_check(
            "reviewer_independence",
            passed=reviewer_independent,
            reason=(
                "workflow has no review jobs"
                if not review_jobs
                else "review lanes are model-family independent"
                if reviewer_independent
                else "one or more review lanes share model family with implementation work"
            ),
        ),
        _lint_coverage_check(
            "fresh_context",
            passed=fresh_context,
            reason=(
                "workflow has no review jobs"
                if not review_jobs
                else "review jobs require fresh context"
                if fresh_context
                else "one or more review jobs can reuse contaminated context"
            ),
        ),
        _lint_coverage_check(
            "write_isolation",
            passed=write_isolated,
            reason=(
                "repo-write jobs are narrowly scoped and isolated"
                if write_isolated
                else "one or more repo-write jobs are broad or lack per-job isolation"
            ),
        ),
        _lint_coverage_check(
            "revision_or_escalation_path",
            passed=has_revision_or_escalation,
            reason=(
                "workflow has no review jobs"
                if not review_jobs
                else "review verdicts have a revision or human escalation path"
                if has_revision_or_escalation
                else "review verdicts lack a revision or human escalation path"
            ),
        ),
        _lint_coverage_check(
            "posture_diversity",
            passed=_has_review_posture_diversity(review_jobs),
            reason=_review_posture_diversity_reason(review_jobs),
        ),
    ]
    score = sum(1 for check in checks if check["passed"] is True)
    max_score = len(checks)
    return {
        "score": score,
        "max_score": max_score,
        "level": _lint_coverage_level(score=score, max_score=max_score),
        "checks": checks,
    }


def _lint_coverage_check(
    check_id: str,
    *,
    passed: bool,
    reason: str,
) -> JsonObject:
    return {
        "id": check_id,
        "passed": passed,
        "weight": 1,
        "reason": reason,
    }


def _has_review_posture_diversity(review_jobs: list[JsonValue]) -> bool:
    if not review_jobs:
        return True
    postures = {
        str(job.get("review_posture") or "neutral")
        for job in review_jobs
    }
    return len(postures) >= 2


def _review_posture_diversity_reason(review_jobs: list[JsonValue]) -> str:
    if not review_jobs:
        return "workflow has no review jobs"
    postures = sorted(
        {
            str(job.get("review_posture") or "neutral")
            for job in review_jobs
        }
    )
    if len(postures) >= 2:
        return f"review jobs cover multiple postures: {', '.join(postures)}"
    return f"review jobs cover only one posture: {postures[0]}"


def _lint_coverage_level(*, score: int, max_score: int) -> str:
    if max_score <= 0:
        return "weak"
    if score == max_score:
        return "strong"
    if score >= max(1, (max_score * 3 + 4) // 5):
        return "adequate"
    return "weak"


def _model_family(value: str) -> str:
    tokens = [token for token in re.split(r"[^a-z0-9]+", value.lower()) if token]
    if not tokens:
        return value.lower()
    if tokens[0] in {"openai", "anthropic", "google"} and len(tokens) > 1:
        return tokens[1]
    return tokens[0]


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

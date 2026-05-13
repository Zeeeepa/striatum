"""Pure workflow generator for RFC 0034 V1."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from pathlib import PurePosixPath
from typing import Any

from striatum.db import ADAPTER_ENFORCEMENT_LEVELS, JsonObject, utc_now
from striatum.errors import WorkflowError
from striatum.workflow import (
    ALLOWED_POSTURES,
    CONSTRAINT_VALUES,
    HARNESS_PROFILE_TOOL_FAMILIES,
    validate_workflow,
    workflow_graph_data,
)

GENERATOR_SCHEMA_VERSION = "striatum.workflow_generator.v1"
PLAN_SCHEMA_VERSION = "striatum.workflow_plan.v1"
SHAPES = frozenset({
    "minimal",
    "review",
    "code_change",
    "human_checkpoint",
    "evidence_backed",
    "multi_review_synthesis",
    "multi_phase",
    "custom",
})
LANE_SETS = frozenset({"local", "single_agent", "author_reviewer", "multi_review", "custom"})
LANE_MODIFIERS = frozenset({"supervised", "worktree_isolated", "constrained", "harness_profiled"})
OPTION_KEYS = frozenset({
    "review_postures",
    "max_revision_cycles",
    "include_support_ledger",
    "constraints",
    "required_enforcement",
    "harness_profiles",
    "reviewer_count",
    "custom_job_artifacts",
    "supervision_compatible",
    "phases",
})
BLOCK_KINDS = frozenset({
    "draft",
    "review",
    "synthesis",
    "implementation",
    "test",
    "human_checkpoint",
    "support_ledger",
    "evidence_audit",
    "final_review",
})


class GeneratorError(WorkflowError):
    """Raised when a workflow generation spec is invalid."""

    def __init__(
        self,
        message: str,
        *,
        field_path: str | None = None,
        hint: str | None = None,
        ref: str | None = None,
    ) -> None:
        super().__init__(message, field_path=field_path)
        self.hint = hint
        self.ref = ref


@dataclass(frozen=True)
class WorkflowGenerationSpec:
    """Normalized workflow generation input."""

    schema_version: str
    shape: str
    lane_set: str
    workflow_id: str
    name: str
    workflow_version: str
    branch: JsonObject
    scaffold_root: str
    artifact_root: str
    lanes: dict[str, JsonObject]
    options: JsonObject = field(default_factory=dict)
    lane_modifiers: list[str] = field(default_factory=list)
    plan: JsonObject | None = None
    context_docs: list[JsonObject] = field(default_factory=list)
    parallelism: JsonObject | None = None

    @classmethod
    def from_json(cls, raw: JsonObject) -> "WorkflowGenerationSpec":
        allowed = {
            "schema_version",
            "shape",
            "lane_set",
            "workflow_id",
            "name",
            "workflow_version",
            "branch",
            "scaffold_root",
            "artifact_root",
            "lanes",
            "options",
            "lane_modifiers",
            "plan",
            "context_docs",
            "parallelism",
        }
        unknown = sorted(set(raw).difference(allowed))
        if unknown:
            raise GeneratorError(f"unknown generator spec field: {unknown[0]}", field_path=f"spec.{unknown[0]}")
        schema_version = _required_str(raw, "schema_version")
        if schema_version != GENERATOR_SCHEMA_VERSION:
            raise GeneratorError("unsupported generator schema_version", field_path="spec.schema_version")
        shape = _choice(raw, "shape", SHAPES)
        lane_set = _choice(raw, "lane_set", LANE_SETS)
        workflow_id = _required_str(raw, "workflow_id")
        name = _required_str(raw, "name")
        workflow_version = _required_str(raw, "workflow_version")
        branch = _required_object(raw, "branch")
        scaffold_root = _safe_path(_required_str(raw, "scaffold_root"), "spec.scaffold_root")
        artifact_root = _safe_path(_required_str(raw, "artifact_root"), "spec.artifact_root")
        lanes = _lanes(raw.get("lanes"), lane_set=lane_set)
        options = _object(raw.get("options", {}), "spec.options")
        unknown_options = sorted(set(options).difference(OPTION_KEYS))
        if unknown_options:
            raise GeneratorError(
                f"unknown generator option: {unknown_options[0]}",
                field_path=f"spec.options.{unknown_options[0]}",
            )
        lane_modifiers = _str_list(raw.get("lane_modifiers", []), "spec.lane_modifiers")
        for index, modifier in enumerate(lane_modifiers):
            if modifier not in LANE_MODIFIERS:
                raise GeneratorError("unknown lane modifier", field_path=f"spec.lane_modifiers[{index}]")
        context_docs = _object_list(raw.get("context_docs", []), "spec.context_docs")
        parallelism = None
        if "parallelism" in raw:
            parallelism = _object(raw["parallelism"], "spec.parallelism")
        plan = None
        if "plan" in raw:
            plan = _object(raw["plan"], "spec.plan")
        if shape == "custom" and plan is None:
            raise GeneratorError("custom shape requires a plan", field_path="spec.plan")
        if shape != "custom" and plan is not None:
            raise GeneratorError("plan is valid only for custom shape", field_path="spec.plan")
        return cls(
            schema_version=schema_version,
            shape=shape,
            lane_set=lane_set,
            workflow_id=workflow_id,
            name=name,
            workflow_version=workflow_version,
            branch=branch,
            scaffold_root=scaffold_root,
            artifact_root=artifact_root,
            lanes=lanes,
            options=options,
            lane_modifiers=lane_modifiers,
            plan=plan,
            context_docs=context_docs,
            parallelism=parallelism,
        )

    def to_json(self) -> JsonObject:
        result: JsonObject = {
            "schema_version": self.schema_version,
            "shape": self.shape,
            "lane_set": self.lane_set,
            "lane_modifiers": list(self.lane_modifiers),
            "workflow_id": self.workflow_id,
            "name": self.name,
            "workflow_version": self.workflow_version,
            "branch": dict(self.branch),
            "scaffold_root": self.scaffold_root,
            "artifact_root": self.artifact_root,
            "lanes": {key: dict(value) for key, value in self.lanes.items()},
            "options": dict(self.options),
            "context_docs": list(self.context_docs),
        }
        if self.plan is not None:
            result["plan"] = dict(self.plan)
        if self.parallelism is not None:
            result["parallelism"] = dict(self.parallelism)
        return result


@dataclass(frozen=True)
class GeneratedWorkflow:
    """Generated workflow envelope returned by API, CLI, and service."""

    workflow: JsonObject
    files: list[JsonObject]
    metadata: JsonObject
    warnings: list[str]
    validation: JsonObject

    def to_json(self) -> JsonObject:
        return {
            "workflow": self.workflow,
            "files": self.files,
            "metadata": self.metadata,
            "warnings": self.warnings,
            "validation": self.validation,
        }


def generate_workflow(spec: WorkflowGenerationSpec | JsonObject) -> GeneratedWorkflow:
    """Compile and validate a workflow generation spec without filesystem writes."""
    normalized = WorkflowGenerationSpec.from_json(spec) if isinstance(spec, dict) else spec
    warnings: list[str] = []
    _validate_modifier_matrix(normalized, warnings)
    lanes, _declared_roles = _compile_lanes(normalized)
    if normalized.shape == "custom":
        jobs, edges, cycles = _compile_custom(normalized, lanes)
        phases: list[JsonObject] = []
    else:
        jobs, edges, cycles, phases = _compile_shape(normalized)
    roles = _roles(sorted({str(job["role_id"]) for job in jobs}))
    workflow: JsonObject = {
        "schema_version": "striatum.workflow.v1.1" if normalized.shape == "multi_phase" else "striatum.workflow.v1",
        "workflow_id": normalized.workflow_id,
        "workflow_version": normalized.workflow_version,
        "name": normalized.name,
        "branch": dict(normalized.branch),
        "coordinator": _coordinator(normalized, lanes),
        "lanes": lanes,
        "roles": roles,
        "context_docs": list(normalized.context_docs),
        "parallelism": normalized.parallelism or _parallelism(normalized),
        "jobs": jobs,
        "edges": edges,
        "cycles": cycles,
    }
    if normalized.shape == "multi_phase":
        workflow["phases"] = phases
    if "harness_profiled" in normalized.lane_modifiers:
        workflow["harness_profiles"] = _harness_profiles(normalized)
    try:
        validate_workflow(workflow)
    except WorkflowError as exc:
        raise GeneratorError(str(exc), field_path=exc.field_path or "workflow") from exc
    graph = workflow_graph_data(workflow)["graph"]
    workflow_path = f"{normalized.scaffold_root}/workflow.json"
    files = _render_files(normalized, workflow, roles)
    metadata: JsonObject = {
        "shape": normalized.shape,
        "lane_set": normalized.lane_set,
        "lane_modifiers": list(normalized.lane_modifiers),
        "graph": graph,
        "catalog_templates": [normalized.shape, normalized.lane_set],
        "scaffold_root": normalized.scaffold_root,
        "workflow_path": workflow_path,
    }
    return GeneratedWorkflow(
        workflow=workflow,
        files=files,
        metadata=metadata,
        warnings=warnings,
        validation={"ok": True, "workflow_id": normalized.workflow_id},
    )


def default_spec(
    *,
    scaffold_root: str,
    artifact_root: str,
    shape: str,
    lane_set: str = "local",
    workflow_id: str | None = None,
    name: str | None = None,
    workflow_version: str | None = None,
    branch_suggestion: str | None = None,
    lanes: dict[str, JsonObject] | None = None,
    lane_modifiers: list[str] | None = None,
    options: JsonObject | None = None,
    plan: JsonObject | None = None,
) -> WorkflowGenerationSpec:
    """Build a normalized spec with CLI-friendly defaults."""
    safe_slug = PurePosixPath(scaffold_root).name or "starter-workflow"
    workflow_id = workflow_id or f"{safe_slug}-starter"
    name = name or f"{safe_slug} starter ({shape.replace('_', '-')})"
    workflow_version = workflow_version or utc_now().split("T", 1)[0]
    branch_suggestion = branch_suggestion or f"striatum/{safe_slug}"
    raw: JsonObject = {
        "schema_version": GENERATOR_SCHEMA_VERSION,
        "shape": shape,
        "lane_set": lane_set,
        "lane_modifiers": lane_modifiers or [],
        "workflow_id": workflow_id,
        "name": name,
        "workflow_version": workflow_version,
        "branch": {"mode": "confirm", "suggested_name": branch_suggestion, "allow_dirty": False},
        "scaffold_root": scaffold_root,
        "artifact_root": artifact_root,
        "lanes": lanes or {},
        "options": options or {},
        "context_docs": [],
    }
    if plan is not None:
        raw["plan"] = plan
    return WorkflowGenerationSpec.from_json(raw)


def _compile_lanes(spec: WorkflowGenerationSpec) -> tuple[JsonObject, JsonObject]:
    if spec.lane_set == "local":
        lanes: JsonObject = {
            "local": {
                "adapter": "process",
                "display_model": "Local Fixture",
                "command": ["sh", "-c", "cat >/dev/null"],
                "capabilities": ["write", "review"],
            }
        }
        return _apply_lane_modifiers(spec, lanes, repo_write_lanes={"local"}), _roles(["author", "reviewer"])
    lane_ids = _lane_ids_for(spec)
    lanes = {}
    for lane_id in lane_ids:
        user_lane = spec.lanes.get(lane_id)
        if user_lane is None:
            raise GeneratorError(
                f"lane_set {spec.lane_set!r} requires lane {lane_id!r}",
                field_path=f"spec.lanes.{lane_id}",
            )
        command = user_lane.get("command")
        if not isinstance(command, list) or not all(isinstance(part, str) for part in command) or not command:
            raise GeneratorError("lane command must be a non-empty JSON string array", field_path=f"spec.lanes.{lane_id}.command")
        lanes[lane_id] = {
            "adapter": str(user_lane.get("adapter", "process")),
            "display_model": str(user_lane.get("display_model", lane_id)),
            "command": command,
            "capabilities": list(user_lane.get("capabilities", ["write", "review", "synthesis"])),
        }
    repo_write = {lane_id for lane_id in lane_ids if "reviewer" not in lane_id}
    return _apply_lane_modifiers(spec, lanes, repo_write_lanes=repo_write), _roles(["author", "reviewer"])


def _lane_ids_for(spec: WorkflowGenerationSpec) -> list[str]:
    if spec.lane_set == "single_agent":
        return ["agent"]
    if spec.lane_set == "author_reviewer":
        return ["author", "reviewer"]
    if spec.lane_set == "multi_review":
        count = _reviewer_count(spec)
        return ["author", *[f"reviewer_{index}" for index in range(1, count + 1)]]
    if spec.lane_set == "custom":
        return sorted(spec.lanes)
    return ["local"]


def _compile_shape(spec: WorkflowGenerationSpec) -> tuple[
    list[JsonObject],
    list[JsonObject],
    list[JsonObject],
    list[JsonObject],
]:
    base = spec.artifact_root
    author_lane = _author_lane(spec)
    reviewer_lane = _reviewer_lane(spec, 1)
    jobs: list[JsonObject] = []
    edges: list[JsonObject] = []
    cycles: list[JsonObject] = []
    phases: list[JsonObject] = []
    if spec.shape == "minimal":
        jobs = [_job("draft", "draft", "Draft starter artifact", "author", author_lane, base, "DRAFT.md", "handoff", "draft", objective="Produce the starter artifact for this workflow.")]
    elif spec.shape in {"review", "code_change"}:
        jobs = [
            _job("draft", "draft", "Draft starter artifact", "author", author_lane, base, "DRAFT.md", "handoff", "draft", objective="Produce the starter artifact for this workflow."),
            _review_job("review", reviewer_lane, f"{base}/review/REVIEW.md", posture=_first_posture(spec)),
            _job("apply", "synthesis", "Apply the accepted review", "author", author_lane, base, "SUMMARY.md", "synthesis", "summary", prompt="apply", objective="Apply the accepted review findings."),
        ]
        edges = [{"from": "draft", "to": "review", "on": "completed"}, {"from": "review", "to": "apply", "on": "completed"}]
        if spec.shape == "code_change":
            cycles = [{"from": "review", "to": "draft", "on_verdict": "needs_revision", "max_iterations": _max_cycles(spec)}]
    elif spec.shape == "human_checkpoint":
        jobs = [
            _job("analysis", "draft", "Analyze the requested decision", "author", author_lane, base, "ANALYSIS.md", "handoff", "analysis", prompt="draft"),
            _job("checkpoint", "human_checkpoint", "Open a human checkpoint", "reviewer", reviewer_lane, base, "CHECKPOINT.md", "handoff", "checkpoint", prompt="review"),
            _job("apply", "synthesis", "Apply the owner decision", "author", author_lane, base, "SUMMARY.md", "synthesis", "summary", prompt="apply"),
        ]
        edges = [{"from": "analysis", "to": "checkpoint", "on": "completed"}, {"from": "checkpoint", "to": "apply", "on": "completed"}]
    elif spec.shape == "evidence_backed":
        jobs = [
            _job("draft", "draft", "Draft evidence-backed artifact", "author", author_lane, base, "DRAFT.md", "handoff", "draft"),
            _job("support_ledger", "build", "Map claims to evidence", "author", author_lane, f"{base}/support", "SUPPORT_LEDGER.md", "support_ledger", "support_ledger", prompt="support_ledger"),
            _review_job("evidence_audit", reviewer_lane, f"{base}/audit/EVIDENCE_AUDIT.md", posture="devils_advocate"),
            _review_job("final_review", reviewer_lane, f"{base}/final/FINAL_REVIEW.md", posture=_first_posture(spec)),
        ]
        edges = [{"from": "draft", "to": "support_ledger", "on": "completed"}, {"from": "support_ledger", "to": "evidence_audit", "on": "completed"}, {"from": "evidence_audit", "to": "final_review", "on": "completed"}]
    elif spec.shape == "multi_review_synthesis":
        count = _reviewer_count(spec)
        postures = _postures(spec, count)
        jobs = [
            _review_job(f"review_{index}", _reviewer_lane(spec, index), f"{base}/review_{index}/REVIEW.md", posture=postures[index - 1])
            for index in range(1, count + 1)
        ]
        jobs.extend([
            _job("synthesis", "synthesis", "Synthesize independent reviews", "author", author_lane, base, "SYNTHESIS.md", "synthesis", "synthesis", prompt="apply"),
            _review_job("final_review", _reviewer_lane(spec, 1), f"{base}/final/FINAL_REVIEW.md", posture="neutral"),
        ])
        edges = [{"from": f"review_{index}", "to": "synthesis", "on": "completed"} for index in range(1, count + 1)]
        edges.append({"from": "synthesis", "to": "final_review", "on": "completed"})
    elif spec.shape == "multi_phase":
        jobs, edges, cycles, phases = _compile_multi_phase(spec)
    else:
        raise GeneratorError("unknown workflow shape", field_path="spec.shape")
    return jobs, edges, cycles, phases


def _compile_multi_phase(
    spec: WorkflowGenerationSpec,
) -> tuple[list[JsonObject], list[JsonObject], list[JsonObject], list[JsonObject]]:
    raw_phases = _object_list(spec.options.get("phases", []), "spec.options.phases")
    if not raw_phases:
        raise GeneratorError("multi_phase requires at least one phase", field_path="spec.options.phases")
    jobs: list[JsonObject] = []
    edges: list[JsonObject] = []
    cycles: list[JsonObject] = []
    phases: list[JsonObject] = []
    previous_synthesis_id: str | None = None
    seen_phase_ids: set[str] = set()
    for phase_index, raw_phase in enumerate(raw_phases):
        phase_path = f"spec.options.phases[{phase_index}]"
        phase_id = _required_str(raw_phase, "id", prefix=phase_path)
        if phase_id in seen_phase_ids:
            raise GeneratorError("duplicate phase id", field_path=f"{phase_path}.id")
        seen_phase_ids.add(phase_id)
        phase_name = _required_str(raw_phase, "name", prefix=phase_path)
        phase: JsonObject = {"id": phase_id, "name": phase_name}
        if "description" in raw_phase:
            description = raw_phase["description"]
            if not isinstance(description, str):
                raise GeneratorError("phase description must be a string", field_path=f"{phase_path}.description")
            phase["description"] = description
        if "color" in raw_phase:
            color = raw_phase["color"]
            if not isinstance(color, str):
                raise GeneratorError("phase color must be a string", field_path=f"{phase_path}.color")
            phase["color"] = color
        tracks = _object_list(raw_phase.get("tracks", []), f"{phase_path}.tracks")
        if not tracks:
            raise GeneratorError("phase requires at least one track", field_path=f"{phase_path}.tracks")
        phase_jobs: list[JsonObject] = []
        phase_edges: list[JsonObject] = []
        phase_cycles: list[JsonObject] = []
        entry_ids: list[str] = []
        terminal_ids: list[str] = []
        seen_track_ids: set[str] = set()
        for track_index, track in enumerate(tracks):
            track_path = f"{phase_path}.tracks[{track_index}]"
            track_id = _required_str(track, "id", prefix=track_path)
            if track_id in seen_track_ids:
                raise GeneratorError("duplicate phase track id", field_path=f"{track_path}.id")
            seen_track_ids.add(track_id)
            track_shape = _choice(track, "shape", SHAPES - {"custom", "multi_phase"}, prefix=track_path)
            track_spec = _track_spec(spec, phase_id=phase_id, track_id=track_id, track_shape=track_shape, track=track)
            track_jobs, track_edges, track_cycles, _track_phases = _compile_shape(track_spec)
            prefix = f"{phase_id}__{track_id}__"
            id_map = {str(job["id"]): f"{prefix}{job['id']}" for job in track_jobs}
            track_entry_ids = _entry_job_ids(track_jobs, track_edges, id_map=id_map)
            phase_jobs.extend(
                _phase_track_jobs(
                    track_jobs,
                    id_map=id_map,
                    phase_id=phase_id,
                    track_id=track_id,
                    parallel_job_ids=set(track_entry_ids),
                )
            )
            phase_edges.extend(_remap_edges(track_edges, id_map=id_map))
            phase_cycles.extend(_remap_cycles(track_cycles, id_map=id_map))
            track_lane_id = track.get("lane_id") if isinstance(track.get("lane_id"), str) else None
            if track_lane_id is not None:
                _apply_track_lane_override(phase_jobs, id_map=set(id_map.values()), lane_id=track_lane_id)
            entry_ids.extend(track_entry_ids)
            terminal_ids.extend(_terminal_job_ids(track_jobs, track_edges, id_map=id_map))
        synthesis_id = f"{phase_id}__synthesis"
        synthesis_lane = _phase_synthesis_lane(spec, raw_phase)
        phase_jobs.append(_phase_synthesis_job(phase_id, phase_name, synthesis_id, synthesis_lane, spec.artifact_root))
        for terminal_id in sorted(set(terminal_ids)):
            phase_edges.append({"from": terminal_id, "to": synthesis_id, "on": "completed"})
        if previous_synthesis_id is not None:
            for entry_id in sorted(set(entry_ids)):
                edges.append({"from": previous_synthesis_id, "to": entry_id, "on": "completed"})
        phases.append(phase)
        jobs.extend(phase_jobs)
        edges.extend(phase_edges)
        cycles.extend(phase_cycles)
        previous_synthesis_id = synthesis_id
    return jobs, edges, cycles, phases


def _track_spec(
    spec: WorkflowGenerationSpec,
    *,
    phase_id: str,
    track_id: str,
    track_shape: str,
    track: JsonObject,
) -> WorkflowGenerationSpec:
    lane_id = track.get("lane_id")
    lanes = dict(spec.lanes)
    if isinstance(lane_id, str):
        if lane_id not in _lane_ids_for(spec):
            raise GeneratorError("track lane_id references missing lane", field_path="spec.options.phases[].tracks[].lane_id")
        lanes = {**lanes, lane_id: spec.lanes.get(lane_id, {})}
    options = dict(spec.options)
    options.pop("phases", None)
    if isinstance(track.get("options"), dict):
        options.update(dict(track["options"]))
    return WorkflowGenerationSpec(
        schema_version=spec.schema_version,
        shape=track_shape,
        lane_set=spec.lane_set,
        workflow_id=f"{spec.workflow_id}-{phase_id}-{track_id}",
        name=f"{spec.name} {phase_id} {track_id}",
        workflow_version=spec.workflow_version,
        branch=dict(spec.branch),
        scaffold_root=spec.scaffold_root,
        artifact_root=f"{spec.artifact_root}/{phase_id}/{track_id}",
        lanes=lanes,
        options=options,
        lane_modifiers=list(spec.lane_modifiers),
        context_docs=list(spec.context_docs),
        parallelism=spec.parallelism,
    )


def _phase_track_jobs(
    jobs: list[JsonObject],
    *,
    id_map: dict[str, str],
    phase_id: str,
    track_id: str,
    parallel_job_ids: set[str],
) -> list[JsonObject]:
    result: list[JsonObject] = []
    for job in jobs:
        remapped = dict(job)
        remapped["id"] = id_map[str(job["id"])]
        remapped["phase_id"] = phase_id
        if remapped["id"] in parallel_job_ids:
            remapped["parallel_group"] = f"{phase_id}:{track_id}"
        result.append(remapped)
    return result


def _apply_track_lane_override(jobs: list[JsonObject], *, id_map: set[str], lane_id: str) -> None:
    for job in jobs:
        if job.get("id") in id_map and job.get("type") != "review":
            job["lane_id"] = lane_id


def _remap_edges(edges: list[JsonObject], *, id_map: dict[str, str]) -> list[JsonObject]:
    return [
        {"from": id_map[str(edge["from"])], "to": id_map[str(edge["to"])], "on": str(edge.get("on", "completed"))}
        for edge in edges
    ]


def _remap_cycles(cycles: list[JsonObject], *, id_map: dict[str, str]) -> list[JsonObject]:
    result: list[JsonObject] = []
    for cycle in cycles:
        remapped = dict(cycle)
        remapped["from"] = id_map[str(cycle["from"])]
        remapped["to"] = id_map[str(cycle["to"])]
        result.append(remapped)
    return result


def _entry_job_ids(jobs: list[JsonObject], edges: list[JsonObject], *, id_map: dict[str, str]) -> list[str]:
    incoming = {str(job["id"]): 0 for job in jobs}
    for edge in edges:
        incoming[str(edge["to"])] += 1
    return [id_map[job_id] for job_id, count in incoming.items() if count == 0]


def _terminal_job_ids(jobs: list[JsonObject], edges: list[JsonObject], *, id_map: dict[str, str]) -> list[str]:
    outgoing = {str(job["id"]): 0 for job in jobs}
    for edge in edges:
        outgoing[str(edge["from"])] += 1
    return [id_map[job_id] for job_id, count in outgoing.items() if count == 0]


def _phase_synthesis_lane(spec: WorkflowGenerationSpec, phase: JsonObject) -> str:
    lane_id = phase.get("synthesis_lane_id")
    lane_ids = _lane_ids_for(spec)
    if isinstance(lane_id, str):
        if lane_id not in lane_ids:
            raise GeneratorError("synthesis_lane_id references missing lane", field_path="spec.options.phases[].synthesis_lane_id")
        return lane_id
    return _reviewer_lane(spec, 1)


def _phase_synthesis_job(
    phase_id: str,
    phase_name: str,
    synthesis_id: str,
    lane_id: str,
    artifact_root: str,
) -> JsonObject:
    job = _job(
        synthesis_id,
        "phase_synthesis",
        f"Synthesize {phase_name}",
        "reviewer",
        lane_id,
        f"{artifact_root}/{phase_id}",
        "SYNTHESIS.md",
        "synthesis",
        "phase_synthesis",
        prompt="apply",
        objective=f"Synthesize the completed work in {phase_name} and record a phase verdict.",
    )
    job["phase_id"] = phase_id
    return job


def _compile_custom(spec: WorkflowGenerationSpec, lanes: JsonObject) -> tuple[list[JsonObject], list[JsonObject], list[JsonObject]]:
    plan = spec.plan or {}
    if plan.get("schema_version") != PLAN_SCHEMA_VERSION:
        raise GeneratorError("custom plan has unsupported schema_version", field_path="spec.plan.schema_version")
    blocks = _object_list(plan.get("blocks", []), "spec.plan.blocks")
    seen: set[str] = set()
    jobs: list[JsonObject] = []
    bindings = _object(plan.get("job_lane_bindings", {}), "spec.plan.job_lane_bindings")
    for index, block in enumerate(blocks):
        block_id = _required_str(block, "id", prefix=f"spec.plan.blocks[{index}]")
        if block_id in seen:
            raise GeneratorError("duplicate custom block id", field_path=f"spec.plan.blocks[{index}].id")
        seen.add(block_id)
        kind = _choice(block, "kind", BLOCK_KINDS, prefix=f"spec.plan.blocks[{index}]")
        lane_id = bindings.get(block_id)
        if not isinstance(lane_id, str):
            raise GeneratorError("custom block missing lane binding", field_path=f"spec.plan.job_lane_bindings.{block_id}")
        if lane_id not in lanes:
            raise GeneratorError("custom lane binding references missing lane", field_path=f"spec.plan.job_lane_bindings.{block_id}")
        if "review_posture" in block and kind not in {"review", "evidence_audit", "final_review"}:
            raise GeneratorError("review-only fields on non-review block", field_path=f"spec.plan.blocks[{index}].review_posture")
        artifact_path = block.get("artifact_path")
        path = str(artifact_path) if isinstance(artifact_path, str) else f"{spec.artifact_root}/{block_id.upper()}.md"
        _safe_artifact_path(path, spec.artifact_root, f"spec.plan.blocks[{index}].artifact_path")
        jobs.append(_custom_job(block_id, kind, lane_id, path, block, index))
    ids = {str(job["id"]) for job in jobs}
    edges = _custom_edges(plan, ids)
    _assert_acyclic(ids, edges)
    cycles = _custom_cycles(plan, ids, jobs)
    return jobs, edges, cycles


def _custom_job(block_id: str, kind: str, lane_id: str, path: str, block: JsonObject, index: int) -> JsonObject:
    review_kinds = {"review", "evidence_audit", "final_review"}
    role = "reviewer" if kind in review_kinds else "author"
    job_type = "review" if kind in review_kinds else ("synthesis" if kind == "synthesis" else "build" if kind in {"implementation", "test", "support_ledger"} else kind)
    artifact_kind = "finding" if role == "reviewer" else ("support_ledger" if kind == "support_ledger" else "handoff")
    job = _job(block_id, job_type, str(block.get("title", block_id.replace("_", " ").title())), role, lane_id, str(PurePosixPath(path).parent), PurePosixPath(path).name, artifact_kind, block_id, prompt=kind)
    if role == "reviewer":
        posture = str(block.get("review_posture", block.get("posture", "neutral")))
        _validate_posture(posture, f"spec.plan.blocks[{index}].posture")
        job["review_posture"] = posture
        job["fresh_session_required"] = True
    return job


def _custom_edges(plan: JsonObject, ids: set[str]) -> list[JsonObject]:
    edges = _object_list(plan.get("edges", []), "spec.plan.edges")
    result: list[JsonObject] = []
    for index, edge in enumerate(edges):
        from_id = _required_str(edge, "from", prefix=f"spec.plan.edges[{index}]")
        to_id = _required_str(edge, "to", prefix=f"spec.plan.edges[{index}]")
        if from_id not in ids:
            raise GeneratorError("edge references missing block", field_path=f"spec.plan.edges[{index}].from")
        if to_id not in ids:
            raise GeneratorError("edge references missing block", field_path=f"spec.plan.edges[{index}].to")
        result.append({"from": from_id, "to": to_id, "on": str(edge.get("on", "completed"))})
    return result


def _custom_cycles(plan: JsonObject, ids: set[str], jobs: list[JsonObject]) -> list[JsonObject]:
    review_ids = {str(job["id"]) for job in jobs if job.get("type") == "review"}
    result: list[JsonObject] = []
    for index, cycle in enumerate(_object_list(plan.get("cycles", []), "spec.plan.cycles")):
        from_id = _required_str(cycle, "from", prefix=f"spec.plan.cycles[{index}]")
        to_id = _required_str(cycle, "to", prefix=f"spec.plan.cycles[{index}]")
        if from_id not in ids:
            raise GeneratorError("cycle references missing block", field_path=f"spec.plan.cycles[{index}].from")
        if to_id not in ids:
            raise GeneratorError("cycle references missing block", field_path=f"spec.plan.cycles[{index}].to")
        if from_id not in review_ids:
            raise GeneratorError("cycle source must be a review block", field_path=f"spec.plan.cycles[{index}].from")
        max_iterations = cycle.get("max_iterations")
        if not isinstance(max_iterations, int) or max_iterations < 1:
            raise GeneratorError("cycle max_iterations must be positive", field_path=f"spec.plan.cycles[{index}].max_iterations")
        result.append({"from": from_id, "to": to_id, "on_verdict": str(cycle.get("on_verdict", "needs_revision")), "max_iterations": max_iterations})
    return result


def _job(
    job_id: str,
    job_type: str,
    title: str,
    role: str,
    lane: str,
    root: str,
    filename: str,
    artifact_kind: str,
    logical_name: str,
    *,
    prompt: str = "draft",
    objective: str | None = None,
) -> JsonObject:
    return {
        "id": job_id,
        "type": job_type,
        "title": title,
        "role_id": role,
        "lane_id": lane,
        "objective": objective or title,
        "task_prompt": {"path": f"prompts/{prompt}.md"},
        "write_scope": {"mode": "repo_write", "repo_write": True, "allowed_paths": [f"{root}/"], "forbidden_paths": [".striatum/"]},
        "expected_artifacts": [{"logical_name": logical_name, "kind": artifact_kind, "path": f"{root}/{filename}", "required": True}],
    }


def _review_job(job_id: str, lane: str, path: str, *, posture: str) -> JsonObject:
    root = str(PurePosixPath(path).parent)
    filename = PurePosixPath(path).name
    job = _job(job_id, "review", "Review the draft", "reviewer", lane, root, filename, "finding", "review", prompt="review")
    job["objective"] = "Review the draft and record a finding."
    job["fresh_session_required"] = True
    job["write_scope"] = {"mode": "review_only_artifact", "repo_write": False, "allowed_paths": [f"{root}/"], "forbidden_paths": [".striatum/"]}
    if posture != "neutral":
        job["review_posture"] = posture
    return job


def _render_files(spec: WorkflowGenerationSpec, workflow: JsonObject, roles: JsonObject) -> list[JsonObject]:
    files: list[JsonObject] = [
        {"path": f"{spec.scaffold_root}/workflow.json", "content": json.dumps(workflow, indent=2, sort_keys=False) + "\n"}
    ]
    for role in sorted(roles):
        files.append({"path": f"{spec.scaffold_root}/roles/{role}.md", "content": _role_stub(role)})
    prompts = {str(job["task_prompt"]["path"]).split("/", 1)[1] for job in workflow["jobs"] if isinstance(job.get("task_prompt"), dict)}
    for prompt in sorted(prompts):
        files.append({"path": f"{spec.scaffold_root}/prompts/{prompt}", "content": _prompt_stub(prompt)})
    return files


def _role_stub(role: str) -> str:
    if role == "reviewer":
        return (
            "# Reviewer Role\n\n"
            "You are the reviewer for this workflow. Read the upstream draft "
            "and write a single review-only finding artifact at the declared "
            "path; do not modify other files.\n"
        )
    return (
        "# Author Role\n\n"
        "You are the author for this workflow. Produce the expected handoff "
        "artifact at the path declared in the workflow. Stay inside the "
        "declared write scope.\n"
    )


def _prompt_stub(prompt: str) -> str:
    stubs = {
        "draft.md": (
            "Draft the initial artifact described by the workflow. Replace this "
            "stub with the concrete authoring instructions for your team.\n"
        ),
        "review.md": (
            "Review the upstream draft and record a finding with one of the "
            "supported verdicts. Replace this stub with reviewer guidance.\n"
        ),
        "apply.md": (
            "Apply the accepted review by producing the final synthesis "
            "artifact. Replace this stub with concrete apply instructions.\n"
        ),
    }
    return stubs.get(prompt, f"Complete the {prompt.removesuffix('.md').replace('_', ' ')} step declared by the workflow.\n")


def _apply_lane_modifiers(spec: WorkflowGenerationSpec, lanes: JsonObject, *, repo_write_lanes: set[str]) -> JsonObject:
    result = {lane_id: dict(lane) for lane_id, lane in lanes.items()}
    if "worktree_isolated" in spec.lane_modifiers:
        for lane_id in repo_write_lanes:
            if lane_id in result:
                result[lane_id]["worktree_isolation"] = "per_job"
    if "constrained" in spec.lane_modifiers:
        constraints = _constraints(spec)
        required = _required_enforcement(spec)
        for lane in result.values():
            lane["constraints"] = constraints
            if required:
                lane["required_enforcement"] = required
    if "harness_profiled" in spec.lane_modifiers:
        profiles = _harness_profiles(spec)
        profile_ids = list(profiles)
        for index, lane_id in enumerate(sorted(result)):
            result[lane_id]["harness_profile_id"] = profile_ids[min(index, len(profile_ids) - 1)]
    return result


def _validate_modifier_matrix(spec: WorkflowGenerationSpec, warnings: list[str]) -> None:
    for index, modifier in enumerate(spec.lane_modifiers):
        if modifier in {"supervised", "harness_profiled"} and spec.lane_set == "local":
            raise GeneratorError("lane modifier is incompatible with lane set", field_path=f"spec.lane_modifiers[{index}]", hint=f"modifier {modifier!r} is forbidden for lane_set 'local'")
        if modifier == "worktree_isolated" and spec.shape == "multi_review_synthesis":
            warnings.append("worktree_isolated has no effect on review-only jobs except synthesis")
    if "harness_profiled" in spec.lane_modifiers:
        _harness_profiles(spec)
    if "constrained" in spec.lane_modifiers:
        _constraints(spec)
        _required_enforcement(spec)


def _harness_profiles(spec: WorkflowGenerationSpec) -> JsonObject:
    profiles = _object(spec.options.get("harness_profiles"), "spec.options.harness_profiles")
    if not profiles:
        raise GeneratorError("harness_profiled requires options.harness_profiles", field_path="spec.options.harness_profiles")
    for profile_id, body in profiles.items():
        if not isinstance(body, dict):
            raise GeneratorError("harness profile body must be an object", field_path=f"spec.options.harness_profiles.{profile_id}")
        family = body.get("tool_family")
        if family not in HARNESS_PROFILE_TOOL_FAMILIES:
            raise GeneratorError("harness profile has unknown tool_family", field_path=f"spec.options.harness_profiles.{profile_id}.tool_family")
    return {
        str(key): _enrich_harness_profile_body(dict(value))
        for key, value in profiles.items()
        if isinstance(value, dict)
    }


def _enrich_harness_profile_body(body: JsonObject) -> JsonObject:
    """Fill the RFC 0040 V1 ``native_delegation`` defaults from the catalog.

    Looks up the per-tool-family fragment in the workflow-template catalog
    and applies ``native_delegation.mode`` + ``native_delegation.instruction``
    when the caller didn't already specify them. Other keys are preserved
    verbatim.
    """
    from striatum.workflow_generator.catalog import get_harness_fragment_by_tool_family

    fragment = get_harness_fragment_by_tool_family(str(body.get("tool_family", "")))
    if fragment is None:
        return body
    native = body.get("native_delegation")
    if not isinstance(native, dict):
        native = {}
    else:
        native = dict(native)
    if "instruction" not in native or not isinstance(native.get("instruction"), str) or not str(native["instruction"]).strip():
        native["instruction"] = str(fragment["native_delegation_instruction"])
    if "mode" not in native and isinstance(fragment.get("native_delegation_mode"), str):
        native["mode"] = str(fragment["native_delegation_mode"])
    body["native_delegation"] = native
    return body


def _constraints(spec: WorkflowGenerationSpec) -> JsonObject:
    constraints = _object(spec.options.get("constraints", {}), "spec.options.constraints")
    for key, value in constraints.items():
        if key not in CONSTRAINT_VALUES or value not in CONSTRAINT_VALUES[key]:
            raise GeneratorError("unknown adapter constraint value", field_path=f"spec.options.constraints.{key}")
    return dict(constraints)


def _required_enforcement(spec: WorkflowGenerationSpec) -> JsonObject:
    required = _object(spec.options.get("required_enforcement", {}), "spec.options.required_enforcement")
    for key, value in required.items():
        if key not in CONSTRAINT_VALUES or value not in ADAPTER_ENFORCEMENT_LEVELS:
            raise GeneratorError("unknown required enforcement value", field_path=f"spec.options.required_enforcement.{key}")
    return dict(required)


def _roles(role_ids: list[str]) -> JsonObject:
    return {role: {"definition_path": f"roles/{role}.md"} for role in role_ids}


def _coordinator(spec: WorkflowGenerationSpec, lanes: JsonObject) -> JsonObject:
    lane = "local" if "local" in lanes else ("author" if "author" in lanes else sorted(lanes)[0])
    return {"role_id": "author", "lane_id": lane}


def _parallelism(spec: WorkflowGenerationSpec) -> JsonObject:
    max_jobs = _reviewer_count(spec) if spec.shape == "multi_review_synthesis" else 1
    return {"mode": "declared", "max_active_jobs": max_jobs, "require_disjoint_write_scopes": True}


def _author_lane(spec: WorkflowGenerationSpec) -> str:
    if spec.lane_set == "local":
        return "local"
    if spec.lane_set == "single_agent":
        return "agent"
    return "author"


def _reviewer_lane(spec: WorkflowGenerationSpec, index: int) -> str:
    if spec.lane_set == "local":
        return "local"
    if spec.lane_set == "single_agent":
        return "agent"
    if spec.lane_set == "multi_review":
        return f"reviewer_{index}"
    return "reviewer"


def _reviewer_count(spec: WorkflowGenerationSpec) -> int:
    value = spec.options.get("reviewer_count")
    if isinstance(value, int) and value > 0:
        return value
    postures = spec.options.get("review_postures")
    if isinstance(postures, list) and postures:
        return len(postures)
    return 2


def _postures(spec: WorkflowGenerationSpec, count: int) -> list[str]:
    raw = spec.options.get("review_postures")
    values = [str(item) for item in raw] if isinstance(raw, list) and raw else ["devils_advocate", "security"]
    for index, posture in enumerate(values):
        _validate_posture(posture, f"spec.options.review_postures[{index}]")
    while len(values) < count:
        values.append("neutral")
    return values[:count]


def _first_posture(spec: WorkflowGenerationSpec) -> str:
    return _postures(spec, 1)[0] if "review_postures" in spec.options else "neutral"


def _max_cycles(spec: WorkflowGenerationSpec) -> int:
    value = spec.options.get("max_revision_cycles", 1)
    if not isinstance(value, int) or value < 1:
        raise GeneratorError("max_revision_cycles must be a positive integer", field_path="spec.options.max_revision_cycles")
    return value


def _validate_posture(posture: str, field_path: str) -> None:
    if posture in ALLOWED_POSTURES:
        return
    if posture.startswith("custom:") and posture.removeprefix("custom:").strip():
        return
    raise GeneratorError("invalid review posture", field_path=field_path)


def _assert_acyclic(ids: set[str], edges: list[JsonObject]) -> None:
    incoming = {item: 0 for item in ids}
    outgoing: dict[str, list[str]] = {item: [] for item in ids}
    for edge in edges:
        outgoing[str(edge["from"])].append(str(edge["to"]))
        incoming[str(edge["to"])] += 1
    ready = [item for item, count in incoming.items() if count == 0]
    visited = 0
    while ready:
        item = ready.pop()
        visited += 1
        for downstream in outgoing[item]:
            incoming[downstream] -= 1
            if incoming[downstream] == 0:
                ready.append(downstream)
    if visited != len(ids):
        raise GeneratorError("custom plan base edges contain a cycle", field_path="spec.plan.edges")


def _safe_artifact_path(path: str, root: str, field_path: str) -> None:
    safe = _safe_path(path, field_path)
    root_prefix = root.rstrip("/") + "/"
    if safe != root and not safe.startswith(root_prefix):
        raise GeneratorError("derived artifact path escapes artifact root", field_path=field_path)


def _safe_path(value: str, field_path: str) -> str:
    if not value or "\x00" in value or value.startswith("/") or "\\" in value:
        raise GeneratorError("path must be repo-relative", field_path=field_path)
    path = PurePosixPath(value)
    if ".." in path.parts or ".git" in path.parts or ".striatum" in path.parts:
        raise GeneratorError("path must not escape the repository or target .git/.striatum", field_path=field_path)
    return path.as_posix().rstrip("/")


def _lanes(value: Any, *, lane_set: str) -> dict[str, JsonObject]:
    if value is None:
        if lane_set == "local":
            return {}
        raise GeneratorError("lanes are required for this lane_set", field_path="spec.lanes")
    raw = _object(value, "spec.lanes")
    return {str(key): _object(body, f"spec.lanes.{key}") for key, body in raw.items()}


def _required_str(raw: JsonObject, key: str, *, prefix: str = "spec") -> str:
    value = raw.get(key)
    if not isinstance(value, str) or not value:
        raise GeneratorError(f"{key} must be a non-empty string", field_path=f"{prefix}.{key}")
    return value


def _choice(raw: JsonObject, key: str, choices: frozenset[str], *, prefix: str = "spec") -> str:
    value = _required_str(raw, key, prefix=prefix)
    if value not in choices:
        raise GeneratorError(f"{key} must be one of {sorted(choices)!r}", field_path=f"{prefix}.{key}")
    return value


def _required_object(raw: JsonObject, key: str) -> JsonObject:
    if key not in raw:
        raise GeneratorError(f"{key} is required", field_path=f"spec.{key}")
    return _object(raw[key], f"spec.{key}")


def _object(value: Any, field_path: str) -> JsonObject:
    if not isinstance(value, dict):
        raise GeneratorError("value must be an object", field_path=field_path)
    return dict(value)


def _object_list(value: Any, field_path: str) -> list[JsonObject]:
    if not isinstance(value, list) or not all(isinstance(item, dict) for item in value):
        raise GeneratorError("value must be a list of objects", field_path=field_path)
    return [dict(item) for item in value]


def _str_list(value: Any, field_path: str) -> list[str]:
    if not isinstance(value, list) or not all(isinstance(item, str) and item for item in value):
        raise GeneratorError("value must be a list of non-empty strings", field_path=field_path)
    return list(value)


def slugify(value: str) -> str:
    """Return a conservative workflow id/path slug."""
    slug = re.sub(r"[^A-Za-z0-9_.-]+", "-", value.strip()).strip("-").lower()
    return slug or "generated-workflow"

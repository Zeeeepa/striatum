from __future__ import annotations

from pathlib import Path
from typing import Any, cast

from striatum.artifact_contracts import ALLOWED_ARTIFACT_KINDS
from striatum.workflow import load_workflow, validate_workflow


ROOT = Path(__file__).resolve().parents[1]
THREE_LANE_FIXTURE = ROOT / "examples" / "three-lane-design-build-review"
THREE_LANE_WORKFLOW = THREE_LANE_FIXTURE / "workflow.json"
IMPLEMENTATION_PANEL_FIXTURE = ROOT / "examples" / "implementation-panel-flow"
IMPLEMENTATION_PANEL_WORKFLOW = IMPLEMENTATION_PANEL_FIXTURE / "workflow.json"


def _load_three_lane_fixture() -> dict[str, Any]:
    return load_workflow(THREE_LANE_WORKFLOW)


def _load_implementation_panel_fixture() -> dict[str, Any]:
    return load_workflow(IMPLEMENTATION_PANEL_WORKFLOW)


def test_three_lane_design_build_review_fixture_validates() -> None:
    workflow = _load_three_lane_fixture()

    validate_workflow(workflow)

    assert workflow["workflow_id"] == "three-lane-design-build-review"

    jobs = cast(list[dict[str, Any]], workflow["jobs"])
    assert {job["id"] for job in jobs} == {
        "design_codex",
        "design_claude",
        "design_gemini",
        "synth",
        "review_design",
        "implement",
        "review_build_codex",
        "review_build_claude",
        "review_build_gemini",
    }

    edges = cast(list[dict[str, Any]], workflow["edges"])
    assert {(edge["from"], edge["to"]) for edge in edges} == {
        ("design_codex", "synth"),
        ("design_claude", "synth"),
        ("design_gemini", "synth"),
        ("synth", "review_design"),
        ("review_design", "implement"),
        ("implement", "review_build_codex"),
        ("implement", "review_build_claude"),
        ("implement", "review_build_gemini"),
    }

    cycles = cast(list[dict[str, Any]], workflow["cycles"])
    assert {(cycle["from"], cycle["to"]) for cycle in cycles} == {
        ("review_design", "synth"),
        ("review_build_codex", "implement"),
        ("review_build_claude", "implement"),
        ("review_build_gemini", "implement"),
    }
    assert all(cycle["on_verdict"] == "needs_revision" for cycle in cycles)
    assert all(cycle["max_iterations"] == 2 for cycle in cycles)
    assert all(cycle["allow_same_lane"] is True for cycle in cycles)


def test_three_lane_design_build_review_referenced_files_exist() -> None:
    workflow = _load_three_lane_fixture()
    referenced_paths: set[str] = set()

    roles = cast(dict[str, dict[str, Any]], workflow["roles"])
    for role in roles.values():
        referenced_paths.add(cast(str, role["definition_path"]))

    context_docs = cast(list[dict[str, Any]], workflow["context_docs"])
    for doc in context_docs:
        referenced_paths.add(cast(str, doc["path"]))

    jobs = cast(list[dict[str, Any]], workflow["jobs"])
    for job in jobs:
        prompt = job.get("task_prompt")
        if isinstance(prompt, dict) and isinstance(prompt.get("path"), str):
            referenced_paths.add(prompt["path"])

    for rel_path in sorted(referenced_paths):
        path = Path(rel_path)
        assert not path.is_absolute(), rel_path
        assert ".." not in path.parts, rel_path
        assert (THREE_LANE_FIXTURE / path).is_file(), rel_path


def test_implementation_panel_flow_fixture_validates() -> None:
    workflow = _load_implementation_panel_fixture()

    validate_workflow(workflow)

    assert workflow["workflow_id"] == "implementation-panel-flow"

    jobs = cast(list[dict[str, Any]], workflow["jobs"])
    assert [job["id"] for job in jobs] == [
        "frame_problem",
        "propose_option_a",
        "propose_option_b",
        "propose_option_c",
        "score_option_a",
        "score_option_b",
        "score_option_c",
        "compile_tradeoffs",
        "arbitrate",
        "review_dissent",
        "record_decision",
    ]

    edges = cast(list[dict[str, Any]], workflow["edges"])
    assert {(edge["from"], edge["to"]) for edge in edges} == {
        ("frame_problem", "propose_option_a"),
        ("frame_problem", "propose_option_b"),
        ("frame_problem", "propose_option_c"),
        ("propose_option_a", "score_option_a"),
        ("propose_option_b", "score_option_b"),
        ("propose_option_c", "score_option_c"),
        ("score_option_a", "compile_tradeoffs"),
        ("score_option_b", "compile_tradeoffs"),
        ("score_option_c", "compile_tradeoffs"),
        ("compile_tradeoffs", "arbitrate"),
        ("arbitrate", "review_dissent"),
        ("review_dissent", "record_decision"),
    }

    cycles = cast(list[dict[str, Any]], workflow["cycles"])
    assert cycles == [
        {
            "from": "review_dissent",
            "to": "arbitrate",
            "on_verdict": "needs_revision",
            "max_iterations": 1,
            "allow_same_lane": True,
        }
    ]


def test_implementation_panel_flow_referenced_files_exist() -> None:
    workflow = _load_implementation_panel_fixture()
    fixture_relative_paths: set[str] = set()

    roles = cast(dict[str, dict[str, Any]], workflow["roles"])
    for role in roles.values():
        fixture_relative_paths.add(cast(str, role["definition_path"]))

    jobs = cast(list[dict[str, Any]], workflow["jobs"])
    for job in jobs:
        prompt = job.get("task_prompt")
        if isinstance(prompt, dict) and isinstance(prompt.get("path"), str):
            fixture_relative_paths.add(prompt["path"])

    for rel_path in sorted(fixture_relative_paths):
        path = Path(rel_path)
        assert not path.is_absolute(), rel_path
        assert ".." not in path.parts, rel_path
        assert (IMPLEMENTATION_PANEL_FIXTURE / path).is_file(), rel_path

    context_docs = cast(list[dict[str, Any]], workflow["context_docs"])
    for doc in context_docs:
        rel_path = cast(str, doc["path"])
        path = Path(rel_path)
        assert not path.is_absolute(), rel_path
        assert ".." not in path.parts, rel_path
        assert (ROOT / path).is_file(), rel_path


def test_implementation_panel_flow_artifact_paths_disjoint() -> None:
    workflow = _load_implementation_panel_fixture()
    jobs = cast(list[dict[str, Any]], workflow["jobs"])

    artifact_paths: set[str] = set()
    allowed_paths: set[str] = set()
    for job in jobs:
        scope = cast(dict[str, Any], job["write_scope"])
        assert scope["forbidden_paths"] == [".striatum/"]
        job_allowed = cast(list[str], scope["allowed_paths"])
        assert len(job_allowed) == 1
        allowed_path = job_allowed[0]
        assert allowed_path.startswith("examples/implementation-panel-flow/artifacts/")
        assert allowed_path not in allowed_paths
        allowed_paths.add(allowed_path)

        artifacts = cast(list[dict[str, Any]], job["expected_artifacts"])
        assert len(artifacts) == 1
        artifact_path = cast(str, artifacts[0]["path"])
        assert artifact_path == allowed_path
        assert artifact_path not in artifact_paths
        artifact_paths.add(artifact_path)


def test_implementation_panel_flow_uses_existing_artifact_kinds() -> None:
    workflow = _load_implementation_panel_fixture()
    jobs = cast(list[dict[str, Any]], workflow["jobs"])

    kinds = {
        cast(str, artifact["kind"])
        for job in jobs
        for artifact in cast(list[dict[str, Any]], job["expected_artifacts"])
    }

    assert kinds == {"decision", "finding", "findings_ledger", "handoff", "synthesis"}
    assert kinds <= ALLOWED_ARTIFACT_KINDS

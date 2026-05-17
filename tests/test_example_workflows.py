from __future__ import annotations

from pathlib import Path
from typing import Any, cast

from striatum.workflow import load_workflow, validate_workflow


ROOT = Path(__file__).resolve().parents[1]
THREE_LANE_FIXTURE = ROOT / "examples" / "three-lane-design-build-review"
THREE_LANE_WORKFLOW = THREE_LANE_FIXTURE / "workflow.json"


def _load_three_lane_fixture() -> dict[str, Any]:
    return load_workflow(THREE_LANE_WORKFLOW)


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

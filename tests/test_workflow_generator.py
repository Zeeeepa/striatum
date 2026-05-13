from __future__ import annotations

import json
from pathlib import Path

from striatum.cli.dispatch import main
from striatum.workflow import validate_workflow
from striatum.workflow_generator import GeneratorError, generate_workflow
from striatum.workflow_generator.catalog import (
    get_harness_fragment,
    get_harness_fragment_by_tool_family,
    get_template,
    list_harness_fragments,
    list_templates,
)
from striatum.workflow_generator.core import GENERATOR_SCHEMA_VERSION


def _spec(shape: str, lane_set: str = "local") -> dict[str, object]:
    lanes: dict[str, object] = {}
    if lane_set == "single_agent":
        lanes = {"agent": {"command": ["agent", "run"], "display_model": "Agent"}}
    elif lane_set == "author_reviewer":
        lanes = {
            "author": {"command": ["author", "run"], "display_model": "Author"},
            "reviewer": {"command": ["reviewer", "run"], "display_model": "Reviewer"},
        }
    elif lane_set == "multi_review":
        lanes = {
            "author": {"command": ["author", "run"], "display_model": "Author"},
            "reviewer_1": {"command": ["reviewer1", "run"], "display_model": "Reviewer 1"},
            "reviewer_2": {"command": ["reviewer2", "run"], "display_model": "Reviewer 2"},
        }
    return {
        "schema_version": GENERATOR_SCHEMA_VERSION,
        "shape": shape,
        "lane_set": lane_set,
        "workflow_id": f"{shape}-test",
        "name": f"{shape} test",
        "workflow_version": "2026-05-12",
        "branch": {"mode": "confirm", "suggested_name": f"striatum/{shape}", "allow_dirty": False},
        "scaffold_root": f"workflows/{shape}",
        "artifact_root": f"striatum/{shape}",
        "lanes": lanes,
        "options": {},
    }


def test_catalog_lists_and_shows_templates() -> None:
    templates = list_templates()
    ids = [item["template_id"] for item in templates]
    assert "code_change" in ids
    assert "multi_phase" in ids
    assert get_template("author_reviewer")["kind"] == "lane_set"


def test_builtin_shapes_validate() -> None:
    cases = [
        ("minimal", "local"),
        ("review", "local"),
        ("code_change", "local"),
        ("human_checkpoint", "author_reviewer"),
        ("evidence_backed", "author_reviewer"),
        ("multi_review_synthesis", "multi_review"),
    ]
    for shape, lane_set in cases:
        generated = generate_workflow(_spec(shape, lane_set))
        validate_workflow(generated.workflow)
        assert generated.validation["ok"] is True
        assert generated.metadata["shape"] == shape


def test_multi_phase_shape_emits_v1_1_phased_graph() -> None:
    spec = _spec("multi_phase", "author_reviewer")
    spec["options"] = {
        "phases": [
            {
                "id": "phase_1_design",
                "name": "Design",
                "description": "Parallel design tracks",
                "color": "#6b7280",
                "tracks": [
                    {"id": "python", "shape": "minimal", "lane_id": "author"},
                    {"id": "docs", "shape": "review", "lane_id": "author"},
                ],
                "synthesis_lane_id": "reviewer",
            },
            {
                "id": "phase_2_build",
                "name": "Build",
                "tracks": [
                    {"id": "python", "shape": "minimal", "lane_id": "author"},
                    {"id": "docs", "shape": "minimal", "lane_id": "author"},
                ],
                "synthesis_lane_id": "reviewer",
            },
        ]
    }
    generated = generate_workflow(spec)
    workflow = generated.workflow

    validate_workflow(workflow)
    assert workflow["schema_version"] == "striatum.workflow.v1.1"
    assert [phase["id"] for phase in workflow["phases"]] == ["phase_1_design", "phase_2_build"]

    jobs = {job["id"]: job for job in workflow["jobs"]}
    assert jobs["phase_1_design__python__draft"]["phase_id"] == "phase_1_design"
    assert jobs["phase_1_design__python__draft"]["parallel_group"] == "phase_1_design:python"
    assert jobs["phase_1_design__synthesis"]["type"] == "phase_synthesis"
    assert jobs["phase_1_design__synthesis"]["phase_id"] == "phase_1_design"
    assert jobs["phase_2_build__synthesis"]["type"] == "phase_synthesis"

    edges = {(edge["from"], edge["to"]) for edge in workflow["edges"]}
    assert ("phase_1_design__synthesis", "phase_2_build__python__draft") in edges
    assert ("phase_1_design__synthesis", "phase_2_build__docs__draft") in edges
    assert ("phase_1_design__python__draft", "phase_1_design__synthesis") in edges


def test_modifier_refusal_carries_field_path() -> None:
    spec = _spec("review", "local")
    spec["lane_modifiers"] = ["supervised"]
    try:
        generate_workflow(spec)
    except GeneratorError as exc:
        assert exc.field_path == "spec.lane_modifiers[0]"
    else:  # pragma: no cover
        raise AssertionError("expected GeneratorError")


def test_custom_plan_compiles_closed_blocks() -> None:
    spec = _spec("custom", "author_reviewer")
    spec["plan"] = {
        "schema_version": "striatum.workflow_plan.v1",
        "blocks": [
            {"id": "draft", "kind": "draft"},
            {"id": "review", "kind": "review", "review_posture": "security"},
        ],
        "edges": [{"from": "draft", "to": "review", "on": "completed"}],
        "cycles": [{"from": "review", "to": "draft", "max_iterations": 1}],
        "job_lane_bindings": {"draft": "author", "review": "reviewer"},
    }
    generated = generate_workflow(spec)
    validate_workflow(generated.workflow)
    assert generated.workflow["cycles"][0]["on_verdict"] == "needs_revision"


def test_cli_workflow_generate_dry_run_and_write(tmp_path: Path, capsys) -> None:  # type: ignore[no-untyped-def]
    rc = main([
        "--repo",
        str(tmp_path),
        "workflow",
        "generate",
        "workflows/demo",
        "--shape",
        "review",
        "--lane-set",
        "local",
        "--artifact-root",
        "striatum/demo",
        "--dry-run",
        "--json",
    ])
    assert rc == 0
    dry_run = json.loads(capsys.readouterr().out)
    assert dry_run["data"]["workflow"]["workflow_id"] == "demo"
    assert not (tmp_path / "workflows" / "demo").exists()

    rc = main([
        "--repo",
        str(tmp_path),
        "workflow",
        "generate",
        "workflows/demo",
        "--shape",
        "review",
        "--lane-set",
        "local",
        "--artifact-root",
        "striatum/demo",
        "--json",
    ])
    assert rc == 0
    assert (tmp_path / "workflows" / "demo" / "workflow.json").exists()


def test_workflow_init_keeps_legacy_envelope(tmp_path: Path, capsys) -> None:  # type: ignore[no-untyped-def]
    target = tmp_path / "wf"
    rc = main(["--repo", str(tmp_path), "workflow", "init", "--style", "code-change", str(target), "--json"])
    assert rc == 0
    payload = json.loads(capsys.readouterr().out)
    assert set(payload["data"]) == {"status", "path", "workflow_path", "style", "files"}
    assert payload["data"]["style"] == "code-change"
    workflow = json.loads((target / "workflow.json").read_text(encoding="utf-8"))
    assert workflow["cycles"][0]["on_verdict"] == "needs_revision"


# --- RFC 0040 V1: harness-profile fragments -------------------------


def test_catalog_lists_harness_fragments() -> None:
    fragments = list_harness_fragments()
    ids = {fragment["profile_id"] for fragment in fragments}
    assert {"claude_code_default", "codex_default", "gemini_default", "generic_default"}.issubset(ids)


def test_get_harness_fragment_by_tool_family_lookup() -> None:
    fragment = get_harness_fragment_by_tool_family("claude_code")
    assert fragment is not None
    assert fragment["profile_id"] == "claude_code_default"
    assert "one-shot supervised invocation" in fragment["native_delegation_instruction"]


def test_gemini_fragment_carries_front_matter_callout() -> None:
    fragment = get_harness_fragment("gemini_default")
    instruction = fragment["native_delegation_instruction"]
    assert "ALL FIVE front-matter fields" in instruction
    assert "verdict_intent" in instruction
    assert "severity from {low,medium,high,critical}" in instruction


def test_harness_profiled_generator_inlines_catalog_instructions() -> None:
    spec = _spec("review", "author_reviewer")
    spec["lane_modifiers"] = ["harness_profiled"]
    spec["options"] = {
        "harness_profiles": {
            "claude_code_default": {
                "tool_family": "claude_code",
                "strategy_version": "2026-05-13",
            }
        }
    }
    generated = generate_workflow(spec)
    profile = generated.workflow["harness_profiles"]["claude_code_default"]
    catalog = get_harness_fragment("claude_code_default")
    assert profile["native_delegation"]["instruction"] == catalog["native_delegation_instruction"]
    assert profile["native_delegation"]["mode"] == catalog["native_delegation_mode"]


def test_harness_profiled_generator_preserves_operator_override() -> None:
    spec = _spec("review", "author_reviewer")
    spec["lane_modifiers"] = ["harness_profiled"]
    custom = "Operator-specific guidance for this lane."
    spec["options"] = {
        "harness_profiles": {
            "claude_code_default": {
                "tool_family": "claude_code",
                "strategy_version": "2026-05-13",
                "native_delegation": {"mode": "encouraged", "instruction": custom},
            }
        }
    }
    generated = generate_workflow(spec)
    profile = generated.workflow["harness_profiles"]["claude_code_default"]
    assert profile["native_delegation"]["instruction"] == custom
    assert profile["native_delegation"]["mode"] == "encouraged"

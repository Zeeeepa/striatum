"""Tests for RFC 0010 V1 harness profile validation and packet exposure."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

import pytest

from striatum.errors import WorkflowError
from striatum.workflow import plan_workflow, validate_workflow

ROOT = Path(__file__).resolve().parents[1]
FIXTURE_WORKFLOW = ROOT / "examples" / "harness-profiles" / "workflow.json"
DOGFOOD_003_WORKFLOW = ROOT / "docs" / "dogfood" / "003" / "workflow.json"

JsonDict = dict[str, Any]


def _load(path: Path) -> JsonDict:
    return cast(JsonDict, json.loads(path.read_text(encoding="utf-8")))


def _minimal_workflow() -> JsonDict:
    """Return the smallest valid workflow we can mutate per test."""
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "test-harness-profiles",
        "workflow_version": "2026-05-08",
        "name": "Test",
        "branch": {
            "mode": "confirm",
            "suggested_name": "striatum/test-harness-profiles",
        },
        "coordinator": {"role_id": "author", "lane_id": "alpha"},
        "lanes": {
            "alpha": {
                "adapter": "process",
                "command": ["true"],
                "capabilities": ["write"],
            }
        },
        "roles": {"author": {"definition_path": "roles/author.md"}},
        "context_docs": [],
        "parallelism": {"mode": "declared", "max_active_jobs": 1},
        "jobs": [
            {
                "id": "demo",
                "type": "generic",
                "title": "Demo",
                "role_id": "author",
                "lane_id": "alpha",
                "objective": "demo",
                "task_prompt": {"path": "prompts/demo.md"},
                "write_scope": {
                    "mode": "repo_write",
                    "repo_write": True,
                    "allowed_paths": ["docs/test/"],
                    "forbidden_paths": [".striatum/"],
                },
                "expected_artifacts": [
                    {
                        "logical_name": "out",
                        "kind": "handoff",
                        "path": "docs/test/OUT.md",
                        "required": True,
                    }
                ],
            }
        ],
        "edges": [],
        "cycles": [],
    }


def _profile() -> JsonDict:
    return {
        "tool_family": "generic",
        "strategy_version": "2026-05-08",
        "native_delegation": {"mode": "off"},
        "feature_flags": {"subagents": "off"},
        "accountability": {
            "native_subagents": "internal_to_parent_session",
            "first_class_registration": "not_supported",
        },
    }


def _run_cli(repo: Path, *args: str, check: bool = True) -> JsonDict:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(
            f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}"
        )
    if result.stdout.strip() == "":
        return {"returncode": result.returncode}
    payload = cast(JsonDict, json.loads(result.stdout))
    payload["returncode"] = result.returncode
    return payload


def _data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def _start_run_with(repo: Path, workflow: JsonDict) -> tuple[str, str]:
    workflow_path = repo / "workflow.json"
    workflow_path.write_text(json.dumps(workflow), encoding="utf-8")
    from striatum.db import init_repo as legacy_init_repo

    legacy_init_repo(repo)
    prepared = _data(_run_cli(repo, "run", "prepare", "--workflow", str(workflow_path)))
    run_id = str(prepared["run_id"])
    _run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/test-hp")
    _run_cli(repo, "run", "start", "--run-id", run_id)
    session = _data(
        _run_cli(
            repo,
            "register-session",
            "--run-id",
            run_id,
            "--role",
            "author",
            "--lane",
            "alpha",
            "--capability",
            "write",
        )
    )
    return run_id, str(session["session_id"])


# ----- workflows without harness_profiles still validate (backwards compat) ---


def test_workflow_without_harness_profiles_validates() -> None:
    workflow = _minimal_workflow()
    validate_workflow(workflow)
    plan = plan_workflow(workflow)
    assert plan["valid"] is True
    assert "warnings" not in plan


# ----- valid harness_profiles + lane reference ----------------------------


def test_valid_harness_profile_and_lane_reference() -> None:
    workflow = _minimal_workflow()
    workflow["harness_profiles"] = {"generic_default": _profile()}
    workflow["lanes"]["alpha"]["harness_profile_id"] = "generic_default"
    validate_workflow(workflow)
    plan = plan_workflow(workflow)
    assert plan["valid"] is True
    assert "warnings" not in plan


# ----- profile body validation -------------------------------------------


def test_profile_must_be_object() -> None:
    workflow = _minimal_workflow()
    workflow["harness_profiles"] = {"x": "not an object"}
    with pytest.raises(WorkflowError, match="body must be an object"):
        validate_workflow(workflow)


def test_profile_missing_tool_family() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    del body["tool_family"]
    workflow["harness_profiles"] = {"x": body}
    with pytest.raises(WorkflowError, match="missing required fields"):
        validate_workflow(workflow)


def test_profile_unknown_tool_family() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    body["tool_family"] = "frobnicator"
    workflow["harness_profiles"] = {"x": body}
    with pytest.raises(WorkflowError, match="unknown tool_family"):
        validate_workflow(workflow)


def test_profile_missing_strategy_version() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    del body["strategy_version"]
    workflow["harness_profiles"] = {"x": body}
    with pytest.raises(WorkflowError, match="missing required fields"):
        validate_workflow(workflow)


def test_profile_rejects_first_class_native_subagents() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    body["accountability"]["native_subagents"] = "first_class_session"
    workflow["harness_profiles"] = {"x": body}
    with pytest.raises(WorkflowError, match="native_subagents must be"):
        validate_workflow(workflow)


def test_profile_rejects_non_internal_first_class_registration() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    body["accountability"]["first_class_registration"] = "supported"
    workflow["harness_profiles"] = {"x": body}
    with pytest.raises(WorkflowError, match="first_class_registration must be"):
        validate_workflow(workflow)


def test_profile_envelope_path_must_be_repo_relative() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    body["prompt_envelope_path"] = "/etc/passwd"
    workflow["harness_profiles"] = {"x": body}
    with pytest.raises(WorkflowError, match="prompt_envelope_path must be repo-relative"):
        validate_workflow(workflow)


def test_profile_envelope_path_rejects_dotdot() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    body["prompt_envelope_path"] = "envs/../etc/passwd"
    workflow["harness_profiles"] = {"x": body}
    with pytest.raises(WorkflowError, match="prompt_envelope_path must be repo-relative"):
        validate_workflow(workflow)


def test_profile_fallback_must_reference_declared_profile() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    body["fallback_profile_id"] = "missing_profile"
    workflow["harness_profiles"] = {"x": body}
    with pytest.raises(WorkflowError, match="fallback_profile_id"):
        validate_workflow(workflow)


# ----- lane reference validation ------------------------------------------


def test_lane_reference_to_undeclared_profile_raises() -> None:
    workflow = _minimal_workflow()
    workflow["lanes"]["alpha"]["harness_profile_id"] = "missing_profile"
    with pytest.raises(WorkflowError, match="references undeclared harness profile"):
        validate_workflow(workflow)


def test_lane_with_empty_profile_reference_raises() -> None:
    workflow = _minimal_workflow()
    workflow["harness_profiles"] = {"generic_default": _profile()}
    workflow["lanes"]["alpha"]["harness_profile_id"] = ""
    with pytest.raises(WorkflowError, match="harness_profile_id must be a non-empty string"):
        validate_workflow(workflow)


# ----- lint-warning behaviour ---------------------------------------------


def test_unknown_sibling_field_produces_warning_not_error() -> None:
    workflow = _minimal_workflow()
    body = _profile()
    body["mystery_field"] = "value"
    workflow["harness_profiles"] = {"x": body}
    workflow["lanes"]["alpha"]["harness_profile_id"] = "x"
    # No warnings collector → silently accepted (lint posture).
    validate_workflow(workflow)
    # With a collector → warning surfaces.
    warnings: list[str] = []
    validate_workflow(workflow, warnings=warnings)
    assert any("mystery_field" in w for w in warnings)
    plan = plan_workflow(workflow)
    assert plan["valid"] is True
    assert "warnings" in plan
    assert any("mystery_field" in w for w in plan["warnings"])


# ----- work-packet exposure ------------------------------------------------


def test_packet_exposes_harness_profile_for_referenced_lane(tmp_path: Path) -> None:
    workflow = _minimal_workflow()
    workflow["harness_profiles"] = {"generic_default": _profile()}
    workflow["lanes"]["alpha"]["harness_profile_id"] = "generic_default"

    _run_id, session_id = _start_run_with(tmp_path, workflow)
    payload = _data(_run_cli(tmp_path, "claim-next", "--session-id", session_id))
    assert payload["status"] == "claimed"
    packet = payload["packet"]
    assert isinstance(packet, dict)
    assert "harness_profile" in packet
    profile = packet["harness_profile"]
    assert profile["profile_id"] == "generic_default"
    assert profile["tool_family"] == "generic"
    assert profile["strategy_version"] == "2026-05-08"
    assert profile["accountability"]["native_subagents"] == "internal_to_parent_session"


def test_packet_omits_harness_profile_when_lane_has_no_reference(tmp_path: Path) -> None:
    workflow = _minimal_workflow()
    # No harness_profiles declared; no lane reference.

    _run_id, session_id = _start_run_with(tmp_path, workflow)
    payload = _data(_run_cli(tmp_path, "claim-next", "--session-id", session_id))
    assert payload["status"] == "claimed"
    packet = payload["packet"]
    assert isinstance(packet, dict)
    assert "harness_profile" not in packet


# ----- V1.5 missing-lane-command-path lint --------------------------------


def test_missing_repo_relative_lane_command_emits_warning(tmp_path: Path) -> None:
    workflow = _minimal_workflow()
    workflow["lanes"]["alpha"]["command"] = ["./bin/missing.sh"]
    warnings: list[str] = []
    validate_workflow(workflow, warnings=warnings, repo_root=tmp_path)
    assert any("missing.sh" in w for w in warnings)


def test_missing_lane_command_warning_skipped_without_repo_root() -> None:
    """The lint warning only fires when a repo_root is supplied."""
    workflow = _minimal_workflow()
    workflow["lanes"]["alpha"]["command"] = ["./bin/missing.sh"]
    warnings: list[str] = []
    validate_workflow(workflow, warnings=warnings)
    assert not any("missing.sh" in w for w in warnings)


def test_present_lane_command_does_not_warn(tmp_path: Path) -> None:
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    target = bin_dir / "wrapper.sh"
    target.write_text("#!/bin/sh\n", encoding="utf-8")
    workflow = _minimal_workflow()
    workflow["lanes"]["alpha"]["command"] = ["./bin/wrapper.sh"]
    warnings: list[str] = []
    validate_workflow(workflow, warnings=warnings, repo_root=tmp_path)
    assert not any("wrapper.sh" in w for w in warnings)


def test_bare_binary_name_does_not_warn(tmp_path: Path) -> None:
    """Bare binaries (resolved via $PATH) are not checked."""
    workflow = _minimal_workflow()
    workflow["lanes"]["alpha"]["command"] = ["codex", "exec", "-"]
    warnings: list[str] = []
    validate_workflow(workflow, warnings=warnings, repo_root=tmp_path)
    assert not any("codex" in w for w in warnings)


def test_absolute_path_does_not_warn(tmp_path: Path) -> None:
    """Absolute paths are not checked by the lint (system binaries)."""
    workflow = _minimal_workflow()
    workflow["lanes"]["alpha"]["command"] = ["/usr/bin/totally-missing-binary"]
    warnings: list[str] = []
    validate_workflow(workflow, warnings=warnings, repo_root=tmp_path)
    assert not any("totally-missing-binary" in w for w in warnings)


def test_workflow_validate_cli_surfaces_missing_lane_command_warning(tmp_path: Path) -> None:
    workflow = _minimal_workflow()
    workflow["lanes"]["alpha"]["command"] = [".striatum/bin/missing-wrapper.sh"]
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(workflow), encoding="utf-8")
    payload = _data(_run_cli(tmp_path, "workflow", "validate", str(workflow_path)))
    assert payload["valid"] is True
    assert "warnings" in payload
    assert any("missing-wrapper.sh" in w for w in payload["warnings"])


# ----- CLI envelope warnings ----------------------------------------------


def test_workflow_validate_cli_surfaces_warnings(tmp_path: Path) -> None:
    workflow = _minimal_workflow()
    body = _profile()
    body["mystery_field"] = "value"
    workflow["harness_profiles"] = {"generic_default": body}
    workflow["lanes"]["alpha"]["harness_profile_id"] = "generic_default"
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(json.dumps(workflow), encoding="utf-8")
    payload = _data(_run_cli(tmp_path, "workflow", "validate", str(workflow_path)))
    assert payload["valid"] is True
    assert "warnings" in payload
    assert any("mystery_field" in w for w in payload["warnings"])


# ----- fixture loading -----------------------------------------------------


def test_examples_harness_profiles_fixture_validates() -> None:
    fixture = _load(FIXTURE_WORKFLOW)
    validate_workflow(fixture)
    plan = plan_workflow(fixture)
    assert plan["valid"] is True
    assert "warnings" not in plan


def test_dogfood_003_fixture_validates_with_four_profiles() -> None:
    """Per design-review F4: load the dogfood-003 workflow itself."""
    fixture = _load(DOGFOOD_003_WORKFLOW)
    validate_workflow(fixture)
    profiles = fixture.get("harness_profiles", {})
    assert set(profiles.keys()) == {
        "generic_default",
        "codex_default",
        "claude_code_default",
        "gemini_cli_default",
    }

"""RFC 0040 V1 tests: `striatum workflow upgrade <path>`.

Coverage:

- Dry run reports the change set without writing.
- Default upgrade fills empty native_delegation.instruction.
- Refuse-on-conflict when an existing non-default instruction is present.
- --force overwrites a conflicting instruction.
- No-changes path when profiles are already at the catalog default.
- Refuse-on-running-workflow when a non-terminal run references the path.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import Any

import pytest

import striatum.cli.workflow as workflow_mod
from striatum.api import invoke
from striatum.cli.workflow import workflow_upgrade
from striatum.errors import WorkflowError
from striatum.workflow_generator.catalog import get_harness_fragment


def _baseline_workflow(profile_instruction: str | None = None) -> dict[str, Any]:
    profile_body: dict[str, Any] = {
        "tool_family": "claude_code",
        "strategy_version": "2026-05-13",
    }
    if profile_instruction is not None:
        profile_body["native_delegation"] = {"instruction": profile_instruction}
    return {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "demo",
        "workflow_version": "2026-05-12",
        "name": "Demo",
        "branch": {"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": False},
        "coordinator": {"role_id": "author", "lane_id": "claude_code"},
        "lanes": {
            "claude_code": {
                "adapter": "process",
                "display_model": "Claude",
                "command": ["claude", "--print"],
                "capabilities": ["write"],
                "harness_profile_id": "claude_code_default",
            }
        },
        "roles": {"author": {"definition_path": "roles/author.md"}},
        "context_docs": [],
        "parallelism": {"mode": "declared", "max_active_jobs": 1, "require_disjoint_write_scopes": True},
        "jobs": [
            {
                "id": "draft",
                "type": "draft",
                "title": "Draft",
                "role_id": "author",
                "lane_id": "claude_code",
                "objective": "Draft.",
                "task_prompt": {"path": "prompts/draft.md"},
                "write_scope": {"mode": "repo_write", "repo_write": True, "allowed_paths": ["scratch/"], "forbidden_paths": [".striatum/"]},
                "expected_artifacts": [{"logical_name": "draft", "kind": "handoff", "path": "scratch/DRAFT.md", "required": True}],
            }
        ],
        "edges": [],
        "cycles": [],
        "harness_profiles": {"claude_code_default": profile_body},
    }


def _parallel_group_workflow() -> dict[str, Any]:
    workflow = _baseline_workflow()
    workflow["roles"]["reviewer"] = {"definition_path": "roles/reviewer.md"}
    workflow["jobs"] = [
        {
            "id": "design_python",
            "type": "draft",
            "title": "Python Design",
            "role_id": "author",
            "lane_id": "claude_code",
            "parallel_group": "design_python",
            "objective": "Draft design.",
            "task_prompt": {"path": "prompts/design.md"},
            "write_scope": {"mode": "repo_write", "repo_write": True, "allowed_paths": ["scratch/design/"], "forbidden_paths": [".striatum/"]},
            "expected_artifacts": [{"logical_name": "design", "kind": "handoff", "path": "scratch/design/DESIGN.md", "required": True}],
        },
        {
            "id": "build_python",
            "type": "build",
            "title": "Python Build",
            "role_id": "author",
            "lane_id": "claude_code",
            "parallel_group": "build_python",
            "objective": "Build implementation.",
            "task_prompt": {"path": "prompts/build.md"},
            "write_scope": {"mode": "repo_write", "repo_write": True, "allowed_paths": ["scratch/build/"], "forbidden_paths": [".striatum/"]},
            "expected_artifacts": [{"logical_name": "build", "kind": "handoff", "path": "scratch/build/BUILD.md", "required": True}],
        },
    ]
    workflow["edges"] = [{"from": "design_python", "to": "build_python", "on": "completed"}]
    return workflow


def _write_workflow(repo: Path, workflow: dict[str, Any]) -> Path:
    path = repo / "workflow.json"
    path.write_text(json.dumps(workflow, indent=2) + "\n", encoding="utf-8")
    return path


def _git_init_repo(repo: Path) -> None:
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    subprocess.run(["git", "checkout", "-qb", "main"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.email", "t@e.com"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.name", "t"], cwd=repo, check=True)


def _striatum_init(repo: Path) -> None:
    """Initialize .striatum/state.sqlite3 so the running-workflow guard
    has somewhere to look.
    """
    subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), "init"],
        cwd=repo,
        check=True,
        capture_output=True,
    )


# --- happy paths ------------------------------------------------------


def test_upgrade_fills_missing_instruction(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(tmp_path, _baseline_workflow())
    result = workflow_upgrade(path, repo=tmp_path)
    catalog_instruction = get_harness_fragment("claude_code_default")["native_delegation_instruction"]
    assert result["status"] == "updated"
    fields = {(c["profile_id"], c["field"]) for c in result["changes"]}
    assert ("claude_code_default", "native_delegation.instruction") in fields
    on_disk = json.loads(path.read_text(encoding="utf-8"))
    assert (
        on_disk["harness_profiles"]["claude_code_default"]["native_delegation"]["instruction"]
        == catalog_instruction
    )


def test_upgrade_dry_run_writes_nothing(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(tmp_path, _baseline_workflow())
    snapshot = path.read_text(encoding="utf-8")
    result = workflow_upgrade(path, repo=tmp_path, dry_run=True)
    assert result["status"] == "would_update"
    assert result["changes"]
    assert path.read_text(encoding="utf-8") == snapshot


def test_upgrade_already_default_is_no_op(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    fragment = get_harness_fragment("claude_code_default")
    workflow = _baseline_workflow(profile_instruction=fragment["native_delegation_instruction"])
    workflow["harness_profiles"]["claude_code_default"]["native_delegation"]["mode"] = fragment["native_delegation_mode"]
    path = _write_workflow(tmp_path, workflow)
    result = workflow_upgrade(path, repo=tmp_path)
    assert result["status"] == "no_changes"
    assert not result["changes"]


# --- conflict handling ------------------------------------------------


def test_upgrade_refuses_on_conflict_without_force(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(
        tmp_path,
        _baseline_workflow(profile_instruction="A hand-tuned instruction the operator authored."),
    )
    snapshot = path.read_text(encoding="utf-8")
    result = workflow_upgrade(path, repo=tmp_path)
    assert result["status"] == "refused_conflict"
    assert result["conflicts"]
    assert path.read_text(encoding="utf-8") == snapshot


def test_upgrade_force_overwrites_conflict(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    catalog_instruction = get_harness_fragment("claude_code_default")["native_delegation_instruction"]
    path = _write_workflow(
        tmp_path,
        _baseline_workflow(profile_instruction="A hand-tuned instruction the operator authored."),
    )
    result = workflow_upgrade(path, repo=tmp_path, force=True)
    assert result["status"] == "updated"
    assert all(c.get("forced") for c in result["changes"] if c["field"] == "native_delegation.instruction")
    on_disk = json.loads(path.read_text(encoding="utf-8"))
    assert (
        on_disk["harness_profiles"]["claude_code_default"]["native_delegation"]["instruction"]
        == catalog_instruction
    )


# --- running-workflow guard ------------------------------------------


def test_upgrade_refuses_when_workflow_has_running_run(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(tmp_path, _baseline_workflow())
    prep = invoke(["run", "prepare", "--workflow", str(path)], repo=tmp_path)
    assert prep.get("ok") is True, prep
    with pytest.raises(WorkflowError) as exc_info:
        workflow_upgrade(path, repo=tmp_path)
    assert "non-terminal runs" in str(exc_info.value)


def test_upgrade_dry_run_reports_running_runs(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(tmp_path, _baseline_workflow())
    prep = invoke(["run", "prepare", "--workflow", str(path)], repo=tmp_path)
    assert prep.get("ok") is True, prep
    result = workflow_upgrade(path, repo=tmp_path, dry_run=True)
    assert result["status"] == "would_refuse_running"
    assert result["running_runs"]


def test_upgrade_refuses_running_runs_from_pg_when_sqlite_absent(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    _git_init_repo(tmp_path)
    path = _write_workflow(tmp_path, _baseline_workflow())
    monkeypatch.setattr(
        workflow_mod,
        "_running_runs_for_workflow_pg",
        lambda **_kwargs: ["run_pg_active"],
    )

    with pytest.raises(WorkflowError) as exc_info:
        workflow_upgrade(path, repo=tmp_path)

    assert "run_pg_active" in str(exc_info.value)


def test_upgrade_fails_closed_after_sqlite_cutover_when_pg_unavailable(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    _git_init_repo(tmp_path)
    path = _write_workflow(tmp_path, _baseline_workflow())
    striatum_dir = tmp_path / ".striatum"
    striatum_dir.mkdir()
    (striatum_dir / "state.sqlite3.migrated").write_text("", encoding="utf-8")
    monkeypatch.setattr(
        workflow_mod,
        "_running_runs_for_workflow_pg",
        lambda **_kwargs: None,
    )

    with pytest.raises(WorkflowError, match="daemon PostgreSQL was unavailable"):
        workflow_upgrade(path, repo=tmp_path)


def test_upgrade_refuses_repo_local_sqlite_fallback_outside_test_harness(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    _git_init_repo(tmp_path)
    path = _write_workflow(tmp_path, _baseline_workflow())
    state_db = tmp_path / ".striatum" / "state.sqlite3"
    state_db.parent.mkdir()
    state_db.write_bytes(b"not a database")
    monkeypatch.setattr(
        workflow_mod,
        "_running_runs_for_workflow_pg",
        lambda **_kwargs: None,
    )
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "1")
    monkeypatch.setattr(
        workflow_mod.sqlite3,
        "connect",
        lambda *_args, **_kwargs: pytest.fail("workflow upgrade opened repo-local SQLite"),
    )

    with pytest.raises(WorkflowError, match="paired test-harness compatibility escape"):
        workflow_upgrade(path, repo=tmp_path)


# --- target validation ------------------------------------------------


def test_upgrade_refuses_missing_path(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    with pytest.raises(WorkflowError):
        workflow_upgrade(tmp_path / "missing.json", repo=tmp_path)


def test_upgrade_handles_workflow_with_no_harness_profiles(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    workflow = _baseline_workflow()
    workflow.pop("harness_profiles")
    workflow["lanes"]["claude_code"].pop("harness_profile_id")
    path = _write_workflow(tmp_path, workflow)
    result = workflow_upgrade(path, repo=tmp_path)
    assert result["status"] == "no_changes"
    assert result.get("note")


def test_upgrade_via_cli_dispatch(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(tmp_path, _baseline_workflow())
    result = invoke(["workflow", "upgrade", str(path), "--dry-run"], repo=tmp_path)
    assert result["ok"] is True
    assert result["data"]["status"] == "would_update"


# --- RFC 0045 add-phases ---------------------------------------------


def test_upgrade_add_phases_preview_writes_nothing_without_apply(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(tmp_path, _parallel_group_workflow())
    snapshot = path.read_text(encoding="utf-8")
    result = workflow_upgrade(path, repo=tmp_path, add_phases=True)
    assert result["status"] == "would_update"
    assert result["mode"] == "add_phases"
    assert result["phases_added"] == [
        {
            "id": "phase_design",
            "name": "Design",
            "synthesis_job_id": "phase_design__synthesis",
        },
        {
            "id": "phase_build",
            "name": "Build",
            "synthesis_job_id": "phase_build__synthesis",
        },
    ]
    assert {"job_id": "design_python", "phase_id": "phase_design"} in result["jobs_relabelled"]
    assert path.read_text(encoding="utf-8") == snapshot


def test_upgrade_add_phases_apply_rewrites_to_v1_1(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(tmp_path, _parallel_group_workflow())
    result = workflow_upgrade(path, repo=tmp_path, add_phases=True, apply=True)
    assert result["status"] == "updated"
    on_disk = json.loads(path.read_text(encoding="utf-8"))
    assert on_disk["schema_version"] == "striatum.workflow.v1.1"
    assert on_disk["phases"] == [
        {
            "id": "phase_design",
            "name": "Design",
            "synthesis_job_id": "phase_design__synthesis",
        },
        {
            "id": "phase_build",
            "name": "Build",
            "synthesis_job_id": "phase_build__synthesis",
        },
    ]
    jobs = {job["id"]: job for job in on_disk["jobs"]}
    assert jobs["design_python"]["phase_id"] == "phase_design"
    assert jobs["build_python"]["phase_id"] == "phase_build"
    assert jobs["phase_design__synthesis"]["type"] == "phase_synthesis"
    assert jobs["phase_build__synthesis"]["type"] == "phase_synthesis"
    edges = {(edge["from"], edge["to"]) for edge in on_disk["edges"]}
    assert ("design_python", "phase_design__synthesis") in edges
    assert ("phase_design__synthesis", "build_python") in edges
    assert ("build_python", "phase_build__synthesis") in edges


def test_upgrade_add_phases_via_cli_dispatch_requires_apply_to_write(tmp_path: Path) -> None:
    _git_init_repo(tmp_path)
    _striatum_init(tmp_path)
    path = _write_workflow(tmp_path, _parallel_group_workflow())
    preview = invoke(["workflow", "upgrade", str(path), "--add-phases"], repo=tmp_path)
    assert preview["ok"] is True
    assert preview["data"]["status"] == "would_update"
    assert json.loads(path.read_text(encoding="utf-8"))["schema_version"] == "striatum.workflow.v1"
    applied = invoke(["workflow", "upgrade", str(path), "--add-phases", "--apply"], repo=tmp_path)
    assert applied["ok"] is True
    assert applied["data"]["status"] == "updated"
    assert json.loads(path.read_text(encoding="utf-8"))["schema_version"] == "striatum.workflow.v1.1"

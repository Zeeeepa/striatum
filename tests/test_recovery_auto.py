"""RFC 0020 V1 tests: `recovery auto` sweep + recovery_policy validator.

These exercise the public sweep envelope shape, the safe-class
classifier (D036: no autonomous repo-write requeue), the policy
validator (workflow-level + CLI overrides + defaults), and the
hook runners under sandboxed conditions.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

from test_cli_mvp import (
    data,
    init_repo,
    run_cli,
)
from test_recovery_extended import _drive_to_human_checkpoint


JsonDict = dict[str, Any]


# ----- recovery_policy validator (workflow.py wiring) -------------------


def test_recovery_policy_omitted_validates() -> None:
    from striatum.recovery.policy import validate_recovery_policy

    validate_recovery_policy(None)  # no-op


def test_recovery_policy_unknown_hook_kind_rejected() -> None:
    from striatum.errors import WorkflowError
    from striatum.recovery.policy import validate_recovery_policy

    with pytest.raises(WorkflowError):
        validate_recovery_policy(
            {"escalation_hook": {"kind": "carrier-pigeon"}}
        )


def test_recovery_policy_negative_threshold_rejected() -> None:
    from striatum.errors import WorkflowError
    from striatum.recovery.policy import validate_recovery_policy

    with pytest.raises(WorkflowError):
        validate_recovery_policy({"checkpoint_timeout_seconds": -1})


def test_recovery_policy_marker_file_inside_striatum_rejected() -> None:
    from striatum.errors import WorkflowError
    from striatum.recovery.policy import validate_recovery_policy

    with pytest.raises(WorkflowError):
        validate_recovery_policy(
            {
                "escalation_hook": {
                    "kind": "marker_file",
                    "path": ".striatum/STALL.md",
                }
            }
        )


def test_recovery_policy_marker_file_traversal_rejected() -> None:
    from striatum.errors import WorkflowError
    from striatum.recovery.policy import validate_recovery_policy

    with pytest.raises(WorkflowError):
        validate_recovery_policy(
            {"escalation_hook": {"kind": "marker_file", "path": "../escape.md"}}
        )


def test_recovery_policy_webhook_requires_url_scheme() -> None:
    from striatum.errors import WorkflowError
    from striatum.recovery.policy import validate_recovery_policy

    with pytest.raises(WorkflowError):
        validate_recovery_policy(
            {"escalation_hook": {"kind": "webhook", "url": "ftp://x"}}
        )


def test_recovery_policy_shell_requires_command_list() -> None:
    from striatum.errors import WorkflowError
    from striatum.recovery.policy import validate_recovery_policy

    with pytest.raises(WorkflowError):
        validate_recovery_policy(
            {"escalation_hook": {"kind": "shell", "command": "rm -rf"}}
        )


def test_resolve_policy_defaults_when_workflow_omits() -> None:
    from striatum.recovery.policy import DEFAULT_POLICY, resolve_policy

    resolved = resolve_policy(workflow_payload=None)
    assert resolved["policy_source"] == "default"
    for key, value in DEFAULT_POLICY.items():
        assert resolved[key] == value


def test_resolve_policy_workflow_block_marks_source_workflow() -> None:
    from striatum.recovery.policy import resolve_policy

    resolved = resolve_policy(
        workflow_payload={
            "autonomous_review_requeue": True,
            "checkpoint_timeout_seconds": 300,
        }
    )
    assert resolved["policy_source"] == "workflow"
    assert resolved["autonomous_review_requeue"] is True
    assert resolved["checkpoint_timeout_seconds"] == 300


def test_resolve_policy_cli_override_marks_source_cli_override() -> None:
    from striatum.recovery.policy import resolve_policy

    resolved = resolve_policy(
        workflow_payload={"autonomous_review_requeue": False},
        cli_overrides={"autonomous_review_requeue": True},
    )
    assert resolved["policy_source"] == "cli_override"
    assert resolved["autonomous_review_requeue"] is True


# ----- hook runners (sandboxed) -----------------------------------------


def test_marker_file_hook_writes_into_target(tmp_path: Path) -> None:
    from striatum.recovery.hooks import run_marker_file_hook

    result = run_marker_file_hook(
        target=tmp_path, path="striatum/STALLED.md", body="hello"
    )
    assert result["wrote"] is True
    written = (tmp_path / "striatum/STALLED.md").read_text(encoding="utf-8")
    assert written.startswith("hello")


def test_marker_file_hook_refuses_striatum_path(tmp_path: Path) -> None:
    from striatum.errors import WorkflowError
    from striatum.recovery.hooks import run_marker_file_hook

    with pytest.raises(WorkflowError):
        run_marker_file_hook(
            target=tmp_path, path=".striatum/STALL.md", body="x"
        )


def test_marker_file_hook_refuses_traversal(tmp_path: Path) -> None:
    from striatum.errors import WorkflowError
    from striatum.recovery.hooks import run_marker_file_hook

    with pytest.raises(WorkflowError):
        run_marker_file_hook(target=tmp_path, path="../bad.md", body="x")


def test_webhook_hook_failure_returns_status_dict() -> None:
    from striatum.recovery.hooks import run_webhook_hook

    result = run_webhook_hook(
        url="http://127.0.0.1:1/does-not-exist",
        payload={"x": 1},
        timeout=0.5,
    )
    assert result["wrote"] is False
    assert "error" in result


def test_webhook_hook_rejects_non_http_scheme() -> None:
    from striatum.recovery.hooks import run_webhook_hook

    result = run_webhook_hook(url="ftp://x", payload={}, timeout=0.5)
    assert result["wrote"] is False
    assert result["error"] == "scheme"


def test_shell_hook_runs_through_subprocess(tmp_path: Path) -> None:
    from striatum.recovery.hooks import run_shell_hook

    result = run_shell_hook(command=["/bin/true"], cwd=tmp_path)
    assert result["wrote"] is True
    assert result["exit_code"] == 0


def test_shell_hook_empty_command_no_op() -> None:
    from striatum.recovery.hooks import run_shell_hook

    result = run_shell_hook(command=[])
    assert result["wrote"] is False


# ----- recovery auto end-to-end -----------------------------------------


def test_recovery_auto_dry_run_returns_envelope_no_state_change(tmp_path: Path) -> None:
    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    result = data(run_cli(tmp_path, "recovery", "auto", "--run-id", run_id, "--dry-run"))
    assert result["dry_run"] is True
    assert result["run_id"] == run_id
    assert result["policy_source"] in {"default", "workflow", "cli_override"}
    assert isinstance(result["actions"], list)
    assert isinstance(result["escalations"], list)
    assert isinstance(result["still_stuck"], list)


def test_recovery_auto_envelope_lists_eligible_human_checkpoint(
    tmp_path: Path,
) -> None:
    """A human_checkpoint blocker that's been open longer than the
    timeout shows up in `escalations`."""
    run_id, _, _, _, blocker_id = _drive_to_human_checkpoint(tmp_path)

    # CLI override the timeout to 0 so the just-opened blocker
    # immediately ages past it.
    result = data(run_cli(
        tmp_path, "recovery", "auto",
        "--run-id", run_id,
        "--checkpoint-timeout", "0",
    ))
    assert any(
        e.get("blocker_id") == blocker_id and e.get("kind") == "checkpoint_timeout"
        for e in result["escalations"]
    ), result


def test_recovery_auto_no_policy_omits_autonomous_actions(tmp_path: Path) -> None:
    """Without `--autonomous-review-requeue`, sweep emits no
    review_requeued actions even if stale review-only work exists."""
    run_id, _, _, _, _ = _drive_to_human_checkpoint(tmp_path)
    result = data(run_cli(tmp_path, "recovery", "auto", "--run-id", run_id))
    requeued = [a for a in result["actions"] if a.get("kind") == "review_requeued"]
    assert requeued == []


def test_recovery_auto_workflow_validate_accepts_recovery_policy(tmp_path: Path) -> None:
    """A workflow with a valid `recovery_policy` block validates."""
    init_repo(tmp_path)
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "rp-fixture",
        "workflow_version": "1",
        "name": "rp",
        "branch": {"mode": "auto", "suggested_name": "x"},
        "coordinator": {"role_id": "r", "lane_id": "codex"},
        "lanes": {"codex": {"adapter": "process", "command": ["true"]}},
        "roles": {"r": {"definition_path": "x.md"}},
        "context_docs": [],
        "parallelism": {"mode": "declared", "max_active_jobs": 1, "require_disjoint_write_scopes": True},
        "jobs": [{
            "id": "j",
            "type": "build",
            "title": "j",
            "role_id": "r",
            "lane_id": "codex",
            "objective": "x",
            "task_prompt": {"path": "x.md"},
            "write_scope": {"mode": "repo_write", "repo_write": True, "allowed_paths": ["x/"], "forbidden_paths": []},
            "expected_artifacts": [],
        }],
        "edges": [],
        "cycles": [],
        "recovery_policy": {
            "autonomous_review_requeue": True,
            "checkpoint_timeout_seconds": 60,
            "escalation_hook": {
                "kind": "marker_file",
                "path": "striatum/test/STALLED.md",
            },
        },
    }
    wf_path = tmp_path / "wf.json"
    wf_path.write_text(json.dumps(workflow), encoding="utf-8")
    out = data(run_cli(tmp_path, "workflow", "validate", str(wf_path)))
    assert out["valid"] is True

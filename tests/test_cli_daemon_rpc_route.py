from __future__ import annotations

import argparse
from pathlib import Path

import pytest

from striatum.cli.daemon_rpc_route import _LOOKUP, try_route
from striatum.errors import StriatumError


def _route(command: str, subcommand: str | None, **kwargs: object) -> tuple[str, dict[str, object]]:
    args = argparse.Namespace(command=command, **kwargs)
    translator = _LOOKUP[(command, subcommand)]
    return translator(args, Path("/repo"))


def test_list_runs_preserves_state_and_limit_filters() -> None:
    method, params = _route(
        "list",
        None,
        list_command="runs",
        state="running",
        limit=25,
    )

    assert method == "list.runs"
    assert params == {"state": "running", "limit": 25}


def test_list_sessions_preserves_run_role_lane_and_state_filters() -> None:
    method, params = _route(
        "list",
        None,
        list_command="sessions",
        run_id="run-1",
        state="active",
        role="reviewer",
        lane="codex",
    )

    assert method == "list.sessions"
    assert params == {
        "run_id": "run-1",
        "state": "active",
        "role": "reviewer",
        "lane": "codex",
    }


def test_list_jobs_preserves_run_state_and_workflow_job_filters() -> None:
    method, params = _route(
        "list",
        None,
        list_command="jobs",
        run_id="run-1",
        state="queued",
        workflow_job_id="build",
    )

    assert method == "list.jobs"
    assert params == {
        "run_id": "run-1",
        "state": "queued",
        "workflow_job_id": "build",
    }


def test_list_artifacts_preserves_run_and_kind_filters() -> None:
    method, params = _route(
        "list",
        None,
        list_command="artifacts",
        run_id="run-1",
        kind="finding",
    )

    assert method == "list.artifacts"
    assert params == {"run_id": "run-1", "kind": "finding"}


def test_list_workflows_preserves_limit_filter() -> None:
    method, params = _route(
        "list",
        None,
        list_command="workflows",
        limit=10,
    )

    assert method == "list.workflows"
    assert params == {"limit": 10}


def test_corpus_export_routes_to_daemon_rpc() -> None:
    method, params = _route(
        "corpus",
        "export",
        corpus_command="export",
        since="HEAD~1",
        out="tmp/corpus",
    )

    assert method == "corpus.export"
    assert params == {"since": "HEAD~1", "out": "tmp/corpus"}


def test_heartbeat_preserves_extend_seconds() -> None:
    method, params = _route(
        "heartbeat",
        None,
        session_id="sess_1",
        lease_id="lease_1",
        extend_seconds=900,
    )

    assert method == "work.heartbeat"
    assert params == {"session_id": "sess_1", "lease_id": "lease_1", "extend_seconds": 900}


def test_release_preserves_message_and_requeue() -> None:
    method, params = _route(
        "release",
        None,
        session_id="sess_1",
        message_id="msg_1",
        lease_id="lease_1",
        reason="yielding",
        requeue=True,
    )

    assert method == "work.release"
    assert params == {
        "session_id": "sess_1",
        "message_id": "msg_1",
        "lease_id": "lease_1",
        "reason": "yielding",
        "requeue": True,
    }


def test_block_preserves_blocker_fields() -> None:
    method, params = _route(
        "block",
        None,
        session_id="sess_1",
        job_id="job_1",
        lease_id="lease_1",
        kind="process_outputs_missing",
        severity="blocked",
        description="missing output",
    )

    assert method == "work.block"
    assert params == {
        "session_id": "sess_1",
        "job_id": "job_1",
        "lease_id": "lease_1",
        "kind": "process_outputs_missing",
        "severity": "blocked",
        "description": "missing output",
    }


def test_register_session_preserves_non_fresh_override() -> None:
    method, params = _route(
        "register-session",
        None,
        run_id="run_1",
        role="reviewer",
        lane="codex",
        fresh=False,
        capability=["review"],
        force_non_fresh=True,
        reason="operator inspected shared context",
        operator_label="local.operator",
    )

    assert method == "session.register"
    assert params == {
        "run_id": "run_1",
        "role": "reviewer",
        "lane": "codex",
        "fresh": False,
        "capability": ["review"],
        "force_non_fresh": True,
        "reason": "operator inspected shared context",
        "operator_label": "local.operator",
    }


def test_submit_review_preserves_artifact_and_verdict_fields() -> None:
    method, params = _route(
        "submit-review",
        None,
        session_id="sess_1",
        job_id="job_review",
        lease_id="lease_1",
        path="docs/review.md",
        verdict="accept_with_findings",
        logical_name="review",
        kind="finding",
        rationale="looks good",
        findings_artifact_id=None,
        allow_no_process_execution=True,
        override_rationale="operator-on-behalf",
    )

    assert method == "review.submit"
    assert params == {
        "session_id": "sess_1",
        "job_id": "job_review",
        "lease_id": "lease_1",
        "path": "docs/review.md",
        "verdict": "accept_with_findings",
        "logical_name": "review",
        "kind": "finding",
        "rationale": "looks good",
        "findings_artifact_id": None,
        "allow_no_process_execution": True,
        "override_rationale": "operator-on-behalf",
    }


def test_verdict_preserves_findings_artifact_id() -> None:
    method, params = _route(
        "verdict",
        None,
        session_id="sess_1",
        job_id="job_review",
        lease_id="lease_1",
        verdict="needs_revision",
        findings_artifact_id="artifact_1",
        rationale="needs work",
    )

    assert method == "review.verdict"
    assert params == {
        "session_id": "sess_1",
        "job_id": "job_review",
        "lease_id": "lease_1",
        "verdict": "needs_revision",
        "findings_artifact_id": "artifact_1",
        "rationale": "needs work",
    }


def test_requeue_stale_preserves_force_justification() -> None:
    method, params = _route(
        "recovery",
        "requeue-stale",
        recovery_command="requeue-stale",
        run_id="run_1",
        job_id="job_1",
        force=True,
        justification="inspected worktree",
    )

    assert method == "recovery.requeue_stale"
    assert params == {
        "run_id": "run_1",
        "job_id": "job_1",
        "force": True,
        "justification": "inspected worktree",
    }


def test_recovery_resume_preserves_blocker_and_completion_fields() -> None:
    method, params = _route(
        "recovery",
        "resume",
        recovery_command="resume",
        blocker_id="blk_1",
        complete=True,
        session_id="sess_1",
        summary="operator remediated",
        force=True,
        extend_seconds=600,
    )

    assert method == "recovery.resume"
    assert params == {
        "blocker_id": "blk_1",
        "session_id": "sess_1",
        "force": True,
        "complete": True,
        "summary": "operator remediated",
        "extend_seconds": 600,
    }


def test_branch_confirm_preserves_branch_mode_flags() -> None:
    method, params = _route(
        "branch",
        "confirm",
        branch_command="confirm",
        run_id="run_1",
        branch="work/rfc-48",
        create=True,
        use_current=False,
        strict=True,
    )

    assert method == "branch.confirm"
    assert params == {
        "run_id": "run_1",
        "branch": "work/rfc-48",
        "create": True,
        "use_current": False,
        "strict": True,
    }


def test_run_retry_job_routes() -> None:
    method, params = _route(
        "run",
        "retry-job",
        run_command="retry-job",
        run_id="run_1",
        job_id="job_1",
    )

    assert method == "run.retry_job"
    assert params == {"run_id": "run_1", "job_id": "job_1"}


def test_decision_record_preserves_run_path_and_followup() -> None:
    method, params = _route(
        "decision",
        "record",
        decision_command="record",
        run_id="run_1",
        path="docs/decisions/D1.md",
        title="Ship it",
        outcome="accepted_with_follow_up",
        decision_id="dec_1",
        rationale="acceptable",
        follow_up="file issue",
    )

    assert method == "decision.record"
    assert params == {
        "run_id": "run_1",
        "path": "docs/decisions/D1.md",
        "title": "Ship it",
        "outcome": "accepted_with_follow_up",
        "decision_id": "dec_1",
        "rationale": "acceptable",
        "follow_up": "file issue",
    }


def test_checkpoint_resolve_preserves_blocker_action_and_decision() -> None:
    method, params = _route(
        "checkpoint",
        "resolve",
        checkpoint_command="resolve",
        blocker_id="blk_1",
        action="continue",
        decision_id="dec_1",
    )

    assert method == "checkpoint.resolve"
    assert params == {
        "blocker_id": "blk_1",
        "action": "continue",
        "decision_id": "dec_1",
    }


def test_registered_rpc_command_fails_closed_when_daemon_unreachable(monkeypatch: pytest.MonkeyPatch) -> None:
    args = argparse.Namespace(command="status", run_id="run_1")
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.resolve_socket_path", lambda: Path("/tmp/missing.sock"))
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.daemon_socket_is_reachable", lambda _path: False)

    with pytest.raises(StriatumError, match="daemon_unreachable"):
        try_route(args, Path("/repo"))


def test_registered_rpc_command_fails_closed_when_repo_unregistered(monkeypatch: pytest.MonkeyPatch) -> None:
    args = argparse.Namespace(command="status", run_id="run_1")
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.resolve_socket_path", lambda: Path("/tmp/striatumd.sock"))
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.daemon_socket_is_reachable", lambda _path: True)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route._lookup_repository_id", lambda _repo: None)

    with pytest.raises(StriatumError, match="repo_not_registered"):
        try_route(args, Path("/repo"))

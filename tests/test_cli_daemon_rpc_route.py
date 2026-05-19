from __future__ import annotations

import argparse
import importlib
import json
from pathlib import Path
from typing import Any, Mapping

import pytest

import striatum
from striatum.cli.daemon_rpc_route import (
    _LOOKUP,
    _call_with_handshake,
    _format_repo_list_table,
    _load_cli_route_contracts,
    _resolve_repository_id,
    _short_repository_id,
    _subcommand,
    try_route,
)
from striatum.daemon_rpc.registry import METHOD_REGISTRY
from striatum.daemon_rpc.server import LOCAL_FILE_AUTHORING_METHODS
from striatum.errors import StriatumError

WORKFLOW_AUTHORING_KEYS: frozenset[tuple[str, str | None]] = frozenset(
    {
        ("workflow", "validate"),
        ("workflow", "plan"),
        ("workflow", "graph"),
        ("workflow", "templates"),
        ("workflow", "init"),
        ("workflow", "generate"),
        ("workflow", "upgrade"),
    }
)


def _method_contract() -> dict[str, dict[str, Any]]:
    contract_path = Path(__file__).resolve().parents[1] / "contracts" / "daemon_methods.json"
    if contract_path.exists():
        raw = json.loads(contract_path.read_text())
        raw_methods: object
        if isinstance(raw, dict):
            raw_methods = raw.get("methods", raw.get("entries", raw))
        else:
            raw_methods = raw
        if isinstance(raw_methods, dict):
            return {
                str(method): dict(meta) if isinstance(meta, dict) else {}
                for method, meta in raw_methods.items()
            }
        if isinstance(raw_methods, list):
            contract: dict[str, dict[str, Any]] = {}
            for item in raw_methods:
                if not isinstance(item, dict):
                    continue
                method = item.get("method", item.get("name"))
                if isinstance(method, str):
                    contract[method] = dict(item)
            return contract
    return {
        method: {
            "required_capability": entry.required_capability,
            "deprecated": entry.deprecated,
            "local_file_authoring": method in LOCAL_FILE_AUTHORING_METHODS,
        }
        for method, entry in METHOD_REGISTRY.items()
    }


def _args_for_lookup(command: str, subcommand: str | None) -> argparse.Namespace:
    return argparse.Namespace(
        command=command,
        run_command=subcommand if command == "run" else None,
        workflow_command=subcommand if command == "workflow" else None,
        list_command=subcommand if command == "list" else None,
        recovery_command=subcommand if command == "recovery" else None,
        evidence_command=subcommand if command == "evidence" else None,
        corpus_command=subcommand if command == "corpus" else None,
        archive_command=subcommand if command == "archive" else None,
        decision_command=subcommand if command == "decision" else None,
        checkpoint_command=subcommand if command == "checkpoint" else None,
        escalation_command=subcommand if command == "escalation" else None,
        branch_command=subcommand if command == "branch" else None,
        session_command=subcommand if command == "session" else None,
        worktree_command=subcommand if command == "worktree" else None,
        supervise_command=subcommand if command == "supervise" else None,
        cross_repo_command=subcommand if command == "cross-repo" else None,
        id="target_1",
        run_id="run_1",
        job_id="job_1",
        session_id="sess_1",
        lease_id="lease_1",
        message_id="msg_1",
        workflow="workflow.yaml",
        role="worker",
        lane="codex",
        fresh=True,
        capability=["read"],
        parent_session_id=None,
        operator_label=None,
        force_non_fresh=False,
        lease_seconds=600,
        extend_seconds=600,
        kind="note",
        body_json="{}",
        severity="blocked",
        description="blocked on dependency",
        summary="done",
        logical_name="artifact",
        path="docs/out.md",
        verdict="accept",
        findings_artifact_id=None,
        rationale="reviewed",
        allow_no_process_execution=False,
        override_rationale=None,
        auto_fresh_session=False,
        state="running",
        workflow_job_id="job_a",
        limit=10,
        format="json",
        graph_orient="tb",
        graph_style="layered",
        reason="operator requested",
        cascade=False,
        force=False,
        justification=None,
        blocker_id="blocker_1",
        escalation_id="esc_1",
        resolution_note="resolved by principal",
        complete=False,
        dry_run=True,
        autonomous_review_requeue=True,
        autonomous_process_reconcile=True,
        max_requeues_per_sweep=1,
        checkpoint_timeout_seconds=60,
        eligible_after_seconds=30,
        interval_seconds=5,
        exit_on_terminal=True,
        max_sweeps=1,
        mtime_grace_seconds=30,
        since="HEAD~1",
        out="tmp/export",
        title="Decision",
        outcome="accepted",
        decision_id="decision_1",
        follow_up=None,
        action="continue",
        branch="work/branch",
        create=False,
        use_current=False,
        strict=False,
        worktree_id="worktree_1",
        packet_id="packet_1",
    )


def _translated_methods() -> dict[tuple[str, str | None], str]:
    translated: dict[tuple[str, str | None], str] = {}
    for key, translator in sorted(_LOOKUP.items()):
        command, subcommand = key
        method, _params = translator(_args_for_lookup(command, subcommand), Path("/repo"))
        translated[key] = method
    return translated


def _route(command: str, subcommand: str | None, **kwargs: object) -> tuple[str, dict[str, object]]:
    args = argparse.Namespace(command=command, **kwargs)
    key = (command, subcommand if subcommand is not None else _subcommand(args))
    translator = _LOOKUP[key]
    return translator(args, Path("/repo"))


def test_workflow_authoring_commands_stay_cli_local() -> None:
    routed = sorted(WORKFLOW_AUTHORING_KEYS & set(_LOOKUP))
    assert not routed


def test_all_daemon_routed_cli_methods_are_registered_contract_methods() -> None:
    contract = _method_contract()
    translated = _translated_methods()

    missing = [f"{key!r} -> {method}" for key, method in translated.items() if method not in contract]

    assert not missing, "CLI daemon route translates to unregistered methods: " + ", ".join(missing)


def test_daemon_cli_route_lookup_is_derived_from_contract() -> None:
    contract_keys = {(route.command, route.subcommand) for route in _load_cli_route_contracts()}

    assert set(_LOOKUP) == contract_keys


def test_cli_local_workflow_authoring_methods_are_not_daemon_routed() -> None:
    translated_methods = set(_translated_methods().values())
    routed_workflow_keys = sorted(key for key in _LOOKUP if key[0] == "workflow")

    assert not routed_workflow_keys
    assert not (translated_methods & LOCAL_FILE_AUTHORING_METHODS)


def test_daemon_routed_cli_methods_do_not_depend_on_daemon_cli_fallback() -> None:
    from striatum.daemon_rpc import server as server_mod

    cli_routes = getattr(server_mod, "CLI_ROUTES", {})
    translated = _translated_methods()
    fallback_backed = [f"{key!r} -> {method}" for key, method in translated.items() if method in cli_routes]

    assert not fallback_backed, "CLI daemon route still depends on daemon CLI fallback: " + ", ".join(fallback_backed)


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


def test_dashboard_all_routes_to_daemon_global_method() -> None:
    method, params = _route("dashboard", None, all=True, run_id=None)

    assert method == "dashboard.all"
    assert params == {}


def test_dashboard_run_id_routes_to_single_repo_method() -> None:
    method, params = _route("dashboard", None, all=False, run_id="run_1")

    assert method == "dashboard"
    assert params == {"run_id": "run_1"}


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


def test_archive_create_routes_to_daemon_rpc() -> None:
    method, params = _route(
        "archive",
        "create",
        archive_command="create",
        run_id="run_1",
        out="archives/run_1",
    )

    assert method == "archive.create"
    assert params == {"run_id": "run_1", "out": "archives/run_1"}


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


def test_session_close_routes_to_pg_handler() -> None:
    method, params = _route(
        "session",
        "close",
        session_command="close",
        session_id="sess_1",
        reason="finished",
    )

    assert method == "session.close"
    assert params == {"session_id": "sess_1", "reason": "finished"}


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


def test_send_routes_to_pg_handler() -> None:
    method, params = _route(
        "send",
        None,
        session_id="sess_1",
        kind="note",
        body_json='{"summary":"working"}',
    )

    assert method == "work.send_message"
    assert params == {
        "session_id": "sess_1",
        "kind": "note",
        "body_json": '{"summary":"working"}',
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


def test_recovery_auto_routes_to_registered_recovery_sweep_method() -> None:
    method, params = _route(
        "recovery",
        "auto",
        recovery_command="auto",
        run_id="run_1",
        dry_run=True,
        autonomous_review_requeue=True,
        autonomous_process_reconcile=False,
        max_requeues_per_sweep=2,
        checkpoint_timeout_seconds=60,
        eligible_after_seconds=30,
    )

    assert method == "recovery.sweep"
    assert params == {
        "run_id": "run_1",
        "dry_run": True,
        "autonomous_review_requeue": True,
        "autonomous_process_reconcile": False,
        "max_requeues_per_sweep": 2,
        "checkpoint_timeout_seconds": 60,
        "eligible_after_seconds": 30,
    }


def test_recovery_watch_is_cli_local_daemon_scheduler_not_rpc_route() -> None:
    assert ("recovery", "watch") not in _LOOKUP


def test_recovery_auto_publish_routes_to_explicit_auto_publish_method() -> None:
    method, params = _route(
        "recovery",
        "auto-publish",
        recovery_command="auto-publish",
        run_id="run_1",
        dry_run=True,
    )

    assert method == "recovery.auto_publish_stale_artifacts"
    assert params == {
        "run_id": "run_1",
        "dry_run": True,
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


def test_run_graph_routes_to_pg_handler() -> None:
    method, params = _route(
        "run",
        "graph",
        run_command="graph",
        run_id="run_1",
        format="ascii",
        graph_orient="lr",
        graph_style="fancy",
    )

    assert method == "run.graph"
    assert params == {
        "run_id": "run_1",
        "format": "ascii",
        "graph_orient": "lr",
        "graph_style": "fancy",
    }


def test_run_graph_human_cli_unwraps_source(monkeypatch: pytest.MonkeyPatch) -> None:
    args = argparse.Namespace(
        command="run",
        run_command="graph",
        run_id="run_1",
        format="mermaid",
        graph_orient="tb",
        graph_style="layered",
        json=False,
    )
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route.resolve_socket_path",
        lambda: Path("/tmp/striatumd.sock"),
    )
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route.daemon_socket_is_reachable",
        lambda _path: True,
    )
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route._resolve_repository_id",
        lambda _sock_path, _repo: "repo_a",
    )
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route._call_with_handshake",
        lambda _path, _method, _params: {
            "ok": True,
            "data": {"format": "mermaid", "source": "flowchart TD\n"},
        },
    )

    routed, data = try_route(args, Path("/repo"))

    assert routed is True
    assert data == "flowchart TD\n"


def test_call_with_handshake_uses_striatum_version_for_hello(monkeypatch: pytest.MonkeyPatch) -> None:
    captured: dict[str, str] = {}

    class FakeEnvelope:
        def __init__(self, payload: bytes) -> None:
            self.payload = payload

        def encode(self) -> bytes:
            return self.payload

    class FakeStream:
        def __init__(self) -> None:
            self.responses = [
                b'{"ok": true, "data": {}}\n',
                b'{"ok": true, "data": {"called": true}}\n',
            ]

        def write(self, data: bytes) -> int:
            return len(data)

        def flush(self) -> None:
            return None

        def readline(self) -> bytes:
            return self.responses.pop(0)

    class FakeSocket:
        def __enter__(self) -> FakeSocket:
            return self

        def __exit__(self, *_exc: object) -> None:
            return None

        def settimeout(self, _seconds: float) -> None:
            return None

        def connect(self, _path: str) -> None:
            return None

        def makefile(self, _mode: str) -> FakeStream:
            return FakeStream()

    def fake_hello_envelope(*, request_id: str, client_name: str, client_version: str) -> FakeEnvelope:
        captured["request_id"] = request_id
        captured["client_name"] = client_name
        captured["client_version"] = client_version
        return FakeEnvelope(b"hello")

    monkeypatch.setattr("striatum.cli.daemon_rpc_route.socket.socket", lambda *_args: FakeSocket())
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.hello_envelope", fake_hello_envelope)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.read_runtime_token", lambda: "tok.secret")

    response = _call_with_handshake(Path("/tmp/striatumd.sock"), "doctor", {"verbose": False})

    assert response == {"ok": True, "data": {"called": True}}
    assert captured["client_name"] == "striatum-cli"
    assert captured["client_version"] == striatum.__version__


def test_daemon_rpc_route_source_has_no_legacy_handshake_versions() -> None:
    source = Path(__file__).resolve().parents[1] / "src" / "striatum" / "cli" / "daemon_rpc_route.py"
    text = source.read_text(encoding="utf-8")
    legacy_versions = {".".join(("1", "51", "0")), ".".join(("1", "67", "0"))}

    assert [version for version in legacy_versions if version in text] == []


def test_worktree_routes_to_pg_handlers() -> None:
    create_method, create_params = _route(
        "worktree",
        "create",
        worktree_command="create",
        session_id="sess_1",
        job_id="job_1",
        lease_id="lease_1",
    )
    release_method, release_params = _route(
        "worktree",
        "release",
        worktree_command="release",
        worktree_id="wt_1",
    )
    list_method, list_params = _route(
        "worktree",
        "list",
        worktree_command="list",
        run_id="run_1",
    )

    assert create_method == "worktree.create"
    assert create_params == {
        "session_id": "sess_1",
        "job_id": "job_1",
        "lease_id": "lease_1",
    }
    assert release_method == "worktree.release"
    assert release_params == {"worktree_id": "wt_1"}
    assert list_method == "worktree.list"
    assert list_params == {"run_id": "run_1"}


def test_supervise_routes_to_pg_handlers() -> None:
    start_method, start_params = _route(
        "supervise",
        "start",
        supervise_command="start",
        session_id="sess_1",
    )
    send_method, send_params = _route(
        "supervise",
        "send",
        supervise_command="send",
        session_id="sess_1",
        packet_id="wp_1",
    )
    stop_method, stop_params = _route(
        "supervise",
        "stop",
        supervise_command="stop",
        session_id="sess_1",
        reason="finished",
    )
    status_method, status_params = _route(
        "supervise",
        "status",
        supervise_command="status",
        session_id="sess_1",
    )
    list_method, list_params = _route(
        "supervise",
        "list",
        supervise_command="list",
        run_id="run_1",
        state="attached",
    )

    assert start_method == "supervise.start"
    assert start_params == {"session_id": "sess_1"}
    assert send_method == "supervise.send"
    assert send_params == {"session_id": "sess_1", "packet_id": "wp_1"}
    assert stop_method == "supervise.stop"
    assert stop_params == {"session_id": "sess_1", "reason": "finished"}
    assert status_method == "supervise.status"
    assert status_params == {"session_id": "sess_1"}
    assert list_method == "supervise.list"
    assert list_params == {"run_id": "run_1", "state": "attached"}


def test_cross_repo_commands_route_to_daemon_rpc() -> None:
    list_method, list_params = _route(
        "cross-repo",
        "list",
        cross_repo_command="list",
    )
    describe_method, describe_params = _route(
        "cross-repo",
        "describe",
        cross_repo_command="describe",
        cross_repo_run_id="xrun_1",
    )
    why_method, why_params = _route(
        "cross-repo",
        "why",
        cross_repo_command="why",
        cross_repo_run_id="xrun_1",
    )
    cancel_method, cancel_params = _route(
        "cross-repo",
        "cancel",
        cross_repo_command="cancel",
        cross_repo_run_id="xrun_1",
        reason="operator requested",
    )

    assert list_method == "cross_repo.list"
    assert list_params == {}
    assert describe_method == "cross_repo.describe"
    assert describe_params == {"cross_repo_run_id": "xrun_1"}
    assert why_method == "cross_repo.why"
    assert why_params == {"cross_repo_run_id": "xrun_1"}
    assert cancel_method == "cross_repo.cancel"
    assert cancel_params == {
        "cross_repo_run_id": "xrun_1",
        "reason": "operator requested",
    }


def test_cross_repo_cancel_routes_when_daemon_is_reachable(monkeypatch: pytest.MonkeyPatch) -> None:
    args = argparse.Namespace(
        command="cross-repo",
        cross_repo_command="cancel",
        cross_repo_run_id="xrun_1",
        reason="operator requested",
    )
    captured: dict[str, object] = {}

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route.resolve_socket_path",
        lambda: Path("/tmp/striatumd.sock"),
    )
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route.daemon_socket_is_reachable",
        lambda _path: True,
    )

    def fake_call(_path: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
        captured["method"] = method
        captured["params"] = dict(params)
        return {"ok": True, "data": {"cross_repo_run_id": "xrun_1", "state": "canceled"}}

    monkeypatch.setattr("striatum.cli.daemon_rpc_route._call_with_handshake", fake_call)

    routed, data = try_route(args, Path("/repo"))

    assert routed is True
    assert data == {"cross_repo_run_id": "xrun_1", "state": "canceled"}
    assert captured == {
        "method": "cross_repo.cancel",
        "params": {"cross_repo_run_id": "xrun_1", "reason": "operator requested"},
    }


def test_cross_repo_direct_dispatch_refuses_production_pg_fallback(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    dispatch_mod = importlib.import_module("striatum.cli.dispatch")

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)

    args = argparse.Namespace(
        command="cross-repo",
        cross_repo_command="list",
        postgres_url=None,
    )

    with pytest.raises(StriatumError, match="daemon_route_required"):
        dispatch_mod._dispatch_cross_repo(args)  # noqa: SLF001 - client-boundary guardrail.


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


def test_escalation_commands_route_to_projection_methods() -> None:
    list_method, list_params = _route(
        "escalation",
        "list",
        escalation_command="list",
        run_id="run_1",
        state="open",
        limit=25,
    )
    show_method, show_params = _route(
        "escalation",
        "show",
        escalation_command="show",
        escalation_id="esc_1",
    )
    resolve_method, resolve_params = _route(
        "escalation",
        "resolve",
        escalation_command="resolve",
        escalation_id="esc_1",
        decision_id="dec_1",
        resolution_note="approved",
    )

    assert list_method == "escalation.list"
    assert list_params == {"run_id": "run_1", "state": "open", "limit": 25}
    assert show_method == "escalation.show"
    assert show_params == {"escalation_id": "esc_1"}
    assert resolve_method == "escalation.resolve"
    assert resolve_params == {
        "escalation_id": "esc_1",
        "decision_id": "dec_1",
        "resolution_note": "approved",
    }


def test_principal_inbox_routes_to_escalation_list() -> None:
    method, params = _route(
        "inbox",
        None,
        run_id="run_1",
        state="open",
        limit=50,
    )

    assert method == "escalation.list"
    assert params == {"run_id": "run_1", "state": "open", "limit": 50}


def test_recovery_auto_finalize_routes_to_split_method() -> None:
    method, params = _route(
        "recovery",
        "auto-finalize",
        recovery_command="auto-finalize",
        run_id="run_1",
        job_id="job_1",
        dry_run=False,
        force=True,
        mtime_grace_seconds=30,
        allow_no_process_execution=True,
    )

    assert method == "recovery.auto_finalize"
    assert params == {
        "run_id": "run_1",
        "job_id": "job_1",
        "force": True,
        "dry_run": False,
        "mtime_grace_seconds": 30,
        "allow_no_process_execution": True,
    }


def test_registered_rpc_command_fails_closed_when_daemon_unreachable(monkeypatch: pytest.MonkeyPatch) -> None:
    args = argparse.Namespace(command="status", run_id="run_1")
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.resolve_socket_path", lambda: Path("/tmp/missing.sock"))
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.daemon_socket_is_reachable", lambda _path: False)

    with pytest.raises(StriatumError, match="daemon_unreachable"):
        try_route(args, Path("/repo"))


def test_registered_rpc_command_fails_closed_with_bare_test_harness(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    args = argparse.Namespace(command="status", run_id="run_1")
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.resolve_socket_path", lambda: Path("/tmp/missing.sock"))
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.daemon_socket_is_reachable", lambda _path: False)

    with pytest.raises(StriatumError, match="daemon_unreachable"):
        try_route(args, Path("/repo"))


def test_registered_rpc_command_allows_explicit_legacy_test_opt_out(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    args = argparse.Namespace(command="status", run_id="run_1")
    monkeypatch.setenv("STRIATUM_TEST_HARNESS", "1")
    monkeypatch.setenv("STRIATUM_DAEMON_REQUIRED", "0")
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)

    assert try_route(args, Path("/repo")) == (False, None)


def test_registered_rpc_command_fails_closed_when_repo_unregistered(monkeypatch: pytest.MonkeyPatch) -> None:
    args = argparse.Namespace(command="status", run_id="run_1")
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.resolve_socket_path", lambda: Path("/tmp/striatumd.sock"))
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.daemon_socket_is_reachable", lambda _path: True)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route._resolve_repository_id", lambda _sock_path, _repo: None)

    with pytest.raises(StriatumError, match="repo_not_registered"):
        try_route(args, Path("/repo"))


def test_repository_resolution_uses_daemon_rpc_repo_resolve_without_client_pg(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    calls: list[tuple[str, dict[str, Any]]] = []

    def forbidden_pg_connect(*_args: object, **_kwargs: object) -> object:
        raise AssertionError("client route must not open daemon Postgres")

    monkeypatch.setattr("striatum.daemon_pg.connection.connect", forbidden_pg_connect)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.METHOD_REGISTRY", dict(METHOD_REGISTRY))
    monkeypatch.setattr("striatum.cli.daemon_rpc_route._REPO_ID_CACHE", {})

    def fake_call(_sock_path: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
        calls.append((method, dict(params)))
        return {
            "ok": True,
            "data": {"repository_id": "repo_a"},
        }

    monkeypatch.setattr("striatum.cli.daemon_rpc_route._call_with_handshake", fake_call)

    assert _resolve_repository_id(Path("/tmp/striatumd.sock"), repo) == "repo_a"
    assert calls == [("repo.resolve", {"path": str(repo.resolve())})]


def test_try_route_resolves_repository_by_daemon_rpc_without_client_pg(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    args = argparse.Namespace(command="status", run_id="run_1")
    calls: list[tuple[str, dict[str, Any]]] = []

    def forbidden_pg_connect(*_args: object, **_kwargs: object) -> object:
        raise AssertionError("client route must not open daemon Postgres")

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr("striatum.daemon_pg.connection.connect", forbidden_pg_connect)
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.METHOD_REGISTRY", dict(METHOD_REGISTRY))
    monkeypatch.setattr("striatum.cli.daemon_rpc_route._REPO_ID_CACHE", {})
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.resolve_socket_path", lambda: Path("/tmp/striatumd.sock"))
    monkeypatch.setattr("striatum.cli.daemon_rpc_route.daemon_socket_is_reachable", lambda _path: True)

    def fake_call(_sock_path: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
        calls.append((method, dict(params)))
        if method == "repo.resolve":
            return {
                "ok": True,
                "data": {"repository_id": "repo_a"},
            }
        return {"ok": True, "data": {"state": "running"}}

    monkeypatch.setattr("striatum.cli.daemon_rpc_route._call_with_handshake", fake_call)

    routed, data = try_route(args, repo)

    assert routed is True
    assert data == {"state": "running"}
    assert calls == [
        ("repo.resolve", {"path": str(repo.resolve())}),
        ("status", {"run_id": "run_1", "repository_id": "repo_a"}),
    ]


def test_repo_list_routes_with_empty_params() -> None:
    method, params = _route(
        "repo",
        "list",
        repo_command="list",
    )

    assert method == "repo.list"
    assert params == {}


def test_repo_list_json_payload_is_unchanged_by_table_formatter(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """``repo list --json`` must keep the daemon payload byte-for-byte
    identical so existing consumers that parse ``data.repositories[]``
    continue to work after GH #25.
    """
    args = argparse.Namespace(command="repo", repo_command="list", json=True)
    payload = {
        "repositories": [
            {
                "repository_id": "repo_a89ecd16640f4f81abc",
                "repo_root": "/repo/a",
                "display_name": "a",
                "state": "active",
                "last_seen_at": "2026-05-19T12:00:00Z",
            }
        ]
    }
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route.resolve_socket_path",
        lambda: Path("/tmp/striatumd.sock"),
    )
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route.daemon_socket_is_reachable",
        lambda _path: True,
    )
    captured: dict[str, object] = {}

    def fake_call(_path: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
        captured["method"] = method
        captured["params"] = dict(params)
        return {"ok": True, "data": payload}

    monkeypatch.setattr("striatum.cli.daemon_rpc_route._call_with_handshake", fake_call)

    routed, data = try_route(args, tmp_path)

    assert routed is True
    assert data == payload
    assert captured == {"method": "repo.list", "params": {}}


def test_repo_list_non_json_renders_human_table(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Non-JSON ``repo list`` must return a formatted table string so
    ``main()`` prints the table rather than serializing the dict as
    JSON. The current repository is sorted first and marked with ``*``.
    """
    args = argparse.Namespace(command="repo", repo_command="list", json=False)
    payload = {
        "repositories": [
            {
                "repository_id": "repo_b1234567cafe",
                "repo_root": "/repo/other",
                "display_name": "other",
                "state": "active",
                "last_seen_at": "2026-05-18T10:00:00Z",
            },
            {
                "repository_id": "repo_a89ecd1664ff",
                "repo_root": str(tmp_path),
                "display_name": "striatum",
                "state": "active",
                "last_seen_at": "2026-05-19T12:00:00Z",
            },
        ]
    }
    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_IN_DAEMON_HANDLER", raising=False)
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route.resolve_socket_path",
        lambda: Path("/tmp/striatumd.sock"),
    )
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route.daemon_socket_is_reachable",
        lambda _path: True,
    )
    monkeypatch.setattr(
        "striatum.cli.daemon_rpc_route._call_with_handshake",
        lambda _path, _method, _params: {"ok": True, "data": payload},
    )

    routed, data = try_route(args, tmp_path)

    assert routed is True
    assert isinstance(data, str)
    lines = data.splitlines()
    assert lines[0].startswith("  DISPLAY_NAME"), lines
    # Current repo row is sorted first and prefixed with ``*``.
    assert lines[1].startswith("* striatum"), lines
    # The other repo row keeps a blank marker.
    assert lines[2].startswith("  other"), lines
    assert "repo_a89ecd16" in lines[1]
    assert "repo_b1234567" in lines[2]
    assert str(tmp_path) in lines[1]


def test_repo_list_table_handles_missing_columns(tmp_path: Path) -> None:
    """Missing ``display_name``, ``state``, and ``last_seen_at`` render
    as ``-`` so the table stays aligned for partial daemon payloads.
    """
    rendered = _format_repo_list_table(
        {
            "repositories": [
                {
                    "repository_id": "repo_deadbeefcafe",
                    "repo_root": "/repo/orphan",
                }
            ]
        },
        tmp_path,
    )
    body_line = rendered.splitlines()[1]
    # Three leading "-" cells (display_name, state, last_seen_at), the
    # short repository id, and the repo root must all be present.
    dash_cells = [cell for cell in body_line.split("  ") if cell.strip() == "-"]
    assert len(dash_cells) == 3
    assert "repo_deadbeef" in body_line
    assert "/repo/orphan" in body_line


def test_short_repository_id_handles_unexpected_shapes() -> None:
    assert _short_repository_id("repo_a89ecd1664ff") == "repo_a89ecd16"
    assert _short_repository_id("custom-12345-id-xxx") == "custom-12345-"
    assert _short_repository_id("repo_") == "repo_"
    assert _short_repository_id("") == "-"


def test_client_boundary_source_does_not_import_daemon_pg_lookup() -> None:
    root = Path(__file__).resolve().parents[1]
    route_text = (root / "src" / "striatum" / "cli" / "daemon_rpc_route.py").read_text(encoding="utf-8")
    service_text = (root / "src" / "striatum" / "service_daemon.py").read_text(encoding="utf-8")

    assert "striatum.daemon_pg.connection" not in route_text
    assert "striatum.daemon_pg.connection" not in service_text
    assert "resolve_config" not in route_text
    assert "resolve_config" not in service_text

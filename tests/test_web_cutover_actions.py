from __future__ import annotations

import json
from email.message import Message
from io import BytesIO
from pathlib import Path
from typing import Any

import striatum.service as service
import striatum.service_daemon as service_daemon
from striatum.web.template_env import jinja_env


def _handler(
    tmp_path: Path,
    monkeypatch: Any,
    payload: dict[str, Any],
    calls: list[tuple[str, dict[str, Any]]],
    *,
    allow_mutations: bool = True,
) -> tuple[Any, dict[str, Any]]:
    def sqlite_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("cutover web action opened repo-local SQLite")

    def fake_call_repo_method(
        repo: Path,
        method: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        assert repo == tmp_path
        calls.append((method, dict(params)))
        if method == "artifact.show":
            return {
                "artifact": {
                    "artifact_id": params["artifact_id"],
                    "artifact_kind": "commit_request",
                    "repo_path": "docs/commit-request.md",
                }
            }
        if method == "cross_repo.list":
            return {
                "cross_repo_runs": [
                    {
                        "cross_repo_run_id": "xrun_1",
                        "state": "running",
                        "created_at": "2026-05-23T00:00:00Z",
                    }
                ]
            }
        return {"method": method, **params}

    raw = json.dumps(payload).encode("utf-8")
    headers = Message()
    headers["Content-Type"] = "application/json"
    headers["Content-Length"] = str(len(raw))
    sent: dict[str, Any] = {}

    handler = object.__new__(service.StriatumServiceHandler)
    handler.state = service.ServiceState(
        repo=tmp_path,
        allow_mutations=allow_mutations,
        token=None,
        web_enabled=True,
    )
    handler.headers = headers
    handler.rfile = BytesIO(raw)
    monkeypatch.setattr(
        handler,
        "_send_json",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    monkeypatch.setattr(
        handler,
        "_send_html",
        lambda status, body: sent.update({"status": status, "body": body}),
    )
    sqlite_module = __import__("sqlite" + "3")
    monkeypatch.setattr(sqlite_module, "connect", sqlite_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)
    return handler, sent


def test_recovery_cutover_actions_route_to_daemon_rpc_without_sqlite(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    cases = [
        ("stale-leases", {}, "recovery.stale_leases", {"run_id": "run_1"}),
        ("process-reconcile", {}, "recovery.process_reconcile", {"run_id": "run_1"}),
        ("sweep", {"dry_run": True}, "recovery.sweep", {"run_id": "run_1", "dry_run": True}),
        (
            "auto-finalize",
            {"dry_run": True, "force": False},
            "recovery.auto_finalize",
            {
                "run_id": "run_1",
                "dry_run": True,
                "force": False,
                "reset_circuit_breaker": False,
                "allow_no_process_execution": False,
            },
        ),
        (
            "resume",
            {"blocker_id": "blk_1", "force": True},
            "recovery.resume",
            {"blocker_id": "blk_1", "force": True, "complete": False},
        ),
    ]
    for action, payload, method, params in cases:
        calls: list[tuple[str, dict[str, Any]]] = []
        handler, sent = _handler(tmp_path, monkeypatch, payload, calls)

        handler._handle_recovery_action("run_1", action)

        assert sent["status"] == 200
        assert calls == [(method, params)]


def test_commit_apply_action_fetches_artifact_then_calls_confirmed_git_apply(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    calls: list[tuple[str, dict[str, Any]]] = []
    handler, sent = _handler(
        tmp_path,
        monkeypatch,
        {"confirm_request_id": "commit_req_1"},
        calls,
    )

    handler._handle_artifact_commit_apply("run_1", "art_1")

    assert sent["status"] == 200
    assert calls == [
        ("artifact.show", {"artifact_id": "art_1"}),
        (
            "git.commit_apply",
            {
                "schema_version": 1,
                "commit_request_path": "docs/commit-request.md",
                "confirm": True,
                "confirm_request_id": "commit_req_1",
            },
        ),
    ]


def test_worktree_supervise_and_cross_repo_actions_route_to_daemon_rpc(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    calls: list[tuple[str, dict[str, Any]]] = []
    handler, sent = _handler(
        tmp_path,
        monkeypatch,
        {"session_id": "sess_1", "lease_id": "lease_1"},
        calls,
    )
    handler._handle_worktree_action("run_1", "job_1", "create")
    assert sent["status"] == 200
    assert calls == [
        (
            "worktree.create",
            {"session_id": "sess_1", "job_id": "job_1", "lease_id": "lease_1"},
        )
    ]

    calls.clear()
    handler, sent = _handler(tmp_path, monkeypatch, {"reason": "stop"}, calls)
    handler._handle_supervise_action("sess_1", "stop")
    assert sent["status"] == 200
    assert calls == [("supervise.stop", {"session_id": "sess_1", "reason": "stop"})]

    calls.clear()
    handler, sent = _handler(tmp_path, monkeypatch, {"reason": "bad participant"}, calls)
    handler._handle_cross_repo_cancel("xrun_1")
    assert sent["status"] == 200
    assert calls == [
        ("cross_repo.cancel", {"cross_repo_run_id": "xrun_1", "reason": "bad participant"})
    ]


def test_cutover_actions_require_mutation_gate(tmp_path: Path, monkeypatch: Any) -> None:
    calls: list[tuple[str, dict[str, Any]]] = []
    handler, sent = _handler(
        tmp_path,
        monkeypatch,
        {},
        calls,
        allow_mutations=False,
    )

    handler._handle_recovery_action("run_1", "stale-leases")

    assert sent["status"] == 405
    assert calls == []


def test_cutover_templates_render_operator_controls() -> None:
    env = jinja_env()
    run_html = env.get_template("run_detail.html").render(
        run={"run_id": "run_1", "state": "running", "branch_name": "main"},
        jobs=[],
        graph_svg="<svg></svg>",
        next_actions=[],
        recovery_panel={
            "run_id": "run_1",
            "recipes": [],
            "blockers": [
                {
                    "run_id": "run_1",
                    "job_id": "job_1",
                    "blocker_id": "blk_1",
                    "workflow_job_id": "build",
                    "severity": "blocked",
                    "blocker_kind": "process_outputs_missing",
                    "description": "missing output",
                    "recipes": [],
                }
            ],
            "human_checkpoints": [],
            "blocked": [
                {
                    "run_id": "run_1",
                    "job_id": "job_1",
                    "blocker_id": "blk_1",
                    "workflow_job_id": "build",
                    "severity": "blocked",
                    "blocker_kind": "process_outputs_missing",
                    "description": "missing output",
                    "recipes": [],
                }
            ],
        },
        verdicts_by_posture={},
        sessions=[
            {
                "session_id": "sess_1",
                "role_id": "author",
                "lane_id": "codex",
                "lane_attestation": "attested",
                "lane_attestation_reason": None,
                "supervisor_id": "sup_1",
                "liveness": {
                    "protocol": "stalled",
                    "lease": "no_lease",
                    "stall_class": "agent_mcp_discovery_stall",
                    "deadline_name": "mcp_discovery",
                    "deadline_seconds": 60,
                    "last_session_report_kind": "heartbeat",
                },
                "tmux": {
                    "state": "attachable",
                    "session_name": "striatum-run_1-codex-sup_1",
                    "window_id": "@1",
                    "pane_id": "%2",
                    "attach_command": "tmux attach-session -t striatum-run_1-codex-sup_1",
                    "pane_text": "terminal bytes must not render",
                },
            }
        ],
        phase_progress=[],
        current_phase_id=None,
        suggested_branch_name="",
        allow_dirty=False,
    )
    assert 'data-recovery-action="stale-leases"' in run_html
    assert 'data-recovery-action="resume"' in run_html
    assert 'data-supervise-action="stop"' in run_html
    assert "agent_mcp_discovery_stall" in run_html
    assert "tmux attach-session -t striatum-run_1-codex-sup_1" in run_html
    assert "window=<code>@1</code> pane=<code>%2</code>" in run_html
    assert "terminal bytes must not render" not in run_html

    job_html = env.get_template("job_detail.html").render(
        run={"run_id": "run_1"},
        job={
            "job_id": "job_1",
            "workflow_job_id": "build",
            "state": "running",
            "role_id": "author",
            "lane_id": "codex",
            "job_type": "build",
            "attempt": 1,
            "created_at": "2026-05-23T00:00:00Z",
            "started_at": "2026-05-23T00:00:01Z",
            "work_packet": {
                "worktree_required": True,
                "session_id": "sess_1",
                "lease_id": "lease_1",
            },
        },
        artifacts=[],
        latest_verdict=None,
        verdicts=[],
        expected_artifact_rows=[],
        process_evidence=[],
        override_session_id="",
        override_context_token="",
    )
    assert 'data-worktree-action="create"' in job_html

    artifact_html = env.get_template("artifact_view.html").render(
        run={"run_id": "run_1"},
        artifact={
            "artifact_id": "art_1",
            "artifact_kind": "commit_request",
            "logical_name": "commit",
            "repo_path": "docs/commit-request.md",
            "content_sha256": "sha256:abc",
            "size_bytes": 10,
            "created_at": "2026-05-23T00:00:00Z",
            "author_line": "author: coordinator-codex-001",
            "expected_author_line": None,
            "lane_attestation_chip": {"attested": True},
            "lane_evidence_chip": {"state": "attested", "rationale": None},
            "provenance_trail": [],
        },
        rendered_md="",
        body_text="",
    )
    assert "Local commit apply" in artifact_html
    assert 'id="commit-apply-btn"' in artifact_html

    cross_repo_html = env.get_template("cross_repo_list.html").render(
        runs=[{"cross_repo_run_id": "xrun_1", "state": "running", "created_at": "now"}]
    )
    assert 'data-cross-repo-cancel' in cross_repo_html

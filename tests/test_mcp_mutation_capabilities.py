from __future__ import annotations

import json
from pathlib import Path
from typing import Any, cast

import pytest

from _harness.mcp import daemon_tool_specs
from striatum.daemon_rpc.capability import authorize
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError, RpcResponse
from striatum.daemon_rpc.registry import CAPABILITIES, METHOD_REGISTRY, mcp_tool_descriptor
from striatum.daemon_rpc.server import DaemonRpcRouter, LOCAL_FILE_AUTHORING_METHODS, PRODUCTION_MCP_HIDDEN_METHODS
from striatum.daemon_pg.mcp_dispatch import dispatch_mcp_tool_call


def test_authorize_refuses_malformed_tokens() -> None:
    for token in ("notoken", ".", "dtok_abc."):
        auth = authorize(object(), required="read", repository_id=None, token=token)
        assert auth.decision == "denied"
        assert auth.denial_reason == "token_malformed"


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


def _contract_bool(meta: dict[str, Any], *names: str) -> bool:
    return any(bool(meta.get(name)) for name in names)


def _expected_daemon_mcp_tools(allowed_capabilities: set[str]) -> set[str]:
    expected: set[str] = set()
    for method, meta in _method_contract().items():
        required = meta.get("required_capability")
        if required is None:
            continue
        if method.startswith("daemon."):
            continue
        if method in PRODUCTION_MCP_HIDDEN_METHODS or _contract_bool(
            meta,
            "local_file_authoring",
            "cli_local",
            "cli_local_only",
        ):
            continue
        if _contract_bool(meta, "deprecated"):
            continue
        if required in allowed_capabilities:
            expected.add(method)
    return expected


def test_daemon_mcp_tools_list_filters_by_capability(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        decision = "allowed" if required == "read" else "denied"
        return RpcAuthContext("client", "token", repository_id, required, decision, None if decision == "allowed" else "capability_missing")

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)

    tools = daemon_tool_specs(object(), {"token": "dtok.secret", "repository_id": "repo_a"})

    names = {tool["name"] for tool in tools}
    assert "status" in names
    assert "workflow.validate" not in names
    assert "workflow.generate" not in names
    assert "workflow.upgrade" not in names
    assert "publish_artifact" not in names
    assert "apply.reviewed_patch" not in names
    assert "dogfood.surgical_recovery" not in names


def test_daemon_mcp_tools_match_registered_non_deprecated_authorized_methods(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    allowed_capabilities = set(CAPABILITIES)

    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        decision = "allowed" if required in allowed_capabilities else "denied"
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            required if decision == "allowed" else None,
            decision,
            None if decision == "allowed" else "capability_missing",
        )

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)

    tools = daemon_tool_specs(object(), {"token": "dtok.secret", "repository_id": "repo_a"})

    names = {tool["name"] for tool in tools}
    assert names == _expected_daemon_mcp_tools(allowed_capabilities)
    tools_by_name = {str(tool["name"]): tool for tool in tools}
    for name in names:
        assert tools_by_name[str(name)] == mcp_tool_descriptor(METHOD_REGISTRY[str(name)])
    assert not (names & LOCAL_FILE_AUTHORING_METHODS)
    assert not (names & PRODUCTION_MCP_HIDDEN_METHODS)
    assert not {
        method
        for method in names
        if _contract_bool(_method_contract().get(str(method), {}), "deprecated")
    }


def test_daemon_mcp_tools_list_excludes_removed_dogfood_composites(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        decision = "allowed" if required == "surgical_recovery" else "denied"
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            required if decision == "allowed" else None,
            decision,
            None if decision == "allowed" else "capability_missing",
        )

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)

    tools = daemon_tool_specs(object(), {"token": "dtok.secret", "repository_id": "repo_a"})

    names = {tool["name"] for tool in tools}
    assert "dogfood.surgical_recovery" not in names
    assert "dogfood.publish_on_behalf" not in names


def test_escalation_mcp_visibility_tracks_read_and_admin_capabilities(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    def read_authorize(
        conn: object,
        *,
        required: str | None,
        repository_id: str | None,
        token: str | None,
    ) -> RpcAuthContext:
        decision = "allowed" if required == "read" else "denied"
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            required if decision == "allowed" else None,
            decision,
            None if decision == "allowed" else "capability_missing",
        )

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", read_authorize)
    read_names = {
        tool["name"]
        for tool in daemon_tool_specs(object(), {"token": "dtok.secret", "repository_id": "repo_a"})
    }

    assert "escalation.list" in read_names
    assert "escalation.show" in read_names
    assert "escalation.resolve" not in read_names

    def admin_authorize(
        conn: object,
        *,
        required: str | None,
        repository_id: str | None,
        token: str | None,
    ) -> RpcAuthContext:
        decision = "allowed" if required == "admin" else "denied"
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            required if decision == "allowed" else None,
            decision,
            None if decision == "allowed" else "capability_missing",
        )

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", admin_authorize)
    admin_names = {
        tool["name"]
        for tool in daemon_tool_specs(object(), {"token": "dtok.secret", "repository_id": "repo_a"})
    }

    assert "escalation.resolve" in admin_names
    assert "escalation.list" not in admin_names
    assert "escalation.show" not in admin_names


def test_escalation_resolve_mcp_call_refuses_read_only_or_wrong_repository_tokens(
    monkeypatch: Any,
) -> None:
    audit_calls: list[dict[str, Any]] = []
    request_logs: list[dict[str, Any]] = []

    def read_only_authorize(
        conn: object,
        *,
        required: str | None,
        repository_id: str | None,
        token: str | None,
    ) -> RpcAuthContext:
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            None,
            "denied",
            "capability_missing",
        )

    def fake_append_audit_row(*args: Any, **kwargs: Any) -> int:
        audit_calls.append(kwargs)
        return 11

    def fake_append_request_log(*args: Any, **kwargs: Any) -> None:
        request_logs.append(kwargs)

    monkeypatch.setattr("striatum.daemon_pg.mcp_dispatch.authorize", read_only_authorize)
    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_audit_row", fake_append_audit_row)
    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_request_log", fake_append_request_log)

    read_only = dispatch_mcp_tool_call(
        pg_conn=object(),
        name="escalation.resolve",
        token="dtok.secret",
        request_id="req-escalation-read-only",
        arguments={"repository_id": "repo_a", "escalation_id": "blk_1"},
    )

    assert read_only["isError"] is True
    structured = read_only["structuredContent"]
    assert isinstance(structured, dict)
    assert structured["error"] == "capability_missing"

    def wrong_repo_authorize(
        conn: object,
        *,
        required: str | None,
        repository_id: str | None,
        token: str | None,
    ) -> RpcAuthContext:
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            None,
            "denied",
            "capability_scope_mismatch",
        )

    monkeypatch.setattr("striatum.daemon_pg.mcp_dispatch.authorize", wrong_repo_authorize)
    wrong_repo = dispatch_mcp_tool_call(
        pg_conn=object(),
        name="escalation.resolve",
        token="dtok.secret",
        request_id="req-escalation-wrong-repo",
        arguments={"repository_id": "repo_b", "escalation_id": "blk_1"},
    )

    assert wrong_repo["isError"] is True
    structured = wrong_repo["structuredContent"]
    assert isinstance(structured, dict)
    assert structured["error"] == "capability_scope_mismatch"
    assert audit_calls[-1]["auth"].denial_reason == "capability_scope_mismatch"


def test_daemon_mcp_tools_call_reauthorizes_and_audits_denial(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    audit_calls: list[dict[str, Any]] = []
    request_logs: list[dict[str, Any]] = []

    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        return RpcAuthContext("client", "token", repository_id, None, "denied", "capability_missing")

    monkeypatch.setattr("striatum.daemon_rpc.capability.authorize", fake_authorize)
    # daemon_pg/mcp_dispatch.py rebinds `authorize` as a local name at
    # import time; under certain pytest collection orders it has already
    # imported the original symbol and the source-module monkeypatch
    # above doesn't propagate. Patch the bound reference too — defensive
    # against test-ordering flakes.
    monkeypatch.setattr("striatum.daemon_pg.mcp_dispatch.authorize", fake_authorize)
    def fake_append_audit_row(*args: Any, **kwargs: Any) -> int:
        audit_calls.append(kwargs)
        return 7

    def fake_append_request_log(*args: Any, **kwargs: Any) -> None:
        request_logs.append(kwargs)

    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_audit_row", fake_append_audit_row)
    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_request_log", fake_append_request_log)

    result = dispatch_mcp_tool_call(
        pg_conn=object(),
        name="publish_artifact",
        token="dtok.secret",
        request_id="req-1",
        arguments={"repository_id": "repo_a"},
    )

    assert result["isError"] is True
    structured = result["structuredContent"]
    assert isinstance(structured, dict)
    assert structured["error"] == "capability_missing"
    assert audit_calls[0]["transport"] == "mcp"
    assert audit_calls[0]["auth"].decision == "denied"
    assert request_logs[0]["decision"] == "denied"


def test_daemon_mcp_retired_apply_reviewed_patch_is_default_denied_and_audited(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    audit_calls: list[dict[str, Any]] = []

    def fake_append_unknown_audit_row(*args: Any, **kwargs: Any) -> int:
        audit_calls.append(kwargs)
        return 9

    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_audit_row", fake_append_unknown_audit_row)
    monkeypatch.setattr("striatum.daemon_rpc.request_log.append_request_log", lambda *args, **kwargs: None)

    result = dispatch_mcp_tool_call(
        pg_conn=object(),
        name="apply.reviewed_patch",
        token=None,
        request_id="req-2",
        arguments={"repository_id": "repo_a"},
    )

    assert result["isError"] is True
    structured = result["structuredContent"]
    assert isinstance(structured, dict)
    assert structured["error"] == "method_unknown"
    assert audit_calls[0]["auth"].denial_reason == "method_unknown"


def test_mcp_structured_error_uses_nested_rpc_error_codes(monkeypatch) -> None:  # type: ignore[no-untyped-def]
    def fake_authorize(conn: object, *, required: str | None, repository_id: str | None, token: str | None) -> RpcAuthContext:
        return RpcAuthContext("client", "token", repository_id, required, "allowed")

    class FakeRouter:
        def handle(self, envelope: RpcEnvelope, **_kwargs: Any) -> RpcResponse:
            return RpcResponse.error_response(
                request_id=envelope.request_id,
                audit_id="aud_23",
                error=RpcError(
                    "command_failed",
                    "wrapped helper failure",
                    details={
                        "error": {
                            "code": "composite_failed",
                            "details": {
                                "specific_error": {
                                    "code": "artifact_error",
                                    "message": "artifact file does not exist",
                                }
                            },
                        }
                    },
                ),
            )

    monkeypatch.setattr("striatum.daemon_pg.mcp_dispatch.authorize", fake_authorize)

    result = dispatch_mcp_tool_call(
        pg_conn=object(),
        router=cast(DaemonRpcRouter, FakeRouter()),
        name="work.send_message",
        arguments={"repository_id": "repo_a", "session_id": "sess_1", "kind": "note"},
        token="dtok.secret",
        request_id="req-nested-error",
    )

    assert result["isError"] is True
    structured = result["structuredContent"]
    assert isinstance(structured, dict)
    assert structured["error"] == "composite_failed"
    assert structured["error_codes"] == ["command_failed", "composite_failed", "artifact_error"]


@pytest.mark.parametrize(
    "method",
    [
        "git.commit_apply",
        "run.pause",
        "run.resume",
        "run.cancel",
        "run.retry_job",
        "recovery.cancel_job",
        "recovery.stale_leases",
        "recovery.requeue_stale",
        "recovery.process_reconcile",
        "recovery.resume",
        "recovery.sweep",
        "recovery.auto_publish_stale_artifacts",
        "recovery.auto_finalize",
        "branch.confirm",
        "decision.record",
        "checkpoint.resolve",
        "session.close",
        "work.release",
        "work.block",
        "worktree.create",
        "worktree.release",
        "supervise.start",
        "supervise.send",
        "supervise.stop",
        "cross_repo.cancel",
        "review.verdict",
        "review.submit",
        "review.override",
    ],
)
def test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc(
    monkeypatch: Any,
    method: str,
) -> None:
    """Pin exact MCP coverage for CLI-retirement parity ledger rows."""

    def fake_authorize(
        conn: object,
        *,
        required: str | None,
        repository_id: str | None,
        token: str | None,
    ) -> RpcAuthContext:
        return RpcAuthContext(
            "client",
            "token",
            repository_id,
            required,
            "allowed",
            None,
        )

    class CaptureRouter:
        def __init__(self) -> None:
            self.calls: list[tuple[RpcEnvelope, dict[str, Any]]] = []

        def handle(self, envelope: RpcEnvelope, **kwargs: Any) -> RpcResponse:
            self.calls.append((envelope, dict(kwargs)))
            return RpcResponse.ok_response(
                request_id=envelope.request_id,
                audit_id="aud_42",
                data={"routed_method": envelope.method},
            )

    router = CaptureRouter()
    monkeypatch.setattr("striatum.daemon_pg.mcp_dispatch.authorize", fake_authorize)

    result = dispatch_mcp_tool_call(
        pg_conn=object(),
        router=cast(DaemonRpcRouter, router),
        name=method,
        arguments={
            "repository_id": "repo_a",
            "run_id": "run_1",
            "job_id": "job_1",
            "session_id": "sess_1",
            "lease_id": "lease_1",
            "blocker_id": "blk_1",
        },
        token="dtok.secret",
        request_id=f"req-{method.replace('.', '-')}",
    )

    assert result["isError"] is False
    structured = result["structuredContent"]
    assert isinstance(structured, dict)
    assert structured["method"] == method
    assert structured["audit_id"] == 42
    assert structured["data"] == {"routed_method": method}
    assert len(router.calls) == 1
    envelope, kwargs = router.calls[0]
    assert envelope.method == method
    assert envelope.params["repository_id"] == "repo_a"
    assert envelope.capability_token == "dtok.secret"
    assert kwargs == {
        "connection_id": "mcp",
        "transport": "mcp",
        "require_handshake": False,
    }

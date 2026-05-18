"""RFC 0023 V1.5 tests: chat_tools.execute_tool + per-tool schemas.

Direct unit tests of the tool dispatcher. Covers the closed-set
membership check, path safety, size caps, hidden-path refusal, and
wrap_tool_result delimiters.
"""

from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path
from typing import Any

from striatum.web.chat_tools import (
    ANTHROPIC_TOOLS,
    DOGFOOD_LIFECYCLE_TOOL_NAMES,
    OPENAI_TOOLS,
    TOOL_NAMES,
    execute_tool,
    tool_names,
    tool_schemas,
    wrap_tool_result,
)


def _git_init(repo: Path) -> None:
    subprocess.run(["git", "init", "-q"], cwd=repo, check=True)
    subprocess.run(["git", "checkout", "-qb", "main"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.email", "t@e.com"], cwd=repo, check=True)
    subprocess.run(["git", "config", "user.name", "t"], cwd=repo, check=True)
    (repo / "seed.txt").write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", "."], cwd=repo, check=True)
    subprocess.run(["git", "commit", "-qm", "seed", "--no-gpg-sign"], cwd=repo, check=True)


# --- closed-set ------------------------------------------------------


def test_execute_tool_closed_set_refuses_unknown(tmp_path: Path) -> None:
    out = execute_tool("rm_rf", {"path": "/"}, repo=tmp_path)
    assert out.startswith("[error] unknown tool")


def test_tool_names_match_schemas() -> None:
    """ANTHROPIC_TOOLS + OPENAI_TOOLS expose the same set as TOOL_NAMES."""
    anthropic_names = {t["name"] for t in ANTHROPIC_TOOLS}
    openai_names = {t["function"]["name"] for t in OPENAI_TOOLS}
    assert anthropic_names == TOOL_NAMES == openai_names


def test_tool_schemas_filter_write_by_mutation_gate() -> None:
    disabled = tool_names(allow_mutations=False)
    enabled = tool_names(allow_mutations=True)
    assert "generate_workflow_preview" in disabled
    assert "generate_workflow_write" not in disabled
    assert "generate_workflow_write" in enabled
    anthropic = {t["name"] for t in tool_schemas(allow_mutations=False, flavor="anthropic_messages")}
    openai = {
        t["function"]["name"]
        for t in tool_schemas(allow_mutations=False, flavor="openai_chat")
    }
    assert anthropic == openai == disabled


# --- read_file -------------------------------------------------------


def test_read_file_basic(tmp_path: Path) -> None:
    _git_init(tmp_path)
    (tmp_path / "hello.md").write_text("# hi\n", encoding="utf-8")
    out = execute_tool("read_file", {"path": "hello.md"}, repo=tmp_path)
    assert "# hi" in out


def test_read_file_path_traversal_refused(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("read_file", {"path": "../../etc/passwd"}, repo=tmp_path)
    assert out.startswith("[error]")


def test_read_file_dotgit_hidden(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("read_file", {"path": ".git/HEAD"}, repo=tmp_path)
    assert out.startswith("[error]")


def test_read_file_nonexistent(tmp_path: Path) -> None:
    out = execute_tool("read_file", {"path": "nope.md"}, repo=tmp_path)
    assert out.startswith("[error]")


def test_read_file_size_cap(tmp_path: Path) -> None:
    _git_init(tmp_path)
    big = "x" * (70 * 1024)
    (tmp_path / "big.txt").write_text(big, encoding="utf-8")
    out = execute_tool("read_file", {"path": "big.txt"}, repo=tmp_path)
    assert "[truncated" in out
    # The body before the truncation marker should be at most the cap.
    body, _, _ = out.partition("[truncated")
    assert len(body.encode("utf-8")) <= 64 * 1024 + 5


# --- list_dir --------------------------------------------------------


def test_list_dir_filters_hidden(tmp_path: Path) -> None:
    _git_init(tmp_path)
    (tmp_path / "subdir").mkdir()
    (tmp_path / "alpha.txt").write_text("a", encoding="utf-8")
    out = execute_tool("list_dir", {"path": "."}, repo=tmp_path)
    assert ".git" not in out
    assert ".striatum" not in out
    assert "alpha.txt" in out
    assert "subdir" in out


def test_list_dir_directory_not_a_directory(tmp_path: Path) -> None:
    _git_init(tmp_path)
    (tmp_path / "f.txt").write_text("x", encoding="utf-8")
    out = execute_tool("list_dir", {"path": "f.txt"}, repo=tmp_path)
    assert out.startswith("[error]")


def test_list_dir_traversal_refused(tmp_path: Path) -> None:
    out = execute_tool("list_dir", {"path": "../.."}, repo=tmp_path)
    assert out.startswith("[error]")


# --- git_log / git_diff ---------------------------------------------


def test_git_log_default_limit(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("git_log", {}, repo=tmp_path)
    assert "seed" in out


def test_git_log_respects_limit(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("git_log", {"limit": 1}, repo=tmp_path)
    assert len(out.splitlines()) == 1


def test_git_diff_clean_tree(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("git_diff", {}, repo=tmp_path)
    assert out == "[no uncommitted changes]"


def test_git_diff_dirty_tree(tmp_path: Path) -> None:
    _git_init(tmp_path)
    (tmp_path / "seed.txt").write_text("changed\n", encoding="utf-8")
    out = execute_tool("git_diff", {}, repo=tmp_path)
    assert "changed" in out


# --- striatum_status / striatum_why ---------------------------------


def test_striatum_status_in_initialized_repo(tmp_path: Path) -> None:
    _git_init(tmp_path)
    # Initialize striatum so the API can answer.
    env = os.environ.copy()
    env["PYTHONPATH"] = str(Path(__file__).resolve().parents[1] / "src")
    import sys
    subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init"],
        cwd=tmp_path, env=env, check=True, capture_output=True,
    )
    out = execute_tool("striatum_status", {}, repo=tmp_path)
    parsed = json.loads(out)
    assert parsed.get("ok") is True


def _workflow_spec() -> dict[str, Any]:
    return {
        "schema_version": "striatum.workflow_generator.v1",
        "shape": "minimal",
        "lane_set": "local",
        "workflow_id": "demo",
        "name": "Demo",
        "workflow_version": "2026-05-12",
        "branch": {"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": False},
        "scaffold_root": "workflows/demo",
        "artifact_root": "striatum/demo",
        "lanes": {},
        "options": {},
    }


def test_generate_workflow_preview_tool_writes_nothing(tmp_path: Path) -> None:
    _git_init(tmp_path)
    out = execute_tool("generate_workflow_preview", {"spec": _workflow_spec()}, repo=tmp_path)
    parsed = json.loads(out)
    assert parsed["ok"] is True
    assert parsed["data"]["workflow"]["workflow_id"] == "demo"
    assert not (tmp_path / "workflows" / "demo").exists()


def test_generate_workflow_preview_tool_routes_through_daemon_rpc(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    import striatum.service_daemon as service_daemon

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)

    calls: list[tuple[Path, str, dict[str, Any]]] = []
    rpc_only_spec = {"not": "a-valid-generator-spec"}

    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {"workflow": {"workflow_id": "rpc-demo"}, "lint": {"valid": True}}

    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    out = execute_tool("generate_workflow_preview", {"spec": rpc_only_spec}, repo=tmp_path)

    assert json.loads(out) == {
        "ok": True,
        "data": {"workflow": {"workflow_id": "rpc-demo"}, "lint": {"valid": True}},
    }
    assert calls == [(tmp_path, "workflow.generate.preview", {"spec": rpc_only_spec})]
    assert not (tmp_path / "workflows" / "demo").exists()


def test_generate_workflow_preview_tool_does_not_fallback_in_production(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
    import striatum.service_daemon as service_daemon

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)

    def fake_call_repo_method(_repo: Path, _method: str, _params: dict[str, Any]) -> dict[str, Any]:
        raise service_daemon.ServiceDaemonRpcError(
            503,
            "daemon_unreachable",
            "daemon_unreachable: workflow.generate.preview requires the Striatum daemon",
        )

    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    out = execute_tool("generate_workflow_preview", {"spec": _workflow_spec()}, repo=tmp_path)
    parsed = json.loads(out)

    assert parsed["ok"] is False
    assert parsed["error"]["code"] == "daemon_unreachable"
    assert parsed["error"]["status"] == 503
    assert not (tmp_path / "workflows" / "demo").exists()


def test_generate_workflow_write_requires_mutation_gate_and_operator_gesture(tmp_path: Path) -> None:
    _git_init(tmp_path)
    args = {"spec": _workflow_spec(), "confirm_write": True}
    disabled = execute_tool(
        "generate_workflow_write",
        args,
        repo=tmp_path,
        allow_mutations=False,
    )
    assert "mutations_disabled" in disabled
    missing_gesture = execute_tool(
        "generate_workflow_write",
        args,
        repo=tmp_path,
        allow_mutations=True,
    )
    assert "operator_gesture_missing" in missing_gesture
    written = execute_tool(
        "generate_workflow_write",
        args,
        repo=tmp_path,
        allow_mutations=True,
        operator_confirmed=True,
    )
    assert json.loads(written)["ok"] is True
    assert (tmp_path / "workflows" / "demo" / "workflow.json").exists()


# --- wrap_tool_result -----------------------------------------------


def test_wrap_tool_result_includes_delimiters() -> None:
    wrapped = wrap_tool_result("read_file", {"path": "x.md"}, "the content")
    assert "<tool_result_begin" in wrapped
    assert "<tool_result_end" in wrapped
    assert "the content" in wrapped


def test_wrap_tool_result_arg_serialization_stable() -> None:
    """Args are JSON-serialized with sorted keys for determinism."""
    a = wrap_tool_result("foo", {"a": 1, "b": 2}, "r")
    b = wrap_tool_result("foo", {"b": 2, "a": 1}, "r")
    assert a == b


# --- RFC 0040 V1 dogfood-lifecycle tools ----------------------------


def test_dogfood_lifecycle_tool_set_is_registered() -> None:
    """All twelve dogfood-lifecycle tools are present in the closed set."""
    expected = {
        "run_prepare",
        "run_start",
        "register_session",
        "supervise_start",
        "claim_next",
        "ack",
        "publish_artifact",
        "verdict",
        "complete",
        "supervise_stop",
        "run_summary",
        "evidence_export",
    }
    assert DOGFOOD_LIFECYCLE_TOOL_NAMES == expected
    assert expected.issubset(TOOL_NAMES)


def test_dogfood_lifecycle_mutation_gate_filters_writes() -> None:
    """The ten state-mutating dogfood tools are hidden when mutations disabled."""
    disabled = tool_names(allow_mutations=False)
    enabled = tool_names(allow_mutations=True)
    write_tools = {
        "run_prepare", "run_start", "register_session", "supervise_start",
        "claim_next", "ack", "publish_artifact", "verdict", "complete",
        "supervise_stop",
    }
    for name in write_tools:
        assert name not in disabled, f"{name} should be gated by mutation flag"
        assert name in enabled, f"{name} should appear with mutations enabled"
    # Read-shaped tools stay available with mutations off.
    assert "run_summary" in disabled
    assert "evidence_export" in disabled


def test_dogfood_lifecycle_mutating_tools_refuse_when_disabled(tmp_path: Path) -> None:
    out = execute_tool(
        "ack",
        {"session_id": "s", "message_id": "m", "lease_id": "l"},
        repo=tmp_path,
        allow_mutations=False,
    )
    assert "mutations_disabled" in out


def test_dogfood_lifecycle_missing_argument_reports_clean_error(tmp_path: Path) -> None:
    out = execute_tool(
        "ack",
        {"session_id": "s", "message_id": "m"},
        repo=tmp_path,
        allow_mutations=True,
    )
    assert "missing_argument" in out
    assert "lease_id" in out


def test_dogfood_lifecycle_verdict_enum_validated(tmp_path: Path) -> None:
    out = execute_tool(
        "verdict",
        {
            "session_id": "s",
            "job_id": "j",
            "lease_id": "l",
            "verdict": "thumbs_up",
        },
        repo=tmp_path,
        allow_mutations=True,
    )
    assert "invalid_argument" in out
    assert "thumbs_up" in out


def test_chat_status_routes_mapped_read_through_daemon_rpc(
    tmp_path: Path, monkeypatch: Any
) -> None:
    import striatum.api as api
    import striatum.service_daemon as service_daemon

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)

    def invoke_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("chat status fell back to striatum.api.invoke")

    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {"method": method, "items": []}

    monkeypatch.setattr(api, "invoke", invoke_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    out = execute_tool("striatum_status", {}, repo=tmp_path, allow_mutations=False)

    assert json.loads(out) == {"ok": True, "data": {"method": "status", "items": []}}
    assert calls == [(tmp_path, "status", {})]


def test_chat_lifecycle_routes_mapped_mutation_through_daemon_rpc(
    tmp_path: Path, monkeypatch: Any
) -> None:
    import striatum.api as api
    import striatum.service_daemon as service_daemon

    monkeypatch.delenv("STRIATUM_TEST_HARNESS", raising=False)
    monkeypatch.delenv("STRIATUM_DAEMON_REQUIRED", raising=False)

    def invoke_tripwire(*args: Any, **kwargs: Any) -> None:
        raise AssertionError("chat lifecycle fell back to striatum.api.invoke")

    calls: list[tuple[Path, str, dict[str, Any]]] = []

    def fake_call_repo_method(repo: Path, method: str, params: dict[str, Any]) -> dict[str, Any]:
        calls.append((repo, method, dict(params)))
        return {"method": method, "run_id": params["run_id"]}

    monkeypatch.setattr(api, "invoke", invoke_tripwire)
    monkeypatch.setattr(service_daemon, "call_repo_method", fake_call_repo_method)

    out = execute_tool(
        "run_start",
        {"run_id": "run_1"},
        repo=tmp_path,
        allow_mutations=True,
    )

    assert json.loads(out) == {
        "ok": True,
        "data": {"method": "run.start", "run_id": "run_1"},
    }
    assert calls == [(tmp_path, "run.start", {"run_id": "run_1"})]


def test_dogfood_lifecycle_run_summary_returns_invoke_envelope(tmp_path: Path) -> None:
    """run_summary is read-shaped: callable without --allow-mutations.

    In the legacy fixture harness it still uses the local invoke fallback.
    Without a real run the underlying CLI returns a structured error envelope,
    which is what we serialize.
    """
    _git_init(tmp_path)
    env = os.environ.copy()
    env["PYTHONPATH"] = str(Path(__file__).resolve().parents[1] / "src")
    import sys
    subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(tmp_path), "init"],
        cwd=tmp_path, env=env, check=True, capture_output=True,
    )
    out = execute_tool(
        "run_summary",
        {"run_id": "run_missing", "path": "scratch/summary.json"},
        repo=tmp_path,
        allow_mutations=False,
    )
    parsed = json.loads(out)
    assert parsed.get("ok") is False
    assert "error" in parsed


def test_dogfood_lifecycle_schemas_round_trip_through_both_flavors() -> None:
    """All dogfood tools surface in both Anthropic and OpenAI schemas."""
    anthropic_names = {
        t["name"]
        for t in tool_schemas(allow_mutations=True, flavor="anthropic_messages")
    }
    openai_names = {
        t["function"]["name"]
        for t in tool_schemas(allow_mutations=True, flavor="openai_chat")
    }
    assert DOGFOOD_LIFECYCLE_TOOL_NAMES.issubset(anthropic_names)
    assert DOGFOOD_LIFECYCLE_TOOL_NAMES.issubset(openai_names)


def test_register_session_argv_default_is_fresh(tmp_path: Path) -> None:
    """register_session defaults to --fresh per RFC 0040 §1 tool signature."""
    from striatum.web.chat_tools import _dogfood_argv

    argv = _dogfood_argv(
        "register_session",
        {"run_id": "r", "role": "author", "lane": "claude_code"},
    )
    assert "--fresh" in argv

    argv_no_fresh = _dogfood_argv(
        "register_session",
        {"run_id": "r", "role": "author", "lane": "claude_code", "fresh": False},
    )
    assert "--fresh" not in argv_no_fresh


def test_claim_next_lease_seconds_validated(tmp_path: Path) -> None:
    out = execute_tool(
        "claim_next",
        {"session_id": "s", "lease_seconds": 5},
        repo=tmp_path,
        allow_mutations=True,
    )
    assert "invalid_argument" in out

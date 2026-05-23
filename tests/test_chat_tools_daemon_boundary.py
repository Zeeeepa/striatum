from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from striatum.web.chat_tools import execute_tool


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

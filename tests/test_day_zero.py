from __future__ import annotations

import argparse
from pathlib import Path
from typing import Any

from striatum.cli.dispatch import dispatch
from striatum.day_zero import first_run_smoke, service_install, service_start, service_status


def test_service_install_dry_run_renders_systemd_unit(tmp_path: Path, monkeypatch: Any) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))

    result = service_install(manager="systemd", dry_run=True)

    assert result["status"] == "would_write"
    assert result["path"].endswith("systemd/user/striatumd.service")
    assert "ExecStart=" in str(result["content"])
    assert "daemon start" in str(result["content"])
    assert not Path(str(result["path"])).exists()


def test_service_start_and_status_wrap_manager_commands(monkeypatch: Any) -> None:
    calls: list[list[str]] = []

    def _fake_run(command: list[str]) -> dict[str, Any]:
        calls.append(command)
        return {"argv": command, "returncode": 0, "stdout": "active\n", "stderr": ""}

    monkeypatch.setattr("striatum.day_zero._run", _fake_run)

    started = service_start(manager="systemd")
    status = service_status(manager="systemd")

    assert started["status"] == "ok"
    assert status["status"] == "ok"
    assert calls == [
        ["systemctl", "--user", "start", "striatumd.service"],
        ["systemctl", "--user", "is-active", "striatumd.service"],
    ]


def test_adopt_dry_run_is_daemon_optional_and_reports_guided_steps(tmp_path: Path, monkeypatch: Any) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))

    args = argparse.Namespace(
        command="adopt",
        repo=str(repo),
        daemon=False,
        profile="generic",
        dry_run=True,
        with_skills=False,
        with_plugins=False,
        with_ddd_layout=True,
        register=False,
        postgres_url=None,
    )

    result = dispatch(args)

    assert isinstance(result, dict)
    assert result["dry_run"] is True
    assert result["init"]["status"] == "would_init"
    assert result["suggested_workflow"].endswith("workflows/first-workflow/workflow.json")
    assert result["ddd_layout"]["dry_run"] is True


def test_first_run_smoke_reports_checks_without_leaking_token(tmp_path: Path, monkeypatch: Any) -> None:
    monkeypatch.setattr("striatum.day_zero.resolve_socket_path", lambda: tmp_path / "striatumd.sock")
    monkeypatch.setattr("striatum.day_zero.daemon_socket_is_reachable", lambda _socket: True)
    monkeypatch.setattr("striatum.day_zero._read_runtime_token_value", lambda: "tok.secret")
    monkeypatch.setattr("striatum.day_zero._lookup_repository_id", lambda _repo: "repo_1")
    monkeypatch.setattr("striatum.day_zero.pg_doctor", lambda: {"ok": True, "status": "ok"})
    monkeypatch.setattr(
        "striatum.day_zero._mcp_capability_check",
        lambda **_: {"id": "mcp_capability", "ok": True, "tool_count": 1},
    )
    monkeypatch.setattr(
        "striatum.day_zero._sample_read_route_check",
        lambda **_: {"id": "sample_read_route", "ok": True, "response_ok": True},
    )

    result = first_run_smoke(tmp_path)

    assert result["ok"] is True
    assert "tok.secret" not in repr(result)
    assert {check["id"] for check in result["checks"]} == {
        "daemon_socket",
        "runtime_token",
        "postgres",
        "repo_registration",
        "mcp_capability",
        "sample_read_route",
    }

from __future__ import annotations

import argparse
from pathlib import Path
from typing import Any

import striatum
from striatum.cli.dispatch import dispatch
from striatum.day_zero import _call_rpc_sequence, first_run_smoke, service_install, service_start, service_status


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


def test_first_run_rpc_sequence_uses_striatum_version_for_hello(
    tmp_path: Path,
    monkeypatch: Any,
) -> None:
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
                b'{"ok": true, "data": {"doctor": "ok"}}\n',
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

    monkeypatch.setattr("socket.socket", lambda *_args: FakeSocket())
    monkeypatch.setattr("striatum.daemon_rpc.client.hello_envelope", fake_hello_envelope)

    response = _call_rpc_sequence(tmp_path / "striatumd.sock", FakeEnvelope(b"doctor"))

    assert response == {"ok": True, "data": {"doctor": "ok"}}
    assert captured["client_name"] == "striatum-first-run"
    assert captured["client_version"] == striatum.__version__


def test_day_zero_source_has_no_legacy_handshake_versions() -> None:
    source = Path(__file__).resolve().parents[1] / "src" / "striatum" / "day_zero.py"
    text = source.read_text(encoding="utf-8")
    legacy_versions = {".".join(("1", "51", "0")), ".".join(("1", "67", "0"))}

    assert [version for version in legacy_versions if version in text] == []

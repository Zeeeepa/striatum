"""RFC 0043 §3 — ``daemon doctor`` works without a daemon.

The doctor command must be reachable when the daemon is down so the
operator can diagnose configuration. Track B keeps the doctor in the
``DAEMON_OPTIONAL_COMMANDS`` allowlist so the daemon-required hook does
not refuse it with exit code 11.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from striatum.cli.daemon_required import (
    DAEMON_OPTIONAL_COMMANDS,
    ENV_DAEMON_REQUIRED,
    ENV_DAEMON_SOCKET,
    enforce_daemon_required,
)


@pytest.fixture(autouse=True)
def _clear_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv(ENV_DAEMON_REQUIRED, raising=False)
    monkeypatch.delenv(ENV_DAEMON_SOCKET, raising=False)


def test_daemon_command_in_allowlist() -> None:
    assert "daemon" in DAEMON_OPTIONAL_COMMANDS


def test_daemon_doctor_not_refused_when_socket_missing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "no-socket"))
    # No DaemonUnreachableError — ``daemon`` family commands bypass the hook.
    enforce_daemon_required("daemon", tmp_path)


def test_init_command_in_allowlist(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "no-socket"))
    enforce_daemon_required("init", tmp_path)


def test_skills_and_plugin_in_allowlist(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "no-socket"))
    enforce_daemon_required("skills", tmp_path)
    enforce_daemon_required("plugin", tmp_path)

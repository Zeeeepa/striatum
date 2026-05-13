"""RFC 0043 §3 exit-code coverage.

Asserts the daemon-required CLI surface exits with the documented codes
and the documented stderr remediation blocks. These tests do not depend on
a running daemon; they exercise the wiring Track B owns: error classes,
stderr templates, JSON-envelope hints, and the env-gated dispatch hook.
"""

from __future__ import annotations

import io
import json
import os
import sys
from pathlib import Path

import pytest

from striatum.cli import dispatch as dispatch_mod
from striatum.cli.daemon_required import (
    ENV_DAEMON_REQUIRED,
    ENV_DAEMON_SOCKET,
    enforce_daemon_required,
    render_daemon_unreachable_hint,
    render_daemon_unreachable_message,
    render_repo_not_migrated_hint,
    render_repo_not_migrated_message,
    resolve_requirement,
    resolve_socket_path,
)
from striatum.errors import (
    EXIT_DAEMON_UNREACHABLE,
    EXIT_REPO_NOT_MIGRATED,
    DaemonUnreachableError,
    RepoNotMigratedError,
)


@pytest.fixture(autouse=True)
def _clear_daemon_required_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv(ENV_DAEMON_REQUIRED, raising=False)
    monkeypatch.delenv(ENV_DAEMON_SOCKET, raising=False)


def test_daemon_unreachable_message_lists_remediation(tmp_path: Path) -> None:
    socket_path = tmp_path / "striatumd.sock"
    text = render_daemon_unreachable_message(socket_path)
    # Code-named first line + the four remediation channels demanded by
    # the design synthesis stderr template.
    assert text.startswith(
        f"daemon_unreachable: could not connect to Striatum daemon at {socket_path}"
    )
    assert "Linux systemd: systemctl --user start striatumd" in text
    assert "macOS launchd: launchctl bootstrap" in text
    assert "Foreground: striatumd --foreground" in text
    assert "Postgres" in text


def test_daemon_unreachable_hint_is_a_single_line(tmp_path: Path) -> None:
    hint = render_daemon_unreachable_hint(tmp_path / "x.sock")
    assert "\n" not in hint
    assert "striatum daemon doctor" in hint


def test_repo_not_migrated_message_names_migrate_verb(tmp_path: Path) -> None:
    text = render_repo_not_migrated_message(tmp_path)
    assert text.startswith(
        f"repo_not_migrated: {tmp_path} has not been migrated to daemon PostgreSQL state"
    )
    assert (
        f"Run: striatum daemon migrate-repo-local --from sqlite --to pg --repo {tmp_path}"
        in text
    )


def test_repo_not_migrated_hint_is_a_single_line(tmp_path: Path) -> None:
    hint = render_repo_not_migrated_hint(tmp_path)
    assert "\n" not in hint
    assert "striatum daemon migrate-repo-local" in hint


def test_daemon_unreachable_error_uses_exit_code_11(tmp_path: Path) -> None:
    err = DaemonUnreachableError(
        render_daemon_unreachable_message(tmp_path / "s"),
        hint=render_daemon_unreachable_hint(tmp_path / "s"),
    )
    assert err.exit_code == EXIT_DAEMON_UNREACHABLE == 11
    assert err.hint is not None


def test_repo_not_migrated_error_uses_exit_code_12(tmp_path: Path) -> None:
    err = RepoNotMigratedError(
        render_repo_not_migrated_message(tmp_path),
        hint=render_repo_not_migrated_hint(tmp_path),
    )
    assert err.exit_code == EXIT_REPO_NOT_MIGRATED == 12
    assert err.hint is not None


def test_enforce_daemon_required_no_op_by_default(tmp_path: Path) -> None:
    # Without the env flag the helper is inert so the historical SQLite
    # path keeps working.
    enforce_daemon_required("status", tmp_path)


def test_enforce_daemon_required_skips_lifecycle_commands(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    # ``daemon``, ``init``, ``skills``, ``plugin`` must run even when
    # enforcement is on and the socket is missing (they manage the daemon
    # itself or touch installer files only).
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "does-not-exist"))
    for command in ("daemon", "init", "skills", "plugin"):
        enforce_daemon_required(command, tmp_path)


def test_enforce_daemon_required_raises_unreachable_when_socket_missing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "no-socket"))
    with pytest.raises(DaemonUnreachableError) as exc:
        enforce_daemon_required("status", tmp_path)
    assert exc.value.exit_code == 11
    assert "daemon_unreachable" in str(exc.value)


def test_enforce_daemon_required_raises_repo_not_migrated_when_socket_listens(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    import socket as socket_mod

    socket_path = tmp_path / "striatumd.sock"
    listener = socket_mod.socket(socket_mod.AF_UNIX, socket_mod.SOCK_STREAM)
    try:
        listener.bind(str(socket_path))
        listener.listen(1)
        monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")
        monkeypatch.setenv(ENV_DAEMON_SOCKET, str(socket_path))
        # Repo presents the pre-cutover signal: a .striatum/state.sqlite3
        # without a tombstone marker. Track B's helper uses that as the
        # "unmigrated" stand-in until Track A's repo_migrations row check
        # lands.
        (tmp_path / ".striatum").mkdir()
        (tmp_path / ".striatum" / "state.sqlite3").write_bytes(b"")
        with pytest.raises(RepoNotMigratedError) as exc:
            enforce_daemon_required("status", tmp_path)
        assert exc.value.exit_code == 12
        assert "repo_not_migrated" in str(exc.value)
    finally:
        listener.close()


def test_dispatch_emits_remediation_block_in_stderr(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "no-socket"))
    rc = dispatch_mod.main(["--repo", str(tmp_path), "status"])
    assert rc == 11
    captured = capsys.readouterr()
    assert "daemon_unreachable" in captured.err
    assert "systemctl --user start striatumd" in captured.err


def test_dispatch_json_envelope_includes_hint(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "1")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "no-socket"))
    rc = dispatch_mod.main(
        ["--repo", str(tmp_path), "status", "--json"]
    )
    assert rc == 11
    payload = json.loads(capsys.readouterr().out)
    assert payload == {
        "ok": False,
        "error": {
            "message": payload["error"]["message"],
            "code": 11,
            "hint": payload["error"]["hint"],
        },
    }
    assert payload["error"]["message"].startswith("daemon_unreachable:")
    assert "striatum daemon doctor" in payload["error"]["hint"]


def test_resolve_requirement_returns_none_without_env(tmp_path: Path) -> None:
    assert resolve_requirement("status") is None


def test_resolve_socket_path_respects_override(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv(ENV_DAEMON_SOCKET, "/tmp/custom.sock")
    assert resolve_socket_path() == Path("/tmp/custom.sock")

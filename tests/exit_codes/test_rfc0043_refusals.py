"""RFC 0043 §3 exit-code coverage.

Asserts the daemon-required CLI surface exits with the documented codes
and the documented stderr remediation blocks. Most tests exercise the
wiring Track B owns without a running daemon: error classes, stderr
templates, JSON-envelope hints, and the env-gated dispatch hook. One
``multi_repo`` smoke uses the foreground daemon harness to keep the
reachable-socket refusal path honest.
"""

from __future__ import annotations

import importlib
import json
from pathlib import Path

import pytest
from _harness.multi_repo import MultiRepoHarness

dispatch_mod = importlib.import_module("striatum.cli.dispatch")

from striatum.cli.daemon_required import (  # noqa: E402  (must follow importlib bind)
    DaemonRequirement,
    ENV_DAEMON_REQUIRED,
    ENV_DAEMON_SOCKET,
    ENV_TEST_HARNESS,
    enforce_daemon_required,
    render_daemon_unreachable_hint,
    render_daemon_unreachable_message,
    render_repo_not_migrated_hint,
    render_repo_not_migrated_message,
    resolve_requirement,
    resolve_socket_path,
)
from striatum.errors import (  # noqa: E402  (must follow importlib bind)
    EXIT_DAEMON_UNREACHABLE,
    EXIT_REPO_NOT_MIGRATED,
    DaemonUnreachableError,
    RepoNotMigratedError,
)


@pytest.fixture(autouse=True)
def _clear_daemon_required_env(monkeypatch: pytest.MonkeyPatch) -> None:
    # V1.5: daemon-required is the default. Removing the env at function
    # scope (which overrides the session-level opt-out from
    # ``tests/conftest.py``) exercises the new mandatory enforcement.
    # V1.6 F-escape: also clear ``STRIATUM_TEST_HARNESS`` so each test
    # asserts the production code path; tests that exercise the paired
    # opt-out set both vars explicitly.
    monkeypatch.delenv(ENV_DAEMON_REQUIRED, raising=False)
    monkeypatch.delenv(ENV_DAEMON_SOCKET, raising=False)
    monkeypatch.delenv(ENV_TEST_HARNESS, raising=False)


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
    assert "Foreground: striatumd" in text
    assert "Postgres" in text


def test_daemon_unreachable_hint_is_a_single_line(tmp_path: Path) -> None:
    hint = render_daemon_unreachable_hint(tmp_path / "x.sock")
    assert "\n" not in hint
    assert "striatum daemon doctor" in hint


def test_repo_not_migrated_message_names_retired_import_window(tmp_path: Path) -> None:
    text = render_repo_not_migrated_message(tmp_path)
    assert text.startswith(
        f"repo_not_migrated: {tmp_path} has not been migrated to daemon PostgreSQL state"
    )
    assert "Retired import windows are closed" in text
    assert "striatum adopt" in text
    assert "striatum repo add --init" in text


def test_repo_not_migrated_hint_is_a_single_line(tmp_path: Path) -> None:
    hint = render_repo_not_migrated_hint(tmp_path)
    assert "\n" not in hint
    assert "striatum adopt" in hint
    assert "striatum repo add --init" in hint


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


def test_enforce_daemon_required_skips_lifecycle_commands(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    # ``daemon``, ``init``, ``skills``, ``plugin`` must run even when
    # enforcement is on and the socket is missing (they manage the daemon
    # itself or touch installer files only).
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "does-not-exist"))
    for command in ("daemon", "init", "skills", "plugin"):
        enforce_daemon_required(command, tmp_path)


def test_enforce_daemon_required_raises_unreachable_when_socket_missing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    # V1.5: enforcement is the default — no env opt-in needed.
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


def test_resolve_requirement_enforces_by_default_when_env_unset() -> None:
    # V1.5: daemon-required is the default. With the env var unset the
    # resolver returns a populated DaemonRequirement, not None.
    requirement = resolve_requirement("status")
    assert isinstance(requirement, DaemonRequirement)
    assert requirement.enforced is True


def test_resolve_requirement_paired_opt_out_is_recognized(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """V1.6 F-escape: the opt-out requires BOTH env vars.

    ``STRIATUM_DAEMON_REQUIRED=0`` paired with ``STRIATUM_TEST_HARNESS=1``
    returns ``None`` so the legacy local state-backed test fixtures stay green
    during the V1.6 → V2.0 substrate migration.
    """
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "0")
    monkeypatch.setenv(ENV_TEST_HARNESS, "1")
    assert resolve_requirement("status") is None


def test_resolve_requirement_bare_env_zero_still_enforces(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """V1.6 F-escape regression: the bare ``STRIATUM_DAEMON_REQUIRED=0``
    opt-out without the test-harness marker is **not** honored anymore.

    Closes the codex dogfood-050 threat-model finding that flagged the
    documented operator escape path. Production environments that
    accidentally export the env var (e.g. shell rc, CI matrix) still see
    enforced daemon-required mode.
    """
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "0")
    # STRIATUM_TEST_HARNESS deliberately unset (autouse fixture).
    requirement = resolve_requirement("status")
    assert isinstance(requirement, DaemonRequirement)
    assert requirement.enforced is True


def test_resolve_requirement_bare_test_harness_does_not_opt_out(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """``STRIATUM_TEST_HARNESS=1`` alone does not opt out either; the
    paired form is the only opt-out shape.
    """
    monkeypatch.setenv(ENV_TEST_HARNESS, "1")
    requirement = resolve_requirement("status")
    assert isinstance(requirement, DaemonRequirement)
    assert requirement.enforced is True


def test_dispatch_bare_env_zero_still_exits_11(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """End-to-end: bare ``STRIATUM_DAEMON_REQUIRED=0`` (no test harness
    marker) still refuses with exit code 11 when the socket is missing.

    The previous V1.5 behavior treated the bare env var as the operator
    escape hatch and would have returned 0 by skipping the gate. The
    V1.6 F-escape change removes that bypass.
    """
    monkeypatch.setenv(ENV_DAEMON_REQUIRED, "0")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "no-socket"))
    rc = dispatch_mod.main(["--repo", str(tmp_path), "status"])
    assert rc == 11
    captured = capsys.readouterr()
    assert "daemon_unreachable" in captured.err


def test_resolve_socket_path_respects_override(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv(ENV_DAEMON_SOCKET, "/tmp/custom.sock")
    assert resolve_socket_path() == Path("/tmp/custom.sock")


# RFC 0043 V1.5 F-test: end-to-end exit-code-12 coverage --------------


def test_dispatch_returns_exit_12_for_unmigrated_repo(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """End-to-end: an unmigrated repo with a reachable daemon socket
    exits with code 12 and the retired retired local state-import guidance.

    The default flip means we exercise this without setting
    ``STRIATUM_DAEMON_REQUIRED=1`` — the unset env reaches the
    enforcement path.
    """
    import socket as socket_mod

    socket_path = tmp_path / "striatumd.sock"
    listener = socket_mod.socket(socket_mod.AF_UNIX, socket_mod.SOCK_STREAM)
    try:
        listener.bind(str(socket_path))
        listener.listen(1)
        monkeypatch.setenv(ENV_DAEMON_SOCKET, str(socket_path))
        # Pre-cutover disk signal — a state.sqlite3 with no tombstone.
        (tmp_path / ".striatum").mkdir()
        (tmp_path / ".striatum" / "state.sqlite3").write_bytes(b"")

        rc = dispatch_mod.main(["--repo", str(tmp_path), "status"])
        assert rc == 12
        captured = capsys.readouterr()
        assert "repo_not_migrated" in captured.err
        assert "Retired import windows are closed" in captured.err
        assert "striatum repo add --init" in captured.err
        # The hint names the resolved repo path so operators can copy the
        # command verbatim.
        assert str(tmp_path.resolve()) in captured.err
    finally:
        listener.close()


def test_dispatch_exit_12_json_envelope(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """The --json error envelope for exit 12 carries the structured hint."""
    import socket as socket_mod

    socket_path = tmp_path / "striatumd.sock"
    listener = socket_mod.socket(socket_mod.AF_UNIX, socket_mod.SOCK_STREAM)
    try:
        listener.bind(str(socket_path))
        listener.listen(1)
        monkeypatch.setenv(ENV_DAEMON_SOCKET, str(socket_path))
        (tmp_path / ".striatum").mkdir()
        (tmp_path / ".striatum" / "state.sqlite3").write_bytes(b"")

        rc = dispatch_mod.main(["--repo", str(tmp_path), "status", "--json"])
        assert rc == 12
        payload = json.loads(capsys.readouterr().out)
        assert payload["ok"] is False
        assert payload["error"]["code"] == 12
        assert payload["error"]["message"].startswith("repo_not_migrated:")
        assert "striatum repo add --init" in payload["error"]["hint"]
    finally:
        listener.close()


def test_repo_list_does_not_refuse_when_local_sqlite_state_exists(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """GH #25: ``repo list`` is a daemon-global registry read. The
    legacy ``.striatum/state.sqlite3`` preflight must not fire for the
    listing verb — it stays in ``adopt`` / ``repo add --init`` where
    setup is happening. ``enforce_daemon_required`` with
    ``check_repo_migration=False`` is the seam the dispatcher uses.
    """
    import socket as socket_mod

    socket_path = tmp_path / "striatumd.sock"
    listener = socket_mod.socket(socket_mod.AF_UNIX, socket_mod.SOCK_STREAM)
    try:
        listener.bind(str(socket_path))
        listener.listen(1)
        monkeypatch.setenv(ENV_DAEMON_SOCKET, str(socket_path))
        (tmp_path / ".striatum").mkdir()
        (tmp_path / ".striatum" / "state.sqlite3").write_bytes(b"")

        # Default behavior (mutation-style path): still refuses.
        with pytest.raises(RepoNotMigratedError):
            enforce_daemon_required("repo", tmp_path)

        # The list-side seam: no refusal even with the legacy file
        # present.
        enforce_daemon_required("repo", tmp_path, check_repo_migration=False)
    finally:
        listener.close()


def test_repo_list_unreachable_daemon_reports_daemon_unreachable(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """GH #25: with the daemon socket unreachable, ``repo list`` exits
    with the documented ``daemon_unreachable`` refusal (exit code 11),
    not the legacy ``repo_not_migrated`` local-state error.
    """
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(tmp_path / "no-socket"))
    (tmp_path / ".striatum").mkdir()
    (tmp_path / ".striatum" / "state.sqlite3").write_bytes(b"")

    rc = dispatch_mod.main(["--repo", str(tmp_path), "repo", "list"])

    assert rc == 11
    captured = capsys.readouterr()
    assert "daemon_unreachable" in captured.err
    assert "repo_not_migrated" not in captured.err


@pytest.mark.multi_repo
def test_dispatch_exit_12_json_envelope_with_foreground_daemon_socket(
    tmp_path: Path,
    multi_repo_harness: MultiRepoHarness,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
) -> None:
    """Live smoke: a reachable foreground daemon socket still refuses an
    unmigrated repo before any local-state fallback can run.
    """
    repo = tmp_path / "repo"
    (repo / ".striatum").mkdir(parents=True)
    (repo / ".striatum" / "state.sqlite3").write_bytes(b"")
    monkeypatch.setenv(ENV_DAEMON_SOCKET, str(multi_repo_harness.socket_path))

    rc = dispatch_mod.main(["--repo", str(repo), "status", "--json"])

    assert rc == 12
    payload = json.loads(capsys.readouterr().out)
    assert payload["ok"] is False
    assert payload["error"]["code"] == 12
    assert payload["error"]["message"].startswith("repo_not_migrated:")
    assert "striatum repo add --init" in payload["error"]["hint"]

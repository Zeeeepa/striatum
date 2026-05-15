"""Tests for ``striatum self-update``.

The verb chains pip install -e + plugin install + skills install. The
dry-run path is fully unit-testable; the live path is partially mocked
to avoid mutating the test runner's environment.
"""

from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Any

import pytest

from striatum.cli.parser import build_parser


def _parse(argv: list[str]) -> Any:
    parser = build_parser()
    return parser.parse_args(["self-update", *argv])


def test_self_update_parser_defaults() -> None:
    args = _parse([])
    assert args.command == "self-update"
    assert args.profile == "claude_code"
    assert args.scope == "user"
    assert args.dry_run is False
    assert args.skip_plugin is False
    assert args.skip_skills is False
    # Source defaults to the repo root containing this CLI module.
    assert Path(args.source).is_dir()


def test_self_update_parser_choices_constrain_profile() -> None:
    parser = build_parser()
    with pytest.raises(SystemExit):
        parser.parse_args(["self-update", "--profile", "bogus"])


def _plan(result: dict[str, Any]) -> dict[str, Any]:
    """Narrow result["plan"] for mypy strict."""
    plan = result["plan"]
    assert isinstance(plan, dict)
    return plan


def test_self_update_dry_run_returns_plan_envelope() -> None:
    from striatum.cli.dispatch import _dispatch_self_update

    args = _parse(["--dry-run", "--profile", "codex", "--scope", "project"])
    result = _dispatch_self_update(args)
    assert result["status"] == "would_run"
    plan = _plan(result)
    assert plan["profile"] == "codex"
    assert plan["scope"] == "project"
    assert "pip install" in plan["pip"]
    assert "--force-reinstall" in plan["pip"]
    assert "--no-deps" in plan["pip"]
    assert "plugin install --profile codex" in plan["plugin"]
    assert "skills install --profile codex" in plan["skills"]


def test_self_update_dry_run_respects_skip_flags() -> None:
    from striatum.cli.dispatch import _dispatch_self_update

    args = _parse(["--dry-run", "--skip-plugin", "--skip-skills"])
    result = _dispatch_self_update(args)
    plan = _plan(result)
    assert plan["plugin"] is None
    assert plan["skills"] is None


def test_self_update_dry_run_skills_falls_back_when_profile_all() -> None:
    """The skills installer doesn't accept profile=all directly; the
    handler downgrades to claude_code for skills while keeping
    profile=all for plugin install."""
    from striatum.cli.dispatch import _dispatch_self_update

    args = _parse(["--dry-run", "--profile", "all"])
    result = _dispatch_self_update(args)
    plan = _plan(result)
    assert "plugin install --profile all" in plan["plugin"]
    assert "skills install --profile claude_code" in plan["skills"]


def test_self_update_live_run_invokes_pip_first(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """The live path invokes the pip command first; failure short-circuits."""
    from striatum.cli.dispatch import _dispatch_self_update
    from striatum.errors import StriatumError

    captured: list[list[str]] = []

    class _FakeProc:
        def __init__(self, exit_code: int) -> None:
            self.returncode = exit_code
            self.stdout = "ok-out"
            self.stderr = "fake-stderr"

    def fake_run(cmd: list[str], *, capture_output: bool, text: bool) -> _FakeProc:
        captured.append(cmd)
        # Fail the pip step to verify short-circuit
        if cmd[:3] == [cmd[0], "-m", "pip"] or "pip" in (cmd[0] if cmd else ""):
            return _FakeProc(1)
        if cmd[0:2] == ["striatum", "plugin"]:
            return _FakeProc(0)
        if cmd[0:2] == ["striatum", "skills"]:
            return _FakeProc(0)
        return _FakeProc(0)

    monkeypatch.setattr(subprocess, "run", fake_run)
    args = _parse(["--source", str(tmp_path), "--profile", "codex"])
    with pytest.raises(StriatumError) as exc_info:
        _dispatch_self_update(args)
    assert exc_info.value.exit_code == 8
    # Only the pip step ran before short-circuit; plugin + skills skipped.
    assert len(captured) == 1


def test_self_update_live_run_continues_past_optional_failures(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """When pip succeeds but plugin/skills fail, the envelope still
    reports `status=updated` — those steps are best-effort."""
    from striatum.cli.dispatch import _dispatch_self_update

    captured: list[list[str]] = []

    class _FakeProc:
        def __init__(self, exit_code: int) -> None:
            self.returncode = exit_code
            self.stdout = "ok-out"
            self.stderr = "fake-stderr"

    def fake_run(cmd: list[str], *, capture_output: bool, text: bool) -> _FakeProc:
        captured.append(cmd)
        if cmd[0:2] == ["striatum", "plugin"]:
            return _FakeProc(2)  # non-fatal
        if cmd[0:2] == ["striatum", "skills"]:
            return _FakeProc(3)  # non-fatal
        return _FakeProc(0)  # pip ok

    monkeypatch.setattr(subprocess, "run", fake_run)
    args = _parse(["--source", str(tmp_path), "--profile", "codex"])
    result = _dispatch_self_update(args)
    assert result["status"] == "updated"
    steps_obj = result["steps"]
    assert isinstance(steps_obj, list)
    steps: list[dict[str, Any]] = steps_obj
    labels = [step["label"] for step in steps]
    assert labels == ["pip", "plugin", "skills"]
    # Plugin and skills both report their non-zero exit codes.
    plugin_step = next(s for s in steps if s["label"] == "plugin")
    skills_step = next(s for s in steps if s["label"] == "skills")
    assert plugin_step["exit_code"] == 2
    assert skills_step["exit_code"] == 3


def test_self_update_json_envelope_double_wrap_avoided(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """The handler returns the inner data dict (status + plan + steps),
    NOT a pre-wrapped `{"ok": …, "data": …}` envelope — the outer CLI
    wrapper does that. We verify by inspecting the dict shape: no
    top-level `ok` or `error` keys."""
    from striatum.cli.dispatch import _dispatch_self_update

    args = _parse(["--dry-run"])
    result = _dispatch_self_update(args)
    assert "ok" not in result
    assert "error" not in result
    assert "status" in result
    assert "plan" in result

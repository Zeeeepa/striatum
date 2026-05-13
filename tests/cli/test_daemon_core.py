from __future__ import annotations

from pathlib import Path

import pytest

from striatum.cli.daemon import ENV_DAEMON_CORE, ENV_GO_BIN, resolve_daemon_core, resolve_go_binary
from striatum.cli.parser import build_parser
from striatum.errors import StriatumError


def test_daemon_start_core_go_parses() -> None:
    args = build_parser().parse_args(["daemon", "start", "--core", "go"])

    assert args.daemon_command == "start"
    assert args.core == "go"


def test_daemon_start_core_env_default(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv(ENV_DAEMON_CORE, "go")

    args = build_parser().parse_args(["daemon", "start"])

    assert args.core == "go"


def test_daemon_core_default_is_python(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv(ENV_DAEMON_CORE, raising=False)

    assert resolve_daemon_core(None) == "python"


def test_daemon_core_flag_wins_over_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv(ENV_DAEMON_CORE, "python")

    assert resolve_daemon_core("go") == "go"


def test_daemon_core_rejects_unknown(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv(ENV_DAEMON_CORE, "rust")

    with pytest.raises(StriatumError) as exc:
        resolve_daemon_core(None)

    assert exc.value.exit_code == 2


def test_go_binary_env_override(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    binary = tmp_path / "striatumd"
    binary.write_text("#!/bin/sh\n", encoding="utf-8")
    monkeypatch.setenv(ENV_GO_BIN, str(binary))

    assert resolve_go_binary() == binary.resolve()

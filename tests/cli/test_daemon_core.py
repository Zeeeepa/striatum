from __future__ import annotations

from pathlib import Path

import pytest

from striatum.cli.daemon import (
    ENV_DAEMON_CORE,
    ENV_GO_BIN,
    _parse_go_describe,
    resolve_daemon_core,
    resolve_go_binary,
)
from striatum.cli.parser import build_parser
from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION
from striatum.daemon_rpc.registry import METHODS_ETAG
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
    binary.write_text(
        "#!/bin/sh\n"
        "if [ \"$1\" = \"--describe\" ]; then\n"
        f"  echo core=go supported_schema={LATEST_DAEMON_DB_VERSION} migration_count={LATEST_DAEMON_DB_VERSION} methods_etag={METHODS_ETAG}\n"
        "  exit 0\n"
        "fi\n",
        encoding="utf-8",
    )
    binary.chmod(0o755)
    monkeypatch.setenv(ENV_GO_BIN, str(binary))

    assert resolve_go_binary() == binary.resolve()


def test_packaged_go_resolver_accepts_find_binary(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    binary = tmp_path / "striatumd"
    binary.write_text(
        "#!/bin/sh\n"
        "if [ \"$1\" = \"--describe\" ]; then\n"
        f"  echo core=go supported_schema={LATEST_DAEMON_DB_VERSION} migration_count={LATEST_DAEMON_DB_VERSION} methods_etag={METHODS_ETAG}\n"
        "  exit 0\n"
        "fi\n",
        encoding="utf-8",
    )
    binary.chmod(0o755)

    class FakeDaemonGo:
        @staticmethod
        def find_binary() -> Path:
            return binary

    import striatum

    monkeypatch.setattr(striatum, "_daemongo", FakeDaemonGo(), raising=False)
    monkeypatch.delenv(ENV_GO_BIN, raising=False)

    assert resolve_go_binary() == binary.resolve()


def test_go_binary_env_override_rejects_stale_schema(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    binary = tmp_path / "striatumd"
    binary.write_text(
        "#!/bin/sh\n"
        f"echo core=go supported_schema={LATEST_DAEMON_DB_VERSION - 1} migration_count={LATEST_DAEMON_DB_VERSION} methods_etag={METHODS_ETAG}\n",
        encoding="utf-8",
    )
    binary.chmod(0o755)
    monkeypatch.setenv(ENV_GO_BIN, str(binary))

    with pytest.raises(StriatumError, match="supports schema"):
        resolve_go_binary()


def test_go_binary_env_override_rejects_stale_method_contract(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    binary = tmp_path / "striatumd"
    binary.write_text(
        "#!/bin/sh\n"
        f"echo core=go supported_schema={LATEST_DAEMON_DB_VERSION} migration_count={LATEST_DAEMON_DB_VERSION} methods_etag=sha256:stale\n",
        encoding="utf-8",
    )
    binary.chmod(0o755)
    monkeypatch.setenv(ENV_GO_BIN, str(binary))

    with pytest.raises(StriatumError, match="method contract"):
        resolve_go_binary()


def test_go_binary_env_override_rejects_stale_migration_count(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    binary = tmp_path / "striatumd"
    binary.write_text(
        "#!/bin/sh\n"
        f"echo core=go supported_schema={LATEST_DAEMON_DB_VERSION} migration_count={LATEST_DAEMON_DB_VERSION - 1} methods_etag={METHODS_ETAG}\n",
        encoding="utf-8",
    )
    binary.chmod(0o755)
    monkeypatch.setenv(ENV_GO_BIN, str(binary))

    with pytest.raises(StriatumError, match="embeds"):
        resolve_go_binary()


def test_parse_go_describe() -> None:
    assert _parse_go_describe("core=go supported_schema=8 migration_count=8 methods_etag=sha256:abc\n") == {
        "core": "go",
        "supported_schema": "8",
        "migration_count": "8",
        "methods_etag": "sha256:abc",
    }

from __future__ import annotations

from pathlib import Path

import pytest

from striatum.cli import daemon as daemon_cli
from striatum.cli.daemon import (
    ENV_GO_BIN,
    _parse_go_describe,
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


def test_daemon_start_core_env_is_ignored(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("STRIATUM_DAEMON_CORE", "python")

    args = build_parser().parse_args(["daemon", "start"])

    assert args.core is None


def test_daemon_start_core_python_is_rejected() -> None:
    with pytest.raises(SystemExit):
        build_parser().parse_args(["daemon", "start", "--core", "python"])


def test_daemon_start_core_unknown_is_rejected() -> None:
    with pytest.raises(SystemExit):
        build_parser().parse_args(["daemon", "start", "--core", "rust"])


def test_go_daemon_launcher_execs_with_migrations_sha_source(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    binary = tmp_path / "striatumd"
    socket = tmp_path / "runtime" / "striatumd.sock"
    captured: dict[str, object] = {}

    monkeypatch.setattr(daemon_cli, "resolve_go_binary", lambda: binary)
    monkeypatch.setattr(daemon_cli, "socket_path", lambda: socket)

    def fake_execv(path: str, argv: list[str]) -> None:
        captured["path"] = path
        captured["argv"] = argv
        raise SystemExit(0)

    os_module = getattr(daemon_cli, "os")
    monkeypatch.setattr(os_module, "execv", fake_execv)

    with pytest.raises(SystemExit):
        daemon_cli.run_go_daemon_foreground(
            postgres_url="postgresql://example/striatum",
            sweep_interval_seconds=12.5,
            max_sweeps=3,
        )

    migrations_sha_source = daemon_cli.resolve_migrations_sha_source()
    assert migrations_sha_source == (
        Path(__file__).resolve().parents[2] / "src" / "striatum" / "daemon_pg" / "sql"
    )
    assert captured["path"] == str(binary)
    assert captured["argv"] == [
        str(binary),
        "--socket",
        str(socket),
        "--postgres-url",
        "postgresql://example/striatum",
        "--sweep-interval-seconds",
        "12.5",
        "--max-sweeps",
        "3",
        "--migrations-sha-source",
        str(migrations_sha_source),
    ]


def test_launch_daemon_start_passes_sweep_options_to_go(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured: dict[str, object] = {}

    def fake_run_go_daemon_foreground(
        *,
        postgres_url: str | None = None,
        sweep_interval_seconds: float = 60.0,
        max_sweeps: int | None = None,
    ) -> dict[str, object]:
        captured["postgres_url"] = postgres_url
        captured["sweep_interval_seconds"] = sweep_interval_seconds
        captured["max_sweeps"] = max_sweeps
        return {"started": True}

    monkeypatch.setattr(
        daemon_cli,
        "run_go_daemon_foreground",
        fake_run_go_daemon_foreground,
    )
    args = build_parser().parse_args(
        [
            "daemon",
            "start",
            "--postgres-url",
            "postgresql://example/striatum",
            "--sweep-interval-seconds",
            "7.5",
            "--max-sweeps",
            "2",
        ]
    )

    assert daemon_cli.launch_daemon_start(args) == {"started": True}
    assert captured == {
        "postgres_url": "postgresql://example/striatum",
        "sweep_interval_seconds": 7.5,
        "max_sweeps": 2,
    }


def test_go_daemon_launcher_rejects_stale_binary_before_exec(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    binary = tmp_path / "striatumd"
    binary.write_text(
        "#!/bin/sh\n"
        "if [ \"$1\" = \"--describe\" ]; then\n"
        f"  echo core=go supported_schema={LATEST_DAEMON_DB_VERSION - 1} "
        f"migration_count={LATEST_DAEMON_DB_VERSION} methods_etag={METHODS_ETAG}\n"
        "  exit 0\n"
        "fi\n"
        "exit 99\n",
        encoding="utf-8",
    )
    binary.chmod(0o755)
    exec_called = False

    def fake_execv(path: str, argv: list[str]) -> None:
        nonlocal exec_called
        exec_called = True
        raise AssertionError(f"stale Go daemon should not exec: {path} {argv}")

    monkeypatch.setattr(daemon_cli, "_resolve_packaged_go_binary", lambda: None)
    os_module = getattr(daemon_cli, "os")
    monkeypatch.setattr(os_module, "execv", fake_execv)
    monkeypatch.setenv(ENV_GO_BIN, str(binary))

    with pytest.raises(StriatumError, match="supports schema"):
        daemon_cli.run_go_daemon_foreground(postgres_url="postgresql://example/striatum")

    assert exec_called is False


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

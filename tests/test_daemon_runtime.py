from __future__ import annotations

import os
from pathlib import Path

import pytest

from striatum.daemon_pg import client_admin
from striatum.daemon_runtime import (
    ENV_RUNTIME,
    DaemonRuntimeTokenError,
    mcp_endpoint_file,
    read_runtime_token,
    socket_path,
    token_file,
    write_runtime_token,
)


def test_runtime_paths_resolve_from_runtime_env(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    runtime = tmp_path / "runtime"
    monkeypatch.setenv(ENV_RUNTIME, str(runtime))

    assert socket_path() == runtime.resolve() / "striatumd.sock"
    assert token_file() == runtime.resolve() / "client-token"
    assert mcp_endpoint_file() == runtime.resolve() / "mcp-http-endpoint"
    assert client_admin.runtime_dir() == runtime.resolve()
    assert client_admin.token_file() == token_file()
    assert client_admin.ENV_RUNTIME == ENV_RUNTIME


def test_runtime_token_round_trips_owner_only_file(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv(ENV_RUNTIME, str(tmp_path / "runtime"))

    write_runtime_token("dtok_test.secret")

    path = token_file()
    assert path.stat().st_mode & 0o777 == 0o600
    assert read_runtime_token() == "dtok_test.secret"


def test_runtime_token_rejects_group_or_world_readable_file(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv(ENV_RUNTIME, str(tmp_path / "runtime"))
    write_runtime_token("dtok_test.secret")
    os.chmod(token_file(), 0o644)

    with pytest.raises(DaemonRuntimeTokenError, match="owner-only"):
        read_runtime_token()
    with pytest.raises(DaemonRuntimeTokenError, match="owner-only"):
        client_admin.read_runtime_token()


def test_runtime_token_ignores_plaintext_env_var(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv(ENV_RUNTIME, str(tmp_path / "runtime"))
    monkeypatch.setenv("STRIATUM_DAEMON_TOKEN", "dtok_forbidden.secret")

    assert read_runtime_token() is None

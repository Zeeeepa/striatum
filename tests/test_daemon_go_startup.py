from __future__ import annotations

import os
import shutil
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]


def test_go_striatumd_refuses_to_serve_without_postgres_config(
    tmp_path: Path,
) -> None:
    go = shutil.which("go")
    if go is None:
        pytest.skip("go toolchain unavailable; cannot run striatumd startup regression")

    home = tmp_path / "home"
    xdg_config = tmp_path / "xdg-config"
    home.mkdir()
    xdg_config.mkdir()
    socket_path = tmp_path / "runtime" / "striatumd.sock"
    env = os.environ.copy()
    env.pop("STRIATUM_DAEMON_DB_URL", None)
    env["HOME"] = str(home)
    env["XDG_CONFIG_HOME"] = str(xdg_config)

    result = subprocess.run(
        [
            go,
            "run",
            "./cmd/striatumd",
            "--socket",
            str(socket_path),
            "--migrate=false",
        ],
        cwd=ROOT / "go",
        env=env,
        text=True,
        capture_output=True,
        check=False,
        timeout=60,
    )

    assert result.returncode != 0
    assert "refuses to start without a Postgres URL" in (
        result.stdout + result.stderr
    )
    assert not socket_path.exists()


def test_go_striatumd_refuses_stale_migrations_before_socket_bind(
    tmp_path: Path,
) -> None:
    go = shutil.which("go")
    if go is None:
        pytest.skip("go toolchain unavailable; cannot run striatumd startup regression")

    home = tmp_path / "home"
    xdg_config = tmp_path / "xdg-config"
    home.mkdir()
    xdg_config.mkdir()
    socket_path = tmp_path / "runtime" / "striatumd.sock"
    migrations_source = tmp_path / "migrations"
    shutil.copytree(ROOT / "src" / "striatum" / "daemon_pg" / "sql", migrations_source)
    migration = migrations_source / "0001_baseline.sql"
    migration.write_text(
        migration.read_text(encoding="utf-8") + "\n-- stale-binary regression\n",
        encoding="utf-8",
    )
    env = os.environ.copy()
    env.pop("STRIATUM_DAEMON_DB_URL", None)
    env["HOME"] = str(home)
    env["XDG_CONFIG_HOME"] = str(xdg_config)

    result = subprocess.run(
        [
            go,
            "run",
            "./cmd/striatumd",
            "--socket",
            str(socket_path),
            "--migrations-sha-source",
            str(migrations_source),
            "--migrate=false",
        ],
        cwd=ROOT / "go",
        env=env,
        text=True,
        capture_output=True,
        check=False,
        timeout=60,
    )

    output = result.stdout + result.stderr
    assert result.returncode != 0
    assert "migrations sha source check failed" in output
    assert "sha mismatch" in output
    assert not socket_path.exists()

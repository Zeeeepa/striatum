# ruff: noqa
from __future__ import annotations

import os
import shutil
import subprocess
import time
import uuid
from pathlib import Path

import pytest

from _harness import pg
from striatum.daemon_pg.connection import connect
from striatum.daemon_rpc.client import call_unix, hello_envelope
from striatum.daemon_rpc.envelope import RpcEnvelope

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
    assert "refuses to start without a Postgres URL" in (result.stdout + result.stderr)
    assert not socket_path.exists()


@pytest.mark.multi_repo
def test_go_striatumd_bootstraps_fresh_admin_runtime_token(tmp_path: Path) -> None:
    go = shutil.which("go")
    if go is None:
        pytest.skip("go toolchain unavailable; cannot run striatumd startup regression")
    binary = tmp_path / "striatumd"
    subprocess.run(
        [go, "build", "-o", str(binary), "./cmd/striatumd"],
        cwd=ROOT / "go",
        text=True,
        capture_output=True,
        check=True,
        timeout=120,
    )

    ephemeral = pg.create_ephemeral_database(pg.resolve_base_url())
    runtime_dir = tmp_path / "runtime"
    socket_path = runtime_dir / "striatumd.sock"
    env = os.environ.copy()
    env["STRIATUM_DAEMON_RUNTIME_DIR"] = str(runtime_dir)
    env["HOME"] = str(tmp_path / "home")
    env["XDG_CONFIG_HOME"] = str(tmp_path / "xdg-config")
    Path(env["HOME"]).mkdir()
    Path(env["XDG_CONFIG_HOME"]).mkdir()

    proc = subprocess.Popen(
        [
            str(binary),
            "--socket",
            str(socket_path),
            "--postgres-url",
            ephemeral.database_url,
            "--migrations-sha-source",
            str(ROOT / "src" / "striatum" / "daemon_pg" / "sql"),
        ],
        cwd=ROOT,
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    try:
        _wait_for_socket(proc, socket_path)
        token_path = runtime_dir / "client-token"
        assert token_path.exists()
        assert token_path.stat().st_mode & 0o777 == 0o600
        assert runtime_dir.stat().st_mode & 0o777 == 0o700
        endpoint_path = runtime_dir / "mcp-http-endpoint"
        assert endpoint_path.exists()
        assert endpoint_path.stat().st_mode & 0o777 == 0o600
        endpoint = endpoint_path.read_text(encoding="utf-8").strip()
        assert endpoint.startswith("http://127.0.0.1:"), endpoint
        assert endpoint.endswith("/mcp/sse"), endpoint
        token = token_path.read_text(encoding="utf-8").strip()
        token_id, sep, secret = token.partition(".")
        assert sep == "."
        assert token_id.startswith("dtok_")
        assert secret

        hello = hello_envelope(
            request_id=f"req_{uuid.uuid4().hex}",
            client_name="daemon-go-startup",
            client_version="0.1",
        )
        assert call_unix(socket_path, hello)["ok"] is True
        response = call_unix(
            socket_path,
            RpcEnvelope(
                schema_version=1,
                request_id=f"req_{uuid.uuid4().hex}",
                method="daemon.describe",
                params={},
                capability_token=token,
            ),
        )
        assert response["ok"] is True, response

        conn = connect(ephemeral.database_url)
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT client_id, client_kind, display_name, token_id
                    FROM striatumd.clients
                    """
                )
                clients = cur.fetchall()
                cur.execute(
                    """
                    SELECT capability
                    FROM striatumd.client_capabilities
                    ORDER BY capability
                    """
                )
                capabilities = [row[0] for row in cur.fetchall()]
        finally:
            conn.close()
        assert len(clients) == 1
        assert clients[0][1:] == ("cli", "bootstrap-admin", token_id)
        assert capabilities == [
            "admin",
            "apply",
            "claim",
            "read",
            "recovery",
            "review",
            "surgical_recovery",
            "write",
        ]
    finally:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=5)
        pg.drop_ephemeral_database(ephemeral)


def _wait_for_socket(proc: subprocess.Popen[str], socket_path: Path) -> None:
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        if proc.poll() is not None:
            stdout, stderr = proc.communicate(timeout=1)
            raise AssertionError(
                f"Go daemon exited during startup\nstdout={stdout}\nstderr={stderr}"
            )
        if socket_path.exists():
            return
        time.sleep(0.05)
    proc.terminate()
    stdout, stderr = proc.communicate(timeout=5)
    raise AssertionError(
        f"Go daemon socket did not appear before startup timeout\nstdout={stdout}\nstderr={stderr}"
    )


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

from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

import pytest

from striatum import daemon
from striatum.db import utc_now
from striatum.errors import (
    EXIT_DAEMON_AUTH,
    EXIT_DAEMON_CAPABILITY,
    EXIT_DAEMON_REGISTRY,
    EXIT_WORKFLOW,
    StriatumError,
)
from striatum.mcp import DaemonRpcServer

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


def _run_cli(repo: Path, *args: str, check: bool = True, env: dict[str, str] | None = None) -> dict[str, Any]:
    merged = os.environ.copy()
    merged["PYTHONPATH"] = str(ROOT / "src")
    if env:
        merged.update(env)
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=merged,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}")
    payload = json.loads(result.stdout) if result.stdout.strip() else {}
    payload["returncode"] = result.returncode
    return cast(dict[str, Any], payload)


def _env(tmp_path: Path) -> dict[str, str]:
    return {
        daemon.ENV_REGISTRY: str(tmp_path / "daemon" / "striatumd.sqlite3"),
        daemon.ENV_RUNTIME: str(tmp_path / "runtime"),
        "XDG_CONFIG_HOME": str(tmp_path / "config"),
        "STRIATUM_DAEMON_DB_URL": "",
    }


def _init_repo(repo: Path, env: dict[str, str]) -> None:
    repo.mkdir(parents=True, exist_ok=True)
    _run_cli(repo, "init", env=env)


def _data(payload: dict[str, Any]) -> dict[str, Any]:
    assert payload["ok"] is True
    return cast(dict[str, Any], payload["data"])


def _prepare_started_run(repo: Path, env: dict[str, str]) -> str:
    prepared = _data(_run_cli(repo, "run", "prepare", "--workflow", str(WORKFLOW), env=env))
    run_id = str(prepared["run_id"])
    _run_cli(repo, "branch", "confirm", "--run-id", run_id, "--branch", "striatum/daemon-test", env=env)
    _run_cli(repo, "run", "start", "--run-id", run_id, env=env)
    return run_id


def test_repo_add_bootstraps_registry_and_dashboard_all_spans_repos(tmp_path: Path) -> None:
    env = _env(tmp_path)
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    _init_repo(repo_a, env)
    _init_repo(repo_b, env)

    first = _data(_run_cli(repo_a, "repo", "add", str(repo_a), env=env))
    second = _data(_run_cli(repo_b, "repo", "add", str(repo_b), env=env))

    assert first["repository_id"] != second["repository_id"]
    assert "bootstrap_admin" in first
    assert first["bootstrap_admin"]["token"].startswith("dtok_")
    listed = _data(_run_cli(repo_a, "repo", "list", env=env))
    assert [row["repo_root"] for row in listed["repositories"]] == [str(repo_a.resolve()), str(repo_b.resolve())]

    dashboard = _data(_run_cli(repo_a, "dashboard", "--all", env=env))
    assert dashboard["mode"] == "daemon"
    assert {row["repository_id"] for row in dashboard["repositories"]} == {
        first["repository_id"],
        second["repository_id"],
    }


def test_forced_daemon_read_requires_token_and_harness_direct_mode_still_works(tmp_path: Path) -> None:
    env = _env(tmp_path)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    _data(_run_cli(repo, "repo", "add", str(repo), env=env))

    direct = _data(_run_cli(repo, "status", env=env))
    assert "mode" not in direct

    token_path = Path(env[daemon.ENV_RUNTIME]) / "client-token"
    token_path.unlink()
    denied = _run_cli(repo, "--daemon", "status", check=False, env=env)
    assert denied["returncode"] == EXIT_DAEMON_AUTH
    assert "token_missing" in denied["error"]["message"]

    conn = sqlite3.connect(env[daemon.ENV_REGISTRY])
    try:
        row = conn.execute(
            "SELECT command, authorization_result, denial_reason FROM audit_log ORDER BY audit_id DESC LIMIT 1"
        ).fetchone()
    finally:
        conn.close()
    assert row == ("status", "denied", "token_missing")


def test_forced_daemon_refuses_unsupported_mutation_verbs(tmp_path: Path) -> None:
    env = _env(tmp_path)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    rejected = _run_cli(
        repo,
        "--daemon",
        "claim-next",
        "--session-id",
        "session_x",
        check=False,
        env=env,
    )
    assert rejected["returncode"] == EXIT_WORKFLOW
    assert "--daemon does not yet route `claim-next`" in rejected["error"]["message"]


def test_daemon_status_reports_pg_migration_privilege_failure_without_traceback(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    class InsufficientPrivilege(Exception):
        sqlstate = "42501"

    from striatum.daemon_pg import connection

    monkeypatch.setattr(daemon, "_pg_connection_configured", lambda: True)
    monkeypatch.setattr(
        connection,
        "connect_and_migrate",
        lambda: (_ for _ in ()).throw(
            InsufficientPrivilege("must be owner of table runs")
        ),
    )

    with pytest.raises(StriatumError) as exc_info:
        daemon.daemon_status()

    exc = exc_info.value
    assert exc.exit_code == EXIT_DAEMON_REGISTRY
    assert "pending migrations require database owner/admin privileges" in str(exc)
    assert "must be owner of table runs" in str(exc)
    assert getattr(exc, "hint") == (
        "run `striatum daemon doctor --apply-migrations --repair-grants` "
        "as a database owner/admin, then retry `striatum daemon status`"
    )


def test_repo_remove_revokes_repo_scoped_capability_and_readd_uses_new_id(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    first = daemon.repo_add(repo)
    first_id = int(first["repository_id"])
    conn = daemon.connect_registry()
    with daemon.registry_transaction(conn):
        token = daemon.create_client(
            conn,
            display_name="repo-reader",
            client_kind="test",
            capabilities=[daemon.READ_CAPABILITY],
            repository_id=first_id,
        )["token"]
    removed = daemon.repo_remove(str(first_id))
    assert removed["state"] == "removed"
    assert removed["revoked_capabilities"] == 1

    with pytest.raises(daemon.DaemonCapabilityError):
        daemon.dashboard_all(token=token)

    second = daemon.repo_add(repo)
    assert int(second["repository_id"]) != first_id


def test_repo_add_refuses_symlink_path(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    link = tmp_path / "repo-link"
    link.symlink_to(repo, target_is_directory=True)

    with pytest.raises(daemon.DaemonCapabilityError):
        daemon.repo_add(link)


def test_repo_add_refuses_symlink_parent_and_traversal(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    real_parent = tmp_path / "real"
    repo = real_parent / "repo"
    _init_repo(repo, env)
    linked_parent = tmp_path / "linked"
    linked_parent.symlink_to(real_parent, target_is_directory=True)

    with pytest.raises(daemon.DaemonCapabilityError):
        daemon.repo_add(linked_parent / "repo")
    with pytest.raises(daemon.DaemonCapabilityError):
        daemon.repo_add(tmp_path / "real" / ".." / "real" / "repo")


def test_repo_add_dedupes_canonical_spellings_by_inode_identity(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    repo = tmp_path / "repo"
    _init_repo(repo, env)

    first = daemon.repo_add(repo)
    second = daemon.repo_add(repo / ".")

    assert second["already_registered"] is True
    assert second["repository_id"] == first["repository_id"]
    assert str(first["repo_identity"]).startswith("inode:")


def test_repo_add_requires_initialized_state_unless_init_flag(tmp_path: Path) -> None:
    env = _env(tmp_path)
    repo = tmp_path / "repo"
    repo.mkdir()

    rejected = _run_cli(repo, "repo", "add", str(repo), check=False, env=env)
    assert rejected["returncode"] == EXIT_DAEMON_CAPABILITY
    assert "not initialized" in rejected["error"]["message"]
    accepted = _data(_run_cli(repo, "repo", "add", str(repo), "--init", env=env))
    assert accepted["state"] == "active"
    assert (repo / ".striatum" / "state.sqlite3").exists()


def test_daemon_mcp_resources_without_pg_conn_fail_closed(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)

    server = DaemonRpcServer()
    tools = server.handle_request({"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
    assert tools is not None
    assert tools["result"]["tools"] == []

    resources = server.handle_request(
        {"jsonrpc": "2.0", "id": 2, "method": "resources/list", "params": {"token": "dtok.secret"}}
    )
    assert resources is not None
    assert resources["error"]["code"] == -32603
    assert "legacy SQLite registry fallback is retired" in resources["error"]["message"]

    mutation = server.handle_request({"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "repo_add"}})
    assert mutation is not None
    assert mutation["error"]["code"] == -32601


def test_daemon_mcp_resource_read_without_pg_conn_fails_closed(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)

    server = DaemonRpcServer()
    read = server.handle_request({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "resources/read",
        "params": {"uri": "striatum://daemon/repos", "token": "dtok.secret"},
    })
    assert read is not None
    assert read["error"]["code"] == -32603
    assert "legacy SQLite registry fallback is retired" in read["error"]["message"]


def test_daemon_sweep_writes_repo_event_with_daemon_byline(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    run_id = _prepare_started_run(repo, env)
    daemon.repo_add(repo)

    result = daemon.daemon_sweep_once()
    assert result["sweeps"][0]["run_id"] == run_id

    conn = sqlite3.connect(repo / ".striatum" / "state.sqlite3")
    try:
        row = conn.execute(
            "SELECT event_type, payload_json FROM events WHERE event_type = 'daemon.recovery_sweep' ORDER BY event_id DESC LIMIT 1"
        ).fetchone()
    finally:
        conn.close()
    assert row is not None
    payload = json.loads(str(row[1]))
    assert row[0] == "daemon.recovery_sweep"
    assert str(payload["author"]).startswith("striatumd-")


def test_daemon_doctor_surfaces_sweep_degraded_and_duplicate_watch(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    run_id = _prepare_started_run(repo, env)
    added = daemon.repo_add(repo)
    repo_id = int(added["repository_id"])
    conn = daemon.connect_registry()
    with daemon.registry_transaction(conn):
        conn.execute(
            """
            INSERT OR REPLACE INTO scheduler_cursors(repository_id, run_id, cursor_kind,
              last_sweep_at, next_sweep_after, last_result_json, state)
            VALUES (?, ?, 'recovery', ?, NULL, '{}', 'sweep_degraded')
            """,
            (repo_id, run_id, utc_now()),
        )
    pidfile = repo / ".striatum" / "scratch" / f"recovery-watch-{run_id}.pid"
    pidfile.parent.mkdir(parents=True, exist_ok=True)
    pidfile.write_text(f"{os.getpid()}\n", encoding="ascii")

    records = daemon.daemon_doctor_records(conn)
    checks = {record["check"] for record in records}
    assert "daemon_sweep_degraded" in checks
    assert "daemon_duplicate_recovery_scheduler" in checks


def test_daemon_sweep_timeout_marks_cursor_degraded(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    run_id = _prepare_started_run(repo, env)
    added = daemon.repo_add(repo)

    daemon.daemon_sweep_once(per_run_timeout_seconds=-1)

    conn = daemon.connect_registry()
    row = conn.execute(
        """
        SELECT state, last_result_json FROM scheduler_cursors
        WHERE repository_id = ? AND run_id = ? AND cursor_kind = 'recovery'
        """,
        (int(added["repository_id"]), run_id),
    ).fetchone()
    assert row is not None
    assert row["state"] == "sweep_degraded"
    assert json.loads(str(row["last_result_json"]))["degraded_reason"] == "sweep_timeout"


def test_daemon_sweep_cli_requires_admin_token_and_audits_denial(tmp_path: Path) -> None:
    env = _env(tmp_path)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    _data(_run_cli(repo, "repo", "add", str(repo), env=env))
    (Path(env[daemon.ENV_RUNTIME]) / "client-token").unlink()

    denied = _run_cli(repo, "daemon", "sweep", check=False, env=env)

    assert denied["returncode"] == EXIT_DAEMON_AUTH
    conn = sqlite3.connect(env[daemon.ENV_REGISTRY])
    try:
        row = conn.execute(
            "SELECT command, authorization_result, denial_reason FROM audit_log ORDER BY audit_id DESC LIMIT 1"
        ).fetchone()
    finally:
        conn.close()
    assert row == ("daemon.sweep", "denied", "token_missing")


def test_audit_segment_closed_rows_are_guarded_and_doctor_checks_manifest(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    daemon.repo_add(repo)
    rotated = daemon.rotate_audit_segment_for_test()
    closed_id = int(rotated["closed_segment_id"])

    conn = daemon.connect_registry()
    with pytest.raises(sqlite3.DatabaseError):
        with daemon.registry_transaction(conn):
            conn.execute(
                "UPDATE audit_segments SET last_hash = 'forged' WHERE segment_id = ?",
                (closed_id,),
            )

    conn.execute("DROP TRIGGER audit_segments_closed_no_update")
    conn.execute(
        "UPDATE audit_segments SET last_hash = 'forged' WHERE segment_id = ?",
        (closed_id,),
    )
    conn.commit()
    checks = {record["check"] for record in daemon.daemon_doctor_records(conn)}
    assert "daemon_audit_segment_manifest" in checks


def test_daemon_restart_preserves_registry_and_instance_id(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    repo = tmp_path / "repo"
    _init_repo(repo, env)
    run_id = _prepare_started_run(repo, env)
    added = daemon.repo_add(repo)
    repo_id = int(added["repository_id"])

    daemon.run_daemon_foreground(max_sweeps=1)
    first_conn = daemon.connect_registry()
    try:
        first_instance = first_conn.execute("SELECT value FROM daemon_meta WHERE key = 'instance_id'").fetchone()[0]
        first_counts = {
            "repos": first_conn.execute("SELECT COUNT(*) FROM repositories").fetchone()[0],
            "clients": first_conn.execute("SELECT COUNT(*) FROM clients").fetchone()[0],
            "caps": first_conn.execute("SELECT COUNT(*) FROM client_capabilities").fetchone()[0],
            "audit": first_conn.execute("SELECT COUNT(*) FROM audit_log").fetchone()[0],
            "segments": first_conn.execute("SELECT COUNT(*) FROM audit_segments").fetchone()[0],
        }
        cursor = first_conn.execute(
            "SELECT state FROM scheduler_cursors WHERE repository_id = ? AND run_id = ?",
            (repo_id, run_id),
        ).fetchone()
    finally:
        first_conn.close()
    assert cursor is not None

    daemon.run_daemon_foreground(max_sweeps=1)
    second_conn = daemon.connect_registry()
    try:
        second_instance = second_conn.execute("SELECT value FROM daemon_meta WHERE key = 'instance_id'").fetchone()[0]
        second_counts = {
            "repos": second_conn.execute("SELECT COUNT(*) FROM repositories").fetchone()[0],
            "clients": second_conn.execute("SELECT COUNT(*) FROM clients").fetchone()[0],
            "caps": second_conn.execute("SELECT COUNT(*) FROM client_capabilities").fetchone()[0],
            "segments": second_conn.execute("SELECT COUNT(*) FROM audit_segments").fetchone()[0],
        }
        second_audit = second_conn.execute("SELECT COUNT(*) FROM audit_log").fetchone()[0]
        cursor_after = second_conn.execute(
            "SELECT state FROM scheduler_cursors WHERE repository_id = ? AND run_id = ?",
            (repo_id, run_id),
        ).fetchone()
    finally:
        second_conn.close()

    assert first_instance == second_instance
    assert first_counts["repos"] == 1
    assert second_counts == {key: value for key, value in first_counts.items() if key != "audit"}
    assert second_audit > first_counts["audit"]
    assert cursor_after is not None

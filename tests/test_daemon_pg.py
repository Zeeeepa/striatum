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
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL, redact_url, resolve_config
from striatum.daemon_pg.connection import doctor as pg_doctor
from striatum.daemon_pg.cutover import CUTOVER_COMPLETED_KEY
from striatum.daemon_pg.migrations import MIGRATIONS
from striatum.daemon_pg.roles import role_repair_sql
from striatum.errors import StriatumError


def _env(tmp_path: Path) -> dict[str, str]:
    env = os.environ.copy()
    env.update(
        {
            daemon.ENV_REGISTRY: str(tmp_path / "daemon" / "striatumd.sqlite3"),
            daemon.ENV_RUNTIME: str(tmp_path / "runtime"),
            "PYTHONPATH": str(Path.cwd() / "src"),
            "XDG_CONFIG_HOME": str(tmp_path / "config"),
        }
    )
    env.pop(ENV_DAEMON_DB_URL, None)
    return env


def _run_cli(repo: Path, *args: str, env: dict[str, str], check: bool = True) -> subprocess.CompletedProcess[str]:
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=Path.cwd(),
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if check and proc.returncode != 0:
        raise AssertionError(proc.stderr or proc.stdout)
    return proc


def _json(proc: subprocess.CompletedProcess[str]) -> dict[str, object]:
    return cast(dict[str, object], json.loads(proc.stdout))


def test_postgres_url_redaction_hides_passwords_and_query_tokens() -> None:
    redacted = redact_url(
        "postgresql://striatum:secret@example.test/striatumd?sslmode=disable&password=also-secret"
    )

    assert "secret" not in redacted
    assert "also-secret" not in redacted
    assert "striatum:<redacted>@" in redacted
    assert "password=%3Credacted%3E" in redacted


def test_resolve_config_precedence_and_missing_url(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))
    monkeypatch.delenv(ENV_DAEMON_DB_URL, raising=False)
    assert resolve_config().url is None

    config_path = tmp_path / "config" / "striatum" / "daemon.toml"
    config_path.parent.mkdir(parents=True)
    config_path.write_text('postgres_url = "postgresql://file/db"\n', encoding="utf-8")
    assert resolve_config().url == "postgresql://file/db"

    monkeypatch.setenv(ENV_DAEMON_DB_URL, "postgresql://env/db")
    assert resolve_config().url == "postgresql://env/db"
    assert resolve_config(postgres_url="postgresql://flag/db").url == "postgresql://flag/db"


def test_daemon_pg_doctor_missing_url_is_structured(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "config"))
    monkeypatch.delenv(ENV_DAEMON_DB_URL, raising=False)

    report = pg_doctor()

    assert report["ok"] is False
    assert report["status"] == "missing_url"
    assert report["configured"] is False
    assert report["redacted_url"] is None
    assert report["onboarding_hints"]


def test_daemon_doctor_cli_reports_postgres_missing_without_leaking(tmp_path: Path) -> None:
    env = _env(tmp_path)
    repo = tmp_path / "repo"
    repo.mkdir()

    proc = _run_cli(repo, "daemon", "doctor", env=env)
    payload = cast(dict[str, Any], _json(proc)["data"])

    assert payload["postgres"]["status"] == "missing_url"
    assert payload["postgres"]["ok"] is False
    assert payload["postgres"]["onboarding_hints"]


def test_daemon_role_repair_sql_keeps_append_only_tables_protected() -> None:
    sql = "\n".join(role_repair_sql(database="striatum_daemon"))

    assert "CREATE ROLE \"striatumd_rw\" WITH LOGIN PASSWORD '<yourpass>'" in sql
    assert "GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd" in sql
    for table in ("audit_log", "events", "artifacts"):
        assert f"REVOKE UPDATE, DELETE ON striatumd.{table} FROM \"striatumd_rw\"" in sql
        assert f"GRANT SELECT, INSERT ON striatumd.{table} TO \"striatumd_rw\"" in sql
    assert 'REVOKE DELETE ON striatumd.repo_event_chain_heads FROM "striatumd_rw"' in sql


def test_baseline_migration_sql_names_expected_v2_tables() -> None:
    sql = MIGRATIONS[0].sql

    for table in (
        "schema_meta",
        "schema_migrations",
        "daemon_meta",
        "repositories",
        "clients",
        "client_capabilities",
        "audit_segments",
        "audit_log",
        "audit_chain_head",
        "scheduler_cursors",
        "rpc_request_log",
        "client_sessions",
    ):
        assert f"striatumd.{table}" in sql
    assert "process_supervisors" not in sql
    assert "cross_repo_runs" not in sql
    assert "mutation_queue" not in sql


def test_cross_repo_migration_sql_names_expected_tables() -> None:
    sql = MIGRATIONS[2].sql

    for table in (
        "cross_repo_runs",
        "cross_repo_run_repositories",
        "cross_repo_cycle_counters",
        "audit_repositories",
    ):
        assert f"striatumd.{table}" in sql
    assert "repository_scope_mode" in sql
    assert "'recovery'" in sql


def test_dogfood_surgical_recovery_migration_extends_capability_constraints() -> None:
    sql = MIGRATIONS[3].sql

    assert "rpc_methods_required_capability_check" in sql
    assert "client_capabilities_capability_check" in sql
    assert "'surgical_recovery'" in sql


def test_sqlite_registry_refuses_after_pg_cutover_marker(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    env = _env(tmp_path)
    monkeypatch.setenv(daemon.ENV_REGISTRY, env[daemon.ENV_REGISTRY])
    monkeypatch.setenv(daemon.ENV_RUNTIME, env[daemon.ENV_RUNTIME])

    conn = daemon.connect_registry()
    with daemon.registry_transaction(conn):
        conn.execute(
            "INSERT INTO daemon_meta(key, value) VALUES(?, ?)",
            (CUTOVER_COMPLETED_KEY, "2026-05-11T00:00:00Z"),
        )
    conn.close()

    with pytest.raises(daemon.DaemonRegistryError, match="cut over to PostgreSQL"):
        daemon.connect_registry()


def test_cutover_refuses_broken_v1_audit_before_postgres_connect(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    env = _env(tmp_path)
    monkeypatch.setenv(daemon.ENV_REGISTRY, env[daemon.ENV_REGISTRY])
    monkeypatch.setenv(daemon.ENV_RUNTIME, env[daemon.ENV_RUNTIME])
    monkeypatch.setenv("XDG_CONFIG_HOME", env["XDG_CONFIG_HOME"])
    monkeypatch.delenv(ENV_DAEMON_DB_URL, raising=False)
    repo = tmp_path / "repo"
    repo.mkdir()
    _run_cli(repo, "init", env=env)
    daemon.repo_add(repo)

    reg = Path(env[daemon.ENV_REGISTRY])
    conn = sqlite3.connect(reg)
    conn.execute("DROP TRIGGER audit_no_update")
    conn.execute("UPDATE audit_log SET command = 'tampered' WHERE audit_id = 1")
    conn.commit()
    conn.close()

    from striatum.daemon_pg.cutover import CutoverOptions, migrate

    with pytest.raises(StriatumError, match="audit chain is not clean"):
        migrate(
            CutoverOptions(
                source_registry=reg,
                postgres_url="postgresql://unused/striatumd",
                dry_run=True,
            )
        )

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

import pytest

from striatum.daemon_pg.client_admin import ENV_REGISTRY
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL, redact_url, resolve_config
from striatum.daemon_pg.connection import doctor as pg_doctor
from striatum.daemon_pg.migrations import MIGRATIONS
from striatum.daemon_pg.roles import role_repair_sql
from striatum.daemon_runtime import ENV_RUNTIME


def _env(tmp_path: Path) -> dict[str, str]:
    env = os.environ.copy()
    env.update(
        {
            ENV_REGISTRY: str(tmp_path / "daemon" / "striatumd.sqlite3"),
            ENV_RUNTIME: str(tmp_path / "runtime"),
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


class _FakePgConn:
    """Minimal psycopg-shaped fake used by the GH #22 owner-connection tests."""

    def __init__(self, *, role: str, server_version_num: int = 160000) -> None:
        self.role = role
        self.server_version_num = server_version_num
        self.closed = False
        self.apply_migrations_called = False
        self.privilege_summary_called = False

    def cursor(self) -> Any:
        return _FakeCursor(self)

    def commit(self) -> None:  # pragma: no cover - exercised indirectly
        return None

    def rollback(self) -> None:  # pragma: no cover - exercised indirectly
        return None

    def close(self) -> None:
        self.closed = True


class _FakeCursor:
    def __init__(self, conn: _FakePgConn) -> None:
        self._conn = conn
        self._row: tuple[Any, ...] | None = None

    def __enter__(self) -> _FakeCursor:
        return self

    def __exit__(self, *_exc: object) -> None:
        return None

    def execute(self, sql: str, *_params: Any) -> None:
        normalized = " ".join(sql.split())
        if normalized.startswith("SHOW server_version_num"):
            self._row = (self._conn.server_version_num,)
        elif normalized.startswith("SELECT current_user, current_database()"):
            self._conn.privilege_summary_called = True
            self._row = (self._conn.role, "striatum_daemon")
        elif "to_regclass" in normalized:
            self._row = (True,)
        elif "has_table_privilege" in normalized:
            # Runtime role: no UPDATE/DELETE. Owner: would have implicit; but
            # owner is never asked because the privilege summary uses the
            # runtime connection only (GH #22).
            self._row = (False,)
        else:
            self._row = (0,)

    def fetchone(self) -> tuple[Any, ...] | None:
        return self._row


def test_doctor_as_owner_applies_migrations_against_owner_connection(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """GH #22 DoD-6: --as-owner routes apply_migrations() through the owner role.

    Simulates the production failure: runtime role gets InsufficientPrivilege
    on ALTER, owner role applies the migration. The privilege summary still
    runs against the runtime connection so the unsafe_privileges guardrail
    keeps its scope.
    """
    runtime_conn = _FakePgConn(role="striatumd_rw")
    owner_conn = _FakePgConn(role="postgres")

    def _fake_connect(url: str) -> Any:
        if url == "postgresql://striatumd_rw@localhost/striatum_daemon":
            return runtime_conn
        if url == "postgresql:///striatum_daemon":
            return owner_conn
        raise AssertionError(f"unexpected connect URL: {url}")

    monkeypatch.setattr("striatum.daemon_pg.connection.connect", _fake_connect)

    def _fake_apply(conn: Any, *, daemon_version: str = "unknown") -> int:
        # InsufficientPrivilege would surface here on the runtime connection.
        # The owner connection is the one we accept.
        if conn is runtime_conn:
            raise AssertionError(
                "apply_migrations must use the owner connection when --as-owner is set"
            )
        assert conn is owner_conn
        conn.apply_migrations_called = True
        return 9

    monkeypatch.setattr("striatum.daemon_pg.connection.apply_migrations", _fake_apply)

    from striatum.daemon_pg.connection import doctor as pg_doctor

    result = pg_doctor(
        postgres_url="postgresql://striatumd_rw@localhost/striatum_daemon",
        apply=True,
        as_owner_url="postgresql:///striatum_daemon",
    )

    assert result["ok"] is True
    assert result["status"] == "ok"
    assert result["schema_version"] == 9
    assert owner_conn.apply_migrations_called is True
    assert owner_conn.closed is True
    assert runtime_conn.privilege_summary_called is True
    # The redacted owner URL must surface so operators can confirm the
    # connection that was used. Peer-auth socket URLs have no password to
    # redact; the value is echoed back verbatim.
    assert result["as_owner_url"] == "postgresql:///striatum_daemon"


def test_doctor_runtime_apply_surfaces_insufficient_privilege_without_as_owner(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """GH #22 DoD-6: without --as-owner, the runtime InsufficientPrivilege still surfaces."""

    class InsufficientPrivilege(Exception):
        sqlstate = "42501"

    runtime_conn = _FakePgConn(role="striatumd_rw")

    monkeypatch.setattr(
        "striatum.daemon_pg.connection.connect",
        lambda _url: runtime_conn,
    )

    def _fake_apply(conn: Any, *, daemon_version: str = "unknown") -> int:
        raise InsufficientPrivilege("must be owner of table runs")

    monkeypatch.setattr("striatum.daemon_pg.connection.apply_migrations", _fake_apply)

    from striatum.daemon_pg.connection import doctor as pg_doctor

    result = pg_doctor(
        postgres_url="postgresql://striatumd_rw@localhost/striatum_daemon",
        apply=True,
    )

    assert result["ok"] is False
    assert result["status"] == "doctor_failed"
    assert "must be owner of table runs" in result["error"]
    assert runtime_conn.closed is True


def test_doctor_as_owner_connection_failure_is_structured(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """GH #22: an unreachable --as-owner URL surfaces as a structured error,
    not a traceback, and does not run apply_migrations."""
    runtime_conn = _FakePgConn(role="striatumd_rw")

    def _fake_connect(url: str) -> Any:
        if url == "postgresql:///striatum_daemon":
            raise RuntimeError("peer authentication failed for user \"postgres\"")
        return runtime_conn

    monkeypatch.setattr("striatum.daemon_pg.connection.connect", _fake_connect)

    def _should_not_apply(*_a: Any, **_k: Any) -> int:
        raise AssertionError("apply_migrations must not run when owner connect fails")

    monkeypatch.setattr("striatum.daemon_pg.connection.apply_migrations", _should_not_apply)

    from striatum.daemon_pg.connection import doctor as pg_doctor

    result = pg_doctor(
        postgres_url="postgresql://striatumd_rw@localhost/striatum_daemon",
        apply=True,
        as_owner_url="postgresql:///striatum_daemon",
    )

    assert result["ok"] is False
    assert result["status"] == "as_owner_unreachable"
    assert result["error_type"] == "RuntimeError"
    assert "peer authentication failed" in result["error"]
    assert runtime_conn.closed is True

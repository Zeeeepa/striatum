from __future__ import annotations

import argparse
from pathlib import Path

import pytest

from _harness.multi_repo import MultiRepoHarness
from striatum.daemon_pg import client_admin as daemon
from striatum.cli.dispatch import _dispatch_daemon
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL
from striatum.errors import EXIT_DAEMON_REGISTRY, StriatumError

pytestmark = pytest.mark.multi_repo


def _configure_pg_daemon_lifecycle(
    harness: MultiRepoHarness,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
    *,
    capabilities: list[str],
) -> Path:
    token = harness.issue_token(capabilities)
    runtime = tmp_path / "runtime"
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(ENV_DAEMON_DB_URL, harness.daemon_pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(runtime))
    monkeypatch.setenv(daemon.ENV_REGISTRY, str(registry))
    monkeypatch.setenv(daemon.ENV_SQLITE_CONNECT_TRIPWIRE, "1")
    monkeypatch.delenv(daemon.ENV_ALLOW_LEGACY_SQLITE_REGISTRY, raising=False)
    daemon.write_runtime_token(token)
    return registry


def test_daemon_status_uses_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_lifecycle(harness, monkeypatch, tmp_path, capabilities=["read"])

    result = daemon.daemon_status()

    assert result["mode"] == "daemon"
    assert result["substrate"] == "postgres"
    assert result["running"] is False
    assert result["pid"] is None
    assert result["instance_id"]
    assert not registry.exists()
    row = harness.audit_rows(transport="cli")[-1]
    assert row["method"] == "daemon.status"
    assert row["decision"] == "allowed"


def test_dispatch_daemon_status_uses_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_lifecycle(harness, monkeypatch, tmp_path, capabilities=["read"])
    args = argparse.Namespace(daemon_command="status")

    result = _dispatch_daemon(args)

    assert isinstance(result, dict)
    assert result["substrate"] == "postgres"
    assert not registry.exists()
    assert harness.audit_rows(transport="cli")[-1]["method"] == "daemon.status"


def test_daemon_stop_authorizes_with_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_lifecycle(harness, monkeypatch, tmp_path, capabilities=["admin"])

    result = daemon.daemon_stop()

    assert result == {"stopped": False, "reason": "not_running"}
    assert not registry.exists()
    row = harness.audit_rows(transport="cli")[-1]
    assert row["method"] == "daemon.stop"
    assert row["decision"] == "allowed"


def test_dispatch_daemon_stop_authorizes_with_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_lifecycle(harness, monkeypatch, tmp_path, capabilities=["admin"])
    args = argparse.Namespace(daemon_command="stop")

    result = _dispatch_daemon(args)

    assert result == {"stopped": False, "reason": "not_running"}
    assert not registry.exists()
    assert harness.audit_rows(transport="cli")[-1]["method"] == "daemon.stop"


def test_daemon_status_reports_pg_migration_privilege_failure_without_traceback(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    class InsufficientPrivilege(Exception):
        sqlstate = "42501"

    from striatum.daemon_pg import connection

    monkeypatch.setenv(ENV_DAEMON_DB_URL, "postgresql://striatum.invalid/daemon")
    monkeypatch.setattr(
        connection,
        "connect_and_migrate",
        lambda *_args, **_kwargs: (_ for _ in ()).throw(
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

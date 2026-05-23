from __future__ import annotations

import argparse
from pathlib import Path
from typing import Any

import pytest

from _harness.multi_repo import MultiRepoHarness
from striatum.daemon_pg import client_admin as daemon
from striatum.cli.dispatch import _dispatch_daemon
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL
from striatum.errors import EXIT_DAEMON_REGISTRY, StriatumError

pytestmark = pytest.mark.multi_repo

RETIRED_DAEMON_REGISTRY_ENV = "STRIATUM_DAEMON_REGISTRY"


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
    monkeypatch.setenv(RETIRED_DAEMON_REGISTRY_ENV, str(registry))
    monkeypatch.setenv(daemon.ENV_SQLITE_CONNECT_TRIPWIRE, "1")
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


def test_daemon_stop_does_not_call_connect_and_migrate(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """GH #22 DoD-7: stop must not require a working migration path.

    Even when connect_and_migrate would raise InsufficientPrivilege, stop
    must succeed against the pidfile and SIGTERM the daemon. The PG audit
    append is best-effort and never blocks stop on pending migrations.
    """
    from striatum.daemon_pg import client_admin, connection

    monkeypatch.setattr(
        client_admin,
        "_read_pid",
        lambda: None,
    )

    def _explode(*_a: Any, **_k: Any) -> Any:
        raise AssertionError("daemon_stop must not call connect_and_migrate (GH #22)")

    monkeypatch.setattr(connection, "connect_and_migrate", _explode)
    monkeypatch.setattr(client_admin, "_pg_connection_configured", lambda: False)

    result = client_admin.daemon_stop()

    assert result == {"stopped": False, "reason": "not_running"}


def test_daemon_stop_running_pid_signals_before_pg_audit(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """GH #22 DoD-3/7: stop SIGTERMs the daemon first, even if PG audit fails."""
    import os as _os

    from striatum.daemon_pg import client_admin

    sent: dict[str, int] = {}

    def _fake_kill(pid: int, sig: int) -> None:
        sent["pid"] = pid
        sent["sig"] = sig

    monkeypatch.setattr(client_admin, "_read_pid", lambda: 4242)
    monkeypatch.setattr(client_admin, "_pid_alive", lambda _pid: True)
    monkeypatch.setattr(_os, "kill", _fake_kill)
    audit_called: list[dict[str, Any]] = []
    monkeypatch.setattr(
        client_admin,
        "_best_effort_audit_stop",
        lambda result: audit_called.append(dict(result)),
    )

    result = client_admin.daemon_stop()

    assert sent == {"pid": 4242, "sig": __import__("signal").SIGTERM}
    assert result == {"stopped": True, "pid": 4242}
    assert audit_called == [{"stopped": True, "pid": 4242}]


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
    hint = getattr(exc, "hint")
    # GH #22: hint must reference the supported --as-owner CLI shape and not
    # the test-harness env var workaround.
    assert "--as-owner" in hint
    assert "--apply-migrations" in hint
    assert "STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK" not in hint
    assert "postgresql:///striatum_daemon" in hint

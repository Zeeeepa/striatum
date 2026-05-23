from __future__ import annotations

import argparse
from pathlib import Path

import pytest

from _harness.multi_repo import MultiRepoHarness
from striatum.daemon_pg import client_admin as daemon
from striatum.cli.dispatch import _dispatch_daemon
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL

pytestmark = pytest.mark.multi_repo

RETIRED_DAEMON_REGISTRY_ENV = "STRIATUM_DAEMON_REGISTRY"


def _configure_pg_daemon_health(
    harness: MultiRepoHarness,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> Path:
    runtime = tmp_path / "runtime"
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(ENV_DAEMON_DB_URL, harness.daemon_pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(runtime))
    monkeypatch.setenv(RETIRED_DAEMON_REGISTRY_ENV, str(registry))
    monkeypatch.setenv(daemon.ENV_SQLITE_CONNECT_TRIPWIRE, "1")
    return registry


def test_daemon_health_uses_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_health(harness, monkeypatch, tmp_path)

    result = daemon.health()

    assert result == {"mode": "daemon", "ok": True, "protocol_version": daemon.PROTOCOL_VERSION}
    assert not registry.exists()
    row = harness.audit_rows(transport="cli")[-1]
    assert row["method"] == "health"
    assert row["decision"] == "allowed"
    assert row["client_id"] is None
    harness.assert_audit_chain()


def test_dispatch_daemon_health_uses_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_health(harness, monkeypatch, tmp_path)
    args = argparse.Namespace(daemon_command="health")

    result = _dispatch_daemon(args)

    assert result == {"mode": "daemon", "ok": True, "protocol_version": daemon.PROTOCOL_VERSION}
    assert not registry.exists()
    assert harness.audit_rows(transport="cli")[-1]["method"] == "health"

from __future__ import annotations

import argparse
from pathlib import Path
from typing import Any

import pytest

from _harness.multi_repo import MultiRepoHarness
from striatum.daemon_pg import client_admin as daemon
from striatum.cli.dispatch import _dispatch_daemon
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL

pytestmark = pytest.mark.multi_repo


def _configure_pg_daemon_audit(
    harness: MultiRepoHarness,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> Path:
    token = harness.issue_token(["admin"])
    runtime = tmp_path / "runtime"
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(ENV_DAEMON_DB_URL, harness.daemon_pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(runtime))
    monkeypatch.setenv(daemon.ENV_REGISTRY, str(registry))
    monkeypatch.setenv(daemon.ENV_SQLITE_CONNECT_TRIPWIRE, "1")
    monkeypatch.delenv(daemon.ENV_ALLOW_LEGACY_SQLITE_REGISTRY, raising=False)
    daemon.write_runtime_token(token)
    return registry


def test_daemon_audit_reads_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_audit(harness, monkeypatch, tmp_path)

    result = daemon.daemon_audit(limit=5)

    assert result["mode"] == "daemon"
    row = result["audit"][0]
    assert row["command"] == "daemon.audit"
    assert row["authorization_result"] == "allowed"
    assert row["transport"] == "cli"
    assert row["schema_version"] == 1
    assert row["hash_format_version"] == 2
    assert not registry.exists()
    harness.assert_audit_chain()


def test_dispatch_daemon_audit_uses_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_audit(harness, monkeypatch, tmp_path)
    args = argparse.Namespace(daemon_command="audit", limit=1)

    result = _dispatch_daemon(args)

    assert isinstance(result, dict)
    audit = result["audit"]
    assert isinstance(audit, list)
    assert len(audit) == 1
    row = audit[0]
    assert isinstance(row, dict)
    assert row["command"] == "daemon.audit"
    assert row["authorization_result"] == "allowed"
    assert not registry.exists()


def test_daemon_audit_read_token_is_denied_and_audited_in_postgres(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    token = harness.issue_token(["read"])
    runtime = tmp_path / "runtime"
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(ENV_DAEMON_DB_URL, harness.daemon_pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(runtime))
    monkeypatch.setenv(daemon.ENV_REGISTRY, str(registry))
    monkeypatch.setenv(daemon.ENV_SQLITE_CONNECT_TRIPWIRE, "1")
    daemon.write_runtime_token(token)

    with pytest.raises(daemon.DaemonCapabilityError, match="capability_missing"):
        daemon.daemon_audit(limit=5)

    audit_rows: list[dict[str, Any]] = harness.audit_rows(transport="cli")
    row = audit_rows[-1]
    assert row["method"] == "daemon.audit"
    assert row["decision"] == "denied"
    assert row["denial_reason"] == "capability_missing"
    assert not registry.exists()

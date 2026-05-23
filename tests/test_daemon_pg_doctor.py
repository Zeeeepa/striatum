from __future__ import annotations

import shutil
from pathlib import Path

import pytest

from _harness.multi_repo import MultiRepoHarness
from striatum.daemon_pg import client_admin as daemon
from striatum.cli.dispatch import _dispatch_daemon
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL

pytestmark = pytest.mark.multi_repo

RETIRED_DAEMON_REGISTRY_ENV = "STRIATUM_DAEMON_REGISTRY"


def _configure_pg_daemon_doctor(
    harness: MultiRepoHarness,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
    *,
    repo_id: str | None = None,
) -> Path:
    token = harness.issue_token(["read"], repo_id=repo_id)
    runtime = tmp_path / "runtime"
    registry = tmp_path / "daemon" / "striatumd.sqlite3"
    monkeypatch.setenv(ENV_DAEMON_DB_URL, harness.daemon_pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(runtime))
    monkeypatch.setenv(RETIRED_DAEMON_REGISTRY_ENV, str(registry))
    monkeypatch.setenv(daemon.ENV_SQLITE_CONNECT_TRIPWIRE, "1")
    monkeypatch.setenv("STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK", "1")
    daemon.write_runtime_token(token)
    return registry


def test_daemon_doctor_reads_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_doctor(harness, monkeypatch, tmp_path)

    result = daemon.read_doctor(repo=None, verbose=True)

    assert result["mode"] == "daemon"
    assert result["protocol_version"] == daemon.PROTOCOL_VERSION
    assert result["problems"] == []
    assert result["problem_records"] == []
    assert not registry.exists()
    row = harness.audit_rows(transport="cli")[-1]
    assert row["method"] == "doctor"
    assert row["decision"] == "allowed"
    assert row["repository_id"] is None
    harness.assert_audit_chain()


def test_repo_scoped_daemon_doctor_reads_postgres_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    repo_id = str(harness.repos[0].repository_id)
    registry = _configure_pg_daemon_doctor(harness, monkeypatch, tmp_path, repo_id=repo_id)

    result = daemon.read_doctor(repo=harness.repos[0].path, verbose=True)

    assert result["mode"] == "daemon"
    assert result["repository_id"] == repo_id
    assert result["problems"] == []
    assert result["daemon_problems"] == []
    assert not registry.exists()
    row = harness.audit_rows(transport="cli")[-1]
    assert row["method"] == "doctor"
    assert row["repository_id"] == repo_id


def test_daemon_doctor_treats_legacy_state_db_path_as_scratch_directory(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    repo = harness.repos[0]
    repo_id = str(repo.repository_id)
    harness.daemon_db_query(
        """
        UPDATE striatumd.repositories
        SET state_db_path = %s
        WHERE repository_id = %s
        """,
        (str(repo.path / ".striatum" / "state.sqlite3"), repo_id),
    )
    _configure_pg_daemon_doctor(harness, monkeypatch, tmp_path, repo_id=repo_id)

    result = daemon.read_doctor(repo=repo.path, verbose=True)

    assert result["daemon_problems"] == []
    assert result["daemon_problem_records"] == []


def test_daemon_doctor_reports_missing_operational_scratch_directory(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    harness.register_all()
    repo = harness.repos[0]
    repo_id = str(repo.repository_id)
    shutil.rmtree(repo.path / ".striatum")
    _configure_pg_daemon_doctor(harness, monkeypatch, tmp_path, repo_id=repo_id)

    result = daemon.read_doctor(repo=repo.path, verbose=True)

    assert result["daemon_problems"] == [
        "registered repository `.striatum/` operational scratch is missing"
    ]
    assert result["daemon_problem_records"] == [
        {
            "check": "daemon_repo_scratch_missing",
            "id": repo_id,
            "message": "registered repository `.striatum/` operational scratch is missing",
            "context": {
                "repo_root": str(repo.path),
                "state_dir": str(repo.path / ".striatum"),
            },
        }
    ]


def test_dispatch_daemon_doctor_reports_pg_diagnostics_without_sqlite_registry(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    registry = _configure_pg_daemon_doctor(harness, monkeypatch, tmp_path)
    args = type(
        "Args",
        (),
        {
            "daemon_command": "doctor",
            "postgres_url": None,
            "apply_migrations": False,
            "provision_rw_role": False,
            "repair_grants": False,
            "explain": False,
            "authority": False,
        },
    )()

    result = _dispatch_daemon(args)

    assert isinstance(result, dict)
    assert result["postgres"]["ok"] is True
    assert result["sqlite_registry"]["status"] == "post_pg_cutover_unused"
    assert result["daemon_diagnostics"]["mode"] == "daemon"
    assert result["daemon_diagnostics"]["problems"] == []
    assert not registry.exists()

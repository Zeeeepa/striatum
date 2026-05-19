"""``daemon doctor`` registry reporting after the PostgreSQL cutover."""

from __future__ import annotations

import argparse
from pathlib import Path
from typing import Any, cast

import pytest

from striatum.daemon_pg import client_admin as daemon_mod
from striatum.cli.dispatch import _dispatch_daemon


def _doctor_args() -> argparse.Namespace:
    return argparse.Namespace(
        daemon_command="doctor",
        postgres_url=None,
        doctor_repo=None,
        apply_migrations=False,
        provision_rw_role=False,
        repair_grants=False,
        explain=False,
        authority=False,
        as_owner_url=None,
    )


def test_sqlite_registry_post_pg_cutover_surfaces_as_disabled_and_records_diagnostics_error(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 5, "status": "ok"},
    )

    def _raise_token_invalid(**_: Any) -> dict[str, Any]:
        raise daemon_mod.DaemonAuthError("daemon authorization failed: token_invalid")

    monkeypatch.setattr(daemon_mod, "read_doctor", _raise_token_invalid)

    result = _dispatch_daemon(_doctor_args())

    assert isinstance(result, dict)
    sqlite_registry = result["sqlite_registry"]
    assert isinstance(sqlite_registry, dict)
    assert sqlite_registry["ok"] is True
    assert sqlite_registry["status"] == "post_pg_cutover_unused"
    assert "PostgreSQL is the authoritative" in sqlite_registry["note"]
    assert result["daemon_diagnostics"] == {
        "ok": False,
        "error": "daemon authorization failed: token_invalid",
    }


def test_sqlite_registry_disabled_with_pg_ok_surfaces_as_benign(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 6, "status": "ok"},
    )

    def _raise_registry_disabled(**_: Any) -> dict[str, Any]:
        raise daemon_mod.DaemonRegistryError(
            "legacy SQLite daemon registry is disabled in production; "
            "use the configured PostgreSQL daemon registry"
        )

    monkeypatch.setattr(daemon_mod, "read_doctor", _raise_registry_disabled)

    result = _dispatch_daemon(_doctor_args())

    assert isinstance(result, dict)
    sqlite_registry = result["sqlite_registry"]
    assert isinstance(sqlite_registry, dict)
    assert sqlite_registry["ok"] is True
    assert sqlite_registry["status"] == "post_pg_cutover_unused"
    assert "PostgreSQL is the authoritative" in sqlite_registry["note"]
    assert result["daemon_diagnostics"] == {
        "ok": False,
        "error": (
            "legacy SQLite daemon registry is disabled in production; "
            "use the configured PostgreSQL daemon registry"
        ),
    }


def test_sqlite_registry_token_invalid_with_pg_down_reports_error(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": False, "status": "unconfigured"},
    )

    def _raise_token_invalid(**_: Any) -> dict[str, Any]:
        raise daemon_mod.DaemonAuthError("daemon authorization failed: token_invalid")

    monkeypatch.setattr(daemon_mod, "read_doctor", _raise_token_invalid)

    result = _dispatch_daemon(_doctor_args())

    assert isinstance(result, dict)
    sqlite_registry = result["sqlite_registry"]
    assert isinstance(sqlite_registry, dict)
    assert sqlite_registry["ok"] is False
    assert "token_invalid" in sqlite_registry["error"]


def test_sqlite_registry_other_auth_failure_reports_error(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 5, "status": "ok"},
    )

    def _raise_other(**_: Any) -> dict[str, Any]:
        raise daemon_mod.DaemonAuthError("daemon authorization failed: token_revoked")

    monkeypatch.setattr(daemon_mod, "read_doctor", _raise_other)

    result = _dispatch_daemon(_doctor_args())

    assert isinstance(result, dict)
    sqlite_registry = result["sqlite_registry"]
    assert isinstance(sqlite_registry, dict)
    assert sqlite_registry["ok"] is True
    assert sqlite_registry["status"] == "post_pg_cutover_unused"
    assert result["daemon_diagnostics"] == {
        "ok": False,
        "error": "daemon authorization failed: token_revoked",
    }


def test_sqlite_registry_success_path_unchanged(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 5, "status": "ok"},
    )
    monkeypatch.setattr(
        daemon_mod,
        "read_doctor",
        lambda **_: {"mode": "daemon", "problems": [], "protocol_version": 1},
    )

    result = _dispatch_daemon(_doctor_args())

    assert isinstance(result, dict)
    sqlite_registry = result["sqlite_registry"]
    assert isinstance(sqlite_registry, dict)
    assert sqlite_registry["status"] == "post_pg_cutover_unused"
    assert result["daemon_diagnostics"] == {"mode": "daemon", "problems": [], "protocol_version": 1}


def test_daemon_doctor_authority_report_names_cutover_state(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 6, "status": "ok"},
    )
    monkeypatch.setattr(
        daemon_mod,
        "read_doctor",
        lambda **_: {
            "ok": True,
            "status": "post_pg_cutover_unused",
            "note": "SQLite client registry is no longer the authoritative auth surface.",
        },
    )
    args = _doctor_args()
    args.authority = True

    result = _dispatch_daemon(args)

    assert isinstance(result, dict)
    payload = cast(dict[str, Any], result)
    authority = payload["authority"]
    assert isinstance(authority, dict)
    assert authority["schema_version"] == "striatum.authority_report.v1"
    assert authority["ok"] is True
    assert authority["live_state_authority"] == "daemon_postgresql"
    assert authority["legacy_sqlite"]["registry_status"] == "disabled"
    assert authority["daemon_methods"]["cli_fallback_route_count"] == 0
    assert authority["recommendations"] == []
    assert "explain" not in result


def test_daemon_doctor_repo_option_embeds_cutover_report(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 6, "status": "ok"},
    )
    monkeypatch.setattr(
        daemon_mod,
        "read_doctor",
        lambda **_: {
            "ok": True,
            "status": "post_pg_cutover_unused",
            "note": "SQLite client registry is no longer the authoritative auth surface.",
        },
    )

    def fake_verify(options: Any) -> dict[str, Any]:
        assert options.repo == repo.resolve()
        assert options.postgres_url == "postgresql://localhost/striatum_test"
        return {
            "schema_version": "striatum.repo_cutover_report.v1",
            "ok": True,
            "mode": "repo_cutover_verification",
            "verify_only": True,
            "repo": str(repo.resolve()),
            "recommendations": [],
        }

    monkeypatch.setattr(
        "striatum.daemon_pg.repo_cutover_report.verify_repo_cutover",
        fake_verify,
    )
    args = _doctor_args()
    args.authority = True
    args.postgres_url = "postgresql://localhost/striatum_test"
    args.doctor_repo = str(repo)

    result = _dispatch_daemon(args)

    assert isinstance(result, dict)
    repo_cutover = result["repo_cutover"]
    assert isinstance(repo_cutover, dict)
    assert repo_cutover["schema_version"] == "striatum.repo_cutover_report.v1"
    authority = result["authority"]
    assert isinstance(authority, dict)
    assert authority["ok"] is True
    assert authority["repository_cutover"] == {
        "schema_version": "striatum.repo_cutover_report.v1",
        "ok": True,
        "repo": str(repo.resolve()),
        "mode": "repo_cutover_verification",
    }


def test_daemon_doctor_repo_option_does_not_import_retired_sqlite_migrator(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    import sys

    repo = tmp_path / "repo"
    repo.mkdir()
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 6, "status": "ok"},
    )
    monkeypatch.setattr(
        daemon_mod,
        "read_doctor",
        lambda **_: {
            "ok": True,
            "status": "post_pg_cutover_unused",
            "note": "SQLite client registry is no longer the authoritative auth surface.",
        },
    )
    monkeypatch.setattr(
        "striatum.daemon_pg.repo_cutover_report.verify_repo_cutover",
        lambda _options: {
            "schema_version": "striatum.repo_cutover_report.v1",
            "ok": True,
            "mode": "repo_cutover_verification",
            "repo": str(repo.resolve()),
            "recommendations": [],
        },
    )
    sys.modules.pop("sqlite3", None)
    sys.modules.pop("striatum.daemon_pg.repo_local_migration", None)
    args = _doctor_args()
    args.authority = True
    args.postgres_url = "postgresql://localhost/striatum_test"
    args.doctor_repo = str(repo)

    _dispatch_daemon(args)

    assert "sqlite3" not in sys.modules
    assert "striatum.daemon_pg.repo_local_migration" not in sys.modules


def test_daemon_doctor_authority_report_flags_repo_cutover_failure(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 6, "status": "ok"},
    )
    monkeypatch.setattr(
        daemon_mod,
        "read_doctor",
        lambda **_: {
            "ok": True,
            "status": "post_pg_cutover_unused",
            "note": "SQLite client registry is no longer the authoritative auth surface.",
        },
    )
    monkeypatch.setattr(
        "striatum.daemon_pg.repo_cutover_report.verify_repo_cutover",
        lambda _options: {
            "schema_version": "striatum.repo_cutover_report.v1",
            "ok": False,
            "mode": "repo_cutover_verification",
            "repo": str(repo.resolve()),
            "recommendations": ["run migration"],
        },
    )
    args = _doctor_args()
    args.authority = True
    args.postgres_url = "postgresql://localhost/striatum_test"
    args.doctor_repo = str(repo)

    result = _dispatch_daemon(args)

    assert isinstance(result, dict)
    authority = result["authority"]
    assert isinstance(authority, dict)
    assert authority["ok"] is False
    assert authority["repository_cutover"]["ok"] is False
    assert "resolve repository cutover verification failures" in authority["recommendations"]
    assert "run migration" in authority["recommendations"]


def test_role_repair_flags_are_passed_to_pg_doctor(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    seen: dict[str, Any] = {}

    def _pg_doctor(**kwargs: Any) -> dict[str, Any]:
        seen.update(kwargs)
        return {"ok": True, "schema_version": 5, "status": "ok"}

    monkeypatch.setattr("striatum.daemon_pg.connection.doctor", _pg_doctor)
    monkeypatch.setattr(
        daemon_mod,
        "read_doctor",
        lambda **_: {"mode": "daemon", "problems": [], "protocol_version": 1},
    )
    args = _doctor_args()
    args.provision_rw_role = True
    args.repair_grants = True

    _dispatch_daemon(args)

    assert seen["provision_rw_role"] is True
    assert seen["repair_grants"] is True


def test_as_owner_url_is_passed_to_pg_doctor(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """GH #22: --as-owner reaches striatum.daemon_pg.connection.doctor."""
    seen: dict[str, Any] = {}

    def _pg_doctor(**kwargs: Any) -> dict[str, Any]:
        seen.update(kwargs)
        return {"ok": True, "schema_version": 9, "status": "ok"}

    monkeypatch.setattr("striatum.daemon_pg.connection.doctor", _pg_doctor)
    monkeypatch.setattr(
        daemon_mod,
        "read_doctor",
        lambda **_: {"mode": "daemon", "problems": [], "protocol_version": 1},
    )
    args = _doctor_args()
    args.apply_migrations = True
    args.as_owner_url = "postgresql:///striatum_daemon"

    _dispatch_daemon(args)

    assert seen["apply"] is True
    assert seen["as_owner_url"] == "postgresql:///striatum_daemon"


def test_explicit_postgres_url_is_passed_to_daemon_diagnostics(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    seen: dict[str, Any] = {}

    monkeypatch.setattr(
        "striatum.daemon_pg.connection.doctor",
        lambda **_: {"ok": True, "schema_version": 6, "status": "ok"},
    )

    def _read_doctor(**kwargs: Any) -> dict[str, Any]:
        seen.update(kwargs)
        return {"mode": "daemon", "problems": [], "protocol_version": 1}

    monkeypatch.setattr(daemon_mod, "read_doctor", _read_doctor)
    args = _doctor_args()
    args.postgres_url = "postgres://striatumd_rw:secret@example.invalid/striatum"

    result = _dispatch_daemon(args)

    assert isinstance(result, dict)
    assert seen["repo"] is None
    assert seen["verbose"] is True
    assert seen["postgres_url"] == args.postgres_url

"""``daemon doctor`` SQLite-registry probe behavior after RFC 0048 Phase C.

The runtime token is minted into ``striatumd.clients`` by
``_bootstrap_pg_admin_if_needed`` and the SQLite registry's ``clients``
table is no longer populated at daemon startup. When PG is authoritative,
``read_doctor`` raises ``DaemonAuthError(token_invalid)``; the dispatcher
should surface that as a benign ``post_pg_cutover_unused`` signal rather
than an error payload.
"""

from __future__ import annotations

import argparse
from typing import Any

import pytest

from striatum import daemon as daemon_mod
from striatum.cli.dispatch import _dispatch_daemon


def _doctor_args() -> argparse.Namespace:
    return argparse.Namespace(
        daemon_command="doctor",
        postgres_url=None,
        apply_migrations=False,
        provision_rw_role=False,
        repair_grants=False,
        explain=False,
    )


def test_sqlite_registry_post_pg_cutover_surfaces_as_benign(
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
    assert "no longer the authoritative" in sqlite_registry["note"]


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
    assert sqlite_registry["ok"] is False
    assert "token_revoked" in sqlite_registry["error"]


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
    assert sqlite_registry.get("problems") == []


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

from __future__ import annotations

import argparse
from pathlib import Path

import pytest

from striatum.cli import daemon as daemon_cli
from striatum.cli.daemon import dispatch_daemon
from striatum.cli.parser import build_parser
from striatum.daemon_pg.repo_local_migration import RepoLocalMigrationOptions


def test_parser_accepts_migrate_repo_local_verify_cutover() -> None:
    args = build_parser().parse_args(
        [
            "daemon",
            "migrate-repo-local",
            "--from",
            "sqlite",
            "--to",
            "pg",
            "--repo",
            "target",
            "--verify-cutover",
            "--json",
        ]
    )

    assert args.verify_cutover is True
    assert args.json is True


def test_parser_accepts_daemon_doctor_repo_cutover() -> None:
    args = build_parser().parse_args(
        [
            "daemon",
            "doctor",
            "--repo",
            "target",
            "--authority",
            "--json",
        ]
    )

    assert args.doctor_repo == "target"
    assert args.authority is True
    assert args.json is True


def test_dispatch_routes_verify_cutover_without_migrating(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    calls: list[RepoLocalMigrationOptions] = []

    def fake_verify(options: RepoLocalMigrationOptions) -> dict[str, object]:
        calls.append(options)
        return {"schema_version": "striatum.repo_cutover_report.v1"}

    def fail_migrate(_options: RepoLocalMigrationOptions) -> dict[str, object]:
        raise AssertionError("verify-cutover must not run migration")

    monkeypatch.setattr(daemon_cli, "verify_repo_cutover", fake_verify)
    monkeypatch.setattr(daemon_cli, "migrate_repo_local", fail_migrate)
    args = argparse.Namespace(
        daemon_command="migrate-repo-local",
        from_substrate="sqlite",
        to_substrate="pg",
        repo_local_repo=str(tmp_path),
        repo=None,
        postgres_url="postgresql://localhost/striatum_test",
        dry_run=False,
        verify_cutover=True,
        keep_sqlite_readonly=True,
        confirm_delete=False,
    )

    result = dispatch_daemon(args)

    assert result == {"schema_version": "striatum.repo_cutover_report.v1"}
    assert calls == [
        RepoLocalMigrationOptions(
            repo=tmp_path,
            postgres_url="postgresql://localhost/striatum_test",
        )
    ]

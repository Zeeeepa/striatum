from __future__ import annotations

import argparse
import sys
from pathlib import Path

import pytest

from striatum.cli.daemon import dispatch_daemon
from striatum.cli.dispatch import _dispatch_daemon
from striatum.cli.parser import build_parser
from striatum.errors import StriatumError


def test_parser_accepts_retired_migrate_repo_local_compatibility_shape() -> None:
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

    assert args.daemon_command == "migrate-repo-local"
    assert args.verify_cutover is True
    assert args.json is True


def test_retired_migrate_repo_local_refuses_before_importing_sqlite_migration(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delitem(sys.modules, "striatum.daemon_pg.repo_local_migration", raising=False)
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

    with pytest.raises(StriatumError) as exc:
        dispatch_daemon(args)

    assert exc.value.exit_code == 12
    assert "SQLite import windows are closed" in str(exc.value)
    assert "striatum.daemon_pg.repo_local_migration" not in sys.modules


def test_retired_daemon_migrate_refuses_before_importing_sqlite_cutover(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delitem(sys.modules, "striatum.daemon", raising=False)
    monkeypatch.delitem(sys.modules, "striatum.daemon_pg.cutover", raising=False)
    args = argparse.Namespace(
        daemon_command="migrate",
        from_substrate="sqlite",
        to_substrate="pg",
        postgres_url="postgresql://localhost/striatum_test",
        source_registry=None,
        dry_run=False,
        keep_sqlite_readonly=False,
        json=False,
    )

    with pytest.raises(StriatumError) as exc:
        _dispatch_daemon(args)

    assert exc.value.exit_code == 12
    assert "SQLite import windows are closed" in str(exc.value)
    assert "striatum.daemon" not in sys.modules
    assert "striatum.daemon_pg.cutover" not in sys.modules



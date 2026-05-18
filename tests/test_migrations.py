from __future__ import annotations

import json
import os
import sqlite3
import subprocess
import sys
from pathlib import Path
from typing import Any, cast

import pytest

from striatum import migrations as migrations_module
from striatum.migrations import LATEST_VERSION, MIGRATIONS, Migration, apply_migrations

ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = ROOT / "examples" / "rfc-ledger-cleanup" / "workflow.json"


JsonDict = dict[str, Any]


def run_cli(repo: Path, *args: str, check: bool = True) -> JsonDict:
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [sys.executable, "-m", "striatum.cli", "--repo", str(repo), *args, "--json"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if check and result.returncode != 0:
        raise AssertionError(
            f"command failed: {result.args}\nstdout={result.stdout}\nstderr={result.stderr}"
        )
    if result.stdout.strip() == "":
        return {}
    payload = cast(JsonDict, json.loads(result.stdout))
    payload["returncode"] = result.returncode
    return payload


def data(payload: JsonDict) -> JsonDict:
    value = payload["data"]
    assert isinstance(value, dict)
    return cast(JsonDict, value)


def db_file(repo: Path) -> Path:
    return repo / ".striatum" / "state.sqlite3"


def legacy_init_repo(repo: Path) -> None:
    from striatum.db import init_repo

    init_repo(repo)


def read_user_version(path: Path) -> int:
    conn = sqlite3.connect(path)
    try:
        row = conn.execute("PRAGMA user_version").fetchone()
        assert row is not None
        return int(row[0])
    finally:
        conn.close()


def test_init_sets_current_user_version(tmp_path: Path) -> None:
    legacy_init_repo(tmp_path)
    assert db_file(tmp_path).exists()
    assert read_user_version(db_file(tmp_path)) == LATEST_VERSION


def test_connect_to_older_db_runs_migrations(tmp_path: Path) -> None:
    state_dir = tmp_path / ".striatum"
    state_dir.mkdir(parents=True, exist_ok=True)
    target = db_file(tmp_path)
    conn = sqlite3.connect(target)
    try:
        # Create a stub V1 database with PRAGMA user_version=0 and only a
        # subset of the v1 schema present, so the migration must complete
        # the install on connect.
        conn.execute(
            """
            CREATE TABLE schema_meta (
              key TEXT PRIMARY KEY,
              value TEXT NOT NULL
            )
            """
        )
        conn.execute("PRAGMA user_version = 0")
        conn.commit()
    finally:
        conn.close()

    assert read_user_version(target) == 0

    # Any CLI command that connects (here: status) should silently upgrade
    # the database to LATEST_VERSION.
    payload = data(run_cli(tmp_path, "status"))
    assert payload["runs"] == []

    assert read_user_version(target) == LATEST_VERSION

    # The migration should have created the rest of the v1 tables.
    conn = sqlite3.connect(target)
    try:
        rows = conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
        ).fetchall()
        names = {row[0] for row in rows}
    finally:
        conn.close()
    for required in (
        "runs",
        "sessions",
        "jobs",
        "queue_messages",
        "leases",
        "events",
    ):
        assert required in names


def test_connect_to_future_db_refuses(tmp_path: Path) -> None:
    # Initialize a real database so the file is well-formed, then set
    # PRAGMA user_version to a future value the runner cannot support.
    legacy_init_repo(tmp_path)
    target = db_file(tmp_path)
    conn = sqlite3.connect(target)
    try:
        conn.execute("PRAGMA user_version = 999")
        conn.commit()
    finally:
        conn.close()

    rejected = run_cli(tmp_path, "status", check=False)
    assert rejected["returncode"] == 9
    error = rejected["error"]
    assert isinstance(error, dict)
    message = str(error["message"])
    assert "newer than this installation supports" in message
    assert "user_version=999" in message


def test_dummy_v2_migration_preserves_data(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    # Initialize at the real LATEST_VERSION (currently 1) and prepare a run
    # so v1 data exists.
    legacy_init_repo(tmp_path)
    prepared = data(run_cli(tmp_path, "run", "prepare", "--workflow", str(WORKFLOW)))
    run_id = str(prepared["run_id"])

    target = db_file(tmp_path)
    base_version = read_user_version(target)
    assert base_version == LATEST_VERSION

    # Register a dummy v2 migration that creates a marker table and inserts
    # a sentinel row. Use the next free version number above the real
    # latest so the registry still validates after the patch.
    new_version = LATEST_VERSION + 1

    def _apply_v2(conn: sqlite3.Connection) -> None:
        conn.execute(
            """
            CREATE TABLE test_marker (
              marker_id TEXT PRIMARY KEY,
              note TEXT NOT NULL
            )
            """
        )
        conn.execute(
            "INSERT INTO test_marker(marker_id, note) VALUES (?, ?)",
            ("m1", "applied"),
        )

    extended_registry = list(MIGRATIONS) + [
        Migration(version=new_version, label="dummy v2 migration", apply=_apply_v2),
    ]
    monkeypatch.setattr(migrations_module, "MIGRATIONS", extended_registry)
    monkeypatch.setattr(migrations_module, "LATEST_VERSION", new_version)

    # Trigger a reconnect by calling apply_migrations directly against the
    # existing database. This is the path taken by every CLI connect.
    conn = sqlite3.connect(target)
    try:
        apply_migrations(conn)
    finally:
        conn.close()

    # Existing v1 data must still be present.
    conn = sqlite3.connect(target)
    try:
        runs = conn.execute("SELECT run_id FROM runs").fetchall()
        marker = conn.execute(
            "SELECT marker_id, note FROM test_marker"
        ).fetchall()
    finally:
        conn.close()
    assert [row[0] for row in runs] == [run_id]
    assert marker == [("m1", "applied")]
    assert read_user_version(target) == new_version

    # Re-running apply_migrations is a no-op once the database is current.
    conn = sqlite3.connect(target)
    try:
        apply_migrations(conn)
    finally:
        conn.close()
    assert read_user_version(target) == new_version

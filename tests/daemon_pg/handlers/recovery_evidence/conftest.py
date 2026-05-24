# ruff: noqa
"""Parity helpers for Track B recovery + evidence handler tests.

This conftest exposes PostgreSQL fixtures for the per-method test files:

1. ``pg_ctx`` — a :class:`RepoHandlerContext` bound to an ephemeral
   PostgreSQL database with the latest migrations applied.

Per RFC 0048 V1.5 #1 the parity rig is now fully unblocked — Track A's
remaining handlers (``record_verdict``, ``submit_review``,
``override_review_verdict``) landed in v1.49.0, so the parent package
imports cleanly without the historical workflow-loop stubs.
"""

from __future__ import annotations

import importlib
from dataclasses import dataclass
from pathlib import Path
from types import ModuleType
from typing import Any, Iterator

import pytest


def import_handler(module_name: str) -> ModuleType:
    """Import a Track B handler module by its short name."""
    full = f"striatum.daemon_pg.handlers.recovery_evidence.{module_name}"
    return importlib.import_module(full)


@dataclass
class Seed:
    """Minimal fixture that mirrors the PostgreSQL runner state we need."""

    repository_id: str
    run_id: str
    job_id: str
    workflow_job_id: str
    session_id: str
    lease_id: str
    message_id: str
    repo_root: Path


def _pg_resolve_url() -> str | None:
    try:
        from _harness.pg import resolve_base_url
    except ImportError:
        return None
    try:
        return resolve_base_url()
    except pytest.skip.Exception:
        return None


@pytest.fixture
def pg_url() -> str:
    """Return a reachable PG base URL or skip."""
    url = _pg_resolve_url()
    if url is None:
        pytest.skip(
            "Track B parity tests require a reachable system PostgreSQL URL; "
            "set STRIATUM_TEST_POSTGRES_URL or run make pg-test first"
        )
    return url


@pytest.fixture
def pg_db(pg_url: str) -> Iterator[Any]:
    """Yield a connection to an ephemeral PG database with migrations applied."""
    from _harness.pg import (
        create_ephemeral_database,
        drop_ephemeral_database,
    )
    from striatum.daemon_pg.connection import connect

    ephemeral = create_ephemeral_database(pg_url)
    conn = connect(ephemeral.database_url)
    try:
        yield conn
    finally:
        conn.close()
        drop_ephemeral_database(ephemeral)


@pytest.fixture
def pg_ctx(pg_db: Any, tmp_path: Path) -> Any:
    """Return a :class:`RepoHandlerContext` for the ephemeral PG database."""
    from striatum.daemon_pg.handlers.context import RepoHandlerContext
    from striatum.daemon_rpc.capability import RpcAuthContext

    repository_id = "rep_track_b_test"
    with pg_db.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path,
              display_name, registered_at, last_schema_version, state
            ) VALUES (%s, %s, %s, %s, %s, now(), 0, 'active')
            ON CONFLICT (repository_id) DO NOTHING
            """,
            (
                repository_id,
                f"identity:{repository_id}",
                str(tmp_path),
                str(tmp_path / ".striatum" / "state.sqlite3"),
                repository_id,
            ),
        )
    pg_db.commit()
    auth = RpcAuthContext(None, None, repository_id, None, "allowed")
    return RepoHandlerContext(
        pg_conn=pg_db,
        repository_id=repository_id,
        repo_root=tmp_path,
        auth=auth,
    )


__all__ = ["Seed", "import_handler"]

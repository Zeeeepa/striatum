"""Parity helpers for Track B recovery + evidence handler tests.

This conftest exposes three things to the seven per-method test files:

1. ``pg_ctx`` — a :class:`RepoHandlerContext` bound to an ephemeral
   PostgreSQL database with the latest migrations applied.
2. ``sqlite_conn`` — a fresh SQLite connection migrated to the same
   schema version the CLI uses today.
3. ``parity_seed`` — a small ``Seed`` object that writes the same fixture
   shape into both stores so handlers can be invoked on each side and
   asserted byte-equal (the synthesis "byte-identical state vs the
   SQLite-backed equivalent on the same input fixture" contract).

The full parity helpers are intentionally minimal until Track A's
remaining ``workflow_loop`` modules (``record_verdict``,
``submit_review``, ``override_review_verdict``) land. Importing
``striatum.daemon_pg.handlers`` requires those to exist; until they do,
the handler tests run by importing handler modules directly (bypassing
the package ``__init__.py``) — see ``import_handler`` below.
"""

from __future__ import annotations

import importlib
import importlib.util
import sqlite3
import sys
from dataclasses import dataclass
from pathlib import Path
from types import ModuleType
from typing import Any, Iterator

import pytest


HANDLERS_ROOT = (
    Path(__file__).resolve().parents[4]
    / "src"
    / "striatum"
    / "daemon_pg"
    / "handlers"
    / "recovery_evidence"
)


def _stub_missing_workflow_loop_modules() -> None:
    """Pre-populate ``sys.modules`` with empty stubs so the parent package
    import chain succeeds even when Track A has not yet landed all 9
    workflow-loop handler modules.

    This is a *test-only* workaround. Once Track A finishes
    ``record_verdict``, ``submit_review``, and ``override_review_verdict``
    the stubs are harmlessly shadowed by the real modules.
    """
    parent = "striatum.daemon_pg.handlers.workflow_loop"
    for missing in ("record_verdict", "submit_review", "override_review_verdict"):
        name = f"{parent}.{missing}"
        if name not in sys.modules:
            sys.modules[name] = ModuleType(name)


def import_handler(module_name: str) -> ModuleType:
    """Import a Track B handler module by its short name.

    The function returns ``recovery_evidence.<module_name>``, stubbing any
    Track A submodules whose absence would otherwise crash the parent
    package import.
    """
    _stub_missing_workflow_loop_modules()
    full = f"striatum.daemon_pg.handlers.recovery_evidence.{module_name}"
    return importlib.import_module(full)


@dataclass
class Seed:
    """Minimal fixture that mirrors the SQLite + PG runner state we need."""

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
    """Return a :class:`RepoHandlerContext` for the ephemeral PG database.

    Track A's :class:`striatum.daemon_pg.handlers.context.RepoHandlerContext`
    is the locked context shape. We bypass the parent package ``__init__``
    to avoid Track A's incomplete workflow-loop import.
    """
    _stub_missing_workflow_loop_modules()
    from striatum.daemon_pg.handlers.context import RepoHandlerContext
    from striatum.daemon_rpc.capability import RpcAuthContext

    repository_id = "rep_track_b_test"
    with pg_db.cursor() as cur:
        cur.execute(
            "INSERT INTO striatumd.repositories(repository_id, repo_root, state) "
            "VALUES (%s, %s, 'active') ON CONFLICT (repository_id) DO NOTHING",
            (repository_id, str(tmp_path)),
        )
    pg_db.commit()
    auth = RpcAuthContext(None, None, repository_id, None, "allowed")
    return RepoHandlerContext(
        pg_conn=pg_db,
        repository_id=repository_id,
        repo_root=tmp_path,
        auth=auth,
    )


@pytest.fixture
def sqlite_conn(tmp_path: Path) -> Iterator[sqlite3.Connection]:
    """Return a fresh SQLite connection at the latest schema version."""
    repo = tmp_path / "repo"
    (repo / ".striatum").mkdir(parents=True)
    from striatum.db import connect

    conn = connect(repo)
    try:
        yield conn
    finally:
        conn.close()


__all__ = ["Seed", "import_handler"]

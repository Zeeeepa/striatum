from __future__ import annotations

import os
from collections.abc import Iterator
from typing import cast

import pytest

from _harness.daemon import DaemonCore
from _harness.multi_repo import MultiRepoHarness, pg_available_url


@pytest.fixture(autouse=True, scope="session")
def _legacy_sqlite_fixtures_opt_out() -> Iterator[None]:
    """RFC 0043 V1.5: daemon-required is the default in production.

    The current test suite still has SQLite-backed fixtures that should
    migrate incrementally rather than by a single disruptive rewrite, so
    the session-level opt-out keeps them green. Tests that exercise the
    daemon-required surface directly (`tests/exit_codes/test_rfc0043_refusals.py`)
    delete or override the env at function scope to assert enforcement.
    """
    previous = os.environ.get("STRIATUM_DAEMON_REQUIRED")
    os.environ["STRIATUM_DAEMON_REQUIRED"] = "0"
    try:
        yield
    finally:
        if previous is None:
            os.environ.pop("STRIATUM_DAEMON_REQUIRED", None)
        else:
            os.environ["STRIATUM_DAEMON_REQUIRED"] = previous


@pytest.fixture(scope="session")
def postgres_url() -> str:
    return pg_available_url()


@pytest.fixture(scope="class")
def daemon_core() -> DaemonCore:
    core = os.environ.get("STRIATUM_MULTI_REPO_DAEMON_CORE", "python")
    if core not in {"python", "go"}:
        raise pytest.UsageError(
            f"STRIATUM_MULTI_REPO_DAEMON_CORE must be python or go, got {core!r}"
        )
    return cast(DaemonCore, core)


@pytest.fixture(scope="class")
def multi_repo_harness(
    tmp_path_factory: pytest.TempPathFactory,
    postgres_url: str,
    daemon_core: DaemonCore,
) -> Iterator[MultiRepoHarness]:
    harness = MultiRepoHarness(
        daemon_pg_url=postgres_url,
        repo_count=2,
        scratch_dir=tmp_path_factory.mktemp("multi_repo"),
        daemon_core=daemon_core,
    )
    harness.start()
    try:
        yield harness
    finally:
        harness.stop()


@pytest.fixture
def clean_daemon_db(multi_repo_harness: MultiRepoHarness) -> Iterator[None]:
    multi_repo_harness.reset_daemon_db()
    yield

from __future__ import annotations

import os
from collections.abc import Iterator

import pytest

from _harness.daemon import DaemonCore
from _harness.multi_repo import MultiRepoHarness, pg_available_url


@pytest.fixture(autouse=True, scope="session")
def _legacy_sqlite_fixtures_opt_out() -> Iterator[None]:
    """RFC 0043 V1.6: daemon-required is the default in production and the
    bare ``STRIATUM_DAEMON_REQUIRED=0`` opt-out no longer bypasses it.

    The test harness pairs ``STRIATUM_DAEMON_REQUIRED=0`` with
    ``STRIATUM_TEST_HARNESS=1`` so legacy SQLite-backed fixtures stay green
    during the incremental migration. Production callers without the
    harness marker stay enforced. Tests that exercise the daemon-required
    surface directly (`tests/exit_codes/test_rfc0043_refusals.py`) clear
    or override either env var at function scope to assert enforcement.
    """
    previous = os.environ.get("STRIATUM_DAEMON_REQUIRED")
    previous_harness = os.environ.get("STRIATUM_TEST_HARNESS")
    os.environ["STRIATUM_DAEMON_REQUIRED"] = "0"
    os.environ["STRIATUM_TEST_HARNESS"] = "1"
    try:
        yield
    finally:
        if previous is None:
            os.environ.pop("STRIATUM_DAEMON_REQUIRED", None)
        else:
            os.environ["STRIATUM_DAEMON_REQUIRED"] = previous
        if previous_harness is None:
            os.environ.pop("STRIATUM_TEST_HARNESS", None)
        else:
            os.environ["STRIATUM_TEST_HARNESS"] = previous_harness


@pytest.fixture(scope="session")
def postgres_url() -> str:
    return pg_available_url()


@pytest.fixture(scope="class")
def daemon_core() -> DaemonCore:
    core = os.environ.get("STRIATUM_MULTI_REPO_DAEMON_CORE", "go")
    if core != "go":
        raise pytest.UsageError(
            f"STRIATUM_MULTI_REPO_DAEMON_CORE must be go, got {core!r}"
        )
    return "go"


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

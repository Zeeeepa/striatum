from __future__ import annotations

from collections.abc import Iterator

import pytest

from _harness.multi_repo import MultiRepoHarness, pg_available_url


@pytest.fixture(scope="session")
def postgres_url() -> str:
    return pg_available_url()


@pytest.fixture(scope="class")
def multi_repo_harness(tmp_path_factory: pytest.TempPathFactory, postgres_url: str) -> Iterator[MultiRepoHarness]:
    harness = MultiRepoHarness(
        daemon_pg_url=postgres_url,
        repo_count=2,
        scratch_dir=tmp_path_factory.mktemp("multi_repo"),
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

from __future__ import annotations

from striatum.migrations import LATEST_VERSION
from striatum.repo_local_schema import LATEST_REPO_LOCAL_SCHEMA_VERSION


def test_repo_local_schema_constant_matches_legacy_migrations() -> None:
    assert LATEST_REPO_LOCAL_SCHEMA_VERSION == LATEST_VERSION

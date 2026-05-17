from __future__ import annotations

from collections.abc import Iterator

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.migrations import LATEST_DAEMON_DB_VERSION, read_schema_version


pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_migration_0008_adds_artifact_override_rationale(pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT column_name, is_nullable, data_type
                FROM information_schema.columns
                WHERE table_schema = 'striatumd'
                  AND table_name = 'artifacts'
                  AND column_name = 'attestation_override_rationale'
                """
            )
            row = cur.fetchone()
        assert row is not None
        assert row[0] == "attestation_override_rationale"
        assert row[1] == "YES"
        assert row[2] == "text"
        assert read_schema_version(conn) == LATEST_DAEMON_DB_VERSION == 8
    finally:
        conn.close()

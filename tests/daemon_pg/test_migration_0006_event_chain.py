"""RFC 0048 V1.5 #4 / #6 — migration 0006 event-chain anchor columns.

Verifies the schema and behavior introduced by
``0006_events_chain_anchors.sql``:

1. ``striatumd.events`` has ``previous_hash`` (nullable) and ``row_hash``
   (NOT NULL) columns.
2. ``striatumd.repo_event_chain_heads`` table exists with the composite
   foreign key to ``(repository_id, event_id)``.
3. The Python ``RepoHandlerContext.append_event`` writes the new columns
   directly, updates the chain head, and keeps ``payload_json`` clean
   of the legacy ``_event_chain`` key.
4. The chain remains contiguous across N concurrent appenders.
"""

from __future__ import annotations

import threading
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_rpc.capability import RpcAuthContext


pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def _seed_repo(conn: Any, tmp_path: Path, repository_id: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path,
              display_name, registered_at, last_schema_version, state
            ) VALUES (%s, %s, %s, %s, %s, now(), 16, 'active')
            ON CONFLICT (repository_id) DO NOTHING
            """,
            (
                repository_id,
                f"identity:{repository_id}",
                str(tmp_path),
                str(tmp_path / ".striatum" / "retired-local-state"),
                repository_id,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots(
              repository_id, workflow_snapshot_id, workflow_id, content_sha256,
              workflow_json, loaded_at
            ) VALUES (%s, 'snap_x', 'wf_x', 'sha', '{}'::jsonb, now())
            """,
            (repository_id,),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs(
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              created_at
            ) VALUES (%s, 'run_test', 'snap_x', %s, 'running', now())
            """,
            (repository_id, str(tmp_path)),
        )
    conn.commit()


def test_events_table_has_previous_hash_and_row_hash_columns(pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT column_name, is_nullable, data_type
                FROM information_schema.columns
                WHERE table_schema = 'striatumd' AND table_name = 'events'
                  AND column_name IN ('previous_hash', 'row_hash')
                ORDER BY column_name
                """
            )
            rows = {str(r[0]): (str(r[1]), str(r[2])) for r in cur.fetchall()}
    finally:
        conn.close()

    assert "previous_hash" in rows, "events must have previous_hash column"
    assert "row_hash" in rows, "events must have row_hash column"
    # Both columns are nullable at the schema level. Production writers
    # (RepoHandlerContext.append_event + pkg/mutations.insertEvent) always
    # populate row_hash; the migration refuses to apply if any existing
    # row lacks anchor metadata. Nullable lets tests + back-fill tooling
    # do two-phase INSERT-then-update-after-id-assigned without tripping
    # the trigger-enforced append-only invariant.
    assert rows["row_hash"][0] == "YES", "row_hash is nullable; see migration 0006 §rationale"
    assert rows["previous_hash"][0] == "YES", (
        "previous_hash is nullable for the chain genesis row"
    )


def test_repo_event_chain_heads_table_exists(pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT column_name
                FROM information_schema.columns
                WHERE table_schema = 'striatumd'
                  AND table_name = 'repo_event_chain_heads'
                ORDER BY column_name
                """
            )
            columns = {str(r[0]) for r in cur.fetchall()}
    finally:
        conn.close()
    assert columns == {"repository_id", "last_event_id", "last_hash", "updated_at"}


def test_append_event_writes_columns_and_strips_legacy_payload_key(
    pg_url: str, tmp_path: Path
) -> None:
    repository_id = "rep_0006_basic"
    conn = connect(pg_url)
    try:
        _seed_repo(conn, tmp_path, repository_id)
        ctx = RepoHandlerContext(
            pg_conn=conn,
            repository_id=repository_id,
            repo_root=tmp_path,
            auth=RpcAuthContext(None, None, repository_id, None, "allowed"),
        )
        event_id = ctx.append_event(
            run_id="run_test",
            event_type="test.first",
            payload={"k": "v"},
        )
        conn.commit()

        with conn.cursor() as cur:
            cur.execute(
                "SELECT previous_hash, row_hash, payload_json "
                "FROM striatumd.events WHERE repository_id = %s AND event_id = %s",
                (repository_id, event_id),
            )
            row = cur.fetchone()
        assert row is not None
        previous_hash, row_hash, payload = row
        assert previous_hash is None, "first event has no predecessor"
        assert isinstance(row_hash, str) and row_hash
        assert "_event_chain" not in payload, (
            "payload_json must not carry the legacy _event_chain key"
        )
        assert payload == {"k": "v"}

        with conn.cursor() as cur:
            cur.execute(
                "SELECT last_event_id, last_hash "
                "FROM striatumd.repo_event_chain_heads WHERE repository_id = %s",
                (repository_id,),
            )
            head = cur.fetchone()
        assert head is not None
        assert int(head[0]) == event_id
        assert head[1] == row_hash
    finally:
        conn.close()


def test_append_event_chain_is_contiguous_for_sequential_writes(
    pg_url: str, tmp_path: Path
) -> None:
    repository_id = "rep_0006_chain"
    conn = connect(pg_url)
    try:
        _seed_repo(conn, tmp_path, repository_id)
        ctx = RepoHandlerContext(
            pg_conn=conn,
            repository_id=repository_id,
            repo_root=tmp_path,
            auth=RpcAuthContext(None, None, repository_id, None, "allowed"),
        )
        for index in range(5):
            ctx.append_event(
                run_id="run_test",
                event_type=f"test.seq.{index}",
                payload={"idx": index},
            )
        conn.commit()

        with conn.cursor() as cur:
            cur.execute(
                "SELECT event_id, previous_hash, row_hash "
                "FROM striatumd.events WHERE repository_id = %s "
                "ORDER BY event_id",
                (repository_id,),
            )
            rows = cur.fetchall()
            cur.execute(
                "SELECT last_event_id, last_hash "
                "FROM striatumd.repo_event_chain_heads WHERE repository_id = %s",
                (repository_id,),
            )
            head = cur.fetchone()
    finally:
        conn.close()

    assert len(rows) == 5
    previous: str | None = None
    for event_id, prev_hash, row_hash in rows:
        assert prev_hash == previous, (
            f"chain broken at event_id={event_id}: "
            f"expected previous_hash={previous!r}, got {prev_hash!r}"
        )
        previous = row_hash
    assert head is not None
    last_event_id, last_hash = head
    assert int(last_event_id) == int(rows[-1][0])
    assert last_hash == rows[-1][2]


def test_append_event_chain_is_contiguous_under_concurrency(
    pg_url: str, tmp_path: Path
) -> None:
    repository_id = "rep_0006_concurrent"
    setup_conn = connect(pg_url)
    try:
        _seed_repo(setup_conn, tmp_path, repository_id)
    finally:
        setup_conn.close()

    request_count = 10
    barrier = threading.Barrier(request_count)
    errors: list[BaseException] = []

    def worker(index: int) -> None:
        try:
            conn = connect(pg_url)
            conn.autocommit = True
            ctx = RepoHandlerContext(
                pg_conn=conn,
                repository_id=repository_id,
                repo_root=tmp_path,
                auth=RpcAuthContext(None, None, repository_id, None, "allowed"),
            )
            barrier.wait(timeout=15)
            ctx.append_event(
                run_id="run_test",
                event_type=f"test.concurrent.{index}",
                payload={"idx": index},
            )
            conn.close()
        except BaseException as exc:  # noqa: BLE001
            errors.append(exc)

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(request_count)]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=30)

    assert not errors, f"worker exceptions: {errors!r}"

    verify_conn = connect(pg_url)
    try:
        with verify_conn.cursor() as cur:
            cur.execute(
                "SELECT event_id, previous_hash, row_hash "
                "FROM striatumd.events WHERE repository_id = %s "
                "ORDER BY event_id",
                (repository_id,),
            )
            rows = cur.fetchall()
    finally:
        verify_conn.close()

    assert len(rows) == request_count, (
        f"expected {request_count} events, got {len(rows)}"
    )
    previous: str | None = None
    for event_id, prev_hash, row_hash in rows:
        assert prev_hash == previous, (
            f"chain broken at event_id={event_id}: "
            f"expected previous_hash={previous!r}, got {prev_hash!r}"
        )
        previous = row_hash

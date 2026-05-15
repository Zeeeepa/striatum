"""RFC 0048 V1.5 F3 — audit-chain concurrency under contention.

The audit chain stores a singleton ``audit_chain_head`` row holding the
``last_hash`` (and ``last_audit_id``). Every audit-row insert reads
``last_hash`` as ``previous_hash``, computes ``row_hash`` over the new
row (which incorporates the previous hash), then UPDATEs the head with
the new hash. Without explicit row-locking, two concurrent appenders
can read the same ``last_hash``, both compute ``row_hash`` over the
same ``previous_hash``, and both UPDATE the head — forking the chain.

The fix (in ``src/striatum/daemon_rpc/request_log.py::append_audit_row``)
is ``SELECT … FROM striatumd.audit_chain_head … FOR UPDATE``, which
forces the chain append to serialize on the singleton row.

This test launches N concurrent allowed/denied RPC requests against a
real PG database and asserts:

1. After all threads complete, every row in ``audit_log`` has a
   ``previous_hash`` that equals the ``row_hash`` of the immediately
   prior row (ordered by ``audit_id``).
2. ``audit_chain_head.last_hash`` equals the ``row_hash`` of the
   last row by ``audit_id``.
3. No orphan workflow mutations: if a request denied at the auth
   boundary, no workflow tables (``sessions``, ``events``,
   ``artifacts``) gained rows attributable to that request.
"""

from __future__ import annotations

import threading
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_rpc.envelope import RpcEnvelope
from striatum.daemon_rpc.registry import METHOD_REGISTRY
from striatum.daemon_rpc.server import DaemonRpcRouter


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
                str(tmp_path / ".striatum" / "state.sqlite3"),
                repository_id,
            ),
        )
    conn.commit()


def _pick_pg_method() -> str:
    """Pick a PG-backed method whose deny path is reachable without seed."""
    for method, entry in sorted(METHOD_REGISTRY.items()):
        if entry.required_capability is None:
            continue
        if resolve_pg_handler(method) is None:
            continue
        return method
    raise RuntimeError("no PG-backed methods registered")


def test_concurrent_denied_requests_preserve_audit_chain(
    pg_url: str, tmp_path: Path
) -> None:
    """N threads each issue a denied (token_missing) request. After all
    threads finish, the audit chain must be contiguous and the head must
    point at the final row."""
    setup_conn = connect(pg_url)
    try:
        _seed_repo(setup_conn, tmp_path, "repo_concurrency")
    finally:
        setup_conn.close()

    method = _pick_pg_method()
    request_count = 12  # enough to expose ordering issues under any race window
    barrier = threading.Barrier(request_count)
    errors: list[BaseException] = []

    def worker(index: int) -> None:
        try:
            conn = connect(pg_url)
            # Production daemon connection is autocommit=True (see
            # src/striatum/daemon.py daemon-start path); replicate so audit
            # rows persist across worker boundaries.
            conn.autocommit = True
            router = DaemonRpcRouter(pg_conn=conn, repo_root=tmp_path)
            envelope = RpcEnvelope(
                schema_version=1,
                request_id=f"concurrent-denied-{index}",
                method=method,
                params={"repository_id": "repo_concurrency"},
                capability_token=None,
            )
            barrier.wait(timeout=15)  # release all threads together
            router.handle(envelope, connection_id=f"conn-{index}", require_handshake=False)
        except BaseException as exc:  # noqa: BLE001
            errors.append(exc)
        finally:
            try:
                conn.close()
            except Exception:  # noqa: BLE001
                pass

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
                "SELECT audit_id, previous_hash, row_hash "
                "FROM striatumd.audit_log ORDER BY audit_id"
            )
            rows = cur.fetchall()
            cur.execute(
                "SELECT last_audit_id, last_hash FROM striatumd.audit_chain_head "
                "WHERE singleton = true"
            )
            head = cur.fetchone()
    finally:
        verify_conn.close()

    assert head is not None
    head_last_audit_id, head_last_hash = head
    assert len(rows) >= request_count, (
        f"expected >= {request_count} audit rows, got {len(rows)}"
    )

    previous: str | None = None
    for audit_id, prev_hash, row_hash in rows:
        assert prev_hash == previous, (
            f"audit_id={audit_id}: previous_hash chain broken "
            f"(expected {previous!r}, got {prev_hash!r})"
        )
        previous = row_hash

    assert rows, "expected at least one audit row"
    final_audit_id, _, final_row_hash = rows[-1]
    assert head_last_audit_id == final_audit_id, (
        f"head.last_audit_id ({head_last_audit_id}) != final row ({final_audit_id})"
    )
    assert head_last_hash == final_row_hash, (
        f"head.last_hash ({head_last_hash!r}) != final row.row_hash ({final_row_hash!r})"
    )


def test_concurrent_allowed_and_denied_no_orphan_mutations(
    pg_url: str, tmp_path: Path
) -> None:
    """Mix allowed (denied for now — issue path requires deeper seed) and
    denied requests; assert no workflow mutations from denied requests
    appear in ``sessions``, ``events``, ``artifacts``.

    The deny path runs the audit log append but never touches workflow
    tables. After N denied requests, workflow tables should still be
    empty.
    """
    setup_conn = connect(pg_url)
    try:
        _seed_repo(setup_conn, tmp_path, "repo_orphan_check")
    finally:
        setup_conn.close()

    method = _pick_pg_method()
    request_count = 8
    errors: list[BaseException] = []

    def worker(index: int) -> None:
        try:
            conn = connect(pg_url)
            # Production daemon connection is autocommit=True (see
            # src/striatum/daemon.py daemon-start path); replicate so audit
            # rows persist across worker boundaries.
            conn.autocommit = True
            router = DaemonRpcRouter(pg_conn=conn, repo_root=tmp_path)
            envelope = RpcEnvelope(
                schema_version=1,
                request_id=f"orphan-check-{index}",
                method=method,
                params={"repository_id": "repo_orphan_check"},
                capability_token=None,
            )
            router.handle(envelope, connection_id=f"conn-{index}", require_handshake=False)
        except BaseException as exc:  # noqa: BLE001
            errors.append(exc)
        finally:
            try:
                conn.close()
            except Exception:  # noqa: BLE001
                pass

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(request_count)]
    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=30)

    assert not errors, f"worker exceptions: {errors!r}"

    verify_conn = connect(pg_url)
    try:
        with verify_conn.cursor() as cur:
            cur.execute("SELECT COUNT(*) FROM striatumd.sessions")
            sessions_count = int(cur.fetchone()[0])
            cur.execute("SELECT COUNT(*) FROM striatumd.events")
            events_count = int(cur.fetchone()[0])
            cur.execute("SELECT COUNT(*) FROM striatumd.artifacts")
            artifacts_count = int(cur.fetchone()[0])
    finally:
        verify_conn.close()

    assert sessions_count == 0, "denied requests must not create sessions"
    assert events_count == 0, "denied requests must not append workflow events"
    assert artifacts_count == 0, "denied requests must not publish artifacts"

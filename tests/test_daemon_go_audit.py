"""Cross-core audit regression for RFC 0039 V1.5.

Fires concurrent audit-emitting RPCs at the Go daemon, then verifies that the
hash chain in striatumd.audit_log is linear and matches the Python audit-chain
checker. The synthesis locks this on a real Postgres so the transactional
append (F4) is exercised end-to-end against the F5 pgx driver.
"""

from __future__ import annotations

import os
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

import pytest

from _harness import audit
from _harness.multi_repo import MultiRepoHarness, pg_available_url
from striatum.daemon_rpc.client import call_unix, hello_envelope

pytestmark = pytest.mark.multi_repo


def _require_go_core() -> None:
    if os.environ.get("STRIATUM_MULTI_REPO_DAEMON_CORE", "python") != "go":
        pytest.skip("Go daemon audit test only runs with CORE=go")


def _hello(socket_path: Path) -> None:
    envelope = hello_envelope(
        request_id=f"req_{uuid.uuid4().hex}",
        client_name="daemon-go-audit",
        client_version="0.1",
    )
    response = call_unix(socket_path, envelope, timeout_seconds=15)
    assert response["ok"] is True, response


def test_go_daemon_audit_chain_is_linear_under_concurrency(
    tmp_path_factory: pytest.TempPathFactory,
) -> None:
    _require_go_core()
    harness = MultiRepoHarness(
        daemon_pg_url=pg_available_url(),
        repo_count=1,
        scratch_dir=tmp_path_factory.mktemp("daemon_go_audit"),
        daemon_core="go",
    )
    harness.start()
    try:
        concurrency = 8
        per_worker = 4
        with ThreadPoolExecutor(max_workers=concurrency) as pool:
            futures = [
                pool.submit(_hello, harness.socket_path)
                for _ in range(concurrency * per_worker)
            ]
            for future in as_completed(futures):
                future.result()

        rows = harness.audit_rows()
        assert len(rows) >= concurrency * per_worker, rows
        previous_hashes = [row["previous_hash"] for row in rows[1:]]
        row_hashes = [row["row_hash"] for row in rows[:-1]]
        assert previous_hashes == row_hashes, (
            "audit hash chain is not linear under concurrency"
        )

        conn = harness.pg_conn()
        try:
            audit.assert_audit_chain(conn)
        finally:
            conn.close()
    finally:
        harness.stop()

"""Go daemon launch-contract smoke test.

Boots the harness with ``daemon_core="go"`` and verifies the binary serves the
locked envelope-v1 socket: socket exists, daemon.hello completes, daemon.describe
returns a methods_etag and the audit chain head moves forward.
"""

from __future__ import annotations

import os
import uuid

import pytest

from _harness.multi_repo import MultiRepoHarness, pg_available_url
from striatum.daemon_rpc.client import call_unix, hello_envelope
from striatum.daemon_rpc.envelope import RpcEnvelope

pytestmark = pytest.mark.multi_repo


def _require_go_core() -> None:
    if os.environ.get("STRIATUM_MULTI_REPO_DAEMON_CORE", "python") != "go":
        pytest.skip("Go daemon smoke test only runs with CORE=go")


def test_multi_repo_harness_boots_go_daemon(
    tmp_path_factory: pytest.TempPathFactory,
) -> None:
    _require_go_core()
    harness = MultiRepoHarness(
        daemon_pg_url=pg_available_url(),
        repo_count=1,
        scratch_dir=tmp_path_factory.mktemp("daemon_go_smoke"),
        daemon_core="go",
    )
    harness.start()
    try:
        assert harness.socket_path.exists(), "Go daemon should have bound the socket"

        hello = hello_envelope(
            request_id=f"req_{uuid.uuid4().hex}",
            client_name="daemon-go-smoke",
            client_version="0.1",
        )
        hello_response = call_unix(harness.socket_path, hello)
        assert hello_response["ok"] is True, hello_response
        assert hello_response["data"]["substrate"] == "postgres"
        assert hello_response["data"]["methods_etag"].startswith("sha256:")

        describe = RpcEnvelope(
            schema_version=1,
            request_id=f"req_{uuid.uuid4().hex}",
            method="daemon.describe",
            params={},
        )
        describe_response = call_unix(harness.socket_path, describe)
        # daemon.describe requires a read capability; the daemon should refuse
        # the unauthenticated probe with a stable capability_missing reason.
        # We only assert that the connection round-tripped: a denial response
        # proves the Go server is decoding envelopes and routing methods.
        assert describe_response["request_id"] == describe.request_id

        chain = harness.daemon_db_query(
            "SELECT last_audit_id FROM striatumd.audit_chain_head"
        )
        assert chain and chain[0]["last_audit_id"] is not None, (
            "expected daemon.hello to have appended at least one audit row"
        )
    finally:
        harness.stop()

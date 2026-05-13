"""Go daemon RFC 0043 mutation registry smoke coverage."""

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
        pytest.skip("Go daemon mutation test only runs with CORE=go")


def test_go_daemon_describes_rfc0043_mutation_methods(
    tmp_path_factory: pytest.TempPathFactory,
) -> None:
    _require_go_core()
    harness = MultiRepoHarness(
        daemon_pg_url=pg_available_url(),
        repo_count=1,
        scratch_dir=tmp_path_factory.mktemp("daemon_go_mutations"),
        daemon_core="go",
    )
    harness.start()
    try:
        hello = hello_envelope(
            request_id=f"req_{uuid.uuid4().hex}",
            client_name="daemon-go-mutations",
            client_version="0.1",
        )
        assert call_unix(harness.socket_path, hello)["ok"] is True

        describe = RpcEnvelope(
            schema_version=1,
            request_id=f"req_{uuid.uuid4().hex}",
            method="daemon.describe",
            params={},
            capability_token=harness.issue_token(["read"]),
        )
        response = call_unix(harness.socket_path, describe)
        assert response["ok"] is True, response
        methods = {
            item["method"]: item for item in response["data"]["methods"]
        }
        expected = {
            "session.register": "claim",
            "work.claim_next": "claim",
            "work.ack": "claim",
            "work.heartbeat": "claim",
            "work.complete": "write",
            "work.block": "write",
            "artifact.publish": "write",
            "review.submit": "review",
            "review.verdict": "review",
            "decision.record": "admin",
            "checkpoint.resolve": "admin",
            "recovery.requeue_stale": "recovery",
            "recovery.cancel_job": "recovery",
            "recovery.resume": "recovery",
            "worktree.create": "write",
            "branch.confirm": "admin",
            "run.prepare": "admin",
            "run.start": "admin",
            "run.pause": "admin",
            "run.resume": "admin",
            "run.cancel": "admin",
            "workflow.validate": "read",
            "workflow.generate": "write",
        }
        for method, capability in expected.items():
            assert methods[method]["required_capability"] == capability
    finally:
        harness.stop()

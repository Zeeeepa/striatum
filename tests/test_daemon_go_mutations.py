"""Go daemon RFC 0043 mutation registry smoke coverage."""

from __future__ import annotations

import os
import uuid
from typing import Any

import pytest
from psycopg.types.json import Jsonb

from _harness.multi_repo import MultiRepoHarness, pg_available_url
from _harness.repos import RepoDescriptor
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


def test_go_daemon_cross_repo_cancel_uses_socket_and_pg_lifecycle(
    tmp_path_factory: pytest.TempPathFactory,
) -> None:
    _require_go_core()
    harness = MultiRepoHarness(
        daemon_pg_url=pg_available_url(),
        repo_count=2,
        scratch_dir=tmp_path_factory.mktemp("daemon_go_cross_repo_cancel"),
        daemon_core="go",
    )
    harness.start()
    try:
        primary = harness.repos[0]
        consumer = harness.repos[1]
        assert primary.repository_id is not None
        assert consumer.repository_id is not None

        _seed_cross_repo_cancel_fixture(
            harness,
            primary=primary,
            consumer=consumer,
            consumer_repo_state="disabled",
        )

        hello = hello_envelope(
            request_id=f"req_{uuid.uuid4().hex}",
            client_name="daemon-go-cross-repo-cancel",
            client_version="0.1",
        )
        assert call_unix(harness.socket_path, hello)["ok"] is True

        envelope = RpcEnvelope(
            schema_version=1,
            request_id=f"req_{uuid.uuid4().hex}",
            method="cross_repo.cancel",
            params={
                "cross_repo_run_id": "xrun_go_cancel",
                "reason": "operator request",
            },
            capability_token=harness.issue_token(["recovery"]),
        )
        response = call_unix(harness.socket_path, envelope)

        assert response["ok"] is True, response
        assert response["data"] == {
            "cross_repo_run_id": "xrun_go_cancel",
            "state": "blocked",
            "canceled": ["primary"],
            "blocked": ["consumer"],
            "blocked_errors": {
                "consumer": f"active repository not found: {consumer.repository_id}",
            },
        }
        assert harness.daemon_db_query(
            """
            SELECT repository_alias, state
            FROM striatumd.cross_repo_run_repositories
            WHERE cross_repo_run_id = 'xrun_go_cancel'
            ORDER BY repository_alias
            """
        ) == [
            {"repository_alias": "consumer", "state": "blocked"},
            {"repository_alias": "primary", "state": "canceled"},
        ]
        assert harness.daemon_db_query(
            """
            SELECT repository_id, run_id, state, stop_reason
            FROM striatumd.runs
            WHERE cross_repo_run_id = 'xrun_go_cancel'
            ORDER BY run_id
            """
        ) == [
            {
                "repository_id": consumer.repository_id,
                "run_id": "run_consumer",
                "state": "running",
                "stop_reason": None,
            },
            {
                "repository_id": primary.repository_id,
                "run_id": "run_primary",
                "state": "canceled",
                "stop_reason": "operator request",
            },
        ]
        assert harness.daemon_db_query(
            """
            SELECT method, decision
            FROM striatumd.audit_log
            WHERE request_id = %s
            """,
            (envelope.request_id,),
        ) == [{"method": "cross_repo.cancel", "decision": "allowed"}]
    finally:
        harness.stop()


def _seed_cross_repo_cancel_fixture(
    harness: MultiRepoHarness,
    *,
    primary: RepoDescriptor,
    consumer: RepoDescriptor,
    consumer_repo_state: str,
) -> None:
    assert primary.repository_id is not None
    assert consumer.repository_id is not None
    conn = harness.pg_conn()
    try:
        _seed_cancelable_participant_run(
            conn,
            repo=primary,
            repository_id=primary.repository_id,
            run_id="run_primary",
        )
        _seed_cancelable_participant_run(
            conn,
            repo=consumer,
            repository_id=consumer.repository_id,
            run_id="run_consumer",
        )
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE striatumd.repositories
                   SET state = %s
                 WHERE repository_id = %s
                """,
                (consumer_repo_state, consumer.repository_id),
            )
            cur.execute(
                """
                INSERT INTO striatumd.cross_repo_runs (
                  cross_repo_run_id, workflow_id, workflow_version,
                  workflow_snapshot_hash, primary_repository_id, state,
                  created_at, started_at, reconcile_after
                )
                VALUES (
                  'xrun_go_cancel', 'wf_go_cancel', '1', repeat('0', 64),
                  %s, 'running', now(), now(), now()
                )
                """,
                (primary.repository_id,),
            )
            for alias, repo_id, run_id in (
                ("primary", primary.repository_id, "run_primary"),
                ("consumer", consumer.repository_id, "run_consumer"),
            ):
                cur.execute(
                    """
                    INSERT INTO striatumd.cross_repo_run_repositories (
                      cross_repo_run_id, repository_alias, repository_id,
                      local_run_id, state, prepared_at, last_observed_at
                    )
                    VALUES (
                      'xrun_go_cancel', %s, %s, %s, 'running', now(), now()
                    )
                    """,
                    (alias, repo_id, run_id),
                )
        conn.commit()
    finally:
        conn.close()


def _seed_cancelable_participant_run(
    conn: Any,
    *,
    repo: RepoDescriptor,
    repository_id: str,
    run_id: str,
) -> None:
    workflow = {
        "workflow_id": "wf_go_cancel",
        "workflow_version": "1",
        "roles": {"author": {}},
        "lanes": {"local": {}},
        "jobs": [{"id": "root", "type": "draft"}],
    }
    suffix = repository_id.replace("-", "_")
    snapshot_id = f"snap_{suffix}"
    job_id = f"job_{suffix}"
    session_id = f"sess_{suffix}"
    lease_id = f"lease_{suffix}"
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots (
              repository_id, workflow_snapshot_id, workflow_id,
              workflow_version, content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, %s, 'wf_go_cancel', '1', %s, %s, now())
            """,
            (
                repository_id,
                snapshot_id,
                "0" * 64,
                Jsonb(workflow),
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs (
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              created_at, started_at, cross_repo_run_id
            )
            VALUES (
              %s, %s, %s, %s, 'running', now(), now(), 'xrun_go_cancel'
            )
            """,
            (repository_id, run_id, snapshot_id, str(repo.path)),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json,
              state, write_scope_json, expected_artifacts_json,
              idempotency_key, created_at, started_at
            )
            VALUES (
              %s, %s, %s, 'root', 'Root', 'draft', 'author',
              %s, %s, 'running', %s, %s, 'root:1', now(), now()
            )
            """,
            (
                repository_id,
                job_id,
                run_id,
                Jsonb({"lane_id": "local"}),
                Jsonb([]),
                Jsonb({"repo_write": False}),
                Jsonb([]),
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions (
              repository_id, session_id, run_id, role_id, lane_id, slug,
              ordinal, capabilities_json, state, registered_at,
              last_heartbeat_at
            )
            VALUES (
              %s, %s, %s, 'author', 'local', %s, 1,
              %s, 'active', now(), now()
            )
            """,
            (
                repository_id,
                session_id,
                run_id,
                f"author-local-{suffix}",
                Jsonb(["admin"]),
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases (
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at
            )
            VALUES (
              %s, %s, %s, 'job', %s, %s,
              'active', now(), '2099-01-01T00:00:00Z'
            )
            """,
            (repository_id, lease_id, run_id, job_id, session_id),
        )
        cur.execute(
            """
            UPDATE striatumd.jobs
               SET current_lease_id = %s
             WHERE repository_id = %s AND job_id = %s
            """,
            (lease_id, repository_id, job_id),
        )

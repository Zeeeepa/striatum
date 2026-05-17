from __future__ import annotations

import inspect
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.recovery_evidence.sweep import handle

from .test_auto_finalize import (
    _count,
    _ctx,
    _events,
    _insert_attached_supervisor,
    _insert_review_job_instance,
    _insert_work_packet_and_clean_process,
    _one,
    _seed_review_job,
    _states,
    _write_finding,
)


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_recovery_sweep_handler_registered() -> None:
    from striatum.daemon_pg.handlers.registry import resolve_pg_handler

    assert resolve_pg_handler("recovery.sweep") is handle
    assert list(inspect.signature(handle).parameters) == ["ctx", "params"]


def test_recovery_sweep_auto_finalizes_before_lazy_lease_expiry_when_opted_in(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001")
        _seed_review_job(conn, repo_root, repository_id="repo_a")
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {"run_id": "run_1", "dry_run": False, "mtime_grace_seconds": 0},
        )

        assert result["run_id"] == "run_1"
        assert result["dry_run"] is False
        assert [action["kind"] for action in result["actions"]] == ["auto_finalized"]
        assert result["actions"][0]["result"]["finalized_count"] == 1
        assert _states(conn, repository_id="repo_a")["run"] == "completed"
        event_types = [row["event_type"] for row in _events(conn, "repo_a")]
        assert "artifact.auto_finalized" in event_types
        assert "job.auto_finalized" in event_types
        assert "lease.expired" not in event_types
    finally:
        conn.close()


def test_recovery_sweep_acceptance_auto_finalizes_valid_written_artifacts_without_operator_on_behalf(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(
            repo_root,
            path_text="docs/review_1.md",
            byline="author: reviewer-codex-gpt-5-001",
        )
        _write_finding(
            repo_root,
            path_text="docs/review_2.md",
            byline="author: reviewer-codex-gpt-5-002",
            verdict_intent="accept",
        )
        _write_finding(
            repo_root,
            path_text="docs/review_3.md",
            byline="author: reviewer-codex-gpt-5-003",
            verdict_intent="accept_with_findings",
        )
        _seed_review_job(
            conn,
            repo_root,
            repository_id="repo_a",
            expected_artifacts=[
                {
                    "logical_name": "review",
                    "kind": "finding",
                    "path": "docs/review_1.md",
                    "required": True,
                }
            ],
        )
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        _insert_review_job_instance(
            conn,
            repository_id="repo_a",
            workflow_job_id="review_2",
            job_id="job_2",
            session_id="sess_2",
            session_ordinal=2,
            lease_id="lease_2",
            message_id="msg_2",
            artifact_path="docs/review_2.md",
        )
        _insert_review_job_instance(
            conn,
            repository_id="repo_a",
            workflow_job_id="review_3",
            job_id="job_3",
            session_id="sess_3",
            session_ordinal=3,
            lease_id="lease_3",
            message_id="msg_3",
            artifact_path="docs/review_3.md",
        )
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {"run_id": "run_1", "dry_run": False, "mtime_grace_seconds": 0},
        )

        assert [action["kind"] for action in result["actions"]] == ["auto_finalized"]
        assert result["actions"][0]["result"]["finalized_count"] == 3
        assert result["actions"][0]["result"]["skipped_count"] == 0
        assert _count(conn, "striatumd.artifacts", repository_id="repo_a") == 3
        assert _count(conn, "striatumd.verdicts", repository_id="repo_a") == 3
        assert _run_state(conn, repository_id="repo_a") == "completed"
        assert _states_by_job(conn, repository_id="repo_a") == {
            "job_1": "completed",
            "job_2": "completed",
            "job_3": "completed",
        }
        assert _states_by_lease(conn, repository_id="repo_a") == {
            "lease_1": "released",
            "lease_2": "released",
            "lease_3": "released",
        }

        events = _events(conn, "repo_a")
        event_types = [row["event_type"] for row in events]
        assert event_types.count("artifact.auto_finalized") == 3
        assert event_types.count("job.auto_finalized") == 3
        assert "dogfood.publish_on_behalf" not in event_types
        assert "provenance.publish_without_process_execution" not in event_types

        with conn.cursor(row_factory=dict_row) as cur:
            cur.execute(
                """
                SELECT repo_path, attestation_override_rationale
                FROM striatumd.artifacts
                WHERE repository_id = %s
                ORDER BY repo_path
                """,
                ("repo_a",),
            )
            artifacts = [dict(row) for row in cur.fetchall()]
        assert artifacts == [
            {"repo_path": "docs/review_1.md", "attestation_override_rationale": None},
            {"repo_path": "docs/review_2.md", "attestation_override_rationale": None},
            {"repo_path": "docs/review_3.md", "attestation_override_rationale": None},
        ]
    finally:
        conn.close()


def test_recovery_sweep_does_not_force_auto_finalize_without_workflow_opt_in(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001")
        _seed_review_job(
            conn,
            repo_root,
            repository_id="repo_a",
            auto_finalize_enabled=False,
        )
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {"run_id": "run_1", "dry_run": False, "mtime_grace_seconds": 0, "force": True},
        )

        assert result["actions"] == []
        assert result["still_stuck"] == []
        assert _count(conn, "striatumd.artifacts", repository_id="repo_a") == 0
        assert _states(conn, repository_id="repo_a")["job"] == "running"
    finally:
        conn.close()


def test_recovery_sweep_dry_run_reports_auto_finalize_without_mutation(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _write_finding(repo_root, byline="author: reviewer-codex-gpt-5-001")
        _seed_review_job(conn, repo_root, repository_id="repo_a")
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_work_packet_and_clean_process(conn, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {"run_id": "run_1", "dry_run": True, "mtime_grace_seconds": 0},
        )

        assert [action["kind"] for action in result["actions"]] == [
            "auto_finalize_eligible"
        ]
        assert result["actions"][0]["result"]["eligible_count"] == 1
        assert _count(conn, "striatumd.artifacts", repository_id="repo_a") == 0
        assert _states(conn, repository_id="repo_a")["job"] == "running"
    finally:
        conn.close()


def test_recovery_sweep_blocks_expired_attached_supervisor_stall(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _seed_review_job(
            conn,
            repo_root,
            repository_id="repo_a",
            auto_finalize_enabled=False,
        )
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _expire_seeded_lease(conn, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {"run_id": "run_1", "dry_run": False, "supervisor_stall_after_seconds": 1},
        )

        assert [action["kind"] for action in result["actions"]] == [
            "supervisor_stall_blocked"
        ]
        action = result["actions"][0]
        assert action["job_id"] == "job_1"
        assert action["lease_id"] == "lease_1"
        assert action["lease_expired"] is True
        assert action["blocker_id"].startswith("blk_")

        assert _states(conn, repository_id="repo_a") == {
            "job": "blocked",
            "job_lease": None,
            "message": "blocked",
            "message_lease": None,
            "lease": "expired",
            "lease_reason": "heartbeat_stall",
            "run": "running",
            "run_stop_reason": None,
            "session": "active",
        }
        assert _one(
            conn,
            """
            SELECT blocker_kind, severity, state, job_id, session_id
            FROM striatumd.blockers
            WHERE repository_id = %s
            """,
            ("repo_a",),
        ) == {
            "blocker_kind": "heartbeat_stall_lease_expired",
            "severity": "blocked",
            "state": "open",
            "job_id": "job_1",
            "session_id": "sess_1",
        }
        assert _one(
            conn,
            """
            SELECT state, stop_reason
            FROM striatumd.process_supervisors
            WHERE repository_id = %s AND supervisor_id = 'sup_1'
            """,
            ("repo_a",),
        ) == {
            "state": "lost",
            "stop_reason": "lease expired after supervisor heartbeat stall",
        }
        event_types = [row["event_type"] for row in _events(conn, "repo_a")]
        assert "supervisor.heartbeat_stall" in event_types
        assert "lease.expired" in event_types
        assert "job.blocked" in event_types
        assert "supervisor.lease_expired_with_supervisor" in event_types

        second = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {"run_id": "run_1", "dry_run": False, "supervisor_stall_after_seconds": 1},
        )
        assert second["actions"] == []
        assert _count(conn, "striatumd.blockers", repository_id="repo_a") == 1
    finally:
        conn.close()


def test_recovery_sweep_dry_run_reports_expired_supervisor_stall_without_mutation(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _seed_review_job(
            conn,
            repo_root,
            repository_id="repo_a",
            auto_finalize_enabled=False,
        )
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _expire_seeded_lease(conn, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {"run_id": "run_1", "dry_run": True, "supervisor_stall_after_seconds": 1},
        )

        assert [action["kind"] for action in result["actions"]] == [
            "supervisor_stall_block_eligible"
        ]
        assert _count(conn, "striatumd.blockers", repository_id="repo_a") == 0
        assert _states(conn, repository_id="repo_a")["job"] == "running"
        assert _states(conn, repository_id="repo_a")["lease"] == "active"
        assert _one(
            conn,
            """
            SELECT state
            FROM striatumd.process_supervisors
            WHERE repository_id = %s AND supervisor_id = 'sup_1'
            """,
            ("repo_a",),
        ) == {"state": "attached"}
    finally:
        conn.close()


def test_recovery_auto_deprecated_alias_stays_auto_publish_not_sweep() -> None:
    from striatum.daemon_pg.handlers.registry import resolve_pg_handler

    from ._helpers import import_handler

    auto_publish = import_handler("auto_publish_stale_artifacts")
    assert resolve_pg_handler("recovery.auto") is auto_publish.handle
    assert resolve_pg_handler("recovery.auto") is not handle


def _run_state(conn: Any, *, repository_id: str) -> str:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            "SELECT state FROM striatumd.runs WHERE repository_id = %s AND run_id = 'run_1'",
            (repository_id,),
        )
        row = cur.fetchone()
    assert row is not None
    return str(row["state"])


def _states_by_job(conn: Any, *, repository_id: str) -> dict[str, str]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            SELECT job_id, state
            FROM striatumd.jobs
            WHERE repository_id = %s
            ORDER BY job_id
            """,
            (repository_id,),
        )
        return {str(row["job_id"]): str(row["state"]) for row in cur.fetchall()}


def _states_by_lease(conn: Any, *, repository_id: str) -> dict[str, str]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            SELECT lease_id, state
            FROM striatumd.leases
            WHERE repository_id = %s
            ORDER BY lease_id
            """,
            (repository_id,),
        )
        return {str(row["lease_id"]): str(row["state"]) for row in cur.fetchall()}


def _expire_seeded_lease(conn: Any, *, repository_id: str) -> None:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            UPDATE striatumd.leases
            SET expires_at = '2026-05-14T00:10:00Z',
                last_heartbeat_at = '2026-05-14T00:00:00Z'
            WHERE repository_id = %s AND lease_id = 'lease_1'
            """,
            (repository_id,),
        )
        cur.execute(
            """
            UPDATE striatumd.process_supervisors
            SET heartbeat_at = '2026-05-14T00:00:00Z'
            WHERE repository_id = %s AND supervisor_id = 'sup_1'
            """,
            (repository_id,),
        )

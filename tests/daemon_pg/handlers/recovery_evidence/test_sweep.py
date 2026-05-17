from __future__ import annotations

import inspect
from collections.abc import Iterator
from pathlib import Path

import pytest

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.recovery_evidence.sweep import handle

from .test_auto_finalize import (
    _count,
    _ctx,
    _events,
    _insert_attached_supervisor,
    _insert_work_packet_and_clean_process,
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


def test_recovery_auto_deprecated_alias_stays_auto_publish_not_sweep() -> None:
    from striatum.daemon_pg.handlers.registry import resolve_pg_handler

    from ._helpers import import_handler

    auto_publish = import_handler("auto_publish_stale_artifacts")
    assert resolve_pg_handler("recovery.auto") is auto_publish.handle
    assert resolve_pg_handler("recovery.auto") is not handle

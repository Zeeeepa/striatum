from __future__ import annotations

import json
from pathlib import Path

import pytest
from psycopg.types.json import Jsonb

from _harness.multi_repo import MultiRepoHarness
from striatum.daemon_pg import client_admin as daemon
from striatum.daemon_pg.config import ENV_DAEMON_DB_URL

pytestmark = pytest.mark.multi_repo


def _configure_pg_daemon_sweep(
    harness: MultiRepoHarness,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
    *,
    token: str | None,
) -> None:
    monkeypatch.setenv(ENV_DAEMON_DB_URL, harness.daemon_pg_url)
    monkeypatch.setenv(daemon.ENV_RUNTIME, str(tmp_path / "runtime"))
    monkeypatch.setenv(daemon.ENV_REGISTRY, str(tmp_path / "daemon" / "striatumd.sqlite3"))
    monkeypatch.setenv(daemon.ENV_SQLITE_CONNECT_TRIPWIRE, "1")
    monkeypatch.delenv(daemon.ENV_ALLOW_LEGACY_SQLITE_REGISTRY, raising=False)
    if token is not None:
        daemon.write_runtime_token(token)


def _seed_running_run(harness: MultiRepoHarness) -> tuple[str, str]:
    harness.register_all()
    repo = harness.repos[0]
    repository_id = str(repo.repository_id)
    run_id = "run_sweep_pg"
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "pg-sweep",
        "workflow_version": "1",
        "jobs": [],
        "edges": [],
    }
    conn = harness.pg_conn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                INSERT INTO striatumd.workflow_snapshots(
                  repository_id, workflow_snapshot_id, workflow_id, workflow_version,
                  source_path, content_sha256, workflow_json, loaded_at
                ) VALUES (%s, 'snap_sweep_pg', 'pg-sweep', '1',
                          'workflow.json', 'sha256:sweep', %s, now())
                """,
                (repository_id, Jsonb(workflow)),
            )
            cur.execute(
                """
                INSERT INTO striatumd.runs(
                  repository_id, run_id, workflow_snapshot_id, repo_root, state,
                  branch_name, branch_base, branch_confirmed_at, created_at, started_at
                ) VALUES (%s, %s, 'snap_sweep_pg', %s, 'running',
                          'striatum/sweep', 'main', now(), now(), now())
                """,
                (repository_id, run_id, str(repo.path)),
            )
        conn.commit()
    finally:
        conn.close()
    return repository_id, run_id


def test_daemon_sweep_timeout_marks_postgres_cursor_degraded_and_doctor_reports_it(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    repository_id, run_id = _seed_running_run(harness)
    _configure_pg_daemon_sweep(
        harness,
        monkeypatch,
        tmp_path,
        token=harness.issue_token(["admin"]),
    )

    result = daemon.daemon_sweep_once(
        per_run_timeout_seconds=-1,
        require_client_auth=True,
    )

    assert result["mode"] == "daemon"
    assert result["sweeps"][0]["repository_id"] == repository_id
    assert result["sweeps"][0]["run_id"] == run_id
    conn = harness.pg_conn()
    try:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT state, last_result_json
                FROM striatumd.scheduler_cursors
                WHERE repository_id = %s AND run_id = %s AND cursor_kind = 'recovery'
                """,
                (repository_id, run_id),
            )
            row = cur.fetchone()
        assert row is not None
        state, payload = row
        assert state == "sweep_degraded"
        decoded = payload if isinstance(payload, dict) else json.loads(str(payload))
        assert decoded["degraded_reason"] == "sweep_timeout"
        checks = {record["check"] for record in daemon.daemon_doctor_records_pg(conn)}
    finally:
        conn.close()
    assert "daemon_sweep_degraded" in checks


def test_daemon_sweep_cli_requires_admin_token_and_audits_denial_in_postgres(
    multi_repo_harness: MultiRepoHarness,
    clean_daemon_db: None,
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    harness = multi_repo_harness
    _seed_running_run(harness)
    _configure_pg_daemon_sweep(harness, monkeypatch, tmp_path, token=None)

    with pytest.raises(daemon.DaemonAuthError, match="token_missing"):
        daemon.daemon_sweep_once(require_client_auth=True)

    row = harness.audit_rows(transport="cli")[-1]
    assert row["method"] == "daemon.sweep"
    assert row["decision"] == "denied"
    assert row["denial_reason"] == "token_missing"

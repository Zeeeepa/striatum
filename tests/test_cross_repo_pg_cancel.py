from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.cross_repo import PgCrossRepoLocalRunner, cancel_cross_repo_run
from striatum.daemon_pg.connection import connect

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_pg_cross_repo_cancel_cascades_to_participant_runs(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        _seed_repo_run(conn, tmp_path, repository_id="repo_primary", run_id="run_primary")
        _seed_repo_run(conn, tmp_path, repository_id="repo_consumer", run_id="run_consumer")
        _seed_cross_repo(
            conn,
            primary_id="repo_primary",
            consumer_id="repo_consumer",
            primary_run="run_primary",
            consumer_run="run_consumer",
        )
        conn.commit()

        result = cancel_cross_repo_run(
            conn,
            cross_repo_run_id="xrun_1",
            local_runner=PgCrossRepoLocalRunner(conn),
            reason="operator request",
        )

        assert result == {
            "cross_repo_run_id": "xrun_1",
            "state": "canceled",
            "canceled": ["consumer", "primary"],
            "blocked": [],
        }
        assert _rows(
            conn,
            """
            SELECT repository_id, state, stop_reason
            FROM striatumd.runs
            ORDER BY repository_id
            """,
        ) == [
            {
                "repository_id": "repo_consumer",
                "state": "canceled",
                "stop_reason": "operator request",
            },
            {
                "repository_id": "repo_primary",
                "state": "canceled",
                "stop_reason": "operator request",
            },
        ]
        assert _rows(
            conn,
            """
            SELECT repository_id, state, release_reason
            FROM striatumd.leases
            ORDER BY repository_id
            """,
        ) == [
            {
                "repository_id": "repo_consumer",
                "state": "released",
                "release_reason": "run_canceled",
            },
            {
                "repository_id": "repo_primary",
                "state": "released",
                "release_reason": "run_canceled",
            },
        ]
        assert _rows(
            conn,
            """
            SELECT repository_id, state, close_reason
            FROM striatumd.sessions
            ORDER BY repository_id
            """,
        ) == [
            {
                "repository_id": "repo_consumer",
                "state": "closed",
                "close_reason": "run_canceled",
            },
            {
                "repository_id": "repo_primary",
                "state": "closed",
                "close_reason": "run_canceled",
            },
        ]
        assert _rows(
            conn,
            """
            SELECT repository_alias, state
            FROM striatumd.cross_repo_run_repositories
            ORDER BY repository_alias
            """,
        ) == [
            {"repository_alias": "consumer", "state": "canceled"},
            {"repository_alias": "primary", "state": "canceled"},
        ]
        assert _one(
            conn,
            "SELECT state FROM striatumd.cross_repo_runs WHERE cross_repo_run_id = 'xrun_1'",
        ) == {"state": "canceled"}
    finally:
        conn.close()


def test_pg_cross_repo_cancel_blocks_inactive_participant(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        _seed_repo_run(conn, tmp_path, repository_id="repo_primary", run_id="run_primary")
        _seed_repo_run(
            conn,
            tmp_path,
            repository_id="repo_consumer",
            run_id="run_consumer",
            repo_state="disabled",
        )
        _seed_cross_repo(
            conn,
            primary_id="repo_primary",
            consumer_id="repo_consumer",
            primary_run="run_primary",
            consumer_run="run_consumer",
        )
        conn.commit()

        result = cancel_cross_repo_run(
            conn,
            cross_repo_run_id="xrun_1",
            local_runner=PgCrossRepoLocalRunner(conn),
            reason="operator request",
        )

        assert result == {
            "cross_repo_run_id": "xrun_1",
            "state": "blocked",
            "canceled": ["primary"],
            "blocked": ["consumer"],
            "blocked_errors": {
                "consumer": "active repository not found: repo_consumer",
            },
        }
        assert _rows(
            conn,
            """
            SELECT repository_alias, state
            FROM striatumd.cross_repo_run_repositories
            ORDER BY repository_alias
            """,
        ) == [
            {"repository_alias": "consumer", "state": "blocked"},
            {"repository_alias": "primary", "state": "canceled"},
        ]
        assert _one(
            conn,
            """
            SELECT state, stop_reason
            FROM striatumd.runs
            WHERE repository_id = 'repo_primary'
            """,
        ) == {"state": "canceled", "stop_reason": "operator request"}
        assert _one(
            conn,
            """
            SELECT state, stop_reason
            FROM striatumd.runs
            WHERE repository_id = 'repo_consumer'
            """,
        ) == {"state": "running", "stop_reason": None}
        assert _one(
            conn,
            "SELECT state FROM striatumd.cross_repo_runs WHERE cross_repo_run_id = 'xrun_1'",
        ) == {"state": "blocked"}
        assert _one(
            conn,
            """
            SELECT last_reconcile_error
            FROM striatumd.cross_repo_runs
            WHERE cross_repo_run_id = 'xrun_1'
            """,
        ) == {"last_reconcile_error": "consumer: active repository not found: repo_consumer"}
    finally:
        conn.close()


def _seed_repo_run(
    conn: Any,
    tmp_path: Path,
    *,
    repository_id: str,
    run_id: str,
    repo_state: str = "active",
) -> None:
    now = "2026-05-17T00:00:00Z"
    repo_root = tmp_path / repository_id
    repo_root.mkdir()
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"author": {}},
        "lanes": {"local": {}},
        "jobs": [{"id": "root", "type": "draft"}],
    }
    job_id = f"job_{repository_id}"
    session_id = f"sess_{repository_id}"
    lease_id = f"lease_{repository_id}"
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories (
              repository_id, repo_identity, repo_root, state_db_path, display_name,
              registered_at, last_schema_version, state
            )
            VALUES (%s, %s, %s, %s, %s, %s, 5, %s)
            """,
            (
                repository_id,
                repository_id,
                str(repo_root),
                str(repo_root / ".striatum"),
                repository_id,
                now,
                repo_state,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots (
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, %s, 'wf', '1', 'sha', %s, %s)
            """,
            (repository_id, f"snap_{repository_id}", Jsonb(workflow), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs (
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              created_at, started_at, cross_repo_run_id
            )
            VALUES (%s, %s, %s, %s, 'running', %s, %s, 'xrun_1')
            """,
            (repository_id, run_id, f"snap_{repository_id}", str(repo_root), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at, started_at
            )
            VALUES (%s, %s, %s, 'root', 'Root', 'draft', 'author',
                    %s, %s, 'running', %s, %s, 'root:1', %s, %s)
            """,
            (
                repository_id,
                job_id,
                run_id,
                Jsonb({"lane_id": "local"}),
                Jsonb([]),
                Jsonb({"repo_write": False}),
                Jsonb([]),
                now,
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions (
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, %s, %s, 'author', 'local', 'author-local-1',
                    1, %s, 'active', %s, %s)
            """,
            (repository_id, session_id, run_id, Jsonb(["admin"]), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases (
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at
            )
            VALUES (%s, %s, %s, 'job', %s, %s,
                    'active', %s, '2099-01-01T00:00:00Z')
            """,
            (repository_id, lease_id, run_id, job_id, session_id, now),
        )
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET current_lease_id = %s
            WHERE repository_id = %s AND job_id = %s
            """,
            (lease_id, repository_id, job_id),
        )


def _seed_cross_repo(
    conn: Any,
    *,
    primary_id: str,
    consumer_id: str,
    primary_run: str,
    consumer_run: str,
) -> None:
    now = "2026-05-17T00:00:00Z"
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.cross_repo_runs (
              cross_repo_run_id, workflow_id, workflow_version,
              workflow_snapshot_hash, primary_repository_id, state, created_at,
              started_at, reconcile_after
            )
            VALUES ('xrun_1', 'wf', '1', repeat('0', 64), %s,
                    'running', %s, %s, %s)
            """,
            (primary_id, now, now, now),
        )
        for alias, repository_id, run_id in (
            ("primary", primary_id, primary_run),
            ("consumer", consumer_id, consumer_run),
        ):
            cur.execute(
                """
                INSERT INTO striatumd.cross_repo_run_repositories (
                  cross_repo_run_id, repository_alias, repository_id,
                  local_run_id, state, prepared_at, last_observed_at
                )
                VALUES ('xrun_1', %s, %s, %s, 'running', %s, %s)
                """,
                (alias, repository_id, run_id, now, now),
            )


def _rows(conn: Any, sql: str) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql)
        return [dict(row) for row in cur.fetchall()]


def _one(conn: Any, sql: str) -> dict[str, Any]:
    rows = _rows(conn, sql)
    assert len(rows) == 1
    return rows[0]

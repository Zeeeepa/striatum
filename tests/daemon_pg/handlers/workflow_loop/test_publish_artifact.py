from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_pg.handlers.workflow_loop.artifact_publish import handle
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.errors import ArtifactError


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_publish_artifact_records_operator_authored_file(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        artifact_path = repo_root / "docs" / "out.md"
        artifact_path.parent.mkdir(parents=True)
        artifact_path.write_text("# Output\nauthor: operator\n\nDone.\n", encoding="utf-8")
        _seed_running_job(conn, repo_root, repository_id="repo_a")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "session_id": "sess_1",
                "job_id": "job_1",
                "lease_id": "lease_1",
                "path": "docs/out.md",
                "kind": "finding",
                "logical_name": "out",
            },
        )

        assert result["status"] == "published"
        artifact_id = str(result["artifact_id"])
        assert _one(
            conn,
            """
            SELECT artifact_id, repo_path, artifact_kind, author_line
            FROM striatumd.artifacts
            WHERE repository_id = %s AND job_id = %s
            """,
            ("repo_a", "job_1"),
        ) == {
            "artifact_id": artifact_id,
            "repo_path": "docs/out.md",
            "artifact_kind": "finding",
            "author_line": "author: operator",
        }
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "artifact.published"
        ]

        again = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "session_id": "sess_1",
                "job_id": "job_1",
                "lease_id": "lease_1",
                "path_text": "docs/out.md",
                "kind": "finding",
                "logical_name": "out",
            },
        )
        assert again == {"status": "already_published", "artifact_id": artifact_id}
    finally:
        conn.close()


def test_publish_artifact_reads_active_worktree_but_records_logical_path(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        worktree_root = tmp_path / "worktree_a"
        artifact_path = worktree_root / "docs" / "out.md"
        artifact_path.parent.mkdir(parents=True)
        artifact_path.write_text("# Output\nauthor: operator\n\nFrom worktree.\n", encoding="utf-8")
        _seed_running_job(conn, repo_root, repository_id="repo_a")
        _insert_active_worktree(conn, repository_id="repo_a", worktree_root=worktree_root)
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "session_id": "sess_1",
                "job_id": "job_1",
                "lease_id": "lease_1",
                "path": "docs/out.md",
                "kind": "finding",
                "logical_name": "out",
            },
        )

        assert result["status"] == "published"
        assert _one(
            conn,
            "SELECT repo_path, content_sha256 FROM striatumd.artifacts "
            "WHERE repository_id = %s AND artifact_id = %s",
            ("repo_a", result["artifact_id"]),
        ) == {
            "repo_path": "docs/out.md",
            "content_sha256": result["sha256"],
        }
    finally:
        conn.close()


def test_publish_artifact_enforces_scope_and_kind(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        (repo_root / "private").mkdir(parents=True)
        (repo_root / "private" / "out.md").write_text("nope", encoding="utf-8")
        _seed_running_job(conn, repo_root, repository_id="repo_a")
        conn.commit()

        with pytest.raises(ArtifactError, match="outside the job write scope"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "session_id": "sess_1",
                    "job_id": "job_1",
                    "lease_id": "lease_1",
                    "path": "private/out.md",
                    "kind": "finding",
                    "logical_name": "out",
                },
            )

        with pytest.raises(ArtifactError, match="transcript artifacts are not allowed"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "session_id": "sess_1",
                    "job_id": "job_1",
                    "lease_id": "lease_1",
                    "path": "private/out.md",
                    "kind": "transcript",
                    "logical_name": "out",
                },
            )
    finally:
        conn.close()


def test_attested_model_publish_requires_process_execution_or_override(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        artifact_path = repo_root / "docs" / "out.md"
        artifact_path.parent.mkdir(parents=True)
        artifact_path.write_text(
            "# Output\nauthor: implementer-codex-gpt-5-001\n\nDone.\n",
            encoding="utf-8",
        )
        _seed_running_job(conn, repo_root, repository_id="repo_a")
        _insert_attached_supervisor(conn, repository_id="repo_a")
        conn.commit()

        with pytest.raises(ArtifactError, match="lane_evidence_missing"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "session_id": "sess_1",
                    "job_id": "job_1",
                    "lease_id": "lease_1",
                    "path": "docs/out.md",
                    "kind": "finding",
                    "logical_name": "out",
                },
            )

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "session_id": "sess_1",
                "job_id": "job_1",
                "lease_id": "lease_1",
                "path": "docs/out.md",
                "kind": "finding",
                "logical_name": "out",
                "allow_no_process_execution": True,
                "override_rationale": "manual recovery publish",
            },
        )

        assert result["status"] == "published"
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "artifact.published",
            "provenance.publish_without_process_execution",
        ]
        assert _events(conn, "repo_a")[1]["payload_json"]["rationale"] == (
            "manual recovery publish"
        )
        assert _one(
            conn,
            """
            SELECT attestation_override_rationale
            FROM striatumd.artifacts
            WHERE repository_id = %s AND artifact_id = %s
            """,
            ("repo_a", result["artifact_id"]),
        ) == {"attestation_override_rationale": "manual recovery publish"}
    finally:
        conn.close()


def test_attested_model_publish_uses_path_specific_supervisor_evidence(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        artifact_path = repo_root / "docs" / "out.md"
        artifact_path.parent.mkdir(parents=True)
        artifact_path.write_text(
            "# Output\nauthor: implementer-codex-gpt-5-001\n\nDone.\n",
            encoding="utf-8",
        )
        _seed_running_job(conn, repo_root, repository_id="repo_a")
        _insert_attached_supervisor(conn, repository_id="repo_a")
        _insert_clean_process_execution(conn, repository_id="repo_a")
        _insert_artifact_observed_event(
            conn,
            repo_root=repo_root,
            repository_id="repo_a",
            path_text="docs/other.md",
        )
        conn.commit()

        with pytest.raises(ArtifactError, match="lane_evidence_missing"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "session_id": "sess_1",
                    "job_id": "job_1",
                    "lease_id": "lease_1",
                    "path": "docs/out.md",
                    "kind": "finding",
                    "logical_name": "out",
                },
            )

        _insert_artifact_observed_event(
            conn,
            repo_root=repo_root,
            repository_id="repo_a",
            path_text="./docs/out.md",
        )
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "session_id": "sess_1",
                "job_id": "job_1",
                "lease_id": "lease_1",
                "path": "docs/out.md",
                "kind": "finding",
                "logical_name": "out",
            },
        )

        assert result["status"] == "published"
        assert _one(
            conn,
            """
            SELECT repo_path, attestation_override_rationale
            FROM striatumd.artifacts
            WHERE repository_id = %s AND artifact_id = %s
            """,
            ("repo_a", result["artifact_id"]),
        ) == {
            "repo_path": "docs/out.md",
            "attestation_override_rationale": None,
        }
    finally:
        conn.close()


def test_artifact_publish_handler_registered() -> None:
    assert resolve_pg_handler("artifact.publish") is handle
    assert resolve_pg_handler("publish_artifact") is handle


def _ctx(conn: Any, repo_root: Path, *, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "write", "allowed"),
    )


def _seed_running_job(conn: Any, repo_root: Path, *, repository_id: str) -> None:
    now = "2026-05-14T00:00:00Z"
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"implementer": {}},
        "lanes": {"local": {"display_model": "codex-gpt-5"}},
        "jobs": [{"id": "build", "type": "implementation"}],
        "cycles": [],
    }
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories (
              repository_id, repo_identity, repo_root, state_db_path, display_name,
              registered_at, last_schema_version, state
            )
            VALUES (%s, %s, %s, %s, %s, %s, 5, 'active')
            """,
            (repository_id, repository_id, str(repo_root), str(repo_root / ".striatum"), repository_id, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots (
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, 'snap_1', 'wf', '1', 'sha', %s, %s)
            """,
            (repository_id, Jsonb(workflow), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs (
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              created_at, started_at
            )
            VALUES (%s, 'run_1', 'snap_1', %s, 'running', %s, %s)
            """,
            (repository_id, str(repo_root), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions (
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES (%s, 'sess_1', 'run_1', 'implementer', 'local',
                    'implementer-local-1', 1, %s, 'active', %s, %s)
            """,
            (repository_id, Jsonb(["write"]), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at, started_at, current_message_id, current_lease_id
            )
            VALUES (%s, 'job_1', 'run_1', 'build', 'Build', 'build',
                    'implementer', %s, %s, 'running', %s, %s, 'build-1',
                    %s, %s, 'msg_1', 'lease_1')
            """,
            (
                repository_id,
                Jsonb({"lane_id": "local"}),
                Jsonb({}),
                Jsonb({"repo_write": True, "allowed_paths": ["docs/"], "forbidden_paths": []}),
                Jsonb([]),
                now,
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.queue_messages (
              repository_id, message_id, run_id, job_id, kind, state,
              target_role_id, payload_json, created_at, updated_at, claimed_at,
              acked_at, current_lease_id
            )
            VALUES (%s, 'msg_1', 'run_1', 'job_1', 'work', 'acked',
                    'implementer', %s, %s, %s, %s, %s, 'lease_1')
            """,
            (repository_id, Jsonb({}), now, now, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases (
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at
            )
            VALUES (%s, 'lease_1', 'run_1', 'job', 'job_1', 'sess_1',
                    'active', %s, '2099-01-01T00:00:00Z')
            """,
            (repository_id, now),
        )


def _insert_active_worktree(
    conn: Any,
    *,
    repository_id: str,
    worktree_root: Path,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.job_worktrees (
              repository_id, worktree_id, run_id, job_id, lease_id, base_branch,
              worktree_path, state, created_at
            )
            VALUES (%s, 'wt_1', 'run_1', 'job_1', 'lease_1', 'main', %s,
                    'active', '2026-05-14T00:00:00Z')
            """,
            (repository_id, str(worktree_root)),
        )


def _insert_attached_supervisor(conn: Any, *, repository_id: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.process_supervisors (
              repository_id, supervisor_id, run_id, session_id, adapter,
              command_json, cwd, scratch_path, pid, state, started_at,
              heartbeat_at
            )
            VALUES (%s, 'sup_1', 'run_1', 'sess_1', 'test', %s, '/tmp',
                    '/tmp/scratch', 1234, 'attached', '2026-05-14T00:00:00Z',
                    '2026-05-14T00:00:00Z')
            """,
            (repository_id, Jsonb(["agent"])),
        )


def _insert_clean_process_execution(conn: Any, *, repository_id: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.work_packets (
              repository_id, packet_id, run_id, job_id, message_id, lease_id,
              session_id, packet_json, packet_sha256, created_at
            )
            VALUES (%s, 'pkt_1', 'run_1', 'job_1', 'msg_1', 'lease_1',
                    'sess_1', %s, 'sha-packet', '2026-05-14T00:00:00Z')
            """,
            (repository_id, Jsonb({})),
        )
        cur.execute(
            """
            INSERT INTO striatumd.process_executions (
              repository_id, process_id, run_id, job_id, session_id, lease_id,
              packet_id, adapter, command_json, cwd, scratch_path, stdin_mode,
              stdio_mode, pid, state, exit_code, started_at, ended_at
            )
            VALUES (%s, 'proc_1', 'run_1', 'job_1', 'sess_1', 'lease_1',
                    'pkt_1', 'process', %s, '/tmp', '/tmp/scratch',
                    'packet', 'suppressed', 1234, 'exited', 0,
                    '2026-05-14T00:00:00Z', '2026-05-14T00:00:01Z')
            """,
            (repository_id, Jsonb(["agent"])),
        )


def _insert_artifact_observed_event(
    conn: Any,
    *,
    repo_root: Path,
    repository_id: str,
    path_text: str,
) -> None:
    _ctx(conn, repo_root, repository_id=repository_id).append_event(
        run_id="run_1",
        event_type="supervisor.artifact_observed",
        actor_session_id="sess_1",
        payload={
            "supervisor_id": "sup_1",
            "control_event_type": "artifact_observed",
            "payload": {"path": path_text},
        },
    )


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)


def _events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            "SELECT * FROM striatumd.events WHERE repository_id = %s ORDER BY event_id",
            (repository_id,),
        )
        return [dict(row) for row in cur.fetchall()]

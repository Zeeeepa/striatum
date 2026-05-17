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
from striatum.daemon_pg.handlers.run_lifecycle.operator_decisions import handle
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.errors import ArtifactError

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_decision_record_writes_file_artifact_row_and_event(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _seed_run(conn, repo_root, repository_id="repo_a")
        _seed_run(conn, tmp_path / "repo_b", repository_id="repo_b")
        conn.commit()

        result = handle(
            _ctx(conn, repo_root, repository_id="repo_a"),
            {
                "run_id": "run_1",
                "path": "docs/decisions/owner-choice.md",
                "decision_id": "owner-choice-001",
                "outcome": "accepted_with_follow_up",
                "title": "Keep decisions as durable artifacts",
                "rationale": "Owner selected a local-first durable record.",
                "follow_up": "Review fuller decision schemas later.",
            },
        )

        assert result["status"] == "recorded"
        assert result["run_id"] == "run_1"
        assert result["decision_id"] == "owner-choice-001"
        assert result["path"] == "docs/decisions/owner-choice.md"
        assert result["outcome"] == "accepted_with_follow_up"
        assert result["run_state_transition"] is None
        assert result["superseded_verdict_count"] == 0

        text = (repo_root / "docs" / "decisions" / "owner-choice.md").read_text(encoding="utf-8")
        assert "schema_version: striatum.decision.v1" in text
        assert 'decision_id: "owner-choice-001"' in text
        assert "artifact_kind: decision" in text
        assert "outcome: accepted_with_follow_up" in text
        assert "follow_up_required: true" in text
        assert "## Rationale" in text
        assert "## Follow-Up" in text

        artifact = _one(
            conn,
            """
            SELECT artifact_kind, job_id, session_id, logical_name, repo_path,
                   content_sha256, size_bytes, author_line
            FROM striatumd.artifacts
            WHERE repository_id = %s AND artifact_id = %s
            """,
            ("repo_a", result["artifact_id"]),
        )
        assert artifact == {
            "artifact_kind": "decision",
            "job_id": None,
            "session_id": None,
            "logical_name": "owner-choice-001",
            "repo_path": "docs/decisions/owner-choice.md",
            "content_sha256": result["sha256"],
            "size_bytes": len(text.encode("utf-8")),
            "author_line": None,
        }
        events = _events(conn, "repo_a")
        assert [row["event_type"] for row in events] == ["decision.recorded"]
        assert events[0]["artifact_id"] == result["artifact_id"]
        assert events[0]["payload_json"] == {
            "decision_id": "owner-choice-001",
            "outcome": "accepted_with_follow_up",
            "path": "docs/decisions/owner-choice.md",
            "sha256": result["sha256"],
        }
        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.artifacts WHERE repository_id = %s",
            ("repo_b",),
        ) == {"count": 0}
        assert _events(conn, "repo_b") == []
    finally:
        conn.close()


def test_decision_record_rejects_duplicate_logical_name_and_path(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _seed_run(conn, repo_root, repository_id="repo_a")
        _seed_decision_artifact(
            conn,
            repository_id="repo_a",
            artifact_id="art_existing",
            logical_name="owner-choice-001",
            repo_path="docs/decisions/owner-choice.md",
        )
        conn.commit()

        with pytest.raises(ArtifactError, match="already exists for this run id/path"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "run_id": "run_1",
                    "path": "docs/decisions/other.md",
                    "decision_id": "owner-choice-001",
                    "outcome": "accepted",
                    "title": "Duplicate logical name",
                },
            )

        with pytest.raises(ArtifactError, match="already exists for this run id/path"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "run_id": "run_1",
                    "path": "docs/decisions/owner-choice.md",
                    "decision_id": "owner-choice-002",
                    "outcome": "accepted",
                    "title": "Duplicate path",
                },
            )
    finally:
        conn.close()


def test_decision_record_rejects_follow_up_without_text_and_path_escape(
    tmp_path: Path,
    pg_url: str,
) -> None:
    conn = connect(pg_url)
    try:
        repo_root = tmp_path / "repo_a"
        _seed_run(conn, repo_root, repository_id="repo_a")
        conn.commit()

        with pytest.raises(ArtifactError, match="accepted_with_follow_up"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "run_id": "run_1",
                    "path": "docs/decisions/follow-up.md",
                    "decision_id": "owner-choice-001",
                    "outcome": "accepted_with_follow_up",
                    "title": "Needs follow-up text",
                },
            )

        with pytest.raises(ArtifactError, match="decision id cannot be empty"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "run_id": "run_1",
                    "path": "docs/decisions/blank-id.md",
                    "decision_id": "  ",
                    "outcome": "accepted",
                    "title": "Blank decision id",
                },
            )

        with pytest.raises(ArtifactError, match="must stay inside the repository"):
            handle(
                _ctx(conn, repo_root, repository_id="repo_a"),
                {
                    "run_id": "run_1",
                    "path": "../outside.md",
                    "decision_id": "owner-choice-002",
                    "outcome": "accepted",
                    "title": "Unsafe path",
                },
            )
        assert not (tmp_path / "outside.md").exists()
        assert _one(
            conn,
            "SELECT COUNT(*) AS count FROM striatumd.artifacts WHERE repository_id = %s",
            ("repo_a",),
        ) == {"count": 0}
        assert _events(conn, "repo_a") == []
    finally:
        conn.close()


def test_decision_record_handler_registered() -> None:
    assert resolve_pg_handler("decision.record") is handle


def _ctx(conn: Any, repo_root: Path, *, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "admin", "allowed"),
    )


def _seed_run(conn: Any, repo_root: Path, *, repository_id: str) -> None:
    repo_root.mkdir(parents=True)
    now = "2026-05-14T00:00:00Z"
    workflow = {
        "workflow_id": "wf",
        "workflow_version": "1",
        "roles": {"operator": {}},
        "lanes": {"local": {}},
        "jobs": [],
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
            (
                repository_id,
                repository_id,
                str(repo_root),
                str(repo_root / ".striatum"),
                repository_id,
                now,
            ),
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
            VALUES (%s, 'run_1', 'snap_1', %s, 'completed', %s, %s)
            """,
            (repository_id, str(repo_root), now, now),
        )


def _seed_decision_artifact(
    conn: Any,
    *,
    repository_id: str,
    artifact_id: str,
    logical_name: str,
    repo_path: str,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.artifacts (
              repository_id, artifact_id, run_id, logical_name, artifact_kind,
              repo_path, content_sha256, size_bytes, publish_mode, created_at
            )
            VALUES (%s, %s, 'run_1', %s, 'decision', %s, 'sha', 1, 'create',
                    '2026-05-14T00:00:00Z')
            """,
            (repository_id, artifact_id, logical_name, repo_path),
        )


def _events(conn: Any, repository_id: str) -> list[dict[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            "SELECT * FROM striatumd.events WHERE repository_id = %s ORDER BY event_id",
            (repository_id,),
        )
        return [dict(row) for row in cur.fetchall()]


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)

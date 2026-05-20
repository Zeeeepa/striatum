from __future__ import annotations

import importlib
import inspect
from collections.abc import Iterator, Mapping
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_pg.migrations import apply_migrations
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import InvalidTransitionError

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_conn(postgres_url: str) -> Iterator[Any]:
    from _harness.pg import create_ephemeral_database, drop_ephemeral_database

    ephemeral = create_ephemeral_database(postgres_url)
    conn = connect(ephemeral.database_url)
    try:
        apply_migrations(conn)
        yield conn
    finally:
        conn.close()
        drop_ephemeral_database(ephemeral)


def test_escalation_handlers_register_with_locked_signature() -> None:
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.escalations")

    assert resolve_pg_handler("escalation.list") is module.list_escalations
    assert resolve_pg_handler("escalation.show") is module.show_escalation
    assert resolve_pg_handler("escalation.resolve") is module.resolve_escalation
    for handler_name in ("list_escalations", "show_escalation", "resolve_escalation"):
        handler = getattr(module, handler_name)
        assert list(inspect.signature(handler).parameters) == ["ctx", "params"]


def test_escalation_list_projects_human_checkpoints_and_rfc0053_kinds(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    _seed(pg_conn, tmp_path)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.escalations")

    result = module.list_escalations(_ctx(pg_conn, tmp_path), {"run_id": "run_1"})

    assert result["count"] == 2
    assert [row["escalation_id"] for row in result["escalations"]] == [
        "blk_authority",
        "blk_human",
    ]
    by_id = {row["escalation_id"]: row for row in result["escalations"]}
    assert by_id["blk_human"]["source"] == "human_checkpoint"
    assert by_id["blk_human"]["class"] == "revision_routing"
    assert by_id["blk_authority"]["source"] == "blocker"
    assert by_id["blk_authority"]["class"] == "missing_authority"
    assert by_id["blk_authority"]["escalation_artifact"] == {
        "artifact_id": "art_escalation",
        "repo_path": "docs/escalations/ESCALATION.md",
        "content_sha256": "sha-escalation",
        "linked_at": "2026-05-16T00:00:01Z",
        "link_source": "artifact.publish",
    }
    assert by_id["blk_human"]["escalation_artifact"] is None


def test_escalation_show_and_resolve_update_blocker_and_append_event(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    _seed(pg_conn, tmp_path)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.escalations")
    ctx = _ctx(pg_conn, tmp_path)

    shown = module.show_escalation(ctx, {"escalation_id": "blk_authority"})
    assert shown["escalation"]["state"] == "open"
    assert shown["escalation"]["escalation_artifact"]["artifact_id"] == "art_escalation"

    inbox_row = _one(
        pg_conn,
        "SELECT state, viewed_at FROM striatumd.escalation_inbox WHERE repository_id = %s AND escalation_id = %s",
        ("repo_1", "blk_authority"),
    )
    assert inbox_row["state"] == "viewed"
    assert inbox_row["viewed_at"] is not None

    # Insert a dummy decision artifact so resolution can link to it
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.artifacts (
              repository_id, artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
              created_at
            )
            VALUES ('repo_1', 'art_decision', 'run_1', NULL, NULL, 'dec_1',
                    'decision', 'docs/decisions/DECISION.md', 'sha-decision', 12,
                    'create', '2026-05-16T00:00:00Z')
            """
        )
    pg_conn.commit()

    resolved = module.resolve_escalation(
        ctx,
        {
            "escalation_id": "blk_authority",
            "decision_id": "dec_1",
            "resolution_note": "principal approved requested authority",
        },
    )

    assert resolved["status"] == "resolved"
    assert resolved["escalation"]["state"] == "resolved"
    blocker = _one(
        pg_conn,
        "SELECT state, payload_json FROM striatumd.blockers WHERE repository_id = %s AND blocker_id = %s",
        ("repo_1", "blk_authority"),
    )
    assert blocker["state"] == "resolved"
    assert blocker["payload_json"]["escalation_resolution"] == {
        "decision_id": "dec_1",
        "resolution_note": "principal approved requested authority",
    }
    
    inbox_resolved = _one(
        pg_conn,
        "SELECT state, resolved_at, decision_artifact_id, resolution_note FROM striatumd.escalation_inbox WHERE repository_id = %s AND escalation_id = %s",
        ("repo_1", "blk_authority"),
    )
    assert inbox_resolved["state"] == "resolved"
    assert inbox_resolved["resolved_at"] is not None
    assert inbox_resolved["decision_artifact_id"] == "art_decision"
    assert inbox_resolved["resolution_note"] == "principal approved requested authority"

    events = _events(pg_conn)
    assert events == [
        {
            "event_type": "escalation.resolved",
            "job_id": "job_1",
            "payload_json": {
                "escalation_id": "blk_authority",
                "decision_id": "dec_1",
                "resolution_note": "principal approved requested authority",
            },
        }
    ]


def test_escalation_resolve_rejects_non_escalation_and_closed_blockers(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    _seed(pg_conn, tmp_path)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.escalations")
    ctx = _ctx(pg_conn, tmp_path)

    with pytest.raises(RpcError, match="escalation not found"):
        module.show_escalation(ctx, {"escalation_id": "blk_process"})
    with pytest.raises(InvalidTransitionError, match="escalation is not open"):
        module.resolve_escalation(ctx, {"escalation_id": "blk_resolved"})


def test_escalation_projection_ignores_stale_artifact_payload_links(
    pg_conn: Any,
    tmp_path: Path,
) -> None:
    _seed(pg_conn, tmp_path)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.escalations")
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.blockers
               SET payload_json = jsonb_set(
                   payload_json,
                   '{escalation_artifact,content_sha256}',
                   '"sha-stale"'::jsonb
               )
             WHERE repository_id = 'repo_1'
               AND blocker_id = 'blk_authority'
            """
        )
    pg_conn.commit()

    shown = module.show_escalation(_ctx(pg_conn, tmp_path), {"escalation_id": "blk_authority"})

    assert shown["escalation"]["escalation_artifact"] is None
    assert shown["escalation"]["payload"]["escalation_artifact"]["artifact_id"] == "art_escalation"


def _ctx(conn: Any, tmp_path: Path) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id="repo_1",
        repo_root=tmp_path / "repo",
        auth=RpcAuthContext("client", "token", "repo_1", "admin", "allowed"),
    )


def _seed(conn: Any, tmp_path: Path) -> None:
    now = "2026-05-16T00:00:00Z"
    repo_root = tmp_path / "repo"
    repo_root.mkdir(exist_ok=True)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories (
              repository_id, repo_identity, repo_root, state_db_path, display_name,
              registered_at, last_schema_version, state
            )
            VALUES ('repo_1', 'repo_1', %s, %s, 'repo_1', %s, 6, 'active')
            """,
            (str(repo_root), str(repo_root / ".striatum" / "state.sqlite3"), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots (
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              content_sha256, workflow_json, loaded_at
            )
            VALUES ('repo_1', 'snap_1', 'wf', '1', 'sha', %s, %s)
            """,
            (Jsonb({"workflow_id": "wf", "jobs": []}), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs (
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              created_at, started_at
            )
            VALUES ('repo_1', 'run_1', 'snap_1', %s, 'running', %s, %s)
            """,
            (str(repo_root), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions (
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES ('repo_1', 'sess_1', 'run_1', 'implementer', 'codex',
                    'implementer-codex-1', 1, %s, 'active', %s, %s)
            """,
            (Jsonb(["read", "write"]), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs (
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              write_scope_json, expected_artifacts_json, idempotency_key,
              created_at
            )
            VALUES ('repo_1', 'job_1', 'run_1', 'build', 'Build', 'build',
                    'implementer', %s, %s, 'blocked', %s, %s, 'build-1', %s)
            """,
            (
                Jsonb({"lane_id": "codex"}),
                Jsonb({}),
                Jsonb({"mode": "repo_write", "allowed_paths": ["src/"], "forbidden_paths": []}),
                Jsonb([]),
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.artifacts (
              repository_id, artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
              created_at
            )
            VALUES ('repo_1', 'art_escalation', 'run_1', 'job_1', 'sess_1',
                    'principal_escalation', 'escalation',
                    'docs/escalations/ESCALATION.md', 'sha-escalation', 12,
                    'create', %s)
            """,
            (now,),
        )
        blockers = [
            ("blk_human", "human_checkpoint", "revision_routing", "open"),
            ("blk_authority", "blocked", "missing_authority", "open"),
            ("blk_process", "blocked", "process_exit_nonzero", "open"),
            ("blk_resolved", "blocked", "override_required", "resolved"),
        ]
        for blocker_id, severity, kind, state in blockers:
            payload: dict[str, Any] = {"seed": blocker_id}
            if blocker_id == "blk_authority":
                payload["escalation_artifact"] = {
                    "artifact_id": "art_escalation",
                    "repo_path": "docs/escalations/ESCALATION.md",
                    "content_sha256": "sha-escalation",
                    "linked_at": "2026-05-16T00:00:01Z",
                    "link_source": "artifact.publish",
                }
            cur.execute(
                """
                INSERT INTO striatumd.blockers (
                  repository_id, blocker_id, run_id, job_id, session_id, severity,
                  blocker_kind, description, state, created_at, resolved_at,
                  payload_json
                )
                VALUES ('repo_1', %s, 'run_1', 'job_1', 'sess_1', %s,
                        %s, %s, %s, %s, %s, %s)
                """,
                (
                    blocker_id,
                    severity,
                    kind,
                    f"{kind} description",
                    state,
                    now,
                    now if state != "open" else None,
                    Jsonb(payload),
                ),
            )
            # Seed escalation_inbox for escalation class blockers
            from striatum.daemon_pg.handlers.reads.escalations import ESCALATION_BLOCKER_KINDS
            is_escalation = (severity == "human_checkpoint") or (kind in ESCALATION_BLOCKER_KINDS)
            if is_escalation:
                inbox_state = "resolved" if state == "resolved" else "pending"
                cur.execute(
                    """
                    INSERT INTO striatumd.escalation_inbox (
                      repository_id, escalation_id, run_id, job_id, session_id,
                      blocker_id, blocker_kind, severity, state, created_at, payload_json
                    )
                    VALUES ('repo_1', %s, 'run_1', 'job_1', 'sess_1',
                            %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        blocker_id,
                        blocker_id,
                        kind,
                        severity,
                        inbox_state,
                        now,
                        Jsonb(payload),
                    ),
                )


def _one(conn: Any, sql: str, params: tuple[Any, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, params)
        row = cur.fetchone()
    assert row is not None
    return dict(row)


def _events(conn: Any) -> list[Mapping[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            SELECT event_type, job_id, payload_json
              FROM striatumd.events
             WHERE repository_id = 'repo_1'
             ORDER BY event_id
            """
        )
        return list(cur.fetchall())

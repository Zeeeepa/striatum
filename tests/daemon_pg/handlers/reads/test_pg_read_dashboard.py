# ruff: noqa
from __future__ import annotations

import os
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext
import pytest
from psycopg.types.json import Jsonb

from striatum.daemon_pg.handlers.reads import dashboard
from striatum.daemon_rpc.envelope import RpcError
from striatum.identity import process_start_time

from test_list_read_handlers import insert_fixture, insert_repo, pg_conn as pg_conn, repo_context


pytest_plugins = ("read_handler_fixtures",)
pytestmark = pytest.mark.multi_repo

def test_dashboard_requires_run_id(empty_ctx: RepoHandlerContext) -> None:
    with pytest.raises(RpcError) as exc:
        dashboard.handle(empty_ctx, {})
    assert exc.value.code == "schema_invalid"


def test_dashboard_payload_matches_terminal_dashboard_dto(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo = tmp_path / "repo-a"
    insert_repo(pg_conn, repo, "repo_a")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo)
    _seed_dashboard_cutover_rows(pg_conn)
    pg_conn.commit()

    payload = dashboard.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo),
        {"run_id": "run_1"},
    )

    assert payload["override_verdict_counts"] == {"accept_with_findings": 1}
    assert payload["override_verdicts"] == [
        {
            "previous_verdict": "needs_revision",
            "verdict": "accept_with_findings",
            "previous_verdict_id": "verdict_1",
            "verdict_id": "verdict_2",
            "rationale": "accepted risk",
            "event_id": 101,
            "created_at": payload["override_verdicts"][0]["created_at"],
        }
    ]
    blocker = payload["status"]["open_blockers"][0]
    assert blocker["created_at"]
    assert blocker["payload_json"] == {"envelope": {"process_id": "proc_1"}}
    session = payload["status"]["sessions"][0]
    assert session["lane_attestation"] == "attested"
    assert session["lane_attestation_reason"] is None
    assert session["pid"] == os.getpid()
    assert session["supervisor_id"] == "sup_1"


def _seed_dashboard_cutover_rows(conn: Any) -> None:
    now = datetime.now(UTC)
    pid = os.getpid()
    pid_start_time = process_start_time(pid)
    if pid_start_time is None:
        pytest.skip("process start-time token unavailable on this platform")
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.workflow_snapshots
            SET workflow_json = jsonb_set(
              workflow_json,
              '{lanes,codex,command}',
              %s::jsonb
            )
            WHERE repository_id = 'repo_a' AND workflow_snapshot_id = 'snap_1'
            """,
            (Jsonb(["agent", "--run"]),),
        )
        cur.execute(
            """
            INSERT INTO striatumd.process_supervisors(
              repository_id, supervisor_id, run_id, session_id, adapter,
              command_json, cwd, scratch_path, pid, pid_start_time, state,
              started_at, heartbeat_at
            )
            VALUES (
              'repo_a', 'sup_1', 'run_1', 'sess_1', 'process',
              %s, '/tmp', '/tmp/scratch', %s, %s, 'attached', %s, %s
            )
            """,
            (Jsonb(["agent", "--run"]), pid, pid_start_time, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions(
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, state, registered_at, last_heartbeat_at
            )
            VALUES (
              'repo_a', 'sess_2', 'run_1', 'reviewer', 'codex',
              'reviewer-codex-2', 2, '["review"]'::jsonb, 'active', %s, %s
            )
            """,
            (now, now),
        )
        cur.execute(
            """
            UPDATE striatumd.verdicts
            SET verdict = 'needs_revision', rationale = 'needs changes'
            WHERE repository_id = 'repo_a' AND verdict_id = 'verdict_1'
            """,
        )
        cur.execute(
            """
            INSERT INTO striatumd.verdicts(
              repository_id, verdict_id, run_id, job_id, session_id, verdict,
              rationale, created_at, posture
            )
            VALUES (
              'repo_a', 'verdict_2', 'run_1', 'job_review', 'sess_2',
              'accept_with_findings', 'accepted risk', %s, 'neutral'
            )
            """,
            (now,),
        )
        cur.execute(
            """
            INSERT INTO striatumd.events(
              repository_id, event_id, run_id, event_type, actor_session_id,
              job_id, payload_json, created_at
            )
            VALUES (
              'repo_a', 101, 'run_1', 'verdict.overridden', 'sess_1',
              'job_review', %s, %s
            )
            """,
            (
                Jsonb(
                    {
                        "previous_verdict": "needs_revision",
                        "verdict": "accept_with_findings",
                        "previous_verdict_id": "verdict_1",
                        "verdict_id": "verdict_2",
                    }
                ),
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.blockers(
              repository_id, blocker_id, run_id, job_id, session_id, severity,
              blocker_kind, description, state, created_at, payload_json
            )
            VALUES (
              'repo_a', 'blk_dashboard', 'run_1', 'job_review', 'sess_1',
              'blocked', 'process_outputs_missing', 'missing output',
              'open', %s, %s
            )
            """,
            (now, Jsonb({"envelope": {"process_id": "proc_1"}})),
        )

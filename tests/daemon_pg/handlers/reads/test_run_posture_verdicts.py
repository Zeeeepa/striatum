from __future__ import annotations

import importlib
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import pytest
from psycopg.types.json import Jsonb

from striatum.daemon_rpc.envelope import RpcError

from test_list_read_handlers import insert_fixture, insert_repo, pg_conn as pg_conn, repo_context

pytestmark = pytest.mark.multi_repo


def test_run_posture_verdicts_lists_matching_posture_and_scopes_repository(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo_a = tmp_path / "repo-a"
    repo_b = tmp_path / "repo-b"
    insert_repo(pg_conn, repo_a, "repo_a")
    insert_repo(pg_conn, repo_b, "repo_b")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo_a)
    insert_fixture(pg_conn, repository_id="repo_b", repo_root=repo_b)
    _set_verdict_posture(
        pg_conn,
        repository_id="repo_a",
        posture="devils_advocate",
        rationale="survived posture sweep",
    )
    _set_verdict_posture(
        pg_conn,
        repository_id="repo_b",
        posture="devils_advocate",
        rationale="wrong repository",
    )
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_posture_verdicts")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo_a),
        {"run_id": "run_1", "posture": "devils_advocate"},
    )

    assert result["run"]["run_id"] == "run_1"
    assert result["posture"] == "devils_advocate"
    assert result["count"] == 1
    verdict = result["verdicts"][0]
    assert verdict["verdict_id"] == "verdict_1"
    assert verdict["rationale"] == "survived posture sweep"
    assert verdict["workflow_job_id"] == "review"
    assert verdict["role_id"] == "reviewer"
    assert verdict["lane_id"] == "codex"
    assert verdict["session_slug"] == "author-codex-1"
    assert verdict["provenance"] == "natural"
    assert verdict["lane_attestation_chip"]["attested"] is False


def test_run_posture_verdicts_returns_empty_for_valid_run_without_matching_posture(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_posture_verdicts")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo),
        {"run_id": "run_1", "posture": "security"},
    )

    assert result["run"]["run_id"] == "run_1"
    assert result["verdicts"] == []
    assert result["count"] == 0


def test_run_posture_verdicts_reports_operator_override_provenance(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo)
    _set_verdict_posture(
        pg_conn,
        repository_id="repo_a",
        posture="security",
        rationale="operator accepted risk",
    )
    with pg_conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.events(
              repository_id, event_id, run_id, event_type, actor_session_id,
              payload_json, created_at
            )
            VALUES (
              'repo_a', 100, 'run_1', 'verdict.overridden', 'sess_1',
              %s, %s
            )
            """,
            (Jsonb({"verdict_id": "verdict_1"}), datetime.now(UTC)),
        )
    pg_conn.commit()
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_posture_verdicts")

    result = module.handle(
        repo_context(pg_conn, repository_id="repo_a", repo_root=repo),
        {"run_id": "run_1", "posture": "security"},
    )

    verdict = result["verdicts"][0]
    assert verdict["provenance"] == "operator-override"
    assert verdict["override_rationale"] == "operator accepted risk"


def test_run_posture_verdicts_rejects_missing_run_and_bad_posture(
    pg_conn: Any, tmp_path: Path
) -> None:
    repo = tmp_path / "repo"
    insert_repo(pg_conn, repo, "repo_a")
    insert_fixture(pg_conn, repository_id="repo_a", repo_root=repo)
    module = importlib.import_module("striatum.daemon_pg.handlers.reads.run_posture_verdicts")
    ctx = repo_context(pg_conn, repository_id="repo_a", repo_root=repo)

    with pytest.raises(RpcError, match="run not found"):
        module.handle(ctx, {"run_id": "run_missing", "posture": "security"})
    with pytest.raises(RpcError, match="posture contains invalid"):
        module.handle(ctx, {"run_id": "run_1", "posture": "../security"})
    with pytest.raises(RpcError, match="posture must be a non-empty string"):
        module.handle(ctx, {"run_id": "run_1"})


def _set_verdict_posture(
    conn: Any,
    *,
    repository_id: str,
    posture: str,
    rationale: str,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.verdicts
            SET posture = %s, rationale = %s
            WHERE repository_id = %s AND verdict_id = 'verdict_1'
            """,
            (posture, rationale, repository_id),
        )
    conn.commit()

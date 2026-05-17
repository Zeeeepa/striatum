from __future__ import annotations

import subprocess
from collections.abc import Iterator, Mapping
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any

import pytest
from psycopg.rows import dict_row
from psycopg.types.json import Jsonb

import striatum.daemon_pg.handlers.worktree as worktree_handler
from _harness.pg import create_ephemeral_database, drop_ephemeral_database
from striatum.daemon_pg.connection import connect
from striatum.daemon_pg.handlers.context import RepoHandlerContext
from striatum.daemon_pg.handlers.registry import resolve_pg_handler
from striatum.daemon_rpc.capability import RpcAuthContext
from striatum.errors import InvalidTransitionError, LeaseError, NotFoundError

pytestmark = pytest.mark.multi_repo


@pytest.fixture
def pg_url(postgres_url: str) -> Iterator[str]:
    ephemeral = create_ephemeral_database(postgres_url)
    try:
        yield ephemeral.database_url
    finally:
        drop_ephemeral_database(ephemeral)


def test_worktree_handlers_register_on_import() -> None:
    assert resolve_pg_handler("worktree.create") is worktree_handler.handle_create
    assert resolve_pg_handler("worktree.release") is worktree_handler.handle_release
    assert resolve_pg_handler("worktree.list") is worktree_handler.handle_list
    assert getattr(resolve_pg_handler("worktree.list"), "read_only", False) is True


def test_create_list_release_scope_to_repository(tmp_path: Path, pg_url: str) -> None:
    conn = connect(pg_url)
    try:
        repo_a = tmp_path / "repo_a"
        repo_b = tmp_path / "repo_b"
        _init_git_repo(repo_a)
        _init_git_repo(repo_b)
        _seed_claimed_work(conn, repo_a, repository_id="repo_a")
        _seed_claimed_work(conn, repo_b, repository_id="repo_b")
        conn.commit()

        created = worktree_handler.handle_create(
            _ctx(conn, repo_a, repository_id="repo_a"),
            {"session_id": "sess_1", "job_id": "job_1", "lease_id": "lease_1"},
        )

        worktree_id = str(created["worktree_id"])
        assert created["worktree_path"] == f".striatum/worktrees/{worktree_id}"
        assert created["base_branch"] == "main"
        target = repo_a / str(created["worktree_path"])
        assert target.is_dir()
        assert (target / ".gitseed").exists()

        _insert_worktree_row(
            conn,
            repository_id="repo_b",
            worktree_id=worktree_id,
            worktree_path=f".striatum/worktrees/{worktree_id}",
        )
        conn.commit()

        listed = worktree_handler.handle_list(
            _ctx(conn, repo_a, repository_id="repo_a"), {"run_id": "run_1"}
        )
        assert listed["worktrees"] == [
            {
                "worktree_id": worktree_id,
                "run_id": "run_1",
                "job_id": "job_1",
                "lease_id": "lease_1",
                "base_branch": "main",
                "worktree_path": f".striatum/worktrees/{worktree_id}",
                "state": "active",
                "created_at": listed["worktrees"][0]["created_at"],
                "released_at": None,
                "removed_at": None,
                "workflow_job_id": "draft",
            }
        ]

        released = worktree_handler.handle_release(
            _ctx(conn, repo_a, repository_id="repo_a"), {"worktree_id": worktree_id}
        )
        assert released == {
            "status": "released",
            "worktree_id": worktree_id,
            "state": "removed",
        }
        assert not target.exists()

        again = worktree_handler.handle_release(
            _ctx(conn, repo_a, repository_id="repo_a"), {"worktree_id": worktree_id}
        )
        assert again == {
            "status": "already_released",
            "worktree_id": worktree_id,
            "state": "removed",
        }

        assert _worktree_state(conn, repository_id="repo_b", worktree_id=worktree_id) == "active"
        assert [row["event_type"] for row in _events(conn, "repo_a")] == [
            "worktree.created",
            "worktree.released",
        ]
        assert _events(conn, "repo_b") == []
    finally:
        conn.close()


def test_create_rejects_wrong_session_and_missing_lease(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo = tmp_path / "repo"
        _init_git_repo(repo)
        _seed_claimed_work(conn, repo, repository_id="repo_a")
        _insert_second_session(conn, repository_id="repo_a")
        conn.commit()

        with pytest.raises(LeaseError, match="owned by another session"):
            worktree_handler.handle_create(
                _ctx(conn, repo, repository_id="repo_a"),
                {
                    "session_id": "sess_other",
                    "job_id": "job_1",
                    "lease_id": "lease_1",
                },
            )

        with pytest.raises(NotFoundError, match="missing_lease"):
            worktree_handler.handle_create(
                _ctx(conn, repo, repository_id="repo_a"),
                {
                    "session_id": "sess_1",
                    "job_id": "job_1",
                    "lease_id": "missing_lease",
                },
            )
        assert _count_worktrees(conn, repository_id="repo_a") == 0
    finally:
        conn.close()


def test_create_rejects_non_isolated_or_non_repo_write_job(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo = tmp_path / "repo"
        _init_git_repo(repo)
        _seed_claimed_work(conn, repo, repository_id="repo_a", isolated=False)
        conn.commit()

        with pytest.raises(InvalidTransitionError, match="worktree_isolation"):
            worktree_handler.handle_create(
                _ctx(conn, repo, repository_id="repo_a"),
                {"session_id": "sess_1", "job_id": "job_1", "lease_id": "lease_1"},
            )

        _replace_work_scope(conn, repository_id="repo_a", repo_write=False)
        conn.commit()
        with pytest.raises(InvalidTransitionError, match="repo-write job"):
            worktree_handler.handle_create(
                _ctx(conn, repo, repository_id="repo_a"),
                {"session_id": "sess_1", "job_id": "job_1", "lease_id": "lease_1"},
            )
    finally:
        conn.close()


def test_release_rejects_worktree_path_outside_state_worktrees(
    tmp_path: Path, pg_url: str
) -> None:
    conn = connect(pg_url)
    try:
        repo = tmp_path / "repo"
        _init_git_repo(repo)
        _seed_claimed_work(conn, repo, repository_id="repo_a")
        _insert_worktree_row(
            conn,
            repository_id="repo_a",
            worktree_id="wt_bad",
            worktree_path="docs/not-a-worktree",
        )
        conn.commit()

        with pytest.raises(InvalidTransitionError, match=".striatum/worktrees"):
            worktree_handler.handle_release(
                _ctx(conn, repo, repository_id="repo_a"), {"worktree_id": "wt_bad"}
            )
        assert _worktree_state(conn, repository_id="repo_a", worktree_id="wt_bad") == "active"
    finally:
        conn.close()


def _init_git_repo(repo: Path) -> None:
    repo.mkdir(parents=True)
    subprocess.run(["git", "init"], cwd=repo, check=True, capture_output=True)
    subprocess.run(["git", "checkout", "-b", "main"], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "config", "user.email", "test@example.com"],
        cwd=repo,
        check=True,
        capture_output=True,
    )
    subprocess.run(
        ["git", "config", "user.name", "Test"],
        cwd=repo,
        check=True,
        capture_output=True,
    )
    (repo / ".gitseed").write_text("seed\n", encoding="utf-8")
    subprocess.run(["git", "add", ".gitseed"], cwd=repo, check=True, capture_output=True)
    subprocess.run(
        ["git", "commit", "-m", "seed", "--no-gpg-sign"],
        cwd=repo,
        check=True,
        capture_output=True,
    )


def _ctx(conn: Any, repo_root: Path, *, repository_id: str) -> RepoHandlerContext:
    return RepoHandlerContext(
        pg_conn=conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=RpcAuthContext("client", "token", repository_id, "write", "allowed"),
    )


def _seed_claimed_work(
    conn: Any,
    repo_root: Path,
    *,
    repository_id: str,
    isolated: bool = True,
) -> None:
    now = datetime.now(UTC)
    workflow = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "wf",
        "workflow_version": "1",
        "name": "PG worktree test",
        "branch": {"mode": "manual"},
        "coordinator": {"role_id": "author", "lane_id": "local"},
        "lanes": {
            "local": {
                "adapter": "manual",
                "worktree_isolation": "per_job" if isolated else "off",
            }
        },
        "roles": {"author": {"summary": "Author"}},
        "context_docs": [],
        "parallelism": {"mode": "serial", "max_active_jobs": 1},
        "jobs": [{"id": "draft", "type": "draft", "role": "author", "lane_id": "local"}],
        "edges": [],
        "cycles": [],
    }
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path, display_name,
              registered_at, last_seen_at, last_schema_version, state, settings_json
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, 5, 'active', '{}'::jsonb)
            """,
            (
                repository_id,
                repository_id,
                str(repo_root.resolve()),
                str((repo_root / ".striatum" / "state.sqlite3").resolve()),
                repository_id,
                now,
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots(
              repository_id, workflow_snapshot_id, workflow_id, workflow_version,
              source_path, content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, 'snap_1', 'wf', '1', 'workflow.json', 'sha', %s, %s)
            """,
            (repository_id, Jsonb(workflow), now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs(
              repository_id, run_id, workflow_snapshot_id, repo_root, state,
              branch_name, branch_base, branch_confirmed_at, branch_confirmed_by,
              created_at, started_at
            )
            VALUES (%s, 'run_1', 'snap_1', %s, 'running', 'main', 'main',
                    %s, 'operator', %s, %s)
            """,
            (repository_id, str(repo_root.resolve()), now, now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.sessions(
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, first_class, fresh_context, state, registered_at,
              last_heartbeat_at
            )
            VALUES (%s, 'sess_1', 'run_1', 'author', 'local', 'author-local-1',
                    1, %s, true, false, 'active', %s, %s)
            """,
            (repository_id, Jsonb(["write"]), now, now),
        )
        cur.execute(
            """
            INSERT INTO striatumd.jobs(
              repository_id, job_id, run_id, workflow_job_id, title, job_type,
              role_id, lane_selector_json, capability_requirements_json, state,
              attempt, max_attempts, fresh_session_required, write_scope_json,
              expected_artifacts_json, idempotency_key, created_at, started_at,
              current_lease_id
            )
            VALUES (%s, 'job_1', 'run_1', 'draft', 'Draft', 'draft', 'author',
                    %s, %s, 'running', 1, 1, false, %s, %s, 'draft:1',
                    %s, %s, 'lease_1')
            """,
            (
                repository_id,
                Jsonb({"lane_id": "local"}),
                Jsonb([]),
                Jsonb(
                    {
                        "mode": "repo_write",
                        "repo_write": True,
                        "allowed_paths": ["docs"],
                        "forbidden_paths": [],
                    }
                ),
                Jsonb([]),
                now,
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.leases(
              repository_id, lease_id, run_id, resource_type, resource_id,
              owner_session_id, state, acquired_at, expires_at, last_heartbeat_at
            )
            VALUES (%s, 'lease_1', 'run_1', 'job', 'job_1', 'sess_1',
                    'active', %s, %s, %s)
            """,
            (repository_id, now, now + timedelta(minutes=15), now),
        )


def _insert_second_session(conn: Any, *, repository_id: str) -> None:
    now = datetime.now(UTC)
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.sessions(
              repository_id, session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, first_class, fresh_context, state, registered_at
            )
            VALUES (%s, 'sess_other', 'run_1', 'author', 'local',
                    'author-local-2', 2, %s, true, false, 'active', %s)
            """,
            (repository_id, Jsonb(["write"]), now),
        )


def _replace_work_scope(conn: Any, *, repository_id: str, repo_write: bool) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE striatumd.jobs
            SET write_scope_json = %s
            WHERE repository_id = %s AND job_id = 'job_1'
            """,
            (Jsonb({"repo_write": repo_write}), repository_id),
        )


def _insert_worktree_row(
    conn: Any,
    *,
    repository_id: str,
    worktree_id: str,
    worktree_path: str,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.job_worktrees(
              repository_id, worktree_id, run_id, job_id, lease_id, base_branch,
              worktree_path, state, created_at
            )
            VALUES (%s, %s, 'run_1', 'job_1', 'lease_1', 'main', %s,
                    'active', %s)
            """,
            (repository_id, worktree_id, worktree_path, datetime.now(UTC)),
        )


def _worktree_state(conn: Any, *, repository_id: str, worktree_id: str) -> str:
    row = _one(
        conn,
        """
        SELECT state
        FROM striatumd.job_worktrees
        WHERE repository_id = %s AND worktree_id = %s
        """,
        (repository_id, worktree_id),
    )
    return str(row["state"])


def _count_worktrees(conn: Any, *, repository_id: str) -> int:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT COUNT(*) FROM striatumd.job_worktrees WHERE repository_id = %s",
            (repository_id,),
        )
        return int(cur.fetchone()[0])


def _events(conn: Any, repository_id: str) -> list[Mapping[str, Any]]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            SELECT event_type, payload_json
            FROM striatumd.events
            WHERE repository_id = %s
            ORDER BY event_id
            """,
            (repository_id,),
        )
        return list(cur.fetchall())


def _one(conn: Any, sql: str, args: tuple[object, ...]) -> dict[str, Any]:
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(sql, args)
        row = cur.fetchone()
    assert row is not None
    return dict(row)

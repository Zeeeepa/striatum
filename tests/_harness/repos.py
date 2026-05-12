"""Repository helpers for multi-repo tests."""

from __future__ import annotations

import shutil
import sqlite3
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping

from striatum import daemon
from striatum.db import db_path, init_repo, json_dumps, new_id, sha256_bytes, utc_now
from striatum.workflow import validate_workflow


@dataclass
class RepoDescriptor:
    alias: str
    path: Path
    repository_id: str | None = None


class RepoLocalRunner:
    def __init__(self, repos_by_id: Mapping[str, RepoDescriptor], workflow: Mapping[str, Any] | None = None) -> None:
        self.repos_by_id = repos_by_id
        self.workflow = workflow
        self.fail_prepare_alias: str | None = None
        self.fail_start_alias: str | None = None
        self.fail_cancel_alias: str | None = None
        self.unreachable_ids: set[str] = set()
        self.checkpoints: list[dict[str, str | None]] = []

    def prepare(self, *, repository_id: str, repository_alias: str, cross_repo_run_id: str) -> str:
        if repository_alias == self.fail_prepare_alias or repository_id in self.unreachable_ids:
            raise RuntimeError(f"repository {repository_alias} unavailable")
        repo = self.repos_by_id[repository_id]
        workflow = dict(self.workflow or {})
        return _insert_local_run(repo.path, workflow=workflow, cross_repo_run_id=cross_repo_run_id)

    def start(self, *, repository_id: str, local_run_id: str) -> None:
        if repository_id in self.unreachable_ids:
            raise RuntimeError("repository unavailable")
        alias = _alias_for_id(self.repos_by_id, repository_id)
        if alias == self.fail_start_alias:
            raise RuntimeError("repository unavailable")
        with sqlite3.connect(db_path(self.repos_by_id[repository_id].path)) as conn:
            conn.execute("UPDATE runs SET state = 'running', started_at = ? WHERE run_id = ?", (utc_now(), local_run_id))
            conn.commit()

    def cancel(self, *, repository_id: str, local_run_id: str, reason: str) -> None:
        if repository_id in self.unreachable_ids:
            raise RuntimeError("repository unavailable")
        alias = _alias_for_id(self.repos_by_id, repository_id)
        if alias == self.fail_cancel_alias:
            raise RuntimeError("repository unavailable")
        with sqlite3.connect(db_path(self.repos_by_id[repository_id].path)) as conn:
            conn.execute(
                "UPDATE runs SET state = 'canceled', completed_at = ?, stop_reason = ? WHERE run_id = ?",
                (utc_now(), reason, local_run_id),
            )
            conn.commit()

    def participant_intact(self, *, repository_id: str, local_run_id: str | None) -> bool:
        if local_run_id is None or repository_id in self.unreachable_ids:
            return False
        with sqlite3.connect(db_path(self.repos_by_id[repository_id].path)) as conn:
            return conn.execute("SELECT 1 FROM runs WHERE run_id = ?", (local_run_id,)).fetchone() is not None

    def human_checkpoint(self, *, repository_id: str, local_run_id: str | None, reason: str) -> None:
        self.checkpoints.append({"repository_id": repository_id, "local_run_id": local_run_id, "reason": reason})
        if local_run_id is None or repository_id not in self.repos_by_id:
            return
        with sqlite3.connect(db_path(self.repos_by_id[repository_id].path)) as conn:
            blocker_id = new_id("blk")
            conn.execute(
                """
                INSERT INTO blockers(
                  blocker_id, run_id, job_id, session_id, severity, blocker_kind,
                  description, state, created_at, payload_json
                )
                VALUES (?, ?, NULL, NULL, 'human_checkpoint', 'human_checkpoint', ?, 'open', ?, '{}')
                """,
                (blocker_id, local_run_id, reason, utc_now()),
            )
            conn.commit()


def init_repositories(scratch_dir: Path, repo_count: int) -> list[RepoDescriptor]:
    repos: list[RepoDescriptor] = []
    for index in range(repo_count):
        path = scratch_dir / f"repo-{index}"
        path.mkdir(parents=True, exist_ok=True)
        init_repo(path)
        repos.append(RepoDescriptor(alias=f"repo{index}", path=path))
    return repos


def register_repo(conn: Any, repo: RepoDescriptor) -> str:
    repository_id = new_id("repo")
    with sqlite3.connect(db_path(repo.path)) as repo_conn:
        version = int(repo_conn.execute("PRAGMA user_version").fetchone()[0])
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.repositories(
              repository_id, repo_identity, repo_root, state_db_path, display_name,
              registered_at, last_seen_at, last_schema_version, state, settings_json
            )
            VALUES (%s, %s, %s, %s, %s, now(), now(), %s, 'active', '{}'::jsonb)
            ON CONFLICT (repo_identity) WHERE state IN ('active','missing','disabled')
            DO UPDATE SET last_seen_at = EXCLUDED.last_seen_at
            RETURNING repository_id
            """,
            (
                repository_id,
                daemon._repo_identity(repo.path),  # test setup uses the production identity helper.
                str(repo.path.resolve()),
                str(db_path(repo.path).resolve()),
                repo.path.name,
                version,
            ),
        )
        row = cur.fetchone()
    conn.commit()
    repo.repository_id = str(row[0])
    return repo.repository_id


def clear_repo_local(repo: RepoDescriptor) -> None:
    preserved = {"schema_meta"}
    conn = sqlite3.connect(db_path(repo.path))
    try:
        rows = conn.execute("SELECT name FROM sqlite_master WHERE type = 'table'").fetchall()
        conn.execute("PRAGMA foreign_keys = OFF")
        for (name,) in rows:
            if str(name).startswith("sqlite_") or str(name) in preserved:
                continue
            conn.execute(f'DELETE FROM "{name}"')
        conn.execute("PRAGMA foreign_keys = ON")
        conn.commit()
    finally:
        conn.close()
    for child in (repo.path / "docs").glob("**/*") if (repo.path / "docs").exists() else []:
        if child.is_file():
            child.unlink()
    docs = repo.path / "docs"
    if docs.exists():
        shutil.rmtree(docs)


def two_repo_workflow(repo_a: str, repo_b: str) -> dict[str, Any]:
    workflow: dict[str, Any] = {
        "schema_version": "striatum.workflow.v1",
        "workflow_id": "multi-repo-e2e",
        "workflow_version": "1",
        "name": "Multi repo e2e",
        "branch": {"mode": "auto", "suggested_name": "test/multi-repo"},
        "coordinator": {"role_id": "coordinator", "lane_id": "codex"},
        "repositories": {"primary": {"repo_id": repo_a}, "consumer": {"repo_id": repo_b}},
        "primary_repository": "primary",
        "lanes": {"codex": {"adapter": "manual"}},
        "roles": {"coordinator": {}, "author": {}, "reviewer": {}},
        "context_docs": [],
        "parallelism": {
            "mode": "declared",
            "max_active_jobs": 2,
            "require_disjoint_write_scopes": True,
            "per_repo_max_active_jobs": {"primary": 1, "consumer": 1},
        },
        "jobs": [
            _job("draft_primary", "primary", "docs/primary.md"),
            _job("draft_consumer", "consumer", "docs/consumer.md"),
        ],
        "edges": [{"from": "draft_primary", "to": "draft_consumer", "on": "completed"}],
        "cycles": [],
    }
    validate_workflow(workflow)
    return workflow


def cycle_workflow(repo_a: str, repo_b: str) -> dict[str, Any]:
    workflow = two_repo_workflow(repo_a, repo_b)
    workflow["edges"] = [{"from": "draft_primary", "to": "draft_consumer", "on": "completed"}]
    workflow["cycles"] = [
        {
            "from": "draft_consumer",
            "to": "draft_primary",
            "on_verdict": "needs_revision",
            "max_iterations": 2,
            "cross_repo_cycle": True,
        }
    ]
    validate_workflow(workflow)
    return workflow


def _job(job_id: str, repository: str, path: str) -> dict[str, Any]:
    return {
        "id": job_id,
        "type": "draft",
        "repository": repository,
        "role_id": "author",
        "lane_id": "codex",
        "write_scope": {
            "mode": "repo_write",
            "repo_write": True,
            "allowed_paths": ["docs/"],
            "forbidden_paths": [],
        },
        "expected_artifacts": [{"logical_name": "out", "kind": "handoff", "path": path}],
    }


def _insert_local_run(repo: Path, *, workflow: dict[str, Any], cross_repo_run_id: str) -> str:
    run_id = new_id("run")
    snapshot_id = new_id("wfs")
    payload = json_dumps(workflow)
    conn = sqlite3.connect(db_path(repo))
    try:
        now = utc_now()
        conn.execute(
            """
            INSERT INTO workflow_snapshots(
              workflow_snapshot_id, workflow_id, workflow_version, source_path,
              content_sha256, workflow_json, loaded_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?)
            """,
            (
                snapshot_id,
                str(workflow.get("workflow_id", "multi-repo-e2e")),
                workflow.get("workflow_version"),
                "<multi-repo-harness>",
                sha256_bytes(payload.encode("utf-8")),
                payload,
                now,
            ),
        )
        conn.execute(
            """
            INSERT INTO runs(
              run_id, workflow_snapshot_id, repo_root, state, branch_name,
              branch_base, created_at, cross_repo_run_id
            )
            VALUES (?, ?, ?, 'ready', ?, NULL, ?, ?)
            """,
            (run_id, snapshot_id, str(repo), "test/multi-repo", now, cross_repo_run_id),
        )
        conn.commit()
    finally:
        conn.close()
    return run_id


def _alias_for_id(repos_by_id: Mapping[str, RepoDescriptor], repository_id: str) -> str:
    return repos_by_id[repository_id].alias

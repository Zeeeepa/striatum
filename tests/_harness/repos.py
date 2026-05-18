"""Repository helpers for multi-repo tests."""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping

from striatum.daemon_pg.repositories import repo_add_pg
from striatum.primitives import json_dumps, new_id, sha256_bytes, utc_now
from striatum.workflow import validate_workflow


@dataclass
class RepoDescriptor:
    alias: str
    path: Path
    repository_id: str | None = None


class PgParticipantRunner:
    def __init__(
        self,
        repos_by_id: Mapping[str, RepoDescriptor],
        pg_conn_factory: Callable[[], Any],
        workflow: Mapping[str, Any] | None = None,
    ) -> None:
        self.repos_by_id = repos_by_id
        self.pg_conn_factory = pg_conn_factory
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
        conn = self.pg_conn_factory()
        try:
            run_id = _insert_pg_run(
                conn,
                repo,
                repository_id=repository_id,
                workflow=workflow,
                cross_repo_run_id=cross_repo_run_id,
            )
            conn.commit()
            return run_id
        finally:
            conn.close()

    def _matches_fail_alias(self, repository_id: str, fail_alias: str | None) -> bool:
        """Return True if `fail_alias` matches `repository_id` under either
        the test-side alias (RepoDescriptor.alias — repo0/repo1) or the
        workflow-side alias (workflow.repositories[alias].repo_id — e.g.
        "primary"/"consumer"). Tests that set fail_alias=workflow-alias
        worked before the runner was extended to use _alias_for_id, which
        only returned the test-side label. This helper accepts both.
        """
        if fail_alias is None:
            return False
        if _alias_for_id(self.repos_by_id, repository_id) == fail_alias:
            return True
        workflow_repos = (self.workflow or {}).get("repositories")
        if isinstance(workflow_repos, Mapping):
            for wf_alias, entry in workflow_repos.items():
                if not isinstance(entry, Mapping):
                    continue
                if str(entry.get("repo_id")) == repository_id and wf_alias == fail_alias:
                    return True
        return False

    def start(self, *, repository_id: str, local_run_id: str) -> None:
        if repository_id in self.unreachable_ids:
            raise RuntimeError("repository unavailable")
        if self._matches_fail_alias(repository_id, self.fail_start_alias):
            raise RuntimeError("repository unavailable")
        conn = self.pg_conn_factory()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    UPDATE striatumd.runs
                    SET state = 'running', started_at = %s
                    WHERE repository_id = %s AND run_id = %s
                    """,
                    (utc_now(), repository_id, local_run_id),
                )
            conn.commit()
        finally:
            conn.close()

    def cancel(self, *, repository_id: str, local_run_id: str, reason: str) -> None:
        if repository_id in self.unreachable_ids:
            raise RuntimeError("repository unavailable")
        if self._matches_fail_alias(repository_id, self.fail_cancel_alias):
            raise RuntimeError("repository unavailable")
        conn = self.pg_conn_factory()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    UPDATE striatumd.runs
                    SET state = 'canceled', completed_at = %s, stop_reason = %s
                    WHERE repository_id = %s AND run_id = %s
                    """,
                    (utc_now(), reason, repository_id, local_run_id),
                )
            conn.commit()
        finally:
            conn.close()

    def participant_intact(self, *, repository_id: str, local_run_id: str | None) -> bool:
        if local_run_id is None or repository_id in self.unreachable_ids:
            return False
        conn = self.pg_conn_factory()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT 1
                    FROM striatumd.runs
                    WHERE repository_id = %s AND run_id = %s
                    """,
                    (repository_id, local_run_id),
                )
                return cur.fetchone() is not None
        finally:
            conn.close()

    def human_checkpoint(self, *, repository_id: str, local_run_id: str | None, reason: str) -> None:
        self.checkpoints.append({"repository_id": repository_id, "local_run_id": local_run_id, "reason": reason})
        if local_run_id is None or repository_id not in self.repos_by_id:
            return
        conn = self.pg_conn_factory()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO striatumd.blockers(
                      repository_id, blocker_id, run_id, job_id, session_id,
                      severity, blocker_kind, description, state, created_at,
                      payload_json
                    )
                    VALUES (
                      %s, %s, %s, NULL, NULL, 'human_checkpoint',
                      'human_checkpoint', %s, 'open', %s, '{}'::jsonb
                    )
                    """,
                    (repository_id, new_id("blk"), local_run_id, reason, utc_now()),
                )
            conn.commit()
        finally:
            conn.close()


def init_repositories(scratch_dir: Path, repo_count: int) -> list[RepoDescriptor]:
    repos: list[RepoDescriptor] = []
    for index in range(repo_count):
        path = scratch_dir / f"repo-{index}"
        path.mkdir(parents=True, exist_ok=True)
        repos.append(RepoDescriptor(alias=f"repo{index}", path=path))
    return repos


def register_repo(conn: Any, repo: RepoDescriptor) -> str:
    result = repo_add_pg(conn, repo.path, display_name=repo.path.name, init=True)
    repo.repository_id = str(result["repository_id"])
    return repo.repository_id


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


def _insert_pg_run(
    conn: Any,
    repo: RepoDescriptor,
    *,
    repository_id: str,
    workflow: dict[str, Any],
    cross_repo_run_id: str,
) -> str:
    run_id = new_id("run")
    snapshot_id = new_id("wfs")
    payload = json_dumps(workflow)
    now = utc_now()
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO striatumd.workflow_snapshots(
              repository_id, workflow_snapshot_id, workflow_id, workflow_version, source_path,
              content_sha256, workflow_json, loaded_at
            )
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
            """,
            (
                repository_id,
                snapshot_id,
                str(workflow.get("workflow_id", "multi-repo-e2e")),
                workflow.get("workflow_version"),
                "<multi-repo-harness>",
                sha256_bytes(payload.encode("utf-8")),
                _jsonb_payload(workflow),
                now,
            ),
        )
        cur.execute(
            """
            INSERT INTO striatumd.runs(
              repository_id, run_id, workflow_snapshot_id, repo_root, state, branch_name,
              branch_base, created_at, cross_repo_run_id
            )
            VALUES (%s, %s, %s, %s, 'ready', %s, NULL, %s, %s)
            """,
            (
                repository_id,
                run_id,
                snapshot_id,
                str(repo.path.resolve()),
                "test/multi-repo",
                now,
                cross_repo_run_id,
            ),
        )
    return run_id


def _alias_for_id(repos_by_id: Mapping[str, RepoDescriptor], repository_id: str) -> str:
    return repos_by_id[repository_id].alias


def _jsonb_payload(value: Mapping[str, Any]) -> Any:
    from psycopg.types.json import Jsonb

    return Jsonb(dict(value))

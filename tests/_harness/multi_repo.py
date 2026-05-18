"""Composable multi-repository daemon harness for RFC 0035 tests."""

from __future__ import annotations

import shutil
import signal
from contextlib import contextmanager
from pathlib import Path
from typing import Any, Iterator, Sequence

from striatum.cross_repo import (
    cancel_cross_repo_run,
    describe_cross_repo_run,
    prepare_cross_repo_run,
    reconcile_cross_repo_preparing,
    start_cross_repo_run,
)
from striatum.daemon_pg.connection import connect

from _harness import audit, pg, repos, tokens
from _harness.daemon import DaemonCore, DaemonProcess, PauseHook
from _harness.mcp import McpClient
from _harness.repos import RepoDescriptor, RepoLocalRunner


class MultiRepoHarness:
    def __init__(
        self,
        daemon_pg_url: str,
        repo_count: int = 2,
        scratch_dir: Path | None = None,
        *,
        daemon_core: DaemonCore = "go",
    ) -> None:
        self._base_pg_url = daemon_pg_url
        self._scratch_root = Path(scratch_dir) if scratch_dir is not None else Path.cwd() / ".tmp-multi-repo"
        self._ephemeral: pg.EphemeralPostgres | None = None
        self.daemon_pg_url = daemon_pg_url
        self.repos: list[RepoDescriptor] = []
        self.daemon: DaemonProcess | None = None
        self.admin_token: str | None = None
        self._repo_count = repo_count
        self._daemon_core: DaemonCore = daemon_core
        self._running = False
        self.local_runner: RepoLocalRunner | None = None

    @property
    def daemon_core(self) -> DaemonCore:
        return self._daemon_core

    @property
    def socket_path(self) -> Path:
        if self.daemon is None:
            return self._scratch_root / "runtime" / "striatumd.sock"
        return self.daemon.socket_path

    def start(self) -> None:
        if self._running:
            raise RuntimeError("multi-repo harness is already running")
        self._scratch_root.mkdir(parents=True, exist_ok=True)
        self._ephemeral = pg.create_ephemeral_database(self._base_pg_url)
        self.daemon_pg_url = self._ephemeral.database_url
        self.daemon = DaemonProcess(
            scratch_dir=self._scratch_root,
            postgres_url=self.daemon_pg_url,
            daemon_core=self._daemon_core,
        )
        self.daemon.start()
        self.repos = repos.init_repositories(self._scratch_root, self._repo_count)
        self.register_all()
        self._running = True

    def stop(self) -> None:
        if self.daemon is not None:
            self.daemon.stop()
        ephemeral = self._ephemeral
        if ephemeral is not None:
            pg.drop_ephemeral_database(ephemeral)
        shutil.rmtree(self._scratch_root, ignore_errors=True)
        self._running = False
        self._ephemeral = None
        self.daemon = None

    def reset_daemon_db(self) -> None:
        conn = self.pg_conn()
        try:
            pg.reset_daemon_db(conn)
        finally:
            conn.close()

    def reset_repo_local(self, repo_index: int) -> None:
        repos.clear_repo_local(self.repos[repo_index])

    def register_all(self) -> list[str]:
        conn = self.pg_conn()
        try:
            ids = [repos.register_repo(conn, repo) for repo in self.repos]
        finally:
            conn.close()
        return ids

    def pg_conn(self) -> Any:
        conn = connect(self.daemon_pg_url)
        # Production daemon runs autocommit=True (see daemon.py daemon-start)
        # so audit rows commit promptly across the in-process McpClient /
        # DaemonRpcServer flow (RFC 0048 V1.5 F3). Keep row_factory at the
        # psycopg default (tuple_row) here — many test helpers do positional
        # `row[0]` access; only the dispatch-side cursors that need mapping
        # rows opt-in via per-cursor `row_factory=dict_row`.
        conn.autocommit = True
        return conn

    def issue_token(self, capabilities: list[str], repo_id: str | None = None, expires_in: int | None = 3600) -> str:
        conn = self.pg_conn()
        try:
            return tokens.issue_token(conn, capabilities=capabilities, repo_id=repo_id, expires_in=expires_in)
        finally:
            conn.close()

    def revoke_token(self, token: str) -> None:
        conn = self.pg_conn()
        try:
            tokens.revoke_token(conn, token)
        finally:
            conn.close()

    def expire_token(self, token: str) -> None:
        conn = self.pg_conn()
        try:
            tokens.expire_token(conn, token)
        finally:
            conn.close()

    def mcp_client(self, token: str, repo_index: int | None = None) -> McpClient:
        repo_root = self.repos[repo_index].path if repo_index is not None else None
        return McpClient(pg_conn=self.pg_conn(), token=token, repo_root=repo_root)

    def audit_rows(self, *, transport: str | None = None) -> list[dict[str, object]]:
        conn = self.pg_conn()
        try:
            return audit.audit_rows(conn, transport=transport)
        finally:
            conn.close()

    def assert_audit_chain(self) -> None:
        conn = self.pg_conn()
        try:
            audit.assert_audit_chain(conn)
        finally:
            conn.close()

    def daemon_db_query(self, sql: str, args: Sequence[object] = ()) -> list[dict[str, object]]:
        conn = self.pg_conn()
        try:
            with conn.cursor() as cur:
                cur.execute(sql, tuple(args))
                if cur.description is None:
                    conn.commit()
                    return []
                rows = cur.fetchall()
                columns = [desc[0] for desc in cur.description]
            return [dict(zip(columns, row, strict=True)) for row in rows]
        finally:
            conn.close()

    def repo_sqlite_query(self, repo_index: int, sql: str, args: Sequence[object] = ()) -> list[dict[str, object]]:
        import sqlite3

        from striatum.db import db_path

        conn = sqlite3.connect(db_path(self.repos[repo_index].path))
        conn.row_factory = sqlite3.Row
        try:
            return [dict(row) for row in conn.execute(sql, tuple(args)).fetchall()]
        finally:
            conn.close()

    def prepare_cross_repo_run(self, workflow: dict[str, object]) -> dict[str, Any]:
        runner = self._runner(workflow)
        conn = self.pg_conn()
        try:
            result = prepare_cross_repo_run(conn, workflow=workflow, local_runner=runner)
            conn.commit()
            return result
        except Exception:
            conn.commit()
            raise
        finally:
            conn.close()

    def start_cross_repo_run(self, cross_repo_run_id: str) -> dict[str, Any]:
        conn = self.pg_conn()
        try:
            result = start_cross_repo_run(conn, cross_repo_run_id=cross_repo_run_id, local_runner=self._runner())
            conn.commit()
            return result
        finally:
            conn.close()

    def cancel_cross_repo_run(self, cross_repo_run_id: str) -> dict[str, Any]:
        conn = self.pg_conn()
        try:
            result = cancel_cross_repo_run(
                conn,
                cross_repo_run_id=cross_repo_run_id,
                local_runner=self._runner(),
                reason="test cancel",
            )
            conn.commit()
            return result
        finally:
            conn.close()

    def describe_cross_repo_run(self, cross_repo_run_id: str) -> dict[str, Any]:
        conn = self.pg_conn()
        try:
            return describe_cross_repo_run(conn, cross_repo_run_id=cross_repo_run_id)
        finally:
            conn.close()

    def reconcile_preparing(self) -> dict[str, Any]:
        conn = self.pg_conn()
        try:
            result = reconcile_cross_repo_preparing(conn, local_runner=self._runner())
            conn.commit()
            return result
        finally:
            conn.close()

    def kill_daemon(self, sig: signal.Signals = signal.SIGKILL) -> None:
        if self.daemon is not None:
            self.daemon.kill(sig)

    def restart_daemon(self) -> None:
        if self.daemon is None:
            self.daemon = DaemonProcess(
                scratch_dir=self._scratch_root,
                postgres_url=self.daemon_pg_url,
                daemon_core=self._daemon_core,
            )
        self.daemon.start()

    def install_pause_hook(self, stage: str) -> PauseHook:
        return PauseHook(stage)

    @contextmanager
    def simulate_repo_unreachable(self, repo_index: int) -> Iterator[None]:
        repo_id = self.repos[repo_index].repository_id
        if repo_id is None:
            raise RuntimeError("repo is not registered")
        runner = self._runner()
        runner.unreachable_ids.add(repo_id)
        try:
            yield
        finally:
            runner.unreachable_ids.discard(repo_id)

    def _runner(self, workflow: dict[str, object] | None = None) -> RepoLocalRunner:
        repos_by_id = {str(repo.repository_id): repo for repo in self.repos if repo.repository_id is not None}
        if self.local_runner is None:
            self.local_runner = RepoLocalRunner(repos_by_id, workflow=workflow)
        else:
            self.local_runner.repos_by_id = repos_by_id
            if workflow is not None:
                self.local_runner.workflow = workflow
        return self.local_runner


def pg_available_url() -> str:
    return pg.resolve_base_url()


def two_repo_workflow(harness: MultiRepoHarness) -> dict[str, object]:
    repo_a = str(harness.repos[0].repository_id)
    repo_b = str(harness.repos[1].repository_id)
    return repos.two_repo_workflow(repo_a, repo_b)


def cycle_workflow(harness: MultiRepoHarness) -> dict[str, object]:
    repo_a = str(harness.repos[0].repository_id)
    repo_b = str(harness.repos[1].repository_id)
    return repos.cycle_workflow(repo_a, repo_b)

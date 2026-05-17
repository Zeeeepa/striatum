"""Process-local state for the local web service."""

from __future__ import annotations

import secrets
import threading
from datetime import datetime, timezone
from pathlib import Path

from striatum.service_http import OriginTuple
from striatum.web.run_list import git_config_get, git_symbolic_ref, parse_github_remote

SSE_MAX_CONCURRENT_PER_RUN = 32

__all__ = ["SSE_MAX_CONCURRENT_PER_RUN", "ServiceState", "utcnow_iso"]


def utcnow_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


class ServiceState:
    """Shared process state for one local service instance."""

    def __init__(
        self,
        *,
        repo: Path,
        allow_mutations: bool,
        token: str | None,
        web_enabled: bool,
    ) -> None:
        self.repo = repo
        self.allow_mutations = allow_mutations
        self.token = token
        self.web_enabled = web_enabled
        self.origin_check_enabled = False
        self.allowed_origins: set[OriginTuple] = set()
        self.started_at = utcnow_iso()
        self._sse_counts: dict[str, int] = {}
        self._sse_lock = threading.Lock()
        self._shutdown = threading.Event()
        self._github_base_url: str | None | bool = False
        self._default_branch: str | None = None
        # GH #10: process-local HMAC secret for binding rendered job
        # pages to override-verdict POSTs. Rotated on every service
        # restart; tokens become invalid after restart, which is
        # acceptable because the page must be reloaded after restart.
        self.web_context_secret = secrets.token_bytes(32)

    def github_base_url(self) -> str | None:
        if self._github_base_url is False:
            self._github_base_url = parse_github_remote(
                git_config_get(self.repo, "remote.origin.url")
            )
        return self._github_base_url if isinstance(self._github_base_url, str) else None

    def default_branch(self) -> str:
        if self._default_branch is None:
            remote_head = git_symbolic_ref(self.repo, "refs/remotes/origin/HEAD")
            if remote_head and "/" in remote_head:
                self._default_branch = remote_head.rsplit("/", 1)[1]
            else:
                local_head = git_symbolic_ref(self.repo, "HEAD")
                self._default_branch = local_head or "main"
        return self._default_branch

    def acquire_sse_slot(self, run_id: str) -> bool:
        with self._sse_lock:
            current = self._sse_counts.get(run_id, 0)
            if current >= SSE_MAX_CONCURRENT_PER_RUN:
                return False
            self._sse_counts[run_id] = current + 1
        return True

    def release_sse_slot(self, run_id: str) -> None:
        with self._sse_lock:
            self._sse_counts[run_id] = max(0, self._sse_counts.get(run_id, 1) - 1)

    @property
    def shutting_down(self) -> bool:
        return self._shutdown.is_set()

    def signal_shutdown(self) -> None:
        self._shutdown.set()

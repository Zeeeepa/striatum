"""Soft imports for Track A's registry and context plumbing.

Track A (RFC 0048 Phase A workflow-loop track) owns
``src/striatum/daemon_pg/handlers/registry.py`` and
``src/striatum/daemon_pg/handlers/context.py``. Track B authors recovery and
evidence handlers under ``recovery_evidence/`` against the same locked
interface — so once Track A's plumbing lands, both subpackages share one
registry and one context shape.

Until Track A's modules land in the same merge train, importing this
subpackage must not fail (otherwise Track A cannot validate its own plumbing
without crashing on import of the Track B subpackage). The fallbacks below
preserve module importability while making the absence loud: the no-op
decorator records the intended method name on the function, and the stub
context exposes the expected attribute surface so static type-checkers can
follow the contract.

Once Track A lands ``registry.py`` and ``context.py``, this shim resolves to
the real symbols and decorator-based self-registration just works.
"""

from __future__ import annotations

from typing import Any, Callable, Mapping, Protocol

HandlerFn = Callable[["RepoHandlerContextProtocol", Mapping[str, Any]], dict[str, Any]]


class RepoHandlerContextProtocol(Protocol):
    """The locked handler-context surface (synthesis L17).

    Track A's concrete ``RepoHandlerContext`` carries:
      - ``pg_conn``: psycopg connection scoped to this RPC request.
      - ``repository_id``: the scope predicate every SQL statement must include.
      - ``repo_root``: filesystem path used by handlers that read or write the
        target repo (notably ``evidence.export`` and
        ``recovery.auto_publish_stale_artifacts``).
      - ``auth``: ``RpcAuthContext`` used to enforce capability tokens before
        writes (synthesis L19, finding 3 follow-up).
      - ``now()``: monotonic UTC timestamp string for row writes.
      - ``new_id()``: identifier minter for newly inserted rows
        (e.g., ``msg_*``, ``ev_*``, ``art_*``).
      - ``append_event()``: chain-aware event insertion that locks the per-repo
        head row ``FOR UPDATE``, computes ``row_hash`` over the canonical
        payload, and updates ``repo_event_chain_heads`` before commit.
    """

    pg_conn: Any
    repository_id: str
    repo_root: Any
    auth: Any

    def now(self) -> str: ...

    def new_id(self, prefix: str) -> str: ...

    def append_event(
        self,
        *,
        run_id: str | None,
        event_type: str,
        actor_session_id: str | None = None,
        job_id: str | None = None,
        message_id: str | None = None,
        artifact_id: str | None = None,
        lease_id: str | None = None,
        payload: Mapping[str, Any] | None = None,
    ) -> str: ...


try:  # pragma: no cover - imported once Track A merges.
    from striatum.daemon_pg.handlers.registry import (  # type: ignore[import-not-found]
        register_pg_handler as _register_pg_handler,
    )
    from striatum.daemon_pg.handlers.context import (  # type: ignore[import-not-found]
        RepoHandlerContext as _RepoHandlerContext,
    )

    register_pg_handler = _register_pg_handler
    RepoHandlerContext = _RepoHandlerContext
    REGISTRY_AVAILABLE = True
except ImportError:
    REGISTRY_AVAILABLE = False

    def register_pg_handler(method: str) -> Callable[[HandlerFn], HandlerFn]:
        """No-op decorator that records the intended RPC method name.

        Track A wires the real registry; until then this records the method
        name on the function so the subpackage's `__init__.py` import path
        is still load-bearing once Track A lands.
        """

        def _decorator(func: HandlerFn) -> HandlerFn:
            setattr(func, "_pg_rpc_method", method)
            return func

        return _decorator

    RepoHandlerContext = RepoHandlerContextProtocol  # type: ignore[misc, assignment]


__all__ = [
    "RepoHandlerContext",
    "RepoHandlerContextProtocol",
    "register_pg_handler",
    "REGISTRY_AVAILABLE",
    "HandlerFn",
]

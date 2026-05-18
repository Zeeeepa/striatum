"""Minimal daemon RPC router."""

from __future__ import annotations

from dataclasses import replace
from pathlib import Path
from typing import Any, Mapping

from striatum.daemon_apply.signing_key import key_loaded
from striatum.daemon_rpc.capability import RpcAuthContext, authorize, require_allowed
from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError, RpcResponse
from striatum.daemon_rpc.handshake import build_welcome
from striatum.daemon_rpc.registry import METHOD_REGISTRY, METHODS_ETAG, describe_methods
from striatum.daemon_rpc.request_log import append_audit_row, append_request_log, request_id_seen
from striatum.errors import (
    ArtifactError,
    BranchConfirmationError,
    InvalidTransitionError,
    LeaseError,
    NotFoundError,
    StriatumError,
    WorkflowError,
)


CLI_ROUTES: dict[str, tuple[str, ...]] = {}

LOCAL_FILE_AUTHORING_METHODS: frozenset[str] = frozenset(
    {
        "workflow.validate",
        "workflow.plan",
        "workflow.graph",
        "workflow.templates.list",
        "workflow.templates.show",
        "workflow.init",
        "workflow.generate",
        "workflow.upgrade",
    }
)

PRODUCTION_MCP_HIDDEN_METHODS: frozenset[str] = LOCAL_FILE_AUTHORING_METHODS


class DaemonRpcRouter:
    def __init__(self, *, pg_conn: Any | None = None, repo_root: Path | None = None, substrate_schema: int = 1) -> None:
        self.pg_conn = pg_conn
        self.repo_root = (repo_root or Path.cwd()).resolve()
        self.substrate_schema = substrate_schema
        self._handshaken_connections: set[str] = set()

    def handle(
        self,
        envelope: RpcEnvelope,
        *,
        connection_id: str = "default",
        transport: str = "rpc",
        require_handshake: bool = True,
    ) -> RpcResponse:
        auth = RpcAuthContext(None, None, _repository_id(envelope.params), None, "allowed")
        authorized_for_route = False
        if self.pg_conn is not None and request_id_seen(self.pg_conn, request_id=envelope.request_id):
            error = RpcError("duplicate_request", "daemon RPC request_id was already used")
            return RpcResponse.error_response(request_id=envelope.request_id, error=error)
        try:
            if (
                require_handshake
                and envelope.method != "daemon.hello"
                and connection_id not in self._handshaken_connections
            ):
                auth = _denied_auth(auth, "version_incompatible")
                raise RpcError("version_incompatible", "daemon.hello must run before ordinary RPC routes")
            entry = METHOD_REGISTRY.get(envelope.method)
            if entry is None:
                auth = _denied_auth(auth, "method_unknown")
                raise RpcError("method_unknown", f"unknown daemon RPC method: {envelope.method}")
            if entry.repository_scope and _repository_id(envelope.params) is None:
                auth = _denied_auth(auth, "repo_not_registered")
                raise RpcError("repo_not_registered", "daemon RPC route requires repository_id")
            if envelope.method == "daemon.hello":
                loaded = key_loaded()
                data = build_welcome(
                    envelope.params,
                    methods_etag=METHODS_ETAG,
                    substrate_schema=self.substrate_schema,
                    sealed_apply_supported=loaded,
                    key_loaded=loaded,
                )
                self._handshaken_connections.add(connection_id)
            else:
                if self.pg_conn is None and entry.required_capability is not None:
                    auth = _denied_auth(auth, "token_missing")
                    raise RpcError("token_missing", "daemon RPC authorization requires a daemon DB connection")
                if self.pg_conn is not None:
                    auth = authorize(
                        self.pg_conn,
                        required=entry.required_capability,
                        repository_id=_repository_id(envelope.params),
                        token=envelope.capability_token,
                    )
                require_allowed(auth)
                repo_root = self._repo_root_for(envelope, auth=auth)
                authorized_for_route = True
                data = self._route(envelope, repo_root=repo_root, auth=auth)
            response = RpcResponse.ok_response(request_id=envelope.request_id, data=data)
        except RpcError as exc:
            if not authorized_for_route:
                auth = _denied_auth(auth, exc.code)
            response = RpcResponse.error_response(request_id=envelope.request_id, error=exc)
        except StriatumError as exc:
            rpc_error = _domain_error_to_rpc(exc)
            if not authorized_for_route:
                auth = _denied_auth(auth, rpc_error.code)
            response = RpcResponse.error_response(request_id=envelope.request_id, error=rpc_error)
        return self._record_and_return(envelope, auth=auth, response=response, transport=transport)

    def _record_and_return(
        self,
        envelope: RpcEnvelope,
        *,
        auth: RpcAuthContext,
        response: RpcResponse,
        transport: str = "rpc",
    ) -> RpcResponse:
        if self.pg_conn is None:
            return response
        audit_value = append_audit_row(
            self.pg_conn,
            auth=auth,
            method=envelope.method,
            transport=transport,
            request_id=envelope.request_id,
            params=envelope.params,
            exit_code=None if response.ok else 10,
        )
        response_with_audit = replace(response, audit_id=f"aud_{audit_value}" if audit_value is not None else None)
        append_request_log(
            self.pg_conn,
            request_id=envelope.request_id,
            method=envelope.method,
            params=envelope.params,
            auth=auth,
            decision=auth.decision,
            response=response_with_audit.to_mapping(),
            audit_id=audit_value,
        )
        return response_with_audit

    def _repo_root_for(self, envelope: RpcEnvelope, *, auth: RpcAuthContext) -> Path:
        repository_id = auth.repository_id or _repository_id(envelope.params)
        if self.pg_conn is None or repository_id is None:
            return self.repo_root
        with self.pg_conn.cursor() as cur:
            cur.execute(
                "SELECT repo_root FROM striatumd.repositories WHERE repository_id = %s AND state = 'active'",
                (repository_id,),
            )
            row = cur.fetchone()
        if row is None:
            raise RpcError("repo_not_registered", "daemon RPC repository is not registered")
        # RFC 0048 V1.5: the daemon serves every registered repository. The
        # router's `self.repo_root` is only a constructor default for callers
        # that don't supply a repository_id (e.g., daemon_global routes); it
        # should NOT gate per-repo lookups. Resolve the repo_root from the
        # repositories row and return it.
        return Path(str(_row_value(row, "repo_root"))).expanduser().resolve()

    def _route(self, envelope: RpcEnvelope, *, repo_root: Path, auth: RpcAuthContext) -> dict[str, Any]:
        if envelope.method == "daemon.describe":
            return describe_methods()
        if envelope.method == "dashboard.all":
            if self.pg_conn is None:
                raise RpcError("daemon_db_missing", "dashboard.all requires daemon PostgreSQL")
            from striatum.daemon_pg.mcp_resources import dashboard_all_payload_pg

            return dashboard_all_payload_pg(self.pg_conn)
        if envelope.method.startswith("repo."):
            return self._route_repo(envelope)
        if envelope.method.startswith("cross_repo."):
            return self._route_cross_repo(envelope, auth=auth)
        if envelope.method.startswith("apply."):
            from striatum.daemon_apply.apply_service import handle_apply_rpc

            return handle_apply_rpc(envelope.method, envelope.params)
        if self.pg_conn is not None:
            import striatum.daemon_pg.handlers  # noqa: F401 - import registers handlers.
            from striatum.daemon_pg.handlers.context import RepoHandlerContext
            from striatum.daemon_pg.handlers.registry import resolve_pg_handler

            handler = resolve_pg_handler(envelope.method)
            if handler is not None:
                repository_id = auth.repository_id or _repository_id(envelope.params)
                if repository_id is None:
                    raise RpcError("repo_not_registered", "daemon RPC route requires repository_id")
                ctx = RepoHandlerContext(
                    pg_conn=self.pg_conn,
                    repository_id=repository_id,
                    repo_root=repo_root,
                    auth=auth,
                )
                return handler(ctx, envelope.params)
        if envelope.method in LOCAL_FILE_AUTHORING_METHODS:
            raise RpcError(
                "not_implemented",
                "workflow authoring is CLI-local and has no daemon live-state fallback",
            )
        raise RpcError("method_unknown", f"method has no handler: {envelope.method}")

    def _route_repo(self, envelope: RpcEnvelope) -> dict[str, Any]:
        from striatum.daemon_pg import repositories

        if self.pg_conn is None:
            raise RpcError("daemon_db_missing", "repo routes require daemon PostgreSQL")
        if envelope.method == "repo.list":
            return repositories.repo_list_pg(self.pg_conn)
        if envelope.method == "repo.resolve":
            raw_path = envelope.params.get("path") or envelope.params.get("repo_root")
            if not isinstance(raw_path, str) or not raw_path:
                raise RpcError("schema_invalid", "repo.resolve requires path")
            try:
                return repositories.repo_resolve_pg(self.pg_conn, Path(raw_path))
            except NotFoundError as exc:
                raise RpcError("repo_not_registered", str(exc)) from exc
        if envelope.method == "repo.add":
            raw_path = envelope.params.get("path")
            if not isinstance(raw_path, str) or not raw_path:
                raise RpcError("schema_invalid", "repo.add requires path")
            display_name = envelope.params.get("display_name")
            if display_name is not None and not isinstance(display_name, str):
                raise RpcError("schema_invalid", "repo.add display_name must be a string")
            return repositories.repo_add_pg(
                self.pg_conn,
                Path(raw_path),
                display_name=display_name,
                no_migrate=bool(envelope.params.get("no_migrate", False)),
                init=bool(envelope.params.get("init", False)),
            )
        if envelope.method == "repo.remove":
            identifier = envelope.params.get("id") or envelope.params.get("repository_id")
            if not isinstance(identifier, str) or not identifier:
                raise RpcError("schema_invalid", "repo.remove requires id")
            return repositories.repo_remove_pg(self.pg_conn, identifier)
        raise RpcError("method_unknown", f"method has no handler: {envelope.method}")

    def _route_cross_repo(
        self,
        envelope: RpcEnvelope,
        *,
        auth: RpcAuthContext,
    ) -> dict[str, Any]:
        from striatum.cross_repo import (
            PgCrossRepoLocalRunner,
            cancel_cross_repo_run,
            describe_cross_repo_run,
            list_cross_repo_runs,
        )

        if self.pg_conn is None:
            raise RpcError("daemon_db_missing", "cross-repo routes require daemon PostgreSQL")
        if envelope.method == "cross_repo.list":
            return list_cross_repo_runs(self.pg_conn)
        run_id = str(envelope.params.get("cross_repo_run_id") or envelope.params.get("run_id") or "")
        if not run_id:
            raise RpcError("schema_invalid", "cross-repo route requires cross_repo_run_id")
        if envelope.method == "cross_repo.describe":
            return describe_cross_repo_run(self.pg_conn, cross_repo_run_id=run_id)
        if envelope.method == "cross_repo.why":
            described = describe_cross_repo_run(self.pg_conn, cross_repo_run_id=run_id)
            return {
                "cross_repo_run_id": run_id,
                "state": described["state"],
                "participants": described["participants"],
            }
        if envelope.method == "cross_repo.cancel":
            return cancel_cross_repo_run(
                self.pg_conn,
                cross_repo_run_id=run_id,
                local_runner=PgCrossRepoLocalRunner(self.pg_conn, auth=auth),
                reason=str(
                    envelope.params.get("reason")
                    or "operator canceled cross-repo run"
                ),
            )
        raise RpcError("method_unknown", f"method has no handler: {envelope.method}")

def _repository_id(params: Mapping[str, Any]) -> str | None:
    value = params.get("repository_id")
    return str(value) if value is not None else None


def _denied_auth(auth: RpcAuthContext, reason: str) -> RpcAuthContext:
    return RpcAuthContext(
        auth.client_id,
        auth.token_id,
        auth.repository_id,
        auth.capability,
        "denied",
        reason,
    )


def _domain_error_to_rpc(exc: StriatumError) -> RpcError:
    code = "command_failed"
    details: dict[str, object] = {}
    if isinstance(exc, NotFoundError):
        code = "not_found"
    elif isinstance(exc, InvalidTransitionError):
        code = "invalid_transition"
    elif isinstance(exc, LeaseError):
        code = "lease_error"
    elif isinstance(exc, ArtifactError):
        code = "artifact_error"
    elif isinstance(exc, BranchConfirmationError):
        code = "branch_confirmation_required"
    elif isinstance(exc, WorkflowError):
        code = "workflow_error"
        field_path = getattr(exc, "field_path", None)
        if isinstance(field_path, str) and field_path:
            details["field_path"] = field_path
    return RpcError(code, str(exc), exit_code=exc.exit_code, details=details)


def _row_value(row: Any, key: str) -> Any:
    if isinstance(row, dict):
        return row[key]
    if hasattr(row, "keys"):
        return row[key]
    if isinstance(row, (tuple, list)):
        return row[0]
    raise TypeError("database row must expose mapping-like keys or sequence values")

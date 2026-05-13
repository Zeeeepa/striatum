"""Minimal daemon RPC router."""

from __future__ import annotations

from dataclasses import replace
from pathlib import Path
from typing import Any, Callable, Mapping

from striatum.api import invoke
from striatum.daemon_apply.signing_key import key_loaded
from striatum.daemon_rpc.capability import RpcAuthContext, authorize, require_allowed
from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError, RpcResponse
from striatum.daemon_rpc.handshake import build_welcome
from striatum.daemon_rpc.registry import METHOD_REGISTRY, METHODS_ETAG, describe_methods
from striatum.daemon_rpc.request_log import append_audit_row, append_request_log, request_id_seen


Handler = Callable[[RpcEnvelope], Mapping[str, Any]]

CLI_ROUTES: dict[str, tuple[str, ...]] = {
    # ----- Read -----
    "status": ("status",),
    "why": ("why",),
    "doctor": ("doctor",),
    "dashboard": ("dashboard",),
    "evidence.export": ("evidence", "export"),
    "corpus.export": ("corpus", "export"),
    "run.summary": ("run", "summary"),
    "run.graph": ("run", "graph"),
    "workflow.validate": ("workflow", "validate"),
    "workflow.plan": ("workflow", "plan"),
    "workflow.graph": ("workflow", "graph"),
    "workflow.templates.list": ("workflow", "templates", "list"),
    "workflow.templates.show": ("workflow", "templates", "show"),
    "list.runs": ("list", "runs"),
    "list.sessions": ("list", "sessions"),
    "list.jobs": ("list", "jobs"),
    "list.artifacts": ("list", "artifacts"),
    "list.workflows": ("list", "workflows"),
    "worktree.list": ("worktree", "list"),
    # ----- Claim -----
    "session.register": ("register-session",),
    "session.close": ("session", "close"),
    "work.claim_next": ("claim-next",),
    "work.ack": ("ack",),
    "work.heartbeat": ("heartbeat",),
    "work.release": ("release",),
    "supervise.start": ("supervise", "start"),
    "supervise.send": ("supervise", "send"),
    "supervise.stop": ("supervise", "stop"),
    "supervise.status": ("supervise", "status"),
    "supervise.list": ("supervise", "list"),
    # ----- Write -----
    "work.send_message": ("send",),
    "work.block": ("block",),
    "work.complete": ("complete",),
    "artifact.publish": ("publish-artifact",),
    "worktree.create": ("worktree", "create"),
    "worktree.release": ("worktree", "release"),
    "workflow.init": ("workflow", "init"),
    "workflow.generate": ("workflow", "generate"),
    "workflow.upgrade": ("workflow", "upgrade"),
    # ----- Review -----
    "review.submit": ("submit-review",),
    "review.verdict": ("verdict",),
    "review.override": ("override-verdict",),
    # ----- Admin -----
    "decision.record": ("decision", "record"),
    "checkpoint.resolve": ("checkpoint", "resolve"),
    "branch.confirm": ("branch", "confirm"),
    "run.prepare": ("run", "prepare"),
    "run.start": ("run", "start"),
    "run.pause": ("run", "pause"),
    "run.resume": ("run", "resume"),
    "run.cancel": ("run", "cancel"),
    "run.retry_job": ("run", "retry-job"),
    # ----- Recovery -----
    "recovery.stale_leases": ("recovery", "stale-leases"),
    "recovery.requeue_stale": ("recovery", "requeue-stale"),
    "recovery.cancel_job": ("recovery", "cancel-job"),
    "recovery.process_reconcile": ("recovery", "process-reconcile"),
    "recovery.resume": ("recovery", "resume"),
    "recovery.auto": ("recovery", "auto"),
    "recovery.watch": ("recovery", "watch"),
    # ----- Legacy aliases (deprecated in the registry) -----
    "ack": ("ack",),
    "block": ("block",),
    "heartbeat": ("heartbeat",),
    "publish_artifact": ("publish-artifact",),
    "complete": ("complete",),
    "release": ("release",),
    "claim_next": ("claim-next",),
    "verdict": ("verdict",),
    "submit_review": ("submit-review",),
}


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
                data = self._route(envelope, repo_root=repo_root)
            response = RpcResponse.ok_response(request_id=envelope.request_id, data=data)
        except RpcError as exc:
            auth = _denied_auth(auth, exc.code)
            response = RpcResponse.error_response(request_id=envelope.request_id, error=exc)
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
        repo_root = Path(str(_row_value(row, "repo_root"))).expanduser().resolve()
        if repo_root != self.repo_root:
            raise RpcError("repo_not_registered", "daemon RPC repository does not match this router")
        return repo_root

    def _route(self, envelope: RpcEnvelope, *, repo_root: Path) -> dict[str, Any]:
        if envelope.method == "daemon.describe":
            return describe_methods()
        if envelope.method == "dashboard.all":
            from striatum.daemon import dashboard_all

            return dashboard_all(token=envelope.capability_token)
        if envelope.method.startswith("cross_repo."):
            return self._route_cross_repo(envelope)
        if envelope.method.startswith("apply."):
            from striatum.daemon_apply.apply_service import handle_apply_rpc

            return handle_apply_rpc(envelope.method, envelope.params)
        if envelope.method.startswith("dogfood."):
            return self._route_dogfood(envelope, repo_root=repo_root)
        prefix = CLI_ROUTES.get(envelope.method)
        if prefix is None:
            raise RpcError("method_unknown", f"method has no handler: {envelope.method}")
        args = [*prefix, *_params_to_args(envelope.params)]
        result = invoke(args, repo=repo_root)
        if not result.get("ok"):
            error = result.get("error")
            if isinstance(error, dict):
                raise RpcError("command_failed", str(error.get("message", "daemon RPC command failed")), exit_code=int(error.get("code", 1)))
            raise RpcError("command_failed", "daemon RPC command failed", exit_code=1)
        data = result.get("data")
        return data if isinstance(data, dict) else {"result": data}

    def _route_cross_repo(self, envelope: RpcEnvelope) -> dict[str, Any]:
        from striatum.cross_repo import describe_cross_repo_run, list_cross_repo_runs

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
            raise RpcError(
                "not_implemented",
                "cross-repo cancel requires the daemon lifecycle service; full E2E harness is deferred",
            )
        raise RpcError("method_unknown", f"method has no handler: {envelope.method}")

    def _route_dogfood(self, envelope: RpcEnvelope, *, repo_root: Path) -> dict[str, Any]:
        from striatum.db import connect
        from striatum.dogfood.operator_tools import publish_on_behalf, surgical_recovery

        with connect(repo_root) as conn:
            if envelope.method == "dogfood.publish_on_behalf":
                return publish_on_behalf(
                    conn,
                    repo=repo_root,
                    session_id=str(envelope.params.get("session_id") or ""),
                    artifact_path=str(envelope.params.get("artifact_path") or ""),
                    artifact_kind=str(envelope.params.get("artifact_kind") or ""),
                    logical_name=str(envelope.params.get("logical_name") or ""),
                    reason=str(envelope.params.get("reason") or ""),
                    verdict=_optional_str(envelope.params.get("verdict")),
                    findings_artifact_id=_optional_str(envelope.params.get("findings_artifact_id")),
                    verdict_rationale=_optional_str(envelope.params.get("verdict_rationale")),
                    summary=_optional_str(envelope.params.get("summary")),
                )
            if envelope.method == "dogfood.surgical_recovery":
                return surgical_recovery(
                    conn,
                    job_id=str(envelope.params.get("job_id") or ""),
                    reason=str(envelope.params.get("reason") or ""),
                    extend_lease_seconds=int(envelope.params.get("extend_lease_seconds") or 900),
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


def _row_value(row: Any, key: str) -> Any:
    if isinstance(row, dict):
        return row[key]
    if hasattr(row, "keys"):
        return row[key]
    if isinstance(row, (tuple, list)):
        return row[0]
    raise TypeError("database row must expose mapping-like keys or sequence values")


def _optional_str(value: Any) -> str | None:
    if value is None:
        return None
    text = str(value)
    return text if text else None


def _params_to_args(params: Mapping[str, Any]) -> list[str]:
    args: list[str] = []
    for key, value in params.items():
        if key in {"repository_id", "capability_token"} or value is None:
            continue
        flag = "--" + key.replace("_", "-")
        if isinstance(value, bool):
            if value:
                args.append(flag)
        elif isinstance(value, list):
            for item in value:
                args.extend([flag, str(item)])
        else:
            args.extend([flag, str(value)])
    return args

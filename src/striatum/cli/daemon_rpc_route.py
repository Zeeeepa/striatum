"""RFC 0048 Phase C — CLI dispatch routes through daemon RPC.

This module translates CLI argparse namespaces to daemon RPC envelopes
and dispatches them over the Unix socket. When a CLI command maps to a
registered RPC method AND the daemon is reachable, the call routes
through ``DaemonRpcRouter`` (which uses the PG-backed handlers from
RFC 0048 V1 Phase A) instead of the in-process SQLite path.

The hook in :func:`try_route` returns ``None`` to fall through to legacy
SQLite dispatch when no RPC mapping exists. That keeps the bootstrap
verbs (``init``, ``skills``, ``plugin``, ``daemon``, ``repo``,
``serve``, ``byline``) and other unported surfaces working unchanged.
"""

from __future__ import annotations

import argparse
import os
import socket
import uuid
from pathlib import Path
from typing import Any, Callable, Mapping

from striatum.cli.daemon_required import daemon_socket_is_reachable, resolve_socket_path
from striatum.daemon_rpc.client import hello_envelope
from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError
from striatum.daemon_rpc.registry import METHOD_REGISTRY
from striatum.db import json_loads
from striatum.errors import StriatumError


def try_route(args: argparse.Namespace, repo: Path) -> tuple[bool, Any]:
    """Route a CLI command through the daemon RPC if possible.

    Returns ``(True, response_data)`` if the command was routed through
    the daemon (success or RPC error). The caller should return that
    value as the CLI result. Returns ``(False, None)`` if no routing
    applies and dispatch should fall through to the legacy SQLite path.

    Routing applies when:
    - The daemon socket is reachable.
    - The CLI verb has a registered RPC method in this module's lookup
      table.
    - The repo (when relevant) is registered with the daemon.

    On RPC error (e.g., ``method_unknown``, ``capability_missing``), the
    function raises :class:`StriatumError` with an appropriate exit code.
    """
    if os.environ.get("STRIATUM_TEST_HARNESS") == "1":
        return (False, None)
    # Historical daemon-internal invoke paths set this guard. Keep honoring it
    # so compatibility/test helpers can still re-enter dispatch locally without
    # trying to route back through daemon RPC.
    if os.environ.get("STRIATUM_IN_DAEMON_HANDLER") == "1":
        return (False, None)
    translator = _LOOKUP.get((args.command, _subcommand(args)))
    if translator is None:
        translator = _LOOKUP.get((args.command, None))
    if translator is None:
        return (False, None)
    method, params = translator(args, repo)
    if method not in METHOD_REGISTRY:
        return (False, None)
    sock_path = resolve_socket_path()
    if not daemon_socket_is_reachable(sock_path):
        raise StriatumError(
            f"daemon_unreachable: {method} is daemon-routed and cannot fall back to SQLite",
            exit_code=12,
        )
    repository_id = params.get("repository_id") or _lookup_repository_id(repo)
    if repository_id is None:
        raise StriatumError(
            f"repo_not_registered: {method} requires a daemon-registered repository",
            exit_code=12,
        )
    if repository_id is not None and "repository_id" not in params:
        params = dict(params)
        params["repository_id"] = repository_id
    response = _call_with_handshake(sock_path, method, params)
    if not response.get("ok"):
        err = response.get("data") or {}
        code = err.get("code", "rpc_error")
        msg = err.get("message", "daemon RPC call failed")
        exit_code = 1
        if code == "method_unknown":
            exit_code = 8
        elif code in ("token_missing", "capability_missing", "version_incompatible"):
            exit_code = 14
        elif code == "duplicate_request":
            exit_code = 9
        raise StriatumError(f"{code}: {msg}", exit_code=exit_code)
    data = response.get("data")
    if (
        method == "run.graph"
        and not bool(getattr(args, "json", False))
        and isinstance(data, dict)
        and isinstance(data.get("source"), str)
    ):
        return (True, data["source"])
    return (True, data)


def _subcommand(args: argparse.Namespace) -> str | None:
    """Extract a verb's subcommand attribute (e.g., ``run_command`` for ``run``)."""
    for attr in ("run_command", "workflow_command", "list_command", "recovery_command",
                 "evidence_command", "corpus_command", "decision_command",
                 "checkpoint_command", "escalation_command", "branch_command", "session_command",
                 "worktree_command", "supervise_command", "cross_repo_command", "archive_command"):
        v = getattr(args, attr, None)
        if v is not None:
            return str(v)
    return None


def _call_with_handshake(sock_path: Path, method: str, params: Mapping[str, Any]) -> dict[str, Any]:
    """Open a Unix socket, do daemon.hello, then call ``method``, return response dict."""
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
        sock.settimeout(30.0)
        sock.connect(str(sock_path))
        stream = sock.makefile("rwb")
        # Handshake
        hello = hello_envelope(
            request_id=uuid.uuid4().hex,
            client_name="striatum-cli",
            client_version="1.51.0",
        )
        stream.write(hello.encode() + b"\n")
        stream.flush()
        hello_resp_line = stream.readline()
        hello_resp = json_loads(hello_resp_line.decode("utf-8"))
        if not hello_resp.get("ok"):
            raise RpcError(
                "version_incompatible",
                f"daemon.hello refused: {hello_resp.get('data')}",
            )
        # Real call — load runtime token so RPC authorization can proceed.
        from striatum.daemon import read_runtime_token

        capability_token = read_runtime_token()
        envelope = RpcEnvelope(
            schema_version=1,
            request_id=uuid.uuid4().hex,
            method=method,
            params=dict(params),
            capability_token=capability_token,
        )
        stream.write(envelope.encode() + b"\n")
        stream.flush()
        resp_line = stream.readline()
        resp = json_loads(resp_line.decode("utf-8"))
        if not isinstance(resp, dict):
            raise RpcError("schema_invalid", "daemon RPC response must be a JSON object")
        return resp


_REPO_ID_CACHE: dict[str, str] = {}


def _lookup_repository_id(repo: Path) -> str | None:
    """Look up the repository_id for ``repo`` in the daemon's Postgres state.

    Cached per-process. Returns None if the repo isn't registered (caller
    falls back to legacy SQLite or surfaces ``repo_not_migrated`` exit 12).
    """
    key = str(repo.resolve())
    if key in _REPO_ID_CACHE:
        return _REPO_ID_CACHE[key]
    # Use daemon's PG connection via repo_local_migration helper if available.
    try:
        from striatum.daemon_pg.config import resolve_config
        from striatum.daemon_pg.connection import connect

        cfg = resolve_config()
        if cfg.url is None:
            return None
        conn = connect(cfg.url)
        try:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT repository_id FROM striatumd.repositories WHERE repo_root = %s AND state = 'active'",
                    (key,),
                )
                row = cur.fetchone()
        finally:
            conn.close()
        if row is None:
            return None
        repo_id = str(row[0])
        _REPO_ID_CACHE[key] = repo_id
        return repo_id
    except Exception:  # noqa: BLE001 — fall back to non-routed dispatch on any lookup failure.
        return None


# ----- Per-verb translators -----
# Each translator returns (rpc_method_name, params_dict).
# Daemon RPC params keys follow RFC 0030 envelope convention.


def _route_status(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    params: dict[str, Any] = {}
    if getattr(args, "run_id", None):
        params["run_id"] = args.run_id
    return ("status", params)


def _route_why(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("why", {"target_id": args.id})


def _route_doctor(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("doctor", {"verbose": bool(getattr(args, "verbose", False))})


def _route_dashboard(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("dashboard", {"run_id": getattr(args, "run_id", None)})


def _route_list(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    sub = getattr(args, "list_command", None) or "runs"
    params = _selected_params(
        args,
        ("run_id", "state", "role", "lane", "workflow_job_id", "kind", "limit"),
    )
    return (f"list.{sub}", params)


def _selected_params(args: argparse.Namespace, names: tuple[str, ...]) -> dict[str, Any]:
    params: dict[str, Any] = {}
    for name in names:
        value = getattr(args, name, None)
        if value is not None:
            params[name] = value
    return params


def _route_run_prepare(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("run.prepare", {"workflow": args.workflow})


def _route_run_start(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("run.start", {"run_id": args.run_id})


def _route_run_pause(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("run.pause", {"run_id": args.run_id, "reason": getattr(args, "reason", "")})


def _route_run_resume(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("run.resume", {"run_id": args.run_id})


def _route_run_cancel(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("run.cancel", {"run_id": args.run_id, "reason": getattr(args, "reason", "")})


def _route_run_retry_job(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("run.retry_job", {"run_id": args.run_id, "job_id": args.job_id})


def _route_run_summary(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("run.summary", {"run_id": args.run_id, "path": getattr(args, "path", None)})


def _route_run_graph(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return (
        "run.graph",
        {
            "run_id": args.run_id,
            "format": args.format,
            "graph_orient": getattr(args, "graph_orient", "tb"),
            "graph_style": getattr(args, "graph_style", "layered"),
        },
    )


def _route_register_session(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    params: dict[str, Any] = {
        "run_id": args.run_id,
        "role": args.role,
        "lane": args.lane,
        "fresh": bool(getattr(args, "fresh", False)),
    }
    if getattr(args, "capability", None):
        params["capability"] = args.capability
    if getattr(args, "parent_session_id", None):
        params["parent_session_id"] = args.parent_session_id
    if getattr(args, "operator_label", None):
        params["operator_label"] = args.operator_label
    if getattr(args, "force_non_fresh", False):
        params["force_non_fresh"] = True
    if getattr(args, "reason", None):
        params["reason"] = args.reason
    return ("session.register", params)


def _route_session_close(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("session.close", {"session_id": args.session_id, "reason": args.reason})


def _route_claim_next(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("work.claim_next", {
        "session_id": args.session_id,
        "lease_seconds": getattr(args, "lease_seconds", None),
    })


def _route_ack(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("work.ack", {
        "session_id": args.session_id,
        "message_id": args.message_id,
        "lease_id": args.lease_id,
    })


def _route_heartbeat(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("work.heartbeat", {
        "session_id": args.session_id,
        "lease_id": args.lease_id,
        "extend_seconds": getattr(args, "extend_seconds", None),
    })


def _route_release(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("work.release", {
        "session_id": args.session_id,
        "message_id": args.message_id,
        "lease_id": args.lease_id,
        "reason": getattr(args, "reason", ""),
        "requeue": bool(getattr(args, "requeue", False)),
    })


def _route_send(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return (
        "work.send_message",
        {
            "session_id": args.session_id,
            "kind": args.kind,
            "body_json": args.body_json,
        },
    )


def _route_block(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("work.block", {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "kind": args.kind,
        "severity": args.severity,
        "description": args.description,
    })


def _route_complete(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("work.complete", {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "summary": getattr(args, "summary", ""),
    })


def _route_publish_artifact(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("artifact.publish", {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "kind": args.kind,
        "logical_name": args.logical_name,
        "path": args.path,
        "allow_no_process_execution": bool(getattr(args, "allow_no_process_execution", False)),
        "override_rationale": getattr(args, "override_rationale", None),
    })


def _route_verdict(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("review.verdict", {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "verdict": args.verdict,
        "findings_artifact_id": getattr(args, "findings_artifact_id", None),
        "rationale": getattr(args, "rationale", ""),
    })


def _route_submit_review(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("review.submit", {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "path": args.path,
        "verdict": args.verdict,
        "logical_name": getattr(args, "logical_name", "review"),
        "kind": getattr(args, "kind", "finding"),
        "rationale": getattr(args, "rationale", None),
        "findings_artifact_id": getattr(args, "findings_artifact_id", None),
        "allow_no_process_execution": bool(getattr(args, "allow_no_process_execution", False)),
        "override_rationale": getattr(args, "override_rationale", None),
    })


def _route_override_verdict(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("review.override", {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "verdict": args.verdict,
        "rationale": args.rationale,
        "findings_artifact_id": getattr(args, "findings_artifact_id", None),
        "auto_fresh_session": bool(getattr(args, "auto_fresh_session", False)),
    })


def _route_recovery(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    recovery_command = str(getattr(args, "recovery_command", None) or "")
    sub = recovery_command.replace("-", "_")
    if not sub:
        raise StriatumError("recovery subcommand required", exit_code=2)
    if recovery_command == "auto":
        method = "recovery.sweep"
    elif recovery_command == "auto-publish":
        method = "recovery.auto_publish_stale_artifacts"
    else:
        method = f"recovery.{sub}"
    params: dict[str, Any] = {}
    if getattr(args, "run_id", None):
        params["run_id"] = args.run_id
    if getattr(args, "job_id", None):
        params["job_id"] = args.job_id
    if getattr(args, "session_id", None):
        params["session_id"] = args.session_id
    if getattr(args, "reason", None):
        params["reason"] = args.reason
    if getattr(args, "cascade", False):
        params["cascade"] = True
    if getattr(args, "force", False):
        params["force"] = True
    if getattr(args, "justification", None):
        params["justification"] = args.justification
    if getattr(args, "blocker_id", None):
        params["blocker_id"] = args.blocker_id
    if getattr(args, "complete", False):
        params["complete"] = True
    if getattr(args, "summary", None):
        params["summary"] = args.summary
    if getattr(args, "extend_seconds", None) is not None:
        params["extend_seconds"] = args.extend_seconds
    for name in (
        "dry_run",
        "autonomous_review_requeue",
        "autonomous_process_reconcile",
        "max_requeues_per_sweep",
        "checkpoint_timeout_seconds",
        "eligible_after_seconds",
        "interval_seconds",
        "exit_on_terminal",
        "max_sweeps",
        "mtime_grace_seconds",
        "allow_no_process_execution",
    ):
        value = getattr(args, name, None)
        if value is not None:
            params[name] = value
    return (method, params)


def _route_evidence_export(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("evidence.export", {
        "run_id": getattr(args, "run_id", None),
        "path": getattr(args, "path", None),
    })


def _route_corpus_export(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("corpus.export", {
        "since": args.since,
        "out": args.out,
    })


def _route_archive_create(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("archive.create", {
        "run_id": args.run_id,
        "out": args.out,
    })


def _route_decision_record(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("decision.record", {
        "run_id": args.run_id,
        "path": args.path,
        "title": args.title,
        "outcome": args.outcome,
        "decision_id": getattr(args, "decision_id", None),
        "rationale": getattr(args, "rationale", ""),
        "follow_up": getattr(args, "follow_up", None),
    })


def _route_checkpoint_resolve(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("checkpoint.resolve", {
        "blocker_id": args.blocker_id,
        "action": args.action,
        "decision_id": getattr(args, "decision_id", None),
    })


def _route_inbox(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("escalation.list", _selected_params(args, ("run_id", "state", "limit")))


def _route_escalation(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    sub = str(getattr(args, "escalation_command", "") or "")
    if sub == "list":
        params = _selected_params(args, ("run_id", "state", "limit"))
        return ("escalation.list", params)
    if sub == "show":
        return ("escalation.show", {"escalation_id": args.escalation_id})
    if sub == "resolve":
        params = {"escalation_id": args.escalation_id}
        if getattr(args, "decision_id", None):
            params["decision_id"] = args.decision_id
        if getattr(args, "resolution_note", None):
            params["resolution_note"] = args.resolution_note
        return ("escalation.resolve", params)
    raise StriatumError("escalation subcommand required", exit_code=2)


def _route_branch_confirm(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    return ("branch.confirm", {
        "run_id": args.run_id,
        "branch": args.branch,
        "create": bool(getattr(args, "create", False)),
        "use_current": bool(getattr(args, "use_current", False)),
        "strict": bool(getattr(args, "strict", False)),
    })


def _route_worktree(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    sub = str(getattr(args, "worktree_command", "") or "")
    if sub == "create":
        return (
            "worktree.create",
            {
                "session_id": args.session_id,
                "job_id": args.job_id,
                "lease_id": args.lease_id,
            },
        )
    if sub == "release":
        return ("worktree.release", {"worktree_id": args.worktree_id})
    if sub == "list":
        return ("worktree.list", {"run_id": getattr(args, "run_id", None)})
    raise StriatumError("worktree subcommand required", exit_code=2)


def _route_supervise(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
    sub = str(getattr(args, "supervise_command", "") or "")
    if sub == "start":
        return ("supervise.start", {"session_id": args.session_id})
    if sub == "send":
        return (
            "supervise.send",
            {"session_id": args.session_id, "packet_id": args.packet_id},
        )
    if sub == "stop":
        return (
            "supervise.stop",
            {"session_id": args.session_id, "reason": args.reason},
        )
    if sub == "status":
        return ("supervise.status", {"session_id": args.session_id})
    if sub == "list":
        return (
            "supervise.list",
            {
                "run_id": args.run_id,
                "state": getattr(args, "state", None),
            },
        )
    raise StriatumError("supervise subcommand required", exit_code=2)


# (command, subcommand) -> translator
_LOOKUP: dict[tuple[str, str | None], Callable[[argparse.Namespace, Path], tuple[str, dict[str, Any]]]] = {
    ("status", None): _route_status,
    ("why", None): _route_why,
    ("doctor", None): _route_doctor,
    ("dashboard", None): _route_dashboard,
    ("list", None): _route_list,
    ("run", "prepare"): _route_run_prepare,
    ("run", "start"): _route_run_start,
    ("run", "pause"): _route_run_pause,
    ("run", "resume"): _route_run_resume,
    ("run", "cancel"): _route_run_cancel,
    ("run", "retry-job"): _route_run_retry_job,
    ("run", "summary"): _route_run_summary,
    ("run", "graph"): _route_run_graph,
    ("register-session", None): _route_register_session,
    ("session", "close"): _route_session_close,
    ("claim-next", None): _route_claim_next,
    ("ack", None): _route_ack,
    ("heartbeat", None): _route_heartbeat,
    ("release", None): _route_release,
    ("send", None): _route_send,
    ("block", None): _route_block,
    ("complete", None): _route_complete,
    ("publish-artifact", None): _route_publish_artifact,
    ("verdict", None): _route_verdict,
    ("submit-review", None): _route_submit_review,
    ("override-verdict", None): _route_override_verdict,
    ("recovery", "stale-leases"): _route_recovery,
    ("recovery", "requeue-stale"): _route_recovery,
    ("recovery", "cancel-job"): _route_recovery,
    ("recovery", "process-reconcile"): _route_recovery,
    ("recovery", "resume"): _route_recovery,
    ("recovery", "auto"): _route_recovery,
    ("recovery", "auto-publish"): _route_recovery,
    ("recovery", "auto-finalize"): _route_recovery,
    ("recovery", "watch"): _route_recovery,
    ("evidence", "export"): _route_evidence_export,
    ("corpus", "export"): _route_corpus_export,
    ("archive", "create"): _route_archive_create,
    ("decision", "record"): _route_decision_record,
    ("checkpoint", "resolve"): _route_checkpoint_resolve,
    ("inbox", None): _route_inbox,
    ("escalation", "list"): _route_escalation,
    ("escalation", "show"): _route_escalation,
    ("escalation", "resolve"): _route_escalation,
    ("branch", "confirm"): _route_branch_confirm,
    ("worktree", "create"): _route_worktree,
    ("worktree", "release"): _route_worktree,
    ("worktree", "list"): _route_worktree,
    ("supervise", "start"): _route_supervise,
    ("supervise", "send"): _route_supervise,
    ("supervise", "stop"): _route_supervise,
    ("supervise", "status"): _route_supervise,
    ("supervise", "list"): _route_supervise,
}

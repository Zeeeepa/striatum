"""RFC 0048 Phase C — CLI dispatch routes through daemon RPC.

This module translates CLI argparse namespaces to daemon RPC envelopes
and dispatches them over the Unix socket. When a CLI command maps to a
registered RPC method, the call routes through the daemon RPC boundary
instead of any in-process state path.

The command-to-method table is declared in ``contracts/daemon_methods.json``
under ``cli_routes``. This module owns only CLI-local parameter extraction.
The hook in :func:`try_route` returns ``None`` to fall through when no RPC
mapping exists, which keeps bootstrap/admin and workflow-authoring surfaces
CLI-local.
"""

from __future__ import annotations

import argparse
import json
import os
import socket
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Mapping, cast

import striatum
from striatum.cli.daemon_required import daemon_socket_is_reachable, resolve_socket_path
from striatum.daemon_rpc.client import hello_envelope
from striatum.daemon_rpc.envelope import RpcEnvelope, RpcError
from striatum.daemon_rpc.registry import CONTRACT_PATH, METHOD_REGISTRY
from striatum.daemon_runtime import read_runtime_token
from striatum.db import json_loads
from striatum.errors import StriatumError

ParamBuilder = Callable[[argparse.Namespace, Path], dict[str, Any]]
RouteTranslator = Callable[[argparse.Namespace, Path], tuple[str, dict[str, Any]]]


@dataclass(frozen=True)
class _CliRouteContract:
    command: str
    subcommand: str | None
    method: str
    params: str


def try_route(args: argparse.Namespace, repo: Path) -> tuple[bool, Any]:
    """Route a CLI command through the daemon RPC if possible.

    Returns ``(True, response_data)`` if the command was routed through
    the daemon (success or RPC error). The caller should return that
    value as the CLI result. Returns ``(False, None)`` if no routing
    applies and dispatch should fall through to CLI-local handling.

    Routing applies when:
    - The daemon socket is reachable.
    - The CLI verb has a registered RPC method in this module's lookup
      table.
    - The repo (when relevant) resolves through daemon RPC.

    On RPC error (e.g., ``method_unknown``, ``capability_missing``), the
    function raises :class:`StriatumError` with an appropriate exit code.
    """
    if (
        os.environ.get("STRIATUM_TEST_HARNESS") == "1"
        and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"
    ):
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
    entry = METHOD_REGISTRY[method]
    repository_id: str | None = None
    if entry.repository_scope:
        try:
            repository_id = params.get("repository_id") or _resolve_repository_id(sock_path, repo)
        except OSError as exc:
            raise StriatumError(
                f"daemon_unreachable: repository resolution for {method} failed: {exc}",
                exit_code=12,
            ) from exc
        except RpcError as exc:
            raise _striatum_error_from_rpc_error(exc) from exc
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


def _striatum_error_from_rpc_error(error: RpcError) -> StriatumError:
    return StriatumError(f"{error.code}: {error}", exit_code=_rpc_error_exit_code(error.code))


def _rpc_error_exit_code(code: str) -> int:
    if code == "method_unknown":
        return 8
    if code in ("token_missing", "capability_missing", "version_incompatible"):
        return 14
    if code == "duplicate_request":
        return 9
    return 1


def _subcommand(args: argparse.Namespace) -> str | None:
    """Extract a verb's subcommand attribute (e.g., ``run_command`` for ``run``)."""
    for attr in ("run_command", "workflow_command", "list_command", "recovery_command",
                 "evidence_command", "corpus_command", "decision_command",
                 "checkpoint_command", "escalation_command", "branch_command", "session_command",
                 "repo_command", "worktree_command", "supervise_command", "cross_repo_command",
                 "archive_command"):
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
            client_version=striatum.__version__,
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


def _resolve_repository_id(sock_path: Path, repo: Path) -> str | None:
    """Resolve ``repo`` to a daemon repository_id over RPC.

    Cached per-process. Returns None only when the daemon reports no active
    registration for the repository. Transport and RPC failures propagate so
    daemon-routed callers fail closed instead of opening local state.
    """
    key = str(repo.resolve())
    if key in _REPO_ID_CACHE:
        return _REPO_ID_CACHE[key]
    repo_id = _resolve_repository_id_uncached(sock_path, key)
    if repo_id is not None:
        _REPO_ID_CACHE[key] = repo_id
    return repo_id


def _resolve_repository_id_uncached(sock_path: Path, repo_root: str) -> str | None:
    """Internal seam for the forthcoming ``repo.resolve`` contract method."""
    if "repo.resolve" in METHOD_REGISTRY:
        response = _call_with_handshake(sock_path, "repo.resolve", {"path": repo_root})
        data = _rpc_data_or_raise(response)
        return _repository_id_from_resolve_payload(data)
    response = _call_with_handshake(sock_path, "repo.list", {})
    data = _rpc_data_or_raise(response)
    return _repository_id_from_list_payload(data, repo_root)


def _rpc_data_or_raise(response: Mapping[str, Any]) -> Any:
    if bool(response.get("ok")):
        return response.get("data")
    raw_error = response.get("data")
    err: Mapping[str, Any] = raw_error if isinstance(raw_error, Mapping) else {}
    code = str(err.get("code") or "rpc_error")
    message = str(err.get("message") or "daemon RPC call failed")
    raw_details = err.get("details")
    details = dict(raw_details) if isinstance(raw_details, Mapping) else {}
    raise RpcError(code, message, details=details)


def _repository_id_from_resolve_payload(data: Any) -> str | None:
    if not isinstance(data, Mapping):
        return None
    repository_id = data.get("repository_id")
    if isinstance(repository_id, str) and repository_id:
        return repository_id
    repository = data.get("repository")
    if isinstance(repository, Mapping):
        repository_id = repository.get("repository_id")
        if isinstance(repository_id, str) and repository_id:
            return repository_id
    return None


def _repository_id_from_list_payload(data: Any, repo_root: str) -> str | None:
    if not isinstance(data, Mapping):
        return None
    raw_repositories = data.get("repositories")
    if not isinstance(raw_repositories, list):
        return None
    for item in raw_repositories:
        if not isinstance(item, Mapping):
            continue
        if item.get("repo_root") != repo_root or item.get("state") != "active":
            continue
        repository_id = item.get("repository_id")
        if isinstance(repository_id, str) and repository_id:
            return repository_id
    return None


# ----- Per-verb translators -----
# Each translator returns (rpc_method_name, params_dict).
# Daemon RPC params keys follow RFC 0030 envelope convention.


def _params_status(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    params: dict[str, Any] = {}
    if getattr(args, "run_id", None):
        params["run_id"] = args.run_id
    return params


def _params_why(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"target_id": args.id}


def _params_doctor(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"verbose": bool(getattr(args, "verbose", False))}


def _params_dashboard(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"run_id": getattr(args, "run_id", None)}


def _params_repo_add(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    params: dict[str, Any] = {
        "path": str(Path(args.path).expanduser()),
        "init": bool(getattr(args, "init", False)),
        "no_migrate": bool(getattr(args, "no_migrate", False)),
    }
    if getattr(args, "display_name", None):
        params["display_name"] = args.display_name
    return params


def _params_repo_list(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {}


def _params_repo_remove(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"id": str(args.id)}


def _params_list(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return _selected_params(
        args,
        ("run_id", "state", "role", "lane", "workflow_job_id", "kind", "limit"),
    )


def _selected_params(args: argparse.Namespace, names: tuple[str, ...]) -> dict[str, Any]:
    params: dict[str, Any] = {}
    for name in names:
        value = getattr(args, name, None)
        if value is not None:
            params[name] = value
    return params


def _params_run_prepare(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"workflow": args.workflow}


def _params_run_start(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"run_id": args.run_id}


def _params_run_pause(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"run_id": args.run_id, "reason": getattr(args, "reason", "")}


def _params_run_resume(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"run_id": args.run_id}


def _params_run_cancel(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"run_id": args.run_id, "reason": getattr(args, "reason", "")}


def _params_run_retry_job(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"run_id": args.run_id, "job_id": args.job_id}


def _params_run_summary(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"run_id": args.run_id, "path": getattr(args, "path", None)}


def _params_run_graph(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "run_id": args.run_id,
        "format": args.format,
        "graph_orient": getattr(args, "graph_orient", "tb"),
        "graph_style": getattr(args, "graph_style", "layered"),
    }


def _params_register_session(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
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
    return params


def _params_session_close(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"session_id": args.session_id, "reason": args.reason}


def _params_claim_next(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "lease_seconds": getattr(args, "lease_seconds", None),
    }


def _params_ack(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "message_id": args.message_id,
        "lease_id": args.lease_id,
    }


def _params_heartbeat(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "lease_id": args.lease_id,
        "extend_seconds": getattr(args, "extend_seconds", None),
    }


def _params_release(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "message_id": args.message_id,
        "lease_id": args.lease_id,
        "reason": getattr(args, "reason", ""),
        "requeue": bool(getattr(args, "requeue", False)),
    }


def _params_send(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "kind": args.kind,
        "body_json": args.body_json,
    }


def _params_block(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "kind": args.kind,
        "severity": args.severity,
        "description": args.description,
    }


def _params_complete(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "summary": getattr(args, "summary", ""),
    }


def _params_publish_artifact(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "kind": args.kind,
        "logical_name": args.logical_name,
        "path": args.path,
        "allow_no_process_execution": bool(getattr(args, "allow_no_process_execution", False)),
        "override_rationale": getattr(args, "override_rationale", None),
    }


def _params_verdict(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
        "verdict": args.verdict,
        "findings_artifact_id": getattr(args, "findings_artifact_id", None),
        "rationale": getattr(args, "rationale", ""),
    }


def _params_submit_review(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
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
    }


def _params_override_verdict(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "verdict": args.verdict,
        "rationale": args.rationale,
        "findings_artifact_id": getattr(args, "findings_artifact_id", None),
        "auto_fresh_session": bool(getattr(args, "auto_fresh_session", False)),
    }


def _params_recovery(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    recovery_command = str(getattr(args, "recovery_command", None) or "")
    if not recovery_command:
        raise StriatumError("recovery subcommand required", exit_code=2)
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
    return params


def _params_evidence_export(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "run_id": getattr(args, "run_id", None),
        "path": getattr(args, "path", None),
    }


def _params_corpus_export(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "since": args.since,
        "out": args.out,
    }


def _params_archive_create(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "run_id": args.run_id,
        "out": args.out,
    }


def _params_decision_record(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "run_id": args.run_id,
        "path": args.path,
        "title": args.title,
        "outcome": args.outcome,
        "decision_id": getattr(args, "decision_id", None),
        "rationale": getattr(args, "rationale", ""),
        "follow_up": getattr(args, "follow_up", None),
    }


def _params_checkpoint_resolve(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "blocker_id": args.blocker_id,
        "action": args.action,
        "decision_id": getattr(args, "decision_id", None),
    }


def _params_inbox(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return _selected_params(args, ("run_id", "state", "limit"))


def _params_escalation_list(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return _selected_params(args, ("run_id", "state", "limit"))


def _params_escalation_show(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"escalation_id": args.escalation_id}


def _params_escalation_resolve(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    params = {"escalation_id": args.escalation_id}
    if getattr(args, "decision_id", None):
        params["decision_id"] = args.decision_id
    if getattr(args, "resolution_note", None):
        params["resolution_note"] = args.resolution_note
    return params


def _params_branch_confirm(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "run_id": args.run_id,
        "branch": args.branch,
        "create": bool(getattr(args, "create", False)),
        "use_current": bool(getattr(args, "use_current", False)),
        "strict": bool(getattr(args, "strict", False)),
    }


def _params_worktree_create(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "session_id": args.session_id,
        "job_id": args.job_id,
        "lease_id": args.lease_id,
    }


def _params_worktree_release(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"worktree_id": args.worktree_id}


def _params_worktree_list(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"run_id": getattr(args, "run_id", None)}


def _params_supervise_start(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"session_id": args.session_id}


def _params_supervise_send(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"session_id": args.session_id, "packet_id": args.packet_id}


def _params_supervise_stop(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"session_id": args.session_id, "reason": args.reason}


def _params_supervise_status(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {"session_id": args.session_id}


def _params_supervise_list(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    return {
        "run_id": args.run_id,
        "state": getattr(args, "state", None),
    }


def _params_cross_repo(args: argparse.Namespace, repo: Path) -> dict[str, Any]:
    params: dict[str, Any] = {}
    if getattr(args, "cross_repo_run_id", None):
        params["cross_repo_run_id"] = str(args.cross_repo_run_id)
    if getattr(args, "cross_repo_command", None) == "cancel":
        params["reason"] = str(getattr(args, "reason", "") or "")
    return params


_PARAM_BUILDERS: dict[str, ParamBuilder] = {
    "status": _params_status,
    "why": _params_why,
    "doctor": _params_doctor,
    "dashboard": _params_dashboard,
    "repo_add": _params_repo_add,
    "repo_list": _params_repo_list,
    "repo_remove": _params_repo_remove,
    "list": _params_list,
    "run_prepare": _params_run_prepare,
    "run_start": _params_run_start,
    "run_pause": _params_run_pause,
    "run_resume": _params_run_resume,
    "run_cancel": _params_run_cancel,
    "run_retry_job": _params_run_retry_job,
    "run_summary": _params_run_summary,
    "run_graph": _params_run_graph,
    "register_session": _params_register_session,
    "session_close": _params_session_close,
    "claim_next": _params_claim_next,
    "ack": _params_ack,
    "heartbeat": _params_heartbeat,
    "release": _params_release,
    "send": _params_send,
    "block": _params_block,
    "complete": _params_complete,
    "publish_artifact": _params_publish_artifact,
    "verdict": _params_verdict,
    "submit_review": _params_submit_review,
    "override_verdict": _params_override_verdict,
    "recovery": _params_recovery,
    "evidence_export": _params_evidence_export,
    "corpus_export": _params_corpus_export,
    "archive_create": _params_archive_create,
    "decision_record": _params_decision_record,
    "checkpoint_resolve": _params_checkpoint_resolve,
    "inbox": _params_inbox,
    "escalation_list": _params_escalation_list,
    "escalation_show": _params_escalation_show,
    "escalation_resolve": _params_escalation_resolve,
    "branch_confirm": _params_branch_confirm,
    "worktree_create": _params_worktree_create,
    "worktree_release": _params_worktree_release,
    "worktree_list": _params_worktree_list,
    "supervise_start": _params_supervise_start,
    "supervise_send": _params_supervise_send,
    "supervise_stop": _params_supervise_stop,
    "supervise_status": _params_supervise_status,
    "supervise_list": _params_supervise_list,
    "cross_repo": _params_cross_repo,
}


def _load_cli_route_contracts(path: Path = CONTRACT_PATH) -> tuple[_CliRouteContract, ...]:
    try:
        loaded = json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise RuntimeError(f"daemon method contract not readable: {path}") from exc
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"daemon method contract is invalid JSON: {path}") from exc
    if not isinstance(loaded, dict):
        raise ValueError(f"daemon method contract must be a JSON object: {path}")
    raw_routes = loaded.get("cli_routes")
    if not isinstance(raw_routes, list):
        raise ValueError("daemon method contract cli_routes must be a list")

    routes: list[_CliRouteContract] = []
    seen: set[tuple[str, str | None]] = set()
    duplicates: list[str] = []
    for index, route_payload in enumerate(raw_routes):
        if not isinstance(route_payload, dict):
            raise ValueError(f"daemon CLI route contract entry {index} must be an object")
        route = _cli_route_from_contract(cast(Mapping[str, object], route_payload), index)
        key = (route.command, route.subcommand)
        if key in seen:
            duplicates.append(_format_route_key(key))
        seen.add(key)
        routes.append(route)

    if duplicates:
        raise ValueError(
            "daemon method contract contains duplicate CLI routes: "
            + ", ".join(sorted(duplicates))
        )
    return tuple(routes)


def _cli_route_from_contract(
    record: Mapping[str, object],
    index: int,
) -> _CliRouteContract:
    expected_fields = {"command", "subcommand", "method", "params"}
    fields = set(record)
    if fields != expected_fields:
        missing = sorted(expected_fields - fields)
        unexpected = sorted(fields - expected_fields)
        details: list[str] = []
        if missing:
            details.append("missing " + ", ".join(missing))
        if unexpected:
            details.append("unexpected " + ", ".join(unexpected))
        raise ValueError(
            f"daemon CLI route contract entry {index} has invalid fields: "
            + "; ".join(details)
        )

    command = _required_contract_str(record, "command", index)
    subcommand_value = record["subcommand"]
    if subcommand_value is not None and not isinstance(subcommand_value, str):
        raise ValueError(
            f"daemon CLI route contract entry {index} field 'subcommand' "
            "must be a string or null"
        )
    method = _required_contract_str(record, "method", index)
    params = _required_contract_str(record, "params", index)
    return _CliRouteContract(
        command=command,
        subcommand=subcommand_value,
        method=method,
        params=params,
    )


def _required_contract_str(record: Mapping[str, object], field: str, index: int) -> str:
    value = record[field]
    if not isinstance(value, str) or not value:
        raise ValueError(
            f"daemon CLI route contract entry {index} field {field!r} "
            "must be a non-empty string"
        )
    return value


def _format_route_key(key: tuple[str, str | None]) -> str:
    command, subcommand = key
    return command if subcommand is None else f"{command} {subcommand}"


def _translator_from_contract(route: _CliRouteContract) -> RouteTranslator:
    if route.method not in METHOD_REGISTRY:
        raise ValueError(
            f"daemon CLI route {_format_route_key((route.command, route.subcommand))!r} "
            f"references unregistered method {route.method!r}"
        )
    builder = _PARAM_BUILDERS.get(route.params)
    if builder is None:
        raise ValueError(
            f"daemon CLI route {_format_route_key((route.command, route.subcommand))!r} "
            f"references unknown params builder {route.params!r}"
        )

    def _translate(args: argparse.Namespace, repo: Path) -> tuple[str, dict[str, Any]]:
        if route.command == "dashboard" and bool(getattr(args, "all", False)):
            return ("dashboard.all", {})
        return (route.method, builder(args, repo))

    return _translate


def _lookup_from_contract(
    routes: tuple[_CliRouteContract, ...] | None = None,
) -> dict[tuple[str, str | None], RouteTranslator]:
    route_contracts = routes if routes is not None else _load_cli_route_contracts()
    return {
        (route.command, route.subcommand): _translator_from_contract(route)
        for route in route_contracts
    }


# (command, subcommand) -> translator. Derived from contracts/daemon_methods.json.
_LOOKUP: dict[tuple[str, str | None], RouteTranslator] = _lookup_from_contract()

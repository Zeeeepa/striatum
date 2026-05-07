"""Top-level CLI dispatch and error envelope handling."""

from __future__ import annotations

import argparse
import sqlite3
import sys
from pathlib import Path
from typing import Sequence

from striatum.artifacts import publish_artifact
from striatum.db import (
    claim_next,
    complete_job,
    connect,
    db_path,
    ensure_initialized,
    init_repo,
    json_dumps,
    transaction,
)
from striatum.errors import StriatumError
from striatum.process_adapter import run_process_adapter
from striatum.workflow import (
    create_run,
    load_workflow,
    plan_workflow,
    workflow_graph_data,
    workflow_graph_dot,
    workflow_graph_mermaid,
)

from striatum.cli.evidence import evidence_export
from striatum.cli.introspect import doctor, run_graph, status, why
from striatum.cli.list_commands import (
    list_artifacts,
    list_jobs,
    list_runs,
    list_sessions,
    list_workflows,
)
from striatum.cli.mutations import (
    ack_work,
    block_work,
    branch_confirm,
    checkpoint_resolve,
    decision_record,
    heartbeat,
    register_session,
    release_work,
    run_start,
    send_message,
    submit_review,
    verdict_work,
)
from striatum.cli.parser import build_parser
from striatum.cli.recovery import cancel_job, requeue_stale, stale_leases
from striatum.cli.run_summary import run_summary_export
from striatum.cli.supervise import (
    supervise_list,
    supervise_send,
    supervise_start,
    supervise_status,
    supervise_stop,
)
from striatum.cli.workflow_init import workflow_init
from striatum.cli.worktree import worktree_create, worktree_list, worktree_release


def main(argv: Sequence[str] | None = None) -> int:
    """Run the CLI."""
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        result = dispatch(args)
    except StriatumError as exc:
        if getattr(args, "json", False):
            print(json_dumps({"ok": False, "error": {"message": str(exc), "code": exc.exit_code}}))
        else:
            print(str(exc), file=sys.stderr)
        return exc.exit_code
    except sqlite3.Error as exc:
        if getattr(args, "json", False):
            print(json_dumps({"ok": False, "error": {"message": str(exc), "code": 1}}))
        else:
            print(str(exc), file=sys.stderr)
        return 1
    if result is not None:
        if getattr(args, "json", False) or isinstance(result, dict):
            print(json_dumps({"ok": True, "data": result}))
        else:
            print(result)
    return 0


def dispatch(args: argparse.Namespace) -> object:
    """Dispatch a parsed command."""
    repo = Path(args.repo).resolve()
    if args.command == "init":
        init_repo(repo)
        return {"state_dir": str(repo / ".striatum"), "db": str(db_path(repo))}
    if args.command == "workflow" and args.workflow_command == "validate":
        workflow = load_workflow(Path(args.path))
        return {"workflow_id": workflow["workflow_id"], "valid": True}
    if args.command == "workflow" and args.workflow_command == "plan":
        workflow = load_workflow(Path(args.path))
        return plan_workflow(workflow)
    if args.command == "workflow" and args.workflow_command == "graph":
        workflow = load_workflow(Path(args.path))
        if args.format == "json":
            return workflow_graph_data(workflow)
        if args.format == "dot":
            dot = workflow_graph_dot(workflow)
            if args.json:
                return {"format": "dot", "source": dot}
            return dot
        mermaid = workflow_graph_mermaid(workflow)
        if args.json:
            return {"format": "mermaid", "source": mermaid}
        return mermaid
    if args.command == "workflow" and args.workflow_command == "init":
        return workflow_init(Path(args.path), style=args.style)
    if args.command == "dashboard":
        from striatum.dashboard import run as run_dashboard

        run_dashboard(
            repo,
            run_id=args.run_id,
            refresh_seconds=float(args.refresh),
            once=bool(args.once),
        )
        return None
    ensure_initialized(repo)
    with connect(repo) as conn:
        if args.command == "run" and args.run_command == "prepare":
            with transaction(conn):
                return create_run(conn, repo=repo, workflow_path=Path(args.workflow))
        if args.command == "branch" and args.branch_command == "confirm":
            return branch_confirm(
                conn,
                repo=repo,
                run_id=args.run_id,
                branch=args.branch,
                create=args.create,
                use_current=args.use_current,
                strict=args.strict,
            )
        if args.command == "run" and args.run_command == "start":
            return run_start(conn, run_id=args.run_id)
        if args.command == "run" and args.run_command == "summary":
            return run_summary_export(conn, repo=repo, run_id=args.run_id, path_text=args.path)
        if args.command == "run" and args.run_command == "graph":
            result = run_graph(conn, run_id=args.run_id, output_format=args.format)
            if args.format == "mermaid" and isinstance(result, str) and args.json:
                return {"format": "mermaid", "source": result}
            return result
        if args.command == "register-session":
            return register_session(
                conn,
                run_id=args.run_id,
                role=args.role,
                lane=args.lane,
                capabilities=args.capability,
                fresh=args.fresh,
                parent_session_id=args.parent_session_id,
            )
        if args.command == "claim-next":
            return claim_next(
                conn,
                repo=repo,
                session_id=args.session_id,
                lease_seconds=args.lease_seconds,
            )
        if args.command == "ack":
            return ack_work(conn, session_id=args.session_id, message_id=args.message_id, lease_id=args.lease_id)
        if args.command == "heartbeat":
            return heartbeat(conn, session_id=args.session_id, lease_id=args.lease_id, extend_seconds=args.extend_seconds)
        if args.command == "release":
            return release_work(
                conn,
                session_id=args.session_id,
                message_id=args.message_id,
                lease_id=args.lease_id,
                reason=args.reason,
                requeue=args.requeue,
            )
        if args.command == "send":
            return send_message(conn, session_id=args.session_id, kind=args.kind, body_json=args.body_json)
        if args.command == "block":
            return block_work(
                conn,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                kind=args.kind,
                severity=args.severity,
                description=args.description,
            )
        if args.command == "publish-artifact":
            return publish_artifact(
                conn,
                repo=repo,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                kind=args.kind,
                logical_name=args.logical_name,
                path_text=args.path,
            )
        if args.command == "complete":
            return complete_job(
                conn,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                summary=args.summary,
            )
        if args.command == "verdict":
            return verdict_work(
                conn,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                verdict=args.verdict,
                findings_artifact_id=args.findings_artifact_id,
                rationale=args.rationale,
            )
        if args.command == "submit-review":
            return submit_review(
                conn,
                repo=repo,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                path_text=args.path,
                verdict=args.verdict,
                logical_name=args.logical_name,
                kind=args.kind,
                rationale=args.rationale,
            )
        if args.command == "evidence" and args.evidence_command == "export":
            return evidence_export(conn, repo=repo, run_id=args.run_id, path_text=args.path)
        if args.command == "decision" and args.decision_command == "record":
            return decision_record(
                conn,
                repo=repo,
                run_id=args.run_id,
                path_text=args.path,
                outcome=args.outcome,
                title=args.title,
                decision_id=args.decision_id,
                rationale=args.rationale,
                follow_up=args.follow_up,
            )
        if args.command == "status":
            return status(conn, run_id=args.run_id)
        if args.command == "why":
            return why(conn, target_id=args.id)
        if args.command == "doctor":
            return doctor(conn, repo=repo, run_id=args.run_id, verbose=args.verbose)
        if args.command == "recovery" and args.recovery_command == "stale-leases":
            return stale_leases(conn, run_id=args.run_id)
        if args.command == "recovery" and args.recovery_command == "requeue-stale":
            return requeue_stale(conn, run_id=args.run_id, job_id=args.job_id)
        if args.command == "recovery" and args.recovery_command == "cancel-job":
            return cancel_job(
                conn,
                run_id=args.run_id,
                job_id=args.job_id,
                reason=args.reason,
                cascade=bool(args.cascade),
            )
        if args.command == "checkpoint" and args.checkpoint_command == "resolve":
            return checkpoint_resolve(
                conn,
                blocker_id=args.blocker_id,
                action=args.action,
                decision_id=args.decision_id,
            )
        if args.command == "adapter" and args.adapter_command == "run":
            return run_process_adapter(
                conn,
                repo=repo,
                session_id=args.session_id,
                lease_id=args.lease_id,
                stdin_mode=args.stdin,
                inherit_stdio=args.inherit_stdio,
            )
        if args.command == "worktree" and args.worktree_command == "create":
            return worktree_create(
                conn,
                repo=repo,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
            )
        if args.command == "worktree" and args.worktree_command == "release":
            return worktree_release(conn, repo=repo, worktree_id=args.worktree_id)
        if args.command == "worktree" and args.worktree_command == "list":
            return worktree_list(conn, run_id=args.run_id)
        if args.command == "supervise" and args.supervise_command == "start":
            return supervise_start(conn, repo=repo, session_id=args.session_id)
        if args.command == "supervise" and args.supervise_command == "send":
            return supervise_send(conn, session_id=args.session_id, packet_id=args.packet_id)
        if args.command == "supervise" and args.supervise_command == "stop":
            return supervise_stop(conn, session_id=args.session_id, reason=args.reason)
        if args.command == "supervise" and args.supervise_command == "status":
            return supervise_status(conn, session_id=args.session_id)
        if args.command == "supervise" and args.supervise_command == "list":
            return supervise_list(conn, run_id=args.run_id, state=args.state)
        if args.command == "list" and args.list_command == "runs":
            return list_runs(conn, state=args.state, limit=args.limit)
        if args.command == "list" and args.list_command == "sessions":
            return list_sessions(
                conn,
                run_id=args.run_id,
                state=args.state,
                role=args.role,
                lane=args.lane,
            )
        if args.command == "list" and args.list_command == "jobs":
            return list_jobs(
                conn,
                run_id=args.run_id,
                state=args.state,
                workflow_job_id=args.workflow_job_id,
            )
        if args.command == "list" and args.list_command == "artifacts":
            return list_artifacts(conn, run_id=args.run_id, kind=args.kind)
        if args.command == "list" and args.list_command == "workflows":
            return list_workflows(conn, limit=args.limit)
    raise StriatumError("unknown command", exit_code=2)

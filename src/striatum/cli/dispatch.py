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
    json_loads,
    transaction,
)
from striatum.errors import InvalidTransitionError, StriatumError
from striatum.process_adapter import run_process_adapter
from striatum.workflow import (
    create_run,
    load_workflow,
    plan_workflow,
    validate_workflow,
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
    close_session,
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
from striatum.cli.recovery import (
    cancel_job,
    process_reconcile,
    requeue_stale,
    stale_leases,
)
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


def _skills_install_dispatch(
    *,
    target: Path,
    profile: str,
    scope: str,
    namespace: str,
    force: bool,
    dry_run: bool,
) -> dict[str, object]:
    """Run ``skills install`` against one profile, or fan out across all.

    ``profile == "all"`` calls ``install(...)`` once per first-class
    profile in :data:`striatum.skills.ALL_PROFILES_ORDER` and returns
    ``{"profile": "all", "scope": ..., "namespace": ..., "results": [...]}``.
    """
    from striatum.skills import install as skills_install
    from striatum.skills.install import ALL_PROFILES_ORDER

    if profile != "all":
        return skills_install(
            target=target,
            profile=profile,
            scope=scope,
            namespace=namespace,
            force=force,
            dry_run=dry_run,
        )
    results: list[dict[str, object]] = []
    for sub_profile in ALL_PROFILES_ORDER:
        results.append(
            skills_install(
                target=target,
                profile=sub_profile,
                scope=scope,
                namespace=namespace,
                force=force,
                dry_run=dry_run,
            )
        )
    return {
        "profile": "all",
        "scope": scope,
        "namespace": namespace,
        "results": results,
        "dry_run": dry_run,
    }


def dispatch(args: argparse.Namespace) -> object:
    """Dispatch a parsed command."""
    repo = Path(args.repo).resolve()
    if args.command == "init":
        init_repo(repo)
        init_result: dict[str, object] = {
            "state_dir": str(repo / ".striatum"),
            "db": str(db_path(repo)),
        }
        with_skills = getattr(args, "with_skills", None)
        if with_skills is not None:
            init_result["skills"] = _skills_install_dispatch(
                target=repo,
                profile=str(with_skills),
                scope="project",
                namespace="striatum-",
                force=False,
                dry_run=False,
            )
        with_plugins = getattr(args, "with_plugins", None)
        if with_plugins is not None:
            from striatum.plugins import install as plugin_install
            init_result["plugins"] = plugin_install.install(
                target=repo,
                profile=str(with_plugins),
                scope="project",
                namespace="striatum",
                force=False,
                dry_run=False,
            )
        if getattr(args, "with_ddd_layout", False):
            from striatum.scaffold import scaffold_ddd_layout
            init_result["ddd_layout"] = scaffold_ddd_layout(
                repo,
                force=bool(getattr(args, "ddd_layout_force", False)),
                dry_run=bool(getattr(args, "ddd_layout_dry_run", False)),
            )
        return init_result
    if args.command == "skills" and args.skills_command == "install":
        return _skills_install_dispatch(
            target=repo,
            profile=str(args.profile),
            scope=str(args.scope),
            namespace=str(args.namespace),
            force=bool(args.force),
            dry_run=bool(args.dry_run),
        )
    if args.command == "plugin" and args.plugin_command == "install":
        from striatum.plugins import install as plugin_install
        target_arg = args.target if getattr(args, "target", None) else repo
        return plugin_install.install(
            target=Path(target_arg),
            profile=str(args.profile),
            scope=str(args.scope),
            namespace=str(args.namespace),
            force=bool(args.force),
            dry_run=bool(args.dry_run),
            with_marketplace=bool(args.with_marketplace),
        )
    if args.command == "plugin" and args.plugin_command == "uninstall":
        from striatum.plugins import install as plugin_install
        target_arg = args.target if getattr(args, "target", None) else repo
        return plugin_install.uninstall(
            target=Path(target_arg),
            profile=str(args.profile),
            scope=str(args.scope),
            namespace=str(args.namespace),
            force=bool(args.force),
        )
    if args.command == "workflow" and args.workflow_command == "validate":
        workflow = load_workflow(Path(args.path))
        warnings: list[str] = []
        validate_workflow(workflow, warnings=warnings, repo_root=repo)
        validation_result: dict[str, object] = {
            "workflow_id": workflow["workflow_id"],
            "valid": True,
        }
        if warnings:
            validation_result["warnings"] = warnings
        return validation_result
    if args.command == "workflow" and args.workflow_command == "plan":
        workflow = load_workflow(Path(args.path))
        return plan_workflow(workflow, repo_root=repo)
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
            graph=getattr(args, "graph", None),
            graph_only=bool(getattr(args, "graph_only", False)),
            graph_style=str(getattr(args, "graph_style", "auto")),
            graph_no_cycles=bool(getattr(args, "graph_no_cycles", False)),
            graph_orient=str(getattr(args, "graph_orient", "tb")),
        )
        return None
    if args.command == "serve":
        from striatum.errors import StriatumError
        from striatum.service import (
            ServiceAlreadyRunningError,
            ServiceConfigError,
            run_service,
        )

        if args.unix is not None and (args.host is not None or args.port is not None):
            raise StriatumError(
                "--unix and --host/--port are mutually exclusive",
                exit_code=8,
            )
        try:
            return run_service(
                repo=repo,
                host=args.host,
                port=args.port,
                unix_path=args.unix,
                token=args.token,
                allow_mutations=bool(args.allow_mutations),
                idle_timeout_seconds=args.idle_timeout_seconds,
                web_enabled=bool(args.web),
            )
        except ServiceConfigError as exc:
            raise StriatumError(str(exc), exit_code=8) from exc
        except ServiceAlreadyRunningError as exc:
            raise StriatumError(str(exc), exit_code=7) from exc
    ensure_initialized(repo)
    with connect(repo) as conn:
        if args.command == "run" and args.run_command == "prepare":
            with transaction(conn):
                prepared = create_run(
                    conn, repo=repo, workflow_path=Path(args.workflow)
                )
            # Auto-branch mode (RFC 0010 follow-up): when the workflow's
            # branch.mode is "auto", drive `branch confirm --create`
            # implicitly so operators don't have to type a separate step.
            # Falls back to `needs_branch_confirmation` if git checkout
            # fails (dirty tree, conflicting branch); the operator can
            # then resolve the issue and run `branch confirm` manually.
            if prepared.get("branch_mode") == "auto":
                suggested = prepared.get("suggested_branch_name")
                if isinstance(suggested, str) and suggested:
                    confirmed = branch_confirm(
                        conn,
                        repo=repo,
                        run_id=str(prepared["run_id"]),
                        branch=suggested,
                        create=True,
                    )
                    return {
                        "run_id": prepared["run_id"],
                        "state": confirmed["state"],
                        "branch": confirmed["branch"],
                        "branch_mode": "auto",
                        "branch_created": confirmed.get("created", False),
                        "current_git_branch": confirmed.get("current_git_branch"),
                        "warning": confirmed.get("warning"),
                    }
            return prepared
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
        if args.command == "run" and args.run_command == "cancel":
            from striatum.db import cancel_run as _cancel_run
            with transaction(conn):
                return _cancel_run(conn, run_id=args.run_id, reason=args.reason)
        if args.command == "run" and args.run_command == "pause":
            from striatum.db import pause_run as _pause_run
            with transaction(conn):
                return _pause_run(conn, run_id=args.run_id, reason=args.reason)
        if args.command == "run" and args.run_command == "resume":
            from striatum.db import resume_run as _resume_run
            with transaction(conn):
                return _resume_run(conn, run_id=args.run_id)
        if args.command == "run" and args.run_command == "retry-job":
            from striatum.db import retry_job as _retry_job
            with transaction(conn):
                return _retry_job(conn, run_id=args.run_id, job_id=args.job_id)
        if args.command == "run" and args.run_command == "summary":
            return run_summary_export(conn, repo=repo, run_id=args.run_id, path_text=args.path)
        if args.command == "run" and args.run_command == "graph":
            result = run_graph(
                conn,
                run_id=args.run_id,
                output_format=args.format,
                graph_orient=str(getattr(args, "graph_orient", "tb")),
                graph_style=str(getattr(args, "graph_style", "layered")),
            )
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
                force_non_fresh=args.force_non_fresh,
                non_fresh_reason=args.reason,
            )
        if args.command == "session" and args.session_command == "close":
            return close_session(
                conn,
                session_id=args.session_id,
                reason=args.reason,
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
        if args.command == "recovery" and args.recovery_command == "process-reconcile":
            return process_reconcile(conn, run_id=args.run_id)
        if args.command == "recovery" and args.recovery_command == "auto":
            from striatum.recovery import resolve_policy, run_auto_sweep

            run_row = conn.execute(
                "SELECT workflow_snapshot_id FROM runs WHERE run_id = ?",
                (args.run_id,),
            ).fetchone()
            workflow_payload = None
            if run_row is not None:
                snap = conn.execute(
                    "SELECT workflow_json FROM workflow_snapshots "
                    "WHERE workflow_snapshot_id = ?",
                    (str(run_row["workflow_snapshot_id"]),),
                ).fetchone()
                if snap is not None:
                    try:
                        wf = json_loads(str(snap["workflow_json"]))
                    except Exception:  # noqa: BLE001
                        wf = {}
                    if isinstance(wf, dict):
                        workflow_payload = wf.get("recovery_policy")
            cli_overrides = {
                "autonomous_review_requeue": getattr(
                    args, "autonomous_review_requeue", None
                ),
                "autonomous_process_reconcile": getattr(
                    args, "autonomous_process_reconcile", None
                ),
                "max_requeues_per_sweep": getattr(
                    args, "max_requeues_per_sweep", None
                ),
                "checkpoint_timeout_seconds": getattr(
                    args, "checkpoint_timeout_seconds", None
                ),
                "eligible_after_seconds": getattr(
                    args, "eligible_after_seconds", None
                ),
            }
            policy = resolve_policy(
                workflow_payload=workflow_payload, cli_overrides=cli_overrides
            )
            return run_auto_sweep(
                conn,
                run_id=args.run_id,
                repo=repo,
                policy=policy,
                dry_run=bool(args.dry_run),
            )
        if args.command == "recovery" and args.recovery_command == "watch":
            from striatum.recovery import run_watch

            cli_overrides = {
                "autonomous_review_requeue": getattr(
                    args, "autonomous_review_requeue", None
                ),
                "autonomous_process_reconcile": getattr(
                    args, "autonomous_process_reconcile", None
                ),
                "max_requeues_per_sweep": getattr(
                    args, "max_requeues_per_sweep", None
                ),
                "checkpoint_timeout_seconds": getattr(
                    args, "checkpoint_timeout_seconds", None
                ),
                "eligible_after_seconds": getattr(
                    args, "eligible_after_seconds", None
                ),
            }
            exit_code = run_watch(
                repo=repo,
                run_id=args.run_id,
                interval_seconds=float(args.interval_seconds),
                exit_on_terminal=bool(args.exit_on_terminal),
                max_sweeps=getattr(args, "max_sweeps", None),
                cli_overrides=cli_overrides,
                json_output=bool(args.json),
            )
            if exit_code != 0:
                # Pidfile collision is the only non-zero exit shape today;
                # InvalidTransitionError uses exit code 4 by construction.
                raise InvalidTransitionError(
                    f"recovery watch refused (exit {exit_code})"
                )
            return None
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
                timeout_seconds=args.timeout_seconds,
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

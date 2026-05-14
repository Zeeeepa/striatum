"""Top-level CLI dispatch and error envelope handling."""

from __future__ import annotations

import argparse
import json
import os
import sqlite3
import sys
from pathlib import Path
from typing import Any, Sequence

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
from striatum.cli.daemon_required import enforce_daemon_required
from striatum.errors import (
    DaemonUnreachableError,
    InvalidTransitionError,
    RepoNotMigratedError,
    StriatumError,
)
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
    resume_blocker,
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
from striatum.cli.workflow import workflow_upgrade
from striatum.cli.workflow_init import workflow_init
from striatum.cli.worktree import worktree_create, worktree_list, worktree_release


def main(argv: Sequence[str] | None = None) -> int:
    """Run the CLI."""
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        result = dispatch(args)
    except (DaemonUnreachableError, RepoNotMigratedError) as exc:
        # RFC 0043 §3 refusals get the multi-line stderr remediation block
        # in human mode and the structured envelope (with ``hint``) in JSON
        # mode. The constructed message already carries the block.
        if getattr(args, "json", False):
            error_envelope: dict[str, object] = {
                "message": str(exc).splitlines()[0],
                "code": exc.exit_code,
            }
            hint = getattr(exc, "hint", None)
            if isinstance(hint, str):
                error_envelope["hint"] = hint
            print(json_dumps({"ok": False, "error": error_envelope}))
        else:
            print(str(exc), file=sys.stderr)
        return exc.exit_code
    except StriatumError as exc:
        if getattr(args, "json", False):
            error: dict[str, object] = {"message": str(exc), "code": exc.exit_code}
            field_path = getattr(exc, "field_path", None)
            if isinstance(field_path, str):
                error["field_path"] = field_path
            hint = getattr(exc, "hint", None)
            if isinstance(hint, str):
                error["hint"] = hint
            ref = getattr(exc, "ref", None)
            if isinstance(ref, str):
                error["ref"] = ref
            print(json_dumps({"ok": False, "error": error}))
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
    # RFC 0043 §3 (V1.5): daemon-required enforcement is the default. Fail
    # fast with exit code 11 (daemon socket unreachable) or 12 (repo not
    # migrated) before the SQLite-backed fallback is touched. The
    # ``STRIATUM_DAEMON_REQUIRED=0`` opt-out exists for SQLite-backed
    # test fixtures and incremental legacy-fixture migration.
    enforce_daemon_required(getattr(args, "command", None), repo)
    daemon_forced = bool(getattr(args, "daemon", False)) or (
        os.environ.get("STRIATUM_DAEMON") == "1"
    )
    # RFC 0048 Phase C: route CLI verbs through daemon RPC (Unix socket)
    # when the verb maps to a registered RPC method AND the daemon is
    # reachable. Falls through to legacy SQLite dispatch when no mapping
    # exists (init, skills, plugin, daemon, repo, serve, byline) or when
    # the daemon is unreachable (test-harness mode or daemon offline).
    if args.command not in {"daemon", "init", "skills", "plugin", "repo", "cross-repo", "serve", "byline", "inbox"}:
        try:
            from striatum.cli.daemon_rpc_route import try_route as _try_route_via_daemon
            routed, payload = _try_route_via_daemon(args, repo)
            if routed:
                return payload
        except StriatumError:
            raise
        except Exception:  # noqa: BLE001 — any unexpected failure falls through to legacy path
            pass
    if args.command == "daemon":
        return _dispatch_daemon(args)
    if args.command == "repo":
        return _dispatch_daemon_repo(args)
    if args.command == "cross-repo":
        return _dispatch_cross_repo(args)
    if daemon_forced and args.command in {"status", "why", "doctor", "dashboard"}:
        return _dispatch_daemon_read(args, repo)
    if daemon_forced:
        # The legacy V1 --daemon flag only routes the four read verbs above.
        # Any other command under --daemon is a routing gap, not the new
        # RFC 0043 §3 refusal — surface it as WorkflowError (exit code 8)
        # so the new exit-code-12 (repo_not_migrated) semantic stays clean.
        raise StriatumError(
            f"--daemon does not yet route `{args.command}`; "
            "set STRIATUM_DAEMON_REQUIRED=1 once the daemon-mediated path lands",
            exit_code=8,
        )
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
        from striatum.plugins.install import ALL_PROFILES_ORDER as _PLUGIN_ALL
        target_arg = args.target if getattr(args, "target", None) else repo
        if str(args.profile) == "all":
            install_results: list[dict[str, Any]] = []
            for prof in _PLUGIN_ALL:
                install_results.append(plugin_install.install(
                    target=Path(target_arg),
                    profile=prof,
                    scope=str(args.scope),
                    namespace=str(args.namespace),
                    force=bool(args.force),
                    dry_run=bool(args.dry_run),
                    with_marketplace=bool(args.with_marketplace),
                ))
            return {"profile": "all", "results": install_results}
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
        from striatum.plugins.install import ALL_PROFILES_ORDER as _PLUGIN_ALL
        target_arg = args.target if getattr(args, "target", None) else repo
        if str(args.profile) == "all":
            uninstall_results: list[dict[str, Any]] = []
            for prof in _PLUGIN_ALL:
                uninstall_results.append(plugin_install.uninstall(
                    target=Path(target_arg),
                    profile=prof,
                    scope=str(args.scope),
                    namespace=str(args.namespace),
                    force=bool(args.force),
                ))
            return {"profile": "all", "results": uninstall_results}
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
    if args.command == "workflow" and args.workflow_command == "upgrade":
        return workflow_upgrade(
            Path(args.path),
            repo=repo,
            force=bool(args.force),
            dry_run=bool(args.dry_run),
            add_phases=bool(args.add_phases),
            apply=bool(args.apply),
        )
    if args.command == "workflow" and args.workflow_command == "templates":
        from striatum.workflow_generator.catalog import get_template, list_templates

        if args.templates_command == "list":
            return {"templates": list_templates(kind=args.kind)}
        if args.templates_command == "show":
            return get_template(str(args.template_id))
    if args.command == "workflow" and args.workflow_command == "generate":
        return _workflow_generate(args, repo)
    if args.command == "dashboard":
        if bool(getattr(args, "all", False)):
            from striatum.daemon import dashboard_all

            return dashboard_all()
        if not args.run_id:
            raise StriatumError("dashboard requires --run-id unless --all is used", exit_code=2)
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
                operator_label=args.operator_label,
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
            # V1.41: default --kind and --logical-name from the workflow's
            # expected_artifacts when --path matches a declared artifact
            # path.
            kind, logical_name = _resolve_publish_defaults(
                conn,
                job_id=args.job_id,
                kind=args.kind,
                logical_name=args.logical_name,
                path_text=args.path,
            )
            allow_no_proc = bool(getattr(args, "allow_no_process_execution", False))
            override_rationale = getattr(args, "override_rationale", None)
            # RFC 0046 V1: explicit refusal at the CLI boundary when
            # --allow-no-process-execution lands without a non-empty
            # --override-rationale. Exit code 2 (invalid args) per
            # argparse convention, before the publish-artifact write
            # transaction opens.
            if allow_no_proc and not (
                override_rationale and override_rationale.strip()
            ):
                raise StriatumError(
                    "publish-artifact --allow-no-process-execution "
                    "requires a non-empty --override-rationale",
                    exit_code=2,
                )
            return publish_artifact(
                conn,
                repo=repo,
                session_id=args.session_id,
                job_id=args.job_id,
                lease_id=args.lease_id,
                kind=kind,
                logical_name=logical_name,
                path_text=args.path,
                allow_no_process_execution=allow_no_proc,
                override_rationale=override_rationale,
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
        if args.command == "override-verdict":
            from striatum.db import override_review_verdict

            # V1.41: --auto-fresh-session takes the operator's named
            # session and, if it already has a verdict for this job
            # (so override-verdict would refuse), registers a fresh
            # operator reviewer session on the same lane and uses it.
            # Removes the two-step "register fresh, then override"
            # dance that operator-on-behalf flows have required since
            # dogfood-049.
            session_id = args.session_id
            if getattr(args, "auto_fresh_session", False):
                session_id = _resolve_override_session(
                    conn,
                    requested_session_id=args.session_id,
                    job_id=args.job_id,
                    rationale=args.rationale,
                )
            return override_review_verdict(
                conn,
                session_id=session_id,
                job_id=args.job_id,
                verdict=args.verdict,
                rationale=args.rationale,
                findings_artifact_id=args.findings_artifact_id,
            )
        if args.command == "submit-review":
            allow_no_proc = bool(getattr(args, "allow_no_process_execution", False))
            override_rationale = getattr(args, "override_rationale", None)
            if allow_no_proc and not (
                override_rationale and override_rationale.strip()
            ):
                raise StriatumError(
                    "submit-review --allow-no-process-execution "
                    "requires a non-empty --override-rationale",
                    exit_code=2,
                )
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
                allow_no_process_execution=allow_no_proc,
                override_rationale=override_rationale,
            )
        if args.command == "evidence" and args.evidence_command == "export":
            return evidence_export(conn, repo=repo, run_id=args.run_id, path_text=args.path)
        if args.command == "corpus" and args.corpus_command == "export":
            from striatum.corpus import export_corpus_bundle

            return export_corpus_bundle(
                conn,
                repo=repo,
                since=args.since,
                out_text=args.out,
            )
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
        if args.command == "recovery" and args.recovery_command == "resume":
            return resume_blocker(
                conn,
                blocker_id=args.blocker_id,
                complete=bool(args.complete),
                session_id=args.session_id,
                summary=args.summary,
                force=bool(args.force),
                extend_seconds=int(args.extend_seconds),
            )
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
        if args.command == "recovery" and args.recovery_command == "auto-publish":
            from striatum.cli.recovery import auto_publish_stale_artifacts

            return auto_publish_stale_artifacts(
                conn,
                repo=repo,
                run_id=args.run_id,
                dry_run=bool(args.dry_run),
            )
        if args.command == "byline":
            return _cli_byline(conn, session_id=args.session_id, job_id=args.job_id)
        if args.command == "inbox":
            return _cli_inbox(conn, session_id=args.session_id)
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


def _workflow_generate(args: argparse.Namespace, repo: Path) -> object:
    from striatum.workflow_generator import generate_workflow
    from striatum.workflow_generator.core import default_spec, slugify
    from striatum.workflow_generator.write import write_generated_workflow

    lane_commands = _parse_keyed_json_arrays(args.lane_command, "lane-command")
    lane_models = _parse_keyed_strings(args.lane_display_model, "lane-display-model")
    lanes: dict[str, dict[str, object]] = {}
    for lane_id, command in lane_commands.items():
        lanes.setdefault(lane_id, {})["command"] = command
    for lane_id, display_model in lane_models.items():
        lanes.setdefault(lane_id, {})["display_model"] = display_model
    plan = None
    if args.plan:
        loaded = json.loads(Path(args.plan).read_text(encoding="utf-8"))
        if not isinstance(loaded, dict):
            from striatum.workflow_generator import GeneratorError

            raise GeneratorError("--plan must point to a JSON object", field_path="spec.plan")
        plan = loaded
    workflow_id = args.workflow_id or slugify(Path(args.path).name)
    spec = default_spec(
        scaffold_root=str(args.path).rstrip("/"),
        artifact_root=str(args.artifact_root).rstrip("/"),
        shape=str(args.shape),
        lane_set=str(args.lane_set),
        workflow_id=workflow_id,
        name=args.name or workflow_id.replace("-", " ").title(),
        workflow_version=args.workflow_version,
        branch_suggestion=args.branch_suggestion,
        lanes={key: dict(value) for key, value in lanes.items()},
        lane_modifiers=list(args.lane_modifier or []),
        options=_parse_options(args.option),
        plan=plan,
    )
    generated = generate_workflow(spec)
    if args.dry_run:
        return generated.to_json()
    return write_generated_workflow(generated, repo=repo)


def _parse_keyed_json_arrays(values: list[str], flag: str) -> dict[str, list[str]]:
    from striatum.workflow_generator import GeneratorError

    parsed: dict[str, list[str]] = {}
    for raw in values:
        if "=" not in raw:
            raise GeneratorError(f"--{flag} must be <lane_id>=<json-array>", field_path=f"cli.{flag}")
        key, value = raw.split("=", 1)
        loaded = json.loads(value)
        if not isinstance(loaded, list) or not all(isinstance(item, str) for item in loaded) or not loaded:
            raise GeneratorError(
                f"--{flag} value must be a non-empty JSON string array",
                field_path=f"cli.{flag}.{key}",
            )
        parsed[key] = loaded
    return parsed


def _parse_keyed_strings(values: list[str], flag: str) -> dict[str, str]:
    from striatum.workflow_generator import GeneratorError

    parsed: dict[str, str] = {}
    for raw in values:
        if "=" not in raw:
            raise GeneratorError(f"--{flag} must be <lane_id>=<value>", field_path=f"cli.{flag}")
        key, value = raw.split("=", 1)
        if not key or not value:
            raise GeneratorError(f"--{flag} requires non-empty key and value", field_path=f"cli.{flag}")
        parsed[key] = value
    return parsed


def _parse_options(values: list[str]) -> dict[str, object]:
    from striatum.workflow_generator import GeneratorError

    options: dict[str, object] = {}
    for raw in values:
        if "=" not in raw:
            raise GeneratorError("--option must be <dotted.key>=<json-value>", field_path="cli.option")
        key, value = raw.split("=", 1)
        loaded = json.loads(value)
        parts = key.split(".")
        if any(part == "" for part in parts):
            raise GeneratorError("--option keys must be non-empty", field_path="cli.option")
        target = options
        for part in parts[:-1]:
            nested = target.setdefault(part, {})
            if not isinstance(nested, dict):
                raise GeneratorError(
                    "--option path conflicts with an existing value",
                    field_path=f"cli.option.{key}",
                )
            target = nested
        target[parts[-1]] = loaded
    return options


def _dispatch_daemon(args: argparse.Namespace) -> object:
    from striatum import daemon as daemon_mod

    if args.daemon_command == "migrate-repo-local":
        from striatum.cli.daemon import dispatch_daemon

        return dispatch_daemon(args)
    if args.daemon_command == "start":
        from striatum.cli.daemon import launch_daemon_start

        return launch_daemon_start(args)
    if args.daemon_command == "doctor":
        from striatum.daemon_pg.connection import doctor as pg_doctor

        pg = pg_doctor(
            postgres_url=getattr(args, "postgres_url", None),
            apply=bool(getattr(args, "apply_migrations", False)),
        )
        try:
            v1 = daemon_mod.read_doctor(repo=None, verbose=True)
        except Exception as exc:  # noqa: BLE001 - daemon doctor must still report PG onboarding.
            v1 = {"ok": False, "error": str(exc)}
        return {"mode": "daemon", "postgres": pg, "sqlite_registry": v1}
    if args.daemon_command == "migrate":
        from striatum.daemon_pg.config import resolve_config
        from striatum.daemon_pg.cutover import CutoverOptions, migrate

        config = resolve_config(postgres_url=getattr(args, "postgres_url", None))
        if config.url is None:
            raise StriatumError("daemon PostgreSQL URL is not configured", exit_code=13)
        source = Path(args.source_registry) if args.source_registry else daemon_mod.registry_path()
        return migrate(
            CutoverOptions(
                source_registry=source,
                postgres_url=config.url,
                dry_run=bool(args.dry_run),
                keep_sqlite_readonly=bool(args.keep_sqlite_readonly),
            )
        )
    if args.daemon_command == "status":
        return daemon_mod.daemon_status()
    if args.daemon_command == "stop":
        return daemon_mod.daemon_stop()
    if args.daemon_command == "health":
        return daemon_mod.health()
    if args.daemon_command == "audit":
        return daemon_mod.daemon_audit(limit=int(args.limit))
    if args.daemon_command == "sweep":
        return daemon_mod.daemon_sweep_once(require_client_auth=True)
    raise StriatumError("unknown daemon command", exit_code=2)


def _dispatch_daemon_repo(args: argparse.Namespace) -> object:
    from striatum import daemon as daemon_mod

    if args.repo_command == "add":
        return daemon_mod.repo_add(
            Path(args.path),
            display_name=args.display_name,
            no_migrate=bool(args.no_migrate),
            init=bool(args.init),
        )
    if args.repo_command == "list":
        return daemon_mod.repo_list()
    if args.repo_command == "remove":
        return daemon_mod.repo_remove(str(args.id))
    raise StriatumError("unknown repo command", exit_code=2)


def _dispatch_cross_repo(args: argparse.Namespace) -> object:
    from striatum.cross_repo import describe_cross_repo_run, list_cross_repo_runs
    from striatum.daemon_pg.connection import connect_and_migrate

    conn = connect_and_migrate(postgres_url=getattr(args, "postgres_url", None))
    try:
        if args.cross_repo_command == "list":
            return list_cross_repo_runs(conn)
        if args.cross_repo_command == "describe":
            return describe_cross_repo_run(
                conn, cross_repo_run_id=str(args.cross_repo_run_id)
            )
        if args.cross_repo_command == "why":
            described = describe_cross_repo_run(
                conn, cross_repo_run_id=str(args.cross_repo_run_id)
            )
            return {
                "cross_repo_run_id": args.cross_repo_run_id,
                "state": described["state"],
                "participants": described["participants"],
            }
        if args.cross_repo_command == "cancel":
            raise StriatumError(
                "cross-repo cancel requires the daemon lifecycle service; "
                "the full multi-repo daemon harness is deferred to TODO Open item 19",
                exit_code=8,
            )
    finally:
        close = getattr(conn, "close", None)
        if callable(close):
            close()
    raise StriatumError("unknown cross-repo command", exit_code=2)


def _dispatch_daemon_read(args: argparse.Namespace, repo: Path) -> object:
    from striatum import daemon as daemon_mod

    if args.command == "status":
        return daemon_mod.read_status(repo, run_id=args.run_id)
    if args.command == "why":
        return daemon_mod.read_why(repo, target_id=args.id)
    if args.command == "doctor":
        return daemon_mod.read_doctor(repo, run_id=args.run_id, verbose=bool(args.verbose))
    if args.command == "dashboard":
        if bool(getattr(args, "all", False)):
            return daemon_mod.dashboard_all()
        if not args.run_id:
            raise StriatumError("dashboard requires --run-id unless --all is used", exit_code=2)
        raise StriatumError(
            "--daemon dashboard --run-id is not a V1 daemon route; "
            "drop --daemon for the dashboard verb or use --daemon status --run-id",
            exit_code=8,
        )
    raise StriatumError("command is not daemon-routable in V1", exit_code=8)


def _resolve_publish_defaults(
    conn: sqlite3.Connection,
    *,
    job_id: str,
    kind: str | None,
    logical_name: str | None,
    path_text: str,
) -> tuple[str, str]:
    """V1.41: derive --kind / --logical-name from expected_artifacts.

    When the operator passes `--path` and the path matches a
    declared `expected_artifacts[].path` for the job, fall back to
    that artifact's declared `kind` / `logical_name`. Pass-through
    if the operator already supplied them, or if the path is
    ambiguous / unmatched.
    """
    import json
    if kind and logical_name:
        return kind, logical_name
    row = conn.execute(
        "SELECT expected_artifacts_json FROM jobs WHERE job_id = ?",
        (job_id,),
    ).fetchone()
    if row is None:
        if not kind or not logical_name:
            raise StriatumError(
                "publish-artifact requires --kind and --logical-name "
                "for unknown jobs",
                exit_code=2,
            )
        return kind, logical_name
    # expected_artifacts_json stores a JSON array; raw json.loads (not the
    # strict striatum.db.json_loads which only accepts objects).
    try:
        declared_raw = json.loads(str(row["expected_artifacts_json"] or "[]"))
    except (json.JSONDecodeError, TypeError):
        declared_raw = []
    declared_list: list[dict[str, object]] = (
        [d for d in declared_raw if isinstance(d, dict)]
        if isinstance(declared_raw, list)
        else []
    )
    matches = [d for d in declared_list if d.get("path") == path_text]
    if len(matches) != 1:
        if not kind or not logical_name:
            expected = ", ".join(
                f"{d.get('path')} ({d.get('logical_name')}/{d.get('kind')})"
                for d in declared_list
            ) or "(none declared)"
            raise StriatumError(
                f"publish-artifact could not default --kind/--logical-name: "
                f"path {path_text!r} matches {len(matches)} declared "
                f"artifacts; expected exactly 1. Declared: {expected}",
                exit_code=2,
            )
        return kind, logical_name
    declared_artifact = matches[0]
    resolved_kind = kind or str(declared_artifact.get("kind", ""))
    resolved_logical = logical_name or str(declared_artifact.get("logical_name", ""))
    return resolved_kind, resolved_logical


def _resolve_override_session(
    conn: sqlite3.Connection,
    *,
    requested_session_id: str,
    job_id: str,
    rationale: str,
) -> str:
    """V1.41: if requested_session_id already has a verdict for job_id,
    register a fresh operator reviewer session on the same lane and
    return its id. Otherwise return requested_session_id unchanged.

    The override path requires a session distinct from the one that
    submitted the original verdict; before V1.41 this was a manual
    two-step (register-session + override-verdict).
    """
    existing = conn.execute(
        "SELECT 1 FROM verdicts WHERE job_id = ? AND session_id = ?",
        (job_id, requested_session_id),
    ).fetchone()
    if existing is None:
        return requested_session_id
    session_row = conn.execute(
        "SELECT run_id, lane_id FROM sessions WHERE session_id = ?",
        (requested_session_id,),
    ).fetchone()
    if session_row is None:
        return requested_session_id
    from striatum.cli.mutations import register_session

    label = f"operator-override-{job_id[-12:]}"
    result = register_session(
        conn,
        run_id=str(session_row["run_id"]),
        role="reviewer",
        lane=str(session_row["lane_id"]),
        capabilities=[],
        fresh=True,
        parent_session_id=None,
        operator_label=label,
    )
    return str(result["session_id"])


def _cli_byline(conn: sqlite3.Connection, *, session_id: str, job_id: str) -> object:
    """V1.41: print the expected author line for a (session, job) pair."""
    from striatum.artifacts import expected_author_line
    job = conn.execute(
        "SELECT * FROM jobs WHERE job_id = ?", (job_id,)
    ).fetchone()
    if job is None:
        raise StriatumError(f"job {job_id!r} not found", exit_code=2)
    line = expected_author_line(conn, job=job, session_id=session_id)
    return {"session_id": session_id, "job_id": job_id, "byline": line}


def _cli_inbox(conn: sqlite3.Connection, *, session_id: str) -> object:
    """V1.41: print the current packet for a session.

    Returns the workflow_job_id, message_id, lease_id, expected_artifacts,
    and the expected author line. Designed for operator-on-behalf flows
    that previously required parsing `striatum why <sid> --json`.
    """
    import json
    from striatum.artifacts import expected_author_line
    session = conn.execute(
        "SELECT * FROM sessions WHERE session_id = ?", (session_id,)
    ).fetchone()
    if session is None:
        raise StriatumError(f"session {session_id!r} not found", exit_code=2)
    job = conn.execute(
        """
        SELECT * FROM jobs
         WHERE current_message_id IS NOT NULL
           AND current_lease_id IS NOT NULL
           AND state IN ('claimed', 'running')
           AND job_id IN (
                 SELECT resource_id FROM leases
                  WHERE owner_session_id = ?
                    AND state = 'active'
                    AND resource_type = 'job'
           )
         ORDER BY started_at DESC LIMIT 1
        """,
        (session_id,),
    ).fetchone()
    if job is None:
        return {
            "session_id": session_id,
            "lane_id": session["lane_id"],
            "role_id": session["role_id"],
            "current_packet": None,
        }
    # expected_artifacts_json is a JSON array, not an object — use raw
    # json.loads here, not the strict striatum.db.json_loads helper.
    try:
        expected_raw = json.loads(str(job["expected_artifacts_json"] or "[]"))
    except (json.JSONDecodeError, TypeError):
        expected_raw = []
    expected: list[object] = list(expected_raw) if isinstance(expected_raw, list) else []
    try:
        byline = expected_author_line(conn, job=job, session_id=session_id)
    except Exception:  # noqa: BLE001
        byline = None
    return {
        "session_id": session_id,
        "lane_id": session["lane_id"],
        "role_id": session["role_id"],
        "current_packet": {
            "workflow_job_id": job["workflow_job_id"],
            "job_id": job["job_id"],
            "message_id": job["current_message_id"],
            "lease_id": job["current_lease_id"],
            "expected_artifacts": expected,
            "expected_author_line": byline,
        },
    }

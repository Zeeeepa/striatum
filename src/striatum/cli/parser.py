"""Argparse construction for the Striatum CLI."""

from __future__ import annotations

import argparse


def build_parser() -> argparse.ArgumentParser:
    """Build the top-level argument parser."""
    parser = argparse.ArgumentParser(prog="striatum")
    parser.add_argument("--repo", default=".", help="repository root")
    sub = parser.add_subparsers(dest="command", required=True)

    init = sub.add_parser("init")
    init.add_argument("--json", action="store_true")
    init.add_argument(
        "--with-skills",
        nargs="?",
        const="claude_code",
        default=None,
        help=(
            "After init, also write the agent skill bundle for the given "
            "profile (default: claude_code). RFC 0015."
        ),
    )

    skills = sub.add_parser("skills")
    skills_sub = skills.add_subparsers(dest="skills_command", required=True)
    skills_install = skills_sub.add_parser("install")
    skills_install.add_argument(
        "--profile",
        choices=["claude_code", "codex", "gemini", "generic", "all"],
        default="claude_code",
    )
    skills_install.add_argument(
        "--scope", choices=["project", "user"], default="project"
    )
    skills_install.add_argument("--namespace", default="striatum-")
    skills_install.add_argument("--force", action="store_true")
    skills_install.add_argument("--dry-run", action="store_true")
    skills_install.add_argument("--json", action="store_true")

    workflow = sub.add_parser("workflow")
    workflow_sub = workflow.add_subparsers(dest="workflow_command", required=True)
    validate = workflow_sub.add_parser("validate")
    validate.add_argument("path")
    validate.add_argument("--json", action="store_true")
    plan = workflow_sub.add_parser("plan")
    plan.add_argument("path")
    plan.add_argument("--json", action="store_true")
    graph = workflow_sub.add_parser("graph")
    graph.add_argument("path")
    graph.add_argument("--format", choices=["mermaid", "json", "dot"], default="mermaid")
    graph.add_argument("--json", action="store_true")
    workflow_init = workflow_sub.add_parser("init")
    workflow_init.add_argument("path")
    workflow_init.add_argument(
        "--style",
        choices=["minimal", "review", "code-change"],
        default="review",
        help="template style for the generated workflow",
    )
    workflow_init.add_argument("--json", action="store_true")

    run = sub.add_parser("run")
    run_sub = run.add_subparsers(dest="run_command", required=True)
    prepare = run_sub.add_parser("prepare")
    prepare.add_argument("--workflow", required=True)
    prepare.add_argument("--json", action="store_true")
    start = run_sub.add_parser("start")
    start.add_argument("--run-id", required=True)
    start.add_argument("--json", action="store_true")
    summary = run_sub.add_parser("summary")
    summary.add_argument("--run-id", required=True)
    summary.add_argument("--path", required=True)
    summary.add_argument("--json", action="store_true")
    run_graph_p = run_sub.add_parser("graph")
    run_graph_p.add_argument("--run-id", required=True)
    run_graph_p.add_argument(
        "--format",
        choices=["mermaid", "json", "ascii"],
        default="mermaid",
    )
    run_graph_p.add_argument("--json", action="store_true")

    branch = sub.add_parser("branch")
    branch_sub = branch.add_subparsers(dest="branch_command", required=True)
    confirm = branch_sub.add_parser("confirm")
    confirm.add_argument("--run-id", required=True)
    confirm.add_argument("--branch", required=True)
    confirm.add_argument("--create", action="store_true")
    confirm.add_argument("--use-current", action="store_true")
    confirm.add_argument(
        "--strict",
        action="store_true",
        help="require the current git branch to match --branch before recording (safe default for CI)",
    )
    confirm.add_argument("--json", action="store_true")

    register = sub.add_parser("register-session")
    register.add_argument("--run-id", required=True)
    register.add_argument("--role", required=True)
    register.add_argument("--lane", required=True)
    register.add_argument("--capability", action="append", default=[])
    register.add_argument("--fresh", action="store_true")
    register.add_argument("--parent-session-id")
    # HARNESS-003: operator escape hatch for the fresh-reviewer policy.
    # When the workflow declares a fresh reviewer and an active author
    # session already exists on the run, register-session refuses unless
    # --force-non-fresh is passed with a non-empty --reason. The reason
    # is stored on the session row (``non_fresh_reason``) so evidence
    # exports record the breach explicitly.
    register.add_argument("--force-non-fresh", action="store_true")
    register.add_argument("--reason")
    register.add_argument("--json", action="store_true")

    # RFC 0011: session command group. ``close`` is the only subcommand
    # for now; future entries (e.g. ``list``, ``inspect``) can extend
    # the group without polluting the top-level subparser namespace.
    session = sub.add_parser("session")
    session_sub = session.add_subparsers(dest="session_command", required=True)
    session_close = session_sub.add_parser("close")
    session_close.add_argument("--session-id", required=True)
    session_close.add_argument("--reason", required=True)
    session_close.add_argument("--json", action="store_true")

    claim = sub.add_parser("claim-next")
    claim.add_argument("--session-id", required=True)
    claim.add_argument("--lease-seconds", type=int, default=1800)
    claim.add_argument("--json", action="store_true")

    ack = sub.add_parser("ack")
    add_work_identity(ack)

    heartbeat = sub.add_parser("heartbeat")
    heartbeat.add_argument("--session-id", required=True)
    heartbeat.add_argument("--lease-id", required=True)
    heartbeat.add_argument("--extend-seconds", type=int, default=1800)
    heartbeat.add_argument("--json", action="store_true")

    release = sub.add_parser("release")
    add_work_identity(release)
    release.add_argument("--reason", required=True)
    release.add_argument("--requeue", action="store_true")

    send = sub.add_parser("send")
    send.add_argument("--session-id", required=True)
    send.add_argument("--kind", required=True)
    send.add_argument("--body-json", default="{}")
    send.add_argument("--json", action="store_true")

    block = sub.add_parser("block")
    block.add_argument("--session-id", required=True)
    block.add_argument("--job-id", required=True)
    block.add_argument("--lease-id", required=True)
    block.add_argument("--kind", required=True)
    block.add_argument("--severity", choices=["blocked", "human_checkpoint"], required=True)
    block.add_argument("--description", required=True)
    block.add_argument("--json", action="store_true")

    publish = sub.add_parser("publish-artifact")
    publish.add_argument("--session-id", required=True)
    publish.add_argument("--job-id", required=True)
    publish.add_argument("--lease-id", required=True)
    publish.add_argument("--kind", required=True)
    publish.add_argument("--logical-name", required=True)
    publish.add_argument("--path", required=True)
    publish.add_argument("--json", action="store_true")

    complete = sub.add_parser("complete")
    complete.add_argument("--session-id", required=True)
    complete.add_argument("--job-id", required=True)
    complete.add_argument("--lease-id", required=True)
    complete.add_argument("--summary")
    complete.add_argument("--json", action="store_true")

    verdict = sub.add_parser("verdict")
    verdict.add_argument("--session-id", required=True)
    verdict.add_argument("--job-id", required=True)
    verdict.add_argument("--lease-id", required=True)
    verdict.add_argument(
        "--verdict",
        choices=["accept", "accept_with_findings", "needs_revision", "reject"],
        required=True,
    )
    verdict.add_argument("--findings-artifact-id")
    verdict.add_argument("--rationale")
    verdict.add_argument("--json", action="store_true")

    submit_review = sub.add_parser("submit-review")
    submit_review.add_argument("--session-id", required=True)
    submit_review.add_argument("--job-id", required=True)
    submit_review.add_argument("--lease-id", required=True)
    submit_review.add_argument("--path", required=True)
    submit_review.add_argument(
        "--verdict",
        choices=["accept", "accept_with_findings", "needs_revision", "reject"],
        required=True,
    )
    submit_review.add_argument("--logical-name", default="review")
    submit_review.add_argument("--kind", default="finding")
    submit_review.add_argument("--rationale")
    submit_review.add_argument("--json", action="store_true")

    evidence = sub.add_parser("evidence")
    evidence_sub = evidence.add_subparsers(dest="evidence_command", required=True)
    evidence_export = evidence_sub.add_parser("export")
    evidence_export.add_argument("--run-id", required=True)
    evidence_export.add_argument("--path", required=True)
    evidence_export.add_argument("--json", action="store_true")

    decision = sub.add_parser("decision")
    decision_sub = decision.add_subparsers(dest="decision_command", required=True)
    decision_record = decision_sub.add_parser("record")
    decision_record.add_argument("--run-id", required=True)
    decision_record.add_argument("--path", required=True)
    decision_record.add_argument(
        "--outcome",
        choices=["accepted", "rejected", "accepted_with_follow_up"],
        required=True,
    )
    decision_record.add_argument("--title", required=True)
    decision_record.add_argument("--decision-id")
    decision_record.add_argument("--rationale")
    decision_record.add_argument("--follow-up")
    decision_record.add_argument("--json", action="store_true")

    status = sub.add_parser("status")
    status.add_argument("--run-id")
    status.add_argument("--json", action="store_true")

    why = sub.add_parser("why")
    why.add_argument("id")
    why.add_argument("--json", action="store_true")

    doctor = sub.add_parser("doctor")
    doctor.add_argument("--run-id")
    doctor.add_argument(
        "--verbose",
        action="store_true",
        help="include structured problem_records alongside the existing problems list",
    )
    doctor.add_argument("--json", action="store_true")

    recovery = sub.add_parser("recovery")
    recovery_sub = recovery.add_subparsers(dest="recovery_command", required=True)
    stale_leases = recovery_sub.add_parser("stale-leases")
    stale_leases.add_argument("--run-id", required=True)
    stale_leases.add_argument("--json", action="store_true")
    requeue_stale = recovery_sub.add_parser("requeue-stale")
    requeue_stale.add_argument("--run-id", required=True)
    requeue_stale.add_argument("--job-id", required=True)
    requeue_stale.add_argument("--json", action="store_true")
    cancel_job_p = recovery_sub.add_parser("cancel-job")
    cancel_job_p.add_argument("--run-id", required=True)
    cancel_job_p.add_argument("--job-id", required=True)
    cancel_job_p.add_argument("--reason", required=True)
    cancel_job_p.add_argument(
        "--cascade",
        action="store_true",
        help="also cancel downstream blocked jobs whose only path was through this job",
    )
    cancel_job_p.add_argument("--json", action="store_true")
    process_reconcile_p = recovery_sub.add_parser(
        "process-reconcile",
        help=(
            "RFC 0014 V1: walk process_executions rows in 'running' "
            "state and transition externally-killed processes to 'lost', "
            "re-running output validation on the newly-lost rows."
        ),
    )
    process_reconcile_p.add_argument("--run-id", required=True)
    process_reconcile_p.add_argument("--json", action="store_true")

    checkpoint = sub.add_parser("checkpoint")
    checkpoint_sub = checkpoint.add_subparsers(dest="checkpoint_command", required=True)
    checkpoint_resolve_p = checkpoint_sub.add_parser("resolve")
    checkpoint_resolve_p.add_argument("--blocker-id", required=True)
    checkpoint_resolve_p.add_argument(
        "--action",
        choices=["continue", "cancel"],
        required=True,
    )
    checkpoint_resolve_p.add_argument(
        "--decision-id",
        help="optional decision artifact id to record alongside the resolution",
    )
    checkpoint_resolve_p.add_argument("--json", action="store_true")

    adapter = sub.add_parser("adapter")
    adapter_sub = adapter.add_subparsers(dest="adapter_command", required=True)
    adapter_run = adapter_sub.add_parser("run")
    adapter_run.add_argument("--session-id", required=True)
    adapter_run.add_argument("--lease-id", required=True)
    adapter_run.add_argument("--stdin", choices=["packet", "none"], default="packet")
    adapter_run.add_argument("--inherit-stdio", action="store_true")
    adapter_run.add_argument(
        "--timeout-seconds",
        type=int,
        default=None,
        help=(
            "RFC 0014 V1: SIGTERM the child after N seconds and block the "
            "job with process_timeout_exceeded. Overrides any "
            "lanes.<id>.adapter_timeout_seconds default. Omit (or set to "
            "the lane default) to keep the historical unbounded behaviour."
        ),
    )
    adapter_run.add_argument("--json", action="store_true")

    worktree = sub.add_parser("worktree")
    worktree_sub = worktree.add_subparsers(dest="worktree_command", required=True)
    worktree_create = worktree_sub.add_parser("create")
    worktree_create.add_argument("--session-id", required=True)
    worktree_create.add_argument("--job-id", required=True)
    worktree_create.add_argument("--lease-id", required=True)
    worktree_create.add_argument("--json", action="store_true")
    worktree_release = worktree_sub.add_parser("release")
    worktree_release.add_argument("--worktree-id", required=True)
    worktree_release.add_argument("--json", action="store_true")
    worktree_list = worktree_sub.add_parser("list")
    worktree_list.add_argument("--run-id")
    worktree_list.add_argument("--json", action="store_true")

    supervise = sub.add_parser("supervise")
    supervise_sub = supervise.add_subparsers(dest="supervise_command", required=True)
    supervise_start_p = supervise_sub.add_parser("start")
    supervise_start_p.add_argument("--session-id", required=True)
    supervise_start_p.add_argument("--json", action="store_true")
    supervise_send_p = supervise_sub.add_parser("send")
    supervise_send_p.add_argument("--session-id", required=True)
    supervise_send_p.add_argument("--packet-id", required=True)
    supervise_send_p.add_argument("--json", action="store_true")
    supervise_stop_p = supervise_sub.add_parser("stop")
    supervise_stop_p.add_argument("--session-id", required=True)
    supervise_stop_p.add_argument("--reason", required=True)
    supervise_stop_p.add_argument("--json", action="store_true")
    supervise_status_p = supervise_sub.add_parser("status")
    supervise_status_p.add_argument("--session-id", required=True)
    supervise_status_p.add_argument("--json", action="store_true")
    supervise_list_p = supervise_sub.add_parser("list")
    supervise_list_p.add_argument("--run-id", required=True)
    supervise_list_p.add_argument("--state")
    supervise_list_p.add_argument("--json", action="store_true")

    serve = sub.add_parser(
        "serve",
        help=(
            "RFC 0012 V1: run the local HTTP / Unix-socket service. "
            "Localhost-only by default; non-loopback hosts refused at "
            "startup with exit 8."
        ),
    )
    serve.add_argument("--unix")
    serve.add_argument("--host", default=None)
    serve.add_argument("--port", type=int, default=None)
    serve.add_argument("--token", default=None)
    serve.add_argument("--allow-mutations", action="store_true")
    serve.add_argument("--idle-timeout-seconds", type=int, default=None)
    serve.add_argument("--web", action="store_true")
    serve.add_argument("--json", action="store_true")

    dashboard = sub.add_parser("dashboard")
    dashboard.add_argument("--run-id", required=True)
    dashboard.add_argument("--refresh", type=float, default=2.0)
    dashboard.add_argument("--once", action="store_true")
    # RFC 0016 V1: graph panel.
    graph_group = dashboard.add_mutually_exclusive_group()
    graph_group.add_argument("--graph", dest="graph", action="store_true", default=None)
    graph_group.add_argument("--no-graph", dest="graph", action="store_false")
    dashboard.add_argument("--graph-only", action="store_true")
    dashboard.add_argument(
        "--graph-style",
        choices=["auto", "layered", "list", "fancy"],
        default="auto",
    )
    dashboard.add_argument("--graph-no-cycles", action="store_true")

    list_cmd = sub.add_parser("list")
    list_sub = list_cmd.add_subparsers(dest="list_command", required=True)

    list_runs_p = list_sub.add_parser("runs")
    list_runs_p.add_argument("--state")
    list_runs_p.add_argument("--limit", type=_positive_int, default=100)
    list_runs_p.add_argument("--json", action="store_true")

    list_sessions_p = list_sub.add_parser("sessions")
    list_sessions_p.add_argument("--run-id", required=True)
    list_sessions_p.add_argument("--state")
    list_sessions_p.add_argument("--role")
    list_sessions_p.add_argument("--lane")
    list_sessions_p.add_argument("--json", action="store_true")

    list_jobs_p = list_sub.add_parser("jobs")
    list_jobs_p.add_argument("--run-id", required=True)
    list_jobs_p.add_argument("--state")
    list_jobs_p.add_argument("--workflow-job-id")
    list_jobs_p.add_argument("--json", action="store_true")

    list_artifacts_p = list_sub.add_parser("artifacts")
    list_artifacts_p.add_argument("--run-id", required=True)
    list_artifacts_p.add_argument("--kind")
    list_artifacts_p.add_argument("--json", action="store_true")

    list_workflows_p = list_sub.add_parser("workflows")
    list_workflows_p.add_argument("--limit", type=_positive_int, default=100)
    list_workflows_p.add_argument("--json", action="store_true")

    return parser


def _positive_int(value: str) -> int:
    """Argparse type for ``--limit``: require a positive integer."""
    try:
        parsed = int(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError(f"expected a positive integer, got {value!r}") from exc
    if parsed <= 0:
        raise argparse.ArgumentTypeError(f"expected a positive integer, got {value!r}")
    return parsed


def add_work_identity(parser: argparse.ArgumentParser) -> None:
    """Add standard work ownership arguments."""
    parser.add_argument("--session-id", required=True)
    parser.add_argument("--message-id", required=True)
    parser.add_argument("--lease-id", required=True)
    parser.add_argument("--json", action="store_true")

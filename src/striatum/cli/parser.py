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
    graph.add_argument("--format", choices=["mermaid", "json"], default="mermaid")
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
    register.add_argument("--json", action="store_true")

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

    adapter = sub.add_parser("adapter")
    adapter_sub = adapter.add_subparsers(dest="adapter_command", required=True)
    adapter_run = adapter_sub.add_parser("run")
    adapter_run.add_argument("--session-id", required=True)
    adapter_run.add_argument("--lease-id", required=True)
    adapter_run.add_argument("--stdin", choices=["packet", "none"], default="packet")
    adapter_run.add_argument("--inherit-stdio", action="store_true")
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

    dashboard = sub.add_parser("dashboard")
    dashboard.add_argument("--run-id", required=True)
    dashboard.add_argument("--refresh", type=float, default=2.0)
    dashboard.add_argument("--once", action="store_true")

    return parser


def add_work_identity(parser: argparse.ArgumentParser) -> None:
    """Add standard work ownership arguments."""
    parser.add_argument("--session-id", required=True)
    parser.add_argument("--message-id", required=True)
    parser.add_argument("--lease-id", required=True)
    parser.add_argument("--json", action="store_true")

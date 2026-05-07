"""Command line interface for the striatum MVP."""

from __future__ import annotations

import argparse
import json
import sqlite3
import subprocess
import sys
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Sequence

from striatum.artifacts import publish_artifact
from striatum.db import (
    JsonObject,
    STATE_DIR,
    WORKTREES_SUBDIR,
    active_lease_for,
    active_worktree_for_job,
    claim_next,
    complete_job,
    connect,
    db_path,
    ensure_initialized,
    expire_leases,
    init_repo,
    insert_event,
    is_repo_write,
    job_lane_id,
    json_dumps,
    json_loads,
    lane_worktree_isolation,
    latest_verdict,
    new_id,
    record_review_verdict,
    repo_relative_path,
    row_by_id,
    sha256_bytes,
    transaction,
    utc_now,
    workflow_for_run,
)
from striatum.errors import ArtifactError, InvalidTransitionError, NotFoundError, StriatumError, WorkflowError
from striatum.identity import artifact_author_identity
from striatum.process_adapter import run_process_adapter
from striatum.workflow import create_run, load_workflow, plan_workflow, workflow_graph_data, workflow_graph_mermaid


EVIDENCE_FREE_TEXT_PLACEHOLDER = "<redacted-free-text>"

# Evidence redaction is a typed-field registry, not a blocklist. The export
# payload schema is fixed and known: every field that should appear verbatim
# in the committed Markdown export must be classified as "safe" below. Any
# field not listed in the registry is redacted by default. This default-deny
# rule is the contract: when someone adds a new field to evidence_snapshot(),
# status(), or doctor(), it is replaced with the placeholder until they
# explicitly extend the registry. See docs/SPEC.md (Artifacts/Evidence).
#
# Policy values:
#   "safe"     - emit value verbatim (ids, enums, counts, hashes, timestamps,
#                role/lane/state names, structured author identity).
#   "redacted" - replace value with EVIDENCE_FREE_TEXT_PLACEHOLDER.
#   "dropped"  - omit the field entirely from the redacted output.
#
# Special keys inside a section policy:
#   "_each"    - apply this policy to every dict element of a list.
#   "_items"   - apply this policy to every primitive element of a list
#                (use "safe" for lists of ids, enum names, or counts).
EVIDENCE_POLICY: JsonObject = {
    # --- status() output -------------------------------------------------
    "runs": {
        "_each": {
            "run_id": "safe",
            "state": "safe",
            "branch_name": "safe",
        },
    },
    "jobs": {
        # status returns a dict[str, int] of state -> count; every key is a
        # job-state enum name and every value is a count, so the whole
        # mapping is safe. Any unexpected nested structure is redacted.
        "_dict": "safe",
    },
    "open_blockers": {
        "_each": {
            "blocker_id": "safe",
            "run_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "severity": "safe",
            "blocker_kind": "safe",
            "description": "redacted",
            "state": "safe",
            "workflow_job_id": "safe",
            "job_state": "safe",
        },
    },
    "human_checkpoints": {
        "_each": {
            "blocker_id": "safe",
            "run_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "severity": "safe",
            "blocker_kind": "safe",
            "description": "redacted",
            "state": "safe",
            "workflow_job_id": "safe",
            "job_state": "safe",
        },
    },
    "latest_non_accepting_review_verdicts": {
        "_each": {
            "verdict_id": "safe",
            "run_id": "safe",
            "job_id": "safe",
            "workflow_job_id": "safe",
            "job_state": "safe",
            "session_id": "safe",
            "verdict": "safe",
            "findings_artifact_id": "safe",
            "rationale": "redacted",
        },
    },
    "claimable_jobs": {
        "_each": {
            "role_id": "safe",
            "lane_id": "safe",
            "count": "safe",
            "workflow_job_ids": {"_items": "safe"},
        },
    },
    "blocked_downstream_jobs": {
        "_each": {
            "job_id": "safe",
            "workflow_job_id": "safe",
            "state": "safe",
            "role_id": "safe",
            "lane": "safe",
            "blocked_by": {
                "_each": {
                    "depends_on_job_id": "safe",
                    "workflow_job_id": "safe",
                    "state": "safe",
                    "required_verdicts": {"_items": "safe"},
                    "latest_verdict": "safe",
                },
            },
        },
    },
    "next_actions": {"_items": "safe"},
    # --- doctor() output -------------------------------------------------
    "ok": "safe",
    "schema_version": "safe",
    "problems": {"_items": "safe"},
    # --- evidence_snapshot() output --------------------------------------
    "exported_at": "safe",
    "workflow": {
        "workflow_id": "safe",
        "workflow_version": "safe",
    },
    "run": {
        "run_id": "safe",
        "branch_name": "safe",
        "state": "safe",
    },
    # snapshot.jobs is a list of job summary dicts (key reused from status
    # but reached via a different path; the walker disambiguates by context).
    "snapshot_jobs": {
        "_each": {
            "job_id": "safe",
            "workflow_job_id": "safe",
            "job_type": "safe",
            "role_id": "safe",
            "lane": "safe",
            "display_model": "safe",
            "author": {
                "role_id": "safe",
                "lane_id": "safe",
                "display_model": "safe",
                "workflow_job_id": "safe",
                "ordinal": "safe",
                "line": "safe",
            },
            "state": "safe",
            "attempt": "safe",
            "max_attempts": "safe",
            "fresh_session_required": "safe",
            # Workflow job titles are project-specific prose; per docs/SPEC.md
            # they are omitted by default. evidence_job_summaries() does not
            # include "title" today, but if a future change adds it the
            # default-deny rule keeps it out of the export.
            "title": "redacted",
            "dependencies": {
                "_each": {
                    "depends_on_job_id": "safe",
                    "workflow_job_id": "safe",
                    "state": "safe",
                    "required_verdicts": {"_items": "safe"},
                    "latest_verdict": "safe",
                },
            },
        },
    },
    "artifacts": {
        "_each": {
            "artifact_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "logical_name": "safe",
            "artifact_kind": "safe",
            "repo_path": "safe",
            "content_sha256": "safe",
            "author": {
                "role_id": "safe",
                "lane_id": "safe",
                "display_model": "safe",
                "workflow_job_id": "safe",
                "ordinal": "safe",
                "line": "safe",
            },
        },
    },
    "verdicts": {
        "_each": {
            "verdict_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "verdict": "safe",
            "findings_artifact_id": "safe",
            "rationale": "redacted",
        },
    },
    "blockers": {
        "_each": {
            "blocker_id": "safe",
            "job_id": "safe",
            "session_id": "safe",
            "severity": "safe",
            "blocker_kind": "safe",
            "description": "redacted",
            "state": "safe",
        },
    },
}

# The "jobs" key appears at the top level in two payload shapes:
#   - status(): dict[state -> count]
#   - evidence_snapshot(): list of job summary dicts
# _evidence_policy_for_top_level() dispatches by value type to pick between
# EVIDENCE_POLICY["jobs"] and EVIDENCE_POLICY["snapshot_jobs"].


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

    return parser


def add_work_identity(parser: argparse.ArgumentParser) -> None:
    """Add standard work ownership arguments."""
    parser.add_argument("--session-id", required=True)
    parser.add_argument("--message-id", required=True)
    parser.add_argument("--lease-id", required=True)
    parser.add_argument("--json", action="store_true")


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
        mermaid = workflow_graph_mermaid(workflow)
        if args.json:
            return {"format": "mermaid", "source": mermaid}
        return mermaid
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
            return doctor(conn, repo=repo, run_id=args.run_id)
        if args.command == "recovery" and args.recovery_command == "stale-leases":
            return stale_leases(conn, run_id=args.run_id)
        if args.command == "recovery" and args.recovery_command == "requeue-stale":
            return requeue_stale(conn, run_id=args.run_id, job_id=args.job_id)
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
    raise StriatumError("unknown command", exit_code=2)


def branch_confirm(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    run_id: str,
    branch: str,
    create: bool = False,
    use_current: bool = False,
    strict: bool = False,
) -> JsonObject:
    """Record branch confirmation.

    Default mode is records-only: write the row, run an advisory git check,
    and emit a warning if the current branch disagrees. The opt-in modes
    actually drive git or refuse to record:

    - ``create``: run ``git checkout -b <branch>`` (idempotent fallback to
      ``git checkout <branch>`` if the branch already exists). Surface git
      errors and exit with ``WorkflowError`` (code 8).
    - ``use_current``: ignore the requested branch as a target and record the
      current git branch instead. Conflicts with a non-matching ``--branch``.
    - ``strict``: refuse to record unless the current git branch already
      matches ``--branch`` exactly.
    """
    if create and use_current:
        raise WorkflowError("--create and --use-current are mutually exclusive")
    if strict and (create or use_current):
        raise WorkflowError("--strict is incompatible with --create and --use-current")

    requested_branch = branch
    created = False
    mode = "records_only"

    if use_current:
        mode = "use_current"
        current = current_git_branch(repo)
        if current is None:
            raise WorkflowError(
                "--use-current requires a detectable current git branch in the target repo"
            )
        if branch != current:
            raise WorkflowError(
                f"--use-current was given but --branch={branch!r} does not match current git branch {current!r}"
            )
        target_branch = current
    elif create:
        mode = "create"
        target_branch, created = git_create_or_checkout_branch(repo, branch)
    elif strict:
        mode = "strict"
        current = current_git_branch(repo)
        if current != branch:
            raise WorkflowError(
                f"--strict requires current git branch to match --branch={branch!r}; "
                f"current branch is {current!r}"
            )
        target_branch = branch
    else:
        target_branch = branch

    with transaction(conn):
        run = row_by_id(conn, "runs", "run_id", run_id)
        if run["state"] not in ("needs_branch_confirmation", "ready"):
            raise InvalidTransitionError("run is not waiting for branch confirmation")
        current_branch = current_git_branch(repo)
        now = utc_now()
        conn.execute(
            """
            UPDATE runs
            SET branch_name = ?, branch_confirmed_at = ?, branch_confirmed_by = 'human',
                state = 'ready'
            WHERE run_id = ?
            """,
            (target_branch, now, run_id),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="run.branch_confirmed",
            payload={"branch": target_branch, "mode": mode, "created": created},
        )
        warning = None
        if current_branch is not None and current_branch != target_branch:
            warning = "current git branch differs from recorded branch confirmation"
        return {
            "run_id": run_id,
            "state": "ready",
            "branch": target_branch,
            "requested_branch": requested_branch,
            "current_git_branch": current_branch,
            "records_only": True,
            "warning": warning,
            "created": created,
            "mode": mode,
        }


def git_create_or_checkout_branch(repo: Path, branch: str) -> tuple[str, bool]:
    """Create ``branch`` via ``git checkout -b`` or fall back to checkout.

    Returns ``(branch, created)`` where ``created`` is True only when the
    branch did not exist beforehand. Raises ``WorkflowError`` if both git
    invocations fail; the latter stderr is included (truncated) so the user
    can diagnose dirty working trees and similar problems.
    """
    create_result = subprocess.run(
        ["git", "checkout", "-b", branch],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if create_result.returncode == 0:
        return branch, True
    checkout_result = subprocess.run(
        ["git", "checkout", branch],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if checkout_result.returncode == 0:
        return branch, False
    stderr = (checkout_result.stderr or create_result.stderr or "").strip()
    if len(stderr) > 200:
        stderr = stderr[:200] + "..."
    raise WorkflowError(
        f"git checkout failed for branch {branch!r}: {stderr}" if stderr
        else f"git checkout failed for branch {branch!r}"
    )


def current_git_branch(repo: Path) -> str | None:
    """Return the current Git branch when detectable."""
    result = subprocess.run(
        ["git", "branch", "--show-current"],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    branch = result.stdout.strip()
    if result.returncode != 0 or branch == "":
        return None
    return branch


def run_start(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Start a prepared run and enqueue root jobs."""
    with transaction(conn):
        run = row_by_id(conn, "runs", "run_id", run_id)
        if run["state"] == "needs_branch_confirmation":
            raise WorkflowError("branch confirmation is required before run start")
        if run["state"] not in ("ready", "running"):
            raise InvalidTransitionError("run cannot be started from its current state")
        if run["state"] == "ready":
            now = utc_now()
            conn.execute("UPDATE runs SET state = 'running', started_at = ? WHERE run_id = ?", (now, run_id))
            roots = conn.execute(
                """
                SELECT j.job_id
                FROM jobs j
                WHERE j.run_id = ?
                  AND NOT EXISTS (
                    SELECT 1 FROM job_dependencies dep WHERE dep.job_id = j.job_id
                  )
                ORDER BY j.created_at
                """,
                (run_id,),
            ).fetchall()
            from striatum.db import enqueue_job

            for root in roots:
                enqueue_job(conn, job_id=str(root["job_id"]))
            insert_event(conn, run_id=run_id, event_type="run.started")
        return {"run_id": run_id, "state": "running"}


def register_session(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    role: str,
    lane: str,
    capabilities: list[str],
    fresh: bool,
    parent_session_id: str | None,
) -> JsonObject:
    """Register an agent session."""
    with transaction(conn):
        run = row_by_id(conn, "runs", "run_id", run_id)
        snapshot = row_by_id(
            conn,
            "workflow_snapshots",
            "workflow_snapshot_id",
            str(run["workflow_snapshot_id"]),
        )
        workflow = json_loads(str(snapshot["workflow_json"]))
        roles = workflow.get("roles", {})
        lanes = workflow.get("lanes", {})
        if not isinstance(roles, dict) or role not in roles:
            raise InvalidTransitionError(f"unknown role {role!r} for run")
        if not isinstance(lanes, dict) or lane not in lanes:
            raise InvalidTransitionError(f"unknown lane {lane!r} for run")
        ordinal_row = conn.execute(
            """
            SELECT COALESCE(MAX(ordinal), 0) + 1 AS next_ordinal
            FROM sessions WHERE run_id = ? AND role_id = ? AND lane_id = ?
            """,
            (run_id, role, lane),
        ).fetchone()
        ordinal = int(ordinal_row["next_ordinal"])
        session_id = new_id("sess")
        slug = f"{role}-{lane}-{ordinal}"
        now = utc_now()
        conn.execute(
            """
            INSERT INTO sessions (
              session_id, run_id, role_id, lane_id, slug, ordinal,
              capabilities_json, parent_session_id, fresh_context, state,
              registered_at, last_heartbeat_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
            """,
            (
                session_id,
                run_id,
                role,
                lane,
                slug,
                ordinal,
                json.dumps(capabilities),
                parent_session_id,
                1 if fresh else 0,
                now,
                now,
            ),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="session.registered",
            actor_session_id=session_id,
            payload={"role": role, "lane": lane, "slug": slug},
        )
        return {"session_id": session_id, "slug": slug}


def ack_work(conn: sqlite3.Connection, *, session_id: str, message_id: str, lease_id: str) -> JsonObject:
    """Acknowledge claimed work and mark it running."""
    with transaction(conn):
        message = row_by_id(conn, "queue_messages", "message_id", message_id)
        job = row_by_id(conn, "jobs", "job_id", str(message["job_id"]))
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
        if message["state"] == "acked":
            return {"status": "acked", "job_id": job["job_id"]}
        if message["state"] != "claimed" or job["state"] != "claimed":
            raise InvalidTransitionError("work must be claimed before ack")
        now = utc_now()
        conn.execute(
            "UPDATE queue_messages SET state = 'acked', acked_at = ?, updated_at = ? WHERE message_id = ?",
            (now, now, message_id),
        )
        conn.execute("UPDATE jobs SET state = 'running', started_at = ? WHERE job_id = ?", (now, job["job_id"]))
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="queue.acked",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=lease_id,
        )
        return {"status": "acked", "job_id": job["job_id"]}


def heartbeat(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    lease_id: str,
    extend_seconds: int,
) -> JsonObject:
    """Refresh session and lease liveness."""
    with transaction(conn):
        lease = active_lease_for(conn, lease_id=lease_id, session_id=session_id)
        now = utc_now()
        expires_at = (
            datetime.now(UTC) + timedelta(seconds=extend_seconds)
        ).replace(microsecond=0).isoformat().replace("+00:00", "Z")
        conn.execute(
            "UPDATE sessions SET last_heartbeat_at = ? WHERE session_id = ?",
            (now, session_id),
        )
        conn.execute(
            "UPDATE leases SET last_heartbeat_at = ?, expires_at = ? WHERE lease_id = ?",
            (now, expires_at, lease_id),
        )
        insert_event(
            conn,
            run_id=str(lease["run_id"]),
            event_type="lease.heartbeat",
            actor_session_id=session_id,
            job_id=str(lease["resource_id"]),
            lease_id=lease_id,
            payload={"expires_at": expires_at},
        )
        return {"status": "heartbeat", "expires_at": expires_at}


def release_work(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    message_id: str,
    lease_id: str,
    reason: str,
    requeue: bool,
) -> JsonObject:
    """Release claimed work."""
    with transaction(conn):
        message = row_by_id(conn, "queue_messages", "message_id", message_id)
        job = row_by_id(conn, "jobs", "job_id", str(message["job_id"]))
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
        from striatum.db import is_repo_write

        now = utc_now()
        if requeue and not is_repo_write(job):
            job_state = "queued"
            msg_state = "pending"
        else:
            job_state = "blocked"
            msg_state = "blocked"
        conn.execute(
            "UPDATE leases SET state = 'released', released_at = ?, release_reason = ? WHERE lease_id = ?",
            (now, reason, lease_id),
        )
        conn.execute(
            "UPDATE jobs SET state = ?, current_lease_id = NULL WHERE job_id = ?",
            (job_state, job["job_id"]),
        )
        conn.execute(
            """
            UPDATE queue_messages
            SET state = ?, current_lease_id = NULL, updated_at = ?
            WHERE message_id = ?
            """,
            (msg_state, now, message_id),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="lease.released",
            actor_session_id=session_id,
            job_id=str(job["job_id"]),
            message_id=message_id,
            lease_id=lease_id,
            payload={"reason": reason, "job_state": job_state},
        )
        return {"status": "released", "job_state": job_state}


def send_message(conn: sqlite3.Connection, *, session_id: str, kind: str, body_json: str) -> JsonObject:
    """Write a structured message event."""
    with transaction(conn):
        session = row_by_id(conn, "sessions", "session_id", session_id)
        body = json.loads(body_json)
        if not isinstance(body, dict):
            raise InvalidTransitionError("message body must be a JSON object")
        message_id = new_id("msg")
        now = utc_now()
        conn.execute(
            """
            INSERT INTO queue_messages (
              message_id, run_id, kind, state, payload_json, created_at, updated_at
            )
            VALUES (?, ?, 'agent_message', 'completed', ?, ?, ?)
            """,
            (message_id, session["run_id"], json.dumps({"kind": kind, "body": body}), now, now),
        )
        insert_event(
            conn,
            run_id=str(session["run_id"]),
            event_type="message.sent",
            actor_session_id=session_id,
            message_id=message_id,
            payload={"kind": kind},
        )
        return {"message_id": message_id}


def block_work(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    kind: str,
    severity: str,
    description: str,
) -> JsonObject:
    """Record a blocker and stop the job."""
    with transaction(conn):
        job = row_by_id(conn, "jobs", "job_id", job_id)
        active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
        now = utc_now()
        blocker_id = new_id("blk")
        state = "waiting_human" if severity == "human_checkpoint" else "blocked"
        conn.execute(
            """
            INSERT INTO blockers (
              blocker_id, run_id, job_id, session_id, severity, blocker_kind,
              description, state, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, 'open', ?)
            """,
            (blocker_id, job["run_id"], job_id, session_id, severity, kind, description, now),
        )
        conn.execute("UPDATE jobs SET state = ?, current_lease_id = NULL WHERE job_id = ?", (state, job_id))
        conn.execute(
            "UPDATE leases SET state = 'released', released_at = ?, release_reason = 'blocked' WHERE lease_id = ?",
            (now, lease_id),
        )
        if job["current_message_id"] is not None:
            conn.execute(
                "UPDATE queue_messages SET state = 'blocked', current_lease_id = NULL WHERE message_id = ?",
                (job["current_message_id"],),
            )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="job.blocked",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={"blocker_id": blocker_id, "severity": severity},
        )
        return {"status": "blocked", "blocker_id": blocker_id}


def verdict_work(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    job_id: str,
    lease_id: str,
    verdict: str,
    findings_artifact_id: str | None,
    rationale: str | None,
) -> JsonObject:
    """Record a review verdict and apply review-gate behavior."""
    return record_review_verdict(
        conn,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        verdict=verdict,
        findings_artifact_id=findings_artifact_id,
        rationale=rationale,
    )


def submit_review(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    job_id: str,
    lease_id: str,
    path_text: str,
    verdict: str,
    logical_name: str,
    kind: str,
    rationale: str | None,
) -> JsonObject:
    """Publish a review artifact and record its verdict in one command."""
    job = row_by_id(conn, "jobs", "job_id", job_id)
    prevalidate_submit_review(
        conn,
        job=job,
        session_id=session_id,
        lease_id=lease_id,
        logical_name=logical_name,
        kind=kind,
        path_text=path_text,
    )
    if job["state"] == "claimed" and job["current_message_id"] is not None:
        ack_work(
            conn,
            session_id=session_id,
            message_id=str(job["current_message_id"]),
            lease_id=lease_id,
        )
    artifact = publish_artifact(
        conn,
        repo=repo,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        kind=kind,
        logical_name=logical_name,
        path_text=path_text,
    )
    verdict_result = record_review_verdict(
        conn,
        session_id=session_id,
        job_id=job_id,
        lease_id=lease_id,
        verdict=verdict,
        findings_artifact_id=str(artifact["artifact_id"]),
        rationale=rationale,
    )
    job = row_by_id(conn, "jobs", "job_id", job_id)
    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    return {
        "artifact": artifact,
        "verdict": verdict_result,
        "job_state": job["state"],
        "run_state": run["state"],
        "blocker_id": verdict_result.get("blocker_id"),
        "downstream_jobs": downstream_jobs(conn, job_id=job_id),
    }


def prevalidate_submit_review(
    conn: sqlite3.Connection,
    *,
    job: sqlite3.Row,
    session_id: str,
    lease_id: str,
    logical_name: str,
    kind: str,
    path_text: str,
) -> None:
    """Reject submit-review calls that would fail after artifact publication."""
    if job["job_type"] != "review":
        raise InvalidTransitionError("submit-review is valid only for review jobs")
    if job["state"] not in {"claimed", "running"}:
        raise InvalidTransitionError("review job must be claimed or running before submit-review")
    if job["state"] == "claimed" and job["current_message_id"] is None:
        raise InvalidTransitionError("claimed review job is missing its current message")
    active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=str(job["job_id"]))
    expected = json.loads(str(job["expected_artifacts_json"]))
    if not isinstance(expected, list):
        raise InvalidTransitionError("expected artifacts must be a list")
    for item in expected:
        if not isinstance(item, dict) or item.get("required") is not True:
            continue
        expected_logical_name = item.get("logical_name")
        expected_kind = item.get("kind")
        expected_path = item.get("path")
        if (expected_logical_name, expected_kind, expected_path) == (logical_name, kind, path_text):
            continue
        found = conn.execute(
            """
            SELECT 1 FROM artifacts
            WHERE job_id = ? AND logical_name = ? AND artifact_kind = ? AND repo_path = ?
            LIMIT 1
            """,
            (job["job_id"], expected_logical_name, expected_kind, expected_path),
        ).fetchone()
        if found is None:
            raise InvalidTransitionError(
                "required artifact would still be missing after submit-review: "
                f"logical_name={expected_logical_name!r}, kind={expected_kind!r}, path={expected_path!r}"
            )


def status(conn: sqlite3.Connection, *, run_id: str | None) -> JsonObject:
    """Return current state summary."""
    if run_id is not None:
        expire_leases(conn, run_id=run_id)
    runs = conn.execute(
        "SELECT run_id, state, branch_name FROM runs WHERE (? IS NULL OR run_id = ?) ORDER BY created_at",
        (run_id, run_id),
    ).fetchall()
    jobs = conn.execute(
        """
        SELECT state, COUNT(*) AS count FROM jobs
        WHERE (? IS NULL OR run_id = ?)
        GROUP BY state ORDER BY state
        """,
        (run_id, run_id),
    ).fetchall()
    open_blockers = blocker_summaries(conn, run_id=run_id, severity=None)
    human_checkpoints = blocker_summaries(conn, run_id=run_id, severity="human_checkpoint")
    non_accepting = latest_non_accepting_verdicts(conn, run_id=run_id)
    claimable = claimable_jobs_by_role_lane(conn, run_id=run_id)
    blocked_downstream = blocked_downstream_jobs(conn, run_id=run_id)
    return {
        "runs": [dict(row) for row in runs],
        "jobs": {str(row["state"]): int(row["count"]) for row in jobs},
        "open_blockers": open_blockers,
        "human_checkpoints": human_checkpoints,
        "latest_non_accepting_review_verdicts": non_accepting,
        "claimable_jobs": claimable,
        "blocked_downstream_jobs": blocked_downstream,
        "next_actions": next_actions(
            open_blockers=open_blockers,
            human_checkpoints=human_checkpoints,
            non_accepting_verdicts=non_accepting,
            claimable_jobs=claimable,
        ),
    }


def why(conn: sqlite3.Connection, *, target_id: str) -> JsonObject:
    """Explain a state id by returning related rows and events."""
    run = conn.execute("SELECT * FROM runs WHERE run_id = ?", (target_id,)).fetchone()
    if run is not None:
        run_id = str(run["run_id"])
        return {
            "target_type": "run",
            "run": dict(run),
            "jobs": [dict(row) for row in jobs_for_run(conn, run_id=run_id)],
            "open_blockers": blocker_summaries(conn, run_id=run_id, severity=None),
            "events": events_for(conn, run_id=run_id),
            "next_actions": status(conn, run_id=run_id)["next_actions"],
        }

    job = conn.execute("SELECT * FROM jobs WHERE job_id = ? OR workflow_job_id = ?", (target_id, target_id)).fetchone()
    message = conn.execute("SELECT * FROM queue_messages WHERE message_id = ?", (target_id,)).fetchone()
    if job is not None or message is not None:
        job_id = str(job["job_id"] if job is not None else message["job_id"])
        return {
            "target_type": "job" if job is not None else "message",
            "job": dict(job) if job is not None else dict(row_by_id(conn, "jobs", "job_id", job_id)),
            "message": dict(message) if message is not None else None,
            "verdict": latest_verdict_row(conn, job_id=job_id),
            "blockers": blockers_for_job(conn, job_id=job_id),
            "downstream_jobs": downstream_jobs(conn, job_id=job_id),
            "events": events_for(conn, job_id=job_id),
        }

    blocker = conn.execute("SELECT * FROM blockers WHERE blocker_id = ?", (target_id,)).fetchone()
    if blocker is not None:
        blocker_job_id = str(blocker["job_id"]) if blocker["job_id"] is not None else None
        run_id = str(blocker["run_id"])
        blocked_jobs = (
            downstream_jobs(conn, job_id=blocker_job_id) if blocker_job_id is not None else []
        )
        blocker_payload = dict(blocker)
        if blocker_job_id is not None:
            blocker_job = row_by_id(conn, "jobs", "job_id", blocker_job_id)
            blocker_payload["workflow_job_id"] = blocker_job["workflow_job_id"]
            blocker_payload["job_state"] = blocker_job["state"]
        checkpoint_context = (
            human_checkpoint_context(blocker_payload, blocked_jobs=blocked_jobs)
            if blocker["severity"] == "human_checkpoint"
            else None
        )
        return {
            "target_type": "blocker",
            "blocker": blocker_payload,
            "run": dict(row_by_id(conn, "runs", "run_id", run_id)),
            "job": dict(row_by_id(conn, "jobs", "job_id", blocker_job_id))
            if blocker_job_id is not None
            else None,
            "session": dict(row_by_id(conn, "sessions", "session_id", str(blocker["session_id"])))
            if blocker["session_id"] is not None
            else None,
            "related_verdict": latest_verdict_row(conn, job_id=blocker_job_id)
            if blocker_job_id is not None
            else None,
            "blocked_downstream_jobs": blocked_jobs,
            "human_checkpoint": checkpoint_context,
            "next_actions": checkpoint_context["next_actions"]
            if checkpoint_context is not None
            else ["inspect_blocker", "export_run_evidence"],
            "events": events_for(conn, job_id=blocker_job_id)
            if blocker_job_id is not None
            else events_for(conn, run_id=run_id),
        }

    artifact = conn.execute("SELECT * FROM artifacts WHERE artifact_id = ?", (target_id,)).fetchone()
    if artifact is not None:
        artifact_job_id = str(artifact["job_id"]) if artifact["job_id"] is not None else None
        return {
            "target_type": "artifact",
            "artifact": dict(artifact),
            "job": dict(row_by_id(conn, "jobs", "job_id", artifact_job_id))
            if artifact_job_id is not None
            else None,
            "verdicts": verdicts_for_artifact(conn, artifact_id=target_id),
            "events": events_for(conn, artifact_id=target_id),
        }

    verdict = conn.execute("SELECT * FROM verdicts WHERE verdict_id = ?", (target_id,)).fetchone()
    if verdict is not None:
        artifact_id = verdict["findings_artifact_id"]
        return {
            "target_type": "verdict",
            "verdict": dict(verdict),
            "job": dict(row_by_id(conn, "jobs", "job_id", str(verdict["job_id"]))),
            "artifact": dict(row_by_id(conn, "artifacts", "artifact_id", str(artifact_id)))
            if artifact_id is not None
            else None,
            "blockers": blockers_for_job(conn, job_id=str(verdict["job_id"])),
            "events": events_for(conn, job_id=str(verdict["job_id"])),
        }

    session = conn.execute("SELECT * FROM sessions WHERE session_id = ? OR slug = ?", (target_id, target_id)).fetchone()
    if session is not None:
        return {
            "target_type": "session",
            "session": dict(session),
            "jobs": jobs_for_session(conn, session_id=str(session["session_id"])),
            "events": events_for(conn, session_id=str(session["session_id"])),
        }

    if table_exists(conn, "process_executions"):
        process = conn.execute("SELECT * FROM process_executions WHERE process_id = ?", (target_id,)).fetchone()
        if process is not None:
            return {
                "target_type": "process",
                "process": dict(process),
                "job": dict(row_by_id(conn, "jobs", "job_id", str(process["job_id"]))),
                "session": dict(row_by_id(conn, "sessions", "session_id", str(process["session_id"]))),
                "events": events_for_process(conn, process_id=target_id),
            }

    raise NotFoundError(
        "target id is not a known run, job, message, blocker, artifact, verdict, session, or process"
    )


def evidence_export(conn: sqlite3.Connection, *, repo: Path, run_id: str, path_text: str) -> JsonObject:
    """Write a redacted Markdown snapshot of runner state."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    target = repo_relative_path(repo, path_text)
    target.parent.mkdir(parents=True, exist_ok=True)
    status_payload = redact_evidence_payload(status(conn, run_id=run_id))
    doctor_payload = redact_evidence_payload(doctor(conn, repo=repo, run_id=run_id))
    snapshot = redact_evidence_payload(evidence_snapshot(conn, run_id=run_id))
    body = render_evidence_markdown(
        run=dict(run),
        status_payload=status_payload,
        doctor_payload=doctor_payload,
        snapshot=snapshot,
    )
    target.write_text(body, encoding="utf-8")
    digest = sha256_bytes(body.encode("utf-8"))
    insert_event(
        conn,
        run_id=run_id,
        event_type="evidence.exported",
        payload={"path": path_text, "sha256": digest},
    )
    return {"status": "exported", "run_id": run_id, "path": path_text, "sha256": digest}


def run_summary_export(conn: sqlite3.Connection, *, repo: Path, run_id: str, path_text: str) -> JsonObject:
    """Write a compact human-facing run summary."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    target = repo_relative_path(repo, path_text)
    target.parent.mkdir(parents=True, exist_ok=True)
    summary = run_summary_snapshot(conn, repo=repo, run_id=run_id)
    body = render_run_summary_markdown(run=dict(run), summary=summary)
    target.write_text(body, encoding="utf-8")
    digest = sha256_bytes(body.encode("utf-8"))
    insert_event(
        conn,
        run_id=run_id,
        event_type="run_summary.exported",
        payload={"path": path_text, "sha256": digest},
    )
    return {"status": "exported", "run_id": run_id, "path": path_text, "sha256": digest}


def decision_record(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    run_id: str,
    path_text: str,
    outcome: str,
    title: str,
    decision_id: str | None,
    rationale: str | None,
    follow_up: str | None,
) -> JsonObject:
    """Write and record an owner decision artifact without requiring a lease."""
    row_by_id(conn, "runs", "run_id", run_id)
    title = title.strip()
    if title == "":
        raise ArtifactError("decision title cannot be empty")
    if outcome == "accepted_with_follow_up" and (follow_up is None or follow_up.strip() == ""):
        raise ArtifactError("accepted_with_follow_up decisions require --follow-up")
    resolved_decision_id = decision_id.strip() if decision_id is not None else new_id("dec")
    if resolved_decision_id == "" or any(character.isspace() for character in resolved_decision_id):
        raise ArtifactError("decision id cannot be empty or contain whitespace")
    existing = conn.execute(
        """
        SELECT artifact_id FROM artifacts
        WHERE run_id = ? AND artifact_kind = 'decision'
          AND (logical_name = ? OR repo_path = ?)
        LIMIT 1
        """,
        (run_id, resolved_decision_id, path_text),
    ).fetchone()
    if existing is not None:
        raise ArtifactError("decision artifact already exists for this run id/path")
    target = repo_relative_path(repo, path_text)
    if target.exists():
        raise ArtifactError("decision artifact path already exists")
    target.parent.mkdir(parents=True, exist_ok=True)
    created_at = utc_now()
    body = render_decision_markdown(
        decision_id=resolved_decision_id,
        run_id=run_id,
        outcome=outcome,
        title=title,
        created_at=created_at,
        rationale=rationale,
        follow_up=follow_up,
    )
    target.write_text(body, encoding="utf-8")
    payload = body.encode("utf-8")
    digest = sha256_bytes(payload)
    artifact_id = new_id("art")
    with transaction(conn):
        conn.execute(
            """
            INSERT INTO artifacts (
              artifact_id, run_id, job_id, session_id, logical_name,
              artifact_kind, repo_path, content_sha256, size_bytes,
              publish_mode, created_at
            )
            VALUES (?, ?, NULL, NULL, ?, 'decision', ?, ?, ?, 'create', ?)
            """,
            (
                artifact_id,
                run_id,
                resolved_decision_id,
                path_text,
                digest,
                len(payload),
                created_at,
            ),
        )
        insert_event(
            conn,
            run_id=run_id,
            event_type="decision.recorded",
            artifact_id=artifact_id,
            payload={
                "decision_id": resolved_decision_id,
                "outcome": outcome,
                "path": path_text,
                "sha256": digest,
            },
        )
    return {
        "status": "recorded",
        "run_id": run_id,
        "decision_id": resolved_decision_id,
        "artifact_id": artifact_id,
        "path": path_text,
        "outcome": outcome,
        "sha256": digest,
    }


def render_decision_markdown(
    *,
    decision_id: str,
    run_id: str,
    outcome: str,
    title: str,
    created_at: str,
    rationale: str | None,
    follow_up: str | None,
) -> str:
    """Render a machine-checkable owner decision Markdown artifact."""
    follow_up_required = outcome == "accepted_with_follow_up"
    lines = [
        "---",
        "schema_version: striatum.decision.v1",
        f"decision_id: {json.dumps(decision_id)}",
        f"run_id: {json.dumps(run_id)}",
        "artifact_kind: decision",
        "owner: human",
        f"outcome: {outcome}",
        f"follow_up_required: {str(follow_up_required).lower()}",
        f"title: {json.dumps(title)}",
        f"created_at: {json.dumps(created_at)}",
        "---",
        "",
        f"# {title}",
        "",
        f"Decision ID: `{decision_id}`",
        f"Run ID: `{run_id}`",
        f"Outcome: `{outcome}`",
        "",
    ]
    if rationale is not None and rationale.strip() != "":
        lines.extend(["## Rationale", "", rationale.strip(), ""])
    if follow_up is not None and follow_up.strip() != "":
        lines.extend(["## Follow-Up", "", follow_up.strip(), ""])
    return "\n".join(lines)


def redact_evidence_payload(payload: JsonObject) -> JsonObject:
    """Return a redacted copy of an evidence payload (status, doctor, or snapshot).

    The redaction is policy-driven: each top-level field is matched against
    EVIDENCE_POLICY and walked recursively. Fields not listed in the policy
    are replaced with EVIDENCE_FREE_TEXT_PLACEHOLDER (default-deny). This
    prevents future schema additions from silently leaking agent or user
    prose into the committed Markdown export.
    """
    redacted: JsonObject = {}
    for key, value in payload.items():
        policy = _evidence_policy_for_top_level(str(key), value)
        result = _apply_evidence_policy(value, policy)
        if result is _EVIDENCE_DROP:
            continue
        redacted[str(key)] = result
    return redacted


_EVIDENCE_DROP = object()


def _evidence_policy_for_top_level(key: str, value: object) -> object:
    """Pick the policy entry for a top-level payload key.

    Disambiguates the "jobs" key, which is a state-count dict in status()
    output but a list of job summary dicts in snapshot output. Other keys
    look up by name; missing keys fall through to default-deny redaction.
    """
    if key == "jobs":
        if isinstance(value, list):
            return EVIDENCE_POLICY["snapshot_jobs"]
        return EVIDENCE_POLICY["jobs"]
    return EVIDENCE_POLICY.get(key, "redacted")


def _apply_evidence_policy(value: object, policy: object) -> object:
    """Recursively apply a policy node to a value.

    Policy is one of:
      - "safe": value passes through verbatim.
      - "redacted": non-None values become the placeholder.
      - "dropped": signals omission (caller must check for _EVIDENCE_DROP).
      - dict with field-name keys: applies to dict values; "_each" applies
        to each dict element of a list; "_items" applies to each primitive
        element of a list; "_dict" applies to all values of a dict.
    """
    if policy == "safe":
        return value
    if policy == "redacted":
        if value is None:
            return None
        return EVIDENCE_FREE_TEXT_PLACEHOLDER
    if policy == "dropped":
        return _EVIDENCE_DROP
    if isinstance(policy, dict):
        if isinstance(value, list):
            if "_each" in policy:
                element_policy = policy["_each"]
                return [
                    _apply_evidence_policy(item, element_policy)
                    for item in value
                    if _apply_evidence_policy(item, element_policy) is not _EVIDENCE_DROP
                ]
            if "_items" in policy:
                item_policy = policy["_items"]
                return [
                    _apply_evidence_policy(item, item_policy)
                    for item in value
                    if _apply_evidence_policy(item, item_policy) is not _EVIDENCE_DROP
                ]
            # List with no list-shape policy: redact by default.
            return EVIDENCE_FREE_TEXT_PLACEHOLDER
        if isinstance(value, dict):
            if "_dict" in policy:
                value_policy = policy["_dict"]
                redacted_dict: JsonObject = {}
                for child_key, child_value in value.items():
                    result = _apply_evidence_policy(child_value, value_policy)
                    if result is _EVIDENCE_DROP:
                        continue
                    redacted_dict[str(child_key)] = result
                return redacted_dict
            redacted_dict = {}
            for child_key, child_value in value.items():
                child_policy = policy.get(str(child_key), "redacted")
                result = _apply_evidence_policy(child_value, child_policy)
                if result is _EVIDENCE_DROP:
                    continue
                redacted_dict[str(child_key)] = result
            return redacted_dict
        # Primitive value with a structured policy: nothing to walk; default
        # redact unless explicitly safe.
        if value is None:
            return None
        return EVIDENCE_FREE_TEXT_PLACEHOLDER
    # Unknown policy shape: redact for safety.
    if value is None:
        return None
    return EVIDENCE_FREE_TEXT_PLACEHOLDER


def blocker_summaries(conn: sqlite3.Connection, *, run_id: str | None, severity: str | None) -> list[JsonObject]:
    """Return open blocker summaries."""
    rows = conn.execute(
        """
        SELECT b.blocker_id, b.run_id, b.job_id, b.session_id, b.severity,
               b.blocker_kind, b.description, b.state, j.workflow_job_id, j.state AS job_state
        FROM blockers b
        LEFT JOIN jobs j ON j.job_id = b.job_id
        WHERE b.state = 'open'
          AND (? IS NULL OR b.run_id = ?)
          AND (? IS NULL OR b.severity = ?)
        ORDER BY b.created_at
        """,
        (run_id, run_id, severity, severity),
    ).fetchall()
    summaries: list[JsonObject] = []
    for row in rows:
        summary = dict(row)
        if row["severity"] == "human_checkpoint":
            job_id = str(row["job_id"]) if row["job_id"] is not None else None
            blocked_jobs = downstream_jobs(conn, job_id=job_id) if job_id is not None else []
            summary["human_checkpoint"] = human_checkpoint_context(summary, blocked_jobs=blocked_jobs)
        summaries.append(summary)
    return summaries


def human_checkpoint_context(blocker: JsonObject, *, blocked_jobs: list[JsonObject]) -> JsonObject:
    """Return explicit operator context for a human checkpoint."""
    affected_jobs: list[JsonObject] = []
    if blocker.get("job_id") is not None:
        affected_jobs.append(
            {
                "job_id": blocker["job_id"],
                "workflow_job_id": blocker.get("workflow_job_id"),
                "state": blocker.get("job_state"),
                "relationship": "checkpoint_job",
            }
        )
    for job in blocked_jobs:
        affected_jobs.append(
            {
                "job_id": job["job_id"],
                "workflow_job_id": job["workflow_job_id"],
                "state": job["state"],
                "relationship": "blocked_downstream",
            }
        )
    return {
        "decision_required": "Human decision required before downstream work can continue.",
        "unblock_path": [
            "inspect_checkpoint_blocker",
            "review_related_verdict_and_artifact",
            "record_owner_decision_or_adjust_workflow",
            "resume_or_requeue_affected_work",
        ],
        "affected_jobs": affected_jobs,
        "next_actions": ["inspect_blocker", "resolve_human_checkpoint", "export_run_evidence"],
    }


def latest_non_accepting_verdicts(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return latest non-accepting verdicts on waiting or failed review jobs."""
    rows = conn.execute(
        """
        SELECT v.verdict_id, v.run_id, v.job_id, j.workflow_job_id, j.state AS job_state,
               v.session_id, v.verdict, v.findings_artifact_id, v.rationale
        FROM verdicts v
        JOIN jobs j ON j.job_id = v.job_id
        WHERE j.job_type = 'review'
          AND j.state IN ('waiting_human','failed')
          AND v.verdict NOT IN ('accept','accept_with_findings')
          AND (? IS NULL OR v.run_id = ?)
          AND v.created_at = (
            SELECT MAX(v2.created_at) FROM verdicts v2 WHERE v2.job_id = v.job_id
          )
        ORDER BY v.created_at
        """,
        (run_id, run_id),
    ).fetchall()
    return [dict(row) for row in rows]


def claimable_jobs_by_role_lane(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return pending work grouped by target role and lane."""
    rows = conn.execute(
        """
        SELECT qm.target_role_id AS role_id, qm.target_lane_id AS lane_id,
               COUNT(*) AS count, GROUP_CONCAT(j.workflow_job_id) AS workflow_job_ids
        FROM queue_messages qm
        JOIN jobs j ON j.job_id = qm.job_id
        WHERE qm.kind = 'work' AND qm.state = 'pending'
          AND (? IS NULL OR qm.run_id = ?)
        GROUP BY qm.target_role_id, qm.target_lane_id
        ORDER BY qm.target_role_id, qm.target_lane_id
        """,
        (run_id, run_id),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        workflow_ids = str(row["workflow_job_ids"] or "").split(",") if row["workflow_job_ids"] else []
        result.append(
            {
                "role_id": row["role_id"],
                "lane_id": row["lane_id"],
                "count": int(row["count"]),
                "workflow_job_ids": workflow_ids,
            }
        )
    return result


def blocked_downstream_jobs(conn: sqlite3.Connection, *, run_id: str | None) -> list[JsonObject]:
    """Return blocked jobs with unsatisfied upstream dependency context."""
    jobs = conn.execute(
        """
        SELECT * FROM jobs
        WHERE state = 'blocked' AND (? IS NULL OR run_id = ?)
        ORDER BY workflow_job_id
        """,
        (run_id, run_id),
    ).fetchall()
    result: list[JsonObject] = []
    for job in jobs:
        dependencies = dependency_context(conn, job_id=str(job["job_id"]))
        if not dependencies:
            continue
        result.append(
            {
                "job_id": job["job_id"],
                "workflow_job_id": job["workflow_job_id"],
                "state": job["state"],
                "role_id": job["role_id"],
                "lane": json_loads(str(job["lane_selector_json"])).get("lane_id"),
                "blocked_by": dependencies,
            }
        )
    return result


def dependency_context(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return dependency rows with upstream state and verdict context."""
    dependencies = conn.execute(
        """
        SELECT dep.depends_on_job_id, dep.gate_json, up.workflow_job_id, up.state, up.job_type
        FROM job_dependencies dep
        JOIN jobs up ON up.job_id = dep.depends_on_job_id
        WHERE dep.job_id = ?
        ORDER BY up.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for dependency in dependencies:
        gate = json_loads(str(dependency["gate_json"]))
        verdict = latest_verdict(conn, job_id=str(dependency["depends_on_job_id"]))
        satisfied = dependency["state"] == "completed"
        required = gate.get("requires_verdict")
        if isinstance(required, list):
            satisfied = satisfied and verdict in set(required)
        if satisfied:
            continue
        result.append(
            {
                "depends_on_job_id": dependency["depends_on_job_id"],
                "workflow_job_id": dependency["workflow_job_id"],
                "state": dependency["state"],
                "required_verdicts": required,
                "latest_verdict": verdict,
            }
        )
    return result


def next_actions(
    *,
    open_blockers: list[JsonObject],
    human_checkpoints: list[JsonObject],
    non_accepting_verdicts: list[JsonObject],
    claimable_jobs: list[JsonObject],
) -> list[str]:
    """Return deterministic coordinator next-action names."""
    actions: list[str] = []
    if claimable_jobs:
        actions.append("claim_available_work")
    if open_blockers:
        actions.extend(["inspect_blocker", "export_run_evidence"])
    if human_checkpoints:
        actions.append("resolve_human_checkpoint")
    if non_accepting_verdicts:
        actions.append("revise_workflow_cycle")
    return list(dict.fromkeys(actions))


def downstream_jobs(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return immediate downstream jobs and their dependency context."""
    rows = conn.execute(
        """
        SELECT j.* FROM job_dependencies dep
        JOIN jobs j ON j.job_id = dep.job_id
        WHERE dep.depends_on_job_id = ?
        ORDER BY j.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    return [
        {
            "job_id": row["job_id"],
            "workflow_job_id": row["workflow_job_id"],
            "state": row["state"],
            "blocked_by": dependency_context(conn, job_id=str(row["job_id"])),
        }
        for row in rows
    ]


def latest_verdict_row(conn: sqlite3.Connection, *, job_id: str | None) -> JsonObject | None:
    """Return the latest verdict row for a job."""
    if job_id is None:
        return None
    row = conn.execute(
        "SELECT * FROM verdicts WHERE job_id = ? ORDER BY created_at DESC, verdict_id DESC LIMIT 1",
        (job_id,),
    ).fetchone()
    return dict(row) if row is not None else None


def blockers_for_job(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return blockers for a job."""
    rows = conn.execute("SELECT * FROM blockers WHERE job_id = ? ORDER BY created_at", (job_id,)).fetchall()
    return [dict(row) for row in rows]


def verdicts_for_artifact(conn: sqlite3.Connection, *, artifact_id: str) -> list[JsonObject]:
    """Return verdicts that cite an artifact."""
    rows = conn.execute(
        "SELECT * FROM verdicts WHERE findings_artifact_id = ? ORDER BY created_at",
        (artifact_id,),
    ).fetchall()
    return [dict(row) for row in rows]


def jobs_for_run(conn: sqlite3.Connection, *, run_id: str) -> list[sqlite3.Row]:
    """Return jobs for a run."""
    return conn.execute(
        "SELECT * FROM jobs WHERE run_id = ? ORDER BY workflow_job_id, attempt",
        (run_id,),
    ).fetchall()


def jobs_for_session(conn: sqlite3.Connection, *, session_id: str) -> list[JsonObject]:
    """Return jobs touched by a session."""
    rows = conn.execute(
        """
        SELECT DISTINCT j.*
        FROM jobs j
        LEFT JOIN leases l ON l.resource_id = j.job_id
        LEFT JOIN verdicts v ON v.job_id = j.job_id
        WHERE l.owner_session_id = ? OR v.session_id = ?
        ORDER BY j.workflow_job_id
        """,
        (session_id, session_id),
    ).fetchall()
    return [dict(row) for row in rows]


def events_for(
    conn: sqlite3.Connection,
    *,
    run_id: str | None = None,
    job_id: str | None = None,
    session_id: str | None = None,
    artifact_id: str | None = None,
) -> list[JsonObject]:
    """Return matching append-only events."""
    clauses: list[str] = []
    values: list[str] = []
    if run_id is not None:
        clauses.append("run_id = ?")
        values.append(run_id)
    if job_id is not None:
        clauses.append("job_id = ?")
        values.append(job_id)
    if session_id is not None:
        clauses.append("actor_session_id = ?")
        values.append(session_id)
    if artifact_id is not None:
        clauses.append("artifact_id = ?")
        values.append(artifact_id)
    where = " AND ".join(clauses) if clauses else "1 = 1"
    rows = conn.execute(
        f"SELECT event_id, event_type, payload_json FROM events WHERE {where} ORDER BY event_id",
        values,
    ).fetchall()
    return [dict(row) for row in rows]


def events_for_process(conn: sqlite3.Connection, *, process_id: str) -> list[JsonObject]:
    """Return process lifecycle events for a process id."""
    rows = conn.execute(
        """
        SELECT e.event_id, e.event_type, e.payload_json
        FROM events e
        JOIN process_executions p ON p.run_id = e.run_id AND p.job_id = e.job_id
        WHERE p.process_id = ?
          AND e.event_type LIKE 'process.%'
        ORDER BY e.event_id
        """,
        (process_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        payload = json_loads(str(row["payload_json"]))
        if payload.get("process_id") != process_id:
            continue
        result.append(dict(row))
    return result


def table_exists(conn: sqlite3.Connection, table_name: str) -> bool:
    """Return whether a SQLite table exists."""
    row = conn.execute(
        "SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?",
        (table_name,),
    ).fetchone()
    return row is not None


def evidence_snapshot(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Return redacted run state for evidence export."""
    run = row_by_id(conn, "runs", "run_id", run_id)
    snapshot = row_by_id(conn, "workflow_snapshots", "workflow_snapshot_id", str(run["workflow_snapshot_id"]))
    workflow = json_loads(str(snapshot["workflow_json"]))
    jobs = evidence_job_summaries(conn, run_id=run_id, workflow=workflow)
    artifacts = evidence_artifact_summaries(conn, run_id=run_id, workflow=workflow)
    verdicts = conn.execute(
        """
        SELECT verdict_id, job_id, session_id, verdict, findings_artifact_id, rationale
        FROM verdicts WHERE run_id = ? ORDER BY created_at
        """,
        (run_id,),
    ).fetchall()
    blockers = conn.execute(
        """
        SELECT blocker_id, job_id, session_id, severity, blocker_kind, description, state
        FROM blockers WHERE run_id = ? ORDER BY created_at
        """,
        (run_id,),
    ).fetchall()
    return {
        "schema_version": "striatum.evidence.v1",
        "exported_at": utc_now(),
        "workflow": {
            "workflow_id": snapshot["workflow_id"],
            "workflow_version": snapshot["workflow_version"],
        },
        "run": {
            "run_id": run["run_id"],
            "branch_name": run["branch_name"],
            "state": run["state"],
        },
        "jobs": jobs,
        "artifacts": artifacts,
        "verdicts": [dict(row) for row in verdicts],
        "blockers": [dict(row) for row in blockers],
        "blocked_downstream_jobs": blocked_downstream_jobs(conn, run_id=run_id),
    }


def evidence_job_summaries(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    workflow: JsonObject,
) -> list[JsonObject]:
    """Return redacted job summaries for evidence export."""
    summaries: list[JsonObject] = []
    for job in jobs_for_run(conn, run_id=run_id):
        lane = json_loads(str(job["lane_selector_json"])).get("lane_id")
        lane_id = lane if isinstance(lane, str) else None
        author = artifact_author_identity(
            workflow,
            role_id=str(job["role_id"]),
            lane_id=lane_id,
            workflow_job_id=str(job["workflow_job_id"]),
        )
        summaries.append(
            {
                "job_id": job["job_id"],
                "workflow_job_id": job["workflow_job_id"],
                "job_type": job["job_type"],
                "role_id": job["role_id"],
                "lane": lane_id,
                "display_model": author["display_model"],
                "author": author,
                "state": job["state"],
                "attempt": job["attempt"],
                "max_attempts": job["max_attempts"],
                "fresh_session_required": bool(job["fresh_session_required"]),
                "dependencies": dependency_summary(conn, job_id=str(job["job_id"])),
            }
        )
    return summaries


def evidence_artifact_summaries(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    workflow: JsonObject,
) -> list[JsonObject]:
    """Return artifact summaries with stable author identity."""
    rows = conn.execute(
        """
        SELECT a.artifact_id, a.job_id, a.session_id, a.logical_name,
               a.artifact_kind, a.repo_path, a.content_sha256,
               j.workflow_job_id, j.role_id, j.lane_selector_json,
               s.role_id AS session_role_id, s.lane_id AS session_lane_id,
               s.ordinal AS session_ordinal
        FROM artifacts a
        LEFT JOIN jobs j ON j.job_id = a.job_id
        LEFT JOIN sessions s ON s.session_id = a.session_id
        WHERE a.run_id = ?
        ORDER BY a.repo_path
        """,
        (run_id,),
    ).fetchall()
    artifacts: list[JsonObject] = []
    for row in rows:
        lane_id: str | None = None
        if row["lane_selector_json"] is not None:
            lane = json_loads(str(row["lane_selector_json"])).get("lane_id")
            lane_id = lane if isinstance(lane, str) else None
        artifact: JsonObject = {
            "artifact_id": row["artifact_id"],
            "job_id": row["job_id"],
            "session_id": row["session_id"],
            "logical_name": row["logical_name"],
            "artifact_kind": row["artifact_kind"],
            "repo_path": row["repo_path"],
            "content_sha256": row["content_sha256"],
        }
        if row["workflow_job_id"] is not None and row["role_id"] is not None:
            author_role = row["session_role_id"] or row["role_id"]
            author_lane = row["session_lane_id"] or lane_id
            author_ordinal = int(row["session_ordinal"]) if row["session_ordinal"] is not None else None
            artifact["author"] = artifact_author_identity(
                workflow,
                role_id=str(author_role),
                lane_id=str(author_lane) if author_lane is not None else None,
                workflow_job_id=str(row["workflow_job_id"]),
                ordinal=author_ordinal,
            )
        artifacts.append(artifact)
    return artifacts


def run_summary_snapshot(conn: sqlite3.Connection, *, repo: Path, run_id: str) -> JsonObject:
    """Return compact run facts for publishable summaries."""
    row_by_id(conn, "runs", "run_id", run_id)
    artifacts = conn.execute(
        """
        SELECT artifact_id, job_id, logical_name, artifact_kind, repo_path, content_sha256
        FROM artifacts
        WHERE run_id = ?
        ORDER BY repo_path
        """,
        (run_id,),
    ).fetchall()
    verdicts = conn.execute(
        """
        SELECT v.verdict_id, v.job_id, j.workflow_job_id, v.verdict, v.findings_artifact_id
        FROM verdicts v
        JOIN jobs j ON j.job_id = v.job_id
        WHERE v.run_id = ?
        ORDER BY v.created_at
        """,
        (run_id,),
    ).fetchall()
    blockers = conn.execute(
        """
        SELECT blocker_id, job_id, severity, blocker_kind, state
        FROM blockers
        WHERE run_id = ?
        ORDER BY created_at
        """,
        (run_id,),
    ).fetchall()
    return {
        "status": status(conn, run_id=run_id),
        "doctor": doctor(conn, repo=repo, run_id=run_id),
        "artifacts": [dict(row) for row in artifacts],
        "verdicts": [dict(row) for row in verdicts],
        "blockers": [dict(row) for row in blockers],
    }


def dependency_summary(conn: sqlite3.Connection, *, job_id: str) -> list[JsonObject]:
    """Return all upstream dependency states for export."""
    rows = conn.execute(
        """
        SELECT dep.depends_on_job_id, dep.gate_json, up.workflow_job_id, up.state
        FROM job_dependencies dep
        JOIN jobs up ON up.job_id = dep.depends_on_job_id
        WHERE dep.job_id = ?
        ORDER BY up.workflow_job_id
        """,
        (job_id,),
    ).fetchall()
    result: list[JsonObject] = []
    for row in rows:
        gate = json_loads(str(row["gate_json"]))
        result.append(
            {
                "depends_on_job_id": row["depends_on_job_id"],
                "workflow_job_id": row["workflow_job_id"],
                "state": row["state"],
                "required_verdicts": gate.get("requires_verdict"),
                "latest_verdict": latest_verdict(conn, job_id=str(row["depends_on_job_id"])),
            }
        )
    return result


def render_evidence_markdown(
    *,
    run: JsonObject,
    status_payload: JsonObject,
    doctor_payload: JsonObject,
    snapshot: JsonObject,
) -> str:
    """Render a redacted evidence snapshot as Markdown."""
    return "\n".join(
        [
            "# Striatum Evidence Export",
            "",
            f"Run ID: `{run['run_id']}`",
            f"Branch: `{run['branch_name']}`",
            f"Run state: `{run['state']}`",
            f"Exported at: `{snapshot['exported_at']}`",
            "",
            "Live SQLite state remains ignored under `.striatum/` and is not part of this export.",
            "",
            "## Status Output",
            "",
            "```json",
            json_dumps(status_payload),
            "```",
            "",
            "## Doctor Output",
            "",
            "```json",
            json_dumps(doctor_payload),
            "```",
            "",
            "## Snapshot",
            "",
            "```json",
            json.dumps(snapshot, indent=2, sort_keys=True),
            "```",
            "",
        ]
    )


def render_run_summary_markdown(*, run: JsonObject, summary: JsonObject) -> str:
    """Render a compact run note intended for durable provenance."""
    status_payload = summary["status"]
    doctor_payload = summary["doctor"]
    jobs = status_payload["jobs"]
    artifacts = summary["artifacts"]
    verdicts = summary["verdicts"]
    blockers = summary["blockers"]
    lines = [
        "# Striatum Run Summary",
        "",
        f"Run ID: `{run['run_id']}`",
        f"Branch: `{run['branch_name']}`",
        f"Run state: `{run['state']}`",
        f"Verification: `doctor ok={str(doctor_payload['ok']).lower()}`",
        "",
        "## Jobs",
        "",
    ]
    if jobs:
        for state, count in sorted(jobs.items()):
            lines.append(f"- `{state}`: {count}")
    else:
        lines.append("- No jobs recorded.")
    lines.extend(["", "## Verdicts", ""])
    if verdicts:
        for verdict in verdicts:
            lines.append(
                f"- `{verdict['verdict']}` on `{verdict['workflow_job_id']}` "
                f"({verdict['verdict_id']})"
            )
    else:
        lines.append("- No verdicts recorded.")
    lines.extend(["", "## Artifacts", ""])
    if artifacts:
        for artifact in artifacts:
            lines.append(
                f"- `{artifact['artifact_kind']}` `{artifact['logical_name']}`: "
                f"`{artifact['repo_path']}`"
            )
    else:
        lines.append("- No artifacts recorded.")
    lines.extend(["", "## Blockers", ""])
    if blockers:
        for blocker in blockers:
            lines.append(
                f"- `{blocker['state']}` `{blocker['severity']}` "
                f"`{blocker['blocker_kind']}` ({blocker['blocker_id']})"
            )
    else:
        lines.append("- No blockers recorded.")
    lines.extend(["", "## Next Actions", ""])
    next_actions = status_payload["next_actions"]
    if next_actions:
        for action in next_actions:
            lines.append(f"- `{action}`")
    else:
        lines.append("- No deterministic next actions.")
    lines.append("")
    return "\n".join(lines)


def stale_leases(conn: sqlite3.Connection, *, run_id: str) -> JsonObject:
    """Inspect stale lease recovery state for a run."""
    row_by_id(conn, "runs", "run_id", run_id)
    with transaction(conn):
        expire_leases(conn, run_id=run_id)
    from striatum.db import is_repo_write

    stale_jobs = conn.execute(
        """
        SELECT j.*, l.lease_id, l.owner_session_id, l.acquired_at, l.expires_at,
               l.released_at, l.release_reason, qm.message_id, qm.state AS message_state
        FROM jobs j
        LEFT JOIN leases l ON l.lease_id = j.current_lease_id
           OR (l.resource_id = j.job_id AND l.state = 'expired')
        LEFT JOIN queue_messages qm ON qm.message_id = j.current_message_id
        WHERE j.run_id = ? AND (j.state = 'stale_lease' OR l.state = 'expired')
        ORDER BY j.workflow_job_id, l.expires_at
        """,
        (run_id,),
    ).fetchall()
    entries: list[JsonObject] = []
    seen: set[tuple[str, str | None]] = set()
    for row in stale_jobs:
        key = (str(row["job_id"]), str(row["lease_id"]) if row["lease_id"] is not None else None)
        if key in seen:
            continue
        seen.add(key)
        repo_write = is_repo_write(row)
        entries.append(
            {
                "job_id": row["job_id"],
                "workflow_job_id": row["workflow_job_id"],
                "job_state": row["state"],
                "lease_id": row["lease_id"],
                "owner_session_id": row["owner_session_id"],
                "expires_at": row["expires_at"],
                "released_at": row["released_at"],
                "release_reason": row["release_reason"],
                "message_id": row["message_id"],
                "message_state": row["message_state"],
                "repo_write": repo_write,
                "recovery_policy": "manual_inspection_required" if repo_write else "safe_to_reclaim_when_pending",
                "next_actions": [
                    "inspect_worktree_and_artifacts",
                    "decide_requeue_or_cancel",
                ]
                if repo_write
                else ["register_or_select_session", "claim_available_work"],
            }
        )
    return {
        "run_id": run_id,
        "stale_count": len(entries),
        "stale_leases": entries,
        "next_actions": ["inspect_worktree_and_artifacts", "decide_requeue_or_cancel"]
        if entries
        else [],
    }


def requeue_stale(conn: sqlite3.Connection, *, run_id: str, job_id: str) -> JsonObject:
    """Requeue stale review-only work after lazy lease expiry."""
    row_by_id(conn, "runs", "run_id", run_id)
    from striatum.db import enqueue_job, is_repo_write

    with transaction(conn):
        expire_leases(conn, run_id=run_id)
        row = conn.execute(
            """
            SELECT j.*, l.lease_id, l.owner_session_id, l.expires_at,
                   qm.message_id, qm.state AS message_state
            FROM jobs j
            JOIN leases l ON l.resource_id = j.job_id AND l.state = 'expired'
            LEFT JOIN queue_messages qm ON qm.message_id = j.current_message_id
            WHERE j.run_id = ? AND j.job_id = ?
              AND j.state IN ('queued', 'blocked', 'stale_lease')
            ORDER BY l.expires_at DESC
            LIMIT 1
            """,
            (run_id, job_id),
        ).fetchone()
        if row is None:
            raise InvalidTransitionError("job has no stale expired lease to requeue")
        if is_repo_write(row):
            raise InvalidTransitionError("repo-write stale jobs require manual inspection")

        now = utc_now()
        already_reclaimable = row["state"] == "queued" and row["message_state"] == "pending"
        message_id = row["message_id"]
        if message_id is None:
            message_id = enqueue_job(conn, job_id=job_id)
        else:
            conn.execute(
                """
                UPDATE jobs
                SET state = 'queued', current_lease_id = NULL
                WHERE job_id = ?
                """,
                (job_id,),
            )
            conn.execute(
                """
                UPDATE queue_messages
                SET state = 'pending', current_lease_id = NULL, updated_at = ?
                WHERE message_id = ?
                """,
                (now, message_id),
            )
        insert_event(
            conn,
            run_id=run_id,
            event_type="recovery.stale_requeued",
            job_id=job_id,
            message_id=str(message_id),
            lease_id=str(row["lease_id"]),
            payload={"already_reclaimable": already_reclaimable, "repo_write": False},
        )
        return {
            "status": "already_reclaimable" if already_reclaimable else "requeued",
            "run_id": run_id,
            "job_id": job_id,
            "workflow_job_id": row["workflow_job_id"],
            "lease_id": row["lease_id"],
            "message_id": message_id,
            "repo_write": False,
            "next_actions": ["register_or_select_session", "claim_available_work"],
        }


def worktree_create(
    conn: sqlite3.Connection,
    *,
    repo: Path,
    session_id: str,
    job_id: str,
    lease_id: str,
) -> JsonObject:
    """Create a per-job git worktree for a claimed repo-write job.

    The lane must declare ``worktree_isolation: per_job``, the job must be
    repo-write, and there must be no other active worktree for the job. The
    worktree is created at ``.striatum/worktrees/<worktree_id>`` based on the
    run's confirmed branch. The directory itself is owned by git; the row is
    the source of truth for state.
    """
    job = row_by_id(conn, "jobs", "job_id", job_id)
    active_lease_for(conn, lease_id=lease_id, session_id=session_id, job_id=job_id)
    if not is_repo_write(job):
        raise InvalidTransitionError("worktree create requires a repo-write job")
    if active_worktree_for_job(conn, job_id=job_id) is not None:
        raise InvalidTransitionError("job already has an active worktree")

    workflow = workflow_for_run(conn, run_id=str(job["run_id"]))
    lane_id = job_lane_id(job)
    isolation = lane_worktree_isolation(workflow, lane_id)
    if isolation != "per_job":
        raise InvalidTransitionError(
            "lane is not configured for worktree_isolation: per_job"
        )

    run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
    base_branch = run["branch_name"]
    if base_branch is None or str(base_branch) == "":
        raise InvalidTransitionError("run has no confirmed branch for worktree base")
    if run["branch_confirmed_at"] is None:
        raise InvalidTransitionError("run branch must be confirmed before worktree create")

    worktree_id = new_id("wt")
    relative = f"{STATE_DIR}/{WORKTREES_SUBDIR}/{worktree_id}"
    target = repo / STATE_DIR / WORKTREES_SUBDIR / worktree_id
    target.parent.mkdir(parents=True, exist_ok=True)

    # Use --detach so the worktree starts at the base branch's tip without
    # conflicting with the main worktree's checkout of the same branch.
    # Operators recover work from the worktree directly; V1 does not commit.
    result = subprocess.run(
        ["git", "worktree", "add", "--detach", str(target), str(base_branch)],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        stderr = (result.stderr or result.stdout or "").strip()
        if len(stderr) > 200:
            stderr = stderr[:200] + "..."
        raise InvalidTransitionError(
            f"git worktree add failed: {stderr}" if stderr
            else "git worktree add failed"
        )

    with transaction(conn):
        now = utc_now()
        conn.execute(
            """
            INSERT INTO job_worktrees (
              worktree_id, run_id, job_id, lease_id, base_branch,
              worktree_path, state, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, 'active', ?)
            """,
            (
                worktree_id,
                str(job["run_id"]),
                job_id,
                lease_id,
                str(base_branch),
                relative,
                now,
            ),
        )
        insert_event(
            conn,
            run_id=str(job["run_id"]),
            event_type="worktree.created",
            actor_session_id=session_id,
            job_id=job_id,
            lease_id=lease_id,
            payload={
                "worktree_id": worktree_id,
                "worktree_path": relative,
                "base_branch": str(base_branch),
            },
        )
    return {
        "worktree_id": worktree_id,
        "worktree_path": relative,
        "base_branch": str(base_branch),
    }


def worktree_release(
    conn: sqlite3.Connection, *, repo: Path, worktree_id: str
) -> JsonObject:
    """Remove a per-job git worktree directory and mark the row removed.

    Idempotent: releasing a worktree that is already in a terminal state
    (``released``, ``removed``, ``abandoned``) returns success without
    rerunning ``git worktree remove``.
    """
    row = conn.execute(
        "SELECT * FROM job_worktrees WHERE worktree_id = ?",
        (worktree_id,),
    ).fetchone()
    if row is None:
        raise NotFoundError(f"could not find job_worktrees row for {worktree_id!r}")
    if row["state"] != "active":
        return {
            "status": "already_released",
            "worktree_id": worktree_id,
            "state": str(row["state"]),
        }
    target = repo / str(row["worktree_path"])
    result = subprocess.run(
        ["git", "worktree", "remove", "--force", str(target)],
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0 and target.exists():
        stderr = (result.stderr or result.stdout or "").strip()
        if len(stderr) > 200:
            stderr = stderr[:200] + "..."
        raise InvalidTransitionError(
            f"git worktree remove failed: {stderr}" if stderr
            else "git worktree remove failed"
        )

    with transaction(conn):
        now = utc_now()
        conn.execute(
            """
            UPDATE job_worktrees
            SET state = 'removed', released_at = ?, removed_at = ?
            WHERE worktree_id = ?
            """,
            (now, now, worktree_id),
        )
        insert_event(
            conn,
            run_id=str(row["run_id"]),
            event_type="worktree.released",
            job_id=str(row["job_id"]),
            lease_id=str(row["lease_id"]),
            payload={
                "worktree_id": worktree_id,
                "worktree_path": str(row["worktree_path"]),
            },
        )
    return {
        "status": "released",
        "worktree_id": worktree_id,
        "state": "removed",
    }


def worktree_list(conn: sqlite3.Connection, *, run_id: str | None) -> JsonObject:
    """Return per-job worktree rows with their workflow_job_id."""
    rows = conn.execute(
        """
        SELECT w.*, j.workflow_job_id
        FROM job_worktrees w
        JOIN jobs j ON j.job_id = w.job_id
        WHERE (? IS NULL OR w.run_id = ?)
        ORDER BY w.created_at, w.worktree_id
        """,
        (run_id, run_id),
    ).fetchall()
    return {"worktrees": [dict(row) for row in rows]}


def doctor(conn: sqlite3.Connection, *, repo: Path, run_id: str | None) -> JsonObject:
    """Return consistency checks for the state database."""
    problems: list[str] = []
    orphan_jobs = conn.execute(
        """
        SELECT j.job_id
        FROM jobs j
        LEFT JOIN leases l ON l.lease_id = j.current_lease_id AND l.state = 'active'
        WHERE j.state IN ('claimed','running') AND l.lease_id IS NULL
          AND (? IS NULL OR j.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in orphan_jobs:
        problems.append(f"active job without active lease: {row['job_id']}")
    dependencies = conn.execute(
        """
        SELECT dep.job_id, dep.depends_on_job_id, dep.gate_json
        FROM job_dependencies dep
        JOIN jobs upstream ON upstream.job_id = dep.depends_on_job_id
        WHERE (? IS NULL OR upstream.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in dependencies:
        try:
            gate = json_loads(str(row["gate_json"]))
        except (json.JSONDecodeError, InvalidTransitionError):
            problems.append(f"dependency gate_json is invalid: {row['depends_on_job_id']} -> {row['job_id']}")
            continue
        if gate.get("requires_verdict") is None:
            continue
        upstream = row_by_id(conn, "jobs", "job_id", str(row["depends_on_job_id"]))
        if upstream["state"] == "completed" and latest_verdict(conn, job_id=str(upstream["job_id"])) not in {
            "accept",
            "accept_with_findings",
        }:
            problems.append(
                "completed review dependency lacks accepting verdict: "
                f"{upstream['workflow_job_id']} -> {row['job_id']}"
            )
    jobs = conn.execute(
        "SELECT job_id, expected_artifacts_json FROM jobs WHERE (? IS NULL OR run_id = ?)",
        (run_id, run_id),
    ).fetchall()
    for job in jobs:
        expected = json.loads(str(job["expected_artifacts_json"]))
        if not isinstance(expected, list):
            continue
        for item in expected:
            if not isinstance(item, dict) or item.get("required") is not True:
                continue
            logical_name = item.get("logical_name")
            existing = conn.execute(
                "SELECT artifact_kind, repo_path FROM artifacts WHERE job_id = ? AND logical_name = ?",
                (job["job_id"], logical_name),
            ).fetchone()
            if existing is None:
                continue
            if existing["artifact_kind"] != item.get("kind") or existing["repo_path"] != item.get("path"):
                problems.append(
                    "required artifact mismatch: "
                    f"job_id={job['job_id']}, logical_name={logical_name!r}, "
                    f"expected kind={item.get('kind')!r}, path={item.get('path')!r}"
                )
    stuck_runs = conn.execute(
        """
        SELECT r.run_id
        FROM runs r
        WHERE r.state = 'running'
          AND (? IS NULL OR r.run_id = ?)
          AND NOT EXISTS (
            SELECT 1 FROM jobs j
            WHERE j.run_id = r.run_id
              AND j.state IN (
                'queued','claimed','running','blocked','stale_lease','waiting_human'
              )
          )
          AND EXISTS (
            SELECT 1 FROM jobs j
            WHERE j.run_id = r.run_id AND j.state = 'failed'
          )
        """,
        (run_id, run_id),
    ).fetchall()
    for row in stuck_runs:
        problems.append(f"run is running but no progressable jobs remain: {row['run_id']}")
    stale_messages = conn.execute(
        """
        SELECT m.message_id
        FROM queue_messages m
        LEFT JOIN leases l ON l.lease_id = m.current_lease_id AND l.state = 'active'
        WHERE m.state IN ('claimed','acked')
          AND m.current_lease_id IS NOT NULL
          AND l.lease_id IS NULL
          AND (? IS NULL OR m.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in stale_messages:
        problems.append(f"queue message has stale claim: {row['message_id']}")
    bad_message_pointers = conn.execute(
        """
        SELECT j.job_id
        FROM jobs j
        LEFT JOIN queue_messages m ON m.message_id = j.current_message_id
        WHERE j.current_message_id IS NOT NULL
          AND (m.message_id IS NULL OR m.job_id IS NOT j.job_id)
          AND (? IS NULL OR j.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in bad_message_pointers:
        problems.append(f"job current_message_id is inconsistent: {row['job_id']}")
    bad_lease_pointers = conn.execute(
        """
        SELECT j.job_id
        FROM jobs j
        LEFT JOIN leases l ON l.lease_id = j.current_lease_id
        WHERE j.current_lease_id IS NOT NULL
          AND (l.lease_id IS NULL OR l.resource_id IS NOT j.job_id)
          AND (? IS NULL OR j.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in bad_lease_pointers:
        problems.append(f"job current_lease_id is inconsistent: {row['job_id']}")
    terminal_sessions = conn.execute(
        """
        SELECT s.session_id
        FROM sessions s
        JOIN runs r ON r.run_id = s.run_id
        WHERE s.state = 'active'
          AND r.state IN ('completed','failed','canceled')
          AND (? IS NULL OR s.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in terminal_sessions:
        problems.append(f"active session on terminal run: {row['session_id']}")
    expired_leases = conn.execute(
        """
        SELECT l.lease_id
        FROM leases l
        WHERE l.state = 'active'
          AND l.expires_at < ?
          AND (? IS NULL OR l.run_id = ?)
        """,
        (utc_now(), run_id, run_id),
    ).fetchall()
    for row in expired_leases:
        problems.append(f"active lease has expired without reap: {row['lease_id']}")
    open_blockers = conn.execute(
        """
        SELECT b.blocker_id
        FROM blockers b
        JOIN runs r ON r.run_id = b.run_id
        WHERE b.state = 'open'
          AND r.state IN ('completed','canceled')
          AND (? IS NULL OR b.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in open_blockers:
        problems.append(f"open blocker on terminal run: {row['blocker_id']}")
    orphan_packets = conn.execute(
        """
        SELECT p.packet_id
        FROM work_packets p
        LEFT JOIN leases l ON l.lease_id = p.lease_id
        LEFT JOIN sessions s ON s.session_id = p.session_id
        WHERE (l.lease_id IS NULL OR s.session_id IS NULL)
          AND (? IS NULL OR p.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in orphan_packets:
        problems.append(f"work packet references missing lease/session: {row['packet_id']}")
    orphan_worktrees = conn.execute(
        """
        SELECT w.worktree_id
        FROM job_worktrees w
        LEFT JOIN leases l ON l.lease_id = w.lease_id AND l.state = 'active'
        WHERE w.state = 'active' AND l.lease_id IS NULL
          AND (? IS NULL OR w.run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in orphan_worktrees:
        problems.append(f"active worktree without active lease: {row['worktree_id']}")
    drifted_worktrees = conn.execute(
        """
        SELECT worktree_id, worktree_path
        FROM job_worktrees
        WHERE state = 'active' AND (? IS NULL OR run_id = ?)
        """,
        (run_id, run_id),
    ).fetchall()
    for row in drifted_worktrees:
        target = repo / str(row["worktree_path"])
        if not target.exists():
            problems.append(
                "active worktree directory missing on disk: "
                f"{row['worktree_id']} ({row['worktree_path']})"
            )
    schema_version = conn.execute(
        "SELECT value FROM schema_meta WHERE key = 'schema_version'"
    ).fetchone()
    return {
        "ok": len(problems) == 0,
        "schema_version": schema_version["value"] if schema_version is not None else None,
        "problems": problems,
    }


if __name__ == "__main__":
    raise SystemExit(main())

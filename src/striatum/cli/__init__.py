"""Command line interface package.

The package keeps historical ``from striatum.cli import ...`` imports working
without importing legacy SQLite-heavy helper modules during a plain package
import. Runtime code should prefer importing concrete submodules directly.
"""

from __future__ import annotations

from importlib import import_module
from typing import Any


_SYMBOL_MODULES = {
    "EVIDENCE_FREE_TEXT_PLACEHOLDER": "striatum.evidence_presentation",
    "EVIDENCE_POLICY": "striatum.evidence_presentation",
    "SUPERVISOR_ACTIVE_STATES": "striatum.cli.supervise",
    "ack_work": "striatum.cli.mutations",
    "add_work_identity": "striatum.cli.parser",
    "block_work": "striatum.cli.mutations",
    "blocked_downstream_jobs": "striatum.cli.introspect",
    "blocker_summaries": "striatum.cli.introspect",
    "blockers_for_job": "striatum.cli.introspect",
    "branch_confirm": "striatum.cli.mutations",
    "build_parser": "striatum.cli.parser",
    "cancel_job": "striatum.cli.recovery",
    "checkpoint_resolve": "striatum.cli.mutations",
    "claimable_jobs_by_role_lane": "striatum.cli.introspect",
    "current_git_branch": "striatum.git_helpers",
    "decision_record": "striatum.cli.mutations",
    "dependency_context": "striatum.cli.introspect",
    "dependency_summary": "striatum.cli.evidence",
    "dispatch": "striatum.cli.dispatch",
    "doctor": "striatum.cli.introspect",
    "downstream_jobs": "striatum.cli.introspect",
    "events_for": "striatum.cli.introspect",
    "events_for_process": "striatum.cli.introspect",
    "evidence_artifact_summaries": "striatum.cli.evidence",
    "evidence_export": "striatum.cli.evidence",
    "evidence_job_summaries": "striatum.cli.evidence",
    "evidence_snapshot": "striatum.cli.evidence",
    "git_create_or_checkout_branch": "striatum.cli.mutations",
    "heartbeat": "striatum.cli.mutations",
    "human_checkpoint_context": "striatum.cli.introspect",
    "jobs_for_run": "striatum.cli.introspect",
    "jobs_for_session": "striatum.cli.introspect",
    "latest_non_accepting_verdicts": "striatum.cli.introspect",
    "latest_verdict_row": "striatum.cli.introspect",
    "list_artifacts": "striatum.cli.list_commands",
    "list_jobs": "striatum.cli.list_commands",
    "list_runs": "striatum.cli.list_commands",
    "list_sessions": "striatum.cli.list_commands",
    "list_workflows": "striatum.cli.list_commands",
    "main": "striatum.cli.dispatch",
    "next_actions": "striatum.next_actions",
    "prevalidate_submit_review": "striatum.cli.mutations",
    "recent_events_for_run": "striatum.cli.introspect",
    "redact_evidence_payload": "striatum.evidence_presentation",
    "register_session": "striatum.cli.mutations",
    "release_work": "striatum.cli.mutations",
    "render_decision_markdown": "striatum.cli.mutations",
    "render_evidence_markdown": "striatum.evidence_presentation",
    "render_run_summary_markdown": "striatum.run_summary_format",
    "requeue_stale": "striatum.cli.recovery",
    "resume_blocker": "striatum.cli.recovery",
    "run_graph": "striatum.cli.introspect",
    "run_start": "striatum.cli.mutations",
    "run_summary_export": "striatum.cli.run_summary",
    "run_summary_snapshot": "striatum.cli.run_summary",
    "send_message": "striatum.cli.mutations",
    "stale_leases": "striatum.cli.recovery",
    "status": "striatum.cli.introspect",
    "submit_review": "striatum.cli.mutations",
    "supervise_list": "striatum.cli.supervise",
    "supervise_send": "striatum.cli.supervise",
    "supervise_start": "striatum.cli.supervise",
    "supervise_status": "striatum.cli.supervise",
    "supervise_stop": "striatum.cli.supervise",
    "table_exists": "striatum.cli.introspect",
    "verdict_work": "striatum.cli.mutations",
    "verdicts_for_artifact": "striatum.cli.introspect",
    "why": "striatum.cli.introspect",
    "workflow_init": "striatum.cli.workflow_init",
    "worktree_create": "striatum.cli.worktree",
    "worktree_list": "striatum.cli.worktree",
    "worktree_release": "striatum.cli.worktree",
}


__all__ = sorted(_SYMBOL_MODULES)


def __getattr__(name: str) -> Any:
    module_name = _SYMBOL_MODULES.get(name)
    if module_name is None:
        raise AttributeError(f"module 'striatum.cli' has no attribute {name!r}")
    module = import_module(module_name)
    value = getattr(module, name)
    globals()[name] = value
    return value

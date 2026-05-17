"""Doctor page presentation helpers."""

from __future__ import annotations

from typing import Any, Mapping


JsonObject = dict[str, Any]


def doctor_record_recipes(record: Mapping[str, Any]) -> list[str]:
    check = str(record.get("check") or "")
    context = record.get("context")
    ctx = context if isinstance(context, dict) else {}
    run_id = str(ctx.get("run_id") or record.get("run_id") or "")
    job_id = str(ctx.get("job_id") or record.get("job_id") or "")
    session_id = str(ctx.get("session_id") or record.get("session_id") or "")
    blocker_id = str(ctx.get("blocker_id") or record.get("blocker_id") or "")
    recipes: list[str] = []
    if check in {"process_running_but_pid_gone", "process_running_with_expired_lease"} and run_id:
        recipes.append(f"striatum recovery process-reconcile --run-id {run_id}")
    elif check == "supervisor_lost_with_held_lease":
        if run_id and job_id:
            recipes.append(
                f'striatum recovery cancel-job --run-id {run_id} --job-id {job_id} --reason "supervisor lost with held lease"'
            )
        if session_id:
            recipes.append(f"striatum supervise stop --session-id {session_id}")
    elif check == "active_session_on_terminal_run" and session_id:
        recipes.append(f"striatum session close --session-id {session_id} --reason terminal_run_cleanup")
    elif check in {"orphaned_worktree", "missing_worktree"} and run_id:
        recipes.append(f"striatum doctor --run-id {run_id} --verbose")
    elif check == "human_checkpoint_open" and blocker_id:
        recipes.append(f"striatum checkpoint resolve --blocker-id {blocker_id} --action continue")
        recipes.append(f"striatum checkpoint resolve --blocker-id {blocker_id} --action cancel")
    return recipes


def shape_doctor_records(records: list[Any]) -> list[JsonObject]:
    shaped: list[JsonObject] = []
    for record in records:
        if not isinstance(record, dict):
            continue
        item = dict(record)
        item["recipes"] = doctor_record_recipes(item)
        shaped.append(item)
    return shaped

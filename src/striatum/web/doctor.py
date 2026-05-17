"""Doctor page presentation helpers."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Mapping


JsonObject = dict[str, Any]
DoctorPayloadLoader = Callable[[Path], Mapping[str, Any]]
LegacyFallbackPredicate = Callable[[str], bool]


@dataclass(frozen=True)
class DoctorPageError(Exception):
    status: int
    code: str
    message: str


@dataclass(frozen=True)
class DoctorPageResponse:
    doctor: JsonObject
    problem_groups: dict[str, list[JsonObject]]


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


def doctor_page_response(
    repo: Path,
    *,
    legacy_fallback_enabled: LegacyFallbackPredicate,
    legacy_payload: DoctorPayloadLoader,
) -> DoctorPageResponse:
    from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

    doctor_payload: Mapping[str, Any]
    try:
        doctor_payload = call_repo_method(repo, "doctor", {"verbose": True})
    except ServiceDaemonRpcError as exc:
        if not legacy_fallback_enabled(exc.code):
            raise DoctorPageError(exc.status, exc.code, exc.message) from exc
        doctor_payload = legacy_payload(repo)
    raw_records = doctor_payload.get("problem_records")
    records = shape_doctor_records(raw_records if isinstance(raw_records, list) else [])
    doctor = dict(doctor_payload)
    doctor["problem_records"] = records
    groups: dict[str, list[JsonObject]] = {}
    for record in records:
        groups.setdefault(str(record.get("check") or "unknown"), []).append(record)
    return DoctorPageResponse(doctor=doctor, problem_groups=groups)

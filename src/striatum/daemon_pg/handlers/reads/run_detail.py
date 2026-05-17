"""PG handler for ``run.detail``."""

from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from striatum.daemon_pg.handlers.context import RepoHandlerContext, session_lane_attestation
from striatum.daemon_rpc.envelope import RpcError
from striatum.errors import WorkflowError
from striatum.workflow import workflow_phase_index

from ._read_model import status_payload, workflow_for_run
from ._registry import register_pg_handler
from ._sql import parse_json_list, parse_json_object, require_text, row_to_json

_PHASE_ACTIVE_STATES = {"queued", "claimed", "running", "stale_lease", "waiting_human", "blocked"}


@register_pg_handler("run.detail", read_only=True)
def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    run_id = require_text(params, "run_id")
    run, workflow = _run_and_workflow(ctx, run_id=run_id)
    jobs = _jobs(ctx, run_id=run_id)
    latest_verdicts = _latest_verdicts(ctx, job_ids=[str(job["job_id"]) for job in jobs])
    override_verdict_ids = _override_verdict_ids(ctx, run_id=run_id)
    for job in jobs:
        job["state_chip"] = _state_chip("job", job.get("state"))
        job["latest_verdict"] = _shape_latest_verdict(
            latest_verdicts.get(str(job["job_id"])),
            override_verdict_ids=override_verdict_ids,
        )
        job["lane_attestation_chip"] = _lane_attestation_chip(
            ctx,
            session_id=_session_id_for_job(ctx, job),
        )

    status = status_payload(ctx, run_id=run_id)
    next_actions = [str(action) for action in status.get("next_actions") or []]
    phase = _phase_progress(workflow=workflow, jobs=jobs, latest_verdicts=latest_verdicts)
    branch_raw = workflow.get("branch")
    branch = branch_raw if isinstance(branch_raw, Mapping) else {}
    run["state_chip"] = _state_chip("run", run.get("state"))
    return {
        "run": run,
        "workflow": workflow,
        "jobs": jobs,
        "next_actions": next_actions,
        "recovery_panel": _recovery_panel(
            ctx,
            run_id=run_id,
            next_actions=next_actions,
            auto_finalize_dry_run=(
                status.get("auto_finalize_dry_run")
                if isinstance(status.get("auto_finalize_dry_run"), Mapping)
                else None
            ),
        ),
        "verdicts_by_posture": _verdicts_by_posture(ctx, run_id=run_id),
        "sessions": status.get("sessions") or [],
        "phase_progress": phase.get("phases", []),
        "current_phase_id": phase.get("current_phase_id"),
        "suggested_branch_name": str(branch.get("suggested_name") or ""),
        "allow_dirty": bool(branch.get("allow_dirty")),
    }


def _run_and_workflow(ctx: RepoHandlerContext, *, run_id: str) -> tuple[dict[str, Any], dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT r.*
            FROM striatumd.runs r
            WHERE r.repository_id = %s AND r.run_id = %s
            """,
            (ctx.repository_id, run_id),
        )
        row = cur.fetchone()
    if row is None:
        raise RpcError("not_found", f"run not found: {run_id}")
    return row_to_json(row), workflow_for_run(ctx, run_id)


def _jobs(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT j.*
            FROM striatumd.jobs j
            WHERE j.repository_id = %s AND j.run_id = %s
            ORDER BY j.workflow_job_id, j.attempt
            """,
            (ctx.repository_id, run_id),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    jobs: list[dict[str, Any]] = []
    for row in rows:
        item = dict(row)
        item["lane_id"] = parse_json_object(item.get("lane_selector_json")).get("lane_id")
        item["expected_artifacts"] = parse_json_list(item.get("expected_artifacts_json"))
        jobs.append(item)
    return jobs


def _latest_verdicts(ctx: RepoHandlerContext, *, job_ids: list[str]) -> dict[str, dict[str, Any]]:
    if not job_ids:
        return {}
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT DISTINCT ON (v.job_id)
                   v.*
            FROM striatumd.verdicts v
            WHERE v.repository_id = %s AND v.job_id = ANY(%s)
            ORDER BY v.job_id, v.created_at DESC, v.verdict_id DESC
            """,
            (ctx.repository_id, job_ids),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    return {str(row["job_id"]): row for row in rows}


def _override_verdict_ids(ctx: RepoHandlerContext, *, run_id: str) -> set[str]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT e.payload_json
            FROM striatumd.events e
            WHERE e.repository_id = %s
              AND e.run_id = %s
              AND e.event_type = 'verdict.overridden'
            """,
            (ctx.repository_id, run_id),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    verdict_ids: set[str] = set()
    for row in rows:
        payload = parse_json_object(row.get("payload_json"))
        verdict_id = payload.get("verdict_id")
        if isinstance(verdict_id, str) and verdict_id:
            verdict_ids.add(verdict_id)
    return verdict_ids


def _shape_latest_verdict(
    verdict: Mapping[str, Any] | None,
    *,
    override_verdict_ids: set[str],
) -> dict[str, Any] | None:
    if verdict is None:
        return None
    item = dict(verdict)
    provenance = (
        "operator-override"
        if item.get("verdict_id") in override_verdict_ids
        else "natural"
    )
    item["provenance"] = provenance
    item["source"] = provenance
    item["override_rationale"] = (
        item.get("rationale") if provenance == "operator-override" else None
    )
    return item


def _session_id_for_job(ctx: RepoHandlerContext, job: Mapping[str, Any]) -> str | None:
    lease_id = job.get("current_lease_id")
    if isinstance(lease_id, str) and lease_id:
        with ctx.cursor() as cur:
            cur.execute(
                """
                SELECT l.owner_session_id
                FROM striatumd.leases l
                WHERE l.repository_id = %s AND l.lease_id = %s
                """,
                (ctx.repository_id, lease_id),
            )
            row = cur.fetchone()
        if row is not None:
            return str(row["owner_session_id"])
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT wp.session_id
            FROM striatumd.work_packets wp
            WHERE wp.repository_id = %s AND wp.job_id = %s
            ORDER BY wp.created_at DESC, wp.packet_id DESC
            LIMIT 1
            """,
            (ctx.repository_id, job.get("job_id")),
        )
        row = cur.fetchone()
    return str(row["session_id"]) if row is not None else None


def _lane_attestation_chip(ctx: RepoHandlerContext, *, session_id: str | None) -> dict[str, Any]:
    if not session_id:
        return {
            "state": "unattested",
            "attested": False,
            "reason": "session_missing",
            "supervisor_id": None,
            "label": "unattested",
        }
    attestation = session_lane_attestation(ctx, session_id=session_id)
    return {
        "state": attestation.get("state"),
        "attested": bool(attestation.get("attested")),
        "reason": attestation.get("reason"),
        "supervisor_id": attestation.get("supervisor_id"),
        "label": attestation.get("state"),
    }


def _recovery_panel(
    ctx: RepoHandlerContext,
    *,
    run_id: str,
    next_actions: list[str],
    auto_finalize_dry_run: Mapping[str, Any] | None,
) -> dict[str, Any]:
    blockers = _open_blockers(ctx, run_id=run_id)
    auto_publish_recipe = (
        f"striatum recovery auto-publish --run-id {run_id} --dry-run"
        if "recovery_auto_publish" in set(next_actions)
        else None
    )
    recipes: list[dict[str, str]] = []
    if auto_publish_recipe:
        recipes.append({"label": "Auto-publish dry run", "command": auto_publish_recipe})
    for blocker in blockers:
        for recipe in blocker.get("recipes") or []:
            recipes.append(
                {
                    "label": str(blocker.get("blocker_kind") or "Recovery command"),
                    "command": str(recipe),
                }
            )
    return {
        "run_id": run_id,
        "blockers": blockers,
        "human_checkpoints": [b for b in blockers if b.get("severity") == "human_checkpoint"],
        "blocked": [b for b in blockers if b.get("severity") != "human_checkpoint"],
        "next_actions": next_actions,
        "auto_publish_recipe": auto_publish_recipe,
        "auto_finalize_dry_run": dict(auto_finalize_dry_run or {}),
        "recipes": recipes,
    }


def _open_blockers(ctx: RepoHandlerContext, *, run_id: str) -> list[dict[str, Any]]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT b.*, j.workflow_job_id, j.job_type, j.state AS job_state
            FROM striatumd.blockers b
            LEFT JOIN striatumd.jobs j
              ON j.repository_id = b.repository_id
             AND j.job_id = b.job_id
            WHERE b.repository_id = %s
              AND b.run_id = %s
              AND b.state = 'open'
            ORDER BY
              CASE b.severity WHEN 'human_checkpoint' THEN 0 WHEN 'blocked' THEN 1 ELSE 2 END,
              b.created_at
            """,
            (ctx.repository_id, run_id),
        )
        rows = [row_to_json(row) for row in cur.fetchall()]
    blockers: list[dict[str, Any]] = []
    for row in rows:
        item = dict(row)
        payload = parse_json_object(item.get("payload_json"))
        item["payload"] = payload
        item["recipes"] = _recipes_for_blocker(item)
        blockers.append(item)
    return blockers


def _recipes_for_blocker(blocker: Mapping[str, Any]) -> list[str]:
    payload = blocker.get("payload")
    payload_obj = payload if isinstance(payload, dict) else {}
    recipes = payload_obj.get("recovery_commands")
    if isinstance(recipes, list):
        return [str(recipe) for recipe in recipes if recipe]
    run_id = str(blocker.get("run_id") or "")
    job_id = str(blocker.get("job_id") or "")
    blocker_id = str(blocker.get("blocker_id") or "")
    kind = str(blocker.get("blocker_kind") or "")
    severity = str(blocker.get("severity") or "")
    if severity == "human_checkpoint":
        return [
            f"striatum checkpoint resolve --blocker-id {blocker_id} --action continue",
            f"striatum checkpoint resolve --blocker-id {blocker_id} --action cancel",
        ]
    if kind.startswith("process_") and run_id:
        return [f"striatum recovery process-reconcile --run-id {run_id}"]
    if job_id:
        return [
            f'striatum recovery cancel-job --run-id {run_id} --job-id {job_id} --reason "operator inspected blocker {blocker_id}"'
        ]
    return []


def _verdicts_by_posture(ctx: RepoHandlerContext, *, run_id: str) -> dict[str, int]:
    with ctx.cursor() as cur:
        cur.execute(
            """
            SELECT v.posture, COUNT(*) AS count
            FROM striatumd.verdicts v
            WHERE v.repository_id = %s AND v.run_id = %s
            GROUP BY v.posture
            ORDER BY v.posture
            """,
            (ctx.repository_id, run_id),
        )
        return {str(row["posture"]): int(row["count"]) for row in cur.fetchall()}


def _phase_progress(
    *,
    workflow: Mapping[str, Any],
    jobs: list[dict[str, Any]],
    latest_verdicts: Mapping[str, Mapping[str, Any]],
) -> dict[str, Any]:
    try:
        phase_index = workflow_phase_index(dict(workflow))
    except WorkflowError:
        return {"phases": [], "current_phase_id": None}
    if phase_index.get("declared") is not True:
        return {"phases": [], "current_phase_id": None}

    latest_jobs: dict[str, dict[str, Any]] = {}
    for job in sorted(jobs, key=lambda row: int(row.get("attempt") or 0), reverse=True):
        latest_jobs.setdefault(str(job["workflow_job_id"]), job)

    phases: list[dict[str, Any]] = []
    phase_order = list(phase_index["phase_order"])
    phase_by_id = phase_index["phase_by_id"]
    job_phase = phase_index["job_phase"]
    synthesis_by_phase = phase_index["synthesis_by_phase"]
    jobs_by_phase: dict[str, list[str]] = {str(phase_id): [] for phase_id in phase_order}
    for workflow_job_id, phase_id in job_phase.items():
        jobs_by_phase.setdefault(str(phase_id), []).append(str(workflow_job_id))

    for index, phase_id_value in enumerate(phase_order):
        phase_id = str(phase_id_value)
        phase = phase_by_id[phase_id]
        workflow_job_ids = jobs_by_phase.get(phase_id, [])
        jobs_by_state: dict[str, int] = {}
        job_states: list[str] = []
        for workflow_job_id in workflow_job_ids:
            row = latest_jobs.get(workflow_job_id)
            state = str(row["state"]) if row is not None else "pending"
            job_states.append(state)
            jobs_by_state[state] = jobs_by_state.get(state, 0) + 1
        synthesis_job_id = synthesis_by_phase.get(phase_id)
        synthesis_row = latest_jobs.get(synthesis_job_id or "")
        synthesis_verdict = (
            latest_verdicts.get(str(synthesis_row["job_id"]))
            if synthesis_row is not None
            else None
        )
        phase_payload: dict[str, Any] = {
            "id": phase_id,
            "name": str(phase.get("name") or phase_id),
            "index": index,
            "state": _derive_phase_state(job_states),
            "jobs_total": len(workflow_job_ids),
            "jobs_completed": jobs_by_state.get("completed", 0),
            "jobs_by_state": jobs_by_state,
            "synthesis_job_id": synthesis_job_id,
            "synthesis_state": str(synthesis_row["state"]) if synthesis_row is not None else None,
            "synthesis_verdict": synthesis_verdict,
        }
        for optional_key in ("description", "color"):
            optional_value = phase.get(optional_key)
            if isinstance(optional_value, str):
                phase_payload[optional_key] = optional_value
        phases.append(phase_payload)

    if not phases:
        return {"phases": [], "current_phase_id": None}
    current_phase_id = phases[-1]["id"]
    for phase in phases:
        if phase["state"] != "completed":
            current_phase_id = phase["id"]
            break
    return {"phases": phases, "current_phase_id": current_phase_id}


def _derive_phase_state(job_states: list[str]) -> str:
    if job_states and all(state == "completed" for state in job_states):
        return "completed"
    if any(state in _PHASE_ACTIVE_STATES for state in job_states):
        return "active"
    if any(state == "failed" for state in job_states):
        return "failed"
    incomplete = [state for state in job_states if state != "completed"]
    if incomplete and all(state in {"canceled", "skipped"} for state in incomplete):
        return "canceled"
    return "pending"


def _state_chip(kind: str, state: Any) -> dict[str, str]:
    normalized = str(state or "unknown")
    return {
        "kind": kind,
        "state": normalized,
        "label": normalized,
        "css_class": f"status-pill status-{normalized}",
    }

"""RFC 0040 V1 workflow CLI helpers.

Currently exposes :func:`workflow_upgrade` for the
``striatum workflow upgrade <path>`` verb. The upgrade backports the
per-model harness-profile fragments from the bundled template catalog
(see :mod:`striatum.workflow_generator.catalog`) into existing
``workflow.json`` files so older workflows pick up the RFC 0040 §5/§6
no-questions instruction and the gemini front-matter completeness
callout.

Refuse-on-conflict default: if a profile's existing
``native_delegation.instruction`` matches neither the catalog default
nor an empty/missing value, the upgrade refuses unless ``--force``.
``--dry-run`` reports the change set without writing. The verb also
refuses if any non-terminal run in the target repository references
the workflow being upgraded (a "running workflow" guard).
"""

from __future__ import annotations

import json
import os
import re
import sqlite3
from pathlib import Path

from striatum.db import JsonObject, db_path
from striatum.errors import WorkflowError
from striatum.workflow import load_workflow
from striatum.workflow_generator.catalog import get_harness_fragment_by_tool_family


_TERMINAL_RUN_STATES: frozenset[str] = frozenset({"completed", "failed", "canceled"})


def workflow_upgrade(
    path: Path,
    *,
    repo: Path,
    force: bool = False,
    dry_run: bool = False,
    add_phases: bool = False,
    apply: bool = False,
) -> JsonObject:
    """Backport RFC 0040 V1 harness-profile fragments into ``path``.

    Returns a JSON envelope describing the change set:

    - ``status`` — ``updated``, ``no_changes``, ``would_update``,
      ``would_no_changes``, ``refused_conflict``, ``refused_running``.
    - ``changes`` — list of {profile_id, field, old_value, new_value}.
    - ``conflicts`` — list of {profile_id, field, current_value,
      catalog_value} when conflicts block the upgrade (set when
      ``status == "refused_conflict"``).

    Raises :class:`WorkflowError` for unrecoverable errors (missing
    file, invalid workflow, running-workflow refusal, conflict
    refusal). The CLI dispatcher translates these into the standard
    error envelope.
    """
    workflow_path = path
    if not workflow_path.exists():
        raise WorkflowError(
            f"workflow upgrade target does not exist: {workflow_path}",
            field_path="path",
        )
    if not workflow_path.is_file():
        raise WorkflowError(
            f"workflow upgrade target is not a regular file: {workflow_path}",
            field_path="path",
        )
    workflow = load_workflow(workflow_path)
    if add_phases:
        return _workflow_upgrade_add_phases(
            workflow_path=workflow_path,
            workflow=workflow,
            repo=repo,
            apply=apply,
        )
    profiles = workflow.get("harness_profiles")
    if not isinstance(profiles, dict) or not profiles:
        return {
            "workflow_path": str(workflow_path),
            "status": "would_no_changes" if dry_run else "no_changes",
            "changes": [],
            "conflicts": [],
            "note": "workflow has no harness_profiles section; nothing to upgrade",
        }

    running_runs = _running_runs_for_workflow(repo=repo, workflow_path=workflow_path)
    if running_runs and not dry_run:
        raise WorkflowError(
            "workflow upgrade refuses to mutate a workflow with non-terminal runs: "
            + ", ".join(running_runs),
            field_path="path",
        )

    changes: list[JsonObject] = []
    conflicts: list[JsonObject] = []
    updated_profiles: JsonObject = {}
    for profile_id, body in profiles.items():
        if not isinstance(body, dict):
            updated_profiles[profile_id] = body
            continue
        new_body = dict(body)
        fragment = get_harness_fragment_by_tool_family(str(body.get("tool_family", "")))
        if fragment is None:
            updated_profiles[profile_id] = new_body
            continue
        native_raw = new_body.get("native_delegation")
        native = dict(native_raw) if isinstance(native_raw, dict) else {}
        catalog_instruction = str(fragment["native_delegation_instruction"])
        current_instruction = native.get("instruction")
        if current_instruction == catalog_instruction:
            # Already up-to-date.
            pass
        elif current_instruction is None or (
            isinstance(current_instruction, str) and not current_instruction.strip()
        ):
            native["instruction"] = catalog_instruction
            changes.append({
                "profile_id": profile_id,
                "field": "native_delegation.instruction",
                "old_value": current_instruction,
                "new_value": catalog_instruction,
            })
        else:
            if force:
                native["instruction"] = catalog_instruction
                changes.append({
                    "profile_id": profile_id,
                    "field": "native_delegation.instruction",
                    "old_value": current_instruction,
                    "new_value": catalog_instruction,
                    "forced": True,
                })
            else:
                conflicts.append({
                    "profile_id": profile_id,
                    "field": "native_delegation.instruction",
                    "current_value": current_instruction,
                    "catalog_value": catalog_instruction,
                })
        catalog_mode = fragment.get("native_delegation_mode")
        if (
            "mode" not in native
            and isinstance(catalog_mode, str)
            and catalog_mode
        ):
            native["mode"] = catalog_mode
            changes.append({
                "profile_id": profile_id,
                "field": "native_delegation.mode",
                "old_value": None,
                "new_value": catalog_mode,
            })
        if native:
            new_body["native_delegation"] = native
        updated_profiles[profile_id] = new_body

    if conflicts and not force:
        return {
            "workflow_path": str(workflow_path),
            "status": "refused_conflict",
            "changes": changes,
            "conflicts": conflicts,
            "hint": "rerun with --force to overwrite, or edit the profile to match the catalog first",
        }

    if running_runs and dry_run:
        return {
            "workflow_path": str(workflow_path),
            "status": "would_refuse_running",
            "changes": changes,
            "conflicts": conflicts,
            "running_runs": running_runs,
        }

    if not changes:
        return {
            "workflow_path": str(workflow_path),
            "status": "would_no_changes" if dry_run else "no_changes",
            "changes": [],
            "conflicts": [],
        }

    if dry_run:
        return {
            "workflow_path": str(workflow_path),
            "status": "would_update",
            "changes": changes,
            "conflicts": [],
        }

    updated_workflow = dict(workflow)
    updated_workflow["harness_profiles"] = updated_profiles
    workflow_path.write_text(
        json.dumps(updated_workflow, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )
    return {
        "workflow_path": str(workflow_path),
        "status": "updated",
        "changes": changes,
        "conflicts": [],
    }


def _workflow_upgrade_add_phases(
    *,
    workflow_path: Path,
    workflow: JsonObject,
    repo: Path,
    apply: bool,
) -> JsonObject:
    """Rewrite a v1 workflow to v1.1 with inferred phases."""
    if workflow.get("schema_version") != "striatum.workflow.v1":
        raise WorkflowError(
            "workflow upgrade --add-phases only accepts striatum.workflow.v1 inputs",
            field_path="schema_version",
        )
    if workflow.get("phases"):
        return {
            "workflow_path": str(workflow_path),
            "status": "no_changes" if apply else "would_no_changes",
            "mode": "add_phases",
            "phases_added": [],
            "jobs_relabelled": [],
            "jobs_inserted": [],
            "edges_rewritten": [],
            "warnings": ["workflow already declares phases"],
        }

    running_runs = _running_runs_for_workflow(repo=repo, workflow_path=workflow_path)
    if running_runs and apply:
        raise WorkflowError(
            "workflow upgrade refuses to mutate a workflow with non-terminal runs: "
            + ", ".join(running_runs),
            field_path="path",
        )

    upgraded, summary = _infer_phase_upgrade(workflow)
    status = "updated" if apply else "would_update"
    if running_runs and not apply:
        status = "would_refuse_running"
        summary["running_runs"] = running_runs
    summary.update({
        "workflow_path": str(workflow_path),
        "status": status,
        "mode": "add_phases",
    })
    if not apply:
        summary["hint"] = "rerun with --apply to write the rewritten workflow"
        return summary

    workflow_path.write_text(
        json.dumps(upgraded, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )
    return summary


def _infer_phase_upgrade(workflow: JsonObject) -> tuple[JsonObject, JsonObject]:
    jobs_raw = workflow.get("jobs")
    edges_raw = workflow.get("edges")
    if not isinstance(jobs_raw, list):
        raise WorkflowError("workflow jobs must be a list", field_path="jobs")
    if not isinstance(edges_raw, list):
        raise WorkflowError("workflow edges must be a list", field_path="edges")

    jobs: list[JsonObject] = []
    for index, item in enumerate(jobs_raw):
        if not isinstance(item, dict):
            raise WorkflowError("each job must be an object", field_path=f"jobs[{index}]")
        jobs.append(dict(item))

    phase_for_job: dict[str, str] = {}
    phase_key_to_id: dict[str, str] = {}
    phase_order: list[str] = []
    jobs_relabelled: list[JsonObject] = []
    for job in jobs:
        job_id = str(job.get("id") or "")
        if not job_id:
            continue
        phase_key = _phase_key_for_job(job)
        if phase_key not in phase_key_to_id:
            phase_id = _unique_phase_id(phase_key, set(phase_key_to_id.values()))
            phase_key_to_id[phase_key] = phase_id
            phase_order.append(phase_id)
        phase_id = phase_key_to_id[phase_key]
        phase_for_job[job_id] = phase_id
        if job.get("phase_id") != phase_id:
            jobs_relabelled.append({"job_id": job_id, "phase_id": phase_id})
        job["phase_id"] = phase_id

    phases = [
        {"id": phase_id, "name": _phase_name_from_id(phase_id)}
        for phase_id in phase_order
    ]
    job_by_id = {str(job.get("id")): job for job in jobs if isinstance(job.get("id"), str)}
    incoming_by_job = _incoming_by_job(edges_raw)
    ordinary_jobs_by_phase: dict[str, list[str]] = {phase_id: [] for phase_id in phase_order}
    synthesis_by_phase: dict[str, str] = {}
    jobs_inserted: list[JsonObject] = []

    for job in jobs:
        job_id = str(job.get("id") or "")
        maybe_phase_id = phase_for_job.get(job_id)
        if maybe_phase_id is None:
            continue
        phase_id = maybe_phase_id
        if _is_synthesis_like(job):
            synthesis_by_phase.setdefault(phase_id, job_id)
        else:
            ordinary_jobs_by_phase.setdefault(phase_id, []).append(job_id)

    for phase_id in phase_order:
        candidate = synthesis_by_phase.get(phase_id)
        ordinary = ordinary_jobs_by_phase.get(phase_id, [])
        if candidate is not None and not all(job_id in incoming_by_job.get(candidate, set()) for job_id in ordinary):
            candidate = None
        if candidate is None:
            candidate = _unique_job_id(f"{phase_id}__synthesis", job_by_id)
            synthesis_job = _new_phase_synthesis_job(
                workflow=workflow,
                phase_id=phase_id,
                job_id=candidate,
            )
            jobs.append(synthesis_job)
            job_by_id[candidate] = synthesis_job
            phase_for_job[candidate] = phase_id
            synthesis_by_phase[phase_id] = candidate
            jobs_inserted.append({"job_id": candidate, "type": "phase_synthesis"})
        else:
            job_by_id[candidate]["type"] = "phase_synthesis"
            synthesis_by_phase[phase_id] = candidate
            phase_for_job[candidate] = phase_id
            job_by_id[candidate]["phase_id"] = phase_id

    new_edges: list[JsonObject] = []
    edges_rewritten: list[JsonObject] = []
    seen_edges: set[tuple[str, str]] = set()
    for edge in edges_raw:
        if not isinstance(edge, dict):
            continue
        from_id = str(edge.get("from") or "")
        to_id = str(edge.get("to") or "")
        from_phase = phase_for_job.get(from_id)
        to_phase = phase_for_job.get(to_id)
        if from_phase is not None and to_phase is not None and from_phase != to_phase:
            rewritten_from = synthesis_by_phase[from_phase]
            edges_rewritten.append({
                "from": rewritten_from,
                "to": to_id,
                "rewritten_from": from_id,
            })
            from_id = rewritten_from
        _append_edge(new_edges, seen_edges, from_id, to_id)

    for phase_id in phase_order:
        synthesis_id = synthesis_by_phase[phase_id]
        for job_id in ordinary_jobs_by_phase.get(phase_id, []):
            _append_edge(new_edges, seen_edges, job_id, synthesis_id)

    for index, phase_id in enumerate(phase_order[:-1]):
        next_phase_id = phase_order[index + 1]
        synthesis_id = synthesis_by_phase[phase_id]
        for job_id in _phase_entry_jobs(
            next_phase_id,
            ordinary_jobs_by_phase=ordinary_jobs_by_phase,
            phase_for_job=phase_for_job,
            edges=new_edges,
        ):
            _append_edge(new_edges, seen_edges, synthesis_id, job_id)

    for phase in phases:
        phase_id = str(phase["id"])
        phase["synthesis_job_id"] = synthesis_by_phase[phase_id]

    upgraded = dict(workflow)
    upgraded["schema_version"] = "striatum.workflow.v1.1"
    upgraded["phases"] = phases
    upgraded["jobs"] = jobs
    upgraded["edges"] = new_edges
    return upgraded, {
        "phases_added": phases,
        "jobs_relabelled": jobs_relabelled,
        "jobs_inserted": jobs_inserted,
        "edges_rewritten": edges_rewritten,
        "warnings": [],
    }


def _phase_key_for_job(job: JsonObject) -> str:
    group = job.get("parallel_group")
    if isinstance(group, str) and group.strip():
        return re.split(r"[_:-]", group.strip(), maxsplit=1)[0]
    job_id = str(job.get("id") or "main")
    return re.split(r"[_:-]", job_id, maxsplit=1)[0] or "main"


def _unique_phase_id(phase_key: str, existing: set[str]) -> str:
    base = _slug(phase_key)
    phase_id = base if base.startswith("phase_") else f"phase_{base}"
    candidate = phase_id
    suffix = 2
    while candidate in existing:
        candidate = f"{phase_id}_{suffix}"
        suffix += 1
    return candidate


def _phase_name_from_id(phase_id: str) -> str:
    name = phase_id
    if name.startswith("phase_"):
        name = name.removeprefix("phase_")
    name = re.sub(r"^\d+_", "", name)
    return name.replace("_", " ").strip().title() or phase_id


def _slug(value: str) -> str:
    slug = re.sub(r"[^a-zA-Z0-9]+", "_", value.strip().lower()).strip("_")
    return slug or "main"


def _incoming_by_job(edges: list[object]) -> dict[str, set[str]]:
    incoming: dict[str, set[str]] = {}
    for edge in edges:
        if not isinstance(edge, dict):
            continue
        from_id = edge.get("from")
        to_id = edge.get("to")
        if isinstance(from_id, str) and isinstance(to_id, str):
            incoming.setdefault(to_id, set()).add(from_id)
    return incoming


def _is_synthesis_like(job: JsonObject) -> bool:
    if job.get("type") == "phase_synthesis":
        return True
    text = " ".join(
        str(job.get(key) or "") for key in ("id", "type", "title")
    ).lower()
    return "synthesis" in text or "synthesize" in text or "consolidate" in text


def _new_phase_synthesis_job(
    *,
    workflow: JsonObject,
    phase_id: str,
    job_id: str,
) -> JsonObject:
    role_id = _first_matching_key(workflow.get("roles"), "review") or _first_key(workflow.get("roles")) or "reviewer"
    lane_id = _first_matching_key(workflow.get("lanes"), "review") or _first_key(workflow.get("lanes"))
    job: JsonObject = {
        "id": job_id,
        "type": "phase_synthesis",
        "title": f"{_phase_name_from_id(phase_id)} Synthesis",
        "role_id": role_id,
        "objective": "Synthesize the phase outputs and record a phase verdict.",
        "task_prompt": {"inline": "Review the phase outputs and publish the synthesis artifact."},
        "write_scope": {
            "mode": "repo_write",
            "repo_write": True,
            "allowed_paths": [f"striatum/{phase_id}/"],
            "forbidden_paths": [".striatum/"],
        },
        "expected_artifacts": [
            {
                "logical_name": "synthesis",
                "kind": "synthesis",
                "path": f"striatum/{phase_id}/SYNTHESIS.md",
                "required": True,
            }
        ],
        "phase_id": phase_id,
    }
    if lane_id is not None:
        job["lane_id"] = lane_id
    return job


def _first_matching_key(value: object, needle: str) -> str | None:
    if not isinstance(value, dict):
        return None
    for key in value:
        if needle in str(key).lower():
            return str(key)
    return None


def _first_key(value: object) -> str | None:
    if not isinstance(value, dict):
        return None
    for key in value:
        return str(key)
    return None


def _unique_job_id(base: str, jobs: dict[str, JsonObject]) -> str:
    candidate = base
    suffix = 2
    while candidate in jobs:
        candidate = f"{base}_{suffix}"
        suffix += 1
    return candidate


def _append_edge(
    edges: list[JsonObject],
    seen: set[tuple[str, str]],
    from_id: str,
    to_id: str,
) -> None:
    if not from_id or not to_id or from_id == to_id:
        return
    key = (from_id, to_id)
    if key in seen:
        return
    seen.add(key)
    edges.append({"from": from_id, "to": to_id, "on": "completed"})


def _phase_entry_jobs(
    phase_id: str,
    *,
    ordinary_jobs_by_phase: dict[str, list[str]],
    phase_for_job: dict[str, str],
    edges: list[JsonObject],
) -> list[str]:
    ordinary = ordinary_jobs_by_phase.get(phase_id, [])
    incoming_same_phase: set[str] = set()
    for edge in edges:
        from_id = edge.get("from")
        to_id = edge.get("to")
        if (
            isinstance(from_id, str)
            and isinstance(to_id, str)
            and phase_for_job.get(from_id) == phase_id
            and phase_for_job.get(to_id) == phase_id
        ):
            incoming_same_phase.add(to_id)
    entries = [job_id for job_id in ordinary if job_id not in incoming_same_phase]
    return entries or ordinary[:1]


def _running_runs_for_workflow(*, repo: Path, workflow_path: Path) -> list[str]:
    """Return non-terminal run ids referencing ``workflow_path`` in ``repo``."""
    candidates = _workflow_source_path_candidates(repo=repo, workflow_path=workflow_path)
    pg_running = _running_runs_for_workflow_pg(repo=repo, candidates=candidates)
    if pg_running is not None:
        return pg_running
    state_db = db_path(repo)
    if _repo_sqlite_cutover_marker_exists(state_db):
        raise WorkflowError(
            "workflow upgrade cannot verify non-terminal runs because this "
            "repository has been migrated off repo-local SQLite and daemon "
            "PostgreSQL was unavailable",
            field_path="path",
        )
    if not state_db.exists():
        return []
    if not _legacy_sqlite_workflow_upgrade_allowed():
        raise WorkflowError(
            "workflow upgrade cannot verify non-terminal runs from repo-local "
            "SQLite outside the paired test-harness compatibility escape; "
            "configure daemon PostgreSQL and rerun the upgrade",
            field_path="path",
        )
    return _running_runs_for_workflow_sqlite(state_db=state_db, candidates=candidates)


def _workflow_source_path_candidates(*, repo: Path, workflow_path: Path) -> set[str]:
    target = str(workflow_path.resolve())
    try:
        rel = workflow_path.resolve().relative_to(repo.resolve())
    except ValueError:
        return {target}
    candidates = {target}
    target_rel = rel.as_posix()
    if target_rel:
        candidates.add(target_rel)
    return candidates


def _running_runs_for_workflow_pg(*, repo: Path, candidates: set[str]) -> list[str] | None:
    """Return PG-backed running runs, or ``None`` when PG is not configured."""
    try:
        from striatum.daemon_pg.config import resolve_config
        from striatum.daemon_pg.connection import connect
    except ImportError:
        return None

    try:
        cfg = resolve_config()
    except Exception:  # noqa: BLE001 - local authoring can run without daemon PG config.
        return None
    if cfg.url is None:
        return None

    conn = None
    try:
        conn = connect(cfg.url)
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT repository_id
                FROM striatumd.repositories
                WHERE repo_root = %s AND state = 'active'
                """,
                (str(repo.resolve()),),
            )
            row = cur.fetchone()
            if row is None:
                return None
            repository_id = str(row[0])
            cur.execute(
                """
                SELECT r.run_id
                FROM striatumd.runs r
                JOIN striatumd.workflow_snapshots w
                  ON w.repository_id = r.repository_id
                 AND w.workflow_snapshot_id = r.workflow_snapshot_id
                WHERE r.repository_id = %s
                  AND r.state NOT IN ('completed', 'failed', 'canceled')
                  AND w.source_path = ANY(%s)
                ORDER BY r.run_id
                """,
                (repository_id, sorted(candidates)),
            )
            return [str(row[0]) for row in cur.fetchall()]
    except Exception:  # noqa: BLE001 - fall back to legacy guard before cutover.
        return None
    finally:
        if conn is not None:
            conn.close()


def _repo_sqlite_cutover_marker_exists(state_db: Path) -> bool:
    return state_db.with_name(state_db.name + ".tombstone").exists() or state_db.with_name(
        state_db.name + ".migrated"
    ).exists()


def _legacy_sqlite_workflow_upgrade_allowed() -> bool:
    return os.environ.get("STRIATUM_TEST_HARNESS") == "1" and os.environ.get("STRIATUM_DAEMON_REQUIRED") == "0"


def _running_runs_for_workflow_sqlite(*, state_db: Path, candidates: set[str]) -> list[str]:
    try:
        conn = sqlite3.connect(state_db)
    except sqlite3.Error:
        return []
    try:
        conn.row_factory = sqlite3.Row
        cur = conn.execute(
            """
            SELECT runs.run_id, runs.state, workflow_snapshots.source_path
            FROM runs
            JOIN workflow_snapshots
              ON runs.workflow_snapshot_id = workflow_snapshots.workflow_snapshot_id
            WHERE runs.state NOT IN ('completed', 'failed', 'canceled')
            """
        )
        running: list[str] = []
        for row in cur.fetchall():
            source = str(row["source_path"] or "")
            if source in candidates:
                running.append(str(row["run_id"]))
        return running
    except sqlite3.Error:
        return []
    finally:
        conn.close()


__all__ = ["workflow_upgrade"]

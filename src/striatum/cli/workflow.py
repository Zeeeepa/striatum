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
import sqlite3
from pathlib import Path
from typing import Any

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
    updated_profiles: dict[str, JsonObject] = {}
    for profile_id, body in profiles.items():
        if not isinstance(body, dict):
            updated_profiles[profile_id] = body  # type: ignore[assignment]
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


def _running_runs_for_workflow(*, repo: Path, workflow_path: Path) -> list[str]:
    """Return non-terminal run ids referencing ``workflow_path`` in ``repo``."""
    state_db = db_path(repo)
    if not state_db.exists():
        return []
    target = str(workflow_path.resolve())
    target_rel: str | None = None
    try:
        rel = workflow_path.resolve().relative_to(repo.resolve())
    except ValueError:
        rel = None
    if rel is not None:
        target_rel = str(rel)
    candidates = {target}
    if target_rel:
        candidates.add(target_rel)
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

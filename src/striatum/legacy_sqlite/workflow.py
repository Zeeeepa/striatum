"""Legacy repo-local SQLite workflow live-state helpers."""

from __future__ import annotations

from pathlib import Path
from typing import TYPE_CHECKING, Any, cast

from striatum.primitives import JsonObject, json_dumps, new_id, sha256_bytes, utc_now
from striatum.workflow import (
    VERDICT_JOB_TYPES,
    _effective_fresh_session_required,
    _list,
    _object,
    _string,
    edge_dependency_pairs,
    load_workflow,
    workflow_job_map,
)

if TYPE_CHECKING:
    import sqlite3


def compute_node_states(
    conn: sqlite3.Connection, *, run_id: str
) -> dict[str, str]:
    """Return ``{workflow_job_id: current_state}`` for the highest attempt."""
    rows = conn.execute(
        """
        SELECT workflow_job_id, state, attempt
        FROM jobs
        WHERE run_id = ?
        ORDER BY workflow_job_id, attempt DESC
        """,
        (run_id,),
    ).fetchall()
    seen: set[str] = set()
    result: dict[str, str] = {}
    for row in rows:
        wf_id = str(row["workflow_job_id"])
        if wf_id in seen:
            continue
        seen.add(wf_id)
        result[wf_id] = str(row["state"])
    return result


def create_run(conn: sqlite3.Connection, *, repo: Path, workflow_path: Path) -> JsonObject:
    """Snapshot workflow JSON and create a prepared run."""
    from striatum.legacy_sqlite.db import insert_event

    workflow = load_workflow(workflow_path)
    now = utc_now()
    raw_json = json_dumps(workflow)
    workflow_snapshot_id = new_id("wfs")
    run_id = new_id("run")
    conn.execute(
        """
        INSERT INTO workflow_snapshots (
          workflow_snapshot_id, workflow_id, workflow_version, source_path,
          content_sha256, workflow_json, loaded_at
        )
        VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
        (
            workflow_snapshot_id,
            workflow["workflow_id"],
            workflow.get("workflow_version"),
            str(workflow_path),
            sha256_bytes(raw_json.encode("utf-8")),
            raw_json,
            now,
        ),
    )
    conn.execute(
        """
        INSERT INTO runs (
          run_id, workflow_snapshot_id, repo_root, state, branch_name,
          branch_base, created_at
        )
        VALUES (?, ?, ?, 'needs_branch_confirmation', ?, ?, ?)
        """,
        (
            run_id,
            workflow_snapshot_id,
            str(repo),
            _object(workflow, "branch").get("suggested_name"),
            None,
            now,
        ),
    )
    workflow_jobs = workflow_job_map(workflow)
    job_map: dict[str, str] = {}
    for job_value in _list(workflow, "jobs"):
        job = cast(dict[str, object], job_value)
        workflow_job_id = _string(job, "id")
        job_id = f"job_{run_id}_{workflow_job_id}"
        job_map[workflow_job_id] = job_id
        lane_id = job.get("lane_id")
        stored_job_type = "review" if job.get("type") == "phase_synthesis" else job.get("type", "generic")
        conn.execute(
            """
            INSERT INTO jobs (
              job_id, run_id, workflow_job_id, title, job_type, role_id,
              lane_selector_json, capability_requirements_json, state, max_attempts,
              fresh_session_required, write_scope_json, expected_artifacts_json,
              idempotency_key, created_at
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'blocked', ?, ?, ?, ?, ?, ?)
            """,
            (
                job_id,
                run_id,
                workflow_job_id,
                job.get("title", workflow_job_id),
                stored_job_type,
                job["role_id"],
                json_dumps({"lane_id": lane_id} if lane_id is not None else {}),
                json_dumps(
                    {
                        "objective": job.get("objective"),
                        "task_prompt": job.get("task_prompt", {}),
                        "inputs": job.get("inputs", []),
                    }
                ),
                int(cast(Any, job.get("max_attempts", 1))),
                1 if _effective_fresh_session_required(job) else 0,
                json_dumps(job.get("write_scope", {})),
                json_dumps(job.get("expected_artifacts", [])),
                f"{run_id}:{workflow_job_id}:1",
                now,
            ),
        )
    for upstream_id, downstream_id, gate in edge_dependency_pairs(workflow):
        upstream_job = workflow_jobs[upstream_id]
        gate_json = dict(gate)
        if upstream_job.get("type") in VERDICT_JOB_TYPES:
            gate_json["requires_verdict"] = ["accept", "accept_with_findings"]
        conn.execute(
            """
            INSERT OR IGNORE INTO job_dependencies(job_id, depends_on_job_id, gate_json)
            VALUES (?, ?, ?)
            """,
            (job_map[downstream_id], job_map[upstream_id], json_dumps(gate_json)),
        )
    insert_event(
        conn,
        run_id=run_id,
        event_type="run.created",
        payload={"workflow_id": workflow["workflow_id"], "workflow_snapshot_id": workflow_snapshot_id},
    )
    branch_section = _object(workflow, "branch")
    return {
        "run_id": run_id,
        "state": "needs_branch_confirmation",
        "branch_mode": branch_section.get("mode", "auto"),
        "suggested_branch_name": branch_section.get("suggested_name"),
    }

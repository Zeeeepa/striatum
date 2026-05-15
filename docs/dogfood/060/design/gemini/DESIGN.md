author: operator [self-declared: designer-gemini-1]

# Design: RFC 0048 Phase C read-surface PG handlers

## 1. Per-method specificity

This phase ports 12 legacy read methods to daemon-native Postgres handlers. Each method will be implemented in its own file under `src/striatum/daemon_pg/handlers/reads/` to match the Phase A pattern. All handlers will use the `@register_pg_handler("<method.name>", read_only=True)` decorator and the `def handle(ctx: RepoHandlerContext, params: dict) -> dict` signature.

### 1.1 `status`
- **Legacy source**: `src/striatum/cli/introspect.py:170-205` (`status`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/status.py`
- **Tables queried**: `striatumd.runs`, `striatumd.jobs`, `striatumd.blockers`, `striatumd.verdicts`, `striatumd.sessions`, `striatumd.workflow_snapshots`, `striatumd.process_supervisors`, `striatumd.leases`.
- **Return-shape parity contract**:
  ```json
  {
      "runs": [], "provenance_mode": "...", "sessions": [], "jobs": {},
      "open_blockers": [], "human_checkpoints": [],
      "latest_non_accepting_review_verdicts": [], "verdicts_by_posture": {},
      "claimable_jobs": [], "blocked_downstream_jobs": [],
      "process_health": {}, "next_actions": [], "phase_progress": null
  }
  ```
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_status.py`. Use the parity rig to compare against SQLite legacy output.
- **Error modes**: `repo_not_registered` via the router, `run_not_found` (returns empty lists if run doesn't exist, as per legacy behavior).

### 1.2 `dashboard`
- **Legacy source**: `src/striatum/dashboard.py:103-211` (`gather_payload`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/dashboard.py`
- **Tables queried**: Same as `status`, plus `striatumd.events` for recent events.
- **Return-shape parity contract**:
  ```json
  {
      "run": {}, "status": {}, "events": [], "verdict_counts": {},
      "posture_counts": {}, "updated_at": "...", "workflow": {},
      "node_states": {}, "override_verdict_counts": {}, "override_verdicts": []
  }
  ```
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_dashboard.py`. Use the parity rig.
- **Error modes**: `run_not_found` returns RPC error.

### 1.3 `list.runs`
- **Legacy source**: `src/striatum/cli/list_commands.py:93-119` (`list_runs`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/list_runs.py`
- **Tables queried**: `striatumd.runs`, `striatumd.workflow_snapshots`.
- **Return-shape parity contract**: `{"items": [], "count": N}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_list_runs.py`. Use the parity rig.
- **Error modes**: `invalid_state` (if provided).

### 1.4 `list.sessions`
- **Legacy source**: `src/striatum/cli/list_commands.py:122-168` (`list_sessions`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/list_sessions.py`
- **Tables queried**: `striatumd.sessions`, `striatumd.runs`.
- **Return-shape parity contract**: `{"items": [], "count": N}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_list_sessions.py`. Use the parity rig.
- **Error modes**: `run_not_found`, `invalid_state`.

### 1.5 `list.jobs`
- **Legacy source**: `src/striatum/cli/list_commands.py:171-223` (`list_jobs`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/list_jobs.py`
- **Tables queried**: `striatumd.jobs`, `striatumd.verdicts`, `striatumd.runs`.
- **Return-shape parity contract**: `{"items": [], "count": N}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_list_jobs.py`. Use the parity rig.
- **Error modes**: `run_not_found`, `invalid_state`.

### 1.6 `list.artifacts`
- **Legacy source**: `src/striatum/cli/list_commands.py:226-266` (`list_artifacts`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/list_artifacts.py`
- **Tables queried**: `striatumd.artifacts`, `striatumd.runs`, `striatumd.jobs` (via workflow).
- **Return-shape parity contract**: `{"items": [], "count": N}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_list_artifacts.py`. Use the parity rig.
- **Error modes**: `run_not_found`, `invalid_kind`.

### 1.7 `list.workflows`
- **Legacy source**: `src/striatum/cli/list_commands.py:269-286` (`list_workflows`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/list_workflows.py`
- **Tables queried**: `striatumd.workflow_snapshots`.
- **Return-shape parity contract**: `{"items": [], "count": N}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_list_workflows.py`. Use the parity rig.
- **Error modes**: standard routing errors.

### 1.8 `run.summary`
- **Legacy source**: `src/striatum/cli/run_summary.py:23-38` (`run_summary_export`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/run_summary_export.py`
- **Tables queried**: `striatumd.runs`, `striatumd.workflow_snapshots`, `striatumd.artifacts`, `striatumd.sessions`, `striatumd.verdicts`, `striatumd.jobs`, `striatumd.blockers`.
- **Return-shape parity contract**: `{"status": "exported", "run_id": "...", "path": "...", "sha256": "..."}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_run_summary.py`. Use the parity rig.
- **Error modes**: `run_not_found`. Note: Appends `run_summary.exported` event (handler not strictly read-only, remove `read_only=True` if writing events).

### 1.9 `why`
- **Legacy source**: `src/striatum/cli/introspect.py:564-633` (`why`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/why.py`
- **Tables queried**: `striatumd.runs`, `striatumd.jobs`, `striatumd.queue_messages`, `striatumd.blockers`, `striatumd.verdicts`, `striatumd.artifacts`, `striatumd.events`.
- **Return-shape parity contract**:
  ```json
  {
      "target_type": "...",
      "...": {},
      "events": [],
      "next_actions": []
  }
  ```
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_why.py`. Use the parity rig.
- **Error modes**: `target_not_found`.

### 1.10 `doctor`
- **Legacy source**: `src/striatum/cli/introspect.py:1204-1600` (`doctor`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/doctor.py`
- **Tables queried**: Extensively scans `runs`, `jobs`, `leases`, `work_packets`, `queue_messages`, `blockers`, `sessions`, `job_worktrees`, `job_dependencies`, `artifacts`.
- **Return-shape parity contract**: `{"ok": bool, "problems": [], "problem_records": []}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_doctor.py`. Use the parity rig.
- **Error modes**: `run_not_found`.

### 1.11 `evidence.export`
- **Legacy source**: `src/striatum/cli/evidence.py:356-383` (`evidence_export`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/evidence_export.py`
- **Note**: This was technically ported in Phase A (`src/striatum/daemon_pg/handlers/recovery_evidence/evidence_export.py`). We will move it to `reads/` for consistency, or keep it in `recovery_evidence/` and just ensure it is properly exposed. (Synthesis lock: move to `reads/` to consolidate read verbs).
- **Return-shape parity contract**: `{"status": "exported", "run_id": "...", "path": "...", "sha256": "..."}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_evidence_export.py`.

### 1.12 `corpus.export`
- **Legacy source**: `src/striatum/corpus/export.py:16-48` (`export_corpus_bundle`)
- **New PG handler path**: `src/striatum/daemon_pg/handlers/reads/corpus_export.py`
- **Tables queried**: Enumerates via `striatumd.events`, `striatumd.artifacts`, `striatumd.decisions`, etc.
- **Return-shape parity contract**: `{"status": "exported", "bundle_sha256": "...", ...}`
- **Test path + parity strategy**: `tests/daemon_pg/handlers/reads/test_corpus_export.py`. Use the parity rig.
- **Error modes**: `invalid_out_path`. Appends `corpus.exported` event.

## 2. Cross-cutting decisions

- **Module layout**: Single file per method under `src/striatum/daemon_pg/handlers/reads/`. This matches the Phase A pattern and isolates the large footprint of `doctor` and `status`. We will move `evidence_export` from `recovery_evidence/` to `reads/` to complete the grouping.
- **repository_id scoping mechanism**: Explicit `WHERE repository_id = %(repository_id)s` discipline in every SQL statement, utilizing the helpers in `_sql.py`. This is the safest approach and aligns with existing code.
- **Parity test strategy**: **Wire the parity rig**. The unused `Seed` and `pg_ctx` / `sqlite_conn` rig from `tests/daemon_pg/handlers/recovery_evidence/conftest.py` will be moved/shared to `tests/daemon_pg/handlers/conftest.py` so both `reads/` and `recovery_evidence/` can use it. We will assert per-key diffs between PG and SQLite paths for all read handlers.
- **Single implement track**: A single track will handle all 12 methods to avoid boundary conflicts. Sub-agents may be used if parallel implementation is required.

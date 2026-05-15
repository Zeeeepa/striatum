---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/060/design/codex/DESIGN.md", "docs/dogfood/060/design/claude_code/DESIGN.md"]
---
author: designer-unknown-model-001

# RFC 0048 Phase C Read-Surface Scope Lock

Dogfood-060 ports the daemon-RPC read surface to native Postgres handlers in one implementation track. The dropped Gemini design slot is not an input for this synthesis; the Codex and Claude designs agree on the implementation shape, and Claude's exact method inventory is the lock: 8 CLI read surfaces implemented as 12 registered RPC methods (`status`, `dashboard`, `list.*`, `run.summary`, `why`, `doctor`, `evidence.export`, `corpus.export`).

## Cross-Cutting Lock

- **Module layout:** one file per RPC method under `src/striatum/daemon_pg/handlers/reads/`, plus `reads/_sql.py` and `reads/_read_model.py`. This matches Track A and Track B's grep-able per-method layout while keeping the shared status/doctor/evidence projections in one owner-owned module.
- **Repository scoping:** every SQL statement that touches a workflow table must include an explicit `WHERE <alias>.repository_id = %(repository_id)s` predicate, and every join must include `joined.repository_id = base.repository_id`. `reads/_sql.py` may inject `repository_id` into params, but it must not hide the predicate behind a wrapper because reviewers need grep-visible scope discipline.
- **Registration:** use the existing `register_pg_handler` mechanism from `striatum.daemon_pg.handlers.registry`; extend it only to accept optional `read_only: bool = True` metadata if the implementer wants that test hook. No new registry or router mechanism.
- **Handler integration:** add one line in `src/striatum/daemon_pg/handlers/__init__.py`: `from . import reads as reads`. Add `reads/__init__.py` that imports each method file for decorator side effects.
- **CLI route prerequisites:** fix `src/striatum/cli/daemon_rpc_route.py::_route_list` to pass existing list filters (`run_id`, `state`, `role`, `lane`, `workflow_job_id`, `kind`, `limit`), and add a `corpus.export` translator. Without those, correct PG handlers cannot receive the legacy contract params.
- **Parity rig:** wire `tests/daemon_pg/handlers/recovery_evidence/conftest.py`'s deferred idea into a new `tests/daemon_pg/handlers/reads/conftest.py`, not by editing the Track B backlog in place. The reads rig must seed equivalent SQLite and PG rows, run the legacy function and PG handler, and assert per-key diffs after normalizing timestamp formatting and generated export paths. Shape-only smoke is rejected because `status`, `doctor`, and evidence shapes have changed recently enough that smoke would miss real regressions.
- **Single implementation track:** locked. Native sub-agents may split local work by cluster inside the one implementer session, but Striatum gets one owner for `reads/_read_model.py`, exports, list filters, and tests.
- **Read-surface write discipline:** no handler mutates `striatumd.*` workflow state in this dogfood. `run.summary`, `evidence.export`, and `corpus.export` still write requested repo files/directories for return-shape parity, but their PG handler contract does not append `run_summary.exported`, `evidence.exported`, or new corpus events in Phase C. This follows the packet's empty write-set constraint.

## Shared Files

| File | Purpose |
|---|---|
| `src/striatum/daemon_pg/handlers/reads/_sql.py` | Tiny helpers for repo-scoped `fetch_all`, `fetch_one`, JSON/timestamp normalization, and reuse of Track B `expire_leases` where legacy reads expire leases before returning. |
| `src/striatum/daemon_pg/handlers/reads/_read_model.py` | PG projections for `status_pg`, `doctor_pg`, `evidence_snapshot_pg`, artifact/session/job summaries, `why` helpers, and node-state inputs. |
| `tests/daemon_pg/handlers/reads/conftest.py` | `ReadSeed`, `pg_ctx`, `sqlite_conn`, `parity_seed`, `assert_payload_parity`, and registration/fail-closed helpers. |

## Method Locks

### `status`

| Field | Lock |
|---|---|
| Legacy source | `src/striatum/cli/introspect.py:170-225` `status(conn, *, run_id)` |
| New handler | `src/striatum/daemon_pg/handlers/reads/status.py` |
| Decorator + signature | `@register_pg_handler("status", read_only=True)`; `def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]` |
| Tables queried | `striatumd.runs(run_id,state,branch_name,created_at,workflow_snapshot_id,paused_at,stop_reason)`, `workflow_snapshots(workflow_snapshot_id,workflow_json)`, `jobs(job_id,run_id,workflow_job_id,state,role_id,lane_selector_json,current_lease_id,current_message_id,expected_artifacts_json,attempt,max_attempts,job_type,created_at)`, `sessions(session_id,run_id,role_id,lane_id,slug,ordinal,state,operator_label,registered_at)`, `queue_messages(message_id,run_id,job_id,state,target_role_id,target_lane_id,current_lease_id,visible_after)`, `leases(lease_id,run_id,resource_id,owner_session_id,state,expires_at)`, `blockers(blocker_id,run_id,job_id,session_id,severity,blocker_kind,description,state,payload_json,created_at)`, `verdicts(verdict_id,run_id,job_id,verdict,posture,created_at)`, `job_dependencies(job_id,depends_on_job_id,gate_json)`, `artifacts(artifact_id,run_id,job_id,logical_name,artifact_kind,repo_path,content_sha256,author_line)`, `process_executions(process_id,run_id,job_id,session_id,lease_id,state,pid)`, `process_supervisors(supervisor_id,run_id,session_id,state,pid,command_json,pid_start_time)`, `process_supervisor_pointers(supervisor_id,run_id,session_id,state,pid,pid_start_time,metadata_json)`, `job_worktrees(worktree_id,run_id,job_id,lease_id,state,worktree_path)` |
| Return keys | `runs`, `provenance_mode`, `sessions`, `jobs`, `open_blockers`, `human_checkpoints`, `latest_non_accepting_review_verdicts`, `verdicts_by_posture`, `claimable_jobs`, `blocked_downstream_jobs`, `process_health`, `next_actions`; when phase metadata exists also `phases`, `current_phase_id` |
| Test | `tests/daemon_pg/handlers/reads/test_status.py`; per-key parity against `striatum.cli.introspect.status()` with claimable, blocked, verdict, stale-lease, process, and phased-run rows |
| Errors | router-level `repo_not_registered`; malformed params -> `schema_invalid`; unknown `run_id` preserves legacy empty/phase-less status shape |

### `dashboard`

| Field | Lock |
|---|---|
| Legacy source | `src/striatum/dashboard.py:84-211` `gather_payload(repo, *, run_id)` |
| New handler | `src/striatum/daemon_pg/handlers/reads/dashboard.py` |
| Decorator + signature | `@register_pg_handler("dashboard", read_only=True)`; same `(ctx, params)` signature |
| Tables queried | status read set, plus `events(event_id,run_id,event_type,payload_json,created_at,actor_session_id,job_id,message_id,artifact_id,lease_id)`, `verdicts(verdict_id,run_id,verdict,posture,rationale,created_at)`, `blockers(blocker_id,run_id,created_at,payload_json,state)`, `workflow_snapshots(workflow_snapshot_id,workflow_json)` |
| Return keys | `run`, `status`, `events`, `verdict_counts`, `posture_counts`, `updated_at`, `workflow`, `node_states`, `override_verdict_counts`, `override_verdicts` |
| Test | `tests/daemon_pg/handlers/reads/test_dashboard.py`; per-key parity against `gather_payload()`, ignoring only `updated_at`; also render through `render_frame()` for a non-empty frame smoke |
| Errors | missing/malformed `run_id` -> `schema_invalid`; unknown run -> `not_found` with `unknown run_id ...` parity |

### `list.*`

All five handlers return the legacy `_envelope(items)` shape: top-level keys `items`, `count`.

| Method | Legacy source | New handler + decorator | Tables queried | Test + parity | Errors |
|---|---|---|---|---|---|
| `list.runs` | `src/striatum/cli/list_commands.py:93-119` `list_runs` | `reads/list_runs.py`; `@register_pg_handler("list.runs", read_only=True)` | `runs(run_id,state,branch_name,created_at,started_at,completed_at,workflow_snapshot_id)`, `workflow_snapshots(workflow_snapshot_id,workflow_id)` | `tests/daemon_pg/handlers/reads/test_list_runs.py`; default limit, explicit limit, state filter parity | invalid state -> `schema_invalid`; no rows -> `{"items": [], "count": 0}` |
| `list.sessions` | `src/striatum/cli/list_commands.py:122-168` `list_sessions` | `reads/list_sessions.py`; `@register_pg_handler("list.sessions", read_only=True)` | `runs(run_id)`, `sessions(session_id,run_id,role_id,lane_id,slug,ordinal,state,capabilities_json,registered_at,last_heartbeat_at,operator_label)`, `process_supervisors(supervisor_id,run_id,session_id,state,pid,pid_start_time)`, `process_supervisor_pointers(supervisor_id,run_id,session_id,state,pid,pid_start_time,metadata_json)` | `test_list_sessions.py`; state/role/lane filters, capabilities parsing, lane-attestation parity | missing run_id -> `schema_invalid`; unknown run -> `not_found`; invalid state -> `schema_invalid` |
| `list.jobs` | `src/striatum/cli/list_commands.py:171-223` `list_jobs` | `reads/list_jobs.py`; `@register_pg_handler("list.jobs", read_only=True)` | `runs(run_id)`, `jobs(job_id,run_id,workflow_job_id,state,role_id,lane_selector_json,attempt,max_attempts,job_type,created_at,started_at,completed_at)`, latest `verdicts(job_id,verdict,created_at,verdict_id)`, `leases(lease_id,run_id,resource_id,state,expires_at)` for lazy expiry | `test_list_jobs.py`; state/workflow-job filters and latest-verdict ordering | missing run_id -> `schema_invalid`; unknown run -> `not_found`; invalid state -> `schema_invalid` |
| `list.artifacts` | `src/striatum/cli/list_commands.py:226-266` `list_artifacts` | `reads/list_artifacts.py`; `@register_pg_handler("list.artifacts", read_only=True)` | `runs(run_id,workflow_snapshot_id)`, `workflow_snapshots(workflow_snapshot_id,workflow_json)`, `artifacts(artifact_id,run_id,job_id,session_id,logical_name,artifact_kind,repo_path,content_sha256,size_bytes,created_at,author_line)`, `jobs(job_id,workflow_job_id,role_id,lane_selector_json,expected_artifacts_json)`, `sessions(session_id,role_id,lane_id,ordinal,operator_label)` | `test_list_artifacts.py`; kind filter and structured `author` parity | missing run_id -> `schema_invalid`; unknown run -> `not_found`; invalid kind -> `schema_invalid` |
| `list.workflows` | `src/striatum/cli/list_commands.py:269-288` `list_workflows` | `reads/list_workflows.py`; `@register_pg_handler("list.workflows", read_only=True)` | `workflow_snapshots(workflow_snapshot_id,workflow_id,workflow_version,source_path,content_sha256,loaded_at)` | `test_list_workflows.py`; default and explicit limit parity | malformed limit -> `schema_invalid`; no rows -> empty envelope |

### `run.summary`

| Field | Lock |
|---|---|
| Legacy source | `src/striatum/cli/run_summary.py:23-38` `run_summary_export`; `src/striatum/cli/run_summary.py:41-110` `run_summary_snapshot` |
| New handler | `src/striatum/daemon_pg/handlers/reads/run_summary.py` |
| Decorator + signature | `@register_pg_handler("run.summary", read_only=True)`; same `(ctx, params)` signature |
| Tables queried | status and doctor read sets, plus `runs(run_id,workflow_snapshot_id,branch_name,state,created_at,started_at,completed_at)`, `workflow_snapshots(workflow_snapshot_id,workflow_json)`, `artifacts(artifact_id,job_id,session_id,logical_name,artifact_kind,repo_path,content_sha256,size_bytes,created_at,author_line)`, `sessions(session_id,role_id,lane_id,ordinal,state,closed_at,close_reason,operator_label)`, `verdicts(verdict_id,job_id,run_id,findings_artifact_id,verdict,posture,created_at)`, `blockers(blocker_id,job_id,severity,blocker_kind,state,created_at)` |
| Return keys | `status`, `run_id`, `path`, `sha256` |
| Test | `tests/daemon_pg/handlers/reads/test_run_summary.py`; compare rendered Markdown bytes/digest and response envelope against `run_summary_export()` with normalized current-branch context |
| Errors | missing `run_id`/`path` -> `schema_invalid`; unknown run -> `not_found`; unsafe path -> `path_outside_scope`/legacy exit-code-8 equivalent |

### `why`

| Field | Lock |
|---|---|
| Legacy source | `src/striatum/cli/introspect.py:564-681` `why(conn, *, target_id)` |
| New handler | `src/striatum/daemon_pg/handlers/reads/why.py` |
| Decorator + signature | `@register_pg_handler("why", read_only=True)`; same `(ctx, params)` signature |
| Tables queried | primary target lookup in order: `runs(*)`, `jobs(*)` by `job_id` or `workflow_job_id`, `queue_messages(*)`, `blockers(*)`, `artifacts(*)`, `verdicts(*)`, `sessions(*)` by `session_id` or `slug`, `process_executions(*)`; helper reads from `events`, `job_dependencies`, `blockers`, `verdicts`, `jobs`, `sessions`, and status read set as needed |
| Return keys | Run: `target_type`, `run`, `jobs`, `open_blockers`, `events`, `next_actions`. Job/message: `target_type`, `job`, `message`, `verdict`, `blockers`, `downstream_jobs`, `events`. Blocker: `target_type`, `blocker`, `run`, `job`, `session`, `related_verdict`, `blocked_downstream_jobs`, `human_checkpoint`, `next_actions`, `events`. Artifact: `target_type`, `artifact`, `job`, `verdicts`, `events`. Verdict: `target_type`, `verdict`, `job`, `artifact`, `blockers`, `events`. Session: `target_type`, `session`, `jobs`, `events`. Process: `target_type`, `process`, `job`, `session`, `events` |
| Test | `tests/daemon_pg/handlers/reads/test_why.py`; one branch test per target type with exact top-level key parity and value parity |
| Errors | missing `target_id` -> `schema_invalid`; unknown target -> `not_found` with the legacy message from lines 679-681 |

### `doctor`

| Field | Lock |
|---|---|
| Legacy source | `src/striatum/cli/introspect.py:1204-1808` `doctor(conn, *, repo, run_id, verbose)` |
| New handler | `src/striatum/daemon_pg/handlers/reads/doctor.py` |
| Decorator + signature | `@register_pg_handler("doctor", read_only=True)`; same `(ctx, params)` signature |
| Tables queried | `runs(run_id,state,workflow_snapshot_id)`, `workflow_snapshots(workflow_snapshot_id,workflow_json)`, `jobs(job_id,run_id,workflow_job_id,state,current_message_id,current_lease_id,expected_artifacts_json)`, `job_dependencies(job_id,depends_on_job_id,gate_json)`, `queue_messages(message_id,run_id,job_id,state,current_lease_id)`, `leases(lease_id,run_id,resource_id,owner_session_id,state,expires_at)`, `sessions(session_id,run_id,state)`, `blockers(blocker_id,run_id,job_id,severity,blocker_kind,state)`, `artifacts(artifact_id,job_id,logical_name,artifact_kind,repo_path)`, `verdicts(verdict_id,job_id,verdict,posture,created_at)`, `work_packets(packet_id,run_id,lease_id,session_id)`, `job_worktrees(worktree_id,run_id,lease_id,worktree_path,state)`, `process_supervisors(supervisor_id,run_id,session_id,state,pid,stdin_pipe_path,ended_at,stop_reason)`, `process_executions(process_id,run_id,job_id,session_id,lease_id,state,pid,started_at)`, plus repo filesystem checks for worktree paths, editable install, skills, and plugins |
| Return keys | always `ok`, `schema_version`, `problems`; when `verbose` is true also `problem_records` |
| Test | `tests/daemon_pg/handlers/reads/test_doctor.py`; per-check parity for every stable `DOCTOR_CHECKS` name and an all-clean baseline |
| Errors | empty repo -> clean legacy shape; unknown `run_id` -> `not_found` so scoped doctor does not silently inspect nothing |

### `evidence.export`

| Field | Lock |
|---|---|
| Legacy source | `src/striatum/cli/evidence.py:356-383` `evidence_export`; `src/striatum/cli/evidence.py:386-426` `evidence_snapshot` |
| New handler | `src/striatum/daemon_pg/handlers/reads/evidence_export.py`; remove the old `recovery_evidence/evidence_export.py` registration/import in the same implementation |
| Decorator + signature | `@register_pg_handler("evidence.export", read_only=True)`; same `(ctx, params)` signature |
| Tables queried | status and doctor read sets, plus `runs(run_id,workflow_snapshot_id,branch_name,state)`, `workflow_snapshots(workflow_snapshot_id,workflow_id,workflow_version,workflow_json)`, `jobs(job_id,workflow_job_id,role_id,lane_selector_json,state,expected_artifacts_json)`, `artifacts(artifact_id,run_id,job_id,session_id,logical_name,artifact_kind,repo_path,content_sha256,size_bytes,created_at,author_line)`, `sessions(session_id,run_id,role_id,lane_id,slug,ordinal,state,closed_at,close_reason,operator_label)`, `verdicts(verdict_id,job_id,session_id,verdict,findings_artifact_id,rationale,posture,created_at)`, `blockers(blocker_id,job_id,session_id,severity,blocker_kind,description,state,created_at)` |
| Return keys | `status`, `run_id`, `path`, `sha256` |
| Test | `tests/daemon_pg/handlers/reads/test_evidence_export.py`; compare redacted payloads, rendered Markdown bytes/digest, and response envelope against `evidence_export()`; import legacy redactor/renderer, do not fork policy |
| Errors | missing `run_id`/`path` -> `schema_invalid`; unknown run -> `not_found`; unsafe path -> legacy path error envelope |

### `corpus.export`

| Field | Lock |
|---|---|
| Legacy source | `src/striatum/corpus/export.py:16-48` `export_corpus_bundle(conn, *, repo, since, out_text)` |
| New handler | `src/striatum/daemon_pg/handlers/reads/corpus_export.py` |
| Decorator + signature | `@register_pg_handler("corpus.export", read_only=True)`; same `(ctx, params)` signature |
| Tables queried | `events(event_id,run_id,event_type,payload_json,created_at)` for audit-chain corpus rows; status/doctor/run-summary read sets for run-summary corpus rows; filesystem/git reads for RFCs, decision log, operator reports, committed run summaries, changelog, ubiquitous language, friction patterns, and commits |
| Return keys | `status`, `since`, `out`, `manifest_path`, `row_counts`, `bundle_sha256` |
| Test | `tests/daemon_pg/handlers/reads/test_corpus_export.py`; export SQLite and PG bundles to separate temp dirs and compare manifest keys, JSONL row counts, and bundle hash after normalizing generated timestamps/substrate version |
| Errors | missing `since`/`out` -> `schema_invalid`; unsafe output path or `.striatum` target -> path error; unknown git ref -> RPC error preserving legacy message |

## Acceptance

Implementation is done when `src/striatum/daemon_pg/handlers/reads/` registers every method above, `resolve_pg_handler()` wins before `CLI_ROUTES` fallback for each method, list/corpus CLI translators pass their params, and `tests/daemon_pg/handlers/reads/` gives per-key parity with a reachable PG URL. Without PG, registration/signature/error-mode smoke tests may skip parity with the existing `tests/_harness/pg.py` reachability gate.

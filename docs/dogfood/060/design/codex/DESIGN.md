author: designer-unknown-model-001

# RFC 0048 Phase C Read-Surface PG Handler Design

## Summary

Phase C should add one native PostgreSQL handler package:

- `src/striatum/daemon_pg/handlers/reads/`
- `tests/daemon_pg/handlers/reads/`

Every module registers with:

```python
@register_pg_handler("<method.name>", read_only=True)
def handle(ctx: RepoHandlerContext, params: dict) -> dict:
    ...
```

The implementation track should be single-track. Parallelism should happen
inside that track with two sub-agent clusters:

- Core reads: `status`, `why`, `doctor`, `dashboard`.
- Reporting reads: `list.*`, `run.summary`, `evidence.export`,
  `corpus.export`.

All SQL must include `repository_id = %s` with `ctx.repository_id`; no helper
may query a workflow table without an explicit repository scope. The PG
handler return value must preserve the legacy top-level JSON shape so the CLI
daemon route remains a substrate switch, not a client-visible API change.

One prerequisite should be included in implementation: extend
`src/striatum/cli/daemon_rpc_route.py::_route_list` to pass the existing
`list` filters (`run_id`, `state`, `role`, `lane`, `workflow_job_id`, `kind`,
`limit`). It currently returns `{}` for every `list.*` method, so filter
parity is impossible even with correct PG handlers.

## Cross-Cutting Rules

- Layout: one file per RPC method under `reads/`, using filesystem-safe names:
  `status.py`, `dashboard.py`, `why.py`, `doctor.py`, `list_runs.py`,
  `list_sessions.py`, `list_jobs.py`, `list_artifacts.py`,
  `list_workflows.py`, `run_summary.py`, `evidence_export.py`,
  `corpus_export.py`.
- Registration: add `from . import reads` to
  `src/striatum/daemon_pg/handlers/__init__.py`, and import every module from
  `reads/__init__.py` for decorator side effects.
- Scoping: use explicit `WHERE <alias>.repository_id = %s` on every table.
  Joins must include both the foreign id and `repository_id`.
- Errors: malformed or missing required params raise `StriatumError` with
  exit-code-equivalent RPC validation errors; unknown `run_id` / target rows
  raise `NotFoundError`; missing repository context remains the router's
  `repo_not_registered` / `repo_not_migrated` envelope path. Registered PG
  handlers must fail closed and never fall back to `CLI_ROUTES`.
- Parity tests: wire the existing deferred parity rig shape from
  `tests/daemon_pg/handlers/recovery_evidence/conftest.py` into
  `tests/daemon_pg/handlers/reads/conftest.py`. Seed a known SQLite fixture,
  migrate or mirror the same rows into PG, call the legacy function and the PG
  handler with identical params, then assert per-key equality after normalizing
  timestamp string formats and export paths. Prefer per-key diffs over broad
  shape-only smoke tests.
- Export commands: `run.summary`, `evidence.export`, and `corpus.export` write
  requested repository artifacts/directories today. They are read-surface CLI
  verbs because they do not advance workflow control state, but `run.summary`
  and `evidence.export` currently append export events. Implementation must
  either preserve those events in PG using `ctx.append_event(...)` or lock a
  decision to stop recording export events; parity says preserve them.

## Method Designs

### `status`

- Legacy source: `src/striatum/cli/introspect.py:170-225`,
  `status(conn, *, run_id)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/status.py`.
- Decorator/signature:
  `@register_pg_handler("status", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs(run_id,state,branch_name,workflow_snapshot_id,paused_at)`,
  `workflow_snapshots(workflow_snapshot_id,workflow_json)`,
  `jobs(job_id,run_id,workflow_job_id,state,role_id,lane_selector_json,job_type,current_lease_id,created_at)`,
  `sessions(session_id,run_id,role_id,lane_id,slug,ordinal,state,operator_label)`,
  `queue_messages(message_id,run_id,job_id,kind,state,target_role_id,target_lane_id,visible_after,current_lease_id)`,
  `leases(lease_id,run_id,resource_id,owner_session_id,state,expires_at)`,
  `blockers(blocker_id,run_id,job_id,session_id,severity,blocker_kind,description,state,payload_json,created_at)`,
  `verdicts(verdict_id,run_id,job_id,verdict,posture,created_at)`,
  `job_dependencies(job_id,depends_on_job_id,gate_json)`,
  `artifacts(artifact_id,run_id,job_id,logical_name,artifact_kind,repo_path,content_sha256,author_line)`,
  `process_executions(process_id,run_id,job_id,session_id,lease_id,state,pid)`,
  `process_supervisors(supervisor_id,run_id,session_id,state,pid,command_json)`,
  `process_supervisor_pointers(supervisor_id,run_id,session_id,state,pid,pid_start_time,metadata_json)`,
  `job_worktrees(worktree_id,run_id,job_id,lease_id,state,worktree_path)`,
  `events(event_id,run_id,event_type,job_id,payload_json,created_at)`.
- Return shape: top-level keys exactly
  `runs`, `provenance_mode`, `sessions`, `jobs`, `open_blockers`,
  `human_checkpoints`, `latest_non_accepting_review_verdicts`,
  `verdicts_by_posture`, `claimable_jobs`, `blocked_downstream_jobs`,
  `process_health`, `next_actions`, plus phase-progress keys when present
  (`phases`, `current_phase`, `phase_summary` if the existing projection
  emits them).
- Test: `tests/daemon_pg/handlers/reads/test_status.py`. Assert a per-key diff
  against `striatum.cli.introspect.status()` on a fixture with open blockers,
  a non-accepting verdict, a claimable job, a process row, and a phased
  workflow. Include unknown `run_id` behavior: empty status shape is legacy
  parity unless synthesis chooses to harden it.

### `dashboard`

- Legacy source: `src/striatum/dashboard.py:84-211`,
  `gather_payload(repo, *, run_id)`, rendered by `run()` at
  `src/striatum/dashboard.py:360-429`.
- New handler: `src/striatum/daemon_pg/handlers/reads/dashboard.py`.
- Decorator/signature:
  `@register_pg_handler("dashboard", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs(run_id,state,branch_name,workflow_snapshot_id)`,
  `workflow_snapshots(workflow_snapshot_id,workflow_json)`,
  all `status` read tables through the PG status helper,
  `blockers(blocker_id,created_at,payload_json,state)`,
  `events(event_id,run_id,event_type,payload_json,created_at)`,
  `verdicts(verdict_id,run_id,verdict,rationale,posture,created_at)`,
  `jobs(job_id,run_id,workflow_job_id,state,job_type)`,
  `job_dependencies(job_id,depends_on_job_id,gate_json)`.
- Return shape: `run`, `status`, `events`, `verdict_counts`,
  `posture_counts`, `updated_at`, `workflow`, `node_states`,
  `override_verdict_counts`, `override_verdicts`.
- Test: `tests/daemon_pg/handlers/reads/test_dashboard.py`. Compare
  `gather_payload()` output to the PG handler per key, ignoring only
  `updated_at`. Also render both payloads through `render_frame()` and assert
  non-empty frame parity for `--once` defaults.
- Error modes: missing `run_id` is a validation error for this method; unknown
  run raises `NotFoundError`.

### `list.runs`

- Legacy source: `src/striatum/cli/list_commands.py:93-119`,
  `list_runs(conn, *, state, limit)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/list_runs.py`.
- Decorator/signature:
  `@register_pg_handler("list.runs", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs(run_id,state,branch_name,created_at,started_at,completed_at,workflow_snapshot_id)`,
  `workflow_snapshots(workflow_snapshot_id,workflow_id)`.
- Return shape: `items`, `count`; each item has `run_id`, `state`,
  `branch_name`, `created_at`, `started_at`, `completed_at`, `workflow_id`.
- Test: `tests/daemon_pg/handlers/reads/test_list_runs.py`. Per-key diff for
  default limit, state filter, and invalid state validation.
- Error modes: invalid state is an RPC validation error; no rows returns
  `{"items": [], "count": 0}`.

### `list.sessions`

- Legacy source: `src/striatum/cli/list_commands.py:122-168`,
  `list_sessions(conn, *, run_id, state, role, lane)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/list_sessions.py`.
- Decorator/signature:
  `@register_pg_handler("list.sessions", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs(run_id)`,
  `sessions(session_id,run_id,role_id,lane_id,slug,ordinal,state,capabilities_json,registered_at,last_heartbeat_at,operator_label)`,
  `process_supervisors(supervisor_id,run_id,session_id,state,pid,command_json)`,
  `process_supervisor_pointers(supervisor_id,run_id,session_id,state,pid,pid_start_time,metadata_json)`.
- Return shape: `items`, `count`; each item preserves decoded
  `capabilities` and `lane_attestation`, `lane_attestation_reason`,
  `supervisor_id`.
- Test: `tests/daemon_pg/handlers/reads/test_list_sessions.py`. Per-key diff
  for state/role/lane filters and lane-attestation states.
- Error modes: missing/unknown `run_id` raises validation/`NotFoundError`;
  invalid state raises validation error.

### `list.jobs`

- Legacy source: `src/striatum/cli/list_commands.py:171-223`,
  `list_jobs(conn, *, run_id, state, workflow_job_id)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/list_jobs.py`.
- Decorator/signature:
  `@register_pg_handler("list.jobs", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs(run_id)`,
  `jobs(job_id,run_id,workflow_job_id,state,role_id,lane_selector_json,attempt,max_attempts,job_type,created_at,started_at,completed_at)`,
  `verdicts(job_id,verdict,created_at,verdict_id)`,
  `leases(lease_id,run_id,resource_id,state,expires_at)`.
- Return shape: `items`, `count`; each item has decoded `lane_id` and a
  `verdict` key that is `None` for non-review jobs.
- Test: `tests/daemon_pg/handlers/reads/test_list_jobs.py`. Per-key diff for
  state and workflow-job filters plus latest-verdict ordering.
- Error modes: missing/unknown `run_id` and invalid state as above.

### `list.artifacts`

- Legacy source: `src/striatum/cli/list_commands.py:226-266`,
  `list_artifacts(conn, *, run_id, kind)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/list_artifacts.py`.
- Decorator/signature:
  `@register_pg_handler("list.artifacts", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs(run_id,workflow_snapshot_id)`,
  `workflow_snapshots(workflow_snapshot_id,workflow_json)`,
  `artifacts(artifact_id,run_id,job_id,session_id,logical_name,artifact_kind,repo_path,content_sha256,size_bytes,created_at,author_line)`,
  `jobs(job_id,workflow_job_id,role_id,lane_selector_json,expected_artifacts_json)`,
  `sessions(session_id,role_id,lane_id,ordinal,operator_label)`,
  `process_supervisors` / `process_supervisor_pointers` for author
  attestation parity if reused from evidence summaries.
- Return shape: `items`, `count`; each item includes `author`.
- Test: `tests/daemon_pg/handlers/reads/test_list_artifacts.py`. Per-key diff
  for kind filter and author identity.
- Error modes: missing/unknown `run_id` and invalid kind as above.

### `list.workflows`

- Legacy source: `src/striatum/cli/list_commands.py:269-288`,
  `list_workflows(conn, *, limit)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/list_workflows.py`.
- Decorator/signature:
  `@register_pg_handler("list.workflows", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `workflow_snapshots(workflow_snapshot_id,workflow_id,workflow_version,source_path,content_sha256,loaded_at)`.
- Return shape: `items`, `count`.
- Test: `tests/daemon_pg/handlers/reads/test_list_workflows.py`. Per-key diff
  for default and explicit limit.
- Error modes: invalid limit is validation error; no rows returns an empty
  envelope.

### `run.summary`

- Legacy source: `src/striatum/cli/run_summary.py:23-38`,
  `run_summary_export(...)`, and `src/striatum/cli/run_summary.py:41-110`,
  `run_summary_snapshot(...)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/run_summary.py`.
- Decorator/signature:
  `@register_pg_handler("run.summary", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs(run_id,workflow_snapshot_id,branch_name,state,created_at,started_at,completed_at)`,
  `workflow_snapshots(workflow_snapshot_id,workflow_json)`,
  all PG `status` and `doctor` read tables,
  `artifacts(...)`, `sessions(...)`, `verdicts(...)`, `jobs(...)`,
  `blockers(blocker_id,job_id,severity,blocker_kind,state,created_at)`,
  `events` only if export-event parity is preserved.
- Return shape: `status`, `run_id`, `path`, `sha256`.
- Test: `tests/daemon_pg/handlers/reads/test_run_summary.py`. Generate both
  Markdown bodies on the same fixture, compare digest and top-level response,
  allowing only current-git-branch differences if the fixture changes branch.
- Error modes: missing `run_id`/`path` validation error; unknown run
  `NotFoundError`; unsafe path uses the existing repo-relative path error.

### `why`

- Legacy source: `src/striatum/cli/introspect.py:564-681`,
  `why(conn, *, target_id)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/why.py`.
- Decorator/signature:
  `@register_pg_handler("why", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs`, `jobs`, `queue_messages`, `blockers`, `artifacts`, `verdicts`,
  `sessions`, `process_executions`, `events`, `job_dependencies`, and status
  helper tables, always repository-scoped. Select `*` for the primary target
  table to preserve legacy row dictionaries.
- Return shape depends on target:
  run: `target_type`, `run`, `jobs`, `open_blockers`, `events`,
  `next_actions`;
  job/message: `target_type`, `job`, `message`, `verdict`, `blockers`,
  `downstream_jobs`, `events`;
  blocker: `target_type`, `blocker`, `run`, `job`, `session`,
  `related_verdict`, `blocked_downstream_jobs`, `human_checkpoint`,
  `next_actions`, `events`;
  artifact: `target_type`, `artifact`, `job`, `verdicts`, `events`;
  verdict: `target_type`, `verdict`, `job`, `artifact`, `blockers`, `events`;
  session: `target_type`, `session`, `jobs`, `events`;
  process: `target_type`, `process`, `job`, `session`, `events`.
- Test: `tests/daemon_pg/handlers/reads/test_why.py`. Fixture should include
  one id of every supported target type; assert exact top-level keys and
  per-key values vs SQLite. Unknown target must raise `NotFoundError`.

### `doctor`

- Legacy source: `src/striatum/cli/introspect.py:1204-1824`,
  `doctor(conn, *, repo, run_id, verbose)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/doctor.py`.
- Decorator/signature:
  `@register_pg_handler("doctor", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs`, `workflow_snapshots`, `jobs`, `job_dependencies`,
  `queue_messages`, `leases`, `sessions`, `blockers`, `artifacts`,
  `verdicts`, `work_packets`, `job_worktrees`, `process_executions`,
  `process_supervisors`, `process_supervisor_pointers`, and filesystem checks
  rooted at `ctx.repo_root` for worktree paths, skills, plugins, and editable
  install checks.
- Return shape: at minimum `ok`, `problems`; when `verbose` is true also
  `problem_records`. Preserve every legacy `check` string.
- Test: `tests/daemon_pg/handlers/reads/test_doctor.py`. Seed one clean run
  and one run with representative problems
  (`active_job_without_active_lease`, `stale_queue_message_claim`,
  `orphan_work_packet`, `process_running_but_pid_gone`). Assert string
  problems and verbose records match SQLite.
- Error modes: unknown run should return `ok: true` with no scoped problems
  only if SQLite does; otherwise harden both paths together.

### `evidence.export`

- Legacy source: `src/striatum/cli/evidence.py:356-383`,
  `evidence_export(...)`, plus `src/striatum/cli/evidence.py:386-426`,
  `evidence_snapshot(...)`.
- New handler: `src/striatum/daemon_pg/handlers/reads/evidence_export.py`.
- Decorator/signature:
  `@register_pg_handler("evidence.export", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  `runs(run_id,workflow_snapshot_id,branch_name,state)`,
  `workflow_snapshots(workflow_snapshot_id,workflow_id,workflow_version,workflow_json)`,
  all PG `status` and `doctor` read tables,
  `jobs`, `artifacts`, `sessions`, `verdicts`, `blockers`,
  `job_dependencies`, `process_supervisors`, `process_supervisor_pointers`,
  `events` if preserving `evidence.exported`.
- Return shape: `status`, `run_id`, `path`, `sha256`.
- Test: `tests/daemon_pg/handlers/reads/test_evidence_export.py`. Compare
  redacted payloads and rendered Markdown digest against the SQLite export on
  a fixture with verdict rationale, blocker description, author bylines, and
  process rows. Keep the existing redactor and renderer imported from
  `striatum.cli.evidence`; do not fork redaction policy.
- Error modes: missing `run_id`/`path`, unknown run, and unsafe path as for
  `run.summary`.

### `corpus.export`

- Legacy source: `src/striatum/corpus/export.py:16-48`,
  `export_corpus_bundle(...)`, called by `src/striatum/cli/dispatch.py:654-662`.
- New handler: `src/striatum/daemon_pg/handlers/reads/corpus_export.py`.
- Decorator/signature:
  `@register_pg_handler("corpus.export", read_only=True)`;
  `def handle(ctx: RepoHandlerContext, params: dict) -> dict`.
- Read set:
  PG live-state reads used by corpus enumeration:
  `runs`, `workflow_snapshots`, `jobs`, `sessions`, `artifacts`,
  `verdicts`, `blockers`, `events`, plus the repository filesystem and git
  history for RFCs, decision log, operator reports, checked-in run summaries,
  changelog, ubiquitous language, friction patterns, and commits.
  `manifest.repo_local_schema_version` should become the daemon PG substrate
  version or a compatibility value agreed in synthesis; it cannot use SQLite
  `PRAGMA user_version`.
- Return shape: preserve `CorpusBundleResult.to_json(repo=repo)` keys:
  `status`, `schema_version`, `since_ref`, `since_commit`, `out`,
  `manifest_path`, `row_counts`, `bundle_sha256`.
- Test: `tests/daemon_pg/handlers/reads/test_corpus_export.py`. Export both
  bundles into separate temp dirs from the same fixture and compare manifest
  keys, JSONL row counts, and bundle hash after normalizing generated
  timestamps and substrate-version field. Also assert `--out` cannot escape
  the repo or enter `.striatum`.
- Error modes: invalid `since`, missing `out`, unsafe output path, and git
  resolution errors must surface as RPC errors; no changed files still writes a
  valid empty-row bundle.

## Implementation Notes

The read handlers should share small PG projection helpers, but not by calling
legacy SQLite functions. Good candidates:

- `reads/_status.py` for status sub-projections reused by dashboard,
  run summary, and evidence export.
- `reads/_doctor.py` for doctor checks reused by run summary and evidence
  export.
- `reads/_evidence.py` for PG equivalents of `evidence_snapshot`,
  `evidence_session_summaries`, and `evidence_artifact_summaries`.
- `reads/_corpus.py` only for the live-state enumerators; keep git/filesystem
  corpus enumerators unchanged.

The parity fixture should normalize psycopg JSONB values into plain dict/list
objects, timestamps into the legacy UTC string form, and bigint event ids into
ints before comparing. All tests should include a negative monkeypatch that
forces the registered PG handler to raise and asserts the daemon router returns
an RPC error rather than invoking `striatum.api.invoke`.

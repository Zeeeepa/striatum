---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs:
  - "docs/rfcs/0048-daemon-side-substrate-migration.md"
  - "docs/dogfood/057/build/track_a/HANDOFF.md"
  - "docs/dogfood/057/build/track_b/HANDOFF.md"
  - "docs/POSTGRES_TRANSITION.md"
  - "docs/dogfood/060/prompts/design.md"
---
author: designer-unknown-model-001

# Design — RFC 0048 Phase C read-surface PG handlers

## 1. Goal & non-goals

**Goal.** Port the eight daemon-RPC read methods that still fall through
`DaemonRpcRouter._route` → `CLI_ROUTES[<method>]` → `striatum.api.invoke` →
`striatum.db.connect` (legacy SQLite) to native PostgreSQL handlers under
`src/striatum/daemon_pg/handlers/reads/`. After this dogfood lands, a repo
that has run `striatum daemon migrate-repo-local` returns real run state
from `striatum status`, `striatum dashboard`, `striatum list ...`, `striatum
run summary`, `striatum why`, `striatum doctor`, `striatum evidence export`,
and `striatum corpus export` — instead of the current exit 3
`state is not initialized` from a tombstoned SQLite.

**Methods in scope (8 distinct registered RPC methods, 12 CLI verbs).**
The work packet's `Method | Legacy contract source` table conflates RPC
methods and CLI subcommands. The actual `DaemonRpcRouter` registry
(`src/striatum/daemon_rpc/registry.py:53-72`) groups them like this:

| RPC method | CLI verb(s) | Legacy contract |
|---|---|---|
| `status` | `striatum status` | `src/striatum/cli/introspect.py:170-225 status()` |
| `dashboard` | `striatum dashboard` (one-shot) | `src/striatum/dashboard.py:84-211 gather_payload()` (also calls `status()`) |
| `why` | `striatum why` | `src/striatum/cli/introspect.py:564-681 why()` |
| `doctor` | `striatum doctor` | `src/striatum/cli/introspect.py:1204-1810 doctor()` |
| `run.summary` | `striatum run summary` | `src/striatum/cli/run_summary.py:23-110 run_summary_export()` + `run_summary_snapshot()` |
| `list.runs` / `list.sessions` / `list.jobs` / `list.artifacts` / `list.workflows` | `striatum list <subcommand>` | `src/striatum/cli/list_commands.py:93-288` |
| `evidence.export` | `striatum evidence export` | `src/striatum/cli/evidence.py:356-426 evidence_export()` + `evidence_snapshot()` |
| `corpus.export` | `striatum corpus export` | `src/striatum/corpus/export.py:16-48 export_corpus_bundle()` |

**Non-goals.**

- Phase B (Go core parity) and the V1.5 follow-ups (codex F2/F3/F4,
  claude HIGH#2 dead code, schema 0006). They have their own dogfoods.
- Editing `src/striatum/cli/daemon_rpc_route.py` (forbidden by the
  workflow `forbidden_paths`; v1.51.0 already wired the CLI hook).
- Editing `DaemonRpcRouter._route`. The existing
  `resolve_pg_handler()` lookup at `src/striatum/daemon_rpc/server.py:235`
  already wires registered handlers in front of the SQLite delegation, so
  this dogfood adds modules that self-register and the router picks them
  up automatically.
- Editing `src/striatum/daemon_pg/sql/` (schema is locked through 0005;
  V1.5 schema 0006 is a separate fix-up dogfood).
- Re-implementing `dashboard.render_frame()`, `evidence.render_evidence_markdown()`,
  `run_summary.render_run_summary_markdown()`, or `corpus.export.write_jsonl_bundle()`
  / `redaction.*` — those are pure Python presentation/redaction helpers
  imported by reference from the new handlers (mirroring how
  `evidence.export` already imports `redact_evidence_payload` and
  `render_evidence_markdown` from `striatum.cli.evidence`).

## 2. Cross-cutting decisions (synthesis MUST lock)

### 2.1 Module layout

**Single file per RPC method** under
`src/striatum/daemon_pg/handlers/reads/`. This matches Track A
(`workflow_loop/`) and Track B (`recovery_evidence/`) and keeps each
method's SQL grep-able under one path. Shared SELECT helpers go into
`reads/_sql.py`; shared read-model builders (status snapshot, doctor
checks, evidence snapshot) go into `reads/_read_model.py` so
`evidence.export`, `run.summary`, and `dashboard` consume them without
duplicating SQL.

```
src/striatum/daemon_pg/handlers/reads/
├── __init__.py                # decorator import side-effects
├── _sql.py                    # repo-scoped fetch_one / fetch_all / parse_json / lazy expire
├── _read_model.py             # shared status / doctor / evidence_snapshot / next_actions
├── status.py                  # @register_pg_handler("status")
├── dashboard.py               # @register_pg_handler("dashboard")
├── why.py                     # @register_pg_handler("why")
├── doctor.py                  # @register_pg_handler("doctor")
├── run_summary.py             # @register_pg_handler("run.summary")
├── list_runs.py               # @register_pg_handler("list.runs")
├── list_sessions.py           # @register_pg_handler("list.sessions")
├── list_jobs.py               # @register_pg_handler("list.jobs")
├── list_artifacts.py          # @register_pg_handler("list.artifacts")
├── list_workflows.py          # @register_pg_handler("list.workflows")
├── evidence_export.py         # @register_pg_handler("evidence.export")  -- replaces stub
└── corpus_export.py           # @register_pg_handler("corpus.export")
```

**Note on `evidence.export`.** The Track B Phase A handler at
`src/striatum/daemon_pg/handlers/recovery_evidence/evidence_export.py`
already self-registers `evidence.export`, but its `_status` and `_doctor`
helpers are stubs (`_status` drops `claimable_jobs` /
`blocked_downstream_jobs` / `next_actions` / `verdicts_by_posture` etc.;
`_doctor` returns `{ok: True, problems: []}` always). This dogfood
**relocates** the handler to `reads/evidence_export.py` so it shares the
real `_read_model.py::status_pg()` / `doctor_pg()` / `evidence_snapshot_pg()`
that all four read consumers (status, dashboard, run.summary,
evidence.export) depend on. The Track B file is deleted in the same
commit; its `__init__.py` import line is removed too. Track B's tests
under `tests/daemon_pg/handlers/recovery_evidence/test_evidence_export.py`
are migrated to `tests/daemon_pg/handlers/reads/test_evidence_export.py`
to follow the handler. The decorator key remains `evidence.export` so
registry lookup at `src/striatum/daemon_rpc/server.py:235` is unchanged.

### 2.2 `repository_id` scoping mechanism

**Explicit `WHERE repository_id = %(repository_id)s` in every SELECT,
funnelled through `_sql.fetch_all(ctx, sql=..., params=...)` which
auto-injects `repository_id` into the parameter dict.** This is the
exact pattern Track B locked in
`src/striatum/daemon_pg/handlers/recovery_evidence/_sql.py:61-73` and
satisfies F1 (fail-closed routing): a forgotten predicate fails loudly
because the helper sets `repository_id` but the SQL must still reference
it, and the test rig (§ 4.2) seeds two repositories and asserts cross-repo
isolation by default.

We do **not** introduce a wrapper function or stored procedure. They hide
the scope from grep, push the predicate further from the SQL, and break
the locked Track B pattern. The Phase B Go port (out of scope here) will
mirror the same shape.

### 2.3 Lazy lease-expiry inside reads

Three legacy reads call `expire_leases()` before reading
(`introspect.py:172` for `status`, `list_commands.py:137,186,240` for
`list.sessions` / `list.jobs` / `list.artifacts`). These are
state-mutating side effects that the SQLite reads have always performed.
The PG ports keep parity by calling
`src/striatum/daemon_pg/handlers/recovery_evidence/_sql.py::expire_leases(ctx,
run_id=...)` — already implemented with the matching repo-write vs
non-repo-write semantics and audit events. Since `_sql.py::expire_leases`
already lives under `recovery_evidence/`, `reads/_sql.py` re-exports it
verbatim as a single import; we do **not** copy it.

A read with no `run_id` filter (legacy `status` / `dashboard` /
`list.runs`) does not call `expire_leases` (matches SQLite — see
`introspect.py:172` `if run_id is not None`).

### 2.4 Parity test strategy

**Wire the unused parity rig + per-key diffs against the SQLite path.**
The rig advertised at
`tests/daemon_pg/handlers/recovery_evidence/conftest.py` (Seed dataclass +
`pg_ctx` + `sqlite_conn`) is documented as deferred V1.5 work (claude
HIGH#1). It has been deferred from V1.5 (dogfood-058) per the V1.5 scope
freeze. Without it, this dogfood's reads can ship "registered + smoke
tests pass" while quietly drifting from the SQLite contract. Reads are
exactly where parity matters: they have no chained side effects to mask
divergence.

We extend the rig under `tests/daemon_pg/handlers/reads/conftest.py`
(separate file so we are not entangled with Track B's HIGH#1 backlog):

- `Seed` grows fields the read tests need: a workflow snapshot row, a
  registered session, a published artifact, an open blocker, a recorded
  verdict, an expired lease. The seeder writes byte-equivalent rows into
  both PG (`pg_db`) and SQLite (`sqlite_conn`) under one `populate(...)`
  call.
- `assert_payload_parity(pg_payload, sqlite_payload)` is a thin per-key
  walk that diffs after stripping the small set of expected divergences
  (timestamps with sub-second drift; nothing else — both substrates store
  ISO-8601 second-precision after our porting).
- The `RFC0048_PARITY=1` env gate that `recovery_evidence/conftest.py`
  uses today stays in place for the new conftest **only** as long as
  `tests/_harness/pg.py::resolve_base_url()` is the reachability gate.
  Once a reachable PG is configured, parity runs by default.

Each per-method test calls the legacy SQLite path on `sqlite_conn`,
calls the PG handler on `pg_ctx`, and asserts
`assert_payload_parity(pg, sqlite)`. Smoke tests (registration,
signature, `repo_not_registered` error envelope) live alongside but are
not the contract — parity is.

**Smoke-only is rejected.** The legacy reads are not stable: introspect.py
shipped behavior changes in HARNESS-001/HARNESS-003 and RFC 0018 V1.5
(per-posture verdict counts) within the last few months. A smoke test
would not have caught any of those.

### 2.5 Single implement track

Synthesis MUST lock a single implement track. The implementer claims one
job and uses sub-agents inside the track for parallelism (e.g., one
sub-agent per `list.*` method, one per status/doctor/why/run.summary
trio, one for evidence/corpus/dashboard). Splitting into "core reads"
and "reporting reads" tracks recreates the boundary conflicts that
caused dogfood-058's cycle exhaustion: `evidence.export` reads
`status_pg()` and `doctor_pg()`; `dashboard` reads `status_pg()`;
`run.summary` reads `status_pg()` and `doctor_pg()` and the same
artifact/session/verdict summaries `evidence.export` reads. There is one
shared `_read_model.py`. One owner.

### 2.6 Fail-closed routing & error envelopes

Every handler raises `RpcError` (with codes from the existing vocabulary)
rather than returning a stub on missing inputs. The router converts
`RpcError` to an `RpcResponse.error_response` envelope at
`server.py:159-161`. Read handlers use:

| Condition | Code | Message shape |
|---|---|---|
| `repository_id` missing on the envelope | `repo_not_registered` | already raised by `server.py:131-133` before dispatch |
| `run_id` filter param refers to a non-existent run | `not_found` | `f"run not found: {run_id}"` (mirrors `_ensure_run_exists` in `list_commands.py:87-90`) |
| `target_id` for `why` matches no row | `not_found` | mirrors `introspect.py:679-681` |
| Required `path`/`run_id` parameter missing | `schema_invalid` | `f"<method> requires <field>"` |
| State counter SELECT returns zero rows | empty list / `{}` per legacy contract | NEVER raise (parity with SQLite: empty repo returns `{"items": [], "count": 0}` not an error) |

Because handlers self-register, the F1 fail-closed routing rule (V1.5
codex F1) is satisfied transitively — once the handler is registered,
the router never falls through to `striatum.api.invoke` for that method.
Each per-method test asserts the route does not call `invoke()` by
monkeypatching `striatum.api.invoke` to raise.

## 3. Per-method specifications

The columns below are: **(a)** legacy source `path:lines`, **(b)** new PG
handler path, **(c)** decorator + signature, **(d)** `striatumd.*` tables
queried with the SELECT column set, **(e)** return-shape parity contract
(top-level keys), **(f)** test path + parity strategy, **(g)** error
modes.

### 3.1 `status`

- **Legacy.** `src/striatum/cli/introspect.py:170-225 status()`.
  Calls `expire_leases` (when `run_id` set), counts runs, jobs by state,
  open blockers + human checkpoints, latest non-accepting review
  verdicts, claimable jobs, blocked downstream jobs, process health,
  computes `next_actions`. When `run_id` is set, augments with
  `phase_progress_for_run()`.
- **New handler.** `src/striatum/daemon_pg/handlers/reads/status.py::handle`.
- **Decorator + signature.**
  ```python
  @register_pg_handler("status", read_only=True)  # see § 5 on the read_only flag
  def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]: ...
  ```
- **Tables (read set).**
  - `striatumd.runs` — `run_id, state, branch_name, created_at, started_at, completed_at, workflow_snapshot_id, paused_at, stop_reason`
  - `striatumd.jobs` — `state` (count by group), `job_id, workflow_job_id, role_id, lane_selector_json, current_lease_id, expected_artifacts_json, attempt, max_attempts, job_type` for the per-row helpers
  - `striatumd.workflow_snapshots` — `workflow_json` (for `_provenance_mode_for_status` and `phase_progress_for_run`)
  - `striatumd.sessions` — `session_id, run_id, role_id, lane_id, slug, state, operator_label, registered_at` (for attestation summaries)
  - `striatumd.process_supervisors` — `supervisor_id, pid, state, started_at` (joined to sessions for `session_lane_attestation`)
  - `striatumd.blockers` — `blocker_id, run_id, job_id, session_id, severity, blocker_kind, description, state` (joined to jobs for `workflow_job_id`)
  - `striatumd.verdicts` — `verdict_id, job_id, run_id, verdict, posture, created_at` (latest-non-accepting + verdicts-by-posture aggregations)
  - `striatumd.queue_messages` — `message_id, job_id, role_id, lane_id, state` (for `claimable_jobs_by_role_lane`)
  - `striatumd.leases` — `lease_id, owner_session_id, state, expires_at, resource_id` (for `_has_supervisor_lost_with_held_lease`, `_has_stale_leases_with_on_disk_artifacts`)
  - `striatumd.process_executions` — `process_id, job_id, session_id, state, pid` (for `_process_health`)
  - All SELECTs scoped by `repository_id = %(repository_id)s`. Joins always carry `b.repository_id = j.repository_id` etc.
- **Return-shape parity contract (top-level keys).** Verbatim from `introspect.py:207-220`:
  - `runs`: `list[{run_id, state, branch_name}]`
  - `provenance_mode`: `str | dict[run_id, str]`
  - `sessions`: `list[{session_id, run_id, role_id, lane_id, slug, state, operator_label, lane_attestation, lane_attestation_reason, supervisor_id}]`
  - `jobs`: `dict[state, count]`
  - `open_blockers`: `list[blocker_summary]`
  - `human_checkpoints`: `list[blocker_summary]`
  - `latest_non_accepting_review_verdicts`: `list[verdict_row]`
  - `verdicts_by_posture`: `dict[posture, dict[verdict, count]]`
  - `claimable_jobs`: `list[claimable_summary]`
  - `blocked_downstream_jobs`: `list[blocked_summary]`
  - `process_health`: `dict` matching `_process_health` shape
  - `next_actions`: `list[str]` (computed by `next_actions()` from `introspect.py:857-927` — **import and reuse, do not re-implement**)
  - When `run_id is not None`: also `phases` and `current_phase_id` from `phase_progress_for_run`.
- **Reuse, do not fork:**
  `striatum.cli.introspect.next_actions(...)` and
  `striatum.workflow.workflow_phase_index(...)` are pure functions over
  inputs. Import them and feed the PG-derived inputs in. Forking either
  is forbidden — they are the contract.
- **Test.** `tests/daemon_pg/handlers/reads/test_status.py`. Per-key
  parity vs `striatum.cli.introspect.status(sqlite_conn, run_id=...)`.
  Fixtures: a registered run with one queued job, one blocked job, one
  open blocker, one stale lease, one non-accepting verdict.
- **Error modes.** `repo_not_registered` (router-level), `not_found`
  (`f"run not found: {run_id}"`) when `run_id` is set and missing.
  Empty repo → returns the legacy empty shape.

### 3.2 `dashboard`

- **Legacy.** `src/striatum/dashboard.py:84-211 gather_payload()`.
  Returns `{run, status, events, verdict_counts, posture_counts,
  override_verdict_counts, override_verdicts, updated_at, workflow,
  node_states}`. Calls `status()` from `introspect.py`, reads recent
  events, builds verdict/posture counts, augments verdict events with
  rationale, and computes workflow node states.
- **New handler.** `src/striatum/daemon_pg/handlers/reads/dashboard.py::handle`.
- **Decorator + signature.** `@register_pg_handler("dashboard", read_only=True)` with the same `(ctx, params)` signature.
- **Tables (read set).**
  - All tables from § 3.1 (delegates to `_read_model.status_pg`).
  - `striatumd.events` — `event_id, run_id, event_type, payload_json, created_at, actor_session_id, job_id, message_id, artifact_id, lease_id` (for `recent_events_for_run`, ordered desc, limit 10).
  - `striatumd.blockers` — `blocker_id, created_at, payload_json` (for `blocker_payloads` augmentation).
  - `striatumd.verdicts` — `verdict_id, verdict, posture, rationale` (for verdict counts and rationale lookups).
  - `striatumd.workflow_snapshots` — `workflow_json` (passed into `compute_node_states`).
- **Return-shape parity contract.** Verbatim from `dashboard.py:200-211`:
  `{run: {run_id, state, branch_name}, status, events, verdict_counts,
  posture_counts, updated_at, workflow, node_states,
  override_verdict_counts, override_verdicts}`.
- **Reuse:** `striatum.workflow.compute_node_states` (pure function over
  workflow + jobs/states), `striatum.dashboard._json_object` (private but
  trivial — keep importable). `render_frame()` is a CLI presentation
  layer and is **not** invoked by the RPC method (the RPC returns
  `gather_payload()`'s dict; the CLI layer renders it). Confirmed at
  `src/striatum/cli/daemon_rpc_route.py` — `dashboard` returns the
  payload, the CLI verb prints it.
- **Test.** `tests/daemon_pg/handlers/reads/test_dashboard.py`. Per-key
  parity vs `striatum.dashboard.gather_payload(repo, run_id=...)`.
  Fixture seeds the same run shape as § 3.1 plus a recent
  `verdict.overridden` event with a verdict_id whose rationale lives in
  `striatumd.verdicts`.
- **Error modes.** `not_found` when the run is missing
  (mirrors `dashboard.py:100-101`).

### 3.3 `why`

- **Legacy.** `src/striatum/cli/introspect.py:564-681 why()`. Resolves
  `target_id` against runs, jobs, queue_messages, blockers, artifacts,
  verdicts, sessions, process_executions in order; returns the matching
  envelope. Raises `NotFoundError` when nothing matches.
- **New handler.** `src/striatum/daemon_pg/handlers/reads/why.py::handle`.
- **Decorator + signature.** `@register_pg_handler("why", read_only=True)` with `(ctx, params)`.
- **Tables (read set).** All eight target tables, queried in the same
  order as the legacy implementation:
  `striatumd.runs`, `striatumd.jobs` (matched by `job_id` OR
  `workflow_job_id`), `striatumd.queue_messages`, `striatumd.blockers`,
  `striatumd.artifacts`, `striatumd.verdicts`, `striatumd.sessions`
  (matched by `session_id` OR `slug`), `striatumd.process_executions`.
  Plus the helpers' read set: `events_for(...)`, `blockers_for_job(...)`,
  `downstream_jobs(...)`, `latest_verdict_row(...)`, `jobs_for_run(...)`,
  `jobs_for_session(...)`, etc. — these become `_read_model.py` helpers.
- **Return-shape parity contract.** One of seven shapes keyed by
  `target_type`:
  - `target_type=run`: `{target_type, run, jobs, open_blockers, events, next_actions}`
  - `target_type=job` or `message`: `{target_type, job, message, verdict, blockers, downstream_jobs, events}`
  - `target_type=blocker`: `{target_type, blocker, run, job, session, related_verdict, blocked_downstream_jobs, human_checkpoint, next_actions, events}`
  - `target_type=artifact`: `{target_type, artifact, job, verdicts, events}`
  - `target_type=verdict`: `{target_type, verdict, job, artifact, blockers, events}`
  - `target_type=session`: `{target_type, session, jobs, events}`
  - `target_type=process`: `{target_type, process, job, session, events}`
- **Reuse.** `human_checkpoint_context()` from `introspect.py:710-742`
  is a pure function over a blocker dict + blocked_jobs list — import.
  `next_actions()` likewise.
- **Test.** `tests/daemon_pg/handlers/reads/test_why.py`. Per-key
  parity for each `target_type` branch (one test per branch). Fixture
  seeds one row per target type so all eight branches are covered in a
  single test module.
- **Error modes.** `not_found` with the legacy message
  (`introspect.py:679-681`).

### 3.4 `doctor`

- **Legacy.** `src/striatum/cli/introspect.py:1204-1810 doctor()`. The
  largest read in the codebase (606 lines). Runs 23 named consistency
  checks, each producing a `problem` string and a structured
  `problem_record`. The check name vocabulary is API
  (`introspect.py:1223-1248 DOCTOR_CHECKS` tuple).
- **New handler.** `src/striatum/daemon_pg/handlers/reads/doctor.py::handle`.
- **Decorator + signature.** `@register_pg_handler("doctor", read_only=True)` with `(ctx, params)`.
- **Tables (read set).** Many. Documented per check in `_read_model.doctor_pg`:
  - `runs` joined to `workflow_snapshots` (`sealed_patch_unsupported`)
  - `jobs` left-joined to `leases` (`active_job_without_active_lease`,
    `job_current_lease_id_inconsistent`,
    `process_running_with_expired_lease`)
  - `job_dependencies` joined to `jobs` and `verdicts`
    (`dependency_gate_json_invalid`,
    `completed_review_dependency_lacks_accepting_verdict`)
  - `jobs.expected_artifacts_json` joined to `artifacts`
    (`required_artifact_kind_or_path_mismatch`)
  - `runs` left-joined to `jobs` (`run_running_with_no_progressable_jobs`,
    `open_blocker_on_terminal_run`, `active_session_on_terminal_run`)
  - `queue_messages` (`stale_queue_message_claim`,
    `job_current_message_id_inconsistent`)
  - `leases` (`unreaped_expired_lease`)
  - `work_packets` left-joined to `jobs`/`messages` (`orphan_work_packet`)
  - `job_worktrees` joined to `leases` (`worktree_orphaned_lease`,
    `worktree_path_missing_on_disk`)
  - `process_supervisors` joined to `leases`
    (`supervisor_lost_with_held_lease`)
  - `process_executions` joined to `leases` (`process_running_but_pid_gone`,
    `process_running_with_expired_lease`)
  - File-system + `sys.path` checks (`editable_install_outside_repo`,
    `skills_missing`, `skills_outdated`, `plugin_missing`, `plugin_outdated`)
    are pure-Python helpers — import and call. They do not depend on the
    substrate.
- **Return-shape parity contract.** `{ok, schema_version: "striatum.doctor.v1",
  problems: list[str], problem_records: list[{check, id, context}]}`.
  When `verbose` is False the legacy implementation still computes
  `problem_records` and just elides them from the return — the new
  handler matches this so callers cannot detect the substrate.
- **Implementation discipline.** Each check is one method on a private
  `_DoctorRun` dataclass in `_read_model.py`; the dataclass holds
  `(ctx, run_id, problems, records, report)` and the public `doctor_pg`
  function is `_DoctorRun(ctx, run_id).run()`. This mirrors the
  legacy function's local `report(...)` closure. **Do not** invent new
  check names; the `DOCTOR_CHECKS` tuple is API and the new handler
  imports it verbatim.
- **Test.** `tests/daemon_pg/handlers/reads/test_doctor.py`. Per-check
  parity: one fixture per check that seeds the failure shape, one
  fixture for the all-clean baseline. Then per-key parity vs
  `striatum.cli.introspect.doctor(sqlite_conn, repo=..., run_id=...)`.
- **Error modes.** No raise on empty inputs (parity: an empty repo is
  doctor-clean). `not_found` when `run_id` is set and missing.

### 3.5 `run.summary`

- **Legacy.** `src/striatum/cli/run_summary.py:23-110`.
  `run_summary_export()` writes a Markdown file via
  `render_run_summary_markdown()` over `run_summary_snapshot()`'s
  payload. The snapshot composes `status()`, `doctor()`,
  `evidence_artifact_summaries()`, `evidence_session_summaries()`, and
  per-run verdicts/blockers reads, plus a branch + timing context.
  Returns `{status: "exported", run_id, path, sha256}` and inserts a
  `run_summary.exported` audit event.
- **New handler.** `src/striatum/daemon_pg/handlers/reads/run_summary.py::handle`.
- **Decorator + signature.** `@register_pg_handler("run.summary", read_only=False)` — note this method **writes** the markdown file and inserts an audit event, so it is not pure read; `read_only=False` documents that the route appends an event.
- **Tables (read set).** Inherits from § 3.1 (status), § 3.4 (doctor),
  plus `verdicts` (verdict listing for the run) and `blockers` (blocker
  listing for the run). Shared with § 3.7 (`evidence.export`) via
  `_read_model.evidence_artifact_summaries_pg` /
  `evidence_session_summaries_pg`.
- **Tables (write set).** `striatumd.events` — single
  `run_summary.exported` event with payload `{path, sha256}`.
- **Return-shape parity contract.** `{status: "exported", run_id, path, sha256}`.
- **Reuse.** `render_run_summary_markdown(run=..., summary=...)` is a
  pure presentation function — import. `_group_verdicts_by_workflow_job`
  and `_format_run_duration` are private helpers; widen visibility (rename
  with leading `_` removed) or copy via reuse — preference: re-export from
  `striatum.cli.run_summary` since they have no SQLite dependency.
- **Test.** `tests/daemon_pg/handlers/reads/test_run_summary.py`. Parity
  on the resulting markdown body bytes (`sha256` equality) AND on the
  RPC return envelope.
- **Error modes.** `not_found` (run missing); `schema_invalid` when
  `path` is missing; `path_outside_scope` mirrors evidence (write must
  be inside the repo via `repo_relative_path`).

### 3.6 `list.runs` / `list.sessions` / `list.jobs` / `list.artifacts` / `list.workflows`

Five separate handlers; one file per method (§ 2.1). Common shape:
`@register_pg_handler("list.<sub>", read_only=True)` with
`(ctx, params)` returning `{items: list, count: int}` (the
`_envelope(...)` shape from `list_commands.py:83-84`).

#### 3.6.1 `list.runs`

- **Legacy.** `list_commands.py:93-119`.
- **Handler.** `reads/list_runs.py::handle`.
- **Tables.** `striatumd.runs` left-joined to `workflow_snapshots` —
  `r.run_id, r.state, r.branch_name, r.created_at, r.started_at,
  r.completed_at, w.workflow_id`. Optional `state` filter validated
  against `RUN_STATES` tuple.
- **Return.** `{items: [{run_id, state, branch_name, created_at,
  started_at, completed_at, workflow_id}], count: int}`.
- **Test.** `tests/daemon_pg/handlers/reads/test_list_runs.py`. Parity
  + `_validate_choice` rejects unknown state with exit_code=2.
- **Errors.** `schema_invalid` for unknown state (matches
  `StriatumError` exit_code 2 from the legacy).

#### 3.6.2 `list.sessions`

- **Legacy.** `list_commands.py:122-168`. Calls `expire_leases` first.
- **Handler.** `reads/list_sessions.py::handle`.
- **Tables.** `striatumd.sessions` (with session attestation join via
  `process_supervisors`) — `session_id, role_id, lane_id, slug, ordinal,
  state, capabilities_json (parsed back to list), registered_at,
  last_heartbeat_at, operator_label`. Lazy `expire_leases(ctx, run_id=...)`
  before reading.
- **Return.** `{items: [{session_id, role_id, lane_id, slug, ordinal,
  state, capabilities, registered_at, last_heartbeat_at, operator_label,
  lane_attestation, lane_attestation_reason, supervisor_id}], count: int}`.
- **Test.** `tests/daemon_pg/handlers/reads/test_list_sessions.py`.
  Parity covers `capabilities_json` parsing AND the attestation triad.
- **Errors.** `not_found` (missing run); `schema_invalid` (unknown state).

#### 3.6.3 `list.jobs`

- **Legacy.** `list_commands.py:171-223`. Calls `expire_leases`.
- **Handler.** `reads/list_jobs.py::handle`.
- **Tables.** `striatumd.jobs` left-joined to a per-job latest verdict
  subquery on `striatumd.verdicts`. Columns: `job_id, workflow_job_id,
  state, role_id, lane_selector_json, attempt, max_attempts, job_type,
  created_at, started_at, completed_at, latest verdict.verdict (subquery)`.
- **Return.** `{items: [{job_id, workflow_job_id, state, role_id,
  lane_id (parsed from lane_selector_json), attempt, max_attempts,
  job_type, created_at, started_at, completed_at,
  verdict (None for non-review)}], count: int}`.
- **Test.** `tests/daemon_pg/handlers/reads/test_list_jobs.py`. Parity
  with a fixture that seeds two attempts of a review job to verify
  latest-attempt ordering.
- **Errors.** `not_found` (missing run); `schema_invalid` (unknown
  state).

#### 3.6.4 `list.artifacts`

- **Legacy.** `list_commands.py:226-266`. Calls `expire_leases` and
  reuses `evidence_artifact_summaries` for author identity.
- **Handler.** `reads/list_artifacts.py::handle`.
- **Tables.** `striatumd.artifacts` — `artifact_id, job_id, session_id,
  logical_name, artifact_kind, repo_path, content_sha256, size_bytes,
  created_at, author_line` joined to per-job `workflow_snapshots` for
  the structured author identity.
- **Return.** `{items: [{artifact_id, job_id, session_id, logical_name,
  artifact_kind, repo_path, content_sha256, size_bytes, created_at,
  author}], count: int}`.
- **Test.** `tests/daemon_pg/handlers/reads/test_list_artifacts.py`.
  Parity covers the structured `author` block (same fields the SQLite
  path emits via `evidence_artifact_summaries`).
- **Errors.** `not_found` (missing run); `schema_invalid` (unknown
  kind).

#### 3.6.5 `list.workflows`

- **Legacy.** `list_commands.py:269-288`.
- **Handler.** `reads/list_workflows.py::handle`.
- **Tables.** `striatumd.workflow_snapshots` — `workflow_snapshot_id,
  workflow_id, workflow_version, source_path, content_sha256,
  loaded_at`.
- **Return.** `{items: [{workflow_snapshot_id, workflow_id,
  workflow_version, source_path, content_sha256, loaded_at}], count: int}`.
- **Test.** `tests/daemon_pg/handlers/reads/test_list_workflows.py`.
- **Errors.** None beyond the router-level repo scope check (no `run_id`
  filter; pure list).

### 3.7 `evidence.export`

- **Legacy.** `src/striatum/cli/evidence.py:356-426`.
- **Current PG state.** `src/striatum/daemon_pg/handlers/recovery_evidence/evidence_export.py`
  is registered but its `_status` and `_doctor` helpers are stubs that
  drop fields. This dogfood **replaces** that handler.
- **New handler.** `src/striatum/daemon_pg/handlers/reads/evidence_export.py::handle`.
  The Track B file is deleted; the `recovery_evidence/__init__.py`
  import line for `evidence_export` is removed; the
  `tests/daemon_pg/handlers/recovery_evidence/test_evidence_export.py`
  tests are migrated to `tests/daemon_pg/handlers/reads/test_evidence_export.py`.
- **Decorator + signature.** `@register_pg_handler("evidence.export", read_only=False)`.
- **Tables (read set).** Reuses `_read_model.status_pg`, `doctor_pg`,
  `evidence_snapshot_pg`. Net read set = union of § 3.1 + § 3.4 +
  artifacts/sessions/verdicts/blockers/blocked_downstream_jobs.
- **Tables (write set).** `striatumd.events` — `evidence.exported` event
  with `{path, sha256}`.
- **Return-shape parity contract.** `{status: "exported", run_id, path, sha256}`.
- **Reuse.** `redact_evidence_payload(...)` and
  `render_evidence_markdown(...)` from `striatum.cli.evidence` — already
  imported by reference in the existing Track B handler; keep.
- **Digest equality contract.** SHA-256 of UTF-8 markdown body bytes,
  computed by `_sha256_text` (already in the Track B file). The contract
  test
  `test_evidence_export_pg_digest_equality` (gated on `RFC0048_PARITY=1`)
  is preserved verbatim — it is the parity test for this method, and it
  becomes meaningful once `_status` and `_doctor` return the real
  shapes.
- **Test.** `tests/daemon_pg/handlers/reads/test_evidence_export.py`.
  Parity on the rendered markdown bytes AND the RPC return envelope.
- **Errors.** `not_found` (missing run); `schema_invalid` (missing
  `path`/`run_id`); `path_outside_scope` for `repo_relative_path`
  refusal.

### 3.8 `corpus.export`

- **Legacy.** `src/striatum/corpus/export.py:16-48 export_corpus_bundle()`.
  Walks git history since a commit ref, enumerates RFCs / decisions /
  operator reports / dogfood run summaries / changelog / ubiquitous
  language / friction patterns / commits, writes a deterministic JSONL
  bundle with a manifest. Reads the SQLite-backed `audit_chain` and
  `run_summary_snapshot()` (which transitively calls `status()` and
  `doctor()`).
- **New handler.** `src/striatum/daemon_pg/handlers/reads/corpus_export.py::handle`.
- **Decorator + signature.** `@register_pg_handler("corpus.export", read_only=False)`. Writes JSONL files + manifest; appends no audit event (legacy does not).
- **Tables (read set).** Limited PG involvement:
  - `striatumd.events` — for `enumerate_audit_chain` (sequenced events with payload + chain hashes).
  - All tables from § 3.1 + § 3.4 transitively, via the run-summary path that `enumerate_run_summaries` calls.
  - File-system reads for git history and docs (no substrate dependency).
- **Tables (write set).** None (filesystem only).
- **Return-shape parity contract.** Verbatim from
  `corpus/export.py::CorpusBundleResult.to_json(...)`:
  `{since_ref, since_commit, out, manifest, row_counts: {kind: count}, bundle_sha256}`.
- **Reuse.** All of `striatum.corpus.enumerator`, `striatum.corpus.git`,
  `striatum.corpus.redaction`, `striatum.corpus.types`,
  `striatum.corpus.writer`, `striatum.corpus.manifest` are pure modules
  — keep. The only refactor needed is **`enumerate_run_summaries` and
  `enumerate_audit_chain` take a `sqlite3.Connection` today**; widen
  them to accept a substrate-agnostic reader (a small `Reader` Protocol
  with `run_summary_snapshot(run_id) -> dict` and `audit_chain_rows()
  -> Iterator[Row]` methods). The PG handler implements the protocol
  using the new `_read_model` helpers; the SQLite path keeps working
  via a shim adapter that delegates to the existing functions. This is
  the only legacy-CLI signature change in this dogfood; without it
  `corpus.export` cannot share the SQLite contract.
- **Test.** `tests/daemon_pg/handlers/reads/test_corpus_export.py`.
  Parity on the manifest's `bundle_sha256` after seeding equivalent git
  history + audit events. Cross-substrate parity is a strong assertion
  here because the bundle's whole reason for existing is reproducible
  output.
- **Errors.** `schema_invalid` (missing `since` or `out` parameter);
  `path_outside_scope` (corpus output path must be inside repo per
  `_validate_out`); `git_ref_unknown` when `git_helpers.resolve_commit`
  raises (mirror as `RpcError` with the legacy message).

## 4. Cross-cutting infrastructure

### 4.1 `reads/_sql.py`

```python
from striatum.daemon_pg.handlers.recovery_evidence._sql import (  # re-export
    expire_leases, parse_json, utc_now, fetch_all, row_by_id,
)

def fetch_one(ctx, *, sql, params): ...     # convenience over fetch_all
def select_count(ctx, *, sql, params): ...   # for count(*) one-row reads
```

Re-exporting from `recovery_evidence/_sql.py` is deliberate: the helpers
are already exercised by 27 Track B tests and the same scope-injection
contract applies. We do **not** copy — copy-and-edit is forbidden by
F1 (single source of truth for the scope predicate).

If recovery_evidence's `_sql.py` is ever moved into a shared
`daemon_pg/handlers/_shared/` location, the re-export collapses to a
direct import. That move is **not** in scope for this dogfood — see
the next paragraph.

### 4.2 `reads/_read_model.py`

Houses the four shared read-models that multiple handlers consume:

| Function | Used by |
|---|---|
| `status_pg(ctx, run_id) -> dict` | `status`, `dashboard`, `run.summary`, `evidence.export` |
| `doctor_pg(ctx, repo, run_id, verbose=False) -> dict` | `doctor`, `run.summary`, `evidence.export` |
| `evidence_snapshot_pg(ctx, run_id) -> dict` | `evidence.export`, `run.summary` |
| `next_actions(...)` | imported from `striatum.cli.introspect` (not re-implemented) |

The status/doctor/snapshot triple is what the existing Track B
`evidence_export.py` had to stub out. Putting them in
`_read_model.py` lets all four consumers share one implementation and
makes the per-method tests parity-test the same code path.

### 4.3 Test rig (`tests/daemon_pg/handlers/reads/conftest.py`)

```python
@dataclass
class ReadSeed:
    """Minimal but rich fixture for read-handler parity."""
    repository_id: str
    repo_root: Path
    run_id: str
    workflow_snapshot_id: str
    job_ids: list[str]              # one queued, one blocked, one running
    session_ids: list[str]          # at least one with attestation
    artifact_ids: list[str]         # at least one published per kind we care about
    blocker_ids: list[str]          # one open blocker, one human checkpoint
    verdict_ids: list[str]          # one accept, one needs_revision
    expired_lease_ids: list[str]    # for stale-lease semantics

@pytest.fixture
def parity_seed(pg_ctx, sqlite_conn) -> ReadSeed: ...

def assert_payload_parity(pg, sqlite, *, ignore_keys=()) -> None: ...
```

The seed writes the same shape into both substrates so per-method tests
can do:

```python
def test_status_parity(pg_ctx, sqlite_conn, parity_seed):
    handler = import_handler("reads.status")
    pg_payload = handler.handle(pg_ctx, {"run_id": parity_seed.run_id, "repository_id": parity_seed.repository_id})
    sqlite_payload = striatum.cli.introspect.status(sqlite_conn, run_id=parity_seed.run_id)
    assert_payload_parity(pg_payload, sqlite_payload)
```

Reachability gate: `tests/_harness/pg.py::resolve_base_url()` (existing
pattern). When a PG URL is set, parity runs; otherwise per-method tests
fall back to the same registration + signature smoke checks Track B uses.
The `RFC0048_PARITY` env marker is only used to gate the legacy stubbed
parity tests under `recovery_evidence/`; the new reads tests use the
reachability gate alone (no opt-in env marker), so once a PG URL is in
CI the parity contract is enforced by default.

### 4.4 Integration with `DaemonRpcRouter._route`

No router edit. The router at
`src/striatum/daemon_rpc/server.py:230-246` already imports
`striatum.daemon_pg.handlers` (which transitively imports both
`workflow_loop` and `recovery_evidence`) and calls
`resolve_pg_handler(envelope.method)` before falling through to
`CLI_ROUTES` + `striatum.api.invoke`. Adding `from . import reads` to
`src/striatum/daemon_pg/handlers/__init__.py` is the only wiring step
needed — the `@register_pg_handler` decorators do the rest.

### 4.5 `read_only=True` decorator extension (proposed)

The current `register_pg_handler` signature
(`src/striatum/daemon_pg/handlers/registry.py:15-23`) takes only
`*methods: str`. This dogfood **proposes** widening the decorator to
accept an optional `read_only: bool = False` flag stored on the
registered handler. The flag is documentation-only (it does not change
routing) but lets the V1.5 follow-up F1 negative-test framework discover
read handlers and assert no SQL-write occurs in their code path.

If synthesis rejects this widening (single-source-of-truth concerns over
adding a flag the router does not consume), drop the flag and document
each handler with a module docstring `read_only: True` line that the
test framework greps for. Either is fine; the parity tests are the
contract.

## 5. Acceptance criteria

This dogfood lands when:

1. All 12 listed methods (8 RPC names) have PG handlers under
   `src/striatum/daemon_pg/handlers/reads/`.
2. Each method has a per-key parity test under
   `tests/daemon_pg/handlers/reads/test_<method>.py` that asserts
   byte-equivalent output vs the SQLite path on the shared `parity_seed`
   fixture.
3. The Track B `recovery_evidence/evidence_export.py` is deleted; tests
   migrated; the `__init__.py` import line removed.
4. `make lint` and `make typecheck` pass (`ruff check
   src/striatum/daemon_pg/handlers/reads tests/daemon_pg/handlers/reads`).
5. With a reachable PG URL: `pytest
   tests/daemon_pg/handlers/reads -q` returns all-green parity.
   Without one: registration + signature + error-mode smoke tests pass
   and parity tests skip with a clear reason.
6. Manual repro: on a repo where `striatum daemon migrate-repo-local`
   has run and `.striatum/state.sqlite3` is tombstoned, `striatum status
   --json`, `striatum dashboard --once`, `striatum list runs --json`,
   `striatum run summary <run-id> docs/...`, `striatum why <run-id>`,
   `striatum doctor --verbose --json`, `striatum evidence export
   <run-id> docs/...`, and `striatum corpus export --since HEAD~1 --out
   ...` all return real state instead of exit 3.
7. `docs/POSTGRES_TRANSITION.md` § "RFC 0048 remaining work" is updated
   to remove the read-surface gap from the list.

## 6. Risk register

- **R1: `doctor` size.** 23 checks at ~25 lines each is the largest
  port. Mitigation: per-check parity tests (one fixture per check)
  surface drift quickly; the `_DoctorRun` dataclass keeps the structure
  identical to the legacy local-`report` closure so reviewers can
  diff side-by-side.
- **R2: `corpus.export` substrate-agnostic Reader Protocol.** The only
  legacy-CLI signature change. Without it the corpus handler cannot
  share the SQLite contract. Risk is the existing SQLite tests need
  re-wiring to the shim adapter; mitigation is a one-line shim and
  zero behavior change on the SQLite path.
- **R3: Parity-test seeding cost.** The richer `ReadSeed` is more code
  than Track B's empty `Seed`. Mitigation: one seed populates one run
  with all the rows the eight handlers care about; per-method tests do
  not write fixtures of their own. Worst case the seed is ~150 lines.
- **R4: `next_actions` ordering drift.** Legacy `next_actions` reuses
  `dict.fromkeys(...)` to dedupe while preserving insertion order
  (`introspect.py:927`). The PG path must feed inputs in the same
  order. Mitigation: the parity test compares the full list and will
  fail loudly on an ordering drift.
- **R5: `process_health` includes filesystem/process-liveness checks
  (`_pid_alive`).** These are pure-Python over PIDs and do not depend
  on the substrate. Mitigation: import the helper directly; do not
  fork.

## 7. References

- `docs/rfcs/0048-daemon-side-substrate-migration.md` — V1 Phase A
  landing summary (L8-34) and V1.5 follow-up list (L36-84).
- `docs/dogfood/057/build/track_a/HANDOFF.md` — workflow-loop handler
  pattern (decorator self-registration, `RepoHandlerContext`,
  `append_event` chain semantics).
- `docs/dogfood/057/build/track_b/HANDOFF.md` — recovery + evidence
  handler pattern (`_sql.py` repo-scoping helpers, parity rig that this
  dogfood activates).
- `docs/POSTGRES_TRANSITION.md` § "RFC 0048 remaining work" — operator
  view of the read-surface gap this dogfood closes.
- `src/striatum/daemon_pg/handlers/registry.py:15-29` — the decorator
  contract.
- `src/striatum/daemon_pg/handlers/context.py:75-205` — the
  `RepoHandlerContext` shape this dogfood inherits.
- `src/striatum/daemon_rpc/server.py:215-264` — the router's PG
  resolution + SQLite fallback that picks up the new handlers
  automatically.
- `src/striatum/daemon_pg/handlers/recovery_evidence/_sql.py` — the
  helpers `reads/_sql.py` re-exports.

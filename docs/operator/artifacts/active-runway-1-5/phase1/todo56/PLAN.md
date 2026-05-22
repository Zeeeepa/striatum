---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/plans/active-runway-1-5.md", "docs/operator/artifacts/todo-55-56-59-60-decisions/final/SUMMARY.md", "docs/DECISION_LOG.md", "docs/TODO.md", "docs/operator/BRIEF.md", "src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py", "src/striatum/daemon_pg/handlers/recovery_evidence/sweep.py", "src/striatum/daemon_pg/handlers/reads/_read_model.py", "src/striatum/dashboard.py", "src/striatum/web/templates/_recovery_panel.html"]
---

# Plan: TODO 56 Auto-Finalize Follow-Up

author: auto-finalize-planner-claude-code-001

## Outcome

D125 keeps global auto-finalize as dry-run projection. Live auto-finalize
remains workflow opt-in. A default-on flip is gated on three successful
live dogfoods across at least two lane shapes with zero contested
audit-chain events.

This plan does not change those invariants. It scopes the bounded
follow-up work the decision listed as a precondition for any future
default-on reconsideration: lane-finalization visibility surfaces,
stable skipped-candidate cause classes, and a
consecutive-failure circuit breaker, plus the tests that hold the
dry-run-by-default and workflow-opt-in shape.

## Decision Boundary

The plan is bound by these invariants. None of the slices may relax
them.

- Global default stays `dry_run=true`. The CLI default in
  `recovery auto-finalize`, the resident sweep call site, and the
  status projection all stay dry-run unless the workflow explicitly
  opted in or `--force` is passed at the CLI.
- Workflow opt-in for live mode keeps its current shape:
  `recovery.auto_finalize.enabled=true` (with `auto_finalize.enabled`
  honored as legacy fallback). No new opt-in surface is introduced.
- The dry-run projection used by status/dashboard/web reads has no
  side effects: it writes nothing to PostgreSQL, emits no events, and
  changes no lease/session state.
- The auto-finalize evidence boundary stays "stable expected artifact
  files on disk plus daemon/PG lease and session state." Terminal
  output, transcripts, and provider hooks remain non-authoritative.
- The default-on flip remains gated. The circuit breaker and skipped
  cause classes do not unlock default-on; they make the live-opt-in
  envelope safer and the skip path queryable.

## Current Behavior Reference

The behavior this plan extends is already implemented at
`src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py`
and read into status at
`src/striatum/daemon_pg/handlers/reads/_read_model.py:88-104` and
`134-135`. The resident sweep wires it through
`src/striatum/daemon_pg/handlers/recovery_evidence/sweep.py:49-186`,
honoring workflow opt-in and never forwarding `--force`. The
terminal dashboard renders the projection in
`src/striatum/dashboard.py:591-641` and the web recovery panel
renders it in
`src/striatum/web/templates/_recovery_panel.html:13-52`. The CLI
verb lives at `src/striatum/cli/parser.py:1066-1109`.

The current skip path returns ad-hoc reason strings (for example
`"no active queue message is attached"`,
`"expected artifact file mtime is inside the grace period"`,
`"lane_evidence_missing: no clean process_executions row for the
session"`, `"artifact logical name already exists with different
content"`, `"live auto-finalize failed: <exc>"`). They are stable
enough to read by hand but not stable enough to query, count, or
trip a circuit breaker against.

The PG schema already records `artifact.auto_finalized` and
`job.auto_finalized` events with `lane_finalization=auto_from_artifact`
in the `job.auto_finalized` payload, but neither status nor dashboard
nor web surfaces "lane finalization" as a first-class field today;
they show eligibility/refusal counts only.

## Slice 1: Skipped-Candidate Cause Classes (Foundational)

Slice 1 has the smallest disjoint write scope and unblocks the other
slices. It must land first.

### Goal

Give each skipped/refused candidate a stable machine-readable cause
class alongside the human-readable reason. Stability lets the circuit
breaker count consecutive failures by cause and lets the visibility
surfaces group skips.

### Cause Class Enumeration

Add `striatum.recovery.auto_finalize_causes` as the named cause set.
The initial enumeration is derived from current refusal sites and is
intentionally finite.

Run-scope causes (returned by `_empty_result` or `_not_eligible`):

- `run_not_running` — `run.state != "running"`.
- `no_required_expected_artifacts` — job declared none.
- `no_active_message` — no queue message attached to the job lease.
- `expected_byline_unresolvable` — ``expected_author_line_pg`` raised.

Artifact-scope causes (returned by `_artifact_refusal`):

- `artifact_path_missing`
- `artifact_kind_invalid`
- `artifact_kind_transcript` (transcripts are never auto-finalized)
- `artifact_logical_name_missing`
- `artifact_path_outside_write_scope`
- `artifact_payload_path_invalid` (from `_artifact_payload_path`)
- `artifact_file_missing`
- `artifact_mtime_inside_grace`
- `artifact_read_failed`
- `artifact_front_matter_invalid` (front-matter schema or required
  field; covers `finding.verdict_intent` missing)
- `artifact_byline_missing`
- `artifact_byline_mismatch`
- `lane_evidence_missing` — no clean `process_executions` row, and
  byline is not `operator`, and `allow_no_process_execution=false`.
- `artifact_conflict_existing_content` — logical name already
  published with different content.

Finalize-time causes (returned when the live finalize path raises):

- `finalize_publish_failed`
- `finalize_complete_failed`
- `finalize_verdict_failed`
- `finalize_unexpected_error` — bucket for anything we did not
  enumerate yet; carries the exception class name in
  `cause_detail`.

Projection causes (returned from `dry_run_projection` only):

- `projection_evaluation_failed` — kept distinct because it does
  not represent a refusal of the candidate; only of the read.

### Schema Shape

Each skip entry adds:

- `cause` — one of the cause class names above.
- `cause_detail` — short string with the exception class name,
  artifact path, or value that triggered the class. Optional.

Existing keys (`workflow_job_id`, `job_id`, `reason`,
`artifacts[].reason`) remain unchanged so existing CLI/dashboard
readers keep working. The artifact-scope refusals add `cause` to
each artifact entry inside `artifacts[]`.

### Write Scope

- `src/striatum/recovery/auto_finalize_causes.py` (new module that
  exposes the enum and a small helper for `(cause, message)`
  pairs). Pure module; no PG access.
- `src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py`
  (consume the helpers; no change to handler signature or
  registration).
- `tests/recovery/test_auto_finalize_causes.py` (new) and the
  existing handler tests under
  `tests/daemon_pg/handlers/recovery_evidence/` updated to assert
  `cause` is present on each skip.

### Acceptance

- Every skip entry returned from `recovery.auto_finalize` and from
  the dry-run projection has a `cause` field whose value is in the
  documented enum.
- The unit tests pin one example per cause class.
- Reason strings remain backwards-compatible (existing callers that
  display `reason` keep working).

## Slice 2: Lane-Finalization Visibility (Status, Dashboard, Web)

### Goal

Surface "how this candidate would finalize" and "how previously
finalized jobs finalized" so operators can see, at a glance, that
auto-finalize is doing what it claims.

### Scope

For each eligible candidate and for each finalized job, surface a
`lane_finalization` field with values:

- `auto_from_artifact` — auto-finalize is the projected path for
  this candidate, and it is the recorded value on already-finalized
  jobs.
- `manual_publish` — the candidate has a real human/lane
  publication path open and is not currently auto-finalize
  eligible. Used in the projection only.
- `pending` — no eligibility resolved yet (run not running, no
  candidates).

The handler already writes `lane_finalization=auto_from_artifact`
into the `job.auto_finalized` event payload at
`auto_finalize.py:653`. The projection must populate the same field
on its returned `eligible[]` rows so the same vocabulary appears in
the dry-run shape.

### Surfaces

- Status (`status_payload`,
  `src/striatum/daemon_pg/handlers/reads/_read_model.py`): add
  `auto_finalize_dry_run.lane_finalization_summary` —
  `{auto_from_artifact: N, manual_publish: M, pending: K}` derived
  from the projection result. Keep `auto_finalize_dry_run` shape
  otherwise unchanged.
- Terminal dashboard (`_render_auto_finalize` in
  `src/striatum/dashboard.py`): show the summary on the header
  line and, per eligible row, the resolved `lane_finalization`
  value. Width-safe (use existing `_truncate`).
- Web recovery panel (`src/striatum/web/templates/_recovery_panel.html`):
  add a `lane_finalization` chip per eligible candidate and a
  summary tally in the existing `recovery-auto-finalize` callout.
  No island-level rework — keep the static template path.
- Run-level recent events: ensure
  `job.auto_finalized` events render with
  `lane_finalization=auto_from_artifact` in the dashboard recent
  events panel where event payload is already projected. (Read-only;
  no new event types.)

### Write Scope

- `src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py`
  (populate `lane_finalization` on `candidate` and `eligible[]`).
- `src/striatum/daemon_pg/handlers/reads/_read_model.py` (compute
  `lane_finalization_summary`).
- `src/striatum/dashboard.py` (render).
- `src/striatum/web/templates/_recovery_panel.html` (render).
- `tests/daemon_pg/handlers/reads/test_status_auto_finalize.py` and
  matching dashboard/web tests.

### Acceptance

- Status JSON for a run with mixed eligibility shows a non-empty
  `lane_finalization_summary` whose counts equal the candidate
  rows.
- A run with no candidates returns `lane_finalization_summary:
  {pending: 0}` and no error.
- The dashboard one-shot and web recovery panel both display the
  summary and per-row `lane_finalization` chips.

## Slice 3: Consecutive-Failure Circuit Breaker

### Goal

When live auto-finalize keeps failing for the same cause inside a
run, stop trying for a bounded window so operators can intervene
instead of seeing the same skip repeated by every sweep tick.

### Semantics

- **Trigger**: any cause from Slice 1 that resolves to the
  finalize-time class set (`finalize_publish_failed`,
  `finalize_complete_failed`, `finalize_verdict_failed`,
  `finalize_unexpected_error`).
- **Counted**: only live auto-finalize failures count. Dry-run
  refusals never trip the breaker.
- **Threshold**: 3 consecutive failures for the same
  `(run_id, workflow_job_id, cause)` triple within a 10-minute
  window. Both values are workflow-configurable under
  `recovery.auto_finalize.circuit_breaker.{max_consecutive,
  window_seconds}`. Defaults live in
  `src/striatum/recovery/auto_finalize_causes.py` next to the
  cause enum.
- **Effect when tripped**:
  - The next live auto-finalize attempt for the same
    `(workflow_job_id, cause)` is refused with cause
    `circuit_breaker_open` and the existing `policy` block in the
    response gains
    `circuit_breaker: {state: "open", until: <UTC iso>,
    cause: <triggering cause>, run_id, workflow_job_id}`.
  - The resident sweep does not retry until `until` passes.
  - Operators can override with the existing `--force` flag plus
    a new `--reset-circuit-breaker` boolean. Without
    `--reset-circuit-breaker`, `--force` continues to allow live
    mode but does not bypass an already-open breaker, so accidental
    `--force` does not erase the safety value.
- **Reset**:
  - Any successful finalize for the same `(workflow_job_id, cause)`
    closes the breaker.
  - Operator-driven `recovery auto-finalize --reset-circuit-breaker`
    closes it explicitly.
  - The window expiring closes it on the next eligibility read.

### Durable State

The breaker must survive daemon restart because the sweep is the
primary consumer and a restart inside the window would otherwise
reset the count. Durable state is required.

Use a new PG table owned by the daemon:

```text
striatumd.auto_finalize_circuit_breakers
  repository_id        text not null
  run_id               text not null
  workflow_job_id      text not null
  cause                text not null
  consecutive_failures int  not null default 0
  first_failed_at      timestamptz not null
  last_failed_at       timestamptz not null
  opened_at            timestamptz null
  opens_until          timestamptz null
  reset_at             timestamptz null
  primary key (repository_id, run_id, workflow_job_id, cause)
```

The table lives next to the other recovery-evidence tables and is
created by a new migration under
`src/striatum/daemon_pg/migrations/`. The migration is
forward-only; rollback is "drop and rebuild from successful
events" (the table is operational scratch, not provenance).

The breaker emits two new events into the existing event stream:

- `recovery.auto_finalize.circuit_breaker_opened`
- `recovery.auto_finalize.circuit_breaker_reset`

Both carry `(run_id, workflow_job_id, cause,
consecutive_failures, opens_until)` and are append-only.

### Write Scope

- `src/striatum/daemon_pg/migrations/<next-number>_auto_finalize_circuit_breakers.sql`
  (new migration).
- `src/striatum/recovery/auto_finalize_causes.py` (defaults).
- `src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py`
  (read/write the breaker; refuse-when-open at the per-candidate
  level; emit the two new events).
- `src/striatum/daemon_pg/handlers/reads/_read_model.py` (expose
  `auto_finalize_dry_run.circuit_breakers[]` — open breakers only).
- `src/striatum/cli/parser.py` (add
  `--reset-circuit-breaker` to `recovery auto-finalize`).
- `src/striatum/dashboard.py` and
  `src/striatum/web/templates/_recovery_panel.html` (render an
  open breaker with the cause that tripped it and the time until
  it closes).
- Tests: `tests/recovery/test_circuit_breaker.py` plus migration
  fixtures.

### Non-Scope

- No cross-run breaker — same job in a new run starts fresh.
- No automatic notification beyond the events.
- No change to how non-circuit-breaker skips behave; their cause
  classes from Slice 1 still appear in `skipped[]` and do not
  count toward the breaker.

### Acceptance

- Three consecutive `finalize_publish_failed` for the same
  `(run_id, workflow_job_id)` in a single sweep cycle open the
  breaker and the fourth call returns cause
  `circuit_breaker_open` without invoking publish.
- A successful finalize for the same job closes the breaker and
  emits `circuit_breaker_reset`.
- Daemon restart preserves an open breaker (table-backed state).
- `--reset-circuit-breaker` from the CLI emits the reset event
  and clears the row.

## Tests (Cross-Slice)

These are required for any default-on revisit and should land
alongside the implementation slices.

- **Dry-run default**:
  `tests/recovery/test_auto_finalize_default.py` — calling
  `recovery.auto_finalize` without `dry_run=false` always returns
  `dry_run: true`, never writes events, and never mutates
  artifacts/leases/jobs. Exercised against both Python and Go
  daemon paths through the existing handler test harness.
- **Live workflow opt-in**:
  - Live without `recovery.auto_finalize.enabled=true` and without
    `--force` raises `InvalidTransitionError` with the existing
    message.
  - Live with workflow opt-in finalizes eligible candidates.
  - Live with `--force` finalizes even without workflow opt-in
    (preserves the operator escape hatch).
- **Skipped causes**:
  - Parametric test that constructs one fixture per cause class
    from Slice 1 and asserts the returned `cause` matches.
  - Asserts no other cause string appears in `skipped[].cause`.
- **Circuit breaker**:
  - Open/close lifecycle (above).
  - Cross-restart persistence (the migration test runs the
    handler twice across a simulated restart and asserts the row
    survives).
  - `--reset-circuit-breaker` clears the row and emits the
    reset event.
- **Audit-chain quiet**:
  - The dry-run path runs with zero new audit rows.
  - The live path adds exactly the existing
    `artifact.auto_finalized` and `job.auto_finalized` rows, plus
    the new circuit-breaker events when applicable.
- **Visibility surfaces**:
  - Status JSON contains `lane_finalization_summary` and
    optional `circuit_breakers[]`.
  - Dashboard one-shot output is width-safe and renders both.
  - Web template renders without JS island regressions.

These tests are the runtime piece of the D125 evidence gate; the
"three live dogfoods across two lane shapes" gate is operator work
on top of the runtime tests, not in place of them.

## First Implementation Slice

The smallest disjoint write scope is **Slice 1** (skipped-candidate
cause classes). It is foundational because the circuit breaker
keys on cause names and the visibility surfaces want them.
Recommended order:

1. **Slice 1** — cause-class module, handler integration, tests.
   Write scope:
   `src/striatum/recovery/auto_finalize_causes.py` (new),
   `src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py`,
   `tests/recovery/test_auto_finalize_causes.py`, plus handler-test
   updates. No migration. No new RPC. No CLI change.
2. **Slice 2** — visibility surfaces. Disjoint from Slice 1 except
   for the small projection edit in `auto_finalize.py` that adds
   `lane_finalization` to the eligible rows.
3. **Slice 3** — circuit breaker with migration. Largest write
   scope; lands last so it can rely on Slices 1-2 already being
   present.

## Open Questions Punted to Implementation

- Whether `lane_finalization` should also carry the
  display-model name (e.g. `auto_from_artifact (claude_code)`).
  Default is "no" until UI feedback says otherwise; the projection
  already exposes the session/lane via existing fields.
- Whether the circuit breaker should also fire for repeated
  `lane_evidence_missing` skips. Default is "no" because that
  cause is operator-recoverable; revisit if dogfood evidence shows
  it spinning.
- Whether the cause enum belongs in
  `src/striatum/recovery/auto_finalize_causes.py` or in
  `src/striatum/artifact_contracts.py`. Default is the
  recovery-local module to keep `artifact_contracts.py` focused
  on artifact schemas.

## Out of Scope

- Flipping the global default to `live`. D125 forbids it until the
  dogfood gate is met.
- Adding non-artifact evidence (terminal output, transcripts,
  provider hooks).
- Cross-repository circuit breakers.
- Hosted notification of breaker events.
- Engram-specific instrumentation or examples.

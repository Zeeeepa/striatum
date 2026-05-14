# RFC 0048 Phase A Handler Port Design

date: 2026-05-14
status: handoff
author: designer-unknown-model-001

## Scope

Design Phase A only: move the selected single-repo daemon RPC routes off
`DaemonRpcRouter._route -> striatum.api.invoke -> striatum.db.connect ->
SQLite` and onto daemon-owned PostgreSQL tables in `striatumd.*`.

Out of scope:

- RFC 0048 Phase B Go core parity.
- RFC 0048 Phase C SQLite removal / fallback deletion.
- The Unix-socket accept-loop gap in `src/striatum/daemon.py`.
- Product semantics changes. Surface behavior must stay byte-compatible with
  the current CLI handlers except where PostgreSQL concurrency requires
  stronger locking.

## Current Delegation To Replace

The current Python daemon RPC stack already has the right outer boundary:

- Method registry: `src/striatum/daemon_rpc/registry.py`, especially
  `METHOD_REGISTRY` and canonical dotted methods such as
  `session.register`, `work.claim_next`, `work.ack`, `work.complete`,
  `review.submit`, `recovery.stale_leases`, and `evidence.export`.
- Router: `src/striatum/daemon_rpc/server.py::DaemonRpcRouter.handle`.
  Authorization happens before dispatch, then `_record_and_return` appends
  one daemon audit row and one request-log row.
- Audit/request log anchors:
  `src/striatum/daemon_rpc/request_log.py::append_audit_row` writes
  `striatumd.audit_log`, updates `striatumd.audit_chain_head`, and updates
  `striatumd.audit_segments`; `append_request_log` writes
  `striatumd.rpc_request_log`.

The part to replace is `DaemonRpcRouter._route`: after inline families, it
looks up `CLI_ROUTES`, builds argv, and calls `striatum.api.invoke`. For
Phase A methods, `_route` should call PG handlers directly and never invoke
`striatum.api.invoke`.

## Handler Layout

Add PG-backed modules under `src/striatum/daemon_pg/handlers/`:

- `context.py`: repository-scoped execution context, row helpers, JSON
  conversion, ID/time helpers, and event insertion.
- `workflow_loop.py`: Track A handlers.
- `recovery.py`: Track B recovery handlers.
- `evidence.py`: Track B evidence export handler.
- `router.py`: method-to-handler map used by `DaemonRpcRouter._route`.

Suggested handler signature:

```python
def handler(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]:
    ...
```

`RepoHandlerContext` should carry `pg_conn`, `repository_id`, `repo_root`,
and the authorized `RpcAuthContext`. Every SQL statement against repo-local
workflow tables must include `repository_id = %s`.

Use a small transaction helper that starts a short PostgreSQL transaction.
For mutating handlers, use `SERIALIZABLE` where the current SQLite code
relies on process-wide single-writer behavior. Claim paths should use
`FOR UPDATE SKIP LOCKED` on `queue_messages` / `jobs` / `leases` to avoid
double claims.

## Router Swap

In `src/striatum/daemon_rpc/server.py`:

1. Remove Phase A methods from the generic `CLI_ROUTES` fallback as they
   land.
2. Before the fallback, call a PG handler resolver:

```python
handler = resolve_repo_handler(envelope.method)
if handler is not None:
    if self.pg_conn is None:
        raise RpcError("daemon_db_missing", "repo handler requires daemon PostgreSQL")
    repository_id = _repository_id(envelope.params)
    if repository_id is None:
        raise RpcError("repo_not_registered", "daemon RPC route requires repository_id")
    ctx = RepoHandlerContext(
        conn=self.pg_conn,
        repository_id=repository_id,
        repo_root=repo_root,
        auth=auth,
    )
    return handler(ctx, envelope.params)
```

3. Keep `striatum.api.invoke` fallback only for methods not yet ported.

Do not move daemon RPC audit into each business handler. The outer
`_record_and_return` call remains the request-level audit anchor for every
allowed/denied RPC call. Business handlers append workflow events to
`striatumd.events` where the current SQLite code calls `insert_event`.

## Shared Porting Rules

- Preserve current JSON response shapes.
- Preserve current error semantics by mapping `InvalidTransitionError`,
  `LeaseError`, `ArtifactError`, and `NotFoundError` to the same CLI-style
  error envelopes used by RPC today.
- Keep `striatumd.events` append-only. Do not update/delete events.
- Do not update/delete `striatumd.artifacts`; artifact replacement remains
  an append/versioning product decision, not a Phase A change.
- Any helper currently reading `rowid` for tie-breaking must get a PG-safe
  replacement. For `override_review_verdict`, order previous verdicts by
  `created_at DESC, verdict_id DESC` or add a better stable ordering before
  the handler lands.
- `0005_repo_local_workflow_state.sql` is the Phase A schema reference:
  `workflow_snapshots`, `runs`, `sessions`, `jobs`, `job_dependencies`,
  `queue_messages`, `leases`, `work_packets`, `artifacts`, `verdicts`,
  `blockers`, `command_requests`, `process_executions`, `events`,
  `job_worktrees`, `process_supervisors`, `process_supervisor_pointers`,
  and `repo_migrations`.

## Track A: Workflow Loop Handlers

### `session.register`

Current entry point: `src/striatum/cli/mutations.py::register_session`.

PG handler: `daemon_pg.handlers.workflow_loop.register_session`.

Tables:

- Read: `runs`, `workflow_snapshots`, `sessions`, optionally
  `process_supervisors` / `process_supervisor_pointers` for lane
  attestation.
- Write: `sessions`, `events`.

Event anchor: append `session.registered`.

Notes:

- Validate role and lane from `workflow_snapshots.workflow_json`.
- Preserve fresh-reviewer refusal / override behavior.
- Generate `(session_id, slug, ordinal)` under a transaction. Lock existing
  session rows for `(repository_id, run_id, role_id, lane_id)` before
  computing `ordinal`.

Tests:

- Port existing `tests/test_cli_mvp.py::test_register_session_rejects_unknown_role_or_lane`.
- Port `tests/test_cli_mvp.py::test_register_session_surfaces_unattested_operator_identity`.
- Port `tests/test_cli_mvp.py::test_register_session_rejects_deceptive_operator_labels`.
- Add `tests/daemon_pg/test_handlers_workflow_loop.py::test_session_register_writes_pg_session_and_event`.

### `work.claim_next`

Current entry point: `src/striatum/db.py::claim_next`.

PG handler: `daemon_pg.handlers.workflow_loop.claim_next`.

Tables:

- Read: `sessions`, `runs`, `queue_messages`, `jobs`,
  `workflow_snapshots`, `work_packets`, `process_supervisors`,
  `process_supervisor_pointers`.
- Write: `leases`, `queue_messages`, `jobs`, `work_packets`, `events`.

Event anchors: append `lease.expired` / `worktree.abandoned` when lazy expiry
runs, then append `queue.claimed`.

Notes:

- Replace the pending-work SELECT with a `FOR UPDATE SKIP LOCKED` claim query
  over `queue_messages` joined to `jobs`.
- Preserve fresh-session filtering against `work_packets`.
- Preserve packet JSON shape from `build_packet`; either port the packet
  builder to PG or create a thin row-adapter layer that makes PG rows satisfy
  the existing builder until it can be retired.
- Supervisor delivery is a side effect today. For Phase A, either keep the
  existing local supervisor writer behind a PG-aware adapter or explicitly
  defer auto-delivery for unsupervised claims only. Do not call SQLite.

Tests:

- Port `tests/test_cli_mvp.py::test_branch_confirmation_blocks_claims`.
- Port `tests/test_cli_mvp.py::test_claim_next_filters_fresh_session_required_in_sql`.
- Port `tests/test_harness_profiles.py::test_packet_exposes_harness_profile_for_referenced_lane`.
- Add a PG concurrency test: two sessions claiming the same role get distinct
  jobs or one gets `no_work`, never the same lease.

### `work.ack`

Current entry point: `src/striatum/cli/mutations.py::ack_work`.

PG handler: `daemon_pg.handlers.workflow_loop.ack_work`.

Tables:

- Read: `queue_messages`, `jobs`, `leases`.
- Write: `queue_messages`, `jobs`, `events`.

Event anchor: append `queue.acked`.

Tests:

- Port `tests/test_cli_mvp.py::test_complete_requires_ack`.
- Add idempotent ack coverage for already-acked messages.

### `work.complete`

Current entry point: `src/striatum/db.py::complete_job`.

PG handler: `daemon_pg.handlers.workflow_loop.complete_job`.

Tables:

- Read: `jobs`, `leases`, `artifacts`, `job_dependencies`,
  `queue_messages`, `runs`, `sessions`.
- Write: `jobs`, `queue_messages`, `leases`, `queue_messages` for
  downstream enqueue, `runs`, `sessions`, `events`.

Event anchors: append `job.completed`; downstream helper may append
`queue.message_enqueued`; run-terminal helper may append `run.completed`,
`run.failed`, `run.canceled`, and `session.closed`.

Tests:

- Port `tests/test_cli_mvp.py::test_artifact_completion_and_verdict_flow`.
- Port `tests/test_cli_mvp.py::test_complete_requires_expected_artifact_path_and_kind`.
- Add `tests/daemon_pg/test_handlers_workflow_loop.py::test_complete_closes_run_and_sessions_in_pg`.

### `work.release`

Current entry point: `src/striatum/cli/mutations.py::release_work`.

PG handler: `daemon_pg.handlers.workflow_loop.release_lease`.

Tables:

- Read: `queue_messages`, `jobs`, `leases`.
- Write: `leases`, `jobs`, `queue_messages`, `events`.

Event anchor: append `lease.released`.

Tests:

- Port `tests/test_cli_mvp.py::test_release_requeues_fresh_review_for_new_session_only`.

### `work.block`

Current entry point: `src/striatum/cli/mutations.py::block_work`.

PG handler: `daemon_pg.handlers.workflow_loop.block_job`.

Tables:

- Read: `jobs`, `leases`.
- Write: `blockers`, `jobs`, `leases`, `queue_messages`, `events`.

Event anchor: append `job.blocked`.

Tests:

- Port the block path from `tests/test_recovery_extended.py::_open_checkpoint`.
- Port `tests/test_cli_mvp.py::test_blocked_review_verdict_appears_in_status`.

### `review.verdict`

Current entry points:

- CLI wrapper: `src/striatum/cli/mutations.py::verdict_work`.
- Core review logic: `src/striatum/db.py::record_review_verdict`.

PG handler: `daemon_pg.handlers.workflow_loop.record_verdict`.

Tables:

- Read: `jobs`, `leases`, `artifacts`, `workflow_snapshots`,
  `process_supervisors`, `process_supervisor_pointers`,
  `job_dependencies`, `queue_messages`, `runs`, `sessions`.
- Write: `verdicts`, `jobs`, `leases`, `queue_messages`, `blockers` for
  revision/human-checkpoint routing, downstream `queue_messages`, `runs`,
  `sessions`, `events`.

Event anchors: append `verdict.recorded`; accept paths append
`job.completed`; reject paths append `job.failed`; needs-revision paths
append existing revision/human-checkpoint events.

Tests:

- Port `tests/test_cli_mvp.py::test_verdict_reject_fails_run_and_does_not_enqueue_downstream`.
- Port `tests/test_cli_mvp.py::test_accepting_review_verdict_unblocks_downstream`.
- Port `tests/test_cli_mvp.py::test_verdict_needs_revision_parallel_reviewers_share_cycle_target`.
- Port `tests/test_cli_mvp.py::test_verdict_needs_revision_uses_declared_cycle`.
- Port `tests/test_cli_mvp.py::test_verdict_needs_revision_without_cycle_waits_human`.
- Port `tests/test_cli_mvp.py::test_verdict_requires_expected_artifact_path_and_kind`.

### `review.submit`

Current entry point: `src/striatum/cli/mutations.py::submit_review`.

PG handler: `daemon_pg.handlers.workflow_loop.submit_review`.

Tables:

- Read: everything from `work.ack`, `artifact.publish`, and
  `review.verdict`.
- Write: `queue_messages`, `jobs`, `artifacts`, `verdicts`, `leases`,
  `blockers`, `runs`, `sessions`, `events`.

Event anchors: append `queue.acked` when auto-acking a claimed review,
`artifact.published`, optional
`provenance.publish_without_process_execution`, then `verdict.recorded`
and downstream/run-terminal events.

Notes:

- This handler depends on a PG port of artifact publication validation.
  It can land after `artifact.publish` is PG-backed or carry a private
  PG artifact publisher used only by `review.submit`.

Tests:

- Port `tests/test_cli_mvp.py::test_submit_review_publishes_artifact_and_applies_gate`.
- Port `tests/test_cli_mvp.py::test_submit_review_prevalidates_before_publishing_artifact`.
- Port `tests/test_cli_mvp.py::test_submit_review_rejects_non_review_before_publishing_artifact`.
- Port RFC 0046 override coverage from `tests/test_lane_evidence_guard.py`.

### `review.override`

Current entry point: `src/striatum/db.py::override_review_verdict`, invoked
by `override-verdict` in `src/striatum/cli/dispatch.py`.

PG handler: `daemon_pg.handlers.workflow_loop.override_review_verdict`.

Tables:

- Read: `jobs`, `sessions`, `verdicts`, `artifacts`,
  `workflow_snapshots`, `job_dependencies`, `runs`.
- Write: `verdicts`, `jobs`, `queue_messages`, `blockers`, downstream
  `queue_messages`, `runs`, `sessions`, `events`.

Event anchor: append `verdict.overridden`.

Notes:

- Replace SQLite `rowid` ordering with a PG-safe stable ordering.

Tests:

- Port `tests/test_cli_mvp.py::test_override_verdict_accepts_completed_needs_revision_with_findings`.
- Port `tests/test_cli_mvp.py::test_override_verdict_accepts_already_completed_needs_revision_review`.
- Keep web-context validation in `tests/test_override_modal_context_validation.py` as service/UI coverage.

## Track B: Recovery Handlers

### `recovery.stale_leases`

Current entry point: `src/striatum/cli/recovery.py::stale_leases`.

PG handler: `daemon_pg.handlers.recovery.stale_leases`.

Tables:

- Read: `runs`, `jobs`, `leases`, `queue_messages`, `job_worktrees`.
- Write: `leases`, `jobs`, `job_worktrees`, `events` only because the
  existing handler calls lazy `expire_leases`.

Event anchors: append `lease.expired`; append `worktree.abandoned` when
expiry abandons active worktrees.

Tests:

- Port `tests/test_cli_mvp.py::test_recovery_stale_leases_reports_repo_write_policy`.

### `recovery.requeue_stale`

Current entry point: `src/striatum/cli/recovery.py::requeue_stale`.

PG handler: `daemon_pg.handlers.recovery.requeue_stale`.

Tables:

- Read: `runs`, `jobs`, `leases`, `queue_messages`.
- Write: `leases`, `jobs`, `queue_messages`, `events`.

Event anchors: lazy expiry events, then `recovery.stale_requeued`.

Tests:

- Port `tests/test_cli_mvp.py::test_recovery_requeue_stale_rejects_repo_write_jobs`.
- Port `tests/test_cli_mvp.py::test_recovery_requeue_stale_allows_review_only_jobs`.

### `recovery.cancel_job`

Current entry point: `src/striatum/cli/recovery.py::cancel_job`.

PG handler: `daemon_pg.handlers.recovery.cancel_job`.

Tables:

- Read: `runs`, `jobs`, `job_dependencies`, `leases`,
  `queue_messages`, `sessions`.
- Write: `leases`, `queue_messages`, `jobs`, `runs`, `sessions`,
  `events`.

Event anchors: append one `job.canceled` per canceled job; run-terminal
helper may append `run.canceled` and `session.closed`.

Tests:

- Port `tests/test_recovery_extended.py::test_recovery_cancel_job_blocks_when_dependents_exist_without_cascade`.
- Port `tests/test_recovery_extended.py::test_recovery_cancel_job_with_cascade_cancels_dependents`.
- Port `tests/test_recovery_extended.py::test_recovery_cancel_job_refuses_terminal_states`.

### `recovery.process_reconcile`

Current entry point: `src/striatum/cli/recovery.py::process_reconcile`.

PG handler: `daemon_pg.handlers.recovery.process_reconcile`.

Tables:

- Read: `runs`, `process_executions`, `jobs`, `artifacts`, `leases`,
  `queue_messages`.
- Write: `process_executions`, `blockers`, `jobs`, `leases`,
  `queue_messages`, `events`.

Event anchors: append `process.lost`; if required outputs are missing,
append `process_adapter.outputs_missing` and `job.blocked`-equivalent
process-completion events.

Tests:

- Port process-reconcile coverage from `tests/test_recovery_resume.py`.
- Add `tests/daemon_pg/test_handlers_recovery.py::test_process_reconcile_marks_missing_pid_lost_in_pg`.

### `recovery.resume`

Current entry point: `src/striatum/cli/recovery.py::resume_blocker`.

PG handler: `daemon_pg.handlers.recovery.resume_blocker`.

Tables:

- Read: `blockers`, `jobs`, `leases`, `artifacts`.
- Write: `blockers`, `jobs`, `leases`, optional `queue_messages`,
  optional `runs`, optional `sessions`, `events`.

Event anchors: append `recovery.blocker_dismissed_terminal` for forced
terminal no-op; append `recovery.process_blocker_resolved` for normal
resume; inline completion appends `job.completed` and downstream/run events.

Tests:

- Port `tests/test_recovery_resume.py::test_recovery_resume_complete_finishes_remediated_job`.
- Port `tests/test_recovery_resume.py::test_recovery_resume_exit_blocker_requires_force`.
- Port `tests/test_recovery_resume.py::test_recovery_resume_review_blocker_allows_accept_with_findings`.
- Port GH-14 coverage in `tests/test_gh14_terminal_blocker_recovery.py`.

### `recovery.auto`

Current requested behavior: `auto_publish_stale_artifacts` from
`src/striatum/cli/recovery.py`.

Registry note: current registry contains `recovery.auto` and
`recovery.watch`; the router has no `recovery.auto_publish` method even
though CLI dispatch exposes `recovery auto-publish`. Phase A should either:

- add canonical method `recovery.auto_publish_stale_artifacts`, capability
  `recovery`, single-repo scope; or
- explicitly route `recovery.auto` to only the broader automatic sweep and
  defer auto-publish as not registry-addressable.

Recommended: add `recovery.auto_publish_stale_artifacts` because RFC 0048
names this handler explicitly.

Tables:

- Dry run read-only: `runs`, `jobs`, `leases`, `queue_messages`.
- Live path reads/writes: `leases`, `jobs`, `queue_messages`,
  `work_packets`, `artifacts`, `runs`, `sessions`, `events`.

Event anchors: lazy expiry events; `artifact.published`; optional
`job.completed`; `recovery.auto_published`.

Tests:

- Port `tests/test_recovery_dry_run_no_side_effects.py::test_dry_run_via_direct_call_writes_nothing_when_lease_wall_clock_stale`.
- Add PG live-path coverage in `tests/daemon_pg/test_handlers_recovery.py`.

## Track B: Evidence Export

### `evidence.export`

Current entry point: `src/striatum/cli/evidence.py::evidence_export`.

PG handler: `daemon_pg.handlers.evidence.evidence_export`.

Tables:

- Read: `runs`, `workflow_snapshots`, `jobs`, `artifacts`, `sessions`,
  `verdicts`, `blockers`, `job_dependencies`, `process_supervisors`,
  `process_supervisor_pointers`.
- Write: `events`.
- Filesystem write: the redacted Markdown export path under `repo_root`.

Event anchor: append `evidence.exported`.

Notes:

- Port `evidence_snapshot`, `evidence_job_summaries`,
  `evidence_artifact_summaries`, and `evidence_session_summaries` to
  repository-scoped PG queries.
- Preserve redaction policy exactly; this is a read/export behavior change
  only in substrate.

Tests:

- Port `tests/test_cli_mvp.py::test_evidence_redaction_drops_unknown_fields_by_default`.
- Port `tests/test_cli_mvp.py::test_evidence_redaction_preserves_safe_fields`.
- Port `tests/test_cli_mvp.py::test_evidence_redacts_workflow_job_titles`.
- Port `tests/test_cli_mvp.py::test_evidence_export_writes_redacted_markdown_and_rejects_bad_paths`.

## Audit Chain Requirements

There are two distinct chains to preserve:

- RPC request audit: `DaemonRpcRouter._record_and_return` must remain the
  only request-level audit writer. It calls `append_audit_row`, which writes
  `striatumd.audit_log`, updates `striatumd.audit_chain_head`, and updates
  the open `striatumd.audit_segments` row. Then it calls
  `append_request_log` to write `striatumd.rpc_request_log`.
- Workflow event provenance: each business transition must append the same
  `striatumd.events.event_type` currently emitted by SQLite handlers. These
  rows are append-only under `0005_repo_local_workflow_state.sql`.

Acceptance for each method should assert both:

- one RPC audit row + request-log row for the method; and
- the expected workflow event rows under the same `repository_id`.

Use existing helpers:

- `tests/_harness/audit.py::assert_audit_chain`.
- `tests/_harness/multi_repo.py::MultiRepoHarness.assert_audit_chain`.

## Test Plan

Add Phase A PG handler tests under:

- `tests/daemon_pg/test_handlers_workflow_loop.py`
- `tests/daemon_pg/test_handlers_recovery.py`
- `tests/daemon_pg/test_handlers_evidence.py`
- `tests/daemon_rpc/test_phase_a_pg_routing.py`

Keep and extend:

- `tests/daemon_rpc/test_registry_rfc0043_coverage.py`: add
  `recovery.auto_publish_stale_artifacts` if accepted.
- `tests/test_daemon_rpc.py`: ensure routed PG handlers still audit denials
  and allowed responses through `_record_and_return`.
- `tests/test_daemon_pg.py`: append-only role/trigger coverage for
  `events` and `artifacts`.
- `tests/daemon_pg/test_repo_local_migration.py`: source-of-truth fixture
  for schema/table compatibility.

For each ported method:

1. Build or migrate a small run into PG with `tests/_harness/pg.py` /
   `tests/_harness/multi_repo.py`.
2. Call the canonical RPC method through `DaemonRpcRouter.handle`, not the
   CLI.
3. Assert no `striatum.api.invoke` call. A monkeypatch that fails if invoked
   should be in `test_phase_a_pg_routing.py`.
4. Assert response compatibility with the SQLite handler fixture.
5. Assert changed PG rows and expected `events`.
6. Assert daemon audit/request-log rows and audit chain validity.

## Landing Order

1. Add shared `daemon_pg.handlers.context` and route resolver with no methods
   registered.
2. Port `session.register`; smallest write surface and validates context.
3. Port `work.ack`, `work.release`, `work.block`; simple lease/message/job
   transitions.
4. Port `work.claim_next`; adds concurrency-sensitive claim locking and packet
   construction.
5. Port `work.complete`; adds downstream/run-terminal helpers.
6. Port `review.verdict`, `review.submit`, `review.override`; highest
   workflow-loop semantic complexity.
7. Port `recovery.stale_leases`, `recovery.requeue_stale`,
   `recovery.cancel_job`.
8. Port `recovery.process_reconcile`, `recovery.resume`,
   `recovery.auto_publish_stale_artifacts`.
9. Port `evidence.export`.

Each step removes that method from fallback routing and adds a regression test
that fails if `_route` reaches `striatum.api.invoke`.

## Open Implementation Risks

- `striatumd.artifacts` in `0005_repo_local_workflow_state.sql` does not show
  the current SQLite `attestation_override_rationale` column used by
  `publish_artifact`. Before `review.submit` or any PG artifact publisher
  lands, reconcile the PG schema with the latest SQLite migration surface.
- The current code has helper logic spread across `src/striatum/db.py`,
  `src/striatum/artifacts.py`, `src/striatum/process_completion.py`, and
  `src/striatum/process_adapter.py`. Avoid importing those helpers if they
  open SQLite or assume unscoped table names; port only the invariant logic.
- `claim_next` supervisor delivery currently writes to local FIFOs from inside
  the claim path. Keep the DB transaction short; do not hold PG locks while
  blocking on filesystem I/O.
- Evidence export writes a repository file. Keep database reads inside a
  transaction only long enough to build the redacted snapshot, then write the
  file and append `evidence.exported` in a short follow-up transaction.

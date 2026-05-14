---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/057/design/codex/DESIGN.md", "docs/dogfood/057/design/claude_code/DESIGN.md", "docs/dogfood/057/design/gemini/DESIGN.md"]
---
author: designer-unknown-model-001

# RFC 0048 Phase A Scope Lock

## Decision

Phase A ports the selected daemon RPC methods to native Postgres handlers under `src/striatum/daemon_pg/handlers/` and leaves unported methods on the existing `DaemonRpcRouter._route -> striatum.api.invoke` fallback. The source behavior remains the current CLI/DB behavior; the implementation boundary changes from SQLite-backed helper calls to repository-scoped `striatumd.*` SQL in one serializable transaction per method.

## Cross-Cutting Locks

- Handler module layout: use per-method files under `src/striatum/daemon_pg/handlers/workflow_loop/` and `src/striatum/daemon_pg/handlers/recovery_evidence/`; one file per method matches RFC 0048's one-method-per-commit rollout and keeps Track A/Track B write ownership disjoint.
- Handler signature: `def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]`; `RepoHandlerContext` carries `pg_conn`, `repository_id`, `repo_root`, `auth`, `now()`, `new_id()`, and `append_event()`, which avoids widening every handler when transaction, audit, or auth details change.
- Handler registration: decorator-based self-registration, `@register_pg_handler("work.ack")`, in `src/striatum/daemon_pg/handlers/registry.py`; `src/striatum/daemon_pg/handlers/__init__.py` imports both subpackages once, so Track B contributes handlers by editing only files under `recovery_evidence/`.
- Repository scope enforcement: every helper requires `ctx.repository_id`, every SQL statement includes `repository_id = %(repository_id)s` in reads and writes, and tests assert no handler SQL touches repo-local tables without the repository predicate; the schema's `repository_id NOT NULL` FKs remain the final guard.
- Half-ported routing: `DaemonRpcRouter._route` checks `resolve_pg_handler(envelope.method)` before `CLI_ROUTES`; if a handler is registered and `pg_conn` is present it runs PG-native, otherwise the method falls through to the legacy SQLite-backed route during Phase A.
- PG fixture: reuse `tests/_harness/pg.py`; add `tests/daemon_pg/handlers/conftest.py` to build equivalent SQLite and PG states for parity cases.
- Transaction shape: every mutating handler opens one `SERIALIZABLE` transaction, locks the specific run/job/message/lease rows it mutates with `FOR UPDATE`, uses `FOR UPDATE SKIP LOCKED` only for dequeue selection, and retries one serialization failure at the router boundary.
- Audit-chain anchor: add a Phase A migration for `striatumd.events.previous_hash`, `striatumd.events.row_hash`, and `striatumd.repo_event_chain_heads(repository_id, last_event_id, last_row_hash, updated_at)`; `ctx.append_event()` locks the per-repo head row `FOR UPDATE`, inserts the event with `previous_hash = last_row_hash`, computes `row_hash` over the canonical event payload, and updates the head before commit. If a method appends multiple events, each event chains from the previous event inserted in the same transaction.

The request-level daemon audit remains in `striatumd.audit_log`; business-state provenance for these ports is the repo-local `striatumd.events` chain.

## Track A: Workflow-Loop Handlers

### `session.register`

- Source: `src/striatum/cli/mutations.py::register_session`, lines 226-353.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/register_session.py::handle`.
- Read tables: `runs(run_id, workflow_snapshot_id, state)`, `workflow_snapshots(workflow_json)`, `sessions(run_id, role_id, lane_id, state, ordinal)`, `process_supervisors`, `process_supervisor_pointers` for attestation.
- Write tables: `sessions(session_id, run_id, role_id, lane_id, slug, ordinal, capabilities_json, parent_session_id, fresh_context, state, registered_at, last_heartbeat_at, non_fresh_reason, operator_label)`, `events`.
- Transaction and locks: serializable; lock the run row and existing `(run_id, role_id, lane_id)` session rows `FOR UPDATE` before computing `ordinal`.
- Event anchor: append `session.registered` with `actor_session_id = session_id` and payload `{role, lane, slug, non_fresh_reason?, operator_label?}` chained from the repo event head.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_register_session.py`.

### `work.claim_next`

- Source: `src/striatum/db.py::claim_next`, lines 993-1127; RPC entry currently maps through `CLI_ROUTES["work.claim_next"]` in `src/striatum/daemon_rpc/server.py`, lines 42-44.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/claim_next.py::handle`.
- Read tables: `sessions(session_id, run_id, role_id, lane_id, state)`, `runs(state, paused_at)`, `queue_messages(kind, state, target_role_id, target_lane_id, priority, created_at)`, `jobs(fresh_session_required, lane_selector_json, write_scope_json, expected_artifacts_json)`, `workflow_snapshots(workflow_json)`, `work_packets(session_id)`, supervisor tables for packet delivery.
- Write tables: `leases`, `queue_messages(state, claimed_at, updated_at, current_lease_id, claim_count)`, `jobs(state, current_lease_id, started_at)`, `work_packets(packet_json, packet_sha256)`, `events`; lazy expiry may also update `leases`, `jobs`, `queue_messages`, and `job_worktrees`.
- Transaction and locks: serializable; lock the session and run rows; select one pending work message with `FOR UPDATE SKIP LOCKED` joined to its job; lock the chosen job before inserting lease and packet rows.
- Event anchor: lazy expiry events chain first when emitted, then `queue.claimed` with `actor_session_id`, `job_id`, `message_id`, and `lease_id`.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_claim_next.py`.

### `work.ack`

- Source: `src/striatum/cli/mutations.py::ack_work`, lines 462-487.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/ack_work.py::handle`.
- Read tables: `queue_messages(message_id, job_id, state, current_lease_id)`, `jobs(job_id, run_id, state)`, `leases(lease_id, owner_session_id, resource_id, state)`.
- Write tables: `queue_messages(state, acked_at, updated_at)`, `jobs(state, started_at)`, `events`.
- Transaction and locks: serializable; lock message, job, and active lease rows `FOR UPDATE`; return idempotently when the message is already `acked`.
- Event anchor: append `queue.acked` only on the state transition from `claimed` to `acked`.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_ack_work.py`.

### `work.complete`

- Source: `src/striatum/db.py::complete_job`, lines 1465-1516; RPC entry currently maps through `CLI_ROUTES["work.complete"]` in `src/striatum/daemon_rpc/server.py`, lines 53-57.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/complete_job.py::handle`.
- Read tables: `jobs(expected_artifacts_json, current_message_id, state)`, `leases`, `artifacts(logical_name, artifact_kind, repo_path)`, `job_dependencies`, `verdicts`, `queue_messages`, `runs`, `sessions`.
- Write tables: `jobs(state, completed_at, current_lease_id)`, `queue_messages(state, completed_at, updated_at, current_lease_id)`, `leases(state, released_at, release_reason)`, downstream `queue_messages`, `runs(completed_at, state, stop_reason)`, `sessions(state, closed_at, close_reason)`, `events`.
- Transaction and locks: serializable; lock job and active lease rows; lock current message when present; lock downstream jobs before enqueue; lock the run row before terminal-state evaluation.
- Event anchor: append `job.completed`, then any downstream `queue.enqueued`, `run.completed`/`run.failed`/`run.canceled`, and `session.closed` events in causal order.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_complete_job.py`.

### `work.release`

- Source: `src/striatum/cli/mutations.py::release_work`, lines 524-573.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/release_lease.py::handle`.
- Read tables: `queue_messages(message_id, job_id, state)`, `jobs(write_scope_json, state)`, `leases`.
- Write tables: `leases(state, released_at, release_reason)`, `jobs(state, current_lease_id)`, `queue_messages(state, current_lease_id, updated_at)`, `events`.
- Transaction and locks: serializable; lock message, job, and active lease; requeue only non-repo-write jobs.
- Event anchor: append `lease.released` with payload `{reason, job_state}`.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_release_lease.py`.

### `work.block`

- Source: `src/striatum/cli/mutations.py::block_work`, lines 605-651.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/block_job.py::handle`.
- Read tables: `jobs(run_id, current_message_id, state)`, `leases`.
- Write tables: `blockers(blocker_id, run_id, job_id, session_id, severity, blocker_kind, description, state, created_at)`, `jobs(state, current_lease_id)`, `leases(state, released_at, release_reason)`, `queue_messages(state, current_lease_id)`, `events`.
- Transaction and locks: serializable; lock job and active lease; lock current message when present.
- Event anchor: append `job.blocked` with payload `{blocker_id, severity}`.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_block_job.py`.

### `review.verdict`

- Source: `src/striatum/cli/mutations.py::verdict_work`, lines 654-684, and `src/striatum/db.py::record_review_verdict`, lines 1585-1647.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/record_verdict.py::handle`.
- Read tables: `jobs(job_type, state, expected_artifacts_json)`, `leases`, `artifacts`, `runs(workflow_snapshot_id)`, `workflow_snapshots(workflow_json)`, `process_supervisors`, `process_supervisor_pointers`, `job_dependencies`, `verdicts`.
- Write tables: `verdicts(verdict_id, run_id, job_id, session_id, verdict, rationale, findings_artifact_id, created_at, posture)`, `jobs`, `leases`, `queue_messages`, downstream `queue_messages`, `runs`, `sessions`, `blockers` for revision routing, `events`.
- Transaction and locks: serializable; lock job and active lease; lock artifact row if `findings_artifact_id` is present; lock run before downstream or terminal transitions.
- Event anchor: append `verdict.recorded`, then chain review transition events produced by accept, needs-revision, or reject handling.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_record_verdict.py`.

### `review.submit`

- Source: `src/striatum/cli/mutations.py::submit_review`, lines 765-834, with prevalidation at lines 837-887.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/submit_review.py::handle`.
- Read tables: `jobs`, `leases`, `artifacts`, `process_executions` for lane evidence guard, `sessions`, `runs`, `workflow_snapshots`, `job_dependencies`, `verdicts`.
- Write tables: `artifacts`, `queue_messages` and `jobs` if auto-acking claimed work, `verdicts`, transition tables touched by `record_verdict`, `events`.
- Transaction and locks: serializable; prevalidate, optional ack, artifact insert, verdict insert, and review transition all occur in one transaction so a published review artifact cannot commit without the matching verdict.
- Event anchor: append `queue.acked` when auto-ack occurs, `artifact.published` from the PG artifact publisher, `verdict.recorded`, and subsequent transition events in that order.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_submit_review.py`.

### `review.override`

- Source: `src/striatum/db.py::override_review_verdict`, lines 1650-1803; RPC entry currently maps through `CLI_ROUTES["review.override"]` in `src/striatum/daemon_rpc/server.py`, lines 63-66.
- Destination: `src/striatum/daemon_pg/handlers/workflow_loop/override_review_verdict.py::handle`.
- Read tables: `jobs(job_type, state, current_message_id)`, `sessions(state, run_id)`, `verdicts(created_at, verdict_id, findings_artifact_id)`, `artifacts`, `runs`, `workflow_snapshots`.
- Write tables: `verdicts`, `jobs` and `queue_messages` when resolving `waiting_human`, `blockers(state, resolved_at)`, downstream `queue_messages`, `runs`, `sessions`, `events`.
- Transaction and locks: serializable; lock job and session; select prior verdicts with `ORDER BY created_at DESC, verdict_id DESC FOR UPDATE` because Postgres has no SQLite `rowid`; lock human-checkpoint blockers before resolving them.
- Event anchor: append `verdict.overridden` with previous and new verdict ids, then chain any blocker resolution, downstream enqueue, run-terminal, and session-close events.
- Test path: `tests/daemon_pg/handlers/workflow_loop/test_override_review_verdict.py`.

## Track B: Recovery And Evidence Handlers

### `recovery.stale_leases`

- Source: `src/striatum/cli/recovery.py::stale_leases`, lines 25-82.
- Destination: `src/striatum/daemon_pg/handlers/recovery_evidence/stale_leases.py::handle`.
- Read tables: `runs`, `jobs(write_scope_json)`, `leases`, `queue_messages`.
- Write tables: `leases`, `jobs`, `queue_messages`, `job_worktrees`, `events` only through lazy expiry.
- Transaction and locks: serializable for expiry, then read-only stale listing; lock expired candidate leases and their jobs during expiry.
- Event anchor: append `lease.expired` and `worktree.abandoned` events only for newly expired rows; pure listing emits no event.
- Test path: `tests/daemon_pg/handlers/recovery_evidence/test_stale_leases.py`.

### `recovery.requeue_stale`

- Source: `src/striatum/cli/recovery.py::requeue_stale`, lines 85-160.
- Destination: `src/striatum/daemon_pg/handlers/recovery_evidence/requeue_stale.py::handle`.
- Read tables: `runs`, `jobs(write_scope_json, state)`, `leases(state, expires_at)`, `queue_messages(state)`.
- Write tables: `jobs(state, current_lease_id)`, `queue_messages(state, current_lease_id, updated_at)` or a new queue row, `leases` via expiry, `events`.
- Transaction and locks: serializable; run lazy expiry first; lock the target job, expired lease, and current message; refuse repo-write jobs.
- Event anchor: append any expiry events, then `recovery.stale_requeued` with `{already_reclaimable, repo_write: false, author?}`.
- Test path: `tests/daemon_pg/handlers/recovery_evidence/test_requeue_stale.py`.

### `recovery.cancel_job`

- Source: `src/striatum/cli/recovery.py::cancel_job`, lines 283-370, helper `_cancel_single_job`, lines 219-280, and helper `_dependents_blocked_only_through`, lines 182-216.
- Destination: `src/striatum/daemon_pg/handlers/recovery_evidence/cancel_job.py::handle`.
- Read tables: `runs`, `jobs(state, current_lease_id, current_message_id)`, `job_dependencies`, `leases`, `queue_messages`.
- Write tables: `leases(state, released_at, release_reason)`, `queue_messages(state, current_lease_id, updated_at)`, `jobs(state, current_lease_id, current_message_id, completed_at)`, `runs`, `sessions`, `events`.
- Transaction and locks: serializable; lock the target job; when `cascade` is true, iteratively lock each dependent job before canceling it; lock run before terminal-state evaluation.
- Event anchor: append one `job.canceled` per canceled job in cancellation order, then any run-terminal/session-close events.
- Test path: `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py`.

### `recovery.process_reconcile`

- Source: `src/striatum/cli/recovery.py::process_reconcile`, lines 607-690.
- Destination: `src/striatum/daemon_pg/handlers/recovery_evidence/process_reconcile.py::handle`.
- Read tables: `runs`, `process_executions(process_id, pid, state, command_json, started_at, ended_at)`, `jobs`, `artifacts`.
- Write tables: `process_executions(state, ended_at)`, `jobs`, `blockers`, `events`.
- Transaction and locks: read running processes first; for each non-live PID, open a serializable transaction and lock that process row plus its job before marking lost and evaluating missing outputs.
- Event anchor: append the PG equivalent of the current process-lost event, then `job.blocked` when output validation opens a blocker.
- Test path: `tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py`.

### `recovery.resume`

- Source: `src/striatum/cli/recovery.py::resume_blocker`, lines 373-593.
- Destination: `src/striatum/daemon_pg/handlers/recovery_evidence/resume_blocker.py::handle`.
- Read tables: `blockers(payload_json, blocker_kind, state)`, `jobs(state, current_lease_id)`, `leases(owner_session_id, state)`, `artifacts`, `verdicts` for output validation.
- Write tables: `leases(last_heartbeat_at, expires_at)`, `blockers(state, resolved_at)`, `jobs(state)`, plus `complete_job` write set when `complete=True`, `events`.
- Transaction and locks: serializable; lock blocker, job, and lease; when `complete=True`, call Track A's PG `complete_job` helper in the same transaction after resolver state is committed in memory, not by re-entering RPC.
- Event anchor: append `recovery.blocker_dismissed_terminal` for forced terminal no-ops or `recovery.process_blocker_resolved` for normal resumes, then chain completion events when inline completion is requested.
- Test path: `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py`.

### `recovery.auto_publish_stale_artifacts`

- Source: `src/striatum/cli/recovery.py::auto_publish_stale_artifacts`, lines 731-980.
- Destination: `src/striatum/daemon_pg/handlers/recovery_evidence/auto_publish_stale_artifacts.py::handle`.
- Read tables: `runs`, `jobs(expected_artifacts_json, workflow_job_id)`, `leases`, `queue_messages`, `sessions`, `process_executions`, `artifacts`.
- Write tables: live mode writes expiry rows, optional ack rows, `artifacts`, `jobs`, `leases`, `queue_messages`, `runs`, `sessions`, `events`; dry-run writes nothing.
- Transaction and locks: dry-run uses a read-only transaction and must not call expiry; live mode processes each candidate in one serializable transaction, locking job, lease, message, and artifact uniqueness rows before publish and complete.
- Event anchor: chain expiry events, optional `queue.acked`, `artifact.published`, `job.completed`, then `recovery.auto_published`; final run-terminal events chain after all candidates.
- Test path: `tests/daemon_pg/handlers/recovery_evidence/test_auto_publish_stale_artifacts.py`.

### `evidence.export`

- Source: `src/striatum/cli/evidence.py::evidence_export`, lines 356-383.
- Destination: `src/striatum/daemon_pg/handlers/recovery_evidence/evidence_export.py::handle`.
- Read tables: `runs`, `workflow_snapshots`, `jobs`, `job_dependencies`, `artifacts`, `sessions`, `verdicts`, `blockers`, `queue_messages`, `leases`, `process_executions` through PG-native `status`, `doctor`, and `evidence_snapshot` helpers.
- Write tables: repository file at `path_text`; `events` only.
- Transaction and locks: build the snapshot in a repeatable-read or serializable read transaction; write the Markdown file after the snapshot is rendered; append `evidence.exported` in a short serializable transaction that verifies the run still exists.
- Event anchor: append `evidence.exported` with payload `{path, sha256}`.
- Test path: `tests/daemon_pg/handlers/recovery_evidence/test_evidence_export.py`.

## Router Swap

Track A owns the one-time router plumbing:

1. Add `src/striatum/daemon_pg/handlers/registry.py` with `register_pg_handler()` and `resolve_pg_handler()`.
2. Add `src/striatum/daemon_pg/handlers/context.py` with `RepoHandlerContext`, transaction helpers, scoped row helpers, and `append_event()`.
3. Update `src/striatum/daemon_pg/handlers/__init__.py` to import `workflow_loop` and `recovery_evidence`.
4. Update `DaemonRpcRouter._route` in `src/striatum/daemon_rpc/server.py` so native handlers run before the `CLI_ROUTES` fallback.

The route branch is:

```python
handler = resolve_pg_handler(envelope.method)
if handler is not None and self.pg_conn is not None:
    ctx = RepoHandlerContext(
        pg_conn=self.pg_conn,
        repository_id=auth.repository_id or str(envelope.params["repository_id"]),
        repo_root=repo_root,
        auth=auth,
    )
    return handler(ctx, envelope.params)
```

No method is removed from `CLI_ROUTES` until its PG parity tests pass. Track B adds only decorated handler modules and tests under its track directory.

## Tests

Add one parity test file per handler at the paths listed above plus:

- `tests/daemon_pg/handlers/conftest.py` for PG database setup, repository registration, migration seeding, and SQLite/PG parity helpers.
- `tests/daemon_pg/handlers/test_router_native_dispatch.py` for `_route` choosing PG handlers before `CLI_ROUTES` and falling back for unported methods.
- `tests/daemon_pg/handlers/test_event_hash_chain.py` for per-repo `previous_hash`/`row_hash` continuity across multiple events in one transaction and across concurrent handler calls.

Every handler test must assert response parity, table-state parity for touched tables, repository scoping, and event-chain continuity. Concurrency tests are mandatory for `claim_next`, `register_session` ordinal allocation, and event head locking.

## Deferred

- RFC 0048 Phase B Go daemon parity is out of scope for this synthesis.
- RFC 0048 Phase C SQLite fallback removal and test-harness escape removal are out of scope.
- The Unix-socket accept-loop gap in `src/striatum/daemon.py` remains deferred.
- README, TODO, CHANGELOG, SPEC, and `HOW_TO_*` updates are operator-owned after the implementation lands.

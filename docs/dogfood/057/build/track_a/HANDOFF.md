---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs: ["docs/dogfood/057/DESIGN_SYNTHESIS.md", "docs/dogfood/057/review/design/REVIEW.md"]
---
author: implementer-unknown-model-002

# Track A Handoff

## Summary

Track A adds native PostgreSQL workflow-loop handlers under
`src/striatum/daemon_pg/handlers/workflow_loop/` and wires
`DaemonRpcRouter._route` to resolve registered PG handlers before the
legacy `striatum.api.invoke` fallback. The router keeps fallback behavior
for unported methods and imports `src/striatum/daemon_pg/handlers/` so
Track B's decorator registrations can attach when that package is present.

The locked migration 0005 schema does not include top-level
`striatumd.events.previous_hash` / `row_hash` columns or a per-repo
event-chain head table, and this job was forbidden from editing
`src/striatum/daemon_pg/sql/`. The PG event appender therefore writes
the locked append-only `striatumd.events` shape and stores chain metadata
inside `payload_json._event_chain` (`previous_hash`, `row_hash`,
`sha256-json-v1`). `canonical_event_hash()` documents the canonical byte
recipe and `event_row_hash()` reads the stored row hash for continuity
checks. Request-level hash-chain continuity remains in the existing daemon
`striatumd.audit_log`.

| Method | Handler | Test |
|---|---|---|
| `session.register` | `workflow_loop/register_session.py::handle` | `test_register_session.py` |
| `work.claim_next` | `workflow_loop/claim_next.py::handle` | `test_claim_next.py` |
| `work.ack` | `workflow_loop/ack_work.py::handle` | `test_ack_work.py` |
| `work.complete` | `workflow_loop/complete_job.py::handle` | `test_complete_job.py` |
| `work.release` | `workflow_loop/release_lease.py::handle` | `test_release_lease.py` |
| `work.block` | `workflow_loop/block_job.py::handle` | `test_block_job.py` |
| `review.verdict` | `workflow_loop/record_verdict.py::handle` | `test_record_verdict.py` |
| `review.submit` | `workflow_loop/submit_review.py::handle` | `test_submit_review.py` |
| `review.override` | `workflow_loop/override_review_verdict.py::handle` | `test_override_review_verdict.py` |

## Method Notes

### `session.register`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/register_session.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_register_session.py -v`
- Coverage: repository scoping, capability denial through `DaemonRpcRouter`, event append digest.
- Byte-equivalence: not confirmed locally; PG tests skipped because the local Postgres user lacks `CREATE DATABASE`.
- Audit anchor: `session.registered` includes `_event_chain.previous_hash` and `_event_chain.row_hash` in `payload_json`.
- Behavior delta: invalid `operator_label` values are normalized to `InvalidTransitionError`, matching SQLite.

### `work.claim_next`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/claim_next.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_claim_next.py -v`
- Coverage: public router dispatch, capability denial, packet insert, lease/message/job state.
- Byte-equivalence: not confirmed locally; PG tests skipped for missing test database privilege.
- Audit anchor: `lease.expired`, `worktree.abandoned`, `queue.claimed`, and supervisor events carry `_event_chain` metadata.
- Behavior delta: no intended API delta; attached-supervisor auto-delivery and stale-lease supervisor/worktree cleanup are ported.

### `work.ack`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/ack_work.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_ack_work.py -v`
- Coverage: repository scoping, capability denial, idempotent ack transition, chained digest check.
- Byte-equivalence: not confirmed locally; PG tests skipped for missing test database privilege.
- Audit anchor: `queue.acked` includes `_event_chain.previous_hash` and `_event_chain.row_hash` in `payload_json`.
- Behavior delta: no intended API delta.

### `work.complete`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/complete_job.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_complete_job.py -v`
- Coverage: job/message/lease completion, downstream enqueue, run completion, repository scoping.
- Byte-equivalence: not confirmed locally; PG tests skipped for missing test database privilege.
- Audit anchor: `job.completed`, downstream `queue.message_enqueued`, and run terminal events are appended in order.
- Behavior delta: terminal run/session close semantics were aligned with SQLite (`run_completed`/`run_canceled` close reasons; canceled run payload includes `all_jobs_canceled`).

### `work.release`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/release_lease.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_release_lease.py -v`
- Coverage: review-only requeue, repo-write block fallback, repository scoping.
- Byte-equivalence: not confirmed locally; PG tests skipped for missing test database privilege.
- Audit anchor: `lease.released` includes `_event_chain.previous_hash` and `_event_chain.row_hash` in `payload_json`.
- Behavior delta: no intended API delta.

### `work.block`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/block_job.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_block_job.py -v`
- Coverage: blocked and human-checkpoint state transitions, blocker insert, repository scoping.
- Byte-equivalence: not confirmed locally; PG tests skipped for missing test database privilege.
- Audit anchor: `job.blocked` includes `_event_chain.previous_hash` and `_event_chain.row_hash` in `payload_json`.
- Behavior delta: no intended API delta.

### `review.verdict`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/record_verdict.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_record_verdict.py -v`
- Coverage: registration smoke for `review.verdict` and deprecated `verdict` route names.
- Byte-equivalence: not confirmed locally; full PG parity coverage still needs a CREATE DATABASE-capable test database.
- Audit anchor: `verdict.recorded` chains before review transition events.
- Behavior delta: `phase_synthesis` jobs are accepted as verdict-capable with neutral posture, matching SQLite; bounded needs-revision cycle cloning is implemented in the handler; coverage is still smoke-level.

### `review.submit`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/submit_review.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_submit_review.py -v`
- Coverage: registration smoke for `review.submit` and deprecated `submit_review` route names.
- Byte-equivalence: not confirmed locally; full PG parity coverage still needs a CREATE DATABASE-capable test database.
- Audit anchor: optional `queue.acked`, `artifact.published`, `verdict.recorded`, then review transition events.
- Behavior delta: `phase_synthesis` jobs are accepted as verdict-capable. Artifact publish uses migration-0005 columns only, so `attestation_override_rationale` is not written until schema support exists; lane-evidence override rationale remains schema-bound.

### `review.override`

- Handler: `src/striatum/daemon_pg/handlers/workflow_loop/override_review_verdict.py::handle`
- Test command: `pytest tests/daemon_pg/handlers/workflow_loop/test_override_review_verdict.py -v`
- Coverage: registration smoke for `review.override`.
- Byte-equivalence: not confirmed locally; full PG parity coverage still needs a CREATE DATABASE-capable test database.
- Audit anchor: `verdict.overridden` event append plus downstream/run-terminal event appends with `_event_chain` metadata.
- Behavior delta: no intended API delta.

## Verification

- `python -m compileall -q src/striatum/daemon_pg/handlers tests/daemon_pg/handlers`
- `ruff check src/striatum/daemon_pg/handlers/context.py src/striatum/daemon_pg/handlers/registry.py src/striatum/daemon_pg/handlers/__init__.py src/striatum/daemon_pg/handlers/workflow_loop src/striatum/daemon_rpc/server.py src/striatum/daemon_rpc/registry.py tests/daemon_pg/handlers/workflow_loop` -> pass
- `pytest tests/daemon_pg/handlers/workflow_loop -q` -> `15 skipped`

The skipped tests require a reachable system Postgres URL and a user with
`CREATE DATABASE`, matching the existing `tests/_harness/pg.py` behavior.

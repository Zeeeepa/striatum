# Implement Prompt: Track A — Workflow-loop handlers (codex)

Produce `docs/dogfood/057/build/track_a/HANDOFF.md`. Front matter:

```
---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs: ["docs/dogfood/057/DESIGN_SYNTHESIS.md", "docs/dogfood/057/review/design/REVIEW.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `implementer-unknown-model-<NN>`.

## Scope

Port these 9 functions from `src/striatum/cli/mutations.py` (SQLite-backed via `striatum.db`) to PG-backed handlers under `src/striatum/daemon_pg/handlers/workflow_loop/` (Track A's exclusive write scope; per-method file or single file per the synthesis):

1. `register_session`
2. `claim_next`
3. `ack_work`
4. `complete_job`
5. `release_lease`
6. `block_job`
7. `record_verdict`
8. `submit_review`
9. `override_review_verdict`

Each handler reads/writes the `striatumd.*` Postgres tables locked in `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql` (sessions, runs, jobs, queue_messages, leases, work_packets, artifacts, verdicts, blockers, events, process_executions). Append audit events to `striatumd.events` with hash-chain continuity per RFC 0033.

## Delegation swap

Update `src/striatum/daemon_rpc/server.py` `DaemonRpcRouter._route` to route the 9 Track A method names AND the 7 Track B method names to the new PG handlers, using the pattern locked by synthesis. Track A owns `server.py`, `registry.py`, and `src/striatum/daemon_pg/handlers/__init__.py`. If synthesis picked decorator-based self-registration, your `handlers/__init__.py` must import the Track B sub-package (`from . import recovery_evidence`) so Track B's decorators run. SQLite delegation stays as the fallback for any un-ported method during integration.

## Tests

Under `tests/daemon_pg/handlers/`, write one test file per method. Each must assert:

- Byte-identical state vs the SQLite-backed equivalent on the same input fixture.
- Audit-chain hash equality (`striatumd.events.prev_hash` chain continuous).
- Capability-token enforcement before any PG write.
- Repository_id scoping (a handler operating on repo A must not see repo B's rows).

If `tests/_harness/pg.py` provides a PG fixture, reuse it. Otherwise scaffold one — minimal, single-tx setup/teardown.

## Sub-agents (use them aggressively)

- **session-lifecycle**: `register_session`, `claim_next`, `ack_work`.
- **job-state**: `complete_job`, `release_lease`, `block_job`.
- **review-flow**: `record_verdict`, `submit_review`, `override_review_verdict`.

Each sub-agent ports its cluster + writes its tests. The implementer integrates and writes the HANDOFF.

## Forbidden writes

Do NOT touch `src/striatum/cli/recovery.py` or `src/striatum/cli/evidence.py` — Track B owns those. Do NOT touch `src/striatum/daemon_pg/sql/` — schema is already locked at migration 0005.

## HANDOFF.md content

For each ported method:

- The new handler path + function name.
- Test path + test command (`pytest tests/daemon_pg/handlers/<method>_test.py -v`).
- Confirmed byte-equivalence with SQLite-backed equivalent.
- Audit-chain anchor evidence (hash before + after).
- One-line summary of any behavior delta (preferably none).

Plus a top-level summary table cross-referencing the synthesis method list.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `implementer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.

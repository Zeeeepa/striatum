# Implement Prompt: Track B — Recovery + evidence handlers (claude)

Produce `docs/dogfood/057/build/track_b/HANDOFF.md`. Front matter:

```
---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs: ["docs/dogfood/057/DESIGN_SYNTHESIS.md", "docs/dogfood/057/review/design/REVIEW.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `implementer-unknown-model-<NN>`.

## Scope

Port these 7 functions to PG-backed handlers under `src/striatum/daemon_pg/handlers/recovery_evidence/` (Track B's exclusive write scope; per-method file or single file per the synthesis):

From `src/striatum/cli/recovery.py`:

1. `stale_leases`
2. `requeue_stale`
3. `cancel_job`
4. `process_reconcile`
5. `resume_blocker`
6. `auto_publish_stale_artifacts`

From `src/striatum/cli/evidence.py`:

7. `evidence_export`

Each handler reads/writes the `striatumd.*` Postgres tables (leases, queue_messages, jobs, blockers, artifacts, process_executions, process_supervisors, events). Append audit events for every state-changing handler; `evidence_export` is read-only and records a digest audit row only.

## Delegation swap

You may NOT edit `src/striatum/daemon_rpc/server.py`, `src/striatum/daemon_rpc/registry.py`, or `src/striatum/daemon_pg/handlers/__init__.py` — those are Track A's exclusive write scope. Your handlers integrate via the registration pattern locked by synthesis (default: decorator-based self-registration on import; Track A's `handlers/__init__.py` imports your sub-package so the decorators run). If synthesis chose a different pattern, your HANDOFF.md must specify the exact line(s) Track A needs to add for each of your 7 method names.

## Tests

Under `tests/daemon_pg/handlers/`, write one test file per method. Each must assert:

- Byte-identical state vs the SQLite-backed equivalent on the same input fixture.
- Audit-chain hash equality (for state-changing handlers).
- `auto_publish_stale_artifacts` publishes the same artifact set as the SQLite path.
- `evidence_export` produces the same deterministic digest under concurrent writers.
- Capability-token enforcement before any PG write.
- Repository_id scoping (a recovery handler on repo A must not touch repo B's rows).

## Sub-agents (use them aggressively, local only)

- **lease-recovery**: `stale_leases`, `requeue_stale`, `cancel_job`.
- **process-recovery**: `process_reconcile`, `resume_blocker`, `auto_publish_stale_artifacts`.
- **evidence**: `evidence_export`.

Each sub-agent ports its cluster + writes its tests. The implementer integrates and writes the HANDOFF.

## Forbidden writes

Do NOT touch `src/striatum/cli/mutations.py` — Track A owns the 9 workflow-loop methods. Do NOT touch `src/striatum/daemon_pg/sql/` — schema is locked.

## HANDOFF.md content

For each ported method:

- The new handler path + function name.
- Test path + test command.
- Confirmed byte-equivalence with SQLite-backed equivalent.
- Audit-chain anchor evidence (state-changing handlers only).
- For `evidence_export`: digest equality proof under fixture.
- One-line summary of any behavior delta (preferably none).

Plus a top-level summary table cross-referencing the synthesis method list.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `implementer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.

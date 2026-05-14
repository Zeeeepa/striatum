---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["rfc-0048", "phase-a", "daemon-pg", "design-review", "ergonomics-dx", "review-posture-ergonomics-dx", "input-dogfood-057-design-synthesis"]
---
author: reviewer-unknown-model-001

# Design Synthesis Review: RFC 0048 Phase A Scope Lock

Posture is ergonomics_dx. All seven mandatory checks pass; three ergonomics_dx
degradations are recorded as required follow-ups, not blockers.

## Mandatory checks

### 1. Method-list completeness — PASS

The synthesis enumerates 16 Phase A methods, matching `src/striatum/daemon_rpc/server.py:20-95` and the cited CLI source.

Track A (workflow-loop, 9 methods):

1. `session.register` (synthesis L29-37) — CLI_ROUTES `session.register` (server.py:42), `register_session` (mutations.py:226).
2. `work.claim_next` (L39-47) — CLI_ROUTES `work.claim_next` (server.py:44), `claim_next` (db.py:993).
3. `work.ack` (L49-57) — CLI_ROUTES `work.ack` (server.py:45), `ack_work` (mutations.py:462).
4. `work.complete` (L59-67) — CLI_ROUTES `work.complete` (server.py:56), `complete_job` (db.py:1465).
5. `work.release` (L69-77) — CLI_ROUTES `work.release` (server.py:47), `release_work` (mutations.py:524).
6. `work.block` (L79-87) — CLI_ROUTES `work.block` (server.py:55), `block_work` (mutations.py:605).
7. `review.verdict` (L89-97) — CLI_ROUTES `review.verdict` (server.py:65), `verdict_work` (mutations.py:654) + `record_review_verdict` (db.py:1585).
8. `review.submit` (L99-107) — CLI_ROUTES `review.submit` (server.py:64), `submit_review` (mutations.py:765) + `prevalidate_submit_review` (mutations.py:837).
9. `review.override` (L109-117) — CLI_ROUTES `review.override` (server.py:66), `override_review_verdict` (db.py:1650).

Track B (recovery + evidence, 7 methods):

1. `recovery.stale_leases` (L121-129) — CLI_ROUTES `recovery.stale_leases` (server.py:78), `stale_leases` (recovery.py:25).
2. `recovery.requeue_stale` (L131-139) — CLI_ROUTES `recovery.requeue_stale` (server.py:79), `requeue_stale` (recovery.py:85).
3. `recovery.cancel_job` (L141-149) — CLI_ROUTES `recovery.cancel_job` (server.py:80), `cancel_job` (recovery.py:283) + `_cancel_single_job` (recovery.py:219) + `_dependents_blocked_only_through` (recovery.py:182).
4. `recovery.process_reconcile` (L151-159) — CLI_ROUTES `recovery.process_reconcile` (server.py:81), `process_reconcile` (recovery.py:607).
5. `recovery.resume` (L161-169) — CLI_ROUTES `recovery.resume` (server.py:82), `resume_blocker` (recovery.py:373).
6. `recovery.auto_publish_stale_artifacts` (L171-179) — CLI_ROUTES `recovery.auto` (server.py:83), `auto_publish_stale_artifacts` (recovery.py:731). *See ergonomics_dx finding 1: section title does not match the registered RPC method name.*
7. `evidence.export` (L181-189) — CLI_ROUTES `evidence.export` (server.py:26), `evidence_export` (evidence.py:356).

No method-list cell is empty.

### 2. Per-method specificity — PASS

For each of the 16 methods the synthesis populates:

- Source path with line range and function name.
- Destination path under `src/striatum/daemon_pg/handlers/{workflow_loop,recovery_evidence}/<file>.py::handle`.
- Read tables with the specific columns referenced (e.g., `runs(state, paused_at)`, `verdicts(created_at, verdict_id, findings_artifact_id)`).
- Write tables with the columns being written (e.g., `leases(state, released_at, release_reason)`).
- Transaction shape — every mutating handler is `SERIALIZABLE`, with named `FOR UPDATE` / `FOR UPDATE SKIP LOCKED` lock targets per method.
- Audit-event row(s) — every method names its event type(s) (`session.registered`, `queue.claimed`, `queue.acked`, `job.completed`, `lease.released`, `job.blocked`, `verdict.recorded`, `verdict.overridden`, `lease.expired`, `worktree.abandoned`, `recovery.stale_requeued`, `job.canceled`, `recovery.blocker_dismissed_terminal`, `recovery.process_blocker_resolved`, `recovery.auto_published`, `evidence.exported`, plus chained downstream events).
- Test file path under `tests/daemon_pg/handlers/{workflow_loop,recovery_evidence}/test_<method>.py`.

No method-row has a blank cell.

### 3. Handler module boundary — PASS

Single layout: per-method files under `src/striatum/daemon_pg/handlers/workflow_loop/` and `src/striatum/daemon_pg/handlers/recovery_evidence/` (synthesis L16).

Single handler signature: `def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]` (L17), with `RepoHandlerContext` exposing `pg_conn`, `repository_id`, `repo_root`, `auth`, `now()`, `new_id()`, `append_event()`.

Single delegation-swap pattern: decorator-based self-registration `@register_pg_handler("work.ack")` registering into `src/striatum/daemon_pg/handlers/registry.py`, with `src/striatum/daemon_pg/handlers/__init__.py` importing both subpackages once (L18). No alternative layouts, signatures, or registration mechanisms are presented.

### 4. Audit-chain anchor — PASS

The synthesis is concrete, not hand-wavy (L23):

- Phase A migration adds `striatumd.events.previous_hash`, `striatumd.events.row_hash`, and a new `striatumd.repo_event_chain_heads(repository_id, last_event_id, last_row_hash, updated_at)` table.
- `ctx.append_event()` locks the per-repo head row `FOR UPDATE`.
- Inserts the event with `previous_hash = last_row_hash`.
- Computes `row_hash` over the canonical event payload.
- Updates the head before commit.
- Multi-event handlers chain each event from the immediately prior event inserted in the same transaction.

Per-method "Event anchor" lines name which events the chain receives and in what causal order (e.g., `work.complete` appends `job.completed`, then downstream `queue.enqueued`, then `run.*` terminal, then `session.closed`).

The phrase "canonical event payload" is the one underspecified piece — the exact serialization (column order, JSON canonicalization rules) is not pinned. Noted as ergonomics_dx finding 2; not a bounce since the mechanism is named.

### 5. Half-ported transition (`_route` decision) — PASS

Synthesis L20 plus the concrete branch at L200-212 specify:

```python
handler = resolve_pg_handler(envelope.method)
if handler is not None and self.pg_conn is not None:
    ctx = RepoHandlerContext(...)
    return handler(ctx, envelope.params)
# else fall through to CLI_ROUTES
```

This is a lookup table (`resolve_pg_handler`) plus an explicit if-check that runs *before* `CLI_ROUTES.get(envelope.method)` at `src/striatum/daemon_rpc/server.py:226`. Methods without a registered native handler fall through to `invoke(args, repo=repo_root)`, preserving SQLite behavior during the half-ported window. The mechanism is concrete.

### 6. `repository_id` enforcement — PASS

Synthesis L19 names a three-layer mechanism, none hand-wavy:

1. Helper-API layer: every helper requires `ctx.repository_id`.
2. Statement-discipline layer: every SQL statement includes `repository_id = %(repository_id)s` in reads and writes.
3. Test layer: tests assert no handler SQL touches repo-local tables without the repository predicate.
4. Schema layer: `repository_id NOT NULL` FKs remain the final guard.

### 7. Test paths — PASS

Each of the 16 method ports has a concrete test file path under `tests/daemon_pg/handlers/{workflow_loop,recovery_evidence}/test_<method>.py`. No wildcard like "tests for handlers". Plus three shared test artifacts at named paths:

- `tests/daemon_pg/handlers/conftest.py` (PG setup + parity helpers).
- `tests/daemon_pg/handlers/test_router_native_dispatch.py` (`_route` PG-first dispatch + fallback).
- `tests/daemon_pg/handlers/test_event_hash_chain.py` (per-repo `previous_hash` / `row_hash` continuity).

Concurrency tests are mandatory for `claim_next`, `register_session` ordinal allocation, and event head locking (L224).

## Ergonomics_dx findings (required follow-ups, not blockers)

### Finding 1: `recovery.auto` ↔ `recovery.auto_publish_stale_artifacts` naming mismatch

The synthesis section heading at L171 is `recovery.auto_publish_stale_artifacts`, but the registered RPC method in `CLI_ROUTES` is `recovery.auto` (server.py:83), and the dispatch sub-argv is `("recovery", "auto")`. The decorator convention shown elsewhere is `@register_pg_handler("<rpc-method>")`, so the actual registration must be `@register_pg_handler("recovery.auto")` — but the section title implies otherwise. An operator grepping for `register_pg_handler("recovery.auto_publish_stale_artifacts")` would find nothing. **Required follow-up:** the implementer must explicitly clarify in the docstring/decorator that the RPC method is `recovery.auto` and rename the section header (or the corresponding decorator string) in the implementation commit.

### Finding 2: "Canonical event payload" hashing input is underspecified

L23 says `row_hash` is "computed over the canonical event payload" without pinning serialization rules (column order, timestamp precision, JSON key sort, NULL handling). Different implementations of "canonical" will produce divergent hashes, and the chain becomes load-bearing only if every writer agrees on the byte-string. **Required follow-up:** before the first port lands, document the exact canonicalization recipe in `src/striatum/daemon_pg/handlers/context.py::append_event` (or a sibling helper) — at minimum, list the columns in hash order and the JSON serialization mode.

### Finding 3: Handler error-message and test-failure conventions not specified

The synthesis specifies what tests must assert (response parity, table-state parity, repository scoping, event-chain continuity) but not the *messaging* shape. Two ergonomics_dx prompt items go unaddressed:

- Handler error messages should cite operator-actionable next steps. The synthesis does not say where these strings live, what format they take, or how they relate to existing CLI error strings.
- Test failure messages should diff state directly, not just `assert False`. The synthesis does not require helpers like `assert_table_state(expected, actual)` with diff output.

**Required follow-up:** decide whether the existing `RpcError` / `invoke` error format is reused verbatim (cheapest path) or whether handlers grow a new structured-error helper, and pick a parity assertion helper for the test conftest.

## Verdict

`accept_with_findings`. Implementation may proceed in parallel with the three required follow-ups above being addressed in the first implementation commit or the conftest commit, not deferred to Phase B.

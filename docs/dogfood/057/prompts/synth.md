# Synthesis Prompt: RFC 0048 Phase A

Produce `docs/dogfood/057/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/057/design/codex/DESIGN.md", "docs/dogfood/057/design/claude_code/DESIGN.md", "docs/dogfood/057/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `designer-unknown-model-<NN>`.

Reconcile the 3 designs into ONE concrete plan with two implementer tracks. Choose; do not enumerate alternatives.

## Track A — Workflow-loop handlers (codex)

Lock the 9 method ports: `register_session`, `claim_next`, `ack_work`, `complete_job`, `release_lease`, `block_job`, `record_verdict`, `submit_review`, `override_review_verdict`.

For each method, the synthesis MUST specify:

- Source: exact file path + function name + line range in `src/striatum/cli/mutations.py`.
- Destination: exact file path + handler function name under `src/striatum/daemon_pg/handlers/`.
- PG tables touched (write set + read set) — cite column names where the schema is non-obvious.
- Transaction shape (single transaction; isolation level; explicit row-level locks).
- Audit-chain anchor — exact `striatumd.events` row(s) appended + how `prev_hash` chains.
- Test file path under `tests/daemon_pg/handlers/`.

## Track B — Recovery + evidence handlers (claude)

Lock the 7 method ports: `stale_leases`, `requeue_stale`, `cancel_job`, `process_reconcile`, `resume_blocker`, `auto_publish_stale_artifacts`, `evidence_export`. Same level of specificity as Track A.

## Cross-cutting decisions

Pick one path for each, with a one-sentence justification:

1. **Handler module layout** — locked by workflow: Track A under `src/striatum/daemon_pg/handlers/workflow_loop/`, Track B under `src/striatum/daemon_pg/handlers/recovery_evidence/`. Choose per-method file vs single file per track sub-dir.
2. **Handler signature** — `(pg_conn, params, auth)` vs context-object vs typed-request.
3. **Delegation-swap pattern in `DaemonRpcRouter._route`** — must allow Track B handlers to integrate without Track B editing `server.py`/`registry.py`/`handlers/__init__.py` (those are Track A's write scope). Default: decorator-based self-registration on import, with Track A's `handlers/__init__.py` importing Track B's sub-package to trigger registration. If you pick a different pattern (lookup table, explicit if/elif), specify how Track B contributes its entries within its write scope.
4. **`repository_id` scope enforcement** — NOT NULL + check, `WHERE repository_id = $1` discipline, or wrapper.
5. **Half-ported routing** — how `DaemonRpcRouter._route` decides PG-handler vs SQLite-fallback during the transition.
6. **PG test fixture** — reuse `tests/_harness/pg.py` (if present) or new harness.

If two designs disagree, pick one. Don't enumerate.

## Out of scope (do not include)

- RFC 0048 Phase B (Go core) and Phase C (SQLite removal).
- The Unix-socket accept-loop gap in `src/striatum/daemon.py` — note as deferred under "deferred" section; do not solve.
- README / TODO / CHANGELOG / SPEC / `HOW_TO_*` updates — operator-only after landing.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `designer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.

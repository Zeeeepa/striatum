# Design Prompt: RFC 0048 Phase A (port daemon RPC single-repo handlers to PG-direct)

Produce DESIGN.md at the path your work packet specifies (under `docs/dogfood/057/design/<lane>/`).

Read `docs/rfcs/0048-daemon-side-substrate-migration.md` first — especially "Background" (substrate facade gap), "Goals", "Phasing" (Phase A scope), and "Acceptance". Then skim:

- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` — substrate boundary + daemon-required runtime.
- `docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md` — daemon Postgres schema, roles, audit chain anchoring.
- `docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md` — RPC envelope, method registry.
- `docs/POSTGRES_TRANSITION.md` — current operator runbook (the new substrate path you are wiring).
- `docs/DECISION_LOG.md` — D094 supersedes D006/D007/D036 and the SQLite half of D009.

Current code to read before designing:

- `src/striatum/cli/mutations.py` — the 9 workflow-loop functions today, all SQLite-backed via `striatum.db.connect`.
- `src/striatum/cli/recovery.py` — the 6 recovery functions today.
- `src/striatum/cli/evidence.py` — `evidence_export` today.
- `src/striatum/daemon_rpc/server.py` — `DaemonRpcRouter._route` and how it currently delegates.
- `src/striatum/daemon_rpc/registry.py` — `METHOD_REGISTRY` and how methods get wired.
- `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql` — the 15 repo-local tables locked in dogfood-048; **these are the PG tables your handlers write to**.
- `src/striatum/daemon_pg/repo_local_migration.py` — how migrated repos identify by `repository_id`.

## Design the implementation across two tracks

### Track A — Workflow-loop handlers (codex)

Port these 9 functions from `src/striatum/cli/mutations.py` to PG-backed handlers:

`register_session`, `claim_next`, `ack_work`, `complete_job`, `release_lease`, `block_job`, `record_verdict`, `submit_review`, `override_review_verdict`.

For each: name the exact PG tables touched, the exact transaction boundary, the audit-event row(s) appended to `striatumd.events`, and the lock strategy needed to keep the audit chain unbroken under concurrent inserts (RFC 0033 row-level lock on `audit_chain_head` analog; or SERIALIZABLE single-tx).

### Track B — Recovery + evidence handlers (claude)

Port these 7 functions from `src/striatum/cli/recovery.py` (6) and `src/striatum/cli/evidence.py` (1):

`stale_leases`, `requeue_stale`, `cancel_job`, `process_reconcile`, `resume_blocker`, `auto_publish_stale_artifacts`, `evidence_export`.

Same level of detail: PG tables, transaction boundary, audit events, lock strategy. `evidence_export` is read-only — locking less complex but still must produce a deterministic digest under concurrent writers.

## Cross-cutting decisions to propose

- **Handler module layout**: one file per handler (`src/striatum/daemon_pg/handlers/<method>.py`), or per cluster (`workflow_loop.py` / `recovery.py` / `evidence.py`)? Justify.
- **Handler signature**: do they take `(pg_conn, params, auth)`, or a thicker context object, or are they pure functions over a typed request? Pick one, cite RFC 0030.
- **Delegation-swap pattern in `DaemonRpcRouter._route`**: lookup table keyed by method name, decorator-registered, or explicit if/elif? Pick one. Must be greppable.
- **Per-method `repository_id` scope**: every handler scopes its writes by `repository_id` from auth context; how is that enforced (NOT NULL column + check, or `WHERE repository_id = $1` discipline, or a wrapper)?
- **Test infrastructure**: tests under `tests/daemon_pg/handlers/<method>_test.py` that run the same workflow input against the SQLite-backed function AND the new PG handler, and assert byte-identical state + audit-chain hash equality. Cite `tests/_harness/pg.py` if it already provides PG fixtures.
- **Migration risk**: how does a half-ported run (some methods PG, others SQLite) stay coherent? RFC 0048 Phase A says SQLite delegation stays as fallback for un-ported methods. Specify the routing.

## Out of scope (this dogfood — V1 only)

- RFC 0048 **Phase B** (Go-core parity in `go/pkg/rpc/`) — separate dogfood.
- RFC 0048 **Phase C** (SQLite removal + escape-env deprecation) — separate dogfood.
- The Unix-socket accept-loop gap in `src/striatum/daemon.py` (`run_daemon_foreground` binds but does not accept). Out of RFC 0048; mention in your design's "deferred" section but do not solve.
- Bundled Postgres distribution, hosted/cloud daemon, multi-tenancy (`tenant_id`).
- README / TODO / CHANGELOG / SPEC / `HOW_TO_*` updates are operator-only after the dogfood lands.

Cite RFC sections by anchor. Hand-waving "we add a method" without pinpoint citation will get the review to bounce.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes, NO lane prefix. Lowercase `author:`. Slug shape: `<role>-unknown-model-<NN>`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.

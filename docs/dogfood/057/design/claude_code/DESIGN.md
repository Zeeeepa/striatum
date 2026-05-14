# RFC 0048 Phase A — Daemon-Side Substrate Port (claude lane)

author: designer-unknown-model-001

## 1. Scope and framing

RFC 0048 §Background names the gap precisely: `DaemonRpcRouter._route`
in `src/striatum/daemon_rpc/server.py:211` calls
`striatum.api.invoke(args, repo=repo_root)` (line 230) which, per
`src/striatum/api.py:21-26`, re-enters the SQLite-backed CLI parser via
`build_parser()` / `dispatch(namespace)`. Every single-repo
mutation, recovery verb, and `evidence.export` therefore still writes to
`.striatum/state.sqlite3` after `daemon migrate-repo-local` has moved
the authoritative rows into `striatumd.*`. RFC 0048 §Phasing > Phase A
demands one method-at-a-time substrate flip on the Python daemon
without disturbing the RPC envelope (RFC 0030 §1), the audit chain
(RFC 0033 §5), or the per-repo namespace (RFC 0043 §1).

This design covers both implementer tracks:

- **Track A (codex)** — port the 9 workflow-loop handlers from
  `src/striatum/cli/mutations.py` (RFC 0048 §Phasing > Phase A,
  bullets 1–2).
- **Track B (claude)** — port `src/striatum/cli/recovery.py` (6 verbs)
  and `src/striatum/cli/evidence.py::evidence_export` (1 verb)
  (RFC 0048 §Phasing > Phase A, bullets 3–4).

The 15 repo-local PG tables enumerated in
`src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql` —
`workflow_snapshots`, `runs`, `sessions`, `jobs`, `job_dependencies`,
`queue_messages`, `leases`, `work_packets`, `artifacts`, `verdicts`,
`blockers`, `command_requests`, `process_executions`, `events`,
`job_worktrees`, `process_supervisors`,
`process_supervisor_pointers` — are the canonical write targets. All
are primary-keyed on `(repository_id, <id>)` per
`0005_repo_local_workflow_state.sql:1-475` and FK-locked back to
`striatumd.repositories`.

## 2. Cross-cutting decisions

### 2.1 Handler module layout — one cluster per file

Choose `src/striatum/daemon_pg/handlers/workflow_loop.py`,
`recovery.py`, `evidence.py`. Rationale:

- The verbs inside each cluster share private helpers
  (`_expire_leases`, `_dependents_blocked_only_through`,
  `_cancel_single_job` already share state in `recovery.py:182-280`).
  One-file-per-method would force module-level helpers or circular
  imports; cluster files keep the helper next to its only caller.
- The cluster names already exist as CLI sub-packages, so the
  daemon-side mirror reads symmetrically (`cli/recovery.py` ↔
  `daemon_pg/handlers/recovery.py`).
- `src/striatum/daemon_pg/handlers/` is currently empty (see
  the path the gemini DESIGN cited at line 19); we are not breaking
  an established layout.

### 2.2 Handler signature

```python
def handle(
    pg_conn: psycopg.Connection,
    *,
    repository_id: str,
    repo_root: Path,
    params: Mapping[str, Any],
    auth: RpcAuthContext,
) -> dict[str, Any]:
```

Rationale, citing RFC 0030 §1 (envelope schema) and §4 (capability
binding):

- `pg_conn` is the daemon-owned connection the router already holds
  (`DaemonRpcRouter.pg_conn`, `server.py:99`). Handlers do not open
  their own connections — that would dodge audit (RFC 0033 §5) and
  request-log accounting (RFC 0030 §6).
- `repository_id` is split out of `params` because RFC 0043 §1
  requires every write to be scoped by it; pulling it from `auth`
  (already resolved via `authorize(...)` at `server.py:147`) makes
  the scoping a typed argument rather than an in-handler dictionary
  read. Handlers MUST refuse to write if `repository_id` is None
  (defense in depth — the router already raises `repo_not_registered`
  at `server.py:131`).
- `repo_root` stays because `evidence.export` and
  `auto_publish_stale_artifacts` need to read/write files under the
  worktree, and `_repo_root_for` at `server.py:194` already validates
  the path against `striatumd.repositories`.
- `params: Mapping[str, Any]` instead of a typed request dataclass:
  RFC 0030 ships method-specific schemas in
  `daemon.describe`, not as Python types. A typed wrapper adds a
  second source of truth. Handlers do their own coercion at the top
  of the function (mirroring the CLI parser today). A follow-up dogfood
  can lift this into generated request types once the registry has a
  schema column.
- `auth: RpcAuthContext` is passed through so handlers can stamp
  events with `actor_session_id` from the capability binding without
  re-reading the token table.

Return shape is the same JSON envelope each CLI function returns
today (the `JsonObject` alias from `striatum.db`); the router wraps it
in `RpcResponse.ok_response` (`server.py:156`).

### 2.3 Delegation-swap pattern — explicit lookup table in `_route`

Add a module-level dict in `daemon_rpc/server.py`:

```python
NATIVE_PG_HANDLERS: dict[str, NativeHandler] = {
    "session.register": _wrap(handlers.workflow_loop.register_session),
    "work.claim_next": _wrap(handlers.workflow_loop.claim_next),
    ...
    "recovery.stale_leases": _wrap(handlers.recovery.stale_leases),
    ...
    "evidence.export": _wrap(handlers.evidence.evidence_export),
}
```

`_route` (`server.py:211-237`) becomes:

```python
def _route(self, envelope, *, repo_root):
    if envelope.method == "daemon.describe": ...
    if envelope.method == "dashboard.all": ...
    if envelope.method.startswith("cross_repo."): ...
    if envelope.method.startswith("apply."): ...
    if envelope.method.startswith("dogfood."): ...
    native = NATIVE_PG_HANDLERS.get(envelope.method)
    if native is not None and self.pg_conn is not None:
        return native(self.pg_conn, envelope, repo_root)
    prefix = CLI_ROUTES.get(envelope.method)
    if prefix is None:
        raise RpcError("method_unknown", ...)
    return _legacy_invoke(prefix, envelope, repo_root)
```

Rationale:

- **Greppable.** A single literal table at module scope means
  `grep "session.register"` lands on the registration site
  immediately. RFC 0048 §Phasing requires "each method is its own
  commit" — a decorator-registered approach makes per-method
  reverts harder.
- **Falls through to SQLite for un-ported methods.** RFC 0048
  §Phasing explicitly preserves SQLite delegation as the fallback
  during the transition. The `NATIVE_PG_HANDLERS.get(...) or
  legacy_invoke(...)` shape is the routing contract that closes the
  facade as ports land, without a flag day.
- **Gated on `pg_conn is not None`.** When `STRIATUM_DAEMON_REQUIRED=0
  STRIATUM_TEST_HARNESS=1` is set (the `POSTGRES_TRANSITION.md` "RFC
  0048 remaining work" escape), `pg_conn` is None and the router
  falls back to the SQLite path. This keeps the legacy test fixtures
  green until Phase C retires the escape.

### 2.4 Per-method `repository_id` scope enforcement

Three-layer defense:

1. **Schema.** Every repo-local table already has
   `repository_id text NOT NULL REFERENCES
   striatumd.repositories(repository_id)`
   (`0005_repo_local_workflow_state.sql:2-475`). Inserts without
   `repository_id` fail at the DB level.
2. **Handler convention.** Every SQL statement a handler issues
   includes `repository_id = %(repository_id)s` in its WHERE / VALUES
   clause. A `psycopg.sql.Composed` helper
   `scope(query, repository_id)` is provided in
   `daemon_pg/handlers/_scope.py` and **used by every read** so a
   forgotten WHERE clause is caught at code review.
3. **Audit.** Every `striatumd.events` row written by a handler
   includes `repository_id` (PK requires it); the existing
   append-only trigger
   (`0005_repo_local_workflow_state.sql:447-455`) refuses
   updates/deletes. A future leaked write surface would still show up
   in the audit chain.

We do not add a Python wrapper that mutates SQL — the cost of
inspecting Composed objects in tests is higher than the value over
the in-handler discipline + DB-level NOT NULL FK.

### 2.5 Audit-chain anchoring under concurrent writers

RFC 0033 §6 requires audit append to be serializable, with a
`previous_hash` read inside the same transaction. The current Python
helper `striatum.db.insert_event` (used by every CLI mutation) runs
inside `transaction(conn)` and trusts SQLite's single-writer model
for chain integrity. PG cannot.

Strategy:

- **All handlers run inside a single `psycopg` transaction** opened
  with `connection.transaction()` and `cur.execute("SET TRANSACTION
  ISOLATION LEVEL SERIALIZABLE")` as the first statement. This
  matches RFC 0033 §6 ("audit append uses a serializable transaction
  with `previous_hash` read inside the same transaction").
- **Audit-chain head locking.** RFC 0033 §5 keeps the V1 hash-chain
  invariant. The repo-local schema today stores raw events in
  `striatumd.events` (`0005_repo_local_workflow_state.sql:324-348`)
  without a per-repo head-pointer column. Phase A adds
  `striatumd.audit_chain_heads(repository_id PK, last_event_id,
  last_row_hash, updated_at)` and a `SELECT ... FOR UPDATE` on
  the head row at the top of every write transaction. This is the
  RFC 0033 §6 "row-level lock on `audit_chain_head` analog" the
  prompt asks for. The head is updated as the last statement before
  COMMIT so the hash computed from `previous_hash || row_payload`
  reads back through the lock.
- **Serialization failures retry once.** A `psycopg.errors.SerializationFailure`
  raised at COMMIT is caught at the router boundary and the handler
  is re-invoked with the same params; the second failure surfaces as
  `RpcError("serialization_retry_exhausted")` and the operator's
  client retries the envelope.

This pattern also covers the workflow-loop methods on Track A:
`claim_next` does not need `FOR UPDATE` on the audit head; it needs
`FOR UPDATE SKIP LOCKED` on `striatumd.queue_messages` to serialize
the dequeue, plus the same head-lock for the `work.claimed` event.

### 2.6 Test infrastructure

`tests/_harness/pg.py` already provides ephemeral databases via
`create_ephemeral_database(base_url)` (see `pg.py:56-75`) and applies
all migrations via `apply_migrations(conn, daemon_version="test")`.
Phase A adds:

- `tests/daemon_pg/handlers/<method>_test.py` — one file per ported
  method.
- Each test file imports both the legacy SQLite function and the new
  PG handler, runs the same canonical input against both, and
  asserts:
  - Equal returned JSON envelopes (modulo timestamps, asserted to be
    parseable RFC 3339).
  - Equal post-state when read back through a `RowSet` helper that
    selects every repo-local table ordered by `(repository_id, *)`.
  - Equal audit chain hashes computed by extending
    `daemon_pg/audit.py::compute_repo_local_reanchor` (already used by
    `repo_local_migration.py` to verify byte-equivalent migration).
- A shared `parity_case(method_name, params)` fixture that runs an
  isolated SQLite repo and an isolated PG ephemeral database, primes
  both with the same workflow snapshot/run, and yields callable
  invokers.

This is the explicit acceptance criterion in RFC 0048 §Acceptance >
Phase A.

### 2.7 Half-ported run coherence

RFC 0048 §Phasing > Phase A explicitly allows partial port: SQLite
delegation stays as fallback. Coherence is preserved because:

- After `migrate-repo-local` runs successfully
  (`repo_local_migration.py`), the daemon has both substrates
  populated and the SQLite file is in a sentinel state (per RFC 0048
  §Phase C the sentinel flips only in Phase C).
- Native handlers read from and write to PG only. Legacy handlers
  reach SQLite via `striatum.api.invoke` → `striatum.cli.dispatch` →
  `striatum.db.connect`. The two substrates therefore diverge on any
  method that is half-ported: e.g. a PG-native `claim_next` followed
  by an unported `complete` would write the claim to PG and the
  completion to SQLite.
- The routing rule is therefore: **a method MUST stay on the SQLite
  path until every method that mutates the same set of rows is
  ported, OR the codepath is exercised against a freshly migrated
  repo that has no live SQLite writers.** Phase A treats both
  workflow-loop and recovery clusters as atomic units; the
  `NATIVE_PG_HANDLERS` table lands all 16 entries in one commit at
  the end of the Phase A burn-down, but each individual handler
  module is its own preceding commit so reverts are surgical.

That sequencing is the only safe interpretation of RFC 0048 §Phasing
"the daemon RPC router swaps the delegated handler for the PG-backed
one as each lands." Without it, half-port creates split-brain.

## 3. Track A — Workflow-loop handlers (codex implementer)

Each entry below names: file/function, PG tables touched, transaction
boundary, audit events, lock strategy.

### 3.1 `register_session` → `handlers/workflow_loop.py::register_session`

- **Tables:** `striatumd.sessions` (INSERT),
  `striatumd.workflow_snapshots` (SELECT for HARNESS-003 policy),
  `striatumd.sessions` (SELECT for the `reviewer_context_policy:
  fresh` check at `mutations.py:275-296`), `striatumd.runs` (SELECT
  for membership validation), `striatumd.events` (INSERT
  `session.registered`).
- **Transaction:** SERIALIZABLE; acquire `audit_chain_heads` FOR
  UPDATE; INSERT session and event in same tx; UPDATE
  `audit_chain_heads`; COMMIT.
- **Audit event:** `session.registered` with payload
  `{slug, role_id, lane_id, ordinal, fresh, non_fresh_reason,
   operator_label}`.
- **Locks:** `audit_chain_heads` row-level FOR UPDATE only. The
  uniqueness invariants (`UNIQUE (repository_id, run_id, slug)` and
  `UNIQUE (repository_id, run_id, role_id, lane_id, ordinal)` on
  `striatumd.sessions`, `0005_repo_local_workflow_state.sql:68-69`)
  serialize concurrent registrations naturally; on conflict the
  handler retries the ordinal computation inside the same tx.

### 3.2 `claim_next` → `handlers/workflow_loop.py::claim_next`

- **Tables:** `striatumd.queue_messages` (SELECT … FOR UPDATE SKIP
  LOCKED, then UPDATE state=claimed), `striatumd.leases` (INSERT),
  `striatumd.jobs` (UPDATE state=claimed, current_lease_id,
  current_message_id), `striatumd.work_packets` (INSERT),
  `striatumd.sessions` (SELECT for capability/role check),
  `striatumd.events` (INSERT `work.claimed`).
- **Transaction:** SERIALIZABLE; head-lock; dequeue with FOR UPDATE
  SKIP LOCKED (RFC 0033 §6 names this pattern); UNIQUE
  `uq_active_resource_lease` on `striatumd.leases`
  (`0005_repo_local_workflow_state.sql:184-186`) prevents double-claim.
- **Audit event:** `work.claimed` payload
  `{job_id, lease_id, message_id, expires_at, packet_id,
   packet_sha256}`.
- **Locks:** queue dequeue uses `FOR UPDATE SKIP LOCKED` on
  `striatumd.queue_messages` so concurrent claimers serialize without
  deadlock; audit head row-level lock.

### 3.3 `ack_work` → `handlers/workflow_loop.py::ack_work`

- **Tables:** `striatumd.queue_messages` (UPDATE state=acked, acked_at),
  `striatumd.jobs` (UPDATE state=running), `striatumd.leases`
  (SELECT to assert active + ownership), `striatumd.events` (INSERT
  `work.acked`).
- **Transaction:** SERIALIZABLE; head-lock; the lease state check
  inside the tx is the gate.
- **Audit event:** `work.acked` payload `{job_id, lease_id, message_id}`.
- **Locks:** SELECT on lease with no FOR UPDATE — uniqueness of
  `uq_active_resource_lease` plus the tx isolation guarantees a
  concurrent release-and-reclaim cannot interleave inside the same
  serializable snapshot.

### 3.4 `complete_job` → `handlers/workflow_loop.py::complete_job`

- **Tables:** `striatumd.jobs` (UPDATE state=completed, completed_at,
  current_lease_id=NULL, current_message_id=NULL),
  `striatumd.leases` (UPDATE state=released, released_at,
  release_reason='completed'), `striatumd.queue_messages` (UPDATE
  state=completed, completed_at), `striatumd.runs` (UPDATE state via
  `maybe_complete_run` port), `striatumd.job_dependencies` (SELECT
  to gate downstream `enqueue_job` calls), `striatumd.queue_messages`
  (INSERT for downstream enqueues), `striatumd.events` (INSERT
  `job.completed`, and one `work.enqueued` per downstream).
- **Transaction:** SERIALIZABLE; head-lock; downstream enqueues all
  inside the same tx so the chain advances atomically.
- **Audit events:** `job.completed {summary}`, `work.enqueued
  {job_id, message_id}` per downstream, optional `run.completed`
  payload from the `maybe_complete_run` port.
- **Locks:** audit head FOR UPDATE; the run-completion check
  re-reads the same head snapshot so the run.completed event chains
  correctly even when another handler also completes a sibling job.

### 3.5 `release_lease` → `handlers/workflow_loop.py::release_lease`

- **Tables:** `striatumd.leases` (UPDATE state=released, released_at,
  release_reason), `striatumd.jobs` (UPDATE state=queued,
  current_lease_id=NULL, current_message_id=NULL),
  `striatumd.queue_messages` (UPDATE state=pending,
  current_lease_id=NULL), `striatumd.events` (INSERT
  `work.released`).
- **Transaction:** SERIALIZABLE; head-lock.
- **Audit event:** `work.released` payload
  `{job_id, lease_id, message_id, reason}`.
- **Locks:** the lease row's FK from `striatumd.jobs.current_lease_id`
  and the partial unique index `uq_active_resource_lease` enforce
  single-active-lease; release is a state flip, not a delete.

### 3.6 `block_job` → `handlers/workflow_loop.py::block_job`

- **Tables:** `striatumd.jobs` (UPDATE state=blocked),
  `striatumd.blockers` (INSERT), `striatumd.queue_messages`
  (UPDATE state=blocked), `striatumd.leases` (UPDATE state=released
  if active), `striatumd.events` (INSERT `job.blocked`).
- **Transaction:** SERIALIZABLE; head-lock.
- **Audit event:** `job.blocked` payload
  `{blocker_id, blocker_kind, severity, description_redacted}`. The
  description column is operator-facing free text; the audit payload
  redacts it per the `EVIDENCE_POLICY` registry the parity tests
  share with `cli/evidence.py:65-104`.
- **Locks:** head FOR UPDATE; no cross-row locks needed.

### 3.7 `record_verdict` → `handlers/workflow_loop.py::record_verdict`

- **Tables:** `striatumd.verdicts` (INSERT, with
  `UNIQUE (repository_id, job_id, session_id)` collision returning
  the existing row idempotently),
  `striatumd.artifacts` (SELECT to resolve `findings_artifact_id`),
  `striatumd.jobs` (UPDATE state=completed for verdict-typed jobs),
  `striatumd.events` (INSERT `verdict.recorded`), plus
  downstream `work.enqueued` events when the verdict unblocks
  dependents.
- **Transaction:** SERIALIZABLE; head-lock; the unique index on
  `(repository_id, job_id, session_id)`
  (`0005_repo_local_workflow_state.sql:253`) is the idempotency key.
- **Audit event:** `verdict.recorded` payload
  `{verdict, posture, findings_artifact_id}`.
- **Locks:** rely on the unique index; no extra FOR UPDATE needed
  beyond audit head.

### 3.8 `submit_review` → `handlers/workflow_loop.py::submit_review`

- **Tables:** `striatumd.artifacts` (INSERT for the findings ledger
  if a path is given), `striatumd.verdicts` (INSERT via
  `record_verdict` port), `striatumd.events` (INSERT
  `review.submitted`).
- **Transaction:** SERIALIZABLE; head-lock; one tx wraps both the
  artifact publish and the verdict insert so a half-submitted review
  cannot exist.
- **Audit event:** `review.submitted` payload
  `{verdict, findings_artifact_id, posture}`.
- **Locks:** the artifact `UNIQUE (repository_id, run_id, job_id,
  logical_name)` (`0005_repo_local_workflow_state.sql:231`) plus the
  verdict unique index hold idempotency.

### 3.9 `override_review_verdict` → `handlers/workflow_loop.py::override_review_verdict`

- **Tables:** `striatumd.verdicts` (INSERT a new verdict row with
  `posture='override'`, leaving prior rows intact — the table is
  not append-only-strict but the contract is preserved by the
  unique index forbidding a second `(job_id, session_id)` row;
  override uses a different `session_id` per the policy in
  `mutations.py`), `striatumd.events` (INSERT
  `verdict.overridden`).
- **Transaction:** SERIALIZABLE; head-lock.
- **Audit event:** `verdict.overridden` payload
  `{prior_verdict_id, new_verdict, reason, actor_session_id}`.
- **Locks:** audit head FOR UPDATE.

## 4. Track B — Recovery + evidence handlers (claude implementer)

### 4.1 `stale_leases` → `handlers/recovery.py::stale_leases`

- **Tables:** `striatumd.leases` (UPDATE state=expired for rows past
  `expires_at`; the lazy expiry port of
  `recovery.py::expire_leases`), `striatumd.jobs` (UPDATE state to
  `stale_lease` for jobs whose active lease just expired),
  `striatumd.queue_messages` (JOIN for the message_state field in
  the response), `striatumd.events` (INSERT one
  `lease.expired` row per expired lease).
- **Transaction:** SERIALIZABLE; head-lock at the top so the
  emitted `lease.expired` events chain in deterministic order; one
  tx for the whole inspection.
- **Audit events:** `lease.expired` per row, payload
  `{lease_id, resource_id, expires_at, repo_write}`.
- **Locks:** `striatumd.leases` UPDATE acquires row-level locks
  on each candidate; PG's MVCC means concurrent claimers attempting
  the same row see the UPDATE first or fail their own lease
  validation. No FOR UPDATE pre-scan needed.

The CLI `stale_leases` function is read-mostly today but performs
the side-effecting `expire_leases` inside `transaction(conn)`
(`recovery.py:28-29`); the PG port keeps the same write
on the same call. Read-only inspection lives in a planned
`recovery.stale_leases_inspect` follow-up (not in Phase A).

### 4.2 `requeue_stale` → `handlers/recovery.py::requeue_stale`

- **Tables:** `striatumd.jobs` (UPDATE state=queued,
  current_lease_id=NULL), `striatumd.queue_messages` (UPDATE
  state=pending, current_lease_id=NULL, updated_at) OR INSERT a new
  message row via the `enqueue_job` port when `message_id IS NULL`,
  `striatumd.leases` (SELECT for the expired-lease lookup
  per `recovery.py:98-110`), `striatumd.events` (INSERT
  `recovery.stale_requeued`).
- **Transaction:** SERIALIZABLE; head-lock; one tx — the call
  is idempotent under the `recovery.py:118` "already_reclaimable"
  branch, which becomes a read of the current message state.
- **Audit event:** `recovery.stale_requeued` payload
  `{job_id, message_id, lease_id, already_reclaimable, repo_write,
   author}`.
- **Locks:** the unique partial index
  `uq_active_work_message_per_job`
  (`0005_repo_local_workflow_state.sql:162-164`) guarantees a
  job cannot have two pending messages; conflict surfaces as
  `psycopg.errors.UniqueViolation` and the handler treats it as
  the already-reclaimable branch.

### 4.3 `cancel_job` → `handlers/recovery.py::cancel_job`

- **Tables:** `striatumd.jobs` (UPDATE state=canceled,
  current_lease_id=NULL, current_message_id=NULL, completed_at;
  recursive UPDATE for cascade dependents), `striatumd.leases`
  (UPDATE state=released, release_reason='canceled' for any active
  or expired-without-reason lease), `striatumd.queue_messages`
  (UPDATE state=canceled, current_lease_id=NULL),
  `striatumd.job_dependencies` (SELECT for the
  `_dependents_blocked_only_through` walk at
  `recovery.py:182-216`), `striatumd.runs` (UPDATE via
  `maybe_complete_run` port), `striatumd.events` (INSERT
  `job.canceled` per canceled job).
- **Transaction:** SERIALIZABLE; head-lock; **single tx for the
  whole cascade** so partial-cascade state cannot persist. The
  recursion is bounded by the `visited` set on the Python side
  (`recovery.py:341-355`).
- **Audit events:** `job.canceled` per canceled job, payload
  `{reason, workflow_job_id, cascade}`. Cascaded children carry
  `reason="cascade:<original>"` matching the CLI.
- **Locks:** head FOR UPDATE; no per-row FOR UPDATE needed because
  the cascade is fully inside one serializable snapshot.

### 4.4 `process_reconcile` → `handlers/recovery.py::process_reconcile`

- **Tables:** `striatumd.process_executions` (SELECT state='running'
  rows then UPDATE state='lost' for PIDs that no longer answer
  `os.kill(pid, 0)`; the `_pid_alive` helper at `recovery.py:693-711`
  ports verbatim), `striatumd.blockers` (INSERT
  `process_lost_with_outputs_missing` per the
  `evaluate_and_block_after_reconcile` port), `striatumd.jobs`
  (UPDATE state=blocked when output validation fails),
  `striatumd.events` (INSERT `process.reconciled_lost` per row).
- **Transaction:** **one tx per reconciled row**, not one tx for the
  whole sweep. RFC 0033 §6 says "long-running daemon operations
  release transactions promptly." The CLI version already mirrors
  this shape — it opens a fresh `with transaction(conn):` block
  inside the per-row loop (`recovery.py:653`). We preserve that.
- **Audit events:** `process.reconciled_lost` payload
  `{process_id, job_id, pid, blocker_kind}` per row.
- **Locks:** head FOR UPDATE per inner tx; the
  `striatumd.process_executions` row uses an implicit row lock
  on UPDATE.

### 4.5 `resume_blocker` → `handlers/recovery.py::resume_blocker`

- **Tables:** `striatumd.blockers` (UPDATE state=resolved,
  resolved_at), `striatumd.jobs` (UPDATE state=running),
  `striatumd.leases` (UPDATE last_heartbeat_at, expires_at for
  the active lease — the `--extend-seconds` semantics from
  `recovery.py:506-516`), `striatumd.events` (INSERT
  `recovery.process_blocker_resolved` or
  `recovery.blocker_dismissed_terminal` for the GH #7 force path),
  and optionally `complete_job` machinery when `--complete` is set
  (delegated to the Track A `complete_job` handler).
- **Transaction:** SERIALIZABLE; head-lock; if `--complete` is
  requested, the call into `complete_job` runs as the same tx (the
  shared `pg_conn` does not start a sub-tx).
- **Audit event:** `recovery.process_blocker_resolved` payload
  `{blocker_id, blocker_kind, verb, force, completed_inline,
   missing_artifact_paths, review_verdict_missing,
   lease_extended_until, original_envelope}` matches the CLI shape
  at `recovery.py:528-548`.
- **Locks:** head FOR UPDATE; `striatumd.blockers` UPDATE acquires
  the row lock.

### 4.6 `auto_publish_stale_artifacts` → `handlers/recovery.py::auto_publish_stale_artifacts`

This is the largest port. The CLI version (`recovery.py:731-984`)
performs `expire_leases` (write), then walks candidate stale
leases, reads the on-disk artifact file under `repo`, validates the
byline against the expected author line, and on match invokes
`ack_work`, `publish_artifact`, and `complete_job` to drive the job
to completion on the dead session's behalf. The dry-run path is
strictly read-only (GH #11 in `recovery.py:761-789`).

- **Tables:**
  - Dry-run path: read `striatumd.jobs`, `striatumd.leases`,
    `striatumd.queue_messages` only; no UPDATE; no event.
  - Live path: `striatumd.leases` (UPDATE state=expired via the
    same lazy port as `stale_leases`), `striatumd.queue_messages`
    (UPDATE state=acked via the `ack_work` port),
    `striatumd.artifacts` (INSERT via the `publish_artifact`
    port; the artifact append-only trigger at
    `0005_repo_local_workflow_state.sql:457-465` still applies),
    `striatumd.jobs` (UPDATE state=completed via the `complete_job`
    port), `striatumd.events` (one `recovery.auto_published` per
    successful auto-publish, plus the constituent
    `work.acked`/`artifact.published`/`job.completed` events from
    the delegated calls).
- **Transaction:** **one tx per auto-publish candidate**, same as
  CLI (`recovery.py:951-963`). The `expire_leases` sweep is a
  separate tx that runs first. A failure inside one candidate's tx
  does not stop the sweep.
- **Audit events:** `recovery.auto_published` payload
  `{workflow_job_id, artifacts, byline}` plus the delegated events.
- **Locks:** head FOR UPDATE per inner tx; the
  `uq_active_resource_lease` partial index forces serial publishing
  per job; concurrent operator-driven `recovery auto` invocations
  conflict on the lease and one retries cleanly.

### 4.7 `evidence_export` → `handlers/evidence.py::evidence_export`

- **Tables (read-only mutation of FS):**
  `striatumd.runs`, `striatumd.workflow_snapshots`,
  `striatumd.jobs`, `striatumd.job_dependencies`,
  `striatumd.artifacts`, `striatumd.sessions`, `striatumd.verdicts`,
  `striatumd.blockers`, `striatumd.events` (no writes).
  Filesystem: WRITE the rendered Markdown at the requested
  `path_text` under `repo`.
  `striatumd.events`: INSERT one `evidence.exported` row.
- **Transaction:** REPEATABLE READ (not SERIALIZABLE — no contended
  audit-head writes other than the single trailing
  `evidence.exported` event), wrapping all SELECTs so the rendered
  snapshot reflects one consistent MVCC view. The snapshot digest
  (`sha256(body)`) is computed from that view; concurrent writers
  cannot alter the digest mid-render. The trailing event is in a
  second small SERIALIZABLE tx after the FS write committed,
  carrying the digest.
- **Audit event:** `evidence.exported` payload
  `{path, sha256}` — matches the CLI at `evidence.py:377-382`.
- **Locks:** none beyond MVCC snapshot acquisition; the audit-event
  tx takes the head lock per the cross-cutting policy. The
  redaction registry (`evidence.py:48-245`) ports unchanged because
  it operates on Python dicts after the DB read.

Deterministic-digest gotcha: the CLI evidence snapshot today
orders DB reads by `created_at` (e.g. `evidence.py:399, 405`) and
`repo_path` (e.g. `evidence.py:519`). The PG port preserves those
ORDER BY clauses verbatim so the redacted JSON
(`json.dumps(snapshot, indent=2, sort_keys=True)` at
`evidence.py:628`) renders byte-identically given the same
underlying rows. The REPEATABLE READ snapshot guarantees the row
set itself is stable.

## 5. Test plan (concrete file paths)

`tests/daemon_pg/handlers/`:

- `register_session_test.py` — parity + HARNESS-003 reviewer
  refusal path.
- `claim_next_test.py` — parity + concurrent claim race (two
  handlers competing for the same queue row, only one wins).
- `ack_work_test.py` — parity + ownership mismatch refusal.
- `complete_job_test.py` — parity + downstream enqueue +
  `maybe_complete_run` transition.
- `release_lease_test.py` — parity.
- `block_job_test.py` — parity + EVIDENCE redaction policy
  agreement on audit payload.
- `record_verdict_test.py` — parity + idempotency on duplicate
  submission.
- `submit_review_test.py` — parity.
- `override_review_verdict_test.py` — parity + audit chain
  preserves both rows.
- `stale_leases_test.py` — parity, repo-write classification
  matches `cli/recovery.py::is_repo_write`.
- `requeue_stale_test.py` — parity + already-reclaimable branch.
- `cancel_job_test.py` — parity + cascade-of-three (recursive
  dependents).
- `process_reconcile_test.py` — parity, including the
  PID-not-found path mocked via `monkeypatch` on the PG handler's
  `_pid_alive` import.
- `resume_blocker_test.py` — parity + GH #7 terminal-job force
  path + `--complete` inline-completion path.
- `auto_publish_stale_artifacts_test.py` — parity dry-run +
  parity live path + skip-on-byline-mismatch.
- `evidence_export_test.py` — parity Markdown body + deterministic
  digest under concurrent writers (start a second tx that inserts
  a benign event mid-export, assert the export digest is unchanged).

A shared fixture in `tests/daemon_pg/handlers/conftest.py` builds
the per-test pair of `(sqlite_conn, pg_conn)` using the existing
`tests/_harness/pg.py::create_ephemeral_database` (see `pg.py:56-75`)
plus `tests/conftest.py::tmp_repo` for the SQLite side. The
`reset_daemon_db` helper (`pg.py:96+`) is invoked between cases to
keep ephemeral DB churn low.

Gate the entire `tests/daemon_pg/handlers/` directory on the
`STRIATUM_PG_TEST_URL` / `STRIATUM_MULTI_REPO_REQUIRE_PG` sentinel
already used by `tests/_harness/pg.py`. RFC 0048 §Open Questions #1
calls this out explicitly.

## 6. Router migration steps (sequencing)

To keep each PR reviewable and revert-safe:

1. **Land scaffolding only** — empty
   `src/striatum/daemon_pg/handlers/{__init__.py, _scope.py,
   workflow_loop.py, recovery.py, evidence.py}` plus
   `tests/daemon_pg/handlers/conftest.py` and the audit-head
   schema migration `0006_repo_local_audit_chain_heads.sql`. No
   router change; no behavior change.
2. **Per-method commit** — implement the handler, its
   parity test, and append one entry to `NATIVE_PG_HANDLERS` in
   `daemon_rpc/server.py`. Each commit removes one SQLite
   delegation path; each is independently revertible.
3. **Cluster gate-flip** — when all workflow_loop entries are in,
   add a `tests/daemon_pg/handlers/cluster_workflow_loop_e2e.py`
   that runs an end-to-end run through native handlers only.
   Repeat for recovery + evidence.
4. **Phase A close-out** — confirm the SQLite delegation path in
   `_route` is unreached except behind the
   `STRIATUM_DAEMON_REQUIRED=0` escape. Phase C (a later dogfood)
   removes the fallback entirely.

## 7. Deferred (named, not solved)

- **Unix-socket accept-loop gap** in
  `src/striatum/daemon.py::run_daemon_foreground` — RFC 0048
  Out-of-scope; only acknowledged here. The router today is reached
  via in-process `RpcEnvelope` construction in tests; production
  CLI verbs that need network transport remain blocked by this gap.
- **Go-core parity (RFC 0048 Phase B)** in `go/pkg/rpc/`,
  `go/pkg/apply/` — separate dogfood. The Python `NATIVE_PG_HANDLERS`
  table is the reference contract Phase B mirrors.
- **SQLite removal (RFC 0048 Phase C)** — removes the legacy
  fallback in `_route` and the
  `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` escape.
  Separate dogfood.
- **Bundled Postgres distribution** (RFC 0033 §8, RFC 0043
  Non-Goals) — out of scope.
- **Multi-tenancy `tenant_id` column** (RFC 0043 Non-Goals) —
  schema is `repository_id`-keyed today; a later RFC adds tenant
  scoping without re-litigating the substrate.
- **Cross-implementation parity matrix** (RFC 0048 §Acceptance
  Phase B) — Phase B prerequisite; the parity-test scaffold in §5
  is the Python-side anchor it will extend.

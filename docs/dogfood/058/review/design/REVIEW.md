---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["rfc-0048", "v1-5", "design-review", "ergonomics_dx", "track-boundaries"]
---
author: reviewer-unknown-model-002

# Design Review: RFC 0048 V1.5 Fix-up Scope Lock (attempt 2)

Target: `docs/dogfood/058/DESIGN_SYNTHESIS.md` (designer-unknown-model-001).
Posture: developer-ergonomics (`ergonomics_dx`), fresh context.
Inputs read (per prompt instruction):
- `docs/dogfood/058/DESIGN_SYNTHESIS.md`
- Cited code surfaces under `src/striatum/daemon_pg/`,
  `src/striatum/daemon_rpc/`, `src/striatum/daemon.py`,
  `src/striatum/cli/`, `tests/daemon_pg/handlers/`,
  `src/striatum/daemon_pg/sql/`, `docs/POSTGRES_TRANSITION.md`.
- Prior-round V1 reviews of dogfood-057:
  `docs/dogfood/057/review/build/codex/REVIEW.md`,
  `docs/dogfood/057/review/build/claude/REVIEW.md`.
- The three 058 design inputs were **not** read.

## Verdict

`needs_revision`. Mandatory checks 1, 2, 3, 4, 5, and 7 pass.
Mandatory check 6 — **track-boundary non-conflict** — fails on
two distinct grounds:

- **Finding #B1 (HIGH)**: Track B section B5 directly mutates
  `src/striatum/daemon_rpc/registry.py`, one of the three files the
  prompt names as Track-B-forbidden. The synthesis's own preamble
  (L12) assigns "router" to Track A, which contradicts B5.
- **Finding #B2 (HIGH)**: Track A section A2 mandates per-handler
  `FOR UPDATE` row-locking and (via the new `read_only=True`
  registry flag) decorator updates inside every mutating handler in
  `recovery_evidence/` — a Track-A-forbidden directory. Neither
  Track A nor Track B is named as the owner of those per-handler
  edits, so the integration plan is silently incomplete.

The synthesis is otherwise concrete and per the prompt's
ergonomics_dx posture passes on the four affordance bullets
(doctor `--explain`, runbook copy-pastability, parity diff
readability, dead-code per-symbol decisions). Findings #E1–#E3
below capture three lower-severity follow-ups that should be
addressed in revision but do not by themselves bounce.

## Mandatory checks (prompt §"Mandatory checks (bounce on any failure)")

### Check 1 — All 6 V1 findings explicitly addressed — PASS

| V1 finding | Synthesis section + lines | Concrete file | Function/symbol | Concrete test |
|---|---|---|---|---|
| codex F1 fail-closed | A1, L20–L51 | `src/striatum/daemon_pg/handlers/registry.py`; `src/striatum/daemon_rpc/server.py` | `is_pg_backed(method)`; `DaemonRpcRouter._route` checks `is_pg_backed` before `CLI_ROUTES`; error codes `daemon_db_missing` (exit 10) / `repo_not_registered` / `handler_failed` (exit 1); no PG path calls `striatum.api.invoke`, `striatum.db.connect`, or `CLI_ROUTES`. | `tests/daemon_rpc/test_fail_closed_routing.py` — parameterized over all 16 Phase A methods; monkeypatches each PG handler to raise; monkeypatches `striatum.api.invoke` and `striatum.db.connect` to fail the test on call. |
| codex F2 cap-denial | B2, L196–L228 | `tests/daemon_pg/handlers/_denial.py` + 16 handler test files | `assert_denied_without_workflow_mutation(method, params, denial_case, *, router, pg_ctx)`; `DenialCase` enumerates six cases; per-handler `test_replay_after_success`. | One case per handler in each of the 16 test files plus the per-handler replay test. |
| codex F3 chain-lock | A2, L54–L86 | `src/striatum/daemon_pg/handlers/context.py`; `src/striatum/daemon_rpc/server.py` | `RepoHandlerContext.write_transaction(*, retries=1)` opens `BEGIN ISOLATION LEVEL SERIALIZABLE`, retries one `SerializationFailure`; `append_event()` locks `repo_event_chain_heads … FOR UPDATE`; router wraps every PG write in `ctx.write_transaction()`; `register_pg_handler(*methods, read_only=False)` flag. | `tests/daemon_pg/handlers/test_event_hash_chain_concurrent.py` — overlapping allowed/denied across `work.claim_next`, `artifact.publish`, `review.verdict`, `work.complete`, `recovery.requeue_stale`; verifies one contiguous per-repo chain and no orphan mutations. |
| codex F4 append-only grants | A4, L111–L151 | `src/striatum/daemon_pg/sql/0007_daemon_role_grants.sql` | DO-block guard verifying `striatumd_rw` exists; explicit `REVOKE UPDATE, DELETE` on `audit_log`, `events`, `artifacts`; `REVOKE DELETE` on `repo_event_chain_heads`. | `tests/daemon_pg/test_role_grants.py` (privilege denial as `striatumd_rw`) + `tests/daemon_pg/test_handler_no_upsert_on_append_only.py` (static `ON CONFLICT DO UPDATE` scan). |
| claude HIGH#1 parity rig | B1, L155–L196 | `tests/daemon_pg/handlers/conftest.py` (moved from `recovery_evidence/conftest.py`); `tests/daemon_pg/handlers/_parity.py` | `Seed` dataclass + `parity_seed` fixture + `state_snapshot(...)` + `assert_state_parity(pg_state, sqlite_state) -> None` raising per-row/column diff; normalizes generated ids, timestamps, `repository_id`, chain-hash columns; **all** `RFC0048_PARITY` skip gates removed (synthesis L174). | One `test_parity_with_sqlite` per handler in each of the 16 test files (enumerated L177–L193). |
| claude HIGH#2 dead code | B4, L274–L329 | `src/striatum/daemon_pg/handlers/workflow_loop/complete_job.py`; `…/ack_work.py`; `…/recovery_evidence/resume_blocker.py`; `…/recovery_evidence/auto_publish_stale_artifacts.py` | `complete_inline(ctx, *, session_id, job_id, lease_id, summary, force) -> dict[str, Any]`; `ack_inline(ctx, *, session_id, message_id, lease_id) -> None`; replace local `_complete_inline` import; one-transaction ack→publish→complete in `auto_publish_stale_artifacts`. | `test_resume_with_complete_runs_inline`; `test_live_mode_ack_publish_complete_rolls_back_on_failure`; `grep -rn 'InvalidTransitionError.*requires Track A' src/striatum/` zero-hit gate (L331). |

All six V1 findings reach a concrete file+function/symbol+test. Pass.

### Check 2 — Accept-loop design is concrete — PASS

- **Function:** `run_daemon_foreground(*, sweep_interval_seconds: float = 60.0, max_sweeps: int | None = None, postgres_url: str | None = None) -> dict[str, Any]` (synthesis L92; matches current signature at `src/striatum/daemon.py:879`).
- **Concurrency model — picked, not enumerated:** *threading*. One accept thread bound by `threading.Event`; one daemon thread per accepted connection; `sock.accept()` with short timeout to poll the stop event (synthesis L96–L98).
- **Bytes flow CLI → response (exact path):** `striatum.daemon_rpc.transport_unix.bind_unix_socket(socket_path())` (owner-only, `0600`) → accept thread `sock.accept()` → connection thread wraps `conn.makefile("rwb")` → `striatum.daemon_rpc.framing.read_envelopes(stream)` → `DaemonRpcRouter.handle(envelope, connection_id=<uuid>, transport="unix", require_handshake=True)` → `striatum.daemon_rpc.framing.write_response(stream, response)` (synthesis L94–L100).
- **Handshake enforcement:** existing connection-local handshake set in the router (synthesis L99).
- **Shutdown:** SIGTERM/SIGINT sets stop event → closes listener (breaks `accept()`) → joins accept thread briefly → closes live connection threads best-effort → unlinks socket + pid files in the existing `finally` (synthesis L102).
- **Router construction site:** `DaemonRpcRouter(pg_conn=<daemon PG connection>, repo_root=<registered active repo root>, substrate_schema=<doctor version>)` created in `run_daemon_foreground` after `daemon_pg.connection.doctor(..., apply=True)` succeeds (synthesis L104).
- **Test path:** `tests/daemon_rpc/test_unix_daemon_foreground.py` (synthesis L106). Asserts `daemon.hello` then a registered-repo `status` call route through `DaemonRpcRouter._route`, with audit/request-log row showing `transport="unix"`.

Function name, picked concurrency model, end-to-end bytes path, and test path are all named. Pass.

### Check 3 — Schema migration 0006 is byte-equivalent for existing rows — PASS

- **DDL** (synthesis L237–L262): adds `striatumd.events.previous_hash`, `striatumd.events.row_hash`, table `striatumd.repo_event_chain_heads`, unique index `ux_events_repository_row_hash(repository_id, row_hash) WHERE row_hash IS NOT NULL`, and `ix_events_repo_chain(repository_id, event_id)`. The same-transaction `UPDATE` populates the new top-level columns from the existing `payload_json->'_event_chain'` JSON values via `decode(..., 'hex')` — this is byte-identical to what `context.append_event` writes today.
- **Re-anchor algorithm** (synthesis L264–L266): `src/striatum/daemon_pg/migrations.py::reanchor_event_chain_0006(conn)` runs **inside** the migration transaction after the DDL. Orders rows by `(repository_id, event_id)`, calls `canonical_event_hash(...)` for any remaining missing `row_hash` rows, fills `previous_hash`, sets `row_hash NOT NULL`, backfills one `repo_event_chain_heads` row per repository from the trailing event, bumps `schema_migrations`. Idempotent — "rows with `row_hash` already present are verified, not rewritten" (L266).
- **Sequential placement** (L230): "Sequential version 0006 is unused on main today, so there is no ordering conflict." Confirmed via `ls src/striatum/daemon_pg/sql/` — current tip is 0005.
- **Tests** (L269–L271): `test_migration_0006_backfill.py` seeds 0005-style `_event_chain` payload rows, applies 0006, asserts top-level columns match byte-for-byte; `test_event_hash_chain.py` asserts the chain invariant (first event `previous_hash IS NULL`, multi-event single-transaction append, head row advances).

Re-anchor algorithm specified, idempotent guard explicit, byte-equivalence asserted in the named test. Pass.

### Check 4 — Parity rig has a real diff helper — PASS

- **Helper:** `assert_state_parity(pg_state: Mapping[str, Any], sqlite_state: Mapping[str, Any]) -> None` (synthesis L168).
- **Output shape — per-key, not raw dict-vs-dict:** "raises an `AssertionError` with table, primary key, column, SQLite value, and PG value for every mismatch" (L172). Operator reading a CI failure can locate the exact table, row, and column that drifted between substrates.
- **Normalization rules:** generated ids, timestamps, `repository_id`, chain-hash columns normalized before compare (L172). These are exactly the columns that diverge under happy-path inputs without normalization — appropriate.
- **Skip-gate removal:** "Remove every `RFC0048_PARITY` skip gate" (L174). The only remaining skip is the repository-wide `STRIATUM_TEST_POSTGRES_URL` skip. Confirms current `tests/daemon_pg/handlers/recovery_evidence/test_stale_leases.py` / `…/test_requeue_stale.py` `@pytest.mark.skipif(... RFC0048_PARITY ...)` blocks are slated for removal.
- **Wired per-handler:** every one of the 16 handler test files gets a `test_parity_with_sqlite` (L177–L193).

Function name, output shape, normalization rules, and per-handler wiring all named. Pass.

### Check 5 — Capability-denial enumerates all 6 cases per handler — PASS

Synthesis B2 enumerates exactly the six prompt-required denial cases per handler (L218–L223):

| Prompt case | Synthesis label | Mapped RPC error code |
|---|---|---|
| missing token | missing token | `token_missing` |
| revoked token | revoked token | `token_revoked` |
| expired token | expired token | `token_expired` |
| wrong required capability | wrong required capability | `capability_required` |
| wrong repository scope | wrong repository scope | `repo_not_registered` |
| replay | replayed `request_id` | `duplicate_request` |

Each case "snapshots workflow tables, `striatumd.events`, `striatumd.artifacts`, and `striatumd.audit_log`; calls `DaemonRpcRouter.handle(...)`; asserts an RPC error with the expected code; asserts no workflow/event/artifact mutation; and asserts exactly one denied audit row for parseable envelopes" (L224–L225). Wired into all 16 handler test files; a separate `test_replay_after_success` per handler covers replay-after-commit (L227–L228).

All six cases enumerated; all 16 handlers covered; replay tested per-handler. Pass.

### Check 6 — Track boundaries don't conflict — FAIL

The prompt fixes track boundaries as:
*Track A doesn't touch `recovery_evidence/`;*
*Track B doesn't touch `daemon.py` / `daemon_rpc/server.py` /
`daemon_rpc/registry.py`.*

The synthesis's own preamble (L12) also locks the boundary at this
shape: "Track A owns router, transport, handler transaction
discipline, and SQL privileges. Track B owns parity and denial
tests, schema migration, dead-code cleanup, doctor UX, and the
Postgres transition runbook."

The synthesis violates this on two grounds.

#### Finding #B1 — Track B section B5 edits `src/striatum/daemon_rpc/registry.py` (HIGH, mandatory-check failure)

- **Evidence (synthesis L333–L344):**
  > "File: `src/striatum/daemon_rpc/registry.py`. Extend
  > `MethodEntry.public_dict(self) -> dict[str, object]` to include:
  > `pg_backed`, `sqlite_fallback_active`, `substrate`."
  This puts a Track B section directly inside one of the three
  files the prompt names as Track-B-forbidden.
- **Internal inconsistency:** synthesis L12 assigns "router" to
  Track A. `daemon_rpc/registry.py` is part of the router/registry
  surface. B5 contradicts the synthesis's own preamble without a
  carve-out clause.
- **Discoverability cost (ergonomics_dx):** a first-time
  contributor reading B5 finds the `MethodEntry.public_dict`
  extension under Track B, but A1 already lands the
  `is_pg_backed(method)` predicate in the sibling
  `daemon_pg/handlers/registry.py`. The two changes are intimately
  coupled — `MethodEntry.public_dict` cannot compute `pg_backed`
  without importing `is_pg_backed` — and the synthesis does not
  name the import ordering between Track A and Track B. The
  developer cannot follow either track in isolation, which
  contradicts the synthesis's own "Track A can land independently"
  claim (L16).
- **Required change (one of):**
  (a) Move the `MethodEntry.public_dict` extension to Track A
  (lands alongside A1's `is_pg_backed` and Track A's existing
  edits in `daemon_rpc/server.py`). B5 retains only `cli/parser.py`
  and `cli/introspect.py::explain_daemon_methods()`. Or
  (b) Have `cli/introspect.py::explain_daemon_methods()` derive
  `pg_backed` / `substrate` directly by importing
  `striatum.daemon_pg.handlers.registry._PG_HANDLERS.keys()` (or a
  public wrapper around it) so that `MethodEntry` is never
  touched. The synthesis must pick one and state it explicitly.

#### Finding #B2 — Track A section A2 implicitly edits handlers in `recovery_evidence/` (HIGH, mandatory-check failure)

- **Evidence (synthesis L82):**
  > "Every mutating handler in `workflow_loop/` and
  > `recovery_evidence/` must call `append_event()` inside this
  > transaction and must lock the run/job/message/lease rows it
  > mutates with `FOR UPDATE`."
  Adding a `FOR UPDATE` clause to the SQL inside each mutating
  `recovery_evidence/` handler (`cancel_job.py`,
  `requeue_stale.py`, `resume_blocker.py`,
  `auto_publish_stale_artifacts.py`) is necessarily an in-file
  edit to the forbidden directory.
- **Compounding:** synthesis L67–L69 adds a
  `read_only: bool = False` kwarg to `register_pg_handler(...)`.
  Read-only recovery handlers (`stale_leases`, `evidence_export`,
  `process_reconcile`) must flip their decorator to
  `read_only=True`. That, too, is an in-file edit to
  `recovery_evidence/`.
- **Owner gap:** Track A nominally owns "handler transaction
  discipline" (L12), which motivates both edit passes, but is
  forbidden from `recovery_evidence/`. Track B nominally owns
  parity/denial/test wiring in those same files, but the synthesis
  does not extend B's scope to the source-side `FOR UPDATE` and
  `read_only=True` flips. Neither track is named as the owner.
- **Ergonomics_dx consequence:** a build-time reviewer who
  assesses Track A's PR against "Track A doesn't touch
  `recovery_evidence/`" will reject correct work. A reviewer who
  assesses Track B's PR against "Track B doesn't touch
  `daemon.py` / `daemon_rpc/`" cannot verify the chain-lock and
  `read_only=True` invariants without Track A's infrastructure
  landing first — and the synthesis claims (L16) Track A and
  Track B can develop in parallel, which is now false at the file
  level.
- **Required change:** explicitly split the integration plan so
  that:
  - Track A ships **infrastructure-only**: `write_transaction`,
    the `read_only=False` registry kwarg in
    `daemon_pg/handlers/registry.py`, the router-side
    `ctx.write_transaction()` wrapping in `daemon_rpc/server.py`,
    and the `append_event()` chain-lock in
    `daemon_pg/handlers/context.py`. **No edits to
    `recovery_evidence/` files.**
  - Track B (or a named third track) owns the per-handler edits:
    decorator `read_only=True` flips for `stale_leases`,
    `evidence_export`, `process_reconcile`, and the per-handler
    `FOR UPDATE` clauses in `cancel_job`, `requeue_stale`,
    `resume_blocker`, `auto_publish_stale_artifacts`.
  - The synthesis must name the owner of every handler-file edit
    it transitively requires, including the `workflow_loop/`
    handler `FOR UPDATE` clauses (the synthesis is silent on
    whether Track A's A2 or Track B's B1/B2 owns those, even
    though `workflow_loop/` is not boundary-forbidden for either
    track).

### Check 7 — No 'TODO' or 'see V1.6' on a non-negotiable — PASS

The synthesis defers only Phase B (Go core parity), Phase C (full
SQLite-fallback removal), hosted mode, bundled Postgres,
multi-tenancy, and broad product-doc rewrites outside
`POSTGRES_TRANSITION.md` (L457–L459). All non-negotiable
items in mandatory checks 1–6 have a V1.5 landing site. No `TODO`
or `see V1.6` deferral appears against a mandatory check. Pass.

## Ergonomics_dx checks (degrade verdict, do not bounce)

### Finding #E1 — `daemon doctor --explain` surface is operator-actionable (KEEP)

Synthesis B5 (L336–L389) supplies the surface a first-time
operator needs:

- **JSON shape** (L362–L376): `method_substrates[]` per-method
  entries with `pg_backed`, `sqlite_fallback_active`,
  `substrate ∈ {pg, cli_fallback, native_daemon}`,
  `required_capability`, `repository_scope_mode`; plus a `totals`
  object aggregating `pg_backed`, `sqlite_fallback_active`,
  `native_daemon` counts.
- **Text table** (L380–L385): one row per method with columns
  `method`, `pg-backed`, `sqlite-fallback`, `capability`, `scope`.
  Example rows cover a Phase A method (`work.ack`), a still-SQLite
  method (`run.summary`), and a native daemon method
  (`daemon.describe`) — so a first-time operator can immediately
  classify any method without reading source.
- **Acceptance test:** `tests/daemon_rpc/test_doctor_explain.py`
  asserts all 16 Phase A methods report `pg_backed=true`, no
  Phase A method reports `sqlite_fallback_active=true`, and row
  count matches `METHOD_REGISTRY` (L389).

This is a discoverable affordance. **Discoverability does
depend on resolving finding #B1** — until the track ownership of
the `MethodEntry.public_dict` extension is fixed, a contributor
cannot follow the per-track diff to find where `pg_backed` is
populated. Pass on the surface; gated on #B1 for ownership.

### Finding #E2 — `POSTGRES_TRANSITION.md` runbook is copy-pasteable (KEEP)

Synthesis B6 (L391–L431) addresses claude-057 finding #6
(runbook gave no indication Phase A had shipped). The runbook
artifacts a first-time operator needs are all named:

- **Anchor heading** `## Provision the daemon-required role`
  (L397–L399) matches the doctor refusal hyperlink
  `docs/POSTGRES_TRANSITION.md#provision-the-daemon-required-role`.
- **Copy-pasteable SQL block** (L403–L410): `CREATE ROLE
  striatumd_rw …`, `GRANT CONNECT`, `GRANT USAGE ON SCHEMA`,
  `GRANT USAGE, SELECT ON ALL SEQUENCES`, with an explicit note
  that migration 0007 grants table-level privileges and the
  operator must not do that manually. That note is
  precisely the kind of "do not do this twice" hint a first-time
  operator following the runbook needs.
- **Verbatim doctor refusal text** (L414) so an operator can
  grep their daemon output straight back to the runbook section.
- **Exit-code row** (L428–L429) for code `13 daemon_role_missing`
  with the remediation `striatum daemon doctor --apply-migrations`.
- **Rewritten section** `## RFC 0048 status (v1.49.0+)`
  (L418–L424) which must enumerate the 16 landed PG-backed
  methods and the four V1.5 hardening axes (fail-closed routing,
  parity, denial, chain columns, role grants). Confirmed via
  `grep -n -E "RFC 0048|Provision|striatumd_rw" docs/POSTGRES_TRANSITION.md`
  that the existing "## RFC 0048 remaining work" section (current
  line 248) is the rewrite target and no `striatumd_rw` references
  exist yet, so B6 lands without conflict.

Copy-pasteable. Pass.

### Finding #E3 — Parity rig diff is readable (KEEP)

`assert_state_parity` (synthesis L168, L172) raises an
`AssertionError` keyed by **table, primary key, column, SQLite
value, PG value** — the per-key shape the prompt's ergonomics_dx
bullet required (no "raw dict-vs-dict"). Normalization rules
(generated ids, timestamps, `repository_id`, chain-hash columns)
match exactly the columns that would diverge under valid happy-
path inputs without normalization, so a CI failure points the
maintainer at *which row, which column* drifted, not at noise.
Pass.

### Finding #E4 — Dead-code decisions are justified per-symbol (KEEP)

Synthesis B4 (L271–L329) makes a KEEP decision **per symbol**
with one-sentence justifications:

- `complete_inline`: define and wire — "`recovery.resume
  --complete` and `recovery.auto` live mode are real operator
  recovery paths; deleting them would remove documented
  functionality" (L277–L278).
- `ack_inline`: define and wire — "`recovery.auto` live mode
  needs idempotent ack before publish/complete" (L279–L280).
- `recovery.resume --complete`: keep — "resolves the blocker
  and completes the job in one transaction" (L280–L281).
- `recovery.auto`: keep — "dry-run is preview; live mode is the
  recovery primitive" (L281–L282).

No bulk "delete all this" decision. The per-symbol decisions are
backed by the `grep -rn 'InvalidTransitionError.*requires Track A'
src/striatum/` zero-hit gate (L331), so CI signal exists if any
of the four symbols regresses to its dogfood-057 dead state.
Pass.

### Finding #E5 — Read-only handler set is not enumerated (LOW, follow-up)

`register_pg_handler(*methods: str, read_only: bool = False)`
(synthesis L67–L69) adds the flag but the synthesis does not
list **which** of the 16 Phase A methods are read-only and
therefore need `read_only=True` at the decorator call site. From
a first-time-reader perspective this leaves the integration step
under-specified: a reviewer cannot verify that
`recovery.stale_leases`, `evidence.export`,
`recovery.process_reconcile` (etc.) are correctly tagged, nor
can the `daemon doctor --explain` `totals.pg_backed` /
`sqlite_fallback_active` counts be predicted in advance for the
acceptance assertion in `test_doctor_explain.py` (synthesis
L389).

This compounds with finding #B2 because the per-handler
`read_only=True` flips are part of the unowned edit pass.

**Required follow-up (post-revision):** enumerate the read-only
subset of Phase A methods inside A2 (or B5), e.g.:
`recovery.stale_leases`, `evidence.export`,
`recovery.process_reconcile`.

### Finding #E6 — Divergent PG primitives surfaced by claude-057 #3 not addressed by name (LOW, follow-up)

The dogfood-057 claude review (finding #3, MEDIUM) flagged that
`maybe_complete_run`, `expire_leases`, and `is_repo_write` /
`is_repo_write_scope` each exist twice with subtly different
semantics (Track A `context.maybe_complete_run` emits
`stop_reason="all_jobs_canceled"` / `session.closed.reason
="run_canceled"`; Track B `recovery_evidence/_sql.py` emits
`stop_reason="job_canceled"` / `session.closed.reason
="run_terminal"`).

The V1.5 synthesis does not surface or unify these. The new
byte-equivalence parity rig (B1) will *expose* the drift in CI
(which is good — and is the principal V1.5 net win), but the
synthesis does not specify which copy becomes canonical, where
the survivor lives, or which dead duplicate is deleted. From an
ergonomics standpoint this remains a maintainer-cost hazard: a
developer touching either path still has to know which copy
applies.

**Required follow-up (post-revision):** add a one-paragraph
"unify divergent primitives" subsection under B4 (or a new B7)
naming the canonical site (likely `context.py`) and the
duplicates marked for deletion in V1.5. The synthesis may
explicitly defer **deletion** of the duplicates to V1.6, but
must at minimum lock the parity tests against the canonical
copy in V1.5.

## Summary

Mandatory checks 1, 2, 3, 4, 5, 7 pass. Mandatory check 6
(track-boundary non-conflict) fails on two distinct grounds —
finding #B1 (B5 edits `daemon_rpc/registry.py`, a
Track-B-forbidden file, in direct contradiction with the
synthesis's own L12 preamble) and finding #B2 (A2 implicitly
mandates per-handler `FOR UPDATE` and `read_only=True` flips
inside `recovery_evidence/`, a Track-A-forbidden directory,
without naming the owner of either edit pass). These are
bounce-level failures per the prompt.

Findings #E1–#E4 confirm the ergonomics_dx affordances land
when read in isolation: `daemon doctor --explain` is concrete
and operator-actionable, `POSTGRES_TRANSITION.md` is
copy-pasteable, the parity rig diff is per-key, and dead-code
decisions are justified per symbol with a CI gate. Findings
#E5 and #E6 are LOW follow-ups that should be folded into the
revision but do not by themselves bounce.

Re-review will lift verdict to `accept_with_findings` once:

1. **#B1** — the owner of the `MethodEntry.public_dict`
   `pg_backed` / `substrate` extension is explicitly assigned
   to Track A (option (a)) **or** `cli/introspect.py` derives
   the field from `_PG_HANDLERS.keys()` without touching
   `MethodEntry` (option (b)). The synthesis must pick one.
2. **#B2** — the per-handler `FOR UPDATE` and `read_only=True`
   edits inside `recovery_evidence/` and `workflow_loop/` are
   assigned to a named owner, with Track A scoped strictly to
   the infrastructure (`write_transaction`, registry kwarg,
   router-side wrapping, `append_event` chain-lock).
3. **#E5** — the read-only subset of Phase A methods is
   enumerated.
4. **#E6** — the divergent primitives are named, the canonical
   site is locked, and the parity tests are pointed at the
   canonical copy. Duplicate-deletion timing may stay V1.6 but
   must be stated explicitly.

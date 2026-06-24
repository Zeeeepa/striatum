# Design-Run Seed — RFC 0167 P0 (REVISION v4 — intended converging cycle)

> This is the **v4 revision** of the RFC 0167 P0 design run, and is intended to be
> the **converging** cycle. v3 RESOLVED the cores of both v2 constraints (a distinct
> `mintOperatorSessionToken` authorizes `run.prepare`; the direct `operator_sessions`
> identity read is closed) but the adjudicator found two new source-verified residuals
> (C1″, C2″) and routed them to the operator. The operator has analyzed both and
> prescribes the fixes below — including a **deeper composed read-scope route the v3
> falsifier did not name**. **Required context docs** (read in full first):
> - `docs/operator/artifacts/rfc-0167-p0-design-v3/dialogue/holder/HOLDER.md` — the v3 SPEC you are revising (the base).
> - `docs/operator/artifacts/rfc-0167-p0-design-v3/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the v3 verdict (C1″/C2″ findings + constraints).
> - `docs/operator/artifacts/rfc-0167-p0-design-v2/dialogue/holder/HOLDER.md` — the v2 SPEC (operator_sessions substrate + projection stamp — context).
> - `docs/rfcs/0167-operator-identity-and-run-attribution.md` — the accepted RFC (D260).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **consolidated, cleared,
falsifiable implementation spec for RFC 0167 P0** the downstream `rfc-0167-p0-build`
`code_change` run executes. It must **resolve C1″ and C2″** and **carry forward,
unregressed, everything cleared through v1+v2+v3**. The committer must publish the
WHOLE consolidated P0 SPEC (not just deltas), because the build run consumes it as the
single source of truth.

## Carried forward — CLEARED through v1+v2+v3 (do NOT reopen, do NOT regress)

- **R1a honesty**; **R1b architecture** (per-session leased word + live-unique partial
  index + write-once `created_by_handle_id` snapshot + the deterministic principal-seeded
  escalation walk); **v2 C1 storage substrate** (the dedicated owner-held
  `operator_sessions` table; `PostgresAuthorizer` resolves the bound session from
  `client_capabilities.session_id` without joining `striatumd.sessions`, `auth_pg.go:104-156`);
  **v2 C2 projection stamp** (run-origin principal resolved in Go via
  `admin.ResolvePrincipalForClient` → `resolve_principal_for_client`, bound `$N`; no direct
  `principal_clients.principal_id` read); **v3 C1′ authorization core** (`mintOperatorSessionToken`
  is the composition of `insertTokenClient`'s caller-supplied capability set with
  `mintSessionBoundToken`'s session binding; the lane slice `{claim,write,read,review}` is
  UNCHANGED so lane tokens cannot gain admin; close-revoke + TTL bound the stale-token stamp);
  **v3 C2′ direct-read closure** (`operator_sessions` column-scoped SELECT excluding
  `principal_id` and `client_id`); **R1c** guarded-UPDATE flap; **R2** `BEFORE UPDATE`
  write-once trigger + owner bundle ordinal 0021; **R3** four open questions; **R4** reuse.

## The two binding constraints to DISCHARGE

### C2″ — close the COMPOSED `client_id → principal_id` reconstruction by ANY route over the full ACL graph (critical; THE decisive blocker)

v3 closed the DIRECT `operator_sessions` read, but `striatumd_rw` still reconstructs the
mapping bundle 0006 / RFC 0114 closed. The v3 falsifier named ONE route; the operator
identifies a SECOND. The fix must close **both** (and prove no third remains by reasoning
over the join graph, not a single query):

- **Route 1 (falsifier-named):** `client_capabilities cc` (runtime-readable: `SELECT` on
  ALL TABLES, `0005:467-472`; carries `client_id` + `session_id`) `JOIN operator_handles oh
  ON oh.leased_session_id = cc.session_id` → reads `oh.principal_id` (v3 grants
  `operator_handles` FULL SELECT). Yields `(cc.client_id, oh.principal_id)`.
- **Route 2 (operator-identified, deeper):** `cc JOIN operator_handles oh ON
  oh.leased_session_id = cc.session_id JOIN runs r ON r.created_by_handle_id = oh.handle_id`
  → reads `r.created_by_principal_id` (runs is runtime-readable). Yields
  `(cc.client_id, r.created_by_principal_id)` — the SAME mapping, even if `oh.principal_id`
  is closed. (`oh.leased_session_id` and `oh.handle_id` CANNOT be revoked — the lease
  heartbeat `WHERE leased_session_id=$` and the stamp `WHERE leased_session_id=app.session_id`
  / `SELECT handle_id` need them. So the only column that closes Route 2 is
  `runs.created_by_principal_id`.)

**The prescribed fix (the established RFC 0114 projection pattern):**
- **Column-scope `operator_handles` SELECT** to the runtime to EXCLUDE `principal_id`
  (keep `handle_id`, `leased_session_id`, `handle`, `repository_id`, `released_at`,
  `leased_until`, `last_heartbeat_at` — all needed by the lease walk / lazy-expiry / heartbeat
  / stamp). Since `operator_handles` is NEW in 0021, this is a column-scoped GRANT (no REVOKE).
- **Column-revoke `runs.created_by_principal_id` SELECT** from the runtime:
  `REVOKE SELECT ON striatumd.runs FROM striatumd_rw; GRANT SELECT (<every other runs column,
  enumerated>) ON striatumd.runs TO striatumd_rw;` while KEEPING `INSERT` (the stamp writes
  the column; `INSERT(col)` is independent of `SELECT(col)`) and the existing
  `UPDATE`/`INSERT` DML. The `BEFORE UPDATE` write-once trigger reads `OLD/NEW` row variables
  (no column SELECT privilege needed), so it is unaffected. Verify NO daemon read path
  besides `whose`/`status --mine`/doctor-attribution SELECTs `created_by_principal_id`
  (enumerate the readers; the ordinary `status`/`dashboard`/`list runs` paths do not need it).
- **Route the identity-bearing reads through SECURITY DEFINER projections** (owned by the
  owner role, `EXECUTE` to `striatumd_rw`, mirroring `resolve_principal_for_client`): a
  `whose` projection returning `{run_id, state, handle, principal_kind, ...}` and the
  `status --mine` filter (runs whose origin principal = the live caller) resolved through a
  projection — so `created_by_principal_id` / `operator_handles.principal_id` are read by the
  DEFINER, never directly by `striatumd_rw`.

**Controls (two-role):** a **NEGATIVE control over the COMPOSED graph** — BOTH Route 1
(`cc ⋈ oh` on `principal_id`) AND Route 2 (`cc ⋈ oh ⋈ runs` on `created_by_principal_id`)
run as `striatumd_rw` must fail `42501` or return zero identity rows / be structurally
impossible; AND a **POSITIVE control** proving the projection-routed `whose` / `status --mine`
+ the run-origin stamp + lease heartbeat/close still work. The falsifier must reason over the
ACL graph for any THIRD route (any other table exposing `principal_id` to the runtime).

### C1″ — the operator-session admin token: justified-acceptance as the P0 boundary (critical)

v3's `{admin, read}` operator token authorizes `run.prepare`, but `CapabilityAdmin` is
coarse — the registry maps it to the whole repo-admin surface (`review.override`,
`checkpoint.resolve`, `decision.record`, `workflow.accept_risk`, `branch.confirm`,
`repo.init`), and the codebase's `verifier.attest` refuses session-bound admin
(`verifier_attest.go:49-59`). The operator analysis: **these admin verbs ARE the operator's
legitimate authority** — a real operator drives `checkpoint resolve`, `review override`,
`decision record`, `branch confirm` (this very campaign does). A "run-lifecycle-only"
capability would BREAK the operator's job. So the correct resolution is **explicit,
justified acceptance** (RFC ledger option (c)), hardened by (b):

- **Accept** the operator-session token carrying the operator's admin authority as the P0
  security boundary, with an **N-tokens blast-radius analysis**: each operator-session token
  is TTL-bounded (`leased_until`/token expiry) AND revoked on graceful session close
  (`client_capabilities.revoked_at`), so it is a **strictly-less-standing materialization** of
  the authority the human ALREADY holds via the static `0600` admin runtime token — fewer,
  not more, standing credentials over time; the per-terminal multiplicity is bounded by live
  operator sessions and each is independently revocable.
- **Harden (b):** confirm the existing `verifier.attest` `IsSessionBound()` refusal
  (`verifier_attest.go:49-59`) still protects the one trust-root route from the operator token
  (the operator token, being session-bound, is ALREADY refused at `verifier.attest`), and
  enumerate any other route that already refuses session-bound tokens — so the accepted admin
  surface is "the operator's authority MINUS the trust-root routes the codebase already
  fences from session-bound tokens." If any genuinely-dangerous-when-session-bound admin route
  exists that does NOT already refuse session-bound tokens, add the analogous refusal.
- **Control:** with a valid minted operator token, the representative run-lifecycle +
  operator-admin routes succeed, AND `verifier.attest` (and any other session-bound-refusing
  route) is refused typed; document the accepted surface as the justified P0 boundary.

## Falsifier guidance

- **Falsifier 1 (C2″ / composed read-scope lens):** Reason over the FULL ACL graph. Can
  `striatumd_rw` recover `client_id → principal_id` by Route 1, Route 2, or ANY third route
  (another table/view/function exposing `principal_id`)? Are the negative controls the actual
  COMPOSED queries (not a single-table read)? Do the projections + column grants still let
  `whose` / `status --mine` / the stamp / heartbeat work (positive control)? Does the
  `runs.created_by_principal_id` column-revoke break any other daemon read path (enumerate)?
- **Falsifier 2 (C1″ / token-scope + carry-forward lens):** Is the operator-admin acceptance
  genuinely justified (the blast-radius analysis sound; the trust-root refusals confirmed)?
  Is `verifier.attest` actually refused for the session-bound operator token? Then verify NO
  carry-forward regressed: R1a, the R1b architecture, the v2 storage + projection stamp, the
  v3 authorization core + direct-read closure, R1c, the write-once trigger, bundle ordinal
  0021, the four open questions, R4.

## Adjudication bar (converging cycle)

Clearing verdict (`accept` / `accept_with_findings`) requires: C2″ closes BOTH named composed
routes with the composed negative controls (this is the decisive, non-negotiable blocker — a
still-reconstructable `client_id → principal_id` is `needs_revision`); C1″ presents a sound
justified-acceptance with the trust-root refusal confirmed; and no carry-forward regressed.
**Prefer `accept_with_findings`** for any MINOR residual (a noted build-time refinement),
reserving `needs_revision` for a materially unclosed identity leak, a broken proof, or a
regressed carry-forward — this is the converging cycle and the build run needs a cleared,
consolidated SPEC. A second `needs_revision` exhausts the cycle and routes to the operator.

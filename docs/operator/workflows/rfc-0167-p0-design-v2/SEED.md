# Design-Run Seed — RFC 0167 P0 (REVISION v2)

> This is the **v2 revision** of the RFC 0167 P0 design run. The v1 run
> (`rfc-0167-p0-design`) ran this same falsification gate and returned
> `needs_revision` with two binding, source-verified constraints (C1, C2). This
> run revises the v1 SPEC to discharge both while carrying forward everything v1
> cleared. **Required context docs** (read in full first):
> - `docs/operator/artifacts/rfc-0167-p0-design/dialogue/holder/HOLDER.md` — the v1 SPEC you are revising (the base; do not rewrite from scratch).
> - `docs/operator/artifacts/rfc-0167-p0-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the v1 verdict; its `findings:` + `constraints:` blocks (C1/C2) are the exact prescribed fixes.
> - `docs/rfcs/0167-operator-identity-and-run-attribution.md` — the accepted RFC (D260).
> - `docs/operator/workflows/rfc-0167-p0-design/SEED.md` — the original charter (R1a–R4 requirements, unchanged).

## Charter — what this run must produce

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0167 P0** that the downstream `rfc-0167-p0-build`
`code_change` run executes. It must **resolve C1 and C2** and **carry forward,
unregressed, everything v1 cleared**. A revision that leaves C1 or C2 open — or
that regresses a carry-forward — has **not** cleared the gate.

## Carried forward — CLEARED by v1 (do NOT reopen, do NOT regress)

The v1 adjudicator explicitly DISCHARGED these; preserve them intact:

- **R1a honesty (v1 §1, A1–A5):** mint+lease in one transaction; the run-origin
  stamp comes from the live-token prelude GUC server-side, never an envelope
  param / tty / tmux / title / env; read surfaces are pure PG joins. No spoof
  path was found.
- **R1b ARCHITECTURE (v1 §2):** the disambiguation design is CORRECT and stays —
  a per-session leased **word** via `operator_handles` + the **live-unique
  partial index** `(repository_id, lower(handle)) WHERE released_at IS NULL`; a
  **write-once `runs.created_by_handle_id` snapshot** (FK -> `operator_handles`)
  joined by `whose` (NOT merely `created_by_principal_id`, whose suffix is
  identical for all of one human's terminals); the deterministic
  principal-seeded candidate walk for default + collision escalation (distinct
  curated words, not numeric `maya2`); A6–A11. Keep this whole architecture.
- **R1c flap renewal (v1 §3, A12):** heartbeat is a guarded UPDATE
  (`WHERE leased_session_id=$ AND released_at IS NULL`) that never transits a
  released state. Keep.
- **R2 DB-write-once face (v1 §4.3):** the `BEFORE UPDATE` trigger
  `refuse_run_origin_change()` (justified over a column REVOKE, 0010 precedent);
  owner bundle ordinal **0021** (verified `LatestOwnerBundleVersion == 20`);
  forward-only / watermark-consistent. Keep.
- **R3 open questions (v1 §5, A20–A23):** OQ1 curated pool + deterministic
  default/escalation + denylist-by-exclusion (in-P0); OQ2 NULL + advisory
  `attribution_unknown`, **no backfill** (justified — `branch_confirmed_by`
  carries `'daemon'`/`'human'`, not a `principal_id`); OQ3 per-repo in-P0,
  daemon-wide deferred to P3; OQ4 byline suffix out of P0 (P2). Keep.
- **R4 reuse (v1 §6):** FK rendering layer over `principal_id`, no parallel
  identity table, opaque `run_id`, product-boundary clean, P0-scoped. Keep —
  but C1's substrate and C2's stamp must STAY a reuse of RFC 0107/0114, not a
  rebuild or a punch-through.
- The four v1 source corrections **C-1..C-4** (owner-ownership forces the owner
  bundle; the GUC holds `client_id` not `principal_id`; there is no migration-0033
  session reaper — release is graceful-close + lazy `leased_until`; backfill
  source carries no identity). Keep.

## The two binding constraints to DISCHARGE

### C1 — name and PROVE the pre-run operator-session substrate + lifecycle (R1b sufficiency, critical)

v1's sufficiency proof depends on each terminal holding a **distinct pre-run
operator session** whose token sets `app.session_id` at `run.prepare`, so the
run INSERT can select `oh.handle_id WHERE leased_session_id = app.session_id`.
But current source cannot create such a session: `sessions.run_id` is `text NOT
NULL` with an FK to `runs` (`0005_repo_local_workflow_state.sql:44-66`,
VERIFIED), and `HandleRegisterSession` requires `run_id`/`role`/`lane`
(`lifecycle.go:41-46`, VERIFIED) — so the reused `mintSessionBoundToken` is
structurally **run-bound** and cannot mint an operator session before any run
exists. With the ordinary repo-scoped token, `app.session_id` is empty, the
handle subquery finds no row, `created_by_handle_id` is NULL, and **both
same-human runs render `maya#7f3`** — the exact R1b collapse the gate forbids.
The shared `/run/striatum/client-token` does not distinguish terminals either,
so the per-terminal operator session is load-bearing.

The revised SPEC must:
- **Name the storage + lifecycle explicitly.** Resolve the `sessions.run_id NOT
  NULL` FK chicken-and-egg by EITHER making `run_id` nullable with an
  operator-session role/state, OR adding a dedicated operator-session table.
  Pick one, justify it, anchor to source. It must be an **owner-bundle change if
  it ALTERs an owner-held table** (e.g. `ALTER sessions`), folded into bundle
  0021.
- Specify **heartbeat, graceful close, lazy expiry**, and the
  `run -> operator_handles` join **without leaning on run-scoped
  `closeRemainingSessions`** (which keys on `run_id`, absent for an operator
  session).
- Stay a **REUSE** of RFC 0107 session machinery (`mintSessionBoundToken`, the
  `principal_clients` link, the prelude GUC), **not a rebuild**.
- Show how `striatum operator bootstrap` becomes the daemon-side mint+lease
  entry that creates the operator session + token the CLI then presents on
  `run.prepare`/`run.start`.

**Gate (pgtest `operator_session_pre_run_stamp`, two-role):** create two pre-run
operator sessions for ONE human; lease two distinct words; create one run per
session; assert two **NON-NULL, DISTINCT** `created_by_handle_id` and
`whose RA != whose RB`.

### C2 — route the principal stamp through the identity projection (R2/R4 two-role safety, critical)

v1's run-origin INSERT resolves `created_by_principal_id` with a **direct**
subquery `SELECT pc.principal_id FROM striatumd.principal_clients pc WHERE
pc.client_id = current_setting('striatum.principal_id', true)`. But owner bundle
0006 **REVOKEs `SELECT` on `principal_clients` from `striatumd_rw`** and grants
back only `(client_id, linked_at, unlinked_at)` — `principal_id` is omitted on
purpose (`0006_identity_read_scope.sql:218-221`; reasserted `owner.go:454-468`;
BOTH VERIFIED). Under the two-role fixture the as-written stamp fails **`42501`**
before the write-once trigger or `operator_handles` grants ever matter — and it
contradicts v1's own C-2 claim to reuse `ResolvePrincipalForClient` (which routes
through the `SECURITY DEFINER` projection `striatumd.resolve_principal_for_client`).

The revised SPEC must:
- Route the run-origin principal stamp **and** the lease-acquisition principal
  resolution through `striatumd.resolve_principal_for_client` /
  `admin.ResolvePrincipalForClient` inside the authorized `run.prepare` /
  `session.register` transaction — **never** a direct `principal_clients.principal_id`
  read.
- **NOT** grant `SELECT(principal_id)` back to `striatumd_rw` (that reopens the
  RFC 0114 read-scope bundle 0006 deliberately closed).
- Confirm the projection function is callable by `striatumd_rw` under the
  two-role fixture (it is `SECURITY DEFINER`; verify the grant), and reconcile
  the INSERT shape: a subquery cannot necessarily call the projection inline, so
  specify the exact mechanism — resolve the principal in Go via
  `admin.ResolvePrincipalForClient` and bind it as a parameter, or call the
  projection function in SQL.

**Gate (pgtest `run_origin_stamp_uses_identity_projection`, two-role):** (a) a
direct `pc.principal_id` read fails `42501` as a control; (b) the
projection-routed stamp succeeds and stores the right principal; (c) a forged
envelope/request parameter cannot affect the stored value.

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (C1 / sufficiency lens):** Is the pre-run operator-session
  substrate now BUILDABLE and fully specified (storage resolving the `run_id`
  FK, heartbeat, close, lazy expiry, the run->handle join)? Does it stay a reuse,
  not a rebuild? Re-run the two-same-human-terminals proof end-to-end against the
  revised substrate: do `whose RA` and `whose RB` return two distinct non-NULL
  answers, or does anything still collapse `created_by_handle_id` to NULL? Did the
  fix introduce a new gap (e.g. an operator session that never closes, a lazy
  expiry that frees a word a live run still references, a nullable-run_id ALTER
  that breaks an existing NOT-NULL assumption elsewhere)?
- **Falsifier 2 (C2 / two-role + carry-forward lens):** Does the stamp now route
  through the projection and PASS the two-role fixture (no 42501)? Is the
  projection actually callable by `striatumd_rw` (the SECURITY DEFINER grant)?
  Does the SPEC avoid granting `SELECT(principal_id)` back? Then verify NO
  carry-forward regressed: R1a honesty, the R1b architecture, R1c flap, the
  write-once trigger, bundle ordinal 0021, the four open questions, R4 reuse.
  Did C1's substrate change perturb owner-bundle ordering or the watermark?

The adjudicator gates on whether C1 and C2 are each **genuinely discharged** (not
merely claimed — the named pgtests must be specified and the mechanisms anchored
to real source) and whether any carry-forward regressed or any **new** material
challenge lands. Clearing verdict (`accept` / `accept_with_findings`) requires
both C1 and C2 discharged with their pgtests and no standing regression. This is
the single allowed v2 revision cycle; a second `needs_revision` ends the gate
uncleared and routes to the operator.

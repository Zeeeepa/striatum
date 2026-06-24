# Design-Run Seed — RFC 0167 P0 (REVISION v3)

> This is the **v3 revision** of the RFC 0167 P0 design run. v2 RESOLVED the
> narrow cores of both v1 constraints (the pre-run operator-session storage and
> the projection-routed stamp), but the v2 adjudicator found two NEW
> source-verified critical defects (C1′, C2′) and, its single revision cycle
> exhausted, routed them to the operator. This run discharges C1′ and C2′ while
> carrying forward everything cleared through v1+v2. **Required context docs**
> (read in full first):
> - `docs/operator/artifacts/rfc-0167-p0-design-v2/dialogue/holder/HOLDER.md` — the v2 SPEC you are revising (the base; do not rewrite from scratch).
> - `docs/operator/artifacts/rfc-0167-p0-design-v2/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the v2 verdict; its `findings:` + `constraints:` (C1′/C2′) are the exact prescribed fixes.
> - `docs/operator/artifacts/rfc-0167-p0-design/dialogue/holder/HOLDER.md` and `.../adjudicator/COLLABORATION_LEDGER_cycle_1.md` — v1 SPEC + v1 verdict (the original C1/C2, now resolved — context).
> - `docs/rfcs/0167-operator-identity-and-run-attribution.md` — the accepted RFC (D260).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0167 P0** the downstream `rfc-0167-p0-build`
`code_change` run executes. It must **resolve C1′ and C2′** and **carry forward,
unregressed, everything cleared through v1+v2**. A revision that leaves C1′ or
C2′ open — or regresses a carry-forward — has NOT cleared the gate.

## Carried forward — CLEARED through v1+v2 (do NOT reopen, do NOT regress)

- **R1a honesty** (v2 §1): mint+lease in one transaction; run-origin stamp from
  the live-token prelude GUC server-side; read surfaces are pure PG joins.
- **R1b ARCHITECTURE** (v2 §2): per-session leased word via `operator_handles` +
  the live-unique partial index `(repository_id, lower(handle)) WHERE released_at
  IS NULL`; the write-once `runs.created_by_handle_id` snapshot joined by `whose`;
  the deterministic principal-seeded escalation walk (distinct curated words).
- **C1 storage (v2 §2.6) — RESOLVED, KEEP:** the dedicated owner-held
  `operator_sessions` table is the pre-run, per-terminal liveness + token-binding
  anchor; it never touches `striatumd.sessions` (so the `sessions.run_id NOT
  NULL` FK is untouched); the load-bearing fact is real — `PostgresAuthorizer`
  resolves the bound session purely from `client_capabilities.session_id` and
  never joins `striatumd.sessions` (`auth_pg.go:104-156`), so a session-bound
  operator token sets `app.session_id` with no `sessions` row.
- **C2 stamp (v2 §C) — RESOLVED, KEEP:** the run-origin principal resolves in Go
  via `admin.ResolvePrincipalForClient` → the SECURITY DEFINER
  `resolve_principal_for_client` projection (`EXECUTE` granted to `striatumd_rw`,
  `owner/0006:181,186`) and binds as `$N`; the direct `principal_clients.principal_id`
  subquery is deleted; the `42501` direct-read control pgtest stays.
- **R1c flap renewal** (v2 §3): guarded UPDATE, never release-then-reacquire.
- **R2 DB-write-once** (v2 §4): the `BEFORE UPDATE` trigger
  `refuse_run_origin_change()`; owner bundle ordinal **0021**
  (`LatestOwnerBundleVersion == 20`); forward-only / watermark interlock.
- **R3 open questions** (v2 §5): OQ1 curated pool + deterministic
  default/escalation; OQ2 NULL + advisory `attribution_unknown`, no backfill;
  OQ3 per-repo in-P0, daemon-wide → P3; OQ4 byline → P2.
- **R4 reuse** (v2 §6): FK rendering layer over `principal_id`, no parallel
  identity table, opaque `run_id`, product-boundary clean, P0-scoped.

## The two binding constraints to DISCHARGE

### C1′ — the operator-session token must authorize `run.prepare` WITHOUT over-granting admin to lane tokens (critical)

v2's sufficiency proof requires the operator-session token to call `run.prepare`
so `app.session_id` is set and `created_by_handle_id` is stamped. But
`run.prepare` requires **`CapabilityAdmin`** (`registry_methods.go:110`; pinned
in `registry_rfc0043_test.go:27` + the command-authority matrix), while the
reused `mintSessionBoundToken` grants only the fixed slice
`{claim, write, read, review}` (`session_token.go:23-46`) and inserts exactly
those into `client_capabilities` (`session_token.go:77-86`) — no admin. The
dispatcher authorizes **before** threading `AuthContext` into the handler
(`server.go:107-124`), so a v2-shaped operator token is rejected
`capability_missing` before `HandleRunPrepare` runs, before the prelude sets
`app.session_id`, before the `created_by_handle_id` subquery can run.
`S1→RA`/`S2→RB` stop at authorization; the proof cannot execute. And a naive add
of `admin` to the shared `sessionBoundCapabilities` slice would over-grant admin
to **every supervised lane token** — a security regression.

The revised SPEC must:
- Name the operator-token **capability + mint shape explicitly**. The operator
  (the human) legitimately holds the authority to prepare/start runs — today via
  the shared admin runtime client-token — so the operator-session token may
  carry the capability `run.prepare`/`run.start` need (admin, or a narrowly
  scoped capability that admits exactly those routes), via a **distinct operator-
  token mint path** (or a `mintSessionBoundToken` parameterized with a
  capability set) — NOT by widening the shared lane slice. Anchor the chosen
  mechanism to source (the capability constants, the mint, the dispatcher).
- Keep lane tokens on the unchanged narrow `{claim, write, read, review}` slice.
- Specify the operator-session token lifecycle vs. authorization: a **closed or
  expired** operator session must NOT keep authorizing `run.prepare` / stamping
  (no stale-token NULL stamp, no expired-handle reuse).
- **Re-specify `operator_session_pre_run_stamp`** to exercise the REAL
  `run.prepare` RPC authorization path with the minted operator token (two
  pre-run operator sessions for one human → two distinct words → one run each →
  two NON-NULL DISTINCT `created_by_handle_id`, `whose RA != whose RB`), PLUS
  controls: (i) an ordinary supervised **lane** session token does NOT gain
  admin / cannot call `run.prepare`; (ii) a closed/expired operator session
  cannot prepare a run or stamp.

### C2′ — bundle 0021 must NOT grant the runtime role a readable operator-session `client_id → principal_id` mapping (critical)

v2's C1 substrate change regresses the bundle-0006 read-scope closure: the new
`operator_sessions` table carries **both** `principal_id` and `client_id`
(v2 §2.6) and 0021 grants table-wide `SELECT, INSERT, UPDATE ON
striatumd.operator_sessions TO striatumd_rw` (v2 §4.2(5)), so
`SELECT client_id, principal_id FROM operator_sessions WHERE state='active'`
succeeds under `striatumd_rw` — recovering exactly the `client_id → principal_id`
mapping bundle 0006 deliberately closed on `principal_clients` (REVOKE SELECT;
grant back only `(client_id, linked_at, unlinked_at)`; rationale: "without
principal_id a leaked runtime credential sees client ids and timestamps, not
whose credentials they are" — `0006_identity_read_scope.sql:204-221`; reasserted
`owner.go:454-468`). v2's "no `SELECT(principal_id)` grant-back" was scoped only
to `principal_clients`; the same identity linkage reopens by another name.

The revised SPEC must:
- Replace the table-wide `GRANT SELECT ON striatumd.operator_sessions` with
  **column-level grants that EXCLUDE `principal_id` AND `client_id`** (keeping the
  liveness/state columns the runtime legitimately needs:
  `operator_session_id`, `repository_id`, `state`, `last_heartbeat_at`,
  `expires_at`, `closed_at`, etc.), **OR** route any identity-bearing
  operator-session read through a **SECURITY DEFINER projection** with daemon
  authority, mirroring `resolve_principal_for_client`. Pick one, justify, anchor.
- Confirm the runtime still has exactly the operator-session DML it needs
  (the bootstrap mint INSERTs the row; heartbeat/close UPDATE liveness columns;
  the run-origin stamp does NOT read `operator_sessions.principal_id` — it
  resolves the principal via the projection and the handle via `operator_handles`).
- Add a two-role **negative control** (to `owner_bundle_0021_applies_clean` or
  `run_origin_stamp_uses_identity_projection`): `SELECT client_id, principal_id
  FROM striatumd.operator_sessions WHERE state='active'` as `striatumd_rw` must
  fail `42501` (or be impossible because the columns are ungranted), with a
  **positive control** proving create / heartbeat / close + the run stamp +
  authorized `whose` / `status --mine` still work through the narrower grants.

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (C1′ / authorization lens):** Does the operator-session token now
  authorize `run.prepare` end-to-end through the REAL dispatcher path, so the
  two-terminal proof executes and yields two NON-NULL DISTINCT
  `created_by_handle_id`? Is the operator-token capability granted via a DISTINCT
  mint path (not the shared lane slice)? Verify lane tokens do NOT gain admin
  (construct the control). Verify a closed/expired operator session cannot stamp.
  Did the fix over-grant (operator token strictly more than run.prepare/start
  needs) or open a privilege-escalation path?
- **Falsifier 2 (C2′ / read-scope + carry-forward lens):** Can `striatumd_rw`
  still read `client_id`+`principal_id` from `operator_sessions` by ANY route
  (table-wide grant residue, a view, a function)? Is the negative control real
  (does it actually fail 42501)? Do the narrower grants still let create /
  heartbeat / close / stamp / render work (positive control)? Then verify NO
  carry-forward regressed: R1a, the R1b architecture, the v2 C1 storage + C2
  projection stamp, R1c, the write-once trigger, bundle ordinal 0021 (still
  next-free, not advanced; watermark/forward-only intact), the four open
  questions, R4 reuse.

The adjudicator gates on whether C1′ and C2′ are each **genuinely discharged**
(not merely claimed — mechanisms anchored to real source, the named pgtests +
controls specified) and whether any carry-forward regressed or any **new**
material challenge lands. Clearing verdict (`accept` / `accept_with_findings`)
requires both C1′ and C2′ discharged with their controls and no standing
regression. This is the single allowed v3 revision cycle; a second
`needs_revision` ends the gate uncleared and routes to the operator.

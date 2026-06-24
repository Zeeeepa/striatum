---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0167-p0-design"
run_id: "run_f400af27d67052dc01e5352d8473e66d"
cycle: 1
topic: "RFC 0167 P0 falsifiable implementation SPEC — operator identity & run attribution (leased handles + write-once run-origin stamp + whose)"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "The holder SPEC claims it discharges all of R1a-R4: identity is bound server-side inside the token-mint transaction (R1a); R1b sufficiency is proven by leasing a word per session via operator_handles + a live-unique partial index, stamping a write-once runs.created_by_handle_id snapshot, and the §2.5 proof that two same-human terminals render maya#7f3 vs theo#7f3 (R1b); heartbeat is a guarded UPDATE that never releases (R1c); the change is owner bundle 0021 with a BEFORE UPDATE write-once trigger proven under the two-role fixture (R2); all four open questions are resolved in-P0/deferred (R3); and it rides RFC 0107 without a parallel identity table (R4). It also publishes four source corrections (C-1..C-4)."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "R1b operator-session gap. The §2.5/A7 sufficiency proof depends on each terminal holding a distinct PRE-RUN operator session whose token populates app.session_id at run.prepare, so the run INSERT can select oh.handle_id WHERE leased_session_id = app.session_id. But current source has no buildable pre-run session: striatumd.sessions.run_id is text NOT NULL with an FK to runs (0005:44-66, VERIFIED), and HandleRegisterSession requires run_id/role/lane and aborts otherwise (lifecycle.go:41-46, VERIFIED) — so mintSessionBoundToken (the 'existing machinery' the SPEC reuses) is structurally run-bound and cannot mint an operator session before any run exists. With the ordinary repo-scoped token, app.session_id resolves empty, the handle subquery finds no row, created_by_handle_id is NULL, and both same-human runs render maya#7f3 — exactly the R1b collapse the gate forbids. The SPEC names the 'operator-session seam' as in-P0 but never specifies the substrate/lifecycle (nullable sessions.run_id with an operator-session state, or a dedicated operator-session table; how it heartbeats, closes, lazily expires, and joins operator_handles without closeRemainingSessions). The decisive proof rests on the one object current source cannot create before the run."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "R2/R4 principal-projection gap. The holder's run-origin INSERT resolves created_by_principal_id with a DIRECT subquery: SELECT pc.principal_id FROM striatumd.principal_clients pc WHERE pc.client_id = current_setting('striatum.principal_id', true). But owner bundle 0006 REVOKEs SELECT on principal_clients from striatumd_rw and grants back only (client_id, linked_at, unlinked_at) — principal_id is omitted on purpose (0006_identity_read_scope.sql:218-221, reasserted owner.go:454-468, BOTH VERIFIED). Under the RFC 0142 two-role fixture the as-written stamp fails SQLSTATE 42501 before the write-once trigger or operator_handles grants ever matter. This contradicts the SPEC's own C-2/§6 claim to reuse ResolvePrincipalForClient (which routes through the SECURITY DEFINER projection striatumd.resolve_principal_for_client), and the named pgtest owner_bundle_0021_applies_clean is a false-green risk if it inserts a literal principal_id instead of exercising the real server-side dereference. R2 (two-role runtime safety of the stamp path) and R4 (reuse RFC 0107/0114, do not punch through) are therefore not demonstrated."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "C1 (carries into the single revision): name the pre-run operator-session storage + lifecycle EXPLICITLY and prove it buildable. Resolve the sessions.run_id NOT NULL FK chicken-and-egg (either make run_id nullable with an operator-session role/state, or add a dedicated operator-session table) and specify heartbeat, graceful close, lazy expiry, and the run->operator_handles join without leaning on run-scoped closeRemainingSessions. Add a two-role pgtest (operator_session_pre_run_stamp) that creates two pre-run sessions for one human, leases two distinct words, creates a run per session, and asserts two NON-NULL DISTINCT created_by_handle_id with whose RA != whose RB."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "C2 (carries into the single revision): route the run-origin principal stamp through striatumd.resolve_principal_for_client (or admin.ResolvePrincipalForClient) inside the authorized run.prepare transaction — NEVER a direct principal_clients.principal_id read — and resolve the lease-acquisition principal through the same projection. Do NOT grant SELECT(principal_id) back to striatumd_rw (that reopens the RFC 0114 read-scope bundle 0006 closed). Replace/extend the pgtest with run_origin_stamp_uses_identity_projection asserting: (a) a direct pc.principal_id read fails 42501 as a control, (b) the projection-routed stamp succeeds and stores the right principal, (c) a forged envelope/request parameter cannot affect the stored value."
verdict: "needs_revision"
rationale: "needs_revision. The SPEC is high-quality, source-grounded, and its disambiguation ARCHITECTURE for R1b is the correct one (per-session leased handle + live-unique partial index + a write-once created_by_handle_id snapshot joined by whose — not merely created_by_principal_id). R1a (honesty), R1c (guarded-UPDATE flap renewal), and R3 (all four open questions, with OQ2's no-backfill well-justified by C-4) are DISCHARGED and went unchallenged; R2's DB-write-once face (BEFORE UPDATE trigger, justified over column REVOKE with the 0010 precedent) is DISCHARGED. BUT BOTH gate-critical requirements carry a standing, verified material challenge. (1) R1b SUFFICIENCY IS NOT PROVEN as buildable: the §2.5/A7 proof depends on a pre-run operator session that current source cannot create — sessions.run_id is NOT NULL with an FK to runs and session.register requires a run_id (both verified), so the 'reuse existing mint machinery' claim is unbuildable and the SPEC leaves the operator-session substrate/lifecycle unspecified; without it created_by_handle_id is NULL and both same-human runs collapse to maya#7f3. (2) R2/R4 TWO-ROLE STAMP SAFETY IS BROKEN: the run-origin INSERT reads the revoked principal_clients.principal_id column directly and fails 42501 under the two-role fixture (bundle 0006 revoke verified), contradicting the SPEC's own reuse claim and risking a false-green pgtest. The rubric requires R1b sufficiency PROVEN and the owner-bundle two-role safety SPECIFIED for a clearing verdict; both are missing here and both falsifier challenges stand unrebutted. This is the single allowed revision cycle: discharge C1 (name + prove the pre-run operator-session substrate) and C2 (route the stamp through resolve_principal_for_client with a 42501-control pgtest). Both repairs are concrete and in-P0-buildable, so the gate is recoverable in one revision; this is needs_revision, not reject. R4 and R2 also depend on C1 staying a reuse (not a rebuild) of RFC 0107 session machinery."
findings:
  - id: F-R1B-PRERUN-SESSION
    severity: critical
    posture: sufficiency_unbuildable
    status: converted_to_constraint
    challenge: "The R1b sufficiency proof (§2.5/A7) requires a pre-run operator session whose token sets app.session_id at run.prepare, but sessions.run_id is NOT NULL with an FK to runs (0005:44-66) and session.register requires run_id/role/lane (lifecycle.go:41-46), so the reused mintSessionBoundToken cannot create such a session before a run exists. The SPEC names the seam but does not specify the substrate/lifecycle, leaving created_by_handle_id NULL and both same-human runs rendering maya#7f3 — the R1b collapse."
    source_refs: ["dialogue:2"]
  - id: F-R2-PRINCIPAL-PROJECTION
    severity: critical
    posture: two_role_privilege_gap
    status: converted_to_constraint
    challenge: "The run-origin INSERT reads principal_clients.principal_id directly, but bundle 0006 revoked SELECT on that column from striatumd_rw and grants back only (client_id, linked_at, unlinked_at) (0006:218-221; owner.go:454-468). The stamp fails 42501 under the two-role fixture, contradicts the SPEC's own ResolvePrincipalForClient reuse claim (C-2/§6), and the owner_bundle_0021_applies_clean pgtest risks a false-green if it inserts a literal principal_id rather than exercising the real dereference."
    source_refs: ["dialogue:3"]
constraints:
  - id: C1-PRERUN-OPERATOR-SESSION
    source_finding: F-R1B-PRERUN-SESSION
    posture: sufficiency_unbuildable
    severity: critical
    kind: gate
    binding: true
    text: "The revised SPEC must name the pre-run operator-session storage + lifecycle explicitly (resolve the sessions.run_id NOT NULL FK: nullable run_id with an operator-session state, or a dedicated operator-session table) and specify heartbeat, graceful close, lazy expiry, and the run->operator_handles join without leaning on run-scoped closeRemainingSessions. It must stay a reuse of RFC 0107 session machinery, not a rebuild."
    source_refs: ["dialogue:2"]
    verification:
      gate: "Two-role pgtest operator_session_pre_run_stamp: two pre-run sessions for one human lease two distinct words; one run per session asserts two NON-NULL DISTINCT created_by_handle_id and whose RA != whose RB."
    final_review_required: true
  - id: C2-IDENTITY-PROJECTION-STAMP
    source_finding: F-R2-PRINCIPAL-PROJECTION
    posture: two_role_privilege_gap
    severity: critical
    kind: gate
    binding: true
    text: "The revised SPEC must route the run-origin principal stamp (and the lease-acquisition principal resolution) through striatumd.resolve_principal_for_client / admin.ResolvePrincipalForClient inside the authorized transaction — never a direct principal_clients.principal_id read — and must NOT grant SELECT(principal_id) back to striatumd_rw."
    source_refs: ["dialogue:3"]
    verification:
      gate: "Two-role pgtest run_origin_stamp_uses_identity_projection: (a) direct pc.principal_id read fails 42501 (control), (b) projection-routed stamp succeeds storing the right principal, (c) a forged envelope/request param cannot affect the stored value."
    final_review_required: true
branches:
  sufficiency_unbuildable: "blocked"
  two_role_privilege_gap: "blocked"
---

# Collaboration Ledger — RFC 0167 P0 design (cycle 1)

**Verdict: `needs_revision`.** This is the single allowed revision cycle. The SPEC
is strong and well-anchored to current `main`, and its R1b disambiguation
*architecture* is correct — but both gate-critical requirements carry a verified,
standing material challenge, so the gate does not clear.

## Per-requirement disposition

| Req | Demand | Status | Basis |
|-----|--------|--------|-------|
| **R1a** | identity bound server-side at token-mint, against the live token; no client-name / display-signal path on any read surface | **DISCHARGED** | §1 A1–A5: mint+lease in one transaction, stamp from the live-token prelude GUC (never an envelope param), read surfaces are pure PG joins. No spoof path found by Falsifier 1. The concrete stamp SQL carries the F2 projection defect, but that is an R2/R4 mechanism bug — the honesty principle holds and C2 preserves it. |
| **R1b** | SUFFICIENCY PROVEN — two same-human terminals return two DISTINCT `whose` answers via a per-session handle, not a principal-derived one | **OPEN** | Architecture is right (per-session `created_by_handle_id` snapshot + live-unique partial index + the run→handle_id join — *not* merely `created_by_principal_id`). But the §2.5/A7 PROOF rests on a pre-run operator session current source cannot create (`sessions.run_id` NOT NULL FK; `session.register` requires a run — both verified). Substrate/lifecycle unspecified ⇒ sufficiency unproven. **F1 standing.** |
| **R1c** | heartbeat renews via guarded UPDATE, never release-then-reacquire | **DISCHARGED** | §3 A12 + `lease_flap_steal`; guarded `WHERE leased_session_id=$2 AND released_at IS NULL`, mirrors the `striatumd.leases` idiom. Unchallenged. |
| **R2** | owner-bundle migration safe under the two-role non-superuser owner DSN; DB-enforced write-once; runtime keeps exactly its needed privileges; named two-role pgtests | **OPEN** | DB-write-once face DISCHARGED (BEFORE UPDATE trigger, justified over column REVOKE with the `0010` precedent; ordinal 21 correct — `LatestOwnerBundleVersion==20` verified; forward-only/watermark interlock sound). BUT two-role *runtime safety of the stamp path is broken*: the direct `principal_clients.principal_id` read fails `42501` (bundle 0006 revoke verified); the named pgtest risks a false-green. **F2 standing.** |
| **R3** | all four open questions resolved (in-P0 / deferred + mechanism + why) | **DISCHARGED** | OQ1 curated pool + deterministic default/escalation + denylist-by-exclusion (in-P0); OQ2 NULL + advisory `attribution_unknown`, no backfill (well-justified by C-4 — `branch_confirmed_by` carries no identity); OQ3 per-repo in-P0, daemon-wide deferred to P3; OQ4 byline suffix out of P0 (P2), scope-only. Unchallenged. |
| **R4** | ride RFC 0107 (operator-id IS `principal_id`); no parallel identity table; product-boundary clean; P0-only | **OPEN** | Mostly clean (FK rendering layer, no parallel identity, reuse, opaque `run_id`, boundary clean, P0-scoped). BUT F2's direct `principal_clients` read *punches through* the RFC 0114 read-scope bundle 0006 closed instead of reusing the `resolve_principal_for_client` projection; and F1's unspecified pre-run-session substrate must be confirmed a *reuse*, not a rebuild, of session machinery. **Bound to C2 (and C1).** |

## Per-falsifier disposition

- **Falsifier 1 — R1b operator-session gap (dialogue:2): MATERIAL, STANDING.**
  Verified at source: `sessions.run_id text NOT NULL` FK→`runs`
  (`0005:44-66`); `HandleRegisterSession` aborts without `run_id/role/lane`
  (`lifecycle.go:41-46`). The SPEC's "reuse existing mint machinery" is
  unbuildable before a run exists, and the operator-session substrate is never
  specified. The R1b sufficiency proof is contingent on an object that cannot be
  created in P0 as written. Not rebutted. → **C1.**
- **Falsifier 2 — R2/R4 principal-projection gap (dialogue:3): MATERIAL, STANDING.**
  Verified at source: bundle 0006 `REVOKE SELECT ON principal_clients … GRANT
  SELECT (client_id, linked_at, unlinked_at)` (`0006:218-221`), reasserted at
  `owner.go:454-468`. The holder's direct `pc.principal_id` subquery fails
  `42501` under the two-role fixture and contradicts the SPEC's own
  `ResolvePrincipalForClient`/projection reuse claim. Not rebutted. → **C2.**

## What clears the gate on revision

The architecture is sound and both defects are concrete, in-P0-buildable
mechanism gaps — recoverable in one revision (hence `needs_revision`, not
`reject`):

1. **C1** — name and prove the pre-run operator-session substrate + lifecycle;
   pgtest `operator_session_pre_run_stamp` (two distinct non-NULL
   `created_by_handle_id`; `whose RA != whose RB`).
2. **C2** — route the principal stamp through
   `striatumd.resolve_principal_for_client` (no direct column read, no
   `SELECT(principal_id)` grant-back); pgtest
   `run_origin_stamp_uses_identity_projection` with the `42501` direct-read
   control.

This is the single allowed revision cycle: a second `needs_revision` ends the
gate uncleared and routes to the operator.

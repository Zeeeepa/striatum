---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0167-p0-design-v4"
run_id: "run_2aa8b41c52902026c21fd12f2b1b1412"
cycle: 1
topic: "RFC 0167 P0 falsifiable implementation SPEC — REVISION v4 (operator identity & run attribution), the intended CONVERGING cycle: discharge the two operator-routed residuals C2'' (close the COMPOSED client_id->principal_id reconstruction over the full ACL graph — Route 1 cc⋈oh, Route 2 cc⋈oh⋈runs) and C1'' (justified-acceptance of the operator-session admin token with the verifier.attest trust-root fence) without regressing any v1+v2+v3 carry-forward"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "The v4 consolidated whole-P0 SPEC discharges BOTH operator-routed residuals and carries v1+v2+v3 forward unregressed. C2'' (the decisive blocker): Route 1 (cc⋈oh on oh.principal_id) is closed by a column-scoped operator_handles SELECT excluding principal_id (NEW table in 0021, granted narrow from creation); Route 2 (cc⋈oh⋈runs on runs.created_by_principal_id) is closed by REVOKE SELECT ON runs + a column re-GRANT excluding created_by_principal_id, which is an irreversible boundary because runs is owner-held (C-1, not in 0018's transfer cohort) so striatumd_rw cannot self-re-grant; identity-bearing reads route through daemon-secret-gated SECURITY DEFINER projections (run_origin_identity, runs_for_origin_client) mirroring resolve_principal_for_client; the three runs star-readers are converted to explicit column lists; the gates are registered in readScopeReasserts so ReassertReadRevokes re-closes them on drift; controls are the COMPOSED negative control (Route 1 + Route 2 each 42501) plus a positive control proving whose/status --mine/stamp/heartbeat still work; A37 claims no third route. C1'' is RESOLVED via JUSTIFIED-ACCEPTANCE (RFC ledger option (c)) hardened by (b): the operator-admin verbs ARE the operator's legitimate repo authority; the surface is repo-scoped (daemon-global credential/key/shutdown/migrate verbs structurally unreachable), the verifier.attest IsSessionBound() refusal already fences the one trust-root route from the session-bound operator token, and the N-token blast-radius is framed as a strictly-less-standing materialization of authority the human already holds via the static admin token. A19 is honestly REVISED to A19' (the runs REVOKE) and the safety it protected is preserved by the owner-held-irreversible + atomic-per-version + reasserted mechanism. Every v1/v2/v3 carry-forward is asserted INTACT."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_and_rebutted
    text: "C2'' / A37-A38 — the third-route enumeration (HOLDER.md:216-229) omits the live RFC 0122 scheduler grant table striatumd.spawn_authorization_grants. Migration 0027 creates it with run_id + owner_principal_id (0027:37-42) and grants table-wide SELECT,INSERT,UPDATE to striatumd_rw (0027:57-61); owner bundle 0018 transfers it into the runtime-OWNED cohort (0018:80-103). The composed route cc⋈oh⋈runs⋈spawn_authorization_grants on sag.owner_principal_id avoids both columns the holder revokes, and every joined/selected column is runtime-readable under the holder's own grant plan, so for an auto_spawn-enabled run it reconstructs client_id->owner_principal_id. A fork the SPEC does not resolve: (1) if the build stores a real principal_id in owner_principal_id (as the column name and RFC 0122/0167 language suggest), C2'' has a third composed principal leak, and it is WORSE than the runs route because spawn_authorization_grants is runtime-owned so a plain column revoke would be self-re-grantable; (2) if owner_principal_id stays today's client id, it is an unmodelled RFC 0122 integration exception that can keep scheduler attribution off the new principal stamp path. The v4 controls catch neither branch — composed_identity_map_unreadable tests only Route 1/Route 2 and no control seeds an auto_spawn run or scans information_schema for this existing *principal_id*-named column. A37 ('no remaining runtime-readable column pairs a credential identifier with a principal_id') is therefore false or at least unproved."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_and_rebutted
    text: "C1'' / A40-A42 + blast-radius — the trust-root fence (verifier.attest refuses any session-bound token, verifier_attest.go:49-59) and the repo-scope bound (a repo-scoped row cannot satisfy daemon-global methods, auth_pg.go:104-140) are REAL and conceded. The defect is the blast-radius accounting: the holder treats the operator-session tokens as a REPLACEMENT for the static bootstrap admin credential, but source shows the static credential is a separate, broader, non-expiring bearer the SPEC neither retires nor segregates. bootstrap.go:18-27 defines bootstrapCapabilities = {admin,read,write,claim,review,apply,recovery,surgical_recovery} (eight, not {admin,read}); BootstrapRuntimeTokenIfNeeded mints bootstrap-admin via insertTokenClient with expiresAt=nil (bootstrap.go:92-98) and writes it to the 0600 runtime client-token file; bootstrap_test.go:25-36,63-75 pins it repository-unscoped and eight-capability. So unless the SPEC proves the static token is revoked/segregated from the operator path, the live system is static-PLUS-N, not strictly-less-standing: a leaked terminal now leaks a repo-admin session token that did not exist before WHILE the broader static token still exists. operator_token_admin_surface proves the new token's surface but never proves the static token is absent from the operator process / not used for run.prepare/checkpoint.resolve/review.override, nor covers the non-admin operator verbs (write/claim/review/apply/recovery) the static token carries and the {admin,read} token lacks. A40's justified-acceptance is thus unproved as a SPEC claim even though the fence is correct."
  - kind: rebuttal
    by: adjudicator
    refs: ["dialogue:2"]
    text: "F1 LANDS as a real enumeration/labeling residual but is REBUTTED on the decisive C2'' axis. Verified at source: striatumd.spawn_authorization_grants.owner_principal_id is 'a bare text column with NO foreign key to striatumd.principals ... It holds the attribution identity the authority prelude installs as striatum.principal_id (today, the run owner's CLIENT id)' (0027:18-27). So the composed route reconstructs client_id->owner_principal_id where owner_principal_id is ITSELF a client_id — i.e. client_id->client_id — NOT client_id->principal_id. This matches the codebase-wide convention (CORRECTION C-2: the GUC named striatum.principal_id carries a client_id; the real principal resolves in Go via ResolvePrincipalForClient). The v4 SPEC does NOT propose to change spawn_authorization_grants (P1 custody is explicitly out of P0, §7), so the build executing this SPEC leaves the column a client_id and the decisive, non-negotiable bar — a still-reconstructable client_id->principal_id by any route — is NOT breached by F1's branch (1). F1's branch (2) is overstated: RFC 0167 D2 (0167:153-155) puts auto-spawned scheduler-run attribution on runs.created_by_principal_id ('the same rules'), so scheduler-origin runs are already on the principal stamp path via the normal run-origin stamp; the grant table is upstream replay state, not the attribution rendering surface. What DOES land: A37 ('no remaining runtime-readable column pairs a credential identifier with a principal_id') is non-exhaustive — A37's own refuting test (an information_schema.role_column_grants scan over *principal_id* columns) WOULD flag spawn_authorization_grants.owner_principal_id, which the §C2''.4 table omitted. Disposition: the two named routes ARE closed; the latent risk (a future change populating owner_principal_id with a real principal_id with no control to catch it) is carried as a binding build-time refinement, not a reopened leak — accept_with_findings, not needs_revision."
  - kind: rebuttal
    by: adjudicator
    refs: ["dialogue:3"]
    text: "F2 LANDS as a real justification-accounting residual but is REBUTTED on the SEED's named C1'' requirements. The decisive C1'' bar is 'a sound justified-acceptance with the verifier.attest trust-root refusal confirmed'; BOTH the trust-root fence (verifier_attest.go:49-59, the session-bound operator token is already refused) and the repo-scope daemon-global bound (auth_pg.go:104-140; the highest-consequence credential/key/shutdown/migrate verbs are structurally unreachable) are confirmed at source and conceded by F2. The accepted surface (repo-scoped operator-admin MINUS the fenced trust-root route) is correctly enumerated and is the operator's legitimate authority. What lands: the 'strictly-less-standing' FRAMING overclaims. Verified: the static bootstrap-admin token is broader (eight capabilities), non-expiring (expiresAt=nil), and repository-unscoped (bootstrap.go:18-27,92-98), so while it persists the live credential set is static-PLUS-N. The honest reframing — which the SPEC's §1(1) design already implies (operator-bootstrap mints and presents the session token) — is that the ROUTINE operator credential narrows to the TTL-bounded, repo-scoped, close-revoked, trust-root-fenced session token, while the static token remains the segregated daemon-root credential for the daemon-global + recovery/apply surfaces the session token deliberately lacks. Disposition: the named C1'' requirements are discharged; the blast-radius reframing + a credential-segregation control are carried as a binding build-time refinement — accept_with_findings, not needs_revision."
verdict: "accept_with_findings"
rationale: "accept_with_findings — this is the intended CONVERGING cycle and it converges. The decisive, non-negotiable C2'' bar is MET: the two operator-routed composed routes are genuinely closed (Route 1 by the column-scoped operator_handles SELECT excluding principal_id; Route 2 by the owner-held-irreversible REVOKE SELECT ON runs + column re-GRANT excluding created_by_principal_id, with the three star-readers converted and the identity reads routed through daemon-secret-gated SECURITY DEFINER projections), the COMPOSED negative control (Route 1 + Route 2 each 42501) plus the positive control plus the drift_reassert_recloses_routes control are specified, and runs being owner-held (C-1) makes the Route 2 REVOKE an irreversible boundary needing no ownership transfer. Falsifier 1's third candidate route via spawn_authorization_grants does NOT breach the bar: that table's owner_principal_id is verified at source (0027:18-27) to hold the run owner's CLIENT id, with NO FK to striatumd.principals — so the route reconstructs client_id->client_id, not client_id->principal_id, and the SPEC does not change it (P1 custody is out of P0). C1'' is discharged via a sound justified-acceptance: the verifier.attest trust-root refusal and the repo-scope daemon-global bound — the SEED's named C1'' requirements — are both confirmed at source and conceded by Falsifier 2, and the accepted surface is correctly enumerated as the operator's legitimate repo authority minus the fenced trust-root route. No carry-forward regressed: A19 is honestly REVISED to A19' (the runs REVOKE), and the invariant it protected (no silent re-grant; no half-applied REVOKE strands the surface) is preserved by a STRONGER mechanism (owner-held-irreversible + atomic-per-version + idempotent + readScopeReasserts re-closure); R1a, the R1b disambiguation architecture, the v2 storage substrate + projection stamp, the v3 C1' authorization core + C2' direct-read closure, R1c, the BEFORE UPDATE write-once trigger, bundle ordinal 0021 (build re-verifies; 0022 if RFC 0142 P4 takes 0021), the four R3 open questions, and R4 reuse are all INTACT and unchallenged on the merits. Two material challenges LANDED and were REBUTTED on the gate-decisive axis, leaving two MINOR residuals that are precise, source-grounded, in-P0-buildable refinements rather than an unclosed leak, a broken proof, or a regressed carry-forward: (F-C2DPRIME-SPAWN-GRANT-ENUMERATION) the §C2''.4 third-route enumeration is non-exhaustive — it omitted spawn_authorization_grants, which A37's own ACL scan would flag — so the build must enumerate it, PIN that owner_principal_id is not a principal_id (a control that fails loudly if a future change populates it with one), and note the RFC 0122 scheduler-attribution seam; and (F-C1DPRIME-STATIC-TOKEN-ACCOUNTING) the 'strictly-less-standing' blast-radius framing overclaims while the broader, non-expiring, unscoped static bootstrap token persists, so the build must reframe the acceptance as the routine-credential narrowing it actually is and add a credential-segregation control. Per the SEED's converging-cycle rubric these are exactly the MINOR build-time refinements for which accept_with_findings is preferred over needs_revision, so the gate CLEARS and the consolidated P0 SPEC proceeds to the committer, with both refinements carried as binding constraints the rfc-0167-p0-build final review must discharge."
findings:
  - id: F-C2DPRIME-SPAWN-GRANT-ENUMERATION
    severity: high
    posture: read_scope_enumeration_completeness
    status: converted_to_constraint
    challenge: "The C2'' third-route ACL enumeration (§C2''.4, HOLDER.md:216-229) and assertion A37 omit the live RFC 0122 table striatumd.spawn_authorization_grants, which is runtime-readable (table-wide SELECT to striatumd_rw, 0027:57-61), runtime-owned after owner bundle 0018 (0018:80-103), and carries a column literally named owner_principal_id (0027:37-42) keyed by run_id. A37's own refuting test (an information_schema.role_column_grants scan over *principal_id* columns) would flag that column, so A37 as stated is non-exhaustive. The decisive C2'' bar is NOT breached because owner_principal_id is verified to hold the run owner's CLIENT id with NO FK to striatumd.principals (0027:18-27) — the composed cc⋈oh⋈runs⋈spawn_authorization_grants route reconstructs client_id->client_id, not client_id->principal_id — and the v4 SPEC does not modify the table (P1 custody is out of P0, §7). Residual risk: a future change populating owner_principal_id with a real principal_id would reopen the leak with no control to catch it, and the runtime-owned table would not be self-re-grant-proof like owner-held runs. Build-time refinement, not a current leak."
    affected_invariants: ["C2-COMPOSED-READ-SCOPE-CLOSURE", "A37-NO-THIRD-ROUTE", "RFC-0122-SCHEDULER-ATTRIBUTION", "R4-read-scope-closure"]
    source_refs: ["dialogue:2"]
  - id: F-C1DPRIME-STATIC-TOKEN-ACCOUNTING
    severity: high
    posture: credential_blast_radius_accounting
    status: converted_to_constraint
    challenge: "The C1'' justified-acceptance's N-token blast-radius is framed as 'strictly-less-standing' on the premise that the operator-session tokens REPLACE the static bootstrap admin credential, but the SPEC neither retires nor segregates that credential. Verified at source: bootstrapCapabilities = {admin,read,write,claim,review,apply,recovery,surgical_recovery} — eight capabilities, not {admin,read} (bootstrap.go:18-27); BootstrapRuntimeTokenIfNeeded mints bootstrap-admin with expiresAt=nil (non-expiring) and writes it to the 0600 runtime token file (bootstrap.go:92-98,105); bootstrap_test.go:25-36,63-75 pins it repository-unscoped. So while the static token persists the live credential set is static-PLUS-N, not strictly-less-standing. The verifier.attest trust-root fence (verifier_attest.go:49-59) and the repo-scope daemon-global bound (auth_pg.go:104-140) — the SEED's named C1'' requirements — ARE confirmed and conceded, so this is a justification-accounting residual, not a broken fence. Build-time refinement: reframe the acceptance as the routine-operator-credential narrowing it actually is, and add a control proving routine repo-admin routes are presented the session-bound token while documenting the static token's residual segregated daemon-root scope."
    affected_invariants: ["C1-OPERATOR-TOKEN-JUSTIFIED-ACCEPTANCE", "A40-ACCEPTED-SURFACE", "blast-radius-honesty"]
    source_refs: ["dialogue:3"]
constraints:
  - id: C-C2DPRIME-SPAWN-GRANT-ENUMERATION
    source_finding: F-C2DPRIME-SPAWN-GRANT-ENUMERATION
    posture: read_scope_enumeration_completeness
    severity: high
    kind: gate
    binding: true
    text: "The build must enumerate striatumd.spawn_authorization_grants in the C2'' third-route ACL analysis and PIN its disposition: owner_principal_id holds the run owner's CLIENT id with no FK to striatumd.principals (0027:18-27), so the composed cc⋈oh⋈runs⋈spawn_authorization_grants route reconstructs client_id->client_id, NOT a principal leak. Add a control asserting that disposition so a future change populating owner_principal_id with a real principal_id (or making it a principals FK) fails loudly; if such a change is ever made, the table needs the same treatment as the other identity-bearing surfaces (column-scope SELECT excluding the identity column, an owner-held/projection read boundary stable against the runtime-owned re-grant trap, and a readScopeReasserts entry). Note the RFC 0122 scheduler-attribution seam: auto-spawned scheduler runs receive runs.created_by_principal_id via the normal run-origin stamp (RFC 0167 D2, 0167:153-155), not via the grant table. This refines, and does not reopen, the two named composed routes, which remain closed."
    source_refs: ["dialogue:2"]
    verification:
      gate: "A two-role pgtest extending composed_identity_map_unreadable: as striatumd_rw the cc⋈oh⋈runs⋈spawn_authorization_grants composed query over an auto_spawn-enabled run yields only client_id values for owner_principal_id (no principal_id; no FK to principals), AND an information_schema.role_column_grants scan over every *principal_id*-named column for striatumd_rw records spawn_authorization_grants.owner_principal_id as the asserted client_id-holding exception rather than a granted real-principal column; scheduler-origin runs still resolve runs.created_by_principal_id through the run_origin_identity DEFINER projection."
      expected_stage: "rfc-0167-p0-build"
    final_review_required: true
  - id: C-C1DPRIME-STATIC-TOKEN-SEGREGATION
    source_finding: F-C1DPRIME-STATIC-TOKEN-ACCOUNTING
    posture: credential_blast_radius_accounting
    severity: high
    kind: gate
    binding: true
    text: "The build must make the C1'' blast-radius accounting source-honest. Because the static bootstrap-admin token is broader (eight capabilities), non-expiring (expiresAt=nil), and repository-unscoped (bootstrap.go:18-27,92-98), the operator-session token is NOT strictly-less-standing while the static token persists. Reframe the acceptance: the ROUTINE operator repo-admin credential narrows to the TTL-bounded, repo-scoped, close-revoked, trust-root-fenced session token, while the static token remains the segregated daemon-root credential for daemon-global + recovery/apply/surgical_recovery surfaces the session token deliberately lacks. Add a credential-segregation control proving routine operator repo-admin routes (run.prepare/checkpoint.resolve/review.override/branch.confirm) are presented the session-bound operator token and that the static bootstrap token is not injected into the launched operator process / MCP client for routine repo-admin; document where the static token legitimately remains usable. The verifier.attest trust-root fence and the repo-scope daemon-global bound are CONFIRMED and must be kept."
    source_refs: ["dialogue:3"]
    verification:
      gate: "operator_token_admin_surface (or a sibling credential-boundary control): routine operator repo-admin routes succeed when presented the session-bound operator token; the static bootstrap-admin token is shown absent from the routine operator repo-admin path (process/MCP client config); the SPEC/docs record the static token's residual segregated daemon-root scope (daemon-global + recovery/apply/surgical_recovery), so the blast-radius is stated as static-segregated-plus-narrowed-routine rather than strictly-less-standing."
      expected_stage: "rfc-0167-p0-build"
    final_review_required: true
branches:
  read_scope_enumeration_completeness: "cleared_with_constraints"
  credential_blast_radius_accounting: "cleared_with_constraints"
---

# Collaboration Ledger — RFC 0167 P0 design v4 (cycle 1)

**Verdict: `accept_with_findings`.** This was the intended **converging** cycle, and it
converges. The decisive, non-negotiable **C2″** bar is met, **C1″** is discharged via a
sound justified-acceptance with the trust-root fence confirmed, and **no carry-forward
regressed**. Two material challenges landed and were rebutted on the gate-decisive axis,
leaving two **minor, in-P0-buildable** residuals carried as binding build-time refinements
— exactly the case the SEED reserves for `accept_with_findings` over `needs_revision`. The
consolidated P0 SPEC clears to the committer.

## C2″ — the decisive blocker: RESOLVED

| Route | Closure in v4 | Verified |
|---|---|---|
| **Route 1** `cc ⋈ oh` on `oh.principal_id` | column-scoped `operator_handles` SELECT excluding `principal_id` (NEW table in 0021 → narrow GRANT from creation) | Negative control `composed_identity_map_unreadable` Route 1 → `42501` (A35) |
| **Route 2** `cc ⋈ oh ⋈ runs` on `runs.created_by_principal_id` | `REVOKE SELECT ON runs` + column re-GRANT excluding `created_by_principal_id`; **irreversible** because `runs` is owner-held (C-1, not in 0018's transfer cohort) — no ownership transfer needed; three star-readers converted to explicit column lists; identity reads via daemon-secret-gated DEFINER projections | Negative control Route 2 → `42501` (A36); positive control `whose_status_mine_via_projection` (A38); `drift_reassert_recloses_routes` (A39) |

Both operator-routed routes are genuinely closed with the **composed** negative controls
the v3 ledger demanded, plus a drift-reassert. **The decisive bar — a still-reconstructable
`client_id → principal_id` by *any* route — is NOT breached.**

**Falsifier 1's third candidate route does not reopen the leak.** `spawn_authorization_grants`
is real, live, runtime-readable RFC 0122 state — but its `owner_principal_id` is verified at
source to hold **the run owner's CLIENT id, with NO foreign key to `striatumd.principals`**
(`0027_spawn_authorization_grants.sql:18-27`). So the composed
`cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants` route reconstructs `client_id → client_id`,
**not** `client_id → principal_id`. This is consistent with CORRECTION C-2 (the GUC named
`striatum.principal_id` carries a client id; the real principal resolves in Go via
`ResolvePrincipalForClient`). The v4 SPEC does not modify this table (P1 custody is out of
P0, §7), so the build leaves the column a client id. Scheduler-origin runs are already on the
principal stamp path via `runs.created_by_principal_id` (RFC 0167 D2, `0167:153-155`), not via
the grant table.

→ **Finding `F-C2DPRIME-SPAWN-GRANT-ENUMERATION` (high, carried):** A37's enumeration is
non-exhaustive — A37's own ACL scan would flag this `*principal_id*`-named column, which
`§C2″.4` omitted. The build must enumerate it, pin that `owner_principal_id` is not a real
principal (a control that fails loudly if a future change makes it one), and note the RFC 0122
seam. This refines, not reopens, C2″.

## C1″ — operator-token justified-acceptance: RESOLVED

The SEED's named C1″ requirements are **confirmed at source and conceded by Falsifier 2**:

- **Trust-root fence confirmed** — `verifier.attest` refuses any session-bound token
  (`verifier_attest.go:49-59`); the session-bound operator token is already refused (A41).
- **Repo-scope bound confirmed** — a repo-scoped capability row cannot satisfy daemon-global
  methods (`auth_pg.go:104-140`); `daemon.token.create`/`key.rotate`/`shutdown`/`migrate` are
  structurally unreachable (A42).
- **Accepted surface correctly enumerated** — per-repo run-lifecycle + operator repo-admin,
  minus the fenced trust-root route, is the operator's legitimate authority; a
  "run-lifecycle-only" capability would break the operator's job.

→ **Finding `F-C1DPRIME-STATIC-TOKEN-ACCOUNTING` (high, carried):** the *"strictly-less-standing"*
framing overclaims while the static `bootstrap-admin` token persists — verified broader (eight
capabilities `{admin,read,write,claim,review,apply,recovery,surgical_recovery}`), non-expiring
(`expiresAt=nil`), and repository-unscoped (`bootstrap.go:18-27,92-98`). The build must reframe
the acceptance as the routine-credential narrowing it actually is and add a credential-segregation
control. The fence itself is sound; this is an accounting/segregation refinement.

## Per-carry-forward disposition (none regressed)

| Carry-forward | Status | Basis |
|---|---|---|
| R1a identity bound server-side; no tty/tmux/title/env path | **INTACT** | §1 A1–A5, unchallenged |
| R1b disambiguation architecture (per-session leased word, live-unique index, write-once handle snapshot, principal-seeded escalation, run→handle join) | **INTACT** | §2 A6–A11/A27; the two-terminal proof now executes through the real authorization path |
| v2 C1 storage substrate (`operator_sessions`; `PostgresAuthorizer` resolves from `client_capabilities.session_id`) | **INTACT** | §2.6, unchallenged |
| v2 C2 projection stamp (`ResolvePrincipalForClient` → `resolve_principal_for_client`) | **INTACT** | §1(2) A28, unchallenged |
| v3 C1′ authorization core (`mintOperatorSessionToken` `{admin,read}`; lane slice untouched; close-revoke + TTL) | **INTACT** | §C1′ A29–A32; now scope-bounded by §C1″ |
| v3 C2′ direct-read closure (`operator_sessions` column-scoped excluding `principal_id`,`client_id`) | **INTACT** | §2.6, §C2″.5 A33 |
| R1c guarded-UPDATE lease flap | **INTACT** | §3 A12, unchallenged |
| R2 `BEFORE UPDATE` write-once trigger; owner bundle ordinal 0021 (build re-verifies → 0022 if RFC 0142 P4 takes 0021); forward-only/watermark | **INTACT** | §4 A13–A18 |
| **A19 → A19′** ("0021 carries no privilege-stripping REVOKE") | **REVISED, not regressed** | Honest, called-out delta (§E, §C2″.5): the runs REVOKE is required to close Route 2; the safety A19 protected (no silent re-grant, no half-applied REVOKE) is preserved by a stronger owner-held-irreversible + atomic-per-version + idempotent + reasserted mechanism |
| R3 four open questions (OQ1–OQ4) | **INTACT** | §5 A20–A23, unchallenged |
| R4 reuse / no parallel identity / opaque run_id / no read-scope punch-through | **INTACT** | §6 A24–A26; the composed-route closure makes the RFC 0114 carry-forward provably intact under the composed graph |

## Per-falsifier disposition

- **Falsifier 1 — C2″ third route (`dialogue:2`): MATERIAL, LANDED, REBUTTED on the decisive
  axis.** Real enumeration/labeling gap (A37 non-exhaustive), but `spawn_authorization_grants.owner_principal_id`
  holds a client id with no FK to `principals` (`0027:18-27`), so it reconstructs
  `client_id → client_id`, not a principal leak; the SPEC does not change it. → carried finding
  `F-C2DPRIME-SPAWN-GRANT-ENUMERATION` + binding constraint `C-C2DPRIME-SPAWN-GRANT-ENUMERATION`.
- **Falsifier 2 — C1″ blast-radius accounting (`dialogue:3`): MATERIAL, LANDED, REBUTTED on the
  named C1″ requirements.** The trust-root fence and repo-scope bound are confirmed/conceded; the
  residual is the `"strictly-less-standing"` framing while the broader non-expiring static token
  persists (`bootstrap.go:18-27,92-98`). → carried finding `F-C1DPRIME-STATIC-TOKEN-ACCOUNTING`
  + binding constraint `C-C1DPRIME-STATIC-TOKEN-SEGREGATION`.

## Gate outcome

The converging cycle **converges**: C2″ (decisive) and C1″ are discharged at source, no
carry-forward regressed, and the two standing challenges were rebutted on the gate-decisive
axis. The two residuals are precise, source-grounded, in-P0-buildable refinements — not an
unclosed identity leak, a broken proof, or a regressed carry-forward — so per the SEED's
converging-cycle rubric the gate **clears with findings**. The consolidated P0 SPEC proceeds
to the committer; the two refinements are carried as **binding constraints** (`final_review_required`)
the downstream `rfc-0167-p0-build` final review must discharge.

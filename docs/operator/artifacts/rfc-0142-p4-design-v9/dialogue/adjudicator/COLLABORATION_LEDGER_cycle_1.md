---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec (v9 REVISION; the single binding cycle-1 finding M7 — the §3.5/§4.5/F18 row-16 cell (complete, decoupled, revoke-embedding) asserted unconditionally where A's decoupled complete branch is fingerprint-conditional and reads neither applied_owner NOR revokeEmbedded — is GENUINELY RESOLVED via the prescribed Option 1: §0.2 adds the A-`revokeEmbedded`-independence-of-the-decoupled-complete-branch sub-invariant and the DERIVATION-RULE-COMPLETENESS sub-invariant; §3.3a step 3, §3.5 row 16 (`==0`/`==20`/`>=21` all conditional `SERVE-verify if in-sync, else awaiting_deploy`, identical to row 15), §1.3, §4.5, and F18 all propagate the conditional; F18 is PARAMETRIC over the seven A-reaching complete-row cells {13/==0,13/==20,15/==0,15/==20,16/==0,16/==20,16/>=21} with the in-sync/out-of-sync sub-dimension; the four `:399`-reaching cells {1/==0,1/==20,13-in-sync/==0,13-in-sync/==20} are UNCHANGED (row 16 is decoupled → ConnectAndVerify never self-records); the normal pre-0021 row-16 state is documented OUT-OF-SYNC; the v8 F18 refutation cell is now correctly handled. BOTH falsifiers (decoupling-boundary/decision-table lens; carry-forward/regression lens) independently concede M7 resolved with no carry-forward regression and no new material design gap; M6(rows 13/15)/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward intact. The only residuals are non-blocking BUILD-PHASE obligations (F18 must construct the row-16 in-sync/out-of-sync sub-cases over owner_bundle_meta absent/20/>=21 and assert no `:399` spy fires; the `64-cell` shorthand must be expanded to concrete cursor-state enum coverage in T-deploy-bootpath-decision-table). Adjudicator source-verified against the run worktree HEAD 3280708f, `git diff --stat 3f9d5734 HEAD -- go/pkg/db/` EMPTY — every cited owner.go / schema_drift.go / connection.go anchor holds byte-identically. → accept_with_findings)"
participants:
  - "holder-author-001"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: "Revised P4 spec (v9) resolves the single binding cycle-1 finding M7 via the prescribed Option 1 (the clean fix parallel to the M6 fix): it DERIVES the row-16 (`cursorState=complete`, `decoupledEnabled=true`, `revokeEmbedded=true`) decision-table cell from A instead of asserting it. §0.2 carries the load-bearing INVARIANT W→A-INDEPENDENCE verbatim and adds two NEW sub-invariants: (a) A-`revokeEmbedded`-INDEPENDENCE OF THE DECOUPLED COMPLETE BRANCH — step 0 (`revokeEmbedded && !decoupledEnabled → awaiting_deploy_config`) is the ONLY A predicate that reads `revokeEmbedded` and it does NOT fire when `decoupledEnabled == true`, so A's decoupled complete branch (§3.3a step 3) reads neither `applied_owner` NOR `revokeEmbedded`, only `cursor.plan_hash` + the recorded `schema_state.fingerprint`, so rows 15 and 16 take the IDENTICAL A3-conditional outcome in every W-passing column; (b) DERIVATION-RULE COMPLETENESS — every A-reaching complete-row cell whose A outcome is fingerprint-conditional is WRITTEN conditionally. §3.5 row 16 in the `==0`, `==20`, AND `>=21` columns becomes conditional `SERVE-verify if in-sync, else awaiting_deploy`, identical to row 15; the normal pre-0021 `==0`/`==20` state is documented OUT-OF-SYNC (→ awaiting_deploy, dominant) and the normal post-0021 `>=21` state IN-SYNC (→ serve-verify, dominant); the opposite sub-case is the degenerate corner A must handle. F18 is PARAMETRIC over the seven A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`}, constructing BOTH in-sync and out-of-sync sub-cases for each. §1.3/§3.3a/§3.5/§4.5 are updated consistently. The four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} are UNCHANGED (row 16 is decoupled → `ConnectAndVerify` never reaches `:353`/`:399`), so §4.5 and the F18 spy list still enumerate the SAME four cells. M6 (rows 13/15), M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 are carried forward verbatim and unregressed; the M7 fix does NOT re-collapse the row-1 fresh-DB serve, regress rows 13/15, weaken the M3 A0 config gate (cells 2/6/10/14 still halt `awaiting_deploy_config`), regress the BC-N2 `applied_owner == 20` edge, or advance `RequiredOwnerBundleVersion` (stays 20). Re-anchored line-by-line against the v9 worktree HEAD; `git diff --stat 3f9d5734 HEAD -- go/pkg/db/` is EMPTY (the M7 fix is a SPEC change, not a source change)."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "DECOUPLING-BOUNDARY / DECISION-TABLE lens — re-attack of the v8 row-16 refutation. Reproduced the v8 refutation cell as a test input (cursorState=complete, decoupledEnabled=true, revokeEmbedded=true, applied_owner=0 or ==20, deploy_plan[plan_hash] present, cursor.plan_hash==expected, LiveFingerprint(recorded)==ExpectedFingerprint(), owner_bundle_meta absent or 20) and re-confirmed the orthogonality premise against source (`owner.go:23-35,145-153`; `schema_drift.go:145-161,171-195` read/write only `striatumd.schema_state`). RESULT: the v9 holder now DERIVES this cell from A instead of asserting it — §0.2 adds the decoupled-complete `revokeEmbedded`-independence and derivation-rule-completeness sub-invariants (HOLDER.md:171-188); §3.3a states A's decoupled complete branch reads neither `applied_owner` nor `revokeEmbedded`, so rows 15 and 16 share the same fingerprint-conditional outcome (HOLDER.md:493-502); §3.5 row 16 says `SERVE-verify if in-sync, else awaiting_deploy` for `==0`, `==20`, AND `>=21`, documenting the normal pre-0021 state out-of-sync (HOLDER.md:656-657); F18 names the seven A-reaching cells and requires both in-sync and out-of-sync sub-cases (HOLDER.md:927-941); the `:399` spy list stays the SAME four cells with no row-16 additions (HOLDER.md:698-721,885-917,941). The `>=21` revoke-embedding cell is also made conditional. The strongest attempted rebuttal — that `complete + revokeEmbedded + applied_owner < 21` is a corrupt state that should halt even if `schema_state` says in-sync — was exactly the v8 gap and requires an explicit guard A does not have; the v9 holder deliberately chooses the OTHER coherent contract (Option 1: conditional on fingerprint-sync, normal pre-0021 case out-of-sync) rather than smuggling that guard in. No standing M7 decision-table falsification remains; no M3, M6, M5(row-1), BC-N2, or RequiredOwnerBundleVersion regression. RESIDUAL (build-phase, not a holder-spec falsification): `T-deploy-bootpath-decision-table` must actually construct the row-16 in-sync AND out-of-sync sub-cases for `applied_owner == 0`, `==20`, and `>=21` and assert the in-sync cells serve verify-only WITHOUT firing the `ApplyMigrations`/`RecordSchemaFingerprint` spies; omitting them would recreate M7 in code. This falsifier does not stop the v9 revision from clearing on M7."
    correspondence: landed_and_rebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "CARRY-FORWARD / REGRESSION lens — re-attack that the local row-16 fix regressed a v8-cleared finding or left a sibling cell asserted. Attempt 1 (M7 regresses M6 rows 13/15 or the spy list): the v9 holder preserves the §0.2 W→A-independence invariant and adds the M7 sub-invariants WITHOUT narrowing the M6 rule; rows 13 and 15 stay conditional in both `==0` and `==20`; the `:399` self-record set stays {1/==0, 1/==20, 13-in-sync/==0, 13-in-sync/==20}; row 16 serves verify-only and fires no `:399` spy (HOLDER.md:654-657,681-684,889-917,941). No M6 regression. Attempt 2 (M7 reopens the M5 fresh-DB serve or the M3 config gate): source still supports the M5 split (`owner.go:23,35,140,145,148`); row 1/`==0` stays `SERVE-legacy — FRESH-DB BRING-UP`; the M3 step-0 gate stays first for every cursor state and cells 2/6/10/14 stay `awaiting_deploy_config`; F17 unchanged (HOLDER.md:484-489,604-614,642-655,940). No M5/M3 regression. Attempt 3 (Option 1 undercuts M1/M2/M4/C3): M1 stays scoped to deploy resume/finalizer step 0 (no new serve-boot stamp verifier — row 16 is conditional, not Option-2-guarded); M2's non-revoke filter + split loader and C3's terminal 0021 are unchanged; F16a/F16b stay staged (HOLDER.md:404-447,566-579,849-850,939). No M1/M2/M4/C3 regression. Attempt 4 (a new completeness gap outside the complete rows): the table still walks all four cursor groups (none / in_progress-step_committed-aborted / finalizing / complete) and F18 names every concrete cursor state. RESIDUAL (implementation watchpoint, not a standing falsification): the `64-cell` shorthand groups `step_committed` with `in_progress` and `aborted` with the non-complete edge (16 row-groups × 4 buckets), so the executable test should table-drive each concrete cursor-state enum named by F18 rather than the prose group label. I do not find a material carry-forward regression or new complete-row derivation gap that should stop the v9 revision from clearing."
    correspondence: landed_and_rebutted
  - kind: rebuttal
    by: "holder-author-001"
    refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
    text: "The revised v9 spec is the standing rebuttal to both re-attacks, and BOTH falsifiers concede it stands. The M7 challenge (a material decision-table executability defect that landed independently from both v8 falsifiers) is rebutted by deriving row 16 from A under one coherent contract (Option 1): the decoupled complete branch reads neither `applied_owner` NOR `revokeEmbedded` (§0.2 sub-invariant, source-confirmed — step 0 is the only `revokeEmbedded`-reading predicate and does not fire when decoupled is ON; `schema_drift.go:145-161`/`:171-195` read/write only `schema_state`, orthogonal to `owner_bundle_meta`), so rows 15 and 16 take the IDENTICAL A3-conditional outcome and §3.5 row 16 `==0`/`==20`/`>=21` are written conditionally `SERVE-verify if in-sync, else awaiting_deploy`; the v8 F18 refutation cell (in-sync row 16) now produces the SAME outcome in §3.3a, §3.5, §4.5, and F18 (A returns nil = serve verify-only; the table now says serve, conditional on in-sync); F18 is parametric over the seven A-reaching complete-row cells; the `:399` spy list stays the four cells (row 16 is decoupled → never self-records), so §4.5 and the F18 spy list still agree. The carry-forward re-attacks are rebutted by the source-anchored preservation of every v8-cleared mechanism: M6 rows-13/15 conditional cells unchanged; M5 row-1 fresh-DB serve (`owner.go:145`) intact; M3 hoisted A0 config gate (cells 2/6/10/14) intact; BC-N2 `applied_owner == 20` non-complete edge (cells 5/7/9/11) intact; `RequiredOwnerBundleVersion = 20` (`owner.go:35`) not advanced; M1/M2/M4/BC-N1/C1/C2/C3 unchanged by the row-16-local fix. The only residuals both falsifiers name are BUILD-PHASE obligations on `T-deploy-bootpath-decision-table` (construct the row-16 in-sync/out-of-sync sub-cases and assert no `:399` spy; expand the grouped cursor-state shorthand to concrete enum coverage) — non-blocking implementation watchpoints forwarded to the rfc-0142-p4-build run, not spec defects."
verdict: "accept_with_findings"
rationale: "REVISION cycle v9 (the gate's single allowed revision cycle). The single binding cycle-1 finding M7 is GENUINELY RESOLVED via the prescribed Option 1 (the clean fix parallel to the M6 fix), and BOTH falsifiers — from the two independent lenses that converged on M7 in v8 (falsifier_1 decoupling-boundary/decision-table; falsifier_2 carry-forward/regression) — independently concede the resolution AND find no carry-forward regression and no new material design gap. M7 RESOLVED: §0.2 adds the two NEW sub-invariants the fix rests on — (a) A-`revokeEmbedded`-INDEPENDENCE OF THE DECOUPLED COMPLETE BRANCH (step 0, the ONLY `revokeEmbedded`-reading A predicate, does NOT fire when `decoupledEnabled == true`, so A's decoupled complete branch reads neither `applied_owner` NOR `revokeEmbedded`, so rows 15 and 16 take the IDENTICAL A3-conditional outcome in every W-passing column) and (b) DERIVATION-RULE COMPLETENESS (every A-reaching complete-row cell whose A outcome is fingerprint-conditional is WRITTEN conditionally); §3.3a step 3 covers row 16 identically to row 15; §3.5 row 16 in the `==0`, `==20`, AND `>=21` columns is conditional `SERVE-verify if in-sync, else awaiting_deploy`, matching row 15, with the normal pre-0021 `==0`/`==20` state documented OUT-OF-SYNC and the normal post-0021 `>=21` state IN-SYNC; §1.3 reclassifies the complete/decoupled in-sync row as `applied_owner`- AND `revokeEmbedded`-independent; F18 is PARAMETRIC over the seven A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`} with the in-sync/out-of-sync sub-dimension; the four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} are UNCHANGED (row 16 is decoupled → `ConnectAndVerify` never reaches `:353`/`:399`), so §4.5 and the F18 spy list still enumerate the SAME four cells. The v8 F18 refutation cell (the in-sync row-16 cell, constructible because `schema_state` and `owner_bundle_meta` are orthogonal) is now CORRECTLY handled: W passes, A returns nil (serve verify-only), and §3.5 row 16 now says serve (conditional on in-sync) — F18 is no longer a false oracle. ADJUDICATOR SOURCE-VERIFICATION this cycle (direct read of the run worktree, HEAD `3280708f`; `git diff --stat 3f9d5734 HEAD -- go/pkg/db/` EMPTY, so every anchor the holder cites against `3f9d5734` holds byte-identically — the holder's cited HEAD `83fe6465` is an immaterial citation drift, the go/pkg/db source is unchanged): `owner.go:145` `if applied == 0 { return nil }` returns BEFORE the `< RequiredOwnerBundleVersion` shortfall at `:148-150` (the M5 fresh-DB serve), `owner.go:23` `LatestOwnerBundleVersion = 20` and `:35` `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20, NOT advanced); `schema_drift.go:145-161` `LiveFingerprint` reads ONLY the `striatumd.schema_state` singleton and `:171-195` `RecordSchemaFingerprint` writes ONLY that singleton — grep confirms ZERO `owner_bundle_meta`/`applied_owner` references in `schema_drift.go` (the `applied` hits are `applied_at`, a `schema_state` timestamp column, and comments), so A's fingerprint inputs are GENUINELY orthogonal to W's owner-watermark inputs and the decoupled complete branch is genuinely `revokeEmbedded`-independent; `connection.go:349/:353/:399` boot order W → `ApplyMigrations` → `RecordSchemaFingerprint` (`This is the only writer of schema_state`) confirmed. The M7 derivation is real and source-accurate. The boot-path decision table is COMPLETE and EXECUTABLE: all 64 cells derived from W and A; the `==0`/`==20` column identity (M6) AND the row-15/row-16 decoupled identity (M7) closed by derivation and walked in the explicit cross-row audit; Universal Invariant B proven in every cell incl. the M3 cell and the shadow-mode fall-through; the legitimate fresh-DB / inert-landing cells (1/`==0`, 1/`==20`) STILL SERVE (not wedged). The eleven carry-forwards M6(rows 13/15)/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are carried forward verbatim and INTACT — both falsifiers concede no regression, and the row-16-local M7 fix does not re-collapse the row-1 fresh-DB serve, regress rows 13/15, weaken the M3 A0 config gate, regress the BC-N2 `applied_owner == 20` edge, advance `Required`, or alter the watermark. No new material challenge stands unrebutted: every fresh attack attempt (the row-16 re-derivation, the `>=21` conditional, the §4.5↔F18 spy-list consistency, the M6/M5/M3/BC-N2/Required regression checks, and the completeness-gap-outside-the-complete-rows probe) was directly rebutted by the source-anchored revised spec. The two residuals both falsifiers raise are non-blocking BUILD-PHASE obligations, NOT spec falsifications (recorded as finding B1, deferred to rfc-0142-p4-build): (i) `T-deploy-bootpath-decision-table` must actually construct the row-16 in-sync AND out-of-sync sub-cases for `applied_owner == 0`/`==20`/`>=21` and assert the in-sync cells serve verify-only WITHOUT firing the `ApplyMigrations`/`RecordSchemaFingerprint` spies (else M7 recurs in code); (ii) the `64-cell` shorthand groups `step_committed` with `in_progress` and `aborted` with the non-complete edge (16 row-groups × 4 buckets), so the executable test must table-drive each concrete cursor-state enum named by F18 rather than the prose group label. Because M7 is genuinely resolved, the decision table is complete and executable (every complete-row cell derived from A's fingerprint-sync predicate, F18 parametric, Invariant B proven everywhere, the legitimate fresh-DB cells still serving, §4.5 and the F18 spy list consistent at four `:399`-reaching cells), all eleven carry-forwards are intact, and no new material challenge stands unrebutted, the gate CLEARS. The verdict is `accept_with_findings` (a clearing verdict) rather than `accept` to record the two non-blocking build-phase obligations so the build run honors them. The ledger verdict — not falsifier completion — clears the phase gate."
findings:
  - id: M7
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED — the single binding cycle-1 finding (the §3.5/§4.5/F18 row-16 cell `complete`/decoupled/revoke-embedding asserted unconditionally where A's decoupled complete branch is fingerprint-conditional and reads neither `applied_owner` NOR `revokeEmbedded`) is genuinely fixed via the prescribed Option 1; BOTH falsifiers independently concede the resolution. §0.2 adds the A-`revokeEmbedded`-INDEPENDENCE OF THE DECOUPLED COMPLETE BRANCH sub-invariant (step 0, the only `revokeEmbedded`-reading A predicate, does not fire when `decoupledEnabled == true`) and the DERIVATION-RULE-COMPLETENESS sub-invariant (HOLDER.md:171-188). §3.3a step 3 covers row 16 identically to row 15 (HOLDER.md:493-502). §3.5 row 16 `==0`/`==20`/`>=21` is conditional `SERVE-verify if in-sync, else awaiting_deploy`, matching row 15; the normal pre-0021 `==0`/`==20` state is documented OUT-OF-SYNC and the normal post-0021 `>=21` state IN-SYNC (HOLDER.md:656-666). F18 is PARAMETRIC over the seven A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`} with the in-sync/out-of-sync sub-dimension (HOLDER.md:927-941). The four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} are UNCHANGED (row 16 is decoupled → `ConnectAndVerify` never self-records), so §4.5 and the F18 spy list still agree (HOLDER.md:698-721,885-917). ADJUDICATOR SOURCE-VERIFICATION (run worktree HEAD `3280708f`; `git diff --stat 3f9d5734 HEAD -- go/pkg/db/` EMPTY): `schema_drift.go:145-161` `LiveFingerprint` reads ONLY `striatumd.schema_state` and `:171-195` `RecordSchemaFingerprint` writes ONLY it — ZERO `owner_bundle_meta`/`applied_owner` references — so the decoupled complete branch is genuinely `applied_owner`- and `revokeEmbedded`-independent; the v8 F18 refutation cell now produces the SAME outcome in A, §3.5, §4.5, and F18 (serve verify-only when in-sync), so F18 is no longer a false oracle. M7 RESOLVED; the decision table is COMPLETE and EXECUTABLE across all complete-row cells."
    affected_invariants: ["boot-path decision table COMPLETE and DERIVED from W and A at cursorState=complete in the decoupled/revoke-embedding cells (row 16) — §3.5/§3.3a/§1.3/§4.5/F18 agree", "the F18 matrix oracle matches the `applied_owner`- and `revokeEmbedded`-independent A predicate for the in-sync row-16 `==0`/`==20`/`>=21` cells", "the holder's own derivation rule applied to EVERY fingerprint-conditional cell (rows 13/15/16), not only rows 13/15", "the table imports no cross-table `owner_bundle_meta < 21 ⟹ fingerprint mismatch` premise A does not read/verify", "the four `:399`-reaching cells unchanged — row 16 reaches the legacy writer in NO column"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M6
    severity: high
    posture: design
    status: answered
    challenge: "INTACT (rows 13/15) — carried forward verbatim, unregressed, and now EXTENDED to row 16 by the M7 derivation; both falsifiers confirm no regression. §0.2 W→A-independence invariant intact (now also covering row 16); §3.5 rows 13 and 15 in the `==0` column stay conditional `serve if in-sync, else awaiting_deploy` IDENTICAL to `==20`; the degenerate 13/`==0`-in-sync idempotent `:399` rewrite stays in BOTH §4.5 AND the F18 spy list, so the four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} stay enumerated identically (HOLDER.md:654-656,698-721,941); the cross-row `==0`-vs-`==20` audit is intact (the complete-rows bullet now records row 16 conditional too, HOLDER.md:681-688). The M7 fix did NOT re-collapse rows 13/15. M6 INTACT."
    affected_invariants: ["rows 13/15 `==0`/`==20` column identity derived from the `applied_owner`-independent A predicate", "the degenerate 13/`==0`-in-sync `:399` rewrite enumerated identically in §4.5 and the F18 spy list", "§0.2 W→A-independence invariant anchored to schema_drift.go:145-161/:171-195", "the cross-row `==0` audit for none/in_progress/finalizing/complete"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M5
    severity: high
    posture: design
    status: answered
    challenge: "INTACT (row 1) — carried forward verbatim, unregressed; both falsifiers confirm no regression. The `applied_owner` dimension stays split `{0/no authority, 1..19 authority shortfall, ==20, >=21}` covering all of ℕ; W = `CheckOwnerBundleWatermark` returns nil for `applied_owner == 0` (the fresh/single-role bootstrap, source-verified `owner.go:145` BEFORE the `:148-150` shortfall), halts `awaiting_owner_ddl` for `1 <= applied_owner < 20`, returns nil for `==20`, and `>=21` is the forward barrier (b). §3.5 cell 1/`==0` stays `SERVE-legacy — FRESH-DB BRING-UP`; cell 1/`==20` is the inert-landing re-boot; F18a (`T-deploy-fresh-db-bootstrap-serves`) pins the fresh-DB serve (HOLDER.md:604-614,642,942). The M7 fix is in the `complete`/decoupled cell (row 16, `cursorState=complete`), not the `cursorState=none` row-1 cell, and does NOT re-collapse it. M5(row-1) INTACT."
    affected_invariants: ["the `{0/1..19/==20/>=21}` owner-watermark split", "cell 1/`==0` serves the fresh-DB bring-up; cell 1/`==20` inert-landing", "F18/F18a dual assertion", "M7 derivation does not re-collapse row 1"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M3
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm no regression. The `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config halt stays HOISTED to step 0 of `CheckDeployActivation`, before any cursor-state branch, firing for EVERY cursor state incl. `complete` (barrier a, HOLDER.md:484-489); §3.5 cells 2/6/10/14 still halt `awaiting_deploy_config` at A0 in every `applied_owner` column that passes W (HOLDER.md:642-655); F17 + F11(g) + F18 carry forward. The M7 fix is in the DECOUPLED revoke-embedding cell (row 16, decoupled ON) where step 0 does NOT fire and A reaches the decoupled complete branch — it does NOT re-open the M3 legacy mutate+self-record bypass (row 16 is decoupled and never reaches the legacy `:399` writer; both falsifiers confirm). M3 INTACT."
    affected_invariants: ["hoisted step-0 config gate (every cursor state, every `applied_owner` column that passes W)", "no revoke-embedding binary reaches the legacy `:399` writer (row 16 is a decoupled serve-verify cell, no `:399`)", "the no-revoke `complete` pre-ApplyMigrations pure-read comparison", "cells 2/6/10/14 still halt `awaiting_deploy_config`"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M4
    severity: medium
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. F16a (`TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, synthetic list / injectable-FS, rollout step 2) drives the exclusion filter WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (`TestOwnerDDLApplyExcludesProductionRevokeBundle`, step 7) asserts the production embed/listing split + houses the forced FMA-007 self-heal pgtest reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError` (`owner.go:367-374`) (HOLDER.md:431-447,939). The M7 fix does not touch F16. M4 INTACT."
    affected_invariants: ["F16a synthetic phase builds green before 0021 exists", "F16b production phase + the forced self-heal", "the M2 filter unchanged by the F16 staging"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: M1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers find no regression. `VerifyStoredTranscript(plan_hash)` (§3.4a) checks the ENTIRE stored transcript `sha256` against the binary's embedded bytes ⇒ `deploy_plan_binary_mismatch`; already-applied entries also verify the DB stamps ⇒ `deploy_plan_db_stamp_mismatch`; the SAME verifier runs on every resume AND as finalizer step 0 (HOLDER.md:566-579). The M7 fix chose Option 1 (the conditional cell), NOT Option 2 (a new boot-time DB-stamp guard), so M1's scope (deploy resume / finalizer step 0, NOT ordinary serve-boot) is unchanged and NOT extended — the row-16 in-sync cell serves because A reads the recorded fingerprint and finds it == `ExpectedFingerprint()`, not via a new serve-boot stamp check. M1 INTACT."
    affected_invariants: ["full stored-transcript byte + DB-stamp verification on resume AND finalizer step 0", "typed mismatch halts", "the deployer finalizer gated by VerifyStoredTranscript step 0", "M1 not extended to serve-boot by the M7 Option-1 fix"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The single non-revoke filter `isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)` + the split loader `OwnerDDLApplyBundles()` bind across every `owner-ddl apply` route incl. the FMA-007 self-heal `ReapplyAllOwnerBundles` (in-loop guards) + the nil-fallback split; `OwnerBundles()` (full, includes 0021) is kept ONLY for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable` (HOLDER.md:404-429). The M7 fix does not touch the filter. M2 INTACT."
    affected_invariants: ["0021 unreachable through every owner-ddl apply route incl. the FMA-007 self-heal", "embed/listing split", "0021 committed only as the terminal deploy step"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: BC-N1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The runtime-owned `deploy_plan` table (migration 0044) storing the IMMUTABLE ordered transcript keyed by `plan_hash`, materialized in the same tx that sets `deploy_cursor → in_progress(0)` BEFORE step 0; resume loads `deploy_plan[cursor.plan_hash]` and never recomputes `BuildPlan`; §1.3 + the transcript-enumerated doctor + F14 (HOLDER.md:287-320). The M7 fix touches only the `complete` rows of the boot-path table. BC-N1 INTACT."
    affected_invariants: ["BC-N1 immutable plan identity across resume", "stable (plan_hash, step_index) receipt key", "resume off the stored transcript"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: BC-N2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm. The universal incomplete-deploy edge halts `awaiting_deploy` DB-untouched on every non-`complete` cursor at `applied_owner == 20` (the `==20` column's non-complete cells 5/7/9/11 verbatim, HOLDER.md:646-653); F11(e)/(f) + the extended `G-old-binary-refuse` carry forward. The M7 fix is in the `complete`/decoupled/revoke-embedding cell (row 16), NOT the non-complete edge — both falsifiers note this explicitly. BC-N2 INTACT."
    affected_invariants: ["BC-N2 universal pre-revoke deploy_cursor-authoritative edge at applied_owner == 20", "incomplete (non-complete) cursor never serves", "M7 does not regress the non-complete edge (cells 5/7/9/11 unchanged)"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C1
    severity: critical
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The `finalizing` state + the idempotent finalizer (VerifyStoredTranscript step 0 → guarded `complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST) + the §1.3 resumable-finalization row + F10 (HOLDER.md:314-320). The M7 fix touches neither the finalizer nor the legacy writer's enforcement — it makes the §3.5 row-16 cell coherent in the `complete` rows the finalizer's `complete` cursor lands in, without changing the finalizer. C1 INTACT."
    affected_invariants: ["finalization boundary (no complete with missing receipt/fingerprint)", "idempotent finalizer gated by VerifyStoredTranscript step 0", "section 1.3 disambiguation"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: C2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm and adjudicator source-verified. `CheckDeployActivation` before `ApplyMigrations` in both boot paths (mutating nothing); the typed `awaiting_deploy`/`awaiting_deploy_config` halts; the forward-watermark rule firing at `applied_owner >= 21` (barrier b, the `>=21` column); and `RequiredOwnerBundleVersion = 20` (source-verified `owner.go:35` = `LatestOwnerBundleVersion` = 20, NOT advanced) (HOLDER.md:805-813). The M7 fix keeps the forward-watermark rule at `applied_owner >= 21` and does NOT advance `Required`, alter the watermark, or change the `applied >= 21` forward rule; it makes the decoupled row-16/`>=21` cell conditional on A3 fingerprint-sync (W tolerates the revoke-embedding binary forward to A). C2 INTACT."
    affected_invariants: ["fail-closed CheckDeployActivation edge before ApplyMigrations", "forward-watermark rule at applied >= 21 (barrier b)", "RequiredOwnerBundleVersion KEPT at 20, not advanced"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C3
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede the mechanism. Bundle 0021 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) identified by `DDLRevokeOwnerBundleVersion = 21`, special-cased out of the owner prefix, applied as the TERMINAL plan step after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE granted by 0018); `LatestOwnerBundleVersion`/`Required` stay 20; F12/`G-revoke-last` (HOLDER.md:849-861). M7 arose BECAUSE the v8 table imported the C3 reachability fact (`a completed revoke deploy means applied_owner >= 21`) into A without A verifying the owner-bundle stamp; the v9 fix makes the cell conditional on fingerprint-sync rather than on owner-watermark state — the correct derivation from A — so C3 itself is not regressed. C3 mechanism INTACT."
    affected_invariants: ["two-role owner/runtime boundary", "revoke-last terminal ordering", "reconcile-while-CREATE-held", "0021 deploy-terminal only"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: B1
    severity: low
    posture: build
    status: deferred_with_owner
    challenge: "BUILD-PHASE OBLIGATION (non-blocking; owner: the rfc-0142-p4-build run) — NOT a spec falsification; both falsifiers raise it as an implementation watchpoint, not a holder-spec gap. (i) falsifier_1: `T-deploy-bootpath-decision-table` (F18) must actually CONSTRUCT the row-16 in-sync AND out-of-sync sub-cases for `applied_owner == 0`, `==20`, and `>=21` (the in-sync arm setting `schema_state.fingerprint == ExpectedFingerprint()` AND `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent/20/`>=21` DB, proving the orthogonality) and must assert the in-sync row-16 cells serve verify-only WITHOUT firing the `ApplyMigrations`/`RecordSchemaFingerprint` spies; if the build omits those cases or silently treats row 16 as a normal-only shortcut, that recreates M7 in code (HOLDER.md:171-178 of falsifier_1). (ii) falsifier_2: the `64-cell` shorthand groups `step_committed` with `in_progress` and `aborted` with the non-complete edge (16 row-groups × 4 buckets), so the executable test must table-drive each CONCRETE cursor-state enum named by F18 (`none`/`in_progress`/`step_committed`/`finalizing`/`complete`/`aborted`) rather than the prose group label. Both are forwarded to the build run; neither blocks the design gate. Recorded so the build honors them."
    affected_invariants: ["F18 must construct the row-16 in-sync/out-of-sync sub-cases and assert no `:399` spy fires", "T-deploy-bootpath-decision-table must table-drive each concrete cursor-state enum, not the grouped prose label"]
    source_refs: ["dialogue:2", "dialogue:3"]
branches:
  design: cleared
---

# COLLABORATION LEDGER — RFC 0142 P4 design run (v9 / REVISION), cycle 1

author: adjudicator-author-001

**Verdict:** `accept_with_findings`

> Scope of this adjudication: the curated dialogue trajectory only — the **revised
> (v9)** Holder's `HOLDER.md` P4 implementation spec and the two falsifiers'
> `FALSIFIER.md` challenges — plus the `SEED.md` charter, with the **v8** `HOLDER.md`
> and the **v8** collaboration ledger (its `M7` finding's `challenge:` field, the
> `rationale:` "Required fix" paragraph, and the `§5` carry-forward entries) as context
> for what the revision had to fix. RFC 0142 is accepted (D258); this judges the **P4
> implementation shape**, not the five-layer design. A clearing verdict is `accept` or
> `accept_with_findings`; this is the latter — the gate **clears**, with two non-blocking
> build-phase findings recorded for the `rfc-0142-p4-build` run. The ledger verdict — not
> falsifier completion — clears the phase gate.

## 1. Decision

`accept_with_findings`. The revision is real, well-anchored, and it **genuinely resolves
the single binding cycle-1 finding M7** — the un-derived `complete`/decoupled/revoke-embedding
row-16 cell — via the **prescribed Option 1** (the clean fix parallel to the M6 fix), and it
**carries M6(rows 13/15), M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 forward
intact**. **Both** falsifiers — from the two independent lenses that converged on M7 in v8
(falsifier_1 decoupling-boundary/decision-table; falsifier_2 carry-forward/regression) —
**independently concede** the resolution and find **no carry-forward regression and no new
material design gap**. The boot-path decision table is **complete and executable**: every
complete-row cell is derived from A's fingerprint-sync predicate, F18 is parametric over all
seven A-reaching complete-row cells, Universal Invariant B holds in every cell, and the
legitimate fresh-DB cells still serve. The only residuals are two **non-blocking build-phase
obligations** (finding B1), which is why the verdict is `accept_with_findings` rather than a
bare `accept`.

- **M7 (the row-16 cell not derived from A) — RESOLVED via Option 1.** §0.2 adds the two
  NEW sub-invariants the fix rests on: **(a) A-`revokeEmbedded`-independence of the decoupled
  complete branch** — step 0 (`revokeEmbedded && !decoupledEnabled → awaiting_deploy_config`)
  is the ONLY A predicate that reads `revokeEmbedded` and it does **not** fire when
  `decoupledEnabled == true`, so A's decoupled complete branch (§3.3a step 3) reads neither
  `applied_owner` NOR `revokeEmbedded`, only `cursor.plan_hash` + the recorded
  `schema_state.fingerprint`, so rows 15 and 16 take the **identical** A3-conditional outcome
  in every W-passing column; **(b) derivation-rule completeness** — every A-reaching
  complete-row cell whose A outcome is fingerprint-conditional is **written conditionally**
  (`HOLDER.md:171-188`). §3.5 row 16 in the `==0`, `==20`, **and** `>=21` columns becomes
  conditional "**SERVE-verify if in-sync, else `awaiting_deploy`**", identical to row 15
  (`HOLDER.md:656-666`); the normal pre-0021 `==0`/`==20` state is documented **out-of-sync**
  (→ `awaiting_deploy`, the dominant case) and the normal post-0021 `>=21` state **in-sync**
  (→ serve-verify, the dominant case), with the opposite sub-case the degenerate corner A
  must handle. F18 is **parametric** over the seven A-reaching complete-row cells {13/`==0`,
  13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`} with the in-sync/out-of-sync
  sub-dimension (`HOLDER.md:927-941`). The four `:399`-reaching cells {1/`==0`, 1/`==20`,
  13-in-sync/`==0`, 13-in-sync/`==20`} are **unchanged** (row 16 is decoupled →
  `ConnectAndVerify` never reaches `:353`/`:399`), so §4.5 and the F18 spy list still agree
  (`HOLDER.md:698-721,885-917`). The v8 F18 refutation cell — the in-sync row-16 cell,
  constructible because `schema_state` and `owner_bundle_meta` are orthogonal — is now
  **correctly handled**: W passes, A returns nil (serve verify-only), and §3.5 row 16 now
  says serve (conditional on in-sync). F18 is no longer a false oracle.

- **M6(rows 13/15), M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, C3 — INTACT.** Carried
  forward verbatim; both falsifiers concede no regression. The row-16-local M7 fix does not
  re-collapse the row-1 fresh-DB serve (`owner.go:145`, source-verified), regress the rows-13/15
  conditional cells, weaken the M3 A0 config gate (cells 2/6/10/14 still halt
  `awaiting_deploy_config`), regress the BC-N2 `applied_owner == 20` non-complete edge (cells
  5/7/9/11 verbatim), advance `Required` (`owner.go:35` = 20, source-verified), or extend M1
  to serve-boot (Option 1, not Option 2).

### Adjudicator source-verification (this cycle)

Direct read of the run worktree (HEAD `3280708f`; `git diff --stat 3f9d5734 HEAD --
go/pkg/db/` **EMPTY**, so every anchor the holder cites against `3f9d5734` holds
byte-identically — the holder's cited HEAD `83fe6465` is an **immaterial citation drift**,
the `go/pkg/db` source is unchanged):

- `owner.go:145` `if applied == 0 { return nil }` returns **before** the
  `< RequiredOwnerBundleVersion` shortfall at `:148-150` — the **M5 fresh-DB serve**.
- `owner.go:23` `LatestOwnerBundleVersion = 20`, `:35` `RequiredOwnerBundleVersion =
  LatestOwnerBundleVersion` (= 20, **NOT advanced**) — the **C2/M7 invariant**.
- `schema_drift.go:145-161` `LiveFingerprint` reads **only** the `striatumd.schema_state`
  singleton; `:171-195` `RecordSchemaFingerprint` writes **only** it — grep confirms **zero**
  `owner_bundle_meta`/`applied_owner` references (the `applied` hits are `applied_at`, a
  `schema_state` timestamp column, and comments). The decoupled complete branch is genuinely
  `applied_owner`- **and** `revokeEmbedded`-independent — the **orthogonality the M6 + M7
  derivations rest on**.
- `connection.go:349/:353/:399` boot order W → `ApplyMigrations` → `RecordSchemaFingerprint`
  ("This is the only writer of schema_state") confirmed.

The M7 derivation is real and source-accurate.

## 2. Per-finding / per-challenge ledger

| # | Source | What is judged | Material? | Resolved / rebutted? | Disposition |
| --- | --- | --- | --- | --- | --- |
| **M7** | cycle-1 binding finding / both falsifiers' re-attack (`dialogue:2`,`dialogue:3`) | row 16 `==0`/`==20`/`>=21` derived from the `applied_owner`- and `revokeEmbedded`-independent A3 predicate (conditional == row 15); §0.2 sub-invariants; F18 parametric over the 7 A-reaching cells; the 4-cell `:399` spy list unchanged (`HOLDER.md:171-188,493-502,656-666,927-941`) | **Yes** (the binding finding) | **Yes — RESOLVED** (both falsifiers concede; adjudicator source-verified the orthogonality + the unchanged spy list) | **RESOLVED (answered)** |
| **M6** | carry-forward / both falsifiers | rows 13/15 `==0` conditional == `==20`; the 4-cell `:399` enumeration; cross-row audit (`HOLDER.md:654-656,681-688,698-721`) | — | **Yes — INTACT** (not re-collapsed by M7) | **INTACT** |
| **M5** | carry-forward / both falsifiers | `{0/1..19/==20/>=21}` split; cell 1/`==0` serve; F18/F18a (`HOLDER.md:604-614,642,942`; `owner.go:145`) | — | **Yes — INTACT** (source-verified `owner.go:145`) | **INTACT** |
| **M3** | carry-forward / both falsifiers | hoisted step-0 config gate (cells 2/6/10/14) + no-revoke `complete` pure-read + F17/F11(g) (`HOLDER.md:484-489,642-655`) | — | **Yes — INTACT** (row 16 is decoupled, does not re-open the legacy bypass) | **INTACT** |
| **M4** | carry-forward / both falsifiers | F16a synthetic (step 2) + F16b production + forced self-heal (step 7) (`HOLDER.md:431-447`) | — | **Yes — INTACT** | **INTACT** |
| **M1** | carry-forward / both falsifiers | `VerifyStoredTranscript` on resume AND finalizer step 0; NOT extended to serve-boot (Option 1) (`HOLDER.md:566-579`) | — | **Yes — INTACT** | **INTACT** |
| **M2** | carry-forward / both falsifiers | single non-revoke filter + in-loop guards + embed/listing split (`HOLDER.md:404-429`) | — | **Yes — INTACT** | **INTACT** |
| **BC-N1** | carry-forward / both falsifiers | immutable `deploy_plan`; resume off the stored transcript; §1.3 + doctor + F14 (`HOLDER.md:287-320`) | — | **Yes — INTACT** | **INTACT** |
| **BC-N2** | carry-forward / both falsifiers | universal non-`complete` edge at `applied_owner == 20`; cells 5/7/9/11 verbatim; F11(e)/(f) (`HOLDER.md:646-653`) | — | **Yes — INTACT** (M7 is the `complete`/decoupled cell, not the non-complete edge) | **INTACT** |
| **C1** | carry-forward / both falsifiers | `finalizing` + idempotent finalizer + §1.3 row + F10 (`HOLDER.md:314-320`) | — | **Yes — INTACT** | **INTACT** |
| **C2** | carry-forward / both falsifiers | `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `Required = 20` (`HOLDER.md:805-813`; `owner.go:35`) | — | **Yes — INTACT** (source-verified `Required = 20`) | **INTACT** |
| **C3** | carry-forward / both falsifiers | 0021 special-cased + terminal + revoke-last; F12/`G-revoke-last` (`HOLDER.md:849-861`) | — | **Yes — mechanism INTACT** (M7 imports a C3 reachability fact into A — a derivation defect now fixed, not a C3 regression) | **INTACT** |
| **B1** | both falsifiers' residual notes (`dialogue:2`,`dialogue:3`) | F18 must construct the row-16 in-sync/out-of-sync sub-cases and assert no `:399` spy; the `64-cell` shorthand must expand to concrete cursor-state enum coverage | **No** (build-phase, not a spec falsification) | n/a — **deferred to the build run** | **DEFERRED (build-phase finding)** |

## 3. Why the verdict turns the way it does

### M7 — the row-16 cell derived from A: RESOLVED

The v8 break was that §3.5 row 16 (`cursorState=complete`, decoupled ON, revoke-embedding)
gave the `==0`/`==20` columns **unconditional** `awaiting_deploy` and `>=21` **unconditional**
`SERVE-verify`, reasoned from an owner-watermark reachability premise (`owner_bundle_meta < 21
⟹ fingerprint mismatch`) that A neither reads nor verifies — so the in-sync row-16 cell was
constructible (the `schema_state`/`owner_bundle_meta` tables are orthogonal) and A served
verify-only there while §3.5 said halt, making F18 a **false oracle** and violating the
holder's own derivation rule. The v8 ledger prescribed **Option 1** (the clean fix parallel
to the M6 fix): make row 16 and its `>=21` variant conditional on the A3 fingerprint
predicate and propagate through §1.3/§3.3a/§3.5/§4.5/F18.

The v9 revision implements exactly that. The crux is the **A-`revokeEmbedded`-independence of
the decoupled complete branch**: step 0 (the only `revokeEmbedded`-reading A predicate) does
not fire when `decoupledEnabled == true`, and §3.3a step 3's decoupled branch reads only
`cursor.plan_hash` + the recorded `schema_state.fingerprint`. I confirmed against source that
`LiveFingerprint`/`RecordSchemaFingerprint` touch only `schema_state` (zero `owner_bundle_meta`
references), so the branch is genuinely `applied_owner`- and `revokeEmbedded`-independent.
Therefore rows 15 (no-revoke decoupled) and 16 (revoke-embedding decoupled) **must** take the
identical conditional outcome in every W-passing column — and the v9 table now writes them
that way. The `>=21` column reaches A for the revoke-embedding binary (W tolerates forward)
and takes the same conditional A3 outcome (normally in-sync → serve-verify, the activated
steady state); rows 13/15 `>=21` halt at W (barrier b, no-revoke), which is why the seven
A-reaching complete-row cells are exactly {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`,
16/`==20`, 16/`>=21`}. F18 is made parametric over precisely those seven cells. The v8 F18
refutation cell now produces the same outcome in A, §3.5, §4.5, and F18.

**Both falsifiers independently concede the resolution.** Falsifier_1 (decoupling-boundary
lens) re-ran the v8 refutation cell and found row 16 now derived from A, the `>=21` cell
conditional, and the §4.5/F18 spy list still at four cells with no row-16 additions.
Falsifier_2 (carry-forward lens) ran four regression attempts (M6 rows-13/15 + spy list; M5
fresh-DB serve + M3 config gate; M1/M2/M4/C3 under Option 1; a completeness gap outside the
complete rows) and found none standing.

### The decision table is complete and executable

All 64 cells are derived from W (the `applied_owner` dimension) and A (the cursor/fingerprint
dimension). The `==0`/`==20` column identity (M6) and the row-15/row-16 decoupled identity
(M7) are closed by **derivation**, not cell-patching, and the cross-row audit walks all four
cursor groups so no sibling cell is left unexamined. Universal Invariant B holds in every cell:
the legacy `:399` writer is reached only by a no-revoke flag-OFF binary over no transcript
(cells 1/`==0`, 1/`==20`) or a `complete` transcript already byte-in-sync (13-in-sync/`==0`,
13-in-sync/`==20`); row 16 is decoupled and reaches `:399` in no column; the legitimate
fresh-DB / inert-landing cells still serve (not wedged). §4.5 and the F18 spy list enumerate
the same four `:399`-reaching cells.

### Carry-forwards intact

Source-verified that the M7 fix advances nothing it must not: `Required = Latest = 20`
(`owner.go:35`), the M5 `applied_owner == 0` serve (`owner.go:145`), and the orthogonality
anchors (`schema_drift.go:145-161`/`:171-195`) all hold. The M3 A0 gate, the BC-N2 `==20`
edge, the rows-13/15 conditional cells, M1's deploy-resume/finalizer scope, M2's filter, the
C3 revoke-last mechanism, and BC-N1/C1 are all preserved verbatim. Both falsifiers concede
each.

## 4. Findings carried to the build run (why `accept_with_findings`, not `accept`)

Two **non-blocking build-phase obligations** (finding **B1**, deferred to `rfc-0142-p4-build`)
— raised by both falsifiers as implementation watchpoints, not holder-spec falsifications:

1. **F18 must actually exercise the row-16 derivation.** `T-deploy-bootpath-decision-table`
   must construct the row-16 **in-sync AND out-of-sync** sub-cases for `applied_owner == 0`,
   `==20`, and `>=21` (the in-sync arm independently setting `schema_state.fingerprint ==
   ExpectedFingerprint()` AND `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent
   / 20 / `>=21` DB, proving the orthogonality) and assert the in-sync row-16 cells serve
   verify-only **without** firing the `ApplyMigrations`/`RecordSchemaFingerprint` spies. If the
   build omits those cases or treats row 16 as a normal-only shortcut, M7 recurs in code.
2. **Expand the grouped cursor-state shorthand.** The "64-cell" shorthand groups
   `step_committed` with `in_progress` and `aborted` with the non-complete edge (16 row-groups
   × 4 buckets). The executable test must table-drive each **concrete** cursor-state enum named
   by F18 (`none`/`in_progress`/`step_committed`/`finalizing`/`complete`/`aborted`), since the
   implementation operates on the enum, not the prose group label.

Neither blocks the design gate; both are recorded so the build honors them.

## 5. What cleared (and must not be regressed downstream)

- **M7** — RESOLVED: §3.5 row 16 `==0`/`==20`/`>=21` conditional on the A3 fingerprint
  predicate (identical to row 15); §0.2 sub-invariants; F18 parametric over the seven
  A-reaching complete-row cells; the four `:399`-reaching cells unchanged; the normal pre-0021
  state documented out-of-sync; §1.3/§3.3a/§3.5/§4.5/F18 consistent.
- **M6(rows 13/15) + M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3** — INTACT
  and unregressed (source-verified for the M5 serve, the `Required = 20` watermark, and the
  M6/M7 orthogonality).
- **Q3** (the per-step resumability body + the full boot-path table proving Invariant B and
  coherence across all complete-row cells) and **Q4** (plain verb + three seams) remain
  resolved with concrete mechanisms.

The gate **clears** on `accept_with_findings`. The `rfc-0142-p4-build` run may proceed
contract-first against this spec, honoring finding B1.

<sub>Adjudicator collaboration ledger (v9 / REVISION), cycle 1, for the RFC 0142 P4
falsification-gate design run. Records the clearing verdict `accept_with_findings`: the single
binding cycle-1 finding **M7** (the §3.5/§4.5/F18 `complete`/decoupled/revoke-embedding row-16
cell asserted unconditionally where A's decoupled complete branch is fingerprint-conditional
and reads neither `applied_owner` NOR `revokeEmbedded`) is GENUINELY RESOLVED via the
prescribed Option 1 — derive row 16 from A under one coherent contract (conditional on A3
fingerprint-sync, identical to row 15), §0.2 sub-invariants added, F18 parametric over the
seven A-reaching complete-row cells, the four `:399`-reaching cells unchanged — both falsifiers
independently concede; M6(rows 13/15)/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward
intact; no new material challenge stands unrebutted; two non-blocking build-phase obligations
(finding B1) deferred to `rfc-0142-p4-build`. Adjudicator source-verified against the run
worktree HEAD `3280708f` (`git diff --stat 3f9d5734 HEAD -- go/pkg/db/` EMPTY): `owner.go:23/:35`
(`Latest = Required = 20`), `owner.go:145` (the `applied == 0` serve), `schema_drift.go:145-161`/`:171-195`
(`schema_state` orthogonal to `owner_bundle_meta`, zero references), and `connection.go:349/:353/:399`
boot order all hold byte-identically.</sub>

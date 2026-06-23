---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec (v6 REVISION; the two cycle-1 findings M3 (complete-cursor activation guard / tightened Invariant B) and M4 (F16 phase-aware split) are GENUINELY RESOLVED and M1/M2/BC-N1/BC-N2/C1/C2/C3 carry forward intact — BUT a NEW material decision-table completeness defect M5 (the §3.5/F18 owner-watermark `applied_owner < 20` bucket collapses the fresh-DB `applied_owner == 0` legacy-serve cell into the `1..19` shortfall halt) lands INDEPENDENTLY from both falsifiers and stands unrebutted; adjudicator source-verified against current `main` HEAD df2a9e70)"
participants:
  - "holder-author-001"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-002"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: "Revised P4 spec (v6) resolves the two binding cycle-1 findings by concrete sub-protocols bound to named code sites, produces the proactive-completeness boot-path decision table (§3.5), and carries M1/M2/BC-N1/BC-N2/C1/C2/C3 forward verbatim, re-verified against current `main` (HEAD df2a9e70; `git diff --stat 2e482077 HEAD -- go/` empty, so every v5 source anchor holds). M3 by HOISTING the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config halt to step 0 of `CheckDeployActivation`, BEFORE any cursor-state branch, so it fires for EVERY cursor state including `complete` (barrier (a), HOLDER.md:349-355); a no-revoke binary over a `complete` transcript with the flag OFF runs a pre-`ApplyMigrations`, DB-untouched PURE-READ comparison (`ExpectedFingerprint() == LiveFingerprint(recorded)` AND `cursor.plan_hash == expected`) and serves only when already in-sync (so legacy `ApplyMigrations` is a no-op and the `:399` self-record is an idempotent rewrite of the SAME fingerprint), else halts `awaiting_deploy` DB-untouched (HOLDER.md:362-378). Universal Invariant B is TIGHTENED (§4.5) so the legacy `connection.go:399` writer is reachable ONLY in §3.5 cells 1 (`none`, no transcript) and 13-in-sync (`complete`, no-revoke, already byte-in-sync), barred from any revoke-embedded state (barrier a), any revoke-applied state (barrier b: the C2 forward-watermark at `applied_owner >= 21`), and any pending-change-over-a-transcript state (the A3 comparison). The exhaustive §3.5 decision table specifies all 48 `cursorState × decoupledEnabled × revokeEmbedded × applied_owner ∈ {<20,==20,>=21}` cells and asserts Invariant B in every cell; F17 (`T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`), F11(g), and F18 (`T-deploy-bootpath-decision-table`) refute it. M4 by SPLITTING F16 phase-aware: F16a (`TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, rollout step 2) drives the exclusion filter through a synthetic bundle list / injectable-FS test hook WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (`TestOwnerDDLApplyExcludesProductionRevokeBundle`, rollout step 7 after 0021 is authored) asserts the production embed/listing split and houses the forced FMA-007 self-heal pgtest required to reach `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError` (owner.go:367-374). M1 (`VerifyStoredTranscript` on every resume AND as finalizer step 0; typed `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`; F15 + extended F14), M2 (the single non-revoke filter `OwnerDDLApplyBundles()` + `isNonRevokeBundle` across every `owner-ddl apply` route incl. FMA-007 self-heal; the embed/listing split), BC-N1 (immutable `deploy_plan` transcript materialized before step 0; resume off the stored transcript; §1.3 + doctor + F14), BC-N2 (the universal non-`complete`-cursor edge at `applied_owner == 20`; F11(e)/(f)), C1 (`finalizing` + idempotent finalizer + §1.3 row + F10), C2 (`CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion` KEPT at 20), and C3 (0021 special-cased + terminal + revoke-last, F12/`G-revoke-last`) are carried forward verbatim and unregressed."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "Revision check (CONCEDED): the v6 holder genuinely closes the v5 M3 reproducer. The config gate is now step 0, before every cursor-state branch — `revokeEmbedded && !decoupledEnabled` returns `awaiting_deploy_config` DB-untouched for `none`/`in_progress`/`finalizing`/`complete` (HOLDER.md:349-355); the `complete` branch no longer short-circuits it; a no-revoke legacy binary gets a pre-`ApplyMigrations` pure-read comparison and serves only when `ExpectedFingerprint() == LiveFingerprint(recorded)` and `cursor.plan_hash == expected` (HOLDER.md:362-378); the decision table marks the old M3 cell (`complete`, flag off, revoke-embedding) `awaiting_deploy_config`, incl. the post-deploy steady-state `applied_owner >= 21` case (cell 14); F17 asserts `awaiting_deploy_config`, `ApplyMigrations` uncalled, `RecordSchemaFingerprint` uncalled, `schema_state` unchanged, DB byte-identical, shadow-mode fall-through never reached. So M3 is NOT kept open; Universal Invariant B is tightened around the legacy `connection.go:399` writer; the BC-N2 non-complete edge is not weakened; C2's required owner frontier remains 20 and the forward-watermark stays at `applied_owner >= 21` (`owner.go:23,35`); no regression in M1's deployer verifier, C1's gated finalizer, or the M2/C3 revoke-last path from this lens. NEW MATERIAL CHALLENGE (the decision table wedges the fresh/no-authority bootstrap cell): the SEED requires the proactive-completeness table to cover the legitimate fresh-DB / inert-landing cells and PROVE they still serve, not merely prove the old M3 cell halts (SEED.md:318-331), and that lifting `ApplyMigrations` must not break fresh-DB bring-up (SEED.md:367-370). §3.5 contradicts that: W = `CheckOwnerBundleWatermark` maps `applied_owner < 20 → awaiting_owner_ddl` and states the `<20` column is UNIFORMLY `awaiting_owner_ddl` (HOLDER.md:443-459); row 1's only serving no-revoke/no-transcript legacy cell is `applied_owner == 20`; the `<20` cell halts; the prose calls cell 1/`==20` the fresh/inert cell that still serves (HOLDER.md:515-518). That is NOT current source behavior: `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent (owner.go:228-236); `CheckOwnerBundleWatermark` documents applied 0 as a 'fresh single-role database' that must NOT halt and returns nil for `applied == 0` BEFORE the `< RequiredOwnerBundleVersion` shortfall check (owner.go:116-149); `owner_pg_test.go:19-20` asserts a fresh migrated DB starts at owner bundle version 0. Concrete refutation: cursorState=none, decoupledEnabled=false, revokeEmbedded=false, applied_owner=0 — under current source the watermark returns nil and §3.3a step 4 returns nil, so legacy `ConnectAndMigrate` performs the normal fresh-DB bring-up; under the v6 §3.5/F18 executable table the same cell is row 1/`<20` and must return `awaiting_owner_ddl`. An implementation following F18 literally either (1) changes the source to halt a fresh no-authority DB, regressing fresh/single-role bootstrap, or (2) preserves the `applied == 0` exception, making the executable table and its F18 oracle false. Either branch is a material table-correctness failure. NOT a re-opening of M3 (the hoisted gate is right for revoke-embedding binaries); the bug is the table collapsing two `<20` states (`== 0` legitimate fresh serve vs `1..19` legitimate `awaiting_owner_ddl` halt) into one column that cannot state both. Required repair: split the watermark dimension `applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}`, specify the no-transcript/no-revoke/flag-off bootstrap cell as serve-legacy for `applied_owner == 0` while retaining `awaiting_owner_ddl` for `1..19`, make F18 assert both cells explicitly, and stop labeling cell 1/`==20` the fresh-DB cell. The gate should require the watermark dimension split before the v6 proposal clears."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "Revision check (CONCEDED): the v6 holder genuinely fixes the v5 M4 staging defect. F16a (`TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, step 2) drives the filter through a synthetic bundle list / test hook, asserts `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`, both `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles` skip a hand-passed synthetic 0021, and the nil fallback resolves to the filtered loader, and explicitly does NOT assert production `OwnerBundles()` contains 0021 yet (HOLDER.md:296-309); F16b (`TestOwnerDDLApplyExcludesProductionRevokeBundle`, step 7 after 0021 authored) asserts production `OwnerBundles()` contains 0021, `ExpectedFingerprint()` includes its bytes, `revokeEmbedded` from the full loader/file presence, and production `OwnerDDLApplyBundles()` excludes it, with the forced self-heal pgtest living here (HOLDER.md:310-316,663-664). That resolves M4 as specified by the v5 ledger; no M2 regression in the restructuring — the single `DDLRevokeOwnerBundleVersion = 21`/`isNonRevokeBundle` filter, the split `OwnerDDLApplyBundles()` loader, in-loop guards on both apply loops, and the nil-fallback split are kept; `OwnerBundles()` remains the full loader only for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable` (HOLDER.md:269-294,587-620); the CLI surface still has only `striatum daemon owner-ddl apply` → `db.ApplyOwnerBundles`, no sibling dry-run/list route (daemon.go:76-156). So M4 is NOT kept open and 0021 is not reachable through an `owner-ddl apply` route. NEW MATERIAL CHALLENGE (the F18 owner-watermark table is false for fresh/no-authority DBs): the SEED requires the table to cover every cell incl. the legitimate fresh-DB bring-up / inert-landing cells that must still serve (SEED.md:318-334), and the holder turns it into executable F18 coverage requiring the matrix to assert the exact §3.5 outcome for every `applied_owner` bucket (HOLDER.md:433-441,666). But §3.5 collapses two distinct owner-watermark states into one `<20` bucket: it maps `applied_owner < 20 → awaiting_owner_ddl`, states `<20` 'ALWAYS halts', and makes every `<20` row uniformly `awaiting_owner_ddl` (HOLDER.md:443-478), then calls cell 1/`==20` the fresh-DB/inert-landing cell that still serves (HOLDER.md:515-518). That is not the live watermark contract: current source makes `applied == 0` the fresh/single-role/no-authority bootstrap case and returns nil before the shortfall check; only `1 <= applied < RequiredOwnerBundleVersion` is a real `awaiting_owner_ddl` shortfall (owner.go:116-149). Concrete refutation: cursorState=none, decoupledEnabled=false, revokeEmbedded=false, applied_owner=0 — `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent, `CheckOwnerBundleWatermark` returns nil for that 0, the boot takes the existing fresh/single-role legacy path; under the v6 F18 table the same cell is row 1/`<20` and must halt `awaiting_owner_ddl`. So an implementation either (1) follows the table and regresses fresh/no-authority bring-up, or (2) keeps the `applied == 0` exception and makes the F18 matrix oracle false for the required `<20` bucket. Material owner-column failure, not an M4 staging nit — it directly touches the packet's 'P2 watermark interlock / fresh-DB bring-up' check; the old M3 cell halts, but the table misclassifies the owner-watermark bootstrap state. Required repair: split the owner-watermark dimension `applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}` and make F18 assert both branches explicitly (`applied_owner == 0` → serve-legacy/fresh bootstrap, `ApplyMigrations`/legacy self-record may run because no deploy transcript exists; `1 <= applied_owner < 20` → `awaiting_owner_ddl` DB-untouched), propagated through the other cursor rows so the table stays executable without changing the bootstrap contract. The v6 proposal should not clear until the owner-watermark dimension distinguishes `0/no authority` from `1..19 authority shortfall`."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "REVISION cycle v6 (the gate's single allowed revision cycle). The two binding cycle-1 findings are each GENUINELY RESOLVED as concrete sub-protocols bound to named code sites — and BOTH falsifiers independently concede each one. M3 (the v5 complete-cursor legacy mutate+self-record bypass) is closed by HOISTING the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config halt to step 0 of `CheckDeployActivation`, BEFORE any cursor-state branch, so it fires for EVERY cursor state incl. `complete` (barrier a, HOLDER.md:349-355); the no-revoke `complete` residual is handled by a pre-`ApplyMigrations`, DB-untouched PURE-READ comparison (`ExpectedFingerprint() == LiveFingerprint(recorded)` AND `cursor.plan_hash == expected`) that serves only when already in-sync (HOLDER.md:362-378); Universal Invariant B is TIGHTENED so the legacy `connection.go:399` writer is reachable ONLY in §3.5 cells 1 (`none`, no transcript) and 13-in-sync (`complete`, no-revoke, already byte-in-sync), barred from any revoke-embedded (barrier a), revoke-applied (barrier b, the forward-watermark at `applied_owner >= 21`), or pending-change-over-a-transcript state (HOLDER.md:622-642). F17 + F11(g) + F18 are named with the exact assertions the v5 ledger prescribed (`awaiting_deploy_config`, `ApplyMigrations`/`RecordSchemaFingerprint` un-called, `schema_state` unchanged, DB byte-identical, shadow-mode fall-through never reached). falsifier_1 (the lens that found M3) EXPLICITLY states it does not keep M3 open. ADJUDICATOR SOURCE-VERIFICATION this cycle: the boot order `connection.go:349 (CheckOwnerBundleWatermark) → :353 (ApplyMigrations) → :376-383 (CheckSchemaDrift) → :384-393 (shadow log + fall-through) → :399 (RecordSchemaFingerprint, 'the only writer of schema_state')` is accurate; a pre-`:353` guard that returns nil is exactly what let the v5 legacy mutator + self-record run, and the M3 fix halts BEFORE `:353` — a concrete sub-protocol, not a claim. M4 (the v5 F16 test-staging defect) is closed by SPLITTING F16 into F16a (synthetic-list pre-0021, step 2) + F16b (production post-0021 incl. the forced FMA-007 self-heal reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`, step 7) (HOLDER.md:296-316). falsifier_2 EXPLICITLY states it does not keep M4 open and finds no M2 regression in the restructuring. The seven carry-forwards M1/M2/BC-N1/BC-N2/C1/C2/C3 are carried forward verbatim and INTACT; both falsifiers concede no regression. BUT a clearing verdict ALSO requires (a) the proactive-completeness boot-path decision table COMPLETE with Universal Invariant B proven in EVERY cell AND the legitimate fresh-DB bring-up / inert-landing cells still serving (not wedged), and (b) no new material challenge standing unrebutted — and BOTH fail on the SAME defect (M5). BOTH falsifiers, from two independent lenses (decoupling-boundary; owner-ddl/test-staging), CONVERGE on a NEW material decision-table completeness failure: §3.5 maps `applied_owner < 20 → awaiting_owner_ddl` and states the `<20` column is UNIFORMLY `awaiting_owner_ddl` and 'ALWAYS halts at W' (HOLDER.md:443-459,461-478), but current source returns nil (SERVES) for `applied_owner == 0` — the fresh / single-role / no-authority bootstrap case — and halts `awaiting_owner_ddl` ONLY for `1 <= applied < RequiredOwnerBundleVersion (20)`. ADJUDICATOR SOURCE-VERIFICATION this cycle (direct read of the run worktree off HEAD df2a9e70): `CheckOwnerBundleWatermark` returns nil for `applied == 0` BEFORE the shortfall check (`go/pkg/db/owner.go:145-147`, with the explicit comment at :116-123 and :140-143 that a fresh 0-watermark DB 'is the bootstrap case ... NOT halted' and only `applied >= 1` lagging the frontier is a true shortfall); `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent (`owner.go:234-235`). BOTH falsifiers' source citations are accurate. The §3.5/F18 table therefore (1) WEDGES the legitimate fresh no-authority boot (cursorState=none, no-revoke, flag-off, applied_owner=0) that the SEED explicitly requires to 'still serve and NOT be wedged' (SEED.md:318-331) and that current source serves, OR (2) makes the EXECUTABLE F18 matrix (`T-deploy-bootpath-decision-table`, required to assert the exact §3.5 outcome for every `applied_owner ∈ {<20,==20,>=21}` cell, HOLDER.md:666) a false oracle for the `applied_owner == 0` cell. The holder's own prose compounds it by labeling cell 1/`==20` — an already-owner-bundled DB at version 20 — the 'fresh-DB bring-up' cell (HOLDER.md:515-518), when the genuine fresh no-authority DB is `applied_owner == 0`. This is MATERIAL (it requires a spec edit — split the `applied_owner` dimension into `{0/no authority, 1..19 shortfall, ==20, >=21}` and respecify F18 to assert both cells — and exposes a real correctness defect in the required, executable decision table). It matches the rubric's explicit needs_revision trigger 'a decision-table cell that ... wedges a legitimate boot', and it is precisely the SEED's warned-of failure mode: 'A re-scaffolded revision that pins exactly the two §4 items but leaves an unaudited boot-path combination open will simply spawn an M5.' It STANDS UNREBUTTED — this cycle carries no holder rebuttal artifact; the falsifiers are the last word and the holder's spec text (HOLDER.md:455-456) affirms the wrong rule. The holder's anticipatable rebuttal (that `<20` was meant to mean authority-bearing) is preempted by both falsifiers: the SEED + F18 explicitly enumerate `applied_owner < 20` as a matrix bucket, current source classifies the fresh case as version 0 (a member of that bucket), and the prose mislabels which cell is fresh — so the written, executable contract is wrong and no interpretation rescues it. Note this is NOT a re-opening of M3: the fresh-DB cell carries no transcript and no revoke, so Invariant B is not VIOLATED there (the M3 gate is correct for revoke-embedding binaries); the M5 failure is the opposite — an over-conservative halt of a cell that must serve. Because the decision table is not complete/correct in the owner-watermark dimension and a new material challenge stands unrebutted, the gate does NOT clear. This is the single allowed revision cycle for v6, so the gate ends unCleared, judged exactly on the merits; §4 records the single load-bearing fix (the owner-watermark dimension split + the F18 fresh-DB serve cell), and §5 records what cleared and must not be regressed (M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, C3 — all genuinely resolved/intact this cycle)."
findings:
  - id: M3
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED — the v5 complete-cursor legacy mutate+self-record bypass is genuinely closed; falsifier_1 (the lens that raised M3) explicitly does not keep it open. The fix is two concrete sub-protocols bound to named code sites: (1) the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config halt is HOISTED to step 0 of `CheckDeployActivation`, BEFORE any cursor-state branch, so it fires for EVERY cursor state incl. `complete` — a revoke-embedding binary with the flag OFF returns `awaiting_deploy_config` DB-untouched before `ApplyMigrations` (`connection.go:353`) and before `RecordSchemaFingerprint` (`:399`) on BOTH `ConnectAndMigrate` and `ConnectAndVerify` (barrier a, HOLDER.md:349-355, decision-table cells 2/6/10/14); (2) for the no-revoke `complete` residual the guard runs a pre-`ApplyMigrations`, DB-untouched PURE-READ comparison (`ExpectedFingerprint() == LiveFingerprint(recorded)` AND `cursor.plan_hash == expected`, all reads per `schema_drift.go:83-100,145-160`) and serves only when already in-sync (so legacy `ApplyMigrations` is a no-op and `:399` rewrites the IDENTICAL fingerprint), else halts `awaiting_deploy` DB-untouched (HOLDER.md:362-378). Universal Invariant B is TIGHTENED (§4.5): the legacy `:399` writer is reachable ONLY in §3.5 cells 1 (`none`, no transcript) and 13-in-sync, barred from any revoke-embedded (barrier a), revoke-applied (barrier b, the C2 forward-watermark at `applied_owner >= 21`), or pending-change-over-a-transcript state (the A3 comparison). F17 (`T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`) + F11(g) + F18 assert `awaiting_deploy_config`, `ApplyMigrations`/`RecordSchemaFingerprint` un-called (spies), `schema_state` unchanged, DB byte-identical, and the shadow-mode fall-through never reached — the exact contract the v5 ledger §4 prescribed. ADJUDICATOR SOURCE-VERIFICATION: confirmed the `connection.go:349/:353/:376-383/:399` boot order directly this cycle — the guard precedes `:353`, so the fix is a concrete sub-protocol, not merely claimed. The BC-N2 non-`complete` edge is NOT weakened (the M3 gate sits ABOVE it and BESIDE it). **M3 RESOLVED.**"
    affected_invariants: ["serve-boot decoupling boundary (no schema mutation as a side effect of restart)", "Universal Invariant B (no self-record around VerifyStoredTranscript when a transcript carrying a pending change is present)", "no revoke-embedding binary ever reaches the legacy connection.go:399 writer", "R3 no #512-class CREATE lockout in any boot path after the revoke", "fail-closed activation interlock for a revoke-embedding binary with the flag OFF, EVERY cursor state"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: M4
    severity: medium
    posture: design
    status: answered
    challenge: "RESOLVED — the v5 F16 test-staging defect (asserting production `OwnerBundles()` contains 0021 in step 2 before 0021 is authored in step 7) is closed by the phase-aware split; falsifier_2 explicitly does not keep it open. F16a (`TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, rollout step 2) drives the exclusion filter through a SYNTHETIC bundle list / injectable-FS test hook — asserts `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`, both `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a hand-passed synthetic 0021, and `ReapplyAllOwnerBundles(nil,…)` resolves to the filtered loader — WITHOUT asserting production `OwnerBundles()` contains 0021, so it builds green before 0021 exists (HOLDER.md:296-309,663). F16b (`TestOwnerDDLApplyExcludesProductionRevokeBundle`, rollout step 7 after 0021 authored) asserts production `OwnerBundles()` contains 0021, `ExpectedFingerprint()` includes its bytes, `revokeEmbedded` derives from the full loader / file presence, and production `OwnerDDLApplyBundles()` excludes it; the forced FMA-007 self-heal pgtest lives here and is required to reach `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError` (`owner.go:367-374`), not merely the pending loop (HOLDER.md:310-316,664). This is exactly the split the v5 ledger §4 prescribed; the M2 filter is unchanged (M4 only restructured F16's staging). **M4 RESOLVED.**"
    affected_invariants: ["F16 lands green incrementally before 0021 is authored (F16a synthetic phase)", "the production embed/listing split proven once 0021 exists (F16b production phase)", "the forced self-heal pgtest reaches ReapplyAllOwnerBundles via isCrossBundleDependencyError", "every load-bearing M2 claim carries a buildable named test"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: M1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim from v5, unregressed; both falsifiers find no regression. `VerifyStoredTranscript(plan_hash)` (§3.4a) checks the ENTIRE stored transcript (every step, already-applied AND not) `sha256` against the running binary's embedded bytes (`MigrationSHASet()` `migrations.go:197`; `OwnerBundle.SHA256()` `owner.go:191`) ⇒ `deploy_plan_binary_mismatch`; for already-applied entries it verifies the DB stamps (`schema_migrations.sha256` / `owner_bundle_meta.sha256`) ⇒ `deploy_plan_db_stamp_mismatch`; the SAME verifier runs on every resume AND as finalizer step 0 before the `complete` receipt / `RecordSchemaFingerprint` / `finalizing → complete` advance (HOLDER.md:419-429). The M3 change EXTENDS Invariant B's enforcement to the legacy/complete-cursor path; it does NOT re-open or gate the deployer verifier — falsifier_1 explicitly confirms no regression in M1's deployer verifier. F15 (extended for the complete-cursor coverage note) + F14 (already-applied tamper) carry forward. **M1 INTACT.**"
    affected_invariants: ["full stored-transcript byte verification on resume AND before the finalizer", "wrong-resume-binary forced to deploy_plan_binary_mismatch", "Universal Invariant B's deployer-finalizer arm (gated by VerifyStoredTranscript step 0)", "P3/P4 fingerprint coherence"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede 0021 stays unreachable through every `owner-ddl apply` route and CREATE stays held. The single non-revoke filter `isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)` + the split loader `OwnerDDLApplyBundles()` bind across `ApplyOwnerBundles` (loads the filtered slice, `owner.go:269`), `applyPendingOwnerBundles` + the FMA-007 self-heal `ReapplyAllOwnerBundles` (in-loop guards), and the nil-fallback (filtered slice); `OwnerBundles()` (full, includes 0021) is kept ONLY for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable` (HOLDER.md:269-294,587-620). M4 only restructured F16's STAGING (the synthetic-vs-production split); it did NOT change the filter — falsifier_2 verified this explicitly and found no sibling owner-ddl dry-run/list route in current source (daemon.go:76-156). **M2 INTACT.**"
    affected_invariants: ["0021 unreachable through every owner-ddl apply route (incl. FMA-007 self-heal)", "embed/listing split (revokeEmbedded/ExpectedFingerprint see 0021; owner-ddl apply never iterates it)", "R3 no early REVOKE CREATE before a CREATE-needing reconcile", "C3 revoke-last ordering holds outside the happy deploy plan"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: BC-N1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The runtime-owned `deploy_plan` table (migration 0044) storing the IMMUTABLE ordered transcript keyed by `plan_hash`, materialized in the same tx that sets `deploy_cursor → in_progress(0)` BEFORE step 0; resume loads `deploy_plan[cursor.plan_hash]` and never recomputes `BuildPlan` over the moved frontier; the §1.3 stored-transcript reclassification; the transcript-enumerated doctor; F14 (HOLDER.md:156-226). The M3 change touches only the activation predicate and the decision table; it does NOT re-open the moving-frontier mechanism. **BC-N1 INTACT.**"
    affected_invariants: ["BC-N1 immutable plan identity across resume", "stable (plan_hash, step_index) receipt key", "section 1.3 incomplete-vs-drift disambiguation", "resume off the stored transcript, never a recomputed BuildPlan"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: BC-N2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; neither falsifier uses it as a blocker and both confirm M3 is the ORTHOGONAL complete-cursor window, not the BC-N2 non-complete window. `CheckDeployActivation` is not gated on `revokeEmbedded`; the universal incomplete-deploy edge (step 1) still halts `awaiting_deploy` DB-untouched on every non-`complete` cursor at `applied_owner == 20` (HOLDER.md:356-360, decision-table cells 5/7/9/11). M3's fix sits ABOVE it (the hoisted config gate, step 0) and BESIDE it (the `complete` branch, step 3) — it does NOT weaken the non-complete edge; F11(e)/(f) + the extended `G-old-binary-refuse` carry forward. NOTE: the new M5 challenge concerns the ORTHOGONAL owner-watermark `applied_owner < 20` dimension at W (`CheckOwnerBundleWatermark`), not the BC-N2 `deploy_cursor` edge at A — BC-N2's resolved `applied_owner == 20` edge is itself not regressed. **BC-N2 INTACT.**"
    affected_invariants: ["BC-N2 universal pre-revoke deploy_cursor-authoritative edge", "incomplete (non-complete) cursor never serves", "the M3 complete-cursor extension does not weaken the non-complete edge"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C1
    severity: critical
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The `finalizing` state, the idempotent finalizer (VerifyStoredTranscript step 0 → guarded `complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST), the §1.3 resumable-finalization row, and F10 are preserved (HOLDER.md:183-189,409-417). M3's harm flowed through the LEGACY `connection.go:399` writer — a DIFFERENT writer than the C1 deployer finalizer (which IS gated by VerifyStoredTranscript). M3 tightens Invariant B to cover the legacy writer; it does not touch the C1 finalizer's boundary semantics — falsifier_1 explicitly confirms no regression in C1's gated finalizer. **C1 INTACT.**"
    affected_invariants: ["finalization boundary (no complete with missing receipt/fingerprint)", "idempotent finalizer repaired exactly once by the right binary, gated by VerifyStoredTranscript step 0", "section 1.3 disambiguation enforcement"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: C2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; falsifier_1 confirms the required owner frontier remains 20 and the forward-watermark stays anchored at `applied_owner >= 21`. `CheckDeployActivation` before `ApplyMigrations` in both boot paths (mutating nothing), the typed `awaiting_deploy`/`awaiting_deploy_config` halts, the forward-watermark rule firing at `applied >= 21` (`owner.go:151-153`, barrier b), and `RequiredOwnerBundleVersion` KEPT at its current-`main` value 20 (`owner.go:35`, NOT advanced to the revoke ordinal 21) all carry forward (HOLDER.md:339-395,540-558). M3 EXTENDS this edge to the `complete`-cursor case (the hoisted config gate + the no-revoke comparison) without advancing `Required`, altering the watermark, or changing the `applied >= 21` forward rule — exactly as the v5 ledger §4/§5 required. **C2 INTACT.**"
    affected_invariants: ["fail-closed CheckDeployActivation edge before ApplyMigrations", "typed awaiting_deploy/awaiting_deploy_config halts", "forward-watermark rule at applied >= 21 (barrier b)", "RequiredOwnerBundleVersion KEPT at 20, not advanced to the revoke ordinal"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: C3
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede the design. Bundle 0021 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) identified by `DDLRevokeOwnerBundleVersion = 21`, special-cased out of the owner prefix, applied as the TERMINAL plan step after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE granted by 0018; the committed steady state denies CREATE); `LatestOwnerBundleVersion`/`Required` stay 20; F12/`G-revoke-last` (HOLDER.md:397-407,523-558). M2 binds the executable exclusion across the self-heal; M3 keeps 0021 off the legacy serve-boot path. The activation deploy still completes (no stranded `ALTER … OWNER TO striatumd_rw`). **C3 INTACT.**"
    affected_invariants: ["two-role owner/runtime boundary", "revoke-last terminal ordering", "reconcile-while-CREATE-held", "activation deploy can complete"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M5
    severity: high
    posture: design
    status: open
    challenge: "OPEN — NEW material challenge landed INDEPENDENTLY by BOTH falsifiers (falsifier_1 decoupling-boundary lens; falsifier_2 owner-ddl/test-staging lens), source-verified against current `main`, standing unrebutted. The proactive-completeness boot-path decision table (§3.5) and its executable matrix F18 (`T-deploy-bootpath-decision-table`) collapse two DISTINCT owner-watermark states into one `applied_owner < 20` bucket. §3.5 maps `applied_owner < 20 → awaiting_owner_ddl`, states `<20` 'ALWAYS halts at W', and renders every `<20` row uniformly `awaiting_owner_ddl` (HOLDER.md:443-459, 461-478), and the prose calls cell 1/`==20` the 'fresh-DB bring-up / inert-landing cell that still serves' (HOLDER.md:515-518). But current source distinguishes: `CheckOwnerBundleWatermark` returns nil (SERVES) for `applied == 0` — the fresh / single-role / no-authority bootstrap case — BEFORE the shortfall check, and halts `awaiting_owner_ddl` ONLY for `1 <= applied < RequiredOwnerBundleVersion (20)`. ADJUDICATOR SOURCE-VERIFICATION (this cycle, direct read against the run's worktree off HEAD df2a9e70): confirmed `go/pkg/db/owner.go:145-147` returns nil for `applied == 0` BEFORE the `if applied < RequiredOwnerBundleVersion` shortfall check at :148-150, with the function comment at :116-123 ('a 0-watermark database is treated as the bootstrap/single-role case and NOT halted. Only a database that HAS an authority schema (applied >= 1) but lags the required frontier is a genuine shortfall') and the inline comment at :140-143; and `owner.go:234-235` returns 0 when `to_regclass('striatumd.owner_bundle_meta') IS NOT NULL` is false (the meta table absent). Both falsifiers' source citations are accurate. Concrete refutation (the cell the SEED requires to remain legitimate): cursorState=none, decoupledEnabled=false, revokeEmbedded=false, applied_owner=0 — under current source the watermark returns nil, §3.3a step 4 returns nil, and legacy `ConnectAndMigrate` performs the normal fresh-DB bring-up (HOLDER.md:379-385); under the v6 §3.5/F18 table the same cell is row 1/`<20` and must return `awaiting_owner_ddl`. So an implementation following F18 literally either (1) changes the source to halt a fresh no-authority DB at `awaiting_owner_ddl`, REGRESSING fresh / single-role bootstrap and violating the SEED's explicit 'the legitimate fresh-DB bring-up / inert-landing cells (no-revoke binary, no transcript) that must still serve and NOT be wedged' requirement (SEED.md:318-331) and 'lifting ApplyMigrations must not break fresh-DB bring-up' (SEED.md:367-370), OR (2) preserves the `applied == 0` exception, making the EXECUTABLE F18 oracle FALSE for the required `applied_owner < 20` bucket (HOLDER.md:666). Either branch is a material table-correctness failure. This is NOT a re-opening of M3 (the hoisted M3 gate is right for revoke-embedding binaries; the fresh-DB cell carries no transcript and no revoke, so Invariant B is not VIOLATED there — the failure is the opposite, an over-conservative halt of a cell that must serve) — it is a fresh defect in the very decision table the SEED required to PREEMPT exactly this class ('A re-scaffolded revision that pins exactly the two §4 items but leaves an unaudited boot-path combination open will simply spawn an M5'). It matches the rubric's explicit needs_revision trigger: 'a decision-table cell that ... wedges a legitimate boot'. The holder's anticipatable rebuttal — that `<20` was meant to mean authority-bearing, with `0/no authority` implicitly outside the table — is preempted by both falsifiers: the SEED + F18 explicitly enumerate `applied_owner < 20` as a matrix bucket, current source represents the fresh no-authority case AS owner-bundle version 0 (a member of that bucket), §3.5 says `<20` always halts, and the prose mislabels cell 1/`==20` (an already-owner-bundled DB at version 20) as the fresh cell — so the written, executable contract is wrong and no interpretation rescues it. STANDS UNREBUTTED (no holder rebuttal artifact this cycle; the spec text affirms the wrong rule at HOLDER.md:455-456). MATERIAL (requires a spec edit and exposes a real correctness defect in an executable named requirement) → needs_revision. Required fix: split the owner-watermark dimension into `applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}`; specify the no-transcript/no-revoke/flag-off bootstrap cell as SERVE-legacy for `applied_owner == 0` (the legacy `:399` self-record is legitimate there — no deploy transcript exists; Invariant B is not in scope) while retaining `awaiting_owner_ddl` for `1..19`; propagate the split through the other cursor rows so the table stays executable; make F18 assert both the `applied_owner == 0` serve cell and the `1..19` halt cell explicitly; and stop labeling cell 1/`==20` the fresh-DB cell (the genuine fresh no-authority DB is `applied_owner == 0`)."
    affected_invariants: ["boot-path decision table COMPLETE and CORRECT in the owner-watermark dimension (Invariant B proven AND the legitimate fresh-DB cells still serve, not wedged)", "fresh / single-role / no-authority bootstrap (applied_owner == 0) still serves on the legacy path", "the executable F18 matrix oracle matches CheckOwnerBundleWatermark's actual applied==0 exception", "P2 watermark interlock / fresh-DB bring-up carried forward without a behavior change"]
    source_refs: ["dialogue:2", "dialogue:3"]
branches:
  design: blocked
---

# COLLABORATION LEDGER — RFC 0142 P4 design run (v6 / REVISION), cycle 1

author: adjudicator-author-002

**Verdict:** `needs_revision`

> Scope of this adjudication: the curated dialogue trajectory only — the **revised
> (v6)** Holder's `HOLDER.md` P4 implementation spec and the two falsifiers'
> `FALSIFIER.md` challenges — plus the `SEED.md` charter, with the **v5** `HOLDER.md`
> and the **v5** collaboration ledger (its §4 "What the revision must fix" + the M3/M4
> findings) as context for what the revision had to fix. RFC 0142 is accepted (D258);
> this judges the **P4 implementation shape**, not the five-layer design. A clearing
> verdict would be `accept` or `accept_with_findings`; this is neither. This is the
> gate's **single allowed revision cycle**, so the gate ends **unCleared** — judged
> exactly on the merits, not graded on the consequence.

## 1. Decision

`needs_revision`. The revision is real, well-anchored, and it **genuinely resolves both
binding cycle-1 findings** — both falsifiers independently concede each one:

- **M3 (the complete-cursor legacy mutate+self-record bypass) — RESOLVED.** The
  `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config halt is **hoisted
  to step 0** of `CheckDeployActivation`, before any cursor-state branch, so it fires for
  **every** cursor state including `complete` (barrier a, `HOLDER.md:349-355`); the
  no-revoke `complete` residual is handled by a pre-`ApplyMigrations`, DB-untouched
  **pure-read** comparison (`ExpectedFingerprint() == LiveFingerprint(recorded)` AND
  `cursor.plan_hash == expected`) that serves only when already in-sync, else halts
  `awaiting_deploy` DB-untouched (`HOLDER.md:362-378`). Universal Invariant B is tightened
  so the legacy `connection.go:399` writer is reachable only in §3.5 cells 1 and
  13-in-sync — barred from any revoke-embedded (barrier a), revoke-applied (barrier b), or
  pending-change-over-a-transcript state (`HOLDER.md:622-642`). F17 + F11(g) + F18 carry
  the exact assertions the v5 ledger §4 prescribed. **falsifier_1 — the lens that found
  M3 — explicitly states it does not keep M3 open.** I verified the boot order
  (`connection.go:349 watermark → :353 ApplyMigrations → :376-383 drift → :384-393 shadow
  fall-through → :399 RecordSchemaFingerprint`) against current source: the post-`:353`
  ordering is exactly why a pre-`:353` predicate returning nil let the v5 legacy mutator +
  self-record run, and the M3 fix halts **before** `:353` on both boot paths. The fix is a
  concrete sub-protocol bound to named code sites, not a claim.
- **M4 (the F16 test-staging defect) — RESOLVED.** F16 is split into **F16a** (synthetic
  bundle list / injectable-FS hook, rollout step 2, builds green before 0021 exists) and
  **F16b** (production embed/listing split + the forced FMA-007 self-heal reaching
  `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`, rollout step 7 after 0021 is
  authored) (`HOLDER.md:296-316`). **falsifier_2 explicitly states it does not keep M4
  open** and finds no M2 regression in the restructuring.

It also carries **M1, M2, BC-N1, BC-N2, C1, C2, and C3 forward verbatim and intact** (the
full-transcript verifier; the non-revoke owner-ddl filter; the immutable `deploy_plan`
transcript; the universal non-`complete`-cursor edge; the `finalizing` finalizer; the
fail-closed activation edge with `RequiredOwnerBundleVersion = 20`; the revoke-last
ownership mechanism) — both falsifiers concede each carry-forward and show no regression.

**But a clearing verdict also requires (a) the proactive-completeness boot-path decision
table to be COMPLETE — Invariant B proven in every cell AND the legitimate fresh-DB
bring-up cells still serving, not wedged — and (b) no new material challenge standing
unrebutted. BOTH fail on the same defect.** **Both** falsifiers, from two independent
lenses, **converge** on a new material decision-table completeness failure (**M5**),
source-verified against current `main`, and it stands unrebutted:

- **M5 — §3.5/F18's uniform `applied_owner < 20 → awaiting_owner_ddl` rule collapses two
  distinct owner-watermark states and either wedges a legitimate fresh-DB boot or makes
  the executable F18 oracle false.** §3.5 states the `<20` column is uniformly
  `awaiting_owner_ddl` and "ALWAYS halts at W" (`HOLDER.md:443-459,461-478`), and the
  prose calls cell 1/`==20` the fresh-DB cell that still serves (`HOLDER.md:515-518`). But
  current source `CheckOwnerBundleWatermark` **returns nil (serves) for `applied == 0`** —
  the fresh / single-role / no-authority bootstrap case — before the shortfall check, and
  halts `awaiting_owner_ddl` **only** for `1 <= applied < 20`. So the §3.5/F18 table either
  (1) **wedges the legitimate fresh no-authority boot** (`cursorState=none`, no-revoke,
  flag-off, `applied_owner=0`) that the SEED explicitly requires to "still serve and NOT be
  wedged" (`SEED.md:318-331,367-370`) and that current source serves, or (2) makes the
  **executable F18 matrix a false oracle** for the `applied_owner == 0` cell. The holder's
  own prose compounds it by labeling cell 1/`==20` — an already-owner-bundled DB at version
  20 — the "fresh-DB bring-up" cell, when the genuine fresh no-authority DB is
  `applied_owner == 0`.

Because **M5 is a material new challenge standing unrebutted** and the decision table is
therefore not complete/correct in the owner-watermark dimension, the gate does **not**
clear. This is the gate's single allowed revision cycle, so it ends **unCleared**.

## 2. Per-finding / per-challenge ledger

| # | Source | What is judged | Material? | Resolved / rebutted? | Disposition |
| --- | --- | --- | --- | --- | --- |
| **M3** | cycle-1 finding / falsifier_1 revision-check | hoisted `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config gate (step 0, every cursor state) + the no-revoke `complete` pre-`ApplyMigrations` pure-read comparison + tightened Invariant B + F17/F11(g)/F18 (`HOLDER.md:349-378,622-642`) | — | **Yes — RESOLVED** (falsifier_1 explicitly does not keep it open; adjudicator verified the `connection.go` boot order) | **RESOLVED** |
| **M4** | cycle-1 finding / falsifier_2 revision-check | F16 split into F16a (synthetic, step 2) + F16b (production + forced self-heal via `isCrossBundleDependencyError`, step 7) (`HOLDER.md:296-316`) | — | **Yes — RESOLVED** (falsifier_2 explicitly does not keep it open) | **RESOLVED** |
| **M1** | carried forward / both falsifiers | `VerifyStoredTranscript` on resume AND finalizer step 0; typed mismatch halts; F15 + F14 (`HOLDER.md:419-429`) | — | **Yes — INTACT, unregressed** (M3 extends Invariant B without gating the verifier) | **INTACT** |
| **M2** | carried forward / both falsifiers | single non-revoke filter `OwnerDDLApplyBundles()` + in-loop guards across every route incl. FMA-007 self-heal; embed/listing split (`HOLDER.md:269-294`) | — | **Yes — INTACT** (M4 restructured only F16's staging) | **INTACT** |
| **BC-N1** | carried forward / both falsifiers | immutable `deploy_plan` transcript; resume off stored transcript; §1.3 + doctor + F14 (`HOLDER.md:156-226`) | — | **Yes — INTACT, unregressed** | **INTACT** |
| **BC-N2** | carried forward / both falsifiers | universal non-`complete`-cursor edge at `applied_owner == 20`; F11(e)/(f) (`HOLDER.md:356-360`) | — | **Yes — INTACT** (M3's complete-cursor extension sits above/beside it, does not weaken it) | **INTACT** |
| **C1** | carried forward / both falsifiers | `finalizing` + idempotent finalizer (VerifyStoredTranscript step 0) + §1.3 row + F10 (`HOLDER.md:183-189,409-417`) | — | **Yes — INTACT** (M3's harm flowed through the legacy `:399` writer, a different writer) | **INTACT** |
| **C2** | carried forward / falsifier_1 | `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `Required = 20` (`HOLDER.md:339-395,540-558`; `owner.go:35`) | — | **Yes — INTACT** (M3 extends the edge without advancing `Required` or altering the watermark) | **INTACT** |
| **C3** | carried forward / both falsifiers | 0021 special-cased + terminal + revoke-last; F12/`G-revoke-last` (`HOLDER.md:397-407,523-558`) | — | **Yes — INTACT** | **INTACT** |
| **M5** | falsifier_1 (`dialogue:2`) + falsifier_2 (`dialogue:3`) — **independent convergence** | §3.5/F18's uniform `applied_owner < 20 → awaiting_owner_ddl` collapses the fresh-DB `applied_owner == 0` serve cell with the `1..19` shortfall halt (`HOLDER.md:443-478,515-518`; `owner.go:116-150,234-235`) | **Yes** — wedges a legitimate fresh-DB boot OR makes the executable F18 oracle false; spec edit required | **No** — no holder rebuttal; the spec text affirms the wrong rule; the "`<20` = authority-bearing" reading is preempted | **STANDS → needs_revision** |

## 3. Why the verdict turns the way it does

### M3 — complete-cursor legacy self-record / mutation bypass: RESOLVED

The v5 break was that §3.3a `CheckDeployActivation` returned nil immediately on
`cursorState == complete` while the `revokeEmbedded && !decoupledEnabled →
awaiting_deploy_config` halt lived only in the `none` branch, so a revoke-embedding binary
with the flag OFF over a `complete` transcript rode the legacy `ConnectAndMigrate`
`ApplyMigrations` + `RecordSchemaFingerprint` path around `VerifyStoredTranscript`. The v6
revision closes this with exactly the v5-prescribed sub-protocol:

- **The config gate is hoisted to step 0**, before any cursor-state branch
  (`HOLDER.md:349-355`), so the `complete` cursor can no longer short-circuit it. A
  revoke-embedding binary with `STRIATUM_DEPLOY_DECOUPLED` OFF returns
  `awaiting_deploy_config`, DB untouched, before `ApplyMigrations` (`connection.go:353`)
  and before `RecordSchemaFingerprint` (`:399`), on **both** `ConnectAndMigrate` and
  `ConnectAndVerify` — the decision-table cells 2/6/10/14 (`HOLDER.md:461-478`).
- **The no-revoke `complete` residual** runs a pre-`ApplyMigrations`, DB-untouched
  **pure-read** comparison (`ExpectedFingerprint()` + `LiveFingerprint` + `plan_hash`, all
  reads per `schema_drift.go:83-100,145-160`) and serves only when already in-sync — so
  `ApplyMigrations` is a no-op and `:399` is an idempotent rewrite of the same fingerprint;
  any divergence halts `awaiting_deploy`, DB untouched (`HOLDER.md:362-378`).
- **Universal Invariant B is tightened** (`HOLDER.md:622-642`): the legacy `:399` writer is
  reachable only in §3.5 cells 1 (`none`, no transcript) and 13-in-sync, barred from any
  revoke-embedded state (barrier a), any revoke-applied state (barrier b — the C2
  forward-watermark at `applied_owner >= 21`), and any pending-change-over-a-transcript
  state (the A3 comparison).
- **F17** (`T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`)
  + **F11(g)** + **F18** assert `awaiting_deploy_config`, `ApplyMigrations` /
  `RecordSchemaFingerprint` un-called (spies), `schema_state` unchanged, DB byte-identical,
  and the shadow-mode fall-through never reached — the exact contract the v5 ledger §4
  required.

**falsifier_1 — the lens that raised M3 — explicitly concedes it is closed.** I verified
the boot order and the predicate hoisting against current source (`go/pkg/db/connection.go:332-404`):
the guard precedes `:353` on both paths, and the fix is a concrete sub-protocol bound to
named code sites, not merely claimed. **M3 is genuinely resolved.**

### M4 — F16 test-staging: RESOLVED

The v5 break was that F16 asserted production `OwnerBundles()` contains 0021 in rollout
step 2 while 0021 is not authored until step 7 — un-buildable. The v6 revision splits F16
exactly as the v5 ledger §4 prescribed: **F16a** drives the exclusion filter through a
synthetic bundle list / injectable-FS hook at step 2 (no production-loader assertion, so it
builds green before 0021 exists); **F16b** asserts the production embed/listing split at
step 7 after 0021 is authored, and houses the forced FMA-007 self-heal pgtest required to
reach `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError` (`owner.go:367-374`), not
merely the pending loop (`HOLDER.md:296-316`). **falsifier_2 explicitly concedes M4 is
resolved** and that the M2 filter is unchanged. **M4 is genuinely resolved.**

### M5 — the decision-table owner-watermark collapse: STANDS (the blocker)

This is the load-bearing reason the gate does not clear, and it is the strongest signal a
falsification gate can produce: **two falsifiers, from two independent lenses
(decoupling-boundary; owner-ddl/test-staging), converged on the same source-verified
defect and each made it their explicit blocker.** I verified their source claims directly
this cycle against the run's worktree (HEAD `df2a9e70`):

- **The table over-halts the `<20` bucket.** §3.5 maps `applied_owner < 20 →
  awaiting_owner_ddl`, states `<20` "ALWAYS halts at W", and renders every `<20` row
  uniformly `awaiting_owner_ddl` (`HOLDER.md:443-459,461-478`).
- **Current source serves `applied == 0`.** `CheckOwnerBundleWatermark` returns nil for
  `applied == 0` (the fresh / single-role / no-authority bootstrap case) at
  `go/pkg/db/owner.go:145-147`, **before** the `if applied < RequiredOwnerBundleVersion`
  shortfall check at `:148-150`. The function's own comment is explicit: a fresh 0-watermark
  DB "is treated as the bootstrap/single-role case and NOT halted. Only a database that HAS
  an authority schema (applied >= 1) but lags the required frontier is a genuine shortfall"
  (`:116-123`, reinforced at `:140-143`). `OwnerBundleVersion` returns 0 when
  `to_regclass('striatumd.owner_bundle_meta')` is null (`owner.go:234-235`). **Confirmed by
  direct read this cycle — both falsifiers' citations are accurate.**
- **The concrete cell is the one the SEED pinned.** `cursorState=none`,
  `decoupledEnabled=false`, `revokeEmbedded=false`, `applied_owner=0`: under current source
  the watermark returns nil and §3.3a step 4 returns nil, so legacy `ConnectAndMigrate`
  performs the normal fresh-DB bring-up (`HOLDER.md:379-385`). Under the v6 §3.5/F18 table
  it is row 1/`<20` and must return `awaiting_owner_ddl`.
- **The fork is a real defect either way.** An implementation following F18 literally
  either (1) halts a fresh no-authority DB — **regressing** fresh / single-role bootstrap
  and violating the SEED's explicit "the legitimate fresh-DB bring-up / inert-landing cells
  (no-revoke binary, no transcript) that must still serve and NOT be wedged"
  (`SEED.md:318-331`) and "lifting `ApplyMigrations` must not break fresh-DB bring-up"
  (`SEED.md:367-370`); or (2) keeps the `applied == 0` exception — making the **executable**
  F18 matrix a false oracle for the required `applied_owner < 20` bucket
  (`HOLDER.md:666`). Both are material.

The holder's anticipatable rebuttal — that `<20` was meant to mean "authority-bearing but
below 20," with `0/no authority` implicitly outside the table — is preempted by both
falsifiers and does not hold: the SEED and F18 explicitly enumerate `applied_owner < 20` as
a matrix bucket; current source represents the fresh no-authority case **as** owner-bundle
version 0, a member of that bucket; §3.5 says `<20` always halts (`HOLDER.md:455-456`); and
the prose mislabels cell 1/`==20` (an already-owner-bundled DB at version 20) as the
fresh-DB cell. The written, executable contract is wrong, and no interpretation rescues it.

This is **not** a re-opening of M3 — the hoisted M3 gate is correct for revoke-embedding
binaries, and the fresh-DB cell carries no transcript and no revoke, so Invariant B is not
*violated* there (the failure is the opposite: an over-conservative halt of a cell that
must serve). It is exactly the failure mode the SEED warned the decision table existed to
preempt: "A re-scaffolded revision that pins exactly the two §4 items but leaves an
unaudited boot-path combination open will simply spawn an M5." It matches the rubric's
explicit needs_revision trigger — "a decision-table cell that ... wedges a legitimate
boot." It is **material** (it requires a spec edit and exposes a real correctness defect in
an executable named requirement) and **stands unrebutted** (no holder rebuttal artifact
this cycle; the falsifiers are the last word and the spec text affirms the wrong rule).
**M5 forces `needs_revision`.**

### M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3 — INTACT; the decision table — INCOMPLETE

All seven carry-forward findings are preserved verbatim and unregressed, and both
falsifiers concede each. **M1** (`VerifyStoredTranscript` on resume and finalizer step 0,
typed mismatch halts, F15 + F14) — M3 extends Invariant B's enforcement to the legacy path
without gating the deployer verifier; falsifier_1 confirms no regression. **M2** (single
non-revoke filter + split loader + in-loop guards + embed/listing split) — M4 restructured
only F16's staging, not the filter; falsifier_2 confirms and finds no sibling owner-ddl
route. **BC-N1** (immutable `deploy_plan` transcript, resume off the stored transcript, §1.3
+ doctor + F14). **BC-N2** (universal non-`complete` edge at `applied_owner == 20`) — M3's
complete-cursor extension sits above and beside it and does not weaken it; note M5 concerns
the orthogonal owner-watermark `<20` dimension at W, not the BC-N2 `deploy_cursor` edge at
A. **C1** (`finalizing` + idempotent finalizer + §1.3 + F10) — M3's harm flowed through the
legacy `:399` writer, a different writer than the gated C1 finalizer. **C2**
(`CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at
`applied >= 21`, `RequiredOwnerBundleVersion = 20`) — M3 extends the edge without advancing
`Required` or altering the watermark. **C3** (0021 special-cased + terminal + revoke-last,
F12/`G-revoke-last`).

**Decision table — NOT COMPLETE.** Invariant B itself is proven in every specified cell for
the `==20` and `>=21` sub-columns, and the M3 cell (14) and the shadow-mode drift-gate
fall-through are covered. But the table is **incomplete/incorrect in the owner-watermark
dimension**: the `<20` column is hand-waved as uniformly `awaiting_owner_ddl` (HOLDER.md:455-456),
which is false for `applied_owner == 0`, so the legitimate fresh-DB serve cell is not
specified and is in fact wedged. A table that drops a serving cell the source serves is not
the COMPLETE table the SEED required (M5).

## 4. What the revision must fix (exact)

A re-scaffolded revision would clear the gate only if it pins the single item below, while
keeping M3, M4, and M1/M2/BC-N1/BC-N2/C1/C2/C3 intact and unregressed.

1. **Split the owner-watermark dimension so the decision table matches the live bootstrap
   contract (M5 — the load-bearing fix, decision-table completeness).**
   - In §3.5 and F18, replace the single `applied_owner < 20` bucket with
     `applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}`.
   - Specify the no-transcript / no-revoke / flag-off bootstrap cell
     (`cursorState=none`, `decoupledEnabled=false`, `revokeEmbedded=false`,
     `applied_owner=0`) as **serve-legacy / fresh bootstrap**: `ApplyMigrations` and the
     legacy `:399` self-record may run because **no deploy transcript exists** (Invariant B
     is not in scope), exactly matching `CheckOwnerBundleWatermark`'s `applied == 0`
     exception (`owner.go:145-147`). Retain `awaiting_owner_ddl` (DB untouched) for
     `1 <= applied_owner < 20`.
   - Propagate the split through the other cursor rows so the table stays executable, and
     make **F18** assert **both** branches explicitly (the `applied_owner == 0` serve cell
     and the `1..19` halt cell), so the matrix oracle matches source without changing the
     bootstrap contract.
   - Stop labeling cell 1/`==20` the "fresh-DB bring-up" cell (`HOLDER.md:515-518`); the
     genuine fresh no-authority DB is `applied_owner == 0`. (If the spec instead deliberately
     wants to drop the `applied == 0` bootstrap exception, that is a product behavior change
     that must be called out as such and reconciled with the SEED's "must still serve"
     requirement — but the conservative, source-preserving split above is the expected fix.)

> Note the asymmetry the fix must preserve: the M3 halt is *conservative on purpose* for a
> revoke-embedding binary (decoupling becomes mandatory once the binary embeds 0021), but
> the watermark `<20` halt must **not** be conservative for `applied_owner == 0` — that
> cell is a legitimate fresh serve, and over-halting it is the M5 defect.

## 5. What already cleared (carry forward — do NOT regress or re-litigate)

- **M3 complete-cursor activation guard — RESOLVED this cycle** (§3 above): preserve the
  hoisted step-0 `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config gate
  (every cursor state), the no-revoke `complete` pre-`ApplyMigrations` pure-read comparison,
  the tightened Universal Invariant B (legacy `:399` reachable only in cells 1 and
  13-in-sync), and F17 / F11(g) / F18's per-cell assertions. (The M5 fix only re-buckets the
  `applied_owner` dimension; it must not weaken the M3 config gate — the revoke-embedding +
  flag-OFF halt stays conservative.)
- **M4 F16 phase-aware split — RESOLVED this cycle**: preserve F16a (synthetic-list pre-0021,
  step 2) + F16b (production post-0021 + the forced self-heal reaching `ReapplyAllOwnerBundles`
  via `isCrossBundleDependencyError`, step 7).
- **M1 full-transcript verifier — intact**: preserve `VerifyStoredTranscript` over the whole
  transcript (byte + already-applied DB stamp), on every resume and as finalizer step 0, the
  typed `deploy_plan_binary_mismatch` / `deploy_plan_db_stamp_mismatch` halts, F15, and F14.
- **M2 non-revoke filter — intact**: preserve `DDLRevokeOwnerBundleVersion = 21`,
  `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, the in-loop guards on both apply loops, the
  nil-fallback split, and the embed/listing split.
- **BC-N1 immutable transcript / resume-off-stored-plan — intact**: preserve the 0044
  `deploy_plan` table, materialize-before-step-0, the §1.3 stored-transcript
  reclassification, the transcript-enumerated doctor, and F14.
- **BC-N2 universal non-complete-cursor edge — intact**: preserve the
  `revokeEmbedded`-independent halt at `applied_owner == 20` and F11(e)/(f). (M3's
  complete-cursor extension does not weaken it; M5 concerns the orthogonal owner-watermark
  dimension.)
- **C1 finalization boundary — intact**: preserve the `finalizing` state + idempotent
  finalizer + §1.3 row + F10, with M1's pre-finalizer `VerifyStoredTranscript` step 0.
- **C2 fail-closed activation edge — intact**: preserve `CheckDeployActivation` before
  `ApplyMigrations`, the typed halts, the forward-watermark rule (fires at `applied >= 21`),
  and `RequiredOwnerBundleVersion = 20` (its current-`main` value, **not** advanced to the
  revoke ordinal). M3 extends this edge to the complete-cursor case without altering it.
- **C3 revoke-last ownership mechanism — intact**: preserve 0021 special-cased + terminal +
  the reconcile-while-CREATE-held ordering + F12 / `G-revoke-last`.
- **Q4 — resolved** (v1–v5): plain verb now, bootstrapping paradox dispositive, run-shape
  door held by the three seams. Neither falsifier contested it.
- **Q3 mid-step resumability body — sound**: Q3-A/Q3-B, the §1.1 step taxonomy. M3/M5
  concern serve-boot guarding and the boot-path table, not the per-step schema/cursor body.
- **Decoupling shape — sound** (now M3-hardened): shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark interlock + P3
  drift gate, the embed-FS-derived plan, the hash-chained per-step receipt.
- **Scope discipline held**: P5 deferred; full capability revocation named as a follow-up
  beyond P4; the local-first boundary respected.

## 6. Gate decision

The ledger verdict — not falsifier completion — clears the phase gate. **M3 and M4 are
both genuinely resolved** (both falsifiers explicitly concede each), and **M1, M2, BC-N1,
BC-N2, C1, C2, and C3 are carried forward intact** (both falsifiers concede no regression).
But a clearing verdict also requires **the boot-path decision table complete (Invariant B
proven in every cell, the legitimate fresh-DB cells still serving) and no new material
challenge standing** — and **M5** is a material challenge standing unrebutted,
source-verified against current `main` and landed independently by **both** falsifiers:

- **M5**: §3.5/F18's uniform `applied_owner < 20 → awaiting_owner_ddl` rule collapses the
  fresh-DB `applied_owner == 0` serve cell (which `CheckOwnerBundleWatermark` returns nil
  for, `owner.go:145-147`) with the `1..19` authority-shortfall halt, so the executable F18
  table either wedges a legitimate fresh / single-role boot the SEED requires to serve, or
  becomes a false oracle. The holder's `<20`-means-authority-bearing reading does not hold
  against the SEED's named bucket, the source's version-0 representation, and the spec's own
  "ALWAYS halts" text.

**Verdict: `needs_revision`.** This is the gate's single allowed revision cycle, so the
gate ends **unCleared** — judged on the merits. A clearing verdict would require a
re-scaffolded revision that pins the §4 owner-watermark dimension split (specifying the
`applied_owner == 0` fresh-bootstrap serve cell and the `1..19` shortfall halt, with F18
asserting both) without regressing M3, M4, or M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3.

<sub>Adjudicator collaboration ledger for the RFC 0142 P4 falsification-gate design run,
v6 / REVISION, cycle 1. Inputs:
`docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/holder/HOLDER.md`,
`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, `SEED.md`;
context: v5 `HOLDER.md` + v5 `COLLABORATION_LEDGER_cycle_1.md` (its §4 + the M3/M4
findings). Adjudicator source verification against the run worktree (HEAD `df2a9e70`):
`go/pkg/db/owner.go:116-150` (the `applied == 0` fresh-DB serve exception confirmed —
returns nil before the `< RequiredOwnerBundleVersion` shortfall check),
`go/pkg/db/owner.go:226-236` (`OwnerBundleVersion` returns 0 when `owner_bundle_meta` is
absent, confirmed), `go/pkg/db/connection.go:332-404` (the `:349/:353/:376-383/:384-393/:399`
boot order confirmed); HOLDER / falsifier anchors as cited. Verdict vocabulary: accept |
accept_with_findings | needs_revision | reject.</sub>

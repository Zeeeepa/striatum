---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec (v5 REVISION; resolves cycle-4 M1 full-transcript verification + M2 owner-ddl self-heal exclusion, carries BC-N1/BC-N2/C1/C2/C3 forward; re-verified against current `main` HEAD 2e482077)"
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
    text: "Revised P4 spec (v5): resolves the two binding cycle-4 findings M1 + M2 by concrete sub-protocols bound to named code sites, does the proactive hardening audit, and carries BC-N1/BC-N2/C1/C2/C3 forward verbatim, re-verified against current `main` (HEAD 2e482077; `git diff --stat 5c5cb36a HEAD -- go/` empty, so every v4 source anchor holds). M1 by a single `VerifyStoredTranscript(plan_hash)` (§3.4a) that checks the ENTIRE stored transcript — every step, already-applied AND not-yet-applied — `sha256` against the running binary's embedded bytes (ANY mismatch ⇒ `deploy_plan_binary_mismatch`, typed halt, DB-untouched) and, for already-applied entries, verifies the DATABASE STAMPS (`schema_migrations.sha256` / `owner_bundle_meta.sha256`) match the stored transcript (mismatch ⇒ `deploy_plan_db_stamp_mismatch`, refuse-to-finalize); the SAME verifier runs on every resume AND as finalizer step 0 BEFORE the `complete` receipt / `RecordSchemaFingerprint` / `finalizing → complete` advance, so a binary that disagrees with an already-applied step can never self-record its `ExpectedFingerprint()` as in-sync; F15 (new) covers the A45/B45 case, the symmetric owner-step case, the DB-stamp case, and the finalizer arm; F14 extended with an already-applied tamper case. M2 by a single non-revoke filter `isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)` + a split loader `OwnerDDLApplyBundles()` bound across EVERY `owner-ddl apply` route — `ApplyOwnerBundles` loads the filtered slice (`owner.go:269`), `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles` (the FMA-007 self-heal) both add in-loop `isNonRevokeBundle` guards, the `ReapplyAllOwnerBundles(nil,…)` fallback loads the filtered slice; `OwnerBundles()` (full, includes 0021) is kept ONLY for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable`, so 'binary embeds 0021' ≠ 'owner-ddl apply iterates 0021'; F16 (new) forces the cross-bundle self-heal with a synthetic 0021 and asserts 0021 not applied / `owner_bundle_meta` never records 21 / CREATE held, plus `TestOwnerDDLApplyExcludesRevokeBundle` and a build-time grep test; F12/`G-revoke-last` extended with the owner-ddl side-path case. Proactive hardening §4.5 names every owner-bundle apply path and every fingerprint/self-record path against current `main` and states two universal invariants (A: the DDL-revoke bundle excluded from ALL apply paths; B: no fingerprint/`complete` written unless the full stored transcript byte-matches the binary + applied DB stamps match). BC-N1 (immutable `deploy_plan` transcript, materialize-before-step-0, resume off the stored transcript, §1.3 + doctor + F14), BC-N2 (universal `CheckDeployActivation` cursor edge, `applied_owner == 20` halt, F11(e)/(f)), C1 (`finalizing` + idempotent finalizer + §1.3 row + F10), C2 (`CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion = 20`), and C3 (0021 special-cased + terminal + revoke-last, F12/`G-revoke-last`) carried forward verbatim and unregressed."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "Revision check (conceded): the v5 holder genuinely resolves the specific v4 M1 reproducer for deploy-plan steps. `VerifyStoredTranscript(plan_hash)` is specified over EVERY stored step (already-applied AND pending) and compares those SHAs to the running binary before any resume apply; for already-applied entries it also verifies `schema_migrations.sha256` / `owner_bundle_meta.sha256` against the transcript; the finalizer runs the same verifier as step 0 before the `complete` receipt, `RecordSchemaFingerprint`, or `finalizing → complete` (`HOLDER.md:84,214-226,560-568,605-645`). F15 directly covers the v4 `A45`/`B45` case and the symmetric owner-step case (`HOLDER.md:836`). So the v4 already-applied deploy-step mismatch no longer reproduces inside the deployer resume/finalizer path. M2 is also pinned at the design level: a single `OwnerDDLApplyBundles()` filtered slice, in-loop `isNonRevokeBundle` guards on both `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles`, and an F16 test that forces the FMA-007 self-heal branch (`HOLDER.md:85,362-420,837`); no new owner-ddl path applies 0021 early. BC-N1's stored-plan identity, BC-N2's non-complete cursor halt, C1's finalization boundary, and C3's revoke-last mechanism are carried forward coherently. NEW MATERIAL CHALLENGE (M3 — C2/decoupling-boundary + M1-Invariant-B hardening core): v5 still leaves a self-record / mutation path AROUND `VerifyStoredTranscript` when the cursor is already `complete` and the process is on the legacy `ConnectAndMigrate` path. The §4.5 Invariant B scopes the legacy `connection.go:399` self-record to 'no transcript, cursor absent' (`HOLDER.md:789-806`), but the §3.3a predicate contradicts that: `CheckDeployActivation` returns nil immediately when `cursorState == complete`, deferring to the drift gate (`HOLDER.md:480-482`), and the `revokeEmbedded && !decoupledEnabled` config halt exists ONLY in the `cursorState == none` branch (`HOLDER.md:483-489`). So a deployer-aware/revoke-embedding binary with a `complete` deploy transcript and `STRIATUM_DEPLOY_DECOUPLED` OFF takes the legacy `ConnectAndMigrate` path over a DB that DOES have `deploy_cursor`/`deploy_plan`. Current source order makes that material: `ConnectAndMigrate` runs the watermark, then `ApplyMigrations` (`connection.go:349-353`), only THEN `CheckSchemaDrift` (`:376-383`), and finally `RecordSchemaFingerprint` (`:399`) — so `CheckDeployActivation` returning nil lets `ApplyMigrations` run before any drift check or self-record. Concrete refutation: after the first P4 deploy (`deploy_cursor.state = complete`, `owner_bundle_meta max >= 21`, CREATE revoked), boot a later revoke-embedding binary with the flag OFF (not exotic — every future binary that still embeds 0021 is deployer-aware and the steady-state cursor is `complete`). (1) `CheckDeployActivation` sees `complete`, returns nil before the config halt; (2) flag OFF ⇒ legacy `ConnectAndMigrate`, not `ConnectAndVerify`; (3) source order reaches legacy `ApplyMigrations` before drift/self-record; (4) if the binary has a pending runtime migration that creates an object, the runtime role applies DDL after 0021 revoked CREATE — the #512-class lockout shape P4 exists to eliminate; if the migration needs no CREATE, the serve path still mutated schema after P4, violating the one-shot-deployer boundary; (5) in shadow mode the post-apply drift gate logs and falls through to `RecordSchemaFingerprint` (`connection.go:384-399`), so a legacy self-record overwrites `schema_state` with no `VerifyStoredTranscript` check — exactly the sibling self-record path §8 asks falsifiers to verify (`HOLDER.md:907-914`). This is NOT the old BC-N2 pre-revoke non-complete window (v5 closes that); it is the normal post-deploy steady state, where `complete` short-circuits the guard that should keep a revoke-embedding binary off the legacy mutator path. F11 only asserts non-complete and no-cursor/idle cases (`HOLDER.md:832`); it omits `cursorState == complete`, revoke embedded, flag OFF, pending runtime step. The holder's best rebuttal (a `complete` + matching fingerprint is legitimately in-sync, and the happy choreography restarts with the flag ON) holds only for the immediate intended restart: a `complete` cursor is complete for the PREVIOUS plan, not proof the current binary has no pending change, and the post-apply drift gate is too late because `ApplyMigrations` already ran. Required repair: make `revokeEmbedded && !decoupledEnabled` a pre-apply halt for EVERY cursor state including `complete` (conservative: revoke-embedding binary + flag OFF → `awaiting_deploy_config`, DB untouched), or add a pre-`ApplyMigrations` plan/fingerprint comparison that cannot mutate or self-record; extend F11/F15 with the complete-cursor case (assert `awaiting_deploy_config`, `ApplyMigrations`/`RecordSchemaFingerprint` un-called, `schema_state` unchanged, DB byte-identical); tighten Invariant B so a DB with `deploy_cursor`/`deploy_plan` present can never reach the legacy `connection.go:399` writer. Material C2/M1-hardening gap; the gate should not clear until the complete-cursor legacy path is made DB-untouched or covered by a pre-apply proof and a named test."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "Revision check (conceded): the v5 holder genuinely resolves the v4 M2 safety break as a design contract. The v4 reproducer was real (`ApplyOwnerBundles` loaded the full `OwnerBundles()` slice and on an FMA-007 cross-bundle dependency failure called `ReapplyAllOwnerBundles` over it; the nil-fallback also loaded the full slice — `owner.go:269,277-289,332-349`). v5 closes that branch: `DDLRevokeOwnerBundleVersion = 21`, `isNonRevokeBundle`, and `OwnerDDLApplyBundles()` as the only slice `owner-ddl apply` iterates, while keeping the full `OwnerBundles()` loader for `revokeEmbedded`/`ExpectedFingerprint`/the deploy plan; both barriers are bound to the self-heal path (`ApplyOwnerBundles` loads the filtered slice; `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles` add in-loop `isNonRevokeBundle` guards; the `ReapplyAllOwnerBundles(nil,…)` fallback loads `OwnerDDLApplyBundles()`) (`HOLDER.md:360,383-424`). F16 forces the FMA-007 self-heal with a synthetic 0021 and asserts 0021 not applied / `owner_bundle_meta` never records 21 / CREATE held (`HOLDER.md:833-837`); F12/`G-revoke-last` extended with the owner-ddl side-path; the proactive audit names the apply routes, the CLI entry, ACL reasserts, the deploy terminal step, and the non-apply `OwnerBundles()` consumers (`HOLDER.md:769-786`); no other owner-ddl dry-run/list surface exists today (`daemon.go:90-144`). So M2 does NOT remain open as an early-revoke safety gap; C3's terminal deploy step, C2's `RequiredOwnerBundleVersion = 20`, the forward-watermark at 21, BC-N2's non-complete cursor edge, and the P2 watermark/fresh-DB interlock are carried forward coherently. NEW (NARROWER) CHALLENGE (M4 — test-contract vs rollout-order): F16's production-loader assertion contradicts the stated rollout order. §6 lands the `owner.go` filter surface + `TestOwnerDDLApplyExcludesRevokeBundle` in step 2 ('inert until 0021 is authored', `HOLDER.md:845-849`), but 0021 is not authored until step 7 (`HOLDER.md:870-872`); yet the step-2 unit test is specified to assert production `OwnerBundles()` DOES contain 0021 so `revokeEmbedded`/`ExpectedFingerprint` see it (`HOLDER.md:439-442`). Those cannot all be true in a green incremental rollout: before `0021_*.sql` is embedded, `OwnerBundles()` will not contain 21, so assertion (b) fails `make test`; delaying it contradicts the step-2 'F16 lands with the filter surface' claim; a synthetic 0021 does not prove the production full-loader/`revokeEmbedded` condition as worded. This is a test-staging defect, not a safety flaw (the in-loop guards + `OwnerDDLApplyBundles()` can be proven against a synthetic slice pre-0021). Required repair: split F16 into phase-aware checks — pre-0021/inert phase uses a synthetic bundle list/test hook to prove `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`, both loops skip a hand-passed synthetic 0021, and the nil-fallback uses the filtered loader (do NOT assert production `OwnerBundles()` contains 0021 yet); activation phase (after 0021 authored) asserts production `OwnerBundles()` contains 0021, `ExpectedFingerprint()` includes its bytes, `revokeEmbedded` derives from the full loader/file presence, and production `OwnerDDLApplyBundles()` excludes it; keep the forced-self-heal pgtest in the activation phase (or make its synthetic fixture explicit) and require it to prove it actually reaches `ReapplyAllOwnerBundles` through `isCrossBundleDependencyError`, not just the pending loop. M2's safety invariant is substantively resolved; the adjudicator should require the phase-aware split so the M2 filters land green before 0021 exists and the embed/listing split is proven once 0021 is authored."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "REVISION cycle v5 (the gate's single allowed revision cycle). The two binding cycle-4 findings are each substantively resolved as concrete sub-protocols bound to named code sites — and BOTH falsifiers independently concede each one. M1 (full stored-transcript byte verification + already-applied DB-stamp verification, on every resume AND as finalizer step 0, ANY mismatch → `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` DB-untouched, F15 + extended F14) RESOLVES the v4 hybrid-self-record reproducer INSIDE the deployer resume/finalizer path — falsifier_1 explicitly states the v4 already-applied deploy-step mismatch no longer reproduces there. M2 (a single non-revoke filter `OwnerDDLApplyBundles()` + `isNonRevokeBundle` in-loop guards + the nil-fallback split, bound across `ApplyOwnerBundles`/`applyPendingOwnerBundles`/`ReapplyAllOwnerBundles`/tests; the embed/listing split so `revokeEmbedded`/`ExpectedFingerprint` see 0021 but `owner-ddl apply` never iterates it; F16 + extended F12/`G-revoke-last`) RESOLVES the v4 FMA-007 self-heal early-revoke break — both falsifiers explicitly state 0021 is no longer reachable through any `owner-ddl apply` route and CREATE stays held. BC-N1 (immutable `deploy_plan` transcript materialized before step 0; resume off the stored transcript; §1.3 + doctor + F14), BC-N2 (the universal `CheckDeployActivation` non-complete-cursor edge at `applied_owner == 20`; F11(e)/(f)), C1 (the `finalizing` state + idempotent finalizer + §1.3 row + F10; M1's fix adds a pre-finalizer gate without changing boundary semantics), C2 (`CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion` correctly KEPT at its current-`main` value 20), and C3 (0021 special-cased + terminal + revoke-last) are carried forward INTACT and unregressed; both falsifiers concede each carry-forward. The proactive-hardening AUDIT is present (§4.5 names every owner-bundle apply path and every fingerprint/self-record path against current `main` and states two universal invariants). BUT a clearing verdict ALSO requires no new material challenge standing unrebutted, and one does: (M3, falsifier_1, C2/decoupling-boundary + M1-Invariant-B core) the spec's own §4.5 Universal Invariant B — 'NO fingerprint/`schema_state` `complete` is written unless the FULL stored transcript byte-matches the binary' — is FALSIFIED by a reachable post-deploy steady state. §3.3a `CheckDeployActivation` returns nil immediately on `cursorState == complete` (defers to the drift gate, `HOLDER.md:480-482`) and the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt lives ONLY in the `cursorState == none` branch (`HOLDER.md:483-489`). So a deployer-aware/revoke-embedding binary with a `complete` cursor and `STRIATUM_DEPLOY_DECOUPLED` OFF takes the legacy `ConnectAndMigrate` path over a DB that DOES carry `deploy_cursor`/`deploy_plan`; current source order runs `ApplyMigrations` (`connection.go:353`) BEFORE `CheckSchemaDrift` (`:376-383`) and `RecordSchemaFingerprint` (`:399`), so the predicate returning nil lets the legacy mutator and the legacy self-record fire WITHOUT `VerifyStoredTranscript`. Harm: a pending runtime step that needs CREATE hits a #512-class lockout AFTER 0021 revoked CREATE (the exact failure P4's root reframe says becomes structurally impossible); a step needing no CREATE still mutates schema on serve-boot (the one-shot-deployer boundary regresses); and in shadow mode the post-apply drift gate logs and falls through to `RecordSchemaFingerprint`, overwriting `schema_state` around the M1 gate. This is NOT a BC-N2 regression (BC-N2's non-complete window stays closed) — it is the orthogonal COMPLETE-cursor window. It is exactly the 'unaudited sibling self-record path' the SEED's proactive-hardening section warned would spawn an M3, and the holder's own §8 raises it as an open tension whose stated 'intended close' covers only NON-`complete` cursors (`HOLDER.md:907-914`) — so the spec, as written, does not close it. MATERIAL (a real serve-boot-decoupling regression + a self-record-around-the-gate path, both in the correctness/safety core; matches the rubric's explicit needs_revision triggers 'a serve-boot decoupling that regresses the boundary' and 'a finalizer/self-record that can write around the full-transcript check') and STANDS UNREBUTTED (this cycle carries no holder rebuttal artifact; the spec text stands). Additionally (M4, falsifier_2) F16's `TestOwnerDDLApplyExcludesRevokeBundle` is specified to assert production `OwnerBundles()` contains 0021 in rollout step 2, before 0021 is authored in step 7 — a real test-contract-vs-rollout-order inconsistency that cannot build green as written; this is a test-staging defect, not a safety gap (M2's invariant is resolved), and on its own would be a clear-with-findings note, but it folds into the required revision. Because M3 is a material new challenge standing unrebutted, the gate does NOT clear. This is the single allowed revision cycle, so the gate ends unCleared; §4 records exactly what a re-scaffolded revision must fix (the M3 complete-cursor halt as the load-bearing item; the M4 F16 phase-aware split as the secondary item), and §5 records what cleared and must not be regressed."
findings:
  - id: M1
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED in the deployer resume/finalizer core; the supporting Universal Invariant B is BREACHED by a sibling path (surfaced as M3). The full-transcript verifier `VerifyStoredTranscript(plan_hash)` (§3.4a) checks EVERY stored step (already-applied AND pending) `sha256` against the running binary's embedded bytes (reusing `MigrationSHASet()` `migrations.go:197` for runtime and `OwnerBundle.SHA256()` `owner.go:190-194` for owner — the exact byte-hashes `ExpectedFingerprint()` composes, `schema_drift.go:83-100`); ANY mismatch ⇒ `deploy_plan_binary_mismatch`, DB-untouched, apply/finalize nothing. For already-applied entries it also verifies the DB stamps (`schema_migrations.sha256` `migrations.go:275` for runtime, `owner_bundle_meta.sha256` `owner.go:529` for owner) against the transcript ⇒ `deploy_plan_db_stamp_mismatch`, refuse-to-finalize. The SAME verifier runs on every resume (§1.2 HOLDER.md:214-226) AND as finalizer step 0 BEFORE the `complete` receipt / `RecordSchemaFingerprint` / `finalizing → complete` advance (§3.4 HOLDER.md:560-580), so a binary that disagrees with an already-applied step can never self-record its `ExpectedFingerprint()` as in-sync through the DEPLOYER finalizer. §1.3 re-keyed to the full transcript + a new `deploy_plan_db_stamp_mismatch` row; F15 (new) covers the A45/B45 case, the symmetric owner-step case, the DB-stamp case, and the finalizer arm; F14 extended with an already-applied tamper case; the doctor adds the stamp/byte-divergence WARN. falsifier_1 EXPLICITLY concedes the v4 already-applied deploy-step mismatch no longer reproduces inside the deployer resume/finalizer path. So the BC-N1/Q3/R4 binary-verification clause is now implemented across the whole transcript as the binding prescription required. HOWEVER the M1 resolution explicitly RELIES on Universal Invariant B ('no fingerprint/`complete` written unless the full stored transcript byte-matches the binary'); falsifier_1 shows that invariant is FALSE for a reachable state — the LEGACY `connection.go:399` self-record fires over a DB that DOES carry a `deploy_cursor`/`deploy_plan` transcript when the cursor is `complete`, the flag is OFF, and the binary embeds 0021 — so the deployer-path fix is genuine but its hardening boundary is breached (M3). **M1 deployer-path RESOLVED; Invariant B incomplete → M3.**"
    affected_invariants: ["full stored-transcript byte verification on resume AND before the finalizer", "wrong-resume-binary forced to deploy_plan_binary_mismatch", "Universal Invariant B (no fingerprint/complete without full-transcript byte+stamp match)", "P3/P4 fingerprint coherence (LiveFingerprint reflects actually-applied schema)"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: M2
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED as a safety contract; only a test-staging inconsistency remains (surfaced as M4). The single non-revoke filter `isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)` + the split loader `OwnerDDLApplyBundles()` are bound across EVERY `owner-ddl apply` route: `ApplyOwnerBundles` loads the filtered slice (`owner.go:269`); both `applyPendingOwnerBundles` and the FMA-007 self-heal `ReapplyAllOwnerBundles` add in-loop `isNonRevokeBundle` guards; the `ReapplyAllOwnerBundles(nil,…)` fallback loads the filtered slice (§3.2a HOLDER.md:402-424). `OwnerBundles()` (full, includes 0021) is kept ONLY for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable`, so 'binary embeds 0021' ≠ 'owner-ddl apply iterates 0021' — two independent barriers (the filtered slice AND the in-loop guard) on the FMA-007 path. F16 (new) embeds a synthetic 0021, FORCES the cross-bundle self-heal (a real `42P01`-class failure so `isCrossBundleDependencyError` `owner.go:367-374` fires), and asserts 0021 not applied / `owner_bundle_meta` never records 21 / `has_schema_privilege('striatumd_rw','striatumd','CREATE')` stays true; plus `TestOwnerDDLApplyExcludesRevokeBundle` and a build-time grep test; F12/`G-revoke-last` extended with the owner-ddl side-path. BOTH falsifiers EXPLICITLY concede 0021 is no longer reachable through any `owner-ddl apply` route incl. the self-heal, and CREATE stays held — the v4 early-revoke #512-class lockout escape is provably closed. The residual is NOT a safety gap: falsifier_2's M4 is that the F16 `TestOwnerDDLApplyExcludesRevokeBundle` is specified to assert production `OwnerBundles()` contains 0021 in rollout step 2 (HOLDER.md:439-442,845-849) while 0021 is not authored until step 7 (HOLDER.md:870-872) — a test-contract-vs-rollout-order inconsistency that cannot build green as written. **M2 safety RESOLVED; F16 test contract under-specified → M4.**"
    affected_invariants: ["0021 unreachable through every owner-ddl apply route (incl. FMA-007 self-heal)", "embed/listing split (revokeEmbedded/ExpectedFingerprint see 0021; owner-ddl apply never iterates it)", "R3 no early REVOKE CREATE before a CREATE-needing reconcile", "C3 revoke-last ordering holds outside the happy deploy plan"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: BC-N1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The runtime-owned `deploy_plan` table (migration 0044) storing the IMMUTABLE ordered transcript keyed by `plan_hash`, materialized in the same tx that sets `deploy_cursor → in_progress(0)` BEFORE step 0 (§1.2 HOLDER.md:180-213); resume loads `deploy_plan[cursor.plan_hash]` and never recomputes `BuildPlan` over the moved frontier (§1.3, §3.2 HOLDER.md:356-358); the §1.3 stored-transcript reclassification; the transcript-enumerated doctor; F14. M1 EXTENDS the binary-verification clause of this fix to already-applied steps + the pre-finalizer check; it does NOT re-open the moving-frontier mechanism. falsifier_1 confirms BC-N1's stored-plan identity is carried forward coherently. **BC-N1 INTACT.**"
    affected_invariants: ["BC-N1 immutable plan identity across resume", "stable (plan_hash, step_index) receipt key", "section 1.3 incomplete-vs-drift disambiguation", "resume off the stored transcript, never a recomputed BuildPlan"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: BC-N2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede and neither uses it as a blocker. `CheckDeployActivation` is not gated on `revokeEmbedded`; every deployer-aware binary reads `deploy_cursor` after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations`/`RecordSchemaFingerprint` in both boot paths and halts `awaiting_deploy` DB-untouched on a non-`complete` cursor at `applied_owner == 20` (§3.3a step 1 HOLDER.md:473-478; net invariant HOLDER.md:497-499); F11(e)/(f) + extended `G-old-binary-refuse`. NOTE: M3 (falsifier_1) is the ORTHOGONAL `complete`-cursor window, NOT the BC-N2 non-complete window — falsifier_1 explicitly says 'this is not the old BC-N2 pre-revoke non-complete window; v5 closes that.' So BC-N2's resolved edge is not regressed; the new gap is a different cursor state the BC-N2 edge was never responsible for. **BC-N2 INTACT.**"
    affected_invariants: ["BC-N2 universal pre-revoke deploy_cursor-authoritative edge", "incomplete (non-complete) cursor never serves", "R2 decoupling does not regress fresh-DB bring-up / clean boot"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C1
    severity: critical
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The `finalizing` state, the idempotent finalizer (guarded `complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST), the §1.3 resumable-finalization row, the operator pin (no single finalization tx spans owner+runtime), and F10 are preserved (§1.2, §1.3, §3.4 HOLDER.md:556-591). M1's fix ADDS a pre-finalizer `VerifyStoredTranscript` step 0 before the `schema_state` write; it does NOT change the finalizer's boundary semantics — the `complete`-with-missing-receipt window stays closed and the finalizer crash-point table (HOLDER.md:584-591) carries forward with the M1 row added. HONEST NOTE (recorded under M3, not as a C1 regression): the M3 harm manifests through the LEGACY `connection.go:399` self-record, a DIFFERENT writer than the C1 deployer finalizer — the deployer finalizer IS gated by `VerifyStoredTranscript`; M3 is about the legacy serve-boot self-record reachable on a `complete` cursor, not the deployer finalizer. **C1 INTACT.**"
    affected_invariants: ["finalization boundary (no complete with missing receipt/fingerprint)", "idempotent finalizer repaired exactly once by the right binary", "section 1.3 disambiguation enforcement"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: C2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT as carried forward (the v4-cleared mechanism is unregressed), but the NEW M3 challenge exposes a C2-ADJACENT hole the prior cycles never surfaced. Carried forward verbatim: `CheckDeployActivation` before `ApplyMigrations` in both boot paths (mutating nothing), the typed `awaiting_deploy`/`awaiting_deploy_config` halts, the forward-watermark rule re-anchored to fire at `applied >= 21`, and `RequiredOwnerBundleVersion` KEPT at its current-`main` value 20 (NOT advanced to the revoke ordinal 21) (§3.3a, §4.2, §4.3 HOLDER.md:692-715; `owner.go:23,35`). NONE of that is regressed. What M3 (falsifier_1) shows is that the §3.3a predicate's `cursorState == complete` branch returns nil and the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt lives ONLY in the `cursorState == none` branch — so a revoke-embedding binary + flag OFF + `complete` cursor is NOT fail-closed and rides the legacy `ApplyMigrations` path. This is a NEW material gap in the activation interlock (tracked as M3), not a regression of the v4-resolved C2 edge. **C2 carried-forward edge INTACT; the complete-cursor activation hole is the substance of M3.**"
    affected_invariants: ["fail-closed CheckDeployActivation edge before ApplyMigrations", "typed awaiting_deploy/awaiting_deploy_config halts", "forward-watermark rule at applied >= 21", "RequiredOwnerBundleVersion not advanced to the revoke ordinal"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: C3
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede the design-level answer. Bundle 0021 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) identified by `DDLRevokeOwnerBundleVersion = 21`, special-cased out of the owner prefix, applied as the TERMINAL plan step after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE granted by 0018 in the non-revoke prefix; the committed steady state denies CREATE); `LatestOwnerBundleVersion` stays 20; F12/`G-revoke-last` (§3.2, §3.3b, §4.4 HOLDER.md:501-536,750-763). M2 binds the EXECUTABLE exclusion across the self-heal branch of `owner-ddl apply` (now provably closed — see M2); it does NOT change the deploy-plan revoke-last mechanism. The deploy can still complete (no stranded `ALTER … OWNER TO striatumd_rw`). **C3 INTACT.**"
    affected_invariants: ["two-role owner/runtime boundary", "revoke-last terminal ordering", "reconcile-while-CREATE-held", "activation deploy can complete"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M3
    severity: high
    posture: design
    status: open
    challenge: "OPEN — NEW material challenge (falsifier_1; C2/decoupling-boundary + M1-Invariant-B core). A `complete` deploy cursor short-circuits the activation guard, letting a revoke-embedding binary with `STRIATUM_DEPLOY_DECOUPLED` OFF ride the legacy `ConnectAndMigrate` mutator + self-record path AROUND `VerifyStoredTranscript`. §3.3a `CheckDeployActivation` returns nil immediately on `cursorState == complete` (defers to the drift gate, HOLDER.md:480-482); the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt exists ONLY in the `cursorState == none` branch (HOLDER.md:483-489). VERIFIED against the holder's own anchors + current source order: in `ConnectAndMigrate`, watermark `connection.go:349` → `ApplyMigrations` `:353` → `CheckSchemaDrift` `:376-383` → `RecordSchemaFingerprint` `:399`; `CheckDeployActivation` returning nil therefore lets `ApplyMigrations` run before any drift check or self-record. Reproducer (normal post-deploy steady state): first P4 deploy complete (`deploy_cursor.state = complete`, `owner_bundle_meta max >= 21`, CREATE revoked); boot a LATER revoke-embedding binary (every future binary that embeds 0021 is deployer-aware) with the flag OFF; the predicate returns nil; the legacy path runs `ApplyMigrations`. If the later binary has a pending runtime migration that creates an object, the runtime role applies create-DDL after 0021 revoked CREATE → #512-class lockout (the exact failure P4's root reframe declares structurally impossible); if it needs no CREATE, serve-boot still mutated schema (one-shot-deployer boundary regressed); and in shadow mode the post-apply drift gate logs and falls through to `RecordSchemaFingerprint` (`connection.go:384-399`), overwriting `schema_state` with no `VerifyStoredTranscript` check — directly FALSIFYING Universal Invariant B (`HOLDER.md:800-806`), which scopes the legacy self-record to 'no transcript, cursor absent.' The holder's §8 raises exactly this tension (HOLDER.md:907-914) but its stated 'intended close' (the universal edge halts NON-`complete` cursors) does not cover the `complete` case, so the spec as written does not close it; F11 omits the `complete`+revoke-embedded+flag-OFF+pending-step case (HOLDER.md:832). The holder's strongest rebuttal — `complete` + matching fingerprint is legitimately in-sync and the happy choreography restarts flag ON — holds only for the immediate restart of the SAME binary: a `complete` cursor is complete for the PREVIOUS plan, not proof the current binary has no pending change, and the post-apply drift gate is too late because `ApplyMigrations` already ran. This is the 'unaudited sibling self-record path' the SEED's proactive-hardening section warned would spawn an M3. MATERIAL (a real serve-boot-decoupling regression + a self-record-around-the-full-transcript-gate path, both in the correctness/safety core; matches the rubric's explicit needs_revision triggers). STANDS unrebutted → needs_revision. Required fix: make `revokeEmbedded && !decoupledEnabled` a pre-apply, DB-untouched halt for EVERY cursor state including `complete` (conservative: → `awaiting_deploy_config`), OR add a pre-`ApplyMigrations` plan/fingerprint comparison that cannot mutate or self-record; tighten Invariant B so a DB carrying `deploy_cursor`/`deploy_plan` can never reach the legacy `connection.go:399` writer; extend F11/F15 with the complete-cursor case asserting `awaiting_deploy_config`, `ApplyMigrations`/`RecordSchemaFingerprint` un-called, `schema_state` unchanged, DB byte-identical."
    affected_invariants: ["serve-boot decoupling boundary (no schema mutation as a side effect of restart)", "Universal Invariant B (no self-record around VerifyStoredTranscript when a transcript is present)", "R3 no #512-class CREATE lockout in any boot path after the revoke", "fail-closed activation interlock for a revoke-embedding binary with the flag OFF"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: M4
    severity: medium
    posture: design
    status: open
    challenge: "OPEN — NEW non-safety finding (falsifier_2; M2 test-contract vs rollout-order). F16's `TestOwnerDDLApplyExcludesRevokeBundle` is specified to assert that production `OwnerBundles()` DOES contain 0021 (so `revokeEmbedded`/`ExpectedFingerprint` see it, HOLDER.md:439-442) and is assigned to rollout step 2 ('lands first, inert until 0021 is authored', HOLDER.md:845-849); but 0021 is not authored until rollout step 7 (HOLDER.md:870-872). Before `0021_*.sql` is embedded, `OwnerBundles()` will not contain 21, so the assertion fails `make test`; delaying it contradicts the step-2 'F16 lands with the filter surface' claim; a synthetic 0021 does not prove the production full-loader/`revokeEmbedded` condition as worded. This is a test-STAGING inconsistency, NOT a safety/correctness defect — falsifier_2 explicitly states M2's safety invariant is substantively resolved and found no path that commits the revoke early. On its own this would be `accept_with_findings`. It folds into the required revision as a secondary fix. Required fix (falsifier_2's split): pre-0021/inert phase uses a synthetic bundle list/test hook to prove `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`, both apply loops skip a hand-passed synthetic 0021, and the nil-fallback uses the filtered loader (do NOT assert production `OwnerBundles()` contains 0021 yet); activation phase (after 0021 authored) asserts production `OwnerBundles()` contains 0021, `ExpectedFingerprint()` includes its bytes, `revokeEmbedded` derives from the full loader/file presence, and production `OwnerDDLApplyBundles()` excludes it; keep the forced-self-heal pgtest in the activation phase and require it to prove it reaches `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`, not just the pending loop. MATERIAL only in that it requires a spec edit (the named test cannot build green as written); non-blocking on its own."
    affected_invariants: ["F16 lands green incrementally before 0021 is authored", "the production embed/listing split is proven once 0021 exists", "every load-bearing M2 claim carries a buildable named test"]
    source_refs: ["dialogue:1", "dialogue:3"]
branches:
  design: blocked
---

# COLLABORATION LEDGER — RFC 0142 P4 design run (v5 / REVISION), cycle 1

author: adjudicator-author-001

**Verdict:** `needs_revision`

> Scope of this adjudication: the curated dialogue trajectory only — the **revised
> (v5)** Holder's `HOLDER.md` P4 implementation spec and the two falsifiers'
> `FALSIFIER.md` challenges — plus the `SEED.md` charter, with the **v4** `HOLDER.md`
> and the **v4** collaboration ledger (its §4 "What the revision must fix" + the M1/M2
> findings) as context for what the revision had to fix. RFC 0142 is accepted (D258);
> this judges the **P4 implementation shape**, not the five-layer design. A clearing
> verdict would be `accept` or `accept_with_findings`; this is neither. This is the
> gate's **single allowed revision cycle**, so the gate ends **unCleared** — judged
> exactly on the merits, not graded on the consequence.

## 1. Decision

`needs_revision`. The revision is real, well-anchored, and it **genuinely resolves both
binding cycle-4 findings** — both falsifiers independently concede each one:

- **M1 (full-transcript binary/DB-stamp verification) — RESOLVED in the deployer
  resume/finalizer core.** `VerifyStoredTranscript(plan_hash)` (§3.4a) checks **every**
  stored step (already-applied **and** pending) `sha256` against the running binary's
  embedded bytes — ANY mismatch ⇒ `deploy_plan_binary_mismatch`, DB-untouched — and for
  already-applied entries verifies the DB stamps (`schema_migrations.sha256` /
  `owner_bundle_meta.sha256`) against the transcript ⇒ `deploy_plan_db_stamp_mismatch`,
  refuse-to-finalize. The **same** verifier runs on every resume **and** as finalizer
  step 0, before the `complete` receipt / `RecordSchemaFingerprint` / `finalizing →
  complete` advance. F15 (new) covers the A45/B45 case, the symmetric owner-step case,
  the DB-stamp case, and the finalizer arm; F14 is extended with an already-applied
  tamper. **falsifier_1 explicitly states the v4 already-applied deploy-step mismatch no
  longer reproduces inside the deployer resume/finalizer path.**
- **M2 (single non-revoke filter across every `owner-ddl apply` route) — RESOLVED as a
  safety contract.** `isNonRevokeBundle` + `OwnerDDLApplyBundles()` + the in-loop guards
  on both `applyPendingOwnerBundles` and the FMA-007 self-heal `ReapplyAllOwnerBundles`
  + the nil-fallback split bind 0021 out of every apply route, while `OwnerBundles()`
  (full) is kept for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`. F16 (new)
  forces the cross-bundle self-heal and asserts 0021 not applied / `owner_bundle_meta`
  never records 21 / CREATE held; F12/`G-revoke-last` extended. **Both falsifiers
  explicitly concede 0021 is no longer reachable through any `owner-ddl apply` route and
  CREATE stays held.**

It also carries **BC-N1, BC-N2, C1, C2, and C3 forward intact** (the immutable
`deploy_plan` transcript; the universal non-complete-cursor edge; the `finalizing`
finalizer; the fail-closed activation edge with `RequiredOwnerBundleVersion = 20`; the
revoke-last ownership mechanism) — both falsifiers concede each carry-forward and show
no regression. And the **proactive-hardening audit is present**: §4.5 names every
owner-bundle apply path and every fingerprint/self-record path against current `main`
and states two universal invariants.

**But a clearing verdict also requires no new material challenge standing unrebutted,
and one does.** falsifier_1 lands a material new challenge (**M3**) squarely in the
C2/decoupling-boundary + M1-Invariant-B core, source-verified against the spec's own
predicate and current source order, and it stands unrebutted:

- **M3 — a `complete` cursor short-circuits the activation guard, letting a
  revoke-embedding binary with `STRIATUM_DEPLOY_DECOUPLED` OFF ride the legacy
  `ConnectAndMigrate` mutator + self-record path around `VerifyStoredTranscript`.**
  §3.3a `CheckDeployActivation` returns nil immediately on `cursorState == complete`
  (`HOLDER.md:480-482`), and the `revokeEmbedded && !decoupledEnabled →
  awaiting_deploy_config` halt lives **only** in the `cursorState == none` branch
  (`HOLDER.md:483-489`). So a deployer-aware binary with a `complete` deploy transcript
  and the flag OFF takes the legacy boot path over a database that **does** carry
  `deploy_cursor`/`deploy_plan`. Current source order runs `ApplyMigrations`
  (`connection.go:353`) **before** `CheckSchemaDrift` (`:376-383`) and
  `RecordSchemaFingerprint` (`:399`), so the predicate returning nil lets the legacy
  mutator and the legacy self-record fire with no `VerifyStoredTranscript` check. If the
  later binary has a pending runtime step that needs CREATE, that is a **#512-class
  lockout after 0021 revoked CREATE** — the precise failure P4's root reframe declares
  structurally impossible; if it needs no CREATE, **serve-boot still mutated schema**
  (the one-shot-deployer boundary regresses); and in shadow mode the post-apply drift
  gate logs and falls through to `RecordSchemaFingerprint`, **overwriting `schema_state`
  around the M1 gate** — directly **falsifying Universal Invariant B**, which scopes the
  legacy self-record to "no transcript, cursor absent."

falsifier_2 also lands a narrower finding (**M4**): F16's
`TestOwnerDDLApplyExcludesRevokeBundle` is specified to assert production
`OwnerBundles()` contains 0021 in rollout step 2, before 0021 is authored in step 7 — a
real test-contract-vs-rollout-order inconsistency that cannot build green as written.
This is a **test-staging defect, not a safety gap** (M2's invariant is resolved); on its
own it would be `accept_with_findings`. It folds into the required revision.

Because **M3 is a material new challenge standing unrebutted**, the gate does **not**
clear. This is the gate's single allowed revision cycle, so it ends **unCleared**.

## 2. Per-finding / per-challenge ledger

| # | Source | What is judged | Material? | Resolved / rebutted? | Disposition |
| --- | --- | --- | --- | --- | --- |
| **M1** | cycle-4 finding / falsifier_1 revision-check | full stored-transcript byte + already-applied DB-stamp verification on resume AND as finalizer step 0; `deploy_plan_binary_mismatch` / `deploy_plan_db_stamp_mismatch`; F15 + extended F14 (`HOLDER.md:84,214-226,560-645,836`) | — | **Yes — deployer-path RESOLVED** (falsifier_1 concedes the v4 reproducer no longer reproduces); Invariant B breached by a sibling path | **RESOLVED** (core); hardening gap → **M3** |
| **M2** | cycle-4 finding / both falsifiers | single non-revoke filter + split loader + in-loop guards across every `owner-ddl apply` route incl. FMA-007 self-heal; F16 + extended F12/`G-revoke-last` (`HOLDER.md:85,360-424,833-837`) | — | **Yes — safety RESOLVED** (both falsifiers concede 0021 unreachable, CREATE held) | **RESOLVED** (safety); test-staging → **M4** |
| **BC-N1** | carried forward / both falsifiers | immutable `deploy_plan` transcript; resume off stored transcript; §1.3 + doctor + F14 (`HOLDER.md:180-213,356-358`) | — | **Yes — INTACT, unregressed** (both concede) | **INTACT** |
| **BC-N2** | carried forward / both falsifiers | universal non-complete-cursor edge at `applied_owner == 20`; F11(e)/(f) (`HOLDER.md:473-499`) | — | **Yes — INTACT** (both concede; M3 is the orthogonal complete window) | **INTACT** |
| **C1** | carried forward / both falsifiers | `finalizing` + idempotent finalizer + §1.3 row + F10; M1 adds pre-finalizer gate, no boundary change (`HOLDER.md:556-591`) | — | **Yes — INTACT, unregressed** | **INTACT** |
| **C2** | carried forward / falsifier_1 | `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `Required = 20` (`HOLDER.md:692-715`; `owner.go:23,35`) | — | **Carried-forward edge INTACT**; new complete-cursor hole | **INTACT** (edge); new gap → **M3** |
| **C3** | carried forward / both falsifiers | 0021 special-cased + terminal + revoke-last; F12/`G-revoke-last` (`HOLDER.md:501-536,750-763`) | — | **Yes — INTACT** (both concede design) | **INTACT** |
| **M3** | falsifier_1 (`dialogue:2`) | `complete` cursor returns nil from `CheckDeployActivation`; revoke-embedding binary + flag OFF rides legacy `ApplyMigrations` + `RecordSchemaFingerprint` around `VerifyStoredTranscript` (`HOLDER.md:480-489,907-914`; `connection.go:349-399`) | **Yes** — serve-boot-decoupling regression + #512-class lockout + self-record around the gate | **No** — §8 raises it; the "intended close" covers only non-`complete` cursors | **STANDS → needs_revision** |
| **M4** | falsifier_2 (`dialogue:3`) | F16 asserts production `OwnerBundles()` contains 0021 in step 2 before 0021 authored in step 7 (`HOLDER.md:439-442,845-849,870-872`) | **Spec-edit only** — not a safety defect | **No** — test contract inconsistent with rollout order | **Folds into the revision** (non-blocking alone) |

## 3. Why the verdict turns the way it does

### M1 — full-transcript verification: RESOLVED in the deployer path

The v4 break was that the transcript/binary check covered only **not-yet-applied** steps,
so a resume binary whose already-applied bytes differed (`B45 ≠ A45`) was classified
resume-off-stored-transcript, completed the plan, and the C1 finalizer self-recorded the
running binary's byte-derived `ExpectedFingerprint()` and marked `complete` — serving a
hybrid as in-sync. v5 closes this **inside the deployer**: `VerifyStoredTranscript`
(§3.4a) byte-checks **every** stored step against the binary (reusing `MigrationSHASet()`
and `OwnerBundle.SHA256()` — the exact hashes `ExpectedFingerprint()` composes) and
DB-stamp-checks every already-applied step, on every resume and as finalizer step 0
before any `schema_state` write; ANY mismatch is a typed halt with the database
untouched. F15 reproduces the v4 `A45`/`B45` case (asserting `deploy_plan_binary_mismatch`,
no step-1 apply, `RecordSchemaFingerprint` un-called, no `complete` cursor), plus the
symmetric owner-step and DB-stamp arms and a finalizer arm; F14 is extended with an
already-applied tamper. **falsifier_1 explicitly concedes** the v4 already-applied
deploy-step mismatch no longer reproduces inside the resume/finalizer path. The binding
BC-N1/Q3/R4 binary-verification prescription is now implemented across the whole
transcript. **M1's deployer-path resolution is genuine.**

What M1's resolution **relies on** is Universal Invariant B — "no fingerprint / `complete`
is ever written unless the full stored transcript byte-matches the binary." M3 shows that
invariant is not airtight (below), so M1 is resolved at its core but its hardening
boundary is breached.

### M2 — owner-ddl self-heal exclusion: RESOLVED as a safety contract

The v4 break was that the FMA-007 self-heal `ReapplyAllOwnerBundles` re-ran every loaded
bundle "regardless of recorded version" over a slice that included 0021, so a cross-bundle
dependency repair during `owner-ddl apply` could commit `REVOKE CREATE` early. v5 binds a
single non-revoke filter (`isNonRevokeBundle`, `< DDLRevokeOwnerBundleVersion = 21`) and a
split loader (`OwnerDDLApplyBundles()`) across **every** apply route — `ApplyOwnerBundles`
loads the filtered slice (`owner.go:269`), both `applyPendingOwnerBundles` and
`ReapplyAllOwnerBundles` add in-loop `isNonRevokeBundle` guards, and the nil-fallback
loads the filtered slice — two independent barriers on the FMA-007 path. The embed/listing
split keeps `OwnerBundles()` (full) for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/
`RuntimeOwnedTablesAlterable` while `owner-ddl apply` never iterates 0021. F16 forces the
real cross-bundle self-heal (injecting a `42P01`-class failure so
`isCrossBundleDependencyError` fires) and asserts 0021 is not applied,
`owner_bundle_meta` never records 21, and CREATE stays held; F12/`G-revoke-last` extended
with the owner-ddl side-path. **Both falsifiers explicitly concede** 0021 is no longer
reachable through any `owner-ddl apply` route. **M2's safety invariant is resolved.** The
only residual is M4 (the F16 test contract vs. rollout order), a staging defect.

### M3 — complete-cursor legacy self-record / mutation bypass: STANDS (the blocker)

This is the load-bearing reason the gate does not clear. I verified falsifier_1's
reproducer against the holder's own §3.3a predicate, §4.5 Invariant B, §8 open tension,
and the holder's own anchor table for the `connection.go` boot order:

- **The predicate has the hole.** §3.3a step 3 returns nil on `cursorState == complete`
  (defer to the drift gate, `HOLDER.md:480-482`); the
  `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt is only in step 4's
  `cursorState == none` branch (`HOLDER.md:483-489`). A revoke-embedding binary with a
  `complete` cursor and the flag OFF is **not** fail-closed.
- **The boot order makes it material.** The holder's own anchor table (§9) records
  `ConnectAndMigrate` as watermark `:349` → apply `:353` → drift gate `:376-383` → self-record
  `:399`. `CheckDeployActivation` runs before `:353` but returns nil, so `ApplyMigrations`
  runs. The drift gate is **post-apply** — too late to prevent the mutation — and in the
  default shadow mode it logs and falls through to `RecordSchemaFingerprint`.
- **The harm is real and reachable.** The normal post-deploy steady state is `complete` +
  0021 applied (CREATE revoked). A later binary that still embeds 0021 is deployer-aware;
  booting it with the flag OFF (a realistic operational state the spec's own
  `awaiting_deploy_config` halt was designed to catch) drives the legacy mutator. A
  pending runtime step needing CREATE → #512-class lockout; a step needing no CREATE →
  serve-path schema mutation after P4; either way the legacy `:399` self-record can write
  `schema_state` around `VerifyStoredTranscript`.
- **The spec does not close it.** §4.5 Invariant B asserts the legacy self-record only
  fires with "no transcript, cursor absent" — M3 exhibits a transcript-**present**,
  cursor-`complete` path that reaches it, so the invariant as stated is **false**. The
  holder's §8 raises this exact tension (`HOLDER.md:907-914`) but its "intended close"
  explicitly covers only **non-`complete`** cursors, and F11 omits the
  `complete`+revoke-embedded+flag-OFF+pending-step case.

The holder's strongest available rebuttal — a `complete` cursor with a matching
fingerprint is legitimately in-sync, and the happy choreography restarts with the flag ON
— holds only for the immediate restart of the **same** binary. A `complete` cursor is
complete for the **previous** plan, not proof the current binary has no pending change,
and the post-apply drift gate cannot prevent a mutation that has already run. This is
precisely the "unaudited sibling self-record path" the SEED warned would spawn an M3. It
is **material** (a serve-boot-decoupling regression and a self-record-around-the-gate
path, both in the correctness/safety core) and **stands unrebutted** (this cycle carries
no holder rebuttal artifact). Per the rubric — "a serve-boot decoupling that regresses
[the boundary]" and a self-record that can write a fingerprint around the full-transcript
check are explicit needs_revision triggers — **M3 alone forces `needs_revision`.**

### M4 — F16 test contract vs. rollout order: real but non-blocking on its own

falsifier_2 concedes M2's safety invariant and finds the F16
`TestOwnerDDLApplyExcludesRevokeBundle` cannot build green as specified: it asserts
production `OwnerBundles()` contains 0021 in rollout step 2, before 0021 is authored in
step 7. This is a **test-staging** inconsistency — a real spec edit (split the test into
a pre-0021 synthetic phase and a post-0021 production phase), but not a safety or
correctness defect. On its own this would be `accept_with_findings`. It is recorded as a
required secondary fix so the v5 build does not inherit an un-buildable named test.

### BC-N1 / BC-N2 / C1 / C2 / C3 — INTACT

All five carry-forward findings are preserved verbatim and unregressed, and both
falsifiers concede each. **BC-N1** (immutable `deploy_plan` transcript, materialize-
before-step-0, resume off the stored transcript, §1.3 + doctor + F14) — M1 extends its
binary-verification clause without re-opening the moving-frontier mechanism. **BC-N2**
(universal non-complete-cursor edge at `applied_owner == 20`, F11(e)/(f)) — falsifier_1
explicitly distinguishes M3 ("this is not the old BC-N2 pre-revoke non-complete window")
as the orthogonal **complete**-cursor window, so BC-N2's resolved edge is not regressed.
**C1** (`finalizing` + idempotent finalizer + §1.3 row + F10) — M1 adds a pre-finalizer
gate without changing boundary semantics; note M3's harm flows through the **legacy**
`connection.go:399` self-record, a different writer than the C1 deployer finalizer (which
**is** gated). **C2** (`CheckDeployActivation` before `ApplyMigrations`, typed halts,
forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion = 20`) — the
v4-resolved edge is unregressed; M3 is a **new** hole in the `complete`-cursor branch the
prior cycles never surfaced, not a C2 regression. **C3** (0021 special-cased + terminal +
revoke-last, F12/`G-revoke-last`) — M2 binds the executable exclusion across the self-heal
without changing the revoke-last mechanism; the activation deploy still completes.

## 4. What the revision must fix (exact)

A re-scaffolded revision would clear the gate only if it pins **both** items below, while
keeping M1's full-transcript verifier, M2's non-revoke filter, and BC-N1/BC-N2/C1/C2/C3
intact and unregressed.

1. **Close the `complete`-cursor legacy self-record / mutation bypass (M3 — the
   load-bearing fix, C2/decoupling-boundary + M1-Invariant-B core).**
   - Make `revokeEmbedded && !decoupledEnabled` a **pre-apply, DB-untouched halt for
     EVERY cursor state, including `complete`** — the conservative rule: a revoke-
     embedding binary with `STRIATUM_DEPLOY_DECOUPLED` OFF returns `awaiting_deploy_config`,
     DB untouched, before `ApplyMigrations` and before `RecordSchemaFingerprint`, on both
     `ConnectAndMigrate` and `ConnectAndVerify`. (If the design instead wants to permit a
     flag-OFF restart after a completed deploy, it must add a **pre-`ApplyMigrations`**
     plan/fingerprint comparison that cannot mutate or self-record — not rely on the
     current **post-apply** `CheckSchemaDrift`.)
   - Tighten **Universal Invariant B** so a database carrying `deploy_cursor` /
     `deploy_plan` can **never** reach the legacy `connection.go:399` writer: the legacy
     self-record is permitted only when no deploy transcript exists and the binary is not
     on the P4 revoke/deploy path.
   - Extend **F11 / F15** with the complete-cursor case: `cursorState == complete`,
     `revokeEmbedded == true`, `STRIATUM_DEPLOY_DECOUPLED` OFF, with a pending runtime
     migration (or a changed expected fingerprint). Assert `awaiting_deploy_config`,
     `ApplyMigrations` **not** called, `RecordSchemaFingerprint` **not** called,
     `schema_state` unchanged, and the DB byte-identical.

2. **Split F16 into phase-aware checks so M2's filters land green before 0021 exists (M4
   — secondary, test-staging).**
   - Pre-0021 / inert phase: use a synthetic bundle list or a test hook to prove
     `OwnerDDLApplyBundles` / `isNonRevokeBundle` exclude every bundle `>= 21`,
     `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles` skip a hand-passed synthetic
     0021, and `ReapplyAllOwnerBundles(nil, …)` uses the filtered loader. Do **not**
     assert production `OwnerBundles()` contains 0021 yet.
   - Activation phase (after 0021 is authored): assert production `OwnerBundles()`
     contains 0021, `ExpectedFingerprint()` includes its bytes, `revokeEmbedded` derives
     from the full loader / file presence, and production `OwnerDDLApplyBundles()`
     excludes it.
   - Keep the forced-self-heal pgtest in the activation phase (or make its synthetic
     fixture explicit) and require it to prove it actually reaches `ReapplyAllOwnerBundles`
     through `isCrossBundleDependencyError`, not merely the pending loop.

## 5. What already cleared (carry forward — do NOT regress or re-litigate)

- **M1 full-transcript verifier — resolved this cycle** (§3 above): preserve
  `VerifyStoredTranscript` over the whole transcript (byte + already-applied DB stamp), on
  every resume **and** as finalizer step 0, with the typed
  `deploy_plan_binary_mismatch` / `deploy_plan_db_stamp_mismatch` halts, F15, and the
  extended F14. (The M3 fix **adds** the missing legacy-path / complete-cursor guard; it
  does not change the deployer verifier.)
- **M2 non-revoke filter — resolved this cycle**: preserve `DDLRevokeOwnerBundleVersion =
  21`, `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, the in-loop guards on both apply
  loops, the nil-fallback split, the embed/listing split, and F16's safety assertions.
  (M4 only restructures F16's **staging**, not the filter.)
- **BC-N1 immutable transcript / resume-off-stored-plan — intact**: preserve the 0044
  `deploy_plan` table, materialize-before-step-0, the §1.3 stored-transcript
  reclassification, the transcript-enumerated doctor, and F14.
- **BC-N2 universal non-complete-cursor edge — intact**: preserve the
  `revokeEmbedded`-independent halt at `applied_owner == 20` and F11(e)/(f). (M3 concerns
  the **complete** cursor, a different state — do not weaken the non-complete edge.)
- **C1 finalization boundary — intact**: preserve the `finalizing` state + idempotent
  finalizer + §1.3 row + F10, with M1's pre-finalizer `VerifyStoredTranscript` step 0.
- **C2 fail-closed activation edge — intact**: preserve `CheckDeployActivation` before
  `ApplyMigrations`, the typed halts, the forward-watermark rule (fires at `applied >=
  21`), and `RequiredOwnerBundleVersion = 20` (its current-`main` value, **not** advanced
  to the revoke ordinal). The M3 fix **extends** this edge to the complete-cursor case; it
  must not advance `Required` or alter the watermark.
- **C3 revoke-last ownership mechanism — intact**: preserve 0021 special-cased + terminal
  + the reconcile-while-CREATE-held ordering + F12 / `G-revoke-last`.
- **Q4 — resolved** (v1–v4): plain verb now, bootstrapping paradox dispositive, run-shape
  door held by the three seams. Neither falsifier contested it.
- **Q3 mid-step resumability body — sound**: Q3-A (cursor advance + receipt in the step
  transaction), Q3-B (`in_progress` pre-marker + invalid-index reconciler), the §1.1 step
  taxonomy. M1/M3 concern verification and serve-boot guarding, not the per-step
  schema/cursor body — preserve the body.
- **Decoupling shape — sound** (subject to the M3 fix): shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark interlock +
  P3 drift gate, the embed-FS-derived plan, the hash-chained per-step receipt.
- **Scope discipline held**: P5 deferred; full capability revocation named as a follow-up
  beyond P4, not assumed; the local-first boundary respected.

## 6. Gate decision

The ledger verdict — not falsifier completion — clears the phase gate. **M1 and M2 are
both genuinely resolved** (both falsifiers concede each), and **BC-N1, BC-N2, C1, C2, and
C3 are carried forward intact**, and the **proactive-hardening audit is present**. But a
clearing verdict also requires **no new material challenge standing**, and **M3** does —
source-verified against the spec's own §3.3a predicate, §4.5 Invariant B, §8 open
tension, and the holder's own `connection.go` boot-order anchors:

- **M3**: a `complete` deploy cursor returns nil from `CheckDeployActivation`, so a
  revoke-embedding binary with `STRIATUM_DEPLOY_DECOUPLED` OFF rides the legacy
  `ConnectAndMigrate` path over a database that already carries a deploy transcript —
  running `ApplyMigrations` (a #512-class lockout if a pending step needs CREATE, or a
  serve-path schema mutation otherwise) and a legacy `RecordSchemaFingerprint` self-record
  around `VerifyStoredTranscript`, falsifying Universal Invariant B. The spec's §8 raises
  the tension but its "intended close" covers only non-`complete` cursors, so the spec as
  written does not close it.

**M4** (F16's production-loader assertion contradicting the rollout order) is a real but
non-safety test-staging finding that folds into the revision.

**Verdict: `needs_revision`.** This is the gate's single allowed revision cycle, so the
gate ends **unCleared** — judged on the merits. A clearing verdict would require a
re-scaffolded revision that pins both §4 items (the M3 complete-cursor pre-apply halt /
tightened Invariant B + the named complete-cursor test as the load-bearing fix; the M4
phase-aware F16 split as the secondary fix) without regressing M1's full-transcript
verifier, M2's non-revoke filter, or BC-N1 / BC-N2 / C1 / C2 / C3.

<sub>Adjudicator collaboration ledger for the RFC 0142 P4 falsification-gate design run,
v5 / REVISION, cycle 1. Inputs:
`docs/operator/artifacts/rfc-0142-p4-design-v5/dialogue/holder/HOLDER.md`,
`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, `SEED.md`;
context: v4 `HOLDER.md` + v4 `COLLABORATION_LEDGER_cycle_1.md` (its §4 + the M1/M2
findings). Source verification against current `main` (HEAD `2e482077`):
`go/pkg/db/connection.go:349-353,376-399`, `go/pkg/db/schema_drift.go:83-100,145-161,171-195`,
`go/pkg/db/owner.go:23,35,197-224,269,277-302,309-322,332-351,367-374,511-541`,
`go/pkg/db/migrations.go:197,275,304-355`; HOLDER anchors as cited. Verdict vocabulary:
accept | accept_with_findings | needs_revision | reject.</sub>

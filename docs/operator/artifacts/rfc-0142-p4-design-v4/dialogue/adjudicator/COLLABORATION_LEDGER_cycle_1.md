---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec (v4 REVISION; resolves cycle-3 BC-N1 moving-frontier + BC-N2 pre-revoke serve, carries C1/C2/C3 forward, re-anchored 0020→0021 / 19→20)"
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
    text: "Revised P4 spec (v4): resolves the two binding cycle-3 findings and carries C1/C2/C3 forward verbatim, re-anchored to current `main` (HEAD 5c5cb36a; DDL-revoke bundle renumbered 0020→0021, owner-bundle frontier 19→20). BC-N1 by materializing the IMMUTABLE ordered transcript ONCE before step 0: a new runtime-owned `deploy_plan` table keyed by `plan_hash` stores base/target frontiers, the terminal-revoke `step_index`, and the full ordered `[{step_index, step_id, role, sha256, transactional}]` transcript, written in the same transaction that sets `deploy_cursor → in_progress(0)` before any frontier-advancing DDL; resume loads `deploy_plan` by `cursor.plan_hash` (never recomputes `BuildPlan` over moved frontiers), §1.3 and the per-step doctor key off the stored transcript so `(plan_hash, step_index)` is stable; `T-deploy-plan-hash-resume-after-step` (F14). BC-N2 by making `deploy_cursor` authoritative before the terminal revoke via one UNIVERSAL hard edge: `CheckDeployActivation` is no longer gated on `revokeEmbedded` — every deployer-aware binary (incl. the no-revoke landing binary) reads `deploy_cursor` after `CheckOwnerBundleWatermark` and before `ApplyMigrations`/`RecordSchemaFingerprint` and halts `awaiting_deploy` DB-untouched on a non-`complete` cursor at `applied_owner == 20`; F11 extended (case e/f) + `G-old-binary-refuse` extended. C1 (`finalizing` state + idempotent finalizer + §1.3 row + F10), C2 (`CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark rule re-anchored to 21, `RequiredOwnerBundleVersion` kept at its current-`main` value 20, F11), and C3 (DDL-revoke bundle 0021 special-cased out of the owner prefix + excluded from `owner-ddl apply` via `applyPendingOwnerBundles` bounded `<= 20` + a generic build guard + terminal-ordering, F12/`G-revoke-last`) carried forward unregressed and kept coherent with the stored-transcript receipt key and the universal edge."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "BC-N1's moving-frontier instability and BC-N2's pre-revoke serve window are both GENUINELY fixed (the v3 reproducers no longer reproduce), and no direct C1/C2/C3 regression. But a NEW material challenge stands squarely inside the BC-N1/Q3 correctness core: the v4 transcript/binary verification rule is specified only for NOT-YET-APPLIED steps, leaving a real transcript-vs-binary mismatch path for ALREADY-APPLIED steps. §1.3 resumes when `deploy_plan[plan_hash]` is present and 'not-yet-applied steps' sha256 match this binary's embedded bytes' (HOLDER.md:235), and `deploy_plan_binary_mismatch` fires only on a not-yet-applied step mismatch (HOLDER.md:238); F14's negative case tampers a not-yet-applied stored step (HOLDER.md:766); and §8 itself concedes the already-applied check (HOLDER.md:833-839) — 'as written, the answer is no.' Reproducer (the v4 two-runtime-step F14 shape): binary A materializes H = [step0 runtime:0045 sha A45, step1 runtime:0046 sha A46, step2 owner:0021 sha A21]; step 0 commits by the Q3-A path (runtime DDL + schema_migrations sha A45 + cursor advance + per-step receipt (H,0,runtime:0045,A45) in one owner-connection tx, `migrations.go:304-355`); crash after step 0. Resume with binary B whose already-applied 0045 bytes differ (B45) but whose not-yet-applied 0046/0021 still match (plausible if the activation binary is rebuilt/amended after the partial deploy). Under the v4 text the not-yet-applied check passes; step 0 is already applied, so its byte mismatch is not in the checked set; the cursor classifies as resume-off-stored-transcript, NOT `deploy_plan_binary_mismatch` (HOLDER.md:235-238). The deployer resumes at step 1, applies 0046+0021, then the C1 finalizer records B's `ExpectedFingerprint()` into `schema_state` via `RecordSchemaFingerprint` (HOLDER.md:212-218). In current source `ExpectedFingerprint()` hashes the running binary's embedded migration/bundle BYTES (`schema_drift.go:83-99`), and `LiveFingerprint` reads the self-recorded singleton, not a recomputation (`schema_drift.go:145-160`), and `CheckSchemaDrift` compares the row to this binary's expected value (`schema_drift.go:239-274`). So binary B finishes a plan whose applied step 0 is actually A's bytes, then writes B's fingerprint as the durable 'live' value; the receipt chain and `schema_migrations` still record A45, but a later B boot sees `LiveFingerprint == ExpectedFingerprint` and serves a hybrid as in-sync. The tightened doctor enumerates missing receipts from `deploy_plan` (HOLDER.md:572-584), but every receipt is present; it specifies no stored-step-SHA-vs-database-stamp check for already-applied steps. The v3 adjudicator's exact repair required resume to 'verify the embedded bytes still match the binary' (v3 ledger:283-285) — NOT 'verify only future bytes.' Required repair: on every resume validate the ENTIRE stored transcript (incl. already-applied steps) against the binary; also verify the DB stamps (`schema_migrations.sha256` / `owner_bundle_meta.sha256`) match the stored transcript for already-applied entries; apply the same full-transcript check before the finalizer writes `schema_state` or advances `finalizing → complete`; extend F4/F14/F13 with an already-applied mismatch case (and the symmetric owner-step case). Standing falsification until the resume/finalizer path verifies the entire stored transcript."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "BC-N2 is genuinely fixed (no-revoke pre-revoke serve window closed) and BC-N1's moved-frontier receipt-key issue is addressed in the stored-transcript design; no direct C1/C2 regression. But a NEW material challenge stands in the C3/R3 core: the v4 'C3 carried forward' claim that 0021 is unreachable through `owner-ddl apply` is not implementation-safe, because the spec closes only one of the two `owner-ddl apply` surfaces. §3.2 binds only `applyPendingOwnerBundles` to `<= LatestOwnerBundleVersion (20)` and adds a generic build guard (HOLDER.md:351-358); §4.4 states there is 'no path' by which the revoke commits early (HOLDER.md:723-739). But on a cross-bundle dependency failure `ApplyOwnerBundles` intentionally falls into `ReapplyAllOwnerBundles` as an ordered FMA-007 self-heal (`owner.go:277-302`), and that function re-runs EVERY shipped owner bundle in ascending order regardless of recorded version (`owner.go:324-349`). If the activation binary's `OwnerBundles()` includes 0021 so that `revokeEmbedded` can detect the file, the full loaded slice the reapply branch iterates includes 0021 — and the v4 text neither names nor binds that branch. Concrete refutation: DB at owner frontier 20, before the activation deploy; run `striatum daemon owner-ddl apply` on the activation binary (which embeds 0021 but keeps `Latest=Required=20`); inject the already-supported FMA-007 condition (a `<= 20` bundle fails with a cross-bundle dependency error); `ApplyOwnerBundles` then calls `ReapplyAllOwnerBundles(ctx, runner, bundles, daemonVersion)` over the full loaded slice (`owner.go:289`), which applies 0021 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) outside the deploy plan, before any pending runtime step and before the C3 ownership reconcile. That recreates the v2 C3 contradiction: a later deployer runtime step tries to reconcile new runtime objects after `striatumd_rw` no longer holds CREATE; the spec's `deploy_create_prerequisite_missing` guard halts before DDL — a #512-class lockout in the activation path, not merely a missed cleanup. §8 itself asks whether `owner-ddl apply` on the activation binary is truly bounded (HOLDER.md:850-856); the spec answers only the direct pending loop and the happy deploy-plan test (F12/`G-revoke-last` only prove the happy ordering, HOLDER.md:764). A generic build guard is too easy to satisfy against the normal path while leaving the reconciliation escape hatch open. Required repair: a single non-revoke bundle filter (excluding every bundle `>= DDLRevokeOwnerBundleVersion`) used by EVERY `owner-ddl apply` path incl. `applyPendingOwnerBundles`, `ReapplyAllOwnerBundles`, tests, dry-run/list; a pgtest that forces the cross-bundle self-heal with a synthetic 0021 and asserts 0021 is not applied and CREATE remains held; extend F12/`G-revoke-last` with the owner-ddl side-path + FMA-007 reapply case; split the embed/listing helper so 'binary embeds 0021' does not imply 'owner-ddl apply can iterate 0021.' Until the self-heal branch is explicitly bounded and tested, a cross-bundle dependency repair can apply the revoke early and recreate the lockout the revision is supposed to remove."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "REVISION cycle v4 (the gate's single allowed revision cycle). The two binding cycle-3 findings are both genuinely fixed: BC-N1's moving-frontier receipt-key instability is RESOLVED by the immutable `deploy_plan` transcript materialized before step 0 + resume off the stored transcript by `cursor.plan_hash` + §1.3 and the per-step doctor keyed to the stored transcript + `T-deploy-plan-hash-resume-after-step` (F14) — BOTH falsifiers explicitly concede the v3 reproducer (re-run rebuilding H' over the moved frontier) no longer reproduces; BC-N2's pre-revoke serve window is RESOLVED by the universal `CheckDeployActivation` cursor edge (no longer gated on `revokeEmbedded`; every deployer-aware binary halts `awaiting_deploy` DB-untouched on a non-`complete` cursor at `applied_owner == 20`, before `ApplyMigrations`/`RecordSchemaFingerprint`) + the extended F11 case (e)/(f) + extended `G-old-binary-refuse` — BOTH falsifiers explicitly concede it and decline to use it as a blocker. C1 (the `finalizing` state + idempotent finalizer + §1.3 row + F10) and C2 (the fail-closed `CheckDeployActivation` edge, typed halts, forward-watermark rule re-anchored to 21, `RequiredOwnerBundleVersion` correctly kept at its current-`main` value 20 and NOT advanced to the revoke ordinal 21) are carried forward INTACT and unregressed; the C3 ownership-transfer MECHANISM (0021 special-cased + terminal + revoke-last, re-anchored 0020→0021) is carried forward and both falsifiers concede the design-level answer. BUT the gate does NOT clear, on two independent NEW material challenges, each source-grounded against current `main` and each landing inside a correctness/safety core: (M1, falsifier_1, BC-N1/Q3 core) the v4 transcript/binary verification rule verifies only NOT-YET-APPLIED steps, so a resume binary whose ALREADY-APPLIED step bytes differ (B45 != A45) passes the not-yet-applied check, is classified resume-off-stored-transcript rather than `deploy_plan_binary_mismatch`, completes the plan, and the C1 finalizer self-records the running binary's `ExpectedFingerprint()` (which hashes embedded file BYTES, `schema_drift.go:83-99`) into the self-recorded `schema_state` singleton (`LiveFingerprint` does not recompute from `schema_migrations`, `schema_drift.go:145-160`) and marks `complete` — converting a hybrid A-applied/B-expected deploy into an apparently in-sync B deploy, the exact transcript-vs-binary mismatch the run was asked to force into `deploy_plan_binary_mismatch`; the v3 prescribed BC-N1 fix said 'verify the embedded bytes still match the binary' UNQUALIFIED, and v4 narrowed it to future steps, so the prescribed sub-protocol is only partially implemented and the holder's own §8 (HOLDER.md:833-839) concedes the already-applied check does not fire. (M2, falsifier_2, C3/R3 core) the 'C3 carried forward' guarantee that 0021 is unreachable through `owner-ddl apply` binds only `applyPendingOwnerBundles` (`owner.go:309-320`); the FMA-007 self-heal branch `ReapplyAllOwnerBundles` (reached from `ApplyOwnerBundles` on a cross-bundle dependency error, `owner.go:277-302`) re-runs every loaded bundle regardless of recorded version (`owner.go:324-349`), and since the activation binary's `OwnerBundles()` must include 0021 for `revokeEmbedded` detection, a cross-bundle dependency repair can commit `REVOKE CREATE` early — before the runtime ownership reconcile — recreating the #512-class CREATE lockout the revoke-last mechanism exists to prevent; the v4 build guard is described generically and the spec names only the pending loop, so the executable requirement that actually closes the violating branch is absent. Both M1 and M2 are material (each exposes a real correctness/safety defect that changes the spec), source-verified against current `main`, and stand UNREBUTTED by the spec as written (this cycle carries no holder rebuttal artifact). Per the role rubric a concrete per-step-receipt interleaving where the resumable-cursor contract is insufficient with no stricter sub-protocol ALONE forces needs_revision; M1 is exactly that, and M2 is a concrete owner-ddl side-path that strands a reconcile under a revoked CREATE. Because this is the single allowed revision cycle, the gate ends unCleared; §4 records exactly what a re-scaffolded revision must fix."
findings:
  - id: BC-N1
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED on the moving-frontier defect; the prescribed binary-byte-verification clause is INCOMPLETELY IMPLEMENTED (surfaced as M1). The v3 break — a re-run rebuilding `H' != H` over the moved frontier and renumbering `step_index` — is provably closed: the new runtime-owned `deploy_plan` table (migration 0044) stores the IMMUTABLE ordered transcript keyed by `plan_hash` (base/target frontiers, terminal-revoke `step_index`, every `{step_index, step_id, role, sha256, transactional}`), materialized in the SAME transaction that sets `deploy_cursor → in_progress(0)` BEFORE step 0 mutates the frontier (HOLDER.md:164-204); resume loads `deploy_plan[cursor.plan_hash]` and resumes at the stored `step_index`, never recomputing `BuildPlan` over the moved frontiers (§1.3 HOLDER.md:232-260, §3.2 HOLDER.md:375-380); §1.3 reclassifies the v3 unclassified-foreign-incomplete bucket as recoverable 'resume with the STORED plan' (HOLDER.md:236); the per-step `doctor schema_deploy_unrecorded` enumerates applied steps from the stored transcript (HOLDER.md:572-584); `T-deploy-plan-hash-resume-after-step` (F14) kills after step 0 AND step 1, asserting the re-run reuses `plan_hash`, preserves `step_index`es, recognizes prior receipts exactly-once, and ends green (HOLDER.md:766). BOTH falsifiers explicitly confirm the moving-frontier reproducer no longer reproduces. HOWEVER the BINDING v3 prescription was 'load the stored transcript by `plan_hash`, verify the embedded bytes still match the binary, and resume at the next step' (v3 ledger §4(1), :283-285) — unqualified across the whole transcript. The v4 spec verifies only NOT-YET-APPLIED steps' bytes (HOLDER.md:235,238; F14 negative case tampers a not-yet-applied step, :766), so the binary-compatibility half of the prescribed sub-protocol is only partially implemented — the holder's own §8 (HOLDER.md:833-839) concedes the already-applied check does not fire. This gap is a NEW material challenge (M1) inside the BC-N1 core and forces needs_revision there."
    affected_invariants: ["BC-N1 immutable plan identity across resume", "stable `(plan_hash, step_index)` receipt key", "section 1.3 incomplete-vs-drift disambiguation", "Q3 resume at the next clean boundary across a moved frontier"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: BC-N2
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED — genuinely fixed, conceded by both falsifiers. The pre-revoke incomplete-deploy serve window is closed by one UNIVERSAL hard edge: `CheckDeployActivation` is no longer gated on `revokeEmbedded`; EVERY deployer-aware binary — including the no-revoke landing binary — reads `deploy_cursor` immediately after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations` and BEFORE `RecordSchemaFingerprint` in both boot paths, and halts `awaiting_deploy` DB-untouched whenever the cursor exists and `state != complete`, regardless of `revokeEmbedded`/watermark (§3.3a step 1 HOLDER.md:404-421; net invariant HOLDER.md:449-456). The edge holds at the v4 pre-revoke frontier `applied_owner == 20` (re-anchored from 19), exactly the window the C3 revoke-last ordering opens, and does NOT block a fresh/idle DB (absent cursor / `idle` serves on the legacy path), so C3 is not regressed and a clean boot is not wedged. F11 is extended with case (e) — a NO-0021 deployer-aware binary at `applied_owner == 20` with `deploy_cursor.state ∈ {in_progress, step_committed, finalizing}` halts `awaiting_deploy`, with spies proving `ApplyMigrations` and `RecordSchemaFingerprint` are NOT called and the DB is byte-identical — and case (f) — the same binary with no cursor / `idle` SERVES (HOLDER.md:763); `G-old-binary-refuse` is extended to prove the pre-revoke window cannot be served (HOLDER.md:763). Both falsifiers state they do not use the BC-N2 reproducer as a blocker. **RESOLVED / INTACT.**"
    affected_invariants: ["BC-N2 pre-revoke `deploy_cursor`-authoritative edge", "incomplete cursor never serves (universal across deployer-aware binaries)", "P3/P4 self-record split (verify-only boot must not mask deploy state)", "R2 decoupling does not regress fresh-DB bring-up"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C1
    severity: critical
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed. The distinct `finalizing` cursor state, the idempotent finalizer (`complete` receipt guarded on `(plan_hash, state=complete)` → `schema_state` fingerprint UPSERT → advance `finalizing → complete` LAST), the §1.3 resumable-finalization row (never serve, never genuine-drift), the operator pin (no single finalization transaction spans the owner+runtime connections), and F10 (`T-deploy-resume-finalization-crash`) are preserved exactly and re-anchored to terminal 0021 (HOLDER.md:210-222, §1.3 HOLDER.md:237, crash-point table HOLDER.md:562-570). Kept coherent with BC-N1 (the finalizer's idempotency keys on the stored `plan_hash`) and BC-N2 (a `finalizing` cursor is non-`complete`, so the universal edge refuses to serve it). The finalizer mechanism is structurally intact and no direct regression is shown. HONEST NOTE (recorded under M1, not as a C1 regression): the finalizer is the VEHICLE through which M1's harm manifests — it writes the RUNNING binary's `ExpectedFingerprint()` (file-byte hash) into `schema_state` and marks `complete` WITHOUT a full-transcript binary/DB-stamp check, so under a wrong resume binary it converts a hybrid deploy into an apparently in-sync deploy. That is the BC-N1 sub-protocol gap (M1), not a C1 finalization-boundary regression; the `complete`-with-missing-receipt window remains closed. **C1 INTACT; preserve verbatim.**"
    affected_invariants: ["finalization boundary (no `complete` with missing receipt/fingerprint)", "Q3 per-step-atomic + resumable-cursor contract", "section 1.3 disambiguation table enforcement"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: C2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim and correctly re-anchored. `CheckDeployActivation` after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations` in both boot paths (mutating nothing), the typed `awaiting_deploy`/`awaiting_deploy_config` halts on the non-restartable exit, and the forward-watermark rule are preserved (§3.3a HOLDER.md:404-441, §4.2 HOLDER.md:623-671). RE-ANCHORING (doc-vs-source accuracy): the v3/SEED 'Required = 19' framing is stale against current `main`; verified `LatestOwnerBundleVersion == 20` and `RequiredOwnerBundleVersion == 20` (`owner.go:23,35`), and the v4 holder correctly keeps `RequiredOwnerBundleVersion` at its current-`main` value 20 and does NOT advance it to the revoke ordinal 21 (0021 is deploy-plan-only; the forward-watermark rule re-anchors to fire at `applied >= 21`) — this preserves the C2 mechanism ('Required is not advanced to the revoke ordinal') exactly, only the ordinals move. The v3 BC-N2 gap (the edge gated only revoke-embedding binaries) is closed by the BC-N2 universal edge as an ADDITION, not a replacement. The cleared EDGE is not regressed. **C2 INTACT.**"
    affected_invariants: ["serve-boot clean-halt (no crash-loop, no `42501`)", "fail-closed `CheckDeployActivation` edge before `ApplyMigrations`", "forward-watermark rule", "`RequiredOwnerBundleVersion` not advanced to the revoke ordinal"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C3
    severity: high
    posture: design
    status: answered
    challenge: "DESIGN MECHANISM INTACT (not regressed); the 'excluded from `owner-ddl apply`' GUARANTEE is INCOMPLETELY IMPLEMENTED (surfaced as M2). The ownership-transfer / revoke-last mechanism is carried forward verbatim and re-anchored 0020→0021: bundle 0021 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) is identified by `DDLRevokeOwnerBundleVersion = 21`, special-cased out of the owner prefix, appended as the TERMINAL plan step after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE, granted by 0018 in the non-revoke prefix; the committed steady state denies CREATE), and `LatestOwnerBundleVersion` stays 20 (§3.2 HOLDER.md:340-358, §3.3b HOLDER.md:458-515, §4.4 HOLDER.md:723-739). BOTH falsifiers explicitly concede the design-level answer. HOWEVER the carry-forward prescription required '`ApplyOwnerBundles` bounded below [the revoke]' (SEED C3, :122-124). The v4 spec bounds only `applyPendingOwnerBundles` (`owner.go:309-320`) and adds a GENERIC build guard, but does not name or bind the FMA-007 self-heal branch `ReapplyAllOwnerBundles` (reached from `ApplyOwnerBundles` on a cross-bundle dependency error, `owner.go:277-302`), which re-runs every loaded bundle regardless of recorded version (`owner.go:324-349`) and — because `OwnerBundles()` must surface 0021 for `revokeEmbedded` detection — can commit 0021 early, before the runtime reconcile, recreating the #512-class lockout. This is a NEW material challenge (M2) inside the C3 core and forces needs_revision there. The design is sound; the executable exclusion is incomplete. **C3 design INTACT; exclusion guarantee under-specified (M2).**"
    affected_invariants: ["two-role owner/runtime boundary", "0021 unreachable through every `owner-ddl apply` route", "R3 no DDL-revocation lockout (no early REVOKE before a CREATE-needing reconcile)", "activation deploy can complete"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: M1
    severity: high
    posture: design
    status: open
    challenge: "OPEN — NEW material challenge (falsifier_1; BC-N1/Q3/R4 correctness core). The v4 stored-transcript verification rule checks only NOT-YET-APPLIED steps, so an ALREADY-APPLIED step whose bytes differ from the resume binary is not forced into `deploy_plan_binary_mismatch`, and the C1 finalizer self-records the wrong binary's fingerprint and marks `complete`. §1.3 resumes on 'not-yet-applied steps' sha256 match this binary's embedded bytes' (HOLDER.md:235) and fires `deploy_plan_binary_mismatch` only on a not-yet-applied mismatch (HOLDER.md:238); F14's negative case tampers a not-yet-applied step (HOLDER.md:766); §8 concedes the already-applied check does not fire (HOLDER.md:833-839). Reproducer (v4 two-runtime-step F14 shape): A materializes H=[runtime:0045 A45, runtime:0046 A46, owner:0021 A21]; step 0 commits A45 (schema_migrations + receipt (H,0,...,A45) atomically, `migrations.go:304-355`); crash; resume with binary B whose 0045 bytes are B45 but 0046/0021 still match. Not-yet-applied check passes; step 0's mismatch is unchecked; classified resume-off-stored-transcript; B applies 0046+0021 and the finalizer records B's `ExpectedFingerprint()` into `schema_state`. VERIFIED against current source: `ExpectedFingerprint()` hashes embedded migration/bundle BYTES keyed by version (`schema_drift.go:83-99`), `LiveFingerprint` reads the self-recorded singleton with NO recomputation from `schema_migrations`/catalog (`schema_drift.go:145-160`), and `RecordSchemaFingerprint` UPSERTs the running binary's expected value (`schema_drift.go:171-194`). So B finalizes a hybrid (A45 actually applied, B expected) and a later B boot reads `LiveFingerprint == ExpectedFingerprint` and serves it as in-sync; the receipt chain and `schema_migrations` still say A45 (provenance preserved) but the serve-gate fingerprint says B — a transcript-vs-binary mismatch NOT forced into `deploy_plan_binary_mismatch`, the exact class the run was asked to hunt and the holder claims is closed. The tightened doctor checks receipt PRESENCE (all present), not stored-SHA-vs-DB-stamp, so it stays green (HOLDER.md:572-584). The v3 prescribed BC-N1 fix said 'verify the embedded bytes still match the binary' UNQUALIFIED (v3 ledger:283-285); v4 narrowed it to future steps. MATERIAL (exposes a real drift-masking false-`complete` correctness defect; a per-step-receipt interleaving where the resumable contract is insufficient with no sub-protocol — alone forces needs_revision per the rubric). STANDS unrebutted → needs_revision."
    affected_invariants: ["BC-N1 wrong-resume-binary forced to `deploy_plan_binary_mismatch`", "P3/P4 fingerprint coherence (LiveFingerprint must reflect actually-applied schema)", "RFC 0142 Layer 3 deploy provenance vs serve-gate fingerprint agreement", "Q3 resume contract sufficiency for the changed-binary interleaving"]
    source_refs: ["dialogue:1", "dialogue:2"]
  - id: M2
    severity: high
    posture: design
    status: open
    challenge: "OPEN — NEW material challenge (falsifier_2; C3/R3 decoupling/migration-safety core). The v4 'C3 carried forward' claim that 0021 is unreachable through `owner-ddl apply` is not implementation-safe: the spec closes only the pending loop, not the FMA-007 self-heal branch. VERIFIED against current source: `ApplyOwnerBundles` runs `applyPendingOwnerBundles` and, on a cross-bundle dependency error (`isCrossBundleDependencyError`), falls into `ReapplyAllOwnerBundles(ctx, runner, bundles, daemonVersion)` over the full loaded slice (`owner.go:277-302`); `ReapplyAllOwnerBundles` applies `applyOneOwnerBundle` for EVERY bundle in ascending order 'regardless of the recorded version' with no `<= Latest` bound and no 0021 skip (`owner.go:324-349`). Since the activation binary's `OwnerBundles()` (the shared loader, `owner.go:196-224`) must include the 0021 file for `revokeEmbedded` detection (HOLDER.md:442-447, anchor table HOLDER.md:878), the slice the reapply branch iterates includes 0021. Reproducer: DB at owner 20; `striatum daemon owner-ddl apply` on the activation binary; inject the FMA-007 cross-bundle condition (a `<= 20` bundle fails on a missing earlier object); `ApplyOwnerBundles` calls `ReapplyAllOwnerBundles`, which commits `REVOKE CREATE` (0021) outside the deploy plan, before any runtime reconcile. A subsequent deployer runtime step then needs CREATE that is gone; the spec's `deploy_create_prerequisite_missing` guard halts before DDL → a #512-class lockout in the activation path. The v4 text binds only `applyPendingOwnerBundles` (HOLDER.md:351-358) and a generic build guard; §8 flags the question (HOLDER.md:850-856) but the body does not close the branch; F12/`G-revoke-last` prove only the happy deploy-plan ordering (HOLDER.md:764). MATERIAL (a real C3 lockout escape in current source; a generic guard satisfiable against the normal path while the reconciliation hatch stays open). The fix is a single non-revoke filter (`>= DDLRevokeOwnerBundleVersion` excluded) shared by `applyPendingOwnerBundles`, `ReapplyAllOwnerBundles`, tests, and dry-run/list surfaces, plus a forced-self-heal pgtest and an extended F12/`G-revoke-last` owner-ddl side-path case, plus splitting the embed/listing helper so 'binary embeds 0021' does not imply 'owner-ddl apply iterates 0021'. STANDS unrebutted → needs_revision."
    affected_invariants: ["0021 unreachable through every `owner-ddl apply` route (incl. FMA-007 self-heal)", "R3 no early REVOKE CREATE before a CREATE-needing reconcile", "C3 revoke-last ordering holds outside the happy deploy plan", "no #512-class lockout in the activation path"]
    source_refs: ["dialogue:1", "dialogue:3"]
branches:
  design: blocked
---

# COLLABORATION LEDGER — RFC 0142 P4 design run (v4 / REVISION), cycle 1

author: adjudicator-author-001

**Verdict:** `needs_revision`

> Scope of this adjudication: the curated dialogue trajectory only — the **revised
> (v4)** Holder's `HOLDER.md` P4 implementation spec and the two falsifiers'
> `FALSIFIER.md` challenges — plus the `SEED.md` charter, with the **v3** `HOLDER.md`
> and the **v3** collaboration ledger (its §4 "What the revision must fix") as
> context for what the revision had to fix. RFC 0142 is accepted (D258); this judges
> the **P4 implementation shape**, not the five-layer design. A clearing verdict
> would be `accept` or `accept_with_findings`; this is neither. This is the gate's
> **single allowed revision cycle**, so the gate ends **unCleared** — judged exactly,
> not graded on the consequence.

## 1. Decision

`needs_revision`. The revision is real, well-anchored work, and it **genuinely resolves
both binding cycle-3 findings**:

- **BC-N1 (moving-frontier receipt-key instability) — RESOLVED.** The immutable
  `deploy_plan` transcript is materialized once (base/target frontiers, terminal-revoke
  `step_index`, the full ordered `{step_index, step_id, role, sha256, transactional}`)
  in the same transaction that sets `deploy_cursor → in_progress(0)` **before step 0
  mutates the frontier**; resume loads the stored transcript by `cursor.plan_hash` and
  never recomputes `BuildPlan` over the moved frontiers; §1.3 reclassifies the v3
  unclassified-foreign bucket as recoverable "resume with the stored plan"; the per-step
  doctor enumerates from the stored transcript; `T-deploy-plan-hash-resume-after-step`
  (F14) kills after steps 0 and 1 and proves `plan_hash`/`step_index`/prior-receipt
  stability. **Both falsifiers confirm the v3 reproducer no longer reproduces.**
- **BC-N2 (pre-revoke incomplete-deploy serve window) — RESOLVED.** `CheckDeployActivation`
  is no longer gated on `revokeEmbedded`: **every** deployer-aware binary, the no-revoke
  landing binary included, reads `deploy_cursor` after the watermark check and before
  `ApplyMigrations`/`RecordSchemaFingerprint`, and halts `awaiting_deploy` DB-untouched on
  a non-`complete` cursor at `applied_owner == 20` (the re-anchored pre-revoke frontier).
  F11 case (e) proves `ApplyMigrations`/`RecordSchemaFingerprint` un-called and the DB
  byte-identical; case (f) proves a fresh/idle DB still serves. **Both falsifiers concede
  it and decline to use it as a blocker.**

It also carries **C1, C2, and C3 forward intact** (the `finalizing` finalizer; the
fail-closed `CheckDeployActivation` edge with `RequiredOwnerBundleVersion` correctly kept
at its current-`main` value 20; the revoke-last ownership mechanism re-anchored 0020→0021)
— both falsifiers concede the C3 design-level answer and show no direct C1/C2/C3 regression.

**But a clearing verdict requires BC-N1 resolved AND BC-N2 resolved AND C1/C2/C3 intact
AND no new material challenge standing.** Two new material challenges land — each
independently, each source-verified against current `main` (HEAD `5c5cb36a`), each squarely
in a correctness/safety core, and each tied to a binding prescription the v4 spec only
partially implements:

- **M1 (falsifier_1, BC-N1/Q3 core).** The v4 transcript/binary verification rule checks
  only **not-yet-applied** steps. A resume binary whose **already-applied** step bytes
  differ (`B45 ≠ A45`) passes that check, is classified resume-off-stored-transcript rather
  than `deploy_plan_binary_mismatch`, completes the plan, and the C1 finalizer self-records
  the running binary's `ExpectedFingerprint()` (a hash of embedded file **bytes**,
  `schema_drift.go:83-99`) into the self-recorded `schema_state` singleton
  (`LiveFingerprint` does **not** recompute from `schema_migrations`/catalog,
  `schema_drift.go:145-160`) and marks `complete`. That converts a hybrid (A-applied /
  B-expected) deploy into an apparently in-sync B deploy — the exact transcript-vs-binary
  mismatch the run set out to force into `deploy_plan_binary_mismatch`. The v3 prescribed
  BC-N1 fix said "**verify the embedded bytes still match the binary**" — unqualified —
  and v4 narrowed it to future steps; the holder's own §8 concedes the already-applied
  check does not fire.
- **M2 (falsifier_2, C3/R3 core).** The "0021 is unreachable through `owner-ddl apply`"
  guarantee binds only `applyPendingOwnerBundles`. The FMA-007 self-heal branch
  `ReapplyAllOwnerBundles` — reached from `ApplyOwnerBundles` on a cross-bundle dependency
  error (`owner.go:277-302`) — re-runs **every** loaded bundle "regardless of the recorded
  version" (`owner.go:324-349`), and the activation binary's `OwnerBundles()` must include
  0021 for `revokeEmbedded` detection. So a cross-bundle dependency repair during
  `owner-ddl apply` can commit `REVOKE CREATE` (0021) **early**, before the runtime
  ownership reconcile — recreating the #512-class CREATE lockout the revoke-last mechanism
  exists to prevent. The v4 build guard is generic and the spec names only the pending loop.

Both challenges are material, source-grounded, and stand **unrebutted** by the spec as
written (this cycle carries no Holder rebuttal artifact; the spec text stands). Per the
role rubric, a concrete per-step-receipt interleaving where the per-step-atomic +
resumable-cursor contract is insufficient with no stricter sub-protocol **alone forces
`needs_revision`**; M1 is exactly that, and M2 is a concrete `owner-ddl apply` side-path
that strands a reconcile under a revoked `CREATE`. The revision must fix exactly the two
items in §4.

## 2. Per-finding / per-challenge ledger

| # | Source | What is judged | Material? | Resolved / rebutted? | Disposition |
| --- | --- | --- | --- | --- | --- |
| **BC-N1** | cycle-3 finding / both falsifiers' revision-check | immutable `deploy_plan` transcript materialized before step 0; resume off stored transcript; §1.3 + doctor keyed to it; F14 (`HOLDER.md:164-204,232-260,375-380,572-584,766`) | — | **Yes — moving-frontier defect genuinely resolved** (both falsifiers concede the v3 reproducer is closed); binary-byte-verification clause only partially implemented | **RESOLVED** (core); binary-verification gap → **M1** |
| **BC-N2** | cycle-3 finding / both falsifiers' revision-check | universal `CheckDeployActivation` cursor edge (not gated on `revokeEmbedded`) at `applied_owner == 20`; F11(e)/(f) + extended `G-old-binary-refuse` (`HOLDER.md:404-421,449-456,763`) | — | **Yes — genuinely resolved**; both falsifiers decline it as a blocker | **RESOLVED / INTACT** |
| **C1** | cycle-3 cleared / both falsifiers | `finalizing` state + idempotent finalizer + §1.3 row + F10 carried forward, re-anchored 0021 (`HOLDER.md:210-222,237,562-570`) | — | **Yes — INTACT, unregressed**; finalizer structurally preserved | **INTACT** (vehicle of M1; not a C1 regression) |
| **C2** | cycle-3 cleared / both falsifiers | `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark re-anchored to 21, `Required` kept at 20 (`HOLDER.md:404-441,623-671`; `owner.go:23,35`) | — | **Yes — INTACT, correctly re-anchored** (19→20; `Required` not advanced to the revoke ordinal) | **INTACT** |
| **C3** | cycle-3 cleared / both falsifiers | revoke-last mechanism: 0021 special-cased + terminal + excluded from `owner-ddl apply` (`HOLDER.md:340-358,458-515,723-739`) | — | **Design Yes — both falsifiers concede**; exclusion guarantee binds only `applyPendingOwnerBundles` | **INTACT** (design); exclusion gap → **M2** |
| **M1** | falsifier_1 (`dialogue:2`) | already-applied-step binary mismatch: only not-yet-applied bytes checked; finalizer self-records the wrong binary's fingerprint, marks `complete` (`HOLDER.md:235-238,212-218`; `schema_drift.go:83-99,145-160,171-194`) | **Yes** — drift-masking false-`complete`; the exact wrong-binary class the run hunts | **No** — full-transcript / DB-stamp verification absent; §8 concedes | **STANDS → needs_revision** |
| **M2** | falsifier_2 (`dialogue:3`) | `ReapplyAllOwnerBundles` FMA-007 self-heal applies 0021 early via `owner-ddl apply`, stranding a reconcile without CREATE (`owner.go:277-302,324-349`; `HOLDER.md:351-358`) | **Yes** — #512-class lockout escape in current source | **No** — only the pending loop is bound; generic build guard | **STANDS → needs_revision** |

## 3. Why the verdict turns the way it does

### BC-N1 — immutable plan identity across resume: RESOLVED (core); binary-verification clause incomplete

The v3 break was that `BuildPlan(applied_owner, applied_runtime)` rebuilt the pending-delta
plan from live frontiers that advance at each committed step, so a re-run reconstructed a
different `plan_hash` and renumbered `step_index`, breaking the exactly-once receipt key, the
§1.3 resume classification, and the per-step doctor. The v4 spec dissolves this: a new
runtime-owned `deploy_plan` table (migration 0044) stores the **immutable** ordered
transcript keyed by `plan_hash` (base/target frontiers, terminal-revoke `step_index`, every
`{step_index, step_id, role, sha256, transactional}`), materialized in the **same
transaction** that sets `deploy_cursor → in_progress(0)` **before step 0 mutates the
frontier** (`HOLDER.md:164-204`). Resume loads `deploy_plan[cursor.plan_hash]` and resumes
at the stored `step_index`, **never** recomputing `BuildPlan` over the moved frontiers; §1.3
reclassifies the v3 unclassified-foreign-incomplete bucket as recoverable "resume with the
STORED plan" (`HOLDER.md:236`); the per-step `doctor schema_deploy_unrecorded` enumerates
applied steps from the stored transcript (`HOLDER.md:572-584`); and
`T-deploy-plan-hash-resume-after-step` (F14) kills after step 0 **and** step 1 and asserts
the re-run reuses `plan_hash`, preserves `step_index`es, recognizes prior receipts
exactly-once, and ends green (`HOLDER.md:766`). **Both falsifiers explicitly confirm the v3
moving-frontier reproducer no longer reproduces.** The moving-frontier defect is genuinely
resolved.

What is **not** fully implemented is the second half of the binding v3 prescription: "load
the stored transcript by `plan_hash`, **verify the embedded bytes still match the binary**,
and resume at the next step" (v3 ledger §4(1), :283-285) — unqualified across the whole
transcript. The v4 spec verifies only **not-yet-applied** steps' bytes (`HOLDER.md:235,238`;
F14's negative case tampers a not-yet-applied step, :766), and §8 concedes the already-applied
check does not fire (`HOLDER.md:833-839`). That gap is the substance of M1.

### M1 — already-applied step vs. resume binary: STANDS (BC-N1/Q3 core)

falsifier_1 concedes the moving-frontier fix and the BC-N2 fix, then lands a distinct gap
**inside the BC-N1 correctness core**. The operative §1.3 rows verify only not-yet-applied
steps: resume proceeds when "`deploy_plan[plan_hash]` present; **not-yet-applied** steps'
`sha256` match this binary's embedded bytes" (`HOLDER.md:235`), and `deploy_plan_binary_mismatch`
fires only on a **not-yet-applied** step mismatch (`HOLDER.md:238`). So an already-applied
step whose bytes differ from the resume binary is never forced into the mismatch bucket.

I verified the consequence against current source, because materiality hinges on what
`ExpectedFingerprint`/`LiveFingerprint` actually compute:

- `ExpectedFingerprint()` hashes the running binary's embedded migration/bundle **bytes**
  (`MigrationSHASet()` + each `OwnerBundle.SHA256()`), keyed by version
  (`schema_drift.go:83-99`). So a binary with different 0045 bytes (`B45 ≠ A45`) computes a
  **different** expected fingerprint.
- `LiveFingerprint` reads the **self-recorded** `schema_state` singleton — there is **no**
  recomputation from `schema_migrations`/catalog (`schema_drift.go:145-160`).
- `RecordSchemaFingerprint` UPSERTs the **running** binary's `ExpectedFingerprint()` into
  that singleton (`schema_drift.go:171-194`); `CheckSchemaDrift` compares the singleton to
  this binary's expected value (`schema_drift.go:239-274`).

So in the F14-shape reproducer (A materializes `[runtime:0045 A45, runtime:0046 A46,
owner:0021 A21]`, step 0 commits `A45` + receipt `(H,0,…,A45)` atomically, crash, resume
with B whose `0045` bytes are `B45` but `0046`/`0021` still match), the not-yet-applied check
passes, step 0's mismatch is unchecked, the cursor classifies as resume-off-stored-transcript,
B applies `0046`+`0021`, and the finalizer records **B's** fingerprint and marks `complete`.
A later B boot reads `LiveFingerprint == ExpectedFingerprint` and serves a database whose
step 0 is actually `A45` as in-sync with B. The receipt chain and `schema_migrations` still
record `A45` (provenance is preserved), but the **serve-gate fingerprint** says B — a
transcript-vs-binary mismatch that is precisely **not** forced into `deploy_plan_binary_mismatch`,
contradicting the holder's claim that the wrong-binary case is now a hard mismatch. The
tightened doctor checks receipt **presence** (all present), not stored-SHA-vs-DB-stamp, so it
stays green (`HOLDER.md:572-584`).

This is material: it is a drift-masking false-`complete` in the exact Q3/BC-N1 core, and per
the role rubric a per-step-receipt interleaving where the resumable contract is insufficient
with no stricter sub-protocol **alone forces needs_revision**. The holder's strongest rebuttal
(stored `deploy_plan` + receipts keep provenance; `plan_hash` includes all step SHAs so the
deploy identity is stable) preserves provenance but **does not** preserve the claimed
binary-compatibility contract or stop the finalizer from converting the deployment into an
apparently in-sync B deploy — falsifier_1's own analysis (`dialogue:2`, "Strongest rebuttal")
anticipates and answers it. **M1 stands unrebutted.**

### BC-N2 — pre-revoke serve window: RESOLVED

`CheckDeployActivation` is no longer gated on `revokeEmbedded`. Every deployer-aware binary —
the no-revoke landing binary included — reads `deploy_cursor` immediately after
`CheckOwnerBundleWatermark` and **before** `ApplyMigrations` and **before**
`RecordSchemaFingerprint`, in both `ConnectAndMigrate` and `ConnectAndVerify`, and halts
`awaiting_deploy` DB-untouched whenever the cursor exists and `state != complete`, regardless
of `revokeEmbedded`/watermark (§3.3a step 1, `HOLDER.md:404-421`; net invariant
`HOLDER.md:449-456`). The edge holds at the v4 pre-revoke frontier `applied_owner == 20`
(re-anchored from 19), exactly the window the C3 revoke-last ordering opens, and it does not
fire on a fresh/idle DB (absent cursor / `idle` still serves), so it neither regresses C3
(the deploy still completes; no stranded `ALTER … OWNER`) nor wedges a clean boot. F11 case
(e) asserts a no-0021 binary at `applied_owner == 20` with `state ∈ {in_progress,
step_committed, finalizing}` halts `awaiting_deploy` with `ApplyMigrations` and
`RecordSchemaFingerprint` un-called and the DB byte-identical; case (f) asserts a no-cursor /
`idle` DB still serves (`HOLDER.md:763`); `G-old-binary-refuse` is extended to prove the
pre-revoke window cannot be served. **Both falsifiers explicitly confirm BC-N2 is genuinely
fixed and decline to use it as a blocker. RESOLVED / INTACT.**

### C1 — finalization boundary: INTACT

The `finalizing` state, the idempotent finalizer (`complete` receipt guarded on `(plan_hash,
state=complete)` → fingerprint UPSERT → `finalizing → complete` last), the §1.3
resumable-finalization row, the operator pin, and F10 are preserved verbatim and re-anchored
to terminal 0021 (`HOLDER.md:210-222,237,562-570`); the finalizer crash-point table is carried
forward unchanged. Both falsifiers confirm no direct regression. The honest note (recorded
under M1, **not** as a C1 regression): the finalizer is the vehicle through which M1's harm
manifests — it writes the running binary's `ExpectedFingerprint()` and marks `complete`
without a full-transcript binary/DB-stamp check — but C1's own boundary closure (no
`complete`-with-missing-receipt window) is intact. The mechanism is right; what M1 shows is
that the finalizer can be reached with a binary that disagrees with an already-applied step.
**C1 is intact; preserve it verbatim.**

### C2 — fail-closed activation edge: INTACT (correctly re-anchored)

`CheckDeployActivation` after `CheckOwnerBundleWatermark` and before `ApplyMigrations` in both
boot paths, the typed `awaiting_deploy`/`awaiting_deploy_config` halts on the non-restartable
exit, and the forward-watermark rule are preserved (`HOLDER.md:404-441,623-671`). A doc-vs-source
accuracy note: the v3/SEED "Required = 19" framing is **stale against current `main`** —
verified `LatestOwnerBundleVersion == 20` and `RequiredOwnerBundleVersion == 20`
(`owner.go:23,35`). The v4 holder correctly keeps `RequiredOwnerBundleVersion` at its
current-`main` value **20** and does **not** advance it to the revoke ordinal 21 (0021 is
deploy-plan-only; the forward-watermark rule re-anchors to fire at `applied >= 21`). That
preserves the C2 mechanism ("Required is not advanced to the revoke ordinal") exactly; only the
ordinals moved. The BC-N2 universal edge is an **addition** to this edge, not a replacement.
**C2 intact.**

### C3 — revoke-last ownership: design INTACT; exclusion guarantee incomplete (M2)

The ownership-transfer / revoke-last mechanism is carried forward verbatim and re-anchored
0020→0021: bundle 0021 is identified by `DDLRevokeOwnerBundleVersion = 21`, special-cased out
of the owner prefix, appended as the **terminal** plan step after every runtime ownership
reconcile (each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE,
granted by 0018 in the non-revoke prefix; the committed steady state denies CREATE), and
`LatestOwnerBundleVersion` stays 20 (`HOLDER.md:340-358,458-515,723-739`). **Both falsifiers
concede the design-level answer.** The design is not regressed.

But the carry-forward prescription required "`ApplyOwnerBundles` **bounded below** [the
revoke]" (SEED C3, :122-124) — `ApplyOwnerBundles` as a whole, which contains both the pending
loop and the self-heal fallback. The v4 spec bounds only `applyPendingOwnerBundles` and adds a
generic build guard; it does not bind `ReapplyAllOwnerBundles`. That is the substance of M2.

### M2 — owner-ddl self-heal side-path applies 0021 early: STANDS (C3/R3 core)

falsifier_2 concedes the C3 design and the BC-N1/BC-N2 fixes, then lands a distinct gap that
makes the C3 "excluded from `owner-ddl apply`" claim implementation-unsafe. I verified the path
against current source:

- `ApplyOwnerBundles` runs `applyPendingOwnerBundles` and, on a cross-bundle dependency error
  (`isCrossBundleDependencyError`, the FMA-007 / #458 condition), falls into
  `ReapplyAllOwnerBundles(ctx, runner, bundles, daemonVersion)` over the **full loaded slice**
  (`owner.go:277-302`).
- `ReapplyAllOwnerBundles` calls `applyOneOwnerBundle` for **every** bundle in ascending order
  "regardless of the recorded version" — no `<= LatestOwnerBundleVersion` bound, no 0021 skip
  (`owner.go:324-349`).
- `OwnerBundles()` is the shared loader (`owner.go:196-224`), and the activation binary must
  surface the 0021 file through it for `revokeEmbedded` detection (`HOLDER.md:442-447`, anchor
  table `HOLDER.md:878`). So the slice the reapply branch iterates **includes 0021**.

Reproducer: a DB at owner frontier 20; `striatum daemon owner-ddl apply` on the activation
binary; inject the already-supported FMA-007 condition (a `<= 20` bundle fails on a missing
earlier object). `ApplyOwnerBundles` then calls `ReapplyAllOwnerBundles`, which commits `REVOKE
CREATE ON SCHEMA striatumd` (0021) **outside the deploy plan**, before any pending runtime step
and before the C3 ownership reconcile. A subsequent deployer runtime step needs CREATE that is
now gone; the spec's `deploy_create_prerequisite_missing` guard halts before DDL — a #512-class
lockout in the activation path, not merely a missed cleanup.

The v4 text binds only `applyPendingOwnerBundles` to `<= 20` and adds a **generic** build guard
("no owner bundle `>= DDLRevokeOwnerBundleVersion` is reachable through `owner-ddl apply`"),
described as a build-time assertion, not a runtime filter on the reapply loop; §8 flags the
question (`HOLDER.md:850-856`) but the body does not close the branch, and F12/`G-revoke-last`
prove only the happy deploy-plan ordering (`HOLDER.md:764`). falsifier_2's point is correct: a
generic guard is satisfiable against the normal path while leaving the reconciliation escape
hatch open. This is material — a real C3 lockout escape in current source — and stands
unrebutted. **M2 stands → needs_revision.**

## 4. What the revision must fix (exact)

A re-scaffolded revision would clear the gate only if it pins **both** items below, while
keeping BC-N1's moving-frontier fix, BC-N2, C1, the C2 edge, and the C3 revoke-last mechanism
intact and not regressing the carry-forward items.

1. **Verify the ENTIRE stored transcript against the resume binary, including already-applied
   steps and their database stamps (M1 — BC-N1/Q3 core).**
   - On every resume, validate **every** stored transcript step's `sha256` against the current
     binary's embedded bytes — **not only not-yet-applied steps**. If any stored step SHA
     differs from the binary's embedded bytes, classify `deploy_plan_binary_mismatch` and apply
     nothing.
   - For **already-applied** transcript entries, also verify the database stamps match the
     stored transcript (`schema_migrations.sha256` for runtime steps,
     `owner_bundle_meta.sha256` for owner steps). A mismatch is a legible
     transcript/database-stamp mismatch that refuses to finalize — never a silent resume.
   - Apply the **same full-transcript check before the C1 finalizer** writes `schema_state` or
     advances `finalizing → complete`, so a binary that disagrees with an already-applied step
     cannot self-record its own `ExpectedFingerprint()` as in-sync.
   - Extend F4 / F14 / F13 with an **already-applied mismatch** case: kill after step 0 commits,
     resume with a binary whose step 0 bytes differ but whose remaining steps match, and assert
     `deploy_plan_binary_mismatch`, **no** step 1 apply, **no** `RecordSchemaFingerprint`, **no**
     `complete` cursor, and a non-green doctor / typed diagnostic. Add the symmetric owner-step
     case (a completed owner prefix / pre-finalization crash, owner-bundle SHA divergence).

2. **Bind every `owner-ddl apply` route — including the FMA-007 self-heal branch — below the
   revoke ordinal (M2 — C3/R3 core).**
   - Define a single non-revoke bundle filter (exclude every bundle `>=
     DDLRevokeOwnerBundleVersion`) and apply it to **every** `owner-ddl apply` path:
     `applyPendingOwnerBundles`, **`ReapplyAllOwnerBundles`** (the cross-bundle self-heal),
     tests, and any dry-run / list surfaces. The filter must exclude the revoke regardless of
     the recorded version — not only bundles newer than the current watermark.
   - Split the embed/listing helper so "the binary embeds 0021" (for `revokeEmbedded` /
     `ExpectedFingerprint`) does **not** imply "`owner-ddl apply` can iterate 0021": the deploy
     plan and `revokeEmbedded` may see 0021; the `owner-ddl apply` bundle slice must not.
   - Add a unit/pgtest that embeds a synthetic 0021 revoke bundle, **forces the cross-bundle
     dependency self-heal path**, runs `ApplyOwnerBundles`, and asserts 0021 is **not** applied,
     `owner_bundle_meta` never records 21, and
     `has_schema_privilege('striatumd_rw','striatumd','CREATE')` remains **true**.
   - Extend F12 / `G-revoke-last` with an `owner-ddl apply` side-path case (activation binary,
     owner frontier 20, `owner-ddl apply` invoked before deploy, including the FMA-007 reapply
     branch) proving 0021 stays pending for the deploy plan and the subsequent runtime ownership
     reconcile can still run while CREATE is held.

## 5. What already cleared (carry forward — do NOT regress or re-litigate)

- **BC-N1 moving-frontier fix — resolved this cycle** (§3 above): preserve the immutable
  `deploy_plan` transcript materialized before step 0, resume off the stored transcript, the
  §1.3 stored-transcript reclassification, the transcript-enumerated doctor, and F14. (M1 is an
  *extension* of the binary-verification clause to already-applied steps, not a re-litigation of
  the moving-frontier mechanism.)
- **BC-N2 universal pre-revoke edge — resolved this cycle**: preserve the `revokeEmbedded`-
  independent `CheckDeployActivation` cursor edge at `applied_owner == 20` and F11(e)/(f) +
  extended `G-old-binary-refuse`.
- **C1 (finalization boundary) — intact**: preserve the `finalizing` state + idempotent
  finalizer + §1.3 finalizing row + F10 verbatim. (M1's fix *adds* a pre-finalizer full-transcript
  check; it does not change the finalizer's boundary semantics.)
- **C2 fail-closed activation edge — intact**: preserve `CheckDeployActivation` before
  `ApplyMigrations`, the typed halts, the forward-watermark rule (re-anchored to fire at `applied
  >= 21`), and `RequiredOwnerBundleVersion = 20` (its current-`main` value, **not** advanced to
  the revoke ordinal).
- **C3 revoke-last ownership mechanism — intact (design)**: preserve 0021 special-cased + terminal
  + the reconcile-while-CREATE-held ordering + F12 / `G-revoke-last`. (M2's fix binds the *self-heal
  branch* of `owner-ddl apply`; it does not change the deploy-plan revoke-last mechanism.)
- **Q4 — resolved** (v1/v2/v3 ledgers): plain verb now, bootstrapping paradox dispositive, run-shape
  door held by the three seams. Neither falsifier contested it.
- **Q3 mid-step resumability body — sound**: Q3-A (cursor advance + receipt in the step transaction),
  Q3-B (`in_progress` pre-marker + invalid-index reconciler), the §1.1 step taxonomy. M1 is about
  full-transcript binary verification, not the per-step schema/cursor body — preserve the body.
- **Decoupling shape — sound**: shadow-first default-OFF `STRIATUM_DEPLOY_DECOUPLED`,
  `ConnectAndVerify` preserving the P2 watermark interlock + P3 drift gate, the embed-FS-derived
  plan, the hash-chained per-step receipt.
- **Scope discipline held**: P5 (rehearse / expand-contract / fidelity tiering / clone) deferred;
  full capability revocation named as a follow-up beyond P4, not assumed; the local-first boundary
  respected.

## 6. Gate decision

The ledger verdict — not falsifier completion — clears the phase gate. **BC-N1 and BC-N2 are both
genuinely resolved, and C1/C2/C3 are carried forward intact** (both falsifiers concede each of
these). But a clearing verdict also requires **no new material challenge standing**, and two do —
each source-verified against current `main` and each inside a correctness/safety core:

- **M1**: a resume binary whose already-applied step bytes differ is not forced into
  `deploy_plan_binary_mismatch`; the C1 finalizer self-records the running binary's
  byte-derived `ExpectedFingerprint()` into the self-recorded `schema_state` and marks `complete`,
  serving a hybrid deploy as in-sync — the exact wrong-binary class the run was asked to close, and
  the under-implemented half of the binding BC-N1 prescription ("verify the embedded bytes still
  match the binary").
- **M2**: the FMA-007 self-heal branch `ReapplyAllOwnerBundles`, reachable through `owner-ddl
  apply`, re-runs every loaded bundle regardless of version and can commit 0021's `REVOKE CREATE`
  before the runtime ownership reconcile — recreating the #512-class lockout the revoke-last
  mechanism exists to prevent — because the v4 spec binds only `applyPendingOwnerBundles`.

**Verdict: `needs_revision`.** This is the gate's single allowed revision cycle, so the gate ends
**unCleared**. A clearing verdict would require a re-scaffolded revision that pins both §4 items
(full-transcript binary + DB-stamp verification on resume and before the finalizer; a single
non-revoke filter bound across **every** `owner-ddl apply` route including the self-heal branch,
with the forced-self-heal pgtest and the extended F12 / `G-revoke-last` side-path case) without
regressing BC-N1's moving-frontier fix, BC-N2, C1, the C2 edge, or the C3 revoke-last mechanism.

<sub>Adjudicator collaboration ledger for the RFC 0142 P4 falsification-gate design run, v4 /
REVISION, cycle 1. Inputs:
`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md`,
`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, `SEED.md`; context: v3
`HOLDER.md` + v3 `COLLABORATION_LEDGER_cycle_1.md` (its §4). Source verification against current
`main` (HEAD `5c5cb36a`): `go/pkg/db/owner.go:23,35,196-224,277-302,324-349`,
`go/pkg/db/schema_drift.go:83-99,145-160,171-194,239-274`. Verdict vocabulary: accept |
accept_with_findings | needs_revision | reject.</sub>

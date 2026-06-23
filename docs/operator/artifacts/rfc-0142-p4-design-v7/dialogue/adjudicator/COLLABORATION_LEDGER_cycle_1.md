---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec (v7 REVISION; the single binding cycle-1 finding M5 row-1 fresh-DB serve is GENUINELY RESOLVED — the §3.5/F18 `applied_owner` dimension is split into `{0/no authority, 1..19 shortfall, ==20, >=21}`, cell 1/`==0` serves the fresh bootstrap, F18/F18a assert both cells, cell `==20` relabeled — and M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 carry forward intact — BUT a NEW material decision-table incoherence M6 (the M5 split is NOT propagated through the `complete` rows: §3.5 rows 13/15 give the `==0` column a different outcome than `==20`, contradicting the holder's own `applied_owner`-independent A predicate, §1.3, and §4.5, so the executable F18 matrix is a false oracle for the in-sync `complete`/`applied_owner==0` cells) lands INDEPENDENTLY from BOTH falsifiers and stands unrebutted; adjudicator source-verified against the run worktree HEAD f4fcbde4, go/pkg/db diff vs 2e482077 empty)"
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
    text: "Revised P4 spec (v7) resolves the single binding cycle-1 finding M5 by a concrete `applied_owner` dimension split bound to named code sites, keeps the proactive-completeness boot-path decision table (now §3.5 over 64 cells = 16 `(cursorState × decoupledEnabled × revokeEmbedded)` rows × 4 `applied_owner` sub-columns), and carries M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 forward verbatim, re-verified against current `main` (claimed HEAD 9ceeb288; `git diff --stat 2e482077 HEAD -- go/pkg/db/` empty, so every v6 source anchor holds). M5 by SPLITTING the §3.5/F18 `applied_owner < 20` bucket into `{0/no authority, 1..19 authority shortfall, ==20, >=21}` (covering all of ℕ): W = `CheckOwnerBundleWatermark` (`owner.go:124-154`) is respecified per the live contract — `applied_owner == 0` returns nil and proceeds to A (the fresh/single-role/no-authority bootstrap exception, `owner.go:145`); `1 <= applied_owner < 20` halts `awaiting_owner_ddl` DB-untouched (`owner.go:148-150`); `applied_owner == 20` returns nil; `applied_owner >= 21` is the forward-watermark barrier (b). Because A = `CheckDeployActivation` does NOT read `applied_owner`, the `0` and `==20` columns are claimed to have IDENTICAL A-gate outcomes (HOLDER.md:471-480), so cell 1/`==0` serves the fresh-DB bring-up exactly as the source serves a fresh DB; the legacy `:399` self-record runs because NO transcript exists (Invariant B not in scope). Cell `==20`/row 1 is relabeled the inert-landing re-boot cell. F18 (`T-deploy-bootpath-decision-table`) asserts BOTH the `applied_owner == 0` serve cell AND the `1..19` halt cell, and a new F18a (`T-deploy-fresh-db-bootstrap-serves`) pins the fresh-DB serve. M3 (the hoisted step-0 `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config gate fires for EVERY cursor state in EVERY `applied_owner` column that passes W incl. the new `0` column; the no-revoke `complete` pre-`ApplyMigrations` pure-read comparison; tightened Invariant B), M4 (F16a synthetic step 2 / F16b production step 7 + forced self-heal via `isCrossBundleDependencyError`), M1 (`VerifyStoredTranscript` on resume AND finalizer step 0; typed mismatch halts), M2 (the single non-revoke filter + embed/listing split), BC-N1 (immutable `deploy_plan` transcript; resume off the stored transcript), BC-N2 (the universal non-`complete` edge at `applied_owner == 20`; the `==20` column's BC-N2 cells 5/7/9/11 verbatim), C1 (`finalizing` + idempotent finalizer + F10), C2 (`CheckDeployActivation` before `ApplyMigrations`; forward-watermark at `applied >= 21`; `RequiredOwnerBundleVersion = 20` NOT advanced), and C3 (0021 special-cased + terminal + revoke-last) are carried forward verbatim and unregressed. §4.5 re-proves Universal Invariant B in every cell; the only cells reaching the legacy `:399` writer are cells 1/`==0`, 1/`==20`, and 13-in-sync (`==20` or a degenerate `==0`)."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "Revision check (CONCEDED): the v7 holder genuinely fixes the v6 M5 row-1 reproducer in the row the v6 ledger prescribed. The `applied_owner` dimension is split into `{0/no authority, 1..19 authority shortfall, ==20, >=21}`; W returns nil for `applied_owner == 0`, halts `awaiting_owner_ddl` for `1 <= applied_owner < 20`, and row 1/`==0` is labeled SERVE-legacy / FRESH-DB BRING-UP (HOLDER.md:454-489), matching current source (`OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent; `CheckOwnerBundleWatermark` returns nil for `applied == 0` before the shortfall check, `owner.go:116-150,226-235`). F18/F18a assert both the `==0` serve case and the `1..19` halt case (HOLDER.md:723-724). No direct regression in the M3 cell (the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` gate remains A step 0 before every cursor branch incl. `complete`, HOLDER.md:352-366); the `1..19` column halts at W before A; the `==20` non-complete BC-N2 edge remains `awaiting_deploy`; `RequiredOwnerBundleVersion` is still 20; the `>=21` forward-watermark column is unaltered (HOLDER.md:487-504,590-614; `owner.go:23,35`). M4/M1/M2/BC-N1/C1/C2/C3 carried forward in text with no new source contradiction from this lens. NEW MATERIAL CHALLENGE (the split is not propagated correctly through the `complete` / `applied_owner == 0` rows): the holder's structural claim is that W passes both `applied_owner == 0` and `== 20`, and A does NOT read `applied_owner`, so the `0` and `==20` columns have IDENTICAL A-gate behavior once W has passed (HOLDER.md:471-480); §1.3 says the same from the cursor side — a `complete` cursor whose stored plan and fingerprint match the binary serves verify-only when decoupled, independent of the owner-watermark bucket (HOLDER.md:223-225). But §3.5 violates that in the `complete` rows for `applied_owner == 0`: row 13 (`complete`, flag off, no-revoke) gives `==0` the outcome `awaiting_deploy` while `==20` is 'SERVE-legacy if in-sync, else `awaiting_deploy`' (HOLDER.md:501), and the §4.5 proof immediately below admits a 'cell 13 / `==0`, degenerate in-sync' path where A3 proves in-sync and the legacy `:399` rewrite is idempotent (HOLDER.md:525-528) — so the table outcome, the A predicate, and the proof disagree; row 15 (`complete`, decoupled on, no-revoke) gives `==0` an unconditional `awaiting_deploy` while `==20` is 'SERVE-verify if in-sync, else `awaiting_deploy`' (HOLDER.md:503), but A3 on the decoupled path serves verify-only whenever `plan_hash == expected` and `LiveFingerprint == ExpectedFingerprint`, with no `applied_owner` input (HOLDER.md:359-360,370-374). Concrete refutation (the executable F18 cell): cursorState=complete, decoupledEnabled=true, revokeEmbedded=false, applied_owner=0, `deploy_plan[plan_hash]` present, `cursor.plan_hash == expected`, `LiveFingerprint(recorded) == ExpectedFingerprint()`, `owner_bundle_meta` absent — W returns nil (the `applied == 0` exception, `owner.go:145`), A takes the `complete` + decoupled branch and serves verify-only because plan-hash and fingerprint are in sync (HOLDER.md:370-374), which is exactly what §1.3 says (HOLDER.md:223-225); but §3.5 row 15/`==0` requires `awaiting_deploy`, and F18 is defined as a parametrized matrix asserting the exact §3.5 outcome for every cell (HOLDER.md:723). So F18 is either a false oracle for this cell, or the implementation must add an unstated `applied_owner == 0 && cursorState == complete` halt that contradicts both the stated A predicate and the 'identical A behavior' claim. The same inconsistency exists in the legacy `complete` row (13/`==0`): the F18 spy list permits `RecordSchemaFingerprint` only in row 13/`==20`-in-sync, not the admitted row 13/`==0`-in-sync (HOLDER.md:723) — again the table or the oracle is wrong. The holder's anticipatable rebuttal (that `complete + applied_owner == 0` is unreachable in the final activation path because a completed deploy applies owner bundles, so the cell is a conservative halt for a corrupt shape) is not enough for the written spec: §3.5 does not mark the cell impossible/corrupt; the specified A predicate has no way to enforce the conservative halt because it 'does NOT read `applied_owner`'; and §4.5 itself admits the row 13/`==0` in-sync subcase. Required repair (choose one and propagate everywhere): (1) make the `==0` column mirror the `==20` column anywhere W passes and A is owner-watermark independent, so rows 13 and 15 become conditional 'serve if in-sync, else `awaiting_deploy`' (and add the 13/`==0` idempotent subcase to §4.5 + the F18 spy list); or (2) explicitly define `complete + applied_owner == 0` as an inconsistent state, add the W/A guard that detects it before serving, drop the 'identical A behavior' claim for the complete rows, and update F18/F18a to assert that typed halt. As written the decision table is not executable. The revision should not clear until the `complete` / `applied_owner == 0` cells are made coherent."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "Revision check (CONCEDED): the direct v6 M5 reproducer is addressed — the `applied_owner` dimension is split into `{0/no authority, 1..19 shortfall, ==20, >=21}`, W serves `applied_owner == 0` and halts only `1 <= applied_owner < 20`, and row 1/`==0` is labeled the fresh-DB bring-up cell instead of mislabeling row 1/`==20` (HOLDER.md:455-489), matching the live watermark contract (`owner.go:116-150,226-235`); F18/F18a name both branches (HOLDER.md:723-724). No DIRECT regression in the nine v6-cleared carry-forwards: M3 (the config gate stays A step 0 before every cursor branch incl. `complete`; the no-revoke `complete` residual stays a pre-`ApplyMigrations` pure-read comparison, HOLDER.md:352-393); M4 (F16a synthetic + F16b production incl. the forced FMA-007 self-heal through `isCrossBundleDependencyError`, HOLDER.md:316-327,721); M1 (`VerifyStoredTranscript` on resume and finalizer step 0 with typed mismatch halts, HOLDER.md:409-430,720); M2/C3 (`OwnerDDLApplyBundles()`/`isNonRevokeBundle` the only apply slice, `OwnerBundles()` the full loader for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable`, 0021 terminal deploy-only, HOLDER.md:286-327,643-675); BC-N1/BC-N2/C1/C2 (immutable `deploy_plan`, universal non-complete cursor halt at `applied_owner == 20`, `finalizing` finalizer, `Required = 20`, `>=21` forward-watermark rule, HOLDER.md:183-242,352-393,590-614). NEW MATERIAL CHALLENGE (F18 is false for `complete` / `applied_owner == 0`): the holder claims the split is executable over every `cursorState × decoupledEnabled × revokeEmbedded × applied_owner` cell, and that once W passes A does not read `applied_owner` so the `0` and `==20` columns have identical A-gate behavior (HOLDER.md:438-480); §1.3 says a `complete` cursor whose stored plan and fingerprint match serves verify-only when decoupled, or serves legacy/no-op when no-revoke and already in sync (HOLDER.md:223-225); F18 requires the parametrized matrix to assert the exact §3.5 outcome for all 64 cells (HOLDER.md:723). The §3.5 table contradicts those claims in the complete/no-revoke `applied_owner == 0` cells: row 13 gives `==0` `awaiting_deploy` while `==20` is 'SERVE-legacy if in-sync, else `awaiting_deploy`' (HOLDER.md:501) and §4.5 admits the degenerate 13/`==0` in-sync idempotent `:399` rewrite (HOLDER.md:525-528); row 15 gives `==0` unconditional `awaiting_deploy` while `==20` is 'SERVE-verify if in-sync, else `awaiting_deploy`' (HOLDER.md:503), but A3 decoupled serves verify-only when `plan_hash == expected` and `LiveFingerprint == ExpectedFingerprint`, with no `applied_owner` input (HOLDER.md:359-374); the F18 spy oracle allows `RecordSchemaFingerprint` only in 1/`==0`, 1/`==20`, and 13/`==20`-in-sync while §4.5 allows the degenerate 13/`==0` rewrite (HOLDER.md:682-701,723) — the test oracle and the invariant proof do not agree. Concrete refutation: cursorState=complete, decoupledEnabled=true, revokeEmbedded=false, applied_owner=0, plan present, `cursor.plan_hash == expected`, `LiveFingerprint(recorded) == ExpectedFingerprint()`, `owner_bundle_meta` absent — W returns nil (`owner.go:145`), A returns nil (serve verify-only, HOLDER.md:370-374), §1.3 says serve; but §3.5 row 15/`==0` requires `awaiting_deploy` and F18 must assert it, so either F18 is a false oracle or the build smuggles in an unstated owner-watermark-dependent complete-cursor guard. The holder's 'complete + applied_owner == 0 is a corrupt / unreachable shape' rebuttal does not rescue the written spec: §3.5 promises exact executable outcomes for all 64 cells and does not mark the cell impossible, A is explicitly owner-watermark independent so the specified predicate cannot produce the conservative halt, and §4.5 simultaneously admits the degenerate in-sync write. Carry-forward impact: this does NOT re-open the M3 mutate+self-record bypass (revoke-embedding + flag-OFF cells still halt at A0; no pending-change transcript passes A3), and does NOT weaken M2/C3, the `==20` BC-N2 edge, `Required = 20`, or the `>=21` forward rule — but it IS material because F18 is the carry-forward proof harness for M3, BC-N2, C2, and M5 together: a build can implement the §3.3a predicate and fail the table, or implement the table and smuggle an unstated guard. Required repair (choose one, propagate everywhere): (1) mirror `==20` anywhere W passes and A is owner-watermark independent — rows 13 and 15 `==0` become conditional 'serve if in-sync, else `awaiting_deploy`', §4.5 and the F18 spy list include the 13/`==0` idempotent subcase; or (2) classify `complete + applied_owner == 0` as inconsistent — add the W/A guard that detects it before serving, remove the 'identical A behavior' claim in the complete rows, and make F18 assert the typed halt. Until one is specified the v7 table still has a material owner-watermark regression; the revision should not clear."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "REVISION cycle v7 (the gate's single allowed revision cycle). The single binding cycle-1 finding M5 is GENUINELY RESOLVED in the row the v6 ledger §4 prescribed, and BOTH falsifiers independently concede it: §3.5 and F18 split the `applied_owner` dimension into `{0/no authority, 1..19 authority shortfall, ==20, >=21}` (covering all of ℕ); W returns nil for `applied_owner == 0` (the fresh/single-role/no-authority bootstrap) before the shortfall check and halts `awaiting_owner_ddl` only for `1 <= applied_owner < 20`; cell 1/`==0` (no-revoke, flag off, cursorState=none, no transcript) SERVES-legacy the fresh-DB bring-up — `ApplyMigrations` + the legacy `:399` self-record run, Invariant B not in scope; F18 asserts BOTH the `==0` serve cell and the `1..19` halt cell, F18a (`T-deploy-fresh-db-bootstrap-serves`) pins the fresh-DB serve; and cell `==20` is relabeled the inert-landing re-boot cell. ADJUDICATOR SOURCE-VERIFICATION this cycle (direct read of the run worktree, HEAD f4fcbde4; `git diff --stat 2e482077 HEAD -- go/pkg/db/` empty, so every v6 db anchor holds — the holder cites HEAD 9ceeb288, an immaterial branch-advance drift since the dialogue artifacts landed, and the go/pkg/db source is byte-identical): `go/pkg/db/owner.go:145` `if applied == 0 { return nil }` returns BEFORE the `if applied < RequiredOwnerBundleVersion` shortfall at `:148-150`, with the function comment at `:116-123`+`:140-143` ('a 0-watermark database … is treated as the bootstrap/single-role case and NOT halted. Only a database that HAS an authority schema (applied >= 1) but lags the required frontier is a genuine shortfall'); `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent; `RequiredOwnerBundleVersion = LatestOwnerBundleVersion = 20` unchanged. The M5 ROW-1 fix is real and source-accurate. The nine carry-forwards M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are carried forward verbatim and INTACT — both falsifiers concede no regression, and the M5 split does not weaken the M3 config gate (cells 2/6/10/14 still halt `awaiting_deploy_config` at A0 in every column that passes W, incl. the new `0` column — a revoke-embedding binary on a fresh DB with the flag OFF still halts), does not regress the BC-N2 `applied_owner == 20` edge (cells 5/7/9/11 verbatim), and does not advance `Required` or alter the watermark (the `>=21` forward rule unchanged). BUT a clearing verdict ALSO requires (a) the proactive-completeness boot-path decision table COMPLETE — every cell with a specified guard/outcome, an EXECUTABLE F18 oracle that matches the §3.3a A predicate, Invariant B proven in every cell, AND the legitimate fresh-DB cells still serving — and (b) no new material challenge standing unrebutted. BOTH fail on the SAME new defect (M6). BOTH falsifiers, from independent lenses (falsifier_1 decoupling-boundary/decision-table; falsifier_2 carry-forward/regression), CONVERGE on a new material decision-table INCOHERENCE: the M5 `applied_owner` split was applied coherently to row 1 (cursorState=none) but NOT propagated through the `complete` rows. §3.5 row 13 gives the `==0` column `awaiting_deploy` while `==20` is 'SERVE-legacy if in-sync, else `awaiting_deploy`' (HOLDER.md:501), and row 15 gives `==0` an unconditional `awaiting_deploy` while `==20` is 'SERVE-verify if in-sync, else `awaiting_deploy`' (HOLDER.md:503) — but the holder's OWN §3.3a A predicate does NOT read `applied_owner` (HOLDER.md:359-360,471-475) and decides solely on `plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint` (HOLDER.md:370-381), and §1.3 says a `complete`/in-sync cursor serves regardless of the owner-watermark bucket (HOLDER.md:223-225). ADJUDICATOR SOURCE-VERIFICATION: `LiveFingerprint` reads the recorded `schema_state.fingerprint` singleton (`schema_drift.go:145-161`), NOT a recompute, and `RecordSchemaFingerprint` writes the binary's `ExpectedFingerprint()` (`schema_drift.go:171-195`) — both ORTHOGONAL to `owner_bundle_meta`/`applied_owner` (separate tables). So an in-sync `complete`/`applied_owner==0` cell (cursor=complete, plan_hash match, fingerprint match, `owner_bundle_meta` absent) IS constructible, and a parametrized F18 over all 64 cells WILL construct it; for that cell A returns nil (serve verify-only on row 15, serve legacy no-op on row 13), but §3.5 says `awaiting_deploy`. Therefore F18 — the required executable matrix asserting the EXACT §3.5 outcome for every cell — is a FALSE ORACLE for the in-sync `complete`/`==0` cells (rows 13 and 15), OR the implementation must smuggle in an unstated `applied_owner`-dependent complete-cursor guard, directly contradicting the holder's own 'A does not read `applied_owner`' / 'identical A behavior for the `0` and `==20` columns' claim. The holder's own §4.5 compounds the contradiction: it ADMITS the 'degenerate cell 13/`==0` in-sync' idempotent `:399` rewrite (HOLDER.md:525-528) while the F18 spy list permits `RecordSchemaFingerprint` ONLY in cells 1/`==0`, 1/`==20`, and 13/`==20`-in-sync (HOLDER.md:723) — omitting 13/`==0`-in-sync — so §4.5 and the F18 oracle disagree. This is MATERIAL (it requires a spec edit — propagate the split through the complete rows per one of two coherent contracts the falsifiers specify — and exposes a real correctness defect in F18, the executable named requirement and the carry-forward proof harness for M3/BC-N2/C2/M5). It is precisely the SEED's warned-of M6: 'A re-scaffolded revision that pins M5 but leaves any further unaudited cell open will spawn an M6 — close the class.' The class is NOT closed: the split is coherent at cursorState=none but incoherent at cursorState=complete. The holder's anticipatable rebuttal (that `complete + applied_owner == 0` is an unreachable/corrupt shape, so the `==0` halts are conservative) is preempted by BOTH falsifiers and does not rescue the written spec: §3.5 promises exact executable outcomes for ALL 64 cells and does not mark the cell impossible; the specified A predicate cannot produce the conservative halt because it does not read `applied_owner`; and §4.5 itself enumerates the in-sync `==0` subcase as reachable. It STANDS UNREBUTTED — this cycle carries no holder rebuttal artifact; the falsifiers are the last word and the spec text (HOLDER.md:501,503,525-528,723) affirms the incoherent table. Note M6 is NOT a SAFETY hole (Invariant B holds in both readings — the only `==0` legacy write that could fire is an idempotent rewrite of the identical fingerprint, and no fingerprint is ever written around a transcript carrying a pending change), and it does NOT regress any carry-forward; it is a decision-table EXECUTABILITY / COMPLETENESS failure that fails the explicit clearing precondition 'the boot-path decision table is complete' + 'no new material challenge standing unrebutted'. Because the decision table is not executable/complete in the `complete`/`applied_owner==0` cells and a new material challenge stands unrebutted, the gate does NOT clear. This is the single allowed revision cycle for v7, so the gate ends unCleared — judged exactly on the merits, not graded on the consequence; §4 records the single load-bearing fix (propagate the M5 split through the `complete` rows under one coherent contract, with §4.5 + F18 made consistent), and §5 records what cleared and must not be regressed (M5 row-1 + M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 — all genuinely resolved/intact this cycle)."
findings:
  - id: M5
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED (row 1) — the single binding cycle-1 finding (the §3.5/F18 owner-watermark dimension collapse) is genuinely fixed in the row the v6 ledger §4 prescribed; BOTH falsifiers explicitly concede the row-1 repair. The `applied_owner < 20` bucket is split into `{0/no authority, 1..19 authority shortfall, ==20, >=21}` covering all of ℕ; W = `CheckOwnerBundleWatermark` is respecified per the live contract — `applied_owner == 0` returns nil and proceeds to A (the fresh/single-role/no-authority bootstrap exception), `1 <= applied_owner < 20` halts `awaiting_owner_ddl` DB-untouched, `==20` returns nil, `>=21` is the forward-watermark barrier (b) (HOLDER.md:454-489). Cell 1/`==0` (no-revoke, flag off, cursorState=none, NO transcript) SERVES-legacy the fresh-DB bring-up — `ApplyMigrations` + the legacy `:399` self-record run, Invariant B not in scope; cell 1/`==20` is relabeled the inert-landing re-boot cell. F18 asserts BOTH the `==0` serve cell and the `1..19` halt cell; F18a (`T-deploy-fresh-db-bootstrap-serves`) pins the fresh-DB serve and the `1..19` halt (HOLDER.md:723-724). ADJUDICATOR SOURCE-VERIFICATION (run worktree, HEAD f4fcbde4; go/pkg/db diff vs 2e482077 empty): `owner.go:145` `if applied == 0 { return nil }` precedes the `:148-150` shortfall; `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent; `RequiredOwnerBundleVersion = 20`. The row-1 fix is real and source-accurate; the relabel is correct (the genuine fresh no-authority DB is `applied_owner == 0`, not `==20`). HOWEVER, the split is NOT propagated coherently through the `complete` rows — see M6. **M5 RESOLVED at row 1; the un-propagated `complete`/`==0` cells are tracked as the new finding M6.**"
    affected_invariants: ["boot-path decision table CORRECT in the owner-watermark dimension at cursorState=none (the fresh-DB serve cell)", "fresh / single-role / no-authority bootstrap (applied_owner == 0) still serves on the legacy path", "the F18 dual-cell assertion (the applied_owner == 0 serve cell AND the 1..19 halt cell)", "cell ==20 no longer mislabeled the fresh-DB bring-up cell"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M6
    severity: high
    posture: design
    status: open
    challenge: "OPEN — NEW material challenge landed INDEPENDENTLY by BOTH falsifiers (falsifier_1 decoupling-boundary/decision-table lens; falsifier_2 carry-forward/regression lens), source-verified against the run worktree, standing unrebutted. The M5 `applied_owner` split is coherent at cursorState=none (row 1) but is NOT propagated through the `complete` rows. The holder's central structural claim is that W passes both `applied_owner == 0` and `== 20`, A does NOT read `applied_owner`, so the `0` and `==20` columns have IDENTICAL A-gate behavior once W has passed (HOLDER.md:359-360,471-475); §1.3 says a `complete` cursor whose stored plan and fingerprint match the binary serves verify-only when decoupled (or serves legacy/no-op when no-revoke and already in sync), independent of the owner-watermark bucket (HOLDER.md:223-225). But §3.5 contradicts those claims: row 13 (`complete`, flag off, no-revoke) gives the `==0` column `awaiting_deploy` while `==20` is 'SERVE-legacy if in-sync, else `awaiting_deploy`' (HOLDER.md:501); row 15 (`complete`, decoupled on, no-revoke) gives `==0` an unconditional `awaiting_deploy` while `==20` is 'SERVE-verify if in-sync, else `awaiting_deploy`' (HOLDER.md:503). The A3 predicate decides solely on `plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint` (HOLDER.md:370-381) — with NO `applied_owner` input — so for the SAME in-sync facts A returns the SAME outcome regardless of the bucket; the table's differing `==0`/`==20` outcomes in rows 13/15 cannot be produced by the specified predicate. ADJUDICATOR SOURCE-VERIFICATION (this cycle): `LiveFingerprint` reads the recorded `striatumd.schema_state.fingerprint` singleton (`go/pkg/db/schema_drift.go:145-161`), NOT a recompute of the live schema, and `RecordSchemaFingerprint` UPSERTs the running binary's `ExpectedFingerprint()` (`schema_drift.go:171-195`) — both ORTHOGONAL to `owner_bundle_meta`/`applied_owner` (separate tables). So an in-sync `complete`/`applied_owner==0` cell (cursor=complete, `deploy_plan[plan_hash]` present, `cursor.plan_hash == expected`, `schema_state.fingerprint == ExpectedFingerprint()`, `owner_bundle_meta` absent) IS constructible, and a parametrized F18 over all 64 cells WILL construct it. Concrete refutation (both falsifiers' identical F18 cell): for that cell W returns nil (`owner.go:145`), A returns nil (serve verify-only on row 15; serve legacy no-op + idempotent `:399` rewrite on row 13, HOLDER.md:370-381), and §1.3 says serve — but §3.5 rows 15/`==0` and 13/`==0` say `awaiting_deploy`, and F18 must assert the exact §3.5 outcome (HOLDER.md:723). Therefore F18 is a FALSE ORACLE for these cells, OR the build must smuggle in an unstated `applied_owner`-dependent complete-cursor guard, contradicting the holder's own 'A does not read `applied_owner`'/'identical A behavior' claim. The holder's OWN §4.5 compounds it: it ADMITS the 'degenerate cell 13/`==0` in-sync' idempotent `:399` rewrite (HOLDER.md:525-528) while the F18 spy list permits `RecordSchemaFingerprint` ONLY in 1/`==0`, 1/`==20`, and 13/`==20`-in-sync (HOLDER.md:723) — omitting 13/`==0`-in-sync — so §4.5 and the F18 oracle disagree. The holder's anticipatable rebuttal (that `complete + applied_owner == 0` is an unreachable/corrupt shape, so the `==0` halts are conservative) is preempted by BOTH falsifiers and does not rescue the written spec: §3.5 promises exact executable outcomes for ALL 64 cells and does not mark the cell impossible; the specified A predicate cannot produce the conservative halt because it does not read `applied_owner`; and §4.5 itself enumerates the in-sync `==0` subcase as reachable. This is the SEED's warned-of M6 ('a revision that pins M5 but leaves any further unaudited cell open will spawn an M6 — close the class'); the class is NOT closed. NOT a SAFETY hole (Invariant B holds in both readings — only idempotent rewrites of the identical fingerprint, and no fingerprint is ever written around a transcript carrying a pending change) and NOT a regression of any carry-forward (M3 cells 2/6/10/14 still halt at A0; the BC-N2 `==20` edge, M2/C3, `Required = 20`, and the `>=21` forward rule are intact) — but MATERIAL (requires a spec edit and exposes a real defect in F18, the executable named requirement and the carry-forward proof harness for M3/BC-N2/C2/M5) and STANDS UNREBUTTED (no holder rebuttal artifact; the spec text affirms the incoherent table) → needs_revision. Required fix (choose ONE coherent contract and propagate it everywhere — §3.5, §3.3a, §1.3, §4.5, F18/F18a): (1) MIRROR `==20` anywhere W passes and A is owner-watermark independent — rows 13 and 15 `==0` become conditional 'serve if in-sync, else `awaiting_deploy`', and §4.5 + the F18 spy list include the 13/`==0` idempotent subcase; or (2) CLASSIFY `complete + applied_owner == 0` as inconsistent — add the explicit W/A guard that detects it before serving, remove the 'identical A behavior' claim in the complete rows, and make F18/F18a assert the typed halt. Until one is specified the executable decision table is not coherent."
    affected_invariants: ["boot-path decision table COMPLETE and EXECUTABLE in the owner-watermark dimension at cursorState=complete (the §3.5 table, the §3.3a A predicate, §1.3, and §4.5 must agree)", "the F18 matrix oracle matches the applied_owner-independent A predicate for the complete/applied_owner==0 cells", "§4.5 Universal Invariant B proof and the F18 spy list agree on which cells reach the legacy :399 writer", "the M5 split propagated coherently through ALL cursor rows, not only cursorState=none"]
    source_refs: ["dialogue:2", "dialogue:3"]
  - id: M3
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm no direct regression. The `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config halt remains HOISTED to step 0 of `CheckDeployActivation`, before any cursor-state branch, so it fires for EVERY cursor state incl. `complete` (barrier a, HOLDER.md:352-366) and in EVERY `applied_owner` column that passes W — incl. the new `0` column: cell 2/`==0` (a revoke-embedding binary on a fresh DB with the flag OFF) still halts `awaiting_deploy_config`, preserving the asymmetry that the M5 serve cell is no-revoke ONLY. The no-revoke `complete` residual remains a pre-`ApplyMigrations`, DB-untouched PURE-READ comparison (`ExpectedFingerprint() == LiveFingerprint(recorded)` AND `cursor.plan_hash == expected`, HOLDER.md:376-381). The tightened Universal Invariant B and F17/F11(g)/F18 carry forward. The M5 split re-buckets ONLY the `applied_owner` dimension at W; it does not weaken the M3 gate at A. **M3 INTACT.** (Note: the M6 incoherence is in the no-revoke `complete`/`==0` cells, NOT the revoke-embedding M3 cells, and does NOT re-open the M3 mutate+self-record bypass — both falsifiers confirm.)"
    affected_invariants: ["hoisted step-0 revokeEmbedded && !decoupledEnabled → awaiting_deploy_config config gate (every cursor state, every applied_owner column that passes W)", "no revoke-embedding binary ever reaches the legacy connection.go:399 writer", "the no-revoke complete pre-ApplyMigrations pure-read comparison", "the preserved asymmetry: the M5 serve cell is no-revoke only (cell 2/==0 still halts)"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M4
    severity: medium
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. F16a (`TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, rollout step 2) drives the exclusion filter through a synthetic bundle list / injectable-FS hook without asserting production `OwnerBundles()` contains 0021; F16b (`TestOwnerDDLApplyExcludesProductionRevokeBundle`, rollout step 7 after 0021 is authored) asserts the production embed/listing split and houses the forced FMA-007 self-heal pgtest reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError` (`owner.go:367-374`) (HOLDER.md:316-327,721). M5 does not touch F16. **M4 INTACT.**"
    affected_invariants: ["F16a synthetic phase builds green before 0021 exists", "F16b production phase + the forced self-heal via isCrossBundleDependencyError", "the M2 filter unchanged by the F16 staging"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: M1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers find no regression. `VerifyStoredTranscript(plan_hash)` (§3.4a) checks the ENTIRE stored transcript `sha256` against the binary's embedded bytes ⇒ `deploy_plan_binary_mismatch`; already-applied entries verify the DB stamps ⇒ `deploy_plan_db_stamp_mismatch`; the SAME verifier runs on every resume AND as finalizer step 0 (HOLDER.md:409-430,720). M5 concerns the `applied_owner` dimension at W; it does not touch the deployer verifier. F15 + F14 carry forward. **M1 INTACT.**"
    affected_invariants: ["full stored-transcript byte verification on resume AND before the finalizer", "typed deploy_plan_binary_mismatch / deploy_plan_db_stamp_mismatch halts", "the deployer finalizer arm gated by VerifyStoredTranscript step 0"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The single non-revoke filter `isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)` + the split loader `OwnerDDLApplyBundles()` bind across every `owner-ddl apply` route incl. the FMA-007 self-heal `ReapplyAllOwnerBundles` (in-loop guards) + the nil-fallback split; `OwnerBundles()` (full, includes 0021) is kept ONLY for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable` (HOLDER.md:286-327,643-675). M5 does not touch the filter. **M2 INTACT.**"
    affected_invariants: ["0021 unreachable through every owner-ddl apply route incl. the FMA-007 self-heal", "embed/listing split", "0021 committed only as the terminal deploy step"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: BC-N1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The runtime-owned `deploy_plan` table (migration 0044) storing the IMMUTABLE ordered transcript keyed by `plan_hash`, materialized in the same tx that sets `deploy_cursor → in_progress(0)` BEFORE step 0; resume loads `deploy_plan[cursor.plan_hash]` and never recomputes `BuildPlan` over the moved frontier; §1.3 + the transcript-enumerated doctor + F14 (HOLDER.md:183-242). M5 touches only the orthogonal `applied_owner` dimension at W. **BC-N1 INTACT.**"
    affected_invariants: ["BC-N1 immutable plan identity across resume", "stable (plan_hash, step_index) receipt key", "resume off the stored transcript, never a recomputed BuildPlan"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: BC-N2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm M5 concerns the ORTHOGONAL owner-watermark `applied_owner` dimension at W (`CheckOwnerBundleWatermark`), NOT the BC-N2 `deploy_cursor` edge at A. The universal incomplete-deploy edge still halts `awaiting_deploy` DB-untouched on every non-`complete` cursor at `applied_owner == 20` (the `==20` column's BC-N2 cells 5/7/9/11 are verbatim from v6, HOLDER.md:487-504); F11(e)/(f) + the extended `G-old-binary-refuse` carry forward. The M5 split adds the `0` and `1..19` columns and leaves the `==20` BC-N2 cells unchanged. **BC-N2 INTACT.** (M6's incoherence is in the `complete`/`==0` cells, not the non-complete BC-N2 edge.)"
    affected_invariants: ["BC-N2 universal pre-revoke deploy_cursor-authoritative edge at applied_owner == 20", "incomplete (non-complete) cursor never serves", "the M5 split does not regress the ==20 BC-N2 cells"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C1
    severity: critical
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The `finalizing` state + the idempotent finalizer (VerifyStoredTranscript step 0 → guarded `complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST) + the §1.3 resumable-finalization row + F10 (HOLDER.md:204-212,409-422). M5 re-buckets the W gate that runs BEFORE both the finalizer and the legacy writer; it touches neither. **C1 INTACT.**"
    affected_invariants: ["finalization boundary (no complete with missing receipt/fingerprint)", "idempotent finalizer gated by VerifyStoredTranscript step 0", "section 1.3 disambiguation"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: C2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm. `CheckDeployActivation` before `ApplyMigrations` in both boot paths (mutating nothing); the typed `awaiting_deploy`/`awaiting_deploy_config` halts; the forward-watermark rule firing at `applied_owner >= 21` (barrier b, the `>=21` column); and `RequiredOwnerBundleVersion = 20` (`owner.go:35`) NOT advanced to the revoke ordinal (HOLDER.md:487-504,590-614). The M5 split adds the `0` and `1..19` columns BELOW the frontier; it does NOT advance `Required`, alter the watermark, or change the `applied >= 21` forward rule. **C2 INTACT.**"
    affected_invariants: ["fail-closed CheckDeployActivation edge before ApplyMigrations", "forward-watermark rule at applied >= 21 (barrier b)", "RequiredOwnerBundleVersion KEPT at 20, not advanced"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C3
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. Bundle 0021 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) identified by `DDLRevokeOwnerBundleVersion = 21`, special-cased out of the owner prefix, applied as the TERMINAL plan step after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE granted by 0018; the steady state denies CREATE); `LatestOwnerBundleVersion`/`Required` stay 20; F12/`G-revoke-last` (HOLDER.md:286-327,643-675). M5 does not touch the revoke mechanism. **C3 INTACT.**"
    affected_invariants: ["two-role owner/runtime boundary", "revoke-last terminal ordering", "reconcile-while-CREATE-held", "activation deploy can complete"]
    source_refs: ["dialogue:1", "dialogue:3"]
branches:
  design: blocked
---

# COLLABORATION LEDGER — RFC 0142 P4 design run (v7 / REVISION), cycle 1

author: adjudicator-author-001

**Verdict:** `needs_revision`

> Scope of this adjudication: the curated dialogue trajectory only — the **revised
> (v7)** Holder's `HOLDER.md` P4 implementation spec and the two falsifiers'
> `FALSIFIER.md` challenges — plus the `SEED.md` charter, with the **v6** `HOLDER.md`
> and the **v6** collaboration ledger (its M5 finding + §4 "What the revision must fix"
> + §5 "What already cleared") as context for what the revision had to fix. RFC 0142 is
> accepted (D258); this judges the **P4 implementation shape**, not the five-layer
> design. A clearing verdict would be `accept` or `accept_with_findings`; this is
> neither. This is the gate's **single allowed revision cycle**, so the gate ends
> **unCleared** — judged exactly on the merits, not graded on the consequence.

## 1. Decision

`needs_revision`. The revision is real, well-anchored, and it **genuinely resolves the
single binding cycle-1 finding M5 at row 1** — both falsifiers independently concede the
row-1 repair — and it **carries M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 forward
intact** (both falsifiers concede no regression). **But** a clearing verdict also requires
**(a) the proactive-completeness boot-path decision table to be COMPLETE and EXECUTABLE**
(an F18 oracle that matches the §3.3a A predicate, Invariant B proven in every cell, and
the legitimate fresh-DB cells still serving) **and (b) no new material challenge standing
unrebutted** — and **both fail on the same new defect (M6)**: the M5 `applied_owner` split
is coherent at `cursorState == none` (row 1) but is **not propagated through the
`complete` rows**, so §3.5 contradicts the holder's own A predicate, §1.3, and §4.5, and
the executable F18 matrix is a **false oracle** for the in-sync `complete` /
`applied_owner == 0` cells. **Both** falsifiers, from independent lenses, **converged** on
this, source-verified, and it stands unrebutted.

- **M5 (the §3.5/F18 owner-watermark dimension collapse) — RESOLVED at row 1.** The
  `applied_owner < 20` bucket is split into `{0/no authority, 1..19 authority shortfall,
  ==20, >=21}` (covering all of ℕ); W = `CheckOwnerBundleWatermark` is respecified per the
  live contract — `applied_owner == 0` returns nil and proceeds to A, `1 <= applied_owner
  < 20` halts `awaiting_owner_ddl`, `==20` returns nil, `>=21` is the forward-watermark
  barrier (b) (`HOLDER.md:454-489`). Cell 1/`==0` (no-revoke, flag off, `cursorState=none`,
  no transcript) **SERVES-legacy** the fresh-DB bring-up; cell 1/`==20` is relabeled the
  inert-landing re-boot cell; F18 asserts both the `==0` serve cell and the `1..19` halt
  cell, and F18a (`T-deploy-fresh-db-bootstrap-serves`) pins the fresh-DB serve
  (`HOLDER.md:723-724`). I source-verified the row-1 fix against the run worktree (HEAD
  `f4fcbde4`; `git diff --stat 2e482077 HEAD -- go/pkg/db/` **empty**, so every v6 db
  anchor holds): `go/pkg/db/owner.go:145` `if applied == 0 { return nil }` precedes the
  `:148-150` shortfall, with the comment block `:116-123`+`:140-143`; `OwnerBundleVersion`
  returns 0 when `owner_bundle_meta` is absent; `RequiredOwnerBundleVersion = 20`. **The
  row-1 repair and the relabel are correct.** (The holder cites HEAD `9ceeb288`; the
  worktree HEAD is `f4fcbde4` — an immaterial branch-advance since the dialogue artifacts
  landed; the `go/pkg/db` source is byte-identical, so all anchors hold.)
- **M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, C3 — INTACT.** Carried forward verbatim; both
  falsifiers concede no regression. The M5 split re-buckets ONLY the `applied_owner`
  dimension at W: the M3 config gate still halts `awaiting_deploy_config` at A0 for cells
  2/6/10/14 in EVERY column that passes W (incl. the new `0` column — a revoke-embedding
  binary on a fresh DB with the flag OFF still halts, preserving the asymmetry that the M5
  serve cell is no-revoke only); the BC-N2 `applied_owner == 20` edge (cells 5/7/9/11) is
  verbatim; `RequiredOwnerBundleVersion = 20` and the `applied >= 21` forward rule are
  unchanged.

**But a clearing verdict also requires the decision table COMPLETE/EXECUTABLE and no new
material challenge standing.** Both fail on **M6** (the SEED's anticipated next finding):

- **M6 — the M5 split is not propagated through the `complete` rows; F18 is a false
  oracle for the in-sync `complete` / `applied_owner == 0` cells.** The holder's central
  structural claim is that A does **not** read `applied_owner`, so the `0` and `==20`
  columns have **identical A-gate behavior** once W has passed (`HOLDER.md:359-360,
  471-475`); §1.3 says a `complete`/in-sync cursor serves regardless of the owner-watermark
  bucket (`HOLDER.md:223-225`). But §3.5 gives the `complete` rows **different** outcomes
  for `==0` vs `==20`: row 13 → `==0` is `awaiting_deploy` while `==20` is "SERVE-legacy if
  in-sync, else `awaiting_deploy`" (`HOLDER.md:501`); row 15 → `==0` is unconditional
  `awaiting_deploy` while `==20` is "SERVE-verify if in-sync, else `awaiting_deploy`"
  (`HOLDER.md:503`). The A3 predicate decides solely on `plan_hash == expected` +
  `LiveFingerprint == ExpectedFingerprint` (`HOLDER.md:370-381`), with no `applied_owner`
  input, so for the same in-sync facts it returns the same outcome regardless of the
  bucket — the table's differing outcomes are unproducible by the specified predicate.

Because **M6 is a material new challenge standing unrebutted** and the decision table is
therefore not complete/executable in the `complete`/`applied_owner == 0` cells, the gate
does **not** clear. This is the gate's single allowed revision cycle, so it ends
**unCleared**.

## 2. Per-finding / per-challenge ledger

| # | Source | What is judged | Material? | Resolved / rebutted? | Disposition |
| --- | --- | --- | --- | --- | --- |
| **M5** | cycle-1 finding / both falsifiers' revision-check | the §3.5/F18 `applied_owner` dimension split `{0/1..19/==20/>=21}`, cell 1/`==0` serve, the F18 dual-cell + F18a assertions, cell `==20` relabel (`HOLDER.md:454-489,723-724`; `owner.go:145,148-150`) | — | **Yes — RESOLVED at row 1** (both falsifiers concede; adjudicator source-verified `owner.go:145`) | **RESOLVED (row 1)** |
| **M6** | falsifier_1 (`dialogue:2`) + falsifier_2 (`dialogue:3`) — **independent convergence** | the M5 split is NOT propagated through the `complete` rows: §3.5 rows 13/15 give `==0` a different outcome than `==20`, contradicting the `applied_owner`-independent A predicate (§3.3a), §1.3, and §4.5; F18 is a false oracle for the in-sync `complete`/`==0` cells (`HOLDER.md:359-381,501,503,525-528,723`) | **Yes** — F18 (the executable named requirement and the proof harness for M3/BC-N2/C2/M5) is a false oracle / the build must smuggle an unstated guard; spec edit required | **No** — no holder rebuttal; the "complete+`==0` is unreachable" reading is preempted by both falsifiers and by §4.5's admitted degenerate subcase | **STANDS → needs_revision** |
| **M3** | carried forward / both falsifiers | hoisted step-0 config gate (every cursor state, every column that passes W incl. `0`) + no-revoke `complete` pure-read comparison + tightened Invariant B + F17/F11(g)/F18 (`HOLDER.md:352-381`) | — | **Yes — INTACT** (M5 re-buckets W only; cell 2/`==0` still halts; M6 does not re-open the bypass) | **INTACT** |
| **M4** | carried forward / both falsifiers | F16a synthetic (step 2) + F16b production + forced self-heal via `isCrossBundleDependencyError` (step 7) (`HOLDER.md:316-327`) | — | **Yes — INTACT** | **INTACT** |
| **M1** | carried forward / both falsifiers | `VerifyStoredTranscript` on resume AND finalizer step 0; typed mismatch halts; F15 + F14 (`HOLDER.md:409-430`) | — | **Yes — INTACT** | **INTACT** |
| **M2** | carried forward / both falsifiers | single non-revoke filter `OwnerDDLApplyBundles()` + in-loop guards across every route incl. FMA-007 self-heal; embed/listing split (`HOLDER.md:286-327`) | — | **Yes — INTACT** | **INTACT** |
| **BC-N1** | carried forward / both falsifiers | immutable `deploy_plan` transcript; resume off the stored transcript; §1.3 + doctor + F14 (`HOLDER.md:183-242`) | — | **Yes — INTACT** | **INTACT** |
| **BC-N2** | carried forward / both falsifiers | universal non-`complete` edge at `applied_owner == 20`; the `==20` BC-N2 cells 5/7/9/11 verbatim; F11(e)/(f) (`HOLDER.md:487-504`) | — | **Yes — INTACT** (M5 is the orthogonal W dimension; M6 is in the `complete`/`==0` cells, not the non-complete edge) | **INTACT** |
| **C1** | carried forward / both falsifiers | `finalizing` + idempotent finalizer (VerifyStoredTranscript step 0) + §1.3 row + F10 (`HOLDER.md:204-212,409-422`) | — | **Yes — INTACT** | **INTACT** |
| **C2** | carried forward / both falsifiers | `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `Required = 20` (`HOLDER.md:487-504,590-614`; `owner.go:35`) | — | **Yes — INTACT** (M5 adds the `0`/`1..19` columns below the frontier; `Required`/watermark/`>=21` rule unchanged) | **INTACT** |
| **C3** | carried forward / both falsifiers | 0021 special-cased + terminal + revoke-last; F12/`G-revoke-last` (`HOLDER.md:286-327,643-675`) | — | **Yes — INTACT** | **INTACT** |

## 3. Why the verdict turns the way it does

### M5 — the owner-watermark dimension split, row 1: RESOLVED

The v6 break was that §3.5/F18 mapped `applied_owner < 20 → awaiting_owner_ddl` uniformly
("ALWAYS halts at W") and mislabeled cell `==20` the "fresh-DB bring-up" cell, while
current source `CheckOwnerBundleWatermark` **serves** `applied_owner == 0`. The v7 revision
closes the row-1 defect exactly as the v6 ledger §4 prescribed: the `applied_owner`
dimension is split into `{0/no authority, 1..19 authority shortfall, ==20, >=21}`; cell
1/`==0` (`cursorState=none`, off, no-revoke, no transcript) **serves-legacy** the fresh-DB
bring-up (`ApplyMigrations` + the legacy `:399` self-record run; Invariant B not in scope —
there is no transcript to write around); `1 <= applied_owner < 20` retains
`awaiting_owner_ddl`; F18 asserts both cells and F18a pins the fresh-DB serve; cell `==20`
is relabeled the inert-landing re-boot cell (`HOLDER.md:454-489,723-724`).

**Both falsifiers explicitly concede the row-1 repair.** I source-verified it against the
run worktree (HEAD `f4fcbde4`; `git diff --stat 2e482077 HEAD -- go/pkg/db/` empty):
`go/pkg/db/owner.go:145` `if applied == 0 { return nil }` returns **before** the
`if applied < RequiredOwnerBundleVersion` shortfall at `:148-150`, with the function
comment ("a 0-watermark database … is treated as the bootstrap/single-role case and NOT
halted. Only a database that HAS an authority schema (applied >= 1) but lags the required
frontier is a genuine shortfall", `:116-123`/`:140-143`); `OwnerBundleVersion` returns 0
when `owner_bundle_meta` is absent; `RequiredOwnerBundleVersion = 20`. **M5 is genuinely
resolved at row 1, and the relabel is correct** (the genuine fresh no-authority DB is
`applied_owner == 0`).

### M6 — the un-propagated split / false F18 oracle in the `complete`/`==0` cells: STANDS (the blocker)

This is the load-bearing reason the gate does not clear, and it is the strongest signal a
falsification gate can produce: **two falsifiers, from two independent lenses
(decoupling-boundary/decision-table; carry-forward/regression), converged on the same
source-verified defect and each made it their explicit blocker.** I verified it directly
this cycle:

- **The holder's structural claim.** §3.5 and §3.3a state that A **does not read
  `applied_owner`**, so the `0` and `==20` columns have **identical A-gate behavior** once
  W has passed — "that identity is the structural core of the M5 fix" (`HOLDER.md:359-360,
  471-475`). §1.3 says a `complete` cursor whose stored plan and fingerprint match the
  binary serves verify-only (decoupled) or serves legacy-no-op (no-revoke, already in-sync)
  regardless of the owner-watermark bucket (`HOLDER.md:223-225`). F18 must assert the exact
  §3.5 outcome for all 64 cells (`HOLDER.md:723`).
- **The table violates the claim in the `complete` rows.** Row 13 (`complete`, flag off,
  no-revoke): `==0` is `awaiting_deploy`, `==20` is "SERVE-legacy if in-sync, else
  `awaiting_deploy`" (`HOLDER.md:501`). Row 15 (`complete`, decoupled on, no-revoke): `==0`
  is unconditional `awaiting_deploy`, `==20` is "SERVE-verify if in-sync, else
  `awaiting_deploy`" (`HOLDER.md:503`). The A3 predicate decides solely on
  `plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint` (`HOLDER.md:370-381`).
  For the **same** in-sync facts, A returns the **same** outcome regardless of
  `applied_owner` — so the table's differing `==0`/`==20` outcomes cannot be produced by the
  specified predicate.
- **Source verification — the in-sync `complete`/`==0` cell is constructible.**
  `LiveFingerprint` reads the recorded `striatumd.schema_state.fingerprint` singleton
  (`go/pkg/db/schema_drift.go:145-161`), **not** a recompute of the live schema, and
  `RecordSchemaFingerprint` UPSERTs the running binary's `ExpectedFingerprint()`
  (`schema_drift.go:171-195`). Both are **orthogonal** to `owner_bundle_meta` /
  `applied_owner` (separate tables). So a DB with `owner_bundle_meta` absent
  (`applied_owner == 0`) **can** carry `schema_state.fingerprint == ExpectedFingerprint()`
  and a `complete` `deploy_cursor` with `cursor.plan_hash == expected`. A parametrized F18
  over all 64 cells **will** construct exactly that cell.
- **The fork is a real defect either way.** For the constructed in-sync `complete`/`==0`
  cell, W returns nil (`owner.go:145`) and A returns nil (serve verify-only on row 15; serve
  legacy no-op + idempotent `:399` rewrite on row 13, `HOLDER.md:370-381`), and §1.3 says
  serve — but §3.5 rows 15/`==0` and 13/`==0` say `awaiting_deploy`. So either (1) **F18 is a
  false oracle** for these cells, or (2) the implementation must add an **unstated
  `applied_owner`-dependent complete-cursor guard**, contradicting the holder's own "A does
  not read `applied_owner`"/"identical A behavior" claim. Both are material.
- **The holder's own §4.5 compounds the contradiction.** §4.5 **admits** the "degenerate
  cell 13/`==0` in-sync" idempotent `:399` rewrite (`HOLDER.md:525-528`) while the F18 spy
  list permits `RecordSchemaFingerprint` **only** in cells 1/`==0`, 1/`==20`, and
  13/`==20`-in-sync (`HOLDER.md:723`) — omitting 13/`==0`-in-sync. So the Invariant-B proof
  and the F18 oracle disagree about which cells reach the legacy writer.

The holder's anticipatable rebuttal — that `complete + applied_owner == 0` is an
unreachable/corrupt shape, so the `==0` halts are conservative — is **preempted by both
falsifiers** and does not rescue the written spec: §3.5 promises exact executable outcomes
for **all 64 cells** and does not mark the cell impossible; the specified A predicate
cannot produce the conservative halt because it does not read `applied_owner`; and §4.5
itself **enumerates** the in-sync `==0` subcase as reachable. If the intended rule is
"a `complete` transcript over `applied_owner == 0` is inconsistent and must halt", that is
a **new guard** that must be specified in W or A, with F18 asserting it.

This is **not** a SAFETY hole — Universal Invariant B holds in **both** readings (the only
`==0` legacy write that could fire is an idempotent rewrite of the **identical**
fingerprint, and no fingerprint is ever written around a transcript carrying a pending
change), and it is **not** a regression of any carry-forward (M3 cells 2/6/10/14 still halt
at A0; the BC-N2 `==20` edge, M2/C3, `Required = 20`, and the `>=21` forward rule are
intact — both falsifiers confirm). But it **is** a decision-table **executability /
completeness** failure: F18 is the **executable named requirement** the SEED mandated and
the **carry-forward proof harness for M3, BC-N2, C2, and M5 together**, and as written it
cannot be both true to the §3.3a predicate and true to the §3.5 table. It is exactly the
failure mode the SEED warned the proactive table existed to preempt: **"A re-scaffolded
revision that pins M5 but leaves any further unaudited cell open will spawn an M6 — close
the class."** The class is **not** closed: the split is coherent at `cursorState = none`
but incoherent at `cursorState = complete`. It is **material** (it requires a spec edit and
exposes a real defect in an executable named requirement) and **stands unrebutted** (no
holder rebuttal artifact this cycle; the spec text affirms the incoherent table). **M6
forces `needs_revision`.**

### M3 / M4 / M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3 — INTACT; the decision table — NOT COMPLETE

All nine carry-forward findings are preserved verbatim and unregressed, and both falsifiers
concede each. **M3** — the hoisted step-0 config gate fires for every cursor state in every
column that passes W (incl. the new `0` column: cell 2/`==0`, a revoke-embedding binary on
a fresh DB with the flag OFF, still halts `awaiting_deploy_config`, preserving the
asymmetry that the M5 serve cell is no-revoke only); the no-revoke `complete` pure-read
comparison and the tightened Invariant B are intact. **M4** — F16a synthetic / F16b
production + forced self-heal. **M1** — `VerifyStoredTranscript` on resume and finalizer
step 0. **M2** — the single non-revoke filter + embed/listing split. **BC-N1** — the
immutable `deploy_plan` transcript. **BC-N2** — the universal non-`complete` edge at
`applied_owner == 20` (the `==20` column's BC-N2 cells verbatim; M5 is the orthogonal W
dimension, M6 is in the `complete`/`==0` cells). **C1** — the `finalizing` finalizer.
**C2** — `CheckDeployActivation` before `ApplyMigrations`, forward-watermark at
`applied >= 21`, `Required = 20` not advanced. **C3** — 0021 special-cased + terminal +
revoke-last.

**Decision table — NOT COMPLETE.** Universal Invariant B itself is proven (no unsafe write
in any cell), and the row-1 owner-watermark split, the M3 cell (2/6/10/14), the BC-N2
`==20` cells, and the shadow-mode drift-gate fall-through are covered. But the table is
**not executable/coherent in the owner-watermark dimension at `cursorState = complete`**:
§3.5 rows 13 and 15 give the `==0` column an outcome the specified `applied_owner`-
independent A predicate cannot produce, §4.5 admits a degenerate `==0` write the F18 spy
list forbids, and §1.3 says serve where §3.5 says halt. A table whose own predicate, proof,
and oracle disagree for a set of cells is not the COMPLETE, executable table the SEED
required (M6).

## 4. What the revision must fix (exact)

A re-scaffolded revision would clear the gate only if it pins the single item below, while
keeping M5 (row 1) and M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, C3 intact and unregressed.

1. **Propagate the M5 `applied_owner` split coherently through the `complete` rows so the
   §3.5 table, the §3.3a A predicate, §1.3, §4.5, and F18/F18a all agree (M6 — the
   load-bearing fix, decision-table executability).** Choose **one** coherent contract and
   apply it everywhere:
   - **(1) Mirror `==20` anywhere W passes and A is owner-watermark independent.** Make
     §3.5 rows 13 and 15 in the `==0` column conditional — "**serve if in-sync, else
     `awaiting_deploy`**" — exactly as the `==20` column; and add the degenerate
     13/`==0`-in-sync idempotent `:399` rewrite to the §4.5 Invariant-B enumeration and to
     the F18 spy list. This keeps the holder's "identical A behavior for the `0` and `==20`
     columns" claim true. **OR**
   - **(2) Classify `complete + applied_owner == 0` as an inconsistent state.** Add an
     explicit guard (in W or A) that **detects** the inconsistency (a `complete`/in-sync
     deploy_cursor over an `owner_bundle_meta`-absent DB) and halts a typed error
     **before** serving; **remove** the "identical A behavior for the `0` and `==20`
     columns" claim for the `complete` rows; and make F18 (and a new sub-assertion) assert
     that typed halt for the `complete`/`==0` cells.

> Note the asymmetry the fix must preserve (carried from the v6 ledger §4): the M3 halt is
> conservative *on purpose* for a revoke-embedding binary, and the watermark `1..19` halt
> is correct for a genuine authority shortfall — but the fix must keep cell 1/`==0`
> (`cursorState = none`) **serving** the fresh bootstrap. The repair is local to the
> `complete` rows; it must not re-collapse the resolved row-1 fresh-DB serve, weaken the M3
> config gate (cells 2/6/10/14 must still halt `awaiting_deploy_config` at A0), regress the
> BC-N2 `applied_owner == 20` edge, or advance `RequiredOwnerBundleVersion`.

## 5. What already cleared (carry forward — do NOT regress or re-litigate)

- **M5 owner-watermark dimension split, ROW 1 — RESOLVED this cycle** (§3 above): preserve
  the `{0/no authority, 1..19 authority shortfall, ==20, >=21}` split; cell 1/`==0`
  serves-legacy the fresh-DB bring-up (the legacy `:399` self-record is legitimate — no
  transcript, Invariant B not in scope); `1 <= applied_owner < 20` halts `awaiting_owner_ddl`;
  F18 asserts both cells; F18a pins the fresh-DB serve; cell `==20` relabeled the
  inert-landing cell. (The M6 fix only propagates the split through the `complete` rows; it
  must not re-collapse this.)
- **M3 complete-cursor activation guard — intact**: preserve the hoisted step-0
  `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config gate (every cursor
  state, every column that passes W incl. `0`), the no-revoke `complete` pre-`ApplyMigrations`
  pure-read comparison, the tightened Invariant B, and F17/F11(g)/F18.
- **M4 F16 phase-aware split — intact**: F16a (synthetic, step 2) + F16b (production +
  forced self-heal via `isCrossBundleDependencyError`, step 7).
- **M1 full-transcript verifier — intact**: `VerifyStoredTranscript` on every resume and as
  finalizer step 0; the typed mismatch halts; F15 + F14.
- **M2 non-revoke filter — intact**: `DDLRevokeOwnerBundleVersion = 21`, `isNonRevokeBundle`,
  `OwnerDDLApplyBundles()`, the in-loop guards, the nil-fallback split, the embed/listing
  split.
- **BC-N1 immutable transcript / resume-off-stored-plan — intact**: the 0044 `deploy_plan`
  table, materialize-before-step-0, §1.3, the transcript-enumerated doctor, F14.
- **BC-N2 universal non-complete-cursor edge — intact**: the `revokeEmbedded`-independent
  halt at `applied_owner == 20` and F11(e)/(f). (M5 is the orthogonal W dimension; M6 is in
  the `complete`/`==0` cells; neither weakens the `==20` non-complete edge.)
- **C1 finalization boundary — intact**: the `finalizing` state + idempotent finalizer +
  §1.3 row + F10, with M1's pre-finalizer `VerifyStoredTranscript` step 0.
- **C2 fail-closed activation edge — intact**: `CheckDeployActivation` before
  `ApplyMigrations`, the typed halts, the forward-watermark rule (fires at `applied >= 21`),
  and `RequiredOwnerBundleVersion = 20` (not advanced to the revoke ordinal).
- **C3 revoke-last ownership mechanism — intact**: 0021 special-cased + terminal + the
  reconcile-while-CREATE-held ordering + F12/`G-revoke-last`.
- **Q4 — resolved** (v1–v6): plain verb, bootstrapping paradox dispositive, run-shape door
  held by the three seams. Neither falsifier contested it.
- **Q3 mid-step resumability body — sound**: Q3-A/Q3-B, the §1.1 step taxonomy. M5/M6
  concern the boot-path decision table's owner-watermark × cursor-state dimension, not the
  per-step schema/cursor body.
- **Decoupling shape — sound** (M3-hardened): shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark interlock +
  P3 drift gate, the embed-FS-derived plan, the hash-chained per-step receipt.
- **Scope discipline held**: P5 deferred; full capability revocation named as a follow-up
  beyond P4; the local-first boundary respected.

## 6. Gate decision

The ledger verdict — not falsifier completion — clears the phase gate. **M5 is genuinely
resolved at row 1** (both falsifiers concede), and **M3, M4, M1, M2, BC-N1, BC-N2, C1, C2,
and C3 are carried forward intact** (both falsifiers concede no regression). But a clearing
verdict also requires **the boot-path decision table complete and executable (the F18
oracle matching the §3.3a predicate, Invariant B proven in every cell, the legitimate
fresh-DB cells still serving) and no new material challenge standing** — and **M6** is a
material challenge standing unrebutted, source-verified against the run worktree and landed
independently by **both** falsifiers:

- **M6**: the M5 `applied_owner` split is propagated to row 1 (`cursorState = none`) but
  **not** to the `complete` rows — §3.5 rows 13/15 give the `==0` column an outcome the
  holder's own `applied_owner`-independent A predicate cannot produce, §4.5 admits a
  degenerate `==0` write the F18 spy list forbids, and §1.3 says serve where §3.5 says halt,
  so the executable F18 matrix is a false oracle for the in-sync `complete`/`applied_owner
  == 0` cells. The holder's "complete+`==0` is unreachable/corrupt" reading does not hold
  against §3.5's "exact outcome for all 64 cells", the `applied_owner`-independent A
  predicate, and §4.5's own admitted degenerate subcase.

**Verdict: `needs_revision`.** This is the gate's single allowed revision cycle, so the
gate ends **unCleared** — judged on the merits. A clearing verdict would require a
re-scaffolded revision that propagates the M5 split coherently through the `complete` rows
(§4 above — one of the two coherent contracts, with §3.5, §3.3a, §1.3, §4.5, and F18/F18a
made consistent) without regressing M5 (row 1) or M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3.

<sub>Adjudicator collaboration ledger for the RFC 0142 P4 falsification-gate design run,
v7 / REVISION, cycle 1. Inputs:
`docs/operator/artifacts/rfc-0142-p4-design-v7/dialogue/holder/HOLDER.md`,
`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, `SEED.md`;
context: v6 `HOLDER.md` + v6 `COLLABORATION_LEDGER_cycle_1.md` (its M5 finding + §4 + §5).
Adjudicator source verification against the run worktree (HEAD `f4fcbde4`;
`git diff --stat 2e482077 HEAD -- go/pkg/db/` empty): `go/pkg/db/owner.go:116-150` (the
`applied == 0` fresh-DB serve exception confirmed — returns nil before the
`< RequiredOwnerBundleVersion` shortfall check), `owner.go:35` (`RequiredOwnerBundleVersion
= 20`), `go/pkg/db/schema_drift.go:145-161` (`LiveFingerprint` reads the recorded
`schema_state.fingerprint` singleton — orthogonal to `owner_bundle_meta`),
`schema_drift.go:171-195` (`RecordSchemaFingerprint` writes the binary's
`ExpectedFingerprint()`), `go/pkg/db/connection.go:349/:353/:399` boot order; HOLDER /
falsifier anchors as cited. Verdict vocabulary: accept | accept_with_findings |
needs_revision | reject.</sub>

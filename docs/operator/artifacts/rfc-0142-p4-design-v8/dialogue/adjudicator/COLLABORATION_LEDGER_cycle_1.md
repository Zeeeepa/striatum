---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsifiable implementation spec (v8 REVISION; the single binding cycle-1 finding M6 — the M5 `applied_owner` split not propagated through the `complete` rows — is GENUINELY RESOLVED at the NAMED rows 13/15: those `==0` cells are now conditional 'serve if in-sync, else awaiting_deploy' identical to `==20`, §0.2 states the W→A-independence invariant anchored to schema_drift.go:145-161/:171-195, and §4.5 + the F18 spy list enumerate the SAME four `:399`-reaching cells {1/==0, 1/==20, 13-in-sync/==0, 13-in-sync/==20}; both falsifiers concede the 13/15 repair, and M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 carry forward intact — BUT a NEW material decision-table incoherence M7 lands INDEPENDENTLY from BOTH falsifiers and stands unrebutted: §3.5 row 16 (complete, decoupled, revoke-embedding) gives the `==0`/`==20` cells UNCONDITIONAL `awaiting_deploy` reasoned via an unstated `applied_owner < 21 ⟹ fingerprint mismatch` coupling, but A's §3.3a step-3 decoupled branch is owner-watermark-blind — it reads only `plan_hash` + the recorded `schema_state.fingerprint`, source-verified orthogonal to `owner_bundle_meta` — so the in-sync row-16 cell is constructible and A serves verify-only there while the table says halt; row 16 is asserted, not derived, violating the holder's OWN derivation rule HOLDER.md:565-566; adjudicator source-verified against the run worktree HEAD a970f546, go/pkg/db diff vs the holder-cited 3f9d5734 EMPTY)"
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
    text: "Revised P4 spec (v8) resolves the single binding cycle-1 finding M6 via contract (1): it DERIVES the entire 64-cell §3.5 boot-path decision table MECHANICALLY from the two predicates W (`CheckOwnerBundleWatermark`) and A (`CheckDeployActivation`), stating the load-bearing INVARIANT W→A-INDEPENDENCE explicitly (§0.2): once W passes (`applied_owner ∈ {0, ==20}`, or `>=21` for a revoke-embedding binary), A is owner-watermark-independent — it reads `cursorState`, `decoupledEnabled`, `revokeEmbedded`, `cursor.plan_hash`, and the recorded `schema_state.fingerprint`, NEVER `owner_bundle_meta`/`applied_owner` (`schema_drift.go:145-161` `LiveFingerprint` reads the `schema_state` singleton; `:171-195` `RecordSchemaFingerprint` writes only `schema_state`) — so for any fixed `(cursorState, decoupledEnabled, revokeEmbedded)` row the `==0` and `==20` columns have IDENTICAL A-gate outcomes in EVERY cursor row. §3.5 rows 13 and 15 in the `==0` column become conditional 'serve if in-sync, else `awaiting_deploy`' identical to `==20`; the degenerate 13/`==0`-in-sync idempotent `:399` rewrite is added to BOTH the §4.5 Universal-Invariant-B enumeration AND the F18 spy list, so the four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} are enumerated identically; §3.5 carries an explicit cross-row `==0`-vs-`==20` audit (none/in_progress/finalizing/complete). M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 are carried forward verbatim and unregressed, re-verified line-by-line against the v8 worktree HEAD `3f9d5734`. The fix is local to the `complete` rows; it does NOT re-collapse the row-1 fresh-DB serve, weaken the M3 config gate (cells 2/6/10/14 still halt `awaiting_deploy_config` at A0), regress the BC-N2 `applied_owner == 20` edge, or advance `RequiredOwnerBundleVersion` (stays 20)."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "Revision check (CONCEDED for rows 13/15): the v8 holder genuinely repairs the named v7 M6 defect. §0.2 states the W→A-independence invariant; the source anchors support it (`owner.go:23-35,124-154`; `schema_drift.go:145-161,171-195` read/write only `schema_state`, never `owner_bundle_meta`). The concrete v7 refutation cell is corrected — row 13/`==0` conditional 'SERVE-legacy if in-sync, else `awaiting_deploy`' exactly like `==20`, row 15/`==0` conditional 'SERVE-verify if in-sync, else `awaiting_deploy`' exactly like `==20` (HOLDER.md:586-588); the degenerate 13/`==0`-in-sync `:399` rewrite is in BOTH §4.5 and the F18 spy list (the four cells {1/==0,1/==20,13-in-sync/==0,13-in-sync/==20}, HOLDER.md:622-645,804-832,855); the cross-row audit walks all four cursor groups (HOLDER.md:598-620). Carry-forwards intact from this lens: the M3 A0 gate (HOLDER.md:434-437,587); M5 row-1/`==0` serve + `1..19` W halt (HOLDER.md:540-546,574); the BC-N2 non-complete `==20` edge (HOLDER.md:578-585); `RequiredOwnerBundleVersion = 20` (`owner.go:23-35`); M1/M2/M4/BC-N1/C1/C2/C3 with no direct contradiction. NEW MATERIAL CHALLENGE — ROW 16 IS STILL NOT MECHANICALLY DERIVED FROM A: the holder's clearing claim is that the full 64-cell table is derived from W and A, each W-passing cell taking A's outcome (HOLDER.md:521-566). A's complete/decoupled branch is unconditional on `revokeEmbedded`/`applied_owner`: if `cursorState == complete` AND `decoupledEnabled == true` AND `cursor.plan_hash == expected` AND `LiveFingerprint == ExpectedFingerprint` it serves verify-only, else `awaiting_deploy` (HOLDER.md:441-445); the complete/in-sync row serves independent of `applied_owner` (HOLDER.md:763-766). Row 16 breaks the derivation: for `cursorState=complete, decoupledEnabled=true, revokeEmbedded=true, applied_owner ∈ {==0,==20}` the table gives UNCONDITIONAL `awaiting_deploy`, reasoned '0021 not yet applied → fingerprint ≠ → not in-sync' (HOLDER.md:589) — but 'fingerprint ≠' is not an output of W or A; it is an unstated consistency assumption inferred from the owner watermark. The SAME orthogonality argument that fixed row 13 proves `schema_state.fingerprint` can be in-sync while `owner_bundle_meta` is absent or 20 (`schema_drift.go:145-161,171-195`). Concrete refutation (the F18 row-16 cell): cursorState=complete, decoupledEnabled=true, revokeEmbedded=true, applied_owner=0 (or ==20), `deploy_plan[plan_hash]` present, `cursor.plan_hash == expected`, `LiveFingerprint(recorded) == ExpectedFingerprint()`, `owner_bundle_meta` absent (or 20) — W passes (`owner.go:145,151-153`), A takes the complete+decoupled branch and returns nil (serve verify-only) because plan-hash and fingerprint are in sync (HOLDER.md:441-445); but §3.5 row 16 `==0`/`==20` requires `awaiting_deploy` and F18 must assert the exact §3.5 outcome (HOLDER.md:855), AND F18 only adds the in-sync/out-of-sync sub-dimension for the complete NO-REVOKE rows 13/15, even though A's complete/decoupled fingerprint predicate is the same for row 16. So a direct implementation of §3.3a serves the in-sync row-16 cell, while a §3.5-following oracle fails it or silently omits the subcase. This is NOT the v7 'one differs from ==20' defect (both columns still match); it is more basic — row 16 is asserted from an `applied_owner`-derived fingerprint premise A neither reads nor verifies. Material: the table smuggles a watermark-derived fingerprint fact into A; if the implementation follows A as written, a revoke-embedding decoupled binary serves verify-only over a `complete` cursor whose `schema_state` claims the 0021-bearing fingerprint while `owner_bundle_meta` is absent/20 — exactly the cross-table inconsistency the table claims to close. Reachability rebuttal ('complete + revoke + applied_owner<21 should mean 0021 applied so this is corrupt/unreachable') is preempted: the v7 M6 fix itself rejected unwritten reachability when §3.5 promises exact outcomes for all cells, and the holder already constructs the orthogonal 13/`==0`-in-sync cell (HOLDER.md:636-638), so it cannot selectively assume row 16 is out-of-sync without naming the additional invariant; `VerifyStoredTranscript` would catch a DB-stamp mismatch on deploy resume/finalization but `CheckDeployActivation` on serve-boot does NOT run it. Required repair (choose one, propagate everywhere): (1) make row 16 `==0`/`==20` conditional on the same A3 fingerprint predicate 'SERVE-verify if in-sync, else `awaiting_deploy`', documenting that the normal reachable pre-0021 state is out-of-sync; or (2) add an explicit consistency guard (`complete && revokeEmbedded && applied_owner < 21` → typed inconsistency halt, or a complete-boot stored-transcript DB-stamp verification proving the terminal owner step committed), define its typed halt, require F18 to assert it, and narrow the W→A-independence claim. As written the table is still partly asserted rather than derived; the revision should not clear until row 16 is made conditional like A or an explicit guard makes the halt executable."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "Revision check (CONCEDED for rows 13/15 + carry-forwards): the named v7 M6 defect is addressed — §0.2 states the W→A independence invariant (HOLDER.md:150-158), the 64-cell table is claimed derived not asserted (HOLDER.md:180-184,519-566), rows 13/15 `==0` are conditional matching `==20`, and the row-13 `==0` idempotent legacy rewrite is in both §4.5 and the F18 spy list (HOLDER.md:586-589,804-832,855); the cross-row audit walks all groups (HOLDER.md:598-620). No DIRECT regression in the requested carry-forward set: M5 row-1 `{0/1..19/==20/>=21}` split with `==0` serve + `1..19` W halt (HOLDER.md:538-546,572-575); M3 hoisted A0 `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` before any cursor branch incl. `complete` (HOLDER.md:434-437,575,579,583,587); M4 F16a synthetic/F16b production + forced FMA-007 self-heal (HOLDER.md:383-399,853); M1 `VerifyStoredTranscript` on resume + finalizer step 0 with typed halts (HOLDER.md:269-275,497-515,852); M2/C3 single non-revoke apply slice + deploy-terminal 0021/revoke-last (HOLDER.md:356-381,483-493,799-802); BC-N1/BC-N2/C1/C2 immutable `deploy_plan`, non-complete halt at A1/A2, `finalizing` finalizer, `Required = 20`, the no-revoke `>=21` forward barrier (HOLDER.md:245-278,438-452,549-552). BUT the row-13/15 repair exposed the next unmodeled complete-row cell — ROW 16 IS NOT MECHANICALLY DERIVED FROM A (it imports an unchecked C3/revoke-last fact into A). The SEED requires the entire 64-cell table derived from W and A (SEED.md:340-348); the holder repeats the rule (HOLDER.md:558-566). A's complete/decoupled branch is owner-watermark-blind (HOLDER.md:441-445,424-430,467-473); source-anchored (`owner.go:124-153`; `schema_drift.go:145-161,171-195`). Row 16 violates it: for `cursorState=complete, decoupledEnabled=true, revokeEmbedded=true, applied_owner == 0` (or ==20) the table gives unconditional `awaiting_deploy` because '0021 not yet applied' ⟹ fingerprint ≠ (HOLDER.md:589) — an inferred C3/revoke-last consistency fact (if the revoke bundle is not stamped, the fingerprint must not match) that the spec's own M6 proof says is invalid unless a guard states it (`schema_state`/`owner_bundle_meta` orthogonal, A reads the former, HOLDER.md:163-184,467-473). Concrete refutation (identical F18 row-16 cell as falsifier_1): W passes (`owner.go:145-153`); A returns nil (serve verify-only, HOLDER.md:441-445); the verify path never calls the legacy `:399` writer (so NOT the old Invariant-B issue) — but §3.5 requires `awaiting_deploy` (HOLDER.md:589) and F18 only sub-dimensions rows 13/15 (HOLDER.md:855); a build implementing §3.3a serves the constructed cell, an oracle blindly following §3.5 fails it or omits the subcase. Carry-forward impact: NOT a regression of row-1/13/15, NOT a re-open of the M3 legacy bypass (row 16 is decoupled, no `:399`), BC-N2 and the `>=21` rule intact — but material pressure on C3 (the steady state after a revoke deploy is `applied_owner >= 21` (HOLDER.md:483-493,770-785), and row 16 uses that reachability to conclude `<21` cannot be in-sync, but A does not verify the owner-bundle stamp before serving), M1 (its DB-stamp verification is specified on deploy resume/finalizer step 0, NOT on ordinary serve-boot over an already-`complete` cursor (HOLDER.md:269-275,497-515) — so if `complete + revoke + applied_owner<21` is meant to halt, A needs a boot-time consistency guard or a complete-boot stamp verifier; the current A predicate checks only plan-hash + `schema_state` fingerprint), and the claimed F18 proof harness (it imports the unstated cross-table invariant `owner_bundle_meta < 21 ⟹ fingerprint mismatch` into row 16 — precisely the premise M6 was meant to eliminate). Reachability rebuttal preempted (same as falsifier_1): the v8 M6 fix rejects unwritten reachability when the table promises exact outcomes; the holder already treats the orthogonal 13/`==0`-in-sync case as constructible, so it cannot selectively assume row 16 is out-of-sync without naming the invariant. Required repair (choose one, propagate through §1.3/§3.3a/§3.5/§4.5/F18): (1) make row 16 `==0`/`==20` conditional 'SERVE-verify if in-sync, else `awaiting_deploy`', documenting the normal pre-0021 state is out-of-sync; or (2) add an explicit consistency guard (`complete && revokeEmbedded && applied_owner < 21` → typed inconsistency, or complete-boot stored-transcript DB-stamp verification proving the terminal owner step committed), have F18 assert it, and narrow the W→A independence claim. As written the 64-cell table remains non-executable; a build can implement A and fail F18, or implement F18 by adding an unstated owner-watermark consistency guard. Material standing falsification."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "REVISION cycle v8 (the gate's single allowed revision cycle). The single binding cycle-1 finding M6 is GENUINELY RESOLVED at the NAMED cells (§3.5 rows 13 and 15), and BOTH falsifiers independently concede the row-13/15 repair: §0.2 states the load-bearing INVARIANT W→A-INDEPENDENCE explicitly (once W passes, A is owner-watermark-independent — it reads `cursorState`/`decoupledEnabled`/`revokeEmbedded`/`cursor.plan_hash`/the recorded `schema_state.fingerprint`, NEVER `owner_bundle_meta`/`applied_owner`); §3.5 rows 13 and 15 in the `==0` column are now conditional 'serve if in-sync, else `awaiting_deploy`' IDENTICAL to `==20`; §4.5 AND the F18 spy list now enumerate the SAME four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} (eliminating the v7 §4.5↔oracle disagreement); and §3.5 carries an explicit cross-row `==0`-vs-`==20` audit. ADJUDICATOR SOURCE-VERIFICATION this cycle (direct read of the run worktree, HEAD `a970f546`; `git diff --stat 3f9d5734 HEAD -- go/pkg/db/` EMPTY, so every anchor the holder cites against `3f9d5734` holds — the go/pkg/db source is byte-identical, an immaterial branch-advance since the v8 scaffold landed): `go/pkg/db/owner.go:145` `if applied == 0 { return nil }` returns BEFORE the `< RequiredOwnerBundleVersion` shortfall, `owner.go:23/:35` `LatestOwnerBundleVersion = RequiredOwnerBundleVersion = 20` (NOT advanced); `go/pkg/db/schema_drift.go:145-161` `LiveFingerprint` reads ONLY the `striatumd.schema_state` singleton and `:171-195` `RecordSchemaFingerprint` writes ONLY that singleton — grep confirms ZERO `owner_bundle_meta`/`applied_owner` references in `schema_drift.go`, so A's fingerprint inputs are GENUINELY orthogonal to W's owner-watermark inputs; `connection.go` boot order W → ApplyMigrations → drift gate → `RecordSchemaFingerprint` ('This is the only writer of schema_state') confirmed. The M6 rows-13/15 fix is real and source-accurate. The ten carry-forwards M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are carried forward verbatim and INTACT — both falsifiers concede no regression, and the M6 derivation does not re-collapse the row-1 fresh-DB serve, weaken the M3 A0 config gate (cells 2/6/10/14 still halt `awaiting_deploy_config` incl. the `0` column), regress the BC-N2 `applied_owner == 20` non-complete edge (cells 5/7/9/11 verbatim), or advance `Required`. BUT a clearing verdict ALSO requires (a) the proactive-completeness boot-path decision table COMPLETE and EXECUTABLE — every cell with an outcome DERIVED FROM W and A, an F18 oracle matching the §3.3a predicate, Invariant B proven in every cell, the legitimate fresh-DB cells still serving — and (b) no new material challenge standing unrebutted. BOTH fail on the SAME new defect (M7). BOTH falsifiers, from independent lenses (falsifier_1 decoupling-boundary/decision-table; falsifier_2 carry-forward/regression), CONVERGE on a new material decision-table INCOHERENCE that is the IDENTICAL class as M6, one cell over: the M6 mechanical derivation closed rows 13 and 15 (no-revoke complete) but row 16 (complete, decoupled, revoke-embedding) is NOT derived from A. §3.5 row 16 gives the `==0` and `==20` columns UNCONDITIONAL `awaiting_deploy` (HOLDER.md:589), reasoned '0021 not yet applied → fingerprint ≠ → not in-sync' — but A's §3.3a step-3 decoupled branch decides solely on `cursor.plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint` (HOLDER.md:441-445), with NO `applied_owner` input, so for the same in-sync facts A returns the SAME outcome regardless of the owner watermark. The holder VIOLATED ITS OWN derivation rule (HOLDER.md:565-566: 'where A's outcome is conditional on the fingerprint-sync state … the cell is written conditionally') — row 16's complete/decoupled outcome IS conditional on fingerprint-sync, yet the cell is written UNCONDITIONALLY. ADJUDICATOR SOURCE-VERIFICATION: because `schema_state` and `owner_bundle_meta` are orthogonal tables (confirmed above), the in-sync row-16 cell (`schema_state.fingerprint == ExpectedFingerprint()` AND `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent / version-20 DB) IS constructible — exactly as the holder itself constructs the degenerate 13/`==0`-in-sync cell (HOLDER.md:636-638) — and a parametrized F18 over all 64 cells WILL construct it. For that cell W returns nil, A returns nil (serve verify-only on the decoupled complete branch), but §3.5 says `awaiting_deploy`. Therefore F18 — the required executable matrix asserting the EXACT §3.5 outcome — is a FALSE ORACLE for the in-sync row-16 `==0`/`==20` cells (and F18 only adds the in-sync/out-of-sync sub-dimension for rows 13/15, NOT 16, HOLDER.md:855), OR the build must smuggle in an unstated `applied_owner`-dependent guard contradicting the W→A-independence claim. This is MATERIAL: row 16 imports the cross-table premise `owner_bundle_meta < 21 ⟹ fingerprint mismatch` that A neither reads nor verifies — precisely the premise the M6 fix was meant to eliminate — so the 64-cell table is NOT fully derived from W and A and F18 is not the complete oracle it claims. It is the SEED's EXPLICITLY warned-of M7 ('a re-scaffolded revision that fixes the `complete` rows but leaves the in_progress or finalizing rows incoherent will spawn an M7 — close the class fully across ALL cursor rows'); the holder fixed the no-revoke `complete` rows (13/15) but left a SIBLING cell in the SAME `complete` cursor-state group (row 16, revoke-embedding decoupled) with the identical asserted-not-derived defect. The class is NOT closed by derivation. The holder's anticipatable reachability rebuttal ('complete + revoke-embedding + applied_owner < 21 means 0021 is pending, so the cell is corrupt/unreachable and the `awaiting_deploy` halt is conservative') is preempted by BOTH falsifiers and does not rescue the written spec: §3.5 promises exact executable outcomes for ALL 64 cells and does not mark the cell impossible; the specified A predicate cannot produce the conservative halt because it does not read `applied_owner`; the holder relies on the very orthogonality it would have to deny here; and `CheckDeployActivation` on serve-boot does NOT run `VerifyStoredTranscript` (the only DB-stamp verifier), so no stated mechanism detects the inconsistency before serving. It STANDS UNREBUTTED — this cycle carries no holder rebuttal artifact; the falsifiers are the last word and the spec text (HOLDER.md:589,855) affirms the asserted row-16 cell. Note M7 is NOT a SAFETY hole (row 16 is decoupled → the verify path never reaches the legacy `:399` writer; Invariant B holds; both falsifiers concede this) and it does NOT regress any carry-forward (the M3 bypass stays closed, BC-N2 `==20` and the `>=21` rule intact, `Required` not advanced) — but it IS a decision-table EXECUTABILITY / COMPLETENESS failure that fails the explicit clearing precondition 'the boot-path decision table is complete and executable (derived from W and A)' + 'no new material challenge standing unrebutted'. Because the decision table is not executable/derived in the `complete`/decoupled/revoke-embedding cells (row 16) and a new material challenge stands unrebutted, the gate does NOT clear. This is the single allowed revision cycle for v8, so the gate ends unCleared — judged exactly on the merits, not graded on the consequence; §4 records the single load-bearing fix (derive row 16 from A under one coherent contract — make `==0`/`==20`/`>=21` conditional on the A3 fingerprint predicate, OR add an explicit consistency guard and have F18 assert it), and §5 records what cleared and must not be regressed (M6 rows-13/15 + M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 — all genuinely resolved/intact this cycle)."
findings:
  - id: M6
    severity: high
    posture: design
    status: answered
    challenge: "RESOLVED (rows 13/15) — the single binding cycle-1 finding (the M5 `applied_owner` split not propagated through the `complete` rows) is genuinely fixed in the NAMED cells; BOTH falsifiers explicitly concede the row-13/15 repair. §0.2 states the load-bearing INVARIANT W→A-INDEPENDENCE explicitly (HOLDER.md:150-158): once W passes (`applied_owner ∈ {0, ==20, >=21-as-barrier}`), A is owner-watermark-independent — it reads `cursorState`/`decoupledEnabled`/`revokeEmbedded`/`cursor.plan_hash`/the recorded `schema_state.fingerprint`, NEVER `owner_bundle_meta`/`applied_owner` — so the `==0` and `==20` columns have IDENTICAL A-gate outcomes in EVERY cursor row. §3.5 rows 13 and 15 in the `==0` column are now CONDITIONAL — 'serve if in-sync, else `awaiting_deploy`' — IDENTICAL to `==20` (HOLDER.md:586,588). The degenerate 13/`==0`-in-sync idempotent `:399` rewrite is added to BOTH the §4.5 Universal-Invariant-B enumeration AND the F18 spy list, so the four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} are enumerated identically (HOLDER.md:622-645,804-832,855) — eliminating the v7 §4.5↔oracle disagreement. §3.5 carries an explicit cross-row `==0`-vs-`==20` audit (none/in_progress/finalizing/complete, HOLDER.md:598-620). ADJUDICATOR SOURCE-VERIFICATION (run worktree, HEAD `a970f546`; `git diff --stat 3f9d5734 HEAD -- go/pkg/db/` empty): `schema_drift.go:145-161` `LiveFingerprint` reads ONLY the `schema_state` singleton, `:171-195` `RecordSchemaFingerprint` writes ONLY it — grep confirms ZERO `owner_bundle_meta`/`applied_owner` references — so A is genuinely owner-watermark-independent; `owner.go:145` serves `applied == 0`, `:35` `Required = 20`. The rows-13/15 fix is real and source-accurate. HOWEVER, the mechanical derivation was NOT carried through to the SIBLING complete-row cell (row 16) — see M7. **M6 RESOLVED at rows 13/15; the un-derived row-16 cell is tracked as the new finding M7.**"
    affected_invariants: ["boot-path decision table coherent in the `complete`/no-revoke rows (13/15): the `==0` and `==20` columns identical, derived from the `applied_owner`-independent A predicate", "the degenerate 13/`==0`-in-sync `:399` rewrite enumerated identically in §4.5 and the F18 spy list (the four `:399`-reaching cells)", "§0.2 the W→A-independence invariant stated and anchored to schema_drift.go:145-161/:171-195", "the cross-row `==0` audit for none/in_progress/finalizing/complete"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M7
    severity: high
    posture: design
    status: open
    challenge: "OPEN — NEW material challenge landed INDEPENDENTLY by BOTH falsifiers (falsifier_1 decoupling-boundary/decision-table lens; falsifier_2 carry-forward/regression lens), source-verified against the run worktree, standing unrebutted. The M6 mechanical derivation closed rows 13/15 (no-revoke `complete`) but row 16 (`complete`, `decoupledEnabled=true`, `revokeEmbedded=true`) is NOT derived from A — it is the IDENTICAL defect class as M6, one cell over (a sibling cell in the SAME `complete` cursor-state group). §3.5 row 16 gives the `==0` and `==20` columns UNCONDITIONAL `awaiting_deploy` (HOLDER.md:589), reasoned '0021 not yet applied on a fresh DB → fingerprint ≠ → not in-sync → run deploy'. But A's §3.3a step-3 decoupled branch decides solely on `cursor.plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint` (HOLDER.md:441-445), with NO `applied_owner` input — and the holder's OWN derivation rule (HOLDER.md:565-566) says 'where A's outcome is conditional on the fingerprint-sync state … the cell is written conditionally'. Row 16's complete/decoupled outcome IS conditional on fingerprint-sync, yet the cell is written UNCONDITIONALLY — the holder violated its own rule. ADJUDICATOR SOURCE-VERIFICATION: `LiveFingerprint`/`RecordSchemaFingerprint` read/write ONLY the `striatumd.schema_state` singleton (`schema_drift.go:145-161`/`:171-195`; grep: zero `owner_bundle_meta` references), so `schema_state.fingerprint` and `owner_bundle_meta`/`applied_owner` are orthogonal tables; the in-sync row-16 cell (`schema_state.fingerprint == ExpectedFingerprint()` AND `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent / version-20 DB) IS constructible — exactly as the holder constructs the degenerate 13/`==0`-in-sync cell (HOLDER.md:636-638) — and a parametrized F18 over all 64 cells WILL construct it. For that cell W returns nil (`owner.go:145,151-153`), A returns nil (serve verify-only on the decoupled complete branch), but §3.5 says `awaiting_deploy`. Therefore F18 — the required executable matrix asserting the EXACT §3.5 outcome — is a FALSE ORACLE for the in-sync row-16 `==0`/`==20` cells (and F18 only adds the in-sync/out-of-sync sub-dimension for rows 13/15, NOT 16, HOLDER.md:855), OR the build must smuggle in an unstated `applied_owner`-dependent guard contradicting the W→A-independence claim. The premise the table imports — `owner_bundle_meta < 21 ⟹ fingerprint mismatch` — is the cross-table coupling A neither reads nor verifies, precisely the premise the M6 fix was meant to eliminate. It is the SEED's warned-of M7 (SEED.md: 'a re-scaffolded revision that fixes the `complete` rows but leaves … a sibling cell incoherent will spawn an M7 — close the class fully across ALL cursor rows'); the class is NOT closed by derivation. The holder's reachability rebuttal ('complete + revoke + applied_owner < 21 means 0021 pending, so the cell is corrupt/unreachable') is preempted by BOTH falsifiers and does not rescue the written spec: §3.5 promises exact outcomes for ALL 64 cells; the specified A predicate cannot produce the conservative halt because it does not read `applied_owner`; the holder relies on the very orthogonality it would have to deny; and `CheckDeployActivation` on serve-boot does NOT run `VerifyStoredTranscript` (the DB-stamp verifier is specified on deploy resume/finalizer step 0 only, HOLDER.md:269-275,497-515), so no stated mechanism detects the inconsistency before serving. NOT a SAFETY hole (row 16 is decoupled → the verify path never reaches the legacy `:399` writer; Invariant B holds — both falsifiers concede) and NOT a regression of any carry-forward (the M3 bypass stays closed, the BC-N2 `==20` edge and the `>=21` rule intact, `Required` not advanced) — but MATERIAL (the 64-cell table is not fully derived from W and A, and F18, the executable named requirement and the carry-forward proof harness, is a false oracle / would smuggle an unstated guard) and STANDS UNREBUTTED → needs_revision. Required fix (choose ONE coherent contract and propagate it through §1.3, §3.3a, §3.5, §4.5, F18): (1) MAKE ROW 16 `==0`/`==20` CONDITIONAL on the same A3 complete/decoupled fingerprint predicate — 'SERVE-verify if in-sync, else `awaiting_deploy`' (and, for full derivation, write the `>=21` revoke-embedding cell conditional too) — documenting that the normal reachable pre-0021 state is out-of-sync; or (2) ADD AN EXPLICIT CONSISTENCY GUARD before A serves (`cursorState == complete && revokeEmbedded && applied_owner < 21` → typed inconsistency halt, or a complete-boot stored-transcript DB-stamp verification proving the terminal owner step committed), require F18 to assert that guard, and NARROW the W→A-independence claim to reflect it. Until one is specified the executable decision table is not fully derived/coherent. The fix must NOT re-collapse the row-1 fresh-DB serve, weaken the M3 A0 gate, regress the BC-N2 `==20` edge, or advance `RequiredOwnerBundleVersion`."
    affected_invariants: ["boot-path decision table COMPLETE and DERIVED from W and A at cursorState=complete in the decoupled/revoke-embedding cells (row 16) — §3.5, §3.3a, §1.3, §4.5, and F18 must agree", "the F18 matrix oracle matches the `applied_owner`-independent A predicate for the in-sync row-16 `==0`/`==20` cells", "the holder's own derivation rule (HOLDER.md:565-566) applied to EVERY fingerprint-conditional cell, not only rows 13/15", "the table imports no cross-table `owner_bundle_meta < 21 ⟹ fingerprint mismatch` premise A does not read/verify"]
    source_refs: ["dialogue:2", "dialogue:3"]
  - id: M5
    severity: high
    posture: design
    status: answered
    challenge: "INTACT (row 1) — carried forward verbatim, unregressed; both falsifiers confirm no regression. The `applied_owner` dimension stays split `{0/no authority, 1..19 authority shortfall, ==20, >=21}` covering all of ℕ; W = `CheckOwnerBundleWatermark` returns nil for `applied_owner == 0` (the fresh/single-role bootstrap, `owner.go:145`, source-verified BEFORE the `:148-150` shortfall), halts `awaiting_owner_ddl` for `1 <= applied_owner < 20`, returns nil for `==20`, and `>=21` is the forward barrier (b) (HOLDER.md:538-546,572-575). Cell 1/`==0` SERVES-legacy the fresh-DB bring-up; cell 1/`==20` is the inert-landing re-boot; F18 asserts both, F18a (`T-deploy-fresh-db-bootstrap-serves`) pins the fresh-DB serve. The M6 derivation propagates the split through the `complete` rows WITHOUT re-collapsing row 1. **M5(row-1) INTACT.** (M7 is in the `complete`/decoupled/revoke-embedding cell, not the row-1 fresh-DB serve.)"
    affected_invariants: ["the `{0/1..19/==20/>=21}` owner-watermark split", "cell 1/`==0` serves the fresh-DB bring-up; cell 1/`==20` inert-landing", "F18/F18a dual assertion", "M6 derivation does not re-collapse row 1"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M3
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm no regression. The `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config halt remains HOISTED to step 0 of `CheckDeployActivation`, before any cursor-state branch, firing for EVERY cursor state incl. `complete` (barrier a, HOLDER.md:434-437) and in EVERY `applied_owner` column that passes W incl. `0`: cells 2/6/10/14 still halt `awaiting_deploy_config` at A0 (HOLDER.md:575,579,583,587), preserving the asymmetry (the serve cell is no-revoke only). The no-revoke `complete` residual stays a pre-`ApplyMigrations` DB-untouched PURE-READ comparison; the tightened Invariant B + F17/F11(g)/F18 carry forward. **M3 INTACT.** (M7 is in the no-revoke-vs-revoke DECOUPLED cell (row 16, decoupled ON), where step 0 does NOT fire and A reaches the decoupled complete branch — it does NOT re-open the M3 legacy mutate+self-record bypass, since row 16 is decoupled and never reaches the legacy `:399` writer; both falsifiers confirm.)"
    affected_invariants: ["hoisted step-0 config gate (every cursor state, every `applied_owner` column that passes W)", "no revoke-embedding binary reaches the legacy `:399` writer (M7 is a decoupled serve-verify cell, no `:399`)", "the no-revoke `complete` pre-ApplyMigrations pure-read comparison", "cells 2/6/10/14 still halt"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M4
    severity: medium
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. F16a (`TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, synthetic list / injectable-FS, rollout step 2) drives the exclusion filter WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (`TestOwnerDDLApplyExcludesProductionRevokeBundle`, step 7) asserts the production embed/listing split + houses the forced FMA-007 self-heal pgtest reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError` (`owner.go:367-374`) (HOLDER.md:383-399,853). M6/M7 do not touch F16. **M4 INTACT.**"
    affected_invariants: ["F16a synthetic phase builds green before 0021 exists", "F16b production phase + the forced self-heal", "the M2 filter unchanged by the F16 staging"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: M1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers find no regression. `VerifyStoredTranscript(plan_hash)` (§3.4a) checks the ENTIRE stored transcript `sha256` against the binary's embedded bytes ⇒ `deploy_plan_binary_mismatch`; already-applied entries also verify the DB stamps ⇒ `deploy_plan_db_stamp_mismatch`; the SAME verifier runs on every resume AND as finalizer step 0 (HOLDER.md:269-275,497-515,852). M6/M7 concern the boot-path table, not the deployer verifier. F15 + F14 carry forward. **M1 INTACT.** (Note: falsifier_2 correctly observes that `VerifyStoredTranscript` is specified on deploy resume / finalizer step 0, NOT on ordinary serve-boot over an already-`complete` cursor — which is WHY the M7 fix would need either a conditional cell or a NEW boot-time guard; this is a gap exposed by M7, not an M1 regression.)"
    affected_invariants: ["full stored-transcript byte + DB-stamp verification on resume AND finalizer step 0", "typed mismatch halts", "the deployer finalizer gated by VerifyStoredTranscript step 0"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: M2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The single non-revoke filter `isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)` + the split loader `OwnerDDLApplyBundles()` bind across every `owner-ddl apply` route incl. the FMA-007 self-heal `ReapplyAllOwnerBundles` (in-loop guards) + the nil-fallback split; `OwnerBundles()` (full, includes 0021) is kept ONLY for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable` (HOLDER.md:356-381,799-802). M6/M7 do not touch the filter. **M2 INTACT.**"
    affected_invariants: ["0021 unreachable through every owner-ddl apply route incl. the FMA-007 self-heal", "embed/listing split", "0021 committed only as the terminal deploy step"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: BC-N1
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The runtime-owned `deploy_plan` table (migration 0044) storing the IMMUTABLE ordered transcript keyed by `plan_hash`, materialized in the same tx that sets `deploy_cursor → in_progress(0)` BEFORE step 0; resume loads `deploy_plan[cursor.plan_hash]` and never recomputes `BuildPlan`; §1.3 + the transcript-enumerated doctor + F14 (HOLDER.md:245-278). M6/M7 touch only the boot-path table's `complete` rows. **BC-N1 INTACT.**"
    affected_invariants: ["BC-N1 immutable plan identity across resume", "stable (plan_hash, step_index) receipt key", "resume off the stored transcript"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: BC-N2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm. The universal incomplete-deploy edge halts `awaiting_deploy` DB-untouched on every non-`complete` cursor at `applied_owner == 20` (the `==20` column's non-complete cells 5/7/9/11 verbatim, HOLDER.md:578-585); F11(e)/(f) + the extended `G-old-binary-refuse` carry forward. M5/M6/M7 concern the ORTHOGONAL owner-watermark dimension at W and the `complete`-cursor cells at A, NOT the BC-N2 non-complete `deploy_cursor` edge — both falsifiers explicitly note M7 is in the `complete`/decoupled/revoke-embedding cell (row 16), not the non-complete edge. **BC-N2 INTACT.**"
    affected_invariants: ["BC-N2 universal pre-revoke deploy_cursor-authoritative edge at applied_owner == 20", "incomplete (non-complete) cursor never serves", "M7 does not regress the non-complete edge"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C1
    severity: critical
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede. The `finalizing` state + the idempotent finalizer (VerifyStoredTranscript step 0 → guarded `complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST) + the §1.3 resumable-finalization row + F10 (HOLDER.md:245-278,495-503). M6/M7 touch neither the finalizer nor the legacy writer's enforcement — they concern the §3.5 table's `complete` rows the finalizer's `complete` cursor lands in. **C1 INTACT.**"
    affected_invariants: ["finalization boundary (no complete with missing receipt/fingerprint)", "idempotent finalizer gated by VerifyStoredTranscript step 0", "section 1.3 disambiguation"]
    source_refs: ["dialogue:1", "dialogue:3"]
  - id: C2
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers confirm. `CheckDeployActivation` before `ApplyMigrations` in both boot paths (mutating nothing); the typed `awaiting_deploy`/`awaiting_deploy_config` halts; the forward-watermark rule firing at `applied_owner >= 21` (barrier b, the `>=21` column); and `RequiredOwnerBundleVersion = 20` (`owner.go:35`, source-verified NOT advanced) (HOLDER.md:438-452,549-552,729-735). The M6 derivation makes the `==0` column mirror `==20` in the `complete` rows; it does NOT advance `Required`, alter the watermark, or change the `applied >= 21` forward rule. **C2 INTACT.**"
    affected_invariants: ["fail-closed CheckDeployActivation edge before ApplyMigrations", "forward-watermark rule at applied >= 21 (barrier b)", "RequiredOwnerBundleVersion KEPT at 20, not advanced"]
    source_refs: ["dialogue:1", "dialogue:2", "dialogue:3"]
  - id: C3
    severity: high
    posture: design
    status: answered
    challenge: "INTACT — carried forward verbatim, unregressed; both falsifiers concede the mechanism. Bundle 0021 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) identified by `DDLRevokeOwnerBundleVersion = 21`, special-cased out of the owner prefix, applied as the TERMINAL plan step after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE granted by 0018); `LatestOwnerBundleVersion`/`Required` stay 20; F12/`G-revoke-last` (HOLDER.md:356-381,483-493). **C3 mechanism INTACT.** (Note: M7's row-16 defect arises BECAUSE the table imports the C3 reachability fact — 'a completed revoke deploy means applied_owner >= 21' — into A without A verifying the owner-bundle stamp before serving; that is an M7 derivation defect, NOT a regression of the C3 revoke-last mechanism itself.)"
    affected_invariants: ["two-role owner/runtime boundary", "revoke-last terminal ordering", "reconcile-while-CREATE-held", "0021 deploy-terminal only"]
    source_refs: ["dialogue:1", "dialogue:3"]
branches:
  design: blocked
---

# COLLABORATION LEDGER — RFC 0142 P4 design run (v8 / REVISION), cycle 1

author: adjudicator-author-001

**Verdict:** `needs_revision`

> Scope of this adjudication: the curated dialogue trajectory only — the **revised
> (v8)** Holder's `HOLDER.md` P4 implementation spec and the two falsifiers'
> `FALSIFIER.md` challenges — plus the `SEED.md` charter, with the **v7** `HOLDER.md`
> and the **v7** collaboration ledger (its M6 finding + §4 "What the revision must fix"
> + §5 "What already cleared") as context for what the revision had to fix. RFC 0142 is
> accepted (D258); this judges the **P4 implementation shape**, not the five-layer
> design. A clearing verdict would be `accept` or `accept_with_findings`; this is
> neither. This is the gate's **single allowed revision cycle**, so the gate ends
> **unCleared** — judged exactly on the merits, not graded on the consequence.

## 1. Decision

`needs_revision`. The revision is real, well-anchored, and it **genuinely resolves the
single binding cycle-1 finding M6 at the NAMED cells** — §3.5 rows 13 and 15 (the
no-revoke `complete` rows) — and both falsifiers independently concede the row-13/15
repair, and it **carries M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 forward
intact** (both falsifiers concede no regression). **But** a clearing verdict also requires
**(a) the proactive-completeness boot-path decision table to be COMPLETE and EXECUTABLE —
every cell DERIVED FROM W and A, an F18 oracle matching the §3.3a A predicate, the
legitimate fresh-DB cells still serving — and (b) no new material challenge standing
unrebutted** — and **both fail on the same new defect (M7)**: the M6 mechanical derivation
closed rows 13/15 but **row 16 (`complete`, decoupled, revoke-embedding) is not derived
from A**. **Both** falsifiers, from independent lenses, **converged** on it, source-verified,
and it stands unrebutted.

- **M6 (the M5 split not propagated through the `complete` rows) — RESOLVED at rows
  13/15.** §0.2 states the load-bearing **INVARIANT W→A-INDEPENDENCE** explicitly (once W
  passes, A reads `cursorState`/`decoupledEnabled`/`revokeEmbedded`/`cursor.plan_hash`/the
  recorded `schema_state.fingerprint`, **never** `owner_bundle_meta`/`applied_owner`), so
  the `==0` and `==20` columns are identical in every cursor row. §3.5 rows 13 and 15 in
  the `==0` column are now conditional — "serve if in-sync, else `awaiting_deploy`" —
  identical to `==20` (`HOLDER.md:586,588`); §4.5 and the F18 spy list both enumerate the
  SAME four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`}
  (`HOLDER.md:804-832,855`); §3.5 carries the cross-row `==0` audit (`HOLDER.md:598-620`).
  I source-verified the orthogonality the fix rests on (run worktree HEAD `a970f546`;
  `git diff --stat 3f9d5734 HEAD -- go/pkg/db/` **empty**): `schema_drift.go:145-161`
  `LiveFingerprint` and `:171-195` `RecordSchemaFingerprint` read/write **only** the
  `schema_state` singleton — grep confirms **zero** `owner_bundle_meta`/`applied_owner`
  references in `schema_drift.go`. **The rows-13/15 repair is real and source-accurate.**
- **M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, C3 — INTACT.** Carried forward
  verbatim; both falsifiers concede no regression. The M6 derivation does not re-collapse
  the row-1 fresh-DB serve (`owner.go:145`, source-verified), weaken the M3 A0 config gate
  (cells 2/6/10/14 still halt `awaiting_deploy_config`), regress the BC-N2 `applied_owner
  == 20` non-complete edge (cells 5/7/9/11 verbatim), or advance `Required` (`owner.go:35`
  = 20, source-verified).

**But a clearing verdict also requires the decision table COMPLETE/EXECUTABLE/DERIVED and
no new material challenge standing.** Both fail on **M7** (the SEED's anticipated next
finding):

- **M7 — row 16 is asserted, not derived from A; F18 is a false oracle for the in-sync
  `complete`/decoupled/revoke-embedding cell.** §3.5 row 16 gives the `==0` and `==20`
  columns **unconditional** `awaiting_deploy` (`HOLDER.md:589`), reasoned "0021 not yet
  applied → fingerprint ≠". But A's §3.3a step-3 decoupled branch decides **solely** on
  `cursor.plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint`
  (`HOLDER.md:441-445`), with **no `applied_owner` input** — and the holder's **own
  derivation rule** (`HOLDER.md:565-566`) says a fingerprint-conditional outcome must be
  "written conditionally". Row 16's outcome IS conditional on fingerprint-sync, yet the
  cell is written unconditionally. Because `schema_state` and `owner_bundle_meta` are
  orthogonal (source-verified), the in-sync row-16 cell is constructible — exactly as the
  holder constructs the degenerate 13/`==0`-in-sync cell — and for it A serves verify-only
  while §3.5 says halt.

Because **M7 is a material new challenge standing unrebutted** and the decision table is
therefore not complete/derived in the `complete`/decoupled/revoke-embedding cells, the gate
does **not** clear. This is the gate's single allowed revision cycle, so it ends
**unCleared**.

## 2. Per-finding / per-challenge ledger

| # | Source | What is judged | Material? | Resolved / rebutted? | Disposition |
| --- | --- | --- | --- | --- | --- |
| **M6** | cycle-1 finding / both falsifiers' revision-check | rows 13/15 `==0` made conditional == `==20`; §0.2 invariant; §4.5/F18 4-cell consistency; cross-row audit (`HOLDER.md:150-158,586-588,598-620,804-832,855`) | — | **Yes — RESOLVED at rows 13/15** (both falsifiers concede; adjudicator source-verified the orthogonality) | **RESOLVED (rows 13/15)** |
| **M7** | falsifier_1 (`dialogue:2`) + falsifier_2 (`dialogue:3`) — **independent convergence** | row 16 (`complete`, decoupled, revoke-embedding) `==0`/`==20` unconditional `awaiting_deploy` not derivable from the owner-watermark-blind A predicate; F18 a false oracle / unstated guard smuggled (`HOLDER.md:441-445,565-566,589,855`) | **Yes** — the 64-cell table is not fully derived from W and A; F18 (the executable named requirement / proof harness) is a false oracle; spec edit required | **No** — no holder rebuttal; the "complete+revoke+`<21` is unreachable" reading is preempted by both falsifiers and by the holder's own orthogonality construction | **STANDS → needs_revision** |
| **M5** | carried forward / both falsifiers | the `{0/1..19/==20/>=21}` split; cell 1/`==0` serve; F18/F18a (`HOLDER.md:538-546,572-575`; `owner.go:145`) | — | **Yes — INTACT (row 1)** (source-verified `owner.go:145`) | **INTACT** |
| **M3** | carried forward / both falsifiers | hoisted step-0 config gate (cells 2/6/10/14) + no-revoke `complete` pure-read + tightened Invariant B + F17/F11(g) (`HOLDER.md:434-437,575,587`) | — | **Yes — INTACT** (M7 is the decoupled cell, does not re-open the legacy bypass) | **INTACT** |
| **M4** | carried forward / both falsifiers | F16a synthetic (step 2) + F16b production + forced self-heal (step 7) (`HOLDER.md:383-399`) | — | **Yes — INTACT** | **INTACT** |
| **M1** | carried forward / both falsifiers | `VerifyStoredTranscript` on resume AND finalizer step 0; typed halts (`HOLDER.md:269-275,497-515`) | — | **Yes — INTACT** (M7 exposes that it does NOT run on serve-boot — a gap the M7 fix must close, not an M1 regression) | **INTACT** |
| **M2** | carried forward / both falsifiers | single non-revoke filter + in-loop guards + embed/listing split (`HOLDER.md:356-381`) | — | **Yes — INTACT** | **INTACT** |
| **BC-N1** | carried forward / both falsifiers | immutable `deploy_plan`; resume off the stored transcript; §1.3 + doctor + F14 (`HOLDER.md:245-278`) | — | **Yes — INTACT** | **INTACT** |
| **BC-N2** | carried forward / both falsifiers | universal non-`complete` edge at `applied_owner == 20`; cells 5/7/9/11 verbatim; F11(e)/(f) (`HOLDER.md:578-585`) | — | **Yes — INTACT** (M7 is the `complete`/decoupled cell, not the non-complete edge) | **INTACT** |
| **C1** | carried forward / both falsifiers | `finalizing` + idempotent finalizer + §1.3 row + F10 (`HOLDER.md:245-278,495-503`) | — | **Yes — INTACT** | **INTACT** |
| **C2** | carried forward / both falsifiers | `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `Required = 20` (`HOLDER.md:438-452`; `owner.go:35`) | — | **Yes — INTACT** (source-verified `Required = 20`) | **INTACT** |
| **C3** | carried forward / both falsifiers | 0021 special-cased + terminal + revoke-last; F12/`G-revoke-last` (`HOLDER.md:356-381,483-493`) | — | **Yes — mechanism INTACT** (M7 imports a C3 reachability fact into A — a derivation defect, not a C3 regression) | **INTACT** |

## 3. Why the verdict turns the way it does

### M6 — the owner-watermark split propagated through rows 13/15: RESOLVED

The v7 break was that §3.5 rows 13/15 gave the `==0` column a different outcome than
`==20`, contradicting the holder's own `applied_owner`-independent A predicate, so the
executable F18 matrix was a false oracle for the in-sync `complete`/`==0` cells. The v8
revision closes that exactly as the v7 ledger §4 prescribed under contract (1): §0.2 states
the **INVARIANT W→A-INDEPENDENCE** explicitly and anchors it to the source; §3.5 rows 13
and 15 in the `==0` column become conditional "serve if in-sync, else `awaiting_deploy`",
identical to `==20`; the degenerate 13/`==0`-in-sync idempotent `:399` rewrite is added to
**both** §4.5 and the F18 spy list, so the four `:399`-reaching cells are enumerated
identically; and §3.5 carries the explicit cross-row `==0` audit.

**Both falsifiers explicitly concede the row-13/15 repair.** I source-verified the
orthogonality the fix rests on against the run worktree (HEAD `a970f546`;
`git diff --stat 3f9d5734 HEAD -- go/pkg/db/` **empty**, so the holder's anchors against
`3f9d5734` hold): `schema_drift.go:145-161` `LiveFingerprint` reads only
`SELECT … FROM striatumd.schema_state …`, `:171-195` `RecordSchemaFingerprint` UPSERTs only
that singleton, and grep finds **zero** `owner_bundle_meta`/`applied_owner` references in
`schema_drift.go`; `owner.go:145` serves `applied == 0` before the shortfall; `owner.go:35`
keeps `Required = 20`. **The rows-13/15 fix is genuinely resolved and source-accurate.**

### M7 — row 16 asserted, not derived; false F18 oracle in the `complete`/decoupled/revoke-embedding cell: STANDS (the blocker)

This is the load-bearing reason the gate does not clear, and it is the strongest signal a
falsification gate can produce: **two falsifiers, from two independent lenses
(decoupling-boundary/decision-table; carry-forward/regression), converged on the same
source-verified defect and each made it their explicit blocker.** I verified it directly:

- **The holder's structural claim and its own derivation rule.** §3.5 derives each
  W-passing cell from A's outcome, and the holder writes (`HOLDER.md:565-566`): "where A's
  outcome is conditional on the fingerprint-sync state … the cell is written conditionally."
  A's `complete`/decoupled branch is `cursor.plan_hash == expected` +
  `LiveFingerprint == ExpectedFingerprint` (`HOLDER.md:441-445`), with no `applied_owner`
  input.
- **Row 16 violates the rule.** Row 16 (`complete`, `decoupledEnabled=true`,
  `revokeEmbedded=true`): `==0` and `==20` are written **unconditional** `awaiting_deploy`
  (`HOLDER.md:589`), reasoned "0021 not yet applied → fingerprint ≠ → not in-sync." But
  that "fingerprint ≠" is inferred from the owner watermark — not read by A. A's outcome
  for row 16 is **conditional** on fingerprint-sync, so by the holder's own rule the cell
  must be written conditionally.
- **Source verification — the in-sync row-16 cell is constructible.** Because
  `schema_state` (what A reads) and `owner_bundle_meta` (= `applied_owner`, what W reads)
  are orthogonal tables (`schema_drift.go:145-161`/`:171-195`; grep: zero cross-references),
  a DB with `owner_bundle_meta` absent/at 20 **can** carry
  `schema_state.fingerprint == ExpectedFingerprint()` and a `complete` `deploy_cursor` with
  `cursor.plan_hash == expected`. The holder **itself** relies on exactly this orthogonality
  to construct the degenerate 13/`==0`-in-sync cell (`HOLDER.md:636-638`). A parametrized
  F18 over all 64 cells **will** construct the in-sync row-16 cell.
- **The fork is a real defect either way.** For that cell W returns nil (`owner.go:145,
  151-153`) and A returns nil (serve verify-only on the decoupled complete branch), but
  §3.5 says `awaiting_deploy`. So either (1) **F18 is a false oracle** for the in-sync
  row-16 `==0`/`==20` cells (and F18 only adds the in-sync/out-of-sync sub-dimension for
  rows 13/15, not 16, `HOLDER.md:855`), or (2) the build must add an **unstated
  `applied_owner`-dependent guard**, contradicting the W→A-independence claim. Both are
  material. The premise the table imports — `owner_bundle_meta < 21 ⟹ fingerprint
  mismatch` — is precisely the cross-table coupling the M6 fix was meant to eliminate.

The holder's anticipatable rebuttal — that `complete + revoke-embedding + applied_owner <
21` is unreachable/corrupt, so the `awaiting_deploy` halt is conservative — is **preempted
by both falsifiers** and does not rescue the written spec: §3.5 promises exact executable
outcomes for **all 64 cells** and does not mark the cell impossible; the specified A
predicate cannot produce the conservative halt because it does not read `applied_owner`;
the holder relies on the very orthogonality it would have to deny; and
`CheckDeployActivation` on serve-boot does **not** run `VerifyStoredTranscript` (the only
DB-stamp verifier, specified on deploy resume / finalizer step 0 only), so no stated
mechanism detects the inconsistency before serving. If the intended rule is "a `complete`
revoke-embedding cursor with `applied_owner < 21` is inconsistent and must halt", that is a
**new guard** that must be specified in A, with F18 asserting it.

This is **not** a SAFETY hole — row 16 is decoupled, so the verify path never reaches the
legacy `connection.go:399` writer; Invariant B holds, and both falsifiers concede this — and
it is **not** a regression of any carry-forward (the M3 bypass stays closed, the BC-N2
`==20` edge and the `>=21` rule are intact, `Required` is not advanced). But it **is** a
decision-table **executability / completeness** failure: F18 is the **executable named
requirement** the SEED mandated and the **carry-forward proof harness**, and as written it
cannot be both true to the §3.3a predicate and true to the §3.5 row-16 cell. It is exactly
the failure mode the SEED warned the proactive table existed to preempt: **"a re-scaffolded
revision that fixes the `complete` rows but leaves … a sibling cell incoherent will spawn
an M7 — close the class fully across ALL cursor rows."** The holder fixed the no-revoke
`complete` rows (13/15) but left a **sibling cell in the same `complete` cursor-state
group** (row 16, revoke-embedding decoupled) with the identical asserted-not-derived defect.
The class is **not** closed by derivation. It is **material** and **stands unrebutted** (no
holder rebuttal artifact this cycle; the spec text affirms the asserted cell). **M7 forces
`needs_revision`.**

### M5(row-1) / M3 / M4 / M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3 — INTACT; the decision table — NOT COMPLETE

All ten carry-forward findings are preserved verbatim and unregressed, and both falsifiers
concede each. The M6 derivation re-buckets nothing at W and does not weaken A's M3 gate:
**M5(row-1)** — cell 1/`==0` still serves (`owner.go:145`, source-verified). **M3** — the
hoisted A0 config gate still fires for cells 2/6/10/14 in every column passing W; M7 is the
decoupled cell where A0 does not fire, and it never reaches the legacy `:399` writer, so the
M3 bypass stays closed. **M4** — F16a/F16b unchanged. **M1** — `VerifyStoredTranscript` on
resume and finalizer step 0 (M7 exposes that it does **not** run on ordinary serve-boot — a
gap the M7 fix must close, not an M1 regression). **M2** — the non-revoke filter unchanged.
**BC-N1** — the immutable `deploy_plan`. **BC-N2** — the non-`complete` edge at `applied_owner
== 20` (M7 is the `complete`/decoupled cell, not this edge). **C1** — the `finalizing`
finalizer. **C2** — `CheckDeployActivation` before `ApplyMigrations`, `Required = 20`
(source-verified), forward-watermark at `applied >= 21`. **C3** — 0021 special-cased +
terminal + revoke-last (M7 imports the C3 *reachability fact* into A — a derivation defect,
not a regression of the revoke-last mechanism).

**Decision table — NOT COMPLETE.** Universal Invariant B itself is proven (no unsafe legacy
write in any cell — the four `:399`-reaching cells are correctly enumerated and consistent
between §4.5 and the F18 spy list), the row-13/15 owner-watermark coherence is fixed, the M3
cell (2/6/10/14), the BC-N2 `==20` cells, and the shadow-mode drift-gate fall-through are
covered, and the cross-row audit confirms rows 1–15 coherent. **But** the table is **not
fully derived/executable at `cursorState = complete` in the decoupled/revoke-embedding cell
(row 16)**: §3.5 gives the `==0`/`==20` columns an unconditional `awaiting_deploy` the
specified `applied_owner`-independent A predicate cannot produce for the constructible
in-sync sub-case, and F18 does not sub-dimension row 16. A table whose own predicate and
oracle disagree for a set of cells is not the COMPLETE, executable table the SEED required
(M7).

## 4. What the revision would have needed to fix (exact) — for the record

A re-scaffolded revision would have cleared the gate only by pinning the single item below,
while keeping M6 (rows 13/15) and M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 intact and
unregressed.

1. **Derive row 16 (`complete`, decoupled, revoke-embedding) from A so §3.5, §3.3a, §1.3,
   §4.5, and F18 all agree (M7 — the load-bearing fix, decision-table executability).**
   Choose **one** coherent contract and apply it everywhere:
   - **(1) Make row 16 conditional on the A3 fingerprint predicate.** Write the `==0` and
     `==20` cells "**serve if in-sync, else `awaiting_deploy`**" (and, for full mechanical
     derivation, write the `>=21` revoke-embedding cell conditionally too, since A's
     decoupled-complete outcome is fingerprint-conditional in every column), documenting
     that the normal reachable pre-0021 state is out-of-sync. Add the in-sync row-16
     sub-dimension to F18 (mirroring the rows-13/15 treatment) and to §4.5 where relevant.
     This keeps the W→A-independence claim true across all `complete` rows. **OR**
   - **(2) Add an explicit consistency guard before A serves.** Detect the inconsistent
     shape (`cursorState == complete && revokeEmbedded && applied_owner < 21`, or a
     `schema_state` fingerprint whose terminal owner-bundle DB stamp is absent/stale) and
     halt a typed error **before** serving; have A (or a complete-boot stored-transcript
     DB-stamp verification) run enough verification to prove the terminal owner step
     committed; make F18 assert that typed halt; and **narrow** the W→A-independence claim
     to reflect the new guard.

> The fix is local to row 16 (the decoupled revoke-embedding `complete` cell); it must not
> re-collapse the row-1 fresh-DB serve, weaken the M3 config gate (cells 2/6/10/14 must
> still halt `awaiting_deploy_config` at A0), regress the BC-N2 `applied_owner == 20` edge,
> or advance `RequiredOwnerBundleVersion`.

## 5. What already cleared (carry forward — do NOT regress or re-litigate)

- **M6 (rows 13/15) — RESOLVED this cycle** (§3 above): preserve the §0.2
  W→A-independence invariant; rows 13 and 15 `==0` conditional "serve if in-sync, else
  `awaiting_deploy`" matching `==20`; the degenerate 13/`==0`-in-sync `:399` rewrite in
  BOTH §4.5 and the F18 spy list (the four cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`,
  13-in-sync/`==20`}); the cross-row `==0` audit. (The M7 fix only derives row 16; it must
  not re-open the rows-13/15 coherence.)
- **M5 owner-watermark dimension split, ROW 1 — intact**: the `{0/1..19/==20/>=21}` split;
  cell 1/`==0` serves the fresh-DB bring-up; F18/F18a.
- **M3 complete-cursor activation guard — intact**: the hoisted step-0 config gate (cells
  2/6/10/14), the no-revoke `complete` pure-read comparison, the tightened Invariant B,
  F17/F11(g)/F18.
- **M4 F16 phase-aware split — intact**: F16a (synthetic, step 2) + F16b (production +
  forced self-heal, step 7).
- **M1 full-transcript verifier — intact**: `VerifyStoredTranscript` on every resume and as
  finalizer step 0; the typed mismatch halts; F15 + F14. (The M7 fix may need to extend a
  stored-transcript / DB-stamp check to serve-boot under contract (2).)
- **M2 non-revoke filter — intact**: `DDLRevokeOwnerBundleVersion = 21`, `isNonRevokeBundle`,
  `OwnerDDLApplyBundles()`, the in-loop guards, the nil-fallback + embed/listing split.
- **BC-N1 immutable transcript / resume-off-stored-plan — intact**: the 0044 `deploy_plan`
  table, materialize-before-step-0, §1.3, the transcript-enumerated doctor, F14.
- **BC-N2 universal non-complete-cursor edge — intact**: the `revokeEmbedded`-independent
  halt at `applied_owner == 20` and F11(e)/(f). (M7 is the `complete`/decoupled cell; it
  does not weaken the non-complete edge.)
- **C1 finalization boundary — intact**: the `finalizing` state + idempotent finalizer +
  §1.3 row + F10, with M1's pre-finalizer `VerifyStoredTranscript` step 0.
- **C2 fail-closed activation edge — intact**: `CheckDeployActivation` before
  `ApplyMigrations`, the typed halts, the forward-watermark rule (fires at `applied >= 21`),
  and `RequiredOwnerBundleVersion = 20` (not advanced).
- **C3 revoke-last ownership mechanism — intact**: 0021 special-cased + terminal + the
  reconcile-while-CREATE-held ordering + F12/`G-revoke-last`. (M7 imports the C3
  reachability fact into A — fix the derivation, not the mechanism.)
- **Q4 — resolved** (v1–v7): plain verb, bootstrapping paradox dispositive, run-shape door.
- **Q3 mid-step resumability body — sound**: Q3-A/Q3-B, the §1.1 step taxonomy. M6/M7
  concern the boot-path decision table's owner-watermark × cursor-state dimension, not the
  per-step schema/cursor body.
- **Decoupling shape — sound** (M3-hardened): shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark interlock +
  P3 drift gate, the embed-FS-derived plan, the hash-chained per-step receipt.
- **Scope discipline held**: P5 deferred; full capability revocation named as a follow-up
  beyond P4; the local-first boundary respected.

## 6. Gate decision

The ledger verdict — not falsifier completion — clears the phase gate. **M6 is genuinely
resolved at the named rows 13/15** (both falsifiers concede), and **M5(row-1), M3, M4, M1,
M2, BC-N1, BC-N2, C1, C2, and C3 are carried forward intact** (both falsifiers concede no
regression). But a clearing verdict also requires **the boot-path decision table complete
and executable (every cell derived from W and A, the F18 oracle matching the §3.3a
predicate, the legitimate fresh-DB cells still serving) and no new material challenge
standing** — and **M7** is a material challenge standing unrebutted, source-verified against
the run worktree and landed independently by **both** falsifiers:

- **M7**: the M6 derivation was carried through rows 13/15 (no-revoke `complete`) but **not**
  to row 16 (revoke-embedding, decoupled, `complete`) — §3.5 gives the `==0`/`==20` columns
  an unconditional `awaiting_deploy` the holder's own `applied_owner`-independent A predicate
  cannot produce for the constructible in-sync sub-case, violating the holder's own
  derivation rule (`HOLDER.md:565-566`), so the executable F18 matrix is a false oracle for
  the in-sync `complete`/decoupled/revoke-embedding cell. The holder's "complete + revoke +
  `<21` is unreachable/corrupt" reading does not hold against §3.5's "exact outcome for all
  64 cells", the `applied_owner`-independent A predicate, and the holder's own orthogonal
  construction of the 13/`==0`-in-sync cell.

**Verdict: `needs_revision`.** This is the gate's single allowed revision cycle, so the
gate ends **unCleared** — judged on the merits. A clearing verdict would require a
re-scaffolded revision that derives row 16 from A under one of the two coherent contracts
(§4 above — with §3.5, §3.3a, §1.3, §4.5, and F18 made consistent) without regressing M6
(rows 13/15) or M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3.

<sub>Adjudicator collaboration ledger for the RFC 0142 P4 falsification-gate design run,
v8 / REVISION, cycle 1. Inputs:
`docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/holder/HOLDER.md`,
`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, `SEED.md`;
context: v7 `HOLDER.md` + v7 `COLLABORATION_LEDGER_cycle_1.md` (its M6 finding + §4 + §5).
Adjudicator source verification against the run worktree (HEAD `a970f546`;
`git diff --stat 3f9d5734 HEAD -- go/pkg/db/` empty, so the holder's `3f9d5734` anchors
hold): `go/pkg/db/owner.go:124-154` (the `applied == 0` fresh-DB serve exception before the
`< RequiredOwnerBundleVersion` shortfall), `owner.go:23/:35` (`Latest = Required = 20`),
`go/pkg/db/schema_drift.go:145-161` (`LiveFingerprint` reads only the `schema_state`
singleton), `:171-195` (`RecordSchemaFingerprint` writes only it) — grep confirms zero
`owner_bundle_meta`/`applied_owner` references in `schema_drift.go`, so A is genuinely
owner-watermark-independent; `go/pkg/db/connection.go` boot order W → ApplyMigrations →
drift gate → `RecordSchemaFingerprint` ("the only writer of schema_state"). HOLDER /
falsifier anchors as cited. Verdict vocabulary: accept | accept_with_findings |
needs_revision | reject.</sub>

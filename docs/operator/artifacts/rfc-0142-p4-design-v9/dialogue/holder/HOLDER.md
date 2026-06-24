# HOLDER (revised, v9) — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: holder-author-001

> **This is the NINTH REVISION (v9).** Design-v1 returned `needs_revision` with C1/C2/C3.
> v2 resolved C1 + C2. v3 resolved C3 (ownership transfer, revoke-last) and closed N1, then
> returned `needs_revision` on BC-N1 + BC-N2. v4 resolved BOTH BC-N1 and BC-N2 and returned
> `needs_revision` on M1 + M2. v5 resolved BOTH M1 and M2 and returned `needs_revision` on M3 +
> M4. v6 resolved BOTH M3 and M4 and returned `needs_revision` on M5. v7 resolved M5 row-1 and
> returned `needs_revision` on M6. v8 (`rfc-0142-p4-design-v8`) **resolved M6** — the M5
> `applied_owner` split propagated through the no-revoke `complete` rows (13/15); §0.2 states the
> W→A-independence invariant anchored to `schema_drift.go:145-161`/`:171-195`; the degenerate
> 13/`==0`-in-sync idempotent `:399` rewrite added to BOTH §4.5 AND the F18 spy list; the four
> `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} enumerated
> identically — BOTH v8 falsifiers AND the v8 adjudicator explicitly conceded the rows-13/15
> repair, and M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward intact — but returned
> `needs_revision` **again** (the gate's single allowed cycle) on one new finding, source-verified
> against the run worktree and landed INDEPENDENTLY by BOTH v8 falsifiers (falsifier_1
> decoupling-boundary/decision-table lens AND falsifier_2 carry-forward/regression lens converged
> on the same defect):
>
> - **M7 (LOAD-BEARING)** — §3.5 row 16 (`cursorState=complete`, `decoupledEnabled=true`,
>   `revokeEmbedded=true`) gave the `==0` and `==20` columns UNCONDITIONAL `awaiting_deploy`
>   (and the `>=21` column UNCONDITIONAL `SERVE-verify`), reasoned '0021 not yet applied →
>   fingerprint ≠ → not in-sync'. But A's §3.3a step-3 decoupled branch decides solely on
>   `cursor.plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint` — with NO
>   `applied_owner` input AND no `revokeEmbedded` input (step 0's config gate does not fire when
>   decoupled is ON) — and the holder's OWN derivation rule (v8 HOLDER.md:565-566) says 'where A's
>   outcome is conditional on the fingerprint-sync state … the cell is written conditionally'. Row
>   16's complete/decoupled outcome IS conditional on fingerprint-sync, yet the cell was written
>   UNCONDITIONALLY — the holder violated its own rule. The in-sync row-16 cell is constructible
>   (exactly as the holder constructs the degenerate 13/`==0`-in-sync cell, because `schema_state`
>   and `owner_bundle_meta` are orthogonal tables) and A serves verify-only there while §3.5 said
>   halt; F18 was therefore a FALSE ORACLE for the in-sync row-16 cells. NOT a safety hole (row 16
>   is decoupled → never reaches the legacy `:399` writer; Invariant B holds) but MATERIAL (the
>   64-cell table is not fully derived from W and A — the SEED's explicitly warned-of "sibling cell
>   in the same `complete` cursor-state group" M7).
>
> **This spec starts from the v8 `HOLDER.md`** (a required context doc) and is a **proper
> revision**: it **resolves M7** per the binding prescribed fix in the v8 collaboration ledger §4
> and `SEED.md` — **Option 1 (the clean fix, parallel to the M6 fix): make row 16 and its `>=21`
> revoke-embedding cell CONDITIONAL on A's §3.3a complete/decoupled fingerprint-sync predicate, and
> propagate the conditional through §1.3, §3.3a, §3.5, §4.5, and F18, making F18 PARAMETRIC over
> ALL A-reaching complete-row cells with the in-sync/out-of-sync sub-dimension.** Concretely: (i)
> §0.2's W→A-independence invariant is strengthened with the **DERIVATION-RULE-COMPLETENESS**
> sub-invariant (every A-reaching complete-row cell whose A outcome is fingerprint-conditional is
> WRITTEN conditionally) and the **A-`revokeEmbedded`-independence of the decoupled complete
> branch** (once decoupled is ON, A's complete branch reads neither `applied_owner` NOR
> `revokeEmbedded`, so rows 15 and 16 take the IDENTICAL A3 outcome in every W-passing column,
> differing only at W in the `>=21` column); (ii) §3.5 row 16 in the `==0`, `==20`, AND `>=21`
> columns becomes conditional — "**SERVE-verify if in-sync, else `awaiting_deploy`**" — exactly as
> row 15; (iii) F18 is parametric over the seven A-reaching complete-row cells {13/`==0`,
> 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`}, constructing BOTH the in-sync
> and out-of-sync sub-cases for each, while the `:399` spy list stays the SAME four cells (row 16
> is decoupled — it never reaches `:399`); (iv) the normal pre-0021 state for row 16 `==0`/`==20`
> is documented OUT-OF-SYNC (→ `awaiting_deploy`, the dominant case) and the normal post-0021
> steady state for row 16 `>=21` is IN-SYNC (→ SERVE-verify, the dominant case). It **carries M6
> (rows 13/15), M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 forward unregressed**.
> §0.1 below is the auditable resolution map; the cycle-1 falsifiers should verify M7 there, then
> re-attack the revised §3.5 row 16, the §4.5/F18 four-cell consistency, F18's parametric
> complete-row extension, and re-confirm each carry-forward.
>
> It is a falsifiable implementation spec for RFC 0142 **P4 only** — the one-shot deployer that
> lifts schema mutation out of serve-boot — that `rfc-0142-p4-build` can execute contract-first
> (TDD). RFC 0142 is accepted (D258); this does NOT re-open the five-layer design. Primary source:
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (Phasing P4 row, Open
> Questions 3 + 4, "the hard correctness core of P4"). **Every anchor below is re-verified
> line-by-line against the v9 worktree HEAD `83fe6465`** — the M6/M7 orthogonality anchors
> `schema_drift.go:145-161` (`LiveFingerprint` reads the `schema_state.fingerprint` singleton) and
> `schema_drift.go:171-195` (`RecordSchemaFingerprint` writes `ExpectedFingerprint()`), and the
> M5 owner-watermark sites `owner.go:145` (`if applied == 0 { return nil }`), `:148-150`
> (shortfall), `:151-153` (tolerate-forward), `owner.go:23` (`LatestOwnerBundleVersion = 20`),
> `:35` (`RequiredOwnerBundleVersion = LatestOwnerBundleVersion`), and the boot order
> `connection.go:349/353/376-383/399` (re-read this cycle — see §9). `git diff --stat 3f9d5734
> 83fe6465 -- go/pkg/db/` is EMPTY, so the go/pkg/db source is byte-identical with the v8 anchors.

## 0. Root reframe held — and its FINAL complete-row coherence edge (M7)

Schema mutation must stop being an **implicit side effect of the serving process's restart** and
become an **explicit, ordered, resumable, provenance-tracked operation owned by a dedicated
deployer**. Then the serving daemon holds **zero create-DDL on the serving path**, "restart
force-commits a half-applied deploy" becomes structurally impossible, and a bad migration can
never wedge the single writer on boot. **M3 (v6) closed** the legacy `ApplyMigrations`-on-serve-boot
edge for a revoke-embedding binary / a transcript with a pending change. **M5 (v7) closed** the
fresh / single-role / no-authority bootstrap edge (`applied_owner == 0` serves, row 1). **M6 (v8)
closed** the no-revoke `complete` rows (13/15) by deriving them from the `applied_owner`-independent
A predicate.

**M7 is the FINAL complete-row coherence edge of the same reframe.** The whole reframe leans on one
executable artifact — the boot-path decision table (§3.5) plus its proof (§4.5) and its oracle
(F18) — being internally consistent and **fully derived from the two boot predicates W and A**, so
a build cannot exploit a predicate/table mismatch to smuggle in an unstated guard. M6 derived rows
13/15 but left the SIBLING cell in the same `complete` cursor-state group — row 16
(revoke-embedding, decoupled) — ASSERTED unconditionally (`awaiting_deploy` for `==0`/`==20`,
`SERVE-verify` for `>=21`) from an owner-watermark reachability inference (`owner_bundle_meta < 21
⟹ fingerprint mismatch`) that A neither reads nor verifies. That made F18 a false oracle for the
constructible in-sync row-16 cell and left the class un-closed. **The v9 revision derives row 16
from A under the SAME contract that fixed rows 13/15 — the conditional "serve verify-only if
in-sync, else `awaiting_deploy`" — so EVERY A-reaching complete-row cell is a consequence of the
predicates, the class is closed by derivation, and F18 is the complete oracle it claims to be.**

The v9 revision changes EXACTLY the thing M7 names, on top of the v8 spec, and re-anchors line
references. It does **not** re-open any resolved mechanism:

- **M7 — the §3.5/§4.5/F18 row-16 cell is made coherent with the `applied_owner`- AND
  `revokeEmbedded`-independent A predicate by deriving it from A.** Row 16 in the `==0`, `==20`,
  AND `>=21` columns becomes conditional — "SERVE-verify if in-sync, else `awaiting_deploy`" —
  IDENTICAL to row 15's decoupled cells, because A's decoupled-complete branch (§3.3a step 3) reads
  the recorded fingerprint and `cursor.plan_hash`, never `applied_owner` and never `revokeEmbedded`
  (step 0's config gate is skipped when decoupled is ON). F18 is extended to be parametric over the
  seven A-reaching complete-row cells, constructing BOTH the in-sync and out-of-sync sub-cases for
  each. The four `:399`-reaching cells are UNCHANGED (row 16 is decoupled — `ConnectAndVerify`
  never reaches `:399`), so the §4.5 enumeration and the F18 spy list still agree exactly.

**Preserved asymmetry (the load-bearing nuance, carried from the v8 ledger §4 / §5):** the M7 fix
is **local to row 16 (the decoupled revoke-embedding `complete` cell) and the F18 parametric
extension**. It must NOT (and does not) re-collapse the resolved row-1 fresh-DB serve (cell 1/`==0`
still serves); must NOT regress the M6 rows-13/15 conditional cells; must NOT weaken the M3 config
gate (cells 2/6/10/14 still halt `awaiting_deploy_config` at A0 in EVERY column that passes W incl.
`0`); must NOT regress the BC-N2 `applied_owner == 20` edge (the `==20` column's non-complete cells
are verbatim); and must NOT advance `RequiredOwnerBundleVersion` (stays 20) or alter the watermark.

---

## 0.1 Addressing the design-v8 findings (the gating requirement)

The single binding v8 finding (M7) is resolved by a concrete derivation bound to named code sites —
not a claim. M6 (rows 13/15), M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 are carried
forward **verbatim** from the v8 HOLDER and re-confirmed unregressed. The "where" column names the
section that specifies the fix and the named test that would refute it.

| Finding | v8 status | v9 disposition | Where / refuting test |
| --- | --- | --- | --- |
| **M7** row 16 (`complete`, decoupled, revoke-embedding) is not derived from A: §3.5 wrote `==0`/`==20` unconditional `awaiting_deploy` and `>=21` unconditional `SERVE-verify`, but A's §3.3a step-3 decoupled branch is fingerprint-conditional and reads neither `applied_owner` nor `revokeEmbedded`; the in-sync row-16 cell is constructible (orthogonal `schema_state`/`owner_bundle_meta`) so F18 is a false oracle, violating the holder's own derivation rule (v8 HOLDER.md:565-566) | **OPEN** (falsifier_1 decoupling-boundary/decision-table lens + falsifier_2 carry-forward/regression lens, independent convergence; adjudicator source-verified `schema_drift.go:145-161`/`:171-195` orthogonal to `owner_bundle_meta`) | **RESOLVED via Option 1 (the clean fix, parallel to the M6 fix): derive row 16 from A.** (a) §0.2 adds the **DERIVATION-RULE-COMPLETENESS** sub-invariant (every A-reaching complete-row cell whose A outcome is fingerprint-conditional is WRITTEN conditionally) and the **A-`revokeEmbedded`-independence of the decoupled complete branch** (once decoupled is ON, step 0 is skipped and A's complete branch reads neither `applied_owner` NOR `revokeEmbedded`, so rows 15 and 16 take the IDENTICAL A3 outcome in every W-passing column). (b) §3.5 row 16 `==0`/`==20`/`>=21` become conditional — "**SERVE-verify if in-sync, else `awaiting_deploy`**" — matching row 15 (`==0`/`==20`: normal pre-0021 state OUT-OF-SYNC → `awaiting_deploy`; `>=21`: normal post-0021 steady state IN-SYNC → SERVE-verify; the opposite sub-case is the degenerate corner A handles correctly). (c) F18 is PARAMETRIC over the seven A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`}, constructing BOTH the in-sync and out-of-sync sub-cases. (d) the `:399` spy list is UNCHANGED (row 16 is decoupled → `ConnectAndVerify` never self-records), so §4.5 and the F18 spy list still enumerate the SAME four cells. §1.3/§3.3a/§3.5/§4.5 updated consistently. The fix is LOCAL to row 16; it does NOT re-collapse row 1, regress rows 13/15, weaken the M3 gate, regress BC-N2, or advance `Required`. | §0.2 (the strengthened invariant), §3.3a step 3 (the decoupled branch covers row 16 identically to row 15), §3.5 (row 16 conditional + the cross-row audit), §1.3 (the `complete`/decoupled/in-sync row is `applied_owner`- AND `revokeEmbedded`-independent), §4.5 (Invariant B; the 4-cell spy list unchanged) / **`T-deploy-bootpath-decision-table` (F18, parametric over the 7 A-reaching complete-row cells; asserts row 16 conditional == row 15) + `T-deploy-fresh-db-bootstrap-serves` (F18a)** |
| **M6** the M5 `applied_owner` split propagated through the no-revoke `complete` rows (13/15); the `==0` column conditional == `==20`; §0.2 W→A-independence invariant; the 4-cell `:399` enumeration in §4.5 == F18 spy list; the cross-row `==0` audit | **RESOLVED (rows 13/15)** (v8; both falsifiers + adjudicator concede) | **CARRIED FORWARD VERBATIM, unregressed — and now EXTENDED to row 16 by the M7 derivation.** Rows 13/15 `==0` stay conditional "serve if in-sync, else `awaiting_deploy`" identical to `==20`; §0.2 invariant intact (now also covering row 16); the four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} enumerated identically in §4.5 AND the F18 spy list (UNCHANGED by M7); the cross-row audit intact (the `complete`-rows bullet now records row 16 conditional too). The M7 fix does NOT re-collapse rows 13/15. | §0.2, §3.5 (rows 13/15 unchanged; row 16 now conditional), §4.5, §3.3a / **F18 (the rows-13/15 arms unchanged; the row-16 arm added) + F18a** |
| **M5** the §3.5/F18 `applied_owner` dimension split `{0/no authority, 1..19 shortfall, ==20, >=21}`; cell 1/`==0` serves the fresh-DB bring-up | **RESOLVED (row 1)** (v7; conceded) | **CARRIED FORWARD VERBATIM at row 1, unregressed.** W = `CheckOwnerBundleWatermark` (`owner.go:124-154`): `applied_owner == 0` → return nil, proceed to A (`owner.go:145`); `1 <= applied_owner < 20` → `awaiting_owner_ddl` (`owner.go:148-150`); `==20` → nil (`:151-153`); `>=21` → revoke-embedding tolerates forward, no-revoke → `awaiting_deploy` (barrier b). Cell 1/`==0` SERVES-legacy; cell 1/`==20` is the inert-landing re-boot. The M7 fix does NOT re-collapse this. | §3.5 (`==0` column row 1 unchanged), §0.2, §9 / **F18 + F18a + F11(e)/(f)** |
| **M3** the `complete`-cursor window let a revoke-embedding binary + flag OFF ride the legacy `ConnectAndMigrate` mutate+self-record path around `VerifyStoredTranscript` | **RESOLVED** (v6; conceded) | **CARRIED FORWARD VERBATIM, unregressed.** The `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt is HOISTED to step 0 of `CheckDeployActivation`, before any cursor branch, firing for EVERY cursor state incl. `complete` (barrier a). **The M7 fix is in the DECOUPLED revoke-embedding cell (row 16, decoupled ON) where step 0 does NOT fire — it does NOT re-open the M3 legacy bypass (row 16 is decoupled and never reaches `:399`); cells 2/6/10/14 still halt `awaiting_deploy_config` at A0.** | §3.3a step 0, §3.5 (cells 2/6/10/14 all columns), §4.5 / **F17 + F11(g) + F18** |
| **M4** F16's revoke-exclusion test asserted production `OwnerBundles()` contains 0021 before 0021 is authored | **RESOLVED** (v6; conceded) | **CARRIED FORWARD VERBATIM, unregressed.** F16a (synthetic list / injectable-FS, step 2) drives the exclusion filter WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (production, step 7) asserts the embed/listing split + houses the forced FMA-007 self-heal pgtest via `isCrossBundleDependencyError` (`owner.go:367-374`). M7 does not touch F16. | §3.2a, §6 / **F16a + F16b + extended F12/`G-revoke-last`** |
| **M1** full stored-transcript byte + already-applied DB-stamp verification on resume AND as finalizer step 0 | **INTACT** (v5–v8) | **CARRIED FORWARD VERBATIM, unregressed.** `VerifyStoredTranscript(plan_hash)` (§3.4a) checks the ENTIRE stored transcript `sha256` vs the binary's embedded bytes ⇒ `deploy_plan_binary_mismatch`; already-applied entries also verify the DB stamps ⇒ `deploy_plan_db_stamp_mismatch`; same verifier on every resume AND finalizer step 0. **M7 is resolved via Option 1 (the conditional cell), NOT Option 2 (a new boot-time DB-stamp guard) — so M1's scope (deploy resume / finalizer step 0, NOT ordinary serve-boot) is unchanged; the row-16 in-sync cell serves because the fingerprint genuinely matches, no new serve-boot stamp check is added.** | §3.4a, §1.2, §1.3, §3.4 / **F15 + F14** |
| **M2** the single non-revoke filter `OwnerDDLApplyBundles()` + `isNonRevokeBundle` excludes 0021 from EVERY `owner-ddl apply` route incl. the FMA-007 self-heal | **INTACT** (v5–v8) | **CARRIED FORWARD VERBATIM, unregressed.** The filter, the in-loop guards, the nil-fallback split, and the embed/listing split. M7 does not touch the filter. | §3.2a, §4.4, §4.5 (Invariant A) / **F16a/F16b + extended F12 / `G-revoke-last`** |
| **BC-N1** immutable `deploy_plan` transcript materialized before step 0; resume off the stored transcript | **INTACT** (v4–v8) | **CARRIED FORWARD VERBATIM, unregressed.** Migration 0044 `deploy_plan` keyed by `plan_hash`; materialize in the same tx as `in_progress(0)` before step 0; resume never recomputes `BuildPlan`; §1.3 + transcript-enumerated doctor + F14. | §1.2, §1.3, §3.2 / **F14** |
| **BC-N2** universal `revokeEmbedded`-independent `CheckDeployActivation` edge halting non-`complete` cursors at `applied_owner == 20` | **INTACT** (v4–v8) | **CARRIED FORWARD VERBATIM, unregressed.** The universal incomplete-deploy edge halts `awaiting_deploy` DB-untouched on every non-`complete` cursor. **M7 is in the `complete`/decoupled/revoke-embedding cell (row 16), NOT the non-complete edge — cells 5/7/9/11 unchanged.** | §3.3a step 1, §3.5 (rows 5/7/9/11), §4.2 / **F11(e)/(f) + extended `G-old-binary-refuse`** |
| **C1** the `finalizing` state + idempotent finalizer + §1.3 row + the M1 pre-finalizer gate | **INTACT** (v2–v8) | **CARRIED FORWARD VERBATIM, unregressed.** M7 touches neither the finalizer nor the legacy writer's enforcement — it makes the §3.5 row-16 cell coherent in the `complete` rows the finalizer's `complete` cursor lands in, without changing the finalizer. | §1.2, §1.3, §3.4 / **F10 + F15 pre-finalizer arm** |
| **C2** `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion` KEPT at 20 | **INTACT** (v2–v8) | **CARRIED FORWARD VERBATIM, unregressed.** The M7 fix keeps the forward-watermark rule at `applied_owner >= 21` (barrier b) and `RequiredOwnerBundleVersion = 20` (`owner.go:35`) — it does NOT advance `Required`, alter the watermark, or change the `applied >= 21` forward rule; it makes row 16 conditional on A3 fingerprint-sync. | §3.3a, §4.2, §4.3, §3.5 (the `>=21` column derived from W then A) / **F11** |
| **C3** the DDL-revoke bundle 0021 special-cased + excluded from `owner-ddl apply` + applied terminal | **INTACT** (v3–v8) | **CARRIED FORWARD VERBATIM.** 0021 special-cased out of the owner prefix, applied terminal; `DDLRevokeOwnerBundleVersion = 21`; `LatestOwnerBundleVersion`/`Required` stay 20. **M7 arose BECAUSE the v8 table imported the C3 reachability fact ('a completed revoke deploy means applied_owner >= 21') into A without A verifying the owner-bundle stamp; the v9 fix makes the cell conditional on fingerprint-sync rather than on owner-watermark state — the correct derivation from A — so C3 itself is not regressed.** | §3.2, §3.2a, §3.3b, §4.4 / **F12 + `G-revoke-last`** |

**Carried forward unchanged (v1–v8 ledgers — do not re-litigate):** Q4 (plain verb + three seams,
§2); the per-step Q3-A/Q3-B mid-step resumability **body** + step taxonomy (§1.1); the shadow-first
decoupling shape (default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2
watermark interlock + P3 drift gate, the embed-FS-derived plan, the hash-chained receipt, `doctor
schema_deploy_unrecorded`); the M1 `VerifyStoredTranscript`; the M2 non-revoke filter; the M3
activation predicate; the M4 F16 split; the M5 `applied_owner` split (row 1); the M6 rows-13/15
derivation. **The v9 revision touches only row 16 of the §3.5 decision table (the `==0`/`==20`/`>=21`
columns of row 16), the F18 parametric complete-row extension, the matching §1.3/§3.3a/§4.5 notes,
the strengthened §0.2 invariant, and the re-anchored line numbers.**

---

## 0.2 The load-bearing mechanical-derivation invariant (the heart of the M6 AND M7 fixes)

> **INVARIANT W→A-INDEPENDENCE (the structural core of the v8 table — carried forward verbatim).**
> *On every boot, the two guard sites run in order — W = `CheckOwnerBundleWatermark` (the
> owner-watermark interlock) then A = `CheckDeployActivation` (the M3 config gate + cursor edge).
> **Once W has returned nil (`applied_owner ∈ {0, ==20}`, or `>=21` for a revoke-embedding binary),
> A is owner-watermark-independent: it reads `cursorState`, `decoupledEnabled`, `revokeEmbedded`,
> `cursor.plan_hash`, and the recorded `schema_state.fingerprint` — and NEVER reads
> `owner_bundle_meta` / `applied_owner`.** Therefore, for any fixed `(cursorState, decoupledEnabled,
> revokeEmbedded)` row, the `==0` and `==20` columns have IDENTICAL A-gate outcomes in EVERY cursor
> row.*

> **SUB-INVARIANT A-`revokeEmbedded`-INDEPENDENCE OF THE DECOUPLED COMPLETE BRANCH (NEW in v9 — the
> heart of the M7 fix).** *Step 0 of A (the M3 config gate `revokeEmbedded && !decoupledEnabled →
> awaiting_deploy_config`) is the ONLY A predicate that reads `revokeEmbedded`, and it does NOT
> fire when `decoupledEnabled == true`. So on the decoupled path, A's `complete`-cursor decision
> (§3.3a step 3, `decoupledEnabled == true` branch) reads NEITHER `applied_owner` NOR
> `revokeEmbedded` — only `cursor.plan_hash` and the recorded `schema_state.fingerprint`. Therefore
> the no-revoke decoupled complete row (15) and the revoke-embedding decoupled complete row (16)
> take the IDENTICAL A3-conditional outcome — "serve verify-only if in-sync, else `awaiting_deploy`"
> — in EVERY W-passing column; they differ ONLY at W, in the `>=21` column (no-revoke → barrier b
> halt; revoke-embedding → tolerate-forward to A).*

> **SUB-INVARIANT DERIVATION-RULE COMPLETENESS (NEW in v9 — what M7 enforces).** *Every A-reaching
> complete-row cell whose A outcome is conditional on the fingerprint-sync state is WRITTEN
> conditionally ("serve if in-sync, else `awaiting_deploy`"); NO complete-row cell is asserted from
> an owner-watermark reachability inference (e.g. `owner_bundle_meta < 21 ⟹ fingerprint
> mismatch`). The v8 table obeyed this for rows 13/15 but violated it for row 16; v9 obeys it for
> ALL seven A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`,
> 16/`==20`, 16/`>=21`}.*

These are not claims to be taken on faith; they are mechanically forced by the source, and the
falsifiers can verify them against the named sites:

- **A reads the fingerprint, not the owner watermark, and not `revokeEmbedded` on the decoupled
  complete branch.** A's `complete`-cursor decoupled decision (§3.3a step 3) is `cursor.plan_hash ==
  expected` AND `LiveFingerprint(recorded) == ExpectedFingerprint()`. `LiveFingerprint` reads the
  recorded `striatumd.schema_state.fingerprint` singleton (`schema_drift.go:145-161`) — a SEPARATE
  table from `owner_bundle_meta`; it does NOT recompute the live schema and does NOT read
  `applied_owner`. `ExpectedFingerprint()` hashes the binary's embedded file bytes
  (`schema_drift.go:83-100`) — also independent of `applied_owner`. `RecordSchemaFingerprint`
  writes only the `schema_state` singleton (`schema_drift.go:171-195`). The decoupled branch does
  not branch on `revokeEmbedded` (step 0 already excluded the `revokeEmbedded && !decoupledEnabled`
  case, and step 0 does not fire when decoupled is ON). So **`schema_state` (what A reads on the
  decoupled complete branch) is orthogonal to both `owner_bundle_meta` (= `applied_owner`, what W
  reads) and the `revokeEmbedded` build flag.**
- **The columns and the no-revoke/revoke-embedding rows differ only in facts A captures through the
  fingerprint condition, never through `applied_owner`/`revokeEmbedded` on the decoupled complete
  branch.** A `complete` cursor over an `owner_bundle_meta`-absent DB, a version-20 DB, and a
  version-`>=21` DB can each carry the SAME or DIFFERENT `schema_state.fingerprint` — the facts are
  independent. The decision-table cell must therefore be written **conditionally on the
  fingerprint-sync state** ("serve if in-sync, else `awaiting_deploy`"), the SAME rule for every
  W-passing column and for BOTH the no-revoke (15) and revoke-embedding (16) decoupled rows. The v8
  bug was writing row 16 UNCONDITIONALLY (`awaiting_deploy` for `==0`/`==20`, `SERVE-verify` for
  `>=21`) while row 15 was conditional — which silently treated `applied_owner` (and the `>=21`/`<21`
  watermark split) as an A input on the decoupled complete branch. The fix removes that false
  dependence.
- **Mechanical consequence.** The full 64-cell table is DERIVED, not asserted: pick W's outcome
  from the `applied_owner` bucket and `revokeEmbedded` (`0`/`==20` → pass to A; `1..19` →
  `awaiting_owner_ddl`; `>=21` → barrier b for no-revoke, tolerate-forward to A for
  revoke-embedding), then for the W-passing buckets apply the SINGLE A predicate (§3.3a) — whose
  decoupled-complete outcome is identical for `==0`, `==20`, and (for a revoke-embedding binary)
  `>=21`, and identical between rows 15 and 16. Any A-reaching complete-row cell whose written
  outcome is unconditional where A's predicate is fingerprint-conditional is, by these invariants,
  a bug; §3.5's cross-row audit confirms none remain.

The `1..19` column (W halts before A) and the `>=21`-no-revoke column (barrier b at W) are the only
columns where the outcome legitimately differs from `==0`/`==20`, and they differ at **W**, not at
A. That is the whole shape of the table.

---

## 1. Decision Q3 — How atomic is "atomic"? (the hard correctness core)

**Claim (Q3-DECISION).** The **per-step-atomic + resumable-cursor** contract is sufficient for
*every* owner+runtime interleaving AND every boot-path combination P4 ships, **provided**: (a) plan
identity is an **immutable durable fact materialized before step 0** and resume reads it (BC-N1);
(b) the two per-step sub-rules (Q3-A/Q3-B) hold for both the schema side effect AND the per-step
receipt, keyed on the **stored transcript's** `(plan_hash, step_index)`; (c) the finalization
boundary is closed by a distinct `finalizing` state + an idempotent finalizer (C1); (d) **no boot
can serve while the cursor is non-`complete`** (BC-N2); (e) on every resume AND before the finalizer
self-records, the ENTIRE stored transcript byte-matches the running binary and the already-applied
DB stamps match the transcript; any mismatch halts the deploy DB-untouched and NO fingerprint is
written (M1); (f) NO boot reaches the legacy `ApplyMigrations`/`RecordSchemaFingerprint`
mutate+self-record path while a deploy transcript with a pending change is present, and NO
revoke-embedding binary EVER reaches that legacy path — the activation guard is the sole authority
for "may this boot auto-apply/self-record on serve-boot", evaluated before `ApplyMigrations` in both
boot paths (M3); (g) the owner-watermark gate W is fail-closed for a genuine authority shortfall
(`1 <= applied_owner < 20` → `awaiting_owner_ddl`) AND fail-OPEN for the legitimate fresh /
single-role / no-authority bootstrap (`applied_owner == 0` → serve) (M5); (h) the decision is
provably correct in EVERY `cursorState × decoupledEnabled × revokeEmbedded × applied_owner ∈ {0,
1..19, ==20, >=21}` cell because the table is DERIVED from W and A, with A owner-watermark-independent
(§0.2), so the `==0` and `==20` columns are identical in EVERY cursor row (M6); **and (i) NEW
emphasis in v9 (M7) — the derivation is COMPLETE across the whole `complete` cursor-state group: A's
decoupled complete branch reads neither `applied_owner` NOR `revokeEmbedded` (§0.2 sub-invariant), so
rows 15 and 16 take the IDENTICAL A3-conditional outcome and row 16's `==0`/`==20`/`>=21` cells are
WRITTEN conditionally ("serve verify-only if in-sync, else `awaiting_deploy`"), closing the last
asserted-not-derived sibling cell — F18 is parametric over all seven A-reaching complete-row cells.**

- **Q3-A / Q3-B** (carry-forward body, unchanged from v4–v8; receipt keyed off the stored
  transcript). See §1.1 and §3.3b.
- **Q3-V — full-transcript verification (M1, carry-forward).** `VerifyStoredTranscript(plan_hash)`
  (§3.4a) on every resume and as finalizer step 0.
- **Q3-W — boot-path activation completeness (M3 + M5 + M6, carry-forward) + the M7 complete-row
  closure.** On EVERY boot, the two guard sites run in order — W = `CheckOwnerBundleWatermark` then
  A = `CheckDeployActivation` — BOTH before any mutation or self-record. §3.5 DERIVES the decision
  from W and A across the whole cell space, proves it fail-closed AND that the legitimate fresh-DB
  cell still serves: the only cells that reach the legacy `:399` writer are **1/`==0`**, **1/`==20`**
  (`cursorState == none`, no transcript), **13-in-sync/`==0`**, and **13-in-sync/`==20`**
  (`complete`-and-already-in-sync no-revoke: an idempotent rewrite of the same fingerprint); **row 16
  is decoupled and reaches `:399` in NO column (M7) — the decoupled `ConnectAndVerify` path never
  self-records.** No revoke-embedding binary, no transcript-with-a-pending-change, and no genuine
  `1..19` shortfall ever reaches the legacy writer.

### 1.1 Step taxonomy (the closed set P4 ships) — unchanged from v1–v8 (re-anchored to current `main`)

| Class | Examples | Transactional? | Protocol | Idempotent form |
| --- | --- | --- | --- | --- |
| **T-DDL** runtime | `CREATE TABLE`, additive `ALTER … ADD COLUMN`, `CREATE INDEX` (non-concurrent), the new runtime migrations | Yes | **Q3-A** (step + ownership-reconcile + version stamps + cursor + **receipt** in one tx) | `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`; re-run is a no-op |
| **T-DDL** owner | owner-bundle DDL, **incl. the terminal DDL-revoke bundle 0021** | Yes (each bundle applies in one tx that stamps `owner_bundle_meta` last — `owner.go:511-541`) | **Q3-A**, reusing the per-bundle transaction; cursor advance + **receipt** join that tx | bundles idempotent (`IF NOT EXISTS` + `ON CONFLICT (version) DO NOTHING`) |
| **NT-DDL** | `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, `DROP INDEX CONCURRENTLY` | No | **Q3-B** (pre-marker → step → reconcile: keyed receipt + post-marker) | `CREATE INDEX CONCURRENTLY IF NOT EXISTS`; resume reconciler drops a left-behind `INVALID` index then recreates; `ADD VALUE … IF NOT EXISTS` |
| **DML-backfill** (bounded in P4; full online backfill is P5) | a one-shot data normalize a runtime migration carries | Yes if small; batched otherwise | Q3-A small / Q3-B batched | `WHERE <not-yet-done>` predicate; re-run resumes from the unprocessed tail |

> The terminal DDL-revoke bundle 0021 is a T-DDL owner step (the existing per-bundle transaction,
> `owner.go:511-541`), so its receipt commits atomically. NT-DDL never creates a new ownable object
> whose owner is wrong, so the C3 ownership reconciliation (§3.3b) is needed only on transactional
> object-creating steps.

### 1.2 `deploy_cursor` + the immutable `deploy_plan` transcript (BC-N1) and crash-resume semantics (C1) — carry-forward

Both are **new runtime-owned tables**, created by runtime migration **0044** (modeled
byte-for-byte on `0043_schema_state.sql:39-52`). Migration 0044 is additive, runtime-owned, carries
no owner DDL.

**`deploy_cursor` (singleton):** `id text PK CHECK (id='singleton')`, `plan_hash text NOT NULL`,
`state text NOT NULL`, `step_index int NOT NULL`, `step_id text NOT NULL`, `updated_at`.
**`deploy_plan` (append-only, keyed by `plan_hash`):** the IMMUTABLE ordered transcript `steps jsonb`
= `[{step_index, step_id, role, sha256, transactional}]`, revoke last, `revoke_step_index`,
`base_owner_version`, `base_runtime_version`, `target_*`, INSERT-once (`ON CONFLICT (plan_hash) DO
NOTHING`).

`state ∈ { idle, in_progress, step_committed, finalizing, complete, aborted }`. Lifecycle (carry-forward):

```
materialize(plan_hash, transcript) + idle─▶in_progress(0) ─▶ step_committed(0) ─▶ … ─▶ step_committed(N-1=0021) ─▶ finalizing ─▶ complete
   [deploy_plan row + cursor=in_progress(0) in ONE tx, BEFORE step 0]   │
                                                                         └─(operator abort / fatal)──▶ aborted
```

- **Materialization (BC-N1).** After the substrate-ensure preamble (§3.2), compute
  `BuildPlan(base_owner, base_runtime)` once, derive `plan_hash`, and in one transaction INSERT the
  immutable `deploy_plan` row and set `deploy_cursor → in_progress(0)` — before step 0.
- **Resume verification (M1).** On every resume, immediately after loading
  `deploy_plan[cursor.plan_hash]` and BEFORE applying or finalizing any step, run
  `VerifyStoredTranscript(plan_hash)` (§3.4a) over the WHOLE transcript.
- **Finalization (C1 + the M1 pre-finalizer gate).** When the terminal step is `step_committed`,
  advance to `finalizing`, then run the idempotent finalizer (§3.4): **(0)** run
  `VerifyStoredTranscript` and abort on mismatch (writing NOTHING); **(1)** append the guarded
  `complete` receipt; **(2)** `RecordSchemaFingerprint` UPSERT; **(3)** advance `finalizing →
  complete` last. The deployer finalizer is the C1 writer of `schema_state` on the deploy path — it
  is gated by step (0); the LEGACY `connection.go:399` writer is governed by the W (M5) + A (§3.3a,
  M3) gates, proven over the whole cell space in §3.5.

### 1.3 The load-bearing disambiguation (C1 + BC-N1 + BC-N2 + M1 + M3) — carry-forward (+ the M6/M7 owner-watermark- AND `revokeEmbedded`-independence note)

`deploy_cursor` is the **second signal** alongside the drift gate's `LiveFingerprint ==
ExpectedFingerprint`. The rows carry forward from v8 verbatim. **M6/M7 note:** every row below keys
on `deploy_cursor.state` and the binary/fingerprint facts (the A dimension), which are **orthogonal
to `applied_owner` (the W dimension)** — `LiveFingerprint` reads `schema_state`, never
`owner_bundle_meta` (`schema_drift.go:145-161`). On the DECOUPLED path A's complete branch is ALSO
orthogonal to `revokeEmbedded` (§0.2 sub-invariant). So the `complete`/in-sync row classifies
"serve" **regardless of the `applied_owner` bucket AND regardless of whether the binary embeds the
revoke** (`==0`, `==20`, and `>=21` all land in the SAME row, and the no-revoke decoupled binary
(row 15) and the revoke-embedding decoupled binary (row 16) land in the SAME row), and the
`complete`/pending-change row classifies `awaiting_deploy` regardless of the bucket and the embed.
§3.5's row 16 is therefore conditional on fingerprint-sync exactly like row 15, by construction.

| `deploy_cursor.state` | condition (vs the **stored transcript** + the binary) | classification | action |
| --- | --- | --- | --- |
| `complete` | `deploy_plan[plan_hash]` present; full transcript byte-matches binary; `== ExpectedFingerprint`; **`decoupledEnabled` (or a no-revoke binary already in-sync, §3.3a)** — **independent of `applied_owner` AND, on the decoupled path, of `revokeEmbedded`** | **in sync** | **serve** (verify-only when decoupled — incl. the revoke-embedding decoupled binary, row 16; legacy no-op-apply when a no-revoke flag-OFF binary is already in-sync) |
| `complete` | **`revokeEmbedded == true` AND `decoupledEnabled == false`** (M3) | **config halt** | refuse-to-serve **`awaiting_deploy_config`**, DB untouched — never reaches `ApplyMigrations`/`:399` |
| `complete` | **decoupled binary (no-revoke row 15 OR revoke-embedding row 16), but `ExpectedFingerprint() ≠` recorded OR `cursor.plan_hash ≠ expected`** (A3 decoupled comparison) — **independent of `applied_owner` and `revokeEmbedded`** | **pending change over a transcript** | refuse-to-serve **`awaiting_deploy`**, DB untouched — never serves an out-of-sync schema (M7: the normal pre-0021 row-16 state is OUT-OF-SYNC and lands here) |
| `complete` | **no-revoke binary, flag OFF, but `ExpectedFingerprint() ≠` recorded OR `cursor.plan_hash ≠ expected`** (M3 comparison) — **independent of `applied_owner`** | **pending change over a transcript** | refuse-to-serve **`awaiting_deploy`**, DB untouched — never auto-applies on serve-boot |
| `in_progress` / `step_committed` | present; the FULL transcript byte-matches the binary AND already-applied DB stamps match | **incomplete, resume off the STORED transcript** (BC-N1) | refuse-to-serve `awaiting_deploy`; `deploy` resumes at `cursor.step_index` |
| `in_progress` / `step_committed` | present but the cursor's `plan_hash` ≠ the binary's freshly-computed pending plan | **incomplete, resume with the STORED plan** (BC-N1) | refuse-to-serve `awaiting_deploy`; resume off the stored transcript by `cursor.plan_hash` — NOT drift |
| `finalizing` | present; full transcript byte-matches binary | **resumable finalization** (C1) | refuse-to-serve `awaiting_deploy`; re-run runs the idempotent finalizer (which re-verifies, §3.4 step 0) |
| `in_progress`/`step_committed`/`finalizing`, `deploy_plan[plan_hash]` ABSENT, **OR ANY stored step `sha256` ≠ the binary's embedded bytes** | full-transcript mismatch (M1) | **`deploy_plan_binary_mismatch`** | refuse-to-serve; apply NOTHING; operator inspects |
| `in_progress`/`step_committed`/`finalizing`, transcript byte-matches, **but an already-applied step's DB stamp ≠ the stored transcript** (M1) | DB-stamp mismatch | **`deploy_plan_db_stamp_mismatch`** | refuse-to-serve; do NOT finalize; apply NOTHING |
| absent `deploy_cursor` table / `state == idle` (NO transcript) | n/a | **no deploy in flight** | **legacy path (flag OFF) + no-revoke binary:** serve (auto-apply, the legitimate `:399` cell — **incl. the fresh `applied_owner == 0` bootstrap, M5, AND the inert `applied_owner == 20` re-boot — the SAME serve, since A does not read `applied_owner`**); **decoupled (flag ON) OR revoke-embedding binary:** refuse `awaiting_deploy`/`awaiting_deploy_config` (C2 + M3 — §3.3a) |
| `aborted` | n/a | **aborted** | refuse-to-serve `awaiting_deploy` |
| `complete`, `plan_hash` ≠ expected (foreign) | foreign | **genuine drift** | refuse-to-serve (`schema_drift`) |

Load-bearing properties this table enforces (unchanged from v8): plan identity is a stored fact and
resume reads `deploy_plan[cursor.plan_hash]` (BC-N1); a resume/finalize binary that disagrees with
ANY stored step is forced to a typed mismatch and never writes a fingerprint (M1); `complete` ⇒ 0021
applied (C1); incomplete-never-serves holds for EVERY deployer-aware binary (BC-N2); a `complete`
cursor no longer short-circuits the activation guard (M3). **M6/M7: every row is `applied_owner`-blind
(it reads `deploy_cursor` + the fingerprint, never `owner_bundle_meta`), AND on the decoupled path
`revokeEmbedded`-blind in the complete branch, so the §3.5 `==0`/`==20`/`>=21` columns produce
identical classifications and rows 15 and 16 are the same conditional — the in-sync
`complete`/decoupled cell serves (whether the binary embeds the revoke or not), and the out-of-sync
`complete`/decoupled cell halts `awaiting_deploy` (the normal pre-0021 row-16 state).**

---

## 2. Decision Q4 — Is a deploy itself a Striatum run? (carry-forward, sound)

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape door held open by
three seams.** Unchanged from v1–v8; no falsifier contested it. The bootstrapping paradox is
dispositive (a dogfooded run needs the daemon + schema up, which is exactly what the deploy brings
online), so the base deployer is a plain verb against a bare/owner-only DB, the same context `daemon
owner-ddl apply` (`daemon.go:90`) occupies. The door stays open because (1) the plan is a
content-addressed durable artifact (`plan_hash` + `deploy_plan`); (2) the receipt is hash-chained
into `audit_log`; (3) the verb is a thin CLI over a `Deployer.Apply(ctx, plan, cursor)` core a
future run step invokes unchanged.

---

## 3. The deployer surface + serve-boot decoupling (shadow-first, by exact code site)

### 3.1 Command site (carry-forward)

Add `case "deploy":` to `RunDaemon` (`daemon.go:67-81`) → `runDaemonDeploy(...)`. DSN resolution
mirrors `runDaemonOwnerDDL` (`daemon.go:90,115`). Flags: `--plan`, `--dry-run`, `--resume`
(default), `--abort`, `--json`. `docs/reference/command-authority-matrix.md` + the authority
guardrail tests gain the `daemon.deploy` row.

**Connection model (load-bearing, carry-forward).** The deployer applies **every deploy-plan step
over the SINGLE owner connection**. The only write routed over a separate runtime view is the C1
finalizer's terminal `schema_state` self-record (mirroring `connection.go:399`) — gated by
`VerifyStoredTranscript` (§3.4a, M1).

### 3.2 The deploy plan — materialized ONCE into an immutable transcript (BC-N1), revoke 0021 sorted last (C3) — carry-forward

Carry-forward from v8, unchanged. Plan identity is computed once at deploy start from the two
embedded `embed.FS` sources the fingerprint hashes (runtime `migrations.go:21`; owner
`owner.go:156`) and persisted as the immutable `deploy_plan` transcript before step 0. The
substrate-ensure preamble applies 0044 idempotently before transcript materialization (0044 is never
a numbered step). `BuildPlan` uses the FULL `OwnerBundles()` loader (includes 0021) but special-cases
the revoke to terminal: pending non-revoke owner bundles (≤20, via `isNonRevokeBundle`) → pending
runtime migrations (≥0045) → the terminal 0021 if pending. `plan_hash` = sha256 of the canonical
transcript (base + ordered `step_id`+`sha256`, 0021 last); the fingerprint is over the *set*
(order-independent) so 0021-last changes `plan_hash` but not the fingerprint. `step_index` is stable
by storage, not recomputation (BC-N1).

### 3.2a The single non-revoke filter + the embed/listing split (M2 — carry-forward) + the M4 phase-aware test staging

**Claim (NON-REVOKE-FILTER — M2, carry-forward verbatim).** The DDL-revoke bundle 0021 is excluded
from **every** `owner-ddl apply` route by a single named filter and a split loader, so 0021's
`REVOKE CREATE` can ONLY ever be committed as the terminal `deploy` step (via `applyOneOwnerBundle`),
NEVER via the pending loop, the FMA-007 self-heal reapply, a nil-fallback, a dry-run/list surface,
or a test helper.

```go
// DDLRevokeOwnerBundleVersion identifies the RFC 0142 P4 C3 DDL-revoke bundle.
// DEPLOY-PLAN-TERMINAL ONLY. LatestOwnerBundleVersion / RequiredOwnerBundleVersion stay 20.
const DDLRevokeOwnerBundleVersion = 21
func isNonRevokeBundle(b OwnerBundle) bool { return b.Version < DDLRevokeOwnerBundleVersion }
func OwnerDDLApplyBundles() ([]OwnerBundle, error) { /* filter(OwnerBundles(), isNonRevokeBundle) */ }
```

**Bindings (every `owner-ddl apply` route, current `main`), unchanged from v8:** (1)
`ApplyOwnerBundles` (`owner.go:265-303`) loads `OwnerDDLApplyBundles()` at `:269`; (2)
`applyPendingOwnerBundles` (`:309-322`) keeps its `<= current` bound (`:312`) + an in-loop
`isNonRevokeBundle` guard; (3) `ReapplyAllOwnerBundles` (`:332-351`) adds the same in-loop guard +
its nil-fallback (`:336-342`) loads `OwnerDDLApplyBundles()`; (4) the self-heal branch (`:277-302`)
is bound by (1) AND (3); (5) all test callers inherit the filter; (6) no owner-ddl dry-run/list
surface exists today. **The embed/listing split:** `OwnerBundles()` (`:197-224`) stays the full
loader (includes 0021) for `revokeEmbedded`, `ExpectedFingerprint` (`schema_drift.go:88`),
`BuildPlan`, and `RuntimeOwnedTablesAlterable` (`owner_runtime_ownership.go:52`);
`OwnerDDLApplyBundles()` is the disjoint apply slice.

**Build-time guard — split phase-aware (M4 — carry-forward from v6/v7/v8).** F16 is split into two
phase-aware tests that each land green in their own rollout phase:

- **`TestOwnerDDLApplyExcludesSyntheticRevokeBundle` (F16a — pre-0021 / inert, rollout step 2).**
  Drives the filter through a **synthetic bundle list / test hook** (a hand-built `[]OwnerBundle`
  containing a synthetic `{Version: 21}` entry, and/or an injectable `ownerBundleFS` test seam):
  asserts (a) `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21` from the
  synthetic list; (b) `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a hand-passed
  synthetic 0021; (c) `ReapplyAllOwnerBundles(nil, …)` resolves its fallback to the filtered loader.
  It does **NOT** assert production `OwnerBundles()` contains 0021 (it does not yet).
- **`TestOwnerDDLApplyExcludesProductionRevokeBundle` (F16b — activation, rollout step 7, after 0021
  is authored).** Asserts (a) production `OwnerBundles()` DOES contain 0021; (b)
  `ExpectedFingerprint()` includes 0021's bytes; (c) `revokeEmbedded` derives from the full loader /
  file presence in `ownerBundleFS`; (d) production `OwnerDDLApplyBundles()` excludes 0021. The
  forced-self-heal pgtest (below) lives here.
- A build-time grep test (in step 2) asserts no production `applyOneOwnerBundle` caller outside the
  deployer terminal-step path receives a bundle `>= 21`.

### 3.3 Lifting `ApplyMigrations` out of serve-boot (shadow-first; carry-forward + the M3 tightening)

**Claim (DECOUPLE).** Serve-boot stops mutating schema, behind a flag, shadow-first. The single
mutation site on the serving path is `ConnectAndMigrate` (`connection.go:332-404`).

1. **New env `STRIATUM_DEPLOY_DECOUPLED`** (default OFF — shadow-first). OFF → boot behaves as today
   for a NO-revoke binary; the decoupling lands to `main` inert.
2. When **ON**, `ConnectAndMigrate` becomes **`ConnectAndVerify`**: keep the watermark interlock
   (`:349`) and the drift gate (`:376-383`) but do NOT call `ApplyMigrations` (`:353`); run the §1.3
   classification and serve only on the in-sync row; the drift self-record (`:399`) is NOT run on the
   verify path.
3. **M3 tightening (carry-forward).** A **revoke-embedding binary** (the binary ships 0021) is
   **structurally barred from the legacy `ConnectAndMigrate` `ApplyMigrations`/`:399` path** by the
   hoisted config gate in `CheckDeployActivation` (§3.3a): with the flag OFF it halts
   `awaiting_deploy_config` BEFORE `:353`, for EVERY cursor state including `complete`. So once a
   binary embeds the revoke, its ONLY serving mode is the decoupled `ConnectAndVerify` path — **and
   on that path its `complete`-cursor serve decision is the fingerprint-conditional A3 (row 16),
   identical to the no-revoke decoupled row 15 (M7).**
4. **No window serving an unmigrated schema (R2).** The decoupled boot verifies before serving; the
   watermark interlock is preserved verbatim, and **the fresh `applied_owner == 0` bootstrap still
   serves on the legacy path for a no-revoke flag-OFF binary (M5)** — the decoupling does NOT wedge
   fresh-DB bring-up.

#### 3.3a The deploy-activation interlock (A) — carry-forward verbatim from v6/v7/v8 (the M3 hoisted config gate + the no-revoke complete comparison) + the universal pre-revoke cursor edge (BC-N2) + the C2 arms

A **`CheckDeployActivation(ctx, runner, revokeEmbedded, decoupledEnabled bool) error`** (in
`go/pkg/db/owner.go` or a new `deploy_activation.go`) is called at the same boot site, **after**
`CheckOwnerBundleWatermark` (W, `connection.go:349`) and **BEFORE `ApplyMigrations`** (`:353`), in
both `ConnectAndMigrate` and `ConnectAndVerify`, and before `RecordSchemaFingerprint` (`:399`). It
mutates nothing. It reads `deploy_cursor` defensively (absent table/row → `cursorState = none`),
`cursor.plan_hash`, and `LiveFingerprint`/`ExpectedFingerprint`. **It does NOT read `applied_owner`
/ `owner_bundle_meta`** — the owner-watermark dimension is W's responsibility (the M5 split), and by
INVARIANT W→A-INDEPENDENCE (§0.2) A's outcome is identical for `applied_owner ∈ {0, ==20}` (both
pass W). **Predicate (fail-closed), in order — carry-forward verbatim from v6/v7/v8:**

0. **(M3 — the hoisted universal decoupling-config gate, fires FIRST for EVERY cursor state.)** If
   **`revokeEmbedded && !decoupledEnabled`** → return **`awaiting_deploy_config`**, DB untouched. *A
   binary that ships the DDL-revoke (0021) must run on the decoupled path; it must NEVER take the
   legacy `ConnectAndMigrate` mutate+self-record path, regardless of cursor state.* **This is the
   ONLY A predicate that reads `revokeEmbedded`; it does NOT fire when `decoupledEnabled == true`
   (§0.2 sub-invariant).**
1. **UNIVERSAL incomplete-deploy edge (BC-N2 — carry-forward verbatim).** If `cursorState ∈
   {in_progress, step_committed, finalizing}` → return **`awaiting_deploy`**, DB untouched.
2. If `cursorState == aborted` → return **`awaiting_deploy`**, DB untouched.
3. If `cursorState == complete`:
   - **`decoupledEnabled == true`** → defer to the drift gate on the VERIFY path: `plan_hash ==
     expected` + `LiveFingerprint == ExpectedFingerprint` → **serve verify-only** (return nil; no
     `ApplyMigrations`, no `:399`); foreign `plan_hash` or fingerprint mismatch → **`awaiting_deploy`**
     (or `schema_drift` under refuse mode). **This branch reads NEITHER `applied_owner` NOR
     `revokeEmbedded` — so it produces the IDENTICAL fingerprint-conditional outcome for the
     no-revoke decoupled binary (row 15) AND the revoke-embedding decoupled binary (row 16), in
     EVERY W-passing column (`==0`, `==20`, and — for the revoke-embedding binary that W tolerates
     forward — `>=21`). This is the M7 derivation: row 16's `==0`/`==20`/`>=21` cells are conditional
     on fingerprint-sync exactly like row 15, NOT unconditional.**
   - **`decoupledEnabled == false`** (⇒ `revokeEmbedded == false`, since step 0 caught
     revoke-embedding + flag-OFF): **(M3 — the no-revoke complete comparison), a pre-`ApplyMigrations`,
     DB-untouched, PURE-READ check:** if **`ExpectedFingerprint() == LiveFingerprint(recorded)` AND
     `cursor.plan_hash == expected`** → **return nil** (in-sync; the subsequent legacy
     `ApplyMigrations` is a no-op and the legacy `:399` self-record is an idempotent rewrite of the
     SAME fingerprint — serve). Otherwise → **return `awaiting_deploy`**, DB untouched.
4. If `cursorState == none` (absent table/row, or `idle` — NO transcript present):
   - `decoupledEnabled == true` → **`awaiting_deploy`** (the decoupled boot never auto-applies;
     fresh-DB bring-up runs `deploy`).
   - `decoupledEnabled == false` (⇒ `revokeEmbedded == false`, by step 0):
     - **return nil**: legacy path, serve, `ApplyMigrations` runs. **This is the legacy serve-legacy
       branch reached by both the fresh `applied_owner == 0` bootstrap and the inert-landing
       `applied_owner == 20` re-boot (both pass W); it carries NO deploy transcript, so it is the ONLY
       `cursorState == none` branch that legitimately reaches the mutating legacy `:399` writer
       (M5/M3).**

`revokeEmbedded` = the embedded owner-bundle FS contains the bundle at `DDLRevokeOwnerBundleVersion =
21` (NOT `Latest >= 21`; `LatestOwnerBundleVersion` stays 20). `decoupledEnabled` =
`STRIATUM_DEPLOY_DECOUPLED` is set.

**M6 + M7 — why steps 3 and 4 are owner-watermark-independent AND (on the decoupled branch)
`revokeEmbedded`-independent.** Neither step reads `applied_owner`. Step 3's `complete`/decoupled
decision is the fingerprint comparison (`LiveFingerprint(recorded) == ExpectedFingerprint()` +
`plan_hash`), reading `schema_state` (`schema_drift.go:145-161`), never `owner_bundle_meta`, and it
does not branch on `revokeEmbedded` (step 0 already handled the only `revokeEmbedded`-reading case,
and step 0 is skipped when decoupled is ON). Step 4's `none` decision reads only `decoupledEnabled` +
`revokeEmbedded`. Therefore for a fixed `(cursorState, decoupledEnabled, fingerprint-sync-state)` on
the decoupled complete branch, A returns the SAME outcome whether `applied_owner ∈ {0, ==20, >=21}`
and whether the binary embeds the revoke or not. **This is the predicate-level fact the §3.5 row-16
== row-15 identity (M7) is derived from — exactly as the §3.5 `==0`/`==20` column identity (M6) is
derived from the `applied_owner`-independence.**

**Net invariant (M3 + BC-N2 + C2 + the M5 W split + the M6/M7 derivation):** no boot serves while
the cursor is non-`complete`; no revoke-embedding binary ever reaches `ApplyMigrations`/`:399` (it is
either halted at A0 with the flag OFF, or runs the verify-only decoupled path); a genuine authority
shortfall (`1..19`) halts at W before A; and the legacy `:399` writer is reached ONLY by a no-revoke
flag-OFF binary that passed W (`applied_owner ∈ {0, ==20}`) over either NO transcript (`cursorState
== none`, rows 1/`==0` and 1/`==20`) or a `complete` transcript it is ALREADY byte-in-sync with (rows
13-in-sync/`==0` and 13-in-sync/`==20`) — **the same cells in the `==0` and `==20` columns, and row
16 reaches it in NO column (decoupled, verify-only).** Proven exhaustively in §3.5.

#### 3.3b Runtime-object ownership: Policy 1 with the revoke sequenced last (C3 — carry-forward, re-anchored)

Carry-forward from v8, unchanged. `Deployer.applyRuntimeStep` (wrapping `applyOne`,
`migrations.go:304-355`): `BEGIN` (owner conn) → assert
`has_schema_privilege('striatumd_rw','striatumd','CREATE')` (else `deploy_create_prerequisite_missing`)
→ snapshot owner-owned oids → run migration SQL + both version stamps + in-tx hash verify →
re-snapshot, `ALTER <kind> … OWNER TO striatumd_rw` for new oids → re-assert DML grants → append the
per-step receipt keyed on the stored `(plan_hash, step_index)` → advance `in_progress(k) →
step_committed(k)` → `COMMIT`. The reconcile runs while `striatumd_rw` still holds CREATE (granted by
bundle 0018 at `0018:98`, in the non-revoke prefix); 0021 commits last, so the steady state denies
CREATE.

### 3.4 The deploy receipt + the idempotent finalizer (C1) + the M1 pre-finalizer gate (carry-forward)

Carry-forward from v8, unchanged. Every applied step writes exactly one hash-chained receipt via
`append_audit_row(...)` keyed on the stored `(plan_hash, step_index)`. The idempotent finalizer:
**(0)** `VerifyStoredTranscript(plan_hash)` (M1) — runs FIRST, writes nothing, ABORTS on
`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`; **(1)** guarded `complete` receipt;
**(2)** `RecordSchemaFingerprint` (`schema_drift.go:171-195`) UPSERT; **(3)** advance `finalizing →
complete` last. The finalizer crash-point table and `doctor schema_deploy_unrecorded` (transcript-
enumerated + the M1 stamp/byte WARN) carry forward verbatim.

### 3.4a `VerifyStoredTranscript` — the full-transcript byte + DB-stamp verifier (M1, carry-forward)

Carry-forward from v8, unchanged. A single PURE-READ function checks, for EVERY step in
`deploy_plan[plan_hash].steps` (already-applied AND not), `step.sha256 ==` the running binary's
embedded bytes (`MigrationSHASet()` `migrations.go:197` for runtime; `OwnerBundle.SHA256()`
`owner.go:191` for owner) ⇒ `ErrDeployPlanBinaryMismatch`; and for every already-applied step,
`step.sha256 ==` the DB stamp (`schema_migrations.sha256` `migrations.go:275` /
`owner_bundle_meta.sha256` `owner.go:529`) ⇒ `ErrDeployPlanDBStampMismatch`. Called (i) on every
resume before any apply, (ii) as finalizer step 0, (iii) by the doctor. The two typed halts join the
`AwaitingOwnerDDLError`/`SchemaDriftError` `errors.As` arms (`main.go:208-227`); all map to
refuse-to-serve, DB untouched. **(M7 note: per the v8 ledger §5, M1's scope is deploy resume /
finalizer step 0, NOT ordinary serve-boot; the M7 fix chooses Option 1 (the conditional cell), so it
adds NO new serve-boot DB-stamp check — the row-16 in-sync cell serves because A reads the recorded
fingerprint and finds it == `ExpectedFingerprint()`. M1 is not extended and not regressed.)**

---

## 3.5 The complete boot-path decision table — DERIVED MECHANICALLY from W and A (M6 + M7 fix; the `==0`/`==20` columns identical in every cursor row, AND row 16 == row 15 on the decoupled complete branch)

> **The SEED requirement (now with the COMPLETE mechanical derivation).** A revision that pins only
> the prior findings but leaves an unaudited boot-path combination — or an incoherent one — open
> spawns the next finding (v6's SEED warned an "unaudited combination" would spawn M5; v7's left the
> `complete`/`==0` cells incoherent and spawned M6; v8's left the SIBLING `complete`/decoupled/
> revoke-embedding cell (row 16) asserted-not-derived and spawned M7). This table closes that class
> **by derivation across ALL complete-row cells**: it states the two predicates W and A, applies
> INVARIANT W→A-INDEPENDENCE + the SUB-INVARIANTS (§0.2), and reads each cell off the predicates
> rather than asserting it. For EVERY `cursorState × decoupledEnabled × revokeEmbedded ×
> applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}` cell it states the exact
> guard/outcome, PROVES §4.5 Universal Invariant B holds, AND proves the legitimate fresh-DB /
> inert-landing cells STILL SERVE (not wedged). It is an **executable, named requirement** (**F18**,
> `T-deploy-bootpath-decision-table`) the falsifiers verify against `CheckOwnerBundleWatermark`'s
> `applied == 0` exception (`owner.go:145`), the revised §3.3a predicate sites, the
> `schema_drift.go:145-161`/`:171-195` orthogonality anchors, and the
> `connection.go:349/:353/:376-383/:399` ordering.

**Two guard sites, evaluated in order on every boot, both BEFORE `ApplyMigrations` (`:353`) /
`RecordSchemaFingerprint` (`:399`), mutating nothing:**

- **W = `CheckOwnerBundleWatermark` (`connection.go:349`, `owner.go:124-154`) — the M5 dimension,
  FOUR disjoint `applied_owner` buckets covering all of ℕ (matching the live source exactly):**
  - **`applied_owner == 0` (no authority schema / fresh / single-role bootstrap)** → **return nil,
    proceed to A** (`owner.go:145`). **The M5 serve bucket — NOT a halt.**
  - **`1 <= applied_owner < 20` (genuine authority shortfall)** → **`awaiting_owner_ddl`**, DB
    untouched (`owner.go:148-150`). W halts BEFORE A — uniform across all 16 rows.
  - **`applied_owner == 20` (in-sync at the frontier)** → in-sync, **return nil, proceed to A**
    (`owner.go:151-153`).
  - **`applied_owner >= 21` (0021 applied)** → a **revoke-embedding** binary tolerates forward →
    proceed to A; a **no-revoke** binary → **`awaiting_deploy`** (the C2 forward-watermark rule
    `owner.go:151-153`, re-anchored to 21). **Barrier (b).**
- **A = `CheckDeployActivation` (§3.3a, carry-forward from v6/v7/v8).** Step 0 (M3 config gate) is
  **barrier (a)** that keeps every revoke-embedding binary off the legacy path. **A does NOT read
  `applied_owner` (§0.2) — so the A-gate outcome is IDENTICAL for the `0` and `==20` columns in
  EVERY cursor row. On the DECOUPLED complete branch A does NOT read `revokeEmbedded` either (§0.2
  sub-invariant) — so rows 15 and 16 take the IDENTICAL A3-conditional outcome in every W-passing
  column. These identities are the structural core of the M6 + M7 fixes.**

**How each cell is derived (not asserted).** For a row `(cursorState, decoupledEnabled,
revokeEmbedded)`:
1. **`1..19` column** → `awaiting_owner_ddl` (W halts). Uniform; no A involvement.
2. **`>=21` column** → for a no-revoke binary, `awaiting_deploy` (barrier b at W); for a
   revoke-embedding binary, W tolerates forward and the cell takes A's outcome for that row (so for
   the decoupled revoke-embedding complete row 16, the `>=21` cell is the SAME conditional A3
   outcome as `==0`/`==20`).
3. **`==0` and `==20` columns** → both pass W and take **A's outcome for that row**, IDENTICAL
   between the two columns (§0.2). Where A's outcome is conditional on the fingerprint-sync state
   (the `complete`/no-revoke rows 13/15 AND the `complete`/decoupled/revoke-embedding row 16), the
   cell is WRITTEN conditionally ("serve if in-sync, else `awaiting_deploy`") — the SAME condition
   for both columns and (on the decoupled branch) for both rows 15 and 16 (§0.2 sub-invariants /
   DERIVATION-RULE COMPLETENESS).

The 16 `(cursorState × decoupledEnabled × revokeEmbedded)` rows × **4** `applied_owner` sub-columns =
**64 cells**, all derived. `cursorState` groups `step_committed` with `in_progress` and treats
`aborted` like the non-complete edge (both → `awaiting_deploy` at A step 1/2), per §1.3.

| # | cursorState | decoupled | revoke | applied_owner **`==0` (fresh/no-authority)** | applied_owner **`1..19` (shortfall)** | applied_owner `==20` | applied_owner `>=21` |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | none | off | no | **SERVE-legacy — FRESH-DB BRING-UP** (W nil for `applied==0`; A4 none/legacy; `:399` reachable — NO transcript) **← the M5 serve cell** | `awaiting_owner_ddl` (W shortfall) | **SERVE-legacy — inert-landing re-boot** (A4; `:399` reachable — NO transcript) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 2 | none | off | yes | **`awaiting_deploy_config`** (W nil, then A0 barrier a) | `awaiting_owner_ddl` (W shortfall) | **`awaiting_deploy_config`** (A0 barrier a) **= `==0`** | **`awaiting_deploy_config`** (A0; W tolerates revoke, then barrier a) |
| 3 | none | on | no | `awaiting_deploy` (A4: decoupled never auto-applies; run deploy to bring up fresh DB) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A4: decoupled never auto-applies) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 4 | none | on | yes | `awaiting_deploy` (A4: run deploy to apply bundles + 0021) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A4: run deploy to apply 0021) **= `==0`** | `awaiting_deploy` (A4; cursor=none+0021-applied inconsistent → conservative halt) |
| 5 | in_progress/step_committed/aborted | off | no | `awaiting_deploy` (A1/A2, BC-N2) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1/A2, BC-N2) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 6 | in_progress/step_committed/aborted | off | yes | **`awaiting_deploy_config`** (W nil, then A0 barrier a) | `awaiting_owner_ddl` (W shortfall) | **`awaiting_deploy_config`** (A0 barrier a) **= `==0`** | **`awaiting_deploy_config`** (A0) |
| 7 | in_progress/step_committed/aborted | on | no | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1/A2, BC-N2 resume) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 8 | in_progress/step_committed/aborted | on | yes | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1/A2, BC-N2 resume) **= `==0`** | `awaiting_deploy` (A1/A2; W tolerates revoke) |
| 9 | finalizing | off | no | `awaiting_deploy` (A1, BC-N2) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1, BC-N2) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 10 | finalizing | off | yes | **`awaiting_deploy_config`** (W nil, then A0 barrier a) | `awaiting_owner_ddl` (W shortfall) | **`awaiting_deploy_config`** (A0 barrier a) **= `==0`** | **`awaiting_deploy_config`** (A0) |
| 11 | finalizing | on | no | `awaiting_deploy` (A1, BC-N2) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1, BC-N2) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 12 | finalizing | on | yes | `awaiting_deploy` (A1, BC-N2 resume finalize) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1, BC-N2 resume finalize) **= `==0`** | `awaiting_deploy` (A1; W tolerates revoke) |
| **13** | **complete** | **off** | **no** | **SERVE-legacy if in-sync, else `awaiting_deploy`** (A3 no-revoke comparison — `applied_owner`-independent; in-sync → idempotent `:399` rewrite, out-of-sync → halt DB-untouched) **← M6** | `awaiting_owner_ddl` (W shortfall) | **SERVE-legacy if in-sync, else `awaiting_deploy`** (A3 no-revoke comparison; `:399` only on the idempotent no-op rewrite) **= `==0`** | `awaiting_deploy` (W barrier b) |
| **14** | **complete** | **off** | **yes** | **`awaiting_deploy_config`** (W nil, then A0 barrier a — **the M3 cell**) | `awaiting_owner_ddl` (W shortfall) | **`awaiting_deploy_config`** (A0 barrier a — **the M3 cell**) **= `==0`** | **`awaiting_deploy_config`** (A0 — the post-deploy steady state with the flag erroneously OFF; the v5 reproducer, halted) |
| **15** | **complete** | **on** | **no** | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled — `applied_owner`-independent; in-sync → `ConnectAndVerify` serve, NO `:399`) **← M6** | `awaiting_owner_ddl` (W shortfall) | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled) **= `==0`** | `awaiting_deploy` (W barrier b) |
| **16** | **complete** | **on** | **yes** | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled — `applied_owner`- AND `revokeEmbedded`-independent; **normal pre-0021 state is OUT-OF-SYNC → `awaiting_deploy`** (0021 not applied → fingerprint ≠); degenerate in-sync corner → SERVE-verify, NO `:399`) **← M7 FIX: now conditional, mirroring row 15** | `awaiting_owner_ddl` (W shortfall) | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled; normal pre-0021 OUT-OF-SYNC) **= `==0`** | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled — **normal post-0021 steady state is IN-SYNC → SERVE-verify** (the activated serving cell, zero serve-path DDL); a further pending change → `awaiting_deploy`) **← M7 FIX: now conditional** |

**What changed vs v8 (M7):** the three W-passing cells in row 16 (`complete`, decoupled, revoke) —
**row 16/`==0`** (was unconditional `awaiting_deploy`), **row 16/`==20`** (was unconditional
`awaiting_deploy`), and **row 16/`>=21`** (was unconditional `SERVE-verify`) — are now all
**conditional**, "SERVE-verify if in-sync, else `awaiting_deploy`", **IDENTICAL to row 15's decoupled
cells**, because A's decoupled complete branch reads neither `applied_owner` NOR `revokeEmbedded`
(§0.2 sub-invariant / §3.3a step 3). Every other cell is verbatim from v8. The `= ==0` annotation on
every `==20` cell makes the column identity auditable at a glance (M6); the row-16 cells now carry
the SAME conditional form as row 15, making the row identity auditable (M7).

**Cross-row `==0`-vs-`==20` audit + the row-15-vs-row-16 audit (M6 + M7 — the class is closed by
derivation, walked explicitly so no sibling cell is left unexamined).** For each cursor-state group,
the `==0` and `==20` columns match for the same reason (A does not read `applied_owner`); and on the
decoupled complete branch, rows 15 and 16 match for the same reason (A does not read `revokeEmbedded`
there):

- **`none` rows (1–4).** Row 1: both SERVE-legacy via A4 (the only label difference, fresh vs inert,
  is descriptive). Rows 2/4: both `awaiting_deploy_config` (A0) / `awaiting_deploy` (A4). Row 3: both
  `awaiting_deploy` (A4). **All `==0`/`==20` match.** (Row 1 is the carried-forward M5 row-1 serve.)
- **`in_progress`/`step_committed`/`aborted` rows (5–8).** Every `==0` and `==20` cell halts at A1/A2
  (BC-N2) — `awaiting_deploy` — or at A0 (row 6). **All match.**
- **`finalizing` rows (9–12).** Every `==0` and `==20` cell halts at A1 (BC-N2 resumable-finalize) —
  `awaiting_deploy` — or at A0 (row 10). **All match.**
- **`complete` rows (13–16).** Row 13 (off, no-revoke): both columns "SERVE-legacy if in-sync, else
  `awaiting_deploy`" (A3 no-revoke comparison) — **M6.** Row 14 (off, revoke): both
  `awaiting_deploy_config` (A0). Row 15 (on, no-revoke): both "SERVE-verify if in-sync, else
  `awaiting_deploy`" (A3 decoupled) — **M6.** Row 16 (on, revoke): **`==0`/`==20`/`>=21` all
  "SERVE-verify if in-sync, else `awaiting_deploy`" (A3 decoupled) — M7 FIX, IDENTICAL to row 15's
  conditional (the `>=21` column reaches A because W tolerates the revoke-embedding binary forward;
  rows 13/15 `>=21` halt at W barrier b because they are no-revoke).** **All `==0`/`==20` match, AND
  rows 15 and 16 take the identical A3-conditional form on the decoupled branch.**

The audit proves the failure pattern (fix the named cell, leave a sibling cell unexamined) does NOT
recur: the `==0`/`==20` identity AND the row-15/row-16 decoupled identity are established in EVERY
`complete`-group cell. An M8 of the form "row X column Y differs from its sibling" or "a complete-row
cell is asserted where A is conditional" is foreclosed by construction — every A-reaching
complete-row cell {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`} is
written conditionally; row 14 halts at A0 (the M3 config gate, unconditional by design); the `1..19`
column and the rows-13/15 `>=21` cells halt at W (uniform).

**Universal Invariant B proven in every cell (UNCHANGED by M7 — row 16 is decoupled).** The legacy
`connection.go:399` writer mutates schema / self-records around `VerifyStoredTranscript` only if a
boot reaches `:353`/`:399` on the legacy `ConnectAndMigrate` path. Across all 64 cells the ONLY cells
that reach `:399` are the **four** below — and the `==0` and `==20` columns reach it in the SAME rows
(1 and 13-in-sync); **row 16 reaches it in NO column** (it is decoupled → `ConnectAndVerify`, which
never calls `:353`/`:399`):

- **Cell 1 / `==0`** (`none`, off, no-revoke, fresh DB): NO transcript present; the running binary IS
  the applier of its own just-applied schema (fresh-DB bring-up). Invariant B not in scope. ✓ **← the
  M5 serve cell, matching `owner.go:145`.**
- **Cell 1 / `==20`** (`none`, off, no-revoke, inert-landing): NO transcript present; an inert binary
  recording its own just-applied schema on an already-bundled DB. Invariant B not in scope. ✓
- **Cell 13 / `==20`, in-sync** (`complete`, off, no-revoke, `ExpectedFingerprint() == recorded`):
  the pre-`ApplyMigrations` comparison (A3) already proved in-sync, so `ApplyMigrations` is a no-op
  and `:399` rewrites the IDENTICAL fingerprint. No mutation. ✓
- **Cell 13 / `==0`, in-sync** (`complete`, off, no-revoke, an `owner_bundle_meta`-absent DB whose
  recorded `schema_state.fingerprint` nonetheless equals `ExpectedFingerprint()` — constructible
  because the tables are orthogonal, `schema_drift.go:145-161`): the SAME A3 in-sync guard fires,
  `ApplyMigrations` is a no-op, and `:399` is an idempotent rewrite of the IDENTICAL fingerprint. No
  mutation. ✓ **← M6: enumerated in BOTH §4.5 and the F18 spy list.**

**Row 16's serve-verify cells (the M7-fixed in-sync sub-cases) serve via `ConnectAndVerify` and reach
`:399` in NO column** — the decoupled verify path never self-records (§3.3 step 2). So the M7 fix
adds NO cells to the `:399` spy list; §4.5 and the F18 spy list still enumerate the SAME four cells.

Every other cell either halts before `:353` (DB byte-identical) or takes the decoupled
`ConnectAndVerify` path:

- **The entire `1..19` column (16 cells)** → `awaiting_owner_ddl` at **W**, BEFORE A and BEFORE
  `:353`. DB untouched. ✓ **← the M5 halt cell.**
- **Every revoke-embedding + flag-OFF cell (2, 6, 10, 14, in the `==0`/`==20`/`>=21` columns)** →
  `awaiting_deploy_config` at A0 (barrier a). DB byte-identical. **The M3 close** — cell 14/`>=21` is
  the v5 reproducer; cell 2/`==0` proves a revoke-embedding binary on a fresh DB with the flag OFF
  ALSO halts (the preserved asymmetry — the serve cell is no-revoke only).
- **Every no-revoke binary on a revoke-applied DB (`>=21` in rows 1, 3, 5, 7, 9, 11, 13, 15)** →
  `awaiting_deploy` at W (barrier b). Never reaches `:353`.
- **Every non-`complete` cursor not already caught by A0** (rows 5, 7, 8, 9, 11, 12 in the `==0` and
  `==20` columns; and the `aborted` variants) → `awaiting_deploy` at A1/A2 (BC-N2). DB untouched.
- **Every decoupled cell (3, 4, 7, 8, 11, 12, 15, 16)** runs `ConnectAndVerify`, which NEVER calls
  `ApplyMigrations`/`:399`; it serves only on the in-sync `complete` rows (15-in-sync/`==0`,
  15-in-sync/`==20`; **16-in-sync/`==0`, 16-in-sync/`==20`, 16-in-sync/`>=21` — the M7-fixed
  serve-verify, which serves but does NOT reach `:399`**) and otherwise halts `awaiting_deploy`. The
  deployer is the only writer of `schema_state` on this path, gated by `VerifyStoredTranscript` (M1).

**The shadow-mode drift-gate fall-through (`connection.go:384-399`) is also covered (unchanged).**
The only legacy-path cells that even reach the drift gate are 1/`==0`, 1/`==20`, 13/`==20`-in-sync,
and 13/`==0`-in-sync. Cell 1 has no recorded fingerprint divergence to log on a fresh/just-applied DB;
cell 13/in-sync has `ExpectedFingerprint() == recorded` so `drifted == false`. **No legacy shadow-mode
self-record can overwrite `schema_state` around a transcript carrying a pending change.** Row 16 never
reaches the legacy drift gate (decoupled).

**The legitimate fresh-DB / inert-landing cells STILL SERVE (R2, not wedged — the M5 requirement,
preserved).** **Cell 1/`==0`** serves the fresh-DB bring-up; **Cell 1/`==20`** serves the
inert-landing re-boot. The conservative halts fire ONLY where they must: the `1..19` genuine
shortfall (W), a revoke-embedding binary with the flag OFF (cells 2/6/10/14, A0), an
incomplete/aborted cursor (BC-N2), a no-revoke binary on a revoke-applied DB (barrier b), or a
`complete` cursor over a DB NOT byte-in-sync (A3 — rows 13/15/16, both columns). **Nothing that must
serve is wedged; the `==0`/`==20` column identity (M6) and the row-15/row-16 decoupled identity (M7)
are closed by derivation; and the activated steady-state row-16/`>=21` cell serves verify-only with
zero serve-path DDL.**

---

## 4. Serving-role DDL revocation (owner bundle 0021), without lockout, without a pre-revoke serve window, without an early self-heal revoke, without a legacy-path revoke bypass, without wedging fresh-DB bring-up, AND with a fully-derived complete-cursor decision table

**Claim (DDL-REVOKE).** The serving role `striatumd_rw` loses its remaining object-creation DDL on
the serving path, shipped as owner bundle 0021, applied **only as the terminal step of `striatum
daemon deploy`** (C3), **gated by the universal cursor edge (BC-N2)**, **excluded from every
`owner-ddl apply` route incl. the FMA-007 self-heal (M2)**, **never reachable via the legacy
serve-boot `ConnectAndMigrate` path on a `complete` cursor (M3)**, **without wedging the legitimate
fresh `applied_owner == 0` bootstrap (M5)**, **over a decision table whose `complete`/no-revoke rows
are coherent (M6)**, **AND — clarified in v9 (M7) — whose `complete`/decoupled/revoke-embedding row
(16) is DERIVED from the `applied_owner`- AND `revokeEmbedded`-independent A3 fingerprint predicate
(conditional, identical to row 15), closing the complete-row class fully** — so failure mode 1
becomes structurally impossible across the whole boot-path cell space without over-halting a healthy
first boot and without an incoherent/non-executable oracle.

### 4.1 What is revoked, precisely (carry-forward)

Bundle 0021 revokes object creation on the runtime schema: `REVOKE CREATE ON SCHEMA striatumd FROM
striatumd_rw;` (+ `REVOKE TEMPORARY` if granted). `striatumd_rw` retains ownership of (and
`ALTER`/`DROP` on) the runtime tables it already owns; new runtime objects are reconciled back to
`striatumd_rw` before 0021 commits (§3.3b). Full capability revocation (owner re-owns runtime tables)
remains a named follow-up beyond P4.

### 4.2 Typed halts + the forward-watermark rule (C2) + the universal cursor edge (BC-N2) + the M1 mismatch halts + the M3 config gate + the M5 fresh-DB serve

Sibling halts of `AwaitingOwnerDDLError` (`owner.go:42,58`), reusing the non-restartable exit
(`main.go:208-227`):

- **`AwaitingOwnerDDLError`** (`awaiting_owner_ddl`): the genuine authority shortfall, `1 <=
  applied_owner < 20` (`owner.go:148-150`). DB untouched. **NOT fired for `applied_owner == 0` (the
  M5 fresh-DB serve, `owner.go:145`).**
- **`AwaitingDeployError`** (`awaiting_deploy`): a deploy is pending/incomplete (BC-N2), OR a
  decoupled binary (no-revoke row 15 OR revoke-embedding row 16) over a `complete` transcript NOT
  in-sync (A3 decoupled — `applied_owner`- and `revokeEmbedded`-independent, so BOTH the `==0` and
  `==20` columns of rows 13/15/16-out-of-sync, AND the row-16/`>=21` out-of-sync sub-case), OR a
  no-revoke binary on a revoke-applied DB (`applied_owner >= 21`, barrier b). DB untouched. **(M7:
  the normal pre-0021 row-16 `==0`/`==20` state is out-of-sync and halts here — the dominant
  real-world case.)**
- **`AwaitingDeployConfigError`** (`awaiting_deploy_config`): the binary ships 0021 but
  `STRIATUM_DEPLOY_DECOUPLED` is OFF — for EVERY cursor state including `complete` (the hoisted M3
  gate, §3.3a step 0), and in EVERY `applied_owner` column that passes W. DB untouched. Remediation:
  "this binary ships the DDL-revoke; set `STRIATUM_DEPLOY_DECOUPLED=1` to serve verify-only, or run
  `striatum daemon deploy` to apply a pending change."
- **`DeployPlanBinaryMismatchError`** / **`DeployPlanDBStampMismatchError`** (M1). DB untouched.

**Forward-watermark rule (`CheckOwnerBundleWatermark` tolerate-forward branch `owner.go:151-153`),
re-anchored to 21 — carry-forward.** `applied >= 21` + this binary does NOT embed 0021 →
`AwaitingDeployError`. This is barrier (b) (the `>=21` column for a no-revoke binary). For a
revoke-embedding binary W tolerates `>=21` forward → A, and the decoupled row-16/`>=21` cell takes
A3's conditional outcome (normally in-sync → serve-verify). `RequiredOwnerBundleVersion` stays **20**
(`owner.go:35`); the revoke is gated by the cursor edge + `CheckDeployActivation` + the flag + its
deploy-plan-terminal placement + the `OwnerDDLApplyBundles` exclusion — NOT the watermark frontier.
**The M7 fix does NOT advance `Required` or change the `applied >= 21` forward rule; it makes the
revoke-embedding decoupled row (16) conditional on A3 fingerprint-sync.**

### 4.3 The deploy choreography (C2 — carry-forward; M3-hardened; M5-clarified; M6/M7-coherent)

Two binaries, in order:

1. **Inert-landing binary** (`Latest = Required = 20`; does NOT embed 0021). Ships the `Deployer`
   engine + `deploy` verb, migration 0044, the decoupled boot path (flag OFF), the doctor block, the
   forward-watermark rule, the universal pre-revoke cursor edge (BC-N2), `VerifyStoredTranscript`
   (M1), `OwnerDDLApplyBundles` (M2), and the `CheckDeployActivation` with the hoisted M3 config gate
   + the no-revoke complete comparison. **Boot auto-applies legacy only because no deploy is in
   flight and the binary embeds no revoke — and W serves `applied_owner == 0` (fresh DB) AND `== 20`
   (already-bundled), so this binary brings up a fresh DB (cell 1/`==0`) and re-boots an existing DB
   (cell 1/`==20`) without wedging either (M5); and on a `complete` cursor it serves iff byte-in-sync,
   identically for the `==0` and `==20` columns (rows 13/15, M6).** **Daemon up, serving.**
2. **Activation binary** (embeds 0021; `Latest = Required` still 20). On boot with the flag OFF it
   halts **`awaiting_deploy_config` for EVERY cursor state** (cells 2/6/10/14 — the M3 gate). The
   operator, with the daemon parked: sets `STRIATUM_DEPLOY_DECOUPLED=1`; runs `striatum daemon
   deploy`; restarts the daemon → `ConnectAndVerify` sees `complete` + in-sync → serves verify-only,
   zero serve-path create-DDL (**cell 16/`>=21`-in-sync — the M7-derived conditional serve, the
   normal post-0021 steady state**). **From activation onward the operator keeps
   `STRIATUM_DEPLOY_DECOUPLED=1` set.**

   **Crash mid-deploy is safe even if the inert binary comes back up (BC-N2).** Its universal cursor
   edge halts `awaiting_deploy` (cells 5/7/9). **`owner-ddl apply` before deploy is safe (M2).** **A
   later revoke-embedding binary that restarts with the flag OFF — at ANY cursor state — halts
   `awaiting_deploy_config` (M3, cells 2/6/10/14).** **A fresh `applied_owner == 0` DB first boot of
   the inert binary still serves (cell 1/`==0`, M5).** **And the `complete`-cursor decision is
   owner-watermark-independent AND (on the decoupled path) `revokeEmbedded`-independent — a
   `complete`/in-sync cursor serves whether `owner_bundle_meta` reads 0/20/`>=21` and whether the
   binary embeds the revoke or not (rows 15 and 16 identical), and a `complete`/pending-change cursor
   halts in every column (M6 + M7) — so the executable F18 oracle is true for every complete-row
   cell, including the constructible in-sync row-16 cell.**

### 4.4 How 0021 ships without lockout (R3), without a pre-revoke serve window (BC-N2), without an early self-heal revoke (M2), without a legacy-path revoke bypass (M3), without wedging fresh-DB bring-up (M5), and over a FULLY-derived complete-cursor table (M6 + M7)

The deployer applies all DDL over the owner/admin connection. **C3:** 0021 is the terminal plan step
and excluded from the owner prefix. **M2:** every `owner-ddl apply` route iterates
`OwnerDDLApplyBundles()` (0021 excluded) + both loops guard `isNonRevokeBundle` in-loop. **BC-N2:**
every deployer-aware binary refuses on a non-`complete` cursor before `ApplyMigrations`/`:399`.
**M3:** every revoke-embedding binary with the flag OFF refuses (`awaiting_deploy_config`) for EVERY
cursor state, and the forward-watermark rule refuses a no-revoke binary on a revoke-applied DB.
**M5:** the owner-watermark gate W serves the fresh `applied_owner == 0` bootstrap (and the `== 20`
re-boot) on the legacy path, halting ONLY the genuine `1..19` shortfall. **M6:** the no-revoke
`complete`-cursor cells (13/15) are derived from the `applied_owner`-independent A predicate, so the
`==0` and `==20` columns are identical. **M7:** the revoke-embedding decoupled `complete`-cursor cell
(16) is derived from the `applied_owner`- AND `revokeEmbedded`-independent A3 predicate, conditional
on fingerprint-sync, identical to row 15 — closing the complete-row class fully; §4.5 + the F18 spy
list still enumerate the SAME four `:399`-reaching cells (row 16 never reaches `:399`). Combined,
**no ordering and no boot-path cell (§3.5) drives boot into `ApplyMigrations` under a revoked CREATE,
lets any binary serve an incomplete deploy or an out-of-sync schema, strands a deploy at a reconcile
under a revoked CREATE, commits the revoke outside the terminal deploy step, self-records around the
full-transcript gate, wedges a legitimate fresh / single-role boot, OR leaves a complete-row cell
whose specified outcome the A predicate cannot produce.**

### 4.5 Proactive hardening — every apply path and every fingerprint path named, two universal invariants (TIGHTENED for M3; the M5 split + the M6/M7 derivation re-proven)

Audited once, exhaustively, against the v9 worktree HEAD `83fe6465`.

**A. EVERY code path that applies owner bundles (file:line) — carry-forward from v6/v7/v8,
unchanged.** `ApplyOwnerBundles` (`owner.go:265-303`, loads `OwnerDDLApplyBundles()` `:269`),
`applyPendingOwnerBundles` (`:309-322`, in-loop guard), `ReapplyAllOwnerBundles` (`:332-351`, in-loop
guard + filtered nil-fallback), `applyOneOwnerBundle` (`:511-541`, the only committer of 0021 via the
terminal deploy step), `runDaemonOwnerDDL` (`daemon.go:90` → `ApplyOwnerBundles`), the deployer
terminal step, the test setups, and the non-apply `OwnerBundles()` consumers
(`ExpectedFingerprint`/`revokeEmbedded`/`BuildPlan`/`RuntimeOwnedTablesAlterable`).

> **UNIVERSAL INVARIANT A (executable, carry-forward).** *The DDL-revoke bundle (`>=
> DDLRevokeOwnerBundleVersion = 21`) is excluded from ALL `owner-ddl apply` routes via
> `OwnerDDLApplyBundles()` + the in-loop `isNonRevokeBundle` guard; only the terminal `deploy` step
> commits it, via `applyOneOwnerBundle`.* Enforced by **F16a** (synthetic) + **F16b** (production).

**B. EVERY code path that self-records / writes a schema fingerprint (file:line):**

| Path | Site | Disposition under M1 + M3 + M5 + M6 + **M7** |
| --- | --- | --- |
| `RecordSchemaFingerprint` (the ONLY `schema_state.fingerprint` writer) | `schema_drift.go:171-195` | unchanged; gated by callers; orthogonal to `owner_bundle_meta`/`applied_owner` (writes only the `schema_state` singleton) |
| Legacy boot self-record | `connection.go:399` ("This is the only writer of schema_state") | **reachable ONLY in the FOUR cells `{1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`}` — a no-revoke flag-OFF binary that passed W (`applied_owner ∈ {0, ==20}`) over NO transcript (`cursorState == none`) OR a `complete` transcript ALREADY byte-in-sync. UNCHANGED by M7 — row 16 is decoupled and reaches `:399` in NO column. W's `1..19` shortfall halts BEFORE this site; W's `>=21`-no-revoke barrier (b) halts before it; barrier (a) (the M3 config gate) bars EVERY revoke-embedding flag-OFF binary; the A3 no-revoke comparison bars a pending change over a transcript. NEVER on the decoupled `ConnectAndVerify` path.** |
| C1 finalizer `schema_state` UPSERT + `finalizing → complete` | P4 `Deployer.Apply` (§3.4) | gated by `VerifyStoredTranscript` finalizer step 0 (M1) — writes nothing on mismatch |
| `ExpectedFingerprint()` / `LiveFingerprint` / `CheckSchemaDrift` (pure reads) | `schema_drift.go:83-100,145-161,254-274` | read-only; no write (the A3 comparison reuses these; `LiveFingerprint` reads `schema_state`, never `owner_bundle_meta`) |

> **UNIVERSAL INVARIANT B (executable, TIGHTENED for M3; the M5 split + the M6/M7 derivation
> re-proven).** *NO `schema_state` fingerprint / `complete` is EVER written around
> `VerifyStoredTranscript` while a deploy transcript carrying a pending change is present, and NO
> revoke-embedding binary EVER reaches the legacy `:399` writer.* Two writers, both bounded: (i) the
> deployer finalizer is gated by `VerifyStoredTranscript` step 0 (M1); (ii) the legacy
> `connection.go:399` writer is reachable ONLY in the **four** §3.5 cells **1/`==0`, 1/`==20`,
> 13-in-sync/`==0`, and 13-in-sync/`==20`** — a no-revoke flag-OFF binary that passed W over NO
> transcript or a `complete` transcript ALREADY byte-in-sync — barred from any genuine `1..19`
> shortfall (W), any revoke-embedded state (barrier a OR — on the decoupled path — the
> non-`:399`-reaching verify path), any revoke-applied state (barrier b for no-revoke), and any
> pending-change-over-a-transcript state (the A3 comparison). **M6: the `==0` and `==20` columns reach
> the legacy writer in the SAME two rows (1 and 13-in-sync). M7: row 16 (revoke-embedding decoupled)
> reaches the legacy writer in NO column — its serve-verify cells serve via `ConnectAndVerify`
> without self-recording — so the M7 fix adds NO cell to the spy list; this enumeration is identical
> to the F18 spy list (§5), preserving the §4.5↔oracle agreement.** *Therefore a database carrying a
> `deploy_cursor`/`deploy_plan` transcript with a pending change, any revoke-embedding binary, or any
> genuine authority shortfall can NEVER reach the legacy `:399` writer — AND the legitimate fresh
> `applied_owner == 0` bootstrap STILL reaches it and serves (Invariant B not in scope there: no
> transcript), AND the degenerate `complete`/`==0`-in-sync cell reaches it only as an idempotent
> rewrite of the identical fingerprint.* Enforced by **F17** (the M3 cell), **F11** (extended),
> **F15** (extended), and **F18** (the §3.5 64-cell decision-table matrix, with the 4-cell `:399`
> spy list = this enumeration, rows 13/15 `==0` asserted conditional == `==20`, and **row 16
> `==0`/`==20`/`>=21` asserted conditional == row 15, serving via the verify path with the `:399`
> spy NOT fired**).

These invariants + the §3.5 decision table are the executable, named requirements the cycle-1
falsifiers verify against the named code sites — so no incoherent or unaudited boot-path combination
(including the `complete`/decoupled/revoke-embedding row-16 cells) can spawn an M8.

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> `T-` = unit/integration; `G-` = game-day against a real two-role cluster. F1–F17 carry forward
> from v6/v7/v8 (re-confirmed against current `main`); **F18 is REVISED for M7 — the
> MECHANICALLY-DERIVED 64-cell table is now PARAMETRIC over ALL seven A-reaching complete-row cells
> (13/15/16) with the in-sync/out-of-sync sub-dimension, the row-16 cells asserted conditional ==
> row 15, the 4-cell `:399` spy list UNCHANGED (row 16 never reaches `:399`) — and F18a (the fresh-DB
> serve) carries forward** — these are what the cycle-1 falsifiers re-attack first.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1–F10, F12–F14** | Carry forward verbatim from v6/v7/v8 (resumability Q3-A/Q3-B, no serve-boot mutation when decoupled, §1.3 classification incl. the M1 mismatch rows, no P2/P3 regression, DDL-revoke structural, per-step receipt, no double-apply, plan≡fingerprint, the C1 finalization boundary with the M1 step-0 gate, the C3 runtime-object ownership extended for M2, per-step crash-safety, immutable plan identity with M1 already-applied verification). Re-confirmed; anchors in §9. | (as v8) |
| **F11 (Activation fail-closed — C2 + BC-N2 universal edge; M3 (g))** | Carry forward from v6/v7/v8: for every deployer-aware binary, boot never calls `ApplyMigrations`/`RecordSchemaFingerprint` while the cursor is non-`complete`; a no-revoke binary at `applied_owner == 20` with a non-`complete` cursor halts `awaiting_deploy` DB-byte-identical; a no-cursor/`idle` no-revoke binary SERVES; (g) a revoke-embedding binary with the flag OFF halts `awaiting_deploy_config` for EVERY cursor state. | `T-deploy-revoke-activation-ordering` cases (a)–(g) (carry-forward); spies prove `applyOne`/`RecordSchemaFingerprint` never entered. `G-old-binary-refuse`. |
| **F15 (Full-transcript verification on resume AND before the finalizer — M1)** | Carry-forward from v6/v7/v8 (the byte case, the owner-step case, the DB-stamp case, the finalizer arm); the finalizer's `VerifyStoredTranscript` step-0 gate AND the §3.3a M3 config gate together mean NO writer self-records around the full-transcript check. **(M7: M1 is NOT extended to serve-boot — the row-16 fix is the conditional cell, not a new serve-boot stamp check.)** | `T-deploy-resume-already-applied-byte-mismatch-refuses` (as v8) + an assertion that the §3.3a guard precedes `connection.go:353` in BOTH boot paths. `G-wrong-binary-resume`. |
| **F16a / F16b (Revoke excluded from EVERY owner-ddl apply route — M2; synthetic + production phases)** | Carry forward from v6/v7/v8: F16a (synthetic list / injectable FS, step 2) excludes every bundle `>= 21` from both apply loops + the nil-fallback WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (step 7, after 0021 authored) asserts the production embed/listing split + the forced FMA-007 self-heal reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`. | `TestOwnerDDLApplyExcludesSyntheticRevokeBundle` + grep test (step 2); `TestOwnerDDLApplyExcludesProductionRevokeBundle` + `T-deploy-revoke-excluded-from-reapply-self-heal` (step 7). |
| **F17 (M3 — the complete-cursor legacy mutate+self-record refusal)** | Carry forward from v6/v7/v8: a revoke-embedding binary with a `complete` cursor, flag OFF, and a pending change refuses `awaiting_deploy_config`, calls NEITHER `ApplyMigrations` NOR `RecordSchemaFingerprint`, leaves `schema_state` unchanged and the DB byte-identical, on BOTH boot paths; symmetric shadow-mode arm asserts the fall-through `connection.go:384-399` is never reached. | `T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`; `G-complete-cursor-flag-off-refuse`. |
| **F18 (Boot-path decision table — proactive completeness — REVISED for M7: PARAMETRIC over ALL seven A-reaching complete-row cells; row 16 conditional == row 15; the 4-cell `:399` spy list UNCHANGED)** | EVERY `cursorState ∈ {none, in_progress, step_committed, finalizing, complete, aborted} × decoupledEnabled ∈ {on, off} × revokeEmbedded ∈ {yes, no} × applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}` cell produces the EXACT §3.5 outcome; in NO cell does the legacy `:399` writer fire over a transcript carrying a pending change or for a revoke-embedding binary; **AND the M6 requirements (carry-forward): (1) for every fixed `(cursorState, decoupledEnabled, revokeEmbedded)` row, the `==0` and `==20` columns produce the IDENTICAL outcome (A is owner-watermark-independent); (2) rows 13 and 15 in the `==0` column are CONDITIONAL "serve if in-sync, else `awaiting_deploy`" matching `==20`; (3) the `:399` spy fires in EXACTLY the four cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} = the §4.5 enumeration; AND the NEW M7 requirements: (4) the decoupled complete branch is `revokeEmbedded`-independent — for every fixed `(complete, decoupledEnabled=on)` the no-revoke row 15 and the revoke-embedding row 16 produce the IDENTICAL outcome in every W-passing column; (5) row 16 in the `==0`, `==20`, AND `>=21` columns is CONDITIONAL "serve verify-only if in-sync, else `awaiting_deploy`" (the in-sync sub-case constructed by setting `schema_state.fingerprint == ExpectedFingerprint()` AND `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent / version-20 / version-`>=21` DB; the out-of-sync sub-case → `awaiting_deploy`); (6) F18 is PARAMETRIC over the SEVEN A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`}, constructing BOTH the in-sync and out-of-sync sub-cases for each (the rows-13/15 `>=21` cells and the entire `1..19` column halt at W BEFORE A, so they carry no A sub-dimension); (7) the decoupled SERVE-verify cells (15-in-sync and 16-in-sync, all W-passing columns) serve but the `:399` spy is NOT fired (the verify path never self-records) — so the M7 fix adds NO cell to the spy list; (8) the normal pre-0021 row-16 `==0`/`==20` state is OUT-OF-SYNC (→ `awaiting_deploy`) and the normal post-0021 row-16 `>=21` state is IN-SYNC (→ serve-verify); the opposite sub-cases are the degenerate corners A must handle.** | **`T-deploy-bootpath-decision-table`**: a parametrized matrix over all 64 cells with the in-sync/out-of-sync sub-dimension for the SEVEN A-reaching complete-row cells (13/15/16); for each, construct the DB + binary state and assert the §3.5 outcome; assert the column-identity property (`==0` outcome == `==20` outcome) for every row AND the row-identity property (row 15 outcome == row 16 outcome on the decoupled complete branch) for every W-passing column; assert `ApplyMigrations`/`RecordSchemaFingerprint` spies fire ONLY in the four cells above (and NOT in any row-16 serve-verify cell). The `applied_owner == 0` arm sets `owner_bundle_meta` ABSENT (`OwnerBundleVersion → 0`); the row-16 in-sync arm independently sets `schema_state.fingerprint == ExpectedFingerprint()` over an `owner_bundle_meta`-absent/20/`>=21` DB (proving the orthogonality, `schema_drift.go:145-161`). Asserts the W gate precedes A and both precede `connection.go:353` on both boot paths. |
| **F18a (Fresh-DB bootstrap serves — the M5 serve cell, carry-forward)** | A no-revoke binary (`Latest = Required = 20`, 0021 NOT embedded), flag OFF, booting against a FRESH database with `striatumd.owner_bundle_meta` ABSENT (`OwnerBundleVersion → 0`, `owner_pg_test.go:19`) and `cursorState = none` (no `deploy_cursor` transcript) SERVES: `CheckOwnerBundleWatermark` returns nil (the `applied == 0` exception, `owner.go:145`), `CheckDeployActivation` step 4 returns nil, legacy `ConnectAndMigrate` runs `ApplyMigrations` + the `:399` self-record, and the daemon serves — it is NOT wedged `awaiting_owner_ddl`. **Conversely**, the SAME binary against a DB with `owner_bundle_meta` at version `k ∈ [1,19]` halts `awaiting_owner_ddl` DB-untouched. | **`T-deploy-fresh-db-bootstrap-serves`**: (1) fresh DB (no `owner_bundle_meta`, no `deploy_cursor`) + inert binary, flag OFF → assert serve, `ApplyMigrations` spy CALLED, `RecordSchemaFingerprint` spy CALLED, daemon up; (2) DB with `owner_bundle_meta.version ∈ {1, 19}` → assert `awaiting_owner_ddl`, both spies un-called, DB byte-identical. `G-fresh-db-first-boot`: a real fresh two-role cluster first boot of the inert binary serves. |

---

## 6. Shadow-first rollout (the P4 build order) — re-anchored + M3 + M4 + M5 + M6 + M7

1. **Migration 0044 `deploy_cursor` + `deploy_plan`** (additive runtime tables; `state` CHECK
   includes `finalizing`; runtime-owned).
2. **`go/pkg/db/owner.go` M2 surface (lands first, inert):** `DDLRevokeOwnerBundleVersion = 21`,
   `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, the in-loop guards, the nil-fallback split, **and
   the F16a SYNTHETIC-phase test + the build-time grep test (M4)**. Inert until 0021 is authored
   (step 7).
3. **`go/pkg/db/deploy.go`**: `DeployPlan`, `BuildPlan` (0021-terminal, FULL `OwnerBundles()`),
   `LoadStoredPlan`, `VerifyStoredTranscript` (M1) + the typed mismatch halts, `Deployer.Apply`
   (Q3-A/Q3-B engine + the `finalizing` finalizer with `VerifyStoredTranscript` step 0), the
   substrate-ensure preamble, `applyRuntimeStep` (C3 reconcile), the receipt writer. Pure-core +
   DB-integration tests (F1, F2, F4, F8, F9, F10, F12, F13, F14, F15) proven BEFORE any boot path
   changes.
4. **`runDaemonDeploy`** verb + matrix/authority-guardrail row + `--dry-run`/`--abort` + the
   0021-activation preflight. F3/F5 wiring.
5. **`CheckDeployActivation`** with the M3 predicate — **the hoisted `revokeEmbedded &&
   !decoupledEnabled → awaiting_deploy_config` config gate (every cursor state, step 0) + the
   no-revoke `complete` pre-`ApplyMigrations` comparison (step 3), neither reading `applied_owner`
   (M6), and the decoupled complete branch reading neither `applied_owner` NOR `revokeEmbedded` (M7,
   §0.2 sub-invariant)** — on top of the universal pre-revoke cursor edge (BC-N2) + the typed halts +
   **the M5-correct `CheckOwnerBundleWatermark` owner-watermark gate (the `applied_owner == 0` serve
   exception preserved, the `1..19` shortfall halt, the forward-watermark rule at `>= 21`)**.
   Decoupled boot path (`ConnectAndVerify`) behind `STRIATUM_DEPLOY_DECOUPLED` (default OFF). **Lands
   inert for a no-revoke binary; the fresh `applied_owner == 0` bootstrap still serves; the
   no-revoke `complete`-cursor cells (13/15) are derived from the `applied_owner`-independent A
   predicate so `==0`/`==20` are identical; and the revoke-embedding decoupled `complete` cell (16) is
   derived from the `applied_owner`- AND `revokeEmbedded`-independent A3 predicate, conditional and
   identical to row 15** (F11 incl. (g), F3, F5, F17, **F18 with the parametric complete-row
   extension + the column-identity + the row-15/row-16 identity + the unchanged 4-cell spy list,
   F18a**).
6. **`doctor schema_deploy_unrecorded`** block — per-step tightened, transcript-enumerated, + the M1
   stamp/byte WARN (F7, F4, F15 doctor arm).
7. **Owner bundle 0021** (DDL revoke) — authored, deploy-plan-terminal, excluded from every
   `owner-ddl apply` route (`LatestOwnerBundleVersion` stays 20). **The F16b PRODUCTION-phase test +
   the forced-self-heal pgtest land here (M4)**, tested in two-role pgtest (F6, F12, F16b);
   activation is the operator choreography (§4.3).

Each phase additive and reversible. Self-record before enforce; detection before mutation-relocation;
the verb + finalizer + full-transcript verifier + ownership reconcile + immutable-transcript resume +
the non-revoke filter + the M3 activation guard + **the M5-correct owner-watermark gate (fresh-DB
serve preserved) + the M6/M7-derived boot-path table (every complete-row cell derived from W and A)**
all proven before the boot path leans on them.

---

## 7. Boundary & explicit deferrals (carry-forward)

- **In P4:** the deployer verb; the deploy plan + `deploy_cursor` (incl. `finalizing`) + the immutable
  `deploy_plan` transcript (BC-N1) + per-step + terminal receipt; the idempotent finalizer (C1) with
  the `VerifyStoredTranscript` pre-finalizer gate (M1); the C3 ownership reconcile with 0021 sequenced
  last; the single non-revoke `owner-ddl apply` filter + embed/listing split (M2); lifting
  `ApplyMigrations` out of serve-boot (flagged); the activation interlock + the universal pre-revoke
  cursor edge (BC-N2) + the M3 complete-cursor config gate + no-revoke comparison + forward-watermark
  rule + **the M5-correct owner-watermark dimension (fresh `applied_owner == 0` serves; `1..19`
  halts) + the M6-coherent no-revoke `complete` rows (13/15) + the M7-derived revoke-embedding
  decoupled `complete` cell (16) — the entire complete-row class conditional on A3 fingerprint-sync,
  derived from W and A**; serving-role create-DDL revocation (bundle 0021, deploy-terminal).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`, `rehearsal_receipt.v1`, fidelity
  tiering, full-data clone, expand/contract reshape, lock-budget guardrail. Full capability revocation
  (owner re-owns runtime tables) is a named follow-up beyond P4 (§4.1).
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the single writer, no hosted
  services / replicas / external persistence.

---

## 8. Open tensions handed to the falsifiers (attack these hardest)

- **M7 / F18 / §3.5 — the row-16 derivation and the row-15/row-16 decoupled identity.** Is A's
  decoupled complete branch (§3.3a step 3, `decoupledEnabled == true`) genuinely `revokeEmbedded`-
  independent — does step 0 (the ONLY `revokeEmbedded`-reading predicate) ever fire when decoupled is
  ON? (Source: §3.3a step 0 `revokeEmbedded && !decoupledEnabled`; step 3 decoupled branch reads only
  `plan_hash` + `LiveFingerprint`/`ExpectedFingerprint` = `schema_state`, `schema_drift.go:145-161`.)
  Given that, are row 16's `==0`/`==20`/`>=21` cells truly identical to row 15's decoupled cells
  ("serve verify-only if in-sync, else `awaiting_deploy`")? Is the in-sync row-16 cell genuinely
  constructible (the `schema_state`/`owner_bundle_meta` tables orthogonal AND the binary embedding the
  revoke) and does F18 construct it for `==0`, `==20`, AND `>=21`? Does F18's spy list stay the SAME
  four cells (row 16 reaches `:399` in NO column)? Does the row-16/`>=21` in-sync serve correctly
  represent the activated steady state (0021 applied → fingerprint matches → serve verify-only)?
- **M7 — does the derivation re-introduce a safety hole or a carry-forward regression?** Does serving
  the in-sync row-16 cell ever mutate schema or self-record (no — the decoupled verify path never
  calls `:353`/`:399`; Invariant B holds)? Does making row 16 conditional re-collapse row 1 (no — row
  1 is `cursorState=none`), regress rows 13/15 (no — those are unchanged), weaken the M3 gate (no —
  cells 2/6/10/14 still halt at A0; row 16 is the decoupled cell where A0 does not fire), regress the
  BC-N2 `==20` edge (no — rows 5/7/9/11 unchanged), or advance `Required` (no — `owner.go:35` stays
  20)? Is the choice of Option 1 over Option 2 (no new serve-boot DB-stamp guard) sound — does the
  in-sync row-16 cell serve ONLY because the recorded fingerprint genuinely == `ExpectedFingerprint()`
  (so the schema IS in the expected state, regardless of the owner watermark)?
- **M6 / F18 / §3.5 — the `==0`/`==20` column identity (carry-forward).** Are rows 13/15 `==0` still
  identical to `==20`? Is the degenerate 13/`==0`-in-sync sub-case still in BOTH §4.5 and the F18 spy
  list? Does the cross-row `==0` audit leave ANY row where `==0` ≠ `==20`?
- **M5 / F18a — the `applied_owner` split (carry-forward).** Is the `applied_owner == 0` cell a SERVE
  under `CheckOwnerBundleWatermark` (`owner.go:145` before the `:148-150` shortfall)? Do the four
  buckets `{0, 1..19, ==20, >=21}` cover all of ℕ?
- **M3 / F17 / F18 — the hoisted config gate vs. the no-revoke comparison.** Is `revokeEmbedded &&
  !decoupledEnabled → awaiting_deploy_config` truly the FIRST A predicate? Is there any §3.5 cell
  where the legacy `:399` writer fires over a transcript carrying a pending change, or for a
  revoke-embedding binary?
- **M4 / F16a — the synthetic seam (carry-forward).** Does the synthetic-list / injectable-FS hook
  exercise `OwnerDDLApplyBundles`/`isNonRevokeBundle`/both apply loops without a real 0021 file?
- **M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3 carry-forward.** Re-confirm none regressed under the M7
  derivation.

---

## 9. Anchor table (re-verified line-by-line against the v9 worktree HEAD `83fe6465`)

> Every anchor below was re-read this cycle against the worktree at HEAD `83fe6465` (branch
> `striatum/rfc-0142-p4-design-v9`). `git diff --stat 3f9d5734 83fe6465 -- go/pkg/db/` is EMPTY, so
> the go/pkg/db source is byte-identical with the v8 anchors — the M7 fix is a SPEC change to the
> §3.5 row-16 cell + the F18 parametric extension, not a source change. The M6/M7 orthogonality sites
> and the M5 owner-watermark sites are spot-confirmed inline.

| Target | Anchor (re-confirmed `83fe6465`) |
| --- | --- |
| **the M6/M7 orthogonality anchors (A is owner-watermark- AND, on the decoupled branch, `revokeEmbedded`-independent)** | `schema_drift.go:145-161` `LiveFingerprint` — reads `(to_regclass('striatumd.schema_state') IS NOT NULL)::text` then `SELECT COALESCE(fingerprint,'') FROM striatumd.schema_state WHERE id=$1`; a SEPARATE table from `owner_bundle_meta`, NO `applied_owner` input. `schema_drift.go:171-195` `RecordSchemaFingerprint` — UPSERTs the `striatumd.schema_state` singleton with `ExpectedFingerprint()`; NO read of `owner_bundle_meta`. `schema_drift.go:83-100` `ExpectedFingerprint` (hashes EMBEDDED FILE BYTES — pure, independent of `applied_owner`/`revokeEmbedded`). A's decoupled complete branch (§3.3a step 3) reuses these reads only; the ONLY `revokeEmbedded`-reading predicate (step 0) does not fire when decoupled is ON. |
| **the M5 owner-watermark gate W (the `applied_owner` dimension)** | `owner.go:124-154` `CheckOwnerBundleWatermark`; **`:145` `if applied == 0 { return nil }` (the fresh/no-authority SERVE exception, returns BEFORE the shortfall)**; **`:148-150` `if applied < RequiredOwnerBundleVersion { return &AwaitingOwnerDDLError{...} }` (the `1..19` shortfall halt)**; `:151-153` tolerate-forward / barrier (b); the comment block `:116-123` + `:140-143`. |
| **the fresh-DB representation (M5)** | `OwnerBundleVersion` `owner.go:226-247`; **`:234-235` `if present != "true" { return 0, nil }` (returns 0 when `to_regclass('striatumd.owner_bundle_meta')` is null)**; `owner_pg_test.go:19` (a fresh migrated DB starts at version 0). |
| **serve-boot mutation site + the W→A→`:353` boot order** | `connection.go:332` `ConnectAndMigrate`; **watermark W `:349`**; **`ApplyMigrations` `:353`**; drift gate `:376-383` (refuse-mode halt); **shadow log + fall-through `:384-393`**; **`RecordSchemaFingerprint` `:399` ("This is the only writer of schema_state")** — re-read this cycle; W returns nil for `applied == 0`/`==20`/`>=21`-revoke and halts for `1..19`, then A precedes `:353`. |
| boot fork / `--migrate` / typed-halt mapping | `authority_bootstrap.go:181,193`; `main.go:76`; typed halts `:208-227` (add `awaiting_deploy`/`awaiting_deploy_config`/`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` arms; `awaiting_owner_ddl` already mapped) |
| deployer command site | `daemon.go:67-81` dispatch (add `deploy`); `runDaemonOwnerDDL` `:90` (admin DSN `:115`); `runDaemonMigrate` `:167`; `EnvDaemonAdminDBURL` `:24` |
| runtime apply engine to reuse/wrap (C3) + M1 oracles | `migrations.go:121` `ApplyMigrations`; `applyOne` `:304-355`; `verifyRecordedHashTx` `:274` (reads `schema_migrations.sha256` `:275` — M1 runtime DB-stamp oracle); `MigrationSHASet` `:197` (M1 runtime byte oracle); `//go:embed sql/*.sql` `:21`; lock `:18` |
| owner-bundle engine / watermark + forward-tolerance (C2) / 0021 exclusion (M2/C3) | `owner.go:265-303` `ApplyOwnerBundles` (slice `:269` → `OwnerDDLApplyBundles()`); `:309-322` `applyPendingOwnerBundles` (`<= current` `:312`, in-loop guard); `:332-351` `ReapplyAllOwnerBundles` (nil-fallback `:336-342`, in-loop guard); `:367-374` `isCrossBundleDependencyError`; `:511-541` `applyOneOwnerBundle` (`owner_bundle_meta.sha256` stamp `:529` — M1 owner DB-stamp oracle); `OwnerBundles()` `:197-224` (FULL loader); `OwnerBundle.SHA256()` `:191` (M1 owner byte oracle); `LatestOwnerBundleVersion = 20` `:23` (**stays 20**); `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` `:35` (= 20, **stays 20**); `revokeEmbedded` = 0021 in `ownerBundleFS` (`//go:embed` `:156`) |
| pure read consuming the FULL owner-bundle loader (must keep seeing 0021) | `owner_runtime_ownership.go:51-52` `RuntimeOwnedTablesAlterable()` → `OwnerBundles()` |
| current owner-bundle frontier (NOT the revoke) | `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql` (the highest non-revoke bundle; the DDL-revoke is the NEW 0021) |
| C3 prerequisite the revoke-last fix relies on | `0018_runtime_table_ownership_transfer.sql:98` (`GRANT CREATE … TO striatumd_rw` before the transfer loop) — 0018 in the non-revoke prefix, CREATE held when runtime reconciles run |
| fingerprint contract (M1 mechanism + the M3 A3 pure-read comparison + the M6/M7 orthogonality) | `schema_drift.go:83-100` `ExpectedFingerprint` (hashes EMBEDDED FILE BYTES — pure); `:105-137` `composeFingerprint` (reused for `plan_hash`); `:145-161` `LiveFingerprint` (reads the self-recorded `schema_state` singleton — NO recompute, NO `owner_bundle_meta`; pure); `:171-195` `RecordSchemaFingerprint` (the gated write, `schema_state` only); `:254-274` `CheckSchemaDrift`; `EnvSchemaDriftRefuse` `:28` |
| `deploy_cursor` + `deploy_plan` table model | `0043_schema_state.sql:39-44` (singleton CHECK), `:46-52` (`striatumd_rw` GRANT `DO` block) — model for both new 0044 tables |
| per-step / terminal receipt SD fn | `0001_authority_phase0.sql:152` (`append_audit_row`), `:238` (`GRANT EXECUTE … TO striatumd_rw`) |
| doctor block model | `go/pkg/reads/doctor_schema_drift.go` (`schemaDriftDoctorBlock`; `ExpectedFingerprint` call) |
| FMA-007 self-heal test to extend (M2 / F16b) | `owner_pg_test.go:219` `TestReapplyAllOwnerBundlesIsOrderedAndIdempotent` (calls `ReapplyAllOwnerBundles(ctx, runner, nil, "test")`) |
| two-role fixture for F12 / F16b / F17 / F18 / F18a | `go/pkg/pgtest/two_role.go:130` `ApplyOwnerBundles` (non-superuser two-role bootstrap) |

<sub>Holder artifact (v9 revision) for the RFC 0142 P4 falsification-gate design run. Resolves cycle-1
(design-v8) finding **M7** (the M6 mechanical derivation closed the no-revoke `complete` rows 13/15
but left the SIBLING `complete`/decoupled/revoke-embedding cell — row 16 — asserted unconditionally,
violating the holder's own derivation rule and making F18 a false oracle for the constructible
in-sync row-16 cell) via **Option 1, the clean fix parallel to the M6 fix**: §0.2 adds the
SUB-INVARIANT that A's decoupled complete branch reads neither `applied_owner` NOR `revokeEmbedded`
(`schema_drift.go:145-161`/`:171-195` confirm the orthogonality; §3.3a step 0 is the only
`revokeEmbedded`-reading predicate and does not fire when decoupled is ON), so rows 15 and 16 take the
IDENTICAL A3-conditional outcome; §3.5 row 16 in the `==0`, `==20`, AND `>=21` columns becomes
conditional "SERVE-verify if in-sync, else `awaiting_deploy`" identical to row 15; F18 is made
PARAMETRIC over the seven A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`,
16/`==0`, 16/`==20`, 16/`>=21`} with the in-sync/out-of-sync sub-dimension; the four `:399`-reaching
cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} are UNCHANGED (row 16 is decoupled →
`ConnectAndVerify` never self-records), so §4.5 and the F18 spy list still agree; the normal pre-0021
row-16 state is documented OUT-OF-SYNC and the normal post-0021 row-16/`>=21` steady state IN-SYNC.
Carries **M6(rows 13/15) + M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3 forward
verbatim and unregressed** (re-verified line-by-line against the v9 worktree HEAD `83fe6465`; `git
diff --stat 3f9d5734 83fe6465 -- go/pkg/db/` empty). The cycle-1 falsifiers (decoupling-boundary/
decision-table lens; carry-forward/regression lens) re-attack §3.5 row 16 (the derived conditional +
the row-15/row-16 identity), §4.5 Invariant B (the unchanged 4-cell enumeration), F18 (the parametric
complete-row extension), F18a, and the tensions in §8; the adjudicator's collaboration ledger — not
falsifier completion — decides whether the gate clears.</sub>



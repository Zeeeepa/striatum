You are the **Holder** for the RFC 0142 P4 design run, and **THIS IS THE EIGHTH
REVISION (v8).** Seven prior design runs ran this same falsification gate. v1
(`rfc-0142-p4-design`) returned `needs_revision` with three findings C1/C2/C3. v2
(`rfc-0142-p4-design-v2`) **resolved C1 and C2** (C3 still open + new finding N1).
v3 (`rfc-0142-p4-design-v3`) **resolved C3 (ownership transfer, revoke-last)** and
closed the immediate N1 hole — both falsifiers conceded C3 — but returned
`needs_revision` on two grounds BC-N1 + BC-N2. v4 (`rfc-0142-p4-design-v4`)
**resolved BOTH BC-N1 and BC-N2** but returned `needs_revision` on two NEW material
challenges M1 + M2. v5 (`rfc-0142-p4-design-v5`) **resolved BOTH M1 and M2** but
returned `needs_revision` on two new findings M3 + M4. v6 (`rfc-0142-p4-design-v6`)
**resolved BOTH M3 and M4** — both v6 falsifiers AND the v6 adjudicator explicitly
conceded each, and M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward intact — but returned
`needs_revision` on M5. v7 (`rfc-0142-p4-design-v7`) **resolved M5 row-1** — the
`{0/no authority, 1..19 shortfall, ==20, >=21}` split; cell 1/`==0` now serves the
fresh-DB bring-up; F18/F18a assert both cells; cell `==20` relabeled inert-landing —
both v7 falsifiers AND the v7 adjudicator explicitly conceded the row-1 repair, and
M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward intact — but returned
`needs_revision` **again** — the gate's single allowed cycle — on one new finding,
source-verified against the run worktree and landed INDEPENDENTLY by BOTH v7 falsifiers:

- **M6 (LOAD-BEARING)** — the M5 `applied_owner` split is NOT propagated coherently
  through the `complete` rows. §3.5 rows 13 and 15 give the `==0` column a different
  outcome than `==20`, but the holder's own §3.3a A predicate (`CheckDeployActivation`)
  does NOT read `applied_owner` — it decides solely on `plan_hash == expected` +
  `LiveFingerprint == ExpectedFingerprint` (`schema_drift.go:145-161` reads the recorded
  `schema_state.fingerprint` singleton — orthogonal to `owner_bundle_meta`/`applied_owner`;
  `schema_drift.go:171-195` writes the binary's `ExpectedFingerprint()` — also orthogonal).
  So for the same in-sync facts A returns the same outcome regardless of the bucket — the
  table's differing `==0`/`==20` outcomes in rows 13/15 cannot be produced by the specified
  predicate. The executable F18 matrix is a FALSE ORACLE for the in-sync
  `complete`/`applied_owner==0` cells, OR the build must smuggle an unstated
  `applied_owner`-dependent complete-cursor guard that contradicts the stated "identical A
  behavior" claim. Worse, §4.5 ADMITS the degenerate 13/`==0`-in-sync idempotent `:399`
  rewrite while the F18 spy list omits that cell — §4.5 and the F18 oracle disagree.

**Start from the v7 `HOLDER.md`** — it is a **required context doc**
(`docs/operator/artifacts/rfc-0142-p4-design-v7/dialogue/holder/HOLDER.md`). Your job
is to REVISE that spec, not write a new one from scratch. The full M6 analysis and
the exact prescribed fix are in the **v7 collaboration ledger** (also a required
context doc:
`docs/operator/artifacts/rfc-0142-p4-design-v7/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— read its `findings:` block for M6, and **§3 + §4 "What the revision must fix" +
§5 "What already cleared"**, in full). `SEED.md` pins the single binding constraint
(M6), keeps the proactive-completeness boot-path decision table requirement (now with
the FULL 64-cell table derived MECHANICALLY from W and A), and the section "Carried
forward — resolved by v7 (do NOT reopen)" (M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 +
BC-N2 + C1 + C2 + C3).

Your revised spec **MUST resolve the cycle-1 finding (M6) per its prescribed fix**,
**keep the proactive-completeness boot-path decision table (now with the full 64-cell
table derived mechanically from W and A)**, and **MUST carry forward M5(row-1), M3,
M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 unregressed**. A revision that leaves M6
open — or that merely *claims* a fix without the concrete mechanical derivation and
the §4.5/F18 consistency fix — or that regresses a carry-forward finding — or whose
decision table is not derived from W and A — has NOT cleared the gate. This is the
gate's single allowed revision cycle, so the cycle-1 falsifiers re-attack each
finding specifically and a second `needs_revision` ends the gate unCleared.

Read the required context docs in full first — `SEED.md`, the v7 `HOLDER.md`, and the
v7 collaboration ledger — plus the committed RFC
(`docs/rfcs/0142-safe-by-construction-database-change-deployment.md`, status
`accepted`, D258). Build on the exact anchors the v7 spec and the SEED anchor table
use; **re-verify them against current `main`** — in particular the M6 source anchors:
`go/pkg/db/schema_drift.go:145-161` (`LiveFingerprint` reads the recorded
`schema_state.fingerprint` singleton — orthogonal to `owner_bundle_meta`/`applied_owner`)
and `go/pkg/db/schema_drift.go:171-195` (`RecordSchemaFingerprint` writes
`ExpectedFingerprint()` — also orthogonal to `owner_bundle_meta`/`applied_owner`). Also
re-verify the M5 anchors (still load-bearing): `go/pkg/db/owner.go:145`
(`if applied == 0 { return nil }`, BEFORE the `if applied < RequiredOwnerBundleVersion`
shortfall at `:148-150`), `owner.go:233-235` (`OwnerBundleVersion` returns 0 for absent
`owner_bundle_meta`), `owner_pg_test.go:19` (fresh DB version 0), `owner.go:23/:35`
(`LatestOwnerBundleVersion = 20` / `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`).
Also re-verify the `ConnectAndMigrate` boot order `go/pkg/db/connection.go:349`
(`CheckOwnerBundleWatermark`) → `:353` (`ApplyMigrations`) → `:376-383`
(`CheckSchemaDrift`) → `:384-399` (shadow fall-through + self-record) → `:399`
(`RecordSchemaFingerprint`). The DDL-revoke bundle stays at the renumbered **0021**
ordinal (0020 is `0020_owner_bundle_watermark_read.sql`,
`LatestOwnerBundleVersion == 20`).

Publish the **revised (v8)** falsifiable implementation spec for RFC 0142 **P4 — the
one-shot deployer** as your `HOLDER.md` artifact. Make it concrete and falsifiable,
not a restatement of the RFC. Open with an auditable resolution map (an "Addressing
the design-v7 findings" subsection) so the falsifiers can verify M6 is resolved and
M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are preserved, rather than infer it.

Hold the root reframe: **schema mutation must stop being an implicit side effect of
the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving daemon
can hold zero DDL privilege and a bad migration can never wedge the single writer on
boot. M6 is the decision-table coherence edge of that reframe: the entire 64-cell
table must be derived mechanically from W and A, so a build cannot exploit a
predicate-table mismatch to smuggle in unstated guards.

Your spec MUST:

0. **Resolve the single binding revision constraint — the gating requirement.**

   - **M6 (propagate the M5 split through the `complete` rows; derive the table
     mechanically from W and A — decision-table executability).** **Fix (binding,
     contract (1) — the expected fix):**
     - **State the load-bearing mechanical-derivation invariant explicitly:** "once W
       passes (`applied_owner ∈ {0, ==20, >=21-as-barrier}`), A is
       owner-watermark-independent — `CheckDeployActivation` does NOT read
       `applied_owner` — so the `==0` and `==20` columns have IDENTICAL A-gate outcomes
       in EVERY cursor row (none, in_progress, finalizing, complete), not only row 1."
       Anchor this to `schema_drift.go:145-161` and `schema_drift.go:171-195`
       (confirming `LiveFingerprint` and `RecordSchemaFingerprint` are both orthogonal
       to `owner_bundle_meta`/`applied_owner`).
     - **Derive the full 64-cell table FROM this invariant.** For any fixed `(cursorState,
       decoupledEnabled, revokeEmbedded)` row, the `==0` and `==20` column outcomes must
       match everywhere W passes (the `1..19` column still halts `awaiting_owner_ddl` at
       W; the `>=21` column fires the forward-watermark barrier — those are unchanged).
     - **In §3.5, make rows 13 and 15 in the `==0` column conditional** — "**serve if
       in-sync, else `awaiting_deploy`**" — exactly as the `==20` column. Row 13 (`complete`,
       flag off, no-revoke): `==0` in-sync → SERVE-legacy (idempotent `:399` rewrite
       legitimate, no pending change); `==0` out-of-sync → `awaiting_deploy`. Row 15
       (`complete`, decoupled on, no-revoke): `==0` in-sync → SERVE-verify; `==0`
       out-of-sync → `awaiting_deploy`.
     - **Add the degenerate 13/`==0`-in-sync idempotent `:399` rewrite** to the §4.5
       Invariant-B per-cell enumeration AND to the F18 spy list. The spy list must equal
       exactly the cells where the table says a legacy/idempotent `:399` write occurs:
       1/`==0` (fresh-DB serve), 1/`==20` (inert-landing), 13-in-sync/`==0` (degenerate
       in-sync), and 13-in-sync/`==20`. Both §4.5 and F18 must enumerate all four.
     - **Audit ALL cursor rows (none/in_progress/finalizing/complete) × the `==0` column**
       explicitly in the spec, confirming each matches the `==20` column for the same
       reason (A does not read `applied_owner`). The v7 failure was that the row-1 fix was
       not propagated to the complete rows; the v8 revision must close the class fully.
     - (The v7 ledger also offers contract (2): classify `complete + applied_owner == 0`
       as inconsistent and halt a typed error before serving. Do NOT use (2) — (1) is the
       expected, source-preserving fix.)

   - **Preserve the asymmetry (the v7 ledger's §4 note).** The M6 fix is local to the
     `complete` rows and the mechanical derivation; it must NOT re-collapse the resolved
     row-1 fresh-DB serve, must NOT weaken the M3 config gate (cells 2/6/10/14 must still
     halt `awaiting_deploy_config` at A0 in every column that passes W incl. `0`), must NOT
     regress the BC-N2 `applied_owner == 20` edge, and must NOT advance
     `RequiredOwnerBundleVersion`.

   - **Proactive completeness — keep the boot-path decision table; close the class (do
     this ONCE, exhaustively, with the FULL mechanical derivation, to preempt the next
     cycle).** For EVERY combination of `cursorState` in {none, in_progress, finalizing,
     complete} (treat `step_committed` / `aborted` per the §1.3 disambiguation) ×
     `decoupledEnabled` in {on, off} × `revokeEmbedded` in {yes, no} × `applied_owner` in
     **{0/no authority, 1..19 authority shortfall, ==20, >=21}**, derive the **exact guard /
     outcome** FROM the two predicates W and A — not ad hoc. **PROVE §4.5 Universal
     Invariant B holds in EVERY cell** — AND prove the legitimate fresh-DB / inert-landing
     cells (`applied_owner == 0`, no-revoke, no transcript) **STILL SERVE and are NOT
     wedged** — explicitly keeping the M3 cell (`complete` + decoupled OFF +
     revoke-embedding + a pending change) and the shadow-mode drift-gate fall-through
     (`connection.go:384-399`) covered unchanged. Make the table an **executable, named
     requirement** the falsifiers can verify against the `schema_drift.go:145-161` and
     `schema_drift.go:171-195` orthogonality anchors. This closes M6 and preempts any
     further unguarded-combination challenge (an M7).

   Explicitly call out, in the revised spec, **how** M6 is now closed (the mechanical
   derivation, the rows 13/15 `==0` fix, the §4.5/F18 consistency), and **confirm**
   M5(row-1) (the `{0/no authority, 1..19, ==20, >=21}` split; cell 1/`==0` serves the
   fresh-DB bring-up; F18/F18a assert both cells; cell `==20` relabeled inert-landing),
   M3 (the hoisted step-0 `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config`
   config gate + the no-revoke `complete` pre-`ApplyMigrations` pure-read comparison +
   tightened Invariant B + F17/F11(g)/F18), M4 (the F16a synthetic-list / F16b production
   phase split + the forced FMA-007 self-heal pgtest in F16b), M1 (the full-transcript
   `VerifyStoredTranscript` byte + DB-stamp verifier on resume AND as finalizer step 0, F15
   + extended F14), M2 (the single non-revoke filter `OwnerDDLApplyBundles()`/`isNonRevokeBundle`
   across every `owner-ddl apply` route incl. the FMA-007 self-heal + the embed/listing
   split, F16 + F12/`G-revoke-last`), BC-N1 (the immutable `deploy_plan` transcript
   materialized before step 0, resume off the stored transcript, §1.3 + per-step doctor,
   F14), BC-N2 (the universal `revokeEmbedded`-independent `CheckDeployActivation` edge
   halting `awaiting_deploy` at `applied_owner == 20`, F11(e)/(f) + extended
   `G-old-binary-refuse`), C1 (the `finalizing` state + idempotent finalizer + §1.3 row +
   F10), C2 (`CheckDeployActivation` before `ApplyMigrations`, typed halts,
   forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion` kept at 20), and C3
   (the DDL-revoke bundle 0021 special-cased + excluded from `owner-ddl apply` + applied
   terminal, F12/`G-revoke-last`) are carried forward **verbatim from the v7 HOLDER** and
   not regressed. Keep the §1.3 table, the `finalizing` finalizer, the stored-transcript
   receipt key, the M1 verifier, the M3 complete-cursor guard, the BC-N2 non-complete edge,
   and the now-M6-fixed boot-path decision table (derived mechanically from W+A) all
   coherent together.

1. **Keep Q3 and Q4 resolved.** Q3 — the per-step-atomic + resumable-cursor contract
   (now including M1's full-transcript verification, M3's complete-cursor activation
   guard, M5's owner-watermark dimension split that keeps the fresh-DB cells serving,
   AND M6's mechanical derivation that makes the decision table coherent across ALL
   cursor rows) is sufficient for every owner+runtime interleaving and boot-path
   combination P4 ships. Q4 — plain verb now with the three run-shape seams. Carry
   both forward; do not re-litigate.

2. **Keep the deployer surface and the serve-boot decoupling intact** (carry forward
   from v7): the `striatum daemon deploy` command site
   (`go/pkg/cli/localcommands/daemon.go`); the embed-FS-derived deploy plan with the
   immutable stored transcript (BC-N1) and the M1 full-transcript verification; the
   `deploy_plan`/`deploy_cursor` runtime migration (≥ 0044); the hash-chained deploy
   receipt into the owner-held `audit_log`; the lift of `ApplyMigrations` out of
   `go/pkg/db/connection.go` `ConnectAndMigrate` / `ConnectAndVerify` with the P2
   watermark interlock, the P3 drift gate, the BC-N2 universal non-complete cursor edge,
   the M3 complete-cursor guard, the M5-correct owner-watermark dimension (cell 1/`==0`
   still serves), and now the M6-coherent full decision table (derived mechanically from W
   and A) intact.

3. **Keep the serving-role DDL revocation (the 0021 owner bundle)** — special-cased and
   sequenced terminal per C3, excluded from EVERY `owner-ddl apply` route including the
   FMA-007 self-heal (M2), and never reachable via the legacy serve-boot path on a
   `complete` cursor (M3). State exactly how it ships without lockout in any boot-path
   cell, with the embed/listing helper split.

4. **State each load-bearing claim as a falsifiable assertion + its named test /
   game-day step.** Carry F1–F18a + `G-revoke-last` + `G-old-binary-refuse` forward
   (re-confirm and re-anchor), and ensure F18 (`T-deploy-bootpath-decision-table`) is
   present and sharp with the MECHANICALLY DERIVED full 64-cell table: the F18 spy list
   matches the §4.5 Invariant-B proof (4 cells reach the legacy writer: 1/`==0`,
   1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`). Ensure F18a pins the fresh-DB serve
   (cell 1/`==0`) and the `1..19` halt.

5. **Stay inside the product boundary and the accepted design.** Local-first,
   single-host, ONE Postgres, ONE daemon as the single writer. Do NOT pull in P5
   (rehearsal receipt / expand-contract / fidelity tiering / clone = Q1/Q2).
   Shadow-first for the new path: a no-revoke inert binary on a clean DB still serves
   (the `applied_owner == 0` cell); a revoke-embedding binary with the flag OFF over a
   deploy transcript halts, never auto-applies (the M3 gate). Additive migrations only,
   self-record before enforce.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.

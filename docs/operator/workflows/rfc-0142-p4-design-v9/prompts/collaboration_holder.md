You are the **Holder** for the RFC 0142 P4 design run, and **THIS IS THE NINTH
REVISION (v9).** Eight prior design runs ran this same falsification gate. v1
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
`needs_revision` **again** on M6. v8 (`rfc-0142-p4-design-v8`) **resolved M6** —
the M5 `applied_owner` split propagated through the no-revoke `complete` rows (13/15);
§0.2 states the W→A-independence invariant; the degenerate 13/`==0`-in-sync idempotent
`:399` rewrite added to BOTH §4.5 AND the F18 spy list; the four `:399`-reaching cells
{1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} enumerated identically —
BOTH v8 falsifiers AND the v8 adjudicator explicitly conceded the rows-13/15 repair,
and M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward intact — but returned
`needs_revision` **again** — the gate's single allowed cycle — on one new finding,
source-verified against the run worktree and landed INDEPENDENTLY by BOTH v8 falsifiers:

- **M7 (LOAD-BEARING)** — §3.5 row 16 (`cursorState=complete`, `decoupledEnabled=true`,
  `revokeEmbedded=true`) gives the `==0` and `==20` columns UNCONDITIONAL
  `awaiting_deploy`, reasoned '0021 not yet applied → fingerprint ≠ → not in-sync'. But
  A's §3.3a step-3 decoupled branch decides solely on `cursor.plan_hash == expected` +
  `LiveFingerprint == ExpectedFingerprint` — with NO `applied_owner` input — and the
  holder's OWN derivation rule (HOLDER.md:565-566) says 'where A's outcome is conditional
  on the fingerprint-sync state … the cell is written conditionally'. Row 16's
  complete/decoupled outcome IS conditional on fingerprint-sync, yet the cell is written
  UNCONDITIONALLY — the holder violated its own rule. The in-sync row-16 cell is
  constructible (exactly as the holder constructs the degenerate 13/`==0`-in-sync cell)
  and A serves verify-only there while §3.5 says halt; F18 is therefore a FALSE ORACLE
  for the in-sync row-16 `==0`/`==20` cells. NOT a safety hole (row 16 is decoupled →
  never reaches the legacy `:399` writer; Invariant B holds) but MATERIAL (the 64-cell
  table is not fully derived from W and A).

**Start from the v8 `HOLDER.md`** — it is a **required context doc**
(`docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/holder/HOLDER.md`). Your job
is to REVISE that spec, not write a new one from scratch. The full M7 analysis and
the exact prescribed fix are in the **v8 collaboration ledger** (also a required
context doc:
`docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— read its `findings:` block for M7 (the `challenge:` field and `status: open`) and
the `rationale:` field (the "Required fix" paragraph specifying Option 1 vs Option 2),
plus the `§5 findings status: answered` entries for the carry-forward set. `SEED.md`
pins the single binding constraint (M7), keeps the proactive-completeness boot-path
decision table requirement (now with ALL complete-row cells derived from A's
fingerprint-sync predicate, including row 16 and `>=21` variants), and the section
"Carried forward — resolved by v8 (do NOT reopen)"
(M6 + M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3).

Your revised spec **MUST resolve the cycle-1 finding (M7) per its prescribed fix**,
**keep the proactive-completeness boot-path decision table (now with ALL complete-row
cells derived from A's fingerprint-sync predicate, including row 16 and `>=21`
variants, and F18 parametric over all of them)**, and **MUST carry forward M6,
M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 unregressed**. A revision
that leaves M7 open — or that merely *claims* a fix without the concrete propagation
through §1.3/§3.3a/§3.5/§4.5/F18 and the F18 parametric extension — or that regresses
a carry-forward finding — or that leaves the `>=21` revoke-embedding complete-row cell
still unconditional — has NOT cleared the gate. This is the gate's single allowed
revision cycle, so the cycle-1 falsifiers re-attack each finding specifically and a
second `needs_revision` ends the gate unCleared.

Read the required context docs in full first — `SEED.md`, the v8 `HOLDER.md`, and the
v8 collaboration ledger — plus the committed RFC
(`docs/rfcs/0142-safe-by-construction-database-change-deployment.md`, status
`accepted`, D258). Build on the exact anchors the v8 spec and the SEED anchor table
use; **re-verify them against current `main`** — in particular the M7 source anchors:
`go/pkg/db/schema_drift.go:145-161` (`LiveFingerprint` reads the recorded
`schema_state.fingerprint` singleton — orthogonal to `owner_bundle_meta`/`applied_owner`)
and `go/pkg/db/schema_drift.go:171-195` (`RecordSchemaFingerprint` writes
`ExpectedFingerprint()` — also orthogonal to `owner_bundle_meta`/`applied_owner`). Also
re-verify the M6 source anchors (still load-bearing): the §0.2 W→A-independence
invariant, `schema_drift.go:145-161`/`:171-195`, and the rows-13/15 conditional cells.
Also re-verify `go/pkg/db/owner.go:145` (`if applied == 0 { return nil }`), `owner.go:23/:35`
(`LatestOwnerBundleVersion = 20` / `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`),
and the `ConnectAndMigrate` boot order `go/pkg/db/connection.go:349` →`:353` →`:376-399`.
The DDL-revoke bundle stays at the renumbered **0021** ordinal (0020 is
`0020_owner_bundle_watermark_read.sql`, `LatestOwnerBundleVersion == 20`).

Publish the **revised (v9)** falsifiable implementation spec for RFC 0142 **P4 — the
one-shot deployer** as your `HOLDER.md` artifact. Make it concrete and falsifiable,
not a restatement of the RFC. Open with an auditable resolution map (an "Addressing
the design-v8 findings" subsection) so the falsifiers can verify M7 is resolved and
M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are preserved, rather than infer it.

Hold the root reframe: **schema mutation must stop being an implicit side effect of
the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving daemon
can hold zero DDL privilege and a bad migration can never wedge the single writer on
boot. M7 is the final complete-row coherence edge of that reframe: the entire
complete-row class (13/15/16 and `>=21` variants) must be derived from A's
fingerprint-sync predicate, so a build cannot exploit a predicate-table mismatch to
smuggle in unstated owner-watermark guards.

Your spec MUST:

0. **Resolve the single binding revision constraint — the gating requirement.**

   - **M7 (make row 16 and its `>=21` variant conditional on A's fingerprint-sync
     predicate; propagate through §1.3/§3.3a/§3.5/§4.5/F18; make F18 parametric —
     decision-table executability / complete-row class closure). Fix (binding,
     Option 1 — the clean fix parallel to the M6 fix):**
     - **MAKE §3.5 row 16 `==0`/`==20` CONDITIONAL** on the same A3 complete/decoupled
       fingerprint predicate — "**SERVE-verify if in-sync, else `awaiting_deploy`**" —
       exactly as A's §3.3a step-3 decoupled branch decides and exactly as the M6 fix
       applied to rows 13/15. Document that the normal reachable pre-0021 state is
       OUT-OF-SYNC: the `awaiting_deploy` outcome is the dominant real-world case; the
       in-sync subcase is the degenerate corner A must not mishandle.
     - **Write the `>=21` revoke-embedding complete-row cell CONDITIONAL too.** For full
       derivation: `cursorState=complete`, `decoupledEnabled=true`, `revokeEmbedded=true`,
       `applied_owner >= 21` — A's §3.3a decoupled branch still decides on fingerprint-sync
       alone; the `>=21` column must also be conditional on in-sync/out-of-sync.
     - **Propagate through §1.3, §3.3a, §3.5, §4.5, and F18.** Every section referencing
       the `complete`/decoupled/revoke-embedding serve behavior must reflect the conditional.
     - **Make F18 PARAMETRIC over ALL complete-row cells** (13/15/16 and `>=21` variants)
       with the in-sync/out-of-sync sub-dimension. F18 currently only adds the sub-dimension
       for rows 13/15; extending it to row 16 and `>=21` closes the class. Document within
       F18 that the normal pre-0021 state is out-of-sync.
     - (The v8 ledger also offers Option 2: add an explicit consistency guard or a
       complete-boot stored-transcript DB-stamp verification. Do NOT use Option 2 — Option 1
       is the expected, source-preserving fix; it keeps the W→A-independence invariant
       intact and the design coherent.)

   - **Preserve the invariants (the v8 ledger note).** The M7 fix must NOT re-collapse
     the row-1 fresh-DB serve, must NOT weaken the M3 config gate (cells 2/6/10/14 must
     still halt `awaiting_deploy_config` at A0 in every column that passes W), must NOT
     regress the M6 rows-13/15 conditional cells, must NOT regress the BC-N2
     `applied_owner == 20` edge, and must NOT advance `RequiredOwnerBundleVersion`.

   - **Proactive completeness — keep the boot-path decision table; close the
     complete-row class FULLY.** For EVERY combination of `cursorState` in
     {none, in_progress, finalizing, complete} × `decoupledEnabled` in {on, off} ×
     `revokeEmbedded` in {yes, no} × `applied_owner` in {0/no authority, 1..19 authority
     shortfall, ==20, >=21}, derive the **exact guard / outcome** FROM the two predicates
     W and A — not ad hoc. In particular: ALL complete-row cells (rows 13/15/16 and their
     `>=21` variants) must now have the in-sync/out-of-sync sub-dimension from A's
     fingerprint-sync predicate. **PROVE §4.5 Universal Invariant B holds in EVERY cell**
     AND prove the legitimate fresh-DB / inert-landing cells STILL SERVE and are NOT
     wedged. Keep the M3 cell (`complete` + decoupled OFF + revoke-embedding) and the
     shadow-mode drift-gate fall-through covered, unchanged.

   Explicitly call out, in the revised spec, **how** M7 is now closed (the row-16
   conditional, the `>=21` conditional, the §1.3/§3.3a/§3.5/§4.5/F18 propagation, the
   F18 parametric extension), and **confirm** M6 (§0.2 invariant intact; rows 13/15 `==0`
   still conditional; the four `:399`-reaching cells still in §4.5 and F18; cross-row audit
   intact), M5(row-1) (the `{0/no authority, 1..19, ==20, >=21}` split; cell 1/`==0`
   serves; F18/F18a; cell `==20` inert-landing), M3 (the hoisted step-0 config gate +
   no-revoke `complete` pure-read comparison + tightened Invariant B + F17/F11(g)/F18),
   M4 (F16a/F16b phase split + forced FMA-007 self-heal pgtest), M1 (the
   full-transcript `VerifyStoredTranscript` byte + DB-stamp verifier on resume AND as
   finalizer step 0, F15 + extended F14), M2 (the single non-revoke filter across every
   `owner-ddl apply` route + the embed/listing split), BC-N1 (the immutable `deploy_plan`
   transcript materialized before step 0, resume off the stored transcript, §1.3 + doctor,
   F14), BC-N2 (the universal `revokeEmbedded`-independent `CheckDeployActivation` edge
   halting `awaiting_deploy` at `applied_owner == 20`, F11(e)/(f) + extended
   `G-old-binary-refuse`), C1 (the `finalizing` state + idempotent finalizer + §1.3 row +
   F10), C2 (`CheckDeployActivation` before `ApplyMigrations`, typed halts,
   forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion` kept at 20), and C3
   (the DDL-revoke bundle 0021 special-cased + excluded from `owner-ddl apply` + applied
   terminal, F12/`G-revoke-last`) are carried forward **verbatim from the v8 HOLDER** and
   not regressed.

1. **Keep Q3 and Q4 resolved.** Q3 — the per-step-atomic + resumable-cursor contract
   (now including M1's full-transcript verification, M3's complete-cursor activation
   guard, M5's owner-watermark dimension split, M6's mechanical derivation for rows 13/15,
   AND M7's closure of the complete-row class for rows 16 and `>=21`) is sufficient for
   every owner+runtime interleaving and boot-path combination P4 ships. Q4 — plain verb
   now with the three run-shape seams. Carry both forward; do not re-litigate.

2. **Keep the deployer surface and the serve-boot decoupling intact** (carry forward
   from v8): the `striatum daemon deploy` command site; the embed-FS-derived deploy plan
   with the immutable stored transcript (BC-N1) and the M1 full-transcript verification;
   the `deploy_plan`/`deploy_cursor` runtime migration (≥ 0044); the hash-chained deploy
   receipt; the lift of `ApplyMigrations` out of `ConnectAndMigrate` / `ConnectAndVerify`
   with the P2 watermark interlock, the P3 drift gate, the BC-N2 universal non-complete
   cursor edge, the M3 complete-cursor guard, the M5-correct owner-watermark dimension,
   the M6-coherent rows-13/15 conditional cells, and now the M7-closed full complete-row
   class (rows 13/15/16 and `>=21` variants all conditional on fingerprint-sync, derived
   from A, F18 parametric) intact.

3. **Keep the serving-role DDL revocation (the 0021 owner bundle)** — special-cased and
   sequenced terminal per C3, excluded from EVERY `owner-ddl apply` route including the
   FMA-007 self-heal (M2), and never reachable via the legacy serve-boot path on a
   `complete` cursor (M3). State exactly how it ships without lockout in any boot-path
   cell, with the embed/listing helper split.

4. **State each load-bearing claim as a falsifiable assertion + its named test /
   game-day step.** Carry F1–F18a + `G-revoke-last` + `G-old-binary-refuse` forward
   (re-confirm and re-anchor), and ensure F18 (`T-deploy-bootpath-decision-table`) is
   present and sharp with the PARAMETRIC complete-row extension: F18 asserts all
   complete-row cells (13/15/16 and `>=21` variants) with the in-sync/out-of-sync
   sub-dimension, documenting that the normal pre-0021 state is out-of-sync; the F18
   spy list still matches the §4.5 Invariant-B proof (4 cells reach the legacy writer:
   1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`; row 16 is decoupled and
   NEVER reaches `:399`).

5. **Stay inside the product boundary and the accepted design.** Local-first,
   single-host, ONE Postgres, ONE daemon as the single writer. Do NOT pull in P5
   (rehearsal receipt / expand-contract / fidelity tiering / clone = Q1/Q2).
   Shadow-first for the new path: a no-revoke inert binary on a clean DB still serves
   (the `applied_owner == 0` cell); a revoke-embedding binary with the flag OFF over a
   deploy transcript halts, never auto-applies (the M3 gate). Additive migrations only,
   self-record before enforce.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.

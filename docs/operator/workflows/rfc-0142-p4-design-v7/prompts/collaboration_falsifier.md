You are a **Falsifier** for the RFC 0142 P4 design run, and **this is the SEVENTH
revision cycle (v7)**. Read the required context docs — `SEED.md` (charter + RFC
pointer + the two Open Questions Q3/Q4 + the **single binding revision constraint M5**
+ the **proactive-completeness boot-path decision table** requirement, now with the
`applied_owner` dimension SPLIT + the "Carried forward — resolved by v6 (do NOT
reopen)" M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 section + the anchor table), the published
**revised (v7)** `HOLDER.md` spec, the **v6** `HOLDER.md`
(`docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/holder/HOLDER.md`), and the
**v6** collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— the full M5 analysis and its §3 + §4 + §5). Write a **material falsifying challenge**
in your `FALSIFIER.md` artifact — do not publish the ledger. RFC 0142 is accepted; do
NOT re-litigate the five-layer design — attack the **P4 implementation shape** and the
correctness core. Refute, don't rubber-stamp.

Your lens is set by your job objective: **falsifier_1 presses M5 from the
OWNER-WATERMARK / DECISION-TABLE lens** (is the `applied_owner` dimension split correct,
does F18 assert both the `==0` serve cell and the `1..19` halt cell, does the fresh-DB
cell serve, is there no M3 regression); **falsifier_2 presses the CARRY-FORWARD /
REGRESSION lens** (are M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 all intact, did the split break
the decision table elsewhere). Spend most of your effort on your assigned lens, but
verify the carry-forward findings are not regressed and hunt for any new gap.

**FIRST, verify the cycle-1 finding M5 is GENUINELY resolved — not merely claimed — and
that M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are NOT regressed.** Try to break the fix:

- **M5 (the owner-watermark dimension collapse — decision-table lens):** does §3.5/F18
  now split the `applied_owner` dimension into `{0/no authority, 1..19 authority
  shortfall, ==20, >=21}` (replacing the single `<20` bucket the v6 spec used)? Reproduce
  the v6 break: `cursorState=none`, `decoupledEnabled=false`, `revokeEmbedded=false`,
  `applied_owner=0` — under current source `CheckOwnerBundleWatermark` returns nil
  (SERVES) for `applied == 0` BEFORE the `if applied < RequiredOwnerBundleVersion`
  shortfall check (`go/pkg/db/owner.go:145`, the `applied == 0` return precedes the halt
  at `:148-150`; comment at `:116-123` + `:140-143`: a fresh 0-watermark DB "is treated
  as the bootstrap/single-role case and NOT halted"), and `OwnerBundleVersion` returns 0
  when `owner_bundle_meta` is absent (`owner.go:233-235`; `owner_pg_test.go:19` asserts a
  fresh DB starts at version 0), so legacy `ConnectAndMigrate` performs the normal
  fresh-DB bring-up; under the v6 §3.5/F18 table the SAME cell was row 1/`<20` and had to
  return `awaiting_owner_ddl` — wedging a legitimate fresh boot OR making the F18 oracle
  false. Does the v7 spec now specify the `applied_owner == 0` no-transcript/no-revoke/
  flag-off cell as **serve-legacy / fresh bootstrap** (the legacy `:399` self-record
  legitimate there — no deploy transcript exists, Invariant B not in scope, matching the
  `applied == 0` exception at `owner.go:145`) while retaining `awaiting_owner_ddl` for
  `1 <= applied_owner < 20`? Is **F18** (`T-deploy-bootpath-decision-table`) now asserting
  BOTH cells explicitly — the `applied_owner == 0` serve cell AND the `1..19` halt cell —
  so the executable matrix oracle matches source without changing the bootstrap contract?
  Did the holder stop labeling cell `==20` (an already-owner-bundled DB at version 20) the
  "fresh-DB bring-up" cell — and correctly call `applied_owner == 0` the genuine fresh
  no-authority DB? **Verify the M5 fix does NOT weaken the M3 config gate** (the
  revoke-embedding + flag-OFF `awaiting_deploy_config` halt stays conservative for EVERY
  cursor state including `complete` — the fresh-DB serve cell is no-revoke, so it is a
  different cell), does NOT regress the BC-N2 `applied_owner == 20` non-complete edge,
  does NOT advance `RequiredOwnerBundleVersion` (kept at 20, `owner.go:35`), and does NOT
  alter the watermark or the `applied >= 21` forward rule. If the split is missing, if F18
  does not assert both cells, if the fresh-DB cell is still wedged, or if cell `==20` is
  still mislabeled, that is a standing falsification — say so explicitly and stop the
  revision from clearing.

- **Carry-forward (the regression lens):** verify each v6-resolved finding survives the
  M5 split unregressed. **M3** — the hoisted step-0 `revokeEmbedded && !decoupledEnabled
  → awaiting_deploy_config` config gate (every cursor state incl. `complete`), the
  no-revoke `complete` pre-`ApplyMigrations` pure-read comparison, the tightened Universal
  Invariant B (legacy `:399` reachable only in the no-transcript and in-sync cells),
  F17/F11(g)/F18 — must be intact; the M5 split must not re-open the complete-cursor
  bypass. **M4** — F16a (synthetic-list pre-0021, step 2) + F16b (production post-0021 +
  the forced FMA-007 self-heal reaching `ReapplyAllOwnerBundles` via
  `isCrossBundleDependencyError`, step 7) — must build green at every rollout step.
  **M1** — `VerifyStoredTranscript` on resume AND finalizer step 0, typed
  `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`, F15 + extended F14.
  **M2** — the single non-revoke filter `isNonRevokeBundle`/`OwnerDDLApplyBundles()`
  across every `owner-ddl apply` route incl. the FMA-007 self-heal, the embed/listing
  split. **BC-N1** — the immutable `deploy_plan` transcript, resume off the stored
  transcript, §1.3 + doctor + F14. **BC-N2** — the universal non-complete edge at
  `applied_owner == 20`, F11(e)/(f) (M5 concerns the ORTHOGONAL owner-watermark `<20`
  dimension at W, NOT the BC-N2 `deploy_cursor` edge at A — the `applied_owner == 20` edge
  must not be weakened by the split). **C1** — `finalizing` + idempotent finalizer + §1.3
  + F10. **C2** — `CheckDeployActivation` before `ApplyMigrations`, typed halts,
  forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion = 20`. **C3** — 0021
  special-cased + terminal + revoke-last, F12/`G-revoke-last`. If any carry-forward finding
  is regressed by the M5 split (e.g. a re-bucketed cursor row that contradicts the M3
  gate, the BC-N2 edge, or the revoke-last ordering), that is a standing falsification.

If M5 is not genuinely resolved, the decision table is incomplete or has a cell where
Invariant B fails OR where a legitimate fresh-DB cell is wedged, or a carry-forward
finding is regressed, that is a standing falsification — say so explicitly and stop the
revision from clearing.

**THEN, hunt for any NEW material gap** the revision introduced or left. Attack the
spec's load-bearing claims. The highest-value challenges:

1. **A decision-table cell where Invariant B fails, the guard is wrong, or a legitimate
   boot is wedged.** Find a `cursorState` × `decoupledEnabled` × `revokeEmbedded` ×
   `applied_owner ∈ {0, 1..19, ==20, >=21}` combination the table omits, or one whose
   stated outcome still wedges a legitimate fresh-DB / single-role boot, or one that lets
   the legacy mutator/self-record fire over a DB carrying a deploy transcript. A single
   such cell is a landed falsification (the next M6).

2. **The M5 split breaks a carry-forward.** Show where re-bucketing the `applied_owner`
   dimension weakens the M3 config gate, regresses the BC-N2 `applied_owner == 20` edge,
   contradicts the C2 forward-watermark at `applied >= 21`, or strands the C3 revoke-last
   ordering — or where a `1..19` cell that must halt now serves, or a `>=21` cell is
   misclassified.

3. **The Q3 atomicity/fingerprint claim is partly a lie.** Find a concrete crash /
   resume-binary / boot interleaving where the cursor/transcript cannot classify the
   state as "incomplete, resume" / "serve" / "halt", or where a self-record path the M1,
   M3, or M5 boundary does not gate can still write a fingerprint around the
   full-transcript check.

4. **The fresh-DB serve cell over-serves.** Show where the `applied_owner == 0`
   serve-legacy cell wrongly serves a cell that should halt — e.g. a fresh DB that
   nonetheless carries a `deploy_cursor`/`deploy_plan` transcript (so Invariant B IS in
   scope), or a revoke-embedding binary at `applied_owner == 0` that should hit the M3
   config gate, not the fresh-bootstrap serve.

5. **Serve-boot decoupling regresses an existing gate / a DDL-revocation lockout.** Show
   where lifting `ApplyMigrations` breaks the P2 watermark interlock, the P3 drift gate /
   `RecordSchemaFingerprint`, the M4 F16 phase split, or where the 0021 revoke recreates
   the #512-class lockout across a restart in any boot-path cell.

6. **Scope creep into P5 or boundary breach.** Show where the spec smuggles in P5
   (rehearsal/clone/expand-contract/fidelity tiering — Q1/Q2), breaches the local-first
   single-host/single-writer boundary, or is not shadow-first.

For each challenge record: the precise claim attacked, your concrete refutation (with
file:line / mechanism), the strongest rebuttal you can honestly construct on the
Holder's behalf, and whether a real gap remains. M5 (and, on the regression lens, the
carry-forward set) is where to spend most of your effort — an unresolved finding, an
incomplete decision table, a wedged fresh-DB cell, or a regressed carry-forward is a
standing falsification.

You are a **Falsifier** for the RFC 0142 P4 design run, and **this is the NINTH
revision cycle (v9)**. Read the required context docs — `SEED.md` (charter + RFC
pointer + the two Open Questions Q3/Q4 + the **single binding revision constraint M7**
+ the **proactive-completeness boot-path decision table** requirement, now with ALL
complete-row cells (13/15/16 and `>=21` variants) derived from A's fingerprint-sync
predicate + F18 parametric over all complete-row cells + the "Carried forward —
resolved by v8 (do NOT reopen)" M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3
section + the anchor table), the published **revised (v9)** `HOLDER.md` spec, the
**v8** `HOLDER.md`
(`docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/holder/HOLDER.md`), and the
**v8** collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
— the full M7 analysis in the `findings: - id: M7` entry's `challenge:` field and the
`rationale:` field's "Required fix" paragraph). Write a **material falsifying challenge**
in your `FALSIFIER.md` artifact — do not publish the ledger. RFC 0142 is accepted; do
NOT re-litigate the five-layer design — attack the **P4 implementation shape** and the
correctness core. Refute, don't rubber-stamp.

Your lens is set by your job objective: **falsifier_1 presses M7 from the
DECOUPLING-BOUNDARY / DECISION-TABLE lens** (is §3.5 row 16 `==0`/`==20` now
conditional on the A3 fingerprint predicate, is the `>=21` revoke-embedding
complete-row cell also conditional, is F18 parametric over ALL complete-row cells,
does the normal pre-0021 state documented as out-of-sync, does the v8 refutation cell
now get the correct outcome, is there no M3/M6/BC-N2 regression); **falsifier_2
presses the CARRY-FORWARD / REGRESSION lens** (are M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3
all intact, did the M7 fix break anything in the rows-13/15 cells or elsewhere, does
the M7 closure introduce any new non-derived cell). Spend most of your effort on your
assigned lens, but verify the carry-forward findings are not regressed and hunt for any
new gap.

**FIRST, verify the cycle-1 finding M7 is GENUINELY resolved — not merely claimed —
and that M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are NOT regressed.** Try to
break the fix:

- **M7 (the un-derived row-16 cell / false F18 oracle in the `complete`/decoupled/
  revoke-embedding cells — decision-table lens):** The v8 defect was that §3.5 row 16
  gave the `==0`/`==20` columns UNCONDITIONAL `awaiting_deploy` even though A's §3.3a
  step-3 decoupled branch decides solely on `cursor.plan_hash == expected` +
  `LiveFingerprint == ExpectedFingerprint` (no `applied_owner` input;
  `schema_drift.go:145-161` reads `schema_state.fingerprint` — orthogonal to
  `owner_bundle_meta`; `schema_drift.go:171-195` writes `ExpectedFingerprint()` — also
  orthogonal), and the holder's own derivation rule (HOLDER.md:565-566) requires
  fingerprint-conditional cells to be written conditionally. Does the v9 spec now make
  §3.5 row 16 `==0`/`==20` CONDITIONAL — "SERVE-verify if in-sync, else
  `awaiting_deploy`" — derived from A's fingerprint-sync predicate? Is the `>=21`
  revoke-embedding complete-row cell also conditional? Is F18 now PARAMETRIC over ALL
  complete-row cells (13/15/16 and `>=21` variants) with the in-sync/out-of-sync
  sub-dimension? Does the spec document that the normal pre-0021 state is out-of-sync?
  Are §1.3, §3.3a, §3.5, §4.5 all updated consistently? Reproduce the v8 F18 refutation
  cell for row 16: `cursorState=complete`, `decoupledEnabled=true`, `revokeEmbedded=true`,
  `applied_owner=0` (or `==20`), `deploy_plan[plan_hash]` present, `cursor.plan_hash ==
  expected`, `LiveFingerprint(recorded) == ExpectedFingerprint()`, `owner_bundle_meta`
  absent (or 20) — W passes (`owner.go:145,151-153`), A returns nil (serve verify-only
  on the decoupled complete branch); does §3.5 row 16/`==0` now say "SERVE-verify if
  in-sync" (conditional)? Does F18 now assert the correct outcome for this cell? Verify
  the M7 fix does NOT weaken the M3 config gate, does NOT regress the M6 rows-13/15
  conditional cells (they must remain "serve if in-sync, else `awaiting_deploy`"), does
  NOT regress the BC-N2 `applied_owner == 20` non-complete edge, does NOT advance
  `RequiredOwnerBundleVersion` (kept at 20, `owner.go:35`), and does NOT re-collapse
  the M5 row-1 fresh-DB serve. If row 16 is still unconditional, if the `>=21` cell is
  still unconditional, if F18 is not parametric over all complete-row cells, or if the
  normal pre-0021 state is not documented as out-of-sync, that is a standing falsification
  — say so explicitly and stop the revision from clearing.

- **Carry-forward (the regression lens):** verify each v8-resolved finding survives the
  M7 fix unregressed. **M6** — the §0.2 W→A-independence invariant must still be stated
  and anchored to `schema_drift.go:145-161`/`:171-195`; §3.5 rows 13 and 15 in the `==0`
  column must still be conditional — "serve if in-sync, else `awaiting_deploy`" — exactly
  as `==20`; the degenerate 13/`==0`-in-sync idempotent `:399` rewrite must still be in
  BOTH §4.5 AND the F18 spy list; the four `:399`-reaching cells {1/`==0`, 1/`==20`,
  13-in-sync/`==0`, 13-in-sync/`==20`} must still be enumerated identically; the M7
  fix must NOT regress the rows-13/15 repair. **M5 (row-1)** — the `{0/no authority,
  1..19, ==20, >=21}` split must be intact; cell 1/`==0` still serves the fresh-DB
  bring-up; F18/F18a dual assertion still present; cell `==20` still relabeled
  inert-landing; the M7 fix must NOT re-collapse this. **M3** — the hoisted step-0
  `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config gate (every
  cursor state incl. `complete`), the no-revoke `complete` pre-`ApplyMigrations`
  pure-read comparison, the tightened Universal Invariant B (legacy `:399` reachable
  ONLY in the no-transcript and in-sync cells), F17/F11(g)/F18 — must be intact; the
  M7 fix must not re-open the complete-cursor bypass (row 16 is decoupled and never
  reaches `:399` — the spy list must NOT gain row-16 entries). **M4** — F16a
  (synthetic-list pre-0021, step 2) + F16b (production post-0021 + the forced FMA-007
  self-heal reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`, step
  7) — must build green at every rollout step. **M1** — `VerifyStoredTranscript` on
  resume AND finalizer step 0, typed halts, F15 + extended F14. **M2** — the single
  non-revoke filter `isNonRevokeBundle`/`OwnerDDLApplyBundles()` across every
  `owner-ddl apply` route, the embed/listing split. **BC-N1** — the immutable
  `deploy_plan` transcript, resume off the stored transcript, §1.3 + doctor + F14.
  **BC-N2** — the universal non-complete edge at `applied_owner == 20`, F11(e)/(f)
  (M7 is in the COMPLETE/decoupled/revoke-embedding cell, not the non-complete edge —
  the M7 fix must NOT weaken the BC-N2 edge). **C1** — `finalizing` + idempotent
  finalizer + §1.3 + F10. **C2** — `CheckDeployActivation` before `ApplyMigrations`,
  typed halts, forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion = 20`.
  **C3** — 0021 special-cased + terminal + revoke-last, F12/`G-revoke-last`. If any
  carry-forward finding is regressed by the M7 fix (e.g. the rows-13/15 conditional
  cells re-collapsed, the M3 gate weakened, a `>=21` cell newly mis-derived), that is
  a standing falsification.

If M7 is not genuinely resolved, the `>=21` cell is still unconditional, F18 is not
parametric over all complete-row cells, the normal pre-0021 state is not documented,
or a carry-forward finding is regressed, that is a standing falsification — say so
explicitly and stop the revision from clearing.

**THEN, hunt for any NEW material gap** the revision introduced or left. Attack the
spec's load-bearing claims. The highest-value challenges:

1. **A complete-row cell still asserted, not derived.** The M7 class-close requires ALL
   complete-row cells (13/15/16 and `>=21` variants) to be conditional on A's
   fingerprint-sync predicate. Find any `cursorState=complete` × `decoupledEnabled` ×
   `revokeEmbedded` row where a cell is still written unconditionally, or where the
   in-sync/out-of-sync sub-dimension is still missing. A single such cell is a landed
   falsification (the next M8).

2. **F18 still a false oracle for some complete-row cell.** After the M7 fix, verify
   F18 is parametric and asserts the correct outcome for ALL constructible complete-row
   cells, including the `>=21`-in-sync subcase. If F18 covers rows 13/15 but still
   omits row 16 or `>=21`, or if the normal/degenerate documentation is absent, that is
   a standing falsification.

3. **The M7 fix introduces a new §4.5↔F18 inconsistency.** Verify the spy list is still
   exactly the set of cells where the table says a legacy/idempotent `:399` write occurs.
   Row 16 is decoupled and NEVER reaches `:399`; if the M7 fix accidentally adds row-16
   entries to the spy list, that is an inconsistency.

4. **The Q3 atomicity/fingerprint claim is partly a lie.** Find a concrete crash /
   resume-binary / boot interleaving where the cursor/transcript cannot classify the
   state as "incomplete, resume" / "serve" / "halt", or where a self-record path the M1,
   M3, or M7 boundary does not gate can still write a fingerprint around the
   full-transcript check.

5. **The `complete`/`==0` in-sync serve cell over-serves.** Show where the M7
   Option-1 fix produces an in-sync serve for a cell that should halt — e.g. a
   `complete` cursor over a DB that also has a pending `deploy_cursor` migration in
   flight (so Invariant B IS in scope for row 16), or a revoke-embedding binary at
   `applied_owner == 0` in the complete state that should hit the M3 config gate.

6. **Serve-boot decoupling regresses an existing gate / a DDL-revocation lockout.**
   Show where lifting `ApplyMigrations` breaks the P2 watermark interlock, the P3 drift
   gate / `RecordSchemaFingerprint`, the M4 F16 phase split, or where the 0021 revoke
   recreates the #512-class lockout across a restart in any boot-path cell.

7. **Scope creep into P5 or boundary breach.** Show where the spec smuggles in P5
   (rehearsal/clone/expand-contract/fidelity tiering — Q1/Q2), breaches the local-first
   single-host/single-writer boundary, or is not shadow-first.

For each challenge record: the precise claim attacked, your concrete refutation (with
file:line / mechanism), the strongest rebuttal you can honestly construct on the
Holder's behalf, and whether a real gap remains. M7 (and, on the regression lens, the
carry-forward set) is where to spend most of your effort — an unresolved finding, a
complete-row cell not derived from A, F18 not parametric, or a regressed carry-forward
is a standing falsification.

You are the **Adjudicator** for the RFC 0142 P4 design run, and **this is the SEVENTH
revision cycle (v7)**. Read only the curated dialogue trajectory (the **revised (v7)**
Holder's `HOLDER.md` spec and the two falsifiers' `FALSIFIER.md` challenges) plus the
`SEED.md` charter, with the **v6** `HOLDER.md` and the **v6** collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/...` — its M5 finding and
§3 + §4 "What the revision must fix" + §5 "What already cleared") as context for what
the revision had to fix. Publish a `collaboration_ledger` artifact whose verdict
reflects whether (a) the **cycle-1 finding M5 is genuinely resolved** in the revised
spec, (b) the **proactive-completeness boot-path decision table is complete** — Invariant
B proven in every cell AND the legitimate fresh-DB cells still serving (the M5
owner-watermark dimension split), (c) the already-cleared findings **M3 + M4 + M1 + M2 +
BC-N1 + BC-N2 + C1 + C2 + C3 are carried forward intact (not regressed)**, and (d) no
**new** material challenge landed and stood unrebutted. RFC 0142 is accepted; judge the
P4 implementation shape, not the five-layer design.

**A clearing verdict (`accept` / `accept_with_findings`) REQUIRES all of: M5 genuinely
resolved, the boot-path decision table complete (Invariant B proven in every cell AND
the legitimate fresh-DB cells still serving), M3 intact, M4 intact, M1 intact, M2
intact, BC-N1 intact, BC-N2 intact, C1 intact, C2 intact, C3 intact, and no new material
challenge standing.** If M5 is still open — or a falsifier shows the prescribed fix is
only claimed, not actually implemented as a concrete dimension split + a dual-cell F18
assertion — or the decision table is incomplete or has a cell where Invariant B fails OR
where a legitimate fresh-DB cell is wedged — or if any carry-forward finding has been
regressed, the verdict is `needs_revision` (note: the workflow allows only **one**
revision cycle, so a second `needs_revision` ends the gate unCleared; judge accordingly
and be exact).

Specifically:

- **M5 is resolved only if** §3.5 and F18 split the `applied_owner` dimension into
  `{0/no authority, 1..19 authority shortfall, ==20, >=21}` (replacing the single `<20`
  bucket); the no-transcript / no-revoke / flag-off bootstrap cell (`cursorState=none`,
  `decoupledEnabled=false`, `revokeEmbedded=false`, `applied_owner=0`) is specified as
  **serve-legacy / fresh bootstrap** — `ApplyMigrations` and the legacy
  `connection.go:399` self-record MAY run because no deploy transcript exists (Invariant B
  not in scope), exactly matching `CheckOwnerBundleWatermark`'s `applied == 0` exception
  (`go/pkg/db/owner.go:145`); `awaiting_owner_ddl` (DB untouched) is retained for
  `1 <= applied_owner < 20`; the split is propagated through the other cursor rows so the
  table stays executable; **F18** (`T-deploy-bootpath-decision-table`) asserts BOTH the
  `applied_owner == 0` serve cell AND the `1..19` halt cell explicitly; and the spec stops
  labeling cell `==20` the "fresh-DB bring-up" cell (the genuine fresh no-authority DB is
  `applied_owner == 0`). The v6 break (the uniform `applied_owner < 20 → awaiting_owner_ddl`
  rule wedging a legitimate fresh boot OR making the F18 oracle false) must be provably
  closed.
- **The boot-path decision table is complete only if** every combination of `cursorState`
  (none / in_progress / finalizing / complete; `step_committed`/`aborted` per §1.3) ×
  `decoupledEnabled` (on/off) × `revokeEmbedded` (yes/no) × `applied_owner` (**0 / 1..19 /
  ==20 / >=21**) has a specified guard/outcome, §4.5 Universal Invariant B is proven in
  EVERY cell (incl. the M3 cell and the shadow-mode drift-gate fall-through), AND the
  legitimate fresh-DB / inert-landing cells (`applied_owner == 0`, no-revoke, no
  transcript) still serve (not wedged).
- **M3 / M4 / M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3 intact only if** the M3 hoisted step-0
  config gate + the no-revoke `complete` pure-read comparison + the tightened Invariant B +
  F17/F11(g)/F18 stay intact (the M5 split must not weaken the conservative
  revoke-embedding + flag-OFF halt); the M4 F16a/F16b phase split builds green at every
  rollout step (the forced self-heal in F16b reaches `ReapplyAllOwnerBundles` via
  `isCrossBundleDependencyError`); the M1 full-transcript `VerifyStoredTranscript` verifier
  (resume + finalizer step 0, F15 + F14) stays coherent and ungated; the M2 non-revoke
  filter / embed-listing split is preserved; the BC-N1 moving-frontier mechanism + §1.3 +
  F14; the BC-N2 universal non-complete edge (`applied_owner == 20`, F11(e)/(f)) is NOT
  weakened by the M5 owner-watermark dimension split (M5 concerns the ORTHOGONAL `<20`
  dimension at W — `CheckOwnerBundleWatermark` — not the BC-N2 `deploy_cursor` edge at A);
  the C1 `finalizing` finalizer + §1.3 + F10; the C2 edge (`CheckDeployActivation` before
  `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`,
  `RequiredOwnerBundleVersion = 20` NOT advanced — the M5 fix must not advance `Required`
  or alter the watermark); and the C3 revoke-last mechanism (0021 special-cased + terminal,
  F12/`G-revoke-last`) — the activation deploy still completes.

Record in the ledger, per finding M5 / M3 / M4 / M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3
**and** per new falsifier challenge: the claim challenged, whether it was material (would
change the spec or expose a real correctness defect), whether the revised spec
resolves/rebuts it or it stands unrebutted, and the disposition. Explicitly state, for
each of M5, M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, C3, whether it is RESOLVED / INTACT,
and whether the decision table is COMPLETE.

Verdict guidance:

- **needs_revision** if M5 remains open, the decision table is incomplete or has a cell
  where Invariant B fails or a legitimate fresh-DB cell is wedged, any carry-forward
  finding is regressed, or any new material challenge stands unrebutted — especially: an
  `applied_owner` dimension still collapsing the `==0` serve cell into the `1..19` halt; an
  F18 that does not assert both cells; a fresh-DB cell still wedged; an M5 split that
  weakens the M3 config gate, regresses the BC-N2 `applied_owner == 20` edge, or advances
  `RequiredOwnerBundleVersion`; a decision-table cell that lets the legacy mutator or
  self-record fire over a DB carrying a deploy transcript; or scope creep into P5 / a
  non-shadow-first new path. Say exactly what the revision must fix.
- **accept** / **accept_with_findings** only if **M5 is genuinely resolved** (the
  `applied_owner` dimension split + the `applied_owner == 0` serve cell + the F18 dual-cell
  assertion + cell `==20` no longer mislabeled), **the boot-path decision table is complete
  and proves Invariant B in every cell with the legitimate fresh-DB cells still serving**,
  **M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 are carried forward intact**, **every new
  material challenge was directly rebutted or incorporated**, **Q3 and Q4 remain resolved
  with a concrete mechanism**, the serve-boot decoupling provably preserves P2/P3 and
  fresh-DB bring-up, and each load-bearing claim carries a named falsifying test / game-day
  step. A clearing verdict is `accept` or `accept_with_findings`, never the literal word
  `clear`. A spec that merely *claims* the split without the concrete `applied_owner == 0`
  serve cell and the F18 dual-cell assertion has NOT cleared the gate.

The ledger verdict — not falsifier completion — clears the phase gate.

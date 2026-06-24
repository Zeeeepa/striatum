# Design-Run Seed (v9 / REVISION) — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> **THIS IS THE NINTH REVISION (v9).** Eight prior design runs ran the same
> falsification gate. v1 (`rfc-0142-p4-design`) returned **`needs_revision`** with
> three findings C1/C2/C3. v2 (`rfc-0142-p4-design-v2`) **resolved C1 and C2** but
> returned `needs_revision` again (C3 still open + a new finding N1). v3
> (`rfc-0142-p4-design-v3`) **resolved C3** (the revoke-last ownership mechanism; both
> falsifiers conceded it) and closed the immediate N1 hole, BUT returned
> `needs_revision` on two NEW grounds BC-N1 + BC-N2. v4 (`rfc-0142-p4-design-v4`)
> **resolved BOTH BC-N1 and BC-N2** but returned `needs_revision` on two independent
> NEW material challenges M1 + M2. v5 (`rfc-0142-p4-design-v5`) **resolved BOTH M1 and
> M2** but returned `needs_revision` on two new findings M3 + M4. v6
> (`rfc-0142-p4-design-v6`) **resolved BOTH M3 and M4** — both v6 falsifiers AND the v6
> adjudicator explicitly concede each (M3's hoisted step-0 config gate closes the
> complete-cursor legacy mutate+self-record bypass; M4's phase-aware F16a/F16b split
> builds green before 0021 is authored), and M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward
> intact — BUT returned `needs_revision` again on **M5**. v7 (`rfc-0142-p4-design-v7`)
> **resolved M5 row-1** (the `{0/no authority, 1..19 shortfall, ==20, >=21}` split; cell
> 1/`==0` now serves the fresh-DB bring-up; F18/F18a assert both cells; cell `==20`
> relabeled inert-landing) — BOTH v7 falsifiers AND the v7 adjudicator explicitly concede
> the row-1 repair — BUT returned **`needs_revision` again** on **M6**. v8
> (`rfc-0142-p4-design-v8`) **resolved M6** (rows 13/15 — the M5 `applied_owner` split
> propagated coherently through the no-revoke `complete` rows; §0.2 states the W→A-independence
> invariant anchored to `schema_drift.go:145-161`/`:171-195`; the degenerate
> 13/`==0`-in-sync `:399` rewrite added to BOTH §4.5 AND the F18 spy list; the four
> `:399`-reaching cells enumerated identically; the cross-row audit covers all cursor groups)
> — BOTH v8 falsifiers AND the v8 adjudicator explicitly concede the rows-13/15 repair
> and confirm M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 carried forward intact —
> BUT returned **`needs_revision` again** (the gate's single allowed
> cycle) on **one new finding, source-verified against the run worktree and landed
> INDEPENDENTLY by BOTH v8 falsifiers**:
> **M7 (LOAD-BEARING)** — §3.5 row 16 (`cursorState=complete`, `decoupledEnabled=true`,
> `revokeEmbedded=true`) gives the `==0` and `==20` columns UNCONDITIONAL
> `awaiting_deploy`, reasoned '0021 not yet applied → fingerprint ≠ → not in-sync'. But
> A's §3.3a step-3 decoupled branch decides solely on `cursor.plan_hash == expected` +
> `LiveFingerprint == ExpectedFingerprint` — with NO `applied_owner` input — and the
> holder's OWN derivation rule (HOLDER.md:565-566) says 'where A's outcome is conditional
> on the fingerprint-sync state … the cell is written conditionally'. Row 16's
> complete/decoupled outcome IS conditional on fingerprint-sync, yet the cell is written
> UNCONDITIONALLY. Because `schema_state` and `owner_bundle_meta` are orthogonal tables
> (source-verified: `schema_drift.go:145-161`/`:171-195` have zero `owner_bundle_meta`
> references), the in-sync row-16 cell (`schema_state.fingerprint == ExpectedFingerprint()`
> AND `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent / version-20 DB)
> IS constructible — exactly as the holder constructs the degenerate 13/`==0`-in-sync
> cell. For that cell W returns nil, A returns nil (serve verify-only on the decoupled
> complete branch), but §3.5 says `awaiting_deploy`. F18 is therefore a FALSE ORACLE for
> the in-sync row-16 `==0`/`==20` cells, OR the build must smuggle in an unstated
> `applied_owner`-dependent guard contradicting the W→A-independence claim. This v9 run
> is a **proper revision**: the holder starts from the **v8** `HOLDER.md` (a required
> context doc), REVISES the spec to **resolve M7** per the prescribed fix below (make row
> 16 and its `>=21` variant conditional on the A3 fingerprint predicate; propagate through
> §1.3/§3.3a/§3.5/§4.5/F18; make F18 parametric over ALL complete-row cells with the
> in-sync/out-of-sync sub-dimension), keeps the proactive-completeness boot-path decision
> table, and **carries M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 forward
> unregressed**; the falsifiers re-attack the revised spec. The v8 design record —
> `dialogue/holder/HOLDER.md`, `dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0142-p4-design-v8/`; the **v8** `HOLDER.md` (the revised
> spec) and the **v8** collaboration ledger (the verdict + the full M7 analysis and
> `rationale:` 'Required fix') are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries the
> charter, restates the two Open Questions P4 must pin (Q3 + Q4), pins the **single
> binding revision constraint M7**, states what already cleared
> (M6 + M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3) and must NOT be
> reopened, names the proactive-completeness boot-path decision table the holder MUST
> keep (now with EVERY complete-row cell derived mechanically from A's fingerprint-sync
> predicate), and pins an operator anchor-verification table you must build on. Read this
> whole file, the **v8** `HOLDER.md` + the **v8** collaboration ledger context docs, and
> the RFC (esp. the Phasing table P4 row and the "Open Questions" section) before
> producing any artifact.

## Charter — what this run must produce

This is a **design run**, not an implementation run. RFC 0142 is **already
accepted**; this run does NOT re-open the five-layer design. The deliverable is a
**falsifiable implementation spec for P4 only** — the one-shot `striatum daemon
deploy` that lifts schema-apply out of serve-boot — that the `rfc-0142-p4-build`
run can execute contract-first (TDD), produced by hardening the P4 shape against
adversarial falsification. **This judges pure engineering rigor — there is NO
maintainer ratification gate; the adjudicator's collaboration ledger decides.**

The committed `PROPOSAL.md` MUST:

1. **Resolve Q3 and Q4** (below) with a concrete, defensible decision each. Q3 is
   "the hard correctness core" — a P4 spec that leaves the resumability/fingerprint
   contract unproven for the interleavings we ship has not cleared the gate.
2. **Specify the deployer surface, the serve-boot decoupling, and the DDL
   revocation** by exact code site (anchor table below), shadow-first.
3. **State every load-bearing correctness claim as a falsifiable assertion**
   paired with the named test / game-day step that would prove it false.
4. **Stay inside the accepted design and the local-first boundary**, and
   explicitly **defer P5** (rehearsal / expand-contract / fidelity tiering / clone
   mechanism = Q1/Q2) — P4 is the deployer + decoupling + DDL revocation only.
5. **Resolve the single binding revision constraint (M7) below**, per its
   prescribed fix, KEEP the **proactive-completeness boot-path decision table** (now
   with EVERY complete-row cell derived from A's fingerprint-sync predicate, including
   row 16 and its `>=21` variant), and **carry forward
   M6 + M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3 unregressed**
   (see "Carried forward"). A revised spec that leaves M7 open — or that regresses any
   carry-forward finding, or whose decision table has a cell where Universal Invariant B
   fails OR where the F18 oracle disagrees with the §3.3a A predicate — has NOT cleared
   the gate. This is the gate's single allowed revision cycle for v9, so a second
   `needs_revision` ends the gate unCleared.

## Carried forward — resolved by v8 (do NOT reopen)

> The v8 collaboration ledger records M6 as genuinely resolved **this cycle**
> (both v8 falsifiers explicitly concede the rows-13/15 repair), and M5(row-1), M3,
> M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 as intact and unregressed (both falsifiers
> concede each carry-forward). The v9 revision MUST preserve them — verbatim from the
> **v8** `HOLDER.md` — and the cycle-1 adjudicator's verdict records them intact. Do
> NOT re-litigate any of them; they are not this cycle's open question. M7 is a
> propagation of the same fingerprint-derivation requirement to the complete/decoupled/
> revoke-embedding cell (row 16), NOT a re-opening of the M6 rows-13/15 fix, the M5
> row-1 fix, the M3 config gate, the M4 F16 split, or any of the nine earlier fixes.
> The M7 fix must NOT re-collapse the resolved rows-13/15 conditional cells.

### M6 — RESOLVED (rows 13/15) by v8 (the M5 split propagated through the no-revoke `complete` rows). Preserve verbatim from the v8 HOLDER.

The §0.2 W→A-independence invariant is stated explicitly and anchored to source
(`schema_drift.go:145-161`/`:171-195`): once W passes, A is owner-watermark-independent
— it reads `cursorState`, `decoupledEnabled`, `revokeEmbedded`, `cursor.plan_hash`, and
the recorded `schema_state.fingerprint`, NEVER `owner_bundle_meta`/`applied_owner` —
so the `==0` and `==20` columns have IDENTICAL A-gate outcomes in EVERY cursor row.
§3.5 rows 13 and 15 in the `==0` column are now CONDITIONAL — "serve if in-sync, else
`awaiting_deploy`" — IDENTICAL to `==20`. The degenerate 13/`==0`-in-sync idempotent
`:399` rewrite is added to BOTH the §4.5 Universal-Invariant-B enumeration AND the F18
spy list, so the four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`,
13-in-sync/`==20`} are enumerated identically. §3.5 carries the explicit cross-row
`==0`-vs-`==20` audit (none/in_progress/finalizing/complete). The M7 fix
is local to the `complete`/decoupled/revoke-embedding cell (row 16); it MUST NOT
regress the rows-13/15 conditional cells, must NOT weaken the M3 config gate, must NOT
regress the BC-N2 `applied_owner == 20` edge, and must NOT advance
`RequiredOwnerBundleVersion`.

### M5 — RESOLVED (row-1) by v7 (the owner-watermark dimension split). Preserve verbatim from the v8 HOLDER.

The `applied_owner < 20` bucket is split into `{0/no authority, 1..19 authority
shortfall, ==20, >=21}` (covering all of ℕ); W = `CheckOwnerBundleWatermark` is
respecified per the live contract — `applied_owner == 0` returns nil and proceeds to A
(the fresh/single-role/no-authority bootstrap exception, `owner.go:145`); `1 <=
applied_owner < 20` halts `awaiting_owner_ddl` DB-untouched; `==20` returns nil;
`>=21` is the forward-watermark barrier (b). Cell 1/`==0` (no-revoke, flag off,
`cursorState=none`, no transcript) **SERVES-legacy** the fresh-DB bring-up — the legacy
`:399` self-record is legitimate there (no deploy transcript, Invariant B not in scope);
cell 1/`==20` is relabeled the inert-landing re-boot cell; F18 asserts BOTH the
`applied_owner == 0` serve cell AND the `1..19` halt cell; F18a
(`T-deploy-fresh-db-bootstrap-serves`) pins the fresh-DB serve. **The M7 fix is in the
`complete`/decoupled/revoke-embedding row (row 16); it must NOT re-collapse the resolved
row-1 fresh-DB serve, must NOT weaken the M3 config gate (cells 2/6/10/14 must still
halt `awaiting_deploy_config` at A0 in EVERY column that passes W incl. `0`), and must
NOT regress the BC-N2 `applied_owner == 20` edge.**

### M3 — RESOLVED by v6 (the COMPLETE-cursor activation guard). Preserve verbatim from the v8 HOLDER.

The v5 break (a revoke-embedding binary with the flag OFF over a `complete` transcript
rode the legacy `ConnectAndMigrate` `ApplyMigrations` + `RecordSchemaFingerprint` path
around `VerifyStoredTranscript`) is closed by:
- the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config halt **HOISTED
  to step 0** of `CheckDeployActivation`, BEFORE any cursor-state branch, so it fires for
  EVERY cursor state including `complete` (barrier (a)) — a revoke-embedding binary with
  `STRIATUM_DEPLOY_DECOUPLED` OFF returns `awaiting_deploy_config` DB-untouched before
  `ApplyMigrations` (`connection.go:353`) and before `RecordSchemaFingerprint` (`:399`) on
  BOTH `ConnectAndMigrate` and `ConnectAndVerify`;
- the no-revoke `complete` residual handled by a **pre-`ApplyMigrations`, DB-untouched
  PURE-READ comparison** (`ExpectedFingerprint() == LiveFingerprint(recorded)` AND
  `cursor.plan_hash == expected`) that serves only when already in-sync, else halts
  `awaiting_deploy` DB-untouched;
- **Universal Invariant B TIGHTENED** so the legacy `connection.go:399` writer is reachable
  ONLY in §3.5 cells 1 (`none`, no transcript) and 13-in-sync (`complete`, no-revoke,
  already byte-in-sync), barred from any revoke-embedded, revoke-applied, or
  pending-change-over-a-transcript state;
- **F17**, **F11(g)**, and **F18** (`T-deploy-bootpath-decision-table`).
Preserve the hoisted step-0 config gate, the no-revoke `complete` pure-read comparison, the
tightened Invariant B, and F17/F11(g)/F18. **The M7 fix is in the decoupled/revoke-embedding
cell (row 16, decoupled ON); it does NOT re-open the M3 legacy bypass (row 16 is
decoupled and never reaches the legacy `:399` writer); the revoke-embedding + flag-OFF
halt must stay conservative.**

### M4 — RESOLVED by v6 (F16 phase split). Preserve verbatim from the v8 HOLDER.

The v5 staging defect (F16 asserted production `OwnerBundles()` contains 0021 in rollout
step 2 while 0021 is not authored until step 7 — un-buildable) is closed by:
- **F16a** (`TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, rollout step 2) — drives the
  exclusion filter through a synthetic bundle list / injectable-FS test hook, asserts
  `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`, both
  `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a hand-passed synthetic 0021,
  and the `ReapplyAllOwnerBundles(nil, …)` fallback resolves to the filtered loader, WITHOUT
  asserting production `OwnerBundles()` contains 0021 (so it builds green before 0021 exists);
- **F16b** (`TestOwnerDDLApplyExcludesProductionRevokeBundle`, rollout step 7 after 0021 is
  authored) — asserts production `OwnerBundles()` contains 0021, `ExpectedFingerprint()`
  includes its bytes, `revokeEmbedded` derives from the full loader / file presence, and
  production `OwnerDDLApplyBundles()` excludes it; the forced FMA-007 self-heal pgtest lives
  here and is required to reach `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`
  (`go/pkg/db/owner.go:367-374`), not merely the pending loop.
Preserve the F16a synthetic-list phase, the F16b production phase, and the forced
self-heal pgtest in F16b. **M4 only restructured F16's staging, not the M2 filter.**

### M1 — INTACT (full-transcript byte + DB-stamp verification). Preserve verbatim from the v8 HOLDER.

A single **`VerifyStoredTranscript(plan_hash)` (§3.4a)** that checks the **ENTIRE**
stored transcript — every step, already-applied AND not-yet-applied — `sha256` against
the running binary's embedded bytes (reusing `MigrationSHASet()` and `OwnerBundle.SHA256()`);
**ANY** mismatch ⇒ `deploy_plan_binary_mismatch`; for **already-applied** entries it ALSO
verifies the **DATABASE STAMPS** (`schema_migrations.sha256` / `owner_bundle_meta.sha256`)
⇒ `deploy_plan_db_stamp_mismatch`; the **SAME** verifier runs on **every resume** AND as
**finalizer step 0** before the `complete` receipt / `RecordSchemaFingerprint` /
`finalizing → complete` advance; **F15** + extended **F14**. M3 EXTENDS Invariant B's
enforcement to the legacy/complete-cursor path — it does NOT re-open or gate the deployer
verifier. Preserve all of it. **(Note: M7's row-16 defect exposed that `VerifyStoredTranscript`
is specified on deploy resume / finalizer step 0, NOT on ordinary serve-boot over an
already-`complete` cursor — which is WHY the M7 fix chooses Option 1 (conditional cell)
rather than adding a new boot-time guard; M1 is not regressed by the M7 fix.)**

### M2 — INTACT (DDL-revoke bundle excluded from every owner-ddl apply route). Preserve verbatim from the v8 HOLDER.

A single non-revoke filter **`isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)`**
and a split loader **`OwnerDDLApplyBundles()` = `filter(OwnerBundles(), isNonRevokeBundle)`**
as the ONLY bundle slice every `owner-ddl apply` route iterates: `ApplyOwnerBundles` loads
the filtered slice; both `applyPendingOwnerBundles` AND the FMA-007 self-heal
`ReapplyAllOwnerBundles` additionally guard `isNonRevokeBundle` in-loop; the
`ReapplyAllOwnerBundles(nil, …)` fallback loads the filtered slice; the **embed/listing
split** keeps `OwnerBundles()` (full, includes 0021) ONLY for `revokeEmbedded`,
`ExpectedFingerprint`, `BuildPlan`, and `RuntimeOwnedTablesAlterable()`. F16 (now F16a/F16b
per M4) + F12/`G-revoke-last`. Preserve `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, the
in-loop guards, the nil-fallback split, and the embed/listing split.

### BC-N1 — INTACT (immutable plan identity across resume). Preserve verbatim from the v8 HOLDER.

The runtime-owned **`deploy_plan` table** (migration 0044) storing the IMMUTABLE ordered
transcript keyed by `plan_hash`, materialized in the same tx that sets
`deploy_cursor → in_progress(0)` BEFORE step 0; **resume loads
`deploy_plan[cursor.plan_hash]`** and never recomputes `BuildPlan` over the moved frontier;
the **§1.3** stored-transcript reclassification; the transcript-enumerated doctor; **F14**.
Preserve all of it.

### BC-N2 — INTACT (universal pre-revoke serve edge). Preserve verbatim from the v8 HOLDER.

`CheckDeployActivation` is not gated on `revokeEmbedded`; every deployer-aware binary reads
`deploy_cursor` and halts **`awaiting_deploy` DB-untouched on a non-`complete` cursor** at
**`applied_owner == 20`**; F11(e)/(f) + extended `G-old-binary-refuse`. **M3's
complete-cursor extension sits ABOVE and BESIDE the BC-N2 non-complete edge — it does NOT
weaken it. M7 is in the COMPLETE/decoupled/revoke-embedding cell (row 16); it does NOT
concern the BC-N2 `deploy_cursor` edge at A — the resolved `applied_owner == 20` BC-N2
edge must NOT be regressed by the M7 fix.** Preserve the universal
`revokeEmbedded`-independent edge, the `applied_owner == 20` halt, F11(e)/(f), and
the extended `G-old-binary-refuse`.

### C1 — INTACT (finalization boundary). Preserve verbatim from the v8 HOLDER.

The distinct **`finalizing`** `deploy_cursor` state; an **idempotent finalizer** (guarded
`complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST), gated by
M1's pre-finalizer `VerifyStoredTranscript` step 0; the **§1.3 row** classifying
`finalizing` as resumable finalization; and **F10**. Preserve all of it.

### C2 — INTACT at the fail-closed EDGE. Preserve verbatim from the v8 HOLDER.

**`CheckDeployActivation`** called immediately after `CheckOwnerBundleWatermark` and
**BEFORE `ApplyMigrations`** in both boot paths, mutating nothing; the typed
non-restartable halts **`awaiting_deploy`** and **`awaiting_deploy_config`**; the
**forward-watermark rule** firing at **`applied >= 21`**; **`RequiredOwnerBundleVersion`
KEPT at its current-`main` value 20** — NOT advanced to the revoke ordinal 21. M3 EXTENDS
this edge to the `complete`-cursor case without advancing `Required` or altering the
watermark. **The M7 fix makes row 16 conditional; it must NOT advance `Required`, alter
the watermark, or change the `applied >= 21` forward rule.**
Preserve all of it, including F11.

### C3 — INTACT (ownership transfer, revoke-last). Preserve verbatim from the v8 HOLDER.

The DDL-revoke bundle (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`), re-anchored
0020→**0021**, identified by `DDLRevokeOwnerBundleVersion = 21`, **special-cased OUT of the
owner prefix** and **EXCLUDED from `owner-ddl apply`**, applied as the **TERMINAL plan step**
after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while
`striatumd_rw` still holds CREATE granted by 0018; the committed steady state denies CREATE);
F12 (`T-deploy-runtime-object-ownership`) in a non-superuser two-role cluster +
`G-revoke-last`. Preserve the design mechanism, F12, and `G-revoke-last`. **(Note: M7's
row-16 defect arises BECAUSE the table imports the C3 reachability fact — 'a completed
revoke deploy means applied_owner >= 21' — into A without A verifying the owner-bundle
stamp before serving; the M7 fix resolves this by making the cell conditional on
fingerprint-sync rather than on owner-watermark state, which is the correct derivation from
A; C3 itself is not regressed.)**

### Other carry-forward (v1–v8 ledgers — do NOT re-litigate)

- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`), bootstrapping
  paradox dispositive, run-shape door held by three seams. No falsifier contested it.
- **Q3 mid-step resumability body — sound.** Q3-A (cursor advance + receipt in the
  step transaction), Q3-B (`in_progress` pre-marker + invalid-index reconciler), the
  §1.3 step taxonomy, the `deploy_cursor`-as-second-signal insight. M3/M5/M6/M7 are
  about serve-boot guarding and the boot-path decision table, not the per-step
  schema/cursor body — preserve the body, close the decision table coherently.
- **Decoupling shape — sound (now M3-hardened).** Shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark interlock +
  P3 drift gate / self-record, the embed-FS-derived plan, the hash-chained per-step
  receipt, `doctor schema_deploy_unrecorded`.
- **Scope discipline held.** P5 deferred; full capability revocation named as a
  follow-up beyond P4, not assumed; the local-first boundary respected.

## The single binding constraint v9 MUST resolve (the v8 adjudicator's needs_revision ground)

> **This is BINDING.** The design-v8 falsification gate returned `needs_revision`
> because a material new falsifier challenge landed unrebutted (**M7**), source-verified
> against the run worktree and landed INDEPENDENTLY by BOTH v8 falsifiers (falsifier_1's
> decoupling-boundary/decision-table lens AND falsifier_2's carry-forward/regression lens
> converged on the same defect). The revised spec clears the gate **only if it pins M7**
> with the prescribed fix, keeps the boot-path decision table complete (now with ALL
> complete-row cells — 13/15/16 and their `>=21` variants — derived from A's
> fingerprint-sync predicate), **and does not regress
> M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3**; the cycle-1 falsifiers re-attack
> each one specifically. The verbatim prescribed fix is in the `rationale:` field and
> the `- id: M7` `challenge:` field of
> `docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> — read that context doc in full (the M7 finding and the rationale's "Required fix"
> paragraph, §4/§5 if present in the narrative body).

### M7 (LOAD-BEARING, high — decision-table executability / row 16 not derived from A)

**The defect.** The v8 holder closed rows 13 and 15 (the no-revoke `complete` rows) by
propagating the M5 `applied_owner` split through them and deriving those cells from A's
fingerprint-sync predicate. But row 16 (`complete`, `decoupledEnabled=true`,
`revokeEmbedded=true`) is NOT derived from A — it is the IDENTICAL defect class as M6,
one cell over. §3.5 row 16 gives the `==0` and `==20` columns UNCONDITIONAL
`awaiting_deploy` (`HOLDER.md:589`), reasoned '0021 not yet applied on a fresh DB →
fingerprint ≠ → not in-sync → run deploy'. But A's §3.3a step-3 decoupled branch
decides **solely** on `cursor.plan_hash == expected` + `LiveFingerprint ==
ExpectedFingerprint` (`HOLDER.md:441-445`), with **NO `applied_owner` input** — and the
holder's OWN derivation rule (`HOLDER.md:565-566`) says 'where A's outcome is conditional
on the fingerprint-sync state … the cell is written conditionally'. Row 16's
complete/decoupled outcome IS conditional on fingerprint-sync, yet the cell is written
UNCONDITIONALLY — the holder violated its own rule.

The concrete refutation cell (the v8 F18 row-16 refutation): `cursorState=complete`,
`decoupledEnabled=true`, `revokeEmbedded=true`, `applied_owner=0` (or `==20`),
`deploy_plan[plan_hash]` present, `cursor.plan_hash == expected`,
`LiveFingerprint(recorded) == ExpectedFingerprint()`, `owner_bundle_meta` absent (or 20)
— W passes (`owner.go:145,151-153`), A takes the complete+decoupled branch and returns
nil (serve verify-only) because plan-hash and fingerprint are in sync
(`HOLDER.md:441-445`); but §3.5 row 16 `==0`/`==20` requires `awaiting_deploy`. F18
must assert the exact §3.5 outcome (`HOLDER.md:855`), but F18 only adds the
in-sync/out-of-sync sub-dimension for rows 13/15, NOT row 16 (`HOLDER.md:855`). A build
implementing §3.3a serves the in-sync row-16 cell, while a §3.5-following oracle fails
it. This is MATERIAL: the table imports the cross-table premise `owner_bundle_meta < 21
⟹ fingerprint mismatch` — A neither reads nor verifies this premise — precisely the
premise the M6 fix was meant to eliminate.

NOTE: M7 is NOT a SAFETY hole (row 16 is decoupled → the verify path never reaches
the legacy `:399` writer; Invariant B holds) and it does NOT regress any carry-forward
(the M3 bypass stays closed, BC-N2 `==20` and the `>=21` rule intact, `Required` not
advanced) — but it IS a decision-table EXECUTABILITY / COMPLETENESS failure.

**Source anchors (RE-VERIFY against your worktree HEAD; correct line numbers if drifted):**
- `go/pkg/db/owner.go:145` (`if applied == 0 { return nil }` before the `:148-150`
  shortfall), `owner.go:23/:35` (`LatestOwnerBundleVersion = 20` / `RequiredOwnerBundleVersion
  = LatestOwnerBundleVersion` = 20), `owner.go:151-153` (the `==20` nil return).
- `go/pkg/db/schema_drift.go:145-161` (`LiveFingerprint` reads the recorded
  `schema_state.fingerprint` singleton — orthogonal to `owner_bundle_meta`/`applied_owner`).
- `go/pkg/db/schema_drift.go:171-195` (`RecordSchemaFingerprint` writes the binary's
  `ExpectedFingerprint()` — also orthogonal to `owner_bundle_meta`/`applied_owner`).
- `go/pkg/db/connection.go:349/:353/:399` boot order.
- v8 HOLDER.md anchors for row 16: (~`:589`), the A predicate step-3 decoupled branch
  (~`:441-445`), the holder's derivation rule (~`:565-566`), F18 spy list (~`:855`).
  (Read the banked v8 HOLDER.md to confirm exact lines.)

**Prescribed fix (binding — Option 1, the clean fix, verbatim from the v8 ledger
`rationale:` "Required fix" paragraph and the M7 `challenge:` field): make row 16 and
its `>=21` variant CONDITIONAL on the A3 fingerprint predicate and propagate through
§1.3, §3.3a, §3.5, §4.5, and F18.**

1. **MAKE ROW 16 `==0`/`==20` CONDITIONAL on the same A3 complete/decoupled fingerprint
   predicate** — "**SERVE-verify if in-sync, else `awaiting_deploy`**" — exactly as the
   `==20` column now is and exactly as the holder applied to rows 13/15 in v8. Document
   that the normal reachable pre-0021 state (a deploy-pending DB where 0021 has not been
   applied yet) is OUT-OF-SYNC — the `awaiting_deploy` outcome is the dominant real-world
   case; the in-sync subcase is the degenerate cell that can be constructed (and A must
   handle correctly) but is normally unreachable under expected operation.
2. **Write the `>=21` revoke-embedding cell conditional too.** For full derivation:
   `cursorState=complete`, `decoupledEnabled=true`, `revokeEmbedded=true`,
   `applied_owner >= 21` — A's §3.3a decoupled branch still decides on fingerprint-sync
   alone (no `applied_owner` input); the `>=21` column must also be conditional on
   in-sync/out-of-sync, not unconditionally serving. This closes the complete-row class
   fully.
3. **Propagate through §1.3, §3.3a, §3.5, §4.5, and F18.** Every section that
   references the `complete`/decoupled/revoke-embedding serve behavior must reflect the
   conditional — not the prior unconditional `awaiting_deploy`.
4. **Make F18 PARAMETRIC over ALL complete-row cells** (13/15/16 and their `>=21`
   variants) with the in-sync/out-of-sync sub-dimension. F18 currently only adds the
   sub-dimension for rows 13/15; extending it to row 16 and `>=21` closes the class.
   Document within F18 that the normal pre-0021 state is out-of-sync, so the `==0`/`==20`
   and `>=21` in-sync subcases are the degenerate corner cases A must not mishandle.
5. (The v8 ledger also offers Option 2: add an explicit consistency guard
   `complete && revokeEmbedded && applied_owner < 21 → typed inconsistency halt`, or a
   complete-boot stored-transcript DB-stamp verification. Do NOT use Option 2 — Option 1
   is the expected, source-preserving fix parallel to the M6 fix and keeps the design
   coherent with the W→A-independence invariant. Mention Option 2 only as the alternative
   the ledger named.)

> **Preserve the invariants (the v8 ledger's rationale note):** the M7 fix must NOT
> re-collapse the row-1 fresh-DB serve, must NOT weaken the M3 A0 config gate (cells
> 2/6/10/14 must still halt `awaiting_deploy_config` at A0), must NOT regress the M6
> rows-13/15 conditional cells, must NOT regress the BC-N2 `applied_owner == 20` edge,
> and must NOT advance `RequiredOwnerBundleVersion`.

## Proactive completeness — the boot-path decision table (keep it; CLOSE THE CLASS FULLY)

The v8 holder built the complete `cursorState × decoupledEnabled × revokeEmbedded ×
applied_owner` boot-path decision table and derived the no-revoke `complete` rows (13/15)
from A's fingerprint-sync predicate, but did NOT carry the derivation through to the
revoke-embedding `complete` row (16). The v9 revision MUST keep the table and close the
class FULLY across ALL complete-row cells:

- **DERIVE ALL complete-row cells MECHANICALLY from A's fingerprint-sync predicate.**
  The load-bearing invariant from §0.2 carries forward: once W passes, A is
  owner-watermark-independent — `CheckDeployActivation` does NOT read `applied_owner` —
  so for any fixed `(cursorState, decoupledEnabled, revokeEmbedded)` row the `==0` and
  `==20` column outcomes are IDENTICAL everywhere W passes AND conditional on A's own
  predicates (fingerprint-sync for the complete/decoupled branch). **This applies to
  rows 13, 15, 16, and their `>=21` variants — the COMPLETE class must be closed by
  derivation, not by patching individual cells.**
- **Make §3.5 row 16 in the `==0`/`==20` columns CONDITIONAL** — "SERVE-verify if
  in-sync, else `awaiting_deploy`" — derived from the A3 fingerprint predicate, NOT
  from an owner-watermark inference. Make the `>=21` revoke-embedding complete-row cell
  conditional too. Document that the normal pre-0021 state is out-of-sync.
- **Make F18 parametric over ALL complete-row cells** (13/15/16 and `>=21` variants)
  with the in-sync/out-of-sync sub-dimension. F18 must not be a false oracle for ANY
  constructible cell in the complete-row class.
- **Require §4.5 (the Invariant-B per-cell enumeration) and the F18 spy list to remain
  DERIVED FROM / CONSISTENT WITH the table**: the spy list = exactly the cells where the
  table says a legacy/idempotent `:399` write occurs. The M7 fix must not introduce a
  new §4.5↔F18 inconsistency.
- **PROVE §4.5 Universal Invariant B still holds in EVERY cell** AND prove the
  legitimate fresh-DB / inert-landing cells still serve and are NOT wedged. Keep the M3
  cell (`complete` + decoupled OFF + revoke-embedding) and the shadow-mode drift-gate
  fall-through covered, unchanged.
- Make **F18** an **executable, named requirement** the falsifiers can verify against
  the `schema_drift.go:145-161` (`LiveFingerprint` orthogonal to `owner_bundle_meta`)
  and `schema_drift.go:171-195` (`RecordSchemaFingerprint` orthogonal to `owner_bundle_meta`)
  anchors. A re-scaffolded revision that closes row 16 but leaves some `>=21` revoke-embedding
  complete-row cell unconditional, or that introduces a new non-derived cell elsewhere,
  will spawn an M8.

## Root reframe (do not lose this)

**Schema mutation must stop being an implicit side effect of the serving process's
restart and become an explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer.** Then the serving daemon can hold zero DDL
privilege, "restart force-commits a half-applied deploy" becomes impossible, and a
bad migration can never wedge the single writer on boot. **But the fresh /
single-role / no-authority bootstrap (`applied_owner == 0`, no transcript, no revoke)
is the one legitimate serve-legacy cell — over-halting it is the M5 defect (already
resolved at row 1); the reframe must not break the decision table's coherence across
ALL complete-row cells (the M7 defect class).**

## The two Open Questions P4 must pin (from the RFC)

- **Q3 — How atomic is "atomic"?** Confirm the **per-step-atomic + resumable-cursor**
  contract is sufficient for every owner+runtime interleaving we actually ship, or
  specify the small set of steps that need a stricter sub-protocol. Every step must
  be idempotent and leave a coherent intermediate the cursor/transcript classifies
  as "incomplete, resume" — not "unknown drift, panic". **In v9 this explicitly
  includes the full boot-path decision table proving Invariant B everywhere AND the
  decision table coherent across ALL complete-row cells (the M7 fix — row 16 and
  `>=21` variants conditional on A's fingerprint-sync predicate, closing the class
  fully).**
  *(RFC: "This is the hard correctness core of P4.")*
- **Q4 — Should a deploy be a Striatum run?** Plain verb vs. a dogfooded run shape,
  with the bootstrapping paradox. Resolved in v1–v8 — carry forward.

## Load-bearing risks (attack these)

- **R1 atomicity/fingerprint-is-partly-a-lie:** non-transactional DDL, non-idempotent
  steps, a two-connection crash window, the receipt key moving with the frontier (BC-N1,
  fixed), a resume binary that disagrees with an already-applied step (M1, fixed), or a
  `complete`-cursor revoke-embedding binary on the legacy path self-recording around the M1
  gate (M3, fixed). Test: kill-and-resume across each step class; resume with a divergent
  binary; boot a revoke-embedding binary on a `complete` cursor with the flag OFF.
- **R2 decoupling regresses a landed gate:** lifting `ApplyMigrations` must NOT break the
  P2 watermark interlock, the P3 drift gate / self-record, **fresh-DB bring-up (the
  `applied_owner == 0` serve cell — M5, resolved at row 1)**, the BC-N2 universal
  non-complete edge / clean boot, nor leave the legacy `ApplyMigrations` path reachable for
  a deployer-aware binary over a DB carrying a deploy transcript (M3, fixed), nor allow the
  decision table to be incoherent in the complete-row class (M7 — row 16 and `>=21`).
- **R3 DDL-revocation lockout / early REVOKE:** revoking serving-role DDL must not lock out
  the runtime path, nor let any `owner-ddl apply` route commit 0021 early (M2, fixed), nor
  re-open a #512-class lockout via the legacy serve-boot path after the revoke (M3, fixed).
- **R4 cursor / transcript / receipt / fingerprint holes:** double-apply/skip at a commit
  boundary; a stored transcript that disagrees with the binary/DB stamps on resume (M1,
  fixed); out-of-order apply; **a decision-table cell that wedges a legitimate boot or makes
  the executable F18 oracle false (M5 fixed at row 1; M6 fixed at rows 13/15; M7 — the
  complete/decoupled/revoke-embedding row 16 and `>=21` must be conditional on fingerprint-sync,
  closing the class)**; **a decision table that smuggles an owner-watermark-derived fingerprint
  fact into A without A reading it (the M7 class)**.

## Anchor verification against current `main` (operator pre-flight)

Verified against `~/git/striatum` @ `origin/main`. P0–P3 + P2 are **landed**; the
P4 surfaces are **NOT-FOUND (to be built)**. Treat as ground truth; re-anchor the
spec to these file:line references. **Re-confirm against your run's `main` HEAD.**

| Claim / target | Status | Anchor (current source) |
| --- | --- | --- |
| Boot-time auto-apply runs runtime migrations as `striatumd_rw` (the coupling P4 removes) | **ACCURATE** | `go/pkg/db/connection.go` `ConnectAndMigrate` → `ApplyMigrations` `:353`; P3 path / `RecordSchemaFingerprint` `:399`. |
| `ConnectAndMigrate` boot order: watermark → `ApplyMigrations` → `CheckSchemaDrift` → `RecordSchemaFingerprint` (the M3 ordering) | **ACCURATE** | `go/pkg/db/connection.go:349` (`CheckOwnerBundleWatermark`) → `:353` (`ApplyMigrations`) → `:376-383` (`CheckSchemaDrift`, refuse-mode halt) → `:384-393` (shadow-mode log + fall-through) → `:399` (`RecordSchemaFingerprint`, "the only writer of schema_state"). |
| **`CheckOwnerBundleWatermark` SERVES (returns nil) for `applied_owner == 0` BEFORE the shortfall check (the M5 fresh-DB exception)** | **ACCURATE** | `go/pkg/db/owner.go:145` `if applied == 0 { return nil }` — BEFORE the `if applied < RequiredOwnerBundleVersion` shortfall at `:148-150`; the function comment at `:116-123` + `:140-143`: a fresh 0-watermark DB "is treated as the bootstrap/single-role case and NOT halted. Only a database that HAS an authority schema (applied >= 1) but lags the required frontier is a genuine shortfall." |
| **`OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent (the fresh-DB representation, M5)** | **ACCURATE** | `go/pkg/db/owner.go:233-235` — `if present != "true" { return 0, nil }` (the `to_regclass('striatumd.owner_bundle_meta') IS NOT NULL` probe is false). |
| **A fresh migrated DB starts at owner bundle version 0 (the M5 fresh-DB assertion)** | **ACCURATE** | `go/pkg/db/owner_pg_test.go:19` — `if v, err := db.OwnerBundleVersion(...); err != nil || v != 0` (`pre-apply version = %d ...; want 0`). |
| **`LiveFingerprint` reads the recorded `schema_state.fingerprint` singleton — ORTHOGONAL to `owner_bundle_meta`/`applied_owner` (the M6/M7 source anchor showing A is owner-watermark-independent)** | **ACCURATE** | `go/pkg/db/schema_drift.go:145-161` — reads `striatumd.schema_state` singleton row, a SEPARATE table from `owner_bundle_meta`; no `applied_owner` input. |
| **`RecordSchemaFingerprint` writes the binary's `ExpectedFingerprint()` — ORTHOGONAL to `owner_bundle_meta`/`applied_owner` (the M6/M7 source anchor confirming A's owner-watermark independence)** | **ACCURATE** | `go/pkg/db/schema_drift.go:171-195` — UPSERTs `striatumd.schema_state` singleton with `ExpectedFingerprint()`; no read of `owner_bundle_meta`; both tables ORTHOGONAL. |
| `ExpectedFingerprint()` hashes EMBEDDED FILE BYTES | **ACCURATE** | `go/pkg/db/schema_drift.go:83-100` — `MigrationSHASet()` + each `OwnerBundle.SHA256()`, keyed by version. |
| `ApplyOwnerBundles` self-heal falls into `ReapplyAllOwnerBundles` on a cross-bundle dependency error | **ACCURATE** | `go/pkg/db/owner.go:277-302`; `isCrossBundleDependencyError` `:367-374`. M2 binds `isNonRevokeBundle` / `OwnerDDLApplyBundles()` here (resolved). |
| `applyPendingOwnerBundles` bounds `<= current`; `ReapplyAllOwnerBundles` re-runs every loaded bundle | **ACCURATE** | `go/pkg/db/owner.go:309-320` / `:324-349`. M2's filter + in-loop guards bind both (resolved). |
| `OwnerBundles()` shared loader (the binary surfaces 0021 through it for `revokeEmbedded`) | **ACCURATE** | `go/pkg/db/owner.go:196-224`. M2's split keeps `OwnerBundles()` for `revokeEmbedded`/`ExpectedFingerprint`; `OwnerDDLApplyBundles()` excludes 0021 (resolved). |
| Owner bundle frontier = 0020 (`0020_owner_bundle_watermark_read.sql`) — NOT the DDL-revoke bundle | **ACCURATE** | `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 20`, `:35` `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20). **The C3 DDL-revoke bundle stays renumbered at 0021.** |
| Runtime migration frontier (P3 added `schema_state`) | **ACCURATE** | `go/pkg/db/migrations.go` `LatestDaemonDBVersion`. New P4 runtime migration (`deploy_plan`/`deploy_cursor`) is **≥ 0044**. |
| P2 watermark interlock + clean halt | **ACCURATE (landed)** | `go/pkg/db/owner.go` `CheckOwnerBundleWatermark`, called **before** `ApplyMigrations` in boot (`connection.go:349`). |
| `owner-ddl apply` applies bundles out-of-band as owner role | **ACCURATE (landed)** | `go/pkg/cli/localcommands/daemon.go` `runDaemonOwnerDDL` → `db.ApplyOwnerBundles()`. The DDL-revoke bundle is **excluded from every route** (M2). |
| `striatum daemon deploy` / `deploy_cursor` / `deploy_plan` / deploy receipt / `CheckDeployActivation` / `VerifyStoredTranscript` | **NOT-FOUND (P4 builds these)** | `go/pkg/cli/localcommands/daemon.go` has no `deploy`; no `deploy_cursor`/`deploy_plan` table; no activation/verify surface yet. |

**Net design implication.** The P4 ground is clean: M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3
are resolved in v1–v8 and must carry forward unregressed. The remaining hard part is **M7** —
make §3.5 row 16 and its `>=21` variant CONDITIONAL on A's fingerprint-sync predicate,
propagate through §1.3/§3.3a/§3.5/§4.5/F18, and make F18 parametric over ALL complete-row
cells (13/15/16 and `>=21` variants) with the in-sync/out-of-sync sub-dimension. The fix
applies Option 1 (the clean contract parallel to the M6 fix): `complete + decoupled +
revoke-embedding` serves verify-only if in-sync, else `awaiting_deploy` — the normal
pre-0021 deployment-pending state is out-of-sync, making the in-sync subcase a degenerate
corner case that A must handle correctly but which is not the dominant path. Be shadow-first:
the deploy-decoupled path stays default-OFF, additive migrations only, C3 DDL-revoke bundle
stays at 0021, `RequiredOwnerBundleVersion` not advanced.

## Clearing condition (pin this)

The gate clears **only** if the cycle-1 adjudicator's collaboration ledger records a
clearing verdict (`accept` or `accept_with_findings`, never `clear`), which requires
**all** of: **M7 genuinely resolved** (§3.5 row 16 `==0`/`==20` cells CONDITIONAL on the
A3 fingerprint predicate — "SERVE-verify if in-sync, else `awaiting_deploy`"; the `>=21`
revoke-embedding complete-row cell also conditional for full derivation; F18 parametric
over ALL complete-row cells (13/15/16 and `>=21` variants) with the in-sync/out-of-sync
sub-dimension; the normal pre-0021 state documented as out-of-sync; the v8 F18 refutation
cell for row 16 correctly handled: W passes, A returns nil (serve verify-only), §3.5 now
says serve (conditional on in-sync); §1.3/§3.3a/§3.5/§4.5/F18 all updated consistently),
the **proactive-completeness boot-path decision table COMPLETE and executable** (every
`cursorState` × `decoupledEnabled` × `revokeEmbedded` × `applied_owner` cell with a
specified guard/outcome derived from W and A, §4.5 Invariant B proven in every cell incl.
the M3 cell and the shadow-mode fall-through, AND the legitimate fresh-DB cells still
serving), **M6 + M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3 carried
forward intact** (in particular: the M6 rows-13/15 conditional cells not regressed, the M3
config gate not weakened, the BC-N2 `applied_owner == 20` edge not regressed,
`RequiredOwnerBundleVersion = 20` not advanced, the row-1 fresh-DB serve not re-collapsed),
and **no new material challenge standing unrebutted**. A spec that merely *claims* row 16 is
now conditional without the concrete propagation through §1.3/§3.3a/§3.5/§4.5/F18, the
`>=21` variant also conditional, and the F18 parametric extension has NOT cleared the gate.
This is the single allowed revision cycle for v9 — a second `needs_revision` ends the gate
unCleared.

---
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run (v9 /
REVISION of `rfc-0142-p4-design-v8`; resolves the cycle-1 finding M7 — make §3.5 row 16
and its `>=21` variant CONDITIONAL on A's fingerprint-sync predicate; propagate through
§1.3/§3.3a/§3.5/§4.5/F18; make F18 parametric over all complete-row cells (13/15/16 and
`>=21` variants); document normal pre-0021 state as out-of-sync; carries
M6/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 forward). Lanes: author=claude
(holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>

## Operator sharpenings (pin these — do NOT let cycle-1 re-derive them)

- **M7: make row 16 and its `>=21` variant conditional on A's fingerprint-sync predicate;
  make F18 parametric over all complete-row cells.** The defect is that §3.5 row 16
  (`complete`, decoupled ON, revoke-embedding) gives the `==0`/`==20` columns
  UNCONDITIONAL `awaiting_deploy` (`HOLDER.md:589`), but A's §3.3a step-3 decoupled
  branch decides solely on `cursor.plan_hash == expected` + `LiveFingerprint ==
  ExpectedFingerprint` (`HOLDER.md:441-445`), with NO `applied_owner` input. The holder
  VIOLATED its OWN derivation rule (`HOLDER.md:565-566`). Apply Option 1: row 16 `==0`/`==20`
  becomes "SERVE-verify if in-sync, else `awaiting_deploy`" (conditional); the `>=21`
  revoke-embedding complete-row cell also becomes conditional; F18 is extended to be
  parametric over ALL complete-row cells (13/15/16 and `>=21` variants) with the
  in-sync/out-of-sync sub-dimension; §1.3/§3.3a/§3.5/§4.5 are updated consistently.
  Document that the normal pre-0021 state is out-of-sync (the in-sync subcase is the
  degenerate corner A must not mishandle).
- **State M7's fix explicitly in the "Addressing the design-v8 findings" map.** The
  falsifiers will look for the M7 resolution in the same auditable form as the M6
  resolution in v8.
- **Carry forward the §0.2 W→A-independence invariant verbatim.** The M7 fix is the
  next application of the same invariant: once W passes, A is owner-watermark-independent
  in EVERY cell, including the complete/decoupled/revoke-embedding cell. The invariant
  does not change; the fix applies it to row 16 and `>=21`.
- **§4.5 + F18 spy list = mechanically consistent with the table, now also for row 16.**
  After the M7 fix the spy list still enumerates exactly the cells where the table says a
  legacy/idempotent `:399` write occurs: 1/`==0`, 1/`==20`, 13-in-sync/`==0`,
  13-in-sync/`==20`. Row 16 is decoupled — it NEVER reaches the legacy `:399` writer —
  so the spy list does NOT gain new cells from the M7 fix; but F18 must now have a
  parametric sub-dimension for all complete-row in-sync/out-of-sync cases.
- **Carry forward unregressed.** M6's rows-13/15 conditional cells and §0.2 invariant;
  M5(row-1)'s `{0/no authority, 1..19, ==20, >=21}` split and the row-1 fresh-DB serve;
  M3's hoisted config gate + complete-cursor pure-read comparison + tightened Invariant B
  + F17/F11(g)/F18; M4's F16a/F16b phase split; M1's full-transcript verifier; M2's
  non-revoke filter; BC-N1's moving-frontier fix; BC-N2's universal non-complete edge
  (`applied_owner == 20`); the C1 finalizer; the C2 edge (`RequiredOwnerBundleVersion = 20`,
  forward-watermark at `applied >= 21`); and the C3 revoke-last mechanism (0021
  special-cased + terminal) must all stay intact. M7 ONLY closes the complete-row class
  fully; it must NOT re-collapse the row-1 fresh-DB serve, regress the rows-13/15 fix,
  weaken the M3 config gate, regress the BC-N2 `applied_owner == 20` edge, advance
  `Required`, or alter the watermark.

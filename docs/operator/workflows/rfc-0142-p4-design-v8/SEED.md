# Design-Run Seed (v8 / REVISION) — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> **THIS IS THE EIGHTH REVISION (v8).** Seven prior design runs ran the same
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
> the row-1 repair — BUT returned **`needs_revision` again** (the gate's single allowed
> cycle) on **one new finding, source-verified against the run worktree and landed
> INDEPENDENTLY by BOTH v7 falsifiers**:
> **M6 (LOAD-BEARING)** — the M5 `applied_owner` split is NOT propagated coherently
> through the `complete` rows. §3.5 rows 13 and 15 give the `==0` column a different
> outcome than `==20`, but the holder's own §3.3a A predicate (`CheckDeployActivation`)
> does NOT read `applied_owner` — it decides solely on `plan_hash == expected` +
> `LiveFingerprint == ExpectedFingerprint` — so for the same in-sync facts A returns the
> same outcome regardless of the bucket. This makes the executable F18 matrix a FALSE
> ORACLE for the in-sync `complete`/`applied_owner==0` cells, OR forces the build to
> smuggle in an unstated `applied_owner`-dependent complete-cursor guard that contradicts
> the stated "identical A behavior" claim. The holder's own §4.5 compounds the
> contradiction: it admits the degenerate 13/`==0`-in-sync idempotent `:399` rewrite while
> the F18 spy list omits that cell — §4.5 and the F18 oracle disagree. This v8 run is a
> **proper revision**: the holder starts from the **v7** `HOLDER.md` (a required context
> doc), REVISES the spec to **resolve M6** per the prescribed fix below (propagate the M5
> split coherently through the `complete` rows, derive the full table mechanically from W
> and A), keeps the proactive-completeness boot-path decision table, and **carries
> M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 forward unregressed**; the falsifiers
> re-attack the revised spec. The v7 design record —
> `dialogue/holder/HOLDER.md`, `dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0142-p4-design-v7/`; the **v7** `HOLDER.md` (the revised
> spec) and the **v7** collaboration ledger (the verdict + the full M6 analysis and §4
> "What the revision must fix") are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries the
> charter, restates the two Open Questions P4 must pin (Q3 + Q4), pins the **single
> binding revision constraint M6**, states what already cleared
> (M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3) and must NOT be
> reopened, names the proactive-completeness boot-path decision table the holder MUST
> keep (now with the FULL 64-cell table derived mechanically from W and A), and pins an
> operator anchor-verification table you must build on. Read this whole file, the **v7**
> `HOLDER.md` + the **v7** collaboration ledger context docs, and the RFC (esp. the
> Phasing table P4 row and the "Open Questions" section) before producing any artifact.

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
5. **Resolve the single binding revision constraint (M6) below**, per its
   prescribed fix, KEEP the **proactive-completeness boot-path decision table** (now
   with the FULL 64-cell table derived MECHANICALLY from W and A), and **carry forward
   M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3 unregressed** (see
   "Carried forward"). A revised spec that leaves M6 open — or that regresses any
   carry-forward finding, or whose decision table has a cell where Universal Invariant B
   fails OR where the F18 oracle disagrees with the §3.3a A predicate — has NOT cleared
   the gate. This is the gate's single allowed revision cycle for v8, so a second
   `needs_revision` ends the gate unCleared.

## Carried forward — resolved by v7 (do NOT reopen)

> The v7 collaboration ledger records M5 row-1 as genuinely resolved **this cycle**
> (both v7 falsifiers explicitly concede the row-1 repair), and M3, M4, M1, M2, BC-N1,
> BC-N2, C1, C2, and C3 as intact and unregressed (both falsifiers concede each
> carry-forward). The v8 revision MUST preserve them — verbatim from the **v7**
> `HOLDER.md` — and the cycle-1 adjudicator's clearing verdict requires them intact. Do
> NOT re-litigate any of them; they are not this cycle's open question. M6 is a
> propagation of the M5 `applied_owner` split through the `complete` rows (so the table
> is coherent everywhere W passes and A is owner-watermark-independent), NOT a
> re-opening of the M5 row-1 fix, the M3 config gate, the M4 F16 split, or any of the
> eight earlier fixes. The M6 fix must NOT re-collapse the resolved row-1 fresh-DB serve.

### M5 — RESOLVED (row-1) by v7 (the owner-watermark dimension split). Preserve verbatim from the v7 HOLDER.

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
(`T-deploy-fresh-db-bootstrap-serves`) pins the fresh-DB serve. **The M6 fix only
propagates the split through the `complete` rows; it must NOT re-collapse the resolved
row-1 fresh-DB serve, must NOT weaken the M3 config gate (cells 2/6/10/14 must still
halt `awaiting_deploy_config` at A0 in EVERY column that passes W incl. `0`), and must
NOT regress the BC-N2 `applied_owner == 20` edge.**

### M3 — RESOLVED by v6 (the COMPLETE-cursor activation guard). Preserve verbatim from the v7 HOLDER.

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
tightened Invariant B, and F17/F11(g)/F18. **The M6 fix propagates the split through the
complete rows; it must NOT weaken the M3 config gate — the revoke-embedding + flag-OFF halt
stays conservative.**

### M4 — RESOLVED by v6 (F16 phase split). Preserve verbatim from the v7 HOLDER.

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

### M1 — INTACT (full-transcript byte + DB-stamp verification). Preserve verbatim from the v7 HOLDER.

A single **`VerifyStoredTranscript(plan_hash)` (§3.4a)** that checks the **ENTIRE**
stored transcript — every step, already-applied AND not-yet-applied — `sha256` against
the running binary's embedded bytes (reusing `MigrationSHASet()` and `OwnerBundle.SHA256()`);
**ANY** mismatch ⇒ `deploy_plan_binary_mismatch`; for **already-applied** entries it ALSO
verifies the **DATABASE STAMPS** (`schema_migrations.sha256` / `owner_bundle_meta.sha256`)
⇒ `deploy_plan_db_stamp_mismatch`; the **SAME** verifier runs on **every resume** AND as
**finalizer step 0** before the `complete` receipt / `RecordSchemaFingerprint` /
`finalizing → complete` advance; **F15** + extended **F14**. M3 EXTENDS Invariant B's
enforcement to the legacy/complete-cursor path — it does NOT re-open or gate the deployer
verifier. Preserve all of it.

### M2 — INTACT (DDL-revoke bundle excluded from every owner-ddl apply route). Preserve verbatim from the v7 HOLDER.

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

### BC-N1 — INTACT (immutable plan identity across resume). Preserve verbatim from the v7 HOLDER.

The runtime-owned **`deploy_plan` table** (migration 0044) storing the IMMUTABLE ordered
transcript keyed by `plan_hash`, materialized in the same tx that sets
`deploy_cursor → in_progress(0)` BEFORE step 0; **resume loads
`deploy_plan[cursor.plan_hash]`** and never recomputes `BuildPlan` over the moved frontier;
the **§1.3** stored-transcript reclassification; the transcript-enumerated doctor; **F14**.
Preserve all of it.

### BC-N2 — INTACT (universal pre-revoke serve edge). Preserve verbatim from the v7 HOLDER.

`CheckDeployActivation` is not gated on `revokeEmbedded`; every deployer-aware binary reads
`deploy_cursor` and halts **`awaiting_deploy` DB-untouched on a non-`complete` cursor** at
**`applied_owner == 20`**; F11(e)/(f) + extended `G-old-binary-refuse`. **M3's
complete-cursor extension sits ABOVE and BESIDE the BC-N2 non-complete edge — it does NOT
weaken it. M5/M6 concern the ORTHOGONAL owner-watermark `applied_owner` dimension at W
(`CheckOwnerBundleWatermark`), NOT the BC-N2 `deploy_cursor` edge at A — the resolved
`applied_owner == 20` BC-N2 edge must NOT be regressed by the M6 fix.** Preserve the
universal `revokeEmbedded`-independent edge, the `applied_owner == 20` halt, F11(e)/(f), and
the extended `G-old-binary-refuse`.

### C1 — INTACT (finalization boundary). Preserve verbatim from the v7 HOLDER.

The distinct **`finalizing`** `deploy_cursor` state; an **idempotent finalizer** (guarded
`complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST), gated by
M1's pre-finalizer `VerifyStoredTranscript` step 0; the **§1.3 row** classifying
`finalizing` as resumable finalization; and **F10**. Preserve all of it.

### C2 — INTACT at the fail-closed EDGE. Preserve verbatim from the v7 HOLDER.

**`CheckDeployActivation`** called immediately after `CheckOwnerBundleWatermark` and
**BEFORE `ApplyMigrations`** in both boot paths, mutating nothing; the typed
non-restartable halts **`awaiting_deploy`** and **`awaiting_deploy_config`**; the
**forward-watermark rule** firing at **`applied >= 21`**; **`RequiredOwnerBundleVersion`
KEPT at its current-`main` value 20** — NOT advanced to the revoke ordinal 21. M3 EXTENDS
this edge to the `complete`-cursor case without advancing `Required` or altering the
watermark. **The M6 fix propagates the M5 split through the complete rows; it must NOT
advance `Required`, alter the watermark, or change the `applied >= 21` forward rule.**
Preserve all of it, including F11.

### C3 — INTACT (ownership transfer, revoke-last). Preserve verbatim from the v7 HOLDER.

The DDL-revoke bundle (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`), re-anchored
0020→**0021**, identified by `DDLRevokeOwnerBundleVersion = 21`, **special-cased OUT of the
owner prefix** and **EXCLUDED from `owner-ddl apply`**, applied as the **TERMINAL plan step**
after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while
`striatumd_rw` still holds CREATE granted by 0018; the committed steady state denies CREATE);
F12 (`T-deploy-runtime-object-ownership`) in a non-superuser two-role cluster +
`G-revoke-last`. Preserve the design mechanism, F12, and `G-revoke-last`.

### Other carry-forward (v1–v7 ledgers — do NOT re-litigate)

- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`), bootstrapping
  paradox dispositive, run-shape door held by three seams. No falsifier contested it.
- **Q3 mid-step resumability body — sound.** Q3-A (cursor advance + receipt in the
  step transaction), Q3-B (`in_progress` pre-marker + invalid-index reconciler), the
  §1.1 step taxonomy, the `deploy_cursor`-as-second-signal insight. M3/M5/M6 are about
  serve-boot guarding and the boot-path decision table, not the per-step schema/cursor
  body — preserve the body, close the decision table coherently.
- **Decoupling shape — sound (now M3-hardened).** Shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark interlock +
  P3 drift gate / self-record, the embed-FS-derived plan, the hash-chained per-step
  receipt, `doctor schema_deploy_unrecorded`.
- **Scope discipline held.** P5 deferred; full capability revocation named as a
  follow-up beyond P4, not assumed; the local-first boundary respected.

## The single binding constraint v8 MUST resolve (the v7 adjudicator's needs_revision ground)

> **This is BINDING.** The design-v7 falsification gate returned `needs_revision`
> because a material new falsifier challenge landed unrebutted (**M6**), source-verified
> against the run worktree and landed INDEPENDENTLY by BOTH v7 falsifiers (falsifier_1's
> decoupling-boundary/decision-table lens AND falsifier_2's carry-forward/regression lens
> converged on the same defect). The revised spec clears the gate **only if it pins M6**
> with the prescribed fix, keeps the boot-path decision table complete (now with the full
> 64-cell table derived mechanically from W and A), **and does not regress
> M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3**; the cycle-1 falsifiers re-attack each
> one specifically. The verbatim prescribed fix is §4 "What the revision must fix" in
> `docs/operator/artifacts/rfc-0142-p4-design-v7/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> — read that context doc in full (its `findings:` M6 entry and §3 + §4 + §5).

### M6 (LOAD-BEARING, high — decision-table executability / the M5 split not propagated through the `complete` rows)

**The defect.** The v7 holder split the `applied_owner` dimension into `{0/no authority,
1..19 shortfall, ==20, >=21}` and fixed the fresh-DB serve at row 1 (`cursorState=none`).
But the split is NOT propagated through the `complete` rows. The holder's own A predicate
(`CheckDeployActivation`, §3.3a) does NOT read `applied_owner` — it decides solely on
`plan_hash == expected` + `LiveFingerprint == ExpectedFingerprint` (`schema_drift.go:145-161`
reads the recorded `schema_state.fingerprint` singleton — orthogonal to
`owner_bundle_meta`/`applied_owner`; `schema_drift.go:171-195` writes the binary's
`ExpectedFingerprint()` — also orthogonal). Yet §3.5 row 13 (`complete`, flag off,
no-revoke) gives the `==0` column `awaiting_deploy` while `==20` is "SERVE-legacy if
in-sync, else `awaiting_deploy`"; row 15 (`complete`, decoupled on, no-revoke) gives `==0`
an unconditional `awaiting_deploy` while `==20` is "SERVE-verify if in-sync, else
`awaiting_deploy`". For an in-sync `complete`/`applied_owner==0` cell (cursor=complete,
plan_hash match, fingerprint match, `owner_bundle_meta` absent — constructible) A returns
*serve*, and §1.3 says serve, but §3.5 says `awaiting_deploy` — so **F18 (the executable
matrix asserting the exact §3.5 outcome for all 64 cells) is a FALSE ORACLE** for those
cells, OR the build must smuggle an unstated `applied_owner`-dependent complete-cursor guard
contradicting the "A does not read applied_owner" claim. Worse, §4.5 ADMITS the degenerate
13/`==0`-in-sync idempotent `:399` rewrite while the F18 spy list permits
`RecordSchemaFingerprint` only in 1/`==0`, 1/`==20`, 13/`==20`-in-sync (omitting
13/`==0`-in-sync) — §4.5 and the F18 oracle disagree. NOT a safety hole (Invariant B holds
— only idempotent rewrites of the identical fingerprint) and NOT a carry-forward regression;
it is a decision-table **executability/coherence** defect.

**Source anchors (RE-VERIFY against your worktree HEAD; correct line numbers if drifted):**
- `go/pkg/db/owner.go:145` (`if applied == 0 { return nil }` before the `:148-150`
  shortfall), `owner.go:23/:35` (`LatestOwnerBundleVersion = 20` / `RequiredOwnerBundleVersion
  = LatestOwnerBundleVersion` = 20).
- `go/pkg/db/schema_drift.go:145-161` (`LiveFingerprint` reads the recorded
  `schema_state.fingerprint` singleton — orthogonal to `owner_bundle_meta`/`applied_owner`).
- `go/pkg/db/schema_drift.go:171-195` (`RecordSchemaFingerprint` writes the binary's
  `ExpectedFingerprint()` — also orthogonal to `owner_bundle_meta`/`applied_owner`).
- `go/pkg/db/connection.go:349/:353/:399` boot order.
- v7 HOLDER.md anchors for the table: rows 13/15 (~`:501`,`:503`), the A predicate
  (~`:359-381`, `:471-475`), §1.3 (~`:223-225`), §4.5 degenerate subcase (~`:525-528`),
  F18 spy list (~`:723`). (Read the banked v7 HOLDER.md to confirm exact lines.)

**Prescribed fix (binding — verbatim from the v7 ledger §4): propagate the M5 split
coherently through the `complete` rows so §3.5, §3.3a, §1.3, §4.5, and F18/F18a ALL agree.
Adopt contract (1) (the expected fix):**

1. **Mirror `==20` anywhere W passes and A is owner-watermark independent.** Make §3.5 rows
   13 and 15 in the `==0` column conditional — "**serve if in-sync, else `awaiting_deploy`**"
   — exactly as the `==20` column. Add the degenerate 13/`==0`-in-sync idempotent `:399`
   rewrite to the §4.5 Universal-Invariant-B enumeration AND to the F18 spy list. This keeps
   the holder's "A is owner-watermark independent → identical A behavior for the `0` and
   `==20` columns" claim TRUE.
2. (The v7 ledger also offers contract (2): classify `complete + applied_owner == 0` as
   inconsistent and halt a typed error before serving. Do NOT use (2) — (1) is the expected,
   source-preserving fix and keeps the design coherent. Mention (2) only as the alternative
   the ledger named.)

> **Preserve the asymmetry (the v7 ledger's §4 note):** the M6 fix is local to the
> `complete` rows; it must NOT re-collapse the resolved row-1 fresh-DB serve, must NOT
> weaken the M3 config gate (cells 2/6/10/14 must still halt `awaiting_deploy_config` at A0
> in every column that passes W incl. `0`), must NOT regress the BC-N2 `applied_owner == 20`
> edge, and must NOT advance `RequiredOwnerBundleVersion`.

## Proactive completeness — the boot-path decision table (keep it; CLOSE THE CLASS)

The v7 holder built the COMPLETE `cursorState × decoupledEnabled × revokeEmbedded ×
applied_owner` boot-path decision table with the M5 `applied_owner` dimension split, but did
NOT propagate the split coherently through the `complete` rows (M6). The v8 revision MUST
KEEP the table and close the class:

- **DERIVE the entire 64-cell table MECHANICALLY from the two predicates W
  (`CheckOwnerBundleWatermark`) and A (`CheckDeployActivation`).** State the load-bearing
  invariant explicitly: **once W passes (`applied_owner ∈ {0, ==20, >=21-as-barrier}`), A is
  owner-watermark-independent — `CheckDeployActivation` does NOT read `applied_owner` — so
  the `==0` and `==20` columns have IDENTICAL A-gate outcomes in EVERY cursor row (none,
  in_progress, finalizing, complete), not only row 1.** This mechanical derivation is the
  foundation: if A does not read `applied_owner`, then for any fixed `(cursorState,
  decoupledEnabled, revokeEmbedded)` row, the `==0` and `==20` column outcomes must match.
  Apply this in EVERY cursor row — none, in_progress, finalizing, complete — not only
  `cursorState=none`.
- **Propagate the split through the `complete` rows so §3.5 rows 13 and 15 in the `==0`
  column are conditional — "serve if in-sync, else `awaiting_deploy`" — exactly as `==20`.**
  Add the degenerate 13/`==0`-in-sync idempotent `:399` rewrite to §4.5 AND to the F18 spy
  list (consistency between the Invariant-B proof and the oracle).
- **Require §4.5 (the Invariant-B per-cell enumeration) and the F18 spy list to be DERIVED
  FROM / CONSISTENT WITH the table**: the spy list = exactly the cells where the table says a
  legacy/idempotent `:399` write occurs. The v7 contradiction (§4.5 admits 13/`==0`-in-sync
  but F18 forbids it) must be eliminated.
- **Demand an audit of ALL cursor rows (none/in_progress/finalizing/complete) × the `==0`
  column for propagation coherence.** The v7 failure was that the row-1 fix was not
  propagated; the v8 revision must explicitly walk the `in_progress` and `finalizing` rows'
  `==0` cells too and confirm they match the `==20` column for the same reason (A does not
  read `applied_owner`).
- **PROVE §4.5 Universal Invariant B still holds in EVERY cell** AND prove the legitimate
  fresh-DB / inert-landing cells still serve and are NOT wedged. Keep the M3 cell (`complete`
  + decoupled OFF + revoke-embedding) and the shadow-mode drift-gate fall-through covered,
  unchanged.
- Make **F18** and **F18a** **executable, named requirements** the falsifiers can verify
  against the `schema_drift.go:145-161` (`LiveFingerprint` orthogonal to `owner_bundle_meta`)
  and `schema_drift.go:171-195` (`RecordSchemaFingerprint` orthogonal to `owner_bundle_meta`)
  anchors — confirming A is genuinely owner-watermark-independent. **A re-scaffolded revision
  that fixes the `complete` rows but leaves the `in_progress` or `finalizing` rows incoherent
  will spawn an M7 — close the class fully across ALL cursor rows.**

## Root reframe (do not lose this)

**Schema mutation must stop being an implicit side effect of the serving process's
restart and become an explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer.** Then the serving daemon can hold zero DDL
privilege, "restart force-commits a half-applied deploy" becomes impossible, and a
bad migration can never wedge the single writer on boot. **But the fresh /
single-role / no-authority bootstrap (`applied_owner == 0`, no transcript, no revoke)
is the one legitimate serve-legacy cell — over-halting it is the M5 defect (already
resolved at row 1); the reframe must not break the decision table's coherence across all
cursor rows (the M6 defect).**

## The two Open Questions P4 must pin (from the RFC)

- **Q3 — How atomic is "atomic"?** Confirm the **per-step-atomic + resumable-cursor**
  contract is sufficient for every owner+runtime interleaving we actually ship, or
  specify the small set of steps that need a stricter sub-protocol. Every step must
  be idempotent and leave a coherent intermediate the cursor/transcript classifies
  as "incomplete, resume" — not "unknown drift, panic". **In v8 this explicitly
  includes the full boot-path decision table proving Invariant B everywhere AND the
  decision table coherent across ALL cursor rows (the M6 fix — A is
  owner-watermark-independent in EVERY row, not only row 1).**
  *(RFC: "This is the hard correctness core of P4.")*
- **Q4 — Should a deploy be a Striatum run?** Plain verb vs. a dogfooded run shape,
  with the bootstrapping paradox. Resolved in v1–v7 — carry forward.

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
  decision table to be incoherent across cursor rows (M6).
- **R3 DDL-revocation lockout / early REVOKE:** revoking serving-role DDL must not lock out
  the runtime path, nor let any `owner-ddl apply` route commit 0021 early (M2, fixed), nor
  re-open a #512-class lockout via the legacy serve-boot path after the revoke (M3, fixed).
- **R4 cursor / transcript / receipt / fingerprint holes:** double-apply/skip at a commit
  boundary; a stored transcript that disagrees with the binary/DB stamps on resume (M1,
  fixed); out-of-order apply; **a decision-table cell that wedges a legitimate boot or makes
  the executable F18 oracle false (M5 fixed at row 1; M6 — complete rows must be coherent)**;
  **a decision table that is mechanically inconsistent because A is treated as
  owner-watermark-dependent in some rows but independent in others (the M6 class)**.

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
| **`LiveFingerprint` reads the recorded `schema_state.fingerprint` singleton — ORTHOGONAL to `owner_bundle_meta`/`applied_owner` (the M6 source anchor showing A is owner-watermark-independent)** | **ACCURATE** | `go/pkg/db/schema_drift.go:145-161` — reads `striatumd.schema_state` singleton row, a SEPARATE table from `owner_bundle_meta`; no `applied_owner` input. |
| **`RecordSchemaFingerprint` writes the binary's `ExpectedFingerprint()` — ORTHOGONAL to `owner_bundle_meta`/`applied_owner` (the M6 source anchor confirming A's owner-watermark independence)** | **ACCURATE** | `go/pkg/db/schema_drift.go:171-195` — UPSERTs `striatumd.schema_state` singleton with `ExpectedFingerprint()`; no read of `owner_bundle_meta`; both tables ORTHOGONAL. |
| `ExpectedFingerprint()` hashes EMBEDDED FILE BYTES | **ACCURATE** | `go/pkg/db/schema_drift.go:83-100` — `MigrationSHASet()` + each `OwnerBundle.SHA256()`, keyed by version. |
| `ApplyOwnerBundles` self-heal falls into `ReapplyAllOwnerBundles` on a cross-bundle dependency error | **ACCURATE** | `go/pkg/db/owner.go:277-302`; `isCrossBundleDependencyError` `:367-374`. M2 binds `isNonRevokeBundle` / `OwnerDDLApplyBundles()` here (resolved). |
| `applyPendingOwnerBundles` bounds `<= current`; `ReapplyAllOwnerBundles` re-runs every loaded bundle | **ACCURATE** | `go/pkg/db/owner.go:309-320` / `:324-349`. M2's filter + in-loop guards bind both (resolved). |
| `OwnerBundles()` shared loader (the binary surfaces 0021 through it for `revokeEmbedded`) | **ACCURATE** | `go/pkg/db/owner.go:196-224`. M2's split keeps `OwnerBundles()` for `revokeEmbedded`/`ExpectedFingerprint`; `OwnerDDLApplyBundles()` excludes 0021 (resolved). |
| Owner bundle frontier = 0020 (`0020_owner_bundle_watermark_read.sql`) — NOT the DDL-revoke bundle | **ACCURATE** | `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 20`, `:35` `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20). **The C3 DDL-revoke bundle stays renumbered at 0021.** |
| Runtime migration frontier (P3 added `schema_state`) | **ACCURATE** | `go/pkg/db/migrations.go` `LatestDaemonDBVersion`. New P4 runtime migration (`deploy_plan`/`deploy_cursor`) is **≥ 0044**. |
| P2 watermark interlock + clean halt | **ACCURATE (landed)** | `go/pkg/db/owner.go` `CheckOwnerBundleWatermark`, called **before** `ApplyMigrations` in boot (`connection.go:349`). |
| `owner-ddl apply` applies bundles out-of-band as owner role | **ACCURATE (landed)** | `go/pkg/cli/localcommands/daemon.go` `runDaemonOwnerDDL` → `db.ApplyOwnerBundles()`. The DDL-revoke bundle is **excluded from every route** (M2). |
| `striatum daemon deploy` / `deploy_cursor` / `deploy_plan` / deploy receipt / `CheckDeployActivation` / `VerifyStoredTranscript` | **NOT-FOUND (P4 builds these)** | `go/pkg/cli/localcommands/daemon.go` has no `deploy`; no `deploy_cursor`/`deploy_plan` table; no activation/verify surface yet. |

**Net design implication.** The P4 ground is clean: M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3
are resolved in v1–v7 and must carry forward unregressed. The remaining hard part is **M6** —
propagate the M5 `applied_owner` split coherently through the `complete` rows by deriving the
entire 64-cell table mechanically from W and A: **once W passes, A is
owner-watermark-independent (confirmed by `schema_drift.go:145-161` and `schema_drift.go:171-195`
— both orthogonal to `owner_bundle_meta`), so the `==0` and `==20` columns have IDENTICAL
A-gate outcomes in EVERY cursor row**. Apply contract (1): rows 13 and 15 in the `==0` column
become "serve if in-sync, else `awaiting_deploy`" exactly as `==20`; add the degenerate
13/`==0`-in-sync subcase to §4.5 AND to the F18 spy list; audit ALL cursor rows × `==0` for
the same propagation coherence. Be shadow-first: serve-boot auto-apply stays the default for a
no-revoke inert binary on a clean DB (the `applied_owner == 0` cell serves), but a
revoke-embedding binary with the flag OFF over a deploy transcript must halt, never auto-apply
(the M3 gate, unchanged).

## Clearing condition (pin this)

The gate clears **only** if the cycle-1 adjudicator's collaboration ledger records a
clearing verdict (`accept` or `accept_with_findings`, never `clear`), which requires
**all** of: **M6 genuinely resolved** (the M5 `applied_owner` split propagated coherently
through ALL cursor rows, not only `cursorState=none`; the entire 64-cell table derived
mechanically from W and A; the `==0` and `==20` columns having IDENTICAL A-gate outcomes in
EVERY cursor row since A does not read `applied_owner`; §3.5 rows 13 and 15 in the `==0`
column conditional — "serve if in-sync, else `awaiting_deploy`" — matching `==20`; the
degenerate 13/`==0`-in-sync idempotent `:399` rewrite added to BOTH §4.5 AND the F18 spy
list; ALL cursor rows × `==0` column audited for propagation coherence so the `in_progress`
and `finalizing` rows do not spawn an M7), the **proactive-completeness boot-path decision
table COMPLETE and executable** (every `cursorState` × `decoupledEnabled` × `revokeEmbedded`
× `applied_owner` cell with a specified guard/outcome derived from W and A, §4.5 Invariant B
proven in every cell incl. the M3 cell and the shadow-mode fall-through, AND the legitimate
fresh-DB cells still serving), **M5(row-1) + M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 +
C3 carried forward intact** (in particular: the M3 config gate not weakened, the BC-N2
`applied_owner == 20` edge not regressed, `RequiredOwnerBundleVersion = 20` not advanced, the
row-1 fresh-DB serve not re-collapsed), and **no new material challenge standing unrebutted**.
A spec that merely *claims* the split is coherent without the concrete mechanical derivation,
the §4.5/F18 consistency fix, and the explicit cross-row audit has NOT cleared the gate. This
is the single allowed revision cycle for v8 — a second `needs_revision` ends the gate
unCleared.

---
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run (v8 /
REVISION of `rfc-0142-p4-design-v7`; resolves the cycle-1 finding M6 — propagate the M5
applied_owner split through the complete rows; mirror ==20 since A is owner-watermark-independent;
make §3.5/§3.3a/§1.3/§4.5/F18 consistent and derive the 64-cell table mechanically; carries
M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 forward). Lanes: author=claude
(holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>

## Operator sharpenings (pin these — do NOT let cycle-1 re-derive them)

- **M6: propagate the M5 split through the complete rows; derive the table mechanically from W
  and A.** The defect is that §3.5 rows 13/15 give the `==0` column a DIFFERENT outcome than
  `==20` (`HOLDER.md:501,503`), but A = `CheckDeployActivation` does NOT read `applied_owner`
  (`HOLDER.md:471-475`; source-confirmed: `schema_drift.go:145-161` reads
  `schema_state.fingerprint` — a SEPARATE table from `owner_bundle_meta`; `schema_drift.go:171-195`
  writes `ExpectedFingerprint()` — also orthogonal). So once W passes, A's behavior is
  identical for `==0` and `==20` in EVERY cursor row. Apply contract (1): rows 13 and 15 `==0`
  become "serve if in-sync, else `awaiting_deploy`" exactly as `==20`; add the degenerate
  13/`==0` in-sync idempotent `:399` rewrite to §4.5 AND to the F18 spy list; confirm the
  `in_progress` and `finalizing` rows' `==0` cells also match `==20` for the same reason.
- **State the load-bearing mechanical-derivation invariant explicitly:** "once W passes
  (`applied_owner ∈ {0, ==20, >=21-as-barrier}`), A is owner-watermark-independent, so the
  `==0` and `==20` columns have IDENTICAL A-gate outcomes in EVERY cursor row". Derive the
  entire 64-cell table FROM this invariant; do not assert cells ad-hoc.
- **§4.5 + F18 spy list = mechanically consistent with the table.** The spy list = exactly
  the cells where the table says a legacy/idempotent `:399` write occurs. After the M6 fix the
  cells reaching the legacy writer are: 1/`==0` (fresh-DB serve), 1/`==20` (inert-landing),
  13-in-sync/`==0` (the new degenerate subcase — idempotent `:399` rewrite of identical
  fingerprint), and 13-in-sync/`==20`. §4.5 and the F18 spy list must both enumerate all four.
- **Audit ALL cursor rows × `==0` column explicitly** in the spec so the falsifiers can verify
  none/in_progress/finalizing/complete are all coherent. The v7 failure pattern (fix the named
  cell, leave a sibling cell unexamined) repeats as M5→M6→M7; the v8 revision must CLOSE the
  class by derivation, not by patching individual cells.
- **Carry forward unregressed.** M5(row-1)'s `{0/no authority, 1..19, ==20, >=21}` split and
  the row-1 fresh-DB serve (cell 1/`==0` serves, cell 1/`==20` inert-landing, F18/F18a dual
  assertion); M3's hoisted config gate + complete-cursor pure-read comparison + tightened
  Invariant B + F17/F11(g)/F18; M4's F16a/F16b phase split; M1's full-transcript verifier;
  M2's non-revoke filter; BC-N1's moving-frontier fix; BC-N2's universal non-complete edge
  (`applied_owner == 20`); the C1 finalizer; the C2 edge (`RequiredOwnerBundleVersion = 20`,
  forward-watermark at `applied >= 21`); and the C3 revoke-last mechanism (0021 special-cased
  + terminal) must all stay intact. M6 ONLY propagates the M5 split through the complete rows;
  it must NOT re-collapse the row-1 fresh-DB serve, weaken the M3 config gate, regress the
  BC-N2 `applied_owner == 20` edge, advance `Required`, or alter the watermark.

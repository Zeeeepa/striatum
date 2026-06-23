# Design-Run Seed (v7 / REVISION) — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> **THIS IS THE SEVENTH REVISION (v7).** Six prior design runs ran the same
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
> intact — BUT returned **`needs_revision` again** (the gate's single allowed cycle) on
> **one new finding, source-verified against current `main` and landed independently by
> BOTH v6 falsifiers**:
> **M5 (LOAD-BEARING)** — the §3.5 / F18 proactive-completeness boot-path decision table
> collapses the owner-watermark dimension: it maps `applied_owner < 20 → awaiting_owner_ddl`
> UNIFORMLY (states the `<20` column "ALWAYS halts at W") and the prose mislabels cell
> `==20` as the "fresh-DB bring-up" cell. But current source `CheckOwnerBundleWatermark`
> SERVES (returns nil) for `applied_owner == 0` — the fresh / single-role / no-authority
> bootstrap case — BEFORE the shortfall check, and halts `awaiting_owner_ddl` ONLY for
> `1 <= applied < 20`. So the executable F18 table either (1) WEDGES a legitimate fresh
> no-authority boot the SEED requires to "still serve and NOT be wedged", or (2) makes the
> F18 oracle FALSE for the `applied_owner == 0` cell. This v7 run is a **proper revision**:
> the holder starts from the **v6** `HOLDER.md` (a required context doc), REVISES the spec
> to **resolve M5** per the prescribed fix below (the `applied_owner` dimension split),
> keeps the proactive-completeness boot-path decision table, and **carries
> M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 forward unregressed**; the falsifiers re-attack the
> revised spec. The v6 design record —
> `dialogue/holder/HOLDER.md`, `dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0142-p4-design-v6/`; the **v6** `HOLDER.md` (the revised
> spec) and the **v6** collaboration ledger (the verdict + the full M5 analysis and §4
> "What the revision must fix") are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries the
> charter, restates the two Open Questions P4 must pin (Q3 + Q4), pins the **single
> binding revision constraint M5**, states what already cleared
> (M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3) and must NOT be reopened, names the
> proactive-completeness boot-path decision table the holder MUST keep (now with the
> `applied_owner` dimension SPLIT), and pins an operator anchor-verification table you
> must build on. Read this whole file, the **v6** `HOLDER.md` + the **v6** collaboration
> ledger context docs, and the RFC (esp. the Phasing table P4 row and the "Open
> Questions" section) before producing any artifact.

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
5. **Resolve the single binding revision constraint (M5) below**, per its
   prescribed fix, KEEP the **proactive-completeness boot-path decision table** (now
   with the `applied_owner` dimension split), and **carry forward
   M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3 unregressed** (see "Carried
   forward"). A revised spec that leaves M5 open — or that regresses any
   carry-forward finding, or whose decision table has a cell where Universal Invariant B
   fails OR where a legitimate fresh-DB cell is wedged — has NOT cleared the gate. This
   is the gate's single allowed revision cycle for v7, so a second `needs_revision`
   ends the gate unCleared.

## Carried forward — resolved by v6 (do NOT reopen)

> The v6 collaboration ledger records M3 and M4 as genuinely resolved **this cycle**
> (both v6 falsifiers explicitly concede each), and M1, M2, BC-N1, BC-N2, C1, C2, and C3
> as intact and unregressed (both falsifiers concede each carry-forward). The v7 revision
> MUST preserve them — verbatim from the **v6** `HOLDER.md` — and the cycle-1 adjudicator's
> clearing verdict requires them intact. Do NOT re-litigate any of them; they are not this
> cycle's open question. M5 is a *re-bucketing* of the `applied_owner` dimension of the
> §3.5/F18 decision table (so the table matches the live bootstrap contract), NOT a
> re-opening of the M3 config gate, the M3 complete-cursor pure-read comparison, the M4
> F16 split, or any of the seven earlier fixes. The M5 fix must NOT weaken the M3 halt.

### M3 — RESOLVED by v6 (the COMPLETE-cursor activation guard). Preserve verbatim from the v6 HOLDER.

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
  `cursor.plan_hash == expected`) that serves only when already in-sync (so legacy
  `ApplyMigrations` is a no-op and `:399` rewrites the IDENTICAL fingerprint), else halts
  `awaiting_deploy` DB-untouched;
- **Universal Invariant B TIGHTENED** so the legacy `connection.go:399` writer is reachable
  ONLY in §3.5 cells 1 (`none`, no transcript) and 13-in-sync (`complete`, no-revoke,
  already byte-in-sync), barred from any revoke-embedded, revoke-applied, or
  pending-change-over-a-transcript state;
- **F17** (`T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`),
  **F11(g)**, and **F18** (`T-deploy-bootpath-decision-table`).
Preserve the hoisted step-0 config gate, the no-revoke `complete` pure-read comparison, the
tightened Invariant B, and F17/F11(g)/F18. **The M5 fix only re-buckets the `applied_owner`
dimension; it must NOT weaken the M3 config gate — the revoke-embedding + flag-OFF halt
stays conservative.**

### M4 — RESOLVED by v6 (F16 phase split). Preserve verbatim from the v6 HOLDER.

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

### M1 — INTACT (full-transcript byte + DB-stamp verification). Preserve verbatim from the v6 HOLDER.

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

### M2 — INTACT (DDL-revoke bundle excluded from every owner-ddl apply route). Preserve verbatim from the v6 HOLDER.

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

### BC-N1 — INTACT (immutable plan identity across resume). Preserve verbatim from the v6 HOLDER.

The runtime-owned **`deploy_plan` table** (migration 0044) storing the IMMUTABLE ordered
transcript keyed by `plan_hash`, materialized in the same tx that sets
`deploy_cursor → in_progress(0)` BEFORE step 0; **resume loads
`deploy_plan[cursor.plan_hash]`** and never recomputes `BuildPlan` over the moved frontier;
the **§1.3** stored-transcript reclassification; the transcript-enumerated doctor; **F14**.
Preserve all of it.

### BC-N2 — INTACT (universal pre-revoke serve edge). Preserve verbatim from the v6 HOLDER.

`CheckDeployActivation` is not gated on `revokeEmbedded`; every deployer-aware binary reads
`deploy_cursor` and halts **`awaiting_deploy` DB-untouched on a non-`complete` cursor** at
**`applied_owner == 20`**; F11(e)/(f) + extended `G-old-binary-refuse`. **M3's
complete-cursor extension sits ABOVE and BESIDE the BC-N2 non-complete edge — it does NOT
weaken it. M5 concerns the ORTHOGONAL owner-watermark `applied_owner` dimension at W
(`CheckOwnerBundleWatermark`), NOT the BC-N2 `deploy_cursor` edge at A — the resolved
`applied_owner == 20` BC-N2 edge must NOT be regressed by the M5 split.** Preserve the
universal `revokeEmbedded`-independent edge, the `applied_owner == 20` halt, F11(e)/(f), and
the extended `G-old-binary-refuse`.

### C1 — INTACT (finalization boundary). Preserve verbatim from the v6 HOLDER.

The distinct **`finalizing`** `deploy_cursor` state; an **idempotent finalizer** (guarded
`complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST), gated by
M1's pre-finalizer `VerifyStoredTranscript` step 0; the **§1.3 row** classifying
`finalizing` as resumable finalization; and **F10**. Preserve all of it.

### C2 — INTACT at the fail-closed EDGE. Preserve verbatim from the v6 HOLDER.

**`CheckDeployActivation`** called immediately after `CheckOwnerBundleWatermark` and
**BEFORE `ApplyMigrations`** in both boot paths, mutating nothing; the typed
non-restartable halts **`awaiting_deploy`** and **`awaiting_deploy_config`**; the
**forward-watermark rule** firing at **`applied >= 21`**; **`RequiredOwnerBundleVersion`
KEPT at its current-`main` value 20** — NOT advanced to the revoke ordinal 21. M3 EXTENDS
this edge to the `complete`-cursor case without advancing `Required` or altering the
watermark. **The M5 fix re-buckets the `applied_owner` dimension of the decision table; it
must NOT advance `Required`, alter the watermark, or change the `applied >= 21` forward
rule.** Preserve all of it, including F11.

### C3 — INTACT (ownership transfer, revoke-last). Preserve verbatim from the v6 HOLDER.

The DDL-revoke bundle (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`), re-anchored
0020→**0021**, identified by `DDLRevokeOwnerBundleVersion = 21`, **special-cased OUT of the
owner prefix** and **EXCLUDED from `owner-ddl apply`**, applied as the **TERMINAL plan step**
after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while
`striatumd_rw` still holds CREATE granted by 0018; the committed steady state denies CREATE);
F12 (`T-deploy-runtime-object-ownership`) in a non-superuser two-role cluster +
`G-revoke-last`. Preserve the design mechanism, F12, and `G-revoke-last`.

### Other carry-forward (v1–v6 ledgers — do NOT re-litigate)

- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`), bootstrapping
  paradox dispositive, run-shape door held by three seams. No falsifier contested it.
- **Q3 mid-step resumability body — sound.** Q3-A (cursor advance + receipt in the
  step transaction), Q3-B (`in_progress` pre-marker + invalid-index reconciler), the
  §1.1 step taxonomy, the `deploy_cursor`-as-second-signal insight. M3/M5 are about
  serve-boot guarding and the boot-path decision table, not the per-step schema/cursor
  body — preserve the body, harden the boot-path table's owner-watermark dimension.
- **Decoupling shape — sound (now M3-hardened).** Shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark interlock +
  P3 drift gate / self-record, the embed-FS-derived plan, the hash-chained per-step
  receipt, `doctor schema_deploy_unrecorded`.
- **Scope discipline held.** P5 deferred; full capability revocation named as a
  follow-up beyond P4, not assumed; the local-first boundary respected.

## The single binding constraint v7 MUST resolve (the v6 adjudicator's needs_revision ground)

> **This is BINDING.** The design-v6 falsification gate returned `needs_revision`
> because a material new falsifier challenge landed unrebutted (**M5**), source-verified
> against current `main` and landed INDEPENDENTLY by BOTH v6 falsifiers (falsifier_1's
> decoupling-boundary lens AND falsifier_2's owner-ddl/test-staging lens converged on the
> same defect). The revised spec clears the gate **only if it pins M5** with the prescribed
> fix, keeps the boot-path decision table complete (now with the split), **and does not
> regress M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3**; the cycle-1 falsifiers re-attack each one
> specifically. The verbatim prescribed fix is §4 "What the revision must fix" in
> `docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> — read that context doc in full (its `findings:` M5 entry and §3 + §4 + §5).

### M5 (LOAD-BEARING, high — decision-table completeness / owner-watermark dimension collapse)

**The defect.** The §3.5 / F18 proactive-completeness boot-path decision table maps
`applied_owner < 20 → awaiting_owner_ddl` UNIFORMLY (states the `<20` column "ALWAYS halts
at W"; v6 `HOLDER.md:443-459,461-478`), and the prose mislabels cell `==20` as the
"fresh-DB bring-up" cell (v6 `HOLDER.md:515-518`). But current source
`CheckOwnerBundleWatermark` SERVES (returns nil) for `applied_owner == 0` — the fresh /
single-role / no-authority bootstrap case — BEFORE the shortfall check, and halts
`awaiting_owner_ddl` ONLY for `1 <= applied < 20`.

**Reproducer (the cell the SEED pinned).** `cursorState=none`, `decoupledEnabled=false`,
`revokeEmbedded=false`, `applied_owner=0` — under current source the watermark returns nil
and §3.3a step 4 returns nil, so legacy `ConnectAndMigrate` performs the normal fresh-DB
bring-up. Under the v6 §3.5/F18 table the same cell is row 1/`<20` and must return
`awaiting_owner_ddl`. **Harm:** an implementation following F18 literally either (1) **WEDGES**
a legitimate fresh no-authority boot (the SEED requires it to "still serve and NOT be
wedged"), regressing fresh / single-role bootstrap; OR (2) preserves the `applied == 0`
exception, making the **EXECUTABLE F18 oracle FALSE** for the `applied_owner == 0` cell.
Either branch is a material table-correctness failure.

This is **NOT** a re-opening of M3: the hoisted M3 gate is right for revoke-embedding
binaries, and the fresh-DB cell carries no transcript and no revoke, so Invariant B is not
*violated* there. The M5 failure is the OPPOSITE — an over-conservative halt of a cell that
must serve. It is exactly the failure class the SEED's v6 proactive-hardening section warned
an unaudited cell would spawn ("A re-scaffolded revision that pins exactly the two §4 items
but leaves an unaudited boot-path combination open will simply spawn an M5") — and it did.

**Source anchors (RE-VERIFY against your run's `main` HEAD and correct line numbers if
drifted):**
- `go/pkg/db/owner.go` — `CheckOwnerBundleWatermark` returns nil for `applied == 0` BEFORE
  the `if applied < RequiredOwnerBundleVersion` shortfall check (the `if applied == 0`
  return nil at `:145`, then the `if applied < RequiredOwnerBundleVersion` shortfall at
  `:148-150`); the explicit comment block at `:116-123` and `:140-143`: a fresh 0-watermark
  DB "is treated as the bootstrap/single-role case and NOT halted. Only a database that HAS
  an authority schema (applied >= 1) but lags the required frontier is a genuine shortfall."
- `go/pkg/db/owner.go` — `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent
  (the `if present != "true" { return 0, nil }` branch at `:233-235`).
- `go/pkg/db/owner_pg_test.go:19` — asserts a fresh migrated DB starts at owner bundle
  version 0 (`pre-apply version = %d ... want 0`).
- `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 20`, `:35`
  `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20).
- v6 HOLDER.md: the §3.5 `<20 → awaiting_owner_ddl` rule (~`:443-478`), the
  `==20`-as-"fresh-DB" mislabel (~`:515-518`), the F18 coverage requirement (~`:666`). (Read
  the banked v6 HOLDER.md to confirm exact lines.)

**Prescribed fix (binding, falsifiable — verbatim from the v6 ledger §4):**

1. In §3.5 and F18, replace the single `applied_owner < 20` bucket with
   `applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}`.
2. Specify the no-transcript / no-revoke / flag-off bootstrap cell (`cursorState=none`,
   `decoupledEnabled=false`, `revokeEmbedded=false`, `applied_owner=0`) as
   **serve-legacy / fresh bootstrap**: `ApplyMigrations` and the legacy `connection.go:399`
   self-record MAY run because **NO deploy transcript exists** (Universal Invariant B is NOT
   in scope), exactly matching `CheckOwnerBundleWatermark`'s `applied == 0` exception. Retain
   `awaiting_owner_ddl` (DB untouched) for `1 <= applied_owner < 20`.
3. Propagate the split through the other cursor rows so the table stays executable, and make
   **F18** assert **BOTH** branches explicitly (the `applied_owner == 0` serve cell AND the
   `1..19` halt cell), so the matrix oracle matches source without changing the bootstrap
   contract.
4. Stop labeling cell `==20` the "fresh-DB bring-up" cell; the genuine fresh no-authority
   DB is `applied_owner == 0`. (If the spec instead deliberately wants to drop the
   `applied == 0` bootstrap exception, that is a product behavior change that must be called
   out as such and reconciled with the SEED's "must still serve" requirement — but the
   conservative, source-preserving split above is the expected fix.)

> **Preserve the asymmetry (the v6 ledger's §4 note):** the M3 halt is conservative ON
> PURPOSE for a revoke-embedding binary (decoupling becomes mandatory once the binary embeds
> 0021), but the watermark `<20` halt must **NOT** be conservative for `applied_owner == 0`
> — that cell is a legitimate fresh serve, and over-halting it is the M5 defect. The M5 fix
> only re-buckets the `applied_owner` dimension; it must NOT weaken the M3 config gate.

## Proactive completeness — the boot-path decision table (keep it; close the M5 dimension)

The v6 holder built the COMPLETE `cursorState × decoupledEnabled × revokeEmbedded ×
applied_owner` boot-path decision table and proved Universal Invariant B in every specified
cell — but its `applied_owner` dimension only had `{<20, ==20, >=21}` buckets, and the `<20`
bucket collapsed the fresh `applied_owner == 0` serve cell into the `1..19` shortfall halt
(M5). The v7 revision MUST KEEP the table and close that dimension:

- Split the `applied_owner` column into `{0/no authority, 1..19 authority shortfall, ==20,
  >=21}` and respecify every cursor row's owner-watermark behavior accordingly. The
  `applied_owner == 0` no-transcript/no-revoke/flag-off bootstrap cell **serves-legacy**
  (the legacy `:399` self-record is legitimate there — no deploy transcript exists, so
  Invariant B is not in scope); `1 <= applied_owner < 20` retains `awaiting_owner_ddl`
  DB-untouched.
- **PROVE §4.5 Universal Invariant B still holds in EVERY cell** — *NO fingerprint /
  `schema_state` `complete` is written unless the FULL stored transcript byte-matches the
  binary* — AND prove the legitimate fresh-DB / inert-landing cells (`applied_owner == 0`,
  no-revoke, no transcript) **STILL SERVE and are NOT wedged**. Keep the M3 cell
  (`complete` + decoupled OFF + revoke-embedding + a pending change) and the shadow-mode
  drift-gate fall-through covered, unchanged.
- Make **F18** (`T-deploy-bootpath-decision-table`) an **executable, named requirement** the
  falsifiers can verify against `CheckOwnerBundleWatermark`'s `applied == 0` exception
  (`owner.go:145`) and the `connection.go:353/:376-383/:399` ordering — asserting BOTH the
  `applied_owner == 0` serve cell AND the `1..19` halt cell explicitly. **A re-scaffolded
  revision that pins M5 but leaves any further unaudited cell open will spawn an M6 — close
  the class.**

## Root reframe (do not lose this)

**Schema mutation must stop being an implicit side effect of the serving process's
restart and become an explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer.** Then the serving daemon can hold zero DDL
privilege, "restart force-commits a half-applied deploy" becomes impossible, and a
bad migration can never wedge the single writer on boot. **But the fresh /
single-role / no-authority bootstrap (`applied_owner == 0`, no transcript, no revoke)
is the one legitimate serve-legacy cell — over-halting it is the M5 defect; the reframe
must NOT wedge a healthy first boot.**

## The two Open Questions P4 must pin (from the RFC)

- **Q3 — How atomic is "atomic"?** Confirm the **per-step-atomic + resumable-cursor**
  contract is sufficient for every owner+runtime interleaving we actually ship, or
  specify the small set of steps that need a stricter sub-protocol. Every step must
  be idempotent and leave a coherent intermediate the cursor/transcript classifies
  as "incomplete, resume" — not "unknown drift, panic". **In v7 this explicitly
  includes the full boot-path decision table proving Invariant B everywhere AND the
  legitimate fresh-DB cells still serving (the M5 owner-watermark dimension split).**
  *(RFC: "This is the hard correctness core of P4.")*
- **Q4 — Should a deploy be a Striatum run?** Plain verb vs. a dogfooded run shape,
  with the bootstrapping paradox. Resolved in v1–v6 — carry forward.

## Load-bearing risks (attack these)

- **R1 atomicity/fingerprint-is-partly-a-lie:** non-transactional DDL, non-idempotent
  steps, a two-connection crash window, the receipt key moving with the frontier (BC-N1,
  fixed), a resume binary that disagrees with an already-applied step (M1, fixed), or a
  `complete`-cursor revoke-embedding binary on the legacy path self-recording around the M1
  gate (M3, fixed). Test: kill-and-resume across each step class; resume with a divergent
  binary; boot a revoke-embedding binary on a `complete` cursor with the flag OFF.
- **R2 decoupling regresses a landed gate:** lifting `ApplyMigrations` must NOT break the
  P2 watermark interlock, the P3 drift gate / self-record, **fresh-DB bring-up (the
  `applied_owner == 0` serve cell — M5)**, the BC-N2 universal non-complete edge / clean
  boot, nor leave the legacy `ApplyMigrations` path reachable for a deployer-aware binary
  over a DB carrying a deploy transcript (M3, fixed).
- **R3 DDL-revocation lockout / early REVOKE:** revoking serving-role DDL must not lock out
  the runtime path, nor let any `owner-ddl apply` route commit 0021 early (M2, fixed), nor
  re-open a #512-class lockout via the legacy serve-boot path after the revoke (M3, fixed).
- **R4 cursor / transcript / receipt / fingerprint holes:** double-apply/skip at a commit
  boundary; a stored transcript that disagrees with the binary/DB stamps on resume (M1,
  fixed); out-of-order apply; **a decision-table cell that wedges a legitimate boot or makes
  the executable F18 oracle false (M5)**.

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
| `ExpectedFingerprint()` hashes EMBEDDED FILE BYTES | **ACCURATE** | `go/pkg/db/schema_drift.go:83-100` — `MigrationSHASet()` + each `OwnerBundle.SHA256()`, keyed by version. |
| `LiveFingerprint` reads the self-recorded singleton — NO recompute | **ACCURATE** | `go/pkg/db/schema_drift.go:145-160`; `RecordSchemaFingerprint` UPSERTs the running binary's expected value `:171-194`; `CheckSchemaDrift` compares the singleton to this binary `:239-274`. |
| `ApplyOwnerBundles` self-heal falls into `ReapplyAllOwnerBundles` on a cross-bundle dependency error | **ACCURATE** | `go/pkg/db/owner.go:277-302`; `isCrossBundleDependencyError` `:367-374`. M2 binds `isNonRevokeBundle` / `OwnerDDLApplyBundles()` here (resolved). |
| `applyPendingOwnerBundles` bounds `<= current`; `ReapplyAllOwnerBundles` re-runs every loaded bundle | **ACCURATE** | `go/pkg/db/owner.go:309-320` / `:324-349`. M2's filter + in-loop guards bind both (resolved). |
| `OwnerBundles()` shared loader (the binary surfaces 0021 through it for `revokeEmbedded`) | **ACCURATE** | `go/pkg/db/owner.go:196-224`. M2's split keeps `OwnerBundles()` for `revokeEmbedded`/`ExpectedFingerprint`; `OwnerDDLApplyBundles()` excludes 0021 (resolved). |
| Owner bundle frontier = 0020 (`0020_owner_bundle_watermark_read.sql`) — NOT the DDL-revoke bundle | **ACCURATE** | `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 20`, `:35` `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20). **The C3 DDL-revoke bundle stays renumbered at 0021.** |
| Runtime migration frontier (P3 added `schema_state`) | **ACCURATE** | `go/pkg/db/migrations.go` `LatestDaemonDBVersion`. New P4 runtime migration (`deploy_plan`/`deploy_cursor`) is **≥ 0044**. |
| P2 watermark interlock + clean halt | **ACCURATE (landed)** | `go/pkg/db/owner.go` `CheckOwnerBundleWatermark`, called **before** `ApplyMigrations` in boot (`connection.go:349`). |
| `owner-ddl apply` applies bundles out-of-band as owner role | **ACCURATE (landed)** | `go/pkg/cli/localcommands/daemon.go` `runDaemonOwnerDDL` → `db.ApplyOwnerBundles()`. The DDL-revoke bundle is **excluded from every route** (M2). |
| `striatum daemon deploy` / `deploy_cursor` / `deploy_plan` / deploy receipt / `CheckDeployActivation` / `VerifyStoredTranscript` | **NOT-FOUND (P4 builds these)** | `go/pkg/cli/localcommands/daemon.go` has no `deploy`; no `deploy_cursor`/`deploy_plan` table; no activation/verify surface yet. |

**Net design implication.** The P4 ground is clean: M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 are
resolved in v1–v6 and must carry forward unregressed. The remaining hard part is **M5** —
split the §3.5/F18 decision table's `applied_owner` dimension into `{0/no authority, 1..19
authority shortfall, ==20, >=21}` so the fresh / single-role / no-authority bootstrap cell
(`applied_owner == 0`, no transcript, no revoke) **serves-legacy** (matching
`CheckOwnerBundleWatermark`'s `applied == 0` exception at `owner.go:145`) while `1..19`
retains `awaiting_owner_ddl`, make F18 assert BOTH cells, and stop mislabeling cell `==20` as
the fresh-DB cell. Be shadow-first: serve-boot auto-apply stays the default for a no-revoke
inert binary on a clean DB (the `applied_owner == 0` cell serves), but a revoke-embedding
binary with the flag OFF over a deploy transcript must halt, never auto-apply (the M3 gate,
unchanged).

## Clearing condition (pin this)

The gate clears **only** if the cycle-1 adjudicator's collaboration ledger records a
clearing verdict (`accept` or `accept_with_findings`, never `clear`), which requires
**all** of: **M5 genuinely resolved** (the §3.5/F18 `applied_owner` dimension SPLIT into
`{0/no authority, 1..19 authority shortfall, ==20, >=21}`; F18 asserts BOTH the
`applied_owner == 0` serve cell — `ApplyMigrations`/legacy self-record may run because no
deploy transcript exists, matching `CheckOwnerBundleWatermark`'s `applied == 0` exception at
`owner.go:145` — AND the `1 <= applied_owner < 20` `awaiting_owner_ddl` halt cell; the split
propagated through the other cursor rows so the table stays executable; cell `==20` no
longer mislabeled the "fresh-DB bring-up" cell), the **proactive-completeness boot-path
decision table COMPLETE** (every `cursorState` × `decoupledEnabled` × `revokeEmbedded` ×
`applied_owner` cell with a specified guard/outcome, Invariant B proven in every cell incl.
the M3 cell and the shadow-mode fall-through, AND the legitimate fresh-DB / inert-landing
cells still serving — not wedged), **M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3
carried forward intact** (in particular: the M3 config gate not weakened, the BC-N2
`applied_owner == 20` edge not regressed, `RequiredOwnerBundleVersion = 20` not advanced),
and **no new material challenge standing unrebutted**. A spec that merely *claims* the split
without the concrete `applied_owner == 0` serve cell and the F18 dual-cell assertion has NOT
cleared the gate. This is the single allowed revision cycle for v7 — a second
`needs_revision` ends the gate unCleared.

---
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run (v7 /
REVISION of `rfc-0142-p4-design-v6`; resolves the cycle-1 finding M5 — the owner-watermark
dimension split — and carries M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3 forward). Lanes: author=claude
(holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>

## Operator sharpenings (pin these — do NOT let cycle-1 re-derive them)

- **M5: split the owner-watermark dimension; serve `applied_owner == 0`.** The defect is
  that §3.5/F18 maps `applied_owner < 20 → awaiting_owner_ddl` UNIFORMLY ("ALWAYS halts at
  W"; v6 `HOLDER.md:443-459,461-478`) and labels cell `==20` the "fresh-DB" cell (v6
  `HOLDER.md:515-518`), but current source `CheckOwnerBundleWatermark` returns nil (SERVES)
  for `applied == 0` BEFORE the shortfall check (`owner.go:145`, the `applied == 0` return
  precedes the `applied < RequiredOwnerBundleVersion` halt at `:148-150`; comment at
  `:116-123` + `:140-143`), and `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is
  absent (`owner.go:233-235`; `owner_pg_test.go:19` asserts version 0 on a fresh DB). The
  fix is to re-bucket `applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20,
  >=21}`, specify `applied_owner == 0` (no transcript, no revoke, flag OFF) as serve-legacy
  / fresh bootstrap (legacy `:399` self-record legitimate — Invariant B not in scope),
  retain `awaiting_owner_ddl` for `1..19`, propagate the split through the other cursor
  rows, make F18 assert BOTH cells, and stop labeling cell `==20` fresh. The reproducer
  cycle-1 falsifiers MUST re-run: boot a no-revoke binary (flag OFF) over a fresh
  `applied_owner == 0` DB with `cursorState=none` and prove it SERVES (fresh bring-up), and
  separately prove `1 <= applied < 20` halts `awaiting_owner_ddl`.
- **Build the decision table ONCE, exhaustively, with the split dimension.** Cover every
  `cursorState` × `decoupledEnabled` × `revokeEmbedded` × `applied_owner ∈ {0, 1..19, ==20,
  >=21}` cell and prove Invariant B AND the fresh-DB serve in each — so cycle-1 cannot spawn
  an M6 from an unaudited combination.
- **Carry forward unregressed.** M3's hoisted config gate + complete-cursor pure-read
  comparison + tightened Invariant B + F17/F11(g)/F18, M4's F16a/F16b phase split, M1's
  full-transcript verifier, M2's non-revoke filter, BC-N1's moving-frontier fix, BC-N2's
  universal non-complete edge (`applied_owner == 20`), the C1 finalizer, the C2 edge
  (`RequiredOwnerBundleVersion = 20`, forward-watermark at `applied >= 21`), and the C3
  revoke-last mechanism (0021 special-cased + terminal) must all stay intact. M5 ONLY
  re-buckets the `applied_owner` dimension of the decision table; it must NOT weaken the M3
  config gate, regress the BC-N2 `applied_owner == 20` edge, advance `Required`, or alter
  the watermark.

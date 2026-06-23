# Design-Run Seed (v6 / REVISION) — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> **THIS IS THE SIXTH REVISION (v6).** Five prior design runs ran the same
> falsification gate. v1 (`rfc-0142-p4-design`) returned **`needs_revision`** with
> three findings C1/C2/C3. v2 (`rfc-0142-p4-design-v2`) **resolved C1 and C2** but
> returned `needs_revision` again (C3 still open + a new finding N1). v3
> (`rfc-0142-p4-design-v3`) **resolved C3** (the revoke-last ownership mechanism; both
> falsifiers conceded it) and closed the immediate N1 hole, BUT returned
> `needs_revision` on two NEW grounds BC-N1 + BC-N2. v4 (`rfc-0142-p4-design-v4`)
> **resolved BOTH BC-N1 and BC-N2** but returned `needs_revision` on two independent
> NEW material challenges M1 + M2. v5 (`rfc-0142-p4-design-v5`) **resolved BOTH M1 and
> M2** — both v5 falsifiers AND the v5 adjudicator explicitly concede each, the v4
> reproducers no longer reproduce in the deployer resume/finalizer path, and
> BC-N1/BC-N2/C1/C2/C3 carried forward intact — BUT returned **`needs_revision` again**
> (the gate's single allowed cycle) on **two new findings**, the load-bearing one
> source-verified against current `main`:
> **M3 (LOAD-BEARING)** — the COMPLETE-cursor window lets the LEGACY `ConnectAndMigrate`
> path mutate + self-record AROUND the M1 `VerifyStoredTranscript` gate, regressing the
> serve-boot decoupling boundary and re-opening a #512-class lockout — and
> **M4 (secondary)** — the F16 test contract asserts production `OwnerBundles()` contains
> 0021 in rollout step 2 but 0021 isn't authored until rollout step 7, so it cannot build
> green as written. This v6 run is a **proper revision**: the holder starts from the
> **v5** `HOLDER.md` (a required context doc), REVISES the spec to **resolve M3 + M4**
> per their prescribed fixes below, produces the proactive-completeness boot-path
> decision table, and **carries M1/M2/BC-N1/BC-N2/C1/C2/C3 forward unregressed**; the
> falsifiers re-attack the revised spec. The v5 design record —
> `dialogue/holder/HOLDER.md`, `dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0142-p4-design-v5/`; the **v5** `HOLDER.md` (the revised
> spec) and the **v5** collaboration ledger (the verdict + the full M3/M4 analysis and §4
> "What the revision must fix") are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries the
> charter, restates the two Open Questions P4 must pin (Q3 + Q4), pins the two binding
> revision constraints M3 + M4, states what already cleared
> (M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3) and must NOT be reopened, names the
> proactive-completeness boot-path decision table the holder MUST produce, and pins an
> operator anchor-verification table you must build on. Read this whole file, the **v5**
> `HOLDER.md` + the **v5** collaboration ledger context docs, and the RFC (esp. the
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
5. **Resolve BOTH binding revision constraints (M3, M4) below**, each per its
   prescribed fix, produce the **proactive-completeness boot-path decision table**, and
   **carry forward M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3 unregressed** (see "Carried
   forward"). A revised spec that leaves M3 or M4 open — or that regresses any
   carry-forward finding, or whose decision table has a cell where Universal Invariant B
   fails — has NOT cleared the gate. This is the gate's single allowed revision cycle for
   v6, so a second `needs_revision` ends the gate unCleared.

## Carried forward — resolved by v5 (do NOT reopen)

> The v5 collaboration ledger records M1 and M2 as genuinely resolved **this cycle**
> (both v5 falsifiers explicitly concede the v4 reproducers no longer reproduce in the
> deployer resume/finalizer path), and BC-N1, BC-N2, C1, C2, and C3 as intact and
> unregressed (both falsifiers concede each carry-forward). The v6 revision MUST preserve
> them — verbatim from the **v5** `HOLDER.md` — and the cycle-1 adjudicator's clearing
> verdict requires them intact. Do NOT re-litigate any of them; they are not this cycle's
> open questions. M3 is an *extension* of `CheckDeployActivation`'s fail-closed edge to
> the `complete`-cursor branch (and a tightening of Universal Invariant B), not a
> re-opening of the M1 deployer-path verifier or the BC-N2 non-complete edge; M4 only
> restructures F16's **staging**, not the M2 non-revoke filter.

### M1 — RESOLVED this cycle (full-transcript byte + DB-stamp verification). Preserve verbatim from the v5 HOLDER.

The v4 narrowing (the binary check covered only not-yet-applied steps, so a resume
binary whose already-applied bytes differ self-recorded a hybrid as in-sync) is closed by:
- a single **`VerifyStoredTranscript(plan_hash)` (§3.4a)** that checks the **ENTIRE**
  stored transcript — every step, already-applied AND not-yet-applied — `sha256` against
  the running binary's embedded bytes (reusing `MigrationSHASet()` and
  `OwnerBundle.SHA256()` — the exact byte-hashes `ExpectedFingerprint()` composes,
  `schema_drift.go:83-100`); **ANY** mismatch ⇒ `deploy_plan_binary_mismatch` (typed halt,
  DB-untouched, apply/finalize nothing);
- for **already-applied** entries it ALSO verifies the **DATABASE STAMPS**
  (`schema_migrations.sha256` for runtime, `owner_bundle_meta.sha256` for owner) match
  the stored transcript ⇒ `deploy_plan_db_stamp_mismatch` (refuse-to-finalize);
- the **SAME** verifier runs on **every resume** AND as **finalizer step 0** BEFORE the
  `complete` receipt / `RecordSchemaFingerprint` / `finalizing → complete` advance, so a
  binary that disagrees with an already-applied step can NEVER self-record its
  `ExpectedFingerprint()` as in-sync through the DEPLOYER finalizer;
- **F15** (new) covers the A45/B45 case, the symmetric owner-step case, the DB-stamp case,
  and the finalizer arm; **F14** extended with an already-applied tamper case; the doctor
  adds the stamp/byte-divergence WARN.
Preserve `VerifyStoredTranscript`, the resume + finalizer-step-0 gating, the typed
`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` halts, F15, and the
extended F14. **M3 EXTENDS Invariant B's enforcement to the legacy/complete-cursor path —
it does NOT re-open the deployer verifier.**

### M2 — RESOLVED this cycle (DDL-revoke bundle excluded from every owner-ddl apply route). Preserve verbatim from the v5 HOLDER.

The v4 escape (the FMA-007 self-heal `ReapplyAllOwnerBundles` re-running every loaded
bundle could commit 0021 early) is closed by:
- a single non-revoke filter **`isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)`**
  and a split loader **`OwnerDDLApplyBundles()` = `filter(OwnerBundles(), isNonRevokeBundle)`**
  as the ONLY bundle slice every `owner-ddl apply` route iterates: `ApplyOwnerBundles`
  loads the filtered slice; both `applyPendingOwnerBundles` AND the FMA-007 self-heal
  `ReapplyAllOwnerBundles` additionally guard `isNonRevokeBundle` in-loop
  (defense-in-depth); the `ReapplyAllOwnerBundles(nil, …)` fallback loads the filtered
  slice;
- the **embed/listing split**: `OwnerBundles()` (full, includes 0021) is kept ONLY for
  `revokeEmbedded` detection, `ExpectedFingerprint`, the deploy plan's `BuildPlan`, and
  `RuntimeOwnedTablesAlterable()` — so "the binary embeds 0021" does NOT imply
  "`owner-ddl apply` iterates 0021"; 0021's REVOKE can ONLY ever be committed as the
  terminal `deploy` step (via `applyOneOwnerBundle`), NEVER via a self-heal reapply;
- **F16** (new) forces the cross-bundle self-heal and asserts 0021 not applied /
  `owner_bundle_meta` never records 21 / CREATE held; F12/`G-revoke-last` extended with
  the owner-ddl side-path.
Preserve `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, the in-loop guards on both apply
loops, the nil-fallback split, the embed/listing split, and F16's safety assertions.
**M4 only restructures F16's STAGING, not the filter.**

### BC-N1 — INTACT (immutable plan identity across resume). Preserve verbatim from the v5 HOLDER.

The runtime-owned **`deploy_plan` table** (migration 0044) storing the IMMUTABLE ordered
transcript keyed by `plan_hash`, materialized in the same tx that sets
`deploy_cursor → in_progress(0)` BEFORE step 0; **resume loads
`deploy_plan[cursor.plan_hash]`** and never recomputes `BuildPlan` over the moved frontier;
the **§1.3** stored-transcript reclassification; the transcript-enumerated doctor; **F14**.
M1 EXTENDS the binary-verification clause; it does NOT re-open the moving-frontier
mechanism. Preserve all of it.

### BC-N2 — INTACT (universal pre-revoke serve edge). Preserve verbatim from the v5 HOLDER.

`CheckDeployActivation` is not gated on `revokeEmbedded`; every deployer-aware binary reads
`deploy_cursor` after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations`/
`RecordSchemaFingerprint` in both boot paths and halts **`awaiting_deploy` DB-untouched on
a non-`complete` cursor** at **`applied_owner == 20`**; F11(e)/(f) + extended
`G-old-binary-refuse`. **M3 is the ORTHOGONAL `complete`-cursor window, NOT the BC-N2
non-complete window — the M3 fix must NOT weaken the non-complete edge.** Preserve the
universal `revokeEmbedded`-independent edge, the `applied_owner == 20` halt, F11(e)/(f),
and the extended `G-old-binary-refuse`.

### C1 — INTACT (finalization boundary). Preserve verbatim from the v5 HOLDER.

The distinct **`finalizing`** `deploy_cursor` state; an **idempotent finalizer** (guarded
`complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST); the
**§1.3 row** classifying `finalizing` as resumable finalization; the operator pin that no
single finalization transaction spans owner+runtime; and **F10**. M1's fix ADDS a
pre-finalizer `VerifyStoredTranscript` step 0; it does NOT change the finalizer's boundary
semantics. **NOTE:** M3's harm manifests through the LEGACY `connection.go:399` self-record
— a DIFFERENT writer than the C1 deployer finalizer (which IS gated). M3 tightens Invariant
B to cover the legacy writer; it does not touch the C1 finalizer.

### C2 — INTACT at the fail-closed EDGE. Preserve verbatim from the v5 HOLDER.

**`CheckDeployActivation`** called immediately after `CheckOwnerBundleWatermark` and
**BEFORE `ApplyMigrations`** in both boot paths, mutating nothing; the typed
non-restartable halts **`awaiting_deploy`** and **`awaiting_deploy_config`**; the
**forward-watermark rule** firing at **`applied >= 21`**; **`RequiredOwnerBundleVersion`
KEPT at its current-`main` value 20** — NOT advanced to the revoke ordinal 21. **The M3 fix
EXTENDS this edge to the `complete`-cursor case — it must NOT advance `Required` or alter
the watermark.** Preserve all of it, including F11.

### C3 — INTACT (ownership transfer, revoke-last). Preserve verbatim from the v5 HOLDER.

The DDL-revoke bundle (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`), re-anchored
0020→**0021** (since #581's `0020_owner_bundle_watermark_read.sql` took ordinal 0020 and
`LatestOwnerBundleVersion == 20`), identified by `DDLRevokeOwnerBundleVersion = 21`,
**special-cased OUT of the owner prefix** and **EXCLUDED from `owner-ddl apply`**, applied
as the **TERMINAL plan step** after every runtime ownership reconcile (each
`ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE granted by
0018; the committed steady state denies CREATE); F12 (`T-deploy-runtime-object-ownership`)
in a non-superuser two-role cluster + `G-revoke-last`. M2 binds the executable exclusion
across the self-heal branch; it does NOT change the deploy-plan revoke-last mechanism.
Preserve the design mechanism, F12, and `G-revoke-last`.

### Other carry-forward (v1–v5 ledgers — do NOT re-litigate)

- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`), bootstrapping
  paradox dispositive, run-shape door held by three seams. No falsifier contested it.
- **Q3 mid-step resumability body — sound.** Q3-A (cursor advance + receipt in the
  step transaction), Q3-B (`in_progress` pre-marker + invalid-index reconciler), the
  §1.1 step taxonomy, the `deploy_cursor`-as-second-signal insight. M3 is about the
  serve-boot activation guard and Invariant B, not the per-step schema/cursor body —
  preserve the body, harden the activation interlock.
- **Decoupling shape — sound (subject to the M3 fix).** Shadow-first default-OFF
  `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark interlock +
  P3 drift gate / self-record, the embed-FS-derived plan, the hash-chained per-step
  receipt, `doctor schema_deploy_unrecorded`.
- **Scope discipline held.** P5 deferred; full capability revocation named as a
  follow-up beyond P4, not assumed; the local-first boundary respected.

## The 2 binding constraints v6 MUST resolve (the v5 adjudicator's needs_revision grounds)

> **These are BINDING.** The design-v5 falsification gate returned `needs_revision`
> because a material new falsifier challenge landed unrebutted (**M3**, falsifier_1, the
> decoupling-boundary / Invariant-B core), source-verified against current `main`, plus a
> non-safety test-staging finding (**M4**, falsifier_2). The revised spec clears the gate
> **only if it pins both** with the prescribed fix, produces the boot-path decision table,
> **and does not regress M1/M2/BC-N1/BC-N2/C1/C2/C3**; the cycle-1 falsifiers re-attack
> each one specifically. The verbatim prescribed fixes are §4 "What the revision must fix"
> in
> `docs/operator/artifacts/rfc-0142-p4-design-v5/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> — read that context doc in full (its `findings:` M3/M4 entries and §3 + §4).

### M3 (LOAD-BEARING, high — C2/decoupling-boundary + M1-Invariant-B core): the COMPLETE-cursor window lets the legacy path mutate + self-record AROUND the M1 gate

**The gap.** §3.3a `CheckDeployActivation` returns nil immediately when
`cursorState == complete` (defers to the drift gate, v5 `HOLDER.md:480-482`), and the
`revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt lives ONLY in the
`cursorState == none` branch (v5 `HOLDER.md:483-489`). So a deployer-aware/revoke-embedding
binary with a `complete` cursor and `STRIATUM_DEPLOY_DECOUPLED` OFF takes the legacy
`ConnectAndMigrate` path over a DB that **DOES** carry `deploy_cursor`/`deploy_plan`.
Verified against current source order in `ConnectAndMigrate`: watermark
(`go/pkg/db/connection.go:349`) → **`ApplyMigrations` (`:353`)** → `CheckSchemaDrift`
(`:376-383`) → **`RecordSchemaFingerprint` (`:399`)** — so the predicate returning nil lets
the legacy mutator AND the legacy self-record fire **WITHOUT** `VerifyStoredTranscript`.

**Reproducer (normal post-deploy steady state).** First P4 deploy complete
(`deploy_cursor.state = complete`, `owner_bundle_meta max >= 21`, CREATE revoked); boot a
LATER revoke-embedding binary (every future binary that embeds 0021 is deployer-aware) with
the flag OFF; the predicate returns nil; the legacy path runs `ApplyMigrations`. **Harm:**
(a) a pending runtime step needing CREATE hits a **#512-class lockout** AFTER 0021 revoked
CREATE (the exact failure P4's root reframe declares structurally impossible); (b) a step
needing no CREATE still **mutates schema on serve-boot** (the one-shot-deployer boundary
regresses); (c) in **shadow mode** the post-apply drift gate logs and falls through to
`RecordSchemaFingerprint` (`connection.go:384-399`), overwriting `schema_state` **around
the M1 gate** — directly **FALSIFYING §4.5 Universal Invariant B** (v5 `HOLDER.md:800-806`),
which scopes the legacy self-record to "no transcript, cursor absent."

This is the orthogonal COMPLETE-cursor window — **NOT** a BC-N2 regression (BC-N2's
non-complete window stays closed; falsifier_1 explicitly says "this is not the old BC-N2
pre-revoke non-complete window"). The holder's own §8 (v5 `HOLDER.md:907-914`) raises this
exact tension but its "intended close" covers only NON-`complete` cursors, so the spec as
written does not close it. The holder's strongest rebuttal (a `complete` cursor with a
matching fingerprint is legitimately in-sync) holds only for the immediate restart of the
SAME binary: a `complete` cursor is complete for the PREVIOUS plan, not proof the current
binary has no pending change, and the post-apply drift gate is too late because
`ApplyMigrations` already ran.

**Prescribed fix (binding, falsifiable):**
- Make `revokeEmbedded && !decoupledEnabled` a **pre-apply, DB-untouched halt for EVERY
  cursor state, including `complete`** — the conservative rule: a revoke-embedding binary
  with `STRIATUM_DEPLOY_DECOUPLED` OFF returns **`awaiting_deploy_config`**, DB untouched,
  before `ApplyMigrations` and before `RecordSchemaFingerprint`, on **both**
  `ConnectAndMigrate` and `ConnectAndVerify`. (If the design instead wants to permit a
  flag-OFF restart after a completed deploy, it MUST add a **pre-`ApplyMigrations`**
  plan/fingerprint comparison that cannot mutate or self-record — NOT rely on the current
  **post-apply** `CheckSchemaDrift`.)
- **Tighten Universal Invariant B** so a database carrying `deploy_cursor` / `deploy_plan`
  can **NEVER** reach the legacy `connection.go:399` writer: the legacy self-record is
  permitted **only** when no deploy transcript exists and the binary is not on the P4
  revoke/deploy path.
- **Extend F11 / F15** with the complete-cursor case: `cursorState == complete`,
  `revokeEmbedded == true`, `STRIATUM_DEPLOY_DECOUPLED` OFF, with a pending runtime
  migration (or a changed expected fingerprint). Assert `awaiting_deploy_config`,
  `ApplyMigrations` **not** called, `RecordSchemaFingerprint` **not** called,
  `schema_state` unchanged, and the DB byte-identical. Name the test e.g.
  **`T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`**.

> Source sites to re-anchor: predicate `cursorState == complete` defer (v5
> `HOLDER.md:480-482`), the `none`-branch-only halt (v5 `HOLDER.md:483-489`); the boot
> order `go/pkg/db/connection.go:349` (watermark) / `:353` (`ApplyMigrations`) / `:376-383`
> (`CheckSchemaDrift`) / `:384-399` (shadow fall-through + self-record) / `:399`
> (`RecordSchemaFingerprint`); §4.5 Invariant B (v5 `HOLDER.md:800-806`); §8 open tension
> (v5 `HOLDER.md:907-914`). Re-verify each against your run's `main` HEAD.

### M4 (secondary, medium — M2 test-staging): F16's test contract cannot build green as written

**The gap.** F16's `TestOwnerDDLApplyExcludesRevokeBundle` is specified to assert
production `OwnerBundles()` CONTAINS 0021 in rollout step 2 (v5 `HOLDER.md:439-442,845-849`)
— so `revokeEmbedded`/`ExpectedFingerprint` see it — but 0021 is not authored until rollout
step 7 (v5 `HOLDER.md:870-872`). Before `0021_*.sql` is embedded, `OwnerBundles()` will not
contain 21, so the step-2 assertion fails `make test`; delaying it contradicts the step-2
"F16 lands with the filter surface" claim; a synthetic 0021 does not prove the production
full-loader/`revokeEmbedded` condition as worded. This is a **test-STAGING** inconsistency,
NOT a safety/correctness defect — falsifier_2 explicitly states M2's safety invariant is
substantively resolved and found no path that commits the revoke early. It folds into the
required revision as a secondary fix.

**Prescribed fix (binding, falsifiable):**
- **Pre-0021 / inert phase (rollout step 2):** use a synthetic bundle list / test hook to
  prove `OwnerDDLApplyBundles` / `isNonRevokeBundle` exclude every bundle `>= 21`,
  `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a hand-passed synthetic 0021,
  and `ReapplyAllOwnerBundles(nil, …)` uses the filtered loader. Do **NOT** assert
  production `OwnerBundles()` contains 0021 yet.
- **Activation phase (rollout step 7, after 0021 is authored):** assert production
  `OwnerBundles()` contains 0021, `ExpectedFingerprint()` includes its bytes,
  `revokeEmbedded` derives from the full loader / file presence, and production
  `OwnerDDLApplyBundles()` excludes it.
- **Keep the forced-self-heal pgtest in the activation phase** (or make its synthetic
  fixture explicit) and require it to prove it actually reaches `ReapplyAllOwnerBundles`
  through `isCrossBundleDependencyError` (`go/pkg/db/owner.go:367-374`), not merely the
  pending loop.

## Proactive completeness — the boot-path decision table (force convergence — the v6 holder MUST do this)

A re-scaffolded revision that pins exactly the two §4 items but leaves an unaudited
boot-path combination open will simply spawn an M5. The SEED's v5 proactive-hardening
section warned an "unaudited sibling self-record path" would spawn M3 — and it did. Close
that class entirely with a **COMPLETE boot-path decision table**:

- For **EVERY** combination of `cursorState` ∈ {none, in_progress, finalizing, complete}
  (treat `step_committed` / `aborted` per the §1.3 disambiguation) × `decoupledEnabled` ∈
  {on, off} × `revokeEmbedded` ∈ {yes, no} × `applied_owner` ∈ {<20, ==20, >=21}, specify
  the **exact guard / outcome**: halt `awaiting_deploy` / `awaiting_deploy_config`, run the
  deployer, run legacy `ConnectAndMigrate` (serve + auto-apply), run `VerifyStoredTranscript`,
  serve, etc.
- **PROVE §4.5 Universal Invariant B holds in EVERY cell** — *NO fingerprint /
  `schema_state` `complete` is written unless the FULL stored transcript byte-matches the
  binary* — explicitly including:
  - the **M3 cell** (`complete` + decoupled OFF + revoke-embedding + a pending change),
  - the **shadow-mode drift-gate fall-through** (`connection.go:384-399` logs then
    self-records),
  - the legitimate fresh-DB bring-up / inert-landing cells (no-revoke binary, no transcript)
    that must still serve and NOT be wedged by the conservative M3 halt.
- Make the table an **executable, named requirement** the falsifiers can verify against the
  named predicate sites (v5 `HOLDER.md:480-489`) and the `connection.go:353/:376-383/:399`
  ordering. This closes M3 and **preempts any further unguarded-combination challenge**.

## Root reframe (do not lose this)

**Schema mutation must stop being an implicit side effect of the serving process's
restart and become an explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer.** Then the serving daemon can hold zero DDL
privilege, "restart force-commits a half-applied deploy" becomes impossible, and a
bad migration can never wedge the single writer on boot. **M3 is exactly a residual
violation of this reframe: the legacy `ApplyMigrations`-on-serve-boot path is still
reachable for a revoke-embedding binary on a `complete` cursor — the deploy must own
that mutation, never serve-boot.**

## The two Open Questions P4 must pin (from the RFC)

- **Q3 — How atomic is "atomic"?** Confirm the **per-step-atomic + resumable-cursor**
  contract is sufficient for every owner+runtime interleaving we actually ship, or
  specify the small set of steps that need a stricter sub-protocol. Every step must
  be idempotent and leave a coherent intermediate the cursor/transcript classifies
  as "incomplete, resume" — not "unknown drift, panic". **In v6 this explicitly
  includes the COMPLETE-cursor activation guard (M3) and the full boot-path decision
  table proving Invariant B everywhere.** *(RFC: "This is the hard correctness core of P4.")*
- **Q4 — Should a deploy be a Striatum run?** Plain verb vs. a dogfooded run shape,
  with the bootstrapping paradox. Resolved in v1–v5 — carry forward.

## Load-bearing risks (attack these)

- **R1 atomicity/fingerprint-is-partly-a-lie:** non-transactional DDL, non-idempotent
  steps, a two-connection crash window, the receipt key moving with the frontier (BC-N1,
  fixed), a resume binary that disagrees with an already-applied step (M1, fixed), or **a
  `complete`-cursor revoke-embedding binary on the legacy path self-recording around the
  M1 gate (M3)**. Test: kill-and-resume across each step class; resume with a divergent
  binary; **and boot a revoke-embedding binary on a `complete` cursor with the flag OFF**.
- **R2 decoupling regresses a landed gate:** lifting `ApplyMigrations` must NOT break the
  P2 watermark interlock, the P3 drift gate / self-record, fresh-DB bring-up, the BC-N2
  universal non-complete edge / clean boot, **nor leave the legacy `ApplyMigrations` path
  reachable for a deployer-aware binary over a DB carrying a deploy transcript (M3)**.
- **R3 DDL-revocation lockout / early REVOKE:** revoking serving-role DDL must not lock out
  the runtime path, nor let any `owner-ddl apply` route commit 0021 early (M2, fixed),
  **nor re-open a #512-class lockout via the legacy serve-boot path after the revoke (M3)**.
- **R4 cursor / transcript / receipt / fingerprint holes:** double-apply/skip at a commit
  boundary; a stored transcript that disagrees with the binary/DB stamps on resume (M1,
  fixed); out-of-order apply; **a self-record path that writes `complete`/fingerprint
  without the full-transcript check — including the legacy `connection.go:399` writer
  reachable on a `complete` cursor (M3)**.

## Anchor verification against current `main` (operator pre-flight)

Verified against `~/git/striatum` @ `origin/main`. P0–P3 + P2 are **landed**; the
P4 surfaces are **NOT-FOUND (to be built)**. Treat as ground truth; re-anchor the
spec to these file:line references. **Re-confirm against your run's `main` HEAD.**

| Claim / target | Status | Anchor (current source) |
| --- | --- | --- |
| Boot-time auto-apply runs runtime migrations as `striatumd_rw` (the coupling P4 removes) | **ACCURATE** | `go/pkg/db/connection.go` `ConnectAndMigrate` → `ApplyMigrations` `:353`; P3 path / `RecordSchemaFingerprint` `:399`. |
| **`ConnectAndMigrate` boot order: watermark → `ApplyMigrations` → `CheckSchemaDrift` → `RecordSchemaFingerprint` (the M3 ordering)** | **ACCURATE** | `go/pkg/db/connection.go:349` (`CheckOwnerBundleWatermark`) → `:353` (`ApplyMigrations`) → `:376-383` (`CheckSchemaDrift`, refuse-mode halt) → `:384-393` (shadow-mode log + fall-through) → `:399` (`RecordSchemaFingerprint`, "the only writer of schema_state"). A pre-`ApplyMigrations` guard returning nil lets the mutator + self-record run; the drift gate is POST-apply. |
| **`ExpectedFingerprint()` hashes EMBEDDED FILE BYTES** | **ACCURATE** | `go/pkg/db/schema_drift.go:83-100` — `MigrationSHASet()` + each `OwnerBundle.SHA256()`, keyed by version. |
| **`LiveFingerprint` reads the self-recorded singleton — NO recompute** | **ACCURATE** | `go/pkg/db/schema_drift.go:145-160`; `RecordSchemaFingerprint` UPSERTs the running binary's expected value `:171-194`; `CheckSchemaDrift` compares the singleton to this binary `:239-274`. |
| **`ApplyOwnerBundles` self-heal falls into `ReapplyAllOwnerBundles` on a cross-bundle dependency error** | **ACCURATE** | `go/pkg/db/owner.go:277-302`; `isCrossBundleDependencyError` `:367-374`. M2 binds `isNonRevokeBundle` / `OwnerDDLApplyBundles()` here (resolved). |
| **`applyPendingOwnerBundles` bounds `<= current`; `ReapplyAllOwnerBundles` re-runs every loaded bundle** | **ACCURATE** | `go/pkg/db/owner.go:309-320` / `:324-349`. M2's filter + in-loop guards bind both (resolved). |
| **`OwnerBundles()` shared loader (the binary surfaces 0021 through it for `revokeEmbedded`)** | **ACCURATE** | `go/pkg/db/owner.go:196-224`. M2's split keeps `OwnerBundles()` for `revokeEmbedded`/`ExpectedFingerprint`; `OwnerDDLApplyBundles()` excludes 0021 (resolved). |
| Owner bundle frontier = 0020 (`0020_owner_bundle_watermark_read.sql`) — NOT the DDL-revoke bundle | **ACCURATE** | `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 20`, `:35` `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20). **The C3 DDL-revoke bundle stays renumbered at 0021.** |
| Runtime migration frontier (P3 added `schema_state`) | **ACCURATE** | `go/pkg/db/migrations.go` `LatestDaemonDBVersion`. New P4 runtime migration (`deploy_plan`/`deploy_cursor`) is **≥ 0044**. |
| P2 watermark interlock + clean halt | **ACCURATE (landed)** | `go/pkg/db/owner.go` `CheckOwnerBundleWatermark`, called **before** `ApplyMigrations` in boot (`connection.go:349`). |
| `owner-ddl apply` applies bundles out-of-band as owner role | **ACCURATE (landed)** | `go/pkg/cli/localcommands/daemon.go` `runDaemonOwnerDDL` → `db.ApplyOwnerBundles()`. The DDL-revoke bundle is **excluded from every route** (M2). |
| `striatum daemon deploy` / `deploy_cursor` / `deploy_plan` / deploy receipt / `CheckDeployActivation` / `VerifyStoredTranscript` | **NOT-FOUND (P4 builds these)** | `go/pkg/cli/localcommands/daemon.go` has no `deploy`; no `deploy_cursor`/`deploy_plan` table; no activation/verify surface yet. |

**Net design implication.** The P4 ground is clean: M1/M2/BC-N1/BC-N2/C1/C2/C3 are
resolved in v1–v5 and must carry forward unregressed. The remaining hard parts are **M3** —
close the COMPLETE-cursor window so a revoke-embedding binary with the flag OFF over a DB
carrying a deploy transcript can never take the legacy `ConnectAndMigrate` mutate+self-record
path AROUND `VerifyStoredTranscript`, tightening Invariant B and proving it across the
COMPLETE boot-path decision table — and **M4** — split F16 phase-aware so M2's filters land
green before 0021 is authored. Be shadow-first: serve-boot auto-apply stays the default for
a no-revoke inert binary on a clean DB, but a revoke-embedding binary with the flag OFF over
a deploy transcript must halt, never auto-apply.

## Clearing condition (pin this)

The gate clears **only** if the cycle-1 adjudicator's collaboration ledger records a
clearing verdict (`accept` or `accept_with_findings`, never `clear`), which requires
**all** of: **M3 genuinely resolved** (the COMPLETE-cursor window closed — a revoke-embedding
binary + flag OFF over a `deploy_cursor`/`deploy_plan`-carrying DB halts pre-apply
DB-untouched OR runs a pre-`ApplyMigrations` comparison that cannot mutate/self-record,
NEVER the post-apply `CheckSchemaDrift`; Invariant B tightened so the legacy
`connection.go:399` writer is unreachable when a transcript is present; F11/F15 extended with
the complete-cursor case asserting `awaiting_deploy_config` + un-called
`ApplyMigrations`/`RecordSchemaFingerprint` + unchanged `schema_state` + byte-identical DB),
**M4 genuinely resolved** (F16 split phase-aware — a pre-0021 synthetic-list phase + a
post-0021 production phase; the forced-self-heal pgtest in the activation phase reaching
`ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`), the **proactive-completeness
boot-path decision table COMPLETE** (every `cursorState` × `decoupledEnabled` ×
`revokeEmbedded` × `applied_owner` cell with a specified guard/outcome and Invariant B proven
in every cell, incl. the M3 cell and the shadow-mode fall-through), **M1 + M2 + BC-N1 + BC-N2
+ C1 + C2 + C3 carried forward intact**, and **no new material challenge standing
unrebutted**. A spec that merely *claims* the two fixes without the concrete complete-cursor
guard and the complete decision table has NOT cleared the gate. This is the single allowed
revision cycle for v6 — a second `needs_revision` ends the gate unCleared.

---
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run (v6 /
REVISION of `rfc-0142-p4-design-v5`; resolves the cycle-1 findings M3 + M4 and carries
M1/M2/BC-N1/BC-N2/C1/C2/C3 forward). Lanes: author=claude (holder/adjudicator/committer),
reviewer=codex (falsifiers).</sub>

## Operator sharpenings (pin these — do NOT let cycle-1 re-derive them)

- **M3: guard the `complete` cursor, halt BEFORE `ApplyMigrations`.** The defect is that
  §3.3a returns nil on `cursorState == complete` (v5 `HOLDER.md:480-482`) and the
  `revokeEmbedded && !decoupledEnabled` halt is only in the `none` branch (v5
  `HOLDER.md:483-489`), so a revoke-embedding binary + flag OFF over a DB carrying a deploy
  transcript rides the legacy `ApplyMigrations` (`connection.go:353`) + post-apply
  `CheckSchemaDrift` (`:376-383`) + self-record (`:399`) path around `VerifyStoredTranscript`
  — re-opening a #512-class lockout and overwriting `schema_state` in shadow mode. The fix
  is a pre-apply, DB-untouched halt for EVERY cursor state (conservative:
  `awaiting_deploy_config`) OR a pre-`ApplyMigrations` plan/fingerprint comparison; plus
  tightening Invariant B so a transcript-carrying DB never reaches `:399`. The reproducer
  cycle-1 falsifier_1 MUST re-run: boot a revoke-embedding binary on a `complete` cursor
  with the flag OFF and a pending change, and prove `awaiting_deploy_config`, NO
  `ApplyMigrations`, NO `RecordSchemaFingerprint`, NO `schema_state` change, DB
  byte-identical.
- **M4: split F16 by rollout phase.** The named test cannot build green as written
  (production `OwnerBundles()` contains 0021 asserted in step 2, but 0021 authored in step
  7). The fix is a pre-0021 synthetic-list phase proving the exclusion-filter contract and
  a post-0021 production phase proving the embed/listing split — with the forced-self-heal
  pgtest in the activation phase actually reaching `ReapplyAllOwnerBundles`.
- **Build the decision table ONCE, exhaustively.** Cover every
  `cursorState` × `decoupledEnabled` × `revokeEmbedded` × `applied_owner` cell and prove
  Invariant B in each — so cycle-1 cannot spawn an M5 from an unaudited combination.
- **Carry forward unregressed.** M1's full-transcript verifier, M2's non-revoke filter,
  BC-N1's moving-frontier fix, BC-N2's universal non-complete edge, the C1 finalizer, the
  C2 edge (`RequiredOwnerBundleVersion = 20`, forward-watermark at `applied >= 21`), and the
  C3 revoke-last mechanism (0021 special-cased + terminal) must all stay intact. M3 EXTENDS
  the C2/activation edge to the complete-cursor case and tightens Invariant B; M4 only
  restructures F16's staging; neither re-opens a resolved mechanism.

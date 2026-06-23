# Design-Run Seed (v5 / REVISION) — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> **THIS IS THE FIFTH REVISION (v5).** Four prior design runs ran the same
> falsification gate. v1 (`rfc-0142-p4-design`) returned **`needs_revision`** with
> three findings C1/C2/C3. v2 (`rfc-0142-p4-design-v2`) **resolved C1 and C2** but
> returned `needs_revision` again because C3 (ownership policy) was still open and
> a new finding N1 (per-step receipt not crash-safe) landed. v3
> (`rfc-0142-p4-design-v3`) **resolved C3** (the revoke-last ownership mechanism;
> both falsifiers conceded it) and closed the immediate N1 hole, BUT returned
> `needs_revision` on two NEW grounds BC-N1 + BC-N2. v4 (`rfc-0142-p4-design-v4`)
> **resolved BOTH BC-N1 and BC-N2** — both v4 falsifiers explicitly conceded the v3
> reproducers no longer reproduce, and C1/C2/C3 carried forward intact — BUT
> returned **`needs_revision` again** (the gate's single allowed cycle) on **two
> independent NEW material challenges**, each source-verified against current `main`
> and each squarely in a correctness/safety core:
> **M1** — the v4 transcript/binary verification rule checks ONLY not-yet-applied
> steps, so a hybrid (already-applied bytes differ) deploy self-records as in-sync —
> and **M2** — the v4 "0021 unreachable via `owner-ddl apply`" guarantee binds only
> `applyPendingOwnerBundles`, leaving the FMA-007 self-heal reapply branch free to
> commit the revoke early (#512-class lockout). This v5 run is a **proper revision**:
> the holder starts from the **v4** `HOLDER.md` (a required context doc), REVISES the
> spec to **resolve M1 + M2** per their prescribed fixes below, and **carries
> BC-N1/BC-N2/C1/C2/C3 forward unregressed**; the falsifiers re-attack the revised
> spec. The v4 design record — `dialogue/holder/HOLDER.md`,
> `dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0142-p4-design-v4/`; the **v4** `HOLDER.md` (the
> revised spec) and the **v4** collaboration ledger (the verdict + the full M1/M2
> analysis and §4 "What the revision must fix") are wired in as required
> `context_docs`.
>
> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries
> the charter, restates the two Open Questions P4 must pin (Q3 + Q4), pins the two
> binding revision constraints M1 + M2, states what already cleared (BC-N1 + BC-N2 +
> C1 + C2 + C3) and must NOT be reopened, names the proactive hardening, and pins an
> operator anchor-verification table you must build on. Read this whole file, the
> **v4** `HOLDER.md` + the **v4** collaboration ledger context docs, and the RFC
> (esp. the Phasing table P4 row and the "Open Questions" section) before producing
> any artifact.

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
5. **Resolve BOTH binding revision constraints (M1, M2) below**, each per its
   prescribed fix, do the **proactive hardening**, and **carry forward BC-N1 + BC-N2
   + C1 + C2 + C3 unregressed** (see "Carried forward"). A revised spec that leaves
   M1 or M2 open — or that regresses any carry-forward finding — has NOT cleared the
   gate. This is the gate's single allowed revision cycle for v5, so a second
   `needs_revision` ends the gate unCleared.

## Carried forward — resolved by v3/v4 (do NOT reopen)

> The v4 collaboration ledger records BC-N1's moving-frontier fix and BC-N2 as
> genuinely resolved **this cycle** (both v4 falsifiers concede the v3 reproducers
> no longer reproduce), and C1, the C2 edge, and the C3 ownership mechanism as
> intact and unregressed (both falsifiers concede the C3 design-level answer). The
> v5 revision MUST preserve them — verbatim from the **v4** `HOLDER.md` — and the
> cycle-5 adjudicator's clearing verdict requires them intact. Do NOT re-litigate any
> of them; they are not this cycle's open questions. M1 is an *extension* of BC-N1's
> binary-verification clause to already-applied steps (and to the pre-finalizer
> check), not a re-opening of the moving-frontier mechanism; M2 binds the
> *self-heal branch* of `owner-ddl apply`, not a re-opening of the C3 deploy-plan
> revoke-last mechanism.

### BC-N1 — RESOLVED this cycle (immutable plan identity across resume). Preserve verbatim from the v4 HOLDER.

The v3 moving-frontier instability — a re-run rebuilding `H' != H` over the moved
live frontiers and renumbering `step_index` — is closed by:
- a new runtime-owned **`deploy_plan` table** (migration ≥ 0044) storing the
  **IMMUTABLE ordered transcript** keyed by `plan_hash` (`base_owner_version`,
  `base_runtime_version`, target frontiers, the terminal-revoke `step_index`, and
  the full ordered `[{step_index, step_id, role, sha256, transactional}]`),
  materialized in the **same transaction** that sets `deploy_cursor →
  in_progress(0)` **BEFORE step 0 mutates the frontier**;
- **resume loads `deploy_plan[cursor.plan_hash]`** and resumes at the stored
  `step_index` — **never** recomputing the pending-delta `BuildPlan` over the moved
  live frontiers;
- **§1.3** reclassifying the v3 unclassified-foreign-incomplete bucket as
  recoverable "resume with the STORED plan", and the per-step `doctor
  schema_deploy_unrecorded` enumerating applied steps from the **stored transcript**;
- **F14 `T-deploy-plan-hash-resume-after-step`** killing after step 0 AND step 1 and
  asserting the re-run reuses `plan_hash`, preserves `step_index`es, recognizes
  prior receipts exactly-once, and ends green.
Preserve the `deploy_plan` table, the materialize-before-step-0 ordering, the
stored-transcript resume, the §1.3 reclassification, the transcript-enumerated
doctor, and F14. **M1 EXTENDS the binary-verification half of this fix — it does NOT
re-open the moving-frontier mechanism.**

### BC-N2 — RESOLVED this cycle (universal pre-revoke serve edge). Preserve verbatim from the v4 HOLDER.

The pre-revoke incomplete-deploy serve window is closed by **one UNIVERSAL hard
edge**:
- **`CheckDeployActivation` is no longer gated on `revokeEmbedded`** — EVERY
  deployer-aware binary (including the no-revoke landing binary) reads
  `deploy_cursor` immediately after `CheckOwnerBundleWatermark` and **BEFORE
  `ApplyMigrations` and BEFORE `RecordSchemaFingerprint`** in both `ConnectAndMigrate`
  and `ConnectAndVerify`, and halts **`awaiting_deploy` DB-untouched** whenever the
  cursor exists and `state != complete`, regardless of `revokeEmbedded`/watermark;
- the edge holds at the pre-revoke frontier **`applied_owner == 20`** (the window the
  C3 revoke-last ordering opens) and does **not** fire on a fresh/idle DB (absent
  cursor / `idle` serves on the legacy path), so C3 is not regressed and a clean boot
  is not wedged;
- **F11 case (e)** — a no-0021 deployer-aware binary at `applied_owner == 20` with
  `deploy_cursor.state ∈ {in_progress, step_committed, finalizing}` halts
  `awaiting_deploy`, with spies proving `ApplyMigrations` and `RecordSchemaFingerprint`
  are NOT called and the DB is byte-identical — and **case (f)** — the same binary
  with no cursor / `idle` SERVES; **`G-old-binary-refuse`** extended to prove the
  pre-revoke window cannot be served.
Preserve the universal `revokeEmbedded`-independent edge, the `applied_owner == 20`
halt, F11(e)/(f), and the extended `G-old-binary-refuse`. Both v4 falsifiers
concede BC-N2 and decline to use it as a blocker.

### C1 — RESOLVED (finalization boundary). Preserve verbatim from the v4 HOLDER.

The unclassifiable terminal state is closed by **Option B + idempotent finalizer**:
the distinct **`finalizing`** `deploy_cursor` state entered in one commit after the
last step; an **idempotent finalizer** that appends the `complete` receipt (guarded
skip on `(plan_hash, state=complete)`), UPSERTs the `schema_state` fingerprint, then
advances `finalizing → complete` **LAST**; the **§1.3 row** classifying `finalizing`
as resumable finalization (never serve, never genuine-drift); the operator pin that
no single finalization transaction spans the owner+runtime connections; and F10
(`T-deploy-resume-finalization-crash`). Preserve all of it. **NOTE:** the finalizer
is the VEHICLE through which M1's harm manifests (it writes the running binary's
`ExpectedFingerprint()` and marks `complete` without a full-transcript check) — M1's
fix ADDS a pre-finalizer full-transcript check; it does NOT change the finalizer's
boundary semantics. The `complete`-with-missing-receipt window stays closed.

### C2 — RESOLVED at the fail-closed EDGE. Preserve verbatim from the v4 HOLDER.

The crash-loop (#512-class) is closed at the edge by: **`CheckDeployActivation`**
called immediately after `CheckOwnerBundleWatermark` and **BEFORE `ApplyMigrations`**
in both boot paths, mutating nothing; **absent `deploy_cursor` treated as incomplete**;
the typed non-restartable halts **`awaiting_deploy`** and **`awaiting_deploy_config`**;
the **forward-watermark rule** re-anchored to fire at **`applied >= 21`**; and
**`RequiredOwnerBundleVersion` KEPT at its current-`main` value 20** — NOT advanced to
the revoke ordinal 21 (0021 is deploy-plan-only). Preserve all of it, including F11.
The BC-N2 universal edge is an **addition** to this edge, not a replacement.

### C3 — RESOLVED (ownership transfer, revoke-last). Preserve verbatim from the v4 HOLDER.

The v2 self-contradiction is resolved by **resolution (a) — sequence the revoke
last**: the DDL-revoke bundle (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`),
re-anchored 0020→**0021** (since #581's `0020_owner_bundle_watermark_read.sql` took
ordinal 0020 and `LatestOwnerBundleVersion == 20`), identified by
`DDLRevokeOwnerBundleVersion = 21`, is **special-cased OUT of the owner prefix** and
**EXCLUDED from `owner-ddl apply`**, applied as the **TERMINAL plan step** after every
runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while
`striatumd_rw` still holds CREATE granted by 0018; the committed steady state denies
CREATE); F12 (`T-deploy-runtime-object-ownership`) in a documented non-superuser
two-role cluster + `G-revoke-last`. Preserve the design mechanism, F12, and
`G-revoke-last`. **M2 binds the executable exclusion across the SELF-HEAL branch of
`owner-ddl apply`; it does NOT change the deploy-plan revoke-last mechanism.**

### Other carry-forward (v1–v4 ledgers — do NOT re-litigate)

- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`), bootstrapping
  paradox dispositive, run-shape door held by three seams. No falsifier contested it.
- **Q3 mid-step resumability body — sound.** Q3-A (cursor advance + receipt in the
  step transaction), Q3-B (`in_progress` pre-marker + invalid-index reconciler), the
  §1.1 step taxonomy, the `deploy_cursor`-as-second-signal insight. M1 is about
  FULL-transcript binary/DB-stamp verification, not the per-step schema/cursor body —
  preserve the body, harden the verification.
- **Decoupling shape — sound.** Shadow-first default-OFF `STRIATUM_DEPLOY_DECOUPLED`,
  `ConnectAndVerify` preserving the P2 watermark interlock + P3 drift gate /
  self-record, the embed-FS-derived plan, the hash-chained per-step receipt,
  `doctor schema_deploy_unrecorded`.
- **Scope discipline held.** P5 deferred; full capability revocation named as a
  follow-up beyond P4, not assumed; the local-first boundary respected.

## The 2 binding constraints v5 MUST resolve (the v4 adjudicator's two unrebutted needs_revision grounds)

> **These are BINDING.** The design-v4 falsification gate returned `needs_revision`
> because two material falsifier challenges landed and stood unrebutted: **M1**
> (falsifier_1, the atomicity/fingerprint lens) and **M2** (falsifier_2, the
> owner-ddl/migration-safety lens), both source-verified against current `main`. The
> revised spec clears the gate **only if it pins both** with the prescribed fix, does
> the proactive hardening, **and does not regress BC-N1/BC-N2/C1/C2/C3**; the cycle-5
> falsifiers re-attack each one specifically. The verbatim prescribed fixes are §4
> "What the revision must fix" in
> `docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
> — read that context doc in full (its `findings:` M1/M2 entries and §4).

### M1 (high — Q3/R4 correctness core): the transcript/binary verification only checks NOT-YET-APPLIED steps, so a hybrid deploy self-records as in-sync

**The gap.** v4 resolved BC-N1's moving frontier by storing the immutable transcript
and resuming off it — but the binary-compatibility half of the v3 prescription
("load the stored transcript by `plan_hash`, **verify the embedded bytes still match
the binary**, and resume" — UNQUALIFIED across the whole transcript) was **narrowed to
FUTURE steps only**. §1.3 resumes when `deploy_plan[plan_hash]` is present and the
**not-yet-applied** steps' `sha256` match the binary's embedded bytes (v4
`HOLDER.md:235`); `deploy_plan_binary_mismatch` fires **only** on a not-yet-applied
step mismatch (`HOLDER.md:238`); F14's negative case tampers a **not-yet-applied**
stored step (`HOLDER.md:766`); and the holder's own **§8 concedes the already-applied
check does not fire** (`HOLDER.md:833-839` — "as written, the answer is no").

So a resume binary whose **already-applied** step bytes differ from the stored
transcript is **not** forced into the mismatch bucket. Reproducer (the v4
two-runtime-step F14 shape): binary A materializes `H = [runtime:0045 sha A45,
runtime:0046 sha A46, owner:0021 sha A21]`; step 0 commits `A45` (runtime DDL +
`schema_migrations` sha A45 + cursor advance + per-step receipt `(H,0,runtime:0045,A45)`
in one owner-connection tx, `go/pkg/db/migrations.go:304-355`); crash after step 0;
resume with binary B whose already-applied `0045` bytes differ (`B45`) but whose
not-yet-applied `0046`/`0021` still match (plausible if the activation binary is
rebuilt/amended after the partial deploy). Under the v4 text the not-yet-applied check
passes; step 0's mismatch is unchecked; the cursor classifies as
**resume-off-stored-transcript** (NOT `deploy_plan_binary_mismatch`); B applies
`0046`+`0021`; then the **C1 finalizer records B's `ExpectedFingerprint()`** into
`schema_state`. Verified against current source:
- **`ExpectedFingerprint()` hashes the running binary's embedded migration/bundle
  BYTES** keyed by version (`go/pkg/db/schema_drift.go:83-99`) — so B computes a
  DIFFERENT expected fingerprint than A;
- **`LiveFingerprint` reads the self-recorded `schema_state` singleton — NO
  recomputation from `schema_migrations`/catalog** (`schema_drift.go:145-160`);
- **`RecordSchemaFingerprint` UPSERTs the running binary's expected value**
  (`schema_drift.go:171-194`); **`CheckSchemaDrift` compares the singleton to this
  binary** (`schema_drift.go:239-274`).

So B finalizes a **hybrid** (A45 actually applied, B expected), writes B's fingerprint
as the durable "live" value, marks `complete`; a later B boot reads `LiveFingerprint
== ExpectedFingerprint` and **serves a hybrid as in-sync** (silent schema corruption).
The receipt chain and `schema_migrations` still record A45 (provenance preserved), but
the serve-gate fingerprint says B. The tightened doctor checks receipt **presence**
(all present), not stored-SHA-vs-DB-stamp, so it stays green (`HOLDER.md:572-584`).
This is exactly the transcript-vs-binary mismatch class the run was asked to force into
`deploy_plan_binary_mismatch`.

**Prescribed fix (binding, falsifiable):**
- On **every resume**, validate **every** stored transcript step's `sha256` against
  the current binary's embedded bytes — **NOT only not-yet-applied steps**. If any
  stored step SHA differs from the binary's embedded bytes (already-applied OR future),
  classify **`deploy_plan_binary_mismatch`** (typed halt, DB-untouched, apply nothing).
- For **already-applied** transcript entries, **also verify the DATABASE STAMPS** match
  the stored transcript (`schema_migrations.sha256` for runtime steps,
  `owner_bundle_meta.sha256` for owner steps). A mismatch is a legible
  transcript/database-stamp mismatch that **refuses to finalize** — never a silent
  resume.
- Apply the **SAME full-transcript check BEFORE the C1 finalizer** writes `schema_state`
  or advances `finalizing → complete`, so a binary that disagrees with an
  already-applied step can **NEVER** self-record its own `ExpectedFingerprint()` as
  in-sync. Make the already-applied verification fire BEFORE any self-record/fingerprint
  write.
- **Named test — `T-deploy-resume-already-applied-byte-mismatch-refuses`** (extend
  F4 / F14 / F13): kill after step 0 commits, resume with a binary whose step-0 bytes
  differ but whose remaining steps match, and assert `deploy_plan_binary_mismatch`, **no**
  step 1 apply, **no** `RecordSchemaFingerprint`, **no** `complete` cursor, and a
  non-green doctor / typed diagnostic. Add the **symmetric owner-step** case (a completed
  owner prefix / pre-finalization crash, owner-bundle SHA divergence).

> Source sites to re-anchor: `go/pkg/db/schema_drift.go:83-99` (`ExpectedFingerprint`
> hashes embedded bytes), `:145-160` (`LiveFingerprint` no recompute), `:171-194`
> (`RecordSchemaFingerprint` UPSERTs running binary's expected), `:239-274`
> (`CheckSchemaDrift`); `go/pkg/db/migrations.go:304-355` (the atomic step tx);
> v4 `HOLDER.md:235,238,572-584,766,833-839`.

### M2 (high — C3/R3 decoupling/migration-safety core): the "0021 unreachable via owner-ddl apply" guarantee binds only the pending loop; the self-heal reapply can commit REVOKE CREATE early

**The gap.** v4's C3 carry-forward bounds only `applyPendingOwnerBundles` (`bundle.Version
<= current`, `go/pkg/db/owner.go:309-320`) and adds a **generic** build guard, but the
FMA-007 self-heal branch is left unbound. `ApplyOwnerBundles` runs
`applyPendingOwnerBundles` and, on a cross-bundle dependency error
(`isCrossBundleDependencyError`, the FMA-007 / #458 condition), falls into
**`ReapplyAllOwnerBundles(ctx, runner, bundles, daemonVersion)`** over the FULL loaded
slice (`owner.go:277-302`); `ReapplyAllOwnerBundles` calls `applyOneOwnerBundle` for
**EVERY** bundle in ascending order "regardless of the recorded version" — **no `<=
LatestOwnerBundleVersion` bound, no 0021 skip** (`owner.go:324-349`). Since the
activation binary's `OwnerBundles()` (the shared loader, `owner.go:196-224`) **MUST
include the 0021 file for `revokeEmbedded` detection**, the slice the reapply branch
iterates **includes 0021**.

Concrete refutation: DB at owner frontier 20, before the activation deploy; run
`striatum daemon owner-ddl apply` on the activation binary (which embeds 0021 but keeps
`Latest=Required=20`); inject the already-supported FMA-007 condition (a `<= 20` bundle
fails on a missing earlier object); `ApplyOwnerBundles` then calls
`ReapplyAllOwnerBundles`, which commits **`REVOKE CREATE ON SCHEMA striatumd FROM
striatumd_rw`** (0021) **outside the deploy plan**, before any pending runtime step and
before the C3 ownership reconcile. A subsequent deployer runtime step needs CREATE that
is now gone; the spec's `deploy_create_prerequisite_missing` guard halts before DDL — a
**#512-class lockout in the activation path**, recreating exactly the contradiction the
revoke-last mechanism exists to prevent. §8 itself flags the question
(`HOLDER.md:850-856`) but the body does not close the branch; F12/`G-revoke-last` prove
only the happy deploy-plan ordering (`HOLDER.md:764`). A generic build guard is too easy
to satisfy against the normal path while leaving the reconciliation escape hatch open.

**Prescribed fix (binding, falsifiable):**
- Define a **single non-revoke bundle filter** (exclude **every** bundle `>=
  DDLRevokeOwnerBundleVersion` = 0021, regardless of the recorded version — not only
  bundles newer than the current watermark) and apply it to **EVERY** `owner-ddl apply`
  path: **`applyPendingOwnerBundles`**, **`ReapplyAllOwnerBundles`** (the cross-bundle
  self-heal), the **`ApplyOwnerBundles`** self-heal branch, **tests**, and any
  **dry-run / list** surfaces. State it as an **executable, named requirement** (not a
  generic build-guard sentence), so 0021's REVOKE can ONLY ever be committed as the
  terminal `deploy` step and NEVER via a self-heal reapply.
- **Split the embed/listing helper** so "the binary embeds 0021" (for `revokeEmbedded`
  / `ExpectedFingerprint`) does **NOT** imply "`owner-ddl apply` can iterate 0021": the
  deploy plan and `revokeEmbedded` may see 0021; the `owner-ddl apply` bundle slice must
  not.
- **Named test — `T-deploy-revoke-excluded-from-reapply-self-heal`:** embed a synthetic
  0021 revoke bundle, **force the cross-bundle dependency self-heal path**, run
  `ApplyOwnerBundles`, and assert 0021 is **NOT** applied, `owner_bundle_meta` never
  records 21, and `has_schema_privilege('striatumd_rw','striatumd','CREATE')` remains
  **TRUE** (CREATE still held).
- **Extend F12 / `G-revoke-last`** with an `owner-ddl apply` side-path case (activation
  binary, owner frontier 20, `owner-ddl apply` invoked before deploy, including the
  FMA-007 reapply branch) proving 0021 stays pending for the deploy plan and the
  subsequent runtime ownership reconcile can still run while CREATE is held.

> Source sites to re-anchor: `go/pkg/db/owner.go:23,35`
> (`LatestOwnerBundleVersion`/`RequiredOwnerBundleVersion` = 20),
> `:196-224` (`OwnerBundles()` loader), `:277-302` (`ApplyOwnerBundles` self-heal
> branch), `:309-320` (`applyPendingOwnerBundles` `<= current` bound), `:324-349`
> (`ReapplyAllOwnerBundles` re-runs every loaded bundle, no bound, no skip);
> v4 `HOLDER.md:351-358,723-739,764,850-856`.

## Proactive hardening (preempt the next cycle — the holder MUST do this in v5)

A re-scaffolded revision that pins exactly the two §4 items but leaves an
unaudited sibling path open will simply spawn an M3. Do the audit ONCE, exhaustively:

- **Audit EVERY code path that applies owner bundles** — `applyPendingOwnerBundles`,
  `ReapplyAllOwnerBundles`, the `ApplyOwnerBundles` self-heal branch, any
  `owner-ddl apply` dry-run / list surface, and any test helper that applies bundles —
  and name each against current `main` (file:line). State the **universal invariant**:
  *the DDL-revoke bundle (`>= DDLRevokeOwnerBundleVersion` = 0021) is excluded from ALL
  apply paths — only the terminal `deploy` step commits it.*
- **Audit EVERY code path that self-records / writes a schema fingerprint** —
  `RecordSchemaFingerprint`, the C1 finalizer's `schema_state` UPSERT + `finalizing →
  complete` advance, any serve-boot self-record (`ConnectAndMigrate`/`ConnectAndVerify`),
  and any test helper — and name each against current `main`. State the **universal
  invariant**: *NO fingerprint / `schema_state` `complete` is ever written unless the
  FULL stored transcript (every step, applied and not) byte-matches the running binary,
  with the already-applied DB stamps verified.*
- Make both invariants **executable, named requirements** the falsifiers can verify
  against named code sites — not aspirational prose.

## Root reframe (do not lose this)

**Schema mutation must stop being an implicit side effect of the serving process's
restart and become an explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer.** Then the serving daemon can hold zero DDL
privilege, "restart force-commits a half-applied deploy" becomes impossible, and a
bad migration can never wedge the single writer on boot.

## The two Open Questions P4 must pin (from the RFC)

- **Q3 — How atomic is "atomic"?** Confirm the **per-step-atomic + resumable-cursor**
  contract is sufficient for every owner+runtime interleaving we actually ship, or
  specify the small set of steps that need a stricter sub-protocol. Every step must
  be idempotent and leave a coherent intermediate the cursor/transcript classifies
  as "incomplete, resume" — not "unknown drift, panic". **In v5 this explicitly
  includes the FULL-transcript binary/DB-stamp verification on resume AND before the
  finalizer (M1).** *(RFC: "This is the hard correctness core of P4.")*
- **Q4 — Should a deploy be a Striatum run?** Plain verb vs. a dogfooded run shape,
  with the bootstrapping paradox. Resolved in v1–v4 — carry forward.

## Load-bearing risks (attack these)

- **R1 atomicity/fingerprint-is-partly-a-lie:** non-transactional DDL, non-idempotent
  steps, a two-connection crash window, the receipt key moving with the frontier
  (BC-N1, fixed), or **a resume binary that disagrees with an already-applied step and
  self-records a hybrid as in-sync (M1)**. Test: kill-and-resume across each step class
  **and resume with a binary whose already-applied step bytes differ**.
- **R2 decoupling regresses a landed gate:** lifting `ApplyMigrations` must NOT break
  the P2 watermark interlock, the P3 drift gate / self-record, fresh-DB bring-up, or the
  BC-N2 universal edge / clean boot.
- **R3 DDL-revocation lockout / early REVOKE:** revoking serving-role DDL must not lock
  out the runtime path, **nor let any `owner-ddl apply` route — including the FMA-007
  self-heal reapply — commit 0021 early (M2)**.
- **R4 cursor / transcript / receipt / fingerprint holes:** double-apply/skip at a commit
  boundary; **a stored transcript that disagrees with the embedded binary or the DB
  stamps on resume (M1)**; out-of-order apply under the plan's edges; **a self-record
  path that writes `complete` without the full-transcript check (M1)**.

## Anchor verification against current `main` (operator pre-flight)

Verified against `~/git/striatum` @ `origin/main`. P0–P3 + P2 are **landed**; the
P4 surfaces are **NOT-FOUND (to be built)**. Treat as ground truth; re-anchor the
spec to these file:line references. **Re-confirm against your run's `main` HEAD.**

| Claim / target | Status | Anchor (current source) |
| --- | --- | --- |
| Boot-time auto-apply runs runtime migrations as `striatumd_rw` | **ACCURATE (the coupling P4 removes)** | `go/pkg/db/connection.go` `ConnectAndMigrate` → `ApplyMigrations`; P3 path / `RecordSchemaFingerprint` alongside. |
| **`ExpectedFingerprint()` hashes EMBEDDED FILE BYTES (the M1 mechanism)** | **ACCURATE** | `go/pkg/db/schema_drift.go:83-99` — `MigrationSHASet()` + each `OwnerBundle.SHA256()`, keyed by version. A binary with different bytes computes a different expected fingerprint. |
| **`LiveFingerprint` reads the self-recorded singleton — NO recompute (the M1 mechanism)** | **ACCURATE** | `go/pkg/db/schema_drift.go:145-160` — `SELECT fingerprint FROM striatumd.schema_state`; no recomputation from `schema_migrations`/catalog. `RecordSchemaFingerprint` UPSERTs the running binary's expected value `:171-194`; `CheckSchemaDrift` compares the singleton to this binary `:239-274`. |
| **`ApplyOwnerBundles` self-heal falls into `ReapplyAllOwnerBundles` on a cross-bundle dependency error (the M2 mechanism)** | **ACCURATE** | `go/pkg/db/owner.go:277-302` — `if isCrossBundleDependencyError(err) { ReapplyAllOwnerBundles(ctx, runner, bundles, daemonVersion) }`. |
| **`applyPendingOwnerBundles` bounds `<= current` (the only path v4 closed)** | **ACCURATE** | `go/pkg/db/owner.go:309-320` — `if bundle.Version <= current { continue }`. |
| **`ReapplyAllOwnerBundles` re-runs EVERY loaded bundle regardless of recorded version — no `<= Latest` bound, no 0021 skip (the M2 escape hatch)** | **ACCURATE** | `go/pkg/db/owner.go:324-349` — iterates the full `bundles` slice, `applyOneOwnerBundle` for each, "regardless of the recorded version". |
| **`OwnerBundles()` shared loader (the activation binary must surface 0021 through it for `revokeEmbedded`)** | **ACCURATE** | `go/pkg/db/owner.go:196-224`. The M2 fix splits the listing so the `owner-ddl apply` slice does NOT include 0021 even though `OwnerBundles()`/`revokeEmbedded`/`ExpectedFingerprint` do. |
| Owner bundle frontier = 0020 (`0020_owner_bundle_watermark_read.sql`, GRANT SELECT) — NOT the DDL-revoke bundle | **ACCURATE** | `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 20`, `:35` `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20). **The C3 DDL-revoke bundle stays renumbered at 0021.** |
| Runtime migration frontier (P3 added `schema_state`) | **ACCURATE** | `go/pkg/db/migrations.go` `LatestDaemonDBVersion`. New P4 runtime migration (`deploy_plan`/`deploy_cursor`) is **≥ 0044**. |
| P2 watermark interlock + clean halt | **ACCURATE (landed)** | `go/pkg/db/owner.go` `CheckOwnerBundleWatermark`, `AwaitingOwnerDDLError`; called **before** `ApplyMigrations` in boot. |
| `owner-ddl apply` applies bundles out-of-band as owner role | **ACCURATE (landed)** | `go/pkg/cli/localcommands/daemon.go` `runDaemonOwnerDDL` → `db.ApplyOwnerBundles()`. The renumbered DDL-revoke bundle is **excluded from every route** (M2). |
| `striatum daemon deploy` / `deploy_cursor` / `deploy_plan` / deploy receipt | **NOT-FOUND (P4 builds these)** | `go/pkg/cli/localcommands/daemon.go` subcommands have no `deploy`; no `deploy_cursor`/`deploy_plan` table. |

**Net design implication.** The P4 ground is clean: BC-N1/BC-N2/C1/C2/C3 are resolved
in v1–v4 and must carry forward unregressed. The remaining hard parts are **M1** — make
resume verify the ENTIRE stored transcript (already-applied bytes + DB stamps) against
the running binary and fire that same check before the finalizer self-records, so a
hybrid deploy is forced to `deploy_plan_binary_mismatch` and can never be recorded
`complete` — and **M2** — exclude the DDL-revoke bundle 0021 from EVERY `owner-ddl apply`
route including the FMA-007 self-heal reapply, with the embed/listing helper split. Plus
the proactive hardening audit naming every fingerprint and every owner-bundle apply path.
Be shadow-first: serve-boot auto-apply stays the default until the deployer is proven,
then flips behind a flag.

## Clearing condition (pin this)

The gate clears **only** if the cycle-5 adjudicator's collaboration ledger records a
clearing verdict (`accept` or `accept_with_findings`, never `clear`), which requires
**all** of: **M1 genuinely resolved** (full stored-transcript byte verification on
resume INCLUDING already-applied steps + DB-stamp verification; the SAME check before the
C1 finalizer self-records; ANY mismatch → `deploy_plan_binary_mismatch` DB-untouched;
`T-deploy-resume-already-applied-byte-mismatch-refuses` + the symmetric owner-step case),
**M2 genuinely resolved** (a single non-revoke filter excluding 0021 from EVERY
`owner-ddl apply` route incl. `ReapplyAllOwnerBundles` / the FMA-007 self-heal; the
embed/listing helper split; `T-deploy-revoke-excluded-from-reapply-self-heal`; extended
F12 / `G-revoke-last`), the **proactive hardening** present (every fingerprint and every
owner-bundle apply path named against current `main`, both universal invariants stated),
**BC-N1 + BC-N2 + C1 + C2 + C3 carried forward intact**, and **no new material challenge
standing unrebutted**. A spec that merely *claims* the two fixes without the concrete
full-transcript verification and the bound self-heal filter has NOT cleared the gate.
This is the single allowed revision cycle for v5 — a second `needs_revision` ends the
gate unCleared.

---
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run (v5 /
REVISION of `rfc-0142-p4-design-v4`; resolves the cycle-4 findings M1 + M2 and carries
BC-N1/BC-N2/C1/C2/C3 forward). Lanes: author=claude (holder/adjudicator/committer),
reviewer=codex (falsifiers).</sub>

## Operator sharpenings (pin these — do NOT let cycle-5 re-derive them)

- **M1: verify the FULL stored transcript, BEFORE any self-record.** The defect is
  that the v4 binary check only covered not-yet-applied steps, so a resume binary whose
  already-applied bytes differ slips through and the finalizer records the WRONG binary's
  byte-derived `ExpectedFingerprint()` as the durable "live" value (`schema_drift.go:83-99`,
  `:145-160`, `:171-194`). The fix is to validate EVERY stored step's `sha256` against the
  binary AND the already-applied DB stamps, classify ANY mismatch as
  `deploy_plan_binary_mismatch`, and run that same check before the finalizer writes
  `schema_state` / advances `finalizing → complete`. The reproducer cycle-5 falsifier_1
  MUST re-run: kill after step 0 commits, resume with a binary whose step-0 bytes differ
  but whose remaining steps match, and prove `deploy_plan_binary_mismatch` with NO apply,
  NO fingerprint write, NO `complete`.
- **M2: bind the SELF-HEAL branch, not just the pending loop.** v4 closed only
  `applyPendingOwnerBundles` (`owner.go:309-320`); the FMA-007 `ReapplyAllOwnerBundles`
  branch (`owner.go:277-302,324-349`) re-runs every loaded bundle regardless of version and
  can commit 0021 early. The fix is a single non-revoke filter (`>=
  DDLRevokeOwnerBundleVersion` excluded, regardless of recorded version) bound across
  `applyPendingOwnerBundles`, `ReapplyAllOwnerBundles`, `ApplyOwnerBundles`, tests, and
  dry-run/list, plus splitting the embed/listing helper so 'embeds 0021' ≠ 'owner-ddl apply
  iterates 0021'. The reproducer cycle-5 falsifier_2 MUST re-run: force the cross-bundle
  self-heal with 0021 loaded and prove 0021 is NOT applied and CREATE stays held.
- **Do the proactive hardening audit ONCE.** Name every owner-bundle apply path and every
  fingerprint/self-record path against current `main` and state the two universal
  invariants as executable, named requirements — so cycle-5 cannot spawn an M3 from an
  unaudited sibling path.
- **Carry forward unregressed.** BC-N1's moving-frontier fix, BC-N2's universal edge, the
  C1 finalizer, the C2 edge (`RequiredOwnerBundleVersion = 20`, forward-watermark at
  `applied >= 21`), and the C3 revoke-last mechanism (0021 special-cased + terminal) must
  all stay intact. M1 ADDS to the binary-verification clause; M2 binds the self-heal branch;
  neither re-opens a resolved mechanism.

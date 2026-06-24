---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
author: "committer-author-001"
title: "RFC 0142 P4 one-shot deployer (decouple schema-apply from serve-boot) — falsification-cleared implementation spec"
run_id: "run_365daa96ebcaa61f7b33175cdf3e9abe"
cycle: 1
inputs:
  - "docs/operator/workflows/rfc-0142-p4-design-v9/SEED.md"
  - "docs/rfcs/0142-safe-by-construction-database-change-deployment.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/falsifier_1/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/falsifier_2/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/holder/HOLDER.md"
  - "docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md"
---

# PROPOSAL — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: committer-author-001

> This is the **committed deliverable** of the RFC 0142 P4 design run (v9 / REVISION):
> the falsifiable, build-ready implementation spec the `rfc-0142-p4-build` run will
> execute contract-first (TDD). It is the v9 Holder's `HOLDER.md` published as the
> committed proposal after the cycle-1 collaboration ledger **cleared the gate**
> (`accept_with_findings`), with the adjudicator's two non-blocking build-phase
> obligations (finding **B1**) folded in as **binding build-run acceptance criteria**
> (§6.5). RFC 0142 is **accepted (D258)**; this does NOT re-open the five-layer design —
> it pins **P4 only**: the one-shot deployer that lifts schema mutation out of serve-boot,
> plus the serve-boot decoupling and the serving-role DDL revocation. Read `SEED.md` and
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (Phasing P4 row,
> Open Questions 3 + 4, "the hard correctness core of P4") first; this spec re-anchors the
> RFC's P4 sketch to current source. **P5 (rehearsal / expand-contract / fidelity tiering /
> clone = Q1/Q2) is explicitly deferred (§7).**

## Gate disposition (why this is committed, and what it folds)

The cycle-1 collaboration ledger
(`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`)
returned **`accept_with_findings`** — a **clearing** verdict. The single binding cycle-1
finding **M7** (the §3.5/§4.5/F18 row-16 cell `complete`/decoupled/revoke-embedding asserted
unconditionally where A's decoupled complete branch is fingerprint-conditional and reads
neither `applied_owner` NOR `revokeEmbedded`) is **genuinely resolved** via the prescribed
**Option 1** (the clean fix parallel to the M6 fix), and **both** falsifiers — from the two
independent lenses that converged on M7 in v8 (falsifier_1 decoupling-boundary/decision-table;
falsifier_2 carry-forward/regression) — **independently concede** the resolution and find **no
carry-forward regression and no new material design gap**. The adjudicator **source-verified**
the orthogonality and watermark anchors against the run worktree HEAD (`git diff --stat
3f9d5734 HEAD -- go/pkg/db/` **EMPTY** — every `owner.go` / `schema_drift.go` / `connection.go`
anchor holds byte-identically).

**This spec therefore carries all twelve fixes as first-class, build-ready design** — the
cycle-1 fix **M7** resolved this cycle, and the eleven earlier fixes **M6 (rows 13/15),
M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, C3** carried forward verbatim and unregressed
— plus the **proactive-completeness boot-path decision table** (every `cursorState ×
decoupledEnabled × revokeEmbedded × applied_owner ∈ {0, 1..19, ==20, >=21}` cell derived
mechanically from W and A; all seven A-reaching complete-row cells conditional on A's
fingerprint-sync predicate; Universal Invariant B proven in each; §4.5 and the F18 spy list
consistent at four `:399`-reaching cells; F18 parametric over all complete-row cells with the
in-sync/out-of-sync sub-dimension; the legitimate fresh-DB cells still serving).

**Finding B1 (two non-blocking build-phase obligations) is folded into §6.5 as binding
acceptance criteria** the `rfc-0142-p4-build` run MUST honor:
1. **F18 (`T-deploy-bootpath-decision-table`) must actually construct the row-16 in-sync AND
   out-of-sync sub-cases** for `applied_owner == 0`, `==20`, and `>=21` (the in-sync arm
   independently setting `schema_state.fingerprint == ExpectedFingerprint()` AND
   `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent / 20 / `>=21` DB, proving
   the orthogonality) and assert the in-sync row-16 cells **serve verify-only without firing
   the `ApplyMigrations`/`RecordSchemaFingerprint` spies**. Omitting these recreates M7 in code.
2. **Expand the grouped cursor-state shorthand.** The "64-cell" shorthand groups
   `step_committed` with `in_progress` and `aborted` with the non-complete edge (16 row-groups
   × 4 buckets); the executable test MUST table-drive each **concrete** cursor-state enum named
   by F18 (`none`/`in_progress`/`step_committed`/`finalizing`/`complete`/`aborted`), since the
   implementation operates on the enum, not the prose group label.

---

## 0. Root reframe (held) — and its FINAL complete-row coherence edge (M7)

Schema mutation must stop being an **implicit side effect of the serving process's restart** and
become an **explicit, ordered, resumable, provenance-tracked operation owned by a dedicated
deployer**. Then the serving daemon holds **zero create-DDL on the serving path**, "restart
force-commits a half-applied deploy" becomes structurally impossible, and a bad migration can
never wedge the single writer on boot. **M3 closed** the legacy `ApplyMigrations`-on-serve-boot
edge for a revoke-embedding binary / a transcript with a pending change. **M5 closed** the fresh
/ single-role / no-authority bootstrap edge (`applied_owner == 0` serves, row 1). **M6 closed**
the no-revoke `complete` rows (13/15) by deriving them from the `applied_owner`-independent A
predicate. **M7 is the FINAL complete-row coherence edge of the same reframe** — the whole
reframe leans on the boot-path decision table (§3.5) + its proof (§4.5) + its oracle (F18) being
internally consistent and **fully derived from the two boot predicates W and A**, so a build
cannot exploit a predicate/table mismatch to smuggle in an unstated guard. M6 left the SIBLING
cell — row 16 (revoke-embedding, decoupled) — ASSERTED unconditionally from an owner-watermark
reachability inference A neither reads nor verifies; **v9 derives row 16 from A under the SAME
contract that fixed rows 13/15** ("serve verify-only if in-sync, else `awaiting_deploy`"), so
EVERY A-reaching complete-row cell is a consequence of the predicates, the class is closed by
derivation, and F18 is the complete oracle it claims to be.

**Preserved asymmetry (load-bearing).** The M7 fix is **local to row 16 (the decoupled
revoke-embedding `complete` cell) and the F18 parametric extension**. It must NOT (and does not)
re-collapse the row-1 fresh-DB serve (cell 1/`==0` still serves); regress the M6 rows-13/15
conditional cells; weaken the M3 config gate (cells 2/6/10/14 still halt `awaiting_deploy_config`
at A0 in every column that passes W incl. `0`); regress the BC-N2 `applied_owner == 20` edge; or
advance `RequiredOwnerBundleVersion` (stays 20) or alter the watermark.

---

## 0.1 Addressing the design findings (the auditable resolution map — all twelve fixes)

The single binding cycle-1 finding (M7) is resolved by a concrete derivation bound to named code
sites — not a claim. M6 (rows 13/15), M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 are
carried forward **verbatim** and re-confirmed unregressed (both falsifiers concede each; the
adjudicator source-verified the M5 serve, the `Required = 20` watermark, and the M6/M7
orthogonality anchors). The "where" column names the section that specifies the fix and the named
test that would refute it.

| Finding | Disposition | Where / refuting test |
| --- | --- | --- |
| **M7** row 16 (`complete`, decoupled, revoke-embedding) not derived from A | **RESOLVED via Option 1 (derive row 16 from A).** §0.2 adds the **DERIVATION-RULE-COMPLETENESS** sub-invariant (every A-reaching complete-row cell whose A outcome is fingerprint-conditional is WRITTEN conditionally) and the **A-`revokeEmbedded`-independence of the decoupled complete branch** (once decoupled is ON, step 0 is skipped and A's complete branch reads neither `applied_owner` NOR `revokeEmbedded`, so rows 15 and 16 take the IDENTICAL A3 outcome in every W-passing column). §3.5 row 16 `==0`/`==20`/`>=21` become conditional — "SERVE-verify if in-sync, else `awaiting_deploy`" — matching row 15; the normal pre-0021 `==0`/`==20` state is OUT-OF-SYNC (→ `awaiting_deploy`, dominant), the normal post-0021 `>=21` state IN-SYNC (→ serve-verify, dominant); the opposite sub-case is the degenerate corner A handles. F18 is PARAMETRIC over the seven A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`}, constructing BOTH sub-cases. The four `:399`-reaching cells are UNCHANGED (row 16 is decoupled → `ConnectAndVerify` never self-records). | §0.2, §3.3a step 3, §3.5 (row 16 conditional + cross-row audit), §1.3, §4.5 / **`T-deploy-bootpath-decision-table` (F18, parametric over the 7 A-reaching cells) + `T-deploy-fresh-db-bootstrap-serves` (F18a)** |
| **M6** rows 13/15 derived from the `applied_owner`-independent A predicate | **CARRIED FORWARD VERBATIM, unregressed — and EXTENDED to row 16 by the M7 derivation.** Rows 13/15 `==0` stay conditional == `==20`; §0.2 invariant intact; the four `:399`-reaching cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} enumerated identically in §4.5 AND the F18 spy list; the cross-row audit intact. M7 did NOT re-collapse rows 13/15. | §0.2, §3.5, §4.5, §3.3a / **F18 (rows-13/15 arms unchanged) + F18a** |
| **M5** the `applied_owner` split `{0/no authority, 1..19 shortfall, ==20, >=21}`; cell 1/`==0` serves fresh-DB | **CARRIED FORWARD VERBATIM at row 1, unregressed.** W = `CheckOwnerBundleWatermark`: `applied_owner == 0` → nil, proceed to A (`owner.go:145`); `1..19` → `awaiting_owner_ddl` (`:148-150`); `==20` → nil (`:151-153`); `>=21` → barrier b (no-revoke) / tolerate-forward (revoke). Cell 1/`==0` SERVES-legacy; cell 1/`==20` inert-landing. M7 does NOT re-collapse it. | §3.5 (`==0` column row 1), §0.2, §8 / **F18 + F18a + F11(e)/(f)** |
| **M3** the COMPLETE-cursor activation guard (hoisted step-0 config gate + no-revoke complete pure-read) | **CARRIED FORWARD VERBATIM, unregressed.** `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` HOISTED to step 0, firing for EVERY cursor state incl. `complete` (barrier a). M7 is in the DECOUPLED revoke cell (row 16) where step 0 does NOT fire — it does NOT re-open the legacy bypass (row 16 is decoupled, never reaches `:399`); cells 2/6/10/14 still halt `awaiting_deploy_config` at A0. | §3.3a step 0, §3.5 (cells 2/6/10/14), §4.5 / **F17 + F11(g) + F18** |
| **M4** the phase-aware F16 split | **CARRIED FORWARD VERBATIM, unregressed.** F16a (synthetic-list / injectable-FS, step 2) drives the exclusion filter WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (production, step 7) asserts the embed/listing split + the forced FMA-007 self-heal pgtest via `isCrossBundleDependencyError` (`owner.go:367-374`). M7 does not touch F16. | §3.2a, §6 / **F16a + F16b + extended F12/`G-revoke-last`** |
| **M1** full stored-transcript byte + DB-stamp verification on resume AND finalizer step 0 | **CARRIED FORWARD VERBATIM, unregressed.** `VerifyStoredTranscript(plan_hash)` (§3.4a) checks the ENTIRE transcript `sha256` vs the binary's bytes ⇒ `deploy_plan_binary_mismatch`; already-applied entries also verify the DB stamps ⇒ `deploy_plan_db_stamp_mismatch`. M7 chose Option 1 (the conditional cell), NOT Option 2 (a new boot-time DB-stamp guard) — M1's scope (deploy resume / finalizer step 0, NOT serve-boot) is unchanged and NOT extended. | §3.4a, §1.2, §1.3, §3.4 / **F15 + F14** |
| **M2** the single non-revoke filter across EVERY `owner-ddl apply` route | **CARRIED FORWARD VERBATIM, unregressed.** `isNonRevokeBundle` + `OwnerDDLApplyBundles()` + in-loop guards + the nil-fallback split + the embed/listing split. M7 does not touch the filter. | §3.2a, §4.4, §4.5 (Invariant A) / **F16a/F16b + extended F12/`G-revoke-last`** |
| **BC-N1** immutable `deploy_plan` transcript materialized before step 0; resume off the stored transcript | **CARRIED FORWARD VERBATIM, unregressed.** Migration 0044 `deploy_plan` keyed by `plan_hash`; materialize in the same tx as `in_progress(0)` before step 0; resume never recomputes `BuildPlan`; §1.3 + transcript-enumerated doctor + F14. | §1.2, §1.3, §3.2 / **F14** |
| **BC-N2** universal `revokeEmbedded`-independent `CheckDeployActivation` edge halting non-`complete` cursors at `applied_owner == 20` | **CARRIED FORWARD VERBATIM, unregressed.** The universal incomplete-deploy edge halts `awaiting_deploy` DB-untouched on every non-`complete` cursor. M7 is in the `complete`/decoupled cell (row 16), NOT the non-complete edge — cells 5/7/9/11 unchanged. | §3.3a step 1, §3.5 (rows 5/7/9/11), §4.2 / **F11(e)/(f) + extended `G-old-binary-refuse`** |
| **C1** the `finalizing` state + idempotent finalizer + §1.3 row + the M1 pre-finalizer gate | **CARRIED FORWARD VERBATIM, unregressed.** M7 touches neither the finalizer nor the legacy writer's enforcement. | §1.2, §1.3, §3.4 / **F10 + F15 pre-finalizer arm** |
| **C2** `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion` KEPT at 20 | **CARRIED FORWARD VERBATIM, unregressed.** M7 keeps the forward-watermark rule at `applied_owner >= 21` (barrier b) and `RequiredOwnerBundleVersion = 20` (`owner.go:35`) — does NOT advance `Required`, alter the watermark, or change the `applied >= 21` forward rule. | §3.3a, §4.2, §4.3, §3.5 / **F11** |
| **C3** the DDL-revoke bundle 0021 special-cased + excluded from `owner-ddl apply` + applied terminal | **CARRIED FORWARD VERBATIM.** 0021 special-cased out of the owner prefix, applied terminal; `DDLRevokeOwnerBundleVersion = 21`; `LatestOwnerBundleVersion`/`Required` stay 20. M7 arose BECAUSE the v8 table imported the C3 reachability fact into A without A verifying the owner-bundle stamp; the v9 fix makes the cell conditional on fingerprint-sync — the correct derivation from A — so C3 itself is not regressed. | §3.2, §3.2a, §3.3b, §4.4 / **F12 + `G-revoke-last`** |

**Carried forward unchanged (do not re-litigate):** Q4 (plain verb + three seams, §2); the
per-step Q3-A/Q3-B mid-step resumability **body** + step taxonomy (§1.1); the shadow-first
decoupling shape (default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2
watermark interlock + P3 drift gate, the embed-FS-derived plan, the hash-chained receipt,
`doctor schema_deploy_unrecorded`).

---

## 0.2 The load-bearing mechanical-derivation invariant (the heart of the M6 AND M7 fixes)

> **INVARIANT W→A-INDEPENDENCE.** *On every boot, the two guard sites run in order — W =
> `CheckOwnerBundleWatermark` then A = `CheckDeployActivation`. **Once W has returned nil
> (`applied_owner ∈ {0, ==20}`, or `>=21` for a revoke-embedding binary), A is
> owner-watermark-independent: it reads `cursorState`, `decoupledEnabled`, `revokeEmbedded`,
> `cursor.plan_hash`, and the recorded `schema_state.fingerprint` — and NEVER reads
> `owner_bundle_meta` / `applied_owner`.** Therefore, for any fixed `(cursorState,
> decoupledEnabled, revokeEmbedded)` row, the `==0` and `==20` columns have IDENTICAL A-gate
> outcomes in EVERY cursor row.*

> **SUB-INVARIANT A-`revokeEmbedded`-INDEPENDENCE OF THE DECOUPLED COMPLETE BRANCH (the heart of
> the M7 fix).** *Step 0 of A (the M3 config gate `revokeEmbedded && !decoupledEnabled →
> awaiting_deploy_config`) is the ONLY A predicate that reads `revokeEmbedded`, and it does NOT
> fire when `decoupledEnabled == true`. So on the decoupled path, A's `complete`-cursor decision
> (§3.3a step 3, `decoupledEnabled == true` branch) reads NEITHER `applied_owner` NOR
> `revokeEmbedded` — only `cursor.plan_hash` and the recorded `schema_state.fingerprint`.
> Therefore the no-revoke decoupled complete row (15) and the revoke-embedding decoupled complete
> row (16) take the IDENTICAL A3-conditional outcome — "serve verify-only if in-sync, else
> `awaiting_deploy`" — in EVERY W-passing column; they differ ONLY at W, in the `>=21` column
> (no-revoke → barrier b halt; revoke-embedding → tolerate-forward to A).*

> **SUB-INVARIANT DERIVATION-RULE COMPLETENESS (what M7 enforces).** *Every A-reaching
> complete-row cell whose A outcome is conditional on the fingerprint-sync state is WRITTEN
> conditionally ("serve if in-sync, else `awaiting_deploy`"); NO complete-row cell is asserted
> from an owner-watermark reachability inference (e.g. `owner_bundle_meta < 21 ⟹ fingerprint
> mismatch`). The v8 table obeyed this for rows 13/15 but violated it for row 16; v9 obeys it for
> ALL seven A-reaching complete-row cells.*

These are mechanically forced by the source (verified by the falsifiers and the adjudicator
against the named sites):

- **A reads the fingerprint, not the owner watermark, and not `revokeEmbedded` on the decoupled
  complete branch.** A's `complete`/decoupled decision (§3.3a step 3) is `cursor.plan_hash ==
  expected` AND `LiveFingerprint(recorded) == ExpectedFingerprint()`. `LiveFingerprint` reads the
  recorded `striatumd.schema_state.fingerprint` singleton (`schema_drift.go:145-161`) — a SEPARATE
  table from `owner_bundle_meta`; no `applied_owner` input. `ExpectedFingerprint()` hashes the
  binary's embedded file bytes (`schema_drift.go:83-100`) — also independent of `applied_owner`.
  `RecordSchemaFingerprint` writes only the `schema_state` singleton (`schema_drift.go:171-195`).
  So **`schema_state` (what A reads on the decoupled complete branch) is orthogonal to both
  `owner_bundle_meta` (= `applied_owner`, what W reads) and the `revokeEmbedded` build flag.**
- **Mechanical consequence — the full 64-cell table is DERIVED, not asserted.** Pick W's outcome
  from the `applied_owner` bucket and `revokeEmbedded` (`0`/`==20` → pass to A; `1..19` →
  `awaiting_owner_ddl`; `>=21` → barrier b for no-revoke, tolerate-forward to A for
  revoke-embedding), then for the W-passing buckets apply the SINGLE A predicate (§3.3a) — whose
  decoupled-complete outcome is identical for `==0`, `==20`, and (for a revoke-embedding binary)
  `>=21`, and identical between rows 15 and 16. The `1..19` column (W halts before A) and the
  `>=21`-no-revoke column (barrier b at W) are the ONLY columns where the outcome legitimately
  differs from `==0`/`==20`, and they differ at **W**, not at A.

---

## 1. Decision Q3 — How atomic is "atomic"? (the hard correctness core)

**Claim (Q3-DECISION).** The **per-step-atomic + resumable-cursor** contract is sufficient for
*every* owner+runtime interleaving AND every boot-path combination P4 ships, **provided**: (a)
plan identity is an **immutable durable fact materialized before step 0** and resume reads it
(BC-N1); (b) the two per-step sub-rules (Q3-A/Q3-B) hold for both the schema side effect AND the
per-step receipt, keyed on the **stored transcript's** `(plan_hash, step_index)`; (c) the
finalization boundary is closed by a distinct `finalizing` state + an idempotent finalizer (C1);
(d) **no boot can serve while the cursor is non-`complete`** (BC-N2); (e) on every resume AND
before the finalizer self-records, the ENTIRE stored transcript byte-matches the running binary
and the already-applied DB stamps match the transcript; any mismatch halts the deploy
DB-untouched and NO fingerprint is written (M1); (f) NO boot reaches the legacy
`ApplyMigrations`/`RecordSchemaFingerprint` mutate+self-record path while a deploy transcript with
a pending change is present, and NO revoke-embedding binary EVER reaches that legacy path — the
activation guard is the sole authority for "may this boot auto-apply/self-record on serve-boot",
evaluated before `ApplyMigrations` in both boot paths (M3); (g) the owner-watermark gate W is
fail-closed for a genuine authority shortfall (`1 <= applied_owner < 20` → `awaiting_owner_ddl`)
AND fail-OPEN for the legitimate fresh / single-role / no-authority bootstrap (`applied_owner ==
0` → serve) (M5); (h) the decision is provably correct in EVERY `cursorState × decoupledEnabled ×
revokeEmbedded × applied_owner ∈ {0, 1..19, ==20, >=21}` cell because the table is DERIVED from W
and A, with A owner-watermark-independent (§0.2), so the `==0` and `==20` columns are identical in
EVERY cursor row (M6); **and (i) the derivation is COMPLETE across the whole `complete`
cursor-state group (M7) — A's decoupled complete branch reads neither `applied_owner` NOR
`revokeEmbedded`, so rows 15 and 16 take the IDENTICAL A3-conditional outcome and row 16's
`==0`/`==20`/`>=21` cells are WRITTEN conditionally ("serve verify-only if in-sync, else
`awaiting_deploy`"), closing the last asserted-not-derived sibling cell — F18 parametric over all
seven A-reaching complete-row cells.**

- **Q3-A / Q3-B** (resumability body; receipt keyed off the stored transcript). See §1.1 / §3.3b.
- **Q3-V — full-transcript verification (M1).** `VerifyStoredTranscript(plan_hash)` (§3.4a) on
  every resume and as finalizer step 0.
- **Q3-W — boot-path activation completeness (M3 + M5 + M6 + the M7 complete-row closure).** On
  EVERY boot, W then A, BOTH before any mutation or self-record. §3.5 DERIVES the decision across
  the whole cell space, proves it fail-closed AND that the legitimate fresh-DB cell still serves:
  the only cells that reach the legacy `:399` writer are **1/`==0`**, **1/`==20`**
  (`cursorState == none`, no transcript), **13-in-sync/`==0`**, and **13-in-sync/`==20`**
  (`complete`-and-already-in-sync no-revoke: an idempotent rewrite of the same fingerprint); **row
  16 is decoupled and reaches `:399` in NO column (M7).**

### 1.1 Step taxonomy (the closed set P4 ships)

| Class | Examples | Transactional? | Protocol | Idempotent form |
| --- | --- | --- | --- | --- |
| **T-DDL** runtime | `CREATE TABLE`, additive `ALTER … ADD COLUMN`, `CREATE INDEX` (non-concurrent), new runtime migrations | Yes | **Q3-A** (step + ownership-reconcile + version stamps + cursor + **receipt** in one tx) | `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`; re-run is a no-op |
| **T-DDL** owner | owner-bundle DDL, **incl. the terminal DDL-revoke bundle 0021** | Yes (each bundle applies in one tx that stamps `owner_bundle_meta` last — `owner.go:511-541`) | **Q3-A**, reusing the per-bundle transaction; cursor advance + **receipt** join that tx | bundles idempotent (`IF NOT EXISTS` + `ON CONFLICT (version) DO NOTHING`) |
| **NT-DDL** | `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, `DROP INDEX CONCURRENTLY` | No | **Q3-B** (pre-marker → step → reconcile: keyed receipt + post-marker) | `CREATE INDEX CONCURRENTLY IF NOT EXISTS`; resume reconciler drops a left-behind `INVALID` index then recreates; `ADD VALUE … IF NOT EXISTS` |
| **DML-backfill** (bounded in P4; full online backfill is P5) | a one-shot data normalize a runtime migration carries | Yes if small; batched otherwise | Q3-A small / Q3-B batched | `WHERE <not-yet-done>` predicate; re-run resumes from the unprocessed tail |

> The terminal DDL-revoke bundle 0021 is a T-DDL owner step (the existing per-bundle transaction,
> `owner.go:511-541`), so its receipt commits atomically. NT-DDL never creates a new ownable object
> whose owner is wrong, so the C3 ownership reconciliation (§3.3b) is needed only on transactional
> object-creating steps.

### 1.2 `deploy_cursor` + the immutable `deploy_plan` transcript (BC-N1) and crash-resume (C1)

Both are **new runtime-owned tables**, created by runtime migration **0044** (modeled
byte-for-byte on `0043_schema_state.sql:39-52`). Migration 0044 is additive, runtime-owned,
carries no owner DDL.

**`deploy_cursor` (singleton):** `id text PK CHECK (id='singleton')`, `plan_hash text NOT NULL`,
`state text NOT NULL`, `step_index int NOT NULL`, `step_id text NOT NULL`, `updated_at`.
**`deploy_plan` (append-only, keyed by `plan_hash`):** the IMMUTABLE ordered transcript `steps jsonb`
= `[{step_index, step_id, role, sha256, transactional}]`, revoke last, `revoke_step_index`,
`base_owner_version`, `base_runtime_version`, `target_*`, INSERT-once (`ON CONFLICT (plan_hash) DO
NOTHING`).

`state ∈ { idle, in_progress, step_committed, finalizing, complete, aborted }`. Lifecycle:

```
materialize(plan_hash, transcript) + idle─▶in_progress(0) ─▶ step_committed(0) ─▶ … ─▶ step_committed(N-1=0021) ─▶ finalizing ─▶ complete
   [deploy_plan row + cursor=in_progress(0) in ONE tx, BEFORE step 0]   │
                                                                         └─(operator abort / fatal)──▶ aborted
```

- **Materialization (BC-N1).** After the substrate-ensure preamble (§3.2), compute
  `BuildPlan(base_owner, base_runtime)` once, derive `plan_hash`, and in one transaction INSERT the
  immutable `deploy_plan` row and set `deploy_cursor → in_progress(0)` — before step 0.
- **Resume verification (M1).** On every resume, immediately after loading
  `deploy_plan[cursor.plan_hash]` and BEFORE applying or finalizing any step, run
  `VerifyStoredTranscript(plan_hash)` (§3.4a) over the WHOLE transcript.
- **Finalization (C1 + the M1 pre-finalizer gate).** When the terminal step is `step_committed`,
  advance to `finalizing`, then run the idempotent finalizer (§3.4): **(0)** run
  `VerifyStoredTranscript` and abort on mismatch (writing NOTHING); **(1)** append the guarded
  `complete` receipt; **(2)** `RecordSchemaFingerprint` UPSERT; **(3)** advance `finalizing →
  complete` last. The deployer finalizer is the C1 writer of `schema_state` on the deploy path —
  gated by step (0); the LEGACY `connection.go:399` writer is governed by the W (M5) + A (§3.3a,
  M3) gates, proven over the whole cell space in §3.5.

### 1.3 The load-bearing disambiguation (C1 + BC-N1 + BC-N2 + M1 + M3 + the M6/M7 owner-watermark- AND `revokeEmbedded`-independence note)

`deploy_cursor` is the **second signal** alongside the drift gate's `LiveFingerprint ==
ExpectedFingerprint`. **M6/M7 note:** every row keys on `deploy_cursor.state` and the
binary/fingerprint facts (the A dimension), which are **orthogonal to `applied_owner` (the W
dimension)** — `LiveFingerprint` reads `schema_state`, never `owner_bundle_meta`
(`schema_drift.go:145-161`). On the DECOUPLED path A's complete branch is ALSO orthogonal to
`revokeEmbedded` (§0.2 sub-invariant). So the `complete`/in-sync row classifies "serve"
**regardless of the `applied_owner` bucket AND regardless of whether the binary embeds the
revoke** (`==0`, `==20`, and `>=21` all land in the SAME row; the no-revoke decoupled binary
(row 15) and the revoke-embedding decoupled binary (row 16) land in the SAME row), and the
`complete`/pending-change row classifies `awaiting_deploy` regardless of bucket/embed.

| `deploy_cursor.state` | condition (vs the **stored transcript** + the binary) | classification | action |
| --- | --- | --- | --- |
| `complete` | full transcript byte-matches binary; `== ExpectedFingerprint`; **`decoupledEnabled` (or a no-revoke binary already in-sync)** — **independent of `applied_owner` AND, on the decoupled path, of `revokeEmbedded`** | **in sync** | **serve** (verify-only when decoupled — incl. the revoke-embedding decoupled binary, row 16; legacy no-op-apply when a no-revoke flag-OFF binary is already in-sync) |
| `complete` | **`revokeEmbedded == true` AND `decoupledEnabled == false`** (M3) | **config halt** | refuse-to-serve **`awaiting_deploy_config`**, DB untouched — never reaches `ApplyMigrations`/`:399` |
| `complete` | **decoupled binary (row 15 OR row 16), but `ExpectedFingerprint() ≠` recorded OR `cursor.plan_hash ≠ expected`** (A3 decoupled comparison) — **independent of `applied_owner` and `revokeEmbedded`** | **pending change over a transcript** | refuse-to-serve **`awaiting_deploy`**, DB untouched (M7: the normal pre-0021 row-16 state is OUT-OF-SYNC and lands here) |
| `complete` | **no-revoke binary, flag OFF, but `ExpectedFingerprint() ≠` recorded OR `cursor.plan_hash ≠ expected`** (M3 comparison) — **independent of `applied_owner`** | **pending change over a transcript** | refuse-to-serve **`awaiting_deploy`**, DB untouched — never auto-applies on serve-boot |
| `in_progress` / `step_committed` | full transcript byte-matches binary AND already-applied DB stamps match | **incomplete, resume off the STORED transcript** (BC-N1) | refuse-to-serve `awaiting_deploy`; `deploy` resumes at `cursor.step_index` |
| `in_progress` / `step_committed` | present but the cursor's `plan_hash` ≠ the binary's freshly-computed pending plan | **incomplete, resume with the STORED plan** (BC-N1) | refuse-to-serve `awaiting_deploy`; resume off the stored transcript by `cursor.plan_hash` — NOT drift |
| `finalizing` | full transcript byte-matches binary | **resumable finalization** (C1) | refuse-to-serve `awaiting_deploy`; re-run runs the idempotent finalizer (which re-verifies, §3.4 step 0) |
| any in-flight, `deploy_plan[plan_hash]` ABSENT, **OR ANY stored step `sha256` ≠ the binary's embedded bytes** (M1) | full-transcript mismatch | **`deploy_plan_binary_mismatch`** | refuse-to-serve; apply NOTHING; operator inspects |
| any in-flight, transcript byte-matches, **but an already-applied step's DB stamp ≠ the stored transcript** (M1) | DB-stamp mismatch | **`deploy_plan_db_stamp_mismatch`** | refuse-to-serve; do NOT finalize; apply NOTHING |
| absent `deploy_cursor` table / `state == idle` (NO transcript) | n/a | **no deploy in flight** | **legacy path (flag OFF) + no-revoke binary:** serve (auto-apply, the legitimate `:399` cell — **incl. the fresh `applied_owner == 0` bootstrap, M5, AND the inert `applied_owner == 20` re-boot — the SAME serve, since A does not read `applied_owner`**); **decoupled (flag ON) OR revoke-embedding binary:** refuse `awaiting_deploy`/`awaiting_deploy_config` (C2 + M3 — §3.3a) |
| `aborted` | n/a | **aborted** | refuse-to-serve `awaiting_deploy` |
| `complete`, `plan_hash` ≠ expected (foreign) | foreign | **genuine drift** | refuse-to-serve (`schema_drift`) |

Load-bearing properties: plan identity is a stored fact and resume reads
`deploy_plan[cursor.plan_hash]` (BC-N1); a resume/finalize binary that disagrees with ANY stored
step is forced to a typed mismatch and never writes a fingerprint (M1); `complete` ⇒ 0021 applied
(C1); incomplete-never-serves holds for EVERY deployer-aware binary (BC-N2); a `complete` cursor
no longer short-circuits the activation guard (M3). **M6/M7: every row is `applied_owner`-blind,
AND on the decoupled path `revokeEmbedded`-blind in the complete branch, so the §3.5
`==0`/`==20`/`>=21` columns produce identical classifications and rows 15 and 16 are the same
conditional.**

---

## 2. Decision Q4 — Is a deploy itself a Striatum run? (plain verb, run-shape door held open)

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape door held open by
three seams.** The bootstrapping paradox is dispositive (a dogfooded run needs the daemon + schema
up, which is exactly what the deploy brings online), so the base deployer is a plain verb against a
bare/owner-only DB, the same context `daemon owner-ddl apply` (`daemon.go:90`) occupies. The door
stays open because (1) the plan is a content-addressed durable artifact (`plan_hash` +
`deploy_plan`); (2) the receipt is hash-chained into `audit_log`; (3) the verb is a thin CLI over
a `Deployer.Apply(ctx, plan, cursor)` core a future run step invokes unchanged.

---

## 3. The deployer surface + serve-boot decoupling (shadow-first, by exact code site)

### 3.1 Command site

Add `case "deploy":` to `RunDaemon` (`daemon.go:67-81`) → `runDaemonDeploy(...)`. DSN resolution
mirrors `runDaemonOwnerDDL` (`daemon.go:90,115`). Flags: `--plan`, `--dry-run`, `--resume`
(default), `--abort`, `--json`. `docs/reference/command-authority-matrix.md` + the authority
guardrail tests gain the `daemon.deploy` row.

**Connection model (load-bearing).** The deployer applies **every deploy-plan step over the SINGLE
owner connection**. The only write routed over a separate runtime view is the C1 finalizer's
terminal `schema_state` self-record (mirroring `connection.go:399`) — gated by
`VerifyStoredTranscript` (§3.4a, M1).

### 3.2 The deploy plan — materialized ONCE into an immutable transcript (BC-N1), revoke 0021 sorted last (C3)

Plan identity is computed once at deploy start from the two embedded `embed.FS` sources the
fingerprint hashes (runtime `migrations.go:21`; owner `owner.go:156`) and persisted as the
immutable `deploy_plan` transcript before step 0. The substrate-ensure preamble applies 0044
idempotently before transcript materialization (0044 is never a numbered step). `BuildPlan` uses
the FULL `OwnerBundles()` loader (includes 0021) but special-cases the revoke to terminal: pending
non-revoke owner bundles (≤20, via `isNonRevokeBundle`) → pending runtime migrations (≥0045) → the
terminal 0021 if pending. `plan_hash` = sha256 of the canonical transcript (base + ordered
`step_id`+`sha256`, 0021 last); the fingerprint is over the *set* (order-independent) so 0021-last
changes `plan_hash` but not the fingerprint. `step_index` is stable by storage, not recomputation
(BC-N1).

### 3.2a The single non-revoke filter + the embed/listing split (M2) + the M4 phase-aware test staging

**Claim (NON-REVOKE-FILTER — M2).** The DDL-revoke bundle 0021 is excluded from **every**
`owner-ddl apply` route by a single named filter and a split loader, so 0021's `REVOKE CREATE` can
ONLY ever be committed as the terminal `deploy` step (via `applyOneOwnerBundle`), NEVER via the
pending loop, the FMA-007 self-heal reapply, a nil-fallback, a dry-run/list surface, or a test
helper.

```go
// DDLRevokeOwnerBundleVersion identifies the RFC 0142 P4 C3 DDL-revoke bundle.
// DEPLOY-PLAN-TERMINAL ONLY. LatestOwnerBundleVersion / RequiredOwnerBundleVersion stay 20.
const DDLRevokeOwnerBundleVersion = 21
func isNonRevokeBundle(b OwnerBundle) bool { return b.Version < DDLRevokeOwnerBundleVersion }
func OwnerDDLApplyBundles() ([]OwnerBundle, error) { /* filter(OwnerBundles(), isNonRevokeBundle) */ }
```

**Bindings (every `owner-ddl apply` route, current `main`):** (1) `ApplyOwnerBundles`
(`owner.go:265-303`) loads `OwnerDDLApplyBundles()` at `:269`; (2) `applyPendingOwnerBundles`
(`:309-322`) keeps its `<= current` bound (`:312`) + an in-loop `isNonRevokeBundle` guard; (3)
`ReapplyAllOwnerBundles` (`:332-351`) adds the same in-loop guard + its nil-fallback (`:336-342`)
loads `OwnerDDLApplyBundles()`; (4) the self-heal branch (`:277-302`) is bound by (1) AND (3); (5)
all test callers inherit the filter; (6) no owner-ddl dry-run/list surface exists today. **The
embed/listing split:** `OwnerBundles()` (`:197-224`) stays the full loader (includes 0021) for
`revokeEmbedded`, `ExpectedFingerprint` (`schema_drift.go:88`), `BuildPlan`, and
`RuntimeOwnedTablesAlterable` (`owner_runtime_ownership.go:52`); `OwnerDDLApplyBundles()` is the
disjoint apply slice.

**Build-time guard — split phase-aware (M4).** F16 is split into two phase-aware tests that each
land green in their own rollout phase:

- **`TestOwnerDDLApplyExcludesSyntheticRevokeBundle` (F16a — pre-0021 / inert, rollout step 2).**
  Drives the filter through a **synthetic bundle list / test hook** (a hand-built `[]OwnerBundle`
  containing a synthetic `{Version: 21}` entry, and/or an injectable `ownerBundleFS` test seam):
  asserts (a) `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21` from the
  synthetic list; (b) `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a hand-passed
  synthetic 0021; (c) `ReapplyAllOwnerBundles(nil, …)` resolves its fallback to the filtered loader.
  It does **NOT** assert production `OwnerBundles()` contains 0021 (it does not yet).
- **`TestOwnerDDLApplyExcludesProductionRevokeBundle` (F16b — activation, rollout step 7, after 0021
  is authored).** Asserts (a) production `OwnerBundles()` DOES contain 0021; (b)
  `ExpectedFingerprint()` includes 0021's bytes; (c) `revokeEmbedded` derives from the full loader /
  file presence in `ownerBundleFS`; (d) production `OwnerDDLApplyBundles()` excludes 0021. The
  forced-self-heal pgtest (below) lives here.
- A build-time grep test (in step 2) asserts no production `applyOneOwnerBundle` caller outside the
  deployer terminal-step path receives a bundle `>= 21`.

### 3.3 Lifting `ApplyMigrations` out of serve-boot (shadow-first + the M3 tightening)

**Claim (DECOUPLE).** Serve-boot stops mutating schema, behind a flag, shadow-first. The single
mutation site on the serving path is `ConnectAndMigrate` (`connection.go:332-404`).

1. **New env `STRIATUM_DEPLOY_DECOUPLED`** (default OFF — shadow-first). OFF → boot behaves as today
   for a NO-revoke binary; the decoupling lands to `main` inert.
2. When **ON**, `ConnectAndMigrate` becomes **`ConnectAndVerify`**: keep the watermark interlock
   (`:349`) and the drift gate (`:376-383`) but do NOT call `ApplyMigrations` (`:353`); run the §1.3
   classification and serve only on the in-sync row; the drift self-record (`:399`) is NOT run on the
   verify path.
3. **M3 tightening.** A **revoke-embedding binary** (the binary ships 0021) is **structurally barred
   from the legacy `ConnectAndMigrate` `ApplyMigrations`/`:399` path** by the hoisted config gate in
   `CheckDeployActivation` (§3.3a): with the flag OFF it halts `awaiting_deploy_config` BEFORE `:353`,
   for EVERY cursor state including `complete`. So once a binary embeds the revoke, its ONLY serving
   mode is the decoupled `ConnectAndVerify` path — **and on that path its `complete`-cursor serve
   decision is the fingerprint-conditional A3 (row 16), identical to the no-revoke decoupled row 15
   (M7).**
4. **No window serving an unmigrated schema (R2).** The decoupled boot verifies before serving; the
   watermark interlock is preserved verbatim, and **the fresh `applied_owner == 0` bootstrap still
   serves on the legacy path for a no-revoke flag-OFF binary (M5)** — the decoupling does NOT wedge
   fresh-DB bring-up.

#### 3.3a The deploy-activation interlock (A)

A **`CheckDeployActivation(ctx, runner, revokeEmbedded, decoupledEnabled bool) error`** (in
`go/pkg/db/owner.go` or a new `deploy_activation.go`) is called at the same boot site, **after**
`CheckOwnerBundleWatermark` (W, `connection.go:349`) and **BEFORE `ApplyMigrations`** (`:353`), in
both `ConnectAndMigrate` and `ConnectAndVerify`, and before `RecordSchemaFingerprint` (`:399`). It
mutates nothing. It reads `deploy_cursor` defensively (absent table/row → `cursorState = none`),
`cursor.plan_hash`, and `LiveFingerprint`/`ExpectedFingerprint`. **It does NOT read `applied_owner`
/ `owner_bundle_meta`** — the owner-watermark dimension is W's responsibility (M5), and by
INVARIANT W→A-INDEPENDENCE (§0.2) A's outcome is identical for `applied_owner ∈ {0, ==20}`.
**Predicate (fail-closed), in order:**

0. **(M3 — the hoisted universal decoupling-config gate, fires FIRST for EVERY cursor state.)** If
   **`revokeEmbedded && !decoupledEnabled`** → return **`awaiting_deploy_config`**, DB untouched. *A
   binary that ships the DDL-revoke (0021) must run on the decoupled path; it must NEVER take the
   legacy mutate+self-record path, regardless of cursor state.* **This is the ONLY A predicate that
   reads `revokeEmbedded`; it does NOT fire when `decoupledEnabled == true` (§0.2 sub-invariant).**
1. **UNIVERSAL incomplete-deploy edge (BC-N2).** If `cursorState ∈ {in_progress, step_committed,
   finalizing}` → return **`awaiting_deploy`**, DB untouched.
2. If `cursorState == aborted` → return **`awaiting_deploy`**, DB untouched.
3. If `cursorState == complete`:
   - **`decoupledEnabled == true`** → defer to the drift gate on the VERIFY path: `plan_hash ==
     expected` + `LiveFingerprint == ExpectedFingerprint` → **serve verify-only** (return nil; no
     `ApplyMigrations`, no `:399`); foreign `plan_hash` or fingerprint mismatch → **`awaiting_deploy`**
     (or `schema_drift` under refuse mode). **This branch reads NEITHER `applied_owner` NOR
     `revokeEmbedded` — so it produces the IDENTICAL fingerprint-conditional outcome for the no-revoke
     decoupled binary (row 15) AND the revoke-embedding decoupled binary (row 16), in EVERY W-passing
     column (`==0`, `==20`, and — for the revoke-embedding binary W tolerates forward — `>=21`). This
     is the M7 derivation: row 16's `==0`/`==20`/`>=21` cells are conditional on fingerprint-sync
     exactly like row 15, NOT unconditional.**
   - **`decoupledEnabled == false`** (⇒ `revokeEmbedded == false`, since step 0 caught
     revoke-embedding + flag-OFF): **(M3 — the no-revoke complete comparison), a pre-`ApplyMigrations`,
     DB-untouched, PURE-READ check:** if **`ExpectedFingerprint() == LiveFingerprint(recorded)` AND
     `cursor.plan_hash == expected`** → **return nil** (in-sync; the subsequent legacy `ApplyMigrations`
     is a no-op and the legacy `:399` self-record is an idempotent rewrite of the SAME fingerprint —
     serve). Otherwise → **return `awaiting_deploy`**, DB untouched.
4. If `cursorState == none` (absent table/row, or `idle` — NO transcript present):
   - `decoupledEnabled == true` → **`awaiting_deploy`** (the decoupled boot never auto-applies;
     fresh-DB bring-up runs `deploy`).
   - `decoupledEnabled == false` (⇒ `revokeEmbedded == false`, by step 0):
     - **return nil**: legacy path, serve, `ApplyMigrations` runs. **This is the legacy serve-legacy
       branch reached by both the fresh `applied_owner == 0` bootstrap and the inert-landing
       `applied_owner == 20` re-boot (both pass W); it carries NO deploy transcript, so it is the ONLY
       `cursorState == none` branch that legitimately reaches the mutating legacy `:399` writer
       (M5/M3).**

`revokeEmbedded` = the embedded owner-bundle FS contains the bundle at `DDLRevokeOwnerBundleVersion =
21` (NOT `Latest >= 21`; `LatestOwnerBundleVersion` stays 20). `decoupledEnabled` =
`STRIATUM_DEPLOY_DECOUPLED` is set.

**Net invariant (M3 + BC-N2 + C2 + the M5 W split + the M6/M7 derivation):** no boot serves while
the cursor is non-`complete`; no revoke-embedding binary ever reaches `ApplyMigrations`/`:399`; a
genuine authority shortfall (`1..19`) halts at W before A; and the legacy `:399` writer is reached
ONLY by a no-revoke flag-OFF binary that passed W (`applied_owner ∈ {0, ==20}`) over either NO
transcript (rows 1/`==0` and 1/`==20`) or a `complete` transcript it is ALREADY byte-in-sync with
(rows 13-in-sync/`==0` and 13-in-sync/`==20`) — **row 16 reaches it in NO column** (decoupled,
verify-only). Proven exhaustively in §3.5.

#### 3.3b Runtime-object ownership: Policy 1 with the revoke sequenced last (C3)

`Deployer.applyRuntimeStep` (wrapping `applyOne`, `migrations.go:304-355`): `BEGIN` (owner conn) →
assert `has_schema_privilege('striatumd_rw','striatumd','CREATE')` (else
`deploy_create_prerequisite_missing`) → snapshot owner-owned oids → run migration SQL + both version
stamps + in-tx hash verify → re-snapshot, `ALTER <kind> … OWNER TO striatumd_rw` for new oids →
re-assert DML grants → append the per-step receipt keyed on the stored `(plan_hash, step_index)` →
advance `in_progress(k) → step_committed(k)` → `COMMIT`. The reconcile runs while `striatumd_rw`
still holds CREATE (granted by bundle 0018 at `0018:98`, in the non-revoke prefix); 0021 commits
last, so the steady state denies CREATE.

### 3.4 The deploy receipt + the idempotent finalizer (C1) + the M1 pre-finalizer gate

Every applied step writes exactly one hash-chained receipt via `append_audit_row(...)` keyed on the
stored `(plan_hash, step_index)`. The idempotent finalizer: **(0)** `VerifyStoredTranscript(plan_hash)`
(M1) — runs FIRST, writes nothing, ABORTS on `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`;
**(1)** guarded `complete` receipt; **(2)** `RecordSchemaFingerprint` (`schema_drift.go:171-195`)
UPSERT; **(3)** advance `finalizing → complete` last. The finalizer crash-point table and `doctor
schema_deploy_unrecorded` (transcript-enumerated + the M1 stamp/byte WARN) carry forward.

### 3.4a `VerifyStoredTranscript` — the full-transcript byte + DB-stamp verifier (M1)

A single PURE-READ function checks, for EVERY step in `deploy_plan[plan_hash].steps` (already-applied
AND not), `step.sha256 ==` the running binary's embedded bytes (`MigrationSHASet()` `migrations.go:197`
for runtime; `OwnerBundle.SHA256()` `owner.go:191` for owner) ⇒ `ErrDeployPlanBinaryMismatch`; and for
every already-applied step, `step.sha256 ==` the DB stamp (`schema_migrations.sha256` `migrations.go:275`
/ `owner_bundle_meta.sha256` `owner.go:529`) ⇒ `ErrDeployPlanDBStampMismatch`. Called (i) on every
resume before any apply, (ii) as finalizer step 0, (iii) by the doctor. The two typed halts join the
`AwaitingOwnerDDLError`/`SchemaDriftError` `errors.As` arms (`main.go:208-227`); all map to
refuse-to-serve, DB untouched. **(M7 note: M1's scope is deploy resume / finalizer step 0, NOT
ordinary serve-boot; the M7 fix chooses Option 1 (the conditional cell), so it adds NO new serve-boot
DB-stamp check — the row-16 in-sync cell serves because A reads the recorded fingerprint and finds it
== `ExpectedFingerprint()`. M1 is not extended and not regressed.)**

---

## 3.5 The complete boot-path decision table — DERIVED MECHANICALLY from W and A (M6 + M7)

**Two guard sites, evaluated in order on every boot, both BEFORE `ApplyMigrations` (`:353`) /
`RecordSchemaFingerprint` (`:399`), mutating nothing:**

- **W = `CheckOwnerBundleWatermark` (`connection.go:349`, `owner.go:124-154`) — the M5 dimension,
  FOUR disjoint `applied_owner` buckets covering all of ℕ:**
  - **`applied_owner == 0`** → **return nil, proceed to A** (`owner.go:145`). **The M5 serve bucket.**
  - **`1 <= applied_owner < 20`** → **`awaiting_owner_ddl`**, DB untouched (`owner.go:148-150`).
  - **`applied_owner == 20`** → in-sync, **return nil, proceed to A** (`owner.go:151-153`).
  - **`applied_owner >= 21`** → a **revoke-embedding** binary tolerates forward → A; a **no-revoke**
    binary → **`awaiting_deploy`** (the C2 forward-watermark rule, re-anchored to 21). **Barrier (b).**
- **A = `CheckDeployActivation` (§3.3a).** Step 0 (M3 config gate) is **barrier (a)**. **A does NOT
  read `applied_owner` (§0.2) — so the A-gate outcome is IDENTICAL for the `0` and `==20` columns in
  EVERY cursor row. On the DECOUPLED complete branch A does NOT read `revokeEmbedded` either (§0.2
  sub-invariant) — so rows 15 and 16 take the IDENTICAL A3-conditional outcome in every W-passing
  column.**

The 16 `(cursorState × decoupledEnabled × revokeEmbedded)` rows × **4** `applied_owner` sub-columns =
**64 cells**, all derived. `cursorState` groups `step_committed` with `in_progress` and treats
`aborted` like the non-complete edge (both → `awaiting_deploy` at A step 1/2), per §1.3. **(Build
note B1.2: the executable F18 MUST table-drive each concrete cursor-state enum, not the prose group.)**

| # | cursorState | decoupled | revoke | applied_owner **`==0` (fresh/no-authority)** | applied_owner **`1..19` (shortfall)** | applied_owner `==20` | applied_owner `>=21` |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | none | off | no | **SERVE-legacy — FRESH-DB BRING-UP** (W nil for `applied==0`; A4 none/legacy; `:399` reachable — NO transcript) **← M5 serve cell** | `awaiting_owner_ddl` (W shortfall) | **SERVE-legacy — inert-landing re-boot** (A4; `:399` reachable — NO transcript) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 2 | none | off | yes | **`awaiting_deploy_config`** (W nil, then A0 barrier a) | `awaiting_owner_ddl` | **`awaiting_deploy_config`** (A0) **= `==0`** | **`awaiting_deploy_config`** (A0; W tolerates revoke, then barrier a) |
| 3 | none | on | no | `awaiting_deploy` (A4: decoupled never auto-applies) | `awaiting_owner_ddl` | `awaiting_deploy` (A4) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 4 | none | on | yes | `awaiting_deploy` (A4: run deploy to apply bundles + 0021) | `awaiting_owner_ddl` | `awaiting_deploy` (A4) **= `==0`** | `awaiting_deploy` (A4; cursor=none+0021-applied inconsistent → conservative halt) |
| 5 | in_progress/step_committed/aborted | off | no | `awaiting_deploy` (A1/A2, BC-N2) | `awaiting_owner_ddl` | `awaiting_deploy` (A1/A2, BC-N2) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 6 | in_progress/step_committed/aborted | off | yes | **`awaiting_deploy_config`** (W nil, then A0) | `awaiting_owner_ddl` | **`awaiting_deploy_config`** (A0) **= `==0`** | **`awaiting_deploy_config`** (A0) |
| 7 | in_progress/step_committed/aborted | on | no | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_owner_ddl` | `awaiting_deploy` (A1/A2, BC-N2) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 8 | in_progress/step_committed/aborted | on | yes | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_owner_ddl` | `awaiting_deploy` (A1/A2, BC-N2) **= `==0`** | `awaiting_deploy` (A1/A2; W tolerates revoke) |
| 9 | finalizing | off | no | `awaiting_deploy` (A1, BC-N2) | `awaiting_owner_ddl` | `awaiting_deploy` (A1, BC-N2) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 10 | finalizing | off | yes | **`awaiting_deploy_config`** (W nil, then A0) | `awaiting_owner_ddl` | **`awaiting_deploy_config`** (A0) **= `==0`** | **`awaiting_deploy_config`** (A0) |
| 11 | finalizing | on | no | `awaiting_deploy` (A1, BC-N2) | `awaiting_owner_ddl` | `awaiting_deploy` (A1, BC-N2) **= `==0`** | `awaiting_deploy` (W barrier b) |
| 12 | finalizing | on | yes | `awaiting_deploy` (A1, BC-N2 resume finalize) | `awaiting_owner_ddl` | `awaiting_deploy` (A1, BC-N2) **= `==0`** | `awaiting_deploy` (A1; W tolerates revoke) |
| **13** | **complete** | **off** | **no** | **SERVE-legacy if in-sync, else `awaiting_deploy`** (A3 no-revoke comparison — `applied_owner`-independent; in-sync → idempotent `:399` rewrite, out-of-sync → halt DB-untouched) **← M6** | `awaiting_owner_ddl` | **SERVE-legacy if in-sync, else `awaiting_deploy`** (A3; `:399` only on idempotent no-op rewrite) **= `==0`** | `awaiting_deploy` (W barrier b) |
| **14** | **complete** | **off** | **yes** | **`awaiting_deploy_config`** (W nil, then A0 — **the M3 cell**) | `awaiting_owner_ddl` | **`awaiting_deploy_config`** (A0 — **the M3 cell**) **= `==0`** | **`awaiting_deploy_config`** (A0 — post-deploy steady state with the flag erroneously OFF; the v5 reproducer, halted) |
| **15** | **complete** | **on** | **no** | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled — `applied_owner`-independent; in-sync → `ConnectAndVerify` serve, NO `:399`) **← M6** | `awaiting_owner_ddl` | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled) **= `==0`** | `awaiting_deploy` (W barrier b) |
| **16** | **complete** | **on** | **yes** | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled — `applied_owner`- AND `revokeEmbedded`-independent; **normal pre-0021 state OUT-OF-SYNC → `awaiting_deploy`**; degenerate in-sync corner → SERVE-verify, NO `:399`) **← M7 FIX: conditional, mirroring row 15** | `awaiting_owner_ddl` | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled; normal pre-0021 OUT-OF-SYNC) **= `==0`** | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled — **normal post-0021 steady state IN-SYNC → SERVE-verify** (activated serving cell, zero serve-path DDL); a further pending change → `awaiting_deploy`) **← M7 FIX: conditional** |

**What changed vs v8 (M7):** the three W-passing cells in row 16 (`==0` and `==20` were
unconditional `awaiting_deploy`; `>=21` was unconditional `SERVE-verify`) are now all
**conditional**, "SERVE-verify if in-sync, else `awaiting_deploy`", **IDENTICAL to row 15's
decoupled cells**, because A's decoupled complete branch reads neither `applied_owner` NOR
`revokeEmbedded` (§0.2 sub-invariant / §3.3a step 3). Every other cell is verbatim from v8.

**Cross-row `==0`-vs-`==20` audit + the row-15-vs-row-16 audit (M6 + M7 — class closed by
derivation):**

- **`none` rows (1–4).** All `==0`/`==20` match (row 1 SERVE-legacy via A4; rows 2/4 A0/A4; row 3 A4).
- **`in_progress`/`step_committed`/`aborted` rows (5–8).** Every `==0`/`==20` cell halts at A1/A2
  (BC-N2) or A0 (row 6). All match.
- **`finalizing` rows (9–12).** Every `==0`/`==20` cell halts at A1 (BC-N2 resumable-finalize) or A0
  (row 10). All match.
- **`complete` rows (13–16).** Row 13 (off, no-revoke): both "SERVE-legacy if in-sync, else
  `awaiting_deploy`" (M6). Row 14 (off, revoke): both `awaiting_deploy_config` (A0). Row 15 (on,
  no-revoke): both "SERVE-verify if in-sync, else `awaiting_deploy`" (M6). Row 16 (on, revoke):
  **`==0`/`==20`/`>=21` all "SERVE-verify if in-sync, else `awaiting_deploy`" (M7), IDENTICAL to
  row 15's conditional** (the `>=21` column reaches A because W tolerates the revoke-embedding binary
  forward; rows 13/15 `>=21` halt at W barrier b because they are no-revoke). **All `==0`/`==20`
  match, AND rows 15 and 16 take the identical A3-conditional form on the decoupled branch.**

An M8 of the form "row X column Y differs from its sibling" or "a complete-row cell is asserted where
A is conditional" is foreclosed by construction — every A-reaching complete-row cell {13/`==0`,
13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`} is written conditionally; row 14
halts at A0 (M3 config gate, unconditional by design); the `1..19` column and the rows-13/15 `>=21`
cells halt at W (uniform).

---

## 4. Serving-role DDL revocation (owner bundle 0021), without lockout / pre-revoke serve window / early self-heal revoke / legacy-path bypass / wedging fresh-DB bring-up, over a fully-derived complete-cursor table

### 4.1 What is revoked, precisely

Bundle 0021 revokes object creation on the runtime schema: `REVOKE CREATE ON SCHEMA striatumd FROM
striatumd_rw;` (+ `REVOKE TEMPORARY` if granted). `striatumd_rw` retains ownership of (and
`ALTER`/`DROP` on) the runtime tables it already owns; new runtime objects are reconciled back to
`striatumd_rw` before 0021 commits (§3.3b). Full capability revocation (owner re-owns runtime tables)
remains a named follow-up beyond P4.

### 4.2 Typed halts + the forward-watermark rule (C2) + the universal cursor edge (BC-N2) + the M1 mismatch halts + the M3 config gate + the M5 fresh-DB serve

Sibling halts of `AwaitingOwnerDDLError` (`owner.go:42,58`), reusing the non-restartable exit
(`main.go:208-227`):

- **`AwaitingOwnerDDLError`** (`awaiting_owner_ddl`): the genuine authority shortfall, `1 <=
  applied_owner < 20` (`owner.go:148-150`). DB untouched. **NOT fired for `applied_owner == 0` (M5).**
- **`AwaitingDeployError`** (`awaiting_deploy`): a deploy is pending/incomplete (BC-N2), OR a
  decoupled binary (row 15 OR row 16) over a `complete` transcript NOT in-sync (A3 decoupled —
  `applied_owner`- and `revokeEmbedded`-independent, so BOTH the `==0` and `==20` columns of rows
  13/15/16-out-of-sync, AND the row-16/`>=21` out-of-sync sub-case), OR a no-revoke binary on a
  revoke-applied DB (`applied_owner >= 21`, barrier b). DB untouched. **(M7: the normal pre-0021
  row-16 `==0`/`==20` state is out-of-sync and halts here — the dominant real-world case.)**
- **`AwaitingDeployConfigError`** (`awaiting_deploy_config`): the binary ships 0021 but
  `STRIATUM_DEPLOY_DECOUPLED` is OFF — for EVERY cursor state including `complete` (M3 gate, §3.3a
  step 0), in EVERY `applied_owner` column that passes W. DB untouched. Remediation: "this binary
  ships the DDL-revoke; set `STRIATUM_DEPLOY_DECOUPLED=1` to serve verify-only, or run `striatum
  daemon deploy` to apply a pending change."
- **`DeployPlanBinaryMismatchError`** / **`DeployPlanDBStampMismatchError`** (M1). DB untouched.

**Forward-watermark rule (`CheckOwnerBundleWatermark` tolerate-forward branch `owner.go:151-153`),
re-anchored to 21.** `applied >= 21` + this binary does NOT embed 0021 → `AwaitingDeployError`
(barrier b). For a revoke-embedding binary W tolerates `>=21` forward → A, and the decoupled
row-16/`>=21` cell takes A3's conditional outcome (normally in-sync → serve-verify).
`RequiredOwnerBundleVersion` stays **20** (`owner.go:35`); the revoke is gated by the cursor edge +
`CheckDeployActivation` + the flag + its deploy-plan-terminal placement + the `OwnerDDLApplyBundles`
exclusion — NOT the watermark frontier. **The M7 fix does NOT advance `Required` or change the
`applied >= 21` forward rule.**

### 4.3 The deploy choreography (C2; M3-hardened; M5-clarified; M6/M7-coherent)

Two binaries, in order:

1. **Inert-landing binary** (`Latest = Required = 20`; does NOT embed 0021). Ships the `Deployer`
   engine + `deploy` verb, migration 0044, the decoupled boot path (flag OFF), the doctor block, the
   forward-watermark rule, the universal pre-revoke cursor edge (BC-N2), `VerifyStoredTranscript`
   (M1), `OwnerDDLApplyBundles` (M2), and `CheckDeployActivation` with the hoisted M3 config gate +
   the no-revoke complete comparison. **Boot auto-applies legacy only because no deploy is in flight
   and the binary embeds no revoke — and W serves `applied_owner == 0` (fresh) AND `== 20`
   (already-bundled), so this binary brings up a fresh DB (cell 1/`==0`) and re-boots an existing DB
   (cell 1/`==20`) without wedging either (M5); and on a `complete` cursor it serves iff byte-in-sync,
   identically for the `==0` and `==20` columns (rows 13/15, M6).** **Daemon up, serving.**
2. **Activation binary** (embeds 0021; `Latest = Required` still 20). On boot with the flag OFF it
   halts **`awaiting_deploy_config` for EVERY cursor state** (cells 2/6/10/14 — the M3 gate). The
   operator, with the daemon parked: sets `STRIATUM_DEPLOY_DECOUPLED=1`; runs `striatum daemon
   deploy`; restarts the daemon → `ConnectAndVerify` sees `complete` + in-sync → serves verify-only,
   zero serve-path create-DDL (**cell 16/`>=21`-in-sync — the M7-derived conditional serve, the normal
   post-0021 steady state**). **From activation onward the operator keeps `STRIATUM_DEPLOY_DECOUPLED=1`
   set.**

   **Crash mid-deploy is safe even if the inert binary comes back up (BC-N2)** (cells 5/7/9 halt
   `awaiting_deploy`). **`owner-ddl apply` before deploy is safe (M2).** **A later revoke-embedding
   binary that restarts with the flag OFF — at ANY cursor state — halts `awaiting_deploy_config` (M3,
   cells 2/6/10/14).** **A fresh `applied_owner == 0` DB first boot of the inert binary still serves
   (cell 1/`==0`, M5).** **And the `complete`-cursor decision is owner-watermark-independent AND (on
   the decoupled path) `revokeEmbedded`-independent — a `complete`/in-sync cursor serves whether
   `owner_bundle_meta` reads 0/20/`>=21` and whether the binary embeds the revoke or not (rows 15 and
   16 identical), and a `complete`/pending-change cursor halts in every column (M6 + M7) — so the
   executable F18 oracle is true for every complete-row cell, including the constructible in-sync
   row-16 cell.**

### 4.4 How 0021 ships without lockout, pre-revoke serve window, early self-heal revoke, legacy-path bypass, or wedging fresh-DB bring-up — over a FULLY-derived complete-cursor table (M6 + M7)

The deployer applies all DDL over the owner/admin connection. **C3:** 0021 is the terminal plan step
and excluded from the owner prefix. **M2:** every `owner-ddl apply` route iterates
`OwnerDDLApplyBundles()` (0021 excluded) + both loops guard `isNonRevokeBundle` in-loop. **BC-N2:**
every deployer-aware binary refuses on a non-`complete` cursor before `ApplyMigrations`/`:399`.
**M3:** every revoke-embedding binary with the flag OFF refuses (`awaiting_deploy_config`) for EVERY
cursor state, and the forward-watermark rule refuses a no-revoke binary on a revoke-applied DB. **M5:**
W serves the fresh `applied_owner == 0` bootstrap (and the `== 20` re-boot) on the legacy path,
halting ONLY the genuine `1..19` shortfall. **M6:** the no-revoke `complete`-cursor cells (13/15) are
derived from the `applied_owner`-independent A predicate, so `==0` and `==20` are identical. **M7:**
the revoke-embedding decoupled `complete`-cursor cell (16) is derived from the `applied_owner`- AND
`revokeEmbedded`-independent A3 predicate, conditional on fingerprint-sync, identical to row 15 —
closing the complete-row class fully; §4.5 + the F18 spy list still enumerate the SAME four
`:399`-reaching cells (row 16 never reaches `:399`).

### 4.5 Proactive hardening — every apply path and every fingerprint path named, two universal invariants

**A. EVERY code path that applies owner bundles (file:line).** `ApplyOwnerBundles`
(`owner.go:265-303`, loads `OwnerDDLApplyBundles()` `:269`), `applyPendingOwnerBundles` (`:309-322`,
in-loop guard), `ReapplyAllOwnerBundles` (`:332-351`, in-loop guard + filtered nil-fallback),
`applyOneOwnerBundle` (`:511-541`, the only committer of 0021 via the terminal deploy step),
`runDaemonOwnerDDL` (`daemon.go:90` → `ApplyOwnerBundles`), the deployer terminal step, the test
setups, and the non-apply `OwnerBundles()` consumers
(`ExpectedFingerprint`/`revokeEmbedded`/`BuildPlan`/`RuntimeOwnedTablesAlterable`).

> **UNIVERSAL INVARIANT A (executable).** *The DDL-revoke bundle (`>= DDLRevokeOwnerBundleVersion =
> 21`) is excluded from ALL `owner-ddl apply` routes via `OwnerDDLApplyBundles()` + the in-loop
> `isNonRevokeBundle` guard; only the terminal `deploy` step commits it, via `applyOneOwnerBundle`.*
> Enforced by **F16a** (synthetic) + **F16b** (production).

**B. EVERY code path that self-records / writes a schema fingerprint (file:line):**

| Path | Site | Disposition under M1 + M3 + M5 + M6 + M7 |
| --- | --- | --- |
| `RecordSchemaFingerprint` (the ONLY `schema_state.fingerprint` writer) | `schema_drift.go:171-195` | unchanged; gated by callers; orthogonal to `owner_bundle_meta`/`applied_owner` (writes only the `schema_state` singleton) |
| Legacy boot self-record | `connection.go:399` ("This is the only writer of schema_state") | **reachable ONLY in the FOUR cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} — a no-revoke flag-OFF binary that passed W (`applied_owner ∈ {0, ==20}`) over NO transcript OR a `complete` transcript ALREADY byte-in-sync. UNCHANGED by M7 — row 16 is decoupled and reaches `:399` in NO column.** |
| C1 finalizer `schema_state` UPSERT + `finalizing → complete` | P4 `Deployer.Apply` (§3.4) | gated by `VerifyStoredTranscript` finalizer step 0 (M1) — writes nothing on mismatch |
| `ExpectedFingerprint()` / `LiveFingerprint` / `CheckSchemaDrift` (pure reads) | `schema_drift.go:83-100,145-161,254-274` | read-only; `LiveFingerprint` reads `schema_state`, never `owner_bundle_meta` |

> **UNIVERSAL INVARIANT B (executable, TIGHTENED for M3; M5 split + M6/M7 derivation re-proven).**
> *NO `schema_state` fingerprint / `complete` is EVER written around `VerifyStoredTranscript` while a
> deploy transcript carrying a pending change is present, and NO revoke-embedding binary EVER reaches
> the legacy `:399` writer.* Two writers, both bounded: (i) the deployer finalizer is gated by
> `VerifyStoredTranscript` step 0 (M1); (ii) the legacy `connection.go:399` writer is reachable ONLY
> in the **four** §3.5 cells **1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`**. **M6: the
> `==0` and `==20` columns reach the legacy writer in the SAME two rows (1 and 13-in-sync). M7: row 16
> reaches the legacy writer in NO column — its serve-verify cells serve via `ConnectAndVerify` without
> self-recording — so the M7 fix adds NO cell to the spy list; this enumeration is identical to the
> F18 spy list (§5).** Enforced by **F17** (the M3 cell), **F11** (extended), **F15** (extended), and
> **F18** (the §3.5 64-cell matrix, with the 4-cell `:399` spy list = this enumeration, rows 13/15
> `==0` conditional == `==20`, and **row 16 `==0`/`==20`/`>=21` conditional == row 15, serving via the
> verify path with the `:399` spy NOT fired**).

**The legitimate fresh-DB / inert-landing cells STILL SERVE (R2, not wedged — the M5 requirement,
preserved).** Cell 1/`==0` serves the fresh-DB bring-up; Cell 1/`==20` serves the inert-landing
re-boot. The conservative halts fire ONLY where they must: the `1..19` shortfall (W), a
revoke-embedding binary with the flag OFF (cells 2/6/10/14, A0), an incomplete/aborted cursor (BC-N2),
a no-revoke binary on a revoke-applied DB (barrier b), or a `complete` cursor over a DB NOT
byte-in-sync (A3 — rows 13/15/16, both columns).

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> `T-` = unit/integration; `G-` = game-day against a real two-role cluster. F1–F17 carry forward;
> **F18 is the MECHANICALLY-DERIVED 64-cell table, PARAMETRIC over ALL seven A-reaching complete-row
> cells (13/15/16) with the in-sync/out-of-sync sub-dimension, the row-16 cells conditional == row
> 15, the 4-cell `:399` spy list UNCHANGED.** F18a (the fresh-DB serve) carries forward.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1–F10, F12–F14** | Resumability Q3-A/Q3-B; no serve-boot mutation when decoupled; §1.3 classification incl. the M1 mismatch rows; no P2/P3 regression; DDL-revoke structural; per-step receipt; no double-apply; plan≡fingerprint; the C1 finalization boundary with the M1 step-0 gate; the C3 runtime-object ownership extended for M2; per-step crash-safety; immutable plan identity with M1 already-applied verification. | (as v8) |
| **F11 (Activation fail-closed — C2 + BC-N2 universal edge; M3 (g))** | For every deployer-aware binary, boot never calls `ApplyMigrations`/`RecordSchemaFingerprint` while the cursor is non-`complete`; a no-revoke binary at `applied_owner == 20` with a non-`complete` cursor halts `awaiting_deploy` DB-byte-identical; a no-cursor/`idle` no-revoke binary SERVES; (g) a revoke-embedding binary with the flag OFF halts `awaiting_deploy_config` for EVERY cursor state. | `T-deploy-revoke-activation-ordering` cases (a)–(g); spies prove `applyOne`/`RecordSchemaFingerprint` never entered. `G-old-binary-refuse`. |
| **F15 (Full-transcript verification on resume AND before the finalizer — M1)** | The byte case, the owner-step case, the DB-stamp case, the finalizer arm; the finalizer's `VerifyStoredTranscript` step-0 gate AND the §3.3a M3 config gate together mean NO writer self-records around the full-transcript check. **(M7: M1 NOT extended to serve-boot.)** | `T-deploy-resume-already-applied-byte-mismatch-refuses` + an assertion that the §3.3a guard precedes `connection.go:353` in BOTH boot paths. `G-wrong-binary-resume`. |
| **F16a / F16b (Revoke excluded from EVERY owner-ddl apply route — M2; synthetic + production phases)** | F16a (synthetic list / injectable FS, step 2) excludes every bundle `>= 21` from both apply loops + the nil-fallback WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (step 7) asserts the production embed/listing split + the forced FMA-007 self-heal reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`. | `TestOwnerDDLApplyExcludesSyntheticRevokeBundle` + grep test (step 2); `TestOwnerDDLApplyExcludesProductionRevokeBundle` + `T-deploy-revoke-excluded-from-reapply-self-heal` (step 7). |
| **F17 (M3 — the complete-cursor legacy mutate+self-record refusal)** | A revoke-embedding binary with a `complete` cursor, flag OFF, and a pending change refuses `awaiting_deploy_config`, calls NEITHER `ApplyMigrations` NOR `RecordSchemaFingerprint`, leaves `schema_state` unchanged and the DB byte-identical, on BOTH boot paths; symmetric shadow-mode arm asserts the fall-through `connection.go:384-399` is never reached. | `T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`; `G-complete-cursor-flag-off-refuse`. |
| **F18 (Boot-path decision table — PARAMETRIC over ALL seven A-reaching complete-row cells; row 16 conditional == row 15; the 4-cell `:399` spy list UNCHANGED)** | EVERY `cursorState ∈ {none, in_progress, step_committed, finalizing, complete, aborted} × decoupledEnabled ∈ {on, off} × revokeEmbedded ∈ {yes, no} × applied_owner ∈ {0, 1..19, ==20, >=21}` cell produces the EXACT §3.5 outcome; in NO cell does the legacy `:399` writer fire over a transcript carrying a pending change or for a revoke-embedding binary; **AND (1) for every fixed `(cursorState, decoupledEnabled, revokeEmbedded)` row, the `==0` and `==20` columns produce the IDENTICAL outcome (A owner-watermark-independent); (2) rows 13 and 15 in the `==0` column are CONDITIONAL "serve if in-sync, else `awaiting_deploy`" matching `==20`; (3) the `:399` spy fires in EXACTLY the four cells {1/`==0`, 1/`==20`, 13-in-sync/`==0`, 13-in-sync/`==20`} = the §4.5 enumeration; (4) the decoupled complete branch is `revokeEmbedded`-independent — for every fixed `(complete, decoupledEnabled=on)` the no-revoke row 15 and the revoke-embedding row 16 produce the IDENTICAL outcome in every W-passing column; (5) row 16 in the `==0`, `==20`, AND `>=21` columns is CONDITIONAL "serve verify-only if in-sync, else `awaiting_deploy`" (the in-sync sub-case constructed by setting `schema_state.fingerprint == ExpectedFingerprint()` AND `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent / version-20 / version-`>=21` DB; the out-of-sync sub-case → `awaiting_deploy`); (6) F18 is PARAMETRIC over the SEVEN A-reaching complete-row cells {13/`==0`, 13/`==20`, 15/`==0`, 15/`==20`, 16/`==0`, 16/`==20`, 16/`>=21`}, constructing BOTH sub-cases for each; (7) the decoupled SERVE-verify cells (15-in-sync and 16-in-sync, all W-passing columns) serve but the `:399` spy is NOT fired; (8) the normal pre-0021 row-16 `==0`/`==20` state is OUT-OF-SYNC (→ `awaiting_deploy`) and the normal post-0021 row-16 `>=21` state is IN-SYNC (→ serve-verify).** | **`T-deploy-bootpath-decision-table`**: a parametrized matrix over all 64 cells with the in-sync/out-of-sync sub-dimension for the SEVEN A-reaching complete-row cells; for each, construct the DB + binary state and assert the §3.5 outcome; assert the column-identity property (`==0` == `==20`) for every row AND the row-identity property (row 15 == row 16 on the decoupled complete branch) for every W-passing column; assert `ApplyMigrations`/`RecordSchemaFingerprint` spies fire ONLY in the four cells above (and NOT in any row-16 serve-verify cell). The `applied_owner == 0` arm sets `owner_bundle_meta` ABSENT; the row-16 in-sync arm independently sets `schema_state.fingerprint == ExpectedFingerprint()` over an `owner_bundle_meta`-absent/20/`>=21` DB (proving the orthogonality). Asserts W precedes A and both precede `connection.go:353` on both boot paths. **MUST table-drive each concrete cursor-state enum (B1.2).** |
| **F18a (Fresh-DB bootstrap serves — the M5 serve cell)** | A no-revoke binary (`Latest = Required = 20`, 0021 NOT embedded), flag OFF, booting against a FRESH database with `striatumd.owner_bundle_meta` ABSENT (`OwnerBundleVersion → 0`) and `cursorState = none` SERVES: `CheckOwnerBundleWatermark` returns nil (the `applied == 0` exception, `owner.go:145`), `CheckDeployActivation` step 4 returns nil, legacy `ConnectAndMigrate` runs `ApplyMigrations` + the `:399` self-record. Conversely, the SAME binary against a DB with `owner_bundle_meta` at version `k ∈ [1,19]` halts `awaiting_owner_ddl` DB-untouched. | **`T-deploy-fresh-db-bootstrap-serves`**: (1) fresh DB + inert binary, flag OFF → assert serve, both spies CALLED, daemon up; (2) DB with `owner_bundle_meta.version ∈ {1, 19}` → assert `awaiting_owner_ddl`, both spies un-called, DB byte-identical. `G-fresh-db-first-boot`. |

---

## 6. Shadow-first rollout (the P4 build order — contract-first, smallest safe first)

1. **Migration 0044 `deploy_cursor` + `deploy_plan`** (additive runtime tables; `state` CHECK
   includes `finalizing`; runtime-owned).
2. **`go/pkg/db/owner.go` M2 surface (lands first, inert):** `DDLRevokeOwnerBundleVersion = 21`,
   `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, the in-loop guards, the nil-fallback split, **and
   the F16a SYNTHETIC-phase test + the build-time grep test (M4)**. Inert until 0021 is authored
   (step 7).
3. **`go/pkg/db/deploy.go`**: `DeployPlan`, `BuildPlan` (0021-terminal, FULL `OwnerBundles()`),
   `LoadStoredPlan`, `VerifyStoredTranscript` (M1) + the typed mismatch halts, `Deployer.Apply`
   (Q3-A/Q3-B engine + the `finalizing` finalizer with `VerifyStoredTranscript` step 0), the
   substrate-ensure preamble, `applyRuntimeStep` (C3 reconcile), the receipt writer. Pure-core +
   DB-integration tests (F1, F2, F4, F8, F9, F10, F12, F13, F14, F15) proven BEFORE any boot path
   changes.
4. **`runDaemonDeploy`** verb + matrix/authority-guardrail row + `--dry-run`/`--abort` + the
   0021-activation preflight. F3/F5 wiring.
5. **`CheckDeployActivation`** with the M3 predicate — **the hoisted `revokeEmbedded &&
   !decoupledEnabled → awaiting_deploy_config` config gate (every cursor state, step 0) + the
   no-revoke `complete` pre-`ApplyMigrations` comparison (step 3), neither reading `applied_owner`
   (M6), and the decoupled complete branch reading neither `applied_owner` NOR `revokeEmbedded` (M7,
   §0.2 sub-invariant)** — on top of the universal pre-revoke cursor edge (BC-N2) + the typed halts +
   **the M5-correct `CheckOwnerBundleWatermark` owner-watermark gate (the `applied_owner == 0` serve
   exception preserved, the `1..19` shortfall halt, the forward-watermark rule at `>= 21`)**.
   Decoupled boot path (`ConnectAndVerify`) behind `STRIATUM_DEPLOY_DECOUPLED` (default OFF). **Lands
   inert for a no-revoke binary; the fresh `applied_owner == 0` bootstrap still serves; the no-revoke
   `complete`-cursor cells (13/15) derived from the `applied_owner`-independent A predicate; the
   revoke-embedding decoupled `complete` cell (16) derived from the `applied_owner`- AND
   `revokeEmbedded`-independent A3 predicate, conditional and identical to row 15** (F11 incl. (g),
   F3, F5, F17, **F18 with the parametric complete-row extension + the column-identity + the
   row-15/row-16 identity + the unchanged 4-cell spy list, F18a**).
6. **`doctor schema_deploy_unrecorded`** block — per-step tightened, transcript-enumerated, + the M1
   stamp/byte WARN (F7, F4, F15 doctor arm).
7. **Owner bundle 0021** (DDL revoke) — authored, deploy-plan-terminal, excluded from every
   `owner-ddl apply` route (`LatestOwnerBundleVersion` stays 20). **The F16b PRODUCTION-phase test +
   the forced-self-heal pgtest land here (M4)**, tested in two-role pgtest (F6, F12, F16b); activation
   is the operator choreography (§4.3).

Each phase additive and reversible. Self-record before enforce; detection before mutation-relocation;
the verb + finalizer + full-transcript verifier + ownership reconcile + immutable-transcript resume +
the non-revoke filter + the M3 activation guard + **the M5-correct owner-watermark gate (fresh-DB
serve preserved) + the M6/M7-derived boot-path table (every complete-row cell derived from W and A)**
all proven before the boot path leans on them.

---

## 6.5 Acceptance Criteria (what an impl-run + verify-run MUST meet) — incl. the mandatory game-day fire tests and finding B1

The build run clears only when every assertion in §5 has its named test green AND the following
game-day fire tests pass against a real two-role cluster. Each maps to an F-/G- assertion above.

- **(a) Crash-resume with a stable key (BC-N1).** Kill `striatum daemon deploy` after a committed
  step; prove clean resume with a stable `plan_hash`/`step_index` and exactly-once per-step receipt
  off the STORED transcript. → F13/F14, `G-...`.
- **(b) Divergent-binary resume refuses (M1).** Resume with a binary whose ALREADY-APPLIED step bytes
  differ; prove `deploy_plan_binary_mismatch`, NO apply, NO fingerprint write, NO `complete`. → F15,
  `G-wrong-binary-resume`.
- **(c) Universal pre-revoke serve edge (BC-N2).** Boot a no-revoke-bundle binary against an
  incomplete pre-revoke cursor at `applied_owner == 20`; prove it does NOT mutate schema and does NOT
  self-record. → F11(e)/(f), `G-old-binary-refuse`.
- **(d) Self-heal does not commit the revoke early (M2).** Force the FMA-007 cross-bundle self-heal
  with 0021 loaded; prove 0021 is NOT applied and CREATE stays held. → F16b,
  `T-deploy-revoke-excluded-from-reapply-self-heal`.
- **(e) Complete-cursor flag-OFF revoke-embedding refusal (M3).** Boot a revoke-embedding binary on a
  `complete` cursor with the flag OFF and a pending change; prove `awaiting_deploy_config`, no
  `ApplyMigrations`, no `RecordSchemaFingerprint`, `schema_state` unchanged, DB byte-identical. → F17,
  `G-complete-cursor-flag-off-refuse`.
- **(f) Fresh-DB serve + shortfall halt (M5/row-1).** Boot a no-revoke binary (flag OFF,
  `cursorState=none`) over a fresh `applied_owner == 0` DB and prove it SERVES the normal fresh
  bring-up (NOT wedged); separately prove a `1 <= applied_owner < 20` DB halts `awaiting_owner_ddl`
  DB-untouched — the F18 dual-cell assertion. → F18a, `G-fresh-db-first-boot`.
- **(g) No-revoke complete in-sync/out-of-sync (M6 — rows 15/`==0`).** Boot a no-revoke binary
  (decoupled ON) over a `complete`/`applied_owner==0`/in-sync DB and prove it SERVES verify-only (NOT
  `awaiting_deploy`); separately prove the same binary over a `complete`/`applied_owner==0`/out-of-sync
  DB halts `awaiting_deploy`. → F18.
- **(h) Degenerate legacy in-sync no-op (M6 — F18 spy list, row 13/`==0`-in-sync).** Boot a no-revoke
  binary (flag OFF) over a `complete`/`applied_owner==0`/in-sync DB and prove it SERVES legacy no-op
  (the idempotent `:399` rewrite) and NOT `awaiting_deploy`. → F18.
- **(i) Revoke-embedding complete in-sync/out-of-sync at `==0` (M7 — row 16/`==0`).** Boot a
  revoke-embedding binary (decoupled ON) over a `complete`/`applied_owner==0`/in-sync DB and prove it
  SERVES verify-only (NOT `awaiting_deploy`); separately prove the same binary over a
  `complete`/`applied_owner==0`/out-of-sync DB halts `awaiting_deploy`. → F18 (parametric).
- **(j) Revoke-embedding complete in-sync/out-of-sync at `==20` (M7 — row 16/`==20`).** Boot a
  revoke-embedding binary (decoupled ON) over a `complete`/`applied_owner==20`/in-sync DB and prove it
  SERVES verify-only; separately over a `complete`/`applied_owner==20`/out-of-sync DB halts
  `awaiting_deploy`. → F18 (parametric). **(And the `>=21` activated steady state serves verify-only
  in-sync — row 16/`>=21`.)**
- **(k) Hash-chained receipt + doctor (BC-N1).** Prove the deploy receipt is hash-chained and `doctor`
  surfaces `schema_deploy_unrecorded`. → F7/F4/F14.
- **(l) Two-role activation + post-deploy CREATE denial (C3).** In a two-role cluster, run the
  activation deploy through to `complete` and prove runtime-object ownership + post-deploy
  `striatumd_rw` CREATE denial. → F12, `G-revoke-last`.

**Finding B1 (binding build-phase obligations, deferred from the cycle-1 ledger — the build run MUST
honor both):**
1. **B1.1 — F18 must actually exercise the row-16 derivation.** `T-deploy-bootpath-decision-table`
   MUST construct the row-16 in-sync AND out-of-sync sub-cases for `applied_owner == 0`, `==20`, and
   `>=21` (the in-sync arm independently setting `schema_state.fingerprint == ExpectedFingerprint()`
   AND `cursor.plan_hash == expected` over an `owner_bundle_meta`-absent / 20 / `>=21` DB, proving the
   orthogonality) and assert the in-sync row-16 cells serve verify-only WITHOUT firing the
   `ApplyMigrations`/`RecordSchemaFingerprint` spies. Omitting these recreates M7 in code.
2. **B1.2 — expand the grouped cursor-state shorthand.** The "64-cell" shorthand groups
   `step_committed` with `in_progress` and `aborted` with the non-complete edge; the executable test
   MUST table-drive each concrete cursor-state enum named by F18
   (`none`/`in_progress`/`step_committed`/`finalizing`/`complete`/`aborted`).

---

## 7. Boundary & explicit deferrals

- **In P4:** the deployer verb; the deploy plan + `deploy_cursor` (incl. `finalizing`) + the immutable
  `deploy_plan` transcript (BC-N1) + per-step + terminal receipt; the idempotent finalizer (C1) with
  the `VerifyStoredTranscript` pre-finalizer gate (M1); the C3 ownership reconcile with 0021 sequenced
  last; the single non-revoke `owner-ddl apply` filter + embed/listing split (M2); lifting
  `ApplyMigrations` out of serve-boot (flagged); the activation interlock + the universal pre-revoke
  cursor edge (BC-N2) + the M3 complete-cursor config gate + no-revoke comparison + forward-watermark
  rule + **the M5-correct owner-watermark dimension (fresh `applied_owner == 0` serves; `1..19` halts)
  + the M6-coherent no-revoke `complete` rows (13/15) + the M7-derived revoke-embedding decoupled
  `complete` cell (16) — the entire complete-row class conditional on A3 fingerprint-sync, derived
  from W and A**; serving-role create-DDL revocation (bundle 0021, deploy-terminal).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`, `rehearsal_receipt.v1`, fidelity
  tiering, full-data clone, expand/contract reshape, lock-budget guardrail. Full capability revocation
  (owner re-owns runtime tables) is a named follow-up beyond P4 (§4.1). The verb-surface decisions
  (content-addressed `plan_hash` + `deploy_plan`; hash-chained receipt; thin CLI over
  `Deployer.Apply`) keep the P5 run-shape door open (Q4, §2).
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the single writer, no hosted
  services / replicas / external persistence.

---

## 8. Anchor table (verified against the run worktree HEAD; `git diff --stat 3f9d5734 HEAD -- go/pkg/db/` EMPTY)

The M7 fix is a SPEC change to the §3.5 row-16 cell + the F18 parametric extension, not a source
change; the go/pkg/db source is byte-identical with the v8 anchors (adjudicator source-verified).

| Target | Anchor |
| --- | --- |
| **the M6/M7 orthogonality anchors (A is owner-watermark- AND, on the decoupled branch, `revokeEmbedded`-independent)** | `schema_drift.go:145-161` `LiveFingerprint` (reads the `striatumd.schema_state` singleton; a SEPARATE table from `owner_bundle_meta`, NO `applied_owner` input). `schema_drift.go:171-195` `RecordSchemaFingerprint` (UPSERTs the `schema_state` singleton with `ExpectedFingerprint()`; NO read of `owner_bundle_meta`). `schema_drift.go:83-100` `ExpectedFingerprint` (hashes EMBEDDED FILE BYTES — pure). A's decoupled complete branch (§3.3a step 3) reuses these reads only; the ONLY `revokeEmbedded`-reading predicate (step 0) does not fire when decoupled is ON. |
| **the M5 owner-watermark gate W** | `owner.go:124-154` `CheckOwnerBundleWatermark`; **`:145` `if applied == 0 { return nil }` (the fresh/no-authority SERVE exception, returns BEFORE the shortfall)**; **`:148-150` `if applied < RequiredOwnerBundleVersion { return &AwaitingOwnerDDLError{...} }` (the `1..19` shortfall halt)**; `:151-153` tolerate-forward / barrier (b); the comment block `:116-123` + `:140-143`. |
| **the fresh-DB representation (M5)** | `OwnerBundleVersion` `owner.go:226-247`; **`:234-235` `if present != "true" { return 0, nil }`**; `owner_pg_test.go:19` (a fresh migrated DB starts at version 0). |
| **serve-boot mutation site + the W→A→`:353` boot order** | `connection.go:332` `ConnectAndMigrate`; **watermark W `:349`**; **`ApplyMigrations` `:353`**; drift gate `:376-383`; **shadow log + fall-through `:384-393`**; **`RecordSchemaFingerprint` `:399` ("This is the only writer of schema_state")**. |
| boot fork / `--migrate` / typed-halt mapping | `authority_bootstrap.go:181,193`; `main.go:76`; typed halts `:208-227` (add `awaiting_deploy`/`awaiting_deploy_config`/`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` arms; `awaiting_owner_ddl` already mapped) |
| deployer command site | `daemon.go:67-81` dispatch (add `deploy`); `runDaemonOwnerDDL` `:90` (admin DSN `:115`); `runDaemonMigrate` `:167`; `EnvDaemonAdminDBURL` `:24` |
| runtime apply engine to reuse/wrap (C3) + M1 oracles | `migrations.go:121` `ApplyMigrations`; `applyOne` `:304-355`; `verifyRecordedHashTx` `:274` (`schema_migrations.sha256` `:275` — M1 runtime DB-stamp oracle); `MigrationSHASet` `:197` (M1 runtime byte oracle); `//go:embed sql/*.sql` `:21`; lock `:18` |
| owner-bundle engine / watermark + forward-tolerance (C2) / 0021 exclusion (M2/C3) | `owner.go:265-303` `ApplyOwnerBundles` (slice `:269` → `OwnerDDLApplyBundles()`); `:309-322` `applyPendingOwnerBundles` (`<= current` `:312`, in-loop guard); `:332-351` `ReapplyAllOwnerBundles` (nil-fallback `:336-342`, in-loop guard); `:367-374` `isCrossBundleDependencyError`; `:511-541` `applyOneOwnerBundle` (`owner_bundle_meta.sha256` stamp `:529` — M1 owner DB-stamp oracle); `OwnerBundles()` `:197-224` (FULL loader); `OwnerBundle.SHA256()` `:191` (M1 owner byte oracle); `LatestOwnerBundleVersion = 20` `:23` (**stays 20**); `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` `:35` (= 20, **stays 20**); `revokeEmbedded` = 0021 in `ownerBundleFS` (`//go:embed` `:156`) |
| pure read consuming the FULL owner-bundle loader (must keep seeing 0021) | `owner_runtime_ownership.go:51-52` `RuntimeOwnedTablesAlterable()` → `OwnerBundles()` |
| current owner-bundle frontier (NOT the revoke) | `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql` (the highest non-revoke bundle; the DDL-revoke is the NEW 0021) |
| C3 prerequisite the revoke-last fix relies on | `0018_runtime_table_ownership_transfer.sql:98` (`GRANT CREATE … TO striatumd_rw` before the transfer loop) — 0018 in the non-revoke prefix, CREATE held when runtime reconciles run |
| fingerprint contract (M1 mechanism + the M3 A3 pure-read comparison + the M6/M7 orthogonality) | `schema_drift.go:83-100` `ExpectedFingerprint`; `:105-137` `composeFingerprint` (reused for `plan_hash`); `:145-161` `LiveFingerprint`; `:171-195` `RecordSchemaFingerprint`; `:254-274` `CheckSchemaDrift`; `EnvSchemaDriftRefuse` `:28` |
| `deploy_cursor` + `deploy_plan` table model | `0043_schema_state.sql:39-44` (singleton CHECK), `:46-52` (`striatumd_rw` GRANT `DO` block) — model for both new 0044 tables |
| per-step / terminal receipt SD fn | `0001_authority_phase0.sql:152` (`append_audit_row`), `:238` (`GRANT EXECUTE … TO striatumd_rw`) |
| doctor block model | `go/pkg/reads/doctor_schema_drift.go` (`schemaDriftDoctorBlock`; `ExpectedFingerprint` call) |
| FMA-007 self-heal test to extend (M2 / F16b) | `owner_pg_test.go:219` `TestReapplyAllOwnerBundlesIsOrderedAndIdempotent` |
| two-role fixture for F12 / F16b / F17 / F18 / F18a | `go/pkg/pgtest/two_role.go:130` `ApplyOwnerBundles` (non-superuser two-role bootstrap) |

---

<sub>Committed proposal (v9) for the RFC 0142 P4 falsification-gate design run. Published by the
committer after the cycle-1 collaboration ledger cleared the gate (`accept_with_findings`): the single
binding finding **M7** (the §3.5/§4.5/F18 `complete`/decoupled/revoke-embedding row-16 cell asserted
unconditionally where A's decoupled complete branch is fingerprint-conditional and reads neither
`applied_owner` NOR `revokeEmbedded`) is GENUINELY RESOLVED via the prescribed **Option 1** — derive
row 16 from A under one coherent contract (conditional on A3 fingerprint-sync, identical to row 15),
§0.2 sub-invariants added, F18 parametric over the seven A-reaching complete-row cells, the four
`:399`-reaching cells unchanged — both falsifiers independently concede;
**M6(rows 13/15)/M5(row-1)/M3/M4/M1/M2/BC-N1/BC-N2/C1/C2/C3** carried forward intact; no new material
challenge stands unrebutted; two non-blocking build-phase obligations (finding **B1**) folded into
§6.5 as binding acceptance criteria for `rfc-0142-p4-build`. This SPEC is the artifact the
rfc-0142-p4-build run consumes contract-first (TDD). P5 (Q1/Q2) deferred; the local-first boundary
held.</sub>

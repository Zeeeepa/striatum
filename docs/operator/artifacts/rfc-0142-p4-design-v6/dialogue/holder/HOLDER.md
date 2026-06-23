# HOLDER (revised, v6) — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: holder-author-001

> **This is the SIXTH REVISION (v6).** Design-v1 returned `needs_revision` with C1/C2/C3.
> v2 resolved C1 + C2. v3 resolved C3 (ownership transfer, revoke-last) and closed N1, then
> returned `needs_revision` on BC-N1 + BC-N2. v4 resolved BOTH BC-N1 and BC-N2 and returned
> `needs_revision` on M1 + M2. v5 (`rfc-0142-p4-design-v5`) **resolved BOTH M1 and M2** — both
> v5 falsifiers AND the v5 adjudicator explicitly concede each, the v4 reproducers no longer
> reproduce in the deployer resume/finalizer path, and BC-N1/BC-N2/C1/C2/C3 carried forward
> intact — BUT returned `needs_revision` again (the gate's single allowed cycle) on **two new
> findings**:
>
> - **M3 (LOAD-BEARING)** — the COMPLETE-cursor window lets the LEGACY `ConnectAndMigrate`
>   path mutate + self-record AROUND the M1 `VerifyStoredTranscript` gate. v5 §3.3a
>   `CheckDeployActivation` returns nil immediately when `cursorState == complete` (defers to
>   the drift gate, v5 `HOLDER.md:480-482`), and the `revokeEmbedded && !decoupledEnabled →
>   awaiting_deploy_config` halt lives ONLY in the `cursorState == none` branch (v5
>   `HOLDER.md:483-489`). So a deployer-aware/revoke-embedding binary with a `complete` cursor
>   and `STRIATUM_DEPLOY_DECOUPLED` OFF takes the legacy path over a DB that DOES carry
>   `deploy_cursor`/`deploy_plan`; current source runs `ApplyMigrations`
>   (`go/pkg/db/connection.go:353`) BEFORE `CheckSchemaDrift` (`:376-383`) and
>   `RecordSchemaFingerprint` (`:399`), so the predicate returning nil lets the legacy mutator
>   AND the legacy self-record fire WITHOUT `VerifyStoredTranscript` — re-opening a #512-class
>   lockout and FALSIFYING §4.5 Universal Invariant B.
> - **M4 (secondary)** — F16's `TestOwnerDDLApplyExcludesRevokeBundle` asserts production
>   `OwnerBundles()` contains 0021 in rollout step 2 but 0021 is not authored until step 7, so
>   it cannot build green as written. A test-staging inconsistency, not a safety gap.
>
> This spec **starts from the v5 `HOLDER.md`** (a required context doc) and is a **proper
> revision**: it **resolves M3 and M4** per the prescribed fixes in the v5 collaboration ledger
> §4 ("What the revision must fix") and `SEED.md`, **produces the proactive-completeness
> boot-path decision table** (the new §3.5), and **carries M1, M2, BC-N1, BC-N2, C1, C2, and C3
> forward unregressed**. §0.1 below is the auditable resolution map; the cycle-1 falsifiers
> should verify each item there, then re-attack the REVISED §3.3a predicate (the M3 config gate
> + the no-revoke complete-cursor comparison), the new §3.5 decision table, the tightened §4.5
> Universal Invariant B, the M4 phase-aware F16 split (§3.2a / §5 / §6), and the new tests
> **F11 (extended)**, **F15 (extended)**, **F17** (the M3 complete-cursor refusal), and
> **F18** (the decision-table matrix).
>
> It is a falsifiable implementation spec for RFC 0142 **P4 only** — the one-shot deployer that
> lifts schema mutation out of serve-boot — that `rfc-0142-p4-build` can execute contract-first
> (TDD). RFC 0142 is accepted (D258); this does NOT re-open the five-layer design. Primary
> source: `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (Phasing P4 row at
> `:234`, Open Questions 3 + 4 at `:298,:302`, "the hard correctness core of P4" at `:255`).
> **Every anchor below is re-verified against current `main` (HEAD `df2a9e70`).** `git diff
> --stat 2e482077 HEAD -- go/` is **empty** — the only commits since the v5 anchor touched
> docs/workflow — so every v5 source anchor holds unchanged; the M3 boot order
> (`connection.go:349/:353/:376-383/:384-393/:399`) and the owner.go apply-route / watermark
> anchors (`owner.go:23,35,124,191,197,265,309,332,367,511`) are re-confirmed line-by-line in §9.

## 0. Root reframe held

Schema mutation must stop being an **implicit side effect of the serving process's restart**
and become an **explicit, ordered, resumable, provenance-tracked operation owned by a
dedicated deployer**. Then the serving daemon holds **zero create-DDL on the serving path**,
"restart force-commits a half-applied deploy" becomes structurally impossible, and a bad
migration can never wedge the single writer on boot. **M3 is exactly a residual violation of
this reframe**: in v5 the legacy `ApplyMigrations`-on-serve-boot path was still reachable for a
revoke-embedding binary on a `complete` cursor. The v6 revision closes that residual so the
deploy — never serve-boot — owns that mutation, while nothing pulls in P5 (rehearse /
expand-contract / fidelity tiering / clone = Q1/Q2).

The v6 revision adds one structural safety property on top of the v5 spec and restructures one
test, each per the v5 ledger's prescribed fix, **without re-opening any resolved mechanism**:

- **M3 — a revoke-embedding binary (or any binary over a transcript-carrying DB with a pending
  change) can NEVER reach the legacy `connection.go:399` mutate+self-record path.** The
  `revokeEmbedded && !decoupledEnabled` config halt is hoisted to fire for **every** cursor
  state including `complete`; a no-revoke binary over a `complete` transcript runs a
  pre-`ApplyMigrations`, DB-untouched fingerprint comparison that serves only when already
  in-sync. The exhaustive boot-path decision table (§3.5) proves Universal Invariant B in every
  cell.
- **M4 — F16 is split phase-aware**: a pre-0021 synthetic-list phase (F16a, rollout step 2)
  proves the exclusion-filter contract before 0021 exists; a post-0021 production phase (F16b,
  rollout step 7) proves the embed/listing split and the forced FMA-007 self-heal once 0021 is
  authored.

---

## 0.1 Addressing the design-v5 findings (the gating requirement)

The two binding v5 findings (M3, M4) are each resolved by a concrete sub-protocol bound to named
code sites — not a claim. M1, M2, BC-N1, BC-N2, C1, C2, and C3 are carried forward **verbatim**
from the v5 HOLDER and re-confirmed unregressed. The "where" column names the section that
specifies the fix and the named test that would refute it.

| Finding | v5 status | v6 disposition | Where / refuting test |
| --- | --- | --- | --- |
| **M3** the `complete`-cursor window lets a revoke-embedding binary + flag OFF ride the legacy `ConnectAndMigrate` mutate+self-record path around `VerifyStoredTranscript` | **OPEN** (falsifier_1; C2/decoupling-boundary + M1-Invariant-B core) | **RESOLVED: (1) the `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt is HOISTED to the TOP of `CheckDeployActivation`, BEFORE any cursor-state branch, so it fires for EVERY cursor state including `complete` (the conservative, binding rule). A revoke-embedding binary with `STRIATUM_DEPLOY_DECOUPLED` OFF returns `awaiting_deploy_config`, DB untouched, before `ApplyMigrations` (`connection.go:353`) and before `RecordSchemaFingerprint` (`:399`), on BOTH `ConnectAndMigrate` and `ConnectAndVerify`. (2) For the no-revoke residual (a no-revoke binary over a `complete` transcript with the flag OFF) the guard runs a pre-`ApplyMigrations`, DB-untouched plan/fingerprint comparison (`ExpectedFingerprint()` == recorded `LiveFingerprint` AND `cursor.plan_hash == expected`) — both PURE READS — and serves only when already in-sync (so `ApplyMigrations` is a no-op and the legacy self-record is an idempotent rewrite of the SAME fingerprint); on any divergence it halts `awaiting_deploy`, DB untouched, never auto-applying on serve-boot. (3) Universal Invariant B is TIGHTENED: the legacy `:399` writer is structurally reachable ONLY when no deploy transcript is present (`cursorState == none`) OR a `complete` transcript is already byte-in-sync, and NEVER with the revoke embedded (barrier (a): the M3 config gate) or the revoke applied (barrier (b): the C2 forward-watermark rule at `applied_owner >= 21`).** | §3.3a (REVISED predicate), §3.5 (the exhaustive decision table), §4.5 (tightened Invariant B), §1.3 (the `complete`+revoke-embedding row now halts) / **`T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord` (F17, new) + F11(g) + F18 (the decision-table matrix)** |
| **M4** F16's `TestOwnerDDLApplyExcludesRevokeBundle` asserts production `OwnerBundles()` contains 0021 in step 2 before 0021 is authored in step 7 — cannot build green | **OPEN** (falsifier_2; M2 test-staging, non-safety) | **RESOLVED: F16 is split into two phase-aware tests. F16a (pre-0021 / inert, lands rollout step 2) proves the exclusion-filter contract against a SYNTHETIC bundle list / test hook — `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`, both `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a hand-passed synthetic 0021, and `ReapplyAllOwnerBundles(nil,…)` uses the filtered loader — WITHOUT asserting production `OwnerBundles()` contains 0021. F16b (activation, lands rollout step 7 after 0021 is authored) asserts production `OwnerBundles()` contains 0021, `ExpectedFingerprint()` includes its bytes, `revokeEmbedded` derives from the full loader / file presence, and production `OwnerDDLApplyBundles()` excludes it; the forced-self-heal pgtest lives here and is required to reach `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError` (`owner.go:367-374`), not merely the pending loop.** | §3.2a (build-time guard split), §6 (rollout steps 2 + 7) / **`F16a` (synthetic phase) + `F16b` (production phase) + extended `F12`/`G-revoke-last`** |
| **M1** full stored-transcript byte + already-applied DB-stamp verification on resume AND as finalizer step 0 | **RESOLVED** (v5; both falsifiers + adjudicator concede the v4 reproducer no longer reproduces) | **CARRIED FORWARD VERBATIM, unregressed.** `VerifyStoredTranscript(plan_hash)` (§3.4a) checks the ENTIRE stored transcript (every step, already-applied AND not) `sha256` against the running binary's embedded bytes ⇒ `deploy_plan_binary_mismatch`; for already-applied entries it verifies the DB stamps ⇒ `deploy_plan_db_stamp_mismatch`; the SAME verifier runs on every resume AND as finalizer step 0. **M3 EXTENDS Invariant B's enforcement to the legacy/complete-cursor path; it does NOT re-open the deployer verifier.** | §3.4a, §1.2, §1.3, §3.4 / **`F15` (extended for the complete-cursor case) + F14 (already-applied tamper)** |
| **M2** the single non-revoke filter `OwnerDDLApplyBundles()` + `isNonRevokeBundle` excludes 0021 from EVERY `owner-ddl apply` route incl. the FMA-007 self-heal | **RESOLVED** (v5; both falsifiers concede 0021 unreachable, CREATE held) | **CARRIED FORWARD VERBATIM, unregressed.** The filter, the in-loop guards on `applyPendingOwnerBundles` + `ReapplyAllOwnerBundles`, the nil-fallback split, and the embed/listing split (`OwnerBundles()` keeps 0021 for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable`; `OwnerDDLApplyBundles()` never iterates it). **M4 only restructures F16's STAGING; it does NOT change the filter.** | §3.2a, §4.4, §4.5 (Invariant A) / **`F16a`/`F16b` (split) + extended F12 / `G-revoke-last`** |
| **BC-N1** immutable `deploy_plan` transcript materialized before step 0; resume off the stored transcript | **RESOLVED** (v4/v5; both falsifiers concede) | **CARRIED FORWARD VERBATIM, unregressed.** Migration 0044 `deploy_plan` keyed by `plan_hash`; materialize in the same tx as `in_progress(0)` before step 0; resume never recomputes `BuildPlan`; §1.3 + transcript-enumerated doctor + F14. | §1.2, §1.3, §3.2 / **`F14`** |
| **BC-N2** universal `revokeEmbedded`-independent `CheckDeployActivation` edge halting non-`complete` cursors at `applied_owner == 20` | **RESOLVED** (v4/v5; both falsifiers concede; M3 is the orthogonal `complete` window) | **CARRIED FORWARD VERBATIM, unregressed.** The universal incomplete-deploy edge still halts `awaiting_deploy` DB-untouched on every non-`complete` cursor; **M3's fix sits ABOVE it (the config gate) and BESIDE it (the `complete` branch) — it does NOT weaken the non-complete edge.** | §3.3a step 1, §4.2 / **`F11(e)/(f)` + extended `G-old-binary-refuse`** |
| **C1** the `finalizing` state + idempotent finalizer + §1.3 row + the M1 pre-finalizer gate | **INTACT** (v2–v5) | **CARRIED FORWARD VERBATIM, unregressed.** M3's harm flowed through the LEGACY `connection.go:399` writer — a DIFFERENT writer than the C1 deployer finalizer (which IS gated). M3 tightens Invariant B to cover the legacy writer; it does not touch the C1 finalizer. | §1.2, §1.3, §3.4 / **`F10` + F15 pre-finalizer arm** |
| **C2** `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion` KEPT at 20 | **INTACT** (v2–v5) | **CARRIED FORWARD VERBATIM, unregressed.** M3 EXTENDS this edge to the `complete`-cursor case (the hoisted config gate + the no-revoke comparison) — it does NOT advance `Required`, alter the watermark, or change the `applied >= 21` forward rule. | §3.3a, §4.2, §4.3 / **`F11`** |
| **C3** the DDL-revoke bundle 0021 special-cased + excluded from `owner-ddl apply` + applied terminal | **INTACT design** (v3–v5; both falsifiers concede) | **CARRIED FORWARD VERBATIM.** 0021 special-cased out of the owner prefix, applied as the TERMINAL plan step after every runtime ownership reconcile; `DDLRevokeOwnerBundleVersion = 21`; `LatestOwnerBundleVersion`/`Required` stay 20. M2 binds the executable exclusion; M3 keeps 0021 off the legacy serve-boot path. | §3.2, §3.2a, §3.3b, §4.4 / **`F12` + `G-revoke-last`** |

**Carried forward unchanged (v1–v5 ledgers — do not re-litigate):** Q4 (plain verb + three
seams, §2); the per-step Q3-A/Q3-B mid-step resumability **body** + step taxonomy (§1.1); the
shadow-first decoupling shape (default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify`
preserving the P2 watermark interlock + P3 drift gate, the embed-FS-derived plan, the
hash-chained receipt, `doctor schema_deploy_unrecorded`); the M1 `VerifyStoredTranscript`; the
M2 non-revoke filter. **The v6 revision touches only the activation predicate (M3, §3.3a), the
new decision table (§3.5), the tightened Invariant B (§4.5), and F16's staging (M4, §3.2a/§6).**

---

## 1. Decision Q3 — How atomic is "atomic"? (the hard correctness core)

**Claim (Q3-DECISION).** The **per-step-atomic + resumable-cursor** contract is sufficient for
*every* owner+runtime interleaving AND every boot-path combination P4 ships, **provided**: (a)
plan identity is an **immutable durable fact materialized before step 0** and resume reads it
(BC-N1); (b) the two per-step sub-rules (Q3-A/Q3-B) hold for both the schema side effect AND the
per-step receipt, keyed on the **stored transcript's** `(plan_hash, step_index)`; (c) the
finalization boundary is closed by a distinct `finalizing` state + an idempotent finalizer (C1);
(d) **no boot — including a no-revoke deployer-aware boot — can serve while the cursor is
non-`complete`** (BC-N2); (e) on every resume AND before the finalizer self-records, the ENTIRE
stored transcript byte-matches the running binary and the already-applied DB stamps match the
transcript; any mismatch halts the deploy DB-untouched and NO fingerprint is written (M1); **and
(f) NEW in v6 (M3) — NO boot reaches the legacy `ApplyMigrations`/`RecordSchemaFingerprint`
mutate+self-record path while a deploy transcript with a pending change is present, and NO
revoke-embedding binary EVER reaches that legacy path: the activation guard is the sole authority
for "may this boot auto-apply/self-record on serve-boot," evaluated before `ApplyMigrations` in
both boot paths, and it is provably correct in EVERY `cursorState × decoupledEnabled ×
revokeEmbedded × applied_owner` cell (§3.5).**

- **Q3-A / Q3-B** (carry-forward body, unchanged from v4/v5; receipt keyed off the stored
  transcript). See §1.1 and §3.3b.
- **Q3-V — full-transcript verification (M1, carry-forward).** `VerifyStoredTranscript(plan_hash)`
  (§3.4a) on every resume and as finalizer step 0; the resume/finalize gate that makes Q3-A/Q3-B
  safe under a changed binary.
- **Q3-W — boot-path activation completeness (NEW in v6, the M3 resolution).** On EVERY boot, the
  activation guard `CheckDeployActivation` (§3.3a, REVISED) decides — before any mutation or
  self-record — whether the boot may serve-legacy, serve-verify, or must halt. §3.5 proves the
  decision is fail-closed for the whole cell space: the only cells that reach the legacy
  `connection.go:399` writer are `cursorState == none` (no transcript: fresh-DB / inert legacy)
  and `complete`-and-already-in-sync (an idempotent rewrite of the same fingerprint); no
  revoke-embedding binary and no transcript-with-a-pending-change ever reaches it.

### 1.1 Step taxonomy (the closed set P4 ships) — unchanged from v1–v5 (re-anchored to current `main`)

| Class | Examples | Transactional? | Protocol | Idempotent form |
| --- | --- | --- | --- | --- |
| **T-DDL** runtime | `CREATE TABLE`, additive `ALTER … ADD COLUMN`, `CREATE INDEX` (non-concurrent), the new runtime migrations | Yes | **Q3-A** (step + ownership-reconcile + version stamps + cursor + **receipt** in one tx) | `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`; re-run is a no-op |
| **T-DDL** owner | owner-bundle DDL, **incl. the terminal DDL-revoke bundle 0021** | Yes (each bundle applies in one tx that stamps `owner_bundle_meta` last — `owner.go:511-541`) | **Q3-A**, reusing the per-bundle transaction; cursor advance + **receipt** join that tx | bundles idempotent (`IF NOT EXISTS` + `ON CONFLICT (version) DO NOTHING`) |
| **NT-DDL** | `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, `DROP INDEX CONCURRENTLY` | No | **Q3-B** (pre-marker → step → reconcile: keyed receipt + post-marker) | `CREATE INDEX CONCURRENTLY IF NOT EXISTS`; resume reconciler drops a left-behind `INVALID` index then recreates; `ADD VALUE … IF NOT EXISTS` |
| **DML-backfill** (bounded in P4; full online backfill is P5) | a one-shot data normalize a runtime migration carries | Yes if small; batched otherwise | Q3-A small / Q3-B batched | `WHERE <not-yet-done>` predicate; re-run resumes from the unprocessed tail |

> The terminal DDL-revoke bundle 0021 is a T-DDL owner step (the existing per-bundle transaction,
> `owner.go:511-541`), so its receipt commits atomically. NT-DDL never creates a new ownable
> object whose owner is wrong, so the C3 ownership reconciliation (§3.3b) is needed only on
> transactional object-creating steps.

### 1.2 `deploy_cursor` + the immutable `deploy_plan` transcript (BC-N1) and crash-resume semantics (C1)

Both are **new runtime-owned tables**, created by runtime migration **0044** (modeled
byte-for-byte on `0043_schema_state.sql:39-52`). Migration 0044 is additive, runtime-owned,
carries no owner DDL.

**`deploy_cursor` (singleton):** `id text PK CHECK (id='singleton')`, `plan_hash text NOT NULL`,
`state text NOT NULL`, `step_index int NOT NULL`, `step_id text NOT NULL`, `updated_at`.
**`deploy_plan` (append-only, keyed by `plan_hash`):** the IMMUTABLE ordered transcript
`steps jsonb` = `[{step_index, step_id, role, sha256, transactional}]`, revoke last,
`revoke_step_index`, `base_owner_version`, `base_runtime_version`, `target_*`, INSERT-once
(`ON CONFLICT (plan_hash) DO NOTHING`).

`state ∈ { idle, in_progress, step_committed, finalizing, complete, aborted }`. Lifecycle (carry-forward):

```
materialize(plan_hash, transcript) + idle─▶in_progress(0) ─▶ step_committed(0) ─▶ … ─▶ step_committed(N-1=0021) ─▶ finalizing ─▶ complete
   [deploy_plan row + cursor=in_progress(0) in ONE tx, BEFORE step 0]   │
                                                                         └─(operator abort / fatal)──▶ aborted
```

- **Materialization (BC-N1).** After the substrate-ensure preamble (§3.2), compute
  `BuildPlan(base_owner, base_runtime)` once, derive `plan_hash`, and in one transaction INSERT
  the immutable `deploy_plan` row and set `deploy_cursor → in_progress(0)` — before step 0.
- **Resume verification (M1).** On every resume, immediately after loading
  `deploy_plan[cursor.plan_hash]` and BEFORE applying or finalizing any step, run
  `VerifyStoredTranscript(plan_hash)` (§3.4a) over the WHOLE transcript.
- **Finalization (C1 + the M1 pre-finalizer gate).** When the terminal step is `step_committed`,
  advance to `finalizing`, then run the idempotent finalizer (§3.4): **(0)** run
  `VerifyStoredTranscript` and abort on mismatch (writing NOTHING); **(1)** append the guarded
  `complete` receipt; **(2)** `RecordSchemaFingerprint` UPSERT; **(3)** advance `finalizing →
  complete` last. The deployer finalizer is the C1 writer of `schema_state` on the deploy path —
  it is gated by step (0); the LEGACY `connection.go:399` writer is governed by the §3.3a
  activation guard (M3), proven unreachable over a transcript-with-a-pending-change in §3.5.

### 1.3 The load-bearing disambiguation (C1 + BC-N1 + BC-N2 + M1 + M3)

`deploy_cursor` is the **second signal** alongside the drift gate's `LiveFingerprint ==
ExpectedFingerprint`. The rows carry forward from v5; **the two `complete`-cursor rows are
revised for M3** (a revoke-embedding binary with the flag OFF now halts `awaiting_deploy_config`
for EVERY cursor state, and a no-revoke binary over a `complete` transcript serves only when the
pre-`ApplyMigrations` comparison shows in-sync).

| `deploy_cursor.state` | condition (vs the **stored transcript** + the binary) | classification | action |
| --- | --- | --- | --- |
| `complete` | `deploy_plan[plan_hash]` present; full transcript byte-matches binary; `== ExpectedFingerprint`; **`decoupledEnabled` (or a no-revoke binary already in-sync, §3.3a)** | **in sync** | **serve** (verify-only when decoupled; legacy no-op-apply when a no-revoke binary is already in-sync) |
| `complete` | **`revokeEmbedded == true` AND `decoupledEnabled == false`** (M3) | **config halt** | refuse-to-serve **`awaiting_deploy_config`**, DB untouched — never reaches `ApplyMigrations`/`:399` (set `STRIATUM_DEPLOY_DECOUPLED=1` to serve verify-only / run `deploy` to finish a pending change) |
| `complete` | **no-revoke binary, flag OFF, but `ExpectedFingerprint() ≠` recorded OR `cursor.plan_hash ≠ expected`** (M3 comparison) | **pending change over a transcript** | refuse-to-serve **`awaiting_deploy`**, DB untouched — never auto-applies on serve-boot; route the change through `deploy` |
| `in_progress` / `step_committed` | present; the FULL transcript byte-matches the binary AND already-applied DB stamps match | **incomplete, resume off the STORED transcript** (BC-N1) | refuse-to-serve `awaiting_deploy`; `deploy` resumes at `cursor.step_index` |
| `in_progress` / `step_committed` | present but the cursor's `plan_hash` ≠ the binary's freshly-computed pending plan | **incomplete, resume with the STORED plan** (BC-N1) | refuse-to-serve `awaiting_deploy`; resume off the stored transcript by `cursor.plan_hash` — NOT drift |
| `finalizing` | present; full transcript byte-matches binary | **resumable finalization** (C1) | refuse-to-serve `awaiting_deploy`; re-run runs the idempotent finalizer (which re-verifies, §3.4 step 0) |
| `in_progress`/`step_committed`/`finalizing`, `deploy_plan[plan_hash]` ABSENT, **OR ANY stored step `sha256` ≠ the binary's embedded bytes** | full-transcript mismatch (M1) | **`deploy_plan_binary_mismatch`** | refuse-to-serve; apply NOTHING; operator inspects |
| `in_progress`/`step_committed`/`finalizing`, transcript byte-matches, **but an already-applied step's DB stamp ≠ the stored transcript** (M1) | DB-stamp mismatch | **`deploy_plan_db_stamp_mismatch`** | refuse-to-serve; do NOT finalize; apply NOTHING |
| absent `deploy_cursor` table / `state == idle` (NO transcript) | n/a | **no deploy in flight** | **legacy path (flag OFF) + no-revoke binary:** serve (auto-apply, the legitimate `:399` cell); **decoupled (flag ON) OR revoke-embedding binary:** refuse `awaiting_deploy`/`awaiting_deploy_config` (C2 + M3 — §3.3a) |
| `aborted` | n/a | **aborted** | refuse-to-serve `awaiting_deploy` |
| `complete`, `plan_hash` ≠ expected (foreign) | foreign | **genuine drift** | refuse-to-serve (`schema_drift`) |

Five load-bearing properties this table now enforces (the fifth is new in v6):

- **(BC-N1)** Plan identity is a stored fact; resume reads `deploy_plan[cursor.plan_hash]`.
- **(M1)** A resume/finalize binary that disagrees with ANY stored step is forced to
  `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`; neither writes a fingerprint.
- **(C1)** The `complete`-with-missing-receipt window cannot occur; `complete` ⇒ 0021 applied.
- **(BC-N2)** `incomplete cursor never serves` holds for EVERY deployer-aware binary.
- **(M3 — NEW)** **A `complete` cursor no longer short-circuits the activation guard.** A
  revoke-embedding binary with the flag OFF halts `awaiting_deploy_config` for EVERY cursor state
  (including `complete`); a no-revoke binary over a `complete` transcript serves only when the
  pre-`ApplyMigrations` comparison proves it already in-sync — so the legacy `connection.go:399`
  writer is unreachable over any transcript carrying a pending change, and unreachable for any
  revoke-embedding binary.

---

## 2. Decision Q4 — Is a deploy itself a Striatum run? (carry-forward, sound)

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape door held open
by three seams.** Unchanged from v1–v5; no falsifier contested it. The bootstrapping paradox is
dispositive (a dogfooded run needs the daemon + schema up, which is exactly what the deploy
brings online), so the base deployer is a plain verb against a bare/owner-only DB, the same
context `daemon owner-ddl apply` (`daemon.go:90`) occupies. The door stays open because (1) the
plan is a content-addressed durable artifact (`plan_hash` + `deploy_plan`); (2) the receipt is
hash-chained into `audit_log`; (3) the verb is a thin CLI over a `Deployer.Apply(ctx, plan,
cursor)` core a future run step invokes unchanged.

---

## 3. The deployer surface + serve-boot decoupling (shadow-first, by exact code site)

### 3.1 Command site (carry-forward)

Add `case "deploy":` to `RunDaemon` (`daemon.go:67-81`) → `runDaemonDeploy(...)`. DSN resolution
mirrors `runDaemonOwnerDDL` (`daemon.go:90,115`). Flags: `--plan`, `--dry-run`, `--resume`
(default), `--abort`, `--json`. `docs/reference/command-authority-matrix.md` + the authority
guardrail tests gain the `daemon.deploy` row.

**Connection model (load-bearing, carry-forward).** The deployer applies **every deploy-plan step
over the SINGLE owner connection**. The only write routed over a separate runtime view is the C1
finalizer's terminal `schema_state` self-record (mirroring `connection.go:399`) — gated by
`VerifyStoredTranscript` (§3.4a, M1).

### 3.2 The deploy plan — materialized ONCE into an immutable transcript (BC-N1), revoke 0021 sorted last (C3)

Carry-forward from v5, unchanged. Plan identity is computed once at deploy start from the two
embedded `embed.FS` sources the fingerprint hashes (runtime `migrations.go:21`; owner
`owner.go:156`) and persisted as the immutable `deploy_plan` transcript before step 0. The
substrate-ensure preamble applies 0044 idempotently before transcript materialization (0044 is
never a numbered step). `BuildPlan` uses the FULL `OwnerBundles()` loader (includes 0021) but
special-cases the revoke to terminal: pending non-revoke owner bundles (≤20, via
`isNonRevokeBundle`) → pending runtime migrations (≥0045) → the terminal 0021 if pending.
`plan_hash` = sha256 of the canonical transcript (base + ordered `step_id`+`sha256`, 0021 last);
the fingerprint is over the *set* (order-independent) so 0021-last changes `plan_hash` but not the
fingerprint. `step_index` is stable by storage, not recomputation (BC-N1).

### 3.2a The single non-revoke filter + the embed/listing split (M2 — carry-forward) + the M4 phase-aware test staging

**Claim (NON-REVOKE-FILTER — M2, carry-forward verbatim).** The DDL-revoke bundle 0021 is
excluded from **every** `owner-ddl apply` route by a single named filter and a split loader, so
0021's `REVOKE CREATE` can ONLY ever be committed as the terminal `deploy` step (via
`applyOneOwnerBundle`), NEVER via the pending loop, the FMA-007 self-heal reapply, a nil-fallback,
a dry-run/list surface, or a test helper.

```go
// DDLRevokeOwnerBundleVersion identifies the RFC 0142 P4 C3 DDL-revoke bundle.
// DEPLOY-PLAN-TERMINAL ONLY. LatestOwnerBundleVersion / RequiredOwnerBundleVersion stay 20.
const DDLRevokeOwnerBundleVersion = 21
func isNonRevokeBundle(b OwnerBundle) bool { return b.Version < DDLRevokeOwnerBundleVersion }
func OwnerDDLApplyBundles() ([]OwnerBundle, error) { /* filter(OwnerBundles(), isNonRevokeBundle) */ }
```

**Bindings (every `owner-ddl apply` route, current `main`), unchanged from v5:** (1)
`ApplyOwnerBundles` (`owner.go:265-303`) loads `OwnerDDLApplyBundles()` at `:269`; (2)
`applyPendingOwnerBundles` (`:309-322`) keeps its `<= current` bound (`:312`) + an in-loop
`isNonRevokeBundle` guard; (3) `ReapplyAllOwnerBundles` (`:332-351`) adds the same in-loop guard
+ its nil-fallback (`:336-342`) loads `OwnerDDLApplyBundles()`; (4) the self-heal branch
(`:277-302`) is bound by (1) AND (3); (5) all test callers inherit the filter; (6) no owner-ddl
dry-run/list surface exists today. **The embed/listing split:** `OwnerBundles()` (`:197-224`)
stays the full loader (includes 0021) for `revokeEmbedded`, `ExpectedFingerprint`
(`schema_drift.go:88`), `BuildPlan`, and `RuntimeOwnedTablesAlterable`
(`owner_runtime_ownership.go:52`); `OwnerDDLApplyBundles()` is the disjoint apply slice.

**Build-time guard — split phase-aware (M4 — the v6 resolution).** The v5 single test
`TestOwnerDDLApplyExcludesRevokeBundle` asserted production `OwnerBundles()` contains 0021 in
rollout step 2, before 0021 is authored (step 7) — un-buildable. It is split into two tests that
each land green in their own rollout phase:

- **`TestOwnerDDLApplyExcludesSyntheticRevokeBundle` (F16a — pre-0021 / inert, rollout step 2).**
  Drives the filter through a **synthetic bundle list / test hook** (a hand-built
  `[]OwnerBundle` containing a synthetic `{Version: 21}` entry, and/or an injectable `ownerBundleFS`
  test seam): asserts (a) `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`
  from the synthetic list; (b) `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a
  hand-passed synthetic 0021; (c) `ReapplyAllOwnerBundles(nil, …)` resolves its fallback to the
  filtered loader. It does **NOT** assert production `OwnerBundles()` contains 0021 (it does not
  yet). This proves the exclusion-filter contract while the filter is a harmless no-op on the real
  ≤20 bundle set.
- **`TestOwnerDDLApplyExcludesProductionRevokeBundle` (F16b — activation, rollout step 7, after
  0021 is authored).** Asserts (a) production `OwnerBundles()` DOES contain 0021 (so
  `revokeEmbedded`/`ExpectedFingerprint` see it); (b) `ExpectedFingerprint()` includes 0021's
  bytes; (c) `revokeEmbedded` derives from the full loader / file presence in `ownerBundleFS`; (d)
  production `OwnerDDLApplyBundles()` excludes 0021. The forced-self-heal pgtest (below) lives here.
- A build-time grep test (in step 2) asserts no production `applyOneOwnerBundle` caller outside the
  deployer terminal-step path receives a bundle `>= 21`.

### 3.3 Lifting `ApplyMigrations` out of serve-boot (shadow-first; carry-forward + the M3 tightening)

**Claim (DECOUPLE).** Serve-boot stops mutating schema, behind a flag, shadow-first. The single
mutation site on the serving path is `ConnectAndMigrate` (`connection.go:332-404`).

1. **New env `STRIATUM_DEPLOY_DECOUPLED`** (default OFF — shadow-first). OFF → boot behaves as
   today for a NO-revoke binary; the decoupling lands to `main` inert.
2. When **ON**, `ConnectAndMigrate` becomes **`ConnectAndVerify`**: keep the watermark interlock
   (`:349`) and the drift gate (`:376-383`) but do NOT call `ApplyMigrations` (`:353`); run the
   §1.3 classification and serve only on the in-sync row; the drift self-record (`:399`) is NOT
   run on the verify path.
3. **M3 tightening (NEW in v6).** A **revoke-embedding binary** (the binary ships 0021) is
   **structurally barred from the legacy `ConnectAndMigrate` `ApplyMigrations`/`:399` path** by
   the hoisted config gate in `CheckDeployActivation` (§3.3a): with the flag OFF it halts
   `awaiting_deploy_config` BEFORE `:353`, for EVERY cursor state including `complete`. So once a
   binary embeds the revoke, its ONLY serving mode is the decoupled `ConnectAndVerify` path
   (flag ON) — the legacy mutator is unreachable. This is the operational end-state of P4:
   **decoupling becomes mandatory exactly when the binary embeds the revoke.**
4. **No window serving an unmigrated schema (R2).** The decoupled boot verifies before serving;
   the watermark interlock is preserved verbatim.

#### 3.3a The deploy-activation interlock — REVISED for M3 (the hoisted config gate + the no-revoke complete comparison) on top of the universal pre-revoke cursor edge (BC-N2) + the C2 arms

A **`CheckDeployActivation(ctx, runner, revokeEmbedded, decoupledEnabled bool) error`** (in
`go/pkg/db/owner.go` or a new `deploy_activation.go`) is called at the same boot site, immediately
after `CheckOwnerBundleWatermark` (`connection.go:349`) and **BEFORE `ApplyMigrations`** (`:353`),
in both `ConnectAndMigrate` and `ConnectAndVerify`, and before `RecordSchemaFingerprint` (`:399`).
It mutates nothing. It reads `deploy_cursor` defensively (absent table/row → `cursorState = none`).
**Predicate (fail-closed), in order — REVISED for M3; the steps marked NEW/REVISED are the M3
fix, the rest is carried forward verbatim from v5:**

0. **NEW (M3 — the hoisted universal decoupling-config gate, fires FIRST for EVERY cursor state).**
   If **`revokeEmbedded && !decoupledEnabled`** → return **`awaiting_deploy_config`**, DB
   untouched. *A binary that ships the DDL-revoke (0021) must run on the decoupled path; it must
   NEVER take the legacy `ConnectAndMigrate` mutate+self-record path, regardless of cursor state —
   `none`, `in_progress`, `finalizing`, OR `complete`.* This is the load-bearing M3 close: it
   moves the v5 `none`-branch-only halt to the TOP so the `complete` cursor can no longer
   short-circuit it.
1. **UNIVERSAL incomplete-deploy edge (BC-N2 — carry-forward verbatim).** If `cursorState ∈
   {in_progress, step_committed, finalizing}` → return **`awaiting_deploy`**, DB untouched. Holds
   at `applied_owner == 20`, so a no-revoke landing binary that restarts mid-deploy halts BEFORE
   `ApplyMigrations`/`RecordSchemaFingerprint`. *(For a revoke-embedding binary + flag OFF, step 0
   already halted `awaiting_deploy_config` — either way DB-untouched, no serve, no mutation.)*
2. If `cursorState == aborted` → return **`awaiting_deploy`**, DB untouched.
3. If `cursorState == complete`:
   - **`decoupledEnabled == true`** → defer to the drift gate on the VERIFY path: `plan_hash ==
     expected` + `LiveFingerprint == ExpectedFingerprint` → **serve verify-only** (return nil; no
     `ApplyMigrations`, no `:399`); foreign `plan_hash` or fingerprint mismatch → **`awaiting_deploy`**
     (or `schema_drift` under refuse mode). *(`revokeEmbedded` is irrelevant here — step 0 only
     fires when the flag is OFF.)*
   - **`decoupledEnabled == false`** (⇒ `revokeEmbedded == false`, since step 0 caught
     revoke-embedding + flag-OFF): **NEW (M3 — the no-revoke complete comparison), a
     pre-`ApplyMigrations`, DB-untouched, PURE-READ check:** if **`ExpectedFingerprint() ==
     LiveFingerprint(recorded)` AND `cursor.plan_hash == expected`** → **return nil**: the binary
     is already in-sync with the completed deploy, so the subsequent legacy `ApplyMigrations` is a
     no-op and the legacy `:399` self-record is an idempotent rewrite of the SAME fingerprint —
     serve. Otherwise (a pending change / a divergent binary over the transcript) → **return
     `awaiting_deploy`**, DB untouched: the legacy mutator is NOT entered; route the change through
     `deploy`. *(`ExpectedFingerprint()` is a pure byte-hash, `schema_drift.go:83-100`;
     `LiveFingerprint` is a pure singleton read, `schema_drift.go:145-160`; `plan_hash` is a pure
     read + compute — so this branch mutates nothing and cannot self-record.)*
4. If `cursorState == none` (absent table/row, or `idle` — NO transcript present):
   - `decoupledEnabled == true` → **`awaiting_deploy`** (the decoupled boot never auto-applies;
     fresh-DB bring-up runs `deploy`).
   - `decoupledEnabled == false` (⇒ `revokeEmbedded == false`, by step 0):
     - **return nil**: legacy path, serve, `ApplyMigrations` runs. The inert-landing binary's
       normal boot and fresh-DB bring-up. **This is the ONLY cell that reaches the legacy `:399`
       writer for an apply that mutates — and it carries NO deploy transcript.**

`revokeEmbedded` = the embedded owner-bundle FS contains the bundle at `DDLRevokeOwnerBundleVersion
= 21`. This is "the binary ships the revoke," NOT `Latest >= 21` (`LatestOwnerBundleVersion` stays
20). `decoupledEnabled` = `STRIATUM_DEPLOY_DECOUPLED` is set.

**Net invariant (the M3 + BC-N2 + C2 close):** **No boot serves while the cursor is non-`complete`;
no revoke-embedding binary ever reaches `ApplyMigrations`/`:399`; and the legacy `:399` writer is
reached ONLY by a no-revoke binary with the flag OFF over either NO transcript (`cursorState ==
none`) or a `complete` transcript it is ALREADY byte-in-sync with (where the write is an idempotent
rewrite of the same fingerprint).** Proven exhaustively in §3.5.

#### 3.3b Runtime-object ownership: Policy 1 with the revoke sequenced last (C3 — carry-forward, re-anchored)

Carry-forward from v5, unchanged. `Deployer.applyRuntimeStep` (wrapping `applyOne`,
`migrations.go:304-355`): `BEGIN` (owner conn) → assert
`has_schema_privilege('striatumd_rw','striatumd','CREATE')` (else `deploy_create_prerequisite_missing`)
→ snapshot owner-owned oids → run migration SQL + both version stamps + in-tx hash verify →
re-snapshot, `ALTER <kind> … OWNER TO striatumd_rw` for new oids → re-assert DML grants → append
the per-step receipt keyed on the stored `(plan_hash, step_index)` → advance `in_progress(k) →
step_committed(k)` → `COMMIT`. The reconcile runs while `striatumd_rw` still holds CREATE (granted
by bundle 0018 at `0018:98`, in the non-revoke prefix); 0021 commits last, so the steady state
denies CREATE.

### 3.4 The deploy receipt + the idempotent finalizer (C1) + the M1 pre-finalizer gate (carry-forward)

Carry-forward from v5, unchanged. Every applied step writes exactly one hash-chained receipt via
`append_audit_row(...)` keyed on the stored `(plan_hash, step_index)`. The idempotent finalizer:
**(0)** `VerifyStoredTranscript(plan_hash)` (M1) — runs FIRST, writes nothing, ABORTS on
`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`; **(1)** guarded `complete` receipt;
**(2)** `RecordSchemaFingerprint` (`schema_drift.go:171-195`) UPSERT; **(3)** advance `finalizing →
complete` last. The finalizer crash-point table and `doctor schema_deploy_unrecorded` (transcript-
enumerated + the M1 stamp/byte WARN) carry forward verbatim.

### 3.4a `VerifyStoredTranscript` — the full-transcript byte + DB-stamp verifier (M1, carry-forward)

Carry-forward from v5, unchanged. A single PURE-READ function checks, for EVERY step in
`deploy_plan[plan_hash].steps` (already-applied AND not), `step.sha256 ==` the running binary's
embedded bytes (`MigrationSHASet()` `migrations.go:197` for runtime; `OwnerBundle.SHA256()`
`owner.go:191` for owner) ⇒ `ErrDeployPlanBinaryMismatch`; and for every already-applied step,
`step.sha256 ==` the DB stamp (`schema_migrations.sha256` `migrations.go:275` / `owner_bundle_meta.sha256`
`owner.go:529`) ⇒ `ErrDeployPlanDBStampMismatch`. Called (i) on every resume before any apply,
(ii) as finalizer step 0, (iii) by the doctor. The two typed halts join the
`AwaitingOwnerDDLError`/`SchemaDriftError` `errors.As` arms (`main.go:208-227`); all map to
refuse-to-serve, DB untouched.

---

## 3.5 The complete boot-path decision table (proactive completeness — closes M3, preempts M5)

> **The SEED requirement.** A revision that pins only the two §4 items but leaves an unaudited
> boot-path combination open spawns an M5 (the v5 SEED warned an "unaudited sibling self-record
> path" would spawn M3 — and it did). This table closes that class entirely: for EVERY
> `cursorState × decoupledEnabled × revokeEmbedded × applied_owner` cell it states the exact
> guard/outcome and PROVES §4.5 Universal Invariant B holds. It is an **executable, named
> requirement** (**F18**, `T-deploy-bootpath-decision-table`) the falsifiers verify against the
> revised §3.3a predicate sites and the `connection.go:349/:353/:376-383/:399` ordering.

**Two guard sites, evaluated in order on every boot, both BEFORE `ApplyMigrations` (`:353`) /
`RecordSchemaFingerprint` (`:399`), mutating nothing:**

- **W = `CheckOwnerBundleWatermark` (`connection.go:349`, `owner.go:124-154`).** `applied_owner <
  20` → `awaiting_owner_ddl` (P2 interlock; owner DDL pending — pre-existing). `applied_owner ==
  20` → in-sync, proceed to A. `applied_owner >= 21` (0021 applied): a **revoke-embedding** binary
  tolerates forward → proceed to A; a **no-revoke** binary → **`awaiting_deploy`** (the C2
  forward-watermark rule at `owner.go:151-153`, re-anchored to 21 — a rolled-back binary on a
  revoke-applied DB). This is **barrier (b)** that keeps a no-revoke binary off a revoke-applied DB.
- **A = `CheckDeployActivation` (§3.3a, REVISED).** Step 0 (M3 config gate) is **barrier (a)** that
  keeps every revoke-embedding binary off the legacy path.

`applied_owner < 20` ALWAYS halts at W (`awaiting_owner_ddl`, DB untouched), so the table below
specifies `applied_owner ∈ {==20, >=21}`; the `<20` column is uniformly `awaiting_owner_ddl`. The
16 `(cursorState × decoupledEnabled × revokeEmbedded)` rows × 3 `applied_owner` sub-columns = **48
cells**, all specified. `cursorState` groups `step_committed` with `in_progress` and treats
`aborted` like the non-complete edge (both → `awaiting_deploy` at A step 1/2), per §1.3.

| # | cursorState | decoupled | revoke | applied_owner `<20` | applied_owner `==20` | applied_owner `>=21` |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | none | off | no | `awaiting_owner_ddl` (W) | **SERVE-legacy** (A4: none/legacy; `:399` reachable — NO transcript) | `awaiting_deploy` (W fwd-watermark, barrier b) |
| 2 | none | off | yes | `awaiting_owner_ddl` (W) | **`awaiting_deploy_config`** (A0, barrier a) | **`awaiting_deploy_config`** (A0; W tolerates revoke, then barrier a) |
| 3 | none | on | no | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A4: decoupled never auto-applies; run deploy) | `awaiting_deploy` (W fwd-watermark) |
| 4 | none | on | yes | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A4: run deploy to apply 0021) | `awaiting_deploy` (A4; cursor=none+0021-applied is inconsistent → conservative halt) |
| 5 | in_progress/step_committed/aborted | off | no | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A1/A2, BC-N2) | `awaiting_deploy` (W fwd-watermark) |
| 6 | in_progress/step_committed/aborted | off | yes | `awaiting_owner_ddl` (W) | **`awaiting_deploy_config`** (A0, barrier a) | **`awaiting_deploy_config`** (A0) |
| 7 | in_progress/step_committed/aborted | on | no | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_deploy` (W fwd-watermark) |
| 8 | in_progress/step_committed/aborted | on | yes | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_deploy` (A1/A2; W tolerates revoke) |
| 9 | finalizing | off | no | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A1, BC-N2) | `awaiting_deploy` (W fwd-watermark) |
| 10 | finalizing | off | yes | `awaiting_owner_ddl` (W) | **`awaiting_deploy_config`** (A0, barrier a) | **`awaiting_deploy_config`** (A0) |
| 11 | finalizing | on | no | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A1, BC-N2) | `awaiting_deploy` (W fwd-watermark) |
| 12 | finalizing | on | yes | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A1, BC-N2 resume finalize) | `awaiting_deploy` (A1; W tolerates revoke) |
| 13 | complete | off | no | `awaiting_owner_ddl` (W) | **SERVE-legacy if in-sync, else `awaiting_deploy`** (A3 no-revoke comparison; `:399` only on the idempotent no-op rewrite) | `awaiting_deploy` (W fwd-watermark) |
| **14** | **complete** | **off** | **yes** | `awaiting_owner_ddl` (W) | **`awaiting_deploy_config`** (A0, barrier a — **the M3 cell**) | **`awaiting_deploy_config`** (A0 — **the post-deploy steady state with the flag erroneously OFF; the v5 reproducer, now halted**) |
| 15 | complete | on | no | `awaiting_owner_ddl` (W) | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled) | `awaiting_deploy` (W fwd-watermark) |
| **16** | **complete** | **on** | **yes** | `awaiting_owner_ddl` (W) | `awaiting_deploy` (A3 decoupled: 0021 pending → not in-sync → run deploy) | **SERVE-verify** (A3 decoupled, in-sync — **the steady-state P4-activated serving cell; zero serve-path DDL**) |

**Universal Invariant B proven in every cell.** The legacy `connection.go:399` writer mutates
schema / self-records around `VerifyStoredTranscript` only if a boot reaches `:353`/`:399` on the
legacy `ConnectAndMigrate` path. Across all 48 cells the ONLY cells that reach `:399` are:

- **Cell 1 / `==20`** (`none`, off, no-revoke): NO transcript present, the running binary IS the
  applier of its own just-applied schema (fresh-DB bring-up / inert legacy). Invariant B is not in
  scope — there is no transcript to write around. ✓
- **Cell 13 / `==20`, in-sync** (`complete`, off, no-revoke, `ExpectedFingerprint() == recorded`):
  the pre-`ApplyMigrations` comparison (A3) already proved in-sync, so `ApplyMigrations` is a no-op
  and `:399` rewrites the IDENTICAL fingerprint. No mutation, no change to `schema_state`. ✓

Every other cell either halts before `:353` (DB byte-identical) or takes the decoupled
`ConnectAndVerify` path (which never calls `:353`/`:399`):

- **Every revoke-embedding + flag-OFF cell (2, 6, 10, 14)** → `awaiting_deploy_config` at A0
  (barrier a). `ApplyMigrations` NOT called, `RecordSchemaFingerprint` NOT called, `schema_state`
  unchanged, DB byte-identical. **This is the M3 close** — cell 14/`>=21` is the exact v5
  reproducer (complete + decoupled OFF + revoke-embedding + a pending change), now a clean halt.
- **Every no-revoke binary on a revoke-applied DB (`>=21` in cells 1, 3, 5, 7, 9, 11, 13, 15)** →
  `awaiting_deploy` at W (barrier b). Never reaches `:353`.
- **Every non-`complete` cursor not already caught by A0** (cells 5, 7, 8, 9, 11, 12 at `==20`;
  and the `aborted` variants) → `awaiting_deploy` at A1/A2 (BC-N2). DB untouched.
- **Every decoupled cell (3, 4, 7, 8, 11, 12, 15, 16)** runs `ConnectAndVerify`, which NEVER calls
  `ApplyMigrations`/`:399`; it serves only on the in-sync `complete` row (cell 16/`>=21`, cell
  15/`==20`) and otherwise halts `awaiting_deploy`. The deployer is the only writer of `schema_state`
  on this path, gated by `VerifyStoredTranscript` (M1).

**The shadow-mode drift-gate fall-through (`connection.go:384-399`) is also covered.** The only
legacy-path cells that even reach the drift gate are 1/`==20` and 13/`==20`-in-sync. Cell 1 has no
recorded fingerprint mismatch to log on a fresh DB (or it is a no-revoke binary recording its own
pre-revoke schema); cell 13/in-sync has `ExpectedFingerprint() == recorded` so `drifted == false`
and the shadow log + fall-through self-record never fires over a divergence. **No legacy
shadow-mode self-record can overwrite `schema_state` around a transcript carrying a pending
change** — the v5 §4.5 Invariant-B breach is closed.

**The legitimate fresh-DB / inert-landing cells still serve (R2, not wedged).** Cell 1/`==20`
(no-revoke inert binary, clean DB, no transcript) serves auto-apply exactly as today; the
conservative M3 halt only ever fires for a revoke-embedding binary (cells 2/6/10/14) or routes a
divergent no-revoke binary over a transcript through `deploy` (cell 13 non-in-sync). Nothing that
must serve is wedged.

---

## 4. Serving-role DDL revocation (owner bundle 0021), without lockout, without a pre-revoke serve window, without an early self-heal revoke, without a legacy-path revoke bypass

**Claim (DDL-REVOKE).** The serving role `striatumd_rw` loses its remaining object-creation DDL on
the serving path, shipped as owner bundle 0021, applied **only as the terminal step of `striatum
daemon deploy`** (C3), **gated by the universal cursor edge (BC-N2)**, **excluded from every
`owner-ddl apply` route incl. the FMA-007 self-heal (M2)**, **AND — NEW in v6 — never reachable
via the legacy serve-boot `ConnectAndMigrate` path on a `complete` cursor (M3)** — so failure
mode 1 becomes structurally impossible across the whole boot-path cell space.

### 4.1 What is revoked, precisely (carry-forward)

Bundle 0021 revokes object creation on the runtime schema: `REVOKE CREATE ON SCHEMA striatumd FROM
striatumd_rw;` (+ `REVOKE TEMPORARY` if granted). `striatumd_rw` retains ownership of (and
`ALTER`/`DROP` on) the runtime tables it already owns; new runtime objects are reconciled back to
`striatumd_rw` before 0021 commits (§3.3b). Full capability revocation (owner re-owns runtime
tables) remains a named follow-up beyond P4.

### 4.2 Typed halts + the forward-watermark rule (C2) + the universal cursor edge (BC-N2) + the M1 mismatch halts + the M3 config gate

Sibling halts of `AwaitingOwnerDDLError` (`owner.go:42,58`), reusing the non-restartable exit
(`main.go:208-227`):

- **`AwaitingDeployError`** (`awaiting_deploy`): a deploy is pending/incomplete (BC-N2), OR — NEW
  in v6 — a no-revoke binary over a `complete` transcript that is NOT in-sync (A3, M3). DB untouched.
- **`AwaitingDeployConfigError`** (`awaiting_deploy_config`): the binary ships 0021 but
  `STRIATUM_DEPLOY_DECOUPLED` is OFF — NOW for EVERY cursor state including `complete` (the hoisted
  M3 gate, §3.3a step 0). DB untouched. Remediation: "this binary ships the DDL-revoke; set
  `STRIATUM_DEPLOY_DECOUPLED=1` to serve verify-only, or run `striatum daemon deploy` to apply a
  pending change."
- **`DeployPlanBinaryMismatchError`** / **`DeployPlanDBStampMismatchError`** (M1). DB untouched.

**Forward-watermark rule (`CheckOwnerBundleWatermark` tolerate-forward branch `owner.go:151-153`),
re-anchored to 21 — carry-forward.** `applied >= 21` + this binary does NOT embed 0021 →
`AwaitingDeployError`. This is barrier (b) in §3.5. `RequiredOwnerBundleVersion` stays **20**
(`owner.go:35`); the revoke is gated by the cursor edge + `CheckDeployActivation` + the flag + its
deploy-plan-terminal placement + the `OwnerDDLApplyBundles` exclusion — NOT the watermark frontier.

### 4.3 The deploy choreography (C2 — carry-forward; M3-hardened)

Two binaries, in order:

1. **Inert-landing binary** (`Latest = Required = 20`; does NOT embed 0021). Ships the `Deployer`
   engine + `deploy` verb, migration 0044, the decoupled boot path (flag OFF), the doctor block,
   the forward-watermark rule, the universal pre-revoke cursor edge (BC-N2), `VerifyStoredTranscript`
   (M1), `OwnerDDLApplyBundles` (M2), **and the REVISED `CheckDeployActivation` with the hoisted M3
   config gate + the no-revoke complete comparison.** Boot auto-applies legacy only because no
   deploy is in flight and the binary embeds no revoke (cell 1). **Daemon up, serving.**
2. **Activation binary** (embeds 0021; `Latest = Required` still 20). On boot with the flag OFF it
   halts **`awaiting_deploy_config` for EVERY cursor state** (cells 2/6/10/14 — the M3 gate). The
   operator, with the daemon parked: sets `STRIATUM_DEPLOY_DECOUPLED=1`; runs `striatum daemon
   deploy` (substrate-ensure → materialize the immutable transcript → non-revoke owner bundles
   (≤20) → pending runtime steps (each reconciling ownership while CREATE held) → bundle 0021
   terminal → finalizer with the `VerifyStoredTranscript` gate); restarts the daemon →
   `ConnectAndVerify` sees `complete` + in-sync → serves verify-only, zero serve-path create-DDL
   (cell 16). **From activation onward the operator keeps `STRIATUM_DEPLOY_DECOUPLED=1` set** —
   the M3 gate makes that explicit and fail-closed (a revoke-embedding binary with the flag OFF
   refuses to serve rather than silently riding the legacy mutator).

   **Crash mid-deploy is safe even if the inert binary comes back up (BC-N2).** Its universal
   cursor edge halts `awaiting_deploy` (cells 5/7/9). **`owner-ddl apply` before deploy is safe
   (M2).** **And a later revoke-embedding binary that restarts with the flag OFF — at ANY cursor
   state including `complete` — halts `awaiting_deploy_config` rather than riding the legacy
   `ApplyMigrations`/`:399` path (M3, cells 2/6/10/14).**

### 4.4 How 0021 ships without lockout (R3), without a pre-revoke serve window (BC-N2), without an early self-heal revoke (M2), and without a legacy-path revoke bypass (M3)

The deployer applies all DDL over the owner/admin connection. **C3:** 0021 is the terminal plan
step (after every runtime reconcile) and excluded from the owner prefix, so no `REVOKE CREATE`
commits before a reconcile that needs CREATE inside the deploy plan. **M2:** every `owner-ddl
apply` route iterates `OwnerDDLApplyBundles()` (0021 excluded) + both loops guard
`isNonRevokeBundle` in-loop, so no `owner-ddl apply` route — incl. the FMA-007 self-heal — commits
the revoke early outside the deploy plan. **BC-N2:** every deployer-aware binary refuses on a
non-`complete` cursor before `ApplyMigrations`/`:399`. **M3 (NEW):** every revoke-embedding binary
with the flag OFF refuses (`awaiting_deploy_config`) for EVERY cursor state, and the forward-
watermark rule refuses a no-revoke binary on a revoke-applied DB — so no boot reaches the legacy
`ApplyMigrations` under a revoked CREATE, and no legacy self-record writes around
`VerifyStoredTranscript` over a transcript with a pending change. Combined, **no ordering and no
boot-path cell (§3.5) drives boot into `ApplyMigrations` under a revoked CREATE, lets any binary
serve an incomplete deploy, strands a deploy at a reconcile under a revoked CREATE, commits the
revoke outside the terminal deploy step, or self-records around the full-transcript gate.**

### 4.5 Proactive hardening — every apply path and every fingerprint path named, two universal invariants (TIGHTENED for M3)

Audited once, exhaustively, against current `main` (HEAD `df2a9e70`; `git diff --stat 2e482077 HEAD
-- go/` empty).

**A. EVERY code path that applies owner bundles (file:line) — carry-forward from v5, unchanged.**
`ApplyOwnerBundles` (`owner.go:265-303`, loads `OwnerDDLApplyBundles()` `:269`),
`applyPendingOwnerBundles` (`:309-322`, in-loop guard), `ReapplyAllOwnerBundles` (`:332-351`,
in-loop guard + filtered nil-fallback), `applyOneOwnerBundle` (`:511-541`, the only committer of
0021 via the terminal deploy step), `runDaemonOwnerDDL` (`daemon.go:90` → `ApplyOwnerBundles`),
the deployer terminal step, the test setups, and the non-apply `OwnerBundles()` consumers
(`ExpectedFingerprint`/`revokeEmbedded`/`BuildPlan`/`RuntimeOwnedTablesAlterable`).

> **UNIVERSAL INVARIANT A (executable, carry-forward).** *The DDL-revoke bundle (`>=
> DDLRevokeOwnerBundleVersion = 21`) is excluded from ALL `owner-ddl apply` routes via
> `OwnerDDLApplyBundles()` + the in-loop `isNonRevokeBundle` guard; only the terminal `deploy` step
> commits it, via `applyOneOwnerBundle`.* Enforced by **F16a** (synthetic) + **F16b** (production).

**B. EVERY code path that self-records / writes a schema fingerprint (file:line):**

| Path | Site | Disposition under M1 + **M3** |
| --- | --- | --- |
| `RecordSchemaFingerprint` (the ONLY `schema_state.fingerprint` writer) | `schema_drift.go:171-195` | unchanged; gated by callers |
| Legacy boot self-record | `connection.go:399` ("This is the only writer of schema_state") | **TIGHTENED (M3): reachable ONLY when the §3.3a activation guard returns nil on the legacy path — i.e. (cell 1) `cursorState == none`, no-revoke binary, flag OFF (NO transcript); or (cell 13) `complete`, no-revoke binary, flag OFF, AND already byte-in-sync (an idempotent rewrite). Barrier (a) (the M3 config gate) bars EVERY revoke-embedding binary; barrier (b) (the forward-watermark rule) bars EVERY no-revoke binary over a revoke-applied DB; the A3 no-revoke comparison bars a no-revoke binary with a pending change over a transcript. NEVER run on the decoupled `ConnectAndVerify` path.** |
| C1 finalizer `schema_state` UPSERT + `finalizing → complete` | P4 `Deployer.Apply` (§3.4) | gated by `VerifyStoredTranscript` finalizer step 0 (M1) — writes nothing on mismatch |
| `ExpectedFingerprint()` / `LiveFingerprint` / `CheckSchemaDrift` (pure reads) | `schema_drift.go:83-100,145-161,254-274` | read-only; no write (the A3 comparison reuses these) |

> **UNIVERSAL INVARIANT B (executable, TIGHTENED for M3).** *NO `schema_state` fingerprint /
> `complete` is EVER written around `VerifyStoredTranscript` while a deploy transcript carrying a
> pending change is present, and NO revoke-embedding binary EVER reaches the legacy `:399` writer.*
> Two writers, both bounded: (i) the deployer finalizer is gated by `VerifyStoredTranscript` step 0
> (M1); (ii) the legacy `connection.go:399` writer is reachable ONLY in §3.5 cells 1 and 13-in-sync
> — `cursorState == none` (no transcript) or a `complete` transcript ALREADY byte-in-sync (an
> idempotent rewrite of the same fingerprint) — by a no-revoke binary with the flag OFF, barred
> from any revoke-embedded (barrier a) or revoke-applied (barrier b) state and from any
> pending-change-over-a-transcript state (the A3 comparison). *Therefore a database carrying a
> `deploy_cursor`/`deploy_plan` transcript with a pending change, or any revoke-embedding binary,
> can NEVER reach the legacy `:399` writer.* Enforced by **F17** (the M3 cell), **F11** (extended),
> **F15** (extended), and **F18** (the §3.5 decision-table matrix).

These invariants + the §3.5 decision table are the executable, named requirements the cycle-1
falsifiers verify against the named code sites — so no unaudited boot-path combination can spawn
an M5.

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> `T-` = unit/integration; `G-` = game-day against a real two-role cluster. F1–F16 carry forward
> from v5 (re-confirmed against current `main`); **F11 + F15 are EXTENDED for the M3 complete-cursor
> case**, **F16 is SPLIT into F16a (synthetic) + F16b (production) for M4**, and **F17 (the M3
> refusal) + F18 (the decision-table matrix) are NEW** — these are what the cycle-1 falsifiers
> re-attack first.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1–F10, F12–F14** | Carry forward verbatim from v5 (resumability Q3-A/Q3-B, no serve-boot mutation when decoupled, §1.3 classification incl. the M1 mismatch rows, no P2/P3 regression, DDL-revoke structural, per-step receipt, no double-apply, plan≡fingerprint, the C1 finalization boundary with the M1 step-0 gate, the C3 runtime-object ownership extended for M2, per-step crash-safety, immutable plan identity with M1 already-applied verification). Re-confirmed; anchors in §9. | (as v5) |
| **F11 (Activation fail-closed — C2 + BC-N2 universal edge; EXTENDED for M3)** | For every deployer-aware binary, boot never calls `ApplyMigrations`/`RecordSchemaFingerprint` while the cursor is non-`complete`; a no-revoke binary at `applied_owner == 20` with a non-`complete` cursor halts `awaiting_deploy` DB-byte-identical; a no-cursor/`idle` no-revoke binary SERVES; **AND (g) NEW — a revoke-embedding binary with `STRIATUM_DEPLOY_DECOUPLED` OFF halts `awaiting_deploy_config` for EVERY cursor state including `complete`, DB-byte-identical, with `ApplyMigrations`/`RecordSchemaFingerprint` un-called (spies).** | `T-deploy-revoke-activation-ordering` cases (a)–(f) (carry-forward) PLUS **(g)** the complete-cursor + revoke-embedded + flag-OFF case; spies prove `applyOne`/`RecordSchemaFingerprint` never entered. `G-old-binary-refuse`. |
| **F15 (Full-transcript verification on resume AND before the finalizer — M1; EXTENDED)** | Carry-forward from v5 (the A45/B45 byte case, the owner-step case, the DB-stamp case, the finalizer arm). **EXTENDED note:** the finalizer's `VerifyStoredTranscript` step-0 gate AND the §3.3a M3 config gate together mean NO writer self-records around the full-transcript check — the deployer finalizer (M1) and the legacy `:399` writer (M3) are both bounded. | `T-deploy-resume-already-applied-byte-mismatch-refuses` (as v5) + an assertion that the §3.3a guard precedes `connection.go:353` in BOTH `ConnectAndMigrate` and `ConnectAndVerify` (so neither path self-records around the gate). `G-wrong-binary-resume`. |
| **F16a (Revoke excluded from EVERY owner-ddl apply route — M2, SYNTHETIC phase; rollout step 2)** | Against a SYNTHETIC bundle list / test hook (a hand-built `[]OwnerBundle` with a synthetic `{Version: 21}` + an injectable FS seam): `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`; `applyPendingOwnerBundles` AND the FMA-007 `ReapplyAllOwnerBundles` skip a hand-passed synthetic 0021; `ReapplyAllOwnerBundles(nil, …)` resolves to the filtered loader. Does NOT assert production `OwnerBundles()` contains 0021 (it does not yet). | `TestOwnerDDLApplyExcludesSyntheticRevokeBundle` (unit, no DB) + a build-time grep test (no production `applyOneOwnerBundle` caller outside the deployer terminal-step path receives a bundle `>= 21`). Builds green BEFORE 0021 is authored. |
| **F16b (Revoke excluded — M2, PRODUCTION phase incl. forced self-heal; rollout step 7)** | After 0021 is authored: production `OwnerBundles()` DOES contain 0021; `ExpectedFingerprint()` includes its bytes; `revokeEmbedded` derives from the full loader / 0021 file presence in `ownerBundleFS`; production `OwnerDDLApplyBundles()` excludes it; the FORCED FMA-007 cross-bundle self-heal (inject a real `42P01` so `isCrossBundleDependencyError` `owner.go:367-374` fires) never commits the revoke; `owner_bundle_meta` never records 21; CREATE stays held. | `TestOwnerDDLApplyExcludesProductionRevokeBundle` (unit) + `T-deploy-revoke-excluded-from-reapply-self-heal` (two-role pgtest; **must drive `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`, not merely the pending loop**, asserted by a spy/log on the self-heal entry). |
| **F17 (M3 — the complete-cursor legacy mutate+self-record refusal — NEW)** | A revoke-embedding binary with a `complete` deploy cursor, `STRIATUM_DEPLOY_DECOUPLED` OFF, and a pending runtime migration (or a changed expected fingerprint) refuses to serve `awaiting_deploy_config`, calls NEITHER `ApplyMigrations` NOR `RecordSchemaFingerprint`, leaves `schema_state` unchanged and the DB byte-identical — on BOTH `ConnectAndMigrate` and `ConnectAndVerify`. | **`T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`**: materialize a `complete` cursor + a `deploy_plan` transcript over a two-role DB; build the running binary as revoke-embedding (0021 in `ownerBundleFS`) with a pending runtime migration; boot with the flag OFF; assert `awaiting_deploy_config`, `ApplyMigrations` spy un-called, `RecordSchemaFingerprint` spy un-called, `schema_state` row byte-identical (snapshot before/after), and `has_schema_privilege('striatumd_rw','striatumd','CREATE')` unchanged. **Symmetric shadow-mode arm:** same setup with `STRIATUM_SCHEMA_DRIFT_REFUSE` unset (shadow); assert the shadow drift-gate fall-through `connection.go:384-399` is never reached (no `:399` self-record). `G-complete-cursor-flag-off-refuse`: on a real post-deploy cluster (`complete`, 0021 applied), boot a later revoke-embedding binary with the flag OFF; assert it refuses `awaiting_deploy_config` and the DB is byte-identical. |
| **F18 (Boot-path decision table — proactive completeness — NEW)** | EVERY `cursorState ∈ {none, in_progress, step_committed, finalizing, complete, aborted} × decoupledEnabled ∈ {on, off} × revokeEmbedded ∈ {yes, no} × applied_owner ∈ {<20, ==20, >=21}` cell produces the EXACT §3.5 outcome, and in NO cell does the legacy `:399` writer fire over a transcript carrying a pending change or for a revoke-embedding binary. | **`T-deploy-bootpath-decision-table`**: a parametrized matrix over all cells; for each, construct the DB + binary state and assert the §3.5 outcome (the typed halt OR serve-legacy/serve-verify), and assert `ApplyMigrations`/`RecordSchemaFingerprint` spies are called ONLY in cells 1/`==20` and 13/`==20`-in-sync (and in 13 only as an idempotent no-op rewrite). Asserts the guard precedes `connection.go:353` on both boot paths. |

---

## 6. Shadow-first rollout (the P4 build order) — re-anchored + M3 + M4

1. **Migration 0044 `deploy_cursor` + `deploy_plan`** (additive runtime tables; `state` CHECK
   includes `finalizing`; runtime-owned).
2. **`go/pkg/db/owner.go` M2 surface (lands first, inert):** `DDLRevokeOwnerBundleVersion = 21`,
   `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, the in-loop guards, the nil-fallback split, **and
   the F16a SYNTHETIC-phase test `TestOwnerDDLApplyExcludesSyntheticRevokeBundle` + the build-time
   grep test (M4 — proves the exclusion contract against a synthetic list BEFORE 0021 exists, so it
   builds green; does NOT assert production `OwnerBundles()` contains 0021 yet).** Inert until 0021
   is authored (step 7).
3. **`go/pkg/db/deploy.go`**: `DeployPlan`, `BuildPlan` (0021-terminal, FULL `OwnerBundles()`),
   `LoadStoredPlan`, `VerifyStoredTranscript` (M1) + the typed mismatch halts, `Deployer.Apply`
   (Q3-A/Q3-B engine + the `finalizing` finalizer with `VerifyStoredTranscript` step 0), the
   substrate-ensure preamble, `applyRuntimeStep` (C3 reconcile), the receipt writer. Pure-core +
   DB-integration tests (F1, F2, F4, F8, F9, F10, F12, F13, F14, F15) proven BEFORE any boot path
   changes.
4. **`runDaemonDeploy`** verb + matrix/authority-guardrail row + `--dry-run`/`--abort` + the
   0021-activation preflight. F3/F5 wiring.
5. **`CheckDeployActivation`** with the REVISED M3 predicate — **the hoisted `revokeEmbedded &&
   !decoupledEnabled → awaiting_deploy_config` config gate (every cursor state, step 0) + the
   no-revoke `complete` pre-`ApplyMigrations` comparison (step 3)** — on top of the universal
   pre-revoke cursor edge (BC-N2) + the typed `awaiting_deploy`/`awaiting_deploy_config` +
   `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` halts + the forward-watermark rule
   (21) in `CheckOwnerBundleWatermark` + the `main.go` `errors.As` arms. Decoupled boot path
   (`ConnectAndVerify`) behind `STRIATUM_DEPLOY_DECOUPLED` (default OFF). **Lands inert for a
   no-revoke binary** (F11 incl. (g)-with-synthetic-revoke, F3, F5, **F17, F18**).
6. **`doctor schema_deploy_unrecorded`** block — per-step tightened, transcript-enumerated, + the
   M1 stamp/byte WARN (F7, F4, F15 doctor arm).
7. **Owner bundle 0021** (DDL revoke) — authored, deploy-plan-terminal, excluded from every
   `owner-ddl apply` route (`LatestOwnerBundleVersion` stays 20). **The F16b PRODUCTION-phase test
   `TestOwnerDDLApplyExcludesProductionRevokeBundle` + the forced-self-heal pgtest land here (M4)**,
   tested in two-role pgtest (F6, F12, F16b); activation is the operator choreography (§4.3).

Each phase additive and reversible. Self-record before enforce; detection before
mutation-relocation; the verb + finalizer + full-transcript verifier + ownership reconcile +
immutable-transcript resume + the non-revoke filter + **the M3 activation guard** all proven before
the boot path leans on them. The M2 filter contract lands green at step 2 against a synthetic list
(F16a); the production embed/listing split is proven once 0021 exists at step 7 (F16b).

---

## 7. Boundary & explicit deferrals (carry-forward)

- **In P4:** the deployer verb; the deploy plan + `deploy_cursor` (incl. `finalizing`) + the
  immutable `deploy_plan` transcript (BC-N1) + per-step + terminal receipt; the idempotent
  finalizer (C1) with the `VerifyStoredTranscript` pre-finalizer gate (M1); the C3 ownership
  reconcile with 0021 sequenced last; the single non-revoke `owner-ddl apply` filter + embed/listing
  split (M2); lifting `ApplyMigrations` out of serve-boot (flagged); the activation interlock + the
  universal pre-revoke cursor edge (BC-N2) + **the M3 complete-cursor config gate + no-revoke
  comparison** + forward-watermark rule; serving-role create-DDL revocation (bundle 0021,
  deploy-terminal).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`, `rehearsal_receipt.v1`,
  fidelity tiering, full-data clone, expand/contract reshape, lock-budget guardrail. Full capability
  revocation (owner re-owns runtime tables) is a named follow-up beyond P4 (§4.1).
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the single writer, no
  hosted services / replicas / external persistence.

---

## 8. Open tensions handed to the falsifiers (attack these hardest)

- **M3 / F17 / F18 — the hoisted config gate vs. the no-revoke comparison.** Is the
  `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt truly the FIRST predicate, so
  the `complete` cursor cannot short-circuit it (the v5 hole)? Verify it precedes
  `connection.go:353` on BOTH `ConnectAndMigrate` and `ConnectAndVerify`, not only one. Is the
  no-revoke `complete` comparison (step 3) genuinely a PURE READ (`ExpectedFingerprint()` +
  `LiveFingerprint` + `plan_hash` compare) that cannot mutate or self-record — and does its
  "in-sync ⇒ serve-legacy" arm ever let `ApplyMigrations` do anything but a no-op? Is there any
  cell in §3.5 where the legacy `:399` writer fires over a transcript carrying a pending change, or
  for a revoke-embedding binary?
- **M3 — the conservative halt vs. wedging.** Does the hoisted gate wedge any cell that MUST serve?
  (It should fire ONLY for a revoke-embedding binary with the flag OFF — is there a legitimate
  scenario where such a binary should serve on the legacy path? The design says no: once a binary
  embeds the revoke, decoupling is mandatory.) Does cell 1 (no-revoke inert, clean DB) still serve?
- **M3 — `cursorState == none` + `applied_owner >= 21` (cell 4).** Is treating a cursor-absent
  but revoke-applied DB as a conservative `awaiting_deploy` correct, or is there a legitimate boot
  there?
- **M4 / F16a — the synthetic seam.** Does the synthetic-list / injectable-FS hook genuinely
  exercise `OwnerDDLApplyBundles`/`isNonRevokeBundle`/both apply loops without depending on a real
  0021 file? Does F16b's forced self-heal truly reach `ReapplyAllOwnerBundles` via
  `isCrossBundleDependencyError` (a real `42P01`/`42703`/`42883`/`42704`), not just the pending loop?
- **M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3 carry-forward (re-attack the v5 tensions §8).** The
  verification-set oracle questions, the two-barrier owner-ddl questions, materialize-before-step-0
  concurrency, the universal-edge-vs-fresh-DB question, the catalog-diff object-kind question, and
  the residual-capability question all carry forward unchanged — re-confirm none regressed.

---

## 9. Anchor table (re-verified against current `main`, HEAD `df2a9e70`)

> `git diff --stat 2e482077 HEAD -- go/` is **empty** (the only commits since the v5 anchor are
> docs/workflow), so every v5 source anchor holds unchanged. The M3 boot order and the owner.go
> apply-route/watermark anchors were re-confirmed line-by-line this cycle (§9 spot-checks below).

| Target | Anchor (re-confirmed `df2a9e70`) |
| --- | --- |
| **serve-boot mutation site + the M3 boot order (lift this; the M3 gate precedes it)** | `connection.go:332` `ConnectAndMigrate`; watermark `:349`; **`ApplyMigrations` `:353`**; drift gate `:376-383` (refuse-mode halt); **shadow log + fall-through `:384-393`**; **`RecordSchemaFingerprint` `:399` ("the only writer of schema_state")** — re-read this cycle; the post-`ApplyMigrations` ordering is exactly why a pre-`:353` guard returning nil lets the legacy mutator + self-record run, and why the M3 fix halts BEFORE `:353`. |
| boot fork / `--migrate` / typed-halt mapping | `authority_bootstrap.go:181,193`; `main.go:76`; typed halts `:208-227` (add `awaiting_deploy`/`awaiting_deploy_config`/`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` arms) |
| deployer command site | `daemon.go:67-81` dispatch (add `deploy`); `runDaemonOwnerDDL` `:90` (admin DSN `:115`); `runDaemonMigrate` `:167`; `EnvDaemonAdminDBURL` `:24` |
| runtime apply engine to reuse/wrap (C3) + M1 oracles | `migrations.go:121` `ApplyMigrations`; `applyOne` `:304-355`; `verifyRecordedHashTx` `:274` (reads `schema_migrations.sha256` `:275` — M1 runtime DB-stamp oracle); `MigrationSHASet` `:197` (M1 runtime byte oracle); `//go:embed sql/*.sql` `:21`; lock `:18` |
| owner-bundle engine / watermark + forward-tolerance (C2) / 0021 exclusion (M2/C3) — re-confirmed `:23,:35,:124,:191,:197,:265,:309,:332,:367,:511` this cycle | `owner.go:265-303` `ApplyOwnerBundles` (slice `:269` → `OwnerDDLApplyBundles()`); `:309-322` `applyPendingOwnerBundles` (`<= current` `:312`, in-loop guard); `:332-351` `ReapplyAllOwnerBundles` (nil-fallback `:336-342`, in-loop guard `:344-349`); `:367-374` `isCrossBundleDependencyError`; `:511-541` `applyOneOwnerBundle` (`owner_bundle_meta.sha256` stamp `:529` — M1 owner DB-stamp oracle); `CheckOwnerBundleWatermark` `:124-154` (tolerate-forward `:151-153` — the forward-watermark rule / barrier b); `OwnerBundles()` `:197-224` (FULL loader); `OwnerBundle.SHA256()` `:191` (M1 owner byte oracle); `LatestOwnerBundleVersion = 20` `:23` (**stays 20**); `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` `:35` (= 20, **stays 20**); `revokeEmbedded` = 0021 in `ownerBundleFS` (`//go:embed` `:156`) |
| pure read consuming the FULL owner-bundle loader (must keep seeing 0021) | `owner_runtime_ownership.go:51-52` `RuntimeOwnedTablesAlterable()` → `OwnerBundles()` |
| current owner-bundle frontier (NOT the revoke) | `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql` (#581 / RFC 0142 Layer 2) — the highest non-revoke bundle; the DDL-revoke is the NEW 0021 |
| C3 prerequisite the revoke-last fix relies on | `0018_runtime_table_ownership_transfer.sql:64-66` (prereq), `:98` (`GRANT CREATE … TO striatumd_rw` before the `:102` transfer loop); `0019_supervisor_pointer_runtime_ownership.sql:53-80` — 0018 in the non-revoke prefix, CREATE held when runtime reconciles run |
| fingerprint contract (M1 mechanism + the M3 A3 pure-read comparison) | `schema_drift.go:83-100` `ExpectedFingerprint` (hashes EMBEDDED FILE BYTES — the A3 binary oracle, pure); `:105-137` `composeFingerprint` (reused for `plan_hash`); `:145-161` `LiveFingerprint` (reads the self-recorded singleton — NO recompute; the A3 recorded oracle, pure); `:171-195` `RecordSchemaFingerprint` (the gated write); `:254-274` `CheckSchemaDrift`; `EnvSchemaDriftRefuse` `:28` |
| `deploy_cursor` + `deploy_plan` table model | `0043_schema_state.sql:39-44` (singleton CHECK), `:46-52` (`striatumd_rw` GRANT `DO` block) — model for both new 0044 tables |
| per-step / terminal receipt SD fn | `0001_authority_phase0.sql:152` (`append_audit_row`), `:237` (runtime direct INSERT revoked), `:238` (`GRANT EXECUTE … TO striatumd_rw`) |
| doctor block model | `go/pkg/reads/doctor_schema_drift.go:26-77` (`schemaDriftDoctorBlock`; `ExpectedFingerprint` call `:32`) |
| FMA-007 self-heal test to extend (M2 / F16b) | `owner_pg_test.go:219` `TestReapplyAllOwnerBundlesIsOrderedAndIdempotent` (calls `ReapplyAllOwnerBundles(ctx, runner, nil, "test")` `:229`) |
| two-role fixture for F12 / F16b / F17 | `go/pkg/pgtest/two_role.go:130` `ApplyOwnerBundles` (non-superuser two-role bootstrap) |

<sub>Holder artifact (v6 revision) for the RFC 0142 P4 falsification-gate design run. Resolves
cycle-1 (design-v5) findings **M3** (the hoisted `revokeEmbedded && !decoupledEnabled →
awaiting_deploy_config` config gate firing for EVERY cursor state including `complete`, plus the
no-revoke `complete`-cursor pre-`ApplyMigrations` PURE-READ fingerprint comparison, so the legacy
`connection.go:399` writer is structurally unreachable over any transcript carrying a pending change
and for any revoke-embedding binary; Universal Invariant B tightened; the proactive-completeness
boot-path decision table §3.5 proving Invariant B in all 48 cells; `T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`
/ F17 + F11(g) + the decision-table matrix F18) and **M4** (F16 split phase-aware into F16a
synthetic-list pre-0021 + F16b production post-0021, the forced FMA-007 self-heal in F16b reaching
`ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`), and carries **M1 + M2 + BC-N1 + BC-N2
+ C1 + C2 + C3 forward verbatim and unregressed** (re-verified against current `main` HEAD
`df2a9e70`; `git diff --stat 2e482077 HEAD -- go/` empty). The cycle-1 falsifiers (decoupling-
boundary/self-record lens; owner-ddl/test-staging lens) re-attack §3.3a, §3.5, §4.5 Invariant B,
F17/F18, the F16a/F16b split, and the tensions in §8; the adjudicator's collaboration ledger — not
falsifier completion — decides whether the gate clears.</sub>

# HOLDER (revised, v7) — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: holder-author-001

> **This is the SEVENTH REVISION (v7).** Design-v1 returned `needs_revision` with C1/C2/C3.
> v2 resolved C1 + C2. v3 resolved C3 (ownership transfer, revoke-last) and closed N1, then
> returned `needs_revision` on BC-N1 + BC-N2. v4 resolved BOTH BC-N1 and BC-N2 and returned
> `needs_revision` on M1 + M2. v5 resolved BOTH M1 and M2 and returned `needs_revision` on M3 +
> M4. v6 (`rfc-0142-p4-design-v6`) **resolved BOTH M3 and M4** — both v6 falsifiers AND the v6
> adjudicator explicitly concede each — and carried M1/M2/BC-N1/BC-N2/C1/C2/C3 forward intact,
> BUT returned `needs_revision` again (the gate's single allowed cycle) on **one new finding,
> source-verified against current `main` and landed INDEPENDENTLY by BOTH v6 falsifiers**:
>
> - **M5 (LOAD-BEARING)** — the §3.5 / F18 proactive-completeness boot-path decision table
>   **collapses the owner-watermark dimension**. v6 §3.5 mapped `applied_owner < 20 →
>   awaiting_owner_ddl` UNIFORMLY (stated the `<20` column "ALWAYS halts at W", v6
>   `HOLDER.md:443-459,461-478`) and the prose mislabeled cell `==20` as the "fresh-DB bring-up"
>   cell (v6 `HOLDER.md:515-518`). But current source `CheckOwnerBundleWatermark` **SERVES
>   (returns nil) for `applied_owner == 0`** — the fresh / single-role / no-authority bootstrap
>   case — BEFORE the shortfall check (`go/pkg/db/owner.go:145` `if applied == 0 { return nil }`
>   precedes the `if applied < RequiredOwnerBundleVersion` halt at `:148-150`), and halts
>   `awaiting_owner_ddl` ONLY for `1 <= applied < 20`. So the v6 §3.5/F18 table either (1)
>   **WEDGES** a legitimate fresh no-authority boot the SEED requires to "still serve and NOT be
>   wedged", or (2) makes the **EXECUTABLE F18 oracle FALSE** for the `applied_owner == 0` cell.
>
> This spec **starts from the v6 `HOLDER.md`** (a required context doc) and is a **proper
> revision**: it **resolves M5** per the prescribed fix in the v6 collaboration ledger §4 ("What
> the revision must fix") and `SEED.md` by **SPLITTING the `applied_owner` dimension of §3.5 and
> F18 into `{0/no authority, 1..19 authority shortfall, ==20, >=21}`**, specifies the
> `applied_owner == 0` no-transcript/no-revoke/flag-off cell as **serve-legacy / fresh bootstrap**
> (matching `CheckOwnerBundleWatermark`'s `applied == 0` exception), retains `awaiting_owner_ddl`
> for `1 <= applied_owner < 20`, makes **F18 assert BOTH cells explicitly**, stops mislabeling cell
> `==20` the "fresh-DB" cell, **keeps the proactive-completeness boot-path decision table** (now
> the §3.5 table over **64 cells** = 16 `(cursorState × decoupledEnabled × revokeEmbedded)` rows ×
> **4** `applied_owner` sub-columns), and **carries M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3
> forward unregressed**. §0.1 below is the auditable resolution map; the cycle-1 falsifiers should
> verify M5 there, then re-attack the revised §3.5 split table, the F18 dual-cell matrix, the §4.5
> Invariant-B proof over the new cells, and re-confirm each carry-forward.
>
> It is a falsifiable implementation spec for RFC 0142 **P4 only** — the one-shot deployer that
> lifts schema mutation out of serve-boot — that `rfc-0142-p4-build` can execute contract-first
> (TDD). RFC 0142 is accepted (D258); this does NOT re-open the five-layer design. Primary source:
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (Phasing P4 row, Open
> Questions 3 + 4, "the hard correctness core of P4"). **Every anchor below is re-verified against
> current `main` (HEAD `9ceeb288`).** `git diff --stat 2e482077 HEAD -- go/pkg/db/` is **empty** —
> the only `go/` commits since the v6 anchor touched `go/pkg/mutations/` + `go/pkg/reads/`, NOT any
> P4-relevant `db/` file — so **every v6 source anchor in `go/pkg/db/` holds unchanged**, and the
> M5 owner-watermark sites were re-confirmed line-by-line this cycle (§9): `owner.go:145`
> (`if applied == 0 { return nil }`), the comment block `:116-123` + `:140-143`, `:148-150`
> (shortfall), `:151-153` (tolerate-forward / barrier b), `OwnerBundleVersion` absent-meta branch
> `:234-235`, `owner_pg_test.go:19` (fresh DB version 0), `owner.go:23` `LatestOwnerBundleVersion =
> 20`, `:35` `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`, and the boot order
> `connection.go:332/349/353/376-383/384-393/399`.

## 0. Root reframe held — and its OTHER edge (M5)

Schema mutation must stop being an **implicit side effect of the serving process's restart** and
become an **explicit, ordered, resumable, provenance-tracked operation owned by a dedicated
deployer**. Then the serving daemon holds **zero create-DDL on the serving path**, "restart
force-commits a half-applied deploy" becomes structurally impossible, and a bad migration can
never wedge the single writer on boot. **M3 (v6) closed one edge of this reframe** — the legacy
`ApplyMigrations`-on-serve-boot path is now unreachable for a revoke-embedding binary or over a
transcript carrying a pending change.

**M5 is the OTHER edge of the same reframe.** The fresh / single-role / no-authority bootstrap
(`applied_owner == 0`, no deploy transcript, no revoke embedded) is the **one legitimate
serve-legacy cell** — the first boot of a fresh database, which must bring its schema up and
serve. The reframe must NOT wedge that healthy first boot. v6's §3.5 over-halted it by collapsing
the `applied_owner == 0` serve case into the `1..19` authority-shortfall halt. The v7 revision
splits the owner-watermark dimension so the decision table matches the live bootstrap contract:
the conservative halt is for a revoke-embedding binary (M3) and for a genuine authority shortfall
(`1..19`), **never** for a fresh `applied_owner == 0` DB.

The v7 revision changes EXACTLY ONE structural thing on top of the v6 spec — the `applied_owner`
dimension of the §3.5/F18 decision table — and re-anchors line references. It does **not** re-open
any resolved mechanism:

- **M5 — the §3.5/F18 `applied_owner` dimension is SPLIT into `{0/no authority, 1..19 authority
  shortfall, ==20, >=21}`.** The `applied_owner == 0` no-transcript/no-revoke/flag-off cell
  **serves-legacy** (W = `CheckOwnerBundleWatermark` returns nil for `applied == 0`, so boot
  proceeds to A and the legacy `:399` self-record runs — NO transcript exists, so Universal
  Invariant B is not in scope), exactly matching `owner.go:145`. `1 <= applied_owner < 20` retains
  `awaiting_owner_ddl` (DB untouched, W halts before A). F18 asserts BOTH cells. The §3.5 table is
  rebuilt over all 64 cells, Invariant B re-proven in every one, and the genuine fresh-DB cell is
  proven to STILL SERVE.

**Preserved asymmetry (the load-bearing nuance):** the M3 halt is conservative ON PURPOSE for a
revoke-embedding binary (decoupling becomes mandatory once the binary embeds 0021) — so the
`applied_owner == 0` SERVE cell is for a **no-revoke** binary (cell 1) ONLY; a revoke-embedding
binary on a fresh DB with the flag OFF still halts `awaiting_deploy_config` at A0 (cell 2). The M5
fix ONLY re-buckets the `applied_owner` dimension at W; it does NOT weaken the M3 config gate,
regress the BC-N2 `applied_owner == 20` edge, advance `RequiredOwnerBundleVersion` (stays 20), or
alter the watermark.

---

## 0.1 Addressing the design-v6 findings (the gating requirement)

The single binding v6 finding (M5) is resolved by a concrete dimension split bound to named code
sites — not a claim. M3, M4, M1, M2, BC-N1, BC-N2, C1, C2, and C3 are carried forward **verbatim**
from the v6 HOLDER and re-confirmed unregressed. The "where" column names the section that
specifies the fix and the named test that would refute it.

| Finding | v6 status | v7 disposition | Where / refuting test |
| --- | --- | --- | --- |
| **M5** the §3.5/F18 `applied_owner < 20 → awaiting_owner_ddl` bucket collapses the fresh-DB `applied_owner == 0` serve cell into the `1..19` shortfall halt — wedges a legitimate fresh boot OR makes the F18 oracle false | **OPEN** (falsifier_1 decoupling-boundary lens + falsifier_2 owner-ddl/test-staging lens, independent convergence; adjudicator source-verified) | **RESOLVED: the §3.5 and F18 `applied_owner` dimension is SPLIT into four disjoint buckets `{0/no authority, 1..19 authority shortfall, ==20, >=21}` (covering all of ℕ).** W = `CheckOwnerBundleWatermark` (`owner.go:124-154`) is respecified per the live contract: `applied_owner == 0` → **return nil, proceed to A** (the fresh/single-role/no-authority bootstrap exception, `owner.go:145`); `1 <= applied_owner < 20` → **`awaiting_owner_ddl`**, DB untouched (the genuine shortfall, `owner.go:148-150`); `applied_owner == 20` → nil, proceed to A; `applied_owner >= 21` → revoke-embedding tolerates forward (proceed to A), no-revoke → `awaiting_deploy` (barrier b, the C2 forward-watermark rule re-anchored to 21). Because A = `CheckDeployActivation` does NOT read `applied_owner`, the `0` column and the `==20` column have **identical A-gate outcomes** — so the genuine fresh-DB serve cell (`cursorState=none`, off, no-revoke, `applied_owner=0`, row 1) SERVES exactly as the source serves: `ApplyMigrations` + the legacy `:399` self-record run, NO transcript exists, so Invariant B is not in scope. The `1..19` column ALWAYS halts `awaiting_owner_ddl` at W. Cell `==20`/row 1 is relabeled the **inert-landing re-boot** cell (an already-owner-bundled DB), NOT "fresh-DB bring-up" (the genuine fresh no-authority DB is `applied_owner == 0`). The M5 fix re-buckets ONLY the `applied_owner` dimension; the M3 config gate, the BC-N2 `applied_owner == 20` edge, `RequiredOwnerBundleVersion = 20`, and the watermark are unchanged. | §3.5 (the SPLIT 64-cell decision table + the W-gate respecification), §4.5 (Invariant B re-proven over the new `:399`-reaching cells), §1.3 (the fresh-DB row), §3.3a (A is `applied_owner`-independent) / **`T-deploy-bootpath-decision-table` (F18, asserts BOTH the `applied_owner == 0` serve cell AND the `1..19` halt cell) + `T-deploy-fresh-db-bootstrap-serves` (F18a, new) + F11(e)/(f)** |
| **M3** the `complete`-cursor window lets a revoke-embedding binary + flag OFF ride the legacy `ConnectAndMigrate` mutate+self-record path around `VerifyStoredTranscript` | **RESOLVED** (v6; both falsifiers + adjudicator concede) | **CARRIED FORWARD VERBATIM, unregressed.** (1) The `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt is HOISTED to step 0 of `CheckDeployActivation`, BEFORE any cursor-state branch, so it fires for EVERY cursor state including `complete` (barrier a). (2) The no-revoke `complete` residual runs a pre-`ApplyMigrations`, DB-untouched PURE-READ comparison (`ExpectedFingerprint() == LiveFingerprint(recorded)` AND `cursor.plan_hash == expected`) and serves only when in-sync. (3) Universal Invariant B is TIGHTENED so the legacy `:399` writer is reachable only by a no-revoke flag-OFF binary over NO transcript or a `complete`-and-in-sync transcript. **The M5 split does NOT weaken this: the M3 gate is at A (step 0), `applied_owner`-independent, and still fires for cells 2/6/10/14 in EVERY `applied_owner` column — incl. the new `0` column (a revoke-embedding binary on a fresh DB with the flag OFF still halts).** | §3.3a (predicate), §3.5 (cells 2/6/10/14 in all four columns), §4.5 (Invariant B) / **F17 + F11(g) + F18** |
| **M4** F16's `TestOwnerDDLApplyExcludesRevokeBundle` asserts production `OwnerBundles()` contains 0021 in step 2 before 0021 is authored in step 7 | **RESOLVED** (v6; both falsifiers concede) | **CARRIED FORWARD VERBATIM, unregressed.** F16a (`TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, step 2) drives the exclusion filter through a synthetic bundle list / injectable-FS hook WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (`TestOwnerDDLApplyExcludesProductionRevokeBundle`, step 7) asserts the production embed/listing split + houses the forced FMA-007 self-heal pgtest reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError` (`owner.go:367-374`). M5 does not touch F16. | §3.2a, §6 / **F16a + F16b + extended F12/`G-revoke-last`** |
| **M1** full stored-transcript byte + already-applied DB-stamp verification on resume AND as finalizer step 0 | **RESOLVED/INTACT** (v5/v6) | **CARRIED FORWARD VERBATIM, unregressed.** `VerifyStoredTranscript(plan_hash)` (§3.4a) checks the ENTIRE stored transcript `sha256` vs the binary's embedded bytes ⇒ `deploy_plan_binary_mismatch`; already-applied entries also verify the DB stamps ⇒ `deploy_plan_db_stamp_mismatch`; the SAME verifier runs on every resume AND as finalizer step 0. M5 concerns the `applied_owner` dimension at W; it does NOT touch the deployer verifier. | §3.4a, §1.2, §1.3, §3.4 / **F15 + F14** |
| **M2** the single non-revoke filter `OwnerDDLApplyBundles()` + `isNonRevokeBundle` excludes 0021 from EVERY `owner-ddl apply` route incl. the FMA-007 self-heal | **RESOLVED/INTACT** (v5/v6) | **CARRIED FORWARD VERBATIM, unregressed.** The filter, the in-loop guards on `applyPendingOwnerBundles` + `ReapplyAllOwnerBundles`, the nil-fallback split, and the embed/listing split (`OwnerBundles()` keeps 0021 for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`/`RuntimeOwnedTablesAlterable`; `OwnerDDLApplyBundles()` never iterates it). M5 does not touch the filter. | §3.2a, §4.4, §4.5 (Invariant A) / **F16a/F16b + extended F12 / `G-revoke-last`** |
| **BC-N1** immutable `deploy_plan` transcript materialized before step 0; resume off the stored transcript | **RESOLVED/INTACT** (v4/v5/v6) | **CARRIED FORWARD VERBATIM, unregressed.** Migration 0044 `deploy_plan` keyed by `plan_hash`; materialize in the same tx as `in_progress(0)` before step 0; resume never recomputes `BuildPlan`; §1.3 + transcript-enumerated doctor + F14. | §1.2, §1.3, §3.2 / **F14** |
| **BC-N2** universal `revokeEmbedded`-independent `CheckDeployActivation` edge halting non-`complete` cursors at `applied_owner == 20` | **RESOLVED/INTACT** (v4/v5/v6) | **CARRIED FORWARD VERBATIM, unregressed.** The universal incomplete-deploy edge halts `awaiting_deploy` DB-untouched on every non-`complete` cursor. **M5 concerns the ORTHOGONAL owner-watermark `applied_owner` dimension at W (`CheckOwnerBundleWatermark`), NOT the BC-N2 `deploy_cursor` edge at A — the resolved `applied_owner == 20` BC-N2 edge is itself not regressed (the split adds the `0` and `1..19` columns and leaves the `==20` column's BC-N2 cells 5/7/9/11 unchanged).** | §3.3a step 1, §4.2 / **F11(e)/(f) + extended `G-old-binary-refuse`** |
| **C1** the `finalizing` state + idempotent finalizer + §1.3 row + the M1 pre-finalizer gate | **INTACT** (v2–v6) | **CARRIED FORWARD VERBATIM, unregressed.** M5 touches neither the finalizer nor the legacy writer's enforcement — it re-buckets the W gate that runs BEFORE both. | §1.2, §1.3, §3.4 / **F10 + F15 pre-finalizer arm** |
| **C2** `CheckDeployActivation` before `ApplyMigrations`, typed halts, forward-watermark at `applied >= 21`, `RequiredOwnerBundleVersion` KEPT at 20 | **INTACT** (v2–v6) | **CARRIED FORWARD VERBATIM, unregressed.** The M5 split keeps the forward-watermark rule firing at `applied_owner >= 21` (barrier b, the `>=21` column) and `RequiredOwnerBundleVersion = 20` (`owner.go:35`) — it does NOT advance `Required`, alter the watermark, or change the `applied >= 21` forward rule; it only adds the `0` and `1..19` columns below the frontier. | §3.3a, §4.2, §4.3, §3.5 (the `>=21` column unchanged) / **F11** |
| **C3** the DDL-revoke bundle 0021 special-cased + excluded from `owner-ddl apply` + applied terminal | **INTACT** (v3–v6) | **CARRIED FORWARD VERBATIM.** 0021 special-cased out of the owner prefix, applied terminal; `DDLRevokeOwnerBundleVersion = 21`; `LatestOwnerBundleVersion`/`Required` stay 20. M5 does not touch the revoke mechanism. | §3.2, §3.2a, §3.3b, §4.4 / **F12 + `G-revoke-last`** |

**Carried forward unchanged (v1–v6 ledgers — do not re-litigate):** Q4 (plain verb + three
seams, §2); the per-step Q3-A/Q3-B mid-step resumability **body** + step taxonomy (§1.1); the
shadow-first decoupling shape (default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify`
preserving the P2 watermark interlock + P3 drift gate, the embed-FS-derived plan, the hash-chained
receipt, `doctor schema_deploy_unrecorded`); the M1 `VerifyStoredTranscript`; the M2 non-revoke
filter; the M3 activation predicate; the M4 F16 split. **The v7 revision touches only the
`applied_owner` dimension of the §3.5 decision table (M5), the matching F18 assertions, the §4.5
cell references, and the re-anchored line numbers.**

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
and the already-applied DB stamps match the transcript; any mismatch halts the deploy DB-untouched
and NO fingerprint is written (M1); (f) NO boot reaches the legacy
`ApplyMigrations`/`RecordSchemaFingerprint` mutate+self-record path while a deploy transcript with
a pending change is present, and NO revoke-embedding binary EVER reaches that legacy path — the
activation guard is the sole authority for "may this boot auto-apply/self-record on serve-boot",
evaluated before `ApplyMigrations` in both boot paths (M3); **and (g) NEW emphasis in v7 (M5) — the
owner-watermark gate W is fail-closed for a genuine authority shortfall (`1 <= applied_owner < 20`
→ `awaiting_owner_ddl`) AND fail-OPEN for the legitimate fresh / single-role / no-authority
bootstrap (`applied_owner == 0` → serve, the legacy `:399` self-record legitimate because NO
transcript exists). The decision is provably correct in EVERY `cursorState × decoupledEnabled ×
revokeEmbedded × applied_owner ∈ {0, 1..19, ==20, >=21}` cell (§3.5).**

- **Q3-A / Q3-B** (carry-forward body, unchanged from v4/v5/v6; receipt keyed off the stored
  transcript). See §1.1 and §3.3b.
- **Q3-V — full-transcript verification (M1, carry-forward).** `VerifyStoredTranscript(plan_hash)`
  (§3.4a) on every resume and as finalizer step 0.
- **Q3-W — boot-path activation completeness (M3, carry-forward) + the M5 owner-watermark split.**
  On EVERY boot, the two guard sites run in order — W = `CheckOwnerBundleWatermark` (the
  owner-watermark interlock, the M5 dimension) then A = `CheckDeployActivation` (the M3 config gate
  + cursor edge) — BOTH before any mutation or self-record. §3.5 proves the decision is fail-closed
  for the whole cell space AND that the legitimate fresh-DB cell still serves: the only cells that
  reach the legacy `:399` writer are `applied_owner ∈ {0, ==20}` × `cursorState == none` (no
  transcript: fresh-DB bring-up / inert legacy) and `applied_owner ∈ {0, ==20}` ×
  `complete`-and-already-in-sync (an idempotent rewrite of the same fingerprint); no
  revoke-embedding binary, no transcript-with-a-pending-change, and no genuine `1..19` shortfall
  ever reaches it.

### 1.1 Step taxonomy (the closed set P4 ships) — unchanged from v1–v6 (re-anchored to current `main`)

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

### 1.2 `deploy_cursor` + the immutable `deploy_plan` transcript (BC-N1) and crash-resume semantics (C1) — carry-forward

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
  it is gated by step (0); the LEGACY `connection.go:399` writer is governed by the W (M5) + A
  (§3.3a, M3) gates, proven over the whole cell space in §3.5.

### 1.3 The load-bearing disambiguation (C1 + BC-N1 + BC-N2 + M1 + M3) — carry-forward (+ the M5 fresh-DB note)

`deploy_cursor` is the **second signal** alongside the drift gate's `LiveFingerprint ==
ExpectedFingerprint`. The rows carry forward from v6 verbatim. **M5 note:** §1.3 keys on
`deploy_cursor.state` (the A dimension), which is orthogonal to `applied_owner` (the W dimension);
the last "absent `deploy_cursor` / `idle`" row is where the fresh / single-role / no-authority
bootstrap (`applied_owner == 0`, NO transcript) lands — it serves on the legacy path for a
no-revoke binary with the flag OFF, exactly as the source serves a fresh DB.

| `deploy_cursor.state` | condition (vs the **stored transcript** + the binary) | classification | action |
| --- | --- | --- | --- |
| `complete` | `deploy_plan[plan_hash]` present; full transcript byte-matches binary; `== ExpectedFingerprint`; **`decoupledEnabled` (or a no-revoke binary already in-sync, §3.3a)** | **in sync** | **serve** (verify-only when decoupled; legacy no-op-apply when a no-revoke binary is already in-sync) |
| `complete` | **`revokeEmbedded == true` AND `decoupledEnabled == false`** (M3) | **config halt** | refuse-to-serve **`awaiting_deploy_config`**, DB untouched — never reaches `ApplyMigrations`/`:399` |
| `complete` | **no-revoke binary, flag OFF, but `ExpectedFingerprint() ≠` recorded OR `cursor.plan_hash ≠ expected`** (M3 comparison) | **pending change over a transcript** | refuse-to-serve **`awaiting_deploy`**, DB untouched — never auto-applies on serve-boot |
| `in_progress` / `step_committed` | present; the FULL transcript byte-matches the binary AND already-applied DB stamps match | **incomplete, resume off the STORED transcript** (BC-N1) | refuse-to-serve `awaiting_deploy`; `deploy` resumes at `cursor.step_index` |
| `in_progress` / `step_committed` | present but the cursor's `plan_hash` ≠ the binary's freshly-computed pending plan | **incomplete, resume with the STORED plan** (BC-N1) | refuse-to-serve `awaiting_deploy`; resume off the stored transcript by `cursor.plan_hash` — NOT drift |
| `finalizing` | present; full transcript byte-matches binary | **resumable finalization** (C1) | refuse-to-serve `awaiting_deploy`; re-run runs the idempotent finalizer (which re-verifies, §3.4 step 0) |
| `in_progress`/`step_committed`/`finalizing`, `deploy_plan[plan_hash]` ABSENT, **OR ANY stored step `sha256` ≠ the binary's embedded bytes** | full-transcript mismatch (M1) | **`deploy_plan_binary_mismatch`** | refuse-to-serve; apply NOTHING; operator inspects |
| `in_progress`/`step_committed`/`finalizing`, transcript byte-matches, **but an already-applied step's DB stamp ≠ the stored transcript** (M1) | DB-stamp mismatch | **`deploy_plan_db_stamp_mismatch`** | refuse-to-serve; do NOT finalize; apply NOTHING |
| absent `deploy_cursor` table / `state == idle` (NO transcript) | n/a | **no deploy in flight** | **legacy path (flag OFF) + no-revoke binary:** serve (auto-apply, the legitimate `:399` cell — **incl. the fresh `applied_owner == 0` bootstrap, M5**); **decoupled (flag ON) OR revoke-embedding binary:** refuse `awaiting_deploy`/`awaiting_deploy_config` (C2 + M3 — §3.3a) |
| `aborted` | n/a | **aborted** | refuse-to-serve `awaiting_deploy` |
| `complete`, `plan_hash` ≠ expected (foreign) | foreign | **genuine drift** | refuse-to-serve (`schema_drift`) |

Load-bearing properties this table enforces (unchanged from v6): plan identity is a stored fact and
resume reads `deploy_plan[cursor.plan_hash]` (BC-N1); a resume/finalize binary that disagrees with
ANY stored step is forced to a typed mismatch and never writes a fingerprint (M1); `complete` ⇒ 0021
applied (C1); incomplete-never-serves holds for EVERY deployer-aware binary (BC-N2); a `complete`
cursor no longer short-circuits the activation guard (M3). **M5 adds nothing to §1.3 — it corrects
the orthogonal `applied_owner` dimension at W (§3.5).**

---

## 2. Decision Q4 — Is a deploy itself a Striatum run? (carry-forward, sound)

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape door held open
by three seams.** Unchanged from v1–v6; no falsifier contested it. The bootstrapping paradox is
dispositive (a dogfooded run needs the daemon + schema up, which is exactly what the deploy brings
online), so the base deployer is a plain verb against a bare/owner-only DB, the same context
`daemon owner-ddl apply` (`daemon.go:90`) occupies. The door stays open because (1) the plan is a
content-addressed durable artifact (`plan_hash` + `deploy_plan`); (2) the receipt is hash-chained
into `audit_log`; (3) the verb is a thin CLI over a `Deployer.Apply(ctx, plan, cursor)` core a
future run step invokes unchanged.

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

### 3.2 The deploy plan — materialized ONCE into an immutable transcript (BC-N1), revoke 0021 sorted last (C3) — carry-forward

Carry-forward from v6, unchanged. Plan identity is computed once at deploy start from the two
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

**Bindings (every `owner-ddl apply` route, current `main`), unchanged from v6:** (1)
`ApplyOwnerBundles` (`owner.go:265-303`) loads `OwnerDDLApplyBundles()` at `:269`; (2)
`applyPendingOwnerBundles` (`:309-322`) keeps its `<= current` bound (`:312`) + an in-loop
`isNonRevokeBundle` guard; (3) `ReapplyAllOwnerBundles` (`:332-351`) adds the same in-loop guard
+ its nil-fallback (`:336-342`) loads `OwnerDDLApplyBundles()`; (4) the self-heal branch
(`:277-302`) is bound by (1) AND (3); (5) all test callers inherit the filter; (6) no owner-ddl
dry-run/list surface exists today. **The embed/listing split:** `OwnerBundles()` (`:197-224`)
stays the full loader (includes 0021) for `revokeEmbedded`, `ExpectedFingerprint`
(`schema_drift.go:88`), `BuildPlan`, and `RuntimeOwnedTablesAlterable`
(`owner_runtime_ownership.go:52`); `OwnerDDLApplyBundles()` is the disjoint apply slice.

**Build-time guard — split phase-aware (M4 — carry-forward from v6).** F16 is split into two
phase-aware tests that each land green in their own rollout phase:

- **`TestOwnerDDLApplyExcludesSyntheticRevokeBundle` (F16a — pre-0021 / inert, rollout step 2).**
  Drives the filter through a **synthetic bundle list / test hook** (a hand-built `[]OwnerBundle`
  containing a synthetic `{Version: 21}` entry, and/or an injectable `ownerBundleFS` test seam):
  asserts (a) `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21` from the
  synthetic list; (b) `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` skip a hand-passed
  synthetic 0021; (c) `ReapplyAllOwnerBundles(nil, …)` resolves its fallback to the filtered
  loader. It does **NOT** assert production `OwnerBundles()` contains 0021 (it does not yet).
- **`TestOwnerDDLApplyExcludesProductionRevokeBundle` (F16b — activation, rollout step 7, after
  0021 is authored).** Asserts (a) production `OwnerBundles()` DOES contain 0021; (b)
  `ExpectedFingerprint()` includes 0021's bytes; (c) `revokeEmbedded` derives from the full loader
  / file presence in `ownerBundleFS`; (d) production `OwnerDDLApplyBundles()` excludes 0021. The
  forced-self-heal pgtest (below) lives here.
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
3. **M3 tightening (carry-forward).** A **revoke-embedding binary** (the binary ships 0021) is
   **structurally barred from the legacy `ConnectAndMigrate` `ApplyMigrations`/`:399` path** by
   the hoisted config gate in `CheckDeployActivation` (§3.3a): with the flag OFF it halts
   `awaiting_deploy_config` BEFORE `:353`, for EVERY cursor state including `complete`. So once a
   binary embeds the revoke, its ONLY serving mode is the decoupled `ConnectAndVerify` path.
4. **No window serving an unmigrated schema (R2).** The decoupled boot verifies before serving;
   the watermark interlock is preserved verbatim, and **the fresh `applied_owner == 0` bootstrap
   still serves on the legacy path for a no-revoke flag-OFF binary (M5)** — the decoupling does NOT
   wedge fresh-DB bring-up.

#### 3.3a The deploy-activation interlock (A) — carry-forward verbatim from v6 (the M3 hoisted config gate + the no-revoke complete comparison) + the universal pre-revoke cursor edge (BC-N2) + the C2 arms

A **`CheckDeployActivation(ctx, runner, revokeEmbedded, decoupledEnabled bool) error`** (in
`go/pkg/db/owner.go` or a new `deploy_activation.go`) is called at the same boot site, **after**
`CheckOwnerBundleWatermark` (W, `connection.go:349`) and **BEFORE `ApplyMigrations`** (`:353`), in
both `ConnectAndMigrate` and `ConnectAndVerify`, and before `RecordSchemaFingerprint` (`:399`). It
mutates nothing. It reads `deploy_cursor` defensively (absent table/row → `cursorState = none`).
**It does NOT read `applied_owner`** — the owner-watermark dimension is W's responsibility (the M5
split), and A's outcome is identical for `applied_owner ∈ {0, ==20}` (both pass W). **Predicate
(fail-closed), in order — carry-forward verbatim from v6:**

0. **(M3 — the hoisted universal decoupling-config gate, fires FIRST for EVERY cursor state.)**
   If **`revokeEmbedded && !decoupledEnabled`** → return **`awaiting_deploy_config`**, DB
   untouched. *A binary that ships the DDL-revoke (0021) must run on the decoupled path; it must
   NEVER take the legacy `ConnectAndMigrate` mutate+self-record path, regardless of cursor state.*
1. **UNIVERSAL incomplete-deploy edge (BC-N2 — carry-forward verbatim).** If `cursorState ∈
   {in_progress, step_committed, finalizing}` → return **`awaiting_deploy`**, DB untouched.
2. If `cursorState == aborted` → return **`awaiting_deploy`**, DB untouched.
3. If `cursorState == complete`:
   - **`decoupledEnabled == true`** → defer to the drift gate on the VERIFY path: `plan_hash ==
     expected` + `LiveFingerprint == ExpectedFingerprint` → **serve verify-only** (return nil; no
     `ApplyMigrations`, no `:399`); foreign `plan_hash` or fingerprint mismatch → **`awaiting_deploy`**
     (or `schema_drift` under refuse mode).
   - **`decoupledEnabled == false`** (⇒ `revokeEmbedded == false`, since step 0 caught
     revoke-embedding + flag-OFF): **(M3 — the no-revoke complete comparison), a
     pre-`ApplyMigrations`, DB-untouched, PURE-READ check:** if **`ExpectedFingerprint() ==
     LiveFingerprint(recorded)` AND `cursor.plan_hash == expected`** → **return nil** (in-sync; the
     subsequent legacy `ApplyMigrations` is a no-op and the legacy `:399` self-record is an
     idempotent rewrite of the SAME fingerprint — serve). Otherwise → **return `awaiting_deploy`**,
     DB untouched.
4. If `cursorState == none` (absent table/row, or `idle` — NO transcript present):
   - `decoupledEnabled == true` → **`awaiting_deploy`** (the decoupled boot never auto-applies;
     fresh-DB bring-up runs `deploy`).
   - `decoupledEnabled == false` (⇒ `revokeEmbedded == false`, by step 0):
     - **return nil**: legacy path, serve, `ApplyMigrations` runs. **This is the legacy serve-legacy
       branch reached by both the fresh `applied_owner == 0` bootstrap and the inert-landing
       `applied_owner == 20` re-boot (both pass W); it carries NO deploy transcript, so it is the
       ONLY branch that legitimately reaches the mutating legacy `:399` writer (M5/M3).**

`revokeEmbedded` = the embedded owner-bundle FS contains the bundle at `DDLRevokeOwnerBundleVersion
= 21` (NOT `Latest >= 21`; `LatestOwnerBundleVersion` stays 20). `decoupledEnabled` =
`STRIATUM_DEPLOY_DECOUPLED` is set.

**Net invariant (M3 + BC-N2 + C2 + the M5 W split):** no boot serves while the cursor is
non-`complete`; no revoke-embedding binary ever reaches `ApplyMigrations`/`:399`; a genuine
authority shortfall (`1..19`) halts at W before A; and the legacy `:399` writer is reached ONLY by a
no-revoke flag-OFF binary that passed W (`applied_owner ∈ {0, ==20}`) over either NO transcript
(`cursorState == none`) or a `complete` transcript it is ALREADY byte-in-sync with. Proven
exhaustively in §3.5.

#### 3.3b Runtime-object ownership: Policy 1 with the revoke sequenced last (C3 — carry-forward, re-anchored)

Carry-forward from v6, unchanged. `Deployer.applyRuntimeStep` (wrapping `applyOne`,
`migrations.go:304-355`): `BEGIN` (owner conn) → assert
`has_schema_privilege('striatumd_rw','striatumd','CREATE')` (else `deploy_create_prerequisite_missing`)
→ snapshot owner-owned oids → run migration SQL + both version stamps + in-tx hash verify →
re-snapshot, `ALTER <kind> … OWNER TO striatumd_rw` for new oids → re-assert DML grants → append
the per-step receipt keyed on the stored `(plan_hash, step_index)` → advance `in_progress(k) →
step_committed(k)` → `COMMIT`. The reconcile runs while `striatumd_rw` still holds CREATE (granted
by bundle 0018 at `0018:98`, in the non-revoke prefix); 0021 commits last, so the steady state
denies CREATE.

### 3.4 The deploy receipt + the idempotent finalizer (C1) + the M1 pre-finalizer gate (carry-forward)

Carry-forward from v6, unchanged. Every applied step writes exactly one hash-chained receipt via
`append_audit_row(...)` keyed on the stored `(plan_hash, step_index)`. The idempotent finalizer:
**(0)** `VerifyStoredTranscript(plan_hash)` (M1) — runs FIRST, writes nothing, ABORTS on
`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`; **(1)** guarded `complete` receipt;
**(2)** `RecordSchemaFingerprint` (`schema_drift.go:171-195`) UPSERT; **(3)** advance `finalizing →
complete` last. The finalizer crash-point table and `doctor schema_deploy_unrecorded` (transcript-
enumerated + the M1 stamp/byte WARN) carry forward verbatim.

### 3.4a `VerifyStoredTranscript` — the full-transcript byte + DB-stamp verifier (M1, carry-forward)

Carry-forward from v6, unchanged. A single PURE-READ function checks, for EVERY step in
`deploy_plan[plan_hash].steps` (already-applied AND not), `step.sha256 ==` the running binary's
embedded bytes (`MigrationSHASet()` `migrations.go:197` for runtime; `OwnerBundle.SHA256()`
`owner.go:191` for owner) ⇒ `ErrDeployPlanBinaryMismatch`; and for every already-applied step,
`step.sha256 ==` the DB stamp (`schema_migrations.sha256` `migrations.go:275` / `owner_bundle_meta.sha256`
`owner.go:529`) ⇒ `ErrDeployPlanDBStampMismatch`. Called (i) on every resume before any apply,
(ii) as finalizer step 0, (iii) by the doctor. The two typed halts join the
`AwaitingOwnerDDLError`/`SchemaDriftError` `errors.As` arms (`main.go:208-227`); all map to
refuse-to-serve, DB untouched.

---

## 3.5 The complete boot-path decision table — REVISED for M5 (the `applied_owner` dimension SPLIT; proactive completeness; preempts M6)

> **The SEED requirement (now with the split).** A revision that pins only the prior findings but
> leaves an unaudited boot-path combination open spawns the next finding (the v6 SEED warned an
> "unaudited boot-path combination" would spawn an M5 — and it did, in the owner-watermark
> dimension). This table closes that class: for EVERY `cursorState × decoupledEnabled ×
> revokeEmbedded × applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}` cell it
> states the exact guard/outcome, PROVES §4.5 Universal Invariant B holds, AND proves the
> legitimate fresh-DB / inert-landing cells STILL SERVE (not wedged). It is an **executable, named
> requirement** (**F18**, `T-deploy-bootpath-decision-table`) the falsifiers verify against
> `CheckOwnerBundleWatermark`'s `applied == 0` exception (`owner.go:145`), the revised §3.3a
> predicate sites, and the `connection.go:349/:353/:376-383/:399` ordering.

**Two guard sites, evaluated in order on every boot, both BEFORE `ApplyMigrations` (`:353`) /
`RecordSchemaFingerprint` (`:399`), mutating nothing:**

- **W = `CheckOwnerBundleWatermark` (`connection.go:349`, `owner.go:124-154`) — the M5 dimension,
  respecified to FOUR disjoint `applied_owner` buckets covering all of ℕ (matching the live source
  exactly):**
  - **`applied_owner == 0` (no authority schema / fresh / single-role bootstrap)** → **return nil,
    proceed to A.** `owner.go:145` `if applied == 0 { return nil }` returns BEFORE the
    `if applied < RequiredOwnerBundleVersion` shortfall check (`:148-150`). `OwnerBundleVersion`
    returns 0 when `owner_bundle_meta` is absent (`:234-235`); a fresh migrated DB starts at
    version 0 (`owner_pg_test.go:19`). **This is the M5 serve bucket — NOT a halt.**
  - **`1 <= applied_owner < 20` (genuine authority shortfall)** → **`awaiting_owner_ddl`**, DB
    untouched (`owner.go:148-150`). The database HAS an authority schema but lags the frontier —
    the pending owner DDL is a hard prerequisite (`owner-ddl apply` out-of-band). W halts BEFORE A.
  - **`applied_owner == 20` (in-sync at the frontier)** → in-sync, **return nil, proceed to A**
    (`owner.go:151-153`).
  - **`applied_owner >= 21` (0021 applied)** → a **revoke-embedding** binary tolerates forward →
    proceed to A; a **no-revoke** binary → **`awaiting_deploy`** (the C2 forward-watermark rule at
    `owner.go:151-153`, re-anchored to 21 — a rolled-back binary on a revoke-applied DB). This is
    **barrier (b)** that keeps a no-revoke binary off a revoke-applied DB.
- **A = `CheckDeployActivation` (§3.3a, carry-forward from v6).** Step 0 (M3 config gate) is
  **barrier (a)** that keeps every revoke-embedding binary off the legacy path. **A does NOT read
  `applied_owner`** — so the A-gate outcome is IDENTICAL for the `0` and `==20` columns (both pass
  W). That identity is the structural core of the M5 fix: the fresh `applied_owner == 0` cell
  behaves exactly like the in-sync `applied_owner == 20` cell at A, so it SERVES at row 1.

**Why the split is the whole fix.** v6 collapsed `applied_owner < 20` into one "ALWAYS halts at W"
column; that is false for `applied_owner == 0` (which W serves). Splitting `<20` into `{0, 1..19}`
makes the table match source: the `0` column mirrors the `==20` column's W-pass-then-A behavior
(serving the fresh-DB cell at row 1), while `1..19` is the genuine "ALWAYS halts at W" shortfall.
The `==20` and `>=21` columns carry forward from v6 verbatim.

The 16 `(cursorState × decoupledEnabled × revokeEmbedded)` rows × **4** `applied_owner`
sub-columns = **64 cells**, all specified. `cursorState` groups `step_committed` with `in_progress`
and treats `aborted` like the non-complete edge (both → `awaiting_deploy` at A step 1/2), per §1.3.

| # | cursorState | decoupled | revoke | applied_owner **`==0` (fresh/no-authority)** | applied_owner **`1..19` (shortfall)** | applied_owner `==20` | applied_owner `>=21` |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | none | off | no | **SERVE-legacy — FRESH-DB BRING-UP** (W nil for `applied==0`; A4 none/legacy; `:399` reachable — NO transcript) **← the M5 serve cell** | `awaiting_owner_ddl` (W shortfall) | **SERVE-legacy — inert-landing re-boot** (A4; `:399` reachable — NO transcript) | `awaiting_deploy` (W fwd-watermark, barrier b) |
| 2 | none | off | yes | **`awaiting_deploy_config`** (W nil, then A0 barrier a — a revoke-embedding binary on a fresh DB still halts) | `awaiting_owner_ddl` (W shortfall) | **`awaiting_deploy_config`** (A0 barrier a) | **`awaiting_deploy_config`** (A0; W tolerates revoke, then barrier a) |
| 3 | none | on | no | `awaiting_deploy` (A4: decoupled never auto-applies; run deploy to bring up fresh DB) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A4: decoupled never auto-applies) | `awaiting_deploy` (W fwd-watermark) |
| 4 | none | on | yes | `awaiting_deploy` (A4: run deploy to apply bundles + 0021) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A4: run deploy to apply 0021) | `awaiting_deploy` (A4; cursor=none+0021-applied inconsistent → conservative halt) |
| 5 | in_progress/step_committed/aborted | off | no | `awaiting_deploy` (A1/A2, BC-N2) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1/A2, BC-N2) | `awaiting_deploy` (W fwd-watermark) |
| 6 | in_progress/step_committed/aborted | off | yes | **`awaiting_deploy_config`** (W nil, then A0 barrier a) | `awaiting_owner_ddl` (W shortfall) | **`awaiting_deploy_config`** (A0 barrier a) | **`awaiting_deploy_config`** (A0) |
| 7 | in_progress/step_committed/aborted | on | no | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_deploy` (W fwd-watermark) |
| 8 | in_progress/step_committed/aborted | on | yes | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1/A2, BC-N2 resume) | `awaiting_deploy` (A1/A2; W tolerates revoke) |
| 9 | finalizing | off | no | `awaiting_deploy` (A1, BC-N2) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1, BC-N2) | `awaiting_deploy` (W fwd-watermark) |
| 10 | finalizing | off | yes | **`awaiting_deploy_config`** (W nil, then A0 barrier a) | `awaiting_owner_ddl` (W shortfall) | **`awaiting_deploy_config`** (A0 barrier a) | **`awaiting_deploy_config`** (A0) |
| 11 | finalizing | on | no | `awaiting_deploy` (A1, BC-N2) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1, BC-N2) | `awaiting_deploy` (W fwd-watermark) |
| 12 | finalizing | on | yes | `awaiting_deploy` (A1, BC-N2 resume finalize) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A1, BC-N2 resume finalize) | `awaiting_deploy` (A1; W tolerates revoke) |
| 13 | complete | off | no | **`awaiting_deploy`** (A3 no-revoke comparison: `complete` cursor over a fresh `applied==0` DB is not byte-in-sync → halt, DB untouched; the degenerate in-sync sub-case is an idempotent `:399` rewrite) | `awaiting_owner_ddl` (W shortfall) | **SERVE-legacy if in-sync, else `awaiting_deploy`** (A3 no-revoke comparison; `:399` only on the idempotent no-op rewrite) | `awaiting_deploy` (W fwd-watermark) |
| **14** | **complete** | **off** | **yes** | **`awaiting_deploy_config`** (W nil, then A0 barrier a — **the M3 cell**) | `awaiting_owner_ddl` (W shortfall) | **`awaiting_deploy_config`** (A0 barrier a — **the M3 cell**) | **`awaiting_deploy_config`** (A0 — **the post-deploy steady state with the flag erroneously OFF; the v5 reproducer, halted**) |
| 15 | complete | on | no | `awaiting_deploy` (A3 decoupled: fresh `applied==0` DB not in-sync → run deploy) | `awaiting_owner_ddl` (W shortfall) | **SERVE-verify if in-sync, else `awaiting_deploy`** (A3 decoupled) | `awaiting_deploy` (W fwd-watermark) |
| **16** | **complete** | **on** | **yes** | `awaiting_deploy` (A3 decoupled: 0021 not yet applied on a fresh DB → not in-sync → run deploy) | `awaiting_owner_ddl` (W shortfall) | `awaiting_deploy` (A3 decoupled: 0021 pending → not in-sync → run deploy) | **SERVE-verify** (A3 decoupled, in-sync — **the steady-state P4-activated serving cell; zero serve-path DDL**) |

**What changed vs v6 (M5):** the v6 single `<20` column ("ALWAYS halts at W", false for
`applied==0`) is replaced by TWO columns — **`==0`** (mirrors the `==20` column: W passes → A;
**serves at row 1** as the fresh-DB bring-up) and **`1..19`** (genuinely ALWAYS `awaiting_owner_ddl`
at W). The `==20` and `>=21` columns are **verbatim from v6**. Cell `==20`/row 1 is relabeled the
**inert-landing re-boot** cell; the genuine fresh-DB cell is `==0`/row 1.

**Universal Invariant B proven in every cell.** The legacy `connection.go:399` writer mutates
schema / self-records around `VerifyStoredTranscript` only if a boot reaches `:353`/`:399` on the
legacy `ConnectAndMigrate` path. Across all 64 cells the ONLY cells that reach `:399` are:

- **Cell 1 / `==0`** (`none`, off, no-revoke, fresh DB): NO transcript present (`cursorState ==
  none`); the running binary IS the applier of its own just-applied schema (fresh-DB bring-up).
  Invariant B is not in scope — there is no transcript to write around. ✓ **← the M5 serve cell,
  matching `CheckOwnerBundleWatermark`'s `applied == 0` exception (`owner.go:145`).**
- **Cell 1 / `==20`** (`none`, off, no-revoke, inert-landing): NO transcript present; an inert
  binary recording its own just-applied schema on an already-bundled DB. Invariant B not in scope. ✓
- **Cell 13 / `==20`, in-sync** (`complete`, off, no-revoke, `ExpectedFingerprint() == recorded`):
  the pre-`ApplyMigrations` comparison (A3) already proved in-sync, so `ApplyMigrations` is a no-op
  and `:399` rewrites the IDENTICAL fingerprint. No mutation, no change to `schema_state`. ✓
- **Cell 13 / `==0`, degenerate in-sync** (`complete`, off, no-revoke, a fresh DB that nonetheless
  reads in-sync): same A3 in-sync guard — an idempotent rewrite of the same fingerprint. In
  practice this cell halts `awaiting_deploy` (a `complete` cursor over an `applied==0` DB is not
  byte-in-sync), so even the idempotent rewrite rarely fires; Invariant B holds either way. ✓

Every other cell either halts before `:353` (DB byte-identical) or takes the decoupled
`ConnectAndVerify` path (which never calls `:353`/`:399`):

- **The entire `1..19` column (16 cells)** → `awaiting_owner_ddl` at **W**, BEFORE A and BEFORE
  `:353`. DB untouched. Invariant B trivially holds (no write). **← the M5 halt cell — the genuine
  authority shortfall, correctly halted.**
- **Every revoke-embedding + flag-OFF cell (2, 6, 10, 14, in the `==0`/`==20`/`>=21` columns)** →
  `awaiting_deploy_config` at A0 (barrier a). `ApplyMigrations` NOT called, `RecordSchemaFingerprint`
  NOT called, `schema_state` unchanged, DB byte-identical. **This is the M3 close** — cell 14/`>=21`
  is the exact v5 reproducer, now a clean halt; cell 2/`==0` proves a revoke-embedding binary on a
  fresh DB with the flag OFF ALSO halts (the preserved asymmetry — the M5 serve cell is no-revoke
  only).
- **Every no-revoke binary on a revoke-applied DB (`>=21` in cells 1, 3, 5, 7, 9, 11, 13, 15)** →
  `awaiting_deploy` at W (barrier b). Never reaches `:353`.
- **Every non-`complete` cursor not already caught by A0** (cells 5, 7, 8, 9, 11, 12 in the `==0`
  and `==20` columns; and the `aborted` variants) → `awaiting_deploy` at A1/A2 (BC-N2). DB untouched.
- **Every decoupled cell (3, 4, 7, 8, 11, 12, 15, 16)** runs `ConnectAndVerify`, which NEVER calls
  `ApplyMigrations`/`:399`; it serves only on the in-sync `complete` row (cell 16/`>=21`, cell
  15/`==20`) and otherwise halts `awaiting_deploy`. The deployer is the only writer of `schema_state`
  on this path, gated by `VerifyStoredTranscript` (M1).

**The shadow-mode drift-gate fall-through (`connection.go:384-399`) is also covered (unchanged).**
The only legacy-path cells that even reach the drift gate are 1/`==0`, 1/`==20`, and 13/`==20`-in-sync
(plus the degenerate 13/`==0`-in-sync). Cell 1 (`==0` or `==20`) has no recorded fingerprint
divergence to log on a fresh/just-applied DB (it records its own pre-revoke schema); cell 13/in-sync
has `ExpectedFingerprint() == recorded` so `drifted == false` and the shadow log + fall-through
self-record never fires over a divergence. **No legacy shadow-mode self-record can overwrite
`schema_state` around a transcript carrying a pending change.**

**The legitimate fresh-DB / inert-landing cells STILL SERVE (R2, not wedged — the M5 requirement).**
**Cell 1/`==0`** (no-revoke binary, fresh `applied_owner == 0` DB, no transcript) **serves the
fresh-DB bring-up** — `ApplyMigrations` + the legacy `:399` self-record run, exactly as
`CheckOwnerBundleWatermark` serves `applied == 0` today; it is NOT wedged. **Cell 1/`==20`**
(no-revoke inert binary, already-bundled DB, no transcript) serves the inert-landing re-boot, also
not wedged. The conservative halts fire ONLY where they must: the `1..19` genuine shortfall (W), a
revoke-embedding binary with the flag OFF (cells 2/6/10/14, A0), an incomplete/aborted cursor
(BC-N2), or a no-revoke binary on a revoke-applied DB (barrier b). **Nothing that must serve is
wedged — the M5 defect is closed.**

---

## 4. Serving-role DDL revocation (owner bundle 0021), without lockout, without a pre-revoke serve window, without an early self-heal revoke, without a legacy-path revoke bypass, AND without wedging fresh-DB bring-up

**Claim (DDL-REVOKE).** The serving role `striatumd_rw` loses its remaining object-creation DDL on
the serving path, shipped as owner bundle 0021, applied **only as the terminal step of `striatum
daemon deploy`** (C3), **gated by the universal cursor edge (BC-N2)**, **excluded from every
`owner-ddl apply` route incl. the FMA-007 self-heal (M2)**, **never reachable via the legacy
serve-boot `ConnectAndMigrate` path on a `complete` cursor (M3)**, **AND — clarified in v7 (M5) —
without wedging the legitimate fresh `applied_owner == 0` bootstrap, which still serves on the
legacy path for a no-revoke flag-OFF binary** — so failure mode 1 becomes structurally impossible
across the whole boot-path cell space without over-halting a healthy first boot.

### 4.1 What is revoked, precisely (carry-forward)

Bundle 0021 revokes object creation on the runtime schema: `REVOKE CREATE ON SCHEMA striatumd FROM
striatumd_rw;` (+ `REVOKE TEMPORARY` if granted). `striatumd_rw` retains ownership of (and
`ALTER`/`DROP` on) the runtime tables it already owns; new runtime objects are reconciled back to
`striatumd_rw` before 0021 commits (§3.3b). Full capability revocation (owner re-owns runtime
tables) remains a named follow-up beyond P4.

### 4.2 Typed halts + the forward-watermark rule (C2) + the universal cursor edge (BC-N2) + the M1 mismatch halts + the M3 config gate + the M5 fresh-DB serve

Sibling halts of `AwaitingOwnerDDLError` (`owner.go:42,58`), reusing the non-restartable exit
(`main.go:208-227`):

- **`AwaitingOwnerDDLError`** (`awaiting_owner_ddl`): the genuine authority shortfall, `1 <=
  applied_owner < 20` (`owner.go:148-150`). DB untouched. **NOT fired for `applied_owner == 0`
  (the M5 fresh-DB serve, `owner.go:145`).**
- **`AwaitingDeployError`** (`awaiting_deploy`): a deploy is pending/incomplete (BC-N2), OR a
  no-revoke binary over a `complete` transcript not in-sync (A3, M3), OR a no-revoke binary on a
  revoke-applied DB (`applied_owner >= 21`, barrier b). DB untouched.
- **`AwaitingDeployConfigError`** (`awaiting_deploy_config`): the binary ships 0021 but
  `STRIATUM_DEPLOY_DECOUPLED` is OFF — for EVERY cursor state including `complete` (the hoisted M3
  gate, §3.3a step 0), and in EVERY `applied_owner` column that passes W (`==0`, `==20`, `>=21`).
  DB untouched. Remediation: "this binary ships the DDL-revoke; set `STRIATUM_DEPLOY_DECOUPLED=1`
  to serve verify-only, or run `striatum daemon deploy` to apply a pending change."
- **`DeployPlanBinaryMismatchError`** / **`DeployPlanDBStampMismatchError`** (M1). DB untouched.

**Forward-watermark rule (`CheckOwnerBundleWatermark` tolerate-forward branch `owner.go:151-153`),
re-anchored to 21 — carry-forward.** `applied >= 21` + this binary does NOT embed 0021 →
`AwaitingDeployError`. This is barrier (b) (the `>=21` column). `RequiredOwnerBundleVersion` stays
**20** (`owner.go:35`); the revoke is gated by the cursor edge + `CheckDeployActivation` + the flag
+ its deploy-plan-terminal placement + the `OwnerDDLApplyBundles` exclusion — NOT the watermark
frontier. **The M5 split adds the `0` and `1..19` columns below the frontier; it does NOT advance
`Required` or change the `applied >= 21` forward rule.**

### 4.3 The deploy choreography (C2 — carry-forward; M3-hardened; M5-clarified)

Two binaries, in order:

1. **Inert-landing binary** (`Latest = Required = 20`; does NOT embed 0021). Ships the `Deployer`
   engine + `deploy` verb, migration 0044, the decoupled boot path (flag OFF), the doctor block,
   the forward-watermark rule, the universal pre-revoke cursor edge (BC-N2), `VerifyStoredTranscript`
   (M1), `OwnerDDLApplyBundles` (M2), and the `CheckDeployActivation` with the hoisted M3 config
   gate + the no-revoke complete comparison. **Boot auto-applies legacy only because no deploy is
   in flight and the binary embeds no revoke — and W serves `applied_owner == 0` (fresh DB) AND
   `applied_owner == 20` (already-bundled), so this binary brings up a fresh DB (cell 1/`==0`) and
   re-boots an existing DB (cell 1/`==20`) without wedging either (M5).** **Daemon up, serving.**
2. **Activation binary** (embeds 0021; `Latest = Required` still 20). On boot with the flag OFF it
   halts **`awaiting_deploy_config` for EVERY cursor state** (cells 2/6/10/14 — the M3 gate), in
   every `applied_owner` column that passes W. The operator, with the daemon parked: sets
   `STRIATUM_DEPLOY_DECOUPLED=1`; runs `striatum daemon deploy`; restarts the daemon →
   `ConnectAndVerify` sees `complete` + in-sync → serves verify-only, zero serve-path create-DDL
   (cell 16/`>=21`). **From activation onward the operator keeps `STRIATUM_DEPLOY_DECOUPLED=1`
   set.**

   **Crash mid-deploy is safe even if the inert binary comes back up (BC-N2).** Its universal
   cursor edge halts `awaiting_deploy` (cells 5/7/9). **`owner-ddl apply` before deploy is safe
   (M2).** **A later revoke-embedding binary that restarts with the flag OFF — at ANY cursor state
   — halts `awaiting_deploy_config` (M3, cells 2/6/10/14).** **And a fresh `applied_owner == 0` DB
   first boot of the inert binary still serves (cell 1/`==0`, M5) — the reframe does not wedge a
   healthy first boot.**

### 4.4 How 0021 ships without lockout (R3), without a pre-revoke serve window (BC-N2), without an early self-heal revoke (M2), without a legacy-path revoke bypass (M3), and without wedging fresh-DB bring-up (M5)

The deployer applies all DDL over the owner/admin connection. **C3:** 0021 is the terminal plan
step and excluded from the owner prefix. **M2:** every `owner-ddl apply` route iterates
`OwnerDDLApplyBundles()` (0021 excluded) + both loops guard `isNonRevokeBundle` in-loop. **BC-N2:**
every deployer-aware binary refuses on a non-`complete` cursor before `ApplyMigrations`/`:399`.
**M3:** every revoke-embedding binary with the flag OFF refuses (`awaiting_deploy_config`) for
EVERY cursor state, and the forward-watermark rule refuses a no-revoke binary on a revoke-applied
DB. **M5:** the owner-watermark gate W serves the fresh `applied_owner == 0` bootstrap (and the
`applied_owner == 20` re-boot) on the legacy path, halting ONLY the genuine `1..19` shortfall — so
the reframe does not wedge a healthy first boot. Combined, **no ordering and no boot-path cell
(§3.5) drives boot into `ApplyMigrations` under a revoked CREATE, lets any binary serve an
incomplete deploy, strands a deploy at a reconcile under a revoked CREATE, commits the revoke
outside the terminal deploy step, self-records around the full-transcript gate, OR wedges a
legitimate fresh / single-role boot.**

### 4.5 Proactive hardening — every apply path and every fingerprint path named, two universal invariants (TIGHTENED for M3; the M5 split re-proven)

Audited once, exhaustively, against current `main` (HEAD `9ceeb288`; `git diff --stat 2e482077 HEAD
-- go/pkg/db/` empty).

**A. EVERY code path that applies owner bundles (file:line) — carry-forward from v6, unchanged.**
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

| Path | Site | Disposition under M1 + M3 + **M5** |
| --- | --- | --- |
| `RecordSchemaFingerprint` (the ONLY `schema_state.fingerprint` writer) | `schema_drift.go:171-195` | unchanged; gated by callers |
| Legacy boot self-record | `connection.go:399` ("This is the only writer of schema_state") | **reachable ONLY when W returns nil (`applied_owner ∈ {0, ==20}`) AND the §3.3a activation guard returns nil on the legacy path — i.e. (cell 1/`==0` or 1/`==20`) `cursorState == none`, no-revoke binary, flag OFF (NO transcript); or (cell 13/`==20` or the degenerate 13/`==0`) `complete`, no-revoke, flag OFF, already byte-in-sync (an idempotent rewrite). W's `1..19` shortfall halts BEFORE this site; W's `>=21`-no-revoke barrier (b) halts before it; barrier (a) (the M3 config gate) bars EVERY revoke-embedding binary; the A3 no-revoke comparison bars a pending change over a transcript. NEVER on the decoupled `ConnectAndVerify` path.** |
| C1 finalizer `schema_state` UPSERT + `finalizing → complete` | P4 `Deployer.Apply` (§3.4) | gated by `VerifyStoredTranscript` finalizer step 0 (M1) — writes nothing on mismatch |
| `ExpectedFingerprint()` / `LiveFingerprint` / `CheckSchemaDrift` (pure reads) | `schema_drift.go:83-100,145-161,254-274` | read-only; no write (the A3 comparison reuses these) |

> **UNIVERSAL INVARIANT B (executable, TIGHTENED for M3; the M5 split re-proven).** *NO
> `schema_state` fingerprint / `complete` is EVER written around `VerifyStoredTranscript` while a
> deploy transcript carrying a pending change is present, and NO revoke-embedding binary EVER
> reaches the legacy `:399` writer.* Two writers, both bounded: (i) the deployer finalizer is gated
> by `VerifyStoredTranscript` step 0 (M1); (ii) the legacy `connection.go:399` writer is reachable
> ONLY in §3.5 cells 1/`==0`, 1/`==20`, and 13-in-sync (`==20` or the degenerate `==0`) — a
> no-revoke flag-OFF binary that passed W (`applied_owner ∈ {0, ==20}`) over NO transcript
> (`cursorState == none`) or a `complete` transcript ALREADY byte-in-sync — barred from any genuine
> `1..19` shortfall (W), any revoke-embedded state (barrier a), any revoke-applied state (barrier
> b), and any pending-change-over-a-transcript state (the A3 comparison). *Therefore a database
> carrying a `deploy_cursor`/`deploy_plan` transcript with a pending change, any revoke-embedding
> binary, or any genuine authority shortfall can NEVER reach the legacy `:399` writer — AND the
> legitimate fresh `applied_owner == 0` bootstrap STILL reaches it and serves (Invariant B not in
> scope there: no transcript).* Enforced by **F17** (the M3 cell), **F11** (extended), **F15**
> (extended), and **F18** (the §3.5 64-cell decision-table matrix, asserting BOTH the
> `applied_owner == 0` serve cell AND the `1..19` halt cell).

These invariants + the §3.5 decision table are the executable, named requirements the cycle-1
falsifiers verify against the named code sites — so no unaudited boot-path combination (including
the owner-watermark dimension) can spawn an M6.

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> `T-` = unit/integration; `G-` = game-day against a real two-role cluster. F1–F17 carry forward
> from v6 (re-confirmed against current `main`); **F18 is REVISED for M5 — the SPLIT
> `applied_owner` dimension with the dual-cell assertion — and a new sub-assertion F18a pins the
> fresh-DB serve cell** — these are what the cycle-1 falsifiers re-attack first.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1–F10, F12–F14** | Carry forward verbatim from v6 (resumability Q3-A/Q3-B, no serve-boot mutation when decoupled, §1.3 classification incl. the M1 mismatch rows, no P2/P3 regression, DDL-revoke structural, per-step receipt, no double-apply, plan≡fingerprint, the C1 finalization boundary with the M1 step-0 gate, the C3 runtime-object ownership extended for M2, per-step crash-safety, immutable plan identity with M1 already-applied verification). Re-confirmed; anchors in §9. | (as v6) |
| **F11 (Activation fail-closed — C2 + BC-N2 universal edge; M3 (g))** | Carry forward from v6: for every deployer-aware binary, boot never calls `ApplyMigrations`/`RecordSchemaFingerprint` while the cursor is non-`complete`; a no-revoke binary at `applied_owner == 20` with a non-`complete` cursor halts `awaiting_deploy` DB-byte-identical; a no-cursor/`idle` no-revoke binary SERVES; (g) a revoke-embedding binary with the flag OFF halts `awaiting_deploy_config` for EVERY cursor state. | `T-deploy-revoke-activation-ordering` cases (a)–(g) (carry-forward); spies prove `applyOne`/`RecordSchemaFingerprint` never entered. `G-old-binary-refuse`. |
| **F15 (Full-transcript verification on resume AND before the finalizer — M1)** | Carry-forward from v6 (the byte case, the owner-step case, the DB-stamp case, the finalizer arm); the finalizer's `VerifyStoredTranscript` step-0 gate AND the §3.3a M3 config gate together mean NO writer self-records around the full-transcript check. | `T-deploy-resume-already-applied-byte-mismatch-refuses` (as v6) + an assertion that the §3.3a guard precedes `connection.go:353` in BOTH boot paths. `G-wrong-binary-resume`. |
| **F16a / F16b (Revoke excluded from EVERY owner-ddl apply route — M2; synthetic + production phases)** | Carry forward from v6: F16a (synthetic list / injectable FS, step 2) excludes every bundle `>= 21` from both apply loops + the nil-fallback WITHOUT asserting production `OwnerBundles()` contains 0021; F16b (step 7, after 0021 authored) asserts the production embed/listing split + the forced FMA-007 self-heal reaching `ReapplyAllOwnerBundles` via `isCrossBundleDependencyError`. | `TestOwnerDDLApplyExcludesSyntheticRevokeBundle` + grep test (step 2); `TestOwnerDDLApplyExcludesProductionRevokeBundle` + `T-deploy-revoke-excluded-from-reapply-self-heal` (step 7). |
| **F17 (M3 — the complete-cursor legacy mutate+self-record refusal)** | Carry forward from v6: a revoke-embedding binary with a `complete` cursor, flag OFF, and a pending change refuses `awaiting_deploy_config`, calls NEITHER `ApplyMigrations` NOR `RecordSchemaFingerprint`, leaves `schema_state` unchanged and the DB byte-identical, on BOTH boot paths; symmetric shadow-mode arm asserts the fall-through `connection.go:384-399` is never reached. | `T-deploy-complete-cursor-decoupled-off-revoke-embedding-refuses-legacy-mutate-and-selfrecord`; `G-complete-cursor-flag-off-refuse`. |
| **F18 (Boot-path decision table — proactive completeness — REVISED for M5: the SPLIT `applied_owner` dimension, dual-cell assertion)** | EVERY `cursorState ∈ {none, in_progress, step_committed, finalizing, complete, aborted} × decoupledEnabled ∈ {on, off} × revokeEmbedded ∈ {yes, no} × applied_owner ∈ {**0/no authority, 1..19 authority shortfall, ==20, >=21**}` cell produces the EXACT §3.5 outcome; in NO cell does the legacy `:399` writer fire over a transcript carrying a pending change or for a revoke-embedding binary; **AND the table asserts BOTH owner-watermark branches explicitly — the `applied_owner == 0` SERVE cell (cell 1/`==0`: no-revoke, flag OFF, `cursorState=none` → SERVES the fresh-DB bring-up, `ApplyMigrations` + `RecordSchemaFingerprint` CALLED, matching `CheckOwnerBundleWatermark`'s `applied == 0` exception at `owner.go:145`) AND the `1 <= applied_owner < 20` HALT cell (any row → `awaiting_owner_ddl` at W, `ApplyMigrations`/`RecordSchemaFingerprint` un-called, DB byte-identical).** | **`T-deploy-bootpath-decision-table`**: a parametrized matrix over all 64 cells; for each, construct the DB + binary state and assert the §3.5 outcome (the typed halt OR serve-legacy/serve-verify), and assert `ApplyMigrations`/`RecordSchemaFingerprint` spies are called ONLY in cells 1/`==0`, 1/`==20`, and 13/`==20`-in-sync (and in 13 only as an idempotent no-op rewrite). The `applied_owner == 0` arm sets up a DB with `owner_bundle_meta` ABSENT (`OwnerBundleVersion → 0`) and asserts W returns nil + cell 1 serves; the `1..19` arm sets up `owner_bundle_meta` at a version in `[1,19]` and asserts W halts `awaiting_owner_ddl`. Asserts the W gate precedes A and both precede `connection.go:353` on both boot paths. |
| **F18a (Fresh-DB bootstrap serves — the M5 serve cell, NEW)** | A no-revoke binary (`Latest = Required = 20`, 0021 NOT embedded), flag OFF, booting against a FRESH database with `striatumd.owner_bundle_meta` ABSENT (`OwnerBundleVersion → 0`, `owner_pg_test.go:19`) and `cursorState = none` (no `deploy_cursor` transcript) SERVES: `CheckOwnerBundleWatermark` returns nil (the `applied == 0` exception, `owner.go:145`), `CheckDeployActivation` step 4 returns nil, legacy `ConnectAndMigrate` runs `ApplyMigrations` + the `:399` self-record, and the daemon serves — it is NOT wedged `awaiting_owner_ddl`. **Conversely**, the SAME binary against a DB with `owner_bundle_meta` at version `k ∈ [1,19]` halts `awaiting_owner_ddl` DB-untouched. | **`T-deploy-fresh-db-bootstrap-serves`**: (1) fresh DB (no `owner_bundle_meta`, no `deploy_cursor`) + inert binary, flag OFF → assert serve, `ApplyMigrations` spy CALLED, `RecordSchemaFingerprint` spy CALLED, daemon up; (2) DB with `owner_bundle_meta.version ∈ {1, 19}` → assert `awaiting_owner_ddl`, both spies un-called, DB byte-identical. `G-fresh-db-first-boot`: a real fresh two-role cluster first boot of the inert binary serves. |

---

## 6. Shadow-first rollout (the P4 build order) — re-anchored + M3 + M4 + M5

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
   no-revoke `complete` pre-`ApplyMigrations` comparison (step 3)** — on top of the universal
   pre-revoke cursor edge (BC-N2) + the typed halts + **the M5-correct `CheckOwnerBundleWatermark`
   owner-watermark gate (the `applied_owner == 0` serve exception preserved, the `1..19` shortfall
   halt, the forward-watermark rule at `>= 21`)**. Decoupled boot path (`ConnectAndVerify`) behind
   `STRIATUM_DEPLOY_DECOUPLED` (default OFF). **Lands inert for a no-revoke binary, and the fresh
   `applied_owner == 0` bootstrap still serves** (F11 incl. (g), F3, F5, F17, **F18 with the split
   dimension, F18a**).
6. **`doctor schema_deploy_unrecorded`** block — per-step tightened, transcript-enumerated, + the
   M1 stamp/byte WARN (F7, F4, F15 doctor arm).
7. **Owner bundle 0021** (DDL revoke) — authored, deploy-plan-terminal, excluded from every
   `owner-ddl apply` route (`LatestOwnerBundleVersion` stays 20). **The F16b PRODUCTION-phase test
   + the forced-self-heal pgtest land here (M4)**, tested in two-role pgtest (F6, F12, F16b);
   activation is the operator choreography (§4.3).

Each phase additive and reversible. Self-record before enforce; detection before
mutation-relocation; the verb + finalizer + full-transcript verifier + ownership reconcile +
immutable-transcript resume + the non-revoke filter + the M3 activation guard + **the M5-correct
owner-watermark gate (fresh-DB serve preserved)** all proven before the boot path leans on them.

---

## 7. Boundary & explicit deferrals (carry-forward)

- **In P4:** the deployer verb; the deploy plan + `deploy_cursor` (incl. `finalizing`) + the
  immutable `deploy_plan` transcript (BC-N1) + per-step + terminal receipt; the idempotent
  finalizer (C1) with the `VerifyStoredTranscript` pre-finalizer gate (M1); the C3 ownership
  reconcile with 0021 sequenced last; the single non-revoke `owner-ddl apply` filter + embed/listing
  split (M2); lifting `ApplyMigrations` out of serve-boot (flagged); the activation interlock + the
  universal pre-revoke cursor edge (BC-N2) + the M3 complete-cursor config gate + no-revoke
  comparison + forward-watermark rule + **the M5-correct owner-watermark dimension (fresh
  `applied_owner == 0` serves; `1..19` halts)**; serving-role create-DDL revocation (bundle 0021,
  deploy-terminal).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`, `rehearsal_receipt.v1`,
  fidelity tiering, full-data clone, expand/contract reshape, lock-budget guardrail. Full capability
  revocation (owner re-owns runtime tables) is a named follow-up beyond P4 (§4.1).
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the single writer, no
  hosted services / replicas / external persistence.

---

## 8. Open tensions handed to the falsifiers (attack these hardest)

- **M5 / F18 / F18a — the `applied_owner` dimension split.** Is the `applied_owner == 0` cell
  genuinely a SERVE (not a halt) under `CheckOwnerBundleWatermark` (`owner.go:145`, the `applied ==
  0` return BEFORE the `:148-150` shortfall)? Is the `0` column's A-gate outcome truly IDENTICAL to
  the `==20` column (because A does not read `applied_owner`), so cell 1/`==0` serves exactly as
  cell 1/`==20`? Does F18 assert BOTH the `applied_owner == 0` serve cell AND the `1..19` halt cell
  explicitly? Is there ANY remaining `applied_owner` value not covered by the four disjoint buckets
  `{0, 1..19, ==20, >=21}` (they cover all of ℕ)? Is the relabel correct — is the genuine fresh
  no-authority DB `applied_owner == 0`, not `==20`?
- **M5 — does the split weaken any carry-forward?** Does serving `applied_owner == 0` ever let a
  revoke-embedding binary through (no — cell 2/`==0` still halts `awaiting_deploy_config` at A0,
  barrier a)? Does it regress the BC-N2 `applied_owner == 20` edge (no — the `==20` column's BC-N2
  cells 5/7/9/11 are verbatim)? Does it advance `Required` or alter the watermark (no — `owner.go:35`
  stays 20; the `>=21` forward rule is unchanged)?
- **M3 / F17 / F18 — the hoisted config gate vs. the no-revoke comparison.** Is the
  `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` halt truly the FIRST A predicate?
  Is the no-revoke `complete` comparison genuinely a PURE READ that cannot mutate or self-record?
  Is there any §3.5 cell where the legacy `:399` writer fires over a transcript carrying a pending
  change, or for a revoke-embedding binary?
- **M4 / F16a — the synthetic seam.** Does the synthetic-list / injectable-FS hook genuinely
  exercise `OwnerDDLApplyBundles`/`isNonRevokeBundle`/both apply loops without depending on a real
  0021 file? Does F16b's forced self-heal truly reach `ReapplyAllOwnerBundles` via
  `isCrossBundleDependencyError`?
- **M1 / M2 / BC-N1 / BC-N2 / C1 / C2 / C3 carry-forward (re-attack the v6 tensions §8).** Re-confirm
  none regressed under the M5 split.

---

## 9. Anchor table (re-verified against current `main`, HEAD `9ceeb288`)

> `git diff --stat 2e482077 HEAD -- go/pkg/db/` is **empty** (the only `go/` commits since the v6
> anchor touched `go/pkg/mutations/` + `go/pkg/reads/`, NOT any `db/` file), so every v6 source
> anchor in `go/pkg/db/` holds unchanged. The M5 owner-watermark sites and the boot order were
> re-confirmed line-by-line this cycle (spot-checks below).

| Target | Anchor (re-confirmed `9ceeb288`) |
| --- | --- |
| **the M5 owner-watermark gate W (the `applied_owner` dimension)** | `owner.go:124-154` `CheckOwnerBundleWatermark`; **`:145` `if applied == 0 { return nil }` (the fresh/no-authority SERVE exception, returns BEFORE the shortfall)**; **`:148-150` `if applied < RequiredOwnerBundleVersion { return &AwaitingOwnerDDLError{...} }` (the `1..19` shortfall halt)**; `:151-153` tolerate-forward / barrier (b); the comment block `:116-123` ("a 0-watermark database … is treated as the bootstrap/single-role case and NOT halted. Only a database that HAS an authority schema (applied >= 1) but lags the required frontier is a genuine shortfall") + the inline comment `:140-143`. |
| **the fresh-DB representation (M5)** | `OwnerBundleVersion` `owner.go:226-245`; **`:234-235` `if present != "true" { return 0, nil }` (returns 0 when `to_regclass('striatumd.owner_bundle_meta')` is null)**; `owner_pg_test.go:19` (`if v, err := db.OwnerBundleVersion(...); ... v != 0` — `pre-apply version = %d … want 0`, a fresh migrated DB starts at version 0). |
| **serve-boot mutation site + the W→A→`:353` boot order** | `connection.go:332` `ConnectAndMigrate`; **watermark W `:349`**; **`ApplyMigrations` `:353`**; drift gate `:376-383` (refuse-mode halt); **shadow log + fall-through `:384-393`**; **`RecordSchemaFingerprint` `:399` ("the only writer of schema_state")** — re-read this cycle; W returns nil for `applied == 0`/`==20`/`>=21`-revoke and halts for `1..19`, then A precedes `:353`. |
| boot fork / `--migrate` / typed-halt mapping | `authority_bootstrap.go:181,193`; `main.go:76`; typed halts `:208-227` (add `awaiting_deploy`/`awaiting_deploy_config`/`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` arms; `awaiting_owner_ddl` already mapped) |
| deployer command site | `daemon.go:67-81` dispatch (add `deploy`); `runDaemonOwnerDDL` `:90` (admin DSN `:115`); `runDaemonMigrate` `:167`; `EnvDaemonAdminDBURL` `:24` |
| runtime apply engine to reuse/wrap (C3) + M1 oracles | `migrations.go:121` `ApplyMigrations`; `applyOne` `:304-355`; `verifyRecordedHashTx` `:274` (reads `schema_migrations.sha256` `:275` — M1 runtime DB-stamp oracle); `MigrationSHASet` `:197` (M1 runtime byte oracle); `//go:embed sql/*.sql` `:21`; lock `:18` |
| owner-bundle engine / watermark + forward-tolerance (C2) / 0021 exclusion (M2/C3) | `owner.go:265-303` `ApplyOwnerBundles` (slice `:269` → `OwnerDDLApplyBundles()`); `:309-322` `applyPendingOwnerBundles` (`<= current` `:312`, in-loop guard); `:332-351` `ReapplyAllOwnerBundles` (nil-fallback `:336-342`, in-loop guard); `:367-374` `isCrossBundleDependencyError`; `:511-541` `applyOneOwnerBundle` (`owner_bundle_meta.sha256` stamp `:529` — M1 owner DB-stamp oracle); `OwnerBundles()` `:197-224` (FULL loader); `OwnerBundle.SHA256()` `:191` (M1 owner byte oracle); `LatestOwnerBundleVersion = 20` `:23` (**stays 20**); `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` `:35` (= 20, **stays 20**); `revokeEmbedded` = 0021 in `ownerBundleFS` (`//go:embed` `:156`) |
| pure read consuming the FULL owner-bundle loader (must keep seeing 0021) | `owner_runtime_ownership.go:51-52` `RuntimeOwnedTablesAlterable()` → `OwnerBundles()` |
| current owner-bundle frontier (NOT the revoke) | `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql` (the highest non-revoke bundle; the DDL-revoke is the NEW 0021) |
| C3 prerequisite the revoke-last fix relies on | `0018_runtime_table_ownership_transfer.sql:98` (`GRANT CREATE … TO striatumd_rw` before the transfer loop) — 0018 in the non-revoke prefix, CREATE held when runtime reconciles run |
| fingerprint contract (M1 mechanism + the M3 A3 pure-read comparison) | `schema_drift.go:83-100` `ExpectedFingerprint` (hashes EMBEDDED FILE BYTES — pure); `:105-137` `composeFingerprint` (reused for `plan_hash`); `:145-161` `LiveFingerprint` (reads the self-recorded singleton — NO recompute; pure); `:171-195` `RecordSchemaFingerprint` (the gated write); `:254-274` `CheckSchemaDrift`; `EnvSchemaDriftRefuse` `:28` |
| `deploy_cursor` + `deploy_plan` table model | `0043_schema_state.sql:39-44` (singleton CHECK), `:46-52` (`striatumd_rw` GRANT `DO` block) — model for both new 0044 tables |
| per-step / terminal receipt SD fn | `0001_authority_phase0.sql:152` (`append_audit_row`), `:238` (`GRANT EXECUTE … TO striatumd_rw`) |
| doctor block model | `go/pkg/reads/doctor_schema_drift.go` (`schemaDriftDoctorBlock`; `ExpectedFingerprint` call) |
| FMA-007 self-heal test to extend (M2 / F16b) | `owner_pg_test.go:219` `TestReapplyAllOwnerBundlesIsOrderedAndIdempotent` (calls `ReapplyAllOwnerBundles(ctx, runner, nil, "test")`) |
| two-role fixture for F12 / F16b / F17 / F18 / F18a | `go/pkg/pgtest/two_role.go:130` `ApplyOwnerBundles` (non-superuser two-role bootstrap) |

<sub>Holder artifact (v7 revision) for the RFC 0142 P4 falsification-gate design run. Resolves
cycle-1 (design-v6) finding **M5** (the §3.5/F18 owner-watermark dimension SPLIT into
`{0/no authority, 1..19 authority shortfall, ==20, >=21}`: the fresh `applied_owner == 0`
no-transcript/no-revoke/flag-off cell SERVES-legacy — `CheckOwnerBundleWatermark` returns nil for
`applied == 0` at `owner.go:145`, NO transcript so Universal Invariant B is not in scope — while
`1 <= applied_owner < 20` retains `awaiting_owner_ddl`; the 64-cell §3.5 table re-proves Invariant
B in every cell and proves the fresh-DB / inert-landing cells STILL SERVE; cell `==20` relabeled the
inert-landing cell; F18 asserts BOTH the `applied_owner == 0` serve cell AND the `1..19` halt cell,
F18a pins the fresh-DB serve) and carries **M3 + M4 + M1 + M2 + BC-N1 + BC-N2 + C1 + C2 + C3 forward
verbatim and unregressed** (re-verified against current `main` HEAD `9ceeb288`; `git diff --stat
2e482077 HEAD -- go/pkg/db/` empty). The cycle-1 falsifiers (decoupling-boundary/self-record lens;
owner-ddl/test-staging lens) re-attack §3.5 (the split table), §4.5 Invariant B, F18/F18a, and the
tensions in §8; the adjudicator's collaboration ledger — not falsifier completion — decides whether
the gate clears.</sub>

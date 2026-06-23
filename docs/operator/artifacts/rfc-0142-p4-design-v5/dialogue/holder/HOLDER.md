# HOLDER (revised, v5) — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: holder-author-001

> **This is the FIFTH REVISION (v5).** Design-v1 returned `needs_revision` with C1/C2/C3.
> v2 resolved C1 + C2. v3 resolved C3 (ownership transfer, revoke-last) and closed the
> immediate N1 hole, then returned `needs_revision` on BC-N1 + BC-N2. v4
> (`rfc-0142-p4-design-v4`) **resolved BOTH BC-N1 and BC-N2** — both v4 falsifiers
> explicitly conceded the v3 reproducers no longer reproduce, and C1/C2/C3 carried forward
> intact — BUT returned `needs_revision` again on **two independent NEW material
> challenges**, each source-verified against current `main`, each squarely in a
> correctness/safety core:
>
> - **M1** (falsifier_1, atomicity/fingerprint lens) — the v4 transcript/binary
>   verification rule verifies ONLY not-yet-applied steps, so a resume binary whose
>   **already-applied** step bytes differ (`B45 ≠ A45`) passes the not-yet-applied check, is
>   classified resume-off-stored-transcript rather than `deploy_plan_binary_mismatch`,
>   completes the plan, and the C1 finalizer self-records the running binary's
>   `ExpectedFingerprint()` (which hashes EMBEDDED FILE BYTES) and marks `complete` —
>   converting a hybrid A-applied/B-expected deploy into an apparently in-sync B deploy
>   (silent schema corruption). The v4 holder's own §8 conceded the already-applied check
>   does not fire.
> - **M2** (falsifier_2, owner-ddl/migration-safety lens) — the v4 "0021 unreachable via
>   `owner-ddl apply`" guarantee binds ONLY `applyPendingOwnerBundles`; the FMA-007 self-heal
>   branch `ReapplyAllOwnerBundles` (reached from `ApplyOwnerBundles` on a cross-bundle
>   dependency error) re-runs EVERY loaded bundle regardless of recorded version, and since
>   the activation binary's `OwnerBundles()` MUST surface 0021 for `revokeEmbedded` detection,
>   a cross-bundle dependency repair can commit `REVOKE CREATE ON SCHEMA striatumd FROM
>   striatumd_rw` early — recreating the #512-class CREATE lockout the revoke-last mechanism
>   exists to prevent.
>
> This spec **starts from the v4 `HOLDER.md`** (a required context doc) and is a **proper
> revision**: it **resolves M1 and M2** per the prescribed fixes in the v4 collaboration
> ledger §4 ("What the revision must fix") and `SEED.md`, does the **proactive hardening
> audit**, and **carries BC-N1, BC-N2, C1, C2, and C3 forward unregressed**. §0.1 below is
> the auditable resolution map; the cycle-5 falsifiers should verify each item there, then
> re-attack §3.4a (the full-transcript verifier), §3.2a (the non-revoke filter + embed/listing
> split), §5 (esp. the new **F15**, **F16**, the extended **F11/F12/F14/G-revoke-last**), the
> two universal invariants in §4.5, and the tensions in §8.
>
> It is a falsifiable implementation spec for RFC 0142 **P4 only** — the one-shot deployer that
> lifts schema mutation out of serve-boot — that `rfc-0142-p4-build` can execute contract-first
> (TDD). RFC 0142 is accepted (D258); this does NOT re-open the five-layer design. Primary
> source: `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (esp. Layer 3,
> the Phasing P4 row, Open Questions 3 + 4). **Every anchor below is re-verified against
> current `main` (HEAD `2e482077`).** The two operator commits since the v4 anchor (`5c5cb36a`)
> touched only docs/workflow — `git diff --stat 5c5cb36a HEAD -- go/` is empty — so every v4
> source anchor holds; the M1 and M2 cores (`schema_drift.go`, `owner.go`, `migrations.go`,
> `connection.go`, `daemon.go`) are re-confirmed line-by-line in §9.

## 0. Root reframe held

Schema mutation must stop being an **implicit side effect of the serving process's restart**
and become an **explicit, ordered, resumable, provenance-tracked operation owned by a
dedicated deployer**. Then the serving daemon holds **zero create-DDL on the serving path**,
"restart force-commits a half-applied deploy" becomes structurally impossible, and a bad
migration can never wedge the single writer on boot. Everything below serves that one move;
nothing pulls in P5 (rehearse / expand-contract / fidelity tiering / clone = Q1/Q2).

The v5 revision adds two safety properties on top of the v4 spec, each closing a
silent-corruption / lockout escape the v4 falsifiers found, **without re-opening any resolved
mechanism**:

- **M1 — no fingerprint is self-recorded unless the WHOLE stored transcript byte-matches the
  running binary** (and the already-applied DB stamps match the transcript). The v4 check
  covered only not-yet-applied steps; v5 covers every step, on every resume *and* before the
  finalizer.
- **M2 — the DDL-revoke bundle 0021 is excluded from EVERY `owner-ddl apply` route, including
  the FMA-007 self-heal reapply**, via a single non-revoke filter and a split between "the
  binary embeds 0021" (for `revokeEmbedded`/`ExpectedFingerprint`/the deploy plan) and
  "`owner-ddl apply` iterates 0021" (never).

---

## 0.1 Addressing the design-v4 findings (the gating requirement)

The two binding v4 findings (M1, M2) are each resolved by a concrete sub-protocol bound to
named code sites — not a claim. BC-N1, BC-N2, C1, C2, and C3 are carried forward **verbatim**
from the v4 HOLDER and re-confirmed unregressed. The "where" column names the section that
specifies the fix and the named test that would refute it.

| Finding | v4 status | v5 disposition | Where / refuting test |
| --- | --- | --- | --- |
| **M1** transcript/binary verification checks ONLY not-yet-applied steps, so a hybrid (already-applied bytes differ) deploy self-records as in-sync | **OPEN** (falsifier_1; BC-N1/Q3/R4 core) | **RESOLVED: a single `VerifyStoredTranscript(plan_hash)` checks the ENTIRE stored transcript — every step, already-applied AND not-yet-applied — `sha256` against the running binary's embedded bytes; ANY mismatch ⇒ `deploy_plan_binary_mismatch` (typed halt, DB-untouched, apply nothing). For already-applied entries it ALSO verifies the DATABASE STAMPS (`schema_migrations.sha256` for runtime steps, `owner_bundle_meta.sha256` for owner steps) match the stored transcript; a divergence ⇒ `deploy_plan_db_stamp_mismatch` (refuse-to-finalize, never a silent resume). The SAME `VerifyStoredTranscript` runs BEFORE the C1 finalizer writes `schema_state` or advances `finalizing → complete`, so a binary that disagrees with an already-applied step can NEVER self-record its `ExpectedFingerprint()` as in-sync. The v4 finalizer's byte-derived fingerprint write (`schema_drift.go:83-100`/`:171-195`) is now gated by a full-transcript byte+stamp check.** | §3.4a (`VerifyStoredTranscript`), §1.3 (rows re-keyed to full-transcript verification), §3.4 (pre-finalizer check), §4.5 (universal fingerprint invariant) / **`T-deploy-resume-already-applied-byte-mismatch-refuses` (F15, new) + the symmetric owner-step case** |
| **M2** "0021 unreachable via `owner-ddl apply`" binds only `applyPendingOwnerBundles`; the FMA-007 self-heal reapply can commit the revoke early | **OPEN** (falsifier_2; C3/R3 core) | **RESOLVED: a single non-revoke filter `isNonRevokeBundle(b) = b.Version < DDLRevokeOwnerBundleVersion (21)` and a split loader `OwnerDDLApplyBundles()` = `filter(OwnerBundles(), isNonRevokeBundle)` are the ONLY bundle slice every `owner-ddl apply` route iterates: `ApplyOwnerBundles` loads `OwnerDDLApplyBundles()` (not `OwnerBundles()`); `applyPendingOwnerBundles` AND `ReapplyAllOwnerBundles` (the FMA-007 self-heal) additionally guard `isNonRevokeBundle` in-loop (defense-in-depth against a hand-passed slice); `ReapplyAllOwnerBundles`'s `bundles == nil` fallback loads `OwnerDDLApplyBundles()`. `OwnerBundles()` (full, includes 0021) is kept ONLY for `revokeEmbedded` detection, `ExpectedFingerprint`, the deploy plan's `BuildPlan`, and the pure read `RuntimeOwnedTablesAlterable()` — so "the binary embeds 0021" does NOT imply "`owner-ddl apply` iterates 0021". 0021's REVOKE can ONLY ever be committed as the terminal `deploy` step (via `applyOneOwnerBundle`), NEVER via a self-heal reapply. A build-time test asserts no `owner-ddl apply` route can reach a bundle `>= DDLRevokeOwnerBundleVersion`.** | §3.2a (non-revoke filter + embed/listing split), §4.4 (no early revoke), §4.5 (universal apply-path invariant) / **`T-deploy-revoke-excluded-from-reapply-self-heal` (F16, new) + extended F12 / `G-revoke-last` owner-ddl side-path** |
| **BC-N1** immutable plan identity across resume (moving-frontier fix) | **RESOLVED** (v4; both falsifiers concede the v3 reproducer is closed) | **CARRIED FORWARD VERBATIM, unregressed.** The runtime-owned `deploy_plan` table (migration 0044) storing the IMMUTABLE ordered transcript keyed by `plan_hash`, materialized in the same tx that sets `deploy_cursor → in_progress(0)` BEFORE step 0; resume loads `deploy_plan[cursor.plan_hash]` and never recomputes `BuildPlan` over the moved frontier; §1.3's stored-transcript reclassification; the transcript-enumerated doctor; F14. **M1 EXTENDS the binary-verification clause of this fix to already-applied steps + the pre-finalizer check; it does NOT re-open the moving-frontier mechanism.** | §1.2, §1.3, §3.2, §3.4 / **`T-deploy-plan-hash-resume-after-step` (F14, extended for M1)** |
| **BC-N2** universal pre-revoke serve edge | **RESOLVED** (v4; both falsifiers concede, decline as blocker) | **CARRIED FORWARD VERBATIM, unregressed.** `CheckDeployActivation` no longer gated on `revokeEmbedded`; every deployer-aware binary reads `deploy_cursor` after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations`/`RecordSchemaFingerprint` in both boot paths and halts `awaiting_deploy` DB-untouched on a non-`complete` cursor at `applied_owner == 20`; F11(e)/(f) + extended `G-old-binary-refuse`. | §3.3a, §4.2 / **`F11` (extended)** |
| **C1** finalization boundary | **INTACT** (v2/v3/v4; both falsifiers concede) | **CARRIED FORWARD VERBATIM, unregressed.** The `finalizing` state, the idempotent finalizer (guarded `complete` receipt → `schema_state` UPSERT → advance `finalizing → complete` LAST), the §1.3 resumable-finalization row, the operator pin (no single finalization tx spans owner+runtime), F10. **M1's fix ADDS a pre-finalizer `VerifyStoredTranscript` gate before the `schema_state` write; it does NOT change the finalizer's boundary semantics.** | §1.2, §1.3, §3.4 / **`T-deploy-resume-finalization-crash` (F10) + F15 pre-finalizer arm** |
| **C2** fail-closed activation edge | **INTACT** (v2/v3/v4) | **CARRIED FORWARD VERBATIM, unregressed.** `CheckDeployActivation` before `ApplyMigrations`, typed `awaiting_deploy`/`awaiting_deploy_config` halts, forward-watermark rule fires at `applied >= 21`, `RequiredOwnerBundleVersion` KEPT at its current-`main` value **20** (NOT advanced to the revoke ordinal 21), F11. | §3.3a, §4.2, §4.3 / **`T-deploy-revoke-activation-ordering` (F11)** |
| **C3** ownership transfer (revoke-last) | **INTACT design** (v3/v4; both falsifiers concede) | **CARRIED FORWARD VERBATIM.** The DDL-revoke bundle 0021 special-cased out of the owner prefix, applied as the TERMINAL plan step after every runtime ownership reconcile (each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE; the committed steady state denies CREATE), `DDLRevokeOwnerBundleVersion = 21`, F12 / `G-revoke-last`. **M2 binds the EXECUTABLE exclusion across the self-heal branch of `owner-ddl apply`; it does NOT change the deploy-plan revoke-last mechanism.** | §3.2, §3.2a, §3.3b, §4.4 / **`T-deploy-runtime-object-ownership` (F12, extended) + `G-revoke-last` (extended)** |

**Carried forward unchanged (v1–v4 ledgers — do not re-litigate):** Q4 (plain verb + three
seams, §2); the per-step Q3-A/Q3-B mid-step resumability **body** + step taxonomy (§1.1, the
`in_progress`/`step_committed` body of §1.2 — M1 changes only the *verification* added around
the stored transcript and finalizer, not the per-step schema/cursor body); the shadow-first
decoupling shape (default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the
P2 watermark interlock + P3 drift gate, the embed-FS-derived plan, the hash-chained receipt,
`doctor schema_deploy_unrecorded`). **The v5 revision touches only the full-transcript
verification (M1) and the non-revoke owner-ddl filter + embed/listing split (M2).**

---

## 1. Decision Q3 — How atomic is "atomic"? (the hard correctness core)

**Claim (Q3-DECISION).** The **per-step-atomic + resumable-cursor** contract is sufficient for
*every* owner+runtime interleaving P4 ships, **provided**: (a) plan identity is an **immutable
durable fact materialized before step 0** and resume reads it (BC-N1); (b) the two per-step
sub-rules (Q3-A/Q3-B) hold for both the schema side effect AND the per-step receipt, keyed on
the **stored transcript's** `(plan_hash, step_index)`; (c) the finalization boundary is closed
by a distinct `finalizing` state + an idempotent finalizer (C1); (d) **no boot — including a
no-revoke deployer-aware boot — can serve while the cursor is non-`complete`** (BC-N2); **and
(e) NEW in v5 (M1) — on every resume AND before the finalizer self-records, the ENTIRE stored
transcript (every step, already-applied and not) byte-matches the running binary and the
already-applied DB stamps match the transcript; any mismatch halts the deploy DB-untouched and
NO fingerprint is written.** No step needs a cross-step transaction; no finalization
transaction spans the owner+runtime connections; no per-step provenance write is left in a
crash window the resume cannot reconcile to exactly-one receipt against a stable key; no resume
recomputes the plan from a moved frontier; **and no resume or finalizer ever trusts an
already-applied stored step it cannot byte-verify against the running binary and the DB stamp.**

- **Q3-A — transactional step (carry-forward body; receipt keyed off the stored transcript).**
  If a step's DDL can run inside a transaction, the **cursor advance, the C3 ownership reconcile,
  the version stamps, AND the per-step receipt commit in the SAME transaction as the step** (over
  the owner connection — §3.3b). Crash → the whole transaction rolls back → cursor unchanged, no
  receipt → resume re-enters the step cleanly. `step_committed(k)` is durable iff its receipt is
  durable, and `k` is the index in the **stored** transcript, stable across the moved frontier.
- **Q3-B — idempotent-resume reconciler where the step is NOT transactional (carry-forward
  body).** `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, and the handful of
  auto-committing `ALTER`s cannot share a transaction with the cursor or the receipt. The
  deployer writes an `in_progress(k)` marker BEFORE the step (where `k` is the stored-transcript
  index), runs the step in its idempotent / self-reconciling form, then in one owner-connection
  reconcile transaction appends exactly one receipt keyed on the stored `(plan_hash, step_index)`
  and writes `step_committed(k)`.
- **Q3-V — full-transcript verification (NEW in v5, the M1 resolution).** On every deploy entry
  that finds an incomplete cursor, and again at the top of the finalizer, the deployer runs
  `VerifyStoredTranscript(plan_hash)` (§3.4a) **before applying or finalizing anything**. This is
  the resume-time and finalize-time gate that makes Q3-A/Q3-B safe under a **changed binary**: a
  resume binary whose embedded bytes disagree with any stored step (applied or pending), or a DB
  whose already-applied stamp disagrees with the transcript, is forced to a typed halt with the
  database untouched and no fingerprint written.

### 1.1 Step taxonomy (the closed set P4 ships) — unchanged from v1–v4 (re-anchored to current `main`)

| Class | Examples | Transactional? | Protocol | Idempotent form |
| --- | --- | --- | --- | --- |
| **T-DDL** runtime | `CREATE TABLE`, additive `ALTER … ADD COLUMN`, `CREATE INDEX` (non-concurrent), the new runtime migrations | Yes | **Q3-A** (step + ownership-reconcile + version stamps + cursor + **receipt** in one tx) | `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`; re-run is a no-op |
| **T-DDL** owner | owner-bundle DDL, **incl. the terminal DDL-revoke bundle 0021** | Yes (each bundle applies in one tx that stamps `owner_bundle_meta` last — `owner.go:511-541`) | **Q3-A**, reusing the per-bundle transaction; cursor advance + **receipt** join that tx | bundles idempotent (`IF NOT EXISTS` + `ON CONFLICT (version) DO NOTHING`, `owner.go:528-535`) |
| **NT-DDL** | `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, `DROP INDEX CONCURRENTLY` | No | **Q3-B** (pre-marker → step → reconcile: keyed receipt + post-marker) | `CREATE INDEX CONCURRENTLY IF NOT EXISTS`; resume reconciler drops a left-behind `INVALID` index then recreates; `ADD VALUE … IF NOT EXISTS` |
| **DML-backfill** (bounded in P4; full online backfill is P5) | a one-shot data normalize a runtime migration carries | Yes if small; batched otherwise | Q3-A small / Q3-B batched | `WHERE <not-yet-done>` predicate; re-run resumes from the unprocessed tail |

> The only genuinely hard per-step corner is NT-DDL: a half-built `CREATE INDEX CONCURRENTLY`
> leaves `pg_index.indisvalid = false`; the fingerprint MUST classify that as "incomplete,
> resume", never "unknown drift, panic" — see §1.3. NT-DDL never creates a new ownable object
> whose owner is wrong (a CONCURRENTLY index inherits its table's owner; `ALTER TYPE … ADD VALUE`
> creates no relation), so the C3 ownership reconciliation (§3.3b) is needed only on transactional
> object-creating steps. The terminal DDL-revoke bundle 0021 is a T-DDL owner step (the existing
> per-bundle transaction, `owner.go:511-541`), so its receipt commits atomically.

### 1.2 `deploy_cursor` + the immutable `deploy_plan` transcript (BC-N1) and crash-resume semantics (C1)

Both are **new runtime-owned tables**, created by runtime migration **0044** (modeled
byte-for-byte on `0043_schema_state.sql:39-52`: the singleton `CHECK (id='singleton')` PK, the
`striatumd_rw` GRANT `DO` block at `0043:46-52`). Migration 0044 is additive, runtime-owned,
carries no owner DDL (passes `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`,
`migrations_test.go:643`).

**`deploy_cursor` (singleton) — the live position:**

```
deploy_cursor(
  id            text PK CHECK (id='singleton'),
  plan_hash     text NOT NULL,         -- FK-by-value into deploy_plan.plan_hash
  state         text NOT NULL,         -- enum below (CHECK constraint)
  step_index    int  NOT NULL,         -- 0-based index INTO the stored transcript (read, not derived)
  step_id       text NOT NULL,         -- e.g. "runtime:0045" / "owner:0021" (denormalized for fast classification)
  updated_at    timestamptz NOT NULL DEFAULT now()
)
```

**`deploy_plan` (append-only, keyed by `plan_hash`) — the IMMUTABLE transcript (BC-N1):**

```
deploy_plan(
  plan_hash              text PK,        -- sha256 of the canonical (base + ordered transcript)
  base_owner_version     int  NOT NULL,
  base_runtime_version   int  NOT NULL,
  target_owner_version   int  NOT NULL,  -- includes the revoke's ordinal 21 when present
  target_runtime_version int  NOT NULL,
  revoke_step_index      int,            -- step_index of the terminal revoke, or NULL when no revoke
  step_count             int  NOT NULL,
  steps                  jsonb NOT NULL, -- ordered [{step_index, step_id, role, sha256, transactional}], revoke last
  created_at             timestamptz NOT NULL DEFAULT now()
)
-- INSERT-once, never UPDATE: a plan_hash row is immutable. A re-run with the same plan_hash
-- finds its row (INSERT … ON CONFLICT (plan_hash) DO NOTHING) and READS it.
```

`state ∈ { idle, in_progress, step_committed, finalizing, complete, aborted }`. The
**`finalizing` state (C1)** is preserved. Lifecycle for a plan of N steps — where the terminal
step N-1 is the DDL-revoke bundle 0021 whenever it is in the plan (C3) — with the BC-N1
materialization as the first durable write:

```
materialize(plan_hash, transcript) + idle─▶in_progress(0) ─▶ step_committed(0) ─▶ … ─▶ step_committed(N-1=0021) ─▶ finalizing ─▶ complete
   [deploy_plan row + cursor=in_progress(0) in ONE tx, BEFORE step 0 mutates the frontier]      │
                                                                                                 └─(operator abort / fatal non-resumable error)──▶ aborted
```

- **Materialization (BC-N1, carry-forward).** After the substrate-ensure preamble (§3.2)
  guarantees `deploy_plan`/`deploy_cursor` exist and observes the base frontiers, the deployer
  computes `BuildPlan(base_owner, base_runtime)` once, derives `plan_hash`, and in one transaction
  INSERTs the immutable `deploy_plan` row and sets `deploy_cursor → in_progress(0)`. This commits
  before step 0's frontier-advancing DDL.
- **Resume verification (NEW in v5, M1).** On every resume, immediately after loading
  `deploy_plan[cursor.plan_hash]` and BEFORE applying or finalizing any step, the deployer runs
  `VerifyStoredTranscript(plan_hash)` (§3.4a) over the WHOLE transcript. Only if it returns nil
  does the deployer resume at `cursor.step_index`; otherwise it halts with the typed mismatch and
  applies nothing.
- **Q3-A / Q3-B steps** (carry-forward body) — as v4; receipt keyed on the stored transcript.
- **Finalization (C1 — carry-forward, with the M1 pre-finalizer gate added).** When the terminal
  step is `step_committed`, the deployer advances the cursor to **`finalizing`** in one commit,
  then runs the **idempotent finalizer** (§3.4): **(0) NEW (M1) — run
  `VerifyStoredTranscript(plan_hash)` and abort the finalizer with the typed mismatch (writing
  NOTHING) if it fails;** (1) append the guarded **`complete` deploy receipt**; (2)
  **`RecordSchemaFingerprint`** (`schema_drift.go:171-195`) UPSERT into `schema_state`; (3) advance
  `finalizing → complete` **last**. Because step (0) gates steps (1)–(3) on a full-transcript
  byte+stamp match, a `complete` cursor now proves the deploy is fully finalized **by a binary
  whose embedded bytes match every applied step**, not merely that the receipt+fingerprint are
  durable.

### 1.3 The load-bearing disambiguation (C1 + BC-N1 + BC-N2 + M1)

`deploy_cursor` is the **second signal** alongside the drift gate's `LiveFingerprint ==
ExpectedFingerprint`. The `finalizing` row (C1) and the `state == complete` tightening are
preserved verbatim from v4; the stored-transcript rows (BC-N1) and the universal-edge note
(BC-N2) are preserved; **the binary/stamp-verification rows are now keyed on the FULL transcript
(M1), and a new `deploy_plan_db_stamp_mismatch` row is added.**

| `deploy_cursor.state` | `plan_hash` vs the **stored transcript** | classification | action |
| --- | --- | --- | --- |
| `complete` | `deploy_plan[plan_hash]` present; full transcript byte-matches binary; `== ExpectedFingerprint` | **in sync** | serve |
| `in_progress` / `step_committed` | present; **the FULL transcript (every step, applied AND pending) byte-matches the binary** AND already-applied DB stamps match the transcript | **incomplete, resume off the STORED transcript** (BC-N1) | refuse-to-serve `awaiting_deploy`; `deploy` resumes at `cursor.step_index` — never recomputes `BuildPlan` |
| `in_progress` / `step_committed` | present but the cursor's `plan_hash` ≠ the binary's freshly-computed pending plan (frontier advanced under the interrupted deploy) | **incomplete, resume with the STORED plan** (BC-N1) | refuse-to-serve `awaiting_deploy`; resume off the stored transcript by `cursor.plan_hash` — divergence from the pending plan is expected/recoverable, NOT drift |
| `finalizing` | present; **full transcript byte-matches binary** (else falls to the mismatch rows) | **resumable finalization** (C1) | refuse-to-serve `awaiting_deploy`; re-run runs the idempotent finalizer (which itself re-verifies, §3.4 step 0); NEVER serve, NEVER genuine-drift |
| `in_progress` / `step_committed` / `finalizing`, with `deploy_plan[plan_hash]` ABSENT, **OR ANY stored step (already-applied OR not-yet-applied) `sha256` ≠ the binary's embedded bytes** | n/a / **full-transcript mismatch (M1)** | **`deploy_plan_binary_mismatch`** (the resuming binary is not the one that materialized the plan) | refuse-to-serve; do NOT auto-resume; apply NOTHING; operator inspects (resume with the original deployer binary, or `abort` + redeploy) |
| `in_progress` / `step_committed` / `finalizing`, transcript byte-matches the binary, **but an already-applied step's DB stamp (`schema_migrations.sha256` / `owner_bundle_meta.sha256`) ≠ the stored transcript `sha256` (M1)** | already-applied stamp mismatch | **`deploy_plan_db_stamp_mismatch`** (the DB does not reflect what the transcript says was applied) | refuse-to-serve; do NOT auto-resume; do NOT finalize; apply NOTHING; operator inspects |
| absent `deploy_cursor` table / `state == idle` | n/a | **no deploy in flight** | **legacy path (flag OFF) + no-revoke binary:** serve (auto-apply); **decoupled (flag ON) OR revoke-embedding binary:** refuse-to-serve `awaiting_deploy`/`awaiting_deploy_config` (C2 — §3.3a) |
| `complete` but `plan_hash` ≠ expected, OR `aborted` | foreign / n/a | **genuine drift / aborted** | refuse-to-serve (existing `schema_drift` halt / `awaiting_deploy`) |
| `complete`, `plan_hash` == expected, fingerprint mismatch | == expected | **genuine post-deploy drift** | refuse-to-serve (`schema_drift`) — unambiguous because `complete` ⇒ fingerprint was durable AND the full transcript byte-matched at finalize |

Four load-bearing properties this table now enforces:

- **(BC-N1) Plan identity is a stored fact, not a recomputation.** The classifier reads
  `deploy_plan[cursor.plan_hash]` and resumes off it; a frontier that moved under the deploy does
  not change the stored identity.
- **(M1) A resume binary that disagrees with ANY stored step — already-applied or pending — is
  forced to `deploy_plan_binary_mismatch`, and a DB whose applied stamp disagrees with the
  transcript is forced to `deploy_plan_db_stamp_mismatch`.** Neither auto-resumes, neither
  finalizes, neither writes a fingerprint. The v4 gap — already-applied bytes unchecked, so a
  hybrid self-records as in-sync — cannot occur: the verification set is the WHOLE transcript, and
  the finalizer re-runs it (§3.4 step 0) before any `schema_state` write.
- **(C1) The `complete`-with-missing-receipt window cannot occur** (`complete` is the final write),
  and because 0021 is the terminal plan step (§3.2), `complete` additionally implies 0021 is
  applied — so an in-sync `complete` never serves a database where `striatumd_rw` still holds
  CREATE on the serving path.
- **(BC-N2) `incomplete cursor never serves` holds for EVERY deployer-aware binary** — the rows
  above are consulted by the universal `CheckDeployActivation` edge (§3.3a) on every boot,
  no-revoke binaries included.

---

## 2. Decision Q4 — Is a deploy itself a Striatum run? (carry-forward, sound)

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape door is held
open by three already-present seams.** Unchanged from v1–v4; neither falsifier contested it.

**Why not a run now — the bootstrapping paradox is dispositive.** A dogfooded run needs the
daemon up and its schema present to drive `runs`/`jobs`/`sessions`/the capability ledger. But the
deploy is precisely the operation that brings a fresh or owner-only database to a serveable schema
(successor to boot-time `ApplyMigrations` and `daemon migrate-db`, `daemon.go:74,167`). So the
base deployer is a plain verb against a bare / owner-only DB with no running daemon, the same
context `daemon owner-ddl apply` (`daemon.go:76,90`) already occupies.

**What keeps the run-shape door open (so P5 needs no re-litigation):** (1) the plan is a
content-addressed, durably-stored artifact (`plan_hash` + `deploy_plan`, §3.2); (2) the deploy
receipt is hash-chained into owner-held `audit_log` (§3.4) — one receipt per step; (3) the verb is
a thin CLI over a `Deployer` package with a pure `Apply(ctx, plan, cursor)` core a future run step
invokes unchanged. **Falsifiable boundary:** if a falsifier shows a real fresh-DB bring-up the
plain verb cannot perform but a run could, Q4 flips.

---

## 3. The deployer surface + serve-boot decoupling (shadow-first, by exact code site)

### 3.1 Command site (carry-forward; connection model sharpened for the receipt)

Add `case "deploy":` to the dispatch in `RunDaemon` (`go/pkg/cli/localcommands/daemon.go:67-81`,
alongside `install|uninstall|status|migrate-db|owner-ddl`) → new `runDaemonDeploy(...)`. DSN
resolution mirrors `runDaemonOwnerDDL` (`daemon.go:90`, owner/admin DSN at `daemon.go:115`):
`--owner-url` / `--admin-url`, then `STRIATUM_DAEMON_ADMIN_DB_URL` (`EnvDaemonAdminDBURL`,
`daemon.go:24`), then the daemon DSN. Flags: `--plan` (default embedded), `--dry-run` (print
ordered plan + cursor delta, mutate nothing), `--resume` (default), `--abort` (mark a wedged
cursor `aborted`), `--json`. `docs/reference/command-authority-matrix.md` and the authority
guardrail tests gain the `daemon.deploy` row (AGENTS.md change discipline).

**Connection model (load-bearing).** The deployer applies **every deploy-plan step over the
SINGLE owner connection**: the runtime migration DDL, the C3 ownership reconcile, the grant
re-assert, the `schema_migrations`/`schema_meta` version stamps (`migrations.go:320,332`), the
runtime-owned `deploy_cursor`/`deploy_plan` writes, and the per-step `append_audit_row` receipt.
Legal because the owner role is a member of `striatumd_rw` (or superuser) in the two-role
bootstrap (`authority_bootstrap.go`) — so it can write the rw-owned tables and `ALTER … OWNER TO
striatumd_rw` — and owns (thus can EXECUTE) the owner-only SD function `append_audit_row`
(`0001_authority_phase0.sql:152,238`), which runs in the caller's transaction. The **only** write
the deployer routes over a separate runtime view is the C1 finalizer's terminal `schema_state`
fingerprint self-record (deliberately mirroring the boot self-record path `connection.go:399`
byte-for-byte — §3.4) — and that write is now gated by `VerifyStoredTranscript` (§3.4a, M1).

### 3.2 The deploy plan — materialized ONCE into an immutable transcript (BC-N1), revoke 0021 sorted last (C3)

**Claim (PLAN-MATERIALIZED-ONCE — BC-N1, carry-forward).** Plan identity is computed once, at
deploy start, from the two embedded `embed.FS` sources the fingerprint already hashes — the
runtime migration FS (`migrations.go:21` `//go:embed sql/*.sql`) and the owner-bundle FS
(`owner.go:156` `//go:embed sql/owner/*.sql`) — and persisted as the immutable `deploy_plan`
transcript before step 0 (§1.2). Resume reads the stored transcript; it never recomputes the plan
from the moved live frontiers.

**Substrate-ensure preamble (carry-forward).** The deployer's FIRST action is an idempotent
substrate-ensure — apply migration 0044 (`CREATE TABLE IF NOT EXISTS …`, both tables) over the
owner connection, reconciling the new tables' ownership to `striatumd_rw` while CREATE is held
(§3.3b) and re-asserting their DML grants — BEFORE transcript materialization. 0044 is idempotent
and pre-transcript, so it is NEVER a numbered transcript step; `base_runtime_version` is observed
*after* substrate-ensure.

`BuildPlan(base_owner, base_runtime) → DeployPlan` (materialization only) — uses the FULL
`OwnerBundles()` loader (which includes 0021, see §3.2a) but special-cases the revoke to terminal:

- **Steps** = pending **non-revoke** owner bundles (ascending `Version` **≤ 20**, tagged `owner`,
  filtered via `isNonRevokeBundle`, §3.2a) **then** pending runtime migrations (ascending `Version`
  **≥ 0045**, tagged `runtime`, `migrations.go`) **then**, as the **TERMINAL step, the DDL-revoke
  bundle 0021** if pending (tagged `owner`, sorted last). The watermark interlock already requires
  owner ≥ Required before any runtime apply (`connection.go:349`, `owner.go:124-154`), promoted into
  explicit dependency edges (every `runtime` step depends on the full non-revoke `owner` prefix).
  The DDL-revoke bundle (identified by `DDLRevokeOwnerBundleVersion = 21`) is special-cased out of
  the owner prefix and appended last, with a dependency edge on every prior step, so it commits
  after the final runtime ownership reconcile.
- Each step carries `{role, step_id, step_index, sha256, transactional}`. The `transactional` bit
  is the Q3-A/Q3-B selector — a per-step `-- striatum:nontransactional` pragma defaulting to
  transactional, validated by a build-time test (sibling of `TestRunnerMigrationsHaveNoNonTransactionalDDL`,
  `migrations.go:299` / `migrations_test.go:1217`).
- **`plan_hash`** = sha256 of the canonical transcript — base frontiers `(base_owner, base_runtime)`
  followed by the ordered `step_id`+`sha256` list, including 0021 in its terminal position —
  reusing the `composeFingerprint` canonicalization (`schema_drift.go:105-137`). The fingerprint is
  over the *set* of applied migrations+bundles (order-independent) while `plan_hash` is over the
  *ordered* transcript + base, so the C3 re-ordering (0021 last) changes `plan_hash` but not the
  fingerprint — deployer and drift gate never disagree on "is the whole plan applied."
- **`step_index` is stable by storage, not recomputation (BC-N1).** The transcript is INSERTed once
  into `deploy_plan` (immutable) and every per-step receipt, the resume reader, **and the M1
  `VerifyStoredTranscript` check** key off the stored `(plan_hash, step_index)`.

### 3.2a The single non-revoke filter + the embed/listing split (M2 — the resolution)

**Claim (NON-REVOKE-FILTER — M2).** The DDL-revoke bundle 0021 is excluded from **every**
`owner-ddl apply` route by a single named filter and a split loader, so 0021's `REVOKE CREATE` can
ONLY ever be committed as the terminal `deploy` step (via `applyOneOwnerBundle`), NEVER via the
pending loop, the FMA-007 self-heal reapply, a nil-fallback, a dry-run/list surface, or a test
helper.

**The named constant and filter (new, `go/pkg/db/owner.go`):**

```go
// DDLRevokeOwnerBundleVersion identifies the RFC 0142 P4 C3 DDL-revoke bundle
// (REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw). It is DEPLOY-PLAN-TERMINAL
// ONLY: excluded from every owner-ddl apply route; committed solely as the terminal
// step of `striatum daemon deploy`. It is NOT in the owner-bundle watermark frontier
// (LatestOwnerBundleVersion / RequiredOwnerBundleVersion stay 20).
const DDLRevokeOwnerBundleVersion = 21

// isNonRevokeBundle reports whether a bundle is eligible for owner-ddl apply.
// It excludes EVERY bundle at or above the revoke ordinal, REGARDLESS of recorded
// version — not only bundles newer than the current watermark.
func isNonRevokeBundle(b OwnerBundle) bool { return b.Version < DDLRevokeOwnerBundleVersion }

// OwnerDDLApplyBundles is the ONLY bundle slice every owner-ddl apply route iterates:
// the embedded owner bundles MINUS every bundle >= DDLRevokeOwnerBundleVersion.
// OwnerBundles() (full, includes 0021) is kept ONLY for revokeEmbedded detection,
// ExpectedFingerprint, the deploy plan's BuildPlan, and RuntimeOwnedTablesAlterable.
func OwnerDDLApplyBundles() ([]OwnerBundle, error) {
	all, err := OwnerBundles()
	if err != nil {
		return nil, err
	}
	out := all[:0:0]
	for _, b := range all {
		if isNonRevokeBundle(b) {
			out = append(out, b)
		}
	}
	return out, nil
}
```

**The bindings (every `owner-ddl apply` route, current `main`):**

1. **`ApplyOwnerBundles` (`owner.go:265-303`)** loads **`OwnerDDLApplyBundles()`** at `owner.go:269`
   instead of `OwnerBundles()`. Both the pending loop (`applyPendingOwnerBundles`, called `:277`)
   and the self-heal fallback (`ReapplyAllOwnerBundles`, called `:289`) therefore iterate the
   filtered slice — 0021 is never in it.
2. **`applyPendingOwnerBundles` (`owner.go:309-322`)** keeps its `<= current` bound (`:312`) AND
   adds an in-loop `if !isNonRevokeBundle(bundle) { continue }` guard (defense-in-depth against a
   hand-passed slice).
3. **`ReapplyAllOwnerBundles` (`owner.go:332-351`)** — the FMA-007 self-heal — adds the same in-loop
   `if !isNonRevokeBundle(bundle) { continue }` guard (`:344-349`), AND its `bundles == nil`
   fallback (`:336-342`) loads **`OwnerDDLApplyBundles()`** instead of `OwnerBundles()`. So even
   when `ApplyOwnerBundles` falls into the FMA-007 reapply over the full loaded slice, the revoke is
   filtered out, and even a caller that passes `nil` gets the filtered set.
4. **The `ApplyOwnerBundles` self-heal branch (`owner.go:277-302`)** is bound by (1) — it hands
   `ReapplyAllOwnerBundles` the already-filtered slice it loaded at `:269` — AND by (3)'s in-loop
   guard. Two independent barriers.
5. **Tests** — every `db.ApplyOwnerBundles(...)`/`db.ReapplyAllOwnerBundles(...)` caller (the
   ~50 pgtest setups, `pgtest/two_role.go:130`, the FMA-007 test
   `owner_pg_test.go:219` which calls `ReapplyAllOwnerBundles(ctx, runner, nil, …)` at `:229`)
   inherits the filter through (1)/(3); the new F16 test asserts it.
6. **Dry-run / list surfaces** — none exist today (`grep` over `daemon.go` finds no `--dry-run`/list
   for owner-ddl); the new `deploy --dry-run` (§3.1) prints the deploy plan, whose 0021 is the
   terminal step by design. Any future owner-ddl list surface MUST iterate `OwnerDDLApplyBundles()`,
   asserted by the build-time test below.

**The embed/listing split (the M2 conceptual fix).** `OwnerBundles()` (`owner.go:197-224`) stays
the full loader (includes 0021 once authored) so that **`revokeEmbedded`** (the presence of the
0021 file in `ownerBundleFS`), **`ExpectedFingerprint()`** (`schema_drift.go:88`, which must hash
0021's bytes for the post-deploy in-sync fingerprint), the **deploy plan** (`BuildPlan`, which
special-cases 0021 to terminal), and the pure read **`RuntimeOwnedTablesAlterable()`**
(`owner_runtime_ownership.go:52`, which derives the runtime-table allowlist from bundle SQL and is
unaffected by a REVOKE-only bundle) all see 0021. **`OwnerDDLApplyBundles()`** is the disjoint
slice for apply routes and never includes it. So "the binary embeds 0021" ≠ "`owner-ddl apply`
iterates 0021."

**Build-time guard (executable, named).** A unit test
(`TestOwnerDDLApplyExcludesRevokeBundle`) asserts: (a) `OwnerDDLApplyBundles()` contains no bundle
`>= DDLRevokeOwnerBundleVersion`; (b) `OwnerBundles()` DOES contain 0021 (so `revokeEmbedded` /
`ExpectedFingerprint` see it); (c) `ReapplyAllOwnerBundles(ctx, runner, nil, …)`'s nil-fallback
resolves to `OwnerDDLApplyBundles()`. A second build-time grep test asserts no production caller of
`applyOneOwnerBundle` outside the deployer's terminal-step path receives a bundle `>= 21`.

### 3.3 Lifting `ApplyMigrations` out of serve-boot (shadow-first; carry-forward)

**Claim (DECOUPLE).** Serve-boot stops mutating schema, behind a flag, shadow-first. The single
mutation site on the serving path is `ConnectAndMigrate` (`connection.go:332-404`), reached via
`BootstrapAndConnect(…, migrate)` (`authority_bootstrap.go:181,193`), driven by `--migrate` (default
`true`, `main.go:76`).

1. **New env `STRIATUM_DEPLOY_DECOUPLED`** (default OFF — shadow-first, cf. `EnvSchemaDriftRefuse`,
   `schema_drift.go:28`). OFF → boot behaves exactly as today; the decoupling lands to `main` inert.
2. When **ON**, `ConnectAndMigrate` becomes **`ConnectAndVerify`**: keep the Layer 2 watermark
   interlock (`connection.go:349`) and the Layer 3 drift gate (`connection.go:376-383`) but do NOT
   call `ApplyMigrations` (`connection.go:353`). Run the §1.3 classification and serve only on the
   in-sync row; on any other row refuse-to-serve via the typed `awaiting_deploy` halt (§4.2). The
   drift self-record (`connection.go:399`) is NOT run on the verify path — only the deployer records
   the fingerprint (§3.4), so a verify-only boot can never overwrite `schema_state` to falsely claim
   in-sync.
3. **No window serving an unmigrated schema (R2).** The decoupled boot verifies before serving and
   refuses on any non-in-sync state; the watermark interlock is preserved verbatim.

#### 3.3a The deploy-activation interlock — the UNIVERSAL pre-revoke cursor edge (BC-N2) + the C2 arms

A **`CheckDeployActivation(ctx, runner, revokeEmbedded, decoupledEnabled bool) error`** (in
`go/pkg/db/owner.go` or a new `deploy_activation.go`) is called at the same boot site, immediately
after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations` (`connection.go:349-353`), in both
`ConnectAndMigrate` and `ConnectAndVerify`, and before `RecordSchemaFingerprint`
(`connection.go:399`). It mutates nothing. It reads `deploy_cursor` defensively (absent table/row →
`cursorState = none`). Predicate (fail-closed), in order — **carried forward verbatim from v4**:

1. **UNIVERSAL incomplete-deploy edge (BC-N2 — every deployer-aware binary).** If `cursorState ∈
   {in_progress, step_committed, finalizing}` → return **`awaiting_deploy`**, DB untouched. Holds at
   `applied_owner == 20` (and 21), so a no-revoke landing binary that restarts mid-deploy halts
   BEFORE `ApplyMigrations` and BEFORE `RecordSchemaFingerprint`. The §1.3 stored-transcript /
   mismatch distinctions apply to how `deploy` then *resumes*; for the *serve* decision, any
   non-`complete` cursor refuses.
2. If `cursorState == aborted` → return **`awaiting_deploy`**. DB untouched.
3. If `cursorState == complete`: defer to the drift gate — `plan_hash == expected` + fingerprint
   match → serve; foreign `plan_hash` or mismatch → existing `schema_drift` halt
   (`connection.go:376-383`). Returns nil here.
4. If `cursorState == none` (absent table/row, or `idle`):
   - `decoupledEnabled == true` → **`awaiting_deploy`** (the decoupled boot never auto-applies).
   - `decoupledEnabled == false` (legacy auto-apply path):
     - `revokeEmbedded == true` → **`awaiting_deploy_config`** (binary ships 0021; set the flag and
       run `deploy`). DB untouched.
     - `revokeEmbedded == false` → **return nil**: legacy path, serve, `ApplyMigrations` runs. The
       inert-landing binary's normal boot and fresh-DB bring-up; NOT blocked.

`revokeEmbedded` = the embedded owner-bundle FS contains the bundle at `DDLRevokeOwnerBundleVersion
= 21`. This is "the binary ships the revoke," NOT `Latest >= 21` — `LatestOwnerBundleVersion` stays
20 even for a revoke-embedding binary (0021 is deploy-plan-only, §3.2a). The activation binary is
detected by the presence of the 0021 file in `ownerBundleFS` (via `revokeEmbedded`), not by the
watermark frontier.

**Net invariant (the BC-N2 + C2 close):** No boot serves while the cursor is non-`complete`, and
`ApplyMigrations` / `RecordSchemaFingerprint` are never reached over an incomplete deploy — for ANY
deployer-aware binary.

#### 3.3b Runtime-object ownership: Policy 1 with the revoke sequenced last (C3 — carry-forward, re-anchored)

The C3 prerequisite: Policy 1 reconciles ownership with `ALTER <kind> striatumd.<name> OWNER TO
striatumd_rw`, but PostgreSQL requires the new owner to hold `CREATE ON SCHEMA striatumd` — the
repo's bundles document this and GRANT `CREATE` FIRST for exactly this reason:
`0018_runtime_table_ownership_transfer.sql:64-66` (prerequisite) and `:98` (`GRANT CREATE ON SCHEMA
striatumd TO striatumd_rw` before the `:102` `ALTER … OWNER TO` loop);
`0019_supervisor_pointer_runtime_ownership.sql:53-80` repeats the prerequisite and grant.

**Resolution (a) — sequence the revoke last (carry-forward).** Bundle 0021 is the terminal plan
step (§3.2). The runtime ownership reconciles all run while `striatumd_rw` still holds CREATE
(granted idempotently by bundle 0018 at `0018:98`, in the non-revoke owner prefix and therefore
applied before any runtime step). 0021 commits last, so the committed steady state still denies
CREATE.

**Chosen policy (one, tested): runtime objects stay `striatumd_rw`-owned.** Mechanism
(`Deployer.applyRuntimeStep`, wrapping the existing `applyOne` body, `migrations.go:304-355`),
unchanged from v4:

1. `BEGIN` (owner connection). Deployer precondition assert:
   `has_schema_privilege('striatumd_rw','striatumd','CREATE')` is true; else halt legibly
   (`deploy_create_prerequisite_missing`) before any DDL.
2. Snapshot the owner-role-owned object oids in schema `striatumd`.
3. Run the migration SQL (the existing `applyOne` DDL + both version stamps + in-tx hash verify,
   `migrations.go:304-355`).
4. Re-snapshot; for exactly the newly-appeared oids run `ALTER <kind> striatumd.<name> OWNER TO
   striatumd_rw`.
5. Re-assert the migration's DML grants to `striatumd_rw`.
6. Append the per-step receipt via `append_audit_row(...)` keyed on the stored `(plan_hash,
   step_index)` (§3.4) — same transaction.
7. Advance `deploy_cursor` `in_progress(k) → step_committed(k)` (Q3-A). `COMMIT`.

The reconciliation is idempotent. It runs only on transactional object-creating runtime steps and
on the substrate-ensure preamble's 0044 tables. The owner role can `ALTER … OWNER TO striatumd_rw`
because it is a member of that role and `striatumd_rw` still holds CREATE at this plan time (0021
not yet applied).

### 3.4 The deploy receipt: per-step, keyed off the STORED transcript (BC-N1 + N1) + the idempotent finalizer (C1) + the M1 pre-finalizer gate

**Claim (RECEIPT — carry-forward).** Every applied step writes exactly one hash-chained deploy
receipt into the owner-held `audit_log` via `append_audit_row(...)`
(`0001_authority_phase0.sql:152,238`; runtime direct `INSERT` revoked at `:237`). Payload:
`{plan_hash, step_index, step_id, role, sha256, state, prev_receipt_hash}`. `state ∈
{step_committed, complete}`. The key `(plan_hash, step_index)` is read from the immutable
`deploy_plan` transcript (§1.2), not recomputed.

**Per-step receipt crash-safety (N1) on a stable key (BC-N1)** — carry-forward:

- **Q3-A (transactional step):** the receipt append occurs in the same owner-connection transaction
  as the DDL + ownership reconcile + grants + version stamps + cursor advance (§3.3b steps 1–7).
  `step_committed(k)` is durable iff its receipt is durable. Same for the terminal 0021 step (its
  receipt joins the per-bundle transaction, `owner.go:511-541`).
- **Q3-B (non-transactional step):** the `in_progress(k)` reconciler appends exactly one receipt
  keyed on the stored `(plan_hash, step_index)` before writing `step_committed(k)`, idempotent on
  re-entry.

**The idempotent finalizer (C1) with the M1 full-transcript gate (NEW):** when the terminal step is
`step_committed`, the cursor advances to `finalizing` in one commit, then the finalizer runs:

0. **`VerifyStoredTranscript(plan_hash)` (M1, NEW — runs FIRST, writes nothing).** Re-validates the
   ENTIRE stored transcript against the running binary's embedded bytes AND verifies every
   already-applied step's DB stamp matches the transcript (§3.4a). On `deploy_plan_binary_mismatch`
   or `deploy_plan_db_stamp_mismatch` the finalizer ABORTS — it does NOT append the `complete`
   receipt, does NOT call `RecordSchemaFingerprint`, does NOT advance `finalizing → complete`. The
   cursor stays at `finalizing` (classified resumable-finalization / mismatch by §1.3), the database
   is untouched, and a later boot refuses to serve (non-`complete` cursor). **This is the structural
   bar that makes M1's hybrid-self-record impossible: the finalizer can never write a fingerprint
   for a binary that disagrees with an already-applied step.**
1. Append the guarded **`complete` deploy receipt** (owner connection, `append_audit_row`) — skip if
   a `(plan_hash, state=complete)` receipt already exists (idempotent).
2. **`RecordSchemaFingerprint`** (`schema_drift.go:171-195`) UPSERT into `schema_state` (idempotent;
   deterministic `ExpectedFingerprint()` `schema_drift.go:83-100`, == this plan's recorded
   fingerprint by §3.2 because step 0 passed).
3. Advance `finalizing → complete` LAST.

Because step (0) gates (1)–(3), a `complete` cursor proves the deploy is fully finalized by a binary
whose embedded bytes byte-match every applied step and whose DB stamps match the transcript. A crash
anywhere in finalization leaves the cursor at `finalizing`; the finalizer is idempotent AND
re-verifies on every entry, so re-run with the *right* binary repairs exactly once and re-run with a
*wrong* binary halts at step (0).

**Finalizer crash-point table (C1 — carry-forward, re-anchored 0021; M1 row added):**

| Crash point | Cursor after crash | §1.3 classification | Re-run repair |
| --- | --- | --- | --- |
| after final step (0021) DDL+receipt commit, before `finalizing` | `step_committed(N-1)` | incomplete, resume (off stored transcript) | verify transcript; advance to `finalizing`; run finalizer |
| after `finalizing`, before verify/receipt | `finalizing` | **resumable finalization** | step 0 verify passes (right binary) → append; fingerprint; `complete` |
| after receipt, before `schema_state` | `finalizing` | **resumable finalization** | step 0 verify passes; receipt present → **skip**; fingerprint; `complete` |
| after `schema_state`, before `complete` | `finalizing` | **resumable finalization** (cursor ≠ `complete` ⇒ never serve) | step 0 verify passes; receipt skip; fingerprint UPSERT no-op; `complete` |
| **finalizer entered under a WRONG binary (M1)** | `finalizing` | **`deploy_plan_binary_mismatch` / `deploy_plan_db_stamp_mismatch`** | step 0 verify FAILS → **abort finalizer, write nothing**; refuse-to-serve; operator inspects |
| after `complete` | `complete`, fingerprint match, receipt present | in sync | none — serve |

**Tightened `doctor schema_deploy_unrecorded` — enumerates from the STORED transcript (BC-N1) +
verifies stamps (M1).** `doctor` (modeled on `schemaDriftDoctorBlock`,
`go/pkg/reads/doctor_schema_drift.go:26-77`) keyed off the stored `deploy_plan` transcript: when a
`deploy_cursor` row exists it loads `deploy_plan[cursor.plan_hash]`, determines which transcript
`step_index`es are applied, and WARNs when (a) any applied transcript step lacks a `step_committed`
receipt for its `(plan_hash, step_index)`, **or (b) NEW (M1) — any applied transcript step's DB
stamp (`schema_migrations.sha256` / `owner_bundle_meta.sha256`) diverges from the stored transcript
`sha256`, or any stored step's `sha256` diverges from this binary's embedded bytes** (i.e. a latent
`deploy_plan_binary_mismatch` / `deploy_plan_db_stamp_mismatch` even before a deploy runs). So the
doctor is green only when all committed transcript steps have receipts AND every stored step
byte-matches the binary AND every applied DB stamp matches the transcript.

### 3.4a `VerifyStoredTranscript` — the full-transcript byte + DB-stamp verifier (M1, the resolution)

**Claim (FULL-TRANSCRIPT-VERIFY — M1).** A single function is the sole authority for "may this
binary resume / finalize this stored plan." It is a PURE READ (mutates nothing) and is called (i) on
every resume before any apply, (ii) as finalizer step 0 before any `schema_state` write, and (iii)
by the doctor. It closes the v4 gap by checking the WHOLE transcript, not just not-yet-applied
steps, and by additionally checking the already-applied DB stamps.

```go
// VerifyStoredTranscript validates the immutable deploy_plan transcript against the
// RUNNING binary and the database, for the M1 resolution. PURE READ.
//
//   For EVERY step in deploy_plan[plan_hash].steps (already-applied AND not-yet-applied):
//     1. byte check: step.sha256 == this binary's embedded bytes for step.step_id
//        (MigrationSHASet()[version] for runtime, OwnerBundle.SHA256() for owner).
//        ANY mismatch -> ErrDeployPlanBinaryMismatch (deploy_plan_binary_mismatch).
//   For every ALREADY-APPLIED step (step_index <= cursor position / stamped in the DB):
//     2. db-stamp check: step.sha256 == the recorded DB stamp
//        (schema_migrations.sha256[version] for runtime, owner_bundle_meta.sha256[version]
//        for owner). ANY mismatch -> ErrDeployPlanDBStampMismatch (deploy_plan_db_stamp_mismatch).
//   Returns nil ONLY when every step byte-matches the binary and every applied step's
//   DB stamp matches the transcript.
func VerifyStoredTranscript(ctx context.Context, runner Runner, planHash string) error
```

- **Byte check (every step).** Reuses `MigrationSHASet()` (`migrations.go:197`) for runtime
  `step_id`s and `OwnerBundle.SHA256()` (`owner.go:190-194`) for owner `step_id`s — the exact
  byte-hashes `ExpectedFingerprint()` composes (`schema_drift.go:83-100`). A binary B whose embedded
  0045 bytes (`B45`) differ from the stored transcript's recorded `A45` is caught here, on step 0
  (already-applied) — exactly the case the v4 not-yet-applied check missed. Result:
  `deploy_plan_binary_mismatch`, DB untouched, apply/finalize nothing.
- **DB-stamp check (already-applied steps).** Reads `schema_migrations.sha256` for the version (the
  same column `verifyRecordedHashTx` reads, `migrations.go:275`) for runtime steps, and
  `owner_bundle_meta.sha256` (stamped at `owner.go:529`) for owner steps, and asserts each equals
  the stored transcript `sha256`. This catches a DB whose applied stamp diverges from the transcript
  (a hand-edited stamp, a pre-fix crash that stamped a different sha, or a transcript that no longer
  describes the real DB) → `deploy_plan_db_stamp_mismatch`, refuse-to-finalize.
- **Together** they pin a three-way equality the v4 spec only half-enforced: **stored transcript
  `sha256` == running binary embedded bytes == DB stamp** for every applied step, and **stored
  transcript == binary** for every pending step. Only then may the deploy resume, and only then may
  the finalizer self-record `ExpectedFingerprint()`.

**Two new typed halts** (siblings of `AwaitingDeployError`, reusing the non-restartable exit
`main.go:208-227`):

- **`ErrDeployPlanBinaryMismatch` / `DeployPlanBinaryMismatchError`** (`deploy_plan_binary_mismatch`):
  a stored step's `sha256` ≠ this binary's embedded bytes. Remediation: "resume with the binary that
  materialized this plan, or `striatum daemon deploy --abort` and redeploy." DB untouched.
- **`ErrDeployPlanDBStampMismatch` / `DeployPlanDBStampMismatchError`**
  (`deploy_plan_db_stamp_mismatch`): an already-applied step's DB stamp ≠ the stored transcript.
  Remediation: "the database does not reflect the recorded transcript; inspect
  `schema_migrations`/`owner_bundle_meta` against the deploy receipts before resuming." DB untouched.

`runDaemonDeploy` and the boot `CheckDeployActivation` path map both to refuse-to-serve; the
`errors.As` arms join the `AwaitingOwnerDDLError`/`SchemaDriftError` arms (`main.go:208-227`).

---

## 4. Serving-role DDL revocation (owner bundle 0021), without lockout, without a pre-revoke serve window, without an early self-heal revoke

**Claim (DDL-REVOKE).** The serving role `striatumd_rw` loses its remaining object-creation DDL on
the serving path, shipped as owner bundle 0021, applied **only as the terminal step of `striatum
daemon deploy`** (C3), **gated by the universal cursor edge (BC-N2)**, and **excluded from every
`owner-ddl apply` route including the FMA-007 self-heal (M2)** — so failure mode 1 becomes
structurally impossible without recreating a #512-class lockout, without the v2 self-contradiction,
without the v3 pre-revoke serve hole, and without the v4 self-heal early-revoke escape.

### 4.1 What is revoked, precisely (carry-forward)

`striatumd_rw` already holds no DDL on owner-owned tables (`0001_authority_phase0.sql:237`; preflight
+ build guard route owner-table DDL to bundles, `migrations.go:158`). The residual serve-path DDL is
object creation on the runtime schema. Bundle 0021 revokes exactly that:

```
-- owner bundle 0021 (DDL-revoke): the serving path holds zero create-DDL.
-- DEPLOY-PLAN-TERMINAL ONLY (RFC 0142 P4 C3); excluded from EVERY owner-ddl apply
-- route via OwnerDDLApplyBundles() (LatestOwnerBundleVersion stays 20).
REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw;
REVOKE TEMPORARY ON DATABASE <db> FROM striatumd_rw;  -- if granted
```

`striatumd_rw` retains ownership of (and `ALTER`/`DROP` on) the runtime tables it already owns; new
runtime objects are reconciled back to `striatumd_rw` before 0021 commits (§3.3b), so "new runtime
objects stay `striatumd_rw`-owned" holds as a tested property (F12). Full capability revocation
(owner re-owns runtime tables) remains a follow-up beyond P4 (it would re-open #442), named, not
assumed.

### 4.2 Typed halts + the forward-watermark rule (C2 — carry-forward) + the universal cursor edge (BC-N2) + the M1 mismatch halts

Sibling halts of `AwaitingOwnerDDLError` (`owner.go:42,58`), reusing the non-restartable exit
(`main.go:208-227`, `exitAwaitingOwnerDDL`):

- **`AwaitingDeployError`** (`awaiting_deploy`): a deploy is pending/incomplete — cursor absent on
  the decoupled path, or `state ∈ {in_progress, step_committed, finalizing, aborted}` on any
  deployer-aware binary (BC-N2). DB untouched.
- **`AwaitingDeployConfigError`** (`awaiting_deploy_config`): the binary ships 0021 but
  `STRIATUM_DEPLOY_DECOUPLED` is OFF and no deploy is in flight. DB untouched.
- **`DeployPlanBinaryMismatchError`** (`deploy_plan_binary_mismatch`) and
  **`DeployPlanDBStampMismatchError`** (`deploy_plan_db_stamp_mismatch`) — NEW (M1, §3.4a). DB
  untouched, no fingerprint write.

**Forward-watermark rule (in `CheckOwnerBundleWatermark`, the tolerate-forward branch
`owner.go:151-153`), re-anchored to 21.** When `applied >= DDLRevokeOwnerBundleVersion (21)` and this
binary does NOT embed the revoke (0021 absent from `ownerBundleFS`), the watermark returns
`AwaitingDeployError` instead of tolerating forward. A revoke-aware binary tolerates forward across
the boundary but is governed by `CheckDeployActivation` (§3.3a). Pre-0021 forward tolerance unchanged.

**Resolution of the v1 contradiction (preserved, re-anchored).** `RequiredOwnerBundleVersion` is NOT
advanced to 21; it stays **20** (`owner.go:35`, its current-`main` value). The DDL-revoke is gated by
the universal cursor edge + `CheckDeployActivation` + the `STRIATUM_DEPLOY_DECOUPLED` flag + its
deploy-plan-terminal placement + the `OwnerDDLApplyBundles` exclusion — NOT by the watermark frontier.

### 4.3 The deploy choreography (C2 — carry-forward; re-anchored; M1/M2 hardened)

Two binaries, in order:

1. **Inert-landing binary** (`Latest = Required = 20`; does NOT embed 0021). Ships: the `Deployer`
   engine + `deploy` verb, the 0044 `deploy_cursor` + `deploy_plan` migration, the
   `STRIATUM_DEPLOY_DECOUPLED` decoupled boot path (flag OFF), the per-step + terminal `doctor
   schema_deploy_unrecorded` block, the forward-watermark rule, the universal pre-revoke cursor edge
   (BC-N2), **`VerifyStoredTranscript` (M1), and `OwnerDDLApplyBundles` (M2)**. Boot auto-applies
   legacy only because no deploy is in flight. Decoupling inert. **Daemon up, serving.**
2. **Activation binary** (embeds 0021; `Latest = Required` still 20; `CheckDeployActivation` active).
   On boot with the deploy incomplete + flag OFF it halts `awaiting_deploy_config`. The operator
   then, with the daemon parked: sets `STRIATUM_DEPLOY_DECOUPLED=1`; runs `striatum daemon deploy`
   (substrate-ensure → materialize the immutable transcript → apply non-revoke owner bundles (≤20) →
   pending runtime steps (each reconciling ownership while CREATE held) → bundle 0021 terminal →
   finalizer with the `VerifyStoredTranscript` gate); restarts the daemon → `ConnectAndVerify` sees
   `complete` + in-sync → serves, verify-only, zero serve-path create-DDL.

   **Crash mid-deploy is safe even if the inert binary comes back up (BC-N2).** Its universal cursor
   edge reads the `step_committed(k)` cursor and halts `awaiting_deploy` DB-untouched.

   **`owner-ddl apply` invoked on the activation binary before deploy is safe (M2, NEW).** Even if
   the operator runs `striatum daemon owner-ddl apply` on the activation binary (which embeds 0021),
   and even if it hits the FMA-007 cross-bundle dependency self-heal, `ApplyOwnerBundles` iterates
   `OwnerDDLApplyBundles()` (0021 excluded) and `ReapplyAllOwnerBundles` guards `isNonRevokeBundle`
   in-loop, so 0021 is NEVER committed by `owner-ddl apply`; CREATE stays held; the subsequent deploy
   runtime reconcile still runs. The #512-class lockout escape the v4 spec left open is closed.

### 4.4 How 0021 ships without lockout (R3), without a pre-revoke serve window (BC-N2), and without an early self-heal revoke (M2)

The deployer applies all DDL (runtime steps and the terminal 0021) over the owner/admin connection —
the same mechanism `daemon migrate-db --admin-url` (`daemon.go:167`) and `owner-ddl apply`
(`daemon.go:90`) already use — so the runtime path is already owner-capable out-of-band; 0021 removes
no capability the deploy path needs. **The C3 fix:** because 0021 is the terminal plan step (after
every runtime ownership reconcile) and is excluded from the owner prefix (§3.2), there is no path by
which `REVOKE CREATE` commits before a reconcile that needs CREATE *inside the deploy plan*. **The
M2 fix:** because every `owner-ddl apply` route iterates `OwnerDDLApplyBundles()` (0021 excluded) and
both apply loops guard `isNonRevokeBundle` in-loop, there is no path by which `owner-ddl apply` —
including the FMA-007 self-heal reapply — commits `REVOKE CREATE` early *outside the deploy plan*.
**The BC-N2 fix:** because every deployer-aware binary reads `deploy_cursor` before
`ApplyMigrations`/`RecordSchemaFingerprint` and refuses on a non-`complete` cursor (§3.3a), there is
no boot that can serve or self-record over the pre-revoke incomplete-deploy window. Combined,
**no ordering — including every interleaving in `T-deploy-revoke-activation-ordering`,
`T-deploy-runtime-object-ownership`, and `T-deploy-revoke-excluded-from-reapply-self-heal` — drives
boot into `ApplyMigrations` under a revoked CREATE, lets any binary serve an incomplete deploy,
strands a deploy at a reconcile under a revoked CREATE, or commits the revoke outside the terminal
deploy step.**

### 4.5 Proactive hardening — every apply path and every fingerprint path named, two universal invariants (the SEED requirement)

Audited once, exhaustively, against current `main` (HEAD `2e482077`).

**A. EVERY code path that applies owner bundles (file:line):**

| Path | Site | Disposition under M2 |
| --- | --- | --- |
| `ApplyOwnerBundles` (the `owner-ddl apply` entry) | `owner.go:265-303` (loads slice `:269`; pending `:277`; self-heal `:289`) | loads **`OwnerDDLApplyBundles()`** — 0021 excluded |
| `applyPendingOwnerBundles` (pending loop) | `owner.go:309-322` (`<= current` bound `:312`; `applyOneOwnerBundle` `:315`) | iterates filtered slice + in-loop `isNonRevokeBundle` guard |
| `ReapplyAllOwnerBundles` (FMA-007 self-heal) | `owner.go:332-351` (unbounded loop `:344-349`; nil-fallback `:336-342`) | in-loop `isNonRevokeBundle` guard + nil-fallback loads `OwnerDDLApplyBundles()` |
| `applyOneOwnerBundle` (per-bundle tx — the actual applier) | `owner.go:511-541` (SQL `:522`; stamp `:528-535`) | reused by the deployer's TERMINAL 0021 step — the ONLY committer of 0021 |
| `runDaemonOwnerDDL` (CLI) | `daemon.go:90` → `ApplyOwnerBundles` `:131` (+ `ReassertWriteRevokes` `:140`, `ReassertReadRevokes` `:144` — ACL re-asserts, NOT bundle appliers, never touch 0021) | inherits the filter via `ApplyOwnerBundles` |
| Deployer terminal step (P4, new) | `Deployer.Apply` (§3.4) | the ONLY legitimate applier of 0021, via `applyOneOwnerBundle` |
| Test setups (~50 `db.ApplyOwnerBundles(...)`; FMA-007 `ReapplyAllOwnerBundles(…, nil, …)` `owner_pg_test.go:229`; `pgtest/two_role.go:130`) | various `*_test.go` | inherit the filter; F16 asserts the self-heal excludes 0021 |
| `OwnerBundles()` consumers that are NOT apply paths (`ExpectedFingerprint` `schema_drift.go:88`; `revokeEmbedded` detection; `BuildPlan`; `RuntimeOwnedTablesAlterable` `owner_runtime_ownership.go:52`) | various | keep the FULL loader (must see 0021); never call `applyOneOwnerBundle` |

> **UNIVERSAL INVARIANT A (executable).** *The DDL-revoke bundle (`>= DDLRevokeOwnerBundleVersion =
> 21`) is excluded from ALL `owner-ddl apply` routes — `ApplyOwnerBundles`, `applyPendingOwnerBundles`,
> `ReapplyAllOwnerBundles`, their nil-fallbacks, any dry-run/list surface, and every test helper —
> via `OwnerDDLApplyBundles()` + the in-loop `isNonRevokeBundle` guard. Only the terminal `deploy`
> step commits it, via `applyOneOwnerBundle`.* Enforced by `TestOwnerDDLApplyExcludesRevokeBundle`
> (§3.2a) and `F16`.

**B. EVERY code path that self-records / writes a schema fingerprint (file:line):**

| Path | Site | Disposition under M1 |
| --- | --- | --- |
| `RecordSchemaFingerprint` (the ONLY `schema_state.fingerprint` writer) | `schema_drift.go:171-195` | unchanged; gated by callers |
| Legacy boot self-record | `connection.go:399` ("This is the only writer of schema_state") | legacy `ConnectAndMigrate` path only; the running binary IS the applier (no transcript, cursor absent), so it stays as-is; NOT run on the decoupled `ConnectAndVerify` path (§3.3) |
| C1 finalizer `schema_state` UPSERT + `finalizing → complete` | P4 `Deployer.Apply` (§3.4) | gated by **`VerifyStoredTranscript` finalizer step 0 (M1)** — writes nothing on mismatch |
| `ExpectedFingerprint()` (byte-hash; pure) | `schema_drift.go:83-100`; read at `connection.go:388`, `doctor_schema_drift.go:32`, `schema_drift.go:255` | read-only; no write |
| `CheckSchemaDrift` (boot gate; pure read) | `schema_drift.go:254-274` | read-only |
| Test helpers | `schema_drift_test.go` etc. | exercise the gate; F15 asserts the finalizer refuses |

> **UNIVERSAL INVARIANT B (executable).** *NO fingerprint / `schema_state` `complete` is ever written
> by the deployer unless the FULL stored transcript (every step, applied and not) byte-matches the
> running binary AND every already-applied DB stamp matches the transcript — enforced by
> `VerifyStoredTranscript` as finalizer step 0 (§3.4) and on every resume (§1.2). The legacy
> `connection.go:399` self-record is the running binary recording its OWN just-applied schema (no
> transcript involved) and is out of scope of this deployer invariant; the decoupled verify path
> never self-records.* Enforced by `F15`.

These two invariants are the executable, named requirements the cycle-5 falsifiers verify against the
named code sites — so an unaudited sibling path cannot spawn an M3.

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> `T-` = unit/integration test; `G-` = game-day against a real two-role cluster. F1–F14 carry forward
> from v4 (re-confirmed against current `main`); **F14 is EXTENDED for M1's already-applied case**,
> **F12 / `G-revoke-last` are EXTENDED for M2's owner-ddl side-path**, and **F15 (M1) + F16 (M2) are
> NEW** — these are what the cycle-5 falsifiers re-attack first.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1 (Resumability — Q3-A)** | Kill `deploy` after a transactional step *k* commits; re-run resumes at *k+1* off the stored transcript, never re-runs *k* or half-applies. | `T-deploy-resume-transactional`; `G-resume-kill-9`. |
| **F2 (Resumability — Q3-B)** | Kill during `CREATE INDEX CONCURRENTLY`; re-run's reconciler drops the `INVALID` index and rebuilds; final state is one VALID index, no double-apply. | `T-deploy-resume-concurrently`; `G-cic-kill`. |
| **F3 (No serve-boot mutation)** | With `STRIATUM_DEPLOY_DECOUPLED=ON`, a boot with a pending plan does NOT apply it; refuses `awaiting_deploy`. | `T-boot-decoupled-no-apply`; `G-pending-restart`. |
| **F4 (Fingerprint coherence — §1.3)** | An interrupted deploy is classified incomplete-resume / resumable-finalization / resume-with-stored-plan, never unknown drift; an absent `deploy_plan` row OR **any** stored step (applied or pending) byte-divergence is `deploy_plan_binary_mismatch`; an already-applied DB-stamp divergence is `deploy_plan_db_stamp_mismatch`. | `T-classify-incomplete-vs-drift`: matrix over `{cursor state} × {deploy_plan present/absent} × {fingerprint match} × {pending plan == cursor plan?} × {already-applied vs pending step byte-tamper} × {DB-stamp tamper}` asserting the §1.3 table exactly. |
| **F5 (No regression of P2/P3 — R2)** | Lifting `ApplyMigrations` keeps the watermark interlock; fresh-DB bring-up works via the verb; a fresh/idle DB (no cursor) is NOT wedged by the universal edge. | `T-decoupled-preserves-watermark`; `T-fresh-db-via-deploy`; `T-idle-cursor-serves`; existing two-role guard stays green. |
| **F6 (DDL revocation — R3, structural)** | After the terminal bundle 0021, `striatumd_rw` cannot `CREATE` on schema `striatumd`; the deployer still applies runtime steps; no lockout across restart. | `T-revoke-blocks-rw-create`; `T-deploy-after-revoke`; `G-revoke-then-restart`. |
| **F7 (Receipt provenance — per-step, stored-transcript key)** | Every applied step writes exactly one hash-chained receipt keyed on the stored `(plan_hash, step_index)`; a committed step without one is surfaced by `doctor schema_deploy_unrecorded`. | `T-receipt-per-step`; `T-doctor-unrecorded-per-step`. |
| **F8 (No double-apply/skip hole — R4)** | No interleaving of crash points across a step boundary yields a skipped/twice-applied step or a missing/duplicated receipt, key read from the stored transcript. | `T-deploy-fault-injection`: parametrized crash at each boundary for a T-DDL and an NT-DDL step. |
| **F9 (Plan ≡ fingerprint binding; 0021-terminal)** | A `complete` deploy's recorded fingerprint equals `ExpectedFingerprint()`, and `plan_hash` is over base + ordered transcript with 0021 last. | `T-plan-hash-binds-fingerprint`; `T-plan-orders-revoke-last`. |
| **F10 (Finalization boundary closed — C1; carry-forward + M1 step 0)** | No crash during finalization leaves a `complete` cursor with a missing receipt/fingerprint; every interrupted finalization is pinned at `finalizing` and repaired exactly once **by the right binary**; the finalizer's `VerifyStoredTranscript` step 0 runs before any write. | `T-deploy-resume-finalization-crash`: kill after each of {final DDL commit; `finalizing`-before-verify; verify-before-receipt; receipt-before-`schema_state`; `schema_state`-before-`complete`}; assert no boot serves until `complete`, exactly one `complete` receipt, `LiveFingerprint == ExpectedFingerprint`, doctor green. `G-finalize-kill`. |
| **F11 (Activation fail-closed — C2 + BC-N2 universal edge)** | For every deployer-aware binary, boot never calls `ApplyMigrations`/`RecordSchemaFingerprint` while the cursor is non-`complete`; a NO-revoke binary at `applied_owner == 20` with `state ∈ {in_progress, step_committed, finalizing}` halts `awaiting_deploy` DB-byte-identical; a no-cursor/`idle` DB SERVES. | `T-deploy-revoke-activation-ordering` cases (a)–(f) (carry-forward from v4); spies prove `applyOne`/`RecordSchemaFingerprint` never entered in (a)–(e). `G-old-binary-refuse`. |
| **F12 (Runtime-object ownership under the 0021-terminal plan — C3; EXTENDED for M2)** | A runtime migration through the deployer yields objects owned by `striatumd_rw` because the reconcile runs while CREATE is held; after the terminal 0021 the serving role cannot create objects; **AND `owner-ddl apply` on the activation binary — including the FMA-007 reapply branch — leaves 0021 pending so the deploy reconcile can still run while CREATE is held.** | `T-deploy-runtime-object-ownership` (two-role pgtest, non-superuser cluster) as v4, PLUS a NEW arm: on the activation binary at owner frontier 20, run `owner-ddl apply` (incl. forced FMA-007 self-heal) and assert `has_schema_privilege('striatumd_rw','striatumd','CREATE')` stays **true** and `owner_bundle_meta` has no version-21 row, THEN run the deploy and assert it completes. `G-revoke-last`: kill `deploy` between the last runtime reconcile and 0021, re-run, assert completes; ALSO run `owner-ddl apply` on the activation binary before deploy and assert CREATE still held. |
| **F13 (Per-step receipt crash-safe on a STABLE key — N1 + BC-N1)** | A crash between a step's commit and its receipt is reconciled to exactly one receipt per applied step on re-run, keyed on the stored `(plan_hash, step_index)`. | `T-deploy-receipt-crash-resume`. `G-receipt-kill`. |
| **F14 (Immutable plan identity across resume — BC-N1; EXTENDED for M1 already-applied verification)** | Plan identity is materialized before step 0 and resume reads the stored transcript, so a mid-plan crash that moves the live frontier does NOT change `plan_hash` or renumber `step_index`; **AND resume runs `VerifyStoredTranscript` over the WHOLE transcript before applying — so a binary whose ALREADY-APPLIED step bytes differ (not just pending) halts `deploy_plan_binary_mismatch`.** | `T-deploy-plan-hash-resume-after-step` (kill after step 0 AND step 1; assert reuse of `plan_hash`, preserved `step_index`es, exactly-once prior receipts, green end) PLUS a negative case tampering an **already-applied** stored step's `sha256` → `deploy_plan_binary_mismatch`. `G-plan-resume-kill`. |
| **F15 (Full-transcript verification on resume AND before the finalizer — M1, NEW)** | On every resume and as finalizer step 0, `VerifyStoredTranscript` validates EVERY stored step (already-applied AND pending) against the binary's embedded bytes and every already-applied step against its DB stamp; a binary that disagrees with an already-applied step is forced to `deploy_plan_binary_mismatch` (or `deploy_plan_db_stamp_mismatch`) with NO apply, NO `RecordSchemaFingerprint`, NO `complete` cursor — so a hybrid A-applied/B-expected deploy can NEVER self-record as in-sync. | **`T-deploy-resume-already-applied-byte-mismatch-refuses`**: materialize `H=[runtime:0045 A45, runtime:0046 A46, owner:0021 A21]`; commit step 0 (`A45` + receipt); crash; resume with binary B whose `0045` bytes are `B45` but `0046`/`0021` match; assert `deploy_plan_binary_mismatch`, NO step-1 apply, `RecordSchemaFingerprint` un-called (spy), NO `complete` cursor, non-green doctor, `schema_state` unchanged. **Symmetric owner-step case**: completed non-revoke owner prefix + pre-finalization crash, resume with a binary whose an applied owner bundle's bytes differ → same refusal. **DB-stamp case**: tamper an applied `schema_migrations.sha256` away from the transcript → `deploy_plan_db_stamp_mismatch`, refuse-to-finalize. **Finalizer arm**: drive the cursor to `finalizing` under binary B, run the finalizer, assert step 0 verify FAILS and nothing is written. `G-wrong-binary-resume`: on a real cluster, `kill -9` after a committed runtime step, swap in a binary with divergent embedded SQL for that step, re-run `deploy`, assert it refuses and writes nothing. |
| **F16 (Revoke excluded from EVERY owner-ddl apply route incl. self-heal — M2, NEW)** | The DDL-revoke bundle 0021 is unreachable through `owner-ddl apply`: `OwnerDDLApplyBundles()` excludes it, `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles` guard `isNonRevokeBundle` in-loop, and `ReapplyAllOwnerBundles`'s nil-fallback loads the filtered slice — so even the forced FMA-007 cross-bundle self-heal never commits the revoke; CREATE stays held. | **`T-deploy-revoke-excluded-from-reapply-self-heal`**: embed a synthetic 0021 revoke bundle; **force the cross-bundle dependency self-heal path** (drop an early-bundle object so a `<= 20` bundle fails with `42P01`, triggering `ReapplyAllOwnerBundles`); run `ApplyOwnerBundles`; assert 0021 is NOT applied, `owner_bundle_meta` never records 21, and `has_schema_privilege('striatumd_rw','striatumd','CREATE')` remains **true**. PLUS `TestOwnerDDLApplyExcludesRevokeBundle` (unit): `OwnerDDLApplyBundles()` has no bundle `>= 21`; `OwnerBundles()` DOES (for `revokeEmbedded`/`ExpectedFingerprint`); the nil-fallback resolves to the filtered slice. PLUS a build-time grep test: no production `applyOneOwnerBundle` caller outside the deployer terminal-step path receives a bundle `>= 21`. |

---

## 6. Shadow-first rollout (the P4 build order) — re-anchored + M1 + M2

1. **Migration 0044 `deploy_cursor` + `deploy_plan`** (additive runtime tables, modeled on
   `0043_schema_state.sql:39-52`; `deploy_cursor.state` CHECK includes `finalizing`; `deploy_plan`
   holds the immutable transcript; runtime-owned — passes `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`).
2. **`go/pkg/db/owner.go` M2 surface (lands first, inert):** `DDLRevokeOwnerBundleVersion = 21`,
   `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, the in-loop guards in `applyPendingOwnerBundles` /
   `ReapplyAllOwnerBundles`, the nil-fallback split, and `TestOwnerDDLApplyExcludesRevokeBundle` +
   the build-time grep test (F16). This is inert until 0021 is authored (step 6) and harmless before
   then (the filter is a no-op when no bundle `>= 21` exists).
3. **`go/pkg/db/deploy.go`**: `DeployPlan`, `BuildPlan` (materialization-only, 0021-terminal
   special-case using the FULL `OwnerBundles()`, §3.2), `LoadStoredPlan(plan_hash)` resume reader,
   **`VerifyStoredTranscript` (M1, §3.4a) + the `deploy_plan_binary_mismatch` /
   `deploy_plan_db_stamp_mismatch` typed halts**, `Deployer.Apply` (Q3-A/Q3-B engine + the
   `finalizing` finalizer with `VerifyStoredTranscript` as step 0, §3.4, advisory-locked via
   `MigrationLockKey` `migrations.go:18`), the substrate-ensure preamble, `applyRuntimeStep` (the C3
   ownership reconcile while CREATE held, §3.3b), the per-step + terminal receipt writer. Pure-core +
   DB-integration tests (F1, F2, F4, F8, F9, F10, F12, F13, **F14 extended, F15, F16**) — engine +
   finalization + ownership + immutable-transcript resume + full-transcript verification + per-step
   receipt proven BEFORE any boot path changes.
4. **`runDaemonDeploy`** verb (`daemon.go`) + matrix/authority-guardrail row + `--dry-run` +
   `--abort` + the 0021-activation preflight (§4.3). F3/F5 wiring.
5. **`CheckDeployActivation`** with the universal pre-revoke cursor edge (BC-N2) + the typed
   `awaiting_deploy`/`awaiting_deploy_config` + **`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`**
   halts + the forward-watermark rule (re-anchored 21) in `CheckOwnerBundleWatermark` + the `main.go`
   `errors.As` arms. Decoupled boot path (`ConnectAndVerify`) behind `STRIATUM_DEPLOY_DECOUPLED`
   (default OFF). Lands inert (F11, F3, F5).
6. **`doctor schema_deploy_unrecorded`** block — per-step tightened, enumerated from the stored
   transcript, **+ the M1 stamp/byte divergence WARN** (F7, F4, F15 doctor arm).
7. **Owner bundle 0021** (DDL revoke) — authored, deploy-plan-terminal and excluded from every
   `owner-ddl apply` route via `OwnerDDLApplyBundles` (`LatestOwnerBundleVersion` stays 20), tested in
   two-role pgtest (F6, F12, F16); activation is the operator choreography (§4.3).

Each phase additive and reversible. Self-record before enforce; detection before
mutation-relocation; the verb + finalizer + full-transcript verifier + ownership reconcile +
immutable-transcript resume + the non-revoke filter all proven before the boot path leans on them.

---

## 7. Boundary & explicit deferrals (carry-forward)

- **In P4:** the deployer verb, the deploy plan + `deploy_cursor` (incl. `finalizing`) + the
  immutable `deploy_plan` transcript (BC-N1) + per-step + terminal receipt, the idempotent finalizer
  (C1) **with the `VerifyStoredTranscript` pre-finalizer gate (M1)**, the C3 ownership reconcile with
  the revoke (0021) sequenced last, the **single non-revoke `owner-ddl apply` filter + embed/listing
  split (M2)**, lifting `ApplyMigrations` out of serve-boot (flagged), the activation interlock + the
  universal pre-revoke cursor edge (BC-N2) + forward-watermark rule, serving-role create-DDL
  revocation (bundle 0021, deploy-terminal).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`, `rehearsal_receipt.v1`, fidelity
  tiering, full-data clone, expand/contract reshape primitive, lock-budget guardrail. P4 names the
  seams P5 builds on but ships none of it. Full capability revocation (owner re-owns runtime tables)
  is a named follow-up beyond P4 (§4.1), not assumed.
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the single writer, no
  hosted services / replicas / external persistence.

---

## 8. Open tensions handed to the falsifiers (attack these hardest)

- **M1 / F15 — the verification set and the DB-stamp oracle.** Is `VerifyStoredTranscript`'s byte
  check truly over EVERY step (no off-by-one excluding the terminal 0021 or step 0)? For an
  already-applied runtime step, is `schema_migrations.sha256` (the column `verifyRecordedHashTx`
  reads, `migrations.go:275`) always populated at the point the deploy stamps it, so the DB-stamp
  check has an oracle? For an already-applied owner step, is `owner_bundle_meta.sha256`
  (`owner.go:529`) the right oracle, and does a bundle re-stamped `ON CONFLICT DO NOTHING` ever carry
  a *stale* sha that would false-positive `deploy_plan_db_stamp_mismatch` on a legitimate resume?
- **M1 / F15 — the finalizer step-0 gate vs. the legacy self-record.** The deployer finalizer is
  gated by `VerifyStoredTranscript`, but `connection.go:399` (legacy `ConnectAndMigrate`) self-records
  unconditionally. Is the invariant scoping (legacy path = running binary recording its OWN
  just-applied schema, no transcript) airtight, or is there a boot where a deployer-aware binary takes
  the legacy `ConnectAndMigrate` path over a database that DOES have a `deploy_cursor`/transcript and
  self-records around the gate? (Intended close: the universal `CheckDeployActivation` edge halts any
  non-`complete` cursor before `RecordSchemaFingerprint` on both paths, §3.3a — verify it actually
  precedes `:399` in `ConnectAndMigrate`, not only `ConnectAndVerify`.)
- **M2 / F16 — the two barriers and the synthetic-0021 test.** Are BOTH barriers (the
  `OwnerDDLApplyBundles()` slice AND the in-loop `isNonRevokeBundle` guard) actually present on the
  FMA-007 path, so removing one in review still fails the test? Does the forced-self-heal pgtest truly
  reach `ReapplyAllOwnerBundles` (it must inject a real `42P01`/`42703`/`42883`/`42704` so
  `isCrossBundleDependencyError` `owner.go:367-374` fires), or does it merely test the pending loop?
  Does any caller pass a hand-built `bundles` slice (not via `OwnerDDLApplyBundles`) into
  `ReapplyAllOwnerBundles` that could smuggle 0021 past the nil-fallback (relying solely on the
  in-loop guard)?
- **M2 — `revokeEmbedded` vs. `OwnerDDLApplyBundles` coherence.** `revokeEmbedded` must see 0021
  (full `OwnerBundles()`) while `owner-ddl apply` must not (`OwnerDDLApplyBundles()`). Is there any
  code path that derives `revokeEmbedded` from the filtered slice (which would make it always false on
  the activation binary), or derives the apply slice from the full loader (re-opening M2)?
- **BC-N1 / F14 (carry-forward) — materialize-before-step-0 + concurrency.** Any crash window between
  observing the base frontiers and committing `deploy_plan` + `in_progress(0)`? Does `ON CONFLICT
  (plan_hash) DO NOTHING` + the singleton cursor + the `MigrationLockKey` advisory lock truly
  serialize two racing deployers?
- **BC-N2 / F11 (carry-forward) — universal edge vs. fresh-DB bring-up.** Does the edge ever fire on a
  path that MUST serve (inert binary's first boot before 0044; fresh DB with no cursor; legitimate
  `complete`)? Is the `none`-vs-`idle`-vs-non-`complete` distinction airtight?
- **C3 / F12 (carry-forward) — the catalog diff.** Does the before/after oid diff miss any object kind
  a runtime migration could create (a TYPE, a trigger function, a `GENERATED … AS IDENTITY` sequence)?
- **§4.1 — the residual capability.** Revoking only `CREATE ON SCHEMA` leaves `striatumd_rw` able to
  `ALTER` its own tables. Is "the serve path issues no DDL" a sufficient P4 close, or can a falsifier
  construct a serve-path DDL (a DML trigger / `DO` block that issues DDL)?
- **Q4 — bootstrapping.** Any path where plain-verb-now forecloses the run-shape later that the three
  seams (§2) don't keep open?

---

## 9. Anchor table (re-verified against current `main`, HEAD `2e482077`)

> `git diff --stat 5c5cb36a HEAD -- go/` is empty (the two intervening operator commits are
> docs/workflow only), so every v4 source anchor holds; the M1/M2 cores were re-verified line-by-line.

| Target | Anchor |
| --- | --- |
| serve-boot mutation site (lift this) | `go/pkg/db/connection.go:332` `ConnectAndMigrate`; apply `:353`; watermark `:349`; drift gate `:376-383`; shadow log `:384-393`; self-record `:399` ("the only writer of schema_state") |
| boot fork / `--migrate` / typed-halt mapping | `authority_bootstrap.go:181,193`; `go/cmd/striatumd/main.go:76`; typed halts `:208-227` (`AwaitingOwnerDDLError` arm `:208-213`, `SchemaDriftError` arm `:222-227`; add `awaiting_deploy`/`awaiting_deploy_config`/`deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch` arms) |
| deployer command site | `daemon.go:67-81` dispatch (`install|uninstall|status|migrate-db|owner-ddl` — add `deploy`); `runDaemonOwnerDDL` `:90` (admin DSN `:115`); `runDaemonMigrate` `:167`; `EnvDaemonAdminDBURL` `:24` |
| runtime apply engine to reuse / wrap (C3) | `migrations.go:121` `ApplyMigrations` (`current = migration.Version` `:171`); `applyOne` `:304-355` (BeginTx `:305` → SQL `:315` → `schema_migrations` stamp `:320` → `schema_meta` `:332` → in-tx hash verify `:347` → Commit `:350`; **no `SET ROLE`/`ALTER OWNER`/grant** — the C3 gap `applyRuntimeStep` wraps); `verifyRecordedHashTx` `:274` (reads `SELECT sha256 FROM schema_migrations WHERE version` `:275` — the M1 runtime DB-stamp oracle); lock `:18`; preflight `:158`; frontier `:17` (=43; new substrate migration = 0044); `MigrationSHASet` `:197` (the M1 runtime byte oracle); `//go:embed sql/*.sql` `:21` |
| owner-bundle engine / per-tx atomicity / watermark + forward-tolerance (C2) / 0021 exclusion (M2/C3) | `owner.go:265-303` `ApplyOwnerBundles` (loads slice `:269` — **change to `OwnerDDLApplyBundles()`**; self-heal branch `:277-302`, cross-bundle check `:283`, `ReapplyAllOwnerBundles` call `:289`); `:309-322` `applyPendingOwnerBundles` (`<= current` `:312`, `applyOneOwnerBundle` `:315` — **add in-loop `isNonRevokeBundle` guard**); `:332-351` `ReapplyAllOwnerBundles` (nil-fallback `:336-342` — **load `OwnerDDLApplyBundles()`**; unbounded loop `:344-349` — **add in-loop guard**); `:367-374` `isCrossBundleDependencyError` (the FMA-007 trigger; SQLSTATEs `:357-362`); `:511-541` `applyOneOwnerBundle` (one tx, SQL `:522`, stamp last `:528-535` — the terminal-0021 step reuses this; `owner_bundle_meta.sha256` is the M1 owner DB-stamp oracle); `CheckOwnerBundleWatermark` `:124-154` (tolerate-forward `:151-153` — the forward-watermark rule hooks here); `OwnerBundles()` `:197-224` (the FULL loader — keep for `revokeEmbedded`/`ExpectedFingerprint`/`BuildPlan`; **add `OwnerDDLApplyBundles()`/`isNonRevokeBundle`/`DDLRevokeOwnerBundleVersion = 21` siblings**); `OwnerBundle.SHA256()` `:190-194` (the M1 owner byte oracle); `LatestOwnerBundleVersion = 20` `:23` (**stays 20**); `RequiredOwnerBundleVersion` `:35` (= 20, **stays 20**); `revokeEmbedded` = 0021 file in `ownerBundleFS` (`//go:embed` `:156`) |
| pure read consuming the FULL owner-bundle loader (must keep seeing 0021) | `owner_runtime_ownership.go:51-52` `RuntimeOwnedTablesAlterable()` → `OwnerBundles()` (derives the runtime-table allowlist from bundle SQL; a REVOKE-only 0021 adds nothing; NOT an apply path) |
| current owner-bundle frontier (NOT the revoke) | `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql` (`GRANT SELECT` on `owner_bundle_meta`, #581 / RFC 0142 Layer 2; label `owner.go:179`) — the highest non-revoke bundle; the DDL-revoke is the NEW 0021 |
| C3 prerequisite the revoke-last fix relies on | `0018_runtime_table_ownership_transfer.sql:64-66` (prereq: `striatumd_rw` MUST hold CREATE), `:98` (`GRANT CREATE ON SCHEMA striatumd TO striatumd_rw` before the `:102` transfer loop); `0019_supervisor_pointer_runtime_ownership.sql:53-80` (same) — 0018 is in the non-revoke owner prefix, so CREATE is held when the runtime reconciles run |
| fingerprint contract (the M1 mechanism + bind plan_hash to) | `schema_drift.go:83-100` `ExpectedFingerprint` (hashes EMBEDDED FILE BYTES via `MigrationSHASet()` + each `OwnerBundle.SHA256()`, keyed by version — a different-bytes binary computes a different fingerprint); `:105-137` `composeFingerprint` (reused for `plan_hash`); `:145-161` `LiveFingerprint` (reads the self-recorded singleton — NO recompute); `:171-195` `RecordSchemaFingerprint` (UPSERTs the running binary's expected value — the M1 write the finalizer gates); `:254-274` `CheckSchemaDrift`; `EnvSchemaDriftRefuse` `:28` |
| `deploy_cursor` + `deploy_plan` table model (incl. `finalizing` + immutable transcript) | `0043_schema_state.sql:39-44` (singleton CHECK), `:46-52` (`striatumd_rw` GRANT `DO` block) — model for both new 0044 tables |
| per-step / terminal receipt SD fn | `0001_authority_phase0.sql:152` (`append_audit_row` SD fn), `:223` (`REVOKE ALL … FROM PUBLIC`), `:237` (runtime direct `audit_log` INSERT revoked), `:238` (`GRANT EXECUTE … TO striatumd_rw`) — runs in the caller's transaction, so the per-step receipt joins the owner-connection step tx |
| doctor block model (per-step tightened, transcript-enumerated, M1 stamp/byte WARN) | `go/pkg/reads/doctor_schema_drift.go:26-77` (`schemaDriftDoctorBlock`; `ExpectedFingerprint` call `:32`) |
| FMA-007 self-heal test to extend (M2) | `owner_pg_test.go:219` `TestReapplyAllOwnerBundlesIsOrderedAndIdempotent` (calls `ReapplyAllOwnerBundles(ctx, runner, nil, "test")` `:229` — exercises the nil-fallback the M2 fix re-points to `OwnerDDLApplyBundles()`) |
| two-role fixture for F12 / F16 | `go/pkg/pgtest/two_role.go:130` `ApplyOwnerBundles` (non-superuser two-role bootstrap) |

<sub>Holder artifact (v5 revision) for the RFC 0142 P4 falsification-gate design run. Resolves cycle-4
findings M1 (full stored-transcript byte verification on resume INCLUDING already-applied steps + the
already-applied DB-stamp check, the SAME `VerifyStoredTranscript` gate before the C1 finalizer
self-records, ANY mismatch → `deploy_plan_binary_mismatch`/`deploy_plan_db_stamp_mismatch`
DB-untouched; `T-deploy-resume-already-applied-byte-mismatch-refuses` / F15 + the symmetric owner-step
case + F14 extended) and M2 (a single non-revoke filter `OwnerDDLApplyBundles()` +
`isNonRevokeBundle` excluding 0021 from EVERY `owner-ddl apply` route incl. `ReapplyAllOwnerBundles` /
the FMA-007 self-heal + its nil-fallback; the embed/listing split so `revokeEmbedded`/`ExpectedFingerprint`
see 0021 but `owner-ddl apply` never iterates it; `T-deploy-revoke-excluded-from-reapply-self-heal` /
F16 + extended F12 / `G-revoke-last`), does the proactive hardening (every owner-bundle apply path and
every fingerprint/self-record path named against current `main` HEAD `2e482077`; UNIVERSAL INVARIANTS A
and B stated as executable named requirements), and carries BC-N1 + BC-N2 + C1 + C2 + C3 forward
verbatim and unregressed. The cycle-5 falsifiers (atomicity/fingerprint lens; owner-ddl/migration-safety
lens) re-attack §3.4a, §3.2a, §4.5, §5 (esp. F15, F16, extended F12/F14), and the tensions in §8; the
adjudicator's collaboration ledger — not falsifier completion — decides whether the gate clears.</sub>

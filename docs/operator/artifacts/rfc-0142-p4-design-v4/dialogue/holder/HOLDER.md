# HOLDER (revised, v4) — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: holder-author-001

> **This is the FOURTH REVISION (v4).** Design-v1 (`rfc-0142-p4-design`) returned
> `needs_revision` with three findings C1/C2/C3. Design-v2 resolved C1 + C2. Design-v3
> (`rfc-0142-p4-design-v3`) **resolved C3** (ownership transfer, revoke-last — both
> falsifiers conceded the design-level answer) and **closed the immediate N1 hole** (the
> per-step receipt now joins the owner-connection step transaction), BUT returned
> `needs_revision` **again** on two independent grounds, each squarely in the Q3
> correctness core:
>
> - **BC-N1** — the per-step `(plan_hash, step_index)` exactly-once receipt key is **NOT
>   stable across a crash-resume**, because `BuildPlan(applied_owner, applied_runtime)`
>   rebuilds the PENDING-delta plan from the LIVE frontiers, which advance at each
>   committed step, and `deploy_cursor` persisted no immutable transcript;
> - **BC-N2** — the C3 revoke-last ordering **opens a pre-terminal-revoke
>   incomplete-deploy serve window** that a no-revoke-bundle deployer-aware binary does
>   not gate (it runs neither `CheckDeployActivation`, inert because `!revokeEmbedded`,
>   nor the forward-watermark rule, inert below the revoke watermark).
>
> This spec **starts from the v3 `HOLDER.md`**
> (`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/holder/HOLDER.md`) and is a
> **proper revision**: it **resolves BC-N1 and BC-N2** per the prescribed fixes in the v3
> collaboration ledger §4 ("What the revision must fix") and `SEED.md`, and **carries C1,
> C2, and C3 forward unregressed**. §0.1 below is the auditable resolution map; the
> cycle-4 falsifiers should verify each item there, then re-attack §5 (esp. the new
> **F14**, the extended **F11**, and **F13**) and the tensions in §8.
>
> It is a falsifiable implementation spec for RFC 0142 **P4 only** — the one-shot deployer
> that lifts schema mutation out of serve-boot — that `rfc-0142-p4-build` can execute
> contract-first (TDD). RFC 0142 is accepted (D258); this does NOT re-open the five-layer
> design. Primary source:
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (esp. Layer 3, the
> Phasing P4 row, Open Questions 3 + 4). **Every anchor below is re-verified against
> current `main` (HEAD `5c5cb36a`); several frontiers MOVED since v3 was authored** — most
> consequentially, **bundle 0020 is now taken by
> `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql`** (a `GRANT SELECT`, #581 /
> RFC 0142 Layer 2) and **`LatestOwnerBundleVersion == 20`, `RequiredOwnerBundleVersion ==
> 20`** (`go/pkg/db/owner.go:23,35`), so **the DDL-revoke bundle renumbers to `0021`** and
> **the pre-revoke window is at `applied_owner == 20`** (not 19). The C3 mechanism is
> unchanged; only the ordinal moves. See the §0.2 re-anchor map and the anchor table.

## 0. Root reframe held

Schema mutation must stop being an **implicit side effect of the serving process's
restart** and become an **explicit, ordered, resumable, provenance-tracked operation owned
by a dedicated deployer**. Then the serving daemon holds **zero create-DDL on the serving
path**, "restart force-commits a half-applied deploy" becomes structurally impossible, and
a bad migration can never wedge the single writer on boot. Everything below serves that one
move; nothing pulls in P5 (rehearse / expand-contract / fidelity tiering / clone = Q1/Q2).

---

## 0.1 Addressing the design-v3 findings (the gating requirement)

The two binding v3 findings (BC-N1, BC-N2) are each resolved by a concrete sub-protocol —
not a claim. C1, C2, and C3 are carried forward **verbatim** from the v3 HOLDER and
re-confirmed unregressed (re-anchored 0020→0021, 19→20). The "where" column names the
section that specifies the fix and the named test that would refute it.

| Finding | v3 status | v4 disposition | Where / refuting test |
| --- | --- | --- | --- |
| **BC-N1** per-step `(plan_hash, step_index)` receipt key not stable across resume (plan rebuilt from moving live frontiers; no durable transcript) | **OPEN** (falsifier_1; Q3/R4 core) | **RESOLVED: plan identity is materialized ONCE into an immutable, durable transcript BEFORE step 0, and resume reads that transcript — never `BuildPlan` over the moved frontiers.** A new runtime-owned `deploy_plan` table stores, keyed by `plan_hash`, the `base_owner_version`/`base_runtime_version`, the target frontiers, the terminal-revoke `step_index`, and the full ordered `[{step_index, step_id, role, sha256, transactional}]` transcript — written in the SAME transaction that sets `deploy_cursor → in_progress(0)`, before any frontier-advancing DDL. Resume loads `deploy_plan` by `cursor.plan_hash`, verifies the embedded bytes of the not-yet-applied steps still match the stored `sha256`, and resumes at the next step **in the stored transcript**. `(plan_hash, step_index)` and every prior receipt are therefore stable across the moved frontier. `BuildPlan(base_owner, base_runtime)` is now used **only to materialize a fresh plan** when no incomplete cursor exists; it is never the resume authority. The holder's old "content-addressed ⇒ stable" claim is no longer circular: identity is a durable fact, not a recomputation. | §1.2 (`deploy_plan` table), §1.3 (stored-transcript classification), §3.2 (materialize-once), §3.4 (receipt keyed off the stored transcript) / **`T-deploy-plan-hash-resume-after-step` (F14, new)** |
| **BC-N2** C3 revoke-last opens a pre-revoke incomplete-deploy serve window a no-revoke binary does not gate | **OPEN** (falsifier_2; decoupling/migration-safety core) | **RESOLVED: `deploy_cursor` is made AUTHORITATIVE before the terminal revoke via one universal hard edge.** `CheckDeployActivation` is **no longer gated on `revokeEmbedded`**: EVERY deployer-aware binary — including the no-revoke landing binary — reads `deploy_cursor` immediately after `CheckOwnerBundleWatermark` and **before `ApplyMigrations` and before `RecordSchemaFingerprint`** in both boot paths, and **halts `awaiting_deploy` DB-untouched whenever the cursor row exists and `state != complete`**, regardless of `revokeEmbedded` / watermark. This edge holds at `applied_owner == 20` (the v4 pre-revoke frontier), exactly the window the C3 ordering opens. It does NOT block a fresh/idle DB (absent cursor table or `state == idle` on the legacy path serves), so it does not regress C3 (the deploy still completes; no stranded `ALTER … OWNER`) nor wedge a clean boot. The existing C2 revoke-embedding arms (`awaiting_deploy_config`, absent-cursor-as-incomplete for the revoke binary, the forward-watermark rule re-anchored to 21) are preserved underneath the universal edge. | §3.3a (universal cursor edge), §4.2 (typed halts + forward-watermark re-anchored), §4.4 (no pre-revoke serve) / **`F11` extended + `G-old-binary-refuse` extended** |
| **C1** finalization boundary | **RESOLVED** (v2; both falsifiers conceded; intact in v3) | **CARRIED FORWARD VERBATIM, unregressed.** The distinct `finalizing` cursor state, the idempotent finalizer (`complete` receipt guarded on `(plan_hash, state=complete)` → `schema_state` fingerprint UPSERT → advance `finalizing → complete` LAST), the §1.3 resumable-finalization row, the operator pin (no single finalization tx spans owner+runtime connections), and F10 are preserved exactly. Kept **coherent** with BC-N1 (the finalizer reads the stored transcript) and BC-N2 (a `finalizing` cursor is non-`complete`, so the universal edge refuses to serve it). | §1.2, §1.3, §3.4 / **`T-deploy-resume-finalization-crash` (F10)** |
| **C2** fail-closed activation edge | **RESOLVED at the edge** (v2; intact in v3) | **CARRIED FORWARD VERBATIM, unregressed — and now globally sufficient.** `CheckDeployActivation` after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations`, the typed `awaiting_deploy`/`awaiting_deploy_config` halts, the forward-watermark rule, `RequiredOwnerBundleVersion` kept at **20** (its current-`main` value; NOT advanced to the revoke's 21), and F11 are preserved. The v3 gap — that the edge gated only revoke-embedding binaries — is closed by the BC-N2 universal edge, which is an **addition** to (not a replacement of) the existing edge. | §3.3a, §4.2, §4.3 / **`T-deploy-revoke-activation-ordering` (F11, extended)** |
| **C3** ownership transfer (revoke-last) | **RESOLVED** (v3; both falsifiers conceded) | **CARRIED FORWARD VERBATIM, re-anchored 0020→0021.** The DDL-revoke bundle (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) is special-cased OUT of the owner prefix and EXCLUDED from `owner-ddl apply` (a dedicated `DDLRevokeOwnerBundleVersion = 21`; `LatestOwnerBundleVersion` / `ApplyOwnerBundles` bounded `<= 20`), and applied as the TERMINAL plan step after every runtime ownership reconcile, so each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE and the committed steady state still denies it. The ordinal moves 0020→0021; the special-case + terminal-ordering + exclude-from-`owner-ddl apply` logic is unchanged. | §3.2, §3.3b, §4.4 / **`T-deploy-runtime-object-ownership` (F12) + `G-revoke-last`** |

**Carried forward unchanged (v1/v2/v3 ledgers — do not re-litigate):** Q4 (plain verb +
three seams, §2); the per-step Q3-A/Q3-B mid-step resumability **body** + step taxonomy
(§1.1, the `in_progress`/`step_committed` body of §1.2 — BC-N1 changes only where plan
**identity** lives, not the per-step schema/cursor body); the shadow-first decoupling shape
(default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2 watermark
interlock + P3 drift gate, the embed-FS-derived plan, the hash-chained receipt, `doctor
schema_deploy_unrecorded`). The v4 revision touches only **plan-identity durability
(BC-N1)** and the **universal pre-revoke cursor edge (BC-N2)**.

## 0.2 v3 → v4 re-anchor map (frontiers MOVED; re-verify, do not silently inherit)

| v3 fact (now false against `main`) | v4 fact (current `main`, HEAD `5c5cb36a`) |
| --- | --- |
| DDL-revoke bundle = **0020** | DDL-revoke bundle = **0021** (0020 is `0020_owner_bundle_watermark_read.sql`, a `GRANT SELECT`, #581) |
| `LatestOwnerBundleVersion` = 19, `Required` = 19 | `LatestOwnerBundleVersion = 20`, `RequiredOwnerBundleVersion = 20` (`owner.go:23,35`); both **stay 20** (revoke 0021 is deploy-plan-only, NOT in the watermark frontier) |
| `DDLRevokeOwnerBundleVersion = 20` | `DDLRevokeOwnerBundleVersion = 21` (new constant) |
| pre-revoke window at `applied_owner == 19` | pre-revoke window at `applied_owner == 20` |
| forward-watermark rule fires at `applied_owner >= 20` | forward-watermark rule fires at `applied_owner >= 21` |
| `revokeEmbedded` = 0020 file in `ownerBundleFS` | `revokeEmbedded` = **0021** file in `ownerBundleFS` |
| runtime frontier 0043; new migration ≥ 0044 | unchanged: `LatestDaemonDBVersion = 43` (`migrations.go:17`); `deploy_cursor`/`deploy_plan` migration **= 0044** |

---

## 1. Decision Q3 — How atomic is "atomic"? (the hard correctness core)

**Claim (Q3-DECISION).** The **per-step-atomic + resumable-cursor** contract is sufficient
for *every* owner+runtime interleaving P4 ships, **provided**: (a) plan identity is an
**immutable durable fact materialized before step 0** and resume reads it, never the moving
live frontier (BC-N1); (b) the two per-step sub-rules (Q3-A/Q3-B) hold for **both the
schema side effect AND the per-step receipt**, keyed on the **stored transcript's**
`(plan_hash, step_index)`; (c) the finalization boundary is closed by a distinct
`finalizing` state + an idempotent finalizer (C1); and (d) **no boot — including a
no-revoke deployer-aware boot — can serve while the cursor is non-`complete`** (BC-N2). No
step needs a cross-step transaction; no finalization transaction spans the owner+runtime
connections (structurally impossible — §3.4); no per-step provenance write is left in a
crash window the resume cannot reconcile to exactly-one receipt against a stable key
(§3.4); and no resume recomputes the plan from a moved frontier (§3.2).

- **Q3-A — transactional step (carry-forward body; receipt keyed off the stored
  transcript).** If a step's DDL can run inside a transaction, the **cursor advance, the C3
  ownership reconcile, the version stamps, AND the per-step receipt commit in the SAME
  transaction as the step** (over the owner connection — §3.3b). Crash → the whole
  transaction rolls back → cursor unchanged, no receipt → resume re-enters the step
  cleanly. **`step_committed(k)` is durable iff its receipt is durable**, and `k` is the
  index in the **stored** transcript, stable across the moved frontier.
- **Q3-B — idempotent-resume reconciler where the step is NOT transactional (carry-forward
  body; receipt keyed off the stored transcript).** `CREATE INDEX CONCURRENTLY`, `ALTER
  TYPE … ADD VALUE`, and the handful of auto-committing `ALTER`s cannot share a transaction
  with the cursor or the receipt. The deployer writes an **`in_progress(k)` marker BEFORE
  the step** (where `k` is the **stored-transcript** index), runs the step in its
  **idempotent / self-reconciling** form, then in one owner-connection reconcile
  transaction **appends exactly one receipt keyed on the stored `(plan_hash, step_index)`
  and writes `step_committed(k)`**. Crash window is *between* the step's own commit and the
  reconcile; on resume the cursor reads `in_progress(k)`, loads the stored transcript, and
  re-enters step `k` through its reconciler.

### 1.1 Step taxonomy (the closed set P4 ships) — unchanged from v1/v2/v3 (re-anchored)

| Class | Examples | Transactional? | Protocol | Idempotent form |
| --- | --- | --- | --- | --- |
| **T-DDL** runtime | `CREATE TABLE`, additive `ALTER … ADD COLUMN`, `CREATE INDEX` (non-concurrent), the new runtime migrations themselves | Yes | **Q3-A** (step + ownership-reconcile + version stamps + cursor + **receipt** in one tx) | `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`; re-run is a no-op |
| **T-DDL** owner | owner-bundle DDL (authority/grant/CHECK changes), **incl. the terminal DDL-revoke bundle 0021** | Yes (each bundle applies in one tx that stamps `owner_bundle_meta` last — `owner.go:511-541`) | **Q3-A**, reusing the existing per-bundle transaction; cursor advance + **receipt** join that tx | bundles already idempotent (`IF NOT EXISTS` + `ON CONFLICT (version) DO NOTHING`, `owner.go:528-535`) |
| **NT-DDL** | `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, `DROP INDEX CONCURRENTLY` | No | **Q3-B** (pre-marker → step → reconcile: keyed receipt + post-marker) | `CREATE INDEX CONCURRENTLY IF NOT EXISTS`; **resume reconciler** drops a left-behind `INVALID` index then recreates; `ADD VALUE … IF NOT EXISTS` (PG12+) |
| **DML-backfill** (bounded in P4; full online backfill is P5) | a one-shot data normalize a runtime migration carries | Yes if small; batched otherwise | Q3-A small / Q3-B batched | `WHERE <not-yet-done>` predicate; re-run resumes from the unprocessed tail |

> The only genuinely hard per-step corner is NT-DDL: **a half-built `CREATE INDEX
> CONCURRENTLY` leaves `pg_index.indisvalid = false`**; the fingerprint MUST classify that
> as "incomplete, resume", never "unknown drift, panic" — see §1.3. **NT-DDL never creates
> a new ownable object whose owner is wrong:** a CONCURRENTLY index inherits its table's
> owner (already `striatumd_rw`), and `ALTER TYPE … ADD VALUE` creates no relation. So the
> C3 ownership reconciliation (§3.3b) is needed only on transactional object-creating
> steps. The **terminal DDL-revoke bundle 0021 is a T-DDL owner step** (the existing
> per-bundle transaction, `owner.go:511-541`), so its receipt commits atomically.

### 1.2 `deploy_cursor` + the immutable `deploy_plan` transcript (BC-N1) and crash-resume semantics (C1)

Both are **new runtime-owned tables**, created by runtime migration **0044** (modeled
byte-for-byte on `0043_schema_state.sql:39-52`: `CHECK (id='singleton')` PK where singleton,
the `striatumd_rw` GRANT `DO` block at `0043:45-52`). Migration 0044 is additive,
runtime-owned, carries **no owner DDL** (passes `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`).

**`deploy_cursor` (singleton) — the live position:**

```
deploy_cursor(
  id            text PK CHECK (id='singleton'),
  plan_hash     text NOT NULL,         -- FK-by-value into deploy_plan.plan_hash; the plan this cursor advances
  state         text NOT NULL,         -- enum below (CHECK constraint)
  step_index    int  NOT NULL,         -- 0-based index INTO the stored transcript (read, not derived)
  step_id       text NOT NULL,         -- e.g. "runtime:0045" / "owner:0021" (denormalized from the transcript for fast classification)
  updated_at    timestamptz NOT NULL DEFAULT now()
)
```

**`deploy_plan` (append-only, keyed by `plan_hash`) — the IMMUTABLE transcript (BC-N1, NEW):**

```
deploy_plan(
  plan_hash              text PK,        -- sha256 of the canonical (base + ordered transcript); the durable plan IDENTITY
  base_owner_version     int  NOT NULL,  -- applied owner frontier observed at materialization
  base_runtime_version   int  NOT NULL,  -- applied runtime frontier observed at materialization (post substrate-ensure)
  target_owner_version   int  NOT NULL,  -- owner frontier after the whole plan (includes the revoke's ordinal 21 when present)
  target_runtime_version int  NOT NULL,  -- runtime frontier after the whole plan
  revoke_step_index      int,            -- step_index of the terminal revoke step, or NULL when no revoke is in this plan
  step_count             int  NOT NULL,
  steps                  jsonb NOT NULL, -- ordered [{step_index, step_id, role, sha256, transactional}], the revoke last when present
  created_at             timestamptz NOT NULL DEFAULT now()
)
-- INSERT-once, never UPDATE: a plan_hash row is immutable. A re-run with the same plan_hash
-- finds its row present (INSERT … ON CONFLICT (plan_hash) DO NOTHING) and READS it.
```

`state ∈ { idle, in_progress, step_committed, finalizing, complete, aborted }`. The
**`finalizing` state (C1)** is preserved. Lifecycle for a plan of N steps — where, under
§3.2, the **terminal step N-1 is the DDL-revoke bundle 0021 whenever it is in the plan**
(C3) — with the **BC-N1 materialization** as the first durable write:

```
materialize(plan_hash, transcript) + idle─▶in_progress(0) ─▶ step_committed(0) ─▶ … ─▶ step_committed(N-1=0021) ─▶ finalizing ─▶ complete
   [deploy_plan row + cursor=in_progress(0) in ONE tx, BEFORE step 0 mutates the frontier]      │
                                                                                                 └─(operator abort / fatal non-resumable error)──▶ aborted
```

- **Materialization (BC-N1, the new first durable write).** After the substrate-ensure
  preamble (§3.2) guarantees `deploy_plan`/`deploy_cursor` exist and observes the base
  frontiers, the deployer computes `BuildPlan(base_owner, base_runtime)` **once**, derives
  `plan_hash`, and in **one transaction** INSERTs the immutable `deploy_plan` row and sets
  `deploy_cursor → in_progress(0)`. This commits **before** step 0's frontier-advancing
  DDL. Crash before this commit → neither row exists → re-run materializes fresh (no
  partial). Crash after this commit, before step 0 → cursor `in_progress(0)`, no step-0
  receipt, frontier unmoved → resume re-enters step 0.
- **Q3-A step (carry-forward body; receipt keyed on the stored transcript):** the
  transition `in_progress(k) → step_committed(k)`, the step DDL, the C3 ownership reconcile
  (§3.3b), the version stamps, **and the per-step receipt** are **one transaction**. Resume
  reads `step_committed(k)`, loads the stored transcript, advances to `k+1`.
- **Q3-B step (carry-forward body; receipt keyed on the stored transcript):**
  `step_committed(k-1) → in_progress(k)` (marker), the step's auto-commit, then one
  reconcile tx that appends the keyed receipt and writes `in_progress(k) →
  step_committed(k)`. Crash resume reads `in_progress(k)` and re-enters step `k` through
  its reconciler.
- **Finalization (C1 — carry-forward verbatim).** When `step_index == N-1` (the terminal
  0021 step, if present) is `step_committed`, the deployer advances the cursor to
  **`finalizing`** in one commit, then runs the **idempotent finalizer** (§3.4): (1) append
  the **`complete` deploy receipt** (owner connection, `append_audit_row`,
  `0001_authority_phase0.sql:152,238`) — guarded idempotent: skip if a receipt for
  `(plan_hash, state=complete)` already exists; (2) **`RecordSchemaFingerprint`**
  (`schema_drift.go:171`) UPSERT into `schema_state` (idempotent; deterministic
  `ExpectedFingerprint()` `schema_drift.go:83`, == this plan's recorded fingerprint by
  §3.2); (3) advance `finalizing → complete` **last**. Because `complete` is written **only
  after** receipt and fingerprint are durable — **and, by §3.2, only after 0021 has
  committed as step N-1** — a `complete` cursor proves the deploy is fully finalized **and**
  the steady-state CREATE-revoke is in force. A crash anywhere in finalization leaves the
  cursor at `finalizing`, and the finalizer is idempotent, so re-run repairs exactly once.

### 1.3 The load-bearing disambiguation (C1 + BC-N1 + BC-N2)

`deploy_cursor` is the **second signal** alongside the drift gate's `LiveFingerprint ==
ExpectedFingerprint`. This is the single most important falsifiable artifact in the spec.
The **`finalizing` row (C1)** and the `state == complete` tightening are preserved verbatim
from v3; the **stored-transcript rows (BC-N1)** and the **universal-edge note (BC-N2)** are
new.

| `deploy_cursor.state` | `plan_hash` vs the **stored transcript** | fingerprint | classification | action |
| --- | --- | --- | --- | --- |
| `complete` | `deploy_plan[plan_hash]` present; == this binary's `ExpectedFingerprint` set | match | **in sync** | serve |
| `in_progress` / `step_committed` | `deploy_plan[plan_hash]` present; not-yet-applied steps' `sha256` match this binary's embedded bytes | (any) | **incomplete, resume off the STORED transcript** (BC-N1) | refuse-to-serve `awaiting_deploy`; `striatum daemon deploy` resumes at `cursor.step_index` in the stored transcript — **never recomputes `BuildPlan` from the moved frontier** |
| `in_progress` / `step_committed` | **`deploy_plan[plan_hash]` present but the cursor's `plan_hash` ≠ the binary's freshly-computed pending plan** (because the frontier advanced under the interrupted deploy) | (any) | **incomplete, resume with the STORED plan** (BC-N1, the bucket §1.3 used to mis-classify) | refuse-to-serve `awaiting_deploy`; resume off the stored transcript by `cursor.plan_hash`; the divergence from the pending plan is **expected and recoverable**, NOT drift |
| **`finalizing`** | `deploy_plan[plan_hash]` present; == expected | **(any — match or mismatch)** | **resumable finalization** (C1) | refuse-to-serve `awaiting_deploy`; re-run runs the idempotent finalizer; **NEVER serve, NEVER genuine-drift** |
| `in_progress` / `step_committed` / `finalizing`, with **`deploy_plan[plan_hash]` ABSENT**, OR a not-yet-applied step's stored `sha256` ≠ this binary's embedded bytes | n/a / mismatch | (any) | **`deploy_plan_binary_mismatch`** (the resuming binary is not the one that materialized the plan) | refuse-to-serve; do **not** auto-resume; operator inspects (resume with the original deployer binary, or `abort` + redeploy) |
| absent `deploy_cursor` table / `state == idle` | n/a | (any) | **no deploy in flight** | **legacy path (flag OFF) + no-revoke binary:** serve (auto-apply); **decoupled (flag ON) OR revoke-embedding binary:** refuse-to-serve `awaiting_deploy`/`awaiting_deploy_config` (C2 — see §3.3a) |
| `complete` but `plan_hash` ≠ expected, OR `aborted` | foreign / n/a | mismatch | **genuine drift / aborted** | refuse-to-serve (existing `schema_drift` halt / `awaiting_deploy`) |
| `complete`, `plan_hash` == expected | == expected | **mismatch** | **genuine post-deploy drift** | refuse-to-serve (`schema_drift`) — unambiguous because `complete` ⇒ fingerprint was durable |

Three load-bearing properties this table now enforces:

- **(BC-N1) Plan identity is a stored fact, not a recomputation.** The classifier reads
  `deploy_plan[cursor.plan_hash]` and resumes off it. The v3 break — "cursor holds `H`
  while the binary recomputes `H'`, so the interrupted deploy is unclassified
  foreign-incomplete" — cannot occur: the cursor's `plan_hash` keys the stored transcript
  directly; a frontier that moved under the deploy does not change the stored identity. The
  only escalation is `deploy_plan_binary_mismatch`, which is a *different, narrower* and
  *legible* condition (wrong binary), not the unclassified-drift bucket.
- **(C1) The `complete`-with-missing-receipt window cannot occur** (under §1.2, `complete`
  is the final write), and because 0021 is the terminal plan step (§3.2), `complete`
  additionally implies 0021 is applied — so an in-sync `complete` classification never
  serves a database where `striatumd_rw` still holds CREATE on the serving path.
- **(BC-N2) `incomplete cursor never serves` holds for EVERY deployer-aware binary.** The
  rows above are consulted by the universal `CheckDeployActivation` edge (§3.3a) on every
  boot, no-revoke binaries included — so the pre-revoke window (`applied_owner == 20`,
  non-`complete` cursor) is refused, not served.

---

## 2. Decision Q4 — Is a deploy itself a Striatum run? (carry-forward, sound)

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape door is
held open by three already-present seams.** Unchanged from v1/v2/v3; neither falsifier
contested it.

**Why not a run now — the bootstrapping paradox is dispositive.** A dogfooded run needs the
daemon up and its schema present to drive `runs`/`jobs`/`sessions`/the capability ledger.
But the deploy is *precisely the operation that brings a fresh or owner-only database to a
serveable schema* (successor to boot-time `ApplyMigrations` and `daemon migrate-db`,
`daemon.go:74,167`). So the base deployer is a **plain verb against a bare / owner-only DB
with no running daemon**, the same context `daemon owner-ddl apply` (`daemon.go:76,90`)
already occupies.

**What keeps the run-shape door open (so P5 needs no re-litigation):** (1) the plan is a
content-addressed, **now durably-stored** artifact (`plan_hash` + `deploy_plan`, §3.2);
(2) the deploy receipt is hash-chained into owner-held `audit_log` (§3.4) — one receipt per
step; (3) the verb is a thin CLI over a `Deployer` package with a pure `Apply(ctx, plan,
cursor)` core a future run step invokes unchanged. **Falsifiable boundary:** if a falsifier
shows a real fresh-DB bring-up the plain verb cannot perform but a run could, Q4 flips.

---

## 3. The deployer surface + serve-boot decoupling (shadow-first, by exact code site)

### 3.1 Command site (carry-forward; connection model sharpened for the receipt)

Add `case "deploy":` to the dispatch in `RunDaemon` (`go/pkg/cli/localcommands/daemon.go:67-81`,
alongside `install|uninstall|status|migrate-db|owner-ddl`) → new `runDaemonDeploy(...)`.
DSN resolution mirrors `runDaemonOwnerDDL` (`daemon.go:90`, owner/admin DSN at `daemon.go:115`):
`--owner-url` / `--admin-url`, then `STRIATUM_DAEMON_ADMIN_DB_URL` (`EnvDaemonAdminDBURL`,
`daemon.go:24`), then the daemon DSN. Flags: `--plan` (default embedded), `--dry-run` (print
ordered plan + cursor delta, mutate nothing), `--resume` (default; explicit for clarity),
`--abort` (mark a wedged cursor `aborted` for operator redeploy), `--json`.
`docs/reference/command-authority-matrix.md` and the authority guardrail tests gain the
`daemon.deploy` row (AGENTS.md change discipline).

**Connection model (load-bearing — stated precisely).** The deployer applies **every
deploy-plan step over the SINGLE owner connection**: the runtime migration DDL, the C3
ownership reconcile, the grant re-assert, the `schema_migrations`/`schema_meta` version
stamps (`migrations.go:320`), the runtime-owned `deploy_cursor`/`deploy_plan` writes, **and
the per-step `append_audit_row` receipt**. Legal because the owner role is a **member of
`striatumd_rw`** (or superuser) in the two-role bootstrap (`authority_bootstrap.go`) — so it
can write the rw-owned `deploy_cursor`/`deploy_plan`/`schema_state` and `ALTER … OWNER TO
striatumd_rw` (§3.3b) — and owns (thus can EXECUTE) the owner-only SD function
`append_audit_row` (`0001_authority_phase0.sql:152,238`), which **runs in the caller's
transaction**. The deployer asserts this membership precondition at start. The **only**
write the deployer routes over a separate **runtime view** is the C1 finalizer's terminal
`schema_state` fingerprint self-record (deliberately mirroring the boot self-record path
`connection.go:399` byte-for-byte — §3.4), which is exactly why the *terminal* finalizer is
two-connection-idempotent while *per-step* writes are single-connection atomic.

### 3.2 The deploy plan — materialized ONCE into an immutable transcript (BC-N1), revoke 0021 sorted last (C3)

**Claim (PLAN-MATERIALIZED-ONCE — BC-N1).** Plan identity is computed **once**, at deploy
start, from the same two embedded `embed.FS` sources the fingerprint already hashes — the
runtime migration FS (`migrations.go` `//go:embed sql/*.sql`) and the owner-bundle FS
(`owner.go:156` `//go:embed sql/owner/*.sql`) — and then **persisted as the immutable
`deploy_plan` transcript before step 0** (§1.2). Resume reads the stored transcript; it
**never** recomputes the plan from the moved live frontiers.

**Substrate-ensure preamble (breaks the bootstrap recursion; pre-transcript, idempotent).**
The `deploy_plan`/`deploy_cursor` tables (migration 0044) are the *floor* of any deploy:
you cannot store a transcript before its tables exist. So the deployer's FIRST action is an
idempotent **substrate-ensure** — apply migration 0044 (`CREATE TABLE IF NOT EXISTS …`,
both tables) over the owner connection, reconciling the new tables' ownership to
`striatumd_rw` while CREATE is held (§3.3b mechanism) and re-asserting their DML grants —
**before** transcript materialization. In the normal rollout the inert-landing binary
already applied 0044 on its boot, so substrate-ensure is a no-op; in the bootstrap-order
case (a DB that never ran the inert binary) it creates the tables. Because 0044 is
idempotent and **pre-transcript**, it needs no stored-transcript receipt — a crash during
substrate-ensure simply re-runs the idempotent CREATEs on the next deploy. **0044 is
therefore NEVER a numbered transcript step**; the transcript's `base_runtime_version` is
observed *after* substrate-ensure.

`BuildPlan(base_owner, base_runtime) → DeployPlan` (materialization only):

- **Steps** = pending **non-revoke** owner bundles (ascending `Version` **≤ 20**, tagged
  `owner`, `owner.go:196-224`) **then** pending runtime migrations (ascending `Version`
  **≥ 0045**, tagged `runtime`, `migrations.go`) **then**, as the **TERMINAL step, the
  DDL-revoke bundle 0021** if pending (tagged `owner`, sorted last). This reproduces today's
  owner-before-runtime semantics for ordinary bundles (the watermark interlock already
  requires owner ≥ Required before any runtime apply, `connection.go:349`, `owner.go:124-154`)
  and promotes it into explicit dependency edges (every `runtime` step depends on the full
  non-revoke `owner` prefix). **The C3 change:** the DDL-revoke bundle (identified by the
  named constant `DDLRevokeOwnerBundleVersion = 21`) is **special-cased out of the owner
  prefix and appended as the last step**, with a dependency edge on **every** prior step, so
  it commits **after the final runtime ownership reconcile**.
- **0021 is excluded from the `owner-ddl apply` watermark path (load-bearing for C3).**
  `LatestOwnerBundleVersion` stays **20** (the highest *non-revoke* bundle = the `owner-ddl
  apply` frontier), so `RequiredOwnerBundleVersion` stays **20** (C2 carry-forward).
  `applyPendingOwnerBundles` (`owner.go:309-320`, the `owner-ddl apply` path) is bounded to
  apply only bundles `<= LatestOwnerBundleVersion` (≤ 20) and **skips 0021**, which is
  **deploy-plan-only**. A new build guard asserts no owner bundle `>= DDLRevokeOwnerBundleVersion`
  is reachable through `owner-ddl apply`. This closes the early-revoke door: the *only*
  applier of 0021 is the deployer, as the terminal plan step.
- Each step carries `{role, step_id, step_index, sha256, transactional}`. The
  `transactional` bit is the Q3-A/Q3-B selector — a per-step annotation
  (`-- striatum:nontransactional` pragma) defaulting to transactional, validated by a
  build-time test that greps the SQL for known non-transactional verbs (`CONCURRENTLY`,
  `ALTER TYPE … ADD VALUE`) and asserts the pragma matches (sibling of the landed
  `TestRunnerMigrationsHaveNoNonTransactionalDDL`, `migrations.go:299`).
- **`plan_hash`** = sha256 of the canonical transcript — the **base frontiers
  `(base_owner, base_runtime)` followed by the ordered `step_id`+`sha256` list, including
  0021 in its terminal position** — reusing the `composeFingerprint` canonicalization
  (`schema_drift.go:105-137`). Binding base frontiers into the hash makes the identity
  unique to the exact deploy. **Bound to `ExpectedFingerprint` by construction:** a
  `complete` plan's recorded fingerprint == `ExpectedFingerprint()` (§3.4 step 2). The
  fingerprint is over the *set* of applied migrations+bundles (order-independent), while
  `plan_hash` is over the *ordered* transcript + base, so the C3 re-ordering (0021 last)
  changes `plan_hash` but **not** the fingerprint — deployer and drift gate still never
  disagree on "is the whole plan applied."
- **`step_index` is now stable by storage, not by recomputation (the BC-N1 fix).** The
  transcript is INSERTed once into `deploy_plan` (immutable) and every per-step receipt and
  the resume reader key off the stored `(plan_hash, step_index)`. The v3 circularity
  ("`step_index` stable because content-addressed by `plan_hash`" while `plan_hash` itself
  moved with the pending delta) is dissolved: `plan_hash` is frozen in `deploy_plan` at
  materialization and the live frontier never feeds the resume path again.

### 3.3 Lifting `ApplyMigrations` out of serve-boot (shadow-first; carry-forward)

**Claim (DECOUPLE).** Serve-boot stops mutating schema, behind a flag, shadow-first. The
single mutation site on the serving path is `ConnectAndMigrate` (`connection.go:332-404`),
reached via `BootstrapAndConnect(…, migrate)` (`authority_bootstrap.go:181,193`), driven by
`--migrate` (default `true`, `main.go:76`).

1. **New env `STRIATUM_DEPLOY_DECOUPLED`** (default OFF — shadow-first, cf.
   `EnvSchemaDriftRefuse`, `schema_drift.go:28`). OFF → boot behaves exactly as today; the
   decoupling lands to `main` **inert**.
2. When **ON**, `ConnectAndMigrate` becomes **`ConnectAndVerify`**: keep the Layer 2
   watermark interlock (`connection.go:349`) and the Layer 3 drift gate
   (`connection.go:376-383`) but **do NOT call `ApplyMigrations` (`connection.go:353`)**.
   Run the §1.3 classification and serve only on the in-sync row; on any other row
   refuse-to-serve via the typed `awaiting_deploy` halt (§4.2). The drift self-record
   (`connection.go:399`) is **NOT** run on the verify path — only the deployer records the
   fingerprint (§3.4), so a verify-only boot can never overwrite `schema_state` to falsely
   claim in-sync.
3. **No window serving an unmigrated schema (R2).** The decoupled boot verifies before
   serving and refuses on any non-in-sync state; the watermark interlock is preserved
   verbatim.

#### 3.3a The deploy-activation interlock — now a UNIVERSAL pre-revoke cursor edge (BC-N2) + the C2 carry-forward arms

A **`CheckDeployActivation(ctx, runner, revokeEmbedded, decoupledEnabled bool) error`** (in
`go/pkg/db/owner.go` or a new `deploy_activation.go`) is called **at the same boot site,
immediately after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations`**
(`connection.go:349-353`), in **both** `ConnectAndMigrate` and `ConnectAndVerify`, and
**before `RecordSchemaFingerprint` (`connection.go:399`)**. It mutates nothing. It reads
`deploy_cursor` **defensively** (absent table/row → `cursorState = none`). Predicate
(fail-closed), in order:

1. **UNIVERSAL incomplete-deploy edge (BC-N2 — runs for EVERY deployer-aware binary,
   regardless of `revokeEmbedded`/watermark).** If `cursorState ∈ {in_progress,
   step_committed, finalizing}` → return **`awaiting_deploy`**, DB untouched. **This is the
   pre-revoke hard edge**: it holds at `applied_owner == 20` (and 21), so a **no-revoke**
   landing binary that restarts mid-deploy reads the non-`complete` cursor and halts BEFORE
   `ApplyMigrations` and BEFORE `RecordSchemaFingerprint`. The §1.3 stored-transcript /
   `deploy_plan_binary_mismatch` distinctions apply to how `deploy` then *resumes*; for the
   *serve* decision, any non-`complete` cursor row refuses.
2. If `cursorState == aborted` → return **`awaiting_deploy`** (operator must `--abort`/
   redeploy or resume). DB untouched.
3. If `cursorState == complete`: defer to the drift gate — `plan_hash == expected` +
   fingerprint match → serve; foreign `plan_hash` or fingerprint mismatch → the existing
   `schema_drift` halt fires (`connection.go:376-383`). `CheckDeployActivation` returns nil
   here and lets the established Layer 3 gate decide, unchanged.
4. If `cursorState == none` (absent table/row, or `idle` — **no deploy in flight**):
   - **`decoupledEnabled == true`** → **`awaiting_deploy`** (the decoupled boot never
     auto-applies; a deploy must be run). DB untouched. *(C2 carry-forward:
     absent-cursor-as-incomplete on the verify path.)*
   - **`decoupledEnabled == false` (legacy auto-apply path):**
     - **`revokeEmbedded == true`** → **`awaiting_deploy_config`** ("this binary ships the
       RFC 0142 DDL-revoke (0021); set `STRIATUM_DEPLOY_DECOUPLED=1` and run `striatum
       daemon deploy`"). DB untouched. *(C2 carry-forward: a revoke-embedding binary never
       legacy-auto-applies.)*
     - **`revokeEmbedded == false`** → **return nil**: no deploy substrate / no deploy in
       flight, legacy path — serve, and `ApplyMigrations` runs. This is the inert-landing
       binary's normal boot and fresh-DB bring-up; **it is NOT blocked** (so C3 is not
       regressed and a clean boot is not wedged).

`revokeEmbedded` = the embedded owner-bundle FS contains the bundle at
`DDLRevokeOwnerBundleVersion = 21`. **This is "the binary ships the revoke," NOT `Latest >=
21`** — `LatestOwnerBundleVersion` stays 20 (the watermark frontier) even for a
revoke-embedding binary, because 0021 is deploy-plan-only (§3.2). The activation binary is
detected by the *presence of the 0021 file in `ownerBundleFS`*, not by the watermark
frontier.

**Net invariant (the BC-N2 + C2 close):** **No boot serves while the cursor is
non-`complete`, and `ApplyMigrations` / `RecordSchemaFingerprint` are never reached over an
incomplete deploy — for ANY deployer-aware binary.** A no-revoke binary halts on the
universal edge during the pre-revoke window (`applied_owner == 20`); a revoke-unaware
*deployer-aware* binary additionally cannot legacy-auto-apply the revoke; and a no-revoke
binary observing a *completed* deploy (`applied_owner == 21`) halts via the forward-watermark
rule (§4.2). The only un-gated path is exactly the safe one: a no-revoke binary on a
fresh/idle DB with no deploy in flight, on the legacy path.

#### 3.3b Runtime-object ownership: Policy 1 with the revoke sequenced last (C3 — carry-forward, re-anchored)

The C3 prerequisite: Policy 1 reconciles ownership with `ALTER <kind> striatumd.<name>
OWNER TO striatumd_rw`, but PostgreSQL requires the **new** owner to hold `CREATE ON SCHEMA
striatumd` — the repo's own bundles document this and **GRANT `CREATE` FIRST** for exactly
this reason: `0018_runtime_table_ownership_transfer.sql:64-66` ("striatumd_rw MUST hold
CREATE on schema striatumd … otherwise `ALTER … OWNER TO striatumd_rw` fails `permission
denied for schema striatumd`") and `:98` (`GRANT CREATE ON SCHEMA striatumd TO
striatumd_rw` before the `:102` `ALTER … OWNER TO` loop);
`0019_supervisor_pointer_runtime_ownership.sql:53-80` repeats the prerequisite and grant.

**Resolution (a) — sequence the revoke last (carry-forward).** Bundle **0021** is the
**terminal** plan step (§3.2). The runtime ownership reconciles all run while `striatumd_rw`
**still holds CREATE** (granted idempotently by bundle 0018 at `0018:98`, in the non-revoke
owner prefix and therefore applied before any runtime step). 0021 commits **last**, so the
**committed steady state still denies CREATE**. No transient elevated grant; no §4.1
safety-claim retraction.

**Chosen policy (one, tested): runtime objects stay `striatumd_rw`-owned.** Preserves the
landed invariant (bundles 0018/0019 transferred runtime-table ownership *to* `striatumd_rw`);
keeps §4.1's residual-capability argument intact; keeps every runtime object uniform.

**Mechanism (`Deployer.applyRuntimeStep`, a deployer-owned wrapper around the existing
`applyOne` body, `migrations.go:304-355`).** `SET ROLE striatumd_rw` is **not** usable for
the create (it would change who owns the new object and, post-0021, `striatumd_rw` lacks
CREATE), so ownership is reconciled **after** the DDL, **inside the same owner-connection
step transaction**:

1. `BEGIN` (owner connection). **Deployer precondition assert (defense-in-depth):**
   `has_schema_privilege('striatumd_rw','striatumd','CREATE')` is `true`; if not, halt
   legibly (`deploy_create_prerequisite_missing`) **before** any DDL rather than failing
   with a raw `42501` mid-plan. By plan ordering this always holds (0018 grants, 0021
   terminal), so the assert is a guard, not the mechanism.
2. Snapshot the owner-role-owned object oids in schema `striatumd` (a `pg_class` read for
   relkinds `r,p,i,I,S,v,m,t` plus `pg_proc`/`pg_type` if a future runtime migration ever
   creates those kinds — currently none do).
3. Run the migration SQL (the existing `applyOne` DDL + both version stamps + in-tx hash
   verify, `migrations.go:304-355`).
4. Re-snapshot; for **exactly the newly-appeared oids** run `ALTER <kind> striatumd.<name>
   OWNER TO striatumd_rw`. Indexes inherit their table's owner; an owned sequence appears as
   its own new `relkind=S` oid and is reassigned too — so the catalog diff covers tables,
   partitions, indexes, sequences, views, matviews, and future object kinds without
   enumerating them. Scoped to **new** oids, so it never touches pre-existing owner-owned
   authority/bundle objects, and never an owner step's objects.
5. Re-assert the migration's DML grants to `striatumd_rw` (the `GRANT` block the migration
   carries, e.g. `0043_schema_state.sql:49`) so a created-as-owner-then-reassigned object
   has the serving role's DML regardless of default ACLs.
6. **Append the per-step receipt** via `append_audit_row(...)` keyed on the **stored
   transcript's** `(plan_hash, step_index)` (§3.4) — same transaction.
7. Advance `deploy_cursor` `in_progress(k) → step_committed(k)` (Q3-A: same transaction).
   `COMMIT`.

The reconciliation is **idempotent**: on resume an already-rw-owned object is not in the
"owner-role-owned" snapshot, so it is skipped. It runs only on transactional
object-creating runtime steps (NT-DDL creates no mis-owned object, §1.1) **and** on the
substrate-ensure preamble's 0044 tables. The owner role can `ALTER … OWNER TO striatumd_rw`
because (i) it is a member of that role (§3.1) and (ii) `striatumd_rw` still holds CREATE at
this plan time (0021 not yet applied).

### 3.4 The deploy receipt: per-step, keyed off the STORED transcript (BC-N1 + N1) + the idempotent finalizer (C1)

**Claim (RECEIPT).** Every applied step writes **exactly one** hash-chained deploy receipt
into the owner-held `audit_log` via the owner-only SD function `append_audit_row(...)`
(`0001_authority_phase0.sql:152,238`; runtime direct `INSERT` revoked at `:237`). Payload:
`{plan_hash, step_index, step_id, role, sha256, state, prev_receipt_hash}`. `state ∈
{step_committed, complete}`. **The key `(plan_hash, step_index)` is read from the immutable
`deploy_plan` transcript (§1.2), not recomputed** — this is the BC-N1 fix at the receipt
layer.

**Per-step receipt crash-safety (N1, kept) on a stable key (BC-N1, new).**

- **Q3-A (transactional step):** the receipt append occurs in the **same owner-connection
  transaction** as the DDL + ownership reconcile + grants + version stamps + cursor advance
  (§3.3b steps 1–7). Therefore **`step_committed(k)` is durable iff its receipt is durable**
  — no crash window between a transactional step's commit and its receipt. The same holds
  for the **terminal T-DDL owner step 0021** (its receipt joins the per-bundle transaction,
  `owner.go:511-541`).
- **Q3-B (non-transactional step):** the `in_progress(k)` reconciler appends **exactly one**
  receipt keyed on the stored `(plan_hash, step_index)` (+ `step_id`, `sha256`,
  `state=step_committed`) **before** writing `step_committed(k)`, in one owner-connection
  reconcile transaction. **Idempotency rule:** if a `step_committed` receipt for `(plan_hash,
  step_index)` already exists, the reconciler **skips** the append (exactly-once), then
  writes `step_committed(k)`. A crash after the receipt append but before `step_committed(k)`
  re-enters at `in_progress(k)`, finds the receipt present, skips, advances; a crash before
  the receipt append re-enters, finds it absent, appends exactly one, advances. The resume
  **never advances past a step lacking its receipt** because the cursor advance and the
  receipt share the reconcile transaction.
- **(BC-N1) the key survives the moved frontier.** Because `step_index` and `plan_hash` are
  read from `deploy_plan` (frozen at materialization), the `(H,0)` receipt for an
  already-applied step is recomputable on any resume: the deployer loads `deploy_plan[H]`,
  reads `steps[0]`, and recognizes its `(H,0)` receipt exactly-once — even after the live
  runtime/owner frontiers have advanced past that step.

**C1 ⊕ N1 ⊕ BC-N1 coherence.** Per-step writes are all over the **single owner connection**,
so a transactional step is **atomic** (one tx) and an NT-DDL step is **idempotent-keyed** on
the stored `(plan_hash, step_index)`. The **terminal finalizer** is **idempotent across two
connections** because the `complete` receipt (owner connection) and the `schema_state`
fingerprint self-record (runtime view, byte-identical to the boot self-record
`connection.go:399`) **cannot share one transaction** — the operator pin, preserved. Its
idempotency is keyed on `(plan_hash, state=complete)`, where `plan_hash` is the stored
identity. A resume never serves (`finalizing`/`step_committed`/`in_progress` ⇒
`awaiting_deploy`, §3.3a), never advances past a step missing its receipt, and the terminal
`complete` receipt stays exactly-once.

**Finalizer crash-point table (C1 — carry-forward verbatim, re-anchored 0021):**

| Crash point | Cursor after crash | §1.3 classification | Re-run repair |
| --- | --- | --- | --- |
| after final step (0021) DDL+receipt commit, before `finalizing` | `step_committed(N-1)` | incomplete, resume (off stored transcript) | advance to `finalizing`, run finalizer |
| after `finalizing`, before receipt | `finalizing` | **resumable finalization** | receipt absent → append; fingerprint; `complete` |
| after receipt, before `schema_state` | `finalizing` | **resumable finalization** | receipt present → **skip (exactly-once)**; fingerprint; `complete` |
| after `schema_state`, before `complete` | `finalizing` (fingerprint now matches) | **resumable finalization** (cursor ≠ `complete` ⇒ never serve) | receipt skip; fingerprint UPSERT no-op; `complete` |
| after `complete` | `complete`, fingerprint match, receipt present | in sync | none — serve |

**Tightened `doctor schema_deploy_unrecorded` — enumerates from the STORED transcript
(BC-N1).** `doctor` gains/keeps `schema_deploy_unrecorded` (modeled on
`schemaDriftDoctorBlock`, `go/pkg/reads/doctor_schema_drift.go:26-77`), now keyed off the
**stored `deploy_plan` transcript**, NOT a recomputed `BuildPlan`: when a `deploy_cursor`
row exists, it loads `deploy_plan[cursor.plan_hash]`, determines which transcript
`step_index`es are **applied** (committed — by `cursor.state`/`step_index` plus the applied
`schema_migrations`/`owner_bundle_meta` frontier), and WARNs when **any** applied transcript
step lacks a `step_committed` receipt for its `(plan_hash, step_index)` — **not** masked by a
present terminal `complete` receipt. When no `deploy_cursor` row exists (legacy
auto-applied DB), the previous terminal-only check (advanced `schema_state.fingerprint`
without a `complete` receipt) is retained. So the doctor is green **only when all committed
transcript steps have receipts**, and it never depends on the moving pending plan
(eliminating the v3 "doctor can't enumerate applied steps after the frontier moved" hole).

---

## 4. Serving-role DDL revocation (owner bundle 0021), without lockout and without a pre-revoke serve window

**Claim (DDL-REVOKE).** The serving role `striatumd_rw` loses its remaining
**object-creation** DDL on the serving path, shipped as **owner bundle 0021**, applied
**only as the terminal step of `striatum daemon deploy`** (C3) — **activated only after the
deployer is the active mutator and after every runtime ownership reconcile** — and **the
pre-revoke incomplete-deploy window is gated by the universal cursor edge (BC-N2)** — so
failure mode 1 becomes structurally impossible without recreating a #512-class lockout,
without the v2 self-contradiction, and without the v3 pre-revoke serve hole.

### 4.1 What is revoked, precisely (carry-forward; argument true-by-construction via C3)

The anchor table confirms `striatumd_rw` already holds **no DDL on owner-owned tables**
(`0001_authority_phase0.sql:237`; preflight + build guard route owner-table DDL to bundles,
`migrations.go:158`). The residual serve-path DDL is **object creation on the runtime
schema**. Bundle 0021 revokes exactly that:

```
-- owner bundle 0021 (DDL-revoke): the serving path holds zero create-DDL.
-- DEPLOY-PLAN-TERMINAL ONLY (RFC 0142 P4 C3 resolution (a)); excluded from the
-- owner-ddl apply watermark path (LatestOwnerBundleVersion stays 20).
REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw;
REVOKE TEMPORARY ON DATABASE <db> FROM striatumd_rw;  -- if granted
```

`striatumd_rw` **retains** ownership of (and `ALTER`/`DROP` on) the runtime tables it
already owns — and, by the C3 policy (§3.3b), **new** runtime objects the deployer creates
are reconciled back to `striatumd_rw` ownership **before** 0021 commits, so the §4.1
invariant "new runtime objects stay `striatumd_rw`-owned" **holds as a tested property**
(`T-deploy-runtime-object-ownership`, F12). The honest residual is unchanged: the serving
role could still `ALTER` its *own* runtime tables, but **the serving daemon issues no DDL
once `ApplyMigrations` is lifted (§3.3)**, so the serve *path* is zero-create-DDL. Full
capability revocation (owner re-owns runtime tables) remains a **follow-up beyond P4** (it
would re-open #442), named, not assumed.

### 4.2 Typed halts + the forward-watermark rule (C2 — carry-forward, re-anchored) + the universal cursor edge (BC-N2)

Two new sibling halts of `AwaitingOwnerDDLError` (`owner.go:58`), reusing the
non-restartable exit path (`main.go:208-227`, `exitAwaitingOwnerDDL`):

- **`ErrAwaitingDeploy` / `AwaitingDeployError`** (`awaiting_deploy`): a deploy is
  pending/incomplete — cursor absent **on the decoupled path**, or `state ∈ {in_progress,
  step_committed, finalizing, aborted}` **on any deployer-aware binary** (the BC-N2
  universal edge). Remediation: "run `striatum daemon deploy`." DB untouched.
- **`ErrAwaitingDeployConfig` / `AwaitingDeployConfigError`** (`awaiting_deploy_config`):
  the binary ships the DDL-revoke (0021) but `STRIATUM_DEPLOY_DECOUPLED` is OFF and no
  deploy is in flight. Remediation: "set `STRIATUM_DEPLOY_DECOUPLED=1` and run `striatum
  daemon deploy`." DB untouched.

`main.go` gains the matching `errors.As` arms next to the `AwaitingOwnerDDLError` and
`SchemaDriftError` arms (`main.go:208-227`).

**Forward-watermark rule (in `CheckOwnerBundleWatermark`, the tolerate-forward branch
`owner.go:151-153`), re-anchored to 21.** The tolerate-forward branch is bounded by the
revoke boundary: when **`applied >= DDLRevokeOwnerBundleVersion (21)` and this binary does
NOT embed the revoke** (the 0021 file is absent from `ownerBundleFS`), the watermark returns
an **`AwaitingDeployError`** ("this database has activated the RFC 0142 DDL-revoke; this
binary predates / does not embed the one-shot deployer's revoke and cannot serve it safely —
upgrade to the activation binary") instead of tolerating forward into `ApplyMigrations`. A
revoke-aware binary still tolerates forward across the boundary but is governed by
`CheckDeployActivation` (§3.3a). Pre-0021 forward tolerance is unchanged.

**Two complementary edges (the BC-N2 close, stated as one invariant):**

| Window | `applied_owner` | no-revoke binary gated by | revoke binary gated by |
| --- | --- | --- | --- |
| pre-revoke, deploy in flight (incomplete cursor) | **20** | **universal cursor edge (§3.3a step 1)** → `awaiting_deploy` | universal cursor edge → `awaiting_deploy` |
| terminal-revoke committed, deploy not yet finalized (incomplete cursor) | 21 | universal cursor edge → `awaiting_deploy` | universal cursor edge → `awaiting_deploy` |
| deploy complete | 21 | **forward-watermark rule (§4.2)** → `awaiting_deploy` (must roll to activation binary) | `CheckDeployActivation` → serve if in-sync |
| fresh / idle, no deploy in flight | 20 | serve (legacy auto-apply) / `awaiting_deploy` if decoupled | `awaiting_deploy_config` (flag OFF) / `awaiting_deploy` (flag ON) |

The v3 gap — a no-revoke binary at `applied_owner == 20` with an incomplete cursor falling
through BOTH the (inert) `revokeEmbedded` gate and the (inert) forward-watermark rule — is
closed by the **universal cursor edge** in the first two rows. The forward-watermark rule
(re-anchored to 21) and the C2 arms remain exactly as carried forward for the other rows.

**Resolution of the v1 contradiction (auto-apply-default vs `Required = revoke ordinal`),
preserved and re-anchored.** `RequiredOwnerBundleVersion` is **NOT** advanced to 21; it
stays **20** (its current-`main` value). The DDL-revoke is gated by the universal cursor
edge + `CheckDeployActivation` + the `STRIATUM_DEPLOY_DECOUPLED` flag + its deploy-plan-
terminal placement, **not** by the watermark frontier. The inert-landing binary has `Latest
= Required = 20` and auto-applies (legacy default intact) **only when no deploy is in
flight**; the activation binary embeds the 0021 file (still `Latest = Required = 20`,
because 0021 is deploy-only) but **never auto-applies**.

### 4.3 The deploy choreography (C2 — carry-forward; re-anchored 0021/20; BC-N2 hardened)

Two binaries, in order:

1. **Inert-landing binary** (`Latest = Required = 20`; does **not** embed the 0021 file).
   Ships: the `Deployer` engine + `deploy` verb, the **0044 `deploy_cursor` + `deploy_plan`
   migration**, the `STRIATUM_DEPLOY_DECOUPLED` decoupled boot path (flag OFF), the per-step
   + terminal `doctor schema_deploy_unrecorded` block, the forward-watermark rule, **and the
   universal pre-revoke cursor edge (BC-N2)**. Boot auto-applies legacy (`striatumd_rw`
   still has `CREATE`; 0044 is additive, runtime-owned — passes
   `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`) **only because no deploy is in flight**.
   Decoupling inert. After this rollout every serving binary is forward-watermark-aware AND
   reads `deploy_cursor` on every boot. **Daemon up, serving.**
2. **Activation binary** (embeds the 0021 file; `Latest = Required` still 20;
   `CheckDeployActivation` active). On boot with the deploy incomplete + flag OFF it halts
   `awaiting_deploy_config` (it never legacy-auto-applies, so no `42501` and no false
   fingerprint self-record). The operator then, **with the daemon parked**:
   - sets `STRIATUM_DEPLOY_DECOUPLED=1`;
   - runs **`striatum daemon deploy`** (no daemon serving): the deployer, over the owner
     connection, runs substrate-ensure (0044 if absent), **materializes the immutable
     transcript into `deploy_plan` + `deploy_cursor=in_progress(0)` BEFORE step 0**, then
     applies the ordered plan — **non-revoke owner bundles (≤20) → pending runtime steps
     (each reconciling ownership while CREATE is held, §3.3b) → bundle 0021 as the terminal
     step** — writing one receipt per step keyed on the stored transcript + the finalizer's
     `complete` receipt + fingerprint + `complete` cursor (§3.4). Because the deployer uses
     the owner connection **and** 0021 is last, every `ALTER … OWNER TO striatumd_rw`
     succeeds and the steady state denies CREATE.
   - restarts the daemon → `ConnectAndVerify` sees `complete` + in-sync → serves,
     verify-only, zero serve-path create-DDL.

   **Crash mid-deploy is now safe even if the inert binary comes back up (BC-N2).** If the
   operator's discipline slips and the **inert (no-0021) binary restarts** after a runtime
   step commits but before the terminal 0021 (the C3-safe point, CREATE still held), the
   inert binary's **universal cursor edge** reads the `step_committed(k)` cursor and halts
   `awaiting_deploy` **DB-untouched** — `ApplyMigrations` and `RecordSchemaFingerprint` both
   un-called, no fingerprint self-record over the partial deploy. The safety no longer rests
   on choreography discipline; it rests on the state-machine edge.

   **Bootstrap-order case:** if step 2 runs on a DB that never ran the inert binary
   (`deploy_cursor` absent), substrate-ensure creates `deploy_cursor`/`deploy_plan` (0044
   over the owner connection, ownership reconciled to `striatumd_rw` while CREATE held)
   before materializing the transcript.

   **Truly-pre-P4 binary (predates even the cursor edge + forward-watermark rule).**
   Protected by the choreography precondition: bundle 0021 is applied **only** by `striatum
   daemon deploy`, which **preflight-refuses to apply 0021 unless the runtime frontier
   already carries the 0044 `deploy_cursor`/`deploy_plan` migration** (the inert-landing
   binary's marker) — i.e. unless a deployer-aware binary has booted this DB. Stated as F11
   / `G-old-binary-refuse`.

### 4.4 How 0021 ships without lockout (R3) and without a pre-revoke serve window (BC-N2)

The deployer applies all DDL (runtime steps **and** the terminal 0021) over the
**owner/admin connection** — the same mechanism `daemon migrate-db --admin-url`
(`daemon.go:167`) and `owner-ddl apply` (`daemon.go:90`) already use — so the "runtime path"
the revoke must not break is **already owner-capable out-of-band**; 0021 removes no
capability the deploy path needs. **The C3 fix:** because 0021 is the **terminal** plan step
(after every runtime ownership reconcile) and is **excluded from `owner-ddl apply` and the
owner prefix** (§3.2), there is **no path** by which `REVOKE CREATE` commits before a
reconcile that needs CREATE. **The BC-N2 fix:** because every deployer-aware binary reads
`deploy_cursor` before `ApplyMigrations`/`RecordSchemaFingerprint` and refuses on a
non-`complete` cursor (§3.3a), there is **no boot** — no-revoke binaries included — that can
serve or self-record over the pre-revoke incomplete-deploy window. Combined with §4.2 (typed
halts + forward-watermark rule), **no ordering — including every interleaving in
`T-deploy-revoke-activation-ordering` and `T-deploy-runtime-object-ownership` — drives boot
into a runtime-runner `ApplyMigrations` under a revoked `CREATE`, lets any binary serve an
incomplete deploy, or strands a deploy at a reconcile under a revoked `CREATE`.**

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> `T-` = unit/integration test (TDD target for the build run); `G-` = game-day against a
> real two-role cluster. F1–F12 carry forward (re-confirmed, re-anchored 0020→0021,
> 19→20); **F11 is EXTENDED for BC-N2**, **F13 is refined for the stored-transcript key**,
> and **F14 is NEW for BC-N1** — these three are what the cycle-4 falsifiers re-attack
> first.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1 (Resumability — Q3-A)** | Kill `deploy` after a transactional step *k* commits; re-run resumes at *k+1* off the stored transcript, never re-runs *k* or half-applies. | `T-deploy-resume-transactional`; `G-resume-kill-9`. |
| **F2 (Resumability — Q3-B)** | Kill `deploy` during a `CREATE INDEX CONCURRENTLY`; re-run's reconciler drops the `INVALID` index and rebuilds; final state is one VALID index, no double-apply. | `T-deploy-resume-concurrently`; `G-cic-kill`. |
| **F3 (No serve-boot mutation)** | With `STRIATUM_DEPLOY_DECOUPLED=ON`, a boot with a pending plan does NOT apply it; refuses-to-serve `awaiting_deploy` via the non-restartable exit. | `T-boot-decoupled-no-apply`; `G-pending-restart`. |
| **F4 (Fingerprint coherence — §1.3)** | An interrupted deploy is classified "incomplete, resume" / "resumable finalization" / "resume with the stored plan", never "unknown drift"; a cursor whose `plan_hash` lacks a `deploy_plan` row (or whose un-applied steps' bytes diverge) is the legible `deploy_plan_binary_mismatch`, not unclassified drift. | `T-classify-incomplete-vs-drift`: matrix over `{cursor state} × {deploy_plan present/absent} × {fingerprint match} × {pending plan == cursor plan?}` asserting the §1.3 table exactly; mutate a stored step `sha256` → flips to `deploy_plan_binary_mismatch`; mutate `plan_hash` to foreign-complete → genuine-drift. |
| **F5 (No regression of P2/P3 — R2)** | Lifting `ApplyMigrations` keeps the watermark interlock intact; fresh-DB bring-up still works via the verb; a fresh/idle DB (no cursor) is NOT wedged by the universal edge. | `T-decoupled-preserves-watermark`; `T-fresh-db-via-deploy`; `T-idle-cursor-serves` (no `deploy_cursor` row, no-revoke binary, flag OFF → serves); existing `migrations_test.go` two-role guard stays green. |
| **F6 (DDL revocation — R3, structural)** | After the deploy's terminal bundle 0021, `striatumd_rw` cannot `CREATE` on schema `striatumd`; the deployer (owner connection) still applies runtime steps; no lockout across restart. | `T-revoke-blocks-rw-create`; `T-deploy-after-revoke`; `G-revoke-then-restart`. |
| **F7 (Receipt provenance — R4, per-step, stored-transcript key)** | Every applied step writes **exactly one** hash-chained deploy receipt keyed on the **stored** `(plan_hash, step_index)`; a committed step without one is surfaced by the tightened `doctor schema_deploy_unrecorded` (enumerating from `deploy_plan`, not masked by a present terminal receipt). | `T-receipt-per-step`; `T-doctor-unrecorded-per-step` (apply a step, delete its per-step receipt, assert doctor WARNs even with the terminal `complete` receipt present, enumerating from the stored transcript). |
| **F8 (Cursor has no double-apply/skip hole — R4)** | No interleaving of crash points across a step boundary yields a skipped/twice-applied step **or** a missing/duplicated per-step receipt, with the key read from the stored transcript. | `T-deploy-fault-injection`: parametrized crash at each of {before materialize, after materialize/before step 0, after marker/before step, after step/before receipt+commit-marker, after receipt+commit-marker} for a T-DDL and an NT-DDL step; assert one apply and exactly one receipt each. |
| **F9 (Plan ≡ fingerprint binding; 0021-terminal)** | A `complete` deploy's recorded fingerprint equals `ExpectedFingerprint()`, and `plan_hash` is over the base + ordered transcript **with 0021 last** (re-ordering changes `plan_hash`, not the fingerprint). | `T-plan-hash-binds-fingerprint`; `T-plan-orders-revoke-last`. |
| **F10 (Finalization boundary closed — C1; carry-forward)** | No crash during finalization leaves a `complete` cursor with a missing receipt/fingerprint; every interrupted finalization is pinned at `finalizing`, classified **resumable finalization**, and repaired exactly once. | **`T-deploy-resume-finalization-crash`**: kill after each of {final DDL commit; `finalizing`-before-receipt; receipt-before-`schema_state`; `schema_state`-before-`complete`}; assert (a) no boot serves until the cursor reads `complete`, (b) exactly one `complete` receipt after repair, (c) `LiveFingerprint == ExpectedFingerprint` and doctor green. `G-finalize-kill`. |
| **F11 (Activation fail-closed — C2 + the BC-N2 universal edge; EXTENDED)** | For **every deployer-aware binary**, boot **never** calls `ApplyMigrations` over the runtime runner nor `RecordSchemaFingerprint` while the cursor is non-`complete`; it halts `awaiting_deploy`/`awaiting_deploy_config` DB-untouched. **Crucially (BC-N2): a NO-revoke deployer-aware binary at `applied_owner == 20` with `deploy_cursor.state ∈ {in_progress, step_committed, finalizing}` halts `awaiting_deploy` — `ApplyMigrations` NOT called, `RecordSchemaFingerprint` NOT called, DB byte-identical.** | **`T-deploy-revoke-activation-ordering`**, parametrized over: (a) revoke-embedding binary, flag OFF, no deploy in flight → `awaiting_deploy_config`; (b) flag ON, `deploy_cursor` absent → `awaiting_deploy`; (c) old/no-0021 binary + `applied_owner == 21` (completed deploy) + pending runtime → `awaiting_deploy` via forward-watermark, `ApplyMigrations` NOT called; (d) revoke-embedding binary + flag OFF + pending runtime → `awaiting_deploy_config`, `ApplyMigrations` NOT called; **(e) NEW — NO-0021 deployer-aware binary, flag OFF, `applied_owner == 20`, `deploy_cursor.state ∈ {in_progress, step_committed, finalizing}`, ±fingerprint mismatch, ±pending runtime → `awaiting_deploy` via the universal cursor edge, `ApplyMigrations` NOT called, `RecordSchemaFingerprint` NOT called, DB byte-identical; (f) NEW — same binary, no `deploy_cursor` row / `idle` → SERVES (legacy auto-apply, not wedged).** Assert via spies that `applyOne` and `RecordSchemaFingerprint` are never entered in (a)–(e) and the schema is byte-identical. `G-old-binary-refuse`: assert `daemon deploy` preflight-refuses 0021 when the 0044 marker is absent, **and** that the inert binary brought back up mid-deploy refuses to serve the pre-revoke incomplete window. |
| **F12 (Runtime-object ownership under the 0021-terminal plan — C3, re-anchored)** | A runtime migration applied through the deployer's owner connection yields objects **owned by `striatumd_rw`** with the serving role's real DML intact, **because the ownership reconcile runs while `striatumd_rw` still holds CREATE (0021 not yet applied)**; after the deploy's terminal 0021 the serving role cannot create objects. | **`T-deploy-runtime-object-ownership`** (two-role pgtest, **documented non-superuser** cluster): build a plan = {a new runtime migration creating a table + index + sequence} + {terminal bundle 0021}; assert during the run `has_schema_privilege('striatumd_rw','striatumd','CREATE')` is **true** at the reconcile and the reconcile step's `step_index` is **strictly less** than 0021's; after the deploy assert `pg_class.relowner`/`pg_get_userbyid` == `striatumd_rw` for **every** created object (table, index, sequence); `SET ROLE striatumd_rw` can `SELECT/INSERT/UPDATE/DELETE` the new table; `SET ROLE striatumd_rw; CREATE TABLE …` → **`42501`** after 0021; `has_schema_privilege … CREATE` **false** post-deploy; idempotent re-run. `G-revoke-last`: on a real two-role cluster, kill `deploy` *between* the last runtime reconcile and 0021, re-run, assert it completes (CREATE still held on resume) and the steady state denies CREATE. |
| **F13 (Per-step receipt crash-safe on a STABLE key — N1 refined by BC-N1)** | A crash between a step's commit and its receipt-write is reconciled to **exactly one** receipt per applied step on re-run, keyed on the **stored** `(plan_hash, step_index)`; the resume never advances past a step lacking its receipt. | **`T-deploy-receipt-crash-resume`**: kill at each crash point — (T-DDL) after DDL+cursor+receipt commit (atomic, single point); (NT-DDL) after the side effect before the reconcile, after the receipt append before `step_committed`, after `step_committed` — and assert after re-run: final schema equality, **exactly one** `step_committed` receipt per applied stored `(plan_hash, step_index)`, one terminal `complete` receipt, green doctor. `G-receipt-kill`. |
| **F14 (Immutable plan identity across resume — BC-N1, NEW)** | Plan identity is materialized into `deploy_plan` **before step 0** and resume reads the **stored** transcript, so a mid-plan crash that **moves the live frontier** does NOT change `plan_hash` or renumber `step_index`, and the prior receipts are recognized exactly-once. Resume **never** recomputes `BuildPlan` from the moved frontier; a binary whose embedded bytes diverge from the stored transcript halts `deploy_plan_binary_mismatch` rather than silently re-keying. | **`T-deploy-plan-hash-resume-after-step`**: a plan with **≥2 runtime steps + terminal 0021** from base `(owner 20, runtime 44)`; **kill after step 0 commits AND (separately) after step 1 commits**; assert the re-run (i) reads the SAME `deploy_plan.plan_hash`, (ii) preserves the original `step_index`es (the moved runtime frontier does NOT renumber them), (iii) recognizes the prior `(plan_hash, step_index)` receipts exactly-once (no re-append, no double-apply), (iv) completes the remaining steps + finalizer, (v) leaves `doctor` green; plus a negative case: tamper a not-yet-applied stored step's `sha256` (or run a binary with divergent embedded SQL) → resume halts `deploy_plan_binary_mismatch`, applies nothing. `G-plan-resume-kill`: `kill -9` after a committed runtime step on a real cluster, advance the live frontier, re-run `deploy`, assert the stored `plan_hash`/`step_index`es and prior receipts survive. |

---

## 6. Shadow-first rollout (the P4 build order) — re-anchored + BC-N1/BC-N2

1. **Migration 0044 `deploy_cursor` + `deploy_plan`** (additive runtime tables, modeled on
   `0043_schema_state.sql:39-52`; the `deploy_cursor.state` CHECK includes `finalizing`;
   `deploy_plan` holds the immutable transcript; no FK to owner objects, runtime-owned —
   passes `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`).
2. **`go/pkg/db/deploy.go`**: `DeployPlan`, `BuildPlan` (materialization-only, **with the
   0021-terminal special-case + 0021 excluded from `owner-ddl apply`**, §3.2), **the
   transcript materialize-once + `LoadStoredPlan(plan_hash)` resume reader (BC-N1)**,
   `Deployer.Apply` (Q3-A/Q3-B engine + the `finalizing` finalizer §3.4, advisory-locked via
   `MigrationLockKey` `migrations.go:18`), the substrate-ensure preamble, `applyRuntimeStep`
   (the **C3 ownership reconcile while CREATE is held**, §3.3b), the **per-step + terminal
   receipt writer keyed on the stored `(plan_hash, step_index)`**. Pure-core + DB-integration
   tests (F1, F2, F4, F8, F9, F10, **F12**, **F13**, **F14**) — engine + finalization +
   ownership + immutable-transcript resume + per-step receipt proven **before** any boot
   path changes.
3. **`runDaemonDeploy`** verb (`daemon.go`) + matrix/authority-guardrail row + `--dry-run` +
   `--abort` + the **0021-activation preflight** (§4.3). F3/F5 wiring.
4. **`CheckDeployActivation`** with the **universal pre-revoke cursor edge (BC-N2)** + the
   typed `awaiting_deploy`/`awaiting_deploy_config` halts + the **forward-watermark rule
   (re-anchored 21)** in `CheckOwnerBundleWatermark` + the `main.go` `errors.As` arms (§3.3a,
   §4.2). Decoupled boot path (`ConnectAndVerify`) behind `STRIATUM_DEPLOY_DECOUPLED`
   (default OFF). Lands inert (**F11 extended**, F3, F5).
5. **`doctor schema_deploy_unrecorded`** block — per-step tightened, **enumerated from the
   stored transcript** (F7).
6. **Owner bundle 0021** (DDL revoke) — authored, **deploy-plan-terminal and excluded from
   `owner-ddl apply`** (`LatestOwnerBundleVersion` stays 20), tested in two-role pgtest (F6,
   F12); activation is the operator choreography (§4.3).

Each phase additive and reversible until the operator flips the flag + the deploy applies
0021 last. Self-record before enforce; detection before mutation-relocation; the verb +
finalizer + ownership reconcile + immutable-transcript resume + per-step receipt proven
before the boot path leans on them.

---

## 7. Boundary & explicit deferrals (carry-forward)

- **In P4:** the deployer verb, the deploy plan + `deploy_cursor` (incl. `finalizing`) + the
  **immutable `deploy_plan` transcript (BC-N1)** + per-step + terminal deploy receipt, the
  idempotent finalizer (C1), the C3 ownership reconcile **with the revoke (0021) sequenced
  last**, lifting `ApplyMigrations` out of serve-boot (flagged), the activation interlock +
  **the universal pre-revoke cursor edge (BC-N2)** + forward-watermark rule, serving-role
  create-DDL revocation (bundle 0021, deploy-terminal).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`, `rehearsal_receipt.v1`,
  fidelity tiering, full-data clone, expand/contract reshape primitive, lock-budget
  guardrail. P4 names the seams (the durably-stored plan artifact, per-step receipt chain,
  `Deployer.Apply` engine) P5 builds on but ships none of it. Full capability revocation
  (owner re-owns runtime tables) is a named follow-up beyond P4 (§4.1), not assumed.
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the single writer,
  no hosted services / replicas / external persistence.

---

## 8. Open tensions handed to the falsifiers (attack these hardest)

- **BC-N1 / F14 — the materialize-before-step-0 ordering.** Is there ANY crash window
  between observing the base frontiers and committing the `deploy_plan` + `in_progress(0)`
  transaction in which a step could commit (moving the frontier) before the transcript is
  durable? Can the substrate-ensure preamble (0044) ever advance a frontier that the
  *not-yet-written* transcript would need to record as `base_runtime`? Is the
  `plan_hash`-includes-base-frontiers definition collision-free across two deploys from
  different bases with the same step set?
- **BC-N1 / F14 — the stored-transcript resume under concurrency.** Under `MigrationLockKey`,
  can two deployers both pass the substrate-ensure and race to INSERT `deploy_plan` for
  different `plan_hash`es against the singleton cursor? Does `ON CONFLICT (plan_hash) DO
  NOTHING` + the singleton cursor + the advisory lock truly serialize them? On resume with a
  binary whose embedded SQL changed, does the per-step `sha256` verification fire for an
  **already-applied** step (whose bytes are also recorded in `schema_migrations`), and is
  `deploy_plan_binary_mismatch` the right disposition vs. silently trusting the stored row?
- **BC-N2 / F11(e) — the universal edge vs. fresh-DB bring-up.** Does the universal cursor
  edge ever fire on a path that MUST serve (the inert binary's first boot before 0044 exists;
  a fresh DB with no cursor; a legitimately `complete` cursor)? Is the `none`-vs-`idle`-vs-
  non-`complete` distinction airtight, and does row (f) of F11 actually serve? Can a deploy
  ever leave the cursor at `idle` with steps half-applied (it must not — materialization
  sets `in_progress(0)` before step 0)?
- **BC-N2 — the decoupled-path interaction.** With flag ON, absent cursor → `awaiting_deploy`;
  does that wedge a fresh DB whose operator intends to run `deploy` (expected) vs. an
  upgrade that should just verify (also expected)? Is there a boot that reads `finalizing` +
  matching fingerprint and wrongly serves (must not — `finalizing` is non-`complete`)?
- **C3 / F12 (carry-forward) — the revoke-last ordering and the catalog diff.** Is there any
  path by which 0021's `REVOKE CREATE` commits before a runtime ownership reconcile that
  needs CREATE (a future bundle `> 21` BuildPlan might sort between runtime steps and 0021; a
  `daemon owner-ddl apply` invoked on the activation binary — is the `<= 20` bound enforced
  and `revokeEmbedded` decoupled from `Latest`)? Does the before/after oid diff miss any
  object kind a runtime migration could create (a TYPE, a trigger function, a `GENERATED … AS
  IDENTITY` sequence)?
- **C2 / F11 (carry-forward) — the old-binary preflight.** Is the `daemon deploy`
  0021-preflight (refuse unless the 0044 marker is present) sufficient against a truly-pre-P4
  binary still serving across a restart, now that 0021 reaches the watermark only at deploy
  end and the universal edge requires the 0044 substrate to exist to bite?
- **§4.1 — the residual capability.** Revoking only `CREATE ON SCHEMA` leaves `striatumd_rw`
  able to `ALTER` its own tables. Is "the serve *path* issues no DDL" a sufficient P4 close
  of failure mode 1, or can a falsifier construct a serve-path DDL (a DML trigger / `DO`
  block that issues DDL)?
- **Q4 — bootstrapping.** Any path where plain-verb-now forecloses the run-shape later that
  the three seams (§2) don't keep open?

---

### Anchor table (re-verified against current `main`, HEAD `5c5cb36a`)

| Target | Anchor |
| --- | --- |
| serve-boot mutation site (lift this) | `go/pkg/db/connection.go:332` `ConnectAndMigrate`; apply at `:353`; watermark `:349`; drift gate `:376-383`; shadow log `:384-393`; self-record `:399` |
| boot fork / `--migrate` flag / typed-halt mapping | `go/pkg/db/authority_bootstrap.go:181,193`; `go/cmd/striatumd/main.go:76`; typed halts `:208-227` (`exitAwaitingOwnerDDL`; C2/BC-N2 add `awaiting_deploy`/`awaiting_deploy_config` arms here) |
| deployer command site | `go/pkg/cli/localcommands/daemon.go:62-81` dispatch (`install|uninstall|status|migrate-db|owner-ddl` — add `deploy`); pattern `runDaemonOwnerDDL` `:90`, `runDaemonMigrate` `:167`; admin DSN `EnvDaemonAdminDBURL` `:24` |
| runtime apply engine to reuse / wrap (C3) | `go/pkg/db/migrations.go:121` `ApplyMigrations` (`current = migration.Version` `:171`); `applyOne` `:304-355` (BeginTx → SQL → version stamps `:320` → in-tx hash verify → Commit; **no `SET ROLE`/`ALTER OWNER`/grant** — the C3 gap `applyRuntimeStep` wraps); lock `:18`; preflight `:158`; frontier `:17` (=43; new substrate migration = 0044) |
| owner-bundle engine / per-tx atomicity / watermark + forward-tolerance (C2) + 0021 exclusion (C3) | `go/pkg/db/owner.go:265-303` `ApplyOwnerBundles`, `:309-320` `applyPendingOwnerBundles` (**bound to `<= LatestOwnerBundleVersion (20)` so it skips 0021**); `:511-541` `applyOneOwnerBundle` (one tx, stamp last `:528-535` — the terminal-0021 step + its receipt reuse this); `CheckOwnerBundleWatermark` `:124-154` (tolerate-forward `:151-153` — the forward-watermark rule hooks here); `OwnerBundleVersion`/`MAX(version)` `:228-247`; frontier `:23` (`LatestOwnerBundleVersion = 20`, **stays 20**; new constant `DDLRevokeOwnerBundleVersion = 21`); `RequiredOwnerBundleVersion` `:35` (= 20, **stays 20**); `revokeEmbedded` = 0021 file in `ownerBundleFS` `:156` |
| current owner-bundle frontier (NOT the revoke) | `go/pkg/db/sql/owner/0020_owner_bundle_watermark_read.sql` (`GRANT SELECT` on `owner_bundle_meta`, #581 / RFC 0142 Layer 2; label `owner.go:179`) — the highest **non-revoke** bundle; the DDL-revoke is the NEW 0021 |
| C3 prerequisite the revoke-last fix relies on | `go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:64-66` (prereq: `striatumd_rw` MUST hold CREATE on schema striatumd), `:98` (`GRANT CREATE ON SCHEMA striatumd TO striatumd_rw` before the `:102` transfer loop); `0019_supervisor_pointer_runtime_ownership.sql:53-80` (same prereq + grant) — 0018 is in the non-revoke owner prefix, so CREATE is held when the runtime reconciles run |
| fingerprint contract (bind plan_hash to) | `go/pkg/db/schema_drift.go:83` `ExpectedFingerprint`; `:105-137` `composeFingerprint` (reused for `plan_hash`); `:145` `LiveFingerprint`; `:171` `RecordSchemaFingerprint` (C1 finalizer step 2); `:254-274` `CheckSchemaDrift` (SHADOW default `:268-271`; `EnvSchemaDriftRefuse` const `:28`, shadow note `:42,199`) |
| `deploy_cursor` + `deploy_plan` table model (incl. `finalizing` state + immutable transcript) | `go/pkg/db/sql/0043_schema_state.sql:39-40` (singleton CHECK), `:45-52` (`striatumd_rw` GRANT `DO` block) — model for both new 0044 tables |
| per-step / terminal receipt SD fn | `go/pkg/db/sql/owner/0001_authority_phase0.sql:152` (`append_audit_row` SD fn), `:223` (`REVOKE ALL … FROM PUBLIC`), `:237` (runtime direct `audit_log` INSERT revoked), `:238` (`GRANT EXECUTE … TO striatumd_rw`) — runs in the caller's transaction, so the per-step receipt joins the owner-connection step tx |
| doctor block model (per-step tightened, transcript-enumerated) | `go/pkg/reads/doctor_schema_drift.go:26-77` (`schemaDriftDoctorBlock`) |

<sub>Holder artifact (v4 revision) for the RFC 0142 P4 falsification-gate design run.
Resolves cycle-3 findings BC-N1 (materialize the immutable ordered transcript into
`deploy_plan` before step 0; resume off the stored transcript; stable `(plan_hash,
step_index)`; §1.3 + per-step doctor keyed to the transcript;
`T-deploy-plan-hash-resume-after-step`) and BC-N2 (the universal pre-revoke
`deploy_cursor`-authoritative edge in `CheckDeployActivation` for every deployer-aware
binary at `applied_owner == 20`; extended F11 + `G-old-binary-refuse`), and carries C1 + C2
+ C3 forward verbatim, re-anchored to current `main` (DDL-revoke bundle renumbered 0020→0021;
`Latest`/`Required` owner-bundle frontier 19→20). The cycle-4 falsifiers (atomicity/resumability
lens; decoupling/migration-safety lens) re-attack §5 (esp. F14, F11, F13) and the tensions in
§8; the adjudicator's collaboration ledger — not falsifier completion — decides whether the
gate clears.</sub>

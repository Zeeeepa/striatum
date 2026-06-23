# HOLDER (revised, v3) — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: holder-author-001

> **This is the THIRD REVISION (v3).** Design-v1 (`rfc-0142-p4-design`) returned
> `needs_revision` with three findings C1/C2/C3. Design-v2 (`rfc-0142-p4-design-v2`)
> **resolved C1 and C2** — both falsifiers conceded the finalization-boundary shape
> (C1) and the fail-closed activation edge (C2) are genuinely closed — but returned
> `needs_revision` **again** because **C3 (ownership policy) stayed open** (the
> Policy-1 `ALTER … OWNER TO striatumd_rw` reconcile needs the `CREATE ON SCHEMA
> striatumd` privilege that this phase's bundle 0020 revokes) and a **new finding N1
> (per-step receipt not crash-safe)** landed.
>
> This spec **starts from the v2 `HOLDER.md`**
> (`docs/operator/artifacts/rfc-0142-p4-design-v2/dialogue/holder/HOLDER.md`) and is a
> **proper revision**: it **resolves C3 and N1** per the prescribed fixes in the v2
> collaboration ledger
> (`docs/operator/artifacts/rfc-0142-p4-design-v2/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`,
> the `findings:` block + §4) and `SEED.md`, and **carries C1 + C2 forward
> unregressed**. §0.1 below is the auditable resolution map; the cycle-3 falsifiers
> should verify each finding there, then re-attack §5 (esp. **F12** and the new
> **F13**) and the tensions in §8.
>
> It is a falsifiable implementation spec for RFC 0142 **P4 only** — the one-shot
> deployer that lifts schema mutation out of serve-boot — that `rfc-0142-p4-build`
> can execute contract-first (TDD). RFC 0142 is accepted (D258); this does NOT
> re-open the five-layer design. Primary source:
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (esp. Layer 3,
> the Phasing P4 row, Open Questions 3 + 4). Anchors re-verified against the worktree
> base of `striatum/rfc-0142-p4-design-v3` (cut from `main`); every file:line below
> is current source.

## 0. Root reframe held

Schema mutation must stop being an **implicit side effect of the serving process's
restart** and become an **explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer**. Then the serving daemon holds **zero DDL on the
serving path**, "restart force-commits a half-applied deploy" becomes structurally
impossible, and a bad migration can never wedge the single writer on boot. Everything
below serves that one move; nothing pulls in P5 (rehearse / expand-contract /
fidelity tiering / clone = Q1/Q2).

---

## 0.1 Addressing the design-v2 findings (the gating requirement)

The two binding v2 findings (C3, N1) are each resolved by a concrete sub-protocol /
chosen-and-tested mechanism — not a claim. C1 and C2 are carried forward **verbatim**
from the v2 HOLDER and re-confirmed unregressed. The "where" column names the section
that specifies it and the named test that would refute the resolution.

| Finding | v2 status | v3 disposition | Where / refuting test |
| --- | --- | --- | --- |
| **C3** ownership transfer self-contradicts the bundle-0020 CREATE revoke | **OPEN** (reopened by falsifier_2) | **RESOLVED via resolution (a): sequence the revoke last.** Bundle 0020 (`REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw`) is **special-cased out of the owner-prefix and the `owner-ddl apply` path** and applied as the **TERMINAL step of the deploy plan**, AFTER every runtime step + ownership reconcile. So each `ALTER … OWNER TO striatumd_rw` runs while `striatumd_rw` still holds CREATE (0020 not yet applied), and the committed steady state still denies CREATE. The v2 F12 internal inconsistency dissolves because the two facts hold at **different plan times**: `relowner == striatumd_rw` during the reconcile (CREATE held), `striatumd_rw CREATE → 42501` after the terminal 0020 commits. | §3.2 (plan ordering), §3.3b (mechanism), §4.4 (no-lockout sequencing) / **`T-deploy-runtime-object-ownership` (F12, revised)** |
| **N1** per-step deploy receipt not crash-safe | **OPEN** (new, falsifier_1; Q3/R4 core) | **RESOLVED: per-step receipt is atomic-or-idempotent, keyed on `(plan_hash, step_index)`.** Q3-A (transactional step) — the per-step receipt append (`append_audit_row`, owner connection) joins the **same owner-connection step transaction** as the DDL + ownership reconcile + grants + version stamps + cursor advance, so `step_committed(k)` is durable **iff** its receipt is durable. Q3-B (NT-DDL step) — the `in_progress(k)` reconciler appends **exactly one** receipt keyed on `(plan_hash, step_index)` (+ `step_id`, `sha256`) **before** writing `step_committed(k)`, resolving duplicate-vs-skip. `doctor schema_deploy_unrecorded` is tightened to surface a **missing per-step** receipt, not just the terminal one. | §3.3b (Q3-A tx), §3.4 (Q3-B keyed reconcile + doctor) / **`T-deploy-receipt-crash-resume` (F13, new)** |
| **C1** finalization boundary unclassifiable | **RESOLVED** (v2; both falsifiers conceded) | **CARRIED FORWARD VERBATIM, unregressed.** The distinct `finalizing` cursor state, the idempotent finalizer (receipt → fingerprint → `complete` last), the §1.3 resumable-finalization row, the operator pin (no single finalization tx spans owner+runtime connections), and F10 are preserved exactly. Kept **coherent** with the new N1 per-step rule (see §3.4 "C1 ⊕ N1 coherence"). | §1.2, §1.3, §3.4 / **`T-deploy-resume-finalization-crash` (F10)** |
| **C2** activation not fail-closed | **RESOLVED at the edge** (v2; both falsifiers conceded) | **CARRIED FORWARD VERBATIM, unregressed — and now *operationally* complete.** `CheckDeployActivation` before `ApplyMigrations`, absent-cursor-as-incomplete, the typed `awaiting_deploy`/`awaiting_deploy_config` halts, the forward-watermark rule, `RequiredOwnerBundleVersion` kept at **19**, and F11 are preserved exactly. The v2 caveat (the `awaiting_deploy` remediation was unsatisfiable because C3 stranded the deploy) is **closed by the C3 fix**: the deploy can now complete, so the C2 halt's only exit is satisfiable. | §3.3a, §4.2, §4.3 / **`T-deploy-revoke-activation-ordering` (F11)** |

**Carried forward unchanged (v1/v2 ledgers — do not re-litigate):** Q4 (plain verb +
three seams, §2); the per-step Q3-A/Q3-B mid-step resumability **body** + step
taxonomy (§1.1, the `in_progress`/`step_committed` body of §1.2 — N1 extends this with
the *receipt*, it does not change the schema/cursor body); the shadow-first decoupling
shape (default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify` preserving the P2
watermark interlock + P3 drift gate/self-record, the embed-FS-derived plan, the
hash-chained receipt, `doctor schema_deploy_unrecorded`). The v3 revision touches only
the **C3 ownership/ordering edge** and the **per-step receipt (N1)**.

---

## 1. Decision Q3 — How atomic is "atomic"? (the hard correctness core)

**Claim (Q3-DECISION).** The **per-step-atomic + resumable-cursor** contract is
sufficient for *every* owner+runtime interleaving P4 ships, **provided** the two
per-step sub-rules (Q3-A/Q3-B) hold for **both the schema side effect AND the per-step
receipt** (N1, new in v3), **and** the finalization boundary is closed by a distinct
`finalizing` state + an idempotent finalizer (C1, v2). No step needs a cross-step
transaction; no finalization transaction spans the owner+runtime connections
(structurally impossible — §3.4); and **no per-step provenance write is left in a
crash window the resume cannot reconcile to exactly-one receipt** (§3.3b, §3.4).

- **Q3-A — transactional step (carry-forward body; receipt added by N1).** If a
  step's DDL can run inside a transaction, the **cursor advance, the C3 ownership
  reconcile, the version stamps, AND the per-step receipt commit in the SAME
  transaction as the step** (over the owner connection post-activation — §3.3b).
  Crash → the whole transaction rolls back → cursor unchanged, no receipt → resume
  re-enters the step cleanly. Strictly safer than separate writes, so always taken
  when available. **`step_committed(k)` is durable iff its receipt is durable.**
- **Q3-B — idempotent-resume reconciler where the step is NOT transactional
  (carry-forward body; receipt keyed by N1).** `CREATE INDEX CONCURRENTLY`, `ALTER
  TYPE … ADD VALUE`, and the handful of auto-committing `ALTER`s cannot share a
  transaction with the cursor or the receipt. The deployer writes an **`in_progress`
  marker BEFORE the step**, runs the step in its **idempotent / self-reconciling
  form**, then in one owner-connection reconcile transaction **appends exactly one
  receipt keyed on `(plan_hash, step_index)` and writes the `step_committed` marker**.
  Crash window is *between* the step's own commit and the reconcile; on resume the
  cursor reads `in_progress(k)` and re-enters step k through its reconciler (a no-op
  or repair by construction), which idempotently appends the (still-missing) receipt
  exactly once before advancing.

### 1.1 Step taxonomy (the closed set P4 ships) — unchanged from v1/v2

| Class | Examples | Transactional? | Protocol | Idempotent form |
| --- | --- | --- | --- | --- |
| **T-DDL** runtime | `CREATE TABLE`, additive `ALTER … ADD COLUMN`, `CREATE INDEX` (non-concurrent), the new runtime migrations themselves | Yes | **Q3-A** (step + ownership-reconcile + version stamps + cursor + **receipt** in one tx) | `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`; re-run is a no-op |
| **T-DDL** owner | owner-bundle DDL (authority/grant/CHECK changes), **incl. the terminal DDL-revoke bundle 0020** | Yes (each bundle applies in one tx that stamps `owner_bundle_meta` last — `owner.go:498-528`) | **Q3-A**, reusing the existing per-bundle transaction; cursor advance + **receipt** join that tx | bundles already idempotent (`IF NOT EXISTS` + `ON CONFLICT (version) DO NOTHING`) |
| **NT-DDL** | `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, `DROP INDEX CONCURRENTLY` | No | **Q3-B** (pre-marker → step → reconcile: keyed receipt + post-marker) | `CREATE INDEX CONCURRENTLY IF NOT EXISTS`; **resume reconciler** drops a left-behind `INVALID` index then recreates; `ADD VALUE … IF NOT EXISTS` (PG12+) |
| **DML-backfill** (bounded in P4; full online backfill is P5) | a one-shot data normalize a runtime migration carries | Yes if small; batched otherwise | Q3-A small / Q3-B batched | `WHERE <not-yet-done>` predicate; re-run resumes from the unprocessed tail |

> The only genuinely hard per-step corner is NT-DDL, reducing to one rule: **a
> half-built `CREATE INDEX CONCURRENTLY` leaves `pg_index.indisvalid = false`**; the
> fingerprint MUST classify that as "incomplete, resume", never "unknown drift,
> panic" — see §1.3. **NT-DDL never creates a new ownable object whose owner is
> wrong:** a CONCURRENTLY index inherits its table's owner (already `striatumd_rw`),
> and `ALTER TYPE … ADD VALUE` creates no relation. So the C3 ownership reconciliation
> (§3.3b) is needed only on transactional object-creating steps, where it runs in the
> step transaction. The **terminal DDL-revoke bundle 0020 is a T-DDL owner step**
> (the existing per-bundle transaction), so its receipt also commits atomically.

### 1.2 `deploy_cursor` states and crash-resume semantics (C1 carry-forward)

`deploy_cursor` is a **new runtime-owned singleton table** (runtime migration
**≥ 0044**, modeled byte-for-byte on `0043_schema_state.sql:39-52`: a `CHECK (id =
'singleton')` PK + the `striatumd_rw` GRANT `DO` block). Columns:

```
deploy_cursor(
  id            text PK CHECK (id='singleton'),
  plan_hash     text NOT NULL,         -- the plan this cursor is advancing
  state         text NOT NULL,         -- enum below (CHECK constraint)
  step_index    int  NOT NULL,         -- 0-based index into the ordered plan (the N1 key)
  step_id       text NOT NULL,         -- e.g. "runtime:0044" / "owner:0020"
  updated_at    timestamptz NOT NULL DEFAULT now()
)
```

`state ∈ { idle, in_progress, step_committed, finalizing, complete, aborted }`. The
**`finalizing` state (C1, v2)** is preserved. Lifecycle for a plan of N steps — where,
under §3.2, the **terminal step N-1 is the DDL-revoke bundle 0020 whenever it is in
the plan** (C3 resolution (a)):

```
idle ─▶ in_progress(0) ─▶ step_committed(0) ─▶ … ─▶ step_committed(N-1=0020) ─▶ finalizing ─▶ complete
                 │                                                                
                 └─(operator abort / fatal non-resumable error)──▶ aborted        
```

- **Q3-A step (carry-forward body + N1 receipt):** the transition `in_progress(k) →
  step_committed(k)`, the step DDL, the C3 ownership reconcile (§3.3b), the version
  stamps, **and the per-step receipt** are **one transaction**. No observable
  `in_progress(k)` and no missing-receipt `step_committed(k)` after a crash on a
  transactional step. Resume reads `step_committed(k)`, advances to `k+1`.
- **Q3-B step (carry-forward body + N1 keyed receipt):** `step_committed(k-1) →
  in_progress(k)` (marker), the step's own auto-commit, then a single reconcile tx
  that appends the keyed receipt and writes `in_progress(k) → step_committed(k)`.
  Crash resume reads `in_progress(k)` and re-enters step k through its reconciler.
- **Finalization (C1 — carry-forward verbatim).** When `step_index == N-1` (the
  terminal 0020 step, if present) is `step_committed`, the deployer advances the
  cursor to **`finalizing`** in one commit, then runs the **idempotent finalizer**
  (§3.4): (1) append the **`complete` deploy receipt** (owner connection,
  `append_audit_row`) — guarded idempotent: skip if a receipt for `(plan_hash,
  state=complete)` already exists; (2) **`RecordSchemaFingerprint`** UPSERT into
  `schema_state` (idempotent; deterministic `ExpectedFingerprint()`, == this plan's
  recorded fingerprint by §3.2); (3) advance `finalizing → complete` **last**. Because
  `complete` is written **only after** receipt and fingerprint are durable — **and,
  by §3.2, only after 0020 has committed as step N-1** — a `complete` cursor is proof
  the deploy is fully finalized **and** the steady-state CREATE-revoke is in force. A
  crash anywhere in finalization leaves the cursor at `finalizing`, and the finalizer
  is idempotent, so re-run repairs exactly once.

### 1.3 The load-bearing disambiguation (C1 carry-forward)

`deploy_cursor` is the **second signal** alongside the drift gate's `LiveFingerprint
== ExpectedFingerprint`. The table below is the single most important falsifiable
artifact in the spec; the **`finalizing` row (C1)** and the `state == complete`
tightening are preserved verbatim from v2.

| `deploy_cursor.state` | `plan_hash` | fingerprint | classification | action |
| --- | --- | --- | --- | --- |
| `complete` | == expected | match | **in sync** | serve |
| `in_progress` / `step_committed` | == the binary's plan | (any) | **incomplete, resume** | refuse-to-serve `awaiting_deploy`; `striatum daemon deploy` resumes at the cursor |
| **`finalizing`** | **== expected** | **(any — match or mismatch)** | **resumable finalization** (C1) | **refuse-to-serve `awaiting_deploy`; re-run runs the idempotent finalizer; NEVER serve, NEVER genuine-drift** |
| absent table / row | n/a | (any) | **deploy not started / pending** | refuse-to-serve `awaiting_deploy` (C2 bootstrap-order case) |
| `complete` but `plan_hash` ≠ expected, OR `aborted` | foreign / n/a | mismatch | **genuine drift** | refuse-to-serve (existing `schema_drift` halt) |
| `complete`, `plan_hash` == expected | == expected | **mismatch** | **genuine post-deploy drift** | refuse-to-serve (`schema_drift`) — unambiguous because `complete` ⇒ fingerprint was durable |

The C1 crash window (`complete` with the **expected** `plan_hash` but a missing
receipt/fingerprint) **cannot occur**: under §1.2, `complete` is the final write.
**v3 addition (C3 coherence):** because 0020 is the terminal plan step (§3.2),
`complete` additionally implies 0020 is applied, so a `complete`+in-sync classification
never serves a database where `striatumd_rw` still holds CREATE on the serving path.

---

## 2. Decision Q4 — Is a deploy itself a Striatum run? (carry-forward, sound)

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape door
is held open by three already-present seams.** Unchanged from v1/v2; neither falsifier
contested it.

**Why not a run now — the bootstrapping paradox is dispositive.** A dogfooded run
needs the daemon up and its schema present to drive `runs`/`jobs`/`sessions`/the
capability ledger. But the deploy is *precisely the operation that brings a fresh or
owner-only database to a serveable schema* (successor to boot-time `ApplyMigrations`
and `daemon migrate-db`, `daemon.go:167-210`). So the base deployer is a **plain verb
against a bare / owner-only DB with no running daemon**, the same context `daemon
owner-ddl apply` (`daemon.go:90-159`) already occupies.

**What keeps the run-shape door open (so P5 needs no re-litigation):** (1) the plan is
a content-addressed artifact (`plan_hash`, §3.2); (2) the deploy receipt is
hash-chained into owner-held `audit_log` (§3.4) — **now one receipt per step, N1**;
(3) the verb is a thin CLI over a `Deployer` package with a pure `Apply(ctx, plan,
cursor)` core a future run step invokes unchanged. **Falsifiable boundary:** if a
falsifier shows a real fresh-DB bring-up the plain verb cannot perform but a run
could, Q4 flips.

---

## 3. The deployer surface + serve-boot decoupling (shadow-first, by exact code site)

### 3.1 Command site (carry-forward; connection model sharpened for N1)

Add `case "deploy":` to the dispatch in `RunDaemon` (`go/pkg/cli/localcommands/daemon.go:67-81`,
alongside `install|uninstall|status|migrate-db|owner-ddl`) → new `runDaemonDeploy(...)`.
DSN resolution mirrors `runDaemonOwnerDDL` (`daemon.go:95-121`): `--owner-url` /
`--admin-url`, then `STRIATUM_DAEMON_ADMIN_DB_URL` (`EnvDaemonAdminDBURL`,
`daemon.go:24`), then the daemon DSN. Flags: `--plan` (default embedded), `--dry-run`
(print ordered plan + cursor delta, mutate nothing), `--resume` (default; explicit for
clarity), `--json`. `docs/reference/command-authority-matrix.md` and the authority
guardrail tests gain the `daemon.deploy` row (AGENTS.md change discipline).

**Connection model (load-bearing for N1 — stated precisely).** The deployer resolves
the owner/admin DSN and applies **every deploy-plan step over the SINGLE owner
connection**: the runtime migration DDL, the C3 ownership reconcile, the grant
re-assert, the `schema_migrations`/`schema_meta` version stamps, the runtime-owned
`deploy_cursor` advance, **and the per-step `append_audit_row` receipt**. This is
legal because the owner role is a **member of `striatumd_rw`** (or superuser) in the
two-role bootstrap (`authority_bootstrap.go`) — so it can write the rw-owned
`deploy_cursor`/`schema_state` and `ALTER … OWNER TO striatumd_rw` (§3.3b) — and owns
(thus can EXECUTE) the owner-only SD function `append_audit_row`
(`owner/0001_authority_phase0.sql:223,238`), which **runs in the caller's
transaction**. The deployer asserts this membership precondition at start. The
**only** writes the deployer routes over a separate **runtime view** are the C1
finalizer's terminal `schema_state` fingerprint self-record (deliberately mirroring
the boot self-record path byte-for-byte — §3.4), which is exactly why the *terminal*
finalizer is two-connection-idempotent while *per-step* writes are single-connection
atomic.

### 3.2 The deploy plan — derived + embedded, with the DDL-revoke bundle sorted last (REVISED for C3)

**Claim (PLAN-DERIVED).** The plan is generated deterministically from the same two
embedded `embed.FS` sources the fingerprint already hashes — the runtime migration FS
(`migrations.go:21` `//go:embed sql/*.sql`) and the owner-bundle FS (`owner.go:156`
`//go:embed sql/owner/*.sql`) — **not** a hand-authored manifest.

`BuildPlan(applied_owner, applied_runtime) → DeployPlan`:

- **Steps** = pending **non-revoke** owner bundles (ascending `Version` **< 20**,
  tagged `owner`, `owner.go:196-223`) **then** pending runtime migrations (ascending
  `Version`, tagged `runtime`, `migrations.go:31-105`) **then**, as the **TERMINAL
  step, the DDL-revoke bundle 0020** if pending (tagged `owner`, sorted last). This
  reproduces today's owner-before-runtime semantics for ordinary bundles (the
  watermark interlock already requires owner ≥ N before any runtime apply,
  `connection.go:349-352`, `owner.go:124-154`) and promotes it into explicit
  dependency edges (every `runtime` step depends on the full non-revoke `owner`
  prefix). **The C3 change:** the DDL-revoke bundle (identified by the named constant
  `DDLRevokeOwnerBundleVersion = 20`) is **special-cased out of the owner prefix and
  appended as the last step**, with a dependency edge on **every** prior step, so it
  commits **after the final runtime ownership reconcile**.
- **0020 is excluded from the `owner-ddl apply` watermark path (load-bearing for C3).**
  `LatestOwnerBundleVersion` stays **19** (the highest *non-revoke* bundle = the
  `owner-ddl apply` frontier), so `RequiredOwnerBundleVersion` stays **19** (C2
  carry-forward). `ApplyOwnerBundles` (`owner.go:264-302`, the `owner-ddl apply` path)
  is bounded to apply only bundles `<= LatestOwnerBundleVersion` (≤ 19) and **skips
  0020**, which is **deploy-plan-only**. A new build guard asserts no owner bundle
  `>= DDLRevokeOwnerBundleVersion` is reachable through `owner-ddl apply`. This closes
  the early-revoke door: the *only* applier of 0020 is the deployer, as the terminal
  plan step, so 0020 can never commit before the runtime ownership reconciles via any
  path (owner prefix, `owner-ddl apply`, or a stray apply).
- Each step carries `{role, step_id, step_index, sha256, transactional: bool}`. The
  `transactional` bit is the Q3-A/Q3-B selector — a per-step annotation
  (`-- striatum:nontransactional` pragma) defaulting to transactional, validated by a
  build-time test that greps the SQL for known non-transactional verbs
  (`CONCURRENTLY`, `ALTER TYPE … ADD VALUE`) and asserts the pragma matches (a
  mis-annotated step reds the build, sibling of the landed
  `TestRunnerMigrationsHaveNoNonTransactionalDDL`, `migrations.go:296-299`).
- **`plan_hash`** = sha256 of the canonical step transcript (the ordered
  `step_id`+`sha256` list, **including 0020 in its terminal position**), reusing the
  `composeFingerprint` canonicalization (`schema_drift.go:105-137`). Bound to
  `ExpectedFingerprint` by construction: **a `complete` plan's recorded fingerprint ==
  `ExpectedFingerprint()`** (§3.4 step 2). The fingerprint is over the *set* of
  applied migrations+bundles (order-independent), `plan_hash` is over the *ordered*
  transcript, so the C3 re-ordering (0020 last) changes `plan_hash` but **not** the
  fingerprint — deployer and drift gate still never disagree on "is the whole plan
  applied," and `step_index` (the N1 receipt key) is stable across re-runs because the
  plan is content-addressed by `plan_hash`.

### 3.3 Lifting `ApplyMigrations` out of serve-boot (shadow-first; carry-forward)

**Claim (DECOUPLE).** Serve-boot stops mutating schema, behind a flag, shadow-first.
The single mutation site on the serving path is `ConnectAndMigrate`
(`connection.go:332-404`), reached via `BootstrapAndConnect(…, migrate)`
(`authority_bootstrap.go:181-201`), driven by `--migrate` (default `true`,
`main.go:76`).

1. **New env `STRIATUM_DEPLOY_DECOUPLED`** (default OFF — shadow-first, cf.
   `EnvSchemaDriftRefuse`, `schema_drift.go:15-28`). OFF → boot behaves exactly as
   today; the decoupling lands to `main` **inert**.
2. When **ON**, `ConnectAndMigrate` becomes **`ConnectAndVerify`**: keep the Layer 2
   watermark interlock (`connection.go:349-352`) and the Layer 3 drift gate
   (`connection.go:376-383`) but **do NOT call `ApplyMigrations` (`connection.go:353`)**.
   Instead run the §1.3 classification and serve only on the in-sync row; on any other
   row refuse-to-serve via the typed `awaiting_deploy` halt (§4.2). The drift
   self-record (`connection.go:394-402`) is **NOT** run on the verify path — only the
   deployer records the fingerprint (§3.4), so a verify-only boot can never overwrite
   `schema_state` to falsely claim in-sync.
3. **No window serving an unmigrated schema (R2).** The decoupled boot verifies before
   serving and refuses on any non-in-sync state; the watermark interlock is preserved
   verbatim.

#### 3.3a The deploy-activation interlock (C2 — carry-forward verbatim): fail closed before mutation

A new **`CheckDeployActivation(ctx, runner, decoupledEnabled bool) error`** (in
`go/pkg/db/owner.go` or a new `deploy_activation.go`) is called **at the same boot
site, immediately after `CheckOwnerBundleWatermark` and BEFORE `ApplyMigrations`**
(`connection.go:349-353`), in **both** `ConnectAndMigrate` and `ConnectAndVerify`. It
mutates nothing. Predicate (fail-closed):

- Let `revokeEmbedded` = the embedded owner-bundle FS contains the bundle at
  `DDLRevokeOwnerBundleVersion = 20` (the bundle at/after which `striatumd_rw` loses
  `CREATE ON SCHEMA striatumd`). **This is "the binary ships the revoke," NOT
  `Latest >= 20`** — `LatestOwnerBundleVersion` stays 19 (the watermark frontier) even
  for a revoke-embedding binary, because 0020 is deploy-plan-only (§3.2). The activation
  binary is detected by the *presence of the 0020 file in `ownerBundleFS`*, not by the
  watermark frontier.
- **If `!revokeEmbedded`** (a pre-revoke binary): the activation interlock is inert
  *here*, but the **forward-watermark rule** (§4.2) in `CheckOwnerBundleWatermark`
  halts this binary if it observes `applied_owner >= 20`. Return nil.
- **If `revokeEmbedded`**: this binary ships the revoke and MUST NOT auto-apply. Read
  `applied = OwnerBundleVersion(...)` (the `42501`-unreadable case is already routed to
  `awaiting_owner_ddl` by `CheckOwnerBundleWatermark`, `owner.go:135-137`). Then:
  - `decoupledEnabled == false` → **`awaiting_deploy_config`** halt: "this binary ships
    the RFC 0142 DDL-revoke; set `STRIATUM_DEPLOY_DECOUPLED=1` and run `striatum daemon
    deploy`." DB untouched.
  - `decoupledEnabled == true` → classify the deploy per §1.3, **reading `deploy_cursor`
    defensively**: absent table/row → **incomplete → `awaiting_deploy`**, never
    error-on-missing-table; present and `state != complete` (or `plan_hash`/fingerprint
    ≠ expected) → **`awaiting_deploy`**; only `complete` + in-sync → return nil and
    serve (verify-only). DB untouched in every non-serving case.

Net invariant (the C2 close, carried forward): **`ApplyMigrations` is never reached
over the runtime runner once the revoke is in play.** Either the binary is
revoke-aware and halts before mutation, or it is revoke-unaware and the
forward-watermark rule halts it. **v3 note:** because 0020 only commits as the terminal
deploy step (§3.2), `applied_owner == 20` is observable *only after* a `complete`
deploy, so the forward-watermark rule and `applied >= 20` reasoning are unchanged in
meaning; the watermark simply reaches 20 at the *end* of the deploy rather than its
start.

#### 3.3b Runtime-object ownership: Policy 1 with the revoke sequenced last (REVISED — C3 resolution (a))

The v2 gap (C3, reopened): Policy 1 reconciles ownership with `ALTER <kind>
striatumd.<name> OWNER TO striatumd_rw`, but PostgreSQL requires the **new** owner to
hold `CREATE ON SCHEMA striatumd` — the repo's own bundles document this and **GRANT
`CREATE` FIRST** for exactly this reason:
`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:64-75` ("striatumd_rw
MUST hold CREATE on schema striatumd … otherwise `ALTER … OWNER TO striatumd_rw` fails
`permission denied for schema striatumd`") and `:98` (`GRANT CREATE ON SCHEMA striatumd
TO striatumd_rw` before the `FOREACH … ALTER TABLE … OWNER TO` loop at `:99-104`);
`go/pkg/db/sql/owner/0019_supervisor_pointer_runtime_ownership.sql:53-80` repeats the
prerequisite and grant. In v2 the plan applied 0020 (`REVOKE CREATE`) in the owner
prefix, **before** the runtime reconcile — so post-0020 the reconcile failed
`permission denied`.

**Resolution (a) — sequence the revoke last (SEED-recommended; chosen).** Bundle 0020
is the **terminal** plan step (§3.2). The runtime ownership reconciles all run while
`striatumd_rw` **still holds CREATE** (granted idempotently by bundle 0018 at
`0018:98`, which is in the non-revoke owner prefix and therefore applied before any
runtime step). 0020 commits **last**, so the **committed steady state still denies
CREATE**. No transient elevated grant is introduced (unlike resolution (b)); no §4.1
safety-claim retraction is needed (unlike resolution (c)).

**Why (a) over (b)/(c):** (b) introduces a transient `GRANT CREATE` window the spec
would have to prove is never observable by a serving boot; (c) makes new runtime
objects owner-owned and forces a §4.1 correction + a per-migration DML-grant guard. (a)
needs only a plan-ordering rule the plan generator already special-cases, and it makes
the C1.3 classification *more* coherent (`complete` ⇒ 0020 applied ⇒ CREATE denied).

**Chosen policy (one, tested): runtime objects stay `striatumd_rw`-owned.** Preserves
the landed invariant (bundles 0018/0019 transferred runtime-table ownership *to*
`striatumd_rw`, `owner.go:177-178`); keeps §4.1's residual-capability argument intact
(no §4.1 correction needed); keeps every runtime object uniform regardless of whether
legacy auto-apply or the deployer created it.

**Mechanism (`Deployer.applyRuntimeStep`, a deployer-owned wrapper around the existing
`applyOne` body, `migrations.go:304-355`).** `SET ROLE striatumd_rw` is **not** usable
for the create (it would change who owns the new object and, post-0020, `striatumd_rw`
lacks CREATE), so ownership is reconciled **after** the DDL, **inside the same owner-
connection step transaction**:

1. `BEGIN` (owner connection). **Deployer precondition assert (defense-in-depth):**
   `has_schema_privilege('striatumd_rw','striatumd','CREATE')` is `true`; if not, halt
   legibly (`deploy_create_prerequisite_missing`) **before** any DDL rather than
   failing with a raw `42501` mid-plan. By plan ordering this always holds (0018 grants,
   0020 is terminal), so the assert is a guard, not the mechanism.
2. Snapshot the owner-role-owned object oids in schema `striatumd` (a catalog read over
   `pg_class` for relkinds `r,p,i,I,S,v,m,t` plus `pg_proc`/`pg_type` if a future
   runtime migration ever creates those kinds — currently none do).
3. Run the migration SQL (the existing `applyOne` DDL + both version stamps + in-tx
   hash verify, `migrations.go:315-349`).
4. Re-snapshot; for **exactly the newly-appeared oids** run `ALTER <kind>
   striatumd.<name> OWNER TO striatumd_rw`. Indexes inherit their table's owner (no
   independent `ALTER INDEX OWNER`), an owned sequence appears as its own new
   `relkind=S` oid and is reassigned too — so the catalog diff covers tables,
   partitions, indexes, sequences, views, matviews, and **future object kinds** without
   enumerating them by hand. Scoped to **new** oids, so it **never** touches
   pre-existing owner-owned authority/bundle objects, and **never** an owner step's
   objects (owner steps are not object-creating runtime steps).
5. Re-assert the migration's DML grants to `striatumd_rw` (the `GRANT` block the
   migration file carries, e.g. `0043_schema_state.sql:46-52`) so a created-as-owner-
   then-reassigned object has the serving role's DML regardless of default ACLs.
6. **Append the per-step receipt** via `append_audit_row(...)` (N1, §3.4) — same
   transaction.
7. Advance `deploy_cursor` `in_progress(k) → step_committed(k)` (Q3-A: same
   transaction). `COMMIT`.

The reconciliation is **idempotent**: on resume an already-rw-owned object is not in
the "owner-role-owned" snapshot, so it is skipped. It runs only on transactional
object-creating runtime steps (NT-DDL creates no mis-owned object, §1.1). The owner
role can `ALTER … OWNER TO striatumd_rw` because (i) it is a member of that role (§3.1)
and (ii) `striatumd_rw` still holds CREATE at this plan time (0020 not yet applied).

### 3.4 The deploy receipt: per-step crash-safe (REVISED — N1) + the idempotent finalizer (C1 carry-forward)

**Claim (RECEIPT).** Every applied step writes **exactly one** hash-chained deploy
receipt into the owner-held `audit_log` via the owner-only SD function
`append_audit_row(...)` (`owner/0001_authority_phase0.sql:223,238`; runtime direct
`INSERT` revoked at `:237`). Payload: `{plan_hash, step_index, step_id, role, sha256,
state, prev_receipt_hash}`. `state ∈ {step_committed, complete}` (per-step vs terminal).

**N1 — per-step receipt crash-safety (the new fix).** The per-step receipt is made
atomic-or-idempotent with the step, keyed on **`(plan_hash, step_index)`** (the plan is
content-addressed, so `step_index` is stable across re-runs):

- **Q3-A (transactional step):** the receipt append occurs in the **same owner-
  connection transaction** as the DDL + ownership reconcile + grants + version stamps +
  cursor advance (§3.3b steps 1–7). Legal because the deployer applies runtime steps
  over the owner connection and `append_audit_row` runs in the caller's transaction.
  Therefore **`step_committed(k)` is durable iff its receipt is durable** — there is no
  crash window between a transactional step's commit and its receipt. The same holds
  for the **terminal T-DDL owner step 0020** (its receipt joins the per-bundle
  transaction, `owner.go:498-528`).
- **Q3-B (non-transactional step):** the receipt cannot be atomic with the side effect.
  The `in_progress(k)` reconciler appends **exactly one** receipt keyed on `(plan_hash,
  step_index)` (+ `step_id`, `sha256`, `state=step_committed`) **before** writing
  `step_committed(k)`, in one owner-connection reconcile transaction. **Idempotency
  rule:** if a `step_committed` receipt for `(plan_hash, step_index)` already exists,
  the reconciler **skips** the append (exactly-once), then writes `step_committed(k)`.
  This resolves the v2 duplicate-vs-skip ambiguity: a crash after the receipt append
  but before `step_committed(k)` re-enters at `in_progress(k)`, finds the receipt
  present, skips, and advances; a crash before the receipt append re-enters, finds it
  absent, appends exactly one, and advances. The resume **never advances past a step
  lacking its receipt** because the cursor advance and the receipt are in the same
  reconcile transaction.

**C1 ⊕ N1 coherence (the SEED requirement).** Both are the *atomic-or-idempotent
provenance discipline*, at the **per-step** vs the **terminal** boundary:

- Per-step writes are all over the **single owner connection**, so a transactional step
  is **atomic** (one tx) and an NT-DDL step is **idempotent-keyed** on `(plan_hash,
  step_index)`.
- The **terminal finalizer** is **idempotent across two connections** because the
  `complete` receipt (owner connection) and the `schema_state` fingerprint self-record
  (deliberately over the **runtime** view, byte-identical to the boot self-record path)
  **cannot share one transaction** — the operator pin, preserved. Its idempotency is
  keyed on `(plan_hash, state=complete)`.

Both guarantee **exactly one** hash-chained receipt per boundary; a resume never serves
(`finalizing`/`step_committed`/`in_progress` ⇒ `awaiting_deploy`), never advances past a
step missing its receipt, and the terminal `complete` receipt stays exactly-once.

**Finalizer crash-point table (C1 — carry-forward verbatim):**

| Crash point | Cursor after crash | §1.3 classification | Re-run repair |
| --- | --- | --- | --- |
| after final step (0020) DDL+receipt commit, before `finalizing` | `step_committed(N-1)` | incomplete, resume | advance to `finalizing`, run finalizer |
| after `finalizing`, before receipt | `finalizing` | **resumable finalization** | receipt absent → append; fingerprint; `complete` |
| after receipt, before `schema_state` | `finalizing` | **resumable finalization** | receipt present → **skip (exactly-once)**; fingerprint; `complete` |
| after `schema_state`, before `complete` | `finalizing` (fingerprint now matches) | **resumable finalization** (cursor ≠ `complete` ⇒ never serve) | receipt skip; fingerprint UPSERT no-op; `complete` |
| after `complete` | `complete`, fingerprint match, receipt present | in sync | none — serve |

**Tightened `doctor schema_deploy_unrecorded` (REVISED — N1).** `doctor` gains/keeps
`schema_deploy_unrecorded` (modeled on `schemaDriftDoctorBlock`,
`go/pkg/reads/doctor_schema_drift.go:26-77`), now a **per-step** check: from the
embedded FS it reconstructs `BuildPlan` and the current applied frontier
(`schema_migrations` max + `owner_bundle_meta` max), determines which plan
`step_index`es are **applied** (committed), and WARNs when **any** applied step lacks a
`step_committed` receipt for its `(plan_hash, step_index)` — **not** masked by a present
terminal `complete` receipt. So the doctor is green **only when all committed steps have
receipts** (plus the terminal `complete` receipt once `complete`). The previous
terminal-only check (advanced `schema_state.fingerprint` without a `complete` receipt)
is retained as the terminal case.

---

## 4. Serving-role DDL revocation (owner bundle 0020), without lockout

**Claim (DDL-REVOKE).** The serving role `striatumd_rw` loses its remaining
**object-creation** DDL on the serving path, shipped as **owner bundle 0020**, applied
**only as the terminal step of `striatum daemon deploy`** (C3 resolution (a)) —
**activated only after the deployer is the active mutator and after every runtime
ownership reconcile** — so failure mode 1 becomes structurally impossible without
recreating a #512-class lockout and without the v2 self-contradiction.

### 4.1 What is revoked, precisely (carry-forward; argument now true-by-construction via C3)

The anchor table confirms `striatumd_rw` already holds **no DDL on owner-owned tables**
(`owner/0001_authority_phase0.sql:229-238`; preflight + build guard route owner-table
DDL to bundles, `migrations.go:158`). The residual serve-path DDL is **object creation
on the runtime schema**. Bundle 0020 revokes exactly that:

```
-- owner bundle 0020 (DDL-revoke): the serving path holds zero create-DDL.
-- DEPLOY-PLAN-TERMINAL ONLY (RFC 0142 P4 C3 resolution (a)); excluded from the
-- owner-ddl apply watermark path (LatestOwnerBundleVersion stays 19).
REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw;
REVOKE TEMPORARY ON DATABASE <db> FROM striatumd_rw;  -- if granted
```

`striatumd_rw` **retains** ownership of (and `ALTER`/`DROP` on) the runtime tables it
already owns — and, by the C3 policy (§3.3b), **new** runtime objects the deployer
creates are reconciled back to `striatumd_rw` ownership **before** 0020 commits, so the
§4.1 invariant "new runtime objects stay `striatumd_rw`-owned" **holds as a tested
property** (`T-deploy-runtime-object-ownership`, F12). The honest residual is unchanged:
the serving role could still `ALTER` its *own* runtime tables, but **the serving daemon
issues no DDL once `ApplyMigrations` is lifted (§3.3)**, so the serve *path* is
zero-DDL. Full capability revocation (owner re-owns runtime tables) remains a
**follow-up beyond P4** (it would re-open #442), named, not assumed.

### 4.2 Typed halts + the forward-watermark rule (C2 — carry-forward verbatim)

Two new sibling halts of `AwaitingOwnerDDLError` (`owner.go:58-97`), reusing the
non-restartable exit path (`main.go:200-228`, `exitAwaitingOwnerDDL`):

- **`ErrAwaitingDeploy` / `AwaitingDeployError`** (`awaiting_deploy`): a deploy is
  pending/incomplete (cursor absent, `in_progress`, `step_committed`, or `finalizing`).
  Remediation: "run `striatum daemon deploy`." DB untouched.
- **`ErrAwaitingDeployConfig` / `AwaitingDeployConfigError`** (`awaiting_deploy_config`):
  the binary ships the DDL-revoke but `STRIATUM_DEPLOY_DECOUPLED` is OFF. Remediation:
  "set `STRIATUM_DEPLOY_DECOUPLED=1` and run `striatum daemon deploy`." DB untouched.

`main.go` gains the matching `errors.As` arms next to the `AwaitingOwnerDDLError` and
`SchemaDriftError` arms (`main.go:208-228`).

**Forward-watermark rule (in `CheckOwnerBundleWatermark`, `owner.go:148-153`).** The
tolerate-forward branch is bounded by the revoke boundary: when **`applied >=
DDLRevokeOwnerBundleVersion (20)` and this binary does NOT embed the revoke** (the 0020
file is absent from `ownerBundleFS`), the watermark returns an **`AwaitingDeployError`**
("this database has activated the RFC 0142 DDL-revoke; this binary predates the one-shot
deployer and cannot serve it safely — upgrade to a deployer-aware binary") instead of
tolerating forward into `ApplyMigrations`. A revoke-aware binary still tolerates forward
across the boundary but is governed by `CheckDeployActivation` (§3.3a). Pre-0020 forward
tolerance is unchanged.

**Resolution of the v1 contradiction (auto-apply-default vs `Required = 20`), preserved.**
`RequiredOwnerBundleVersion` is **NOT** advanced to 20; it stays **19**. The DDL-revoke
is gated by `CheckDeployActivation` + the `STRIATUM_DEPLOY_DECOUPLED` flag + its
deploy-plan-terminal placement, **not** by the watermark frontier. The inert-landing
binary has `Latest = Required = 19` and auto-applies (legacy default intact); the
activation binary embeds the 0020 file (still `Latest = Required = 19`, because 0020 is
deploy-only — §3.2) but **never auto-applies** — `CheckDeployActivation` halts it before
`ApplyMigrations`.

### 4.3 The deploy choreography (C2 — carry-forward; step-2 plan order revised for C3)

Two binaries, in order:

1. **Inert-landing binary** (`Latest = Required = 19`; does **not** embed the 0020
   file). Ships: the `Deployer` engine + `deploy` verb, the ≥0044 `deploy_cursor`
   migration, the `STRIATUM_DEPLOY_DECOUPLED` decoupled boot path (flag OFF), the
   per-step + terminal `doctor schema_deploy_unrecorded` block, **and the
   forward-watermark rule**. Boot auto-applies legacy (`striatumd_rw` still has
   `CREATE`; 0044 is additive, runtime-owned — passes
   `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`). Decoupling inert. After this
   rollout every serving binary is forward-watermark-aware. **Daemon up, serving.**
2. **Activation binary** (embeds the 0020 file; `Latest = Required` still 19;
   `CheckDeployActivation` active). On boot with the deploy incomplete + flag OFF it
   halts `awaiting_deploy_config` (it never legacy-auto-applies, so no `42501` and no
   false fingerprint self-record). The operator then, **with the daemon parked**:
   - sets `STRIATUM_DEPLOY_DECOUPLED=1`;
   - runs **`striatum daemon deploy`** (no daemon serving): the deployer, over the owner
     connection, applies the ordered plan — **non-revoke owner bundles (≤19) → pending
     runtime steps (each reconciling ownership while CREATE is held, §3.3b) → bundle
     0020 as the terminal step** — writing one receipt per step (N1) + the finalizer's
     `complete` receipt + fingerprint + `complete` cursor (§3.4). Because the deployer
     uses the owner connection **and** 0020 is last, every `ALTER … OWNER TO
     striatumd_rw` succeeds and the steady state denies CREATE.
   - restarts the daemon → `ConnectAndVerify` sees `complete` + in-sync → serves,
     verify-only, zero serve-path DDL.

   **Bootstrap-order case:** if step 2 runs on a DB that never ran the inert binary
   (`deploy_cursor` absent), `CheckDeployActivation` reads the absent table as
   **incomplete → `awaiting_deploy`** (not error), and `striatum daemon deploy` creates
   `deploy_cursor` (applying ≥0044 over the owner connection, reconciling its ownership
   to `striatumd_rw` while CREATE is held) as an early step before recording the first
   cursor row.

   **Truly-pre-P4 binary (predates even the forward-watermark rule).** Protected by the
   choreography precondition: bundle 0020 is applied **only** by `striatum daemon
   deploy`, which **preflight-refuses to apply 0020 unless the runtime frontier already
   carries the ≥0044 `deploy_cursor` migration** (the inert-landing binary's marker) —
   i.e. unless a deployer-aware binary has booted this DB. Stated as F11 /
   `G-old-binary-refuse`.

### 4.4 How 0020 ships without lockout (the R3 sequencing) — REVISED for C3

The deployer applies all DDL (runtime steps **and** the terminal 0020) over the
**owner/admin connection** — the same mechanism `daemon migrate-db --admin-url`
(`daemon.go:161-210`) and `owner-ddl apply` already use — so the "runtime path" the
revoke must not break is **already owner-capable out-of-band**; 0020 removes no
capability the deploy path needs. **The C3 fix:** because 0020 is the **terminal** plan
step (after every runtime ownership reconcile) and is **excluded from `owner-ddl apply`
and the owner prefix** (§3.2), there is **no path** by which `REVOKE CREATE` commits
before a reconcile that needs CREATE. Combined with §4.2 (typed halts +
forward-watermark rule) and §3.3a (activation interlock fires before `ApplyMigrations`),
**no ordering — including every interleaving in `T-deploy-revoke-activation-ordering`
and `T-deploy-runtime-object-ownership` — drives boot into a runtime-runner
`ApplyMigrations` under a revoked `CREATE`, and no deploy strands itself at a reconcile
under a revoked `CREATE`.**

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> `T-` = unit/integration test (TDD target for the build run); `G-` = game-day against
> a real two-role cluster. F1–F11 carry forward (re-confirmed); **F12 is REVISED for
> C3 resolution (a)** and **F13 is NEW for N1** — these two are what the cycle-3
> falsifiers re-attack first.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1 (Resumability — Q3-A)** | Kill `deploy` after a transactional step *k* commits; re-run resumes at *k+1*, never re-runs *k* or half-applies. | `T-deploy-resume-transactional`; `G-resume-kill-9`. |
| **F2 (Resumability — Q3-B)** | Kill `deploy` during a `CREATE INDEX CONCURRENTLY`; re-run's reconciler drops the `INVALID` index and rebuilds; final state is one VALID index, no double-apply. | `T-deploy-resume-concurrently`; `G-cic-kill`. |
| **F3 (No serve-boot mutation)** | With `STRIATUM_DEPLOY_DECOUPLED=ON`, a boot with a pending plan does NOT apply it; refuses-to-serve `awaiting_deploy` via the non-restartable exit. | `T-boot-decoupled-no-apply`; `G-pending-restart`. |
| **F4 (Fingerprint coherence — §1.3)** | An interrupted deploy is classified "incomplete, resume" / "resumable finalization", never "unknown drift". | `T-classify-incomplete-vs-drift`: matrix over `{cursor state} × {fingerprint match}` asserting the §1.3 table exactly; mutate `plan_hash` to foreign → flips to genuine-drift. |
| **F5 (No regression of P2/P3 — R2)** | Lifting `ApplyMigrations` keeps the watermark interlock + Layer 3 self-record intact; fresh-DB bring-up still works via the verb. | `T-decoupled-preserves-watermark`; `T-fresh-db-via-deploy`; existing `migrations_test.go` two-role guard stays green. |
| **F6 (DDL revocation — R3, structural)** | After the deploy's terminal bundle 0020, `striatumd_rw` cannot `CREATE` on schema `striatumd`; the deployer (owner connection) still applies runtime steps; no lockout across restart. | `T-revoke-blocks-rw-create`; `T-deploy-after-revoke`; `G-revoke-then-restart`. |
| **F7 (Receipt provenance — R4, per-step; REVISED for N1)** | Every applied step writes **exactly one** hash-chained deploy receipt keyed on `(plan_hash, step_index)`; a committed step without one is surfaced by the tightened `doctor schema_deploy_unrecorded` (not masked by a present terminal receipt). | `T-receipt-per-step`; `T-doctor-unrecorded-per-step` (apply a step, delete its per-step receipt, assert doctor WARNs even with the terminal `complete` receipt present). |
| **F8 (Cursor has no double-apply/skip hole — R4; extended for the receipt boundary)** | No interleaving of crash points across a step boundary yields a skipped/twice-applied step **or** a missing/duplicated per-step receipt. | `T-deploy-fault-injection`: parametrized crash at each of {before marker, after marker/before step, after step/before receipt+commit-marker, after receipt+commit-marker} for a T-DDL and an NT-DDL step; assert one apply and exactly one receipt each. |
| **F9 (Plan ≡ fingerprint binding; 0020-terminal)** | A `complete` deploy's recorded fingerprint equals `ExpectedFingerprint()`, and `plan_hash` is over the ordered transcript **with 0020 last** (re-ordering changes `plan_hash`, not the fingerprint). | `T-plan-hash-binds-fingerprint`; `T-plan-orders-revoke-last`. |
| **F10 (Finalization boundary closed — C1; carry-forward)** | No crash during finalization leaves a `complete` cursor with a missing receipt/fingerprint; every interrupted finalization is pinned at `finalizing`, classified **resumable finalization**, and repaired exactly once. | **`T-deploy-resume-finalization-crash`**: kill after each of {final DDL commit; `finalizing`-before-receipt; receipt-before-`schema_state`; `schema_state`-before-`complete`}; assert (a) no boot serves until the cursor reads `complete`, (b) exactly one `complete` receipt after repair, (c) `LiveFingerprint == ExpectedFingerprint` and doctor green after repair. `G-finalize-kill`. |
| **F11 (Activation fail-closed — C2; carry-forward)** | With the revoke active (deploy `complete`) or pending, boot **never** calls `ApplyMigrations` over the runtime runner; it halts `awaiting_deploy`/`awaiting_deploy_config` DB-untouched with a remediation naming `striatum daemon deploy` / the flag. A revoke-unaware binary observing `applied_owner >= 20` halts via the forward-watermark rule, not a `42501` crash-loop. | **`T-deploy-revoke-activation-ordering`**, parametrized over: (a) revoke-embedding binary, flag OFF → `awaiting_deploy_config`; (b) flag ON, `deploy_cursor` absent/incomplete → `awaiting_deploy`; (c) old binary (no 0020 file) + `applied_owner=20` + pending runtime migration → `awaiting_deploy` via forward-watermark, **`ApplyMigrations` NOT called**; (d) revoke-embedding binary + flag OFF + pending runtime migration → `awaiting_deploy_config`, **`ApplyMigrations` NOT called**. Assert via a spy that `applyOne` is never entered and the schema is byte-identical. `G-old-binary-refuse`: assert `daemon deploy` preflight-refuses 0020 when the ≥0044 marker is absent. |
| **F12 (Runtime-object ownership under the 0020-terminal plan — C3, REVISED)** | A runtime migration applied through the deployer's owner connection yields objects **owned by `striatumd_rw`** with the serving role's real DML intact, **because the ownership reconcile runs while `striatumd_rw` still holds CREATE (0020 not yet applied)**; after the deploy's terminal 0020 the serving role cannot create objects. The v2 internal inconsistency is gone: both facts hold at different plan times. | **`T-deploy-runtime-object-ownership`** (two-role pgtest, **documented non-superuser** cluster): build a plan = {a new runtime migration creating a table + index + sequence} + {terminal bundle 0020}; assert during the run `has_schema_privilege('striatumd_rw','striatumd','CREATE')` is **true** at the reconcile and the reconcile step's `step_index` is **strictly less** than 0020's; after the deploy assert `pg_class.relowner`/`pg_get_userbyid` == `striatumd_rw` for **every** created object (table, index, sequence); assert `SET ROLE striatumd_rw` can `SELECT/INSERT/UPDATE/DELETE` the new table; assert `SET ROLE striatumd_rw; CREATE TABLE …` → **`42501`** after the terminal 0020 commits; assert `has_schema_privilege('striatumd_rw','striatumd','CREATE')` is **false** post-deploy. Re-run the deployer → idempotent no-op. `G-revoke-last`: on a real two-role cluster, kill `deploy` *between* the last runtime reconcile and 0020, re-run, assert it completes (CREATE still held on resume) and the steady state denies CREATE. |
| **F13 (Per-step receipt crash-safe — N1, NEW)** | A crash between a step's commit and its receipt-write is reconciled to **exactly one** receipt per applied step on re-run; for a transactional step `step_committed(k)` is durable iff its receipt is; for an NT-DDL step the keyed `(plan_hash, step_index)` reconcile appends exactly one receipt before `step_committed(k)`, resolving duplicate-vs-skip; the resume never advances past a step lacking its receipt. | **`T-deploy-receipt-crash-resume`**: kill at each crash point — (T-DDL) after DDL+cursor+receipt commit (atomic, single point); (NT-DDL) after the side effect before the reconcile, after the receipt append before `step_committed`, after `step_committed` — and assert after re-run: final schema equality, **exactly one** `step_committed` receipt per applied `(plan_hash, step_index)`, one terminal `complete` receipt, and a **green** doctor. `G-receipt-kill`: `kill -9` between an NT-DDL side effect and its reconcile on a real cluster; `deploy` again; assert single per-step receipt. |

---

## 6. Shadow-first rollout (the P4 build order) — REVISED

1. **Migration ≥ 0044 `deploy_cursor`** (additive runtime table, modeled on
   `0043_schema_state.sql`; the `state` CHECK includes `finalizing`; `step_index`
   column present; no FK, runtime-owned — passes
   `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`).
2. **`go/pkg/db/deploy.go`**: `DeployPlan`, `BuildPlan` (**with the 0020-terminal
   special-case + 0020 excluded from `owner-ddl apply`**, §3.2), `Deployer.Apply`
   (Q3-A/Q3-B engine + the `finalizing` finalizer §3.4, advisory-locked via
   `MigrationLockKey`), `applyRuntimeStep` (the **C3 ownership reconcile while CREATE is
   held**, §3.3b), the **per-step + terminal receipt writer keyed on `(plan_hash,
   step_index)`** (N1). Pure-core + DB-integration tests (F1, F2, F4, F8, F9, F10,
   **F12**, **F13**) — engine + finalization + ownership + per-step receipt proven
   **before** any boot path changes.
3. **`runDaemonDeploy`** verb (`daemon.go`) + matrix/authority-guardrail row +
   `--dry-run` + the **0020-activation preflight** (§4.3). F3/F5 wiring.
4. **`CheckDeployActivation`** + the typed `awaiting_deploy`/`awaiting_deploy_config`
   halts + the **forward-watermark rule** in `CheckOwnerBundleWatermark` + the `main.go`
   `errors.As` arms (§3.3a, §4.2). Decoupled boot path (`ConnectAndVerify`) behind
   `STRIATUM_DEPLOY_DECOUPLED` (default OFF). Lands inert (**F11**, F3, F5).
5. **`doctor schema_deploy_unrecorded`** block — **per-step** tightened (F7).
6. **Owner bundle 0020** (DDL revoke) — authored, **deploy-plan-terminal and excluded
   from `owner-ddl apply`** (`LatestOwnerBundleVersion` stays 19), tested in two-role
   pgtest (F6, F12); activation is the operator choreography (§4.3).

Each phase additive and reversible until the operator flips the flag + the deploy
applies 0020 last. Self-record before enforce; detection before mutation-relocation;
the verb + finalizer + ownership reconcile + per-step receipt proven before the boot
path leans on them.

---

## 7. Boundary & explicit deferrals (carry-forward)

- **In P4:** the deployer verb, the deploy plan + `deploy_cursor` (incl. `finalizing`)
  + per-step + terminal deploy receipt (N1), the idempotent finalizer (C1), the C3
  ownership reconcile **with the revoke sequenced last**, lifting `ApplyMigrations` out
  of serve-boot (flagged), the activation interlock + forward-watermark rule,
  serving-role create-DDL revocation (bundle 0020, deploy-terminal).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`,
  `rehearsal_receipt.v1`, fidelity tiering, full-data clone, expand/contract reshape
  primitive, lock-budget guardrail. P4 names the seams (plan_hash artifact, per-step
  receipt chain, `Deployer.Apply` engine) P5 builds on but ships none of it. Full
  capability revocation (owner re-owns runtime tables) is a named follow-up beyond P4
  (§4.1), not assumed.
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the single
  writer, no hosted services / replicas / external persistence.

---

## 8. Open tensions handed to the falsifiers (attack these hardest)

- **C3 / F12 — the revoke-last ordering.** Is there *any* path by which 0020's `REVOKE
  CREATE` commits before a runtime ownership reconcile that needs CREATE? Probe: a
  mixed plan where an *owner* bundle in the prefix (≤19) is somehow the object-creator;
  a future bundle `> 20` that BuildPlan might sort between the runtime steps and 0020; a
  `daemon owner-ddl apply` invoked on the activation binary (must NOT apply 0020 — is
  the `<= LatestOwnerBundleVersion` bound actually enforced, and is `revokeEmbedded`
  decoupled from `LatestOwnerBundleVersion`?). Does the deployer's CREATE-precondition
  assert (§3.3b step 1) fire correctly if 0018's grant was hand-reverted?
- **C3 / F12 — the catalog-diff under 0020-last.** Does the before/after oid diff miss
  any object kind a runtime migration could create (a TYPE, a trigger function, an owned
  sequence created implicitly by `GENERATED … AS IDENTITY`)? Can the diff ever reassign
  an owner-bundle object created in the same plan (an owner step preceding a runtime
  step)? (It is scoped to new oids on *runtime* steps only — verify.)
- **N1 / F13 — the per-step receipt key.** Is `(plan_hash, step_index)` truly stable and
  unique across re-runs and across two concurrent deployers under `MigrationLockKey`?
  Can the Q3-B reconcile ever append two receipts (race between the existence check and
  the append within one owner-connection tx)? Does the tightened doctor correctly
  enumerate "applied steps" for a partially-applied plan (does it over- or under-report
  when the live frontier straddles a step)?
- **C1 ⊕ N1 coherence.** Does adding the per-step receipt to the step transaction
  perturb the finalizer's two-connection idempotence or the §1.3 classification? Is
  there a boot that reads `finalizing` + matching fingerprint and wrongly serves?
- **C2 / F11 (carry-forward) — the old-binary preflight.** Is the `daemon deploy`
  0020-preflight (refuse unless the ≥0044 marker is present) a sufficient close against
  a truly-pre-P4 binary still serving across a restart, now that 0020 reaches the
  watermark only at deploy end?
- **§4.1 — the residual capability.** Revoking only `CREATE ON SCHEMA` leaves
  `striatumd_rw` able to `ALTER` its own tables. Is "the serve *path* issues no DDL" a
  sufficient P4 close of failure mode 1, or can a falsifier construct a serve-path DDL
  (a DML trigger / `DO` block that issues DDL)?
- **Q4 — bootstrapping.** Any path where plain-verb-now forecloses the run-shape later
  that the three seams (§2) don't keep open?

---

### Anchor table (re-verified against the worktree base of `striatum/rfc-0142-p4-design-v3`)

| Target | Anchor |
| --- | --- |
| serve-boot mutation site (lift this) | `go/pkg/db/connection.go:332` `ConnectAndMigrate`; apply at `:353`; watermark `:349-352`; drift gate `:376-383`; self-record `:399-402` |
| boot fork / `--migrate` flag / typed-halt mapping | `go/pkg/db/authority_bootstrap.go:181-201`; `go/cmd/striatumd/main.go:76,192-198`; typed halts `:200-228` (C2 adds `awaiting_deploy`/`awaiting_deploy_config` arms here) |
| deployer command site | `go/pkg/cli/localcommands/daemon.go:67-81` dispatch; pattern `runDaemonOwnerDDL` `:90-159`, `runDaemonMigrate` `:167-210` |
| runtime apply engine to reuse / wrap (C3) | `go/pkg/db/migrations.go:121-178` `ApplyMigrations`; `applyOne` `:304-355` (BeginTx → SQL → version stamps → in-tx hash verify → Commit; **no `SET ROLE`/`ALTER OWNER`/grant** — the C3 gap the deployer's `applyRuntimeStep` wraps); lock `:18,:122-134`; preflight `:158`; frontier `:17` (=43, new ≥0044) |
| owner-bundle engine / per-tx atomicity / watermark + forward-tolerance (C2) + 0020 exclusion (C3) | `go/pkg/db/owner.go:264-302` `ApplyOwnerBundles` (**bound to `<= LatestOwnerBundleVersion` so it skips 0020**); `:498-528` `applyOneOwnerBundle` (one tx, stamp last — the terminal-0020 step + its receipt reuse this); `CheckOwnerBundleWatermark` `:124-154` (forward-tolerance `:148-153` — the C2 rule); frontier `:23` (`LatestOwnerBundleVersion`=19, **stays 19**; new constant `DDLRevokeOwnerBundleVersion=20`); `RequiredOwnerBundleVersion` `:35` (stays 19) |
| C3 prerequisite the revoke-last fix relies on | `go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:64-75` (prereq #2: `striatumd_rw` MUST hold CREATE on schema striatumd), `:98` (`GRANT CREATE ON SCHEMA striatumd TO striatumd_rw` before the `:99-104` transfer loop); `0019_supervisor_pointer_runtime_ownership.sql:53-80` (same prereq + grant) — 0018 is in the non-revoke owner prefix, so CREATE is held when the runtime reconciles run |
| fingerprint contract (bind plan_hash to) | `go/pkg/db/schema_drift.go:83-100` `ExpectedFingerprint`; `:105-137` `composeFingerprint` (reused for `plan_hash`); `:145-161` `LiveFingerprint`; `:171-195` `RecordSchemaFingerprint` (C1 finalizer step 2); `:254-274` `CheckSchemaDrift` |
| `deploy_cursor` table model (incl. `finalizing` state + `step_index`) | `go/pkg/db/sql/0043_schema_state.sql:39-52` (singleton CHECK + `striatumd_rw` GRANT block) |
| per-step / terminal receipt SD fn (N1) | `go/pkg/db/sql/owner/0001_authority_phase0.sql:223,238` (`append_audit_row` — owner-owned SD fn, runtime EXECUTE-only, direct `audit_log` INSERT revoked `:237`; runs in the caller's transaction, so the per-step receipt joins the owner-connection step tx) |
| doctor block model (per-step tightened) | `go/pkg/reads/doctor_schema_drift.go:26-77` |

<sub>Holder artifact (v3 revision) for the RFC 0142 P4 falsification-gate design run.
Resolves cycle-2 findings C3 (resolution (a): sequence the bundle-0020 revoke last,
excluded from `owner-ddl apply`) and N1 (per-step receipt atomic/idempotent keyed on
`(plan_hash, step_index)`), and carries C1 + C2 forward verbatim (§0.1). The cycle-3
falsifiers (atomicity/resumability lens; decoupling/migration-safety lens) re-attack §5
(esp. F12 + F13) and the tensions in §8; the adjudicator's collaboration ledger — not
falsifier completion — decides whether the gate clears.</sub>

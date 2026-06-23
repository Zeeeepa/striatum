# HOLDER — RFC 0142 P4: the one-shot `striatum daemon deploy` (decouple schema-apply from serve-boot)

author: holder-author-001

> **This is the leading claim the falsifiers attack.** It is a falsifiable
> implementation spec for RFC 0142 **P4 only** — the one-shot deployer that lifts
> schema mutation out of serve-boot — that the `rfc-0142-p4-build` run can execute
> contract-first (TDD). RFC 0142 is **accepted (D258)**; this does NOT re-open the
> five-layer design. Primary source:
> `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (esp. Layer 3,
> the Phasing P4 row, and Open Questions 3 + 4). Anchors were re-verified against
> the worktree base of `striatum/rfc-0142-p4-design` (cut from `main`); every
> file:line below is current source.

## 0. Root reframe held

Schema mutation must stop being an **implicit side effect of the serving process's
restart** and become an **explicit, ordered, resumable, provenance-tracked
operation owned by a dedicated deployer**. Then the serving daemon can hold
**zero DDL on the serving path**, "restart force-commits a half-applied deploy"
becomes structurally impossible, and a bad migration can never wedge the single
writer on boot. Everything below is in service of that one move; nothing here
pulls in P5 (rehearse / expand-contract / fidelity tiering / clone = Q1/Q2).

---

## 1. Decision Q3 — How atomic is "atomic"? (the hard correctness core)

**Claim (Q3-DECISION).** The **per-step-atomic + resumable-cursor** contract is
sufficient for *every* owner+runtime interleaving P4 ships, **provided** two
sub-rules hold, and **no step needs a cross-step transaction** (which is
impossible across two connections / non-transactional DDL anyway):

- **Q3-A — strict sub-protocol where the step is transactional.** If a step's DDL
  can run inside a transaction, the **cursor advance commits in the SAME
  transaction as the step**. Crash → the whole transaction rolls back → cursor
  unchanged → resume re-enters the step cleanly. This is the stricter
  single-connection/single-transaction sub-protocol, and it is **mandatory for
  every transactional step** (it is strictly safer than a separate cursor write,
  so we always take it when available).
- **Q3-B — idempotent-resume reconciler where the step is NOT transactional.**
  `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, and the handful of
  auto-committing `ALTER`s cannot share a transaction with the cursor. For these
  the deployer writes an **`in_progress` cursor marker BEFORE the step**, runs the
  step in its **idempotent / self-reconciling form**, then writes the
  **`step_committed` marker AFTER**. The crash window is *between* the step's own
  commit and the marker commit; on resume the cursor still reads `in_progress(N)`,
  so the deployer re-enters step N through its reconciler, which is a no-op (or a
  repair) by construction.

### 1.1 Step taxonomy (the closed set P4 ships)

| Class | Examples | Transactional? | Protocol | Idempotent form |
| --- | --- | --- | --- | --- |
| **T-DDL** runtime | `CREATE TABLE`, additive `ALTER … ADD COLUMN`, `CREATE INDEX` (non-concurrent), the new runtime migrations themselves | Yes | **Q3-A** (step + cursor in one tx) | `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`; re-run is a no-op |
| **T-DDL** owner | owner-bundle DDL (authority/grant/CHECK changes) | Yes (each bundle already applies in one tx that stamps `owner_bundle_meta` last — `owner.go:454-484`) | **Q3-A**, reusing the existing per-bundle transaction; cursor advance joins that tx | bundles are already idempotent (`IF NOT EXISTS` + `ON CONFLICT (version) DO NOTHING`) |
| **NT-DDL** | `CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD VALUE`, `DROP INDEX CONCURRENTLY` | No | **Q3-B** (pre-marker → step → post-marker) | `CREATE INDEX CONCURRENTLY IF NOT EXISTS`; **resume reconciler** drops a left-behind `INVALID` index then recreates; `ADD VALUE … IF NOT EXISTS` (PG12+) |
| **DML-backfill** (bounded use in P4; full online backfill is P5) | a one-shot data normalize a runtime migration carries | Yes if small; batched otherwise | Q3-A small / Q3-B batched | `WHERE <not-yet-done>` predicate; re-run resumes from the unprocessed tail |

> The **only genuinely hard corner is NT-DDL**, and it reduces to one rule the
> falsifiers must press: **a half-built `CREATE INDEX CONCURRENTLY` leaves a
> catalog row in `pg_index.indisvalid = false`.** The fingerprint MUST classify
> that as **"incomplete, resume"**, never "unknown drift, panic" — see §1.3.

### 1.2 `deploy_cursor` states and crash-resume semantics

`deploy_cursor` is a **new runtime-owned singleton table** (runtime migration
**≥ 0044**, modeled byte-for-byte on `0043_schema_state.sql:39-52`: a `CHECK (id =
'singleton')` PK + the `striatumd_rw` GRANT `DO` block). Columns:

```
deploy_cursor(
  id            text PK CHECK (id='singleton'),
  plan_hash     text NOT NULL,         -- the plan this cursor is advancing
  state         text NOT NULL,         -- enum below (CHECK constraint)
  step_index    int  NOT NULL,         -- 0-based index into the ordered plan
  step_id       text NOT NULL,         -- e.g. "runtime:0044" / "owner:0020"
  updated_at    timestamptz NOT NULL DEFAULT now()
)
```

`state ∈ { idle, in_progress, step_committed, complete, aborted }`. Lifecycle for
a plan of N steps:

```
idle ─▶ in_progress(0) ─▶ step_committed(0) ─▶ in_progress(1) ─▶ … ─▶ step_committed(N-1) ─▶ complete
                  │                                                          
                  └─(operator abort / fatal non-resumable error)──▶ aborted   
```

- **Q3-A step:** the transition `in_progress(k) → step_committed(k)` and the step
  DDL commit **are one transaction**. There is no observable `in_progress(k)`
  after a crash on a transactional step — it either never started or fully
  committed both. Resume reads `step_committed(k)` and advances to `k+1`.
- **Q3-B step:** transitions are three separate commits: `step_committed(k-1) →
  in_progress(k)` (marker), then the step's own auto-commit, then `in_progress(k)
  → step_committed(k)`. **Crash resume reads `in_progress(k)` and re-enters step k
  through its reconciler.** The reconciler is the source of truth for "did the
  side effect land": e.g. for `CREATE INDEX CONCURRENTLY` it checks
  `pg_index.indisvalid`; INVALID → `DROP INDEX CONCURRENTLY IF EXISTS` then
  recreate; valid → skip; absent → create.
- **`complete`** is written only when `step_index == N-1` is `step_committed`; the
  deployer then writes the deploy receipt (§3.4) and calls
  `RecordSchemaFingerprint` (`schema_drift.go:171`) so `schema_state` and
  `deploy_cursor` agree.

### 1.3 The load-bearing disambiguation (why this clears the gate)

Today the drift gate has **one** signal: `LiveFingerprint == ExpectedFingerprint`?
(binary: match / **drift**, `schema_drift.go:254-274`). That is exactly the
"unknown drift, panic" failure the SEED warns against — an interrupted apply looks
identical to a corrupted one. **P4's `deploy_cursor` is the second signal that
disambiguates them:**

| `deploy_cursor.state` | fingerprint | classification | action |
| --- | --- | --- | --- |
| `complete`, `plan_hash` == expected | match | **in sync** | serve |
| `in_progress` / `step_committed`, `plan_hash` == the binary's plan | mismatch | **incomplete, resume** | `striatum daemon deploy` resumes at the cursor |
| absent / `complete` but `plan_hash` ≠ expected, OR `aborted` | mismatch | **genuine drift** | refuse-to-serve (the existing `schema_drift` halt) |

This table is the single most important falsifiable artifact in the spec. **R1 /
R4 (the SEED's load-bearing risks) live entirely in this table's middle row.**

---

## 2. Decision Q4 — Is a deploy itself a Striatum run?

**Claim (Q4-DECISION). Plain verb now (`striatum daemon deploy`); the run-shape
door is held open by three already-present seams, not by speculative scaffolding.**

**Why not a run now — the bootstrapping paradox is dispositive.** A dogfooded run
needs the daemon up and its schema present to drive the runner's own tables
(`runs`, `jobs`, `sessions`, the capability ledger). But the deploy is *precisely
the operation that brings a fresh or owner-only database to a serveable schema*
(it is the successor to today's boot-time `ApplyMigrations` and to
`daemon migrate-db`, `daemon.go:167-210`). A run-shaped deployer **cannot bootstrap
the database it would itself run on**, and would be unavailable in exactly the
incident this RFC closes (the single writer wedged, unable to serve). So the base
deployer is a **plain verb that runs against a bare / owner-only DB with no
running daemon**, the same execution context `daemon owner-ddl apply`
(`daemon.go:90-159`) and `daemon migrate-db` already occupy.

**What keeps the run-shape door open (so P5 / the RFC's Provocation needs no
re-litigation):**

1. **The plan is a content-addressed artifact** (`plan_hash`, §3.2) — the exact
   inter-stage hand-off unit an `expand_rehearsal → contract_swap` run would pass
   between stages.
2. **The deploy receipt is hash-chained into the owner-held `audit_log`** (§3.4) —
   already the adjudicated-provenance unit a run emits.
3. **The verb is a thin CLI over a `Deployer` package** (`go/pkg/db/deploy.go`,
   new) with a pure `Apply(ctx, plan, cursor)` core. A future run step invokes the
   same `Deployer.Apply`; the verb and the run share one engine. No engine rewrite
   is needed to promote it.

**Falsifiable boundary:** if a falsifier shows a real fresh-DB bring-up that the
plain verb cannot perform but a run could (it cannot — a run is strictly more
dependency-laden), Q4 flips. Otherwise plain-verb-now stands.

---

## 3. The deployer surface + serve-boot decoupling (shadow-first, by exact code site)

### 3.1 Command site

Add `case "deploy":` to the dispatch in `RunDaemon`
(`go/pkg/cli/localcommands/daemon.go:67-81`, alongside
`install|uninstall|status|migrate-db|owner-ddl`) → new `runDaemonDeploy(...)`.
DSN resolution mirrors `runDaemonOwnerDDL` exactly (`daemon.go:95-121`):
`--owner-url` / `--admin-url`, then `STRIATUM_DAEMON_ADMIN_DB_URL`
(`EnvDaemonAdminDBURL`, `daemon.go:24`), then the daemon DSN. Flags: `--plan`
(default: the embedded plan), `--dry-run` (print the ordered plan + cursor delta,
mutate nothing), `--resume` (default behavior; explicit for clarity), `--json`.
The command-authority matrix (`docs/reference/command-authority-matrix.md`) and the
authority guardrail tests gain the `daemon.deploy` row (AGENTS.md change
discipline).

### 3.2 The deploy plan — derived + embedded, NOT a new hand-authored manifest

**Claim (PLAN-DERIVED).** The plan is **generated deterministically from the same
two embedded `embed.FS` sources the fingerprint already hashes** — the runtime
migration FS (`migrations.go:21` `//go:embed sql/*.sql`) and the owner-bundle FS
(`owner.go:112` `//go:embed sql/owner/*.sql`) — **not** a separately
hand-maintained TOML/JSON that could drift from the SQL. (Layer 0's reservation
ledger already governs ordinal allocation; a second hand-authored manifest would
reintroduce drift, the very class P3 closed.)

`BuildPlan(applied_owner, applied_runtime) → DeployPlan`:

- **Steps** = the pending owner bundles (ascending `Version`, tagged `owner`,
  `owner.go:151-179`) **then** the pending runtime migrations (ascending
  `Version`, tagged `runtime`, `migrations.go:31-105`). This reproduces today's
  semantics exactly: the watermark interlock already requires owner ≥ N *before*
  any runtime apply (`connection.go:349-352`, `owner.go:91-110`). The plan
  promotes that implicit ordering into **explicit dependency edges** (every
  `runtime` step depends on the full `owner` prefix) so a future fine-grained
  interleave is expressible without changing the engine.
- Each step carries `{role, step_id, sha256, transactional: bool}`. The
  `transactional` bit is the Q3-A/Q3-B selector; for P4 it is a per-step
  annotation in the migration/bundle file header (a `-- striatum:nontransactional`
  pragma) defaulting to transactional, validated by a build-time test that greps
  the SQL for the known non-transactional verbs (`CONCURRENTLY`, `ALTER TYPE …
  ADD VALUE`) and asserts the pragma matches — so a mis-annotated step reds the
  build, not prod.
- **`plan_hash`** = sha256 of the canonical step transcript, reusing the
  `composeFingerprint` canonicalization style (`schema_drift.go:105-137`). It is
  bound to `ExpectedFingerprint` by construction: **a `complete` plan's recorded
  fingerprint == `ExpectedFingerprint()`**, so the deployer and the drift gate
  never disagree about "is the whole plan applied."

### 3.3 Lifting `ApplyMigrations` out of serve-boot

**Claim (DECOUPLE).** Serve-boot stops mutating schema, behind a flag, shadow-first.

The single mutation site on the serving path is `ConnectAndMigrate`
(`connection.go:332-404`), reached from boot via `BootstrapAndConnect(…, migrate)`
(`authority_bootstrap.go:181-201`, the `if migrate { ConnectAndMigrate } else {
Connect }` fork), driven by the `--migrate` flag (default `true`,
`main.go:51,76`). P4 inverts the default **behind a new gate**, not in one
cut:

1. **New env `STRIATUM_DEPLOY_DECOUPLED`** (default OFF — shadow-first, the
   codebase convention, cf. `EnvSchemaDriftRefuse`, `schema_drift.go:15-28`). When
   **OFF**, boot behaves exactly as today (auto-apply stays the default) — so the
   decoupling lands to `main` **inert** and a false assumption cannot cause an
   outage on the next restart.
2. When **ON**, `ConnectAndMigrate` becomes **`ConnectAndVerify`**: it keeps the
   Layer 2 watermark interlock (`connection.go:349-352`) and the Layer 3 drift gate
   (`connection.go:376-383`) but **does NOT call `ApplyMigrations`
   (`connection.go:353`)**. Instead it runs the §1.3 classification:
   - in sync → serve;
   - incomplete-resume → refuse-to-serve with a **new typed
     `awaiting_deploy`** halt (sibling of `AwaitingOwnerDDLError`,
     `owner.go:44-64`), message = "a deploy is in progress / pending; run
     `striatum daemon deploy`", reusing the existing non-restartable exit path
     (`main.go:200-228`, `exitAwaitingOwnerDDL`);
   - genuine drift → the existing `schema_drift` refuse path, unchanged.
3. **No window where the daemon serves on an unmigrated schema (R2).** Because the
   decoupled boot path *verifies before serving* and refuses on any non-in-sync
   state, there is never a moment where it both skips apply *and* serves on a stale
   schema. The watermark interlock and drift self-record (`connection.go:394-402`)
   are preserved verbatim; the only thing removed under the flag is the
   `ApplyMigrations` call.

The deployer (`Deployer.Apply`) becomes the **only** caller of `ApplyMigrations`'
DDL (it reuses the per-migration apply + advisory lock machinery,
`migrations.go:121-178`, `MigrationLockKey` `migrations.go:18`, so two deployers
can't race) — and it runs runtime DDL over the **owner/admin connection** (see
§4), which is how `daemon migrate-db --admin-url` already applies runtime
migrations as the owner today (`daemon.go:161-210`).

### 3.4 The deploy receipt

**Claim (RECEIPT).** Every applied step writes a **hash-chained deploy receipt into
the owner-held `audit_log`**, so each schema change is first-class adjudicated
provenance — and `doctor` gains **`schema_deploy_unrecorded`**.

- The receipt row is appended via the existing owner-only SD function
  `append_audit_row(...)` (`owner/0001_authority_phase0.sql:223,238`; runtime
  direct `INSERT` on `audit_log` is revoked at `:237`) — so the receipt is written
  by the **owner connection**, hash-chained into the same tamper-evident chain as
  every other audit row. Payload: `{plan_hash, step_id, role, sha256, state,
  prev_receipt_hash}`.
- **`doctor` block `schema_deploy_unrecorded`** (new, modeled on
  `schemaDriftDoctorBlock`, `go/pkg/reads/doctor_schema_drift.go:26-77`): WARNING
  (never hard-red, per the shadow-first doctor convention) when
  `schema_state.fingerprint` advanced but the matching `complete` deploy receipt
  is absent in `audit_log` — i.e. a schema change reached the DB without a recorded
  deploy. This is the post-apply oracle that a hand-applied migration (bypassing
  the deployer) is **visible**, not silent.

---

## 4. Serving-role DDL revocation (owner bundle ≥ 0020), without lockout

**Claim (DDL-REVOKE).** The serving role `striatumd_rw` loses its remaining
**object-creation** DDL on the serving path, shipped as **owner bundle ≥ 0020**,
**activated only after the deployer is the active mutator** — so failure mode 1
(a serve-path DDL touching an owner-held table) becomes structurally impossible,
without recreating a #512-class lockout.

### 4.1 What is revoked, precisely

The anchor table confirms `striatumd_rw` already holds **no DDL on owner-owned
tables** (`owner/0001_authority_phase0.sql:229-238`; preflight + build guard route
owner-table DDL to bundles, `migrations.go:158`, `owner.go:133-134`). The residual
serve-path DDL surface is **object creation on the runtime schema**: the serving
role can `CREATE TABLE` / `CREATE INDEX` because it applies runtime migrations on
boot. P4 revokes exactly that:

```
-- owner bundle 0020 (DDL-revoke): the serving path holds zero create-DDL.
REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw;
REVOKE TEMPORARY ON DATABASE <db> FROM striatumd_rw;  -- if granted
```

`striatumd_rw` **retains** ownership of (and therefore `ALTER`/`DROP` on) the
runtime tables it already owns — reassigning that ownership to the owner role is
the regressive move bundles 0018/0019 explicitly *undid* (`owner.go:133-134`) and
is **out of P4 scope** (it would re-open #442). The honest residual: the serving
role could still `ALTER` its *own* runtime tables — but **the serving daemon never
issues DDL once `ApplyMigrations` is lifted (§3.3)**, so the serve *path* is
zero-DDL even though one latent *capability* remains. Full capability revocation
(owner re-owns runtime tables + deployer runs all DDL as owner) is named here as a
**follow-up beyond P4**, not silently assumed.

### 4.2 How it ships without lockout (the R3 sequencing)

The deployer must apply runtime DDL, but after 0020 the serving role can't. **The
deployer runs runtime steps over the owner/admin connection** — the *same*
mechanism `daemon migrate-db --admin-url` (`daemon.go:161-210`) and `owner-ddl
apply` already use to apply DDL the runtime role can't. So the "runtime path" the
revoke must not break is **already owner-capable out-of-band**; 0020 does not
remove a capability the deploy path needs.

**Strict ordering (falsifiable):**

1. P4 lands the deployer + the decoupled boot path **behind `STRIATUM_DEPLOY_-
   DECOUPLED` OFF** (auto-apply still the default). Bundle 0020 is **NOT applied
   yet**.
2. Operator verifies `striatum daemon deploy` brings the DB to frontier on their
   real two-role cluster (the P0 two-role pgtest fixture is the CI oracle).
3. Operator applies bundle 0020 (`daemon owner-ddl apply`) **and** flips
   `STRIATUM_DEPLOY_DECOUPLED=ON` **together** — after this, boot verifies-only and
   the serving role can't create objects.
4. The watermark interlock (`owner.go:35` `RequiredOwnerBundleVersion =
   LatestOwnerBundleVersion`) advancing to 20 means a binary built against 0020
   that boots a DB still at 0019 **halts cleanly** (`awaiting_owner_ddl`,
   `main.go:208-214`) — never crash-loops. The revoke can only take effect *after*
   the deployer exists, because the same binary that ships bundle 0020 ships the
   `deploy` verb.

If 0020 were applied while a binary *without* the deployer were serving (a botched
order), boot would refuse cleanly (drift / awaiting), not lock out — but the
ordering above makes even that transient impossible.

---

## 5. Falsifiable assertions (each paired with the test / game-day that refutes it)

> These are the claims the falsifiers attack. Each is stated so that a single
> named experiment can prove it false. `T-` = unit/integration test (TDD target
> for the build run); `G-` = game-day against a real two-role cluster.

| # | Assertion | Refuting experiment |
| --- | --- | --- |
| **F1 (Resumability — Q3-A)** | Kill `striatum daemon deploy` after a **transactional** step *k* commits; re-run resumes at *k+1* and never re-runs *k* or half-applies. | `T-deploy-resume-transactional`: inject a panic between the step tx commit and the next step; assert cursor == `step_committed(k)`, re-run applies only `k+1…`. `G-resume-kill-9`: `kill -9` mid-plan on a real cluster, `deploy` again, diff schema == full plan. |
| **F2 (Resumability — Q3-B, the hard corner)** | Kill `deploy` *during* a `CREATE INDEX CONCURRENTLY` step; re-run's reconciler drops the `INVALID` index and rebuilds; final state is one VALID index, no double-apply. | `T-deploy-resume-concurrently`: kill after the marker `in_progress(k)` but before `step_committed(k)`, leaving `pg_index.indisvalid=false`; assert reconciler path runs and ends VALID. `G-cic-kill`: same on a prod-shaped index. |
| **F3 (No serve-boot mutation)** | With `STRIATUM_DEPLOY_DECOUPLED=ON`, a daemon boot with a pending plan does **not** apply it; it refuses-to-serve (`awaiting_deploy`) via the non-restartable exit. | `T-boot-decoupled-no-apply`: pending migration + flag ON → assert `ApplyMigrations` not called (schema unchanged) and typed halt returned. `G-pending-restart`: stage a pending migration, restart the unit, assert it parks in `failed` with the `deploy` remediation, schema untouched. |
| **F4 (Fingerprint coherence — the disambiguation table §1.3)** | An interrupted deploy is classified **"incomplete, resume"** (cursor in `in_progress`/`step_committed` with matching `plan_hash`), never **"unknown drift"**. | `T-classify-incomplete-vs-drift`: matrix test over `{cursor state} × {fingerprint match}` asserting the §1.3 table exactly; mutate `deploy_cursor.plan_hash` to a foreign hash → asserts it flips to genuine-drift/refuse. |
| **F5 (No regression of P2/P3 — R2)** | Lifting `ApplyMigrations` keeps the Layer 2 watermark interlock and Layer 3 self-record intact; fresh-DB bring-up still works via the verb. | `T-decoupled-preserves-watermark`: flag ON + owner shortfall → still `awaiting_owner_ddl`. `T-fresh-db-via-deploy`: empty DB → `daemon deploy` → serveable, `schema_state` recorded, `LiveFingerprint == ExpectedFingerprint`. Existing `migrations_test.go:229-259` two-role guard stays green. |
| **F6 (DDL revocation — R3, structural)** | After bundle 0020, `striatumd_rw` cannot `CREATE` on schema `striatumd`; the deployer (owner connection) still applies runtime steps; no lockout across restart. | `T-revoke-blocks-rw-create` (two-role pgtest): `SET ROLE striatumd_rw; CREATE TABLE …` → `42501`. `T-deploy-after-revoke`: with 0020 applied, `deploy` applies a new runtime migration over the owner connection successfully. `G-revoke-then-restart`: apply 0020, flip flag, restart → daemon serves, no crash-loop. |
| **F7 (Receipt provenance — R4)** | Every applied step writes a hash-chained deploy receipt into `audit_log`; a schema change without a receipt is surfaced by `doctor schema_deploy_unrecorded`. | `T-receipt-per-step`: after `deploy`, assert one chained receipt per step, chain verifies. `T-doctor-unrecorded`: hand-bump `schema_state` without a receipt → `doctor` warns `schema_deploy_unrecorded`. |
| **F8 (Cursor has no double-apply/skip hole — R4)** | No interleaving of crash points across a step boundary yields a skipped or twice-applied step. | `T-deploy-fault-injection`: parametrized crash at each of {before marker, after marker/before step, after step/before commit-marker, after commit-marker} for both a T-DDL and an NT-DDL step; assert the final schema == plan exactly, once. |
| **F9 (Plan ≡ fingerprint binding)** | A `complete` deploy's recorded fingerprint equals `ExpectedFingerprint()`; deployer and drift gate never disagree on "fully applied." | `T-plan-hash-binds-fingerprint`: build a plan, apply it, assert `schema_state.fingerprint == ExpectedFingerprint()` and the `complete` cursor's `plan_hash` derives the same input set. |

---

## 6. Shadow-first rollout (the P4 build order)

1. **Migration ≥ 0044 `deploy_cursor`** (additive runtime table, modeled on
   `0043_schema_state.sql`; no FK, runtime-owned — passes
   `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`).
2. **`go/pkg/db/deploy.go`**: `DeployPlan`, `BuildPlan`, `Deployer.Apply` (Q3-A /
   Q3-B engine, advisory-locked via `MigrationLockKey`), receipt writer. Pure-core
   + DB-integration tests (F1, F2, F4, F8, F9) — engine proven **before** any boot
   path changes.
3. **`runDaemonDeploy`** verb (`daemon.go`) + matrix/authority-guardrail row +
   `--dry-run`. F3/F5 wiring.
4. **Decoupled boot path** behind `STRIATUM_DEPLOY_DECOUPLED` (default OFF) —
   `ConnectAndVerify` + `awaiting_deploy` typed halt. Lands inert (F3, F5).
5. **`doctor schema_deploy_unrecorded`** block (F7).
6. **Owner bundle ≥ 0020** (DDL revoke) — authored, tested in two-role pgtest
   (F6), **not auto-applied**; activation is the operator step that flips the flag.

Each phase is additive and reversible until the operator flips the flag + applies
0020. Self-record before enforce; detection before mutation-relocation; the verb
proven before the boot path leans on it.

---

## 7. Boundary & explicit deferrals

- **In P4:** the deployer verb, the deploy plan + `deploy_cursor` + deploy receipt,
  lifting `ApplyMigrations` out of serve-boot (flagged), serving-role create-DDL
  revocation (bundle ≥ 0020).
- **Explicitly deferred to P5 (Q1/Q2):** `striatum daemon rehearse`,
  `rehearsal_receipt.v1`, fidelity tiering, the full-data clone mechanism,
  expand/contract reshape primitive, lock-budget guardrail. P4 names the seams
  (plan_hash artifact, receipt chain, `Deployer.Apply` engine) that P5 builds on
  but ships none of it.
- **Local-first boundary held:** single host, ONE Postgres, ONE daemon as the
  single writer, no hosted services / replicas / external persistence. The
  deployer is a local verb over the existing owner + runtime DSNs.

---

## 8. Open tensions handed to the falsifiers (attack these hardest)

- **R1/F2 — NT-DDL resume is the soft spot.** Is "`in_progress` marker + invalid-
  index reconciler" airtight for *every* non-transactional verb we'd ship, or does
  `ALTER TYPE … ADD VALUE` (which cannot run in a transaction and has weaker
  idempotency than `CREATE INDEX … IF NOT EXISTS`) need its own narrower handling?
- **§4.1 — the residual capability.** Revoking only `CREATE ON SCHEMA` leaves
  `striatumd_rw` able to `ALTER` its own tables. Is "the serve *path* issues no
  DDL" a sufficient P4 close of failure mode 1, or does a falsifier construct a
  serve-path DDL that survives (e.g. a DML trigger / `DO` block that issues DDL)?
- **§3.2 — deriving the plan from the embed FS** assumes the pending-owner-then-
  pending-runtime ordering reproduces today's semantics exactly. Is there a real
  interleaving (an owner bundle that depends on a *runtime* object) that breaks the
  "owner prefix first" edge?
- **Q4 — bootstrapping.** Is there any path where plain-verb-now forecloses the
  run-shape later that the three seams (§2) don't actually keep open?

---

### Anchor table (re-verified against the worktree base)

| Target | Anchor |
| --- | --- |
| serve-boot mutation site (lift this) | `go/pkg/db/connection.go:332` `ConnectAndMigrate`; apply at `:353`; watermark `:349-352`; drift gate `:376-383`; self-record `:394-402` |
| boot fork / `--migrate` flag | `go/pkg/db/authority_bootstrap.go:181-201`; `go/cmd/striatumd/main.go:51,76,192-198`; typed halts `:200-228` |
| deployer command site | `go/pkg/cli/localcommands/daemon.go:67-81` dispatch; pattern `runDaemonOwnerDDL` `:90-159`, `runDaemonMigrate` `:167-210` |
| runtime apply engine to reuse | `go/pkg/db/migrations.go:121-178` `ApplyMigrations`; lock `:18`,`:122-134`; preflight `:158`; frontier `:17` (=43, new ≥0044) |
| owner-bundle engine / per-tx atomicity | `go/pkg/db/owner.go:204-258` `ApplyOwnerBundles`; `:454-484` `applyOneOwnerBundle` (one tx, stamp last); frontier `:23` (=19, new ≥0020); watermark `:35,91-110` |
| fingerprint contract (bind plan_hash to) | `go/pkg/db/schema_drift.go:83-100` `ExpectedFingerprint`; `:105-137` `composeFingerprint`; `:145-161` `LiveFingerprint`; `:171-195` `RecordSchemaFingerprint`; `:254-274` `CheckSchemaDrift` |
| `deploy_cursor` table model | `go/pkg/db/sql/0043_schema_state.sql:39-52` (singleton CHECK + `striatumd_rw` GRANT block) |
| doctor block model | `go/pkg/reads/doctor_schema_drift.go:26-77` |
| DDL-revoke surface | `go/pkg/db/sql/owner/0001_authority_phase0.sql:229-238` (role create + revokes/grants) |

<sub>Holder artifact for the RFC 0142 P4 falsification-gate design run. The
falsifiers (atomicity/resumability lens; decoupling/migration-safety lens) attack
the assertions in §5 and the tensions in §8; the adjudicator's collaboration
ledger — not falsifier completion — decides whether the gate clears.</sub>

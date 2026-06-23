# Design-Run Seed (v2 / REVISION) — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> **THIS IS A REVISION RUN.** A first design run (`rfc-0142-p4-design`) ran the
> same falsification gate and the adjudicator returned **`needs_revision`** with
> three material findings (C1, C2, C3 — see "Binding revision constraints" below).
> This v2 run is a **proper revision**: the holder starts from the v1 `HOLDER.md`
> (a required context doc), REVISES the spec to resolve **every** finding per its
> prescribed fix, and the falsifiers re-attack the revised spec. The v1 design
> record — `dialogue/holder/HOLDER.md`, `dialogue/falsifier_1/FALSIFIER.md`,
> `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — lives under
> `docs/operator/artifacts/rfc-0142-p4-design/`; the v1 `HOLDER.md` and the v1
> collaboration ledger are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed
> at `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries
> the charter, restates the two Open Questions P4 must pin (Q3 + Q4), pins the
> three binding revision constraints, and pins an operator anchor-verification
> table you must build on. Read this whole file, the v1 `HOLDER.md` + v1
> collaboration ledger context docs, and the RFC (esp. the Phasing table P4 row
> and the "Open Questions" section) before producing any artifact.

## Charter — what this run must produce

This is a **design run**, not an implementation run. RFC 0142 is **already
accepted**; this run does NOT re-open the five-layer design. The deliverable is a
**falsifiable implementation spec for P4 only** — the one-shot `striatum daemon
deploy` that lifts schema-apply out of serve-boot — that the `rfc-0142-p4-build`
run can execute contract-first (TDD), produced by hardening the P4 shape against
adversarial falsification.

The committed `PROPOSAL.md` MUST:

1. **Resolve Q3 and Q4** (below) with a concrete, defensible decision each. Q3 is
   "the hard correctness core" — a P4 spec that leaves the resumability contract
   unproven for the interleavings we ship has not cleared the gate.
2. **Specify the deployer surface, the serve-boot decoupling, and the DDL
   revocation** by exact code site (anchor table below), shadow-first.
3. **State every load-bearing correctness claim as a falsifiable assertion**
   paired with the named test / game-day step that would prove it false.
4. **Stay inside the accepted design and the local-first boundary**, and
   explicitly **defer P5** (rehearsal / expand-contract / fidelity tiering / clone
   mechanism = Q1/Q2) — P4 is the deployer + decoupling + DDL revocation only.
5. **Resolve EVERY binding revision constraint (C1, C2, C3) below**, each per its
   prescribed fix, and not regress the parts the v1 ledger judged sound. A revised
   spec that leaves any one of the three findings open has NOT cleared the gate.

## Binding revision constraints (design-v1 gate findings — MUST resolve)

> **These are BINDING.** The design-v1 falsification gate returned
> `needs_revision` because three material falsifier challenges landed and stood
> unrebutted, two of them squarely in the P4 correctness core. The revised spec
> clears the gate **only if it pins all three** with the prescribed fix; the
> cycle-2 falsifiers re-attack each one specifically and the adjudicator's
> clearing verdict requires every finding genuinely resolved (not merely claimed)
> AND no new material challenge standing. The verbatim prescribed fixes are copied
> from `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` (the `findings:`
> block and §4 "What the revision must fix"); read that context doc in full.

### C1 (critical — Q3 correctness core): the finalization boundary is unclassifiable

**The gap.** §1.2 of the v1 spec orders `deploy_cursor.state = complete` to be
written **before** the receipt append and `RecordSchemaFingerprint`. A crash
**after** the cursor reaches `complete` (with the **expected** `plan_hash`) but
**before** the receipt + `schema_state` fingerprint are durable leaves a state no
row of the load-bearing §1.3 classification table covers: not in-sync (that row
requires fingerprint **match**), not incomplete/resume (that row requires cursor
`in_progress`/`step_committed`), not genuine-drift (that row is `complete` with a
**foreign** `plan_hash`, or `aborted`). It hits **even a purely transactional
plan** (no `CREATE INDEX CONCURRENTLY` required) because the terminal `complete`
marker can commit ahead of the receipt and fingerprint. So the per-step-atomic +
resumable-cursor contract is insufficient at the final boundary with no
sub-protocol specified — **per the role rubric this alone forces
`needs_revision`.**

**Prescribed fix (binding).** Adopt **one** concrete finalization sub-protocol and
add the matching §1.3 row:
- *Option A:* keep the cursor at `step_committed(N-1)` until the deploy receipt
  **and** the `schema_state` fingerprint are durable, then set `complete`
  **last** — so the existing incomplete/resume row covers finalization retry; or
- *Option B:* add a distinct `finalizing` state and classify
  `finalizing` / expected `plan_hash` / fingerprint-mismatch as **resumable
  finalization** (never serve, never genuine-drift).

Either way: **state which finalization writes share a transaction/role**, or
specify an **idempotent finalizer** that runs correctly after every crash point.
Add `T-deploy-resume-finalization-crash` that kills after each of: final DDL
commit, cursor-`complete`-before-receipt, receipt-before-`schema_state`, and
`schema_state`-before-cursor-`complete` — asserting **no serving until
finalization is coherent**, exactly-once repair on re-run, and
`doctor schema_deploy_unrecorded` green after repair.

### C2 (high — R3 lockout / serve-boot regression): 0020 + `STRIATUM_DEPLOY_DECOUPLED` activation is not fail-closed

**The gap.** The owner watermark **tolerates a forward bundle** (`applied >
required` returns `nil`, `owner.go:76-80,104-109`), so with owner bundle 0020
applied and the flag OFF/absent (a pre-P4 binary, OR a P4 binary with the flag
unset) boot still reaches `ApplyMigrations` over the **runtime** runner
(`connection.go:349-353`). Under the revoked `CREATE` (0020 revokes `CREATE ON
SCHEMA striatumd FROM striatumd_rw`) any pending runtime migration — including the
P4 `deploy_cursor` migration `>= 0044` before `daemon deploy` has run — fails with
a raw `42501` that `main.go` does **not** map to a typed halt (it recognizes only
`AwaitingOwnerDDLError` and `SchemaDriftError`, `main.go:199-228`), so it falls
through to the fatal crash-loop path — the exact **#512-class lockout** P4 exists
to close. The v1 "even a botched order refuses cleanly" claim is falsified against
current source, and the spec self-contradicts ("auto-apply still the default" vs
`RequiredOwnerBundleVersion = 20`).

**Prescribed fix (binding).** Make activation **fail closed before mutation**:
- Add a typed non-restartable halt (`awaiting_deploy` / `awaiting_deploy_config`)
  that fires **before** `ConnectAndMigrate` reaches `ApplyMigrations` whenever
  `owner_bundle_meta >= 20` **and** (decoupled mode OFF **or** the deploy is
  incomplete). The DB must be **untouched** and the remediation must name
  `striatum daemon deploy` / the missing flag/config state.
- Add a **forward-watermark rule** so an older binary that observes applied owner
  bundle 0020 does **not** proceed into runtime auto-apply as if forward tolerance
  were harmless.
- **Resolve** the "auto-apply still default" vs `RequiredOwnerBundleVersion = 20`
  contradiction, and state the deploy choreography precisely (daemon up/down,
  which binary, parked vs. serving).
- Add `T-deploy-revoke-activation-ordering` over: 0020-before-flag; 0020-before
  `daemon deploy`; old-binary + 0020 + pending runtime migration; P4-binary +
  flag-OFF + pending runtime migration — asserting `ApplyMigrations` is **not**
  called, the DB is **untouched**, and the remediation names
  `striatum daemon deploy` / the missing flag.

### C3 (high — two-role boundary): runtime-object ownership contract is undefined under owner/admin apply

**The gap.** The v1 spec routes all post-0020 runtime DDL through the owner/admin
connection, but `applyOne` runs the migration SQL on the supplied runner with
**no `SET ROLE striatumd_rw`, no `ALTER … OWNER TO striatumd_rw`, and no post-step
grant/ownership reconciliation** (`migrations.go:304-335`). Run through
owner/admin, the same SQL creates **owner-owned** runtime objects — silently
changing ownership semantics of every future runtime migration and undermining
the §4.1 residual-capability argument, which leans on new runtime objects staying
`striatumd_rw`-owned. The spec uses **both** policies and commits to **neither**.

**Prescribed fix (binding).** Choose **one** policy and **test** it:
- *If runtime objects stay `striatumd_rw`-owned:* specify the post-step
  ownership/grant transfer (`ALTER … OWNER TO` + grants) covering tables, indexes,
  sequences, views, and **future object kinds**; or
- *If owner/admin owns new runtime objects:* add a build/load guard that every
  runtime migration grants the **exact DML** the daemon needs, **and** correct
  §4.1 so it no longer claims runtime ownership as a P4 safety property for new
  objects.

Add `T-deploy-runtime-object-ownership`: apply a new runtime migration through the
deployer's owner connection and assert **both** the recorded catalog owner **and**
the serving role's real DML behavior (SELECT/INSERT/UPDATE/DELETE) under
`striatumd_rw`, plus DDL failure after bundle 0020.

### What already cleared (carry forward — do NOT regress or re-litigate)

The v1 ledger §5 records these as sound; the revision must preserve them:
- **Q4 — resolved.** Plain verb now (`striatum daemon deploy`) with the
  bootstrapping paradox dispositive against a run-shape base deployer, and the
  run-shape door held open by three concrete seams (content-addressed `plan_hash`,
  hash-chained receipt in `audit_log`, a `Deployer.Apply` engine the verb and a
  future run share). Neither falsifier contested it.
- **Q3 mid-step resumability — sound.** Q3-A (cursor advance in the **same**
  transaction as a transactional step) and Q3-B (`in_progress` pre-marker +
  invalid-index reconciler for `CREATE INDEX CONCURRENTLY` / `ALTER TYPE … ADD
  VALUE`), the step taxonomy (§1.1), and the "`deploy_cursor` is the **second
  signal** disambiguating incomplete from drift" insight. The defect is at the
  **final** boundary (C1), not the per-step body — preserve the per-step design.
- **Decoupling shape — sound; only the activation edge (C2) is missing.**
  Shadow-first default-OFF `STRIATUM_DEPLOY_DECOUPLED`, `ConnectAndVerify`
  preserving the P2 watermark interlock and P3 drift gate/self-record, the
  `awaiting_deploy` typed halt, the embed-FS-**derived** plan (no second
  hand-authored manifest), the hash-chained deploy receipt, and
  `doctor schema_deploy_unrecorded`.
- **Scope discipline held.** P5 (rehearse / expand-contract / fidelity tiering /
  clone) is deferred; the local-first boundary (one host, one Postgres, one daemon
  writer, no external persistence) is respected.

## Root reframe (do not lose this)

**Schema mutation must stop being an implicit side effect of the serving
process's restart and become an explicit, ordered, resumable, provenance-tracked
operation owned by a dedicated deployer.** Then the serving daemon can hold zero
DDL privilege, "restart force-commits a half-applied deploy" becomes impossible,
and a bad migration can never wedge the single writer on boot.

## The two Open Questions P4 must pin (from the RFC)

- **Q3 — How atomic is "atomic"?** Confirm the **per-step-atomic + resumable-
  cursor** contract is sufficient for every owner+runtime interleaving we actually
  ship, or specify the small set of steps that need a stricter
  single-connection/single-transaction sub-protocol. Every step must be idempotent
  and leave a coherent intermediate the fingerprint classifies as "incomplete,
  resume" — not "unknown drift, panic". *(RFC: "This is the hard correctness core
  of P4.")*
- **Q4 — Should a deploy be a Striatum run?** Plain verb vs. a dogfooded run shape
  (`expand_rehearsal` → `contract_swap`), with the bootstrapping paradox (a run
  needs a schema to run the deploy that changes the schema). Decide before P4/P5
  lock the verb surface.

## Load-bearing risks (attack these)

- **R1 atomicity-is-partly-a-lie:** non-transactional DDL (`CREATE INDEX
  CONCURRENTLY`, `ALTER TYPE … ADD VALUE`), non-idempotent steps, or a two-
  connection (owner+runtime) crash window that the fingerprint reads as "unknown
  drift, panic". Test: kill-and-resume across each step class.
- **R2 decoupling regresses a landed gate:** lifting `ApplyMigrations` out of
  `ConnectAndMigrate` must NOT break the P2 watermark interlock, the P3 drift gate
  / fingerprint self-record, or fresh-DB bring-up; no window where the daemon
  serves on an unmigrated schema.
- **R3 DDL-revocation lockout:** revoking serving-role DDL (owner bundle ≥ 0020)
  must not lock out the runtime path before the deployer exists, nor recreate a
  #512-class lockout (the role that must run the deploy can't, across a restart).
- **R4 cursor holes:** double-apply/skip at a commit boundary; receipt written
  out of step with the real schema; out-of-order apply under the plan's edges.

## Anchor verification against current `main` (operator pre-flight, 2026-06-22)

Verified against `~/git/striatum` @ `main`. P0–P3 + P2 are **landed**; the P4
surfaces are **NOT-FOUND (to be built)**. Treat as ground truth; re-anchor the
spec to these file:line references.

| Claim / target | Status | Anchor (current source) |
| --- | --- | --- |
| Boot-time auto-apply runs runtime migrations as `striatumd_rw` | **ACCURATE (the coupling P4 removes)** | `go/pkg/db/connection.go:353` `ApplyMigrations(ctx, pool.Runner, daemonVersion)`; called from boot via `db.BootstrapAndConnect()` at `go/cmd/striatumd/main.go:193,199`. |
| Runtime migration frontier = 0043 (P3 added `schema_state`) | **ACCURATE** | `go/pkg/db/migrations.go:17` `LatestDaemonDBVersion = 43`; `:74` migration 43 = "schema-fingerprint drift gate state (RFC 0142 P3 / #570)". New P4 migration is **≥ 0044**. |
| Owner bundle frontier = 0019 | **ACCURATE** | `go/pkg/db/owner.go:23` `LatestOwnerBundleVersion = 19`. New P4 owner bundle (DDL revoke) is **≥ 0020**. |
| P3 fingerprint machinery (P4 builds on it) | **ACCURATE (landed)** | `go/pkg/db/schema_drift.go`: `ExpectedFingerprint()` `:83-100`, `LiveFingerprint()` `:145-161`, `CheckSchemaDrift()` `:254-274`, env `STRIATUM_SCHEMA_DRIFT_REFUSE` `:28` (shadow-first: log+continue default, refuse only when set); doctor block `go/pkg/reads/doctor_schema_drift.go:26-77`; boot sequence `connection.go:376-399` (check drift after apply, then `RecordSchemaFingerprint`). |
| `schema_state` table records last-applied fingerprint | **ACCURATE (landed)** | `go/pkg/db/sql/0043_schema_state.sql:39-44` `striatumd.schema_state(id singleton, fingerprint, daemon_version, applied_at)`; UPSERT `schema_drift.go:187-194`. |
| P2 watermark interlock + clean halt | **ACCURATE (landed, #574)** | `go/pkg/db/owner.go:37-64` `ErrAwaitingOwnerDDL`/`AwaitingOwnerDDLError`, `:91-110` `CheckOwnerBundleWatermark`; called **before** `ApplyMigrations` at `connection.go:349-352`; clean non-restartable exit `cmd/striatumd/main.go:208-214`. |
| Two-role boundary build guard | **ACCURATE (landed)** | `go/pkg/db/migrations_test.go:229-259` `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` (+ FK-into-owner guard); load-time `preflightRuntimeMigrationOwnership()` `migrations.go:158`. |
| `owner-ddl apply` applies bundles out-of-band as owner role, advances `owner_bundle_meta` | **ACCURATE (landed)** | `go/pkg/cli/localcommands/daemon.go:90-159` `runDaemonOwnerDDL` (connects via `--owner-url`/`STRIATUM_DAEMON_ADMIN_DB_URL`), calls `db.ApplyOwnerBundles()` `owner.go:204-249`. P4's `deploy` lives alongside this. |
| Two roles: owner/bootstrap vs runtime `striatumd_rw`; runtime has NO DDL on owner tables | **ACCURATE** | `go/pkg/db/authority_bootstrap.go:31-46` (`RuntimeURL`/`OwnerURL`/`RuntimeRole`, default `striatumd_rw` `main.go:660`); `go/pkg/db/sql/owner/0001_authority_phase0.sql:229-238` revokes direct `audit_log` INSERT, grants only the SD function. P4's DDL-revoke makes the serving path's zero-DDL explicit. |
| `striatum daemon deploy` / `deploy_cursor` / deploy plan / deploy receipt | **NOT-FOUND (P4 builds these)** | `daemon.go:62-82` subcommands = `install,uninstall,status,migrate-db,owner-ddl` — no `deploy`. No `deploy_cursor` table, no plan/receipt machinery anywhere. |

**Net design implication.** The P4 ground is clean: P2 (watermark interlock) and
P3 (drift gate + fingerprint self-record) are landed and are exactly the
contract P4 leans on. The hard part is **Q3** — proving the per-step-atomic +
resumable-cursor contract holds for the real interleavings (especially
non-transactional DDL and the owner+runtime two-connection boundary), and lifting
`ApplyMigrations` out of `connection.go` without regressing P2/P3 or fresh-DB
bring-up. Falsifiers must press the resumability contract hardest. Be
shadow-first: serve-boot auto-apply stays the default until the deployer is
proven, then flips behind a flag.

---
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run (v2 /
REVISION of `rfc-0142-p4-design`; resolves the cycle-1 findings C1/C2/C3). Lanes:
author=claude (holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>

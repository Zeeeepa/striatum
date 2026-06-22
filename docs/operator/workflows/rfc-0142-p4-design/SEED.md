# Design-Run Seed — RFC 0142 P4: the one-shot deployer (decouple schema-apply from serve-boot)

> This document is the **required input** for the RFC 0142 P4 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed
> at `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
> **accepted, D258**) — read it in full as your primary source; this SEED carries
> the charter, restates the two Open Questions P4 must pin (Q3 + Q4), and pins an
> operator anchor-verification table you must build on. Read this whole file and
> the RFC (esp. the Phasing table P4 row and the "Open Questions" section) before
> producing any artifact.

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
<sub>Operator scaffold for the RFC 0142 P4 falsification-gate design run. Lanes:
author=claude (holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>

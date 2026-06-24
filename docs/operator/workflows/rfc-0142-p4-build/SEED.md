# SEED — RFC 0142 P4 one-shot deployer (build run)

This run **builds RFC 0142 P4 — the one-shot `striatum daemon deploy` that decouples
schema-apply from serve-boot**, **test-first**. The design is falsification-cleared (v9,
`accept_with_findings`); this is execution, not design. The deliverable is **real Go
source** in this repo.

## Read these first (your context docs)

- **`docs/operator/artifacts/rfc-0142-p4-design-v9/commit/proposal/PROPOSAL.md`** — the
  falsification-cleared implementation spec. **This is your primary, authoritative
  contract.** It gives the exact §6 seven-step build order, the §6.5 twelve acceptance
  criteria (a)-(l), the binding finding B1 (B1.1/B1.2), the full assertion set (§5 /
  F1–F18, G-* named tests), and the §8 anchor table (exact source sites per target). Build
  to it contract-first (TDD).
- **`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`**
  — the cycle-1 collaboration ledger containing **finding B1** (the two non-blocking
  build-phase obligations the build run MUST honor, folded into §6.5 as binding acceptance
  criteria). Read it to understand B1.1 and B1.2 before writing any test.
- `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` — the RFC (background:
  Phasing P4 row, Open Questions 3+4, "the hard correctness core of P4").
- `AGENTS.md` — product boundary + build conventions (Go module in `go/`; golangci-lint
  pinned in `go/Makefile`; CI runs pgtests; reproduce lint/test locally before claiming
  green).
- Source anchors (§8): `go/pkg/db/owner.go`, `go/pkg/db/schema_drift.go`,
  `go/pkg/db/connection.go`, `go/pkg/db/migrations.go`,
  `go/pkg/db/owner_runtime_ownership.go`, `go/pkg/reads/doctor_schema_drift.go`,
  `go/pkg/pgtest/two_role.go`, `go/cmd/striatumd/daemon.go`,
  `go/cmd/striatumd/authority_bootstrap.go`.

## RECOVERY ADOPTION (this run only — read FIRST)

A prior draft of this exact build already implemented the FULL contract test-first and
self-verified it CLEAN (`go build`/`go vet`/CI `golangci-lint` green; non-PG tests green;
pg-integration tests compile and skip without `STRIATUM_PG_TEST_URL`). That work could not
be sealed only because the run's write_scope used the unsatisfiable pattern `go/**` (a
daemon prefix-matcher footgun, GH #586) — a tooling defect, NOT a code problem. The verified
implementation is preserved on branch **`backup/rfc-0142-p4-build-impl-2026-06-24`** (22
files: migration `0044_deploy_cursor.sql`, owner bundle `0021_revoke_create_privilege.sql`,
`go/pkg/db/deploy.go` + `deploy_apply.go` + `deploy_activation.go`, `doctor_deploy.go`, the
`runDaemonDeploy` verb wiring, and the F1/F16/F18/B1.1/B1.2/M1/M3 tests, plus edits to
`owner.go`/`connection.go`/`migrations.go`/`main.go`/`daemon.go`/`doctor.go`).

**Your job: ADOPT and VERIFY that preserved implementation, do not re-implement from
scratch.** In your worktree: `git fetch origin backup/rfc-0142-p4-build-impl-2026-06-24`,
then bring its source into your worktree (e.g.
`git checkout origin/backup/rfc-0142-p4-build-impl-2026-06-24 -- go/ docs/operator/artifacts/rfc-0142-p4-build/`).
Then VERIFY against the PROPOSAL §5 assertions + §6.5 (a)-(l) + finding B1: run `go build
./...`, `go vet ./...`, the CI `golangci-lint`, and `go test ./...` (non-PG) from `go/`.
Read every adopted file critically against the contract; fix any gap you find against the
spec (the prior draft is strong but you are the author of record — own it). Publish `DRAFT.md`
describing the implementation, then `work.complete`. The write_scope is now `go/` (fixed), so
the seal will succeed. The seven-step contract below is the spec the adopted code must satisfy.

## Scope — build ALL seven steps in PROPOSAL §6 order (contract-first)

This run implements the FULL P4 contract from PROPOSAL.md §6 in the prescribed seven-step
order. There is no "layer 2 is OUT" carve-out — all seven steps ship:

1. **Migration 0044** (`deploy_cursor` + `deploy_plan`) — additive runtime-owned tables;
   `state` CHECK includes `finalizing`; modeled on `0043_schema_state.sql:39-52`; grants
   `striatumd_rw` via `DO` block (per-step receipt SD fn model at `0001_authority_phase0.sql:152`).
2. **`owner.go` M2 surface (inert):** `DDLRevokeOwnerBundleVersion = 21`, `isNonRevokeBundle`,
   `OwnerDDLApplyBundles()`, in-loop guards, nil-fallback split, plus the **F16a
   synthetic-phase test + build-time grep test (M4)**. `LatestOwnerBundleVersion` and
   `RequiredOwnerBundleVersion` STAY 20 — do NOT advance them.
3. **`go/pkg/db/deploy.go`** (new file): `DeployPlan`, `BuildPlan` (0021-terminal, full
   `OwnerBundles()`), `LoadStoredPlan`, `VerifyStoredTranscript` (M1) + typed mismatch halts,
   `Deployer.Apply` (Q3-A/Q3-B engine + `finalizing` finalizer with `VerifyStoredTranscript`
   step 0), substrate-ensure preamble, `applyRuntimeStep` (C3 reconcile), receipt writer.
   Pure-core + DB-integration tests (F1, F2, F4, F8, F9, F10, F12, F13, F14, F15) proven
   BEFORE any boot-path changes.
4. **`runDaemonDeploy` verb** + matrix/authority-guardrail row + `--dry-run`/`--abort` + the
   0021-activation preflight. F3/F5 wiring. Dispatch added to `daemon.go:67-81`.
5. **`CheckDeployActivation`** with the M3/M5/M6/M7 predicate — the hoisted
   `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` config gate (step 0, every
   cursor state) + the no-revoke `complete` pre-`ApplyMigrations` comparison (step 3, neither
   reading `applied_owner`, M6) + the decoupled complete branch reading neither `applied_owner`
   NOR `revokeEmbedded` (M7, §0.2 sub-invariant) — behind `STRIATUM_DEPLOY_DECOUPLED`
   (default OFF). Universal pre-revoke cursor edge (BC-N2) + typed halts +
   M5-correct `CheckOwnerBundleWatermark` (fresh `applied_owner == 0` serves; `1..19` halts;
   forward-watermark rule at `>=21`). Tests: F11 (incl. (e)(f)(g)), F3, F5, F17, **F18
   parametric** over the seven A-reaching complete-row cells + column-identity +
   row-15/row-16 identity + unchanged 4-cell spy list, **F18a** (fresh-DB bootstrap).
   Boot-path typed-halt arms added to `authority_bootstrap.go:208-227`.
6. **`doctor schema_deploy_unrecorded`** block — per-step tightened, transcript-enumerated +
   M1 stamp/byte WARN. Model: `go/pkg/reads/doctor_schema_drift.go`. Tests: F7, F4, F15
   doctor arm.
7. **Owner bundle 0021** (DDL revoke) — authored, deploy-plan-terminal, excluded from every
   `owner-ddl apply` route (`LatestOwnerBundleVersion` STAYS 20). F16b production-phase test +
   forced-self-heal pgtest land here (M4). Two-role pgtest (F6, F12, F16b); activation is the
   operator choreography (§4.3).

**Each phase additive and reversible.** Core proven before boot-path leans on it.

## Shadow-first invariants (hard constraints — do not violate)

- **Default OFF.** All new deploy-path behavior is behind `STRIATUM_DEPLOY_DECOUPLED`
  (environment variable, default OFF). The existing `ConnectAndMigrate` serve-boot path is
  UNCHANGED when the flag is absent or false.
- **Migration 0044 is ADDITIVE runtime-owned.** `deploy_cursor` + `deploy_plan`, `state`
  CHECK includes `finalizing`. No owner DDL, no `owner_bundle_meta` touch.
- **Owner bundle 0021 lands INERT and is EXCLUDED from every `owner-ddl apply` route.**
  `LatestOwnerBundleVersion` and `RequiredOwnerBundleVersion` STAY 20 (do NOT advance them).
  `DDLRevokeOwnerBundleVersion = 21` is a new constant; `OwnerDDLApplyBundles()` filters it
  out. The `LatestOwnerBundleVersion = 20` anchor (`owner.go:23`) is unchanged.
- **Self-record before enforce; detection before mutation-relocation.** Each step builds on
  proven prior steps. The deploy core (`deploy.go`) is proven before the boot path leans on it.
- **The fresh `applied_owner == 0` bootstrap MUST still serve (M5).** `CheckOwnerBundleWatermark`
  at `owner.go:145` returns nil for `applied_owner == 0` — this is not changed.
- **Do not regress M6 (rows 13/15) or M7 (row 16).** The no-revoke complete in-sync cells
  (13/`==0`, 13/`==20`) reach the legacy `:399` writer as an idempotent no-op; the decoupled
  complete cells (rows 15/16, in-sync) serve verify-only; the decoupled complete cells
  (rows 15/16, out-of-sync) halt `awaiting_deploy` DB-untouched.
- **The existing serve-boot test suites MUST pass UNCHANGED.** No preflight/timeout/trust
  change. `connection.go:399` ownership note ("This is the only writer of schema_state") is
  still true for the non-deploy serve-boot path.

## Finding B1 (binding acceptance criteria — MUST honor both)

**B1.1 — F18 must actually exercise the row-16 derivation.**
`T-deploy-bootpath-decision-table` MUST construct the row-16 in-sync AND out-of-sync
sub-cases for `applied_owner == 0`, `==20`, and `>=21` (the in-sync arm independently
setting `schema_state.fingerprint == ExpectedFingerprint()` AND `cursor.plan_hash == expected`
over an `owner_bundle_meta`-absent / 20 / `>=21` DB, proving orthogonality) and assert the
in-sync row-16 cells serve verify-only WITHOUT firing `ApplyMigrations`/`RecordSchemaFingerprint`
spies. Omitting these recreates M7 in code.

**B1.2 — Expand the grouped cursor-state shorthand.**
The "64-cell" shorthand groups `step_committed` with `in_progress` and `aborted` with the
non-complete edge. The executable test MUST table-drive each **concrete** cursor-state enum
named by F18: `none`/`in_progress`/`step_committed`/`finalizing`/`complete`/`aborted`.
Not the prose group labels — the enum values.

## §6.5 Acceptance criteria (build run must meet all twelve)

- **(a)** Crash-resume stable key (BC-N1) — F13/F14, `G-...`.
- **(b)** Divergent-binary resume refuses (M1) — F15, `G-wrong-binary-resume`.
- **(c)** Universal pre-revoke serve edge (BC-N2) — F11(e)/(f), `G-old-binary-refuse`.
- **(d)** Self-heal does not commit the revoke early (M2) — F16b, `T-deploy-revoke-excluded-from-reapply-self-heal`.
- **(e)** Complete-cursor flag-OFF revoke-embedding refusal (M3) — F17, `G-complete-cursor-flag-off-refuse`.
- **(f)** Fresh-DB serve + shortfall halt (M5/row-1) — F18a, `G-fresh-db-first-boot`.
- **(g)** No-revoke complete in-sync/out-of-sync (M6 — rows 15/`==0`) — F18.
- **(h)** Degenerate legacy in-sync no-op (M6 — F18 spy list, row 13/`==0`-in-sync) — F18.
- **(i)** Revoke-embedding complete in-sync/out-of-sync at `==0` (M7 — row 16/`==0`) — F18 parametric.
- **(j)** Revoke-embedding complete in-sync/out-of-sync at `==20` and `>=21` (M7 — row 16/`==20`, `>=21`) — F18 parametric.
- **(k)** Hash-chained receipt + doctor (BC-N1) — F7/F4/F14.
- **(l)** Two-role activation + post-deploy CREATE denial (C3) — F12, `G-revoke-last`.

## Conventions

- Go module is in `go/` — run `go build ./...`, `go vet ./...`, `golangci-lint run`, and
  `go test ./...` from `go/`. CI runs pgtests; reproduce locally before claiming green. Use
  `STRIATUM_PG_TEST_URL` (not the daemon DSN) for tests requiring Postgres.
- The two-role pgtest fixture is `go/pkg/pgtest/two_role.go:130` (`ApplyOwnerBundles`,
  non-superuser bootstrap) — extend it for F12/F16b/F17/F18/F18a.
- New `go/pkg/db/deploy.go` is the canonical home for all deployer types and the
  `Deployer.Apply` engine.
- Migration SQL goes under `go/pkg/db/sql/` (modeled on `0043_schema_state.sql`); owner
  bundle SQL under `go/pkg/db/sql/owner/` (modeled on existing owner bundles).
- Stay in write_scope (`go/`, `docs/operator/artifacts/rfc-0142-p4-build/`). Do not
  touch `.striatum/` or `docs/operator/workflows/**`.
- Match `expected_artifacts[].author_line` exactly if any artifact's title block
  specifies `author:`.

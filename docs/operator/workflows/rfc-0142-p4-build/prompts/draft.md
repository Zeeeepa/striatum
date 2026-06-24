# Task — Draft: build RFC 0142 P4 (the one-shot `striatum daemon deploy` decoupler), test-first

Read `SEED.md` and your context docs first — above all
**`docs/operator/artifacts/rfc-0142-p4-design-v9/commit/proposal/PROPOSAL.md`** (your
authoritative contract: §6 build order, §6.5 acceptance criteria, §5 assertions, §8
anchor table), the cycle-1 adjudicator ledger (finding B1 binding obligations), `AGENTS.md`,
and the §8-named source anchors.

You make **real source changes** in your worktree (this lane has
`publish_source_changes: true`). The actual Go code is the deliverable; `DRAFT.md` just
describes it. Hold the root reframe: **schema mutation must stop being an implicit side
effect of the serving process's restart** and become an explicit, ordered, resumable,
provenance-tracked operation.

## Build, in PROPOSAL §6's contract-first seven-step order (TDD — write the named test first,
## watch it fail for the right reason, then implement until green)

### Step 1 — Migration 0044: `deploy_cursor` + `deploy_plan`

New runtime-owned tables, additive, created by migration `0044_deploy_cursor.sql` (or
similar) under `go/pkg/db/sql/`. Model on `0043_schema_state.sql:39-52` (singleton CHECK
pattern; `striatumd_rw` GRANT `DO` block). The `state` CHECK must include `finalizing` as a
valid value:
`state TEXT NOT NULL CHECK (state IN ('idle','in_progress','step_committed','finalizing','complete','aborted'))`.
`deploy_plan` is append-only, keyed by `plan_hash`, INSERT-once
(`ON CONFLICT (plan_hash) DO NOTHING`), `steps jsonb`. Both tables runtime-owned; no owner
DDL; no `owner_bundle_meta` touch.

### Step 2 — `owner.go` M2 surface (lands inert)

In `go/pkg/db/owner.go`:
- Add `DDLRevokeOwnerBundleVersion = 21` constant (new; `LatestOwnerBundleVersion` stays 20;
  `RequiredOwnerBundleVersion` stays 20).
- Add `isNonRevokeBundle(v int) bool` predicate.
- Add `OwnerDDLApplyBundles() []OwnerBundle` — returns all bundles with
  `version < DDLRevokeOwnerBundleVersion` (filters 0021 out).
- Adjust `ApplyOwnerBundles`, `applyPendingOwnerBundles`, `ReapplyAllOwnerBundles` to route
  through `OwnerDDLApplyBundles()` instead of the full list wherever they apply non-revoke
  bundles; preserve the nil-fallback split and in-loop guard pattern.
- Add the **F16a synthetic-phase test**: a test that supplies a synthetic bundle list (not
  the production embed) and asserts 0021 is excluded from apply; plus a **build-time grep
  test (M4)** asserting `OwnerBundles()` is not called directly in apply routes that should
  route through `OwnerDDLApplyBundles()`.
- This step is inert until 0021 is authored (step 7). Existing `owner_pg_test.go` suites
  must pass unchanged.

### Step 3 — `go/pkg/db/deploy.go` (new file): the deploy core

New file with:
- `DeployPlan` type: immutable ordered transcript `Steps []DeployStep`, `PlanHash string`,
  `BaseOwnerVersion`, `BaseRuntimeVersion`, `TargetOwnerVersion`, `TargetRuntimeVersion`,
  `RevokeStepIndex int`.
- `BuildPlan(base_owner, base_runtime int) (*DeployPlan, error)` — assembles from full
  `OwnerBundles()` (0021-terminal; 0021 is the final step).
- `LoadStoredPlan(ctx, db, planHash string) (*DeployPlan, error)`.
- `VerifyStoredTranscript(plan *DeployPlan, stored *DeployPlan) error` — full byte-match
  check; returns typed `DeployPlanBinaryMismatch` or `DeployPlanDBStampMismatch` errors on
  divergence.
- `Deployer` type with:
  - `Apply(ctx, db, adminDB)` — the Q3-A/Q3-B engine: substrate-ensure preamble (materialize
    `deploy_plan` row + set cursor `in_progress(0)` in one tx before step 0); per-step loop
    (Q3-A transactional; Q3-B NT-DDL with pre/post markers); `finalizing` finalizer with
    `VerifyStoredTranscript` as step 0 (abort on mismatch), then append `complete` receipt,
    then `RecordSchemaFingerprint`, then advance `finalizing → complete`.
  - Resume verification (M1): on every resume, after loading `deploy_plan[cursor.plan_hash]`
    and BEFORE applying or finalizing any step, run `VerifyStoredTranscript`.
  - `applyRuntimeStep`: step + ownership-reconcile (C3) + version stamps + cursor advance +
    receipt in one tx.
  - Receipt writer: hash-chained, keyed on `(plan_hash, step_index)`.

Pure-core + DB-integration tests (F1/F2/F4/F8/F9/F10/F12/F13/F14/F15) proven BEFORE any
boot-path changes. Use the two-role fixture (`go/pkg/pgtest/two_role.go`) where needed.

### Step 4 — `runDaemonDeploy` verb

In `go/cmd/striatumd/daemon.go`:
- Add `"deploy"` to the dispatch at `:67-81` (alongside existing verbs).
- `runDaemonDeploy(ctx, flags)` function: parses `--dry-run`/`--abort`; calls
  `CheckOwnerBundleWatermark` then `CheckDeployActivation` (step 5, added next); invokes
  `Deployer.Apply`; 0021-activation preflight (requires `STRIATUM_DEPLOY_DECOUPLED`).
- Add authority-guardrail matrix row for `deploy` (per `docs/reference/command-authority-matrix.md`
  pattern; needs admin DSN like `runDaemonOwnerDDL:115`).
- Wire F3/F5 (verb-dispatch and dry-run tests).

### Step 5 — `CheckDeployActivation` (the M3/M5/M6/M7 predicate, behind `STRIATUM_DEPLOY_DECOUPLED`)

This is the heart of the boot-path decoupling. In `go/pkg/db/connection.go` (or a new
`go/pkg/db/deploy_activation.go`):

`CheckDeployActivation(ctx, db, revokeEmbedded, decoupledEnabled bool, cursorState string, planHash, expectedPlanHash string) (BootDecision, error)`

The function implements the §3.3a/§3.5 logic exactly:
- **Step 0 (M3 config gate, every cursor state):** if `revokeEmbedded && !decoupledEnabled →
  return awaiting_deploy_config`. DB-untouched.
- **Step 1 (BC-N2 pre-revoke edge):** if cursor is non-`complete` AND `applied_owner == 20` →
  return `awaiting_deploy` DB-untouched.
- **Step 2 (non-complete cursors other than `complete`):** return `awaiting_deploy`.
- **Step 3 (complete cursor):**
  - If `!decoupledEnabled` (no-revoke path): compare `ExpectedFingerprint() == LiveFingerprint`
    AND `planHash == expectedPlanHash`. If in-sync → allow legacy serve (`:399` path for rows
    13/`==0`, 13/`==20`). If out-of-sync → `awaiting_deploy`. Neither reads `applied_owner`
    (M6).
  - If `decoupledEnabled`: compare `cursor.plan_hash == expected` AND
    `LiveFingerprint == ExpectedFingerprint()`. If in-sync → serve verify-only (rows 15/16,
    any `applied_owner` bucket). If out-of-sync → `awaiting_deploy`. Reads neither
    `applied_owner` NOR `revokeEmbedded` (M7, §0.2 sub-invariant).

The `ConnectAndVerify` decoupled boot path (behind `STRIATUM_DEPLOY_DECOUPLED`) calls
`CheckOwnerBundleWatermark` (W, `owner.go:124-154`) then `CheckDeployActivation` (A) BEFORE
`ApplyMigrations` in the decoupled case. The typed halts (`awaiting_deploy`,
`awaiting_deploy_config`, `deploy_plan_binary_mismatch`, `deploy_plan_db_stamp_mismatch`,
`awaiting_owner_ddl`) map to arms in `authority_bootstrap.go:208-227`.

**Tests (F11 incl. (e)/(f)/(g), F3, F5, F17, F18 parametric, F18a):**

F18 (`T-deploy-bootpath-decision-table`) is the critical oracle. It MUST (B1.1 + B1.2):
- Table-drive each **concrete** cursor-state enum: `none`, `in_progress`, `step_committed`,
  `finalizing`, `complete`, `aborted` — NOT grouped shorthands.
- For the `complete` cursor row, construct BOTH in-sync AND out-of-sync sub-cases for
  `applied_owner == 0`, `==20`, and `>=21` (six concrete DB states for row 16 alone).
- For the row-16 in-sync sub-cases (`applied_owner == 0`, `==20`, `>=21`): independently set
  `schema_state.fingerprint == ExpectedFingerprint()` AND `cursor.plan_hash == expected` over
  an `owner_bundle_meta`-absent / `==20` / `>=21` DB, and assert serve verify-only WITHOUT
  firing `ApplyMigrations`/`RecordSchemaFingerprint` spies. Omitting these recreates M7 in
  code.
- The four `:399`-reaching spy cells (row 1/`==0`, 1/`==20`, 13-in-sync/`==0`,
  13-in-sync/`==20`) are enumerated UNCHANGED.

F18a (`T-deploy-fresh-db-bootstrap-serves`): fresh `applied_owner == 0` (no `owner_bundle_meta`
row) boots and SERVES — not wedged.

### Step 6 — `doctor schema_deploy_unrecorded` block

In `go/pkg/reads/doctor_schema_drift.go` (or a new sibling file), model on
`schemaDriftDoctorBlock`. Add a `deployUnrecordedDoctorBlock` that:
- Is per-step tightened: checks each `deploy_cursor` step in the transcript.
- Is transcript-enumerated: surfaces `schema_deploy_unrecorded` for steps present in the
  cursor but not in the receipt trail.
- Adds M1 stamp/byte WARN: warns when `deploy_plan.steps[i].sha256` diverges from
  `schema_migrations.sha256` for already-applied runtime steps.
Tests: F7, F4, F15 doctor arm.

### Step 7 — Owner bundle 0021 (DDL revoke, deploy-plan-terminal)

Under `go/pkg/db/sql/owner/`, add `0021_revoke_create_privilege.sql` — the DDL that
revokes `CREATE` from `striatumd_rw`. This bundle:
- Is deploy-plan-terminal (always the last step in `BuildPlan`).
- Is excluded from EVERY `owner-ddl apply` route (via `OwnerDDLApplyBundles()` from step 2).
- `LatestOwnerBundleVersion` STAYS 20; `DDLRevokeOwnerBundleVersion = 21` is the new
  constant.

Land the **F16b production-phase test** (asserts the embed/listing split + the forced
FMA-007 self-heal pgtest via `isCrossBundleDependencyError`) and the **forced-self-heal
pgtest** here. Use the two-role fixture (F6, F12, F16b). Activation is the operator
choreography (§4.3) — do NOT self-activate in this run.

## Shadow-first invariants (do not violate)

- `STRIATUM_DEPLOY_DECOUPLED` default OFF; existing `ConnectAndMigrate` path unchanged when
  flag absent.
- `LatestOwnerBundleVersion = 20` (owner.go:23) unchanged.
- `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (= 20, owner.go:35) unchanged.
- Migration 0044 is runtime-owned: no owner DDL, no `owner_bundle_meta` touch.
- Fresh `applied_owner == 0` bootstrap still serves (owner.go:145 unchanged).
- Existing test suites pass unchanged.

## Finding B1 obligations (MUST honor — do not skip)

**B1.1:** `T-deploy-bootpath-decision-table` MUST construct the row-16 in-sync AND
out-of-sync sub-cases for `applied_owner == 0`, `==20`, AND `>=21`, and assert the in-sync
cells serve verify-only WITHOUT firing `ApplyMigrations`/`RecordSchemaFingerprint` spies.

**B1.2:** Table-drive each **concrete** cursor-state enum (`none`/`in_progress`/
`step_committed`/`finalizing`/`complete`/`aborted`), not grouped shorthands.

## Verify before you hand off

Run from `go/`:
```
go build ./... && go vet ./... && golangci-lint run
go test ./pkg/db/... ./pkg/reads/... ./pkg/pgtest/... ./cmd/striatumd/...
```
(per `AGENTS.md`; lint pinned v2.12.2 in `go/Makefile`).

The game-day fire tests (GD-1…GD-12 per §6.5, live two-role cluster) are the
`rfc-0142-p4-verify` run's job, not yours — build the surface so they *can* fire.

## Deliverable

Make the source changes, then publish
**`docs/operator/artifacts/rfc-0142-p4-build/DRAFT.md`** (kind `handoff`) describing:
files changed (one-line role each), how each of the seven build steps + each named F-/G-
test is discharged (map to file:symbol), how B1.1/B1.2 are honored, how each §6.5 criterion
is addressed, and your `go build`/`vet`/`test`/`lint` results. Do not touch `.striatum/` or
`docs/operator/workflows/**`.

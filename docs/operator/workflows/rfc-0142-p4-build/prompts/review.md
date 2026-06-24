# Task — Review: does the build correctly implement RFC 0142 P4?

You are a **fresh-session reviewer**. Read `SEED.md`,
`docs/operator/artifacts/rfc-0142-p4-design-v9/commit/proposal/PROPOSAL.md` (the contract),
the cycle-1 adjudicator ledger (finding B1), the upstream **`DRAFT.md`**, and the **actual
source diff** the draft produced (inspect the changed files in the worktree: `go/pkg/db/`,
`go/pkg/reads/`, `go/pkg/pgtest/`, `go/cmd/striatumd/`). Review the **CODE**, not just the
handoff.

## Check, concretely (against PROPOSAL.md §5 assertions + §6.5 acceptance criteria + finding B1)

### 1. Migration 0044 (BC-N1 / §1.2)
- Does `deploy_cursor` have the singleton CHECK (`id='singleton'`), the correct `state`
  CHECK (must include `finalizing`), `plan_hash`, `state`, `step_index`, `step_id`,
  `updated_at`?
- Does `deploy_plan` have `plan_hash PK`, `steps jsonb`, `revoke_step_index`,
  `base_owner_version`, `base_runtime_version`, `target_*`, INSERT-once
  (`ON CONFLICT (plan_hash) DO NOTHING`)?
- Are both runtime-owned (no owner DDL, no `owner_bundle_meta` touch)?
- Are `striatumd_rw` grants present?

### 2. `owner.go` M2 surface (step 2)
- Is `DDLRevokeOwnerBundleVersion = 21` added? Are `LatestOwnerBundleVersion` and
  `RequiredOwnerBundleVersion` STILL 20 (not advanced)?
- Is `OwnerDDLApplyBundles()` filtering 0021 out? Do apply routes route through it?
- Does the F16a synthetic test exercise the exclusion WITHOUT touching production embed?
- Do existing `owner_pg_test.go` suites pass unchanged?

### 3. `deploy.go` core (step 3)
- Does `BuildPlan` include 0021 as the terminal step (full `OwnerBundles()`)?
- Does `VerifyStoredTranscript` perform full byte-match AND DB-stamp check, returning typed
  errors (`DeployPlanBinaryMismatch`, `DeployPlanDBStampMismatch`)?
- Does `Deployer.Apply` materialize `deploy_plan` + set `in_progress(0)` in one tx BEFORE
  step 0 (BC-N1)?
- Does the finalizer: (0) run `VerifyStoredTranscript` and abort on mismatch, (1) append
  `complete` receipt, (2) `RecordSchemaFingerprint`, (3) advance `finalizing → complete` LAST?
- Does resume call `VerifyStoredTranscript` after loading the stored plan and BEFORE any
  step (M1)?
- Are F1/F2/F4/F8/F9/F10/F12/F13/F14/F15 present and asserting the right things (not
  weakened to pass)?

### 4. `runDaemonDeploy` verb (step 4)
- Is `"deploy"` added to the daemon dispatch? Does it use admin DSN (like `runDaemonOwnerDDL`)?
- Are `--dry-run`/`--abort` flags handled?
- Is the 0021-activation preflight gated on `STRIATUM_DEPLOY_DECOUPLED`?
- Is the authority-guardrail matrix row present?
- Are F3/F5 wiring tests present?

### 5. `CheckDeployActivation` (step 5 — the M3/M5/M6/M7 predicate)
- Is **step 0** (M3 config gate) the FIRST predicate, firing for EVERY cursor state on
  `revokeEmbedded && !decoupledEnabled → awaiting_deploy_config` — DB-untouched?
- Is the **complete/decoupled branch** reading NEITHER `applied_owner` NOR `revokeEmbedded`
  (M7 / §0.2 sub-invariant)?
- Is the **complete/no-revoke branch** reading NEITHER `applied_owner` (M6)?
- Is **`applied_owner == 0` still serving** (M5 / `owner.go:145` unchanged)?
- Is the **`>=21` forward-watermark** rule present (barrier b for no-revoke; tolerate-forward
  to A for revoke-embedding)?
- Are the **typed halts** (`awaiting_deploy`, `awaiting_deploy_config`,
  `deploy_plan_binary_mismatch`, `deploy_plan_db_stamp_mismatch`, `awaiting_owner_ddl`) wired
  into `authority_bootstrap.go:208-227`?
- Is `STRIATUM_DEPLOY_DECOUPLED` default OFF with the existing `ConnectAndMigrate` path
  unchanged?

### 6. Finding B1 (binding — check BOTH obligations)

**B1.1 — F18 row-16 derivation exercise:**
- Does `T-deploy-bootpath-decision-table` construct row-16 **in-sync AND out-of-sync
  sub-cases** for ALL THREE `applied_owner` buckets (`==0`, `==20`, `>=21`)?
- Does the in-sync arm independently set `schema_state.fingerprint == ExpectedFingerprint()`
  AND `cursor.plan_hash == expected` over the appropriate DB (`owner_bundle_meta`-absent / 20 /
  `>=21`) — proving orthogonality?
- Does it assert the in-sync row-16 cells serve verify-only WITHOUT firing
  `ApplyMigrations`/`RecordSchemaFingerprint` spies? (Omitting this recreates M7 in code.)

**B1.2 — Concrete cursor-state enum coverage:**
- Does the table-driven test cover each concrete enum value:
  `none`, `in_progress`, `step_committed`, `finalizing`, `complete`, `aborted`?
- NOT the prose group labels — the enum values from the `state` CHECK.

### 7. `doctor schema_deploy_unrecorded` block (step 6)
- Is the block transcript-enumerated and per-step tightened?
- Does it surface `schema_deploy_unrecorded` for steps in the cursor but not in the receipt
  trail?
- Does it add the M1 stamp/byte WARN?
- Are F7/F4/F15 doctor arms present?

### 8. Owner bundle 0021 (step 7)
- Does `0021_revoke_create_privilege.sql` exist and revoke `CREATE` from `striatumd_rw`?
- Is 0021 excluded from every `owner-ddl apply` route via `OwnerDDLApplyBundles()`?
- Are `LatestOwnerBundleVersion` and `RequiredOwnerBundleVersion` STILL 20?
- Is F16b (production-phase test + forced self-heal) present? Is the two-role pgtest
  extended (F6/F12/F16b)?

### 9. Shadow-first invariants
- New behavior default-OFF behind `STRIATUM_DEPLOY_DECOUPLED` everywhere?
- Migration 0044 strictly additive runtime-owned (no owner DDL)?
- Owner bundle 0021 excluded from all apply routes, `LatestOwnerBundleVersion` STAYS 20?
- Fresh `applied_owner == 0` bootstrap still serves (`owner.go:145` unchanged)?
- Existing serve-boot test suites pass unchanged?

### 10. Build + tests
- Does the tree `go build ./... && go vet ./... && golangci-lint run` clean?
- Do `go test ./pkg/db/... ./pkg/reads/... ./pkg/pgtest/... ./cmd/striatumd/...` pass?
- Is the diff minimal and idiomatic to the surrounding code (reuses existing patterns,
  models on existing oracle functions)?

## §6.5 Acceptance criteria coverage check

Map each of (a)-(l) to a concrete named test or code assertion in the draft. Flag any
criterion with no named test as a defect.

## Verdict

Record a `finding` (the verdict path). Use:
- **`needs_revision`** if any of: a B1.1/B1.2 obligation is missing or weakened; a
  `LatestOwnerBundleVersion`/`RequiredOwnerBundleVersion` was advanced; `STRIATUM_DEPLOY_DECOUPLED`
  is not default OFF; the M7 row-16 derivation is asserted unconditionally; `applied_owner` is
  read on the decoupled complete branch; `revokeEmbedded` is read on the decoupled complete
  branch; a named test is absent/fabricated/weakened; the tree won't build/vet/lint/test;
  migration 0044 touches owner DDL; 0021 is reachable via `owner-ddl apply`. List each
  defect precisely (one revision cycle is available).
- **`accept`** / **`accept_with_findings`** if the build matches the contract, all shadow-first
  invariants hold, B1.1/B1.2 are honored, the named tests pass, and all §6.5 criteria are
  covered (note minor nits as findings).

Write only your review finding artifact at the declared path. Do not modify source.

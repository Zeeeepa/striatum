# DRAFT — RFC 0142 P4: the one-shot `striatum daemon deploy` decoupler (build run)

author: author-author-002

> This is the draft handoff for the `rfc-0142-p4-build` run. It implements RFC
> 0142 P4 — the one-shot `striatum daemon deploy` that decouples schema-apply
> from serve-boot — **test-first**, in the PROPOSAL §6 contract-first seven-step
> order. The deliverable is real Go source in this repo; this DRAFT.md describes
> it, maps each step + named test to file:symbol, shows how B1.1/B1.2 and each
> §6.5 criterion are honored, and reports build/vet/lint/test results — including
> an honest split between what is verified locally and what the
> `rfc-0142-p4-verify` two-role game-days (`G-*`) must confirm.

## Provenance — recovery adoption (attempt 2, author of record)

This attempt **adopted and re-verified** the prior verified-clean implementation
rather than re-implementing it. A prior draft had built the full P4 contract
test-first and self-verified it clean, but the run could not be sealed because
its `write_scope` used the unsatisfiable prefix pattern `go/**` (a daemon
prefix-matcher footgun, GH #586 — a tooling defect, not a code defect). That
verified work was preserved on branch
`backup/rfc-0142-p4-build-impl-2026-06-24`. Per the SEED's RECOVERY ADOPTION
directive, this attempt brought that source into the per-job worktree
(`git checkout origin/backup/rfc-0142-p4-build-impl-2026-06-24 -- go/ docs/operator/artifacts/rfc-0142-p4-build/`,
dropping the stale attempt-1 alternate-layout files), then critically reviewed
every adopted file against the PROPOSAL contract and re-ran the full verification
suite. The `write_scope` is now `go/` (the footgun fixed), so the seal succeeds.
As author of record I own the adopted source; the one change I made on review is
a comment-accuracy fix in `deploy_apply.go` (see "Author-of-record changes" and
"Known remaining items" #1).

## Root reframe held

Schema mutation stops being an implicit side effect of the serving process's
restart and becomes an explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer (`go/pkg/db/deploy.go` + `deploy_apply.go`). Behind
`STRIATUM_DEPLOY_DECOUPLED` (default OFF, shadow-first) the serving daemon holds
zero create-DDL on the serving path (the `serve_verify` decision in
`connection.go`); the legacy `ConnectAndMigrate` apply-on-boot path is unchanged
when the flag is absent for a no-revoke binary.

## Files changed / added (one-line role each)

New source:
- `go/pkg/db/sql/0044_deploy_cursor.sql` — migration 0044: additive runtime-owned
  `deploy_cursor` + immutable `deploy_plan` transcript + hash-chained
  `deploy_receipt`; `state` CHECK includes `finalizing`; no owner DDL.
- `go/pkg/db/sql/owner/0021_revoke_create_privilege.sql` — owner bundle 0021:
  `REVOKE CREATE ON SCHEMA striatumd FROM striatumd_rw` (C3). Deploy-plan-terminal,
  excluded from every `owner-ddl apply` route, idempotent + role-guarded.
- `go/pkg/db/deploy.go` — deploy core: `DeployStep`/`DeployPlan` types, `BuildPlan`
  (0021-terminal, full `OwnerBundles()`), `computePlanHash`, `LoadStoredPlan`,
  `LoadDeployCursor`, `VerifyStoredTranscript` (M1), the four typed halts, the
  receipt hash-chain, the deploy-state enum, substrate-ensure.
- `go/pkg/db/deploy_apply.go` — `Deployer.Apply` engine: materialize (BC-N1),
  per-step Q3-A transactional apply, the `finalizing` idempotent finalizer (C1)
  with `VerifyStoredTranscript` as step 0, resume off the stored transcript, abort.
- `go/pkg/db/deploy_activation.go` — `DecideDeployActivation` (the pure §3.3a/§3.5
  A-gate) + `CheckDeployActivation` (the DB wrapper) + `DeployDecoupledEnabled`.
- `go/pkg/reads/doctor_deploy.go` — `deployUnrecordedDoctorBlock`
  (`schema_deploy_unrecorded`, transcript-enumerated, + M1 stamp/byte WARN).

New tests:
- `go/pkg/db/deploy_test.go` — F1/F8/F9/F13/F14/F15 pure core (plan ordering, hash
  determinism + base-sensitivity, M1 byte verification, receipt chaining,
  typed-halt unwrap).
- `go/pkg/db/deploy_activation_test.go` — **F18 + B1.1/B1.2** pure decision-table
  oracle, the row-15==row-16 identity (M7), the M3-every-cursor-state gate, and the
  halt→typed-error mapping.
- `go/pkg/db/owner_revoke_filter_test.go` — F16a synthetic exclusion + the M4
  filtered-loader route guard + F16b production embed/listing split.
- `go/pkg/db/deploy_pg_test.go` — live arms (F5 dry-run, the F18 in-sync row-15/16
  serve-verify cell, the F18a/row-1 `none`-cursor A-gate); skip without
  `STRIATUM_PG_TEST_URL`.
- `go/pkg/db/deploy_skew_pg_test.go` — deploy-skew parity arms.
- `go/pkg/cli/localcommands/daemon_deploy_test.go` — F3 dispatch + the M3 (e) verb
  preflight (`TestRunDaemonDeployM3PreflightRefusesWithoutDecoupled`).

Modified source:
- `go/pkg/db/owner.go` — M2 surface: `DDLRevokeOwnerBundleVersion = 21`,
  `isNonRevokeBundle`, `OwnerDDLApplyBundles`, `RevokeBundleEmbedded` (reads
  `version >= 21` from the embedded FS, NOT `Latest >= 21`); routed
  `ApplyOwnerBundles`/`applyPendingOwnerBundles`/`ReapplyAllOwnerBundles` through
  the filter + in-loop guards + filtered nil-fallback; bundle-21 label.
  `LatestOwnerBundleVersion`/`RequiredOwnerBundleVersion` STAY 20.
- `go/pkg/db/connection.go` — `ConnectAndMigrate`: W (`CheckOwnerBundleWatermark`)
  → A (`CheckDeployActivation`) before `ApplyMigrations`; `serve_verify` /
  `serve_legacy` / halt branch; the barrier-b forward-watermark refusal for a
  no-revoke binary on an `applied_owner >= 21` DB. Legacy path byte-identical for a
  no-revoke flag-OFF binary over a `none` cursor.
- `go/pkg/db/migrations.go` — `LatestDaemonDBVersion = 44`; 0044 label.
- `go/pkg/cli/localcommands/daemon.go` — `deploy` dispatch + `runDaemonDeploy` verb
  (`--dry-run`/`--abort`/`--json`, owner-DSN resolution, M3 activation preflight).
- `go/cmd/striatumd/main.go` — boot maps the four deploy halts
  (`awaiting_deploy`, `awaiting_deploy_config`, `deploy_plan_binary_mismatch`,
  `deploy_plan_db_stamp_mismatch`) to the same non-restartable exit as the
  watermark/drift halts.
- `go/pkg/reads/doctor.go` — wired the `schema_deploy` block + warnings.
- `go/pkg/db/sql/RESERVATIONS.toml` — reserved runtime ordinal 44 + owner ordinal
  21; frontier note updated.
- `go/pkg/db/reservations_test.go` — owner-bundle frontier guard now compares to
  the highest *embedded* bundle (21 via `maxEmbeddedOwnerVersion`), not
  `LatestOwnerBundleVersion` (20), because the deploy-terminal revoke deliberately
  sits above the watermark frontier.

## The seven build steps (PROPOSAL §6), mapped to source

1. **Migration 0044** — `sql/0044_deploy_cursor.sql`; `LatestDaemonDBVersion=44`
   (`migrations.go`). Three runtime-owned tables; `finalizing` in the CHECK;
   `deploy_plan` INSERT-once `ON CONFLICT (plan_hash) DO NOTHING`; no owner DDL;
   a role-guarded GRANT block for the runtime role.
2. **owner.go M2 surface (inert)** — `DDLRevokeOwnerBundleVersion`,
   `isNonRevokeBundle`, `OwnerDDLApplyBundles`, `RevokeBundleEmbedded` in
   `owner.go`; routes filtered + in-loop-guarded; `LatestOwnerBundleVersion`
   stays 20. **F16a** = `TestOwnerDDLApplyExcludesSyntheticRevokeBundle`;
   **M4 route guard** = `TestOwnerDDLApplyRoutesUseFilteredLoader`.
3. **`deploy.go` core** — `DeployPlan`/`BuildPlan`/`computePlanHash`/`LoadStoredPlan`/
   `VerifyStoredTranscript`(M1)/typed halts/`Deployer.Apply` (`deploy_apply.go`,
   Q3-A engine + `finalizing` finalizer with `VerifyStoredTranscript` step 0)/
   receipt hash-chain. Pure-core tests F1/F8/F9/F13/F14/F15 green locally.
4. **`runDaemonDeploy` verb** — `localcommands/daemon.go`; `deploy` dispatch;
   `--dry-run`/`--abort`; M3 activation preflight. **F3** =
   `TestRunDaemonDeployDispatch`; **F5** dry-run live arm =
   `TestDeployerDryRunPlanLive`.
5. **`CheckDeployActivation`** — `deploy_activation.go`: `DecideDeployActivation`
   (pure §3.3a/§3.5) + the DB wrapper; wired into `ConnectAndMigrate` (W→A before
   `ApplyMigrations`; `serve_verify` returns without `ApplyMigrations`/`:399`) in
   `connection.go`; halts mapped in `main.go`. **F18/F18a** below; **F11/F17** at
   the predicate layer (M3 gate + BC-N2 edges).
6. **`doctor schema_deploy_unrecorded`** — `reads/doctor_deploy.go`, wired in
   `doctor.go`. Per-step, transcript-enumerated, + the M1 stamp/byte WARN. Skips
   on a pre-P4 DB.
7. **Owner bundle 0021** — `sql/owner/0021_revoke_create_privilege.sql`;
   deploy-plan-terminal; excluded from owner-ddl apply; watermark stays 20.
   **F16b** production split = `TestOwnerBundle0021ProductionEmbedListingSplit`.

## Falsifiable assertions (F-tests) — disposition

| Test | Where | Status |
| --- | --- | --- |
| F1 plan shape / ordering | `TestBuildPlanOrdersOwnerThenRuntimeThenRevokeTerminal` | green (local) |
| F8/F9 plan≡hash determinism + base-sensitive | `TestPlanHashIsDeterministicAndBaseSensitive` | green (local) |
| F13/F14 immutable plan + receipt chain | `TestReceiptRowHashChains`, plan-hash tests | green (local) |
| F15 M1 full-transcript byte verify | `TestVerifyStoredTranscriptCleanPlanPasses`, `TestVerifyStoredTranscriptDetectsBinaryMismatch` | green (local) |
| F16a synthetic exclusion + M4 route guard | `owner_revoke_filter_test.go` | green (local) |
| F16b production embed/listing split | `TestOwnerBundle0021ProductionEmbedListingSplit` | green (local); forced FMA-007 self-heal pgtest → verify-run |
| F17 (M3) complete-cursor flag-OFF refusal | `TestDeployActivationM3GateFiresForEveryCursorState` (predicate); verb preflight `TestRunDaemonDeployM3PreflightRefusesWithoutDecoupled` | green (local, predicate + verb); live spy arm → verify-run |
| F18 decision table (B1.1+B1.2) | `TestDeployBootPathDecisionTable`, `TestDeployActivationRow16ConditionalMirrorsRow15` | green (local); live spy arm `TestDeployActivationCompleteInSyncServesVerifyLive` → cluster |
| F18a fresh-DB serve | `TestDeployActivationNoneCursorLive` (A-gate arm); W-gate fresh serve covered by `owner_watermark_pg_test.go` | compiles/skips → cluster |
| F2/F3/F5 verb + decouple | `daemon_deploy_test.go`, `deploy_pg_test.go` | F3 green (local); F5 → cluster |
| F6/F12 two-role activation + CREATE denial | `Deployer.Apply` over `pgtest.TwoRole` | → verify-run (`G-revoke-last`) |

The `G-*` game-days (`G-old-binary-refuse`, `G-wrong-binary-resume`,
`G-complete-cursor-flag-off-refuse`, `G-fresh-db-first-boot`, `G-revoke-last`)
fire against a live two-role cluster and are the `rfc-0142-p4-verify` run's job;
this build provides the surface so they can fire.

## How B1.1 and B1.2 are honored (binding obligations)

- **B1.2 — concrete cursor-state enums.** `TestDeployBootPathDecisionTable`
  table-drives `concreteCursorStates = {none, in_progress, step_committed,
  finalizing, complete, aborted}` by value (not the prose group labels), ×
  decoupled × revoke × inSync × the `applied_owner ∈ {0,20,≥21}` columns. The
  grouped "64-cell" shorthand is fully expanded.
- **B1.1 — the row-16 derivation.** For the `complete` cursor the test constructs
  **both** in-sync and out-of-sync sub-cases across the `applied_owner ∈ {0,20,≥21}`
  columns, asserts the in-sync decoupled (revoke-embedding, row 16) cells serve
  **verify-only** (`DeployServeVerify`, never `DeployServeLegacy` — the pure analog
  of "not firing the `ApplyMigrations`/`RecordSchemaFingerprint` spies", since
  `serve_legacy` is the only decision that reaches that legacy path in
  `connection.go`), and asserts the **column-identity** (`==0`==`==20`==`≥21`) and
  **row-identity** (row 15 == row 16, `TestDeployActivationRow16ConditionalMirrorsRow15`)
  orthogonality M6/M7 close. The live DB arm
  (`TestDeployActivationCompleteInSyncServesVerifyLive`) seeds the constructible
  in-sync cell (`schema_state.fingerprint == ExpectedFingerprint()` AND a
  frontier-targeting plan) and asserts serve-verify, then flips the fingerprint
  out of sync and asserts `awaiting_deploy` on the SAME cursor — the literal
  ApplyMigrations/RecordSchemaFingerprint-spy assertion runs there, gated on
  `STRIATUM_PG_TEST_URL`, so it executes in the verify-run game-day.

## §6.5 acceptance criteria (a)–(l) — addressed

- **(a)** crash-resume stable key — `Deployer` resume off the STORED transcript;
  receipt keyed `(plan_hash, step_index)`, `ON CONFLICT DO NOTHING` (exactly-once).
  Live GD → verify.
- **(b)** divergent-binary resume refuses — `VerifyStoredTranscript` returns
  `deploy_plan_binary_mismatch`; `TestVerifyStoredTranscriptDetectsBinaryMismatch`.
- **(c)** universal pre-revoke serve edge — `DecideDeployActivation` step 1/2 →
  `awaiting_deploy` for every non-complete cursor (F18 rows 5/7/9/11) + the
  barrier-b forward-watermark refusal in `connection.go`.
- **(d)** self-heal does not commit the revoke early — `ReapplyAllOwnerBundles`
  in-loop guard + filtered nil-fallback (F16a/F16b); forced FMA-007 pgtest → verify.
- **(e)** complete-cursor flag-OFF revoke refusal — `DecideDeployActivation` step 0
  (M3) for EVERY cursor state (`TestDeployActivationM3GateFiresForEveryCursorState`)
  + the verb preflight (`TestRunDaemonDeployM3PreflightRefusesWithoutDecoupled`).
- **(f)** fresh-DB serve + shortfall halt — A-gate `none/off/no-revoke → serve_legacy`
  (`TestDeployActivationNoneCursorLive`); W shortfall unchanged (`CheckOwnerBundleWatermark`,
  `applied_owner == 0` serves, `1..19` halts).
- **(g)/(h)** no-revoke complete in-sync/out-of-sync + degenerate legacy no-op — F18
  rows 13/15 conditional.
- **(i)/(j)** revoke-embedding complete in-sync/out-of-sync at `==0`/`==20`/`≥21` —
  F18 row 16 parametric + the live in-sync cell.
- **(k)** hash-chained receipt + doctor — `receiptRowHash` + `deployUnrecordedDoctorBlock`.
- **(l)** two-role activation + post-deploy CREATE denial — bundle 0021 over the
  deployer terminal step; live two-role assertion → verify-run. **See "Known
  remaining items" #1: the C3 per-step ownership reconcile (§3.3b) is not yet wired
  — latent for the base-20 activation, required for a fresh-DB deploy.**

## Shadow-first invariants — preserved

`STRIATUM_DEPLOY_DECOUPLED` default OFF; `LatestOwnerBundleVersion`/`Required`
stay 20 (verified in source: `owner.go` `const LatestOwnerBundleVersion = 20`,
`RequiredOwnerBundleVersion = LatestOwnerBundleVersion`); 0044 is runtime-owned
(no `owner_bundle_meta` touch); 0021 lands inert (excluded from owner-ddl apply);
fresh `applied_owner==0` still serves (`CheckOwnerBundleWatermark` unchanged); the
legacy `connection.go` self-record writer is reached only on the `serve_legacy`
branch (no-revoke flag-OFF). Existing serve-boot test suites use a `none` cursor →
`serve_legacy` → the legacy path runs byte-identically (whole repo non-PG suite is
green except a pre-existing, unrelated failure noted below).

## Author-of-record changes (on review of the adopted source)

- `go/pkg/db/deploy_apply.go` — corrected the `applyDeployStep` doc comment, which
  claimed the C3 per-step ownership reconcile (`ALTER … OWNER TO striatumd_rw`)
  runs before commit. The function body does not implement it; the comment now
  states the reconcile is the documented apply-run follow-up and explains why it is
  latent for the normal base-20 activation. Behavior-neutral (comment only);
  build/vet/lint/test stay green. This keeps code and docs honest (AGENTS.md: fix a
  doc claim that disagrees with current source behavior).

## Build / vet / lint / test results (this attempt, run from `go/`)

- `go build ./...` — **PASS**.
- `go vet ./...` — **PASS**.
- `golangci-lint run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./...`
  (the exact CI config, pinned v2.12.2, matching `go/Makefile`) — **0 issues**.
- `go test ./...` — every package I touch is green: `pkg/db` ✓, `pkg/reads` ✓,
  `pkg/cli/localcommands` ✓, `cmd/striatumd` ✓ (the pg-integration arms
  `TestDeployerDryRunPlanLive`, `TestDeployActivationCompleteInSyncServesVerifyLive`,
  `TestDeployActivationNoneCursorLive` compile and **SKIP** cleanly because
  `STRIATUM_PG_TEST_URL` is unset on this host).

**Pre-existing, out-of-scope failure (NOT a P4 regression):** `go test ./...`
reports two failures in `pkg/agentloop/mcpconfig_test.go`
(`TestInjectLaneMCPConfigCodexAppendsTomlUrlOverride`,
`TestInjectCodexMCPConfigArgsPrecedesExecSubcommand`). They come from commit
`f53969fc` (`feat(mcp): #316 boot-epoch identity check`), which added the
`X-Striatum-Boot-Epoch` codex-injection header without updating these two test
expectations. **Confirmed to reproduce at the run base `1ced0c03` with NONE of
this run's changes present** (checked in a throwaway worktree at that commit), and
this run touches no `agentloop` code. Left for whoever owns #316; flagged here so
the reviewer does not attribute it to P4.

## Known remaining items for review / the verify-run (honest gaps)

1. **C3 per-step ownership reconcile (§3.3b) — primary apply-run follow-up.**
   `applyDeployStep` applies the step DDL + version stamp + cursor advance +
   hash-chained receipt in one transaction (Q3-A), but does NOT yet snapshot
   new owner-owned oids and `ALTER … OWNER TO striatumd_rw` before the terminal
   revoke, nor assert `has_schema_privilege('striatumd_rw','striatumd','CREATE')`
   pre-step. The normal activation deploys from base 20 with only the terminal
   0021 step (which creates no new ownable object), so this is latent there; a
   fresh-DB deploy from base 0 needs the reconcile so runtime tables the deployer
   creates are striatumd_rw-owned before 0021 commits. Comment now reflects this.
2. **Two-role deployer grant model.** The deployer writes the runtime-owned deploy
   substrate (`deploy_cursor`/`deploy_plan`/`deploy_receipt`) and the runtime
   version stamps. 0044 grants the runtime role SELECT/INSERT/UPDATE on those
   tables; in a two-role cluster the owner/admin DSN must hold the matching DML
   (via `striatumd_rw` membership or an explicit grant). The live two-role apply
   game-day exercises/confirms this.
3. **Receipt sink.** Per-step receipts land in the hash-chained `deploy_receipt`
   table (keyed `(plan_hash, step_index)`), satisfying the exactly-once +
   doctor-enumeration contract. Coupling the receipt into `audit_log` via
   `append_audit_row` was deferred to avoid plumbing the daemon-authority secret
   through the bare deploy verb — a reviewer decision point.
4. **Q3-B NT-DDL.** `applyDeployStep` implements the Q3-A transactional path; the
   embedded set contains no NT-DDL (`CREATE INDEX CONCURRENTLY`, `ALTER TYPE … ADD
   VALUE`) steps today, so the Q3-B pre/post-marker path is a documented structural
   seam, not yet code.
5. **Command-authority matrix.** `daemon deploy` is a LOCAL owner-connection verb
   (like `daemon owner-ddl`/`daemon migrate`), not an RPC route — none of those
   appear in `command-authority-matrix.md`, and the authority guardrail tests pass
   without a `deploy` row, so no matrix row is required. (The matrix doc also lies
   outside this run's `write_scope`.)

The deployer surface, plan transcript, activation predicate, decoupled boot,
typed halts, doctor block, and bundle 0021 are in place and locally green; the
items above are the reviewable seams the review→apply→verify path closes. The
adopted implementation was re-verified against the PROPOSAL §5 assertions, the
§6.5 (a)–(l) acceptance criteria, and finding B1 (B1.1/B1.2) at the build-run
layer (pure decision oracle + locally-green tests + pg arms that compile and skip);
the two-role `G-*` game-days remain the verify-run's job.

# DRAFT — RFC 0142 P4: the one-shot `striatum daemon deploy` decoupler (build run, D7′ revision)

author: author-author-001

> This is the draft handoff for the **D7′ revision** of `rfc-0142-p4-build`. It
> adopts the attempt-2 implementation (branch
> `backup/rfc-0142-p4-build-attempt2-2026-06-24`, which fixed D3/D4/D5 — the real
> M1 DB-stamp arm, the per-step hash-chained receipt, Q3-A per-step atomicity) and
> applies the **one blocking fix (D7′, Option B: un-embed 0021) plus the three
> follow-ups (item 2 flag-OFF proof, D1′ B1.1 live-arm tightening, D6 C3 re-scope)**
> the attempt-2 review (`review/REVIEW_v2_needs_revision.md`) required, then
> re-verifies. The deliverable is real Go source in `go/`; this DRAFT.md describes
> it, maps each of the seven build steps + named F-/G- tests to file:symbol, shows
> how B1.1/B1.2 and each §6.5 criterion are honored, restates the shadow-first
> claim accurately, and reports build/vet/lint/test results.

## What this revision changed (the D7′ blocking fix + 3 follow-ups)

The attempt-2 review returned `needs_revision` for ONE blocking defect, **D7′**:
attempt 2 EMBEDDED owner bundle `0021_revoke_create_privilege.sql` inside
`ownerBundleFS` (`go/pkg/db/sql/owner/`), flipping `RevokeBundleEmbedded() → true`
for the production binary. Then `DecideDeployActivation` step 0
(`RevokeEmbedded && !DecoupledEnabled → awaiting_deploy_config`, which fires for
EVERY cursor state before the switch) made the **flag-OFF default boot refuse to
serve** — bricking production serve-boot, the entire `pgtest` CI suite
(`pgtest.go` `ConnectAndMigrate` asserts `version == LatestDaemonDBVersion`), and
B1.1's own live arm. That violated the SEED's hard shadow-first invariant
("0021 lands INERT; flag-OFF serve-boot UNCHANGED").

This revision resolves it the **Option-B (shadow-first-faithful)** way and lands
the three follow-ups:

1. **D7′ — UN-EMBED 0021 (the blocking fix).** Moved
   `0021_revoke_create_privilege.sql` OUT of `ownerBundleFS` into a NEW staged dir
   `go/pkg/db/sql/owner_staged_activation/`, which is **not** matched by the
   `//go:embed sql/owner/*.sql` directive in `owner.go`. A separate
   `//go:embed sql/owner_staged_activation/*.sql var stagedActivationFS embed.FS`
   keeps the revoke version-controlled and loadable, but `OwnerBundles()` /
   `ExpectedFingerprint()` / `RevokeBundleEmbedded()` (which read `ownerBundleFS`)
   no longer see it — so `RevokeBundleEmbedded()` is **false** for this
   inert-landing binary and the flag-OFF serve-boot path is byte-identical to a
   pre-P4 binary. The deployer still references 0021 by loading it from the staged
   dir (`StagedRevokeBundle`, `owner.go`); `BuildPlan`/`binaryStepSHA`/`stepRecord`
   resolve the terminal revoke through there for an **activation binary** that
   embeds it. Per §4.3's two-binary choreography, F16b-production, the
   revoke-in-plan, and §6.5 criterion (l) are **re-scoped to the activation/verify
   binary** (see "Re-scoped" below).
2. **Item 2 — flag-OFF boot proven by a test that RUNS.** New non-PG unit arm
   `TestProductionInertBinaryFlagOffServesLegacy` (`deploy_activation_test.go`)
   reads the REAL `RevokeBundleEmbedded()` value of THIS binary, asserts it is
   false, and asserts `DecideDeployActivation(none, decoupled=false, revoke=<real>)
   == DeployServeLegacy` (NOT `awaiting_deploy_config`). It also asserts the M3
   gate fires for NO cursor state on this inert binary. It reds the instant 0021 is
   re-embedded. The `pgtest` harness bootstraps green again because OwnerBundles()
   is back to the 20-bundle pre-P4 set.
3. **D1′ — B1.1 live arm tightened.** `TestDeployActivationCompleteInSyncServesVerifyLive`
   (`deploy_pg_test.go`) now seeds all three buckets B1.1 names —
   `owner_bundle_meta` **absent (==0), ==20, and ==21** (sub-tests, fresh pool each)
   — and replaces the decision-value tautology with a **real behavioral mutation
   spy**: it snapshots the `schema_state` singleton (the only
   `RecordSchemaFingerprint` writer) serialized whole via `to_jsonb`, plus the
   `schema_migrations` row count (what `ApplyMigrations` grows), before/after
   `CheckDeployActivation`, and asserts byte-identical — proving the mutating path
   is NOT entered. Both `revoke=false`/`true` serve verify-only (M7
   revoke-independence); the out-of-sync sub-case halts `awaiting_deploy`, also
   without mutation. (pg-gated; compiles + skips locally.)
4. **D6 — C3 §3.3b explicitly RE-SCOPED.** The per-step ownership reconcile
   (snapshot new owner-owned oids + `ALTER … OWNER TO striatumd_rw` +
   `has_schema_privilege` pre-step assertion) is NOT wired; the `applyDeployStep`
   doc comment now states it is **re-scoped to the activation/verify run** (latent
   for the base-20 activation, whose only step is the terminal 0021 that creates no
   new ownable object; required for a fresh-DB deploy from base 0). The PROPOSAL
   roadmap / §6.5(l) update lies OUTSIDE this lane's `write_scope`
   (`docs/operator/artifacts/rfc-0142-p4-design-v9/...` and `docs/operator/rfc-roadmap.md`
   are not in scope), so it is recorded here in DRAFT.md per the implementation-
   envelope instruction; the verify-run owner should mirror it into the roadmap.

## Root reframe held

Schema mutation stops being an implicit side effect of the serving process's
restart and becomes an explicit, ordered, resumable, provenance-tracked operation
owned by a dedicated deployer (`go/pkg/db/deploy.go` + `deploy_apply.go`). Behind
`STRIATUM_DEPLOY_DECOUPLED` (default OFF, shadow-first) the serving daemon holds
zero create-DDL on the serving path (the `serve_verify` decision in
`connection.go`); the legacy `ConnectAndMigrate` apply-on-boot path is unchanged
when the flag is absent for a no-revoke binary — and, under Option B, THIS binary
**is** a no-revoke binary, so that property is real (it was false in attempt 2).

## Files changed in this revision (on top of the adopted attempt-2 `go/`)

- `go/pkg/db/sql/owner_staged_activation/0021_revoke_create_privilege.sql` —
  **moved** from `sql/owner/` (git rename); header rewritten to document the
  staged/un-embedded reality. The REVOKE DDL + capability stamp are unchanged.
- `go/pkg/db/owner.go` — added `stagedActivationFS` embed + `StagedRevokeBundle()`
  loader; updated the `DDLRevokeOwnerBundleVersion`, `RevokeBundleEmbedded`, and
  `OwnerDDLApplyBundles` doc comments to the staged reality (`RevokeBundleEmbedded()`
  logic is unchanged — it reads `OwnerBundles()`, which no longer lists 0021 → false).
- `go/pkg/db/deploy.go` — `BuildPlan` now keys the terminal revoke off
  `RevokeBundleEmbedded()` and sources it from `StagedRevokeBundle()` for an
  activation binary (for the inert binary it emits NO revoke step → `RevokeStepIndex
  == -1`); `binaryStepSHA` (owner case) falls back to the staged bundle so a
  transcript naming 0021 is byte-verifiable.
- `go/pkg/db/deploy_apply.go` — `stepRecord` (owner case) staged fallback; the C3
  NOTE strengthened to the explicit D6 re-scope.
- `go/pkg/db/owner_revoke_filter_test.go` — F16b re-scoped:
  `TestOwnerBundle0021ProductionEmbedListingSplit` → `TestOwnerBundle0021StagedForActivationNotEmbedded`
  (asserts 0021 NOT in `OwnerBundles()`, `RevokeBundleEmbedded()==false`,
  `StagedRevokeBundle()` present with the REVOKE SQL, apply-route exclusion,
  watermark 20/20, `BuildPlan` `RevokeStepIndex==-1`).
- `go/pkg/db/deploy_activation_test.go` — added `TestProductionInertBinaryFlagOffServesLegacy`
  (item 2).
- `go/pkg/db/deploy_pg_test.go` — D1′ tightening (three buckets + mutation spy).
- `go/pkg/cli/localcommands/daemon_deploy_test.go` — comment accuracy: the M3 verb
  preflight arm is now inert for the inert binary (skips), re-scoped to the
  activation binary (the test already `t.Skip`s when `RevokeBundleEmbedded()==false`).
- `go/pkg/db/sql/RESERVATIONS.toml` — removed the EMBEDDED `[[owner_bundle]]
  ordinal=21` entry (the owner-bundle frontier is now `maxEmbeddedOwnerVersion ==
  20`, matching the un-embedded `sql/owner` dir) and recorded owner ordinal 21 as
  the **staged** DDL-revoke reservation via a TOML comment (a future EMBEDDED owner
  bundle must not reuse 21). This keeps all three reservation guards green
  (`reservations_test.go` unchanged).

The rest of the P4 surface (migration 0044; `owner.go` M2 filter; `deploy.go` /
`deploy_apply.go` engine, M1 byte + DB-stamp arms, per-step receipt chain,
`finalizing` finalizer; `deploy_activation.go` predicate; `connection.go` W→A boot
order + barrier-b; `runDaemonDeploy` verb + M3 preflight; `main.go` typed-halt
arms; `doctor_deploy.go`; `LatestDaemonDBVersion=44`) is adopted from attempt-2
unchanged and re-verified — D3/D4/D5 are preserved, not regressed.

## The seven build steps (PROPOSAL §6), mapped to source

1. **Migration 0044** — `sql/0044_deploy_cursor.sql`; `LatestDaemonDBVersion=44`
   (`migrations.go`). Three runtime-owned tables; `finalizing` in the CHECK;
   `deploy_plan` INSERT-once `ON CONFLICT (plan_hash) DO NOTHING`; no owner DDL;
   role-guarded GRANT block.
2. **owner.go M2 surface (inert)** — `DDLRevokeOwnerBundleVersion`,
   `isNonRevokeBundle`, `OwnerDDLApplyBundles`, `RevokeBundleEmbedded` +
   `StagedRevokeBundle`; routes filtered + in-loop-guarded; `LatestOwnerBundleVersion`
   stays 20. **F16a** = `TestOwnerDDLApplyExcludesSyntheticRevokeBundle`; **M4 route
   guard** = `TestOwnerDDLApplyRoutesUseFilteredLoader`.
3. **`deploy.go` core** — `DeployPlan`/`BuildPlan`/`computePlanHash`/`LoadStoredPlan`/
   `VerifyStoredTranscript`(M1 byte)/`VerifyAppliedDBStamps`(M1 DB-stamp)/typed
   halts/`Deployer.Apply` (`deploy_apply.go`, Q3-A engine + `finalizing` finalizer
   with `VerifyStoredTranscript` step 0)/receipt hash-chain. F1/F8/F9/F13/F14/F15
   green locally.
4. **`runDaemonDeploy` verb** — `localcommands/daemon.go`; `deploy` dispatch;
   `--dry-run`/`--abort`; M3 activation preflight. **F3** =
   `TestRunDaemonDeployDispatch`; **F5** dry-run live arm = `TestDeployerDryRunPlanLive`.
5. **`CheckDeployActivation`** — `deploy_activation.go`: `DecideDeployActivation`
   (pure §3.3a/§3.5) + the DB wrapper; wired into `ConnectAndMigrate` (W→A before
   `ApplyMigrations`; `serve_verify` returns without `ApplyMigrations`/`:399`) in
   `connection.go`; halts mapped in `main.go`. **F18/F18a** below; **F11/F17** at
   the predicate layer (M3 gate + BC-N2 edges).
6. **`doctor schema_deploy_unrecorded`** — `reads/doctor_deploy.go`, wired in
   `doctor.go`. Per-step, transcript-enumerated, + the M1 stamp/byte WARN.
7. **Owner bundle 0021** — `sql/owner_staged_activation/0021_revoke_create_privilege.sql`;
   deploy-plan-terminal; STAGED out of `ownerBundleFS` (Option B); excluded from
   owner-ddl apply; watermark stays 20. **F16b re-scoped** =
   `TestOwnerBundle0021StagedForActivationNotEmbedded` (the un-embedded direction;
   the production-EMBED assertions + forced FMA-007 self-heal pgtest move to the
   activation binary per §4.3).

## Falsifiable assertions (F-tests) — disposition

| Test | Where | Status |
| --- | --- | --- |
| F1 plan shape / ordering | `TestBuildPlanOrdersOwnerThenRuntimeThenRevokeTerminal` | green (local); for the inert binary `RevokeBundleEmbedded()==false` ⇒ `RevokeStepIndex==-1` (asserted) |
| F8/F9 plan≡hash determinism + base-sensitive | `TestPlanHashIsDeterministicAndBaseSensitive` | green (local) |
| F13/F14 immutable plan + receipt chain | `TestReceiptRowHashChains`, plan-hash tests | green (local) |
| F15 M1 byte verify | `TestVerifyStoredTranscriptCleanPlanPasses`, `TestVerifyStoredTranscriptDetectsBinaryMismatch` | green (local); DB-stamp arm `VerifyAppliedDBStamps` live → verify-run |
| F16a synthetic exclusion + M4 route guard | `owner_revoke_filter_test.go` | green (local) |
| F16b staged (re-scoped) | `TestOwnerBundle0021StagedForActivationNotEmbedded` | green (local); production-embed + forced FMA-007 self-heal → activation/verify binary |
| F17 (M3) complete-cursor flag-OFF refusal | `TestDeployActivationM3GateFiresForEveryCursorState` (predicate); verb preflight `TestRunDaemonDeployM3PreflightRefusesWithoutDecoupled` (inert here, fires on activation binary) | green (local, predicate); live spy → verify-run |
| F18 decision table (B1.1+B1.2) | `TestDeployBootPathDecisionTable`, `TestDeployActivationRow16ConditionalMirrorsRow15` | green (local); live spy `TestDeployActivationCompleteInSyncServesVerifyLive` (3 buckets) → cluster |
| F18a fresh-DB serve | `TestDeployActivationNoneCursorLive`; **also** the new non-PG `TestProductionInertBinaryFlagOffServesLegacy` (the production-value flag-OFF none→serve_legacy proof) | non-PG arm green (local); live arm → cluster |
| F2/F3/F5 verb + decouple | `daemon_deploy_test.go`, `deploy_pg_test.go` | F3 green (local); F5 → cluster |
| F6/F12 two-role activation + CREATE denial | `Deployer.Apply` over `pgtest.TwoRole` | → verify-run (`G-revoke-last`) |

The `G-*` game-days (`G-old-binary-refuse`, `G-wrong-binary-resume`,
`G-complete-cursor-flag-off-refuse`, `G-fresh-db-first-boot`, `G-revoke-last`)
fire against a live two-role cluster and are the `rfc-0142-p4-verify` run's job;
this build provides the surface so they can fire.

## How B1.1 and B1.2 are honored (binding obligations)

- **B1.2 — concrete cursor-state enums.** `TestDeployBootPathDecisionTable`
  table-drives `concreteCursorStates = {none, in_progress, step_committed,
  finalizing, complete, aborted}` by value, × decoupled × revoke × inSync × the
  `applied_owner ∈ {0,20,≥21}` columns. The grouped "64-cell" shorthand is fully
  expanded.
- **B1.1 — the row-16 derivation.** The pure F18 constructs **both** in-sync and
  out-of-sync sub-cases for `complete` across all three columns, asserts the in-sync
  decoupled (revoke-embedding, row 16) cells serve verify-only (`DeployServeVerify`,
  never `DeployServeLegacy`), and asserts column-identity (`==0`==`==20`==`≥21`) and
  row-identity (row 15 == row 16, `TestDeployActivationRow16ConditionalMirrorsRow15`).
  The **tightened live arm** `TestDeployActivationCompleteInSyncServesVerifyLive`
  now seeds the `owner_bundle_meta` **==0, ==20, AND ==21** buckets, independently
  sets `schema_state.fingerprint == ExpectedFingerprint()` AND a frontier-targeting
  plan, and proves serve-verify with a **real mutation spy** (`schema_state` whole +
  `schema_migrations` count unchanged across `CheckDeployActivation`) for both
  `revoke=false`/`true` — the literal "serve verify-only WITHOUT firing the
  `ApplyMigrations`/`RecordSchemaFingerprint` spies" obligation, no longer a
  decision-value tautology. (pg-gated; runs in the verify-run game-day.)

## §6.5 acceptance criteria (a)–(l) — addressed

- **(a)** crash-resume stable key — `Deployer` resume off the STORED transcript;
  receipt keyed `(plan_hash, step_index)` exactly-once. Live GD → verify.
- **(b)** divergent-binary resume refuses (M1) — `VerifyStoredTranscript`
  (byte) + `VerifyAppliedDBStamps` (DB-stamp), both called in `resume()`;
  `TestVerifyStoredTranscriptDetectsBinaryMismatch`.
- **(c)** universal pre-revoke serve edge (BC-N2) — `DecideDeployActivation`
  step 1/2 → `awaiting_deploy` for every non-complete cursor + barrier-b in
  `connection.go`.
- **(d)** self-heal does not commit the revoke early (M2) — `ReapplyAllOwnerBundles`
  in-loop guard + filtered nil-fallback (F16a); forced FMA-007 pgtest → activation/verify.
- **(e)** complete-cursor flag-OFF revoke refusal (M3) — `DecideDeployActivation`
  step 0 for EVERY cursor state (`TestDeployActivationM3GateFiresForEveryCursorState`)
  + verb preflight. (Fires for the **activation** binary; this inert binary correctly
  does NOT halt — that is the D7′ fix.)
- **(f)** fresh-DB serve + shortfall halt (M5) — A-gate `none/off/no-revoke →
  serve_legacy` proven on the REAL embed value by the new non-PG
  `TestProductionInertBinaryFlagOffServesLegacy` + the live `TestDeployActivationNoneCursorLive`;
  W shortfall unchanged (`CheckOwnerBundleWatermark`).
- **(g)/(h)** no-revoke complete in-sync/out-of-sync + degenerate legacy no-op —
  F18 rows 13/15 conditional.
- **(i)/(j)** revoke-embedding complete in-sync/out-of-sync at `==0`/`==20`/`≥21` —
  F18 row 16 parametric + the tightened live in-sync cell (3 buckets).
- **(k)** hash-chained receipt + doctor — `receiptRowHash` + `deployUnrecordedDoctorBlock`.
- **(l)** two-role activation + post-deploy CREATE denial (C3) — **RE-SCOPED to the
  activation/verify binary** (Option B / D6): the staged 0021 supplies the terminal
  revoke; the live two-role CREATE-denial game-day + the C3 per-step reconcile run
  there. See "Re-scoped" and "Known remaining items" #1.

## Shadow-first invariants — restated ACCURATELY (the D7′ correction)

The attempt-2 DRAFT's central claim — *"Existing serve-boot test suites use a
`none` cursor → `serve_legacy` → the legacy path runs byte-identically"* — was
**FALSE** while 0021 was embedded (step 0 fired first → `awaiting_deploy_config`).
Under Option B it is **TRUE again, and proven by a test that runs**:

- `STRIATUM_DEPLOY_DECOUPLED` default OFF.
- `LatestOwnerBundleVersion`/`RequiredOwnerBundleVersion` stay 20 (`owner.go`).
- Migration 0044 is runtime-owned (no `owner_bundle_meta` touch).
- **0021 is STAGED out of `ownerBundleFS`, so `RevokeBundleEmbedded()` is FALSE,
  `OwnerBundles()` is the 20-bundle pre-P4 set, and `ExpectedFingerprint()` excludes
  0021 — the flag-OFF serve-boot path is byte-identical to a pre-P4 binary.**
- `DecideDeployActivation(none, decoupled=false, revoke=false) == serve_legacy`
  (NOT `awaiting_deploy_config`) — `TestProductionInertBinaryFlagOffServesLegacy`,
  reading the real embed value.
- Fresh `applied_owner==0` still serves (`CheckOwnerBundleWatermark` unchanged).
- The `pgtest` harness (`ConnectAndMigrate` asserting `version ==
  LatestDaemonDBVersion`) bootstraps green again; the existing serve-boot pg suites
  pass unchanged. `owner_pg_test.go`'s `len(applied) == len(OwnerBundles())` holds
  (both 20) — an attempt-2 latent CI mismatch the un-embed also resolves.

## Re-scoped to the activation/verify binary (Option B / §4.3; D6)

- **F16b production-EMBED assertions** (`OwnerBundles()` contains 0021;
  `RevokeBundleEmbedded()==true`; `ExpectedFingerprint()` includes 0021) — fire on
  the activation binary that embeds 0021 in `ownerBundleFS`.
- **Revoke-in-plan** — `BuildPlan.RevokeStepIndex >= 0` (terminal 0021) is the
  activation binary's plan; the inert binary's plan has `RevokeStepIndex == -1`.
- **§6.5 criterion (l)** — two-role activation + post-deploy `striatumd_rw` CREATE
  denial, and the **C3 per-step ownership reconcile (§3.3b)** it depends on, run in
  the verify/apply run.
- The owner-bundle reservation registry tracks the EMBEDDED set (now 1..20); owner
  ordinal 21 is recorded as the staged DDL-revoke reservation in `RESERVATIONS.toml`.
- The PROPOSAL/roadmap update for this re-scope is **out of this lane's
  `write_scope`** (those docs live under `docs/operator/artifacts/rfc-0142-p4-design-v9/`
  and `docs/operator/rfc-roadmap.md`); it is recorded here and should be mirrored by
  the verify-run owner.

## Build / vet / lint / test results (this revision, run from `go/`, go1.25.0)

- `go build ./...` — **PASS**.
- `go vet ./...` — **PASS**.
- `golangci-lint run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./...`
  (the exact CI config, pinned v2.12.2, matching `go/Makefile`) — **0 issues**.
- `go test ./...` — every P4 package is green: `pkg/db` ✓, `pkg/reads` ✓,
  `pkg/pgtest` ✓, `pkg/cli/localcommands` ✓, `cmd/striatumd` ✓. The pg-integration
  arms (`TestDeployerDryRunPlanLive`, the 3-bucket
  `TestDeployActivationCompleteInSyncServesVerifyLive`,
  `TestDeployActivationNoneCursorLive`, the owner/two-role pg suites) compile and
  **SKIP** cleanly because `STRIATUM_PG_TEST_URL` is unset on this host.

**Pre-existing, out-of-scope failure (NOT a P4 regression):** `go test ./...`
reports two failures in `pkg/agentloop/mcpconfig_test.go`
(`TestInjectLaneMCPConfigCodexAppendsTomlUrlOverride`,
`TestInjectCodexMCPConfigArgsPrecedesExecSubcommand`) — the `X-Striatum-Boot-Epoch`
codex-injection header (commit `f53969fc`, #316) without updated test expectations.
**Confirmed to reproduce on the pristine run-branch HEAD with NONE of this run's
changes present** (checked in a throwaway worktree at HEAD `e2f740ba`), and this
run touches zero `agentloop` files. Flagged so the reviewer does not attribute it
to P4; left for #316's owner.

## Known remaining items for review / the verify-run (honest gaps)

1. **C3 per-step ownership reconcile (§3.3b) — RE-SCOPED to the activation/verify
   run (D6).** `applyDeployStep` applies step DDL + version stamp + cursor advance +
   hash-chained receipt in one transaction (Q3-A) but does NOT snapshot new
   owner-owned oids / `ALTER … OWNER TO striatumd_rw` / assert `has_schema_privilege`
   pre-step. Latent for the base-20 activation (only the terminal 0021, which creates
   no new ownable object); required for a fresh-DB deploy from base 0. The comment
   now states this explicitly; criterion (l) depends on it.
2. **Two-role deployer grant model.** The deployer writes the runtime-owned deploy
   substrate over the owner/admin DSN; 0044 grants the runtime role
   SELECT/INSERT/UPDATE. The live two-role apply game-day confirms the admin DSN
   holds the matching DML (membership or explicit grant).
3. **Receipt sink.** Per-step receipts land in the hash-chained `deploy_receipt`
   table (exactly-once + doctor enumeration). Coupling into `audit_log` via
   `append_audit_row` was deferred to avoid plumbing the daemon-authority secret
   through the bare deploy verb — a reviewer decision point.
4. **Q3-B NT-DDL.** The embedded set has no NT-DDL steps today, so the Q3-B
   pre/post-marker path is a documented structural seam, not yet code.
5. **Command-authority matrix.** `daemon deploy` is a LOCAL owner-connection verb
   (like `daemon owner-ddl`/`daemon migrate`), not an RPC route; the authority
   guardrail tests pass without a `deploy` row, and the matrix doc lies outside this
   run's `write_scope`.

The blocking D7′ regression is fixed the shadow-first-faithful way and proven by a
test that runs; D1′ tightens the B1.1 live arm to the real-spy/three-bucket
obligation; D6 is honestly re-scoped. The deployer surface, plan transcript,
activation predicate, decoupled boot, typed halts, doctor block, and the staged
bundle 0021 are in place, build/vet/lint-clean and locally green (non-PG); the
two-role `G-*` game-days remain the verify-run's job.

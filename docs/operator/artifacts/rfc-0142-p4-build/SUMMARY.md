---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - "docs/operator/artifacts/rfc-0142-p4-build/DRAFT.md"
  - "docs/operator/artifacts/rfc-0142-p4-build/review/REVIEW.md"
---

author: author-author-002

# SUMMARY — RFC 0142 P4 build (`striatum daemon deploy` decoupler), D7′ revision finalized

**Verdict adopted:** `accept_with_findings` (review `REVIEW.md`, severity low). The
sole blocking defect of the prior attempt (**D7′** — embedding owner bundle 0021
bricked the flag-OFF serve-boot path) is fixed the shadow-first-faithful way
(Option B: un-embed 0021), the three required follow-ups landed, and the four
non-blocking findings (F1–F5) are honest, recorded deferrals to the
`rfc-0142-p4-verify` run. This apply pass re-verified the tree green, tidied the
one in-scope cosmetic nit (F5), and publishes this summary. **No scope expansion**:
no P5, no `LatestOwnerBundleVersion` advance, no live 0021 activation.

---

## 1. Files changed (and one-line role)

New files:

| File | Role |
| --- | --- |
| `go/pkg/db/sql/0044_deploy_cursor.sql` | Migration 0044 — additive **runtime-owned** deploy substrate (`deploy_cursor` + `deploy_plan` + `deploy_receipt`); `state` CHECK includes `finalizing`; role-guarded GRANT. |
| `go/pkg/db/sql/owner_staged_activation/0021_revoke_create_privilege.sql` | The C3 DDL-revoke bundle, **staged OUTSIDE `ownerBundleFS`** (Option B) so it is version-controlled but not embedded. |
| `go/pkg/db/deploy.go` | Deployer core: `DeployPlan`/`BuildPlan`/`computePlanHash`/`LoadStoredPlan`, M1 byte arm `VerifyStoredTranscript` + M1 DB-stamp arm `VerifyAppliedDBStamps`, typed halts, `receiptRowHash` chain. |
| `go/pkg/db/deploy_apply.go` | `Deployer.Apply` Q3-A per-step atomicity (`applyDeployStep`: DDL + version stamp + cursor advance + receipt in ONE tx) and the `finalizing` finalizer. |
| `go/pkg/db/deploy_activation.go` | Pure boot-path predicate `DecideDeployActivation` (§3.3a/§3.5) + the DB `CheckDeployActivation` wrapper. |
| `go/pkg/db/deploy_test.go` | Non-PG F-tests for plan shape, hash determinism, receipt chain, typed halts. |
| `go/pkg/db/deploy_activation_test.go` | Non-PG decision-table tests, incl. the new flag-OFF-proof `TestProductionInertBinaryFlagOffServesLegacy`. |
| `go/pkg/db/deploy_pg_test.go` | pg-gated live arms (3-bucket B1.1 mutation-spy, none-cursor, dry-run). |
| `go/pkg/db/owner_revoke_filter_test.go` | F16a synthetic-revoke exclusion + M4 route guard + the re-scoped F16b `TestOwnerBundle0021StagedForActivationNotEmbedded`. |
| `go/pkg/cli/localcommands/daemon_deploy_test.go` | `runDaemonDeploy` dispatch F3 + the M3 verb-preflight arm (inert on this binary). |
| `go/pkg/reads/doctor_deploy.go` | `deployUnrecordedDoctorBlock` — `schema_deploy_unrecorded` + the M1 stamp/byte WARN. |

Modified files:

| File | Role |
| --- | --- |
| `go/pkg/db/owner.go` | M2 surface: `DDLRevokeOwnerBundleVersion = 21`, `isNonRevokeBundle`, `OwnerDDLApplyBundles()` (filters `>= 21`), `RevokeBundleEmbedded()` (derives from `OwnerBundles()` → **false**), added `stagedActivationFS` embed + `StagedRevokeBundle()`. `LatestOwnerBundleVersion`/`RequiredOwnerBundleVersion` **stay 20**. |
| `go/pkg/db/migrations.go` | `LatestDaemonDBVersion = 44`. |
| `go/pkg/db/connection.go` | W→A boot order: `CheckDeployActivation` before `ApplyMigrations`; `serve_verify` returns without the legacy apply; barrier-b forward-watermark; boot comment restated accurately (`none → serve_legacy → byte-identical`). |
| `go/cmd/striatumd/main.go` | Typed-halt arms mapping the deploy decision/verify errors to operator-facing exits. |
| `go/pkg/cli/localcommands/daemon.go` | `"deploy"` added to `RunDaemon` dispatch; admin-DSN resolution; `--dry-run`/`--abort`/`--json`; M3 activation preflight gated on `STRIATUM_DEPLOY_DECOUPLED`. |
| `go/pkg/reads/doctor.go` | Wires `deployUnrecordedDoctorBlock` into the doctor run. |
| `go/pkg/db/reservations_test.go` | Reservation-ledger guard (unchanged behavior; stays green with owner frontier 20). |
| `go/pkg/db/sql/RESERVATIONS.toml` | Owner ordinal 21 recorded as the **staged** DDL-revoke reservation; embedded frontier is 20. **F5 cosmetic fix applied this pass**: the `Frontiers:` header line now reads `owner_bundle = 0020 (embedded)` (was `0021`), matching the detailed note. Comment-only — parser reads only `ordinal`/`file`, so `TestReservationLedgerMatchesOnDisk` stays green. |

---

## 2. Build-step discharge table (PROPOSAL §6, steps 1–7)

| § | Step | Added schema/type/function | Code site (file:symbol) | Named test(s) |
| --- | --- | --- | --- | --- |
| 1 | Migration 0044 | `deploy_cursor`/`deploy_plan`/`deploy_receipt` (runtime-owned); `finalizing` in CHECK | `sql/0044_deploy_cursor.sql`; `migrations.go:LatestDaemonDBVersion=44` | `reservations_test.go:TestReservationLedgerMatchesOnDisk`; live apply F6/F12 → verify |
| 2 | `owner.go` M2 surface (inert) | `DDLRevokeOwnerBundleVersion=21`, `isNonRevokeBundle`, `OwnerDDLApplyBundles()`, `RevokeBundleEmbedded()`, `StagedRevokeBundle()` | `owner.go:OwnerDDLApplyBundles` / `owner.go:RevokeBundleEmbedded` / `owner.go:StagedRevokeBundle` | F16a `TestOwnerDDLApplyExcludesSyntheticRevokeBundle`; M4 `TestOwnerDDLApplyRoutesUseFilteredLoader` |
| 3 | `deploy.go` core | `BuildPlan`, `computePlanHash`, `VerifyStoredTranscript` (M1 byte), `VerifyAppliedDBStamps` (M1 DB-stamp), `receiptRowHash` chain | `deploy.go:BuildPlan` / `deploy.go:VerifyAppliedDBStamps` / `deploy_apply.go:applyDeployStep` | F1 `TestBuildPlanOrdersOwnerThenRuntimeThenRevokeTerminal`; F8/F9 `TestPlanHashIsDeterministicAndBaseSensitive`; F13/F14 `TestReceiptRowHashChains`; F15 `TestVerifyStoredTranscriptCleanPlanPasses` / `TestVerifyStoredTranscriptDetectsBinaryMismatch`; `TestDeployTypedHaltsUnwrapToSentinels` |
| 4 | `runDaemonDeploy` verb | `"deploy"` dispatch; `--dry-run`/`--abort`/`--json`; M3 preflight | `localcommands/daemon.go:RunDaemon` (deploy case) | F3 `TestRunDaemonDeployDispatch`; M3 preflight `TestRunDaemonDeployM3PreflightRefusesWithoutDecoupled` (inert here); F5 live `TestDeployerDryRunPlanLive` → cluster |
| 5 | `CheckDeployActivation` (M3/M5/M6/M7) | `DecideDeployActivation` (pure) + DB wrapper; wired into boot | `deploy_activation.go:DecideDeployActivation`; `connection.go` (W→A order, `serve_verify`); `main.go` (typed halts) | F18 `TestDeployBootPathDecisionTable` + `TestDeployActivationRow16ConditionalMirrorsRow15`; M3 `TestDeployActivationM3GateFiresForEveryCursorState`; `TestProductionInertBinaryFlagOffServesLegacy`; `TestDeployActivationDecisionErrorMapping`; live `TestDeployActivationCompleteInSyncServesVerifyLive` / `TestDeployActivationNoneCursorLive` → cluster |
| 6 | `doctor schema_deploy_unrecorded` | `deployUnrecordedDoctorBlock` (per-step transcript enumeration + M1 stamp/byte WARN) | `reads/doctor_deploy.go:deployUnrecordedDoctorBlock`; wired in `reads/doctor.go` | covered by `pkg/reads` suite (green); live trail enumeration → verify |
| 7 | Owner bundle 0021 (staged) | `0021_revoke_create_privilege.sql` staged out of `ownerBundleFS`; excluded from apply routes; watermark stays 20 | `sql/owner_staged_activation/0021_revoke_create_privilege.sql`; `owner.go:stagedActivationFS` | F16b (re-scoped) `TestOwnerBundle0021StagedForActivationNotEmbedded`; production-EMBED + forced FMA-007 self-heal → activation/verify binary |

The `G-*` two-role game-days (`G-old-binary-refuse`, `G-wrong-binary-resume`,
`G-complete-cursor-flag-off-refuse`, `G-fresh-db-first-boot`, `G-revoke-last`) need
a live two-role cluster and are the `rfc-0142-p4-verify` run's job; this build
provides the surface so they can fire.

---

## 3. B1.1 / B1.2 discharge (binding obligation)

**B1.2 — concrete cursor-state enums are table-driven by VALUE.**
`TestDeployBootPathDecisionTable` (`deploy_activation_test.go`) drives
`concreteCursorStates = {none, in_progress, step_committed, finalizing, complete,
aborted}` *by value* — each enum value is its own row, not the prose group label —
crossed with `decoupled × revoke × inSync × applied_owner ∈ {0, 20, ≥21}`. The
grouped "64-cell" shorthand of §6 is therefore fully expanded.

**B1.1 — the row-16 derivation, both sub-cases, all three buckets.**
`TestDeployBootPathDecisionTable` constructs the `complete` cursor in **both** the
in-sync AND out-of-sync sub-cases across all three `applied_owner` columns
(`==0`, `==20`, `>=21`):

- in-sync decoupled (revoke-embedding, row 16) cells serve **verify-only**
  (`DeployServeVerify`, never `DeployServeLegacy`);
- the spy invariant asserts **no `serve_legacy`** for any decoupled or
  revoke-embedding cell;
- `TestDeployActivationRow16ConditionalMirrorsRow15` is the explicit row-15 ≡
  row-16 (and column `==0` ≡ `==20` ≡ `>=21`) identity — the decoupled-complete
  branch reads **neither `applied_owner` nor `revokeEmbedded`**
  (`deploy_activation.go:DecideDeployActivation`, the `DecoupledEnabled` complete
  branch), so the outcome is identical across buckets.

The genuine orthogonality construction is the tightened **live** arm
`TestDeployActivationCompleteInSyncServesVerifyLive` (`deploy_pg_test.go`): it seeds
the `owner_bundle_meta` buckets **absent (`==0`), `==20`, and `==21`** (sub-tests,
fresh pool each), independently sets `schema_state.fingerprint ==
ExpectedFingerprint()` AND a frontier-targeting plan, and replaces the prior
decision-value tautology with a **real behavioral mutation spy** — it snapshots the
`schema_state` singleton serialized whole via `to_jsonb` plus the
`schema_migrations` row count before/after `CheckDeployActivation` and asserts them
byte-identical, proving the mutating path (`ApplyMigrations` /
`RecordSchemaFingerprint`) is NOT entered. Both `revoke=false`/`true` serve
verify-only (M7 revoke-independence); the out-of-sync sub-case halts
`awaiting_deploy`, also without mutation. (pg-gated; compiles + skips locally.)

---

## 4. §6.5 acceptance-criteria coverage (a)–(l)

| | Criterion | Named test / code site | Status |
| --- | --- | --- | --- |
| (a) | crash-resume stable key (BC-N1) | `deploy_apply.go:Deployer.Apply` resume off STORED transcript; receipt keyed `(plan_hash, step_index)` | met at unit level; live GD → verify |
| (b) | divergent-binary resume refuses (M1) | `deploy.go:VerifyStoredTranscript` (byte) + `deploy.go:VerifyAppliedDBStamps` (DB-stamp), both called in `resume()`; `TestVerifyStoredTranscriptDetectsBinaryMismatch` | **met** |
| (c) | universal pre-revoke serve edge (BC-N2) | `deploy_activation.go:DecideDeployActivation` non-complete rows → `awaiting_deploy`; barrier-b in `connection.go`; F18 rows | met at predicate level |
| (d) | self-heal does not commit revoke early (M2) | `owner.go:OwnerDDLApplyBundles` in-loop guard + filtered nil-fallback; F16a | met at unit level; forced FMA-007 pgtest → verify |
| (e) | complete-cursor flag-OFF revoke refusal (M3) | `deploy_activation.go:DecideDeployActivation` step-0 gate; `TestDeployActivationM3GateFiresForEveryCursorState` + verb preflight | predicate + preflight met (fires on the **activation** binary; this inert binary correctly does NOT halt — the D7′ fix) |
| (f) | fresh-DB serve + shortfall halt (M5) | `TestProductionInertBinaryFlagOffServesLegacy` (real embed value, non-PG, RUNS) + live `TestDeployActivationNoneCursorLive`; `CheckOwnerBundleWatermark` unchanged | **met by a test that runs**; live arm → cluster |
| (g)/(h) | no-revoke complete in/out-of-sync; legacy no-op (M6 r13/r15) | F18 rows 13/15 conditional | met at predicate level |
| (i)/(j) | revoke-embedding complete (M7 r16, `==0/==20/≥21`) | F18 row 16 parametric + row15≡row16; live 3-bucket arm | predicate met; **live 3-bucket arm now real (D1′)** |
| (k) | hash-chained receipt + doctor (BC-N1) | `deploy.go:receiptRowHash` chain + `doctor_deploy.go:deployUnrecordedDoctorBlock` | **met** |
| (l) | two-role activation + post-deploy CREATE denial (C3) | staged 0021 supplies terminal revoke; C3 §3.3b reconcile + live game-day | **RE-SCOPED to the verify run** (Option B / D6, finding F1) — must not be silently dropped |

---

## 5. Shadow-first invariants — confirmed (the D7′ correction)

All hold, and the central claim is now **true and proven by a test that runs**
(it was false in the embedded attempt 2):

- **`STRIATUM_DEPLOY_DECOUPLED` default OFF** — `deploy_activation.go:DeployDecoupledEnabled`;
  the legacy `ConnectAndMigrate` path is unchanged for a no-revoke flag-OFF binary.
- **`LatestOwnerBundleVersion` STAYS 20** and **`RequiredOwnerBundleVersion` STAYS 20**
  (`owner.go:LatestOwnerBundleVersion = 20`, `RequiredOwnerBundleVersion =
  LatestOwnerBundleVersion`).
- **Migration 0044 is strictly additive runtime-owned** — no owner DDL, no
  `owner_bundle_meta` touch; role-guarded GRANT.
- **0021 excluded from all apply routes** — staged in `sql/owner_staged_activation/`,
  NOT matched by `//go:embed sql/owner/*.sql`; `OwnerBundles()` does not list it;
  `OwnerDDLApplyBundles()` filters `>= 21`. Therefore `RevokeBundleEmbedded()` returns
  **false** and `ExpectedFingerprint()` does not hash 0021's bytes — the flag-OFF
  serve-boot path is **byte-identical to a pre-P4 binary**. Proven by
  `TestProductionInertBinaryFlagOffServesLegacy` (reads the REAL embed value, asserts
  `DecideDeployActivation(none, decoupled=false, revoke=<real>) == DeployServeLegacy`,
  NOT `awaiting_deploy_config`; reds the instant 0021 is re-embedded).
- **Fresh `applied_owner == 0` bootstrap still serves** — `CheckOwnerBundleWatermark`
  unchanged; the `none` branch reaches `serve_legacy`.
- **Existing serve-boot suites pass unchanged** — the `pgtest` harness
  (`ConnectAndMigrate` asserting `version == LatestDaemonDBVersion`) bootstraps green
  again (`pkg/pgtest` ✓, `pkg/db` ✓); `len(applied) == len(OwnerBundles())` holds at 20.

---

## 6. Verification

Run from `go/` in the per-job worktree (go1.25.0, golangci-lint v2.12.2):

- `go build ./...` — **PASS** (exit 0).
- `go vet ./...` — **PASS** (exit 0).
- `golangci-lint run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./...`
  (the exact `go/Makefile` CI config) — **0 issues**.
- `go test ./pkg/db/... ./pkg/reads/... ./pkg/pgtest/... ./cmd/striatumd/...` — **all green**:
  `pkg/db` ok, `pkg/reads` ok, `pkg/pgtest` ok, `cmd/striatumd` ok. The pg-integration
  arms compile and **SKIP** cleanly (`STRIATUM_PG_TEST_URL` unset on this host).
- Working tree clean apart from the SUMMARY/DRAFT artifacts and the one-line F5
  cosmetic fix in `RESERVATIONS.toml`.

The two `pkg/agentloop/mcpconfig_test.go` failures observed under a full
`go test ./...` are **pre-existing and unrelated** (the `X-Striatum-Boot-Epoch`
codex-injection header from `f53969fc`/#316; this run touches zero `agentloop`
files) and are correctly not attributable to P4.

**The live game-day** — §6.5 (a)–(l) against a real two-role cluster, plus the
`G-*` deployer game-days — is the **`rfc-0142-p4-verify` run's job**; this build
delivers the surface so those arms can fire. **Operator choreography for 0021
activation is deferred** to the activation/verify binary per PROPOSAL §4.3
(two-binary choreography).

### Non-blocking findings carried to the verify run (F1–F5, do not silently drop)

- **F1 (medium)** — C3 §3.3b per-step ownership reconcile + criterion (l) owed by
  the verify run (Option B / D6). Latent for the base-20 activation (terminal 0021
  creates no new ownable object); required for a fresh-DB deploy from base 0.
- **F2 (low)** — the C1 finalizer omits a terminal "complete" receipt row; per-step
  receipts are hash-chained + doctor-enumerated (criterion (k) met), so this is a
  minor provenance gap to add in the verify/activation run.
- **F3 (low)** — `command-authority-matrix.md` `daemon.deploy` row not added;
  `daemon deploy` is a LOCAL owner-connection verb (not an RPC route), so the
  authority guardrail tests pass without it. The matrix doc is outside this lane's
  `write_scope`; the verify-run owner should add the row or record the matrix-exempt
  rationale.
- **F4 (informational)** — live two-role game-days deferred (expected; verify-run job).
- **F5 (cosmetic)** — `RESERVATIONS.toml` `Frontiers:` header — **addressed this pass**
  (header now reads `owner_bundle = 0020 (embedded)`).

The roadmap/PROPOSAL updates for the F1/F3 re-scope live under
`docs/operator/artifacts/rfc-0142-p4-design-v9/…` and `docs/operator/rfc-roadmap.md`,
which are **outside this lane's `write_scope`**; they are recorded here and in
`DRAFT.md`/`REVIEW.md` for the verify-run owner to mirror.

---

## 7. Scope

Confirmed — **no scope expansion** beyond the accepted contract:

- **P5 is deferred** — no rehearsal, no fidelity tiering, no full-data clone, no
  expand-contract.
- **No `LatestOwnerBundleVersion` advance** — stays 20 (`RequiredOwnerBundleVersion`
  stays 20).
- **No live 0021 activation** — 0021 lands INERT (staged out of `ownerBundleFS`,
  excluded from every apply route); the flag stays default OFF; activation is the
  `rfc-0142-p4-verify` run's job under §4.3's two-binary choreography.

No test or shadow-first invariant was weakened in this finalization.

---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: author-author-002
run_id: run_992bd797fc136f1e3d782f443f9fb2ad
title: "RFC 0170 P0 build - applied implementation summary"
inputs:
  - "docs/operator/artifacts/rfc-0170-p0-design-v5/commit/proposal/PROPOSAL.md"
  - "docs/operator/artifacts/rfc-0170-p0-design-v5/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md"
  - "docs/rfcs/0170-self-culling-repository-and-cull-workflow-class.md"
  - "docs/operator/artifacts/rfc-0170-p0-build/DRAFT.md"
  - "docs/operator/artifacts/rfc-0170-p0-build/review/REVIEW.md"
---

# SUMMARY - RFC 0170 P0 Build

author: author-author-002

The reviewed implementation is finalized for verifier handoff. The review
verdict was `accept_with_findings` with no gate-critical implementation gap and
no required revision. This apply pass re-verified the worktree, records the
assertion coverage, and leaves only the live PostgreSQL verifier residual named
below.

## Files Changed By Gate

### G1 - Tier-1 Predicate

- `go/pkg/recovery/decay_tick_sweep.go` - implements the tree-local Tier-1
  predicate over RFC, decision, and status-bearing doc rows: structural status
  admission, live-successor extraction, static protected pathspec, active inbound
  citation filtering, and nomination/withdrawal delta construction.
- `go/pkg/recovery/decay_tick_sweep_test.go` - adds the BC-618 known-set corpus,
  protected-path fixture, bare/bold `Status:` parser regression, branch-exclusion
  assertion, and SQL/static-surface guard.
- `docs/operator/artifacts/rfc-0170-p0-build/DRAFT.md` - records the draft
  implementation mapping for the reviewed G1 predicate.

### G2 - Read-Only Safety

- `go/pkg/recovery/decay_tick_sweep.go` - adds `DecayTickSweep.SweepOnce` with
  O(1) single-slot launch/skip behavior, detached goroutine timeout ownership,
  panic recovery, cooperative cancellation, L4 compute-then-commit, and zero
  writes after timeout or scan failure.
- `go/cmd/striatumd/main.go` - wires one persistent `DecayTickSweep` after
  `ActiveRunSweep` and the metrics fold, logs fold errors, and returns the
  recovery result/error unchanged so cull work stays off the wait-gating path.
- `go/pkg/recovery/decay_tick_sweep_test.go` - covers panic isolation, HANG A/B
  off-path timing, the wait-path negative control, cooperative timeout, timeout
  relation, and BC-619 late-return-zero-write behavior.
- `docs/operator/artifacts/rfc-0170-p0-build/DRAFT.md` - records the reviewed G2
  safety mapping.

### G3 - Substrate Correctness

- `go/pkg/db/sql/0045_cullable_entity.sql` - adds runtime-owned
  `striatumd.cullable_entity` with `PRIMARY KEY (kind, ref)`, the required `kind`
  and `candidacy_state` CHECKs, no FK, no owner DDL, and guarded
  `GRANT SELECT, INSERT, UPDATE` to `striatumd_rw` with no DELETE grant.
- `go/pkg/db/sql/RESERVATIONS.toml` - reserves runtime migration ordinal 45 for
  `0045_cullable_entity.sql`.
- `go/pkg/db/migrations.go` - advances `LatestDaemonDBVersion` to 45 and labels
  the RFC 0170 P0 migration.
- `go/pkg/db/read_authority_inventory.go` - adds
  `readAuthorityInventory["cullable_entity"] = ReadClassRuntimeOperational`.
- `go/pkg/db/write_authority_inventory.go` - adds
  `writeAuthorityInventory["cullable_entity"] = ClassRuntimeDML`.
- `go/pkg/recovery/decay_tick_sweep.go` - reads `cullable_entity` with explicit
  columns only and writes only insert/update candidacy observations.
- `go/pkg/recovery/decay_tick_sweep_test.go` - guards against `SELECT *` and
  forbidden state/authority surfaces in the sweep.
- `README.md` - updates the front-door schema reference to runtime schema 45.
- `CHANGELOG.md` - records the RFC 0170 P0 observe-only substrate.
- `docs/operator/artifacts/rfc-0170-p0-build/DRAFT.md` - records the reviewed G3
  substrate mapping.

### G4 - Forward Compatibility And P0 Boundary

- `go/pkg/db/sql/0045_cullable_entity.sql` - keeps `(kind, ref)` as the stable
  conflict key and uses an additive `candidacy_state` CHECK for later cull phases.
- `go/pkg/recovery/decay_tick_sweep.go` - uses `(kind, ref) ON CONFLICT` and
  emits observe-only nominated/withdrawn state; it does not implement tombstone,
  deletion, page, doctor class, run-admission, or `cull_gate` behavior.
- `go/pkg/recovery/decay_tick_sweep_test.go` - enforces the P0 SQL/static-surface
  boundary.
- `README.md`, `CHANGELOG.md`, and `docs/operator/artifacts/rfc-0170-p0-build/DRAFT.md`
  - record the observe-only P0 boundary for operators and reviewers.

## Assertion Discharge

| Assertion | Disposition | Named test or check |
| --- | --- | --- |
| A1 | Structural status is the only candidate admission source for Tier-1 docs/RFCs/decisions. | `TestDecayTickStructuralStatusParsesBareAndBoldTitleBlocks`; `TestDecayTickKnownSetCorpus` |
| A2 | A superseded/tombstoned entity needs a parseable live successor; bare superseded rows are withheld. | `TestDecayTickKnownSetCorpus` (`rfc:0028`, `decision:D084`) |
| A3 | Static tree-local protected pathspec protects root/provenance/scaffold paths while RFCs and decisions remain eligible. | `TestDecayTickProtectedPathspecIsTreeLocal` |
| A4 | Active-baseline inbound citation filtering preserves known live/safe RFCs and decisions. | `TestDecayTickKnownSetCorpus` (`rfc:0097`, `rfc:0027`, `rfc:0039`, `rfc:0041`, `decision:D174`) |
| A5 | The decision successor rule nominates known-dead `decision:D267`. | `TestDecayTickKnownSetCorpus` |
| A6 | `decision:D081` is the documented #618 safe false negative, withheld by the audit citation. | `TestDecayTickKnownSetCorpus` |
| A7 | P0 emits no branch candidacy. | `TestDecayTickKnownSetCorpus` |
| A8 | Predicate stays mechanical/tree-local and does not consult verdict/audit-log/doctor runtime state. | `TestDecayTickStaticSQLShape` |
| B1 | `SweepOnce` is O(1) launch/skip and never waits for scan completion. | `TestDecayTickSweepOffPathDoesNotDeferNextRecoveryTick` |
| B2 | Scanner panic is recovered inside the detached goroutine and writes nothing. | `TestDecayTickSweepPanicRecoveredAndNoWrite` |
| B3 | Cull fold cost is bounded below the recovery cadence and avoids broad runtime-state reads. | `TestDefaultCullFoldTimeoutBelowSweepInterval`; `TestDecayTickStaticSQLShape` |
| B4 | Timeout, scan error, panic, and late-return paths do not reach the write phase. | `TestDecayTickSweepCooperativeScanStopsAtTimeout`; `TestDecayTickSweepLateReturnAfterTimeoutWritesNothing`; `TestDecayTickSweepPanicRecoveredAndNoWrite` |
| B5 | Cursor refresh is not deferred by the cull fold, and the assertion is non-vacuous against a wait-path negative control. | `TestDecayTickSweepOffPathDoesNotDeferNextRecoveryTick`; `TestDecayTickSweepRefreshAssertionCatchesWaitPathJoin` |
| C1 | Runtime slot 0045 is reserved and `LatestDaemonDBVersion` reaches 45. | `TestReservationLedgerMatchesOnDisk`; `TestMigrationsAreOrdered`; `TestApplyMigrationsRecordsVersion` |
| C2 | `0045_cullable_entity.sql` carries table + both CHECKs + runtime grant, with no owner DDL, FK, or DELETE grant. | `TestFullMigrationSetAppliesAsNonOwnerRoleOnTwoRoleDB` residual for live PG; `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`; `TestFutureRuntimeMigrationsDoNotFKOwnerHeldTables` |
| C3 | Both authority inventory rows exist and `cullable_entity` reads use explicit columns, not `SELECT *`. | `TestReadAuthorityInventoryCoversEmbeddedTablesWithoutPostgres`; `TestWriteAuthorityInventoryCoversEmbeddedTablesWithoutPostgres`; `TestDecayTickStaticSQLShape` |
| D1 | P0 schema and writes are forward-compatible and observe-only: `(kind, ref)` upsert, additive states, no tombstone/delete/page/doctor/run-admission action. | `TestDecayTickStaticSQLShape`; review source audit; `go vet ./...` |

## BC-618 And BC-619

BC-618 is discharged for P0 by `TestDecayTickKnownSetCorpus` and
`TestDecayTickProtectedPathspecIsTreeLocal`: the known preserved set is withheld,
`decision:D267` is nominated, and `decision:D081` is recorded as the documented
#618 withheld member caused by the status-frozen audit citation outside
`docs/records/_frozen/`.

BC-619 is discharged for P0 by
`TestDecayTickSweepLateReturnAfterTimeoutWritesNothing` plus the off-path refresh
tests: a ctx-ignoring scan that returns after `DefaultCullFoldTimeout` performs
zero `cullable_entity` UPSERTs, and the fold does not join back into the recovery
wait path.

## Migration Slot

The runtime migration slot is verified as 0045: `origin/main...HEAD` adds
`go/pkg/db/sql/0045_cullable_entity.sql`, the previous runtime migration on disk
is `0044_deploy_cursor.sql`, `RESERVATIONS.toml` reserves ordinal 45, and
`LatestDaemonDBVersion` is 45.

## Verification

- PASS: `cd go && go build ./... && go vet ./...`
- PASS: `go test ./pkg/recovery -run 'TestDecayTick' -count=1`
- PASS: `go test ./pkg/db -run 'Test(Read|Write)?Authority|TestReservation|Test.*Reservation|TestMigration' -count=1`
- PASS: `go test ./pkg/recovery ./pkg/db ./cmd/striatumd -count=1`
- PASS: `go test ./cmd/striatum -run TestReadmeFrontDoorStatusStaysCurrent -count=1`
- SKIP in this local lane, expected verifier residual:
  `go test ./pkg/db -run 'TestFullMigrationSetAppliesAsNonOwnerRoleOnTwoRoleDB|TestTwoRoleApplyCatchesForbiddenOwnerFK' -count=1 -v`
  skipped because `STRIATUM_PG_TEST_URL` is not set.

The verifier stage must prove the PG-backed pgtests live, especially the two-role
runtime-migration path (`TestFullMigrationSetAppliesAsNonOwnerRoleOnTwoRoleDB`)
and its forbidden owner-FK negative control
(`TestTwoRoleApplyCatchesForbiddenOwnerFK`).

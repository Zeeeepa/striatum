---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["rfc-0170", "p0", "review", "cullable_entity", "decay_tick_sweep"]
---

# Review - RFC 0170 P0 Build

author: reviewer-reviewer-001

**Verdict: accept_with_findings.** I reviewed the draft worktree at
`486bee2227959f2539106e699c92b40317ab2467` against the v5 cleared SPEC and
collaboration ledger. I found no gate-critical implementation gap and no
revision-driving defect. The implementation is ready for the downstream apply
stage, with the normal verifier-stage residual noted below.

## Gate Review

### G3 substrate

Accepted. `go/pkg/db/sql/0045_cullable_entity.sql` is the runtime migration in the
free 0045 slot, and `go/pkg/db/migrations.go` advances `LatestDaemonDBVersion` to
45. The table has `PRIMARY KEY (kind, ref)`, both required CHECKs, no foreign
keys, no owner DDL, and the guarded `GRANT SELECT, INSERT, UPDATE` to
`striatumd_rw` with no DELETE grant. `go/pkg/db/sql/RESERVATIONS.toml` reserves
ordinal 45, and both authority inventories include `cullable_entity` with
`ReadClassRuntimeOperational` and `ClassRuntimeDML`. Source grep found no actual
`SELECT *` against `cullable_entity`; the sweep reads explicit columns.

### G1 predicate

Accepted. The Tier-1 predicate is tree-local and mechanical: it reads structural
`Status:` fields from the repository tree, uses fixed successor parsing and
inbound line filtering, does not consult `striatumd.verdicts` or
`superseded_by_decision_id`, and produces no branch candidacy. The known-set test
exercises the real live tree path, not a stub: it asserts `decision:D267`
nominated; `decision:D081` withheld by the documented #618 audit citation; the
known preserved RFCs `0097`, `0027`, `0039`, `0041`, `0028`; decisions `D084` and
`D174` withheld; and no `kind=branch` nomination. The protected-path test keeps
RFCs and decisions eligible while protecting the static tree-local root set.

### G2 read-only safety

Accepted. `DecayTickSweep.SweepOnce` performs only the O(1) single-slot
compare-and-swap and goroutine launch, then returns immediately. The detached
goroutine owns the timeout, panic recovery, scan, and commit path. `striatumd`
wires a persistent `DecayTickSweep` after `ActiveRunSweep` and the metrics fold,
logs cull fold errors, and returns the recovery result/error unchanged, so the
fold stays off the recovery wait-gating path. The only write path is
`cullable_entity` insert/update in `commitCullableChanges`; timeout, scan error,
panic, and late-return paths write nothing. Tests cover the timeout relation,
panic recovery, off-path refresh timing with a wait-path negative control,
cooperative timeout, and the BC-619 late-return-zero-write guard.

### G4 forward compatibility and P0 boundary

Accepted. The UPSERT uses `(kind, ref) ON CONFLICT`, and the `candidacy_state`
CHECK is additive for later states. I found no P0-forbidden tombstone, deletion,
page, doctor-class, run-admission, or `cull_gate` action in the new sweep path.
The migration admits later kinds while the P0 scanner itself emits only RFC,
decision, and doc evaluations.

## Verification

- PASS: `cd go && go build ./... && go vet ./...`
- PASS: `go test ./pkg/recovery -run 'TestDecayTick' -count=1`
- PASS: `go test ./pkg/db -run 'Test(Read|Write)?Authority|TestReservation|Test.*Reservation|TestMigration' -count=1`
- PASS: `go test ./pkg/recovery ./pkg/db ./cmd/striatumd -count=1`
- PASS: `go test ./cmd/striatum -run TestReadmeFrontDoorStatusStaysCurrent -count=1`

## Non-blocking Finding

F1 (low, verifier-stage): I did not require or rerun full `go test ./...` for this
review because the packet's required command is build plus vet, and the draft
already reports an unrelated `pkg/agentloop` expectation mismatch in the broader
suite. The RFC 0170 load-bearing packages and README schema guard are green under
the focused commands above. The verifier stage should decide whether to re-run the
full repository suite after that unrelated fixture is reconciled.

# RFC 0170 P0 Build Draft

author: author-author-002

## Scope

Implemented the RFC 0170 P0 observe-only self-culling substrate from the v5 cleared SPEC. This slice writes only `striatumd.cullable_entity` candidacy rows and takes no cull action: no deletion, tombstone, page, doctor class, run-admission effect, or branch candidacy.

## Deliverables

- `go/pkg/db/sql/0045_cullable_entity.sql`: verified `0045` was free before creation. Adds `striatumd.cullable_entity(kind, ref, last_reinforced_at, decay_score, reachable_from_root, candidacy_state)` with primary key `(kind, ref)`, the specified `kind` and `candidacy_state` checks, no foreign keys, no owner-held-table DDL, and guarded `GRANT SELECT, INSERT, UPDATE` to `striatumd_rw` with no `DELETE` grant. Discharges G3 / C2 / PC1 / PC2.
- `go/pkg/db/sql/RESERVATIONS.toml` and `go/pkg/db/migrations.go`: reserve runtime ordinal 45 and advance `LatestDaemonDBVersion` with the RFC 0170 label. Discharges G3 / C1 / PC2.
- `go/pkg/db/read_authority_inventory.go` and `go/pkg/db/write_authority_inventory.go`: add `readAuthorityInventory["cullable_entity"] = ReadClassRuntimeOperational` and `writeAuthorityInventory["cullable_entity"] = ClassRuntimeDML`. Discharges G3 / C1.
- `go/pkg/recovery/decay_tick_sweep.go`: adds `DefaultCullFoldTimeout = 10 * time.Second`, `DecayTickSweep`, the detached single-in-flight off-wait-path fold, panic recovery in the detached goroutine, cooperative filesystem scan cancellation, bounded status-head reads, explicit-column `SELECT kind, ref, candidacy_state`, compute-then-commit delta assembly, all-or-nothing UPSERT/withdraw transactions, and title-block parsing for both `Status:` and `**Status:**`. Discharges G1 / A1-A8, G2 / B1-B5, C3, and D1.
- `go/cmd/striatumd/main.go`: wires one persistent `DecayTickSweep` instance after the active recovery sweep at the existing metrics-fold position. The recovery sweep result and error are returned unchanged; cull fold failures are logged and discarded. Discharges G2 / B1 / B4 / B5.
- `go/pkg/recovery/decay_tick_sweep_test.go`: adds the known-set corpus test, protected-path fixture, bare/bold status parser regression, timeout relation test, panic-isolation regression, off-path A/B refresh-not-deferred test plus wait-path negative control, cooperative timeout test, late-return-zero-write guard, and static SQL-shape/cost guards. Discharges BC-618 and BC-619 for P0.
- `CHANGELOG.md`: records the RFC 0170 P0 observe-only substrate under Unreleased.

## Gate Mapping

- G1 / A1-A8: the predicate is mechanical and tree-local over `kind in {rfc, decision, doc}`. It reads structural status fields only, applies the decision successor rule (cell 3 then cell 5), withholds protected/static roots, applies live inbound citation filtering, excludes branch candidacy, ignores verdict supersession, nominates `decision:D267`, records `decision:D081` as the documented #618 withheld false negative, and preserves the known live/safe set.
- G2 / B1-B5: `SweepOnce` does O(1) slot check/spawn/skip only. The detached goroutine owns `context.WithTimeout`, recovers panics at its top frame, performs zero writes on timeout or late return, and never joins back into the recovery goroutine. The only write path is `cullable_entity`; there is no doctor/admission/cursor write.
- G3 / C1-C3: migration slot 0045 is reserved and implemented, both authority inventories are present, the runtime grant is `SELECT, INSERT, UPDATE`, and every `cullable_entity` read projects explicit columns.
- G4 / D1: P0 uses `(kind, ref)` plus additive state semantics and `ON CONFLICT`, leaving phase/toll writers and later `candidacy_state` additions additive.

## Build Carries

- BC-618: implemented the known-set table-driven live-tree fixture. It asserts zero false positives on preserved RFCs/decisions, `decision:D267` nominated, `decision:D081` withheld by the documented audit citation, and the static protected pathspec remains tree-local with RFCs/decisions eligible.
- BC-619: implemented the late-return-zero-write guard. A scan that ignores `ctx.Done()` and returns after `DefaultCullFoldTimeout` reaches zero UPSERTs. The off-path fold test proves tick 2 refresh timing matches a no-cull control and the negative control fails when a wait-path join is simulated.

## Verification

- The source delta adds `go/pkg/db/sql/0045_cullable_entity.sql` after verifying the free runtime slot in the run context; the current worktree now shows `0045_cullable_entity.sql` as the expected new migration and `0044_deploy_cursor.sql` as the previous runtime migration.
- `go test ./pkg/recovery` passes.
- `go test ./pkg/db` passes.
- `go test ./pkg/recovery ./pkg/db ./cmd/striatumd` passes.
- `go build ./... && go vet ./...` passes.
- `go test ./...` was also run. It is not green because current tests require changes outside this packet's write scope: `cmd/striatum` expects README to say `runtime schema 45`, and `pkg/agentloop` has existing MCP boot-epoch header expectation drift around the `X-Striatum-Boot-Epoch` injected Codex MCP header. I did not edit those out-of-scope files in this draft lane.

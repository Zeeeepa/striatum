# FALSIFIER - RFC 0142 P4 v9 carry-forward / regression review

author: falsifier-reviewer-002

## Bottom Line

I do not find a standing carry-forward or regression falsification in the v9
holder. The M7 fix is at least addressed, and the row-16 repair does not appear
to break the v8-cleared findings M6, M5(row-1), M3, M4, M1, M2, BC-N1, BC-N2,
C1, C2, or C3.

The direct v8 refutation cell now has the correct written outcome: row 16
(`complete`, decoupled ON, revoke-embedding) is conditional on A3 fingerprint
sync in the `==0`, `==20`, and `>=21` columns. F18 is also extended over the
seven A-reaching complete-row cells and keeps the `connection.go:399` spy list at
exactly the same four cells. I found implementation obligations the P4 build
must honor, but not a material design gap that should stop v9 from clearing.

## Challenge Attempt 1: M7 Fix Regresses M6 Rows 13/15 Or The Spy List

### Claim Attacked

The most likely regression would be a local row-16 fix that accidentally
re-collapses the v8 M6 repair: rows 13 and 15 in the `==0` column must remain
conditional exactly like `==20`, and the degenerate row-13 in-sync `:399` rewrite
must remain in both section 4.5 and F18. Row 16 must not be added to the legacy
self-record spy list.

### Concrete Refutation Re-run

The v9 holder preserves the W-to-A independence invariant and adds the M7
sub-invariants without narrowing the M6 rule. Section 0.2 still says A never
reads `owner_bundle_meta` / `applied_owner` after W passes, and now also states
that the decoupled complete branch reads neither `applied_owner` nor
`revokeEmbedded` (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:161`,
`:171`, `:182`).

The table keeps row 13 and row 15 conditional in both `==0` and `==20` columns,
and then makes row 16 conditional in `==0`, `==20`, and `>=21`
(`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:654`,
`:656`, `:657`). The cross-row audit explicitly restates that rows 13 and 15
still match across `==0`/`==20`, and that row 16 now matches row 15 on the
decoupled branch (`HOLDER.md:681`, `:684`).

Section 4.5 keeps the legacy self-record set unchanged: only `{1/==0, 1/==20,
13-in-sync/==0, 13-in-sync/==20}` reach `connection.go:399`; row 16 serves via
verify-only and fires no `:399` spy (`HOLDER.md:889`, `:894`). F18 repeats the
same four-cell spy list and adds the row-16 conditional assertions without
adding row-16 spy entries (`HOLDER.md:941`).

### Strongest Holder Rebuttal

The strongest rebuttal is correct: row 16 is decoupled, so an in-sync serve cell
never calls the legacy mutation/self-record path. The M7 fix changes the table
oracle, not the write path. Therefore it does not reopen the M3/M6 Invariant B
problem.

### Result

No M6 regression remains. Rows 13/15 stay conditional and the section 4.5 / F18
spy list remains coherent.

## Challenge Attempt 2: M7 Reopens The M5 Fresh-DB Serve Or M3 Config Gate

### Claim Attacked

A row-16 repair could overgeneralize the decoupled complete rule and weaken two
older boundaries: the M5 fresh database cell `applied_owner == 0` must still
serve for the inert no-revoke first boot, while every revoke-embedding binary
with decoupling OFF must still halt at the hoisted M3 config gate before
`ApplyMigrations` and before `RecordSchemaFingerprint`.

### Concrete Refutation Re-run

The source anchors still support the M5 split. `RequiredOwnerBundleVersion` is
still `LatestOwnerBundleVersion`, and `LatestOwnerBundleVersion` is still 20
(`go/pkg/db/owner.go:23`, `:35`). `CheckOwnerBundleWatermark` still returns nil
for `applied == 0` before the `applied < RequiredOwnerBundleVersion` shortfall
(`go/pkg/db/owner.go:140`, `:145`, `:148`). The v9 table keeps row 1 / `==0` as
`SERVE-legacy - FRESH-DB BRING-UP`, and F18a still tests fresh serve plus the
`1..19` halt (`HOLDER.md:642`, `:942`).

The M3 gate is also intact in the spec. Section 3.3a keeps step 0 as
`revokeEmbedded && !decoupledEnabled -> awaiting_deploy_config`, first for every
cursor state (`HOLDER.md:484`). The table keeps the row-2, row-6, row-10, and
row-14 revoke-embedding / flag-OFF cells as `awaiting_deploy_config`
(`HOLDER.md:643`, `:647`, `:651`, `:655`). F17 remains the build assertion that
this path calls neither `ApplyMigrations` nor `RecordSchemaFingerprint`
(`HOLDER.md:940`).

The current source still has the legacy boot order that the build must move
behind the new A gate: W at `connection.go:349`, `ApplyMigrations` at `:353`,
drift gate at `:376`, and `RecordSchemaFingerprint` at `:399`
(`go/pkg/db/connection.go:341`, `:349`, `:353`, `:376`, `:399`). I found no
`go/pkg/db` source drift from the v8 anchor set (`git diff --stat 3f9d5734 HEAD
-- go/pkg/db` was empty in this worktree), so the holder's source assumptions for
M5/M3/M6/M7 still line up with the carried-forward anchors.

### Strongest Holder Rebuttal

The row-16 conditional serve can look suspicious over `applied_owner == 0`, but
it is scoped to `cursorState=complete`, decoupled ON, and fingerprint in-sync. It
does not touch the `cursorState=none` fresh bring-up cell. Likewise, the M3 gate
is skipped only when decoupling is ON, which is the intended verify-only path;
with decoupling OFF it remains the first A predicate.

### Result

No M5 or M3 regression remains.

## Challenge Attempt 3: M1/M2/M4/C3 Carry-Forward Is Undercut By Option 1

### Claim Attacked

Option 1 for M7 deliberately does not add a new serve-boot DB-stamp verifier.
That could have reopened M1, or it could have hidden a new M2/C3 ordering hole by
letting bundle 0021 reach an owner-ddl apply path before the deploy-terminal step.

### Concrete Refutation Re-run

The holder is explicit that M1 remains scoped to deploy resume and finalizer step
0, not ordinary serve-boot; row 16 is made conditional instead of guarded by a
new boot-time stamp verifier (`HOLDER.md:570`, `:576`). That is the Option 1
repair the v8 ledger allowed. The recorded-fingerprint premise is still the A3
predicate, and the source confirms `LiveFingerprint` reads only
`striatumd.schema_state` while `RecordSchemaFingerprint` writes only that
singleton (`go/pkg/db/schema_drift.go:145`, `:154`, `:171`, `:187`).

M2 and C3 are also not changed by the M7 fix. Section 3.2a still specifies the
single non-revoke filter and split loader, with `OwnerDDLApplyBundles()` as the
apply slice and `OwnerBundles()` as the full embed/listing loader
(`HOLDER.md:404`, `:412`, `:420`, `:426`). Section 4.4 still says 0021 is the
terminal deploy step and excluded from every `owner-ddl apply` route
(`HOLDER.md:849`, `:850`). F16a/F16b remain the staged proof: synthetic first,
production only after 0021 exists (`HOLDER.md:939`).

This remains a build obligation rather than a new v9 design gap: current source
has not implemented `OwnerDDLApplyBundles()` yet because P4 is still a design
run, and the holder's rollout step 2 says that surface lands first
(`HOLDER.md:948`, `:950`). The carry-forward question for this lane is whether
M7 changed or contradicted the M2/C3 mechanism; I did not find such a change.

### Strongest Holder Rebuttal

The holder can honestly say Option 1 avoids inventing a second verifier in the
serve path. It makes the table match the predicate the build will implement, and
keeps the existing M1 verifier at the deployer boundary where M1 was specified.
The DDL-revoke exclusion is still tested by F16a/F16b and is sequenced before
0021 is authored in production.

### Result

No M1, M2, M4, or C3 regression remains. The build must still implement the M2
filtered loader exactly before authoring 0021.

## Challenge Attempt 4: A New Completeness Gap Outside The Complete Rows

### Claim Attacked

A common way for the v9 fix to look complete while remaining non-executable
would be to repair row 16 but leave another cursor-state group asserted rather
than derived, or to make F18 too narrow for grouped states such as
`step_committed` and `aborted`.

### Concrete Refutation Re-run

The v9 table still walks all four practical cursor groups: `none`,
`in_progress/step_committed/aborted`, `finalizing`, and `complete`
(`HOLDER.md:674`, `:677`, `:679`, `:681`). Rows 5-8 cover
`in_progress/step_committed/aborted` through A1/A2, rows 9-12 cover `finalizing`
through A1, and rows 1-4 cover the no-transcript case (`HOLDER.md:642`, `:646`,
`:650`). The F18 assertion also names every concrete cursor state, including
`step_committed` and `aborted`, not just the grouped labels (`HOLDER.md:941`).

The only wording I would make the build team treat carefully is the continued
"64 cells" shorthand. The table groups `step_committed` with `in_progress` and
`aborted` with the non-complete edge, so the design row count is 16 row groups x
4 owner buckets (`HOLDER.md:636`). But the executable test should still expand
or explicitly table-drive each concrete cursor-state enum named by F18, because
the implementation will not operate on the prose group label.

### Strongest Holder Rebuttal

The grouping is defensible because those states intentionally share the exact
same A outcome: non-complete cursors halt `awaiting_deploy` before serve, except
for the earlier A0 config gate. Nothing in the row-16 M7 fix changes that group.
F18's text is broad enough to require the concrete enum coverage even if the row
count shorthand stays grouped.

### Result

I do not treat the 64-cell shorthand as a standing falsification. It is an
implementation watchpoint: `T-deploy-bootpath-decision-table` should make the
state expansion unambiguous.

## Carry-Forward Summary

- M7: addressed. Row 16 `==0`, `==20`, and `>=21` are conditional on A3
  fingerprint-sync, F18 is parametric over the seven complete-row A-reaching
  cells, and normal pre-0021 row-16 state is documented out-of-sync.
- M6: intact. Rows 13/15 remain conditional in `==0` exactly like `==20`; the
  four `:399` cells remain identical between section 4.5 and F18.
- M5(row-1): intact. `applied_owner == 0` still serves the fresh no-revoke
  first boot; `1..19` still halts.
- M3: intact. The hoisted config gate still fires before any cursor branch when
  a revoke-embedding binary has decoupling OFF; row 16 is the decoupled verify
  path and does not reach `:399`.
- M4/M2/C3: intact at the spec level. The non-revoke filter, staged F16a/F16b,
  and terminal 0021 ordering are unchanged by M7.
- M1/BC-N1/BC-N2/C1/C2: intact. Immutable transcript, resume/finalizer
  verification, non-complete cursor halt, finalizing boundary, and
  `RequiredOwnerBundleVersion = 20` are not weakened.

I do not find a material carry-forward regression or new complete-row derivation
gap that should stop the v9 revision from clearing.
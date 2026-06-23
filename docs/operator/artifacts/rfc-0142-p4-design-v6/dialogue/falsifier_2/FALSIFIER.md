# FALSIFIER - RFC 0142 P4 v6 owner-ddl/test-staging review

author: falsifier-reviewer-002

## Revision check

The v6 holder genuinely fixes the v5 M4 staging defect. The old break was that
F16 landed with the owner-ddl filter in rollout step 2 while also asserting that
production `OwnerBundles()` already contained 0021, even though 0021 was not
authored until rollout step 7. The v6 text splits that into two phase-aware
checks:

- F16a / `TestOwnerDDLApplyExcludesSyntheticRevokeBundle`, landing in step 2,
  drives the filter through a synthetic bundle list / test hook, asserts
  `OwnerDDLApplyBundles` / `isNonRevokeBundle` exclude every bundle `>= 21`,
  verifies both `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles` skip a
  hand-passed synthetic 0021, and verifies the nil fallback resolves to the
  filtered loader. It explicitly does **not** assert production `OwnerBundles()`
  contains 0021 yet
  (`docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/holder/HOLDER.md:296-309`).
- F16b / `TestOwnerDDLApplyExcludesProductionRevokeBundle`, landing after 0021 is
  authored in step 7, asserts production `OwnerBundles()` contains 0021,
  `ExpectedFingerprint()` includes its bytes, `revokeEmbedded` comes from the
  full loader / file presence, and production `OwnerDDLApplyBundles()` excludes
  it. The forced self-heal pgtest lives in this phase
  (`HOLDER.md:310-316`, `:663-664`, `:674-700`).

That resolves M4 as specified by the v5 collaboration ledger. I also do not find
an M2 regression in the F16 restructuring: the holder keeps the single
`DDLRevokeOwnerBundleVersion = 21` / `isNonRevokeBundle` filter, the split
`OwnerDDLApplyBundles()` loader, in-loop guards on both apply loops, and the
nil-fallback split; `OwnerBundles()` remains the full loader only for
`revokeEmbedded`, `ExpectedFingerprint`, `BuildPlan`, and
`RuntimeOwnedTablesAlterable` (`HOLDER.md:269-294`, `:587-620`). The current CLI
surface still has only `striatum daemon owner-ddl apply`, which calls
`db.ApplyOwnerBundles`; I did not find a sibling owner-ddl dry-run/list route in
current source (`go/pkg/cli/localcommands/daemon.go:76-156`).

I therefore do not keep M4 open and do not claim 0021 is reachable through an
`owner-ddl apply` route under the v6 proposal.

## Challenge: the F18 owner-watermark table is false for fresh/no-authority DBs

### Claim attacked

The v6 seed requires the boot-path decision table to cover every
`cursorState x decoupledEnabled x revokeEmbedded x applied_owner` cell, including
the "legitimate fresh-DB bring-up / inert-landing cells" that must still serve
(`docs/operator/workflows/rfc-0142-p4-design-v6/SEED.md:318-334`). The holder
turns that table into executable F18 coverage, requiring the matrix to assert the
exact section 3.5 outcome for every `applied_owner` bucket
(`HOLDER.md:433-441`, `:666`).

But section 3.5 collapses two distinct owner-watermark states into one `<20`
bucket. It says `CheckOwnerBundleWatermark` maps `applied_owner < 20` to
`awaiting_owner_ddl`, then states that `<20` "ALWAYS halts" and every row in the
`<20` column is uniformly `awaiting_owner_ddl` (`HOLDER.md:443-478`). The prose
then calls cell 1 / `==20` the fresh-DB / inert-landing cell that still serves
(`HOLDER.md:515-518`).

That is not the live watermark contract the proposal claims to carry forward.
Current source makes `applied == 0` the fresh / single-role / no-authority
bootstrap case and returns nil before the shortfall check. Only an
authority-bearing DB with `1 <= applied < RequiredOwnerBundleVersion` is a real
`awaiting_owner_ddl` shortfall (`go/pkg/db/owner.go:116-149`).

### Concrete refutation

Construct the no-revoke inert landing cell the seed says must remain valid:

```text
cursorState = none
decoupledEnabled = false
revokeEmbedded = false
applied_owner = 0
```

Under current source, `OwnerBundleVersion` returns 0 when `owner_bundle_meta` is
absent, `CheckOwnerBundleWatermark` returns nil for that 0, and the boot can take
the existing fresh/single-role legacy path. Under the v6 F18 table, the same cell
is row 1 / `<20` and must halt `awaiting_owner_ddl`.

So an implementation has only two choices:

1. follow the table and regress fresh/no-authority bring-up by halting a DB that
   current source deliberately allows to bootstrap, or
2. keep the `applied == 0` exception and make the F18 matrix oracle false for the
   packet's required `applied_owner <20` bucket.

This is a material owner-column failure, not an M4 staging nit. It directly
touches the packet's "P2 watermark interlock / fresh-DB bring-up" check. The
holder has proven the old M3 `complete` + revoke-embedding + flag-off cell halts,
but the table now misclassifies the owner-watermark bootstrap state.

### Strongest rebuttal

The holder can argue that `<20` was intended to mean "authority-bearing but below
20", with `0/no authority` implicitly outside the table. That would match current
source and would preserve the fresh-DB exception.

The written contract does not say that. The seed and F18 both name
`applied_owner <20` as a matrix bucket, source represents the fresh no-authority
case as applied owner version 0, and section 3.5 says `<20` always halts. The
prose also points to `==20` for fresh bring-up, which is an already-owner-bundled
database, not a fresh/no-authority one.

### Required repair

Split the owner-watermark dimension before the proposal clears:

```text
applied_owner in {0/no authority, 1..19 authority shortfall, ==20, >=21}
```

Then make F18 assert both branches explicitly:

- `cursorState == none`, no-revoke, flag off, `applied_owner == 0` -> serve
  legacy / fresh bootstrap; `ApplyMigrations` and the legacy self-record may run
  because no deploy transcript exists.
- `cursorState == none`, no-revoke, flag off, `1 <= applied_owner < 20` ->
  `awaiting_owner_ddl`, DB untouched.

The same split should be propagated through the other cursor rows so the
decision table remains executable without changing the current bootstrap
contract.

## Verdict

M4 is resolved and M2's owner-ddl exclusion is not regressed by the phase split.
The remaining blocker I can justify from this lens is the proactive-completeness
owner-watermark table: its uniform `<20 -> awaiting_owner_ddl` rule contradicts
the current `applied == 0` fresh/no-authority bootstrap exception and makes F18
either regress a legitimate boot path or fail as an executable oracle. The v6
proposal should not clear until the owner-watermark dimension distinguishes
`0/no authority` from `1..19 authority shortfall`.

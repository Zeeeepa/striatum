# FALSIFIER - RFC 0142 P4 v7 owner-watermark / decision-table review

author: falsifier-reviewer-003

## Revision check

The direct v6 M5 reproducer is fixed. The v7 holder splits the owner-watermark
dimension into `{0/no authority, 1..19 authority shortfall, ==20, >=21}`;
`applied_owner == 0` passes W and row 1/`==0` is now the named fresh-DB
serve-legacy cell, while `1 <= applied_owner < 20` remains
`awaiting_owner_ddl` (`HOLDER.md:454-489`). That matches current source:
`OwnerBundleVersion` returns 0 when `owner_bundle_meta` is absent, and
`CheckOwnerBundleWatermark` returns nil for `applied == 0` before the
`applied < RequiredOwnerBundleVersion` halt (`go/pkg/db/owner.go:116-150,
226-235`; `go/pkg/db/owner_pg_test.go:19`). F18/F18a now explicitly assert the
row-1 `applied_owner == 0` serve cell and the `1..19` halt cell
(`HOLDER.md:723-724`). Cell `==20` is no longer mislabeled as the fresh DB; it
is the inert-landing re-boot cell (`HOLDER.md:506-510`).

I also do not find a direct regression in the carried-forward M3/BC-N2/C2
edges from the row-1 fix. The `revokeEmbedded && !decoupledEnabled ->
awaiting_deploy_config` gate remains A step 0, before every cursor-state branch
including `complete` (`HOLDER.md:352-366`). The `1..19` column halts at W
before A, the `==20` non-complete BC-N2 cells remain `awaiting_deploy`,
`RequiredOwnerBundleVersion` remains 20 (`go/pkg/db/owner.go:23,35`), and the
`>=21` forward-watermark column is carried forward (`HOLDER.md:487-504,
590-614`). M4, M1, M2, BC-N1, C1, and C3 are textually carried forward and I
found no source contradiction in those mechanisms from this lens.

## Standing challenge: the split is not propagated coherently through the `complete` / `applied_owner == 0` cells

### Claim attacked

The holder's structural M5 claim is stronger than "row 1 serves": after W
passes, A does not read `applied_owner`, so the `0` and `==20` columns must have
identical A-gate behavior (`HOLDER.md:359-360,471-480`). F18 then makes the
§3.5 table executable: every cell must produce the exact stated outcome
(`HOLDER.md:723`). §1.3 likewise says a `complete` cursor with a stored plan
that byte-matches the binary and `LiveFingerprint == ExpectedFingerprint`
serves verify-only when decoupled is enabled (`HOLDER.md:223-225`).

The §3.5 table contradicts those claims in the `complete` / no-revoke /
`applied_owner == 0` cells:

- Row 13 (`complete`, flag off, no-revoke) makes the `==0` column
  **`awaiting_deploy`**, while the `==20` column is "SERVE-legacy if in-sync,
  else `awaiting_deploy`" (`HOLDER.md:501`). But the proof below the table then
  admits a row 13/`==0` "degenerate in-sync" path where A3 proves in-sync and the
  legacy `:399` rewrite is idempotent (`HOLDER.md:525-528`). The table outcome,
  the A predicate, and the proof do not agree.
- Row 15 (`complete`, decoupled on, no-revoke) makes the `==0` column
  unconditional `awaiting_deploy`, while the `==20` column is "SERVE-verify if
  in-sync, else `awaiting_deploy`" (`HOLDER.md:503`). But the decoupled A3 branch
  is explicitly `plan_hash == expected` plus `LiveFingerprint == ExpectedFingerprint`
  -> serve verify-only, and A does not read `applied_owner`
  (`HOLDER.md:370-374,359-360`).

### Concrete refutation

Construct the F18 cell:

```text
cursorState = complete
decoupledEnabled = true
revokeEmbedded = false
applied_owner = 0
owner_bundle_meta absent
deploy_plan[plan_hash] present
cursor.plan_hash == expected
LiveFingerprint(recorded) == ExpectedFingerprint()
```

W returns nil because `applied_owner == 0` is the fresh / single-role /
no-authority exception (`go/pkg/db/owner.go:140-146`). A then enters the
`complete` + `decoupledEnabled == true` branch and serves verify-only because
the plan hash and fingerprint are in sync (`HOLDER.md:370-374`). That is also
the §1.3 `complete` / in-sync row (`HOLDER.md:223-225`).

But §3.5 row 15/`==0` requires `awaiting_deploy` (`HOLDER.md:503`), and F18 is
defined as a matrix oracle over the exact §3.5 outcome (`HOLDER.md:723`). So an
implementation following the specified W+A predicates over-serves a
transcript-carrying `applied_owner == 0` complete state that the table says
should halt; an implementation following the table must add an unstated
`complete && applied_owner == 0` halt that contradicts A's owner-watermark
independence.

The same defect exists in the legacy complete row. Row 13/`==0` states
`awaiting_deploy`, but §4.5 admits the in-sync subcase reaches an idempotent
legacy `:399` rewrite (`HOLDER.md:501,682`). F18's spy list, however, allows
`RecordSchemaFingerprint` only in cells 1/`==0`, 1/`==20`, and
13/`==20`-in-sync; it omits the admitted 13/`==0` in-sync case
(`HOLDER.md:723`). Either the table is false, or the oracle is false, or the
predicate is missing a guard.

### Strongest rebuttal

The strongest holder rebuttal is that `complete + applied_owner == 0` is an
inconsistent or unreachable database shape. In the normal P4 activation path,
a completed deploy should have owner bundles applied to version 20 or 21, not
remain at 0, so the row 13/15 `==0` entries are intended as conservative halts
for corruption rather than legitimate serve cells.

That rebuttal does not rescue the written spec. First, §3.5 claims all 64 cells
are specified as executable outcomes; it does not mark `complete + applied==0`
as impossible or corrupt (`HOLDER.md:483-485`). Second, the specified A
predicate has no mechanism to enforce that conservative halt because it does not
read `applied_owner` and it serves a `complete` cursor when the stored plan and
fingerprint match. Third, the holder itself admits the row 13/`==0` in-sync
subcase in the Invariant-B proof (`HOLDER.md:525-528,682`). If the intended
rule is "a complete transcript over `applied_owner == 0` is inconsistent and
must halt," that is a new W/A guard and must be specified, not left as a table
entry contradicted by the predicate.

### Required repair

Propagate the M5 split through the `complete` rows, not only the no-transcript
row. Either:

1. make `applied_owner == 0` mirror `==20` wherever W passes and A is
   owner-watermark independent, so rows 13 and 15 become conditional
   "serve if in-sync, else `awaiting_deploy`"; or
2. explicitly classify `complete + applied_owner == 0` as an inconsistent
   transcript-carrying state, add the guard that detects it before serving, and
   update F18/F18a plus the §4.5 spy allowances to assert that halt.

The current text does neither. This leaves a material owner-watermark
decision-table gap: F18 will either fail the row 15/`==0` in-sync cell or encode
an unstated guard, and a direct implementation of the written W+A predicates
will serve a cell the table says must halt.

## Verdict

The row-1 M5 repair is real: the fresh no-transcript/no-revoke
`applied_owner == 0` bootstrap cell now serves, `1..19` halts, and `==20` is no
longer the mislabeled fresh-DB cell. The M3 config gate and the listed
carry-forwards are not directly regressed by that row-1 fix.

But the v7 §3.5/F18 table is still not executable as written. It says the
`0` and `==20` columns have identical A behavior after W passes, then gives
different outcomes for the `complete` / no-revoke cells and contradicts the
specified A predicate. The revision should not clear until the
`complete` / `applied_owner == 0` cells are made coherent.

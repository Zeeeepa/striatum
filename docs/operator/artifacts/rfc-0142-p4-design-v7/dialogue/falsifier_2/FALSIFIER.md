# FALSIFIER - RFC 0142 P4 v7 carry-forward / regression review

author: falsifier-reviewer-004

## Revision Check

The direct v6 M5 reproducer is addressed. The v7 holder splits the
`applied_owner` dimension into `{0/no authority, 1..19 authority shortfall,
==20, >=21}`; W now serves `applied_owner == 0`, halts only
`1 <= applied_owner < 20`, and labels row 1/`==0` as the fresh-DB bring-up cell
instead of mislabeling row 1/`==20`
(`docs/operator/artifacts/rfc-0142-p4-design-v7/dialogue/holder/HOLDER.md:454-489`).
That matches the live watermark contract: `OwnerBundleVersion` returns 0 when
`owner_bundle_meta` is absent, and `CheckOwnerBundleWatermark` returns nil for
`applied == 0` before the shortfall check (`go/pkg/db/owner.go:116-150,226-235`).
F18/F18a now name both branches explicitly (`HOLDER.md:723-724`).

I also do not find a direct regression in the nine v6-cleared carry-forwards:

- **M3:** the `revokeEmbedded && !decoupledEnabled -> awaiting_deploy_config`
  halt remains A step 0 before every cursor branch, including `complete`; the
  no-revoke `complete` residual remains a pre-`ApplyMigrations` pure-read
  comparison (`HOLDER.md:352-378`).
- **M4:** the F16a synthetic pre-0021 test and F16b production post-0021 test,
  including the forced FMA-007 self-heal through `isCrossBundleDependencyError`,
  are preserved as rollout-step requirements (`HOLDER.md:719-721`).
- **M1:** `VerifyStoredTranscript` still runs on resume and as finalizer step 0,
  with typed binary/DB-stamp mismatch halts (`HOLDER.md:179-213,719-720`).
- **M2/C3:** the non-revoke owner-DDL apply slice and in-loop guard contract are
  carried forward, while 0021 remains deploy-terminal/revoke-last only
  (`HOLDER.md:286-327,643-675`).
- **BC-N1/BC-N2/C1/C2:** the immutable `deploy_plan`, universal non-complete
  cursor halt at `applied_owner == 20`, `finalizing` finalizer,
  `RequiredOwnerBundleVersion = 20`, and `>=21` forward-watermark rule are
  carried forward (`HOLDER.md:179-242,352-378,590-614`;
  `go/pkg/db/owner.go:23-35`).

That said, the M5 split is not propagated coherently through the `complete`
rows. This is a standing decision-table/F18 defect, not a re-opening of the old
row-1 fresh-DB wedge.

## Challenge: F18 Is False For Complete / applied_owner == 0

### Claim Attacked

The holder claims the split is executable over every
`cursorState x decoupledEnabled x revokeEmbedded x applied_owner` cell, and
that once W passes, A does not read `applied_owner`, so the `0` and `==20`
columns have identical A-gate behavior (`HOLDER.md:471-480`). The predicate in
§3.3a says a `complete` cursor whose stored plan and fingerprint match the
binary serves verify-only when decoupled, or serves legacy/no-op when
no-revoke and already in sync (`HOLDER.md:370-378`). F18 then requires the
parametrized matrix to assert the exact §3.5 outcome for all 64 cells
(`HOLDER.md:723`).

The §3.5 table contradicts those claims in the complete/no-revoke
`applied_owner == 0` cells:

- Row 13 (`complete`, flag off, no-revoke) says the `==0` column is
  `awaiting_deploy`, while the `==20` column is "SERVE-legacy if in-sync, else
  `awaiting_deploy`" (`HOLDER.md:501`). The proof immediately below admits a
  "cell 13 / `==0`, degenerate in-sync" path where A3 proves in-sync and the
  legacy `:399` rewrite is idempotent (`HOLDER.md:522-528`).
- Row 15 (`complete`, decoupled on, no-revoke) says the `==0` column is
  unconditional `awaiting_deploy`, while the `==20` column is
  "SERVE-verify if in-sync, else `awaiting_deploy`" (`HOLDER.md:503`). But A3
  on the decoupled path serves verify-only when `plan_hash == expected` and
  `LiveFingerprint == ExpectedFingerprint`; A has no `applied_owner` input
  (`HOLDER.md:359-374`).
- The F18 spy oracle permits `ApplyMigrations`/`RecordSchemaFingerprint` only
  in cells 1/`==0`, 1/`==20`, and 13/`==20`-in-sync, while §4.5 permits the
  "degenerate 13/`==0`" idempotent rewrite (`HOLDER.md:682-701,723`). The test
  oracle and invariant proof do not agree.

### Concrete Refutation

Construct the executable F18 cell:

```text
cursorState = complete
decoupledEnabled = true
revokeEmbedded = false
applied_owner = 0
deploy_plan[plan_hash] present
cursor.plan_hash == expected
LiveFingerprint(recorded) == ExpectedFingerprint()
owner_bundle_meta absent
```

W returns nil because `applied_owner == 0` is the fresh/single-role exception
(`owner.go:145`). A then takes the `complete` + `decoupledEnabled == true`
branch and returns nil because the plan hash and fingerprint are in sync
(`HOLDER.md:370-374`). Under the holder's own predicate, this is a serve-verify
cell.

But §3.5 row 15/`==0` requires `awaiting_deploy`, and F18 is required to assert
the exact §3.5 outcome. So either:

1. F18 is a false oracle for this cell; or
2. the implementation must add an unstated guard that treats
   `complete + applied_owner == 0` as inconsistent, contradicting the written A
   predicate and the claim that W-passing `0` and `==20` have identical A
   behavior.

The same mismatch exists on the legacy complete row. With `decoupledEnabled =
false` and the same in-sync facts, A3's no-revoke comparison returns nil before
`ApplyMigrations`; the holder's proof admits row 13/`==0` is an idempotent
rewrite, but the table headline says `awaiting_deploy` and the F18 spy list
does not permit that write.

### Strongest Rebuttal

The holder can argue that `complete + applied_owner == 0` is not a normal final
P4 state. A successful two-role deploy normally applies owner bundles in the
plan, so a complete cursor should have owner watermark 20 or 21 rather than 0.
On that reading, row 13/`==0` and row 15/`==0` are conservative halts for a
corrupt or unreachable database shape, not legitimate serve cells.

That rebuttal does not rescue the written spec. §3.5 promises exact executable
outcomes for all 64 cells; it does not mark complete/`==0` impossible. A is
explicitly owner-watermark independent, so the specified predicate cannot
produce the conservative halt. The holder also cannot rely on "impossible"
while §4.5 simultaneously admits the degenerate row 13/`==0` in-sync write. If
the intended rule is "a complete transcript with owner watermark 0 is
inconsistent and must halt," the spec needs a real guard and F18 needs to
assert it.

### Carry-Forward Impact

This does not re-open the original M3 complete-cursor mutate+self-record bypass:
the revoke-embedding + flag-OFF cells still halt at A0, and no pending-change
transcript is allowed through the A3 comparison. It also does not weaken
M2/C3, the `==20` BC-N2 edge, `RequiredOwnerBundleVersion = 20`, or the `>=21`
forward-watermark rule.

It is still material because F18 is the carry-forward proof harness for M3,
BC-N2, C2, and M5 together. As written, a build can either implement the §3.3a
predicate and fail the table, or implement the table and smuggle in an unstated
owner-watermark-dependent complete-cursor guard. Either path means the decision
table is not executable as specified.

## Required Repair

Choose one coherent contract and propagate it everywhere:

1. **Mirror `==20` anywhere W passes and A is owner-watermark independent.** Rows
   13 and 15 in the `==0` column become conditional "serve if in-sync, else
   `awaiting_deploy`"; §4.5 and F18's spy list include the row 13/`==0`
   idempotent subcase.
2. **Classify `complete + applied_owner == 0` as inconsistent.** Add the W/A
   guard that detects it before serving, remove the claim that the `0` and
   `==20` columns have identical A behavior in complete rows, and make F18
   assert the typed halt.

Until one of those is specified, the v7 table still has a material
owner-watermark regression. The revision should not clear.

## Verdict

M5 row 1 is genuinely fixed and the nine v6 carry-forward findings are not
directly regressed by the row-1 split. However, the split leaves the
`complete` / `applied_owner == 0` rows internally inconsistent with §3.3a,
§3.5, §4.5, and F18. That is a standing falsification of the executable
decision table.

# FALSIFIER - RFC 0142 P4 v8 decoupling-boundary / decision-table review

author: falsifier-reviewer-003

## Revision Check

The named v7 M6 defect is genuinely repaired in the cells the v8 seed required
this lane to re-check first.

The revised holder now states the load-bearing invariant directly: once W has
passed, A does not read `applied_owner` or `owner_bundle_meta`; it reads
`cursorState`, `decoupledEnabled`, `revokeEmbedded`, `cursor.plan_hash`, and the
recorded `schema_state.fingerprint`, so the `==0` and `==20` columns must take
the same A outcome for the same cursor row and fingerprint-sync facts
(`HOLDER.md:148-188,424-473`). That predicate boundary matches the live source
anchors: `CheckOwnerBundleWatermark` returns nil for `applied == 0`, halts only
the genuine `1..19` shortfall, and keeps `RequiredOwnerBundleVersion` at 20
(`go/pkg/db/owner.go:23-35,124-154`); `OwnerBundleVersion` returns 0 when
`owner_bundle_meta` is absent (`owner.go:226-247`); `LiveFingerprint` reads only
`striatumd.schema_state.fingerprint`, and `RecordSchemaFingerprint` writes only
the `schema_state` singleton (`go/pkg/db/schema_drift.go:145-161,171-195`).

The v7 F18 reproducer cell now has the correct answer. Row 13/`==0` is
conditional, "SERVE-legacy if in-sync, else `awaiting_deploy`", exactly like
`==20`; row 15/`==0` is conditional, "SERVE-verify if in-sync, else
`awaiting_deploy`", exactly like `==20` (`HOLDER.md:586-589`). Section 4.5 and
F18 now agree on the legacy writer: the only four cells that reach
`connection.go:399` are `1/==0`, `1/==20`, `13-in-sync/==0`, and
`13-in-sync/==20` (`HOLDER.md:622-645,804-832,855`). The cross-row audit also
walks `none`, `in_progress`/`step_committed`/`aborted`, `finalizing`, and
`complete`, and explicitly shows `==0` matching `==20` in each group
(`HOLDER.md:598-620`). The v7 M6 row-13/15 contradiction is not still open.

The requested carry-forward checks are also intact from this lens:

- M5 row 1/`==0` still serves the fresh no-authority/no-transcript bootstrap,
  while `1..19` still halts at W (`HOLDER.md:538-546,574`; `owner.go:145-150`).
- M3 remains hoisted at A step 0: `revokeEmbedded && !decoupledEnabled` returns
  `awaiting_deploy_config` before any cursor-state branch, including `complete`
  (`HOLDER.md:434-437,575,579,583,587`).
- The BC-N2 non-complete edge remains `awaiting_deploy` in the `==20` column
  for rows 5, 7, 9, and 11 (`HOLDER.md:578-585`).
- `RequiredOwnerBundleVersion` remains `LatestOwnerBundleVersion`, currently 20
  (`go/pkg/db/owner.go:23-35`), so the fix does not advance the watermark to
  21.
- M1/M2/M4/BC-N1/C1/C2/C3 are carried forward without a direct contradiction
  from the row-13/15 M6 repair.

That does not clear the revision. The full 64-cell table still has a material
decoupling-boundary gap in the adjacent complete/revoke/decoupled row.

## Standing Challenge: Row 16 Still Imports An Unchecked Owner-Stamp Fact

### Claim Attacked

The holder's clearing claim is stronger than "rows 13 and 15 now match." It
claims the entire decision table is mechanically derived from W and A: for every
cell, W decides only the owner-watermark bucket, then any W-passing `==0` and
`==20` cell takes A's outcome (`HOLDER.md:519-566`). A's complete/decoupled
branch is simple and owner-watermark-blind: when `cursorState == complete` and
`decoupledEnabled == true`, `cursor.plan_hash == expected` plus
`LiveFingerprint == ExpectedFingerprint` serves verify-only; otherwise it
returns `awaiting_deploy` (`HOLDER.md:441-445`). A does not read
`owner_bundle_meta` (`HOLDER.md:424-430,467-473`).

Row 16 does not follow that derivation. For `cursorState=complete`,
`decoupledEnabled=true`, `revokeEmbedded=true`, and `applied_owner == 0` or
`==20`, the table gives unconditional `awaiting_deploy`, on the premise that
0021 is not yet applied and therefore the fingerprint is not in sync
(`HOLDER.md:589`). But that premise is not produced by W or A. It is an unstated
cross-table consistency assumption: "if `owner_bundle_meta` is absent or still
20, then `schema_state.fingerprint` cannot equal the 0021-bearing
`ExpectedFingerprint`." The spec's own M6 proof is built on the opposite
mechanical fact: `schema_state` and `owner_bundle_meta` are separate tables, and
A reads only `schema_state` (`HOLDER.md:163-184,467-473,636-638`;
`schema_drift.go:145-161,171-195`).

### Concrete Refutation

Construct the F18 row-16 cell:

```text
cursorState = complete
decoupledEnabled = true
revokeEmbedded = true
applied_owner = 0        # same fork exists for ==20
deploy_plan[plan_hash] present
cursor.plan_hash == expected
LiveFingerprint(recorded) == ExpectedFingerprint()
owner_bundle_meta absent # or version 20
```

W passes for `applied_owner == 0` and for `==20` (`owner.go:145,151-153`). A
then takes the `complete` + `decoupledEnabled == true` branch and returns nil
because the plan hash and fingerprint are in sync (`HOLDER.md:441-445`). The
verify path does not call `ApplyMigrations` or the legacy `connection.go:399`
writer, so this is not a replay of the old Invariant-B legacy-write defect; it
is a serve-verify outcome under the written A predicate.

Section 3.5 instead requires `awaiting_deploy` for row 16 `==0` and `==20`
(`HOLDER.md:589`). F18 says the parametrized matrix asserts the exact table
outcome for every cell, but it only adds an in-sync/out-of-sync sub-dimension
for the complete no-revoke rows 13/15, even though the same complete/decoupled
fingerprint predicate controls row 16 (`HOLDER.md:855`). A direct implementation
of section 3.3a will serve the constructed row-16 cell; a test oracle following
section 3.5 will either fail that implementation or skip the in-sync subcase.

### Why This Is Material

This is not the old v7 M6 "0 differs from ==20" bug; both columns still match.
The remaining problem is that row 16 is asserted from a C3/revoke-last
reachability story rather than derived from the stated boot predicate. The table
treats `applied_owner < 21` as proof that the fingerprint is not in sync, but
the M6 source anchors say fingerprint state is orthogonal to the owner watermark
unless some guard verifies the relationship.

That matters to the product boundary of P4. A revoke-embedding, decoupled binary
serving with `owner_bundle_meta` absent or still 20 has not proven that the
terminal 0021 owner bundle committed, so the "serving role loses DDL" guarantee
is not established by the watermark. The normal deploy path is supposed to make
that state unreachable by applying and stamping 0021 before finalization
(`HOLDER.md:245-278,483-493`; `owner.go:511-541`), but ordinary serve-boot over
an already `complete` cursor does not run `VerifyStoredTranscript`; A checks
only plan hash and the recorded fingerprint (`HOLDER.md:441-445,505-515`).

So the spec has a fork:

1. If row 16 is truly derived from A, then its `==0` and `==20` cells need the
   same conditional wording as A: "SERVE-verify if in-sync, else
   `awaiting_deploy`." The normal reachable pre-0021 case can still be described
   as out-of-sync.
2. If `complete + revokeEmbedded + applied_owner < 21` is meant to be corrupt or
   impossible and must halt even when `schema_state` says in-sync, then the spec
   needs an explicit executable guard before A serves, or a complete-boot
   verification step that proves the terminal owner-step DB stamp actually
   committed. F18 then has to assert that guard, and the W-to-A independence claim
   must be narrowed accordingly.

The current text does neither. It uses the row-13 orthogonality argument when it
needs the degenerate `13/==0` in-sync cell to exist, but it silently rejects the
same construction in row 16 by assuming `owner_bundle_meta < 21` forces
`LiveFingerprint != ExpectedFingerprint`. That is exactly the sort of unmodeled
premise the v8 mechanical-derivation requirement was supposed to remove.

### Strongest Rebuttal

The strongest holder rebuttal is normal-path reachability: under a healthy P4
deploy, a `complete` cursor for a revoke-embedding binary should only exist
after the terminal 0021 bundle applied and stamped `owner_bundle_meta`, so the
only legitimate in-sync row-16 serve cell is `>=21`. In normal operation, the
row 16 `==0` and `==20` entries are shorthand for "0021 is pending, therefore
the fingerprint will not match."

That rebuttal is not enough for the written spec. Section 3.5 and F18 promise
exact executable outcomes for the full cell space. The holder already treats
orthogonal `schema_state`/`owner_bundle_meta` facts as constructible for the
13/`==0` in-sync case (`HOLDER.md:636-638`), and A as specified has no way to
turn the row-16 constructed state into `awaiting_deploy`. If the invariant is
"complete plus revoke-embedded requires owner watermark >=21," that is a new
boot-time guard or verifier, not a consequence of A as currently written.

## Required Repair

Choose one coherent contract and propagate it through section 1.3, section 3.3a,
section 3.5, section 4.5, and F18:

1. Make row 16 `==0` and `==20` conditional on the complete/decoupled A predicate:
   `SERVE-verify if in-sync, else awaiting_deploy`, while documenting that the
   normal reachable state before 0021 is out-of-sync.
2. Or add an explicit consistency guard before A serves, for example
   `cursorState == complete && revokeEmbedded && applied_owner < 21` halts with
   a typed inconsistency, or a complete-boot verifier checks the stored
   owner-step DB stamp for 0021. F18 must assert that guard, and the current
   owner-watermark-independence language must be narrowed to account for it.

## Verdict

The v8 holder genuinely resolves the v7 row-13/15 M6 blocker and preserves the
named carry-forwards from this lens. It still should not clear. Row 16 is not
mechanically derived from W and A as written: it imports an owner-watermark
consistency fact that the specified A predicate does not check. A build can
implement A and fail F18, or implement F18 only by adding an unstated guard.

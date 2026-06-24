# FALSIFIER - RFC 0142 P4 v9 decoupling-boundary / decision-table review

author: falsifier-reviewer-001

## Bottom Line

I do not find a standing M7 decision-table falsification in the v9 holder. The v8
row-16 refutation cell now has the same outcome in section 3.3a, section 3.5,
section 4.5, and F18: row 16 is conditional on A3 fingerprint sync in the
`==0`, `==20`, and `>=21` columns, and F18 is parametric over the complete-row
sub-cases that matter.

That is not a rubber stamp of the build. The build must still implement the
matrix exactly, including the degenerate in-sync row-16 cells over
`owner_bundle_meta` absent / 20 / >=21. But from this lane's assigned
DECOUPLING-BOUNDARY / DECISION-TABLE lens, the written v9 spec answers M7 and I
do not find a new material gap that should stop the revision from clearing.

## Challenge Attempt 1: Reproduce The v8 Row-16 Refutation

### Claim Attacked

The v8 defect was precise: row 16 (`cursorState=complete`, decoupled ON,
revoke-embedding) was written from an owner-watermark reachability premise rather
than from A. The v8 table made `==0` and `==20` unconditional
`awaiting_deploy`, and `>=21` unconditional `SERVE-verify`, while A's decoupled
complete branch only reads `cursor.plan_hash` and `LiveFingerprint ==
ExpectedFingerprint` (`docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/holder/HOLDER.md:558-589`).
The v8 collaboration ledger required Option 1 as an acceptable repair: make row
16 conditional like A, including the `>=21` variant, and make F18 parametric over
all A-reaching complete-row cells (`docs/operator/artifacts/rfc-0142-p4-design-v8/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md:37-42`).

### Concrete Refutation Re-run

The refutation cell remains constructible as a test input:

```text
cursorState = complete
decoupledEnabled = true
revokeEmbedded = true
applied_owner = 0        # same branch for ==20
deploy_plan[plan_hash] present
cursor.plan_hash == expected
LiveFingerprint(recorded) == ExpectedFingerprint()
owner_bundle_meta absent # or version 20
```

Source still supports the orthogonality premise. W passes `applied_owner == 0`
and `==20`; `RequiredOwnerBundleVersion` remains 20
(`go/pkg/db/owner.go:23-35`, `go/pkg/db/owner.go:145-153`).
`LiveFingerprint` reads only `striatumd.schema_state`, and
`RecordSchemaFingerprint` writes only that singleton, not `owner_bundle_meta`
(`go/pkg/db/schema_drift.go:145-161`, `go/pkg/db/schema_drift.go:171-195`).
So the table cannot infer fingerprint mismatch solely from `owner_bundle_meta`.

### v9 Result

The v9 holder now derives this cell from A instead of asserting it:

- Section 0.2 adds the decoupled-complete `revokeEmbedded`-independence and
  derivation-rule-completeness invariants: every A-reaching complete-row cell
  whose A outcome is fingerprint-conditional must be written conditionally
  (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:171-188`).
- Section 3.3a states A's decoupled complete branch reads neither
  `applied_owner` nor `revokeEmbedded`; row 15 and row 16 therefore share the
  same fingerprint-conditional outcome in every W-passing column
  (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:493-502`).
- Section 3.5 row 16 now says `SERVE-verify if in-sync, else awaiting_deploy`
  for `==0`, `==20`, and `>=21`; it also documents the normal pre-0021
  `==0`/`==20` state as out-of-sync and the normal post-0021 `>=21` state as
  in-sync (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:656-657`).
- The explicit v9 change note confirms those three row-16 cells were changed
  from unconditional outcomes to the same conditional form as row 15
  (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:659-666`).
- F18 now names the seven A-reaching complete-row cells `{13/==0, 13/==20,
  15/==0, 15/==20, 16/==0, 16/==20, 16/>=21}` and requires both in-sync and
  out-of-sync sub-cases for each (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:927-941`).

### Strongest Rebuttal

The strongest attempted falsifier rebuttal is that `complete + revokeEmbedded +
applied_owner < 21` might represent a corrupt or impossible state that should
halt even if `schema_state` says in-sync. That was exactly the v8 gap: such a
halt requires an explicit guard or stamp verifier, because A does not read the
owner watermark.

The v9 holder does not smuggle that guard in. It deliberately chooses the other
coherent contract: row 16 is conditional on fingerprint sync, while documenting
that the normal reachable pre-0021 case is out-of-sync. That matches the v8
ledger's permitted Option 1. No real M7 gap remains on this challenge.

## Challenge Attempt 2: The `>=21` Revoke-Embedding Cell

### Claim Attacked

The packet specifically asked whether the `>=21` revoke-embedding complete cell
was also made conditional for full derivation. In v8 it was unconditional
`SERVE-verify`, which was the symmetric half of the same asserted-not-derived
problem.

### Result

The v9 table makes row 16 / `>=21` conditional: `SERVE-verify if in-sync, else
awaiting_deploy` (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:657`).
The derivation text says the `>=21` column reaches A only for a revoke-embedding
binary and therefore takes A's row outcome (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:622-628`).
F18 also requires the `16/>=21` in-sync and out-of-sync sub-cases, with the
normal post-0021 state in-sync but the opposite degenerate corner still tested
(`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:941`).

No standing gap remains here.

## Challenge Attempt 3: Section 4.5 Versus F18 Spy List

### Claim Attacked

A common way to botch the M7 repair would be to add row 16 serve cells to the
legacy `connection.go:399` spy list, confusing serve-verify with legacy
self-record.

### Result

The v9 holder keeps the spy list unchanged: only `1/==0`, `1/==20`,
`13-in-sync/==0`, and `13-in-sync/==20` reach the legacy fingerprint writer
(`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:698-721`).
Section 4.5 states row 16 reaches the legacy writer in no column, and F18
requires the same four-cell spy list with no row-16 additions
(`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:885-917`,
`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:941`).

The M7 fix therefore does not create a section 4.5 / F18 inconsistency.

## Carry-Forward Regression Check

I do not find a regression in the carry-forward items this lane was asked to
verify:

- M3 is still hoisted at A step 0: `revokeEmbedded && !decoupledEnabled` returns
  `awaiting_deploy_config` before every cursor-state branch, including
  `complete` (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:484-489`).
  Section 3.5 cells 2, 6, 10, and 14 still halt `awaiting_deploy_config` in
  every W-passing column (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:642-657`).
- M6 rows 13 and 15 remain conditional in `==0` and match `==20`; row 13 is the
  legacy in-sync/no-op case and row 15 is the verify-only decoupled case
  (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:654-656`).
- The four `connection.go:399` cells remain identical in section 4.5 and F18,
  preserving the row-13 `==0` in-sync repair
  (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:698-721`,
  `docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:941`).
- M5 row 1 is not re-collapsed: `applied_owner == 0` still serves the fresh
  no-authority/no-transcript bootstrap, while `1..19` halts at W
  (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:604-614`,
  `docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:642`).
- BC-N2's non-complete `==20` edge remains `awaiting_deploy` in rows 5, 7, 9,
  and 11 (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:646-653`).
- `RequiredOwnerBundleVersion` is not advanced: source still has
  `LatestOwnerBundleVersion = 20` and `RequiredOwnerBundleVersion =
  LatestOwnerBundleVersion` (`go/pkg/db/owner.go:23-35`), and the holder repeats
  that the M7 fix does not advance Required
  (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:805-813`).
- M1 remains scoped to deploy resume and finalizer step 0, not an unstated
  serve-boot stamp check; the row-16 fix is the conditional cell, not a hidden
  Option 2 verifier (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:566-579`).
- M2/M4/C3 remain carried forward as design obligations: 0021 is excluded from
  owner-ddl apply routes, F16 remains phase-split, and the revoke is terminal
  rather than a watermark-frontier advance (`docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:404-447`,
  `docs/operator/artifacts/rfc-0142-p4-design-v9/dialogue/holder/HOLDER.md:815-845`).

## Residual Build-Phase Obligation

The remaining risk is implementation, not a holder-spec falsification: F18 must
actually be implemented as the spec now says. In particular,
`T-deploy-bootpath-decision-table` must construct the row-16 in-sync and
out-of-sync sub-cases for `applied_owner == 0`, `==20`, and `>=21`, and must
assert that the in-sync row-16 cells serve verify-only without firing the
`ApplyMigrations` or `RecordSchemaFingerprint` spies. If the build omits those
cases or silently treats row 16 as a normal-only shortcut, that would recreate
M7 in code. The v9 text itself, however, names the required cases clearly.

## Verdict

M7 is genuinely resolved from the decoupling-boundary / decision-table lens. The
v8 F18 refutation cell now produces the same conditional outcome in A, section
3.5, section 4.5, and F18. I found no M3, M6, M5(row-1), BC-N2, or
RequiredOwnerBundleVersion regression, and no new complete-row cell that remains
asserted instead of derived. This falsifier does not stop the v9 revision from
clearing on M7.

# FALSIFIER - RFC 0142 P4 v8 carry-forward / regression review

author: falsifier-reviewer-004

## Revision Check

The v8 holder genuinely addresses the named v7 M6 defect for the cells the
ledger prescribed. The spec now states the load-bearing invariant explicitly:
after W (`CheckOwnerBundleWatermark`) returns nil, A (`CheckDeployActivation`)
does not read `owner_bundle_meta` / `applied_owner`; it reads cursor state,
config, plan hash, and `schema_state.fingerprint` instead
(`HOLDER.md:150-184`, `HOLDER.md:424-473`). The current source anchors match
that predicate split: W returns nil for `applied == 0`, keeps the shortfall at
`1..19`, and keeps `RequiredOwnerBundleVersion = 20`
(`go/pkg/db/owner.go:23-35`, `go/pkg/db/owner.go:116-154`), while
`LiveFingerprint` and `RecordSchemaFingerprint` only touch
`striatumd.schema_state` (`go/pkg/db/schema_drift.go:83-195`).

The original M6 reproducer is repaired. Rows 13 and 15 in the `==0` column now
say "serve if in-sync, else `awaiting_deploy`", matching the `==20` column
(`HOLDER.md:586-588`). The degenerate row-13 `==0` idempotent legacy
`:399` rewrite is present in both the section 4.5 invariant and F18's spy list:
the legacy writer is allowed only in `1/==0`, `1/==20`,
`13-in-sync/==0`, and `13-in-sync/==20` (`HOLDER.md:804-832`,
`HOLDER.md:855`). The cross-row audit also walks `none`,
`in_progress`/`step_committed`/`aborted`, `finalizing`, and `complete` rows and
shows the `==0` and `==20` columns matching (`HOLDER.md:598-620`).

I do not find a direct regression in the required carry-forward set:

- M5 row 1 remains split into `{0/no authority, 1..19, ==20, >=21}`; row
  `1/==0` still serves the fresh no-transcript bring-up, `1..19` still halts at
  W, and `==20` is still the inert-landing serve cell (`HOLDER.md:538-574`,
  `HOLDER.md:856`).
- M3 remains hoisted at A step 0: `revokeEmbedded && !decoupledEnabled` returns
  `awaiting_deploy_config` before any cursor branch, including `complete`, and
  the no-revoke complete path remains a pure pre-`ApplyMigrations` comparison
  (`HOLDER.md:434-452`, `HOLDER.md:587`, `HOLDER.md:854`).
- M4 remains a phase split: F16a is synthetic/pre-0021, while F16b is the
  production/post-0021 assertion that includes the FMA-007 self-heal path
  (`HOLDER.md:853`, `HOLDER.md:860-891`).
- M1 remains a full stored-transcript verifier on resume and as finalizer step
  0, with typed binary and DB-stamp mismatch halts (`HOLDER.md:269-275`,
  `HOLDER.md:850-852`).
- M2 and C3 remain stated as design obligations: 0021 is special-cased,
  deploy-terminal, and excluded from owner-DDL apply routes, while the full
  owner-bundle loader remains available only to the revoke/fingerprint/plan
  surfaces (`HOLDER.md:787-802`, `HOLDER.md:853`). Current source is still
  pre-P4 here, so these are future implementation requirements, not landed
  Go surfaces.
- BC-N1, BC-N2, C1, and C2 remain textually intact: immutable `deploy_plan`,
  resume off the stored transcript, the universal non-complete cursor halt at
  `applied_owner == 20`, `finalizing` plus idempotent finalizer,
  `CheckDeployActivation` before `ApplyMigrations`, `RequiredOwnerBundleVersion
  = 20`, and the no-revoke `>=21` forward-watermark barrier
  (`HOLDER.md:245-300`, `HOLDER.md:438-452`, `HOLDER.md:578-585`,
  `HOLDER.md:862-891`).

That does not clear the revision. The M6 propagation exposes a still-unmodeled
complete-row cell: row 16 imports a C3/revoke-last reachability fact into the
table, but the written A predicate does not check that fact on serve-boot.

## Standing Challenge: Row 16 Is Not Derived From A

### Claim Attacked

The holder's v8 clearing claim is that the entire 64-cell table is derived from
W and A, not asserted ad hoc. Section 3.5 says every W-passing `==0`/`==20`
cell takes A's outcome, and where A is conditional on fingerprint sync the cell
is written conditionally (`HOLDER.md:521-566`). A's complete/decoupled branch is
also explicit: when `cursorState == complete` and `decoupledEnabled == true`,
`plan_hash == expected` plus `LiveFingerprint == ExpectedFingerprint` serves
verify-only; mismatch returns `awaiting_deploy` (`HOLDER.md:441-445`). That
predicate does not read `applied_owner`.

Row 16 contradicts that derivation. For `cursorState=complete`,
`decoupledEnabled=true`, `revokeEmbedded=true`, and `applied_owner == 0` or
`==20`, the table unconditionally returns `awaiting_deploy` because "0021 not
yet applied" implies `fingerprint !=` / not in-sync (`HOLDER.md:589`). That
implication is not produced by W or A. It is a separate consistency premise:
if the terminal revoke bundle is not stamped, the recorded schema fingerprint
cannot match the revoke-embedding binary.

The v8 M6 proof itself says that premise cannot be inferred from the owner
watermark alone. `schema_state` and `owner_bundle_meta` are orthogonal tables;
A reads the former and W reads the latter (`HOLDER.md:163-184`,
`HOLDER.md:467-473`; `go/pkg/db/schema_drift.go:145-195`). The holder used that
orthogonality to make rows 13 and 15 conditional. Row 16 needs the same
treatment, or it needs a named guard that makes the extra consistency premise
executable.

### Concrete Refutation

Construct the row-16 cell that F18 claims to cover:

```text
cursorState = complete
decoupledEnabled = true
revokeEmbedded = true
applied_owner = 0        # same issue for ==20
deploy_plan[plan_hash] present
cursor.plan_hash == expected
LiveFingerprint(recorded) == ExpectedFingerprint()
owner_bundle_meta absent # or stamped only through version 20
```

W passes for both `applied_owner == 0` and `==20`
(`go/pkg/db/owner.go:145-153`). A then takes the complete + decoupled branch and
returns nil because the plan hash and fingerprint are in sync
(`HOLDER.md:441-445`). This is a verify-only serve path: it does not call
`ApplyMigrations` and it does not reach the legacy `connection.go:399` writer,
so it is not the old M3 legacy mutate+self-record bypass.

The table instead says row 16 `==0` and `==20` are `awaiting_deploy`
(`HOLDER.md:589`). F18 says the matrix must assert the exact section 3.5 outcome
for every cell, but it only adds an in-sync/out-of-sync subdimension for rows 13
and 15 (`HOLDER.md:855`). A build that implements the written A predicate will
serve the constructed row-16 in-sync cell; a test oracle that implements the
written table will halt it or omit the in-sync subcase. Either way, the table is
not mechanically derived from W+A.

### Carry-Forward Impact

This is not a regression of M5 row 1, M3, or BC-N2. The fresh `1/==0` serve
still stands; revoke-embedding with the flag off still halts at A0; and the
non-complete `applied_owner == 20` edge remains `awaiting_deploy`. It also does
not let any `owner-ddl apply` route commit 0021 early.

The material pressure is on C3, M1, and F18:

- C3 says 0021 is terminal and revoke-last, so the normal steady state for a
  revoke-embedding deployment should be `applied_owner >=21`. Row 16 relies on
  that reachability story to conclude `applied_owner <21` cannot be in sync, but
  A does not verify the owner-bundle stamp before serving.
- M1's stored-transcript DB-stamp verification is specified on deploy resume and
  finalizer step 0, not on ordinary serve-boot over an already `complete` cursor
  (`HOLDER.md:269-275`, `HOLDER.md:850-852`). If `complete + revokeEmbedded +
  applied_owner <21` is meant to be corrupt, A needs a boot-time consistency
  guard or a complete-boot verifier that checks the stored owner-step stamp.
- F18 is the executable proof harness for the decision table. As written, it
  imports an unstated invariant into row 16: `owner_bundle_meta <21` implies
  fingerprint mismatch. That is exactly the kind of table-only premise M6 was
  supposed to eliminate.

### Strongest Rebuttal

The strongest holder rebuttal is normal-path reachability. A healthy P4 deploy
that reaches `complete` under a revoke-embedding binary should have already
applied and stamped terminal bundle 0021, so row 16 with `applied_owner == 0` or
`==20` is corrupt or unreachable. In that reading, the row's `awaiting_deploy`
wording is shorthand for "0021 is pending, so the fingerprint will not be in
sync."

That rebuttal is not enough for the written spec. The v8 table promises exact
executable outcomes for all cells, and its own invariant rejects unwritten
reachability assumptions when deriving A outcomes. If this row is really a
corruption check, the design must name the check and the typed halt. Without
that guard, the specified A predicate can only see a matching plan hash and a
matching `schema_state` fingerprint, and therefore serves verify-only.

## Required Repair

Choose one coherent contract and propagate it through section 1.3, section
3.3a, section 3.5, section 4.5, and F18:

1. Make row 16 `==0` and `==20` conditional on A's complete/decoupled predicate:
   `SERVE-verify if in-sync, else awaiting_deploy`, while documenting that the
   normal reachable pre-0021 case is out of sync.
2. Add an explicit consistency guard before A serves, for example
   `cursorState == complete && revokeEmbedded && applied_owner < 21` halts with
   a typed inconsistent-state error, or A runs enough stored-transcript DB-stamp
   verification to prove the terminal owner step actually committed. Then narrow
   the W->A independence claim and make F18 assert the guard.

## Verdict

The v8 holder fixes the original row-13/15 M6 defect and keeps the named
carry-forward findings intact in the written design. It still should not clear.
Row 16 remains asserted from a C3 reachability assumption rather than derived
from W and A, so the 64-cell table is not executable as written.

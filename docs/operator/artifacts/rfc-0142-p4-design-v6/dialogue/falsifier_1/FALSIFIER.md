# FALSIFIER - RFC 0142 P4 v6 decoupling-boundary review

author: falsifier-reviewer-001

## Revision check

The v6 holder genuinely closes the v5 M3 reproducer. In v5, the
`cursorState == complete` branch returned nil before the
`revokeEmbedded && !decoupledEnabled` halt, so a revoke-embedding binary with
`STRIATUM_DEPLOY_DECOUPLED` off could reach `ConnectAndMigrate` over a DB that
already carried `deploy_cursor` / `deploy_plan`. Current source order makes that
old break material: `ConnectAndMigrate` runs `CheckOwnerBundleWatermark`, then
`ApplyMigrations`, then the shadow drift gate, and finally
`RecordSchemaFingerprint` (`go/pkg/db/connection.go:349-399`).

The revised §3.3a fixes that specific hole. The config gate is now step 0,
before every cursor-state branch: `revokeEmbedded && !decoupledEnabled` returns
`awaiting_deploy_config` DB-untouched for `none`, `in_progress`, `finalizing`,
and `complete`
(`docs/operator/artifacts/rfc-0142-p4-design-v6/dialogue/holder/HOLDER.md:349-355`).
The `complete` branch no longer short-circuits the gate; a no-revoke legacy
binary gets a pre-`ApplyMigrations` pure-read comparison and serves only when
`ExpectedFingerprint() == LiveFingerprint(recorded)` and `cursor.plan_hash ==
expected` (`HOLDER.md:362-378`). The decision table then marks the old M3 cell
(`complete`, flag off, revoke-embedding) as `awaiting_deploy_config`, including
the post-deploy steady-state `applied_owner >= 21` case (`HOLDER.md:475-497`).
F17 is sharp: it asserts `awaiting_deploy_config`, `ApplyMigrations` uncalled,
`RecordSchemaFingerprint` uncalled, `schema_state` unchanged, DB byte-identical,
and the shadow-mode fall-through never reached (`HOLDER.md:665`).

So I do not keep M3 open. The complete-cursor legacy mutate+self-record bypass is
materially repaired, Universal Invariant B is tightened around the legacy
`connection.go:399` writer (`HOLDER.md:622-642`), and the BC-N2 non-complete
edge is not weakened (`HOLDER.md:356-360`). C2's required owner frontier remains
20, and the forward-watermark rule remains anchored at applied owner >= 21
(`HOLDER.md:540-558`; `go/pkg/db/owner.go:23,35`). I also did not find a
regression in M1's deployer verifier, C1's gated finalizer, or the M2/C3
revoke-last path from this lens.

## Challenge: the decision table wedges the fresh/no-authority bootstrap cell

### Claim attacked

The seed requires the proactive-completeness table to cover the legitimate
fresh-DB / inert-landing cells and prove they still serve, not merely prove the
old M3 cell halts. It explicitly includes `applied_owner <20` in the matrix and
calls out "the legitimate fresh-DB bring-up / inert-landing cells (no-revoke
binary, no transcript) that must still serve and NOT be wedged by the
conservative M3 halt"
(`docs/operator/workflows/rfc-0142-p4-design-v6/SEED.md:318-331`). It also
states that lifting `ApplyMigrations` must not break fresh-DB bring-up
(`SEED.md:367-370`).

The v6 holder contradicts that requirement in §3.5. It says W =
`CheckOwnerBundleWatermark` maps `applied_owner < 20` to `awaiting_owner_ddl`
and then states that the `<20` column is uniformly `awaiting_owner_ddl`
(`HOLDER.md:443-459`). In row 1, the only serving no-revoke/no-transcript legacy
cell is `applied_owner ==20`; the `<20` cell halts (`HOLDER.md:461-464`). The
follow-on prose doubles down: "Cell 1/==20" is the legitimate fresh/inert cell
that still serves (`HOLDER.md:515-518`).

That is not current source behavior, and it is not the bootstrap contract the
seed told v6 to preserve. Today `OwnerBundleVersion` returns 0 when
`owner_bundle_meta` is absent (`go/pkg/db/owner.go:228-236`), and
`CheckOwnerBundleWatermark` documents that applied 0 means "fresh single-role
database" and must not halt (`owner.go:116-123`). The code then returns nil for
`applied == 0` before the `< RequiredOwnerBundleVersion` shortfall check
(`owner.go:140-149`). The pgtest suite asserts a fresh migrated database starts
with owner bundle version 0 (`go/pkg/db/owner_pg_test.go:19-20`).

### Concrete refutation

Construct the cell the seed says must remain legitimate:

```text
cursorState = none       # no deploy_cursor / no deploy transcript
decoupledEnabled = false # inert landing, legacy boot
revokeEmbedded = false   # no 0021 yet
applied_owner = 0        # no owner_bundle_meta / no authority schema yet
```

Under current source and the carried-forward P2 bootstrap exception,
`CheckOwnerBundleWatermark` returns nil for applied 0. §3.3a step 4 then returns
nil for `cursorState == none`, flag off, no-revoke, and the legacy
`ConnectAndMigrate` path performs the normal fresh-DB bring-up
(`HOLDER.md:379-385`).

Under the v6 §3.5 executable table, the same cell is row 1 / `<20` and must
return `awaiting_owner_ddl`. F18 then requires the parametrized matrix to assert
the exact §3.5 outcome for every `applied_owner <20` cell (`HOLDER.md:666`). An
implementation that follows F18 literally will either:

1. change the source to halt a fresh no-authority DB at `awaiting_owner_ddl`,
   regressing fresh-DB / single-role bootstrap, or
2. preserve the current `applied == 0` exception, causing the executable decision
   table and its F18 expected outcome to be false.

Either branch is a material table-correctness failure. This is not a re-opening of
M3; the hoisted M3 gate remains right for revoke-embedding binaries. The bug is
that the table collapses two different `<20` states:

- `applied_owner == 0`: no authority schema / bootstrap, legitimate no-revoke
  no-transcript legacy serve.
- `1 <= applied_owner < 20`: authority-bearing DB lagging the required owner
  frontier, legitimate `awaiting_owner_ddl` halt.

The current table has only one `<20` column, so it cannot state both outcomes.

### Strongest rebuttal

The holder can argue that "applied_owner <20" in §3.5 was intended to mean an
authority-bearing database with some owner bundle applied but below the required
frontier, not the `0` bootstrap case. That interpretation would preserve the
existing source exception and keep the M3 fix conservative only where it matters.

But that is not what the text says. The seed's matrix explicitly uses
`applied_owner <20`, and the current source explicitly classifies 0 as a member
of the owner-watermark state space. The holder also names "fresh-DB bring-up" in
the same decision-table proof, but points to cell 1/`==20`, which is not a fresh
no-authority database. Because F18 is supposed to be executable over every
`applied_owner ∈ {<20, ==20, >=21}` cell, ambiguity here becomes a wrong test
oracle or a wrong boot behavior.

### Required repair

Split the watermark dimension instead of using a single `<20` bucket:

```text
applied_owner ∈ {0/no authority, 1..19 authority shortfall, ==20, >=21}
```

Then specify the no-transcript/no-revoke/flag-off bootstrap cell as serve-legacy
for `applied_owner == 0`, while retaining `awaiting_owner_ddl` for
authority-bearing `1..19`. F18 should assert both cells explicitly:

- `cursorState == none`, no-revoke, flag off, `applied_owner == 0` -> legacy
  fresh bootstrap may call `ApplyMigrations` / `RecordSchemaFingerprint` because
  no deploy transcript exists.
- `cursorState == none`, no-revoke, flag off, `1 <= applied_owner < 20` ->
  `awaiting_owner_ddl`, DB untouched.

The prose at `HOLDER.md:515-518` should stop calling cell 1/`==20` the fresh-DB
cell unless the spec deliberately drops the current `applied == 0` bootstrap
exception, which would need to be called out as a product behavior change and
would violate the seed's "must still serve" requirement.

## Verdict

M3 itself is resolved: a revoke-embedding binary with the flag off cannot reach
the legacy mutate/self-record path, including on a `complete` cursor, and the
named F17/F18 coverage is aimed at the right v5 failure.

A new material gap remains in the proactive-completeness table. The table's
uniform `<20 -> awaiting_owner_ddl` rule contradicts the current
`applied == 0` bootstrap exception and the seed's explicit fresh-DB requirement.
As written, the F18 matrix either wedges legitimate fresh/no-authority bring-up or
becomes an executable oracle that the implementation must violate to preserve
bootstrap. The gate should require the watermark dimension split above before the
v6 proposal clears.

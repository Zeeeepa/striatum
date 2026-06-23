# FALSIFIER - RFC 0142 P4 v4 complete-cursor legacy-apply gap

author: falsifier-reviewer-004

## Revision check: BC-N2 and carry-forward constraints

The v4 holder genuinely closes the specific BC-N2 pre-terminal-revoke hole from
v3, once the ordinal drift is re-anchored. The v3 reproducer named
`applied_owner == 19` because the revoke was bundle 0020 and the non-revoke
frontier was 19. Current `main` has `0020_owner_bundle_watermark_read.sql`, so
v4 correctly moves the pre-revoke window to `applied_owner == 20` and the
terminal revoke to 0021
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:36-42`,
`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:79-89`).

The actual BC-N2 mechanism is no longer merely claimed. `CheckDeployActivation`
is specified as a universal edge for every deployer-aware binary, not gated on
`revokeEmbedded`: it reads `deploy_cursor` immediately after
`CheckOwnerBundleWatermark` and before both `ApplyMigrations` and
`RecordSchemaFingerprint`; `in_progress`, `step_committed`, and `finalizing`
return `awaiting_deploy` DB-untouched for no-0021 binaries as well as
revoke-embedding binaries
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:404-421`).
F11 now includes the missing no-0021 case at `applied_owner == 20`, with spies
asserting `ApplyMigrations` and `RecordSchemaFingerprint` are not entered
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:763`).
So I do not use the old "no-revoke binary serves an incomplete pre-revoke deploy"
as the standing blocker.

BC-N1's original moved-frontier key defect is also addressed for the v3
reproducer. The new `deploy_plan` stores an immutable transcript with base and
target frontiers, terminal-revoke placement, and every step's index/id/role/SHA
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:164-180`).
The deployer materializes that row and `deploy_cursor -> in_progress(0)` before
step 0 mutates any frontier
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:193-200`),
and receipts are keyed from the stored transcript rather than a recomputed
pending plan
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:517-549`).

C1 and C3 are structurally carried forward: `finalizing` remains non-serving and
`complete` is still the last cursor write
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:210-222`,
`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:562-570`);
0021 is terminal, deploy-plan-only, and sorted after every runtime ownership
reconcile while `striatumd_rw` still holds CREATE
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:340-358`,
`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:458-515`).

The standing gap below is a C2 regression introduced by the v4 predicate order.
It is not the v3 BC-N2 no-revoke hole; it is the revoke-embedding binary with a
previously `complete` cursor and `STRIATUM_DEPLOY_DECOUPLED` accidentally or
deliberately OFF.

## Challenge: a revoke-embedding binary can still reach legacy `ApplyMigrations` after a complete prior deploy

### Claim attacked

The holder claims C2 is carried forward: a revoke-embedding binary never reaches
legacy schema mutation before the deployer is active, and F11 explicitly requires
the revoke-embedding + flag-OFF + pending-runtime case to halt before
`ApplyMigrations`:

- C2 disposition says `CheckDeployActivation` remains after
  `CheckOwnerBundleWatermark` and before `ApplyMigrations`, typed halts are
  preserved, `RequiredOwnerBundleVersion` stays 20, and the BC-N2 edge is an
  addition rather than a replacement
  (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:67`).
- The activation binary is described as embedding 0021 while `Latest =
  Required` still stays 20, and on flag-OFF boot it should halt
  `awaiting_deploy_config` rather than legacy auto-apply
  (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:686-689`).
- F11 case (d) requires: "revoke-embedding binary + flag OFF + pending runtime ->
  `awaiting_deploy_config`, `ApplyMigrations` NOT called"
  (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:763`).

But the actual §3.3a predicate only places the `revokeEmbedded && flag OFF`
`awaiting_deploy_config` halt inside the `cursorState == none` branch:

```text
cursorState == complete -> return nil; let the drift gate decide
cursorState == none and decoupledEnabled == false and revokeEmbedded == true
  -> awaiting_deploy_config
```

That is the wrong order for C2. A completed prior deploy is the normal steady
state after 0021 has revoked CREATE. If a later revoke-embedding binary boots
with the decoupling flag OFF, the predicate returns nil on the `complete` cursor
before it considers the flag/config halt
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:424-440`).

### Concrete refutation

Use the first successful v4 activation deploy as the setup:

```text
owner_bundle_meta max = 21
deploy_cursor.state = complete
schema_state records the completed P4 plan
striatumd_rw no longer has CREATE on schema striatumd
```

Now ship a later P4-era binary that still embeds the 0021 revoke marker and has a
new pending runtime migration. This is ordinary P4 scope: after the one-shot
deployer lands, future schema changes still arrive as runtime migrations and
owner bundles, but should be applied by `striatum daemon deploy`, not by serving
boot.

If `STRIATUM_DEPLOY_DECOUPLED` is OFF on that boot, §3.3a does this:

1. `CheckOwnerBundleWatermark` sees applied owner watermark 21. The v4
   forward-watermark branch only refuses no-revoke binaries; a revoke-aware
   binary "still tolerates forward across the boundary" and is governed by
   `CheckDeployActivation`
   (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:640-648`).
2. `CheckDeployActivation` reads the cursor as `complete` and returns nil, saying
   foreign plan/fingerprint mismatch is left to the established Layer 3 drift gate
   (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:424-427`).
3. On the flag-OFF path, "the established Layer 3 drift gate" is too late.
   Current source runs `ApplyMigrations` immediately after the watermark check and
   before `CheckSchemaDrift` or `RecordSchemaFingerprint`
   (`go/pkg/db/connection.go:341-353`,
   `go/pkg/db/connection.go:376-402`). `CheckDeployActivation` is inserted in that
   same gap; returning nil means the legacy runtime mutator runs.

So the later revoke-embedding binary can enter `ApplyMigrations` as the serving
runtime role after 0021 has removed `CREATE`. If the pending runtime migration
creates a table/index/sequence, it hits the same raw privilege failure shape P4
is supposed to eliminate. The C3 prerequisite is explicit in the source: the new
owner must hold `CREATE ON SCHEMA striatumd`, and bundles 0018/0019 grant it
before ownership transfer for that reason
(`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:64-72`,
`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:97-103`,
`go/pkg/db/sql/owner/0019_supervisor_pointer_runtime_ownership.sql:53-80`). Bundle
0021 revokes exactly that create privilege
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:605-610`).

If the migration happens not to need CREATE, this is still a violation: the
serve path has mutated schema after P4. If the P3 drift gate notices anything
afterward, it is shadow by default unless `STRIATUM_SCHEMA_DRIFT_REFUSE` is set,
and `ConnectAndMigrate` then self-records the running binary's expected
fingerprint (`go/pkg/db/schema_drift.go:239-274`,
`go/pkg/db/connection.go:384-402`). Either way, F11(d)'s "ApplyMigrations NOT
called" assertion is false for the `complete`-cursor case.

### Why this is material

This is a C2 carry-forward regression, not a cosmetic test hole. The original
C2 point was to stop a revoke-embedding binary from reaching runtime
`ApplyMigrations` before the deployer is the only mutator. V4 preserves that
for `cursorState == none` and for non-`complete` cursors, but misses the most
important steady state: a completed prior deploy, where CREATE is already
revoked and future changes must go through `deploy`.

It also undercuts the BC-N2 repair's stated invariant. The holder says
"activation binary embeds the 0021 file ... but never auto-applies"
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:664-671`).
That is only true if `revokeEmbedded && !decoupledEnabled` halts before the
`complete` cursor returns nil. As written, it does not.

### Strongest rebuttal on the Holder's behalf

The strongest rebuttal is that a `complete` cursor plus matching fingerprint is
an in-sync database, so returning nil is safe for the same binary that just
completed the deploy. That is true for the immediate restart in the happy-path
choreography when the operator restarts with `STRIATUM_DEPLOY_DECOUPLED=1`.

It does not answer the actual F11(d) case. A later binary with pending runtime
steps has a `complete` cursor for the previous plan, not for its new expected
plan. The spec itself includes "revoke-embedding binary + flag OFF + pending
runtime" in F11, and current `ConnectAndMigrate` cannot let the drift gate decide
unchanged because unchanged means drift is checked after `ApplyMigrations`.

### Required repair

1. Move the `revokeEmbedded && !decoupledEnabled` check ahead of the
   `cursorState == complete` fall-through. For a revoke-embedding binary on the
   legacy path, return `awaiting_deploy_config` DB-untouched regardless of
   whether the cursor is `none`, `idle`, or `complete`.
2. If the holder wants to allow an immediate flag-OFF restart after a completed
   deploy, it must prove there are no pending runtime/owner changes before
   `ApplyMigrations`; that is a new pre-apply verification gate, not the existing
   post-apply drift gate.
3. Extend F11 with the missing state: revoke-embedding binary, flag OFF,
   `deploy_cursor.state == complete` for an older plan, applied_owner >= 21,
   pending runtime migration. Assert `awaiting_deploy_config`,
   `ApplyMigrations` not called, `RecordSchemaFingerprint` not called, DB
   byte-identical.
4. Add the game-day case: after a successful 0021 deploy, roll a later binary
   with a new object-creating runtime migration and intentionally omit
   `STRIATUM_DEPLOY_DECOUPLED=1`. The result must be the typed config halt, not
   a runtime-role `42501`, not a mutation, and not a shadow self-record.

## Verdict

BC-N2's no-revoke pre-revoke window is genuinely fixed in v4, re-anchored to
`applied_owner == 20`. BC-N1's original moved-frontier receipt-key defect is
also genuinely addressed by the stored transcript. C1 and C3 remain coherent as
design mechanisms.

C2 is not carried forward intact as written. The §3.3a predicate lets a
revoke-embedding binary with a `complete` cursor and the decoupling flag OFF
fall through to legacy `ConnectAndMigrate`, where current source calls
`ApplyMigrations` before drift verification. That recreates the post-revoke
serve-boot mutator path and can produce the same #512-class lockout P4 is meant
to remove. This is a standing falsification until the flag/config halt covers
the `complete` steady state before any legacy apply can run.

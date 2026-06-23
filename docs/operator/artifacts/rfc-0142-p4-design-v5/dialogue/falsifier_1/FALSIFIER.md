# FALSIFIER - RFC 0142 P4 v5 complete-cursor self-record bypass

author: falsifier-reviewer-003

## Revision check

The v5 holder genuinely resolves the specific v4 M1 reproducer for deploy-plan
steps. The new `VerifyStoredTranscript(plan_hash)` is specified over every stored
step, including already-applied and pending entries, and it compares those step
SHAs to the running binary before any resume apply. For already-applied entries it
also verifies `schema_migrations.sha256` / `owner_bundle_meta.sha256` against the
stored transcript. The finalizer runs the same verifier as step 0, before the
`complete` receipt, `RecordSchemaFingerprint`, or `finalizing -> complete`
(`docs/operator/artifacts/rfc-0142-p4-design-v5/dialogue/holder/HOLDER.md:84`,
`:214-226`, `:560-568`, `:605-645`). F15 directly covers the v4 `A45`/`B45`
case and the symmetric owner-step case (`HOLDER.md:836`). So I do not claim the
v4 already-applied deploy-step mismatch still reproduces inside the deployer
resume/finalizer path.

M2 also appears pinned at the design level: v5 names a single
`OwnerDDLApplyBundles()` filtered slice, in-loop `isNonRevokeBundle` guards for
both `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles`, and an F16 test that
forces the FMA-007 self-heal branch (`HOLDER.md:85`, `:362-420`, `:837`). I did
not find a new owner-ddl path that applies 0021 early.

BC-N1's stored-plan identity, BC-N2's non-complete cursor halt, C1's finalization
boundary, and C3's revoke-last mechanism are otherwise carried forward coherently.
The blocker below is narrower: v5's proactive hardening still leaves a
self-record path around `VerifyStoredTranscript` when the cursor is already
`complete` and the process is on the legacy `ConnectAndMigrate` path.

## Challenge: `complete` cursor + flag OFF can bypass `VerifyStoredTranscript`

### Claim attacked

The proactive hardening table says every fingerprint/self-record path is audited,
and specifically scopes the legacy boot self-record at `connection.go:399` to the
case where the running binary is recording its own just-applied schema with "no
transcript, cursor absent" (`HOLDER.md:789-806`). It also says the decoupled
verify path never self-records, while the deployer finalizer is gated by
`VerifyStoredTranscript`.

But the §3.3a predicate contradicts that scoping. `CheckDeployActivation` is
inserted before `ApplyMigrations` and `RecordSchemaFingerprint`, but it returns
nil immediately when `cursorState == complete`, deferring to the drift gate
(`HOLDER.md:464-482`). The `revokeEmbedded && !decoupledEnabled` config halt is
only in the `cursorState == none` branch (`HOLDER.md:483-486`). Therefore a
deployer-aware binary with a complete deploy transcript and
`STRIATUM_DEPLOY_DECOUPLED` off can take the legacy `ConnectAndMigrate` path over
a database that does have `deploy_cursor` / `deploy_plan`.

Current source order makes that material: `ConnectAndMigrate` checks the owner
watermark, then calls `ApplyMigrations` at `go/pkg/db/connection.go:349-353`, only
then runs `CheckSchemaDrift` at `:376-383`, and finally calls
`RecordSchemaFingerprint` at `:399`. `RecordSchemaFingerprint` writes this
binary's `ExpectedFingerprint()` (`go/pkg/db/schema_drift.go:83-100`, `:171-195`);
`LiveFingerprint` later reads the singleton rather than recomputing from
`schema_migrations` or `owner_bundle_meta` (`schema_drift.go:145-161`).

### Concrete refutation

Setup:

```text
A successful P4 deploy has completed:
  deploy_cursor.state = complete
  deploy_plan[plan_hash] exists
  owner_bundle_meta max >= 21
  schema_state records the finalizer's fingerprint
  striatumd_rw no longer has CREATE on schema striatumd
```

Now boot a later revoke-embedding / deployer-aware binary with
`STRIATUM_DEPLOY_DECOUPLED` accidentally or deliberately OFF. This is not an
exotic state: after the first P4 deploy, every future binary that still embeds
0021 is deployer-aware, and the steady-state cursor is `complete`.

Under v5 §3.3a:

1. `CheckDeployActivation` sees `cursorState == complete` and returns nil before
   consulting the flag/config halt.
2. Because the flag is OFF, the caller is the legacy `ConnectAndMigrate` path, not
   `ConnectAndVerify`.
3. The source order then reaches legacy `ApplyMigrations` before drift checking or
   self-record.
4. If the binary has any pending runtime migration that creates an object, the
   runtime role is now applying DDL after 0021 revoked CREATE. That is the
   #512-class lockout shape P4 exists to eliminate. If the migration does not need
   CREATE, the serve path still mutated schema after P4, violating the one-shot
   deployer boundary.
5. If the post-apply drift gate sees a mismatch in shadow mode, current source logs
   and then falls through to `RecordSchemaFingerprint` (`connection.go:384-399`),
   so a legacy self-record can overwrite `schema_state` without any
   `VerifyStoredTranscript` check. That is exactly the sibling self-record path
   v5's §8 asks falsifiers to verify (`HOLDER.md:907-914`).

This is not the old BC-N2 pre-revoke non-complete window; v5 closes that. The gap
is the normal post-deploy steady state, where `complete` short-circuits the guard
that is supposed to keep a revoke-embedding binary off the legacy mutator path.
F11 only asserts non-complete cursor cases and no-cursor/idle cases (`HOLDER.md:832`);
it does not include `cursorState == complete`, revoke embedded, flag OFF, pending
runtime step.

### Strongest rebuttal

The holder's best rebuttal is that `complete` plus matching fingerprint is a
legitimate in-sync state, and the happy-path choreography restarts with
`STRIATUM_DEPLOY_DECOUPLED=1`, so `ConnectAndVerify` serves without mutation
(`HOLDER.md:729-734`). That is true for the immediate intended restart.

It does not close the contract. The v5 typed halt explicitly says a binary that
ships 0021 with the flag off should raise `awaiting_deploy_config` DB-untouched
(`HOLDER.md:696-701`), and C2/P4's point is that future schema changes no longer
ride serve-boot. A `complete` cursor is only complete for the previous plan; it is
not proof that the current binary has no pending runtime changes, and the existing
post-apply drift gate is too late because `ApplyMigrations` has already run. In
refuse mode that is still DB-touched before the halt; in shadow mode it can also
fall through to the legacy self-record.

### Required repair

1. Make `revokeEmbedded && !decoupledEnabled` a pre-apply halt for every cursor
   state, including `complete`, unless a new pre-apply proof shows the current
   binary has no pending deploy work and no fingerprint delta. The conservative
   rule is simpler: revoke-embedding binary + flag OFF -> `awaiting_deploy_config`,
   DB untouched.
2. If the design wants to permit a flag-OFF restart after a completed deploy, it
   needs a pre-`ApplyMigrations` plan/fingerprint comparison that cannot mutate or
   self-record, not the current post-apply `CheckSchemaDrift` path.
3. Extend F11/F15 with a complete-cursor case: `cursorState == complete`,
   `revokeEmbedded == true`, `STRIATUM_DEPLOY_DECOUPLED` OFF, pending runtime
   migration or changed expected fingerprint. Assert `awaiting_deploy_config`,
   `ApplyMigrations` not called, `RecordSchemaFingerprint` not called,
   `schema_state` unchanged, and DB byte-identical.
4. Tighten §4.5 invariant B: legacy `connection.go:399` self-record is allowed
   only when no deploy transcript exists and the binary is not on the P4
   revoke/deploy path. A database with `deploy_cursor` / `deploy_plan` present
   must not reach that writer.

## Verdict

The exact M1 deployer-resume bug from v4 is fixed, and the M2 self-heal early
revoke path is specified with the right barriers. However v5 still leaves an
unaudited sibling self-record/mutation path: a complete cursor returns nil before
the config halt, allowing a deployer-aware binary with the decoupling flag OFF to
enter legacy `ConnectAndMigrate` over a database that already has a deploy
transcript. That can mutate after CREATE is revoked and can self-record
`schema_state` without `VerifyStoredTranscript`.

This is a material C2/M1-hardening gap. The gate should not clear until the
complete-cursor legacy path is made DB-untouched or covered by a pre-apply proof
and a named test.
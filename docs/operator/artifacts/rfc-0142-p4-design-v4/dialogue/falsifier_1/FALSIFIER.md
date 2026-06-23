# FALSIFIER - RFC 0142 P4 v4 transcript/binary resume gap

author: falsifier-reviewer-003

## Revision check: BC-N1, BC-N2, and C1/C2/C3

The v4 holder materially fixes the v3 moving-frontier receipt-key break. The new
`deploy_plan` table stores an immutable transcript keyed by `plan_hash`, including
base frontiers, target frontiers, revoke placement, and every
`{step_index, step_id, role, sha256, transactional}` row
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:164-179`).
The deployer writes that row and `deploy_cursor -> in_progress(0)` in one
transaction before step 0 mutates a frontier
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:193-200`),
then resume classifies the v3 `H` vs `H'` case as "resume with the STORED plan"
instead of unclassified drift
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:232-241`).
The per-step doctor is also changed to enumerate from the stored transcript, not
the moving pending plan
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:572-584`).
So the specific v3 reproducer - kill after runtime step 0 moves the frontier and
then re-run reconstructs `H'` - is genuinely addressed.

The v4 holder also fixes the v3 no-revoke pre-revoke serve window. `CheckDeployActivation`
is no longer gated by `revokeEmbedded`; every deployer-aware binary reads
`deploy_cursor` after the owner watermark check and before both `ApplyMigrations`
and `RecordSchemaFingerprint`
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:404-421`).
The net invariant explicitly covers no-revoke binaries in the pre-revoke
`applied_owner == 20` window
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:449-456`),
and F11 adds the missing no-0021 case with spies proving `ApplyMigrations` and
`RecordSchemaFingerprint` are not reached
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:763`).
I do not use BC-N2 as my standing blocker.

C1 is carried forward coherently: `finalizing` remains distinct, the terminal
receipt/fingerprint/`complete` finalizer is idempotent and `complete` is last
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:210-222`,
`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:562-570`).
C2's typed halts and forward-watermark rule are preserved and re-anchored to
0021 (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:623-671`).
C3's revoke-last ownership mechanism is also preserved: 0021 is excluded from
`owner-ddl apply`, sorted terminal, and every runtime ownership reconcile runs
while `striatumd_rw` still has `CREATE`
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:340-358`,
`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:458-515`).

The standing problem is narrower than v3. It is not "plan identity moves" anymore.
It is that the v4 binary/transcript verification rule is only specified for
not-yet-applied steps. That leaves a real transcript-vs-binary mismatch path for
already-applied steps, exactly inside the BC-N1/F14 correctness core.

## Challenge: an already-applied step can mismatch the resume binary and still finalize

### Claim attacked

The v3 adjudicator's exact repair required resume to "verify the embedded bytes
still match the binary" before resuming from the stored transcript
(`docs/operator/artifacts/rfc-0142-p4-design-v3/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md:277-285`).
The v4 holder claims the wrong-binary case is now legible as
`deploy_plan_binary_mismatch`, not silent re-keying
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:245-251`),
and F14 says a binary whose embedded bytes diverge from the stored transcript
halts rather than silently re-keying
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:766`).

But the operative §1.3 rows check only "not-yet-applied" bytes:

- `in_progress` / `step_committed` resumes when "`deploy_plan[plan_hash]` present;
  not-yet-applied steps' `sha256` match this binary's embedded bytes"
  (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:235`).
- `deploy_plan_binary_mismatch` fires when a not-yet-applied step's stored `sha256`
  does not match this binary's embedded bytes
  (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:238`).
- F14's negative case tampers a not-yet-applied stored step, not an already-applied
  one (`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:766`).

The holder itself flags the missing pressure point in §8: "does the per-step
`sha256` verification fire for an already-applied step"
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:833-839`).
As written, the answer is no.

### Concrete refutation

Use the v4 two-runtime-step F14 shape:

```text
binary A materializes H:
  step 0 = runtime:0045 sha=A45
  step 1 = runtime:0046 sha=A46
  step 2 = owner:0021  sha=A21
```

The materialization write is durable before step 0. Step 0 then commits by the
Q3-A path: runtime DDL, version stamps, cursor advance, and the per-step receipt
share one owner-connection transaction
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:201-204`,
`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:529-534`).
The current runtime engine records the migration SHA in `schema_migrations`
inside the apply transaction (`go/pkg/db/migrations.go:304-355`), and the
frontier advances after the apply (`go/pkg/db/migrations.go:161-172`).

Now crash after step 0 commits. The database has:

```text
deploy_cursor.state = step_committed
deploy_cursor.plan_hash = H
schema_migrations[0045].sha256 = A45
audit receipt = (H, 0, runtime:0045, A45)
runtime frontier = 45
```

Resume with binary B. B has the same `step_id` / version sequence, but its already
applied runtime 0045 bytes differ (`B45`), while the remaining not-yet-applied
steps still match (`A46`, `A21`). This is plausible if the activation binary is
rebuilt or amended after the partial deploy; the stored transcript is specifically
meant to protect this boundary.

Under the v4 text, the not-yet-applied check passes. Step 0 is already applied, so
its byte mismatch is not in the checked set. The cursor is therefore classified as
resume-off-stored-transcript rather than `deploy_plan_binary_mismatch`
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:235-238`).
The deployer resumes at step 1, applies 0046 and 0021, then runs the C1 finalizer.

That finalizer records the current binary's `ExpectedFingerprint()` into
`schema_state` through `RecordSchemaFingerprint`
(`docs/operator/artifacts/rfc-0142-p4-design-v4/dialogue/holder/HOLDER.md:212-218`).
In current source, `ExpectedFingerprint()` is computed from the embedded migration
and owner-bundle bytes of the running binary (`go/pkg/db/schema_drift.go:83-99`),
and `RecordSchemaFingerprint` simply UPSERTs that expected value into the singleton
row (`go/pkg/db/schema_drift.go:171-194`). `LiveFingerprint` later reads that
recorded singleton row (`go/pkg/db/schema_drift.go:145-160`), and
`CheckSchemaDrift` compares the row to this binary's expected value
(`go/pkg/db/schema_drift.go:239-274`).

So binary B can finish a plan whose applied step 0 is actually A's bytes, then
write B's fingerprint as the durable "live" value. The receipt chain still records
`A45`; `schema_migrations` still records `A45`; but the deploy-complete fingerprint
can say "B". A later B boot sees `LiveFingerprint == ExpectedFingerprint` because
"live" is the self-recorded singleton, not a recomputation from
`schema_migrations` / `owner_bundle_meta`. The tightened doctor enumerates missing
receipts from `deploy_plan` (`HOLDER.md:572-584`), but every receipt is present; it
does not specify a stored-step-SHA-vs-database-stamp check for already-applied
steps.

This is a material Q3/R4 gap: the spec can complete and serve a hybrid transcript
under the wrong binary, while claiming the wrong-binary case is a hard
`deploy_plan_binary_mismatch`.

### Strongest rebuttal on the Holder's behalf

The holder's best rebuttal is that the stored `deploy_plan` and audit receipt are
the authoritative provenance, so the system still knows step 0 was `A45`; a changed
resume binary cannot alter the already-committed step, and the remaining steps are
verified before being applied. It can also argue that `plan_hash` includes all
stored step SHAs, so the deploy identity itself remains stable.

That rebuttal preserves provenance, but it does not preserve the claimed binary
compatibility contract. The prescribed fix did not say "verify only future bytes";
it said the embedded bytes still match the binary. The finalizer is especially
load-bearing: it writes the running binary's expected fingerprint and then marks
`complete`. If an already-applied stored step differs from that running binary,
the finalizer must not convert the deployment into an apparently in-sync B deploy.

### Required repair

1. On every resume, validate the entire stored transcript against the current
   binary, not only not-yet-applied steps. If any stored step SHA differs from the
   binary's embedded bytes, classify `deploy_plan_binary_mismatch` and apply
   nothing.
2. For already-applied transcript entries, also verify the database stamps match
   the stored transcript: `schema_migrations.sha256` for runtime steps and
   `owner_bundle_meta.sha256` for owner steps. A mismatch is not "resume"; it is a
   legible transcript/database-stamp mismatch that refuses to finalize.
3. Apply the same full-transcript check before the C1 finalizer writes
   `schema_state` or advances `finalizing -> complete`.
4. Extend F4/F14/F13 with an already-applied mismatch case: kill after step 0
   commits, resume with a binary whose step 0 bytes differ but whose remaining
   steps match, and assert `deploy_plan_binary_mismatch`, no step 1 apply, no
   `RecordSchemaFingerprint`, no `complete` cursor, and a non-green doctor or
   typed diagnostic. Add the symmetric owner-step case for a completed owner
   prefix / pre-finalization crash.

## Verdict

BC-N1's original plan-hash/step-index instability is genuinely fixed, BC-N2 is
genuinely fixed, and I do not find a direct C1/C2/C3 regression. But the v4
transcript/binary verification rule is incomplete in the Q3 correctness core. A
resume binary can disagree with an already-applied stored step, pass the
not-yet-applied check, finish the remaining plan, self-record its own fingerprint,
and mark `complete`. That leaves the exact class the prompt asked us to hunt:
a transcript-vs-binary mismatch that is not forced into `deploy_plan_binary_mismatch`.

This is a standing falsification until the resume/finalizer path verifies the
entire stored transcript, including already-applied steps and their database
stamps.

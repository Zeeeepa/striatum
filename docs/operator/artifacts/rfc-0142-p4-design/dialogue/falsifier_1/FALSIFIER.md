# FALSIFIER - RFC 0142 P4 terminal finalization gap

author: falsifier-reviewer-003

## Challenge: `complete` is allowed to outrun the receipt and fingerprint

### Claim attacked

The Holder's Q3 decision says the per-step-atomic + resumable-cursor contract is
sufficient for every P4 owner+runtime interleaving (`HOLDER.md:27-32`). For
transactional steps, the step and cursor advance commit in the same transaction
(`HOLDER.md:34-40`). For non-transactional steps, `in_progress` plus a reconciler
is supposed to classify the crash window as "incomplete, resume" rather than
"unknown drift" (`HOLDER.md:41-49`).

The terminal boundary does not get the same treatment. The state machine permits
`step_committed(N-1) -> complete` (`HOLDER.md:82-88`), then says `complete` is
written when the last step is committed and the deployer *then* writes the deploy
receipt and calls `RecordSchemaFingerprint` so `schema_state` and `deploy_cursor`
agree (`HOLDER.md:102-105`). The disambiguation table has no row for
`complete` with the expected `plan_hash` but a mismatched or absent fingerprint:
`complete` plus match means serve, `in_progress`/`step_committed` plus mismatch
means resume, and absent/foreign/aborted plus mismatch means genuine drift
(`HOLDER.md:115-119`).

### Concrete refutation

There is a concrete crash point that is outside the Holder's classifier:

1. The final plan step commits. This can be a purely transactional plan; no
   `CREATE INDEX CONCURRENTLY` is required.
2. The deployer advances `deploy_cursor.state` to `complete` for the expected
   `plan_hash`.
3. The process or host dies before the deploy receipt append is durable, before
   `RecordSchemaFingerprint` updates `striatumd.schema_state`, or between those
   two finalization writes.

After restart, the real schema may contain the whole plan, but the durable
metadata is incoherent:

- `deploy_cursor.state = complete`;
- `deploy_cursor.plan_hash == the binary's expected plan`;
- `schema_state.fingerprint` is stale or absent, so `LiveFingerprint !=
  ExpectedFingerprint()` or `LiveFingerprint == ""`;
- the deploy receipt chain is missing its terminal record, or has diverged from
  the schema record.

That state is not "in sync", because the fingerprint/receipt finalization is not
coherent. It is not "incomplete, resume", because the cursor is no longer
`in_progress` or `step_committed`. It is not "genuine drift" as specified,
because the `complete` cursor carries the expected plan hash. The system has only
bad choices: serve with stale provenance, refuse a fully applied schema as drift,
or treat `complete` as terminal and never repair the missing metadata. None of
those is the Q3 contract: every crash intermediate must be coherent and
classified as "incomplete, resume".

Current source makes this boundary load-bearing. `RecordSchemaFingerprint` is a
separate UPSERT into `schema_state` (`go/pkg/db/schema_drift.go:171-194`), not
part of the deploy cursor write. `LiveFingerprint` returns an empty string when
`schema_state` is missing or unrecorded (`schema_drift.go:145-160`), and
`EvaluateSchemaDrift` treats empty live fingerprint as not drift
(`schema_drift.go:226-236`). That empty-live behavior is safe for today's
boot-time self-record path, but it is not safe if P4 has already made a
`complete` cursor durable while the fingerprint and receipt are missing. Under
one reading, decoupled boot could serve because live is "not yet recorded";
under the other, P4 overrides it and refuses. The Holder has not specified the
repairable middle state.

This is exactly the crash-window class the current migration engines were changed
to avoid. Runtime `applyOne` commits the DDL, `schema_migrations`, and
`schema_meta` in one transaction so a crash cannot leave DDL-applied but
version-unstamped (`go/pkg/db/migrations.go:285-354`). Owner bundles likewise
apply the SQL and stamp `owner_bundle_meta` in one transaction, with the stamp
last (`go/pkg/db/owner.go:454-484`). The Holder preserves that discipline inside
steps but drops it at the terminal deploy boundary where `complete`, receipt, and
fingerprint are separate facts.

The named tests do not close the hole. F4 tests the table as written but omits
`complete + expected plan + fingerprint mismatch` (`HOLDER.md:350`). F7 checks a
doctor warning for an unrecorded schema change, which is visibility after the
fact rather than a resumability protocol (`HOLDER.md:353`). F9 covers the happy
path where a complete deploy's fingerprint matches (`HOLDER.md:355`). F8's crash
points stop at step boundaries and do not inject failures through finalization
after the terminal cursor write (`HOLDER.md:354`).

### Strongest rebuttal on the Holder's behalf

The Holder can argue that `complete` was intended to mean "all finalization is
done", and that the wording at `HOLDER.md:102-105` is only narrative ordering.
It can also argue that `deploy` should re-check the receipt and fingerprint even
when the cursor says `complete`, and that `doctor schema_deploy_unrecorded` would
make the missing receipt visible.

That rebuttal does not clear the gate as written. If `complete` means
finalization done, the state machine must forbid making `complete` durable before
receipt and fingerprint are durable, and the tests must kill at each finalization
write to prove it. If `complete` can be durable first, `deploy` needs an explicit
idempotent finalizer and the classifier needs a row for that state. A doctor
warning is not a Q3 resume classification.

### Required design repair

Before P4 clears, the spec needs one concrete terminal-finalization protocol:

1. Keep the cursor at `step_committed(N-1)` until the deploy receipt chain and
   `schema_state` fingerprint are durable, then set `complete` last. Resume from
   `step_committed(N-1)` must run an idempotent finalizer, not re-apply the final
   DDL.
2. Or add a distinct `finalizing` state. Classify `finalizing` or
   `complete_missing_metadata` with expected `plan_hash` and fingerprint mismatch
   as "incomplete, resume", never as serve and never as genuine drift.
3. Define whether `deploy_cursor`, audit receipt append, and `schema_state` can
   share one transaction/connection. If they cannot, specify the idempotent
   reconciliation order and the unique keys that make re-running it exactly-once
   in effect.
4. Add a named fault-injection test, e.g.
   `T-deploy-resume-finalization-crash`, that kills after final DDL commit, after
   terminal cursor write, after receipt append, and after `schema_state` update.
   The assertion should be: no serving until metadata is coherent; re-running
   `striatum daemon deploy` repairs the metadata without re-applying DDL; doctor
   is green after repair.

### Verdict

Real gap remains. The Holder answers the mid-step Q3 problem, especially the
non-transactional DDL case, but leaves the terminal deploy boundary
under-specified. As written, a crash after the cursor reaches `complete` but
before receipt/fingerprint finalization creates an incoherent intermediate that
cannot be classified as "incomplete, resume" by the proposed table.
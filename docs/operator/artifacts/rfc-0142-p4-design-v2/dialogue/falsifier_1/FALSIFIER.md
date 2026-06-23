# FALSIFIER - RFC 0142 P4 v2 per-step receipt gap

author: falsifier-reviewer-003

## Revision check: C1/C2/C3

I do not find the original C1 terminal-finalization bug still open as written. The
revision adopts the ledger's Option B: a distinct `finalizing` cursor state is
entered after the last step, the finalizer appends the `complete` receipt, records
`schema_state`, and writes `complete` last (`HOLDER.md:140-155`). The
classification table treats `finalizing` plus the expected plan as resumable
finalization, never serve and never genuine drift (`HOLDER.md:165-178`), and the
crash table / F10 covers the specific kills after final DDL commit,
`finalizing`-before-receipt, receipt-before-`schema_state`, and
`schema_state`-before-`complete` (`HOLDER.md:395-408`, `HOLDER.md:560`). That closes
the v1 C1 shape.

C2 is also specified at the design level: `CheckDeployActivation` is called after
the owner watermark and before `ApplyMigrations`, including when `deploy_cursor`
is absent (`HOLDER.md:293-322`); the forward-watermark rule blocks revoke-unaware
binaries at `applied >= 20` (`HOLDER.md:462-470`); `RequiredOwnerBundleVersion`
stays 19 (`HOLDER.md:474-484`); and F11 covers the listed bad interleavings
(`HOLDER.md:561`). I am not using C2 as the standing blocker in this atomicity
lens.

C3 chooses one policy: runtime objects remain `striatumd_rw`-owned, with a
same-step catalog-diff ownership reconciliation and DML grant reassertion
(`HOLDER.md:339-374`), plus F12 (`HOLDER.md:562`). The object-kind breadth is still
worth pressure, but my material falsification below is not C3. The new gap is in
Q3/R4: per-step receipts are not made atomic or resumable with the step they
attest.

## Challenge: a step can commit without its receipt, and resume does not repair it

### Claim attacked

The Holder claims every applied step writes a hash-chained deploy receipt into
`audit_log` (`HOLDER.md:378-385`), and F7 says the build must assert one chained
receipt per step (`HOLDER.md:557`). That receipt is not decoration: RFC 0142 Layer
3 says every schema change becomes first-class adjudicated provenance through the
deploy receipt (`docs/rfcs/0142-safe-by-construction-database-change-deployment.md:181-193`).

But the Q3 step protocols only define atomicity between the schema side effect and
the cursor marker. For transactional steps, Q3-A says the step DDL and cursor
advance commit in the same transaction (`HOLDER.md:72-77`, `HOLDER.md:132-135`).
For non-transactional steps, Q3-B says the deployer writes `in_progress`, runs the
self-reconciling non-transactional DDL, then writes `step_committed`
(`HOLDER.md:78-85`, `HOLDER.md:136-139`). The C3 transaction recipe enumerates
`BEGIN`, migration SQL, ownership reconciliation, DML grants, cursor advance, and
`COMMIT` (`HOLDER.md:347-370`). It does not place the per-step receipt in that
transaction or define a step-level receipt finalizer.

### Concrete refutation

Take a transactional runtime step `k` that creates a table and index:

1. `Deployer.applyRuntimeStep` opens the owner-connection transaction, runs the
   migration SQL, reconciles ownership, reasserts grants, advances
   `deploy_cursor` to `step_committed(k)`, and commits. That is the entire
   spelled-out transaction (`HOLDER.md:347-370`).
2. The process dies before the deploy receipt for step `k` is appended to
   `audit_log`.
3. On restart, the cursor is `step_committed(k)` with the expected plan hash. The
   §1.3 table classifies that as `incomplete, resume` and says the deploy resumes
   at the cursor (`HOLDER.md:165-170`). Under §1.2, resume from
   `step_committed(k)` advances to `k+1` (`HOLDER.md:132-139`).
4. Nothing in the spec tells the deployer to detect and backfill the missing
   receipt for step `k` before proceeding. The only idempotency guard in §3.4 is
   for the terminal `(plan_hash, state=complete)` receipt (`HOLDER.md:403-408`).

The final deploy can therefore reach `complete`, record the final fingerprint, and
append the terminal `complete` receipt while the per-step receipt for `k` is absent.
The schema, cursor, and fingerprint are coherent, but the audit chain is not a
faithful record of the real schema history. `doctor schema_deploy_unrecorded` as
specified only warns when `schema_state.fingerprint` advanced but the matching
`complete` receipt is absent (`HOLDER.md:382-385`); if the `complete` receipt exists,
that doctor can be green while a step receipt is missing.

The non-transactional Q3-B case is worse because the receipt cannot be made atomic
with the DDL side effect. A `CREATE INDEX CONCURRENTLY` or `ALTER TYPE ... ADD
VALUE` step has at least two uncovered crash windows:

- Crash after the non-transactional side effect commits but before the per-step
  receipt and before `step_committed(k)`. Resume sees `in_progress(k)` and the
  reconciler may observe a valid index or already-present enum value and skip the
  side effect, but the spec does not require it to append the missing receipt
  before writing `step_committed(k)`.
- Crash after a receipt append but before `step_committed(k)`. Resume still sees
  `in_progress(k)`. Without an explicit idempotency key for the step receipt, the
  repair path either risks a duplicate receipt or must silently infer that the
  side effect is already attested. Neither rule is specified.

This is a Q3/R4 correctness gap, not just an observability nit. The state machine
can leave an intermediate where the real schema side effect is durable and the
cursor says resume can continue, but the provenance side effect is missing or
ambiguous. The current source shows why this class matters: runtime `applyOne`
intentionally commits DDL plus both version stamps in one transaction to avoid
DDL-applied/version-unstamped states (`go/pkg/db/migrations.go:285-355`), and owner
bundles commit SQL plus `owner_bundle_meta` stamp together (`go/pkg/db/owner.go:498-528`).
The v2 holder keeps that discipline for schema markers, but drops it for the
per-step receipt that P4 makes load-bearing.

### Tests that do not close it

F8 injects crashes around step markers and asserts final schema equality
(`HOLDER.md:558`); it does not assert the receipt chain has exactly one receipt for
every step after each crash. F7 is a happy-path receipt assertion and a doctor check
for an unrecorded completed schema change (`HOLDER.md:557`), not a crash-repair
protocol. F10 is terminal-finalization-only (`HOLDER.md:560`) and proves the
`complete` receipt/fingerprint boundary, not receipt atomicity for each preceding
step.

### Strongest rebuttal on the Holder's behalf

The Holder can argue that `append_audit_row` is explicitly atomic with the caller's
transaction in current SQL (`go/pkg/db/sql/owner/0001_authority_phase0.sql:148-151`),
so Q3-A could simply append the step receipt inside the same transaction as the DDL,
ownership reconciliation, grants, and cursor update. That is a valid repair for
transactional steps.

It does not rescue the spec as written. The Q3-A transaction recipe omits the
receipt, and Q3-B cannot share a transaction with `CREATE INDEX CONCURRENTLY` /
`ALTER TYPE ... ADD VALUE` at all. Non-transactional steps need an explicit
step-level idempotent receipt finalizer, with a unique receipt identity and a rule
for the crash after receipt but before `step_committed`. The only idempotent
finalizer v2 specifies is the terminal C1 finalizer.

### Required repair

Before the revision clears, the spec needs a per-step receipt protocol:

1. For Q3-A, state that the step receipt append occurs in the same transaction as
   the DDL, ownership reconciliation, grants, version stamp, and cursor advance;
   name the connection/role that makes that legal.
2. For Q3-B, add a `receipt_pending` / `step_finalizing` state or require the
   `in_progress` reconciler to verify and idempotently append exactly one receipt
   for `(plan_hash, step_id, sha256, state=step_committed)` before it writes
   `step_committed(k)`.
3. Extend §1.3 and F8/F7 with `T-deploy-step-receipt-crash`: kill after
   transactional DDL+cursor commit but before receipt; after NT-DDL side effect
   before receipt; after receipt before `step_committed`; and after
   `step_committed`. Assert final schema equality, exactly one receipt per applied
   step, one terminal `complete` receipt, and a green doctor.

### Verdict

Real gap remains. The v2 holder fixes the v1 terminal finalization boundary, but
it still lets a step become durably applied without a specified, crash-safe receipt
repair path. A P4 design whose receipt chain can disagree with the real schema
history has not cleared the Q3/R4 correctness core.
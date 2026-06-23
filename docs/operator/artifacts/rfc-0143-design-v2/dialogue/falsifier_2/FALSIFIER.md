# FALSIFIER - RFC 0143 design-v2 lifecycle challenge

author: falsifier-reviewer-002

## F1-F7 Revision Check

- **F1 - conditionally closed, but still channel-dependent.** The revision correctly stops routing the no-token floor through `work.block` / `session.report` and instead names a durable `session_unrecoverable_across_rotation` blocker via the supervisor path (`HOLDER.md:214-241`, `HOLDER.md:245-263`). That resolves the original "MCP verb with no MCP token" shape if, and only if, the helper event is a real control event rather than parsed provider terminal output.
- **F2 - mostly closed on the original same-uid file replay.** Retiring the lane-readable `0600` reseal bearer file is the right structural fix (`HOLDER.md:172-179`, `HOLDER.md:265-284`). The remaining replay question moves to the supervisor channel, not to filesystem mode.
- **F3 - not genuinely closed.** The revised predicate lists the right ingredients, but its "no recovery-generation change" guard is not an exact database predicate. Current `jobs` has `current_lease_id` but no lease/recovery generation (`go/pkg/db/sql/0005_repo_local_workflow_state.sql:75-104`), current `leases` has state/expires/release fields but no generation (`:166-179`), and `job_recovery_state` has counters rather than a lease-issued generation (`go/pkg/db/sql/0020_job_recovery_state.sql:13-28`). The spec does not say which column is added, where it is incremented, or what value is stamped into the lease/work packet for reseal-time comparison.
- **F4 - resolved at design level.** The `ResealAlternate` route-admission plan keeps `RequiredCapability=write` for ordinary callers while admitting `CapabilityReseal` only on the three named routes and recording `AuthContext.Capability == reseal` (`HOLDER.md:313-341`). That is a concrete answer to the single-capability prelude finding without falling back to plain `write`.
- **F5 - not genuinely closed.** The revision chooses "active lease window plus bounded daemon-side reseal grace" (`HOLDER.md:343-359`), but does not define the grace bound, its source, or the lock/generation protocol that prevents recovery from requeueing the same job while reseal revives the old lease. The named current helper, `activeLeaseFor`, still rejects expired leases with raw `lease_error` and performs no generation check (`go/pkg/mutations/mutations.go:803-820`).
- **F6 - mostly closed on the overclaim, still dependent on the receiver path.** The adapter matrix correctly stops claiming in-place Codex MCP survival and says Codex seals only over the supervisor receiver path (`HOLDER.md:361-380`). That is honest, but the lifecycle correctness of the receiver path depends on F3/F5 being race-free.
- **F7 - mostly closed on the mirror/file integrity issue.** The revision moves endpoint/epoch away from lane-writable scratch and requires missing-epoch supervised requests to fail (`HOLDER.md:382-408`). The remaining concern is not file mode; it is whether reseal/reconnect failures consistently route through the typed class under the lease/recovery races above.

## Material Challenge C1 - Reseal Grace Has No Race-Free Generation Contract

**Precise claim attacked.** The revised Holder claims F3 and F5 are resolved by a new `resealInFlightJob` predicate: session live, same leased job, active lease or bounded reseal grace, no recovery-generation change, expected artifact path, and accepted boot epoch; failures route to `session_unrecoverable_across_rotation`, never raw `lease_error` (`HOLDER.md:286-311`, `HOLDER.md:343-359`).

**Concrete refutation.** The design names the shape of the predicate, but not the state that makes it race-free. Current source has an active-lease helper, not a reseal predicate: `activeLeaseFor` loads the lease, requires `state == active`, owner/session/job equality, and `expires_at > now()`, then returns `lease_error` for expired leases (`go/pkg/mutations/mutations.go:803-820`). Both `artifact.publish` and `work.complete` still call that helper on their normal paths (`go/pkg/mutations/artifact.go:75-85`, `:124-130`; `go/pkg/mutations/lifecycle.go:1135-1180`).

The revision adds two new lifecycle concepts without pinning them to concrete storage:

1. **"Bounded reseal grace" has no number or source.** It is not tied to `lease.heartbeat_after_seconds`, `leases.expires_at`, a run/workflow setting, a daemon constant, or a maintainer-ratified upper bound. A grace with no bound is not a falsifiable lifecycle contract.
2. **"No recovery-generation change" has no named column or increment protocol.** The current schema has `jobs.current_lease_id` but no job lease generation, and `leases` has no generation. `job_recovery_state.requeue_count` / `transfer_count` are recovery-budget counters, not a lease-issued generation stamped into the work packet. `review_generation` exists, but it is explicitly the review epoch for verdict coherence, not a job lease/recovery epoch (`go/pkg/db/sql/owner/0009_review_generation.sql:11-17`, `:26-30`).

That leaves the common rotation race unresolved:

- The lane loses MCP after daemon restart and cannot heartbeat.
- `leases.expires_at` passes, but the lane is still within the unspecified reseal grace.
- The recovery sweep drains helper events before the main sweep, then expires leases and may requeue or escalate inside the same recovery pass (`go/pkg/mutations/recovery.go:575-587`, `:619-623`, `:866-890`).
- A reseal request and recovery can now race over the same job. If reseal simply reuses `activeLeaseFor`, the expired lease still yields raw `lease_error`, violating F5. If reseal bypasses or extends the lease inside grace without a stamped generation and lock order shared with recovery, it can revive a lease after recovery has requeued/retired the job, violating F3.

The spec gestures at "one transaction" but does not state the transaction's serialization point or row locks. The existing seal paths use `lockRunForJob` plus row locks before normal publish/complete (`artifact.go:75-85`, `lifecycle.go:1135-1180`); recovery uses its own per-run lock and pre-sweep helper drain (`recovery.go:575-610`). The revised predicate must say exactly how `resealInFlightJob` serializes with those paths and which generation value it compares. Without that, the adjudicator cannot tell whether GD-1b proves a safe same-lease renew-and-seal or merely observes whichever side of a race happened to win.

**Strongest rebuttal for the Holder.** The intended implementation could add a new monotonic `jobs.lease_generation` or `job_recovery_state.recovery_generation`, increment it in every claim/requeue/release transition, stamp the issued value into the lease/work packet/helper reseal request, and run `resealInFlightJob` under the same per-run lock as recovery plus `FOR UPDATE` locks on job, lease, and recovery-state rows. It could also define `resealGrace` as a short fixed daemon constant or as a function of the packet heartbeat window. In that version, the expired-lease case could deterministically become either "same lease extended within grace" or the typed blocker.

**Why that rebuttal does not clear the spec.** None of those choices is in the revised spec. The prescribed F3 fix required "the exact database predicate" and "the named generation/lease check"; the prescribed F5 fix required the lease-clock behavior that GD-1b can falsify. A conceptual "recovery-generation change" and an unbounded "reseal grace" are not enough for a security/authz-hot lifecycle path that bypasses the ordinary MCP token and ordinary heartbeat authority.

## Required Repair Before Clearing

- Name the concrete generation field and migration/owner-bundle location, or explicitly reuse an existing field and prove it is monotonic for every claim/requeue/release path.
- State when that generation is incremented and where the issued value is stamped so a stale lane cannot invent or omit it.
- Define `resealGrace` numerically, with its source and maximum.
- Specify the exact lock order for `resealInFlightJob` relative to `artifact.publish`, `work.complete`, and recovery sweep.
- Add negative tests: `TestResealGraceCannotReviveRequeuedLease`, `TestRecoveryRequeueWinsOverExpiredLeaseReseal`, `TestResealBeyondGraceRoutesTypedNotLeaseError`, and `TestResealPredicateUsesStampedRecoveryGeneration`.

## Verdict For This Falsifier

**Material gap remains.** The revision is stronger than v1 on admin-token widening, the OR-capability prelude, and the same-uid bearer-file replay. It still does not clear the lifecycle gate because the lease/grace/recovery-generation repair is not concrete enough to prove no split-brain and no raw `lease_error` in the common post-rotation expiry race. The adjudicator should treat F3/F5 as still open unless the Holder pins the generation, grace bound, and lock-order contract.

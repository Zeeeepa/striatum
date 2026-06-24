# FALSIFIER - RFC 0168 P0 lease lifecycle / pool exhaustion challenge

author: falsifier-reviewer-003

## Verdict

**needs_revision.** I credit the holder on the assigned lens where the proposal is concrete: pool sizing is host-global across concurrent runs, exhaustion is typed and fail-closed, the pool is host-provisioned rather than daemon-created, and the uid lease is deliberately session-scoped and stored in daemon-owned PostgreSQL. Those are the right design directions.

The standing gap is narrower and still gate-blocking: the proposal claims dirty uid quarantine, but the executable state model it specifies cannot represent "not leased, not free." The table only names `active|returned`; allocation picks a uid with no active lease row; restart reconstruction derives the free set from the absence of an active row. That makes the scrub failure path ambiguous exactly where OQ2 is load-bearing. A uid whose prior lease left a process, tmux server, HOME residue, or `~/.claude` credential can become free by schema interpretation before the daemon has durably proven the uid domain clean.

This is not `reject`: the per-lane uid direction remains a structural narrowing and I found no daemon `useradd`/`userdel` expansion, admin-token widening, or shared lane-readable reseal bearer. It is not `accept_with_findings`: complete scrub, leaked-uid reaping, restart survival, and safe exhaustion are explicit clearing conditions, not post-clearance polish.

## Challenge: Dirty Uids Have No Durable Non-Free State

### Precise Claim Attacked

The holder's OQ2 decision introduces `striatumd.lane_uid_leases` with:

- `{ repository_id, pool_uid, session_id, supervisor_id, generation bigint, state active|returned, leased_at, returned_at, scrub_status }`;
- a unique active index on `pool_uid`;
- allocation that picks a uid by finding "no active `lane_uid_leases` row";
- restart reconstruction where the free set is "uids with no active lease row";
- prose saying failed scrub leaves the uid `quarantined` and records `lane_uid_scrub_failed`.

Those statements conflict. `quarantined` is load-bearing, but it is not a state in the named state machine. If it is encoded only as `returned + scrub_status=failed`, the stated free predicate can reallocate it. If it is encoded as `active` to preserve the unique index, then the row is no longer a lease to a live session, and the proposal has no retry, doctor, recovery, or exhaustion accounting semantics for that zombie-active row.

### Source-Anchored Evidence

RFC 0168 makes this exact lifecycle question a gate input: it asks when a uid is returned and scrubbed, including home dir, credential store, tmux server, stray processes, and how a leaked/never-returned uid is reaped (`docs/rfcs/0168-per-lane-security-principal.md:124-131`).

The holder acknowledges today's teardown is insufficient and says P0 must add full uid-domain scrub. It cites the new lease table and its two states at `HOLDER.md:246-257`, allocation by "no active" row at `HOLDER.md:259-262`, scrub steps at `HOLDER.md:263-283`, reaper extension at `HOLDER.md:284-292`, and restart free-set derivation at `HOLDER.md:293-303`. P0 then depends on "return + scrub + reaper" and "quarantine-on-scrub-failure" at `HOLDER.md:477-478`.

The current source reinforces why this cannot be hand-waved as existing cleanup. `session.close` stops supervisors and then removes only narrow config files (`go/pkg/mutations/lifecycle.go:529-568`). `stopSupervisorsForTerminalSession` kills the known tmux session or known pid/helper and removes stdin pipe paths (`go/pkg/mutations/mutations.go:1596-1687`). `stopTmuxBackedLane` is a tmux `kill-session` with pid fallback (`go/pkg/mutations/supervision_process.go:19-39`), and `terminateProcessWithStartToken` signals only the recorded pid after a start-token check (`go/pkg/mutations/supervision_process.go:128-144`). This is not a per-uid process domain scrub, HOME scrub, credential scrub, or zero-residue proof.

The existing lease schema shows the pattern the holder is extending: current job leases have explicit states `active`, `released`, and `expired`, with a partial unique index only for active resource leases (`go/pkg/db/sql/0005_repo_local_workflow_state.sql:166-186`). For uid reuse, the missing state is not cosmetic. The allocator, recovery sweep, restart reconstruction, and doctor output all need the same durable answer to "is this uid free?"

### Concrete Failing Case

1. Session S1 leases uid U and starts a lane. The lane leaves residue: a daemonized U-owned process, a live or stale U-owned tmux server/socket, HOME scratch, and `~U/.claude/.credentials.json`.
2. S1 closes, or the daemon dies mid-lease and the first post-restart sweep decides the session is dead. P0 begins the proposed scrub.
3. The scrub does not reach a clean postcondition. Examples: injected scrub failure, a same-uid process still present in `/proc` after the first `kill -KILL -1`, a tmux socket/server still addressable, credential deletion failure, or HOME cleanup failure.
4. The daemon must now record U as **not leased to a live session and not free**. The holder's schema gives it only `active` or `returned`. If it marks `returned` or removes the active row, allocation by "no active lease row" can hand U to S2. If it leaves the row `active`, U is now an active lease for a terminal/dead session, which contradicts the reaper story and consumes pool capacity without a specified recovery path.
5. Under concurrent-lane pressure, the system either reuses dirty U for S2 or leaks capacity until `lane_uid_pool_exhausted` fires even though an operator-visible quarantine/retry workflow should exist. Dirty reuse is a cross-lease same-uid leak; zombie-active capacity loss makes exhaustion unsafe and opaque.

The damaging branch is S2 running as U while S1 residue still exists. At that point RFC 0168's core separation no longer applies inside U. A stale U-owned process can read U-owned files, keep credentials in memory, mutate U's tmux server, recreate files in U's HOME, or race with S2's `0600` reseal token. OQ5's generation token helps only on daemon-mediated attestation/control paths; it does not stop same-uid local file, process, tmux, or in-memory credential residue inside the OS uid domain.

### Why The Holder's Best Rebuttal Does Not Clear It

The strongest rebuttal is that `scrub_status` could encode failure while the row remains excluded from allocation. That is plausible, but it is not the proposal's executable state machine. The proposal says `state active|returned`, "free" means no active row, and `quarantined` only appears in prose. A falsifiable implementation spec must define which exact rows are free, which rows are quarantined, and which transitions are legal across close, sweep, restart, doctor, operator retry, and exhaustion.

The second rebuttal is that A9/A10/A11 tests mention scrub, reaper, and restart survival. The positive checks are necessary, but they do not define the negative path. The gate needs the failure invariant: a uid with any unproven or failed scrub result is never allocated, survives daemon restart as non-free, and has one explicit retry path back to free only after the same zero-residue proof passes.

### Required Revision

Revise OQ2/P0 into an executable state machine:

1. Add explicit states such as `active`, `scrubbing`, `quarantined`, and `returned` (or an equivalent precise model). Define the free predicate as only clean `returned` for the latest generation, never merely "no active row."
2. Make allocation transactional for a `pool_uid` and impossible while any row for that uid is `active`, `scrubbing`, or `quarantined`.
3. Define the scrub postcondition: no non-zombie uid-owned process capable of running user code, no uid-owned tmux server/socket, no credential store, no old reseal token, and HOME private scratch reset. Mark `returned` only after that proof.
4. Define failed-scrub lifecycle: persist `quarantined`, emit typed `lane_uid_scrub_failed`, expose it in `doctor`, preserve it across restart, exclude it from the free set, and provide a recovery/operator retry that can transition to `returned` only after the same proof.
5. Extend A7/A9/A10/A11 with negative tests: injected scrub failure quarantines; a quarantined uid is not allocated after daemon restart; exhausting clean uids while one is quarantined yields typed `lane_uid_pool_exhausted`; clearing residue plus retry is the only path back to free.

## Checks Credited

- Pool sizing correctly targets host-global live lane concurrency across concurrent runs, not one workflow's `max_active_jobs`.
- Exhaustion is correctly fail-closed and typed in the happy path: no shared-uid fallback and no daemon-managed autogrow.
- Host-runbook provisioning is the right privilege-boundary answer. The runbook currently gives launch-as authority (`docs/how-to/lane-sandbox.md:88-100`) and states this is not new daemon authority (`docs/how-to/lane-sandbox.md:413-417`); the holder preserves that boundary rather than giving the daemon uid-lifecycle power.
- A session-bound, PostgreSQL-persisted uid lease is the right restart-survival foundation. The challenge is the missing dirty/quarantine state and proof-before-return rule.

## Bottom Line

RFC 0168 only clears if a returned uid is either proved clean or kept durably non-free. As written, the holder has a strong happy-path scrub story but no executable quarantine/free-set model for the failure path. That can hand lease N+1 the same uid while lease N residue still exists, recreating a same-uid leak across leases; or it can leak pool capacity into ambiguous zombie-active rows and make exhaustion unsafe. OQ2/OQ1 have not cleared.

# FALSIFIER - RFC 0170 P0 design v5 read-only safety review

author: falsifier-reviewer-002

## Disposition

I do not have a material P0-blocking falsification under the re-scoped
READ-ONLY SAFETY + NO-REGRESSION bar. The strongest attacks I can ground either
die on the v5 off-the-`wait`-path fold, land in the explicitly P1 #619
cull-liveness bucket, or remain downstream implementation proof obligations
rather than design contradictions. This is not a gate verdict; it is the
falsifier_2 handoff for the G2/no-regression lens.

## Attack 1 - a cull fold could starve the recovery scheduler

**Claim challenged.** B1/B4/B5: the cull fold cannot block the recovery loop,
defer the next `scheduler_cursors` refresh, or worsen
`doctorRecoverySweepCursor`.

**Counterexample attempt.** A synchronous `DecayTickSweep` bolted into
`SweepOnce` would reproduce the v4/v3 carrier: `RunScheduler` does not call
`wait(interval)` until `opts.SweepOnce(ctx)` returns, so any scan that blocks
inside the sweep can hold the single recovery scheduler goroutine and delay the
next persisted cursor refresh.

**Evidence.** The source carrier is real for a synchronous fold:
`go/pkg/recovery/scheduler.go:55-80` increments `Sweeps`, calls
`opts.SweepOnce(ctx)`, and only then waits. `ActiveRunSweep.SweepOnce` refreshes
the persisted recovery cursor before returning: normal sweeps call
`upsertSchedulerCursor` at `go/pkg/recovery/sweep.go:114`, and the upsert writes
`scheduler_cursors.last_result_json` at `go/pkg/recovery/sweep.go:246-263`.
`doctorRecoverySweepCursor` reads that persisted cursor JSON
(`go/pkg/reads/doctor.go:383-390`) and keys the quiet window on
`last_lane_advanced_at -> started_at -> created_at`, not `last_sweep_at`
(`go/pkg/reads/doctor.go:438-480`). The existing metrics fold precedent in
`go/cmd/striatumd/main.go:889-897` also shows the relevant order: wrap
`innerSweep`, let `innerSweep` return, then run observational fold work and
return the original recovery result.

**Strongest rebuttal.** The v5 holder's safety move directly removes the
carrier: the cull fold position does only O(1) slot-check/spawn/skip work,
launches the scan in a detached single-in-flight goroutine, and never joins it
(`HOLDER.md:622-645`, `:716-724`, `:807-843`). With that exact layout,
`innerSweep` has already refreshed the current cursor before the fold position,
and the wrapped `SweepOnce` return is not delayed by scan duration, so the next
`wait(interval)` and next `innerSweep` refresh happen on the same schedule as a
no-cull control. The B5 negative control is also the right one: put the fold
back on the `wait`-gating path and the tick-2 refresh-not-deferred assertion
must fail (`HOLDER.md:845-890`).

**Unanswered gap.** This remains an implementation-sensitive invariant, not a
v5 design break. The build must preserve the O(1) fold position and must not
write a "select on scan done or cullCtx.Done()" join back into the recovery
goroutine. If it does, that is a direct P0 safety regression; the v5 SPEC
already names the refuting test.

## Attack 2 - the #619 filesystem hang deferral could hide a torn or stale write

**Claim challenged.** B4-prime/B5: a non-cooperative, ctx-ignoring filesystem
hang is only a restart-recoverable cull-liveness degradation, not a P0 safety
break.

**Counterexample attempt.** If the design releases the single cull slot on
timeout without a generation fence, or if a ctx-ignoring scan returns after
`cullCtx` has expired and still enters the UPSERT phase, then the deferred #619
late-writer problem is no longer just liveness. It can become a stale
`cullable_entity` write after the tick's safety deadline, which is exactly the
kind of torn/stale write the packet asks this lane to stop on.

**Evidence.** v5 does not make that unsafe move. It explicitly says a
non-cooperative blocking syscall is not preemptible by `cullCtx`; it holds the
single cull slot until daemon restart, and later ticks logged-skip the cull
instead of starting a second writer (`HOLDER.md:725-735`). The L4 write rule is
also stated in the safe direction: the detached goroutine first computes the
full candidacy delta in memory, and performs the UPSERT transaction only when
the scan completed before `cullCtx` fired; if the deadline fires first, the
write phase never begins and the in-memory delta is discarded
(`HOLDER.md:738-749`). In P0 the slot is not released on timeout, so the P1
generation fence is explicitly future hardening for a future design that does
release the slot (`HOLDER.md:767-769`, `:948-949`).

**Strongest rebuttal.** Under the v5 assumptions, #619 is honestly scoped as
read-only liveness: the stuck worker does not run in the recovery goroutine, so
it cannot block `RunScheduler`; it does not write `scheduler_cursors`, lanes,
runs, doctor state, pages, tombstones, or run-admission state; and with the L4
"complete before deadline or write nothing" gate, it cannot produce a partial
or stale cull write. That leaves a real operational degradation - the cull
observation can stop advancing until daemon restart - but not data loss, daemon
suicide, recovery-loop blockage, cursor-refresh deferral, or a torn write.

**Unanswered gap.** The build should add the obvious late-return guard test in
addition to the non-returning hang test: a scan that ignores `ctx.Done()`, then
returns after `DefaultCullFoldTimeout`, must perform zero UPSERTs. I do not
score that as a v5 design falsification because L4 already requires
"completed before `cullCtx` fired" before the write phase. It is the exact
implementation assertion that prevents #619 from disguising a P0 safety break.

## Attack 3 - G3/G4 could regress while the safety text stays clean

**Claim challenged.** The v5 rewrite did not smuggle in substrate or
forward-compat regressions: no wrong migration path, no missing authority rows,
no `SELECT *`, and no P1+ deletion/page/doctor/run-admission behavior.

**Counterexample attempt.** A common failure mode would be to treat P0 as more
than observation: add a `doctor` block, page, tombstone/delete, run-admission
effect, or use a broad read like `SELECT * FROM cullable_entity` that recreates
the #614 column-grant hazard. Another would be to put the runtime migration in
a nonexistent `go/pkg/db/migrations` directory or collide with an occupied
`0045` slot.

**Evidence.** The v5 holder keeps the G3 instructions unchanged: runtime
migration at `go/pkg/db/sql/0045_cullable_entity.sql`, both read/write authority
inventory rows, `striatumd_rw` `SELECT, INSERT, UPDATE` grant, no owner DDL/FK
for migrations >=27, and explicit-column reads only (`HOLDER.md:146-205`,
`:983-985`). In the current tree, `go/pkg/db/sql/0045*` is still free, so the
specified path remains available. The G4 table keeps deletion machinery out of
P0: `cull_tombstone`, `doctor` integrity, `cull_gate`, timed reaper,
`accretion_ledger`, and run-admission throttle are all P1 or later
(`HOLDER.md:938-955`). The P0 read-only assertion says the cull fold writes
only `striatumd.cullable_entity` and takes no tombstone, deletion, page,
doctor, or run-admission action (`HOLDER.md:771-787`).

**Strongest rebuttal.** I found no v5 wording that widens the downstream P0
implementation envelope beyond observe-only candidacy. The substrate and
forward-compat traps are still explicit refuting conditions rather than
implicit latitude.

**Unanswered gap.** None at the design level. The build still has to prove the
authority inventory and `SELECT *` claims with the existing static/PG guards,
but the v5 holder has not regressed the contract.

## Positive checks against the scoped bar

- The old scheduler-starvation carrier is named honestly and fixed by design,
  provided the build keeps the cull fold detached and O(1).
- The persisted cursor timing claim is source-true against current
  `scheduler.go`, `sweep.go`, and `doctor.go`: the cursor is upserted in
  `ActiveRunSweep` before the fold position, doctor reads persisted
  `last_result_json`, and quietness does not key on `last_sweep_at`.
- The non-cooperative filesystem hang is not re-labeled as bounded. v5 says it
  is not preempted by `cullCtx`, holds the cull slot until restart, and belongs
  to #619 unless it can be shown to block recovery, defer refresh, destabilize
  the daemon, or write stale/torn data.
- I found no G3 regression in the specified substrate shape and no G4 smuggling
  of tombstone/deletion/page/doctor/run-admission behavior into P0.

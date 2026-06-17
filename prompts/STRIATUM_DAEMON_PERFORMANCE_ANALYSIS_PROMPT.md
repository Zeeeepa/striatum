# Striatum Daemon Performance Analysis Prompt

Status: reusable
Date: 2026-06-17
author: operator-claude-opus-4-8-001

Hand this to a coding agent (or scaffold it as a Striatum investigation run)
to drive an *evidenced* performance analysis of the `striatumd` daemon that
also yields concrete, buildable instrumentation/observability recommendations.

It is deliberately reframed for this product: "performance" here means
**failure-mode blast-radius + forward-progress**, not request p99, and
observability is **mined from the data the daemon already writes**
(`striatumd.events` / `audit_log`) rather than bolted on — so every
recommendation stays inside the local-first product boundary (no hosted
telemetry, no external persistence) and is actionable at single-operator
scale. Grounding facts (zero metrics today, the 60s recovery-sweep that kills
the daemon on error, the #198 git-in-lock convoy, hazards #322/#325/#245) are
current as of v2.33.0; re-verify the live `events` vocabulary and code line
numbers before trusting any number derived from them.

Developed via the `adhd` divergent-ideation skill (5 cognitive frames →
score/cluster → 3 deepened spines: log-as-observability-plane,
failure-mode/forward-progress reframe, cheapest-revertible-hooks).

```markdown
# Analyze striatumd runtime performance and recommend instrumentation & observability

## Mission
Produce an EVIDENCED performance analysis of the `striatumd` daemon and a set of
concrete, buildable instrumentation/observability recommendations. The daemon is a
local-first, single-operator Go service backed by one PostgreSQL cluster the operator
owns, running on one workstation. It has zero metrics/telemetry today (stdlib `log`
only, one loopback `/v1/health`, read-only `doctor`/`status`/`dashboard` MCP methods).
This is NOT an enterprise SRE engagement. Every recommendation must be actionable at
laptop scale and must stay inside the product boundary below.

## What "performance" means here (read this first — it reorders everything)
For a single-operator daemon, a 200 ms p99 is irrelevant next to one error that takes
down every lane. Rank your analysis by, in order:
1. **Blast-radius per failure** — how far one fault propagates (the 60s recovery sweep
   currently KILLS the whole daemon on any sweep error; one git shell-out inside a
   lock-holding transaction stalls every waiter — the #198 "convoy").
2. **Forward progress / liveness** — does work actually advance (claim → heartbeat →
   complete), or is the daemon silently wedged while still answering health checks?
3. **Tail latency on contended paths** — p50/p99/max and distribution SHAPE, never the
   mean, on any path that holds a lock. Averages are banned as a primary metric.
Request throughput is the LOWEST priority. Do not lead with it.

## The four subjects that matter (failure-mode forensics)
Treat these as the only first-class subjects. For EACH, (a) describe the mechanism in
code, (b) define the inverse-of-the-page health invariant — the single boolean+timestamp
"last-known-good" marker the operator wishes had been latched and that SURVIVES a crash,
and (c) state where in the code it could be latched cheaply:
- **Sweep-suicide (P0):** a sweep-iteration error cancels the daemon. Want: a durable
  `last_sweep_started/ended/panicked` breadcrumb written from a `recover()` so a corpse
  daemon names the iteration that killed it.
- **Convoy-hang (#198):** git subprocess runs inside a lock-holding txn. Want: per-span
  start/end/duration; a span that opened and never closed (age > budget) is the signature.
- **Deadlock (#325):** parallel completion contends on the same run row. Want: a 40P01
  counter + `last_completion_error`, visible without a profiler.
- **Silent claim-starvation (#322/#245):** Want: an idle-vs-wedged DISCRIMINATOR —
  `a claimable job exists (ready, deps met, under max_active_jobs) AND zero lanes advanced
  in N seconds` ⇒ WEDGED; `claimable == 0` ⇒ correctly quiet. Only the conjunction pages.

## Primary method — the existing log IS the observability plane
The daemon already appends `striatumd.events` (state transitions, with `created_at`,
`event_type`, `run_id`, `job_id`, `lease_id`, `actor_session_id`) and a separate
`audit_log` (`request_id`, `ts`). Use them as the zero-instrumentation telemetry plane
BEFORE proposing anything new:
- Reconstruct stage durations purely in SQL:
  `EXTRACT(EPOCH FROM created_at - lag(created_at) OVER (PARTITION BY repository_id, job_id ORDER BY created_at, event_id))`,
  pivot on `event_type`, bucket with `percentile_cont(ARRAY[0.5,0.9,0.99])`.
- **Evidence discipline:** no performance claim is admissible unless it cites a concrete
  row-range (`event_id` lo..hi, partition key, row count summarized). "Slow" without a
  citable interval is rejected.
- **Boundary-of-timestamp discipline (load-bearing):** for every derived latency, declare
  which PHYSICAL boundary each timestamp marks. `events.created_at` marks a state
  TRANSITION, not lock-acquire or COMMIT — so an inter-event delta includes agent
  think-time, network, and PG queueing. Do NOT report inter-event deltas as lock-hold
  time. Cross-validate event-derived durations against `audit_log` for the same mutation;
  treat clock agreement as a cited invariant.
- **Blind-spot ledger:** explicitly enumerate the hot-path latencies the current event
  vocabulary CANNOT resolve — intra-txn lock-hold time, recovery-sweep wall time,
  work.heartbeat cadence, the git-shell duration inside lease-expiry. The #198 convoy
  hold-time is the headline blind spot: there is no lock-acquire/commit event pair today,
  so quantifying it REQUIRES the first additive column you propose.

## Secondary method — cheapest revertible runtime hooks (one afternoon, ordered)
Turn hooks on cheapest-and-most-diagnostic first; do NOT touch daemon control flow until
the data demands it. Every hook must be one flag/env/GUC, loopback-only, and revert to
byte-identical baseline by unsetting it (no migration, nothing surviving restart unless an
env-var still set). Deliver as an idempotent `instrument.sh` + a guaranteed `revert.sh`.
1. Operator-PG, zero daemon code: enable `pg_stat_statements` + `auto_explain`
   (`log_min_duration='200ms'`, `log_analyze`, `log_nested_statements`), reset, replay one
   real recovery-sweep + one claim/heartbeat cycle, read top rows by `total_exec_time`.
2. 1 Hz `pg_locks JOIN pg_stat_activity` sampler into a tagged CSV while you DELIBERATELY
   provoke a parallel-completion run, to catch #325 and the #198 convoy in flight
   (`wait_event`, blocking pid, held-lock mode).
3. Piggyback the EXISTING `/v1/health` handler behind `?debug=1` to also marshal
   `runtime/metrics`, `runtime.NumGoroutine()`, and `pgxpool.Stat()` (pool saturation:
   `AcquiredConns` vs `MaxConns`). No new route, no new port.
4. Only if 1–3 leave a timing gap: a `defer`-timing decorator around the sweep and
   `withTx`, gated by `STRIATUM_TIMING=1`, emitted via the existing stdlib logger, awk'd to
   p50/p95.
5. Last/optional: `expvar`/`net/http/pprof` on the loopback listener only.
DEMOTE allocation hooks (`GODEBUG=gctrace`, heap profiles): this daemon is lock/IO/
subprocess-bound, not alloc-bound — use them only to confirm-or-ignore, never to lead.

## Tertiary method — static contention topology (rank before/without running)
Map every `withTx` scope, advisory lock (`lockRunForSession`/`lockRepo`), `FOR UPDATE
SKIP LOCKED`, and git-in-transaction site into a lock-ordering + hold-duration graph;
rank predicted convoy/deadlock hotspots from source. This ranks danger but cannot
quantify it — it must be paired with the runtime evidence above, never used alone.
Also: measure the claim-poll empty-hit ratio and sweep work-vs-idle ratio; if dominated
by wasteful periodic wake-ups, consider adaptive cadence / `LISTEN`-`NOTIFY` as the lever.

## Instrumentation recommendation rules (hard gates)
- **Additive-column discipline:** any proposed schema change must be a NULLABLE,
  append-only column written inside the mutation txn that already emits the event (commits/
  rolls back atomically, crash-survivable), default-NULL for old rows; any index goes
  through an owner bundle (the `events` table is owner-held).
- **Non-perturbation:** instrumentation must be observably non-perturbing — out-of-band,
  async, or on a SEPARATE short-lived connection. Markers for the failure modes must NOT be
  written inside the lock-holding transaction (that extends the hold window and can worsen
  #198/#325). A failed marker write is non-fatal telemetry — the cure must never become a
  new way to kill the daemon. State an explicit overhead/contention budget.
- **Stays-local proof:** every recommendation carries one clause certifying it cannot
  export data past loopback and adds no durable external sink.
- **Terminates in a fix:** every finding ends in a code-located remediation tied to a real
  hazard (#322 ungated max_active_jobs, #325 deadlock, #245 claim race, #198 convoy, the
  sweep-suicide P0). No orphan metric, no dashboard nobody acts on.

## Product boundary (refuse violations)
Local-first only. Forbidden without an explicit product decision: hosted services, cloud
APIs, telemetry export, durable transcript capture/export, any external persistence, any
new always-on listener. Maintain a **refusal register**: list the measurements you
deliberately decline (e.g. per-keystroke/agent-process content timing, transcript
sampling) and the boundary clause each would violate.

## Deliverable shape (the report must contain)
1. **Bottleneck ledger** — ranked list; each row: subject → measured-bottleneck vs ruled-out
   → cited evidence (row-range or captured artifact) → code location → recommended fix.
2. **Reproduction protocol** per hazard (#322/#325/#245/#198 + sweep-suicide) that a second
   operator can replay deterministically from a seeded PG fixture, with the audit/event rows
   the repro is expected to emit listed UP FRONT as a falsifiable prediction.
3. **Blind-spot ledger** with, per blind spot, the single smallest additive column/view/
   index that turns it into lawful crash-survivable evidence.
4. **Instrumentation plan** ordered cheapest→deepest, each with its revert step, overhead
   budget, and stays-local proof; shipped as `instrument.sh`/`revert.sh`.
5. **Refusal register.**
6. **Recommended first move** — the smallest change that yields the most: almost certainly
   the convoy hold-time additive column + an "in contempt of lock" `doctor` integrity check
   so #198 regresses loudly (red doctor = stop-and-fix) instead of being re-derived by hand.

## Acceptance self-check (assert before declaring done)
- [ ] Every perf number cites a row-range or a captured artifact (no anecdotes).
- [ ] Every timestamp-derived latency declares which physical boundary it marks.
- [ ] No recommendation requires data to leave loopback or adds a durable external sink.
- [ ] No proposed instrumentation runs inside a lock-holding txn or can kill the daemon.
- [ ] Tail/distribution (not mean) used for every contended path; convoy treated as bimodal.
- [ ] Every finding terminates in a code-located fix tied to a tracked hazard.
- [ ] `instrument.sh` is fully reverted by `revert.sh` to baseline behavior.
```

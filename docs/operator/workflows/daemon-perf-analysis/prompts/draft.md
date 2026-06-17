# Task: Evidenced performance analysis of the striatumd daemon

Produce the draft performance-analysis report at the declared artifact path
(`docs/operator/artifacts/daemon-perf-analysis/DRAFT.md`). Stay inside your
declared `write_scope`. The canonical, full instructions for this analysis are
in the context doc
`prompts/STRIATUM_DAEMON_PERFORMANCE_ANALYSIS_PROMPT.md` — read it first and
follow it exactly. This file is the short operational restatement.

## What to deliver in DRAFT.md

An EVIDENCED performance analysis of `striatumd` plus concrete, buildable
instrumentation/observability recommendations, ranked for a local-first,
single-operator daemon. "Performance" here means, in priority order:
1. **Blast-radius per failure** (the 60s recovery sweep that kills the daemon
   on error; the #198 git-shell-inside-lock-holding-transaction convoy).
2. **Forward progress / liveness** (does work advance claim→heartbeat→complete,
   or is the daemon silently wedged while still answering `/v1/health`?).
3. **Tail latency on contended paths** (p50/p99/max + distribution shape;
   averages are banned as a primary metric).
Request throughput is the lowest priority — do not lead with it.

## Method (follow the canonical prompt for full detail)

- **Primary — the existing log IS the observability plane.** Reconstruct stage
  durations from `striatumd.events` timestamp deltas (window functions over
  `(repository_id, job_id)`); cross-validate against `audit_log`. Enforce the
  evidence discipline (every claim cites an `event_id` row-range) and the
  boundary-of-timestamp discipline (declare which physical boundary each
  timestamp marks — `events.created_at` is a state-transition boundary, NOT
  lock-acquire/commit). Produce a **blind-spot ledger** for latencies the log
  cannot resolve today (intra-txn lock-hold time, sweep wall time, heartbeat
  cadence, git-shell duration), each with the smallest schema-compatible
  additive column/view/index that would make it lawful, crash-survivable
  evidence.
- **Secondary — cheapest revertible runtime hooks** (one afternoon, ordered
  cheapest→deepest): `pg_stat_statements`+`auto_explain`; a 1 Hz
  `pg_locks ⋈ pg_stat_activity` sampler during a deliberately provoked
  parallel-completion run; `/v1/health?debug=1` dumping `runtime/metrics` +
  `pgxpool.Stat()`; a `STRIATUM_TIMING=1` defer-decorator; `expvar`/`pprof`
  last. Demote `GODEBUG`/heap hooks (this daemon is lock/IO/subprocess-bound,
  not alloc-bound). Ship the plan as `instrument.sh`/`revert.sh`.
- **Tertiary — static contention topology**: map every `withTx`, advisory lock,
  `FOR UPDATE SKIP LOCKED`, and git-in-transaction site into a lock-order +
  hold-duration graph and rank convoy/deadlock hotspots; pair with runtime
  evidence, never used alone.

## Hard gates (the report is rejected otherwise)

- Every instrumentation recommendation is **non-perturbing** (out-of-band /
  async / separate connection; nothing inside a lock-holding txn; a failed
  marker write must never be able to kill the daemon) and carries a
  **stays-local proof** (cannot export data past loopback; no durable external
  sink; no new always-on listener).
- Every finding **terminates in a code-located remediation** tied to a tracked
  hazard (#322, #325, #245, #198, sweep-suicide P0). No orphan metrics.
- Maintain a **refusal register** of measurements you decline (per-keystroke /
  transcript / agent-content timing) and the product-boundary clause each
  would violate.

## Required structure of DRAFT.md

1. Bottleneck ledger (ranked; subject → measured/ruled-out → cited evidence →
   code location → fix).
2. Reproduction protocol per hazard from a seeded PG fixture, expected
   audit/event rows listed up front as a falsifiable prediction.
3. Blind-spot ledger (per blind spot: smallest additive column/view/index).
4. Instrumentation plan (cheapest→deepest, each with revert step, overhead
   budget, stays-local proof).
5. Refusal register.
6. Recommended first move (smallest change, highest leverage — likely the
   convoy hold-time additive column + an "in contempt of lock" `doctor` check).

Ground every concrete claim in the source under `go/` and re-verify the live
`striatumd.events` vocabulary and code line numbers; the grounding facts in the
canonical prompt are current as of v2.33.0 but must be confirmed, not trusted.
Read `ARCHITECTURE.md`, `docs/reference/spec.md`, and the standing work-list
`STRIATUM_DEEP_ARCHITECTURE_REVIEW_CLAUDE_FABLE_5_2026-06-11.md` (§E.1 sweep
suicide / convoy) for context.

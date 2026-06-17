# striatumd Runtime Performance Analysis — Failure-mode blast-radius & forward-progress

Status: evidenced analysis
Date: 2026-06-17
Daemon: `striatumd` `v2.33.0-47-gc2213393` (HEAD `c2213393`), pid 82183, single-operator
author: operator-claude-opus-4-8-001

> Executes `prompts/STRIATUM_DAEMON_PERFORMANCE_ANALYSIS_PROMPT.md`. "Performance"
> here means **failure-mode blast-radius + forward-progress**, not request p99.
> Every number cites a live `striatumd.events` / `striatumd.audit_log` row-range
> (read-only against `striatum_daemon`, schema `striatumd`, 2026-06-17). All code
> citations re-verified at `c2213393`. No recommendation leaves loopback or adds a
> durable external sink.

---

## 0. Headline & method-integrity disclosure

**Headline:** the daemon's dominant runtime risk is a **single per-repository
event-chain-head serialization point** (`repo_event_chain_heads … FOR UPDATE`
inside `append_event_row`) that *every* event append on a repo queues behind.
`supervisor.progress` is **98.6 % of all 13.6 M events** and floods that lock —
measured at **98,008 appends in one second from one repository**. Any transaction
that holds the chain head (or a per-run advisory lock) while shelling out to git
turns that flood into a convoy: a waiter's `append_event_row` blocks until its
statement timeout fires as **SQLSTATE 57014** — which is exactly the live,
currently-open **#355** ("run prepare hard-fails with 57014 … recurrence of #198
at 2.33.0; doctor stays green"). `doctor` stays green because the convoy corrupts
no integrity row — it only stalls forward progress, and the 57014 is reclassified
as benign by `isTransientDaemonLoadError`.

Three of the four "first-class hazards" from the prompt are **already fixed** at
2.33.0 (sweep-suicide error path, #198 sweep path, #325 deadlock); their residuals
are narrower than the prompt assumes. The analysis below states the real current
state of each, with proof.

**Method-integrity disclosure (evidence discipline applied to ourselves).** During
this analysis one investigator derived a "fact" that `striatumd_rw` carried
`lock_timeout=3s, idle_in_transaction_session_timeout=15s` and concluded the
"finite lock_timeout" lever was already pulled. That was a **measurement artifact**:
the investigator had itself `ALTER ROLE`-injected those GUCs into the live daemon
role and mis-identified them as baseline. The authoritative start-of-analysis
capture showed the true baseline is `{statement_timeout=600s}` only — **no
role-level `lock_timeout`** (so it inherits the server default `0` = disabled).
The injected GUCs were reset (`ALTER ROLE striatumd_rw RESET lock_timeout` /
`… RESET idle_in_transaction_session_timeout`); the role is verified back at
`{statement_timeout=600s}`. **Conclusion: a finite, loud `lock_timeout` is a valid,
not-yet-applied lever.** This is recorded here because the prompt's own evidence
rule ("no performance claim is admissible without a citable interval") caught a
self-inflicted false fact — exactly the failure the discipline exists to prevent.

---

## 1. Bottleneck ledger (ranked by blast-radius)

Each row: subject → measured bottleneck **vs** ruled-out → cited evidence → code
location → recommended fix. Distribution shape, never the mean.

| # | Subject | Measured bottleneck (or ruled out) | Cited evidence | Code location (`c2213393`) | Recommended fix |
|---|---|---|---|---|---|
| **1** | **Event-chain-head serialization (root substrate of #198/#355)** | **MEASURED.** One per-repo singleton row (`repo_event_chain_heads`) is `FOR UPDATE`-locked by *every* event append (hash-chaining requires single-writer). `supervisor.progress` = 98.6 % of events floods it: **112,682 rows in minute 16:02, 98,008 in the single second 16:02:50, all from ONE `repository_id`** (`event_id 15,116,075..15,311,876`; burned ~196k event_ids that minute). | `events` total 13,596,859 rows (`event_id 1..15,401,346`); `supervisor.progress`=13,412,409; per-min p99=11,666, max=112,682 over `event_id 13,056,470..15,314,352` (7d) | `go/pkg/db/sql/owner/0004_phase2_events.sql:121-124` (the `FOR UPDATE`); `go/pkg/db/event_write.go:54-96`; `go/pkg/mutations/mutations.go:1388-1423` (`appendEvent`) | **Measure first:** additive `events.lock_wait_us` (§3 headline). **Then design:** keep `supervisor.progress` off the hash chain or batch it (a product decision — high-frequency liveness chatter should not serialize on the provenance chain). |
| **2** | **`work.complete` per-run-lock convoy (#355 holder candidate)** | **MEASURED holder pattern.** Holds `lockRunForJob` across **four** git-shelling ops, three unbounded (porter commit, source-publish commit, worktree anchor). Any sibling `work.complete`/`claim`/append on the same run queues behind it. | A: lock/hold topology; B: claimed→completed p99=7,771s, max=46,688s (`n=1070`, recent window) bundles this hold time but cannot isolate it (blind spot a) | `go/pkg/mutations/lifecycle.go:1118` (`withTxRetryOnDeadlock`), lock `:1137`; git at `write_scope_guard.go:231`, `artifact_durability.go:138/152/157/235`, `artifact_source_publish.go:102/115/120`, anchor `lifecycle.go:1235` | **Hoist git out of the lock-holding txn** — apply the #198 pre-compute pattern (`recovery.go:530-565`) to `work.complete`: do the porter/source/anchor git **before** `withTx`, let the txn record only resolved values. |
| **3** | **`run.prepare` git-in-txn (#355 visible victim)** | **MEASURED victim.** `runPrepare` shells `git rev-parse`/`git branch` **inside** its `withTx`, then appends `run.created`/`run.branch_confirmed`; under load its `append_event_row` is the 57014 casualty. No `isTransientDaemonLoadError` swallow → surfaces loudly as `exit_code=10`. | `run.prepare`: 370 allowed / 33 denied; failures carry `exit_code=10` (`audit_log`, `rpc/envelope.go:31`) | `go/pkg/mutations/run.go:27-29` (`withTx`→`runPrepare`), git at `run.go:815/828/841` via `:921/:931`, append `:1056/:1063` | **Same hoist:** compute `currentGitHead`+`gitEnsureBranchRef` before `withTx`; inject resolved branch/created-flag; txn records only durable values. |
| **4** | **`worktree.gc` repo-wide lock × N worktrees** | **PREDICTED from source (not yet fired in a repro).** Holds the **repo-wide** `lockRepo` while looping every active worktree and shelling git 2–3× each (probe + per-artifact `git show` + `worktree remove --force`). Highest *structural* blast radius (blocks every run on the repo). | A topology (no live convoy row captured — repro D would) | `go/pkg/mutations/worktree.go:536`, lock `:548`; git `:580/:615/:634` | Hoist the git probes/removes out of the `lockRepo` window; take the lock only to record decisions. |
| **5** | **`run.integrate` repo-wide lock across plumbing** | **PREDICTED from source.** Holds `lockRepo` across `merge-tree`/`commit-tree`/`update-ref` (unbounded), at run completion — overlaps the busiest moment. | A topology | `go/pkg/mutations/integrate.go:27`, lock `:48`; git `:87/:117/:145/:203/:223` | Same hoist; or bound the plumbing under a child timeout outside the lock. |
| **6** | **`artifact.publish` S3-under-lock** | **PREDICTED (conditional).** Holds `lockRunForJob` across an unbounded blob `PutBytes` when blob storage is enabled. | A topology | `go/pkg/mutations/artifact.go:75/76`, upload `:314` | Upload blob before the lock; txn records the resolved blob ref only. |
| **7** | **#322 claim-starvation** | **MEASURED partial.** `max_active_jobs` is enforced **only in the planner** (`runreconcile.PlanLaunch`), not in the atomic claim txn — DB will over-grant a direct `work.await_packet`. No discriminator distinguishes "correctly quiet" from "wedged". | `runreconcile_test.go:114`, `rundrive_test.go:47` exercise the planner cap only | `go/pkg/runreconcile/runreconcile.go:138-140` (planner `break`); **gap:** `go/pkg/mutations/claim.go:193` (`claimChosenJob` has no cap check) | DB-side `COUNT(in-flight) < max_active_jobs` guard in `claimChosenJob`; + the **claimable>0 ∧ advance_gap>N** discriminator latched in the sweep cursor (§3 row c). |
| **R-1** | **Sweep-suicide (P0)** | **RULED OUT (error path).** A non-context sweep error is caught per-iteration and the loop continues; only context-cancel / `MaxSweeps` cancels. Live proof: 5 `sweep_degraded` cursors coexist with 326 `active` + a current `last_sweep_at`. **Residual: panic** — there is **no `recover()` anywhere in `go/`**, so a panic on the sweep/RPC goroutine still crashes the process. | `scheduler_cursors`: 326 `active`, 5 `sweep_degraded`; `daemon.recovery_sweep`=149,962, cadence p90/p99=60s | `go/pkg/recovery/scheduler.go:56-74`; `go/pkg/recovery/sweep.go:64-86`; `go/cmd/striatumd/main.go:817-835` (cancel only on `MaxSweeps`/non-context) | Wrap `SweepOnce` and the RPC dispatch goroutine (`rpc/server.go:176`) in `recover()` that latches a durable `daemon_health(component, last_ok_at, healthy)` row, then re-panics or backs off. |
| **R-2** | **#325 deadlock (40P01)** | **RULED OUT.** RFC 0104 `lockRun`-first ordering eliminates the `{sessions,runs}` cycle; `withTxRetryOnDeadlock` retries the loser ×3. | `verdict.recorded`=491, no raw 40P01 surfaced; lock-order guard tests pass | `go/pkg/mutations/mutations.go:456/462/503-523`; `artifact.go:76`; `lifecycle.go:1137` | **Residual:** no regression counter — emit `deadlock.retry_exhausted` at `mutations.go:523` so a new unguarded path that re-inverts the order is loud. |
| **R-3** | **queued→claimed / claimed→completed tails** | **RULED OUT as daemon latency.** queued→claimed p99=6,238s, max=145,618s; claimed→completed p99=7,771s — these are **operator launch latency + parked-job wait + agent think-time**, not daemon work (boundary: both endpoints are state transitions, not lock events). Flagged so they are not mistaken for a daemon bottleneck. | queued→claimed `n=1263`; claimed→completed `n=1070` (recent window, partition `(repository_id,job_id)`) | n/a (cross-validated: event clocks agree with `audit_log` to ≤1s, §evidence) | None — correct behavior. Do **not** instrument; would violate the refusal register (agent think-time). |

---

## 2. Reproduction protocols (deterministic, seeded-fixture replayable)

Each protocol states **up front**, as a falsifiable prediction, the exact
`audit_log`/`events` rows it should emit. PG-gated tests skip unless
`STRIATUM_PG_TEST_URL` is set (`go/pkg/pgtest`). Of the five, only the **#355
convoy (Repro B) lacks existing scaffolding** — build it; the other four already
have seeded fixtures.

### Repro A — #325 deadlock / 40P01
- **Scaffolding (exists):** `go/pkg/mutations/run_lock_deadlock_test.go::TestRunLockClaimVerdictSweepDeadlock` (`:34`); fixture `seedRunLockRaceFixture` (`:136`). Lock-order guard: `run_lock_guard_test.go::checkLockRunFirst` (`:49`).
- **Steps:** seed one run + completed upstream job + running review job + sibling claimant; from a barrier race ×80: 1 `HandleRecordVerdict(accept)` (jobs→runs→sessions) + 6 `HandleClaimNext` (sessions→runs) + 1 `SweepRun`.
- **Falsifiable prediction:** **zero** raw `40P01` surfaced to any caller (lockRun-first prevents the cycle); tolerated benign outcome `claim: … session is closed`. No `audit_log` row with `exit_code=10` carrying `sqlstate:40P01`. **If a raw 40P01 appears → a new path skipped `lockRun`** (mechanism falsified for that handler).

### Repro B — #355 convoy / run.prepare 57014  *(scaffolding gap — build this)*
- **Closest existing:** `await_transient_load_test.go:23` injects a synthetic `57014` to prove the *await* loop swallows it — it does **not** reproduce the chain-head convoy. Build from `pgtest.Pool` + `intgSeedRepo`/`intgSeedRun`.
- **Steps:** (1) on the test role `SET statement_timeout='2s'` and leave `lock_timeout=0` (mirror prod baseline). (2) Open tx-A: `SELECT last_hash FROM striatumd.repo_event_chain_heads WHERE repository_id=$r FOR UPDATE` and **hold** (simulates a holder parked on git). (3) Concurrently fire ≥2 `HandleRunPrepare` on the same repo — each blocks on the chain-head `FOR UPDATE` (`0004_phase2_events.sql:124`). (4) After 2s the contenders are canceled.
- **Falsifiable prediction:** blocked `run.prepare` returns `append_event_row (sd): … SQLSTATE 57014` → `audit_log.method='run.prepare' exit_code=10` (**not** swallowed — `run.prepare` has no `isTransientDaemonLoadError` guard). **No** new `run.created`/`run.branch_confirmed` for the failed prepare (whole txn rolled back). `daemon.recovery_sweep` cadence shows **no gap > 60s** (sweep unaffected → confirms doctor stays green). Contrast control: the same 57014 on `HandleAwaitPacket` **is** swallowed (`mutations.go:489`, proven by `TestAwaitPacketSwallowsStatementTimeoutAsTransient`) — which is precisely why the convoy is visible on `run.prepare` but invisible on `await`. **If run.prepare returns `no_work`/`transient_daemon_load`, the victim moved off run.prepare** (falsified).

### Repro C — sweep-suicide (error path FIXED; panic path open)
- **Scaffolding (exists):** `go/pkg/recovery/scheduler_test.go`; `sweep.go::ActiveRunSweep`.
- **Steps:** drive `RunScheduler` with a `SweepOnce` returning a non-context error on tick 1 then succeeding, `MaxSweeps=3`, instant `Wait` stub. Separately, a `SweepOnce` that `panic()`s.
- **Falsifiable prediction:** error case → `result.Sweeps==3`, `FailedSweeps==1`, `Reason==max_sweeps_reached`, `cancel()` **not** called for the sweep error (`main.go:830` only on `MaxSweeps`). Live corroboration: 5 `sweep_degraded` cursors + 326 `active` + current `last_sweep_at`. Panic case → process aborts (`grep recover() go/pkg/recovery go/cmd/striatumd` = empty). **If the error case calls `cancel()` → suicide regressed.**

### Repro D — #322 / #245 starvation
- **Scaffolding (exists):** `runreconcile_test.go:114`; `rundrive_test.go::TestRunDriveHonorsMaxActiveJobs:47`.
- **Steps:** seed a run with `parallelism.max_active_jobs:1` + 5 deps-met queued jobs. `PlanLaunch` → expect 1 launch, 4 left queued (`runreconcile.go:138 break`). Mark the single lane `running`, **never complete it**; re-run `PlanLaunch`.
- **Falsifiable prediction:** 1 `LaunchRegisterFresh`/`LaunchAdoptExisting`, 4 jobs stay `queued`; events show **1** `queue.claimed` with **no following `job.completed`** while `claimable_job_count>0`. **No** cap-enforcement event from `claimChosenJob` (`claim.go:193`) — confirming the cap is planner-only and the DB would still grant a direct `work.await_packet`. Discriminator `claimable>0 ∧ advance_gap>N` ⇒ WEDGED; the same query with the lane completed (claimable→0) ⇒ correctly-quiet. **If `claimChosenJob` refuses the over-cap claim → the cap moved into the txn** (gap closed, falsified).

---

## 3. Blind-spot ledger

Hot-path latencies the current event vocabulary **cannot** resolve, each with the
**smallest** additive, NULLABLE, append-only, crash-survivable instrument written
**inside the txn that already emits the event** (default-NULL for old rows, no
backfill, no new event type). Boundary discipline: `events.created_at` marks a
*state transition*, not lock-acquire or commit — an inter-event delta is never
lock-hold time.

| # | Blind spot | Why unresolvable today | Smallest lawful instrument (`table.column : type` — where written) |
|---|---|---|---|
| **(a) HEADLINE — #198/#355 lock-hold** | Intra-txn time `append_event_row` holds `repo_event_chain_heads … FOR UPDATE`. | **No lock-acquire/commit event pair exists.** One timestamp at the body of the SD fn; adjacent-event deltas = think-time. | **`striatumd.events.lock_wait_us bigint NULL`** — bracket the `FOR UPDATE` SELECT at `0004_phase2_events.sql:121-124` with `clock_timestamp()`, store the µs delta. Additive, nullable, crash-survivable (rides the committing txn), default-NULL for all 13.6M existing rows. **The first column that makes the convoy a measured quantity instead of an inference.** Also resolves the open question of whether the effective timeout is 60s (pool RuntimeParam, `connection.go:287`) or 600s (role) by recording the actual wait. |
| **(b) recovery-sweep wall time** | `daemon.recovery_sweep` is appended **after** the sweep txn commits (`recovery.go:1325`) → completion marker only, not the body span. | Nothing records sweep-txn open vs commit. | **`payload_json.sweep_duration_ms`** on the existing `daemon.recovery_sweep` event — stamp a monotonic start at `HandleRecoveryAuto` entry (`recovery.go:520`), fold `time.Since(start)` into the `result` map already embedded in the payload (`recovery.go:1327`). **Zero schema change** (JSONB key). |
| **(c) work.heartbeat true cadence + the #322 discriminator** | Measured heartbeat cadence (p50=0/p90=68/p99=270s, `n=10,806`, `event_id 659..15,028,816`) is polluted by supervisor auto-refreshes (`supervision.go:329`) that fire on every PTY frame — masking whether the agent's own `work.heartbeat` keeps the 330s deadline (`liveness.go:217-218`); max gap 1011s (3× deadline) survived only via auto-refresh. And nothing distinguishes "correctly quiet" from "wedged". | `lease.heartbeat` cannot tell source; no claimable-vs-advance latch. | **`lease.heartbeat` `payload_json.source text`** = `work_heartbeat` \| `supervisor_progress` (set at the two emit sites). **Plus** extend the sweep cursor `last_result_json` (`sweep.go:201-217`, written outside the lock) with `{claimable_job_count, last_lane_advanced_at}` → page predicate **`claimable>0 ∧ now()-last_lane_advanced_at>N`**; never page when `claimable==0`. |
| **(d) git-shell duration in sweep / worktree ops** | `expireLeases` (`recovery.go:586`) + worktree anchoring shell git; duration invisible (git was correctly moved *out* of the txn at `recovery.go:559-565` but never timed). | No timer around the pre-txn git. | **`payload_json.worktree_anchor_ms` / `git_shell_ms`** on `daemon.recovery_sweep` — wrap `buildRunWorktreeAnchorOracle` (`recovery.go:561`) with a monotonic timer, fold into the oracle result already serialized into the payload. JSONB-only. |
| **(e) general lock-acquire→commit pair** | No mutation records a commit timestamp distinct from its body timestamp → claim/complete/verdict acquire→commit windows unmeasurable. | Same root as (a), whole `appendEvent` family. | **Cannot be closed by one in-txn column** (commit happens after the body). Once (a) lands, p99 lock-*wait* per `event_type` is covered; the full acquire→commit pair belongs to a **read-side `pg_stat_activity` sampler / view** (instrumentation hook 2), not a gold-plated column. **Deferred deliberately.** |

**Clock cross-validation (cited invariant).** `artifact.publish` audit rows joined to
`artifact.published` events on `repository_id` + nearest `ts` (±10s), `n=537`,
`audit_id 6,806,952..17,052,270`, `event_id 4,303,831..14,978,756`: skew
`event.created_at − audit.ts` is **0s at p50/p90, ≤1s for 98.1 % (527/537)**, max
+9s (publish straddling a second boundary). Both are `now()` on the same daemon
host (no host-to-host skew). **Conclusion: event-derived wall-clock spans are
admissible (to sub-second) — but still not as lock-hold time.**

---

## 4. Instrumentation plan (cheapest → deepest)

Turn on the most-diagnostic-cheapest first; do not touch daemon control flow until
the data demands it. **Live constraints (verified):** owner role `halbritt` is
**not** a superuser (cannot `ALTER SYSTEM` / `LOAD` / `pg_stat_statements_reset()`
/ restricted-GUC `ALTER ROLE`) — so superuser steps are split out for
`sudo -u postgres`. `pg_stat_statements` is **already preloaded and populated**, so
its reads need zero changes and no restart. Scripts `instrument.sh` / `revert.sh`
ship alongside this report.

| Hook | What it illuminates | Enable | Revert | Overhead budget | Stays-local proof |
|---|---|---|---|---|---|
| **1. `pg_stat_statements` reads + `auto_explain`** | Which statements accrue `total_exec_time` while holding locks (the git-in-txn `append_event_row` path); slow-mutation plans + nested statements | Owner: read pgss now (already on). Superuser: `pg_stat_statements_reset()` then `ALTER SYSTEM SET session_preload_libraries='auto_explain'` + `auto_explain.log_min_duration='200ms'`/`log_analyze`/`log_nested_statements` + `pg_reload_conf()` (no restart for `session_preload_libraries`). Replay one 60s sweep + one claim/heartbeat cycle. | Superuser `ALTER SYSTEM RESET …` + reload. Owner side is read-only. **Do not DROP pgss** (pre-existing). | pgss ≈ sub-µs/stmt (sunk). `log_analyze` overhead applies **only to statements >200ms** — concentrated on the convoy victims; ≪1 % hot-path. | pgss/auto_explain write to the cluster catalog + PostgreSQL log on the loopback-only cluster. No row leaves the box; no new sink/listener. |
| **2. 1 Hz `pg_locks ⋈ pg_stat_activity` sampler → tagged CSV** | The **transient** lock graph (40P01/57014 vanish in ms): `wait_event`, blocking pid, held-lock mode, blocked statement — caught while deliberately provoking parallel completion | `instrument.sh` runs the sampler (owner-doable; `pg_locks`/`pg_stat_activity` readable). Then provoke: complete/publish two sibling lanes on one run near-simultaneously. | `revert.sh` kills the sampler PID + deletes the CSV. No PG state touched. | One short-lived read-only connection, 1 query/s scanning ~100 lock rows. <0.1 % CPU. Separate connection → `SELECT` on `pg_locks` takes no row locks → cannot perturb the daemon. | Read-only `SELECT` over loopback; CSV under `/tmp` (operator-local diagnostic, deleted on revert); no durable sink/export/listener. |
| **3. `/v1/health?debug=1` piggyback** | pgxpool saturation (`AcquiredConns` vs `MaxConns` — bears on #322 + convoy connection-holding), `runtime.NumGoroutine()` (sweep/supervisor leaks), `runtime/metrics` (GC — confirm-or-ignore only) | Build + deploy the gated branch (`make install` **then** `systemctl --user restart striatumd` — install alone does not restart). Handler: `go/pkg/webservice/service.go:93`; pool via `db.Pool.RawPool.Stat` (`connection.go:146`) threaded as a nil-safe `PoolStat func() *pgxpool.Stat` in `webservice.Config`. | Redeploy without the diff, or simply never pass `?debug=1` (endpoint is **byte-identical** when the param is absent). | Zero on the hot path (branch runs only with `?debug=1`); `Stat()` is an atomic snapshot, `NumGoroutine()` O(1). | Reuses the existing loopback `STRIATUM_DAEMON_WEB_*` listener behind bearer auth; no new port/route; emits aggregate counters only, never row/transcript content. |
| **4. `defer`-timing decorator (`STRIATUM_TIMING=1`)** | Daemon-side Go wall-time of a mutation/sweep that PG attributes to "waiting" — separates "blocked in PG" from "slow in Go between statements" (the git subprocess inside the lock) | Wrap `withTx` (`mutations.go:379`) + `SweepOnce` (`sweep.go:21`, via the `OnSweepError`/sweep seam at `main.go:817-826`) in an env-gated `defer time.Since(t0)` logged via stdlib `log`; set env in the unit + restart. | Unset env + restart. No code path when env absent (defer never registered). | ~1µs + 1 log line/mutation **when on**, zero when off. Logged **after** commit/rollback — never inside the lock-holding txn. | Emits via stdlib logger → systemd journal (loopback-local); `dur_ms` only, no payload; no new sink/listener. |
| **5. `expvar` / `net/http/pprof` (loopback only)** | block/mutex profiles if 1–4 can't localize a stall | Register `/debug/pprof/*` on the **existing** loopback mux behind `STRIATUM_PPROF=1` + bearer auth + restart. | Unset env + restart. | block/mutex profiling has measurable overhead — enable for one repro window, then revert. Zero when off. | Bound to the existing loopback listener behind auth; **must never bind a new `0.0.0.0`/extra port**. |

**Demoted:** allocation hooks (`GODEBUG=gctrace`, heap profiles) — this daemon is
lock/IO/subprocess-bound, not alloc-bound. Use only to confirm-or-ignore, never to
lead.

---

## 5. Refusal register

| Measurement DECLINED | Boundary clause it would violate |
|---|---|
| Per-keystroke / agent-process content timing (instrumenting PTY FIFOs / lane stdio for latency) | "Marker files, tmux panes, terminal output … are not authoritative workflow state" + "no durable transcript capture/export." |
| Transcript / trajectory-log **content** sampling for "what was the agent doing" timing | "Operator-local PTY logs … are private diagnostics only" + "no durable transcript capture/export." |
| Any always-on metrics listener (Prometheus `/metrics`, statsd, OTel) | "no new always-on listener" + "no telemetry export." (Hook 3 piggybacks the existing loopback `/v1/health` precisely to avoid this.) |
| Hosted dashboards / cloud APM (Grafana Cloud, Datadog, any exporter past loopback) | "FORBIDDEN: hosted services, cloud APIs, telemetry export, external persistence." |
| Durable external metrics sink (time-series DB, off-box table, S3/Garage export of samples) | "no external persistence." (The sampler CSV is `/tmp` scratch, deleted on revert.) |
| A failure-marker row written **inside** the lock-holding mutation txn | Non-perturbation gate: lengthens the lock-hold window and could fail-closed and kill the mutation. The cure must never become a new way to kill the daemon. (The `lock_wait_us` column is the *safe exception* — it times an event the txn is already committing, adding no new lock scope.) |
| `CREATE INDEX` directly on owner-held `striatumd.events` | Additive-column discipline: any index on `events` routes through an **owner bundle**, never a direct `CREATE INDEX`. The doctor check uses existing indexes (`idx_events_job` / the hot-read index from owner bundle `0011`). |

---

## 6. Recommended first move — the smallest change that yields the most

Ship the **convoy hold-time additive column + a loud "in contempt of lock" doctor
check + stop swallowing the lock-wait 57014**. This converts the single most
expensive recurring diagnostic — hand-reconstructing the #198/#355 convoy from PG
internals every time it recurs — into a standing, durable, crash-survivable signal
that regresses **loudly**.

1. **Additive column (crash-survivable, in-txn):** add `events.lock_wait_us bigint NULL`,
   written inside `append_event_row` at `go/pkg/db/sql/owner/0004_phase2_events.sql:121-124`
   by bracketing the chain-head `FOR UPDATE` SELECT with `clock_timestamp()`. The txn
   already INSERTs that row, so this adds **no new lock scope and no new write** —
   it commits/rolls back atomically, default-NULL for the 13.6M existing rows.
2. **Loud doctor check:** add a block to `HandleDoctor` (`go/pkg/reads/doctor.go`,
   following the `problems`/`problemRecords` pattern used by the `needs_operator`
   block) that reads recent `events` where `lock_wait_us` exceeds a threshold (e.g.
   approaching the effective statement timeout) and appends a
   `convoy_lock_contention.<run_id>` problem that flips `ok=false`. **Red doctor =
   stop-and-fix** instead of re-deriving the convoy by hand. Reads use existing
   indexes — no new index, no owner bundle.
3. **Stop swallowing the loud signal:** narrow `isTransientDaemonLoadError`
   (`go/pkg/mutations/mutations.go:483-494`) so a 57014 raised **while waiting on a
   `lockRun*` / chain-head lock** surfaces as a distinct named `lock_convoy_timeout`
   error rather than the benign "busy, retry" class. This is why #355 is invisible
   today: the loud signal is reclassified as benign — that is the "doctor stays
   green" mechanism.
4. **(Now a valid, not-yet-done lever — see §0 disclosure):** set a finite,
   role-level `lock_timeout` on `striatumd_rw` so convoy waiters fail *fast and
   legibly* instead of blocking until `statement_timeout`. Owner-revertible
   (`ALTER ROLE striatumd_rw SET lock_timeout='Ns'` / `RESET`), so the threshold can
   be tuned to bracket the git-subprocess duration once (1) is measuring it. The
   baseline today is **no role-level `lock_timeout` (inherits 0/disabled)**.

The structural fix these signals point at is the **git-hoist** (rows 2–6 of the
bottleneck ledger): apply the already-landed #198 pre-compute pattern
(`recovery.go:530-565`) to `work.complete`, `run.prepare`, `worktree.gc`,
`run.integrate`. The instrumentation (hooks 1–2) then becomes the one-time tool to
confirm the column records the right thing and to set the doctor threshold — after
which the convoy never has to be re-derived by hand again.

---

## 7. Acceptance self-check

- [x] Every perf number cites a row-range or captured artifact (event_id/audit_id ranges + partition + counts throughout §1–§3).
- [x] Every timestamp-derived latency declares its physical boundary (boundary table in §3; deltas labelled think-time vs transition; tails ruled out as agent latency in row R-3).
- [x] No recommendation requires data to leave loopback or adds a durable external sink (stays-local proof per hook §4; refusal register §5).
- [x] No proposed instrumentation runs inside a lock-holding txn or can kill the daemon (`lock_wait_us` times an already-committing event; markers forbidden in-txn per §5; timing logged post-commit).
- [x] Tail/distribution (never mean) on every contended path; convoy treated as bimodal (§1, §3 stage tables use `percentile_cont`).
- [x] Every finding terminates in a code-located fix tied to a tracked hazard (#322/#325/#245/#198/#355 + sweep-suicide), with `file:line`.
- [x] `instrument.sh` is fully reverted by `revert.sh` to baseline behavior (owner side read-only; sampler killed + CSV removed; superuser RESETs printed).
- [x] **Method-integrity:** a self-inflicted false fact (injected `lock_timeout=3s`) was caught by the evidence rule, corrected, and the live daemon role restored to baseline (§0).

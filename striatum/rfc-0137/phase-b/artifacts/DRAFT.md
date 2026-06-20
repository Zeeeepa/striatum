# RFC 0137 Phase B — failure-mode taxonomy (apoptosis/necrosis + lifecycle emit)

author: author-author-001

Phase B of RFC 0137 (`striatumd` Prometheus exporter) is implemented and green,
built on top of the landed Phase A read-path skeleton. It adds the
failure-mode-shaped taxonomy — the apoptosis/necrosis spine, the F-A6 reversible
liveness counter, lease-transition counts, and the wedge-age / liveness-margin
histograms — folded from the durable `striatumd.events` ledger at the sweep tick
so the counters are transaction-safe and restart-consistent by construction.
Phase C and Phase D were deliberately **not** implemented.

This is a re-attempt: a prior draft on this branch failed review (`needs_revision`)
on three source-grounded findings. **All three are fixed and proven green against
a real PostgreSQL**, not just asserted. The fixes are summarized first because
they were the gate.

## Prior-review findings → fix (the gate)

### F1 — `run_wedge_age_seconds` reset on unrelated run events → FIXED

The wedge-age fold measured `now − MAX(events.created_at)` across **every** event
for a non-terminal run. The recovery sweep records a run-scoped
`daemon.recovery_sweep` event every tick (`recovery.go:1358`), so a wedged run
looked freshly active and the age stayed low exactly when it should grow.

Fix (`collector.go`): the histogram is now derived from **job-state-advance
evidence only** — a closed `jobStateAdvanceEventTypes` set (`job.*` lifecycle
transitions) filtered in SQL via `e.event_type = ANY($3)`. `daemon.recovery_sweep`,
`lease.heartbeat`, and every other non-job event are excluded. The closed set is
the single source of truth shared by the SQL filter and the in-process
`isJobStateAdvanceEvent` predicate.

> **Latent bug also fixed.** Repairing this exposed that the pre-existing
> `nonTerminalRunStates` was a `[]any`, which **pgx cannot encode as a Postgres
> array** — so `runWedgeAges` had been silently erroring (its error is swallowed
> by the best-effort Phase B fold) and the wedge family never populated at all.
> Both array params are now `[]string` (the codebase idiom, e.g. `claim.go:425`,
> `supervision_control.go:925`). The behavioral test below would have stayed empty
> under the old binding; it now produces a real observation.

Regression coverage: `TestRunWedgeAgeCountsOnlyJobStateAdvances` (pg, behavioral)
seeds a 90-min-old `job.queued` and a 30-s-old `daemon.recovery_sweep` and proves
the age reflects the old job event (`le="60"`/`le="3600"` buckets stay empty),
plus `TestJobStateAdvanceSetExcludesNonJobEvents` (unit) pins the set excludes
`daemon.recovery_sweep`.

### F2 — `lease_handoff` / `supervisor_drained` were decorative → FIXED

Both apoptosis reasons were in the enum but `classifyLifecycleEvent` never
produced them. They are now wired to their real durable events:

- **`lease_handoff` ← `lease.released`** carrying a handoff (`lifecycle.go:311`
  emits `reason="superseded"`; `lifecycle.go:983` emits `reason` + `transfer`).
  A release is a handoff when `transfer=true` or its reason ∈
  `{superseded, recovery_transfer, operator_transfer}`. A completion/expiry
  release is **not** a handoff (completion is already counted via `job.completed`;
  expiry surfaces in `lease_transitions`) — so `lease_handoff` stays a
  healthy-handoff signal, not a generic release tally.
- **`supervisor_drained` ← `supervisor.stopped`** (`supervision_control.go:631`
  for an operator/clean stop; `mutations.go:1681` for the reconcile-sweep reaping
  a terminal-run supervisor — the `helper_events.drained` drain path's durable
  termination event). A supervisor stop is a programmed drain of the supervisor's
  own lifecycle → apoptosis, origin `supervisor`.

The collector SQL (`lifecycleEventCounts`) now selects both event types and, for
`lease.released` only (guarded by a `CASE` so a free-text `supervisor.stopped`
reason never widens cardinality), the `reason`/`transfer` payload fields. None of
these reach the wire — render emits only the closed origin/reason enum.

Coverage: `TestApoptosisReasonsAreAllProducedFromRealEventTypes` (unit) folds one
event of each real type and then asserts the **entire** closed apoptosis enum is
covered — a future hollow reason fails here. `TestApoptosisHandoffAndDrainFoldedFromDurableEvents`
(pg, behavioral) proves the collector SQL itself folds a real `lease.released`
(transfer) and `supervisor.stopped` into the metric while a plain `completed`
release does not.

### F3 — F-A6 test was a tautology → FIXED

The old `TestLivenessMissCanRecoverWithoutNecrosis` folded two hand-written
`LifecycleEvent` literals. It is now a **behavioral** test in package `mutations`
that drives the REAL emit path: it seeds an active-but-stalled session, calls the
real `refreshRunLiveness` to classify a stall and emit
`session.liveness_deadline_missed` (`recovery.go:1229`), makes the session's
protocol activity fresh, calls `refreshRunLiveness` again to emit
`session.liveness_recovered` (`recovery.go:1244`), then folds the **real**
`metrics.Collector` and asserts on the wire surface that the liveness counter
moved on both halves while `striatum_necrosis_total` shows **no series at all**.
The metrics-package unit `TestLivenessFoldRoutesReversiblePairOutsideNecrosis`
keeps the fast routing check.

## Mechanism chosen for tx-safe, restart-consistent counters — and why

**Fold-from-durable-events at the sweep tick** (RFC 0137 §"Design guidance", the
*preferred* option), not in-memory post-commit atomics.

- **Tx-safe.** Counters are derived from the append-only `striatumd.events` table.
  A lifecycle transaction that rolls back never inserted its event, so it can
  never over-count; the count only reflects committed events. No post-commit
  ordering hazard inside the delicate recovery `withTx`.
- **Restart-consistent.** The counter is re-derived from durable history every
  tick, so a daemon restart resumes at the true cumulative value rather than
  resetting to zero. (Prometheus tolerates resets; we avoid them anyway.)
- **No import cycle.** Derivation lives entirely in `go/pkg/metrics` reading
  events via the existing `Querier`; `metrics` never imports `mutations`. The one
  cooperating edit on the `mutations` side — the `closeStalledOwningSession`
  `lifecycle_metric` site tag and the necrosis-domain guardrail — uses the
  sanctioned `mutations → metrics` direction (proven acyclic by `go build ./...`).
- **Cost.** Fixed, small `GROUP BY` aggregates at the ~60 s tick, never on the
  scrape path, preserving Phase A's O(1) lock-disjoint scrape.

### Tagging apoptosis vs necrosis "at the site"

Apoptosis and necrosis share the same terminal DB transition, so the split is
decided from the durable event each path writes (`classifyLifecycleEvent`):

- **Apoptosis** reads unambiguous healthy-termination event types the terminator
  already emits: `run.completed`→`run_completed`, `job.completed`→`job_succeeded`,
  clean `session.closed`→`session_closed_clean`, `supervisor.stopped`→
  `supervisor_drained`, and a handoff `lease.released`→`lease_handoff`.
- **Necrosis** reads intentional tags the recovery/liveness paths write for an
  *unannounced* exit: the `closeStalledOwningSession` `lifecycle_metric` tag
  (`recovery_decision_tree.go:1435`, necrosis for a confirmed-dead stall class,
  `recovery_transfer` for an honest-stall transfer that the fold skips) and the
  `recovery_exhausted` blocker kind on `run.escalated` / `recovery.job_quarantined`.

The `mutations` source already carried the necrosis site-tag and the union
guardrail from the prior attempt (the review did not fault them); this slice adds
no new `mutations` source edits — only the new behavioral test file — keeping the
delicate recovery code untouched.

## Enum → source-constant anchoring

The necrosis domain is pinned to EXACTLY the confirmed-dead set by
`TestNecrosisDomainMatchesConfirmedDeadConstants` (package `mutations`, where the
unexported constants are visible; imports `metrics`), built from the real
constants so renaming a value or adding a class breaks the build:

| metrics enum value | anchored source constant | location |
| --- | --- | --- |
| `NecrosisAgentPIDDead = "agent_pid_dead"` | `stallClassAgentPIDDead` | `recovery_decision_tree.go:153` |
| `NecrosisAgentExitedUnsealed = "agent_exited_unsealed"` | `stallClassAgentExitedUnsealed` | `recovery_decision_tree.go:159` |
| `NecrosisRecoveryExhausted = "recovery_exhausted"` | `recoveryExhaustedBlockerKind` | `recovery_escalation.go:15` |

Apoptosis reasons map to real event types (F2): `run_completed`←`run.completed`,
`job_succeeded`←`job.completed`, `session_closed_clean`←clean `session.closed`,
`lease_handoff`←handoff `lease.released`, `supervisor_drained`←`supervisor.stopped`.
The liveness reasons (`deadline_missed`, `recovered`) are the closed F-A6 enum.

## How F-A6 is enforced

`liveness_deadline_missed` is a **reversible** pre-death observation
(`session.liveness_recovered` proves it is not death). It is kept out of necrosis
three ways: (1) domain exclusion — it is not a `NecrosisReason`, asserted absent
by the guardrail; (2) routing — `classifyLifecycleEvent` maps both liveness event
types to `ClassLiveness` → `striatum_liveness_deadline_events_total`, never
necrosis; (3) executable behavioral proof — `TestLivenessMissCanRecoverWithoutNecrosis`
drives the real `active → deadline_missed → recovered` path and asserts the
liveness counter moved while `necrosis_total` rendered no series.

## Acceptance criteria → proving test

| Criterion | Proving test |
| --- | --- |
| Each family exists with closed-enum labels pinned to source constants; cardinality cannot grow with run/job count | `TestNecrosisDomainMatchesConfirmedDeadConstants`; `TestLeaseReasonBucketingIsBounded`; golden render emits only closed enums |
| **No apoptosis reason is hollow** (req. 1 / F2) | `TestApoptosisReasonsAreAllProducedFromRealEventTypes` (unit, full-enum coverage) + `TestApoptosisHandoffAndDrainFoldedFromDurableEvents` (pg) |
| **Wedge age from real job-state advance only** (req. 2 / F1) | `TestRunWedgeAgeCountsOnlyJobStateAdvances` (pg, behavioral) + `TestJobStateAdvanceSetExcludesNonJobEvents` (unit) |
| **F-A6 behavioral** (req. 3 / F3) | `TestLivenessMissCanRecoverWithoutNecrosis` (pg, drives the real refresh) + `TestLivenessFoldRoutesReversiblePairOutsideNecrosis` (unit) |
| Necrosis fold is real (F-A6 zero is not vacuous) | `TestNecrosisIsCountedFromTaggedSiteEvents`; `TestApoptosisHandoffAndDrainFoldedFromDurableEvents` (necrosis stays empty on healthy events) |
| Apoptosis/necrosis tagged at the lifecycle site | `TestRecoveryTransferCloseIsNotApoptosis`; `TestApoptosisClassifiedFromHealthyEventTypes` |
| Exfiltration contract holds with the new families | `TestMetricsRedactionGoldenAndForbiddenContent` (golden now exercises `lease_handoff`/`supervisor_drained`; forbidden-regex still green) |
| Scrape stays O(1), zero-DB-query, lock-disjoint | `TestScrapeIssuesZeroQueries`; `TestConcurrentScrapesSeeIdenticalSnapshot` |

## Files touched

| File | Change |
| --- | --- |
| `go/pkg/metrics/taxonomy.go` | `LifecycleEvent` gains `LeaseReason`/`LeaseTransfer`; `leaseHandoffReasons` closed set; `classifyLifecycleEvent` wires `supervisor.stopped`→`supervisor_drained` and handoff `lease.released`→`lease_handoff`. |
| `go/pkg/metrics/collector.go` | `lifecycleEventCounts` folds `lease.released`/`supervisor.stopped` (lease payload guarded by `CASE`); `jobStateAdvanceEventTypes` closed set + `isJobStateAdvanceEvent`; `runWedgeAges` filters to job-state advances; `nonTerminalRunStates` / arrays are now `[]string` (pgx-encodable). |
| `go/pkg/metrics/redaction_test.go` | Sentinel events add a handoff `lease.released`, a completion `lease.released` (must NOT count), and a `supervisor.stopped`. |
| `go/pkg/metrics/testdata/metrics_golden.txt` | Regenerated for the two new apoptosis series; forbidden-content regex re-verified. |
| `go/pkg/metrics/taxonomy_test.go` | F2 non-hollow + handoff-vs-completion + F1 set-membership tests; old tautological F-A6 test renamed to the unit routing test. |
| `go/pkg/mutations/metrics_failure_taxonomy_pg_test.go` | **New.** Behavioral pg tests: F-A6 (`TestLivenessMissCanRecoverWithoutNecrosis`), wedge-age (F1), handoff/drain fold (F2). No `mutations` source edits. |

## New families (RFC §3)

- `striatum_apoptosis_total{origin,reason}` — counter (all 5 reasons now non-hollow)
- `striatum_necrosis_total{origin,reason}` — counter (confirmed-dead only)
- `striatum_lease_transitions_total{from,to,reason}` — counter
- `striatum_liveness_deadline_events_total{reason}` — counter (F-A6 home)
- `striatum_run_wedge_age_seconds{origin}` — histogram (job-state-advance evidence)
- `striatum_liveness_deadline_margin_seconds{origin}` — histogram

## Verification commands and results

Run from the per-job worktree `go/` directory on `striatum/rfc0137-phase-b`. The
behavioral pg tests are gated on `STRIATUM_PG_TEST_URL` (skip cleanly without it,
like every integration test in `pkg/mutations`); they were run for real against a
throwaway local PostgreSQL 17 cluster.

- `make -C go build` → **PASS** (striatum, striatumd, supervisor-helper).
- `go build ./...` → **PASS** — confirms no `metrics`↔`mutations` import cycle.
- `go vet ./pkg/metrics/... ./pkg/mutations/...` → **PASS** (clean).
- `go test ./pkg/metrics/...` → **PASS** (11 tests incl. golden, scrape O(1)/zero-query, F2/F1 units).
- `go test ./cmd/striatumd` → **PASS** (Phase A `/metrics` wiring still routes).
- `go test ./pkg/mutations/...` (with `STRIATUM_PG_TEST_URL` set) → all pass
  **except** the pre-existing, environment-dependent `TestSpawnRunAsSpecResolvesLaneUser`
  (`spawn_grant_test.go`, wants host lane user `striatum-lane`) — unrelated to this
  change and the same failure the prior reviewer recorded. The four taxonomy
  tests pass for real:
  `TestLivenessMissCanRecoverWithoutNecrosis`,
  `TestRunWedgeAgeCountsOnlyJobStateAdvances`,
  `TestApoptosisHandoffAndDrainFoldedFromDurableEvents`,
  `TestNecrosisDomainMatchesConfirmedDeadConstants`.

## Scope confirmation — Phase C / Phase D NOT implemented

Explicitly **not** built: **Phase C** — no `Classification` taxonomy / `Register()`
refusal of `Forbidden`, no per-family series budget / `cardinality_clipped_total`,
no boot-time `metrics_allowlist.json` hash check, no `doctor_problems{class}`
collector, no `TestDoctorClassRejectsDynamicIdentifiers`. **Phase D** — no
capability-scoped `/metrics` filtering, no `metrics_repo_consent` gauge, no
`tick_status` publish-on-errored-tick, no bundled Prometheus rules, no cold
DB-projection tier. The bind stays loopback-only (inherited from Phase A). The
cardinality/privacy contract for the new labels is upheld by closed enums and the
golden + forbidden-regex redaction test; the *enforced* Classification/allowlist
machinery is Phase C as designed.

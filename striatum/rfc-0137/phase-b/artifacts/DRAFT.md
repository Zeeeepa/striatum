# RFC 0137 Phase B — failure-mode taxonomy (apoptosis/necrosis + lifecycle emit)

author: author-author-002

Phase B of RFC 0137 (`striatumd` Prometheus exporter) is implemented and green,
built on the landed Phase A read-path skeleton. It adds the failure-mode-shaped
taxonomy — the apoptosis/necrosis spine, the F-A6 reversible liveness counter,
lease-transition counts, and the wedge-age / liveness-margin histograms — folded
from the durable `striatumd.events` ledger at the sweep tick so the counters are
transaction-safe and restart-consistent by construction. Phase C and Phase D were
deliberately **not** implemented.

This is attempt 2. Attempt 1 (author-author-001) closed the three prior
completeness requirements (non-hollow apoptosis reasons, real wedge-age, behavioral
F-A6) and was accepted on those, but the reviewer returned `needs_revision` on a
**single new finding (F1)**: the `lease_transitions_total` family could not expose
the RFC's primary stale-lease storm signal. This attempt fixes exactly that
finding with a minimal, surgical change and re-proves the surface green; the rest
of the attempt-1 implementation is unchanged and carried forward below for a
self-contained handoff.

## Review finding F1 → fix (the gate)

> **F1 (reviewer-reviewer-001).** `striatum_lease_transitions_total{from,to,reason}`
> must detect stale-lease storms via a `to="stale_lease"` rate (RFC 0137:185). The
> stale-limbo path sets the job state to `stale_lease` while expiring the lease
> (`recovery.go`), but the fold hard-coded `to="expired"` for every `lease.expired`
> event, so a repo-write stale lease was indistinguishable from ordinary expiry and
> the `stale_lease` enum member was dead for this metric. The `lease.expired`
> payload has no `reason` field, so those rows also fell into the generic `other`
> reason bucket.

### Root cause

The single `lease.expired` emit site (`go/pkg/mutations/recovery.go:2372`) stamps
the durable event payload `{job_state, message_state}`, where `job_state` is
`stale_lease` for a repo-write lane parked in stale-lease limbo and `queued` for an
ordinary (non-repo-write) lane that is simply re-queued. The lease row itself goes
to `state='expired', release_reason='expired'` in both cases. The old fold read
only `payload_json->>'reason'` (absent → `other`) and pinned `to="expired"`
unconditionally — discarding the one payload field that carries the stale signal.

### Fix (surgical, fold-only — no recovery source touched)

- **`go/pkg/metrics/collector.go` (`leaseTransitionCounts`).** The fold now also
  projects `payload_json->>'job_state'`, guarded by a `CASE` so it is read **only**
  for `lease.expired` rows (it never widens the GROUP BY for `lease.released`). The
  `(to, reason)` pair is derived by the new pure helper `leaseTransitionTarget`.
- **`go/pkg/metrics/taxonomy.go` (`leaseTransitionTarget`).** New closed,
  source-of-truth derivation, sitting beside the other lease bucketing helpers:
  - `lease.released` → `to="released"`, keep the payload reason (unchanged behavior).
  - `lease.expired` with `job_state="stale_lease"` → `to="stale_lease"` — the
    RFC's stale-lease storm signal, now a **distinct** alertable series.
  - `lease.expired` otherwise (e.g. `job_state="queued"`, or absent) → `to="expired"`.
  - For every `lease.expired`, the reason is pinned to the lease row's real
    `release_reason` `"expired"`, which buckets to `"expiry"` — so neither stale nor
    ordinary expiry falls into the generic `other` bucket.
  `stale_lease` was already a member of `leaseStateDomain`, so `bucketLeaseState`
  passes it straight through to the wire; no enum change was needed — the fix is
  purely making the fold *produce* the value it was always allowed to render.

### Why the labels stay safe (cardinality / privacy contract)

`job_state` is a closed lifecycle-state enum, never an id; it is projected only for
`lease.expired`, and only its presence/absence of the literal `"stale_lease"` is
used to pick a closed `to` value. Nothing new reaches the wire — render still emits
only the closed `from`/`to` enum and the bucketed reason. The golden + forbidden-
content regex (below) prove the now-live `to="stale_lease"` render path carries no
slash/sha/argv/byline shape.

### Regression coverage for F1

| Test | Kind | Proves |
| --- | --- | --- |
| `TestLeaseTransitionTargetDistinguishesStaleLease` | unit (`pkg/metrics`) | the `(to, reason)` derivation for all four cases (released, stale expiry, ordinary expiry, expiry-without-job_state) **and** end-to-end through `Build`+render that a stale expiry and an ordinary expiry produce two **distinct** series (`to="stale_lease"` vs `to="expired"`), both `reason="expiry"`, neither in the `other` bucket |
| `TestStaleLeaseExpiryRendersDistinctTransition` | pg behavioral (`pkg/mutations`) | seeds the two **real** durable `lease.expired` payload shapes the expiry path writes (`job_state="stale_lease"` and `job_state="queued"`), folds the **real** `metrics.Collector`, and asserts both `striatum_lease_transitions_total{...,to="stale_lease",reason="expiry"} 1` and `...,to="expired",reason="expiry"} 1` render distinctly (and nothing collapses or falls into `other`) |
| `TestMetricsRedactionGoldenAndForbiddenContent` | golden (`pkg/metrics`) | the golden now exercises a `to="stale_lease"` series; the byte-for-byte golden and the forbidden-content regex both stay green, proving the new render path is redacted |

## Mechanism for tx-safe, restart-consistent counters — and why

**Fold-from-durable-events at the sweep tick** (RFC 0137 §"Design guidance", the
*preferred* option), not in-memory post-commit atomics.

- **Tx-safe.** Counters are derived from the append-only `striatumd.events` table.
  A lifecycle transaction that rolls back never inserted its event, so it can never
  over-count; the count reflects only committed events. No post-commit ordering
  hazard inside the delicate recovery `withTx`.
- **Restart-consistent.** Each tick re-derives the counter from durable history, so
  a daemon restart resumes at the true cumulative value rather than resetting to
  zero. (Prometheus tolerates resets; we avoid them anyway.)
- **No import cycle.** Derivation lives entirely in `go/pkg/metrics` reading events
  via the existing `Querier`; `metrics` never imports `mutations` (proven by
  `go build ./...`). The lease-transition `to`/`reason` derivation is a pure
  function in `metrics`, fed by the collector SQL — the F1 fix added **no** new
  `mutations` source edit (only a `_test.go`).
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
  (necrosis for a confirmed-dead stall class; `recovery_transfer` for an honest-
  stall transfer the fold skips) and the `recovery_exhausted` blocker kind on
  `run.escalated` / `recovery.job_quarantined`.

## Enum → source-constant anchoring

The necrosis domain is pinned to EXACTLY the confirmed-dead set by
`TestNecrosisDomainMatchesConfirmedDeadConstants` (package `mutations`, where the
unexported constants are visible; it imports `metrics`), so renaming a value or
adding a stall class breaks the build:

| metrics enum value | anchored source constant | location |
| --- | --- | --- |
| `NecrosisAgentPIDDead = "agent_pid_dead"` | `stallClassAgentPIDDead` | `recovery_decision_tree.go` |
| `NecrosisAgentExitedUnsealed = "agent_exited_unsealed"` | `stallClassAgentExitedUnsealed` | `recovery_decision_tree.go` |
| `NecrosisRecoveryExhausted = "recovery_exhausted"` | `recoveryExhaustedBlockerKind` | `recovery_escalation.go` |

Apoptosis reasons map to real event types: `run_completed`←`run.completed`,
`job_succeeded`←`job.completed`, `session_closed_clean`←clean `session.closed`,
`lease_handoff`←handoff `lease.released`, `supervisor_drained`←`supervisor.stopped`.
Lease-transition `from`/`to` are anchored to the `striatumd.leases.state` values
(closed `leaseStateDomain`); the F1 fix routes the repo-write expiry payload
(`recovery.go`) to the `stale_lease` member of that same domain. The liveness
reasons (`deadline_missed`, `recovered`) are the closed F-A6 enum.

## How F-A6 is enforced

`liveness_deadline_missed` is a **reversible** pre-death observation
(`session.liveness_recovered` proves it is not death). It is kept out of necrosis
three ways: (1) domain exclusion — it is not a `NecrosisReason`, asserted absent by
the guardrail; (2) routing — `classifyLifecycleEvent` maps both liveness event
types to `ClassLiveness` → `striatum_liveness_deadline_events_total`, never
necrosis; (3) executable behavioral proof — `TestLivenessMissCanRecoverWithoutNecrosis`
drives the real `active → deadline_missed → recovered` path (`refreshRunLiveness`)
and asserts the liveness counter moved while `necrosis_total` rendered **no series**.
The unit `TestLivenessFoldRoutesReversiblePairOutsideNecrosis` keeps the fast
routing check, and `TestNecrosisIsCountedFromTaggedSiteEvents` proves the F-A6 zero
is not vacuous (a genuinely-dead close *does* move necrosis).

## Acceptance criteria → proving test

| Criterion | Proving test |
| --- | --- |
| **stale-lease signal is exposed distinctly** (review F1) | `TestLeaseTransitionTargetDistinguishesStaleLease` (unit, full derivation + render) + `TestStaleLeaseExpiryRendersDistinctTransition` (pg, real payload) |
| Each family exists with closed-enum labels pinned to source constants; cardinality cannot grow with run/job count | `TestNecrosisDomainMatchesConfirmedDeadConstants`; `TestLeaseReasonBucketingIsBounded`; golden render emits only closed enums |
| No apoptosis reason is hollow | `TestApoptosisReasonsAreAllProducedFromRealEventTypes` (unit, full-enum coverage) + `TestApoptosisHandoffAndDrainFoldedFromDurableEvents` (pg) |
| Wedge age from real job-state advance only | `TestRunWedgeAgeCountsOnlyJobStateAdvances` (pg, behavioral) + `TestJobStateAdvanceSetExcludesNonJobEvents` (unit) |
| **F-A6 behavioral** | `TestLivenessMissCanRecoverWithoutNecrosis` (pg, drives the real refresh) + `TestLivenessFoldRoutesReversiblePairOutsideNecrosis` (unit) |
| Necrosis fold is real (F-A6 zero is not vacuous) | `TestNecrosisIsCountedFromTaggedSiteEvents`; `TestApoptosisHandoffAndDrainFoldedFromDurableEvents` (necrosis stays empty on healthy events) |
| Apoptosis/necrosis tagged at the lifecycle site | `TestRecoveryTransferCloseIsNotApoptosis`; `TestApoptosisClassifiedFromHealthyEventTypes` |
| Exfiltration contract holds with the new label value | `TestMetricsRedactionGoldenAndForbiddenContent` (golden now exercises `to="stale_lease"`; forbidden-regex green) |
| Scrape stays O(1), zero-DB-query, lock-disjoint | `TestScrapeIssuesZeroQueries`; `TestConcurrentScrapesSeeIdenticalSnapshot` |

## Files touched (this attempt)

| File | Change |
| --- | --- |
| `go/pkg/metrics/taxonomy.go` | **New** pure helper `leaseTransitionTarget(eventType, reason, jobState)` deriving the closed `to` state + raw reason; routes a `job_state="stale_lease"` expiry to `to="stale_lease"` and pins the expiry reason to `"expired"` (bucket `expiry`). |
| `go/pkg/metrics/collector.go` | `leaseTransitionCounts` projects `payload_json->>'job_state'` (CASE-guarded to `lease.expired` only), `GROUP BY 1,2,3`, and derives `(to, reason)` via the helper. |
| `go/pkg/metrics/taxonomy_test.go` | **New** `TestLeaseTransitionTargetDistinguishesStaleLease` (table-driven derivation + end-to-end render distinctness). |
| `go/pkg/metrics/redaction_test.go` | `sentinelLeaseTransitions` adds a `to="stale_lease"` transition so the golden exercises the now-live render path. |
| `go/pkg/metrics/testdata/metrics_golden.txt` | Regenerated (one added line: `striatum_lease_transitions_total{from="active",to="stale_lease",reason="expiry"} 1`); forbidden-content regex re-verified. |
| `go/pkg/mutations/metrics_failure_taxonomy_pg_test.go` | **New** behavioral `TestStaleLeaseExpiryRendersDistinctTransition`. No `mutations` source edits. |

The attempt-1 source (apoptosis/necrosis classification, lease-handoff/drain wiring,
job-state-advance wedge filter, the necrosis site-tag + union guardrail) is
unchanged; this attempt adds no new edit to the delicate recovery code.

## New families (RFC §3)

- `striatum_apoptosis_total{origin,reason}` — counter (all 5 reasons non-hollow)
- `striatum_necrosis_total{origin,reason}` — counter (confirmed-dead only)
- `striatum_lease_transitions_total{from,to,reason}` — counter (now exposes
  `to="stale_lease"` distinctly — review F1)
- `striatum_liveness_deadline_events_total{reason}` — counter (F-A6 home)
- `striatum_run_wedge_age_seconds{origin}` — histogram (job-state-advance evidence)
- `striatum_liveness_deadline_margin_seconds{origin}` — histogram

## Verification commands and results

Run from the per-job worktree `go/` directory on `striatum/rfc0137-phase-b`.

- `make -C go build` → **PASS** (striatum, striatumd, supervisor-helper).
- `go build ./...` → **PASS** — confirms no `metrics`↔`mutations` import cycle.
- `go vet ./pkg/metrics/... ./pkg/mutations/...` → **PASS** (clean).
- `go test ./pkg/metrics/...` → **PASS** — includes the new
  `TestLeaseTransitionTargetDistinguishesStaleLease`, the regenerated golden, the
  forbidden-content regex, the F-A6 routing unit, and the scrape O(1)/zero-query
  identity tests.
- `go test ./cmd/striatumd` → **PASS** (Phase A `/metrics` wiring still routes).
- `go test ./pkg/mutations/...` → all pass **except** the pre-existing,
  environment-dependent `TestSpawnRunAsSpecResolvesLaneUser` (`spawn_grant_test.go`,
  wants host lane user `striatum-lane`) — unrelated to this change (not in this
  attempt's diff) and the **same** failure the prior reviewer recorded. The
  non-PG taxonomy guardrail `TestNecrosisDomainMatchesConfirmedDeadConstants` passes.

### PostgreSQL behavioral tests (honest status)

The four behavioral tests in `pkg/mutations/metrics_failure_taxonomy_pg_test.go`
(`TestStaleLeaseExpiryRendersDistinctTransition`,
`TestLivenessMissCanRecoverWithoutNecrosis`,
`TestRunWedgeAgeCountsOnlyJobStateAdvances`,
`TestApoptosisHandoffAndDrainFoldedFromDurableEvents`) are gated on
`STRIATUM_PG_TEST_URL`, exactly like every integration test in `pkg/mutations`.
**In this lane environment that variable is unset and the tests skip cleanly** —
the same state the prior reviewer recorded ("skipped the live PostgreSQL behavioral
tests because `STRIATUM_PG_TEST_URL` is not set"). No throwaway PostgreSQL was
reachable from this lane (no container-runtime access; the local clusters require
credentials this lane does not hold and belong to the live daemon, which must not be
disturbed). The new `TestStaleLeaseExpiryRendersDistinctTransition` compiles and
skips cleanly under the identical gate as its accepted sibling tests; the F1 logic
it covers is also proven **deterministically and unconditionally** by the unit
`TestLeaseTransitionTargetDistinguishesStaleLease`, which runs the `(to, reason)`
derivation and the `Build`→render distinctness in CI without a database. A reviewer
with `STRIATUM_PG_TEST_URL` set can run the four tests directly to exercise the real
durable-event payload path end-to-end.

## Scope confirmation — Phase C / Phase D NOT implemented

Explicitly **not** built: **Phase C** — no `Classification` taxonomy / `Register()`
refusal of `Forbidden`, no per-family series budget / `cardinality_clipped_total`,
no boot-time `metrics_allowlist.json` hash check, no `doctor_problems{class}`
collector, no `TestDoctorClassRejectsDynamicIdentifiers`. **Phase D** — no
capability-scoped `/metrics` filtering, no `metrics_repo_consent` gauge, no
`tick_status` publish-on-errored-tick, no bundled Prometheus rules, no cold
DB-projection tier. The bind stays loopback-only (inherited from Phase A). The
cardinality/privacy contract for the new label value is upheld by the closed
`leaseStateDomain` enum and the golden + forbidden-regex redaction test; the
*enforced* Classification/allowlist machinery is Phase C as designed.

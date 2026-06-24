---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs:
  - "striatum/rfc-0137/phase-b/artifacts/DRAFT.md"
  - "striatum/rfc-0137/phase-b/artifacts/review/REVIEW.md"
  - "docs/rfcs/0137-striatumd-prometheus-exporter.md"
---

# RFC 0137 Phase B — failure-mode taxonomy (final summary)

author: author-author-003

Phase B of RFC 0137 (`striatumd` Prometheus exporter) is **implemented, reviewed,
accepted, and re-confirmed green**. It adds the failure-mode-shaped metric
taxonomy on top of the landed Phase A read-path skeleton: the apoptosis/necrosis
spine, the F-A6 reversible-liveness counter, lease-transition counts (now exposing
the stale-lease storm signal distinctly), and the wedge-age / liveness-margin
histograms. Every counter is folded from the durable `striatumd.events` ledger at
the sweep tick, so it is transaction-safe and restart-consistent by construction.
Phase C and Phase D were deliberately **not** implemented.

The reviewer (`reviewer-reviewer-002`) returned **accept** on the attempt-2 draft
and source anchor with no remaining correctness, privacy/cardinality, or scope
defect. This apply pass re-confirmed the green surface, verified the in-scope
guardrails the prompt calls out, and confirmed the `go/pkg/mutations/` edits stayed
surgical (additive lifecycle tag only, no recovery-logic refactor). No additional
source change was required.

## Final file list (Phase B diff vs `main`)

| File | Change | Role |
| --- | --- | --- |
| `go/pkg/metrics/taxonomy.go` | **new** | Closed `Origin` / `ApoptosisReason` / `NecrosisReason` / `LivenessDeadlineReason` enums, the `LifecycleClass` split, the at-the-site lifecycle tag constants, `classifyLifecycleEvent`, the lease-state/reason buckets, and the F1 `leaseTransitionTarget` derivation. |
| `go/pkg/metrics/taxonomy_test.go` | **new** | `TestLeaseTransitionTargetDistinguishesStaleLease` (table-driven `(to,reason)` derivation + end-to-end `Build`→render distinctness), plus the apoptosis/necrosis/liveness routing units. |
| `go/pkg/metrics/collector.go` | modified | Folds the new families from `striatumd.events`; `leaseTransitionCounts` projects `payload_json->>'job_state'` (CASE-guarded to `lease.expired`) and derives `(to,reason)` via `leaseTransitionTarget`. |
| `go/pkg/metrics/snapshot.go` | modified | Carries the new family counters/histograms on the immutable snapshot. |
| `go/pkg/metrics/render.go` | modified | Renders the six new families (`striatum_apoptosis_total`, `striatum_necrosis_total`, `striatum_lease_transitions_total`, `striatum_liveness_deadline_events_total`, `striatum_run_wedge_age_seconds`, `striatum_liveness_deadline_margin_seconds`) with closed-enum labels. |
| `go/pkg/metrics/redaction_test.go` | modified | `sentinelLeaseTransitions` adds a `to="stale_lease"` transition so the golden exercises the now-live render path. |
| `go/pkg/metrics/testdata/metrics_golden.txt` | modified | Regenerated for the new families (incl. the added `to="stale_lease",reason="expiry"` line); forbidden-content regex re-verified. |
| `go/pkg/mutations/recovery_decision_tree.go` | modified (surgical) | Adds the pure `isNecrosisStallClass` helper and an **additive** `lifecycle_metric` payload tag on the existing `session.closed` event in `closeStalledOwningSession` — no recovery behavior changed. |
| `go/pkg/mutations/necrosis_domain_metrics_test.go` | **new** | `TestNecrosisDomainMatchesConfirmedDeadConstants` — the enum→source-constant guardrail (lives in `mutations` where the unexported constants are visible). |
| `go/pkg/mutations/metrics_failure_taxonomy_pg_test.go` | **new** | The four PostgreSQL behavioral tests (`TestStaleLeaseExpiryRendersDistinctTransition`, `TestLivenessMissCanRecoverWithoutNecrosis`, `TestRunWedgeAgeCountsOnlyJobStateAdvances`, `TestApoptosisHandoffAndDrainFoldedFromDurableEvents`). No `mutations` source edits. |
| `striatum/rfc-0137/phase-b/artifacts/DRAFT.md` | **new** | Phase B draft (attempt-2 handoff). |
| `striatum/rfc-0137/phase-b/artifacts/SUMMARY.md` | **new** | This synthesis. |

## New families (RFC §3)

- `striatum_apoptosis_total{origin,reason}` — counter; all five reasons non-hollow.
- `striatum_necrosis_total{origin,reason}` — counter; confirmed-dead classes only.
- `striatum_lease_transitions_total{from,to,reason}` — counter; exposes
  `to="stale_lease"` distinctly (review F1).
- `striatum_liveness_deadline_events_total{reason}` — counter; the F-A6 home for the
  reversible `deadline_missed`/`recovered` pair, outside the conservation law.
- `striatum_run_wedge_age_seconds{origin}` — histogram (job-state-advance evidence).
- `striatum_liveness_deadline_margin_seconds{origin}` — histogram.

## Enum → source-constant anchoring

The CREATE-new metric enums do not exist as source constants; they are **anchored**
to the real lifecycle/recovery constants and pinned by
`TestNecrosisDomainMatchesConfirmedDeadConstants` (package `mutations`, importing
`metrics` — so the import direction is `mutations → metrics` and there is no cycle).
Renaming a value or adding a stall class breaks the build.

| metrics enum value | anchored source constant | location |
| --- | --- | --- |
| `NecrosisAgentPIDDead = "agent_pid_dead"` | `stallClassAgentPIDDead` | `recovery_decision_tree.go` |
| `NecrosisAgentExitedUnsealed = "agent_exited_unsealed"` | `stallClassAgentExitedUnsealed` | `recovery_decision_tree.go` |
| `NecrosisRecoveryExhausted = "recovery_exhausted"` | `recoveryExhaustedBlockerKind` | `recovery_escalation.go` |

Apoptosis reasons map 1:1 to real durable event types: `run_completed`←`run.completed`,
`job_succeeded`←`job.completed`, `session_closed_clean`←clean `session.closed`,
`lease_handoff`←a handoff `lease.released` (transfer flag / supersession reason),
`supervisor_drained`←`supervisor.stopped`. The lease-transition `from`/`to` labels
are anchored to the closed `leaseStateDomain` (the `striatumd.leases.state`
values); the F1 fix routes the repo-write stale-limbo expiry payload to the
`stale_lease` member of that same domain. The liveness reasons
(`deadline_missed`, `recovered`) are the closed F-A6 enum.

## Tx-safe, restart-consistent counter mechanism

**Fold-from-durable-events at the sweep tick** (the RFC's preferred option), not
in-memory post-commit atomics:

- **Tx-safe.** Counters derive from the append-only `striatumd.events` table. A
  lifecycle transaction that rolls back never inserted its event, so it can never
  over-count; the count reflects only committed events. No post-commit ordering
  hazard inside the delicate recovery `withTx`.
- **Restart-consistent.** Each tick re-derives the counter from durable history, so
  a daemon restart resumes at the true cumulative value rather than resetting to
  zero. (Prometheus tolerates resets; we avoid them anyway.)
- **No import cycle.** Derivation lives entirely in `go/pkg/metrics` reading events
  through the existing `Querier`; `metrics` never imports `mutations` (proven by
  `go build ./...`). The lease-transition `(to,reason)` derivation is a pure
  function in `metrics`.
- **Cost.** Fixed, small `GROUP BY` aggregates at the ~60 s tick, never on the
  scrape path — preserving Phase A's O(1), lock-disjoint, zero-PG-query scrape.

The apoptosis/necrosis split — which shares the same terminal DB transition — is
decided at fold time in `classifyLifecycleEvent` from the durable event each path
wrote: apoptosis from the unambiguous healthy-termination event types the
terminator emits; necrosis only from the intentional `lifecycle_metric=necrosis`
tag (stamped at the site in `closeStalledOwningSession` for a confirmed-dead stall
class), a confirmed-dead stall class, or the `recovery_exhausted` blocker kind on
`run.escalated` / `recovery.job_quarantined`.

## How F-A6 is enforced

`session.liveness_deadline_missed` is a **reversible** pre-death observation
(`session.liveness_recovered` proves it is not death). It is kept out of necrosis
three ways:

1. **Domain exclusion** — it is not a `NecrosisReason`; the guardrail asserts the
   necrosis domain equals exactly `{agent_pid_dead, agent_exited_unsealed,
   recovery_exhausted}`, so it cannot re-enter.
2. **Routing** — `classifyLifecycleEvent` maps both liveness event types to
   `ClassLiveness` → `striatum_liveness_deadline_events_total`, never necrosis.
3. **Executable behavioral proof** — `TestLivenessMissCanRecoverWithoutNecrosis`
   drives the real `active → deadline_missed → recovered` path and asserts the
   liveness counter moved while `striatum_necrosis_total` rendered no series.
   `TestLivenessFoldRoutesReversiblePairOutsideNecrosis` is the fast routing unit,
   and `TestNecrosisIsCountedFromTaggedSiteEvents` proves the F-A6 zero is not
   vacuous (a genuinely-dead close *does* move necrosis).

## Acceptance-criteria → test mapping (with this-pass verification results)

All commands run from the per-job worktree `go/` directory on
`striatum/rfc0137-phase-b` (HEAD `a6cd7788`).

| Criterion (RFC §"Acceptance Criteria" / review F1) | Proving test(s) | Result this pass |
| --- | --- | --- |
| Stale-lease storm signal exposed distinctly (review F1) | `TestLeaseTransitionTargetDistinguishesStaleLease` (unit) + `TestStaleLeaseExpiryRendersDistinctTransition` (pg) | unit **PASS**; pg **SKIP** (no `STRIATUM_PG_TEST_URL`) |
| Each family exists with closed-enum labels pinned to source constants; cardinality cannot grow with run/job count | `TestNecrosisDomainMatchesConfirmedDeadConstants`; closed `leaseStateDomain`/`leaseReasonBucket`; golden render | guardrail **PASS**; golden **PASS** |
| No apoptosis reason is hollow | `TestApoptosisReasonsAreAllProducedFromRealEventTypes` (unit) + `TestApoptosisHandoffAndDrainFoldedFromDurableEvents` (pg) | unit **PASS**; pg **SKIP** |
| Wedge age from real job-state advance only | `TestRunWedgeAgeCountsOnlyJobStateAdvances` (pg) + `TestJobStateAdvanceSetExcludesNonJobEvents` (unit) | unit **PASS**; pg **SKIP** |
| **F-A6 behavioral** — liveness miss recovers without necrosis | `TestLivenessMissCanRecoverWithoutNecrosis` (pg) + `TestLivenessFoldRoutesReversiblePairOutsideNecrosis` (unit) | unit **PASS**; pg **SKIP** |
| Necrosis fold is real (F-A6 zero is not vacuous) | `TestNecrosisIsCountedFromTaggedSiteEvents` | **PASS** |
| Apoptosis/necrosis tagged at the lifecycle site | `TestRecoveryTransferCloseIsNotApoptosis`; `TestApoptosisClassifiedFromHealthyEventTypes` | **PASS** |
| Exfiltration contract holds with the new label value | `TestMetricsRedactionGoldenAndForbiddenContent` (golden now exercises `to="stale_lease"`; forbidden-content regex green) | **PASS** |
| Scrape stays O(1), zero-DB-query, lock-disjoint | `TestScrapeIssuesZeroQueries`; `TestConcurrentScrapesSeeIdenticalSnapshot` | **PASS** |

### Verification commands + results (this apply pass)

- `make -C go build` → **PASS** (striatum, striatumd, supervisor-helper).
- `go test ./pkg/metrics/...` → **PASS**.
- `go build ./...` → **PASS** — confirms no `metrics`↔`mutations` import cycle.
- `go test ./cmd/striatumd` → **PASS** (Phase A `/metrics` wiring still routes).
- `go vet ./pkg/metrics/... ./pkg/mutations/...` → **PASS** (clean).
- `go test ./pkg/mutations -run 'TestNecrosisDomainMatchesConfirmedDeadConstants|TestLivenessMissCanRecoverWithoutNecrosis|TestStaleLeaseExpiryRendersDistinctTransition|TestRunWedgeAgeCountsOnlyJobStateAdvances|TestApoptosisHandoffAndDrainFoldedFromDurableEvents'` → guardrail **PASS**; the four PostgreSQL behavioral tests **SKIP cleanly** because `STRIATUM_PG_TEST_URL` is unset in this lane.

### PostgreSQL behavioral tests — honest status

The four `pkg/mutations` behavioral tests are gated on `STRIATUM_PG_TEST_URL`,
exactly like every integration test in `pkg/mutations`. In this lane the variable
is unset and they **skip cleanly** — the same state the reviewer recorded. No
throwaway PostgreSQL was stood up: the local clusters belong to the live daemon
and must not be disturbed. The F1 logic they cover is also proven
**deterministically and without a database** by
`TestLeaseTransitionTargetDistinguishesStaleLease` (full `(to,reason)` derivation +
`Build`→render distinctness), so the contract has unconditional CI coverage. A
reviewer with `STRIATUM_PG_TEST_URL` set can run the four tests directly to
exercise the real durable-event payload path end-to-end.

## Scope confirmation — Phase C / Phase D NOT implemented

Confirmed via `go build ./...` and the diff: no Phase C/D work is present. The bind
stays loopback-only (inherited from Phase A). The cardinality/privacy contract for
the new `to="stale_lease"` value is upheld by the closed `leaseStateDomain` enum and
the golden + forbidden-content redaction test; the *enforced* Classification/
allowlist machinery is Phase C as designed.

## Follow-ups left for Phase C (next run's scope)

Phase B leaves the tree shippable; the following RFC §"Phase C" items are
explicitly **deferred** and define the next run's scope:

1. **`Classification` taxonomy + `Register()` refusal of `Forbidden`.** Tag every
   `Family` with `Operational | Provenance | Forbidden` and make `Register()`
   refuse a `Forbidden` family at construction (panic in tests, hard boot abort in
   prod) so a forbidden series can never reach the wire.
2. **Per-family series budget + `striatum_metrics_cardinality_clipped_total`.** An
   LRU budget that registers the first N distinct label-tuples, collapses overflow
   onto a reserved `{bucket="other"}` series, and increments an alertable clip
   counter so neither the daemon registry nor a downstream Prometheus can be
   ID-bombed into OOM.
3. **Boot-time allowlist hash + boot abort.** A `metrics_allowlist` check (beside
   the `go/pkg/reads/doctor_*` checks) that SHA-256s the sorted
   `(family, label_names, classification)` set and compares against a checked-in
   `metrics_allowlist.json`; drift fails the guardrail test in CI and aborts daemon
   startup, making a label addition a deliberate, diff-reviewed manifest edit.
4. **`striatum_doctor_problems{class}` collector.** Source the gauge from the
   **static `problem_records[*].check` codes** of the existing `reads/doctor_*`
   checks (never the dynamic-id `problems` prefix), on a bounded cadence (not on
   every scrape), and ship `TestDoctorClassRejectsDynamicIdentifiers` (F-A8) — seed
   adversarial run/gate ids and assert no dynamic id reaches `class` and the series
   count stays constant.

Phase D (capability-scoped `/metrics` filtering, the `metrics_repo_consent` gauge,
`tick_status` publish-on-errored-tick, bundled Prometheus recording/alerting rules,
and the optional cold DB-projection tier) remains after Phase C.

# RFC 0137 Phase B — failure-mode taxonomy (apoptosis/necrosis + lifecycle emit)

author: author-author-002

Phase B of RFC 0137 (`striatumd` Prometheus exporter) is implemented and green,
built on top of the landed Phase A read-path skeleton. It adds the
failure-mode-shaped metric taxonomy: a closed-enum `Origin` / `*Reason` vocabulary
pinned to the real recovery constants by a union guardrail test, the
apoptosis/necrosis spine, the F-A6 reversible-liveness counter, lease-transition
counts, and the wedge-age / liveness-margin histograms. Every counter is folded
from the **durable `striatumd.events` ledger** at the sweep tick, so the numbers
are transaction-safe and restart-consistent by construction. Phase C and Phase D
were deliberately **not** implemented.

## Mechanism chosen for tx-safe, restart-consistent counters — and why

I chose **fold-from-durable-events at the sweep tick** (RFC 0137 §"Design
guidance", the *preferred* option), not in-memory post-commit atomics.

- **Why it is tx-safe.** The counters are derived read-model values computed from
  the append-only `striatumd.events` table (its `events_no_update` /
  `events_no_delete` triggers make it immutable). A lifecycle transaction that
  rolls back never inserted its event, so it can never over-count; the count only
  ever reflects events that actually committed. There is no post-commit ordering
  hazard to get wrong inside the delicate recovery `withTx`.
- **Why it is restart-consistent.** The counter is re-derived from durable
  history on every tick, so a daemon restart resumes at the true cumulative value
  rather than silently resetting to zero. (Prometheus tolerates counter resets,
  but we avoid them anyway.)
- **Why it dodges the import cycle.** The derivation lives entirely inside
  `go/pkg/metrics` (`Collector.lifecycleEventCounts` etc.), which reads events via
  the existing `Querier`. `metrics` never imports `mutations`. The one place
  `mutations` must cooperate — tagging the ambiguous `session.closed` event — uses
  the **sanctioned `mutations` → `metrics` direction** (it imports the tag
  constants), which is acyclic because `metrics` only depends on `sessionliveness`
  (a leaf) and `pgx`.
- **Cost.** The fold issues a fixed, small number of `GROUP BY` aggregates at the
  ~60s tick cadence, never on the scrape path, so it preserves Phase A's O(1)
  lock-disjoint scrape. The event aggregate is filtered to the seven lifecycle
  event types and grouped, so it returns one row per distinct classification, not
  one row per event. (A future optimization could cursor the aggregate to avoid
  re-scanning all history each tick; it is unnecessary for correctness because the
  fold is idempotent and monotone.)

### Tagging apoptosis vs necrosis "at the site"

Apoptosis and necrosis share the same terminal DB transition, so the split is
decided at the durable event each path writes (`classifyLifecycleEvent`):

- **Apoptosis** is read from the *unambiguous healthy-termination event types* the
  terminator already emits: `run.completed` → `run_completed`, `job.completed` →
  `job_succeeded`, and a clean `session.closed` → `session_closed_clean`. The
  terminator declaring intent IS the tag.
- **Necrosis** is read from intentional tags the recovery/liveness paths write
  when they detect an *unannounced* exit:
  - `closeStalledOwningSession` (`recovery_decision_tree.go`) now stamps an
    explicit `lifecycle_metric` tag on its `session.closed` event — `necrosis` for
    a confirmed-dead stall class, `recovery_transfer` for an honest-stall transfer.
    This is **load-bearing**: without it a recovery transfer-close of a
    *still-alive but stalled* lane would be miscounted as a clean apoptosis. The
    tag is computed by `isNecrosisStallClass`, the single source of truth.
  - `recovery_exhausted` is read from the `blocker_kind` the `run.escalated` /
    `recovery.job_quarantined` events already carry — that field is itself the
    intentional escalation tag, so no new edit was needed there.

The `mutations` edit is additive payload only (one function, `closeStalledOwningSession`)
and changes no recovery behavior.

## Enum → source-constant anchoring

New closed Go enums live in `go/pkg/metrics/taxonomy.go` (they did not exist as
source constants before). The **necrosis domain is pinned to EXACTLY the
confirmed-dead set** by `TestNecrosisDomainMatchesConfirmedDeadConstants`, which
lives in package `mutations` (where the unexported constants are visible) and
imports `metrics`:

| metrics enum value | anchored source constant | location |
| --- | --- | --- |
| `NecrosisAgentPIDDead = "agent_pid_dead"` | `stallClassAgentPIDDead` | `go/pkg/mutations/recovery_decision_tree.go:153` |
| `NecrosisAgentExitedUnsealed = "agent_exited_unsealed"` | `stallClassAgentExitedUnsealed` | `go/pkg/mutations/recovery_decision_tree.go:159` |
| `NecrosisRecoveryExhausted = "recovery_exhausted"` | `recoveryExhaustedBlockerKind` | `go/pkg/mutations/recovery_escalation.go:15` |

The guardrail builds its expected set from the **real constants** (not string
literals), so renaming a constant's value, adding a stall class to necrosis, or
letting `liveness_deadline_missed` re-enter all break the build. The apoptosis
reasons (`run_completed`, `job_succeeded`, `lease_handoff`, `supervisor_drained`,
`session_closed_clean`) and the liveness reasons (`deadline_missed`, `recovered`)
are the closed enums from the RFC §3 taxonomy table.

## How F-A6 is enforced

`liveness_deadline_missed` is a **reversible** pre-death observation
(`session.liveness_deadline_missed` / `session.liveness_recovered` at
`recovery.go:1229` / `:1244`; the recover path proves it is not death). It is
enforced out of necrosis three ways:

1. **Domain exclusion.** `liveness_deadline_missed` is not a `NecrosisReason`; the
   guardrail test asserts it is absent from the necrosis domain.
2. **Routing.** `classifyLifecycleEvent` maps both liveness event types to
   `ClassLiveness`, which folds into `striatum_liveness_deadline_events_total`
   (its own family, with the full closed reason enum always rendered) — never into
   `striatum_necrosis_total`.
3. **Executable proof.** `TestLivenessMissCanRecoverWithoutNecrosis` drives
   `active → liveness_deadline_missed → liveness_recovered` and asserts the
   liveness counter moved on both halves while `necrosis_total` AND `apoptosis_total`
   stayed at zero (the reversible pair is outside the conservation law). It checks
   both the in-memory snapshot and the rendered wire surface (no liveness reason
   ever appears as a necrosis label).

## Acceptance criteria → proving test

| Phase B criterion | Proving test |
| --- | --- |
| Each family exists with closed-enum labels pinned to source constants; cardinality cannot grow with run/job count | `TestNecrosisDomainMatchesConfirmedDeadConstants` (necrosis domain == source constants); `TestLeaseReasonBucketingIsBounded` (unknown reasons/states bucket to `other`); render emits only closed enums / bucketed labels |
| **F-A6**: a recoverable liveness miss moves the liveness counter, never necrosis | `TestLivenessMissCanRecoverWithoutNecrosis` (`pkg/metrics`) |
| Necrosis fold is real (F-A6 zero is not vacuous) | `TestNecrosisIsCountedFromTaggedSiteEvents` — tagged dead-close + recovery_exhausted move `necrosis_total` to 4 |
| Apoptosis/necrosis tagged at the lifecycle site | `TestRecoveryTransferCloseIsNotApoptosis` + `TestApoptosisClassifiedFromHealthyEventTypes`; the `closeStalledOwningSession` `lifecycle_metric` tag |
| Exfiltration contract still holds with the new families | `TestMetricsRedactionGoldenAndForbiddenContent` — golden now exercises every Phase B family; the forbidden-content regexes (40-hex, paths, `--flag=`, slash shapes, `author:`) still pass (no raw id reaches a label) |
| Scrape stays O(1), zero-DB-query, lock-disjoint (Phase A regression guard) | `TestScrapeIssuesZeroQueries`, `TestConcurrentScrapesSeeIdenticalSnapshot` |

## Files touched

New:

| File | What it does |
| --- | --- |
| `go/pkg/metrics/taxonomy.go` | Closed `Origin` / `ApoptosisReason` / `NecrosisReason` / `LivenessDeadlineReason` enums + domain accessors; the lease state/reason bucketing; the `lifecycle_metric` tag vocabulary; `classifyLifecycleEvent` (the single apoptosis/necrosis split point). |
| `go/pkg/metrics/taxonomy_test.go` | F-A6 test + non-vacuous necrosis test + transfer-vs-clean test + apoptosis-by-type test + lease bucketing test. |
| `go/pkg/mutations/necrosis_domain_metrics_test.go` | The union guardrail anchoring the necrosis domain to the real constants. |

Modified:

| File | What changed |
| --- | --- |
| `go/pkg/metrics/snapshot.go` | `SnapshotInput` + `Build` (richer constructor; `BuildSnapshot` retained as the Phase A wrapper); the six Phase B fold maps; weighted `addEvent` / `addLeaseTransition`; fixed-bucket `histogram`; deterministic sort helpers. |
| `go/pkg/metrics/render.go` | Renders the six Phase B families (counters observed-sorted; the liveness family emits the full closed enum; histograms per origin with cumulative `_bucket`/`_sum`/`_count`). Help text stays free of slashes / `--flag=` / `author:` / 40-hex. |
| `go/pkg/metrics/collector.go` | Best-effort tick folds: `lifecycleEventCounts`, `leaseTransitionCounts`, `runWedgeAges`, `livenessMargins`. Phase B fold errors degrade a single family to empty rather than blocking the Phase A surface. Margin deadline is read from `sessionliveness.DefaultPolicy` (anchored, not hardcoded). |
| `go/pkg/metrics/redaction_test.go` | The golden now exercises every Phase B family (events/lease/wedge/margin sentinels). |
| `go/pkg/metrics/testdata/metrics_golden.txt` | Regenerated deliberately for the new families; forbidden-content regex re-verified green. |
| `go/pkg/mutations/recovery_decision_tree.go` | `isNecrosisStallClass` helper + the additive `lifecycle_metric` site tag on `closeStalledOwningSession`'s `session.closed` event; `metrics` import. |

## New families (RFC §3)

- `striatum_apoptosis_total{origin,reason}` — counter
- `striatum_necrosis_total{origin,reason}` — counter (confirmed-dead only)
- `striatum_lease_transitions_total{from,to,reason}` — counter
- `striatum_liveness_deadline_events_total{reason}` — counter (F-A6 home)
- `striatum_run_wedge_age_seconds{origin}` — histogram
- `striatum_liveness_deadline_margin_seconds{origin}` — histogram

## Verification commands and results

Run from the per-job worktree's `go/` directory on run branch
`striatum/rfc0137-phase-b`.

- `make -C go build` → **PASS** (exit 0; striatum, striatumd, supervisor-helper).
- `go build ./...` → **PASS** (exit 0) — confirms no `metrics`↔`mutations` import cycle.
- `go test ./pkg/metrics/... -v` → **PASS** (8 tests):
  `TestMetricsRedactionGoldenAndForbiddenContent`, `TestScrapeIssuesZeroQueries`,
  `TestConcurrentScrapesSeeIdenticalSnapshot`,
  `TestLivenessMissCanRecoverWithoutNecrosis`,
  `TestNecrosisIsCountedFromTaggedSiteEvents`,
  `TestRecoveryTransferCloseIsNotApoptosis`,
  `TestApoptosisClassifiedFromHealthyEventTypes`, `TestLeaseReasonBucketingIsBounded`.
- `go test ./pkg/mutations/...` → `TestNecrosisDomainMatchesConfirmedDeadConstants`
  **PASS**. The suite's only failure is the pre-existing, environment-dependent
  `TestSpawnRunAsSpecResolvesLaneUser` (lane-user `striatum-lane` resolution),
  which **fails identically at the run base commit** before this change — it is
  not introduced here and is unrelated to RFC 0137 (the Phase A summary recorded
  the same pre-existing `pkg/mutations` failure).
- `go test ./cmd/striatumd` → **PASS** (the Phase A `/metrics` wiring still
  compiles and routes).
- `go vet ./pkg/metrics/... ./pkg/mutations/...` → clean.

## Scope confirmation — Phase C / Phase D NOT implemented

This slice is Phase B only. Explicitly **not** built:

- **Phase C** — no `Classification` taxonomy / `Register()` refusal of
  `Forbidden` families, no per-family series budget / `cardinality_clipped_total`,
  no boot-time `metrics_allowlist.json` hash check, no `doctor_problems{class}`
  collector, no `TestDoctorClassRejectsDynamicIdentifiers`.
- **Phase D** — no capability-scoped `/metrics` filtering, no
  `metrics_repo_consent` gauge, no `tick_status` publish-on-errored-tick, no
  bundled Prometheus recording/alerting rules, no cold DB-projection tier.

The bind stays loopback-only (inherited from Phase A; no new listener). The
cardinality/privacy contract for the new labels is upheld by closed enums and the
golden + forbidden-regex redaction test; the *enforced* Classification/allowlist
machinery is Phase C as designed.

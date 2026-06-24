---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - striatum/rfc-0137/phase-d/artifacts/DRAFT.md
  - striatum/rfc-0137/phase-d/artifacts/review/REVIEW.md
  - striatum/rfc-0137/phase-c/artifacts/SUMMARY.md
  - docs/rfcs/0137-striatumd-prometheus-exporter.md
---

# RFC 0137 Phase D — SUMMARY (completes A→D)

author: author-author-005

Phase D is the **final** phase of RFC 0137. The review verdict was **accept**
with **no blocking findings and no nits** (`reviewer-reviewer-004`,
`striatum/rfc-0137/phase-d/artifacts/review/REVIEW.md`). This apply step
re-confirmed green and ships the deliverable summary; no source change was
required to satisfy the review. **RFC 0137 A→D is now fully implemented** — read
path (A) + failure-mode taxonomy (B) + enforced metrics contract (C) +
multi-tenant hardening, consent, staleness SLI, and semantics-grounded alert
rules (D).

## Final file list

New (`go/pkg/metrics/`):
- `surrogate.go` — the salted per-repo surrogate
  (`bucket = HMAC-SHA256(daemon-secret, repo_id) mod K`, K=256). The collision is
  intentionally lossy and is merged **only** in the unscoped/loopback aggregate
  view; the scoped path filters by real `repository_id` first.
- `consent.go` — the `TickStatus` enum, the OQ2 lifecycle-balance blind-spot
  helper, and the publish-on-errored-tick constructor.
- `semantics.go` — per-family rate semantics (`monotonic` vs `snapshot`), the
  ground truth the rules guardrail validates against (DEFECT 2).
- `rules.go` + `rules/recording_rules.yml` + `rules/alerting_rules.yml` —
  version-controlled Prometheus rules, embedded into the binary.
- `phase_d_test.go`, `rules_test.go` — Phase D unit + guardrail tests.

Modified (`go/pkg/metrics/`):
- `registry.go` — register the four new families (`metrics_tick_status`,
  `lifecycle_balance`, `metrics_repo_consent` Operational; `repo_runs`
  Provenance).
- `snapshot.go` — `RepoMetric` input; per-repo observations **retained keyed by
  real `repository_id`** (`repoSeries`/`repoSeriesEntry`) with the consent gate at
  fold time; render-time `aggregateRepoConsent`/`aggregateRepoRuns` (bucket-merge
  **after** the repo_id filter); `RepoIDs()`/`TickStatus()` accessors (replacing
  the leaky `RepoBuckets()`); lifecycle-balance accounting in `addEvent`.
- `render.go` — `WriteText` → `WriteTextScoped(w, now, allowedRepos)` filtering the
  per-repo families by **authorized `repository_id`**; render funcs for the four
  new families; exported `ScrapeContentType()`.
- `collector.go` — `NewCollectorWithSurrogate`; per-repo consent
  (`repoConsentFlags`) + run-state (`repoRunStateCounts`) folds; `Refresh` tracks
  `ok|partial|error` and publishes a carried-forward snapshot on an errored tick.
- `metrics_allowlist.json` + `testdata/metrics_golden.txt` — regenerated for the
  four new families (reviewed diffs; enum/small-integer labels only).
- `registry_test.go`, `redaction_test.go` — updated for the first Provenance
  family + per-repo sentinels.

New / modified (`go/cmd/striatumd/`):
- `metrics_scope.go` (new) — the capability-scoped `/metrics` wrapper; filters by
  real `repository_id` (`authorizedRepos`).
- `metrics_scope_test.go` (new) — loopback-full / remote-scoped / **colliding-repo
  isolation** / fail-closed tests.
- `main.go` — build the surrogate from the RFC 0110 authority secret, wire it into
  the collector, and wrap the exporter handler with `newScopedMetricsHandler`.

`git diff --stat main...HEAD` over the branch: 20 files changed, ~2300
insertions. No SQL migration file was added (see Persistence below).

## Multi-tenant / consent / alerting surface — and how it reuses RFC 0043

**Capability-scoped `/metrics` (Deliverable 1, reuses RFC 0043).** Loopback stays
default-open (Phase A). Beyond loopback, `newScopedMetricsHandler`
(`go/cmd/striatumd/metrics_scope.go`):

1. Classifies the peer by **`RemoteAddr`** (not the attacker-controllable `Host`
   header), failing closed on an empty/unparseable address.
2. Requires `Authorization: Bearer <token>` (401 if absent).
3. Resolves the authorized repositories by asking the **same `rpc.Authorizer`
   that gates RPC** — `authorizer.Authorize(&rpc.CapabilityRead, repoID, token)` —
   for each repository the published snapshot folded (`Snapshot.RepoIDs()`). **No
   parallel ACL is invented**: a daemon-global read grant authorizes every repo, a
   repo-scoped token only its own. A token authorizing none of the served repos is
   **403** (fail closed).
4. Renders `WriteTextScoped(w, now, allowedRepos)`: repo-aggregate **Operational**
   families always emit; the two families carrying the salted `bucket`
   (`metrics_repo_consent`, `repo_runs`) are aggregated from the retained per-repo
   entries **over only the authorized `repository_id`s, before** the bucket label is
   applied.

The load-bearing correctness property (the prior review's DEFECT 1): the salted
bucket is **lossy**, so two repos can collide into one bucket. Because the scope
filter runs on the **real `repository_id`** and the bucket is applied **only
afterwards**, a repo-A token sees repo-A's series **alone** even when a colliding
repo-B shares its bucket. The scoped path does only RFC 0043 auth lookups (one per
served repo) and **never re-folds** metrics; the loopback path stays strictly
zero-query.

**Consent (Deliverable 2).** `striatum_metrics_repo_consent{bucket}` (Operational)
is emitted for **every** active repo (0 or 1), so the *absence* of provenance is
itself scrapeable. `striatum_repo_runs{bucket,state}` (the first **Provenance**
family) is folded **only for consented repos** (gate at fold time) and filtered
again by capability scope at render time — both gates compose. Operational
families default ON; Provenance defaults **OFF** per repo.

**Staleness SLI + errored-tick publish (Deliverable 3).** The Phase A
`striatum_metrics_snapshot_age_seconds` gauge is the staleness SLI;
`striatum_metrics_tick_status{status="ok|partial|error"}` is emitted **every**
scrape (closed enum, always present, immediately alertable). On an **errored**
tick `Collector.Refresh` republishes a **carried-forward** snapshot that preserves
the last-good data **and** the prior `builtAt` — so `snapshot_age` keeps climbing
rather than resetting — while stamping `tick_status=error`. A wedged/erroring
reconcile loop is thus directly visible instead of silently serving last-good
numbers.

**Semantics-grounded rules (Deliverable 4).** `go/pkg/metrics/rules/` ships
`recording_rules.yml` (5 pre-aggregations in the `striatum:level:op` colon
namespace) and `alerting_rules.yml`. The five RFC-mandated alerts — `NecrosisRate`,
`DoctorRed`, `WedgeAgeTail`, `LivenessMarginCollapse`, `SupervisorOriginFlood` —
lead, followed by `MetricsSnapshotStale`, `MetricsTickErrored`,
`LifecycleBalanceNonzero`, `MetricsCardinalityClipped`. The correctness property
(prior review's DEFECT 2): only **true monotonic counters** are wrapped in
`rate()/increase()`; **gauge-histograms** (`run_wedge_age`, `liveness_margin`) are
read by `histogram_quantile` **directly over the buckets** (no `rate()`), and the
snapshot clip counter uses `max_over_time`. `semantics.go` is the source of truth
and `TestPrometheusRulesRespectMetricRateSemantics` rejects any rule that applies a
counter-only function to a snapshot family.

## Open Questions — in/out-for-V1 decisions (all five recorded)

- **OQ1 — snapshot-staleness-as-liar / cold tier: PARTIAL-IN.** Staleness signals
  IN (`tick_status` + publish-on-errored-tick + `snapshot_age`/`MetricsSnapshotStale`
  + `MetricsTickErrored`); a second cold DB-projection principal is OUT for V1
  (coupled to OQ3's principal cost).
- **OQ2 — `lifecycle_balance` conservation gauge ("second doctor"): IN.** Shipped
  `striatum_lifecycle_balance`, folded at the apoptosis/necrosis site. V1 detects
  the highest-value blind spot: a necrosis-tagged terminal transition whose
  `stall_class` is outside the closed necrosis domain — a confirmed-dead transition
  that would otherwise vanish from both counters. Stays **zero** in healthy
  operation and across the F-A6 reversible-liveness path. Broader accounting noted
  as future work.
- **OQ3 — cold-tier authentication (postgres_exporter vs `striatum metrics --once`):
  OUT.** Decided with OQ1 — no cold tier means no second principal to authenticate;
  the in-process snapshot + `tick_status` already answers freshness/liveness.
- **OQ4 — event-sourced replay (`striatum metrics replay --since`): OUT.** The
  counters are re-derived from the durable `striatumd.events` ledger every tick, so
  they are restart-consistent by construction and Prometheus tolerates counter
  resets. Forensic-only; deferred.
- **OQ5 — `flora_diversity` Shannon index: OUT.** Needs a defensible alert
  threshold we do not yet have; the low-cardinality `origin` enum already turns a
  monoculture (e.g. an all-`origin="supervisor"` flood, #417) into a directly
  countable signal `SupervisorOriginFlood` alerts on. In until a threshold is data
  justified.

## Acceptance-criteria → test mapping

| Requirement | Test (package) | Verify |
| --- | --- | --- |
| D1: repo-A token sees only repo-A series; loopback sees all | `TestScopedMetricsRemoteFiltersByCapability`, `TestScopedMetricsLoopbackServesFull`, `TestScopedRenderHidesForeignBuckets` (striatumd, metrics) | `go test ./cmd/striatumd/ ./pkg/metrics/` |
| **D1 (collision safety): colliding repos stay isolated under a scoped token** | `TestScopedMetricsIsolatesCollidingReposByRepoID` (striatumd), `TestScopedRenderIsolatesCollidingRepos` (metrics) | `go test ./cmd/striatumd/ ./pkg/metrics/` |
| D1: fail closed beyond loopback | `TestScopedMetricsRemoteRequiresBearer`, `TestRequestIsLoopback` (striatumd) | `go test ./cmd/striatumd/` |
| D2: no-consent → no Provenance series but `repo_consent=0`; consent → Provenance appears | `TestConsentGatesProvenanceFamily` (metrics) + golden | `go test ./pkg/metrics/` |
| D2: raw repo_id never leaks; only salted bucket | `TestSurrogateDeterministicBounded`, `TestMetricsRedactionGoldenAndForbiddenContent` (metrics) | `go test ./pkg/metrics/` |
| D3: errored tick sets `tick_status` + preserves last-good age | `TestRefreshErroredTickPublishesErrorStatus`, `TestErroredTickSnapshotPreservesDataAndStampsError` (metrics) | `go test ./pkg/metrics/` |
| D4: rules valid YAML, reference only registered metrics, 5 alerts present | `TestPrometheusRulesAreValidAndWellFormed`, `TestPrometheusRulesReferenceRegisteredMetrics`, `TestPrometheusRulesIncludeMandatedAlerts` (metrics) | `go test ./pkg/metrics/` |
| **D4 (semantics): each rule's query valid for its metric's TYPE/semantics** | `TestPrometheusRulesRespectMetricRateSemantics`, `TestMetricRateSemanticsCoversRegistry`, `TestCounterFunctionArgumentsScanner` (metrics) | `go test ./pkg/metrics/` |
| OQ2 (F-A6): liveness miss recovers without necrosis; balance stays 0 | `TestLivenessMissCanRecoverWithoutNecrosis`, `TestLifecycleBalanceCountsUnaccountedTerminal` (metrics) | `go test ./pkg/metrics/` |
| Contract: allowlist hash + boot abort updated for new families | `TestMetricsAllowlistMatchesRegistry`, `TestVerifyAllowlistDetectsDrift` (metrics) | `go test ./pkg/metrics/` |
| Contract: Forbidden still refused; only `repo_runs` is Provenance | `TestRegisterRefusesForbiddenFamily`, `TestDefaultRegistryIsOperationalAndStable` (metrics) | `go test ./pkg/metrics/` |
| Necrosis domain still pinned to confirmed-dead constants | `TestNecrosisDomainMatchesConfirmedDeadConstants` (mutations) | `go test ./pkg/mutations/ -run …` |

## Verification — commands + results (re-run for this apply step, in `go/`)

```
$ make -C go build                                                               # exit 0
  go build -o bin/striatum / bin/striatumd / bin/striatum-supervisor-helper

$ go build ./...                                                                  # exit 0
$ go vet ./pkg/metrics/... ./cmd/striatumd/...                                    # exit 0

$ go test -count=1 ./pkg/metrics/...
ok  	github.com/halbritt/striatum/go/pkg/metrics	0.008s

$ go test -count=1 ./cmd/striatumd/...
ok  	github.com/halbritt/striatum/go/cmd/striatumd	0.030s

$ go test -count=1 ./pkg/db/...                 # migration contiguity invariant intact (no migration added)
ok  	github.com/halbritt/striatum/go/pkg/db	0.119s

$ go test -count=1 ./pkg/mutations/ -run TestNecrosisDomainMatchesConfirmedDeadConstants
ok  	github.com/halbritt/striatum/go/pkg/mutations	0.003s
```

Named Phase D acceptance tests confirmed individually (all PASS):
`TestMetricsAllowlistMatchesRegistry`, `TestConsentGatesProvenanceFamily`,
`TestScopedRenderIsolatesCollidingRepos`, `TestRefreshErroredTickPublishesErrorStatus`,
`TestMetricsRedactionGoldenAndForbiddenContent`, `TestPrometheusRulesIncludeMandatedAlerts`,
`TestPrometheusRulesRespectMetricRateSemantics`.

Non-vacuity (DEFECT 2): temporarily wrapping the wedge-age bucket back in `rate(...)`
makes `TestPrometheusRulesRespectMetricRateSemantics` FAIL; restoring the direct
`histogram_quantile` returns it to green. The redaction golden
(`testdata/metrics_golden.txt`) and `metrics_allowlist.json` were regenerated with
`STRIATUM_UPDATE_GOLDEN=1 STRIATUM_UPDATE_ALLOWLIST=1`, diffs reviewed (enum/small-int
labels only), and the forbidden-content regex still passes.

## Deploy note (HELD operator step)

Consent is persisted in the **existing** `striatumd.repositories.settings_json`
jsonb column under the key `metrics_provenance_consent` (a repo defaults to no
consent when the key is absent). **No new SQL migration was added** — the branch
base is at migration **0040** and the runner enforces a strict contiguity invariant
(`TestMigrationsAreOrdered`: `len(migrations) == LatestDaemonDBVersion` and
`migration.Version == index+1`). Adding a `0043` file while `0041`/`0042` are
reserved by concurrent work absent from this base would create a version gap and
fail that guardrail; using the existing per-repo settings column resolves the
prompt's "number it 0043+" instruction against the in-branch contiguity guardrail
with zero DDL. A dedicated table, if preferred later, must be numbered ≥ the
then-current contiguous head, not 0043 in this base.

The operator enablement step is **HELD**: flipping a repo's
`settings_json.metrics_provenance_consent` to `true` (plus the next sweep-tick
refold) is the explicit per-repo product decision that turns on Provenance
families. No repo is flipped on by this change; every repo defaults OFF. Any future
migration `apply` + daemon restart is likewise a separate, explicitly-held operator
step, not part of this lane.

## Closing

**RFC 0137 A→D is now fully implemented** — the exporter ships a lock-disjoint
zero-query read path (A); a failure-mode taxonomy with the apoptosis/necrosis spine
and the F-A6 liveness counter (B); the `Classification`/`Register()` refusal, series
budget + `cardinality_clipped_total`, boot-time allowlist hash check, and
doctor-as-collector enforced contract (C); and — this phase — capability-scoped
multi-tenant filtering **isolated by real `repository_id` (collision-safe)**, opt-in
per-repo provenance consent, a tick-status staleness SLI with publish-on-errored-tick,
the lifecycle-balance "second doctor", and committed recording/alerting rules
**grounded in each metric's true time-series semantics**, with all five Open
Questions explicitly decided. The review verdict was **accept** with no blocking
findings; build and all in-scope test suites are green. **Ready for the verifier
gate.**

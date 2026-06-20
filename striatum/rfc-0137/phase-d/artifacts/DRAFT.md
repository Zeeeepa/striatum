# RFC 0137 Phase D — multi-tenant hardening, consent, alert rules (DRAFT)

author: author-author-002

Phase D is the **final** phase of RFC 0137. It builds on the landed Phase A read
path (snapshot / render / `/metrics`), the Phase B failure-mode taxonomy
(apoptosis/necrosis split, F-A6 liveness counter, lease/wedge/margin families),
and the Phase C contract harness (`Classification`/`Register()` refusal, series
budget + `cardinality_clipped_total`, boot-time allowlist hash check,
`doctor_problems{class}` collector). It adds the four §Design-Sketch 4 / §Roadmap
Phase D deliverables and resolves all five Open Questions with explicit in/out-for-V1
decisions. **This completes RFC 0137 A→D.**

Everything is green: `make -C go build`, `go build ./...`, `go vet`,
`go test ./pkg/metrics/...`, `go test ./cmd/striatumd/...`, and
`go test ./pkg/db/...` all pass (exact commands + output at the end).

## Files touched

New (`go/pkg/metrics/`):
- `surrogate.go` — the salted per-repo surrogate (`bucket = HMAC-SHA256(daemon-secret, repo_id) mod K`, K=256).
- `consent.go` — the `TickStatus` enum, the OQ2 lifecycle-balance blind-spot helper, and the publish-on-errored-tick constructor.
- `rules.go` + `rules/recording_rules.yml` + `rules/alerting_rules.yml` — version-controlled Prometheus rules, embedded.
- `phase_d_test.go`, `rules_test.go` — Phase D unit + guardrail tests.

Modified (`go/pkg/metrics/`):
- `registry.go` — register the four new families (`metrics_tick_status`, `lifecycle_balance`, `metrics_repo_consent` Operational; `repo_runs` Provenance).
- `snapshot.go` — `RepoMetric` input, per-repo consent/run fold with consent gating, `repoBuckets`/`tickStatus`/`unaccountedTerminal` state, `RepoBuckets()`/`TickStatus()` accessors, lifecycle-balance accounting in `addEvent`.
- `render.go` — `WriteText` → `WriteTextScoped(w, now, allowed)`; render funcs for the four new families; exported `ScrapeContentType()`.
- `collector.go` — `NewCollectorWithSurrogate`; per-repo consent (`repoConsentFlags`) + run-state (`repoRunStateCounts`) folds; `Refresh` now tracks `ok|partial|error` and publishes a carried-forward snapshot on an errored tick.
- `metrics_allowlist.json` + `testdata/metrics_golden.txt` — regenerated for the four new families (reviewed diffs).
- `registry_test.go`, `redaction_test.go` — updated for the first Provenance family + per-repo sentinels.

New / modified (`go/cmd/striatumd/`):
- `metrics_scope.go` (new) — the capability-scoped `/metrics` wrapper.
- `metrics_scope_test.go` (new) — loopback-full / remote-scoped / fail-closed tests.
- `main.go` — build the surrogate from the RFC 0110 authority secret, wire it into the collector, and wrap the exporter handler with `newScopedMetricsHandler`.

## Deliverable 1 — capability-scoped `/metrics` filtering (reuses RFC 0043)

Loopback stays **default-open** (Phase A). Beyond loopback the wrapper
(`newScopedMetricsHandler`, `go/cmd/striatumd/metrics_scope.go`):

1. Classifies the peer by **`RemoteAddr`** (not the attacker-controllable `Host`
   header), failing closed on an empty/unparseable address (`requestIsLoopback`).
2. Requires an `Authorization: Bearer <token>` (401 if absent).
3. Resolves the authorized buckets by asking the **same `rpc.Authorizer` that
   gates RPC** — `authorizer.Authorize(&rpc.CapabilityRead, repoID, token)` — for
   each repository the published snapshot folded (`Snapshot.RepoBuckets()`), and
   keeps the bucket of every repo the token is `allowed` for. **No parallel ACL is
   invented**; a daemon-global read grant authorizes every repo, a repo-scoped
   token only its own (`MemoryAuthorizer`/`PostgresAuthorizer` semantics,
   `go/pkg/rpc/capability.go`). A token authorizing none of the served repos is
   **403** (fail closed).
4. Renders `WriteTextScoped(w, now, allowed)`: repo-aggregate **Operational**
   families are always emitted; the two families carrying the salted `bucket`
   (`metrics_repo_consent`, `repo_runs`) are emitted only for authorized buckets.

A repo-A token therefore never sees repo-B's surrogate buckets. The repo_id →
bucket map is read from the already-published snapshot, so the scoped path does
only RFC 0043 auth lookups (one per served repo, the cost of one RPC auth) and
**never re-folds metrics**; the loopback path stays strictly zero-query (the
Phase A `panic-on-query` test still targets the unwrapped handler).

## Deliverable 2 — consent gauge + per-repo consent

- `striatum_metrics_repo_consent{bucket}` (Operational) is emitted for **every**
  active repo (0 or 1), so the **absence** of provenance is itself a scrapeable
  fact. It carries only the salted bucket and the single consent bit.
- `striatum_repo_runs{bucket,state}` (the first **Provenance** family) is folded
  **only for consented repos** (gating at fold time, `Snapshot.addRepoMetric`) and
  filtered again by the capability scope at render time. Both gates compose.
- Operational families default ON; Provenance defaults **OFF** per repo.

### Persistence (migration number — read this)

Consent is persisted in the **existing** `striatumd.repositories.settings_json`
jsonb column under the key `metrics_provenance_consent` (read by
`Collector.repoConsentFlags`; a repo defaults to no consent when the key is
absent). **No new SQL migration was added**, deliberately:

- This branch base is at migration **0040**, and the runner enforces a **strict
  contiguity invariant** — `TestMigrationsAreOrdered`
  (`go/pkg/db/migrations_test.go:217`) asserts `len(migrations) ==
  LatestDaemonDBVersion` **and** `migration.Version == index+1` for every file.
  Adding a `0043` file while `0041`/`0042` are absent in this base (they are
  reserved by concurrent work not present here) would create a version gap and
  **fail that guardrail** / break boot version-checks. The RFC prompt's "number it
  0043+" instruction and the in-branch contiguity guardrail are in direct
  tension; using the existing per-repo settings column resolves it without
  weakening the guardrail and is the **smallest possible schema footprint** (zero
  DDL). The prompt makes the migration conditional ("if you add a SQL migration").
- **Operator deploy note (HELD):** flipping a repo's
  `settings_json.metrics_provenance_consent` to `true` is the explicit per-repo
  **product-decision** that enables Provenance families; that flag-write + the next
  sweep-tick refold is the separate operator enablement step and is currently
  **HELD** (no repo is flipped on by this change; every repo defaults OFF). Were a
  dedicated table preferred later, it must be numbered ≥ the then-current
  contiguous head, not 0043 in this base.

## Deliverable 3 — staleness SLI + errored-tick publish

- The Phase A `striatum_metrics_snapshot_age_seconds` gauge is the staleness SLI;
  the `MetricsSnapshotStale` alert fires on it (>300s).
- `striatum_metrics_tick_status{status="ok|partial|error"}` is emitted **every**
  scrape (1 for the active status, 0 for the others — a closed enum, always
  present, immediately alertable).
- `Collector.Refresh` now classifies each tick: `error` when a load-bearing Phase
  A fold fails, `partial` when a best-effort Phase B/C/D fold degrades, else `ok`.
  On an **errored** tick it republishes a **carried-forward** snapshot
  (`erroredTickSnapshot`) that preserves the last-good data **and** the prior
  `builtAt` — so `snapshot_age` keeps climbing (the SLI is not reset) — while
  stamping `tick_status=error`. A wedged/erroring reconcile loop is thus **directly
  visible** rather than silently serving last-good numbers.

## Deliverable 4 — version-controlled Prometheus rules

`go/pkg/metrics/rules/` ships `recording_rules.yml` (5 pre-aggregations using the
`striatum:level:op` colon namespace) and `alerting_rules.yml`. The five RFC-mandated
alerts — **`NecrosisRate`, `DoctorRed`, `WedgeAgeTail`, `LivenessMarginCollapse`,
`SupervisorOriginFlood`** — lead, followed by `MetricsSnapshotStale`,
`MetricsTickErrored`, `LifecycleBalanceNonzero`, and `MetricsCardinalityClipped`
(OQ1/OQ2 coverage). The files are embedded (`rules.go`), and
`TestPrometheusRulesReferenceRegisteredMetrics` parses them and fails if any rule
references a series the registry does not export (histogram `_bucket/_sum/_count`
suffixes are resolved to their family); `TestPrometheusRulesIncludeMandatedAlerts`
pins the five names.

## Open Questions — explicit in/out-for-V1 decisions

**OQ1 — snapshot-staleness-as-liar / cold tier: PARTIAL-IN (staleness signals IN;
cold tier OUT).** Implemented `tick_status` + publish-on-errored-tick + the
`snapshot_age`/`MetricsSnapshotStale` and `MetricsTickErrored` alerts. *Rationale:*
these make a wedged/erroring fold loop directly visible without standing up a
second DB principal. A separate cold DB-projection tier (out-of-band liveness
check) is **not** in scope for V1 — it is coupled to OQ3's principal cost and adds
operational surface we do not yet need to answer "is the fold loop alive?".

**OQ2 — `lifecycle_balance` conservation gauge ("second doctor"): IN.** Shipped
`striatum_lifecycle_balance`, folded at the apoptosis/necrosis site. V1 detects the
highest-value, well-defined blind spot: a terminal transition that **declared a
death** (a necrosis-tagged `session.closed`) whose `stall_class` is outside the
closed necrosis domain — a confirmed-dead transition that would otherwise vanish
from **both** counters. *Rationale:* the RFC's own Acceptance Criteria reference
this gauge (F-A6), and a continuously-scraped conservation assertion is exactly the
"metrics layer as a second doctor" idea. It stays **zero** in healthy operation
(and across the F-A6 reversible-liveness path). Broadening the accounting to every
terminal transition class is noted as future work; the narrow V1 definition is
correct and testable rather than fragile.

**OQ3 — cold-tier authentication (postgres_exporter vs `striatum metrics --once`):
OUT.** *Rationale:* decided together with OQ1 — with no cold tier in V1 there is no
second principal to authenticate; introducing a `postgres_exporter` under the RFC
0110 PG-auth boundary is operational coupling we are deliberately deferring. The
in-process snapshot + `tick_status` already answers the freshness/liveness need.

**OQ4 — event-sourced replay (`striatum metrics replay --since`): OUT.**
*Rationale:* the Phase B/D counters are already re-derived from the durable
`striatumd.events` ledger every tick, so they are restart-consistent by
construction and Prometheus tolerates counter resets. A one-shot historical
exposition is a forensic nicety with no current operator demand; deferred.

**OQ5 — `flora_diversity` Shannon index: OUT.** *Rationale:* a single
monoculture vital sign needs a **defensible alert threshold** we do not yet have,
and the existing low-cardinality `origin` enum already turns a monoculture (e.g. an
all-`origin="supervisor"` flood, #417) into a directly-countable signal that
`SupervisorOriginFlood` alerts on. In until a threshold is justified by real data —
out for V1.

## Acceptance-criteria → test mapping

| Requirement | Test (package) | Verify |
| --- | --- | --- |
| D1: repo-A token sees only repo-A series; loopback sees all | `TestScopedMetricsRemoteFiltersByCapability`, `TestScopedMetricsLoopbackServesFull`, `TestScopedRenderHidesForeignBuckets` (striatumd, metrics) | `go test ./cmd/striatumd/ ./pkg/metrics/` |
| D1: fail closed beyond loopback | `TestScopedMetricsRemoteRequiresBearer`, `TestRequestIsLoopback` (striatumd) | `go test ./cmd/striatumd/` |
| D2: no-consent → no Provenance series but `repo_consent=0`; consent → Provenance appears | `TestConsentGatesProvenanceFamily` (metrics) + golden | `go test ./pkg/metrics/` |
| D2: raw repo_id never leaks; only salted bucket | `TestSurrogateDeterministicBounded`, `TestMetricsRedactionGoldenAndForbiddenContent` (metrics) | `go test ./pkg/metrics/` |
| D3: errored tick sets `tick_status` + preserves last-good age | `TestRefreshErroredTickPublishesErrorStatus`, `TestErroredTickSnapshotPreservesDataAndStampsError` (metrics) | `go test ./pkg/metrics/` |
| D4: rules valid YAML, reference only registered metrics, 5 alerts present | `TestPrometheusRulesAreValidAndWellFormed`, `TestPrometheusRulesReferenceRegisteredMetrics`, `TestPrometheusRulesIncludeMandatedAlerts` (metrics) | `go test ./pkg/metrics/` |
| OQ2 (F-A6): liveness miss recovers without necrosis; balance stays 0 | `TestLivenessMissCanRecoverWithoutNecrosis`, `TestLifecycleBalanceCountsUnaccountedTerminal` (metrics) | `go test ./pkg/metrics/` |
| Contract: allowlist hash + boot abort updated for new families | `TestMetricsAllowlistMatchesRegistry`, `TestVerifyAllowlistDetectsDrift` (metrics) | `go test ./pkg/metrics/` |
| Contract: Forbidden still refused; only `repo_runs` is Provenance | `TestRegisterRefusesForbiddenFamily`, `TestDefaultRegistryIsOperationalAndStable` (metrics) | `go test ./pkg/metrics/` |
| Necrosis domain still pinned to confirmed-dead constants | `TestNecrosisDomainMatchesConfirmedDeadConstants` (mutations) | `go test ./pkg/mutations/` |

### Verification commands + results (run in `go/`)

```
$ make -C go build
… go build -o bin/striatum / bin/striatumd / bin/striatum-supervisor-helper      # exit 0

$ go build ./...                                                                  # exit 0
$ go vet ./pkg/metrics/... ./cmd/striatumd/...                                    # exit 0

$ go test -count=1 ./pkg/metrics/...
ok  	github.com/halbritt/striatum/go/pkg/metrics	0.007s

$ go test -count=1 ./cmd/striatumd/...
ok  	github.com/halbritt/striatum/go/cmd/striatumd	0.026s

$ go test -count=1 ./pkg/db/...                 # migration contiguity invariant intact
ok  	github.com/halbritt/striatum/go/pkg/db	0.107s

$ go test -count=1 ./pkg/mutations/ -run TestNecrosisDomainMatchesConfirmedDeadConstants
ok  	github.com/halbritt/striatum/go/pkg/mutations	0.003s
```

The redaction golden (`testdata/metrics_golden.txt`) and `metrics_allowlist.json`
were regenerated with `STRIATUM_UPDATE_GOLDEN=1 STRIATUM_UPDATE_ALLOWLIST=1 go test
./pkg/metrics/...` and the diffs reviewed; the new `bucket`/`status` labels are
enum/small-integer only (no raw ids) and the forbidden-content regex still passes.

### Pre-existing, out-of-scope failure (not introduced here)

`go test ./pkg/mutations/...` has **one** failing test in this environment —
`TestSpawnRunAsSpecResolvesLaneUser` (it expects a configured host `striatum-lane`
user / writable lane home). It fails **identically on the untouched `main` tree**
(`/home/halbritt/git/striatum/go`), is unrelated to RFC 0137 / metrics, and is
left untouched. No metrics-importing test in that package regressed.

## Status

RFC 0137 Phases A→D are complete. The exporter now ships: a lock-disjoint
zero-query read path (A); a failure-mode taxonomy with the apoptosis/necrosis
spine and the F-A6 liveness counter (B); the Classification/Register refusal,
series budget, boot-time allowlist hash check, and doctor-as-collector (C); and —
this phase — capability-scoped multi-tenant filtering, opt-in per-repo provenance
consent, a tick-status staleness SLI with publish-on-errored-tick, the
lifecycle-balance "second doctor", and committed recording/alerting rules (D), with
all five Open Questions explicitly decided.

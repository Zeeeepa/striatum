# RFC 0137 Phase A — read-path skeleton + redaction harness (handoff)

author: author-author-001

Phase A of RFC 0137 (`striatumd` Prometheus exporter) is implemented and green:
a new `go/pkg/metrics` package plus its wiring into `striatumd`. The exporter
serves a Prometheus `/metrics` surface whose scrape cost is O(1) and
lock-disjoint from every state mutator — `Load()` an `atomic.Pointer`, render
text, write. The snapshot is folded **once per resident recovery-sweep tick**,
never on the scrape path, and the exfiltration contract is enforced by a
committed golden file plus forbidden-content regexes that run in the normal test
suite.

This was built contract-first (TDD): the three tests were written and watched
fail (the redaction test failed on the missing golden; the scrape tests
compiled and ran) before the golden was committed and the wiring finished.

## What I implemented

New package `go/pkg/metrics`:

- **`snapshot.go`** — the immutable `Snapshot` value object (seed Phase A
  metrics) and the **package-level `atomic.Pointer[Snapshot]`** with
  `Publish()` (Store) / `Load()` (Load), following the exact lock-free
  copy-on-publish pattern in `go/pkg/db/write_boundary.go` and
  `go/pkg/db/authority.go`. `BuildSnapshot` folds `[]RunObservation` →
  closed-enum run-state counts, discarding every sensitive column. The closed
  run-state enum is pinned to `runs_state_check`
  (`go/pkg/db/sql/0021_run_needs_operator.sql`); unknown states bucket to
  `other` so the `state` label can never grow cardinality with run history.
- **`render.go`** — deterministic Prometheus text exposition (`# HELP`/`# TYPE`)
  for the three seed metrics. Help text is deliberately free of slashes,
  `--flag=` shapes, `author:` bylines, and 40-hex runs so it can never trip the
  forbidden-content regexes.
- **`collector.go`** — `Collector` owns the daemon runner for the tick-time
  fold (`Refresh`: two small aggregate queries → `Publish`) and exposes the
  `/metrics` `Handler()`. The scrape path reads only the published pointer; the
  runner is reachable only through `Refresh`.
- **`testdata/metrics_golden.txt`** — the committed byte-for-byte golden.

Seed metrics (Phase A only — small but real):

- `striatum_stranded_supervisors` (gauge) — `process_supervisors` still
  `attached` to a terminal run, the #417 phantom-supervisor signal (the exact
  shape the status read path joins + probes; see
  `go/pkg/db/sql/0033_reap_terminal_run_supervisors.sql`).
- `striatum_runs{state=…}` (gauge) — run counts over the closed run-state enum
  plus an `other` bucket.
- `striatum_metrics_snapshot_age_seconds` (gauge) — `now − builtAt`, the
  staleness signal.

Wiring into `striatumd` (`go/cmd/striatumd/`):

- **`web_service.go`** — `newDaemonHTTPHandler` now routes an exact `/metrics`
  to the exporter on the **existing loopback MCP/web listener** (no new public
  listener), next to `/v1/health`. A nil exporter falls through, keeping the
  route purely additive.
- **`main.go`** — constructs the `metrics.Collector` over the live runner, does
  a best-effort initial fold so `/metrics` has data before the first tick, and
  wraps `recoverypkg.ActiveRunSweep.SweepOnce` so each resident recovery-sweep
  tick folds + publishes a fresh snapshot. A fold error is logged and never
  fails the recovery sweep (metrics are observational; the last-good snapshot
  keeps serving). `startMCPHTTPServer` / `startRecoveryScheduler` thread the
  handler/collector through.

Files touched:

| File | Change |
| --- | --- |
| `go/pkg/metrics/snapshot.go` | new — `Snapshot`, package atomic pointer, fold |
| `go/pkg/metrics/render.go` | new — Prometheus text exposition |
| `go/pkg/metrics/collector.go` | new — tick fold + `/metrics` handler |
| `go/pkg/metrics/redaction_test.go` | new — golden + forbidden-regex test |
| `go/pkg/metrics/scrape_test.go` | new — zero-query + concurrent-identity tests |
| `go/pkg/metrics/testdata/metrics_golden.txt` | new — committed golden |
| `go/cmd/striatumd/web_service.go` | `/metrics` route in `newDaemonHTTPHandler` |
| `go/cmd/striatumd/main.go` | collector construction + sweep-tick fold wiring |
| `go/cmd/striatumd/web_mux_test.go` | route signature update + `/metrics` routing test |
| `go/cmd/striatumd/scheduler_panic_test.go` | scheduler signature update (nil collector) |

No new third-party dependencies: the Prometheus text format is hand-rendered (it
is trivial), keeping the privacy/cardinality contract enforced in our own code
per the RFC's "enforced in code" stance. The `prometheus` client library was not
pulled in.

## Phase A acceptance criteria → proving test

| RFC Phase A acceptance criterion | Where it is proven |
| --- | --- |
| `/metrics` serves valid Prometheus text with **zero PG queries** at scrape time (panic-on-query) | `TestScrapeIssuesZeroQueries` — drives the handler (built from a collector whose `Querier.Query` panics) with 256 scrapes; all 200, zero queries. A `Refresh` sanity check proves the querier IS a live DB entrypoint, so the zero is non-vacuous. |
| **Concurrent-scrape identity** (atomic pointer read, never recomputed) | `TestConcurrentScrapesSeeIdenticalSnapshot` — 1000 concurrent `Load()`s all observe the identical published `*Snapshot`. |
| **Golden-file + forbidden-content redaction**, wired into `make check` | `TestMetricsRedactionGoldenAndForbiddenContent` — seeds the snapshot input with a repo path, branch, 40-hex sha, `--prompt=` argv fragment, and `author:` byline; asserts the body byte-for-byte against `testdata/metrics_golden.txt` AND that no sentinel literal / forbidden-shape regex appears. It is a normal `Test*` in `go/pkg/metrics`, so `go test ./...` (`make -C go test`, root `make check: lint test`) runs it — not manual-only. |
| **Snapshot published from the sweep tick** (no separate DB-polling loop; no PG on scrape) | `main.go` `startRecoveryScheduler` wraps `ActiveRunSweep.SweepOnce` to call `collector.Refresh` once per tick; the fold issues a fixed, small number of aggregate queries off the hot path. |
| **`localhost` bind** (no new public listener) | `/metrics` is mounted into the existing loopback daemon HTTP handler; `TestDaemonHTTPHandlerRoutesMetrics` asserts an exact `/metrics` routes to the exporter and falls through to the web service when nil. |

The redaction test embodies the RFC's "the golden hash only catches changed
label *names*; the regex catches a leaked *value* under an already-allowed
name" — both assertions must pass.

## Verification commands and results

Run from the per-job worktree's `go/` directory.

| Command | Result |
| --- | --- |
| `make -C go build` | **PASS** — `bin/striatum`, `bin/striatumd`, `bin/striatum-supervisor-helper` all built |
| `go build ./...` | **PASS** — exit 0 (nothing else broke) |
| `go test ./pkg/metrics/...` | **PASS** — `ok … 0.003s` (3 tests: redaction-golden, zero-query, concurrent-identity) |
| `go test ./cmd/striatumd/` | **PASS** — `ok …` (includes `TestDaemonHTTPHandlerRoutesMetrics`, `TestDaemonHTTPHandlerRoutesMCPAndWeb`, `TestRecoverySchedulerRecoversAndCancelsOnPanic`) |
| `go vet ./cmd/striatumd/... ./pkg/metrics/...` | **PASS** — exit 0 |
| `gofmt -l pkg/metrics/ cmd/striatumd/…` | **CLEAN** — no files listed |

Verbose metrics run:

```
--- PASS: TestMetricsRedactionGoldenAndForbiddenContent (0.00s)
--- PASS: TestScrapeIssuesZeroQueries (0.00s)
--- PASS: TestConcurrentScrapesSeeIdenticalSnapshot (0.00s)
ok  github.com/halbritt/striatum/go/pkg/metrics
```

Lint note: `golangci-lint` is not installed on this box (`make lint` needs the
pinned binary via `make -C go lint-tools`, which requires network). I reproduced
its enabled analyzers as far as locally available — `go vet` (govet) and
`gofmt` are clean, and the code uses explicit `_ =` error discards on the
write/observe paths so `errcheck` has nothing to flag. The full PG-backed
`make -C go test ./...` suite was not run here (it needs a live test database);
`go build ./...` plus the targeted package tests above are green.

## Scope confirmation — Phase B/C/D deliberately NOT implemented

I implemented **Phase A only**. The following are explicitly out of scope and
were not built:

- No failure-mode taxonomy (`apoptosis_total` / `necrosis_total` /
  `lease_transitions_total` / `run_wedge_age_seconds` /
  `liveness_deadline_*`), no `Origin`/`*Reason` enums (Phase B).
- No `Classification` / `Register()` refusal of `Forbidden` families, no
  per-family series budget / `cardinality_clipped_total`, no
  `metrics_allowlist.json` boot-time hash check, no `doctor_problems{class}`
  collector, no `TestDoctorClassRejectsDynamicIdentifiers` (Phase C).
- No capability-scoped `/metrics` filtering, no `metrics_repo_consent` gauge,
  no staleness SLI alert / `tick_status`, no Prometheus recording/alerting
  rules, no cold DB-projection tier (Phase D).

The seed metrics, the package-level atomic snapshot, the sweep-tick fold, the
localhost-bound `/metrics` handler, and the golden + forbidden-regex redaction
harness are the complete Phase A surface, and nothing beyond it was added.

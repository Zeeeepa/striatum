---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
workflow: "rfc0137-phase-a"
phase: "apply"
tags: ["rfc_0137", "phase_a", "metrics"]
inputs:
  - "striatum/rfc-0137/phase-a/artifacts/DRAFT.md"
  - "striatum/rfc-0137/phase-a/artifacts/review/REVIEW.md"
---

# RFC 0137 Phase A — Apply / Finalize Summary

author: author-author-002

Phase A of RFC 0137 (`striatumd` Prometheus exporter) is **accepted and
finalized**. The reviewer returned `accept` with no actionable nits within Phase
A scope (`striatum/rfc-0137/phase-a/artifacts/review/REVIEW.md`), so this apply
pass made **no source changes**: it re-confirmed the slice is green, verified the
redaction golden/forbidden-regex test runs in the normal `go test ./...` path,
confirmed the surface stays loopback-only, and confirmed Phase B/C/D was not
implemented. This artifact is the durable provenance for that finalization.

## Final file list

New package `go/pkg/metrics/` — a lock-disjoint, zero-DB-query Prometheus read
surface published once per recovery-sweep tick:

| File | Lines | What it does |
| --- | ---: | --- |
| `go/pkg/metrics/snapshot.go` | 149 | Immutable `Snapshot` value object + the package-level `atomic.Pointer[Snapshot]` with `Publish()` (Store) / `Load()` (Load), the lock-free copy-on-publish pattern from `go/pkg/db/write_boundary.go` / `authority.go`. `BuildSnapshot` folds `[]RunObservation` → closed run-state-enum counts, discarding every sensitive column; the canonical state enum is pinned to `runs_state_check`, unknown states bucket to `other` so the `state` label cannot grow cardinality with run history. |
| `go/pkg/metrics/render.go` | 65 | Deterministic Prometheus text exposition (`# HELP`/`# TYPE`) for the three seed metrics. Help text is free of slashes, `--flag=` shapes, `author:` bylines, and 40-hex runs so it can never trip the forbidden-content regexes. |
| `go/pkg/metrics/collector.go` | 135 | `Collector` owns the daemon runner for the tick-time fold (`Refresh`: two small aggregate queries → `Publish`) and exposes the `/metrics` `Handler()`. The scrape path reads only the published pointer; the runner is reachable only through `Refresh`. |
| `go/pkg/metrics/redaction_test.go` | 117 | The exfiltration contract: golden byte-equality + forbidden-content regexes. |
| `go/pkg/metrics/scrape_test.go` | 92 | Zero-DB-query (panic-on-query) and concurrent-scrape-identity tests. |
| `go/pkg/metrics/testdata/metrics_golden.txt` | 18 | The committed byte-for-byte golden exposition. |

Wiring into `go/cmd/striatumd/`:

| File | What changed |
| --- | --- |
| `go/cmd/striatumd/web_service.go` | `newDaemonHTTPHandler` routes an exact `/metrics` to the exporter on the **existing loopback MCP/web listener** (no new public listener), next to `/v1/health`. A nil exporter falls through to the web service, keeping the route purely additive. |
| `go/cmd/striatumd/main.go` | Constructs `metrics.Collector` over the live runner (type-asserted to `metrics.Querier`), does a best-effort initial fold so `/metrics` has data before the first tick, and wraps the recovery-sweep tick so each resident tick folds + publishes a fresh snapshot. A fold error is logged and never fails the sweep (metrics are observational; the last-good snapshot keeps serving). The loopback bind guard (`main.go:543-557`) rejects any non-loopback MCP HTTP bind. |
| `go/cmd/striatumd/web_mux_test.go` | Route-signature update + `TestDaemonHTTPHandlerRoutesMetrics`: asserts an exact `/metrics` routes to the exporter and falls through to the web service when the exporter is nil. |
| `go/cmd/striatumd/scheduler_panic_test.go` | Scheduler-signature update (passes a nil collector) so the existing panic-recovery test compiles against the new `startRecoveryScheduler` arity. |

Seed metrics (Phase A only): `striatum_stranded_supervisors` (gauge, the #417
phantom-supervisor signal), `striatum_runs{state=…}` (gauge over the closed
run-state enum + `other`), `striatum_metrics_snapshot_age_seconds` (gauge,
`now − builtAt` staleness). No new third-party dependency — the Prometheus text
format is hand-rendered, keeping the privacy/cardinality contract enforced in our
own code per the RFC's "enforced in code" stance.

## Phase A acceptance criteria → proving test

| RFC Phase A acceptance criterion | Proving test |
| --- | --- |
| `/metrics` serves valid Prometheus text with **zero PG queries** at scrape time | `TestScrapeIssuesZeroQueries` (`go/pkg/metrics/scrape_test.go`) — 256 scrapes against a handler built from a collector whose `Querier.Query` panics; all 200, zero queries. A `Refresh` sanity check proves the querier IS a live DB entrypoint, so the zero is non-vacuous. |
| **Concurrent-scrape identity** (atomic pointer read, never recomputed) | `TestConcurrentScrapesSeeIdenticalSnapshot` — 1000 concurrent `Load()`s all observe the identical published `*Snapshot`. |
| **Golden-file + forbidden-content redaction**, wired into the normal test path | `TestMetricsRedactionGoldenAndForbiddenContent` — seeds the snapshot input with a repo path, branch, 40-hex sha, `--prompt=` argv fragment and `author:` byline; asserts the body byte-for-byte against `testdata/metrics_golden.txt` AND that no sentinel literal / forbidden-shape regex appears. It is a plain `Test*` with **no build tags**, so `go test ./...` / `make test` / `make check` (`check-tests` runs `go test … ./...`) execute it — not a manual-only check. |
| **Snapshot published from the sweep tick** (no separate DB-polling loop; no PG on scrape) | `main.go:769-793` `startRecoveryScheduler` wraps the recovery-sweep tick to call `metricsCollector.Refresh` once per tick; the fold issues a fixed, small number of aggregate queries off the hot path. |
| **`localhost` bind** (no new public listener) | `web_service.go:61-68` mounts `/metrics` into the existing loopback daemon HTTP handler; `main.go:543-557` rejects a non-loopback MCP HTTP bind; `TestDaemonHTTPHandlerRoutesMetrics` pins the routing. |

## Verification commands and results (this apply pass)

Run from the per-job worktree's `go/` directory on run branch
`striatum/rfc0137-phase-a`.

`make -C go build` → **PASS**:

```
go build -ldflags "…" -o bin/striatum ./cmd/striatum
go build -ldflags "…" -o bin/striatumd ./cmd/striatumd
go build -ldflags "…" -o bin/striatum-supervisor-helper ./cmd/striatum-supervisor-helper
EXIT: 0
```

`go test ./pkg/metrics/... -v` → **PASS** (3 tests):

```
--- PASS: TestMetricsRedactionGoldenAndForbiddenContent (0.00s)
--- PASS: TestScrapeIssuesZeroQueries (0.00s)
--- PASS: TestConcurrentScrapesSeeIdenticalSnapshot (0.00s)
PASS
ok  	github.com/halbritt/striatum/go/pkg/metrics	0.003s
```

`go build ./...` → **PASS** (exit 0).

`go test ./cmd/striatumd ./pkg/metrics` → **PASS**:

```
ok  	github.com/halbritt/striatum/go/cmd/striatumd	0.030s
ok  	github.com/halbritt/striatum/go/pkg/metrics	0.003s
```

The repository-wide `go test ./...` has pre-existing failures in `pkg/agentloop`
and `pkg/mutations` that **reproduce at the run base commit** (`a0e5b676`) and are
unrelated to this RFC 0137 change — the reviewer confirmed the same, so they are
not introduced here and are out of Phase A scope. `git status` in the worktree is
clean and nothing outside the declared write scope
(`striatum/rfc-0137/phase-a/artifacts/`, `go/pkg/metrics/`, `go/cmd/striatumd/`)
was touched.

## Scope confirmation — loopback-only, Phase B/C/D not implemented

- **Loopback-only.** The surface binds nothing new: it mounts on the existing
  loopback MCP/web listener, and `main.go:543-557` hard-rejects any non-loopback
  MCP HTTP bind. There is no remote-write, push gateway, or wider listener.
- **Phase A only.** Only the read-path skeleton (atomic snapshot + sweep-tick
  fold + loopback `/metrics`), the three seed metrics, and the redaction harness
  were built. A source grep (reviewer-confirmed) found no Phase B/C/D families or
  enforcement surfaces (`apoptosis`, `necrosis`, `doctor_problems`,
  `metrics_allowlist`, `Classification`, `cardinality_clipped`) outside
  explanatory comments.

## Follow-ups deferred to later phases

These are intentionally **out of scope** here and define the next run's work:

- **Phase B — failure-mode taxonomy:** closed `Origin` / `*Reason` enums + the
  union guardrail test, and the `apoptosis_total` / `necrosis_total` /
  `lease_transitions_total` / `run_wedge_age_seconds` /
  `liveness_deadline_margin_seconds` / `liveness_deadline_events_total` families,
  emitted at the lifecycle-termination code sites. The RFC's hardening findings
  ride here: **F-A6** — `liveness_deadline_missed` is reversible and must live in
  the non-terminal `liveness_deadline_events_total` counter, **never** in
  `necrosis_total` (with `TestLivenessMissCanRecoverWithoutNecrosis`).
- **Phase C — contract enforcement + doctor-as-collector:** the `Classification`
  taxonomy with `Register()` refusal of `Forbidden` families, the per-family
  series budget + `cardinality_clipped_total`, the boot-time
  `metrics_allowlist.json` hash check (guardrail test + boot abort), and the
  `doctor_problems{class}` gauge sourced from **static `problem_records[*].check`
  codes only** (F-A8, with `TestDoctorClassRejectsDynamicIdentifiers`).
- **Phase D — multi-tenant hardening, consent, alert rules:** capability-scoped
  `/metrics` filtering by authorized repos, the opt-in `metrics_repo_consent`
  gauge gating `Provenance` families, the `snapshot_age_seconds` staleness SLI +
  `tick_status` publish-on-errored-tick, version-controlled Prometheus
  recording/alerting rules, and the optional cold DB-projection tier
  (`striatum metrics --once`).

The seed metrics, the package-level atomic snapshot, the sweep-tick fold, the
loopback-bound `/metrics` handler, and the golden + forbidden-regex redaction
harness are the complete, accepted Phase A surface — nothing beyond it was added.

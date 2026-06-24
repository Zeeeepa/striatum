# Phase A — read-path skeleton + redaction harness (contract-first TDD)

You are implementing **Phase A of RFC 0137** (`striatumd` Prometheus exporter).
The full spec is your required context doc:
`docs/rfcs/0137-striatumd-prometheus-exporter.md`. Read it before writing code.
**Implement Phase A only.** Do NOT build the failure-mode taxonomy
(apoptosis/necrosis), the `Classification`/`Register()` refusal, the
`metrics_allowlist.json` hash check, the `doctor_problems` collector, or any
multi-tenant/consent/alert-rules work — those are Phases B/C/D and are
out of scope for this job. A reviewer will reject scope creep.

## What you are building (RFC §Design-Sketch 1 + §Roadmap Phase A)

A new `go/pkg/metrics` package plus its wiring into `striatumd`:

1. **`MetricsSnapshot`** — an immutable value struct holding the seed Phase A
   metrics, and a package-level `atomic.Pointer[MetricsSnapshot]` published with
   `.Store()` / read with `.Load()`. Follow the exact lock-free copy-on-publish
   pattern already in `go/pkg/db/write_boundary.go:48` and
   `go/pkg/db/authority.go:29`. Reading the snapshot must take **no mutex** and
   issue **zero** DB queries.
2. **Seed metrics** (Phase A only — keep the set small but real):
   - stranded-supervisor count (supervisors `attached` to terminal runs — the
     #417 signal),
   - run-state counts (a small closed set of run states),
   - `striatum_metrics_snapshot_age_seconds` (now − snapshot `builtAt`).
   Render them as valid Prometheus text exposition (with `# HELP` / `# TYPE`).
3. **Fold + publish from the sweep tick.** Build the snapshot **once per
   resident recovery-sweep tick** and `.Store()` it at the end of the tick,
   reusing rows the tick already scanned where practical. The tick is
   `startRecoveryScheduler` → `recoverypkg.ActiveRunSweep.SweepOnce`
   (`go/cmd/striatumd/main.go:752`). Do not add a separate DB-polling loop and
   do not query PG on the scrape path.
4. **`/metrics` HTTP handler.** Mount it into the existing daemon HTTP handler
   `newDaemonHTTPHandler` (`go/cmd/striatumd/web_service.go:57`) next to
   `/v1/health`. The handler does exactly `Load()` → render text → write: no PG
   round-trip, no shared mutex. It binds on the existing loopback daemon
   listener (localhost default — do not add a new public listener).

## Contract-first: write these tests FIRST, watch them fail, then implement

This is TDD. Land the **failing** tests before the production code, then make
them green. Ship all three:

1. **Golden-file + forbidden-content redaction test** (the exfiltration
   contract — the load-bearing backstop). Build a snapshot seeded with
   deliberately distinctive **sentinel** values: a repo filesystem path, a git
   branch name, a 40-char hex sha, an argv/prompt fragment, and an `author:`
   byline. Render `/metrics` once and:
   - assert the body **byte-for-byte** against a committed golden file under
     `go/pkg/metrics/testdata/`, AND
   - assert the body does **not** match any forbidden-content regex
     (filesystem-path shapes, 40-hex-run shapes, branch-name shapes,
     prompt/argv fragments, `author:` bylines). The golden hash only catches
     changed label *names*; the regex catches a leaked *value* under an
     already-allowed name. Both must pass.
2. **Zero-DB-query test.** Drive the scrape handler with a runner whose `Query`
   (and any DB entrypoint) **panics**, and assert a scrape succeeds and issues
   zero queries — proving the scrape path never touches PG.
3. **Concurrent-scrape identity test.** Fire ~1000 concurrent scrapes against a
   fixed published snapshot and assert they observe the **identical** snapshot
   pointer (the `atomic.Pointer` is read, never recomputed per scrape).

Wire the redaction test so it runs under `make -C go check` / `make check`
(it must be part of the normal test suite, not a manual-only check).

## Constraints

- Stay strictly inside your write scope: `go/pkg/metrics/`,
  `go/cmd/striatumd/`, and your artifact dir. Do not write elsewhere or to
  `.striatum/`.
- Keep the daemon building and the full Go suite green. Verify locally with
  `make -C go build` and `go test ./pkg/metrics/...` (plus a `go build ./...`
  to prove nothing else broke). Reproduce lint if you can.
- Match the existing code's style, error handling, and package conventions.
- No new third-party deps unless unavoidable — hand-render the Prometheus text
  format (it is simple) rather than pulling the prometheus client library, to
  keep the privacy/cardinality contract enforced in our own code (consistent
  with the RFC's "enforced in code" stance). If you believe a dep is genuinely
  warranted, justify it explicitly in DRAFT.md.

## Deliverable artifact: DRAFT.md

Write `striatum/rfc-0137/phase-a/artifacts/DRAFT.md` (the required handoff
artifact). It must:
- summarize what you implemented and the files you touched,
- map each **Phase A acceptance criterion** in the RFC to the test that proves
  it (zero-PG, concurrent-scrape identity, golden+forbidden-regex redaction,
  snapshot published from the sweep tick, localhost bind),
- paste the exact commands you ran to verify and their pass/fail result
  (`make -C go build`, the `go test` runs),
- explicitly confirm you did NOT implement Phase B/C/D surface.

Do the work, prove it green, then publish DRAFT.md and complete the job.

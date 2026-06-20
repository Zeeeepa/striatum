# Review — RFC 0137 Phase A implementation

You are the independent reviewer (different model family from the author). Review
the Phase A implementation of the `striatumd` Prometheus exporter against the
RFC's Phase A scope and acceptance criteria. Your required context doc is the
spec: `docs/rfcs/0137-striatumd-prometheus-exporter.md`. The author's summary is
`striatum/rfc-0137/phase-a/artifacts/DRAFT.md`.

**Verify against the source, not the prose.** Read the actual changed files
(`go/pkg/metrics/`, `go/cmd/striatumd/web_service.go`,
`go/cmd/striatumd/main.go`) and the tests. To avoid a no-tool-progress stall,
keep making tool calls (read files, run `go test`/`go build`) as you work —
do not go silent for long stretches.

## Acceptance checklist (RFC §Acceptance + §Roadmap Phase A)

1. **Zero-PG, lock-disjoint scrape.** `/metrics` serves valid Prometheus text
   with zero PG queries and zero shared-mutex acquisition at scrape time.
   Confirm the panic-on-query test and the ~1000-way concurrent-scrape identity
   test exist and genuinely prove this (the handler does `Load()` → render →
   write only).
2. **Redaction contract is real and fails closed.** The golden-file +
   forbidden-content-regex redaction test exists, is seeded with distinctive
   sentinel values (path, branch, 40-hex sha, argv/prompt fragment, `author:`
   byline), asserts the body byte-for-byte against a committed golden, AND
   rejects any forbidden-content regex match. Sanity-check that it would
   actually fail if a sentinel leaked (e.g. the regex set is non-trivial and the
   golden is exact). Confirm it is wired into `make check` / `go test`, not
   manual-only.
3. **Snapshot read path.** `MetricsSnapshot` is immutable and published via a
   package-level `atomic.Pointer` (`.Store()`/`.Load()`, copy-on-publish like
   `db/write_boundary.go`), folded once per recovery-sweep tick — not recomputed
   per scrape, not via a new DB-polling loop.
4. **Wiring + bind.** `/metrics` is mounted in `newDaemonHTTPHandler` next to
   `/v1/health`; the surface stays on the loopback daemon listener (no new
   public listener in Phase A).
5. **No scope creep.** Phase B/C/D surface (apoptosis/necrosis taxonomy,
   `Classification`/`Register()` refusal, `metrics_allowlist.json` hash check,
   `doctor_problems` collector, capability-scoping/consent/alert rules) is NOT
   present. Phase A is a clean, shippable slice.
6. **Green + clean.** `make -C go build` and the metrics tests pass; nothing
   else in the suite is broken; the change stays inside the declared write
   scope.

## Verdict

Record a finding artifact at the declared path with a single verdict:
- **accept** — all six hold; the slice is correct, tested, and in-scope.
- **needs_revision** — cite each concrete defect (file/line, which criterion it
  violates, and what must change). Be specific and actionable so one revision
  cycle can clear it. Reserve `needs_revision` for real correctness/contract
  gaps, not style preferences.

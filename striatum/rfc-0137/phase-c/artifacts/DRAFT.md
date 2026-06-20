# RFC 0137 Phase C — contract enforcement + doctor-as-collector (DRAFT)

author: author-author-002

Phase C builds on the landed Phase A read path (snapshot / render / `/metrics`)
and the Phase B failure-mode taxonomy. It adds the four enforcement deliverables
the RFC §Design-Sketch 2 + §Roadmap Phase C call for: a family Classification
with a `Register()` refusal of `Forbidden`, a per-family series budget with a
clip counter, a boot-time `metrics_allowlist.json` hash check that aborts daemon
startup on drift, and a `striatum_doctor_problems{class}` collector pinned to the
**static** `problem_records[*].check` codes (F-A8). Phase D was deliberately not
implemented (see the last section).

Everything is green: `make -C go build`, `go build ./...`, `go vet`, and
`go test -race ./pkg/metrics/...` all pass (commands + output at the end).

## Files touched

All within the declared write scope (`go/pkg/metrics/`, `go/cmd/striatumd/`).

New:
- `go/pkg/metrics/registry.go` — `Classification` enum, `Family`, `Registry`,
  `Register()`/`MustRegister()` refusal, `DefaultRegistry()`, canonical `Hash()`.
- `go/pkg/metrics/budget.go` — `applySeriesBudget` (per-family distinct-key cap)
  + the reserved `other` bucket constant.
- `go/pkg/metrics/doctor.go` — `extractDoctorProblemRecords` (reads only
  `problem_records`) + `foldDoctorProblemRecords` (class from static `check`
  only, with a shape sanitizer); the F-A8 contract lives entirely here.
- `go/pkg/metrics/allowlist.go` — embedded `metrics_allowlist.json`,
  `BuildAllowlist`/`MarshalAllowlist`/`LoadEmbeddedAllowlist`, and
  `VerifyAllowlist()` (boot guardrail).
- `go/pkg/metrics/metrics_allowlist.json` — the checked-in, diff-reviewed
  manifest (committed under `go/pkg/metrics/` and embedded into the binary).
- Tests: `registry_test.go`, `budget_test.go`, `allowlist_test.go`,
  `doctor_test.go`.

Modified:
- `go/pkg/metrics/snapshot.go` — `SnapshotInput.DoctorProblemRecords`; the
  `doctorProblems` / `cardinalityClipped` snapshot maps; fold + budget in
  `Build`; `sortedStringKeys` helper.
- `go/pkg/metrics/render.go` — two new metric-name constants and
  `writeDoctorProblems` / `writeCardinalityClipped` render blocks.
- `go/pkg/metrics/collector.go` — `Refresh` now folds doctor problems on the
  bounded sweep cadence via `doctorProblemRecords` + `activeRepositoryIDs`
  (imports `db`, `reads`, `rpc`).
- `go/pkg/metrics/redaction_test.go` — `sentinelDoctorRecords` seeded into the
  golden so the redaction contract covers the new render path.
- `go/pkg/metrics/testdata/metrics_golden.txt` — regenerated for the two new
  families.
- `go/cmd/striatumd/main.go` — `metrics.VerifyAllowlist()` fatal check before
  the `/metrics` listener binds.

No import cycle: `go/pkg/metrics` now imports `go/pkg/reads` (and `db`/`rpc`),
read-only; `reads` does not import `metrics` (verified with `go list -deps`).

## 1. Classification + `Register()` refusal

`registry.go` defines `Classification` (`operational` | `provenance` |
`forbidden`) and a `Family{Name, Type, Classification, Labels}`. The `Registry`
is the single source of truth for the closed set of families and label *names*
the exporter may emit.

`Register()` **refuses** a `Forbidden`-classified family at construction
(returning an error) — and also refuses an empty name, an unknown
classification, and a duplicate name. `MustRegister()` is the panic form: it is
how `DefaultRegistry()` is built, so a Forbidden family added there panics in
tests and aborts the daemon boot in prod (a forbidden series can never reach the
wire). Every Phase A–C family is `Operational`; `Provenance` / consent-gated
families are Phase D.

Proof: `TestRegisterRefusesForbiddenFamily` asserts `Register` errors,
`MustRegister` panics, and the Forbidden family never enters `Specs()`.
`TestRegisterRefusesUnknownClassificationAndDuplicate` covers the other refusals.

## 2. Per-family series budget + `striatum_metrics_cardinality_clipped_total`

`budget.go`'s `applySeriesBudget(counts, limit, other)` keeps the `limit`
lexicographically-smallest distinct keys and collapses the rest onto the reserved
`other` bucket, returning the number of distinct keys clipped. Emitted series are
therefore bounded at `limit+1` regardless of how many distinct tuples arrive — so
neither the daemon registry nor a downstream Prometheus can be ID-bombed into
OOM. The clip count renders as
`striatum_metrics_cardinality_clipped_total{family="doctor_problems"}`, which is
itself alertable (silent dimension loss is made visible).

The budget is applied to `doctor_problems` (the one Phase C family whose label
domain is not a compile-time-closed enum). The closed-enum families
(apoptosis/necrosis/lease/liveness/wedge/margin) are structurally bounded by
their CREATE-defined Go enums, so the budget is a no-op for them; `budget.go` is
generic and reusable for future open-domain families.

**Design note (LRU vs deterministic cap).** The RFC describes an "LRU budget".
The metrics snapshot is rebuilt from durable state every tick, so there is no
cross-tick series recency to LRU over; a deterministic *sorted* cap delivers the
same hard OOM bound, is byte-stable for the golden, and needs no persistent
collector state to survive a restart. That is the realization here, documented so
the reviewer sees it is a deliberate choice, not an oversight.

Proof: `TestSeriesBudgetClipsOverflow` (N+1 distinct → one `other` + clip == 1,
series bounded at N+1), `TestSeriesBudgetNoClipUnderLimit`, and
`TestDoctorProblemsBudgetClipsThroughBuild` (end-to-end through `Build`/render).

## 3. Boot-time `metrics_allowlist.json` hash check (guardrail + boot abort)

`Registry.Hash()` SHA-256s the canonical `(name, type, classification, sorted
label names)` serialization of every family. `metrics_allowlist.json` commits
that hash plus the family list (so a manifest change is a reviewable diff) and is
embedded into the binary with `//go:embed`, so the boot check is self-contained
(no source tree at runtime).

- **Boot abort:** `main.go` calls `metrics.VerifyAllowlist()` *before*
  `startMCPHTTPServer` mounts `/metrics`; a hash mismatch is `fatalf` — the
  daemon refuses to start with an unreviewed series.
- **Guardrail (CI):** `TestMetricsAllowlistMatchesRegistry` recomputes from the
  live registry and asserts the embedded manifest matches (families + hash).
  Regenerate deliberately with `STRIATUM_UPDATE_ALLOWLIST=1 go test
  ./pkg/metrics/...`.

This mirrors the existing redaction-golden precedent in the same package
(`STRIATUM_UPDATE_GOLDEN`) and the generated-route / error-catalog guardrail
pattern. It is already wired into `make check`: `make check` →
(`go/Makefile`) `check-tests` → `go test -race -count=1 ... ./...`, which runs
`./pkg/metrics/` and therefore this guardrail (and the redaction golden and the
F-A8 test). No Makefile edit is required.

Proof: `TestMetricsAllowlistMatchesRegistry` (guardrail) and
`TestVerifyAllowlistDetectsDrift` (live registry verifies clean; a registry with
one extra family fails the same check the daemon runs at boot).

## 4. `striatum_doctor_problems{class}` — collector from the doctor checks

`collector.go`'s `Refresh` folds `doctor_problems` on the **bounded sweep
cadence**, never on the scrape path: `doctorProblemRecords` enumerates the active
repositories (`activeRepositoryIDs`, selecting only `repository_id`) and calls
`reads.HandleDoctor(..., verbose=true)` per repo inside a 15s timeout, then reads
`result["problem_records"]`. It is best-effort like the Phase B folds — a doctor
error degrades the gauge to empty for that snapshot rather than failing the whole
surface. The scrape path is unchanged (still `Load → render → write`, proven by
the existing `TestScrapeIssuesZeroQueries`).

### F-A8 (load-bearing): class is sourced ONLY from static check codes

The `class` label is derived in exactly one place (`doctor.go`):

- `extractDoctorProblemRecords` reads **only** `result["problem_records"]` —
  **never** `result["problems"]`, whose strings interpolate dynamic ids in their
  prefix (`run_needs_operator.<run_id>`, `supervisor_liveness.<supervisor_id>`,
  `recovery_sweep_cursor_latch_error.<run_id>`). A dynamic id as a label value
  would re-open the A3 value-leak and A5 cardinality holes.
- `foldDoctorProblemRecords` counts by each record's static `check` field. A
  second wall behind that: a `check` value that is not a static snake_case code
  (`^[a-z][a-z0-9_]*$`) is bucketed to `other`, so even a future check that
  accidentally interpolated an id into `check` cannot leak it.
- The per-family series budget (deliverable #2) is the third wall, capping the
  emitted `class` series.

Because the check codes are static, the series count tracks the number of
distinct failing check *types* (a small bounded set), not the number of failing
runs/gates/supervisors.

### F-A8 test — `TestDoctorClassRejectsDynamicIdentifiers`

Seeds quorum / recovery / fan-in / artifact-anchor failures with adversarial
run/supervisor/gate ids (each carrying a distinctive `Z9LEAKSENTINEL` token) in
`problem_records` **and** a populated dynamic-id `problems` summary list, renders
`/metrics`, and asserts:

- (a) only the static `problem_records[*].check` codes appear as `class`
  (one series per check, with the seeded count);
- (b) no dynamic id, no 40-hex run, and no `problems` summary string leaks into
  the body (`Z9LEAKSENTINEL` / `deadbeef` / `: adversarial failure` all absent);
- (c) the `doctor_problems` series count is **constant** whether 1 or 25
  entities are failing per check — only the per-class value scales.

Supporting tests: `TestExtractDoctorProblemRecordsIgnoresProblemsList` (a result
with only a dynamic-id `problems` list yields no records/series) and
`TestFoldDoctorSanitizesNonStaticCheck` (id-bearing / slash-shaped `check` values
bucket to `other`). The redaction golden also now seeds `sentinelDoctorRecords`
with sensitive id fields, so the byte-for-byte golden + forbidden-content regex
prove the doctor render path leaks nothing.

## Acceptance criteria → test mapping

| Deliverable / criterion | Test(s) |
| --- | --- |
| #1 `Forbidden` family refused at `Register()` | `TestRegisterRefusesForbiddenFamily`, `TestRegisterRefusesUnknownClassificationAndDuplicate` |
| #2 series budget + clip counter (N+1 → one `other` + clip) | `TestSeriesBudgetClipsOverflow`, `TestSeriesBudgetNoClipUnderLimit`, `TestDoctorProblemsBudgetClipsThroughBuild` |
| #3 allowlist hash matches manifest (guardrail + boot abort) | `TestMetricsAllowlistMatchesRegistry`, `TestVerifyAllowlistDetectsDrift` |
| #4 `doctor_problems{class}` from static `problem_records[*].check`, bounded cadence | `collector.go` `Refresh`/`doctorProblemRecords`; folded in `Build`; `TestFoldDoctorSanitizesNonStaticCheck` |
| #5 F-A8 adversarial-id test | `TestDoctorClassRejectsDynamicIdentifiers`, `TestExtractDoctorProblemRecordsIgnoresProblemsList` |
| Scrape stays zero-DB / lock-disjoint (unchanged) | `TestScrapeIssuesZeroQueries`, `TestConcurrentScrapesSeeIdenticalSnapshot` |
| Redaction contract incl. new families | `TestMetricsRedactionGoldenAndForbiddenContent` |
| Every registered family is actually rendered | `TestDefaultRegistryIsOperationalAndStable` |

## Verification commands + results

Run from `go/` in the per-job worktree:

```
make -C go build            # → bin/striatum, bin/striatumd, bin/striatum-supervisor-helper  (exit 0)
go build ./...              # exit 0
go vet ./pkg/metrics/... ./cmd/striatumd/...   # exit 0
go vet ./pkg/mutations/... ./pkg/reads/...     # exit 0 (necrosis guardrail consumer still compiles)
go test -race -count=1 ./pkg/metrics/...       # ok  github.com/halbritt/striatum/go/pkg/metrics  ~1.0s
```

The redaction golden was regenerated deliberately
(`STRIATUM_UPDATE_GOLDEN=1`) for the two new families; the forbidden-content
regex still passes. The `metrics_allowlist.json` hash was generated with
`STRIATUM_UPDATE_ALLOWLIST=1` and matches the live registry
(`sha256 78b89687…`). `./pkg/metrics` is not in `COVERAGE_PKGS`, so the
`make check` coverage floor is unaffected.

## Phase D was NOT implemented (explicit confirmation)

None of the following Phase D items were built (a reviewer rejects scope creep):
capability-scoped `/metrics` filtering by authorized repos; the
`metrics_repo_consent` gauge / `Provenance` consent gating; the `snapshot_age`
staleness SLI alert / publish-on-errored-tick `tick_status` label; version-
controlled Prometheus recording/alert rules; and the cold DB-projection tier.
The `Classification` enum *defines* `Provenance` so Phase D can register
consent-gated families later, but no `Provenance` or `Forbidden` family is
registered now — every Phase A–C family is `Operational`.

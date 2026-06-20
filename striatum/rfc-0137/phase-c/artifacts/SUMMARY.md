---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs:
  - "striatum/rfc-0137/phase-c/artifacts/DRAFT.md"
  - "striatum/rfc-0137/phase-c/artifacts/review/REVIEW.md"
  - "striatum/rfc-0137/phase-c/artifacts/OPERATOR_DECISION.md"
  - "docs/rfcs/0137-striatumd-prometheus-exporter.md"
---

# RFC 0137 Phase C — contract enforcement + doctor-as-collector (final summary)

author: author-author-003

Phase C of RFC 0137 (`striatumd` Prometheus exporter) is **implemented,
operator-accepted, and re-confirmed green**. It builds the cardinality/privacy
contract *enforcement* layer on top of the landed Phase A read path (snapshot /
render / `/metrics`) and the Phase B failure-mode taxonomy. Four enforcement
deliverables from RFC §"Design Sketch 2" + §"Roadmap Phase C" are present and
non-stub: a family `Classification` with a `Register()` refusal of `Forbidden`,
a per-family series budget with an alertable clip counter, a boot-time
`metrics_allowlist.json` hash check that aborts daemon startup on drift, and a
`striatum_doctor_problems{class}` collector pinned to the **static**
`problem_records[*].check` codes (F-A8). Phase D was deliberately **not**
implemented (scope confirmation at the end).

## Review / acceptance posture

The reviewer (`reviewer-reviewer-002`) returned **needs_revision** twice
(bounded cycle exhausted) on one residual finding — that
`striatum_metrics_cardinality_clipped_total` carries `counter` TYPE but
per-snapshot (rebuilt-each-tick) semantics. The operator adjudicated the
reviewer-author disagreement in `OPERATOR_DECISION.md`
(`dec_rfc0137_phasec_accept`, outcome **accepted**): independent verification
(source read + `go build ./...` + `go test ./pkg/metrics/...` green) confirmed
every RFC Phase C deliverable present, correctly wired, and non-stub, and the
residual clip-counter finding is **superseded by the operator decision**. Per
that adjudication this apply pass made **no implementation change** — it
re-confirmed green, verified each in-scope contract guard the prompt calls out,
and produced this synthesis. (The clip-counter monotonicity question is noted
below as a Phase D follow-up so it is not silently dropped.)

## Final file list (Phase C diff vs the Phase B baseline)

All within the declared write scope (`go/pkg/metrics/`, `go/cmd/striatumd/`).

| File | Change | Role |
| --- | --- | --- |
| `go/pkg/metrics/registry.go` | **new** | `Classification` enum (`operational`/`provenance`/`forbidden`), `Family{Name,Type,Classification,Labels}`, `Registry`, `Register()`/`MustRegister()` with the `Forbidden` refusal, `DefaultRegistry()`, canonical `Hash()`. |
| `go/pkg/metrics/registry_test.go` | **new** | `TestRegisterRefusesForbiddenFamily`, `TestRegisterRefusesUnknownClassificationAndDuplicate`, `TestDefaultRegistryIsOperationalAndStable`. |
| `go/pkg/metrics/budget.go` | **new** | `applySeriesBudget(counts, limit, other)` — per-family distinct-key cap collapsing overflow onto the reserved `other` bucket; returns the clip count. |
| `go/pkg/metrics/budget_test.go` | **new** | `TestSeriesBudgetClipsOverflow`, `TestSeriesBudgetNoClipUnderLimit`, `TestDoctorProblemsBudgetClipsThroughBuild`. |
| `go/pkg/metrics/doctor.go` | **new** | `extractDoctorProblemRecords` (reads **only** `problem_records`) + `foldDoctorProblemRecords` (class from the static `check` field, with a snake_case shape sanitizer). The F-A8 contract lives entirely here. |
| `go/pkg/metrics/doctor_test.go` | **new** | `TestDoctorClassRejectsDynamicIdentifiers` (F-A8), `TestExtractDoctorProblemRecordsIgnoresProblemsList`, `TestFoldDoctorSanitizesNonStaticCheck`. |
| `go/pkg/metrics/allowlist.go` | **new** | `//go:embed`-ed `metrics_allowlist.json`; `BuildAllowlist`/`MarshalAllowlist`/`LoadEmbeddedAllowlist`; `VerifyAllowlist()` boot guardrail. |
| `go/pkg/metrics/allowlist_test.go` | **new** | `TestMetricsAllowlistMatchesRegistry` (guardrail), `TestVerifyAllowlistDetectsDrift`. |
| `go/pkg/metrics/metrics_allowlist.json` | **new** | The checked-in, diff-reviewed manifest (family list + `sha256` `78b89687…`), embedded into the binary. |
| `go/pkg/metrics/snapshot.go` | modified | `SnapshotInput.DoctorProblemRecords`; the `doctorProblems` / `cardinalityClipped` snapshot maps; fold + budget applied in `Build`; `sortedStringKeys` helper. |
| `go/pkg/metrics/render.go` | modified | Two new metric-name constants + `writeDoctorProblems` / `writeCardinalityClipped` render blocks. |
| `go/pkg/metrics/collector.go` | modified | `Refresh` now folds doctor problems on the bounded sweep cadence via `doctorProblemRecords` + `activeRepositoryIDs` (imports `db`, `reads`, `rpc`, read-only). |
| `go/pkg/metrics/redaction_test.go` | modified | `sentinelDoctorRecords` seeded into the golden so the redaction contract covers the new doctor render path. |
| `go/pkg/metrics/testdata/metrics_golden.txt` | modified | Regenerated for the two new families; forbidden-content regex re-verified. |
| `go/cmd/striatumd/main.go` | modified | `metrics.VerifyAllowlist()` fatal check at `main.go:320`, before the `/metrics` listener binds. |

No import cycle: `go/pkg/metrics` imports `go/pkg/reads` (and `db`/`rpc`)
read-only; `reads` does not import `metrics` (proven by `go build ./...`).

## How the cardinality/privacy contract is now enforced — in code + CI

The Phase A/B redaction golden proves *no forbidden value* reaches the wire;
Phase C adds the three structural walls that make the contract **fail closed**
rather than promised in prose.

1. **`Register()` refuses `Forbidden` at construction (a forbidden series can
   never exist).** `registry.go` makes the `Registry` the single source of truth
   for the closed set of emittable families and label *names*. `Register()`
   returns an error for a `Forbidden` classification (and for an empty name,
   unknown classification, or duplicate); `MustRegister()` is the panic form
   that builds `DefaultRegistry()`, so a `Forbidden` family added there panics in
   tests and hard-aborts the daemon boot in prod. Every Phase A–C family is
   `Operational`. **CI:** `TestRegisterRefusesForbiddenFamily` (PASS).

2. **Per-family series budget + `striatum_metrics_cardinality_clipped_total`.**
   `applySeriesBudget` keeps the `limit` lexicographically-smallest distinct keys
   and collapses the rest onto the reserved `other` bucket, bounding emitted
   series at `limit+1` regardless of how many distinct tuples arrive — so neither
   the daemon registry nor a downstream Prometheus can be ID-bombed into OOM. The
   clip count renders as
   `striatum_metrics_cardinality_clipped_total{family="doctor_problems"}` and is
   itself alertable (silent dimension loss is visible). It is applied to
   `doctor_problems` (the one Phase C family whose label domain is not a
   compile-time-closed enum); the closed-enum families are structurally bounded
   already, so the budget is a deliberate generic no-op for them. **CI:**
   `TestSeriesBudgetClipsOverflow`, `TestSeriesBudgetNoClipUnderLimit`,
   `TestDoctorProblemsBudgetClipsThroughBuild` (all PASS).

3. **Boot-time `metrics_allowlist.json` hash check (guardrail + boot abort).**
   `Registry.Hash()` SHA-256s the canonical `(name, type, classification, sorted
   label names)` serialization of every family. The committed manifest
   (`sha256 78b89687e846ca38f96f0254b095cb04153bca4abf2bd48e93976efd24aa1abd`) is
   `//go:embed`-ed so the check is self-contained at runtime. `main.go:320` calls
   `metrics.VerifyAllowlist()` **before** the `/metrics` listener binds; a hash
   mismatch is `fatalf` — the daemon refuses to start with an unreviewed series,
   so adding/renaming a label becomes a deliberate, diff-reviewed manifest edit
   (regenerate with `STRIATUM_UPDATE_ALLOWLIST=1 go test ./pkg/metrics/...`).
   **CI:** `TestMetricsAllowlistMatchesRegistry` + `TestVerifyAllowlistDetectsDrift`
   (PASS) exercise the same `VerifyAllowlist()` path the daemon runs at boot.

These three are wired into `make check` with **no Makefile edit**:
`make check` → `check-tests` → `go test -race -count=1 -timeout 30m ... ./...`
(`go/Makefile:76-77`) runs `./pkg/metrics/`, and therefore the allowlist
guardrail, the F-A8 test, and the redaction golden, on every `make check`.

## How `doctor_problems{class}` is pinned to static codes (F-A8)

The `class` label is derived in exactly one place — `doctor.go` — behind three
walls, so a dynamic id (which would re-open the A3 value-leak and A5 cardinality
holes) cannot reach the wire:

1. **Source restriction.** `extractDoctorProblemRecords` reads **only**
   `result["problem_records"]`, **never** `result["problems"]`, whose strings
   interpolate dynamic ids in their prefix (`run_needs_operator.<run_id>`,
   `supervisor_liveness.<supervisor_id>`,
   `recovery_sweep_cursor_latch_error.<run_id>`).
2. **Shape sanitizer.** `foldDoctorProblemRecords` counts by each record's static
   `check` field; any `check` that is not a static snake_case code
   (`^[a-z][a-z0-9_]*$`) buckets to `other`, so even a future check that
   accidentally interpolated an id into `check` cannot leak it.
3. **Series budget.** Deliverable #2 caps the emitted `class` series as the third
   wall.

Because the check codes are static, the series count tracks the number of
distinct failing check *types* (a small bounded set), not the number of failing
runs/gates/supervisors. The collector folds `doctor_problems` on the **bounded
sweep cadence** (`collector.go` `Refresh` → `doctorProblemRecords` →
`reads.HandleDoctor(..., verbose=true)` per active repo inside a 15s timeout),
never on the scrape path — the scrape stays `Load → render → write`, still
zero-DB-query and lock-disjoint. **CI:** `TestDoctorClassRejectsDynamicIdentifiers`
seeds quorum/recovery/fan-in/artifact-anchor failures with adversarial
run/supervisor/gate ids (each carrying a `Z9LEAKSENTINEL` token) plus a populated
dynamic-id `problems` summary, then asserts (a) only static `check` codes appear
as `class`, (b) no dynamic id / 40-hex run / `problems` string leaks into the
body, and (c) the `doctor_problems` series count is **constant** whether 1 or 25
entities fail per check (PASS), backed by
`TestExtractDoctorProblemRecordsIgnoresProblemsList` and
`TestFoldDoctorSanitizesNonStaticCheck` (PASS).

## Acceptance-criteria → test mapping (with this-pass verification results)

All commands run from the per-job worktree `go/` directory on
`striatum/rfc0137-phase-c` (HEAD `18a3f221`).

| Deliverable / RFC criterion | Proving test(s) | Result this pass |
| --- | --- | --- |
| #1 `Forbidden` family refused at `Register()` (panic via `MustRegister`/`DefaultRegistry`) | `TestRegisterRefusesForbiddenFamily`, `TestRegisterRefusesUnknownClassificationAndDuplicate` | **PASS** |
| #2 series budget + clip counter (N+1 distinct → one `other` + clip == 1, series bounded) | `TestSeriesBudgetClipsOverflow`, `TestSeriesBudgetNoClipUnderLimit`, `TestDoctorProblemsBudgetClipsThroughBuild` | **PASS** |
| #3 allowlist hash matches manifest; boot abort on drift | `TestMetricsAllowlistMatchesRegistry`, `TestVerifyAllowlistDetectsDrift` (same `VerifyAllowlist()` as `main.go:320`) | **PASS** |
| #4 `doctor_problems{class}` from static `problem_records[*].check`, bounded cadence | `collector.go` `Refresh`/`doctorProblemRecords`; `TestFoldDoctorSanitizesNonStaticCheck` | **PASS** |
| #5 F-A8 adversarial-id test (no dynamic id leaks; series count constant) | `TestDoctorClassRejectsDynamicIdentifiers`, `TestExtractDoctorProblemRecordsIgnoresProblemsList` | **PASS** |
| Scrape stays O(1) / zero-DB / lock-disjoint (Phase A invariant unchanged) | `TestScrapeIssuesZeroQueries`, `TestConcurrentScrapesSeeIdenticalSnapshot` | **PASS** |
| Exfiltration contract incl. the two new families | `TestMetricsRedactionGoldenAndForbiddenContent` | **PASS** |
| Every registered family is actually rendered | `TestDefaultRegistryIsOperationalAndStable` | **PASS** |

### Verification commands + results (this apply pass)

Run from the per-job worktree (`.striatum/worktrees/…`, HEAD `18a3f221`):

```
make -C go build                                  # exit 0 — striatum, striatumd, supervisor-helper
go test ./pkg/metrics/...                          # ok  github.com/halbritt/striatum/go/pkg/metrics  ~0.009s
go build ./...                                     # exit 0 — confirms no metrics↔reads import cycle
go test ./pkg/metrics/... -run \
  'TestRegisterRefusesForbiddenFamily|TestSeriesBudget*|TestDoctorProblemsBudgetClipsThroughBuild|\
   TestMetricsAllowlistMatchesRegistry|TestVerifyAllowlistDetectsDrift|\
   TestDoctorClassRejectsDynamicIdentifiers|TestExtractDoctorProblemRecordsIgnoresProblemsList|\
   TestFoldDoctorSanitizesNonStaticCheck|TestMetricsRedactionGoldenAndForbiddenContent|\
   TestDefaultRegistryIsOperationalAndStable' -v                # 11/11 PASS
```

`make check` wiring confirmed by inspection: `go/Makefile:88` `check: verify vet
lint check-tests` and `go/Makefile:76-77` `check-tests:` →
`go test -race -count=1 ... ./...` runs `./pkg/metrics/`, so the allowlist
guardrail, the F-A8 test, and the redaction golden all gate `make check`.
`./pkg/metrics` is not in `COVERAGE_PKGS`, so the coverage floor is unaffected.

## Phase D was NOT implemented (explicit scope confirmation)

Confirmed by `go build ./...` and the diff — no Phase D work is present:
no capability-scoped `/metrics` filtering, no `metrics_repo_consent` gauge /
`Provenance` consent gating, no `snapshot_age` staleness SLI / errored-tick
`tick_status` label, no bundled Prometheus recording/alert rules, and no cold
DB-projection tier. The `Classification` enum *defines* `Provenance` so Phase D
can register consent-gated families later, but no `Provenance` or `Forbidden`
family is registered now — every Phase A–C family is `Operational`, and the
bind stays loopback-only (inherited from Phase A).

## Follow-ups left for Phase D (next run's scope)

RFC §"Roadmap Phase D" — multi-tenant hardening, consent, alert rules:

1. **Capability-scoped `/metrics` filtering** by authorized repos (RFC 0043
   per-repo capability), so a tailnet scraper holding only repo-A's token cannot
   see repo-B's surrogate buckets.
2. **Opt-in `striatum_metrics_repo_consent{bucket}` gauge** gating
   `Provenance`-classified families per repo (persisted product-decision flag);
   `Operational` defaults on, `Provenance` defaults off.
3. **`snapshot_age_seconds` staleness SLI alert + publish-on-errored-tick
   `tick_status` label** (RFC OQ1 — a wedged reconcile loop must not let scrapers
   serve last-good numbers silently).
4. **Version-controlled Prometheus recording + alerting rules** next to the
   exporter (`NecrosisRate`, `DoctorRed`, `WedgeAgeTail`, `LivenessMarginCollapse`,
   `SupervisorOriginFlood`).
5. **Optional cold DB-projection tier** (`striatum metrics --once`) for federation
   and as an out-of-band liveness check on the in-process tier (RFC OQ1/OQ3).

Additionally carried forward from the bounded review cycle (operator-superseded,
not a Phase C blocker): **clip-counter monotonicity** — the reviewer noted
`striatum_metrics_cardinality_clipped_total` has `counter` TYPE but is rebuilt
from the current tick's input each refresh, so a later tick with fewer
over-budget classes can lower the sample. The contract-aligned resolution
(monotonic per-family accumulation across refreshes with a no-decrease
regression test, or a deliberate gauge re-type updating the registry, allowlist,
golden, and RFC wording) belongs to the Phase D alerting/rules slice that makes
the clip signal operationally load-bearing.

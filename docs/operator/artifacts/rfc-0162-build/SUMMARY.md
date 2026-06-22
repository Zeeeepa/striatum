---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
inputs:
  - "docs/operator/artifacts/rfc-0162-build/DRAFT.md"
  - "docs/operator/artifacts/rfc-0162-build/review/REVIEW.md"
---

# SUMMARY — RFC 0162 lane-auth silent-failure observability MVP (build run, finalized)

author: author-author-002

> Finalization of the `rfc-0162-build` run. The reviewer verdict was
> **`accept_with_findings`** (low severity, *"No revision is required to
> proceed"*). This apply stage addressed the two actionable in-scope nits, recorded
> the three info-level findings for the follow-up, and re-verified the tree green.
> The deliverable is the **real Go source** built test-first to the
> falsification-cleared `PROPOSAL.md` (build-order steps 1–5; step 6 / Layer 2 is
> OUT). The live game-day fire tests (GD-1…GD-5) are the **`rfc-0162-verify`** run's
> job — this run builds the surface so they *can* fire.

## 1. Files changed (one-line role each)

**`go/pkg/laneproviderauth/` — the F2 resolver + roster + expiry + sampler (pure, leaf; `lane_provider_auth.go` UNTOUCHED):**
- `roster.go` — `RosterEntry`/`Roster`, `LoadRoster`/`ParseRoster`, the `{oauth,api_key}` closed kind enum, `DefaultRosterPath()`, and the OQ4 `Effective{Staleness,ExpiryLead}Seconds()` (declared value, else a default from the **declared** cadence — never observed lifetime).
- `resolver.go` — the **F2 fail-closed credential-resolver contract** `ResolveCredential` + the exported `ErrResolverMismatch` sentinel (codex `$CODEX_HOME/auth.json`→`$HOME/.codex/…`; claude `$CLAUDE_CONFIG_DIR/.credentials.json`→`$HOME/.claude/…`; config-dir key wins over a HOME decoy).
- `expiry.go` — provider-specific `ParseExpiry` (codex JWT `exp`/`expiry`; claude `expiresAt` ms; `api_key`→`HasExpiry=false`; unparseable OAuth→fail-closed).
- `sampler.go` — `SampleLaneCredential` (resolve→read→parse) + `ErrCredentialAbsent`; distinguishes **absent** (census) from **unprovable** (page); `LaneFileReader` is the prod sudo-`-n`-as-lane reader.
- `rfc0162_test.go` — the F2/FA-4/FA-F2 named tests.

**`go/pkg/metrics/` — the eight families + folds + render + alerts:**
- `lane_auth.go` — fold types + `foldLaneAuth` + the per-family **lane series budget** (≤32 lanes, overflow→`lane="other"`, clip counted) + deterministic sort + label sanitizer.
- `lane_auth_fold.go` — Collector-side best-effort folds: `laneAuthRosterObservations`, `sampleLaneCredentials` (Layer 1, per-lane timeout, no DB), `(*Collector).laneAuthSuccessObservations` (Layer 3 `MAX(created_at)` per lane). **Apply-stage refinement here:** F-1 — `isResolverMismatch` now uses `errors.Is(err, laneproviderauth.ErrResolverMismatch)` instead of a message substring; F-4 — softened the `laneLaunchEnv` comment so it no longer over-claims a live-process read.
- `lane_auth_test.go` — the metrics fold named tests.
- `registry.go` — registers the 8 families in `DefaultRegistry()` (all `ClassificationOperational`, closed label sets).
- `render.go` — metric-name constants + `writeLaneAuth` (HELP/TYPE always emitted; closed labels only).
- `snapshot.go` / `semantics.go` / `collector.go` — `SnapshotInput` fields + `foldLaneAuth` call in `Build`; the 8 declared `SeriesSnapshot` (gauges); `Refresh` wires the three best-effort folds after the load-bearing Phase A folds.
- `metrics_allowlist.json` / `testdata/metrics_golden.txt` — **regenerated** (+8 families / +8 HELP/TYPE blocks; no other family drifted).
- `rules/alerting_rules.yml` — the six MVP alerts appended.

**`go/pkg/mutations/` — the Layer 3 heartbeat write:**
- `supervision_provider_auth.go` — emits `lane.auth_success` at the gate success branch (`if result.Passed()`), best-effort via `emitLaneAuthSuccessEvent` in its own short tx; **swallows errors** (FA-7); codex-scoped by construction.
- `supervision_control.go` — threads `runner` into `runSuperviseProviderAuthGate`.
- `supervision_provider_auth_rfc0162_test.go` — FA-5 / FA-7 named tests.

**`go/pkg/reads/` — the doctor reconciliation logic:**
- `doctor_lane_auth_roster.go` — `ReconcileLaneAuthRoster` (pure) → static check codes `lane_auth_missing_sample` / `lane_auth_unrostered_live_lane` / `lane_auth_resolver_mismatch`; `LaneAuthLiveLanesFromEvents`.
- `doctor_lane_auth_roster_test.go` — reconciliation tests.

## 2. Build-step discharge (PROPOSAL build-order 1–5)

| Step | Metric family / event / fold | Code site (file:symbol) | FA-* test that proves it |
| --- | --- | --- | --- |
| **1. Backbone roster + fold** | `striatum_lane_auth_expected{lane,provider,kind}`, `…_auth_staleness_threshold_seconds{lane}`, `…_cred_expiry_lead_seconds{lane}`; source = roster file (no event); fold = `laneAuthRosterObservations`→`foldLaneAuth` | `laneproviderauth/roster.go:LoadRoster`,`Effective*Seconds`; `metrics/lane_auth_fold.go:laneAuthRosterObservations`; `metrics/registry.go:174-181`; `reads/doctor_lane_auth_roster.go:ReconcileLaneAuthRoster` | `TestThresholdFromRosterNotObserved` (FA-4), `TestLaneAuthSeriesMissingNamesMissingLane` (FA-F1), `TestReconcileLaneAuthRoster` |
| **2. F2 resolver contract** | `striatum_lane_cred_resolver_mismatch{lane,kind}`; modeled as `LaneResolverMismatchObservation` (folded in-process from the sampler — see §F-3, no separate `lane.cred_resolver_mismatch` event row); fail-closed | `laneproviderauth/resolver.go:ResolveCredential`,`ErrResolverMismatch`; `metrics/lane_auth_fold.go:sampleLaneCredentials`,`isResolverMismatch` | `TestCredResolverFailsClosedOnUnprovenSource`, `TestCredResolverTracksLaunchEnvNotHomeDecoy` (FA-F2), `TestLaneCredSeriesBudget` (FA-3) |
| **3. Layer 1 sampler** | `striatum_lane_cred_seconds_to_expiry{lane,kind}`, `…_cred_age_seconds{lane,kind}`, `…_cred_sample_present{lane,kind}`; samples folded directly in-process in `Refresh` (no `lane.cred_expiry_sampled` event — see §F-3) | `laneproviderauth/sampler.go:SampleLaneCredential`; `expiry.go:ParseExpiry`; `metrics/lane_auth_fold.go:sampleLaneCredentials`; `metrics/collector.go:Refresh` | `TestCredExpirySamplerReadsLanePresentedCredential` (FA-F2/FA-6), `TestNoExpiryCredentialDoesNotSatisfyExpiryCensus`, `TestScalarCountCannotMaskRosterMismatch` (FA-F1), `TestLaneCredSeriesBudget` (FA-3) |
| **4. Layer 3 heartbeat** | `striatum_lane_auth_last_success_timestamp_seconds{lane}`; event `lane.auth_success` at the gate `Passed()` branch; fold = `MAX(created_at)` per lane_user→roster slug | `mutations/supervision_provider_auth.go:emitLaneAuthSuccessEvent` (`if result.Passed()`); `mutations/supervision_control.go`; `metrics/lane_auth_fold.go:(*Collector).laneAuthSuccessObservations` | `TestAuthSuccessEventOnlyOnPassedCodex` (FA-5), `TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict` (FA-7) |
| **5. Alert rules** | `LaneCredExpirySoon`, `LaneCredRenewalStalled`, `LaneAuthSampleMissing`, `LaneCredResolverMismatch`, `LaneAuthHeartbeatStale`, `LaneAuthExporterDown` (PromQL = PROPOSAL §"Exact alert surface"; each carries the affected `lane` label except the intentional shared-fate `LaneAuthExporterDown`) | `metrics/rules/alerting_rules.yml` | `TestPrometheusRulesReferenceRegisteredMetrics` (every referenced series is a registered family) |

All eight families land in `DefaultRegistry()`, all `ClassificationOperational`, with only their closed label sets; the scalar `striatum_lane_auth_expected_count` is retired (the F1 defect).

## 3. F1 / F2 folds (binding — honored, not regressed)

- **F1 (coverage).** Per-lane `striatum_lane_auth_expected{lane,provider,kind}` (roster) + observed `striatum_lane_cred_sample_present{lane,kind}`. The census is the **label-preserving** rule `striatum_lane_auth_expected unless on(lane) (striatum_lane_cred_sample_present == 1)` (alert `LaneAuthSampleMissing`) — per-lane, never an aggregate `count(...) < scalar`. MVP narrows positive expiry telemetry to **expiring (OAuth)** credentials; non-codex `api_key` lanes are **census-covered** (`sample_present=1`, `HasExpiry=false` → no `seconds_to_expiry`/`age` series) and carry **no** positive-validity claim (explicit accepted/deferred risk). A healthy `api_key` lane neither pages nor is silently dropped. Proven by `TestScalarCountCannotMaskRosterMismatch`, `TestNoExpiryCredentialDoesNotSatisfyExpiryCensus`, `TestLaneAuthSeriesMissingNamesMissingLane`.
- **F2 (resolution).** `ResolveCredential` **fails closed** into `ErrResolverMismatch` → `striatum_lane_cred_resolver_mismatch` (alert `LaneCredResolverMismatch`) for an unknown provider, an unresolvable launch env, an unreadable file, or an unparseable payload — **never** a green gauge from a fallback/decoy path. The config-dir launch-env key (`CODEX_HOME` / `CLAUDE_CONFIG_DIR`) wins over `HOME`, so a fresher `HOME` decoy is never read; the sampler distinguishes ABSENT (`os.ErrNotExist`→census) from UNPROVABLE (→mismatch) so a vanished credential pages via the census, not as a mismatch. The roster `credential_path_template` is a drift cross-check only, never the read path. Proven by `TestCredResolverFailsClosedOnUnprovenSource`, `TestCredResolverTracksLaunchEnvNotHomeDecoy`, `TestCredExpirySamplerReadsLanePresentedCredential`.

## 4. Boundary (FA-7 — read-only telemetry over the auth boundary)

- The existing `laneproviderauth` and `supervision_provider_auth` suites pass **UNCHANGED** — the diff adds only NEW `_test.go` files (zero edits/deletions to existing tests). `laneproviderauth.Check()` (`lane_provider_auth.go`) is untouched and stays a pure classifier with no DB handle; the heartbeat write lives in the caller.
- The Layer 3 heartbeat write and the Layer 1 sampler are on success/observation paths only, are **best-effort**, run in their own short tx, and **swallow their own errors** — a write failure can never flip a gate decision or alter a timeout. Proven by `TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict`.
- No change to preflight behavior, timeouts, or the credential trust model (that is RFC 0143). Local-first, pull-only; no per-repo private data on the wire (`lane` = roster slug / OS user; `provider`/`kind` = closed enums; never a repo/run/session id, path, sha, branch, prompt, or byline — `safeLaneLabel` clamps to `other`).

## 5. Verification (this run)

| Check | Result |
| --- | --- |
| `go build ./...` | clean (exit 0) |
| `go vet ./...` | clean (exit 0) |
| `golangci-lint run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./...` (pinned **v2.12.2**) | **0 issues** |
| `go test ./go/pkg/metrics/... ./go/pkg/laneproviderauth/... ./go/pkg/mutations/... ./go/pkg/reads/...` | all four packages `ok` |
| Exposition guardrails | `TestMetricsAllowlistMatchesRegistry`, `TestVerifyAllowlistDetectsDrift`, `TestMetricsRedactionGoldenAndForbiddenContent`, `TestPrometheusRulesReferenceRegisteredMetrics` — all PASS |
| Named FA-* tests | all present and PASS (FA-3/4/5/7, FA-F1, FA-F2) |

- **Allowlist + boot hash.** `metrics_allowlist.json` regenerated; `sha256 = 275863abdcb068f1769876acef293dac73de92a82b0a07047a0d1c4cbd5a6651`. Re-running `STRIATUM_UPDATE_ALLOWLIST=1` / `STRIATUM_UPDATE_GOLDEN=1` is a **no-op** against the committed manifest/golden — the apply-stage nits (F-1 `errors.Is`, F-4 comment) changed no metric surface.
- **Game-day boundary.** The live fire tests **GD-1…GD-5** (actually firing each alert via Alertmanager → Slack) are the **`rfc-0162-verify`** run's job, NOT this run's. This build makes the metric/alert surface so they *can* fire.
- **Alert re-vendoring.** The six alerts in `go/pkg/metrics/rules/alerting_rules.yml` await the operator re-vendoring them **verbatim** into `halbritt/proximal` (`observability/prometheus/rules/striatum-alerting.rules.yml`, `striatum` group → `#proximal-alerts`). This run does **not** touch the proximal repo.

## 6. Scope

- Build-order **steps 1–5 only**. **Layer 2** (active prober / `striatum-lane-auth-prober.timer`, step 6) is **deferred** — not built; its follow-up metric names are not registered.
- The `halbritt/proximal` repo is **untouched** by this run.
- **Review findings carried to the follow-up** (info-level, not blocking, all out of MVP scope):
  - **F-2** — when the doctor reconciliation is wired into the live bundle, `LaneAuthLiveLanesFromEvents` must also count lanes with a successful sample (`sample_present=1`), or it will false-positive on healthy claude/`api_key` lanes. `ReconcileLaneAuthRoster` is pure and correct for its inputs today; the wiring is the follow-up.
  - **F-3** — the implementation folds Layer 1 samples **in-process** inside `Collector.Refresh` rather than via the PROPOSAL's `lane.cred_expiry_sampled` event (defensible: point-in-time snapshot gauges re-derived each sweep, no counter needing tx-safety, identical wire surface). This is the root cause of F-2 (no sample event for the reconciliation to read) and is noted here so the divergence from the PROPOSAL mechanism is explicit, not silent (the PROPOSAL lives under `…/rfc-0162-design/`, outside this run's write scope).
  - **F-5** — a parseable codex `auth.json` with no recognizable expiry field is reported present-no-expiry (`HasExpiry=false`) rather than failing closed; a malformed-yet-JSON OAuth token would then emit `sample_present=1` with no expiry alarm. Acceptable for the MVP; recorded as the known boundary.

# Task — Draft: build the RFC 0162 lane-auth observability MVP (test-first)

Read `SEED.md` and your context docs first — above all
**`docs/operator/artifacts/rfc-0162-design/commit/proposal/PROPOSAL.md`** (your
authoritative contract), the cycle-2 ledger (F1/F2 binding folds), `AGENTS.md`, and
the three source anchors (`go/pkg/metrics/registry.go`,
`go/pkg/laneproviderauth/lane_provider_auth.go`,
`go/pkg/mutations/supervision_provider_auth.go`).

You make **real source changes** in your worktree (this lane has
`publish_source_changes: true`). The actual Go code is the deliverable; the
`DRAFT.md` handoff just describes it. Hold the root reframe: **alert on the absence
of expected success.**

## Build, in the PROPOSAL's contract-first order (steps 1–5; step 6 / Layer 2 is OUT)

Work TDD — for each step, write the named test, watch it fail for the right reason,
then implement until green.

1. **Backbone roster** + fold → `striatum_lane_auth_expected{lane,provider,kind}` +
   the OQ4 threshold gauges (`…staleness_threshold_seconds{lane}`,
   `…cred_expiry_lead_seconds{lane}`) + the doctor reconciliation check. Tests:
   `TestThresholdFromRosterNotObserved`, `TestScalarCountCannotMaskRosterMismatch`.
2. **F2 resolver contract** (codex + claude) + `lane.cred_resolver_mismatch` event +
   fold → `striatum_lane_cred_resolver_mismatch{lane,kind}`. **Fail closed** — never
   a green gauge from a decoy/fallback path; read the credential the lane's **launch
   env** resolves, not a fresher `HOME` decoy. Tests:
   `TestCredResolverFailsClosedOnUnprovenSource`,
   `TestCredResolverTracksLaunchEnvNotHomeDecoy`,
   `TestCredExpirySamplerReadsLanePresentedCredential`.
3. **Layer 1 sampler** in `Collector.Refresh` (resolver-proven read as the lane user,
   bounded by a per-lane timeout like `doctorFoldTimeout`) + `lane.cred_expiry_sampled`
   event + fold → `…cred_seconds_to_expiry{lane,kind}`, `…cred_age_seconds{lane,kind}`,
   `…cred_sample_present{lane,kind}`. `api_key` lanes emit `sample_present=1` when
   healthy but **no** expiry series. Tests: `TestLaneAuthSeriesMissingNamesMissingLane`,
   `TestNoExpiryCredentialDoesNotSatisfyExpiryCensus`, `TestLaneCredSeriesBudget`
   (extended to the new `{lane,…}` families).
4. **Layer 3 heartbeat** — emit `lane.auth_success` at the gate success branch
   `go/pkg/mutations/supervision_provider_auth.go:56` (`if result.Passed()`), and the
   other real-success sites the PROPOSAL names (`doctor lane_provider_auth`,
   `run drive`); fold → `striatum_lane_auth_last_success_timestamp_seconds{lane}`.
   **Codex-scoped by construction** — never emit on the `supported==false` early
   return. Tests: `TestAuthSuccessEventOnlyOnPassedCodex`,
   `TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict`.
5. **Alert rules** — append the six MVP alerts (exact PromQL in PROPOSAL §"Exact
   alert surface": `LaneCredExpirySoon`, `LaneCredRenewalStalled`,
   `LaneAuthSampleMissing`, `LaneCredResolverMismatch`, `LaneAuthHeartbeatStale`,
   `LaneAuthExporterDown`) to **`go/pkg/metrics/rules/alerting_rules.yml`**. Keep the
   guardrail test `go/pkg/metrics/rules_test.go` green (every referenced series must
   be a registered family). Do **not** edit the `halbritt/proximal` repo — the
   operator re-vendors this file separately.

All eight MVP families land in `DefaultRegistry()` (`registry.go:149`) + `render.go`,
`ClassificationOperational`, closed label sets only; **regenerate `metrics_allowlist.json`
and the boot-time allowlist hash** (part of the deliverable).

## Binding folds (do not regress)

- **F1:** the census rule is
  `striatum_lane_auth_expected unless on(lane) (striatum_lane_cred_sample_present == 1)`
  — **preserve the `lane` label**; retire any scalar `expected_count`. MVP scope =
  expiring OAuth for positive expiry; non-codex `api_key` = census-covered only
  (explicit accepted risk), neither pages while healthy nor goes silent.
- **F2:** resolver fails closed into `lane_cred_resolver_mismatch`; `credential_path_template`
  is a drift cross-check, never the authoritative read path.

## Boundary (FA-7 — do not violate)

- **No change to preflight behavior, timeouts, or trust model.** The heartbeat/sampler
  writes are best-effort on success/observation paths and swallow their own errors —
  a write failure can never flip a gate decision. `laneproviderauth.Check()` stays a
  pure classifier (no DB handle); the write lives in the caller.
- **The existing `laneproviderauth` and `supervision_provider_auth` suites MUST pass
  UNCHANGED.** No private data on the wire (`lane`=roster slug; `provider`/`kind`
  closed enums; no id/path/sha/branch/prompt/byline).

## Verify before you hand off

`go build ./... && go vet ./... && golangci-lint run` (per `AGENTS.md`; lint pinned
v2.12.2) and
`go test ./go/pkg/metrics/... ./go/pkg/laneproviderauth/... ./go/pkg/mutations/... ./go/pkg/reads/...`.
The live **game-day** (firing alerts to Slack) is the verify-run's job, not yours —
build the surface so they *can* fire.

## Deliverable

Make the source changes, then publish
**`docs/operator/artifacts/rfc-0162-build/DRAFT.md`** (kind `handoff`) describing:
files changed (one-line role each), how each build step + each FA-* test is
discharged (map to file:symbol), the F1/F2 folds, the allowlist regen, and your
`go build`/`vet`/`test`/`lint` results. Do not touch `.striatum/`.

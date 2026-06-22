# SEED — RFC 0162 lane-auth observability MVP (build run)

This run **builds the RFC 0162 lane-auth silent-failure observability MVP**,
**test-first**. The design is already hardened and falsification-cleared; this is
execution, not design. The deliverable is **real Go source** in this repo.

## Read these first (your context docs)

- **`docs/operator/artifacts/rfc-0162-design/commit/proposal/PROPOSAL.md`** — the
  falsification-cleared implementation spec. **This is your primary, authoritative
  contract.** It gives the exact metric surface (§"Exact metric surface"), the
  exact code sites, the events/folds, the F2 resolver contract, the named tests
  (FA-*), and the **6-step build order**. Build to it.
- **`…/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_2.md`** — the two **binding
  findings F1 and F2** (with `closest_acceptable_answer`) the spec folds. Do not
  regress them.
- `docs/rfcs/0162-lane-auth-silent-failure-observability.md` — the RFC (background).
- `AGENTS.md` — product boundary + build conventions (golangci-lint v2.12.2 pinned
  in `go/Makefile`; CI runs pgtests; reproduce lint locally before claiming green).
- `go/pkg/metrics/registry.go`, `go/pkg/laneproviderauth/lane_provider_auth.go`,
  `go/pkg/mutations/supervision_provider_auth.go` — the surfaces you edit.

## Scope — the MVP = PROPOSAL build-order steps 1–5 (step 6 / Layer 2 is OUT)

Build, in the PROPOSAL's **contract-first order**:

1. **Backbone roster** file + fold → `striatum_lane_auth_expected{lane,provider,kind}`
   + the OQ4 threshold gauges (`…staleness_threshold_seconds{lane}`,
   `…cred_expiry_lead_seconds{lane}`); the doctor reconciliation check.
2. **F2 resolver contract** (codex + claude) + `lane.cred_resolver_mismatch` event +
   fold → `striatum_lane_cred_resolver_mismatch{lane,kind}`; **fail-closed**
   semantics (never a green gauge from a decoy/fallback path).
3. **Layer 1 sampler** in `Collector.Refresh` (resolver-proven read as the lane user)
   + `lane.cred_expiry_sampled` event + fold →
   `striatum_lane_cred_seconds_to_expiry{lane,kind}`,
   `…cred_age_seconds{lane,kind}`, `…cred_sample_present{lane,kind}`.
4. **Layer 3 heartbeat** — emit `lane.auth_success` at the gate success branch
   `go/pkg/mutations/supervision_provider_auth.go:56` (and the other real-success
   sites named in the PROPOSAL); fold → `striatum_lane_auth_last_success_timestamp_seconds{lane}`.
   **Codex-scoped by construction** — never synthesize a heartbeat for a provider
   whose `Check()` never ran (FA-5).
5. **Alert rules** — append the six MVP alerts (PromQL in PROPOSAL §"Exact alert
   surface") to **`go/pkg/metrics/rules/alerting_rules.yml`** (the guardrail test
   `go/pkg/metrics/rules_test.go` / `TestPrometheusRulesReferenceRegisteredMetrics`
   asserts every referenced series is a registered family — keep it green). The
   operator re-vendors this file verbatim into `halbritt/proximal` separately; do
   **not** edit the proximal repo from this run.

**Layer 2 (active prober / `striatum-lane-auth-prober.timer`, step 6) is OUT of
scope** — it is the explicit follow-up. You MAY register its follow-up metric
*names* per the PROPOSAL if trivially additive, but do not build the prober.

All eight MVP families land in `DefaultRegistry()` (`registry.go:149`) + `render.go`,
all `ClassificationOperational`, with **only** their closed label sets; regenerate
`metrics_allowlist.json` and the boot-time allowlist hash (a diff-reviewed,
CI-guarded manifest edit — part of the deliverable).

## Binding folds (do not regress — from cycle-2 ledger / the override decision)

- **F1 (coverage).** Per-lane `striatum_lane_auth_expected` vector + the
  `sample_present` observed vector; the census rule is
  `striatum_lane_auth_expected unless on(lane) (striatum_lane_cred_sample_present == 1)`
  — it **preserves the `lane` label** (no aggregate-only rule). MVP is narrowed to
  **expiring (OAuth) credentials** for positive expiry telemetry; non-codex
  `api_key` lanes are **census-covered** (absence) but carry **no** positive-validity
  claim (explicit accepted/deferred risk). A healthy `api_key` lane must neither
  page nor be silently dropped.
- **F2 (resolution).** The credential resolver **fails closed** into a pageable
  `striatum_lane_cred_resolver_mismatch` when the runtime credential source cannot
  be proven; it reads the credential the lane's **launch env** resolves, never a
  fresher `HOME` decoy. The roster `credential_path_template` is a drift cross-check,
  never the authoritative read path.

## Boundary (FA-7 — read-only telemetry over the auth boundary)

- **No change to preflight behavior, timeouts, or the credential trust model** (that
  is RFC 0143). The heartbeat/sampler writes are on success/observation paths only,
  are **best-effort**, and swallow their own errors — a write failure can **never**
  flip a gate decision or alter a timeout.
- **The existing `laneproviderauth` and `supervision_provider_auth` test suites MUST
  pass UNCHANGED.** `laneproviderauth.Check()` stays a pure classifier (no DB
  handle) — the heartbeat write lives in the caller.
- Local-first, pull-only; no hosted/cloud/push. No per-repo private data on the wire:
  `lane` = roster slug (OS user), `provider`/`kind` = closed enums; never a
  repo/run/session id, path, sha, branch, prompt, or byline.

## Named tests to land green (contract-first — from PROPOSAL FA-table)

`TestLaneCredSeriesBudget` (extended), `TestThresholdFromRosterNotObserved`,
`TestAuthSuccessEventOnlyOnPassedCodex`, `TestLaneAuthSeriesMissingNamesMissingLane`,
`TestNoExpiryCredentialDoesNotSatisfyExpiryCensus`,
`TestScalarCountCannotMaskRosterMismatch`,
`TestCredResolverFailsClosedOnUnprovenSource`,
`TestCredResolverTracksLaunchEnvNotHomeDecoy`,
`TestCredExpirySamplerReadsLanePresentedCredential`,
`TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict`.

## Verification (what this run proves vs. what the verify-run proves)

- **This build run** must make the named Go tests pass and the tree
  **`go build` / `go vet` / `golangci-lint` clean**, with the allowlist hash
  regenerated. Run at least:
  `go test ./go/pkg/metrics/... ./go/pkg/laneproviderauth/... ./go/pkg/mutations/... ./go/pkg/reads/...`
- The **game-day fire tests (GD-1…GD-5)** — actually firing each alert via
  Alertmanager → Slack — are the **`rfc-0162-verify` run's** job, NOT this run's.
  Build the metric/alert surface so they *can* fire; do not attempt the live
  game-day here.

Keep the diff minimal and idiomatic to the surrounding `go/pkg/metrics` code
(reuse `applySeriesBudget`, the event-fold pattern in `Collector.Refresh`, the
`doctor_problems{class}` precedent for the bounded `lane` label). Do not touch
`.striatum/`.

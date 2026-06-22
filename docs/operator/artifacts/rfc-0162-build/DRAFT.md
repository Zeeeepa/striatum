# DRAFT — RFC 0162 lane-auth silent-failure observability MVP (build run)

author: author-author-001

> Handoff for the `rfc-0162-build` run. The deliverable is **real Go source** in
> this repo, built **test-first** to the falsification-cleared
> `PROPOSAL.md` (build-order steps 1–5; step 6 / Layer 2 is OUT). This handoff
> describes the source changes; the source is the artifact. The live **game-day
> fire tests (GD-1…GD-5)** are the `rfc-0162-verify` run's job — this run builds
> the surface so they *can* fire.

## Root reframe held

Every family below makes the **absence of expected success** loud and never reads
green off a fallback/decoy path. A non-codex lane whose credential vanishes (no
`sample_present`) or whose runtime source cannot be proven (`resolver_mismatch`)
pages **with its lane named**.

## Files changed (one-line role each)

**`go/pkg/laneproviderauth/` (the F2 resolver + roster + expiry + sampler — pure, leaf):**
- `roster.go` — `RosterEntry`/`Roster` + `LoadRoster`/`ParseRoster` (bare-array or `{"lanes":[…]}`), the `{oauth,api_key}` closed kind enum, `DefaultRosterPath()`, and the OQ4 `Effective{Staleness,ExpiryLead}Seconds()` (declared value, else a default from the **declared** cadence — never observed lifetime).
- `resolver.go` — the **F2 fail-closed credential-resolver contract** (`ResolveCredential`): per-provider runtime source + precedence (codex `$CODEX_HOME/auth.json`→`$HOME/.codex/…`; claude `$CLAUDE_CONFIG_DIR/.credentials.json`→`$HOME/.claude/…`), reading the **launch-env**-selected credential (config-dir key wins over HOME — no decoy), `ErrResolverMismatch` when unprovable.
- `expiry.go` — provider-specific expiry extraction (`ParseExpiry`): codex `tokens.id_token` JWT `exp` / `expiry`; claude `claudeAiOauth.expiresAt` (ms); `api_key` → `HasExpiry=false`; unparseable OAuth → `ErrUnparseableCredential`.
- `sampler.go` — `SampleLaneCredential` ties resolver→read→parse with the injectable `CredentialReader`; distinguishes **absent** (`ErrCredentialAbsent` → census) from **unprovable** (`ErrResolverMismatch` → page); `LaneFileReader` is the prod sudo-`-n`-as-lane reader.
- `rfc0162_test.go` — the F2/FA-4 named tests (below).

**`go/pkg/metrics/` (the eight families + folds + render + alerts):**
- `lane_auth.go` — fold types + `foldLaneAuth` + the per-family **lane series budget** (≤32 lanes, overflow → `lane="other"`, clip counted) + deterministic sort helpers + label sanitizer.
- `lane_auth_fold.go` — Collector-side best-effort folds: `laneAuthRosterObservations` (roster), `sampleLaneCredentials` (Layer 1 sampler, per-lane timeout, no DB), `(*Collector).laneAuthSuccessObservations` (Layer 3 heartbeat `MAX(created_at)` per lane, unrostered → `other`). Reader/roster/home are package-var seams for tests.
- `lane_auth_test.go` — the metrics fold named tests (below).
- `registry.go` — registers the 8 families in `DefaultRegistry()` (all `ClassificationOperational`, closed label sets).
- `render.go` — metric-name constants + `writeLaneAuth` (HELP/TYPE always emitted; closed labels only).
- `snapshot.go` — `SnapshotInput` lane-auth fields + the `laneAuth` snapshot field + the `foldLaneAuth` call in `Build`.
- `semantics.go` — all 8 declared `SeriesSnapshot` (gauges; never `rate()`/`increase()`).
- `collector.go` — `Refresh` wires the three best-effort lane-auth folds after the load-bearing Phase A folds (a heartbeat-query error degrades the tick to `partial`).
- `metrics_allowlist.json` — **regenerated** (`STRIATUM_UPDATE_ALLOWLIST=1`): +8 families, new sha256.
- `testdata/metrics_golden.txt` — **regenerated** (`STRIATUM_UPDATE_GOLDEN=1`): +8 HELP/TYPE blocks (no series, since the sentinel snapshot feeds no lane-auth input).
- `rules/alerting_rules.yml` — the six MVP alerts appended (below).

**`go/pkg/mutations/` (the Layer 3 heartbeat write):**
- `supervision_provider_auth.go` — emits `lane.auth_success` at the gate success branch (`if result.Passed()`), best-effort via `emitLaneAuthSuccessEvent` (package-var seam) in its own short tx; **swallows errors** so it can never flip the gate verdict (FA-7). Codex-scoped by construction (the Passed branch is reachable only for the supported codex self-driving gate).
- `supervision_control.go` — threads `runner` into `runSuperviseProviderAuthGate`.
- `supervision_provider_auth_rfc0162_test.go` — FA-5 / FA-7 named tests.

**`go/pkg/reads/` (the doctor reconciliation logic):**
- `doctor_lane_auth_roster.go` — `ReconcileLaneAuthRoster` (pure): roster-vs-observation drift → the static check codes `lane_auth_missing_sample` / `lane_auth_unrostered_live_lane` / `lane_auth_resolver_mismatch` (only the static `check` becomes the `doctor_problems` class; the lane slug rides a detail field the fold never reads). `LaneAuthLiveLanesFromEvents` reads observed live lanes from the events ledger.
- `doctor_lane_auth_roster_test.go` — reconciliation tests.

## Build-order steps → discharge

1. **Backbone roster + fold** — `roster.go` + `laneAuthRosterObservations` → `striatum_lane_auth_expected{lane,provider,kind}` + `…staleness_threshold_seconds{lane}` + `…cred_expiry_lead_seconds{lane}`; reconciliation in `doctor_lane_auth_roster.go`.
2. **F2 resolver contract** — `resolver.go` (`ResolveCredential`, fail-closed) → `lane.cred_resolver_mismatch` modeled as `LaneResolverMismatchObservation` → `striatum_lane_cred_resolver_mismatch{lane,kind}`.
3. **Layer 1 sampler** — `sampleLaneCredentials` in `Refresh` (resolver-proven read as the lane user, `laneAuthSampleTimeout` per lane) → `…cred_seconds_to_expiry{lane,kind}`, `…cred_age_seconds{lane,kind}`, `…cred_sample_present{lane,kind}`; `api_key` emits `sample_present=1` with **no** expiry series.
4. **Layer 3 heartbeat** — `emitLaneAuthSuccessEvent` at `supervision_provider_auth.go` success branch → fold `laneAuthSuccessObservations` → `striatum_lane_auth_last_success_timestamp_seconds{lane}`.
5. **Alert rules** — six MVP alerts appended to `rules/alerting_rules.yml` (the guardrail test stays green).

## FA-* / named tests → discharge

| Test (file) | Discharges |
| --- | --- |
| `TestThresholdFromRosterNotObserved` (laneproviderauth + metrics) | FA-4 — thresholds are roster-declared, invariant to observed/sampled lifetime |
| `TestCredResolverFailsClosedOnUnprovenSource` (laneproviderauth) | FA-F2 — unknown provider / no resolvable path → `ErrResolverMismatch` |
| `TestCredResolverTracksLaunchEnvNotHomeDecoy` (laneproviderauth) | FA-F2 — config-dir launch-env key wins over a fresher HOME decoy |
| `TestCredExpirySamplerReadsLanePresentedCredential` (laneproviderauth) | FA-F2/FA-6 — sampler reads the launch-env-resolved credential; absent vs mismatch vs api_key branches |
| `TestParseExpiryShapes` / `TestParseRosterNormalizes` (laneproviderauth) | expiry parsing + roster normalization (drops unbounded kind) |
| `TestLaneAuthSeriesMissingNamesMissingLane` (metrics) | FA-F1 — per-lane expected vector names the missing lane (no aggregate-only) |
| `TestScalarCountCannotMaskRosterMismatch` (metrics) | FA-F1 — healthy api_key lane covered (expected + sample_present), not dropped; per-lane, not scalar |
| `TestNoExpiryCredentialDoesNotSatisfyExpiryCensus` (metrics) | FA-F1 — api_key emits sample_present but never a seconds_to_expiry series |
| `TestLaneCredSeriesBudget` (metrics) | FA-3 — 100 lanes → bounded at budget+1 (`lane="other"`) + clip counter |
| `TestAuthSuccessEventOnlyOnPassedCodex` (mutations) | FA-5 — heartbeat only on a real `Passed()`, never on the unsupported early return or a failed check |
| `TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict` (mutations) | FA-7 — a failed heartbeat write is swallowed; the gate verdict is unchanged |
| `TestReconcileLaneAuthRoster[CleanState]` (reads) | reconciliation classes + static check codes |

## F1 / F2 folds (binding — not regressed)

- **F1**: per-lane `expected{lane,provider,kind}` + observed `sample_present{lane,kind}`; the census rule `striatum_lane_auth_expected unless on(lane) (striatum_lane_cred_sample_present == 1)` (alert `LaneAuthSampleMissing`) **preserves the `lane` label**; no scalar `expected_count`. MVP = expiry telemetry for OAuth; non-codex `api_key` is **census-covered only** (explicit accepted risk) — neither pages while healthy nor goes silent.
- **F2**: the resolver **fails closed** into `striatum_lane_cred_resolver_mismatch` (alert `LaneCredResolverMismatch`); the roster `credential_path_template` is a drift cross-check, never the read path; the sampler reads the launch-env-resolved credential, never a fresher HOME decoy.

## Exact alert surface (appended to `rules/alerting_rules.yml`)

`LaneCredExpirySoon`, `LaneCredRenewalStalled`, `LaneAuthSampleMissing`,
`LaneCredResolverMismatch`, `LaneAuthHeartbeatStale`, `LaneAuthExporterDown` — each
carries the affected `lane` label (no aggregate-only rule). PromQL matches the
PROPOSAL §"Exact alert surface". The operator re-vendors this file into
`halbritt/proximal` separately; this run does **not** touch the proximal repo.

## Boundary (FA-7) compliance

- No change to preflight behavior, timeouts, or the credential trust model. The
  heartbeat write and the Layer 1 sampler are on success/observation paths only,
  are best-effort, and swallow their own errors.
- `laneproviderauth.Check()` stays a pure classifier (no DB handle); the heartbeat
  write lives in the caller. The **existing `laneproviderauth` and
  `supervision_provider_auth` suites pass UNCHANGED** (no existing test reaches the
  Passed-codex heartbeat branch).
- No per-repo private data on the wire: `lane` = roster slug (OS user),
  `provider`/`kind` = closed enums (sanitized to `other` on any non-conforming
  shape). The redaction golden + forbidden-content guardrail pass.

## Scope honesty (what is wired vs. proven later)

- The Collector folds are **wired and unit-tested via seams** (`laneRosterLoader`,
  `laneCredentialReader`, `laneOSUserHome`). The **live** sampler exec (sudo-cat as
  the lane user) and the heartbeat DB write are only meaningfully exercisable on a
  host with rostered lanes — that is the `rfc-0162-verify` game-day's job. With no
  roster file present the sampler is a zero-cost no-op.
- The doctor reconciliation is delivered as a **pure, tested function**;
  surfacing it through the live doctor bundle is corroboration (the page comes from
  the Slack census/resolver alerts, per the PROPOSAL) and is a small follow-up —
  the existing codex doctor check is likewise not in the default bundle.
- **Layer 2** (active prober / `striatum-lane-auth-prober.timer`) is OUT of scope
  (the explicit follow-up); its follow-up metric names are not registered here.

## Verification results (this run)

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `golangci-lint run --default=none --enable=govet --enable=staticcheck --enable=errcheck --enable=ineffassign ./...` (pinned v2.12.2) — **0 issues**.
- `go test ./go/pkg/metrics/... ./go/pkg/laneproviderauth/... ./go/pkg/mutations/... ./go/pkg/reads/...` — **all pass** (incl. the regenerated allowlist + golden guardrails and the rules guardrail).

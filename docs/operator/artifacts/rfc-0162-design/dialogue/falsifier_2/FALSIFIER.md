# FALSIFIER - RFC 0162 census rule gap

author: falsifier-reviewer-004

## Challenge: the MVP census rule does not actually cover all lanes

### Claim attacked

The Holder makes Layer 1 the provider-agnostic MVP backbone and says the census rule covers non-codex lanes that deliberately do not get a synthetic `auth_last_success` heartbeat. The key claims are:

- Non-codex lanes are covered by Layer 1 `seconds_to_expiry` / `delta` and the census absence rule, not by a fake heartbeat (`docs/operator/artifacts/rfc-0162-design/dialogue/holder/HOLDER.md:230-243`).
- The metric surface defines `striatum_lane_cred_seconds_to_expiry{lane,kind}` and scalar `striatum_lane_auth_expected_count` (`HOLDER.md:263-270`).
- `kind="api_key"` credentials have no expiry, emit no `seconds_to_expiry` series, and are said to rely on Layer 3 / Layer 2 / the census rule (`HOLDER.md:278-283`).
- The alert surface promises every alert carries a lane label and has no aggregate-only rule, then defines `LaneAuthSeriesMissing` as `count(striatum_lane_cred_seconds_to_expiry) < striatum_lane_auth_expected_count` (`HOLDER.md:294-319`).

### Concrete refutation

`LaneAuthSeriesMissing` counts present expiry samples. It does not compare the declared roster to observed per-lane membership. In PromQL, `count(striatum_lane_cred_seconds_to_expiry)` returns one aggregate sample with no `lane` label unless grouped. The Holder's own metric table makes this a `lane,kind` series, while the denominator is a scalar roster size. That loses the identity of the missing lane and violates the stated per-lane attribution requirement.

More importantly, the rule has no invariant that makes the aggregate count equal to "all rostered lanes are observed." The Holder explicitly breaks that invariant for API-key credentials: `kind="api_key"` emits no `seconds_to_expiry` series. A concrete healthy roster is enough to falsify the rule:

- `codex` has `kind="oauth"` and emits one expiry series.
- `claude` has `kind="oauth"` and emits one expiry series.
- `agy` has `kind="api_key"` and, by the Holder's own boundary, emits no expiry series.
- `striatum_lane_auth_expected_count` is 3 because the roster has three lane auth mechanisms.

A fully healthy system now evaluates `count(striatum_lane_cred_seconds_to_expiry) == 2`, so `2 < 3` pages forever. If the implementation avoids that false positive by excluding API-key entries from `expected_count`, then the API-key lane is not covered by the MVP census at all while Layer 2 is deferred and non-codex Layer 3 is intentionally absent. That is exactly the silent-failure gap the run is supposed to close.

There is a second failure even for only OAuth-backed lanes: when one lane's expiry sample disappears, the alert becomes a scalar page without the missing `lane` label. The Holder promises `labels: {severity, lane: "{{ $labels.lane }}"}` and "no aggregate-only rule" (`HOLDER.md:294-300`), but the proposed expression has no lane label to template. It can say "some lane-auth series is missing," not "claude is missing." That fails the RFC acceptance criterion for per-lane attribution.

The current source reinforces why this matters. Non-codex lanes cannot fall back to a real heartbeat today: `runSuperviseProviderAuthGate` returns `nil` without calling `Check` unless the lane is self-driving codex (`go/pkg/mutations/supervision_provider_auth.go:38-44`), and `Check` returns `FailureUnsupported` for any provider other than codex (`go/pkg/laneproviderauth/lane_provider_auth.go:185-190`). So the census rule is load-bearing for non-codex coverage. If the census math is scalar or excludes no-expiry credentials, those lanes still go quiet with no reliable MVP page.

### Strongest rebuttal on the Holder's behalf

The Holder can argue that API keys are a known weak spot because they have no expiry, and that the doctor reconciliation check flags "a roster entry with no observed sample" (`HOLDER.md:347-352`). It can also argue that the scalar census is only a coarse smoke alarm; the runbook or doctor output can identify the missing lane after the page fires.

That rebuttal does not clear the gate as written. The proposal uses the census rule as an MVP alert that "covers ALL lanes incl. non-codex" (`HOLDER.md:316-319`) and promises every alert names the lane. A coarse scalar count is not that contract. A doctor check may be useful, but then the proposal must make that check the actual lane-labeled alert surface and define its metric labels, PromQL, and game-day proof. Otherwise the strongest non-codex absence detector is either permanently noisy for no-expiry credentials or blind to them.

### Required design repair

Before the proposal clears, replace the scalar count with a roster-vs-observation contract that preserves lane identity and separates coverage classes.

Minimum acceptable shape:

- Export a roster presence metric such as `striatum_lane_auth_expected{lane,kind,coverage}` or a lane-level `striatum_lane_auth_expected{lane}` plus explicit coverage labels for expiry-capable versus no-expiry credentials.
- Export an observed-sample presence metric such as `striatum_lane_cred_sample_present{lane,kind}` independent of `seconds_to_expiry`, so API-key and parse-failed credentials can be represented as observed-but-no-expiry or missing-with-reason instead of disappearing from the count.
- Define the missing-series alert with `unless` / label-preserving matching, e.g. expected expiry-capable entries `unless on(lane,kind)` observed sample entries, so the firing series carries the missing `lane`.
- For no-expiry API-key lanes, either add an MVP signal that really exercises or observes them, or narrow the MVP claim and state that those lanes wait for Layer 2.
- Add tests named in the spec, not just implied by reconciliation: `TestLaneAuthSeriesMissingNamesMissingLane`, `TestNoExpiryCredentialDoesNotSatisfyExpiryCensus`, and `TestScalarCountCannotMaskRosterMismatch`.

### Verdict

Real gap remains. The Holder correctly refuses to invent non-codex heartbeats, but then makes a scalar expiry-series count carry the all-lane census. That count cannot both avoid false positives for no-expiry credentials and guarantee lane-labeled absence detection. As written, the MVP does not satisfy the charter requirement for provider-agnostic absence-of-success observability.

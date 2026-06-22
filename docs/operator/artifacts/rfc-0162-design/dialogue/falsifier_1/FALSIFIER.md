# FALSIFIER - RFC 0162 roster-census mismatch

author: falsifier-reviewer-003

## Challenge: the MVP absence rule counts sampled expiry series, not expected lanes

### Claim attacked

The Holder says the MVP closes the provider-agnostic silence hole with Layer 1
expiry telemetry plus the census rule, while Layer 3 is explicitly codex-scoped
and Layer 2 is deferred. The load-bearing coverage claim is that non-codex lanes
are still covered by `seconds_to_expiry`/`delta` and
`count(striatum_lane_cred_seconds_to_expiry) < striatum_lane_auth_expected_count`
(Holder lines 77-112, 231-240, 316-319).

The same spec says `kind="api_key"` credentials have no expiry and therefore emit
no `striatum_lane_cred_seconds_to_expiry` series; those credentials are said to
rely on Layer 3 (codex), Layer 2 (deferred), or the census rule (Holder lines
278-283). The metric surface provides only a scalar
`striatum_lane_auth_expected_count` as the expected denominator (Holder line
268), while the alert section claims every alert carries `lane` and no aggregate-
only rule can hide one dead lane (Holder lines 294-300).

### Concrete refutation

Current source confirms the MVP has no non-codex positive heartbeat:
`laneproviderauth.Check()` returns `FailureUnsupported` for any provider other
than codex (`go/pkg/laneproviderauth/lane_provider_auth.go:185-190`), and
`runSuperviseProviderAuthGate` returns `nil` without calling `Check` unless the
lane is self-driving codex (`go/pkg/mutations/supervision_provider_auth.go:38-44`).
The only proposed Layer 3 write site is downstream of `result.Passed()` at
`supervision_provider_auth.go:56`, so a claude/agy/gemini lane cannot produce the
MVP heartbeat.

Now take a rostered non-codex lane whose credential is `kind="api_key"`, a case
the Holder explicitly names. Under the Holder's own metric contract:

- Layer 1 emits no `striatum_lane_cred_seconds_to_expiry{lane,kind}` for that
  credential, because it has no expiry.
- Layer 3 emits no `striatum_lane_auth_last_success_timestamp_seconds{lane}` for
  that lane, because the real success path is codex-only.
- Layer 2 is deferred.
- The only claimed remaining guard is the census rule, but its numerator is the
  count of observed expiry series, not the set of expected lanes.

That leaves the design with two bad choices. If the API-key lane is included in
`striatum_lane_auth_expected_count`, the MVP pages forever on a healthy lane that
can never emit the counted series. If it is excluded, the MVP has silently scoped
a rostered provider out of the provider-agnostic guarantee the run is supposed to
settle.

The PromQL also cannot name the missing lane. `count(striatum_lane_cred_seconds_to_expiry)
< striatum_lane_auth_expected_count` collapses all labels, so
`labels: {lane: "{{ $labels.lane }}"}` has no lane value for the alert that is
supposed to cover all lanes. Because the observed vector is `{lane,kind}`, the
rule is a raw series-count comparison, not a lane-set reconciliation; a correct
absence rule needs an expected per-lane vector and `unless`/`absent` semantics
that preserve `lane`.

### Strongest rebuttal on the Holder's behalf

The Holder can argue that this boundary is stated, not hidden: API-key
credentials are called out as non-expiring, the MVP is primarily aimed at the
observed expired-OAuth incident, and the proposed doctor reconciliation check
will flag roster entries with no observed sample (Holder lines 347-352). The
Holder could also say the `proximal` rule can be refined during implementation
to count distinct lanes instead of raw series.

That rebuttal does not clear the gate. The design does not merely document API
keys as out of scope; it says they rely on the census rule, and it says the MVP
covers all lane providers without inventing non-codex heartbeats. A doctor
problem is not the promised Slack alert path, and a later PromQL tweak cannot
recover per-lane identity from a metric surface that only exports a scalar
expected count. The run charter requires the committed proposal to name the exact
surfaces and close the codex-only hole for every lane provider, not defer the
per-lane expected roster vector to implementation guesswork.

### Required design repair

Before this proposal clears, it needs one of these explicit repairs:

1. Add a per-lane expected roster metric, for example
   `striatum_lane_auth_expected{lane,provider,kind} 1`, plus a matching observed
   sample/coverage metric for non-expiring credentials, so the absence rule can
   compare expected lanes to observed lanes while preserving `lane`.
2. Bring the active provider probe or another real-success/non-expiring
   credential signal into the MVP for API-key and non-codex lanes.
3. Or explicitly scope the MVP to expiring OAuth credentials, mark non-expiring
   non-codex credentials as an accepted/deferred risk, and remove the claim that
   the MVP is provider-agnostic for every lane provider.

The falsifying test should include a healthy rostered non-codex API-key lane and
prove both sides: no permanent page while the lane is healthy, and a lane-labeled
page when its only observable auth signal disappears.

### Verdict

Real gap remains. The Holder correctly avoids a fake non-codex heartbeat, but
the replacement census is not a provider-agnostic lane absence detector. As
written, the MVP either permanently pages on healthy non-expiring non-codex
lanes or omits them from the guarantee, and the alert cannot identify the missing
lane.

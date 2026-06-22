# RFC 0162: Lane auth silent-failure observability (detect absence-of-success)

Status: proposed
Date: 2026-06-22
Context: [#569](https://github.com/halbritt/striatum/issues/569); RFC 0091 (lane-health / liveness classification), RFC 0131 (transport-aware liveness confidence), RFC 0137 (striatumd Prometheus exporter), RFC 0143 (lane credential survival across boot-epoch rotation)
author: proposer-claude-opus-4-8

## Problem

A lane's **provider-auth preflight** can time out, and the breakage goes
**unnoticed**. The preflight (`go/pkg/laneproviderauth/lane_provider_auth.go`,
`DefaultTimeout = 45s`) classifies a context-deadline overrun as
`FailureTimeout` → `"lane_provider_preflight_timeout"`, and the offline
`auth.json` probe has its own ~5s timeout (delegated through `sudo -n -u <user>`
when the lane runs as the RFC 0096 sandbox OS user). The recent fix
(`#556`/`#567`, on `origin/main` @ `bb77ed75`) made the preflight *behave*
correctly — offline codex auth preflight + no crash-loop on a deterministic
refusal — but did **not** add any way to *notice* that a lane's auth has
silently stopped succeeding.

The gap is observability, not behavior. There is no alert in
`halbritt/proximal` →
`observability/prometheus/prometheus/rules/striatum-alerting.rules.yml` for
lane-auth health (today's striatum rules cover necrosis rate, doctor-red,
wedge-age, liveness-margin collapse, supervisor-origin flood, and metrics-loop
health — not auth). Auth failure currently only surfaces at point-of-use: the
`supervise.start` gate (`go/pkg/mutations/supervision_provider_auth.go`) and
`run drive` (`go/pkg/cli/rundrive/rundrive.go`). On a quiet lane — one not
currently being driven — a dead credential produces **no signal at all** until
the next time something tries to use it, which may be hours or days later.

**Root reframe.** The weakness is not "auth timed out." It is that we **alert
on the presence of errors, never on the absence of success**. A lane that stops
authenticating successfully emits no error to trigger on — it goes quiet — and
quiet currently reads as healthy. This is the same failure class RFC 0091
(lane-health/liveness) and RFC 0131 (transport-aware liveness confidence) tackle
for *session* liveness; this RFC extends absence-detection to the *credential /
auth* dimension.

This RFC was scoped with a divergent-ideation pass (5 cognitive frames × 6 ideas
→ scored/clustered → top-3 deepened). The converged result is a 3-layer
defense-in-depth plus a registry backbone, recorded below.

## Goals

- A silently dead or timed-out lane auth (the `lane_provider_preflight_timeout`
  failure mode, **and** the "stopped trying entirely" mode) raises an alert via
  the existing Alertmanager → Slack `#proximal-alerts` path within a bounded,
  documented detection window.
- Detection does **not** depend on organic traffic hitting the lane — a quiet
  lane is still exercised.
- Catch the most common root cause (a credential renewal that silently stopped)
  *during the grace window*, before the credential lapses.
- Per-lane attribution: an alert names the specific lane and links a runbook.

## Non-Goals

- Changing the auth preflight behavior, timeouts, or the credential trust model
  (that is RFC 0143's surface; this RFC is read-only telemetry over it).
- Re-classifying *session* liveness (RFC 0091 / 0131 / 0077) — this is the
  *credential/auth* dimension, complementary to those.
- Implementing the alert rules inside this repo: alert rules live in
  `halbritt/proximal/observability`. This repo owns the **metric surface**
  (exported via the RFC 0137 striatumd Prometheus exporter); `proximal` owns the
  **rules**. The RFC names both halves so the contract is chosen once.

## Proposal

A 3-layer defense-in-depth (proactive → active → passive), each from a distinct
idea cluster so they compose rather than overlap, plus a registry backbone.

### Layer 1 (proactive) — Expiry & renewal-health telemetry  ⭐ most on-target

Most "timed out unnoticed" incidents are really *the refresh silently stopped*.
Export, per lane, read-only credential telemetry through the RFC 0137 exporter:

- `striatum_lane_cred_seconds_to_expiry{lane,kind,id} = not_after - now` — a
  static tripwire alert at a lead-time threshold (≈3× the renewal cadence) fires
  **while the lane is still healthy**.
- **Trend signal:** each successful refresh bumps `not_after`, so
  `delta(striatum_lane_cred_seconds_to_expiry[1.5*cycle])` going strictly
  negative ⇒ the renewer is dead even though the credential is valid *right now*
  (catches it days early).
- `striatum_lane_cred_age_seconds` — flags the one frozen lane whose credential
  never rotates while sibling lanes rotate in lockstep.

**Load-bearing risk:** the exporter must read the *same* credential the lane
actually presents at runtime (the `auth.json` / session-bound token the lane
resolves via `agentloop.ResolveTokenMaterial`), not a stand-in at rest — a fresh
file the process never reloaded looks healthy while the live lane coasts to
death. **First step:** for one real lane, pin the authoritative source of the
presented credential and emit a provably-correct seconds-to-expiry before any
alert.

### Layer 2 (active) — Cross-lane differential probe + negative probe

A prober (decoupled from organic traffic) runs the lane-auth preflight's
read-only check (cf. `doctor lane_provider_auth`,
`go/pkg/reads/doctor_lane_provider_auth.go`) across **all** lanes on a fixed
interval, recording per-lane outcome + latency + identity. Plus a **negative
probe** presenting a deliberately expired/invalid credential, asserting the lane
still **rejects** it — any cached "success" ⇒ fail-open ⇒ immediate page (this
doubles as a live fail-open regression test).

Detection is **differential**: page only when one lane diverges from the sibling
quorum over K-of-M cycles, so a shared upstream outage that fails all lanes
together stays quiet (or fires one aggregated page), while a single rotted lane
is unmistakable and self-isolating to root cause.

**Load-bearing risk:** the prober is a hidden Nth lane — if its own
creds/egress/check endpoint rot, or it dies silently, you get false divergence
or a dead prober reading as all-green. Mandatory: a dead-man's-switch on the
prober itself + an always-expected-fail synthetic lane proving the prober can
still detect failure. **First step:** a one-off script that hits each lane's
preflight with a valid and a deliberately-expired credential and prints the
`{outcome, identity, latency}` diff table — the seed of both the probe contract
and the parity rule.

### Layer 3 (passive backstop) — Per-lane auth freshness dead-man's switch

Each lane writes `striatum_lane_auth_last_success_timestamp_seconds{lane}`
**strictly downstream of a real auth success** (in the preflight success path,
`laneproviderauth`). The alert inverts the usual sense:

```
time() - striatum_lane_auth_last_success_timestamp_seconds{lane}
    > striatum_lane_auth_staleness_threshold_seconds{lane}   # per-lane, ≈1.5× credential lifetime
```

Plus a census rule `count(striatum_lane_auth_last_success_timestamp_seconds) <
striatum_expected_lane_count` to catch a lane that disappears entirely (absence
of series, not just a stale value). ~30 lines of recording/alerting rules in
`proximal` + a one-line write here, reusing the existing Alertmanager → Slack
path.

**Load-bearing risk:** shared-fate — the heartbeat must be downstream of a *real
lane success* (not the prober's own auth), and "no series at all" (exporter
crash, scrape down) must page as loudly as "stale series," or the dead-man's
switch dies quietly. **First step:** add the post-success write for one lane,
scrape it, hand-write the staleness rule, block that lane's credential, watch it
fire.

### Backbone — Signed auth registry + roster reconciliation

A declared registry of every lane auth mechanism with a per-lane liveness SLA; a
reconciliation check fails CI/doctor if a live lane has no registry entry or an
entry has no observed heartbeat. Closes the "a lane I forgot about" gap — the
three layers all alert against this declared roster (the
`striatum_expected_lane_count` denominator above derives from it).

### Rejected alternatives (traps)

- **Fever (throttle all lanes when one degrades)** — turns a one-lane fault into
  a system-wide outage; blast radius unacceptable.
- **Circadian token shredding / expiry shorter than the detection window** —
  aggressively short *synchronized* TTLs create a correlated failure mode (one
  renewal hiccup kills every lane at once); the cure manufactures a bigger
  outage. (Note: bounded short TTLs are RFC 0143's surface, not a detection
  mechanism.)
- **Sacrificial hair-trigger canary lane** — a decoy that does not share the
  prod lane's actual failure mode gives false confidence and page noise.

## Acceptance Criteria

- [ ] The `lane_provider_preflight_timeout` failure mode **and** the
  "stopped-trying-entirely" mode each raise an alert via Alertmanager → Slack
  `#proximal-alerts` within a documented detection window.
- [ ] Detection does not depend on organic traffic (a quiet lane is exercised).
- [ ] The alert/probe does **not share fate** with what it watches: a dead
  prober / crashed exporter / vanished metric series pages as loudly as a stale
  value (absence-of-series rule present).
- [ ] A game-day / synthetic test proves each layer's alert actually fires
  before a real incident would (an alert that has never fired is a liability).
- [ ] Per-lane attribution: alerts name the specific lane and link a runbook; no
  aggregate-only panel can average one dead lane into green.
- [ ] New metric names are added to the RFC 0137 exporter surface and documented;
  alert rules land in `halbritt/proximal/observability` (separate PR, referenced).

## Open Questions

1. **Layer ordering / MVP.** Layer 3 (dead-man's switch) is the cheapest and
   highest-fit backstop; Layer 1 (expiry trend) is the most on-target for the
   actual root cause. Ship which first, or both as the MVP and Layer 2 (active
   differential probe) as a follow-up?
2. **Prober location.** Does the active probe live in the daemon (a recovery-sweep-
   adjacent loop) or as an external systemd timer on `proximal` (outside the
   daemon's blast radius — the "who watches the watcher" argument)?
3. **Metric cardinality.** Per-lane × per-credential-kind series interact with the
   RFC 0137 cardinality budget (cf. `MetricsCardinalityClipped`). What is the
   lane/kind cap?
4. **Threshold source.** Is `striatum_lane_auth_staleness_threshold_seconds{lane}`
   derived from the credential lifetime automatically, or operator-declared in the
   registry?

## Domain Modeling

This introduces **lane auth liveness** as a *derived/observed property* of a lane
— a read-model projection over the auth preflight success path, not a new
aggregate. It is the credential-dimension sibling of RFC 0091's lane-health
classification and RFC 0077's MCP activity-liveness deadlines: the same
"absence-of-expected-success ⇒ unhealthy" shape applied to the auth boundary.
`auth_last_success` is a timestamp **value object**; the staleness/census alerts
are read-model rules. Per `docs/DDD.md § "Adding to the model"` (RFC 0019
precedent), no new write boundary or daemon method is required for Layers 1/3
beyond the exporter surface; Layer 2's prober, if daemon-resident, would emit a
typed `lane_auth_probe` **domain event**.

## Wider opportunity (optional follow-up, out of scope)

The pattern under all three layers is identical: *"a value that should
periodically step back up, but only ever declines."* Auth-TTL, backup recency,
the skill-mining backfill, Garage writes — the `proximal` host is full of
silent-rot-prone periodic jobs. A reusable `expected_periodic_success`
recording-rule macro in `proximal/observability`, with lane-auth as its first
consumer, would convert this one-off fix into a host-wide silent-failure immune
system. Track separately in `halbritt/proximal` if pursued.

## Pointers

- Auth path: `go/pkg/laneproviderauth/lane_provider_auth.go` (`DefaultTimeout`,
  `FailureTimeout` → `lane_provider_preflight_timeout`; offline `auth.json` probe)
- RPC / gate / drive: `go/pkg/reads/doctor_lane_provider_auth.go`,
  `go/pkg/mutations/supervision_provider_auth.go`, `go/pkg/cli/rundrive/rundrive.go`
- Metric surface: `go/pkg/metrics/registry.go` (`striatum_*`), exported via RFC 0137
- Alert rules (separate repo):
  `halbritt/proximal/observability/prometheus/prometheus/rules/striatum-alerting.rules.yml`
- Recent behavior fix that motivates the observability gap: `#556` / `#567`
  (`origin/main` @ `bb77ed75`)
- Adjacent decisions: RFC 0091, RFC 0131, RFC 0077, RFC 0143, RFC 0152

---
<sub>Scoped via the ADHD divergent-ideation skill (5 frames × 6 ideas → scored/clustered → top-3 deepened). Folds GH #569.</sub>

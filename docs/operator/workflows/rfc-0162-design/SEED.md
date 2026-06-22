# Design-Run Seed — RFC 0162 Lane-auth silent-failure observability

> This document is the **required input** for the RFC 0162 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed
> at `docs/rfcs/0162-lane-auth-silent-failure-observability.md` (status
> `proposed`) — read it in full as your primary source; this SEED carries the
> charter, restates the four Open Questions, and pins an operator
> anchor-verification table you must build on. Read this whole file and the RFC
> before producing any artifact.

## Charter — what this run must produce

This is a **design run**, not an implementation run. The deliverable is a
**falsifiable implementation spec** for RFC 0162: a concrete, buildable
specification that the `rfc-0162-build` run can execute contract-first (TDD),
produced by hardening the RFC against adversarial falsification.

The committed `PROPOSAL.md` MUST:

1. **Resolve all four Open Questions** (below) with a concrete, defensible
   decision each — *in-MVP / deferred*, *which mechanism*, *why*. A design run
   that leaves an Open Question unresolved has not cleared the gate.
2. **Close the codex-only preflight hole** (see anchor table) so the chosen
   absence-of-success signal is downstream of a *real* per-lane auth success for
   **every** lane provider, not just codex.
3. **Name the exact surfaces**: new metric names + closed label sets in
   `go/pkg/metrics/registry.go` (exported via the RFC 0137 exporter); the
   success-write / sampling site in `go/pkg/laneproviderauth/` or its caller; and
   the `proximal` alert rules with PromQL.
4. **State every load-bearing claim as a falsifiable assertion** paired with the
   named test / game-day step that would prove it false.
5. **Stay inside the Non-Goals and the local-first product boundary** (read-only
   telemetry; no preflight-behavior/timeout/trust-model change — that is
   RFC 0143; no hosted/cloud/push/remote-write; no per-repo private-data leak in
   labels). Honor the three rejected traps.

## Root reframe (do not lose this)

We **alert on the presence of errors, never on the absence of success.** A lane
that stops authenticating goes *quiet*, and quiet currently reads as healthy.
Every layer here is a mechanism to make *absence of expected success* loud. The
shared-fate corollary: the thing that watches must not die in the same silence —
**no series at all must page as loudly as a stale series.**

## The three layers (from the RFC, for reference)

- **Layer 1 (proactive) — expiry & renewal-health telemetry.** Per-lane
  `striatum_lane_cred_seconds_to_expiry{lane,kind,id}` (static tripwire at a
  lead-time threshold) + a negative-`delta` trend (refresh silently stopped) +
  `striatum_lane_cred_age_seconds`. ⭐ most on-target for the real root cause.
- **Layer 2 (active) — cross-lane differential + negative probe.** A prober runs
  the read-only auth check across all lanes on a fixed interval; pages only on
  K-of-M divergence from the sibling quorum; a negative probe asserts a deliberately
  invalid credential is still *rejected* (fail-open regression test).
- **Layer 3 (passive backstop) — dead-man's switch.** Each lane writes
  `striatum_lane_auth_last_success_timestamp_seconds{lane}` strictly downstream
  of a real auth success; the alert inverts the sense
  (`time() - last_success > staleness_threshold`) plus a census rule
  `count(...) < striatum_expected_lane_count`.
- **Backbone — signed auth registry + roster reconciliation:** the declared
  roster the layers alert against (the `expected_lane_count` denominator).

## Open Questions to resolve (design calls)

1. **Layer ordering / MVP.** Layer 3 is the cheapest backstop; Layer 1 is the
   most on-target. Ship which first — and exactly which compose the MVP vs.
   follow-up? (Operator note: Layer 1 is *provider-agnostic* — it can read any
   provider's credential expiry at rest — which interacts directly with the
   codex-only hole below; weigh that.)
2. **Prober location.** Layer 2's active probe in the daemon (recovery-sweep-
   adjacent loop) or as an external systemd timer on `proximal` (outside the
   daemon blast radius)? Name the unit/loop.
3. **Metric cardinality.** The per-lane × per-credential-kind series cap against
   the RFC 0137 budget (`MetricsCardinalityClipped`). Give the numeric cap +
   overflow behavior.
4. **Threshold source.** Is `striatum_lane_auth_staleness_threshold_seconds{lane}`
   auto-derived from the credential lifetime, or operator-declared in the
   registry backbone?

## Load-bearing risks (the RFC's "first step" for each — attack these)

- **L1 same-credential-at-rest:** the exporter must read the *same* credential
  the lane presents at runtime, not a fresh file the live process never reloaded
  (a healthy-looking gauge over a lane coasting to death).
- **L2 prober is a hidden Nth lane:** if its own creds/egress die, you get false
  divergence or a dead prober reading all-green — needs a dead-man's switch on
  the prober + an always-expected-fail synthetic lane.
- **L3 shared-fate:** the heartbeat must be downstream of a *real lane success*
  (not the prober's own auth), and "no series at all" (exporter crash, scrape
  down) must page as loudly as "stale series."

## Anchor verification against current `main` (operator pre-flight, 2026-06-22)

Verified against `~/git/striatum` @ `main`. Treat as ground truth; re-anchor the
spec to these.

| RFC claim / area | Status | Correction (current source) |
| --- | --- | --- |
| "each lane writes `auth_last_success` … in the preflight success path (`laneproviderauth`)" (Layer 3) | **DRIFTED — load-bearing** | `laneproviderauth.Check()` (`go/pkg/laneproviderauth/lane_provider_auth.go:178`) **only supports `provider == "codex"`** — anything else returns `FailureUnsupported` immediately. The gate caller `runSuperviseProviderAuthGate` (`go/pkg/mutations/supervision_provider_auth.go:30`) further gates on `AgentLoopMode == self-driving && provider == ProviderCodex` (line 38); for any other provider under `GateAuto` it returns `nil` **without calling Check at all** (lines 39-44). The natural success site `if result.Passed() { return nil }` is **line 56** — but it fires **only for codex self-driving lanes**. So a Layer-3 write there gives **no heartbeat for claude/agy/gemini lanes**. The spec MUST resolve this: a provider-agnostic success site, and/or lean on Layer 1 (which reads any provider's credential file directly). |
| Offline preflight reads `$CODEX_HOME/auth.json` as the lane user | **ACCURATE** | `checkCodexOfflineAuth` (`lane_provider_auth.go:497`) delegates a `cat` of `$CODEX_HOME/auth.json` via `sudo -n -u <lane> env -i` and validates a credential field. It checks *presence of a credential field*, **not expiry** — a present-but-expired token passes the offline probe. (Operator-confirmed live: this box's lane codex auth is fresh, but the lane *claude* OAuth token was found **expired ~14h with no signal** — the exact RFC 0162 failure mode. Layer 1 expiry telemetry would have caught it; the codex offline probe would not, because claude has no preflight.) |
| Metric surface `go/pkg/metrics/registry.go`, exported via RFC 0137 | **ACCURATE** | `registry.go` is the RFC 0137 Phase-C `Family` registry: each family carries `Name`, `Type` (gauge/counter/histogram), closed `Labels` (NAMES only, never values), and a `Classification` (`operational`/`provenance`/`forbidden`; Forbidden is refused at construction). New families are added here and are part of the **boot-time allowlist hash + CI guardrail** — adding a label is a deliberate, diff-reviewed manifest edit. Per-lane labels must fit the cardinality/redaction contract. |
| Layer 2 reuses the read-only check (`doctor lane_provider_auth`) | **ACCURATE (path)** | `go/pkg/reads/doctor_lane_provider_auth.go` exists; the gate lives in `go/pkg/mutations/supervision_provider_auth.go`; `run drive` in `go/pkg/cli/rundrive/rundrive.go`. Note the doctor check is **not** in the default `doctor` bundle output today — auth has no standing signal, which is the whole gap. |
| Alert-rules path `…/observability/prometheus/prometheus/rules/striatum-alerting.rules.yml` | **DRIFTED (typo)** | The real file is `halbritt/proximal` → `observability/prometheus/rules/striatum-alerting.rules.yml` (single `prometheus/`). It already carries the RFC 0137 striatum alert group (`NecrosisRate`, `DoctorRed`, `WedgeAgeTail`, `LivenessMarginCollapse`, `SupervisorOriginFlood`, `MetricsSnapshotStale`, `MetricsCardinalityClipped`, …) routed via Alertmanager → Slack `#proximal-alerts` (live). New lane-auth rules append to this group as a **separate `proximal` PR**. |
| Motivating fix `#556`/`#567` @ `origin/main` `bb77ed75` | **ACCURATE** | The offline-auth preflight + no-crash-loop behavior fix; it made preflight *behave* but added no way to *notice* a silently-dead auth. |

**Net design implication.** The RFC's headline absence-of-success framing is
sound, but its Layer-3 "write in the preflight success path" anchor is
**codex-only** in current source. The single most important decision for this
run is how the chosen MVP gives a **provider-agnostic** absence-of-success
signal — most likely Layer 1 expiry telemetry (provider-agnostic by reading the
credential file at rest) as the MVP backbone, with Layer 3's heartbeat either
moved to a provider-agnostic success site or scoped explicitly to codex with the
census rule covering the rest. Falsifiers should press exactly here.

---
<sub>Operator scaffold for the RFC 0162 falsification-gate design run. Lanes:
author=claude (holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>

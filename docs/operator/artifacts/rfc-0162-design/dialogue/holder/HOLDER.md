# HOLDER — RFC 0162 falsifiable implementation spec

author: holder-author-001

> This is the **leading proposal** for the RFC 0162 design run: a falsifiable,
> buildable spec for lane-auth silent-failure observability. It is the published
> claim the falsifiers will attack and the adjudicator will gate. It resolves all
> four Open Questions, re-anchors every load-bearing claim to current `main`,
> closes the codex-only preflight hole, names every metric/alert surface, and
> pairs each load-bearing claim with the named test or game-day step that would
> refute it. Read `SEED.md` and `docs/rfcs/0162-lane-auth-silent-failure-observability.md`
> first; this spec supersedes the RFC's sketch where the two disagree (the RFC is
> `proposed`, this spec re-anchors it).

## Root reframe (held)

We **alert on the absence of expected success, never on the presence of errors.**
A lane that stops authenticating goes *quiet*, and quiet currently reads as
healthy. Every mechanism below makes *absence of success* loud, and the thing
that watches must not die in the same silence: **no series at all must page as
loudly as a stale series.**

## The load-bearing architectural fact (re-anchored, not in the RFC)

The RFC 0137 exporter (`go/pkg/metrics/`) **folds every metric from the
daemon-owned PostgreSQL — the `striatumd.events` ledger and the `striatumd.*`
tables — once per recovery-sweep tick (`Collector.Refresh`, default 60s), and
serves the published snapshot on `/metrics` with zero DB or filesystem access on
the scrape path** (`collector.go:79` fold; `collector.go:622` `Handler` reads the
atomic snapshot pointer only). It does **not** read credential files, today, on
either path.

This single fact drives the design:

- A new metric is fed most naturally by an **event written to
  `striatumd.events`**, folded into a gauge/counter by `Collector.Refresh` —
  exactly like `livenessMargins`, `lifecycleEventCounts`, `leaseTransitionCounts`
  already are. Folding from the durable append-only ledger is what makes a counter
  tx-safe (a rolled-back launch never wrote the event) and restart-consistent
  (re-derived from history, never reset). We reuse that property; we do not invent
  a new collection substrate.
- The pure classifier `laneproviderauth.Check()` (`lane_provider_auth.go:178`) has
  **no DB handle** and must stay pure. Any heartbeat/expiry write therefore lives
  in the **caller**, not in `Check`.
- A label whose value domain is not a compile-time enum (here `lane`) is admitted
  exactly the way `doctor_problems{class}` already is: bounded by a declared
  source **and** backstopped by the per-family series budget
  (`budget.go:applySeriesBudget`), with overflow made visible through the existing
  `striatum_metrics_cardinality_clipped_total` counter.

## Re-anchor against current source (corrects the RFC and confirms the SEED table)

| Claim | Status on `main` | Spec consequence |
| --- | --- | --- |
| "each lane writes `auth_last_success` in the preflight success path (`laneproviderauth`)" | **DRIFTED — codex-only.** `Check()` returns `FailureUnsupported` for any `provider != "codex"` (`lane_provider_auth.go:185`); the gate caller `runSuperviseProviderAuthGate` returns `nil` **without calling `Check`** unless `AgentLoopMode == self-driving && provider == codex` (`supervision_provider_auth.go:38-44`). The only real success site is `if result.Passed() { return nil }` at `supervision_provider_auth.go:56` — codex self-driving only. | Layer 3's positive heartbeat is **codex-scoped by construction** and the spec says so. Provider-agnostic coverage comes from Layer 1 (reads the file at rest) + the census rule, **not** from a fake heartbeat for providers that never ran a successful `Check`. |
| Offline probe reads `$CODEX_HOME/auth.json` as the lane user via `sudo -n -u <lane> env -i cat -- <path>` | **ACCURATE** (`lane_provider_auth.go:497`, `:522`). It checks *presence of a credential field*, **not expiry** — a present-but-expired token passes. | Layer 1's sampler **reuses this exact read shape** (`ResolveAuthHome`/`SanitizeEnv`/`sudo -n -u` delegation) so it reads the *same* file the lane presents, then additionally extracts the credential's expiry. |
| Metric surface `go/pkg/metrics/registry.go`, exported via RFC 0137; boot-time allowlist hash + CI guardrail | **ACCURATE.** `DefaultRegistry()` (`registry.go:149`) is the closed family set; adding a family/label regenerates `metrics_allowlist.json` and is a deliberate, diff-reviewed manifest edit. Labels are NAMES only; no value is ever interpolated from a repo/run/session id, path, branch, sha, prompt, or byline (`render.go:20-23`). | New families land in `DefaultRegistry()` + `render.go`, all `ClassificationOperational`, label names `{lane,kind}` only. The manifest-hash change is part of the deliverable. |
| Layer 2 reuses `doctor lane_provider_auth` | **ACCURATE path** (`reads/doctor_lane_provider_auth.go`), but the doctor check is **codex-only** (`:32`) and is **not** in the default doctor bundle — auth has no standing signal today. | Layer 2 (deferred) generalizes this read; the MVP does not depend on it. |
| Alert-rules path | **DRIFTED (typo).** Real file: `halbritt/proximal` → `observability/prometheus/rules/striatum-alerting.rules.yml` (single `prometheus/`); the RFC's `.../prometheus/prometheus/rules/...` has a stray segment. It already carries the `striatum` alert group routed Alertmanager → Slack `#proximal-alerts`. | New rules append to that group in a **separate `proximal` change**; PromQL named below. |

**Net:** the headline absence-of-success framing is sound, but the Layer-3
"write in the preflight success path" anchor is **codex-only**. The single most
important decision (OQ1) is how the MVP gives a **provider-agnostic**
absence-of-success signal. Answer: **Layer 1 (credential expiry at rest, provider-
agnostic) is the MVP backbone**; Layer 3 ships beside it as the codex-scoped
corroborator it can honestly be; the census rule carries the non-codex lanes.

---

## Open Question resolutions (all four, with decision + why)

### OQ1 — Layer ordering / MVP

**Decision. MVP = Layer 1 (expiry & renewal-health telemetry) + Layer 3
(dead-man's-switch heartbeat) + the Backbone roster + the absence-of-series
census rule. Layer 2 (active differential + negative probe) is the follow-up.**
Within the MVP, **Layer 1 is the provider-agnostic backbone** and **Layer 3 is
explicitly codex-scoped** (it fires only where a real `Check` success exists).

**Why Layer 1 is the backbone, not the deferral.** The motivating incident
(SEED, operator-confirmed live): the lane **claude** OAuth token was expired ~14h
with **no signal**. Claude has no preflight (`Check` is codex-only), so a naive
Layer-3 heartbeat would never fire for claude — it would look permanently dead or,
if absence were ignored, permanently healthy. **Layer 1 reads claude's token file
at rest, extracts its expiry, and is provider-agnostic** — it would have fired the
`seconds_to_expiry < lead` tripwire days before the lapse, and the
negative-`delta` trend the moment the renewer stopped. It targets the actual root
cause (a renewal that silently stopped) rather than the symptom (a credential that
has already lapsed).

**Why Layer 3 still ships in the MVP (and why it is codex-scoped).** Where a real
auth round-trip *does* happen (codex self-driving gate), the heartbeat is the
cheapest possible positive proof that the lane can actually authenticate — not
just that a file at rest looks unexpired (a present token can be revoked
server-side; Layer 1 cannot see that). It is one event write on an existing
success path + one fold + one staleness rule. We ship it for codex and **state
the boundary**: non-codex lanes get Layer 1 + census, never a synthetic
heartbeat.

**Why Layer 2 is the follow-up.** The active prober is the heaviest and the only
component that is *itself a lane* (its own creds/egress can rot), and it is the
only thing that can give non-codex lanes a *positive* success signal. Its
shared-fate hazards (dead prober reading all-green, fail-open regression) demand a
dead-man's switch + an always-expected-fail synthetic lane, which is more surface
than the MVP needs to close the motivating gap. It is named and contracted below
so the metric surface is chosen once, but it is deferred.

**Falsifiable assertion FA-1.** *The motivating incident (a silently-expired
non-codex lane credential) is caught by the MVP without any positive heartbeat for
that provider.* **Refuted if:** blocking/expiring the claude lane's token does not
fire `LaneCredExpirySoon`/`LaneCredRenewalStalled`/`LaneAuthSeriesMissing` within
the documented window — or if catching it requires a `auth_last_success` series
for claude. **Game-day GD-1** below proves it.

### OQ2 — Prober location

**Decision. Two distinct collectors, two locations:**

- **Layer 1 sampler (MVP) lives IN the daemon**, as a best-effort step of
  `Collector.Refresh` on the recovery-sweep cadence (`collector.go:79`), bounded
  by a per-lane timeout in the spirit of the existing `doctorFoldTimeout = 15s`
  (`collector.go:258`). It is a **read-only file sample with no provider round-
  trip** (it does *not* call the provider API), so it carries none of the active-
  probe blast radius. It reuses `checkCodexOfflineAuth`'s `sudo -n -u <lane> env -i
  cat -- <path>` delegation to read the lane-owned file as the lane user, then
  extracts expiry, then writes a `lane.cred_expiry_sampled` event.
- **Layer 2 active prober (follow-up) lives OUTSIDE the daemon**, as a systemd
  timer on `proximal`: unit **`striatum-lane-auth-prober.timer` → `.service`**
  (`User=halbritt`, `Restart=on-failure`, loopback-only), exactly the shape of the
  existing `proximal` service units (cf. `whisper-stt.service`/`praxis-stt-shim`).
  It performs the real read-only auth check across all lanes + a negative probe,
  and writes its results back through the daemon RPC boundary (or a Prometheus
  textfile-collector file) — it never mutates daemon state directly.

**Why split.** "Who watches the watcher" applies to the *active* prober: a probe
that performs real auth round-trips and the fail-open negative-probe must **not
share fate** with the daemon, or a wedged daemon yields a prober reading all-green.
External systemd timer + its own dead-man's switch is the correct location for
that. But Layer 1's sampler is a pure file read with no behavior risk; co-locating
it with the existing DB fold is cheaper, and the daemon already performs the
identical cross-user `sudo` read in `checkCodexOfflineAuth`. The shared-fate
concern for the *daemon-resident* sampler is fully discharged by absence-detection:
if the daemon dies, `/metrics` is unscrapeable → `up{job="striatumd"}==0` and the
existing `MetricsSnapshotStale` (snapshot_age climbing) page; if the sampler step
alone fails, it degrades the tick to `tick_status=partial` (alertable today) and
the per-lane census rule fires. A dead daemon **cannot** make Layer 1 read falsely
green.

**Falsifiable assertion FA-2.** *A dead daemon / dead sampler pages; it never
reads green.* **Refuted if:** killing the daemon produces no page within 5m, or a
sampler-step failure leaves the expiry gauge serving a last-good value with no
`tick_status=partial` and no census alert. **Game-day GD-4** proves it.

### OQ3 — Metric cardinality

**Decision. Label set is `{lane, kind}` only (the RFC's `id` is dropped). Per-
family series budget = 32**, collapsing overflow onto `{lane="other"}` via the
existing `applySeriesBudget` (`budget.go:35`), incrementing
`striatum_metrics_cardinality_clipped_total{family="..."}`.

- `kind` is a **closed enum `{oauth, api_key}`** (≤ 2 values).
- `lane` is the **declared-roster slug = the lane OS user** (`codex`, `claude`,
  `agy`, `gemini`, …) — a small, host-level, operator-declared, **non-private**
  set, **never** the run-scoped `lane_id`, and **never** a repo/run/session id,
  path, branch, sha, or byline. The fold validates each event's lane against the
  declared roster and folds an unknown value to `{lane="other"}` (defense-in-depth
  identical to `doctor_problems{class}`).
- 16 lanes × 2 kinds = 32 → a budget of 32 caps emitted series at 33 (incl.
  `other`) regardless of how many distinct lanes arrive. This stays well inside the
  RFC 0137 cardinality budget.

**Why drop `id`.** A credential `id` (account id / token id) is a higher-cardinality,
potentially repo-linkable value that would violate the RFC 0137 redaction contract
and blow the series budget. `{lane, kind}` is sufficient for per-lane attribution;
identity beyond the roster slug never reaches the wire.

**Falsifiable assertion FA-3.** *lane×kind series are hard-bounded at 33 and any
overflow is visible.* **Refuted if:** a test feeding 100 synthetic lanes produces
> 33 series for any family, or produces a clip with no increment of
`striatum_metrics_cardinality_clipped_total`. **Test `TestLaneCredSeriesBudget`**.

### OQ4 — Staleness-threshold source

**Decision. Operator-declared in the Backbone roster, exported as a gauge —
NOT auto-derived from the observed credential lifetime.** Two per-lane thresholds
are declared in the roster and folded onto the wire:

- `striatum_lane_auth_staleness_threshold_seconds{lane}` (Layer 3; default ≈ 1.5×
  credential lifetime)
- `striatum_lane_cred_expiry_lead_seconds{lane}` (Layer 1 tripwire lead; default ≈
  3× renewal cadence)

A sensible default derived from the *declared* lifetime is emitted so a newly-
rostered lane is covered without hand-tuning, but the **source of authority is the
declared roster**, which the operator diff-reviews.

**Why not auto-derive from observed lifetime.** Auto-derivation is **circular**:
the very failure mode (renewal stopped) corrupts the observed-lifetime signal, so
a threshold derived from it lets a degrading credential silently move its own
goalposts — the alert relaxes exactly as the thing it watches rots. The roster is
already required (it provides the `expected_lane_count` denominator), so the
threshold lives in the same diff-reviewed artifact the census rule reads.
Exporting it as a metric means retuning or adding a lane is a **roster edit, not a
`proximal` rule edit** (the alert reads the threshold via `on(lane) group_left()`).

**Falsifiable assertion FA-4.** *A degrading credential does not shrink its own
threshold.* **Refuted if:** a test in which a lane's observed lifetime shrinks
causes its exported threshold to shrink. **Test `TestThresholdFromRosterNotObserved`**.

---

## Closing the codex-only preflight hole (charter item 2 — exact write site)

**The post-success write for Layer 3 lives in the gate caller, at the success
branch, never in the pure `Check`:**

- **Site:** `go/pkg/mutations/supervision_provider_auth.go:56`, inside
  `if result.Passed() { … return nil }`. On a real `Passed()` result, emit a
  `lane.auth_success` domain event into `striatumd.events` with payload
  `{lane_user, provider, kind, repository_id, run_id}`. Mirror the write at the
  other real-success sites that run `Check`: `doctor lane_provider_auth`
  (`reads/doctor_lane_provider_auth.go:51`) and `run drive`
  (`cli/rundrive/rundrive.go`).
- **The event write must be best-effort and side-effect-free on the gate
  verdict**: a failed event write is logged and swallowed; it can **never** change
  whether a lane launches (FA-7 below). This keeps the change read-only telemetry
  over the auth boundary (Non-Goal compliance).
- **The fold:** `Collector.Refresh` adds a best-effort fold
  `SELECT payload_json->>'lane_user', MAX(EXTRACT(EPOCH FROM created_at)) FROM
  striatumd.events WHERE event_type='lane.auth_success' GROUP BY 1`, mapping each
  lane_user to its roster slug → `striatum_lane_auth_last_success_timestamp_seconds{lane}`.
  A lane_user not in the roster folds to `{lane="other"}`.

**What the absence/census rule does for a provider with no preflight.** Because
the heartbeat is codex-only, the **census rule
`count(striatum_lane_cred_seconds_to_expiry) < striatum_lane_auth_expected_count`**
(Layer 1 series, which *are* provider-agnostic) is what guarantees a non-codex
lane that vanishes pages. Non-codex lanes are covered by: (1) Layer 1
`seconds_to_expiry`/`delta` (the proactive catch), and (2) the census absence
rule. They are deliberately **not** given a synthetic `auth_last_success` — a
heartbeat that is not downstream of a real success is a lie, and a lie that reads
green is worse than no signal. Layer 2 (follow-up) is what eventually earns
non-codex lanes a *positive* heartbeat.

**Falsifiable assertion FA-5.** *The heartbeat is emitted strictly downstream of a
real `result.Passed()`, and only for providers that actually run `Check`.*
**Refuted if:** an `auth_last_success` series appears for a lane whose `Check`
never returned `Passed()` (e.g. a claude lane), or if the event is emitted on the
`supported==false` early return (`supervision_provider_auth.go:39-44`). **Test
`TestAuthSuccessEventOnlyOnPassedCodex`**.

---

## Exact metric surface (this repo: `go/pkg/metrics/`)

All families `ClassificationOperational`; added to `DefaultRegistry()`
(`registry.go:149`) and rendered in `render.go`; the change regenerates
`metrics_allowlist.json` and updates the boot-time allowlist hash (diff-reviewed,
CI-guarded). Label names below are the **complete closed set** for each family.

**MVP families:**

| Family | Type | Labels | Fed by | Layer |
| --- | --- | --- | --- | --- |
| `striatum_lane_cred_seconds_to_expiry` | gauge | `lane,kind` | `lane.cred_expiry_sampled` event fold (latest per lane,kind) | L1 |
| `striatum_lane_cred_age_seconds` | gauge | `lane,kind` | same event (`now - issued_at`) | L1 |
| `striatum_lane_auth_last_success_timestamp_seconds` | gauge | `lane` | `lane.auth_success` event fold (MAX) | L3 (codex) |
| `striatum_lane_auth_expected_count` | gauge | *(none)* | Backbone roster size | census denom |
| `striatum_lane_auth_staleness_threshold_seconds` | gauge | `lane` | roster (OQ4) | L3 |
| `striatum_lane_cred_expiry_lead_seconds` | gauge | `lane` | roster (OQ4) | L1 |

**Follow-up families (named now, deferred):** `striatum_lane_auth_probe_success`
gauge `{lane}` (1/0 last probe), `striatum_lane_auth_probe_last_run_timestamp_seconds`
gauge `{lane}` (prober dead-man), `striatum_lane_auth_negative_probe_rejected`
gauge `{lane}` (1 = invalid credential correctly rejected; **0 = fail-open, page
immediately**).

**Provider-specific expiry extraction (honest boundary).** The sampler parses
expiry per credential shape: codex OAuth `id_token` JWT `exp` (or a
`tokens.expiry`/`expires_at` field when present); claude OAuth token `expiresAt`.
**`kind="api_key"` credentials have no expiry** → they emit **no**
`seconds_to_expiry` series and rely on Layer 3 (codex) / Layer 2 (deferred) / the
census rule for coverage. This gap is explicit, not silent.

**Falsifiable assertion FA-6 (L1 same-credential).** *The sampler reads the SAME
credential the lane presents at runtime, not a stand-in at rest.* **Refuted if:**
the sampler reads any path the live lane process does not resolve. The sampler
**must** resolve the path via the identical `ResolveAuthHome`/`SanitizeEnv` +
`sudo -n -u <lane>` shape as `checkCodexOfflineAuth` (`lane_provider_auth.go:497`).
**Test `TestCredExpirySamplerReadsLanePresentedCredential`** (a decoy file at a
different path must NOT move the gauge). **Game-day GD-2**: rotate the lane's live
token; `seconds_to_expiry` must jump within one sweep.

## Exact alert surface (separate repo: `halbritt/proximal`)

Append to the existing `striatum` group in
`observability/prometheus/rules/striatum-alerting.rules.yml` (corrected path;
routed Alertmanager → Slack `#proximal-alerts`). Every alert carries
`labels: {severity, lane: "{{ $labels.lane }}"}` and a runbook annotation — no
aggregate-only rule, so one dead lane can never average into green.

```yaml
# L1 proactive — fires WHILE the lane is still healthy
- alert: LaneCredExpirySoon
  expr: striatum_lane_cred_seconds_to_expiry
          < on(lane) group_left() striatum_lane_cred_expiry_lead_seconds
  for: 15m
- alert: LaneCredRenewalStalled            # renewer silently stopped; days early
  expr: delta(striatum_lane_cred_seconds_to_expiry[6h]) < 0
  for: 2h
# L3 passive backstop (codex lanes that run a real Check)
- alert: LaneAuthHeartbeatStale
  expr: time() - striatum_lane_auth_last_success_timestamp_seconds
          > on(lane) group_left() striatum_lane_auth_staleness_threshold_seconds
  for: 10m
# Census / absence-of-series — covers ALL lanes incl. non-codex
- alert: LaneAuthSeriesMissing
  expr: count(striatum_lane_cred_seconds_to_expiry) < striatum_lane_auth_expected_count
  for: 10m
# Shared-fate — exporter/daemon down pages as loudly as a stale value
- alert: LaneAuthExporterDown
  expr: absent(striatum_lane_cred_seconds_to_expiry) or up{job="striatumd"} == 0
  for: 5m   # complements the existing MetricsSnapshotStale rule
```

**`LaneCredRenewalStalled` window caveat (stated, not hidden).** The `[6h]`
window must exceed ≈1.5× the *longest-cadence* rostered lane's renewal interval; a
single rule cannot carry a per-lane window. Document the assumption in the runbook;
**FA / GD-3** below tests it. If lane cadences diverge widely, split into per-
cadence recording rules (follow-up).

---

## Backbone roster (the declared denominator + thresholds)

A declared, operator-owned roster of every lane auth mechanism, host-level (keyed
by lane OS user), each entry:
`{lane, provider, kind, credential_path_template, staleness_threshold_seconds,
expiry_lead_seconds, renewal_cadence_seconds}`. To avoid a schema migration for
the MVP it is a daemon-config file (e.g. `lane-auth-roster.json` under the daemon
config dir), read at fold time (small, host-local, daemon-readable) — mirroring how
the per-repo consent flag avoided a migration by living in `settings_json`
(`collector.go:180`). It folds to `striatum_lane_auth_expected_count`,
`striatum_lane_auth_staleness_threshold_seconds{lane}`, and
`striatum_lane_cred_expiry_lead_seconds{lane}`.

**Reconciliation (doctor check).** A new doctor check flags: (a) a live lane (one
that emitted a `lane.auth_success` or `lane.cred_expiry_sampled` event within the
sweep window) with **no** roster entry, and (b) a roster entry with **no** observed
sample within its declared SLA. This closes the "a lane I forgot about" gap and
keeps `expected_count` honest. Surfaced via the existing `striatum_doctor_problems{class}`
family — no new wire surface.

**Falsifiable assertion FA-7 (no behavior change — Non-Goal compliance).**
*Nothing in this spec changes preflight behavior, timeouts, or the credential
trust model.* **Refuted if:** any existing `laneproviderauth` / gate test changes
verdict, or a sampler/heartbeat/event-write failure flips a gate decision or alters
a timeout. The heartbeat write and the sampler are on success/observation paths
only and swallow their own errors. **Tests:** the existing `laneproviderauth` and
`supervision_provider_auth` suites pass **unchanged**; add
`TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict`.

---

## Game-day proof (each alert fires before a real incident)

An alert that has never fired is a liability. Each MVP alert has a synthetic
trigger that must fire within its documented window:

- **GD-1 (the motivating incident):** block/expire the **claude** lane token.
  Expect `LaneCredExpirySoon` then `LaneAuthSeriesMissing` to fire; expect **no**
  `LaneAuthHeartbeatStale` for claude (claude has no heartbeat — documented, not a
  bug). *Refuted if claude's silent expiry produces no page.*
- **GD-2 (L1 same-credential):** rotate a lane's live token → `seconds_to_expiry`
  jumps within one sweep; touch only a decoy file → gauge does not move.
- **GD-3 (renewal-stalled, days early):** freeze the renewer (stop advancing
  `not_after`) while the token is still valid → `LaneCredRenewalStalled` fires
  *before* `seconds_to_expiry` crosses the lead tripwire.
- **GD-4 (shared-fate):** `systemctl stop` the daemon → `up{job="striatumd"}==0`,
  `MetricsSnapshotStale`, and `LaneAuthExporterDown` all fire within 5m.
- **GD-5 (codex heartbeat):** block the codex lane's `auth.json` →
  `LaneAuthHeartbeatStale{lane="codex"}` fires after the staleness threshold;
  restore → it clears on the next real gated launch.

## Product-boundary & rejected-trap compliance (checklist)

- **Read-only telemetry over the auth boundary.** Sampler reads files + emits
  events; heartbeat writes on an existing success path; no provider round-trip in
  the MVP (Layer 2's active probe deferred). ✔
- **No preflight behavior / timeout / trust-model change** (that is RFC 0143). FA-7. ✔
- **Local-first, pull-only.** Metrics via the existing RFC 0137 exporter
  (loopback/tailnet pull); alerts via the existing `proximal` Prometheus →
  Alertmanager → Slack. No hosted/cloud/push/remote-write. ✔
- **No per-repo private-data leak.** `lane` = roster slug (OS user); `kind` =
  closed enum; `id` dropped; no repo/run/session id, path, sha, prompt, or byline
  on the wire. Respects the RFC 0137 redaction + cardinality contract. ✔
- **Rejected traps honored.** No fever/throttle (telemetry never throttles a
  lane); no circadian short-TTL (that is RFC 0143); no sacrificial canary — the
  deferred Layer-2 negative probe is an *always-expected-fail synthetic assertion*,
  not a decoy prod lane. ✔

## Build order for `rfc-0162-build` (contract-first / TDD)

1. Backbone roster file + fold → `expected_count` + threshold gauges; doctor
   reconciliation check (FA-4, FA reconciliation). Smallest, unblocks census.
2. Layer 1 sampler (reuse `checkCodexOfflineAuth` read shape) + `lane.cred_expiry_sampled`
   event + fold → `seconds_to_expiry`/`age` gauges (FA-3, FA-6; GD-2/GD-3).
3. Layer 3 heartbeat event at `supervision_provider_auth.go:56` (+ doctor/drive
   success sites) + fold (FA-5, FA-7; GD-5).
4. `proximal` rules (separate change): the five MVP alerts (GD-1/GD-4).
5. Layer 2 prober (`striatum-lane-auth-prober.timer`, follow-up): active check +
   negative probe + prober dead-man + synthetic always-fail lane.

---
<sub>Holder proposal for the RFC 0162 falsification-gate design run. Re-anchored
to `~/git/striatum` @ current source. The adjudicator's collaboration ledger —
not falsifier completion — decides whether the gate clears.</sub>

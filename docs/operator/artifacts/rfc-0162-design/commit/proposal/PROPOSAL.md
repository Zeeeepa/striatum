---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
author: "committer-author-002"
title: "RFC 0162 lane-auth silent-failure observability — falsification-cleared implementation spec"
run_id: "run_623ba123a529b1c867186c759ac02015"
cycle: 2
inputs:
  - "docs/operator/workflows/rfc-0162-design/SEED.md"
  - "docs/rfcs/0162-lane-auth-silent-failure-observability.md"
  - "docs/operator/artifacts/rfc-0162-design/dialogue/holder/HOLDER.md"
  - "docs/operator/artifacts/rfc-0162-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md"
  - "docs/operator/artifacts/rfc-0162-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_2.md"
  - "docs/operator/artifacts/rfc-0162-design/dialogue/falsifier_1/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0162-design/dialogue/falsifier_2/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0162-design/DECISION_override_commit_fold_f1_f2.md"
---

# PROPOSAL — RFC 0162 lane-auth silent-failure observability (falsification-cleared implementation spec)

author: committer-author-002

> This is the **committed deliverable** of the RFC 0162 design run: the
> falsifiable, buildable implementation spec the `rfc-0162-build` run will execute
> contract-first (TDD). It is the Holder's `HOLDER.md` hardened by folding the two
> material falsifier findings (F1, F2) that the adjudicator recorded across cycles
> 1–2, per the binding fold instructions in
> `DECISION_override_commit_fold_f1_f2.md`. Read `SEED.md` and
> `docs/rfcs/0162-lane-auth-silent-failure-observability.md` first; this spec
> supersedes the RFC's sketch where the two disagree (the RFC is `proposed`; this
> spec re-anchors it to current source).

## Gate disposition (why this is committed, and what it folds)

The cycle-2 collaboration ledger returned **`needs_revision`** (F1 + F2 stood
unrebutted). The operator decision **`DECISION-rfc-0162-design-override-fold-f1-f2`**
(`accepted_with_follow_up`, `owner: human`) overrides that verdict and authorizes
this committer to publish **provided F1 and F2 are folded as binding criteria**,
each per its `closest_acceptable_answer`. The override rationale: cycle-2
`needs_revision` was a template limitation (the revision edge routed to a
falsifier, never back to the Holder, so `HOLDER.md` was never revised) plus two
honest over-claim findings with concrete narrowing fixes — **not** fundamental
defects. All load-bearing design is credited sound and carried forward intact.

**This spec therefore:**

- **Folds F1** (census coverage) — adds the per-lane roster-presence vector
  `striatum_lane_auth_expected{lane,provider,kind}`, an observed-sample presence
  vector `striatum_lane_cred_sample_present{lane,kind}`, and rewrites the absence
  rule with `unless on(lane)` semantics that **preserve the `lane` label**; retires
  the scalar `expected_count`; and **narrows the MVP** to expiry telemetry for
  expiring file-backed OAuth credentials, marking non-codex `api_key` lanes a
  deferred/accepted risk for *positive* validity while keeping them inside the
  *absence* census so they cannot go silent without a lane-labeled page.
- **Folds F2** (resolver resolution) — replaces "reuse the codex read shape" with a
  **provider-agnostic credential-resolver contract** that names the exact runtime
  source + precedence per in-scope provider and **fails closed** into a pageable
  `resolver_mismatch` (never a green gauge) when the runtime source cannot be
  proven; narrows the L1 same-credential claim to credentials the resolver can
  prove.
- **Carries intact** the credited strengths (both falsifiers and the adjudicator
  credited these, unattacked): the **correctly-refused fake non-codex heartbeat**
  (FA-5), **OQ4** threshold source (operator-declared, FA-4), **OQ3** numeric
  cardinality cap (FA-3), and **FA-7** Non-Goal / RFC 0143 boundary compliance.

## Root reframe (held)

We **alert on the absence of expected success, never on the presence of errors.**
A lane that stops authenticating goes *quiet*, and quiet currently reads as
healthy. Every mechanism below makes *absence of success* loud, and the thing that
watches must not die in the same silence: **no series at all must page as loudly as
a stale series.** The folded design tightens this: a non-codex lane whose
credential vanishes, *or* whose runtime credential source cannot be proven, now
pages **with its lane named** — it can never read green from a fallback path.

## The load-bearing architectural fact (re-anchored; not in the RFC)

The RFC 0137 exporter (`go/pkg/metrics/`) **folds every metric from the
daemon-owned PostgreSQL — the `striatumd.events` ledger and the `striatumd.*`
tables — once per recovery-sweep tick (`Collector.Refresh`, default 60s), and
serves the published snapshot on `/metrics` with zero DB or filesystem access on
the scrape path** (`collector.go` fold; `collector.go` `Handler` reads the atomic
snapshot pointer only). It does **not** read credential files today, on either
path.

This single fact drives the design:

- A new metric is fed most naturally by an **event written to `striatumd.events`**,
  folded into a gauge/counter by `Collector.Refresh` — exactly like
  `livenessMargins`, `lifecycleEventCounts`, `leaseTransitionCounts` already are.
  Folding from the durable append-only ledger is what makes a counter tx-safe (a
  rolled-back launch never wrote the event) and restart-consistent (re-derived from
  history, never reset). We reuse that property; we do not invent a new collection
  substrate.
- The pure classifier `laneproviderauth.Check()`
  (`go/pkg/laneproviderauth/lane_provider_auth.go:178`) has **no DB handle** and
  must stay pure. Any heartbeat/expiry/sample write therefore lives in the
  **caller**, not in `Check`.
- A label whose value domain is not a compile-time enum (here `lane`) is admitted
  exactly the way `doctor_problems{class}` already is: bounded by a declared source
  **and** backstopped by the per-family series budget (`budget.go:35
  applySeriesBudget`), with overflow made visible through the existing
  `striatum_metrics_cardinality_clipped_total` counter.

## Re-anchor against current source (confirms the SEED table; verified @ `main`)

| Claim | Status on `main` | Spec consequence |
| --- | --- | --- |
| "each lane writes `auth_last_success` in the preflight success path" | **DRIFTED — codex-only.** `Check()` returns `FailureUnsupported` for any `provider != "codex"` (`lane_provider_auth.go:185-191`); the gate caller `runSuperviseProviderAuthGate` returns `nil` **without calling `Check`** unless `AgentLoopMode == self-driving && provider == codex` (`supervision_provider_auth.go:38-44`). The only real success site is `if result.Passed() { return nil }` at `supervision_provider_auth.go:56` — codex self-driving only. | Layer 3's positive heartbeat is **codex-scoped by construction** (FA-5). Provider coverage comes from Layer 1 expiry telemetry (file-backed, resolver-proven) + the per-lane census, **never** a synthetic heartbeat. |
| Credential resolution reuses `ResolveAuthHome` / sanitized env / `sudo -n -u <lane>` | **DRIFTED — codex-only (load-bearing for F2).** `ResolveAuthHome` honors `CODEX_HOME` only for codex (`lane_provider_auth.go:356-360`) and falls back to **bare `HOME`** for every other provider (`:361-366`); the sanitized preflight env allowlist (`:340-352`) carries **no** provider-specific credential-home/token-path variable, and `providerAuthPreflightEnv` applies that allowlist *after* merging `LaunchEnv` (`supervision_env.go`), so a provider-specific credential location can be stripped before any resolver sees it. There is **no** non-codex credential-path knowledge in the tree (only codex `~/.codex`, `:363`). | The MVP **must not** "reuse the codex shape" for non-codex lanes — it would resolve `$HOME/…`, not where the claude CLI resolves its token. F2's resolver contract + fail-closed `resolver_mismatch` is the fold (below). |
| Offline probe reads `$CODEX_HOME/auth.json` as the lane user; checks *presence*, not expiry | **ACCURATE** (`lane_provider_auth.go:497`, `:522`). A present-but-expired token passes the offline probe. | Layer 1's sampler additionally extracts expiry; it reads the credential the **resolver contract** proves, not whatever the codex shape happens to return for a non-codex provider. |
| Metric surface `go/pkg/metrics/registry.go`; boot-time allowlist hash + CI guardrail | **ACCURATE.** `DefaultRegistry()` (`registry.go:149`) is the closed family set; adding a family/label regenerates `metrics_allowlist.json` and is a deliberate, diff-reviewed manifest edit. Labels are NAMES only; no value is interpolated from a repo/run/session id, path, branch, sha, prompt, or byline. | New families land in `DefaultRegistry()` + `render.go`, all `ClassificationOperational`, label names from the closed sets below only. The manifest-hash change is part of the deliverable. |
| Layer 2 reuses `doctor lane_provider_auth` | **ACCURATE path** (`reads/doctor_lane_provider_auth.go`), but the doctor check is codex-only and **not** in the default doctor bundle. | Layer 2 (deferred) generalizes this read; the MVP does not depend on it. |
| Alert-rules path | **DRIFTED (typo).** Real file: `halbritt/proximal` → `observability/prometheus/rules/striatum-alerting.rules.yml` (single `prometheus/`). It carries the `striatum` alert group routed Alertmanager → Slack `#proximal-alerts`. | New rules append to that group in a **separate `proximal` change**; PromQL named below. |

**Net.** The headline absence-of-success framing is sound. The single most
important decision (OQ1) is how the MVP gives a **lane-attributed** absence signal
for *every* rostered lane and an **expiry** signal for every expiry-capable,
resolver-proven credential — without synthesizing a fake heartbeat and without ever
reading green off an unproven fallback path. The answer below: Layer 1 (expiry,
OAuth) + the per-lane roster/sample census (all lanes) + codex-scoped Layer 3 +
fail-closed resolver, with Layer 2 the follow-up.

---

## Open Question resolutions (all four — decision + why)

### OQ1 — Layer ordering / MVP

**Decision. MVP =**

1. **Backbone roster** → per-lane expected vector `striatum_lane_auth_expected{lane,provider,kind}` + the OQ4 threshold gauges (the census denominator and the alert thresholds).
2. **Layer 1 (expiry & renewal-health telemetry)** — provider-agnostic *for resolver-proven, expiry-capable (OAuth) file-backed credentials*; emits `seconds_to_expiry` / `age` and the `sample_present` coverage signal; fails closed to `resolver_mismatch`.
3. **Layer 3 (dead-man's-switch heartbeat)** — explicitly **codex-scoped** (fires only where a real `Check().Passed()` exists).
4. **The per-lane absence/census rule** — covers **all** rostered lanes (incl. non-codex `api_key`) via `expected unless sample_present`, lane label preserved.

**Layer 2** (active differential + negative probe) is the **follow-up** — it is the
only component that can give non-codex lanes a *positive validity* signal, and it is
itself a lane (its own creds/egress can rot), so it is deferred behind its own
dead-man's switch (OQ2).

**Scope narrowing (F1 fold — binding).** The MVP delivers:

- **Positive expiry telemetry** for every **expiry-capable, resolver-proven**
  file-backed credential (`kind="oauth"`: codex, claude). This is provider-agnostic
  *within that class* and is the direct fix for the motivating incident.
- **Lane-attributed absence detection** for **every** rostered lane, including
  non-codex `api_key` lanes: a healthy lane emits `sample_present=1`; a lane whose
  credential vanishes or whose runtime source cannot be proven emits no
  `sample_present` (or `resolver_mismatch=1`) → the census pages **with the lane
  named**.
- The MVP does **not** claim a *positive validity* signal for non-codex `api_key`
  lanes (an `api_key` that is present-but-revoked server-side is not caught until
  Layer 2). This is an **explicit accepted/deferred risk**, bounded by the absence
  census so the lane cannot go *silent* without a page when its credential
  disappears. We no longer assert the MVP is "provider-agnostic for every lane
  provider" in the positive sense; we state exactly what each coverage class gives.

**Falsifiable assertion FA-1.** *The motivating incident (a silently-expired
non-codex OAuth lane credential) is caught by the MVP without any positive
heartbeat for that provider; and a healthy non-codex `api_key` lane neither pages
nor is silently dropped.* **Refuted if:** expiring the claude OAuth token does not
fire `LaneCredExpirySoon`/`LaneCredRenewalStalled` within the documented window; or
a healthy `api_key` lane pages; or removing an `api_key` lane's credential produces
no lane-labeled `LaneAuthSampleMissing`. **Game-day GD-1 / GD-1b** prove it.

### OQ2 — Prober location

**Decision. Two distinct collectors, two locations:**

- **Layer 1 sampler (MVP) lives IN the daemon**, as a best-effort step of
  `Collector.Refresh` on the recovery-sweep cadence, bounded by a per-lane timeout
  in the spirit of the existing `doctorFoldTimeout = 15s`. It is a **read-only file
  sample with no provider round-trip** (it does *not* call the provider API), so it
  carries none of the active-probe blast radius. It resolves the credential via the
  **resolver contract** (F2 fold, below), reads the lane-owned file as the lane user
  (`sudo -n -u <lane> env -i`), extracts expiry, and writes a
  `lane.cred_expiry_sampled` event (or a `lane.cred_resolver_mismatch` event when it
  cannot prove the source).
- **Layer 2 active prober (follow-up) lives OUTSIDE the daemon**, as a systemd timer
  on `proximal`: unit **`striatum-lane-auth-prober.timer` → `.service`**
  (`User=halbritt`, `Restart=on-failure`, loopback-only), exactly the shape of the
  existing `proximal` units (cf. `whisper-stt.service` / `praxis-stt-shim`). It
  performs the real read-only auth check across all lanes + a negative probe, and
  writes results back through the daemon RPC boundary (or a Prometheus
  textfile-collector file) — it never mutates daemon state directly.

**Why split.** "Who watches the watcher" applies to the *active* prober: a probe
that performs real auth round-trips and the fail-open negative probe must **not
share fate** with the daemon. External systemd timer + its own dead-man's switch is
the correct location. Layer 1's sampler is a pure file read with no behavior risk;
co-locating it with the existing DB fold is cheaper, and the shared-fate concern is
fully discharged by absence detection: if the daemon dies, `/metrics` is
unscrapeable → `up{job="striatumd"}==0` and `MetricsSnapshotStale` page; if the
sampler step alone fails, it degrades the tick to `tick_status=partial` (alertable
today) and the per-lane census fires. **A dead daemon cannot make Layer 1 read
falsely green.**

**Falsifiable assertion FA-2.** *A dead daemon / dead sampler pages; it never reads
green.* **Refuted if:** killing the daemon produces no page within 5m, or a
sampler-step failure leaves the expiry gauge serving a last-good value with no
`tick_status=partial` and no census alert. **Game-day GD-4** proves it.

### OQ3 — Metric cardinality

**Decision. Closed label sets and a per-family series budget of 32**, collapsing
overflow onto `{lane="other"}` via the existing `applySeriesBudget`
(`budget.go:35`) and incrementing
`striatum_metrics_cardinality_clipped_total{family="..."}`.

- `kind` is a **closed enum `{oauth, api_key}`** (≤ 2 values).
- `provider` is a **closed enum** (`codex`, `claude`, `agy`, `gemini`, …) — the
  adapter identity, a small host-level set.
- `lane` is the **declared-roster slug = the lane OS user** — a small, host-level,
  operator-declared, **non-private** set; **never** the run-scoped `lane_id`, and
  **never** a repo/run/session id, path, branch, sha, or byline. The fold validates
  each event's lane against the declared roster and folds an unknown value to
  `{lane="other"}` (defense-in-depth identical to `doctor_problems{class}`).
- **Budget math incl. the F1-folded families.** A given lane has exactly **one**
  `(provider, kind)`, so the new `striatum_lane_auth_expected{lane,provider,kind}`
  vector has exactly `roster_size` series (one per rostered mechanism), and
  `striatum_lane_cred_sample_present{lane,kind}` likewise tracks roster size. With
  ≤16 rostered lanes that is ≤16 series per family — well inside the **per-family
  budget of 32** (16 lanes × 2 kinds worst case). Each `{lane,…}` family is
  independently budgeted at 32 and overflows to `{lane="other"}`. The new vectors do
  **not** blow the OQ3 budget.

**Why drop the credential `id`.** A credential `id` (account/token id) is
higher-cardinality and potentially repo-linkable — it would violate the RFC 0137
redaction contract. `{lane, provider, kind}` is sufficient for per-lane attribution;
identity beyond the roster slug never reaches the wire.

**Falsifiable assertion FA-3.** *lane-scoped series are hard-bounded at 33 per
family and any overflow is visible.* **Refuted if:** a test feeding 100 synthetic
lanes produces >33 series for any family, or produces a clip with no increment of
`striatum_metrics_cardinality_clipped_total`. **Test `TestLaneCredSeriesBudget`**
(extended to cover `lane_auth_expected` and `lane_cred_sample_present`).

### OQ4 — Staleness-threshold source

**Decision. Operator-declared in the Backbone roster, exported as gauges — NOT
auto-derived from observed credential lifetime.** Two per-lane thresholds are
declared in the roster and folded onto the wire:

- `striatum_lane_auth_staleness_threshold_seconds{lane}` (Layer 3; default ≈ 1.5×
  credential lifetime)
- `striatum_lane_cred_expiry_lead_seconds{lane}` (Layer 1 tripwire lead; default ≈
  3× renewal cadence)

A sensible default derived from the *declared* lifetime is emitted so a newly
rostered lane is covered without hand-tuning, but the **source of authority is the
declared roster**, which the operator diff-reviews.

**Why not auto-derive from observed lifetime.** Auto-derivation is **circular**: the
very failure mode (renewal stopped) corrupts the observed-lifetime signal, so a
threshold derived from it lets a degrading credential silently move its own
goalposts — the alert relaxes exactly as the thing it watches rots. The roster is
already required (it provides the census denominator), so the threshold lives in the
same diff-reviewed artifact. Exporting it as a metric means retuning or adding a lane
is a **roster edit, not a `proximal` rule edit** (the alert reads the threshold via
`on(lane) group_left()`).

**Falsifiable assertion FA-4.** *A degrading credential does not shrink its own
threshold.* **Refuted if:** a test in which a lane's observed lifetime shrinks causes
its exported threshold to shrink. **Test `TestThresholdFromRosterNotObserved`.**

---

## F1 fold — per-lane roster-presence vector + non-expiring coverage (binding)

The scalar census `count(seconds_to_expiry) < expected_count` is **removed**. It
could not both (a) avoid false-positives for no-expiry `api_key` credentials and (b)
name the missing lane. The replacement is a roster-vs-observation contract that
preserves `lane` and separates coverage classes:

1. **Expected (roster) vector** — `striatum_lane_auth_expected{lane,provider,kind} 1`,
   one series per rostered lane auth mechanism, folded from the Backbone roster. This
   is the census *expected set*. (`count(striatum_lane_auth_expected)` derives the
   denominator the retired scalar used to carry.)
2. **Observed-sample presence** — `striatum_lane_cred_sample_present{lane,kind}`,
   **independent of `seconds_to_expiry`**: `1` when the sampler **resolved (per the
   F2 contract) and parsed** the lane's runtime credential this sweep; **absent**
   when not observed. An `api_key` lane that has no expiry still emits
   `sample_present=1` when healthy (observed-but-no-expiry) — so it is *covered* by
   the census without being forced to produce an expiry series it cannot have.
3. **Absence rule with label-preserving `unless`** —
   `striatum_lane_auth_expected unless on(lane) (striatum_lane_cred_sample_present == 1)`
   fires for any rostered lane with no successful sample, **carrying that lane's
   `lane`/`provider` labels**. This restores per-lane attribution and the "no
   aggregate-only rule" guarantee.

**Coverage classes, stated explicitly:**

| Class | Positive validity (MVP) | Expiry telemetry (MVP) | Absence census (MVP) |
| --- | --- | --- | --- |
| codex OAuth | ✔ Layer 3 heartbeat | ✔ Layer 1 | ✔ |
| non-codex OAuth (claude) | ✘ (Layer 2 follow-up) | ✔ Layer 1 (resolver-proven) | ✔ |
| non-codex `api_key` (agy/…) | ✘ deferred/**accepted risk** | n/a (no expiry) | ✔ (`sample_present`) |

**Falsifiable assertion FA-F1.** *The census names the missing lane and a healthy
`api_key` lane neither pages nor is dropped.* **Refuted if:** removing one lane's
sample yields a firing series with no `lane` label; or a healthy `api_key` lane
pages; or an `api_key` lane is excluded from `striatum_lane_auth_expected`. **Tests
`TestLaneAuthSeriesMissingNamesMissingLane`, `TestNoExpiryCredentialDoesNotSatisfyExpiryCensus`,
`TestScalarCountCannotMaskRosterMismatch`** + a healthy-`api_key`-lane fixture.

## F2 fold — provider-agnostic credential-resolver contract (fail-closed) (binding)

"Reuse the codex `ResolveAuthHome` shape" is **removed** for non-codex lanes: it
resolves bare `HOME`, not where a non-codex CLI resolves its token, and the
sanitized env strips provider-specific credential locations (re-anchor table above).
The replacement is an explicit resolver contract:

1. **Per in-scope provider, name the exact runtime source + precedence the lane CLI
   actually resolves**, tied to **adapter identity**, the **launch-env keys** that
   select the credential, and the env keys that are intentionally **forbidden**:
   - **codex** → `$CODEX_HOME/auth.json` (else `$HOME/.codex/auth.json`) — already
     known (`lane_provider_auth.go:356-363`).
   - **claude (OAuth, in-scope)** → the path the claude adapter's `LaunchEnv`
     selects (its credential-home / config-dir env key), resolved **in the same
     precedence the CLI uses**, read as the lane user. The resolver must consult the
     adapter's launch env, **not** a stripped preflight allowlist, so it samples the
     credential the live lane process resolves.
2. **The resolver fails closed.** When the runtime credential source cannot be proven
   for a lane (no resolver entry for the provider, the launch-env key absent, the
   resolved path unreadable, or the file unparseable), the sampler emits
   **`lane.cred_resolver_mismatch`** → `striatum_lane_cred_resolver_mismatch{lane,kind} 1`
   and writes **no** `seconds_to_expiry` and **no** `sample_present` for that lane.
   A green expiry gauge is **never** emitted from a fallback/decoy path. The
   `resolver_mismatch` series and the absent `sample_present` both page (below).
3. **The roster `credential_path_template` is a cross-check, not the proof.** It is
   used to *detect drift* between the declared path and the resolver-proven path (a
   reconciliation signal), never as the authoritative read location.

This is the operator-authorized route: a resolver contract that fails closed into a
pageable `resolver_mismatch`, narrowing the L1 same-credential claim to credentials
whose runtime resolution is **proven**.

**Falsifiable assertion FA-F2 / FA-6 (L1 same-credential, narrowed).** *The sampler
reads the SAME credential the lane presents at runtime, or it fails closed — it never
reads a stand-in at rest as green.* **Refuted if:** the sampler emits a green
`seconds_to_expiry` for a lane whose runtime source it cannot prove; or it tracks a
fresher `HOME` decoy instead of the launch-env-resolved credential; or a missing
resolver entry yields silence rather than `resolver_mismatch`. **Tests
`TestCredResolverFailsClosedOnUnprovenSource`,
`TestCredResolverTracksLaunchEnvNotHomeDecoy`,
`TestCredExpirySamplerReadsLanePresentedCredential`.** **Game-day GD-2 / GD-2b**.

---

## Closing the codex-only preflight hole (Layer 3 write site — carried, FA-5)

**The post-success write for Layer 3 lives in the gate caller, at the success
branch, never in the pure `Check`:**

- **Site:** `go/pkg/mutations/supervision_provider_auth.go:56`, inside
  `if result.Passed() { … return nil }`. On a real `Passed()` result, emit a
  `lane.auth_success` domain event into `striatumd.events` with payload
  `{lane_user, provider, kind, repository_id, run_id}`. Mirror the write at the other
  real-success sites that run `Check`: `doctor lane_provider_auth`
  (`reads/doctor_lane_provider_auth.go`) and `run drive` (`cli/rundrive/rundrive.go`).
- **Best-effort and side-effect-free on the gate verdict:** a failed event write is
  logged and swallowed; it can **never** change whether a lane launches (FA-7). This
  keeps the change read-only telemetry over the auth boundary.
- **The fold:** `Collector.Refresh` adds a best-effort
  `SELECT payload_json->>'lane_user', MAX(EXTRACT(EPOCH FROM created_at)) FROM
  striatumd.events WHERE event_type='lane.auth_success' GROUP BY 1`, mapping each
  lane_user → roster slug → `striatum_lane_auth_last_success_timestamp_seconds{lane}`.
  A lane_user not in the roster folds to `{lane="other"}`.

**Why no synthetic heartbeat for non-codex.** A heartbeat that is not downstream of a
real success is a lie, and a lie that reads green is worse than no signal. Non-codex
lanes are covered by Layer 1 (OAuth expiry) + the `sample_present` census + (for
api_key) the deferred Layer 2 positive probe. Both falsifiers and the adjudicator
credited this refusal as correct; it is preserved.

**Falsifiable assertion FA-5.** *The heartbeat is emitted strictly downstream of a
real `result.Passed()`, only for providers that actually run `Check`.* **Refuted
if:** an `auth_last_success` series appears for a lane whose `Check` never returned
`Passed()` (e.g. a claude lane), or if the event is emitted on the
`supported==false` early return (`supervision_provider_auth.go:39-44`). **Test
`TestAuthSuccessEventOnlyOnPassedCodex`.**

---

## Exact metric surface (this repo: `go/pkg/metrics/`)

All families `ClassificationOperational`; added to `DefaultRegistry()`
(`registry.go:149`) and rendered in `render.go`; the change regenerates
`metrics_allowlist.json` and updates the boot-time allowlist hash (diff-reviewed,
CI-guarded). Label names below are the **complete closed set** per family.

**MVP families:**

| Family | Type | Labels | Fed by | Layer / role |
| --- | --- | --- | --- | --- |
| `striatum_lane_auth_expected` | gauge `1` | `lane,provider,kind` | Backbone roster (one per mechanism) | **census expected set (F1)** |
| `striatum_lane_cred_sample_present` | gauge `1/0` | `lane,kind` | `lane.cred_expiry_sampled` / sampler success (incl. api_key observed-no-expiry) | **census observed set (F1)** |
| `striatum_lane_cred_resolver_mismatch` | gauge `1` | `lane,kind` | `lane.cred_resolver_mismatch` event | **fail-closed (F2)** |
| `striatum_lane_cred_seconds_to_expiry` | gauge | `lane,kind` | `lane.cred_expiry_sampled` (latest; OAuth only) | L1 |
| `striatum_lane_cred_age_seconds` | gauge | `lane,kind` | same event (`now - issued_at`) | L1 |
| `striatum_lane_auth_last_success_timestamp_seconds` | gauge | `lane` | `lane.auth_success` event fold (MAX) | L3 (codex) |
| `striatum_lane_auth_staleness_threshold_seconds` | gauge | `lane` | roster (OQ4) | L3 |
| `striatum_lane_cred_expiry_lead_seconds` | gauge | `lane` | roster (OQ4) | L1 |

*The scalar `striatum_lane_auth_expected_count` from the Holder draft is **retired**
(it was the F1 defect); the denominator is `count(striatum_lane_auth_expected)`.*

**Follow-up families (named now, deferred — Layer 2):**
`striatum_lane_auth_probe_success` gauge `{lane}` (1/0 last probe);
`striatum_lane_auth_probe_last_run_timestamp_seconds` gauge `{lane}` (prober
dead-man); `striatum_lane_auth_negative_probe_rejected` gauge `{lane}` (1 = invalid
credential correctly rejected; **0 = fail-open, page immediately**).

**Provider-specific expiry extraction (honest boundary).** The sampler parses expiry
per credential shape: codex OAuth `id_token` JWT `exp` (or a `tokens.expiry` /
`expires_at` field when present); claude OAuth token `expiresAt`. **`kind="api_key"`
credentials have no expiry** → they emit **no** `seconds_to_expiry` series; they emit
`sample_present=1` when healthy and rely on the census for absence detection and on
Layer 2 (deferred) for positive validity. This gap is explicit, not silent.

## Exact alert surface (separate repo: `halbritt/proximal`)

Append to the existing `striatum` group in
`observability/prometheus/rules/striatum-alerting.rules.yml` (corrected path; routed
Alertmanager → Slack `#proximal-alerts`). **Every alert carries
`labels: {severity, lane: "{{ $labels.lane }}"}`** (the census rule now preserves
`lane` via `unless on(lane)`), and a runbook annotation. **No aggregate-only rule**,
so one dead lane can never average into green.

```yaml
# L1 proactive — fires WHILE the lane is still healthy (OAuth, resolver-proven)
- alert: LaneCredExpirySoon
  expr: striatum_lane_cred_seconds_to_expiry
          < on(lane) group_left() striatum_lane_cred_expiry_lead_seconds
  for: 15m
- alert: LaneCredRenewalStalled            # renewer silently stopped; days early
  expr: delta(striatum_lane_cred_seconds_to_expiry[6h]) < 0
  for: 2h
# Per-lane census / absence — covers ALL rostered lanes incl. non-codex api_key,
# preserving the missing lane's label (F1 fold; replaces the scalar count)
- alert: LaneAuthSampleMissing
  expr: striatum_lane_auth_expected
          unless on(lane) (striatum_lane_cred_sample_present == 1)
  for: 10m
# Fail-closed resolver — unproven runtime credential source pages, never green (F2)
- alert: LaneCredResolverMismatch
  expr: striatum_lane_cred_resolver_mismatch == 1
  for: 10m
# L3 passive backstop (codex lanes that run a real Check)
- alert: LaneAuthHeartbeatStale
  expr: time() - striatum_lane_auth_last_success_timestamp_seconds
          > on(lane) group_left() striatum_lane_auth_staleness_threshold_seconds
  for: 10m
# Shared-fate — exporter/daemon down pages as loudly as a stale value
- alert: LaneAuthExporterDown
  expr: absent(striatum_lane_auth_expected) or up{job="striatumd"} == 0
  for: 5m   # complements the existing MetricsSnapshotStale rule
```

**`LaneCredRenewalStalled` window caveat (stated, not hidden).** The `[6h]` window
must exceed ≈1.5× the *longest-cadence* rostered lane's renewal interval; a single
rule cannot carry a per-lane window. Document the assumption in the runbook; **GD-3**
tests it. If lane cadences diverge widely, split into per-cadence recording rules
(follow-up).

---

## Backbone roster (declared denominator + thresholds + resolver hints)

A declared, operator-owned roster of every lane auth mechanism, host-level (keyed by
lane OS user), each entry:
`{lane, provider, kind, credential_path_template, staleness_threshold_seconds,
expiry_lead_seconds, renewal_cadence_seconds}`. To avoid a schema migration for the
MVP it is a daemon-config file (e.g. `lane-auth-roster.json` under the daemon config
dir), read at fold time (small, host-local, daemon-readable) — mirroring how the
per-repo consent flag avoided a migration by living in `settings_json`. It folds to
`striatum_lane_auth_expected{lane,provider,kind}`,
`striatum_lane_auth_staleness_threshold_seconds{lane}`, and
`striatum_lane_cred_expiry_lead_seconds{lane}`. `credential_path_template` is the F2
drift cross-check (declared-vs-resolver), **never** the authoritative read path.

**Reconciliation (doctor check).** A new doctor check flags: (a) a live lane (one
that emitted a `lane.auth_success` or `lane.cred_expiry_sampled` event within the
sweep window) with **no** roster entry; (b) a roster entry with **no** observed
sample within its declared SLA; and (c) a `resolver_mismatch` standing for longer
than one sweep. Surfaced via the existing `striatum_doctor_problems{class}` family —
no new wire surface — *in addition to* the Slack-paging census/resolver alerts (the
doctor check is corroboration, not the page).

**Falsifiable assertion FA-7 (no behavior change — Non-Goal / RFC 0143 compliance).**
*Nothing in this spec changes preflight behavior, timeouts, or the credential trust
model.* **Refuted if:** any existing `laneproviderauth` / gate test changes verdict,
or a sampler/heartbeat/event-write failure flips a gate decision or alters a timeout.
The heartbeat write and the sampler are on success/observation paths only and swallow
their own errors. **Tests:** the existing `laneproviderauth` and
`supervision_provider_auth` suites pass **unchanged**; add
`TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict`.

---

## Falsifiable assertions ↔ named tests (consolidated)

| ID | Assertion (refuted-if) | Named test / game-day |
| --- | --- | --- |
| FA-1 | Motivating incident caught without a non-codex heartbeat; healthy api_key lane neither pages nor is dropped | GD-1, GD-1b |
| FA-2 | Dead daemon / dead sampler pages; never reads green | GD-4 |
| FA-3 | lane series hard-bounded at 33/family; overflow visible | `TestLaneCredSeriesBudget` |
| FA-4 | Degrading credential cannot shrink its own threshold | `TestThresholdFromRosterNotObserved` |
| FA-5 | Heartbeat only downstream of real `Passed()`, codex-only | `TestAuthSuccessEventOnlyOnPassedCodex` |
| FA-F1 | Census names the missing lane; api_key healthy lane covered, not dropped | `TestLaneAuthSeriesMissingNamesMissingLane`, `TestNoExpiryCredentialDoesNotSatisfyExpiryCensus`, `TestScalarCountCannotMaskRosterMismatch` |
| FA-F2 / FA-6 | Sampler reads the resolver-proven credential or fails closed; never green from a decoy | `TestCredResolverFailsClosedOnUnprovenSource`, `TestCredResolverTracksLaunchEnvNotHomeDecoy`, `TestCredExpirySamplerReadsLanePresentedCredential` |
| FA-7 | No preflight behavior / timeout / trust-model change | existing suites unchanged + `TestAuthSuccessEventWriteFailureDoesNotChangeGateVerdict` |

## Game-day proof (each alert fires before a real incident)

An alert that has never fired is a liability. Each MVP alert has a synthetic trigger
that must fire within its documented window:

- **GD-1 (motivating incident):** expire the **claude** OAuth token → `LaneCredExpirySoon`
  then `LaneCredRenewalStalled` fire; **no** `LaneAuthHeartbeatStale` for claude
  (claude has no heartbeat — documented, not a bug). *Refuted if claude's silent
  expiry produces no page.*
- **GD-1b (api_key absence):** a healthy non-codex `api_key` lane emits no page;
  remove its credential file → `LaneAuthSampleMissing{lane="agy"}` fires **with the
  lane named**. *Refuted if it pages while healthy, or fires without the lane label.*
- **GD-2 (L1 same-credential):** rotate a lane's live token → `seconds_to_expiry`
  jumps within one sweep; point the lane's launch env at one credential while `HOME`
  holds a *fresher* decoy → the gauge tracks the launch-env-resolved credential, not
  the decoy.
- **GD-2b (resolver fail-closed):** roster a provider with no resolver entry (or a
  missing launch-env key) → `LaneCredResolverMismatch` fires and **no** green
  `seconds_to_expiry` appears for that lane.
- **GD-3 (renewal-stalled, days early):** freeze the renewer (stop advancing
  `not_after`) while the token is still valid → `LaneCredRenewalStalled` fires
  *before* `seconds_to_expiry` crosses the lead tripwire.
- **GD-4 (shared-fate):** `systemctl stop` the daemon → `up{job="striatumd"}==0`,
  `MetricsSnapshotStale`, and `LaneAuthExporterDown` all fire within 5m.
- **GD-5 (codex heartbeat):** block the codex lane's `auth.json` →
  `LaneAuthHeartbeatStale{lane="codex"}` fires after the staleness threshold; restore
  → it clears on the next real gated launch.

---

## Acceptance Criteria (what an impl-run + verify-run MUST meet)

1. **All MVP metric families** (`lane_auth_expected`, `lane_cred_sample_present`,
   `lane_cred_resolver_mismatch`, `lane_cred_seconds_to_expiry`,
   `lane_cred_age_seconds`, `lane_auth_last_success_timestamp_seconds`,
   `lane_auth_staleness_threshold_seconds`, `lane_cred_expiry_lead_seconds`) exist in
   `DefaultRegistry()`, are `ClassificationOperational`, carry only their closed label
   sets, and the regenerated `metrics_allowlist.json` + boot-time hash are committed
   and CI-green.
2. **Every named test (FA-1…FA-7, FA-F1, FA-F2) is present and passing.** The
   existing `laneproviderauth` and `supervision_provider_auth` suites pass
   **unchanged** (FA-7).
3. **Per-lane attribution rule.** No aggregate-only alert or dashboard panel may
   average a dead lane into green: every MVP alert fires with a `lane` label,
   including the census (`unless on(lane)`), and the verify run asserts this.
4. **Mandatory game-day fire test.** Each MVP alert (GD-1, GD-1b, GD-2, GD-2b, GD-3,
   GD-4, GD-5) fires on its synthetic trigger within its documented window in a
   game-day exercise on `proximal`; the verify run records the fire evidence. An MVP
   alert that has never fired blocks acceptance.
5. **Fail-closed proof.** The verify run demonstrates GD-2 and GD-2b: a decoy/fresher
   `HOME` credential never moves the gauge, and an unprovable runtime source yields
   `LaneCredResolverMismatch` rather than a green gauge.
6. **No over-claim survives into code.** The verify run confirms the MVP makes **no**
   positive-validity claim for non-codex `api_key` lanes (they are census-covered
   only) and the docs/runbook state the accepted/deferred Layer-2 risk.
7. **`proximal` rules land as a separate change** to
   `observability/prometheus/rules/striatum-alerting.rules.yml`, in the `striatum`
   group, routed to `#proximal-alerts`, with runbook annotations.

## Product-boundary & rejected-trap compliance (checklist)

- **Read-only telemetry over the auth boundary.** Sampler reads files + emits
  events; heartbeat writes on an existing success path; no provider round-trip in the
  MVP (Layer 2's active probe deferred). ✔
- **No preflight behavior / timeout / trust-model change** (that is RFC 0143). FA-7. ✔
- **Local-first, pull-only.** Metrics via the existing RFC 0137 exporter
  (loopback/tailnet pull); alerts via the existing `proximal` Prometheus →
  Alertmanager → Slack. No hosted/cloud/push/remote-write. ✔
- **No per-repo private-data leak.** `lane` = roster slug (OS user); `provider`,
  `kind` = closed enums; credential `id` dropped; no repo/run/session id, path, sha,
  prompt, or byline on the wire. Respects the RFC 0137 redaction + cardinality
  contract. ✔
- **Rejected traps honored.** No fever/throttle (telemetry never throttles a lane);
  no circadian short-TTL (that is RFC 0143); no sacrificial canary — the deferred
  Layer-2 negative probe is an *always-expected-fail synthetic assertion*, not a decoy
  prod lane. ✔

## Build order for `rfc-0162-build` (contract-first / TDD)

1. **Backbone roster file + fold** → `striatum_lane_auth_expected{lane,provider,kind}`
   + threshold gauges; doctor reconciliation check (FA-4, FA reconciliation).
   Smallest, unblocks the census.
2. **F2 resolver contract** (codex + claude) + `lane.cred_resolver_mismatch` event +
   fold → `resolver_mismatch` gauge; fail-closed semantics (FA-F2; GD-2/GD-2b).
3. **Layer 1 sampler** (resolver-proven read) + `lane.cred_expiry_sampled` event +
   fold → `seconds_to_expiry` / `age` / `sample_present` gauges (FA-3, FA-F1, FA-6;
   GD-1/GD-1b/GD-3).
4. **Layer 3 heartbeat** event at `supervision_provider_auth.go:56` (+ doctor/drive
   success sites) + fold (FA-5, FA-7; GD-5).
5. **`proximal` rules** (separate change): the six MVP alerts (GD-1/GD-1b/GD-2b/GD-4).
6. **Layer 2 prober** (`striatum-lane-auth-prober.timer`, follow-up): active check +
   negative probe + prober dead-man + synthetic always-fail lane — earns non-codex
   lanes a *positive* validity signal and closes the accepted `api_key` risk.

---
<sub>Committer deliverable for the RFC 0162 falsification-gate design run. Cleared
via `DECISION-rfc-0162-design-override-fold-f1-f2` (operator override of the cycle-2
`needs_revision`, folding F1 + F2 as binding criteria). Re-anchored to
`~/git/striatum` @ current source. The `rfc-0162-build` (contract-first) and
`rfc-0162-verify` (game-day) runs confirm no over-claim survives into code.</sub>

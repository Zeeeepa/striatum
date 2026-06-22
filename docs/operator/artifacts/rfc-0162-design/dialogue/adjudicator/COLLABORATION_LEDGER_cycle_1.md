---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
topic: "RFC 0162 lane-auth silent-failure observability — falsifiable implementation spec"
participants:
  - "holder-author-001"
  - "falsifier-reviewer-001"
  - "falsifier-reviewer-002"
  - "adjudicator-author-001"
cycle: 1
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: "MVP (Layer 1 expiry telemetry + codex-scoped Layer 3 heartbeat + roster census) delivers a provider-agnostic absence-of-success signal that catches a silently-expired non-codex lane credential without synthesizing a fake non-codex heartbeat."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "The census rule count(seconds_to_expiry) < expected_count cannot honestly cover a non-codex api_key lane: it emits no expiry series, so the lane either pages permanently from day zero (if counted in expected_count) or is silently dropped (if excluded); the aggregate rule also carries no lane label, contradicting the spec's own no-aggregate-only-rule guarantee, and no per-lane roster-presence vector is named."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "Layer 1's FA-6 same-credential claim is unproven for non-codex lanes: the cited ResolveAuthHome/SanitizeEnv/sudo shape is codex-only in current source, so non-codex resolution degrades to an operator-declared roster path that can drift from the credential the provider CLI actually resolves — reintroducing the exact healthy-gauge-over-a-coasting-to-death-lane failure the SEED told the run to falsify; GD-2's decoy test is too weak to prove the sampled file is the CLI's real source."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "Both material falsifier challenges landed and stand unrebutted by the spec as written. The provider-agnostic guarantee that replaces the (correctly-refused) fake non-codex heartbeat is incomplete on two independent axes: coverage (F1 — non-expiring api_key lanes; aggregate-only census with no per-lane vector and no lane label) and resolution (F2 — no provider-agnostic credential resolver; the roster path is config, not proof of runtime resolution). Charter item 2 (a provider-agnostic absence-of-success signal downstream of a real success for every lane provider) is therefore not yet falsifiably delivered. OQ4 (threshold source), OQ3 numerics, FA-5 (codex-scoped heartbeat) and FA-7 (Non-Goal/RFC 0143 compliance) are resolved and carry forward intact. One revision cycle is available; the falsifiers re-attack the revised spec."
findings:
  - id: F1
    severity: high
    posture: design
    status: open
    challenge: "MVP census cannot honestly cover non-expiring (api_key) non-codex lanes. Fix: add a per-lane roster-presence vector (e.g. striatum_lane_auth_expected{lane,provider,kind}) with unless/absent semantics preserving the lane label; then either bring a positive non-expiring signal into MVP downstream of a real success, or explicitly narrow MVP scope to expiring OAuth and mark api_key lanes deferred/accepted-risk. Add a healthy non-codex api_key lane test."
  - id: F2
    severity: high
    posture: design
    status: open
    challenge: "Layer 1 does not prove it samples the same credential a non-codex lane presents. Fix: a provider-specific resolver contract (adapter identity, launch-env precedence, CLI credential search order) per MVP non-codex provider, with fail-closed resolver_mismatch instead of a green gauge from a fallback path, plus non-codex resolver tests including a provider-env credential vs a fresher HOME decoy; or narrow the MVP claim accordingly."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0162 design run (cycle 1)

author: adjudicator-author-001

> Adjudication of the cycle-1 dialogue trajectory for the RFC 0162 lane-auth
> silent-failure observability design run. Inputs read: the Holder spec
> (`dialogue/holder/HOLDER.md`), both falsifier challenges
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`),
> and the `SEED.md` charter + operator anchor table. No raw terminal output or
> source was read; the falsifiers' source citations are credited because they
> agree with the SEED's operator-verified anchor table (codex-only `Check`,
> codex-only `ResolveAuthHome`).

## Verdict

**verdict: needs_revision**

Two material challenges landed and **both stand unrebutted by the spec as
written**. They are not peripheral: together they show the spec's headline
claim — a *provider-agnostic absence-of-success signal* (charter item 2, the
root reframe) — is **not yet falsifiably delivered** for non-codex lanes. Each
maps directly onto a named `needs_revision` trigger in the adjudication rubric
(an L1 gauge read off a credential the live lane never reloaded; an
absence/coverage mechanism asserted without a concrete buildable mechanism).
One revision cycle is available: the Holder revises `HOLDER.md`, the falsifiers
re-attack the revised spec.

The Holder spec is strong where it counts most and the revision must **build on**
that, not regress it (see "Credited strengths" below). The gate does not clear
only because the *replacement* provider-agnostic guarantee the spec substitutes
for the (correctly-refused) fake non-codex heartbeat is itself incomplete on two
independent axes — *coverage* (F1) and *resolution* (F2).

## Challenge ledger

### F1 — `falsifier-reviewer-001`: the MVP census cannot honestly cover non-expiring (api_key) non-codex lanes

- **Claim challenged.** The MVP is provider-agnostic because Layer 1 emits
  `striatum_lane_cred_seconds_to_expiry{lane,kind}` and the census rule
  `count(striatum_lane_cred_seconds_to_expiry) < striatum_lane_auth_expected_count`
  "guarantees a non-codex lane that vanishes pages" (HOLDER OQ1 + lines 234-243,
  316-319), combined with Layer 3 being codex-scoped, Layer 2 deferred, and the
  explicit admission that `kind="api_key"` credentials have no expiry so emit no
  expiry series and "rely on … the census rule" (HOLDER lines 278-283).
- **Material?** **Yes.** It attacks charter item 2 (the absence-of-success
  signal must be downstream of a real success for *every* provider) and the root
  reframe (no-series must page as loudly as a stale series). A non-codex
  `api_key` lane is in scope — `kind` is a closed enum that *includes* `api_key`
  (HOLDER OQ3) — and the census rule is the spec's *only* claimed coverage for it.
- **Rebutted, or stands?** **Stands unrebutted.** The census rule as specified
  counts `seconds_to_expiry` series against a *scalar* `expected_count` (roster
  size). For an `api_key` lane that emits no expiry series this is internally
  contradictory: either the lane is in `expected_count` → **permanent paging from
  day zero even when healthy** (the lane can never produce the counted series), or
  it is excluded → the lane is **silently dropped** from the "all lane providers"
  contract. The spec defines no per-lane roster-presence vector (e.g.
  `striatum_lane_auth_expected{lane,provider,kind} 1`); and `count(...) <
  scalar` is an **aggregate-only rule that carries no `lane` label**, directly
  contradicting the spec's own promise (HOLDER lines 294-300) that every alert
  carries `lane` and that "no aggregate-only rule [exists] so one dead lane can
  never average into green." The falsifier's "strongest rebuttal on the Holder's
  behalf" (scope to expiring OAuth / refine PromQL later) is acknowledged and
  correctly judged insufficient: a later PromQL refinement cannot invent an
  absent per-lane roster vector the metric surface never named, and a doctor
  warning is not the Slack alert contract the spec leans on.
- **Disposition.** Material defect, unrebutted → drives `needs_revision`.

### F2 — `falsifier-reviewer-002`: Layer 1 does not prove it reads the credential a non-codex lane presents (FA-6 overclaim)

- **Claim challenged.** Layer 1 is the provider-agnostic MVP backbone that reads
  the *same* credential the lane presents at runtime, by reusing the
  `ResolveAuthHome` / `SanitizeEnv` / `sudo -n -u <lane>` shape from
  `checkCodexOfflineAuth` (HOLDER lines 56, 119-126, 285-292; FA-6), with the
  Backbone roster supplying `credential_path_template` (HOLDER lines 336-345).
- **Material?** **Yes.** This is the SEED's explicitly-named load-bearing L1 risk
  ("read the *same* credential the lane presents at runtime, not a fresh file the
  live process never reloaded — a healthy-looking gauge over a lane coasting to
  death") and the rubric's named `needs_revision` trigger. It is also the
  motivating incident itself (the silently-expired non-codex *claude* token).
- **Rebutted, or stands?** **Stands unrebutted.** The cited mechanism is
  **codex-only** in current source, per the SEED anchor table and the falsifier's
  citations: `ResolveAuthHome` honors `CODEX_HOME` only for codex and falls back to
  bare `HOME` for every other provider (`lane_provider_auth.go:354-368`); the
  sanitized preflight env allowlist carries no provider-specific credential-home /
  token-path variables (`:336-352`), and `providerAuthPreflightEnv` runs that
  allowlist *after* merging `LaunchEnv` (`supervision_env.go:66-74`), so a
  provider-specific credential location can be stripped before the resolver sees
  it. Therefore the spec's FA-6 mechanism ("reuse the codex shape") **cannot
  resolve claude/agy/gemini runtime credentials**, and the fallback — trust the
  operator-declared roster `credential_path_template` — reintroduces the exact
  failure the SEED told the run to falsify: a fresh file at the declared path
  makes `seconds_to_expiry{lane="claude"}` read green while the real claude CLI
  resolves a different, expired credential. The proposed GD-2 (a random decoy
  file does not move the gauge) is correctly judged too weak: it does not prove
  the *sampled* file is the CLI's actual credential source. The falsifier's
  "strongest rebuttal" (roster is the authoritative mapping, build adds parsers)
  is acknowledged and correctly rejected: a roster path is config, not proof of
  runtime resolution.
- **Disposition.** Material defect, unrebutted → drives `needs_revision`.

## Credited strengths (preserve these through the revision)

These were not refuted by either falsifier and both falsifiers explicitly credit
them; the revision must not regress them while closing F1/F2:

- **The fake-heartbeat trap was correctly refused.** Layer 3 is honestly
  codex-scoped (it fires only where a real `Check().Passed()` exists); the spec
  refuses to synthesize an `auth_last_success` series for providers that never run
  a real check. Both falsifiers explicitly credit this ("successfully avoids
  lying about non-codex Layer 3 heartbeats" / "correctly avoids fake non-codex
  heartbeats"). FA-5 / `TestAuthSuccessEventOnlyOnPassedCodex` is sound.
- **OQ4 (threshold source) is resolved with a concrete, defensible mechanism**:
  operator-declared in the roster, exported as a gauge, *not* auto-derived from
  observed lifetime — with the correct circularity argument (a degrading
  credential must not move its own goalposts). FA-4 /
  `TestThresholdFromRosterNotObserved` is a real falsifying test. Unattacked.
- **OQ3 cardinality gives a concrete numeric cap** (label set `{lane,kind}`, `id`
  dropped, per-family budget 32 → ≤33 series incl. `{lane="other"}`, overflow
  visible via `striatum_metrics_cardinality_clipped_total`). FA-3 /
  `TestLaneCredSeriesBudget` is concrete. Unattacked. (Note: F1's per-lane-vector
  fix interacts with this — the revision must keep the new expected-vector inside
  the same budget.)
- **FA-7 (no behavior change — Non-Goal / RFC 0143 boundary) and the
  product-boundary checklist** (read-only, pull-only, no hosted/cloud/push, no
  private-data labels) are well-argued and unattacked.

## What the revision MUST fix to clear on re-attack

The revision must close **both** material gaps, or honestly narrow the headline
claim. Concretely:

1. **Close F2 — provider-agnostic resolver contract, not a roster path.** For
   each non-codex MVP provider the spec keeps in scope (at minimum claude), name
   the *exact* credential source and precedence the lane CLI actually resolves —
   tied to adapter identity, the launch-env keys that matter, and which env keys
   are intentionally forbidden — rather than only an operator-declared roster
   template. Make the sampler **fail closed** into an explicit `resolver_mismatch`
   / missing-sample state (which must itself page via the absence rule) when the
   runtime credential source cannot be proven, instead of emitting a green gauge
   from a fallback path. Add provider-specific resolver tests for non-codex lanes,
   including the case where a provider-specific launch env points at one
   credential while the default `HOME` holds a *fresher* decoy (the gauge must
   track the credential the CLI uses, not the decoy). **Or** narrow the MVP claim
   to "operator-declared expiry telemetry for known file-backed credentials" and
   stop asserting it closes the provider-agnostic same-credential hole.

2. **Close F1 — per-lane roster-presence vector + non-expiring coverage.**
   Replace the scalar-only `expected_count` census with a per-lane expected
   vector (e.g. `striatum_lane_auth_expected{lane,provider,kind} 1`) so the
   absence rule compares expected-vs-observed lanes with `unless` / `absent`
   semantics that **preserve the `lane` label** (restoring per-lane attribution
   and the spec's own "no aggregate-only rule" guarantee). Either bring a positive
   non-expiring-credential signal into the MVP for `api_key` lanes (downstream of
   a real provider success), **or** explicitly scope the MVP to expiring OAuth
   credentials and mark non-codex `api_key` lanes as a deferred/accepted risk —
   and in that case stop claiming the MVP is provider-agnostic for *all* lane
   providers. Add a test with a **healthy** non-codex `api_key` lane so the MVP
   cannot pass by either permanent-paging or silent omission. Keep the new
   expected-vector inside the OQ3 series budget.

3. **Internal consistency.** After (1)/(2), re-state FA-1, FA-6, the metric
   surface table, and the "every alert carries `lane`, no aggregate-only rule"
   claim so they are mutually consistent — today FA-6 and the census rule
   over-claim relative to the named surfaces.

Items resolved this cycle (OQ4, OQ3 numbers, FA-5 codex scoping, FA-7 /
Non-Goal compliance) need not be re-litigated and should be carried forward
intact.

---
<sub>Adjudicator collaboration ledger for the RFC 0162 falsification-gate design
run, cycle 1. The ledger verdict — not falsifier completion — gates the phase:
`needs_revision` returns the spec to the Holder for one revision cycle.</sub>

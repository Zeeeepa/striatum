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
  - "adjudicator-author-002"
cycle: 2
entries:
  - kind: claim
    by: "holder-author-001"
    refs: ["dialogue:1"]
    text: "The MVP (Layer 1 provider-agnostic expiry telemetry + codex-scoped Layer 3 heartbeat + the roster census rule) is a provider-agnostic absence-of-success signal: it catches a silently-expired non-codex lane credential without synthesizing a fake non-codex heartbeat, and every alert carries a lane label with no aggregate-only rule that can average one dead lane into green."
  - kind: challenge
    by: "falsifier-reviewer-001"
    refs: ["dialogue:2"]
    text: "Coverage gap. The census rule count(striatum_lane_cred_seconds_to_expiry) < striatum_lane_auth_expected_count cannot honestly cover a non-codex api_key lane: api_key credentials emit no expiry series (spec's own admission), Layer 3 is codex-scoped, Layer 2 is deferred, so the lane either pages permanently from day zero (if counted in the scalar expected_count) or is silently dropped (if excluded). The rule is also aggregate-only — it carries no lane label and counts {lane,kind} series rather than distinct lanes — directly contradicting the spec's own no-aggregate-only-rule guarantee. No per-lane roster-presence vector is named."
    correspondence: landed_unrebutted
  - kind: challenge
    by: "falsifier-reviewer-002"
    refs: ["dialogue:3"]
    text: "Same-credential (FA-6) overclaim. Layer 1's claim to read the SAME credential a non-codex lane presents at runtime is unproven: the cited ResolveAuthHome/SanitizeEnv/sudo shape is codex-only in current source (ResolveAuthHome honors CODEX_HOME only for codex and falls back to bare HOME otherwise; the sanitized preflight env carries no provider-specific credential-home variables and is applied after merging LaunchEnv). The only remaining mechanism — trust the operator-declared roster credential_path_template — is config, not proof of runtime resolution: a fresh file at the declared path makes seconds_to_expiry{lane=claude} read green while the real claude CLI resolves a different, expired credential. That is exactly the healthy-gauge-over-a-coasting-to-death-lane failure the SEED told the run to falsify; GD-2's decoy test is too weak to prove the sampled file is the CLI's actual source."
    correspondence: landed_unrebutted
verdict: "needs_revision"
rationale: "Re-adjudication of the unchanged cycle-1 dialogue (the holder spec was never revised between cycles — see Provenance note). Both material falsifier challenges land and remain unrebutted by the spec as written, because the spec still ASSERTS the provider-agnostic guarantee (HOLDER lines 61-66, 234-243) and still leans on the codex-shaped resolver / roster path (HOLDER lines 285-292, 336-345) — it contains the claims being refuted, not a rebuttal to them. Charter item 2 (a provider-agnostic absence-of-success signal downstream of a real success for EVERY lane provider) is therefore not falsifiably delivered, on two independent axes: coverage (F1 — non-expiring api_key lanes; aggregate-only census with no per-lane vector and no lane label) and resolution (F2 — no provider-agnostic credential resolver; the roster path is configuration, not proof of runtime resolution). The schema's RFC 0094 §5 Check-B rule confirms the call: a clearing verdict is refused while any challenge stands landed_unrebutted, and neither challenge can be honestly recorded landed_and_rebutted or not_material. OQ4 (threshold source), OQ3 numerics, FA-5 (honest codex-scoped heartbeat) and FA-7 (Non-Goal / RFC 0143 compliance) are resolved and carry forward intact. The one workflow revision iteration is exhausted and the holder spec is frozen, so this gate cannot self-clear without operator action — see the Path-forward note."
findings:
  - id: F1
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:2"]
    closest_acceptable_answer: "Add a per-lane roster-presence vector (e.g. striatum_lane_auth_expected{lane,provider,kind} 1) so the absence rule compares expected-vs-observed lanes with unless/absent semantics that PRESERVE the lane label; then EITHER bring a positive non-expiring-credential signal into the MVP for api_key lanes downstream of a real provider success, OR explicitly scope the MVP to expiring OAuth credentials and mark non-codex api_key lanes a deferred/accepted risk (and drop the for-every-provider claim). Keep the new vector inside the OQ3 series budget. Add a test with a HEALTHY non-codex api_key lane so the MVP cannot pass by either permanent-paging or silent omission."
    challenge: "MVP census cannot honestly cover non-expiring (api_key) non-codex lanes, and the count(...) < scalar census is an aggregate-only rule with no lane label, contradicting the spec's own no-aggregate-only-rule guarantee."
  - id: F2
    severity: high
    posture: design
    status: open
    source_refs: ["dialogue:3"]
    closest_acceptable_answer: "Specify a buildable provider-resolution CONTRACT, not just a roster template: for each non-codex MVP provider in scope (at minimum claude) name the exact credential source and precedence the lane CLI resolves — tied to adapter identity, which launch-env keys matter, and which are intentionally forbidden. Make the sampler FAIL CLOSED into an explicit resolver_mismatch / missing-sample state (which itself pages via the absence rule) when the runtime credential source cannot be proven, rather than emitting a green gauge from a fallback path. Add non-codex resolver tests including the provider-env-credential vs fresher-HOME-decoy case. OR narrow the Layer 1 claim to 'operator-declared expiry telemetry for known file-backed credentials' and stop asserting it closes the provider-agnostic same-credential hole."
    challenge: "Layer 1's FA-6 same-credential guarantee is unproven for non-codex lanes; the cited resolver is codex-only in current source and the fallback (operator-declared roster path) reintroduces the exact silent-failure the SEED told the run to falsify."
branches:
  design: blocked
---

# Collaboration Ledger — RFC 0162 design run (cycle 2)

author: adjudicator-author-002

> Second, independent adjudication of the RFC 0162 lane-auth silent-failure
> observability design run. Inputs read: the Holder spec
> (`dialogue/holder/HOLDER.md`), both falsifier challenges
> (`dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`),
> the `SEED.md` charter + operator anchor table, and the cycle-1 ledger
> (`dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`) for continuity. No raw
> terminal output or product source was read for the adjudication itself; the
> falsifiers' source citations are credited because they agree with the SEED's
> operator-verified anchor table (codex-only `Check`, codex-only
> `ResolveAuthHome`). This ledger is a fresh judgment of the same trajectory, not
> a re-publication of cycle 1.

## Verdict

**verdict: needs_revision**

Both material challenges land and **both stand unrebutted by the spec as
written**. They are not peripheral: together they show the spec's headline claim
— a *provider-agnostic absence-of-success signal* for **every** lane provider
(charter item 2, the root reframe) — is **not yet falsifiably delivered** for
non-codex lanes. Each maps onto a named `needs_revision` trigger in the
adjudication rubric (an L1 gauge read off a credential the live lane never
reloaded; an absence/coverage mechanism asserted without a concrete buildable
mechanism). The gate stays blocked.

This is consistent with the schema's own clearing rule (RFC 0094 §5 Check-B): a
clearing verdict requires every recorded challenge to be `landed_and_rebutted`
and none left `landed_unrebutted`. Neither F1 nor F2 can be honestly recorded as
rebutted (the spec offers no answer to them) or as not-material (they attack the
central deliverable), so a clearing verdict is unavailable on the merits — not
merely declined.

The Holder spec is **strong where it counts most**, and any revision must
*build on* it, not regress it (see "Credited strengths"). The gate does not
clear only because the *replacement* provider-agnostic guarantee the spec
substitutes for the (correctly-refused) fake non-codex heartbeat is itself
incomplete on two independent axes — **coverage (F1)** and **resolution (F2)**.

## Provenance note — why cycle 2 reaches the same verdict

This is a re-opened adjudication round (attempt 2). The dialogue trajectory it
reviews is **identical** to cycle 1's: per the run's own provenance, `holder`,
`falsifier_1`, and `falsifier_2` each ran exactly once and completed, and the
`HOLDER.md` spec has a single publication in the run history — **it was never
revised between the cycles.** The cycle-1 verdict was `needs_revision`; the
workflow's revision cycle (`on_verdict: needs_revision → falsifier_1`,
`max_iterations: 1`) re-opened the adjudication rather than returning the spec
to the Holder for an edit. Re-attacking or re-adjudicating a frozen spec cannot
manufacture a rebuttal the spec does not contain, so the substance of the
verdict is unchanged. What is *new* in cycle 2 is this explicit finding that the
spec is frozen and the run therefore cannot self-clear — see "Path forward."

## Challenge ledger

### F1 — `falsifier-reviewer-001`: the MVP census cannot honestly cover non-expiring (api_key) non-codex lanes

- **Claim challenged.** The MVP is provider-agnostic because Layer 1 emits
  `striatum_lane_cred_seconds_to_expiry{lane,kind}` and the census rule
  `count(striatum_lane_cred_seconds_to_expiry) < striatum_lane_auth_expected_count`
  "guarantees a non-codex lane that vanishes pages" (HOLDER OQ1 + lines 234-243,
  316-319), with Layer 3 codex-scoped, Layer 2 deferred, and the explicit
  admission that `kind="api_key"` credentials have no expiry, emit no expiry
  series, and "rely on … the census rule" (HOLDER lines 278-283).
- **Material?** **Yes.** It attacks charter item 2 (the absence-of-success
  signal must be downstream of a real success for *every* provider) and the root
  reframe (no-series must page as loudly as a stale series). A non-codex
  `api_key` lane is in scope — `kind` is a closed enum that *includes* `api_key`
  (HOLDER OQ3) — and the census rule is the spec's *only* claimed coverage for
  it.
- **Rebutted, or stands?** **Stands unrebutted (`landed_unrebutted`).** The
  contradiction is internal to the spec and the revised spec would have to
  resolve it: a scalar `expected_count` counted against `seconds_to_expiry`
  series forces a healthy `api_key` lane into one of two failing states —
  **counted ⇒ permanent paging from day zero** (it can never produce the counted
  series) or **excluded ⇒ silently dropped** from the "all lane providers"
  contract. The spec names no per-lane roster-presence vector (e.g.
  `striatum_lane_auth_expected{lane,provider,kind} 1`), and `count(...) < scalar`
  is an **aggregate-only rule carrying no `lane` label** — flatly contradicting
  the spec's own promise (HOLDER lines 294-300) that every alert carries `lane`
  and "no aggregate-only rule [exists] so one dead lane can never average into
  green." The falsifier's own strongest-rebuttal-on-the-Holder's-behalf (scope
  to OAuth / refine PromQL later) is correctly judged insufficient: a later
  PromQL refinement cannot invent an absent per-lane vector the surface never
  named, and a doctor warning is not the Slack alert contract the spec leans on.
- **Disposition.** Material defect, unrebutted → drives `needs_revision`. Fix
  recorded as finding **F1** (`closest_acceptable_answer`).

### F2 — `falsifier-reviewer-002`: Layer 1 does not prove it reads the credential a non-codex lane presents (FA-6 overclaim)

- **Claim challenged.** Layer 1 is the provider-agnostic MVP backbone that reads
  the *same* credential the lane presents at runtime, by reusing the
  `ResolveAuthHome` / `SanitizeEnv` / `sudo -n -u <lane>` shape from
  `checkCodexOfflineAuth` (HOLDER lines 56, 119-126, 285-292; FA-6), with the
  Backbone roster supplying `credential_path_template` (HOLDER lines 336-345).
- **Material?** **Yes.** This is the SEED's explicitly-named load-bearing L1 risk
  ("read the *same* credential the lane presents at runtime, not a fresh file the
  live process never reloaded — a healthy-looking gauge over a lane coasting to
  death") and a named `needs_revision` trigger in the rubric. It is also the
  motivating incident itself (the silently-expired non-codex *claude* token).
- **Rebutted, or stands?** **Stands unrebutted (`landed_unrebutted`).** The cited
  mechanism is **codex-only** in current source, per the SEED anchor table and
  the falsifier's citations: `ResolveAuthHome` honors `CODEX_HOME` only for codex
  and falls back to bare `HOME` for every other provider; the sanitized preflight
  env allowlist carries no provider-specific credential-home / token-path
  variables and is applied *after* merging `LaunchEnv`, so a provider-specific
  credential location can be stripped before the resolver sees it. The spec's
  FA-6 mechanism ("reuse the codex shape") therefore **cannot resolve
  claude/agy/gemini runtime credentials**, and its only fallback — trust the
  operator-declared roster `credential_path_template` — reintroduces precisely
  the failure the SEED told the run to falsify: a fresh file at the declared path
  makes `seconds_to_expiry{lane="claude"}` read green while the real claude CLI
  resolves a different, expired credential. GD-2 (a random decoy file does not
  move the gauge) is correctly judged too weak: it does not prove the *sampled*
  file is the CLI's actual credential source. The falsifier's strongest rebuttal
  on the Holder's behalf (roster is the authoritative mapping; build adds
  parsers) is correctly rejected: a roster path is configuration, not proof of
  runtime resolution.
- **Disposition.** Material defect, unrebutted → drives `needs_revision`. Fix
  recorded as finding **F2** (`closest_acceptable_answer`).

## Credited strengths (preserve these through any revision)

Neither falsifier refuted these; both explicitly credit the core. A revision (or
a committer authorized to fold these findings in) must not regress them:

- **The fake-heartbeat trap was correctly refused.** Layer 3 is honestly
  codex-scoped — it fires only where a real `Check().Passed()` exists — and the
  spec refuses to synthesize an `auth_last_success` series for providers that
  never run a real check. Both falsifiers credit this ("successfully avoids lying
  about non-codex Layer 3 heartbeats" / "correctly avoids fake non-codex
  heartbeats"). FA-5 / `TestAuthSuccessEventOnlyOnPassedCodex` is sound.
- **OQ4 (threshold source) is resolved** concretely: operator-declared in the
  roster, exported as a gauge, *not* auto-derived from observed lifetime — with
  the correct circularity argument (a degrading credential must not move its own
  goalposts). FA-4 / `TestThresholdFromRosterNotObserved` is a real falsifying
  test. Unattacked.
- **OQ3 (cardinality) gives a concrete numeric cap**: `{lane,kind}`, `id`
  dropped, per-family budget 32 → ≤33 series incl. `{lane="other"}`, overflow
  visible via `striatum_metrics_cardinality_clipped_total`. FA-3 /
  `TestLaneCredSeriesBudget`. Unattacked. (F1's per-lane vector must land inside
  this same budget.)
- **FA-7 (no behavior change — Non-Goal / RFC 0143 boundary) and the
  product-boundary checklist** (read-only, pull-only, no hosted/cloud/push, no
  private-data labels) are well-argued and unattacked.
- **OQ1/OQ2 are resolved as concrete decisions** (MVP = L1 + codex-scoped L3 +
  census; L2 deferred; L1 sampler in-daemon on the sweep fold, L2 prober as the
  external `striatum-lane-auth-prober.timer`). They are sound *except* where F1/F2
  puncture the provider-agnostic coverage claim that OQ1 rests on.

## What a clearing revision MUST fix

The revision must close **both** material gaps, or honestly **narrow** the
headline claim. Either route clears; the current text does neither.

1. **Close F2 — a provider-agnostic resolver *contract*, not a roster path.** For
   each non-codex MVP provider kept in scope (at minimum claude) name the exact
   credential source and precedence the lane CLI actually resolves — tied to
   adapter identity, the launch-env keys that matter, and which env keys are
   intentionally forbidden. Make the sampler **fail closed** into an explicit
   `resolver_mismatch` / missing-sample state (which must itself page via the
   absence rule) when the runtime source cannot be proven, instead of serving a
   green gauge from a fallback path. Add provider-specific resolver tests for
   non-codex lanes, including a provider-specific launch env pointing at one
   credential while `HOME` holds a *fresher* decoy (the gauge must track the
   credential the CLI uses, not the decoy). **Or** narrow the Layer 1 claim to
   "operator-declared expiry telemetry for known file-backed credentials" and
   stop asserting it closes the provider-agnostic same-credential hole.

2. **Close F1 — per-lane roster-presence vector + non-expiring coverage.**
   Replace the scalar-only `expected_count` census with a per-lane expected
   vector (e.g. `striatum_lane_auth_expected{lane,provider,kind} 1`) so the
   absence rule compares expected-vs-observed lanes with `unless` / `absent`
   semantics that **preserve the `lane` label** (restoring per-lane attribution
   and the spec's own no-aggregate-only-rule guarantee). Either bring a positive
   non-expiring-credential signal into the MVP for `api_key` lanes (downstream of
   a real provider success), **or** explicitly scope the MVP to expiring OAuth
   credentials and mark non-codex `api_key` lanes a deferred/accepted risk — and
   in that case drop the "provider-agnostic for *all* lane providers" claim. Add
   a **healthy** non-codex `api_key` lane test so the MVP cannot pass by either
   permanent-paging or silent omission. Keep the new vector inside the OQ3 budget.

3. **Internal consistency.** After (1)/(2), restate FA-1, FA-6, the metric
   surface table, and the "every alert carries `lane`, no aggregate-only rule"
   claim so they are mutually consistent — today FA-6 and the census rule
   over-claim relative to the named surfaces.

Items resolved (OQ4, OQ3 numerics, FA-5 codex scoping, FA-7 / Non-Goal
compliance, OQ1/OQ2 mechanism choices) need not be re-litigated and should carry
forward intact.

## Path forward (operator) — the gate cannot self-clear as configured

The honest verdict is `needs_revision`, but two run-level facts mean this run
**cannot reach a clearing verdict on its own**, and the operator should choose
how to proceed:

- The single revision iteration (`cycles[].max_iterations: 1`) is **consumed**,
  and the workflow's `needs_revision` edge returns to `falsifier_1`, **not** to
  the Holder — so no step in this run can edit `HOLDER.md` to actually answer
  F1/F2. The spec is frozen.
- A clearing verdict is therefore structurally unreachable here without one of:
  **(a)** the operator re-opening the `holder` job (or running a follow-up
  design cycle) so the spec is genuinely revised along F1/F2, then re-attacked
  and re-adjudicated; or **(b)** the operator explicitly authorizing the
  `commit_proposal` committer to *fold F1/F2 into the committed `PROPOSAL.md`*
  (the committer prompt already mandates closing the codex-only hole "for every
  lane provider" and incorporating material challenges) — with this ledger's F1
  and F2 `closest_acceptable_answer` as the binding acceptance criteria, and a
  recorded accepted-risk entry if the narrowing route is taken; or **(c)**
  accepting that the design is sound-but-overclaiming and deferring the
  provider-agnostic completion to the `rfc-0162-build` run with F1/F2 logged as
  explicit accepted risks.

This is flagged separately to the operator via `session.report`. It is a
workflow-shape observation, not a defect in the spec's engineering, which is
strong.

---
<sub>Adjudicator collaboration ledger for the RFC 0162 falsification-gate design
run, cycle 2. The ledger verdict — not falsifier completion — gates the phase:
`needs_revision` keeps the commit phase blocked. The spec is frozen this run, so
clearing requires operator action (re-open the Holder, authorize the committer
to fold F1/F2, or defer them as recorded accepted risks).</sub>

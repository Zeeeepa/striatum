---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Analysis of TODO 56: Default Auto-Finalize Policy

author: analyst-claude-code-001
Date: 2026-05-21

## Objective

Prepare decision options for the default / live auto-finalize policy and
the dogfood-acceptance bar in TODO 56 (Architecture remediation Phase 8 /
RFC 0051). The bounded slice has landed: `recovery.auto_finalize` runs
dry-run by default, runs live only when the workflow opts in, projects
eligibility through status/dashboard/web, and never overrides the
workflow opt-in from the sweep. This analysis frames the remaining
product choices for the human principal; it does not decide them.

## Current State (Inputs)

- `recovery.auto_finalize` is a daemon-only method registered in
  `src/striatum/daemon_rpc/daemon_methods.json` and implemented at
  `src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py`.
- Live mode is gated by `workflow.recovery.auto_finalize.enabled=true`
  (or, for the CLI only, an explicit `--force`). The sweep
  (`recovery.sweep`) never supplies `--force` and never makes live
  finalize global (`docs/SPEC.md:967-989`).
- Per-candidate eligibility requires: run state `running`, an active
  lease that is not expired, an active session, an attached queue
  message in `claimed`/`acked`, every required `expected_artifacts[]`
  entry present on disk, mtime stable past the grace window
  (default 30s), front matter valid against the declared kind,
  the byline exactly equal to the work-packet's expected author
  line, no logical-name conflict, and clean `process_executions`
  lane evidence unless the artifact is operator-bylined or the caller
  opts into `allow_no_process_execution`.
- Acceptance pin: PG sweep regression where three valid written review
  findings auto-finalize with zero operator-on-behalf or override
  provenance (`docs/ROADMAP.md:753-755`).
- Operator-on-behalf publishes remain available via RFC 0046 V1; the
  v1.48.1 wrapper auth fix relieved the urgency that drove RFC 0051.
- TODO 39 (RFC 0051 V1) is **🟡 daemon method slice landed**; TODO 56
  is **⏳ default policy blocked on live dogfood confidence**.

---

## 1. Dry-Run Default vs. Workflow-Opt-In Live Behavior

Today's default posture is dry-run-visible everywhere; live mode is a
per-workflow knob (`recovery.auto_finalize.enabled=true`). The decision
is how to evolve that posture.

- **Option A: Keep current posture indefinitely** — dry-run default
  global, live opt-in per workflow. Status/dashboard/web continue to
  show the dry-run projection so authors can see what *would* auto-
  finalize. Pros: no behavior change, no regressions to existing
  audits, the operator-on-behalf path retains its symmetry. Cons:
  every workflow author who wants the lane-stall relief has to copy
  the opt-in stanza into their `workflow.json`; the safety net is
  unused by default and the operator-on-behalf load can resurface in
  any workflow that forgets to opt in.

- **Option B: Default-on with explicit opt-out** — workflows get
  `recovery.auto_finalize.enabled=true` by default; authors can set
  `enabled=false` to suppress. Pros: removes the per-workflow boilerplate
  for the common case; operator-on-behalf becomes the genuine exception.
  Cons: changes the meaning of every existing workflow snapshot that
  did not opt in; the eligibility gates and lane-evidence requirements
  become the only thing standing between a stall and an automatic
  publish; would require a written rollback plan if a regression slips.

- **Option C: Two-step rollout** — promote a small, audited set of
  workflow shapes (e.g. dogfood-only, examples/, internal harness
  fixtures) to a default-on tier first; leave external/registered
  target-repo workflows on opt-in until a follow-up review. Pros:
  preserves operator control over target-repo behavior; lets us
  gather one more round of evidence on shapes Striatum already owns.
  Cons: introduces a "workflow tier" concept that did not exist
  before and that needs to be documented; risks bit-rot if the
  promoted set is not refreshed.

- **Option D: Defer the policy change** — keep dry-run-only as the
  global default, leave the opt-in untouched, and revisit after a
  defined-size run window (e.g. 25 attested-lane finalizations across
  ≥3 workflow shapes with zero contested audit-chain events). Pros:
  evidence-first; matches RFC 0051's stated boundary
  (`docs/rfcs/0051-auto-finalize-from-frontmatter.md` Migration). Cons:
  keeps the lane-stall toll on workflows that have not opted in;
  defers the conversation rather than settling it.

---

## 2. Safety Conditions for Live Auto-Finalize

These conditions are already enforced in the handler; the decision is
which of them are non-negotiable invariants vs. operator-tunable knobs.

- **A. Required-artifact completeness.** Live finalize fires only if
  every `expected_artifacts[required=true]` entry exists on disk; if
  any are missing, the candidate is skipped. *Recommend non-negotiable.*

- **B. Stable mtime window.** Default 30s grace
  (`DEFAULT_MTIME_GRACE_SECONDS`). Decision: keep 30s, lengthen for
  conservatism, or make it per-workflow-tunable. Trade-off: shorter
  grace makes lane-stall recovery faster but risks finalizing a partial
  write; longer grace prolongs the stall window.

- **C. Front-matter + byline strictness.** Schema validation (`finding`
  needs `verdict_intent`; others must validate against the kind schema)
  and an exact `expected_author_line` match. *Recommend non-negotiable.*

- **D. Lane evidence.** A clean `process_executions` row for the
  session is required unless the byline is `author: operator` or the
  caller passes `allow_no_process_execution`. Decision: keep
  `allow_no_process_execution` CLI-only (today's posture), or expose
  it as a workflow knob for narrow lanes (e.g. stub adapters)? The
  sweep path never sets it; only the CLI can.

- **E. Run/lease/session/message liveness.** Run `running`, lease
  active and unexpired, session active, queue message in
  `claimed`/`acked`. *Recommend non-negotiable.* Any drift here is a
  symptom of a different bug.

- **F. Logical-name idempotency.** Conflict on logical name with
  different content refuses; same content + same path is treated as
  `already_published`. *Recommend non-negotiable.*

- **G. Transcript exclusion.** `kind == transcript` is never
  auto-finalized. *Recommend non-negotiable.*

The choice surface: of A–G, which are pure invariants (no operator
override exposed) and which are operator-tunable knobs whose defaults
remain conservative? The conservative reading is A, C, E, F, G are
invariants; B (grace) and D (process-execution evidence override) are
knobs.

---

## 3. Visibility Requirements (Status / Dashboard / Web)

The bounded slice already projects `auto_finalize_dry_run` previews
through status and dashboard surfaces, and the web recovery panel
renders the same preview without enabling live finalize globally.
Open visibility decisions:

- **Provenance marker prominence.** Auto-finalized artifacts already
  emit `artifact.auto_finalized` and `job.auto_finalized` events and
  carry `lane_finalization=auto_from_artifact` in the job event
  payload; PG evidence summaries expose `publish_origin=auto_from_artifact`.
  Decision: do status/run-summary surfaces need to call this out by
  default (visible in `striatum status` / dashboard / web run view),
  or remain an audit-trail detail surfaced only on drill-down?

- **Refusal-reason fidelity.** Skipped candidates carry a single
  `reason` string plus per-artifact refusals. Decision: keep current
  granularity, or extend the projection to include the *cause class*
  (e.g. `mtime_grace`, `byline_mismatch`, `lane_evidence_missing`,
  `frontmatter_invalid`) so dashboards/web can group and filter?

- **Per-candidate "would publish" preview.** The dry-run projection
  surfaces eligible candidates with logical name, kind, path, and
  byline. Decision: should the preview also include the SHA-256 +
  size + computed verdict (for finding kinds) so the operator can
  diff against the live finalize result without re-running?

- **Audit-chain coupling.** Auto-finalize events thread the audit
  chain like any other state mutation. Decision: do we need a
  dedicated audit query / CLI verb (`striatum audit auto-finalize
  --run-id`) that returns just the auto-finalize lineage, or does
  the existing `striatum status / why` / web event timeline suffice?

- **Operator-on-behalf differentiation.** Three publish paths must
  remain visually distinguishable: agent-called, runner-auto-
  finalized, operator-on-behalf. The current per-event payload
  carries this. Decision: make this distinction a first-class column
  in dashboards/web run-summary tables, or keep it in the event
  drill-down only?

---

## 4. Recovery Scheduler Behavior

The Go production-daemon recovery scheduler invokes `recovery.sweep`,
which delegates to `recovery.auto_finalize` *before* lazy lease expiry
*only when the workflow opted in* and never supplies `--force`. The
relevant decisions are how aggressive that scheduler should be.

- **Cadence vs. lease heartbeat.** RFC 0051 §Design ties auto-finalize
  to the existing lease-heartbeat tick. Decision: keep that cadence
  (cheap, predictable, lease-aligned), bump it to a faster dedicated
  tick (sooner recovery from gemini-class stalls but more PG load),
  or make cadence per-workflow-tunable?

- **Eligibility-before-expiry ordering.** Today auto-finalize runs
  before lazy lease expiry inside the sweep — the agent's still-active
  lease is what makes finalization safe. Decision: keep that ordering
  as an invariant (avoid the "lease expired then finalized on stale
  state" race), or expose a workflow toggle to also finalize artifacts
  whose lease just expired but were stat-stable for longer than the
  grace?

- **Scheduler-vs-CLI parity.** The CLI form (`striatum recovery
  auto-finalize --run-id ...`) accepts `--force` and
  `--allow-no-process-execution`; the sweep does not. Decision: keep
  that asymmetry (sweep is always policy-bounded; CLI is the operator
  break-glass), or surface a workflow toggle for force-class behavior
  in the sweep when the workflow author already accepted the risk?

- **Failure isolation.** Live finalize wraps each candidate in a try /
  except and records `live auto-finalize failed: <exc>` in skipped[];
  the sweep does not abort on individual failures. Decision: keep
  failure isolation as current, or add a configurable circuit-breaker
  that pauses the sweep after N consecutive auto-finalize failures so
  operators can investigate before the same regression fires across
  many jobs?

- **Dry-run cost.** The dry-run projection is cheap-but-not-free; it
  walks every active session's expected artifacts every tick.
  Decision: keep unconditional dry-run projection, or gate the
  projection behind a `recovery.auto_finalize.preview=true` workflow
  knob so workflows that never opt in pay nothing?

---

## 5. Dogfood Evidence — What's Enough?

RFC 0051 acceptance lists four regression pins plus "one dogfood run
end-to-end with **zero** operator-on-behalf publishes on jobs whose
agents wrote valid artifacts." The PG sweep acceptance regression
(three valid written review findings auto-finalize with zero
operator-on-behalf or override) covers the structural case. The
decision is whether that bar is enough to flip a default-on policy or
whether we want broader live evidence first.

- **Option A: Ship default-on now.** Treat the regression pin +
  v1.48.1 wrapper auth fix evidence (zero operator-on-behalf publishes
  in gh-16 across three lanes) as sufficient. Pros: minimum delay;
  the safety net was already designed to be safe even if always-on.
  Cons: a single dogfood is a narrow window; failure modes that only
  surface under genuine multi-lane traffic could go unnoticed.

- **Option B: Land N live dogfood successes before flipping.** Adopt
  an explicit evidence target — e.g. 3 dogfoods across ≥2 lane shapes
  (gemini-class artifact-then-stall *and* attested-CLI-completion) with
  zero contested audit-chain events and zero operator overrides
  triggered by auto-finalize false positives. Pros: tractable;
  measurable; aligns with how RFC 0046 V1 / RFC 0047 / RFC 0050 were
  graduated. Cons: requires the runway to actually run those dogfoods
  in opt-in mode first.

- **Option C: Hold for a long-window field test.** Require ≥30 days
  of opt-in live use across the internal dogfood corpus + an explicit
  audit-chain review pass before flipping defaults. Pros: maximally
  conservative; deepest evidence base. Cons: postpones the relief
  RFC 0051 was designed to deliver; risks the policy decision
  becoming permanently deferred.

- **Option D: Require a specific harness-improvement coverage pass.**
  Block the flip until additional regression coverage lands for the
  known failure modes the V1 implementation already enumerates:
  malformed frontmatter, byline mismatch, partial expected_artifacts,
  expired lease, in-flight artifact rewrite during grace window,
  and conflict on logical name with different content. Pros: turns
  the policy decision into a coverage gate, not a calendar gate.
  Cons: assumes test coverage is the binding constraint, which may
  not match field experience.

The dogfood-acceptance bar in TODO 56 is "live dogfood confidence";
the decision is to give that phrase a concrete shape and to choose
between the calendar gate, the count gate, and the coverage gate.

---

## 6. Cross-Cutting Considerations

- **Composability with operator-on-behalf.** Auto-finalize and
  operator-on-behalf are now first-class peer paths. A default-on
  flip should not weaken either path's audit-chain distinguishability.

- **Cross-repo workflows.** Per-repo opt-in semantics are clear under
  RFC 0028 / RFC 0032; a default-on flip needs an explicit decision
  on whether external target-repo workflows inherit the new default
  or stay on opt-in until their author opts in.

- **Backwards-compat for older snapshots.** Workflow snapshots are
  durable. Decision: if defaults change, are existing snapshots
  re-evaluated against the new default (making a snapshot's effective
  policy a function of the daemon version) or pinned to the policy
  in effect at snapshot time?

- **Decision-log discipline.** Any policy change should land a
  `decision` artifact and update `docs/DECISION_LOG.md`, `docs/TODO.md`
  TODO 56 status, and `docs/ROADMAP.md` §4.12.

---

## 7. Recommendation Frame (for the human principal)

The recommended decision shape is two adjacent decisions:

1. **Default policy choice** — pick between Options A / B / C / D in
   §1 (dry-run-default-with-opt-in vs. default-on-with-opt-out vs.
   two-tier vs. defer). The other choices flow from this one.
2. **Evidence bar** — pick between Options A / B / C / D in §5 (ship
   now vs. N-dogfood gate vs. 30-day window vs. coverage gate). This
   binds when the §1 choice can actually flip.

Adjacent but separable: the §2 safety-condition split (invariant vs.
knob), §3 visibility extensions, and §4 scheduler-cadence choices can
each be ratified independently once the §1 / §5 axes are decided.

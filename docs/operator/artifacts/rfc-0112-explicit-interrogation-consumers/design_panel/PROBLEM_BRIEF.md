# RFC 0112 Problem Brief — Explicit Interrogation Consumers

author: problem-framer-claude-opus-4.8-001
date: 2026-06-05
run: rfc-0112-explicit-interrogation-consumers-design-panel

This brief frames the implementation problem for RFC 0112
(`docs/rfcs/0112-explicit-interrogation-consumers.md`) so the proposal panel can
produce an implementation-ready plan. It states the failure, the binding
constraints, the disallowed workaround, the decision criteria, and the six
questions every proposal must answer.

## 1. The failure, precisely

The `awaiting_interrogation` preserved-context window is owned by an inferred
consumer set: the **direct dependents** of the interrogable job.
`interrogationConsumersPending` (`go/pkg/mutations/interrogation.go:516`) walks
`job_dependencies` rows where `depends_on_job_id = <interrogable job>` and keeps
the target session alive while any such job is outside the terminal set
`('completed','failed','canceled','skipped','waiting_human')`
(`interrogation.go:373`).

That predicate is correct for the original interrogating-panel shape, where
reviewer jobs depend directly on the interrogable job. It is wrong for
`adjudicated_constraint_extraction` (ACE), whose generator
(`compileAdjudicatedConstraintExtraction`, `go/pkg/workflowgenerate/generate.go:899`)
emits:

```text
convener_draft (interrogable: true, build)
  -> convener_synthesis (phase_synthesis gate)
    -> cross_examiner_1..N (build)
```

`convener_draft` (`generate.go:927`) sits **behind** its phase's
`convener_synthesis` gate, as RFC 0045 phase rules require. Its only direct
dependent is `convener_synthesis`. So the moment `convener_synthesis` records an
accepting verdict and goes terminal, the pending-consumer predicate is empty and
the window machinery closes the convener's session with
`interrogation_window_closed` — **before the cross-examiners, the jobs that
exist to consume that preserved context, are even claimable**. The first
cross-examiner's `interrogation.open` then returns the non-wedging
`interrogation_unavailable` signal with `reason: panel_window_closed`
(`interrogation.go:767`). This reproduced live in the RFC 0106 ACE graduation
experiment.

A second, latent half of the failure: the release hook is
`releaseInterrogationTargetForCompletedReview`, invoked only from
review-verdict paths (`go/pkg/mutations/review.go:326,569,617`). ACE
cross-examiners are ordinary `build` jobs that terminalize via `work.complete`,
not via verdicts. Even if the window survived the synthesis gate, **nothing
would close it** when the true consumers finish — the window would leak, which
is exactly the silent-wedge class the RFC 0105 gate exists to refuse.

So the problem is two-sided: the consumer set is too narrow (closes too early),
and the closure hook is too narrow (would close too late or never).

## 2. Hard constraints

From **RFC 0082** (interrogation sessions, accepted D138):

- C-082-1: `interrogation.open/ask/answer/close` remains the only mutation
  surface. No new daemon RPC family for interrogation (also RFC 0112 non-goal).
- C-082-2: Interrogation turns are curated authored records (D028). No
  transcript capture, no raw provider output, ever.
- C-082-3: Daemon-owned PostgreSQL is the sole live-state authority and the
  daemon the sole writer. Window state is daemon state, not lane behavior.
- C-082-4: A target must be live (attested, or in the `awaiting_interrogation`
  agent-loop window) to be opened against; single-run scope.

From **RFC 0095** (revision-safe lifecycle):

- C-095-1: The window is **panel-owned**, never owned by a single interrogation
  thread; `interrogation.close` ends a thread, the target re-arms while
  consumers remain pending. Lane-independent.
- C-095-2: Revision reopen retires the prior target session and any open
  interrogations against it; the new attempt produces a **fresh**
  `awaiting_interrogation` target. RFC 0112 must preserve this: explicit
  consumers re-blocked by a `needs_revision` cascade must hold the window open
  for the *fresh attempt's* session, not resurrect the old one.
- C-095-3: A `closed` session may never claim work; attempt scoping (lease,
  message, artifact, verdict valid only for `jobs.attempt`) must not be
  weakened by the new consumer relation.
- C-095-4: The existing close guards stay: never close while an interrogation
  is open against the target or the target holds an active lease
  (`interrogation.go:397-416`).

From **RFC 0098** (ACE shape semantics):

- C-098-1: ACE is an RFC 0045 phased workflow; cross-phase edges must originate
  from the source phase's `phase_synthesis` job. The generated ACE graph shape
  is fixed and phase-valid; the fix may not alter it.
- C-098-2: Cross-examiners interrogating the convener's preserved context is
  the load-bearing point of the shape ("unanswered interrogation is evidence",
  §3). A fix that makes cross-exam artifact-only defeats the shape.
- C-098-3: The bounded `needs_revision` cycle (`adjudicate -> convener_draft`,
  `max_cycles`) and cycle-aware logical names stay as generated.

From **RFC 0105** (reliability harness, accepted D161):

- C-105-1: The proof bar is a hermetic per-shape fixture driving the real
  lifecycle through the in-process daemon's **production handlers** with the
  fake agent — not a unit test of the predicate.
- C-105-2: Every fixture cell must complete or escalate loudly within budget;
  a leaked window or a job stuck with no lease/session/escalation is a gate
  failure. The fixture becomes a standing CI gate (`make check`, PG-gated).

From **RFC 0106** (support tiers, accepted D162):

- C-106-1: ACE stays `experimental` until a genuine green RFC 0105 fixture
  exists; the guard test means the tier cannot lie. ACE graduation itself is
  **out of scope** for RFC 0112 (its non-goal), but the design must make the
  graduation fixture *possible*.
- C-106-2: New-shape freeze: the fix must repair ACE in place, not author a
  replacement shape. ACE's graph is genuinely distinct (the D169 isomorphism
  escape hatch does not apply).
- C-106-3: Existing direct-dependent interrogating-panel behavior and tests
  must pass unchanged with no `interrogation_targets` declared (RFC 0112 AC 5).

## 3. Why fake `convener_draft -> cross_examiner_*` edges are disallowed

The obvious one-line "fix" — adding `job_dependencies` edges from
`convener_draft` to each cross-examiner so the existing predicate sees them —
is rejected, for stacked reasons:

1. **Phase-rule violation.** RFC 0045 requires cross-phase edges to originate
   from the source phase's synthesis job. `convener_draft` is not its phase's
   synthesis job; edges from it into the `cross_exam` phase are exactly what
   `run.prepare` phase validation exists to forbid.
2. **Dependency edges are load-bearing scheduling truth.** They drive
   `dependenciesSatisfied`, claimability, the RFC 0095 §3 revision-reopen
   transitive-downstream reset, recovery decisions, and dashboards. Encoding an
   interrogation-liveness concern as a scheduling edge changes all of those
   semantics at once — e.g. the reopen cascade's reset set would silently grow.
3. **Concern conflation.** "This job consumes that job's live context" is not
   "this job is gated on that job's artifact." Conflating them means the next
   phase-gated shape with non-adjacent consumers re-hits the same wall. The fix
   must make the consumer relation a first-class, declared concept.
4. **RFC 0112 explicitly forbids it** (non-goal: "No graph-shape workaround
   that adds false dependency edges solely to keep target sessions alive").

## 4. Decision criteria for the winning plan

Score proposals against these, in rough priority order:

1. **Correct window lifetime, both directions.** The convener window survives
   the `convener_synthesis` gate into the cross-examiner fan-out, and closes
   once all consumers (direct ∪ explicit) are terminal with no open
   interrogations and no active lease. No early close; no leak.
2. **Closure-path completeness.** The generalized release hook fires on
   *every* path that terminalizes a potential consumer: `work.complete`,
   `review.verdict`/`submit-review`, `override-verdict`, and recovery/cancel
   transitions. A plan that enumerates call sites and shows how a missed path
   would be caught (guard test or shared helper choke point) wins over one
   that patches the visible two.
3. **Backward compatibility.** Zero behavior change for workflows that declare
   no `interrogation_targets`; existing panel tests pass untouched.
4. **Graph soundness + validation teeth.** Validation rejects targets that are
   missing, self-referential, not `interrogable: true`, or not reachable
   downstream of the target; honest lint (warn, not block) for soft issues.
5. **RFC 0095 coherence under revision.** A `needs_revision` reopen of
   `convener_draft` retires the old session, re-blocks the fan-out and join,
   and the *fresh* attempt session is held open for the re-run consumers.
   The plan must say explicitly how explicit consumers interact with
   `reopenJobForAttempt`-style cascades.
6. **Packet legibility.** Claimed consumer packets carry resolved
   `target_session_id` plus a legible `available`/`unavailable`/`not_ready`
   state so a lane never burns a state-changing call to discover absence; the
   unavailable path stays non-wedging (#131 signal preserved).
7. **Provable via RFC 0105 machinery.** The plan names the hermetic fixtures —
   ACE happy path with ≥1 genuine interrogation per cross-examiner, revision
   reopen with fresh-window re-interrogation, and a dead-lane fault during the
   re-cascade (RFC 0112 AC 2–4) — and where they live in
   `go/pkg/adapterconformance`.
8. **Smallest blast radius.** No new RPC family, no new aggregate root, no
   schema change beyond what consumer resolution strictly needs, D028 intact.
   Prefer extending the existing predicate/hook over parallel machinery.
9. **Honest envelope accounting.** This panel's downstream jobs write only
   under `docs/operator/artifacts/rfc-0112-explicit-interrogation-consumers/design_panel/`.
   The real implementation touches `go/pkg/workflowgenerate`,
   `go/pkg/mutations`, workflow validation/lint, conformance fixtures, and
   `docs/reference/{spec,ubiquitous-language}.md` — proposals must state that
   follow-up scope explicitly rather than pretend the frozen panel scope
   suffices.

## 5. The six questions the panel must answer

1. **`interrogation_targets` field name/shape.** Confirm (or amend, with
   rationale) the V1 per-entry shape `{workflow_job_id, required}` on the
   consumer job, including how unknown entry fields are treated (lint warning
   vs hard error) and where the field is documented in the workflow schema.
2. **`required` semantics.** What does `required: true` mean in V1 — packet
   instruction strength only, with the non-wedging `interrogation_unavailable`
   fallback — and what, concretely, is deferred to a later hard-gate version
   (RFC 0112 OQ 1)? Must a surfaced-unavailable target leave a durable record
   (OQ 3)?
3. **Multiple targets in V1.** Does validation allow N targets per consumer
   job now, or cap V1 at one target until a real shape needs more (RFC 0112
   OQ 2)? Whichever way: what is the predicate/packet behavior with several
   targets in different states?
4. **Terminal paths for the release hook.** The exact set of production
   mutation paths that must invoke the generalized
   `releaseInterrogationTargetsForTerminalConsumer`, how recovery/cancel
   transitions are covered, and what guard keeps a future terminalizing path
   from bypassing it.
5. **The RFC 0105 fixture that proves ACE can graduate.** The concrete fixture
   plan — shape cells, fault injections, assertions — that satisfies RFC 0112
   AC 2–4 and would later let RFC 0106 graduate ACE on genuine (non-isomorphic)
   coverage. The fixture is in scope to design now even though graduation
   itself is not.
6. **The work-packet namespace.** Where the resolved projection lives (the RFC
   proposes `context.interrogation_targets[]`), its exact fields
   (`target_session_id`, `state`, `instruction`), how `not_ready` differs from
   `unavailable`, and how the block stays consistent with the existing packet
   `context` contract consumed by lanes.

## 6. Non-goals (binding on all proposals)

- No ACE support-tier graduation in this RFC (fixture design yes; tier flip no).
- No new daemon RPC family, hosted service, telemetry, or transcript capture.
- No semantic scoring of interrogation quality — liveness, addressing, and
  closure only.
- No fake dependency edges (§3).
- No edits in this panel to RFC status, the decision log,
  `docs/reference/spec.md`, source code, or VERSION — the panel produces a
  plan; implementation lands separately with its own scope.

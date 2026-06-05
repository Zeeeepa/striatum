# RFC 0112: Explicit interrogation consumers for phased collaboration shapes

Status: proposed
Date: 2026-06-05
author: proposer-codex-gpt-5-001
Context: [RFC 0082](0082-interrogation-sessions.md),
[RFC 0095](0095-revision-safe-workflow-lifecycle.md),
[RFC 0098](0098-adjudicated-constraint-extraction-loop.md),
[RFC 0106](0106-workflow-shape-support-tiers.md);
`go/pkg/mutations/interrogation.go`,
`go/pkg/mutations/review.go`, `go/pkg/workflowgenerate/generate.go`.

## Problem

The current panel-owned interrogation window is inferred from graph adjacency:
the pending consumers of an interrogable job are the jobs that directly depend on
that job. That was correct for the original interrogating-panel shape, where the
interrogable synthesis feeds reviewer jobs directly.

`adjudicated_constraint_extraction` (ACE) breaks that assumption. Its
`convener_draft` job is `interrogable: true`, but the graph must pass through the
same phase's `convener_synthesis` `phase_synthesis` gate before the next
phase's cross-examiner fan-out can start. The generated shape is:

```text
convener_draft -> convener_synthesis -> {cross_examiner_1..N}
```

The RFC 0106 ACE graduation experiment reproduced the failure: after
`convener_synthesis` records an accepting verdict, the first cross-examiner's
`interrogation.open` against the convener session returns
`interrogation_unavailable` with `reason: panel_window_closed`. The system closed
the preserved-context window before the jobs that exist to cross-examine that
context were even claimable.

Adding fake dependency edges from `convener_draft` to each cross-examiner is not
a sound fix. It distorts the workflow graph and conflicts with RFC 0045 phase
rules that require cross-phase edges to originate from the source phase's
synthesis job.

## Goals

1. Make interrogation-window consumers explicit in workflow JSON instead of
   relying only on direct `job_dependencies`.
2. Preserve the existing direct-dependent behavior for already-valid
   interrogating-panel workflows.
3. Let phased collaboration shapes declare consumers that are downstream of a
   phase-synthesis gate, especially ACE cross-examiners consuming
   `convener_draft`.
4. Expose resolved target-session information in work packets so a consumer lane
   can open interrogation without guessing session ids.
5. Keep revision reopen behavior from RFC 0095: reopening an interrogable target
   retires the prior target session and requires a fresh attempt/session.
6. Keep interrogation turns as curated message-bus records; do not introduce
   transcript capture or raw provider output.

## Non-Goals

- No new hosted service, external persistence, telemetry, or transcript export.
- No new daemon RPC family for interrogation. Existing
  `interrogation.open/ask/answer/close` remains the mutation surface.
- No semantic enforcement that an LLM asked a "good" question. This RFC only
  fixes target liveness, packet addressing, and lifecycle closure.
- No ACE support-tier graduation. ACE remains `experimental` until this RFC
  lands and a genuine RFC 0105 reliability fixture passes.
- No graph-shape workaround that adds false dependency edges solely to keep
  target sessions alive.

## Proposal

### 1. Workflow field: `interrogation_targets`

A job may declare the interrogable upstream workflow jobs whose live context it
intends to consume:

```json
{
  "id": "cross_examiner_1",
  "type": "build",
  "interrogation_targets": [
    {
      "workflow_job_id": "convener_draft",
      "required": true
    }
  ]
}
```

V1 fields:

- `workflow_job_id` (required): the upstream workflow job id whose latest active
  awaiting-interrogation session is the target.
- `required` (optional, default `false`): packet-facing instruction strength.
  It does not add a hard completion gate in V1. If the target has already
  retired, the consumer should receive the existing non-wedging
  `interrogation_unavailable` signal and proceed on the published artifact.

Validation rules:

- The target workflow job must exist in the same workflow and declare
  `interrogable: true`.
- The consumer job must be reachable downstream from the target in the workflow
  graph. Direct dependency is sufficient but not required.
- `interrogation_targets` is rejected on the target job itself.
- Unknown fields inside an `interrogation_targets[]` entry are lint warnings in
  V1, not hard errors.

### 2. Window ownership

The pending-consumer predicate for an interrogable job becomes:

```text
pending consumers =
  direct dependent jobs
  UNION
  jobs in the same run whose workflow definition declares
    interrogation_targets[].workflow_job_id == target.workflow_job_id
```

A consumer is pending while its job row is not in the existing terminal set:
`completed`, `failed`, `canceled`, `skipped`, or `waiting_human`.

The direct-dependent rule remains for compatibility and for simple panels. The
explicit declaration extends the ownership set for phase-gated shapes where the
real consumers are not direct dependents.

### 3. Release hook

`releaseInterrogationTargetForCompletedReview` should become a generalized
terminal-job hook, for example
`releaseInterrogationTargetsForTerminalConsumer`.

It must run after any job that may be an interrogation consumer reaches a
terminal state, not only after accepting review verdicts. This matters because
ACE cross-examiner jobs are ordinary `build` jobs, not verdict-capable review
jobs.

The hook should be called from the same production mutation paths that terminalize
jobs, including:

- `work.complete`
- `review.verdict` / `submit-review`
- `override-verdict`
- recovery/cancel paths that transition a consumer to a terminal state

If a consumer is terminal but other direct or explicit consumers are still
pending, the target session stays active. Once no consumers remain and no
interrogations are open, the target closes with the existing
`interrogation_window_closed` reason.

### 4. Work-packet projection

When a claimed job declares `interrogation_targets`, `claim-next` adds a block to
the work packet:

```json
{
  "context": {
    "interrogation_targets": [
      {
        "workflow_job_id": "convener_draft",
        "required": true,
        "target_session_id": "sess_...",
        "state": "available",
        "instruction": "Open interrogation against target_session_id before recording findings."
      }
    ]
  }
}
```

If the target previously existed but its panel window has already closed, the
entry is still present with `state: "unavailable"` and the same guidance as
`interrogation.open`'s non-wedging `interrogation_unavailable` result. This makes
the absence legible before the lane burns a state-changing call.

If no target has ever entered awaiting-interrogation for that workflow job, the
entry uses `state: "not_ready"`; in a valid dependency graph this should be
unusual, because the consumer should not be claimable until the target path has
completed.

### 5. ACE generator update

`compileAdjudicatedConstraintExtraction` declares every cross-examiner as a
consumer of `convener_draft`:

```json
{
  "id": "cross_examiner_1",
  "interrogation_targets": [
    {"workflow_job_id": "convener_draft", "required": true}
  ]
}
```

The generated graph remains phase-valid:

```text
convener_draft -> convener_synthesis -> cross_examiner_N
```

No false dependency edges are added.

### 6. Revision reopen

RFC 0095 behavior is preserved. When `adjudicate --needs_revision-->`
`convener_draft` reopens the target, the prior target session and any open
interrogations against it are retired by the existing revision-reopen path. The
new attempt produces a fresh `session.awaiting_interrogation` target for the next
cross-examination pass.

## Acceptance Criteria

1. Workflow validation accepts ACE with cross-examiner
   `interrogation_targets` pointing at `convener_draft`, and rejects a target that
   is missing, self-referential, not reachable, or not `interrogable: true`.
2. A conformance fixture proves the ACE happy path through production handlers:
   `convener_draft` completes into `awaiting_interrogation`,
   `convener_synthesis` accepts, cross-examiners claim work, each can open at
   least one genuine interrogation against the convener target, the fan-out joins
   at `cross_exam_synthesis`, and the run reaches a clearing adjudication.
3. A revision fixture proves `adjudicate needs_revision` reopens
   `convener_draft`, retires the prior target session, re-blocks the cross-exam
   fan-out and join, and allows a fresh target session to be interrogated on the
   next cycle.
4. A fault fixture injects a hard dead lane into a re-opened cross-examiner
   during the revision re-cascade; the production recovery sweep requeues that
   branch on the same attempt while the join remains blocked, then a fresh lane
   completes it and the run finishes or escalates loudly within budget.
5. Existing direct-dependent interrogation-panel tests still pass without
   adding `interrogation_targets`.
6. Work packets for jobs with `interrogation_targets` include resolved
   `target_session_id` and a legible `available` / `unavailable` / `not_ready`
   state.
7. `docs/reference/spec.md` and
   `docs/reference/ubiquitous-language.md` document explicit interrogation
   consumers and the direct-dependent compatibility fallback.

## Open Questions

1. Should `required: true` become a hard completion/verdict gate in a later
   version, requiring either a completed interrogation row or a recorded
   `interrogation_unavailable` signal?
2. Should `interrogation_targets` allow multiple targets for one consumer job in
   V1, or should validation limit V1 consumers to one target until a real shape
   needs more?
3. Should unavailable target signals be recorded as durable events when surfaced
   in work packets, or only when a lane actually calls `interrogation.open`?
4. Should the same explicit-consumer mechanism cover future conversation/floor
   control shapes, or should conversation consumers remain a separate model?

## Domain Modeling

New term:

- **Interrogation consumer** - a workflow job that is allowed or expected to
  interrogate an upstream `interrogable` job's preserved live context. Consumers
  may be direct dependents or explicit `interrogation_targets` declarers.

This is a workflow value object and lifecycle boundary clarification, not a new
aggregate root. The run aggregate remains the authority over jobs, sessions,
leases, interrogations, and events.

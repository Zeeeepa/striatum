# RFC 0112: Explicit interrogation consumers for phased collaboration shapes

Status: accepted (D171)
Date: 2026-06-05
author: proposer-codex-gpt-5-001
Panel resolution: design-panel recommendation
`dec_84e8f185604900a12982e453246fdfd1` recommended the lifecycle-first plan
with follow-up edits.
Owner acceptance: D171 accepted RFC 0112 after those follow-up edits landed.
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
- A consumer may declare one or more targets. Multiple targets are resolved
  independently as per-entry lifecycle state. More than three targets is a lint
  warning in V1, not a hard error.
- `required` (optional, default `false`): packet-facing instruction strength.
  It does not add a hard completion gate in V1. If the target has already
  retired, the consumer should receive the existing non-wedging
  `interrogation_unavailable` signal and proceed on the published artifact.
  V1 records evidence for skipped required targets but does not refuse
  `work.complete`.

Validation rules:

- The target workflow job must exist in the same workflow and declare
  `interrogable: true`.
- The consumer job must be reachable downstream from the target in the workflow
  graph. Direct dependency is sufficient but not required.
- Self-reference is a hard error: a job cannot target itself.
- Duplicate target workflow job ids in one consumer's `interrogation_targets`
  are a hard error.
- V1 rejects `interrogation_targets` on a job that also declares
  `interrogable: true`; chained interrogable consumers are deferred until a real
  shape needs them.
- A target that is not reachable downstream, does not exist, or is not
  `interrogable: true` is a hard error.
- A lane that cannot use the `interrogate` capability for a targeted consumer
  is a lint warning in V1.
- An explicit target that is also a direct dependency is valid but linted as
  redundant.
- Unknown fields inside an `interrogation_targets[]` entry are lint warnings in
  V1, not hard errors.

### 2. Window ownership

The consumer relation is snapshot-derived, never stored. Runtime code resolves
it as a pure function of the run's frozen `workflow_snapshots.workflow_json`
plus live `jobs` rows, behind one helper used by validation-sensitive lifecycle
code and packet projection. This RFC adds no table, migration, RPC family, fake
dependency edge, transcript capture, or external persistence.

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

Revision reopen correctness uses existing RFC 0095 downstream re-blocking plus
the reachable-downstream validation rule. This RFC adds no new reopen algorithm.

### 3. Terminal consumer release hook

`releaseInterrogationTargetForCompletedReview` should become a generalized
terminal-job hook, for example
`releaseInterrogationTargetsForTerminalConsumer`.

All production paths that terminalize a job must flow through a single lifecycle
choke point, named `markJobTerminal` in the current Go mutation package shape.
That choke point invokes `releaseInterrogationTargetsForTerminalConsumer` after
any job that may be an interrogation consumer reaches a terminal state. This
matters because ACE cross-examiner jobs are ordinary `build` jobs, not
verdict-capable review jobs.

The terminal-path inventory covered by the choke point is:

- `work.complete`
- `work.block` transitions to `waiting_human`
- all `review.verdict` / `submit-review` outcomes, including absorbed
  `needs_revision`, `reject`, and `human_checkpoint`
- `override-verdict`
- `checkpoint.resolve` and escalation resolve paths that terminalize or resume a
  terminal consumer
- recovery auto-publish, validated-output completion, and auto-finalize paths
- `recovery.cancel_job`, including cascaded cancellations

An AST/static guard should fail tests when a code path sets a job to a terminal
state outside the choke point. The only structural allowlist is:

- `run.cancel`
- run-failure finalization

Both allowlisted paths must be covered by `closeRemainingSessions`. They are not
consumer-specific terminal transitions. RFC 0104's per-run lock serialization is
the concurrency assumption for this hook.

If a consumer is terminal but other direct or explicit consumers are still
pending, the target session stays active. Once no consumers remain and no
interrogations are open, the target closes with the existing
`interrogation_window_closed` reason.

The end-of-run invariant, not the recovery sweep, is the backstop for the
all-consumers-terminal leak class: a terminal run must not leave any
awaiting-interrogation sessions open.

V1 durable events:

- `interrogation.unavailable_signaled` is appended only when a lane actually
  calls `interrogation.open` and receives a non-wedging unavailable result.
- `interrogation.required_skipped` is appended when a consumer terminalizes
  having skipped a `required: true` target without either an interrogation row or
  an `interrogation.unavailable_signaled` event for that target and attempt.

Packet projection writes no durable rows.

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
        "target_attempt": 1,
        "state": "available",
        "reason": null,
        "instruction": "Open interrogation against target_session_id before recording findings."
      }
    ]
  }
}
```

Packet fields:

- `workflow_job_id`
- `required`
- `state`
- `target_session_id`
- `target_attempt`
- `reason`
- daemon-authored `instruction`, mirroring the `interrogation.open` signal text

State resolution is attempt-aware:

- `available`: the target's current attempt has a live
  awaiting-interrogation session. `target_session_id` and `target_attempt` are
  present; `reason` is null.
- `unavailable`: an awaiting session for the relevant attempt existed but is
  now retired. The retired `target_session_id` is kept for evidence linkage and
  `reason` is the session close reason verbatim. A target job in terminal
  `skipped` state also projects `unavailable` with `reason: "target_skipped"`.
- `not_ready`: no awaiting-interrogation session exists for the target's current
  attempt. After revision reopen this uses `reason: "revision_reopened"` and
  must not expose a stale prior-attempt session.

If no target has ever entered awaiting-interrogation for that workflow job, the
entry uses `state: "not_ready"`; in a valid dependency graph this should be
unusual, because the consumer should not be claimable until the target path has
completed.

The `session.awaiting_interrogation` event payload gains additive `attempt`
metadata. Legacy events without `attempt` are interpreted as attempt 1.

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

In-flight ACE runs prepared before this field exists keep their frozen workflow
snapshots. They are not silently rewritten; an operator must re-prepare the run
after implementation to receive explicit consumer declarations.

### 7. Later V2 hard gate

V1 keeps `required: true` advisory to avoid recreating the revision-wedge family
from #84/#65. A later V2 can promote it to a hard completion gate by flipping a
pure predicate over V1 evidence:

```text
refuse work.complete when
  required target window is still live for this consumer attempt
  AND consumer has no interrogation row for that target and attempt
  AND consumer has no interrogation.unavailable_signaled event for that target and attempt
```

No V1 enforcement code should implement this gate.

## Acceptance Criteria

1. Workflow validation accepts ACE with cross-examiner
   `interrogation_targets` pointing at `convener_draft`, and rejects a target that
   is missing, self-referential, duplicated, not reachable, or not
   `interrogable: true`. It also rejects V1 chained interrogable consumers and
   emits lint warnings for unknown entry fields, redundant direct dependencies,
   consumer lanes without `interrogate`, and target counts above three.
2. A conformance fixture proves the ACE happy path through production handlers:
   `convener_draft` completes into `awaiting_interrogation`,
   `convener_synthesis` accepts, cross-examiners claim work, each can open at
   least one genuine interrogation against the convener target, the fan-out joins
   at `cross_exam_synthesis`, and the run reaches a clearing adjudication. The
   fixture seeds a convener-only fact that is never written to artifacts and
   fails if the cross-examiner answer is derivable from artifacts alone.
3. A revision fixture proves `adjudicate needs_revision` reopens
   `convener_draft`, retires the prior target session, re-blocks the cross-exam
   fan-out and join, and allows only the fresh target session to be interrogated
   on the next cycle.
4. A fault fixture injects a hard dead lane into a re-opened cross-examiner
   during the revision re-cascade; the production recovery sweep requeues that
   branch on the same attempt while the join remains blocked, then a fresh lane
   completes it and the run finishes or escalates loudly within budget.
5. A fixture cell covers a consumer that enters `waiting_human`; the target
   stays alive while the checkpoint is open, then closes or remains open
   according to the same pending-consumer predicate after resolution.
6. Fixture and mutation tests assert `interrogation.unavailable_signaled` on an
   actual `interrogation.open` refusal, and
   `interrogation.required_skipped` when a consumer terminalizes having skipped a
   required target.
7. The terminal-release AST guard covers every terminal path in section 3, with
   only the two structural allowlist entries named there.
8. Completed runs have no lingering awaiting-interrogation sessions after all
   consumers terminalize.
9. Existing direct-dependent interrogation-panel tests still pass without
   adding `interrogation_targets`.
10. Work packets for jobs with `interrogation_targets` include resolved
   `target_session_id`, `target_attempt`, `reason`, and a legible
   `available` / `unavailable` / `not_ready` state, including
   `reason: "target_skipped"` for skipped targets.
11. `docs/reference/spec.md` and
   `docs/reference/ubiquitous-language.md` document explicit interrogation
   consumers and the direct-dependent compatibility fallback.

## Resolved Design Decisions

1. `required: true` is advisory in V1. A later V2 hard gate is pre-declared in
   section 7 and must be implemented as a predicate over V1 evidence, not as new
   transcript or packet-projection state.
2. `interrogation_targets` allows multiple targets in V1. Entries are
   independent; more than three targets is a lint warning.
3. Packet projection writes no durable state. Durable unavailable evidence is
   recorded only when a lane actually calls `interrogation.open` and receives
   `interrogation_unavailable`; required-skip evidence is recorded when the
   terminal choke point observes a skipped required target.
4. Future conversation/floor-control consumers remain deferred. This RFC covers
   interrogation-window consumers only.

## Domain Modeling

New term:

- **Interrogation consumer** - a workflow job that is allowed or expected to
  interrogate an upstream `interrogable` job's preserved live context. Consumers
  may be direct dependents or explicit `interrogation_targets` declarers.

This is a workflow value object and lifecycle boundary clarification, not a new
aggregate root. The run aggregate remains the authority over jobs, sessions,
leases, interrogations, and events.

Implementation must update `docs/reference/spec.md` for the workflow field and
`striatum.work-packet.v1` packet contract, and
`docs/reference/ubiquitous-language.md` for the "interrogation consumer" term.

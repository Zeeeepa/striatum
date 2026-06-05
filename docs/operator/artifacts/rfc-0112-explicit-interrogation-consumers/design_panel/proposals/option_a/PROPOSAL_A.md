# Proposal A - Minimal Explicit Interrogation Consumers

author: proposer-a-codex-gpt-5.5-xhigh-001
date: 2026-06-05
run: run_c57e270528b569e2c53c2befec8c3b82
workflow_job: propose_option_a

## Summary

Option A is the smallest implementation that fixes ACE without changing the
workflow graph. It adds one optional workflow job field,
`interrogation_targets`, resolves that field from the frozen workflow snapshot at
runtime, projects target availability into the consumer work packet, and widens
the existing panel-window pending predicate and release hook from "direct review
dependents" to "direct dependents plus explicit consumers."

It does not add fake dependency edges, a new daemon RPC family, a new aggregate,
or a durable `interrogation_consumers` table. The workflow snapshot remains the
source of the declared consumer relation; daemon-owned PostgreSQL remains the
live-state authority for jobs, sessions, leases, events, and interrogations.

The implementation work is outside this panel's frozen write scope. It would
touch `go/pkg/workflowauthoring`, `go/pkg/workflowgenerate`,
`go/pkg/mutations`, `go/pkg/adapterconformance`, and
`docs/reference/{spec.md,ubiquitous-language.md}`.

## Workflow Field Shape

Add `interrogation_targets` to a consumer job definition:

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

Fields:

- `workflow_job_id` is required and names the upstream target workflow job.
- `required` is optional, defaults to `false`, and must be boolean when present.
- Unknown entry fields are accepted by validation in V1 and surfaced as lint
  warnings.

V1 should keep the field as an array but reject more than one entry per consumer.
ACE needs exactly one target per cross-examiner. Capping V1 at one target avoids
defining multi-target ordering, partial availability, and hard-gate semantics
before a real workflow needs them. The array shape avoids churn when V2 allows
multiple targets.

Validation rules:

- `interrogation_targets`, when present, must be a non-empty array of objects.
- Each entry must include a non-empty string `workflow_job_id`.
- `required`, when present, must be boolean.
- V1 rejects duplicate entries and `len(interrogation_targets) > 1`.
- The target workflow job must exist in the same workflow.
- The target must not be the consumer itself.
- The target job must declare `interrogable: true`.
- The consumer must be reachable downstream of the target through ordinary
  workflow `edges`. Cycle arcs are not used to prove initial downstream
  reachability.
- Lint should warn when the consumer lane lacks `interrogate`, when the explicit
  target is already a direct dependency and is therefore redundant, and when an
  entry contains unknown fields.

The ACE generator update is only:

```json
"interrogation_targets": [
  {
    "workflow_job_id": "convener_draft",
    "required": true
  }
]
```

on each generated `cross_examiner_N` job. The graph stays:

```text
convener_draft -> convener_synthesis -> cross_examiner_N
```

No `convener_draft -> cross_examiner_N` dependency edge is added.

## Required Semantics

`required: true` is packet-facing guidance in V1, not a hard completion gate.

When a target is `available`, a required consumer packet instructs the lane to
open at least one interrogation against `target_session_id` before publishing its
artifact. If the target is unavailable, the lane gets a legible packet signal and
the existing non-wedging `interrogation_unavailable` behavior remains the
runtime fallback.

This must not become a hard gate in V1 because a hard gate needs a durable proof
model that does not exist yet: either an interrogation row for the consumer, a
durable unavailable/waived record, or an operator override. Adding that now would
mix the liveness fix with new adjudication semantics and could wedge replacement
lanes that correctly receive `panel_window_closed`.

Deferred hard-gate version:

- Require each `required: true` target to have either a closed interrogation row
  opened by the consumer's current attempt, a durable target-unavailable record,
  or an explicit waiver/override.
- Define the unavailable record as curated daemon provenance, not provider
  transcript capture.
- Enforce at `work.complete` or verdict submission time.

## Runtime Consumer Predicate

Keep the current direct-dependent fallback and add explicit consumers from the
workflow snapshot:

```text
pending consumers for target job =
  direct dependent jobs in job_dependencies
  union
  jobs in the same run whose workflow job definition declares
    interrogation_targets[0].workflow_job_id == target.workflow_job_id
```

A consumer is pending while its current job row state is not in:

```text
completed, failed, canceled, skipped, waiting_human
```

Implementation outline:

- Change `interrogationConsumersPending(ctx, runner, repositoryID,
  interrogableJobID)` to load the target job row, its `run_id`, and its
  `workflow_job_id`.
- Resolve explicit consumer workflow ids from the run's frozen
  `workflow_snapshots.workflow_json`.
- Query `jobs` for the same run and those workflow ids with non-terminal states.
- Keep the current direct `job_dependencies` query and OR the two results.
- Leave workflows with no `interrogation_targets` behaviorally unchanged.

This keeps `convener_draft` alive after `convener_synthesis` accepts, because
the cross-examiner jobs are still blocked, queued, claimed, or running and are
explicit consumers even though they are not direct dependents of the target.

## Generalized Release Hook

Rename and widen the current review-only closer:

```go
releaseInterrogationTargetsForTerminalConsumer(
    ctx, runner, repositoryID, runID, consumerJobID,
)
```

The helper should:

1. Gather direct upstream runtime job ids from `job_dependencies` for the
   completed/canceled/failed/waiting consumer job.
2. Gather explicit target workflow ids from the consumer job definition in the
   workflow snapshot.
3. Resolve those workflow ids to runtime job ids in the same run.
4. Deduplicate target job ids.
5. For each target, resolve its current target session and call
   `maybeCloseInterrogationTarget`.

The helper is safe to call after any job reaches a terminal consumer state. It
no-ops when the upstream job is not interrogable, when no target session exists,
when another consumer is still pending, when an interrogation is still open, or
when the target still holds an active lease.

Terminal mutation paths that must call the generalized hook:

- `HandleCompleteWork` after an ordinary build job reaches `completed`, before
  `maybeEnqueueDownstream`.
- `HandleRecordVerdict` / `review.submit` after `accept` or
  `accept_with_findings` completes a verdict-capable job.
- `HandleRecordVerdict` when `needs_revision` is absorbed by a downstream
  adjudicator and the review job is completed.
- `HandleRecordVerdict` when terminal `reject` fails a review job.
- `openHumanCheckpoint` after a review job enters `waiting_human`.
- `HandleOverrideVerdict` after an operator override completes a parked review.
- `checkpoint.resolve` when `override` completes a waiting review.
- `checkpoint.resolve` when `cancel` cancels a waiting review.
- `recovery.cancel_job`, through `cancelSingleJob`, for both the requested job
  and any cascade-canceled downstream jobs.
- Recovery auto-publish / auto-finalize helpers that mark jobs `completed`.
- `run.cancel`, either through a batch call over jobs it moved to `canceled` or
  by an explicit test-backed exemption because `run.cancel` closes all remaining
  sessions immediately.

The guard against missed paths should be behavioral, not just a code comment:
add a table-driven mutation test with a tiny explicit-consumer fixture and drive
each terminalizer above. The assertion is that the target session stays active
before the final consumer terminalizes and closes only after the final consumer
terminalizes, unless the whole run is canceled and all sessions close.

## Work-Packet Projection

Add the projection under the existing packet `context` object:

```json
{
  "context": {
    "interrogation_targets": [
      {
        "workflow_job_id": "convener_draft",
        "required": true,
        "state": "available",
        "target_session_id": "sess_...",
        "target_attempt": 1,
        "instruction": "open_interrogation_before_artifact"
      }
    ]
  }
}
```

Fields:

- `workflow_job_id`: target workflow job id from the workflow definition.
- `required`: resolved boolean, defaulted to `false`.
- `state`: one of `available`, `unavailable`, or `not_ready`.
- `target_session_id`: present for `available` and `unavailable`; absent for
  `not_ready`.
- `target_attempt`: current runtime attempt of the target job.
- `instruction`: stable enum-like guidance for the lane.
- `reason`: optional machine-readable detail such as
  `panel_window_closed`, `target_not_awaiting_interrogation`, or
  `target_session_active`.

State definitions:

- `available`: the target job's current attempt has entered
  `session.awaiting_interrogation`, and the resolved target session is active
  and accepted by the existing live-target rules. The lane should call
  `interrogation.open` with `target_session_id`.
- `unavailable`: the target job's current attempt did enter
  `session.awaiting_interrogation`, but that target session is now closed or no
  longer live. The lane should proceed on the published artifact and record the
  absence in its artifact. A later `interrogation.open` call should still return
  the existing non-wedging `interrogation_unavailable` signal.
- `not_ready`: no current-attempt target session has entered
  `session.awaiting_interrogation`. In a valid ACE run this should not appear on
  a claimable cross-examiner packet. If it does, the lane should not guess a
  stale session id; it should report/block if the artifact is insufficient.

For revision safety, projection must be attempt-aware. Add the target job
attempt to future `session.awaiting_interrogation` event payloads:

```json
{
  "session_id": "sess_...",
  "workflow_job_id": "convener_draft",
  "attempt": 2
}
```

Then resolve packet targets against the target job's current `attempt`. For
backward compatibility, events without `attempt` can be considered attempt 1
only. This prevents a reopened consumer packet from seeing a prior attempt's
closed target session while the fresh target attempt is still rerunning.

## Revision Reopen Behavior

Keep RFC 0095 behavior unchanged.

When `adjudicate --needs_revision--> convener_draft` fires:

- `reopenJobForAttempt` continues to call
  `closeInterrogationTargetForReopen` before bumping the target job attempt.
- The prior target session closes with `revision_reopened`.
- Any open interrogations against that stale target are closed.
- The dependency-based downstream reset still reaches ACE cross-examiners
  transitively through `convener_synthesis`, so no fake edges are needed for the
  reset.
- The fresh `convener_draft` attempt later emits a fresh
  `session.awaiting_interrogation` event with the new attempt.
- Reclaimed cross-examiner packets resolve to the fresh target session id, not
  the retired one.

If a stale cross-examiner packet somehow survives across the revision boundary,
its `target_session_id` points at a session closed with `revision_reopened`; the
existing live-target check refuses it. The fresh requeued consumer packet is the
authoritative path.

## RFC 0105 Reliability Fixture

Add a small ACE fixture under `go/pkg/adapterconformance`, for example
`ace_interrogation_consumers_test.go`, driven through production mutation
handlers and the fake agent. Use an ACE generator spec with two
`options.review_postures` entries so the fixture proves both fan-out and
"do not close after the first consumer" behavior without paying for the full
default posture pack.

Required cells:

1. Happy path:
   - Generate ACE with explicit `interrogation_targets` and assert
     `ValidatePhaseShapes` still passes.
   - Assert there is no `convener_draft -> cross_examiner_N` edge.
   - Complete `convener_draft` and assert the convener session enters
     `awaiting_interrogation`.
   - Accept `convener_synthesis`.
   - Claim each cross-examiner and assert packet
     `context.interrogation_targets[0].state == "available"` with the same
     `target_session_id`.
   - For each cross-examiner, open an interrogation, ask at least one question,
     deliver an answer from the target lane's await loop, and close the
     interrogation.
   - Complete the first cross-examiner and assert the target session remains
     active.
   - Complete the final cross-examiner and assert the target closes only after
     no consumers and no open interrogations remain.
   - Continue through `cross_exam_synthesis` and a clearing adjudication.

2. Revision reopen:
   - Drive the first cycle to an adjudication `needs_revision`.
   - Assert the prior convener target session closes with `revision_reopened`.
   - Re-run `convener_draft` and assert a different fresh target session enters
     `awaiting_interrogation`.
   - Reclaim cross-examiners and assert packet targets point to the fresh
     session id.
   - Open at least one fresh interrogation in the second cycle.

3. Dead-lane recovery during the re-cascade:
   - During the reopened cross-examiner fan-out, leave one cross-examiner in a
     hard dead-lane running state.
   - Run the production recovery sweep.
   - Assert the branch is requeued on the same attempt, the join remains
     blocked, and the fresh claimant receives an `available` target projection
     for the fresh convener session.
   - Complete the branch and assert the run finishes or escalates loudly within
     the fixture budget.

This is enough to prove ACE gets genuine preserved-context cross-examination
after the implementation. ACE support-tier graduation remains a separate RFC
0106 decision and should not be flipped in this slice.

## Migration And Backward Compatibility

- Workflows without `interrogation_targets` continue to use direct dependents
  only. Existing interrogating-panel tests should pass unchanged.
- No owner-table migration is required for the V1 relation. The declaration
  lives in workflow JSON; target-session liveness continues to live in sessions,
  events, leases, jobs, and interrogations.
- New generated ACE workflows get explicit targets. Existing frozen workflow
  snapshots do not change automatically; in-flight ACE runs generated before
  this field exists should be treated as old runs and restarted or operator-
  recovered rather than silently rewritten.
- Existing `session.awaiting_interrogation` events without attempt metadata are
  compatible as attempt 1 only.
- CLI and MCP interrogation APIs remain unchanged.
- No transcript capture or exported durable conversation log is introduced.

## Risks

- The largest correctness risk is a missed terminalizer. The behavioral
  terminalizer coverage test is required before accepting the implementation.
- Attempt-aware packet resolution is easy to omit but important. Without it,
  a reopened consumer could see a prior target session and degrade to a false
  `unavailable` instead of waiting for or using the fresh target.
- V1's advisory `required` semantics let a poorly behaved lane skip
  interrogation. The RFC 0105 fixture should prove the intended path, while a
  later hard-gate RFC can enforce proof or waiver records.
- Capping V1 at one target may require a small validation change later when a
  real workflow needs several live targets. That is preferable to inventing
  partial-order semantics now.
- Runtime snapshot parsing in the close path is slightly more work than a
  persisted relation table, but workflow sizes are small and the simpler
  authority model is worth it for V1.

## Rejected Alternatives

- Fake `convener_draft -> cross_examiner_N` dependency edges: rejected because
  they violate phase rules and corrupt scheduling, revision reset, recovery, and
  dashboard semantics.
- A new `interrogation_consumers` table: rejected for V1 because the workflow
  snapshot already contains the declaration and no live mutable relation is
  needed.
- A new interrogation RPC family: rejected because RFC 0082 already defines the
  mutation surface and RFC 0112 only needs liveness, addressing, and closure.
- Hard-gating `required: true` in V1: rejected until durable unavailable/waiver
  records are designed.
- Multiple targets per consumer in V1: rejected as unnecessary for ACE and a
  source of avoidable packet and completion semantics.

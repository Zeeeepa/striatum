# Research: review-job validator + packet shape (RFC 0018)

author: researcher-codex-gpt-5.5-001
date: 2026-05-09

## Goal

Map the existing review-job machinery so the RFC 0018 V1 patch is
surgical, and surface a lifecycle ambiguity in the RFC's "step 2"
language that the synthesis must resolve.

## Existing surfaces

### Workflow validator (`src/striatum/workflow.py`)

- `_validate_review_job_fields` (lines ~1077–1128): rejects
  `reviewer_access_scope` / `reviewer_context_policy` on
  non-review jobs (raises `WorkflowError`, exit code 8); validates
  closed-set membership for each field; enforces the `fresh` ↔
  `fresh_session_required: true` implication.
- `REVIEWER_ACCESS_SCOPE_VALUES` and `REVIEWER_CONTEXT_POLICY_VALUES`
  are module-level frozensets — the precedent the new
  `ALLOWED_POSTURES` constant should match.
- The validator is called once per job from `_validate_jobs`;
  step 1's `_validate_review_posture` slots in beside
  `_validate_review_job_fields`.

### Work-packet builder (`src/striatum/db.py`)

- `_build_review_policy` (lines ~1002–1040): assembles the
  `review_policy` block when a review job declares the RFC 0002
  fields. Returns `None` when neither field is declared, in which
  case `build_packet` skips the block entirely.
- `_REVIEWER_ACCESS_INSTRUCTIONS` and `_REVIEWER_CONTEXT_INSTRUCTIONS`
  are concatenated into `instruction`. Step 1 extends this:
  - Add `posture` to the returned dict when the job declares
    `review_posture`.
  - Concatenate `POSTURE_INSTRUCTIONS[posture]` onto
    `instruction` when posture is non-neutral and first-class.
- `build_packet` line ~881 calls `_build_review_policy`; the
  resulting block is attached at line ~937. No further
  changes needed at the packet-construction level.

### Verdict + downstream-enqueue path

- `record_review_verdict` (line ~1222): inserts a row into the
  `verdicts` table. **No `posture` column today** — RFC 0018 step 3
  adds it; deferred per the RFC's own implementation path.
- `maybe_enqueue_downstream` + `dependencies_satisfied` (lines 513–550):
  the existing edge-verdict gate. A workflow edge can declare
  `requires_verdict: ["accept", "accept_with_findings"]`; the
  next downstream job stays `blocked` until the upstream review's
  latest verdict matches.
- `complete_job` (line ~1141): does **not** today gate on
  downstream reviews. The job moves `running → completed` after
  `verify_required_artifacts` passes, then
  `maybe_enqueue_downstream` fires. Downstream review jobs are
  *enqueued* by build completion; they can't *gate* it.

## Lifecycle ambiguity in RFC 0018 step 2

RFC 0018's "step 2" text says:

> Runtime acceptance rule (in the `complete` mutation path):
> Walk the workflow's edges from this build job. Collect every
> downstream `type: "review"` job's `review_posture`. […] If any
> required posture is missing an accepting verdict, refuse the
> `complete` call with `InvalidTransitionError` and exit code 4.

This **does not work** against striatum's actual lifecycle. A
build's downstream review job is `pending`/`blocked` when the
build's `complete` mutation is called — by definition. The verdict
does not exist yet. Gating build completion on downstream review
verdicts deadlocks.

The RFC author appears to have conflated two different gates:

1. **A workflow-validation-time gate** that refuses to start a
   run whose `required_review_postures` cannot be satisfied by
   the declared review jobs (the "doctor check" the RFC also
   describes). This *is* implementable against the static graph.
2. **A runtime gate** that delays *something* until reviews
   accept. The natural place is the run-completion path
   (`maybe_complete_run`), not the build-completion path.

The synthesis must pick one. Options:

- **A. Workflow-validation-only gate.** V1 enforces the contract
  at `workflow validate` / `run prepare` time: a build with
  `required_review_postures: [P, ...]` requires the workflow to
  declare at least one review job with `review_posture: P`
  reachable from the build via the edge graph. Refusal raises
  `WorkflowError` (exit code 8). No runtime gate beyond today's
  edge-verdict mechanism. Operator value: catches mis-wired
  workflows at authoring time.
- **B. Workflow-validation gate + run-completion runtime gate.**
  As (A), plus `maybe_complete_run` refuses to mark a run
  `completed` until every build's `required_review_postures` is
  satisfied with accepting verdicts on the build's current
  attempt. The run stays `running` (no error; it just waits).
- **C. As written in the RFC (build-completion gate).** Doesn't
  work against the lifecycle; deadlocks. Reject.

Recommendation: **A** for V1. The doctor check + workflow
validation catches every mis-wired workflow at authoring time.
The runtime acceptance is already enforced by the existing
edge-verdict gate (a downstream-of-review job stays blocked
until the review accepts), and the run cannot terminate until
all jobs reach a terminal state — so a build whose required
review never accepts will leave the run open without a runtime
gate addition.

V1.5 can revisit (B) if dogfood evidence shows operators want
explicit run-level "blocked on missing posture" surfacing.

## Test-file precedent

Existing review-related tests:

- `tests/test_workflow_validator.py` — validator rejection
  cases. New posture validator cases extend this file or live
  in a new `test_review_postures.py`.
- `tests/test_work_packet.py` — packet construction. New
  `review_policy.posture` exposure cases extend this file.
- `tests/test_recovery.py` and `tests/test_cli_mvp.py` exercise
  the existing edge-verdict gate; serve as integration
  precedents for `required_review_postures` end-to-end.

V1 adds a single new file `tests/test_review_postures.py`
covering: posture validator rejections, packet exposure,
`required_review_postures` validator rejections, the
workflow-validation gate (recommendation A), zero-regression for
posture-omitting workflows, custom: postures, and an end-to-end
fixture that exercises a full posture-aware run.

## Summary table

| RFC step | Touchpoint | File:line | V1 action |
| --- | --- | --- | --- |
| 1 | Posture validator | `workflow.py:~1102` | New `_validate_review_posture`; `ALLOWED_POSTURES` + `POSTURE_INSTRUCTIONS` constants |
| 1 | Packet exposure | `db.py:~1029` | Extend `_build_review_policy` to carry `posture` + augment `instruction` |
| 2 | `required_review_postures` validator | `workflow.py:~_validate_build_job_fields` (new) | Reject on non-build jobs; validate closed-set membership; reject empty list |
| 2 | Workflow-validation gate (recommendation A) | `workflow.py:_validate_jobs` (new helper) | Walk edge graph; verify each `required_review_postures` entry has a reachable review with that posture |
| 3 (DEFERRED) | `verdicts.posture` column | migration v10 | Out of scope for V1 |
| 3 (DEFERRED) | Status / dashboard / web UI surfacing | various | Out of scope for V1 |

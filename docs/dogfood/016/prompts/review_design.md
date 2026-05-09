# Design review prompt

## Goal

Adversarial review of `docs/dogfood/016/DESIGN_SYNTHESIS.md`. You
are the design reviewer for RFC 0018 V1 (steps 1+2). Verdict:
`accept`, `accept_with_findings`, `needs_revision`, or `reject`.

## Sweep

1. Is the V1 posture vocabulary right-sized? Are any first-class
   postures missing that operators would obviously want?
2. Is the `custom:<name>` grammar unambiguous? What happens with
   `custom:foo:bar`, `custom: ` (space), `custom:` (bare)?
3. Does the acceptance gate compose correctly with today's
   "downstream reviews must accept" rule? Specifically: a build
   with no `required_review_postures` declared should behave
   *byte-identically* to today.
4. Does the gate handle `attempt` correctly? When a build
   re-runs (cycle), each new attempt independently needs each
   required posture to clear.
5. Is the test plan complete against the V1 acceptance criteria?
   Note any gaps.
6. Are there workflow-validator interactions missed? E.g., does
   declaring `required_review_postures: ["security"]` on a
   build whose downstream reviews collectively cannot satisfy
   that posture get caught at workflow validation, or only at
   runtime?
7. Step-3-deferred safety: does V1's data model (no
   `verdicts.posture` column yet) leak through the
   acceptance gate? The gate should derive posture from the
   review *job*, not the verdict row.

## Deliverable

`docs/dogfood/016/review/design/DESIGN_REVIEW.md` — title block
with `author: <line>`, `date:`, `verdict:`, then a sweep matrix
covering the seven points above plus any extras you find.

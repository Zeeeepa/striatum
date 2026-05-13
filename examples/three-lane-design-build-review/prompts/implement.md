# Implementation

Task: {{TASK}}

Build the smallest-scope item from
`docs/three-lane-design-build-review/DESIGN_SYNTHESIS.md`, incorporating
any required revisions from the design review.

Scope:

- Land code under `src/` and tests under `tests/` (or operator-customized
  paths) within the job's allowed write scope.
- Keep the change reviewable. Resist scope creep; defer follow-on items
  to a future run.
- Update or add tests for behavior you change.

Hand off:

- Write `docs/three-lane-design-build-review/build/HANDOFF.md` summarizing
  what landed, what was deferred, and the exact verification commands the
  reviewers should run.

When the handoff is complete, emit the `submit-handoff` packet from the
runner's work packet.

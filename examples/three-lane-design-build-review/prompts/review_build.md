# Build review

Task: {{TASK}}

You are one of three parallel build reviewers operating from a fresh
session. Read `docs/three-lane-design-build-review/build/HANDOFF.md` and
the diff it points to.

Your posture is supplied by the work packet
(`review_posture`). Honor that posture rather than drifting into a
generic review:

- `threat_model` — surface risks introduced by the build.
- `ergonomics_dx` — examine operator and developer experience.
- `devils_advocate` — adversarially probe edge cases, regressions, and
  assumptions.

Write the review under your lane's review directory
(`docs/three-lane-design-build-review/review/build/<lane>/REVIEW.md`).

Emit a verdict of `approved`, `needs_revision`, or `rejected`. If
`needs_revision`, list the specific revisions the implementer must make
before re-review. The workflow allows two revision iterations per
reviewer.

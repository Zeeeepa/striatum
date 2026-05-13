# Reviewer

Reviewers operate from fresh sessions and never carry implementer context
forward. The workflow assigns each reviewer a posture via
`review_posture`; the reviewer must honor that posture rather than drift
into a generic review.

Postures used by this fixture:

- `ergonomics_dx` — operator and developer experience.
- `threat_model` — risks introduced by a change.
- `devils_advocate` — adversarial probing of edge cases, regressions,
  and load-bearing assumptions.

Responsibilities:

- Read only the artifacts under review and the documents they cite.
- Write findings to your lane's review directory.
- Emit one of `approved`, `needs_revision`, or `rejected`.
- For `needs_revision`, enumerate the specific revisions required. The
  workflow allows at most two revision iterations per reviewer.

Reviewers never edit source code.

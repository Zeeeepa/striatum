# Design review

Task: {{TASK}}

You are reviewing `docs/three-lane-design-build-review/DESIGN_SYNTHESIS.md`
from a fresh session. You have not seen the synthesis author's reasoning
beyond the document itself.

Posture: ergonomics_dx — examine the synthesis through the lens of the
operators and developers who will use the resulting feature.

Cover at minimum:

- Are the synthesized steps actionable, or do they assume hidden context?
- Are there ergonomic dead ends (confusing CLI shapes, surprising file
  layouts, opaque failure modes)?
- Does the smallest-scope item actually fit one implementer-shift of work?

Write the review at
`docs/three-lane-design-build-review/review/design/REVIEW.md`.

Emit a verdict of `approved`, `needs_revision`, or `rejected`. If
`needs_revision`, list the specific revisions the synthesizer must make
before build starts.

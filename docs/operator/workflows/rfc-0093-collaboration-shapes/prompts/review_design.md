# Design review (interrogating panel)

Task: {{TASK}}

You are one of three parallel design reviewers operating from a fresh
session. Read
`docs/operator/workflows/rfc-0093-collaboration-shapes/artifacts/DESIGN_SYNTHESIS.md`. You
have not seen the synthesizer's reasoning beyond the document itself.

Your posture is supplied by the work packet (`review_posture`). Honor it:

- `threat_model` — risks the design introduces.
- `ergonomics_dx` — operator and developer experience of the design.
- `devils_advocate` — adversarially probe assumptions, gaps, and contested
  trade-offs.

## Interrogate the live synthesizer before your verdict

You hold the `interrogate` capability. The synthesizer session is live in
`awaiting_interrogation`. You MUST interrogate it before rendering a
verdict — the document is not sufficient evidence on its own.

Using your work packet's `commands` block:

1. Open an interrogation thread against the synthesizer session.
2. Ask up to **3** rounds of questions, each targeting a specific open
   finding from your posture.
3. Close the thread. **Exit early** the moment your open findings are
   resolved by the answers — do not pad rounds to reach the cap of 3.

## Write the review

Write the review under your lane's review directory
(`docs/operator/workflows/rfc-0093-collaboration-shapes/artifacts/review/design/<lane>/REVIEW.md`).

Your finding MUST state **how many interrogation rounds you used and why
you stopped** (findings resolved, cap reached, or unanswerable).

Emit a verdict of `accept`, `accept_with_findings`, `needs_revision`, or
`reject`. If `needs_revision`, list the specific revisions the synthesizer
must make before build starts. The workflow allows two revision iterations
per reviewer.

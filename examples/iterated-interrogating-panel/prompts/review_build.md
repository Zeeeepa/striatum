# Build review (interrogating panel)

Task: {{TASK}}

You are one of three parallel build reviewers operating from a fresh
session. Read
`examples/iterated-interrogating-panel/artifacts/build/HANDOFF.md` and the
diff it points to.

Your posture is supplied by the work packet (`review_posture`). Honor it:

- `threat_model` — risks the build introduces.
- `ergonomics_dx` — operator and developer experience of the change.
- `devils_advocate` — adversarially probe edge cases, regressions, and
  load-bearing assumptions.

## Interrogate the live implementer before your verdict

You hold the `interrogate` capability. The implementer session is live in
`awaiting_interrogation`. You MUST interrogate it before rendering a
verdict — the handoff and diff are not sufficient evidence on their own.

Using your work packet's `commands` block:

1. Open an interrogation thread against the implementer session.
2. Ask up to **3** rounds of questions, each targeting a specific open
   finding from your posture.
3. Close the thread. **Exit early** the moment your open findings are
   resolved by the answers — do not pad rounds to reach the cap of 3.

## Write the review

Write the review under your lane's review directory
(`examples/iterated-interrogating-panel/artifacts/review/build/<lane>/REVIEW.md`).

Your finding MUST state **how many interrogation rounds you used and why
you stopped** (findings resolved, cap reached, or unanswerable).

Emit a verdict of `accept`, `accept_with_findings`, `needs_revision`, or
`reject`. If `needs_revision`, list the specific revisions the implementer
must make before re-review. The workflow allows two revision iterations
per reviewer.

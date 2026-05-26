# Design review — F42 turn-driver (interrogating panel)

Read the task in `docs/operator/workflows/f42-conversation-turn-driver/TASK.md`
and the synthesis at
`docs/operator/workflows/f42-conversation-turn-driver/artifacts/DESIGN_SYNTHESIS.md`.
You are one of two parallel design reviewers from a fresh session; you have not
seen the synthesizer's reasoning beyond the document.

Your posture is supplied by the work packet (`review_posture`). Honor it:

- `threat_model` — risks the design introduces; especially whether the
  turn-driver weakens the autonomous-MCP-client / attestation boundary or can
  slide back into a packet-spoon-feeding proxy.
- `ergonomics_dx` — operator and developer experience of the proposed command
  and the documented conversation recipe.

## Interrogate the live synthesizer before your verdict

You hold the `interrogate` capability. The synthesizer session is live in
`awaiting_interrogation`. You MUST interrogate it before rendering a verdict —
the document is not sufficient evidence on its own. Using your work packet's
`commands` block: open a thread, ask up to **3** rounds each targeting a
specific open finding from your posture, then close. **Exit early** the moment
your findings are resolved.

## Write the review

Write the review under your lane's review directory
(`docs/operator/workflows/f42-conversation-turn-driver/artifacts/review/design/<lane>/REVIEW.md`).
Your finding MUST state **how many interrogation rounds you used and why you
stopped**.

Emit a verdict of `accept`, `accept_with_findings`, `needs_revision`, or
`reject`. If `needs_revision`, list the specific revisions the synthesizer must
make before build starts (at most two iterations per reviewer).

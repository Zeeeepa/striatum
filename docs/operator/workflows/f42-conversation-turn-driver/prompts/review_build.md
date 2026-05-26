# Build review — F42 turn-driver (interrogating panel)

Read the task in `docs/operator/workflows/f42-conversation-turn-driver/TASK.md`,
the handoff at
`docs/operator/workflows/f42-conversation-turn-driver/artifacts/build/HANDOFF.md`,
and the diff it points to.

You are one of two parallel build reviewers from a fresh session. Your posture
is supplied by the work packet (`review_posture`). Honor it:

- `threat_model` — risks the build introduces; verify the turn-driver does not
  weaken the autonomous-MCP-client / attestation boundary and cannot be slid back
  into a packet-spoon-feeding proxy. Check that the loop is crash-safe and does
  not double-speak or stall the round-robin.
- `ergonomics_dx` — operator and developer experience of the new command and the
  updated conversation recipe; run the verification commands.

## Interrogate the live implementer before your verdict

You hold the `interrogate` capability. The implementer session is live in
`awaiting_interrogation`. You MUST interrogate it before rendering a verdict —
the handoff and diff are not sufficient evidence on their own. Using your work
packet's `commands` block: open a thread, ask up to **3** rounds each targeting
a specific open finding from your posture, then close. **Exit early** when your
findings are resolved.

## Write the review

Write the review under your lane's review directory
(`docs/operator/workflows/f42-conversation-turn-driver/artifacts/review/build/<lane>/REVIEW.md`).
Your finding MUST state **how many interrogation rounds you used and why you
stopped**.

Emit a verdict of `accept`, `accept_with_findings`, `needs_revision`, or
`reject`. If `needs_revision`, list the specific revisions the implementer must
make before re-review (at most two iterations per reviewer).

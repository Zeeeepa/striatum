# Implementation — F42 turn-driver (interrogable)

Read the task in `docs/operator/workflows/f42-conversation-turn-driver/TASK.md`,
the synthesis at
`docs/operator/workflows/f42-conversation-turn-driver/artifacts/DESIGN_SYNTHESIS.md`,
and the design-panel findings under
`docs/operator/workflows/f42-conversation-turn-driver/artifacts/review/design/`.

Build the smallest-scope item from the synthesis, incorporating required design
revisions.

Scope:

- Implement the generic turn-driver in Go per the synthesis (command surface +
  loop). The loop must key off "this CLI is single-shot", not the literal string
  "gemini".
- Add tests for the loop logic: floor detection, our-turn vs not-our-turn,
  content capture/sanitization, conversation-closed exit, agent-failure/empty
  output. The agent invocation MUST be seam-tested with an injected/fake content
  generator — no dependency on a live gemini binary.
- Keep the change reviewable; defer chat-UI rendering (F43) and non-gemini
  consumers beyond proving genericity.
- Run `make lint typecheck test` (or `go test ./...`) and capture the result.

Hand off:

- Write
  `docs/operator/workflows/f42-conversation-turn-driver/artifacts/build/HANDOFF.md`
  summarizing what landed, what was deferred, how the spoon-feeding boundary is
  kept enforceable, and the exact verification commands the reviewers should run.

This node is **interrogable**. After you publish the handoff, your session stays
live so the build-review panel can interrogate your preserved reasoning. Keep
the decisions behind the diff in working memory.

If a build reviewer returns `needs_revision`, you will re-enter this job (up to
two iterations per reviewer) to land the enumerated revisions.

When the handoff is complete, emit the `submit-handoff` packet from your work
packet.

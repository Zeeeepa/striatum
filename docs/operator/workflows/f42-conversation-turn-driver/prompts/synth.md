# Design synthesis — F42 turn-driver (interrogable)

Read the task in `docs/operator/workflows/f42-conversation-turn-driver/TASK.md`
and both design proposals under
`docs/operator/workflows/f42-conversation-turn-driver/artifacts/design/`.
Produce a single buildable synthesis at
`docs/operator/workflows/f42-conversation-turn-driver/artifacts/DESIGN_SYNTHESIS.md`.

The synthesis should:

- Pick a concrete approach, not a menu of options.
- Cite each input design by lane (codex / claude_code) for ideas carried forward
  and rejected.
- Resolve the spoon-feeding-hazard tension explicitly: state where in code the
  turn-driver/agent-loop boundary lives and how it stays enforceable (e.g. a
  test, a guardrail, a clear seam).
- Name the smallest implementable scope a single implementer can land, and the
  exact files it will touch (command surface, package, tests, docs).

Do not edit files under
`docs/operator/workflows/f42-conversation-turn-driver/artifacts/design/`. Do not
write source code.

This node is **interrogable**. After you publish, your session stays live so the
design-review panel can interrogate your preserved reasoning. Keep the rationale
behind every choice in working memory — answer from your own context, not by
re-deriving from the document.

If the panel returns `needs_revision`, you will re-enter this job (up to two
iterations per reviewer) to address the findings.

When the synthesis is complete, emit the `submit-handoff` packet from your work
packet.

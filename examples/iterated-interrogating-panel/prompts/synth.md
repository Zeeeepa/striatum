# Design synthesis (interrogable)

Task: {{TASK}}

You have read all three design proposals under
`examples/iterated-interrogating-panel/artifacts/design/`. Produce a single
buildable synthesis at
`examples/iterated-interrogating-panel/artifacts/DESIGN_SYNTHESIS.md`.

The synthesis should:

- Pick a concrete approach, not a menu of options.
- Cite each input design by lane (codex / claude_code / gemini) for the
  ideas you carry forward and the ones you reject.
- Call out any unresolved contradictions and explain how the build phase
  should handle them.
- List the smallest implementable scope a single implementer can land next.

Do not edit files under
`examples/iterated-interrogating-panel/artifacts/design/`. Do not write
source code.

This node is **interrogable**. After you publish, your session stays live
so the design-review panel can interrogate your preserved reasoning. Keep
the rationale behind every choice in your working memory — you will be
asked to justify it through interrogation, and you must answer from your
own context rather than re-deriving from the document.

If the panel returns `needs_revision`, you will re-enter this job (up to
two iterations per reviewer) to address the enumerated findings.

When the synthesis is complete, emit the `submit-handoff` packet that the
runner provided.

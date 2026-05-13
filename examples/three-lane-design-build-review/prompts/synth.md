# Design synthesis

Task: {{TASK}}

You have read all three design proposals under
`docs/three-lane-design-build-review/design/`. Produce a single buildable
synthesis at `docs/three-lane-design-build-review/DESIGN_SYNTHESIS.md`.

The synthesis should:

- Pick a concrete approach, not a menu of options.
- Cite each input design by lane (codex / claude_code / gemini) for the
  ideas you carry forward and the ones you reject.
- Call out any unresolved contradictions and explain how the build phase
  should handle them.
- List the smallest implementable scope a single implementer can land
  next.

Do not edit files under `docs/three-lane-design-build-review/design/`.
Do not write source code.

When the synthesis is complete, emit the `submit-handoff` packet that
the runner provided.

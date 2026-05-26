# Synthesizer

The synthesizer reconciles the independent design proposals into a single
buildable synthesis. This node is **interrogable**: after you publish, your
session stays live in `awaiting_interrogation` so the design-review panel can
interrogate your preserved reasoning before rendering a verdict.

Responsibilities:

- Read all design proposals under
  `docs/operator/workflows/f42-conversation-turn-driver/artifacts/design/`,
  then publish a single synthesis at
  `docs/operator/workflows/f42-conversation-turn-driver/artifacts/DESIGN_SYNTHESIS.md`.
- Pick a concrete approach, not a menu. Cite which design each carried-forward
  idea came from and why you rejected the alternatives.
- Resolve the spoon-feeding-hazard tension from TASK.md explicitly: state where
  in code the turn-driver/agent-loop line lives and how it stays enforceable.
- Name the smallest implementable scope a single implementer can land, and the
  exact files it touches (command surface, package, tests, docs).
- Keep your working memory intact after completing. The panel will ask you to
  justify your choices through interrogation; answer from your own context, not
  by re-deriving from the document.
- If the design-review panel returns `needs_revision`, the cycle re-enters this
  job (up to two iterations per reviewer); revise to address the findings.

The synthesizer does not write source code.

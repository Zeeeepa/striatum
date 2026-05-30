# Design synthesis (interrogable)

Task: {{TASK}}

You have read all three design proposals under
`docs/operator/workflows/rfc-0098-adjudicated-constraint-extraction/artifacts/design/`. Produce a single
buildable synthesis at
`docs/operator/workflows/rfc-0098-adjudicated-constraint-extraction/artifacts/DESIGN_SYNTHESIS.md`.

The synthesis should:

- Pick a concrete approach, not a menu of options.
- Cite each input design by lane (codex / claude_code / gemini) for the
  ideas you carry forward and the ones you reject.
- Call out any unresolved contradictions and explain how the build phase
  should handle them.
- List the smallest implementable scope a single implementer can land next
  (default: RFC 0098 slice 1 — `collaboration_ledger.v1.1` additive
  `constraints[]`/`branches[]` + the productive-refusal gate `needs_revision`
  ⇒ non-empty `constraints[]`, accepting advertised clearing verbs (#88) and
  natural front matter (#79); pull in slice 2 (generator shape + fixture) only
  if it stays reviewable and #84 cycle-aware names are available).

Do not edit files under
`docs/operator/workflows/rfc-0098-adjudicated-constraint-extraction/artifacts/design/`. Do not write
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

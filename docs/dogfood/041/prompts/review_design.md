# Review Design Prompt (ergonomics_dx posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (ALL FIVE FIELDS REQUIRED):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0038", "web-ui", "frontend-toolchain"]
---
```

`schema_version` must be the exact string `"striatum.finding.v1"` (not `"1"` — dogfood-040 friction).

Review `docs/dogfood/041/DESIGN_SYNTHESIS.md` under **ergonomics_dx** posture. Verdict acceptance means the affordances are discoverable and consistent from a first-time-user perspective.

In scope:

- **Tree browser navigation grammar** — placeholder text, click-to-expand, breadcrumbs, keyboard nav, empty states.
- **Workflow chooser wizard** — step ordering (shape → lane set → modifiers → fields → preview → confirm-write), preview-before-write specifically, copy quality (`recommended_for` specific, not boilerplate), operator confirmation gate at Step 6.
- **Drag-drop graph editor widget set** — radio buttons for postures, dropdowns for enums, multi-select for tags, file pickers for paths. Are widget choices ergonomically right?
- **Code viewer** — shiki grammar coverage (8 languages), fallback for unknown types, line numbers, copy-to-clipboard, raw-link, dark-mode parity.
- **Promoted Edit affordance** — primary button position next to Run; visual treatment matches existing primary-button style.
- **Accessibility** — keyboard nav completeness, focus management on `<dialog>`, ARIA labels on all new widgets, color contrast.
- **Toolchain honesty** — Vite + React + TypeScript chosen, no scope creep into full SPA.
- **Island deployment shape preserved** — Jinja2 page shells stay; React islands mount into specific slots, not page-replacing.
- **No new runtime deps** — only contributor-side node deps; operator install stays pip-only.
- **Build determinism** — bundle hash check in CI is the gate.
- **Disjoint implementer scope** — codex toolchain side, claude components side, written-out clearly.
- **Staging plan low-risk-first** — toolchain + Edit-button promotion before any island that adds visual complexity.

For `needs_revision`: list minimum concrete changes. For `accept_with_findings`: findings explicitly non-blocking.

**IMPORTANT — write the REVIEW.md artifact directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf. Per dogfood-040: do not surface strategy and exit. Per dogfood-037 intervention #5: do not ask the operator a clarifying question and exit.

Stay inside the review write scope. Do not modify the synthesis.

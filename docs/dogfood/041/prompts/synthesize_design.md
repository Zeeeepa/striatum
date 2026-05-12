# Synthesize Design Prompt

Produce `docs/dogfood/041/DESIGN_SYNTHESIS.md`. Start with:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/041/design/codex/DESIGN.md", "docs/dogfood/041/design/claude_code/DESIGN.md", "docs/dogfood/041/design/gemini/DESIGN.md"]
---
```

Byline as plain markdown line AFTER the block.

Read all three designs and synthesize ONE implementation plan for RFC 0038. Explicitly choose; do not enumerate.

Required sections:

- **Accepted Scope** — map each RFC 0038 §Acceptance Criteria bullet to a concrete code-and-test plan with one named owner per bullet (implement_toolchain_codex or implement_components_claude).
- **Toolchain Selection** — confirm Vite + React + TypeScript + react-flow + shiki. No alternatives at this point.
- **Project Layout** — `src/striatum/web/frontend/{package.json, vite.config.ts, tsconfig.json, .gitignore, src/{islands/, shared/, __tests__/, main.ts}, index.html}` + `src/striatum/web/static/build/`.
- **Makefile + CI** — `ui-install`, `ui-build`, `ui-dev`, `ui-test`; CI node-22-LTS step; bundle-hash check shape.
- **Wheel Distribution** — `pyproject.toml` `package_data` includes `striatum.web.static.build`.
- **Island Mount Pattern** — `<div id="island-<name>" data-props='...'></div>` + `<script type="module" src="/static/build/island-<name>.js" defer></script>`. Per-island `createRoot()` calls into named DOM slots.
- **Five Feature Additions** —
  - 5a Edit affordance promotion (Jinja2 template, non-island, codex scope)
  - 5b Tree browser island (claude scope) + `GET /v1/repo/tree` route (codex scope)
  - 5c Workflow chooser wizard island (claude scope) calling existing RFC 0034 V1 endpoints
  - 5d Drag-drop graph editor island (claude scope) saving to existing workflow-edit endpoint
  - 5e Syntax-highlighted code viewer island (claude scope) over existing `/view/<path>`
- **Disjoint Implementer Write Scopes** — codex owns Python + Jinja2 + Makefile + pyproject + CI + `static/build/` committed output; claude owns `frontend/src/`, `frontend/index.html`, all docs, combined BUILD_HANDOFF.
- **Accessibility Checklist** — keyboard nav completeness, focus management on `<dialog>`, ARIA labels, color contrast, dark-mode parity.
- **Cross-Platform / Browser Matrix** — Linux + macOS only; modern browsers only; node v22+.
- **npm Supply-Chain Posture** — narrow package set, lockfile committed, `npm audit --omit=dev` baseline in CI.
- **Build Determinism** — bundle hash check shape (where hashes live).
- **Test Strategy** — JS unit tests (Vitest), existing UI snapshot tests, new Python tests for `/v1/repo/tree`, manual checklist for unautomatable things.
- **Documentation Deltas** — FRONTEND_DEVELOPMENT.md (new), HOW_TO_HUMAN, UBIQUITOUS_LANGUAGE, CLI_REFERENCE cross-reference, CHANGELOG, RFC 0038 status update.
- **Staging Plan per RFC 0038 §Implementation Plan** — Step 1 toolchain + CI; Step 2 D073/D092 + Edit-button promotion; Step 3 code viewer (lowest-risk island first); Step 4 tree browser; Step 5 chooser wizard; Step 6 graph editor; Step 7 docs.
- **Human-Decision Questions** — any open issues the implementer cannot resolve from the synthesis alone.

If the three designs disagree, pick one path and explain the tradeoff. If a guarantee is advisory, label it advisory.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim AFTER the front matter and a blank line.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.

# Synthesize Design Prompt

Produce `docs/dogfood/039/DESIGN_SYNTHESIS.md`. The file must start with a `striatum.synthesis.v1` front matter block (JSON-encoded values; quote strings; JSON arrays for lists):

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/039/design/codex/DESIGN.md", "docs/dogfood/039/design/claude_code/DESIGN.md", "docs/dogfood/039/design/gemini/DESIGN.md"]
---
```

The byline appears AFTER the front matter block, as a plain Markdown line `author: <slug>` (lowercase, no bold/italics/heading/quotes).

Read all three design artifacts and synthesize ONE implementation plan for RFC 0037 (web UI ergonomic improvements). The synthesis must explicitly choose, not enumerate.

Required sections:

- **Accepted Implementation Scope** — map each RFC 0037 §Acceptance Criteria bullet 1:1 to a concrete code-and-test plan, with one named owner per bullet (which template, which JS file, which CSS class).
- **Deferred Scope** — filter-state-in-querystring (V1.5; localStorage for V1); sticky-positioned next-actions banner (future RFC); keyboard-shortcut configurability (future RFC); per-doctor-problem-kind doc anchors that don't yet exist in HOW_TO_HUMAN (note as defer-to-doc-pass or add as part of this work).
- **JS Architecture** — vanilla JS, no framework, no bundler, no build step. ES6+. Per-file self-contained, loaded with `defer`. DOMContentLoaded guards.
- **localStorage Key Naming Convention** — `striatum.ui.<feature>.<field>`; embed `version` in each value; on read, version mismatch → reset to defaults.
- **Data Island Pattern** — `<script type="application/json" id="...">...</script>` embedded server-side; JS reads + filters client-side. Server still renders the full table initially (progressive enhancement).
- **Filter UX Choices** — exact placeholder text per filter; default state; clear-filter affordance; empty-result state copy.
- **Duration Column Format** — concrete (< 60s: `Xs`; < 1h: `Xm Ys`; ≥ 1h: `Xh Ym`; running: relative with setInterval).
- **Localtime Toggle UX** — header placement; state indicator (`UTC` / `Local`); default UTC; persistence semantics.
- **Keyboard Shortcut Overlay UX** — `<dialog>` element; `?` opens; Esc/outside-click closes; what keys are listed; input-focus guard.
- **Empty-State Copy** — exact strings (run list / workflows / doctor); specific + actionable + linked.
- **Next-Actions Banner Layout** — directly below run-header; full-width; hidden for terminal runs; aria-label.
- **Doctor Grouping Behavior** — default-collapsed when > 5 in group; "hide terminal-run problems" toggle default ON when count > 20; per-kind collapsed-state persistence.
- **Graph Node Tooltips** — hover-only; positioned `fixed`; pointer-events none; shows job name + role + state + duration.
- **app.css Dark-Mode Parity Audit** — full class list with per-class decision (explicit dark block / inherits / N/A).
- **Accessibility Checklist** — keyboard nav completeness, focus management on `<dialog>`, ARIA labels, skip-link.
- **Responsive Behavior** — filter-row wraps, run-grid collapses at < 900px, dialog fits 1024px viewport. Mobile-first is OUT OF SCOPE.
- **Empty-State Illustration Choice** — inline SVG icon (48x48px, `currentColor`). Reuses base.css palette.
- **Test Strategy** — existing UI snapshot tests pass unchanged; new JS unit tests for duration formatter + localStorage helpers + filter predicates + input-focus guard; manual checklist for keyboard/dark-mode/tooltip/dialog/empty-state.
- **Staging Plan** — order of implementation, lowest-risk-first: (1) localtime toggle + base.js scaffold + skip-link + help dialog; (2) run-list filter + duration + empty state; (3) workflows-index filter + last-modified + empty state; (4) doctor grouping + terminal-hide + empty state; (5) graph node tooltips; (6) next-actions banner promotion; (7) app.css dark-mode parity; (8) docs (HOW_TO_HUMAN, CHANGELOG, README cross-reference if applicable).
- **Documentation Deltas** — HOW_TO_HUMAN (filter UX walkthrough + keyboard shortcuts table); CHANGELOG Unreleased entry; RFC 0037 status block.
- **Human-Decision Questions** — any open questions the implementer cannot resolve from the synthesis alone.

If the three designs disagree, pick one path and explain the tradeoff. If a guarantee is advisory, label it advisory.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim AFTER the front matter and a blank line.

- Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.

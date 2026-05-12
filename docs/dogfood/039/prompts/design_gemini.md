# Gemini Design Prompt

Produce `docs/dogfood/039/design/gemini/DESIGN.md`.

Design an implementation plan for RFC 0037 with attention to accessibility, responsive behavior, state persistence layering, dark-mode parity, empty-state illustration choice, and the test strategy.

Your plan must cover:

**Accessibility:**

- Keyboard navigation completeness: every interactive element (filter pills, toggle, dialog open/close, table-row links) reachable via Tab; visible focus indicator on all of them.
- Focus management: opening the keyboard-shortcut help `<dialog>` traps focus inside; closing returns focus to the trigger.
- ARIA labels on filter pills (`role="group" aria-label="Filter by state"`); aria-pressed on toggle states; aria-expanded on `<details>` collapsibles.
- `<dialog>` element for the help overlay (native focus trap, keyboard accessibility).
- Skip-link in `base.html` pointing to `<main>`.

**Responsive behavior at narrow viewports:**

- RFC 0037 explicit non-goal: mobile-first redesign. But widgets shouldn't break on a 1024px-wide laptop.
- Filter row should wrap to 2 lines if needed (flexbox `flex-wrap: wrap`).
- Run-detail run-grid (sidebar + main) collapses to single-column at < 900px.
- The keyboard-shortcut help dialog must fit on a 1024px viewport.

**State persistence layering:**

- localStorage for filter state + localtime preference + doctor collapsed-kinds.
- Default-state-on-first-visit: if a key is missing or unparseable, fall back to defaults (don't crash).
- Migration when keys change: each key has an embedded `version` field; on read, if version mismatches the current schema, reset to defaults.
- localStorage key naming convention: `striatum.ui.<feature>.<field>` — namespaced and explicit.

**Dark-mode parity audit checklist:**

For each app.css class, decide: needs explicit dark block / inherits from base.css palette variables / N/A. Concrete list:

- `.job-list` — needs explicit
- `.job-link` — needs explicit (hover state)
- `.status-pill` — has per-status colors; need dark variants for each
- `.posture-chip` — same as status pill
- `.run-grid` — N/A (layout only)
- `.run-jobs-rail` — needs explicit (background)
- `.run-meta` — inherits OK
- `.run-events` — needs explicit (alternating row backgrounds)
- `.workflow-graph` — needs explicit (SVG colors)
- `.workflow-edit-form` — needs explicit (input/textarea backgrounds)

**Empty-state illustration choice:**

Pick ONE of:

- Simple ASCII (e.g., `┌──┐\n│  │\n└──┘` for empty box) — local-first, no external assets.
- System emoji — works on most platforms, may render inconsistently.
- Inline SVG icon (~20-30 lines) — flexible, no external dependency.

Recommendation: inline SVG icon, sized 48x48px, using `currentColor` for dark-mode compat. Reuse the existing SVG palette from `base.css` palette variables.

**Test strategy:**

- Existing UI snapshot tests must pass unchanged (server-side render only adds filter-row markup + new JS includes; no semantic table content change).
- New JS unit tests for: duration formatter (covers <60s, <1h, ≥1h, running cases); localStorage helpers (get/set/migrate); filter predicates (search substring match across multiple fields, state match, date-range comparison); keyboard shortcut input-focus guard.
- Manual checklist for things that can't be automated:
  - Keyboard shortcuts disabled when input has focus.
  - Dark-mode parity visual check (system pref toggle).
  - Tooltip positioning on graph nodes (no off-screen clipping).
  - Help overlay dismissal via Escape + outside-click.
  - localtime toggle rewrites every `<time>` element on the page.
  - Empty-state copy renders correctly when there are zero items.

**Cross-platform reality:**

- localStorage is universal (all evergreen browsers).
- `<dialog>` element has wide support (Chrome/Edge/Safari/Firefox modern). Add `<dialog>` polyfill stub IF browser detection shows older Safari? Recommendation: no, the RFC 0022 V1 audience is modern browsers; skip the polyfill.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-gemini-pro-001`
- Wrong: `**Author:** designer-gemini-pro-001` (bolded variant)
- Wrong: `Author: designer-gemini-pro-001` (capital A)
- Wrong: `author: "designer-gemini-pro-001"` (quoted)

The `handoff` kind does not require YAML front matter. Just the plain `author: <slug>` byline. (Do not add finding-style front matter — that was a dogfood-037 friction note for gemini's handoff artifact.)

**IMPORTANT — produce the artifact, do not surface strategy and exit.** Per dogfood-036 OPERATOR_REPORT intervention #2: previous gemini sessions surfaced a strategy summary and asked the operator "should I proceed?" and exited without writing the file. Do not repeat that pattern. The work packet's `expected_artifacts` requires the file on disk; you are inside a supervised wrapper that runs `gemini --prompt -` once per packet, with no follow-up turn. Write the DESIGN.md file with byline + body in this single invocation.

Do not call striatum CLI; the operator publishes on your behalf otherwise.

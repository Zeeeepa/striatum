# Claude Code Design Prompt

Produce `docs/dogfood/039/design/claude_code/DESIGN.md`.

Design an implementation plan for RFC 0037 emphasizing the first-time-user experience and the affordance grammar that makes each gap into a discoverable surface.

Focus on:

**Filter row UX:**

- Placeholder text on the search input: specific (e.g. "Search runs by id, branch, workflow id…" not just "Search").
- Default state: all-runs visible; no surprise filtering on first visit (filter state initialized from localStorage but defaults to empty).
- Persistence semantics: changing the filter writes to localStorage; on next visit the same filter is applied. Reloading the page preserves the filter. Navigating away and back preserves it.
- Clear-filter affordance: an X button inside the search input + a "Clear all filters" link when any filter is non-default.
- Empty-result state: when the filter matches nothing, show "No runs match the current filter" + the clear-all link.

**Duration column format:**

- < 60s: `Xs`
- < 1h: `Xm Ys`
- ≥ 1h: `Xh Ym`
- Running runs: relative (`Xm ago started`) — recalculated on a setInterval (every 30s)
- Terminal runs: static (`created → completed`)

**Localtime toggle:**

- Visual placement: in the site header next to the brand or in the top-right corner; small affordance, not a primary action.
- State indicator: the toggle text shows the CURRENT mode (`UTC` or `Local`); clicking switches.
- Effect: every `<time datetime="...">` element on the page has its visible text rewritten by a single DOMContentLoaded + click handler.
- Default: UTC (preserves current behavior; opt-in toggle).

**Keyboard shortcut overlay UX:**

- When to show: when the user presses `?` (only when focus is not on an input/textarea/contenteditable).
- How to dismiss: `Escape` or click outside the dialog or click a dismiss button.
- What keys are listed: the four `g X` navigation shortcuts + `?` + any future ones.
- Visual: HTML `<dialog>` element, opened with `.showModal()`. Native browser keyboard accessibility (Tab cycles focus within).
- Discoverability: a small "?" badge in the bottom-right corner of every page hinting "press ? for shortcuts".

**Empty-state copy quality bar:**

- Specific (names the action the user should take).
- Actionable (includes a copy-paste CLI example).
- Linked (points at `docs/HOW_TO_HUMAN.md` anchor).
- Examples:
  - Run list (no runs): "No runs yet. Run `striatum run prepare --workflow <path>` to create your first one; see `docs/HOW_TO_HUMAN.md#starting-a-run`."
  - Workflows index (no workflows): "No workflow.json files found. Run `striatum workflow generate <path> --shape minimal --lane-set local --artifact-root striatum/<name>` to create one."
  - Doctor (no problems): "0 problems found. Nothing to triage."

**Next-actions banner layout:**

- When shown: non-terminal runs (running, paused, awaiting human checkpoint).
- When hidden: terminal runs (completed, canceled, failed).
- Position: directly below the `<header class="run-header">` block, full-width, distinct background color (not the same as the run-header).
- Accessibility: appropriate `role="region"` + `aria-label="Next actions"`; keyboard-focusable.

**Doctor grouping default-collapsed behavior:**

- Default: any group with > 5 problems is collapsed by default; ≤ 5 expanded.
- Toggle: each group has an expand/collapse caret.
- "Hide terminal-run problems" toggle: state in localStorage; default ON when the unfiltered count would be > 20 problems, default OFF otherwise.
- Collapsed-by-default state persists in localStorage (`striatum.ui.doctor.collapsed_kinds`).

**Justifications (one-liner per decision):**

Each ergonomic decision in the design must have a one-line "why this and not the alternative" justification. Reviewers will hold you to it.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-claude-opus-001`

The `handoff` kind does not require YAML front matter.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.

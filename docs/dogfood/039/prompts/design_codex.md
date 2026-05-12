# Codex Design Prompt

Produce `docs/dogfood/039/design/codex/DESIGN.md`.

Design an implementation plan for RFC 0037: web UI ergonomic improvements. Sit on top of the existing RFC 0013 V1 / RFC 0022 V1 / RFC 0023 V1.5 / RFC 0024 V1+V1.5+V2+V3+V4 web UI; do not redesign any of it.

Cover concrete file-by-file edits:

**Templates (`src/striatum/web/templates/`):**

- `base.html` — add localtime toggle button + keyboard-shortcut help `<dialog>` + skip-link.
- `run_list.html` — add filter row (search + state pills + date-range select) + duration column + empty-state block.
- `workflows_index.html` — add filter row (search + status pills) + last-modified column + empty-state block.
- `doctor.html` — group problems by kind with `<details>` collapsible sections + terminal-hide toggle + empty-state block.
- `run_detail.html` — promote `Next actions` panel from sidebar to banner just below run header (only when there are actions; hidden for terminal runs).

**New JS files (`src/striatum/web/static/`):**

- `base.js` (~80 lines) — localtime toggle handler; keyboard-shortcut framework (g r / g w / g c / g d / ?); skip-link + help dialog wiring; localStorage helpers (get/set/migrate).
- `run_list.js` (~80 lines) — data-island reader + filter handler + duration formatter (mm:ss / hh:mm).
- `workflows_index.js` (~50 lines) — data-island reader + filter handler.
- `doctor.js` (~40 lines) or doctor template inline JS — collapsible groups + terminal-hide toggle.
- `run_detail.js` (extension or new) — graph node hover tooltip.

**CSS (`src/striatum/web/static/`):**

- `app.css` — add `@media (prefers-color-scheme: dark)` blocks for `.job-list`, `.job-link`, `.status-pill`, `.posture-chip`, `.run-grid`, `.run-jobs-rail`, `.run-meta`, `.run-events`, `.workflow-graph`, `.workflow-edit-form`. Audit each component class.

**Data islands:**

For client-side filtering, embed run/workflow data as a `<script type="application/json" id="run-data">...</script>` block in the template. The JS reads + filters this. No new HTTP endpoint. Server still renders the full table initially (works without JS); filter is JS-progressive-enhancement.

**localStorage key naming convention:**

- `striatum.ui.timezone` — `"utc"` | `"local"`
- `striatum.ui.run_list.filter` — `{search: "", state: "all", date: "all"}`
- `striatum.ui.workflows_index.filter` — `{search: "", status: "all"}`
- `striatum.ui.doctor.hide_terminal` — `boolean`
- `striatum.ui.doctor.collapsed_kinds` — `string[]`

Use `JSON.stringify` + `JSON.parse` with try/catch + fallback to defaults.

**Test strategy:**

- Existing UI snapshot tests must continue to pass (server-side render is unchanged except for filter row markup + new JS includes).
- New JS unit tests where reasonable: small isolated functions (duration formatter, localStorage helpers, filter predicates).
- Manual checklist for things that can't be automated: keyboard shortcuts disabled when focus is on input; dark-mode parity visual check; tooltip positioning on graph nodes; help overlay dismissal.

**JS architecture rules:**

- Vanilla JS only. No framework, no bundler, no build step.
- ES6+ syntax (modern browsers; matches existing static files).
- No external dependencies (no CDN scripts).
- Each new JS file is self-contained; loaded via `<script src="/static/<file>.js" defer>`.
- DOM-ready guard: top-level `document.addEventListener("DOMContentLoaded", ...)`.

**No changes to:**

- `src/striatum/service.py` route table
- The JSON API (`/v1/*`)
- The SSE event feed
- The CSP
- The MCP surface
- The mutation gate
- The workflow visual builder
- The chat surface

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.

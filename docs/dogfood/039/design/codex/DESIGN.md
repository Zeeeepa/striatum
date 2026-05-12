# RFC 0037 Web UI Ergonomic Improvements - Codex Design

Date: 2026-05-12
Status: implementation design
author: designer-codex-gpt-5.5-001

## Summary

RFC 0037 should land as a progressive-enhancement pass over the current server-rendered Jinja2 UI. Keep the route table, JSON API, SSE feed, mutation gate, CSP, MCP surface, chat surface, and workflow visual builder behavior unchanged. The implementation should add three small vanilla-JS islands (`base.js`, `run_list.js`, `workflows_index.js`) plus two page-specific islands (`doctor.js`, `run_detail.js`) and minor data-shape additions to existing render helpers where the templates need fields already present in local state or on disk.

The core pattern is: the server still renders a complete usable page, then client-side JS reads a privacy-safe JSON data island and hides, labels, or reformats existing rows. There is no framework, bundler, node toolchain, CDN script, or new runtime dependency.

## File-By-File Plan

### `src/striatum/web/templates/base.html`

Add a skip link as the first body child and give the main element `id="main-content"` so keyboard users can bypass the header. Keep the visible nav as-is, then add a compact header tool group with a `button` for the time display mode and a `button` for shortcut help. Include a native `<dialog id="shortcut-help">` near the end of `<body>` with the four navigation shortcuts and the help shortcut. Load `/static/base.js` with `defer` before page-specific scripts.

The time toggle should expose `aria-pressed`, start with label `UTC`, and let `base.js` update it after reading `localStorage`. Do not add inline scripts or event attributes; CSP stays compatible with `script-src 'self'`.

### `src/striatum/web/templates/run_list.html`

Add a filter toolbar above the table with:

- Search input matching `run_id`, `branch_name`, and `workflow_id`.
- State segmented controls for `all`, `running`, `completed`, `canceled`, `failed`, and `paused`.
- Date-range select for `24h`, `7d`, `30d`, and `all`.

Add columns for `Workflow`, `Created`, and `Duration`; the RFC only requires duration, but workflow id is needed for the specified search target and is useful enough to show. Render timestamps as `<time datetime="{{ value }}">{{ value }}</time>` so `base.js` can rewrite them. Duration should be rendered server-side as a stable fallback when `completed_at` exists, and `run_list.js` can update running durations after load.

Embed a data island:

```html
<script type="application/json" id="run-list-data">{{ runs | tojson }}</script>
```

Each `<tr>` should carry `data-run-id`, `data-state`, `data-branch`, `data-workflow-id`, `data-created-at`, `data-started-at`, and `data-completed-at`. The table remains fully populated without JS; `run_list.js` only filters rows and refreshes duration cells.

Replace the empty copy with: "No runs yet. Run `striatum run prepare` to create your first run; see HOW_TO_HUMAN." Link to `/view/docs/HOW_TO_HUMAN.md` if the view route is available; otherwise use plain text plus the repo-relative doc path.

### `src/striatum/web/templates/workflows_index.html`

Add a filter toolbar above the table with path/workflow id search and status segmented controls for `all`, `valid`, and `invalid`. Add a `Last modified` column rendered with `<time datetime="{{ wf.modified_at }}">{{ wf.modified_at }}</time>`.

Embed:

```html
<script type="application/json" id="workflows-index-data">{{ workflows | tojson }}</script>
```

Each workflow row should carry `data-path`, `data-workflow-id`, and `data-status`, where invalid rows normalize both `workflow_error` and `parse_error` to `invalid` for filtering. Keep the first 200 chars of the error message in the status title as today.

Update the empty copy to: "No workflow.json files found. Run `striatum workflow generate` to create one; see HOW_TO_HUMAN."

### `src/striatum/web/templates/doctor.html`

Render problems grouped by `record.check` when `doctor.problem_records` is present. Use one `<details class="doctor-group" data-problem-kind="...">` per kind with a summary containing the kind, count, and optional doc link. Default-open groups with five or fewer problems; groups over five can be collapsed unless `doctor.js` has a saved preference.

Add a checkbox toggle above the groups: "Hide problems on terminal runs". Rows should include `data-terminal-run="true|false"` when the record context identifies a terminal run state. If context lacks enough structure, render `false` and let the problem stay visible. The fallback `doctor.problems` string list can be grouped under a single "legacy" group.

Add an empty-state block for `doctor.ok`: "0 problems found. Nothing to triage." Keep the existing ok pill so current tests remain easy to update.

Known doc-link mapping can be conservative in V1:

- `stale_queue_message_claim`, `unreaped_expired_lease`, and stale lease-like checks -> `docs/HOW_TO_HUMAN.md` recovery section.
- `active_session_on_terminal_run` -> `docs/HOW_TO_HUMAN.md` session close / doctor guidance when an anchor exists; if no stable anchor exists, link only to the doc.

### `src/striatum/web/templates/run_detail.html`

Promote next actions out of the status section. Immediately below the run header, render a `.next-actions-banner` only when `next_actions` is non-empty and `run.state` is not terminal (`completed`, `failed`, `canceled`). Keep the list text exactly as returned by `status()`; do not invent new action labels.

Render Status timestamps as `<time>` elements. Leave mutation controls, branch confirmation, pause/resume, cancel, posture chips, jobs rail, and graph placement otherwise unchanged.

Load `/static/run_detail.js` with `defer` on this page. The graph already has SVG `<title>` text, but RFC 0037 wants a fixed-position tooltip; do that client-side without changing navigation.

### `src/striatum/web/static/base.js` (new)

This is the shared UI utility island. It should be self-contained and loaded on every SSR page.

Responsibilities:

- LocalStorage helpers: `readJson(key, fallback)`, `writeJson(key, value)`, and `readString(key, fallback)` with try/catch fallback.
- Timezone toggle using key `striatum.ui.timezone` with values `"utc"` and `"local"`.
- Rewrite every `time[datetime]` on page load and when toggled. UTC mode preserves the ISO-ish text; local mode uses `Intl.DateTimeFormat` with date, time, and time zone name.
- Keyboard navigation: `g r` -> `/`, `g w` -> `/workflows`, `g c` -> `/chat`, `g d` -> `/doctor`, and `?` -> open help dialog.
- Disable shortcuts when focus is inside `input`, `textarea`, `select`, button editing contexts, or any `contenteditable` element.
- Escape closes the shortcut dialog when open.

Keep the module around 80 lines by using small helpers and one `DOMContentLoaded` handler. Do not export globals unless the later JS files genuinely need them; duplicated tiny localStorage helpers in page files are acceptable if that keeps coupling low.

### `src/striatum/web/static/run_list.js` (new)

Read `#run-list-data` and bind the filter toolbar. Persist the filter state under:

```text
striatum.ui.run_list.filter
```

Shape:

```json
{"search":"","state":"all","date":"all"}
```

Filtering rules:

- Search is case-insensitive substring over run id, branch, and workflow id.
- State `all` matches all; other states match `run.state`.
- Date range compares `created_at` to browser `Date.now()`. If parsing fails, keep the row visible.

Duration formatting should be deterministic and compact: `<60s` as `Xs`, `<1h` as `Xm Ys`, otherwise `Xh Ym`. Terminal runs use `created_at -> completed_at` if both exist. Running/prepared-like runs should prefer `started_at || created_at -> now`, refreshed once at page load and optionally every 30 seconds while the page is open.

When filters hide every row, show an in-page filtered empty message distinct from the true no-runs empty state: "No runs match the current filters."

### `src/striatum/web/static/workflows_index.js` (new)

Read `#workflows-index-data` and bind the search/status controls. Persist under:

```text
striatum.ui.workflows_index.filter
```

Shape:

```json
{"search":"","status":"all"}
```

Filtering rules:

- Search is case-insensitive substring over path and workflow id.
- Status `valid` matches only `valid`; `invalid` matches anything other than `valid`.

Show "No workflows match the current filters." when the table exists but filters hide all rows.

### `src/striatum/web/static/doctor.js` (new)

Use key `striatum.ui.doctor.hide_terminal` for the terminal-run toggle and `striatum.ui.doctor.collapsed_kinds` for a string array of manually collapsed group names. The template should be usable without JS because `<details>` provides native collapse behavior; JS only persists the toggle and collapse state and hides rows with `data-terminal-run="true"`.

When all rows in a group are hidden, hide the group. When all groups are hidden, show "No visible problems with the current filter."

### `src/striatum/web/static/run_detail.js` (new)

Attach hover/focus handlers to `.graph-node` or `.graph-node-link`. Create one fixed-position `.graph-tooltip` element. Read tooltip content from data attributes if available; otherwise fall back to SVG text content and the nested `<title>`.

The richer design is to add `data-job-id`, `data-role-id`, `data-state`, and `data-duration` to graph nodes in `graph_svg.py` during implementation. If the implementer wants to keep Python untouched for V1, the existing node title already carries `job_id: state (role)`, and `run_detail.js` can show that until a follow-up improves duration. The acceptance criterion asks for duration, so the better V1 path is to pass duration into the graph renderer via node metadata rather than scrape the status table.

Tooltip behavior: `position: fixed`, `pointer-events: none`, offset from cursor by 12px, clamped to viewport. It must not interfere with graph-node click navigation.

### `src/striatum/web/static/base.css`

Most active SSR styling already lives here. Add styles for skip link, header tool group, iconless compact buttons, shortcut dialog, filter toolbar, segmented filter buttons, filtered-empty states, next-actions banner, doctor groups, graph tooltip, and table duration/time cells.

Use existing variables (`--bg-base`, `--bg-elevated`, `--bg-overlay`, `--fg-*`, `--border`, `--accent`, `--status-*`) instead of adding a second palette. Keep radius at the existing `--radius` value.

### `src/striatum/web/static/app.css`

This file is now legacy-SPA CSS, but RFC 0037 explicitly calls out dark-mode parity here, so add a small `@media (prefers-color-scheme: dark)` block for the named classes: `.job-list`, `.job-link`, `.status-pill`, `.posture-chip`, `.run-grid`, `.run-jobs-rail`, `.run-meta`, `.run-events`, `.workflow-graph`, and `.workflow-edit-form`.

Where a named selector is active only in `base.css`, still include the selector in `app.css` with variable-backed colors so the acceptance check is satisfied without changing the active SSR page shape. Avoid changing the legacy SPA layout unless a test proves it is still user-visible.

## Minimal Server-Side Data Additions

RFC 0037 says no route-table or API changes, but the existing render helpers need a few extra fields:

- In `_render_run_list_page`, select `workflow_snapshot_id`, `started_at`, `completed_at`, and `paused_at` in addition to the current fields. Join or look up `workflow_snapshots.workflow_id` so search can match workflow id. The route stays `/`.
- In `_render_workflows_index_page`, include each workflow file's mtime. The cleanest place is `striatum.web.workflows.discover()`: add `modified_at` as UTC ISO 8601 from `path.stat().st_mtime`. Keep validation behavior unchanged.
- For graph tooltip duration, either enrich `render_run_graph()` with optional `node_meta` or compute duration data attributes from job rows. This is an internal helper signature change only; no public API change.
- For doctor terminal-run filtering, prefer using structured `problem_records[].context` as-is. If a check's context contains `run_state` in the terminal set, mark it terminal. Do not add new doctor checks for this RFC.

## JavaScript Architecture

Use vanilla ES6+ and one deferred script per page. There is no bundler and no shared package registry. Each file should follow this shape:

```js
document.addEventListener("DOMContentLoaded", () => {
  // Read data islands.
  // Restore localStorage state.
  // Bind events.
  // Apply once.
});
```

Data islands are the contract for filters. They should contain only the rows already rendered on the page, not hidden workflow prompts, artifact bodies, or free-text transcripts. The rendered table is the no-JS fallback; the data island is a convenience copy for filter predicates and duration calculations.

LocalStorage keys are stable and namespaced:

```text
striatum.ui.timezone
striatum.ui.run_list.filter
striatum.ui.workflows_index.filter
striatum.ui.doctor.hide_terminal
striatum.ui.doctor.collapsed_kinds
```

All JSON parsing should catch failures and reset to defaults. Invalid stored values should not break page rendering.

## Coverage Of The Ten RFC 0037 Wins

1. Run-list filter and duration: `run_list.html`, `run_list.js`, `_render_run_list_page`.
2. Workflows-index filter and last-modified: `workflows_index.html`, `workflows_index.js`, `web.workflows.discover`.
3. Doctor grouping and terminal-hide toggle: `doctor.html`, `doctor.js`.
4. Localtime toggle: `base.html`, `base.js`, `<time datetime>` markup across pages.
5. Graph-node tooltips: `run_detail.js`, with optional `graph_svg.py` data attributes.
6. Keyboard shortcuts: `base.js` and `base.html` help dialog.
7. `app.css` dark-mode parity: add the explicit media block while keeping active SSR colors in `base.css`.
8. Next-actions promotion: `run_detail.html` banner just below the run header for non-terminal runs.
9. Empty-state copy: `run_list.html`, `workflows_index.html`, and `doctor.html`, plus filtered-empty JS messages.
10. No new runtime dependencies: all changes are Jinja2, CSS, and vanilla JS over existing service data.

## Test Strategy

Keep existing UI tests passing, then add focused assertions rather than full browser automation.

Update existing tests:

- `tests/test_web_ui_redesign.py`: run-list empty copy, `base.js` script inclusion, shortcut dialog markup, localtime toggle markup, next-actions banner placement, graph tooltip hook.
- `tests/test_web_workflows.py`: workflows filter controls, `Last modified` column, workflow data island, revised empty copy.
- `tests/test_web_ui.py`: new static assets are served under `/static`, CSP remains free of `unsafe-inline` and `unsafe-eval`, and no external URL invariant still passes.

Add `tests/test_web_ergonomics.py` for RFC 0037:

- HTML contains run filter controls, state/date controls, duration header, and row data attributes.
- HTML contains workflow filter controls and mtime `<time>` elements.
- Doctor page renders grouped `<details>` and terminal-hide toggle for verbose problem records.
- Static JS contains the expected localStorage keys and shortcut mappings.
- CSS contains the named dark-mode selectors in `app.css`.

Where reasonable, add pure helper tests only if JS helpers are factored into importable Python-equivalent logic is not worth it. Since this repo does not have a Node test harness and RFC 0037 forbids a node toolchain, prefer server-rendered markup/static-file assertions plus manual verification for browser-only behavior.

Manual checklist before handoff:

- Keyboard shortcuts work and are ignored while focus is in search inputs or textareas.
- UTC/local toggle rewrites all visible `<time>` elements and persists across reloads.
- Run and workflow filters persist and restore after reload.
- Doctor terminal-hide toggle does not remove non-terminal problems.
- Graph tooltip follows hover/focus, stays inside viewport, and click navigation still works.
- OS dark mode renders the run list, run detail, doctor, workflows index, and workflow edit controls legibly.

Run targeted tests first:

```bash
pytest tests/test_web_ui.py tests/test_web_ui_redesign.py tests/test_web_workflows.py tests/test_web_ergonomics.py
```

Then run the requested broader checks:

```bash
make test
make smoke
```

## Risks And Guardrails

The highest risk is accidentally converting server-rendered pages into JS-required pages. Keep the initial HTML complete and use JS only to filter and enhance. The second risk is CSP drift; all event handlers must be bound from deferred static files. The third risk is stale hidden data in localStorage; every parser should have defaults and validation.

Do not broaden the RFC into visual-builder improvements, chat changes, hosted UX, query-string filter state, sticky next-actions, graph zoom/pan, or a doctor mutation button. Those are explicitly V1.5 or separate-RFC territory.

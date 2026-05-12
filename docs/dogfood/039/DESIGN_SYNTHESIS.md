---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/039/design/codex/DESIGN.md", "docs/dogfood/039/design/claude_code/DESIGN.md", "docs/dogfood/039/design/gemini/DESIGN.md"]
---

author: designer-codex-gpt-5.5-001

# RFC 0037 Design Synthesis

Status: implementation plan
Date: 2026-05-12

## Accepted Implementation Scope

RFC 0037 V1 should land as a progressive-enhancement pass over the current Jinja2 pages. The implementation uses vanilla ES6+ files loaded with `defer`, keeps the server-rendered tables and lists usable without JavaScript, and adds JSON data islands only for client-side filtering and formatting. There is no framework, bundler, Node toolchain, new Python dependency, new route, new API endpoint, MCP change, or mutation-gate change.

The acceptance-criteria mapping is:

| RFC 0037 acceptance criterion | Concrete owner and plan |
|---|---|
| Run list page has a working free-text + state + date-range filter, with state persisted in `localStorage`. | `run_list.html` owns the filter toolbar, data island, row `data-*` attributes, and filtered-empty region. `run_list.js` owns predicates, restore/save, clear controls, and applying hidden rows. |
| Run list shows a duration column. | `_render_run_list_page` in `service.py` owns adding `started_at` and `completed_at` to the run rows. `run_list.html` owns the `Duration` column and fallback text. `run_list.js` owns live running durations. |
| Workflows index has a working path / workflow_id search + status filter. | `workflows_index.html` owns the toolbar, status buttons, data island, row `data-path`, `data-workflow-id`, and normalized `data-status`. `workflows_index.js` owns filtering and persistence. |
| Workflows index shows a `Last modified` column. | `src/striatum/web/workflows.py` owns `modified_at` from `path.stat().st_mtime` as UTC ISO 8601. `workflows_index.html` owns the `<time datetime="...">` column. |
| Doctor page groups problems by kind with collapsible sections. | `_render_doctor_page` owns grouping `doctor.problem_records` by `record.check` before rendering. `doctor.html` owns `<details class="doctor-group">` sections. `doctor.js` owns persistence for user-expanded and user-collapsed kinds. |
| Doctor page has a "hide terminal-run problems" toggle. | `doctor.html` owns the checkbox and `data-terminal-run` / `data-run-state` attributes. `doctor.js` owns defaulting, filtering, and visible-empty messaging. Terminal detection is advisory because not every doctor record carries run-state context. |
| Localtime toggle in the header switches every `<time>` element on the page; preference persists. | `base.html` owns the header button and loads `base.js`. `base.js` owns the time formatter, storage read/write, and re-rendering `time[datetime]`. Templates touched by RFC 0037 must wrap visible timestamps in `<time datetime="...">`. |
| Graph nodes on run detail show hover tooltips with job state + role + duration. | `graph_svg.py` owns adding `data-job-id`, `data-role-id`, `data-state`, and `data-duration` when metadata is available. `run_detail.js` owns the fixed-position tooltip and focus/hover handlers. |
| Keyboard shortcuts `g r` / `g w` / `g c` / `g d` work when focus is not on an input; `?` opens help. | `base.html` owns the native `<dialog>` and help button. `base.js` owns the `g` sequence state machine, input-focus guard, dialog open/close, outside-click close, and navigation targets. |
| `app.css` has `prefers-color-scheme: dark` blocks for all named components. | `app.css` owns an explicit dark-mode audit block for `.job-list`, `.job-link`, `.status-pill`, `.posture-chip`, `.run-grid`, `.run-jobs-rail`, `.run-meta`, `.run-events`, `.workflow-graph`, and `.workflow-edit-form`. `base.css` owns active SSR utility styling. |
| Run detail page surfaces "Next actions" prominently for non-terminal runs. | `run_detail.html` owns moving the existing `next_actions` list directly below `.run-header` into `.next-actions-banner`; terminal states hide it. `base.css` owns banner styling. |
| Empty-state copy added to run list, workflows index, doctor. | `run_list.html`, `workflows_index.html`, and `doctor.html` own true empty states. `run_list.js`, `workflows_index.js`, and `doctor.js` own filtered-empty states. |
| No new runtime dependencies. | All implementation files stay under existing Python/Jinja2/CSS/static-JS surfaces. Tests should assert no external URLs and CSP remains without `unsafe-inline` / `unsafe-eval`. |
| No regressions to existing functionality. | Existing mutation scripts (`run_cancel.js`, `run_pause_resume.js`, `run_branch_confirm.js`, `job_actions.js`, `workflow_run.js`, `workflow_edit.js`), chat pages, browse/view pages, and workflow builder pages remain wired as they are. |
| `make test` + `make smoke` + existing UI snapshot tests pass. | Implementer runs focused UI tests first, then `make test` and `make smoke` before handoff. |
| Doc-link tests pass. | Documentation update owns any new `HOW_TO_HUMAN.md` anchors used by empty states or doctor links. Where per-kind anchors do not exist, V1 links to the broad doc section and marks finer anchors as doc-pass follow-up. |
| Changelog + README + UBIQUITOUS_LANGUAGE + HOW_TO_HUMAN updates if applicable. | Docs phase owns `CHANGELOG.md`, `HOW_TO_HUMAN.md`, and RFC 0037 status. README/UBIQUITOUS_LANGUAGE only change if implementation introduces durable vocabulary; the preferred plan introduces none. |

## Deferred Scope

Filter-state-in-querystring is deferred to V1.5. V1 persists filter state only in `localStorage` and does not make filtered views shareable by URL.

Sticky-positioned next-actions is deferred to a future RFC if requested. V1 promotes the block in normal document flow and avoids sticky overlap risks.

Keyboard-shortcut configurability is deferred to a future RFC. V1 ships a fixed small shortcut set.

Per-doctor-problem-kind doc anchors that do not exist in `HOW_TO_HUMAN.md` should not block implementation. The implementer should add anchors for the known RFC 0037 links when editing docs; unknown or newly discovered doctor kinds should link to the broad recovery/doctor section and be noted as defer-to-doc-pass.

Do not add graph zoom/pan, drag-and-drop workflow editing, hosted-mode UX, mobile-first redesign, doctor mutation buttons, chat behavior changes, or new API endpoints.

## JS Architecture

Use vanilla ES6+, no framework, no bundler, and no build step. Every script is a static file under `src/striatum/web/static/` and is loaded with `defer` from the owning template. Use one shared file, `base.js`, plus page-level files: `run_list.js`, `workflows_index.js`, `doctor.js`, and `run_detail.js`.

Each file should follow the same shape:

```js
document.addEventListener("DOMContentLoaded", () => {
  // read data island or DOM rows
  // restore versioned localStorage state
  // bind events
  // apply once
});
```

No inline handlers, no inline executable script, and no global dependency between page files. If a helper would introduce coupling, duplicate a small safe helper locally. The exception is `base.js`, which may expose a tiny `window.StriatumUI` object for storage and duration helpers only if page scripts need it; otherwise keep page files self-contained.

## localStorage Key Naming Convention

Use `striatum.ui.<feature>.<field>` for every persisted preference:

| Key | Value shape |
|---|---|
| `striatum.ui.time.mode` | `{"version":1,"value":"utc"}` or `{"version":1,"value":"local"}` |
| `striatum.ui.run_list.filter` | `{"version":1,"search":"","state":"all","date":"all"}` |
| `striatum.ui.workflows_index.filter` | `{"version":1,"search":"","status":"all"}` |
| `striatum.ui.doctor.hide_terminal` | `{"version":1,"value":"on"}` or `{"version":1,"value":"off"}` |
| `striatum.ui.doctor.groups` | `{"version":1,"collapsed":[],"expanded":[]}` |

On read, parse with `try/catch`. If the object is missing, malformed, has an unexpected `version`, or contains an unsupported value, reset to defaults and continue rendering. Defaults must preserve pre-RFC behavior except the doctor terminal-hide toggle, whose first-visit default is ON only when the unfiltered problem count is greater than 20.

## Data Island Pattern

Use server-rendered JSON data islands:

```html
<script type="application/json" id="run-list-data">{{ runs | tojson }}</script>
<script type="application/json" id="workflows-index-data">{{ workflows | tojson }}</script>
```

The server still renders the full table initially. JavaScript reads the island and/or existing row attributes to filter client-side. Data islands contain only fields already visible or needed for UI predicates: ids, branch names, workflow ids, states, UTC timestamps, validation status, and paths. They must not include prompts, artifact bodies, transcript-like text, tokens, or hidden workflow content.

## Filter UX Choices

Run list:

- Search placeholder: `Search runs by id, branch, workflow id...`
- State buttons: `all`, `running`, `completed`, `canceled`, `failed`, `paused`
- Date select labels: `All time`, `Last 24h`, `Last 7 days`, `Last 30 days`
- Defaults: search `""`, state `all`, date `all`
- Clear affordances: an input clear button for search and a `Clear filters` button for the whole toolbar
- Filtered-empty copy: `No runs match the current filters.`

Workflows index:

- Search placeholder: `Search workflows by path or workflow id...`
- Status buttons: `all`, `valid`, `invalid`
- Defaults: search `""`, status `all`
- Clear affordances: input clear button and `Clear filters`
- Filtered-empty copy: `No workflows match the current filters.`

Doctor:

- No text search in V1
- Toggle label: `Hide problems on terminal runs`
- Defaults: ON when unfiltered problem count is greater than 20, otherwise OFF, unless storage has a valid value
- Filtered-empty copy: `No visible problems with the current filter.`

Search is case-insensitive substring matching. Date range compares `created_at` to browser `Date.now()`; if parsing fails, leave the row visible.

## Duration Column Format

Use one formatter for run durations and graph tooltip durations:

- Less than 60 seconds: `Xs`
- Less than 1 hour: `Xm Ys`
- 1 hour or more: `Xh Ym`
- Missing start: `-`

Terminal runs use `created_at -> completed_at` when both exist. Running or prepared-like runs use `started_at || created_at -> now` and refresh every 30 seconds with `setInterval`. The visible running text should be the duration only in the table cell; tooltip copy may say `running for Xm Ys`.

## Localtime Toggle UX

Place a compact button in the right side of `.site-header`, after the nav. Button text shows the current mode: `UTC` or `Local`. Default is UTC. Clicking flips the mode, persists `striatum.ui.time.mode`, updates `aria-pressed`, and rewrites every `time[datetime]`.

UTC mode restores text from the immutable `datetime` attribute. Local mode uses `Intl.DateTimeFormat` with the browser locale and timezone, including date and time but not seconds. If formatting fails, leave the existing text unchanged. The `datetime` attribute is never modified.

## Keyboard Shortcut Overlay UX

Use one native `<dialog id="shortcut-help">` in `base.html`. `?` opens the dialog. `Esc` closes through native behavior. A close button and outside-click handler also close it. The dialog content is a `<dl>` listing:

- `g r` Runs
- `g w` Workflows
- `g c` Chat
- `g d` Doctor
- `?` Show this help
- `Esc` Close this help

The shortcut listener uses a one-second `g` sequence window. It ignores events when focus is inside `input`, `textarea`, `select`, `button`, `[contenteditable]`, or any element inside an open dialog.

## Empty-State Copy

Use inline SVG empty icons, then this exact copy:

Run list true empty:

```text
No runs yet.
Run `striatum run prepare --workflow <workflow.json>` to create your first run; see docs/HOW_TO_HUMAN.md.
```

Run list filtered empty:

```text
No runs match the current filters.
```

Workflows true empty:

```text
No workflow.json files found.
Run `striatum workflow generate <path> --shape minimal --lane-set local --artifact-root striatum/<name>` to create one; see docs/HOW_TO_HUMAN.md.
```

Workflows filtered empty:

```text
No workflows match the current filters.
```

Doctor true empty:

```text
0 problems found. Nothing to triage.
```

Doctor filtered empty:

```text
No visible problems with the current filter.
```

Link `docs/HOW_TO_HUMAN.md` through the existing view route if available, otherwise use a normal relative doc link only in docs contexts. The implementation should prefer `/view/docs/HOW_TO_HUMAN.md` for web pages.

## Next-Actions Banner Layout

Move the existing `next_actions` block in `run_detail.html` to directly below `.run-header` and before `.run-grid`.

```html
<section class="next-actions-banner" role="region" aria-label="Next actions" tabindex="0">
  <h2>Next actions</h2>
  <ul class="next-actions">...</ul>
</section>
```

Show only when `next_actions` is non-empty and `run.state` is not one of `completed`, `failed`, or `canceled`. It is full-width within `.page-section`, not sticky. Use existing action text verbatim.

## Doctor Grouping Behavior

Group structured records by `record.check`. Each group is a native `<details>` with a `<summary>` showing kind, count, and optional doc link. Groups with more than five problems are collapsed by default; groups with five or fewer are open by default. The `striatum.ui.doctor.groups` value stores explicit user overrides as `collapsed` and `expanded` arrays so new groups still follow the default rule.

The terminal-hide toggle filters records with terminal run state in context. Terminal states are `completed`, `failed`, and `canceled`. If a record lacks structured run state, mark it non-terminal and keep it visible. This is an advisory filter over available context, not a new doctor guarantee.

## Graph Node Tooltips

Use `run_detail.js`. Create a single `.graph-tooltip` element with `position: fixed` and `pointer-events: none`. Attach `mouseenter`, `mousemove`, `mouseleave`, `focus`, and `blur` to graph node links or groups. Position 12px from the pointer and clamp within the viewport.

Tooltip content is:

```text
<job name>
Role: <role id>
State: <state>
Duration: <duration or "-">
```

The preferred implementation adds data attributes in `graph_svg.py`. Fallback may read nested `<title>`, but acceptance requires role, state, and duration, so `graph_svg.py` should receive or derive metadata rather than relying only on text scraping.

## app.css Dark-Mode Parity Audit

The active SSR UI is mostly styled by `base.css`, but RFC 0037 explicitly requires `app.css` parity. Add an explicit `@media (prefers-color-scheme: dark)` block in `app.css` with these decisions:

| Selector | Decision |
|---|---|
| `.job-list` | Explicit dark block: elevated background and border token. |
| `.job-link` | Explicit dark block: foreground token and hover background token. |
| `.status-pill` | Explicit dark block: preserve state-specific colors with readable text tokens. |
| `.posture-chip` | Explicit dark block: elevated background, border, foreground. |
| `.run-grid` | Explicit dark block: border/background tokens for contained rails. |
| `.run-jobs-rail` | Explicit dark block: elevated background and border. |
| `.run-meta` | Inherits foreground/muted tokens; add explicit muted descendant rule if literals remain. |
| `.run-events` | Explicit dark block for event rows, borders, and background. |
| `.workflow-graph` | Explicit dark block for container background/border and SVG text/edge colors when CSS-controlled. |
| `.workflow-edit-form` | Explicit dark block for inputs, cards, borders, and help text. |
| `.next-actions-banner` | Style in `base.css`; add `app.css` parity selector if the class appears there. |
| `.filter-toolbar`, `.filtered-empty`, `.graph-tooltip`, `.doctor-group` | Style in `base.css`; not required by RFC list, but include dark-safe token usage. |

Do not introduce a second palette. Reuse existing `base.css` variables wherever possible.

## Accessibility Checklist

- Add a skip link before the header and `id="main-content"` on `<main>`.
- Filter toolbars use real labels or `aria-label`; segmented buttons use `aria-pressed`.
- Clear controls are real `<button>` elements.
- The shortcut help is a native `<dialog>` with close button, outside-click close, and no autofocus stealing on normal page load.
- Keyboard shortcuts are disabled in editable contexts.
- The next-actions banner has `role="region"`, `aria-label="Next actions"`, and `tabindex="0"`.
- Doctor groups use native `<details>` / `<summary>`.
- Graph tooltip also appears on keyboard focus, not only mouse hover.
- Graph nodes keep existing navigation and accessible names.
- Empty-state SVGs are decorative with `aria-hidden="true"` unless they carry unique text, which they should not.
- Focus outlines remain visible in light and dark mode.

## Responsive Behavior

Desktop remains the primary target. The implementation should still avoid obvious breakage at 1024px.

- Filter rows use `display: flex`, `flex-wrap: wrap`, and gap tokens.
- Search inputs have a sensible `min-width` and can grow to the available row width.
- The existing run-grid collapse at widths below 900px is preserved or added if missing: jobs, status, and graph stack vertically.
- The shortcut dialog width is `min(32rem, calc(100vw - 2rem))`, so it fits a 1024px viewport and narrower development windows.
- Tables keep horizontal overflow behavior rather than crushing text.
- Mobile-first redesign remains out of scope.

## Empty-State Illustration Choice

Use one reusable inline SVG icon pattern: 48x48px, `viewBox="0 0 48 48"`, `fill="none"`, `stroke="currentColor"`, `aria-hidden="true"`. Keep it generic, such as a small outlined folder/search panel shape, and vary only CSS class or surrounding copy per page. This avoids emoji, external images, base64 assets, and new icon dependencies while remaining theme-compatible through `currentColor`.

## Test Strategy

Automated tests should stay Python-side because RFC 0037 forbids adding a Node toolchain. Add focused assertions rather than broad brittle snapshots:

- Existing UI tests continue to pass.
- Static asset tests assert `/static/base.js`, `/static/run_list.js`, `/static/workflows_index.js`, `/static/doctor.js`, and `/static/run_detail.js` are served and CSP still excludes `unsafe-inline` and `unsafe-eval`.
- Markup tests assert run-list filter controls, data island, duration header, row data attributes, and true empty copy.
- Workflow tests assert filter controls, `Last modified`, `<time datetime>`, data island, and empty copy.
- Doctor tests assert grouped `<details>`, terminal-hide toggle, grouped counts, and zero-problem copy.
- Run-detail tests assert next-actions banner placement and terminal hiding.
- Static-content tests assert expected localStorage keys, shortcut mappings, duration formatter branches, filter predicate branches, input-focus guard, and dark-mode selectors appear in the JS/CSS files.
- Doc-link tests assert any anchors linked from empty states or known doctor mappings exist.

Manual checklist before handoff:

- `UTC` / `Local` rewrites all visible `<time>` values and persists after reload.
- `g r`, `g w`, `g c`, `g d`, and `?` work; shortcuts do nothing while typing in a filter input or textarea.
- Help dialog closes via Esc, close button, and outside click.
- Run-list and workflow filters persist and clear correctly.
- Running duration updates after 30 seconds without changing terminal durations.
- Doctor groups collapse/expand by keyboard and persist user choices.
- Graph tooltip follows hover/focus, does not block click navigation, and stays inside the viewport.
- OS dark mode keeps run list, run detail, workflows index, doctor, and workflow edit controls legible.

Run focused tests first:

```bash
pytest tests/test_web_ui.py tests/test_web_ui_redesign.py tests/test_web_workflows.py tests/test_web_ergonomics.py
```

Then run:

```bash
make test
make smoke
```

## Staging Plan

1. Localtime toggle + `base.js` scaffold + skip-link + help dialog. This is lowest risk and creates the shared static-JS pattern.
2. Run-list filters, duration column, and empty state. This proves the data-island pattern on the simplest table.
3. Workflows-index filters, last-modified column, and empty state. This reuses the same pattern with filesystem metadata.
4. Doctor grouping, terminal-hide toggle, and empty state. This is noisier because the source data is less uniform.
5. Graph node tooltips. This may need a small `graph_svg.py` metadata extension but does not affect routing.
6. Next-actions banner promotion. This is a template reorder using existing data.
7. `app.css` dark-mode parity. This is mechanical but should happen after classes settle.
8. Docs: `HOW_TO_HUMAN`, `CHANGELOG`, RFC 0037 status block, and README cross-reference only if applicable.

## Documentation Deltas

`docs/HOW_TO_HUMAN.md` should gain a short web UI walkthrough covering run-list filters, workflow filters, localtime toggle, keyboard shortcuts, doctor grouping, and the meaning of next actions. It should also own stable anchors for starting a run, generating a workflow, stale-lease recovery, closing sessions, and doctor triage if those anchors are linked from templates.

`CHANGELOG.md` should get an Unreleased entry for RFC 0037 ergonomic improvements.

`docs/rfcs/0037-web-ui-ergonomic-improvements.md` should move from proposed to accepted/implemented status when the implementation lands.

`README.md` only needs a cross-reference if the web UI section starts naming these controls; otherwise avoid README churn.

`docs/UBIQUITOUS_LANGUAGE.md` likely needs no update because this plan reuses existing terms: run, workflow, doctor, next action, local web UI, and artifact.

## Human-Decision Questions

No blocking human decision is required for implementation. The synthesis chooses localStorage-only V1 state, inline SVG empty illustrations, fixed shortcut bindings, in-flow next-actions banner, vanilla JS files, and broad `HOW_TO_HUMAN` links when fine-grained doctor anchors do not exist.

One non-blocking implementation judgment remains: if adding duration metadata to `graph_svg.py` becomes disproportionate, the implementer may first ship state/role tooltip data and record duration as a small follow-up. That would miss the strict RFC acceptance criterion, so the preferred V1 implementation is to add the metadata now.

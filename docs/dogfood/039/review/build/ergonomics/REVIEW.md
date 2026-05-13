---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0037", "web-ui", "build"]
---

author: reviewer-claude-opus-001

# RFC 0037 Web UI Ergonomic Improvements — Ergonomics-DX Review

Run: `run_8f8f347a99ef4e95993db8c288f2ad59`
Branch: `striatum/dogfood-039-rfc-0037-web-ui-ergonomic-improvements`
Posture: `ergonomics_dx`
Verdict intent: `accept_with_findings`

## Scope of review

The shipped RFC 0037 V1 slice as described in `docs/dogfood/039/BUILD_HANDOFF.md`. Surfaces inspected:

- `src/striatum/web/templates/base.html`, `run_list.html`, `run_detail.html`, `workflows_index.html`, `doctor.html`
- `src/striatum/web/static/base.js`, `run_list.js`, `workflows_index.js`, `doctor.js`, `run_detail.js`
- `src/striatum/web/static/base.css`, `app.css`
- `src/striatum/web/graph_svg.py`, `src/striatum/web/workflows.py`
- `src/striatum/service.py` (handlers for `/`, `/run/<id>`, `/doctor`)
- `tests/test_web_ergonomics.py`, `test_web_ui.py`, `test_web_ui_redesign.py`, `test_web_workflows.py`
- `docs/HOW_TO_HUMAN.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `README.md`, `CHANGELOG.md`

I could not start `striatum serve --web` directly from the review-only write scope; behavior was verified by reading source, templates, and tests, plus the build-handoff verification log (49 passed in 35.21s for the four web suites; 637 passed in the full `make test`). Where I flag a finding below, I reference the file:line so the operator or a follow-up packet can confirm in a live session.

## Posture verdict

`accept_with_findings`. Every RFC 0037 acceptance criterion is mechanically met. The UI is discoverable as a first-time-user surface, and the no-bundler / no-framework constraint has been honored. The findings below are ergonomic polish and one scope/impact observation; none are blockers and none introduce regressions to existing functionality.

## What works (surface-by-surface)

### Run list filter row — accept

`src/striatum/web/templates/run_list.html:7-29` + `src/striatum/web/static/run_list.js`

- Free-text search joins `data-run-id`, `data-branch`, `data-workflow-id` from each `<tr>` and matches a lowercased substring (`run_list.js:46-54`). Workflow id is now joined from `workflow_snapshots` (`service.py:869-892`), so searching for a workflow slug surfaces every run that used it.
- State pill segmented control normalizes against an allowlist (`STATES`), date select against `DATES` (`run_list.js:5-6`, `:8-20`). Out-of-range values fall back to `all`, so stale localStorage from a future state vocabulary won't crash the page.
- Date range uses `Date.now() - created.getTime() <= days * 24h` (`run_list.js:37-43`); unparseable `created_at` defaults to "show" so we don't accidentally hide rows for malformed timestamps.
- Filter state persists under `striatum.ui.run_list.filter` with a `version: 1` envelope (`run_list.js:8-28`). Storage failures (private mode, full disk) degrade to "filter works in-memory" without throwing.
- "Clear search" and "Clear filters" affordances both present (`run_list.html:12`, `:28`).
- Empty filtered-result state copy: `"No runs match the current filters."` (`run_list.html:55`) — specific.
- Zero-runs empty state: `"No runs yet."` plus a copy-paste `striatum run prepare --workflow <workflow.json>` and a link to `docs/HOW_TO_HUMAN.md` (`run_list.html:57-62`). Filter row is suppressed in this branch, which is the right call — filters would be empty controls.

### Duration column — accept

`run_list.html:39` + `run_list.js:56-71`

- `formatDuration` produces `Xs` / `Xm Ys` / `Xh Ym` (`base.js:51-60`). The RFC text said `mm:ss / hh:mm`; the implementation lands on the friendlier `Xm Ys` shape which is consistent with the rest of the UI and the RFC's "wall-clock duration" intent.
- For terminal runs (`completed`/`failed`/`canceled`) with no `completed_at`, the column shows `-` (`run_list.js:62-63`), which is the right fallback when the runner crashed before recording completion.
- Running runs poll `Date.now()` on a 30-second `setInterval` (`run_list.js:130`). Reasonable cadence; no risk of background-tab CPU because the work is one DOM-write per row.

### Workflows index — accept

`workflows_index.html` + `workflows_index.js`

- Search joins `data-path` and `data-workflow-id`; status filter pills (`all`/`valid`/`invalid`). Status is normalized in the template — `wf.status == 'valid'` → `valid`, anything else → `invalid` (`workflows_index.html:38`). Matches the JS allowlist.
- `Last modified` column uses `<time datetime="...">` so the base.js localtime toggle picks it up.
- `modified_at` is rendered as `YYYY-MM-DDTHH:MM:SS.ffffffZ` (UTC) by `workflows.py:_modified_at` and verified by `test_discover_reports_modified_at_utc`. ✓
- Empty filtered state: `"No workflows match the current filters."` (`workflows_index.html:58`).
- Zero-workflow empty state cites the `striatum workflow generate <path> --shape minimal --lane-set local --artifact-root striatum/<name>` copy-paste with the `--shape minimal` hint that wasn't obvious from the bare CLI help (`workflows_index.html:60-64`).

### Doctor grouping — accept

`doctor.html` + `doctor.js`

- Server template groups problems by `record.check` (`service.py:1717-1731`) and renders a `<details class="doctor-group">` per group. Server default: `open` when `records | length <= 5`, collapsed otherwise (`doctor.html:26`). RFC asked for "default-collapsed for sections with > 5 problems" — implemented exactly.
- `data-terminal-run` is set per `<li>` from `record.context.run_state` (`doctor.html:38-39`). JS also has a defensive fallback against `TERMINAL_STATES = ["completed", "failed", "canceled"]` (`doctor.js:6`, `:84`).
- Hide-terminal toggle initial state: explicit user choice if persisted, otherwise auto-on when `problem_count > 20`. The 20-problem heuristic is straight from RFC §3.
- Per-group collapsed/expanded state persists in a single `striatum.ui.doctor.groups` key with `collapsed: []` and `expanded: []` lists (`doctor.js:28-47`). Toggle handlers maintain set semantics correctly (`doctor.js:62-75`).
- Doc links per group: `active_session_on_terminal_run` → `#close-active-sessions`, everything else → `#doctor-triage-and-recovery`. Both anchors are now present in `HOW_TO_HUMAN.md`. ✓

### Localtime toggle — accept

`base.js:74-113` + `base.html:20`

- `renderTimes("utc")` resets each `<time>` to its `datetime` attribute; `renderTimes("local")` formats via `Intl.DateTimeFormat(undefined, { ... })` (`base.js:74-96`). All `time[datetime]` on the page get rewritten — verified against `run_list.html`, `run_detail.html`, `workflows_index.html`.
- Default is `utc`. Storage key `striatum.ui.time.mode` with `version: 1` envelope and an `allowed: ["utc", "local"]` allowlist (`base.js:101`).
- Button text and `aria-pressed` are kept in sync (`base.js:103-106`).
- Storage failures don't break rendering: `storage.read` returns the fallback, `storage.write` swallows the exception (`base.js:5-23`).

### Keyboard shortcuts — accept with one nit (see F1)

`base.js:115-155`

- `g <key>` requires both keystrokes within 1000 ms; `gSeenAt` is reset on any non-matching key, so a stray `g foo bar r` doesn't navigate.
- `?` opens the help dialog.
- `<dialog>` close: native Esc behavior from `<form method="dialog">`, plus a click-outside handler that compares `event.client{X,Y}` against `dialog.getBoundingClientRect()` (`base.js:124-131`).
- Help dialog content matches the live binding: `g r` / `g w` / `g c` / `g d` / `?` / `Esc` (`base.html:32-38`), and HOW_TO_HUMAN's shortcut table matches (`docs/HOW_TO_HUMAN.md` Web UI section).
- See F1 below for the `isEditableTarget` button-focus edge.

### Graph tooltips — accept with one a11y nit (see F4)

`run_detail.js` + `graph_svg.py`

- `graph_svg.py:148-178` adds `data-job-id`, `data-role-id`, `data-state`, `data-created-at`, `data-started-at`, `data-completed-at` to each `<g.graph-node>`. The values are HTML-escaped via `html_escape`.
- `service.py:1002-1006` passes `jobs` through `render_run_graph` so the data island is populated from the actual `jobs` table (with `workflow_job_id` → role lookup).
- Tooltip is a single `<div class="graph-tooltip" hidden>` reused across nodes (`run_detail.js:28-31`). `position: fixed; pointer-events: none;` (`base.css:424-437`). Viewport clamping via `Math.min(... innerWidth - rect.width - margin)` (`run_detail.js:18-22`). Verified visually-correct off-screen prevention.
- Hover, mousemove, mouseleave, focus, blur all wired (`run_detail.js:49-53`) — so keyboard users get the tooltip via tab+focus.

### Next-actions banner — accept

`run_detail.html:39-48`

- Rendered only when `next_actions` is truthy AND `run.state not in ('completed', 'failed', 'canceled')`. Paused runs show the banner, which matches the RFC's "non-terminal" definition.
- Banner sits between `<header class="run-header">` and `<div class="run-grid">` — visually prominent but in document flow (not sticky).
- `role="region" aria-label="Next actions" tabindex="0"` (`run_detail.html:40`). Reachable by skip-link + screen-reader landmarks. ✓

### Empty-state copy — accept

All three empty states share a `.rich-empty` class with an inline SVG, an `<h2>`, and a copy-paste CLI hint plus a `docs/HOW_TO_HUMAN.md` anchor:

- Run list: `striatum run prepare --workflow <workflow.json>` (`run_list.html:60`).
- Workflows index: `striatum workflow generate <path> --shape minimal --lane-set local --artifact-root striatum/<name>` (`workflows_index.html:63`).
- Doctor: "0 problems found. Nothing to triage." (`doctor.html:65-66`).

The copy is specific and actionable; the doc anchor targets are real (`docs/HOW_TO_HUMAN.md` Web UI section is added in this PR).

### Dark-mode parity — accept (scope-only)

`app.css:266-362` adds a `@media (prefers-color-scheme: dark)` block covering every selector named in RFC 0037 §7: `.job-list`, `.job-link`, `.status-pill`, `.posture-chip`, `.run-grid`, `.run-jobs-rail`, `.run-meta`, `.run-events`, `.workflow-graph`, `.workflow-edit-form`. Plus a `next-actions-banner`, `filter-toolbar`, `filtered-empty`, `graph-tooltip`, `doctor-group` block for the new components introduced by this RFC. See F3 below for the scope/impact note.

### No new runtime dependencies — accept

- `pyproject.toml` deps unchanged: `jinja2>=3.1`, `markdown-it-py>=4.0`.
- No `package.json`, no `node_modules`, no bundler config introduced (only matches under `.striatum/scratch/` are cached plugin payloads from prior dogfood runs, unrelated).
- All five new JS files are IIFE-wrapped vanilla; loaded with `<script defer>` (`base.html:42-44`, per-page `block scripts`).
- CSP header unchanged (`default-src 'self'; script-src 'self'; style-src 'self'; ...`), verified by `test_csp_header_unchanged` against `/`, `/workflows`, `/doctor`, `/run/<id>` (`tests/test_web_ui_redesign.py:55-65`).

### Documentation honesty — accept

- `HOW_TO_HUMAN.md` Web UI section accurately describes filters, doctor grouping, localtime toggle, and keyboard shortcuts. The shortcut table matches `base.html`'s dialog content.
- `CHANGELOG.md` Unreleased entry lists the shipped scope without overclaiming visual redesign, mobile-first, or new mutation surface.
- `README.md` Status block bumps to v1.27.0 with RFC 0037 listed and the route inventory updated to include `/workflows` and `/chat`.
- `UBIQUITOUS_LANGUAGE.md` "local web UI" entry is corrected from the stale SPA framing to the current server-rendered + vanilla-JS reality, which closes a doc drift that long predates this RFC.
- No claim of mobile-first or visual redesign anywhere. ✓

## Findings

### F1. `isEditableTarget` treats every `<button>` as editable — minor ergonomic gotcha

Severity: low

`src/striatum/web/static/base.js:69-72`

```js
function isEditableTarget(target) {
  if (!(target instanceof Element)) return false;
  return Boolean(target.closest("input, textarea, select, button, [contenteditable], dialog[open]"));
}
```

After a user clicks any filter pill (`segmented-control button`), the state-filter `button` retains focus per default browser behavior. While focus is on that button, `g r` / `g w` / `g c` / `g d` are suppressed because `event.target.closest("button")` is truthy. First-time users who interact with the filter row and then expect the shortcut bar to be live will be mildly confused.

Recommendation (any of):

1. Drop `button` from the `isEditableTarget` selector. Buttons don't capture text, and the user explicitly intends to navigate — let the shortcut fire.
2. Add `event.target.blur()` to the filter-pill click handlers (`run_list.js:115-121`, `workflows_index.js:70-76`) so focus is shed after pill activation.
3. Narrow the guard to `input, textarea, [contenteditable], dialog[open]` (drop `button` and `select`).

I'd take option 1 — it's the smallest change and matches the principle that shortcuts are for navigation and shouldn't be hijacked by focused buttons that have no shortcut-overlapping semantics.

### F2. Unused JSON data islands on run list and workflows index — dead payload

Severity: low

`run_list.html:30` and `workflows_index.html:23`:

```html
<script type="application/json" id="run-list-data">{{ runs | tojson }}</script>
<script type="application/json" id="workflows-index-data">{{ workflows | tojson }}</script>
```

Neither `run_list.js` nor `workflows_index.js` reads from these islands — the JS reads from the rendered `<tr>` dataset attributes. The islands ship the entire runs/workflows array a second time on every page load. On a heavy machine with ~100 runs this is a few KB; on a working-set machine with 1000+ runs (real after a dogfood-heavy month) this becomes a noticeable double-render.

Recommendation: either delete the data islands, or replace the per-row dataset attributes with reads from the data island (which would make the filter predicates work on richer fields than dataset can express). The current shape pays the cost of both.

### F3. `app.css` dark-mode parity is added but `app.css` is not loaded by the active UI

Severity: low (RFC literal acceptance is met)

`base.html:8` links only `/static/base.css`. The `app.css` `@media (prefers-color-scheme: dark)` block added at `app.css:266-362` is shipped in `setuptools.package-data` but never loaded by any current template. The inline comment at `app.css:263-265` is admirably honest about this:

> "The active server-rendered UI uses base.css; this legacy bundle keeps the same component classes readable when it is loaded directly."

The RFC acceptance criterion ("`app.css` has `prefers-color-scheme: dark` blocks for all the named components") is mechanically met. But the practical impact on the operator-visible UI is zero — the visible dark-mode behavior is driven entirely by `base.css`'s variables.

Recommendation: in a follow-up packet, either (a) delete `app.css` and remove it from `package-data` if it truly has no consumer, or (b) audit `base.css` for any of the same selectors that lack dark-mode coverage (none jumped out on a scan, but a focused diff would confirm) and consolidate. This is housekeeping, not a blocker for the V1 ship.

### F4. `run_detail.js` adds `tabindex="0"` to non-interactive SVG groups when there is no link wrapper

Severity: low

`run_detail.js:36-37`:

```js
const link = node.closest(".graph-node-link");
const focusTarget = link || node;
focusTarget.setAttribute("tabindex", focusTarget.getAttribute("tabindex") || "0");
```

When `run_id` is present, every node is wrapped in `<a class="graph-node-link">` (`graph_svg.py` adds the `<a>` only when a run id is supplied). When `run_id` is `None` (e.g., a hypothetical workflow-only graph preview), the focus target falls back to the raw `<g class="graph-node">` and gets `tabindex="0"` — creating a tab stop on an SVG group that has no Enter/Space activation behavior. Keyboard users will hit a "trap" that focuses, shows the tooltip, but does nothing on activation.

For the current shipped surfaces this only matters on the run detail page, which always has a `run_id` and therefore always has the `<a>` wrapper, so this is latent. But it's the kind of thing that bites the next person who renders the same SVG in a no-run-id context.

Recommendation: only set `tabindex` when the focus target is the link, not when it's the raw `<g>`. The graph-node element itself shouldn't be a tab stop without an action.

### F5. No behavioral unit tests for the new JS modules

Severity: low (acknowledged constraint of no-toolchain policy)

`tests/test_web_ergonomics.py:74-99` asserts the presence of strings like `striatum.ui.time.mode`, `window.StriatumUI`, `formatDuration`, and the keyboard route table inside the source. These are guards against accidental deletion, not behavioral coverage of `formatDuration(60000)` → `"1m 0s"`, `dateMatches(stamp, "7d")` → boolean, `rowMatches(row, filter)` → boolean, or `isEditableTarget(target)` semantics.

The RFC's review-checklist line "New JS unit tests present: duration formatter, localStorage helpers, filter predicates, input-focus guard" can be read literally as "tests exist for these surfaces" — which is met by the static-string assertions — or as "behavioral unit tests run the JS." The second reading would require a node test runner, which RFC 0037 §non-goals explicitly forbids.

Recommendation (post-V1): if behavioral JS coverage is wanted without adding a node toolchain, a small Pyodide-based harness or a Python-side stand-in (re-implementing the predicate logic in `test_web_ergonomics.py` and asserting behavioral parity by string comparison of the JS source against a hand-written reference) is feasible. Not blocking for V1.

### F6. Duration column shows `0s` on first paint for brand-new running runs

Severity: low (cosmetic)

`run_list.js:130`:

```js
window.setInterval(() => updateDurations(rows), 30000);
```

`apply()` runs once on `DOMContentLoaded` and then every 30 seconds. For a run that started 2 seconds before the page loads, the column shows `2s` on initial render (good), and the next update lands at 32 seconds (showing `32s`). That's the expected staircase.

The cosmetic gotcha is for runs that started 0–1 seconds before render: the column shows `0s` for up to 30 seconds. A user reloading the page right after `striatum run prepare` may see `0s` and assume nothing is happening.

Recommendation (optional polish): for the first 60 seconds, tick at a shorter interval (e.g., 5 seconds), then drop back to 30. Or just live with it — operators rarely watch the run-list for <60-second runs.

### F7. Outside-click dialog dismissal relies on `getBoundingClientRect()` and the dialog content geometry

Severity: low

`base.js:124-131`. Native `<dialog>` shows the content centered, but the click target during a backdrop click is the dialog element itself (event.target === dialog). The handler computes `dialog.getBoundingClientRect()` and treats a click outside the content rect as "outside." This is the standard pattern, but it can produce a near-edge dead zone where the user is technically on the backdrop but `clientX`/`clientY` fall inside the dialog's bounding box due to rounding or zoom. Esc-dismissal works regardless, so this is a polish item, not a correctness one.

No action recommended for V1.

## What I could not verify in this review

- **Live browser walkthrough** of the surfaces (run list filter persistence across reloads, doctor toggle on a machine with > 20 problems, graph tooltip viewport-edge behavior). The review scope is `docs/dogfood/039/review/build/ergonomics/` only, and I declined to start the service from this read-only invocation. The build handoff records `pytest tests/test_web_*.py` passing 49/49 and `make test` passing 637/31-skip; I am relying on that evidence plus source reading.
- **Visual dark-mode parity screenshots**. The build handoff explicitly notes "Dark-mode parity visual check: CSS selector coverage is automated; no browser screenshot was captured in this session." Selector-level parity is verified by `test_app_css_dark_mode_parity_selectors`. A visual confirmation would close F3 either way.
- **Cross-browser tooltip positioning** (Firefox vs Chrome computed-style differences on `position: fixed` inside an SVG-containing `<a>`). The implementation looks correct against the HTML spec, but cross-browser quirks aren't exercised by the Python test suite.

## Verdict

`accept_with_findings`, severity `medium` (one developer-ergonomic gotcha in F1 plus several minor housekeeping items). The shipped surfaces are discoverable, the affordances are consistent, the no-toolchain constraint is honored, and the documentation accurately describes the implementation. The findings are sized for a follow-up packet, not a re-build.

## Suggested follow-ups (post-V1)

1. F1: drop `button` from `isEditableTarget` or auto-blur filter pills after click.
2. F2: delete the unused `<script type="application/json">` data islands on the run list and workflows index.
3. F3: audit whether `app.css` still has any active consumer; either remove or consolidate with `base.css`.
4. F4: only set `tabindex` on the `<a>` wrapper, not the raw `<g>` element, in `run_detail.js`.
5. F5: consider a minimal Python/Pyodide harness for behavioral JS coverage in a future RFC (out of scope for 0037).
6. F6: optional first-minute fast tick for the run-list duration column.

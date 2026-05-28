# RFC 0037: Web UI Ergonomic Improvements

Status: accepted (V1)
Date: 2026-05-13
Context:
[`RFC 0013`](0013-local-web-ui.md),
[`RFC 0022`](0022-web-ui-redesign.md),
[`RFC 0023`](0023-web-chat-and-browse.md),
[`RFC 0024`](0024-workflow-browser-and-builder.md),
[`docs/DECISION_LOG.md`](../decisions/decision-log.md) (D058, D059, D073, D074, D075),
`src/striatum/service.py`,
`src/striatum/web/static/base.css`,
`src/striatum/web/static/app.css`,
`src/striatum/web/templates/run_list.html`,
`src/striatum/web/templates/run_detail.html`,
`src/striatum/web/templates/doctor.html`,
`src/striatum/web/templates/workflows_index.html`

## Problem

The web UI shipped over RFC 0013 V1 (D059), RFC 0022 V1 (D073, server-
rendered Jinja2 with dark mode), RFC 0023 V1 + V1.5 (chat + view + chat
tools), and RFC 0024 V1+V1.5+V2+V3+V4 (workflow browser + visual builder +
run-now + cancel + pause/resume + per-job cancel/retry) is functionally
complete: every CLI verb that needs a UI surface has one. After a
first-time-operator walkthrough over the tailscale-bridged dev instance,
several ergonomic gaps are visible that prevent the UI from being the
operator's primary surface for steady-state observability and triage.

The gaps are not bugs; they are first-time-user friction points that
multiply across the dozens of artifacts and hundreds of events a single
dogfood produces:

- **Run list is unfiltered, unsearchable, untruncated.** The current
  run-list table shows every run ever recorded, in reverse chronological
  order, with no controls. After 50 runs the table is a scrolling chore.
  Operators want "show me runs from the last 7 days that aren't completed
  yet" or "find runs touching branch `striatum/dogfood-`".
- **Run list omits wall-clock duration.** First-time operators looking at
  a run list can't tell which runs were quick vs slow. Adding a
  `created → completed` duration column makes the list useful for triage.
- **Doctor shows raw problem-by-problem list.** With 30 stale-claim +
  active-session problems on a developer machine that has been used for
  many dogfoods, doctor becomes a noise wall. Operators want
  group-by-kind, and a toggle to hide problems on terminal (completed/
  canceled) runs.
- **Workflows index has 47 entries, no filter.** Same shape as run list:
  every `**/workflow.json` in the repo, no filter, no group-by-status,
  no recent-edit indicator.
- **Timestamps are UTC-only.** All times in the UI are ISO 8601 UTC.
  Local operators have to convert in their head. Adding a localtime
  toggle (persisted in `localStorage`) is a few-line ergonomic win.
- **Graph nodes have no tooltips.** The SVG dependency graph on the run
  detail page is click-navigable but doesn't reveal job state, role, or
  duration on hover. New operators don't realize the graph is
  interactive.
- **No keyboard navigation.** Power users (and accessibility users) have
  no way to navigate the top-level pages without a mouse. Common verbs
  (`g r` for runs, `g w` for workflows, `g c` for chat, `g d` for doctor)
  cost almost nothing to add.
- **`app.css` has no dark-mode parity with `base.css`.** `base.css`
  (677 lines) has `prefers-color-scheme: dark` rules for the global
  surface; `app.css` (261 lines) has zero. App-specific components (job
  list rail, status pills, run grid, posture chips) inherit dark colors
  inconsistently.
- **"Next actions" panel is buried.** On the run detail page, the "next
  actions" list (from the same data source `striatum status` exposes) is
  in a sidebar/footer position rather than prominent. For a triage user
  this should be near the top.
- **No empty-state polish in a few places.** The run list, workflows
  list, and doctor pages don't have a friendly empty state when there
  are zero items; they just show an empty table.

Some of these gaps were noted in earlier RFCs as deferred:

- RFC 0022 V1 deferred inline Markdown rendering on artifact pages (now
  partially landed in RFC 0023 V1.5) and SVG zoom/pan (still deferred).
- RFC 0023 V1.5 noted graph-node click 404 + doctor problem list +
  chat double-render as bundled fixes; the doctor problem list noise is
  still present.
- RFC 0024 V1 deferred drag-and-drop graph editing (still out of scope).

This RFC scopes the V1 ergonomic-improvement slice. UI redesign, hosted
mode, and feature additions are not in scope.

## Goals

- Add filter + search controls to the run list and workflows index.
- Add a duration column to the run list.
- Group + filter problems on the doctor page.
- Add a localtime toggle for timestamps, persisted in `localStorage`.
- Add hover tooltips to graph nodes on the run detail page.
- Add keyboard navigation shortcuts for top-level pages.
- Bring `app.css` to dark-mode parity with `base.css`.
- Promote the "next actions" panel to a prominent position on the run
  detail page.
- Add empty-state copy + illustration to the run list, workflows index,
  and doctor pages.
- Keep the changes minimal-DOM-impact: no new runtime dependencies,
  no client-side framework, no SPA conversion. Server-rendered Jinja2 +
  vanilla JS only (consistent with RFC 0022 V1 and D073).

## Non-Goals

- New feature surface. This RFC is ergonomic polish, not new
  capabilities.
- Visual redesign. The RFC 0022 V1 palette + spacing scale + system fonts
  stay.
- Mobile-first responsive overhaul. The UI is intentionally desktop-first
  per D073; a narrow-viewport pass is a separate RFC.
- Hosted-mode UX. Single-user, owner-only, loopback-or-tailscale only,
  per D058 + D083.
- New runtime dependencies. No client-side framework, no CSS framework,
  no node toolchain.
- Workflow visual editor enhancements (RFC 0024 territory).
- Drag-and-drop graph editing (RFC 0024 V2 deferred).
- SVG zoom/pan (RFC 0022 V1.5 deferred).

## External Prior Art

The shape of these improvements borrows from operator-facing dashboards
in the Argo Workflows UI, GitHub Actions runs, GitLab CI pipelines, and
the Vercel deployments dashboard. The useful patterns are:

- **Argo Workflows** — group runs by status; collapsible problem lists;
  click-navigable graphs with hover tooltips. The non-goal for Striatum
  is Argo's full workflow-template UX (RFC 0024 covers our equivalent).
- **GitHub Actions** — date-range filter on workflow runs ("last 7
  days", "last 30 days"); branch-substring filter; duration column. The
  non-goal is GitHub's hosted-mode permissions UX.
- **GitLab CI Pipelines** — filter pills above the table; rich status
  pills with state colors. We already have status pills (D073); we add
  the filter row.
- **Vercel deployments** — localtime + relative-time toggles ("2 hours
  ago" vs ISO timestamp); search-as-you-type filter input. We borrow the
  toggle; we use plain JS for the filter without a framework.

## Proposal

### 1. Run list improvements

`src/striatum/web/templates/run_list.html` + `src/striatum/web/static/run_list.js` (new, ~80 lines).

- Add a filter row above the table:
  - Free-text search box (matches run_id, branch, workflow_id substrings)
  - State filter pills (`all` / `running` / `completed` / `canceled` /
    `failed` / `paused`)
  - Date-range select (`last 24h` / `last 7d` / `last 30d` / `all`)
- Add a `Duration` column: `created → completed` wall-clock for terminal
  runs; live-updating for running runs (`Xm Ys ago started`).
- Server returns all runs (existing behavior); JS filters client-side
  against a JSON data island in the page (no new endpoint).
- Filter state is persisted in `localStorage`.

### 2. Workflows index improvements

`src/striatum/web/templates/workflows_index.html` + `src/striatum/web/static/workflows_index.js` (new, ~50 lines).

- Add a filter row above the table:
  - Free-text path / workflow_id search
  - Status filter pills (`all` / `valid` / `invalid`)
- Add a `Last modified` column (file mtime, with localtime toggle).
- Filter state is persisted in `localStorage`.

### 3. Doctor page improvements

`src/striatum/web/templates/doctor.html` + small JS extension (~40 lines).

- Group problems by `kind` with collapsible sections (default-collapsed
  for sections with > 5 problems).
- Add a toggle "hide problems on terminal runs" (default ON when the
  count would otherwise be > 20 problems, default OFF otherwise).
- Add per-problem-kind doc links pointing to `docs/HOW_TO_HUMAN.md`
  anchors for known kinds (stale claim → recovery section; active
  session on terminal run → session close section).
- Toggle state persists in `localStorage`.

### 4. Localtime toggle

`src/striatum/web/static/base.js` (new, ~30 lines) + template wiring.

- Add a small toggle in the site header: `UTC` ↔ `Local`.
- All `<time datetime="...">` elements get rewritten on toggle by a
  small vanilla-JS handler.
- Toggle state persisted in `localStorage`.
- Default: UTC (preserves current behavior; toggle is opt-in).

### 5. Graph node tooltips

`src/striatum/web/static/run_detail.js` (small extension) or inline JS in the run_detail template (~30 lines).

- On hover over an SVG `<g.node>` element, show a tooltip with: job
  name, role, state, duration (if available).
- Use the existing data attributes the graph already embeds; no new
  server data needed.
- Tooltip is positioned with `position: fixed; pointer-events: none;`
  to avoid breaking the click-navigate.

### 6. Keyboard shortcuts

`src/striatum/web/static/base.js` (extension) (~30 lines).

- `g r` → `/` (runs)
- `g w` → `/workflows` (workflows)
- `g c` → `/chat` (chat)
- `g d` → `/doctor` (doctor)
- `?` → keyboard-shortcut help overlay
- Disabled when focus is on an input / textarea / contenteditable
  element.
- Help overlay is a `<dialog>` element listing the available shortcuts.

### 7. `app.css` dark-mode parity

`src/striatum/web/static/app.css` extension (~80 lines).

- Add `@media (prefers-color-scheme: dark)` blocks for: `.job-list`,
  `.job-link`, `.status-pill`, `.posture-chip`, `.run-grid`,
  `.run-jobs-rail`, `.run-meta`, `.run-events`, `.workflow-graph`,
  `.workflow-edit-form`.
- Audit each app-specific class against the `base.css` dark palette
  variables (`--color-bg`, `--color-fg`, `--color-muted`, etc.) and use
  them instead of literal colors where possible.

### 8. Promote "next actions" on run detail

`src/striatum/web/templates/run_detail.html` reorder.

- Move the "Next actions" block from the bottom-right sidebar to a
  prominent banner just below the run header (only when there are
  actions). For terminal runs, the banner is hidden.

### 9. Empty-state copy

`src/striatum/web/templates/run_list.html`, `workflows_index.html`, `doctor.html` extensions.

- Run list: "No runs yet. Run `striatum run prepare` to create your
  first run; see `docs/HOW_TO_HUMAN.md`." + link.
- Workflows index: "No workflow.json files found. Run `striatum workflow
  generate` to create one; see `docs/HOW_TO_HUMAN.md`." + link.
- Doctor: "0 problems found. Nothing to triage." (when count is zero —
  rare, but worth the friendly state).

### 10. No changes to

- The server route table.
- The runtime dependency tree.
- The chat surface (RFC 0023 V1.5 covered).
- The workflow visual builder (RFC 0024 V1.5).
- The CSP / security posture.
- The CLI surface.
- The API surface (`/v1/*`).
- The MCP surface.
- The audit chain.

## Acceptance Criteria

- Run list page has a working free-text + state + date-range filter,
  with state persisted in `localStorage`.
- Run list shows a duration column (terminal: `Xm Ys`; running: relative).
- Workflows index has a working path / workflow_id search + status filter.
- Workflows index shows a `Last modified` column.
- Doctor page groups problems by kind with collapsible sections.
- Doctor page has a "hide terminal-run problems" toggle.
- Localtime toggle in the header switches every `<time>` element on the
  page; preference persists.
- Graph nodes on run detail show hover tooltips with job state + role +
  duration.
- Keyboard shortcuts `g r` / `g w` / `g c` / `g d` work when focus is
  not on an input; `?` opens help.
- `app.css` has `prefers-color-scheme: dark` blocks for all the named
  components.
- Run detail page surfaces "Next actions" prominently for non-terminal
  runs.
- Empty-state copy added to run list, workflows index, doctor.
- No new runtime dependencies.
- No regressions to existing functionality (run-now, cancel, pause/
  resume, per-job cancel/retry, workflow validate/generate/edit, chat).
- `make test` + `make smoke` + existing UI snapshot tests pass.
- Doc-link tests pass.
- Changelog + README + UBIQUITOUS_LANGUAGE + HOW_TO_HUMAN updates if
  applicable.

## Implementation Plan

### Step 1. Localtime toggle + base.js scaffold

Add `src/striatum/web/static/base.js` with the localtime toggle and
keyboard-shortcut framework. Add a `<dialog>` for the help overlay.
Wire to `base.html`. This is the smallest, highest-leverage change.

### Step 2. Run list filters + duration column

Add `run_list.js` + filter row template. Add the duration column. Add
the empty-state copy.

### Step 3. Workflows index filters

Add `workflows_index.js` + filter row template + Last-modified column +
empty state.

### Step 4. Doctor grouping + filter

Group problems by kind in the template + collapsible JS + the "hide
terminal-run problems" toggle + empty state.

### Step 5. Graph tooltips

Add hover tooltips to run_detail SVG nodes via the existing template's
inline JS or a small new JS file.

### Step 6. Next-actions promotion

Reorder `run_detail.html` to surface next-actions prominently for
non-terminal runs.

### Step 7. app.css dark-mode parity

Audit each component class, add `prefers-color-scheme: dark` blocks.

### Step 8. Docs + tests

Update HOW_TO_HUMAN with screenshots of the new filter UI + keyboard
shortcuts + localtime toggle. Update CHANGELOG. Run UI snapshot tests
+ doc-link tests + `make test/smoke`.

## Open Questions

- Should the run list filter pills include a `recently-failed` or
  `needs-attention` quick filter (combining `failed` + `paused` +
  `human_checkpoint`)? Recommendation: yes for V1.5; V1 ships the
  five basic state filters only.
- Should the localtime toggle default to `Local` on first visit?
  Argument for: most users are not in UTC. Argument against: the rest
  of the runner (CLI, exports, audit chain) uses UTC, and the toggle
  is opt-in. Recommendation: keep default UTC; the toggle is the
  affordance, and we don't change the cli/export behavior.
- Should the "next actions" banner be sticky-positioned at the top of
  the page when scrolling? Recommendation: no for V1; sticky positioning
  has accessibility and small-viewport pitfalls. Banner appears in flow
  just below the run header.
- Should keyboard shortcuts be configurable? Recommendation: no for
  V1; the four `g r` / `g w` / `g c` / `g d` mappings mirror the top-
  nav order and are mnemonic. Re-mapping is a future RFC if requested.
- Should the doctor page expose a one-click "resolve all stale claims
  on terminal runs" mutation? Recommendation: tempting, but the
  recovery vocabulary (RFC 0020) already has `recovery auto` and the
  pattern of UI-mutates-state requires more guardrails. Leave that for
  a separate UX RFC.
- Should the filter state for run list / workflows / doctor be query-
  string-encoded (shareable URLs) instead of localStorage? Recommendation:
  yes, but V1.5. V1 ships localStorage for simplicity.

## Domain Modeling

This RFC adds no new domain concepts. The run, job, blocker, workflow,
doctor problem aggregates from D006 / D058 / D059 / D073 stay as-is.
The web UI is purely a read surface over them (with the existing
mutation buttons unchanged); this RFC adds presentation polish.

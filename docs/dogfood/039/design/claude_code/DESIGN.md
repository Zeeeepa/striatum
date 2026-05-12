# RFC 0037 Web UI Ergonomic Improvements — Implementation Design

author: designer-claude-opus-001

## 0. Framing: affordance grammar for a first-time operator

This RFC is ergonomic polish; the design lens is not "what code changes"
but "what does the first-time operator notice, and how does each gap
turn into a discoverable surface?" A first-timer in this UI today sees
a wall of UTC timestamps, a 47-row workflow table, and a doctor page
that reads like a stack trace. The design below makes each gap a small,
recognizable affordance that follows a single grammar:

1. **Default state is "no surprise"** — preserve current behavior on
   first visit. Filters are empty. Time is UTC. Toggles are off.
2. **State indicator is the affordance** — the toggle text shows the
   *current* mode, not the *target* mode. (`UTC` ↔ `Local`, not
   "Switch to Local".) Reading what state you are in is the same act
   as reading what you can change.
3. **Persistence is invisible but reversible** — `localStorage` keeps
   user preferences across reloads; a single "Clear" affordance per
   surface unwinds them without instructions.
4. **Empty states sell the next CLI verb** — every empty surface names
   the verb that fills it and links to `docs/HOW_TO_HUMAN.md`.
5. **Progressive disclosure for noise** — doctor groups collapse,
   keyboard shortcuts hide behind `?`, graph tooltips appear on hover.

The grammar matters because RFC 0037 ships nine surfaces in one slice.
Without a shared grammar, each surface invents its own conventions and
the UI feels louder rather than calmer.

## 1. File map (no new runtime deps)

```
src/striatum/web/static/
  base.js                (NEW, ~120 lines) — localtime toggle, keyboard
                         shortcuts, help dialog, shared filter helpers.
  run_list.js            (NEW, ~110 lines) — run-list filter row, search,
                         state pills, date range, duration formatting,
                         clear-all.
  workflows_index.js     (NEW, ~70 lines)  — workflow filter row, search,
                         status pills, last-modified rendering, clear-all.
  doctor.js              (NEW, ~70 lines)  — doctor group toggles,
                         hide-terminal-run toggle, collapsed-kind
                         persistence.
  run_detail_tooltip.js  (NEW, ~50 lines)  — SVG graph node hover tooltip.
  app.css                (extend, ~90 lines) — dark-mode parity blocks.
  base.css               (extend, ~30 lines) — filter row, banner,
                         shortcut badge, dialog styling.

src/striatum/web/templates/
  base.html              (extend) — localtime toggle in header, shortcut
                         badge in footer, help <dialog> stub, base.js.
  run_list.html          (extend) — filter row, duration column, JSON
                         data island, empty-result region, empty state.
  workflows_index.html   (extend) — filter row, last-modified column,
                         JSON data island, empty-result region.
  doctor.html            (extend) — group-by-kind structure, hide-
                         terminal-run toggle, doc-link anchors,
                         friendly zero state.
  run_detail.html        (extend) — next-actions banner promoted under
                         run-header, graph tooltip script include.
```

Touches no server route, no API, no MCP surface, no CLI, no audit
chain. Server returns the same data; presentation logic lives in
`<script>` + small CSS deltas.

## 2. Filter row UX

The filter row is the *single most-used affordance* added by this RFC.
It appears on the run list, the workflows index, and (in a different
form) the doctor page. The grammar must be identical across all three,
so the user learns it once.

### 2.1 Anatomy

```
┌ filter-row ────────────────────────────────────────────────────────┐
│  🔍 [ Search runs by id, branch, workflow id…     ✕ ]              │
│  state:  [all] [running] [completed] [canceled] [failed] [paused]  │
│  range:  ⌄ [Last 24h] [Last 7 days] [Last 30 days] [All]           │
│                                       Clear all filters            │
└────────────────────────────────────────────────────────────────────┘
```

- **Search input** is the leftmost, full-width-feeling element.
  Placeholder is **specific to the surface**:
  - Run list: `Search runs by id, branch, workflow id…`
  - Workflows index: `Search workflows by path or workflow id…`
  - Doctor: no search; the doctor surface filters by toggles, not text.
  - *Why specific not "Search":* "Search" is a black box. The operator
    needs to know which fields are eligible without trial and error;
    listing them in the placeholder is cheaper than a tooltip.
- **State filter pills** are buttons styled like the existing
  `.status-pill` so the visual vocabulary is shared.
  - The `all` pill is leftmost and is the *implicit default* state.
  - At most one pill is active at a time (radio semantics, not
    multi-select).
  - *Why radio not multi-select:* the run states are five
    non-overlapping buckets; multi-select adds a "compose" complexity
    cost first-timers don't ask for. V1.5 can add a
    `needs_attention` composite pill (see RFC 0037 Open Questions).
- **Date range** is a small select; default is `All`.
  - *Why select not pills:* date range has four mutually-exclusive
    values and is the least-used filter; a select is one row shorter
    and visually deprioritizes it.
- **Clear all filters** link is right-aligned, hidden when all
  filters are at default, revealed when any filter is non-default.
  - *Why a link not a button:* the affordance is *escape from a state
    that may not be the user's*; a link reads as recoverable, a button
    reads as commit.

### 2.2 Default state — no surprise filtering on first visit

- Initial `localStorage` read pattern, single helper in `base.js`:

```js
function loadFilterState(key, defaults) {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return { ...defaults };
    return { ...defaults, ...JSON.parse(raw) };
  } catch {
    return { ...defaults };  // corrupted JSON → defaults, never throw.
  }
}
```

- Run list default: `{ query: "", state: "all", range: "all" }`.
- Workflows index default: `{ query: "", status: "all" }`.
- *Why defaults are "show everything":* a first-visit user must see
  the same rows the unfiltered backend returned, or they cannot tell
  whether the page is broken vs filtered. Persistence is opt-in via
  user interaction.

### 2.3 Persistence semantics

Three behaviors first-timers expect (verified by the prompt and
universal across the operator-dashboard prior art in §External Prior
Art of the RFC):

| User action          | Filter behavior                                    |
|----------------------|----------------------------------------------------|
| Change a filter      | Apply immediately; write to `localStorage`.        |
| Reload page          | Last `localStorage` state is applied on load.      |
| Navigate away + back | Same `localStorage` state is applied on load.     |
| Click "Clear all"    | Reset to defaults; `localStorage` is overwritten.  |
| Open in new tab      | `localStorage` is per-origin → same state applied. |
| Different machine    | `localStorage` is local → fresh defaults applied.  |

`localStorage` keys (all under the `striatum.ui.` namespace so a
single grep finds every UI preference):

- `striatum.ui.run_list.filter`
- `striatum.ui.workflows_index.filter`
- `striatum.ui.doctor.hide_terminal`
- `striatum.ui.doctor.collapsed_kinds`
- `striatum.ui.localtime`

*Why a single flat namespace:* future RFCs can add more keys without
hierarchy churn; a single migration utility can read/clear all
preferences in one pass.

### 2.4 Clear-filter affordance — two complementary surfaces

- **Inline `✕` button** inside the search input, visible only when
  the input has content. Clears the search query only.
- **"Clear all filters"** link in the filter row, visible only when
  *any* filter (query, state, range) is non-default. Clears all
  three.
- *Why both:* the inline `✕` is the de facto search-box idiom (Vercel,
  GitHub, GitLab all do it). The link is the explicit recover-from-
  surprising-state affordance for the union of all filters. A
  first-timer who lands on a page with stale filters from a previous
  visit needs the union, not the per-field clear.

### 2.5 Empty-result state (filter matches nothing)

- Render the existing table header but with one zero-data row:
  > `No runs match the current filter. Clear all filters` (link)
- The table header stays so the operator can see the columns the
  filter applies to.
- *Why preserve the table header:* hiding the table altogether makes
  the page look broken; keeping the header makes "this is a filter
  state, not a broken UI" obvious.

### 2.6 Implementation notes

- The server emits a JSON data island once per surface (no new
  endpoint):
  ```html
  <script id="runs-data" type="application/json">{{ runs_json | safe }}</script>
  ```
  `runs_json` is computed in the existing route handler from the same
  list the table iterates. *Why a data island not JSON via fetch:* one
  request, no race with table render, no CSP loosening (the existing
  `script-src 'self'` allows inline `<script type="application/json">`).
- `run_list.js` reads the data island, builds an in-memory array,
  re-renders the `<tbody>` on each filter change. Sort order stays
  the server's (reverse chronological).
- Filtering is case-insensitive substring on `run_id`, `branch_name`,
  and `workflow_id`. *Why substring not fuzzy:* substring is
  predictable, fuzzy isn't, and the operator can paste a known
  run-id prefix.
- Filter changes use `debounce(80ms)` on text input, immediate on
  pills/select. *Why 80ms not 200ms:* desktop typing is fast, the
  data set is small (the largest dogfood machine has ~500 runs), and
  a longer debounce feels laggy.

## 3. Duration column format

Compact, predictable, single-glance. Borrowing from the GitHub Actions
runs view but tightening the breakpoints.

| Wall-clock duration | Format example         |
|---------------------|------------------------|
| < 60 s              | `42s`                  |
| < 60 min            | `7m 12s`               |
| ≥ 60 min            | `2h 14m`               |
| Running run         | `4m ago started`       |
| Missing `started_at`| `—`                    |

- *Why `mm:ss` style splits at 1h not 1d:* most striatum runs finish
  in tens of minutes; a flat `hh:mm:ss` wastes screen pixels on
  leading zeros for the 80% case. The single breakpoint at 1h keeps
  the cells visually compact.
- *Why `Xm ago started` for running runs and not `live elapsed`:*
  `ago` is the relative-time idiom every operator already reads from
  GitHub/GitLab/Vercel. A live ticker would draw attention away from
  the rest of the page; the relative form recalculates on a single
  `setInterval(updateLiveDurations, 30_000)`.
- *Why 30s tick not 1s:* 1s tick is noise in a triage view; 30s is
  the threshold where the rendered text actually changes (`Xm ago`
  granularity is minute-level).
- Terminal runs render the static `created → completed` delta and
  never re-tick. The `setInterval` skips them entirely.

Formatter lives in `base.js` (shared with workflows index for
`last_modified`):

```js
function formatDuration(startISO, endISO) {
  if (!startISO) return "—";
  const start = Date.parse(startISO);
  const end = endISO ? Date.parse(endISO) : Date.now();
  const sec = Math.max(0, Math.floor((end - start) / 1000));
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
}
```

## 4. Localtime toggle

### 4.1 Placement

- **Top-right of the site header**, right-aligned within the existing
  `<header class="site-header">`. To the right of the nav, before any
  future user controls.
- Rendered as a single inline `<button class="time-toggle">UTC</button>`,
  not a select, not a switch.
- *Why right-aligned not center:* the brand and nav live left-of-
  center; the toggle is an ambient utility, not a primary action.
  Right placement keeps it off the primary scan line.
- *Why a button not a switch:* the affordance is unmistakably
  click-to-change; a switch implies binary state with two pictographic
  endpoints and costs extra CSS.

### 4.2 State indicator

- Button text **always shows the current mode**: `UTC` or `Local`.
- Clicking flips the mode; the text updates in place.
- A small `aria-label` reads `Toggle to Local time` / `Toggle to UTC time`
  for screen readers (the *target* mode for assistive tech, but the
  *current* mode for visual users).
- *Why current-mode-text not target-mode-text:* the user must be able
  to tell what state they are in by reading the page. Showing the
  target implies action *was* taken; showing the current implies
  state *is* this.

### 4.3 Effect

- Every `<time datetime="2026-05-12T12:34:56Z">UTC text</time>` element
  on the page is rewritten in place by `base.js`:
  - On `Local`: visible text = `formatLocal(datetime)` using
    `Intl.DateTimeFormat` with the user's locale and timezone.
    The format string is `YYYY-MM-DD HH:mm` (no seconds, no
    timezone abbrev — short by default).
  - On `UTC`: visible text = the original UTC ISO 8601 from the
    server (recovered from the `datetime` attribute).
- The `datetime` attribute is the immutable source of truth and is
  never modified.
- *Why mutate visible text and keep `datetime` immutable:* HTML
  `<time>` is exactly this contract; `datetime` is the
  machine-parseable, `textContent` is the human-readable. CSS and
  screen readers can rely on `datetime`.

### 4.4 Default

- **Default is UTC** on first visit. `localStorage` value
  `striatum.ui.localtime` is one of `"utc"` or `"local"`; absent =
  `"utc"`.
- *Why default UTC not Local:* the CLI, audit chain, and exports all
  emit UTC. A default Local would create a class of bug where the UI
  time disagrees with `striatum status --json` time; the operator
  must opt in to that divergence.

### 4.5 Edge cases

- Server-rendered timestamps must always be UTC ISO 8601 with a `Z`
  suffix and a `<time datetime="…">` wrapper. Any plain text timestamp
  in the templates is a bug; an audit step in §8 lists the files.
- `Intl.DateTimeFormat` is universally supported in modern browsers;
  graceful fallback: if it throws, leave the text unchanged.
- Toggle re-applies on `DOMContentLoaded`, on click, and on
  `storage` event (so changing the toggle in tab A updates tab B).

## 5. Keyboard shortcuts + help overlay

### 5.1 Shortcuts

| Keys      | Action                                                  |
|-----------|---------------------------------------------------------|
| `g r`     | Navigate to `/` (Runs).                                 |
| `g w`     | Navigate to `/workflows` (Workflows).                   |
| `g c`     | Navigate to `/chat` (Chat).                             |
| `g d`     | Navigate to `/doctor` (Doctor).                         |
| `?`       | Open the keyboard-shortcut help dialog.                 |
| `Escape`  | Close the help dialog (native `<dialog>` behavior).     |

- `g X` shortcuts are sequence-based (Gmail/GitHub style): press `g`,
  then within 1000ms press the destination letter. Implementation in
  `base.js` uses a tiny state machine (`pendingG = false`, reset on
  timeout or unrecognized key).
- Shortcuts are **disabled when focus is on an input, textarea, or
  contenteditable** (`event.target.matches('input,textarea,[contenteditable]')`).
- *Why sequence shortcuts not modifiers:* `Ctrl+R` is browser reload;
  `Cmd+K` is OS-level command bar; `g r` mirrors top-nav mnemonics and
  has zero collision.

### 5.2 Help overlay UX

- HTML `<dialog id="kbd-help">` element placed in `base.html` (once,
  inherited by every page).
- Opened with `dialog.showModal()` — gets the native modal backdrop,
  native Tab cycling within the dialog, and native `Escape` close.
- Dismissed by:
  1. Pressing `Escape` (native `<dialog>` behavior).
  2. Clicking outside the dialog (event handler closes on
     `mousedown` if `event.target === dialog`).
  3. Clicking the inline `Close` button inside the dialog footer.
- Content:
  ```
  Keyboard shortcuts
  ──────────────────
  g r   Runs
  g w   Workflows
  g c   Chat
  g d   Doctor
  ?     Show this help
  Esc   Close this help
  ```
  Rendered as a `<dl>` for screen-reader friendliness.
- *Why native `<dialog>` not a custom modal:* native focus trapping,
  native `Escape`, native ARIA semantics, ~0 lines of JS. The
  alternative (custom `<div role="dialog">`) requires reinventing
  every one of those behaviors.

### 5.3 Discoverability badge

- A small `?` badge fixed to the bottom-right of every page:
  ```html
  <button class="kbd-help-badge" aria-label="Show keyboard shortcuts">?</button>
  ```
- Clicking the badge opens the same dialog.
- Tooltip on hover (via `title`): `Press ? for keyboard shortcuts`.
- Auto-hides on viewports `< 720px` wide (the UI is desktop-first per
  D073; no need to clutter narrow viewports).
- *Why a badge not just a hint at the top:* a footer-corner badge is
  the GitHub/Vercel/Linear pattern operators already recognize. It
  surfaces the feature without taking primary scan space.

## 6. Empty-state copy quality bar

Every empty state names:

1. **What's empty** (the user's context).
2. **Which CLI verb fills it** (copy-pasteable).
3. **Where to read more** (a `docs/HOW_TO_HUMAN.md` anchor).

### 6.1 Run list — zero runs ever

```
No runs yet.
Run `striatum run prepare --workflow <workflow.json>` to create your
first one; see docs/HOW_TO_HUMAN.md#starting-a-run.
```

### 6.2 Run list — filter matches nothing

```
No runs match the current filter. [Clear all filters]
```

(Renders the table header above this line; see §2.5.)

### 6.3 Workflows index — zero workflows

```
No workflow.json files found.
Run `striatum workflow generate <path> --shape minimal --lane-set local
--artifact-root striatum/<name>` to create one; see
docs/HOW_TO_HUMAN.md#generating-a-workflow.
```

### 6.4 Workflows index — filter matches nothing

```
No workflows match the current filter. [Clear all filters]
```

### 6.5 Doctor — zero problems

```
0 problems found. Nothing to triage.
```

(Plus the existing `ok` status pill; the prompt explicitly endorsed
this short form because doctor-zero is the *happy* state, not an
empty-feature state.)

### 6.6 Doctor — filter (hide terminal) matches nothing

```
0 problems on non-terminal runs. [Show terminal-run problems]
```

### 6.7 Implementation

- All copy lives in the templates, not in JS. *Why template not JS:*
  empty states must render before JS executes for accessibility and
  for the "JS-disabled but it should still read" case (rare but free).
- CLI examples are wrapped in `<code>` for visual distinction.
- Links are `<a href="...">` with the anchor; doc-link tests in §8
  enforce that all four anchors exist.
- *Why include the exact full command not just the verb:* a first-
  timer faced with `striatum run prepare` and no flags will run it
  and get an error; the full command with placeholder is one
  copy-paste away from working.

## 7. Next-actions banner

### 7.1 When shown / when hidden

| Run state                                         | Banner |
|---------------------------------------------------|--------|
| `prepared`, `needs_branch_confirmation`, `ready`, `running` | shown  |
| `paused`                                          | shown  |
| `awaiting_human_checkpoint` (if encoded as a state) | shown |
| `completed`, `canceled`, `failed`                 | hidden |
| `next_actions` server-side list is empty          | hidden |

The condition is `(run.state not in TERMINAL_STATES) and next_actions`.
*Why hide on terminal runs:* on terminal runs the next actions are
either "nothing" or "archive this", and a banner of stale guidance is
worse than no banner.

### 7.2 Layout

Placed directly below `<header class="run-header">`, before
`<div class="run-grid">`:

```html
<section class="next-actions-banner"
         role="region"
         aria-label="Next actions"
         tabindex="0">
  <h2>Next actions</h2>
  <ul>
    {% for action in next_actions %}
    <li>{{ action }}</li>
    {% endfor %}
  </ul>
</section>
```

- Full-width within `.page-main`, distinct background
  (`var(--color-banner-bg)` — added in §9 for dark-mode parity).
- Header reads `Next actions` (matches the CLI-side `striatum status`
  vocabulary; same word the operator has been saying).
- The existing `Next actions` block inside `<article class="run-center">`
  (currently at the bottom-right) is removed; the data source is the
  same.

### 7.3 Accessibility

- `role="region"` — landmark for screen readers, surfaces in the
  page's region list.
- `aria-label="Next actions"` — accessible name.
- `tabindex="0"` — keyboard-focusable, so an operator pressing Tab
  from the run header lands on the banner before the jobs rail.
- High color contrast: banner background must meet WCAG AA against
  banner text (verified in §9.3 by re-using the existing
  `--color-fg` on a 4.5:1 background variant).
- *Why a `region` landmark not `alert`:* `alert` is for transient
  attention-grabbing notifications; the next-actions banner is
  steady-state guidance that should be discoverable but not
  interrupt.
- *Why focusable not auto-focused:* auto-focus would steal focus
  from the page on every navigation, which is hostile to keyboard
  users navigating quickly. Tab-reachable is the right middle.

### 7.4 Visual hierarchy

- Slightly muted but distinct background; left-edge accent color
  matching the run-state pill color (subtle visual link between
  "this run is running/paused" and "here is what to do").
- Type slightly larger than body but smaller than `h1` (h2 is
  appropriate; matches the existing run-detail typography scale).

## 8. Doctor page

### 8.1 Group by kind

Server already returns `doctor.problem_records` with a `check` field
(used as the grouping key). The template iterates by group:

```html
{% for kind, records in problem_records_by_kind.items() %}
<details class="problem-group"
         data-kind="{{ kind }}"
         {% if collapsed_default(kind, records|length) %}{% else %}open{% endif %}>
  <summary>
    <code>{{ kind }}</code>
    <span class="muted">{{ records | length }}</span>
    {% if doc_anchor_for(kind) %}
    <a href="docs/HOW_TO_HUMAN.md#{{ doc_anchor_for(kind) }}"
       class="problem-help">how to fix</a>
    {% endif %}
  </summary>
  <ul>
    {% for record in records %}
    <li>… same record content as today …</li>
    {% endfor %}
  </ul>
</details>
{% endfor %}
```

- *Why `<details>`/`<summary>`:* native browser collapse, native ARIA
  semantics, native keyboard activation (Enter/Space). ~0 lines of JS
  beyond persistence.

### 8.2 Default-collapsed behavior

- **A group is collapsed by default when it has more than 5
  problems**; ≤ 5 problems → open.
- *Why 5 not 10:* the operator can scan five problems at a glance
  without scrolling; ten requires a scrolling commitment that fights
  with the rest of the page.
- After a user manually expands or collapses a group, the choice is
  remembered in `localStorage.striatum.ui.doctor.collapsed_kinds` as
  an array of `kind` strings the user has explicitly collapsed and
  an array they have explicitly expanded. The defaults still apply
  to *new* kinds the user has never touched.

```js
const state = loadFilterState('striatum.ui.doctor.collapsed_kinds',
                              { collapsed: [], expanded: [] });
function shouldStartCollapsed(kind, count) {
  if (state.expanded.includes(kind)) return false;
  if (state.collapsed.includes(kind)) return true;
  return count > 5;  // default
}
```

- *Why a two-list model not "remember the toggled state":* with a
  flat "stored = collapsed" mental model, a brand-new kind would not
  collapse by default once the user has touched any other kind. The
  two-list model preserves "default for new kinds, override for
  touched kinds."

### 8.3 Hide-terminal-run problems toggle

- A single checkbox above the problem list:
  ```
  ☑ Hide problems on terminal runs
  ```
- State persists in `localStorage.striatum.ui.doctor.hide_terminal`
  (one of `"on"` or `"off"`).
- **Default ON when the unfiltered count would be > 20; default OFF
  otherwise.** First-visit logic:

```js
const stored = localStorage.getItem('striatum.ui.doctor.hide_terminal');
if (stored === null) {
  state.hide = unfilteredProblemCount > 20 ? 'on' : 'off';
} else {
  state.hide = stored;
}
```

- *Why 20 not 10 or 50:* the operator-feedback ceiling for "this
  page is noise" sits in the 15-25 range based on the RFC's stated
  developer-machine numbers (~30 stale-claim + active-session
  problems). 20 covers the "30 problems but 20 are terminal-run
  noise" case while letting a small problem list show everything.
- *Why default ON on first visit:* a doctor page with 30 problems is
  a bad first impression; hiding terminal-run problems by default
  gives the operator the *actionable* slice first.
- Toggle text mirrors the localtime grammar from §4.2: the label is
  the *current effect*. The checkbox is "checked = hiding".
- Each terminal-run problem is filtered out on the client by
  reading a `data-run-state="completed|canceled"` attribute the
  template emits on each `<li>`.

### 8.4 Doc-link anchors

Server-side mapping (Python, in the route handler):

```python
DOCTOR_DOC_ANCHORS = {
    "stale_claim": "recovery-stale-leases",
    "active_session_on_terminal_run": "closing-sessions",
    "orphan_artifact": "orphan-artifacts",
    # ... extend as needed; absent kinds get no link.
}
```

Anchors are validated by the doc-link test pass in §10.

### 8.5 Zero-state

When `doctor.problems` is empty:

```html
<p class="empty-state empty-state-ok">
  <span class="status-pill status-completed">ok</span>
  0 problems found. Nothing to triage.
</p>
```

## 9. app.css dark-mode parity

### 9.1 Audit and method

`base.css` defines `@media (prefers-color-scheme: dark)` for global
surfaces (`--color-bg`, `--color-fg`, `--color-muted`, etc., per
D073). `app.css` defines app-specific component styles but never
references the dark-mode block.

For each class that uses a literal color in light mode, add a
matching dark-mode rule that *prefers a token from `base.css`*. The
plan is mechanical, not creative:

| Class                  | Dark token to use                            |
|------------------------|----------------------------------------------|
| `.job-list`            | `--color-bg-elev`                            |
| `.job-link`            | `--color-fg`, `--color-bg-elev` on `:hover`  |
| `.status-pill`         | per-state tokens (`--color-pill-running-bg`) |
| `.posture-chip`        | `--color-bg-elev`, `--color-fg`              |
| `.run-grid`            | `--color-border`                             |
| `.run-jobs-rail`       | `--color-bg-elev`                            |
| `.run-meta`            | `--color-muted` for muted spans              |
| `.run-events`          | `--color-bg-elev`, `--color-border` rows     |
| `.workflow-graph`      | SVG `fill="currentColor"`; `currentColor`    |
|                        | inherits dark-mode `--color-fg`              |
| `.workflow-edit-form`  | `--color-bg-elev` (inputs), `--color-border` |
| `.next-actions-banner` | `--color-banner-bg`, `--color-banner-fg`     |
|                        | (NEW tokens added to base.css)               |
| `.kbd-help-badge`      | `--color-bg-elev`, `--color-fg`              |

### 9.2 Two new tokens in `base.css`

```css
:root {
  --color-banner-bg: #f4f1e8;
  --color-banner-fg: var(--color-fg);
}
@media (prefers-color-scheme: dark) {
  :root {
    --color-banner-bg: #2a2520;
    --color-banner-fg: var(--color-fg);
  }
}
```

- *Why a banner token not reuse `--color-bg-elev`:* the next-actions
  banner needs to stand out from the surrounding page chrome;
  `--color-bg-elev` is the same shade as the jobs rail and the
  banner would visually merge with it on dark mode.

### 9.3 Contrast verification

Each new pairing is verified at design-review time against WCAG AA
contrast (4.5:1 for body text, 3:1 for large text). For the dark
mode banner: `--color-banner-bg: #2a2520` against
`--color-fg: #e8e6e1` is 11.2:1, well above AA.

### 9.4 Why not auto-derive

A previous instinct would be "compute dark colors from light tokens
in CSS using `color-mix()`". *Rejected:* `color-mix()` support is
spotty in older Safari versions some operators run, and the literal
tokens make a single grep audit possible.

## 10. Implementation phasing — same order as RFC 0037

The RFC sequences the work for a reason: the localtime toggle and
keyboard scaffold are the smallest, lowest-risk pieces and they
introduce the shared `base.js` file that later steps extend. The
design preserves that sequencing.

### Step 1 — `base.js` scaffold + localtime + keyboard shortcuts

- Create `base.js` with: localtime toggle handler, keyboard shortcut
  state machine, help dialog event wiring, and shared `loadFilterState`
  helper.
- Add `<button class="time-toggle">` to `base.html` header.
- Add `<dialog id="kbd-help">` to `base.html` body (single instance).
- Add `<button class="kbd-help-badge">?</button>` to `base.html`.
- Add minimal CSS to `base.css` for these three elements.
- **Acceptance:** open any page → click toggle → all `<time>` elements
  re-render; press `?` → dialog opens; `Esc` closes; `g r` navigates
  to runs.

### Step 2 — Run list filters + duration column + empty states

- Add filter row to `run_list.html`.
- Add JSON data island.
- Create `run_list.js` (reads data island, applies filters, re-renders
  `<tbody>`, runs `setInterval` for live durations).
- Add empty-state copy + "no filter match" state.
- **Acceptance:** typing in search filters rows; clearing search
  restores all rows; reload preserves filter state.

### Step 3 — Workflows index filters + last-modified

- Server route enriches each workflow with `mtime_iso` (filesystem
  mtime, ISO 8601 UTC).
- Add filter row + last-modified column to `workflows_index.html`.
- Create `workflows_index.js`.
- **Acceptance:** typing in search filters rows; `Last modified`
  column respects localtime toggle.

### Step 4 — Doctor grouping + hide-terminal toggle

- Server route emits `problem_records_by_kind` (groups dict) and
  attaches `run_state` to each record where available.
- Add `<details>`-based grouping, hide-terminal checkbox, doc-link
  anchors to `doctor.html`.
- Create `doctor.js` for persistence.
- **Acceptance:** > 5-problem groups start collapsed; hide-terminal
  toggle filters terminal-run problems; preferences persist across
  reload.

### Step 5 — Graph tooltips

- Create `run_detail_tooltip.js`.
- Wire to `run_detail.html`.
- *Why last among interactive steps:* the SVG graph hover is the
  smallest user-facing addition and depends on no other change.

### Step 6 — Promote next-actions banner

- Reorder `run_detail.html`: move next-actions block to between
  `<header class="run-header">` and `<div class="run-grid">`.
- Wrap in `<section class="next-actions-banner" role="region"
  aria-label="Next actions" tabindex="0">`.
- Hide on terminal runs via Jinja condition.

### Step 7 — `app.css` dark-mode parity

- Mechanical audit (the table in §9.1). Add `prefers-color-scheme:
  dark` blocks. Verify contrast.

### Step 8 — Docs + tests

- Update `docs/HOW_TO_HUMAN.md` with the four anchors referenced
  by empty-state and doctor doc-links:
  - `#starting-a-run`
  - `#generating-a-workflow`
  - `#recovery-stale-leases`
  - `#closing-sessions`
- Update `docs/CHANGELOG.md` with the V1 ergonomic-improvements line.
- Update `README.md` and `docs/UBIQUITOUS_LANGUAGE.md` if any new
  terms surface (likely none; the design intentionally re-uses
  existing vocabulary).
- Run `make test`, `make smoke`, UI snapshot tests, doc-link tests.

## 11. Cross-surface invariants (the "shared grammar" contract)

These invariants must hold across every change in this RFC; treat
them as acceptance gates the reviewer applies to each PR slice.

1. **No new server route** is added for filter behavior. All
   filtering is client-side over a data island.
2. **No new runtime dependency** is added (no `npm`, no CSS
   framework, no bundler).
3. **`localStorage` keys are namespaced `striatum.ui.*`.**
4. **Every persisted preference has a default that matches the
   pre-RFC behavior.** First-time operators see today's UI.
5. **Every interactive affordance must work with keyboard only.**
   Inline buttons are real `<button>` elements; toggles are real
   `<button>` or `<input type="checkbox">` elements; dialogs use
   native `<dialog>`.
6. **Every `<time>` element on every page has a `datetime` attribute
   in UTC ISO 8601 with `Z` suffix.** Plain text timestamps are
   bugs.
7. **Every empty state names a CLI verb and links to HOW_TO_HUMAN.**
8. **No CSP loosening.** `script-src 'self'` and `style-src 'self'`
   stay; all JS is in `/static/*.js`, all CSS is in `/static/*.css`.
9. **Dark mode parity in `app.css`.** No `app.css` rule may set a
   literal color in light mode without setting one in dark mode.
10. **Justification per decision.** Every affordance choice in the
    template/CSS/JS has a one-line `// why:` comment when the
    choice is non-obvious (no editorializing comments for
    well-named identifiers, per project comment discipline).

## 12. Risks and mitigations

- **JS-disabled operator.** Filters, localtime toggle, and keyboard
  shortcuts degrade to "no effect" but the underlying tables still
  render (server-side). *Mitigation:* this is intentional and
  consistent with D073's server-rendered Jinja2 baseline.
- **Tampered `localStorage`.** A user with stale or malformed JSON
  in `localStorage` could see filter behavior that doesn't match
  the defaults. *Mitigation:* `loadFilterState` catches JSON parse
  errors and falls back to defaults. The "Clear all filters" link
  is always one click away.
- **Date-range filter on a clock-skewed machine.** `Last 24h`
  is computed against the client's `Date.now()`. *Mitigation:* the
  server timestamps are absolute UTC; if a machine's clock is hours
  off, "Last 24h" will be incorrect by that offset. Acceptable for
  V1; documenting the limitation in HOW_TO_HUMAN.
- **`setInterval` for live durations leaks if the page is left open
  for hours.** *Mitigation:* the interval is a single 30s tick that
  iterates a small array; no closure capture; no listener
  accumulation. Verified by spot test in step 2.
- **Help dialog conflicts with browser shortcuts.** `?` is rarely
  bound at the browser level, but `g`-prefix sequences could
  collide with a vimium-style extension a user has installed.
  *Mitigation:* `g` shortcuts are explicitly disabled when focus is
  in an input; users with their own bindings can override.

## 13. Out of scope (for the reviewer)

To keep the slice tight, the design explicitly *does not* cover:

- Query-string-encoded filter state (the RFC defers to V1.5).
- A `needs_attention` composite filter pill (V1.5).
- Sticky positioning of the next-actions banner (V1 stays in flow).
- Configurable keyboard shortcuts.
- A "resolve all stale claims" mutation button on doctor.
- Mobile-narrow-viewport responsive overhaul.
- Hosted-mode UX.
- SVG zoom/pan.

If a reviewer raises any of these, the response is "deferred — see
RFC 0037 Open Questions / Non-Goals." This design is not the place
to relitigate scope.

## 14. Reviewer checklist

A reviewer holding the line on the shared grammar from §1 should ask,
for each piece:

- Does the affordance preserve today's default on first visit?
- Is the indicator the current state, not the target?
- Is `localStorage` namespaced and JSON-safe?
- Is there a one-click escape to defaults?
- Does every empty state name a CLI verb?
- Is every `<time>` element a real `<time datetime="…">`?
- Does the dark-mode block parallel the light-mode block 1:1?
- Is there a `// why:` comment for every non-obvious choice?

If those eight questions can be answered "yes" for every piece, the
RFC 0037 V1 slice is done.

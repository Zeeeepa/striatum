# Design: RFC 0037 Web UI Ergonomic Improvements

author: designer-gemini-pro-001

## 1. Overview

This design addresses the ergonomic gaps identified in RFC 0037 for the Striatum web UI. The focus is on improving observability, triage, and navigation through filtering, search, accessibility enhancements, and visual polish, while maintaining the zero-runtime-dependency invariant.

## 2. Implementation Strategy

### 2.1 Accessibility (A11y)
- **Keyboard Navigation:** Implement a global listener in `base.js` for `g` + `r/w/c/d` shortcuts.
- **Help Overlay:** Use the native `<dialog>` element for the `?` shortcut. It provides built-in focus management and escape-key handling.
- **Focus Management:** Ensure that when a filter is applied, focus remains in a logical place (e.g., the search box if it was active).
- **ARIA Labels:**
  - Filter pills: Use `role="group"` for the pill container and `aria-pressed` for the active filter button.
  - Search inputs: Explicit `<label>` or `aria-label`.
  - SVG Graph: Ensure nodes have `<title>` for hover tooltips (which doubles as accessible names) and `role="link"` for navigation.

### 2.2 Responsive Behavior
- **Viewport Target:** Primary target is desktop-first (≥1200px), but ergonomic at 1024px.
- **Filter Rows:** Use `display: flex; flex-wrap: wrap;` for filter rows to ensure they stack gracefully on narrower viewports without breaking the layout.
- **Run Grid:** Keep the existing 1024px breakpoint in `base.css` that collapses the 3-column layout into a single column.

### 2.3 State Persistence & Layering
- **Mechanism:** `localStorage` for client-side persistence.
- **Schema:** 
  ```json
  {
    "version": 1,
    "filters": {
      "run_list": { "search": "", "state": "all", "date": "7d" },
      "workflows": { "search": "", "status": "all" },
      "doctor": { "hide_terminal": true }
    },
    "settings": {
      "localtime": false
    }
  }
  ```
- **Migration:** A simple `migrateStorage()` function in `base.js` will check the `version` key and reset or transform data if the schema changes in future RFCs.
- **First Visit:** Default state is applied if no `localStorage` entry exists.

### 2.4 Dark-Mode Parity Audit
- **Audit Findings:** `app.css` components currently inherit some dark variables but lack explicit overrides for borders, background-elevated contrasts, and specific pill colors.
- **Checklist:**
  - [ ] `.job-list`, `.job-link`: Update hover backgrounds for better contrast.
  - [ ] `.status-pill`: Ensure contrast ratios for text on colored backgrounds.
  - [ ] `.run-grid`, `.run-jobs-rail`: Consistent border colors using `--border`.
  - [ ] `.workflow-edit-form`: Ensure inputs and cards have correct dark backgrounds.
  - [ ] `.chat-message`: Verify `bg-overlay` vs `bg-elevated` contrast in dark mode.

### 2.5 Empty-State Illustrations
- **Choice:** **Inline SVG Icons**.
- **Rationale:** More polished than ASCII/emoji, lightweight, and allows for consistent styling with the CSS palette.
- **Example:** A simple magnifying glass with a "0" or strike-through for search; a folder icon for workflows; a stethoscope/shield for doctor.

## 3. Component Details

### 3.1 Run List (`run_list.html` + `run_list.js`)
- **JSON Data Island:** Render `<script id="runs-data" type="application/json">{{ runs_json | safe }}</script>`.
- **Filtering Logic:** 
  - Search: Match `run_id`, `branch_name`, `workflow_id`.
  - State: Exact match on `state`.
  - Date: Compare `created_at` against `Date.now() - offset`.
- **Duration Column:**
  - Terminal: `completed_at - created_at`.
  - Running: `Date.now() - created_at` (updated via `setInterval` every 30s).

### 3.2 Doctor Page (`doctor.html`)
- **Grouping:** Group by `problem.kind` server-side.
- **Collapsible Sections:** Use `<details>` with `<summary>` for native browser behavior.
- **Toggle:** A checkbox/toggle for "Hide problems on terminal runs".

### 3.3 Graph Tooltips (`run_detail.js`)
- **Implementation:** 
  - `mouseenter` on `.graph-node`: Create/show a `div.tooltip` with `position: fixed`.
  - `mouseleave`: Hide.
  - Use `getBoundingClientRect()` to position relative to the mouse.

### 3.4 Localtime Toggle (`base.js` + `base.html`)
- **Header Toggle:** Add a button or checkbox in the `.site-header` styled as a toggle switch.
- **Transformation:** Use `document.querySelectorAll('time[datetime]')` to find all timestamps. 
  - UTC: Use `Intl.DateTimeFormat` with `timeZone: 'UTC'`.
  - Local: Use `Intl.DateTimeFormat` with the browser's default locale.
- **Persistence:** Value saved in `settings.localtime`.

### 3.5 Next Actions Promotion (`run_detail.html`)
- **Reordering:** Move the `{% if next_actions %}` block from the sidebar to a new `.promoted-actions` container located immediately following the `.run-header`.
- **Styling:** Use a banner-like style (`background: var(--bg-overlay); border-left: 4px solid var(--accent);`) to make it visually distinct and high-priority.
- **Visibility:** Only visible for non-terminal runs.

## 4. Test Strategy

### 4.1 Automated Tests
- **UI Snapshots:** Update existing snapshots in `tests/test_web_ui.py` to include the new filter rows and columns.
- **JS Unit Tests:** Add `tests/static_unit/` (new) using a headless runner (or simple JSDOM-based tests) to verify:
  - Filtering logic (search, date range).
  - Persistence (localStorage read/write).
  - Time toggle (DOM transformation).

### 4.2 Manual Checklist
- [ ] Keyboard navigation: `g r`, `g w`, `g c`, `g d` work from all pages.
- [ ] Keyboard help: `?` opens dialog, `Esc` closes.
- [ ] Persistence: Filter "failed" runs, refresh page -> "failed" stays selected.
- [ ] Dark mode: Toggle OS dark mode and verify readability of all status pills.
- [ ] Responsive: Resize browser to 1024px; filter row wraps correctly.
- [ ] Accessibility: Tab through filters; ensure focus indicators are visible.

## 5. Deployment & Migration
- No database migrations required.
- `localStorage` handles client-side state; first-visit defaults provide a smooth "out of box" experience.

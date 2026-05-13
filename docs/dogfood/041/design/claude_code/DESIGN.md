# RFC 0038 UI Features — Claude Code Design (UX-Oriented)

author: designer-claude-opus-001

## 0. Scope and posture

This design covers the five framework-island UI additions from
[RFC 0038](../../../../rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md)
§5 and their first-time-operator UX. It does **not** redesign the
toolchain, project layout, build/CI, or wheel-packaging shape — those
are handled by the codex-side design. Here we focus on what each island
*looks like*, how operators move through it, and what accessibility
guarantees each surface must hold.

Anchor invariants (from RFC 0038, RFC 0037, base.html):

- Server-rendered Jinja2 pages stay. Each island mounts into a single
  named DOM slot via `createRoot()`.
- CSP unchanged (no inline scripts, no eval). Styles come from
  `base.css` palette variables; new island CSS extends, never shadows.
- Dark mode is driven by `prefers-color-scheme` and the palette variables
  already exposed on `:root` / `:root[data-theme="dark"]`.
- The skip-link, top-nav, and footer added by RFC 0037 are not touched.
- "Operator confirmation" for any mutation reuses the RFC 0036 gesture
  pattern (an explicit click on a confirmation button labelled with the
  exact action verb, after a preview).
- Repo-relative paths only (no host filesystem escape).

Each island below specifies, in order:

1. Navigation grammar / IA.
2. First-time UX (what an operator sees the first time they land).
3. Widget inventory + field bindings.
4. Keyboard map.
5. Focus / ARIA / `<dialog>` handling.
6. Empty / error / loading / validation states.
7. API contracts touched.

## 1. Tree browser island — `/view/`

### 1.1 Route and IA

- `/view/` (no path) renders `view_index.html` (new Jinja2 template)
  that mounts a single island `#island-tree-browser` with
  `data-props='{"rootPath": ""}'`.
- `/view/<path>` (existing) keeps its current behaviour — single-file
  viewer. For a directory path, the server redirects to `/view/` with
  `?initial=<path>` so the tree opens pre-expanded to that node.
- Breadcrumb at top is server-rendered into `base.html`'s content-header
  slot, *not* the island; the island only renders the tree itself plus
  the search input. This means the breadcrumb is in DOM before JS runs
  and is announced first by screen readers.

### 1.2 Navigation grammar

```
/view/                          → tree rooted at repo top
/view/?initial=docs/rfcs        → tree open with docs/rfcs expanded
/view/docs/SPEC.md              → single-file viewer (existing route)
/view/?q=spec                   → tree rendered + search filter "spec"
```

The `?initial=` query is read once on mount, then mirrored back to the
URL on every expand / collapse via `history.replaceState()` so a
browser refresh restores the same expansion state. Limit: max 32 open
directories serialized in the URL; older expansions silently drop. (No
local storage — keeps the surface stateless per RFC 0013 V1.)

### 1.3 First-time UX

1. Operator clicks "View" in the top nav for the first time.
2. The Jinja2 shell paints immediately: page header, breadcrumb
   `repo /`, a short empty placeholder card reading
   "Loading repository tree…" with a spinner.
3. The island hydrates and replaces the placeholder with the top-level
   entries from `GET /v1/repo/tree?path=` (empty path = repo root).
4. The first row receives `tabindex="0"`, the rest are `tabindex="-1"`
   (standard roving tabindex pattern). Visible focus ring uses the
   existing `:focus-visible` palette token.
5. A help affordance is rendered as an unobtrusive `<details>` element
   under the search box: "Keyboard shortcuts" — open it once and the
   key map appears inline. (No tooltip layer; no toast layer.)

### 1.4 Widget inventory

- **Search box** (`<input type="search" aria-label="Filter files in
  this tree">`). Fuzzy match against the already-loaded subtree only;
  does not fan out and lazy-load every collapsed directory (would melt
  the daemon on a big repo). Empty matches show
  "No matches in expanded tree. Press Enter to load all directories
  and search again." That Enter recursively expands and re-filters,
  with an inline progress indicator and a hard cap (1000 entries; if
  exceeded, abort + message).
- **Tree node row** — single `<div role="treeitem">` per entry,
  containing:
  - directory: chevron icon + folder icon + name (chevron rotates on
    expand);
  - file: ext-based glyph + name + a muted size suffix
    (`12 KiB` style, formatted server-side and returned in
    `entries[].size_human`).
- **Empty directory state** — italic muted line "Empty directory" with
  an icon. Not a row — a static child of the expanded directory's
  `role="group"` container.
- **Loading state per-directory** — a single `role="treeitem"` with
  `aria-busy="true"` and label "Loading…" replaces the children for
  150 ms after expansion; this avoids flicker on fast responses.

### 1.5 Keyboard map

| Key | Effect |
| --- | --- |
| `↓` / `↑` | Move focus to next / previous visible row. |
| `→` | If collapsed dir: expand. If expanded: move to first child. If file: no-op. |
| `←` | If expanded dir: collapse. If collapsed / file: move to parent. |
| `Enter` | If directory: toggle expand. If file: navigate to `/view/<path>`. |
| `Esc` | If search has focus: clear filter. Else: move focus to parent dir. |
| `Home` / `End` | Jump to first / last visible row. |
| `/` (slash) | Move focus to the search input (single-key search shortcut, common pattern). |
| `Type-ahead` | Letter keys with focus on the tree jump to the next row starting with that letter within the current expanded view. |

This matches the WAI-ARIA tree-view authoring practice. All shortcuts
are documented in the inline `<details>` block in §1.3.

### 1.6 Focus / ARIA

- Container: `<div role="tree" aria-label="Repository file tree">`.
- Each row: `role="treeitem"` plus `aria-level="N"`, `aria-expanded`
  (directories only), `aria-selected="false"` (we do not implement
  multi-select; selection is implicit on focus).
- Children container: `<div role="group">` nested directly under the
  parent `treeitem`. (Per ARIA APG, `group` children of a `treeitem`.)
- Roving tabindex: exactly one row carries `tabindex="0"`; `Tab` thus
  exits the tree to the next page region (the file pane on the right
  on wide layouts, footer otherwise) — it does **not** trap.
- Focus indicator: existing `:focus-visible` palette ring. Never
  remove the outline.
- Live region: a visually-hidden `<div aria-live="polite">` announces
  load failures and "Search loaded N additional directories".
- Hovers do not select; only keyboard arrows and click change the
  focused row. (Hover styling stays subtle to avoid implying selection.)

### 1.7 Error states

| Failure | Surface |
| --- | --- |
| `GET /v1/repo/tree` 5xx | Inline row replacing expanded children: "Couldn't load. Retry." button. Live region announces error. |
| 403 / path-escape | Static error card replacing tree: "This path is outside the repo. Open the repo root." with a "Open `/`" button. |
| Network offline | Same as 5xx; the daemon is local so this is rare but possible. |

### 1.8 API contract touched

Adds `GET /v1/repo/tree?path=<rel>` (matches the codex-side route
plan). Response shape consumed by this island:

```json
{
  "path": "docs/rfcs",
  "entries": [
    {"name": "0038-...md", "kind": "file", "size": 18293, "size_human": "18 KiB"},
    {"name": "subdir", "kind": "dir", "size_human": null}
  ],
  "truncated": false
}
```

`truncated: true` (>500 entries) renders a trailing row "Showing 500
of N — refine search to load more".

## 2. Workflow chooser wizard — `/workflows/new`

### 2.1 Route and IA

- `/workflows/new` renders `workflow_new.html` (new template) that
  mounts `#island-workflow-chooser`.
- The wizard is a **single page with a stepper** in the header, *not* a
  series of URLs. Each step replaces the body content; the URL gains
  `?step=1..6` via `history.replaceState()` so a refresh restores the
  step and a back button works.
- "Cancel" at any step posts no data and navigates to `/workflows`.

### 2.2 First-time UX (golden path)

The chooser is the only mutation surface in this RFC, so first-time
clarity matters most.

1. The page header shows a stepper:
   `[1 Shape] · [2 Lane set] · [3 Modifiers] · [4 Details] · [5 Preview] · [6 Save]`
   with the active step highlighted; completed steps are clickable
   (rewind), future steps are not.
2. Step 1 paints radio cards immediately after `GET /workflow-templates`
   resolves. Card content: shape `display_name` (heading),
   `summary` (paragraph), `recommended_for` (chip list). The first
   card is the keyboard-focused option by default; "Next" disables
   until a shape is chosen.
3. Each subsequent step opens with focus on its primary control
   (the first radio card; the first form field; the Preview button)
   so an operator who never touches the mouse can complete the wizard.
4. Step 5 — Preview — opens with a side-by-side layout: generated
   `workflow.json` rendered as the existing SVG graph on the left, the
   would-write file list on the right. *Nothing has been written yet.*
   A persistent banner reads "Preview only — no files have been
   written. Review then confirm in Step 6."
5. Step 6 — Save — is a `<dialog>`-modal confirmation: the operator
   must click a primary button labelled exactly
   "Generate workflow into `<scaffold_root>` and `<artifact_root>`".
   The button is disabled for 600 ms after the dialog opens to defuse
   double-clicks. Success state replaces the wizard with a card linking
   to `/workflows/<new_path>`.

### 2.3 Step-by-step widget inventory

| Step | Widget | Notes |
| --- | --- | --- |
| 1 Shape | `<div role="radiogroup">` of cards | Each card is `role="radio"` with `aria-checked`. Arrow keys move between radios; Space / Enter selects. Card layout: title + 1-line summary + chip list of `recommended_for` tags. |
| 2 Lane set | Same radiogroup pattern, filtered to `default_lane_sets` for the chosen shape. | If the chosen shape exposes no default lane sets, this step is automatically skipped with a banner: "This shape uses a fixed lane set; advancing to lane modifiers." |
| 3 Lane modifiers | Multi-select listbox with checkbox children | `<ul role="listbox" aria-multiselectable="true">` of `<li role="option" aria-selected="..">`. Each entry shows the modifier name + a 1-line `effect` + a small `incompatible_with` warning chip if compatibility fails after a click; the offending checkbox auto-unchecks and a live-region announces the conflict. |
| 4 Details | Form with explicit field grouping | Two columns on wide layouts; one column under 720 px. Each field uses the widget below. |
| 5 Preview | Read-only grid: SVG graph + file list | "Back" returns to Step 4 keeping all answers; "Generate" advances to Step 6 dialog. |
| 6 Save | Modal `<dialog>` confirmation | See §2.5. |

Step 4 (Details) field-to-widget mapping:

| Field | Widget | Validation |
| --- | --- | --- |
| `workflow_id` | `<input type="text" pattern="[a-z0-9-]+">` | Lowercase, kebab; blocks the Preview button while invalid; inline error message. |
| `name` | `<input type="text" maxlength="80">` | Required; visible character counter only when within 10 chars of the cap. |
| `scaffold_root` | Path picker (combobox; see §2.4) | Path is repo-relative; defaults to `docs/dogfood/<workflow_id>/`. |
| `artifact_root` | Path picker | Defaults to `docs/dogfood/<workflow_id>/artifacts/`. |
| `branch suggestion` | `<input type="text">` | Defaults to `striatum/<workflow_id>`. |
| `lane commands` | Stack of rows per lane (label + `<input>` for the command) | Pre-filled from the chosen lane set's known commands; operator can edit. |

### 2.4 Path picker (used in Step 4 + the graph editor §3)

Reusable component:

- `<input type="text" role="combobox" aria-expanded aria-controls>` plus
  a `<ul role="listbox">` popover below.
- On focus + typing, the listbox shows up to 8 matching directory
  entries from `GET /v1/repo/tree?path=<typed prefix>`; arrow keys
  navigate, Enter accepts.
- Escape closes the listbox and returns focus to the input.
- Selecting a directory inserts its path; selecting a file shows an
  inline warning "expected a directory" and keeps the input as-is.
- The component degrades to a plain `<input>` if the API returns no
  entries — operator can still type the path manually.

### 2.5 Step 6 confirmation `<dialog>`

- HTML: `<dialog id="generate-confirm">` with the React modal mounted
  inside via the React 19 `<dialog>`-as-portal pattern; opened via
  `dialogRef.current.showModal()`, closed via
  `dialogRef.current.close()`.
- On open:
  - **Focus moves to the first focusable element inside the dialog**
    (the "Generate workflow…" primary button, after the 600 ms
    enable delay; before that, the Cancel button receives focus).
  - The previously focused element is stored; on close, focus returns
    to it (the Step 5 Preview "Generate" button).
  - The page background is `inert` (`<main inert>`), preventing tab
    escape; `Tab` cycles inside the dialog only.
- `Esc` closes the dialog (HTML `<dialog>` semantics); explicit
  Cancel button does the same.
- The dialog body lists the exact files about to be written
  (relative paths), echoing the Step 5 preview, then the primary
  button at the bottom.
- Submitting calls `POST /workflows/generate` with
  `confirm_write: true`; on 4xx the dialog stays open with an inline
  error region; on 2xx the dialog closes and the page replaces the
  wizard with a success card.

### 2.6 Keyboard map (chooser)

| Key | Effect |
| --- | --- |
| `Tab` / `Shift+Tab` | Move between fields and stepper controls (single sequence; no `tabindex` > 0). |
| `Enter` | On radio-card group: select focused card. On form fields: do not advance step (prevents accidental Step 5 launch). On the explicit Next button: advance. |
| `Esc` | Cancel — same as Cancel button; confirm-dialog if dirty (see §2.7). |
| Arrow keys | Move within radio-card groups and multi-select lists. |

### 2.7 Dirty / abandon protection

If the wizard has user-entered data and the operator clicks Cancel,
navigates back, or presses Esc on any step ≥ 2, a non-modal
confirmation banner (not a `confirm()` popup) slides in:
"Discard wizard answers and return to /workflows?" with
Discard / Keep editing buttons. Refusal to confirm keeps state.

### 2.8 Empty / error states

- `GET /workflow-templates` 5xx: Step 1 renders an error card with a
  Retry button; the rest of the stepper is hidden.
- `POST /workflows/generate/preview` 4xx: stays on Step 4 with inline
  field errors mapped from the server error payload (`field_errors:
  {workflow_id: "already exists"}` etc).
- `POST /workflows/generate` 4xx: dialog stays open; banner inside
  the dialog with the error; the primary button re-enables.

## 3. Drag-drop workflow graph editor

### 3.1 Route and IA

- `/workflows/edit/<path>` continues to render `workflow_edit.html`.
  The template grows two stacked regions:
  1. The graph editor island `#island-workflow-graph-editor`
     (mounted full-bleed, fixed height ~640 px on desktop, 100 vh
     minus header on small screens).
  2. The side panel `#island-workflow-graph-editor-side` mounted into
     the same React root as a portal so React state is shared. (One
     `createRoot()` call; the side panel and canvas are siblings in the
     React tree.)
- The legacy form-driven editor stays available behind a "Use the
  text editor instead" link at the bottom of the page, for one
  release cycle per RFC 0038 §Open Questions.

### 3.2 First-time UX

1. Operator clicks the (promoted) **Edit** button on `/workflows/<path>`
   (§5). The page paints the graph canvas with the existing jobs as
   react-flow nodes laid out by `dagre` (left-to-right, the same shape
   as the existing SVG graph). The side panel is empty with a placeholder
   "Click a node to edit it, or drop a block from the palette to add
   a new job."
2. The palette is a left-rail vertical list of "block kinds" from
   RFC 0034 §5 closed vocabulary — one chip per kind, drag-source. Each
   chip has a tooltip describing its kind.
3. Operator clicks a node → side panel populates with that job's
   editable fields. The panel scrolls independently; Save is a sticky
   footer button always visible.
4. Operator drags a palette chip onto the canvas → react-flow drops a
   new node with placeholder defaults; the side panel auto-opens with
   the new node selected.
5. Operator wires nodes by dragging from a node's right-edge handle to
   another node's left-edge handle. The edge's `on` verdict is
   editable from a popover that opens on edge click. Edge double-click
   deletes (with an Undo banner for 5 seconds; non-modal).

### 3.3 Side-panel widget mapping (per RFC 0038 §5d)

| Field | Widget | Notes |
| --- | --- | --- |
| `job_id` | `<input type="text">` | Read-only after creation (changing breaks edges). A "Rename" button promotes it to editable + warns. |
| `type` | `<select>` | Dropdown of the closed `generic` / `synthesis` / `review` / `cycle_check` vocabulary. |
| `role_id` | `<select>` from `GET /v1/roles` | Populated lazily on focus; loading state in the dropdown. |
| `lane_id` | `<select>` from `GET /v1/lanes` | Same. |
| `review_posture` | `<fieldset role="radiogroup">` of 9 first-class postures as radio buttons + a 10th "Custom…" radio that reveals an `<input>` underneath when chosen | Each radio has a 1-line description as `<label>` content. |
| `required_review_postures` | Multi-select with checkbox listbox | Same 9 closed postures as `role="option"` items. |
| `write_scope.allowed_paths` | Repeatable rows: path picker (§2.4) + remove button. "+ Add path" button under the last row. | Empty list collapses to the "+ Add path" button alone with a muted "No paths configured" line. |
| `write_scope.forbidden_paths` | Same shape. | Default row pre-filled with `.striatum/` and read-only — operator can't remove it. |
| `expected_artifacts[]` | Repeatable card per artifact, each card containing: kind `<select>`; logical_name `<input>`; path picker; required `<input type="checkbox" role="switch">`. | Cards are reorderable via up / down buttons (no native drag-to-reorder for accessibility; drag is graph-canvas-only). |
| `parallel_group` | `<input role="combobox">` with autocomplete listing existing groups in the workflow | Type-ahead matches in-workflow groups; submitting an unknown value creates a new group (announced via live region). |
| `on` (edge verdict) | `<select>` in the edge popover | Closed verdict vocabulary (`pass` / `fail` / `block` / `escalate` etc; pulled from the same source as the legacy editor's options). |

### 3.4 Canvas keyboard map

Drag-drop is mouse-first by definition, but every action must have a
keyboard equivalent. Per WAI-ARIA APG "rich application" guidance:

| Key | On canvas focus | On side-panel focus |
| --- | --- | --- |
| `Tab` | Cycles through nodes in topo order; visible focus ring on the focused node. | Standard form tab order. |
| `Enter` | Open the focused node in the side panel; move focus into the panel. | Submits the focused button; on a row, no-op. |
| `Space` | Same as Enter on a node. | Toggle a checkbox / switch. |
| `Delete` / `Backspace` | Delete the focused node (with confirm banner) or edge. | No-op. |
| `Arrow` keys | Pan the canvas viewport (10 px per press). | Move between fields. |
| `+` / `-` | Zoom in / out (1.1×). | No-op. |
| `0` | Fit-to-view. | No-op. |
| `n` | Open the palette in a small menu near the focused node so an operator can add a block without dragging. | No-op. |

A persistent help button at the bottom-right of the canvas opens a
non-modal popover listing every shortcut.

### 3.5 ARIA / focus / dialog

- The react-flow canvas has `role="application"` + an `aria-label`
  describing what it is, *plus* a hidden text region the daemon emits:
  "Workflow `<id>` with N jobs and M edges. Use Tab to step through
  jobs." This is the operator's anchor on first hydration before they
  see anything.
- Every node is wrapped in a focusable container with
  `aria-label="job <job_id>, role <role_id>, lane <lane_id>"` so
  screen readers can announce the node identity without rendering the
  graph topology.
- The edge popover and the palette "add block" popover trap focus
  while open (Esc closes; click outside closes). The side panel is
  *not* a focus trap — Tab can leave it back to the canvas.
- Field-level validation errors render adjacent to the field with
  `aria-describedby`; the field gains `aria-invalid="true"`.
- The Save button on the sticky footer reads "Save workflow"
  unconditionally; on submit the button shows a spinner with
  `aria-busy="true"` and the live region announces "Saving…" then
  "Saved." or the error.

### 3.6 Validation cadence

- Field-change: lightweight client check (regex shapes, required
  flags, enum membership) renders inline error chips immediately. No
  server call.
- Save: posts the full graph JSON to the existing workflow-edit
  endpoint; the server runs `workflow validate`; the response payload
  carries `field_errors` keyed by `<job_id>.<field>` so the React side
  scrolls the first error into view, focuses the offending widget, and
  paints the surrounding card with the error tone.
- A dirty-state indicator in the page header (`Unsaved changes`)
  appears whenever any field differs from the last-saved state; a
  beforeunload guard fires if the operator tries to navigate away.

### 3.7 Empty / loading / error

- Empty workflow (no jobs): canvas shows a centered illustration-free
  "Drop a block from the palette to begin." with the palette pre-opened.
- Loading roles / lanes: dropdowns show a single disabled option
  "Loading…" with `aria-busy="true"`.
- Save 5xx: error banner above the canvas; nothing is mutated locally.

## 4. Code viewer island — `/view/<path>` (non-Markdown)

### 4.1 Route and IA

- `/view/<path>` keeps its current Markdown rendering path. When the
  file extension is not Markdown, the template instead mounts
  `#island-code-viewer` with `data-props='{"path": "<path>",
  "language": "<detected>"}'`.
- The Jinja2 server-template includes the raw file contents inside the
  mount slot as `<pre>` so the viewer is *legible without JS* — the
  island upgrades the pre-rendered text in place rather than fetching
  it again. (Falls cleanly back if hydration fails.)

### 4.2 Language detection and fallback

Detection order:

1. Extension → grammar lookup in a static map shipped with the
   bundle (`.json` → `json`, `.py` → `python`, `.ts`/`.tsx`/`.js`/`.jsx`
   → `ts`/`tsx`/`js`, `.sh`/`.bash` → `bash`, `.yaml`/`.yml` → `yaml`,
   `.toml` → `toml`, `.md` → `markdown`, `.sql` → `sql`).
2. Shebang sniff for extensionless files (`#!.*python` → python,
   `#!.*sh` → bash). Done client-side from the first line of the
   pre-rendered text.
3. Unknown → render as plain monospace with line numbers, no shiki
   call. A muted footer reads "Plain-text view — no grammar bundled
   for `.<ext>` files." This is the explicit fallback path required
   by the prompt; do not attempt to download additional grammars.

### 4.3 First-time UX

1. Server-rendered `<pre>` paints first — the file is readable even
   on JS-disabled browsers.
2. shiki replaces the `<pre>` content with tokenized spans inline;
   line numbers, copy button, and raw-link appear in the header strip.
3. For files > 500 lines, only the first 200 lines render expanded;
   the rest are inside a collapsed `<details>` element with the
   summary "Show remaining N lines". The collapse is a real
   `<details>` so it works without JS and tells screen readers what
   it does.

### 4.4 Widget inventory

- **Header strip** above the code:
  - Path breadcrumb (re-rendered for parity with `/view/`).
  - "Copy" button — copies the full file body; success announced via
    live region ("Copied 84 lines.").
  - "Raw" anchor — links to `/raw/<path>` (existing route).
  - "Wrap" toggle switch — switches between overflow-x scroll and
    soft-wrap; persists in `sessionStorage` only.
- **Line gutter** — left-aligned, monospace, palette-muted; line
  numbers are `<span aria-hidden="true">` so screen readers ignore
  them while still seeing the code.
- **Copy-to-clipboard** uses the Clipboard API; if denied (CSP / older
  browser), falls back to a hidden textarea + `execCommand("copy")`
  shim and shows a brief tooltip.

### 4.5 Keyboard map

| Key | Effect |
| --- | --- |
| `Tab` | Standard order: Copy, Raw, Wrap, then into the code body (the code body is a single focusable region; arrow keys do not move per-line — operators use OS-level page navigation). |
| `Esc` | If a "Copied!" announcement is showing, dismiss it. |

The code body itself is not a custom keyboard widget — operators read
it with normal scroll / page-down. (Avoid reinventing line navigation.)

### 4.6 ARIA / dark mode

- shiki is invoked with both `light` and `dark` themes; the wrapper
  applies the right theme via a CSS class driven by
  `prefers-color-scheme` plus the existing `data-theme` attribute on
  `<html>` (set by RFC 0037's theme toggle). No JS theme-switch logic
  on the island — just CSS.
- The code container has `role="region" aria-label="File contents:
  <path>"` so screen readers announce its identity on focus.
- Copy / Raw / Wrap each have an `aria-label` with the exact action
  ("Copy file contents", "Open raw file", "Toggle line wrap").
- No `<dialog>` is used; nothing to focus-trap.

### 4.7 Performance

- Initial bundle target: ≤ 80 KB gzip for shiki + the 8 grammars
  combined. (Shiki's tree-shakable build supports per-grammar
  imports; the codex-side toolchain design specifies the static
  import list.)
- Files > 5 MB: do not call shiki — render the plain `<pre>` and
  show a muted "Highlighting disabled for files > 5 MB." footer.

## 5. Promoted Edit affordance

### 5.1 Visual

- Replace
  `<a href="/workflows/edit/{{ workflow.path }}" class="muted">Edit</a>`
  with
  `<a href="/workflows/edit/{{ workflow.path }}" class="primary-button secondary" role="button" data-action="edit">…</a>`.
- Button label: "Edit workflow" plus an inline SVG pencil glyph
  before the label (`fill: currentColor`).
- Variant: a *secondary* primary-button (same shape and size as
  "Run this workflow now"; different background tone via a `secondary`
  modifier class to keep visual hierarchy — Run is the dominant
  action, Edit is the supporting one).

### 5.2 Placement

- Render *immediately to the left* of the Run button in the existing
  `<div class="run-meta">` cluster. Order in DOM (and visually on LTR):

  1. status pill
  2. version chip
  3. **Edit workflow** button (new)
  4. **Run this workflow now** button

- On viewports < 720 px the row wraps and Edit sits on its own line
  *above* Run (more important last) — already handled by the existing
  flex-wrap on `.run-meta`.
- On RTL locales the order mirrors automatically; no extra work.

### 5.3 Accessibility

- The element is an `<a>` styled as a button. We keep it an `<a>` so
  middle-click / open-in-new-tab / keyboard "open link" semantics keep
  working. The `role="button"` attribute is added so screen readers
  announce it as an action button (matching its visual treatment), and
  `Space` triggers it via a small JS keydown handler. Mouse / Enter
  use native anchor semantics.
- Focus ring uses the `:focus-visible` palette token (same as Run).
- The button is the *third* focusable element in `run-meta` — no
  `tabindex` is set; natural DOM order is correct.

### 5.4 No new JS bundle

The change is purely Jinja2 + CSS. No island, no React, no new
endpoint. Validated by the workflow-detail snapshot test.

## 6. Accessibility — system-wide guarantees

Every island shares these rules; they are non-negotiable.

### 6.1 Focus management

- Every island places initial focus deliberately (see each section
  above). No autofocus on text inputs unless the page exists only to
  enter that input (Step 4 details form).
- `<dialog>` modals (chooser Step 6 confirm, future similar) must:
  - call `showModal()` (not just `show()`) so the browser applies the
    top-layer + `inert` semantics;
  - move focus to the first focusable element inside on open;
  - store the previously focused element and restore it on close;
  - close on `Esc` (native `<dialog>` behaviour, kept enabled);
  - never disable Tab — Tab cycles within the dialog;
  - render their content with `aria-labelledby` pointing to the
    dialog title.
- Non-modal popovers (edge `on` editor, palette-on-keyboard menu)
  trap with a `useFocusTrap` shared hook but close on outside click
  *and* on Esc.

### 6.2 ARIA labels — required minimum per surface

| Surface | Required labels |
| --- | --- |
| Tree browser | `role="tree"` on root; `role="treeitem"` per row; live region for load errors; search input `aria-label`. |
| Chooser | Each step heading is an `<h2>`; each radio-card group is `role="radiogroup"` with an `aria-label` describing the step; the path picker is `role="combobox"` with `aria-controls` + `aria-expanded`. |
| Graph editor | Canvas `role="application"` + textual fallback region; each node has `aria-label` describing job_id+role+lane; the palette is `role="toolbar"`; the side panel is `<aside aria-label="Selected job details">`. |
| Code viewer | `role="region"` on the code container; line numbers `aria-hidden`; control buttons each `aria-label`'d. |
| Edit button | Anchor styled as button with `role="button"`. |

### 6.3 Keyboard completeness checklist

Every island ships with a one-page test plan (in
`docs/FRONTEND_DEVELOPMENT.md`) that asserts each of:

- Operator can reach every interactive element via Tab only.
- Operator can complete the primary task without a mouse:
  - tree: navigate to a file, open it;
  - chooser: complete all 6 steps and confirm;
  - graph editor: add a node, wire it, save;
  - code viewer: copy + jump to raw.
- Esc cancels modals and popovers; never strands focus.
- Focus indicators are visible against both light and dark palettes
  (verified via `make ui-test` snapshot).

### 6.4 Color, contrast, motion

- All island colors come from CSS variables in `base.css`. Islands
  may not declare hard-coded hex values.
- WCAG AA contrast on all text — verified by the snapshot test that
  renders each island in both themes and asserts contrast on labels.
- react-flow's drag animation respects
  `@media (prefers-reduced-motion: reduce)` — disable pan inertia and
  edge-routing animations under that media query.
- Status / live-region announcements use `aria-live="polite"`
  (default) except for fatal errors which use `aria-live="assertive"`.

### 6.5 Screen-reader manual smoke

Before each release, an operator runs each island through VoiceOver +
NVDA reading the role announcements end-to-end. The output is recorded
as a text transcript in `docs/dogfood/041/evidence/sr_smoke.md` (this
is an evidence artifact, not a test target — a docs-quality gate).

## 7. Documentation deltas (claude-side responsibility)

| Doc | Change |
| --- | --- |
| `docs/FRONTEND_DEVELOPMENT.md` (new) | Contributor-side guide: node setup, `make ui-install/build/dev/test`, island mounting pattern, type contracts (`shared/types.ts`), how to add a new island, accessibility test checklist. |
| `docs/HOW_TO_HUMAN.md` | New sections walking through `/view/` (tree browser), `/workflows/new` (chooser wizard), the graph editor, and the code viewer. Add the keyboard-shortcut table from §1.5 + §3.4. |
| `docs/UBIQUITOUS_LANGUAGE.md` | Add `frontend island`, `tree browser`, `workflow chooser`, `graph editor`, `code viewer`, `path picker`, `posture radio group`. Each entry one paragraph; cross-link to RFC 0038 §5. |
| `docs/CLI_REFERENCE.md` | No new verbs; add a "See also (web)" footer cross-referencing the new routes. |
| `CHANGELOG.md` | `Decided` entry for D092 is already landed. Add `Added` entries: "Tree-browser file explorer at `/view/`", "Workflow chooser wizard at `/workflows/new`", "Drag-drop workflow graph editor", "Syntax-highlighted code viewer for `/view/<path>`", "Promoted Edit affordance on workflow detail." |
| `docs/SPEC.md` | (Codex-side responsibility) Add the frontend toolchain section + island mount pattern. Claude-side does not duplicate. |

`docs/FRONTEND_DEVELOPMENT.md` template (skeleton):

```
# Frontend Development

## Toolchain
... (codex-side fills toolchain specifics; claude-side fills component
ergonomics) ...

## Island catalog
- tree-browser — mounted on /view/ ; data-props: rootPath
- workflow-chooser — mounted on /workflows/new
- workflow-graph-editor — mounted on /workflows/edit/<path>
- code-viewer — mounted on /view/<path> for non-Markdown

## Adding a new island
1. Create frontend/src/islands/<name>/
2. Export a default React component reading props from data-props
3. Register in src/main.ts so the bundle picks up the entry point
4. Add the Jinja2 template slot: <div id="island-<name>" data-props='...'>
5. Add a vitest under src/__tests__/<name>.test.tsx
6. Add the WCAG checklist run-through (see §6.3 of the RFC 0038 design)
```

## 8. Field bindings summary (graph editor → workflow JSON)

For implementer reference, this is the field-to-schema map the side
panel must produce. The shape is the existing workflow JSON schema —
no schema changes.

```
job ──┬── id              str            (input; immutable after creation)
      ├── type            enum           (select)
      ├── role_id         str            (select from /v1/roles)
      ├── lane_id         str            (select from /v1/lanes)
      ├── review_posture  str            (radio group + custom input)
      ├── required_review_postures str[]  (multi-select checkbox listbox)
      ├── parallel_group  str|null       (combobox with workflow-local autocomplete)
      ├── write_scope ──┬── allowed_paths      str[]   (repeatable path picker)
      │                 ├── forbidden_paths    str[]   (repeatable path picker; .striatum/ locked)
      │                 ├── mode               enum    (select)
      │                 └── repo_write         bool    (switch)
      └── expected_artifacts[]
            ├── kind          enum     (select)
            ├── logical_name  str      (input)
            ├── path          str      (path picker)
            └── required      bool     (switch)
```

Edges:
```
edge ──┬── from           str         (set by react-flow connect)
       ├── to             str         (set by react-flow connect)
       ├── on             enum        (select in popover)
       └── requires_verdict str|null  (select in popover)
```

## 9. Open questions for synthesis

These do not block design acceptance; flag them to the synthesizer.

1. **Custom-posture text input vs. closed posture list.** The prompt
   asks for a 10th "Custom" radio in the posture group. The workflow
   JSON schema does allow arbitrary strings, but practice has been to
   stay inside the 9 first-class postures. Recommendation: ship the
   "Custom…" radio but mark the surfaced input with a "may break
   automated tooling" warning, and gate it behind a click.
2. **Wizard step rewind.** Step rewinds are clickable; should they
   preserve later-step state if the operator returns to an earlier
   step and changes nothing? Recommendation: yes, preserve; only a
   *change* to an earlier step invalidates later steps. The
   change-invalidation flow is shown as a banner.
3. **Code viewer: large-file streaming?** > 5 MB falls back to plain
   `<pre>`. Should we add an explicit "Open in editor" link to the
   header strip instead? Recommendation: no — the raw-link already
   covers this and adding an editor-protocol handler is out of scope.
4. **Graph-editor undo depth.** Implementing full undo is large.
   Recommendation: V1 ships only delete-undo (a 5-second banner after
   each node / edge deletion). Field-level undo is browser-native
   inside each `<input>`. Add full undo / redo in a follow-up RFC.
5. **react-flow license / bundle size sanity.** react-flow is MIT.
   Bundle estimate ~120 KB gzip including its peer styles. Combined
   with shiki (~80 KB) and the chooser + tree bundles (~30 KB each)
   the total per-page island budget is ≤ ~260 KB gzip on the
   heaviest page (workflow edit). Implementers must verify this in
   the bundle-hash CI step.

## 10. Acceptance criteria additions (UX-specific)

The codex-side design covers the structural acceptance (toolchain, CI,
package data, etc.). This design adds the following UX-specific
acceptance items:

- Tree-browser passes the keyboard checklist in §6.3 (Tab reachability,
  full mouseless navigation, focus indicators in both themes).
- Chooser wizard's Step 6 confirm dialog focuses inside on open and
  restores focus on close; Tab does not escape; Esc cancels.
- Graph-editor canvas has a textual fallback region announcing job
  count + edge count; every node has a screen-readable label.
- Code-viewer renders without JS (server-side `<pre>` fallback); shiki
  upgrades in place; unknown extensions stay plain-text with a footer.
- Promoted Edit affordance is a button-styled `<a>` immediately to
  the left of Run; visible in both light + dark themes.
- All five islands respect `prefers-reduced-motion`.
- The skip-link from RFC 0037 still works on every new page
  (`/view/`, `/workflows/new`, `/workflows/edit/<path>`).
- A new vitest suite under `frontend/src/__tests__/` covers the
  keyboard maps for tree-browser and chooser at minimum.

## 11. Out of scope (explicit)

- Multi-select in the tree (no need; the tree is read-only).
- Search across collapsed directories without explicit Enter
  (perf risk).
- Drag-reorder of `expected_artifacts[]` (use the up/down buttons
  for accessibility).
- Per-line copy buttons in the code viewer (file-level copy is
  enough).
- Mobile-phone-sized layouts. Wizards and the graph editor degrade to
  usable-but-cramped layouts ≥ 720 px; below that they remain
  functional but explicitly not optimized. (Per RFC 0038 non-goals.)
- A second framework. React + react-flow + shiki only.

## 12. Handoff

The codex implementer ships:
- the Vite + TypeScript + npm scaffold under `src/striatum/web/frontend/`,
- the Make targets and CI integration,
- the `GET /v1/repo/tree` endpoint and the `/view/`, `/workflows/new`,
  and `workflow_edit.html` Jinja2 template updates,
- the promoted Edit button on `workflow_detail.html`,
- the committed bundle output under `src/striatum/web/static/build/`.

The claude implementer ships:
- the four React islands (`tree-browser`, `workflow-chooser`,
  `workflow-graph-editor`, `code-viewer`),
- `shared/api-client.ts` (typed against the existing `/v1/*` shapes)
  and `shared/types.ts`,
- a `useFocusTrap` shared hook + the `<dialog>`-management helpers
  consumed by the chooser's Step 6,
- `docs/FRONTEND_DEVELOPMENT.md` (component-ergonomics section) and
  the `docs/HOW_TO_HUMAN.md` walkthroughs.

Both implementers share the BUILD_HANDOFF artifact that records the
agreed island prop shapes, the bundle-hash baseline, and the
accessibility checklist results.

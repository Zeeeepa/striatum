---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0038", "web-ui", "build"]
---

author: reviewer-claude-opus-003

# Build Review — RFC 0038 V1 Component Ergonomics (claude lane, attempt 2)

Posture: ergonomics_dx (per RFC 0018). First-time-operator viewpoint
on the four React islands shipped under
`src/striatum/web/frontend/src/islands/`, the shared utilities in
`src/striatum/web/frontend/src/shared/`, the Vitest suite, the
template mount points, and the contributor + operator documentation.

This is a fresh-context, second-attempt review at the same artifact
path. The prior attempt's "prop-contract drift" headline was a
misread — `WorkflowChooserProps`, `CodeViewerProps`,
`WorkflowGraphEditorProps`, and `TreeBrowserProps` in
`src/striatum/web/frontend/src/shared/types.ts` accept exactly the
field names the templates emit (`allowMutations`, `templatesUrl`,
`previewUrl`, `generateUrl`, `path`, `language`, `saveUrl`,
`fallback`, `rootPath`, `treeUrl`). The optional fields are guarded by
sensible defaults inside each island. The templates load correctly
and the islands hydrate. I re-read each interface and template by
hand to confirm.

The build does have a handful of real ergonomic and accessibility
gaps (below), but they are localized polish issues, not contract
breaks. Verdict intent: **accept_with_findings**.

## Scope inspected

- `src/striatum/web/frontend/src/islands/tree-browser/TreeBrowser.tsx`
- `src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`
- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`
- `src/striatum/web/frontend/src/islands/code-viewer/CodeViewer.tsx`
- `src/striatum/web/frontend/src/shared/{types,api-client,mount,theme.css}.ts`
- `src/striatum/web/frontend/src/main.ts`
- `src/striatum/web/frontend/src/__tests__/*.test.ts` (six suites)
- `src/striatum/web/templates/{view_tree,view_file,workflow_new,workflow_edit,workflow_detail}.html`
- `src/striatum/service.py` `_render_view_path` (boundary check only)
- `docs/FRONTEND_DEVELOPMENT.md`
- `docs/HOW_TO_HUMAN.md` Web UI section

Required-checks status from the components angle:

- ✅ Bundled `src/striatum/web/static/build/island-*.js` +
  `manifest.sha256` committed.
- ✅ Jinja2 page shells preserved; islands mount into named DOM slots
  (`island-tree-browser`, `island-workflow-chooser`,
  `island-workflow-graph-editor`, `island-code-viewer`).
- ✅ Vitest suites cover each named widget plus the api-client and
  mount helpers; hostile-content escape paths exercised
  (`code-viewer.test.ts`).
- ✅ Prop contract centralised in `shared/types.ts`; templates emit
  matching field names.
- ✅ FRONTEND_DEVELOPMENT.md exists and is detailed (layout,
  prerequisites, mount pattern, prop contract, accessibility
  checklist, supply-chain, testing).
- ✅ HOW_TO_HUMAN walkthroughs cover `/view/`, `/workflows/new`, the
  graph editor, and the syntax-highlighted code viewer with concrete
  toolbar names, keyboard shortcuts, and gating behaviour.
- ✅ RFC 0038 status block updated to `accepted (V1)`; CHANGELOG
  Unreleased entry added.

Out of this lane: CSP, no new Python deps, package-data wheel
coverage, CI bundle-hash gate, `/v1/repo/tree` server-side
path-traversal rejection, snapshot-test parity. Those are systems
and supply-chain angles delegated to the codex/gemini lanes.

## Findings (severity-ranked, ergonomics_dx posture)

### F1 — Workflow chooser advertises "per-lane commands" but ships no input UI (medium, doc-vs-code drift)

`WorkflowChooser.tsx` keeps `FormState.laneCommands: Record<string,
string>` and `buildSpec` (lines 74–94) serializes it into
`lane_commands`. The Vitest suite even exercises lane-command trim
behavior (`workflow-chooser.test.ts:6-27`). But `renderDetailsStep()`
(lines 329–365) renders no input for `laneCommands` whatsoever — only
`workflow_id`, `name`, `scaffold_root`, `artifact_root`, and
`branch_suggestion`. A first-time operator can never populate per-
lane commands through the wizard.

Both halves of the docs promise the feature:

- `docs/HOW_TO_HUMAN.md:692` — "Step 4 fills the required fields
  (`workflow_id`, `name`, `scaffold_root`, `artifact_root`,
  `branch_suggestion`, optional per-lane commands)."
- `WorkflowChooser.tsx:7-9` header docstring describes the six-step
  wizard implicitly carrying lane_commands through.

Recommended fix: add a small repeating per-lane editor to step 4 —
lane id text + space-tokenised command — wired to `form.laneCommands`.
Or remove the field from `FormState`, `buildSpec`, the Vitest case,
and the HOW_TO_HUMAN walkthrough. Today the wizard ships a
documented affordance the UI does not surface.

### F2 — Graph editor inspector hides `task_prompt.path` and `fresh_session_required` (medium, blocks first-write workflow)

`Inspector` (`WorkflowGraphEditor.tsx:188-456`) exposes `id`, `type`,
`title`, `objective`, `role_id`, `lane_id`, `write_scope.{mode,
allowed_paths, forbidden_paths}`, `review_posture` (review-only),
`reviewer_access_scope` (review-only),
`reviewer_context_policy` (review-only),
`required_review_postures` (build-only), `parallel_group`, and
`expected_artifacts`. The palette adds new jobs with
`task_prompt: { path: "" }` (line 160) — but **the inspector has no
field for `task_prompt.path` and no field for
`fresh_session_required`.**

A first-time operator who drags "Implementation" onto the canvas,
fills role/lane/objective, clicks Save, and sees the server reject
with `task_prompt.path is required` has no inspector affordance to
fix it. They must round-trip back to the legacy form editor (still
mounted below) or hand-edit the JSON. RFC 0038 §5d's reason for
existing was to remove that round-trip.

Recommended fix: add a plain text input for `task_prompt.path` to the
inspector (with a small "hint: relative path under `prompts/`"
caption) and a boolean checkbox for `fresh_session_required` next to
`parallel_group`. Both fields exist in `WorkflowJob` (`types.ts:135-
163`) and are part of the workflow validator's required surface for
many real job kinds.

### F3 — `workflow_edit.html` mounts both editors plus a duplicate Save/Cancel cluster (medium, ergonomics regression)

`src/striatum/web/templates/workflow_edit.html` lines 15–28 mount
`island-workflow-graph-editor` *and* render the entire legacy vanilla-
JS form sections (`#edit-roles`, `#edit-lanes`, `#edit-jobs`,
`#edit-edges`, `#edit-cycles`) plus a second `Save`/`Cancel` button
row driven by `static/workflow_edit.js`. From a first-time-user
perspective the page now has two complete editing surfaces and two
Save buttons, both posting to the same endpoint.

The two save flows can race or silently overwrite each other: the
React surface saves via `saveWorkflow()` (`api-client.ts`) using its
own state, while the legacy surface saves via the vanilla JS using
`#workflow-data`. An operator who edits in the React inspector and
then clicks the legacy Save below loses the React-side edits.

The design synthesis explicitly anticipated this: "Legacy workflow
editor fallback is best-effort for one release only; replacement is
acceptable if fallback doubles maintenance." Today it ships double-
mounted by default (`fallback: true` is hard-coded into the template).

Recommended fix options, in order of preference:

1. Drop the legacy form sections from the template entirely (delete
   `#edit-header` through `#edit-cycles`, the duplicate
   `.edit-actions` cluster, and the `static/workflow_edit.js` script
   tag). The React graph editor's own Save/Cancel is sufficient.
2. If the fallback must stay for one release, gate it behind a query
   string (`?legacy=1`) and surface a small banner explaining why
   two surfaces exist. Hide the React Save when fallback is on, or
   vice-versa.

Either is preferable to shipping both at once.

### F4 — Tree browser breadcrumb back-navigation routes to a 404 (medium, broken navigation)

`TreeBrowser.tsx:240-247` renders the breadcrumb as
`<a href={joinUrl(viewBase, seg.path)}>`. With `viewBase="/view/"`,
clicking a parent-directory segment sends the operator to
`/view/<dir>`. But `service.py:_render_view_path` (lines 2275-2277)
returns HTTP 404 with `"directory listing not in V1; view a file
directly"` for any subpath that resolves to a directory. **The
breadcrumb back-navigation through a directory hierarchy is broken
the moment the operator clicks anything other than the root crumb.**

This contradicts the HOW_TO_HUMAN walkthrough (lines 678–680) which
says "The breadcrumb at the top of the page links to every ancestor."

Recommended fix (smallest patch): route the tree-browser breadcrumb
crumbs back to `/view/?path=<segment>` and have `view_tree.html`
read the query parameter into `root_path`. This keeps `/view/<file>`
as the single-file route and avoids needing a `/view/<dir>` server
branch. Update the breadcrumb's `<a href>` accordingly. (The single-
file `/view/<file>` viewer also has no breadcrumb back to its parent
directory — see F11.)

### F5 — Chooser radio cards lack arrow-key roving (medium, accessibility regression vs. design synthesis)

`DESIGN_SYNTHESIS.md` Accessibility Checklist required: "Chooser
radio cards use radiogroup semantics and arrow-key movement."
`WorkflowChooser.tsx` ships the ARIA roles correctly
(`role="radiogroup"`, `role="radio"`, `aria-checked`) but **no
`onKeyDown` handler implements arrow-key roving between cards.** The
native `<button role="radio">` defaults Tab to move between cards,
which conflicts with the WAI-ARIA Authoring Practices for radio
groups (Tab should leave the group; ArrowLeft/Right roams selection).

This affects step 1 (shape) and step 2 (lane set). Step 3 (modifiers)
is correctly modelled as `role="checkbox"` so Tab between options is
appropriate there.

Recommended fix: extract the roving-tabindex pattern from
`TreeBrowser.tsx` (lines 179–228) into `shared/use-roving-tabindex.ts`
and reuse it for the radiogroup steps. Set `tabIndex={selected ? 0 :
-1}` on each radio button and handle `ArrowLeft/Up`/`ArrowRight/Down`
to move selection.

### F6 — Graph editor: destructive job delete with no undo (medium, contradicts design synthesis V1 commitment)

`DESIGN_SYNTHESIS.md` Human-Decision Questions stated: "Graph editor
undo is limited to delete-undo banners in V1." `handleJobDelete`
(`WorkflowGraphEditor.tsx:774-783`) removes the job, its node, and
its incident edges with no recovery surface. The Inspector's "Delete
job" button (lines 446–453) is a single click with no confirmation
prompt. A first-time operator who clicks it loses the structured
fields immediately; the only recovery is the page-level "Cancel"
link, which discards every other unsaved change.

Recommended fix: capture the deleted job and its edges into a small
undo buffer, render a banner ("Deleted `<id>`. Undo · Dismiss") that
restores via `setWorkflow` / `setNodes` / `setEdges`. This was the
explicit V1 affordance.

### F7 — Graph editor: node coordinates never persisted (medium, layout work lost on every reload)

`WorkflowGraphEditor.tsx:8-13` declares "Coordinates are UI-only and
are never persisted to workflow JSON." `jobsToNodes` lays nodes on a
`sqrt(n)` grid every page load. An operator who tunes positions for a
15-job workflow watches the layout reset on every reload. RFC 0038
§5d says "Nodes are draggable;" the implicit operator contract is
that the layout sticks.

Recommended fix (smallest): persist a `{<jobId>: {x, y}}` map in
`localStorage` keyed by the workflow path. This matches the existing
filter / timezone storage pattern in `static/base.js`. A workflow-
JSON-side `ui_layout` field is heavier and would force a schema
discussion; defer that to V1.5 if `localStorage` proves
insufficient.

### F8 — Tree row navigation uses `<div onClick>` instead of `<a href>` (low/medium, lost browser idioms)

`TreeBrowser.tsx:298-313` renders each tree row as a `<div>` with an
`onClick` that calls `window.location.href = ...` for files. Operators
familiar with GitHub / GitLab / VS Code's file trees expect:

- Cmd/Ctrl-click to open in a new tab.
- Middle-click to open in a new tab.
- Right-click → "Copy link address" to share the path.

None of these work because the click target is not an anchor. The
breadcrumb (lines 240–247) uses `<a href>`, so the rest of the page
sets up the expectation that the file rows would too.

Recommended fix: make the file-row label an `<a href>` (the row
container can stay a `div` for keyboard semantics and roving
tabindex), or wrap the row content in an anchor. Suppress the
default click on directories and keep the toggle behavior; let files
use the anchor's native nav.

### F9 — Tree filter only narrows already-loaded entries (low, doc-vs-expectation mismatch)

`TreeBrowser.tsx:251` placeholders the filter as "Filter loaded
entries…" — honest but limited. A first-time operator who types
"SPEC" expects the tree to surface `docs/SPEC.md` even if `docs/`
has not been expanded yet. The HOW_TO_HUMAN walkthrough (line 681)
calls it "the filter input narrows the visible rows by fuzzy
subsequence match" without naming the load-state caveat.

Recommended fix (lowest cost): add a small muted line under the
filter input — "Expand directories to load more entries" — when the
filter is active. A V1.5 follow-up could add a server-side
`/v1/repo/tree?recursive=1` for whole-tree fuzzy search.

### F10 — Tree row drops `size` and `mtime_utc` from the API response (low)

`RepoTreeEntry` includes `size: number | null` and `mtime_utc: string
| null` (`shared/types.ts:48-54`). The `/v1/repo/tree` endpoint
returns both. The tree row only renders `entry.name`. Operators
expect file size and modified time in any modern file browser; the
data is on the wire and discarded.

Recommended fix: append two muted spans (humanized size and a
relative-time label for `mtime_utc`) after the file name. Skip
directories.

### F11 — `/view/<file>` page has no breadcrumb back to the tree (low, ergonomics)

`view_file.html:6` shows only `← Home`. The natural operator flow is
`/view/` → click `docs/` → click `SPEC.md` → finish reading → return
to `docs/`. There is no affordance to return to the parent directory
in the tree browser. The `view_tree.html` template already sets the
breadcrumb pattern; mirror it in `view_file.html` with each ancestor
segment linking back to `/view/?path=<segment>` (depends on F4's
fix).

### F12 — Graph editor inspector renames jobs per-keystroke (low, jarring UX)

`WorkflowGraphEditor.tsx:215-222` — editing the `id` input fires
`onChange` per keystroke, and `handleJobChange` rebuilds nodes and
edges on every character. Typing `review_2` from `r` performs seven
cascading rename operations. The input element does not unmount
(React preserves the controlled input across re-renders), but the
visual fan-out across the canvas is noisy and easy to mis-target if
the user pauses mid-edit and clicks elsewhere.

Recommended fix: switch the `id` field to commit on `onBlur` (or
short debounce) and validate uniqueness before applying. Other
inspector fields (objective, title) don't fan out to graph topology
and can stay per-keystroke.

### F13 — Graph editor has no keyboard path to create edges (low, accessibility caveat)

React Flow's edge creation relies on dragging from a node handle to
another node handle — mouse-only. Users on keyboard-only or assistive
input have no way to author edges. The textual fallback region
(`graph-editor-textual`, lines 856–863, visually hidden via
`theme.css`) reads the graph for screen readers but does not expose
an authoring surface.

Recommended fix (V1.5): add an "Add edge" affordance to the inspector
that opens a small dialog with two `<select>`s (from / to) and a
verdict dropdown, then dispatches `addEdge` via the same path as the
React Flow connect handler.

### F14 — Code viewer: silent fallback when grammar / size guard trips (low)

`CodeViewer.tsx:165-182` falls back to `plainTextHtml` when the byte
size exceeds 5 MB or the detected language isn't in
`SUPPORTED_LANGS`. The toolbar updates the language label to
`"plaintext"` but offers no hint that highlighting was deliberately
skipped. A first-time operator with a `.kln` file (or a 6 MB
`workflow.json`) sees plain text without explanation.

Recommended fix: add a small muted line ("Highlighting skipped:
files over 5 MB" / "No highlighting available for `.kln`") next to
the language label.

### F15 — Code viewer: no-extension filenames never highlight (low)

`detectLanguage` (`CodeViewer.tsx:72-81`) only inspects the substring
after the last `.`. Files without an extension — `Makefile`,
`Dockerfile`, `.bashrc`, `Procfile` — fall to plaintext. Common in
real repos.

Recommended fix: add a small `FILENAME_LANG_MAP` (`Dockerfile` →
`bash`-ish via shell grammar, `.bashrc` → `bash`) and consult it
before the extension fallback. Polish-level; defer to V1.5 if
priorities are tight.

### F16 — Chooser step-4 details: missing field never named (low, discoverability)

`WorkflowChooser.tsx:157-176` computes `canAdvance` on truthy
`workflowId.trim()`, `scaffoldRoot.trim()`, and `artifactRoot.trim()`.
When any is blank, "Next" greys out with no inline message naming
the missing field. The `required` HTML attribute only fires the
browser-native bubble on form submit, which the wizard never issues.

Recommended fix: render an inline "Required: workflow_id /
scaffold_root / artifact_root" hint above the disabled Next button,
or per-field `aria-invalid` + an `aria-describedby` tip. Cheap, makes
the gate visible.

### F17 — Chooser confirm-success banner never visible (low)

`WorkflowChooser.tsx:441-444` renders an `island-success` panel when
`writeStatus.status === "ok"`, but `performWrite` (lines 205–218)
calls `dialogRef.current?.close()` and then `window.location.href = …`
before React commits the success state. The success panel is
unreachable.

Recommended fix: defer the navigation by 500 ms (with a "Workflow
written. Opening…" banner) or skip the auto-redirect and present a
"View workflow" link.

### F18 — Tree-row icons are emoji (low, possible a11y noise)

`TreeBrowser.tsx:306-311` renders `📁` and `📄` inside spans marked
`aria-hidden="true"`. Most screen readers honor `aria-hidden` on
emoji nodes, but several macOS VoiceOver / Edge Narrator combinations
still announce "folder open" / "page facing up." Since the row's
adjacent name is the actual label, the duplication is mild noise;
flagging only because the design synthesis emphasized AA-clean
accessibility.

Recommended fix: replace with inline SVG icons referencing
`currentColor` (matches the `workflow_detail.html` Edit button SVG
pattern). Or set `role="presentation"` alongside `aria-hidden`.

## Documentation observations

- `docs/FRONTEND_DEVELOPMENT.md` is high-quality and complete:
  prerequisites, project layout, make targets, mount pattern, prop
  contract, closed vocabularies, accessibility checklist, bundle-hash
  workflow, supply-chain posture, and testing. Honest about the
  manual cross-language coordination ("There is no runtime contract
  validator"). The only drift to flag is the implication (line
  144-146) that "tests on each side cover the most common drift" —
  Python web tests check that the mount-point IDs and script tags
  appear in HTML; they do not parse `data-props` JSON against
  `types.ts`. A small Python contract test that round-trips each
  template through `_jinja_env().get_template(...).render(...)`,
  extracts `data-props`, and validates the JSON shape would close
  this loop and catch any future genuine drift.
- `docs/HOW_TO_HUMAN.md` walkthroughs are concrete and specific —
  they name keyboard shortcuts (`ArrowUp/Down`, `Home/End`), toolbar
  buttons (Copy / Wrap / Raw), the Shiki size cap (5 MB), the
  collapse threshold (500 lines), the `<dialog>` confirmation gate,
  and the React Flow palette block list. F1 (lane_commands), F2
  (`task_prompt.path`), and F4 (breadcrumb routing) are the three
  documented behaviors that today's UI does not actually deliver.
- `src/striatum/web/frontend/src/shared/types.ts` correctly serves as
  the single source of truth; the closed-vocabulary exports
  (`ALLOWED_REVIEW_POSTURES`, `JOB_TYPES`, `EDGE_VERDICTS`,
  `REVIEWER_ACCESS_SCOPES`, `REVIEWER_CONTEXT_POLICIES`,
  `WRITE_SCOPE_MODES`) match the workflow validator vocabulary.

## Test-coverage observations (Vitest side)

The Vitest suite exercises the helper surfaces well — api-client
envelopes, mount JSON / text payload helpers, tree-browser path /
fuzzy / URL helpers, chooser spec assembly + modifier compatibility,
graph editor JSON round-trips + palette vocabulary parity, code
viewer language detection + escape + line-number injector + size /
collapse constants. Gaps worth filing as follow-ups, not blockers:

- No test asserts the chooser actually renders an input for
  `lane_commands` (F1).
- No test asserts the inspector renders a `task_prompt.path` input
  (F2).
- No test asserts arrow-key roving between chooser radio cards (F5).
- No test asserts an undo banner after deleting a job (F6).
- No test pins the dialog's tab-trap / focus-restore behavior. The
  native `<dialog>` element provides browser-default focus
  management, but a smoke check would lock the contract.

## Accessibility audit (compact)

| Surface | Keyboard | ARIA | Focus | Verdict |
| --- | --- | --- | --- | --- |
| Tree browser | ArrowUp/Down/Left/Right/Home/End/Enter wired (TreeBrowser.tsx:179-228); roving tabindex via `aria-selected` + `tabIndex={active ? 0 : -1}` | `role=tree`, `role=treeitem`, `aria-expanded`, `aria-level`, breadcrumb `nav`, polite live region | Active row receives `el?.focus()` (line 184) | Solid |
| Workflow chooser | Tab moves through steps; Enter on Next/Back; modifier toggle on Enter | `radiogroup`, `radio`, `checkbox` roles correct; `aria-current="step"` on step list; dialog `aria-labelledby` set | Native `<dialog>.showModal()` provides browser-default focus trap and Esc-close | F5 (no arrow-key roving), F17 (success banner unreachable) |
| Graph editor | React Flow defaults (Tab into canvas, arrow-key pan); palette + inspector all keyboard-reachable | Fieldsets with `aria-label` for radio groups; chip-remove buttons have `aria-label`; textual summary `role="region"` and visually hidden via `theme.css` | No focus restore after Delete; node selection via mouse only | F6, F12, F13 caveats |
| Code viewer | Toolbar buttons Tab-reachable; Copy/Wrap/Raw have `aria-label`; Wrap is `aria-pressed` toggle | `role="toolbar"`; polite live region for "Copied"; line-number gutter `aria-hidden`; Raw `rel="noopener noreferrer"` | Default focus management — no focus restore after Copy | Solid for V1 |

`prefers-reduced-motion` is honored in `theme.css` lines 454-460 (the
React Flow nodes/edges `transition` and `animation` set to `none`).

## Suggested ordered fixes

1. **F1** — add the lane-commands editor (or remove the field). Closes
   doc-vs-code drift.
2. **F2** — add `task_prompt.path` and `fresh_session_required` to the
   inspector. Closes the first-write gap.
3. **F3** — drop or gate the legacy form editor in `workflow_edit.html`
   so only one Save flow is live at a time.
4. **F4 + F11** — re-route tree-browser breadcrumb to
   `/view/?path=<segment>` and add a parent-directory breadcrumb to
   `view_file.html`.
5. **F5** — share a roving-tabindex helper from the tree browser into
   the chooser radiogroups.
6. **F6 + F7** — delete-undo banner; persist node coordinates in
   `localStorage`.
7. **F8 + F9 + F10** — tree-row anchor + filter caveat copy + size /
   mtime columns.
8. **F12** — debounce inspector `id` rename to `onBlur`.
9. **F13 + F14 + F15 + F16 + F17 + F18** — V1.5 polish.
10. Optional: land a small Python `tests/test_web_island_props.py` that
    parses each rendered `data-props` and validates required keys
    against a TypedDict mirror of `shared/types.ts`. Single file,
    high-leverage drift catch.

## Net judgement

The build implements every named V1 feature; the contract surface
between Jinja2 templates and React islands is well-typed; the Vitest
suite covers the testable seams; the documentation is detailed and
mostly honest; and the dark-mode / contrast inheritance from
`base.css` holds. The findings above (F1–F4 medium, the rest low)
are concrete but localized and can be cleared in a single follow-up
patch — none requires a rewrite. Recommending
**accept_with_findings** so V1 ships and the patch lands as a V1.1
polish round.

# RFC 0024: Workflow Browser And Visual Builder

Status: accepted (V1)
Date: 2026-05-09
Context:
RFC 0007 (workflow visualization, accepted) — `workflow_graph_data`
provides the topology this RFC's SVG previews reuse,
RFC 0016 (dashboard dependency graph, accepted) — same layered
layout idea applied to file-on-disk workflows rather than running
runs,
RFC 0022 (web UI redesign, accepted V1) — Jinja2 multi-page foundation,
SVG dependency graph, dark-mode CSS palette,
RFC 0023 (web chat + view + browse, accepted V1+V1.5) — `/view/<path>`
endpoint pattern, Markdown rendering, chat tools (the
chat surface naturally pairs with this — "ask the model to scaffold
a workflow, then edit it in the visual builder"),
`docs/SPEC.md` § "Workflow Schema" (validator rules),
`src/striatum/workflow.py` — `validate_workflow`, `workflow_graph_data`,
`workflow_init` (existing CLI scaffold helper).

## Problem

Authoring a `workflow.json` from scratch is the highest-friction part
of using striatum, and it's the surface that determines whether a new
operator gets value in their first hour.

Today the path is:

1. Read `docs/SPEC.md` § "Workflow Schema" (long).
2. Find a similar example under `examples/` or `docs/dogfood/<id>/`
   (they're not centrally indexed; you grep for them).
3. Copy a workflow.json into your target repo.
4. Hand-edit job ids, role ids, lane ids, edges, expected_artifacts,
   write_scope paths.
5. Run `striatum workflow validate <path>` to find the typos. (Often
   several round-trips.)
6. Run `striatum run prepare`, find more issues, iterate.

Each step is manual text-editing in a separate program. The workflow
graph — the *load-bearing* artifact — only becomes visible after
`workflow validate` passes and you run `workflow graph`. Authors who
think visually are designing in their heads and translating to JSON;
authors who are new to striatum's vocabulary don't know which fields
matter.

There's also no central place to *browse* the workflows already in
the repo. Operators inspecting why a run behaved a certain way often
want to see the workflow that produced it; today they navigate via
file tree to `docs/dogfood/<id>/workflow.json` and read JSON.

This RFC adds two paired surfaces to the web UI:

- **`/workflows/`** — a *browse* surface that lists every
  `**/workflow.json` (and `examples/**/workflow.json`,
  `docs/dogfood/**/workflow.json`) in the target repo, with
  validation status, SVG graph preview, and metadata at a glance.
- **`/workflows/edit/<path>`** (V1.5) — a *visual builder* with form
  widgets to add/edit roles, lanes, jobs, edges, posture fields,
  expected-artifacts blocks. Form-driven (no client-side graph
  editor library); the SVG re-renders server-side on every save;
  validation runs per save.

V1 ships only the browse surface. V1.5 adds editing. Splitting keeps
V1 small (read-only, ~3-4 days) and lets the editor's UX get scoped
properly without blocking the browse half.

## Goals

- **`/workflows/`** index page lists every workflow in the repo with:
  - Repo-relative path
  - `workflow_id` + version
  - Validation status (valid / `WorkflowError` summary)
  - Number of jobs / lanes / roles
  - SVG graph thumbnail (small, click-to-expand)
- **`/workflows/<path>`** detail page shows:
  - Full SVG dependency graph (state-colored if a recent run used
    this workflow; else neutral)
  - Tabular jobs / lanes / roles / cycles listing
  - Validation result inline (or error trace if invalid)
  - "Open in editor" link (V1.5)
  - "Prepare run from this workflow" button (mutation-gated;
    V1.5+)
- **`/workflows/edit/<path>`** (V1.5) form-driven visual builder:
  - Job list with add/remove/reorder
  - Per-job form (id, type, role, lane, write_scope.allowed_paths,
    expected_artifacts, review fields, posture fields, required_postures)
  - Edge list with from/to/on selectors
  - Lane + role tables
  - Save action runs `workflow validate` server-side; surfaces errors
    inline; refuses save on invalid (or saves to a `.draft` path).
- **Reuse RFC 0022 V1's SVG renderer** for graph thumbnails + full graphs
  (no new client-side graph library).
- **No new runtime deps.** Jinja2 + markdown-it-py already present
  from RFC 0022 / RFC 0023; the form-driven editor uses HTML forms
  and POST endpoints.

## Non-Goals

- **No drag-and-drop graph editor.** V1.5 ships a form-driven editor:
  the operator types names, the SVG re-renders server-side. Drag-and-
  drop would require a client-side graph library (Cytoscape, ReactFlow,
  d3-force) — V2 territory if dogfood evidence shows operators want it.
- **No live multi-user collaboration.** Same single-operator model as
  RFC 0022/0023 — the edit endpoint takes a lock-by-write semantic
  (last writer wins; striatum doesn't currently track concurrent edits).
- **No workflow-running from the UI** (V1). "Prepare run from this
  workflow" mutation is V1.5; gated by `--allow-mutations`.
- **No git integration in the editor.** The editor saves files; it does
  not commit them. The operator commits via their existing git
  workflow.
- **No template marketplace** or external workflow imports. Operators
  start from `examples/` (they're already in the tree) or from scratch.
- **No JSON-Schema-driven form generation in V1**. The form layout is
  hand-rolled per workflow field. JSON-Schema-driven forms (where
  changes to `validate_workflow` automatically update the editor)
  are a V1.6+ pursuit.

## Proposal

### V1 — Browse-only surface (3 landable steps)

#### Step 1. Workflow discovery + list page

A new server-side helper `striatum.web.workflows.discover(repo) -> list[dict]`
walks the repo for `**/workflow.json` files (excluding `.git/`,
`.striatum/`, `node_modules/`, `__pycache__/`). For each:

- Compute repo-relative path.
- Read + JSON-parse; capture parse errors.
- Run `validate_workflow(...)` in a try/except; capture error message
  if invalid.
- Extract `workflow_id`, `workflow_version`, count jobs / lanes / roles.

`GET /workflows/` renders `workflows_index.html` with this list:

```
| Path                                        | ID                 | Status   | Jobs | Lanes | Roles |
|---------------------------------------------|--------------------|----------|------|-------|-------|
| docs/dogfood/022/workflow.json              | dogfood-022-...    | valid    | 5    | 2     | 5     |
| docs/dogfood/021/workflow.json              | dogfood-021-...    | valid    | 9    | 2     | 5     |
| examples/code-change-flow/workflow.json     | code-change-flow   | valid    | 3    | 1     | 2     |
| ...                                         | ...                | ...      | ...  | ...   | ...   |
```

Each row links to `/workflows/<repo-relative-path>` (the path is
URL-encoded to handle slashes).

The index defaults to listing **all** workflows. Optional `?filter=examples`
or `?filter=dogfood` query parameter narrows.

#### Step 2. Workflow detail page

`GET /workflows/<path>` renders `workflow_detail.html`:

- Header: workflow_id + version + status pill.
- Full SVG graph via `striatum.web.graph_svg.render_run_graph(workflow,
  node_states={})` — no run-state colors since this is a file, not a run.
- Tabular sections: **Jobs**, **Lanes**, **Roles**, **Cycles**,
  **Edges**.
- Each job row: id, type, role, lane, expected artifacts, posture
  fields (RFC 0018), required_review_postures (RFC 0018).
- "Validation" section: green checkmark + "valid" if it passed, OR
  red box with the `WorkflowError` message.

The path validation mirrors `/view/<path>`: `..`, leading `/`, null
bytes, symlink-escapes, `.git/` / `.striatum/` all return 400/404.

#### Step 3. Nav + chat-tool addition

`base.html` top-nav adds "Workflows" link.

A seventh chat tool (RFC 0023 V1.5 closed set) — `list_workflows()` —
returns the same data the browse surface lists. The model can now
discover workflows for the operator's chat questions ("which workflow
produced run X?"). No new RFC needed; the tool slot is a small
extension of `chat_tools.py`.

### V1.5 — Visual builder (4 landable steps)

V1.5 lands as a separate dogfood after V1 settles.

#### Step 1. Edit page form scaffold

`GET /workflows/edit/<path>` renders `workflow_edit.html`:

- Top: workflow_id + version (editable text inputs).
- "Roles" panel: name + definition_path columns; add/remove buttons.
- "Lanes" panel: lane_id + adapter + capabilities; add/remove.
- "Jobs" panel: collapsible per-job card. Within each card: id, type,
  role, lane, objective, write_scope, expected_artifacts.
- "Edges" panel: from, to, on (+ optional `requires_verdict`).
- "Cycles" panel: from, to, on_verdict, max_iterations.
- "Save" button at the bottom.

#### Step 2. Save action + validation

`POST /workflows/edit/<path>` reads the form fields, assembles a
`workflow.json`-shaped dict, runs `validate_workflow(...)`. On success:
writes the file (mutation-gated; `--allow-mutations` required). On
validation error: renders the same edit page with errors inline next
to the offending field.

For unknown paths or paths that exist as directories: 400.
For paths that don't exist yet: writes them (creates intermediate dirs).

#### Step 3. Per-job posture + required-posture widgets

Per RFC 0018: each `type: "review"` job has a `review_posture` dropdown
(closed set of nine + custom: text input). Each `type: "build"` job has
a multi-select for `required_review_postures`.

#### Step 4. Reload + re-validate UX

After save: redirect to `/workflows/<path>` (the detail page) with a
flash banner. The detail page re-renders the SVG with the saved state.

### V2 (out of scope here)

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- "Run this workflow now" full lifecycle button (from prepare → start
  → driving — significant scope).
- AI-assisted workflow scaffolding (chat tool that *writes*
  workflow.json — that's a mutation tool, requires per-tool gating).

## Acceptance Criteria

- `striatum --repo $REPO serve --web` exposes
  `https://<host>/workflows/` with a list of all workflows in the repo.
- Each row's status is "valid" or shows the first 200 chars of the
  validation error.
- Each row's SVG thumbnail renders as inline SVG.
- `/workflows/<path>` shows the full SVG + tabular sections.
- A workflow with `WorkflowError` renders the error inline, doesn't
  500.
- Path traversal (`..`, absolute, symlink) returns 400.
- `.git/` and `.striatum/` paths return 404.
- The chat surface can call the new `list_workflows` tool and receive
  the discovered list.
- CSP unchanged; no new runtime deps.
- Tests at `tests/test_web_workflows.py` cover discovery, detail
  rendering, invalid-workflow rendering, path safety.
- `make lint`, `make typecheck`, full `make test` pass.

## Open Questions

- **Q1: Workflow-discovery scope.** V1 walks the entire repo. For very
  large target repos, this could be slow. *Recommendation*: cache the
  list per-server-process, invalidate on file mtime change. V1.5 if
  performance is real.
- **Q2: SVG thumbnail size for the index page.** Layered graphs of 9-
  job workflows render at ~600×500 px in the run-detail view; for an
  index thumbnail we want ~200×150 px. *Recommendation*: render full
  size with CSS `width: 200px; height: auto;` to avoid a separate
  thumbnail-render path.
- **Q3: Edit-page concurrency.** What if two operators (or the same
  operator in two tabs) edit the same workflow? *Recommendation*: V1.5
  ships last-writer-wins. V2 may add a per-file lockfile under
  `.striatum/scratch/edit-<file-hash>/` for advisory locking.
- **Q4: Edit-page draft persistence.** Should `Save` always overwrite,
  or save to a `.draft.json` first? *Recommendation*: overwrite when
  validation passes; reject (with inline errors) when validation fails.
  Drafts that don't validate stay in the operator's browser; refresh
  loses them. V2 may add a draft endpoint.
- **Q5: List-workflows chat tool budget.** A repo with 1000 workflow
  files would return a huge tool result. *Recommendation*: cap at
  100 entries; truncation marker. (Same pattern as `list_dir`.)
- **Q6: How does the visual builder express posture coverage?**
  Per RFC 0018, a build job's `required_review_postures` interacts with
  reachable review jobs' postures. The editor should refuse save when
  the reachability gate fails, with a clear error pointing at the build
  job. *Pinned in V1.5 step 3.*

## Implementation Path

### V1 ships as v1.14.0

Three steps land sequentially in dogfood-023:

1. **Step 1 (discovery + list)**: new
   `src/striatum/web/workflows.py` (~120 LoC); new
   `workflows_index.html` template; service.py route table extension;
   tests for discovery + list-render.
2. **Step 2 (detail page)**: new `workflow_detail.html`; route
   `/workflows/<path>`; SVG embed via existing `graph_svg`; tests for
   detail render + invalid-workflow render + path safety.
3. **Step 3 (nav + chat tool)**: `base.html` adds Workflows nav;
   `chat_tools.py` adds `list_workflows` to the closed set; tests for
   nav + tool dispatch.

### V1.5 ships as v1.15.0 (separate dogfood)

Four steps:

1. Edit form scaffold (`workflow_edit.html`).
2. Save + validation round-trip.
3. Per-job posture + required-posture widgets.
4. UX polish: flash banner, redirect-after-save, error inline rendering.

## Domain Modeling

This RFC adds three value objects:

- **workflow file** — a value object naming a discovered
  `workflow.json` on disk: `(path, validation_status, metadata)`.
  Identity is the absolute resolved path; equality is by path.
  Constructed at discovery time; never mutated in flight (the file is
  re-discovered if it changes on disk).
- **validation status** — closed enum `valid | parse_error |
  workflow_error` with an optional `message` string when not `valid`.
- **edit draft** (V1.5) — a not-yet-saved form state. Lives only in
  the browser tab. V1 doesn't need to model this; V1.5 does.

The runner's bounded context is *unchanged*. Workflow files have
always lived on disk and been read by `striatum workflow validate`,
`run prepare`, etc. This RFC adds a *visualization* and *editing*
layer on top — not a new state store, not a new mutation surface. The
write path goes through the operator's filesystem; the runner doesn't
auto-commit, doesn't mirror to SQLite, doesn't subscribe to changes.

Per `docs/DDD.md § "Adding to the model"`:

1. **Glossary** — `docs/UBIQUITOUS_LANGUAGE.md` adds entries for
   `workflow file`, `validation status`, `edit draft`.
2. **Pattern** — three value objects (above).
3. **Validator** — `striatum.web.workflows.discover` runs
   `validate_workflow` per file; the existing validator is the source
   of truth. V1.5 edit-save runs the same validator.
4. **Surface** — `/workflows/`, `/workflows/<path>`, V1.5
   `/workflows/edit/<path>`. The chat tool `list_workflows`.
5. **Citation** — DECISION_LOG D076 cites the new entries when V1
   lands.

## Relationship To Other RFCs

- **RFC 0007** (workflow visualization, accepted) — `workflow_graph_data`
  is reused for the SVG layouts.
- **RFC 0022** (web UI redesign, accepted V1) — V1 of this RFC extends
  the Jinja2 template tree + the `striatum.web.graph_svg` SVG
  renderer; same SSR pattern.
- **RFC 0023** (web chat + view + browse, accepted V1+V1.5) — the
  chat tool `list_workflows` is a small extension of the V1.5 closed
  tool set. The visual builder pairs with the chat: "ask the model to
  scaffold a workflow, then refine it in the editor."
- **RFC 0018** (review postures, accepted V1+step 3) — V1.5's edit
  form respects posture + required_review_postures fields; the
  validator's reachability gate becomes a UI error message.
- **RFC 0021** (DDD layout scaffold, accepted V1+V1.5) — unrelated;
  `striatum init --with-ddd-layout` doesn't scaffold workflow files
  (those are operator-authored, not project-template-shipped).

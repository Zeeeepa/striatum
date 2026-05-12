# RFC 0038: Web UI Feature Additions and Frontend Toolchain

Status: proposed
Date: 2026-05-13
Context:
[`RFC 0013`](0013-local-web-ui.md),
[`RFC 0022`](0022-web-ui-redesign.md),
[`RFC 0023`](0023-web-chat-and-browse.md),
[`RFC 0024`](0024-workflow-browser-and-builder.md),
[`RFC 0034`](0034-workflow-generator-and-template-catalog.md) §9,
[`RFC 0037`](0037-web-ui-ergonomic-improvements.md),
[`docs/DECISION_LOG.md`](../DECISION_LOG.md) (D073, D082, D084, D092),
`src/striatum/service.py`,
`src/striatum/web/`

## Problem

The web UI shipped progressively over RFCs 0013/0022/0023/0024/0036/0037
is functionally complete for steady-state observability: every CLI verb
has a UI surface, dark mode and filters are in place, the chat surface
exposes the mutation chat tools, the visual workflow builder accepts a
form-driven edit. But an operator UI walkthrough on 2026-05-13 (via the
tailscale-bridged dev instance) surfaced five concrete gaps that RFC
0037's polish pass did not address:

1. **The Edit link on the workflow detail page is muted** (`class="muted"`)
   and easy to miss next to the prominent "Run this workflow now" button.
   A first-time operator wanting to inspect or modify a workflow has to
   hunt for the affordance.
2. **There is no top-level repo / file browser.** The `/view/<path>`
   route from RFC 0023 V1.5 is a read-only Markdown-rendering file
   viewer, but it requires a specific path. There is no `/view/` landing
   with a tree-style file browser, breadcrumbs, or click-through. Every
   other operator-facing dashboard tool (Argo Workflows, GitHub Actions,
   GitLab CI, Vercel) has this as a first-class surface.
3. **There is no "New workflow" web entry.** RFC 0034 V1 shipped the
   CLI generator and the local API endpoints (`POST /workflows/generate/preview`,
   `POST /workflows/generate`), but the §9 web chooser UI was explicitly
   deferred. Operators creating their first workflow must use the CLI;
   the chat-assisted scaffolding tool from RFC 0036 partially fills the
   gap but assumes the chat surface is configured.
4. **The workflow editor is text-field driven.** RFC 0024 V1.5 nominally
   shipped a "form-driven visual editor with widgets" but operator
   feedback is that radio buttons, dropdowns, mouse-movable affordances,
   and drag-drop graph editing are still missing. The current editor is
   structured form fields over the workflow JSON schema — better than
   raw JSON but still requires typing into text boxes for every field.
5. **There is no syntax-highlighted code viewer.** `/view/<path>` renders
   `.md` files with inline Markdown but renders other file types (json,
   py, sh, yaml) as plain `<pre>`. Operators inspecting workflow files,
   source code, or build artifacts get no syntax highlighting.

The common thread across these gaps is that the feature ambition of the
UI has grown past where pure vanilla JS scales cleanly. Each gap can be
filled with vanilla JS, but the combined surface (tree browser + syntax
highlighter + drag-drop workflow editor + chooser wizard + future
features) strains the architecture established by RFC 0022 V1.

D092 supersedes D073's implicit "no node toolchain" rule. RFC 0038 picks
the specific toolchain, framework, and deployment shape, then ships the
five named feature additions over the new foundation.

## Goals

- Land a modern frontend toolchain (Vite + a framework + TypeScript +
  npm-managed dependencies) as a contributor-side development dependency.
  Operator-side install stays pip-only; bundled output ships in the
  wheel.
- Pick a specific framework + framework-aware libraries based on the
  feature ambition (tree browser, syntax highlighting, drag-drop graph
  editor, chooser wizard).
- Ship the five feature additions:
  - Promote the workflow-detail Edit link to a primary button.
  - Add a top-level `/view/` repo file browser with tree + click-through.
  - Add a `/workflows/new` chooser wizard calling `POST /workflows/generate/preview`
    then `POST /workflows/generate` with operator confirmation.
  - Upgrade the visual workflow editor to a drag-drop graph editor with
    radio buttons, dropdowns, and rich field widgets.
  - Add syntax highlighting to `/view/<path>` for non-Markdown file
    types.
- Keep the deployment shape progressive: existing server-rendered Jinja2
  pages stay; new component-heavy surfaces are framework "islands"
  mounted into specific page slots. Not a full SPA conversion.
- Preserve CSP, JSON API, SSE event feed, mutation gate, local-first
  ethos.

## Non-Goals

- Full SPA conversion. The existing multi-page server-rendered
  architecture from D073 stays; framework usage is island-mounted, not
  page-replacing.
- Hosted-mode UX (D058 + D083).
- Mobile-first responsive redesign (separate RFC).
- Operator-side node installation. Operators install via `pip install
  striatum-orchestrator` and get the bundled assets via the wheel; no
  node required at runtime.
- A second web UI framework on top of the chosen one. Pick once.
- Replacing the Jinja2 server-side rendering entirely. Jinja2 still
  renders page shells; framework components mount into slots.
- New auth / multi-user / multi-tenant surfaces.

## External Prior Art

Modern Python web projects with a JavaScript-framework surface follow
a few well-established patterns. The useful ones for Striatum:

- **Django + Vite + React/Vue (Inertia.js or component islands)** —
  Python server rendering with framework islands. Vite handles bundling
  + dev hot-reload. Django ships bundled output via `collectstatic`.
  The pattern is operator-tested and matches Striatum's
  server-rendered-pages-plus-progressive-enhancement posture.
- **FastAPI + Vite + Svelte/Solid** — similar shape with smaller
  framework footprints. Same islands pattern.
- **Astro (Islands architecture)** — Astro popularized the term but is a
  Node-side framework; not a direct fit for a Python-backend project.
  The concept (server-render-most, hydrate-some) is what we adopt.
- **Argo Workflows UI** — React + react-flow for the dependency graph
  editor; full SPA. Striatum has stayed multi-page so does not adopt
  the full SPA pattern.
- **GitHub Actions / GitLab CI** — server-rendered with progressive JS
  enhancement; no framework islands explicitly. Striatum's existing
  shape matches this; the change is adding islands where complexity
  justifies it.

## Proposal

### 1. Toolchain selection

**Vite** as the bundler:
- Fast dev server with hot module replacement.
- Pre-configures TypeScript + JSX without custom config.
- Single-binary build output suitable for committing into the wheel.
- Industry-standard; not a niche choice.

**React + TypeScript** as the framework + type system:
- Largest framework ecosystem for the specific libraries Striatum needs
  (react-flow for the workflow graph editor; highlight.js or
  shiki-react for syntax highlighting; well-supported file-tree
  components).
- TypeScript catches the API-shape mismatches that have been the
  primary class of UI bugs in past iterations (status pill class names
  drifting, posture vocabulary changes, verdict enum additions).
- React's "component island" deployment is well-trodden via libraries
  like react-mount-anywhere or simple `createRoot()` calls into named
  DOM slots.

**Alternative considered but not selected:**
- **Svelte/SvelteKit** — smaller bundle, simpler reactivity, but
  svelte-flow (graph editor) is less mature than react-flow.
- **Vue 3** — small ecosystem mismatch with the graph-editor library
  ecosystem.
- **Solid** — smallest reactivity engine, but library ecosystem still
  growing; risk of finding-yourself-writing-your-own for niche needs.
- **Preact + JSX-via-htm** — D073 alignment but the graph editor and
  syntax highlighting libraries are React-targeted.

### 2. Project layout

```
src/striatum/
  web/
    static/            # current vanilla JS + CSS (stays for existing pages)
    templates/         # current Jinja2 templates (stays)
    frontend/          # NEW: contributor-side TypeScript source
      package.json
      vite.config.ts
      tsconfig.json
      src/
        islands/
          tree-browser/
          workflow-chooser/
          workflow-graph-editor/
          code-viewer/
        shared/
          api-client.ts
          types.ts
          theme.css
        main.ts          # entry point; registers island mount points
    static/build/      # NEW: Vite-bundled output, committed to repo
      island-tree-browser.js
      island-workflow-chooser.js
      island-workflow-graph-editor.js
      island-code-viewer.js
      island-shared.js
      style.css
```

The Vite build emits one bundle per island (entry point) plus a shared
chunk. Bundles are committed to `src/striatum/web/static/build/` so the
PyPI wheel ships them as package data. Operators never see the
contributor-side `frontend/` tree.

### 3. Build integration

**Contributor-side:**

- New `make ui-install` runs `npm install` inside `frontend/`.
- New `make ui-build` runs `npm run build` to produce the bundled output.
- New `make ui-dev` runs `vite dev` for hot-reload during UI development.
- Existing `make install` does NOT change. UI rebuild is contributor-only.

**CI:**

- Existing CI matrix gains a node setup step (node 22 LTS).
- `make ui-build` runs as part of CI; the resulting bundle hash is
  compared against the committed hash. If they differ, CI fails with a
  reminder to rerun `make ui-build` and commit.
- The committed bundle is the authoritative artifact; CI does not
  rebuild and ship a different one.

**Wheel:**

- `pyproject.toml` `package-data` already includes `src/striatum/web/static/`
  (per RFC 0013 V1); add `src/striatum/web/static/build/` to ensure the
  bundled output ships.
- No runtime node dependency.

### 4. Island mounting pattern

Each Jinja2 template that needs framework components includes:

```html
<div id="island-tree-browser" data-props='{"rootPath": "/"}'></div>
<script type="module" src="/static/build/island-tree-browser.js"></script>
```

The island's `main.ts` reads `data-props`, calls `createRoot()` on the
container, and renders the React tree. Other parts of the page stay
server-rendered Jinja2.

This means: a workflow detail page is Jinja2-rendered for header /
navigation / meta, and embeds React islands for the graph editor and
the file tree. The page is not an SPA; framework usage is scoped.

### 5. Five feature additions

#### 5a. Promote workflow-detail Edit link

`src/striatum/web/templates/workflow_detail.html`:
- Move `Edit` from a `class="muted"` link to a `class="secondary-button"`
  button next to "Run this workflow now".
- Add an icon (inline SVG, `currentColor`).

This is the only non-island change; no framework needed.

#### 5b. Top-level `/view/` tree file browser

New island: `frontend/src/islands/tree-browser/`.

- Server route: extend `/view/` (no path) to render a Jinja2 page that
  mounts the tree-browser island.
- Server API: new `GET /v1/repo/tree?path=<rel>` returns
  `{entries: [{name, kind: "file"|"dir", size}]}` for a directory.
- React component: collapsible tree with lazy directory expansion; click
  a file → navigate to `/view/<path>` (existing single-file route);
  click a directory → expand.
- Library: pick a small one (e.g. `@radix-ui/react-tree-view`) or write
  ~80 lines of TSX.

#### 5c. `/workflows/new` chooser wizard

New island: `frontend/src/islands/workflow-chooser/`.

- Server route: new `/workflows/new` renders a Jinja2 page that mounts
  the chooser island.
- Server API: existing `GET /workflow-templates` (RFC 0034 V1) for the
  catalog list and `GET /workflow-templates/<id>` for shape/lane-set
  details.
- React component: step-by-step wizard:
  1. Pick shape (radio cards with shape `summary` + `recommended_for`).
  2. Pick lane set (radio cards filtered by `default_lane_sets`).
  3. Pick lane modifiers (multi-select with compatibility-matrix
     validation).
  4. Fill required fields (workflow_id, name, scaffold_root,
     artifact_root, branch suggestion, lane commands).
  5. Preview: call `POST /workflows/generate/preview`, render the
     `GeneratedWorkflow` envelope (workflow JSON, file list, graph).
  6. Confirm + Save: call `POST /workflows/generate` with `confirm_write:
     true`.
- Same operator-confirmation gate as the chat tools from RFC 0036; the
  React component cannot bypass `confirm_write`.

#### 5d. Drag-drop workflow graph editor

New island: `frontend/src/islands/workflow-graph-editor/`.

- Replaces (or augments) the existing form-driven `workflow_edit.html`
  template's editor section.
- Library: **react-flow** (the most-mature React graph editor; widely
  used by Argo Workflows UI and similar).
- Nodes are draggable; edges are clickable to delete or change `on`
  verdict; new nodes are added via a palette (one per block kind from
  RFC 0034 §5 closed vocabulary).
- Per-node panels expose the structured fields (role, lane, write
  scope, expected artifacts) with proper widgets (dropdowns for
  enums, multi-select for tags, file picker for paths).
- Save calls existing workflow-edit POST endpoint with the produced
  JSON; existing validation runs server-side.

#### 5e. Syntax-highlighted `/view/<path>` code viewer

New island: `frontend/src/islands/code-viewer/`.

- Server route: existing `/view/<path>` template mounts the code-viewer
  island when the file is not Markdown.
- Library: **shiki** (modern, accurate, small grammar bundles). Vendor
  the grammar bundles for json, py, ts/js, sh, yaml, toml, md, sql.
- Render: line numbers + collapsible-by-default for files > 500 lines +
  copy-to-clipboard button + raw-link.

### 6. Documentation

- `docs/SPEC.md` — add a section on the frontend toolchain.
- `docs/UBIQUITOUS_LANGUAGE.md` — add "frontend island", "tree browser",
  "workflow chooser", "graph editor", "code viewer".
- `docs/HOW_TO_HUMAN.md` — link to the new `/workflows/new` and `/view/`
  surfaces; update keyboard-shortcut table if any new shortcuts land.
- New `docs/FRONTEND_DEVELOPMENT.md` — contributor-side guide for
  developing UI features (node setup, `make ui-install/build/dev`,
  island mounting pattern, type contracts).
- `docs/CLI_REFERENCE.md` — no new CLI verbs.
- `CHANGELOG.md` — Decided entry for D092 + Added entries for the
  feature additions.

### 7. No changes to

- The CLI surface (`striatum *` verbs).
- The JSON API (`/v1/*` endpoints).
- The SSE event feed.
- The MCP surface.
- The CSP header.
- The mutation gate (RFC 0013 step 7).
- The audit chain.
- The workflow JSON schema.
- The pip-installable runtime dependency tree (no new Python deps).

## Acceptance Criteria

- D092 supersedes D073's implicit no-node rule; the decision-log entry
  is in place before this RFC is accepted.
- `frontend/package.json` + `vite.config.ts` + `tsconfig.json` land
  under `src/striatum/web/frontend/`.
- `make ui-install` / `make ui-build` / `make ui-dev` targets work.
- CI rebuilds the UI in a node-22-LTS environment and verifies the
  committed bundle hash matches.
- Bundled output lives under `src/striatum/web/static/build/` and ships
  with the wheel.
- Workflow detail's Edit affordance is a primary button next to "Run
  this workflow now".
- `/view/` (no path) renders a tree-browser island that lazily loads
  directory contents via `GET /v1/repo/tree`.
- `/workflows/new` renders a chooser wizard that calls
  `POST /workflows/generate/preview` and (with operator confirmation)
  `POST /workflows/generate`.
- Workflow editor uses react-flow for the graph + structured widgets
  for the per-node panels.
- `/view/<path>` for non-Markdown files uses shiki for syntax
  highlighting with line numbers + copy-to-clipboard + raw-link.
- All islands respect prefers-color-scheme dark; reuse base.css palette
  variables.
- Doc-link tests pass.
- Existing UI snapshot tests pass.
- New JS unit tests cover: api-client typings, island mount entry
  points, react-flow JSON serialization, shiki rendering paths.
- New `make ui-test` runs the contributor-side test suite (Vitest).

## Implementation Plan

### Step 1. Toolchain + CI

Land `frontend/` skeleton: `package.json` with React, react-flow,
shiki, typescript, vite, vitest; `vite.config.ts` for multi-entry
build; `tsconfig.json`. Make targets: `ui-install`, `ui-build`,
`ui-dev`, `ui-test`. CI gains node-22-LTS setup + bundle-hash check.
First commit checks in an empty build output to establish the path.

### Step 2. D073 supersession + Edit-link promotion

Land D092 in DECISION_LOG.md. Promote the workflow-detail Edit link to
a primary button (non-island, server-template change). Smoke test that
the existing UI still renders identically minus the link change.

### Step 3. Code viewer

Land the code-viewer island first (lowest-risk addition: read-only,
no mutation, no operator-confirmation). Wire `/view/<path>` to mount
the island for non-Markdown files. Verify shiki bundle size, grammar
coverage, dark-mode parity.

### Step 4. Tree browser

Land the tree-browser island + `GET /v1/repo/tree` endpoint. Extend
`/view/` (no path) to render the island. Verify clicking files
navigates to the single-file viewer.

### Step 5. Workflow chooser wizard

Land the chooser island + `/workflows/new` route. Wire to existing
RFC 0034 V1 endpoints. Verify the operator-confirmation gate is
reused from the chat-tool gate (no duplicate path).

### Step 6. Workflow graph editor

Land the react-flow-based graph editor. Replace (or live alongside)
the existing form-driven editor in `workflow_edit.html`. Per-node
panels with structured widgets. Save calls the existing workflow-edit
POST endpoint.

### Step 7. Docs + contributor-side guide

Update SPEC, UBIQUITOUS_LANGUAGE, HOW_TO_HUMAN, CLI_REFERENCE (no
new verbs), CHANGELOG. New `docs/FRONTEND_DEVELOPMENT.md` covers the
contributor-side toolchain.

## Open Questions

- Should we lock the framework choice to React, or leave it as an
  open question for the design phase? Recommendation: lock to React in
  this RFC so the design phase converges faster; future framework
  changes get their own RFC.
- Should the tree-browser endpoint return only repo-relative paths
  (existing `/view/` posture) or expose any path the daemon owner can
  read? Recommendation: repo-relative only, matches `/view/<path>`
  semantics and the `--repo` flag intent.
- Should the chooser wizard support shape: `custom` (with the
  graph-plan compiler from RFC 0034 §5)? Recommendation: V1 ships only
  the built-in shapes; `custom` is a V1.5 follow-up after the basic
  flow stabilizes.
- Should the graph editor support real-time validation (running
  workflow_validate on every keystroke)? Recommendation: server-side
  validation runs on save; client-side schema-shape check on
  field-change for fast feedback; full validation is the gate.
- Should we bundle shiki with all grammars or lazy-load grammars?
  Recommendation: bundle the 8 named grammars (json, py, ts/js, sh,
  yaml, toml, md, sql); operator file types beyond those fall back to
  plain `<pre>`.
- Should the legacy form-driven workflow editor be retired or kept as
  a fallback? Recommendation: keep it as a fallback for one release
  cycle, then retire in a follow-up RFC after operator validation.

## Domain Modeling

This RFC adds presentation surfaces, not new domain concepts. The
existing aggregates (run, job, blocker, workflow, doctor problem,
artifact, capability token, chat session, generation spec) stay as-is.
The web UI gains framework-islands as a deployment shape; the islands
are scoped views into the same data the server has always exposed.

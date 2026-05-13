# RFC 0038 Codex Design

author: designer-codex-gpt-5.5-001

## Objective

Implement RFC 0038 as a progressive upgrade to the existing local web UI:
Jinja2 keeps owning page shells and routing, while React islands take over the
component-heavy surfaces that have outgrown the current vanilla JavaScript.
The CLI, SQLite state model, service mutation gate, SSE/event surfaces, audit
posture, and local-first product boundary stay unchanged.

The important architectural line is this: Vite, React, TypeScript, React Flow,
Shiki, and Vitest are contributor-side build dependencies. Operators still get
a pip-installable Python wheel with prebuilt static assets under
`striatum.web.static`. No runtime Node dependency, CDN asset, hosted service,
telemetry, transcript capture, or external template catalog is introduced.

## Current Baseline

The current UI is server-rendered by `src/striatum/service.py` using Jinja2
templates from `src/striatum/web/templates/` and static assets from
`src/striatum/web/static/`. Route dispatch is centralized in
`_dispatch_get()` and `_dispatch_post()`. Static assets are served through
`importlib.resources.files("striatum.web.static")`, and package data currently
ships flat `*.html`, `*.js`, `*.css`, and `*.svg` entries for
`striatum.web.static`.

Mutation gates are already server-side. `/v1/invoke` refuses non-read commands
without `--allow-mutations`, workflow edit/save and run-now endpoints gate
independently, and workflow generation already exposes
`POST /workflows/generate/preview` plus mutation-gated
`POST /workflows/generate`. RFC 0038 should reuse those endpoints rather than
creating an alternate write path.

The existing templates already use a light island pattern:
`workflow_edit.html` serializes workflow JSON in script tags for
`workflow_edit.js`, and graph pages emit data attributes for client-side
tooltips. RFC 0038 formalizes that pattern for React, rather than replacing
the UI with a SPA.

## Toolchain Bootstrap

Add `src/striatum/web/frontend/` as the only Node-owned source tree:

```text
src/striatum/web/frontend/
  package.json
  package-lock.json
  vite.config.ts
  tsconfig.json
  .gitignore
  src/
    islands/
      code-viewer/
      tree-browser/
      workflow-chooser/
      workflow-graph-editor/
    shared/
      api-client.ts
      mount.ts
      types.ts
      theme.css
```

`package.json` should declare runtime dependencies `react`, `react-dom`,
`react-flow`, and `shiki`, with dev dependencies `typescript`, `vite`,
`vitest`, `@types/react`, and `@types/react-dom`. Commit
`package-lock.json`; do not use CDN dependencies. Keep scripts boring:
`build`, `dev`, `test`, and optionally `typecheck`.

`tsconfig.json` should use strict TypeScript, `target: "ES2022"`,
`module: "ESNext"`, `moduleResolution: "Bundler"`, `jsx: "react-jsx"`,
`noImplicitAny: true`, `noUncheckedIndexedAccess: true`, and
`exactOptionalPropertyTypes: true`. The component job should avoid `any`; if a
server payload is not yet typed, model it as `unknown` and narrow at the API
boundary.

`vite.config.ts` should use multi-entry Rollup input so each island has an
addressable bundle:

```ts
export default defineConfig({
  build: {
    outDir: "../static/build",
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      input: {
        shared: "src/shared/entry.ts",
        "tree-browser": "src/islands/tree-browser/main.tsx",
        "workflow-chooser": "src/islands/workflow-chooser/main.tsx",
        "workflow-graph-editor": "src/islands/workflow-graph-editor/main.tsx",
        "code-viewer": "src/islands/code-viewer/main.tsx",
      },
      output: {
        entryFileNames: "island-[name].js",
        chunkFileNames: "island-[name]-[hash].js",
        assetFileNames: "island-[name][extname]",
      },
    },
  },
});
```

Use stable entry names for scripts referenced by Jinja templates. Shared
chunks may be hashed, but the hash check described below must make drift
visible. `.gitignore` in `frontend/` should contain `node_modules/`, `.vite/`,
and `dist/`; committed output lives in `../static/build/`, not `dist/`.

## Makefile, CI, And Package Data

Add Makefile targets:

```make
.PHONY: ui-install ui-build ui-dev ui-test

ui-install:
	cd src/striatum/web/frontend && npm install

ui-build:
	cd src/striatum/web/frontend && npm run build

ui-dev:
	cd src/striatum/web/frontend && npx vite dev

ui-test:
	cd src/striatum/web/frontend && npx vitest run
```

Do not make `make install` run npm. For release confidence, `check` can grow a
UI check after the first build is stable, but the minimum RFC 0038 CI
requirement is a GitHub Actions Node 22 LTS step that runs `npm ci`,
`make ui-build`, `make ui-test`, and a bundle-hash check.

The hash check should compare a deterministic manifest committed with the
bundle, for example `src/striatum/web/static/build/manifest.sha256`, against
fresh hashes after `make ui-build`. A small script such as
`scripts/check_ui_bundle_hashes.py` can sort all files under
`src/striatum/web/static/build/`, ignore the manifest itself, and fail with:

```text
UI bundle drift detected; rerun `make ui-build` and commit the updated build output.
```

Update `.github/workflows/ci.yml` to install Node 22 before the UI build step.
Keep Python lint, typecheck, tests, release metadata, package smoke, and fresh
clone smoke intact.

Update `[tool.setuptools.package-data]` in `pyproject.toml` with an explicit
build package entry:

```toml
"striatum.web.static.build" = ["*.js", "*.css", "*.json", "*.wasm", "*.sha256"]
```

Also add `src/striatum/web/static/build/__init__.py` so the nested build
directory is a package data target. If Vite emits an `assets/` subdirectory
later, add a second package-data entry rather than relying on an implicit
recursive include.

## Island Mounting Contract

Keep Jinja2 as page owner. A page that needs React renders a stable container
and a module script:

```html
<div id="island-code-viewer" data-props='{{ code_viewer_props | tojson }}'></div>
<script type="module" src="/static/build/island-code-viewer.js"></script>
```

Use JSON script tags for large payloads such as workflow JSON or file content;
use `data-props` only for small scalar options. This avoids giant escaped
attributes and matches the current `workflow_edit.html` shape. A shared
`mount.ts` helper should parse props, find the root, call `createRoot()`, and
render an error panel if required props are missing.

Do not add inline scripts, `unsafe-inline`, `unsafe-eval`, remote fonts, CDN
stylesheets, or runtime calls outside same-origin endpoints. Vite dev HMR may
require a separate documented contributor mode, but production pages served by
`striatum serve --web` must keep the current CSP:
`default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:;
connect-src 'self'`.

## Feature 1: Promote Workflow Edit Affordance

Change `src/striatum/web/templates/workflow_detail.html` first. Replace:

```html
<a href="/workflows/edit/{{ workflow.path }}" class="muted">Edit</a>
```

with a visible button-style link next to "Run this workflow now":

```html
<a href="/workflows/edit/{{ workflow.path }}" class="secondary-button">Edit</a>
```

RFC 0038 text alternates between primary and secondary. Use
`secondary-button` unless synthesis explicitly chooses otherwise, because
"Run this workflow now" is the destructive lifecycle action and should remain
the single primary button. Add or update the existing test in
`tests/test_web_workflow_edit.py` to assert the edit affordance is not
`class="muted"` and sits in the run-meta action row.

## Feature 2: `/view/` Tree Browser

Add `GET /v1/repo/tree?path=<rel>` in `service.py`. It should:

- accept only repo-relative paths;
- reject `..`, leading `/`, null bytes, and symlink escapes;
- return 404 for `.git/` and `.striatum/`;
- return 404 when the target is not a directory;
- sort directories first, then files, case-insensitive;
- omit hidden files by default unless the design later adds an explicit query
  parameter;
- return `{entries: [{name, path, kind, size, mtime_utc}]}`.

`mtime_utc` should be an ISO-8601 UTC string derived from `stat().st_mtime`.
`path` should be repo-relative with forward slashes so the React tree can
navigate to `/view/<path>` without reconstructing ancestry from names.

Add a top-level `/view/` route before the existing `path.startswith("/view/")`
branch, rendering a new `view_tree.html` template. The template mounts
`island-tree-browser` with root props only; directory expansion happens lazily
through `/v1/repo/tree`. Existing `/view/<path>` file behavior remains.
Update `tests/test_web_view.py`: the current `test_view_directory_404` should
stay true for `/view/subdir`, while a new test should assert `/view/` returns
the tree mount and `/v1/repo/tree?path=` returns expected entries.

## Feature 3: `/workflows/new` Chooser Wizard

Add a `/workflows/new` GET route before `/workflows/<path>` dispatch. The
route renders a new `workflow_new.html` template with the
`island-workflow-chooser` mount.

The island should use existing generator endpoints:

- `GET /workflow-templates?kind=shape`;
- `GET /workflow-templates?kind=lane_set` if the catalog exposes it, otherwise
  use full template metadata from the shape details;
- `GET /workflow-templates/<id>`;
- `POST /workflows/generate/preview`;
- `POST /workflows/generate`.

Preview writes nothing. Save requires both `--allow-mutations` on the server
and `confirm_write: true` in the request body. Preserve the separate browser
operator confirmation from the chat tool flow: show the generated file list,
target paths, and validation warnings before enabling the final Save button.
The React code should not try to infer mutation authority as a security
decision; `/v1/health.allow_mutations` is only for disabling/hiding controls.

V1 should support built-in shapes only. Defer `shape: "custom"` graph-plan
authoring until the graph editor is stable enough to share its block palette.

## Feature 4: Drag-Drop Workflow Graph Editor

Keep `GET /workflows/edit/<path>` and `POST /workflows/edit/<path>` as the
server contract. Replace the form-driven DOM sections in
`workflow_edit.html` with:

```html
<div id="island-workflow-graph-editor"></div>
<script id="workflow-data" type="application/json">{{ workflow_json | safe }}</script>
<script id="workflow-sha256" type="application/json">"{{ workflow_sha256 }}"</script>
<script type="module" src="/static/build/island-workflow-graph-editor.js"></script>
```

Keep the existing `If-Match` behavior. The React island should POST the full
workflow JSON to the existing endpoint with `Content-Type: application/json`
and `If-Match: "<sha>"` when a hash exists. Server-side
`validate_workflow()` remains the gate; client-side validation is ergonomic
feedback only.

Use React Flow for the graph canvas. Nodes represent jobs; directed edges map
to workflow `edges`; bounded revision loops map to workflow `cycles` and
should render as a visually distinct edge style. A side panel owns structured
editing:

- dropdowns for role, lane, job type, review posture, edge event, and verdict;
- checkboxes/toggles for `fresh_session_required`, `repo_write`, and branch
  `allow_dirty`;
- repeatable rows for expected artifacts, context docs, write-scope paths, and
  lane capabilities;
- controlled text inputs for ids, titles, objective, prompt paths, and artifact
  paths;
- field-level error mapping from 422 responses when the server can provide it.

Keep the legacy vanilla editor as a fallback for one release cycle by retaining
`workflow_edit.js` behind a hidden or query-flagged fallback template only if
the implementers can do so without doubling maintenance. If that adds churn,
replace it directly and rely on tests plus the old script in git history.

## Feature 5: Syntax-Highlighted Code Viewer

Modify `view_file.html` so Markdown keeps server-rendered Markdown, binary
files keep the metadata panel, and non-Markdown text mounts
`island-code-viewer`. Pass `rel_path`, language, byte size, raw text, and a
raw endpoint URL via a JSON script tag. The existing `<pre>` can remain inside
`<noscript>` or as a fallback when the build bundle is absent in development.

Use Shiki with the bounded grammar set from RFC 0038: json, py, ts/js, sh,
yaml, toml, md, and sql. Unknown extensions fall back to escaped plain text.
The viewer should render line numbers, a copy button, a raw link, and collapse
files over 500 lines until expanded. Do not fetch grammars from a CDN at
runtime.

Because file contents are repo data, keep escaping discipline conservative:
Shiki HTML should be produced from text, not from trusted HTML, and React
should render it only through the library output. Tests should include a file
containing `<script>` and assert the raw tag is not executable page markup.

## Python-Side File Edits

Expected systems-lane edit set:

- `Makefile`: add `ui-install`, `ui-build`, `ui-dev`, `ui-test`; optionally
  include `ui-build` and the hash check in `check` after the first stable
  bundle lands.
- `.github/workflows/ci.yml`: add Node 22 setup, `npm ci`, `make ui-build`,
  `make ui-test`, and bundle-hash verification.
- `pyproject.toml`: package `striatum.web.static.build` data.
- `src/striatum/web/static/build/__init__.py`: make build output package-data
  addressable.
- `src/striatum/web/frontend/package.json`, `package-lock.json`,
  `vite.config.ts`, `tsconfig.json`, `.gitignore`: bootstrap the toolchain.
- `src/striatum/web/templates/base.html`: add a block for module scripts if
  page-local scripts need to load after vanilla `base.js`; do not load every
  island on every page.
- `src/striatum/web/templates/workflow_detail.html`: promote Edit affordance.
- `src/striatum/web/templates/view_file.html`: mount code viewer for
  non-Markdown text.
- `src/striatum/web/templates/view_tree.html`: new tree browser page.
- `src/striatum/web/templates/workflow_new.html`: new chooser wizard page.
- `src/striatum/web/templates/workflow_edit.html`: mount graph editor.
- `src/striatum/service.py`: add `/v1/repo/tree`, `/view/`, `/workflows/new`,
  and pass island props to templates.
- `tests/test_web_view.py`: tree endpoint, `/view/`, path safety, and code
  viewer mount assertions.
- `tests/test_web_workflow_edit.py`: edit-button class, graph-editor mount,
  existing POST/If-Match behavior still passing.
- `tests/test_service.py`: workflow chooser endpoints already exist; extend
  only if route-level coverage for `/workflows/new` belongs there.
- `tests/test_web_ui.py`: static build asset served, package-data resolution,
  no external URL invariant includes built JS/CSS.
- `tests/test_web_ui_redesign.py` and `tests/test_web_chat.py`: CSP assertions
  unchanged.

Expected TypeScript-lane edit set:

- `src/striatum/web/frontend/src/shared/api-client.ts`;
- `src/striatum/web/frontend/src/shared/types.ts`;
- `src/striatum/web/frontend/src/shared/mount.ts`;
- `src/striatum/web/frontend/src/islands/code-viewer/**`;
- `src/striatum/web/frontend/src/islands/tree-browser/**`;
- `src/striatum/web/frontend/src/islands/workflow-chooser/**`;
- `src/striatum/web/frontend/src/islands/workflow-graph-editor/**`;
- colocated Vitest tests for API typing, mount behavior, workflow JSON
  serialization, and Shiki rendering fallbacks.

These sets are intentionally disjoint except for the interface between
templates and island props. Define the prop schemas in TypeScript first and
mirror them in small Python dictionaries; do not let both jobs edit the same
component source files.

## Endpoint Design Details

`GET /v1/repo/tree` response:

```json
{
  "ok": true,
  "data": {
    "path": "docs",
    "entries": [
      {"name": "rfcs", "path": "docs/rfcs", "kind": "dir", "size": 4096, "mtime_utc": "2026-05-12T21:00:00Z"},
      {"name": "SPEC.md", "path": "docs/SPEC.md", "kind": "file", "size": 12345, "mtime_utc": "2026-05-12T21:00:00Z"}
    ]
  }
}
```

Errors should follow the existing JSON envelope style:
400 for malformed path, 404 for hidden/missing/not-directory targets, and 500
only for unexpected OS errors.

Workflow generation should continue returning the existing generator envelope.
Do not add a UI-only endpoint that transforms generator output; the chooser can
render `GeneratedWorkflow.files`, `metadata.graph`, `warnings`, and `workflow`
directly.

Workflow graph editor save response should remain compatible with the current
tests: success returns the new sha256, validation failures return 422 without
writing, stale `If-Match` returns 412 with `current_sha256`, and mutation-off
returns 405.

## CSS And Visual Integration

React islands should consume existing CSS variables from `base.css` instead of
introducing a separate design system. Add a thin shared CSS file from the Vite
build only for component internals that cannot be expressed cleanly with
existing classes. The bundle may emit `island-style.css`; include it only on
pages that need React islands.

The graph editor is a tool surface, not a marketing page. Keep it dense,
predictable, and keyboard-reachable: a left palette, central graph canvas, and
right inspector panel. Buttons should use existing `primary-button` and
`secondary-button` classes where possible. Radio cards are appropriate in the
workflow chooser because that is a selection wizard; graph editing controls
should prefer dropdowns, toggles, and compact repeatable rows.

## Verification Strategy

Focused Python tests before wider tests:

```bash
make ui-build
make ui-test
.venv/bin/python -m pytest tests/test_web_view.py tests/test_web_workflow_edit.py tests/test_web_ui.py -q
```

Then run:

```bash
make lint
make typecheck
make test
make package-smoke
```

CI should exercise the same UI build path on Node 22. Package smoke should
prove installed wheels can serve `/static/build/island-*.js` through
`importlib.resources`, not from the source tree by accident.

Manual smoke:

```bash
striatum --repo /path/to/target serve --web --allow-mutations
```

Then inspect `/view/`, `/view/pyproject.toml`, `/workflows`,
`/workflows/new`, and `/workflows/edit/<workflow-path>`. Verify CSP console
errors are absent, no network requests leave the local service, preview writes
nothing, generation without `--allow-mutations` fails closed, generation with
confirmation writes files, and workflow edit save still respects stale
`If-Match`.

## Risks And Mitigations

- Vite output shape may drift. Mitigate with explicit `entryFileNames`,
  package-data tests, and bundle-hash CI.
- Shiki can inflate bundle size. Mitigate by bundling only the named grammars
  and falling back to escaped plain text for unknown languages.
- React Flow can make workflow JSON serialization lossy if graph coordinates
  are treated as domain data. Keep node positions UI-only; persist only the
  existing workflow schema.
- The chooser could duplicate generator rules. Keep it as a typed client over
  existing endpoints; server validation remains the authority.
- CSP regressions are easy to miss with Vite. Keep no-inline/no-eval tests and
  scan built assets for external URLs.

## Implementation Order

1. Toolchain skeleton, package-data path, Makefile targets, CI Node 22, and
   hash check.
2. Edit affordance promotion and static build serving tests.
3. Code viewer island, because it is read-only and isolated to `/view/<path>`.
4. Tree browser endpoint and `/view/` landing.
5. Workflow chooser over existing RFC 0034 endpoints.
6. React Flow graph editor over the existing edit GET/POST contract.
7. Documentation updates: `docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
   `docs/HOW_TO_HUMAN.md`, `docs/FRONTEND_DEVELOPMENT.md`, and
   `CHANGELOG.md`.

This order front-loads build and packaging risk, then lands the smallest
read-only surfaces before the mutation-heavy graph editor.

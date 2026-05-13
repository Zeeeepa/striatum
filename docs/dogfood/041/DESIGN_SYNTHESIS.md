---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/041/design/codex/DESIGN.md", "docs/dogfood/041/design/claude_code/DESIGN.md", "docs/dogfood/041/design/gemini/DESIGN.md"]
---

author: designer-codex-gpt-5.5-001

# RFC 0038 Design Synthesis

## Accepted Scope

This plan implements RFC 0038 as a progressive server-rendered web UI upgrade. Jinja2 remains the page-shell owner, Striatum remains local-first, and React is used only for named component islands. The CLI surface, repo-local SQLite authority, SSE feed, CSP, mutation gate, audit posture, and workflow schema do not change.

Acceptance-criteria mapping:

| RFC 0038 acceptance criterion | Concrete plan | Owner |
| --- | --- | --- |
| D092 supersedes D073 before acceptance | Treat D092 as already accepted in `docs/DECISION_LOG.md`; do not reopen the no-node decision. If docs are touched, keep D092 as the citation for the contributor-side Node toolchain. | implement_toolchain_codex |
| Frontend scaffold lands under `src/striatum/web/frontend/` | Add Vite + React + TypeScript scaffold, strict `tsconfig.json`, npm lockfile, and Vite multi-entry build. | implement_toolchain_codex |
| `make ui-install`, `ui-build`, `ui-dev`, `ui-test` work | Add Makefile targets that run npm only inside `src/striatum/web/frontend/`; `make install` remains Python-only. | implement_toolchain_codex |
| CI rebuilds UI on node-22-LTS and verifies committed bundle hash | Add Node 22 setup, `npm ci`, `make ui-build`, `make ui-test`, `npm audit --omit=dev`, and a deterministic static-build hash check. | implement_toolchain_codex |
| Bundled output lives under `src/striatum/web/static/build/` and ships with the wheel | Commit Vite output and `manifest.sha256`; add package-data coverage for build files. | implement_toolchain_codex |
| Workflow detail Edit affordance is promoted | Change `workflow_detail.html` from muted text link to a visible button-styled anchor next to "Run this workflow now"; keep Run visually dominant. | implement_toolchain_codex |
| `/view/` renders a tree-browser island using `GET /v1/repo/tree` | Add `/view/` Jinja2 route and `GET /v1/repo/tree?path=<rel>`; implement lazy React tree navigation. | Endpoint/template: implement_toolchain_codex. Island/tests: implement_components_claude. |
| `/workflows/new` renders a chooser wizard over generator preview/write endpoints | Add route and template mount; implement six-step React wizard using existing RFC 0034 endpoints and `confirm_write: true`. | Route/template: implement_toolchain_codex. Island/tests: implement_components_claude. |
| Workflow editor uses react-flow and structured widgets | Preserve existing edit GET/POST and If-Match semantics; replace or augment the form surface with a React Flow graph editor and inspector widgets. | Template/server contract: implement_toolchain_codex. Island/tests: implement_components_claude. |
| `/view/<path>` non-Markdown files use shiki with line numbers, copy, raw link | Keep Markdown server-rendered; mount code-viewer for non-Markdown text with server-rendered `<pre>` fallback. | Template props: implement_toolchain_codex. Island/tests: implement_components_claude. |
| Islands respect dark mode and base palette | Use existing `base.css` variables; React CSS must not introduce a separate theme system. | implement_components_claude |
| Doc-link and UI snapshot tests pass | Preserve existing tests; add focused Python and Vitest coverage before full suite. | Both, by touched scope |
| JS unit tests cover API typings, mounts, graph serialization, shiki paths | Add Vitest tests under `frontend/src/__tests__/`. | implement_components_claude |
| `make ui-test` runs Vitest | Wire npm script and Makefile target. | Makefile: implement_toolchain_codex. Tests: implement_components_claude. |

## Toolchain Selection

Use Vite + React + TypeScript, with React Flow for the workflow graph editor and Shiki for syntax highlighting. No framework alternatives remain open for this implementation.

The V1 dependency set should stay narrow: `react`, `react-dom`, `@xyflow/react` or the current React Flow package name selected by npm ecosystem reality, and `shiki` as runtime dependencies; `@vitejs/plugin-react`, `typescript`, `vite`, `vitest`, `@types/react`, and `@types/react-dom` as dev dependencies. Do not add Radix, lucide, clsx, Tailwind helpers, ESLint, or an accessibility plugin in the first pass unless the component implementer demonstrates a concrete gap that cannot be handled with plain React and existing CSS. This is the main tradeoff across the designs: Gemini proposed a broader UI dependency stack, but RFC 0038's supply-chain posture is better served by the smallest package set that can ship the named features.

Commit `package-lock.json`. CI installs with `npm ci`. Shiki bundles only these grammars: json, py/python, ts/js, sh/bash, yaml, toml, md/markdown, and sql. Unknown file types fall back to escaped plain text.

## Project Layout

Use this layout:

```text
src/striatum/web/frontend/
  package.json
  package-lock.json
  vite.config.ts
  tsconfig.json
  .gitignore
  index.html
  src/
    main.ts
    islands/
      tree-browser/
      workflow-chooser/
      workflow-graph-editor/
      code-viewer/
    shared/
      api-client.ts
      mount.ts
      types.ts
      theme.css
    __tests__/
src/striatum/web/static/build/
  island-tree-browser.js
  island-workflow-chooser.js
  island-workflow-graph-editor.js
  island-code-viewer.js
  island-shared-<hash>.js
  island-style.css
  manifest.sha256
```

`frontend/.gitignore` includes `node_modules/`, `.vite/`, `coverage/`, and local Vite temp output only. It must not ignore `../static/build/`, because committed build output is part of the wheel.

`vite.config.ts` uses multi-entry Rollup input with stable entry names for the four islands. Shared chunks may be hash-named; the manifest hash check makes any drift explicit.

## Makefile + CI

Add:

```make
.PHONY: ui-install ui-build ui-dev ui-test

ui-install:
	cd src/striatum/web/frontend && npm ci

ui-build:
	cd src/striatum/web/frontend && npm run build

ui-dev:
	cd src/striatum/web/frontend && npm run dev

ui-test:
	cd src/striatum/web/frontend && npm run test -- --run
```

CI adds a Node 22 LTS step after Python setup and before packaging checks:

```text
npm ci
make ui-build
make ui-test
npm audit --omit=dev
python scripts/check_ui_bundle_hashes.py
git diff --exit-code src/striatum/web/static/build
```

`scripts/check_ui_bundle_hashes.py` sorts files under `src/striatum/web/static/build/`, ignores `manifest.sha256`, and compares SHA-256 values against the committed manifest. The failure message should tell contributors to rerun `make ui-build` and commit the updated bundle.

## Wheel Distribution

Update `pyproject.toml` package data so installed wheels serve build assets through `importlib.resources`, not from the source tree by accident. The accepted shape includes an explicit `striatum.web.static.build` package-data entry:

```toml
"striatum.web.static" = ["*.html", "*.js", "*.css", "*.svg"]
"striatum.web.static.build" = ["*.js", "*.css", "*.json", "*.wasm", "*.sha256", "assets/*", "chunks/*"]
```

If setuptools package discovery requires `src/striatum/web/static/build/__init__.py`, add it. Package smoke must verify `/static/build/island-code-viewer.js` or an equivalent stable island bundle is served after wheel install.

## Island Mount Pattern

Every island mounts from a Jinja2-owned page slot:

```html
<div id="island-<name>" data-props='{{ props | tojson }}'></div>
<script type="module" src="/static/build/island-<name>.js" defer></script>
```

Use per-island `createRoot()` calls into named DOM slots. Small scalar props may live in `data-props`; large payloads such as file contents and workflow JSON should use adjacent `script type="application/json"` tags to avoid oversized escaped attributes:

```html
<div id="island-workflow-graph-editor" data-props='{{ editor_props | tojson }}'></div>
<script id="workflow-data" type="application/json">{{ workflow_json | safe }}</script>
<script id="workflow-sha256" type="application/json">"{{ workflow_sha256 }}"</script>
<script type="module" src="/static/build/island-workflow-graph-editor.js" defer></script>
```

`frontend/src/shared/mount.ts` owns prop parsing, missing-root handling, and rendering a small same-page error panel. Production pages must keep the existing CSP shape: no inline scripts, no `unsafe-inline`, no `unsafe-eval`, no CDN, and no external runtime fetch.

## Five Feature Additions

### 5a. Edit affordance promotion

`implement_toolchain_codex` changes `src/striatum/web/templates/workflow_detail.html` only. Replace the muted `Edit` link with an anchor styled as a button, placed immediately next to "Run this workflow now". Use label `Edit workflow`. Keep it an `<a>` so open-in-new-tab works. Use `secondary-button` or the existing secondary visual treatment so the run-now mutation remains the dominant primary action.

Add a focused Python test asserting the workflow detail page no longer renders `class="muted">Edit` and that the edit href appears in the run action cluster.

### 5b. Tree browser island

`implement_toolchain_codex` adds:

- `GET /v1/repo/tree?path=<rel>` in `src/striatum/service.py`.
- A `/view/` route before the existing `/view/<path>` branch.
- A new `view_tree.html` template mounting `island-tree-browser`.

Endpoint contract:

```json
{
  "ok": true,
  "data": {
    "path": "docs",
    "entries": [
      {"name": "rfcs", "path": "docs/rfcs", "kind": "dir", "size": 4096, "mtime_utc": "2026-05-12T21:00:00Z"},
      {"name": "SPEC.md", "path": "docs/SPEC.md", "kind": "file", "size": 12345, "mtime_utc": "2026-05-12T21:00:00Z"}
    ],
    "truncated": false
  }
}
```

Reject `..`, leading `/`, null bytes, symlink escapes, `.git/`, and `.striatum/`. Return 404 for missing or non-directory targets. Sort directories first, then files, case-insensitive. Keep the endpoint repo-relative only.

`implement_components_claude` builds the lazy tree island with WAI-ARIA tree semantics, roving tabindex, keyboard navigation, search over loaded entries, per-directory retry states, and file-click navigation to `/view/<path>`.

### 5c. Workflow chooser wizard

`implement_toolchain_codex` adds `/workflows/new` before `/workflows/<path>` dispatch and renders `workflow_new.html` with `island-workflow-chooser`.

`implement_components_claude` builds a six-step wizard:

1. Shape radio cards from `GET /workflow-templates?kind=shape`.
2. Lane-set radio cards filtered by the selected shape metadata.
3. Lane modifier multi-select with compatibility feedback.
4. Required details: workflow id, name, scaffold root, artifact root, branch suggestion, lane commands.
5. Preview using `POST /workflows/generate/preview`, rendering generated workflow JSON, file list, warnings, and graph metadata.
6. Save in a real `<dialog>` confirmation, calling `POST /workflows/generate` with `confirm_write: true`.

Preview writes nothing. The final write still depends on server `--allow-mutations` and operator confirmation. The React code may hide disabled controls based on health data, but security remains server-side.

V1 supports built-in shapes only. `shape: "custom"` graph-plan authoring is deferred until the graph editor can share its block palette.

### 5d. Drag-drop workflow graph editor

Keep existing `GET /workflows/edit/<path>` and `POST /workflows/edit/<path>` contracts, including validation, mutation gating, stale `If-Match` handling, and sha256 response behavior.

`implement_toolchain_codex` updates `workflow_edit.html` to mount `island-workflow-graph-editor` and preserve workflow JSON + sha256 payloads. If low-churn, keep the existing vanilla form editor behind a fallback query or link for one release; if that doubles maintenance, replace it directly and rely on tests plus git history.

`implement_components_claude` builds the React Flow editor:

- Nodes represent workflow jobs.
- Normal edges represent workflow `edges`.
- Bounded revision loops represent workflow `cycles` with distinct styling.
- A left palette exposes RFC 0034 custom-plan block kinds.
- A right inspector edits role, lane, job type, review posture, required postures, write scope, expected artifacts, parallel group, and edge verdict fields.
- Coordinates are UI-only and never persisted into workflow JSON.
- Save posts the full existing workflow schema JSON to the existing endpoint.

Client validation is ergonomic only. Server `validate_workflow()` remains authoritative.

### 5e. Syntax-highlighted code viewer

`implement_toolchain_codex` modifies `view_file.html` so Markdown continues server-rendering as Markdown, binary or undisplayable files keep the metadata/empty state, and non-Markdown text renders a server-side `<pre>` fallback plus `island-code-viewer` mount.

`implement_components_claude` builds Shiki highlighting over the pre-rendered text:

- Line numbers.
- Copy full file.
- Raw link.
- Wrap toggle.
- Collapse-by-default for files over 500 lines.
- Plain-text fallback for unknown grammars.
- No runtime grammar downloads.

For files over 5 MB, skip Shiki and keep plain escaped text. Tests must include text containing `<script>` and assert it is not executable markup.

## Disjoint Implementer Write Scopes

`implement_toolchain_codex` owns Python-side and integration files:

- `src/striatum/web/frontend/package.json`
- `src/striatum/web/frontend/package-lock.json`
- `src/striatum/web/frontend/vite.config.ts`
- `src/striatum/web/frontend/tsconfig.json`
- `src/striatum/web/frontend/.gitignore`
- `src/striatum/web/templates/`
- `src/striatum/web/static/build/`
- `src/striatum/service.py`
- `src/striatum/web/workflows.py` only if route helpers need it
- `tests/test_web_ui.py`
- `tests/test_web_ui_redesign.py`
- `tests/test_web_workflows.py`
- `tests/test_service.py`
- `Makefile`
- `.github/workflows/ci.yml`
- `pyproject.toml`
- `scripts/check_ui_bundle_hashes.py`
- `docs/dogfood/041/build/toolchain/HANDOFF.md`

`implement_components_claude` owns TypeScript-side components and docs:

- `src/striatum/web/frontend/index.html`
- `src/striatum/web/frontend/src/main.ts`
- `src/striatum/web/frontend/src/islands/**`
- `src/striatum/web/frontend/src/shared/**`
- `src/striatum/web/frontend/src/__tests__/**`
- `docs/FRONTEND_DEVELOPMENT.md`
- `docs/HOW_TO_HUMAN.md`
- `docs/UBIQUITOUS_LANGUAGE.md`
- `docs/CLI_REFERENCE.md`
- `CHANGELOG.md`
- `docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md` status/update notes
- `docs/dogfood/041/build/components/HANDOFF.md` if the workflow later adds one
- the combined build handoff if assigned by the operator

The only shared boundary is the prop contract between Jinja2 templates and React islands. Define TypeScript prop types first, then mirror the same fields in Python dictionaries. Do not let both implementers edit the same component source files.

## Accessibility Checklist

Every island must satisfy:

- Keyboard nav completeness for its primary task.
- Visible focus indicators in light and dark modes.
- No `tabindex` greater than 0.
- `<dialog>` confirmation in the chooser uses `showModal()`, moves focus inside on open, restores focus on close, closes on Esc, and does not let Tab escape.
- Tree browser uses `role="tree"`, `role="treeitem"`, `aria-expanded`, `aria-level`, and a polite live region for load failures.
- Chooser radio cards use radiogroup semantics and arrow-key movement.
- Graph editor canvas has a textual fallback region, focusable nodes, keyboard add/delete/save paths, and screen-readable node labels.
- Code viewer line numbers are `aria-hidden`; Copy, Raw, and Wrap controls have exact `aria-label`s.
- All text meets WCAG AA contrast using existing palette variables.
- `prefers-reduced-motion` disables graph/editor animation and pan inertia.
- The RFC 0037 skip link still reaches page content on `/view/`, `/workflows/new`, and `/workflows/edit/<path>`.

## Cross-Platform / Browser Matrix

Contributor builds are supported on Linux and macOS only. Node v22+ is required for frontend work. Runtime browser support is modern evergreen browsers: current Chrome/Edge, Firefox, and Safari with native ESM and `<dialog>` support. Windows daemon or Windows frontend contributor support is out of scope for RFC 0038 V1.

## npm Supply-Chain Posture

Use npm with a committed `package-lock.json` and `npm ci` in CI. Keep the package set narrow, avoid CDNs, and prefer widely used packages with stable licenses. CI runs `npm audit --omit=dev`; high or critical runtime vulnerabilities fail. Development-only findings are recorded for operator review but do not automatically block unless they affect the shipped bundle path.

This guarantee is advisory against npm ecosystem risk. The real controls are a small dependency set, lockfile review, no runtime network fetch, committed build output, and reproducible bundle hashes.

## Build Determinism

The committed bundle is the distribution artifact. `src/striatum/web/static/build/manifest.sha256` records deterministic SHA-256 hashes for all generated build files except the manifest itself. CI rebuilds, recomputes, and fails if the manifest or generated files differ.

The hash check is not a cryptographic supply-chain guarantee; it is a drift detector that prevents source and committed bundle output from silently diverging.

## Test Strategy

Focused tests first:

```bash
make ui-build
make ui-test
.venv/bin/python -m pytest tests/test_web_view.py tests/test_web_workflow_edit.py tests/test_web_ui.py -q
```

Then broader verification:

```bash
make lint
make typecheck
make test
make package-smoke
```

Python tests add coverage for `/v1/repo/tree`, `/view/`, `/workflows/new`, static build serving, package-data resolution, Edit button promotion, graph-editor mount, code-viewer mount, path traversal rejection, symlink escape rejection, and unchanged CSP.

Vitest covers typed API wrappers, island mount entry points, tree keyboard behavior, chooser step navigation and confirmation dialog focus, React Flow workflow JSON serialization, Shiki rendering/fallback paths, and escaping of hostile file contents.

Manual checklist covers unautomatable pieces: browser console has no CSP violations, no external network requests leave the local service, chooser preview writes nothing, generation fails closed without `--allow-mutations`, generation succeeds only with confirmation, workflow edit preserves stale `If-Match`, and dark-mode parity holds.

## Documentation Deltas

`implement_components_claude` updates:

- `docs/FRONTEND_DEVELOPMENT.md`: new contributor guide covering Node setup, make targets, island layout, prop contracts, adding an island, accessibility checks, and bundle hash workflow.
- `docs/HOW_TO_HUMAN.md`: add operator walkthroughs for `/view/`, `/workflows/new`, graph editor, and code viewer.
- `docs/UBIQUITOUS_LANGUAGE.md`: add `frontend island`, `tree browser`, `workflow chooser`, `graph editor`, `code viewer`, `path picker`, and `bundle hash manifest`.
- `docs/CLI_REFERENCE.md`: no new CLI verbs; add web-route cross-reference only.
- `CHANGELOG.md`: add feature entries for tree browser, chooser wizard, graph editor, code viewer, Edit affordance, and frontend toolchain.
- `docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md`: update status when implementation lands.

`implement_toolchain_codex` updates `docs/SPEC.md` with the frontend toolchain contract, island mounting pattern, committed bundle distribution, and the statement that operator install remains pip-only.

## Staging Plan per RFC 0038 Implementation Plan

1. Toolchain + CI: scaffold `frontend/`, add package data, Makefile targets, Node 22 CI, Vitest, bundle hash manifest, static build serving tests.
2. D073/D092 acknowledgement + Edit-button promotion: keep D092 as accepted, change `workflow_detail.html`, run focused workflow UI tests.
3. Code viewer: ship the lowest-risk read-only island first, wire non-Markdown `/view/<path>`, verify Shiki size and fallback behavior.
4. Tree browser: add `/v1/repo/tree` and `/view/`, then lazy tree island with path-safety tests.
5. Chooser wizard: add `/workflows/new`, wire existing generator endpoints, enforce preview-before-confirmed-write behavior.
6. Graph editor: ship React Flow editor over the existing edit contract, keeping server validation authoritative.
7. Docs: land frontend contributor guide, operator docs, glossary, changelog, SPEC update, and RFC status update.

This order front-loads build/package risk, then moves from read-only islands to mutation-heavy authoring surfaces.

## Human-Decision Questions

No blocker requires an operator decision before implementation. The synthesis resolves the open design issues as follows:

- V1 dependency set stays narrow; extra UI/helper libraries are deferred.
- Tree endpoint is repo-relative only.
- Chooser V1 excludes `shape: "custom"`.
- Full server validation runs on save, not every keystroke.
- Shiki bundles only the named grammars and falls back to plain text.
- Legacy workflow editor fallback is best-effort for one release only; replacement is acceptable if fallback doubles maintenance.
- Graph editor undo is limited to delete-undo banners in V1.

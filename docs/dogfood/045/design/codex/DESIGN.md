# RFC 0038 V1.5 Design: frontend integration hardening

author: designer-unknown-model-001
date: 2026-05-13
status: proposed

## Scope

RFC 0038 V1 shipped the React island source tree, templates, and committed
bundle paths, but dogfood-041 found that the integration contract was still
unsafe: the Vite build could emit placeholders, `/workflows/new` had a server
API shape that did not match the chooser island, the global shared entry could
mount every island twice once real bundles exist, the package-data story needs
to be explicit, and npm supply-chain hygiene needs a repeatable baseline.

V1.5 should be a narrow integration-hardening pass. It must preserve the public
served bundle paths under `/static/build/`, the Jinja2 page shells, and the
existing island IDs. It should not add new UI surfaces.

## F1: real Vite builds only

Current problem: `src/striatum/web/frontend/vite.config.ts` imports
`existsSync`, declares `placeholderIslandPlugin`, maps missing entries to
virtual modules, and emits `console.info` stubs instead of failing the build
when an island entry is absent (`vite.config.ts:1`, `vite.config.ts:17`,
`vite.config.ts:39`, `vite.config.ts:48`). The current committed bundles prove
the failure: `src/striatum/web/static/build/island-workflow-chooser.js` is a
single placeholder log line, and the other island bundles are similarly tiny.

V1.5 acceptance:

- Delete `placeholderIslandPlugin` entirely and remove the `existsSync` /
  `type Plugin` imports from `vite.config.ts`.
- Keep the island entry names and output file names stable:
  `island-tree-browser` -> `src/islands/tree-browser/main.tsx`,
  `island-workflow-chooser` -> `src/islands/workflow-chooser/main.tsx`,
  `island-workflow-graph-editor` -> `src/islands/workflow-graph-editor/main.tsx`,
  and `island-code-viewer` -> `src/islands/code-viewer/main.tsx`
  (`vite.config.ts:11` through `vite.config.ts:14`).
- Run `make ui-build`. Post-fix, `src/striatum/web/static/build/` must contain
  real Rollup output for all four named island bundles, CSS output, any hashed
  shared chunks, and a regenerated `manifest.sha256`. Hashes must change from
  the placeholder baseline because the emitted JavaScript is now compiled React
  code.
- Add or update a test/CI assertion that fails when any top-level island bundle
  contains the placeholder sentinel string `Striatum frontend island placeholder
  loaded` or is below a small sanity threshold such as 1 KiB. This belongs next
  to the existing `ui-check-bundle` drift check rather than inside the Python
  service.

The intent is fail-closed contributor builds: if a React entry is missing or
does not compile, Vite fails and the committed bundle hash check fails.

## F2: workflow chooser catalog contract

Current problem: the service endpoint for `GET /workflow-templates` returns
`{"ok": true, "data": {"templates": list_templates(...)}}`
(`src/striatum/service.py:707` through `src/striatum/service.py:713`), while
the chooser island calls `fetchWorkflowTemplates()` and stores the result as a
`WorkflowTemplateCatalog` (`src/striatum/web/frontend/src/shared/api-client.ts:56`,
`src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx:117`).
That TypeScript catalog type requires `shapes` and `lane_sets`
(`src/striatum/web/frontend/src/shared/types.ts:93`), and the component
immediately dereferences `catalog.shapes` and `catalog.lane_sets`
(`WorkflowChooser.tsx:130`, `WorkflowChooser.tsx:136`).

Move the React side, not the server side. The server shape is already the RFC
0034 catalog-list endpoint, and changing it would risk CLI/chat/web callers
that expect a list of templates.

V1.5 acceptance:

- Introduce a frontend type for the actual list endpoint, for example
  `WorkflowTemplateListResponse { templates: WorkflowTemplate[] }`, alongside
  the existing generation types.
- Change `fetchWorkflowTemplates()` to return that server shape, or add a
  dedicated `fetchWorkflowTemplateList()` and keep the old name only if tests
  are updated to prove the shape.
- Change `WorkflowChooser` to derive its wizard choices from `templates` rather
  than from nonexistent `catalog.shapes` and `catalog.lane_sets`. The simplest
  V1.5-compatible derivation is:
  select a template in step 1, show its shape/lane-set metadata from the
  template record, then construct the generation spec from the selected
  template's declared shape and lane defaults.
- Keep `src/striatum/web/templates/workflow_new.html` unchanged except for
  optional prop additions; it already mounts the correct island and passes
  `templatesUrl`, `previewUrl`, `generateUrl`, and `allowMutations`
  (`workflow_new.html:12`, `workflow_new.html:16`).
- Add a Vitest case with a literal mocked response shaped as
  `{ok: true, data: {templates: [...]}}` and assert the chooser renders a
  selectable option instead of a load/error state.

The compatibility rule is that `/workflows/new` continues to render the same
Jinja2 page and the same `island-workflow-chooser.js` script path, while the
island adapts to the current server API.

## F3: shared chunk without mount side effects

Current problem: `vite.config.ts` maps `island-shared` to `src/main.ts`
(`vite.config.ts:9` and `vite.config.ts:10`). That file is a dev-only shell
that imports every island component and calls `mount()` for all four mount
slots (`src/striatum/web/frontend/src/main.ts:1`, `src/main.ts:32`,
`src/main.ts:39`, `src/main.ts:46`, `src/main.ts:53`). The global template
loads `/static/build/island-shared.js` on every page (`base.html:44`), then
individual pages load their own island bundle, such as
`/static/build/island-workflow-chooser.js` (`workflow_new.html:16`). With real
bundles, this creates a double-mount risk and also makes non-island pages pay
for island side effects.

V1.5 acceptance:

- Stop declaring `island-shared` as a Rollup entry. Shared code should be an
  implementation chunk produced by Rollup from imports, not a page-loaded entry
  with side effects.
- Add a side-effect-free shared entry only if the template must load global
  island CSS directly. Preferred shape: import `src/shared/theme.css` from each
  island `main.tsx` as it already does in `workflow-chooser/main.tsx:10`; let
  Vite emit CSS and chunks from those imports.
- Remove the global script tag for `/static/build/island-shared.js` from
  `base.html:44`. Page templates should load only the island bundle they mount.
- Keep `src/striatum/web/frontend/src/main.ts` as a dev-only Vite shell for
  `make ui-dev` and `index.html`, but do not include it in production
  `rollupOptions.input`.
- Keep `chunkFileNames: "island-shared-[hash].js"` or an equivalent hashed
  chunk naming convention for Rollup-generated shared chunks. Those chunks are
  imported by the island bundles; Jinja2 templates should not name them.
- Add a mount test asserting that loading the chooser entry and then invoking a
  shared helper does not call `createRoot()` twice for the same container.

This keeps existing islands mounting while removing production side effects
from the shared asset path.

## F4: output and package-data semantics

Current state: Vite writes to `resolve(rootDir, "../static/build")`, which is
`src/striatum/web/static/build/` from the frontend directory
(`vite.config.ts:50`). The Python static handler serves `/static/<relative>`
from package resources under `striatum.web.static`
(`src/striatum/service.py:2353` through `src/striatum/service.py:2363`).
Package data includes both `striatum.web.static` with `build/*.js`,
`build/*.css`, `build/*.sha256`, and `striatum.web.static.build` with the same
file classes (`pyproject.toml:47` through `pyproject.toml:50`).

V1.5 acceptance:

- Preserve the output directory and served URLs. The public paths remain
  `/static/build/island-tree-browser.js`,
  `/static/build/island-workflow-chooser.js`,
  `/static/build/island-workflow-graph-editor.js`, and
  `/static/build/island-code-viewer.js`.
- Decide whether `vite`'s generated `.vite/manifest.json` is package data. If
  retained, add `build/*.json` or `build/.vite/*.json` to package-data and
  tests. If not retained, set Vite `manifest: false` and rely only on
  `manifest.sha256`.
- The server template-rendering code does not consume Vite's manifest file.
  Templates use hardcoded stable bundle names, and the service serves package
  resources by path. Therefore `manifest.sha256` is a CI drift detector only,
  not runtime routing data.
- Update packaging tests to assert that an installed wheel exposes the four
  stable island bundles and any Rollup-generated hashed shared chunks through
  `importlib.resources.files("striatum.web.static").joinpath("build/...")`.
- Keep `emptyOutDir: true` only if `manifest.sha256` is regenerated after every
  build. The current `make ui-build` already runs `ui-bundle-hash`
  (`Makefile:35` through `Makefile:37`).

This makes the wheel contract explicit: templates name stable entry bundles,
Rollup entry bundles import hashed chunks, and package data must include both.

## Supply-chain hygiene

Current state: `src/striatum/web/frontend/package-lock.json` exists, while
`package.json` declares runtime dependencies on React, React DOM, reactflow,
Shiki, and the Vite React plugin, plus TypeScript/Vite/Vitest/jsdom development
dependencies (`package.json:11` through `package.json:26`). `make ui-install`
currently runs `npm install` (`Makefile:32` and `Makefile:33`), which can
rewrite the lockfile.

V1.5 acceptance:

- Treat `package-lock.json` as committed source. CI and ordinary contributor
  bootstrap should use `npm ci --prefix src/striatum/web/frontend` so installs
  are lockfile-reproducible. `make ui-install` can either switch to `npm ci` or
  split into `ui-ci-install` and a clearly documented `ui-update-lock`.
- Add `make ui-audit` running
  `npm audit --prefix src/striatum/web/frontend --audit-level=high`, or add the
  same command to `ui-check-bundle`. High/critical findings fail CI unless a
  checked-in baseline explicitly records the accepted issue.
- Capture the first baseline in a small tracked file such as
  `docs/FRONTEND_DEVELOPMENT.md` or
  `src/striatum/web/frontend/npm-audit-baseline.json`. Prefer the JSON file if
  CI is expected to diff machine-readable IDs; prefer the doc if the baseline
  is empty and only the command policy needs recording.
- Add a dependency-review section to `docs/FRONTEND_DEVELOPMENT.md`: why each
  dependency is present, whether it ships to operator browsers, and who owns
  updates. `@vitejs/plugin-react` is currently in `dependencies`; V1.5 should
  move it to `devDependencies` unless runtime evidence says otherwise.
- CI regressions surface in two places: lock drift through `npm ci` refusing
  mismatched `package.json`/`package-lock.json`, and vulnerability drift through
  `make ui-audit` failing above the configured threshold.

This is not a cryptographic supply-chain control. It is the minimum hygiene
needed once the project accepts npm-managed frontend dependencies.

## Test and verification matrix

V1.5 implementation is complete when these checks pass:

- `make ui-install` or the new CI install target uses the checked-in lockfile.
- `make ui-test` passes after updating API-client and chooser tests.
- `make ui-build` emits real bundles and regenerates `manifest.sha256`.
- `make ui-check-bundle` fails on placeholder stubs or uncommitted build drift.
- `make ui-audit` passes or records an explicit baseline.
- Python web tests confirm `/workflows/new` serves
  `/static/build/island-workflow-chooser.js`, `/view/` serves
  `/static/build/island-tree-browser.js`, and static serving can read packaged
  build assets.
- A browser smoke check of `/workflows/new` verifies that the chooser renders
  at least one template from the `{templates: [...]}` API response and can reach
  preview without JavaScript runtime errors.

## Non-goals

- No new workflow-generator API shape.
- No new island or route beyond repairing the existing RFC 0038 V1 surfaces.
- No hosted assets, CDN, telemetry, or remote template catalog.
- No renaming of public bundle paths loaded by Jinja2 templates.

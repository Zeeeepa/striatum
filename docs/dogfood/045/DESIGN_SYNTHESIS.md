---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/045/design/codex/DESIGN.md", "docs/dogfood/045/design/claude_code/DESIGN.md", "docs/dogfood/045/design/gemini/DESIGN.md"]
---
author: designer-unknown-model-002

# RFC 0038 V1.5 Design Synthesis

## Chosen Plan

RFC 0038 V1.5 is an integration-hardening pass over the RFC 0038 V1 web UI toolchain. It does not add new UI surfaces. It makes the committed Vite bundles real, aligns the `/workflows/new` island with the server contract that already exists, removes production mount side effects from the shared bundle, makes package-data coverage explicit, and adds minimum npm supply-chain hygiene.

The public compatibility contract stays stable: existing island mount IDs remain unchanged, served bundle URLs under `/static/build/` remain unchanged, and `/workflows/new` keeps rendering the workflow chooser page.

## F1 Placeholder Removal

Delete `placeholderIslandPlugin` from `src/striatum/web/frontend/vite.config.ts` entirely. That means removing the `existsSync` import, the `type Plugin` import, the whole `placeholderIslandPlugin()` function, and changing `plugins: [placeholderIslandPlugin(), react()]` to `plugins: [react()]`.

The surviving Rollup input mapping is:

```ts
const islandEntries: Record<string, string> = {
  "island-shared": resolve(srcDir, "shared/island-shared-entry.ts"),
  "island-tree-browser": resolve(srcDir, "islands/tree-browser/main.tsx"),
  "island-workflow-chooser": resolve(srcDir, "islands/workflow-chooser/main.tsx"),
  "island-workflow-graph-editor": resolve(srcDir, "islands/workflow-graph-editor/main.tsx"),
  "island-code-viewer": resolve(srcDir, "islands/code-viewer/main.tsx")
};
```

`make ui-build` remains the build command. It runs the Vite build into `src/striatum/web/static/build/` and then regenerates `manifest.sha256`. The expected committed outputs are the stable entry files `island-shared.js`, `island-tree-browser.js`, `island-workflow-chooser.js`, `island-workflow-graph-editor.js`, `island-code-viewer.js`, `style.css`, `manifest.sha256`, and any Rollup-generated shared chunks matching `island-shared-[hash].js`.

`make ui-check-bundle` must fail if any stable island entry contains the sentinel string `Striatum frontend island placeholder loaded` or if any stable island JavaScript file is below a small sanity threshold such as 1 KiB. This check belongs beside the existing build-drift check, so placeholder output cannot satisfy CI by regenerating matching hashes.

## F2 Chooser Prop Contract

Rewrite the component side to consume the server's current `{templates: [...]}` shape. The server route `src/striatum/service.py::_handle_workflow_templates` already exposes the RFC 0034 template-list endpoint and should stay stable for current web and future chat/API callers; adapting one island is lower risk than changing a shared endpoint contract.

In `src/striatum/web/frontend/src/shared/types.ts`, add a `WorkflowTemplateListResponse` type with `templates: WorkflowTemplate[]`, matching the rows returned by `src/striatum/workflow_generator/catalog.py::list_templates`: `template_id`, `kind`, `display_name`, `summary`, `recommended_for`, and `default_lane_sets`.

In `src/striatum/web/frontend/src/shared/api-client.ts`, change `fetchWorkflowTemplates()` to return `ApiResult<WorkflowTemplateListResponse>`. In `src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`, store that list response, render step 1 as template selection, and derive `shape` from the selected template's `kind` and `lane_set` from one of its `default_lane_sets`. Remove the dependency on nonexistent `catalog.shapes`, `catalog.lane_sets`, and `catalog.modifiers` for V1.5.

`src/striatum/web/templates/workflow_new.html` already passes the right island props: `templatesUrl`, `previewUrl`, `generateUrl`, and `allowMutations`. It should not need a server-side prop reshape.

## F3 Double-Mount Fix

Use a separate non-mounting shared entry. Production must not point `island-shared` at `src/striatum/web/frontend/src/main.ts`, because `main.ts` mounts all islands and is only appropriate for the Vite dev shell.

Create `src/striatum/web/frontend/src/shared/island-shared-entry.ts` as a side-effect-free production entry except for shared CSS import:

```ts
import "./theme.css";
export {};
```

The individual island entries continue to import their dependencies directly from `../../shared/...`, for example `../../shared/mount`, `../../shared/api-client`, `../../shared/types`, and `../../shared/theme.css` where needed. Rollup may still factor common code into `island-shared-[hash].js` chunks, but those chunks are imported by the stable island entry bundles rather than manually mounted.

Keep `src/striatum/web/templates/base.html` loading `/static/build/island-shared.js` only if that file is the non-mounting shared entry above. Do not load `src/main.ts` output in production. Add a regression test that loads the shared entry and the chooser entry and asserts `createRoot()` is called once for `#island-workflow-chooser`.

## F4 Output And Package Data Layout

Preserve the output directory `src/striatum/web/static/build/` and the public stable bundle URLs:

```text
/static/build/island-shared.js
/static/build/island-tree-browser.js
/static/build/island-workflow-chooser.js
/static/build/island-workflow-graph-editor.js
/static/build/island-code-viewer.js
```

The package-data contract in `pyproject.toml [tool.setuptools.package-data]` must include stable entries, CSS, hash manifest, and generated shared chunks:

```toml
"striatum.web.static" = ["*.html", "*.js", "*.css", "*.svg", "build/*.js", "build/*.css", "build/*.sha256", "build/.vite/*.json"]
"striatum.web.static.build" = ["*.js", "*.css", "*.sha256", ".vite/*.json"]
```

Alternatively, set `manifest: false` in `vite.config.ts` and omit the `.vite/*.json` globs. Choose one in implementation; because the Python templates use stable hardcoded paths and do not consume Vite's manifest, the simpler implementation is `manifest: false` plus `manifest.sha256` as the only committed manifest.

The Python serving path remains `src/striatum/service.py` resolving package resources from `striatum.web.static` and serving `/static/<relative>`. Add packaging tests that read `importlib.resources.files("striatum.web.static").joinpath("build/island-workflow-chooser.js")` and at least one `build/island-shared-*.js` chunk after a build.

## Supply-Chain Hygiene

Commit `src/striatum/web/frontend/package-lock.json` and treat it as source. `make ui-install` should use `npm ci --prefix src/striatum/web/frontend` so normal installs are lockfile-reproducible. If contributors need dependency updates, add a separate `make ui-update-lock` target using `npm install --prefix src/striatum/web/frontend`.

Add `make ui-audit`, implemented as:

```make
ui-audit:
	npm audit --prefix src/striatum/web/frontend --audit-level=high
```

Store the audit baseline at `src/striatum/web/frontend/npm-audit-baseline.json`. If the baseline is empty, commit an empty JSON object with a short schema comment in `docs/FRONTEND_DEVELOPMENT.md`; if there are accepted high/critical findings, record package name, advisory id, severity, reason accepted, and review date in the JSON baseline.

Move `@vitejs/plugin-react` to `devDependencies` unless implementation finds a runtime reason for browser delivery. The dependency-review cadence trigger is any `package.json` or `package-lock.json` change, plus every RFC that adds a new browser dependency.

## Backward Compatibility And Regression Assertions

Existing islands must still mount into `#island-tree-browser`, `#island-workflow-chooser`, `#island-workflow-graph-editor`, and `#island-code-viewer`. Served bundle URLs stay unchanged for the four public island entries, and `/workflows/new` must keep rendering the chooser shell and loading `/static/build/island-workflow-chooser.js`.

Implementation must pin these regression assertions:

- `make ui-build` emits real bundles and regenerates `manifest.sha256`.
- `make ui-check-bundle` rejects placeholder sentinels and build drift.
- `make ui-test` covers the chooser with a literal `{ok: true, data: {templates: [...]}}` mocked response.
- Python web tests assert `/workflows/new` includes `/static/build/island-workflow-chooser.js`, `/view/` includes `/static/build/island-tree-browser.js`, `/workflows/edit/<path>` includes `/static/build/island-workflow-graph-editor.js`, and non-Markdown `/view/<path>` includes `/static/build/island-code-viewer.js`.
- Static/package-data tests assert the built bundles are available through `importlib.resources`.
- A browser smoke check for `/workflows/new` verifies at least one template option renders and preview can be requested without a JavaScript runtime error.

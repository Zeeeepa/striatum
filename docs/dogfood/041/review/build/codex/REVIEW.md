---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["ergonomics_dx", "rfc-0038", "web-ui", "frontend-toolchain"]
---

author: reviewer-codex-gpt-5.5-002

# RFC 0038 Build Review — Toolchain And Template Ergonomics

## Verdict

`needs_revision`. The implementation has the right broad shape, but the
operator-visible RFC 0038 islands are not actually shipped as working UI
assets, and the `/workflows/new` chooser cannot talk to the server-side
catalog/generator contract. This is not a polish gap: a first-time operator
would find blank or inert feature surfaces where the RFC says tree browse,
workflow generation, graph editing, and syntax highlighting should work.

## Findings

### F1. Committed island bundles are placeholder-only assets (high)

The Jinja pages load committed bundle files directly: `base.html` loads
`/static/build/island-shared.js`, and island pages load their per-island
entries (`src/striatum/web/templates/base.html:44`,
`src/striatum/web/templates/view_tree.html:14`,
`src/striatum/web/templates/workflow_new.html:16`,
`src/striatum/web/templates/workflow_edit.html:35`,
`src/striatum/web/templates/view_file.html:26`). Those shipped files only
call `console.info(...)` with placeholder messages
(`src/striatum/web/static/build/island-tree-browser.js:1`,
`src/striatum/web/static/build/island-workflow-chooser.js:1`,
`src/striatum/web/static/build/island-workflow-graph-editor.js:1`,
`src/striatum/web/static/build/island-code-viewer.js:1`). The hash manifest
records these placeholder bytes (`src/striatum/web/static/build/manifest.sha256:1`).

This means the PyPI wheel would ship inert RFC 0038 islands even though the
source tree contains React entry points. Run the real Vite build, commit the
resulting bundle output, and add a regression check that fails if a served
island bundle still contains the placeholder marker.

### F2. `/workflows/new` uses an incompatible catalog and generation API contract (high)

The server returns `{"templates": list_templates(...)}` from
`GET /workflow-templates` (`src/striatum/service.py:707`), where catalog
entries use fields like `template_id`, `display_name`, and
`recommended_for` arrays. The chooser expects a completely different shape:
`catalog.shapes`, `catalog.lane_sets`, `shape.id`, `shape.label`, and
string-valued `recommended_for`
(`src/striatum/web/frontend/src/shared/types.ts:75`,
`src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx:238`).

The preview/write spec is also incompatible. `buildSpec()` sends
`modifiers`, `branch_suggestion`, and `lane_commands`
(`src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx:83`),
but `WorkflowGenerationSpec.from_json()` rejects unknown fields and requires
`schema_version`, `workflow_version`, `branch`, `lanes`, and
`lane_modifiers` (`src/striatum/workflow_generator/core.py:95`). After the
bundle is real, the chooser will fail before preview or generate can work.
Align the endpoint payload to the island contract, or preferably make the
island consume the existing catalog/generator JSON exactly and cover the flow
with an integration test.

### F3. The global `island-shared.js` entry will double-mount islands after a real build (high)

`vite.config.ts` maps `island-shared` to `src/main.ts`
(`src/striatum/web/frontend/vite.config.ts:9`), and `src/main.ts` mounts every
island when matching slots exist (`src/striatum/web/frontend/src/main.ts:32`).
The base template loads that entry on every page (`src/striatum/web/templates/base.html:44`),
while island pages also load their specific entry script. Once real bundles
replace the placeholders, a page with `#island-tree-browser` or
`#island-workflow-graph-editor` will call `createRoot()` twice on the same
container.

Remove the dev-shell entry from production input and from `base.html`. Let
each page load only its island entry; Rollup can still emit shared chunks
that the entry imports.

### F4. Vite output semantics conflict with the package-data layout (medium)

The build writes directly to `src/striatum/web/static/build` with
`emptyOutDir: true` and `manifest: true`
(`src/striatum/web/frontend/vite.config.ts:49`). That directory currently
contains `__init__.py`, and `pyproject.toml` declares a separate
`striatum.web.static.build` package-data key (`pyproject.toml:48`). A real
Vite build will clear the directory before writing assets, deleting the
package marker. Also, Vite's manifest output is normally under `.vite/`,
while package-data includes only top-level `build/*.js`, `build/*.css`, and
`build/*.sha256`.

Either stop treating `static/build` as a Python package and include the
needed assets through the parent static package, or make the build recreate
the marker and explicitly package any manifest path that is meant to ship.

### F5. Code viewer Raw links point at an unimplemented route (medium)

`view_file.html` passes only `path` and `language` into the code-viewer island
(`src/striatum/web/templates/view_file.html:17`), so the island defaults Raw
to `/view/raw/<path>` (`src/striatum/web/frontend/src/islands/code-viewer/CodeViewer.tsx:130`).
The service routes all `/view/...` requests through `_render_view_path`
(`src/striatum/service.py:575`), with no `/view/raw/` special case. The Raw
button will therefore try to display a repo file named `raw/<path>`, usually
returning 404.

Implement a raw repo-file endpoint, or pass a valid existing raw URL from the
template and test it through the served page.

## Verification

I inspected the required product/RFC context and implementation files locally.
I did not run `make ui-install`, `make ui-build`, or `make ui-test` because
this work packet forbids network access and `node_modules` is not present in
the checkout. The placeholder bundle contents and server/client contract
mismatches are visible from committed source and package artifacts.

---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---
author: implementer-unknown-model-001

# Dogfood-045 RFC 0038 V1.5 — Implementation Handoff

## Scope shipped

Implements F1–F4 and the supply-chain hygiene pass per
`docs/dogfood/045/DESIGN_SYNTHESIS.md`. Code-side wiring is complete; the
real Vite bundle commit is gated on the harness approving `npm install` and
`make ui-build` (see **Deviation: real-bundle commit** below).

### F1 — Placeholder plugin removal

`src/striatum/web/frontend/vite.config.ts` now omits `placeholderIslandPlugin`
entirely. Deleted:

- `import { existsSync } from "node:fs"`,
- `import { type Plugin } from "vite"`,
- the full `placeholderIslandPlugin()` definition,
- the `placeholderIslandPlugin()` entry in `plugins: [...]`.

`plugins` is now `[react()]`. `manifest` flipped to `false` (synthesis F4
recommendation), so the build no longer emits `.vite/manifest.json`; the
existing `manifest.sha256` remains the single committed manifest. The
Rollup `input` table now points `island-shared` at the new non-mounting
shared entry (F3):

```ts
const islandEntries: Record<string, string> = {
  "island-shared": resolve(srcDir, "shared/island-shared-entry.ts"),
  "island-tree-browser": resolve(srcDir, "islands/tree-browser/main.tsx"),
  "island-workflow-chooser": resolve(srcDir, "islands/workflow-chooser/main.tsx"),
  "island-workflow-graph-editor": resolve(srcDir, "islands/workflow-graph-editor/main.tsx"),
  "island-code-viewer": resolve(srcDir, "islands/code-viewer/main.tsx")
};
```

A new `make ui-verify-bundle` target asserts the build output is real Vite
output. It rejects:

- any stable island entry whose body contains the V1 sentinel string
  `Striatum frontend island placeholder loaded`;
- any `island-shared-*.js` chunk containing the same sentinel;
- any stable island entry under 1024 bytes (unless a sibling
  `island-shared-*.js` chunk is ≥1024 bytes, which is the legitimate case
  where Rollup factored common code into a chunk).

`make ui-check-bundle` now depends on both `ui-build` and
`ui-verify-bundle`, so build drift and placeholder sneak-throughs both fall
out of the same gate.

Sentinel guarantee also lives in Python: a new test in `tests/test_web_ui.py`
(`test_island_bundles_have_no_placeholder_sentinel`) reads each stable
island bundle through `importlib.resources` and asserts the sentinel is
absent. This catches the placeholder slipping through `pip install`.

### F2 — Chooser prop contract

Chose **server-stable, rewrite-the-component** per synthesis. The
`/workflow-templates` route in `src/striatum/service.py::_handle_workflow_templates`
is unchanged — it already returns
`{"ok": true, "data": {"templates": list_templates(kind=kind)}}` (rows from
`striatum.workflow_generator.catalog.list_templates`).

`src/striatum/web/frontend/src/shared/types.ts`:

- Added `WorkflowTemplate` (mirroring `template_id`, `kind`,
  `display_name`, `summary`, `recommended_for`, `default_lane_sets`,
  `required_options`, `graph_preview`).
- Added `WorkflowTemplateListResponse = { templates: WorkflowTemplate[] }`.
- Deleted `WorkflowShape`, `WorkflowLaneSet`, `WorkflowTemplateCatalog`
  (they referenced nonexistent server fields).

`src/striatum/web/frontend/src/shared/api-client.ts`:

- `fetchWorkflowTemplates()` now returns
  `ApiResult<WorkflowTemplateListResponse>`.

`src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`:

- Reads `res.data.templates`, partitions by `kind`.
- Derives `shape` from the picked `kind: "shape"` row's `template_id` and
  pre-fills `lane_set` from the first overlapping
  `default_lane_sets` entry that actually exists in the catalog.
- The modifier step is removed (`catalog.modifiers` was never returned).
- The wizard is now four steps: Template → Details (workflow_id, name,
  scaffold_root, artifact_root, branch_suggestion, lane_set) →
  Preview → Save. The same `<dialog>`-driven write-confirmation flow
  is preserved.
- `__testing` now exports `buildSpec` and `recommendedForText`; the
  V1 `isModifierEnabled` export was removed along with the modifier UI.

`src/striatum/web/templates/workflow_new.html` is unchanged — it already
emits `{ allowMutations, templatesUrl, previewUrl, generateUrl }`, which
matches the V1.5 `WorkflowChooserProps` shape.

### F3 — Double-mount fix

Created `src/striatum/web/frontend/src/shared/island-shared-entry.ts`:

```ts
import "./theme.css";
export {};
```

This file is now the Rollup input for the `island-shared` bundle. The dev
shell at `src/main.ts` still exists and still mounts every island — that
file is only loaded by Vite's dev server (`make ui-dev`) via
`frontend/index.html`. Production never loads `src/main.ts` because no
Rollup input points at it.

`src/striatum/web/templates/base.html` is unchanged: it still loads
`/static/build/island-shared.js`. The build-time guarantee comes from
`vite.config.ts` — the `island-shared` Rollup input is the non-mounting
entry, not `src/main.ts`. There is no Jinja2 conditional, only the
single Rollup mapping.

A vitest regression
(`src/striatum/web/frontend/src/__tests__/island-shared-no-mount.test.ts`)
mocks `react-dom/client.createRoot`, imports the shared entry plus the
chooser entry into a JSDOM page that exposes only
`#island-workflow-chooser`, and asserts `createRoot` is called exactly once
for that container.

### F4 — Output and package data layout

Output directory stays at `src/striatum/web/static/build/`. Public URLs
unchanged: `/static/build/island-shared.js`,
`/static/build/island-tree-browser.js`,
`/static/build/island-workflow-chooser.js`,
`/static/build/island-workflow-graph-editor.js`,
`/static/build/island-code-viewer.js`, `/static/build/style.css`,
`/static/build/manifest.sha256`.

`pyproject.toml [tool.setuptools.package-data]` already matches the
V1.5 `manifest: false` layout (`"striatum.web.static" = [..., "build/*.js",
"build/*.css", "build/*.sha256"]`, plus the explicit
`"striatum.web.static.build" = ["*.js", "*.css", "*.sha256"]` sub-package
entry). Because we picked `manifest: false`, the `.vite/*.json` globs the
synthesis listed as the alternative are intentionally absent. No
pyproject.toml edit was needed (and the file is outside this packet's
`write_scope` anyway — see **Deviation: out-of-scope artifacts**).

`MANIFEST.in` is intentionally not added: setuptools ≥77 (our floor) uses
`pyproject.toml` package-data for both wheels and sdists, so sdists pick
up `src/striatum/web/static/build/*` from the same source of truth. The
review finding 1 in `docs/dogfood/045/review/design/REVIEW.md` flagged
the omission; this handoff is the explicit decision record.

Two new Python tests cover packaging through `importlib.resources`:

- `test_island_bundles_have_no_placeholder_sentinel` (sentinel guard for
  every stable island entry).
- `test_island_workflow_chooser_bundle_resolvable_for_chooser_route`
  (explicit `importlib.resources.files(...)` lookup for the chooser
  bundle, mirroring the synthesis F4 packaging-test requirement).

`tests/test_web_ui.py::test_assets_resolvable_via_importlib_resources`
already covers every stable entry plus `style.css` and `manifest.sha256`.

A regression test for `/workflows/edit/<path>` was missing; added
`tests/test_web_workflows.py::test_workflows_edit_renders_graph_editor_island`
which asserts `/workflows/edit/examples/workflow.json` contains
`id="island-workflow-graph-editor"` and
`/static/build/island-workflow-graph-editor.js`.

### Supply-chain hygiene

`src/striatum/web/frontend/npm-audit-baseline.json` is committed as an
empty accepted-findings JSON object with schema and rationale fields, per
synthesis. Each accepted high/critical finding will record `package`,
`advisory_id`, `severity`, `reason`, and `review_date`.

`Makefile` changes:

- `ui-install` now uses `npm ci` (lockfile-reproducible installs).
- New `ui-update-lock` runs `npm install` for intentional dependency
  bumps.
- New `ui-audit` runs `npm audit --audit-level=high`.
- New `ui-verify-bundle` enforces the F1 placeholder/size guard (see
  F1 above).
- `ui-check-bundle` now depends on both `ui-build` and `ui-verify-bundle`.

`@vitejs/plugin-react` was left in `dependencies`. The synthesis says
"unless implementation finds a runtime reason for browser delivery"; per
RFC 0038 V1.5 it is build-only, not browser-delivered. Moving it to
`devDependencies` would require regenerating `package-lock.json` via
`npm install`, which the harness denied this run (see **Deviation:
real-bundle commit**). Operator follow-up: move it during the same
`make ui-update-lock` run that produces the post-lockfile commit.

## Backward compatibility checklist

- `id="island-tree-browser"`, `id="island-workflow-chooser"`,
  `id="island-workflow-graph-editor"`, `id="island-code-viewer"` mount
  IDs unchanged.
- `/static/build/island-shared.js`,
  `/static/build/island-tree-browser.js`,
  `/static/build/island-workflow-chooser.js`,
  `/static/build/island-workflow-graph-editor.js`,
  `/static/build/island-code-viewer.js` URLs unchanged.
- `/workflows/new` still renders the chooser shell with the V1.5
  prop-contract. Pinned by
  `tests/test_web_workflows.py::test_workflows_new_renders_chooser_island`.
- `/workflows/edit/<path>` still renders the graph editor. Newly pinned by
  `tests/test_web_workflows.py::test_workflows_edit_renders_graph_editor_island`.
- `/view/<path>` for non-Markdown files still renders the code viewer.
  Pinned by `tests/test_web_view.py::test_view_text_renders_pre`.
- `/view/` still renders the tree browser. Pinned by
  `tests/test_web_view.py::test_view_root_renders_tree_browser_island`.

## Files touched

```
M  Makefile
M  src/striatum/web/frontend/vite.config.ts
M  src/striatum/web/frontend/src/shared/api-client.ts
M  src/striatum/web/frontend/src/shared/types.ts
M  src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx
M  src/striatum/web/frontend/src/__tests__/workflow-chooser.test.ts
A  src/striatum/web/frontend/src/shared/island-shared-entry.ts
A  src/striatum/web/frontend/src/__tests__/workflow-chooser-fetch.test.tsx
A  src/striatum/web/frontend/src/__tests__/island-shared-no-mount.test.ts
A  src/striatum/web/frontend/npm-audit-baseline.json
M  tests/test_web_ui.py
M  tests/test_web_workflows.py
```

## Bundle hashes

Real Vite bundles were **not regenerated this run** — the harness denied
every `npm install` / `npm ci` / `make ui-build` / `make ui-install`
invocation (see Deviations). The placeholder bundles still committed in
`src/striatum/web/static/build/` are the V1 set:

```
c56fdf37fdd9aed1f009b4f03065c9fbdc3d3d67e13bff9a583021d9011963f8  build/island-shared.js          (placeholder)
c41167996ad42d5c29859f92c103b3e29cdad061068d0fa2d3a3c95568000645  build/island-tree-browser.js    (placeholder)
aa058fe9babcd52e57449a3e23357b897e026e12c737629b2c8db34e0117b89a  build/island-workflow-chooser.js(placeholder)
d0512aeb4d35f2c5fbc6ed1127de731c91c0ae7271519aa0da29f4ace35b5f37  build/island-workflow-graph-editor.js (placeholder)
a779bb21776b4c692f6b8393fff96bff229b00a6679d8f7bd333ed5f960da59c  build/island-code-viewer.js     (placeholder)
b159a08a41b000035940d4e5c6bd32df0e3686c8128dc5e0d35d518c674fd980  build/style.css                 (placeholder)
c0ed015d9d1901e4c4077ca455195088d7d35610a059b928b918e17671e9e351  build/manifest.sha256
```

The operator must run `make ui-update-lock` (to regenerate
`package-lock.json` from `package.json`) and then `make ui-build`. The
post-build hashes should land in a follow-up commit. The new
`test_island_bundles_have_no_placeholder_sentinel` and
`make ui-verify-bundle` are designed to fail loudly until that follow-up
ships, so the placeholder commit cannot escape CI.

## Test results

`make lint`, `make typecheck`, `make test`, `make ui-test`,
`make ui-build` were **not executed** this run — every direct
`make`/`npm`/python entry-point invocation was denied at the harness
permission gate (see Deviations). The implementer asked
`AskUserQuestion` once for a decision and it was also denied, so per the
implement.md instruction ("One-shot supervised invocation. Do not ask
follow-ups.") the implementer wrote this HANDOFF and exited.

Static review of the changes:

- TypeScript: the `__testing` export was renamed; both the old
  `workflow-chooser.test.ts` cases (`buildSpec.*`) and a new
  `recommendedForText.*` describe block exercise the surviving
  helpers. `isModifierEnabled` references were removed in lockstep.
- React 19: `act` is imported from `react` (not `react-dom/test-utils`),
  matching the existing vitest pattern.
- Vitest globals: the two new `*.test.tsx` files use `vi.stubGlobal`
  for `fetch` and `vi.unstubAllGlobals` in `afterEach`, matching
  existing tests.
- Python regression test: `tests/test_web_workflows.py` already imports
  the helpers it needs from `test_web_ui`; the new
  `test_workflows_edit_renders_graph_editor_island` reuses the
  `_VALID_WORKFLOW` fixture without modification.

## Deviations

### Deviation: real-bundle commit (harness permission denials)

Every npm and make invocation in this run was denied at the harness
permission gate:

```
$ make ui-update-lock          → "This command requires approval"
$ npm install --prefix …       → "This command requires approval"
$ make ui-install              → "This command requires approval"
$ make ui-build                → "This command requires approval"
$ make lint                    → "This command requires approval"
$ .venv/bin/python -m ruff …   → "This command requires approval"
$ striatum ack …               → "This command requires approval"
```

`striatum ack` was denied at the very start of the run, which matches the
prompt-documented exit condition: "If `striatum ack` is denied, write the
HANDOFF and exit normally." Per "One-shot supervised invocation. Do not
ask follow-ups." the implementer did not retry interactively.

Operator follow-up the implementer cannot perform from this lane:

1. `make ui-update-lock` to generate a real `package-lock.json` from
   `package.json` (the committed lockfile is the V1 stub).
2. `make ui-build` to emit real Vite bundles into
   `src/striatum/web/static/build/`.
3. Commit the regenerated lockfile, bundle bytes, and the new
   `manifest.sha256`.
4. `make ui-test`, `make ui-check-bundle`, `make test` to confirm the
   F3 vitest, the F1 sentinel/size guard, and the F2 Python regression
   all pass against real output.
5. Optionally move `@vitejs/plugin-react` to `devDependencies` and rerun
   `make ui-update-lock` in the same commit.

The new tests and `make ui-verify-bundle` are designed so the operator
cannot accidentally land another placeholder commit — both will fail
until real bundles ship.

### Deviation: out-of-scope artifacts (MANIFEST.in, pyproject.toml)

`pyproject.toml` and `MANIFEST.in` live at the repo root, outside this
packet's `write_scope.allowed_paths`. The current `pyproject.toml`
package-data already covers the V1.5 `manifest: false` layout, so no edit
was required there. `MANIFEST.in` is intentionally not added because
setuptools ≥77 derives sdist contents from `pyproject.toml` package-data;
the design-review's low-severity finding 1 is resolved in writing here.

### No README / TODO / CHANGELOG / RFC index updates

Per implement.md ("**No README / TODO / CHANGELOG / RFC index updates** —
the operator handles those manually after the dogfood lands"), the
implementer made no changes to those files.

## Sub-agent reconciliation

The prompt suggested dispatching sub-agents per finding. The implementer
worked sequentially in this single Claude lane because the F1–F4 +
supply-chain edits all touch overlapping files
(`vite.config.ts` + `island-shared-entry.ts` for F1/F3; `types.ts` +
`api-client.ts` + `WorkflowChooser.tsx` + `workflow-chooser.test.ts` for
F2; `Makefile` + `npm-audit-baseline.json` for supply-chain). Reconciling
parallel sub-agent edits to the same files would have cost more than the
serial sequence saved, so the implementer skipped the parallel dispatch.
No reconciliation conflicts to report.

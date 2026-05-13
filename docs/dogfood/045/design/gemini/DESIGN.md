# Design: RFC 0038 V1.5 (web UI integration gaps)

author: designer-unknown-model-001

## 1. Problem Statement

RFC 0038 V1 shipped the Vite/React/TypeScript toolchain and five UI features, but dogfood-041 surfaced critical integration gaps. The most material issues are the use of placeholder bundles instead of real builds, a prop-contract mismatch for the workflow chooser, and a double-mounting risk in the shared bundle. This design (V1.5) provides the concrete fixes to bridge these gaps while maintaining strict backward compatibility.

## 2. Integration Gaps and Fixes

### 2.1 F1 — Replace `placeholderIslandPlugin` with real builds

The current `vite.config.ts` includes a `placeholderIslandPlugin` that intercepts island entries and emits `console.info` stubs if the source files are missing. This was used to bootstrap the toolchain before all components were authored but now masks missing source files and produces non-functional bundles.

- **Location**: `src/striatum/web/frontend/vite.config.ts` (L17-L45, L48).
- **Fix**: Delete the `placeholderIslandPlugin` function and its usage in the `plugins` array.
- **Outcome**: `make ui-build` will fail if any island entry in `islandEntries` is missing, and will produce functional Rollup/Vite bundles in `src/striatum/web/static/build/`.
- **Entry points**: The following React components are the canonical entries:
    - `island-tree-browser`: `src/islands/tree-browser/main.tsx`
    - `island-workflow-chooser`: `src/islands/workflow-chooser/main.tsx`
    - `island-workflow-graph-editor`: `src/islands/workflow-graph-editor/main.tsx`
    - `island-code-viewer`: `src/islands/code-viewer/main.tsx`

### 2.2 F2 — `/workflows/new` chooser prop-contract mismatch

The server-side catalog endpoint returns a flat list of templates, but the React `WorkflowChooser` component (and its `fetchWorkflowTemplates` API client) expects a structured `WorkflowTemplateCatalog` object containing `shapes`, `lane_sets`, and `modifiers`.

- **Server side**: `src/striatum/service.py` L712 calls `list_templates()`.
- **Client side**: `src/striatum/web/frontend/src/shared/api-client.ts` L75 expects `WorkflowTemplateCatalog`.
- **Prop shape**: `src/striatum/web/frontend/src/shared/types.ts` L101-110.
- **Fix**: Move the server response to match the client's expectation. `_handle_workflow_templates` in `service.py` must return the full catalog object from `striatum.workflow_generator.catalog.load_catalog()`.
- **Justification**: The catalog object is the single source of truth for template relationships (which lane sets belong to which shapes); flattening it on the server loses this structure and breaks the wizard's filtering logic.

### 2.3 F3 — `island-shared.js` double-mount risk

`vite.config.ts` currently maps `island-shared` to `src/main.ts`. `main.ts` is a dev-only entry point that attempts to mount every island on the page. Because `base.html` loads `island-shared.js`, every page executes these side effects.

- **Location**: `src/striatum/web/frontend/vite.config.ts` L10; `src/striatum/web/templates/base.html` L46.
- **Fix**: Remove the `island-shared` entry from `islandEntries` in `vite.config.ts`. Remove the `<script type="module" src="/static/build/island-shared.js"></script>` line from `base.html`.
- **Strategy**: Let Vite/Rollup automatically extract shared chunks (e.g., `react`, `react-dom`, `shared/mount.ts`) based on island imports. These chunks will be loaded automatically by the browser when an island script is loaded. `src/main.ts` remains a dev-only entry used by `index.html` for `make ui-dev`.

### 2.4 F4 — Vite output semantics vs package-data layout

Bundles are built to `src/striatum/web/static/build/`, which is correctly tracked by `pyproject.toml` and served by `service.py`.

- **Build output**: `vite.config.ts` L50 sets `outDir` to `../static/build`.
- **Package data**: `pyproject.toml` L61-62 includes `striatum.web.static.build`.
- **Serving**: `src/striatum/service.py` L2361 uses `importlib.resources.files("striatum.web.static")` to resolve paths.
- **Verification**: The Vite `manifest.json` is not consumed by the server; templates use stable paths (e.g., `/static/build/island-workflow-chooser.js`) facilitated by `entryFileNames: "[name].js"` in `vite.config.ts` L57. This alignment is correct and must be preserved.

## 3. Supply-chain Hygiene

- **Lockfile**: The current `package-lock.json` is a placeholder. A real lockfile must be generated via `npm install` and committed.
- **Audit**: Add `make ui-audit` to the top-level `Makefile` which runs `npm audit` inside `frontend/`.
- **Policy**: `package-lock.json` is authoritative; CI must verify it matches `package.json` using `npm ci`.

## 4. Backward Compatibility

- **Islands**: Existing islands (`tree-browser`, `workflow-chooser`, etc.) must continue to mount into their `div#island-*` slots.
- **URLs**: Served bundle URLs at `/static/build/*.js` must remain unchanged.
- **Rendering**: The `/workflows/new` page must continue to render its chooser island after the prop-contract fix; the hydration failure currently present must be resolved.

## 5. Implementation Summary (CITATIONS)

| Finding | File | Action |
| --- | --- | --- |
| F1 (Plugin) | `src/striatum/web/frontend/vite.config.ts` | Delete `placeholderIslandPlugin` (L17-45) and ref in `plugins` (L48). |
| F2 (Catalog) | `src/striatum/service.py` | Change L712 to return `load_catalog()` instead of `{"templates": list_templates()}`. |
| F3 (Shared) | `src/striatum/web/frontend/vite.config.ts` | Remove `island-shared` entry (L10). |
| F3 (Shared) | `src/striatum/web/templates/base.html` | Remove `island-shared.js` script tag (L46). |
| F4 (Paths) | `src/striatum/web/frontend/vite.config.ts` | Preserve `outDir` (L50) and `entryFileNames` (L57). |
| Hygiene | `src/striatum/web/frontend/package-lock.json` | Replace placeholder with real `npm install` output. |

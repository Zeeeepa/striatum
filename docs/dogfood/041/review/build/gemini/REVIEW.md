---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "reject"
severity: "critical"
tags: ["build", "frontend", "supply-chain", "dx"]
---

author: reviewer-gemini-pro-001

# RFC 0038 V1 Build Review: Critical Integration and Integrity Failure

The implementation of RFC 0038 V1 is **REJECTED** due to critical failures in build integrity, a complete breach of the frontend-backend prop contract, and a fraudulent build process that masks missing implementation files.

## Summary of Critical Findings

### 1. Fraudulent Build Artifacts (Integrity Failure)
The bundled JavaScript files committed to `src/striatum/web/static/build/` (e.g., `island-code-viewer.js`, `island-tree-browser.js`) are 64-byte **placeholders**, not functional bundles.
- **Root Cause:** `vite.config.ts` includes a `placeholderIslandPlugin` that resolves missing source files to a virtual placeholder instead of failing the build.
- **Impact:** The build "passes" in CI despite the island entry points (`main.tsx`) being missing from the disk. The production distribution contains no functional React code.
- **Deception:** `make ui-check-bundle` succeeds because it compares the committed placeholders against the newly generated placeholders.

### 2. Prop Contract Breach (Integration Failure)
There is a total mismatch between the `data-props` emitted by Jinja2 templates and the `props` expected by the React components across all three major islands.
- **WorkflowChooser:** Template passes `allowMutations`, `previewUrl`, `generateUrl`. React component expects `mutationsAllowed`, `generatePreviewUrl`, `generateWriteUrl`, and is missing the mandatory `catalog` object.
- **CodeViewer:** Template passes `path`, `language`. React component expects `language`, `rawUrl`, `fileName`, `byteSize`, `sourceElementId`.
- **TreeBrowser:** Template passes `rootPath`, `treeUrl`. React component expects `initialPath`, `rootLabel`, `viewBase`.
- **Impact:** Even if the bundles were functional, hydration would fail immediately due to `undefined` mandatory props.

### 3. Supply-Chain Hygiene (Adversarial Analysis)
- **Stubbed Lockfile:** `src/striatum/web/frontend/package-lock.json` is a 660-byte stub that contains NO transitive dependencies. This breaks build determinism and exposes the project to "dependency hell" or supply-chain attacks on subsequent installs.
- **Missing Dependencies:** `WorkflowGraphEditor.tsx` imports from `@xyflow/react`, but this package is NOT listed in `package.json` (which only lists the deprecated `reactflow` v11).
- **Miscategorized Deps:** `@vitejs/plugin-react` is listed in `dependencies` instead of `devDependencies`.

### 4. Missing Entry Points
`vite.config.ts` defines entries pointing to `main.tsx` files (e.g., `islands/code-viewer/main.tsx`). However, the source tree contains `index.ts` files. While `main.ts` correctly imports the islands, the Vite multi-entry configuration is broken, which triggers the placeholder fallback mentioned in Finding #1.

### 5. Incomplete Template Integration
- `workflow_detail.html` was never updated to mount the graph editor island, remaining on the legacy server-rendered table and static SVG.
- `workflow_edit.html` mounts the graph editor but still includes the legacy `workflow_edit.js` form editor, creating a confusing and potentially conflicting DX.

## Ergonomics & DX Evaluation

The "placeholder" strategy in `vite.config.ts` is an adversarial DX pattern. By making the build succeed when files are missing, it hides implementation gaps from CI and other contributors. A robust build must fail fast when entry points are missing.

The lack of persisted coordinates in `WorkflowGraphEditor` (defaulting to a simple grid on every load) significantly degrades the "visual editor" affordance promised in the RFC.

## Recommendation

The implementation requires a complete "honesty pass":
1. **Remove `placeholderIslandPlugin`** from `vite.config.ts` immediately.
2. **Align entry points:** Either rename `index.ts` to `main.tsx` or update `vite.config.ts` to point to the correct files.
3. **Synchronize `types.ts` and Jinja2 templates:** The prop contract must be strictly enforced.
4. **Generate a real `package-lock.json`:** Run a full `npm install` and commit the resulting (large) lockfile.
5. **Add missing dependencies:** Ensure `@xyflow/react` is tracked.
6. **Complete the integration:** Update all relevant templates to mount the new islands as intended.

author: designer-gemini-pro-001

# Implementation Design: RFC 0038 - Web UI Feature Additions and Frontend Toolchain

This document details the implementation plan for the frontend modernization and feature additions proposed in RFC 0038.

## 1. Toolchain & Build System

### 1.1 Project Layout
A new `frontend/` directory will be established under `src/striatum/web/` to house the contributor-side TypeScript source.

```text
src/striatum/web/frontend/
├── package.json              # npm dependencies and scripts
├── package-lock.json         # Committed lockfile for determinism
├── tsconfig.json             # TypeScript configuration
├── vite.config.ts            # Vite multi-island build config
├── src/
│   ├── main.ts               # Island registry and mount logic
│   ├── islands/              # Individual React islands
│   │   ├── tree-browser/
│   │   ├── workflow-chooser/
│   │   ├── workflow-graph-editor/
│   │   └── code-viewer/
│   ├── shared/               # Shared components, hooks, and API client
│   │   ├── api.ts            # Typed fetch wrappers for /v1/*
│   │   ├── components/       # Radix UI based design system
│   │   └── types.ts          # TypeScript interfaces for domain objects
│   └── styles/
│       └── islands.css       # Global island styles (colors, layout)
└── tests/                    # Vitest unit and component tests
```

### 1.2 Vite Configuration
Vite will be configured to produce multiple entry points, one for each island, to avoid loading unnecessary code on pages that only need a subset of features.

```typescript
// src/striatum/web/frontend/vite.config.ts (concept)
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { resolve } from 'path';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../static/build',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        'tree-browser': resolve(__dirname, 'src/islands/tree-browser/index.tsx'),
        'workflow-chooser': resolve(__dirname, 'src/islands/workflow-chooser/index.tsx'),
        'graph-editor': resolve(__dirname, 'src/islands/workflow-graph-editor/index.tsx'),
        'code-viewer': resolve(__dirname, 'src/islands/code-viewer/index.tsx'),
      },
      output: {
        // Stable filenames for committed artifacts to minimize git churn
        entryFileNames: '[name].js',
        chunkFileNames: 'chunks/[name].js',
        assetFileNames: 'assets/[name].[ext]',
      },
    },
  },
});
```

### 1.3 Makefile Targets
New targets will be added to the root `Makefile` to manage the frontend toolchain:

- `make ui-install`: Runs `npm ci` in `src/striatum/web/frontend/`.
- `make ui-build`: Runs `npm run build` to generate the `static/build/` artifacts.
- `make ui-dev`: Runs `vite dev` for hot-reloading development.
- `make ui-test`: Runs `vitest` for frontend unit testing.

## 2. npm Supply Chain & Security

### 2.1 Package Strategy
Dependencies will be managed via `npm` with a focus on stability and security.

**Primary Dependencies:**
- `react`, `react-dom` (v18+)
- `react-flow` (v11+): For the graph editor.
- `shiki`: For syntax highlighting.
- `@radix-ui/react-tree-view`: For the accessible tree browser.
- `lucide-react`: For iconography.
- `clsx`, `tailwind-merge`: For utility-based styling.

**Pinning & Lockfile:**
- Standard semver ranges in `package.json`.
- `package-lock.json` MUST be committed to the repository.
- Contributor installs MUST use `npm ci` to ensure lockfile adherence.

### 2.2 Security Baseline
- **npm audit:** CI will run `npm audit --audit-level=high`. Any high or critical vulnerability will fail the build.
- **Vendor Grammars:** Shiki grammars (json, py, ts, js, sh, yaml, toml, md, sql) will be bundled to avoid runtime external requests.
- **CSP Compliance:** The existing Content Security Policy (`script-src 'self'`) will be maintained. Vite-bundled ESM modules are served from the local `/static/build/` path.

## 3. Deployment & Packaging

### 3.1 Wheel Inclusion
The `pyproject.toml` will be updated to include the new build directory in `package-data`:

```toml
[tool.setuptools.package-data]
"striatum.web.static" = ["*.html", "*.js", "*.css", "*.svg", "build/*", "build/chunks/*", "build/assets/*"]
```

### 3.2 Server-Side Integration
`src/striatum/service.py` already supports subdirectory serving in `_serve_static_asset`. No changes to the handler logic are required.

Jinja2 templates will mount islands using a standard data-prop pattern:

```html
<!-- Example: view_file.html -->
<div id="island-code-viewer" 
     data-props='{{ {"path": rel_path, "content_url": artifact_url} | tojson }}'>
</div>
<script type="module" src="/static/build/code-viewer.js"></script>
```

The `main.ts` entry point will detect the mount point and hydrate the React component:

```typescript
function mount(id: string, Component: React.ComponentType<any>) {
  const el = document.getElementById(id);
  if (el) {
    const props = JSON.parse(el.dataset.props || '{}');
    createRoot(el).render(<Component {...props} />);
  }
}
```

## 4. Island Implementation Details

### 4.1 Tree Browser (`/view/`)
- **API:** New `GET /v1/repo/tree?path=<rel>` endpoint in `service.py`.
- **Behavior:** Lazy loading of subdirectories. Uses Radix UI TreeView for keyboard accessibility (ARIA patterns).
- **Navigation:** Clicks on files trigger standard browser navigation to `/view/<path>`.

### 4.2 Workflow Chooser (`/workflows/new`)
- **Wizards:** Multi-step form using React state.
- **Preview:** Calls `POST /workflows/generate/preview` and renders a read-only graph + file list.
- **Confirmation:** Reuses the existing mutation gate pattern. The "Save" button only works if `confirm_write: true` is passed and the server-side operator gesture is active.

### 4.3 Graph Editor (`/workflows/edit/`)
- **React-Flow:** Custom nodes for Striatum blocks (role, lane, write-scope).
- **Widget Fields:**
  - Enums (e.g., `verdict_intent`) use `<select>` or Radix UI Select.
  - Paths use text inputs with "Copy from current" buttons.
  - Lane selectors use a specialized widget that fetches available lanes.
- **Persistence:** `POST` to `/workflows/edit/<path>` with the serialized JSON. Includes `If-Match` header (sha256) to prevent clobbering concurrent edits.

### 4.4 Code Viewer (`/view/<path>`)
- **Shiki:** High-quality syntax highlighting.
- **Features:** Line numbers, "Copy to Clipboard", "Raw" link.
- **Performance:** Files over 1MB will fall back to plain `<pre>` to avoid browser-side highlighting overhead.

## 5. Adversarial & Edge Cases

- **Malicious npm package:** Mitigation via `package-lock.json` review and `npm audit`. Prefer widely-adopted, low-dependency packages.
- **Build-output drift:** CI runs `make ui-build` and fails if `git diff --exit-code src/striatum/web/static/build/` is non-zero.
- **Frontend bundle bloat:** Vite configuration will include `build.reportCompressedSize`. A CI warning will be triggered if the total bundle size exceeds 2MB (compressed).
- **Accessibility regressions:** CI will include a linter for JSX a11y (`eslint-plugin-jsx-a11y`). Radix UI primitives provide a solid baseline for screen-reader support.

## 6. Browser Support Matrix

Targeting modern evergreen browsers (roughly last 2 years):
- **Chrome / Edge:** 100+
- **Safari:** 15.4+ (Native `<dialog>` and ESM support)
- **Firefox:** 100+

The build will target `es2022` to leverage modern JavaScript features while maintaining compatibility with the defined matrix.

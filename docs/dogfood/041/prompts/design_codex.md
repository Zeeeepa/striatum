# Codex Design Prompt

Produce `docs/dogfood/041/design/codex/DESIGN.md`.

Design an implementation plan for RFC 0038: web UI feature additions + Vite/React/TypeScript frontend toolchain. D092 supersedes D073's "no node toolchain" rule.

Cover concrete file-by-file edits:

**Frontend toolchain bootstrap (Python-side scope):**

- `src/striatum/web/frontend/package.json` — dependency set: react, react-dom, react-flow, shiki, typescript, vite, vitest, @types/react, @types/react-dom. No CDN packages. Lock file (`package-lock.json`) committed.
- `src/striatum/web/frontend/vite.config.ts` — multi-entry build emitting one bundle per island + a shared chunk under `../static/build/island-<name>.js`. `defineConfig({ build: { rollupOptions: { input: { ... } } } })`.
- `src/striatum/web/frontend/tsconfig.json` — strict mode; target ES2022; jsx: "react-jsx"; no `any` allowed.
- `src/striatum/web/frontend/.gitignore` — `node_modules/`, `.vite/`, `dist/` (committed output is under `../static/build/`, not `dist/`).

**Makefile targets:**

- `make ui-install` — `cd src/striatum/web/frontend && npm install`.
- `make ui-build` — `cd src/striatum/web/frontend && npm run build`. Emits to `src/striatum/web/static/build/`.
- `make ui-dev` — `cd src/striatum/web/frontend && npx vite dev`.
- `make ui-test` — `cd src/striatum/web/frontend && npx vitest run`.

**CI integration:**

- New node-22-LTS setup step.
- `make ui-build` runs in CI.
- Bundle hash check: compare CI-built `src/striatum/web/static/build/*.js` hashes against the committed hashes. Mismatch fails the build with "rerun `make ui-build` and commit".

**Wheel package-data update (pyproject.toml):**

- Add `"striatum.web.static.build" = ["*.js", "*.css"]` to `[tool.setuptools.package-data]`.

**Jinja2 template updates (Python-side scope):**

- `base.html` — add `<script type="module" src="/static/build/island-shared.js"></script>` at the top, and any island-specific script tags per page.
- `workflow_detail.html` — promote `<a class="muted">Edit</a>` to `<button class="primary-button">Edit</button>` next to "Run this workflow now".
- `view_file.html` (existing) — add mount point `<div id="island-code-viewer" data-props='{"path": "{{ path }}", "content": "..."}'></div>` for non-Markdown files.
- New template page `/view/` (no path) — renders the tree-browser mount point.
- New template page `/workflows/new` — renders the workflow-chooser mount point.
- `workflow_edit.html` — replace form-driven section with the graph-editor mount point (or augment as the synthesis chooses).

**Service.py route additions (Python-side scope):**

- New route `GET /v1/repo/tree?path=<rel>` returning `{entries: [{name, kind: "file"|"dir", size, mtime_utc}]}` for a directory.
- New route `GET /view/` (no path) rendering the tree-browser landing page.
- New route `GET /workflows/new` rendering the chooser wizard page.
- Existing `/workflows/edit/<path>` already exists; verify the island mount replaces the form-only editor.

**Implementer split:**

- `implement_toolchain_codex` (you, if codex is the systems lane): all Python-side changes above + `docs/dogfood/041/build/toolchain/HANDOFF.md`. Do NOT write into `src/striatum/web/frontend/src/` — that's claude's TypeScript-side scope.
- `implement_components_claude` (claude_code): the React island components under `frontend/src/islands/` + JS unit tests + docs + combined BUILD_HANDOFF.md.

**Disjoint write scopes verified in workflow.json.**

**Test coverage strategy (Python-side):**

- Existing UI snapshot tests must continue to pass with the new island mount points + Edit-button promotion + new routes.
- New tests for `GET /v1/repo/tree` route behavior.
- Bundle hash check in CI.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.

# Implement Toolchain Prompt (codex lane)

Blocked until `review_design_ergonomics` returns an accepting verdict.

After the gate opens, implement the **Python-side toolchain half** of accepted RFC 0038 V1. The TypeScript-side component half is implemented in parallel by claude_code (`implement_components_claude`); their write scope is disjoint from yours.

**Your scope (codex, Python + Jinja2 + CI + bundled-output-commit):**

- `src/striatum/web/frontend/package.json` — narrow dependency set per synthesis (react, react-dom, react-flow, shiki, typescript, vite, vitest, @types/*). Lockfile (`package-lock.json`) committed.
- `src/striatum/web/frontend/vite.config.ts` — multi-entry build emitting per-island bundles + shared chunk to `../static/build/`.
- `src/striatum/web/frontend/tsconfig.json` — strict mode, ES2022 target, jsx: "react-jsx", no `any`.
- `src/striatum/web/frontend/.gitignore` — `node_modules/`, `.vite/`, `dist/`.
- Makefile targets: `ui-install`, `ui-build`, `ui-dev`, `ui-test`.
- CI: node-22-LTS setup step; `make ui-build`; bundle-hash check.
- `pyproject.toml` `[tool.setuptools.package-data]` adds `"striatum.web.static.build" = ["*.js", "*.css"]`.
- Run `make ui-install && make ui-build` once and commit the resulting `src/striatum/web/static/build/*.js`/`*.css` files.
- Jinja2 template updates:
  - `base.html` — add `<script type="module" src="/static/build/island-shared.js" defer></script>` at end of head/body per synthesis choice.
  - `workflow_detail.html` — promote `<a class="muted">Edit</a>` to `<button class="primary-button">Edit</button>` next to Run.
  - `view_file.html` — add code-viewer island mount point for non-Markdown files.
  - New template page rendered at `/view/` (no path) — tree-browser mount point.
  - New template page rendered at `/workflows/new` — chooser-wizard mount point.
  - `workflow_edit.html` — add graph-editor mount point (replaces or augments form-driven section per synthesis).
- `src/striatum/service.py` — new route `GET /v1/repo/tree?path=<rel>` returning tree listings; new routes `GET /view/` and `GET /workflows/new` rendering the new pages.
- `src/striatum/web/workflows.py` — any helpers needed for the chooser-wizard or `/v1/repo/tree` route.
- Test files: `tests/test_service.py` for the new `/v1/repo/tree` route; existing `tests/test_web_ui*.py` snapshot tests still pass with the new mount points.
- `docs/dogfood/041/build/toolchain/HANDOFF.md` summarizing your shipped scope.

**Do NOT write into the TypeScript components scope** (those are owned by claude_code):

- `src/striatum/web/frontend/src/` (TypeScript React components + tests)
- `src/striatum/web/frontend/index.html`
- Documentation paths (`docs/HOW_TO_HUMAN.md`, `docs/FRONTEND_DEVELOPMENT.md`, etc.)
- `docs/dogfood/041/build/components/`, `docs/dogfood/041/BUILD_HANDOFF.md`
- `CHANGELOG.md`, `README.md` (claude owns; you focus on systems)

The disjoint scope is enforced by the workflow validator.

## Maximize sub-agent usage

Spawn sub-agents in parallel for:

- One sub-agent per config file (`package.json`, `vite.config.ts`, `tsconfig.json`).
- One sub-agent for Makefile target additions.
- One sub-agent for CI integration (`.github/workflows/*.yml` if applicable).
- One sub-agent per Jinja2 template change.
- One sub-agent for the `/v1/repo/tree` route + helpers.
- One sub-agent for each new test file or test update.

**Operational note**: `make ui-install` is a one-time setup; `make ui-build` produces the bundled output that gets committed. Both must run successfully before commit. Verify `make install/lint/typecheck/test/smoke/ui-build` all pass.

If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.

## Handoff

Produce `docs/dogfood/041/build/toolchain/HANDOFF.md` summarizing shipped scope, decisions, deferred items.

Byline: `author: implementer-codex-gpt-5.5-001` (or whatever the work packet supplies). Plain markdown line, lowercase, no decoration.

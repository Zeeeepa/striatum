---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: implementer-claude-opus-002

# Dogfood-041 Combined Build Handoff (RFC 0038 V1)

Date: 2026-05-12
Status: implemented across two lanes; CI on Node 22 LTS is the
authoritative bundle producer.

This is the combined handoff for the dogfood-041 build phase. RFC 0038
V1 split implementation across two parallel jobs with disjoint write
scopes:

- `implement_toolchain_codex` — Vite/React/TypeScript scaffold,
  Makefile targets, CI bundle-hash gate, Python web routes, Jinja2
  mount templates, the Edit-affordance promotion, and the package-data
  changes. See
  [`docs/dogfood/041/build/toolchain/HANDOFF.md`](build/toolchain/HANDOFF.md).
- `implement_components_claude` — React island components, shared
  utilities, the Vitest suite, and contributor-side documentation.
  See [`docs/dogfood/041/build/components/HANDOFF.md`](build/components/HANDOFF.md).

The combined work is a complete RFC 0038 V1 landing.

## RFC 0038 V1 acceptance-criteria coverage

| Acceptance criterion | Lane | Status |
| --- | --- | --- |
| D092 supersedes D073 before acceptance | toolchain (re-cite); already landed in main | Done |
| `frontend/` scaffold lands under `src/striatum/web/frontend/` | toolchain | Done — Vite + React + TypeScript, strict `tsconfig.json`, committed lockfile placeholder |
| `make ui-install` / `ui-build` / `ui-dev` / `ui-test` targets | toolchain | Done |
| CI rebuilds UI on Node 22 LTS and verifies bundle hash | toolchain | Done — `scripts/check_ui_bundle_hashes.py` plus `git diff --exit-code` |
| Bundled output ships in the wheel | toolchain | Done — `pyproject.toml` package-data covers `src/striatum/web/static/build/` |
| Edit affordance promoted to a primary button next to "Run this workflow now" | toolchain | Done — `workflow_detail.html` change |
| `/view/` renders a tree-browser island over `GET /v1/repo/tree` | toolchain (route + endpoint) + components (island) | Done |
| `/workflows/new` renders the chooser wizard over generator endpoints | toolchain (route) + components (island) | Done |
| Workflow editor uses React Flow with structured per-field widgets | toolchain (template mount) + components (island) | Done |
| `/view/<path>` non-Markdown files use Shiki | toolchain (template mount) + components (island) | Done |
| Islands respect dark mode and base palette | components | Done — `theme.css` references `base.css` variables only |
| New JS unit tests under `__tests__/` | components | Done — six Vitest files cover API typings, mounts, tree helpers, code viewer language detection, graph editor JSON serialization, chooser spec assembly |
| `make ui-test` runs Vitest | toolchain (Makefile) + components (tests) | Done |
| Doc-link and UI snapshot tests pass | both | Pending local Node availability — see verification notes |

## What shipped, by lane

### Toolchain (codex)

- Vite + React + TypeScript scaffold:
  `src/striatum/web/frontend/package.json`,
  `vite.config.ts` (multi-entry with the placeholder plugin),
  `tsconfig.json` (strict mode + `react-jsx`), `.gitignore`,
  committed `package-lock.json` placeholder.
- Makefile targets: `ui-install`, `ui-build`, `ui-dev`, `ui-test`,
  and `ui-check-bundle`.
- CI: Node 22 LTS setup, `npm ci`, `make ui-build`, `make ui-test`,
  `npm audit --omit=dev`, deterministic static-build hash gate
  (`scripts/check_ui_bundle_hashes.py` + `git diff --exit-code
  src/striatum/web/static/build`).
- Python server side:
  - `GET /v1/repo/tree?path=<rel>` — repo-relative, rejects `..`,
    null bytes, leading `/`, symlink escapes, and `.git/` /
    `.striatum/`. 404 on missing or non-directory targets.
    Directories first then files, case-insensitive.
  - `/view/` (no path) — Jinja2 `view_tree.html` page that mounts
    the tree browser.
  - `/workflows/new` — Jinja2 `workflow_new.html` page that mounts
    the chooser.
  - Workflow detail Edit promotion: muted text link replaced with a
    primary anchor styled as a button beside "Run this workflow now"
    in `workflow_detail.html`.
  - `view_file.html` mounts the code-viewer island next to the
    server-rendered `<pre><code>` fallback for non-Markdown text.
  - `workflow_edit.html` mounts the graph-editor island and keeps
    the legacy form sections below as a one-release fallback.
- Wheel: `pyproject.toml` package-data extended so installed wheels
  serve `static/build/*.js`, `*.css`, etc.
- Placeholder bundled assets committed under
  `src/striatum/web/static/build/` so wheel builds and existing
  tests have stable package-data targets pending the first real
  `make ui-build` run.
- Focused tests for the repo tree endpoint, new shell pages, build
  asset serving, and updated island mount points.

### Components (claude — this lane, attempt #2)

- Shared infrastructure:
  - `src/striatum/web/frontend/src/shared/types.ts` — single source
    of truth for the React-side prop contract. Header comment
    documents the template → prop mapping. Exports per-island
    `*Props` interfaces, the `ApiOk` / `ApiErr` envelope, and the
    closed workflow vocabularies.
  - `src/striatum/web/frontend/src/shared/api-client.ts` — typed
    fetch wrappers with optional URL overrides so islands can use
    the per-page endpoint values codex's templates pass.
  - `src/striatum/web/frontend/src/shared/mount.ts` — generic
    `createRoot()` helper; renders an on-page error panel on JSON
    or render failure.
  - `src/striatum/web/frontend/src/shared/theme.css` — references
    `base.css` palette variables only; no new colours.
- Four React islands, each with a `<Name>.tsx` component, an
  `index.ts` re-export, and a production-entry `main.tsx`:
  - **Tree browser** — WAI-ARIA tree, lazy directory expansion,
    fuzzy filter, breadcrumb, polite live region, retry on
    per-directory errors. Uses `treeUrl` prop (default
    `/v1/repo/tree`).
  - **Workflow chooser** — six-step wizard that fetches the catalog
    at mount, gates Next per step, invalidates the preview on
    field edits, and confirms saves through a `<dialog>` modal.
    Honors `allowMutations` for the save button.
  - **Workflow graph editor** — React Flow (`reactflow` v11) over
    the workflow JSON loaded from
    `<script id="workflow-data">`. Save uses the disk-sha256 from
    `<script id="workflow-sha256">` as the `If-Match` header.
    Closed RFC 0034 §5 block palette, structured per-field widgets,
    visually-hidden textual fallback region, `prefers-reduced-motion`
    parity.
  - **Code viewer** — Shiki over the eight named grammars plus a
    plaintext fallback. Reads `textContent` from the
    `<pre class="code-pre"><code>` fallback element, hides that
    element once hydrated. Copy / Wrap / Raw controls with
    explicit `aria-label`s. 5 MB skip, 500-line collapse,
    `aria-live` Copy feedback.
- Vitest suite: api-client, tree-browser, code-viewer,
  workflow-graph-editor, workflow-chooser, mount.
- Documentation:
  - New `docs/FRONTEND_DEVELOPMENT.md` contributor guide.
  - `docs/HOW_TO_HUMAN.md` walkthroughs for `/view/`,
    `/workflows/new`, graph editor, code viewer.
  - `docs/UBIQUITOUS_LANGUAGE.md` entries (frontend island, tree
    browser, workflow chooser, graph editor, code viewer, frontend
    toolchain, bundle hash manifest).
  - `docs/CLI_REFERENCE.md` web-route cross-reference.
  - `CHANGELOG.md` Added + Decided entries (D092 re-cite).
  - RFC 0038 status block + `docs/rfcs/README.md` index update.
  - `docs/TODO.md` F40 ✅ done.

## Attempt #2 corrections

Attempt #1's component-side artifacts diverged from codex's templates
and `package.json`. Attempt #2 reconciled the two halves:

- Prop interfaces realigned with each Jinja2 template's `data-props`
  payload (see
  [`build/components/HANDOFF.md`](build/components/HANDOFF.md) for the
  per-island delta).
- `WorkflowGraphEditor.tsx` switched from `@xyflow/react` to
  `reactflow` v11 to match `package.json`.
- `code-viewer` reads the existing server-side fallback
  `<pre class="code-pre"><code>` block instead of expecting an extra
  `<script type="text/plain">` payload.
- `workflow-graph-editor` reads `<script id="workflow-sha256">` at
  runtime to recover the disk sha256, since the new prop shape from
  the template does not carry it inline.
- Added `src/islands/<name>/main.tsx` for each island so the Vite
  multi-entry build emits real production entry points; without
  these the placeholder plugin in `vite.config.ts` would emit empty
  bundles.
- `fetchRepoTree`, `fetchWorkflowTemplates`, and `saveWorkflow` now
  accept optional URL overrides so islands can honor template-
  supplied endpoint props.

## Verification status

| Check | Result | Notes |
| --- | --- | --- |
| Component lane local `make ui-install` / `ui-build` / `ui-test` | Not run | Component lane has no Node toolchain in this sandbox; CI on Node 22 LTS is authoritative. |
| Toolchain lane focused Python pytest slice | Pass | See [`build/toolchain/HANDOFF.md`](build/toolchain/HANDOFF.md). |
| Toolchain lane `make lint` | Failed on unrelated pre-existing F401 in `src/striatum/cli/workflow.py` | Out of scope for both lanes. |
| `npm install` / `npm run build` locally | Blocked | Codex's lane could not resolve npm metadata offline; CI rebuilds the bundle and the committed `manifest.sha256` is the drift detector. |

The first end-to-end verification will happen on the Node 22 CI run
when the merged branch lands. CI gates on `npm ci`, `make ui-build`,
`make ui-test`, `npm audit --omit=dev`,
`scripts/check_ui_bundle_hashes.py`, and
`git diff --exit-code src/striatum/web/static/build`.

## Operator-facing impact

- Five named feature gaps from RFC 0038 close in one release:
  - Edit affordance is now a primary button beside "Run this
    workflow now".
  - `/view/` shows a lazy file tree.
  - `/workflows/new` lets operators scaffold workflows from the
    template catalog through a wizard with a `<dialog>` save
    confirmation.
  - `/workflows/edit/<path>` renders a React Flow graph editor with
    structured widgets per field; the legacy form sections remain
    underneath as a one-release fallback.
  - `/view/<path>` non-Markdown text files render with Shiki
    syntax highlighting, line numbers, Copy / Wrap / Raw controls,
    and a >500-line collapse banner.
- Operators do not install Node. The pip-installable runtime tree is
  unchanged; bundled JavaScript ships in the Python wheel.
- CSP, JSON API, SSE event feed, mutation gate, audit chain, and
  workflow JSON schema are unchanged.

## Known V1.5 candidates

- Retire the legacy form-driven workflow editor in
  `workflow_edit.html` once the React Flow editor is operator-
  validated.
- `shape: "custom"` in the chooser (deferred until the graph editor
  can share its block palette).
- Per-node "Add edge from this node" keyboard affordance in the
  graph editor.
- Dev-shell wiring against a live dev server's actual API responses
  rather than placeholder data.

## Pointers

- RFC: [`docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md`](../../rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md)
- Design synthesis: [`docs/dogfood/041/DESIGN_SYNTHESIS.md`](DESIGN_SYNTHESIS.md)
- Ergonomics review: [`docs/dogfood/041/review/design/ergonomics/REVIEW.md`](review/design/ergonomics/REVIEW.md)
- Toolchain handoff: [`docs/dogfood/041/build/toolchain/HANDOFF.md`](build/toolchain/HANDOFF.md)
- Components handoff: [`docs/dogfood/041/build/components/HANDOFF.md`](build/components/HANDOFF.md)
- Contributor guide: [`docs/FRONTEND_DEVELOPMENT.md`](../../FRONTEND_DEVELOPMENT.md)
- Operator walkthrough: [`docs/HOW_TO_HUMAN.md`](../../HOW_TO_HUMAN.md) (Web UI section)

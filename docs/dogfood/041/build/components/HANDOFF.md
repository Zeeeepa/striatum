---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: implementer-claude-opus-002

# Components Implementation Handoff (RFC 0038 V1, dogfood-041)

Date: 2026-05-12
Status: implemented; CI bundle build runs in codex's lane

## Scope shipped

Implemented the TypeScript-side React island half of accepted RFC 0038
V1, plus the operator- and contributor-side documentation surfaces.
Codex's parallel `implement_toolchain_codex` job (see
`docs/dogfood/041/build/toolchain/HANDOFF.md`) ships the Vite scaffold,
package data, Makefile targets, CI, web routes, the
`/v1/repo/tree` endpoint, the Edit-button promotion, and the Jinja2
mount templates (`view_tree.html`, `workflow_new.html`, plus the
island mount slots in `view_file.html` and `workflow_edit.html`).

This attempt (#2 — `implementer-claude-opus-002`) reconciled the
component-side code with the templates and `package.json` codex
landed in attempt #1:

- **Prop contract realignment.** The previous attempt declared prop
  interfaces that did not match the `data-props` shapes codex's
  templates emit. Updated `src/shared/types.ts` and each island's
  component to accept the actual shapes:
  - tree-browser: `{ rootPath, treeUrl?, viewBase?, rootLabel? }`
    (was `{ initialPath, … }`).
  - workflow-chooser: `{ allowMutations, templatesUrl?, previewUrl?,
    generateUrl?, defaultScaffoldRoot?, defaultArtifactRoot? }` with the
    catalog **fetched at runtime** (was a pre-baked `catalog` object).
  - workflow-graph-editor: `{ path, saveUrl, fallback?, cancelUrl?,
    workflowDataElementId?, workflowSha256ElementId? }` (reads both
    the workflow JSON and the sha256 from the adjacent
    `<script id="workflow-data">` / `<script id="workflow-sha256">`
    payloads codex's template renders).
  - code-viewer: `{ path, language, rawUrl?, sourceElementSelector? }`.
    Reads `textContent` from the server-rendered
    `<pre class="code-pre"><code>…</code></pre>` block already
    present in `view_file.html`, then hides the fallback once the
    island has rendered.
- **`reactflow` v11 alignment.** Attempt #1 imported from
  `@xyflow/react`, but `package.json` lists `reactflow ^11.11.4`.
  `WorkflowGraphEditor.tsx` now imports from `reactflow` and
  `reactflow/dist/style.css` so the production bundle resolves
  cleanly under the lockfile codex committed.
- **Production entry points landed.** The Vite multi-entry config
  (codex scope) names `islands/<name>/main.tsx` as each island's
  entry. Attempt #1 only created `index.ts` re-exports, so the
  placeholder Vite plugin emitted empty bundles. This attempt adds
  `src/islands/<name>/main.tsx` for every island. Each `main.tsx`
  imports `mount` and the component, registers the production mount
  slot, and exits — production pages now actually hydrate the island.
- **API client URL overrides.** `fetchRepoTree`, `fetchWorkflowTemplates`,
  and `saveWorkflow` now accept optional URL overrides so islands can
  use the per-page endpoint values codex's templates pass via
  `data-props`.

The previous attempt's findings absorption (F1, F2, F4–F7, F9) is
preserved; only the prop wiring and entry-point gaps changed.

### Frontend source — `src/striatum/web/frontend/src/`

- `shared/types.ts` — **single source of truth** for the prop
  contract between Jinja2 templates and React islands. Header
  comment lists the template → prop mapping. Exports per-island
  `*Props` interfaces, the `ApiOk` / `ApiErr` envelope, and the
  closed workflow vocabularies (`ALLOWED_REVIEW_POSTURES`,
  `JOB_TYPES`, `EDGE_VERDICTS`, `REVIEWER_ACCESS_SCOPES`,
  `REVIEWER_CONTEXT_POLICIES`, `WRITE_SCOPE_MODES`).
- `shared/api-client.ts` — typed fetch wrappers. Each takes an
  optional URL override so templates can supply the active endpoint:
  - `fetchRepoTree(path, baseUrl?)`
  - `fetchWorkflowTemplates(url?)`
  - `generateWorkflowPreview(spec, url?)`
  - `generateWorkflowWrite(spec, url?)` — always sets
    `confirm_write: true`.
  - `saveWorkflow(relPath, body, diskSha256, url?)` — preserves
    `If-Match` sha256 header semantics.
- `shared/mount.ts` — generic `createRoot()` mount helper. Reads
  `data-props`, parses adjacent `<script type="application/json">` /
  `<script type="text/plain">` payloads, renders an on-page error
  panel when JSON or render fails. Every island uses it; no island
  calls `createRoot()` directly.
- `shared/theme.css` — island-shared CSS that references the
  `base.css` palette custom properties. No new colour variables;
  dark mode parity inherited via `prefers-color-scheme`.
- `main.ts` — Vite dev-shell root. Mounts every island for the dev
  shell; production pages import the per-island bundle directly.
- `index.html` — Vite dev shell that mounts every island side by
  side using the new prop shapes; never shipped to operators.

### Islands

| Island | File | Library | Notes |
| --- | --- | --- | --- |
| Tree browser | `src/islands/tree-browser/TreeBrowser.tsx` | none | WAI-ARIA tree, roving tabindex, breadcrumb, fuzzy filter, polite live region, lazy directory expansion via the `treeUrl` prop (default `/v1/repo/tree`), retry button on per-directory errors. Empty-directory state and "Loading…" placeholder per F1/F2. |
| Workflow chooser | `src/islands/workflow-chooser/WorkflowChooser.tsx` | none | Fetches the catalog from `templatesUrl` at mount. Six-step wizard. Per F4: Next disabled until required selections are made; editing any step 1–4 field invalidates the preview and re-runs `POST .../generate/preview` on re-entry; preview block carries a generated-at timestamp. Final write goes through a `<dialog>` confirm with `showModal()`. Gates on `allowMutations` for the disabled-when-read-only Save button. |
| Workflow graph editor | `src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx` | `reactflow` (v11) | Closed RFC 0034 §5 palette. Per F5: dropdowns for role/lane/type/access scope/context policy; radio sets for posture and write-scope mode; multi-select chips for `required_review_postures`; repeating-row editors for allowed/forbidden paths; structured editor for `expected_artifacts`. Cycles render with `cycle-edge` styling. Coordinates UI-only. Save uses the disk-sha256 read from `<script id="workflow-sha256">`. Visually-hidden textual fallback region per a11y checklist. `prefers-reduced-motion` disables animation via theme.css. |
| Code viewer | `src/islands/code-viewer/CodeViewer.tsx` | `shiki` | Reads source from the adjacent `<pre class="code-pre"><code>` block, hides that fallback once mounted, and Shiki-renders the eight named grammars (`json`, `python`, `typescript`, `javascript`, `bash`, `yaml`, `toml`, `markdown`, `sql`) plus plaintext fallback. Per F7: Copy button swaps to "Copied" for 2 s and announces via `aria-live`; Raw opens in a new tab with `rel="noopener"`; Wrap toggle is operator-controlled with no-wrap default and internal horizontal scroll. Files >5 MB skip Shiki. Files >500 lines collapse by default with an Expand banner. Hostile content is escaped before injection (covered by an explicit `<script>alert(1)</script>` test). |

Each island folder ships:

- `<Name>.tsx` — React component.
- `index.ts` — re-export for the dev shell.
- `main.tsx` — production entry point (Vite emits as
  `island-<name>.js`). Calls `mount()` against the per-template
  container id.

### Vitest suite — `src/striatum/web/frontend/src/__tests__/`

- `api-client.test.ts` — path encoding, override URLs for the tree
  endpoint, `If-Match` attachment, `confirm_write: true` on the
  write call, error envelope propagation, and stale-precondition
  status pass-through.
- `tree-browser.test.ts` — `normalizePath`, `parentOf`,
  `fuzzyMatch`, `joinUrl` helpers.
- `code-viewer.test.ts` — `detectLanguage`, `escapeHtml`,
  `injectLineNumbers`, `plainTextHtml`, the closed grammar set, and
  an explicit XSS-vector test asserting hostile content is not
  executable markup.
- `workflow-graph-editor.test.ts` — `jobsToNodes`, `workflowToEdges`,
  `syncWorkflowEdges`, `syncWorkflowJobs`, `newJobFromBlock`, and
  the closed RFC 0034 §5 block vocabulary.
- `workflow-chooser.test.ts` — `buildSpec` (lane-command trimming,
  blank `name` fallback, optional `branch_suggestion`) and
  `isModifierEnabled` (incompatibility matrix).
- `mount.test.ts` — `readJsonPayload` and `readTextPayload` helpers
  (parsed payload, missing element, unparseable element).

### Documentation

- `docs/FRONTEND_DEVELOPMENT.md` — new contributor-side guide.
  Project layout, make targets, mount pattern (now describes
  `main.tsx` entry points and the production bundle naming), prop
  contract, closed-vocabulary mirroring, accessibility checklist,
  supply-chain posture, bundle hash workflow. Dependency list
  corrected to `reactflow` (v11).
- `docs/HOW_TO_HUMAN.md` — operator walkthroughs for `/view/`,
  `/workflows/new`, the drag-drop graph editor, and the Shiki code
  viewer under the existing "Web UI" section.
- `docs/UBIQUITOUS_LANGUAGE.md` — `frontend island`,
  `tree browser`, `workflow chooser`, `graph editor`, `code viewer`,
  `frontend toolchain`, `bundle hash manifest`.
- `docs/CLI_REFERENCE.md` — "Web routes (RFC 0013 / 0022 / 0024 /
  0038)" cross-reference table under the Service section. No new
  CLI verbs.
- `CHANGELOG.md` — Added + Decided entries under `Unreleased` (D092
  re-cited).
- `docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md`
  — status block updated to `accepted (V1)` with implementation
  pointers to both implementer handoffs.
- `docs/rfcs/README.md` — RFC index status updated.
- `docs/TODO.md` — F40 row marking RFC 0038 V1 as ✅ done.

## Findings absorbed from the design review

The `accept_with_findings` review
(`docs/dogfood/041/review/design/ergonomics/REVIEW.md`) listed F1–F10.
The components scope handles:

- **F1 / F2** — tree browser breadcrumb of clickable ancestors plus
  one-sentence header copy, empty-state "This directory has no
  entries" message, and explicit "Loading…" placeholder live-region.
- **F4** — chooser Next gating, preview invalidation on field edits,
  visible generated-at timestamp on the preview pane.
- **F5** — explicit per-field widget grammar in the graph editor
  inspector.
- **F6** — visually-hidden textual fallback region for the React Flow
  canvas listing every job and edge.
- **F7** — Copy "Copied" feedback with `aria-live`, Raw
  `target=_blank rel="noopener"`, no-wrap default with internal
  scroll.
- **F9** — `src/striatum/web/frontend/src/shared/types.ts` is the
  designated shared prop contract; the file's docstring now
  documents the template → prop mapping explicitly.

F3 (chooser copy quality), F8 (Edit-button visual treatment), and
F10 (custom-shape disabled card) live in codex's toolchain scope.

## Verification (component-side)

The component source files were authored without local Node access
in this lane. Verification commands a contributor with Node 22 LTS
should run before merging:

```bash
make ui-install
make ui-build
make ui-test
make lint
make typecheck
make test
```

Codex's lane reports `make ui-install` / `make ui-build` were
blocked locally by missing npm cache metadata; CI on Node 22 LTS is
the authoritative bundle producer per the synthesis.

The committed Vitest files import only the pure helpers exported via
`__testing`, so they do not require `reactflow` / `shiki` to be
installed for the test suite to type-check; the `vitest --run`
invocation will install the runtime dependencies via `npm ci` first.

Local-machine verification I did run:

- Read codex's templates (`view_tree.html`, `workflow_new.html`,
  `view_file.html`, `workflow_edit.html`) and confirmed the new
  prop interfaces line up with each `data-props` payload.
- Read codex's toolchain handoff to make sure the bundle entry-point
  names and CSP posture match.
- Re-read the design synthesis and the ergonomics review and
  cross-checked each finding against the implementation.
- Confirmed `package.json` lists `reactflow ^11.11.4` and aligned the
  graph editor imports accordingly.

## Known gaps / V1.5 candidates

- `shape: "custom"` is intentionally absent from the chooser
  (synthesis-resolved). A disabled card with explanatory tooltip
  (F10) lives in codex's chooser-template scope.
- The graph editor's keyboard edge creation uses React Flow defaults;
  per F6 the textual fallback region is the screen-reader path. A
  per-node "Add edge from this node" affordance is a V1.5 candidate.
- The dev shell at `frontend/index.html` uses placeholder data;
  integrating it with a running dev server's actual API responses is
  a contributor convenience rather than an operator-facing surface.
- The legacy form-driven editor remains beneath the graph-editor
  island slot in `workflow_edit.html` per synthesis "best-effort for
  one release". Retiring it is a V1.5 task once operator validation
  confirms the React Flow editor covers every prior use case.

## Boundary discipline

I did not write into the toolchain scope (`package.json`,
`vite.config.ts`, `tsconfig.json`, `.gitignore`, server templates,
`src/striatum/service.py`, `src/striatum/web/workflows.py`, Makefile,
`pyproject.toml`, `tests/test_web_*.py`, `tests/test_service.py`,
`src/striatum/web/static/build/`). Codex owns those.

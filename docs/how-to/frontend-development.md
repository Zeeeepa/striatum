# Frontend Development

Status: V1
Date: 2026-05-12
author: implementer-claude-opus-001

This guide covers the contributor-side TypeScript toolchain that ships
the Striatum web UI's React island components. It complements
[`docs/SPEC.md`](SPEC.md) (architecture and product boundary),
[`docs/HOW_TO_HUMAN.md`](HOW_TO_HUMAN.md) (operator walkthroughs), and
[`docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md`](rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md)
(the RFC that introduced this surface).

Operators do not need Node installed. Operators install Striatum from
Go release archives; the Vite-bundled JavaScript ships inside the
Go binary under `src/striatum/web/static/build/`.

## When you need this guide

You only need the Node toolchain when you are working on:

- React components under `src/striatum/web/frontend/src/islands/`,
- shared utilities under `src/striatum/web/frontend/src/shared/`,
- island-side CSS,
- Vitest unit tests under `src/striatum/web/frontend/src/__tests__/`,
- the dev shell at `src/striatum/web/frontend/index.html`.

Server-rendered Jinja2 templates, the Python web routes, the static
vanilla-JS files, and the existing `app.css` / `base.css` are not part
of this surface.

## Prerequisites

- **Operating system**: Linux or macOS. Windows contributor support is
  out of scope for RFC 0038 V1.
- **Node**: 22 LTS. Match the version CI uses so committed bundle hashes
  reproduce locally. `node --version` should report `v22.*`.
- **npm**: ships with Node 22.

## Project layout

```text
src/striatum/web/frontend/
  package.json          # Node-side dependencies (codex scope)
  package-lock.json     # committed (codex scope)
  vite.config.ts        # multi-entry build (codex scope)
  tsconfig.json         # strict TS config (codex scope)
  .gitignore            # node_modules, .vite, etc. (codex scope)
  index.html            # Vite dev shell — runs every island side by side
  src/
    main.ts             # dev shell entry point; mounts every island
    islands/
      tree-browser/
        TreeBrowser.tsx
        index.ts        # re-exports the component for the dev shell
        main.tsx        # production entry — Vite emits this as island-tree-browser.js
      workflow-chooser/
      workflow-graph-editor/
      code-viewer/
    shared/
      api-client.ts     # typed fetch wrappers
      types.ts          # prop contracts + closed vocabularies (source of truth)
      mount.ts          # generic createRoot mount helper
      theme.css         # references base.css custom properties only
    __tests__/          # Vitest suite

src/striatum/web/static/build/   # committed Vite output (codex scope)
  island-tree-browser.js
  island-workflow-chooser.js
  island-workflow-graph-editor.js
  island-code-viewer.js
  island-shared-<hash>.js
  island-style.css
  manifest.sha256
```

## Make targets

```bash
make ui-install   # npm ci inside src/striatum/web/frontend/
make ui-build     # npm run build → static/build/ + manifest.sha256
make ui-dev       # vite dev server with hot reload
make ui-test      # vitest --run
```

`make install` does not run any of these. The installation path is Go-only.

## Local feature loop

```bash
make ui-install        # one time per checkout
make ui-dev            # opens http://localhost:5173 with the dev shell
# Edit src/striatum/web/frontend/src/islands/<name>/<file>.tsx
# Save → Vite hot-reloads in the dev shell
make ui-test           # vitest suite, fast
make ui-build          # produces the committed bundle
git diff src/striatum/web/static/build/  # confirm only intended hash drift
```

CI rebuilds the bundle on Node 22 LTS and compares the resulting hashes
against the committed `manifest.sha256`. A mismatch fails CI with a
reminder to rerun `make ui-build` and commit.

## Island mounting pattern

Every island lives in its own folder under `src/islands/`. Every island
mounts into a Jinja2-owned page slot. The Jinja2 template emits:

```html
<div id="island-<name>" data-props='{{ props | tojson }}'></div>
<script type="module" src="/static/build/island-<name>.js" defer></script>
```

Small scalar props live in `data-props`. Large payloads (workflow JSON,
file bytes) live in adjacent `<script type="application/json">` or
`<script type="text/plain">` tags so escaped attributes do not balloon
into multi-megabyte values:

```html
<div id="island-workflow-graph-editor"
     data-props='{{ editor_props | tojson }}'></div>
<script id="workflow-data" type="application/json">
  {{ workflow_json | safe }}
</script>
<script id="workflow-sha256" type="application/json">
  "{{ workflow_sha256 }}"
</script>
<script type="module" src="/static/build/island-workflow-graph-editor.js" defer></script>
```

The shared `mount()` helper in `src/shared/mount.ts` parses
`data-props`, calls `createRoot()`, and renders an on-page error panel
if the JSON or render fails. Use it for every island. Do not call
`createRoot()` directly from an island file.

Production pages must keep the existing CSP shape: no inline scripts,
no `unsafe-inline`, no `unsafe-eval`, no CDN, no external runtime fetch.

## Prop contracts

`src/shared/types.ts` is the single source of truth for the
TypeScript-side prop contract between Jinja2 templates and React
islands. Every island imports its prop type from this file. Codex's
Jinja2 templates mirror these field shapes when emitting `tojson`
payloads. Any prop-shape change goes through `types.ts` first; the
template change follows.

Two consequences:

1. The TS prop type and the Python `tojson` shape must match. There is
   no runtime contract validator; tests on each side cover the most
   common drift.
2. New islands add a new exported `*Props` interface to `types.ts` and a
   matching mount block in `main.ts`.

## Adding a new island

1. Create `src/islands/<name>/<Name>.tsx` exporting a default React
   component that takes a `<Name>Props` argument.
2. Create `src/islands/<name>/index.ts` that re-exports the component
   and any test-only helpers (`__testing`).
3. Create `src/islands/<name>/main.tsx` that imports `mount` and the
   component and registers the production mount slot. The Vite multi-
   entry build emits this file as `island-<name>.js`.
4. Add a `<Name>Props` interface to `src/shared/types.ts`.
5. Add a `mount<…>(…)` block to `src/main.ts` (and to the dev shell at
   `index.html`) so contributors can exercise the island without a
   server roundtrip.
6. Ask codex to extend `vite.config.ts` with the new entry point and
   `pyproject.toml` package-data accordingly.
7. Add Vitest coverage under `src/__tests__/` for the pure helpers
   (filter predicates, formatters, JSON serializers).

## Closed vocabularies

`src/shared/types.ts` exports the closed enums used by the workflow
graph editor (`ALLOWED_REVIEW_POSTURES`, `JOB_TYPES`,
`EDGE_VERDICTS`, `REVIEWER_ACCESS_SCOPES`,
`REVIEWER_CONTEXT_POLICIES`, `WRITE_SCOPE_MODES`). These mirror the
workflow validator vocabulary in `src/striatum/workflow.py`. When the
validator's vocabulary changes, update both.

## Accessibility checklist

Every island must satisfy:

- Keyboard navigation completeness for its primary task. Tree browser
  uses roving tabindex and arrow-key movement. The chooser wizard moves
  through steps with Tab. The code viewer is read-only and focusable
  per row.
- Visible focus indicators in light and dark modes. Use the
  `--accent` border / outline pattern from `theme.css`.
- No `tabindex` greater than 0.
- `<dialog>` confirmation in the chooser uses `showModal()`, moves
  focus inside on open, restores focus on close, closes on Esc, and
  does not let Tab escape.
- Tree browser uses `role="tree"`, `role="treeitem"`,
  `aria-expanded`, `aria-level`, and a polite live region for load
  failures.
- Chooser radio cards use `radiogroup` semantics.
- Graph editor canvas exposes a textual fallback region (visually
  hidden but discoverable to screen readers) listing every job and
  edge.
- Code viewer line numbers are `aria-hidden`; Copy / Wrap / Raw
  controls have exact `aria-label`s.
- All text meets WCAG AA contrast using existing `base.css` palette
  variables.
- `prefers-reduced-motion` disables React Flow animation.
- The RFC 0037 skip link still reaches page content on `/view/`,
  `/workflows/new`, and `/workflows/edit/<path>`.

## Bundle hash workflow

`src/striatum/web/static/build/manifest.sha256` records deterministic
SHA-256 hashes for every generated file in the committed bundle. CI
rebuilds the bundle and refuses any drift. Locally:

```bash
make ui-build                           # rewrites bundle + manifest
git diff src/striatum/web/static/build/ # review the hash + content delta
git add src/striatum/web/static/build/  # commit alongside source changes
```

Treat the manifest as a drift detector — it prevents source and
committed bundle from silently diverging. It is not a cryptographic
supply-chain control.

## Supply-chain posture

- We use npm with a committed `package-lock.json` and `npm ci` in CI.
- The runtime dependency set is intentionally narrow: `react`,
  `react-dom`, `reactflow`, `shiki`. Anything new requires a
  decision-log entry.
- CI runs `npm audit --omit=dev`. High or critical runtime
  vulnerabilities fail the job. Dev-only findings are recorded but do
  not automatically block.
- No CDN. No external runtime fetch. Every script and asset is
  same-origin.

## Testing

- `make ui-test` runs Vitest in `--run` mode. Use this before
  `make test` (the Python suite); Vitest finishes in seconds.
- Vitest covers the typed API client, mount helpers, tree browser
  filter and path helpers, code viewer language detection / escape
  helpers / line-number injector, workflow chooser spec assembly, and
  workflow graph editor JSON serialization.
- The escape helpers and the language fallback path explicitly test
  hostile content (`<script>alert(1)</script>`) to confirm the viewer
  cannot inject executable markup.

## See also

- [`docs/SPEC.md`](SPEC.md) — frontend toolchain section (codex).
- [`docs/UBIQUITOUS_LANGUAGE.md`](UBIQUITOUS_LANGUAGE.md) — frontend
  island vocabulary.
- [`docs/HOW_TO_HUMAN.md`](HOW_TO_HUMAN.md) — `/view/`, `/workflows/new`,
  graph editor, code viewer walkthroughs.
- [`docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md`](rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md)
  — the RFC.
- [`docs/dogfood/041/DESIGN_SYNTHESIS.md`](dogfood/041/DESIGN_SYNTHESIS.md)
  — the accepted synthesis.

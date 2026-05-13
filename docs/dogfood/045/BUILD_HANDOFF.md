author: implementer-claude-1

# Dogfood-045 Build Handoff — RFC 0038 V1.5 Web UI Integration Gaps

Run: `run_8a909addd31e4455b85ad58768169e4a`
Branch: `striatum/dogfood-045-rfc-0038-v1-5`
Workflow: `docs/dogfood/045/workflow.json` — 9-job single-track for RFC
0038 V1.5 (F1-F4 + supply-chain hygiene findings from dogfood-041
deferred by cycle-exhaustion override
`dec_251e8a5f3d674c409de0dad9eacd5844`)

This handoff consolidates the implementation HANDOFF
(`docs/dogfood/045/build/HANDOFF.md`) into the combined per-finding
narrative plus the three build review verdicts. The per-finding HANDOFF
remains authoritative for file-level detail.

## Scope

RFC 0038 V1.5 closes the four codex attempt-2 findings (F1-F4) from
the dogfood-041 build review iteration 2 plus the gemini attempt-1
supply-chain hygiene findings and the claude attempt-2
medium-severity ergonomics polish. No new public CLI verbs, no new
MCP tool names, no public bundle URLs change, no daemon RPC envelope
changes. The frontend toolchain stays Vite + React + TypeScript per
D092; the islands architecture (Jinja2 page shells + React islands)
is preserved.

**Implementer:** claude (frontend TypeScript / Vite work). This is the
first dogfood deliberately not using codex as implementer to avoid the
codex/codex implementer+reviewer anti-pattern after four independent
instances (D095-D098). The codex reviewer in this run is therefore
**reviewer-of-claude-implementer**, not codex/codex — a structurally
different pairing from the cycle-exhaustion overrides.

## Per-finding implementation

### F1 — Placeholder-plugin removal

`src/striatum/web/frontend/vite.config.ts` no longer imports or
defines `placeholderIslandPlugin`. The `plugins` array is now
`[react()]`. `manifest` flipped to `false` per synthesis F4 so the
build no longer emits `.vite/manifest.json`; the existing
`manifest.sha256` remains the single committed manifest. The Rollup
`input` table maps `island-shared` to a new non-mounting shared
entry (F3) and the four stable island entries to their respective
island `main.tsx` files.

A new `make ui-verify-bundle` target asserts the build output is real
Vite output. It rejects: (a) any stable island entry whose body
contains the V1 sentinel `Striatum frontend island placeholder
loaded`; (b) any `island-shared-*.js` chunk containing the same
sentinel; (c) any stable island entry under 1024 bytes (unless a
sibling `island-shared-*.js` chunk is ≥ 1024 bytes, which is the
legitimate factored-chunk case). `make ui-check-bundle` now depends
on both `ui-build` and `ui-verify-bundle`, so build drift and
placeholder sneak-throughs both fall out of the same gate.

A Python regression test
`tests/test_web_ui.py::test_island_bundles_have_no_placeholder_sentinel`
reads each stable island bundle through `importlib.resources` and
asserts the sentinel is absent. This catches the placeholder slipping
through `pip install`.

### F2 — Chooser prop contract

The chooser was rewritten around the server-stable `templates` shape
per synthesis. The `/workflow-templates` route in
`src/striatum/service.py::_handle_workflow_templates` is unchanged —
it already returns
`{"ok": true, "data": {"templates": list_templates(kind=kind)}}`
backed by `striatum.workflow_generator.catalog.list_templates`.

Frontend changes:

- `src/striatum/web/frontend/src/shared/types.ts` adds
  `WorkflowTemplate` (mirroring `template_id`, `kind`, `display_name`,
  `summary`, `recommended_for`, `default_lane_sets`,
  `required_options`, `graph_preview`) and
  `WorkflowTemplateListResponse = { templates: WorkflowTemplate[] }`.
  The dead `WorkflowShape`, `WorkflowLaneSet`, and
  `WorkflowTemplateCatalog` types (which referenced nonexistent
  server fields) are deleted.
- `src/striatum/web/frontend/src/shared/api-client.ts` —
  `fetchWorkflowTemplates()` now returns
  `ApiResult<WorkflowTemplateListResponse>`.
- `src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`
  reads `res.data.templates`, partitions by `kind`, derives `shape`
  from the picked `kind: "shape"` row's `template_id`, and pre-fills
  `lane_set` from the first overlapping `default_lane_sets` entry
  that actually exists in the catalog. The V1 modifier step is
  removed because `catalog.modifiers` was never returned. Wizard is
  now four steps: Template → Details (workflow_id, name,
  scaffold_root, artifact_root, branch_suggestion, lane_set) →
  Preview → Save. `__testing` exports `buildSpec` and
  `recommendedForText`; the V1 `isModifierEnabled` export was
  removed along with the modifier UI.
- `src/striatum/web/templates/workflow_new.html` is unchanged — it
  already emits `{ allowMutations, templatesUrl, previewUrl,
  generateUrl }`, matching the V1.5 `WorkflowChooserProps` shape.

### F3 — Island-shared double-mount fix

The root cause was that the V1 `vite.config.ts` mapped the
`island-shared` Rollup input to `src/main.ts`, which is the dev-shell
mounter that calls `createRoot` on every island. Loading
`island-shared.js` from `base.html` on every page would therefore
mount each island twice — once from `island-shared.js` and once from
the page-specific island bundle.

Fix: created
`src/striatum/web/frontend/src/shared/island-shared-entry.ts`:

```ts
import "./theme.css";
export {};
```

This file is now the Rollup input for the `island-shared` bundle.
`src/main.ts` still exists and still mounts every island, but it is
only loaded by the Vite dev server (`make ui-dev`) via
`frontend/index.html`. Production never loads `src/main.ts` because
no Rollup input points at it.

`src/striatum/web/templates/base.html` is unchanged: it still loads
`/static/build/island-shared.js`. The build-time guarantee comes
entirely from `vite.config.ts` — the `island-shared` Rollup input is
the non-mounting entry, not `src/main.ts`. There is no Jinja2
conditional, only the single Rollup mapping.

Vitest regression
`src/striatum/web/frontend/src/__tests__/island-shared-no-mount.test.ts`
mocks `react-dom/client.createRoot`, imports the shared entry plus
the chooser entry into a JSDOM page that exposes only
`#island-workflow-chooser`, and asserts `createRoot` is called
exactly once for that container.

### F4 — Output and package-data layout

Output directory stays at `src/striatum/web/static/build/`. Public
URLs unchanged: `/static/build/island-shared.js`,
`/static/build/island-tree-browser.js`,
`/static/build/island-workflow-chooser.js`,
`/static/build/island-workflow-graph-editor.js`,
`/static/build/island-code-viewer.js`, `/static/build/style.css`,
`/static/build/manifest.sha256`.

`pyproject.toml [tool.setuptools.package-data]` already matches the
V1.5 `manifest: false` layout (`"striatum.web.static" = [..., "build/*.js",
"build/*.css", "build/*.sha256"]`, plus the explicit
`"striatum.web.static.build" = ["*.js", "*.css", "*.sha256"]`
sub-package entry). Because the implementation picked `manifest:
false`, the `.vite/*.json` globs the synthesis listed as the
alternative are intentionally absent. No pyproject.toml edit was
needed (and the file is outside this packet's `write_scope` anyway).

`MANIFEST.in` is intentionally not added: setuptools ≥77 (the project
floor) uses `pyproject.toml` package-data for both wheels and sdists,
so sdists pick up `src/striatum/web/static/build/*` from the same
source of truth.

Two new Python tests cover packaging through `importlib.resources`:

- `test_island_bundles_have_no_placeholder_sentinel` (sentinel guard
  for every stable island entry).
- `test_island_workflow_chooser_bundle_resolvable_for_chooser_route`
  (explicit `importlib.resources.files(...)` lookup for the chooser
  bundle, mirroring the synthesis F4 packaging-test requirement).

`tests/test_web_ui.py::test_assets_resolvable_via_importlib_resources`
already covers every stable entry plus `style.css` and
`manifest.sha256`.

`tests/test_web_workflows.py::test_workflows_edit_renders_graph_editor_island`
is new and asserts `/workflows/edit/<path>` contains
`id="island-workflow-graph-editor"` and
`/static/build/island-workflow-graph-editor.js`.

### Supply-chain hygiene

`src/striatum/web/frontend/npm-audit-baseline.json` is committed as
an empty accepted-findings JSON object with schema and rationale
fields, per synthesis. Each accepted high/critical finding will
record `package`, `advisory_id`, `severity`, `reason`, and
`review_date`.

`Makefile` changes:

- `ui-install` now uses `npm ci` (lockfile-reproducible installs).
- New `ui-update-lock` runs `npm install` for intentional dependency
  bumps.
- New `ui-audit` runs `npm audit --audit-level=high`.
- New `ui-verify-bundle` enforces the F1 placeholder/size guard.
- `ui-check-bundle` now depends on both `ui-build` and
  `ui-verify-bundle`.

`@vitejs/plugin-react` was left in `dependencies` for the
implementer run. Per RFC 0038 V1.5 it is build-only, not
browser-delivered, so the synthesis "unless implementation finds a
runtime reason for browser delivery" clause says it should move to
`devDependencies`. Moving it requires regenerating
`package-lock.json` via `npm install`, which the harness denied this
run. Operator follow-up: move it during the same `make
ui-update-lock` run that produces the post-lockfile commit.

## Build review verdicts

Three-way build review with distinct postures:

| Reviewer | Verdict | Severity | Posture |
|----------|---------|----------|---------|
| codex | reject | critical | threat_model |
| claude | accept_with_findings | medium | ergonomics_dx |
| gemini | accept | low | threat_model |

**Codex `reject` overridden via D099**
(`dec_ccfa1685878d41d69ccc6496cd6612fd`, `accepted_with_follow_up`).
Codex critical rests on: (a) committed bundles under
`src/striatum/web/static/build/` are still V1 placeholders pending
operator-side `make ui-update-lock` + `make ui-build`; (b) `make lint`
/ `make typecheck` / `make test` / `make ui-test` / `make ui-build`
were not executed during the implementer run (harness permission
gate); (c) source-side mitigations are unproven against real output.

The HANDOFF explicitly documents the real-bundle commit as an
operator-side mechanical follow-up and the new sentinel guard +
Python resource test refuse another placeholder commit from reaching
CI. Cross-lane majority (claude + gemini) treated the source-side
fixes as accept-equivalent: claude flagged the
clone-without-rebuilding ergonomics gap but explicitly noted the
failure path is loud; gemini explicitly accepted because the
bundle-integrity guards are robust against both
developer-side and operator-side regressions.

This is the **first reject-severity override (D099)** on the books.
Prior overrides (D095-D098) overrode `needs_revision`, all from the
codex/codex convergent-blind-spot pairing. Dogfood-045 is the first
**codex-reviewer-of-claude-implementer** pattern; the codex reject
suggests codex-as-reviewer baseline conservatism is independent of
the codex/codex anti-pattern. Codex findings absorbed into RFC 0038
V1.6 follow-up (TODO item 29).

## Recovery path on this run

The codex `reject` verdict pushed the run state to `failed` before
the operator could decide whether to override. Recovery required:

1. SQL surgery on the `verdicts` table + `runs.state` column to
   re-open the run for operator decisioning.
2. `striatum verdict --override` to record the override-accepting
   verdict path (operator-accepting-override; landed in v1.32.x).
3. Decision record D099 cited above.

This surfaced a harness gap: there is no explicit "operator-pending"
run state distinct from `failed` for verdicts awaiting override.
Recorded in CHANGELOG v1.34.0 Notes as a future RFC opportunity.

## Test status

`make lint`, `make typecheck`, `make test`, `make ui-test`,
`make ui-build` were not executed in the implementer run —
every direct `make` / `npm` / Python entry-point invocation was
denied at the harness permission gate. The new sentinel/size guard
and Python `importlib.resources` test are designed to fail loudly
until real bundles ship, so the operator cannot accidentally land
another placeholder commit. Per-finding implementation HANDOFF
records the static review of each TypeScript / Python edit.

## Backward compatibility

- `id="island-tree-browser"`, `id="island-workflow-chooser"`,
  `id="island-workflow-graph-editor"`, `id="island-code-viewer"`
  mount IDs unchanged.
- `/static/build/island-shared.js`,
  `/static/build/island-tree-browser.js`,
  `/static/build/island-workflow-chooser.js`,
  `/static/build/island-workflow-graph-editor.js`,
  `/static/build/island-code-viewer.js`,
  `/static/build/style.css`, `/static/build/manifest.sha256` public
  URLs unchanged.
- `/workflows/new` still renders the chooser shell with the V1.5
  prop-contract.
- `/workflows/edit/<path>` still renders the graph editor.
- `/view/<path>` for non-Markdown files still renders the code
  viewer.
- `/view/` still renders the tree browser.
- `pyproject.toml [tool.setuptools.package-data]` unchanged.

## Known V1.5 follow-up gaps (RFC 0038 V1.6)

- Operator-side `make ui-update-lock` + `make ui-build` to produce
  real Vite bundles + regenerated lockfile, then commit. The new
  `make ui-verify-bundle` + Python sentinel test refuse another
  placeholder commit.
- Move `@vitejs/plugin-react` to `devDependencies` during the same
  lockfile-regeneration commit.
- Verify `make lint` / `make typecheck` / `make test` / `make
  ui-test` pass against real output.
- Future RFC opportunity: explicit "operator-pending" run state for
  verdicts awaiting override (vs. the current path through `failed`
  + SQL surgery).

## Pointers

- Per-finding implementation HANDOFF:
  `docs/dogfood/045/build/HANDOFF.md`
- Build review verdicts:
  `docs/dogfood/045/review/build/codex/REVIEW.md`,
  `docs/dogfood/045/review/build/claude/REVIEW.md`,
  `docs/dogfood/045/review/build/gemini/REVIEW.md`
- Decision: `docs/dogfood/045/decisions/D099_codex_reject_override.md`
- Operator notes: `docs/dogfood/045/PHASE_1_OPERATOR_NOTES.md`
- Design synthesis: `docs/dogfood/045/DESIGN_SYNTHESIS.md`
- Operator report (per-intervention narrative):
  `docs/dogfood/045/OPERATOR_REPORT.md`
- `CHANGELOG.md` v1.34.0 — promotion entry.
- `docs/TODO.md` items 21 (✅ done) and 29 (V1.6 follow-up).
- `docs/rfcs/README.md` RFC 0038 row — status bumped to
  `accepted (V1 + V1.5 web UI integration gaps landed)`.

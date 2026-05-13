---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0038", "v1-5", "build"]
---

author: reviewer-unknown-model-002

# Build Review — RFC 0038 V1.5 (claude / ergonomics_dx)

**Verdict:** `accept_with_findings` — severity `medium`.

The V1.5 fixes for F1–F4 plus supply-chain hygiene are architecturally
correct, the prop contract for `/workflows/new` is now honest, and the
guard rails (sentinel guard, no-mount regression, fetch-contract
regression) will refuse to ship a regression. The blocker that prevents
me from accepting outright is that the operator who clones this branch
and opens `/workflows/new` without first rebuilding the UI gets a page
that never hydrates — the bundles in `src/striatum/web/static/build/`
are still the V1 64-byte `console.info(...)` placeholders. The handoff
acknowledges this candidly and the failure path is loud, so the
discoverability cost is bounded; it is not zero.

## Scope of this review

Posture: `ergonomics_dx`. First-time-developer perspective on the
`/workflows/new` flow end-to-end after the V1.5 prop-contract fix,
including: can I `make ui-build` and see real bundles mount; is the
double-mount actually fixed; is the error surface discoverable.

Inputs read:

- `docs/dogfood/045/build/HANDOFF.md`
- `docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md`
- `docs/dogfood/041/OPERATOR_REPORT.md`
- The implementation files cited below.

I did not consult prior reviewer artifacts or other dogfood ledgers.

## What works

### F1 — Placeholder plugin removal (architecturally correct)

`src/striatum/web/frontend/vite.config.ts:1-36` now imports just
`react`, `defineConfig`, and `node:path`. There is no `placeholderIslandPlugin`,
no `existsSync` import, and no `Plugin` import. `plugins` is `[react()]`
(line 17). `manifest` is `false` (line 21). The Rollup `input` table
(lines 8-14) maps `island-shared` to the new non-mounting entry —
*not* to `src/main.ts`, which closes the F3 hole at the build-config
layer rather than at runtime.

The new `make ui-verify-bundle` target at `Makefile:58-64` enforces
two things at once: (a) the V1 sentinel string `Striatum frontend
island placeholder loaded` must not appear in any stable island entry
or in any `island-shared-*.js` chunk, and (b) every stable island
entry must be ≥1024 B unless a sibling shared chunk legitimately
holds the common code. `make ui-check-bundle` chains
`ui-build → ui-verify-bundle → git diff --exit-code` (line 66-67), so
build drift and placeholder sneak-throughs both surface in the same
`make check` gate.

The Python-side guard `tests/test_web_ui.py:376-385`
(`test_island_bundles_have_no_placeholder_sentinel`) reads each
stable island entry through `importlib.resources` and asserts the
sentinel is absent. This catches the placeholder slipping through
`pip install` on a clean wheel — which the V1 path missed.

### F2 — Chooser prop contract (correct end-to-end)

The server returns the literal envelope
`{"ok": true, "data": {"templates": list_templates(kind=kind)}}` at
`src/striatum/service.py:707-714::_handle_workflow_templates` — that
endpoint is unchanged from V1, so the fix is entirely on the client
side.

`src/striatum/web/frontend/src/shared/types.ts:85-98` introduces
`WorkflowTemplate` and `WorkflowTemplateListResponse` mirroring the
fields the catalog actually emits (`template_id`, `kind`,
`display_name`, `summary`, `recommended_for`, `default_lane_sets`,
`required_options`, `graph_preview`). The phantom V1 types
(`WorkflowShape`, `WorkflowLaneSet`, `WorkflowTemplateCatalog`,
`catalog.modifiers`) are gone.

`src/striatum/web/frontend/src/shared/api-client.ts:56-64`
(`fetchWorkflowTemplates`) now declares the matching return type
`ApiResult<WorkflowTemplateListResponse>`.

`src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx:111-117`
reads `res.data.templates` directly. Lines 121-128 partition by `kind`
into `shapes` / `laneSets`. Lines 248-252 pre-fill `lane_set` from the
first overlapping `default_lane_sets` entry that actually exists in
the catalog (sensible default that survives shapes whose recommended
lane sets are not in the partition — the empty-`default_lane_sets`
case falls through to the unfiltered list at line 138). The wizard is
now four steps (Template → Details → Preview → Save) per the
`STEP_LABELS` constant at line 60; the V1 Modifier step is gone since
`catalog.modifiers` was never returned by the server.

The Jinja2 template `src/striatum/web/templates/workflow_new.html:12`
emits exactly `{"allowMutations": ..., "templatesUrl":
"/workflow-templates", "previewUrl": "/workflows/generate/preview",
"generateUrl": "/workflows/generate"}` as `data-props`. The shape
matches `WorkflowChooserProps` at `types.ts:113-125` field-for-field.
The dispatch lives at `service.py:584-586` and renders with
`allow_mutations` (`service.py:1149-1156`).

A new fetch-contract regression at
`src/striatum/web/frontend/src/__tests__/workflow-chooser-fetch.test.tsx:84-127`
mounts the chooser against the literal `{ok:true, data:{templates:[...]}}`
envelope and asserts shape cards render plus the empty-state branch
fires when `templates` is missing. This is the test that would have
caught the V1 break.

### F3 — Double-mount fix (verified by regression test)

`src/striatum/web/frontend/src/shared/island-shared-entry.ts` is
twelve lines, the entire body is `import "./theme.css"; export {};`
with a docstring explaining why the file MUST NOT mount. The fact
that `vite.config.ts:9` now points `island-shared` at this file means
production never bundles `src/main.ts` (the dev-shell that mounts
every island) into the shared chunk.

The regression at
`src/striatum/web/frontend/src/__tests__/island-shared-no-mount.test.ts:69-81`
mocks `react-dom/client.createRoot`, imports both the shared entry
and the chooser entry into a JSDOM page that exposes only
`#island-workflow-chooser`, and asserts createRoot is called exactly
once for that container. A regression to the V1 behaviour (where
`island-shared` re-imported every island entry via `src/main.ts`)
would fail this test.

### F4 — Output and package data (URLs unchanged)

Output directory remains `src/striatum/web/static/build/` and the five
served URLs (`/static/build/island-shared.js`, plus the four stable
island entries, plus `style.css` and `manifest.sha256`) are unchanged.
Backward compatibility is pinned by
`tests/test_web_workflows.py:193-203::test_workflows_new_renders_chooser_island`
(asserts `/workflows/new` body contains `id="island-workflow-chooser"`
and `/static/build/island-workflow-chooser.js`) and the newly added
`tests/test_web_workflows.py:206-223::test_workflows_edit_renders_graph_editor_island`
(same shape for `/workflows/edit/<path>`). The packaging-resource
lookup is pinned by
`tests/test_web_ui.py:388-397::test_island_workflow_chooser_bundle_resolvable_for_chooser_route`,
which mirrors what `service._serve_static_file` does in production.

### Supply-chain hygiene (committed baseline + audit verb)

`src/striatum/web/frontend/npm-audit-baseline.json` is a real JSON
artifact with `schema_version`, a written `rationale`, and an empty
`accepted_findings` array — exactly the shape future findings need to
land into. `Makefile:32-39` introduces `ui-install` (uses `npm ci` for
lockfile-reproducible installs), `ui-update-lock` (intentional
`npm install`), and `ui-audit` (`npm audit --audit-level=high`). The
verb names are descriptive enough that a first-time developer reading
`make help`-equivalent output understands the install/update split.

## Findings

### F-CLAUDE-1 — Bundles still placeholders on this branch (severity: medium)

`src/striatum/web/static/build/island-workflow-chooser.js` is 69
bytes:

```
console.info("Striatum workflow-chooser island placeholder loaded");
```

`island-shared.js` (68 B), `island-tree-browser.js` (65 B),
`island-workflow-graph-editor.js` (74 B), and `island-code-viewer.js`
(64 B) are the same V1 placeholder set. `style.css` is 44 B.

What this means in practice for the ergonomics-DX angle: a first-time
developer who clones this branch, runs `make install`, runs
`striatum --web ...`, and opens `/workflows/new`, gets a Jinja2 shell
that loads `/static/build/island-workflow-chooser.js` (verified by
`test_workflows_new_renders_chooser_island`), but the loaded module
is a `console.info(...)` stub. The mount target div renders empty;
the chooser UI does not appear; the only signal is one
console-info line.

The HANDOFF acknowledges this candidly (`HANDOFF.md` "Bundle hashes"
section, lines 233-255) and explains the harness denied every
`npm install` / `make ui-build` invocation in the implementer's run.
The implementer chose the right structural fix: `make ui-verify-bundle`
and `test_island_bundles_have_no_placeholder_sentinel` will both fail
loudly until real bundles ship, so the placeholder commit cannot
escape `make check`. The discoverability path is also explicit — the
sentinel test message says "rebuild with `make ui-build` (RFC 0038
V1.5 F1)" — and the failing assertion points the operator at the
right verb.

This is the reason the verdict is `accept_with_findings` rather than
`accept`. The fix-mechanism is correct; the bundles themselves are
not. Operator follow-up to land before merging:

1. `make ui-update-lock` (regenerate `package-lock.json` from
   `package.json` — see F-CLAUDE-2).
2. `make ui-build` (emit real Vite bundles).
3. Commit the regenerated lockfile + bundle bytes + new
   `manifest.sha256`.
4. `make ui-test`, `make ui-check-bundle`, `make test` to confirm the
   F1/F2/F3 regressions pass against real output.

Cite: `src/striatum/web/static/build/island-workflow-chooser.js` (and
the four sibling stubs); `docs/dogfood/045/build/HANDOFF.md` Bundle
hashes section + Deviation: real-bundle commit section.

### F-CLAUDE-2 — `package-lock.json` is also a stub (severity: low)

`src/striatum/web/frontend/package-lock.json` is 660 bytes — a
lockfileVersion-3 skeleton with the top-level package entry but no
resolved transitive dependency tree. `make ui-install` (`npm ci`)
will refuse to run against this lockfile because `npm ci` requires a
fully resolved lock. The first command the operator must run is
therefore `make ui-update-lock` (which calls `npm install` and
regenerates the lockfile), not `make ui-install`.

The HANDOFF documents this as the very first operator follow-up step
but the Makefile target ordering itself does not signal it. A
first-time developer reading `make`'s phony-target list at
`Makefile:11` sees `ui-install ui-update-lock ui-audit ui-build
ui-dev ui-test ui-bundle-hash ui-check-bundle ui-verify-bundle` and
has to read the HANDOFF to learn the right initial sequence. See
F-CLAUDE-4 below for the structural mitigation.

Cite: `src/striatum/web/frontend/package-lock.json` (660 B,
top-level entry only); `Makefile:32-36`.

### F-CLAUDE-3 — `@vitejs/plugin-react` still in `dependencies` (severity: low)

`package.json` lists `@vitejs/plugin-react` in `dependencies`, not
`devDependencies`. RFC 0038 V1.5 says it is build-only and not
browser-delivered, so the right home is `devDependencies`. Operator
follow-up the implementer documented (HANDOFF.md "Supply-chain
hygiene" section): move it to `devDependencies` in the same
`make ui-update-lock` pass that produces the post-lockfile commit.
Harmless at runtime since vite consumes it server-side; flagged
because it is a small footgun for the next dependency-tree review.

Cite: `src/striatum/web/frontend/package.json` (dependencies block).

### F-CLAUDE-4 — No bootstrap alias / onboarding sequence in the Makefile (severity: low)

The five-step bootstrap path
(`ui-update-lock → ui-install → ui-build → ui-verify-bundle →
ui-test`) is documented in the HANDOFF but is not a single Makefile
target. `make check` chains `ui-check-bundle ui-test` for the CI
gate but not the bootstrap path. The discoverability cost is real
for the first-time UI developer: they have to read the HANDOFF or
the operator report to learn which targets to run, and in which
order, before `/workflows/new` will render.

A `make ui-bootstrap` alias chaining the right verbs (or a
`docs/FRONTEND_DEVELOPMENT.md` quickstart, which RFC 0038 §6 calls
for but is not in scope of this packet) would close the gap. Not a
ship-blocker; flagged for V1.6 or the operator follow-up commit.

Cite: `Makefile:11` (.PHONY list); `docs/dogfood/045/build/HANDOFF.md`
Deviation: real-bundle commit section, numbered operator follow-up
list.

## Build verification spot-check

Per the prompt's "spot-check at least one" requirement: I attempted
to run `make ui-verify-bundle` from this review session. The harness
denied the `make` invocation (same denial pattern the implementer hit
across the entire build run, documented in HANDOFF.md "Deviation:
real-bundle commit"). I read the target body at `Makefile:58-64`
instead and confirmed by inspection that it would exit with the
listed bad-bundle messages against the current placeholder bundles —
each of the five stable entries is well below the 1024 B floor and
two of them contain the sentinel string. The target body is correct
Python and would emit non-zero exit code on this branch.

Static review of the cited test files confirms they exercise the
right contracts:

- `workflow-chooser-fetch.test.tsx` mounts the real component against
  the literal server envelope and asserts the failure-mode error
  surfaces (no silent loading state).
- `island-shared-no-mount.test.ts` exercises both the order-dependent
  and order-independent double-mount cases.
- `workflow-chooser.test.ts` covers `buildSpec` (lane-command trim,
  workflow_id-as-name fallback, blank-branch omission, modifiers
  always `[]`) and `recommendedForText` (array vs string vs
  undefined).
- `test_island_bundles_have_no_placeholder_sentinel` and
  `test_island_workflow_chooser_bundle_resolvable_for_chooser_route`
  in `tests/test_web_ui.py` cover the wheel-side packaging path.

## Backward compatibility

Confirmed by reading the cited regressions:

- `id="island-tree-browser"`, `id="island-workflow-chooser"`,
  `id="island-workflow-graph-editor"`, `id="island-code-viewer"`
  mount IDs unchanged.
- `/static/build/island-*.js` URLs unchanged; pinned by the two
  regressions in `tests/test_web_workflows.py`.
- `/workflows/new` still renders the chooser shell with V1.5
  prop-contract; pinned by
  `test_workflows_new_renders_chooser_island`.
- `/workflows/edit/<path>` still renders the graph editor; newly
  pinned by `test_workflows_edit_renders_graph_editor_island`.

## Summary

Architecturally correct fixes for F1–F4 + supply-chain. The
`/workflows/new` prop contract now matches the server; the
double-mount is closed at both the build-config and runtime layers;
the placeholder sneak-through path is gated by sentinel checks at
both `make ui-verify-bundle` and the wheel-installed Python test.
The committed bundle bytes are still V1 placeholders (acknowledged
deviation, harness-denied rebuild), and the operator follow-up to
ship real bundles is documented and gated. Lockfile is also a stub
and must be regenerated in the same follow-up. Ergonomics-DX
verdict: `accept_with_findings`, severity `medium`.

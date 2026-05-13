---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0038", "v1-5", "design"]
---

author: reviewer-unknown-model-001

# RFC 0038 V1.5 Design Synthesis — Ergonomics-DX Review

## Lens

First-time-developer perspective: after reading `docs/dogfood/045/DESIGN_SYNTHESIS.md`,
can the reader execute `make ui-build`, watch real island bundles land in
`src/striatum/web/static/build/`, and load `/workflows/new` without going back
to the operator to disambiguate? Each finding is graded by whether the fix
names files, symbols, and shapes precisely enough to act on.

Overall: yes. The synthesis is a pinpoint plan. Every prop-contract debate
picks one side, every file is named by full repo-relative path, every Make
target and bundle URL is spelled out, and the closing "Backward Compatibility
And Regression Assertions" section gives the executor a concrete test list to
write against. A first-time developer can land V1.5 without back-channel
questions on the core five findings.

The notes below are low-severity polish items that affect discoverability at
the margins, not blockers.

## What works for a first-time developer

- F1 (synthesis §"F1 Placeholder Removal") spells out the exact deletion in
  `src/striatum/web/frontend/vite.config.ts` — the `existsSync` import, the
  `type Plugin` import, the entire `placeholderIslandPlugin()` function, and
  the `plugins: [placeholderIslandPlugin(), react()]` → `plugins: [react()]`
  rewrite. The surviving Rollup `islandEntries` map is given in full, so the
  reader knows exactly what `make ui-build` should emit. The
  `make ui-check-bundle` placeholder-sentinel check ("Striatum frontend
  island placeholder loaded") gives CI a positive guard that the deletion
  was complete. This is the clearest of the five findings.
- F2 (synthesis §"F2 Chooser Prop Contract") commits to the
  rewrite-the-component side, names the server endpoint by both file and
  function (`src/striatum/service.py::_handle_workflow_templates`), names
  the chooser island
  (`src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`),
  names the shared type module (`src/shared/types.ts`) and the api client
  (`src/shared/api-client.ts`), and cites the catalog source
  (`src/striatum/workflow_generator/catalog.py::list_templates`) so the
  reader can verify the row shape (`template_id`, `kind`, `display_name`,
  `summary`, `recommended_for`, `default_lane_sets`). The "remove dependency
  on nonexistent `catalog.shapes`, `catalog.lane_sets`, `catalog.modifiers`"
  line explicitly names what to delete from the component, which is exactly
  what the codex attempt-2 review (per dogfood-041 OPERATOR_REPORT §9 F2)
  identified as the unaddressed integration gap.
- F3 (synthesis §"F3 Double-Mount Fix") picks the non-mounting-entry
  approach and gives the literal file body:

  ```ts
  import "./theme.css";
  export {};
  ```

  Reader knows the path
  (`src/striatum/web/frontend/src/shared/island-shared-entry.ts`), what to
  put in it, and that the per-island entries continue to import their own
  dependencies from `../../shared/...` rather than via the shared bundle as
  a side-effecting trampoline. The regression-test ask
  ("`createRoot()` is called once for `#island-workflow-chooser`") gives an
  executor a literal assertion to write.
- F4 (synthesis §"F4 Output And Package Data Layout") names the served
  URLs, the package-data globs in `pyproject.toml
  [tool.setuptools.package-data]`, and the Python serving path
  (`src/striatum/service.py` resolving from `striatum.web.static`), plus a
  `importlib.resources.files(...)` packaging test. The dual `pyproject.toml`
  keys (`"striatum.web.static"` and `"striatum.web.static.build"`) cover
  both the directory walk and the explicit sub-package shape.
- Supply-chain hygiene (synthesis §"Supply-Chain Hygiene") commits to
  `npm ci --prefix src/striatum/web/frontend`, names the lockfile path
  (`src/striatum/web/frontend/package-lock.json`), names the audit baseline
  path (`src/striatum/web/frontend/npm-audit-baseline.json`), and gives the
  exact Makefile body for `make ui-audit`. The cadence trigger ("any
  `package.json` or `package-lock.json` change, plus every RFC that adds a
  new browser dependency") is a discoverable rule, not a vibe.
- Backward compatibility (synthesis §"Backward Compatibility And Regression
  Assertions") lists the four mount IDs, the four bundle URLs, and the
  `/workflows/new` rendering invariant by name. The five regression
  assertions at the end of the section form a concrete acceptance checklist
  for the executor.

## Findings

### Finding 1 — F4 omits `MANIFEST.in` even though the review prompt names it as a packaging surface (low)

The review prompt's F4 check is literal: "names the exact
`package_data`/`MANIFEST.in`/`pyproject.toml` surface and the Python serving
path." The synthesis covers `pyproject.toml` glob entries and the
`src/striatum/service.py` serving path, but does not mention `MANIFEST.in`
at all. A first-time developer producing both a wheel and an sdist will not
know whether `MANIFEST.in` is intentionally out of scope (e.g. the project
relies on `setuptools-scm` + `pyproject.toml`-only inclusion and never ships
sdists with bundled assets), or whether it was simply forgotten.

Citation: synthesis §"F4 Output And Package Data Layout" — package-data
section names only `pyproject.toml [tool.setuptools.package-data]`.

Suggested concrete fix in V1.5 implementation: have the executor add one
sentence to the F4 step in the implementer prompt declaring either
"`MANIFEST.in` is not used; sdist inclusion of `src/striatum/web/static/build/`
is covered by `pyproject.toml` package-data" or "also extend `MANIFEST.in`
with `recursive-include src/striatum/web/static/build *.js *.css *.sha256`".
The packaging test should additionally assert that a built sdist contains
the bundle files, not only the wheel.

### Finding 2 — F4 leaves the `vite.config.ts` manifest flag as an implementation choice (low)

F4 reads: "Alternatively, set `manifest: false` in `vite.config.ts` and omit
the `.vite/*.json` globs. Choose one in implementation; because the Python
templates use stable hardcoded paths and do not consume Vite's manifest, the
simpler implementation is `manifest: false` plus `manifest.sha256` as the
only committed manifest." The synthesis does recommend the
`manifest: false` path, but explicitly delegates the final choice to the
implementer.

For an ergonomics-DX synthesis whose stated job is to pick one side of each
debate, a first-time implementer reading this will reasonably wonder whether
to expect a `.vite/manifest.json` in the committed build output or not, and
whether `make ui-check-bundle` should diff against that file. Two
implementers reading the synthesis can ship two different layouts.

Citation: synthesis §"F4 Output And Package Data Layout", second paragraph
of the package-data block ("Alternatively, …Choose one in implementation").

Suggested concrete fix: rewrite the paragraph to commit to `manifest: false`
plus `manifest.sha256` as THE V1.5 layout, and drop the `build/.vite/*.json`
glob from the recommended `pyproject.toml` entries above it so the two
sub-sections are consistent.

### Finding 3 — F1 sanity-threshold is named as an example, not a value (low)

F1 reads: "`make ui-check-bundle` must fail if any stable island entry
contains the sentinel string `Striatum frontend island placeholder loaded`
or if any stable island JavaScript file is below a small sanity threshold
such as 1 KiB." The sentinel-string check is concrete; the size check is
"such as 1 KiB". A first-time implementer must choose a number, and a
1 KiB threshold may or may not be tight enough — Rollup can emit
near-empty entries that import everything from `island-shared-[hash].js`,
which would defeat the size guard without tripping the sentinel guard.

Citation: synthesis §"F1 Placeholder Removal", final paragraph.

Suggested concrete fix: pick the value (e.g. "below 1024 bytes after
gzip-stripped output") and call out explicitly that the check applies to
every entry in `islandEntries` plus any `island-shared-[hash].js` chunk, so
an implementer cannot satisfy CI by importing everything from a chunk that
itself is under threshold.

### Finding 4 — F3 base.html guard is phrased as a conditional, not a build invariant (low)

F3 reads: "Keep `src/striatum/web/templates/base.html` loading
`/static/build/island-shared.js` only if that file is the non-mounting
shared entry above. Do not load `src/main.ts` output in production." The
intent is "ensure the build pipeline routes the `island-shared.js` URL to
the non-mounting entry, never to the `main.ts` output," which is a
build-time invariant. As written, "only if" reads as a runtime conditional
that a first-time implementer might try to express in Jinja2 logic.

Citation: synthesis §"F3 Double-Mount Fix", penultimate paragraph.

Suggested concrete fix: rewrite as "`src/striatum/web/templates/base.html`
continues to load `/static/build/island-shared.js` unchanged. The
build-time guarantee is that this URL is produced from
`src/shared/island-shared-entry.ts` (via the `island-shared` Rollup input
declared in F1), never from `src/main.ts`. Do not emit `src/main.ts` to
`src/striatum/web/static/build/` in production." That removes the
conditional reading and ties the guarantee back to F1's `islandEntries`
map.

## Verdict

`accept` with severity `low`. The synthesis is concrete, files and symbols
are named at pinpoint resolution on every finding, the chosen plan
explicitly picks one side of each V1 debate (rewrite-component over
reshape-server; non-mounting-shared-entry over guarded-main.ts), and the
regression-assertion section gives an executor a literal test checklist.
A first-time developer can land V1.5 from this document without operator
back-channel for the core five findings. The four notes above are
polish-level — they refine F4 packaging completeness, F4 manifest decisiveness,
F1 threshold concreteness, and F3 phrasing — and should be folded into the
implementer prompt rather than blocking acceptance of the synthesis.

# Gemini Design Prompt

Produce `docs/dogfood/041/design/gemini/DESIGN.md`.

Design an implementation plan for RFC 0038 with attention to cross-platform reality, npm supply-chain hygiene, build determinism, browser support matrix, bundle bloat, and accessibility regressions.

**Cross-platform reality (Linux + macOS only; Windows daemon stays deferred):**

- Vite builds on Linux + macOS identically with node v22+. No platform-specific build steps.
- `make ui-install` / `make ui-build` / `make ui-dev` / `make ui-test` work on both platforms.
- Bundled output is deterministic across platforms when node + npm versions match.

**npm supply-chain hygiene:**

- Lockfile (`package-lock.json`) committed and is the source of truth for dependency versions.
- Package set narrow: react, react-dom, react-flow, shiki, typescript, vite, vitest, @types/*. Refuse any dependency that pulls in dozens of transitive deps unnecessarily.
- `npm audit --omit=dev` baseline in CI; high-severity vulns block the build.
- Pin strategy: caret ranges for direct deps (allow minor/patch), exact for transitives via lockfile.
- No CDN scripts.
- No telemetry packages.

**Build determinism (CI bundle hash check):**

- CI runs `make ui-build` against the committed `package-lock.json` + the source tree.
- Compute sha256 of each emitted bundle file under `src/striatum/web/static/build/`.
- Compare against the committed hashes (stored where? — design choice; recommend a `bundles.lock.json` or sha256 sidecar files committed alongside).
- Mismatch fails CI with clear message: "rerun `make ui-build` and commit the updated bundles + lock".

**Wheel package-data semantics:**

- `pyproject.toml` `[tool.setuptools.package-data]` includes `"striatum.web.static.build" = ["*.js", "*.css"]`.
- The wheel ships the bundled output as a Python package resource.
- Operator install path: `pip install striatum-orchestrator` → wheel includes bundled UI assets.

**Browser support matrix:**

- Modern browsers only. ES2022 target. `<dialog>` native (no polyfill).
- No IE / no legacy Edge / no Safari < 15.
- TypeScript strict mode catches API-shape mismatches.

**Bundle bloat:**

- Per-island bundle size budget: react-flow island ~ 200 KB compressed (real cost of the graph editor); shiki island ~ 100 KB compressed with 8 grammars vendored; tree-browser + chooser islands ~ 30-50 KB each compressed.
- Shared chunk extracts react + react-dom + common helpers.
- Tree-shaking enabled via Vite defaults.

**Accessibility regressions:**

- Existing UI keyboard-shortcut overlay (RFC 0037 V1) must continue to work.
- New island components must not introduce focus traps that block Tab navigation.
- ARIA labels on all new interactive widgets.
- Skip-link from `base.html` (RFC 0037 V1) continues to work.
- Color contrast: any new colors meet WCAG AA against both light and dark palettes.

**Adversarial cases (each becomes a unit or integration test):**

- Malicious npm package added — `npm audit` catches in CI.
- Build-output drift between CI and contributor laptop — bundle hash check fails.
- `npm install` offline — `make ui-install` fails cleanly with named remediation.
- `node` missing — `make ui-install` fails cleanly with named remediation.
- Bundle bloat creeps over per-island budget — manual review at PR time (no automated budget yet for V1).
- Accessibility regression: keyboard nav blocked on a new island — manual checklist in BUILD_HANDOFF.
- Shiki language detection wrong for an obscure file — fallback to plain `<pre>`.
- Tree browser asks for a path containing `..` — server-side `GET /v1/repo/tree` refuses with `path_traversal`.

**Test strategy:**

- Existing UI snapshot tests pass.
- New JS unit tests (Vitest) for: tree-browser navigation logic, chooser-wizard step transitions, graph-editor JSON serialization, code-viewer language detection.
- New integration tests for `GET /v1/repo/tree` route (Python-side, in `tests/test_service.py`).
- Manual checklist for things that can't be automated: visual island rendering across light/dark mode, keyboard nav completeness, bundle size budget enforcement.

**State what cannot be claimed:**

- Mobile-first responsive overhaul — separate RFC.
- Hosted-mode UX — D083 single-user single-machine.
- Real-time collaboration — out of scope.
- Cross-machine repo browsing — out of scope.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:` exactly. Correct: `author: designer-gemini-pro-001`.

The `handoff` kind (DESIGN.md) does NOT need YAML front matter. Just the plain `author:` byline. Do NOT add finding-style front matter.

**IMPORTANT — produce the artifact in this single supervised invocation.** Do not surface strategy and ask the operator to proceed. The supervised wrapper has no follow-up turn.

Do not call striatum CLI; the operator publishes on your behalf otherwise.

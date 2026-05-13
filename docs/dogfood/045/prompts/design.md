# Design Prompt: RFC 0038 V1.5 (web UI integration gaps)

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/045/design/<lane>/`).

Design **RFC 0038 V1.5 acceptance criteria** addressing the 4 codex findings F1-F4 surfaced in dogfood-041 plus the gemini supply-chain hygiene findings. Read `docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md` and `docs/dogfood/041/OPERATOR_REPORT.md` (findings section) first. Cite `src/striatum/web/frontend/vite.config.ts`, `src/striatum/web/frontend/src/main.ts`, `src/striatum/web/frontend/package.json`, and the `/workflows/new` chooser island.

Cover concretely:

- **F1 — Replace `placeholderIslandPlugin` with real builds**: `src/striatum/web/frontend/vite.config.ts` ships a placeholder Rollup plugin that emits `console.info` stubs instead of compiled islands. Committed island bundles in `src/striatum/web/static/build/` must be real Vite/Rollup output. Identify the entry files, the plugin removal path, and what `make ui-build` produces post-fix (hashes change). Name the React component entries.
- **F2 — `/workflows/new` chooser prop-contract mismatch**: server returns `{templates: [...]}` but the React component expects a different shape. Find both sides (server route in `src/striatum/web/` and the chooser island under `src/striatum/web/frontend/src/islands/workflow-chooser/`) and pick which side moves. Justify in one sentence.
- **F3 — `island-shared.js` double-mount risk**: `vite.config.ts` maps the `island-shared` rollup entry to `src/main.ts`, so after a real build the shared bundle runs `main.ts` side effects AND every island that imports from it. Identify the shared-bundle entry that does not auto-mount, and how islands consume it.
- **F4 — Vite output semantics vs package-data layout**: bundles land in `../static/build/` and Python serves from `src/striatum/web/static/build/`. Identify the `package_data`/`MANIFEST.in` surface that must keep these bundle paths reachable after install, and whether the manifest file is consumed by the server template-rendering code.
- **Supply-chain hygiene**: `package-lock.json` present + checked in; `npm audit` baseline captured; dependency tree review. Identify the lockfile policy, the baseline-capture target (Makefile or doc), and how regressions surface.

Designers MUST cite existing code in `src/striatum/web/frontend/`, `src/striatum/web/`, and the served-template surface (file names + line refs where load-bearing). Hand-waving the plugin-removal wiring is grounds for design review to bounce.

**Backward compatibility (non-negotiable)**: existing islands must keep mounting; bundle paths served from `src/striatum/web/static/build/` by the Python layer must keep working without renames; the `/workflows/new` page must continue to render its chooser after the prop-contract fix.

Out of scope: new island surface beyond F1-F4 wiring, hosted services, RFC 0038 future-work items.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:`. Expected shape: `(role)-unknown-model-<NN>` where NN is the session ordinal. Do NOT prefix with the lane name.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.

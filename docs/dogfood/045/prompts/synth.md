# Synthesis Prompt: RFC 0038 V1.5 (web UI integration gaps)

Produce `docs/dogfood/045/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/045/design/codex/DESIGN.md", "docs/dogfood/045/design/claude_code/DESIGN.md", "docs/dogfood/045/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Expected shape: `(role)-unknown-model-<NN>` (session ordinal, no lane prefix).

Reconcile the 3 designs into ONE concrete plan for RFC 0038 V1.5:

- **F1 placeholder removal**: exact deletion of `placeholderIslandPlugin` in `src/striatum/web/frontend/vite.config.ts`, exact island entry mapping that survives, and the `make ui-build` invocation that produces real bundles. Name the resulting filenames or hash-pattern that lands under `src/striatum/web/static/build/`.
- **F2 chooser prop-contract**: pick one side — server emits the shape the component wants, OR component is rewritten to consume `{templates: [...]}`. Justify in one sentence. Name the server route file and the chooser island file.
- **F3 double-mount fix**: pick one — separate non-mounting `island-shared` entry, or make `main.ts` idempotent / guarded. Name the shared-bundle entry file and how islands import from it.
- **F4 output / package-data layout**: name the `package_data` glob (or `MANIFEST.in` lines, or `pyproject.toml [tool.setuptools.package-data]` section) that must include the bundle outputs, and the Python serving path that resolves them.
- **Supply-chain hygiene**: lockfile commit policy (yes/no, justified), npm-audit baseline storage location (file path), Makefile target name. Name the dependency-review cadence trigger.
- **Backward-compatibility**: explicit assertion that existing islands still mount, served bundle URLs unchanged, `/workflows/new` keeps rendering. List the regression assertions implementation must pin.

Choose; do not enumerate. Output is a SPECIFIC plan ready to implement against. If the three designs disagree on a point, pick one and justify in one sentence.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Expected shape: `(role)-unknown-model-<NN>` (do NOT prefix with lane name).

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.

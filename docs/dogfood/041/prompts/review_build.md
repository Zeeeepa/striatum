# Review Build Prompt (ergonomics_dx posture, 3-way parallel)

Shared prompt for THREE parallel build reviewers (codex + claude + gemini). Your lane determines the subdirectory: `docs/dogfood/041/review/build/{codex,claude,gemini}/REVIEW.md`.

Front matter (ALL FIVE REQUIRED — schema_version exact):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0038", "web-ui", "build"]
---
```

Review the RFC 0038 V1 implementation under **ergonomics_dx** posture. Inspect both implementer halves: toolchain (codex implemented under `src/striatum/web/frontend/{package.json,vite.config.ts,tsconfig.json}`, Makefile, CI integration, Jinja2 templates, service.py routes, bundled output under `src/striatum/web/static/build/`) and components (claude implemented React islands under `src/striatum/web/frontend/src/islands/`, JS tests, FRONTEND_DEVELOPMENT.md, HOW_TO_HUMAN updates, docs).

ergonomics_dx posture (per RFC 0018): "This is a developer-ergonomics review. Evaluate the artifact's surface from a first-time-user perspective; verdict acceptance means the affordances are discoverable and consistent."

Per-lane review angle (you are ONE of three reviewers):

- **codex review** (you, if codex): systems angle — Vite config correctness, Makefile ergonomics (clear errors when node missing, sensible defaults, `ui-install/build/dev/test` all work), wheel package-data path semantics, Jinja2 template changes correctness (Edit-button promotion visually right, island mount points present), service.py routes for `/v1/repo/tree` correct.
- **claude review** (you, if claude_code): component-ergonomics angle — React island affordance discoverability (tree browser navigation, chooser wizard step ordering, graph editor widget choices, code viewer language detection); accessibility (keyboard nav, focus management on dialogs, ARIA labels, color contrast); documentation quality (FRONTEND_DEVELOPMENT.md complete + accurate; HOW_TO_HUMAN walkthroughs specific).
- **gemini review** (you, if gemini): adversarial angle — npm supply-chain hygiene (lockfile committed, package set narrow, no unused deps, `npm audit --omit=dev` baseline); build determinism (CI bundle hash check working); cross-platform reality (builds same on Linux + macOS); browser support matrix (modern only, no polyfills); bundle bloat; accessibility regressions on existing surfaces; failure modes (offline install, missing node, malformed package.json).

Required checks (all reviewers):

- No new runtime dependencies. No Python deps added. Operator install stays pip-only.
- Bundled output under `src/striatum/web/static/build/` is committed AND shipped via `pyproject.toml` package-data.
- Jinja2 page shells stay; islands mount into specific DOM slots; NOT full SPA conversion.
- CSP unchanged. JSON API + SSE feed unchanged.
- Existing UI snapshot tests pass.
- New JS unit tests (Vitest) present for the named widgets.
- Documentation honest: FRONTEND_DEVELOPMENT.md exists; HOW_TO_HUMAN walkthroughs cover `/view/`, `/workflows/new`, graph editor.
- Cross-link integrity: RFC 0038 status block updated to `accepted (V1)`; CHANGELOG entry; RFC index updated.

Use `needs_revision` for behavior gaps, missing tests, dark-mode parity gaps, accessibility regressions, new runtime deps snuck in, or documentation overstating scope.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write the artifact and exit normally — operator publishes on your behalf. Do not ask clarifying questions and exit (dogfood-037 intervention #5). Do not surface strategy and exit (dogfood-036 / dogfood-040). The supervised wrapper has no follow-up turn.

Stay inside your lane's review write scope.

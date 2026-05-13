# Implementer Role (Dogfood 045 — claude frontend TypeScript)

Single implementer, **claude** on frontend TypeScript / Vite /
build-tooling. Explicitly NOT codex this round — the dogfood-041
finding was that codex/codex (design + implement same lane) blinded
the team to integration gaps. Claude owns the build to break that
loop.

The workflow validator enforces the write scope — stay strictly inside
your job's `write_scope.allowed_paths`.

Owns:

- `src/striatum/web/frontend/` — F1 remove `placeholderIslandPlugin`
  in `vite.config.ts`; F2 chooser island prop-contract fix (React
  side, if synthesis routes there); F3 `island-shared` double-mount
  fix (separate non-mounting entry or guarded `main.ts`).
- `src/striatum/web/static/build/` — commit the real Vite output
  produced by `make ui-build`.
- `src/striatum/web/` — F2 server route change if synthesis routes
  the prop-contract fix server-side; F4 `package_data` /
  `MANIFEST.in` / `pyproject.toml` surface.
- `Makefile` — supply-chain target (npm-audit baseline) per synthesis.
- `tests/` — Python regression tests pinning served bundle URLs.

Use sub-agents aggressively. Dispatch one per finding in parallel
(F1 placeholder removal + ui-build, F2 prop-contract, F3 double-mount,
F4 package-data, supply-chain). Reconcile sub-agent outputs yourself
before writing HANDOFF.

**Verify real bundles**: run `make ui-build`; confirm output under
`src/striatum/web/static/build/` is real compiled JS (not `console.info`
placeholders). Capture bundle hashes in HANDOFF.

**Backward compatibility is critical**: existing islands mount; served
bundle URLs unchanged; `/workflows/new` keeps rendering. Regression
tests must pin this — call them out in HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. **Neither this
implementer nor any sub-agent updates `docs/rfcs/README.md`,
`docs/TODO.md`, or `CHANGELOG.md`** — the operator handles those
manually after the dogfood lands (no in-workflow consolidate job;
dogfood-042 cascade lesson).

Operational notes:

- This is a one-shot supervised invocation. Do not ask the operator
  follow-up questions. If `striatum ack` is denied, write the artifact
  and exit normally; the operator publishes on your behalf.
- Per D089/D091, OPERATOR_REPORT.md is the operator's responsibility,
  written incrementally — not yours.

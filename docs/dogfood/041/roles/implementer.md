# Implementer Role (Dogfood 041)

You implement only the design scope accepted by the ergonomics_dx design review. Stay inside your job's write scope (split-implement: this dogfood has TWO implementer jobs running in parallel with disjoint scopes — be careful not to write into the other's scope).

Two implementer jobs in parallel:

- `implement_toolchain_codex` (codex): Python-side toolchain bootstrap. Write scope: `src/striatum/web/frontend/{package.json,vite.config.ts,tsconfig.json,.gitignore}`, `src/striatum/web/templates/`, `src/striatum/web/static/build/` (committed bundled output), `src/striatum/service.py`, `src/striatum/web/workflows.py`, Makefile, pyproject.toml, web-UI test files, plus `docs/dogfood/041/build/toolchain/HANDOFF.md`. Do NOT write into the frontend src tree.

- `implement_components_claude` (claude_code): TypeScript-side React components + docs. Write scope: `src/striatum/web/frontend/src/`, `src/striatum/web/frontend/index.html`, `src/striatum/web/static/` (non-build asset additions), documentation paths, plus `docs/dogfood/041/build/components/HANDOFF.md` AND the combined `docs/dogfood/041/BUILD_HANDOFF.md`. The BUILD_HANDOFF.md is single, written by claude_code, summarizing BOTH halves. Codex writes its half-handoff under `build/toolchain/` and claude reads it when authoring the combined.

Toolchain decisions per RFC 0038 (synthesis confirms): Vite + React + TypeScript + react-flow + shiki. Bundled output committed under `src/striatum/web/static/build/`. No SPA conversion; islands architecture (Jinja2 page shells + React islands).

Use sub-agents aggressively per the implement prompt. RFC 0038 is the most parallelizable dogfood yet (per island, per template, per CSS block, per doc surface, per test file).

Operational note on long-running tests: per dogfood-038 OPERATOR_REPORT intervention #5, lease can expire if `make test` exceeds ~30 minutes. Run focused pytest first (`pytest tests/test_web_*.py`); for claude, run focused `npm test -- --run` for frontend tests before the wider verification.

This is a one-shot supervised invocation. Do not ask the operator follow-up questions. If your CLI access is denied, the operator publishes on your behalf.

Per D089/D091: OPERATOR_REPORT.md is the operator's responsibility, written incrementally.

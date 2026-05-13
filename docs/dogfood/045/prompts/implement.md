# Implement: RFC 0038 V1.5 (claude frontend TypeScript)

Blocked until `review_design` returns an accepting verdict.

Implement RFC 0038 V1.5 per `docs/dogfood/045/DESIGN_SYNTHESIS.md`. **You write TypeScript / Vite / build-tooling, plus the supply-chain artifacts and the Python `package_data` surface that ships the bundles.** Claude is the implementer this round (frontend work; explicitly not codex, to avoid the codex/codex anti-pattern from dogfood-041).

**Your scope (claude frontend + build-tooling-side):**

- `src/striatum/web/frontend/` — F1 remove `placeholderIslandPlugin` from `vite.config.ts`; F2 chooser prop-contract fix on whichever side the synthesis picked; F3 double-mount fix on the shared-bundle entry; tests under `src/__tests__/`.
- `src/striatum/web/static/build/` — committed real island bundles produced by `make ui-build`.
- `src/striatum/web/` — F2 server-route change if synthesis routes the prop-contract fix server-side; F4 `package_data`/manifest surface as named by synthesis.
- `Makefile` — supply-chain target (npm-audit baseline) if synthesis names one.
- `tests/` — Python-side regression tests that the bundle paths still serve.
- `docs/dogfood/045/build/HANDOFF.md` — handoff summarizing shipped scope, files touched, `make ui-build` bundle hash, test results, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per finding, dispatched in parallel:

- Sub-agent F1: delete `placeholderIslandPlugin` in `vite.config.ts`, run `make ui-build`, capture the real bundle filenames + hashes, commit them under `src/striatum/web/static/build/`.
- Sub-agent F2: chooser prop-contract — change the side the synthesis picked; touch BOTH the server route and the React island only if synthesis demands it.
- Sub-agent F3: shared-bundle double-mount fix — separate non-mounting entry OR guarded `main.ts` per synthesis. Show in HANDOFF which approach was taken.
- Sub-agent F4: `package_data` / `MANIFEST.in` / `pyproject.toml` surface so installed bundles serve at the same URLs.
- Sub-agent supply-chain: lockfile commit (or policy doc), npm-audit baseline at the synthesized path, Makefile target wired.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Verify real bundles produced**: run `make ui-build` end-to-end; confirm output under `src/striatum/web/static/build/` is real Vite output (not `console.info` placeholders). Capture bundle hashes in HANDOFF.

**Backward compatibility (critical)**: existing islands must keep mounting; served bundle URLs unchanged; `/workflows/new` keeps rendering its chooser after prop-contract fix. Regression tests must pin this — call them out in HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. **No README / TODO / CHANGELOG / RFC index updates** — the operator handles those manually after the dogfood lands (no in-workflow consolidate job; dogfood-042 cascade lesson).

Verification: `make lint`, `make typecheck`, `make test`, `make ui-build` all pass. The shipped bundles are real Vite output.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.

## Byline discipline

HANDOFF byline is a plain markdown line. Expected shape `(role)-unknown-model-<NN>` (session ordinal, no lane prefix). Lowercase `author:`, no decoration.

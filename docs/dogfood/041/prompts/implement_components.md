# Implement Components Prompt (claude_code lane)

Blocked until `review_design_ergonomics` returns an accepting verdict.

After the gate opens, implement the **TypeScript-side React component half** of accepted RFC 0038 V1. The Python-side toolchain half is implemented in parallel by codex (`implement_toolchain_codex`); their write scope is disjoint from yours.

**Your scope (claude_code, TypeScript + docs):**

- `src/striatum/web/frontend/src/islands/tree-browser/` — React component with lazy directory expansion via `GET /v1/repo/tree`, click-to-navigate, breadcrumb, fuzzy search, keyboard nav, empty states.
- `src/striatum/web/frontend/src/islands/workflow-chooser/` — Step-by-step wizard calling `GET /workflow-templates`, `POST /workflows/generate/preview`, `POST /workflows/generate`. Operator confirmation gate at the write step.
- `src/striatum/web/frontend/src/islands/workflow-graph-editor/` — react-flow-based drag-drop editor. Per-node side panel with structured widgets (dropdowns, radio buttons, multi-select). Save via existing workflow-edit POST endpoint.
- `src/striatum/web/frontend/src/islands/code-viewer/` — shiki-based syntax highlighter. Bundle 8 grammars. Line numbers, copy-to-clipboard, raw-link, dark-mode parity.
- `src/striatum/web/frontend/src/shared/` — `api-client.ts`, `types.ts`, theme helpers, common utilities.
- `src/striatum/web/frontend/src/main.ts` — entry points; `createRoot()` calls into named DOM slots.
- `src/striatum/web/frontend/src/__tests__/` — Vitest unit tests for duration-formatter equivalents, filter predicates, language detection, tree node serialization.
- `src/striatum/web/frontend/index.html` — Vite dev shell (NOT shipped; codex's static/build bundles are what ships).
- Documentation:
  - New `docs/FRONTEND_DEVELOPMENT.md` — contributor-side guide.
  - `docs/HOW_TO_HUMAN.md` — walkthroughs for `/view/`, `/workflows/new`, the graph editor.
  - `docs/HOW_TO_AGENT.md` — AI-session guidance updates if any.
  - `docs/UBIQUITOUS_LANGUAGE.md` — new entries.
  - `docs/CLI_REFERENCE.md` — cross-reference the web routes (no new CLI verbs).
  - `docs/SPEC.md` — frontend section update if applicable.
  - `docs/TODO.md` — mark RFC 0038 V1 as done.
  - `docs/DECISION_LOG.md` — record any decision-log entries the implementation surfaces.
  - `docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md` — status update to `accepted (V1)`.
  - `docs/rfcs/README.md` — RFC index status update.
  - `README.md` — cross-reference if applicable.
  - `CHANGELOG.md` — Added/Decided entries (D092 already landed).
- `docs/dogfood/041/build/components/HANDOFF.md` summarizing your shipped scope.
- **`docs/dogfood/041/BUILD_HANDOFF.md`** — combined handoff covering BOTH halves. Read `docs/dogfood/041/build/toolchain/HANDOFF.md` (codex's, written in parallel) when authoring this.

**Do NOT write into the toolchain scope** (those are owned by codex):

- `src/striatum/web/frontend/package.json`, `vite.config.ts`, `tsconfig.json`, `.gitignore`
- `src/striatum/web/templates/` (Jinja2 templates — codex)
- `src/striatum/web/static/build/` (committed bundled output — codex's `make ui-build`)
- `src/striatum/service.py`, `src/striatum/web/workflows.py`
- `tests/test_web_*.py`, `tests/test_service.py` (Python tests — codex)
- `Makefile`, `pyproject.toml`

The disjoint scope is enforced by the workflow validator.

## Maximize sub-agent usage

This is the most parallelizable dogfood yet. Spawn sub-agents in parallel for:

- One sub-agent per React island (tree-browser, workflow-chooser, workflow-graph-editor, code-viewer).
- One sub-agent for `shared/api-client.ts` + `shared/types.ts`.
- One sub-agent per Vitest test file.
- One sub-agent per documentation surface (FRONTEND_DEVELOPMENT.md, HOW_TO_HUMAN.md, UBIQUITOUS_LANGUAGE.md, RFC 0038 status update, CHANGELOG.md).
- Exploratory sub-agents to read the existing chat-tool registry pattern (RFC 0023 V1.5), the existing SVG graph renderer (RFC 0022 V1), and the workflow-edit POST endpoint shape.

**Operational note**: `make ui-test` (Vitest) is your focused test command. Run it before `make install/lint/typecheck/test/smoke` to catch component-side issues fast. Lease can expire if make test exceeds ~30 minutes; prefer the focused-test path.

If `striatum ack` is denied, write artifacts and exit normally; the operator publishes on your behalf. Per dogfood-037 intervention #5: do not ask the operator a clarifying question and exit.

## Handoff

Produce `docs/dogfood/041/build/components/HANDOFF.md` AND `docs/dogfood/041/BUILD_HANDOFF.md`. The latter combines both halves; read codex's `docs/dogfood/041/build/toolchain/HANDOFF.md` when authoring.

Byline: `author: implementer-claude-opus-001`. Plain markdown line, lowercase, no decoration.

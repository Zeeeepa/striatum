# Implement Prompt

Implementation is blocked until `review_design_threat` returns an accepting verdict. Do not start implementation from RFC 0036 alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/038/DESIGN_SYNTHESIS.md` and the resolved threat-model review findings. Stay inside the workflow write scope.

Expected behavior:

**RFC 0036 V1 slice:**

- `striatum-mcp` skill body in claude_code + generic templates with the section ordering accepted by the synthesis
- `CLAUDE_CODE_SKILLS` tuple in `src/striatum/skills/install.py` extended with `"mcp"`
- Generic concatenation pipeline emits the new body into `STRIATUM_AGENT_GUIDE.md`
- Gemini single-file guide append wiring
- Two new closed-set chat tools `generate_workflow_preview` and `generate_workflow_write` extending the RFC 0023 V1.5 framework
- Dispatch glue calling the existing RFC 0034 V1 endpoints (`POST /workflows/generate/preview`, `POST /workflows/generate`)
- Operator-confirmation gate reuse from RFC 0013 step 7
- Mutation-not-allowed path: write tools hidden from `tools/list`; fallback refusal with `mutations_disabled` if dispatched anyway
- Audit row append for every mutating chat-tool call (allowed or denied) using the existing RFC 0032 V2 hash-chain append helper
- System-prompt briefing extension mentioning the two new tools and the preview-then-write idiom
- Documentation updates: SPEC, MCP, UBIQUITOUS_LANGUAGE, HOW_TO_AGENT, HOW_TO_HUMAN, RFC 0034 status (§10 deferral → implemented in RFC 0036), RFC 0036 status, CHANGELOG, README

Do NOT:

- author the `examples/` workflow that exercises the chat-generate flow end-to-end (deferred per RFC 0036 §Open Questions);
- add the operator-side `daemon describe --workflow` enhancement (deferred);
- introduce a new MCP server (RFC 0036 is the agent-facing harness on top of the existing MCP surface);
- introduce new capability vocabulary (the seven from RFC 0030 are stable);
- auto-issue capability tokens (operator-only per RFC 0030/0031);
- bypass the RFC 0013 step 7 mutation gate;
- duplicate the RFC 0032 V2 audit-append path;
- add devils_advocate / security review jobs to this dogfood's workflow (deferred per operator decision in commit 9d95487);
- retire the RFC 0023 V1.5 read-only chat tools (they remain).

**Test coverage requirements:**

- Skill install plan unit tests (new skill emitted at all three target paths)
- Chat tool registry unit tests (both new tools registered when `--allow-mutations`; hidden otherwise)
- Chat tool dispatch integration tests (right endpoint, right payload)
- Operator-confirmation gate integration tests (gate is reused; chat model cannot bypass UI gesture)
- Audit row append tests (every mutating call lands an audit row including denials with documented vocabulary)
- Mutation-not-allowed path tests (`tools/list` filtering; fallback refusal)
- Plugin regeneration tests (`.claude-plugin/`, `.codex-plugin/`, `gemini-extension.json` all pick up the new skill body)
- Adversarial test cases from the design prompts (each becomes a unit or integration test)

## Maximize sub-agent usage where it helps

Per the harness profile, native sub-agent delegation is **encouraged**. Use it aggressively for the parts of this implementation that are independent enough to parallelize.

Good candidates:

- one sub-agent for the claude_code + generic skill template authorship (the body is similar across profiles)
- one sub-agent for the `CLAUDE_CODE_SKILLS` tuple update + plan emitter tests
- one sub-agent per chat tool (`generate_workflow_preview`, `generate_workflow_write`)
- one sub-agent for the dispatch wiring + registry extension
- one sub-agent for the mutation-not-allowed path implementation
- one sub-agent for the audit append wiring
- one sub-agent per doc surface (SPEC, MCP, UBIQUITOUS_LANGUAGE, HOW_TO_AGENT, HOW_TO_HUMAN, RFC 0034 status, RFC 0036 status, CHANGELOG, README)
- one sub-agent per test file (skill install plan, chat tool registry, chat tool dispatch, operator-confirmation gate, audit append, mutation-not-allowed path, plugin regeneration)
- exploratory sub-agents to read existing modules (`src/striatum/skills/`, `src/striatum/web/chat/`, `src/striatum/mcp.py`, `src/striatum/daemon_rpc/`, `src/striatum/service.py`, the RFC 0023 V1.5 chat tools location) and produce one-page integration-point summaries

Do NOT delegate (parent session owns these):

- the BUILD_HANDOFF.md authorship
- the integration step where sub-agents' outputs are reconciled
- any `make lint`/`typecheck`/`test`/`smoke` invocation
- final commit-shape and scope discipline

## Verification

Run `make install`, `make lint`, `make typecheck`, `make test`, `make smoke` after all changes are in place.

## Handoff

Produce `docs/dogfood/038/BUILD_HANDOFF.md` summarizing changes, new modules, tests added/passing, deferred items with pointers, and any human-decision items the threat-model review did not pre-resolve. If sub-agents were used, briefly note which sub-tasks were delegated.

The byline must be `author: implementer-codex-gpt-5.5-001` (or whatever the work packet supplies) — plain Markdown line, lowercase `author:`, no decoration.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.

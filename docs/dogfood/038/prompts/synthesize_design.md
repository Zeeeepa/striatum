# Synthesize Design Prompt

Produce `docs/dogfood/038/DESIGN_SYNTHESIS.md`. The file must start with a `striatum.synthesis.v1` front matter block (JSON-encoded values; quote strings; JSON arrays for lists):

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/038/design/codex/DESIGN.md", "docs/dogfood/038/design/claude_code/DESIGN.md", "docs/dogfood/038/design/gemini/DESIGN.md"]
---
```

The byline appears AFTER the front matter block, as a plain Markdown line `author: <slug>` (lowercase, no bold/italics/heading/quotes).

Read all three design artifacts and synthesize ONE implementation plan for RFC 0036 (MCP harness for daemon V2 mutation surface). The synthesis must explicitly choose, not enumerate.

Required sections:

- **Accepted Implementation Scope** — map each RFC 0036 §Acceptance Criteria bullet 1:1 to a concrete code-and-test plan, with one named owner per bullet (which `src/striatum/` module, which test file).
- **Deferred Scope** — the `examples/` workflow exercising the chat-generate flow end-to-end is deferred per RFC 0036 §Open Questions; any operator-side `daemon describe --workflow` enhancement is deferred to a future RFC. Each line says why deferred and where it lands.
- **`striatum-mcp` Skill Body** — concrete section ordering (when to invoke / authoritative reference / common patterns / capability scope / denial recovery / what-not-to-do), what each section contains, example invocation strings.
- **Skill Install Plan Wiring** — `CLAUDE_CODE_SKILLS` tuple update, claude_code + generic templates, gemini single-file guide append.
- **Plugin Bundle Regeneration** — how `striatum plugin install` picks up the new skill across the three bundle shapes.
- **Chat Tool Schemas** — exact input/output JSON schemas for `generate_workflow_preview` and `generate_workflow_write`.
- **Chat Tool Dispatch Wiring** — registry shape, capability check integration, operator-confirmation gate reuse, mutation-not-allowed path semantics.
- **System Prompt Briefing Extension** — exact string change to the RFC 0023 V1.5 system-prompt briefing.
- **Audit Wiring** — how the audit row is appended for every mutating chat-tool call (reuse the RFC 0032 V2 append helper, not a duplicate path).
- **Test Strategy** — every adversarial test case from the design prompts; skill install plan tests; chat tool registry tests; chat tool dispatch tests; operator-confirmation gate tests; audit append tests; plugin regeneration tests.
- **Documentation Deltas** — MCP / HOW_TO_AGENT / HOW_TO_HUMAN / SPEC / UBIQUITOUS_LANGUAGE / RFC 0034 status (§10 deferral → implemented in RFC 0036) / RFC 0036 status / CHANGELOG.
- **Staging Plan** — what lands in V1 (skill + chat tools + mutation-not-allowed path + docs + plugin regeneration) vs what is deferred.
- **Human-Decision Questions** — any open questions the implementer cannot resolve from the synthesis alone.

If the three designs disagree, pick one path and explain the tradeoff. If a guarantee is advisory, label it advisory.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim AFTER the front matter and a blank line.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.

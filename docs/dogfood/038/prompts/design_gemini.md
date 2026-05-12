# Gemini Design Prompt

Produce `docs/dogfood/038/design/gemini/DESIGN.md`.

Design an implementation plan for RFC 0036 with attention to cross-platform reality, skill discoverability across profiles, plugin regeneration semantics, and adversarial test cases.

Your plan must cover:

**Skill body discoverability across the three first-class agent CLIs:**

- claude_code skill bundle at `.claude/skills/<ns>striatum-mcp/SKILL.md` per RFC 0015 V1
- codex agent file at `.codex/agents/<ns>mcp.md` per RFC 0015 step 3 (D063)
- gemini single-file guide append at `GEMINI.md` per the generic-profile concatenation pattern
- Each must render the same skill body content from the same source template; no per-profile divergence beyond formatting

**Plugin regeneration semantics across the three plugin bundle shapes:**

- `.claude-plugin/` plugin manifest + skill bodies (RFC 0025 step 1)
- `.codex-plugin/` plugin manifest + skill bodies (RFC 0025 step 2)
- `gemini-extension.json` extension manifest + skill bodies (RFC 0025 step 3)
- `striatum plugin install` regenerates all three with the new skill body included; no manual file editing required

**Chat tool dispatch wiring against RFC 0023 V1.5:**

- The existing closed-set chat tools (read_file, list_dir, striatum_status, striatum_why, git_log, git_diff) all live in one registry; the new tools (`generate_workflow_preview`, `generate_workflow_write`) extend that registry
- Tool registry shape: name, input schema, output schema, handler function reference, capability requirement (read/write), mutation flag
- Tool dispatch flow: chat model calls tool by name → registry lookup → capability check → handler invocation → response back to chat model with structured envelope

**Cross-platform reality:**

- The skill body is markdown and renders the same on macOS / Linux / Windows-via-WSL
- The chat tools call HTTP endpoints on the local service which work the same across platforms
- The plugin bundles regenerate via the same `striatum plugin install` path on all three OSes
- No platform-specific work expected in V1

**Adversarial test cases (each becomes a unit test):**

- Hostile chat client requesting `tools/list` then `tools/call` with elevated args → tool not in list, refused if dispatched anyway
- Expired token replay attempt → daemon refuses with `token_expired`, audit row appended
- Revoked token replay attempt → daemon refuses with `token_revoked`, audit row appended
- Scope mismatch (token scoped to repo A trying to generate workflow content into repo B's tree) → refused with `capability_missing`, audit records scope mismatch
- Operator-confirmation bypass attempt (chat model fills `confirm_write: true` but UI gesture missing) → UI never fires the HTTP call; no audit row because no request reached the server
- Operator-confirmation forge attempt (chat model tries to call the UI gesture endpoint directly) → endpoint is operator-only; refused with `capability_missing`
- Audit chain tamper attempt via the chat path → daemon role enforcement refuses append outside the append-only contract; existing RFC 0032 V2 audit chain integrity check catches the tamper
- Mutation-not-allowed path probe → tool hidden from `tools/list`; if model still dispatches → server refuses with `mutations_disabled`, audit row appended

**Test coverage strategy:**

- Skill install plan unit tests (asserts the install plan emits the new skill at all three target paths)
- Chat tool registry unit tests (asserts both new tools are registered when `--allow-mutations`, hidden otherwise)
- Chat tool dispatch integration tests (asserts the right endpoint is called with the right payload)
- Operator-confirmation gate integration tests (asserts the gate is reused, not duplicated)
- Audit row append tests (asserts every mutating call lands an audit row, including denials)
- Plugin regeneration tests (asserts the existing plugin install path picks up the new skill body)

**`examples/` workflow that exercises the chat-generate flow end-to-end:**

Per RFC 0036 §Open Questions, this is deferred to a follow-up. Note the deferral; do not author the example here.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-gemini-pro-001`
- Wrong: `**Author:** designer-gemini-pro-001` (the bolded variant that has been the dogfood-031 / 033 friction)
- Wrong: `Author: designer-gemini-pro-001` (capital A)
- Wrong: `author: "designer-gemini-pro-001"` (quoted)

If you produce schema-bearing artifacts (synthesis, finding), the file must start with a JSON-encoded `key: <value>` front matter block. Example for `finding`:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0036"]
---
```

The byline appears AFTER the front matter block and a blank line, not inside it.

**IMPORTANT — produce the artifact, do not surface strategy and exit.** Per the dogfood-036 OPERATOR_REPORT.md intervention #2, a previous gemini design session surfaced a strategy summary and then asked the operator "should I proceed with drafting the formal artifact?" and exited without writing the file. Do not repeat that pattern. The work packet's `expected_artifacts` requires the file on disk; the operator is not on a back-and-forth chat with you — you are inside a supervised wrapper that runs `gemini --prompt -` once per packet, and there is no follow-up turn. Write the DESIGN.md file with byline + body in this single invocation.

Do not call striatum CLI; the operator publishes on your behalf otherwise.

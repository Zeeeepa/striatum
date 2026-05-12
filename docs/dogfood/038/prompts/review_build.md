# Review Build Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0036", "mcp-harness", "chat-tools", "build"]
---
```

Review the implementation under the **threat_model** posture. Verify behavior, tests, docs, migrations, and workflow compatibility. Inspect the repository within the review write scope policy (repo-level access).

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads the daemon's signing key, kills the daemon, or impersonates the daemon process.

Required checks:

- **Capability gating on every chat tool route**: read the chat-tool dispatch code path and verify both `generate_workflow_preview` and `generate_workflow_write` flow through the existing RFC 0032 V2 capability layer. The daemon refuses calls whose token lacks the required capability. Audit row records the denial with documented vocabulary.
- **Operator-confirmation gate is reused, not duplicated**: read the `generate_workflow_write` handler and verify it calls the existing RFC 0013 step 7 mutation gate. A prompt-injected chat model that fills `confirm_write: true` cannot bypass the UI gesture.
- **Mutation-not-allowed path is hidden, not partial**: `tools/list` returned to a no-mutations chat session does not contain `generate_workflow_write`. Fallback dispatch (chat model constructs the call anyway) refuses cleanly with `mutations_disabled` + audit row appended.
- **Default-deny**: unknown tool returns not-found + audit row; missing-capability returns the structured `capability_missing` + audit row.
- **Audit row appended for every mutating chat-tool call including denials**: read the audit-append code path and verify both tools land an audit row in the daemon DB hash chain. Tests exercise both allowed and denied paths.
- **No duplicate audit-append path**: the chat tools use the existing RFC 0032 V2 hash-chain append helper, not a separate path.
- **Skill body correctness**: the `striatum-mcp` skill body teaches the correct denial-vocabulary recovery (`capability_missing`, `token_revoked`, `token_expired`, `method_unknown`) and the correct capability scope semantics. The body does NOT contain "trusted client" framing or wildcard-capability-grant guidance. The body does NOT teach direct `.striatum/state.sqlite3` writes through any path.
- **Skill install plan covers all three target paths**: `striatum skills install --profile all` writes the new skill to `.claude/skills/<ns>striatum-mcp/`, `.codex/agents/<ns>mcp.md`, and the gemini single-file guide.
- **Plugin bundle regeneration**: `striatum plugin install` for each of the three first-class agent CLIs regenerates the plugin bundle with the new skill body included.
- **System prompt briefing extension**: the RFC 0023 V1.5 chat-session briefing mentions the two new tools and the preview-then-write idiom.
- **Documentation honesty**: SPEC, MCP, UBIQUITOUS_LANGUAGE, HOW_TO_AGENT, HOW_TO_HUMAN, RFC 0034 status, RFC 0036 status, CHANGELOG, README reflect actual shipped behavior. No claims of cross-machine semantics, tamper-proof receipts, cryptographic non-repudiation, or malicious-local-root resistance.
- **Tests cover happy paths and adversarial bypasses**: every adversarial test case from the design prompts is exercised; capability denial paths covered; audit append covered; mutation-not-allowed path covered; capability scope mismatch covered; operator-confirmation bypass covered.
- **Write scopes and fixtures do not normalize** direct `.striatum/` edits, audit log tampering, or bypassing the operator-confirmation gate.

Use `needs_revision` for: behavior gaps in the shipped scope, missing tests for the threat surfaces above, capability default-deny failures, audit append gaps, mutation-not-allowed path leaking partial information, "trusted client" framing in the skill body, or documentation that overstates daemon authority. Use `accept_with_findings` for non-blocking cleanup or follow-up RFC scope.

Stay inside the review write scope (`docs/dogfood/038/review/build/threat/`). Do not modify the implementation. Do not call striatum CLI; the operator publishes otherwise.

# Claude Code Design Prompt

Produce `docs/dogfood/038/design/claude_code/DESIGN.md`.

Design an implementation plan for RFC 0036 emphasizing trust boundaries and operator-mistake footguns.

Focus on:

**Trust boundaries:** operator, daemon process, MCP client (potentially prompt-injected), chat session (also potentially prompt-injected), supervised lane process. Per RFC 0031 §Threat Model: scope is over-eager AI agents acting through documented interfaces + operator-mistake footguns; malicious-local-root is out of scope. Documentation must reflect this exactly.

**Capability authorization on every chat tool route:**

- Both `generate_workflow_preview` and `generate_workflow_write` flow through the existing RFC 0032 V2 capability-gating layer because they call the local API endpoints which are mutation-gated.
- The chat model cannot escalate beyond its session's token capabilities; the daemon refuses requests whose token lacks the required capability, audit row appended with `capability_missing`.
- A `write` token scoped to repo A cannot generate workflow content into repo B; the scope mismatch is caught at the daemon layer and refused with `capability_missing`, audit row records the attempted scope.

**Operator-confirmation gate the chat model cannot bypass:**

- Reuse the RFC 0013 step 7 mutation gate: the chat UI surfaces a confirmation gesture before calling `generate_workflow_write`. The chat model passes `confirm_write: true` as a function argument; the UI separately enforces the gesture.
- The gesture is operator-side, not model-side. A prompt-injected chat model that fills `confirm_write: true` cannot bypass the UI gesture.
- The audit row records both the model's claim (`confirm_write: true` in `params_hash`) and the operator's gesture (separate field in the audit row).

**Default-deny gating:**

- `tools/list` returned to a chat session is filtered by token capability. A token with only `read` does not see `generate_workflow_write`.
- A chat session without `--allow-mutations` on the service does not see any write tool, even with a `write` capability token. The flag is the operator's coarse-grained gate; the token is the fine-grained gate.
- Unknown chat tool calls return the standard not-found error and produce an audit row with `decision=denied`, `denial_reason=tool_unknown`.

**Prompt-injection mitigation:**

- Tokens are the operator-controlled gate; a prompt-injected chat model cannot escalate beyond its token's capabilities.
- Short-lived tokens (e.g., `daemon.token.create --capability write --expires-in 1h --repo <id>`) for mutation; documented as the recommended posture in the skill body and HOW_TO_HUMAN.
- Operator UX for revoking a leaked token + the audit chain showing the attack timeline.

**Mutation-not-allowed path:**

- Hidden from `tools/list` is the default. Emitting partial-information errors ("you don't have permission to call this tool") would teach the chat model the tool exists, defeating part of the default-deny posture.
- If the chat model still constructs the call (e.g., from training data or an old session restored), the server returns the structured "mutations disabled" error; audit row appended.

**Operator UX:**

- Operator-side: `striatum daemon describe --workflow <path>` could list required capabilities per workflow shape so the operator knows what tokens to issue. This is forward-looking; V1 of RFC 0036 does not need it.
- Operator-side: `daemon audit show --transport chat` filters audit rows produced by the chat path so the operator can review what the chat tool did.

**Adversarial test cases (must appear in the design):**

- Hostile chat client requesting `tools/list` to enumerate tools, then calling `tools/call` with elevated args.
- Prompt-injected chat model claiming "trusted identity" via system-prompt manipulation.
- Capability token leaked across repos (token scoped to repo A used against repo B → refuse + audit `capability_missing`).
- Operator-confirmation bypass attempt (model passes `confirm_write: true` without the UI gesture → UI never fires the HTTP call).
- Mutation-not-allowed path probe (chat model on a no-mutations service tries to call write tool → tool not in `tools/list`; if dispatched anyway → `mutations_disabled` audit row).
- Audit chain tamper attempt via the chat path (role-enforced append-only refuses).

**Concrete touch points in `src/striatum/`:**

- `src/striatum/skills/install.py` (new skill in fan-out tuple)
- `src/striatum/skills/templates/claude_code/mcp.md.tmpl` + `generic/mcp.md.tmpl` (new files)
- `src/striatum/web/chat/` (or wherever the V1.5 closed-set lives) — new tools + dispatch
- `src/striatum/web/chat/system_prompt.py` (or equivalent) — briefing extension
- `src/striatum/plugin/` (no changes; existing emitter picks up the new skill)

**State what cannot be claimed even after this dogfood lands:**

- The chat-side audit is not a hosted-mode tamper-proof receipt; it's a local hash chain, same as the rest of the audit surface.
- Cross-machine multi-tenant chat semantics — deferred indefinitely (D083 single-user single-machine).
- Malicious-local-root resistance — RFC 0031 threat model is the AI-guardrail framing.
- Auto-issuance of capability tokens — operator-only per RFC 0030/0031.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-claude-opus-001`

The `handoff` kind does not require YAML front matter.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.

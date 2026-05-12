# Codex Design Prompt

Produce `docs/dogfood/038/design/codex/DESIGN.md`.

Design an implementation plan for RFC 0036: the agent-facing harness for the daemon V2 mutation surface. Sit on top of RFC 0015 (skill bundle install), RFC 0025 (plugin bundles), RFC 0023 V1.5 (chat tools closed set + system-prompt briefing), RFC 0030 (daemon RPC method registry), RFC 0031 (daemon-owned supervision + sealed-apply), RFC 0032 V2 (MCP capability-gated `tools/call` + `tools/list` filtering + audit), and RFC 0034 V1 (workflow generator local API). Do not redesign any of those.

Your plan must cover:

**`striatum-mcp` skill body (claude_code + generic templates):**

- Section ordering: `When to invoke` → `Authoritative reference` → `Common patterns` → `Capability scope` → `Denial recovery` → `What not to do`.
- `Authoritative reference` lists the actual daemon RPC methods (`daemon.hello`, `daemon.welcome`, `daemon.describe`) and MCP verbs (`tools/list`, `tools/call`) the agent uses. Be exact.
- `Common patterns` includes copy-paste example invocations of the preview-then-write idiom against the existing RFC 0034 V1 endpoints.
- `Capability scope` covers `repo_id`-scoped vs daemon-global tokens; a `write` token scoped to repo A cannot call write-paths against repo B (default-deny with `capability_missing`; audit row records the scope mismatch).
- `Denial recovery` enumerates the documented denial vocabulary from RFC 0032 V2 audit: `capability_missing`, `token_revoked`, `token_expired`, `method_unknown`, and what the agent should do for each.
- `What not to do` includes: no identity escalation; no direct `.striatum/state.sqlite3` writes through any path; no wildcard capability requests; no loop on `token_revoked` (the audit chain records the loop and the operator decided).

**Skill fan-out wiring:**

- Add `"mcp"` to `CLAUDE_CODE_SKILLS` in `src/striatum/skills/install.py`.
- Add `src/striatum/skills/templates/claude_code/mcp.md.tmpl` and `src/striatum/skills/templates/generic/mcp.md.tmpl`.
- Generic profile concatenates the new body into `STRIATUM_AGENT_GUIDE.md` via the existing concatenation pipeline.
- Gemini single-file guide append: the existing fan-out pattern, no new code.
- Plugin bundles (RFC 0025): `.claude-plugin/`, `.codex-plugin/`, `gemini-extension.json` regenerate with the new skill body via `striatum plugin install`.

**Chat tools `generate_workflow_preview` + `generate_workflow_write`:**

- Closed-set extension to the existing RFC 0023 V1.5 chat tools (read_file, list_dir, striatum_status, striatum_why, git_log, git_diff).
- Input schemas: `WorkflowGenerationSpec` from RFC 0034 V1 for both; `generate_workflow_write` adds required `confirm_write: bool`.
- Output schemas: `GeneratedWorkflow` envelope from RFC 0034 V1 for preview; `{written: [paths], validation: {ok}}` for write.
- Dispatch glue: thin HTTP client over the existing local service surface, calling `POST /workflows/generate/preview` and `POST /workflows/generate`. Reuse the existing service-client helpers; do not duplicate.
- Operator-confirmation gate: reuse the RFC 0013 step 7 mutation gate. The chat model passing `confirm_write: true` is necessary but not sufficient; the chat UI also requires an operator gesture (button or explicit "yes, write it" message). The HTTP call only fires after both are satisfied.
- System-prompt briefing: extend the existing RFC 0023 V1.5 chat-session briefing to mention the two new tools and the preview-then-write idiom. The briefing is templated; the update is a string change in one place.

**Mutation-not-allowed path:**

- `striatum serve` started without `--allow-mutations` hides write tools from `tools/list` returned to the chat session. The model never sees `generate_workflow_write`.
- If the model somehow constructs and dispatches a `generate_workflow_write` call anyway (e.g., across a session restart), the server returns the structured "mutations disabled" error from the existing service surface, audit row appended with `decision=denied`, `denial_reason=mutations_disabled`.

**Audit:**

- Every mutating chat-tool call produces an audit row in the daemon DB hash chain with `client_id`, `repo_id` (if scoped), method, params_hash, decision, denial_reason (if denied), transport=`chat`, audit chain link. Reuse the RFC 0032 V2 audit append helper; do not invent a separate path.

**Concrete touch points in `src/striatum/`:**

- `src/striatum/skills/install.py` — `CLAUDE_CODE_SKILLS` tuple, plan emitter.
- `src/striatum/skills/templates/claude_code/mcp.md.tmpl` — new file.
- `src/striatum/skills/templates/generic/mcp.md.tmpl` — new file.
- `src/striatum/web/chat/tools/` (or wherever the RFC 0023 V1.5 tools live) — add `generate_workflow_preview` and `generate_workflow_write`; extend the closed-set registry.
- `src/striatum/web/chat/system_prompt.py` (or equivalent) — extend the briefing string.
- `src/striatum/service.py` or `src/striatum/web/server.py` — no changes (the endpoints already exist; the chat tools call them).
- `src/striatum/plugin/install.py` — no changes (the existing plan emitter picks up the new skill via the install path).

**Tests:**

- Skill install plan emits the new skill at all three target paths (claude_code / codex / gemini).
- Chat tool registry includes both new tools when `--allow-mutations` is set.
- `tools/list` returned to a chat session hides write tools when `--allow-mutations` is NOT set.
- `generate_workflow_preview` dispatches to the right endpoint and returns the envelope.
- `generate_workflow_write` enforces `confirm_write: true`; refuses without it.
- Operator-confirmation gate from RFC 0013 step 7 is reused (not duplicated).
- Audit row appended for every mutating chat-tool call including denials.
- Plugin regeneration covers the new skill body.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value.
- Lowercase `author:` exactly.
- Correct: `author: designer-codex-gpt-5.5-001`
- Wrong: `**Author:** ...`, `Author: ...`, `# author: ...`, `author: "..."`.

The `handoff` artifact kind does not require YAML front matter. Synthesis and finding artifacts later in this dogfood will, with the JSON-encoded block.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.

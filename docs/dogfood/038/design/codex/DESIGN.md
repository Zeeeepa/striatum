author: designer-codex-gpt-5.5-001

# RFC 0036 MCP Harness Implementation Design

Status: design handoff
Date: 2026-05-12

## Design Position

RFC 0036 should land as a harness layer over surfaces that already exist:
RFC 0015 skill installation, RFC 0025 plugin bundle generation, RFC 0023
chat tools, RFC 0032 daemon MCP capability gating, and RFC 0034 workflow
generation endpoints. The implementation should not add a new MCP server, a
new daemon capability vocabulary, or a new workflow-generation API.

The two product slices are independent but related:

- Add a `striatum-mcp` skill body so agent CLIs know how to use the daemon
  MCP mutation surface safely.
- Add chat tools `generate_workflow_preview` and `generate_workflow_write`
  so the web chat surface can call the existing RFC 0034 generator endpoints
  using preview-before-write and an operator confirmation gate.

## Skill Bundle Plan

Add `mcp` to `CLAUDE_CODE_SKILLS` in `src/striatum/skills/install.py`:

```python
CLAUDE_CODE_SKILLS: tuple[str, ...] = (
    "workflow",
    "scaffold",
    "claim-loop",
    "supervise",
    "recover",
    "mcp",
)
```

This single tuple change makes `claude_code` emit
`.claude/skills/<namespace>mcp/SKILL.md` and makes `codex` emit
`.codex/agents/<namespace>mcp.md`, because the Codex plan already reuses
`CLAUDE_CODE_SKILLS`. The existing Gemini guide is a single template and
will not pick up the new tuple automatically; update
`src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl` to
append the same MCP section. Update the generic guide template to include
the MCP section from `src/striatum/skills/templates/generic/mcp.md.tmpl` or
refactor the generic renderer to concatenate per-surface templates before
rendering. Prefer the smaller implementation if the current renderer has no
generic concatenation helper.

Add these files:

- `src/striatum/skills/templates/claude_code/mcp.md.tmpl`
- `src/striatum/skills/templates/generic/mcp.md.tmpl`

The body order is fixed:

1. `When to invoke`
2. `Authoritative reference`
3. `Common patterns`
4. `Capability scope`
5. `Denial recovery`
6. `What not to do`

The authoritative reference must name `daemon.hello`, `daemon.welcome`,
`daemon.describe`, MCP `tools/list`, and MCP `tools/call`. It should also
state that `tools/list` is filtered by the token's effective capability set
and that `tools/call` re-authorizes every call.

The common patterns should use copy-paste JSON-RPC examples, not invented CLI
verbs. Preview should call the daemon MCP tool name that maps to RFC 0034's
preview route; write should call the write tool with `confirm_write: true`.
The examples should include the `WorkflowGenerationSpec` object under
`spec`, because the existing HTTP endpoint expects `{ "spec": ... }`.

The denial section is a closed table:

| Denial | Agent response |
|---|---|
| `capability_missing` | Stop the attempted mutation and ask the operator for a short-lived token with the named capability and correct repository scope. |
| `token_revoked` | Stop retrying. Treat revocation as an operator decision. |
| `token_expired` | Ask the operator for a fresh token with the same narrow scope. |
| `method_unknown` | Call `daemon.describe` and use the advertised method name; treat persistent mismatch as version skew. |

The "What not to do" section must include no identity escalation, no direct
`.striatum/state.sqlite3` writes, no wildcard/admin capability requests for
ordinary workflow generation, and no loop on `token_revoked`.

## Plugin Regeneration

RFC 0025 plugin bundles copy skill bodies into plugin layouts under
`src/striatum/plugins/templates/<profile>/skills/`. Add the MCP skill file
to each first-class plugin profile:

- `src/striatum/plugins/templates/claude_code/skills/mcp.md.tmpl`
- `src/striatum/plugins/templates/codex/skills/mcp.md.tmpl`
- `src/striatum/plugins/templates/gemini/skills/mcp.md.tmpl`

Then update the plugin install plan's skill list to include `mcp`. If the
plugin installer has an independent tuple, make it mirror
`CLAUDE_CODE_SKILLS` instead of maintaining a second list. That avoids the
next skill addition drifting between `skills install` and `plugin install`.

## Chat Tools Plan

Extend `src/striatum/web/chat_tools.py`, which currently owns the RFC 0023
V1.5 closed set and dispatch. Add two tool schemas:

```text
generate_workflow_preview(spec: WorkflowGenerationSpec) -> GeneratedWorkflow
generate_workflow_write(spec: WorkflowGenerationSpec, confirm_write: bool) -> {"written": [...], "validation": {"ok": true}}
```

The `spec` schema should be the RFC 0034 V1 `WorkflowGenerationSpec` shape.
Use `type: object` with `additionalProperties: true` at the chat-tool schema
boundary unless the project already exposes a reusable JSON schema for the
generator spec. The authoritative validation remains
`WorkflowGenerationSpec.from_json()` inside the existing service endpoint.

Dispatch should be thin. Do not call `generate_workflow()` directly from
chat tools, and do not duplicate generator write logic. Add a local service
client helper that posts JSON to:

- `POST /workflows/generate/preview`
- `POST /workflows/generate`

The helper should return the service envelope as a compact JSON string for
the model. Preserve field-specific errors from the service, including
`field_path`, `hint`, and `ref`.

`generate_workflow_preview` is always visible because it writes nothing.
`generate_workflow_write` is visible only when the web service was started
with `--allow-mutations` and the chat UI has an operator-confirmation state
for the pending write.

## Operator Confirmation Gate

The model passing `confirm_write: true` is necessary but not sufficient.
Reuse the existing RFC 0013 step 7 web mutation gate: the browser/server
must require an operator gesture before executing the write tool.

Recommended flow:

1. Model calls `generate_workflow_preview`.
2. UI renders generated graph, file list, and validation summary.
3. UI stores a pending write request keyed by a hash of the preview spec.
4. Operator clicks a write button or sends an explicit confirmation that the
   chat lifecycle recognizes as confirmation for that pending spec.
5. Only then does the server expose/execute `generate_workflow_write` with
   `confirm_write: true`.

This avoids a prompt-injected model placing `confirm_write: true` into a tool
call and bypassing the human review moment.

## Mutation-Disabled Behavior

When `striatum serve` is started without `--allow-mutations`:

- The chat tool list must omit `generate_workflow_write`.
- The system-prompt briefing must say preview is available but writing is
  disabled for this service session.
- If a stale transcript or crafted request still tries
  `generate_workflow_write`, dispatch must return the existing structured
  service error for mutations disabled. The returned denial vocabulary should
  use `mutations_disabled` for chat-tool audit, while the HTTP response can
  preserve the current `405` shape from `service.py`.

## Audit Plan

Both chat tools should leave daemon-side metadata evidence with
`transport = "chat"`. `generate_workflow_preview` is read-only, so it can use
the request-log/read-audit path with `decision = "allowed"` or a structured
denial when dispatch fails before generation. `generate_workflow_write` is a
mutating chat-tool call and must append a daemon audit row, allowed or denied.
Use the RFC 0032 audit helper that already backs daemon MCP mutation calls; do
not create a chat-only audit format.

Audit fields:

- `transport = "chat"`
- `method = "workflow.generate"` or the exact daemon method name chosen for
  the RFC 0034 write route
- `client_id`
- `repo_id` when scoped
- `params_hash`, never raw spec content
- `decision = "allowed" | "denied"`
- `denial_reason` for denied calls, including `mutations_disabled`,
  `capability_missing`, `token_revoked`, `token_expired`, or
  `method_unknown`
- hash-chain linkage through the daemon DB

Preview calls do not mutate repository files, but RFC 0036 should still make
them visible in the daemon metadata chain so the operator can correlate a
write with the preview that preceded it. The mutating audit guarantee applies
to write attempts; the traceability guarantee applies to both tools.

## Threat Surfaces

Prompt-injected MCP client requesting elevated mutation: capability tokens
remain the only authority. `tools/list` filtering is usability only;
`tools/call` re-authorizes the requested method and repository scope. A model
claiming to be trusted or asking for admin is ignored by the daemon.

Capability token leaked across repositories: repo-scoped tokens authorize
only their registered `repo_id`. A write token for repo A calling a write
path against repo B receives `capability_missing`; the audit row records the
scope mismatch without logging the token secret.

Operator-confirmation bypass attempt: `confirm_write: true` in model output
does not execute a write. The server must require the RFC 0013 web mutation
gate's operator gesture for the pending previewed spec before dispatch.

Audit-chain coverage for both tools: preview is read-only and should be logged
as chat transport metadata so a later write can be correlated to its preview;
write is mutating and always audit-logged, including denials. Do not describe
preview as a mutating call, but do include it in the trace chain.

Denial-vocabulary handling: the skill, chat tool error strings, and tests
should use the same vocabulary. `capability_missing`, `token_revoked`,
`token_expired`, and `method_unknown` come from RFC 0032. `mutations_disabled`
is the chat/service mutation-gate denial and should be documented separately
so it is not confused with daemon capability failure.

## Concrete Touch Points

| Area | Files |
|---|---|
| Skill fan-out | `src/striatum/skills/install.py`, `src/striatum/skills/templates/claude_code/mcp.md.tmpl`, `src/striatum/skills/templates/generic/mcp.md.tmpl`, `src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl` |
| Plugin bundles | `src/striatum/plugins/install.py`, `src/striatum/plugins/templates/*/skills/mcp.md.tmpl` |
| Chat tool registry | `src/striatum/web/chat_tools.py` |
| Chat briefing | `_build_chat_briefing()` in `src/striatum/service.py` unless it is split into a dedicated `system_prompt.py` first |
| Service endpoints | No new endpoints; use existing handlers in `src/striatum/service.py` for `/workflows/generate/preview` and `/workflows/generate` |
| Daemon method registry | Add generator preview/write method entries only if chat dispatch routes through daemon RPC method names; otherwise leave daemon registry unchanged and audit at the chat gateway |
| Tests | `tests/test_skills_install.py`, `tests/test_plugin_install.py`, `tests/test_web_chat_tools.py` or existing chat/service test files |

## Test Plan

- `striatum skills install --profile claude_code --dry-run --json` includes
  `.claude/skills/striatum-mcp/SKILL.md`.
- `striatum skills install --profile codex --dry-run --json` includes
  `.codex/agents/striatum-mcp.md`.
- `striatum skills install --profile gemini --dry-run --json` includes the
  MCP section in `striatum-STRIATUM_GEMINI_GUIDE.md`.
- `striatum skills install --profile generic --dry-run --json` includes the
  MCP section in `striatum-STRIATUM_AGENT_GUIDE.md`.
- Plugin dry runs for `claude_code`, `codex`, and `gemini` include the MCP
  skill body.
- Chat tool schemas include `generate_workflow_preview` in all chat sessions.
- Chat tool schemas include `generate_workflow_write` only when mutations are
  allowed and the confirmation gate can be satisfied.
- Preview dispatch posts to `/workflows/generate/preview`, returns the
  `GeneratedWorkflow` envelope, and writes no files.
- Write dispatch refuses missing `confirm_write: true`.
- Write dispatch refuses without operator confirmation even when
  `confirm_write: true` is present.
- Mutation-disabled write attempts return structured `mutations_disabled`
  errors and do not write files.
- Allowed write attempts call `/workflows/generate` and return written paths
  plus validation status.
- Preview appends daemon metadata with `transport = "chat"` and a params hash
  so the operator can correlate it with a later write.
- Every write attempt appends a daemon audit row with `transport = "chat"`;
  tests cover both allowed and denied rows.

## Documentation Updates

- `docs/MCP.md`: add an agent-facing mutation section with effective tool
  sets, preview-before-write, token scope, denial recovery, short-lived-token
  posture, and audit guarantees.
- `docs/HOW_TO_AGENT.md`: list `striatum-mcp` alongside the existing skills
  and state when an agent should use it instead of CLI commands.
- `docs/HOW_TO_HUMAN.md`: describe issuing, scoping, expiring, and revoking
  capability tokens for agents that use daemon MCP or chat generation.
- `docs/SPEC.md`: record that RFC 0036 implements the chat-assisted
  workflow-generation tool over the existing RFC 0034 endpoints.
- `docs/UBIQUITOUS_LANGUAGE.md`: clarify MCP mutation surface, effective
  tool set, operator-confirmed chat mutation, and `mutations_disabled`.
- `docs/rfcs/0034-workflow-generator-and-template-catalog.md`: update the
  section 10 deferred note to "implemented by RFC 0036".
- `CHANGELOG.md`: note the new skill and chat tools.

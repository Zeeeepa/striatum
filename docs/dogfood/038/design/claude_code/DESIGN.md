# RFC 0036 MCP Harness: Implementation Design

author: designer-claude-opus-001

## 1. Posture and Scope

This design implements RFC 0036 V1 — the agent-facing harness on top of the
already-shipped daemon V2 mutation surface — through the **trust-boundary**
lens: the reviewer will judge whether the design contains an over-eager AI
agent (CLI client, MCP client, or chat session) and the operator's own
mistakes, while continuing to refuse to claim defenses against a malicious
local root.

V1 ships **one new skill (`striatum-mcp`) + two new chat tools
(`generate_workflow_preview` and `generate_workflow_write`) wired through
the existing RFC 0023 V1.5 closed-set dispatch and the RFC 0034 V1 local
API endpoints**. No new MCP server, no new capability vocabulary, no new
mutation gate, no new audit shape, no new local API endpoints. Everything
new in this RFC is harness that **teaches and routes through machinery
that already exists**.

Explicit non-goals (forwarded from the RFC, restated here so the synthesis
can hold the line):

- A new MCP server. Daemon V2 already provides `tools/call` + `tools/list`
  per RFC 0028 / RFC 0032; we add the agent-facing harness on top.
- A new capability vocabulary. The seven existing capabilities
  (`read`/`write`/`review`/`claim`/`apply`/`admin`/`recovery`) from
  RFC 0030 are stable.
- Auto-issuance of capability tokens. Operators issue them explicitly via
  admin-only paths per RFC 0030 / RFC 0031.
- Web UI redesign. We add two tools to the existing chat-tools closed set;
  we do not change the chat lifecycle, the streaming shape, or the
  provider plumbing.
- Cross-machine / multi-tenant chat semantics — deferred indefinitely per
  D083 (daemon V2 is single-user, single-machine).

## 2. Trust Boundaries

Per RFC 0031 §Threat Model, **scope is over-eager AI agents acting through
documented interfaces, plus operator-mistake footguns**. Malicious-local-
root is out of scope. The design preserves that framing verbatim and does
not bolt on defenses that pretend otherwise.

The five trust principals in this design and their privileges:

1. **Operator (human at the terminal).** Highest trust. Issues + revokes
   capability tokens. Starts `striatum serve` with or without
   `--allow-mutations`. Performs the chat-UI confirmation gesture. Reads
   the audit chain. The operator is the only principal that can grant
   write authority; nothing else in the system can escalate around them.
2. **Daemon process (`striatum daemon`, with PostgreSQL).** Trusted to
   enforce capability authorization, append-only audit rows, and the
   RFC 0030 method registry. The daemon is the only writer of audit and
   request-log rows. Its trust is bounded by the OS account that runs it
   (no privilege boundary above local-root, by design).
3. **Local API service (`striatum serve`).** Trusted to enforce the
   coarse-grained `--allow-mutations` gate and the per-endpoint
   `confirm_write: true` check (service.py lines 696-711). Forwards
   capability-gated calls to the daemon when daemon mode is on; in
   daemon-off SQLite-only mode the gates still apply but the audit row
   target is the local SQLite request log.
4. **MCP client (Claude Code, Codex, Gemini, etc.) holding a capability
   token.** Untrusted past its token's stated authority. May be prompt-
   injected. Token capabilities, expiry, and repo scope are the *only*
   reasons the daemon ever lets a call through; identity claims in the
   call body are ignored.
5. **Chat session (LLM in the web chat surface from RFC 0023 V1.5).**
   Untrusted; may also be prompt-injected. Backed by the same local API
   `--allow-mutations` gate as everything else, plus the operator-side
   UI confirmation gesture that the model cannot drive. The model's
   `confirm_write: true` argument is *not* the gate; it is data recorded
   in the audit row alongside the operator's separate gesture.

Cross-cutting: the **supervised lane process** (claude-code / codex /
gemini CLI under a striatum supervisor) is downstream of all of the
above. RFC 0036 does not change its privileges. A lane process that
wants to mutate workflow state through MCP must hold its own capability
token; lane attestation (D080) does not implicitly grant write authority.

### 2.1 What we explicitly do not claim

- The audit chain is a **local hash chain on a local DB**. It is not a
  cryptographically tamper-proof hosted receipt. An attacker with local-
  root can rewrite history; we accept that and stay in scope.
- The chat session's prompt-injection containment is **capability-token
  bounded**, not content-bounded. Nothing in the chat path inspects the
  model's text for "suspicious instructions"; we rely on the token gate
  and the operator gesture, both of which are external to the model.
- Cross-repo / cross-machine flows continue to be **deferred** per D083.
  RFC 0036 does not weaken or revisit that boundary.
- Capability tokens are **never auto-issued** by anything in this RFC.
  The skill teaches operators what to ask for; it never asks for itself.

## 3. The `striatum-mcp` Skill

### 3.1 Template files

Two new files, one each for the claude_code (canonical) and generic
(concatenation) profile families:

- `src/striatum/skills/templates/claude_code/mcp.md.tmpl`
- `src/striatum/skills/templates/generic/mcp.md.tmpl`

The claude_code template is the body of truth; the generic template
contains the same body shaped for inline concatenation into
`STRIATUM_AGENT_GUIDE.md`. Per the existing `_plan_codex` fan-out
(install.py:309-329), codex reuses the claude_code body verbatim; per
`_plan_gemini` (install.py:332-351), the gemini guide appends the
section. No new fan-out code is needed beyond extending the
`CLAUDE_CODE_SKILLS` tuple.

### 3.2 Skill body sections

The template renders six sections, matching the existing five-skill
shape (workflow.md.tmpl / claim-loop.md.tmpl / etc.) so the bundle
remains consistent:

1. **When to invoke**
   - The agent holds a capability token and wants to mutate workflow
     state through MCP rather than the CLI.
   - The agent wants to generate a workflow through the local API
     rather than ask the operator to run the CLI.
   - The agent saw a `capability_missing` / `token_revoked` /
     `token_expired` / `method_unknown` denial and wants to recover.
2. **Authoritative reference**
   - `daemon.hello` / `daemon.welcome` — handshake.
   - `daemon.describe` — broad method discovery (operator-side
     authoritative; agent-side falls back to `tools/list`).
   - `tools/list` — the **effective tool set**: method registry ∩ token
     capabilities ∩ repo scope. The agent reads this first to know what
     it can actually call.
   - `tools/call` — invoke a method. Mutating calls pass `confirm_write:
     true` where the method declares it.
   - Audit rows are appended for **every mutating call**, including
     denials. The agent cannot suppress them.
3. **Common patterns**
   ```text
   # Effective tool set
   tools/list -> {tools: [...]}

   # Preview before write (safe, no capability beyond `read`)
   tools/call name=workflow.generate.preview args={spec}
     -> {workflow: {...}, files: [...], metadata: {...}, validation: {ok}}

   # Write with operator confirmation (requires `write` + UI gesture)
   tools/call name=workflow.generate args={spec, confirm_write: true}
     -> {written: [...], validation: {ok}}
   ```
4. **Capability scope**
   - A `write`-capability token scoped to repo A cannot call write-paths
     against repo B. The daemon refuses with `capability_missing` and
     appends an audit row recording the attempted scope mismatch.
   - A token with only `read` does not see `write` tools in
     `tools/list`. This is **filtering, not refusal**: the tool simply
     isn't visible. (Confirmed in mcp.py:543-552: `tools/list` only
     appends entries when `auth.decision == "allowed"`.)
5. **Denial recovery**
   - `capability_missing`: ask the operator for a token with the
     required capability. The required capability is named in the audit
     row's `denial_reason` and in the method registry; the agent should
     not guess.
   - `token_revoked`: stop retrying. The operator made an explicit
     decision; looping is itself recorded in the audit chain.
   - `token_expired`: short-lived tokens expire by design. Ask for a
     fresh one with the same scope and capability.
   - `method_unknown`: typo or version skew. Re-read `tools/list`; if
     still absent, call `daemon.describe` from the operator's side.
6. **What not to do**
   - Don't escalate by claiming a different identity. The daemon never
     bypasses capability gating regardless of client identity claims
     (the auth context is constructed from the token alone in
     capability.py:52-103).
   - Don't write to `.striatum/state.sqlite3` directly. There is no
     non-admin RPC method that exposes raw DB writes; mutations flow
     through capability-gated RPC.
   - Don't request wildcard capability ("give me admin"). Wildcard
     grants are explicitly listed in the RFC 0031 footgun catalog. The
     correct posture is the narrowest capability that fits the task,
     short-lived.
   - Don't loop on `token_revoked`. The audit chain records the loop;
     the operator sees it.

### 3.3 Install fan-out

Extend the existing `CLAUDE_CODE_SKILLS` tuple in
`src/striatum/skills/install.py:51-57`:

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

The downstream fan-out paths already iterate this tuple
(`_plan_claude_code` line 353, `_plan_codex` line 309) so the new skill
appears at:

- `.claude/skills/<ns>striatum-mcp/SKILL.md`
- `.codex/agents/<ns>mcp.md`

The generic single-file guide is rendered from a *different* template
(`generic/STRIATUM_AGENT_GUIDE.md.tmpl`) that already concatenates the
existing skill bodies inline. To pick up the new skill body, that
generic template must append a new section sourced from
`generic/mcp.md.tmpl`. We do this by adding a render call in the same
shape as the existing sections in the generic template (mechanical;
the renderer's `_StrictFormatMap` plus `_expand_helpers` does the
substitution).

The gemini single-file guide (`gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`)
gets the same append. The two single-file templates stay
deterministic so `--profile all` produces a stable plan.

### 3.4 Plugin bundle regeneration

Per RFC 0025, plugin bundles are emitted from the same templates by
`striatum plugin install`. No emitter changes are required — the
existing emitter iterates the bundled templates, so the new
`mcp.md.tmpl` is picked up automatically. We add one unit test
asserting that running `striatum plugin install` after this RFC
ships includes a section for the mcp skill in each provider's bundle.

## 4. The Two New Chat Tools

### 4.1 Where they live

The chat-tools closed set lives in `src/striatum/web/chat_tools.py` (the
RFC 0023 V1.5 list at lines 49-143). We extend the `_TOOLS` list with
two more entries; the existing `TOOL_NAMES` / `ANTHROPIC_TOOLS` /
`OPENAI_TOOLS` derivations pick them up automatically.

The dispatch happens in `execute_tool` (chat_tools.py:189-221). We add
two new branches that call the local API service via `striatum.api.invoke`
(the same in-process indirection the existing `striatum_status` and
`striatum_why` tools use, chat_tools.py:308-330).

### 4.2 Tool schemas

Flavor-neutral schemas, added to `_TOOLS` in the order they appear in
the briefing:

```python
{
    "name": "generate_workflow_preview",
    "description": (
        "Preview a workflow generated from a WorkflowGenerationSpec "
        "without writing any files. Safe to call freely; returns the "
        "GeneratedWorkflow envelope (graph, files, metadata, validation)."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "spec": {
                "type": "object",
                "description": (
                    "A WorkflowGenerationSpec JSON object. See "
                    "docs/rfcs/0034-workflow-generator-and-template-catalog.md."
                ),
            },
        },
        "required": ["spec"],
        "additionalProperties": False,
    },
},
{
    "name": "generate_workflow_write",
    "description": (
        "Write a generated workflow to disk. Requires --allow-mutations "
        "on the service, requires confirm_write: true in the call, and "
        "requires the operator to perform the UI confirmation gesture. "
        "The model's confirm_write argument is recorded in the audit row "
        "but cannot bypass the UI gesture."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "spec": {
                "type": "object",
                "description": "WorkflowGenerationSpec JSON object.",
            },
            "confirm_write": {
                "type": "boolean",
                "description": (
                    "Must be true. Recorded in the audit row alongside "
                    "the operator's separate UI gesture."
                ),
            },
        },
        "required": ["spec", "confirm_write"],
        "additionalProperties": False,
    },
},
```

The descriptions are deliberately blunt about the operator gesture so a
prompt-injected model that tries to fill `confirm_write: true` cannot
plausibly claim it didn't know the UI gesture was separate.

### 4.3 Dispatch glue

Both tools call the existing RFC 0034 V1 service endpoints
(`POST /workflows/generate/preview` and `POST /workflows/generate`,
service.py:684-720). The simplest dispatch is in-process: import
`generate_workflow` and `write_generated_workflow` directly the same
way `_handle_workflow_generate` does (service.py:685-686) and call them
from the new branches in `execute_tool`. This keeps the chat path
identical to the REST path so the gating and audit shape line up.

The new branches look like (schematic, not final code):

```python
if name == "generate_workflow_preview":
    return _tool_generate_workflow_preview(repo, args.get("spec"))
if name == "generate_workflow_write":
    return _tool_generate_workflow_write(
        repo,
        args.get("spec"),
        bool(args.get("confirm_write", False)),
    )
```

`_tool_generate_workflow_preview` returns the same `GeneratedWorkflow`
JSON the REST endpoint emits, wrapped through `wrap_tool_result` so the
RFC 0023 V1.5 BEGIN/END delimiter convention applies. It does **not**
require `confirm_write` and does **not** require `--allow-mutations`
(matching service.py:695-708, where `preview` is the un-gated branch).

`_tool_generate_workflow_write` enforces three gates before any write
happens:

1. **`--allow-mutations` gate.** Check the same `state.allow_mutations`
   flag the REST endpoint checks. If false, return a structured
   `mutations_disabled` error string. (See §6 for why this path is
   visible at all.)
2. **`confirm_write: true` argument.** If not true, return a
   `confirm_write_missing` error. Recorded in the audit row.
3. **Operator UI gesture.** The chat-side dispatch will not invoke the
   write tool until the chat UI surfaces a confirmation gesture and the
   operator presses it. The model **never sees** the gesture token; the
   UI consumes a one-shot confirmation token and only then routes the
   `tools/call` through to `execute_tool`. This is the RFC 0013 step 7
   mutation gate reused; see §5.

On success, the write tool returns the `{written: [...], validation:
{ok}}` payload from `write_generated_workflow`.

### 4.4 System-prompt briefing extension

`_build_chat_briefing` in `src/striatum/service.py:222-296` lists the
available tools at line 289 ("read_file, list_dir, striatum_status,
striatum_why, git_log, git_diff"). We extend that list with the two new
names and add a paragraph about the preview-then-write idiom and the
operator-gesture requirement.

The briefing addition (verbatim text we plan to insert):

> Two mutation-adjacent tools are available **only on services started
> with `--allow-mutations`**: `generate_workflow_preview` (safe to call
> freely, returns the generated workflow without writing it) and
> `generate_workflow_write` (writes the workflow to disk, requires
> `confirm_write: true` AND a separate operator confirmation gesture in
> the chat UI). The operator's gesture is enforced by the UI, not by
> you; passing `confirm_write: true` is necessary but not sufficient.
> Always call `generate_workflow_preview` first and surface the result
> to the operator before suggesting a write.

The briefing keeps the existing BEGIN/END delimiter instruction so tool
results from the new tools are also treated as data, not instructions.

## 5. The Operator-Confirmation Gate

### 5.1 Reuse of RFC 0013 step 7

RFC 0013 step 7 specifies that the chat UI surfaces a one-shot
confirmation gesture before any mutation flows. In the chat surface
today, this is the `Mutations are gated. Start the service with
--allow-mutations to enable new chat sessions.` flow visible in
`templates/chat_index.html:27` and the `allow_mutations` cache in
`static/app.js:128`. We extend that with a per-call confirmation token
for the new write tool:

- When the model emits a `generate_workflow_write` tool call, the chat
  UI **intercepts** the dispatch instead of routing straight to
  `execute_tool`. The intercept renders an inline preview (using the
  paired `generate_workflow_preview` envelope) and a "Write this
  workflow" button.
- The button generates a server-issued one-shot confirmation token
  bound to the (chat_session_id, tool_call_id) pair, with a short TTL
  (e.g., 60s). The model cannot guess or forge this token; it never
  appears in any tool-result the model has seen.
- Only when the operator clicks the button does the UI submit the
  tool-call envelope plus the confirmation token to the service. The
  service then invokes `execute_tool` for the write tool with the
  gesture already validated.

### 5.2 What the chat model can and cannot do

- The model **can** fill `confirm_write: true`. This is its claim of
  intent; it appears in `params_hash` on the audit row.
- The model **cannot** produce the confirmation token. The token is
  never rendered to the model's view.
- The model **cannot** retry past a denied gesture. If the operator
  closes the inline preview without confirming, the UI surfaces a
  `mutation_canceled` result to the model; the model can ask again,
  but a fresh gesture is required.
- The model **cannot** bypass `--allow-mutations`. If the service is
  started without it, the write tool is hidden from `tools/list`
  (see §6).

### 5.3 Audit-row shape for the chat path

Every chat-mediated write call appends an audit row with these fields
(reusing the RFC 0030 audit schema; daemon.py:537-547):

- `method = "workflow.generate"` (or the local-API equivalent; the
  audit shape parallels the daemon RPC method registry even for
  service-only endpoints).
- `transport = "chat"` (new value in the existing `transport` column;
  joins the existing `mcp` / `cli` / `daemon_rpc` set).
- `decision = "allowed" | "denied"`.
- `denial_reason` is one of `capability_missing`, `token_revoked`,
  `token_expired`, `mutations_disabled`, `confirm_write_missing`,
  `operator_gesture_missing`, or null when allowed.
- `params_hash` includes `confirm_write` as the model recorded it.
- A new `operator_gesture` field captures the gesture state as the UI
  recorded it: `confirmed` (button pressed), `denied` (operator closed
  without confirming), or `absent` (model attempted to dispatch without
  gesture). This split is the auditable distinction between the model's
  claim and the operator's act.

The audit chain hash continues to be append-only and role-enforced
exactly as in daemon.py:537-547 / daemon_rpc/request_log.py:78-104. We
add no new write paths into the audit table.

## 6. The Mutation-Not-Allowed Path

### 6.1 Default: hide, do not refuse

When the service is started without `--allow-mutations`:

- `tools/list` returned to the chat session contains
  `generate_workflow_preview` but **omits** `generate_workflow_write`.
  This matches the RFC 0032 capability-filtering posture: the model
  does not see tools it cannot call. (Implementation: in the chat-tools
  path, the tool list is filtered by `state.allow_mutations` before
  being passed to `ANTHROPIC_TOOLS` / `OPENAI_TOOLS` builders.)
- The system-prompt briefing's mention of the write tool is conditioned
  on `state.allow_mutations`; on a mutations-disabled service, the
  briefing simply does not mention the write tool exists.

This is deliberate. Emitting a "you don't have permission to call this
tool" error would *teach the model the tool exists*, defeating part of
the default-deny posture. A model that does not know a tool name will
not synthesize a call to it from training data with any reliability.

### 6.2 Fallback: structured error if the model dispatches anyway

If a chat session restored from history (or a model with stale training
data) constructs a `generate_workflow_write` call against a
mutations-disabled service, `execute_tool` returns the structured
result:

```text
[error] mutations_disabled: service started without --allow-mutations;
ask the operator to restart with --allow-mutations to enable workflow
write
```

An audit row is appended with `decision=denied,
denial_reason=mutations_disabled, transport=chat,
operator_gesture=absent`. This is the same shape the REST endpoint
already produces at service.py:696-708; we route through the same code
path.

### 6.3 Why hiding > refusing

A documented capability vocabulary (`capability_missing`,
`token_revoked`, `token_expired`, `method_unknown`) is for cases where
the *agent must know what to ask for next*. The mutation-not-allowed
case is different: the operator has made a deliberate choice not to
allow mutation. There is nothing the agent can ask the operator for; a
better posture is to keep the surface small. The agent learns the tool
exists by the operator restarting the service; the agent never learns
of a "hidden" tool from a denial message.

## 7. Capability Authorization on Every Chat Tool Route

The two new tools flow through the existing RFC 0032 V2 capability-
gating layer because they call local API endpoints which are mutation-
gated. The flow for the write tool, end-to-end, is:

1. Chat model emits `generate_workflow_write` tool call.
2. Chat UI intercepts; renders preview + confirmation button.
3. Operator clicks button; UI submits envelope + one-shot gesture
   token.
4. Service validates gesture token (binding + TTL).
5. Service checks `state.allow_mutations`; if false →
   `mutations_disabled`, audit + return.
6. Service checks `confirm_write: true`; if false →
   `confirm_write_missing`, audit + return.
7. Service calls into the local API generator path
   (`write_generated_workflow`), which is the same function the REST
   endpoint calls. If the daemon is enabled and the generator path
   touches a capability-gated daemon RPC method (e.g., a future
   workflow-registration RPC), the daemon's capability check applies
   exactly as today (capability.py:52-103). If the call lacks the
   required capability for that token → `capability_missing`, audit +
   return.
8. Write happens; audit row appended with `decision=allowed`.

The chat model never holds the daemon token directly in the V1 design;
the *service* holds the daemon-side identity it was launched with. The
chat surface gates writes at the service layer (`--allow-mutations` +
confirm + gesture). Future RFCs may issue per-chat-session capability
tokens; for V1, the coarser-grained service gate is the boundary.

**Cross-repo isolation.** If the workflow spec targets a `repo_id`
different from the service's bound repository, the local API rejects
it at the same layer the REST endpoint does today; we do not relax
that check for the chat path. A `write` token scoped to repo A cannot
write into repo B regardless of how the spec is shaped; the audit row
records the attempted scope mismatch.

## 8. Adversarial Test Cases

These test cases must appear in the test suite for RFC 0036 to land.
Each names the principal it stresses and the expected outcome.

### 8.1 Hostile chat client enumerating tools

A chat client (operator-malicious or prompt-injected) calls
`tools/list` and then `tools/call` with elevated args (e.g., a
`generate_workflow_write` call against a service started without
`--allow-mutations`). Expected:

- `tools/list` omits `generate_workflow_write` entirely.
- If the client constructs the call name from training data anyway,
  `execute_tool` returns the `mutations_disabled` structured error.
- One audit row appended with `decision=denied,
  denial_reason=mutations_disabled, transport=chat,
  operator_gesture=absent`.

### 8.2 Prompt-injected chat model claiming trusted identity

The model emits text claiming to be "the operator" or "trusted system
prompt" and then constructs `generate_workflow_write` with
`confirm_write: true`. Expected:

- The chat UI intercepts the dispatch as in §5.1; the model's text
  does not affect the gesture path.
- Without the operator pressing the button, the call is never
  submitted to the service.
- If the model retries, each attempt produces a `tool_call` event the
  UI handles identically; no privilege escalation.

### 8.3 Capability token leaked across repos

A `write` token scoped to repo A is somehow presented against repo B
(e.g., the operator points an MCP client at the wrong daemon endpoint
while still holding the A token). Expected (already in the daemon
layer; the test asserts the chat path inherits it):

- `authorize()` returns `decision=denied, denial_reason=
  capability_missing` (capability.py:92).
- Audit row appended with the scope mismatch recorded in
  `repository_id` vs. the token's scope.
- The chat client sees the structured `capability_missing` error
  string, identical to what an MCP client would see.

### 8.4 Operator-confirmation bypass attempt

A prompt-injected model fills `confirm_write: true` without the UI
gesture having fired. Expected:

- The UI never produces the gesture token; the chat-side dispatch
  cannot synthesize one because it is server-issued and bound to a
  specific `(chat_session_id, tool_call_id)`.
- The tool call is not forwarded to `execute_tool`.
- An audit row is appended with `decision=denied, denial_reason=
  operator_gesture_missing, transport=chat, operator_gesture=absent`,
  with `confirm_write: true` recorded in `params_hash` to capture the
  model's claim.

### 8.5 Mutation-not-allowed probe

On a service started without `--allow-mutations`, the model
constructs a `generate_workflow_write` call. Expected:

- The tool was not in `tools/list`; the model is operating from a
  guess.
- `execute_tool` returns `[error] mutations_disabled: ...`.
- Audit row appended with `denial_reason=mutations_disabled`.

### 8.6 Audit chain tamper via the chat path

The chat path attempts to write directly into the audit / request_log
tables (e.g., by smuggling SQL through a generated workflow's
metadata). Expected:

- No code path in `execute_tool` accepts SQL; the generator path
  writes only `workflow.json` files under controlled paths.
- The audit table is role-enforced append-only (daemon.py:225); even
  a successful path cannot rewrite history.
- The test asserts that after a successful chat-mediated write, the
  audit chain hash chain validates (existing
  `daemon_pg_audit_row_hash` check, audit.py:72).

## 9. Concrete Touch Points in `src/striatum/`

The work is small and surgical. The files that change in V1:

- **`src/striatum/skills/install.py`** — add `"mcp"` to
  `CLAUDE_CODE_SKILLS` (line 51-57). No other change; the generic and
  gemini concatenation templates pick it up via the existing renderer.
- **`src/striatum/skills/templates/claude_code/mcp.md.tmpl`** — new
  file. Body per §3.2.
- **`src/striatum/skills/templates/generic/mcp.md.tmpl`** — new file.
  Same body, shaped for inline concatenation.
- **`src/striatum/skills/templates/generic/STRIATUM_AGENT_GUIDE.md.tmpl`**
  — extend the concatenation list to include the new mcp section.
- **`src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`**
  — same append.
- **`src/striatum/web/chat_tools.py`** — extend `_TOOLS` (lines 49-143)
  with the two new entries; extend `execute_tool` (lines 189-221) with
  two new branches; add `_tool_generate_workflow_preview` and
  `_tool_generate_workflow_write` helpers near the existing
  `_tool_striatum_status` (lines 308-330).
- **`src/striatum/service.py`** — extend `_build_chat_briefing` (lines
  222-296) to mention the two new tools when `state.allow_mutations`;
  add the chat-tools filter that omits `generate_workflow_write` from
  the tools list when `--allow-mutations` is off; add the one-shot
  gesture-token machinery (small new helper near the chat session
  handlers around service.py:1819-1880).
- **`src/striatum/web/templates/chat_index.html` + `static/app.js`** —
  add the inline preview + confirmation button UX for the
  `generate_workflow_write` tool call interception. Reuse the existing
  `allow_mutations` cache (app.js:128) and the existing
  `Mutations are gated` template hint (chat_index.html:27).

The files that do **not** change in V1:

- `src/striatum/mcp.py` — the daemon-side MCP wrapper already does
  capability gating and `tools/list` filtering correctly
  (mcp.py:540-552, 554-640). The skill teaches its existing behavior;
  no new methods.
- `src/striatum/daemon_rpc/registry.py` — no new methods.
- `src/striatum/daemon_rpc/capability.py` — capability vocabulary is
  unchanged.
- `src/striatum/plugin/` — emitter picks up the new template
  automatically.
- `src/striatum/workflow_generator/` — the generator is reused
  verbatim; we do not modify its behavior or its envelope.

## 10. Operator UX

### 10.1 Token-lifecycle posture

The skill body teaches the recommended short-lived posture:

```bash
striatum daemon token create \
    --capability write \
    --expires-in 1h \
    --repo <repo_id> \
    --label "claude-code-workflow-author"
```

The skill body explains why short-lived: a leaked token's blast radius
is bounded by its TTL, and the operator can always issue a fresh one
without rotating long-lived credentials. The skill explicitly tells the
agent **not** to ask for `admin` or for unbounded TTLs.

### 10.2 Audit review

The skill points the operator to:

```bash
striatum daemon audit show --transport chat \
    --since 24h \
    --decision denied
```

(or the equivalent existing audit-query verb; the surface is the same
schema the daemon already exposes). The skill clarifies the
chat-specific fields the operator should look at: `transport=chat`,
`operator_gesture`, `denial_reason`, `params_hash` for the model's
claimed `confirm_write` value.

### 10.3 Revocation flow

The skill (and the operator-side `HOW_TO_HUMAN.md` section we add)
documents the revocation path:

```bash
striatum daemon token revoke --token-id <id>
```

Subsequent calls with that token produce `token_revoked` audit rows.
The chat UI does not see the revocation directly; the next call from
the chat session will surface the error. The operator can additionally
end the chat session if they want to stop the model from looping.

### 10.4 Deferred: per-workflow capability hints

`striatum daemon describe --workflow <path>` listing required
capabilities per workflow shape is **forward-looking** and not in V1.
We mention it in the RFC 0036 open-questions and in the synthesis
ledger so it doesn't get lost, but V1 ships without it.

## 11. Documentation Deltas

Updates land in the same PR(s) as the code changes; doc-link checks
must pass before merge:

- **`docs/MCP.md`** — new section "Mutation Surface for Agents" covering
  the preview-then-write idiom, the four denial vocab items, capability
  scope, the operator-gesture gate for the chat path, and the audit-row
  fields the operator should inspect.
- **`docs/HOW_TO_AGENT.md`** — extend the skill list to mention
  `striatum-mcp` alongside the existing five; one paragraph on when an
  agent should reach for it.
- **`docs/HOW_TO_HUMAN.md`** — operator-side: how to issue and revoke
  capability tokens for agents using the new chat tools; the
  short-lived-TTL recommendation; how to read `daemon audit show
  --transport chat`.
- **`docs/SPEC.md`** — extend the MCP surface section to mention the
  chat-path mutation gate and the `operator_gesture` audit field.
- **`docs/UBIQUITOUS_LANGUAGE.md`** — add or clarify entries for
  "effective tool set", "operator-confirmation gesture",
  "mutation-not-allowed path", "chat-path audit row".
- **`docs/CLI_REFERENCE.md`** — no new CLI verbs; cross-reference the
  new skill body for agents.
- **`docs/rfcs/0034-...`** — flip §10 "deferred" to "implemented in
  RFC 0036" with a link.
- **`docs/DECISION_LOG.md`** — add a new D-row capturing: "chat-path
  mutation writes flow through `--allow-mutations` + per-call
  `confirm_write` + per-call operator-gesture token; the chat model
  cannot bypass any of three; the audit row captures all three signals
  separately."
- **`CHANGELOG.md`** — `Unreleased` entry under "Added" for the
  `striatum-mcp` skill and the two chat tools; under "Changed" for the
  briefing extension; under "Security" for the audit-row gesture
  field.

## 12. Acceptance Criteria

The criteria are pulled forward from the RFC §Acceptance Criteria with
the additions from this design highlighted; the build job will be
judged against this list verbatim:

- [ ] `src/striatum/skills/templates/claude_code/mcp.md.tmpl` and
  `src/striatum/skills/templates/generic/mcp.md.tmpl` exist with body
  covering invoke triggers, authoritative reference, common patterns,
  capability scope, denial recovery, and what-not-to-do.
- [ ] `CLAUDE_CODE_SKILLS` tuple in `src/striatum/skills/install.py`
  includes `"mcp"`.
- [ ] `striatum skills install --profile claude_code` writes
  `.claude/skills/<ns>striatum-mcp/SKILL.md`.
- [ ] `striatum skills install --profile codex` writes
  `.codex/agents/<ns>mcp.md`.
- [ ] `striatum skills install --profile gemini` appends the mcp
  section to the gemini guide.
- [ ] `striatum skills install --profile generic` concatenates the mcp
  section into `STRIATUM_AGENT_GUIDE.md`.
- [ ] `striatum skills install --profile all` covers all four.
- [ ] `striatum plugin install` regenerates plugin bundles with the new
  skill body included for each first-class agent CLI.
- [ ] `src/striatum/web/chat_tools.py` exposes
  `generate_workflow_preview` and `generate_workflow_write` in
  `_TOOLS` and dispatches them in `execute_tool`.
- [ ] `_build_chat_briefing` mentions the two new tools, conditioned on
  `state.allow_mutations`.
- [ ] **`generate_workflow_write` is filtered out of the tools list
  surfaced to the chat session when `--allow-mutations` is off.**
- [ ] **`generate_workflow_write` enforces, in order:
  `--allow-mutations`, `confirm_write: true`, operator-gesture token.
  Each failure path produces a distinct structured error string and a
  distinct `denial_reason` in the audit row.**
- [ ] **The chat UI intercepts `generate_workflow_write` tool calls
  with a one-shot gesture token; the chat model cannot synthesize the
  token.**
- [ ] **Every chat-mediated write call appends an audit row with
  `transport=chat`, the new `operator_gesture` field, the
  `denial_reason` for failures, and a `params_hash` that includes the
  model's `confirm_write` claim.**
- [ ] Unit tests cover the six adversarial cases in §8.
- [ ] Doc-links pass.
- [ ] `docs/MCP.md`, `docs/HOW_TO_AGENT.md`, `docs/HOW_TO_HUMAN.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, `docs/SPEC.md`, RFC 0034 §10 status,
  `docs/DECISION_LOG.md`, and `CHANGELOG.md` updated.

## 13. Implementation Plan (Sequenced)

A four-step plan; each step is a stand-alone PR-sized unit:

**Step 1 — Skill body + install fan-out.**
Author `claude_code/mcp.md.tmpl` and `generic/mcp.md.tmpl`. Extend
`CLAUDE_CODE_SKILLS`. Extend the generic and gemini single-file
guides to include the new section. Unit test: `install --profile all`
emits the new skill at all four target paths with deterministic
content. Plugin regeneration test: `plugin install` for each provider
includes the new section.

**Step 2 — Chat tools (preview path).**
Extend `_TOOLS` and `execute_tool` with `generate_workflow_preview`
only. Wire to the existing `generate_workflow` function in the same
shape `_handle_workflow_generate` uses. Update `_build_chat_briefing`
to mention the preview tool. Unit tests: tool returns the
`GeneratedWorkflow` envelope; result is wrapped in BEGIN/END
delimiters; safe to call regardless of `--allow-mutations`.

**Step 3 — Chat tools (write path) + operator-gesture gate.**
Add `generate_workflow_write` to `_TOOLS` and `execute_tool`,
conditioned on `state.allow_mutations`. Implement the one-shot
gesture-token machinery in `service.py`. Implement the chat-UI
intercept in `chat_index.html` + `app.js`. Add the `operator_gesture`
audit-row field (DB migration; bump audit schema). Unit tests: the
six adversarial cases in §8.

**Step 4 — Documentation + RFC 0034 status flip.**
Update all docs listed in §11. Add the D-row to `docs/DECISION_LOG.md`.
Flip RFC 0034 §10 to "implemented in RFC 0036" with a link.
Doc-links pass.

## 14. Open Questions

Carried forward from RFC 0036, with this design's recommendations:

- **`daemon.describe` vs. `tools/list`** as the canonical discovery
  surface taught in the skill. **Recommendation: teach `tools/list`
  first.** It is honest about what the agent's token can actually call.
  Mention `daemon.describe` as the operator-side authoritative view.
- **Both-belts-and-suspenders on `confirm_write`** (model argument
  *and* UI gesture). **Recommendation: yes, keep both.** The model's
  argument captures intent in the audit row; the UI gesture captures
  the operator's act. Stripping either weakens the audit chain's
  ability to explain "who agreed to this write".
- **Skill name `striatum-mcp` vs. `striatum-daemon-tools` vs.
  `striatum-mutation-surface`.** **Recommendation: `striatum-mcp`.**
  Short, parallel to the existing names, and `mcp` is the vocabulary
  the agent already knows.
- **Bundled examples workflow.** **Recommendation: defer to a
  follow-up dogfood.** Keep V1 scoped to skill body + chat tools; an
  `examples/rfc-0036-...` fixture can land separately once the new
  tools are exercised in anger.

Additional open question this design surfaces:

- **Should `transport=chat` and `operator_gesture` be added via a new
  audit schema migration, or by repurposing existing fields?** This
  design assumes a new migration (additive, append-only column adds).
  The cost is the migration; the gain is unambiguous audit semantics
  for an operator reviewing chat-path activity. The build job should
  validate this decision against the existing daemon_pg migration
  layout in `src/striatum/daemon_pg/sql/`.

## 15. What Cannot Be Claimed Even After This Lands

The reviewer and the synthesis should hold these lines:

- **The chat-side audit is not a hosted-mode tamper-proof receipt.** It
  is a local hash chain over a local DB, same as the rest of the audit
  surface. We do not gain remote attestation by adding `transport=chat`.
- **Cross-machine multi-tenant chat semantics remain deferred
  indefinitely.** D083 stands. RFC 0036 is single-user, single-machine.
- **Malicious-local-root resistance remains out of scope.** RFC 0031's
  AI-guardrail framing is what we are inside. A local-root attacker can
  bypass `--allow-mutations`, forge gesture tokens, and rewrite the
  audit chain; we have nothing to claim against them and we do not
  pretend to.
- **Capability tokens are operator-only.** Nothing in RFC 0036 issues
  tokens. The skill body teaches what to ask for; the operator decides.
- **The chat model is still untrusted text.** The operator-gesture gate
  does not make the model "safer"; it makes the *write surface* safer
  from any prompt-injected model. The model itself can still emit
  arbitrary text inside the chat transcript. The audit row records
  what happened, not what the model "really meant".

## 16. Done Criteria for This Design

This design is done when the builder can:

1. Identify every file that changes (§9) and every file that
   deliberately does not change (§9).
2. Translate §3.2's skill body sections into rendered Markdown bodies
   per profile.
3. Implement `_tool_generate_workflow_preview` and
   `_tool_generate_workflow_write` from §4.3 without re-reading the
   RFC.
4. Implement the one-shot gesture-token machinery from §5.1 without
   inventing new audit-row fields beyond `operator_gesture`.
5. Land each of the six adversarial test cases (§8) as a runnable
   unit test.
6. Update each doc in §11 in one pass without surprises.

If the reviewer finds the design ambiguous against any of those six
checks, the design is wrong and a synthesis should send it back rather
than accept it as built-as-designed.

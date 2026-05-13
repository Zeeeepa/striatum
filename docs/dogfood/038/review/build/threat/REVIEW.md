---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0036", "mcp-harness", "chat-tools", "build"]
---

# RFC 0036 V1 — Threat-Model Review of Build

author: reviewer-claude-opus-001

## Scope

Fresh-context threat-model review of the RFC 0036 V1 implementation:
the `striatum-mcp` skill body across all install profiles, the two
new chat tools (`generate_workflow_preview`, `generate_workflow_write`),
the operator-confirmation gate, the mutation-not-allowed path, and
related documentation.

Per RFC 0031 § Threat Model and the work-packet brief, in scope:
defending against over-eager AI agents acting through documented
interfaces and operator-mistake footguns. Out of scope: a malicious
local-root operator who reads the daemon signing key, kills the
daemon, or impersonates the daemon process.

## Summary verdict

**accept_with_findings.** The core threat-model controls work:

- The web-chat write path queues a `tool_confirmation` JSONL entry
  and refuses to execute the underlying `write_generated_workflow`
  until a second HTTP gesture lands on
  `/chat/<session>/confirm-tool/<tool_id>` with a matching one-shot
  token. The chat model cannot mint that gesture; it has no way to
  POST to the confirm endpoint from inside the tool-call loop.
- `tool_schemas(allow_mutations=False, …)` and
  `tool_names(allow_mutations=False)` hide `generate_workflow_write`
  from the schema list returned to the provider; the closed-set
  dispatcher refuses unknown names with `[error] unknown tool`; and
  if a model fabricates `generate_workflow_write` anyway, the
  service-side confirmation queue and `_handle_chat_confirm_tool`
  both refuse with `mutations_disabled` when `--allow-mutations` is
  off.
- The skill body never says "trusted client", never recommends
  wildcard or admin grants, never teaches direct
  `.striatum/state.sqlite3` writes, and routes denial recovery
  through the documented `capability_missing` / `token_revoked` /
  `token_expired` / `method_unknown` / `mutations_disabled`
  vocabulary.
- Daemon-RPC `tools/call` retains the existing RFC 0032 V2
  capability gate and audit-row append (every allowed and denied
  mutating call). The method registry entries for
  `workflow.generate.preview` (`read`) and `workflow.generate`
  (`write`) are present and capability-scoped.
- Install fan-out covers Claude Code skill dir, Codex flat agent
  doc, Gemini single-file guide, generic concatenated guide, and
  all three first-class plugin bundles. Manifests are byte-stable;
  doctor surfaces drift.

The findings below are non-blocking but should be tracked because
they create a documentation-versus-implementation gap that the
threat-model review explicitly asked us to flag.

## Findings

### F1 (medium) — Chat-tool writes do not flow through the RFC 0032 V2 audit hash-chain

The work packet asks us to verify that "the chat tools use the
existing RFC 0032 V2 hash-chain append helper, not a separate path."
They do not.

`src/striatum/web/chat_tools.py:_tool_generate_workflow_preview` and
`_tool_generate_workflow_write` call `generate_workflow(spec)` and
`write_generated_workflow(generated, repo=repo)` in-process. The
chat-send loop in `src/striatum/service.py` (around lines 1960–2102)
routes every chat tool call into `execute_tool` and the
`_queue_chat_workflow_write_confirmation` /
`_handle_chat_confirm_tool` pair. None of these call
`daemon_rpc.request_log.append_audit_row`. The only audit trail for
chat-driven workflow writes is the per-session
`.striatum/scratch/chat-<id>/transcript.jsonl` (`tool_use` →
`tool_confirmation` → `tool_result` entries).

This is internally consistent if we read the chat surface as
"operator's own session, gated by `--allow-mutations` plus an
operator gesture, not by a capability token." But it conflicts with
two things:

1. The threat-model checklist in the work packet (and RFC 0036's
   skill-body promise: "Audit rows are appended for every mutating
   call including denials. The agent cannot suppress them; the
   operator inspects them via `daemon audit show` and the daemon
   DB."). A reader of either the prompt or the skill will expect to
   find chat-driven writes in `daemon audit show`. They will not.
2. `docs/UBIQUITOUS_LANGUAGE.md` line 181 frames the MCP mutation
   surface as the "agent-facing daemon/chat tool surface that
   exposes capability-gated methods through `tools/list` and
   `tools/call`." That sentence elides the trust-domain split:
   daemon MCP is capability-gated and audited centrally; chat is
   operator-trusted and audited in the chat transcript.

Threat exposure: an operator who reviews `daemon audit show` and
believes that gives them a complete view of mutating activity will
miss any chat-driven workflow writes. The data is recoverable from
chat transcripts, but the operator has to know to look.

**Recommendation (follow-up, non-blocking).** Either (a) extend
`_handle_chat_confirm_tool` and `_queue_chat_workflow_write_confirmation`
to call `append_audit_row` with `transport="chat"` (the audit row
shape already supports custom transports per `daemon_rpc/server.py`
line 111), or (b) document the split explicitly in `docs/MCP.md`
and `docs/UBIQUITOUS_LANGUAGE.md` and update the `striatum-mcp`
skill body to scope the audit-chain promise to MCP
`tools/call`-mediated writes only. Option (a) is the cleaner
posture and removes the documentation footgun.

### F2 (medium) — Daemon RPC registry advertises workflow generation methods that have no router

`src/striatum/daemon_rpc/registry.py` line 56–57 registers
`workflow.generate.preview` (read, `repository_scope=True`) and
`workflow.generate` (write, `repository_scope=True`). The
`striatum-mcp` skill body teaches agents to call exactly these via
`tools/call`. However, `src/striatum/daemon_rpc/server.py:CLI_ROUTES`
(lines 20–48) does not include either method, and `_route` ends in
`raise RpcError("method_unknown", "method has no handler: ...")` for
any registered method not in `CLI_ROUTES`.

Net effect: an MCP client with a valid `read` or `write` capability
that follows the skill's instructions will pass authorization, the
audit-row append will fire (an allowed row), and then `_route` will
raise `method_unknown` AFTER the audit row has already recorded the
call as "allowed". The agent receives a confusing error, the audit
log says the call was allowed but no work was done, and the skill's
"call `tools/list` first; it returns the effective tool set" advice
does not save them — `tools/list` will list these methods because
they are in the registry.

Threat exposure: this is not a security hole — it's a correctness
hole — but it creates two threat-model relevant surprises:

- An agent that "knows" `workflow.generate` is in the registry may
  loop on `method_unknown`, defeating the skill's "do not loop on
  denials" guidance. The audit log will record repeated allowed
  calls that produced no effect.
- An operator who reads the audit log will see successful-looking
  capability checks for write methods that never wrote anything,
  which is harder to reason about than either a clean denial or a
  clean success.

**Recommendation.** Either (a) wire a real handler for
`workflow.generate.preview` and `workflow.generate` into the daemon
RPC router that shares the generator with the chat path and emits a
clean audit row, or (b) remove the two entries from the method
registry and update the `striatum-mcp` skill body to teach only the
chat-tools path. Option (a) keeps RFC 0036's intent — an agent with
a capability token can drive workflow generation over MCP — and
restores the audit promise. Option (b) is honest about the V1
scope: the chat surface is the only working write path.

The current state straddles both and is the threat-model footgun.

### F3 (low) — TOCTOU on chat tool confirmation token

`_handle_chat_confirm_tool` in `src/striatum/service.py` (around
lines 2052–2106) reads the JSONL, validates the token via
`_find_pending_tool_confirmation` (which returns the pending entry
unless a later `used` entry exists), executes the write, and only
then appends a `state: "used"` marker. There is no lock or atomic
transition between the find and the execute.

Two concurrent confirms with the same token will both pass the
token check before either has marked the entry used, and both will
call `write_generated_workflow`. In practice this requires the
operator to either click the confirm button twice during a small
race window or to have a malicious local process forging requests
with the token. The local-process branch is partially in the
"malicious-local-root" out-of-scope bucket; the double-click branch
is the operator-mistake footgun the RFC explicitly wants to
defend.

Threat exposure: low. `write_generated_workflow` is largely
idempotent for the same spec; impact is duplicate file writes and
two `tool_result` entries against the same `tool_use_id`. The
chat-session JSONL preserves the evidence.

**Recommendation.** Persist the `used` marker before invoking the
write (or hold a per-session file lock around find+execute). Mirror
the design used elsewhere in the codebase for lease transitions.

### F4 (low) — Chat surface is fully gated by --allow-mutations; read-only chat is unreachable

`_handle_chat_new` (service.py line 1850) and `_handle_chat_send`
(service.py line 1886) both refuse with HTTP 405 when
`self.state.allow_mutations` is False. The chat index page
(`_render_chat_index_page`) still renders, but no chat session can
be created or driven.

RFC 0036 §2 describes the read-only chat tools as freely available
(`generate_workflow_preview` is described as "safe to call freely")
and only `generate_workflow_write` as gated. `_build_chat_briefing`
even has a branch for `allow_mutations=False` ("Workflow writing is
disabled for this service session…") that is currently unreachable
in practice because no chat session can be created in that mode.

This is not a security finding — if anything, it's overly
conservative — but it is a documentation-versus-implementation
mismatch. `docs/MCP.md` line 213 says
`generate_workflow_write` is "hidden unless the service was started
with `--allow-mutations`," which implies the rest of the chat tools
work without it. They do not.

Threat exposure: minimal. The operator who runs `striatum serve
--web` without `--allow-mutations` cannot drive the chat at all, so
no AI-agent-driven escalation is possible from a non-mutating
service. But operators may interpret the docs as promising a
read-only chat mode and be confused.

**Recommendation (doc honesty, non-blocking).** Either (a) gate
only `_handle_chat_send`-with-write-tool-call on `--allow-mutations`,
allowing read-only chat sessions when mutations are off, or (b)
update `docs/MCP.md`, `docs/HOW_TO_HUMAN.md`, and the chat briefing
to state plainly that the chat surface itself requires
`--allow-mutations`. Option (b) is the smaller change and matches
the current behavior.

### F5 (low) — Chat tools bypass the HTTP endpoints the RFC said they would call

RFC 0036 §5 states: "The chat tools call the existing RFC 0034 V1
endpoints (`POST /workflows/generate/preview`, `POST
/workflows/generate`)." The actual implementation in `chat_tools.py`
calls `generate_workflow()` and `write_generated_workflow()`
in-process. The behavioral checks (`allow_mutations`,
`confirm_write`, `operator_confirmed`) are duplicated between the
HTTP handler (`_handle_workflow_generate`) and the chat tool
(`_tool_generate_workflow_write`).

Threat exposure: the duplication risks drift. If a future change
adds an authentication gate, rate limit, audit row, or input
sanitization at the HTTP endpoint, the chat path will silently
skip it. Conversely, fixes to the chat path may not land on the
HTTP endpoint.

**Recommendation.** Either align the implementation to the RFC by
routing the chat tool through the HTTP endpoint via an in-process
HTTP client or shared service-layer function, or update the RFC's
§5 to say "the chat tools share the underlying generator and write
helper with the HTTP endpoints." Pick one source of truth and
mention the duplication explicitly.

### F6 (low) — Skill body's audit promise applies only to the MCP surface

The `striatum-mcp` skill body (in
`src/striatum/skills/templates/claude_code/mcp.md.tmpl`,
`generic/mcp.md.tmpl`, `gemini/STRIATUM_GEMINI_GUIDE.md.tmpl` MCP
section, and the three plugin-bundle copies) says:

> Mutating calls append metadata-only audit rows, including
> denials. Audit rows carry hashes and method metadata, not
> transcripts or raw generated workflow content.

This is correct for MCP `tools/call` once F2 is resolved. It is
NOT correct for chat-tool-mediated writes today (see F1). An
agent reading the skill body may believe its chat-tool writes are
captured in the same audit chain as MCP writes.

**Recommendation.** Tighten the wording to distinguish surfaces,
e.g., "MCP `tools/call` appends a metadata-only audit row to the
daemon DB for every mutating call (allowed or denied). Chat-tool
writes are captured in the chat session transcript instead." This
removes the surprise for an agent that switches between the two
surfaces.

### F7 (informational) — Cancel path on chat confirmation is implemented but untested

`_handle_chat_confirm_tool` supports `action=cancel` (line 2075–
2082), appending a `[mutation_canceled]` system message and marking
the pending entry used. The test suite covers the happy-path
confirm in
`tests/test_web_chat.py:test_chat_workflow_write_requires_operator_confirmation`
but not the cancel path or the token-mismatch path. The cancel
path is part of the operator-confirmation gate's promise that the
operator can refuse a write the model proposed.

**Recommendation.** Add unit/E2E coverage for `action=cancel`,
`token` mismatch, and re-use of a consumed token (the find-pending
helper returns None for used entries, but explicit coverage would
make the invariant durable).

### F8 (informational) — Adversarial bypass tests are minimal at the service level

Test coverage of the chat path is well-thought-out at the unit
level (`tests/test_chat_tools.py` covers each tool individually,
including `generate_workflow_write` refusing without
`allow_mutations`, without `confirm_write`, and without
`operator_confirmed`). The service-level E2E test covers one happy
path. The threat-model checklist asks for coverage of:

- Capability-scope mismatch refused with the documented vocabulary
  — covered for the MCP surface in RFC 0032 V2 tests; not exercised
  on the chat path (chat has no capability tokens).
- `tools/list` filter not including `generate_workflow_write` when
  `allow_mutations=False` — covered as a unit test
  (`test_tool_schemas_filter_write_by_mutation_gate`), not at the
  service level via the streaming provider mock.
- Crafted-call fallback when a model fabricates
  `generate_workflow_write` against a non-mutating service — not
  directly covered. The unit test asserts the dispatcher returns
  `mutations_disabled`, but no E2E asserts that an
  `allow_mutations=False` service refuses cleanly when a fake
  provider streams the write tool call.
- No-chat-model-identity-escalation — implicitly covered by the
  fact that the closed-set dispatcher only knows the eight tools in
  `_TOOLS` and refuses unknown names; not tested with a
  prompt-injected identity claim.

These are belt-and-suspenders gaps, not security holes. The design
makes the bypasses impossible (the closed-set dispatch, the filter,
and the confirmation gate are mechanical), but exercising them
explicitly would defend against regression.

**Recommendation.** Add three small E2E tests using
`FakeOpenAIWorkflowWriteHandler` as a template: (a) service
without `--allow-mutations` + fabricated write call → tool result
contains `mutations_disabled`; (b) `tools/list` filter at the
provider layer omits the write tool when not mutating; (c)
double-confirm with same token (F3) yields exactly one write.

### F9 (informational) — Skill body bullet "Do not treat audit as malicious-local-root-resistant proof" is the right framing

Explicitly calling out the malicious-local-root caveat in the skill
body is the right doctrine and matches `docs/SPEC.md` line 1132 and
`docs/HOW_TO_HUMAN.md` line 549. No change requested. Noting for
positive signal.

## Threat-model checklist (per work packet)

| Check | Status | Notes |
|---|---|---|
| Capability gating on every chat-tool route via RFC 0032 V2 | partial | Chat path uses `--allow-mutations` + operator gesture; no capability tokens. See F1. |
| Operator-confirmation gate reused (RFC 0013 step 7) | yes | `_queue_chat_workflow_write_confirmation` + `_handle_chat_confirm_tool` enforce a separate gesture; model cannot bypass. |
| Mutation-not-allowed path is hidden, not partial | yes | `tool_schemas(allow_mutations=False, …)` omits the write tool; fallback returns `mutations_disabled`. |
| Default-deny for unknown tool / missing capability | yes | Closed-set dispatcher refuses unknown names; daemon RPC returns `capability_missing` / `method_unknown` with audit row. |
| Audit row appended for every mutating chat-tool call including denials | partial | MCP surface: yes (existing RFC 0032 V2). Chat surface: appended to JSONL transcript only, not daemon DB. See F1. |
| No duplicate audit-append path | partial | MCP uses the existing helper; chat does not append to the daemon audit chain at all (so no duplication, but no append either). See F1. |
| Skill body teaches correct denial vocabulary + scope semantics | yes | All four templates (Claude Code, generic, Gemini, plugin bundles) include `capability_missing`, `token_revoked`, `token_expired`, `method_unknown`, `mutations_disabled`, and scope-mismatch guidance. |
| Skill body lacks "trusted client" framing and wildcard guidance | yes | Verified by inspection. The "what not to do" section explicitly refuses wildcard/admin requests for ordinary generation. |
| Skill body refuses direct `.striatum/state.sqlite3` writes | yes | Present in every template. |
| Skill install plan covers all three target paths | yes | `CLAUDE_CODE_SKILLS` includes `"mcp"`; codex and gemini plans render the same body; `_PROFILE_SKILLS` in plugin install matches. Tests in `test_skills_install.py` and `test_plugin_install.py` assert presence. |
| Plugin bundle regeneration includes new skill | yes | `tests/test_plugin_install.py` asserts `skills/striatum-mcp/SKILL.md` for claude_code, codex, gemini bundles. |
| System prompt briefing mentions the two new tools | yes | `_build_chat_briefing` in `service.py` lines 296–323 names `generate_workflow_preview` always and `generate_workflow_write` conditionally on `allow_mutations`. |
| Tests cover happy paths and adversarial bypasses | partial | Strong unit coverage; one E2E for confirmation. See F7, F8 for gaps. |
| Write scopes / fixtures do not normalize direct `.striatum/` edits or audit tampering | yes | No fixture or test in the build writes to `.striatum/state.sqlite3` directly or modifies audit rows. |
| Documentation honesty | partial | SPEC, MCP, UBIQUITOUS_LANGUAGE are mostly accurate but elide the chat-vs-MCP audit split. See F1, F4, F5, F6. CHANGELOG entry is honest. README delta not reviewed in detail. |

## Files inspected

- `src/striatum/web/chat_tools.py`
- `src/striatum/service.py` (chat lifecycle, confirmation queue,
  briefing builder)
- `src/striatum/skills/install.py` and
  `src/striatum/plugins/install.py`
- `src/striatum/daemon_rpc/registry.py` and
  `src/striatum/daemon_rpc/server.py`
- `src/striatum/skills/templates/claude_code/mcp.md.tmpl`
- `src/striatum/skills/templates/generic/mcp.md.tmpl`
- `src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`
- `src/striatum/plugins/templates/claude_code/skills/mcp.md.tmpl`
- `src/striatum/plugins/templates/codex/skills/mcp.md.tmpl`
- `src/striatum/plugins/templates/gemini/skills/mcp.md.tmpl`
- `tests/test_chat_tools.py`
- `tests/test_web_chat.py`
- `tests/test_skills_install.py`
- `tests/test_plugin_install.py`
- `docs/rfcs/0036-mcp-harness-for-daemon-v2-mutation-surface.md`
- `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
  `docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`, `CHANGELOG.md`

## Verdict rationale

The build correctly defends the threat surfaces RFC 0036 was
written to address: an over-eager AI agent that fabricates
`confirm_write: true` cannot trigger writes because the gesture
gate is enforced by a second HTTP request the model cannot
produce; an agent that fabricates a write-tool name against a
non-mutating service hits a closed-set refusal; the skill body
teaches narrow short-lived tokens and refuses identity-escalation
and direct-SQLite shortcuts.

The findings cluster around a single design choice — the chat
surface uses a different trust model and audit primitive than the
MCP surface — that is not surfaced clearly enough in the
documentation, the skill body, or the daemon RPC registry. F1, F2,
and F6 are the load-bearing items; resolving them (either by
wiring the MCP path and the chat audit append, or by documenting
the split explicitly) closes the threat-model honesty gap. F3, F4,
F5, F7, and F8 are smaller follow-ups.

None of the findings block the V1 ship. The build does not contain
"trusted client" framing, does not teach wildcard grants, does not
normalize direct SQLite writes, and does enforce the
operator-confirmation gate that the model cannot bypass. Verdict:
**accept_with_findings**.

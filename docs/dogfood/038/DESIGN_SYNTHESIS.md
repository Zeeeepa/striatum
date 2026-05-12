---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/038/design/codex/DESIGN.md", "docs/dogfood/038/design/claude_code/DESIGN.md", "docs/dogfood/038/design/gemini/DESIGN.md"]
---

author: designer-codex-gpt-5.5-001

# RFC 0036 MCP Harness Implementation Plan

Status: design synthesis
Date: 2026-05-12

## Accepted Implementation Scope

RFC 0036 V1 lands as harness over existing machinery: RFC 0015 skill install, RFC 0025 plugin bundles, RFC 0023 V1.5 chat tools, RFC 0034 workflow generation endpoints, and RFC 0032 daemon MCP capability/audit semantics. It does not add a new MCP server, capability vocabulary, workflow generator API, or hosted/remote surface.

| RFC 0036 acceptance criterion | Concrete plan | Owner module | Test owner |
|---|---|---|---|
| `claude_code/mcp.md.tmpl` and `generic/mcp.md.tmpl` exist with the required body. | Add two deterministic templates with the chosen six-section body below. The Claude template is the canonical body; the generic body is the same content shaped for the single guide. | `src/striatum/skills/templates/claude_code/mcp.md.tmpl`, `src/striatum/skills/templates/generic/mcp.md.tmpl` | `tests/test_skills_install.py` |
| `CLAUDE_CODE_SKILLS` includes `"mcp"`. | Append `"mcp"` to the existing tuple so Claude Code and Codex fan-out pick it up from the same source of truth. | `src/striatum/skills/install.py` | `tests/test_skills_install.py` |
| `skills install --profile claude_code` writes `.claude/skills/<ns>striatum-mcp/SKILL.md`. | Existing `_plan_claude_code` iterates `CLAUDE_CODE_SKILLS`; no new planner path beyond the tuple and template. | `src/striatum/skills/install.py` | `tests/test_skills_install.py` |
| `skills install --profile codex` writes `.codex/agents/<ns>mcp.md`. | Existing `_plan_codex` reuses Claude templates and the same tuple. Verify the default path is `.codex/agents/striatum-mcp.md`. | `src/striatum/skills/install.py` | `tests/test_skills_install.py` |
| `skills install --profile gemini` appends the MCP section. | Add the MCP section to the single Gemini guide template; do not depend on the tuple because Gemini currently renders one file. | `src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl` | `tests/test_skills_install.py` |
| `skills install --profile generic` includes the MCP section. | Add the MCP section to `generic/STRIATUM_AGENT_GUIDE.md.tmpl`; keep output single-file and deterministic. | `src/striatum/skills/templates/generic/STRIATUM_AGENT_GUIDE.md.tmpl` | `tests/test_skills_install.py` |
| `skills install --profile all` covers all profiles. | Extend the existing all-profile test to assert Claude, Codex, Gemini, and generic all contain the MCP body. | `src/striatum/skills/install.py` | `tests/test_skills_install.py` |
| `plugin install` regenerates bundles with the new skill body. | Add `mcp` to `_PROFILE_SKILLS` in plugin install and add per-profile plugin skill templates. Keep plugin paths explicit because plugin templates are separate package data. | `src/striatum/plugins/install.py`, `src/striatum/plugins/templates/*/skills/mcp.md.tmpl` | `tests/test_plugin_install.py` |
| Chat closed set includes `generate_workflow_preview` and `generate_workflow_write`. | Add both schemas to `src/striatum/web/chat_tools.py`. Preserve `read_file`/`list_dir`/`striatum_status`/`striatum_why`/`git_log`/`git_diff`/`list_workflows`. | `src/striatum/web/chat_tools.py` | `tests/test_chat_tools.py` |
| Chat briefing mentions both tools and preview-then-write. | Extend `_build_chat_briefing` to accept or consult mutation state, then mention preview always and write only when mutations are allowed. | `src/striatum/service.py` | `tests/test_web_chat.py` |
| `generate_workflow_write` enforces operator confirmation. | Reuse the existing service mutation posture and add a one-shot chat confirmation token bound to `(chat_session_id, tool_call_id, spec_hash)` before dispatching the write. Model `confirm_write: true` is required but not sufficient. | `src/striatum/service.py`, `src/striatum/web/templates/chat_index.html`, `src/striatum/web/static/app.js` | `tests/test_web_chat.py`, `tests/test_chat_tools.py` |
| Both tools respect `--allow-mutations`. | Preview is always visible and writes nothing. Write is hidden from model tool lists when `allow_mutations` is false; crafted calls return `mutations_disabled` and write nothing. | `src/striatum/web/chat_tools.py`, `src/striatum/service.py` | `tests/test_chat_tools.py`, `tests/test_web_chat.py` |
| Unit tests cover skill rendering, dispatch, confirmation, mutation-disabled behavior, audit, plugin regeneration. | Add focused unit coverage rather than a broad browser e2e fixture. | test files named in this table | all named test files |
| Doc links pass and required docs are updated. | Update docs listed in Documentation Deltas and run the existing doc-link test. | `docs/*`, `CHANGELOG.md` | `tests/test_doc_links.py` |

## Deferred Scope

| Deferred item | Why deferred | Lands in |
|---|---|---|
| `examples/` workflow exercising chat-generate end-to-end | The first slice should settle the skill body, chat tool contract, confirmation gate, and denial paths before adding a long-form fixture that may encode churn. | Follow-up dogfood after RFC 0036 V1 lands and the tools have stable names. |
| Operator-side `daemon describe --workflow` / required-capability explainer | Useful for operator ergonomics, but it is a new CLI/product surface and not required to make the harness safe. | Future RFC for daemon/operator diagnostics. |
| Per-chat-session capability tokens | V1 chat uses service-side `--allow-mutations` plus operator gesture; adding per-chat token issuance would reopen token lifecycle design. | Future RFC if chat becomes a primary mutation surface. |
| Web workflow chooser UI | RFC 0034 already deferred it separately; RFC 0036 only adds chat generation tools. | RFC 0034 follow-up. |

## `striatum-mcp` Skill Body

The skill name is `striatum-mcp`. The section order is fixed:

1. `When to invoke`
2. `Authoritative reference`
3. `Common patterns`
4. `Capability scope`
5. `Denial recovery`
6. `What not to do`

`When to invoke` says to use this skill when an agent has an operator-issued capability token and wants to use daemon MCP instead of CLI commands, wants to preview or write a generated workflow through the local API/chat path, or needs to recover from `capability_missing`, `token_revoked`, `token_expired`, `method_unknown`, or `mutations_disabled`.

`Authoritative reference` teaches `tools/list` first because it is the effective tool set for the current token. It then names `tools/call`, `daemon.hello`, `daemon.welcome`, and `daemon.describe`. `daemon.describe` is described as the broader operator-side method registry lookup, not as proof that the current token can call a method. The section states that `tools/call` re-authorizes every call and that mutating calls append audit rows including denials.

`Common patterns` uses JSON-RPC/MCP-shaped examples, not invented CLI verbs:

```json
{"method":"tools/list","params":{"token":"<capability-token>"}}
```

```json
{"method":"tools/call","params":{"name":"workflow.generate.preview","arguments":{"spec":{"schema_version":"striatum.workflow_generator.v1","shape":"code_change","lane_set":"author_reviewer","workflow_id":"my-change","name":"My change","workflow_version":"2026-05-12","branch":{"mode":"confirm","suggested_name":"striatum/my-change","allow_dirty":false},"scaffold_root":"workflows/my-change","artifact_root":"striatum/my-change","lanes":{"author":{"display_model":"Codex GPT-5.5","command":["codex","exec"]},"reviewer":{"display_model":"Claude Opus","command":["claude","--print"]}},"options":{"max_revision_cycles":1}}}}}
```

```json
{"method":"tools/call","params":{"name":"workflow.generate","arguments":{"spec":{"schema_version":"striatum.workflow_generator.v1","shape":"code_change","lane_set":"author_reviewer","workflow_id":"my-change","name":"My change","workflow_version":"2026-05-12","branch":{"mode":"confirm","suggested_name":"striatum/my-change","allow_dirty":false},"scaffold_root":"workflows/my-change","artifact_root":"striatum/my-change","lanes":{"author":{"display_model":"Codex GPT-5.5","command":["codex","exec"]},"reviewer":{"display_model":"Claude Opus","command":["claude","--print"]}},"options":{"max_revision_cycles":1}},"confirm_write":true}}}
```

The write example must say `confirm_write: true` is still not enough in web chat: the chat UI must also record an operator confirmation gesture.

`Capability scope` states that a repo-scoped write token for repo A cannot write repo B, that `tools/list` filtering is not a security boundary, and that `tools/call` is the real authorization point.

`Denial recovery` is a closed table:

| Denial | Agent response |
|---|---|
| `capability_missing` | Stop the attempted mutation and ask the operator for a short-lived token with the named capability and correct repository scope. |
| `token_revoked` | Stop retrying; treat revocation as an operator decision. |
| `token_expired` | Ask for a fresh token with the same narrow scope. |
| `method_unknown` | Re-read `tools/list`; if still absent, ask the operator to inspect `daemon.describe` for version skew. |
| `mutations_disabled` | Do not retry. The service was intentionally started without mutation authority; the operator must restart it with `--allow-mutations` if they want writes. |

`What not to do` forbids identity escalation, direct `.striatum/state.sqlite3` writes, wildcard/admin capability requests for ordinary generation, looping on `token_revoked`, and treating audit as malicious-local-root-resistant.

## Skill Install Plan Wiring

Add `"mcp"` to `CLAUDE_CODE_SKILLS` in `src/striatum/skills/install.py`. This is the chosen fan-out path for Claude Code and Codex.

Add `src/striatum/skills/templates/claude_code/mcp.md.tmpl` and `src/striatum/skills/templates/generic/mcp.md.tmpl`. Also update:

| Profile | Wiring |
|---|---|
| `claude_code` | Existing `_plan_claude_code` iterates `CLAUDE_CODE_SKILLS` and writes `.claude/skills/<namespace>mcp/SKILL.md`. |
| `codex` | Existing `_plan_codex` iterates `CLAUDE_CODE_SKILLS` and writes `.codex/agents/<namespace>mcp.md`. |
| `generic` | Append the MCP section to `generic/STRIATUM_AGENT_GUIDE.md.tmpl`; no multi-file generic output. |
| `gemini` | Append the MCP section to `gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`; Gemini remains a single-file guide. |

## Plugin Bundle Regeneration

Plugin install has its own `_PROFILE_SKILLS` tuple and separate package-data templates, so it must be updated explicitly. Add `"mcp"` to `_PROFILE_SKILLS` in `src/striatum/plugins/install.py` and add:

- `src/striatum/plugins/templates/claude_code/skills/mcp.md.tmpl`
- `src/striatum/plugins/templates/codex/skills/mcp.md.tmpl`
- `src/striatum/plugins/templates/gemini/skills/mcp.md.tmpl`

The plugin manifests do not need a new top-level capability field. Existing plugin emitters include every skill in `_PROFILE_SKILLS`, so `striatum plugin install --profile claude_code|codex|gemini` regenerates bundles with the new skill under `skills/<namespace>-mcp/SKILL.md`. Tests should assert dry-run output includes those paths and that `--profile all` covers all three bundle shapes.

## Chat Tool Schemas

The chat closed-set names are `generate_workflow_preview` and `generate_workflow_write`. The chat schema accepts `spec` as an object and delegates authoritative validation to `WorkflowGenerationSpec.from_json`; this avoids duplicating RFC 0034 validation in the chat layer.

`generate_workflow_preview` input schema:

```json
{
  "type": "object",
  "properties": {
    "spec": {
      "type": "object",
      "description": "WorkflowGenerationSpec JSON object with schema_version striatum.workflow_generator.v1."
    }
  },
  "required": ["spec"],
  "additionalProperties": false
}
```

`generate_workflow_preview` output schema:

```json
{
  "ok": true,
  "data": {
    "workflow": {},
    "files": [
      {"path": "workflows/my-change/workflow.json", "content": "{}\n"}
    ],
    "metadata": {
      "shape": "code_change",
      "lane_set": "author_reviewer",
      "lane_modifiers": [],
      "graph": {"nodes": [], "edges": [], "cycles": []},
      "catalog_templates": []
    },
    "warnings": [],
    "validation": {"ok": true}
  }
}
```

`generate_workflow_write` input schema:

```json
{
  "type": "object",
  "properties": {
    "spec": {
      "type": "object",
      "description": "WorkflowGenerationSpec JSON object with schema_version striatum.workflow_generator.v1."
    },
    "confirm_write": {
      "type": "boolean",
      "description": "Must be true. Necessary but not sufficient; the chat UI also requires an operator confirmation gesture."
    }
  },
  "required": ["spec", "confirm_write"],
  "additionalProperties": false
}
```

`generate_workflow_write` output schema:

```json
{
  "ok": true,
  "data": {
    "written": [
      {"path": "workflows/my-change/workflow.json", "status": "written"}
    ],
    "validation": {"ok": true}
  }
}
```

Errors preserve the service/generator envelope with `code`, `message`, and optional `field_path`, `hint`, and `ref`.

## Chat Tool Dispatch Wiring

Keep chat dispatch closed-set in `src/striatum/web/chat_tools.py`, but add a mutation-aware tool-list helper instead of exposing static lists unconditionally. The chosen shape is:

- Keep `_TOOLS` as the full internal registry.
- Add `tool_schemas(allow_mutations: bool, flavor: str) -> list[dict[str, Any]]`.
- Add `tool_names(allow_mutations: bool) -> frozenset[str]`.
- `generate_workflow_preview` is always included.
- `generate_workflow_write` is included only when `allow_mutations` is true.

Update `service.py` chat send to call `tool_schemas(self.state.allow_mutations, config.flavor)` rather than importing static `ANTHROPIC_TOOLS` / `OPENAI_TOOLS` directly. Keep the static constants for backward-compatible tests if useful, but service must use the filtered helper.

Dispatch helpers:

- `_tool_generate_workflow_preview(repo, spec_body)` calls `WorkflowGenerationSpec.from_json(spec_body)`, `generate_workflow(spec)`, and returns `{"ok": true, "data": generated.to_json()}` as compact JSON.
- `_tool_generate_workflow_write(repo, spec_body, confirm_write, *, allow_mutations, operator_confirmed)` enforces gates in this order: `allow_mutations`, `confirm_write is True`, `operator_confirmed is True`, then `write_generated_workflow(generate_workflow(spec), repo=repo)`.

Do not duplicate endpoint-specific write logic beyond calling the same generator/write functions used by `_handle_workflow_generate`. No new endpoint is needed.

## Operator-Confirmation Gate Reuse

The gate reuses the RFC 0013 step 7 posture: mutations require `--allow-mutations`, and the web UI is the operator gesture surface. For RFC 0036, add a one-shot confirmation token bound to the pending chat write:

```json
{
  "chat_session_id": "chat_...",
  "tool_call_id": "tool_...",
  "spec_hash": "sha256:...",
  "expires_at": "2026-05-12T08:00:00Z"
}
```

The model never sees this token. The UI intercepts `generate_workflow_write`, renders the preview/file list/validation summary, and only forwards the write call after the operator confirms. If the operator cancels, the model receives `mutation_canceled`. If a crafted request reaches the server without the token, dispatch returns `operator_gesture_missing`.

## Mutation-Not-Allowed Path Semantics

When `striatum serve` starts without `--allow-mutations`:

- `generate_workflow_preview` remains in tool lists and briefing.
- `generate_workflow_write` is hidden from tool lists and omitted from the briefing.
- A crafted or stale `generate_workflow_write` call returns:

```text
[error] mutations_disabled: service started without --allow-mutations; ask the operator to restart with --allow-mutations before writing workflows
```

This is a deliberate hide-first posture, mirroring RFC 0032 effective-tool-set filtering. A fallback refusal exists only for stale transcripts, provider retries, or crafted local requests.

## System Prompt Briefing Extension

Replace the fixed tool sentence in `_build_chat_briefing` with a mutation-aware sentence. The exact added text is:

```text
Workflow generation tools: generate_workflow_preview is safe to call freely and returns the generated workflow, files, graph metadata, warnings, and validation without writing files. When this service is started with --allow-mutations, generate_workflow_write may also be available; it writes generated workflow files only after generate_workflow_preview, confirm_write: true, and a separate operator confirmation gesture in the chat UI. The operator gesture is enforced by Striatum, not by you, and confirm_write: true is necessary but not sufficient.
```

When `allow_mutations` is false, the briefing must not name `generate_workflow_write`; it should say:

```text
Workflow writing is disabled for this service session because it was not started with --allow-mutations.
```

## Audit Wiring

Use the existing RFC 0032 daemon MCP audit/request-log helper for daemon-mediated mutating calls. Do not add a chat-only audit table or raw transcript persistence. For V1 chat service writes, append the same metadata-only audit shape at the gateway when the daemon audit helper is available; direct repo-local mode may record only the existing service refusal/response until daemon mode is active, and documentation must label that as advisory.

For `generate_workflow_write`, every allowed or denied call records:

| Field | Value |
|---|---|
| `transport` | `chat` |
| `method` | `workflow.generate` |
| `params_hash` | Canonical hash of `spec` plus `confirm_write`; never raw spec text in audit. |
| `decision` | `allowed` or `denied` |
| `denial_reason` | `mutations_disabled`, `confirm_write_missing`, `operator_gesture_missing`, `capability_missing`, `token_revoked`, `token_expired`, `method_unknown`, or null. |
| `repo_id` | Present when daemon/repository scope is known. |

The three designs disagree slightly on adding a durable `operator_gesture` audit column. The chosen V1 path is not to require a schema migration unless existing audit metadata cannot carry it. Prefer encoding the gesture state in the existing metadata/request-log payload if the helper supports extensible metadata. If it does not, the implementer may add an additive migration for `operator_gesture`, but that is an implementation necessity, not a product requirement.

Preview calls are read-only. They should be request-logged or metadata-audited when the infrastructure supports it so a write can be correlated to its preview, but RFC 0036's hard audit guarantee is for mutating write attempts including denials.

## Test Strategy

Skill install tests in `tests/test_skills_install.py`:

- Claude Code dry-run and real install include `.claude/skills/striatum-mcp/SKILL.md`.
- Codex dry-run and real install include `.codex/agents/striatum-mcp.md`.
- Gemini guide contains `MCP Mutation Surface`.
- Generic guide contains `MCP Mutation Surface`.
- `--profile all` covers all four.
- No external URL invariant still passes.
- Reinstall remains idempotent and modified-file refusal still works with the extra skill.

Plugin tests in `tests/test_plugin_install.py`:

- Claude Code, Codex, and Gemini dry-runs include `skills/striatum-mcp/SKILL.md`.
- Manifest includes the MCP template entry.
- Plugin reinstall remains idempotent.

Chat registry tests in `tests/test_chat_tools.py`:

- Tool names/schemas include preview.
- Tool names/schemas include write only when `allow_mutations=True`.
- Unknown tool still refuses closed-set.
- Anthropic and OpenAI schema adapters expose the same filtered set.

Chat dispatch tests in `tests/test_chat_tools.py` or `tests/test_service.py`:

- Preview dispatch returns a `GeneratedWorkflow` envelope and writes no files.
- Preview preserves `field_path`, `hint`, and `ref` generator errors.
- Write refuses `confirm_write` missing/false.
- Write refuses `operator_gesture_missing` even with `confirm_write: true`.
- Write with all gates calls the same write helper as `POST /workflows/generate` and returns written paths.
- Mutation-disabled write returns `mutations_disabled` and creates no files.

Operator-confirmation tests in `tests/test_web_chat.py`:

- A model-emitted write call is intercepted before execution.
- The server accepts only a valid one-shot confirmation token bound to the matching session/tool call/spec hash.
- Expired, mismatched, reused, or absent gesture tokens refuse.
- Operator cancel returns `mutation_canceled`.

Adversarial tests:

- Hostile chat client enumerates tools without mutations: write is absent; crafted write is denied.
- Prompt-injected model claims trusted identity: no effect without UI gesture.
- Repo-scope mismatch through daemon capability layer returns `capability_missing` and records denial when daemon mode is active.
- Expired/revoked token paths continue to surface `token_expired` / `token_revoked`.
- Audit tamper attempt via generated workflow content cannot affect audit rows because only hashes/metadata are recorded.

Audit tests in `tests/test_mcp_mutation_capabilities.py` or daemon audit tests:

- Allowed chat write appends `transport=chat`, `method=workflow.generate`, `decision=allowed`.
- Denied chat write appends `transport=chat`, `decision=denied`, and the exact denial reason.
- Audit row stores `params_hash`, not raw spec or transcript content.

Documentation and regression tests:

- `tests/test_doc_links.py` passes.
- Existing service workflow-generator tests still pass unchanged for REST endpoints.
- `make lint`, `make typecheck`, and `make test` are the expected final verification set.

## Documentation Deltas

| File | Delta |
|---|---|
| `docs/MCP.md` | Add "Mutation Surface for Agents": effective tool set, preview-then-write, token scope, denial recovery, short-lived tokens, audit expectations, and `mutations_disabled`. |
| `docs/HOW_TO_AGENT.md` | Add `striatum-mcp` to the skill list and say agents should use it for daemon MCP/tool-call mutation instead of raw CLI driving. |
| `docs/HOW_TO_HUMAN.md` | Document issuing, scoping, expiring, and revoking capability tokens; document restarting `serve` with `--allow-mutations` for chat writes. |
| `docs/SPEC.md` | Record RFC 0036 as the implemented chat-assisted workflow-generation harness over RFC 0034 endpoints; keep local-first/no-hosted boundary. |
| `docs/UBIQUITOUS_LANGUAGE.md` | Clarify or add MCP mutation surface, effective tool set, operator-confirmed chat mutation, and mutation-not-allowed path. |
| `docs/CLI_REFERENCE.md` | No new CLI verbs; cross-reference the new skill for MCP mutation usage. |
| `docs/rfcs/0034-workflow-generator-and-template-catalog.md` | Update §10 chat-assisted scaffolding from deferred to implemented by RFC 0036. |
| `docs/rfcs/0036-mcp-harness-for-daemon-v2-mutation-surface.md` | Flip status from proposed to accepted/implemented after build and review land. |
| `docs/DECISION_LOG.md` | Add a D-row only if the implementer needs to make a product decision beyond this synthesis, such as a new audit column. |
| `CHANGELOG.md` | Add entries for the MCP skill, chat preview/write tools, mutation-disabled filtering, and audit/confirmation behavior. |

## Staging Plan

V1 lands in this order:

1. Skill body first: add templates, tuple wiring, single-file guide appends, and skill install tests.
2. Chat tool wiring: add preview/write schemas and preview dispatch first, then write dispatch.
3. Mutation-not-allowed path: filter write from tool lists/briefing, add crafted-call refusal, and wire operator confirmation.
4. Docs: update MCP, HOW_TO_AGENT, HOW_TO_HUMAN, SPEC, UBIQUITOUS_LANGUAGE, CLI_REFERENCE, RFC 0034, RFC 0036, and CHANGELOG.
5. Plugin regeneration: add plugin MCP templates and `_PROFILE_SKILLS` update, then verify all plugin profiles.

Deferred after V1:

- `examples/` chat-generate workflow fixture.
- Operator-side capability explainer such as `daemon describe --workflow`.
- Per-chat capability-token issuance.
- Web workflow chooser UI.

## Human-Decision Questions

The implementer can proceed without new human decisions on skill name, section order, chat tool names, schema shape, mutation-disabled semantics, staging order, or the `examples/` deferral.

One implementation decision may require human/operator confirmation if the existing daemon audit helper cannot carry `operator_gesture` metadata without schema changes: either encode gesture state in existing metadata or add an additive audit migration. The synthesis recommends using existing metadata first and adding a migration only if needed for queryability.

The audit guarantee should be labeled carefully: mutating daemon-mediated chat calls are metadata-audited, including denials. Direct repo-local service mode can only provide advisory/local service evidence unless routed through daemon audit infrastructure.

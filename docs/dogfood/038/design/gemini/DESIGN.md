# Implementation Design: RFC 0036 MCP Harness

**Author:** designer-gemini-pro-001
**Status:** Draft
**Date:** 2026-05-12
**RFC Reference:** [RFC 0036: MCP Harness for Daemon V2 Mutation Surface](../../../rfcs/0036-mcp-harness-for-daemon-v2-mutation-surface.md)

## Overview

This document defines the implementation details for the MCP harness as proposed in RFC 0036. It covers the cross-platform integration of the `striatum-mcp` skill, the wiring of new chat tools for workflow generation, and the security posture against adversarial attempts.

## 1. Skill Body Discoverability & Plugin Regeneration

The `striatum-mcp` skill serves as the primary educational artifact for AI agents to discover and use the daemon-mediated MCP surface.

### 1.1 Skill Templates

New templates will be added to the packaged skill set. The skill name will be consistently `striatum-mcp`.

- `src/striatum/skills/templates/claude_code/mcp.md.tmpl`: The authoritative Markdown guide for Claude Code and Codex.
- `src/striatum/skills/templates/generic/mcp.md.tmpl`: The concatenated guide for generic agents.
- `src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`: A new `## MCP Mutation Surface` section will be appended before the `## What not to do` section.

### 1.2 Registration & Install Pipeline

Update `src/striatum/skills/install.py`:
```python
CLAUDE_CODE_SKILLS: tuple[str, ...] = (
    "workflow",
    "scaffold",
    "claim-loop",
    "supervise",
    "recover",
    "mcp",  # Add mcp here
)
```

The `install()` pipeline in `striatum.skills.install` and `striatum.plugins.install` will automatically pick up the new skill and fan it out to:
- `.claude/skills/striatum-mcp/SKILL.md`
- `.codex/agents/striatum-mcp.md`
- `STRIATUM_GEMINI_GUIDE.md` (appended)

### 1.3 Plugin Regeneration Semantics

When `striatum plugin install` is run, the plugin manifests (`plugin.json`, `gemini-extension.json`) will be regenerated. The `mcp` skill will be included in the `skills/` directory of the bundle. For Gemini, it remains part of the single `GEMINI.md` context file as defined in the extension manifest.

## 2. Chat Tool Dispatch Wiring

The RFC 0023 V1.5 chat-tools framework will be extended with two new tools: `generate_workflow_preview` and `generate_workflow_write`.

### 2.1 Tool Schemas (`src/striatum/web/chat_tools.py`)

```python
_TOOLS.extend([
    {
        "name": "generate_workflow_preview",
        "description": "Preview a workflow generation from a spec. Read-only.",
        "parameters": {
            "type": "object",
            "properties": {
                "spec": {"type": "object", "description": "WorkflowGenerationSpec as defined in RFC 0034."}
            },
            "required": ["spec"],
        },
    },
    {
        "name": "generate_workflow_write",
        "description": "Write a generated workflow to disk. Requires operator confirmation.",
        "parameters": {
            "type": "object",
            "properties": {
                "spec": {"type": "object", "description": "WorkflowGenerationSpec."},
                "confirm_write": {"type": "boolean", "description": "Must be true to proceed."}
            },
            "required": ["spec", "confirm_write"],
        },
    }
])
```

### 2.2 Dispatch Logic

Update `execute_tool()` to handle these by calling the existing local API logic or `striatum.api.invoke` counterparts.
- `generate_workflow_preview` -> Calls `POST /workflows/generate/preview` logic.
- `generate_workflow_write` -> Calls `POST /workflows/generate` logic.

### 2.3 Mutation Gate

The `generate_workflow_write` tool will be hidden from `tools/list` and its execution refused unless `striatum serve` is started with `--allow-mutations`. The UI confirmation gesture (RFC 0023 V1.5) remains the authoritative gate for the write operation.

## 3. Security & Adversarial Test Cases

The MCP harness relies on the "Sealed Apply Boundary" (RFC 0031) and "Capability Gating" (RFC 0032).

### 3.1 Hostile Enumeration (`tools/list`)
- **Attack:** A hostile client calls `tools/list` to discover sensitive mutation tools it shouldn't have access to.
- **Defense:** `DaemonRpcServer.daemon_tool_specs` in `src/striatum/mcp.py` filters the tool set based on the token's authorized capabilities.
- **Test:** Use a `read`-only token and verify `workflow.generate` is absent from the response.

### 3.2 Token Expiry & Revocation
- **Attack:** Replaying a previously valid but now expired/revoked token.
- **Defense:** `authorize()` check in the daemon validates token state against the DB.
- **Test:** Manually expire/revoke a token in the test DB and verify `tools/call` returns `token_expired` / `token_revoked`.

### 3.3 Scope Mismatch
- **Attack:** A client with a token scoped to `repo_A` attempts to call `workflow.generate` against `repo_B`.
- **Defense:** The `authorize()` call passes the `repository_id` from the request. If the token's scope doesn't match, `capability_missing` is returned.
- **Test:** Issue a token for `repo_A`, call a mutating method with `repository_id="repo_B"`, verify denial and audit row recording the mismatch.

### 3.4 Operator-Confirmation Bypass
- **Attack:** Client calls `generate_workflow_write` with `confirm_write: false` or attempts to bypass the chat UI gesture.
- **Defense:** 
    1. The API endpoint itself refuses if `confirm_write` is not true.
    2. The Chat UI only allows the `generate_workflow_write` tool call to proceed after an explicit operator gesture.
- **Test:** Attempt an MCP `tools/call` with `confirm_write: false` and verify refusal.

### 3.5 Audit Chain Tamper
- **Attack:** Attempting to mutate state without leaving an audit trail.
- **Defense:** `append_audit_row` is called within the `call_daemon_tool` path *before* any action is taken. The ID returned by `append_audit_row` is part of the tool response.
- **Test:** Verify every `tools/call` (allowed or denied) results in a new row in the `audit_log` table with the correct `request_id`.

## 4. Cross-platform vs. Platform-specific

| Feature | Cross-platform | Platform-specific |
|---|---|---|
| **Skill Content** | `mcp.md.tmpl` prose is neutral. | Paths (`.claude/` vs `.codex/`). |
| **Tool Discovery** | `tools/list` filtered by capabilities. | Manifest registration (`plugin.json`). |
| **Chat Tools** | Schema definition and dispatch logic. | Adaptation to Anthropic/OpenAI shapes. |
| **Security** | Capability checks and Audit logging. | N/A (Daemon is the single source of truth). |

## 5. Implementation Steps

1. **Step 1:** Add `striatum-mcp` skill templates and wire into `install.py`.
2. **Step 2:** Implement `generate_workflow_preview` and `generate_workflow_write` in `chat_tools.py`.
3. **Step 3:** Add adversarial test cases in `tests/test_mcp_harness_adversarial.py`.
4. **Step 4:** Update documentation (`docs/MCP.md`, `docs/HOW_TO_AGENT.md`, etc.).

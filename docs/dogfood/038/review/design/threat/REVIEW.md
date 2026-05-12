---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0036", "mcp-harness", "chat-tools"]
---

author: reviewer-gemini-pro-001

# Threat-Model Review: RFC 0036 MCP Harness Design

Status: accept
Date: 2026-05-12
Posture: threat_model

## Enclosure

This review covers the synthesized implementation plan for RFC 0036 (MCP Harness for Daemon V2 Mutation Surface) as documented in `docs/dogfood/038/DESIGN_SYNTHESIS.md`.

## Trust Boundaries and Attack Surfaces

The design introduces the following trust boundaries and attack surfaces:

1.  **MCP Tool Surface (Daemon-Mediated):** The primary interface for agents holding capability tokens. The daemon acts as the policy enforcement point (PEP) for all `tools/call` and `tools/list` requests.
2.  **Chat Tool Surface (Service-Mediated):** A closed set of tools exposed to the web-based chat interface. This surface introduces a new path for mutations (`generate_workflow_write`) that bypasses direct daemon capability tokens but is gated by service-level flags and operator gestures.
3.  **Operator Confirmation Gate:** A mechanism to ensure that no mutation is performed by the chat model without an explicit human action. This is the primary defense against over-eager AI agents in the chat context.
4.  **Visibility Boundary:** The filtering of available tools based on session context (capabilities for MCP, `--allow-mutations` for chat).

## Threat Model Compliance (RFC 0031 §Threat Model)

The design successfully addresses the in-scope threats defined in RFC 0031:

-   **Over-eager AI Agents:** Defended via mandatory capability gating, operator confirmation gestures for chat mutations, and clear "What not to do" instructions in the `striatum-mcp` skill.
-   **Operator-mistake Footguns:** Mitigated by repository-scoped tokens, short-lived token recommendations, and the requirement for explicit `confirm_write: true` plus UI gestures.
-   **Malicious Local-Root:** Appropriately identified as OUT OF SCOPE. The audit chain and signing keys are acknowledged as non-resistant to an operator with root access or code execution as the daemon user.

## Verification of Mandatory Security Controls

| Control Requirement | Verification Status | Design Evidence |
| :--- | :--- | :--- |
| **Capability gating on every chat tool route** | **PASS** | `generate_workflow_write` is gated by `allow_mutations` and operator confirmation. |
| **Operator-confirmation gate bypass prevention** | **PASS** | Enforced by a server-verified one-shot confirmation token bound to `(chat_session_id, tool_call_id, spec_hash)`. |
| **Audit row for every mutating call (incl. denials)** | **PASS** | Every allowed or denied `workflow.generate` call records `transport=chat`, `method`, `params_hash`, `decision`, and `denial_reason`. |
| **Visibility filtering (Hide vs. Refuse)** | **PASS** | `tools/list` filters tools based on token capabilities. `generate_workflow_write` is hidden from chat tool lists when `allow_mutations` is false. |
| **Default-deny enforcement** | **PASS** | Missing capabilities or expired/revoked tokens result in `capability_missing`, `token_expired`, or `token_revoked` denials. |
| **No global mutation flag bypass** | **PASS** | `generate_workflow_write` respects the `--allow-mutations` flag on `striatum serve`. |
| **No identity escalation** | **PASS** | The skill explicitly forbids identity escalation; the daemon enforces gating regardless of claimed identity. |
| **Capability scope mismatch refusal** | **PASS** | Repo-scoped tokens are refused when used against a different repository ID, with documented denial vocabulary. |

## Findings and Observations

### 1. "Trusted Chat Client" Framing Avoidance
The design avoids the "trusted chat client" overclaim. It acknowledges that while the UI enforces the operator gesture, the *server* also verifies a one-shot gesture token. If a crafted request reaches the server without this token, it is refused with `operator_gesture_missing`. This ensures that a compromised or malicious chat client (or a prompt-injected model attempting to spoof the client) cannot trigger mutations directly.

### 2. Audit Row Integrity
The use of `params_hash` in audit rows instead of raw specification text prevents audit-log-injection attacks where a model might attempt to mask its actions by writing misleading content into the audit log.

### 3. Visibility Filtering as UX, not Security
The design correctly identifies that `tools/list` filtering is a UX/ergonomics feature, not a security boundary. The real authorization point remains `tools/call`, which re-authorizes every invocation. This distinction is critical for maintaining a robust security posture.

## Verdict

**VERDICT: ACCEPT**
**SEVERITY: NONE**

The RFC 0036 implementation plan as synthesized for Dogfood 038 is compliant with the project's threat model and security standards. It provides a multi-layered defense (capability gating, operator gestures, visibility filtering, and auditing) that effectively mitigates the risk of unauthorized or accidental mutations by AI agents.

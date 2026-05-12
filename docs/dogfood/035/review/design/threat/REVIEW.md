---
artifact_kind: finding
schema_version: striatum.finding.v1
verdict: accept
logical_name: design_review_threat
author: reviewer-gemini-pro-001
---
# Threat-Model Review: RFC 0032 Cross-Repo and MCP Mutation

**Author:** reviewer-gemini-pro-001
**Date:** 2026-05-12
**Status:** Finding
**Logical Name:** design_review_threat

## Executive Summary

RFC 0032 introduces cross-repository workflow coordination and state-mutating MCP tools to the Striatum daemon. This review evaluates the design against seven specific trust boundaries and attack surfaces. The design leverages the RPC capability model (RFC 0030) and daemon-owned supervision (RFC 0031) to establish a robust "default-deny" posture. All identified boundaries are acknowledged and mitigated, provided the implementation adheres to the specified authorization and reconciliation logic.

## Trust Boundary Analysis

### 1. Prompt-Injected MCP Client (Elevated Mutation)
*   **Threat:** A prompt-injected agent attempts to call restricted tools (e.g., `daemon.shutdown`, `apply.reviewed_patch`).
*   **Mitigation:** Gated by explicit capability tokens. MCP tools map 1:1 to Daemon RPC methods. The `tools/list` method is filtered to only show tools for which the client holds a capability. `tools/call` enforces `authorize()` checks before execution.
*   **Verdict:** **Acknowledged & Mitigated.** The capability model is the primary defense against over-eager AI agents.

### 2. Capability Token Leaked Across Repos
*   **Threat:** A token granted for Repository A is used to mutate Repository B.
*   **Mitigation:** The `client_capabilities` schema supports an optional `repository_id` scope. The `authorize()` logic (in `src/striatum/daemon_rpc/capability.py`) strictly matches the requested `repository_id` against the token's scope.
*   **Verdict:** **Acknowledged & Mitigated.** Repo-scoping prevents lateral movement between repositories using leaked tokens unless the token was explicitly granted global (`daemon`) scope.

### 3. Daemon Crash Mid-Cross-Repo Run
*   **Threat:** Daemon crashes during multi-repo state updates, leaving repos in inconsistent states.
*   **Mitigation:** Two-phase commit semantics managed by the daemon. The daemon DB records the "preparing" run state before updating local repo SQLite files. Daemon startup includes a reconciliation step to either "complete" or "roll back" pending transitions.
*   **Verdict:** **Acknowledged & Mitigated.** While distributed transactions across separate SQLite files are best-effort, the reconciliation logic ensures eventual consistency for workflow coordination.

### 4. Repository Unregistered Mid-Run
*   **Threat:** A repository is removed while a cross-repo run is actively targeting it.
*   **Mitigation:** The design recommends pausing the run with a human checkpoint and refusing further job advancement until the repository is re-registered or the run is canceled.
*   **Verdict:** **Acknowledged & Mitigated.** This prevents undefined behavior or opaque failures during repo removal.

### 5. Per-Repo Write-Scope Bypass
*   **Threat:** Job A (originating from Repo A) targets Repo B and attempts to write outside Repo B's allowed paths.
*   **Mitigation:** `write_scope.allowed_paths` is relative to the target repository. The daemon-owned supervisor (RFC 0031) is initialized with the root of the target repository, ensuring file-system enforcement remains scoped to the correct repo.
*   **Verdict:** **Acknowledged & Mitigated.** Job target repository determines the root of the enforcement boundary.

### 6. Audit Logging (Mutations and Denials)
*   **Threat:** Unauthorized tool calls or malicious attempts are hidden from operators.
*   **Mitigation:** Every MCP `tools/call` (mapping to an RPC call) records an audit row in the daemon DB, regardless of the authorization outcome. Denials are recorded with a `denial_reason: capability_missing`.
*   **Verdict:** **Acknowledged & Mitigated.** The centralized daemon audit log (introduced in RFC 0030/0033) provides a single source of truth for all cross-repo and MCP activity.

### 7. Operator-Mistake Footguns (Wildcard Grants)
*   **Threat:** An operator accidentally grants a global `write` capability (`scope: "daemon"`) to an untrusted agent.
*   **Mitigation:** No bulk-grant capability exists. Wildcard grants are explicit (NULL `repository_id`). Tokens default to `read` only. `apply` capability is never granted by default.
*   **Verdict:** **Acknowledged.** While wildcard grants are a necessary feature for coordinators, the design mitigates risk through conservative defaults and explicit grant UX.

## Conclusion

The RFC 0032 design successfully extends Striatum's security model to multi-repository environments. The concentration of authority in the daemon is balanced by the explicit capability gating and append-only audit logging. The "AI-guardrail" posture is maintained even as mutation capabilities are exposed to agents.

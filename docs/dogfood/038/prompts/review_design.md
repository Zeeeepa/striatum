# Review Design Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0036", "mcp-harness", "chat-tools"]
---
```

Review `docs/dogfood/038/DESIGN_SYNTHESIS.md` under the **threat_model** posture. Use only an accepting verdict (`accept` or `accept_with_findings`) if the plan enumerates the trust boundaries and attack surfaces the MCP harness introduces, and each is either acknowledged or mitigated.

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads the daemon's signing key, kills the daemon, or impersonates the daemon process.

In scope for this review:

- **Capability gating on every chat tool route**: does the synthesis specify that both new chat tools flow through the existing RFC 0032 V2 capability layer? That `generate_workflow_write` refuses missing-capability requests? That every mutating call produces an audit row including denials?
- **Operator-confirmation gate**: does the synthesis specify that the RFC 0013 step 7 gate is reused (not duplicated)? That the chat model passing `confirm_write: true` is necessary but not sufficient — the UI gesture is the second factor? That a prompt-injected model cannot bypass the UI?
- **Mutation-not-allowed path**: does the synthesis specify that write tools are hidden from `tools/list` rather than emitting partial-information errors? Is the fallback (model dispatches anyway) refused cleanly with `mutations_disabled` + audit?
- **Default-deny**: `tools/list` is capability-filtered; `tools/call` refuses missing-capability; unknown tool returns not-found + audit `denial_reason=tool_unknown`.
- **Capability scope enforcement**: a `write` token scoped to repo A refused against repo B with `capability_missing`; audit row records the scope mismatch.
- **Audit chain integrity**: the chat-tool audit row uses the same hash chain helper as the RPC audit rows; no duplicate audit-append path; denials recorded with documented vocabulary.
- **Prompt-injection mitigation**: capability tokens are the only access path; short-lived tokens for mutation; operator-controlled revocation; operator-UX for inspecting the audit chain.
- **Provenance overclaim**: does the synthesis claim cross-machine semantics? Tamper-proof receipts? Cryptographic non-repudiation? Malicious-local-root resistance? Any of those would be overclaim per RFC 0031 threat model.
- **Scope discipline**: does the synthesis stay inside RFC 0036 V1 scope, or wander into hosted mode, multi-tenant, or new capability vocabulary?
- **"Trusted client" framing**: any text that implies the daemon trusts the chat model's identity claim should be flagged. The daemon trusts the token, not the model.
- **Wildcard capability grants**: the skill body must NOT suggest "give me admin" or "give me all capabilities". Flag any such text.

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. For `accept_with_findings`, the findings must be non-blocking and explicitly say so.

**IMPORTANT — write the REVIEW.md artifact directly in this invocation.** Per the dogfood-036 OPERATOR_REPORT.md intervention #2, a previous gemini design-review session surfaced a strategy summary and then asked the operator "should I proceed with drafting the formal REVIEW.md artifact?" and exited without producing the file. Do not repeat that pattern. The work packet's `expected_artifacts` requires the file on disk at `docs/dogfood/038/review/design/threat/REVIEW.md`; the operator is not on a back-and-forth chat with you — you are inside a supervised wrapper that runs `gemini --prompt -` once per packet, and there is no follow-up turn. Write the file with front matter + byline + verdict line + reasoning in this single invocation.

Stay inside the review write scope (`docs/dogfood/038/review/design/threat/`). Do not modify the synthesis. Do not call striatum CLI; the operator publishes otherwise.

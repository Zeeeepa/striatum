---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["f42", "design-review", "threat_model"]
---

author: operator

# Threat-Model Review

Verdict: accept_with_findings.

The synthesis draws the right primary boundary: the supervised turn-driver, not the child generator, is the autonomous MCP client; the child receives only topic plus ordered transcript; output can only become `conversation.say(body)`. That is a meaningful, testable distinction from packet spoon-feeding. A future change would need to widen `ConversationContext`, alter prompt rendering, or route output into generic MCP calls to regress into a proxy, so the current design is not just relying on comments.

Finding 1 (medium): the design overclaims credential isolation for the child generator. Section 4 says the child "cannot reach the daemon even if it tried" because `STRIATUM_*` credentials are scrubbed. The interrogation clarified that this is too strong: a co-located child may still discover the daemon socket or token cache outside the scrubbed environment. That residual is not the packet-spoon-feeding hazard, but it is attestation-adjacent and matters because the supervisor surface is justified on attestation parity.

Required resolution: revise the design/decision record to acknowledge this as an accepted v1 attestation risk, or add a concrete non-discoverability guard. The minimum acceptable v1 guard is that the turn-driver must not newly materialize any token or token file in the child generator's view, with a unit test alongside the `STRIATUM_*` env-scrub test. Deeper isolation such as memory-only token topology for all driven lanes or per-lane socket namespaces can remain follow-up platform work if the risk is explicitly recorded.

Threat surfaces reviewed:

- Packet proxy drift: mitigated by typed `ConversationContext{Topic, Transcript}` and by never routing generator output into generic MCP/tool calls.
- Misconfigured self-driving adapter: bounded; it would be used wastefully as a one-shot content generator, but the driver still does not feed packet JSON or leases to it.
- Duplicate or forged turn: bounded by daemon-owned floor advancement and `conversation.say` idempotence.
- Child credential discovery outside env: not fully mitigated in the synthesis as written; record as accepted risk and prevent F42 from adding new child-readable credentials.
- Operator shell-loop regression: mitigated by selecting the supervised lane path as the production surface and deleting the `/tmp/gemini-driver.sh` recipe.

Interrogation used 2 rounds. Round 1 resolved the packet-spoon-feeding question and separated it from child credential self-discovery. Round 2 pinned the needed treatment of that residual risk: accept and record it for v1, plus require a no-new-child-readable-token invariant. I stopped there because the remaining boundary question was resolved enough to render a verdict.

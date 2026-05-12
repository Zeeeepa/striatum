# Review Design Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0032", "cross-repo", "mcp-mutation"]
---
```

Review `docs/dogfood/035/DESIGN_SYNTHESIS.md` under the **threat_model** posture. Use only an accepting verdict (`accept` or `accept_with_findings`) if the plan enumerates the trust boundaries and attack surfaces the cross-repo + MCP-mutation changes introduce, and each is either acknowledged or mitigated.

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads the daemon's signing key, kills the daemon, or impersonates the daemon process.

**Multi-repo / cross-repo END-TO-END integration tests are EXPLICITLY DEFERRED** to a follow-up RFC (`docs/TODO.md` Open item 19, multi-repo test harness). Do not refuse the design for lack of harness-level cross-repo tests as long as the synthesis lists unit-level + mock-based coverage with the deferral documented.

In scope for this review:

- **MCP capability gating**: does the synthesis specify that `tools/list` filters by token capability, that `tools/call` refuses missing-capability requests, and that every mutating call produces an audit row including denials?
- **No global `--allow-mutations`**: does the design refuse a global mutation flag and require explicit token capabilities? Reviewers should flag any text resembling "trusted client" framing as overclaim.
- **Capability scope enforcement**: does a `repo_id`-scoped token refuse calls against other `repo_id`s with documented `capability_missing` denial? Does the audit row record the attempted-scope mismatch?
- **Cross-repo run lifecycle**: does the synthesis specify daemon-crash reconciliation (preparing → started or aborted)? What happens when one participating repo is unregistered mid-run? Should fail-safe (pause + human checkpoint) rather than data-loss-by-accident.
- **Per-repo write-scope enforcement**: a job targeting repo B cannot write into repo A. Does the design specify this guarantee at the workflow validator level + at publish-artifact runtime?
- **Audit chain integrity across MCP mutations**: do MCP audit rows participate in the same hash chain as RPC audit rows on the RFC 0033 substrate? Are denials recorded with the documented vocabulary?
- **Prompt-injection mitigation**: capability tokens are the only access path. Short-lived tokens for mutation. Operator-controlled revocation. The synthesis must say this explicitly without weakening it.
- **Cross-platform reality**: cross-repo identity across macOS/Linux/Windows-via-WSL paths and realpath/inode derivation. Is the design specific or hand-wavy?
- **Provenance overclaim**: does the synthesis claim atomic file-system mutations across two repos? Cross-machine semantics? Cryptographic non-repudiation? Malicious-local-root resistance? Any of those would be overclaim per RFC 0031 threat model and the deferral list.
- **Scope discipline**: does the synthesis stay inside RFC 0032 V2 scope, or wander into cross-machine, Go port, or bundled-PG territory?

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. For `accept_with_findings`, the findings must be non-blocking and explicitly say so.

Stay inside the review write scope (`docs/dogfood/035/review/design/threat/`). Do not modify the synthesis. Do not call striatum CLI; the operator publishes otherwise.

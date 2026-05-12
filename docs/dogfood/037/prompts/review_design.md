# Review Design Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0035", "multi-repo-test-harness"]
---
```

Review `docs/dogfood/037/DESIGN_SYNTHESIS.md` under the **threat_model** posture. Use only an accepting verdict (`accept` or `accept_with_findings`) if the harness will exercise the threat surfaces RFC 0032 V2 introduced and the harness itself is deterministic + cleanly isolated.

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads the daemon's signing key, kills the daemon, or impersonates the daemon process.

In scope for this review:

- **Capability scope mismatch coverage**: token scoped to repo A used against repo B → refused with `capability_missing` + audit row. Is there a test? Does it assert both the refusal AND the audit row?
- **Default-deny coverage**: unknown method → `method_unknown` audit row. Capability not held → `capability_missing` audit row. Test asserts both.
- **Audit chain integrity coverage**: every mutating call produces an audit row; hash chain links continuously. Is there a test that asserts chain continuity across a sequence of allow + deny calls?
- **Crash recovery coverage**: SIGKILL mid-prepare/start/cancel → daemon-restart reconciliation rolls back or completes. Is there a test for each?
- **Per-repo write-scope coverage**: validator catches at submit time; runtime refuses with `write_scope_violation`. Both paths tested?
- **Audit chain tamper attempt coverage**: the test asserts the daemon's audit-append code path refuses out-of-order or tampered append calls.
- **No parallel production-code path**: does the synthesis state that the harness uses the same daemon binary, same migrations, same RPC envelope, same capability vocabulary, same audit chain helper? Any test helper that bypasses production code (e.g., direct SQL injection of audit rows) should be flagged.
- **Per-test reset semantics**: does the synthesis specify that audit chain rows are cleared cleanly between tests? Otherwise chain assertions will leak across test functions.
- **Determinism + cleanup hygiene**: ephemeral PG dropped, scratch dir removed, Unix socket deleted on stop. Does the synthesis specify the cleanup order and the SIGTERM+timeout posture?
- **CI integration**: `make test-multi-repo` skip-with-message when PG is unavailable. Will CI cleanly skip on macOS-no-PG?
- **Wall-clock budget**: < 60s added to local `make test`. Does the synthesis confirm this with per-class fixture scope amortizing daemon startup?
- **Scope discipline**: does the synthesis stay inside RFC 0035 V1, or wander into Docker, Windows daemon, cross-machine, or Go-client testing territory?

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. For `accept_with_findings`, the findings must be non-blocking and explicitly say so.

**IMPORTANT — write the REVIEW.md artifact directly in this single supervised invocation.** Per the dogfood-036 OPERATOR_REPORT.md intervention #2, a previous gemini design-review session surfaced a strategy summary and asked the operator "should I proceed?" and exited without producing the file. Do not repeat that pattern. The work packet's `expected_artifacts` requires the file at `docs/dogfood/037/review/design/threat/REVIEW.md`; you are inside a supervised wrapper that runs `gemini --prompt -` once per packet, with no follow-up turn. Write the file with the EXACT front-matter shape above (`verdict_intent` not `verdict`; `severity` from {low,medium,high,critical} not `none`; `tags` as a JSON array; `author: <slug>` byline as a plain markdown line AFTER the front-matter block, NOT a key inside it) + the verdict reasoning in this single invocation.

Stay inside the review write scope (`docs/dogfood/037/review/design/threat/`). Do not modify the synthesis. Do not call striatum CLI; the operator publishes otherwise.

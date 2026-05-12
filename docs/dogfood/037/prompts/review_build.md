# Review Build Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0035", "multi-repo-test-harness", "build"]
---
```

Review the implementation under the **threat_model** posture. Verify behavior, tests, harness mechanics, docs, and CI integration. Actually inspect the test cases from a first-principles threat-model perspective — does each test actually exercise the threat surface it claims, or is it stubbed?

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads the daemon's signing key, kills the daemon, or impersonates the daemon process.

Required checks:

- **Per-e2e-test-file threat-surface coverage**: read each of the five e2e files and verify the test assertions match the synthesis's threat-surface claim. A test labeled "scope mismatch refused" must actually assert refusal AND audit-row append.
- **Harness smoke test exercises the actual lifecycle**: start + register + reset + stop, with back-to-back instances. Read the test, not just the name.
- **Per-test DB reset truncates the right tables**: audit chain rows must be cleared cleanly between tests; otherwise chain continuity assertions leak. Read the `reset_daemon_db()` implementation and verify the TRUNCATE list.
- **Ephemeral cleanup**: PG database dropped on stop; scratch dir removed; Unix socket deleted. Read the `stop()` implementation.
- **`make test-multi-repo` skip path**: when PG is unavailable, the target skips cleanly with a clear message, not a confusing pytest error.
- **No flaky-on-CI patterns**: subprocess + Unix socket means no port conflict; SIGTERM + timeout means no hangs; the harness doesn't depend on race timing.
- **No parallel production-code path**: the harness uses the same daemon binary, same migrations, same RPC envelope, same capability vocabulary, same audit chain helper. Any test helper that injects audit rows directly or bypasses capability gating should be flagged.
- **Adversarial test coverage**: hostile MCP client probe; expired token replay; revoked token replay; scope mismatch; operator-confirmation bypass simulation; audit chain tamper attempt.
- **Documentation honesty**: TODO Open item 19 marked done with this dogfood as the landing point; SPEC/HOW_TO_HUMAN/CHANGELOG updates reflect actual shipped behavior. No claims of cross-machine support, Windows daemon support, Docker integration, or Go-client coverage.
- **Tests cover happy + denial + revoked + expired + scope-mismatch + unknown-method paths** — every path documented in the RFC 0030 capability vocabulary.

Use `needs_revision` for: behavior gaps in the shipped scope, missing tests for the threat surfaces above, audit-row append gaps, parallel production-code paths introduced by the harness, flaky cleanup, or documentation that overstates shipped behavior. Use `accept_with_findings` for non-blocking cleanup or follow-up RFC scope.

Stay inside the review write scope (`docs/dogfood/037/review/build/threat/`). Do not modify the implementation. Do not call striatum CLI; the operator publishes otherwise.

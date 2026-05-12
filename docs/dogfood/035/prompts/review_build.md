# Review Build Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0032", "cross-repo", "mcp-mutation", "build"]
---
```

Review the implementation under the **threat_model** posture. Verify behavior, tests, docs, migrations, and workflow compatibility. Inspect the repository within the review write scope policy (repo-level access).

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads the daemon's signing key, kills the daemon, or impersonates the daemon process.

**Multi-repo / cross-repo END-TO-END integration tests are EXPLICITLY DEFERRED** to a follow-up RFC (`docs/TODO.md` Open item 19, multi-repo test harness). **Do not refuse the build for lack of harness-level cross-repo tests** as long as:

- unit-level + mock-based coverage is present for each new module;
- schema/validator/write-scope/capability-gate tests are present;
- the deferral is clearly documented in `docs/dogfood/035/BUILD_HANDOFF.md` with a pointer to the follow-up RFC.

Required checks:

- **MCP capability gating on every mutation route**: read the `tools/call` code path and verify every mutating method has a declared required capability, and the daemon refuses calls whose token lacks that capability. Audit row records the denial with the documented vocabulary.
- **Per-token `tools/list` filtering correctness**: a token with only `read` does not see `write` tools; a token scoped to repo A does not see tools acting on repo B.
- **Default-deny enforcement for unknown methods**: an MCP request for an unregistered method returns the standard unknown-method error and records an audit row.
- **Audit row appended for every mutating `tools/call` including denials**: read the audit-append code path and verify every mutation lands an audit row in the daemon DB hash chain. Tests exercise both allowed and denied paths.
- **Cross-repo run state coherence under daemon crash**: there's a test (single-repo simulation acceptable) that exercises daemon-crash mid-prepare and asserts reconciliation completes or rolls back correctly. Real two-repo daemon-crash testing is DEFERRED.
- **Per-repo write-scope enforcement when a job targets a different registered repo**: tests with mocked registered repos verify that a job targeting repo B cannot publish artifacts into repo A's paths.
- **No raw `.striatum/state.sqlite3` write paths through MCP mutation clients**: the daemon never exposes a way for a non-admin MCP token to write to repo-local state directly. The daemon DB (RFC 0033) is the daemon's write surface; repo-local state mutations go through capability-gated RPC, not direct SQLite writes.
- **Workflow schema additions work**: the `repositories` block validator accepts well-formed inputs, refuses malformed ones (missing primary, unknown repo_id, circular cross-repo dependencies). Tests cover both.
- **Documentation honesty**: SPEC, MCP, UBIQUITOUS_LANGUAGE, CLI_REFERENCE, HOW_TO_HUMAN, RFC 0032 status reflect actual shipped behavior. No claims of atomic file-system mutations across repos, cryptographic non-repudiation, cross-machine semantics, or malicious-local-root resistance.
- **Tests cover happy paths and adversarial bypasses** for capability denial, audit append, write-scope enforcement, `tools/list` filtering, default-deny on unknown methods, capability scope mismatches.
- **Write scopes and fixtures do not normalize** direct `.striatum/` edits, transcript capture, or audit log tampering.

Use `needs_revision` for: behavior gaps in the shipped scope, missing tests for the threat surfaces above (excluding deferred multi-repo END-TO-END coverage), migration breakage, capability default-deny failures, or documentation that overstates daemon authority. Use `accept_with_findings` for non-blocking cleanup or follow-up RFC scope.

Stay inside the review write scope (`docs/dogfood/035/review/build/threat/`). Do not modify the implementation. Do not call striatum CLI; the operator publishes otherwise.

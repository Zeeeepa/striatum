# Review Build Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings). Example:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0030", "rfc-0031", "daemon", "build"]
---
```

Review the implementation under the **threat_model** posture. Verify behavior, tests, docs, migrations, and workflow compatibility. Inspect the repository within the review write scope policy (repo-level access).

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads the daemon's signing key, kills the daemon, or impersonates the daemon process. Reviews relying on the malicious-local-root framing should be redirected; that scrutiny is post-implementation in `devils_advocate` + `security` postures (not gated by this dogfood).

Required checks:

- **Capability authorization correctness on every RPC route**: every method registered with the daemon has a declared required capability, and the daemon refuses calls whose token lacks that capability. The audit row records the denial with the documented vocabulary (`capability_missing`, `token_revoked`, `token_expired`, etc.).
- **Audit row appended for every mutating request**: read the audit-append code path and verify every mutation lands an audit row including denials. Check tests exercise both allowed and denied paths.
- **Supervisor reattach behavior across daemon restart**: there's an integration test that kills the daemon mid-supervised-run and asserts the next daemon start reattaches by pid + pid_start_time, or marks the supervisor `lost` if those don't match.
- **Sealed-apply refuse paths**: at least one test for each refuse condition (digest mismatch, base-tree drift, wrong verdict, missing signing key). Receipts produced on the happy path carry the documented fields and avoid overclaiming non-repudiation in their text.
- **Signing key custody honesty**: code uses OS keyring with `0600` runtime fallback; the daemon refuses to start sealed-mode runs without a loadable key; the key is never written to env vars, audit log, repo files, or the request log. Tests verify the refuse-on-missing-key path. Per RFC 0031 threat model the operator-readable signing key is a documented non-goal; verify SPEC.md and README.md text reflects that exactly without weakening it.
- **Version-handshake refusal semantics**: a CLI with mismatched envelope is refused with exit code 10; no silent fallback to direct mode on version errors. Test asserts both sides.
- **No raw `.striatum/state.sqlite3` write paths through daemon clients**: the daemon never exposes a way for a non-admin token to write to repo-local state directly. The substrate (RFC 0033 daemon DB) is the daemon's write surface; repo-local state mutations go through the existing CLI commands routed via RPC, not via direct SQLite writes.
- **Workflow schema additions work**: `require_daemon: true`, `apply_gate: true`, `sealed_patch_provider: refuse` (debug aid) are validated and refused at workflow-validate time when used outside their declared positions.
- **Documentation honesty**: SPEC, MCP, UBIQUITOUS_LANGUAGE, CLI_REFERENCE, HOW_TO_HUMAN, RFC 0030 status, RFC 0031 status reflect actual shipped behavior. No claims of cryptographic non-repudiation, model-token authorship proof, or malicious-local-root resistance.
- **Tests cover happy paths and adversarial bypasses** for: capability denial, audit append, supervisor reattach, sealed apply refuse-on-mismatch, version skew, two-daemons-against-one-DB, token revocation race against in-flight calls.
- **Write scopes and fixtures do not normalize** direct `.striatum/` edits, transcript capture, or audit log tampering.

Use `needs_revision` for behavior gaps, missing tests for the threat surfaces above, migration breakage, capability default-deny failures, or documentation that overstates daemon authority. Use `accept_with_findings` only for non-blocking cleanup or follow-up RFC scope.

Stay inside the review write scope (`docs/dogfood/034/review/build/threat/`). Do not modify the implementation. Do not call striatum CLI; the operator publishes otherwise.

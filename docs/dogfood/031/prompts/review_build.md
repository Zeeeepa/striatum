# Review Build Prompt

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter.

Review the V1 daemon implementation under your assigned posture. Verify behavior, tests, docs, migrations, and workflow compatibility. Inspect the repository within the review write scope policy.

Required checks:

- every RFC 0028 §Acceptance Criteria bullet claimed by the synthesis is backed by code and at least one test, or is explicitly marked deferred;
- direct CLI mode continues to work for the existing dogfood workflows and tests, unless the accepted plan explicitly retires a verb and migrates the fixtures;
- daemon mutation tools default to refused without an explicit client capability; the audit log records every mutating request with client id, repository id, command, authorization result, and timestamp;
- `striatum repo add` refuses symlink and path-traversal tricks and does not expose raw `.striatum/state.sqlite3` write paths through daemon clients;
- daemon restart with a pre-existing registry and at least one registered repo-local state store is exercised by tests, including supervised-process re-attach behavior described by the synthesis;
- multi-repo dashboard and MCP resource listing return correct, deduplicated state across registered repositories;
- recovery sweep runs against all active runs without requiring per-run `recovery watch`, while preserving D036 safety policy;
- docs and status/evidence/run-summary/web surfaces describe actual daemon guarantees honestly and do not imply RFC 0026 attestation or RFC 0027 sealed guarantees the V1 daemon does not provide;
- write scopes and fixtures do not normalize direct `.striatum/` edits, transcript capture, or audit log tampering.

Use `needs_revision` for behavior gaps, missing tests for acceptance-criteria bullets, migration breakage, capability default-deny failures, or documentation that overstates daemon authority. Use `accept_with_findings` only for non-blocking cleanup.

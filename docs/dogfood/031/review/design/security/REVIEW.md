---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "medium"
tags: ["security", "design_review", "rfc-0028"]
---

# Security Review: RFC 0028 V1 Daemon Design

author: operator

Status: review
Date: 2026-05-11
Posture: security
Target: `docs/dogfood/031/DESIGN_SYNTHESIS.md`

## Verdict

`needs_revision`.

The synthesized plan makes the right high-level security calls: V1 is optional,
local-first, Unix-socket-first, read-only for daemon MCP, explicit for CLI daemon
mode, and honest about RFC 0026/RFC 0027 provenance limits. It also rejects the
highest-risk expansions from the input designs, especially daemon-owned
supervision, generic daemon `/invoke`, MCP mutation tools, apply authority,
signing keys, and remote serving.

The plan is not yet implementation-ready from the security posture because two
security boundaries are still open-ended: capability-token lifecycle/defaults
and daemon audit retention/integrity. Those are acceptance-critical daemon
features, not optional operational polish. A build started from the current
synthesis could plausibly ship an accidentally permissive read surface or an
audit log that meets the column-shape requirement while failing basic local
forensics and privacy expectations.

## Blocking Findings

### F1. Capability-token issuance and defaults are underspecified

The synthesis says the default posture is fail closed, that there are no
anonymous reads except a minimal health check, and that socket access alone is
not a mutation capability. That is good. But it also leaves the concrete token
model unresolved: "CLI over owner-only Unix socket may use an operator-configured
read token" in `MCP Surface And Capability Defaults`, while
`Human-Decision Questions` still asks whether even read resources require
explicit tokens for all MCP clients. `Registry Storage Decision` defines
`clients.token_hash` and `token_salt`, but the implementation plan does not
specify token id format, secret generation entropy, first-run bootstrap
behavior, plaintext storage rules, client config file permissions, rotation,
revocation UX, expiry defaults, or constant-time comparison.

This is a blocking security gap because RFC 0028's daemon is multi-repository.
A read token is enough to expose workflow metadata, blockers, artifacts, stale
leases, and run history across all repos in scope. Prompt-injected MCP clients
also become much more dangerous if the default read-token story is implicit or
shared too broadly.

Minimum changes:

- State that every non-health daemon request, including Unix-socket CLI and MCP
  reads, requires an explicit token.
- Define first-run bootstrap token creation, one-time secret display, storage
  permissions, and the expected operator path for minting a non-admin read
  token.
- Define token id plus secret lookup, hashing, salt handling, constant-time
  comparison, expiry, revocation, rotation, and repository scoping.
- Add tests for no-token reads, repo-out-of-scope reads, expired/revoked tokens,
  token redaction in audit and errors, and MCP prompt-injected mutation-shaped
  calls.

### F2. Audit logging is metadata-safe but not yet integrity- or retention-ready

The synthesis correctly requires metadata-only audit rows and excludes prompts,
response bodies, artifact content, terminal output, token secrets, salts, and
tracebacks in `Audit Log Shape`. It also acknowledges that "audit retention /
rotation defaults" need a human decision before broad release. That is too late
for this V1 slice because RFC 0028 acceptance requires the daemon to record
every client request, and the security posture specifically depends on denied
MCP/tool attempts and capability failures remaining inspectable.

The current plan does not say whether audit rows are append-only at the API
layer, how log growth is bounded, what happens on rotation, whether old audit
segments are immutable or merely renamed, how `daemon doctor` detects dangerous
registry/audit file permissions, or how corruption/tamper is surfaced. It is
acceptable for V1 not to defend against local root, but it should still prevent
accidental deletion through daemon APIs and make local tamper or missing audit
state visible.

Minimum changes:

- Specify append-only daemon API behavior for audit rows: no update/delete
  endpoint and no request path that rewrites prior audit entries.
- Define retention and rotation defaults before implementation, including what
  stays queryable through `daemon audit`.
- Add registry and audit file permission checks to `daemon doctor`.
- Add tests for denied requests being audited, audit redaction, rotated or
  missing audit storage surfacing as a doctor problem, and registry schema/user
  version tamper refusing unsafe startup.

## Non-Blocking Findings

### F3. Unix socket and loopback HTTP posture is acceptable but needs exact bind checks

`Accepted Implementation Scope`, `MCP Surface And Capability Defaults`, and the
test matrix all commit to Unix socket by default, optional loopback HTTP, and
non-loopback refusal. That is the right V1 boundary. The implementation should
make this precise: socket directory `0700`, socket owner-only, registry file
`0600`, refusal of `0.0.0.0`, `::`, and non-loopback hostnames that resolve
outside loopback, and shared authorization for Unix and HTTP transports.

This can be handled during implementation if it is explicitly captured in tests.

### F4. Repository registration path safety is correctly identified

The synthesis requires canonical repo roots, duplicate canonical-root refusal,
and refusal of symlink or traversal ambiguity in `Existing-state migration
plan` and the test matrix. That satisfies the design-level requirement. The
implementation should use resolved paths consistently for both `repo_root` and
`state_db_path`, and should avoid accepting a `.striatum/state.sqlite3` reached
through a symlink escape.

### F5. Raw SQLite write exposure is honestly bounded

The design avoids a generic daemon `/invoke`, exposes explicit read endpoints,
and keeps existing workflow mutations on the direct CLI path. That avoids giving
daemon clients a raw `.striatum/state.sqlite3` write channel. The remaining
ability of a local filesystem user to edit SQLite directly is explicitly called
out as outside V1's adversarial-local-root model, which is consistent with
RFC 0026 and RFC 0027.

### F6. Upgrade and downgrade behavior needs one more explicit refusal rule

The `Compatibility And Upgrade Risks` section covers newer/older CLI and daemon
protocol skew and newer repo-local DB refusal. Add one concrete rule: a forced
daemon mode request (`--daemon` or `STRIATUM_DAEMON=1`) must never silently fall
back to direct mode on version skew, missing token, missing daemon, or denied
capability. The synthesis implies this, but the rule should be stated because it
prevents a downgrade from daemon-required policy into direct CLI behavior.

### F7. Provenance honesty is strong

The synthesis is careful about RFC 0026 and RFC 0027. `Staging Plan For
Provenance Honesty` says daemon audit proves only client request metadata, token
labels are not bylines, lane attestation is still derived from repo-local
helpers, and `sealed_patch` continues to refuse start. This meets the security
posture as long as UI/docs avoid words like "proof", "trusted apply", or
"sealed" for the V1 daemon.

## Minimum Revision Set

Before implementation proceeds, update the synthesis or build handoff with:

1. A concrete capability-token lifecycle and default policy, including mandatory
   auth for all non-health reads.
2. A concrete audit retention, rotation, append-only API, redaction, and doctor
   integrity plan.
3. An explicit no-fallback rule for forced daemon mode on version/auth failures.
4. Tests covering the above, plus exact Unix-socket/loopback bind refusal and
   path-registration escape cases already listed in the synthesis.

With those changes, the design should be acceptable from the security posture
for the narrow RFC 0028 V1 slice.

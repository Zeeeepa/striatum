---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0040", "mcp-harness", "build"]
---

author: reviewer-codex-gpt-5.5-002

# RFC 0040 Build Review - Systems Threat Model

Verdict: needs_revision

This review covers the systems-side RFC 0040 surfaces: daemon MCP
composite-tool execution, composite atomicity, `surgical_recovery`
capability gating, supervised-progress watcher concurrency, and adversarial
test coverage. Threat posture follows RFC 0031: over-eager AI agents and
operator-mistake footguns are in scope; malicious local root is out of scope.

## Trust Boundaries Reviewed

RFC 0040 introduces four systems authority boundaries. First, daemon MCP
clients can discover and call mutation methods from the method registry.
Second, `dogfood.publish_on_behalf` collapses ack, artifact publication,
review verdict or completion, and operator reason capture into one operator
action. Third, `dogfood.surgical_recovery` bypasses the normal repo-write
stale-lease refusal after operator inspection. Fourth, the supervised-progress
watcher can refresh leases based on filesystem progress signals from a
supervised process.

The implementation acknowledges these boundaries, but several are not yet
enforced strongly enough for acceptance.

## Findings

F1 - High - Allowed daemon MCP tool calls authorize and audit but do not
dispatch the requested method.

`DaemonRpcServer.call_daemon_tool` authorizes the named method and appends an
MCP audit/request-log row in `src/striatum/mcp.py:599` through
`src/striatum/mcp.py:631`, then returns `_daemon_tool_result(name, ok=True,
audit_id=audit_id)` at `src/striatum/mcp.py:632`. It never routes the allowed
call through `DaemonRpcRouter`, `invoke`, or the dogfood handlers in
`src/striatum/daemon_rpc/server.py:200`. Impact: `tools/list` can advertise
`dogfood.publish_on_behalf` and `dogfood.surgical_recovery`, and `tools/call`
can return success with an audit id, while no lease, artifact, verdict,
supervisor, or job state changes. That is a misleading success path at the
main MCP mutation boundary.

F2 - High - `dogfood.publish_on_behalf` can record a review verdict with only
the composite method's `write` capability.

The method registry gates `dogfood.publish_on_behalf` with only `write` in
`src/striatum/daemon_rpc/registry.py:77`. The helper then records review
verdicts when the target job is a review job in
`src/striatum/dogfood/operator_tools.py:137` through
`src/striatum/dogfood/operator_tools.py:153`. Ordinary verdict submission is
gated by `review` (`src/striatum/daemon_rpc/registry.py:75`), so the composite
creates a privilege-escalation path for a write-only token to perform a review
decision. RFC 0040 says the composite requires the underlying
publish-artifact plus complete/verdict capability chain; this implementation
does not enforce that chain.

F3 - High - `dogfood.publish_on_behalf` is not atomic across its composed
state transitions.

`publish_on_behalf` calls `ack_work`, `publish_artifact`, and then
`record_review_verdict` or `complete_job` at
`src/striatum/dogfood/operator_tools.py:98`,
`src/striatum/dogfood/operator_tools.py:108`, and
`src/striatum/dogfood/operator_tools.py:140`/`155`. Those callees own their
own transactions, while the composite event is inserted only afterward at
`src/striatum/dogfood/operator_tools.py:166`. If a later step fails, earlier
state is already committed and the final composite event may never be written.
Impact: a caller can leave claimed work advanced to running, or an artifact
published without the intended verdict/complete step, with no single
composite record explaining the failed operation.

F4 - High - The supervised-progress watcher is implemented but not wired into
daemon or supervisor lifecycle.

`src/striatum/daemon_supervisor/progress_watcher.py:154` defines the
DB-aware watcher, and `src/striatum/daemon_supervisor/progress_watcher.py:331`
defines the polling loop. I found no production caller outside tests and the
shared advisory-lock import used by surgical recovery. RFC 0040 requires the
daemon to start one watcher per supervisor; without that integration, the
dogfood-038 failure mode remains: actively running supervised work can still
expire its lease.

F5 - Medium - Watcher race and signal hardening are incomplete.

The DB-aware watcher reads an active lease at
`src/striatum/daemon_supervisor/progress_watcher.py:261` and later calls
heartbeat at `src/striatum/daemon_supervisor/progress_watcher.py:215`. If the
job completes or releases its lease between those operations, `heartbeat`
revalidates and can raise, but `tick` does not convert that normal race into a
benign no-active-lease result. The progress signal is also only fresh mtime
from `os.stat` (`src/striatum/daemon_supervisor/progress_watcher.py:88`), not
proven file growth, and the liveness check is PID-only (`os.kill(pid, 0)`)
without `pid_start_time` identity. Impact: the watcher can crash on expected
lifecycle races, can be kept alive by touched or symlinked logs, and can
heartbeat after PID reuse.

F6 - Medium - Tests cover helper happy paths and mocked gating, not the
adversarial systems contract.

The focused test suite passed, but key threat cases are missing. There is no
allowed MCP `tools/call` test that proves the method mutates repo-local state,
no write-only token test proving `publish_on_behalf` cannot record a review
verdict, no rollback test for mid-chain composite failure, no RPC/MCP denial
test proving `dogfood.surgical_recovery` leaves repo state unchanged with the
wrong capability or wrong repo scope, and no production watcher lifecycle test.
The current tests exercise direct helpers and registry visibility, which is
not enough for the trust boundaries above.

## Recommendation

Revise before acceptance. The minimum repair is to make allowed daemon MCP
tool calls execute the same method path as daemon RPC, enforce `review` when
`publish_on_behalf` records a review verdict, make publish-on-behalf
all-or-nothing or always record failed composite attempts, wire the progress
watcher into supervisor lifecycle with race handling and PID/log hardening,
and add adversarial tests for the exact privilege and concurrency cases.

## Verification

Focused tests passed locally:

`pytest tests/test_dogfood_publish_on_behalf.py tests/test_dogfood_surgical_recovery.py tests/test_supervised_progress_watcher.py tests/test_daemon_rpc.py tests/test_mcp_mutation_capabilities.py tests/test_daemon_rpc_registry.py`

Result: 32 passed.

---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0040", "v1-5", "build"]
---

# RFC 0040 V1.5 Build Review

author: reviewer-unknown-model-001

## Verdict

needs_revision

## Scope

This is a document-only threat-model review of the packet-provided RFC 0040 materials: `docs/rfcs/0040-mcp-driven-dogfood-harness.md`, `docs/dogfood/040/OPERATOR_REPORT.md`, and `docs/dogfood/040/decisions/cycle-exhaustion-codex-build-review.md`.

The trust boundaries introduced by this artifact are:

- MCP `tools/list` and `tools/call` capability filtering for operator-side dogfood lifecycle tools.
- Dispatch from MCP tool calls into the RFC 0030 method registry.
- Audit-chain recording for allowed and denied calls, including composed operations.
- Composite state transitions for `dogfood.publish_on_behalf`.
- Admin-bound state restoration for `dogfood.surgical_recovery`.
- Daemon-side supervised-progress watcher authority to refresh active leases from log-file mtime.
- Harness-profile prompt fragments that influence one-shot supervised agents but are not execution authority.

## Findings

### F1 - Implementation Evidence Missing For Required Threat-Model Checks

The review prompt requires implementation-site citations, backward-compatibility regression-test names, end-to-end MCP-path tests, and a failing-step fixture that demonstrates composite rollback. The packet's document-only input does not include the dogfood-044 implementation handoff or source/test evidence, so those checks cannot be verified from the allowed material.

This is blocking because the RFC acceptance criteria are explicitly behavioral: dogfood-lifecycle tools must be capability-gated and audit-appending, `publish_on_behalf` and `surgical_recovery` must work end-to-end, the supervised-progress watcher must refresh leases when logs grow, composite tools need harness coverage, and no existing lifecycle behavior may regress (`docs/rfcs/0040-mcp-driven-dogfood-harness.md:341`). Without implementation and test citations, accepting the build would be a provenance gap, not a review result.

### F2 - Prior Dispatch And Audit Failures Are The Exact Critical Boundary

The highest-risk boundary is MCP authorization versus actual method dispatch. RFC 0040 states that dogfood-lifecycle tools require existing per-method capabilities and append an audit row per allowed or denied call (`docs/rfcs/0040-mcp-driven-dogfood-harness.md:189`). Dogfood 040 then records that the daemon MCP `tools/call` path authorized and audited but did not dispatch through the method registry, leaving composite tools non-functional while producing misleading audit success (`docs/dogfood/040/OPERATOR_REPORT.md:138`).

That failure shape is a security and audit-chain problem, not just an ergonomics bug. A caller can receive apparent success evidence for an operation that did not execute, and downstream operators may trust the audit chain as if it represented a completed state transition. The V1.5 build needs direct evidence that the MCP dispatch route now performs authorization, registry dispatch, result/error recording, and audit-row append as one coherent path.

### F3 - Composite Atomicity And Verdict Semantics Are Still Unproven

RFC 0040 deliberately turns `ack` + `publish-artifact` + `verdict` + `complete` into the `dogfood.publish_on_behalf` composite tool (`docs/rfcs/0040-mcp-driven-dogfood-harness.md:193`) and says the daemon performs the lookup and sequence internally (`docs/rfcs/0040-mcp-driven-dogfood-harness.md:219`). It also models the composite tools as single audit-chain operations with `composition_steps` metadata (`docs/rfcs/0040-mcp-driven-dogfood-harness.md:437`).

Dogfood 040 records unresolved F2/F3 findings around `publish_on_behalf` atomicity and verdict-recording semantics, including the risk that a single audit row records success even if a composed step fails partway (`docs/dogfood/040/OPERATOR_REPORT.md:141`). The V1.5 build must show an atomic transaction or compensating failure semantics for every mid-step failure point, especially after artifact publish succeeds but before verdict or complete records.

### F4 - Surgical Recovery Needs Strong Authorization And Invariant Evidence

`dogfood.surgical_recovery` crosses the strongest boundary in the RFC: it is admin-bound, reactivates an expired lease, reattaches a supervisor, and restores queue message plus job state (`docs/rfcs/0040-mcp-driven-dogfood-harness.md:241`). Dogfood 040 recorded that the capability/registry wiring was incomplete for this path and that the finding was deferred to V1.5 (`docs/dogfood/040/OPERATOR_REPORT.md:89`, `docs/dogfood/040/decisions/cycle-exhaustion-codex-build-review.md:23`).

The threat model requires proof that this cannot be reached through ordinary write/review tokens, stale MCP tool definitions, or direct composite dispatch without the new admin-bound capability. It also needs invariant checks for expected artifact presence, no concurrent supervisor, and exact restoration to post-ack-pre-complete state. The packet documents the required shape but does not provide implementation evidence that these constraints are enforced.

### F5 - Watcher Race And Signal Correctness Remain A Lease-Authority Risk

The supervised-progress watcher grants daemon authority to refresh leases based on log-file mtime (`docs/rfcs/0040-mcp-driven-dogfood-harness.md:262`). The planned implementation polls `os.stat(log_path).st_mtime` every 30 seconds and heartbeats when mtime is recent (`docs/rfcs/0040-mcp-driven-dogfood-harness.md:269`). Dogfood 040 explicitly left watcher wiring, race handling, and signal hardening as V1.5 follow-up (`docs/dogfood/040/OPERATOR_REPORT.md:140`, `docs/dogfood/040/OPERATOR_REPORT.md:142`).

This boundary needs adversarial tests for missing log at watcher start, log rotation, stale path reuse, supervisor stop during heartbeat, SIGTERM/SIGINT while a heartbeat is in progress, and watcher shutdown after supervisor loss. Without those tests, the watcher can either fail open by refreshing a lease after the supervised process is no longer authoritative, or fail closed by allowing active work to expire under the same conditions RFC 0040 is meant to fix.

## Required Revision

Provide the build handoff and implementation evidence for F1-F6, including file/function or line citations for the MCP dispatch route, composite transaction boundaries, surgical-recovery capability checks, watcher lifecycle integration, watcher race/signal handling, and end-to-end tests. The evidence should include a failing-step rollback fixture and full MCP-path tests rather than mocked-only gating, because dogfood 040 identified mocked-only coverage as a remaining risk (`docs/dogfood/040/OPERATOR_REPORT.md:143`).

---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags:
  - rfc-0112
  - design-review
  - dissent-review
---

# Dissent Review: RFC 0112 Explicit Interrogation Consumers

author: dissent-reviewer-gemini-3.5-flash-high-001
date: 2026-06-05

## Executive Summary

The unified arbitration plan presented in [ARBITRATOR_SYNTHESIS.md](ARBITRATOR_SYNTHESIS.md) successfully resolves the core lifecycle bugs of RFC 0112. By using a snapshot-derived consumer set and centralizing terminal transitions behind the `markJobTerminal` choke point, it eliminates both the early-close window bug and the late-close session leak.

However, from a devil's-advocate perspective, several load-bearing design decisions introduce unnecessary complexity or leave subtle edge cases unaddressed. This review details these findings and recommends adjustments before finalizing the design.

---

## Findings

### Finding 1: V1 Target Cardinality ($N \ge 1$) is a Premature Generalization (L3)

* **Severity**: Minor
* **Description**: The arbitration synthesis defaults to allowing multiple targets ($N \ge 1$) in V1, using a lint warning for $>3$ targets. While the underlying database query and release logic are set-based, allowing multiple targets in V1 complicates packet projection, validation, and lane behavior (e.g., handling mixed target states where one target is `available` and another is `unavailable`).
* **Counterargument**: No current or near-term workflow shape (including the primary target, Adjudicated Constraint Extraction) requires more than a single interrogation target. Capping the array length at `len = 1` in V1 validation rules simplifies the state space and avoids having to define or test complex mixed-state packet projection rules.
* **Recommendation**: Set the target cardinality dial to **cap at exactly one target** (`len = 1`) for the V1 release. The schema can retain the array shape to ensure forward compatibility, but the validator should reject configurations with multiple targets.

### Finding 2: Toothless Advisory-Only Required Semantics (L2)

* **Severity**: Minor
* **Description**: The plan designates `required: true` as advisory in V1 to prevent deadlocks under revision reopen cascades. However, this means a malfunctioning or lazy lane agent can complete its job without performing required interrogations, with the only consequence being a post-mortem event log entry (`interrogation.required_skipped`).
* **Counterargument**: If the workflow shape depends on cross-examination as a load-bearing evidence gate, allowing the lane to bypass it without a hard gate weakens the reliability guarantee.
* **Recommendation**: Define a concrete graduation path for V2 where the daemon enforces the gate under strict conditions: if a required target session is still `active` or `awaiting_interrogation` (i.e., not closed or unavailable), refuse `work.complete` unless the consumer has opened at least one interrogation against it.

### Finding 3: Skipped Targets in Packet Projection (L6)

* **Severity**: Info
* **Description**: The packet projection rules define `not_ready` as the state when no awaiting session exists for the target's current attempt. However, if a target job is skipped (e.g., due to a workflow branching decision or manual skip), a session for its current attempt will never be created.
* **Counterargument**: Under the proposed rules, a skipped target would project as `not_ready` (with reason `target_not_yet_completed` or similar) indefinitely, which is misleading since the target will never run.
* **Recommendation**: Explicitly handle target job states in the projection helper. If the target job is in the `skipped` state, project `unavailable` with a specific reason: `target_skipped`.

### Finding 4: Incomplete Fixture Rigor for Event Recording (L5)

* **Severity**: Info
* **Description**: The proposed RFC 0105 fixture cells in `go/pkg/adapterconformance/ace_interrogation_test.go` assert happy paths, revision reopens, and dead-lane requeues. However, they do not assert that the newly introduced evidence events (`interrogation.unavailable_signaled` and `interrogation.required_skipped`) are correctly recorded in the database.
* **Counterargument**: Without asserting event creation, we run the risk of silent failures in the event-logging code path which would defeat the advisory auditing mechanism.
* **Recommendation**: Add explicit assertions in the fixture test cells verifying that:
  1. `interrogation.unavailable_signaled` is appended when a consumer attempts to open a session against an unavailable target.
  2. `interrogation.required_skipped` is appended when a consumer completes a job without interrogating a required target.

---

## Verdict

**Verdict Intent**: `accept_with_findings`

The arbitration plan is implementation-ready. Resolving the cardinality dial (capping at 1) and refining the skipped-target projection will make the V1 implementation simpler, more robust, and easier to test.

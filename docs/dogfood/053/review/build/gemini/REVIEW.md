---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "high"
tags: ["threat_model", "rfc-0046", "v1", "build", "adversarial"]
---

author: reviewer-unknown-model-001

# Adversarial Build Review: RFC 0046 V1 (Lane Evidence Guard)

This review evaluates the V1 implementation of the Lane Evidence Guard against the adversarial objectives and the original RFC 0046 design.

## Executive Summary

The V1 build successfully implements the schema migration, the core guard logic in `publish_artifact`, and the operator override mechanism. However, as identified in the `HANDOFF.md`, the implementation deviates significantly from the RFC by omitting path-specific evidence verification. This creates a large trust boundary loophole that allows "attestation by association" rather than "attestation by evidence."

**Verdict: ACCEPT_WITH_FINDINGS**

---

## Adversarial Findings

### A1: Attestation by Association (Loophole)
**Severity: High**
**Objective:** "Enumerate trust boundaries and attack surfaces."

The current implementation of `_lane_evidence_present` only checks if *any* successful (exit 0) process execution exists for the session. 

- **Mechanism:** A single clean execution (e.g., `ls` or `git status`) satisfies the guard for all subsequent artifact publishes in that session.
- **Adversarial Vector:** An attacker with control over the operator's environment can run a benign command via `striatum supervise start`, then manually create a malicious file and call `striatum publish-artifact` with the model byline. The system will accept the artifact as "attested" because a clean process row exists, even though that process never touched the artifact.
- **Impact:** The "model byline" no longer guarantees that the model actually produced the file content.

### A2: Path-Specific Verification Gap (RFC Deviation)
**Severity: Medium**
**Objective:** "Refuse model-byline publish when no process_executions row covers the path."

The RFC 0046 design explicitly requires that the `observed_output_paths_json` contains the artifact's repo-relative path. 

- **Finding:** This check is entirely absent in V1. The `HANDOFF.md` cites a lack of the column in the `process_executions` schema as the reason.
- **Risk:** Without this link, the system cannot distinguish between an artifact produced by the lane and an artifact placed in the worktree by an outside process.

### A3: Weak Override Rationale Validation
**Severity: Low**
**Objective:** "Override flag with empty rationale refuses (exit 2)."

While the system correctly refuses an empty rationale, the validation is a simple `.strip()` check.

- **Mechanism:** Rationale strings like `"."`, `"x"`, or `"bypass"` are accepted.
- **Risk:** Operators may use junk strings to bypass the guard quickly, degrading the quality of the audit chain. 

---

## Objective Verification

| Objective | Status | Notes |
| :--- | :--- | :--- |
| Schema migration adds column | **PASSED** | v15 migration verified in `tests/test_lane_evidence_guard.py`. |
| Refuse if no evidence covers path | **DEGRADED** | Refuses if *no* evidence exists, but does not check path coverage (A2). |
| Override refuses empty rationale | **PASSED** | Verified in `dispatch.py` and unit tests. |
| Override stores rationale + event | **PASSED** | Verified in `artifacts.py::publish_artifact`. |
| Operator-byline pass-through | **PASSED** | `_is_operator_byline` correctly gates the check. |

---

## Recommendations

1.  **V1.7 Priority:** Prioritize the addition of `observed_output_paths_json` to `process_executions` and implement the path-specific check in `_lane_evidence_present`.
2.  **Rationale Minimums:** Consider a minimum length or a "suggested rationale" prompt in the CLI to discourage junk rationales.
3.  **Audit Visibility:** Ensure the Web UI prominently flags artifacts where "Attestation by Association" occurred (i.e., multiple artifacts sharing one process execution vs. 1:1 mapping).

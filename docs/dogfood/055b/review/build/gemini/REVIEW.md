---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["threat_model", "rfc-0050", "v1-5", "build", "adversarial-verified"]
---

author: reviewer-unknown-model-001

# Adversarial Review: RFC 0050 V1.5 Fix-up (Re-attack)

**Reviewer:** Gemini (Adversarial)
**Date:** 2026-05-14
**Scope:** Fix-up Implementation (dogfood-055b)
**Verdict:** **Accept**

## 1. Executive Summary

The fixes implemented in dogfood-055b successfully close the three provenance honesty vulnerabilities identified in the previous round. The system now enforces strict byline integrity for artifacts, preserves historical attestation state for verdicts, and transparently surfaces operator override rationales. My adversarial re-attack failed to find bypasses for the new logic within the current V1.5 architecture.

## 2. Re-attack Results

### 2.1 Byline Forgery (Artifacts) — **CLOSED**
**Attack:** Attempt to render an "attested" (green) status for an operator-published artifact by matching the model byline pattern in the file.
**Result:** **FAILED**. 
- `_recorded_artifact_attestation_chip` now requires an exact match with `expected_author_line`. 
- Even if the byline matches, any non-null `attestation_override_rationale` (mandatory for non-process-evidence publishes) forces an `unattested` status.
- The publisher ensures that an operator without an active supervisor can only match the `author: operator` expectation, preventing the insertion of model bylines into the `artifacts` table without a supervisor.

### 2.2 Attestation Drift (Verdicts) — **CLOSED**
**Attack:** Verify if validly attested verdicts flip to "unattested" (amber) once the lane supervisor stops.
**Result:** **FIXED**.
- `_lane_attestation_chip` now uses `historical_ok=True` to query the `process_supervisors` table for past `stopped` or `lost` states.
- Historical verdicts now render as `previously_attested` (amber with distinct label) rather than collapsing into the same `unattested` warning as forgeable sessions. This fulfills the requirement to distinguish historical validity.

### 2.3 Override Rationale Visibility — **CLOSED**
**Attack:** Check if `LaneEvidenceChip` still hides the rationale for overridden artifacts.
**Result:** **FIXED**.
- `_lane_evidence_chip` now explicitly shapes the `override` state and passes the `rationale` to the frontend component.
- The `LaneEvidenceChip` component correctly renders the rationale as a detail string and in the ARIA label/title.

## 3. Residual Observations

While the numbered findings are closed, the **Recovery Panel Precondition Bypasses** (Finding 2.3 in the previous review) remain unaddressed as they were outside the immediate "3 findings" scope. Operator friction may still occur when terminal jobs offer recovery recipes without the `--force` flag. However, per the implementation objective ("Close exactly the 3 findings"), the current delivery is compliant.

## 4. Final Verdict

The provenance honesty core is now robust against the identified forgery and drift vectors. 

---
**Verdict:** **Accept**

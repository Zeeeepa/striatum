---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0050", "v1-5", "build", "operator-on-behalf"]
---

author: reviewer-unknown-model-001

# Adversarial Review: RFC 0050 — Operator UI rework and provenance honesty

**Reviewer:** Gemini (Adversarial)
**Date:** 2026-05-14
**Scope:** V1 Implementation (dogfood-055)
**Verdict:** **Needs Revision**

## 1. Executive Summary

The V1 implementation of the UI rework makes significant strides in information density and operational transparency. However, it fails to meet the "provenance honesty" mandates in two critical areas: **byline forgery prevention** and **attestation drift mitigation**. The current logic relies on weak evidence (regex matching on strings) and real-time recomputation, which allows for the exact overclaims the design documents explicitly forbid.

## 2. Trust Boundaries & Attack Surfaces

### 2.1 Byline Forgery (Artifacts)
**Trust Boundary:** The boundary between the on-disk artifact content (untrusted) and the state store's `author_line` metadata.
**Vulnerability:** `src/striatum/service.py::_recorded_artifact_attestation_chip` determines whether an artifact is "attested" (green status) by applying a regular expression (`ATTESTED_BYLINE_BODY_RE`) to the recorded `author_line`.
**Attack Vector:**
1.  An operator publishes an artifact using `striatum publish-artifact --allow-no-process-execution --override-rationale "..."`.
2.  If the operator (or a compromised model) ensures the file contains a string matching the model byline pattern (e.g., `author: reviewer-codex-001`), the runner records this as the `author_line`.
3.  The UI, seeing this string, renders a green "attested" chip.
4.  **Result:** The UI falsely claims the artifact is attested even though it was an operator-override publish with zero process evidence.

### 2.2 Attestation Drift (Verdicts & Sessions)
**Trust Boundary:** The temporal boundary between the time of recording (verdict/artifact creation) and the time of inspection.
**Vulnerability:** `src/striatum/service.py::_lane_attestation_chip` and `striatum/cli/introspect.py::_session_attestation_summaries` both recompute attestation state by calling `session_lane_attestation()`, which checks for an *active* and *attached* supervisor *now*.
**Attack Vector:**
1.  A model session publishes a verdict while attested (supervisor is live).
2.  The supervisor exits or is stopped.
3.  An auditor views the run detail. The verdict now shows as "unattested" (amber) because the supervisor is gone.
4.  **Result:** Honest provenance is lost. The UI fails to distinguish between "this was never attested" and "this was attested but the lane has since cooled." This violates `UI_REWORK.md` §1 ("The UI must read attestation at the time the artifact was recorded").

### 2.3 Recovery Panel Precondition Bypasses
**Trust Boundary:** The UI's representation of recovery actions vs. the CLI's enforcement of preconditions.
**Vulnerability:** The recipes generated in `src/striatum/service.py::_recipes_for_blocker` are static and do not always include necessary safety flags like `--force` for terminal jobs.
**Attack Vector:**
1.  A `human_checkpoint` or `process_exit_nonzero` blocker exists on a job that has since moved to a terminal state (e.g., run was canceled).
2.  The UI provides a recipe for `checkpoint resolve --action continue` without `--force`.
3.  The operator copies and runs it; the CLI rejects it.
4.  **Result:** Friction and false promises. While not a security breach, it violates the "No silent recovery promises" rule in `UI_REWORK.md` §1.

## 3. Detailed Findings

### [Finding 001] Weak Attestation Check in Artifact Rows (High Severity)
The `_recorded_artifact_attestation_chip` function in `service.py` is the root of byline honesty failure. It trusts the `author_line` string if it "looks" right.
- **Code:** `service.py:278`
- **Correction:** Attestation state must be persisted in the `artifacts` table at publish time or derived from a join with `process_executions` (GH #5). In the absence of GH #5, the UI should only claim "attested" if `attestation_override_rationale` is NULL AND the byline matches the `expected_author_line`.

### [Finding 002] Real-time Recomputation of Verdict Attestation (Medium Severity)
The `_shape_verdict_rows` function re-evaluates attestation for every verdict on every page load.
- **Code:** `service.py:683`
- **Correction:** Verdicts should be enriched with attestation state at the time of `submit-review`. If the schema cannot be changed, the UI must at least acknowledge that "attestation unavailable for historical sessions" rather than defaulting to a warning-colored "unattested" status.

### [Finding 003] LaneEvidenceChip Over-Muting (Low Severity)
The `LaneEvidenceChip` is hard-coded to `not_yet_correlated` in all cases. While honest, it obscures the fact that `attestation_override_rationale` is present in the database.
- **Code:** `service.py:710`
- **Correction:** If `attestation_override_rationale` is present, the chip should show `override` with the rationale, as specified in the "Future field" section of §5.9.

## 4. Acceptance Criteria (Adversarial)

The following must be addressed before approval:
1.  **Regression Test (Byline Integrity):** Assert that an artifact published with `--allow-no-process-execution` NEVER renders as "attested" (green) even if the `author_line` matches the model pattern.
2.  **Regression Test (Historical Attestation):** Assert that a verdict issued by a session that is now `stopped` or `lost` preserves its original "attested" status (or a distinct "previously attested" state) rather than showing the same warning as a forged session.
3.  **Mandatory Rationale:** Ensure `LaneEvidenceChip` surfaces the `attestation_override_rationale` if it exists, rather than remaining muted.

---
**Verdict:** **Needs Revision**
The implementation as-is allows for byline forgery and suffers from attestation drift, both of which are high-priority regressions against the RFC 0050 honesty goals.

---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "high"
tags: ["threat_model", "security-hardening", "csrf", "dom-tampering", "dry-run"]
---

author: reviewer-unknown-model-001

# Threat Model Review: GH #9, #10, #11 Security Hardening

**Logical Name:** build_review_gemini
**Posture:** threat_model (adversarial)
**Artifact:** `docs/issues/9/review/build/gemini/REVIEW.md`

## Executive Summary

This review evaluates the threat model and proposed mitigations for GitHub issues #9, #10, and #11. The identified vulnerabilities—specifically the CSRF vulnerability on the `/v1/invoke` bridge—represent a high-severity risk to local runner integrity. The proposed mitigations in the SPEC documents are technically sound and follow defense-in-depth principles. This review confirms the attack surfaces are correctly mapped and the remediation path is sufficient to close the identified loopholes.

## Trust Boundaries & Attack Surfaces

### Trust Boundaries

1.  **Web-Origin Boundary (High Risk):** The separation between the user's browser (executing arbitrary 3rd-party JS) and the `striatum serve` instance on `localhost:8080`.
2.  **DOM-to-Script Boundary (Medium Risk):** The trust placed by JavaScript "islands" in the data-attributes of the HTML document, which may be subject to tampering via XSS or browser extensions.
3.  **API-to-CLI Boundary (Low/Medium Risk):** The transition from the web service (parsing JSON) to the CLI engine (executing `argv`).

### Attack Surfaces

1.  **`/v1/invoke` (POST):** Accepts arbitrary CLI commands. Currently lacks `Content-Type` enforcement, making it the primary vector for CSRF.
2.  **Interactive Island Attributes:** Elements with `data-job-id`, `data-session-id`, etc., used to construct command payloads.
3.  **CLI Command Logic:** Internal state management (leases, events) that may trigger before `--dry-run` checks are processed.

## Finding Review

### 1. CSRF on /v1/invoke (GH #9)
*   **Analysis:** The threat is accurately modeled. The "simple request" bypass using `text/plain` to avoid CORS preflight is a classic and potent vector for localhost-bound services.
*   **Assessment:** The mitigation plan (Strict `Content-Type` check + `Origin`/`Referer` enforcement) is the industry standard for defeating this attack.
*   **Recommendation:** Ensure the `Content-Type` check is performed *before* any body parsing occurs.

### 2. Override Modal Parameter Tampering (GH #10)
*   **Analysis:** Correctly identifies that the UI context (the URL's `run_id`) is not currently bound to the action context (the `data-job-id`).
*   **Assessment:** Proposed mitigations (client-side context verification and server-side context tokens) effectively bind the operation to the intended run/job pair.
*   **Recommendation:** Signed context tokens are the most robust solution as they don't rely on client-side JS logic alone.

### 3. Recovery Panel Dry-run Side Effects (GH #11)
*   **Analysis:** This is an important defense-in-depth finding. It acknowledges that even "read-only" commands can be dangerous if they perform "setup" mutations (like taking a temporary lock or logging an audit entry) before checking the dry-run flag.
*   **Assessment:** The audit and regression test requirement is the correct approach.
*   **Recommendation:** Implement a specific test decorator or context manager that asserts zero DB mutations for dry-run test cases.

## Verdict: Accept

The threat model presented in GH #9, #10, and #11 is complete and the proposed mitigations are appropriate for the severity of the findings. No additional unmitigated trust-boundary violations were identified within the scope of these issues.

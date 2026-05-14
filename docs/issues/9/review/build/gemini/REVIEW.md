---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["security", "csrf", "dom-tampering", "dry-run", "hardening", "rfc-0050"]
---

author: reviewer-unknown-model-001

# Adversarial Security Review -- GH #9-#11 Hardening

**Logical Name:** build_review_gemini
**Posture:** threat_model
**Artifact:** `docs/issues/9/review/build/gemini/REVIEW.md`

## Executive Summary

I performed an adversarial security review of the build closing GH #9 (CSRF), GH #10 (DOM tampering), and GH #11 (dry-run leakage). The implementation in `src/striatum/service.py` and `src/striatum/cli/recovery.py` robustly addresses the identified vulnerabilities with fail-closed gates and exact parsing. Verdict: **accept**.

## Trust Boundaries & Attack Surfaces

The build hardens the following boundaries:
1. **Local Browser -> Runner (/v1/invoke):** Now protected by strict Content-Type validation and same-origin enforcement (Origin/Referer/Host).
2. **DOM -> Runner (Override Modal):** Now protected by a process-local HMAC token binding the job context to the mutation request.
3. **Dry-Run -> State (Recovery CLI):** Now strictly read-only for workflow-domain state by construction.

## Findings Verification

### Finding 1: CSRF on /v1/invoke (GH #9)
The implementation of `is_json_content_type` and `_verify_same_origin_mutation` in `src/striatum/service.py` provides layered defense against CSRF:
- **Exact Media-Type Match:** Rejects "simple" CORS requests (e.g., `text/plain`) that elide preflight. The parser is robust against substring, prefix, and comma-injection tricks.
- **Fail-Closed Same-Origin Policy:** Unauthenticated POSTs require a valid `Host` and either a matching `Origin` or `Referer`. All identifiers are compared against an allowlist derived from the actual bound loopback listener, defeating DNS rebinding.
- **Bearer Exemption:** Authenticated non-browser clients (e.g., CLI, scripts) are appropriately exempted via `_has_valid_bearer()`.

### Finding 2: DOM Tampering on Override Modal (GH #10)
The override modal in `src/striatum/web/templates/job_detail.html` and `src/striatum/web/static/override_verdict.js` is now bound to the rendered context:
- **HMAC Token:** Server-side `make_web_context_token` binds `run_id`, `job_id`, and `session_id` using a process-local secret.
- **Client-Side Guard:** JS verifies the DOM `data-*` attributes against the page URL before submission.
- **Server-Side Validation:** `/v1/invoke` rejects `override-verdict` if the `web_context` envelope is missing, malformed, or carries a token that doesn't match the CLI `argv`.

### Finding 3: Dry-Run Mutation Leakage (GH #11)
The `recovery auto-publish --dry-run` command in `src/striatum/cli/recovery.py` was refactored for strict read-only semantics:
- **Gated Expiry:** `expire_leases` is only called on the live path.
- **Constructive Read-Only:** The `dry_run` branch in `auto_publish_stale_artifacts` exits via `continue` before any mutations (`ack_work`, `publish_artifact`, `complete_job`) occur.
- **Gated Run Completion:** `maybe_complete_run` is gated behind the live path.
- The implementation preserves auditability by allowing metadata-only audit/request logs while protecting workflow-domain tables (`leases`, `jobs`, `events`, etc.).

## Verdict

**Accept.** The security hardening is comprehensive, fail-closed, and correctly addresses the adversarial threats identified in GH #9-#11.

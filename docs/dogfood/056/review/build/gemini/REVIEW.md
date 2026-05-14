---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "high"
tags: ["threat_model", "rfc-0050", "v2", "adversarial", "csrf", "v1-invoke"]
---

author: reviewer-unknown-model-001

# Build Review — Gemini Adversarial

**Logical Name:** build_review_gemini
**Posture:** threat_model (adversarial)
**Artifact:** `docs/dogfood/056/review/build/gemini/REVIEW.md`

## Executive Summary

The implementation of RFC 0050 V2 correctly delivers the functional redesign and data-binding requirements. However, the introduction of the `/v1/invoke` mutation bridge and the new interactive islands exposes several adversarial trust-boundary risks. The most critical finding is the absence of CSRF protection for `/v1/invoke`, which allows any website visited in the same browser to perform operator-level mutations (such as overriding verdicts or canceling runs) if the server is run without a Bearer token (the default for local UI usage).

## Finding 1: Lack of CSRF Protection for `/v1/invoke`

- **Severity:** High
- **Description:** `POST /v1/invoke` accepts arbitrary CLI `argv` payloads and executes them as the operator. While `striatum serve` is restricted to loopback, browsers allow cross-origin requests to `localhost`.
- **Attack Vector:** A malicious site can issue a `POST` request to `http://localhost:8080/v1/invoke` with a body like `{"argv": ["run", "cancel", "--run-id", "all"]}`.
- **Why CORS doesn't save us:** The server's `_read_json_body` (L3190) reads the request body as JSON but **does not validate the `Content-Type` header**. An attacker can send the payload with `Content-Type: text/plain`, which is a "simple" request and does not trigger a CORS preflight.
- **Impact:** Remote command execution on the local runner on behalf of the operator.
- **Mitigation:**
    1. Enforce `Content-Type: application/json` strictly in `_read_json_body`.
    2. Add a CSRF token requirement for all non-GET requests when `web_enabled` is true.
    3. Validate the `Origin` or `Referer` header against the service's own host.

## Finding 2: Override Modal Parameter Tampering

- **Severity:** Medium
- **Description:** The override modal in `static/override_verdict.js` builds its `argv` using `data-` attributes (`data-job-id`, `data-session-id`) read from the DOM.
- **Attack Vector:** An attacker who can manipulate the DOM (e.g., via a separate XSS vulnerability or even a malicious user script) can change these identifiers to target different jobs or sessions than intended by the UI context.
- **Impact:** Unauthorized verdict overrides on unintended jobs.
- **Mitigation:** While the server ultimately validates the `argv`, the client should verify that the targeted `job_id` belongs to the current run context before posting.

## Finding 3: Recovery Panel "Dry Run" Side Effects

- **Severity:** Medium
- **Description:** The `recovery-panel` island uses `/v1/invoke` to run `recovery auto-publish --dry-run`.
- **Attack Vector:** An attacker can trigger this "dry run" via CSRF (see Finding 1). While a dry run is intended to be safe, the runner's `auto-publish` logic must guarantee that `--dry-run` is strictly read-only and produces no side effects (like lease state changes).
- **Impact:** Unexpected state transitions if the CLI verb implementation has side-effects even in dry-run mode.
- **Mitigation:** Ensure the `auto-publish` verb is strictly read-only when `--dry-run` is present.

## Finding 4: Clipboard Hijack via `data-copy`

- **Severity:** Low
- **Description:** `static/copy_on_click.js` hooks to any element with a `data-copy` attribute.
- **Attack Vector:** A malicious PR or an XSS attack could add `data-copy` to a large, transparent overlay or a common navigation link. When the operator clicks it, a malicious command (e.g., `rm -rf /`) is placed in their clipboard.
- **Impact:** Clipboard poisoning leading to potential command execution if the operator blindly pastes into a terminal.
- **Mitigation:** Restrict the `copy_on_click` behavior to specific allowed container classes (e.g., `.recipe-list` or `.data-table`) rather than the entire document.

## Finding 5: Workflow Editor Data Binding Integrity (Ghost Fields)

- **Severity:** Low
- **Description:** In `WorkflowGraphEditor.tsx`, the `require_attested_lane` field is only visible in the inspector for `review` jobs. However, if a job's `type` is changed from `review` to `generic`, the `require_attested_lane` property is **not purged** from the internal state.
- **Evidence:** `jobNodeLabel` (L146) will still render `require_attested_lane=true` for a `generic` job if it was previously a `review` job, even though the checkbox is now hidden in the inspector.
- **Impact:** Confusing UI and potentially invalid workflow JSON being saved to disk (though the server-side validator should catch it).
- **Mitigation:** Update `handleJobChange` or the inspector to explicitly purge fields that are not applicable to the current job type.

## Verdict

**Conditional Accept.** The adversarial risks around CSRF and parameter tampering are inherent to the `/v1/invoke` bridge design but are exacerbated by the lack of `Content-Type` validation and CSRF tokens. Acceptance is contingent on acknowledging these trust boundaries and prioritizing CSRF mitigation in the next security hardening pass.
